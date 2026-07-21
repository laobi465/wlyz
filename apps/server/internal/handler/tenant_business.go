// 开发者控制台业务接口 Handler
// 包含工作台、设备、订单、云变量、版本、代理、邀请码、支付配置、公告等管理
// 严格遵循铁律 04/05/06：禁止硬编码、配置走 CfgCache、不确定处标注「待核实」
package handler

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/your-org/keyauth-saas/apps/server/internal/grayscale"
	"github.com/your-org/keyauth-saas/apps/server/internal/heartbeat"
	"github.com/your-org/keyauth-saas/apps/server/internal/logger"
	"github.com/your-org/keyauth-saas/apps/server/internal/middleware"
	"github.com/your-org/keyauth-saas/apps/server/internal/model"
	"github.com/your-org/keyauth-saas/apps/server/internal/multilevel"
	"github.com/your-org/keyauth-saas/apps/server/internal/notify"
	"github.com/your-org/keyauth-saas/apps/server/internal/quota"
)

// ============== 辅助 ==============

// parsePagination 解析 page/page_size，返回 (page, pageSize)
// 默认 page=1, page_size=20，pageSize 上限 100
func parsePagination(c *gin.Context) (int, int) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	return page, pageSize
}

// sumResult 用于 SUM 聚合查询的扫描目标
type sumResult struct {
	Total float64
}

// genInviteCodeUnique 生成 16 位大小写字母数字邀请码（使用 crypto/rand）
// 重试 5 次保证唯一性，传入 db 以支持事务内可见性
func genInviteCodeUnique(db *gorm.DB) (string, error) {
	const charset = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
	const codeLen = 16
	for attempt := 0; attempt < 5; attempt++ {
		buf := make([]byte, codeLen)
		if _, err := rand.Read(buf); err != nil {
			return "", err
		}
		for i := range buf {
			buf[i] = charset[int(buf[i])%len(charset)]
		}
		code := string(buf)
		var count int64
		if err := db.Model(&model.AgentInviteCode{}).Where("code = ?", code).Count(&count).Error; err != nil {
			return "", err
		}
		if count == 0 {
			return code, nil
		}
	}
	return "", fmt.Errorf("生成唯一邀请码失败（重试 5 次均冲突）")
}

// ============== 1. 开发者工作台 ==============

// TenantDashboard 开发者工作台
// GET /tenant/dashboard
// 一次返回全部统计指标 + 7 日收入趋势 + 最近 10 个订单 + Top 5 应用
func TenantDashboard(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID := getTenantID(c)
		if tenantID == 0 {
			middleware.Fail(c, http.StatusForbidden, 1003, "无法识别租户身份")
			return
		}
		ctx := c.Request.Context()
		now := time.Now()
		todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
		heartbeatTimeout := deps.CfgCache.GetInt(ctx, "app.default.heartbeat_timeout", 180)
		onlineThreshold := now.Add(-time.Duration(heartbeatTimeout) * time.Second)

		// ---- 统计指标 ----
		var appTotal, appActive, cardTotal, cardActive, cardUsed int64
		var deviceOnline, deviceTotal, orderToday, settlementPending, agentTotal int64
		var revenueToday, revenueMonth, settlementAmount float64

		deps.DB.Model(&model.App{}).Where("tenant_id = ?", tenantID).Count(&appTotal)
		deps.DB.Model(&model.App{}).Where("tenant_id = ? AND status = ?", tenantID, "active").Count(&appActive)
		deps.DB.Model(&model.AppCard{}).Where("tenant_id = ?", tenantID).Count(&cardTotal)
		deps.DB.Model(&model.AppCard{}).Where("tenant_id = ? AND status = ?", tenantID, "active").Count(&cardActive)
		// card_used：已激活过的卡密（active + expired）
		// 待核实：是否应包含 banned 状态
		deps.DB.Model(&model.AppCard{}).Where("tenant_id = ? AND status IN ?", tenantID, []string{"active", "expired"}).Count(&cardUsed)
		// device_online：简化用 last_heartbeat_at > NOW()-heartbeat_timeout 判定
		// 待核实：是否应改用 heartbeat.CountOnline 按 app 累加（更精确但需遍历应用）
		deps.DB.Model(&model.AppDevice{}).Where("tenant_id = ? AND last_heartbeat_at > ?", tenantID, onlineThreshold).Count(&deviceOnline)
		deps.DB.Model(&model.AppDevice{}).Where("tenant_id = ?", tenantID).Count(&deviceTotal)
		deps.DB.Model(&model.AppOrder{}).Where("tenant_id = ? AND pay_status = ? AND paid_at >= ?", tenantID, "paid", todayStart).Count(&orderToday)

		var revTodaySum sumResult
		deps.DB.Model(&model.AppOrder{}).
			Where("tenant_id = ? AND pay_status = ? AND paid_at >= ?", tenantID, "paid", todayStart).
			Select("COALESCE(SUM(total_amount), 0) as total").Scan(&revTodaySum)
		revenueToday = revTodaySum.Total

		var revMonthSum sumResult
		deps.DB.Model(&model.AppOrder{}).
			Where("tenant_id = ? AND pay_status = ? AND paid_at >= ?", tenantID, "paid", monthStart).
			Select("COALESCE(SUM(total_amount), 0) as total").Scan(&revMonthSum)
		revenueMonth = revMonthSum.Total

		deps.DB.Model(&model.PlatformSettlement{}).Where("tenant_id = ? AND status = ?", tenantID, "pending").Count(&settlementPending)
		var settleSum sumResult
		deps.DB.Model(&model.PlatformSettlement{}).
			Where("tenant_id = ? AND status = ?", tenantID, "pending").
			Select("COALESCE(SUM(net_amount), 0) as total").Scan(&settleSum)
		settlementAmount = settleSum.Total

		deps.DB.Model(&model.Agent{}).Where("tenant_id = ?", tenantID).Count(&agentTotal)

		// ---- 收入趋势（近 7 天）----
		// Bug 2 P0：前端 tenant.ts/Dashboard.vue 期望字段名 amount（非 revenue），统一改 amount
		type revenueTrendItem struct {
			Date   string  `json:"date"`
			Amount float64 `json:"amount"`
		}
		sevenDaysAgo := todayStart.AddDate(0, 0, -6)
		var trendRows []revenueTrendItem
		deps.DB.Model(&model.AppOrder{}).
			Select("DATE(paid_at) as date, COALESCE(SUM(total_amount), 0) as amount").
			Where("tenant_id = ? AND pay_status = ? AND paid_at >= ?", tenantID, "paid", sevenDaysAgo).
			Group("DATE(paid_at)").Order("date ASC").Scan(&trendRows)
		trendMap := make(map[string]float64, len(trendRows))
		for _, r := range trendRows {
			trendMap[r.Date] = r.Amount
		}
		revenueTrend := make([]revenueTrendItem, 0, 7)
		for i := 0; i < 7; i++ {
			d := sevenDaysAgo.AddDate(0, 0, i).Format("2006-01-02")
			revenueTrend = append(revenueTrend, revenueTrendItem{Date: d, Amount: trendMap[d]})
		}

		// ---- 最近 10 个订单 ----
		// Bug 4 P0：前端 Dashboard.vue 表格列期望 amount/status（非 total_amount/pay_status）
		type recentOrderItem struct {
			ID        uint64  `json:"id"`
			OrderNo   string  `json:"order_no"`
			Amount    float64 `json:"amount"`
			Status    string  `json:"status"`
			CreatedAt string  `json:"created_at"`
		}
		var rawOrders []model.AppOrder
		deps.DB.Where("tenant_id = ?", tenantID).Order("id DESC").Limit(10).Find(&rawOrders)
		recentOrders := make([]recentOrderItem, 0, len(rawOrders))
		for _, o := range rawOrders {
			recentOrders = append(recentOrders, recentOrderItem{
				ID:        o.ID,
				OrderNo:   o.OrderNo,
				Amount:    o.TotalAmount,
				Status:    o.PayStatus,
				CreatedAt: o.CreatedAt.Format(time.RFC3339),
			})
		}

		// ---- Top 5 应用（按卡密数排序）----
		// Bug 3 P0：前端 Dashboard.vue 期望 id/name/revenue 字段名 + revenue 聚合字段
		type topAppItem struct {
			ID        uint64  `json:"id"`
			Name      string  `json:"name"`
			CardCount int64   `json:"card_count"`
			Revenue   float64 `json:"revenue"`
		}
		var topApps []topAppItem
		deps.DB.Table("app_card").
			Select("app_card.app_id as id, app.name as name, COUNT(*) as card_count, "+
				"COALESCE(SUM(app_order.total_amount), 0) as revenue").
			Joins("LEFT JOIN app ON app.id = app_card.app_id").
			Joins("LEFT JOIN app_order ON app_order.app_id = app_card.app_id AND app_order.pay_status = 'paid'").
			Where("app_card.tenant_id = ?", tenantID).
			Group("app_card.app_id, app.name").
			Order("card_count DESC").
			Limit(5).
			Scan(&topApps)

		middleware.Success(c, gin.H{
			"app_total":          appTotal,
			"app_active":         appActive,
			"card_total":         cardTotal,
			"card_active":        cardActive,
			"card_used":          cardUsed,
			"device_online":      deviceOnline,
			"device_total":       deviceTotal,
			"order_today":        orderToday,
			"revenue_today":      revenueToday,
			"revenue_month":      revenueMonth,
			"settlement_pending": settlementPending,
			"settlement_amount":  settlementAmount,
			"agent_total":        agentTotal,
			"revenue_trend":      revenueTrend,
			"recent_orders":      recentOrders,
			"top_apps":           topApps,
		})
	}
}

// ============== 2. 设备列表 ==============

// deviceListItem 设备列表项（字段名按前端 TenantDevice 接口映射）
type deviceListItem struct {
	ID          uint64     `json:"id"`
	AppID       uint64     `json:"app_id"`
	AppName     string     `json:"app_name"`
	CardID      uint64     `json:"card_id"`
	CardKey     string     `json:"card_key"`
	DeviceID    string     `json:"device_id"` // 对应 hwid
	DeviceName  string     `json:"device_name"`
	IP          string     `json:"ip"`
	Location    string     `json:"location"` // Bug 6 P0：前端表格列期望 location，暂返回空字符串（IP→地理位置需 GeoIP 依赖，v0.6.x 再接入）
	UserAgent   string     `json:"user_agent"`
	HeartbeatAt *time.Time `json:"heartbeat_at"`
	CreatedAt   time.Time  `json:"created_at"`
	IsOnline    bool       `json:"is_online"`
}

// TenantListDevices 设备列表
// GET /tenant/devices?page=&page_size=&app_id=&keyword=&online=
func TenantListDevices(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID := getTenantID(c)
		if tenantID == 0 {
			middleware.Fail(c, http.StatusForbidden, 1003, "无法识别租户身份")
			return
		}
		page, pageSize := parsePagination(c)
		ctx := c.Request.Context()
		heartbeatTimeout := deps.CfgCache.GetInt(ctx, "app.default.heartbeat_timeout", 180)
		onlineThreshold := time.Now().Add(-time.Duration(heartbeatTimeout) * time.Second)

		q := deps.DB.Table("app_device").
			Select("app_device.id, app_device.app_id, app.name as app_name, app_device.card_id, "+
				"app_card.card_key as card_key, app_device.hwid as device_id, app_device.device_name, "+
				"app_device.ip_address as ip, app_device.last_heartbeat_at as heartbeat_at, app_device.created_at").
			Joins("LEFT JOIN app ON app.id = app_device.app_id").
			Joins("LEFT JOIN app_card ON app_card.id = app_device.card_id").
			Where("app_device.tenant_id = ?", tenantID)

		if appIDStr := c.Query("app_id"); appIDStr != "" {
			appID, _ := strconv.ParseUint(appIDStr, 10, 64)
			q = q.Where("app_device.app_id = ?", appID)
		}
		if kw := c.Query("keyword"); kw != "" {
			q = q.Where("app_device.device_name LIKE ? OR app_device.hwid LIKE ? OR app_device.ip_address LIKE ?",
				"%"+kw+"%", "%"+kw+"%", "%"+kw+"%")
		}
		switch c.Query("online") {
		case "true":
			q = q.Where("app_device.last_heartbeat_at > ?", onlineThreshold)
		case "false":
			q = q.Where("app_device.last_heartbeat_at IS NULL OR app_device.last_heartbeat_at <= ?", onlineThreshold)
		}

		var total int64
		q.Count(&total)

		var items []deviceListItem
		if err := q.Order("app_device.id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Scan(&items).Error; err != nil {
			logger.Error("tenant: list devices query failed", "err", err, "tenant_id", tenantID)
			middleware.Fail(c, http.StatusInternalServerError, 5001, "查询失败")
			return
		}
		for i := range items {
			items[i].IsOnline = items[i].HeartbeatAt != nil && items[i].HeartbeatAt.After(onlineThreshold)
		}

		middleware.Success(c, gin.H{
			"list":      items,
			"total":     total,
			"page":      page,
			"page_size": pageSize,
		})
	}
}

// ============== 3. 踢设备下线 ==============

// TenantKickDevice 踢设备下线
// POST /tenant/devices/:id/kick
func TenantKickDevice(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID := getTenantID(c)
		if tenantID == 0 {
			middleware.Fail(c, http.StatusForbidden, 1003, "无法识别租户身份")
			return
		}
		id, err := strconv.ParseUint(c.Param("id"), 10, 64)
		if err != nil {
			middleware.Fail(c, http.StatusBadRequest, 1001, "设备 ID 格式错误")
			return
		}

		var device model.AppDevice
		if err := deps.DB.Where("id = ? AND tenant_id = ?", id, tenantID).First(&device).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				middleware.Fail(c, http.StatusNotFound, 1008, "设备不存在或无权访问")
				return
			}
			middleware.Fail(c, http.StatusInternalServerError, 5001, "查询失败")
			return
		}

		if err := deps.DB.Model(&model.AppDevice{}).Where("id = ?", id).
			Updates(map[string]interface{}{
				"status":            "offline",
				"last_heartbeat_at": nil,
			}).Error; err != nil {
			middleware.Fail(c, http.StatusInternalServerError, 5002, "踢下线失败")
			return
		}

		// 移除 Redis 心跳在线状态（deps.Redis 为 nil 时 Remove 内部直接返回）
		_ = heartbeat.Remove(c.Request.Context(), deps.Redis, device.AppID, device.ID)

		middleware.Success(c, gin.H{"ok": true})
	}
}

// ============== 4. 订单列表 ==============

// orderListItem 订单列表项（嵌入 AppOrder + 联表字段 + 计算 channel）
type orderListItem struct {
	model.AppOrder
	AppName       string `json:"app_name"`
	CardTypeName  string `json:"card_type_name"`
	AgentUsername string `json:"agent_username"`
	Channel       string `json:"channel"` // h5/agent/manual
}

// TenantListOrders 订单列表
// GET /tenant/orders?page=&page_size=&app_id=&status=&channel=&start_date=&end_date=&keyword=
func TenantListOrders(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID := getTenantID(c)
		if tenantID == 0 {
			middleware.Fail(c, http.StatusForbidden, 1003, "无法识别租户身份")
			return
		}
		page, pageSize := parsePagination(c)

		q := deps.DB.Table("app_order").
			Select("app_order.*, app.name as app_name, app_card_type.name as card_type_name, agent.username as agent_username").
			Joins("LEFT JOIN app ON app.id = app_order.app_id").
			Joins("LEFT JOIN app_card_type ON app_card_type.id = app_order.card_type_id").
			Joins("LEFT JOIN agent ON agent.id = app_order.agent_id").
			Where("app_order.tenant_id = ?", tenantID)

		if appIDStr := c.Query("app_id"); appIDStr != "" {
			appID, _ := strconv.ParseUint(appIDStr, 10, 64)
			q = q.Where("app_order.app_id = ?", appID)
		}
		if status := c.Query("status"); status != "" {
			q = q.Where("app_order.pay_status = ?", status)
		}
		// channel 简化判定：agent_id != null = 'agent'，pay_channel 含 'manual' = 'manual'，其余 'h5'
		switch c.Query("channel") {
		case "agent":
			q = q.Where("app_order.agent_id IS NOT NULL")
		case "manual":
			q = q.Where("app_order.agent_id IS NULL AND app_order.pay_channel LIKE ?", "%manual%")
		case "h5":
			q = q.Where("app_order.agent_id IS NULL AND app_order.pay_channel NOT LIKE ?", "%manual%")
		}
		if startDate := c.Query("start_date"); startDate != "" {
			q = q.Where("DATE(app_order.paid_at) >= ?", startDate)
		}
		if endDate := c.Query("end_date"); endDate != "" {
			q = q.Where("DATE(app_order.paid_at) <= ?", endDate)
		}
		if kw := c.Query("keyword"); kw != "" {
			q = q.Where("app_order.order_no LIKE ? OR app_order.buyer_contact LIKE ?", "%"+kw+"%", "%"+kw+"%")
		}

		var total int64
		q.Count(&total)

		var orders []orderListItem
		if err := q.Order("app_order.id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Scan(&orders).Error; err != nil {
			logger.Error("tenant: list orders query failed", "err", err, "tenant_id", tenantID)
			middleware.Fail(c, http.StatusInternalServerError, 5001, "查询失败")
			return
		}
		// 后处理：计算 channel 字段
		for i := range orders {
			channel := "h5"
			if orders[i].AgentID != nil {
				channel = "agent"
			} else if strings.Contains(orders[i].PayChannel, "manual") {
				channel = "manual"
			}
			orders[i].Channel = channel
		}

		middleware.Success(c, gin.H{
			"list":      orders,
			"total":     total,
			"page":      page,
			"page_size": pageSize,
		})
	}
}

// ============== 5. 云变量列表 ==============

// cloudVarListItem 云变量列表项
// Bug 1 P0：前端 tenant.ts/CloudVars.vue 期望 key/value/value_type/description 字段名
//   - 不再嵌入 model.AppCloudVar（避免 var_key/var_value/var_type/remark 透传导致前端字段全错）
type cloudVarListItem struct {
	ID          uint64 `json:"id"`
	AppID       uint64 `json:"app_id"`
	AppName     string `json:"app_name"`
	Key         string `json:"key"`
	Value       string `json:"value"`
	ValueType   string `json:"value_type"`
	ReadOnly    bool   `json:"read_only"`
	Description string `json:"description"` // 对应 model.Remark
	Status      string `json:"status"`
	UpdatedAt   string `json:"updated_at"`
	CreatedAt   string `json:"created_at"`
}

// TenantListCloudVars 云变量列表
// GET /tenant/cloud_vars?page=&page_size=&app_id=&keyword=
func TenantListCloudVars(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID := getTenantID(c)
		if tenantID == 0 {
			middleware.Fail(c, http.StatusForbidden, 1003, "无法识别租户身份")
			return
		}
		page, pageSize := parsePagination(c)

		q := deps.DB.Table("app_cloud_var").
			Select("app_cloud_var.id, app_cloud_var.app_id, app.name as app_name, "+
				"app_cloud_var.var_key as key, app_cloud_var.var_value as value, "+
				"app_cloud_var.var_type as value_type, app_cloud_var.read_only, "+
				"app_cloud_var.remark as description, app_cloud_var.status, "+
				"app_cloud_var.updated_at, app_cloud_var.created_at").
			Joins("LEFT JOIN app ON app.id = app_cloud_var.app_id").
			Where("app_cloud_var.tenant_id = ?", tenantID)

		if appIDStr := c.Query("app_id"); appIDStr != "" {
			appID, _ := strconv.ParseUint(appIDStr, 10, 64)
			q = q.Where("app_cloud_var.app_id = ?", appID)
		}
		if kw := c.Query("keyword"); kw != "" {
			q = q.Where("app_cloud_var.var_key LIKE ?", "%"+kw+"%")
		}

		var total int64
		q.Count(&total)

		var items []cloudVarListItem
		if err := q.Order("app_cloud_var.id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Scan(&items).Error; err != nil {
			logger.Error("tenant: list cloud vars query failed", "err", err, "tenant_id", tenantID)
			middleware.Fail(c, http.StatusInternalServerError, 5001, "查询失败")
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

// ============== 6. 创建/更新云变量 ==============

// upsertCloudVarReq 云变量 upsert 请求
type upsertCloudVarReq struct {
	AppID       uint64 `json:"app_id" binding:"required"`
	Key         string `json:"key" binding:"required,min=1,max=128"`
	Value       string `json:"value" binding:"omitempty"`
	ValueType   string `json:"value_type" binding:"omitempty,oneof=string number json bool"`
	ReadOnly    bool   `json:"read_only"`
	Description string `json:"description" binding:"omitempty,max=255"` // 写入 model.Remark
}

// TenantUpsertCloudVar 创建/更新云变量
// POST /tenant/cloud_vars
func TenantUpsertCloudVar(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID := getTenantID(c)
		if tenantID == 0 {
			middleware.Fail(c, http.StatusForbidden, 1003, "无法识别租户身份")
			return
		}
		var req upsertCloudVarReq
		if err := c.ShouldBindJSON(&req); err != nil {
			middleware.Fail(c, http.StatusBadRequest, 1001, "参数错误: "+err.Error())
			return
		}

		if !checkAppOwnership(deps.DB, req.AppID, tenantID) {
			middleware.Fail(c, http.StatusForbidden, 1003, "应用不存在或无权访问")
			return
		}

		valueType := req.ValueType
		if valueType == "" {
			valueType = "string"
		}

		var cv model.AppCloudVar
		result := deps.DB.Where("tenant_id = ? AND app_id = ? AND var_key = ?", tenantID, req.AppID, req.Key).First(&cv)
		if result.Error == gorm.ErrRecordNotFound {
			cv = model.AppCloudVar{
				TenantID: tenantID,
				AppID:    req.AppID,
				VarKey:   req.Key,
				VarValue: req.Value,
				VarType:  valueType,
				ReadOnly: req.ReadOnly,
				Remark:   req.Description,
				Status:   "active",
			}
			if err := deps.DB.Create(&cv).Error; err != nil {
				logger.Error("tenant: create cloud var failed", "err", err, "tenant_id", tenantID, "app_id", req.AppID)
				middleware.Fail(c, http.StatusInternalServerError, 5001, "创建云变量失败")
				return
			}
		} else if result.Error != nil {
			logger.Error("tenant: query cloud var failed", "err", result.Error, "tenant_id", tenantID, "app_id", req.AppID)
			middleware.Fail(c, http.StatusInternalServerError, 5001, "查询失败")
			return
		} else {
			updates := map[string]interface{}{
				"var_value": req.Value,
				"var_type":  valueType,
				"read_only": req.ReadOnly,
				"remark":    req.Description,
			}
			if err := deps.DB.Model(&model.AppCloudVar{}).Where("id = ?", cv.ID).Updates(updates).Error; err != nil {
				logger.Error("tenant: update cloud var failed", "err", err, "id", cv.ID)
				middleware.Fail(c, http.StatusInternalServerError, 5002, "更新云变量失败")
				return
			}
		}

		middleware.Success(c, gin.H{"id": cv.ID, "upserted": true})
	}
}

// ============== 7. 删除云变量 ==============

// TenantDeleteCloudVar 删除云变量
// DELETE /tenant/cloud_vars/:id
func TenantDeleteCloudVar(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID := getTenantID(c)
		if tenantID == 0 {
			middleware.Fail(c, http.StatusForbidden, 1003, "无法识别租户身份")
			return
		}
		id, err := strconv.ParseUint(c.Param("id"), 10, 64)
		if err != nil {
			middleware.Fail(c, http.StatusBadRequest, 1001, "ID 格式错误")
			return
		}

		var cv model.AppCloudVar
		if err := deps.DB.Where("id = ? AND tenant_id = ?", id, tenantID).First(&cv).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				middleware.Fail(c, http.StatusNotFound, 1008, "云变量不存在或无权访问")
				return
			}
			middleware.Fail(c, http.StatusInternalServerError, 5001, "查询失败")
			return
		}

		if err := deps.DB.Delete(&model.AppCloudVar{}, id).Error; err != nil {
			middleware.Fail(c, http.StatusInternalServerError, 5002, "删除失败")
			return
		}
		middleware.Success(c, gin.H{"id": id, "deleted": true})
	}
}

// ============== 8. 版本列表 ==============

// versionListItem 版本列表项
type versionListItem struct {
	model.AppVersion
	AppName string `json:"app_name"`
}

// TenantListVersions 版本列表
// GET /tenant/versions?page=&page_size=&app_id=&channel=
func TenantListVersions(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID := getTenantID(c)
		if tenantID == 0 {
			middleware.Fail(c, http.StatusForbidden, 1003, "无法识别租户身份")
			return
		}
		page, pageSize := parsePagination(c)

		q := deps.DB.Table("app_version").
			Select("app_version.*, app.name as app_name").
			Joins("LEFT JOIN app ON app.id = app_version.app_id").
			Where("app_version.tenant_id = ?", tenantID)

		if appIDStr := c.Query("app_id"); appIDStr != "" {
			appID, _ := strconv.ParseUint(appIDStr, 10, 64)
			q = q.Where("app_version.app_id = ?", appID)
		}
		if channel := c.Query("channel"); channel != "" {
			q = q.Where("app_version.channel = ?", channel)
		}

		var total int64
		q.Count(&total)

		var items []versionListItem
		if err := q.Order("app_version.id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Scan(&items).Error; err != nil {
			logger.Error("tenant: list versions query failed", "err", err, "tenant_id", tenantID)
			middleware.Fail(c, http.StatusInternalServerError, 5001, "查询失败")
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

// ============== 9. 创建版本 ==============

// createVersionReq 创建版本请求（v0.4.0 升级：含灰度发布字段）
type createVersionReq struct {
	AppID       uint64 `json:"app_id" binding:"required"`
	Version     string `json:"version" binding:"required,min=1,max=32"`
	Channel     string `json:"channel" binding:"omitempty,oneof=stable beta dev"`
	MinVersion  string `json:"min_version" binding:"omitempty,max=32"`
	DownloadURL string `json:"download_url" binding:"omitempty,max=255"`
	BackupURL   string `json:"backup_url" binding:"omitempty,max=255"` // v0.4.0：补全原 DTO 遗漏字段
	ForceUpdate bool   `json:"force_update"`
	UpdateLog   string `json:"update_log" binding:"omitempty,max=5000"`
	Published   bool   `json:"published"`
	// v0.4.0 灰度发布字段
	ReleaseStrategy    string  `json:"release_strategy" binding:"omitempty,oneof=full grayscale canary"`
	GrayscaleRate      float64 `json:"grayscale_rate" binding:"omitempty,min=0,max=100"`
	GrayscalePlatforms string  `json:"grayscale_platforms" binding:"omitempty,max=200"`
	GrayscaleRegions   string  `json:"grayscale_regions" binding:"omitempty,max=500"`
	GrayscaleChannels  string  `json:"grayscale_channels" binding:"omitempty,max=200"`
}

// TenantCreateVersion 创建版本
// POST /tenant/versions
// published 字段映射到 status（active/draft）
func TenantCreateVersion(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID := getTenantID(c)
		if tenantID == 0 {
			middleware.Fail(c, http.StatusForbidden, 1003, "无法识别租户身份")
			return
		}
		var req createVersionReq
		if err := c.ShouldBindJSON(&req); err != nil {
			middleware.Fail(c, http.StatusBadRequest, 1001, "参数错误: "+err.Error())
			return
		}

		if !checkAppOwnership(deps.DB, req.AppID, tenantID) {
			middleware.Fail(c, http.StatusForbidden, 1003, "应用不存在或无权访问")
			return
		}

		status := "draft"
		if req.Published {
			status = "active"
		}
		minVersion := req.MinVersion
		if minVersion == "" {
			minVersion = "0.0.0"
		}
		channel := req.Channel
		if channel == "" {
			channel = "stable"
		}

		// v0.4.0：发布策略默认 full，灰度比例默认从 sys_config 读取
		strategy := req.ReleaseStrategy
		if strategy == "" {
			strategy = grayscale.StrategyFull
		}
		rate := req.GrayscaleRate
		if strategy != grayscale.StrategyFull && rate == 0 {
			rate = grayscale.DefaultRate(c.Request.Context(), deps.CfgCache)
		}

		v := &model.AppVersion{
			TenantID:           tenantID,
			AppID:              req.AppID,
			Version:            req.Version,
			Channel:            channel,
			ReleaseStrategy:    strategy,
			GrayscaleRate:      rate,
			GrayscalePlatforms: req.GrayscalePlatforms,
			GrayscaleRegions:   req.GrayscaleRegions,
			GrayscaleChannels:  req.GrayscaleChannels,
			MinVersion:         minVersion,
			DownloadURL:        req.DownloadURL,
			BackupURL:          req.BackupURL,
			ForceUpdate:        req.ForceUpdate,
			UpdateContent:      req.UpdateLog,
			Status:             status,
		}
		if err := deps.DB.Create(v).Error; err != nil {
			logger.Error("tenant: create version failed", "err", err, "tenant_id", tenantID, "app_id", req.AppID)
			middleware.Fail(c, http.StatusInternalServerError, 5001, "创建版本失败")
			return
		}
		middleware.Success(c, v)
	}
}

// ============== 10. 更新版本（v0.4.0 新增） ==============

// updateVersionReq 更新版本请求（v0.4.0 新增）
// 不允许修改 version/app_id（避免破坏已发布客户端的版本判断）
type updateVersionReq struct {
	MinVersion         string  `json:"min_version" binding:"omitempty,max=32"`
	DownloadURL        string  `json:"download_url" binding:"omitempty,max=255"`
	BackupURL          string  `json:"backup_url" binding:"omitempty,max=255"`
	ForceUpdate        *bool   `json:"force_update"`
	UpdateLog          string  `json:"update_log" binding:"omitempty,max=5000"`
	Published          *bool   `json:"published"`
	ReleaseStrategy    string  `json:"release_strategy" binding:"omitempty,oneof=full grayscale canary"`
	GrayscaleRate      float64 `json:"grayscale_rate" binding:"omitempty,min=0,max=100"`
	GrayscalePlatforms *string `json:"grayscale_platforms" binding:"omitempty,max=200"`
	GrayscaleRegions   *string `json:"grayscale_regions" binding:"omitempty,max=500"`
	GrayscaleChannels  *string `json:"grayscale_channels" binding:"omitempty,max=200"`
}

// TenantUpdateVersion 更新版本
// PUT /tenant/versions/:id
// v0.4.0 新增：支持编辑已创建版本的灰度规则、下载地址、强制更新等
func TenantUpdateVersion(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID := getTenantID(c)
		if tenantID == 0 {
			middleware.Fail(c, http.StatusForbidden, 1003, "无法识别租户身份")
			return
		}
		id, err := strconv.ParseUint(c.Param("id"), 10, 64)
		if err != nil {
			middleware.Fail(c, http.StatusBadRequest, 1001, "ID 格式错误")
			return
		}

		// 查询版本并校验归属
		var v model.AppVersion
		if err := deps.DB.Where("id = ? AND tenant_id = ?", id, tenantID).First(&v).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				middleware.Fail(c, http.StatusNotFound, 1008, "版本不存在或无权访问")
				return
			}
			middleware.Fail(c, http.StatusInternalServerError, 5001, "查询失败")
			return
		}

		var req updateVersionReq
		if err := c.ShouldBindJSON(&req); err != nil {
			middleware.Fail(c, http.StatusBadRequest, 1001, "参数错误: "+err.Error())
			return
		}

		// 构建 updates map（仅更新非空字段）
		updates := map[string]interface{}{}
		if req.MinVersion != "" {
			updates["min_version"] = req.MinVersion
		}
		if req.DownloadURL != "" {
			updates["download_url"] = req.DownloadURL
		}
		if req.BackupURL != "" {
			updates["backup_url"] = req.BackupURL
		}
		if req.ForceUpdate != nil {
			updates["force_update"] = *req.ForceUpdate
		}
		if req.UpdateLog != "" {
			updates["update_content"] = req.UpdateLog
		}
		if req.Published != nil {
			if *req.Published {
				updates["status"] = "active"
			} else {
				updates["status"] = "draft"
			}
		}
		if req.ReleaseStrategy != "" {
			updates["release_strategy"] = req.ReleaseStrategy
			// 切换到灰度策略且未指定比例时，用默认比例
			if req.ReleaseStrategy != grayscale.StrategyFull && req.GrayscaleRate == 0 && v.GrayscaleRate == 0 {
				updates["grayscale_rate"] = grayscale.DefaultRate(c.Request.Context(), deps.CfgCache)
			}
		}
		if req.GrayscaleRate > 0 {
			updates["grayscale_rate"] = req.GrayscaleRate
		}
		if req.GrayscalePlatforms != nil {
			updates["grayscale_platforms"] = *req.GrayscalePlatforms
		}
		if req.GrayscaleRegions != nil {
			updates["grayscale_regions"] = *req.GrayscaleRegions
		}
		if req.GrayscaleChannels != nil {
			updates["grayscale_channels"] = *req.GrayscaleChannels
		}

		if len(updates) == 0 {
			middleware.Fail(c, http.StatusBadRequest, 1001, "无更新字段")
			return
		}

		if err := deps.DB.Model(&model.AppVersion{}).Where("id = ? AND tenant_id = ?", id, tenantID).
			Updates(updates).Error; err != nil {
			logger.Error("tenant: update version failed", "err", err, "id", id, "tenant_id", tenantID)
			middleware.Fail(c, http.StatusInternalServerError, 5001, "更新失败")
			return
		}

		// 重新查询返回最新数据
		var updated model.AppVersion
		_ = deps.DB.Where("id = ? AND tenant_id = ?", id, tenantID).First(&updated).Error
		middleware.Success(c, updated)
	}
}

// ============== 11. 删除版本 ==============

// TenantDeleteVersion 删除版本
// DELETE /tenant/versions/:id
func TenantDeleteVersion(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID := getTenantID(c)
		if tenantID == 0 {
			middleware.Fail(c, http.StatusForbidden, 1003, "无法识别租户身份")
			return
		}
		id, err := strconv.ParseUint(c.Param("id"), 10, 64)
		if err != nil {
			middleware.Fail(c, http.StatusBadRequest, 1001, "ID 格式错误")
			return
		}

		var v model.AppVersion
		if err := deps.DB.Where("id = ? AND tenant_id = ?", id, tenantID).First(&v).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				middleware.Fail(c, http.StatusNotFound, 1008, "版本不存在或无权访问")
				return
			}
			middleware.Fail(c, http.StatusInternalServerError, 5001, "查询失败")
			return
		}

		if err := deps.DB.Delete(&model.AppVersion{}, id).Error; err != nil {
			middleware.Fail(c, http.StatusInternalServerError, 5002, "删除失败")
			return
		}
		middleware.Success(c, gin.H{"id": id, "deleted": true})
	}
}

// ============== 11. 开发者代理列表 ==============

// agentListItem 代理列表项（嵌入 Agent + 子查询统计字段）
type agentListItem struct {
	model.Agent
	TotalCommission float64    `json:"total_commission"`
	TotalWithdraw   float64    `json:"total_withdraw"`
	FrozenBalance   float64    `json:"frozen_balance"`
	LastActiveAt    *time.Time `json:"last_active_at"`
}

// TenantListAgents 开发者代理列表
// GET /tenant/agents?page=&page_size=&keyword=&status=
func TenantListAgents(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID := getTenantID(c)
		if tenantID == 0 {
			middleware.Fail(c, http.StatusForbidden, 1003, "无法识别租户身份")
			return
		}
		page, pageSize := parsePagination(c)

		q := deps.DB.Model(&model.Agent{}).Where("tenant_id = ?", tenantID)
		if kw := c.Query("keyword"); kw != "" {
			q = q.Where("username LIKE ? OR real_name LIKE ? OR phone LIKE ?", "%"+kw+"%", "%"+kw+"%", "%"+kw+"%")
		}
		if status := c.Query("status"); status != "" {
			q = q.Where("status = ?", status)
		}

		var total int64
		q.Count(&total)

		var agents []model.Agent
		if err := q.Order("id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&agents).Error; err != nil {
			logger.Error("tenant: list agents query failed", "err", err, "tenant_id", tenantID)
			middleware.Fail(c, http.StatusInternalServerError, 5001, "查询失败")
			return
		}

		// 批量查询避免 N+1：commission / withdraw / frozen
		agentIDs := make([]uint64, 0, len(agents))
		for _, a := range agents {
			agentIDs = append(agentIDs, a.ID)
		}
		commissionMap := agentTotalCommissionBatch(deps.DB, agentIDs)
		withdrawMap := agentTotalWithdrawPaidBatch(deps.DB, agentIDs)
		frozenMap := agentFrozenBalanceBatch(deps.DB, agentIDs)

		// 后处理：子查询统计佣金/提现
		items := make([]agentListItem, 0, len(agents))
		for _, a := range agents {
			item := agentListItem{
				Agent:           a,
				LastActiveAt:    a.LastLoginAt,
				TotalCommission: commissionMap[a.ID],
				TotalWithdraw:   withdrawMap[a.ID],
				FrozenBalance:   frozenMap[a.ID],
			}
			items = append(items, item)
		}

		middleware.Success(c, gin.H{
			"list":      items,
			"total":     total,
			"page":      page,
			"page_size": pageSize,
		})
	}
}

// ============== 12. 更新代理 ==============

// updateAgentReq 更新代理请求
type updateAgentReq struct {
	Status         *string  `json:"status" binding:"omitempty,oneof=active disabled"`
	CommissionMode *string  `json:"commission_mode" binding:"omitempty,oneof=percentage diff"`
	CommissionRate *float64 `json:"commission_rate" binding:"omitempty,min=0,max=100"`
}

// TenantUpdateAgent 更新代理
// PUT /tenant/agents/:id
func TenantUpdateAgent(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID := getTenantID(c)
		if tenantID == 0 {
			middleware.Fail(c, http.StatusForbidden, 1003, "无法识别租户身份")
			return
		}
		id, err := strconv.ParseUint(c.Param("id"), 10, 64)
		if err != nil {
			middleware.Fail(c, http.StatusBadRequest, 1001, "ID 格式错误")
			return
		}

		var req updateAgentReq
		if err := c.ShouldBindJSON(&req); err != nil {
			middleware.Fail(c, http.StatusBadRequest, 1001, "参数错误: "+err.Error())
			return
		}

		var agent model.Agent
		if err := deps.DB.Where("id = ? AND tenant_id = ?", id, tenantID).First(&agent).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				middleware.Fail(c, http.StatusNotFound, 1008, "代理不存在或无权访问")
				return
			}
			middleware.Fail(c, http.StatusInternalServerError, 5001, "查询失败")
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

		if len(updates) == 0 {
			middleware.Fail(c, http.StatusBadRequest, 1001, "未提交任何更新字段")
			return
		}

		if err := deps.DB.Model(&model.Agent{}).Where("id = ? AND tenant_id = ?", id, tenantID).
			Updates(updates).Error; err != nil {
			logger.Error("tenant: update agent failed", "err", err, "id", id, "tenant_id", tenantID)
			middleware.Fail(c, http.StatusInternalServerError, 5002, "更新失败")
			return
		}
		middleware.Success(c, gin.H{"id": id, "updated": true})
	}
}

// ============== 13. 生成邀请码 ==============

// genInviteCodeReq 生成邀请码请求
type genInviteCodeReq struct {
	Count      int    `json:"count" binding:"omitempty,min=1,max=100"`
	ExpireDays int    `json:"expire_days" binding:"omitempty,min=1,max=365"`
	Remark     string `json:"remark" binding:"omitempty,max=255"`
}

// TenantGenInviteCode 生成邀请码
// POST /tenant/agents/invite_codes
func TenantGenInviteCode(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID := getTenantID(c)
		userID := getUserID(c)
		if tenantID == 0 {
			middleware.Fail(c, http.StatusForbidden, 1003, "无法识别租户身份")
			return
		}

		var req genInviteCodeReq
		if err := c.ShouldBindJSON(&req); err != nil {
			middleware.Fail(c, http.StatusBadRequest, 1001, "参数错误: "+err.Error())
			return
		}
		if req.Count == 0 {
			req.Count = 1
		}
		if req.ExpireDays == 0 {
			req.ExpireDays = 30
		}

		// 校验套餐配额（代理数上限）—— v0.3.5：抽到 quota 包统一管理
		// 注：邀请码本身不是代理，但生成邀请码隐含招募代理意图，提前校验避免发放无效邀请码
		if err := quota.CheckMaxAgents(deps.DB, tenantID); err != nil {
			var qErr *quota.ExceededError
			if errors.As(err, &qErr) {
				if qErr.Limit == 0 {
					middleware.Fail(c, http.StatusForbidden, 1007, "当前套餐不支持招募代理，请升级套餐")
				} else {
					middleware.Fail(c, http.StatusForbidden, 1007,
						"已达套餐代理数上限 "+itoa(qErr.Limit)+" 个，无法生成新邀请码")
				}
			} else {
				logger.Error("tenant: quota check agents failed", "err", err, "tenant_id", tenantID)
				middleware.Fail(c, http.StatusForbidden, 1007, "代理配额校验失败")
			}
			return
		}

		ctx := c.Request.Context()
		// 默认佣金比例从 sys_config 读取（铁律 05）
		defaultRate := deps.CfgCache.GetFloat64(ctx, "agent.default_commission_rate", 10.00)

		// Bug 5 P0：前端 InviteCodes.vue 期望 codes 为对象数组（含 code 字段），而非 []string
		type genInviteCodeItem struct {
			Code       string    `json:"code"`
			MaxUses    int       `json:"max_uses"`
			UsedCount  int       `json:"used_count"`
			ValidDays  int       `json:"valid_days"`
			ExpiresAt  time.Time `json:"expires_at"`
			Status     string    `json:"status"`
			Commission float64   `json:"commission_rate"`
		}
		codes := make([]genInviteCodeItem, 0, req.Count)
		expiresAt := time.Now().AddDate(0, 0, req.ExpireDays)

		txErr := deps.DB.Transaction(func(tx *gorm.DB) error {
			for i := 0; i < req.Count; i++ {
				code, err := genInviteCodeUnique(tx)
				if err != nil {
					return fmt.Errorf("生成第 %d 个邀请码失败: %w", i+1, err)
				}
				ic := &model.AgentInviteCode{
					TenantID:              tenantID,
					Code:                  code,
					MaxUses:               1,
					UsedCount:             0,
					ValidDays:             req.ExpireDays,
					ExpiresAt:             expiresAt,
					Status:                "active",
					DefaultCommissionRate: defaultRate,
					CreatedBy:             userID,
				}
				if err := tx.Create(ic).Error; err != nil {
					return fmt.Errorf("入库第 %d 个邀请码失败: %w", i+1, err)
				}
				codes = append(codes, genInviteCodeItem{
					Code:       code,
					MaxUses:    ic.MaxUses,
					UsedCount:  ic.UsedCount,
					ValidDays:  ic.ValidDays,
					ExpiresAt:  ic.ExpiresAt,
					Status:     ic.Status,
					Commission: ic.DefaultCommissionRate,
				})
			}
			return nil
		})
		if txErr != nil {
			middleware.Fail(c, http.StatusInternalServerError, 5003, "批量生成失败: "+txErr.Error())
			return
		}

		middleware.Success(c, gin.H{"codes": codes})
	}
}

// ============== 14. 邀请码列表 ==============

// inviteCodeListItem 邀请码列表项（嵌入 AgentInviteCode + 联表 used_by_username）
type inviteCodeListItem struct {
	model.AgentInviteCode
	UsedByUsername string `json:"used_by_username"`
}

// TenantListInviteCodes 邀请码列表
// GET /tenant/invite_codes?page=&page_size=&status=
// 前端按 used_count 判定 unused/used 状态
func TenantListInviteCodes(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID := getTenantID(c)
		if tenantID == 0 {
			middleware.Fail(c, http.StatusForbidden, 1003, "无法识别租户身份")
			return
		}
		page, pageSize := parsePagination(c)

		q := deps.DB.Table("agent_invite_code").
			Select("agent_invite_code.*, agent.username as used_by_username").
			Joins("LEFT JOIN agent ON agent.id = agent_invite_code.used_by_agent_id").
			Where("agent_invite_code.tenant_id = ?", tenantID)
		if status := c.Query("status"); status != "" {
			q = q.Where("agent_invite_code.status = ?", status)
		}

		var total int64
		q.Count(&total)

		var items []inviteCodeListItem
		if err := q.Order("agent_invite_code.id DESC").
			Offset((page - 1) * pageSize).Limit(pageSize).Scan(&items).Error; err != nil {
			logger.Error("tenant: list invite codes query failed", "err", err, "tenant_id", tenantID)
			middleware.Fail(c, http.StatusInternalServerError, 5001, "查询失败")
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

// ============== 15. 禁用邀请码 ==============

// TenantDisableInviteCode 禁用邀请码
// POST /tenant/invite_codes/:id/disable
func TenantDisableInviteCode(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID := getTenantID(c)
		if tenantID == 0 {
			middleware.Fail(c, http.StatusForbidden, 1003, "无法识别租户身份")
			return
		}
		id, err := strconv.ParseUint(c.Param("id"), 10, 64)
		if err != nil {
			middleware.Fail(c, http.StatusBadRequest, 1001, "ID 格式错误")
			return
		}

		var ic model.AgentInviteCode
		if err := deps.DB.Where("id = ? AND tenant_id = ?", id, tenantID).First(&ic).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				middleware.Fail(c, http.StatusNotFound, 1008, "邀请码不存在或无权访问")
				return
			}
			middleware.Fail(c, http.StatusInternalServerError, 5001, "查询失败")
			return
		}

		if err := deps.DB.Model(&model.AgentInviteCode{}).Where("id = ?", id).
			Update("status", "disabled").Error; err != nil {
			middleware.Fail(c, http.StatusInternalServerError, 5002, "禁用失败")
			return
		}
		middleware.Success(c, gin.H{"id": id, "status": "disabled"})
	}
}

// ============== 16. 开发者支付配置列表 ==============

// payConfigDetail 支付配置详情（解密后返回给前端）
type payConfigDetail struct {
	PID       string `json:"pid"`
	Key       string `json:"key"`
	APIURL    string `json:"api_url"`
	NotifyURL string `json:"notify_url"`
	ReturnURL string `json:"return_url"`
}

// payConfigItem 支付配置列表项
type payConfigItem struct {
	ID        uint64          `json:"id"`
	TenantID  uint64          `json:"tenant_id"` // Bug 15 P1：前端 TenantPayConfig.tenant_id 期望字段
	Channel   string          `json:"channel"`
	Config    payConfigDetail `json:"config"`
	Status    string          `json:"status"`     // active/disabled
	CreatedAt string          `json:"created_at"` // Bug 15 P1：前端期望字段
	UpdatedAt string          `json:"updated_at"` // Bug 15 P1：前端期望字段
}

// TenantListPayConfig 开发者支付配置列表
// GET /tenant/pay_config
func TenantListPayConfig(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID := getTenantID(c)
		if tenantID == 0 {
			middleware.Fail(c, http.StatusForbidden, 1003, "无法识别租户身份")
			return
		}

		var configs []model.TenantPayConfig
		if err := deps.DB.Where("tenant_id = ?", tenantID).Find(&configs).Error; err != nil {
			logger.Error("tenant: list pay config query failed", "err", err, "tenant_id", tenantID)
			middleware.Fail(c, http.StatusInternalServerError, 5001, "查询失败")
			return
		}

		items := make([]payConfigItem, 0, len(configs))
		for _, cfg := range configs {
			// 解密 key_encrypted 为 config.key 返回
			keyPlain, err := deps.Crypto.DecryptAES(cfg.KeyEncrypted)
			if err != nil {
				// 待核实：解密失败是否应整体返回错误，当前降级为空 key
				keyPlain = ""
			}
			status := "disabled"
			if cfg.Enabled {
				status = "active"
			}
			items = append(items, payConfigItem{
				ID:       cfg.ID,
				TenantID: cfg.TenantID,
				Channel:  cfg.Channel,
				Config: payConfigDetail{
					PID:       cfg.PID,
					Key:       keyPlain,
					APIURL:    cfg.GatewayURL,
					NotifyURL: cfg.NotifyPath,
					ReturnURL: cfg.ReturnPath,
				},
				Status:    status,
				CreatedAt: cfg.CreatedAt.Format("2006-01-02 15:04:05"),
				UpdatedAt: cfg.UpdatedAt.Format("2006-01-02 15:04:05"),
			})
		}

		middleware.Success(c, gin.H{"list": items})
	}
}

// ============== 17. 保存支付配置 ==============

// savePayConfigReq 保存支付配置请求
type savePayConfigReq struct {
	Channel string          `json:"channel" binding:"required,oneof=epay wechat alipay"`
	Config  payConfigDetail `json:"config"`
	Status  string          `json:"status" binding:"omitempty,oneof=active disabled"`
}

// TenantSavePayConfig 保存支付配置
// POST /tenant/pay_config
// 按 (tenant_id, channel) upsert，key 加密入库
// v0.4.x 收尾项 D：enabled 状态变更时通知该开发者名下所有代理（站内信 + 公告）
func TenantSavePayConfig(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID := getTenantID(c)
		if tenantID == 0 {
			middleware.Fail(c, http.StatusForbidden, 1003, "无法识别租户身份")
			return
		}

		var req savePayConfigReq
		if err := c.ShouldBindJSON(&req); err != nil {
			middleware.Fail(c, http.StatusBadRequest, 1001, "参数错误: "+err.Error())
			return
		}

		// 加密 key
		keyEnc, err := deps.Crypto.EncryptAES(req.Config.Key)
		if err != nil {
			logger.Error("tenant: encrypt pay config key failed", "err", err, "tenant_id", tenantID)
			middleware.Fail(c, http.StatusInternalServerError, 5001, "加密 key 失败")
			return
		}
		enabled := req.Status == "active"

		// 按 (tenant_id, channel) upsert
		// v0.4.x 收尾项 D：先记录旧 enabled 状态，upsert 后比较以决定是否通知代理
		var cfg model.TenantPayConfig
		oldEnabled := false
		found := false
		result := deps.DB.Where("tenant_id = ? AND channel = ?", tenantID, req.Channel).First(&cfg)
		if result.Error == gorm.ErrRecordNotFound {
			cfg = model.TenantPayConfig{
				TenantID:     tenantID,
				Channel:      req.Channel,
				Enabled:      enabled,
				GatewayURL:   req.Config.APIURL,
				PID:          req.Config.PID,
				KeyEncrypted: keyEnc,
				NotifyPath:   req.Config.NotifyURL,
				ReturnPath:   req.Config.ReturnURL,
			}
			if err := deps.DB.Create(&cfg).Error; err != nil {
				logger.Error("tenant: create pay config failed", "err", err, "tenant_id", tenantID, "channel", req.Channel)
				middleware.Fail(c, http.StatusInternalServerError, 5002, "创建支付配置失败")
				return
			}
		} else if result.Error != nil {
			logger.Error("tenant: query pay config failed", "err", result.Error, "tenant_id", tenantID, "channel", req.Channel)
			middleware.Fail(c, http.StatusInternalServerError, 5001, "查询失败")
			return
		} else {
			oldEnabled = cfg.Enabled
			found = true
			updates := map[string]interface{}{
				"enabled":       enabled,
				"gateway_url":   req.Config.APIURL,
				"pid":           req.Config.PID,
				"key_encrypted": keyEnc,
				"notify_path":   req.Config.NotifyURL,
				"return_path":   req.Config.ReturnURL,
			}
			if err := deps.DB.Model(&model.TenantPayConfig{}).Where("id = ?", cfg.ID).Updates(updates).Error; err != nil {
				logger.Error("tenant: update pay config failed", "err", err, "id", cfg.ID)
				middleware.Fail(c, http.StatusInternalServerError, 5002, "更新支付配置失败")
				return
			}
		}

		// v0.4.x 收尾项 D：检测 enabled 状态变更，通知该开发者名下所有代理
		// 新建记录（!found）或 enabled 状态变化时触发通知；通知失败不阻塞主流程
		if !found || oldEnabled != enabled {
			notifyPayModeChanged(deps, c.Request.Context(), tenantID, req.Channel, enabled)
		}

		middleware.Success(c, gin.H{"id": cfg.ID, "channel": req.Channel, "saved": true})
	}
}

// notifyPayModeChanged 通知开发者名下所有代理支付通道状态已变更
// 通知失败仅记录日志，不阻塞主流程
// 严格遵循铁律 04：开发者名优先用 company，其次 username（sys_tenant 无 name 字段）
func notifyPayModeChanged(deps *Deps, ctx context.Context, tenantID uint64, channel string, enabled bool) {
	var tenant model.SysTenant
	if err := deps.DB.First(&tenant, tenantID).Error; err != nil {
		logger.Error("pay_mode_changed: query tenant failed",
			"err", err, "tenant_id", tenantID, "channel", channel)
		return
	}
	tenantName := tenant.Company
	if tenantName == "" {
		tenantName = tenant.Username
	}

	action := "enabled"
	if !enabled {
		action = "disabled"
	}

	variables := map[string]interface{}{
		"tenant_name": tenantName,
		"channel":     channel,
		"action":      action,
		"time":        time.Now().Format("2006-01-02 15:04:05"),
	}

	if err := notify.NotifyAgentsByTenant(ctx, deps.DB, tenantID, variables); err != nil {
		logger.Error("pay_mode_changed: notify agents failed",
			"err", err, "tenant_id", tenantID, "channel", channel)
	}
}

// ============== 18. 测试支付配置 ==============

// TenantTestPayConfig 测试支付配置
// POST /tenant/pay_config/:id/test
// 简化：仅校验配置完整性（gateway_url/pid/key 非空）+ 解密成功
func TenantTestPayConfig(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID := getTenantID(c)
		if tenantID == 0 {
			middleware.Fail(c, http.StatusForbidden, 1003, "无法识别租户身份")
			return
		}
		id, err := strconv.ParseUint(c.Param("id"), 10, 64)
		if err != nil {
			middleware.Fail(c, http.StatusBadRequest, 1001, "ID 格式错误")
			return
		}

		var cfg model.TenantPayConfig
		if err := deps.DB.Where("id = ? AND tenant_id = ?", id, tenantID).First(&cfg).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				middleware.Fail(c, http.StatusNotFound, 1008, "支付配置不存在或无权访问")
				return
			}
			middleware.Fail(c, http.StatusInternalServerError, 5001, "查询失败")
			return
		}

		success := true
		message := "OK"
		if cfg.GatewayURL == "" || cfg.PID == "" || cfg.KeyEncrypted == "" {
			success = false
			message = "配置不完整：gateway_url/pid/key 不能为空"
		} else if _, err := deps.Crypto.DecryptAES(cfg.KeyEncrypted); err != nil {
			logger.Error("tenant: pay config key decrypt failed", "err", err, "config_id", cfg.ID)
			success = false
			message = "key 解密失败，请重新配置"
		}

		// 记录测试结果（简化）
		resultStr := "success"
		if !success {
			resultStr = "failed"
		}
		_ = deps.DB.Model(&model.TenantPayConfig{}).Where("id = ?", id).
			Updates(map[string]interface{}{
				"last_test_at":     time.Now(),
				"last_test_result": resultStr,
			}).Error

		middleware.Success(c, gin.H{"success": success, "message": message})
	}
}

// ============== 19. 开发者公告列表 ==============

// noticeListItem 公告列表项（嵌入 Notice + 暴露字段名按前端规范）
type noticeListItem struct {
	model.Notice
	// 冗余字段，便于前端直接消费
	Pinned    bool   `json:"pinned"`     // 映射 IsPinned
	PublishAt string `json:"publish_at"` // 映射 StartAt
	ExpireAt  string `json:"expire_at"`  // 映射 EndAt
}

// TenantListNotices 开发者公告列表
// GET /tenant/notices?page=&page_size=&type=&status=
// 列表查询 type IN ('tenant', 'agent', 'h5')，默认仅返回开发者向公告与代理通知
func TenantListNotices(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID := getTenantID(c)
		if tenantID == 0 {
			middleware.Fail(c, http.StatusForbidden, 1003, "无法识别租户身份")
			return
		}
		page, pageSize := parsePagination(c)

		q := deps.DB.Model(&model.Notice{}).Where("tenant_id = ?", tenantID)

		if t := c.Query("type"); t != "" {
			q = q.Where("type = ?", t)
		} else {
			// 默认仅返回开发者向公告与代理通知
			q = q.Where("type IN ?", []string{"tenant", "agent", "h5"})
		}
		if status := c.Query("status"); status != "" {
			q = q.Where("status = ?", status)
		}

		var total int64
		q.Count(&total)

		var notices []noticeListItem
		if err := q.Order("is_pinned DESC, sort DESC, id DESC").
			Offset((page - 1) * pageSize).Limit(pageSize).
			Find(&notices).Error; err != nil {
			logger.Error("tenant: list notices query failed", "err", err, "tenant_id", tenantID)
			middleware.Fail(c, http.StatusInternalServerError, 5001, "查询失败")
			return
		}

		// 后处理填充冗余字段
		for i := range notices {
			notices[i].Pinned = notices[i].IsPinned
			notices[i].PublishAt = notices[i].StartAt.Format("2006-01-02 15:04:05")
			if notices[i].EndAt != nil {
				notices[i].ExpireAt = notices[i].EndAt.Format("2006-01-02 15:04:05")
			}
		}

		middleware.Success(c, gin.H{
			"list":      notices,
			"total":     total,
			"page":      page,
			"page_size": pageSize,
		})
	}
}

// ============== 20. 创建公告 ==============

// tenantCreateNoticeReq 创建公告请求 DTO
type tenantCreateNoticeReq struct {
	Type          string `json:"type" binding:"required,oneof=tenant agent h5"`
	Title         string `json:"title" binding:"required,max=255"`
	Content       string `json:"content" binding:"required"`
	ContentFormat string `json:"content_format" binding:"omitempty,oneof=text html"`
	Status        string `json:"status" binding:"omitempty,oneof=draft published offline"`
	Pinned        bool   `json:"pinned"`
	IsPopup       bool   `json:"is_popup"`
	ShowBadge     *bool  `json:"show_badge"`
	Sort          int    `json:"sort" binding:"omitempty,min=0"`
	PublishAt     string `json:"publish_at"` // 映射到 start_at
	ExpireAt      string `json:"expire_at"`  // 映射到 end_at
}

// TenantCreateNotice 创建公告
// POST /tenant/notices
// 字段 type(tenant/agent/h5)
func TenantCreateNotice(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID := getTenantID(c)
		if tenantID == 0 {
			middleware.Fail(c, http.StatusForbidden, 1003, "无法识别租户身份")
			return
		}
		userID := getUserID(c)
		var req tenantCreateNoticeReq
		if err := c.ShouldBindJSON(&req); err != nil {
			middleware.Fail(c, http.StatusBadRequest, 1001, "参数错误: "+err.Error())
			return
		}

		// publish_at 解析，默认 now
		var startAt time.Time
		if req.PublishAt != "" {
			if t, err := time.ParseInLocation("2006-01-02 15:04:05", req.PublishAt, time.Local); err == nil {
				startAt = t
			} else if t, err := time.ParseInLocation("2006-01-02", req.PublishAt, time.Local); err == nil {
				startAt = t
			} else {
				middleware.Fail(c, http.StatusBadRequest, 1001, "publish_at 格式错误，应为 YYYY-MM-DD HH:MM:SS 或 YYYY-MM-DD")
				return
			}
		} else {
			startAt = time.Now()
		}

		// expire_at 解析
		var endAt *time.Time
		if req.ExpireAt != "" {
			if t, err := time.ParseInLocation("2006-01-02 15:04:05", req.ExpireAt, time.Local); err == nil {
				endAt = &t
			} else if t, err := time.ParseInLocation("2006-01-02", req.ExpireAt, time.Local); err == nil {
				endAt = &t
			} else {
				middleware.Fail(c, http.StatusBadRequest, 1001, "expire_at 格式错误，应为 YYYY-MM-DD HH:MM:SS 或 YYYY-MM-DD")
				return
			}
		}

		// status 默认 draft
		status := req.Status
		if status == "" {
			status = "draft"
		}
		ctx := c.Request.Context()

		// v0.4.0：content_format 默认 text，富文本需 richtext.enabled=1
		contentFormat := req.ContentFormat
		if contentFormat == "" {
			contentFormat = "text"
		}
		if contentFormat == "html" && deps.CfgCache != nil {
			if !deps.CfgCache.GetBool(ctx, "notice.richtext.enabled", true) {
				middleware.Fail(c, http.StatusBadRequest, 1001, "富文本编辑功能已禁用")
				return
			}
		}
		if deps.CfgCache != nil {
			maxLen := deps.CfgCache.GetInt(ctx, "notice.richtext.max_length", 10000)
			if maxLen > 0 && len(req.Content) > maxLen {
				middleware.Fail(c, http.StatusBadRequest, 1001, "内容超过最大长度限制")
				return
			}
		}
		// v0.4.0：ShowBadge 默认 true
		showBadge := true
		if req.ShowBadge != nil {
			showBadge = *req.ShowBadge
		}

		notice := model.Notice{
			Type:          req.Type,
			TenantID:      &tenantID,
			Title:         req.Title,
			Content:       req.Content,
			ContentFormat: contentFormat,
			IsPinned:      req.Pinned,
			IsPopup:       req.IsPopup,
			ShowBadge:     showBadge,
			Sort:          req.Sort,
			StartAt:       startAt,
			EndAt:         endAt,
			Status:        status,
			ViewCount:     0,
			CreatedBy:     userID,
		}
		if err := deps.DB.Create(&notice).Error; err != nil {
			logger.Error("tenant: create notice failed", "err", err, "tenant_id", tenantID, "title", req.Title)
			middleware.Fail(c, http.StatusInternalServerError, 5002, "创建公告失败")
			return
		}

		middleware.Success(c, gin.H{
			"id":         notice.ID,
			"type":       notice.Type,
			"title":      notice.Title,
			"status":     notice.Status,
			"created_at": notice.CreatedAt,
		})
	}
}

// ============== 21. 更新 / 删除公告 ==============

// tenantUpdateNoticeReq 更新公告请求 DTO（所有字段可选）
type tenantUpdateNoticeReq struct {
	Title         *string `json:"title" binding:"omitempty,max=255"`
	Content       *string `json:"content" binding:"omitempty"`
	ContentFormat *string `json:"content_format" binding:"omitempty,oneof=text html"`
	Status        *string `json:"status" binding:"omitempty,oneof=draft published offline"`
	Pinned        *bool   `json:"pinned"`
	IsPopup       *bool   `json:"is_popup"`
	ShowBadge     *bool   `json:"show_badge"`
	Sort          *int    `json:"sort" binding:"omitempty,min=0"`
	PublishAt     *string `json:"publish_at"` // 映射 start_at
	ExpireAt      *string `json:"expire_at"`  // 映射 end_at
}

// TenantUpdateNotice 更新公告
// PUT /tenant/notices/:id
// 部分更新：title/content/status/pinned/publish_at/expire_at
func TenantUpdateNotice(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID := getTenantID(c)
		if tenantID == 0 {
			middleware.Fail(c, http.StatusForbidden, 1003, "无法识别租户身份")
			return
		}
		id, err := strconv.ParseUint(c.Param("id"), 10, 64)
		if err != nil {
			middleware.Fail(c, http.StatusBadRequest, 1001, "公告 ID 格式错误")
			return
		}

		var notice model.Notice
		if err := deps.DB.Where("id = ? AND tenant_id = ?", id, tenantID).First(&notice).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				middleware.Fail(c, http.StatusNotFound, 1008, "公告不存在或无权访问")
				return
			}
			middleware.Fail(c, http.StatusInternalServerError, 5001, "查询失败")
			return
		}

		var req tenantUpdateNoticeReq
		if err := c.ShouldBindJSON(&req); err != nil {
			middleware.Fail(c, http.StatusBadRequest, 1001, "参数错误: "+err.Error())
			return
		}
		ctx := c.Request.Context()

		// v0.4.0：富文本校验
		if req.ContentFormat != nil && *req.ContentFormat == "html" && deps.CfgCache != nil {
			if !deps.CfgCache.GetBool(ctx, "notice.richtext.enabled", true) {
				middleware.Fail(c, http.StatusBadRequest, 1001, "富文本编辑功能已禁用")
				return
			}
		}
		if req.Content != nil && deps.CfgCache != nil {
			maxLen := deps.CfgCache.GetInt(ctx, "notice.richtext.max_length", 10000)
			if maxLen > 0 && len(*req.Content) > maxLen {
				middleware.Fail(c, http.StatusBadRequest, 1001, "内容超过最大长度限制")
				return
			}
		}

		updates := map[string]interface{}{}
		if req.Title != nil {
			updates["title"] = *req.Title
		}
		if req.Content != nil {
			updates["content"] = *req.Content
		}
		if req.ContentFormat != nil {
			updates["content_format"] = *req.ContentFormat
		}
		if req.Status != nil {
			updates["status"] = *req.Status
		}
		if req.Pinned != nil {
			updates["is_pinned"] = *req.Pinned
		}
		if req.IsPopup != nil {
			updates["is_popup"] = *req.IsPopup
		}
		if req.ShowBadge != nil {
			updates["show_badge"] = *req.ShowBadge
		}
		if req.Sort != nil {
			updates["sort"] = *req.Sort
		}
		if req.PublishAt != nil {
			if t, err := time.ParseInLocation("2006-01-02 15:04:05", *req.PublishAt, time.Local); err == nil {
				updates["start_at"] = t
			} else if t, err := time.ParseInLocation("2006-01-02", *req.PublishAt, time.Local); err == nil {
				updates["start_at"] = t
			} else {
				middleware.Fail(c, http.StatusBadRequest, 1001, "publish_at 格式错误")
				return
			}
		}
		if req.ExpireAt != nil {
			if t, err := time.ParseInLocation("2006-01-02 15:04:05", *req.ExpireAt, time.Local); err == nil {
				updates["end_at"] = t
			} else if t, err := time.ParseInLocation("2006-01-02", *req.ExpireAt, time.Local); err == nil {
				updates["end_at"] = t
			} else {
				middleware.Fail(c, http.StatusBadRequest, 1001, "expire_at 格式错误")
				return
			}
		}

		if len(updates) == 0 {
			middleware.Fail(c, http.StatusBadRequest, 1001, "无可更新字段")
			return
		}
		if err := deps.DB.Model(&model.Notice{}).Where("id = ? AND tenant_id = ?", id, tenantID).
			Updates(updates).Error; err != nil {
			logger.Error("tenant: update notice failed", "err", err, "id", id, "tenant_id", tenantID)
			middleware.Fail(c, http.StatusInternalServerError, 5002, "更新公告失败")
			return
		}

		middleware.Success(c, gin.H{"id": id, "updated": true})
	}
}

// TenantDeleteNotice 删除公告
// DELETE /tenant/notices/:id
// 硬删除（Notice 模型无 gorm.DeletedAt 软删除字段）
func TenantDeleteNotice(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID := getTenantID(c)
		if tenantID == 0 {
			middleware.Fail(c, http.StatusForbidden, 1003, "无法识别租户身份")
			return
		}
		id, err := strconv.ParseUint(c.Param("id"), 10, 64)
		if err != nil {
			middleware.Fail(c, http.StatusBadRequest, 1001, "公告 ID 格式错误")
			return
		}

		// 先校验归属权，再硬删除
		var notice model.Notice
		if err := deps.DB.Where("id = ? AND tenant_id = ?", id, tenantID).First(&notice).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				middleware.Fail(c, http.StatusNotFound, 1008, "公告不存在或无权访问")
				return
			}
			middleware.Fail(c, http.StatusInternalServerError, 5001, "查询失败")
			return
		}

		// 事务删除：先删关联的 NoticeTarget / NoticeRead，再删 Notice 本身，避免脏数据
		deps.DB.Where("notice_id = ?", id).Delete(&model.NoticeRead{})
		deps.DB.Where("notice_id = ?", id).Delete(&model.NoticeTarget{})
		if err := deps.DB.Delete(&model.Notice{}, id).Error; err != nil {
			logger.Error("tenant: delete notice failed", "err", err, "id", id, "tenant_id", tenantID)
			middleware.Fail(c, http.StatusInternalServerError, 5002, "删除公告失败")
			return
		}

		middleware.Success(c, gin.H{"id": id, "deleted": true})
	}
}

// ============== v0.4.0 多级代理：代理树查询 ==============

// TenantGetAgentTree GET /api/v1/tenant/agents/:id/tree
// 开发者查询指定代理的下级代理树（递归，最深 max_level-1 层）
func TenantGetAgentTree(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID := getTenantID(c)
		agentID, err := parseUintParam(c, "id")
		if err != nil || agentID == 0 {
			middleware.Fail(c, http.StatusBadRequest, 1001, "代理 ID 无效")
			return
		}

		// 校验：代理必须属于当前开发者
		var agent model.Agent
		if err := deps.DB.Where("id = ? AND tenant_id = ?", agentID, tenantID).First(&agent).Error; err != nil {
			middleware.Fail(c, http.StatusNotFound, 1004, "代理不存在或无权访问")
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
			logger.Error("tenant: build agent tree failed", "err", err, "agent_id", agentID, "tenant_id", tenantID)
			middleware.Fail(c, http.StatusInternalServerError, 5002, "构建代理树失败")
			return
		}

		middleware.Success(c, gin.H{
			"tree": tree,
		})
	}
}

// ============== v0.4.x D-15 开发者安全设置（IP 黑名单 + 频率限制） ==============

// tenantUpdateSecurityReq 安全配置更新请求体
type tenantUpdateSecurityReq struct {
	IPBlacklist           []string `json:"ip_blacklist" binding:"omitempty,dive,max=45"`                  // IP 或 CIDR 数组
	VerifyRateLimitPerMin *int     `json:"verify_rate_limit_per_min" binding:"omitempty,min=0,max=10000"` // 0=不限
	LoginRateLimitPerMin  *int     `json:"login_rate_limit_per_min" binding:"omitempty,min=0,max=10000"`
}

// TenantGetSecurity GET /api/v1/tenant/security
// 返回当前开发者安全配置（无记录则返回空配置）
func TenantGetSecurity(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID := getTenantID(c)
		if tenantID == 0 {
			middleware.Fail(c, http.StatusForbidden, 1003, "无法识别租户身份")
			return
		}

		var sec model.TenantSecurityConfig
		err := deps.DB.Where("tenant_id = ?", tenantID).First(&sec).Error
		if err != nil {
			if err == gorm.ErrRecordNotFound {
				// 无配置：返回空安全配置（向后兼容）
				middleware.Success(c, gin.H{
					"tenant_id":                 tenantID,
					"ip_blacklist":              []string{},
					"verify_rate_limit_per_min": 0,
					"login_rate_limit_per_min":  0,
					"updated_at":                nil,
				})
				return
			}
			middleware.Fail(c, http.StatusInternalServerError, 5001, "查询安全配置失败")
			return
		}

		// 解析 ip_blacklist JSON 数组
		ipList := []string{}
		if sec.IPBlacklist != "" && sec.IPBlacklist != "[]" {
			_ = json.Unmarshal([]byte(sec.IPBlacklist), &ipList)
		}

		middleware.Success(c, gin.H{
			"tenant_id":                 sec.TenantID,
			"ip_blacklist":              ipList,
			"verify_rate_limit_per_min": sec.VerifyRateLimitPerMin,
			"login_rate_limit_per_min":  sec.LoginRateLimitPerMin,
			"updated_at":                sec.UpdatedAt,
		})
	}
}

// TenantUpdateSecurity PUT /api/v1/tenant/security
// 事务：upsert tenant_security_config 记录
func TenantUpdateSecurity(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID := getTenantID(c)
		if tenantID == 0 {
			middleware.Fail(c, http.StatusForbidden, 1003, "无法识别租户身份")
			return
		}

		var req tenantUpdateSecurityReq
		if err := c.ShouldBindJSON(&req); err != nil {
			middleware.Fail(c, http.StatusBadRequest, 1001, "参数错误: "+err.Error())
			return
		}

		// 序列化 IP 黑名单为 JSON 数组（空数组也存 "[]"，避免 NOT NULL 约束冲突）
		ipBlacklistJSON := "[]"
		if len(req.IPBlacklist) > 0 {
			// 去重 + 去空 + 校验格式
			seen := make(map[string]bool, len(req.IPBlacklist))
			deduped := make([]string, 0, len(req.IPBlacklist))
			for _, entry := range req.IPBlacklist {
				entry = strings.TrimSpace(entry)
				if entry == "" || seen[entry] {
					continue
				}
				// 格式校验：单 IP 或 CIDR
				if !isValidIPEntry(entry) {
					middleware.Fail(c, http.StatusBadRequest, 1001, "IP 黑名单格式错误: "+entry)
					return
				}
				seen[entry] = true
				deduped = append(deduped, entry)
			}
			if len(deduped) > 0 {
				b, _ := json.Marshal(deduped)
				ipBlacklistJSON = string(b)
			}
		}

		// 查现有记录
		var sec model.TenantSecurityConfig
		notFound := false
		if err := deps.DB.Where("tenant_id = ?", tenantID).First(&sec).Error; err != nil {
			if err != gorm.ErrRecordNotFound {
				middleware.Fail(c, http.StatusInternalServerError, 5001, "查询安全配置失败")
				return
			}
			notFound = true
			sec = model.TenantSecurityConfig{
				TenantID:    tenantID,
				IPBlacklist: "[]",
			}
		}

		// 合并更新字段
		updates := map[string]interface{}{
			"ip_blacklist": ipBlacklistJSON,
		}
		if req.VerifyRateLimitPerMin != nil {
			updates["verify_rate_limit_per_min"] = *req.VerifyRateLimitPerMin
		}
		if req.LoginRateLimitPerMin != nil {
			updates["login_rate_limit_per_min"] = *req.LoginRateLimitPerMin
		}

		if notFound {
			// INSERT：填入默认值 + 用户提交值
			sec.IPBlacklist = ipBlacklistJSON
			if req.VerifyRateLimitPerMin != nil {
				sec.VerifyRateLimitPerMin = *req.VerifyRateLimitPerMin
			}
			if req.LoginRateLimitPerMin != nil {
				sec.LoginRateLimitPerMin = *req.LoginRateLimitPerMin
			}
			if err := deps.DB.Create(&sec).Error; err != nil {
				logger.Error("tenant: save security config failed", "err", err, "tenant_id", tenantID)
				middleware.Fail(c, http.StatusInternalServerError, 5001, "保存安全配置失败")
				return
			}
		} else {
			if err := deps.DB.Model(&sec).Updates(updates).Error; err != nil {
				logger.Error("tenant: update security config failed", "err", err, "tenant_id", tenantID)
				middleware.Fail(c, http.StatusInternalServerError, 5001, "更新安全配置失败")
				return
			}
		}

		RecordOperation(deps, c, "tenant_security", "update", "success", "tenant_security_config", &tenantID, map[string]interface{}{
			"ip_blacklist_count":        len(req.IPBlacklist),
			"verify_rate_limit_per_min": req.VerifyRateLimitPerMin,
			"login_rate_limit_per_min":  req.LoginRateLimitPerMin,
		})

		middleware.Success(c, gin.H{
			"tenant_id":                 tenantID,
			"ip_blacklist":              req.IPBlacklist,
			"verify_rate_limit_per_min": updates["verify_rate_limit_per_min"],
			"login_rate_limit_per_min":  updates["login_rate_limit_per_min"],
		})
	}
}

// isValidIPEntry 校验 IP 或 CIDR 格式
func isValidIPEntry(entry string) bool {
	if entry == "" {
		return false
	}
	// 单 IP
	if !strings.Contains(entry, "/") {
		if ip := net.ParseIP(entry); ip != nil {
			return true
		}
		return false
	}
	// CIDR
	_, _, err := net.ParseCIDR(entry)
	return err == nil
}
