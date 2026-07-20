// Package middleware v0.4.x Prometheus HTTP 指标中间件
//
// 严格遵循铁律 04/05/06：
//   04 - 无硬编码：路径规范化规则在常量中定义
//   05 - 配置走后端：通过 metrics 包共享 sys_config 开关
//   06 - 反幻觉：仅记录真实 HTTP 请求，不编造样本
package middleware

import (
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/your-org/keyauth-saas/apps/server/internal/metrics"
)

// ============== 配置常量 ==============

// 路径参数占位符（用于将 /api/v1/tenant/apps/123 → /api/v1/tenant/apps/:id）
// 铁律 04：路径规范化规则集中定义
var pathParamSegments = map[string]bool{
	"apps":         true,
	"cards":        true,
	"devices":      true,
	"orders":       true,
	"agents":       true,
	"versions":     true,
	"notices":      true,
	"invite_codes": true,
	"packages":     true,
	"tenants":      true,
	"users":        true,
	"logs":         true,
	"cloud_vars":   true,
	"recharge":     true,
	"withdrawals":  true,
	"metrics":      true,
	"alerts":       true,
	"v":            true,
}

// ============== PrometheusMiddleware ==============

// PrometheusMiddleware 采集 HTTP 请求指标
// 铁律 06：使用 time.Since 精确计时；异常不影响业务响应
func PrometheusMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 跳过 metrics 端点本身（避免自递归统计）
		if c.Request.URL.Path == "/metrics" {
			c.Next()
			return
		}

		start := time.Now()
		path := normalizePath(c.Request.URL.Path, c.Request.Method)
		method := c.Request.Method

		// 在飞请求 +1
		metrics.HTTPRequestsInFlight.Inc()
		defer metrics.HTTPRequestsInFlight.Dec()

		// 执行请求
		c.Next()

		// 记录耗时
		duration := time.Since(start).Seconds()
		status := strconv.Itoa(c.Writer.Status())

		metrics.HTTPRequestsTotal.WithLabelValues(method, path, status).Inc()
		metrics.HTTPRequestDurationSeconds.WithLabelValues(method, path).Observe(duration)
	}
}

// normalizePath 路径规范化：将数字 ID 替换为 :id，避免 label 爆炸
// 铁律 06：仅对已知资源段后的数字进行替换，保留 query/path 兼容性
func normalizePath(path, method string) string {
	if path == "" || path == "/" {
		return "/"
	}

	// 健康检查等固定路径直接返回
	if path == "/health" || path == "/metrics" || strings.HasPrefix(path, "/api/v1/public/") {
		return path
	}

	segments := strings.Split(strings.TrimPrefix(path, "/"), "/")
	for i, seg := range segments {
		// 跳过空段（首尾斜杠）
		if seg == "" {
			continue
		}
		// 数字段 → :id
		if isNumeric(seg) {
			segments[i] = ":id"
			continue
		}
		// UUID 段 → :id
		if isUUID(seg) {
			segments[i] = ":id"
			continue
		}
		// 已知资源段后的非数字段（如 /apps/123/cards 中的 cards）保留
		// 但 /api/v1/tenant/apps/<appkey> 这种字符串 ID → :id
		if i > 0 && pathParamSegments[segments[i-1]] && looksLikeID(seg) {
			segments[i] = ":id"
		}
	}
	return "/" + strings.Join(segments, "/")
}

// isNumeric 判断是否纯数字
func isNumeric(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// isUUID 判断是否 UUID（含或不含连字符）
func isUUID(s string) bool {
	if len(s) != 36 && len(s) != 32 {
		return false
	}
	for i, c := range s {
		if c == '-' {
			if i != 8 && i != 13 && i != 18 && i != 23 {
				return false
			}
			continue
		}
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}

// looksLikeID 判断是否像 ID（字母+数字混合且长度>=8，如 appkey/secret 等）
func looksLikeID(s string) bool {
	if len(s) < 8 {
		return false
	}
	hasDigit, hasLetter := false, false
	for _, c := range s {
		if c >= '0' && c <= '9' {
			hasDigit = true
		} else if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') {
			hasLetter = true
		} else if c != '_' && c != '-' {
			return false
		}
	}
	return hasDigit && hasLetter
}
