// Cloudflare WAF 真实 IP 中间件（v0.4.0 第十五项迁移）
// 严格遵循铁律 04：所有配置键名以常量声明
// 严格遵循铁律 05：所有开关/头名/CIDR 列表走 sys_config 后台可视化编辑
//
// 工作流程：
//  1. 若 cloudflare.enabled=0，直接 c.Next()（默认行为）
//  2. 若 enabled=1，从 cloudflare.real_ip_header 头取真实 IP
//  3. 若 cloudflare.trusted_cidrs 非空，校验请求 RemoteAddr 是否在受信 CIDR 内
//  4. 将真实 IP 注入 c.Set("real_ip", ip) 与 c.Set("ip_country", country)
//  5. 后续中间件/handler 通过 RealIP(c) 工具函数读取
package middleware

import (
	"context"
	"net"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// Cloudflare 配置键常量（铁律 04：与 internal/risk 包常量保持一致）
const (
	configKeyCFEnabled         = "cloudflare.enabled"
	configKeyCFRealIPHeader    = "cloudflare.real_ip_header"
	configKeyCFIPCountryHeader = "cloudflare.ip_country_header"
	configKeyCFTrustedCIDRs    = "cloudflare.trusted_cidrs"

	// ContextKeyRealIP gin.Context 中真实 IP 的 key
	ContextKeyRealIP    = "real_ip"
	ContextKeyIPCountry = "ip_country"
)

// CloudflareRealIP Cloudflare 真实 IP 中间件
// 必须在 IPBlacklist / RateLimitByIP / RiskEngine 之前注册
func CloudflareRealIP(cfgReader ConfigReader) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		enabled := cfgReader.GetBool(ctx, configKeyCFEnabled, false)
		if !enabled {
			// 未启用 CF：使用 c.ClientIP() 作为 real_ip（保持兼容）
			c.Set(ContextKeyRealIP, c.ClientIP())
			c.Next()
			return
		}

		headerName := cfgReader.GetString(ctx, configKeyCFRealIPHeader, "CF-Connecting-IP")

		// 校验来源 CIDR（生产环境强烈建议配置 Cloudflare CIDR 列表）
		// P1-04 修复：trustedCIDRs 为空时不得信任 CF-Connecting-IP 头（攻击者可伪造），
		//           回退到 c.ClientIP()；仅当 remote addr 命中受信 CIDR 时才读取 CF 头。
		trustedCIDRs := cfgReader.GetString(ctx, configKeyCFTrustedCIDRs, "")
		realIP := ""
		if trustedCIDRs != "" {
			remoteIP := hostFromAddr(c.Request.RemoteAddr)
			if remoteIP != "" && ipInCIDRList(remoteIP, trustedCIDRs) {
				// 来源受信：读取 CF 头作为真实 IP
				realIP = strings.TrimSpace(c.GetHeader(headerName))
			}
			// 否则来源不受信，realIP 留空，下面回退到 c.ClientIP()
		}
		// trustedCIDRs 为空：realIP 留空，下面回退到 c.ClientIP()

		if realIP == "" {
			realIP = c.ClientIP()
		}

		c.Set(ContextKeyRealIP, realIP)

		// 同时提取国家代码（CF-IPCountry 由 Cloudflare 注入，ISO 3166-1 alpha-2）
		countryHeader := cfgReader.GetString(ctx, configKeyCFIPCountryHeader, "CF-IPCountry")
		if country := strings.TrimSpace(c.GetHeader(countryHeader)); country != "" {
			c.Set(ContextKeyIPCountry, country)
		}

		c.Next()
	}
}

// RealIP 从 gin.Context 中读取真实 IP
// 优先取 CloudflareRealIP 中间件注入的 real_ip；未注入则回退到 c.ClientIP()
// 所有需要 IP 的中间件/handler 应统一使用此函数（而非直接 c.ClientIP()）
func RealIP(c *gin.Context) string {
	if v, ok := c.Get(ContextKeyRealIP); ok {
		if ip, ok := v.(string); ok && ip != "" {
			return ip
		}
	}
	return c.ClientIP()
}

// IPCountry 从 gin.Context 中读取 IP 国家代码（仅 Cloudflare 启用时有效）
func IPCountry(c *gin.Context) string {
	if v, ok := c.Get(ContextKeyIPCountry); ok {
		if country, ok := v.(string); ok {
			return country
		}
	}
	return ""
}

// hostFromAddr 从 "host:port" 提取 host
func hostFromAddr(addr string) string {
	if addr == "" {
		return ""
	}
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return addr
	}
	return host
}

// ipInCIDRList 校验 ip 是否在 CIDR 列表（逗号分隔）内
// 任一 CIDR 命中即返回 true；解析失败的 CIDR 跳过
func ipInCIDRList(ipStr, cidrList string) bool {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return false
	}

	for _, cidr := range strings.Split(cidrList, ",") {
		cidr = strings.TrimSpace(cidr)
		if cidr == "" {
			continue
		}
		// 兼容不带前缀的单 IP（视为 /32 或 /128）
		if !strings.Contains(cidr, "/") {
			if cidrIP := net.ParseIP(cidr); cidrIP != nil && cidrIP.Equal(ip) {
				return true
			}
			continue
		}
		_, network, err := net.ParseCIDR(cidr)
		if err != nil {
			continue
		}
		if network.Contains(ip) {
			return true
		}
	}
	return false
}

// 接口编译期检查：确保 ConfigReader 接口包含 GetBool/GetString
var _ = func(_ ConfigReader) {}

// 上下文编译期检查：确保 context 可用
var _ = context.Background

// 编译期检查：确保 net/http 可用（保留导入）
var _ = http.StatusOK
