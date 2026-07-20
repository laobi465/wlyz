// Package multilevel 多级代理核心逻辑单元测试
// v0.4.0：覆盖跨级佣金分发 / 层级校验 / 代理树构建 三大核心路径
// 严格遵循铁律 06：所有断言基于已知固定输入，无随机/不确定性
package multilevel

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/your-org/keyauth-saas/apps/server/internal/config"
	"github.com/your-org/keyauth-saas/apps/server/internal/model"
)

// setupTestDB 启动 SQLite 内存库 + AutoMigrate agent / agent_invite_code / agent_balance_log / sys_config
func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared&_pragma=foreign_keys(1)"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&model.Agent{}, &model.AgentInviteCode{}, &model.AgentBalanceLog{}, &model.SysConfig{},
	))
	// 清空（cache=shared 模式下可能残留旧数据）
	db.Exec("DELETE FROM agent")
	db.Exec("DELETE FROM agent_invite_code")
	db.Exec("DELETE FROM agent_balance_log")
	db.Exec("DELETE FROM sys_config")
	return db
}

// setupTestCfgCache 启动 miniredis + ConfigCache + 预置多级代理配置
func setupTestCfgCache(t *testing.T, db *gorm.DB, overrides map[string]string) *config.ConfigCache {
	t.Helper()
	mr, err := miniredis.Run()
	require.NoError(t, err)
	t.Cleanup(mr.Close)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	// 预置默认配置
	defaults := map[string]string{
		"agent.commission.cross_level_2_rate":  "50.00",
		"agent.commission.cross_level_3_rate":  "20.00",
		"agent.commission.max_level":           "3",
		"agent.invite_code.agent_can_create":   "1",
	}
	if overrides == nil {
		overrides = map[string]string{}
	}
	for k, v := range defaults {
		if _, ok := overrides[k]; !ok {
			overrides[k] = v
		}
	}
	for k, v := range overrides {
		require.NoError(t, db.Create(&model.SysConfig{
			ConfigKey:   k,
			ConfigValue: v,
			ConfigType:  "string",
			ConfigGroup: "agent",
		}).Error)
	}
	return config.NewConfigCache(db, rdb)
}

// seedAgent 在 DB 中创建一个代理（指定层级 + 父级 + 余额）
func seedAgent(t *testing.T, db *gorm.DB, id uint64, tenantID uint64, parentID uint64, level int, balance float64, status string) *model.Agent {
	t.Helper()
	a := &model.Agent{
		BaseModel:      model.BaseModel{ID: id},
		TenantID:       tenantID,
		Username:       "agent_" + strconvFormatUint(id),
		PasswordHash:   "$2a$12$dummyhashplaceholderdummyhashplaceholderdummyhash",
		Status:         status,
		Balance:        balance,
		CommissionRate: 10.00,
		CommissionMode: "percentage",
		ParentID:       parentID,
		Level:          level,
	}
	require.NoError(t, db.Create(a).Error)
	return a
}

// strconvFormatUint 简单 uint64 → string，避免引入额外包
func strconvFormatUint(n uint64) string {
	if n == 0 {
		return "0"
	}
	var buf []byte
	for n > 0 {
		buf = append([]byte{byte('0' + n%10)}, buf...)
		n /= 10
	}
	return string(buf)
}

// ============== 1. DistributeCrossCommission 跨级佣金分发 ==============

func TestDistributeCrossCommission_Level1_NoParent(t *testing.T) {
	// 一级代理（parent_id=0）→ 无跨级佣金
	db := setupTestDB(t)
	cfg := setupTestCfgCache(t, db, nil)

	agent := seedAgent(t, db, 1, 100, 0, 1, 100.00, "active")

	results, err := DistributeCrossCommission(context.Background(), db, cfg, agent, 50.00, "[1,2,3]")
	require.NoError(t, err)
	assert.Nil(t, results, "一级代理无父级，应返回 nil")

	// 验证：DB 中无 cross_commission 流水
	var count int64
	db.Model(&model.AgentBalanceLog{}).Where("type = ?", "cross_commission").Count(&count)
	assert.Equal(t, int64(0), count, "不应有跨级佣金流水")
}

func TestDistributeCrossCommission_Level2_ParentGets50Percent(t *testing.T) {
	// 二级代理产生 100 佣金 → 父级（一级）获得 100 * 50% = 50
	db := setupTestDB(t)
	cfg := setupTestCfgCache(t, db, nil)

	parent := seedAgent(t, db, 1, 100, 0, 1, 200.00, "active")
	child := seedAgent(t, db, 2, 100, parent.ID, 2, 50.00, "active")

	results, err := DistributeCrossCommission(context.Background(), db, cfg, child, 100.00, "[10,11,12]")
	require.NoError(t, err)
	require.Len(t, results, 1, "应只有一笔跨级佣金")

	// 验证结果字段
	assert.Equal(t, parent.ID, results[0].AgentID)
	assert.Equal(t, 1, results[0].Level)
	assert.Equal(t, 50.00, results[0].Amount, "100 * 50% = 50")
	assert.Equal(t, 50.00, results[0].Rate)
	assert.Equal(t, child.ID, results[0].SourceAgentID)

	// 验证父级 balance 已加 50
	var updatedParent model.Agent
	db.First(&updatedParent, parent.ID)
	assert.Equal(t, 250.00, updatedParent.Balance, "父级余额 200 + 50 = 250")

	// 验证流水
	var log model.AgentBalanceLog
	db.Where("type = ? AND agent_id = ?", "cross_commission", parent.ID).First(&log)
	assert.Equal(t, 50.00, log.Amount)
	assert.Equal(t, 250.00, log.BalanceAfter)
	assert.Equal(t, "settled", log.Status)
	assert.Contains(t, log.Remark, "跨级佣金")
}

func TestDistributeCrossCommission_Level3_ParentAndGrandparent(t *testing.T) {
	// 三级代理产生 100 佣金：
	//   - 父级（二级）获得 100 * 50% = 50
	//   - 祖父级（一级）获得 100 * 20% = 20
	db := setupTestDB(t)
	cfg := setupTestCfgCache(t, db, nil)

	grand := seedAgent(t, db, 1, 100, 0, 1, 0.00, "active")   // 一级（祖父）
	parent := seedAgent(t, db, 2, 100, grand.ID, 2, 0.00, "active") // 二级（父）
	child := seedAgent(t, db, 3, 100, parent.ID, 3, 0.00, "active") // 三级

	results, err := DistributeCrossCommission(context.Background(), db, cfg, child, 100.00, "[20]")
	require.NoError(t, err)
	require.Len(t, results, 2, "应有 2 笔跨级佣金（父级 + 祖父级）")

	// 第一笔：父级（二级）获得 50
	assert.Equal(t, parent.ID, results[0].AgentID)
	assert.Equal(t, 2, results[0].Level)
	assert.Equal(t, 50.00, results[0].Amount)
	assert.Equal(t, 50.00, results[0].Rate)

	// 第二笔：祖父级（一级）获得 20
	assert.Equal(t, grand.ID, results[1].AgentID)
	assert.Equal(t, 1, results[1].Level)
	assert.Equal(t, 20.00, results[1].Amount)
	assert.Equal(t, 20.00, results[1].Rate)

	// 验证余额
	var updatedParent, updatedGrand model.Agent
	db.First(&updatedParent, parent.ID)
	db.First(&updatedGrand, grand.ID)
	assert.Equal(t, 50.00, updatedParent.Balance)
	assert.Equal(t, 20.00, updatedGrand.Balance)
}

func TestDistributeCrossCommission_DisabledParent_Skipped(t *testing.T) {
	// 父级代理 status='disabled' → 跳过该级（不分发跨级佣金）
	db := setupTestDB(t)
	cfg := setupTestCfgCache(t, db, nil)

	parent := seedAgent(t, db, 1, 100, 0, 1, 100.00, "disabled") // 已禁用
	child := seedAgent(t, db, 2, 100, parent.ID, 2, 50.00, "active")

	results, err := DistributeCrossCommission(context.Background(), db, cfg, child, 100.00, "[1]")
	require.NoError(t, err)
	assert.Nil(t, results, "父级已禁用应跳过，无跨级佣金")

	var updatedParent model.Agent
	db.First(&updatedParent, parent.ID)
	assert.Equal(t, 100.00, updatedParent.Balance, "父级余额不变")
}

func TestDistributeCrossCommission_ZeroCommission_NoOp(t *testing.T) {
	// 佣金为 0 → 无跨级分发
	db := setupTestDB(t)
	cfg := setupTestCfgCache(t, db, nil)

	parent := seedAgent(t, db, 1, 100, 0, 1, 100.00, "active")
	child := seedAgent(t, db, 2, 100, parent.ID, 2, 50.00, "active")

	results, err := DistributeCrossCommission(context.Background(), db, cfg, child, 0.00, "[1]")
	require.NoError(t, err)
	assert.Nil(t, results, "佣金为 0 应返回 nil")
}

func TestDistributeCrossCommission_NilAgent_NoOp(t *testing.T) {
	db := setupTestDB(t)
	cfg := setupTestCfgCache(t, db, nil)

	results, err := DistributeCrossCommission(context.Background(), db, cfg, nil, 100.00, "[]")
	require.NoError(t, err)
	assert.Nil(t, results, "nil agent 应返回 nil")
}

func TestDistributeCrossCommission_CustomRateFromConfig(t *testing.T) {
	// 自定义跨级佣金比例：level2_rate=30, level3_rate=10
	db := setupTestDB(t)
	cfg := setupTestCfgCache(t, db, map[string]string{
		"agent.commission.cross_level_2_rate": "30.00",
		"agent.commission.cross_level_3_rate": "10.00",
	})

	grand := seedAgent(t, db, 1, 100, 0, 1, 0.00, "active")
	parent := seedAgent(t, db, 2, 100, grand.ID, 2, 0.00, "active")
	child := seedAgent(t, db, 3, 100, parent.ID, 3, 0.00, "active")

	results, err := DistributeCrossCommission(context.Background(), db, cfg, child, 200.00, "[1]")
	require.NoError(t, err)
	require.Len(t, results, 2)

	// 父级（二级）应得 200 * 30% = 60
	assert.Equal(t, 60.00, results[0].Amount)
	// 祖父级（一级）应得 200 * 10% = 20
	assert.Equal(t, 20.00, results[1].Amount)
}

// ============== 2. CanCreateSubordinate 层级资格校验 ==============

func TestCanCreateSubordinate_Level1_Max3_CanCreate(t *testing.T) {
	db := setupTestDB(t)
	cfg := setupTestCfgCache(t, db, nil)
	agent := seedAgent(t, db, 1, 100, 0, 1, 0.00, "active")

	err := CanCreateSubordinate(context.Background(), cfg, agent)
	assert.NoError(t, err, "level=1 且 max_level=3 应可创建下级")
}

func TestCanCreateSubordinate_Level3_Max3_CannotCreate(t *testing.T) {
	db := setupTestDB(t)
	cfg := setupTestCfgCache(t, db, nil)
	agent := seedAgent(t, db, 1, 100, 0, 3, 0.00, "active")

	err := CanCreateSubordinate(context.Background(), cfg, agent)
	assert.ErrorIs(t, err, ErrLevelExceedsMax, "level=3 = max_level，应禁止创建下级")
}

func TestCanCreateSubordinate_MaxLevel1_CannotCreate(t *testing.T) {
	// sys_config max_level=1 → 任何代理都不能创建下级
	db := setupTestDB(t)
	cfg := setupTestCfgCache(t, db, map[string]string{
		"agent.commission.max_level": "1",
	})
	agent := seedAgent(t, db, 1, 100, 0, 1, 0.00, "active")

	err := CanCreateSubordinate(context.Background(), cfg, agent)
	assert.ErrorIs(t, err, ErrLevelExceedsMax, "max_level=1 时即使 level=1 也不能创建下级")
}

func TestCanCreateSubordinate_AgentCanCreateFalse(t *testing.T) {
	// sys_config agent_can_create=0 → 禁止
	db := setupTestDB(t)
	cfg := setupTestCfgCache(t, db, map[string]string{
		"agent.invite_code.agent_can_create": "0",
	})
	agent := seedAgent(t, db, 1, 100, 0, 1, 0.00, "active")

	err := CanCreateSubordinate(context.Background(), cfg, agent)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "agent_can_create")
}

func TestCanCreateSubordinate_DisabledAgent_CannotCreate(t *testing.T) {
	db := setupTestDB(t)
	cfg := setupTestCfgCache(t, db, nil)
	agent := seedAgent(t, db, 1, 100, 0, 1, 0.00, "disabled")

	err := CanCreateSubordinate(context.Background(), cfg, agent)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "active")
}

func TestCanCreateSubordinate_NilAgent_ErrAgentNotFound(t *testing.T) {
	db := setupTestDB(t)
	cfg := setupTestCfgCache(t, db, nil)

	err := CanCreateSubordinate(context.Background(), cfg, nil)
	assert.ErrorIs(t, err, ErrAgentNotFound)
}

// ============== 3. ComputeSubordinateLevel 层级计算 ==============

func TestComputeSubordinateLevel_TenantInviteCode_Level1(t *testing.T) {
	// 开发者创建的邀请码 → 新代理 level=1, parent_id=0
	db := setupTestDB(t)
	cfg := setupTestCfgCache(t, db, nil)

	ic := &model.AgentInviteCode{
		TenantID:       100,
		Code:           "TENANTCODE001",
		Status:         "active",
		CreatorType:    "tenant",
		CreatorAgentID: 0,
	}

	parentID, level, err := ComputeSubordinateLevel(context.Background(), db, cfg, ic)
	require.NoError(t, err)
	assert.Equal(t, uint64(0), parentID, "开发者邀请码 → parent_id=0")
	assert.Equal(t, 1, level, "开发者邀请码 → level=1")
}

func TestComputeSubordinateLevel_AgentInviteCode_Level2(t *testing.T) {
	// 一级代理创建的邀请码 → 新代理 level=2, parent_id=creator.id
	db := setupTestDB(t)
	cfg := setupTestCfgCache(t, db, nil)

	creator := seedAgent(t, db, 5, 100, 0, 1, 0.00, "active")
	ic := &model.AgentInviteCode{
		TenantID:       100,
		Code:           "AGENTCODE001",
		Status:         "active",
		CreatorType:    "agent",
		CreatorAgentID: creator.ID,
	}

	parentID, level, err := ComputeSubordinateLevel(context.Background(), db, cfg, ic)
	require.NoError(t, err)
	assert.Equal(t, creator.ID, parentID)
	assert.Equal(t, 2, level)
}

func TestComputeSubordinateLevel_Level3Creator_ExceedsMax(t *testing.T) {
	// 三级代理（已到 max_level=3）创建邀请码 → 应拒绝
	db := setupTestDB(t)
	cfg := setupTestCfgCache(t, db, nil)

	creator := seedAgent(t, db, 5, 100, 1, 3, 0.00, "active") // level=3
	ic := &model.AgentInviteCode{
		TenantID:       100,
		Code:           "AGENTCODE002",
		Status:         "active",
		CreatorType:    "agent",
		CreatorAgentID: creator.ID,
	}

	_, _, err := ComputeSubordinateLevel(context.Background(), db, cfg, ic)
	assert.ErrorIs(t, err, ErrLevelExceedsMax)
}

func TestComputeSubordinateLevel_CreatorNotFound(t *testing.T) {
	// 邀请码 creator_agent_id 指向不存在的代理 → 应报错
	db := setupTestDB(t)
	cfg := setupTestCfgCache(t, db, nil)

	ic := &model.AgentInviteCode{
		TenantID:       100,
		Code:           "GHOSTAGENT001",
		Status:         "active",
		CreatorType:    "agent",
		CreatorAgentID: 999, // 不存在
	}

	_, _, err := ComputeSubordinateLevel(context.Background(), db, cfg, ic)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "不存在")
}

func TestComputeSubordinateLevel_NilInviteCode(t *testing.T) {
	db := setupTestDB(t)
	cfg := setupTestCfgCache(t, db, nil)

	_, _, err := ComputeSubordinateLevel(context.Background(), db, cfg, nil)
	assert.ErrorIs(t, err, ErrAgentNotFound)
}

// ============== 4. BuildAgentTree 代理树构建 ==============

func TestBuildAgentTree_ThreeLevelTree(t *testing.T) {
	// 构建三层代理树：1 → 2 → 3
	db := setupTestDB(t)
	seedAgent(t, db, 1, 100, 0, 1, 0.00, "active")                // 一级
	seedAgent(t, db, 2, 100, 1, 2, 0.00, "active")                // 二级（父=1）
	seedAgent(t, db, 3, 100, 2, 3, 0.00, "active")                // 三级（父=2）
	seedAgent(t, db, 4, 100, 1, 2, 0.00, "active")                // 二级（父=1，兄弟）
	seedAgent(t, db, 5, 100, 4, 3, 0.00, "active")                // 三级（父=4）

	tree, err := BuildAgentTree(context.Background(), db, 1, 2) // maxDepth=2
	require.NoError(t, err)

	assert.Equal(t, uint64(1), tree.Agent.ID)
	require.Len(t, tree.Children, 2, "一级代理应有 2 个二级子节点")

	// 子 1（id=2）
	child1 := tree.Children[0]
	assert.Equal(t, uint64(2), child1.Agent.ID)
	require.Len(t, child1.Children, 1, "二级代理 id=2 应有 1 个三级子节点")
	assert.Equal(t, uint64(3), child1.Children[0].Agent.ID)

	// 子 2（id=4）
	child2 := tree.Children[1]
	assert.Equal(t, uint64(4), child2.Agent.ID)
	require.Len(t, child2.Children, 1, "二级代理 id=4 应有 1 个三级子节点")
	assert.Equal(t, uint64(5), child2.Children[0].Agent.ID)
}

func TestBuildAgentTree_MaxDepth0_OnlyRoot(t *testing.T) {
	db := setupTestDB(t)
	seedAgent(t, db, 1, 100, 0, 1, 0.00, "active")
	seedAgent(t, db, 2, 100, 1, 2, 0.00, "active")

	tree, err := BuildAgentTree(context.Background(), db, 1, 0)
	require.NoError(t, err)
	assert.Equal(t, uint64(1), tree.Agent.ID)
	assert.Empty(t, tree.Children, "maxDepth=0 应只返回根节点")
}

func TestBuildAgentTree_AgentNotFound(t *testing.T) {
	db := setupTestDB(t)

	_, err := BuildAgentTree(context.Background(), db, 999, 3)
	assert.ErrorIs(t, err, ErrAgentNotFound)
}

func TestBuildAgentTree_TenantIsolation(t *testing.T) {
	// 跨租户的下级不应被查询到（BuildAgentTree 内部 WHERE tenant_id = parent.tenant_id）
	db := setupTestDB(t)
	seedAgent(t, db, 1, 100, 0, 1, 0.00, "active") // 租户 100 的一级
	seedAgent(t, db, 2, 200, 1, 2, 0.00, "active") // 租户 200 的代理（虽然 parent_id=1，但 tenant 不同）

	tree, err := BuildAgentTree(context.Background(), db, 1, 2)
	require.NoError(t, err)
	assert.Empty(t, tree.Children, "跨租户的代理不应出现在子节点")
}

// ============== 5. ListSubordinates 直接下级列表 ==============

func TestListSubordinates_SingleLevel(t *testing.T) {
	db := setupTestDB(t)
	seedAgent(t, db, 1, 100, 0, 1, 0.00, "active")
	seedAgent(t, db, 2, 100, 1, 2, 0.00, "active")
	seedAgent(t, db, 3, 100, 1, 2, 0.00, "active")
	seedAgent(t, db, 4, 100, 2, 3, 0.00, "active") // 三级（不应出现在 id=1 的直接下级中）

	subs, err := ListSubordinates(context.Background(), db, 1, 100)
	require.NoError(t, err)
	assert.Len(t, subs, 2, "id=1 的直接下级应有 2 个（id=2, id=3）")

	// 不应包含三级代理
	for _, s := range subs {
		assert.NotEqual(t, uint64(4), s.ID, "不应包含三级代理 id=4")
	}
}

func TestListSubordinates_NoChildren(t *testing.T) {
	db := setupTestDB(t)
	seedAgent(t, db, 1, 100, 0, 1, 0.00, "active")

	subs, err := ListSubordinates(context.Background(), db, 1, 100)
	require.NoError(t, err)
	assert.Empty(t, subs)
}

func TestListSubordinates_TenantIsolation(t *testing.T) {
	// 跨租户的下级不应被查询到
	db := setupTestDB(t)
	seedAgent(t, db, 1, 100, 0, 1, 0.00, "active")
	seedAgent(t, db, 2, 200, 1, 2, 0.00, "active") // 不同租户

	subs, err := ListSubordinates(context.Background(), db, 1, 100)
	require.NoError(t, err)
	assert.Empty(t, subs, "跨租户不应返回")
}

// ============== 6. 边界场景 ==============

func TestDistributeCrossCommission_NegativeCommission_NoOp(t *testing.T) {
	// 负佣金（理论不应出现，但 DistributeCrossCommission 应安全处理）
	db := setupTestDB(t)
	cfg := setupTestCfgCache(t, db, nil)

	parent := seedAgent(t, db, 1, 100, 0, 1, 100.00, "active")
	child := seedAgent(t, db, 2, 100, parent.ID, 2, 50.00, "active")

	results, err := DistributeCrossCommission(context.Background(), db, cfg, child, -10.00, "[]")
	require.NoError(t, err)
	assert.Nil(t, results, "负佣金应返回 nil")

	// 验证父级余额未变
	var updatedParent model.Agent
	db.First(&updatedParent, parent.ID)
	assert.Equal(t, 100.00, updatedParent.Balance)
}

func TestDistributeCrossCommission_ParentChainBroken(t *testing.T) {
	// 父级已被物理删除（parent_id 指向不存在的代理）→ 应优雅停止，不报错
	db := setupTestDB(t)
	cfg := setupTestCfgCache(t, db, nil)

	// 创建一个 parent_id=999（不存在）的二级代理
	child := seedAgent(t, db, 2, 100, 999, 2, 50.00, "active")

	results, err := DistributeCrossCommission(context.Background(), db, cfg, child, 100.00, "[1]")
	require.NoError(t, err, "父级不存在应优雅停止")
	assert.Nil(t, results, "父级不存在应返回 nil")
}

// 防止 time 包未使用导入报错（agent 表 LastLoginAt 字段使用 *time.Time）
var _ = time.Time{}
