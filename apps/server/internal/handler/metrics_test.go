// Package handler v0.4.x Prometheus /metrics 端点测试
// 严格遵循铁律 06：测试覆盖开关 / BasicAuth / 路径 / Collector 注册全场景
package handler

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/your-org/keyauth-saas/apps/server/internal/config"
	"github.com/your-org/keyauth-saas/apps/server/internal/metrics"
	"github.com/your-org/keyauth-saas/apps/server/internal/model"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// setupTestCache 测试用 ConfigCache（带内存 SQLite db，避免 getRaw 查 DB 时 panic）
func setupTestCache(t *testing.T) (*config.ConfigCache, *gorm.DB, func()) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared&_pragma=foreign_keys(1)"), &gorm.Config{})
	require.NoError(t, err)
	_ = db.Migrator().DropTable(&model.SysConfig{})
	require.NoError(t, db.AutoMigrate(&model.SysConfig{}))
	cache := config.NewConfigCache(db, rdb)
	return cache, db, func() {
		_ = rdb.Close()
		mr.Close()
	}
}

// ============== 1. MetricsPath 测试 ==============

func TestMetricsPath_Default(t *testing.T) {
	cache, _, cleanup := setupTestCache(t)
	defer cleanup()

	path := MetricsPath(cache)
	assert.Equal(t, "/metrics", path)
}

func TestMetricsPath_NilCache(t *testing.T) {
	// nil cache 应返回默认 /metrics，不 panic
	path := MetricsPath(nil)
	assert.Equal(t, "/metrics", path)
}

func TestMetricsPath_CustomPath(t *testing.T) {
	cache, _, cleanup := setupTestCache(t)
	defer cleanup()

	// 设置自定义路径
	require.NoError(t, cache.Set(context.Background(), metrics.CfgKeyPromPath, "/custom/metrics", "Prometheus 端点路径", "monitor", "测试"))

	path := MetricsPath(cache)
	assert.Equal(t, "/custom/metrics", path)
}

func TestMetricsPath_InvalidPath(t *testing.T) {
	cache, _, cleanup := setupTestCache(t)
	defer cleanup()

	// 不以 / 开头的路径应回退到 /metrics
	require.NoError(t, cache.Set(context.Background(), metrics.CfgKeyPromPath, "invalid", "测试", "monitor", "测试"))
	assert.Equal(t, "/metrics", MetricsPath(cache))

	// 空路径应回退到 /metrics
	require.NoError(t, cache.Set(context.Background(), metrics.CfgKeyPromPath, "", "测试", "monitor", "测试"))
	assert.Equal(t, "/metrics", MetricsPath(cache))
}

// ============== 2. MetricsHandler 测试 ==============

func TestMetricsHandler_Enabled(t *testing.T) {
	cache, _, cleanup := setupTestCache(t)
	defer cleanup()
	// 默认 enabled=true（fallback）

	r := gin.New()
	r.GET("/metrics", MetricsHandler(cache))

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/metrics", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	// 应该返回 Prometheus exposition 格式
	body := w.Body.String()
	assert.True(t,
		strings.Contains(body, "# HELP") || strings.Contains(body, "# TYPE"),
		"响应应包含 Prometheus 指标格式")
}

func TestMetricsHandler_Disabled(t *testing.T) {
	cache, _, cleanup := setupTestCache(t)
	defer cleanup()
	// 显式禁用
	require.NoError(t, cache.Set(context.Background(), metrics.CfgKeyPromEnabled, "0", "Prometheus 开关", "monitor", "测试"))

	r := gin.New()
	r.GET("/metrics", MetricsHandler(cache))

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/metrics", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestMetricsHandler_NoBasicAuthWhenUnconfigured(t *testing.T) {
	cache, _, cleanup := setupTestCache(t)
	defer cleanup()
	// 未配置 BasicAuth 凭据 → 应无需鉴权即可访问

	r := gin.New()
	r.GET("/metrics", MetricsHandler(cache))

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/metrics", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestMetricsHandler_BasicAuth_Required(t *testing.T) {
	cache, _, cleanup := setupTestCache(t)
	defer cleanup()
	require.NoError(t, cache.Set(context.Background(), metrics.CfgKeyPromBasicAuthUser, "promuser", "BasicAuth 用户", "monitor", "测试"))
	require.NoError(t, cache.Set(context.Background(), metrics.CfgKeyPromBasicAuthPass, "prompass123", "BasicAuth 密码", "monitor", "测试"))

	r := gin.New()
	r.GET("/metrics", MetricsHandler(cache))

	// 1. 无 Authorization 头 → 401
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/metrics", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Contains(t, w.Header().Get("WWW-Authenticate"), "Basic")

	// 2. 错误凭据 → 401
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/metrics", nil)
	req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte("wrong:credentials")))
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)

	// 3. 正确凭据 → 200
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/metrics", nil)
	req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte("promuser:prompass123")))
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestMetricsHandler_BasicAuth_OnlyUserConfigured(t *testing.T) {
	cache, _, cleanup := setupTestCache(t)
	defer cleanup()
	// 只配置了 user，未配置 pass → 应视为不启用 BasicAuth
	require.NoError(t, cache.Set(context.Background(), metrics.CfgKeyPromBasicAuthUser, "promuser", "BasicAuth 用户", "monitor", "测试"))
	// pass 留空

	r := gin.New()
	r.GET("/metrics", MetricsHandler(cache))

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/metrics", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code, "仅配置 user 未配置 pass 时应不启用 BasicAuth")
}

// ============== 3. RegisterSystemCollector 测试 ==============

func TestRegisterSystemCollector_Idempotent(t *testing.T) {
	// 重复注册不应 panic
	// 注意：不传真实 db/cache，因为这里只验证幂等性
	assert.NotPanics(t, func() {
		RegisterSystemCollector(nil, nil)
		RegisterSystemCollector(nil, nil)
		RegisterSystemCollector(nil, nil)
	})
}
