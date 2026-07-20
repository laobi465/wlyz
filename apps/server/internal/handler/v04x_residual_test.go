// Package handler v0.4.x 残留 4 项后端核心功能单元测试
// 覆盖：
//   1. S-04 应用审核：AdminListPendingApps / AdminAuditApp / AdminOfflineApp / AdminOnlineApp
//   2. D-15 开发者安全设置：TenantGetSecurity / TenantUpdateSecurity + TenantSecurityMiddleware（IP 黑名单 + 频率限制）
//   3. S-17 代理注册退款：AdminRefundAgentRegistration（事务：退款 + 禁用代理）
//   4. 月费订单：TenantGetMonthlyFeeCurrent + processMonthlyFeePaid（MFD 前缀分发）
//
// 严格遵循铁律 06：所有断言基于已知固定输入，无随机/不确定性
// 测试基础设施：SQLite 内存库 + miniredis（复用现有 setupGin 等辅助函数）
package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
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
	"github.com/your-org/keyauth-saas/apps/server/internal/middleware"
	"github.com/your-org/keyauth-saas/apps/server/internal/model"
	"github.com/your-org/keyauth-saas/apps/server/pkg/crypto"
	"github.com/your-org/keyauth-saas/apps/server/pkg/epay"
)

// ============== 测试基础设施 ==============

// setupV04xDB 启动独立 SQLite 内存库 + AutoMigrate v0.4.x 4 项功能所需的全部表
// 注：使用独立 DSN（file:v04x_test?mode=memory&cache=shared）避免与其他测试串扰
func setupV04xDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:v04x_test?mode=memory&cache=shared"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&model.SysConfig{},
		&model.SysAdmin{},
		&model.SysTenant{},
		&model.App{},
		&model.Agent{},
		&model.AgentRegistrationOrder{},
		&model.TenantSecurityConfig{},
		&model.TenantMonthlyFeeOrder{},
		&model.LogOperation{},
	))
	// 清空表（cache=shared 模式下可能残留旧数据）
	for _, t := range []string{
		"sys_config", "sys_admin", "sys_tenant", "app", "agent",
		"agent_registration_order", "tenant_security_config",
		"tenant_monthly_fee_order", "log_operation",
	} {
		db.Exec("DELETE FROM " + t)
	}
	return db
}

// setupV04xCrypto 启动 AES-256 crypto manager（ConfigCache.Set 加密配置值需要）
func setupV04xCrypto(t *testing.T) *crypto.Manager {
	t.Helper()
	mgr, err := crypto.NewManager("0123456789abcdef0123456789abcdef", "", "")
	require.NoError(t, err)
	return mgr
}

// setupV04xMiniRedis 启动 miniredis + 返回 redis.Client
func setupV04xMiniRedis(t *testing.T) *redis.Client {
	t.Helper()
	mr, err := miniredis.Run()
	require.NoError(t, err)
	t.Cleanup(mr.Close)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	return rdb
}

// setupV04xDeps 构造测试 Deps（含真实 ConfigCache + 注入 sys_config 默认值）
func setupV04xDeps(t *testing.T, cfgOverrides map[string]string) *Deps {
	t.Helper()
	rdb := setupV04xMiniRedis(t)
	db := setupV04xDB(t)

	// 注入默认 sys_config（v0.4.x 4 项功能所需）
	defaults := map[string]string{
		"app.audit.enabled":               "0",
		"pay.tenant_monthly_fee.enabled":  "0",
		"pay.tenant_monthly_fee.amount":   "50.00",
		"pay.tenant_monthly_fee.free_days": "30",
		"pay.platform.enabled":            "1",
		"pay.platform.gateway_url":        "https://epay.example.com",
		"pay.platform.pid":               "1001",
		"pay.platform.key_encrypted":     "", // 单测中解密失败由 handler 内部 fail，不影响 list/get 类测试
		"pay.platform.sign_type":         "MD5",
		"pay.platform.order_name_prefix": "KeyAuth",
		"pay.platform.notify_path":       "/api/v1/pay/notify/epay",
		"pay.platform.return_path":       "/api/v1/pay/return/epay",
	}
	if cfgOverrides != nil {
		for k, v := range cfgOverrides {
			defaults[k] = v
		}
	}
	for k, v := range defaults {
		require.NoError(t, db.Create(&model.SysConfig{
			ConfigKey:   k,
			ConfigValue: v,
			ConfigType:  "string",
			ConfigGroup: "test",
		}).Error)
	}

	cfgCache := config.NewConfigCache(db, rdb)
	require.NoError(t, cfgCache.Preload(context.Background()))

	return &Deps{
		DB:       db,
		Redis:    rdb,
		Crypto:   setupV04xCrypto(t),
		CfgCache: cfgCache,
	}
}

// ============== 通用辅助 ==============

// setupAdminTestRouter 注册所有需要测试的 admin 路由，返回 gin.Engine
// 注入 admin 上下文（role=user_id=username），避免每个测试重复注册
func setupAdminTestRouter(deps *Deps, adminID uint64) *gin.Engine {
	g := setupGin()
	inject := func(c *gin.Context) {
		c.Set("role", "admin")
		c.Set("user_id", adminID)
		c.Set("username", "admin_root")
		c.Next()
	}
	g.GET("/admin/apps/pending", inject, AdminListPendingApps(deps))
	g.POST("/admin/apps/:id/audit", inject, AdminAuditApp(deps))
	g.POST("/admin/apps/:id/offline", inject, AdminOfflineApp(deps))
	g.POST("/admin/apps/:id/online", inject, AdminOnlineApp(deps))
	g.POST("/admin/agent_registrations/:id/refund", inject, AdminRefundAgentRegistration(deps))
	g.POST("/admin/monthly_fee_orders/:id/mark_paid", inject, AdminMarkMonthlyFeePaid(deps))
	return g
}

// callAdminRoute 使用 setupAdminTestRouter 调用任意 admin 路由
func callAdminRoute(t *testing.T, deps *Deps, method, path string, body interface{}, adminID uint64) *httptest.ResponseRecorder {
	t.Helper()
	g := setupAdminTestRouter(deps, adminID)
	w := httptest.NewRecorder()
	var req *http.Request
	if body != nil {
		b, _ := json.Marshal(body)
		req, _ = http.NewRequest(method, path, strings.NewReader(string(b)))
		req.Header.Set("Content-Type", "application/json")
	} else {
		req, _ = http.NewRequest(method, path, nil)
	}
	g.ServeHTTP(w, req)
	return w
}

// callAdminJSON 兼容旧调用名（保留以防其他地方引用）
func callAdminJSON(t *testing.T, deps *Deps, method, path string, body interface{}, adminID uint64) *httptest.ResponseRecorder {
	return callAdminRoute(t, deps, method, path, body, adminID)
}

// callTenantJSON 模拟 tenant 上下文调用端点（注入 tenant_id），GET/PUT/POST JSON body
func callTenantJSON(t *testing.T, deps *Deps, method, path string, body interface{}, tenantID uint64) *httptest.ResponseRecorder {
	t.Helper()
	g := setupGin()
	var handler gin.HandlerFunc
	switch path {
	case "/tenant/security":
		if method == "GET" {
			handler = TenantGetSecurity(deps)
		} else {
			handler = TenantUpdateSecurity(deps)
		}
	case "/tenant/monthly_fee/current":
		handler = TenantGetMonthlyFeeCurrent(deps)
	case "/tenant/monthly_fee/pay":
		handler = TenantPayMonthlyFee(deps)
	default:
		handler = func(c *gin.Context) { c.String(http.StatusNotFound, "no test route") }
	}
	g.Handle(method, path, func(c *gin.Context) {
		c.Set("role", "tenant")
		c.Set("tenant_id", tenantID)
		c.Set("user_id", tenantID)
		c.Set("username", "tenant_user")
		c.Next()
	}, handler)

	w := httptest.NewRecorder()
	var req *http.Request
	if body != nil {
		b, _ := json.Marshal(body)
		req, _ = http.NewRequest(method, path, strings.NewReader(string(b)))
		req.Header.Set("Content-Type", "application/json")
	} else {
		req, _ = http.NewRequest(method, path, nil)
	}
	g.ServeHTTP(w, req)
	return w
}

// parseResp 解析响应体为 {code, message, data} 结构
func parseResp(t *testing.T, w *httptest.ResponseRecorder) (int, string, map[string]interface{}) {
	t.Helper()
	var resp struct {
		Code    int                    `json:"code"`
		Message string                 `json:"message"`
		Data    map[string]interface{} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	return resp.Code, resp.Message, resp.Data
}

// ============== 测试种子数据 ==============

func seedTenantForV04x(t *testing.T, db *gorm.DB, id uint64, username string) *model.SysTenant {
	t.Helper()
	tt := &model.SysTenant{
		BaseModel: model.BaseModel{ID: id},
		Username:  username,
		Status:    "active",
	}
	require.NoError(t, db.Create(tt).Error)
	return tt
}

func seedAppForV04x(t *testing.T, db *gorm.DB, id, tenantID uint64, name, auditStatus string) *model.App {
	t.Helper()
	a := &model.App{
		BaseModel:   model.BaseModel{ID: id},
		TenantID:    tenantID,
		AppKey:      "ak_" + name,
		AppSecret:   "secret_enc",
		SignSecret:  "sign_enc",
		Name:        name,
		Status:      "active",
		AuditStatus: auditStatus,
	}
	require.NoError(t, db.Create(a).Error)
	return a
}

func seedAgentRegistrationOrder(t *testing.T, db *gorm.DB, id, tenantID uint64, orderNo string, amount float64, payStatus string, agentID *uint64) *model.AgentRegistrationOrder {
	t.Helper()
	o := &model.AgentRegistrationOrder{
		BaseModel:    model.BaseModel{ID: id},
		OrderNo:      orderNo,
		InviteCodeID: 0,
		TenantID:     tenantID,
		AgentID:      agentID,
		Username:     "agent_" + orderNo,
		Amount:       amount,
		PayChannel:   "epay_alipay",
		PayStatus:    payStatus,
		ClientIP:     "127.0.0.1",
		RefundStatus: "none",
	}
	require.NoError(t, db.Create(o).Error)
	return o
}

func seedAgentForV04x(t *testing.T, db *gorm.DB, id, tenantID uint64, username, status string) *model.Agent {
	t.Helper()
	a := &model.Agent{
		BaseModel:    model.BaseModel{ID: id},
		TenantID:     tenantID,
		Username:     username,
		PasswordHash: "bcrypt_hash",
		Status:       status,
	}
	require.NoError(t, db.Create(a).Error)
	return a
}

func seedMonthlyFeeOrder(t *testing.T, db *gorm.DB, id, tenantID uint64, orderNo string, amount float64, payStatus string) *model.TenantMonthlyFeeOrder {
	t.Helper()
	now := time.Now()
	periodStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	periodEnd := periodStart.AddDate(0, 1, 0).Add(-time.Second)
	o := &model.TenantMonthlyFeeOrder{
		BaseModel:   model.BaseModel{ID: id},
		TenantID:    tenantID,
		PeriodStart: periodStart,
		PeriodEnd:   periodEnd,
		Amount:      amount,
		PayStatus:   payStatus,
		PayMode:     "platform_epay",
		OrderNo:     orderNo,
	}
	require.NoError(t, db.Create(o).Error)
	return o
}

// ============== 任务 1：S-04 应用审核单元测试 ==============

// TestAdminListPendingApps_Default 待审核应用列表返回 pending 状态的应用
func TestAdminListPendingApps_Default(t *testing.T) {
	deps := setupV04xDeps(t, nil)
	seedAppForV04x(t, deps.DB, 1001, 5001, "AppPending1", "pending")
	seedAppForV04x(t, deps.DB, 1002, 5001, "AppApproved", "approved")

	w := callAdminJSON(t, deps, "GET", "/admin/apps/pending", nil, 1)
	require.Equal(t, http.StatusOK, w.Code, "应返回 200")

	code, _, data := parseResp(t, w)
	require.Equal(t, 0, code)
	list := data["list"].([]interface{})
	assert.Len(t, list, 1, "仅返回 pending 状态应用")
	item := list[0].(map[string]interface{})
	assert.Equal(t, "AppPending1", item["name"])
	assert.Equal(t, "pending", item["audit_status"])

	total, _ := data["total"].(float64)
	assert.Equal(t, float64(1), total)
}

// TestAdminAuditApp_Approve 审核通过：pending → approved
func TestAdminAuditApp_Approve(t *testing.T) {
	deps := setupV04xDeps(t, nil)
	seedAppForV04x(t, deps.DB, 2001, 5001, "AppToApprove", "pending")

	body := map[string]interface{}{
		"status": "approved",
		"remark": "审核通过，应用规范",
	}
	w := callAdminJSON(t, deps, "POST", "/admin/apps/2001/audit", body, 1)
	require.Equal(t, http.StatusOK, w.Code)

	code, _, data := parseResp(t, w)
	require.Equal(t, 0, code)
	assert.Equal(t, "approved", data["audit_status"])

	// 校验 DB 状态已更新
	var app model.App
	require.NoError(t, deps.DB.First(&app, 2001).Error)
	assert.Equal(t, "approved", app.AuditStatus)
	assert.Equal(t, "审核通过，应用规范", app.AuditRemark)
	assert.Equal(t, uint64(1), app.AuditedBy)
	require.NotNil(t, app.AuditedAt)
}

// TestAdminAuditApp_Reject 审核驳回：pending → rejected
func TestAdminAuditApp_Reject(t *testing.T) {
	deps := setupV04xDeps(t, nil)
	seedAppForV04x(t, deps.DB, 2002, 5001, "AppToReject", "pending")

	body := map[string]interface{}{
		"status": "rejected",
		"remark": "应用描述违规",
	}
	w := callAdminJSON(t, deps, "POST", "/admin/apps/2002/audit", body, 1)
	require.Equal(t, http.StatusOK, w.Code)

	code, _, data := parseResp(t, w)
	require.Equal(t, 0, code)
	assert.Equal(t, "rejected", data["audit_status"])

	var app model.App
	require.NoError(t, deps.DB.First(&app, 2002).Error)
	assert.Equal(t, "rejected", app.AuditStatus)
	assert.Equal(t, "应用描述违规", app.AuditRemark)
}

// TestAdminAuditApp_AlreadyAudited 重复审核已通过的应用应拒绝
func TestAdminAuditApp_AlreadyAudited(t *testing.T) {
	deps := setupV04xDeps(t, nil)
	seedAppForV04x(t, deps.DB, 2003, 5001, "AppAlreadyApproved", "approved")

	body := map[string]interface{}{"status": "rejected", "remark": "再次审核"}
	w := callAdminJSON(t, deps, "POST", "/admin/apps/2003/audit", body, 1)
	// middleware.Fail 设置 HTTP 400
	assert.Equal(t, http.StatusBadRequest, w.Code)

	code, msg, _ := parseResp(t, w)
	assert.NotEqual(t, 0, code, "已审核应用应返回错误码")
	assert.Contains(t, msg, "已审核")
}

// TestAdminOfflineApp 违规下架：app.status → disabled
func TestAdminOfflineApp(t *testing.T) {
	deps := setupV04xDeps(t, nil)
	seedAppForV04x(t, deps.DB, 3001, 5001, "AppToOffline", "approved")

	// AdminOfflineApp 需要 reason 字段（binding:"required"）
	body := map[string]interface{}{"reason": "违规应用，需下架"}
	w := callAdminJSON(t, deps, "POST", "/admin/apps/3001/offline", body, 1)
	require.Equal(t, http.StatusOK, w.Code)

	code, _, data := parseResp(t, w)
	require.Equal(t, 0, code)
	assert.Equal(t, "disabled", data["status"])

	var app model.App
	require.NoError(t, deps.DB.First(&app, 3001).Error)
	assert.Equal(t, "disabled", app.Status)
}

// TestAdminOnlineApp 重新上架：app.status → active
func TestAdminOnlineApp(t *testing.T) {
	deps := setupV04xDeps(t, nil)
	// 直接 seed 一个 disabled 应用
	a := seedAppForV04x(t, deps.DB, 3002, 5001, "AppToOnline", "approved")
	a.Status = "disabled"
	require.NoError(t, deps.DB.Save(a).Error)

	w := callAdminJSON(t, deps, "POST", "/admin/apps/3002/online", nil, 1)
	require.Equal(t, http.StatusOK, w.Code)

	code, _, data := parseResp(t, w)
	require.Equal(t, 0, code)
	assert.Equal(t, "active", data["status"])

	var app model.App
	require.NoError(t, deps.DB.First(&app, 3002).Error)
	assert.Equal(t, "active", app.Status)
}

// ============== 任务 2：D-15 开发者安全设置单元测试 ==============

// TestTenantGetSecurity_Empty 无配置时返回空安全配置
func TestTenantGetSecurity_Empty(t *testing.T) {
	deps := setupV04xDeps(t, nil)
	w := callTenantJSON(t, deps, "GET", "/tenant/security", nil, 5001)
	require.Equal(t, http.StatusOK, w.Code)

	code, _, data := parseResp(t, w)
	require.Equal(t, 0, code)
	assert.Equal(t, float64(5001), data["tenant_id"])
	assert.Equal(t, []interface{}{}, data["ip_blacklist"])
	assert.Equal(t, float64(0), data["verify_rate_limit_per_min"])
	assert.Equal(t, float64(0), data["login_rate_limit_per_min"])
}

// TestTenantUpdateSecurity_IPBlacklist 设置 IP 黑名单 + 限速阈值
func TestTenantUpdateSecurity_IPBlacklist(t *testing.T) {
	deps := setupV04xDeps(t, nil)
	verifyLimit := 100
	loginLimit := 50
	body := map[string]interface{}{
		"ip_blacklist":              []string{"1.2.3.4", "10.0.0.0/8"},
		"verify_rate_limit_per_min": verifyLimit,
		"login_rate_limit_per_min":  loginLimit,
	}
	w := callTenantJSON(t, deps, "PUT", "/tenant/security", body, 5002)
	require.Equal(t, http.StatusOK, w.Code)

	// 校验 DB 已落库
	var sec model.TenantSecurityConfig
	require.NoError(t, deps.DB.Where("tenant_id = ?", 5002).First(&sec).Error)
	assert.Equal(t, `["1.2.3.4","10.0.0.0/8"]`, sec.IPBlacklist)
	assert.Equal(t, 100, sec.VerifyRateLimitPerMin)
	assert.Equal(t, 50, sec.LoginRateLimitPerMin)

	// 再次查询应返回新配置
	w2 := callTenantJSON(t, deps, "GET", "/tenant/security", nil, 5002)
	require.Equal(t, http.StatusOK, w2.Code)
	_, _, data2 := parseResp(t, w2)
	ipList := data2["ip_blacklist"].([]interface{})
	assert.Len(t, ipList, 2)
	assert.Equal(t, "1.2.3.4", ipList[0])
	assert.Equal(t, "10.0.0.0/8", ipList[1])
	assert.Equal(t, float64(100), data2["verify_rate_limit_per_min"])
}

// TestTenantUpdateSecurity_InvalidIP 非法 IP 格式应返回 400
func TestTenantUpdateSecurity_InvalidIP(t *testing.T) {
	deps := setupV04xDeps(t, nil)
	body := map[string]interface{}{
		"ip_blacklist": []string{"not-an-ip"},
	}
	w := callTenantJSON(t, deps, "PUT", "/tenant/security", body, 5003)
	// middleware.Fail 设置 HTTP 400 + JSON 错误体
	assert.Equal(t, http.StatusBadRequest, w.Code)

	code, msg, _ := parseResp(t, w)
	assert.NotEqual(t, 0, code)
	assert.Contains(t, msg, "IP 黑名单格式错误")
}

// TestTenantSecurityMiddleware_IPBlacklist 黑名单 IP 应被拦截
func TestTenantSecurityMiddleware_IPBlacklist(t *testing.T) {
	deps := setupV04xDeps(t, nil)
	// 准备：开发者 5001 配置 IP 黑名单 ["1.2.3.4"]
	require.NoError(t, deps.DB.Create(&model.TenantSecurityConfig{
		TenantID:    5001,
		IPBlacklist: `["1.2.3.4"]`,
	}).Error)
	app := seedAppForV04x(t, deps.DB, 6001, 5001, "AppSecTest", "approved")

	g := setupGin()
	g.POST("/client/verify", func(c *gin.Context) {
		// 模拟 SignatureAuth 注入 app
		c.Set("app", app)
		c.Next()
	}, middleware.TenantSecurityMiddleware(deps.DB, deps.Redis, "verify"), func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	// 1.2.3.4 应被 403 拦截
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/client/verify", nil)
	req.RemoteAddr = "1.2.3.4:12345"
	g.ServeHTTP(w, req)
	assert.Equal(t, http.StatusForbidden, w.Code, "黑名单 IP 应返回 403")

	// 5.6.7.8 应放行
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest("POST", "/client/verify", nil)
	req2.RemoteAddr = "5.6.7.8:12345"
	g.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusOK, w2.Code, "非黑名单 IP 应放行")
}

// TestTenantSecurityMiddleware_RateLimit 超过频率限制应返回 429
func TestTenantSecurityMiddleware_RateLimit(t *testing.T) {
	deps := setupV04xDeps(t, nil)
	// 准备：开发者 5001 配置 verify 限速 3/min
	require.NoError(t, deps.DB.Create(&model.TenantSecurityConfig{
		TenantID:                 5001,
		IPBlacklist:              "[]",
		VerifyRateLimitPerMin:    3,
		LoginRateLimitPerMin:     0,
	}).Error)
	app := seedAppForV04x(t, deps.DB, 6002, 5001, "AppRateLimit", "approved")

	g := setupGin()
	g.POST("/client/verify", func(c *gin.Context) {
		c.Set("app", app)
		c.Next()
	}, middleware.TenantSecurityMiddleware(deps.DB, deps.Redis, "verify"), func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	// 同一 IP 连续请求 4 次：前 3 次 200，第 4 次 429
	for i := 1; i <= 4; i++ {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/client/verify", nil)
		req.RemoteAddr = "9.9.9.9:1000"
		g.ServeHTTP(w, req)
		if i <= 3 {
			assert.Equal(t, http.StatusOK, w.Code, "第 %d 次请求应放行", i)
		} else {
			assert.Equal(t, http.StatusTooManyRequests, w.Code, "第 4 次请求应被限速")
		}
	}
}

// TestTenantSecurityMiddleware_NoConfig 无安全配置应放行（向后兼容）
func TestTenantSecurityMiddleware_NoConfig(t *testing.T) {
	deps := setupV04xDeps(t, nil)
	app := seedAppForV04x(t, deps.DB, 6003, 5001, "AppNoSec", "approved")

	g := setupGin()
	g.POST("/client/verify", func(c *gin.Context) {
		c.Set("app", app)
		c.Next()
	}, middleware.TenantSecurityMiddleware(deps.DB, deps.Redis, "verify"), func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/client/verify", nil)
	req.RemoteAddr = "8.8.8.8:12345"
	g.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code, "无安全配置应放行")
}

// ============== 任务 3：S-17 代理注册退款单元测试 ==============

// TestAdminRefundAgentRegistration_Success 退款事务成功：订单 refunded + Agent disabled
func TestAdminRefundAgentRegistration_Success(t *testing.T) {
	deps := setupV04xDeps(t, nil)
	seedTenantForV04x(t, deps.DB, 5001, "tenant_refund")
	agentID := uint64(7001)
	seedAgentForV04x(t, deps.DB, agentID, 5001, "agent_refund", "active")
	seedAgentRegistrationOrder(t, deps.DB, 8001, 5001, "REG-8001", 100.00, "paid", &agentID)

	body := map[string]interface{}{"reason": "用户申请退款"}
	w := callAdminJSON(t, deps, "POST", "/admin/agent_registrations/8001/refund", body, 1)
	require.Equal(t, http.StatusOK, w.Code)

	code, _, data := parseResp(t, w)
	require.Equal(t, 0, code)
	assert.Equal(t, "refunded", data["refund_status"])
	refundAmount, _ := data["refund_amount"].(float64)
	assert.Equal(t, float64(100.00), refundAmount)

	// 校验订单已 refunded
	var order model.AgentRegistrationOrder
	require.NoError(t, deps.DB.First(&order, 8001).Error)
	assert.Equal(t, "refunded", order.RefundStatus)
	assert.Equal(t, 100.00, order.RefundAmount)
	require.NotNil(t, order.RefundAt)
	assert.Equal(t, uint64(1), order.RefundBy)
	assert.Equal(t, "用户申请退款", order.RefundReason)

	// 校验 Agent 已被禁用（事务内同步）
	var agent model.Agent
	require.NoError(t, deps.DB.First(&agent, agentID).Error)
	assert.Equal(t, "disabled", agent.Status, "退款后代理账号应被禁用")
}

// TestAdminRefundAgentRegistration_Unpaid 未支付订单不能退款
func TestAdminRefundAgentRegistration_Unpaid(t *testing.T) {
	deps := setupV04xDeps(t, nil)
	seedTenantForV04x(t, deps.DB, 5001, "tenant_refund2")
	// 未支付订单
	seedAgentRegistrationOrder(t, deps.DB, 8002, 5001, "REG-8002", 100.00, "pending", nil)

	body := map[string]interface{}{"reason": "测试未支付退款"}
	w := callAdminJSON(t, deps, "POST", "/admin/agent_registrations/8002/refund", body, 1)
	// middleware.Fail 设置 HTTP 400
	assert.Equal(t, http.StatusBadRequest, w.Code)

	code, msg, _ := parseResp(t, w)
	assert.NotEqual(t, 0, code)
	assert.Contains(t, msg, "未支付")
}

// TestAdminRefundAgentRegistration_AlreadyRefunded 已退款订单不能重复退款
func TestAdminRefundAgentRegistration_AlreadyRefunded(t *testing.T) {
	deps := setupV04xDeps(t, nil)
	seedTenantForV04x(t, deps.DB, 5001, "tenant_refund3")
	agentID := uint64(7002)
	seedAgentForV04x(t, deps.DB, agentID, 5001, "agent_refunded", "disabled")
	// 已退款订单
	o := seedAgentRegistrationOrder(t, deps.DB, 8003, 5001, "REG-8003", 100.00, "paid", &agentID)
	o.RefundStatus = "refunded"
	o.RefundAmount = 100.00
	now := time.Now()
	o.RefundAt = &now
	o.RefundBy = 1
	o.RefundReason = "已退款"
	require.NoError(t, deps.DB.Save(o).Error)

	body := map[string]interface{}{"reason": "重复退款测试"}
	w := callAdminJSON(t, deps, "POST", "/admin/agent_registrations/8003/refund", body, 1)
	// middleware.Fail 设置 HTTP 400
	assert.Equal(t, http.StatusBadRequest, w.Code)

	code, msg, _ := parseResp(t, w)
	assert.NotEqual(t, 0, code)
	assert.Contains(t, msg, "已退款")
}

// TestAdminRefundAgentRegistration_NoAgent 无关联代理时仅更新订单（不报错）
func TestAdminRefundAgentRegistration_NoAgent(t *testing.T) {
	deps := setupV04xDeps(t, nil)
	seedTenantForV04x(t, deps.DB, 5001, "tenant_refund4")
	// 订单未关联 agent_id（agent_id=NULL）
	seedAgentRegistrationOrder(t, deps.DB, 8004, 5001, "REG-8004", 50.00, "paid", nil)

	body := map[string]interface{}{"reason": "无关联代理退款"}
	w := callAdminJSON(t, deps, "POST", "/admin/agent_registrations/8004/refund", body, 1)
	require.Equal(t, http.StatusOK, w.Code)

	code, _, _ := parseResp(t, w)
	require.Equal(t, 0, code)

	var order model.AgentRegistrationOrder
	require.NoError(t, deps.DB.First(&order, 8004).Error)
	assert.Equal(t, "refunded", order.RefundStatus)
	assert.Equal(t, 50.00, order.RefundAmount)
}

// ============== 任务 4：月费订单单元测试 ==============

// TestTenantGetMonthlyFeeCurrent_Disabled 月费未启用时返回 enabled=false
func TestTenantGetMonthlyFeeCurrent_Disabled(t *testing.T) {
	deps := setupV04xDeps(t, nil) // 默认 pay.tenant_monthly_fee.enabled=0
	seedTenantForV04x(t, deps.DB, 5001, "tenant_mf1")

	w := callTenantJSON(t, deps, "GET", "/tenant/monthly_fee/current", nil, 5001)
	require.Equal(t, http.StatusOK, w.Code)

	code, _, data := parseResp(t, w)
	require.Equal(t, 0, code)
	assert.Equal(t, false, data["enabled"])
	assert.Equal(t, float64(50.00), data["amount"])
	assert.Equal(t, float64(30), data["free_days"])
}

// TestTenantGetMonthlyFeeCurrent_Enabled 启用月费后返回当前账单周期
func TestTenantGetMonthlyFeeCurrent_Enabled(t *testing.T) {
	deps := setupV04xDeps(t, map[string]string{
		"pay.tenant_monthly_fee.enabled": "1",
	})
	seedTenantForV04x(t, deps.DB, 5001, "tenant_mf2")

	w := callTenantJSON(t, deps, "GET", "/tenant/monthly_fee/current", nil, 5001)
	require.Equal(t, http.StatusOK, w.Code)

	code, _, data := parseResp(t, w)
	require.Equal(t, 0, code)
	assert.Equal(t, true, data["enabled"])

	// period_start/period_end 应为月初/月末
	now := time.Now()
	expectedStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	expectedEnd := expectedStart.AddDate(0, 1, 0).Add(-time.Second)
	assert.Equal(t, float64(expectedStart.Unix()), data["period_start"])
	assert.Equal(t, float64(expectedEnd.Unix()), data["period_end"])
}

// TestTenantGetMonthlyFeeCurrent_WithPendingOrder 有 pending 订单时返回 current_order
func TestTenantGetMonthlyFeeCurrent_WithPendingOrder(t *testing.T) {
	deps := setupV04xDeps(t, map[string]string{
		"pay.tenant_monthly_fee.enabled": "1",
	})
	seedTenantForV04x(t, deps.DB, 5001, "tenant_mf3")
	seedMonthlyFeeOrder(t, deps.DB, 9001, 5001, "MFD-9001", 50.00, "pending")

	w := callTenantJSON(t, deps, "GET", "/tenant/monthly_fee/current", nil, 5001)
	require.Equal(t, http.StatusOK, w.Code)

	_, _, data := parseResp(t, w)
	currentOrder := data["current_order"].(map[string]interface{})
	assert.Equal(t, "MFD-9001", currentOrder["order_no"])
	assert.Equal(t, "pending", currentOrder["pay_status"])
	assert.Equal(t, float64(1), data["pending_count"])
	assert.Equal(t, float64(50.00), data["pending_amount"])
}

// TestTenantPayMonthlyFee_Disabled 月费未启用时禁止支付
func TestTenantPayMonthlyFee_Disabled(t *testing.T) {
	deps := setupV04xDeps(t, nil) // enabled=0
	seedTenantForV04x(t, deps.DB, 5001, "tenant_mf_pay1")

	body := map[string]interface{}{"pay_type": "alipay"}
	w := callTenantJSON(t, deps, "POST", "/tenant/monthly_fee/pay", body, 5001)
	// middleware.Fail 设置 HTTP 403 + JSON 错误体
	assert.Equal(t, http.StatusForbidden, w.Code)

	code, msg, _ := parseResp(t, w)
	assert.NotEqual(t, 0, code)
	assert.Contains(t, msg, "未启用")
}

// TestTenantPayMonthlyFee_CreatesOrder 启用月费后发起支付应创建订单
// 注：平台易支付 key_encrypted 为空会导致构造支付 URL 失败，这里仅断言订单创建成功
func TestTenantPayMonthlyFee_CreatesOrder(t *testing.T) {
	deps := setupV04xDeps(t, map[string]string{
		"pay.tenant_monthly_fee.enabled": "1",
	})
	seedTenantForV04x(t, deps.DB, 5001, "tenant_mf_pay2")

	body := map[string]interface{}{"pay_type": "alipay"}
	w := callTenantJSON(t, deps, "POST", "/tenant/monthly_fee/pay", body, 5001)
	// 由于 key_encrypted 为空，handler 会返回 500 错误（middleware.Fail 设置 HTTP 500）
	// 但订单应已创建（在构造 URL 之前已 Create 入库）
	// 接受 200（成功）或 500（构造 URL 失败）两种结果
	if w.Code != http.StatusOK && w.Code != http.StatusInternalServerError {
		t.Fatalf("期望 200 或 500，实际 %d", w.Code)
	}

	if w.Code == http.StatusOK {
		// 订单创建成功 + 返回 pay_url
		code, _, data := parseResp(t, w)
		require.Equal(t, 0, code)
		orderNo, _ := data["order_no"].(string)
		assert.True(t, strings.HasPrefix(orderNo, "MFD"), "订单号应以 MFD 前缀开头")
	} else {
		// 构造 URL 失败但订单应已创建（幂等：下次可重试）
		var errResp struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &errResp))
		assert.Contains(t, errResp.Message, "支付配置错误")
	}

	// 校验订单已落库
	var order model.TenantMonthlyFeeOrder
	require.NoError(t, deps.DB.Where("tenant_id = ?", 5001).First(&order).Error)
	assert.True(t, strings.HasPrefix(order.OrderNo, "MFD"))
	assert.Equal(t, 50.00, order.Amount)
	assert.Equal(t, "pending", order.PayStatus)
}

// TestProcessMonthlyFeePaid_Success 月费订单支付回调成功：pending → paid
func TestProcessMonthlyFeePaid_Success(t *testing.T) {
	deps := setupV04xDeps(t, nil)
	seedTenantForV04x(t, deps.DB, 5001, "tenant_mf_cb1")
	seedMonthlyFeeOrder(t, deps.DB, 9101, 5001, "MFD-9101", 50.00, "pending")

	// 模拟易支付回调
	notify := &epay.NotifyParams{
		OutTradeNo: "MFD-9101",
		TradeNo:    "epay-trade-9101",
		Money:      "50.00",
	}
	err := processMonthlyFeePaid(deps, notify)
	require.NoError(t, err)

	// 校验订单已 paid
	var order model.TenantMonthlyFeeOrder
	require.NoError(t, deps.DB.First(&order, 9101).Error)
	assert.Equal(t, "paid", order.PayStatus)
	assert.Equal(t, "platform_epay", order.PayMode)
	require.NotNil(t, order.PaidAt)
}

// TestProcessMonthlyFeePaid_Idempotent 重复回调应幂等返回 nil
func TestProcessMonthlyFeePaid_Idempotent(t *testing.T) {
	deps := setupV04xDeps(t, nil)
	seedTenantForV04x(t, deps.DB, 5001, "tenant_mf_cb2")
	seedMonthlyFeeOrder(t, deps.DB, 9102, 5001, "MFD-9102", 50.00, "paid")

	notify := &epay.NotifyParams{
		OutTradeNo: "MFD-9102",
		TradeNo:    "epay-trade-9102",
		Money:      "50.00",
	}
	err := processMonthlyFeePaid(deps, notify)
	assert.NoError(t, err, "已支付订单重复回调应幂等返回 nil")
}

// TestProcessMonthlyFeePaid_AmountMismatch 金额不匹配应返回错误
func TestProcessMonthlyFeePaid_AmountMismatch(t *testing.T) {
	deps := setupV04xDeps(t, nil)
	seedTenantForV04x(t, deps.DB, 5001, "tenant_mf_cb3")
	seedMonthlyFeeOrder(t, deps.DB, 9103, 5001, "MFD-9103", 50.00, "pending")

	notify := &epay.NotifyParams{
		OutTradeNo: "MFD-9103",
		TradeNo:    "epay-trade-9103",
		Money:      "999.99", // 金额不符
	}
	err := processMonthlyFeePaid(deps, notify)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "金额不匹配")

	// 订单状态应仍为 pending
	var order model.TenantMonthlyFeeOrder
	require.NoError(t, deps.DB.First(&order, 9103).Error)
	assert.Equal(t, "pending", order.PayStatus)
}

// TestDispatchPaidOrder_MFDPrefix 验证 MFD 前缀能正确分发到 processMonthlyFeePaid
func TestDispatchPaidOrder_MFDPrefix(t *testing.T) {
	deps := setupV04xDeps(t, nil)
	seedTenantForV04x(t, deps.DB, 5001, "tenant_dispatch")
	seedMonthlyFeeOrder(t, deps.DB, 9104, 5001, "MFD-9104", 50.00, "pending")

	notify := &epay.NotifyParams{
		OutTradeNo: "MFD-9104",
		TradeNo:    "epay-trade-9104",
		Money:      "50.00",
	}
	err := dispatchPaidOrder(deps, notify)
	require.NoError(t, err)

	var order model.TenantMonthlyFeeOrder
	require.NoError(t, deps.DB.First(&order, 9104).Error)
	assert.Equal(t, "paid", order.PayStatus, "MFD 前缀应通过 dispatchPaidOrder 正确分发并标记 paid")
}

// TestAdminMarkMonthlyFeePaid_Success 超管手动标记月费订单已支付
func TestAdminMarkMonthlyFeePaid_Success(t *testing.T) {
	deps := setupV04xDeps(t, nil)
	seedTenantForV04x(t, deps.DB, 5001, "tenant_mark")
	seedMonthlyFeeOrder(t, deps.DB, 9105, 5001, "MFD-9105", 50.00, "pending")

	w := callAdminJSON(t, deps, "POST", "/admin/monthly_fee_orders/9105/mark_paid", nil, 1)
	require.Equal(t, http.StatusOK, w.Code)

	code, _, data := parseResp(t, w)
	require.Equal(t, 0, code)
	assert.Equal(t, "paid", data["pay_status"])
	assert.Equal(t, "manual", data["pay_mode"])

	var order model.TenantMonthlyFeeOrder
	require.NoError(t, deps.DB.First(&order, 9105).Error)
	assert.Equal(t, "paid", order.PayStatus)
	assert.Equal(t, "manual", order.PayMode)
}
