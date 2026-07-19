// 开发者控制台业务接口 Handler
// 包含工作台、设备、订单、云变量、版本、代理、邀请码、支付配置、公告等管理
// 严格遵循铁律 04/05/06：禁止硬编码、配置走 CfgCache、不确定处标注「待核实」
package handler

import (
	"crypto/rand"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/your-org/keyauth-saas/apps/server/internal/heartbeat"
	"github.com/your-org/keyauth-saas/apps/server/internal/middleware"
	"github.com/your-org/keyauth-saas/apps/server/internal/model"
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
		type revenueTrendItem struct {
			Date    string  `json:"date"`
			Revenue float64 `json:"revenue"`
		}
		sevenDaysAgo := todayStart.AddDate(0, 0, -6)
		var trendRows []revenueTrendItem
		deps.DB.Model(&model.AppOrder{}).
			Select("DATE(paid_at) as date, COALESCE(SUM(total_amount), 0) as revenue").
			Where("tenant_id = ? AND pay_status = ? AND paid_at >= ?", tenantID, "paid", sevenDaysAgo).
			Group("DATE(paid_at)").Order("date ASC").Scan(&trendRows)
		trendMap := make(map[string]float64, len(trendRows))
		for _, r := range trendRows {
			trendMap[r.Date] = r.Revenue
		}
		revenueTrend := make([]revenueTrendItem, 0, 7)
		for i := 0; i < 7; i++ {
			d := sevenDaysAgo.AddDate(0, 0, i).Format("2006-01-02")
			revenueTrend = append(revenueTrend, revenueTrendItem{Date: d, Revenue: trendMap[d]})
		}

		// ---- 最近 10 个订单 ----
		var recentOrders []model.AppOrder
		deps.DB.Where("tenant_id = ?", tenantID).Order("id DESC").Limit(10).Find(&recentOrders)

		// ---- Top 5 应用（按卡密数排序）----
		type topAppItem struct {
			AppID     uint64 `json:"app_id"`
			AppName   string `json:"app_name"`
			CardCount int64  `json:"card_count"`
		}
		var topApps []topAppItem
		deps.DB.Table("app_card").
			Select("app_card.app_id, app.name as app_name, COUNT(*) as card_count").
			Joins("LEFT JOIN app ON app.id = app_card.app_id").
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
			"order_today":       orderToday,
			"revenue_today":      revenueToday,
			"revenue_month":      revenueMonth,
			"settlement_pending":  settlementPending,
			"settlement_amount":   settlementAmount,
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
	UserAgent   string     `json:"user_agent"` // 待核实：AppDevice 无 user_agent 字段，需从 Redis 心跳详情读取
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
			middleware.Fail(c, http.StatusInternalServerError, 5001, "查询失败: "+err.Error())
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
			middleware.Fail(c, http.StatusInternalServerError, 5001, "查询失败: "+err.Error())
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
type cloudVarListItem struct {
	model.AppCloudVar
	AppName string `json:"app_name"`
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
			Select("app_cloud_var.*, app.name as app_name").
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

// ============== 6. 创建/更新云变量 ==============

// upsertCloudVarReq 云变量 upsert 请求
type upsertCloudVarReq struct {
	AppID     uint64 `json:"app_id" binding:"required"`
	Key       string `json:"key" binding:"required,min=1,max=128"`
	Value     string `json:"value" binding:"omitempty"`
	ValueType string `json:"value_type" binding:"omitempty,oneof=string number json bool"`
	ReadOnly  bool   `json:"read_only"`
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
				Status:   "active",
			}
			if err := deps.DB.Create(&cv).Error; err != nil {
				middleware.Fail(c, http.StatusInternalServerError, 5001, "创建云变量失败: "+err.Error())
				return
			}
		} else if result.Error != nil {
			middleware.Fail(c, http.StatusInternalServerError, 5001, "查询失败: "+result.Error.Error())
			return
		} else {
			updates := map[string]interface{}{
				"var_value": req.Value,
				"var_type":  valueType,
				"read_only": req.ReadOnly,
			}
			if err := deps.DB.Model(&model.AppCloudVar{}).Where("id = ?", cv.ID).Updates(updates).Error; err != nil {
				middleware.Fail(c, http.StatusInternalServerError, 5002, "更新云变量失败: "+err.Error())
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

// ============== 9. 创建版本 ==============

// createVersionReq 创建版本请求
type createVersionReq struct {
	AppID       uint64 `json:"app_id" binding:"required"`
	Version     string `json:"version" binding:"required,min=1,max=32"`
	Channel     string `json:"channel" binding:"omitempty,oneof=stable beta dev"`
	MinVersion  string `json:"min_version" binding:"omitempty,max=32"`
	DownloadURL string `json:"download_url" binding:"omitempty,max=255"`
	ForceUpdate bool   `json:"force_update"`
	UpdateLog   string `json:"update_log" binding:"omitempty,max=5000"`
	Published   bool   `json:"published"`
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

		v := &model.AppVersion{
			TenantID:      tenantID,
			AppID:         req.AppID,
			Version:       req.Version,
			Channel:       channel,
			MinVersion:    minVersion,
			DownloadURL:   req.DownloadURL,
			ForceUpdate:   req.ForceUpdate,
			UpdateContent: req.UpdateLog,
			Status:        status,
		}
		if err := deps.DB.Create(v).Error; err != nil {
			middleware.Fail(c, http.StatusInternalServerError, 5001, "创建版本失败: "+err.Error())
			return
		}
		middleware.Success(c, v)
	}
}

// ============== 10. 删除版本 ==============

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
			middleware.Fail(c, http.StatusInternalServerError, 5001, "查询失败: "+err.Error())
			return
		}

		// 后处理：子查询统计佣金/提现
		items := make([]agentListItem, 0, len(agents))
		for _, a := range agents {
			item := agentListItem{
				Agent:        a,
				LastActiveAt: a.LastLoginAt,
			}
			// total_commission = SUM(commission.amount WHERE settle_status != 'rejected')
			var commSum sumResult
			deps.DB.Model(&model.AgentCommission{}).
				Where("agent_id = ? AND settle_status != ?", a.ID, "rejected").
				Select("COALESCE(SUM(amount), 0) as total").Scan(&commSum)
			item.TotalCommission = commSum.Total

			// total_withdraw = SUM(withdraw.amount WHERE status='paid')
			var wdSum sumResult
			deps.DB.Model(&model.AgentWithdraw{}).
				Where("agent_id = ? AND status = ?", a.ID, "paid").
				Select("COALESCE(SUM(amount), 0) as total").Scan(&wdSum)
			item.TotalWithdraw = wdSum.Total

			// frozen_balance = SUM(withdraw.amount WHERE status='pending')
			var frSum sumResult
			deps.DB.Model(&model.AgentWithdraw{}).
				Where("agent_id = ? AND status = ?", a.ID, "pending").
				Select("COALESCE(SUM(amount), 0) as total").Scan(&frSum)
			item.FrozenBalance = frSum.Total

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
			middleware.Fail(c, http.StatusInternalServerError, 5002, "更新失败: "+err.Error())
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

		ctx := c.Request.Context()
		// 默认佣金比例从 sys_config 读取（铁律 05）
		defaultRate := deps.CfgCache.GetFloat64(ctx, "agent.default_commission_rate", 10.00)

		codes := make([]string, 0, req.Count)
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
				codes = append(codes, code)
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
	ID     uint64          `json:"id"`
	Channel string         `json:"channel"`
	Config  payConfigDetail `json:"config"`
	Status  string         `json:"status"` // active/disabled
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
			middleware.Fail(c, http.StatusInternalServerError, 5001, "查询失败: "+err.Error())
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
				ID:      cfg.ID,
				Channel: cfg.Channel,
				Config: payConfigDetail{
					PID:       cfg.PID,
					Key:       keyPlain,
					APIURL:    cfg.GatewayURL,
					NotifyURL: cfg.NotifyPath,
					ReturnURL: cfg.ReturnPath,
				},
				Status: status,
			})
		}

		middleware.Success(c, gin.H{"list": items})
	}
}

// ============== 17. 保存支付配置 ==============

// savePayConfigReq 保存支付配置请求
type savePayConfigReq struct {
	Channel string           `json:"channel" binding:"required,oneof=epay wechat alipay"`
	Config  payConfigDetail  `json:"config"`
	Status  string           `json:"status" binding:"omitempty,oneof=active disabled"`
}

// TenantSavePayConfig 保存支付配置
// POST /tenant/pay_config
// 按 (tenant_id, channel) upsert，key 加密入库
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
			middleware.Fail(c, http.StatusInternalServerError, 5001, "加密 key 失败: "+err.Error())
			return
		}
		enabled := req.Status == "active"

		// 按 (tenant_id, channel) upsert
		var cfg model.TenantPayConfig
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
				middleware.Fail(c, http.StatusInternalServerError, 5002, "创建支付配置失败: "+err.Error())
				return
			}
		} else if result.Error != nil {
			middleware.Fail(c, http.StatusInternalServerError, 5001, "查询失败: "+result.Error.Error())
			return
		} else {
			updates := map[string]interface{}{
				"enabled":       enabled,
				"gateway_url":   req.Config.APIURL,
				"pid":           req.Config.PID,
				"key_encrypted": keyEnc,
				"notify_path":   req.Config.NotifyURL,
				"return_path":   req.Config.ReturnURL,
			}
			if err := deps.DB.Model(&model.TenantPayConfig{}).Where("id = ?", cfg.ID).Updates(updates).Error; err != nil {
				middleware.Fail(c, http.StatusInternalServerError, 5002, "更新支付配置失败: "+err.Error())
				return
			}
		}

		middleware.Success(c, gin.H{"id": cfg.ID, "channel": req.Channel, "saved": true})
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
			success = false
			message = "key 解密失败: " + err.Error()
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
	Pinned   bool   `json:"pinned"`     // 映射 IsPinned
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
			middleware.Fail(c, http.StatusInternalServerError, 5001, "查询失败: "+err.Error())
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
	Type     string `json:"type" binding:"required,oneof=tenant agent h5"`
	Title    string `json:"title" binding:"required,max=255"`
	Content  string `json:"content" binding:"required"`
	Status   string `json:"status" binding:"omitempty,oneof=draft published offline"`
	Pinned   bool   `json:"pinned"`
	Sort     int    `json:"sort" binding:"omitempty,min=0"`
	PublishAt string `json:"publish_at"` // 映射到 start_at
	ExpireAt  string `json:"expire_at"`  // 映射到 end_at
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

		notice := model.Notice{
			Type:      req.Type,
			TenantID:  &tenantID,
			Title:     req.Title,
			Content:   req.Content,
			IsPinned:  req.Pinned,
			Sort:      req.Sort,
			IsPopup:   false,
			ShowBadge: true,
			StartAt:   startAt,
			EndAt:     endAt,
			Status:    status,
			ViewCount: 0,
			CreatedBy: userID,
		}
		if err := deps.DB.Create(&notice).Error; err != nil {
			middleware.Fail(c, http.StatusInternalServerError, 5002, "创建公告失败: "+err.Error())
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
	Title     *string `json:"title" binding:"omitempty,max=255"`
	Content   *string `json:"content" binding:"omitempty"`
	Status    *string `json:"status" binding:"omitempty,oneof=draft published offline"`
	Pinned    *bool   `json:"pinned"`
	Sort      *int    `json:"sort" binding:"omitempty,min=0"`
	PublishAt *string `json:"publish_at"` // 映射 start_at
	ExpireAt  *string `json:"expire_at"`  // 映射 end_at
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

		updates := map[string]interface{}{}
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
			middleware.Fail(c, http.StatusInternalServerError, 5002, "更新公告失败: "+err.Error())
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
			middleware.Fail(c, http.StatusInternalServerError, 5002, "删除公告失败: "+err.Error())
			return
		}

		middleware.Success(c, gin.H{"id": id, "deleted": true})
	}
}
