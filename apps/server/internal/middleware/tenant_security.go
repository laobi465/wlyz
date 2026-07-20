// 开发者安全配置中间件（v0.4.x D-15）
// 在 client 验证 API 链路中校验：
//   1. 租户 IP 黑名单（tenant_security_config.ip_blacklist JSON 数组）
//   2. 客户端验证 API 限速（verify_rate_limit_per_min，每分钟，0=不限）
//   3. 客户端登录 API 限速（login_rate_limit_per_min，每分钟，0=不限）
//
// 严格遵循铁律 04/05/06：
//   04 - 无硬编码：黑名单与频率阈值均存表
//   05 - 配置走后端：tenant_security_config 表 + sys_config 配合
//   06 - 不确定处标注「待核实」
package middleware

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"github.com/your-org/keyauth-saas/apps/server/internal/model"
)

// 限流键前缀（铁律 04：常量声明，避免散落）
const (
	tenantRateLimitKeyPrefix = "rate:tenant:"
)

// TenantSecurityMiddleware 开发者安全配置中间件
// 必须挂在 SignatureAuth 之后（依赖 c.MustGet("app") 注入的 *model.App）
// limitKey 决定走 verify / login 哪一档限速（与现有 RateLimitByIP 风格一致）
func TenantSecurityMiddleware(db *gorm.DB, rdb *redis.Client, limitKey string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 1. 取 app + tenant_id
		appVal, exists := c.Get("app")
		if !exists {
			// SignatureAuth 未通过：放行（让签名中间件自己拒绝）
			c.Next()
			return
		}
		app, ok := appVal.(*model.App)
		if !ok || app == nil {
			c.Next()
			return
		}
		tenantID := app.TenantID
		if tenantID == 0 {
			c.Next()
			return
		}

		// 2. 查租户安全配置（无记录则放行，向后兼容）
		var sec model.TenantSecurityConfig
		if err := db.Where("tenant_id = ?", tenantID).First(&sec).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				c.Next()
				return
			}
			// DB 故障：fail-open，避免 DB 抖动导致全站不可用
			c.Next()
			return
		}

		// 3. IP 黑名单校验
		ip := RealIP(c)
		if ip != "" && sec.IPBlacklist != "" && sec.IPBlacklist != "[]" {
			if isIPInTenantBlacklist(ip, sec.IPBlacklist) {
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
					"code":    1003,
					"message": "请求 IP 已被开发者加入黑名单",
				})
				return
			}
		}

		// 4. 频率限制（按 limitKey 选 verify/login 阈值；其他场景不限速）
		var limitPerMin int
		switch limitKey {
		case "verify":
			limitPerMin = sec.VerifyRateLimitPerMin
		case "login":
			limitPerMin = sec.LoginRateLimitPerMin
		default:
			// 不识别的 limitKey：放行（保持向后兼容）
			c.Next()
			return
		}
		if limitPerMin <= 0 {
			c.Next()
			return
		}

		// 5. Redis 滑动窗口限速：1 分钟内同 IP 同 tenant 最多 N 次
		if rdb == nil {
			c.Next()
			return
		}
		ctx := c.Request.Context()
		key := fmt.Sprintf("%s%s:%d:%s", tenantRateLimitKeyPrefix, limitKey, tenantID, ip)
		count, err := rdb.Incr(ctx, key).Result()
		if err != nil {
			// Redis 故障：fail-open（与 RateLimitByIP 一致）
			c.Next()
			return
		}
		if count == 1 {
			_ = rdb.Expire(ctx, key, time.Minute).Err()
		}
		if count > int64(limitPerMin) {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"code":    1005,
				"message": "请求过于频繁，请稍后再试",
			})
			return
		}

		c.Next()
	}
}

// isIPInTenantBlacklist 校验 IP 是否在租户黑名单中
// ipBlacklistJSON 形如 ["1.2.3.4","10.0.0.0/8"]
// 支持单 IP 与 CIDR 两种格式，任一命中即返回 true
func isIPInTenantBlacklist(ipStr, ipBlacklistJSON string) bool {
	var list []string
	if err := json.Unmarshal([]byte(ipBlacklistJSON), &list); err != nil {
		// JSON 解析失败：fail-open（避免错误配置误伤所有请求）
		return false
	}
	parsedIP := net.ParseIP(ipStr)
	if parsedIP == nil {
		return false
	}
	for _, entry := range list {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		// 单 IP 直接比较
		if !strings.Contains(entry, "/") {
			if entryIP := net.ParseIP(entry); entryIP != nil && entryIP.Equal(parsedIP) {
				return true
			}
			continue
		}
		// CIDR 范围比较
		_, network, err := net.ParseCIDR(entry)
		if err != nil {
			continue
		}
		if network.Contains(parsedIP) {
			return true
		}
	}
	return false
}

// 编译期检查：保留 context 导入（与 cloudflare.go 风格一致）
var _ = context.Background
