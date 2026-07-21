// v0.4.0 公告弹窗增强 + 数据统计看板
// 严格遵循铁律 04/05/06
package handler

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/your-org/keyauth-saas/apps/server/internal/logger"
	"github.com/your-org/keyauth-saas/apps/server/internal/middleware"
	"github.com/your-org/keyauth-saas/apps/server/internal/model"
)

// ============== 配置键常量（铁律 04：禁止硬编码） ==============

const (
	CfgKeyNoticePopupEnabled       = "notice.popup.enabled"
	CfgKeyNoticePopupMaxUnread     = "notice.popup.max_unread"
	CfgKeyNoticePopupDismissTTLHrs = "notice.popup.dismiss_ttl_hours"
	CfgKeyNoticeRichtextEnabled    = "notice.richtext.enabled"
	CfgKeyNoticeRichtextMaxLength  = "notice.richtext.max_length"

	CfgKeyStatsVerifyTrendDefaultDays   = "stats.verify_trend.default_days"
	CfgKeyStatsVerifyTrendMaxDays       = "stats.verify_trend.max_days"
	CfgKeyStatsAgentRankingDefaultLimit = "stats.agent_ranking.default_limit"
	CfgKeyStatsAgentRankingMaxLimit     = "stats.agent_ranking.max_limit"
)

// 默认值常量（铁律 04：仅作为 sys_config 兜底，不在业务逻辑中硬编码）
const (
	defaultNoticePopupMaxUnread     = 5
	defaultNoticePopupDismissTTLHrs = 24
	defaultNoticeRichtextMaxLength  = 10000

	defaultStatsVerifyTrendDays   = 30
	maxStatsVerifyTrendDays       = 90
	defaultStatsAgentRankingLimit = 10
	maxStatsAgentRankingLimit     = 100
)

// ============== 1. 公告弹窗：admin / tenant / agent 三端 popup 列表 ==============

// popupNoticeItem 弹窗公告返回项
type popupNoticeItem struct {
	ID            uint64     `json:"id"`
	Type          string     `json:"type"`
	Title         string     `json:"title"`
	Content       string     `json:"content"`
	ContentFormat string     `json:"content_format"`
	IsPinned      bool       `json:"is_pinned"`
	ShowBadge     bool       `json:"show_badge"`
	PublishAt     time.Time  `json:"publish_at"`
	ExpireAt      *time.Time `json:"expire_at,omitempty"`
}

// popupListResponse 弹窗列表响应
type popupListResponse struct {
	Enabled         bool              `json:"enabled"`
	DismissTTLHours int               `json:"dismiss_ttl_hours"`
	List            []popupNoticeItem `json:"list"`
	Total           int64             `json:"total"`
}

// queryPopupNotices 通用查询：返回当前用户未读的 is_popup=true 已发布公告
// userType: admin / tenant / agent
// userID: 当前用户 ID
// tenantID: 当前租户 ID（admin=0；agent 用于查 tenant 向公告；tenant 用于过滤自己的公告）
func queryPopupNotices(deps *Deps, userType string, userID, tenantID uint64) ([]popupNoticeItem, error) {
	ctx := context.Background()
	maxUnread := defaultNoticePopupMaxUnread
	if deps.CfgCache != nil {
		maxUnread = deps.CfgCache.GetInt(ctx, CfgKeyNoticePopupMaxUnread, defaultNoticePopupMaxUnread)
	}
	if maxUnread < 1 {
		maxUnread = defaultNoticePopupMaxUnread
	}

	now := time.Now()
	q := deps.DB.Model(&model.Notice{}).
		Where("status = ?", "published").
		Where("is_popup = ?", 1).
		Where("start_at <= ?", now).
		Where("end_at IS NULL OR end_at > ?", now).
		Where("id NOT IN (SELECT notice_id FROM notice_read WHERE user_type = ? AND user_id = ?)",
			userType, userID)

	// 按用户类型限定可见范围
	switch userType {
	case "admin":
		// admin 仅看平台公告
		q = q.Where("type = ?", "platform")
	case "tenant":
		// tenant 看平台公告 + 自己的开发者公告
		q = q.Where("(type = ? AND tenant_id IS NULL) OR (type = ? AND tenant_id = ?)",
			"platform", "tenant", tenantID)
	case "agent":
		// agent 看平台公告 + 当前租户的开发者公告 + 当前租户的代理通知
		q = q.Where("(type = ? AND tenant_id IS NULL) OR (type = ? AND tenant_id = ?) OR (type = ? AND tenant_id = ?)",
			"platform", "tenant", tenantID, "agent_notify", tenantID)
	}

	var notices []model.Notice
	if err := q.Order("is_pinned DESC, sort DESC, start_at DESC").
		Limit(maxUnread).Find(&notices).Error; err != nil {
		return nil, err
	}

	list := make([]popupNoticeItem, 0, len(notices))
	for _, n := range notices {
		list = append(list, popupNoticeItem{
			ID:            n.ID,
			Type:          n.Type,
			Title:         n.Title,
			Content:       n.Content,
			ContentFormat: n.ContentFormat,
			IsPinned:      n.IsPinned,
			ShowBadge:     n.ShowBadge,
			PublishAt:     n.StartAt,
			ExpireAt:      n.EndAt,
		})
	}
	return list, nil
}

// AdminPopupNotices admin 端弹窗公告列表
// GET /admin/notices/popup
func AdminPopupNotices(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		// 总开关
		enabled := true
		if deps.CfgCache != nil {
			enabled = deps.CfgCache.GetBool(ctx, CfgKeyNoticePopupEnabled, true)
		}
		dismissTTL := defaultNoticePopupDismissTTLHrs
		if deps.CfgCache != nil {
			dismissTTL = deps.CfgCache.GetInt(ctx, CfgKeyNoticePopupDismissTTLHrs, defaultNoticePopupDismissTTLHrs)
		}

		if !enabled {
			middleware.Success(c, popupListResponse{
				Enabled:         false,
				DismissTTLHours: dismissTTL,
				List:            []popupNoticeItem{},
				Total:           0,
			})
			return
		}

		userID := getUserID(c)
		list, err := queryPopupNotices(deps, "admin", userID, 0)
		if err != nil {
			logger.Error("notice_stats: query popup notices failed", "err", err, "role", "admin", "user_id", userID)
			middleware.Fail(c, http.StatusInternalServerError, 5001, "查询弹窗公告失败")
			return
		}

		middleware.Success(c, popupListResponse{
			Enabled:         true,
			DismissTTLHours: dismissTTL,
			List:            list,
			Total:           int64(len(list)),
		})
	}
}

// TenantPopupNotices tenant 端弹窗公告列表
// GET /tenant/notices/popup
func TenantPopupNotices(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		enabled := true
		if deps.CfgCache != nil {
			enabled = deps.CfgCache.GetBool(ctx, CfgKeyNoticePopupEnabled, true)
		}
		dismissTTL := defaultNoticePopupDismissTTLHrs
		if deps.CfgCache != nil {
			dismissTTL = deps.CfgCache.GetInt(ctx, CfgKeyNoticePopupDismissTTLHrs, defaultNoticePopupDismissTTLHrs)
		}

		if !enabled {
			middleware.Success(c, popupListResponse{
				Enabled:         false,
				DismissTTLHours: dismissTTL,
				List:            []popupNoticeItem{},
				Total:           0,
			})
			return
		}

		tenantID := getTenantID(c)
		if tenantID == 0 {
			middleware.Fail(c, http.StatusForbidden, 1003, "无法识别租户身份")
			return
		}
		userID := getUserID(c)
		list, err := queryPopupNotices(deps, "tenant", userID, tenantID)
		if err != nil {
			middleware.Fail(c, http.StatusInternalServerError, 5001, "查询弹窗公告失败: "+err.Error())
			return
		}

		middleware.Success(c, popupListResponse{
			Enabled:         true,
			DismissTTLHours: dismissTTL,
			List:            list,
			Total:           int64(len(list)),
		})
	}
}

// AgentPopupNotices agent 端弹窗公告列表
// GET /agent/notices/popup
func AgentPopupNotices(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		enabled := true
		if deps.CfgCache != nil {
			enabled = deps.CfgCache.GetBool(ctx, CfgKeyNoticePopupEnabled, true)
		}
		dismissTTL := defaultNoticePopupDismissTTLHrs
		if deps.CfgCache != nil {
			dismissTTL = deps.CfgCache.GetInt(ctx, CfgKeyNoticePopupDismissTTLHrs, defaultNoticePopupDismissTTLHrs)
		}

		if !enabled {
			middleware.Success(c, popupListResponse{
				Enabled:         false,
				DismissTTLHours: dismissTTL,
				List:            []popupNoticeItem{},
				Total:           0,
			})
			return
		}

		tenantID := getTenantID(c)
		if tenantID == 0 {
			middleware.Fail(c, http.StatusForbidden, 1003, "无法识别租户身份")
			return
		}
		agentID := getUserID(c)
		if agentID == 0 {
			middleware.Fail(c, http.StatusForbidden, 1003, "无法识别代理身份")
			return
		}
		list, err := queryPopupNotices(deps, "agent", agentID, tenantID)
		if err != nil {
			middleware.Fail(c, http.StatusInternalServerError, 5001, "查询弹窗公告失败: "+err.Error())
			return
		}

		middleware.Success(c, popupListResponse{
			Enabled:         true,
			DismissTTLHours: dismissTTL,
			List:            list,
			Total:           int64(len(list)),
		})
	}
}

// MarkNoticeReadByPopup 通用：标记公告已读（POST /:role/notices/:id/read）
// 三端共用：admin/tenant/agent 都通过此 handler 标记已读
// userType 由调用方根据 role 决定
func MarkNoticeReadByPopup(deps *Deps, userType string) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := getUserID(c)
		if userID == 0 {
			middleware.Fail(c, http.StatusForbidden, 1003, "无法识别用户身份")
			return
		}
		noticeID, err := strconv.ParseUint(c.Param("id"), 10, 64)
		if err != nil || noticeID == 0 {
			middleware.Fail(c, http.StatusBadRequest, 1001, "公告 ID 格式错误")
			return
		}

		// 校验公告存在
		var notice model.Notice
		if err := deps.DB.First(&notice, noticeID).Error; err != nil {
			middleware.Fail(c, http.StatusNotFound, 1008, "公告不存在")
			return
		}

		// FirstOrCreate 幂等
		read := &model.NoticeRead{
			NoticeID: noticeID,
			UserType: userType,
			UserID:   userID,
		}
		if err := deps.DB.Where("notice_id = ? AND user_type = ? AND user_id = ?",
			noticeID, userType, userID).
			FirstOrCreate(read).Error; err != nil {
			middleware.Fail(c, http.StatusInternalServerError, 5002, "标记已读失败")
			return
		}

		middleware.Success(c, gin.H{
			"notice_id": noticeID,
			"read":      true,
			"read_at":   read.ReadAt,
		})
	}
}

// ============== 2. 数据统计：验证趋势图 ==============

// verifyTrendDay 验证趋势单日数据
type verifyTrendDay struct {
	Date           string `json:"date"`
	Total          int64  `json:"total"`
	Success        int64  `json:"success"`
	Fail           int64  `json:"fail"`
	Banned         int64  `json:"banned"`
	Expired        int64  `json:"expired"`
	DeviceMismatch int64  `json:"device_mismatch"`
	RateLimited    int64  `json:"rate_limited"`
}

// verifyTrendResponse 验证趋势响应
type verifyTrendResponse struct {
	Days            int              `json:"days"`
	Total           int64            `json:"total"`
	Trend           []verifyTrendDay `json:"trend"`
	ActionBreakdown map[string]int64 `json:"action_breakdown"`
}

// AdminVerifyTrend admin 端验证趋势图（全平台）
// GET /admin/stats/verify_trend?days=30
func AdminVerifyTrend(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		days := parseDaysParam(c, deps)
		trend, total, actionBreakdown, err := queryVerifyTrend(deps, 0, 0, days)
		if err != nil {
			middleware.Fail(c, http.StatusInternalServerError, 5001, "查询验证趋势失败: "+err.Error())
			return
		}
		middleware.Success(c, verifyTrendResponse{
			Days:            days,
			Total:           total,
			Trend:           trend,
			ActionBreakdown: actionBreakdown,
		})
	}
}

// TenantVerifyTrend tenant 端验证趋势图（仅当前租户）
// GET /tenant/stats/verify_trend?days=30
func TenantVerifyTrend(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID := getTenantID(c)
		if tenantID == 0 {
			middleware.Fail(c, http.StatusForbidden, 1003, "无法识别租户身份")
			return
		}
		days := parseDaysParam(c, deps)
		trend, total, actionBreakdown, err := queryVerifyTrend(deps, tenantID, 0, days)
		if err != nil {
			middleware.Fail(c, http.StatusInternalServerError, 5001, "查询验证趋势失败: "+err.Error())
			return
		}
		middleware.Success(c, verifyTrendResponse{
			Days:            days,
			Total:           total,
			Trend:           trend,
			ActionBreakdown: actionBreakdown,
		})
	}
}

// parseDaysParam 解析 days 参数（受 sys_config 上下限约束）
func parseDaysParam(c *gin.Context, deps *Deps) int {
	defaultDays := defaultStatsVerifyTrendDays
	maxDays := maxStatsVerifyTrendDays
	if deps.CfgCache != nil {
		defaultDays = deps.CfgCache.GetInt(c.Request.Context(), CfgKeyStatsVerifyTrendDefaultDays, defaultStatsVerifyTrendDays)
		maxDays = deps.CfgCache.GetInt(c.Request.Context(), CfgKeyStatsVerifyTrendMaxDays, maxStatsVerifyTrendDays)
	}
	if maxDays < 1 {
		maxDays = maxStatsVerifyTrendDays
	}
	if defaultDays < 1 || defaultDays > maxDays {
		defaultDays = maxStatsVerifyTrendDays
	}

	days, _ := strconv.Atoi(c.DefaultQuery("days", ""))
	if days < 1 {
		days = defaultDays
	}
	if days > maxDays {
		days = maxDays
	}
	return days
}

// queryVerifyTrend 查询验证趋势
// tenantID=0 表示全平台，>0 表示限定租户
// appID=0 表示不限应用，>0 表示限定应用
func queryVerifyTrend(deps *Deps, tenantID, appID uint64, days int) ([]verifyTrendDay, int64, map[string]int64, error) {
	now := time.Now()
	startOfToday := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local)
	startDate := startOfToday.AddDate(0, 0, -(days - 1))

	// 按日聚合
	q := deps.DB.Model(&model.LogVerify{}).
		Where("created_at >= ?", startDate)
	if tenantID > 0 {
		q = q.Where("tenant_id = ?", tenantID)
	}
	if appID > 0 {
		q = q.Where("app_id = ?", appID)
	}

	type dayRow struct {
		Date   string
		Result string
		Count  int64
	}
	var rows []dayRow
	if err := q.Select("DATE(created_at) AS date, result, COUNT(*) AS count").
		Group("DATE(created_at), result").
		Order("date ASC").
		Scan(&rows).Error; err != nil {
		return nil, 0, nil, err
	}

	// 按 result 维度填充每日数据
	dayMap := make(map[string]*verifyTrendDay, days)
	trend := make([]verifyTrendDay, 0, days)
	for i := 0; i < days; i++ {
		date := startDate.AddDate(0, 0, i).Format("2006-01-02")
		d := verifyTrendDay{Date: date}
		dayMap[date] = &d
		trend = append(trend, d)
	}
	for _, row := range rows {
		d, ok := dayMap[row.Date]
		if !ok {
			continue
		}
		d.Total += row.Count
		switch row.Result {
		case "success":
			d.Success += row.Count
		case "fail":
			d.Fail += row.Count
		case "banned":
			d.Banned += row.Count
		case "expired":
			d.Expired += row.Count
		case "device_mismatch":
			d.DeviceMismatch += row.Count
		case "rate_limited":
			d.RateLimited += row.Count
		}
	}
	// 把修改后的数据回写到 trend slice
	for i := range trend {
		if d, ok := dayMap[trend[i].Date]; ok {
			trend[i] = *d
		}
	}

	// 总计 + action 维度聚合
	var total int64
	q2 := deps.DB.Model(&model.LogVerify{}).Where("created_at >= ?", startDate)
	if tenantID > 0 {
		q2 = q2.Where("tenant_id = ?", tenantID)
	}
	if appID > 0 {
		q2 = q2.Where("app_id = ?", appID)
	}
	q2.Count(&total)

	type actionRow struct {
		Action string
		Count  int64
	}
	var actions []actionRow
	q2.Select("action, COUNT(*) AS count").
		Group("action").
		Scan(&actions)
	actionBreakdown := make(map[string]int64, len(actions))
	for _, a := range actions {
		actionBreakdown[a.Action] = a.Count
	}

	return trend, total, actionBreakdown, nil
}

// ============== 3. 数据统计：代理业绩排行 ==============

// agentRankingItem 代理排行单项
type agentRankingItem struct {
	AgentID     uint64  `json:"agent_id"`
	Username    string  `json:"username"`
	RealName    string  `json:"real_name"`
	TenantID    uint64  `json:"tenant_id"`
	TenantName  string  `json:"tenant_name"`
	OrderCount  int64   `json:"order_count"`
	TotalAmount float64 `json:"total_amount"`
	Commission  float64 `json:"commission"`
	NetAmount   float64 `json:"net_amount"`
	Rank        int     `json:"rank"`
}

// agentRankingResponse 排行响应
type agentRankingResponse struct {
	StartAt string             `json:"start_at"`
	EndAt   string             `json:"end_at"`
	SortBy  string             `json:"sort_by"`
	Limit   int                `json:"limit"`
	Total   int64              `json:"total"`
	List    []agentRankingItem `json:"list"`
}

// AdminAgentRanking admin 端代理业绩排行（全平台）
// GET /admin/stats/agent_ranking?start=&end=&limit=10&sort_by=total_amount
func AdminAgentRanking(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID := uint64(0)
		list, total, sortBy, limit, startAt, endAt, err := queryAgentRanking(c, deps, tenantID)
		if err != nil {
			logger.Error("notice_stats: query agent ranking failed", "err", err, "tenant_id", tenantID)
			middleware.Fail(c, http.StatusInternalServerError, 5001, "查询代理排行失败")
			return
		}
		middleware.Success(c, agentRankingResponse{
			StartAt: startAt.Format("2006-01-02 15:04:05"),
			EndAt:   endAt.Format("2006-01-02 15:04:05"),
			SortBy:  sortBy,
			Limit:   limit,
			Total:   total,
			List:    list,
		})
	}
}

// TenantAgentRanking tenant 端代理业绩排行（仅当前租户）
// GET /tenant/stats/agent_ranking?start=&end=&limit=10&sort_by=total_amount
func TenantAgentRanking(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID := getTenantID(c)
		if tenantID == 0 {
			middleware.Fail(c, http.StatusForbidden, 1003, "无法识别租户身份")
			return
		}
		list, total, sortBy, limit, startAt, endAt, err := queryAgentRanking(c, deps, tenantID)
		if err != nil {
			logger.Error("notice_stats: query agent ranking failed", "err", err, "tenant_id", tenantID)
			middleware.Fail(c, http.StatusInternalServerError, 5001, "查询代理排行失败")
			return
		}
		middleware.Success(c, agentRankingResponse{
			StartAt: startAt.Format("2006-01-02 15:04:05"),
			EndAt:   endAt.Format("2006-01-02 15:04:05"),
			SortBy:  sortBy,
			Limit:   limit,
			Total:   total,
			List:    list,
		})
	}
}

// queryAgentRanking 查询代理业绩排行
// tenantID=0 表示全平台，>0 表示限定租户
// sort_by: total_amount（默认） / commission / net_amount / order_count
func queryAgentRanking(c *gin.Context, deps *Deps, tenantID uint64) ([]agentRankingItem, int64, string, int, time.Time, time.Time, error) {
	ctx := c.Request.Context()

	// 默认 limit
	defaultLimit := defaultStatsAgentRankingLimit
	maxLimit := maxStatsAgentRankingLimit
	if deps.CfgCache != nil {
		defaultLimit = deps.CfgCache.GetInt(ctx, CfgKeyStatsAgentRankingDefaultLimit, defaultStatsAgentRankingLimit)
		maxLimit = deps.CfgCache.GetInt(ctx, CfgKeyStatsAgentRankingMaxLimit, maxStatsAgentRankingLimit)
	}
	if defaultLimit < 1 {
		defaultLimit = defaultStatsAgentRankingLimit
	}
	if maxLimit < 1 {
		maxLimit = maxStatsAgentRankingLimit
	}

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", ""))
	if limit < 1 {
		limit = defaultLimit
	}
	if limit > maxLimit {
		limit = maxLimit
	}

	// 排序字段
	sortBy := c.DefaultQuery("sort_by", "total_amount")
	orderExpr := "total_amount DESC"
	switch sortBy {
	case "total_amount":
		orderExpr = "total_amount DESC"
	case "commission":
		orderExpr = "commission DESC"
	case "net_amount":
		orderExpr = "net_amount DESC"
	case "order_count":
		orderExpr = "order_count DESC"
	default:
		sortBy = "total_amount"
		orderExpr = "total_amount DESC"
	}

	// 时间范围（默认近 30 天）
	now := time.Now()
	endAt := now
	startAt := now.AddDate(0, 0, -29)
	startAt = time.Date(startAt.Year(), startAt.Month(), startAt.Day(), 0, 0, 0, 0, time.Local)
	if start := c.Query("start"); start != "" {
		if t, err := time.ParseInLocation("2006-01-02 15:04:05", start, time.Local); err == nil {
			startAt = t
		} else if t, err := time.ParseInLocation("2006-01-02", start, time.Local); err == nil {
			startAt = t
		}
	}
	if end := c.Query("end"); end != "" {
		if t, err := time.ParseInLocation("2006-01-02 15:04:05", end, time.Local); err == nil {
			endAt = t
		} else if t, err := time.ParseInLocation("2006-01-02", end, time.Local); err == nil {
			endAt = t.Add(24*time.Hour - time.Second)
		}
	}

	// 联表查询 agent + sys_tenant + 订单聚合
	q := deps.DB.Table("agent AS a").
		Select("a.id AS agent_id, a.username, a.real_name, a.tenant_id, "+
			"t.username AS tenant_name, "+
			"COUNT(o.id) AS order_count, "+
			"COALESCE(SUM(o.total_amount), 0) AS total_amount, "+
			"COALESCE(SUM(o.commission_amount), 0) AS commission, "+
			"COALESCE(SUM(o.total_amount - o.commission_amount), 0) AS net_amount").
		Joins("LEFT JOIN sys_tenant AS t ON t.id = a.tenant_id").
		Joins("LEFT JOIN app_order AS o ON o.agent_id = a.id AND o.pay_status = ? AND o.paid_at >= ? AND o.paid_at <= ?",
			"paid", startAt, endAt).
		Where("a.status = ?", "active")

	if tenantID > 0 {
		q = q.Where("a.tenant_id = ?", tenantID)
	}

	// 先统计有订单的代理数（仅作 total 字段，前端参考用）
	var total int64
	countQ := deps.DB.Table("agent AS a").Where("a.status = ?", "active")
	if tenantID > 0 {
		countQ = countQ.Where("a.tenant_id = ?", tenantID)
	}
	countQ.Count(&total)

	type rankRow struct {
		AgentID     uint64
		Username    string
		RealName    string
		TenantID    uint64
		TenantName  string
		OrderCount  int64
		TotalAmount float64
		Commission  float64
		NetAmount   float64
	}
	var rows []rankRow
	if err := q.Group("a.id, a.username, a.real_name, a.tenant_id, t.username").
		Order(orderExpr).
		Limit(limit).
		Scan(&rows).Error; err != nil {
		return nil, 0, "", 0, startAt, endAt, err
	}

	list := make([]agentRankingItem, 0, len(rows))
	for i, r := range rows {
		list = append(list, agentRankingItem{
			AgentID:     r.AgentID,
			Username:    r.Username,
			RealName:    r.RealName,
			TenantID:    r.TenantID,
			TenantName:  r.TenantName,
			OrderCount:  r.OrderCount,
			TotalAmount: r.TotalAmount,
			Commission:  r.Commission,
			NetAmount:   r.NetAmount,
			Rank:        i + 1,
		})
	}
	return list, total, sortBy, limit, startAt, endAt, nil
}
