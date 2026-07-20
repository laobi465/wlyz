// 平台超管业务接口 Handler
// 包含工作台、开发者管理、套餐管理、代理管理、公告管理、日志审计、安全防护
// 严格遵循铁律 04/05：禁止硬编码、配置走 CfgCache
// 严格遵循铁律 06：不确定处标注「待核实」
package handler

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/your-org/keyauth-saas/apps/server/internal/middleware"
	"github.com/your-org/keyauth-saas/apps/server/internal/model"
	"github.com/your-org/keyauth-saas/apps/server/internal/multilevel"
	"github.com/your-org/keyauth-saas/apps/server/pkg/crypto"
)

// ============== DTO ==============

type adminCreateTenantReq struct {
	Username   string `json:"username" binding:"required,min=3,max=64"`
	Password   string `json:"password" binding:"required,min=8,max=64"`
	Email      string `json:"email" binding:"omitempty,email,max=128"`
	Phone      string `json:"phone" binding:"omitempty,max=32"`
	Company    string `json:"company" binding:"omitempty,max=128"`
	PackageID  uint64 `json:"package_id" binding:"omitempty"`
	ExpireDays int    `json:"expire_days" binding:"omitempty,min=0,max=3650"`
	Remark     string `json:"remark" binding:"omitempty,max=255"`
}

type adminUpdateTenantReq struct {
	Status     *string `json:"status" binding:"omitempty,oneof=active disabled pending suspended deleted"`
	PackageID  *uint64 `json:"package_id" binding:"omitempty"`
	ExpireDays *int    `json:"expire_days" binding:"omitempty,min=0,max=3650"`
	Password   *string `json:"password" binding:"omitempty,min=8,max=64"`
	Remark     *string `json:"remark" binding:"omitempty,max=255"`
}

type adminCreatePackageReq struct {
	Name         string  `json:"name" binding:"required,min=1,max=64"`
	Description  string  `json:"description" binding:"omitempty,max=255"`
	MaxApps      int     `json:"max_apps" binding:"omitempty,min=0"`
	MaxCards     int     `json:"max_cards" binding:"omitempty,min=0"`
	MaxAgents    int     `json:"max_agents" binding:"omitempty,min=0"`
	PriceMonthly float64 `json:"price_monthly" binding:"omitempty,min=0"`
	PriceYearly  float64 `json:"price_yearly" binding:"omitempty,min=0"`
	Features     string  `json:"features" binding:"omitempty,max=2000"`
	Status       string  `json:"status" binding:"omitempty,oneof=active disabled"`
}

type adminCreateNoticeReq struct {
	Type      string     `json:"type" binding:"required,oneof=platform tenant agent"`
	Title     string     `json:"title" binding:"required,min=1,max=255"`
	Content   string     `json:"content" binding:"required,min=1"`
	Status    string     `json:"status" binding:"omitempty,oneof=draft published archived"`
	Pinned    bool       `json:"pinned"`
	Sort      int        `json:"sort" binding:"omitempty,min=0"`
	PublishAt *time.Time `json:"publish_at"`
	ExpireAt  *time.Time `json:"expire_at"`
}

type adminUpdateNoticeReq struct {
	Type      *string    `json:"type" binding:"omitempty,oneof=platform tenant agent"`
	Title     *string    `json:"title" binding:"omitempty,min=1,max=255"`
	Content   *string    `json:"content" binding:"omitempty,min=1"`
	Status    *string    `json:"status" binding:"omitempty,oneof=draft published archived"`
	Pinned    *bool      `json:"pinned"`
	Sort      *int       `json:"sort" binding:"omitempty,min=0"`
	PublishAt *time.Time `json:"publish_at"`
	ExpireAt  *time.Time `json:"expire_at"`
}

type adminUpdateAgentReq struct {
	Status         *string  `json:"status" binding:"omitempty,oneof=active disabled pending"`
	CommissionMode *string  `json:"commission_mode" binding:"omitempty,oneof=percentage diff"`
	CommissionRate *float64 `json:"commission_rate" binding:"omitempty,min=0,max=100"`
	Balance        *float64 `json:"balance" binding:"omitempty"`
}

type adminAddIPBlacklistReq struct {
	IP          string `json:"ip" binding:"required,max=45"`
	Reason      string `json:"reason" binding:"omitempty,max=255"`
	ExpireHours int    `json:"expire_hours" binding:"omitempty,min=0,max=87600"` // 0=永久
}

// ============== 公开公告 ==============

// uint64Ptr 工具：将 uint64 转为 *uint64
func uint64Ptr(v uint64) *uint64 {
	return &v
}

// PublicPlatformNotices 公开平台公告（无需鉴权）
// GET /api/v1/public/notices/platform
func PublicPlatformNotices(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		var list []model.Notice
		now := time.Now()
		if err := deps.DB.Where("type = ? AND status = ? AND start_at <= ? AND (end_at IS NULL OR end_at > ?)",
			"platform", "published", now, now).
			Order("is_pinned DESC, start_at DESC").
			Find(&list).Error; err != nil {
			middleware.Fail(c, http.StatusInternalServerError, 5001, "查询公告失败")
			return
		}
		middleware.Success(c, gin.H{"list": list})
	}
}

// ============== 1. 平台看板 ==============

// AdminDashboard 平台看板（S-01）
// GET /admin/dashboard
// 一次返回全部统计 + 最近 10 个开发者 + 最近 10 个订单 + 7 日收入趋势
func AdminDashboard(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		// 统计开发者
		var tenantTotal, tenantActive int64
		deps.DB.Model(&model.SysTenant{}).Count(&tenantTotal)
		deps.DB.Model(&model.SysTenant{}).Where("status = ?", "active").Count(&tenantActive)

		// 统计代理
		var agentTotal, agentActive int64
		deps.DB.Model(&model.Agent{}).Count(&agentTotal)
		deps.DB.Model(&model.Agent{}).Where("status = ?", "active").Count(&agentActive)

		// 统计应用 / 卡密
		var appTotal, cardTotal, cardActive int64
		deps.DB.Model(&model.App{}).Count(&appTotal)
		deps.DB.Model(&model.AppCard{}).Count(&cardTotal)
		deps.DB.Model(&model.AppCard{}).Where("status = ?", "active").Count(&cardActive)

		// 今日订单 / 收入
		var orderToday int64
		var revenueToday, revenueMonth float64
		startOfToday := time.Date(time.Now().Year(), time.Now().Month(), time.Now().Day(), 0, 0, 0, 0, time.Local)
		deps.DB.Model(&model.AppOrder{}).Where("pay_status = ? AND paid_at >= ?", "paid", startOfToday).Count(&orderToday)
		deps.DB.Model(&model.AppOrder{}).Where("pay_status = ? AND paid_at >= ?", "paid", startOfToday).
			Select("COALESCE(SUM(total_amount), 0)").Scan(&revenueToday)
		startOfMonth := time.Date(time.Now().Year(), time.Now().Month(), 1, 0, 0, 0, 0, time.Local)
		deps.DB.Model(&model.AppOrder{}).Where("pay_status = ? AND paid_at >= ?", "paid", startOfMonth).
			Select("COALESCE(SUM(total_amount), 0)").Scan(&revenueMonth)

		// 结算待处理
		var settlementPending int64
		var settlementAmount float64
		deps.DB.Model(&model.PlatformSettlement{}).Where("status = ?", "pending").Count(&settlementPending)
		deps.DB.Model(&model.PlatformSettlement{}).Where("status = ?", "pending").
			Select("COALESCE(SUM(net_amount), 0)").Scan(&settlementAmount)

		// 最近 10 个开发者
		var recentTenants []model.SysTenant
		deps.DB.Select("id, username, status, created_at").Order("id DESC").Limit(10).Find(&recentTenants)
		recentTenantList := make([]gin.H, 0, len(recentTenants))
		for _, t := range recentTenants {
			recentTenantList = append(recentTenantList, gin.H{
				"id":         t.ID,
				"username":   t.Username,
				"status":     t.Status,
				"created_at": t.CreatedAt,
			})
		}

		// 最近 10 个订单
		var recentOrders []model.AppOrder
		deps.DB.Select("id, order_no, total_amount, pay_status, created_at").Order("id DESC").Limit(10).Find(&recentOrders)
		recentOrderList := make([]gin.H, 0, len(recentOrders))
		for _, o := range recentOrders {
			recentOrderList = append(recentOrderList, gin.H{
				"id":         o.ID,
				"order_no":   o.OrderNo,
				"amount":     o.TotalAmount,
				"status":     o.PayStatus,
				"created_at": o.CreatedAt,
			})
		}

		// 7 日收入趋势
		revenueTrend := make([]gin.H, 0, 7)
		for i := 6; i >= 0; i-- {
			dayStart := startOfToday.AddDate(0, 0, -i)
			dayEnd := dayStart.AddDate(0, 0, 1)
			var dayRevenue float64
			deps.DB.Model(&model.AppOrder{}).
				Where("pay_status = ? AND paid_at >= ? AND paid_at < ?", "paid", dayStart, dayEnd).
				Select("COALESCE(SUM(total_amount), 0)").Scan(&dayRevenue)
			revenueTrend = append(revenueTrend, gin.H{
				"date":   dayStart.Format("2006-01-02"),
				"amount": dayRevenue,
			})
		}

		_ = ctx

		middleware.Success(c, gin.H{
			"tenant_total":        tenantTotal,
			"tenant_active":       tenantActive,
			"agent_total":         agentTotal,
			"agent_active":        agentActive,
			"app_total":           appTotal,
			"card_total":          cardTotal,
			"card_active":         cardActive,
			"order_today":         orderToday,
			"revenue_today":       revenueToday,
			"revenue_month":       revenueMonth,
			"settlement_pending":  settlementPending,
			"settlement_amount":   settlementAmount,
			"recent_tenants":      recentTenantList,
			"recent_orders":       recentOrderList,
			"revenue_trend":       revenueTrend,
		})
	}
}

// ============== 2. 开发者管理 ==============

// AdminListTenants 开发者列表（S-02）
// GET /admin/tenants?page=&page_size=&keyword=&status=
func AdminListTenants(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		page, pageSize := parsePagination(c)

		q := deps.DB.Model(&model.SysTenant{})
		if kw := c.Query("keyword"); kw != "" {
			q = q.Where("username LIKE ? OR email LIKE ? OR company LIKE ?", "%"+kw+"%", "%"+kw+"%", "%"+kw+"%")
		}
		if status := c.Query("status"); status != "" {
			q = q.Where("status = ?", status)
		}

		var total int64
		q.Count(&total)

		var tenants []model.SysTenant
		if err := q.Order("id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&tenants).Error; err != nil {
			middleware.Fail(c, http.StatusInternalServerError, 5001, "查询失败: "+err.Error())
			return
		}

		// 联表查询套餐名 + 统计 app_count / card_count
		list := make([]gin.H, 0, len(tenants))
		for _, t := range tenants {
			var pkgName string
			deps.DB.Model(&model.SysPackage{}).Where("id = ?", t.PackageID).Select("name").Scan(&pkgName)

			var appCount, cardCount int64
			deps.DB.Model(&model.App{}).Where("tenant_id = ?", t.ID).Count(&appCount)
			deps.DB.Model(&model.AppCard{}).Where("tenant_id = ?", t.ID).Count(&cardCount)

			// 开发者结算余额 = 已结算的 net_amount 累计
			var settledBalance float64
			deps.DB.Model(&model.PlatformSettlement{}).
				Where("tenant_id = ? AND status = ?", t.ID, "settled").
				Select("COALESCE(SUM(net_amount), 0)").Scan(&settledBalance)

			list = append(list, gin.H{
				"id":           t.ID,
				"username":     t.Username,
				"email":        t.Email,
				"phone":        t.Phone,
				"company":      t.Company,
				"status":       t.Status,
				"package_id":   t.PackageID,
				"package_name": pkgName,
				"app_count":    appCount,
				"card_count":   cardCount,
				"balance":      settledBalance,
				"created_at":   t.CreatedAt,
				"expired_at":   t.ExpiresAt,
				"remark":       t.Remark,
				"last_login_at": t.LastLoginAt,
			})
		}

		middleware.Success(c, gin.H{
			"list":      list,
			"total":     total,
			"page":      page,
			"page_size": pageSize,
		})
	}
}

// AdminCreateTenant 创建开发者
// POST /admin/tenants
func AdminCreateTenant(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req adminCreateTenantReq
		if err := c.ShouldBindJSON(&req); err != nil {
			middleware.Fail(c, http.StatusBadRequest, 1001, "参数错误: "+err.Error())
			return
		}

		// 用户名唯一性
		var count int64
		if err := deps.DB.Model(&model.SysTenant{}).Where("username = ?", req.Username).Count(&count).Error; err != nil {
			middleware.Fail(c, http.StatusInternalServerError, 5001, "查询失败")
			return
		}
		if count > 0 {
			middleware.Fail(c, http.StatusConflict, 1011, "用户名已被使用")
			return
		}

		// 默认套餐
		if req.PackageID == 0 {
			middleware.Fail(c, http.StatusBadRequest, 1001, "package_id 必填")
			return
		}
		var pkg model.SysPackage
		if err := deps.DB.First(&pkg, req.PackageID).Error; err != nil {
			middleware.Fail(c, http.StatusBadRequest, 1008, "套餐不存在")
			return
		}

		// 密码加密
		passwordHash, err := crypto.HashPassword(req.Password)
		if err != nil {
			middleware.Fail(c, http.StatusInternalServerError, 5002, "密码加密失败")
			return
		}

		// 到期时间
		var expiresAt *time.Time
		if req.ExpireDays > 0 {
			t := time.Now().AddDate(0, 0, req.ExpireDays)
			expiresAt = &t
		}

		tenant := &model.SysTenant{
			TenantCode:   genTenantCode(),
			Username:     req.Username,
			PasswordHash: passwordHash,
			Email:        req.Email,
			Phone:        req.Phone,
			Company:      req.Company,
			Status:       "active",
			PackageID:    req.PackageID,
			ExpiresAt:    expiresAt,
			Remark:       req.Remark,
		}
		if err := deps.DB.Create(tenant).Error; err != nil {
			middleware.Fail(c, http.StatusInternalServerError, 5003, "创建失败: "+err.Error())
			return
		}

		middleware.Success(c, gin.H{
			"id":         tenant.ID,
			"username":   tenant.Username,
			"package_id": tenant.PackageID,
			"expires_at": tenant.ExpiresAt,
		})
	}
}

// AdminUpdateTenant 更新开发者
// PUT /admin/tenants/:id
func AdminUpdateTenant(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseUint(c.Param("id"), 10, 64)
		if err != nil {
			middleware.Fail(c, http.StatusBadRequest, 1001, "ID 格式错误")
			return
		}

		var req adminUpdateTenantReq
		if err := c.ShouldBindJSON(&req); err != nil {
			middleware.Fail(c, http.StatusBadRequest, 1001, "参数错误: "+err.Error())
			return
		}

		var tenant model.SysTenant
		if err := deps.DB.First(&tenant, id).Error; err != nil {
			middleware.Fail(c, http.StatusNotFound, 1008, "开发者不存在")
			return
		}

		updates := make(map[string]interface{})
		if req.Status != nil {
			updates["status"] = *req.Status
		}
		if req.PackageID != nil {
			updates["package_id"] = *req.PackageID
		}
		if req.Password != nil {
			hash, err := crypto.HashPassword(*req.Password)
			if err != nil {
				middleware.Fail(c, http.StatusInternalServerError, 5001, "密码加密失败")
				return
			}
			updates["password_hash"] = hash
		}
		// expire_days > 0 时延长：从 max(expires_at, now) 开始加
		if req.ExpireDays != nil && *req.ExpireDays > 0 {
			base := time.Now()
			if tenant.ExpiresAt != nil && tenant.ExpiresAt.After(base) {
				base = *tenant.ExpiresAt
			}
			newExpire := base.AddDate(0, 0, *req.ExpireDays)
			updates["expires_at"] = newExpire
		}
		if req.Remark != nil {
			updates["remark"] = *req.Remark
		}

		if len(updates) == 0 {
			middleware.Fail(c, http.StatusBadRequest, 1001, "未提交任何更新字段")
			return
		}

		if err := deps.DB.Model(&model.SysTenant{}).Where("id = ?", id).Updates(updates).Error; err != nil {
			middleware.Fail(c, http.StatusInternalServerError, 5002, "更新失败: "+err.Error())
			return
		}

		middleware.Success(c, gin.H{"id": id, "updated": true})
	}
}

// ============== 3. 套餐管理 ==============

// AdminListPackages 套餐列表（S-03）
// GET /admin/packages?page=&page_size=&keyword=
func AdminListPackages(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		page, pageSize := parsePagination(c)

		q := deps.DB.Model(&model.SysPackage{})
		if kw := c.Query("keyword"); kw != "" {
			q = q.Where("name LIKE ?", "%"+kw+"%")
		}

		var total int64
		q.Count(&total)

		var packages []model.SysPackage
		if err := q.Order("id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&packages).Error; err != nil {
			middleware.Fail(c, http.StatusInternalServerError, 5001, "查询失败")
			return
		}

		// 字段映射：MonthlyPrice→price_monthly, YearlyPrice→price_yearly
		list := make([]gin.H, 0, len(packages))
		for _, p := range packages {
			list = append(list, gin.H{
				"id":            p.ID,
				"name":          p.Name,
				"description":   p.Description,
				"max_apps":      p.MaxApps,
				"max_cards":     p.MaxCards,
				"max_agents":    p.MaxAgents,
				"price_monthly": p.MonthlyPrice,
				"price_yearly":  p.YearlyPrice,
				"features":      p.Features,
				"status":        p.Status,
				"created_at":    p.CreatedAt,
			})
		}

		middleware.Success(c, gin.H{
			"list":      list,
			"total":     total,
			"page":      page,
			"page_size": pageSize,
		})
	}
}

// AdminCreatePackage 创建套餐
// POST /admin/packages
func AdminCreatePackage(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req adminCreatePackageReq
		if err := c.ShouldBindJSON(&req); err != nil {
			middleware.Fail(c, http.StatusBadRequest, 1001, "参数错误: "+err.Error())
			return
		}

		status := req.Status
		if status == "" {
			status = "active"
		}

		pkg := &model.SysPackage{
			Name:         req.Name,
			Description:  req.Description,
			MonthlyPrice: req.PriceMonthly,
			YearlyPrice:  req.PriceYearly,
			MaxApps:      req.MaxApps,
			MaxCards:     req.MaxCards,
			MaxAgents:    req.MaxAgents,
			Features:     req.Features,
			Status:       status,
		}
		if err := deps.DB.Create(pkg).Error; err != nil {
			middleware.Fail(c, http.StatusInternalServerError, 5001, "创建失败: "+err.Error())
			return
		}

		middleware.Success(c, gin.H{
			"id":            pkg.ID,
			"name":          pkg.Name,
			"price_monthly": pkg.MonthlyPrice,
			"price_yearly":  pkg.YearlyPrice,
			"status":        pkg.Status,
		})
	}
}

// AdminUpdatePackage 更新套餐（新增）
// PUT /admin/packages/:id
func AdminUpdatePackage(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseUint(c.Param("id"), 10, 64)
		if err != nil {
			middleware.Fail(c, http.StatusBadRequest, 1001, "ID 格式错误")
			return
		}

		var req adminCreatePackageReq
		if err := c.ShouldBindJSON(&req); err != nil {
			middleware.Fail(c, http.StatusBadRequest, 1001, "参数错误: "+err.Error())
			return
		}

		updates := map[string]interface{}{
			"name":          req.Name,
			"description":   req.Description,
			"monthly_price": req.PriceMonthly,
			"yearly_price":  req.PriceYearly,
			"max_apps":      req.MaxApps,
			"max_cards":     req.MaxCards,
			"max_agents":    req.MaxAgents,
			"features":      req.Features,
		}
		if req.Status != "" {
			updates["status"] = req.Status
		}

		if err := deps.DB.Model(&model.SysPackage{}).Where("id = ?", id).Updates(updates).Error; err != nil {
			middleware.Fail(c, http.StatusInternalServerError, 5001, "更新失败: "+err.Error())
			return
		}

		middleware.Success(c, gin.H{"id": id, "updated": true})
	}
}

// ============== 4. 代理管理（平台维度） ==============

// AdminListAgents 平台代理列表
// GET /admin/agents?page=&page_size=&keyword=&status=&tenant_id=
func AdminListAgents(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		page, pageSize := parsePagination(c)

		q := deps.DB.Model(&model.Agent{})
		if kw := c.Query("keyword"); kw != "" {
			q = q.Where("username LIKE ? OR real_name LIKE ? OR phone LIKE ?", "%"+kw+"%", "%"+kw+"%", "%"+kw+"%")
		}
		if status := c.Query("status"); status != "" {
			q = q.Where("status = ?", status)
		}
		if tenantID := c.Query("tenant_id"); tenantID != "" {
			q = q.Where("tenant_id = ?", tenantID)
		}

		var total int64
		q.Count(&total)

		var agents []model.Agent
		if err := q.Order("id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&agents).Error; err != nil {
			middleware.Fail(c, http.StatusInternalServerError, 5001, "查询失败")
			return
		}

		list := make([]gin.H, 0, len(agents))
		for _, a := range agents {
			// 联表开发者用户名
			var tenantName string
			deps.DB.Model(&model.SysTenant{}).Where("id = ?", a.TenantID).Select("username").Scan(&tenantName)

			// 联表邀请人用户名（如有）
			inviterUsername := ""
			if a.InviterID != nil {
				deps.DB.Model(&model.Agent{}).Where("id = ?", *a.InviterID).Select("username").Scan(&inviterUsername)
			}

			// 统计字段
			frozenBalance := agentFrozenBalance(deps.DB, a.ID)
			totalCommission := agentTotalCommission(deps.DB, a.ID)
			totalWithdraw := agentTotalWithdrawPaid(deps.DB, a.ID)

			list = append(list, gin.H{
				"id":               a.ID,
				"username":         a.Username,
				"real_name":        a.RealName,
				"phone":            a.Phone,
				"email":            a.Email,
				"tenant_id":        a.TenantID,
				"tenant_name":      tenantName,
				"balance":          a.Balance,
				"frozen_balance":   frozenBalance,
				"total_commission": totalCommission,
				"total_withdraw":   totalWithdraw,
				"status":           a.Status,
				"commission_mode":  a.CommissionMode,
				"commission_rate":  a.CommissionRate,
				"inviter_id":       a.InviterID,
				"inviter_username": inviterUsername,
				"last_login_at":    a.LastLoginAt,
				"last_login_ip":    a.LastLoginIP,
				"created_at":       a.CreatedAt,
			})
		}

		middleware.Success(c, gin.H{
			"list":      list,
			"total":     total,
			"page":      page,
			"page_size": pageSize,
		})
	}
}

// AdminUpdateAgent 更新代理
// PUT /admin/agents/:id
func AdminUpdateAgent(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseUint(c.Param("id"), 10, 64)
		if err != nil {
			middleware.Fail(c, http.StatusBadRequest, 1001, "ID 格式错误")
			return
		}

		var req adminUpdateAgentReq
		if err := c.ShouldBindJSON(&req); err != nil {
			middleware.Fail(c, http.StatusBadRequest, 1001, "参数错误: "+err.Error())
			return
		}

		var agent model.Agent
		if err := deps.DB.First(&agent, id).Error; err != nil {
			middleware.Fail(c, http.StatusNotFound, 1008, "代理不存在")
			return
		}

		updates := make(map[string]interface{})
		if req.Status != nil {
			updates["status"] = *req.Status
		}
		if req.CommissionMode != nil {
			updates["commission_mode"] = *req.CommissionMode
		}
		if req.CommissionRate != nil {
			updates["commission_rate"] = *req.CommissionRate
		}
		// balance 调整应写 agent_balance_log，简化方案直接更新
		if req.Balance != nil {
			updates["balance"] = *req.Balance
			// 写一条调整流水（简化）
			_ = deps.DB.Create(&model.AgentBalanceLog{
				AgentID:  id,
				TenantID: agent.TenantID,
				Type:     "adjust",
				Amount:   *req.Balance - agent.Balance,
				Status:   "settled",
				Remark:   "管理员调整余额",
			}).Error
		}

		if len(updates) == 0 {
			middleware.Fail(c, http.StatusBadRequest, 1001, "未提交任何更新字段")
			return
		}

		if err := deps.DB.Model(&model.Agent{}).Where("id = ?", id).Updates(updates).Error; err != nil {
			middleware.Fail(c, http.StatusInternalServerError, 5001, "更新失败: "+err.Error())
			return
		}

		middleware.Success(c, gin.H{"id": id, "updated": true})
	}
}

// ============== 5. 公告管理 ==============

// AdminListNotices 公告列表（S-15/S-16）
// GET /admin/notices?page=&page_size=&type=&status=&keyword=
func AdminListNotices(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		page, pageSize := parsePagination(c)

		q := deps.DB.Model(&model.Notice{})
		if t := c.Query("type"); t != "" {
			q = q.Where("type = ?", t)
		}
		if s := c.Query("status"); s != "" {
			q = q.Where("status = ?", s)
		}
		if kw := c.Query("keyword"); kw != "" {
			q = q.Where("title LIKE ?", "%"+kw+"%")
		}

		var total int64
		q.Count(&total)

		var notices []model.Notice
		if err := q.Order("is_pinned DESC, id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&notices).Error; err != nil {
			middleware.Fail(c, http.StatusInternalServerError, 5001, "查询失败")
			return
		}

		// 字段映射：IsPinned→pinned, StartAt→publish_at, EndAt→expire_at
		list := make([]gin.H, 0, len(notices))
		for _, n := range notices {
			list = append(list, gin.H{
				"id":         n.ID,
				"type":       n.Type,
				"title":      n.Title,
				"content":    n.Content,
				"status":     n.Status,
				"pinned":     n.IsPinned,
				"sort":       n.Sort,
				"publish_at": n.StartAt,
				"expire_at":  n.EndAt,
				"created_at": n.CreatedAt,
				"updated_at": n.UpdatedAt,
			})
		}

		middleware.Success(c, gin.H{
			"list":      list,
			"total":     total,
			"page":      page,
			"page_size": pageSize,
		})
	}
}

// AdminCreateNotice 创建公告
// POST /admin/notices
func AdminCreateNotice(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req adminCreateNoticeReq
		if err := c.ShouldBindJSON(&req); err != nil {
			middleware.Fail(c, http.StatusBadRequest, 1001, "参数错误: "+err.Error())
			return
		}

		userID := getUserID(c)
		status := req.Status
		if status == "" {
			status = "draft"
		}

		startAt := time.Now()
		if req.PublishAt != nil {
			startAt = *req.PublishAt
		}

		notice := &model.Notice{
			Type:      req.Type,
			Title:     req.Title,
			Content:   req.Content,
			IsPinned:  req.Pinned,
			Sort:      req.Sort,
			StartAt:   startAt,
			EndAt:     req.ExpireAt,
			Status:    status,
			CreatedBy: userID,
		}
		if err := deps.DB.Create(notice).Error; err != nil {
			middleware.Fail(c, http.StatusInternalServerError, 5001, "创建失败: "+err.Error())
			return
		}

		middleware.Success(c, gin.H{
			"id":         notice.ID,
			"type":       notice.Type,
			"title":      notice.Title,
			"status":     notice.Status,
			"pinned":     notice.IsPinned,
			"publish_at": notice.StartAt,
			"expire_at":  notice.EndAt,
		})
	}
}

// AdminUpdateNotice 更新公告（新增）
// PUT /admin/notices/:id
func AdminUpdateNotice(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseUint(c.Param("id"), 10, 64)
		if err != nil {
			middleware.Fail(c, http.StatusBadRequest, 1001, "ID 格式错误")
			return
		}

		var req adminUpdateNoticeReq
		if err := c.ShouldBindJSON(&req); err != nil {
			middleware.Fail(c, http.StatusBadRequest, 1001, "参数错误: "+err.Error())
			return
		}

		updates := make(map[string]interface{})
		if req.Type != nil {
			updates["type"] = *req.Type
		}
		if req.Title != nil {
			updates["title"] = *req.Title
		}
		if req.Content != nil {
			updates["content"] = *req.Content
		}
		if req.Status != nil {
			updates["status"] = *req.Status
		}
		if req.Pinned != nil {
			updates["is_pinned"] = *req.Pinned
		}
		if req.Sort != nil {
			updates["sort"] = *req.Sort
		}
		if req.PublishAt != nil {
			updates["start_at"] = *req.PublishAt
		}
		if req.ExpireAt != nil {
			updates["end_at"] = *req.ExpireAt
		}

		if len(updates) == 0 {
			middleware.Fail(c, http.StatusBadRequest, 1001, "未提交任何更新字段")
			return
		}

		if err := deps.DB.Model(&model.Notice{}).Where("id = ?", id).Updates(updates).Error; err != nil {
			middleware.Fail(c, http.StatusInternalServerError, 5001, "更新失败: "+err.Error())
			return
		}

		middleware.Success(c, gin.H{"id": id, "updated": true})
	}
}

// AdminDeleteNotice 删除公告（新增）
// DELETE /admin/notices/:id
func AdminDeleteNotice(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseUint(c.Param("id"), 10, 64)
		if err != nil {
			middleware.Fail(c, http.StatusBadRequest, 1001, "ID 格式错误")
			return
		}

		if err := deps.DB.Delete(&model.Notice{}, id).Error; err != nil {
			middleware.Fail(c, http.StatusInternalServerError, 5001, "删除失败: "+err.Error())
			return
		}

		// 级联清理 notice_read / notice_target，避免脏数据
		deps.DB.Where("notice_id = ?", id).Delete(&model.NoticeRead{})
		deps.DB.Where("notice_id = ?", id).Delete(&model.NoticeTarget{})

		middleware.Success(c, gin.H{"id": id, "deleted": true})
	}
}

// ============== 6. 日志审计 ==============

// AdminListLogs 日志审计列表
// GET /admin/logs?page=&page_size=&type=&user_id=&start_date=&end_date=&keyword=
// 数据源：log_operation 表（login/pay/security/system 类型暂返回空，留待 v0.4.x 接入对应日志表）
func AdminListLogs(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		page, pageSize := parsePagination(c)

		q := deps.DB.Model(&model.LogOperation{})
		if uid := c.Query("user_id"); uid != "" {
			q = q.Where("operator_id = ?", uid)
		}
		if startDate := c.Query("start_date"); startDate != "" {
			q = q.Where("created_at >= ?", startDate+" 00:00:00")
		}
		if endDate := c.Query("end_date"); endDate != "" {
			q = q.Where("created_at <= ?", endDate+" 23:59:59")
		}
		if kw := c.Query("keyword"); kw != "" {
			q = q.Where("action LIKE ? OR module LIKE ?", "%"+kw+"%", "%"+kw+"%")
		}
		// type 字段：log_operation 仅含 operation 类型，其他类型（login/pay/security/system）暂返回空
		if t := c.Query("type"); t != "" && t != "operation" {
			middleware.Success(c, gin.H{
				"list":      []interface{}{},
				"total":     0,
				"page":      page,
				"page_size": pageSize,
			})
			return
		}

		var total int64
		q.Count(&total)

		var logs []model.LogOperation
		if err := q.Order("id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&logs).Error; err != nil {
			middleware.Fail(c, http.StatusInternalServerError, 5001, "查询失败")
			return
		}

		// 字段映射：OperatorType→role, OperatorID→user_id, OperatorIP→ip
		// v0.3.1：username/user_agent/status 已落库
		list := make([]gin.H, 0, len(logs))
		for _, l := range logs {
			list = append(list, gin.H{
				"id":         l.ID,
				"type":       "operation",
				"user_id":    l.OperatorID,
				"username":   l.Username,
				"role":       l.OperatorType,
				"action":     l.Action,
				"target":     l.TargetType,
				"ip":         l.OperatorIP,
				"user_agent": l.UserAgent,
				"status":     l.Status,
				"detail":     l.Detail,
				"created_at": l.CreatedAt,
			})
		}

		middleware.Success(c, gin.H{
			"list":      list,
			"total":     total,
			"page":      page,
			"page_size": pageSize,
		})
	}
}

// ============== 7. 安全防护 ==============

// AdminSecurityStats 安全看板
// GET /admin/security
func AdminSecurityStats(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		// IP 黑名单总数 / 生效中
		var ipBlacklistCount, ipBlacklistActive int64
		deps.DB.Model(&model.SecIPBlacklist{}).Count(&ipBlacklistCount)
		deps.DB.Model(&model.SecIPBlacklist{}).
			Where("expires_at IS NULL OR expires_at > ?", time.Now()).
			Count(&ipBlacklistActive)

		// 今日登录失败次数（v0.3.1：从 log_login_failed 表查询）
		failedLoginToday := securityFailedLoginToday(deps, "")
		// 今日自动封禁 IP 数（source=auto 且今日创建）
		failedLoginBlocked := securityBlockedIPsToday(deps)

		// 2FA 已启用用户数（v0.3.1：含 agent）
		var totpEnabledAdmin, totpEnabledTenant, totpEnabledAgent int64
		deps.DB.Model(&model.SysAdmin{}).Where("totp_secret != ''").Count(&totpEnabledAdmin)
		deps.DB.Model(&model.SysTenant{}).Where("totp_secret != ''").Count(&totpEnabledTenant)
		deps.DB.Model(&model.Agent{}).Where("totp_secret != ''").Count(&totpEnabledAgent)
		totpEnabledUsers := totpEnabledAdmin + totpEnabledTenant + totpEnabledAgent

		// 今日敏感操作数（log_operation 今日总数）
		startOfToday := time.Date(time.Now().Year(), time.Now().Month(), time.Now().Day(), 0, 0, 0, 0, time.Local)
		var sensitiveOpsToday int64
		deps.DB.Model(&model.LogOperation{}).Where("created_at >= ?", startOfToday).Count(&sensitiveOpsToday)

		// 最近封禁 IP（最近 10 条）
		var recentBlocked []model.SecIPBlacklist
		deps.DB.Order("id DESC").Limit(10).Find(&recentBlocked)
		recentBlockedIPs := make([]gin.H, 0, len(recentBlocked))
		for _, b := range recentBlocked {
			recentBlockedIPs = append(recentBlockedIPs, gin.H{
				"ip":         b.IP,
				"reason":     b.Reason,
				"blocked_at": b.CreatedAt,
			})
		}

		middleware.Success(c, gin.H{
			"ip_blacklist_count":     ipBlacklistCount,
			"ip_blacklist_active":    ipBlacklistActive,
			"failed_login_today":     failedLoginToday,
			"failed_login_blocked":   failedLoginBlocked,
			"totp_enabled_users":     totpEnabledUsers,
			"sensitive_ops_today":    sensitiveOpsToday,
			"recent_blocked_ips":     recentBlockedIPs,
		})
	}
}

// AdminListIPBlacklist IP 黑名单列表
// GET /admin/security/ip_blacklist?page=&page_size=
func AdminListIPBlacklist(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		page, pageSize := parsePagination(c)

		var total int64
		deps.DB.Model(&model.SecIPBlacklist{}).Count(&total)

		var list []model.SecIPBlacklist
		if err := deps.DB.Order("id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&list).Error; err != nil {
			middleware.Fail(c, http.StatusInternalServerError, 5001, "查询失败")
			return
		}

		// 字段映射：ExpiresAt→expire_at
		result := make([]gin.H, 0, len(list))
		for _, b := range list {
			result = append(result, gin.H{
				"id":              b.ID,
				"ip":              b.IP,
				"reason":          b.Reason,
				"source":          b.Source,
				"expire_at":       b.ExpiresAt,
				"created_by":      b.CreatedBy,
				"created_by_type": b.CreatedByType,
				"created_at":      b.CreatedAt,
			})
		}

		middleware.Success(c, gin.H{
			"list":      result,
			"total":     total,
			"page":      page,
			"page_size": pageSize,
		})
	}
}

// AdminAddIPBlacklist 加入 IP 黑名单
// POST /admin/security/ip_blacklist
func AdminAddIPBlacklist(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req adminAddIPBlacklistReq
		if err := c.ShouldBindJSON(&req); err != nil {
			middleware.Fail(c, http.StatusBadRequest, 1001, "参数错误: "+err.Error())
			return
		}

		var expiresAt *time.Time
		if req.ExpireHours > 0 {
			t := time.Now().Add(time.Duration(req.ExpireHours) * time.Hour)
			expiresAt = &t
		}

		item := &model.SecIPBlacklist{
			IP:            req.IP,
			Reason:        req.Reason,
			Source:        "manual",
			CreatedBy:     uint64Ptr(getUserID(c)),
			CreatedByType: "admin",
			ExpiresAt:     expiresAt,
		}
		if err := deps.DB.Create(item).Error; err != nil {
			middleware.Fail(c, http.StatusInternalServerError, 5001, "创建失败: "+err.Error())
			return
		}

		middleware.Success(c, gin.H{
			"id":         item.ID,
			"ip":         item.IP,
			"reason":     item.Reason,
			"expire_at":  item.ExpiresAt,
			"created_at": item.CreatedAt,
		})
	}
}

// AdminRemoveIPBlacklist 移出 IP 黑名单
// DELETE /admin/security/ip_blacklist/:id
func AdminRemoveIPBlacklist(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseUint(c.Param("id"), 10, 64)
		if err != nil {
			middleware.Fail(c, http.StatusBadRequest, 1001, "ID 格式错误")
			return
		}

		if err := deps.DB.Delete(&model.SecIPBlacklist{}, id).Error; err != nil {
			middleware.Fail(c, http.StatusInternalServerError, 5001, "删除失败: "+err.Error())
			return
		}

		middleware.Success(c, gin.H{"id": id, "deleted": true})
	}
}

// ============== v0.4.0 多级代理：代理树查询 ==============

// AdminGetAgentTree GET /api/v1/admin/agents/:id/tree
// 平台超管查询指定代理的下级代理树（递归，最深 max_level-1 层，跨租户可查）
func AdminGetAgentTree(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		agentID, err := parseUintParam(c, "id")
		if err != nil || agentID == 0 {
			middleware.Fail(c, http.StatusBadRequest, 1001, "代理 ID 无效")
			return
		}

		maxLevel := int(deps.CfgCache.GetInt(c.Request.Context(), "agent.commission.max_level", 3))
		if maxLevel < 1 {
			maxLevel = 1
		}
		maxDepth := maxLevel - 1

		tree, err := multilevel.BuildAgentTree(c.Request.Context(), deps.DB, agentID, maxDepth)
		if err != nil {
			if errors.Is(err, multilevel.ErrAgentNotFound) {
				middleware.Fail(c, http.StatusNotFound, 1004, "代理账号不存在")
				return
			}
			middleware.Fail(c, http.StatusInternalServerError, 5002, "构建代理树失败: "+err.Error())
			return
		}

		middleware.Success(c, gin.H{
			"tree": tree,
		})
	}
}

// ============== v0.4.0 灰度发布：版本管理（超管跨租户查询） ==============

// adminVersionListItem 超管版本列表项（含租户名 + 应用名）
type adminVersionListItem struct {
	model.AppVersion
	TenantName string `json:"tenant_name"`
	AppName    string `json:"app_name"`
}

// AdminListVersions GET /api/v1/admin/versions
// 平台超管跨租户查询所有应用版本（支持 tenant_id/app_id/channel/release_strategy 过滤）
func AdminListVersions(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		page, pageSize := parsePagination(c)

		q := deps.DB.Table("app_version").
			Select("app_version.*, sys_tenant.username as tenant_name, app.name as app_name").
			Joins("LEFT JOIN sys_tenant ON sys_tenant.id = app_version.tenant_id").
			Joins("LEFT JOIN app ON app.id = app_version.app_id")

		if tenantIDStr := c.Query("tenant_id"); tenantIDStr != "" {
			tid, _ := strconv.ParseUint(tenantIDStr, 10, 64)
			q = q.Where("app_version.tenant_id = ?", tid)
		}
		if appIDStr := c.Query("app_id"); appIDStr != "" {
			aid, _ := strconv.ParseUint(appIDStr, 10, 64)
			q = q.Where("app_version.app_id = ?", aid)
		}
		if channel := c.Query("channel"); channel != "" {
			q = q.Where("app_version.channel = ?", channel)
		}
		if strategy := c.Query("release_strategy"); strategy != "" {
			q = q.Where("app_version.release_strategy = ?", strategy)
		}

		var total int64
		q.Count(&total)

		var items []adminVersionListItem
		if err := q.Order("app_version.id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Scan(&items).Error; err != nil {
			middleware.Fail(c, http.StatusInternalServerError, 5001, "查询失败: "+err.Error())
			return
		}

		middleware.Success(c, gin.H{
			"list":      items,
			"total":     total,
			"page":      page,
			"page_size": pageSize,
		})
	}
}

// AdminGetVersion GET /api/v1/admin/versions/:id
// 平台超管查询单个版本详情（跨租户）
func AdminGetVersion(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := parseUintParam(c, "id")
		if err != nil || id == 0 {
			middleware.Fail(c, http.StatusBadRequest, 1001, "版本 ID 无效")
			return
		}

		var v model.AppVersion
		if err := deps.DB.Where("id = ?", id).First(&v).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				middleware.Fail(c, http.StatusNotFound, 1008, "版本不存在")
				return
			}
			middleware.Fail(c, http.StatusInternalServerError, 5001, "查询失败")
			return
		}
		middleware.Success(c, v)
	}
}

// 防止未使用导入报错（gorm 在某些函数中显式使用，但保留兜底）
var _ = gorm.ErrRecordNotFound
