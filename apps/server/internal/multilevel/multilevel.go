// Package multilevel 多级代理核心逻辑（v0.4.0）
//
// 提供跨级佣金计算与代理树查询两个核心能力，所有可变参数（跨级佣金比例 / 最大层级）
// 通过 sys_config 注入，严格遵循铁律 05（配置后台化）。
//
// 设计要点：
//   - 最大支持 3 级代理（level 1/2/3），level > max_level 时禁止注册下级
//   - 跨级佣金仅沿 parent_id 链向上传递，不走 InviterID（开发者维度）
//   - 跨级佣金类型在 AgentBalanceLog.Type 中新增 "cross_commission"
//   - 所有比例从 sys_config 读取，默认值仅作 fallback（cfgCache 未命中时使用）
package multilevel

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"

	"github.com/your-org/keyauth-saas/apps/server/internal/config"
	"github.com/your-org/keyauth-saas/apps/server/internal/model"
)

// ErrAgentNotFound 代理不存在
var ErrAgentNotFound = errors.New("agent not found")

// ErrLevelExceedsMax 超出最大层级
var ErrLevelExceedsMax = errors.New("agent level exceeds max_level configured")

// CrossCommissionResult 单笔跨级佣金结算结果
type CrossCommissionResult struct {
	AgentID      uint64  // 接收跨级佣金的上级代理 ID
	Level        int     // 接收方代理层级
	Amount       float64 // 跨级佣金金额
	Rate         float64 // 应用的跨级佣金比例（百分比）
	SourceAgentID uint64 // 产生佣金的下级代理 ID
}

// DistributeCrossCommission 在代理产生佣金后，沿 parent_id 链向上分发跨级佣金
//
// 调用场景：AgentGenerateCards 计算出当前代理（agent）的实时佣金 commission 后调用
//
// 规则：
//   - 当前代理 level=1：无上级，不分发
//   - 当前代理 level=2：父级（level=1）获得 commission * cross_level_2_rate / 100
//   - 当前代理 level=3：父级（level=2）获得 commission * cross_level_2_rate / 100
//                          祖父级（level=1）获得 commission * cross_level_3_rate / 100
//   - 父级代理状态必须为 active，否则跳过（不向已禁用代理发放）
//   - 所有跨级佣金在事务内同时更新父级 balance + 写 AgentBalanceLog{Type:"cross_commission"}
//
// 返回值：每笔已结算的跨级佣金明细（用于响应字段 / 审计）
//
// 注意：调用方必须已开启 gorm 事务，本函数不自行 Begin/Commit
func DistributeCrossCommission(
	ctx context.Context,
	tx *gorm.DB,
	cfgCache *config.ConfigCache,
	agent *model.Agent,
	commission float64,
	relatedCardIDsJSON string,
) ([]CrossCommissionResult, error) {
	if agent == nil || agent.ParentID == 0 || commission <= 0 {
		return nil, nil
	}

	maxLevel := int(cfgCache.GetInt(ctx, "agent.commission.max_level", 3))
	if maxLevel < 1 {
		maxLevel = 1
	}
	crossLevel2Rate := cfgCache.GetFloat64(ctx, "agent.commission.cross_level_2_rate", 50.00)
	crossLevel3Rate := cfgCache.GetFloat64(ctx, "agent.commission.cross_level_3_rate", 20.00)

	var results []CrossCommissionResult

	// 向上遍历 parent 链，最多 2 层（level 3 → level 2 → level 1）
	current := agent
	for depth := 0; depth < 2 && current.ParentID != 0; depth++ {
		var parent model.Agent
		if err := tx.Where("id = ?", current.ParentID).First(&parent).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				// 父级代理已被物理删除，停止向上分发
				break
			}
			return results, fmt.Errorf("查询父级代理失败: %w", err)
		}

		// 父级代理状态非 active 时跳过（不向已禁用代理发放跨级佣金）
		if parent.Status != "active" {
			break
		}

		// 根据源代理层级（agent.Level）和向上深度选择比例：
		//   - 源 level=2 → 父级（level=1）使用 cross_level_2_rate
		//   - 源 level=3 → 父级（level=2）使用 cross_level_2_rate，祖父级（level=1）使用 cross_level_3_rate
		// 注意：以 agent.Level 为准，不能用 current.Level（current 会随 depth 变化）
		var rate float64
		if agent.Level == 2 {
			rate = crossLevel2Rate
		} else if agent.Level == 3 {
			if depth == 0 {
				rate = crossLevel2Rate
			} else {
				rate = crossLevel3Rate
			}
		} else {
			// 超出设计层级的代理（理论不应出现），停止向上
			break
		}

		amount := commission * rate / 100
		if amount <= 0 {
			current = &parent
			continue
		}

		// 1. 更新父级 balance（gorm.Expr 避免并发覆盖）
		if err := tx.Model(&model.Agent{}).
			Where("id = ?", parent.ID).
			UpdateColumn("balance", gorm.Expr("balance + ?", amount)).Error; err != nil {
			return results, fmt.Errorf("更新父级代理余额失败: %w", err)
		}

		// 2. 重新读取父级最新 balance 作为 BalanceAfter
		var updatedParent model.Agent
		if err := tx.Where("id = ?", parent.ID).First(&updatedParent).Error; err != nil {
			return results, fmt.Errorf("重新查询父级代理失败: %w", err)
		}

		// 3. 写跨级佣金流水
		log := &model.AgentBalanceLog{
			AgentID:        parent.ID,
			TenantID:       parent.TenantID,
			Type:           "cross_commission",
			Amount:         amount,
			BalanceAfter:   updatedParent.Balance,
			RelatedCardIDs: relatedCardIDsJSON,
			Status:         "settled",
			Remark:         fmt.Sprintf("下级代理 #%d 跨级佣金（层级 %d → %d）", agent.ID, agent.Level, parent.Level),
		}
		if err := tx.Create(log).Error; err != nil {
			return results, fmt.Errorf("写入跨级佣金流水失败: %w", err)
		}

		results = append(results, CrossCommissionResult{
			AgentID:       parent.ID,
			Level:         parent.Level,
			Amount:        amount,
			Rate:          rate,
			SourceAgentID: agent.ID,
		})

		current = &parent
		_ = maxLevel // max_level 在 CanCreateSubordinate 中使用，此处保留以备扩展
	}

	return results, nil
}

// CanCreateSubordinate 校验当前代理是否可以创建下级代理
//
// 规则：
//   - sys_config agent.invite_code.agent_can_create 必须为 true
//   - 当前代理 level 必须 < max_level
//   - 当前代理状态必须为 active
func CanCreateSubordinate(ctx context.Context, cfgCache *config.ConfigCache, agent *model.Agent) error {
	if agent == nil {
		return ErrAgentNotFound
	}
	if agent.Status != "active" {
		return fmt.Errorf("代理状态非 active，无法创建下级")
	}
	canCreate := cfgCache.GetBool(ctx, "agent.invite_code.agent_can_create", true)
	if !canCreate {
		return fmt.Errorf("系统未开启代理创建下级邀请码（agent.invite_code.agent_can_create=false）")
	}
	maxLevel := int(cfgCache.GetInt(ctx, "agent.commission.max_level", 3))
	if maxLevel < 1 {
		maxLevel = 1
	}
	if agent.Level >= maxLevel {
		return ErrLevelExceedsMax
	}
	return nil
}

// ComputeSubordinateLevel 计算下级代理的层级
//
// 调用方在 processAgentRegisterPaid 创建 Agent 时使用：
//   - 邀请码 creator_type='tenant' → 新代理 level=1, parent_id=0
//   - 邀请码 creator_type='agent'  → 新代理 level=creator.level+1, parent_id=creator.id
//
// 返回 (parentID, level, error)
func ComputeSubordinateLevel(ctx context.Context, db *gorm.DB, cfgCache *config.ConfigCache, ic *model.AgentInviteCode) (uint64, int, error) {
	if ic == nil {
		return 0, 1, ErrAgentNotFound
	}
	if ic.CreatorType != "agent" || ic.CreatorAgentID == 0 {
		// 开发者邀请码 → 一级代理
		return 0, 1, nil
	}

	// 代理邀请码 → 查上级代理
	var creator model.Agent
	if err := db.Where("id = ?", ic.CreatorAgentID).First(&creator).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, 0, fmt.Errorf("邀请码创建者代理 #%d 不存在", ic.CreatorAgentID)
		}
		return 0, 0, fmt.Errorf("查询邀请码创建者失败: %w", err)
	}

	if err := CanCreateSubordinate(ctx, cfgCache, &creator); err != nil {
		return 0, 0, err
	}

	return creator.ID, creator.Level + 1, nil
}

// AgentTreeNode 代理树节点
type AgentTreeNode struct {
	Agent     model.Agent   `json:"agent"`
	Children  []AgentTreeNode `json:"children,omitempty"`
}

// BuildAgentTree 构建代理下级树（递归，最多 maxLevel 层）
//
// 参数：
//   - rootAgentID：根代理 ID
//   - maxDepth：最大递归深度（通常等于 max_level - 1，例如 max_level=3 时根 level=1 可下钻 2 层）
//
// 返回树形结构，已包含每层代理的完整字段
func BuildAgentTree(ctx context.Context, db *gorm.DB, rootAgentID uint64, maxDepth int) (*AgentTreeNode, error) {
	if maxDepth < 0 {
		maxDepth = 0
	}
	var root model.Agent
	if err := db.Where("id = ?", rootAgentID).First(&root).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrAgentNotFound
		}
		return nil, fmt.Errorf("查询根代理失败: %w", err)
	}
	node := &AgentTreeNode{Agent: root}
	if maxDepth == 0 {
		return node, nil
	}
	if err := buildAgentTreeChildren(ctx, db, node, maxDepth); err != nil {
		return nil, err
	}
	return node, nil
}

// buildAgentTreeChildren 递归填充 children
func buildAgentTreeChildren(ctx context.Context, db *gorm.DB, parent *AgentTreeNode, remainingDepth int) error {
	if remainingDepth <= 0 {
		return nil
	}
	var children []model.Agent
	if err := db.Where("parent_id = ? AND tenant_id = ?", parent.Agent.ID, parent.Agent.TenantID).
		Order("created_at ASC").Find(&children).Error; err != nil {
		return fmt.Errorf("查询下级代理失败: %w", err)
	}
	for _, child := range children {
		childNode := &AgentTreeNode{Agent: child}
		if err := buildAgentTreeChildren(ctx, db, childNode, remainingDepth-1); err != nil {
			return err
		}
		parent.Children = append(parent.Children, *childNode)
	}
	return nil
}

// ListSubordinates 列出直接下级代理（单层，不递归）
//
// 用于代理控制台"我的下级"页面：返回 parent_id = agentID 的所有代理
func ListSubordinates(ctx context.Context, db *gorm.DB, agentID, tenantID uint64) ([]model.Agent, error) {
	var agents []model.Agent
	if err := db.Where("parent_id = ? AND tenant_id = ?", agentID, tenantID).
		Order("created_at ASC").Find(&agents).Error; err != nil {
		return nil, fmt.Errorf("查询下级代理失败: %w", err)
	}
	return agents, nil
}
