// Package middleware v0.4.x Prometheus HTTP 指标中间件测试
// 严格遵循铁律 06：测试覆盖路径规范化、中间件全流程、边缘场景
package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// ============== 1. 路径规范化测试 ==============

func TestNormalizePath_StaticPaths(t *testing.T) {
	cases := []struct {
		path     string
		method   string
		expected string
	}{
		{"/", "GET", "/"},
		{"", "GET", "/"},
		{"/health", "GET", "/health"},
		{"/metrics", "GET", "/metrics"},
		{"/api/v1/public/auth/admin/login", "POST", "/api/v1/public/auth/admin/login"},
		{"/api/v1/public/notices/123", "GET", "/api/v1/public/notices/123"}, // public 路径不规范化
	}
	for _, c := range cases {
		got := normalizePath(c.path, c.method)
		assert.Equal(t, c.expected, got, "normalizePath(%q) should be %q", c.path, c.expected)
	}
}

func TestNormalizePath_NumericID(t *testing.T) {
	cases := []struct {
		path     string
		expected string
	}{
		{"/api/v1/tenant/apps/123", "/api/v1/tenant/apps/:id"},
		{"/api/v1/admin/tenants/456", "/api/v1/admin/tenants/:id"},
		{"/api/v1/tenant/apps/123/cards/456", "/api/v1/tenant/apps/:id/cards/:id"},
		// ORD20260720001 是字母+数字混合 >=8 字符，且 orders 是已知资源段，会被替换为 :id
		{"/api/v1/h5/orders/ORD20260720001", "/api/v1/h5/orders/:id"},
		// 短字符串（< 8 字符）不被替换
		{"/api/v1/h5/orders/ORD123", "/api/v1/h5/orders/ORD123"},
	}
	for _, c := range cases {
		got := normalizePath(c.path, "GET")
		assert.Equal(t, c.expected, got, "normalizePath(%q) should be %q", c.path, c.expected)
	}
}

func TestNormalizePath_UUID(t *testing.T) {
	cases := []struct {
		path     string
		expected string
	}{
		{"/api/v1/tenant/apps/550e8400-e29b-41d4-a716-446655440000", "/api/v1/tenant/apps/:id"},
		{"/api/v1/admin/logs/abc123de-4567-89ab-cdef-0123456789ab", "/api/v1/admin/logs/:id"},
	}
	for _, c := range cases {
		got := normalizePath(c.path, "GET")
		assert.Equal(t, c.expected, got, "normalizePath(%q) should be %q", c.path, c.expected)
	}
}

func TestNormalizePath_StringIDAfterKnownSegment(t *testing.T) {
	// /apps/<appkey> 应该被替换（appkey 通常是字母+数字混合 >=8 字符）
	cases := []struct {
		path     string
		expected string
	}{
		{"/api/v1/tenant/apps/ak_abc12345def", "/api/v1/tenant/apps/:id"},   // looksLikeID
		{"/api/v1/client/apps/abcdefgh12345678", "/api/v1/client/apps/:id"}, // 8+ 字符 字母+数字
		// 不足 8 字符不替换
		{"/api/v1/tenant/apps/abc", "/api/v1/tenant/apps/abc"},
		// 不是已知资源段后的不替换
		{"/api/v1/unknown/abc12345def", "/api/v1/unknown/abc12345def"},
	}
	for _, c := range cases {
		got := normalizePath(c.path, "GET")
		assert.Equal(t, c.expected, got, "normalizePath(%q) should be %q", c.path, c.expected)
	}
}

// ============== 2. 工具函数测试 ==============

func TestIsNumeric(t *testing.T) {
	assert.True(t, isNumeric("0"))
	assert.True(t, isNumeric("123"))
	assert.True(t, isNumeric("999999"))
	assert.False(t, isNumeric(""))
	assert.False(t, isNumeric("12a"))
	assert.False(t, isNumeric("a12"))
	assert.False(t, isNumeric("12.34"))
}

func TestIsUUID(t *testing.T) {
	// 标准 UUID（36 字符，含 4 个连字符）
	assert.True(t, isUUID("550e8400-e29b-41d4-a716-446655440000"))
	// 紧凑 UUID（32 字符，无连字符）
	assert.True(t, isUUID("550e8400e29b41d4a716446655440000"))
	// 非法
	assert.False(t, isUUID(""))
	assert.False(t, isUUID("abc"))
	assert.False(t, isUUID("550e8400-e29b-41d4-a716"))     // 过短
	assert.False(t, isUUID("550e8400-e29b-41d4-a716-44665544000g")) // 含非法字符
	assert.False(t, isUUID("550e8400-e29b-41d4-a716-446655440000-extra")) // 过长
}

func TestLooksLikeID(t *testing.T) {
	assert.True(t, looksLikeID("ak_abc12345"))      // 字母+数字 >=8 字符
	assert.True(t, looksLikeID("abcd1234"))         // 8 字符 字母+数字
	assert.True(t, looksLikeID("sk-abc123def456"))  // 含 - 字母+数字
	assert.False(t, looksLikeID(""))                // 空
	assert.False(t, looksLikeID("abc"))             // 过短
	assert.False(t, looksLikeID("12345678"))        // 纯数字（不满足字母+数字）
	assert.False(t, looksLikeID("abcdefgh"))        // 纯字母（不满足字母+数字）
	assert.False(t, looksLikeID("abc!@#$%^"))       // 含非法字符
}

// ============== 3. 中间件集成测试 ==============

func TestPrometheusMiddleware_BasicRequest(t *testing.T) {
	r := gin.New()
	r.Use(PrometheusMiddleware())
	r.GET("/api/v1/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/test", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	// 中间件不应影响业务响应
	assert.Contains(t, w.Body.String(), "ok")
}

func TestPrometheusMiddleware_SkipsMetricsEndpoint(t *testing.T) {
	r := gin.New()
	r.Use(PrometheusMiddleware())
	r.GET("/metrics", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"metrics": "ok"})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/metrics", nil)
	r.ServeHTTP(w, req)

	// /metrics 端点应跳过指标采集（避免自递归）
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestPrometheusMiddleware_DifferentStatusCodes(t *testing.T) {
	r := gin.New()
	r.Use(PrometheusMiddleware())
	r.GET("/ok", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{}) })
	r.GET("/notfound", func(c *gin.Context) { c.JSON(http.StatusNotFound, gin.H{}) })
	r.GET("/error", func(c *gin.Context) { c.JSON(http.StatusInternalServerError, gin.H{}) })

	cases := []struct {
		path         string
		expectedCode int
	}{
		{"/ok", 200},
		{"/notfound", 404},
		{"/error", 500},
	}
	for _, c := range cases {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", c.path, nil)
		r.ServeHTTP(w, req)
		assert.Equal(t, c.expectedCode, w.Code)
	}
}

// ============== 4. 路径参数表完整性测试 ==============

func TestPathParamSegments_CoversAllResources(t *testing.T) {
	// 铁律 06：覆盖所有需要路径参数化的资源段
	requiredSegments := []string{
		"apps", "cards", "devices", "orders", "agents",
		"versions", "notices", "invite_codes", "packages",
		"tenants", "users", "logs", "cloud_vars",
		"recharge", "withdrawals", "metrics", "alerts",
	}
	for _, seg := range requiredSegments {
		assert.True(t, pathParamSegments[seg], "缺少资源段: %s", seg)
	}
}
