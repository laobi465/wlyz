// Package handler P0 修复并发测试 + 失败回滚测试
// 覆盖前序 P0 修复的核心守门逻辑（事务 + 状态守门 + RowsAffected + agent 锁 + frozen_balance 守门）：
//   1. TenantApproveRecharge 充值审核：并发仅一次成功 + 失败回滚
//   2. TenantRejectWithdraw 提现驳回：并发仅一次成功（balance 仅退一次）
//   3. TenantPayWithdraw 提现打款：并发仅一次成功
//   4. processPaidOrder 支付回调：并发仅一次成功 + 幂等短路
//   5. AdminPayTenantWithdraw 开发者提现打款：frozen_balance 不足时事务回滚
//
// 严格遵循铁律 06：所有断言基于已知固定输入，无随机/不确定性
// 测试基础设施：SQLite 内存库（cache=shared + busy_timeout）+ miniredis
//
// 注：SQLite 静默忽略 SELECT ... FOR UPDATE（clause.Locking{Strength:"UPDATE"}），
// 并发安全完全依赖 WHERE status='pending' + RowsAffected 检查（数据库级幂等）。
// 多 goroutine 并发场景下，SQLite 写入串行化（busy_timeout 防止 "database is locked"），
// 第一个事务提交后其余事务的 WHERE 子句不再匹配 → RowsAffected=0 → 返回错误，从而验证守门有效性。
package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/your-org/keyauth-saas/apps/server/internal/config"
	"github.com/your-org/keyauth-saas/apps/server/internal/model"
	"github.com/your-org/keyauth-saas/apps/server/pkg/crypto"
	"github.com/your-org/keyauth-saas/apps/server/pkg/epay"
)

// ============== P0 测试基础设施 ==============

// setupP0DB 启动独立 SQLite 内存库（独立 DSN 避免与其他测试串扰）
// 启用 busy_timeout=5000ms 防止并发写时 "database is locked" 错误
func setupP0DB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:p0_conc_test?mode=memory&cache=shared&_pragma=busy_timeout(5000)"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&model.SysConfig{},
		&model.SysAdmin{},
		&model.SysTenant{},
		&model.SysPackage{},
		&model.App{},
		&model.AppCardType{},
		&model.AppCard{},
		&model.AppOrder{},
		&model.Agent{},
		&model.AgentBalanceLog{},
		&model.AgentWithdraw{},
		&model.TenantWithdraw{},
		&model.TenantBalanceLog{},
		&model.PlatformSettlement{},
		&model.LogOperation{},
	))
	// 清空表（cache=shared 模式下可能残留旧数据）
	for _, tbl := range []string{
		"sys_config", "sys_admin", "sys_tenant", "sys_package",
		"app", "app_card_type", "app_card", "app_order",
		"agent", "agent_balance_log", "agent_withdraw",
		"tenant_withdraw", "tenant_balance_log", "platform_settlement",
		"log_operation",
	} {
		db.Exec("DELETE FROM " + tbl)
	}
	return db
}

// setupP0Crypto 启动 AES-256 crypto manager
func setupP0Crypto(t *testing.T) *crypto.Manager {
	t.Helper()
	mgr, err := crypto.NewManager("0123456789abcdef0123456789abcdef", "", "")
	require.NoError(t, err)
	return mgr
}

// setupP0MiniRedis 启动 miniredis + 返回 redis.Client
func setupP0MiniRedis(t *testing.T) *redis.Client {
	t.Helper()
	mr, err := miniredis.Run()
	require.NoError(t, err)
	t.Cleanup(mr.Close)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	return rdb
}

// setupP0Deps 构造测试 Deps（含真实 ConfigCache + 注入 sys_config 默认值）
func setupP0Deps(t *testing.T, cfgOverrides map[string]string) *Deps {
	t.Helper()
	rdb := setupP0MiniRedis(t)
	db := setupP0DB(t)

	defaults := map[string]string{
		"pay.platform.enabled":           "1",
		"pay.platform.commission_default": "5.00",
	}
	for k, v := range cfgOverrides {
		defaults[k] = v
	}
	for k, v := range defaults {
		require.NoError(t, db.Create(&model.SysConfig{
			ConfigKey:   k,
			ConfigValue: v,
			ConfigType:  "string",
			ConfigGroup: "test",
		}).Error)
	}

	cfgCache := config.NewConfigCache(db, rdb)
	require.NoError(t, cfgCache.Preload(context.Background()))

	return &Deps{
		DB:       db,
		Redis:    rdb,
		Crypto:   setupP0Crypto(t),
		CfgCache: cfgCache,
	}
}

// ============== 通用辅助：HTTP 调用 ==============

// callTenantEndpoint 模拟 tenant 上下文调用任意 tenant 端点
// routePattern 必须使用 :id 等 gin 参数占位符（如 /tenant/recharge_requests/:id/approve），
// requestPath 是实际请求 URL（如 /tenant/recharge_requests/7101/approve）。
// 注入 tenant_id / user_id / role，让 handler 内 getTenantID / getUserID 能正确读取
func callTenantEndpoint(t *testing.T, deps *Deps, method, routePattern, requestPath string, body interface{}, tenantID uint64, handlerFn gin.HandlerFunc) *httptest.ResponseRecorder {
	t.Helper()
	g := setupGin()
	g.Handle(method, routePattern, func(c *gin.Context) {
		c.Set("role", "tenant")
		c.Set("tenant_id", tenantID)
		c.Set("user_id", tenantID)
		c.Set("username", "tenant_user")
		c.Next()
	}, handlerFn)

	w := httptest.NewRecorder()
	var req *http.Request
	if body != nil {
		b, _ := json.Marshal(body)
		req, _ = http.NewRequest(method, requestPath, strings.NewReader(string(b)))
		req.Header.Set("Content-Type", "application/json")
	} else {
		req, _ = http.NewRequest(method, requestPath, nil)
	}
	g.ServeHTTP(w, req)
	return w
}

// callAdminEndpoint 模拟 admin 上下文调用任意 admin 端点
// 同 callTenantEndpoint：routePattern 使用 :id 占位符，requestPath 为实际 URL
func callAdminEndpoint(t *testing.T, deps *Deps, method, routePattern, requestPath string, body interface{}, adminID uint64, handlerFn gin.HandlerFunc) *httptest.ResponseRecorder {
	t.Helper()
	g := setupGin()
	g.Handle(method, routePattern, func(c *gin.Context) {
		c.Set("role", "admin")
		c.Set("user_id", adminID)
		c.Set("username", "admin_root")
		c.Next()
	}, handlerFn)

	w := httptest.NewRecorder()
	var req *http.Request
	if body != nil {
		b, _ := json.Marshal(body)
		req, _ = http.NewRequest(method, requestPath, strings.NewReader(string(b)))
		req.Header.Set("Content-Type", "application/json")
	} else {
		req, _ = http.NewRequest(method, requestPath, nil)
	}
	g.ServeHTTP(w, req)
	return w
}

// ============== 种子数据 ==============

func seedP0Tenant(t *testing.T, db *gorm.DB, id uint64, username string, balance, frozenBalance float64) *model.SysTenant {
	t.Helper()
	tt := &model.SysTenant{
		BaseModel:     model.BaseModel{ID: id},
		TenantCode:    fmt.Sprintf("tc-%d", id),
		Username:      username,
		Status:        "active",
		Balance:       balance,
		FrozenBalance: frozenBalance,
	}
	require.NoError(t, db.Create(tt).Error)
	return tt
}

func seedP0Package(t *testing.T, db *gorm.DB, id uint64, commissionRate float64) *model.SysPackage {
	t.Helper()
	pkg := &model.SysPackage{
		BaseModel:              model.BaseModel{ID: id},
		Name:                   fmt.Sprintf("pkg-%d", id),
		PlatformCommissionRate: commissionRate,
		Status:                 "active",
	}
	require.NoError(t, db.Create(pkg).Error)
	return pkg
}

func seedP0Agent(t *testing.T, db *gorm.DB, id, tenantID uint64, username, status string, balance float64) *model.Agent {
	t.Helper()
	a := &model.Agent{
		BaseModel:    model.BaseModel{ID: id},
		TenantID:     tenantID,
		Username:     username,
		PasswordHash: "bcrypt_hash",
		Status:       status,
		Balance:      balance,
	}
	require.NoError(t, db.Create(a).Error)
	return a
}

func seedP0RechargeLog(t *testing.T, db *gorm.DB, id, agentID, tenantID uint64, amount float64, status string) *model.AgentBalanceLog {
	t.Helper()
	log := &model.AgentBalanceLog{
		BaseModel:    model.BaseModel{ID: id},
		AgentID:      agentID,
		TenantID:     tenantID,
		Type:         "recharge",
		Amount:       amount,
		Status:       status,
	}
	require.NoError(t, db.Create(log).Error)
	return log
}

func seedP0Withdraw(t *testing.T, db *gorm.DB, id, agentID, tenantID uint64, amount float64, status string) *model.AgentWithdraw {
	t.Helper()
	w := &model.AgentWithdraw{
		BaseModel:  model.BaseModel{ID: id},
		AgentID:    agentID,
		TenantID:   tenantID,
		Amount:     amount,
		PayMethod:  "alipay",
		PayAccount: "test@example.com",
		Status:     status,
	}
	require.NoError(t, db.Create(w).Error)
	return w
}

// seedP0WithdrawBalanceLog 创建与提现申请关联的 balance_log（type=withdraw）
// relatedWithdrawID 指向 agent_withdraw.id，供 TenantPayWithdraw / TenantRejectWithdraw 精确匹配
func seedP0WithdrawBalanceLog(t *testing.T, db *gorm.DB, id, agentID, tenantID, withdrawID uint64, amount float64, status string) *model.AgentBalanceLog {
	t.Helper()
	log := &model.AgentBalanceLog{
		BaseModel:        model.BaseModel{ID: id},
		AgentID:          agentID,
		TenantID:         tenantID,
		Type:             "withdraw",
		Amount:           amount,
		RelatedWithdrawID: &withdrawID,
		Status:           status,
	}
	require.NoError(t, db.Create(log).Error)
	return log
}

func seedP0App(t *testing.T, db *gorm.DB, id, tenantID uint64, name string) *model.App {
	t.Helper()
	a := &model.App{
		BaseModel:   model.BaseModel{ID: id},
		TenantID:    tenantID,
		AppKey:      "ak_" + name,
		AppSecret:   "secret_enc",
		SignSecret:  "sign_enc",
		Name:        name,
		Status:      "active",
		AuditStatus: "approved",
	}
	require.NoError(t, db.Create(a).Error)
	return a
}

func seedP0CardType(t *testing.T, db *gorm.DB, id, tenantID, appID uint64, price float64) *model.AppCardType {
	t.Helper()
	ct := &model.AppCardType{
		BaseModel:        model.BaseModel{ID: id},
		TenantID:         tenantID,
		AppID:            appID,
		Name:             fmt.Sprintf("ct-%d", id),
		Type:             "duration",
		DurationSeconds:  86400,
		MaxUses:          1,
		Price:            price,
		AgentBasePrice:   price,
		Status:           "active",
	}
	require.NoError(t, db.Create(ct).Error)
	return ct
}

func seedP0Order(t *testing.T, db *gorm.DB, id, tenantID, appID, cardTypeID uint64, orderNo string, totalAmount float64, quantity int, payStatus string) *model.AppOrder {
	t.Helper()
	o := &model.AppOrder{
		BaseModel:   model.BaseModel{ID: id},
		TenantID:    tenantID,
		AppID:       appID,
		CardTypeID:  cardTypeID,
		OrderNo:     orderNo,
		Quantity:    quantity,
		UnitPrice:   totalAmount / float64(quantity),
		TotalAmount: totalAmount,
		PayChannel:  "epay_alipay",
		PayStatus:   payStatus,
	}
	require.NoError(t, db.Create(o).Error)
	return o
}

func seedP0TenantWithdraw(t *testing.T, db *gorm.DB, id, tenantID uint64, amount float64, status string) *model.TenantWithdraw {
	t.Helper()
	w := &model.TenantWithdraw{
		BaseModel:  model.BaseModel{ID: id},
		TenantID:   tenantID,
		Amount:     amount,
		PayMethod:  "alipay",
		PayAccount: "tenant@example.com",
		Status:     status,
	}
	require.NoError(t, db.Create(w).Error)
	return w
}

// ============== 2.1 充值审核并发测试 ==============

// TestTenantApproveRecharge_Concurrent 10 个 goroutine 并发审核同一笔充值申请
// 断言：
//   - agent.balance 仅增加 1 次（100，不是 1000）
//   - agent_balance_log.status == "settled"
//   - 仅一个 goroutine 成功（其余 9 个返回 5xx + 错误消息）
//
// 验证点：SQLite 静默忽略 FOR UPDATE，并发安全完全依赖
//        WHERE id=? AND status='pending' + RowsAffected 检查（数据库级幂等）
func TestTenantApproveRecharge_Concurrent(t *testing.T) {
	deps := setupP0Deps(t, nil)
	const (
		tenantID  = uint64(5101)
		agentID   = uint64(6101)
		logID     = uint64(7101)
		amount    = 100.00
		goroutine = 10
	)
	seedP0Tenant(t, deps.DB, tenantID, "tenant_approve_recharge", 0, 0)
	seedP0Agent(t, deps.DB, agentID, tenantID, "agent_approve_recharge", "active", 0)
	seedP0RechargeLog(t, deps.DB, logID, agentID, tenantID, amount, "pending")

	routePattern := "/tenant/recharge_requests/:id/approve"
	requestPath := fmt.Sprintf("/tenant/recharge_requests/%d/approve", logID)

	var (
		wg         sync.WaitGroup
		successCnt int64
		failCnt    int64
	)
	wg.Add(goroutine)
	for i := 0; i < goroutine; i++ {
		go func() {
			defer wg.Done()
			w := callTenantEndpoint(t, deps, "POST", routePattern, requestPath, nil, tenantID, TenantApproveRecharge(deps))
			if w.Code == http.StatusOK {
				code, _, _ := parseResp(t, w)
				if code == 0 {
					atomic.AddInt64(&successCnt, 1)
					return
				}
			}
			atomic.AddInt64(&failCnt, 1)
		}()
	}
	wg.Wait()

	// 断言：恰好 1 个成功，9 个失败
	assert.Equal(t, int64(1), atomic.LoadInt64(&successCnt), "应有且仅有一个 goroutine 审核成功")
	assert.Equal(t, int64(goroutine-1), atomic.LoadInt64(&failCnt), "其余 goroutine 应失败")

	// 断言：agent.balance 仅增加 1 次（100，不是 1000）
	var agent model.Agent
	require.NoError(t, deps.DB.First(&agent, agentID).Error)
	assert.Equal(t, 100.00, agent.Balance, "余额应仅被增加一次（100，不是 1000）")

	// 断言：流水状态已变更
	var logRow model.AgentBalanceLog
	require.NoError(t, deps.DB.First(&logRow, logID).Error)
	assert.Equal(t, "settled", logRow.Status, "流水状态应为 settled")
}

// ============== 2.2 充值审核失败回滚测试 ==============

// TestTenantApproveRecharge_RollbackOnAgentNotFound 充值申请指向不存在的 agent_id
// 断言：
//   - 返回错误（事务内 First agent 失败）
//   - agent_balance_log.status 仍为 pending（事务回滚，未变更）
func TestTenantApproveRecharge_RollbackOnAgentNotFound(t *testing.T) {
	deps := setupP0Deps(t, nil)
	const (
		tenantID     = uint64(5201)
		nonExistAgent = uint64(999999)
		logID        = uint64(7201)
		amount       = 100.00
	)
	seedP0Tenant(t, deps.DB, tenantID, "tenant_rollback", 0, 0)
	// 不创建 agent，让事务内 First 失败
	seedP0RechargeLog(t, deps.DB, logID, nonExistAgent, tenantID, amount, "pending")

	routePattern := "/tenant/recharge_requests/:id/approve"
	requestPath := fmt.Sprintf("/tenant/recharge_requests/%d/approve", logID)
	w := callTenantEndpoint(t, deps, "POST", routePattern, requestPath, nil, tenantID, TenantApproveRecharge(deps))

	// 事务失败 → middleware.Fail 返回 500
	assert.Equal(t, http.StatusInternalServerError, w.Code, "agent 不存在应返回 500")
	code, msg, _ := parseResp(t, w)
	assert.NotEqual(t, 0, code)
	assert.Contains(t, msg, "审核通过失败")

	// 断言：流水状态仍为 pending（事务回滚）
	var logRow model.AgentBalanceLog
	require.NoError(t, deps.DB.First(&logRow, logID).Error)
	assert.Equal(t, "pending", logRow.Status, "事务回滚后流水状态应保持 pending")
}

// ============== 2.3 提现驳回并发测试 ==============

// TestTenantRejectWithdraw_Concurrent 10 个 goroutine 并发驳回同一笔提现申请
// 提现申请时 agent.balance 已扣（balance=0），驳回会退回 amount
// 断言：
//   - agent.balance 仅退回 1 次（100，不是 1000）
//   - agent_withdraw.status == "rejected"
//   - 仅一个 goroutine 成功
//
// 验证点：状态守门 + RowsAffected 检查防止并发重复退回余额
func TestTenantRejectWithdraw_Concurrent(t *testing.T) {
	deps := setupP0Deps(t, nil)
	const (
		tenantID  = uint64(5301)
		agentID   = uint64(6301)
		withdrawID = uint64(7301)
		logID     = uint64(8301)
		amount    = 100.00
		goroutine = 10
	)
	seedP0Tenant(t, deps.DB, tenantID, "tenant_reject_withdraw", 0, 0)
	// 提现申请时已扣 balance，故 agent.balance=0
	seedP0Agent(t, deps.DB, agentID, tenantID, "agent_reject_withdraw", "active", 0)
	seedP0Withdraw(t, deps.DB, withdrawID, agentID, tenantID, amount, "pending")
	seedP0WithdrawBalanceLog(t, deps.DB, logID, agentID, tenantID, withdrawID, amount, "settled")

	routePattern := "/tenant/withdrawals/:id/reject"
	requestPath := fmt.Sprintf("/tenant/withdrawals/%d/reject", withdrawID)
	body := map[string]interface{}{"reason": "驳回测试"}

	var (
		wg         sync.WaitGroup
		successCnt int64
		failCnt    int64
	)
	wg.Add(goroutine)
	for i := 0; i < goroutine; i++ {
		go func() {
			defer wg.Done()
			w := callTenantEndpoint(t, deps, "POST", routePattern, requestPath, body, tenantID, TenantRejectWithdraw(deps))
			if w.Code == http.StatusOK {
				code, _, _ := parseResp(t, w)
				if code == 0 {
					atomic.AddInt64(&successCnt, 1)
					return
				}
			}
			atomic.AddInt64(&failCnt, 1)
		}()
	}
	wg.Wait()

	assert.Equal(t, int64(1), atomic.LoadInt64(&successCnt), "应有且仅有一个 goroutine 驳回成功")
	assert.Equal(t, int64(goroutine-1), atomic.LoadInt64(&failCnt), "其余 goroutine 应失败")

	// 断言：balance 仅退回 1 次（100，不是 1000）
	var agent model.Agent
	require.NoError(t, deps.DB.First(&agent, agentID).Error)
	assert.Equal(t, 100.00, agent.Balance, "余额应仅被退回一次（100，不是 1000）")

	// 断言：提现状态已变更为 rejected
	var wd model.AgentWithdraw
	require.NoError(t, deps.DB.First(&wd, withdrawID).Error)
	assert.Equal(t, "rejected", wd.Status, "提现状态应为 rejected")

	// 断言：balance_log 状态也同步变更为 rejected
	var logRow model.AgentBalanceLog
	require.NoError(t, deps.DB.First(&logRow, logID).Error)
	assert.Equal(t, "rejected", logRow.Status, "balance_log 状态应同步为 rejected")
}

// ============== 2.4 提现打款并发测试 ==============

// TestTenantPayWithdraw_Concurrent 10 个 goroutine 并发对同一笔提现申请打款
// 断言：
//   - agent_withdraw.status == "paid"
//   - 仅一个 goroutine 成功
//   - balance_log 状态变为 settled
//
// 注：提现打款不涉及 balance 变动（提现申请时已扣余额），仅状态转换
func TestTenantPayWithdraw_Concurrent(t *testing.T) {
	deps := setupP0Deps(t, nil)
	const (
		tenantID   = uint64(5401)
		agentID    = uint64(6401)
		withdrawID = uint64(7401)
		logID      = uint64(8401)
		amount     = 100.00
		goroutine  = 10
	)
	seedP0Tenant(t, deps.DB, tenantID, "tenant_pay_withdraw", 0, 0)
	seedP0Agent(t, deps.DB, agentID, tenantID, "agent_pay_withdraw", "active", 0)
	seedP0Withdraw(t, deps.DB, withdrawID, agentID, tenantID, amount, "pending")
	seedP0WithdrawBalanceLog(t, deps.DB, logID, agentID, tenantID, withdrawID, amount, "pending")

	routePattern := "/tenant/withdrawals/:id/pay"
	requestPath := fmt.Sprintf("/tenant/withdrawals/%d/pay", withdrawID)
	body := map[string]interface{}{"pay_trade_no": "trade-7401"}

	var (
		wg         sync.WaitGroup
		successCnt int64
		failCnt    int64
	)
	wg.Add(goroutine)
	for i := 0; i < goroutine; i++ {
		go func() {
			defer wg.Done()
			w := callTenantEndpoint(t, deps, "POST", routePattern, requestPath, body, tenantID, TenantPayWithdraw(deps))
			if w.Code == http.StatusOK {
				code, _, _ := parseResp(t, w)
				if code == 0 {
					atomic.AddInt64(&successCnt, 1)
					return
				}
			}
			atomic.AddInt64(&failCnt, 1)
		}()
	}
	wg.Wait()

	assert.Equal(t, int64(1), atomic.LoadInt64(&successCnt), "应有且仅有一个 goroutine 打款成功")
	assert.Equal(t, int64(goroutine-1), atomic.LoadInt64(&failCnt), "其余 goroutine 应失败")

	// 断言：提现状态已变更为 paid
	var wd model.AgentWithdraw
	require.NoError(t, deps.DB.First(&wd, withdrawID).Error)
	assert.Equal(t, "paid", wd.Status, "提现状态应为 paid")
	require.NotNil(t, wd.PaidAt, "paid_at 应被设置")

	// 断言：balance_log 状态变为 settled
	var logRow model.AgentBalanceLog
	require.NoError(t, deps.DB.First(&logRow, logID).Error)
	assert.Equal(t, "settled", logRow.Status, "balance_log 状态应为 settled")
}

// ============== 2.5 支付回调并发测试 ==============

// TestProcessPaidOrder_Concurrent 10 个 goroutine 并发处理同一笔订单支付回调
// 断言：
//   - order.pay_status == "paid"
//   - 仅发卡 1 次（card_count == quantity，不是 quantity*10）
//   - 仅创建 1 个 platform_settlement 记录
//   - 所有 goroutine 最终返回 nil（首个完成实际发卡，其余幂等短路）
//
// 验证点：RowsAffected 检查让第二个及之后的事务跳过发卡/抽成写入
//
// 注：生产环境 EpayNotify 使用 Redis SETNX 锁串行化同一订单的回调（仅 1 个 goroutine
// 进入 processPaidOrder），RowsAffected 检查是第二道防线。本测试直连 processPaidOrder
// 跳过 Redis 锁以验证第二道防线；SQLite 共享缓存模式下并发写会触发
// "database table is locked"（SQLITE_LOCKED）瞬时错误，故对锁错误做有限重试，
// 模拟生产环境支付平台对回调失败的重试行为，最终所有调用应返回 nil（首个完成发卡，
// 其余 RowsAffected==0 幂等短路返回 nil）。
func TestProcessPaidOrder_Concurrent(t *testing.T) {
	deps := setupP0Deps(t, nil)
	const (
		tenantID   = uint64(5501)
		appID      = uint64(6501)
		cardTypeID = uint64(7501)
		orderID    = uint64(8501)
		pkgID      = uint64(9501)
		quantity   = 3
		goroutine  = 10
		amount     = 30.00
	)
	seedP0Package(t, deps.DB, pkgID, 5.00)
	tt := seedP0Tenant(t, deps.DB, tenantID, "tenant_ppo_concurrent", 0, 0)
	tt.PackageID = pkgID
	require.NoError(t, deps.DB.Save(tt).Error)
	seedP0App(t, deps.DB, appID, tenantID, "app_ppo_concurrent")
	seedP0CardType(t, deps.DB, cardTypeID, tenantID, appID, 10.00)
	seedP0Order(t, deps.DB, orderID, tenantID, appID, cardTypeID, "ORD-8501", amount, quantity, "pending")

	notify := &epay.NotifyParams{
		OutTradeNo: "ORD-8501",
		TradeNo:    "epay-trade-8501",
		Money:      "30.00",
	}

	var (
		wg            sync.WaitGroup
		nilErrCnt     int64
		nonNilErrCnt  int64
		errMsgMu      sync.Mutex
		errMsgs       []string
	)
	wg.Add(goroutine)
	for i := 0; i < goroutine; i++ {
		go func() {
			defer wg.Done()
			// 对 SQLite "database table is locked"（SQLITE_LOCKED）做有限重试，
			// 模拟生产环境支付平台对回调失败的重试；非锁类错误不重试
			const maxRetries = 30
			var lastErr error
			for retry := 0; retry < maxRetries; retry++ {
				err := processPaidOrder(deps, notify)
				if err == nil {
					atomic.AddInt64(&nilErrCnt, 1)
					return
				}
				lastErr = err
				if strings.Contains(err.Error(), "database table is locked") ||
					strings.Contains(err.Error(), "database is locked") {
					time.Sleep(20 * time.Millisecond)
					continue
				}
				// 非锁类业务错误：直接计入失败
				atomic.AddInt64(&nonNilErrCnt, 1)
				errMsgMu.Lock()
				errMsgs = append(errMsgs, err.Error())
				errMsgMu.Unlock()
				return
			}
			// 重试上限耗尽仍为锁错误：计为业务失败（不应发生）
			atomic.AddInt64(&nonNilErrCnt, 1)
			errMsgMu.Lock()
			errMsgs = append(errMsgs, "重试上限耗尽: "+lastErr.Error())
			errMsgMu.Unlock()
		}()
	}
	wg.Wait()

	// 调试：打印所有非锁类错误消息（仅在存在错误时）
	if len(errMsgs) > 0 {
		t.Logf("processPaidOrder 并发非锁类错误详情（共 %d 条）:", len(errMsgs))
		for i, m := range errMsgs {
			t.Logf("  [%d] %s", i+1, m)
		}
	}

	// 注：processPaidOrder 在 RowsAffected==0 时返回 nil（幂等成功，跳过发卡/抽成），
	// 故所有 goroutine 均返回 nil。需要通过副作用（card 数量 / settlement 数量）验证仅 1 次实际写入。
	assert.Equal(t, int64(goroutine), atomic.LoadInt64(&nilErrCnt), "幂等返回 nil 应为全部")
	assert.Equal(t, int64(0), atomic.LoadInt64(&nonNilErrCnt), "不应有非锁类错误")

	// 断言：order 已 paid
	var order model.AppOrder
	require.NoError(t, deps.DB.First(&order, orderID).Error)
	assert.Equal(t, "paid", order.PayStatus, "订单状态应为 paid")

	// 断言：发卡数量 == quantity（不是 quantity*goroutine）
	var cardCount int64
	require.NoError(t, deps.DB.Model(&model.AppCard{}).Where("order_id = ?", orderID).Count(&cardCount).Error)
	assert.Equal(t, int64(quantity), cardCount, "应仅发卡 %d 张（不是 %d）", quantity, quantity*goroutine)

	// 断言：仅 1 个 platform_settlement 记录
	var settlementCount int64
	require.NoError(t, deps.DB.Model(&model.PlatformSettlement{}).Where("order_id = ?", orderID).Count(&settlementCount).Error)
	assert.Equal(t, int64(1), settlementCount, "应仅创建 1 条 settlement 记录")
}

// ============== 2.6 支付回调幂等测试 ==============

// TestProcessPaidOrder_Idempotent 已 paid 的订单重复回调应返回 nil 且无副作用
// 断言：
//   - 返回 nil
//   - 不重复发卡（card 数量不变）
//   - 不重复创建 settlement（数量不变）
func TestProcessPaidOrder_Idempotent(t *testing.T) {
	deps := setupP0Deps(t, nil)
	const (
		tenantID   = uint64(5601)
		appID      = uint64(6601)
		cardTypeID = uint64(7601)
		orderID    = uint64(8601)
		pkgID      = uint64(9601)
		quantity   = 2
		amount     = 20.00
	)
	seedP0Package(t, deps.DB, pkgID, 5.00)
	tt := seedP0Tenant(t, deps.DB, tenantID, "tenant_ppo_idempotent", 0, 0)
	tt.PackageID = pkgID
	require.NoError(t, deps.DB.Save(tt).Error)
	seedP0App(t, deps.DB, appID, tenantID, "app_ppo_idempotent")
	seedP0CardType(t, deps.DB, cardTypeID, tenantID, appID, 10.00)
	// 订单已 paid
	seedP0Order(t, deps.DB, orderID, tenantID, appID, cardTypeID, "ORD-8601", amount, quantity, "paid")

	// 预先存在的副作用（理论上 0，因为订单初始即为 paid，未走过 processPaidOrder）
	var cardCountBefore int64
	require.NoError(t, deps.DB.Model(&model.AppCard{}).Where("order_id = ?", orderID).Count(&cardCountBefore).Error)
	var settlementCountBefore int64
	require.NoError(t, deps.DB.Model(&model.PlatformSettlement{}).Where("order_id = ?", orderID).Count(&settlementCountBefore).Error)

	notify := &epay.NotifyParams{
		OutTradeNo: "ORD-8601",
		TradeNo:    "epay-trade-8601",
		Money:      "20.00",
	}
	err := processPaidOrder(deps, notify)
	assert.NoError(t, err, "已 paid 订单重复回调应返回 nil（幂等）")

	// 断言：无新发卡
	var cardCountAfter int64
	require.NoError(t, deps.DB.Model(&model.AppCard{}).Where("order_id = ?", orderID).Count(&cardCountAfter).Error)
	assert.Equal(t, cardCountBefore, cardCountAfter, "幂等回调不应发新卡")

	// 断言：无新 settlement
	var settlementCountAfter int64
	require.NoError(t, deps.DB.Model(&model.PlatformSettlement{}).Where("order_id = ?", orderID).Count(&settlementCountAfter).Error)
	assert.Equal(t, settlementCountBefore, settlementCountAfter, "幂等回调不应创建新 settlement")

	// 断言：订单状态仍为 paid（未被破坏）
	var order model.AppOrder
	require.NoError(t, deps.DB.First(&order, orderID).Error)
	assert.Equal(t, "paid", order.PayStatus)
}

// ============== 2.7 开发者提现打款 frozen_balance 不足测试 ==============

// TestAdminPayTenantWithdraw_FrozenBalanceInsufficient frozen_balance 不足时事务回滚
// 断言：
//   - 返回错误（frozen_balance 不足）
//   - tenant.frozen_balance 仍为 50（未被强制设为 0）
//   - tenant_withdraw.status 仍为 pending（事务回滚，未变为 paid）
//
// 验证点：WHERE frozen_balance >= amount 守门 + RowsAffected 检查，
//        不足时事务失败回滚，禁止强制设为 0
func TestAdminPayTenantWithdraw_FrozenBalanceInsufficient(t *testing.T) {
	deps := setupP0Deps(t, nil)
	const (
		tenantID   = uint64(5701)
		withdrawID = uint64(7701)
		frozenBal  = 50.00
		amount     = 100.00 // > frozenBal，触发不足
	)
	seedP0Tenant(t, deps.DB, tenantID, "tenant_frozen_insufficient", 0, frozenBal)
	seedP0TenantWithdraw(t, deps.DB, withdrawID, tenantID, amount, "pending")

	routePattern := "/admin/tenant_withdrawals/:id/pay"
	requestPath := fmt.Sprintf("/admin/tenant_withdrawals/%d/pay", withdrawID)
	body := map[string]interface{}{"pay_trade_no": "trade-7701"}
	w := callAdminEndpoint(t, deps, "POST", routePattern, requestPath, body, 1, AdminPayTenantWithdraw(deps))

	// 事务失败 → middleware.Fail 返回 500
	assert.Equal(t, http.StatusInternalServerError, w.Code, "frozen_balance 不足应返回 500")
	code, msg, _ := parseResp(t, w)
	assert.NotEqual(t, 0, code)
	assert.Contains(t, msg, "冻结余额不足", "错误消息应明确指出 frozen_balance 不足")

	// 断言：tenant.frozen_balance 仍为 50（事务回滚，未被强制设为 0）
	var tenant model.SysTenant
	require.NoError(t, deps.DB.First(&tenant, tenantID).Error)
	assert.Equal(t, frozenBal, tenant.FrozenBalance, "frozen_balance 应保持原值 50（事务回滚）")

	// 断言：tenant_withdraw.status 仍为 pending（事务回滚，未变为 paid）
	var wd model.TenantWithdraw
	require.NoError(t, deps.DB.First(&wd, withdrawID).Error)
	assert.Equal(t, "pending", wd.Status, "提现状态应保持 pending（事务回滚）")
	require.Nil(t, wd.PaidAt, "paid_at 应为 nil")
}

// ============== 编译期断言：确保本测试文件不会随时间 drift ==============

// 引用 time 包，避免 import 被自动移除（占位）
var _ = time.Now
