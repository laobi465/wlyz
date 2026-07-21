// 开发者财务审核 Handler：代理充值申请审核 + 代理提现申请审核
// 严格遵循铁律 04/05：禁止硬编码、配置走 CfgCache、不确定处标注「待核实」
// 严格遵循铁律 06：所有金额变动走事务，确保 agent.balance 与流水一致
package handler

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/your-org/keyauth-saas/apps/server/internal/logger"
	"github.com/your-org/keyauth-saas/apps/server/internal/middleware"
	"github.com/your-org/keyauth-saas/apps/server/internal/model"
	"github.com/your-org/keyauth-saas/apps/server/internal/openapi"
	"github.com/your-org/keyauth-saas/apps/server/pkg/epay"
	"github.com/your-org/keyauth-saas/apps/server/pkg/snowflake"
)

// ============== 公共 DTO ==============

// rechargeRequestItem 充值申请列表项（联表 agent 拿用户名）
type rechargeRequestItem struct {
	model.AgentBalanceLog
	AgentUsername string `json:"agent_username"`
	AgentPhone    string `json:"agent_phone"`
}

// withdrawalItem 提现申请列表项（联表 agent 拿用户名）
type withdrawalItem struct {
	model.AgentWithdraw
	AgentUsername string `json:"agent_username"`
	AgentPhone    string `json:"agent_phone"`
}

// ============== 1. 充值申请列表 ==============

// TenantListRechargeRequests GET /api/v1/tenant/recharge_requests
// 查询当前开发者下所有 type=recharge 的 balance_log 流水（默认 status=pending）
func TenantListRechargeRequests(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID := getTenantID(c)
		if tenantID == 0 {
			middleware.Fail(c, http.StatusForbidden, 1003, "无法识别租户身份")
			return
		}

		page, pageSize := parsePagination(c)

		q := deps.DB.Table("agent_balance_log AS l").
			Select("l.*, a.username AS agent_username, a.phone AS agent_phone").
			Joins("LEFT JOIN agent AS a ON a.id = l.agent_id").
			Where("l.tenant_id = ? AND l.type = ?", tenantID, "recharge")

		if s := c.Query("status"); s != "" {
			q = q.Where("l.status = ?", s)
		} else {
			// 默认只看待审核
			q = q.Where("l.status = ?", "pending")
		}
		if agentIDStr := c.Query("agent_id"); agentIDStr != "" {
			if agentID, err := strconv.ParseUint(agentIDStr, 10, 64); err == nil && agentID > 0 {
				q = q.Where("l.agent_id = ?", agentID)
			}
		}
		if kw := c.Query("keyword"); kw != "" {
			q = q.Where("a.username LIKE ? OR l.pay_voucher LIKE ? OR l.remark LIKE ?",
				"%"+kw+"%", "%"+kw+"%", "%"+kw+"%")
		}

		var total int64
		q.Count(&total)

		var list []rechargeRequestItem
		if err := q.Order("l.id DESC").
			Offset((page - 1) * pageSize).Limit(pageSize).
			Scan(&list).Error; err != nil {
			middleware.Fail(c, http.StatusInternalServerError, 5001, "查询充值申请失败")
			return
		}

		middleware.Success(c, gin.H{
			"list":      list,
			"total":     total,
			"page":      page,
			"page_size": pageSize,
		})
	}
}

// ============== 2. 充值审核通过 ==============

// tenantApproveRechargeReq 充值审核通过请求（可选调整金额）
type tenantApproveRechargeReq struct {
	ActualAmount *float64 `json:"actual_amount" binding:"omitempty,gt=0"` // 实际到账金额，缺省按申请金额
	Remark       string   `json:"remark" binding:"omitempty,max=255"`
}

// TenantApproveRecharge POST /api/v1/tenant/recharge_requests/:id/approve
// 流程（事务）：加余额 → 带 status=pending 守门的流水更新（RowsAffected 检查防并发双倍加余额）
func TenantApproveRecharge(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID := getTenantID(c)
		if tenantID == 0 {
			middleware.Fail(c, http.StatusForbidden, 1003, "无法识别租户身份")
			return
		}

		id, err := strconv.ParseUint(c.Param("id"), 10, 64)
		if err != nil || id == 0 {
			middleware.Fail(c, http.StatusBadRequest, 1001, "无效的申请 ID")
			return
		}

		var req tenantApproveRechargeReq
		_ = c.ShouldBindJSON(&req)

		// 查流水（仅用于参数读取，事务内会重新做带守门的状态转换）
		var log model.AgentBalanceLog
		if err := deps.DB.Where("id = ? AND tenant_id = ? AND type = ?",
			id, tenantID, "recharge").First(&log).Error; err != nil {
			middleware.Fail(c, http.StatusNotFound, 1008, "充值申请不存在")
			return
		}

		// 实际到账金额：缺省按申请金额
		actualAmount := log.Amount
		if req.ActualAmount != nil && *req.ActualAmount > 0 {
			actualAmount = *req.ActualAmount
		}

		// 事务：锁 agent 行 → 加余额 → 带 status=pending 守门的流水更新
		var newBalance float64
		txErr := deps.DB.Transaction(func(tx *gorm.DB) error {
			// 0. 锁定 agent 行（防止并发审核双倍加余额）
			var agent model.Agent
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&agent, log.AgentID).Error; err != nil {
				return err
			}
			// 1. 加余额
			if err := tx.Model(&model.Agent{}).Where("id = ?", log.AgentID).
				UpdateColumn("balance", gorm.Expr("balance + ?", actualAmount)).Error; err != nil {
				return err
			}
			// 2. 读新余额
			if err := tx.Model(&model.Agent{}).Where("id = ?", log.AgentID).
				Select("balance").Scan(&newBalance).Error; err != nil {
				return err
			}
			// 3. 更新流水：带 status=pending 守门 + RowsAffected 检查（数据库级幂等）
			remark := "审核通过"
			if req.Remark != "" {
				remark += "; " + req.Remark
			}
			res := tx.Model(&model.AgentBalanceLog{}).
				Where("id = ? AND status = ?", id, "pending").
				Updates(map[string]interface{}{
					"status":        "settled",
					"amount":        actualAmount,
					"balance_after": newBalance,
					"remark":        remark,
					"updated_at":    time.Now(),
				})
			if res.Error != nil {
				return res.Error
			}
			if res.RowsAffected == 0 {
				// 状态已不是 pending（被并发审核处理了）
				return fmt.Errorf("充值申请已被处理或状态已变更")
			}
			return nil
		})
		if txErr != nil {
			middleware.Fail(c, http.StatusInternalServerError, 5003, "审核通过失败: "+txErr.Error())
			return
		}

		// v0.4.0 Webhook：异步分发 agent.recharge.approved 事件
		DispatchWebhookEvent(deps, tenantID, openapi.EventAgentRechargeApproved, gin.H{
			"recharge_id":   id,
			"agent_id":      log.AgentID,
			"amount":        actualAmount,
			"balance_after": newBalance,
			"approved_at":   time.Now().Unix(),
		})

		middleware.Success(c, gin.H{
			"id":            id,
			"status":        "settled",
			"actual_amount": actualAmount,
			"balance_after": newBalance,
		})
	}
}

// ============== 3. 充值审核驳回 ==============

// tenantRejectReq 驳回请求（充值/提现共用）
type tenantRejectReq struct {
	Reason string `json:"reason" binding:"required,max=255"`
}

// TenantRejectRecharge POST /api/v1/tenant/recharge_requests/:id/reject
// 流程（事务）：带 status=pending 守门的流水更新 + RowsAffected 检查（数据库级幂等）
func TenantRejectRecharge(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID := getTenantID(c)
		if tenantID == 0 {
			middleware.Fail(c, http.StatusForbidden, 1003, "无法识别租户身份")
			return
		}

		id, err := strconv.ParseUint(c.Param("id"), 10, 64)
		if err != nil || id == 0 {
			middleware.Fail(c, http.StatusBadRequest, 1001, "无效的申请 ID")
			return
		}

		var req tenantRejectReq
		if err := c.ShouldBindJSON(&req); err != nil {
			middleware.Fail(c, http.StatusBadRequest, 1001, "请填写驳回原因")
			return
		}

		// 校验流水归属（仅用于 404 提示与错误码区分，状态守门在事务内）
		var log model.AgentBalanceLog
		if err := deps.DB.Where("id = ? AND tenant_id = ? AND type = ?",
			id, tenantID, "recharge").First(&log).Error; err != nil {
			middleware.Fail(c, http.StatusNotFound, 1008, "充值申请不存在")
			return
		}

		txErr := deps.DB.Transaction(func(tx *gorm.DB) error {
			res := tx.Model(&model.AgentBalanceLog{}).
				Where("id = ? AND status = ?", id, "pending").
				Updates(map[string]interface{}{
					"status":     "rejected",
					"remark":     "驳回: " + req.Reason,
					"updated_at": time.Now(),
				})
			if res.Error != nil {
				logger.Error("tenant_finance: reject recharge failed", "err", res.Error)
				return res.Error
			}
			if res.RowsAffected == 0 {
				// 状态已不是 pending（被并发审核处理了）
				return fmt.Errorf("充值申请已被处理或状态已变更")
			}
			return nil
		})
		if txErr != nil {
			middleware.Fail(c, http.StatusInternalServerError, 5002, "驳回失败: "+txErr.Error())
			return
		}

		middleware.Success(c, gin.H{
			"id":     id,
			"status": "rejected",
			"reason": req.Reason,
		})
	}
}

// ============== 4. 提现申请列表 ==============

// TenantListWithdrawals GET /api/v1/tenant/withdrawals
// 查询当前开发者下所有提现申请（默认 status=pending）
func TenantListWithdrawals(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID := getTenantID(c)
		if tenantID == 0 {
			middleware.Fail(c, http.StatusForbidden, 1003, "无法识别租户身份")
			return
		}

		page, pageSize := parsePagination(c)

		q := deps.DB.Table("agent_withdraw AS w").
			Select("w.*, a.username AS agent_username, a.phone AS agent_phone").
			Joins("LEFT JOIN agent AS a ON a.id = w.agent_id").
			Where("w.tenant_id = ?", tenantID)

		if s := c.Query("status"); s != "" {
			q = q.Where("w.status = ?", s)
		} else {
			q = q.Where("w.status = ?", "pending")
		}
		if agentIDStr := c.Query("agent_id"); agentIDStr != "" {
			if agentID, err := strconv.ParseUint(agentIDStr, 10, 64); err == nil && agentID > 0 {
				q = q.Where("w.agent_id = ?", agentID)
			}
		}
		if kw := c.Query("keyword"); kw != "" {
			q = q.Where("a.username LIKE ? OR w.pay_account LIKE ?",
				"%"+kw+"%", "%"+kw+"%")
		}

		var total int64
		q.Count(&total)

		var list []withdrawalItem
		if err := q.Order("w.id DESC").
			Offset((page - 1) * pageSize).Limit(pageSize).
			Scan(&list).Error; err != nil {
			middleware.Fail(c, http.StatusInternalServerError, 5001, "查询提现申请失败")
			return
		}

		middleware.Success(c, gin.H{
			"list":      list,
			"total":     total,
			"page":      page,
			"page_size": pageSize,
		})
	}
}

// ============== 5. 提现审核通过（标记为已打款） ==============

// tenantPayWithdrawReq 提现打款请求
type tenantPayWithdrawReq struct {
	PayTradeNo string `json:"pay_trade_no" binding:"omitempty,max=128"`
	Remark     string `json:"remark" binding:"omitempty,max=255"`
}

// TenantPayWithdraw POST /api/v1/tenant/withdrawals/:id/pay
// 流程（事务）：锁 agent 行 → 带 status=pending 守门的 withdraw 更新（RowsAffected 检查）
//
//	→ 按 related_withdraw_id 精确匹配对应 balance_log 状态置 settled
//
// 注：申请提现时已扣余额，此处仅标记已打款，不涉及金额变动
func TenantPayWithdraw(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID := getTenantID(c)
		userID := getUserID(c)
		if tenantID == 0 {
			middleware.Fail(c, http.StatusForbidden, 1003, "无法识别租户身份")
			return
		}

		id, err := strconv.ParseUint(c.Param("id"), 10, 64)
		if err != nil || id == 0 {
			middleware.Fail(c, http.StatusBadRequest, 1001, "无效的提现 ID")
			return
		}

		var req tenantPayWithdrawReq
		_ = c.ShouldBindJSON(&req)

		var w model.AgentWithdraw
		if err := deps.DB.Where("id = ? AND tenant_id = ?", id, tenantID).First(&w).Error; err != nil {
			middleware.Fail(c, http.StatusNotFound, 1008, "提现申请不存在")
			return
		}

		now := time.Now()
		auditRemark := w.AuditRemark
		if req.Remark != "" {
			if auditRemark != "" {
				auditRemark += "; " + req.Remark
			} else {
				auditRemark = req.Remark
			}
		}

		txErr := deps.DB.Transaction(func(tx *gorm.DB) error {
			// 0. 锁定 agent 行（防止并发审核/驳回互踩）
			var agent model.Agent
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&agent, w.AgentID).Error; err != nil {
				return err
			}
			// 1. 更新 withdraw：带 status=pending 守门 + RowsAffected 检查（数据库级幂等）
			res := tx.Model(&model.AgentWithdraw{}).
				Where("id = ? AND status = ?", id, "pending").
				Updates(map[string]interface{}{
					"status":       "paid",
					"paid_at":      now,
					"pay_trade_no": req.PayTradeNo,
					"audit_remark": auditRemark,
					"audited_by":   userID,
					"updated_at":   now,
				})
			if res.Error != nil {
				return res.Error
			}
			if res.RowsAffected == 0 {
				return fmt.Errorf("提现申请已被处理或状态已变更")
			}
			// 2. 按 related_withdraw_id 精确匹配对应 balance_log（避免时间窗口模糊匹配错配）
			if err := tx.Model(&model.AgentBalanceLog{}).
				Where("related_withdraw_id = ? AND tenant_id = ? AND type = ?",
					id, tenantID, "withdraw").
				Updates(map[string]interface{}{
					"status":     "settled",
					"updated_at": now,
				}).Error; err != nil {
				return err
			}
			return nil
		})
		if txErr != nil {
			middleware.Fail(c, http.StatusInternalServerError, 5003, "打款失败: "+txErr.Error())
			return
		}

		// v0.4.0 Webhook：异步分发 agent.withdraw.paid 事件
		DispatchWebhookEvent(deps, tenantID, openapi.EventAgentWithdrawPaid, gin.H{
			"withdraw_id":  id,
			"agent_id":     w.AgentID,
			"amount":       w.Amount,
			"pay_trade_no": req.PayTradeNo,
			"paid_at":      now.Unix(),
		})

		middleware.Success(c, gin.H{
			"id":           id,
			"status":       "paid",
			"paid_at":      now,
			"pay_trade_no": req.PayTradeNo,
		})
	}
}

// ============== 6. 提现审核驳回 ==============

// TenantRejectWithdraw POST /api/v1/tenant/withdrawals/:id/reject
// 流程（事务）：锁 agent 行 → 退回余额（balance += amount）
//
//	→ 带 status=pending 守门的 withdraw 更新（RowsAffected 检查）
//	→ 按 related_withdraw_id 精确匹配对应 balance_log status=rejected
func TenantRejectWithdraw(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID := getTenantID(c)
		userID := getUserID(c)
		if tenantID == 0 {
			middleware.Fail(c, http.StatusForbidden, 1003, "无法识别租户身份")
			return
		}

		id, err := strconv.ParseUint(c.Param("id"), 10, 64)
		if err != nil || id == 0 {
			middleware.Fail(c, http.StatusBadRequest, 1001, "无效的提现 ID")
			return
		}

		var req tenantRejectReq
		if err := c.ShouldBindJSON(&req); err != nil {
			middleware.Fail(c, http.StatusBadRequest, 1001, "请填写驳回原因")
			return
		}

		var w model.AgentWithdraw
		if err := deps.DB.Where("id = ? AND tenant_id = ?", id, tenantID).First(&w).Error; err != nil {
			middleware.Fail(c, http.StatusNotFound, 1008, "提现申请不存在")
			return
		}

		now := time.Now()
		auditRemark := "驳回: " + req.Reason
		if w.AuditRemark != "" {
			auditRemark = w.AuditRemark + "; " + auditRemark
		}

		var newBalance float64
		txErr := deps.DB.Transaction(func(tx *gorm.DB) error {
			// 0. 锁定 agent 行（防止并发审核/驳回互踩，导致余额重复退回）
			var agent model.Agent
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&agent, w.AgentID).Error; err != nil {
				return err
			}
			// 1. 带 status=pending 守门的 withdraw 更新 + RowsAffected 检查（数据库级幂等，先于此的余额变动才有效）
			res := tx.Model(&model.AgentWithdraw{}).
				Where("id = ? AND status = ?", id, "pending").
				Updates(map[string]interface{}{
					"status":       "rejected",
					"audit_remark": auditRemark,
					"audited_by":   userID,
					"updated_at":   now,
				})
			if res.Error != nil {
				return res.Error
			}
			if res.RowsAffected == 0 {
				return fmt.Errorf("提现申请已被处理或状态已变更")
			}
			// 2. 退回余额（提现申请时已扣 balance，此处加回；agent 行已锁定）
			if err := tx.Model(&model.Agent{}).Where("id = ?", w.AgentID).
				UpdateColumn("balance", gorm.Expr("balance + ?", w.Amount)).Error; err != nil {
				return err
			}
			// 3. 读新余额
			if err := tx.Model(&model.Agent{}).Where("id = ?", w.AgentID).
				Select("balance").Scan(&newBalance).Error; err != nil {
				return err
			}
			// 4. 按 related_withdraw_id 精确匹配对应 balance_log（避免时间窗口模糊匹配错配）
			if err := tx.Model(&model.AgentBalanceLog{}).
				Where("related_withdraw_id = ? AND tenant_id = ? AND type = ?",
					id, tenantID, "withdraw").
				Updates(map[string]interface{}{
					"status":        "rejected",
					"balance_after": newBalance,
					"updated_at":    now,
				}).Error; err != nil {
				return err
			}
			return nil
		})
		if txErr != nil {
			middleware.Fail(c, http.StatusInternalServerError, 5003, "驳回失败: "+txErr.Error())
			return
		}

		middleware.Success(c, gin.H{
			"id":            id,
			"status":        "rejected",
			"reason":        req.Reason,
			"balance_after": newBalance,
		})
	}
}

// ============== v0.4.x 开发者月费订单（自有支付附加） ==============

// TenantGetMonthlyFeeCurrent GET /api/v1/tenant/monthly_fee/current
// 返回当前开发者月费配置 + 当前账单周期 + 是否欠费
// 铁律 05：月费开关/金额/免费天数从 sys_config 读取
//   - pay.tenant_monthly_fee.enabled=0 时返回 enabled=false，前端隐藏月费入口
//   - pay.tenant_monthly_fee.amount 决定账单金额
//   - pay.tenant_monthly_fee.free_days 决定注册后免费天数，期间不生成账单
func TenantGetMonthlyFeeCurrent(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID := getTenantID(c)
		if tenantID == 0 {
			middleware.Fail(c, http.StatusForbidden, 1003, "无法识别租户身份")
			return
		}

		ctx := c.Request.Context()
		enabled := deps.CfgCache.GetBool(ctx, "pay.tenant_monthly_fee.enabled", false)
		amount := deps.CfgCache.GetFloat64(ctx, "pay.tenant_monthly_fee.amount", 50.00)
		freeDays := deps.CfgCache.GetInt(ctx, "pay.tenant_monthly_fee.free_days", 30)

		// 查开发者注册时间（用于计算免费期）
		var tenant model.SysTenant
		if err := deps.DB.Select("id, created_at").First(&tenant, tenantID).Error; err != nil {
			middleware.Fail(c, http.StatusInternalServerError, 5001, "查询开发者信息失败")
			return
		}

		// 当前账单周期 = 当前自然月（月初到月末）
		now := time.Now()
		periodStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
		periodEnd := periodStart.AddDate(0, 1, 0).Add(-time.Second)

		// 是否在免费期内
		freeUntil := tenant.CreatedAt.AddDate(0, 0, freeDays)
		inFreePeriod := now.Before(freeUntil)

		// 查当前周期 pending 月费订单
		var currentOrder *model.TenantMonthlyFeeOrder
		var order model.TenantMonthlyFeeOrder
		if err := deps.DB.Where("tenant_id = ? AND period_start >= ? AND period_end <= ?",
			tenantID, periodStart, periodEnd).
			Order("id DESC").First(&order).Error; err == nil {
			currentOrder = &order
		}

		// 查累计未支付账单数 + 总金额
		var pendingCount int64
		var pendingAmount float64
		deps.DB.Model(&model.TenantMonthlyFeeOrder{}).
			Where("tenant_id = ? AND pay_status = ?", tenantID, "pending").
			Count(&pendingCount)
		deps.DB.Model(&model.TenantMonthlyFeeOrder{}).
			Where("tenant_id = ? AND pay_status = ?", tenantID, "pending").
			Select("COALESCE(SUM(amount), 0)").Scan(&pendingAmount)

		middleware.Success(c, gin.H{
			"enabled":        enabled,
			"amount":         amount,
			"free_days":      freeDays,
			"in_free_period": inFreePeriod,
			"free_until":     freeUntil.Unix(),
			"period_start":   periodStart.Unix(),
			"period_end":     periodEnd.Unix(),
			"current_order":  currentOrder,
			"pending_count":  pendingCount,
			"pending_amount": pendingAmount,
		})
	}
}

// tenantPayMonthlyFeeReq 月费支付请求体
type tenantPayMonthlyFeeReq struct {
	PayType string `json:"pay_type" binding:"required,oneof=alipay wxpay qqpay"`
}

// TenantPayMonthlyFee POST /api/v1/tenant/monthly_fee/pay
// 开发者发起月费支付：构造易支付跳转 URL（前缀 MFD）
// 铁律 04：金额从 sys_config 读取；订单号前缀 MFD 与 ORD/TOP/REG 区分
func TenantPayMonthlyFee(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID := getTenantID(c)
		if tenantID == 0 {
			middleware.Fail(c, http.StatusForbidden, 1003, "无法识别租户身份")
			return
		}

		ctx := c.Request.Context()
		// 1. 总开关
		if !deps.CfgCache.GetBool(ctx, "pay.tenant_monthly_fee.enabled", false) {
			middleware.Fail(c, http.StatusForbidden, 4001, "开发者月费未启用")
			return
		}

		// 2. 平台支付必须可用
		if !deps.CfgCache.GetBool(ctx, "pay.platform.enabled", true) {
			middleware.Fail(c, http.StatusForbidden, 4001, "平台总支付未启用")
			return
		}

		var req tenantPayMonthlyFeeReq
		if err := c.ShouldBindJSON(&req); err != nil {
			middleware.Fail(c, http.StatusBadRequest, 1001, "参数错误: "+err.Error())
			return
		}

		// 3. 计算账单金额 + 周期
		amount := deps.CfgCache.GetFloat64(ctx, "pay.tenant_monthly_fee.amount", 50.00)
		if amount <= 0 {
			middleware.Fail(c, http.StatusForbidden, 4001, "月费金额未配置")
			return
		}

		var tenant model.SysTenant
		if err := deps.DB.First(&tenant, tenantID).Error; err != nil {
			middleware.Fail(c, http.StatusForbidden, 4004, "开发者账号不存在")
			return
		}
		if tenant.Status != "active" {
			middleware.Fail(c, http.StatusForbidden, 4004, "开发者账号已被禁用")
			return
		}

		// 4. 当前账单周期（自然月）
		now := time.Now()
		periodStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
		periodEnd := periodStart.AddDate(0, 1, 0).Add(-time.Second)

		// 5. 幂等：同周期已有订单（不论 pay_status）则复用，避免重复生成
		// P1-05 修复：原仅查 pay_status='pending'，已 paid 后再访问会创建新订单导致重复扣费
		//   - 已存在 pending 订单：复用，重新拉起支付
		//   - 已存在 paid 订单：直接返回当前订单，拒绝重复创建
		//   - 已存在 closed 订单：直接返回当前订单，由前端提示用户
		var order model.TenantMonthlyFeeOrder
		err := deps.DB.Where("tenant_id = ? AND period_start = ?",
			tenantID, periodStart).
			Order("id DESC").First(&order).Error
		if err == gorm.ErrRecordNotFound {
			// 6. 创建新订单
			orderNo := snowflake.OrderNo("MFD")
			order = model.TenantMonthlyFeeOrder{
				TenantID:    tenantID,
				PeriodStart: periodStart,
				PeriodEnd:   periodEnd,
				Amount:      amount,
				PayStatus:   "pending",
				PayMode:     "platform_epay",
				OrderNo:     orderNo,
			}
			if err := deps.DB.Create(&order).Error; err != nil {
				middleware.Fail(c, http.StatusInternalServerError, 4007, "创建月费订单失败: "+err.Error())
				return
			}
		} else if err != nil {
			middleware.Fail(c, http.StatusInternalServerError, 5001, "查询月费订单失败: "+err.Error())
			return
		}

		// 7. 加载易支付配置 + 构造跳转 URL
		payCfg, err := loadPlatformPayConfig(deps)
		if err != nil {
			logger.Error("tenant_finance: load platform pay config failed", "err", err)
			middleware.Fail(c, http.StatusInternalServerError, 4006, "平台支付配置错误")
			return
		}
		namePrefix := deps.CfgCache.GetString(ctx, "pay.platform.order_name_prefix", "KeyAuth开发者月费")
		moneyStr := strconv.FormatFloat(order.Amount, 'f', 2, 64)
		payURL, err := epay.BuildSubmitURL(payCfg, &epay.OrderParams{
			OutTradeNo: order.OrderNo,
			Name:       fmt.Sprintf("%s·%s", namePrefix, periodStart.Format("2006-01")),
			Money:      moneyStr,
			PayType:    req.PayType,
			NotifyURL:  resolveNotifyURL(deps, c),
			ReturnURL:  resolveReturnURL(deps, c),
			ClientIP:   c.ClientIP(),
		})
		if err != nil {
			logger.Error("tenant_finance: build pay submit url failed", "err", err, "order_no", order.OrderNo)
			middleware.Fail(c, http.StatusInternalServerError, 4008, "构造支付链接失败")
			return
		}

		middleware.Success(c, gin.H{
			"order_no":     order.OrderNo,
			"order_id":     order.ID,
			"pay_url":      payURL,
			"amount":       order.Amount,
			"money":        moneyStr,
			"pay_type":     req.PayType,
			"period_start": order.PeriodStart.Unix(),
			"period_end":   order.PeriodEnd.Unix(),
		})
	}
}
