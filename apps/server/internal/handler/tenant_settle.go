// 开发者结算与提现 Handler（v0.3.4 新增）
// 对称 agent_business.go / tenant_finance.go 的设计：
//   - TenantListSettlements：开发者查询自己的 platform_settlement 记录
//   - TenantListOwnWithdrawals：开发者查询自己的 tenant_withdraw 提现记录
//   - TenantListBalanceLogs：开发者查询自己的 tenant_balance_log 流水
//   - TenantWithdraw：开发者发起提现申请（事务：扣 balance → 加 frozen_balance → 写 withdraw → 写 balance_log）
// 严格遵循铁律 04/05/06
package handler

import (
	"context"
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

type tenantWithdrawReq struct {
	Amount     float64 `json:"amount" binding:"required,gt=0"`
	PayMethod  string  `json:"pay_method" binding:"required,oneof=alipay wechat bank"`
	PayAccount string  `json:"pay_account" binding:"required,min=1,max=128"`
	Remark     string  `json:"remark" binding:"omitempty,max=255"`
}

// ============== 1. 开发者查询自己的结算记录 ==============

// TenantListSettlements GET /api/v1/tenant/settlements
// 查询当前开发者的 platform_settlement 记录（按 status / order_no / 日期筛选）
func TenantListSettlements(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID := getTenantID(c)
		if tenantID == 0 {
			middleware.Fail(c, http.StatusForbidden, 1003, "无法识别租户身份")
			return
		}

		page, pageSize := parsePagination(c)

		q := deps.DB.Model(&model.PlatformSettlement{}).Where("tenant_id = ?", tenantID)
		if s := c.Query("status"); s != "" {
			q = q.Where("status = ?", s)
		}
		if orderNo := c.Query("order_no"); orderNo != "" {
			q = q.Where("order_no LIKE ?", "%"+orderNo+"%")
		}
		if startDate := c.Query("start_date"); startDate != "" {
			q = q.Where("created_at >= ?", startDate+" 00:00:00")
		}
		if endDate := c.Query("end_date"); endDate != "" {
			q = q.Where("created_at <= ?", endDate+" 23:59:59")
		}

		var total int64
		q.Count(&total)

		var list []model.PlatformSettlement
		if err := q.Order("id DESC").
			Offset((page - 1) * pageSize).Limit(pageSize).
			Find(&list).Error; err != nil {
			middleware.Fail(c, http.StatusInternalServerError, 5001, "查询结算记录失败")
			return
		}

		// 汇总：待结算金额 / 已结算金额
		var pendingSum, settledSum float64
		deps.DB.Model(&model.PlatformSettlement{}).Where("tenant_id = ? AND status = ?", tenantID, "pending").
			Select("COALESCE(SUM(net_amount), 0)").Scan(&pendingSum)
		deps.DB.Model(&model.PlatformSettlement{}).Where("tenant_id = ? AND status = ?", tenantID, "settled").
			Select("COALESCE(SUM(net_amount), 0)").Scan(&settledSum)

		middleware.Success(c, gin.H{
			"list":         list,
			"total":        total,
			"page":         page,
			"page_size":    pageSize,
			"pending_sum":  pendingSum,
			"settled_sum":  settledSum,
		})
	}
}

// ============== 2. 开发者查询自己的提现申请 ==============

// TenantListOwnWithdrawals GET /api/v1/tenant/withdrawals/mine
// 查询当前开发者发起的 tenant_withdraw 提现记录
func TenantListOwnWithdrawals(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID := getTenantID(c)
		if tenantID == 0 {
			middleware.Fail(c, http.StatusForbidden, 1003, "无法识别租户身份")
			return
		}

		page, pageSize := parsePagination(c)

		q := deps.DB.Model(&model.TenantWithdraw{}).Where("tenant_id = ?", tenantID)
		if s := c.Query("status"); s != "" {
			q = q.Where("status = ?", s)
		}
		if startDate := c.Query("start_date"); startDate != "" {
			q = q.Where("created_at >= ?", startDate+" 00:00:00")
		}
		if endDate := c.Query("end_date"); endDate != "" {
			q = q.Where("created_at <= ?", endDate+" 23:59:59")
		}

		var total int64
		q.Count(&total)

		var list []model.TenantWithdraw
		if err := q.Order("id DESC").
			Offset((page - 1) * pageSize).Limit(pageSize).
			Find(&list).Error; err != nil {
			middleware.Fail(c, http.StatusInternalServerError, 5001, "查询提现记录失败")
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

// ============== 3. 开发者查询自己的余额流水 ==============

// TenantListBalanceLogs GET /api/v1/tenant/balance_logs
// 查询当前开发者的 tenant_balance_log 流水
func TenantListBalanceLogs(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID := getTenantID(c)
		if tenantID == 0 {
			middleware.Fail(c, http.StatusForbidden, 1003, "无法识别租户身份")
			return
		}

		page, pageSize := parsePagination(c)

		q := deps.DB.Model(&model.TenantBalanceLog{}).Where("tenant_id = ?", tenantID)
		if t := c.Query("type"); t != "" {
			q = q.Where("type = ?", t)
		}
		if s := c.Query("status"); s != "" {
			q = q.Where("status = ?", s)
		}
		if startDate := c.Query("start_date"); startDate != "" {
			q = q.Where("created_at >= ?", startDate+" 00:00:00")
		}
		if endDate := c.Query("end_date"); endDate != "" {
			q = q.Where("created_at <= ?", endDate+" 23:59:59")
		}

		var total int64
		q.Count(&total)

		var list []model.TenantBalanceLog
		if err := q.Order("id DESC").
			Offset((page - 1) * pageSize).Limit(pageSize).
			Find(&list).Error; err != nil {
			middleware.Fail(c, http.StatusInternalServerError, 5001, "查询流水失败")
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

// ============== 4. 开发者发起提现 ==============

// TenantWithdraw POST /api/v1/tenant/withdraw
// 流程：校验余额/最小金额 → 事务扣 balance + 加 frozen_balance → 写 tenant_withdraw → 写 balance_log
func TenantWithdraw(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		tenantID := getTenantID(c)
		if tenantID == 0 {
			middleware.Fail(c, http.StatusForbidden, 1003, "无法识别租户身份")
			return
		}

		var req tenantWithdrawReq
		if err := c.ShouldBindJSON(&req); err != nil {
			middleware.Fail(c, http.StatusBadRequest, 1001, "参数错误: "+err.Error())
			return
		}

		// 校验最小提现金额（铁律 05：从 sys_config 读取，复用 pay.settlement.min_amount）
		minAmount := deps.CfgCache.GetFloat64(ctx, "pay.settlement.min_amount", 100.00)
		if req.Amount < minAmount {
			middleware.Fail(c, http.StatusBadRequest, 1001,
				"提现金额不能少于 "+strconv.FormatFloat(minAmount, 'f', 2, 64))
			return
		}

		// 事务：扣 balance + 加 frozen_balance + 写 withdraw + 写 balance_log
		var withdrawID uint64
		var balanceAfter float64
		txErr := deps.DB.Transaction(func(tx *gorm.DB) error {
			// 1. 查开发者并锁定
			var tenant model.SysTenant
			if err := tx.Set("gorm:query_option", "FOR UPDATE").First(&tenant, tenantID).Error; err != nil {
				return fmt.Errorf("查询开发者失败: %w", err)
			}
			if tenant.Status != "active" {
				return fmt.Errorf("开发者账号已被禁用")
			}
			if tenant.Balance < req.Amount {
				return fmt.Errorf("余额不足，当前可提现余额 %.2f", tenant.Balance)
			}

			// 2. 扣 balance + 加 frozen_balance
			newBalance := tenant.Balance - req.Amount
			newFrozen := tenant.FrozenBalance + req.Amount
			if err := tx.Model(&tenant).Updates(map[string]interface{}{
				"balance":         newBalance,
				"frozen_balance":  newFrozen,
			}).Error; err != nil {
				return fmt.Errorf("更新余额失败: %w", err)
			}
			balanceAfter = newBalance

			// 3. 写 tenant_withdraw
			wd := &model.TenantWithdraw{
				TenantID:   tenantID,
				Amount:     req.Amount,
				PayMethod:  req.PayMethod,
				PayAccount: req.PayAccount,
				Status:     "pending",
			}
			if err := tx.Create(wd).Error; err != nil {
				return fmt.Errorf("创建提现记录失败: %w", err)
			}
			withdrawID = wd.ID

			// 4. 写 tenant_balance_log（type='withdraw', status='pending', amount 为负）
			log := &model.TenantBalanceLog{
				TenantID:          tenantID,
				Type:              "withdraw",
				Amount:            -req.Amount,
				BalanceAfter:      newBalance,
				RelatedWithdrawID: &withdrawID,
				PayMethod:         req.PayMethod,
				Status:            "pending",
				Remark:            req.Remark,
			}
			if err := tx.Create(log).Error; err != nil {
				return fmt.Errorf("写入流水失败: %w", err)
			}
			return nil
		})
		if txErr != nil {
			middleware.Fail(c, http.StatusInternalServerError, 5003, "提现申请失败: "+txErr.Error())
			return
		}

		// 异步记录操作日志
		RecordOperation(deps, c, "finance", "tenant_withdraw", "success", "tenant_withdraw", &withdrawID, map[string]interface{}{
			"tenant_id":   tenantID,
			"amount":      req.Amount,
			"pay_method":  req.PayMethod,
			"pay_account": req.PayAccount,
		})

		middleware.Success(c, gin.H{
			"id":            withdrawID,
			"status":        "pending",
			"amount":        req.Amount,
			"balance_after": balanceAfter,
			"message":       "提现申请已提交，等待平台审核",
		})
	}
}

// ============== 5. 开发者余额概览 ==============

// TenantBalanceOverview GET /api/v1/tenant/balance_overview
// 返回开发者当前 balance / frozen_balance / 累计已结算 / 累计已提现
func TenantBalanceOverview(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		_ = context.Background()
		tenantID := getTenantID(c)
		if tenantID == 0 {
			middleware.Fail(c, http.StatusForbidden, 1003, "无法识别租户身份")
			return
		}

		var tenant model.SysTenant
		if err := deps.DB.Select("balance, frozen_balance").First(&tenant, tenantID).Error; err != nil {
			middleware.Fail(c, http.StatusNotFound, 4009, "开发者不存在")
			return
		}

		// 累计已结算净额（status=settled 的 platform_settlement.net_amount 求和）
		var settledTotal float64
		deps.DB.Model(&model.PlatformSettlement{}).
			Where("tenant_id = ? AND status = ?", tenantID, "settled").
			Select("COALESCE(SUM(net_amount), 0)").Scan(&settledTotal)

		// 累计已提现金额（status=paid 的 tenant_withdraw.amount 求和）
		var withdrawnTotal float64
		deps.DB.Model(&model.TenantWithdraw{}).
			Where("tenant_id = ? AND status = ?", tenantID, "paid").
			Select("COALESCE(SUM(amount), 0)").Scan(&withdrawnTotal)

		// 待审核提现金额（status=pending）
		var pendingWithdraw float64
		deps.DB.Model(&model.TenantWithdraw{}).
			Where("tenant_id = ? AND status = ?", tenantID, "pending").
			Select("COALESCE(SUM(amount), 0)").Scan(&pendingWithdraw)

		middleware.Success(c, gin.H{
			"balance":             tenant.Balance,
			"frozen_balance":      tenant.FrozenBalance,
			"settled_total":       settledTotal,
			"withdrawn_total":     withdrawnTotal,
			"pending_withdraw":    pendingWithdraw,
			"updated_at":          time.Now(),
		})
	}
}
