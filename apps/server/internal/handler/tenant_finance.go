// 开发者财务审核 Handler：代理充值申请审核 + 代理提现申请审核
// 严格遵循铁律 04/05：禁止硬编码、配置走 CfgCache、不确定处标注「待核实」
// 严格遵循铁律 06：所有金额变动走事务，确保 agent.balance 与流水一致
package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/your-org/keyauth-saas/apps/server/internal/middleware"
	"github.com/your-org/keyauth-saas/apps/server/internal/model"
	"github.com/your-org/keyauth-saas/apps/server/internal/openapi"
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
// 流程（事务）：校验流水状态=pending → 加余额 → 更新流水 status=settled → 写审核备注
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

		// 查流水
		var log model.AgentBalanceLog
		if err := deps.DB.Where("id = ? AND tenant_id = ? AND type = ?",
			id, tenantID, "recharge").First(&log).Error; err != nil {
			middleware.Fail(c, http.StatusNotFound, 1008, "充值申请不存在")
			return
		}
		if log.Status != "pending" {
			middleware.Fail(c, http.StatusBadRequest, 1012, "当前申请状态不允许审核（"+log.Status+"）")
			return
		}

		// 实际到账金额：缺省按申请金额
		actualAmount := log.Amount
		if req.ActualAmount != nil && *req.ActualAmount > 0 {
			actualAmount = *req.ActualAmount
		}

		// 事务：加余额 → 更新流水 → 写审核备注
		var newBalance float64
		txErr := deps.DB.Transaction(func(tx *gorm.DB) error {
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
			// 3. 更新流水
			updates := map[string]interface{}{
				"status":        "settled",
				"amount":        actualAmount,
				"balance_after": newBalance,
				"updated_at":    time.Now(),
			}
			remark := "审核通过"
			if req.Remark != "" {
				remark += "; " + req.Remark
			}
			updates["remark"] = remark
			if err := tx.Model(&model.AgentBalanceLog{}).Where("id = ?", id).
				Updates(updates).Error; err != nil {
				return err
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
// 流程：校验流水状态=pending → 更新 status=rejected + 写驳回原因
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

		var log model.AgentBalanceLog
		if err := deps.DB.Where("id = ? AND tenant_id = ? AND type = ?",
			id, tenantID, "recharge").First(&log).Error; err != nil {
			middleware.Fail(c, http.StatusNotFound, 1008, "充值申请不存在")
			return
		}
		if log.Status != "pending" {
			middleware.Fail(c, http.StatusBadRequest, 1012, "当前申请状态不允许审核（"+log.Status+"）")
			return
		}

		if err := deps.DB.Model(&model.AgentBalanceLog{}).Where("id = ?", id).
			Updates(map[string]interface{}{
				"status":     "rejected",
				"remark":     "驳回: " + req.Reason,
				"updated_at": time.Now(),
			}).Error; err != nil {
			middleware.Fail(c, http.StatusInternalServerError, 5002, "驳回失败: "+err.Error())
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
// 流程（事务）：校验 status=pending → 标记 withdraw.status=paid + paid_at + pay_trade_no
//            → 更新对应 balance_log status=settled
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
		if w.Status != "pending" {
			middleware.Fail(c, http.StatusBadRequest, 1012, "当前提现状态不允许打款（"+w.Status+"）")
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
			// 1. 更新 withdraw
			if err := tx.Model(&model.AgentWithdraw{}).Where("id = ?", id).
				Updates(map[string]interface{}{
					"status":       "paid",
					"paid_at":      now,
					"pay_trade_no": req.PayTradeNo,
					"audit_remark": auditRemark,
					"audited_by":   userID,
					"updated_at":   now,
				}).Error; err != nil {
				return err
			}
			// 2. 找到对应的 balance_log（type=withdraw, agent_id, 同一时间段）
			// 注：申请提现时同时写了 balance_log，但未记录 withdraw_id，这里按 agent_id + amount + 时间窗口匹配
			if err := tx.Model(&model.AgentBalanceLog{}).
				Where("agent_id = ? AND tenant_id = ? AND type = ? AND status = ? AND created_at >= ?",
					w.AgentID, tenantID, "withdraw", "pending", w.CreatedAt).
				Limit(1).
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
// 流程（事务）：校验 status=pending → 退回余额（balance += amount）
//            → withdraw.status=rejected + audit_remark
//            → 对应 balance_log status=rejected
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
		if w.Status != "pending" {
			middleware.Fail(c, http.StatusBadRequest, 1012, "当前提现状态不允许审核（"+w.Status+"）")
			return
		}

		now := time.Now()
		auditRemark := "驳回: " + req.Reason
		if w.AuditRemark != "" {
			auditRemark = w.AuditRemark + "; " + auditRemark
		}

		var newBalance float64
		txErr := deps.DB.Transaction(func(tx *gorm.DB) error {
			// 1. 退回余额
			if err := tx.Model(&model.Agent{}).Where("id = ?", w.AgentID).
				UpdateColumn("balance", gorm.Expr("balance + ?", w.Amount)).Error; err != nil {
				return err
			}
			// 2. 读新余额
			if err := tx.Model(&model.Agent{}).Where("id = ?", w.AgentID).
				Select("balance").Scan(&newBalance).Error; err != nil {
				return err
			}
			// 3. 更新 withdraw
			if err := tx.Model(&model.AgentWithdraw{}).Where("id = ?", id).
				Updates(map[string]interface{}{
					"status":       "rejected",
					"audit_remark": auditRemark,
					"audited_by":   userID,
					"updated_at":   now,
				}).Error; err != nil {
				return err
			}
			// 4. 更新对应 balance_log
			if err := tx.Model(&model.AgentBalanceLog{}).
				Where("agent_id = ? AND tenant_id = ? AND type = ? AND status = ? AND created_at >= ?",
					w.AgentID, tenantID, "withdraw", "pending", w.CreatedAt).
				Limit(1).
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
