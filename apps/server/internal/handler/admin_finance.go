// 平台超管财务审核 Handler（v0.3.4 新增）
// 对称 tenant_finance.go 的设计：
//   - AdminListTenantWithdrawals：超管查询开发者提现申请列表（联表 sys_tenant 拿用户名）
//   - AdminPayTenantWithdraw：超管打款（事务：withdraw.status=paid + 退 frozen_balance + balance_log.status=settled）
//   - AdminRejectTenantWithdraw：超管驳回（事务：退 balance + 减 frozen_balance + withdraw.status=rejected + balance_log.status=rejected）
//   - AdminBatchSettle：超管批量结算 platform_settlement（事务：循环累加 tenant.balance + 写批次流水）
//   - AdminReconciliation：超管对账（按时间区间统计订单/抽成/应结/已结/未结）
// 严格遵循铁律 04/05/06：所有金额变动走事务
package handler

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/your-org/keyauth-saas/apps/server/internal/middleware"
	"github.com/your-org/keyauth-saas/apps/server/internal/model"
)

// ============== DTO ==============

// tenantWithdrawalItem 开发者提现申请列表项（联表 sys_tenant 拿用户名）
type tenantWithdrawalItem struct {
	model.TenantWithdraw
	TenantUsername string `json:"tenant_username"`
	TenantCompany  string `json:"tenant_company"`
}

type adminPayTenantWithdrawReq struct {
	PayTradeNo string `json:"pay_trade_no" binding:"omitempty,max=128"`
	Remark     string `json:"remark" binding:"omitempty,max=255"`
}

type adminRejectTenantWithdrawReq struct {
	Reason string `json:"reason" binding:"required,min=1,max=255"`
}

type adminBatchSettleReq struct {
	SettlementIDs []uint64 `json:"settlement_ids" binding:"required,min=1,max=100"`
	Method        string   `json:"method" binding:"omitempty,oneof=manual alipay wechat bank"`
	Remark        string   `json:"remark" binding:"omitempty,max=255"`
}

// ============== 1. 开发者提现申请列表 ==============

// AdminListTenantWithdrawals GET /api/v1/admin/tenant_withdrawals
// 查询所有开发者提现申请（默认 status=pending）
func AdminListTenantWithdrawals(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		page, pageSize := parsePagination(c)

		q := deps.DB.Table("tenant_withdraw AS w").
			Select("w.*, t.username AS tenant_username, t.company AS tenant_company").
			Joins("LEFT JOIN sys_tenant AS t ON t.id = w.tenant_id")

		if s := c.Query("status"); s != "" {
			q = q.Where("w.status = ?", s)
		} else {
			q = q.Where("w.status = ?", "pending")
		}
		if tenantIDStr := c.Query("tenant_id"); tenantIDStr != "" {
			if tenantID, err := strconv.ParseUint(tenantIDStr, 10, 64); err == nil && tenantID > 0 {
				q = q.Where("w.tenant_id = ?", tenantID)
			}
		}
		if kw := c.Query("keyword"); kw != "" {
			q = q.Where("t.username LIKE ? OR w.pay_account LIKE ? OR w.pay_trade_no LIKE ?",
				"%"+kw+"%", "%"+kw+"%", "%"+kw+"%")
		}
		if startDate := c.Query("start_date"); startDate != "" {
			q = q.Where("w.created_at >= ?", startDate+" 00:00:00")
		}
		if endDate := c.Query("end_date"); endDate != "" {
			q = q.Where("w.created_at <= ?", endDate+" 23:59:59")
		}

		var total int64
		q.Count(&total)

		var list []tenantWithdrawalItem
		if err := q.Order("w.id DESC").
			Offset((page - 1) * pageSize).Limit(pageSize).
			Scan(&list).Error; err != nil {
			middleware.Fail(c, http.StatusInternalServerError, 5001, "查询开发者提现失败")
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

// ============== 2. 超管打款 ==============

// AdminPayTenantWithdraw POST /api/v1/admin/tenant_withdrawals/:id/pay
// 事务：1) withdraw.status=paid + paid_at + pay_trade_no + audited_by
//       2) sys_tenant.frozen_balance -= amount
//       3) balance_log.status=settled
func AdminPayTenantWithdraw(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := parseUintParam(c, "id")
		if err != nil || id == 0 {
			middleware.Fail(c, http.StatusBadRequest, 1001, "提现 ID 参数错误")
			return
		}

		var req adminPayTenantWithdrawReq
		_ = c.ShouldBindJSON(&req)

		adminID := getUserID(c)

		var wd model.TenantWithdraw
		if err := deps.DB.First(&wd, id).Error; err != nil {
			middleware.Fail(c, http.StatusNotFound, 4009, "提现记录不存在")
			return
		}
		if wd.Status != "pending" {
			middleware.Fail(c, http.StatusBadRequest, 4011, "该提现申请当前状态不允许打款")
			return
		}

		now := time.Now()
		txErr := deps.DB.Transaction(func(tx *gorm.DB) error {
			// 1. 更新提现记录
			updates := map[string]interface{}{
				"status":       "paid",
				"paid_at":      now,
				"audited_by":   adminID,
				"audit_remark": req.Remark,
			}
			if req.PayTradeNo != "" {
				updates["pay_trade_no"] = req.PayTradeNo
			}
			if err := tx.Model(&wd).Updates(updates).Error; err != nil {
				return fmt.Errorf("更新提现记录失败: %w", err)
			}

			// 2. 扣减冻结余额（钱已实际打给开发者，从 frozen_balance 减去）
			var tenant model.SysTenant
			if err := tx.Set("gorm:query_option", "FOR UPDATE").First(&tenant, wd.TenantID).Error; err != nil {
				return fmt.Errorf("查询开发者失败: %w", err)
			}
			newFrozen := tenant.FrozenBalance - wd.Amount
			if newFrozen < 0 {
				newFrozen = 0 // 防御性兜底
			}
			if err := tx.Model(&tenant).Update("frozen_balance", newFrozen).Error; err != nil {
				return fmt.Errorf("更新冻结余额失败: %w", err)
			}

			// 3. 更新对应的 balance_log 状态为 settled
			if err := tx.Model(&model.TenantBalanceLog{}).
				Where("related_withdraw_id = ? AND tenant_id = ?", wd.ID, wd.TenantID).
				Updates(map[string]interface{}{
					"status": "settled",
				}).Error; err != nil {
				return fmt.Errorf("更新流水状态失败: %w", err)
			}
			return nil
		})
		if txErr != nil {
			middleware.Fail(c, http.StatusInternalServerError, 5002, "打款失败: "+txErr.Error())
			return
		}

		RecordOperation(deps, c, "finance", "pay_tenant_withdraw", "success", "tenant_withdraw", &wd.ID, map[string]interface{}{
			"tenant_id":    wd.TenantID,
			"amount":       wd.Amount,
			"pay_trade_no": req.PayTradeNo,
		})

		middleware.Success(c, gin.H{
			"id":           wd.ID,
			"status":       "paid",
			"paid_at":      now,
			"pay_trade_no": req.PayTradeNo,
		})
	}
}

// ============== 3. 超管驳回 ==============

// AdminRejectTenantWithdraw POST /api/v1/admin/tenant_withdrawals/:id/reject
// 事务：1) withdraw.status=rejected + audit_remark
//       2) sys_tenant.balance += amount 退回
//       3) sys_tenant.frozen_balance -= amount
//       4) balance_log.status=rejected
//       5) 写一条 refund 类型的 balance_log 记录退回
func AdminRejectTenantWithdraw(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := parseUintParam(c, "id")
		if err != nil || id == 0 {
			middleware.Fail(c, http.StatusBadRequest, 1001, "提现 ID 参数错误")
			return
		}

		var req adminRejectTenantWithdrawReq
		if err := c.ShouldBindJSON(&req); err != nil {
			middleware.Fail(c, http.StatusBadRequest, 1001, "驳回原因必填: "+err.Error())
			return
		}

		adminID := getUserID(c)

		var wd model.TenantWithdraw
		if err := deps.DB.First(&wd, id).Error; err != nil {
			middleware.Fail(c, http.StatusNotFound, 4009, "提现记录不存在")
			return
		}
		if wd.Status != "pending" {
			middleware.Fail(c, http.StatusBadRequest, 4011, "该提现申请当前状态不允许驳回")
			return
		}

		var balanceAfter float64
		txErr := deps.DB.Transaction(func(tx *gorm.DB) error {
			// 1. 更新提现记录
			if err := tx.Model(&wd).Updates(map[string]interface{}{
				"status":       "rejected",
				"audited_by":   adminID,
				"audit_remark": req.Reason,
			}).Error; err != nil {
				return fmt.Errorf("更新提现记录失败: %w", err)
			}

			// 2. 退回余额 + 减冻结
			var tenant model.SysTenant
			if err := tx.Set("gorm:query_option", "FOR UPDATE").First(&tenant, wd.TenantID).Error; err != nil {
				return fmt.Errorf("查询开发者失败: %w", err)
			}
			newBalance := tenant.Balance + wd.Amount
			newFrozen := tenant.FrozenBalance - wd.Amount
			if newFrozen < 0 {
				newFrozen = 0
			}
			if err := tx.Model(&tenant).Updates(map[string]interface{}{
				"balance":         newBalance,
				"frozen_balance":  newFrozen,
			}).Error; err != nil {
				return fmt.Errorf("退回余额失败: %w", err)
			}
			balanceAfter = newBalance

			// 3. 更新原 withdraw 流水状态为 rejected
			if err := tx.Model(&model.TenantBalanceLog{}).
				Where("related_withdraw_id = ? AND tenant_id = ? AND type = ?",
					wd.ID, wd.TenantID, "withdraw").
				Updates(map[string]interface{}{"status": "rejected"}).Error; err != nil {
				return fmt.Errorf("更新原流水状态失败: %w", err)
			}

			// 4. 写一条 refund 类型流水（金额为正）
			refundLog := &model.TenantBalanceLog{
				TenantID:          wd.TenantID,
				Type:              "refund",
				Amount:            wd.Amount,
				BalanceAfter:      newBalance,
				RelatedWithdrawID: &wd.ID,
				PayMethod:         wd.PayMethod,
				Status:            "settled",
				Remark:            "提现驳回退回: " + req.Reason,
			}
			if err := tx.Create(refundLog).Error; err != nil {
				return fmt.Errorf("写入退回流水失败: %w", err)
			}
			return nil
		})
		if txErr != nil {
			middleware.Fail(c, http.StatusInternalServerError, 5002, "驳回失败: "+txErr.Error())
			return
		}

		RecordOperation(deps, c, "finance", "reject_tenant_withdraw", "success", "tenant_withdraw", &wd.ID, map[string]interface{}{
			"tenant_id": wd.TenantID,
			"amount":    wd.Amount,
			"reason":    req.Reason,
		})

		middleware.Success(c, gin.H{
			"id":            wd.ID,
			"status":        "rejected",
			"balance_after": balanceAfter,
		})
	}
}

// ============== 4. 超管批量结算 ==============

// AdminBatchSettle POST /api/v1/admin/settlements/batch_settle
// 批量将多条 pending 的 platform_settlement 标记为 settled，并累加 tenant.balance
// 单次最多 100 条，事务保证原子性
func AdminBatchSettle(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req adminBatchSettleReq
		if err := c.ShouldBindJSON(&req); err != nil {
			middleware.Fail(c, http.StatusBadRequest, 1001, "参数错误: "+err.Error())
			return
		}
		if req.Method == "" {
			req.Method = "manual"
		}

		now := time.Now()
		batchNo := fmt.Sprintf("BSTL%s%06d", now.Format("20060102"), now.Unix()%1000000)

		// 按 tenant_id 分组累计，避免同一开发者多次更新
		type settleGroup struct {
			TenantID uint64
			TotalNet float64
			IDs      []uint64
		}

		var groups []settleGroup
		groupMap := make(map[uint64]*settleGroup)

		var balanceAfterMap = make(map[uint64]float64)
		var successCount int64

		txErr := deps.DB.Transaction(func(tx *gorm.DB) error {
			// 1. 逐条查询并锁定
			for _, sid := range req.SettlementIDs {
				var s model.PlatformSettlement
				if err := tx.Set("gorm:query_option", "FOR UPDATE").First(&s, sid).Error; err != nil {
					return fmt.Errorf("结算记录 %d 不存在", sid)
				}
				if s.Status == "settled" {
					return fmt.Errorf("结算记录 %d 已结算", sid)
				}

				// 更新结算记录
				if err := tx.Model(&s).Updates(map[string]interface{}{
					"status":          "settled",
					"settled_at":      now,
					"settle_batch_no": batchNo,
					"settle_method":   req.Method,
					"settle_remark":   req.Remark,
				}).Error; err != nil {
					return fmt.Errorf("更新结算记录 %d 失败: %w", sid, err)
				}

				// 分组累计
				if g, ok := groupMap[s.TenantID]; ok {
					g.TotalNet += s.NetAmount
					g.IDs = append(g.IDs, sid)
				} else {
					groupMap[s.TenantID] = &settleGroup{
						TenantID: s.TenantID,
						TotalNet: s.NetAmount,
						IDs:      []uint64{sid},
					}
				}
				successCount++
			}

			// 2. 按 tenant 累加 balance + 写流水
			for _, g := range groupMap {
				groups = append(groups, *g)

				var tenant model.SysTenant
				if err := tx.Set("gorm:query_option", "FOR UPDATE").First(&tenant, g.TenantID).Error; err != nil {
					return fmt.Errorf("查询开发者 %d 失败: %w", g.TenantID, err)
				}
				newBalance := tenant.Balance + g.TotalNet
				if err := tx.Model(&tenant).Update("balance", newBalance).Error; err != nil {
					return fmt.Errorf("更新开发者 %d 余额失败: %w", g.TenantID, err)
				}
				balanceAfterMap[g.TenantID] = newBalance

				// 写一条汇总流水（type=settle, amount=该批次累计净额）
				log := &model.TenantBalanceLog{
					TenantID:       g.TenantID,
					Type:           "settle",
					Amount:         g.TotalNet,
					BalanceAfter:   newBalance,
					PayMethod:      req.Method,
					SettleBatchNo:  batchNo,
					Status:         "settled",
					Remark:         fmt.Sprintf("批量结算 %d 笔: %s", len(g.IDs), req.Remark),
				}
				if err := tx.Create(log).Error; err != nil {
					return fmt.Errorf("写入开发者 %d 流水失败: %w", g.TenantID, err)
				}
			}
			return nil
		})
		if txErr != nil {
			middleware.Fail(c, http.StatusInternalServerError, 5002, "批量结算失败: "+txErr.Error())
			return
		}

		RecordOperation(deps, c, "settlement", "batch_settle", "success", "platform_settlement", nil, map[string]interface{}{
			"batch_no":    batchNo,
			"count":       successCount,
			"tenant_count": len(groups),
		})

		middleware.Success(c, gin.H{
			"batch_no":      batchNo,
			"success_count": successCount,
			"tenant_count":  len(groups),
			"balances":      balanceAfterMap,
			"settled_at":    now,
		})
	}
}

// ============== 5. 对账报表 ==============

// AdminReconciliation GET /api/v1/admin/reconciliation
// 按时间区间统计：订单总额 / 平台抽成 / 开发者应结 / 已结 / 未结 / 已提现
// 支持按 tenant_id 维度过滤
func AdminReconciliation(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		startDate := c.DefaultQuery("start_date", "")
		endDate := c.DefaultQuery("end_date", "")
		tenantIDStr := c.Query("tenant_id")

		// 默认最近 30 天
		if startDate == "" {
			startDate = time.Now().AddDate(0, 0, -30).Format("2006-01-02")
		}
		if endDate == "" {
			endDate = time.Now().Format("2006-01-02")
		}
		startTime := startDate + " 00:00:00"
		endTime := endDate + " 23:59:59"

		q := deps.DB.Model(&model.PlatformSettlement{}).
			Where("created_at >= ? AND created_at <= ?", startTime, endTime)
		if tenantIDStr != "" {
			if tenantID, err := strconv.ParseUint(tenantIDStr, 10, 64); err == nil && tenantID > 0 {
				q = q.Where("tenant_id = ?", tenantID)
			}
		}

		// 聚合统计
		var stats struct {
			OrderCount    int64   `json:"order_count"`
			GrossTotal    float64 `json:"gross_total"`
			CommissionSum float64 `json:"commission_sum"`
			NetTotal      float64 `json:"net_total"`
			SettledSum    float64 `json:"settled_sum"`
			PendingSum    float64 `json:"pending_sum"`
		}

		deps.DB.Model(&model.PlatformSettlement{}).
			Where("created_at >= ? AND created_at <= ?", startTime, endTime).
			Select(`COUNT(*) AS order_count,
				COALESCE(SUM(gross_amount), 0) AS gross_total,
				COALESCE(SUM(commission_amount), 0) AS commission_sum,
				COALESCE(SUM(net_amount), 0) AS net_total,
				COALESCE(SUM(CASE WHEN status = 'settled' THEN net_amount ELSE 0 END), 0) AS settled_sum,
				COALESCE(SUM(CASE WHEN status = 'pending' THEN net_amount ELSE 0 END), 0) AS pending_sum`).
			Scan(&stats)

		// 已提现金额（同一时间区间内 tenant_withdraw.status=paid 的 amount 求和）
		var withdrawnSum float64
		withdrawQ := deps.DB.Model(&model.TenantWithdraw{}).
			Where("created_at >= ? AND created_at <= ? AND status = ?", startTime, endTime, "paid")
		if tenantIDStr != "" {
			if tenantID, err := strconv.ParseUint(tenantIDStr, 10, 64); err == nil && tenantID > 0 {
				withdrawQ = withdrawQ.Where("tenant_id = ?", tenantID)
			}
		}
		withdrawQ.Select("COALESCE(SUM(amount), 0)").Scan(&withdrawnSum)

		// 待审核提现金额
		var pendingWithdrawSum float64
		pendingWQ := deps.DB.Model(&model.TenantWithdraw{}).
			Where("status = ?", "pending")
		if tenantIDStr != "" {
			if tenantID, err := strconv.ParseUint(tenantIDStr, 10, 64); err == nil && tenantID > 0 {
				pendingWQ = pendingWQ.Where("tenant_id = ?", tenantID)
			}
		}
		pendingWQ.Select("COALESCE(SUM(amount), 0)").Scan(&pendingWithdrawSum)

		middleware.Success(c, gin.H{
			"start_date":           startDate,
			"end_date":             endDate,
			"order_count":          stats.OrderCount,
			"gross_total":          stats.GrossTotal,
			"commission_sum":       stats.CommissionSum,
			"net_total":            stats.NetTotal,
			"settled_sum":          stats.SettledSum,
			"pending_sum":          stats.PendingSum,
			"withdrawn_sum":        withdrawnSum,
			"pending_withdraw_sum": pendingWithdrawSum,
			// 对账差：应结净额 - 已提现 = 开发者账户余额（理论值）
			"balance_theory":       stats.SettledSum - withdrawnSum,
		})
	}
}

// ============== v0.4.x 开发者月费订单管理 ==============

// monthlyFeeOrderItem 月费订单列表项（联表 sys_tenant 拿用户名）
type monthlyFeeOrderItem struct {
	model.TenantMonthlyFeeOrder
	TenantUsername string `json:"tenant_username"`
}

// AdminListMonthlyFeeOrders GET /admin/monthly_fee_orders
// 查询所有开发者月费订单（默认 status=pending）
func AdminListMonthlyFeeOrders(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		page, pageSize := parsePagination(c)

		q := deps.DB.Table("tenant_monthly_fee_order AS o").
			Select("o.*, t.username AS tenant_username").
			Joins("LEFT JOIN sys_tenant AS t ON t.id = o.tenant_id")

		if s := c.Query("pay_status"); s != "" {
			q = q.Where("o.pay_status = ?", s)
		} else {
			q = q.Where("o.pay_status = ?", "pending")
		}
		if tenantIDStr := c.Query("tenant_id"); tenantIDStr != "" {
			if tid, err := strconv.ParseUint(tenantIDStr, 10, 64); err == nil && tid > 0 {
				q = q.Where("o.tenant_id = ?", tid)
			}
		}
		if kw := c.Query("keyword"); kw != "" {
			q = q.Where("t.username LIKE ? OR o.order_no LIKE ?",
				"%"+kw+"%", "%"+kw+"%")
		}
		if startDate := c.Query("start_date"); startDate != "" {
			q = q.Where("o.period_start >= ?", startDate+" 00:00:00")
		}
		if endDate := c.Query("end_date"); endDate != "" {
			q = q.Where("o.period_end <= ?", endDate+" 23:59:59")
		}

		var total int64
		q.Count(&total)

		var list []monthlyFeeOrderItem
		if err := q.Order("o.id DESC").
			Offset((page - 1) * pageSize).Limit(pageSize).
			Scan(&list).Error; err != nil {
			middleware.Fail(c, http.StatusInternalServerError, 5001, "查询月费订单失败")
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

// AdminMonthlyFeeStats GET /admin/monthly_fee_orders/stats
// 月费订单统计（已收/未收/退款）
func AdminMonthlyFeeStats(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		q := deps.DB.Model(&model.TenantMonthlyFeeOrder{})
		if startDate := c.Query("start_date"); startDate != "" {
			q = q.Where("period_start >= ?", startDate+" 00:00:00")
		}
		if endDate := c.Query("end_date"); endDate != "" {
			q = q.Where("period_end <= ?", endDate+" 23:59:59")
		}

		var (
			totalOrders    int64
			pendingCount   int64
			paidCount      int64
			closedCount    int64
			pendingAmount  float64
			paidAmount     float64
		)
		q.Count(&totalOrders)

		deps.DB.Model(&model.TenantMonthlyFeeOrder{}).Where("pay_status = ?", "pending").
			Count(&pendingCount)
		deps.DB.Model(&model.TenantMonthlyFeeOrder{}).Where("pay_status = ?", "paid").
			Count(&paidCount)
		deps.DB.Model(&model.TenantMonthlyFeeOrder{}).Where("pay_status = ?", "closed").
			Count(&closedCount)

		deps.DB.Model(&model.TenantMonthlyFeeOrder{}).Where("pay_status = ?", "pending").
			Select("COALESCE(SUM(amount), 0)").Scan(&pendingAmount)
		deps.DB.Model(&model.TenantMonthlyFeeOrder{}).Where("pay_status = ?", "paid").
			Select("COALESCE(SUM(amount), 0)").Scan(&paidAmount)

		middleware.Success(c, gin.H{
			"total_orders":   totalOrders,
			"pending_count":  pendingCount,
			"paid_count":     paidCount,
			"closed_count":   closedCount,
			"pending_amount": pendingAmount,
			"paid_amount":    paidAmount,
		})
	}
}

// AdminMarkMonthlyFeePaid POST /admin/monthly_fee_orders/:id/mark_paid
// 超管手动标记月费订单已支付（线下对公转账场景）
// 铁律 06：事务内更新 pay_status + pay_mode=manual + paid_at
func AdminMarkMonthlyFeePaid(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		orderID, err := parseUintParam(c, "id")
		if err != nil || orderID == 0 {
			middleware.Fail(c, http.StatusBadRequest, 1001, "订单 ID 格式错误")
			return
		}

		var order model.TenantMonthlyFeeOrder
		if err := deps.DB.First(&order, orderID).Error; err != nil {
			middleware.Fail(c, http.StatusNotFound, 1008, "月费订单不存在")
			return
		}
		if order.PayStatus == "paid" {
			middleware.Fail(c, http.StatusBadRequest, 1009, "订单已支付")
			return
		}
		if order.PayStatus == "closed" {
			middleware.Fail(c, http.StatusBadRequest, 1009, "订单已关闭")
			return
		}

		adminID := getUserID(c)
		now := time.Now()
		txErr := deps.DB.Transaction(func(tx *gorm.DB) error {
			if err := tx.Model(&order).Updates(map[string]interface{}{
				"pay_status": "paid",
				"pay_mode":   "manual",
				"paid_at":    &now,
			}).Error; err != nil {
				return fmt.Errorf("更新月费订单状态失败: %w", err)
			}
			return nil
		})
		if txErr != nil {
			middleware.Fail(c, http.StatusInternalServerError, 5001, "标记支付失败: "+txErr.Error())
			return
		}

		RecordOperation(deps, c, "monthly_fee", "mark_paid", "success", "tenant_monthly_fee_order", &orderID, map[string]interface{}{
			"order_no":  order.OrderNo,
			"tenant_id": order.TenantID,
			"amount":    order.Amount,
			"admin_id":  adminID,
		})

		middleware.Success(c, gin.H{
			"order_id":   orderID,
			"pay_status": "paid",
			"pay_mode":   "manual",
			"paid_at":    now.Unix(),
		})
	}
}
