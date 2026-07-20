// Package handler v0.4.x Prometheus /metrics 端点
//
// 严格遵循铁律 04/05/06：
//   04 - 无硬编码：端点路径 / 开关 / BasicAuth 凭据全部从 sys_config 读取
//   05 - 配置走后端：monitor.prometheus.enabled / .path / .basic_auth_user / .basic_auth_pass
//   06 - 反幻觉：使用 promhttp.Handler() 输出真实指标，不编造样本
package handler

import (
	"context"
	"crypto/subtle"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"gorm.io/gorm"

	"github.com/your-org/keyauth-saas/apps/server/internal/config"
	"github.com/your-org/keyauth-saas/apps/server/internal/metrics"
	"github.com/your-org/keyauth-saas/apps/server/internal/monitor"
)

// ============== 全局注册（仅一次） ==============

// prometheusRegistered 防止重复注册 collector（测试场景多次调用 Register）
var prometheusRegistered bool

// RegisterSystemCollector 注册自定义 SystemCollector 到默认 registry
// 铁律 06：仅注册一次；重复注册时 prometheus.Register 返回 AlreadyRegisteredError 但不 panic
func RegisterSystemCollector(db *gorm.DB, cache *config.ConfigCache) {
	if prometheusRegistered {
		return
	}
	manager := monitor.NewManager(db, cache)
	collector := metrics.NewSystemCollector(manager, cache, db)
	// 用 Register 而非 MustRegister，避免重复注册时 panic
	if err := prometheus.Register(collector); err != nil {
		// 已注册是允许的（测试场景 / 多次调用）
		_ = err
	}
	prometheusRegistered = true
}

// ============== /metrics 端点 ==============

// MetricsHandler Prometheus /metrics 端点
// 铁律 05：开关 / BasicAuth 凭据均从 sys_config 实时读取
// 铁律 06：常量时间比较防时序攻击；未配置 user/pass 时无鉴权（内网部署场景）
func MetricsHandler(cfgCache *config.ConfigCache) gin.HandlerFunc {
	handler := promhttp.Handler()
	return func(c *gin.Context) {
		// 1. 开关校验
		if !cfgCache.GetBool(c.Request.Context(), metrics.CfgKeyPromEnabled, true) {
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{
				"code":    503,
				"message": "prometheus metrics disabled",
			})
			return
		}

		// 2. BasicAuth 校验（若配置了 user+pass）
		user := cfgCache.GetString(c.Request.Context(), metrics.CfgKeyPromBasicAuthUser, "")
		pass := cfgCache.GetString(c.Request.Context(), metrics.CfgKeyPromBasicAuthPass, "")
		if user != "" && pass != "" {
			reqUser, reqPass, ok := c.Request.BasicAuth()
			if !ok ||
				subtle.ConstantTimeCompare([]byte(reqUser), []byte(user)) != 1 ||
				subtle.ConstantTimeCompare([]byte(reqPass), []byte(pass)) != 1 {
				c.Header("WWW-Authenticate", `Basic realm="metrics"`)
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
					"code":    401,
					"message": "unauthorized",
				})
				return
			}
		}

		// 3. 交给 promhttp 处理
		handler.ServeHTTP(c.Writer, c.Request)
	}
}

// MetricsPath 从 sys_config 读取自定义路径（默认 /metrics）
// 用于 router 注册时动态决定路径
// 铁律 06：使用 context.Background() 兜底（router 注册时无请求 context）
func MetricsPath(cfgCache *config.ConfigCache) string {
	if cfgCache == nil {
		return "/metrics"
	}
	path := strings.TrimSpace(cfgCache.GetString(context.Background(), metrics.CfgKeyPromPath, "/metrics"))
	if path == "" || !strings.HasPrefix(path, "/") {
		path = "/metrics"
	}
	return path
}
