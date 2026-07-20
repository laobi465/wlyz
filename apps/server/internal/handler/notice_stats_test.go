// Package handler 公告弹窗 + 数据统计单元测试
// v0.4.0 第十六项迁移：覆盖 notice popup / verify trend / agent ranking 三大模块
// 严格遵循铁律 06：所有断言基于已知固定输入，无随机/不确定性
package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
)

// ============== 测试基础设施 ==============

// setupNoticeStatsDB 启动 SQLite 内存库 + AutoMigrate 公告/统计相关表
func setupNoticeStatsDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:notice_stats_test?mode=memory&cache=shared"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&model.Notice{},
		&model.NoticeRead{},
		&model.NoticeTarget{},
		&model.SysConfig{},
		&model.LogVerify{},
		&model.Agent{},
		&model.SysTenant{},
		&model.AppOrder{},
	))
	// 清空表（cache=shared 模式下可能残留旧数据）
	db.Exec("DELETE FROM notice")
	db.Exec("DELETE FROM notice_read")
	db.Exec("DELETE FROM notice_target")
	db.Exec("DELETE FROM sys_config")
	db.Exec("DELETE FROM log_verify")
	db.Exec("DELETE FROM agent")
	db.Exec("DELETE FROM sys_tenant")
	db.Exec("DELETE FROM app_order")
	return db
}

// setupNoticeStatsCfgCache 构造真实 ConfigCache + 注入 sys_config 默认值
func setupNoticeStatsCfgCache(t *testing.T, db *gorm.DB, overrides map[string]string) *config.ConfigCache {
	t.Helper()
	mr, err := miniredis.Run()
	require.NoError(t, err)
	t.Cleanup(mr.Close)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	defaults := map[string]string{
		CfgKeyNoticePopupEnabled:           "1",
		CfgKeyNoticePopupMaxUnread:         "5",
		CfgKeyNoticePopupDismissTTLHrs:     "24",
		CfgKeyNoticeRichtextEnabled:        "1",
		CfgKeyNoticeRichtextMaxLength:      "10000",
		CfgKeyStatsVerifyTrendDefaultDays:  "30",
		CfgKeyStatsVerifyTrendMaxDays:      "90",
		CfgKeyStatsAgentRankingDefaultLimit: "10",
		CfgKeyStatsAgentRankingMaxLimit:     "100",
	}
	if overrides == nil {
		overrides = map[string]string{}
	}
	for k, v := range defaults {
		if _, ok := overrides[k]; !ok {
			overrides[k] = v
		}
	}
	for k, v := range overrides {
		require.NoError(t, db.Create(&model.SysConfig{
			ConfigKey:   k,
			ConfigValue: v,
			ConfigType:  "string",
			ConfigGroup: "test",
		}).Error)
	}
	cfgCache := config.NewConfigCache(db, rdb)
	require.NoError(t, cfgCache.Preload(context.Background()))
	return cfgCache
}

// setupNoticeStatsDeps 构造测试 Deps
func setupNoticeStatsDeps(t *testing.T, cfgOverrides map[string]string) *Deps {
	t.Helper()
	db := setupNoticeStatsDB(t)
	cfgCache := setupNoticeStatsCfgCache(t, db, cfgOverrides)
	return &Deps{
		DB:       db,
		CfgCache: cfgCache,
	}
}

// ============== 辅助函数 ==============

func seedNotice(t *testing.T, db *gorm.DB, n *model.Notice) {
	t.Helper()
	require.NoError(t, db.Create(n).Error)
}

func seedNoticeRead(t *testing.T, db *gorm.DB, noticeID uint64, userType string, userID uint64) {
	t.Helper()
	require.NoError(t, db.Create(&model.NoticeRead{
		NoticeID: noticeID,
		UserType: userType,
		UserID:   userID,
		ReadAt:   time.Now(),
	}).Error)
}

func seedLogVerify(t *testing.T, db *gorm.DB, tenantID, appID uint64, action, result string, createdAt time.Time) {
	t.Helper()
	require.NoError(t, db.Create(&model.LogVerify{
		TenantID:  tenantID,
		AppID:     appID,
		Action:    action,
		Result:    result,
		ClientIP:  "127.0.0.1",
		CreatedAt: createdAt,
	}).Error)
}

func seedAgent(t *testing.T, db *gorm.DB, id, tenantID uint64, username, realName string) {
	t.Helper()
	require.NoError(t, db.Create(&model.Agent{
		BaseModel:      model.BaseModel{ID: id},
		TenantID:       tenantID,
		Username:       username,
		RealName:       realName,
		Status:         "active",
		CommissionRate: 10.0,
	}).Error)
}

func seedTenant(t *testing.T, db *gorm.DB, id uint64, username string) {
	t.Helper()
	require.NoError(t, db.Create(&model.SysTenant{
		BaseModel:  model.BaseModel{ID: id},
		TenantCode: "tc-" + uintToStr(id),
		Username:   username,
		Status:     "active",
	}).Error)
}

func seedOrder(t *testing.T, db *gorm.DB, id, tenantID uint64, agentID uint64, total, commission float64, paidAt time.Time) {
	t.Helper()
	require.NoError(t, db.Create(&model.AppOrder{
		BaseModel:        model.BaseModel{ID: id},
		TenantID:         tenantID,
		AgentID:          &agentID,
		OrderNo:          "test-order-" + time.Now().Format("150405.000000") + "-" + string(rune(id)),
		Quantity:         1,
		UnitPrice:        total,
		TotalAmount:      total,
		CommissionAmount: commission,
		PayChannel:       "balance",
		PayStatus:        "paid",
		PaidAt:           &paidAt,
	}).Error)
}

// callEndpoint 通用 GET 调用
func callEndpoint(t *testing.T, method, path string, handler gin.HandlerFunc, ctxSetup func(c *gin.Context)) map[string]interface{} {
	t.Helper()
	g := setupGin()
	g.Handle(method, path, handler)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(method, path, nil)
	g.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code, "接口应返回 200")

	var resp struct {
		Code int                    `json:"code"`
		Data map[string]interface{} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Equal(t, 0, resp.Code, "code 应为 0")
	return resp.Data
}

// ============== 1. 公告弹窗测试 ==============

// TestAdminPopupNotices_DisabledByConfig notice.popup.enabled=0 时返回 enabled=false
func TestAdminPopupNotices_DisabledByConfig(t *testing.T) {
	deps := setupNoticeStatsDeps(t, map[string]string{
		CfgKeyNoticePopupEnabled: "0",
	})

	data := callEndpoint(t, "GET", "/admin/notices/popup", AdminPopupNotices(deps), nil)
	assert.Equal(t, false, data["enabled"])
	assert.Equal(t, float64(24), data["dismiss_ttl_hours"])
	list, ok := data["list"].([]interface{})
	require.True(t, ok)
	assert.Empty(t, list)
}

// TestAdminPopupNotices_NoUnread 无未读弹窗公告时返回空 list
func TestAdminPopupNotices_NoUnread(t *testing.T) {
	deps := setupNoticeStatsDeps(t, nil)

	data := callEndpoint(t, "GET", "/admin/notices/popup", AdminPopupNotices(deps), nil)
	assert.Equal(t, true, data["enabled"])
	list, ok := data["list"].([]interface{})
	require.True(t, ok)
	assert.Empty(t, list)
	assert.Equal(t, float64(0), data["total"])
}

// TestAdminPopupNotices_WithUnread 有未读弹窗公告时返回列表
func TestAdminPopupNotices_WithUnread(t *testing.T) {
	deps := setupNoticeStatsDeps(t, nil)

	now := time.Now()
	seedNotice(t, deps.DB, &model.Notice{
		Type:          "platform",
		Title:         "公告 1",
		Content:       "内容 1",
		ContentFormat: "text",
		IsPinned:      true,
		IsPopup:       true,
		ShowBadge:     true,
		StartAt:       now.Add(-1 * time.Hour),
		Status:        "published",
	})
	seedNotice(t, deps.DB, &model.Notice{
		Type:          "platform",
		Title:         "公告 2",
		Content:       "内容 2",
		ContentFormat: "html",
		IsPinned:      false,
		IsPopup:       true,
		ShowBadge:     true,
		StartAt:       now.Add(-30 * time.Minute),
		Status:        "published",
	})
	// 非 popup 公告不应出现
	seedNotice(t, deps.DB, &model.Notice{
		Type:     "platform",
		Title:    "非弹窗公告",
		Content:  "不应返回",
		IsPopup:  false,
		StartAt:  now.Add(-15 * time.Minute),
		Status:   "published",
	})
	// 未发布的不应出现
	seedNotice(t, deps.DB, &model.Notice{
		Type:     "platform",
		Title:    "草稿",
		Content:  "不应返回",
		IsPopup:  true,
		StartAt:  now.Add(-10 * time.Minute),
		Status:   "draft",
	})
	// 已过期的不应出现
	endAt := now.Add(-5 * time.Minute)
	seedNotice(t, deps.DB, &model.Notice{
		Type:     "platform",
		Title:    "已过期",
		Content:  "不应返回",
		IsPopup:  true,
		StartAt:  now.Add(-2 * time.Hour),
		EndAt:    &endAt,
		Status:   "published",
	})

	data := callEndpoint(t, "GET", "/admin/notices/popup", AdminPopupNotices(deps), nil)
	list, ok := data["list"].([]interface{})
	require.True(t, ok)
	assert.Len(t, list, 2, "应返回 2 条未读弹窗公告")
	// 第一条应为 is_pinned=true 的
	first := list[0].(map[string]interface{})
	assert.Equal(t, "公告 1", first["title"])
	assert.Equal(t, true, first["is_pinned"])
	assert.Equal(t, float64(2), data["total"])
}

// TestAdminPopupNotices_ExcludesRead 已读公告不应再次返回
func TestAdminPopupNotices_ExcludesRead(t *testing.T) {
	deps := setupNoticeStatsDeps(t, nil)

	now := time.Now()
	n1 := &model.Notice{
		Type:     "platform",
		Title:    "已读公告",
		Content:  "内容",
		IsPopup:  true,
		StartAt:  now.Add(-1 * time.Hour),
		Status:   "published",
	}
	seedNotice(t, deps.DB, n1)
	seedNoticeRead(t, deps.DB, n1.ID, "admin", 888) // admin user_id=888

	// 模拟 admin user_id=888 调用
	g := setupGin()
	g.GET("/admin/notices/popup", func(c *gin.Context) {
		c.Set("user_id", uint64(888))
		c.Set("role", "admin")
		c.Next()
	}, AdminPopupNotices(deps))

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/admin/notices/popup", nil)
	g.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	var resp struct {
		Code int                    `json:"code"`
		Data map[string]interface{} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Equal(t, 0, resp.Code)

	list, ok := resp.Data["list"].([]interface{})
	require.True(t, ok)
	assert.Empty(t, list, "已读弹窗公告不应再次返回")
}

// TestAdminPopupNotices_MaxUnreadLimit 受 max_unread 配置限制
func TestAdminPopupNotices_MaxUnreadLimit(t *testing.T) {
	deps := setupNoticeStatsDeps(t, map[string]string{
		CfgKeyNoticePopupMaxUnread: "2",
	})

	now := time.Now()
	for i := 0; i < 5; i++ {
		seedNotice(t, deps.DB, &model.Notice{
			Type:     "platform",
			Title:    "公告 " + string(rune('A'+i)),
			Content:  "内容",
			IsPopup:  true,
			StartAt:  now.Add(-time.Duration(5-i) * time.Hour),
			Status:   "published",
		})
	}

	data := callEndpoint(t, "GET", "/admin/notices/popup", AdminPopupNotices(deps), nil)
	list, ok := data["list"].([]interface{})
	require.True(t, ok)
	assert.Len(t, list, 2, "受 max_unread=2 限制应返回 2 条")
}

// TestTenantPopupNotices_Scope tenant 端仅看平台 + 自己的开发者公告
func TestTenantPopupNotices_Scope(t *testing.T) {
	deps := setupNoticeStatsDeps(t, nil)

	now := time.Now()
	tenant1001 := uint64(1001)
	tenant1002 := uint64(1002)

	// 平台公告（应可见）
	seedNotice(t, deps.DB, &model.Notice{
		Type:     "platform",
		Title:    "平台公告",
		Content:  "p",
		IsPopup:  true,
		StartAt:  now.Add(-1 * time.Hour),
		Status:   "published",
	})
	// 租户 1001 的开发者公告（应可见）
	seedNotice(t, deps.DB, &model.Notice{
		Type:     "tenant",
		TenantID: &tenant1001,
		Title:    "租户 1001 公告",
		Content:  "t1",
		IsPopup:  true,
		StartAt:  now.Add(-1 * time.Hour),
		Status:   "published",
	})
	// 租户 1002 的开发者公告（不应可见）
	seedNotice(t, deps.DB, &model.Notice{
		Type:     "tenant",
		TenantID: &tenant1002,
		Title:    "租户 1002 公告",
		Content:  "t2",
		IsPopup:  true,
		StartAt:  now.Add(-1 * time.Hour),
		Status:   "published",
	})

	// 构造请求模拟 tenant 上下文
	g := setupGin()
	g.GET("/tenant/notices/popup", func(c *gin.Context) {
		c.Set("tenant_id", uint64(1001))
		c.Set("user_id", uint64(1001))
		c.Next()
	}, TenantPopupNotices(deps))

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/tenant/notices/popup", nil)
	g.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	var resp struct {
		Code int                    `json:"code"`
		Data map[string]interface{} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Equal(t, 0, resp.Code)

	list, ok := resp.Data["list"].([]interface{})
	require.True(t, ok)
	assert.Len(t, list, 2, "租户 1001 应只看到平台公告 + 自己的开发者公告")
}

// TestMarkNoticeReadByPopup_Idempotent 标记已读幂等
func TestMarkNoticeReadByPopup_Idempotent(t *testing.T) {
	deps := setupNoticeStatsDeps(t, nil)

	n := &model.Notice{
		Type:     "platform",
		Title:    "测试",
		Content:  "c",
		IsPopup:  true,
		StartAt:  time.Now(),
		Status:   "published",
	}
	seedNotice(t, deps.DB, n)

	// 标记已读需设置 user_id 上下文
	markRead := func() {
		g := setupGin()
		g.POST("/admin/notices/:id/read", func(c *gin.Context) {
			c.Set("user_id", uint64(777))
			c.Set("role", "admin")
			c.Next()
		}, MarkNoticeReadByPopup(deps, "admin"))

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/admin/notices/"+uintToStr(n.ID)+"/read", nil)
		g.ServeHTTP(w, req)
		require.Equal(t, http.StatusOK, w.Code)

		var resp struct {
			Code int                    `json:"code"`
			Data map[string]interface{} `json:"data"`
		}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		require.Equal(t, 0, resp.Code)
	}

	// 第一次调用
	markRead()
	// 第二次调用（幂等）
	markRead()

	// 验证 notice_read 表只有一条记录
	var count int64
	deps.DB.Model(&model.NoticeRead{}).
		Where("notice_id = ? AND user_type = ? AND user_id = ?", n.ID, "admin", 777).
		Count(&count)
	assert.Equal(t, int64(1), count, "幂等：重复调用只创建一条记录")
}

// uintToStr uint64 → string（避免引入 strconv）
func uintToStr(n uint64) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	pos := len(buf)
	for n > 0 {
		pos--
		buf[pos] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[pos:])
}

// ============== 2. 验证趋势测试 ==============

// TestAdminVerifyTrend_Empty 空表时返回 days 个 0 数据点
func TestAdminVerifyTrend_Empty(t *testing.T) {
	deps := setupNoticeStatsDeps(t, nil)

	data := callEndpoint(t, "GET", "/admin/stats/verify_trend", AdminVerifyTrend(deps), nil)
	assert.Equal(t, float64(30), data["days"], "默认 30 天")
	assert.Equal(t, float64(0), data["total"])

	trend, ok := data["trend"].([]interface{})
	require.True(t, ok)
	assert.Len(t, trend, 30, "应返回 30 个数据点")

	// 每个数据点应为 0
	first := trend[0].(map[string]interface{})
	assert.Equal(t, float64(0), first["total"])
	assert.Equal(t, float64(0), first["success"])
	assert.Equal(t, float64(0), first["fail"])
}

// TestAdminVerifyTrend_WithData 有数据时按 result 聚合
func TestAdminVerifyTrend_WithData(t *testing.T) {
	deps := setupNoticeStatsDeps(t, nil)

	now := time.Now()
	// 今日数据
	seedLogVerify(t, deps.DB, 1001, 2001, "login", "success", now)
	seedLogVerify(t, deps.DB, 1001, 2001, "login", "success", now)
	seedLogVerify(t, deps.DB, 1001, 2001, "verify", "fail", now)
	seedLogVerify(t, deps.DB, 1001, 2001, "heartbeat", "banned", now)
	// 3 天前数据
	seedLogVerify(t, deps.DB, 1001, 2001, "login", "success", now.Add(-3*24*time.Hour))
	seedLogVerify(t, deps.DB, 1001, 2001, "verify", "expired", now.Add(-3*24*time.Hour))

	data := callEndpoint(t, "GET", "/admin/stats/verify_trend", AdminVerifyTrend(deps), nil)
	assert.Equal(t, float64(6), data["total"])

	trend, ok := data["trend"].([]interface{})
	require.True(t, ok)
	assert.Len(t, trend, 30)

	// 最后一个数据点（今日）应有 4 个事件
	today := trend[len(trend)-1].(map[string]interface{})
	assert.Equal(t, float64(4), today["total"])
	assert.Equal(t, float64(2), today["success"])
	assert.Equal(t, float64(1), today["fail"])
	assert.Equal(t, float64(1), today["banned"])

	// 倒数第 4 个数据点（3 天前）应有 2 个事件
	day3 := trend[len(trend)-4].(map[string]interface{})
	assert.Equal(t, float64(2), day3["total"])
	assert.Equal(t, float64(1), day3["success"])
	assert.Equal(t, float64(1), day3["expired"])

	// action 维度聚合
	actionBreakdown, ok := data["action_breakdown"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, float64(3), actionBreakdown["login"])
	assert.Equal(t, float64(2), actionBreakdown["verify"])
	assert.Equal(t, float64(1), actionBreakdown["heartbeat"])
}

// TestAdminVerifyTrend_DaysParam days 参数受 sys_config 上下限约束
func TestAdminVerifyTrend_DaysParam(t *testing.T) {
	deps := setupNoticeStatsDeps(t, map[string]string{
		CfgKeyStatsVerifyTrendMaxDays:     "7",
		CfgKeyStatsVerifyTrendDefaultDays: "3",
	})

	// 不传 days：使用 default_days=3
	data := callEndpoint(t, "GET", "/admin/stats/verify_trend", AdminVerifyTrend(deps), nil)
	assert.Equal(t, float64(3), data["days"])

	// 传 days=100：受 max_days=7 限制
	g := setupGin()
	g.GET("/admin/stats/verify_trend", AdminVerifyTrend(deps))
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/admin/stats/verify_trend?days=100", nil)
	g.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	var resp struct {
		Code int                    `json:"code"`
		Data map[string]interface{} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, float64(7), resp.Data["days"], "超过 max_days 应被限制为 7")
}

// TestTenantVerifyTrend_Scope tenant 端仅查自己租户的验证日志
func TestTenantVerifyTrend_Scope(t *testing.T) {
	deps := setupNoticeStatsDeps(t, nil)

	now := time.Now()
	// 租户 1001 数据
	seedLogVerify(t, deps.DB, 1001, 2001, "login", "success", now)
	seedLogVerify(t, deps.DB, 1001, 2001, "login", "fail", now)
	// 租户 1002 数据（不应被统计）
	seedLogVerify(t, deps.DB, 1002, 2002, "login", "success", now)

	// 模拟 tenant 1001 调用
	g := setupGin()
	g.GET("/tenant/stats/verify_trend", func(c *gin.Context) {
		c.Set("tenant_id", uint64(1001))
		c.Set("user_id", uint64(1001))
		c.Next()
	}, TenantVerifyTrend(deps))

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/tenant/stats/verify_trend", nil)
	g.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	var resp struct {
		Code int                    `json:"code"`
		Data map[string]interface{} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Equal(t, 0, resp.Code)
	assert.Equal(t, float64(2), resp.Data["total"], "仅统计租户 1001 的 2 条日志")
}

// ============== 3. 代理业绩排行测试 ==============

// TestAdminAgentRanking_Empty 空表时返回空 list
func TestAdminAgentRanking_Empty(t *testing.T) {
	deps := setupNoticeStatsDeps(t, nil)

	data := callEndpoint(t, "GET", "/admin/stats/agent_ranking", AdminAgentRanking(deps), nil)
	list, ok := data["list"].([]interface{})
	require.True(t, ok)
	assert.Empty(t, list)
	assert.Equal(t, float64(10), data["limit"])
	assert.Equal(t, "total_amount", data["sort_by"])
}

// TestAdminAgentRanking_WithData 有数据时按 total_amount 排序
func TestAdminAgentRanking_WithData(t *testing.T) {
	deps := setupNoticeStatsDeps(t, nil)

	now := time.Now()
	// 准备 1 个租户 + 3 个代理
	seedTenant(t, deps.DB, 5001, "tenant-A")
	seedAgent(t, deps.DB, 6001, 5001, "agent-A", "代理 A")
	seedAgent(t, deps.DB, 6002, 5001, "agent-B", "代理 B")
	seedAgent(t, deps.DB, 6003, 5001, "agent-C", "代理 C")

	// 各代理的订单数据（注意：未 paid 的不应统计）
	seedOrder(t, deps.DB, 7001, 5001, 6001, 1000, 100, now) // A: 1000
	seedOrder(t, deps.DB, 7002, 5001, 6001, 500, 50, now)   // A: 500（累计 1500）
	seedOrder(t, deps.DB, 7003, 5001, 6002, 2000, 200, now) // B: 2000
	seedOrder(t, deps.DB, 7004, 5001, 6003, 800, 80, now)   // C: 800
	// 未付款订单不应统计
	pendingPaidAt := now.Add(-1 * time.Hour)
	seedOrder(t, deps.DB, 7005, 5001, 6001, 99999, 9999, pendingPaidAt)
	deps.DB.Model(&model.AppOrder{}).Where("id = ?", 7005).Update("pay_status", "pending")

	data := callEndpoint(t, "GET", "/admin/stats/agent_ranking", AdminAgentRanking(deps), nil)
	list, ok := data["list"].([]interface{})
	require.True(t, ok)
	require.Len(t, list, 3, "应返回 3 个代理")

	// 排序应为 B(2000) > A(1500) > C(800)
	first := list[0].(map[string]interface{})
	assert.Equal(t, "agent-B", first["username"])
	assert.Equal(t, float64(2000), first["total_amount"])
	assert.Equal(t, float64(1), first["rank"])

	second := list[1].(map[string]interface{})
	assert.Equal(t, "agent-A", second["username"])
	assert.Equal(t, float64(1500), second["total_amount"])
	assert.Equal(t, float64(2), second["rank"])

	third := list[2].(map[string]interface{})
	assert.Equal(t, "agent-C", third["username"])
	assert.Equal(t, float64(800), third["total_amount"])
	assert.Equal(t, float64(3), third["rank"])
}

// TestAdminAgentRanking_SortByCommission sort_by=commission 按佣金排序
func TestAdminAgentRanking_SortByCommission(t *testing.T) {
	deps := setupNoticeStatsDeps(t, nil)

	now := time.Now()
	seedTenant(t, deps.DB, 5001, "tenant-A")
	seedAgent(t, deps.DB, 6001, 5001, "agent-A", "代理 A")
	seedAgent(t, deps.DB, 6002, 5001, "agent-B", "代理 B")

	// A: 总额 1500，佣金 150
	seedOrder(t, deps.DB, 7001, 5001, 6001, 1000, 100, now)
	seedOrder(t, deps.DB, 7002, 5001, 6001, 500, 50, now)
	// B: 总额 1200，佣金 300
	seedOrder(t, deps.DB, 7003, 5001, 6002, 1200, 300, now)

	// 按 commission 排序，B 应在前
	g := setupGin()
	g.GET("/admin/stats/agent_ranking", AdminAgentRanking(deps))
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/admin/stats/agent_ranking?sort_by=commission", nil)
	g.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	var resp struct {
		Code int                    `json:"code"`
		Data map[string]interface{} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Equal(t, 0, resp.Code)
	assert.Equal(t, "commission", resp.Data["sort_by"])

	list, ok := resp.Data["list"].([]interface{})
	require.True(t, ok)
	require.Len(t, list, 2)
	first := list[0].(map[string]interface{})
	assert.Equal(t, "agent-B", first["username"], "佣金排序 B(300) 应在前")
	assert.Equal(t, float64(300), first["commission"])
}

// TestAdminAgentRanking_LimitParam limit 参数受 max_limit 配置限制
func TestAdminAgentRanking_LimitParam(t *testing.T) {
	deps := setupNoticeStatsDeps(t, map[string]string{
		CfgKeyStatsAgentRankingMaxLimit:     "5",
		CfgKeyStatsAgentRankingDefaultLimit: "3",
	})

	// 不传 limit：使用 default_limit=3
	data := callEndpoint(t, "GET", "/admin/stats/agent_ranking", AdminAgentRanking(deps), nil)
	assert.Equal(t, float64(3), data["limit"])

	// 传 limit=100：受 max_limit=5 限制
	g := setupGin()
	g.GET("/admin/stats/agent_ranking", AdminAgentRanking(deps))
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/admin/stats/agent_ranking?limit=100", nil)
	g.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	var resp struct {
		Code int                    `json:"code"`
		Data map[string]interface{} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, float64(5), resp.Data["limit"], "超过 max_limit 应被限制为 5")
}

// TestTenantAgentRanking_Scope tenant 端仅查自己租户的代理
func TestTenantAgentRanking_Scope(t *testing.T) {
	deps := setupNoticeStatsDeps(t, nil)

	now := time.Now()
	seedTenant(t, deps.DB, 5001, "tenant-A")
	seedTenant(t, deps.DB, 5002, "tenant-B")
	seedAgent(t, deps.DB, 6001, 5001, "agent-A1", "代理 A1")
	seedAgent(t, deps.DB, 6002, 5002, "agent-B1", "代理 B1") // 不应出现
	seedOrder(t, deps.DB, 7001, 5001, 6001, 1000, 100, now)
	seedOrder(t, deps.DB, 7002, 5002, 6002, 99999, 9999, now) // 不应统计

	// 模拟 tenant 5001 调用
	g := setupGin()
	g.GET("/tenant/stats/agent_ranking", func(c *gin.Context) {
		c.Set("tenant_id", uint64(5001))
		c.Set("user_id", uint64(5001))
		c.Next()
	}, TenantAgentRanking(deps))

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/tenant/stats/agent_ranking", nil)
	g.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	var resp struct {
		Code int                    `json:"code"`
		Data map[string]interface{} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Equal(t, 0, resp.Code)

	list, ok := resp.Data["list"].([]interface{})
	require.True(t, ok)
	require.Len(t, list, 1, "租户 5001 应只看到自己的 1 个代理")
	first := list[0].(map[string]interface{})
	assert.Equal(t, "agent-A1", first["username"])
}

// ============== 4. 配置键常量测试 ==============

// TestNoticeStatsConfigKeys 配置键常量应与 sys_config 中的 key 一致
func TestNoticeStatsConfigKeys(t *testing.T) {
	assert.Equal(t, "notice.popup.enabled", CfgKeyNoticePopupEnabled)
	assert.Equal(t, "notice.popup.max_unread", CfgKeyNoticePopupMaxUnread)
	assert.Equal(t, "notice.popup.dismiss_ttl_hours", CfgKeyNoticePopupDismissTTLHrs)
	assert.Equal(t, "notice.richtext.enabled", CfgKeyNoticeRichtextEnabled)
	assert.Equal(t, "notice.richtext.max_length", CfgKeyNoticeRichtextMaxLength)
	assert.Equal(t, "stats.verify_trend.default_days", CfgKeyStatsVerifyTrendDefaultDays)
	assert.Equal(t, "stats.verify_trend.max_days", CfgKeyStatsVerifyTrendMaxDays)
	assert.Equal(t, "stats.agent_ranking.default_limit", CfgKeyStatsAgentRankingDefaultLimit)
	assert.Equal(t, "stats.agent_ranking.max_limit", CfgKeyStatsAgentRankingMaxLimit)
}

// TestDefaultConstants 默认值常量校验
func TestDefaultConstants(t *testing.T) {
	assert.Equal(t, 5, defaultNoticePopupMaxUnread)
	assert.Equal(t, 24, defaultNoticePopupDismissTTLHrs)
	assert.Equal(t, 10000, defaultNoticeRichtextMaxLength)
	assert.Equal(t, 30, defaultStatsVerifyTrendDays)
	assert.Equal(t, 90, maxStatsVerifyTrendDays)
	assert.Equal(t, 10, defaultStatsAgentRankingLimit)
	assert.Equal(t, 100, maxStatsAgentRankingLimit)
}
