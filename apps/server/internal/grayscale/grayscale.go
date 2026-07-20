// Package grayscale 灰度发布核心逻辑（v0.4.0）
//
// 提供应用版本灰度匹配能力。所有可变参数（灰度开关 / 默认比例 / 哈希盐值）
// 通过 sys_config 注入，严格遵循铁律 05（配置后台化）。
//
// 设计要点：
//   - 灰度分桶基于客户端唯一标识（hwid/device_id/ip）+ hash_salt 计算
//     SHA-256 哈希取模 100，保证同一客户端在多次版本检查中得到稳定结果
//   - 三种发布策略：
//       full       全量发布（所有客户端可见）
//       grayscale  灰度发布（按比例 + 平台/地区/渠道规则过滤）
//       canary     金丝雀发布（与 grayscale 同逻辑，语义上用于内部测试）
//   - 规则匹配顺序：平台 → 渠道 → 地区 → 灰度比例
//   - 全局总开关 app.version.grayscale.enabled=false 时，所有 grayscale 策略降级为 full
package grayscale

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"strconv"
	"strings"

	"github.com/your-org/keyauth-saas/apps/server/internal/config"
	"github.com/your-org/keyauth-saas/apps/server/internal/model"
)

// 配置键常量（铁律 05：所有可调参数走 sys_config）
const (
	CfgKeyEnabled      = "app.version.grayscale.enabled"
	CfgKeyDefaultRate  = "app.version.grayscale.default_rate"
	CfgKeyHashSalt     = "app.version.grayscale.hash_salt"

	// 策略常量
	StrategyFull      = "full"
	StrategyGrayscale = "grayscale"
	StrategyCanary    = "canary"
)

// MatchRequest 灰度匹配请求
type MatchRequest struct {
	Version   *model.AppVersion // 候选版本
	ClientID  string            // 客户端唯一标识（hwid > device_id > client_ip）
	Platform  string            // windows/macos/linux/android/ios
	Channel   string            // stable/beta/dev（从请求或 app 默认渠道）
	Region    string            // 客户端地区（省/州代码）
}

// MatchResult 匹配结果
type MatchResult struct {
	Matched  bool    // 是否命中
	Reason   string  // 未命中原因（用于审计日志）
	Rate     float64 // 实际生效的灰度比例
	Bucket   int     // 客户端所在桶（0-99）
}

// Match 判断客户端是否命中指定版本的灰度规则
//
// 调用场景：ClientVersion handler 遍历候选版本时，对每个版本调用本函数
//
// 规则：
//   - release_strategy=full → 直接命中（全量发布）
//   - release_strategy=grayscale/canary 且全局开关 enabled=false → 直接命中（降级为全量）
//   - 平台限制非空且客户端平台不在列表 → 不命中
//   - 渠道限制非空且客户端渠道不在列表 → 不命中
//   - 地区限制非空且客户端地区不在列表 → 不命中
//   - 灰度比例 <= 0 → 不命中（无用户可见）
//   - 灰度比例 >= 100 → 命中（全量灰度）
//   - 否则计算哈希桶：bucket = sha256(salt + ":" + appID + ":" + clientID) 取前 4 字节小端 % 100
//     bucket < rate → 命中
func Match(ctx context.Context, cfgCache *config.ConfigCache, req MatchRequest) MatchResult {
	if req.Version == nil {
		return MatchResult{Matched: false, Reason: "version is nil"}
	}

	strategy := req.Version.ReleaseStrategy
	if strategy == "" {
		strategy = StrategyFull
	}

	// 1. 全量发布直接命中
	if strategy == StrategyFull {
		return MatchResult{Matched: true, Reason: "full strategy", Rate: 100}
	}

	// 2. 灰度策略但全局开关关闭 → 降级为全量
	enabled := cfgCache.GetBool(ctx, CfgKeyEnabled, true)
	if !enabled {
		return MatchResult{Matched: true, Reason: "grayscale disabled globally, fallback to full", Rate: 100}
	}

	// 3. 平台过滤
	if req.Version.GrayscalePlatforms != "" {
		allowed := ParseList(req.Version.GrayscalePlatforms)
		if req.Platform == "" || !contains(allowed, req.Platform) {
			return MatchResult{Matched: false, Reason: "platform not in grayscale list: " + req.Platform}
		}
	}

	// 4. 渠道过滤
	if req.Version.GrayscaleChannels != "" {
		allowed := ParseList(req.Version.GrayscaleChannels)
		clientChannel := req.Channel
		if clientChannel == "" {
			clientChannel = "stable" // 默认渠道
		}
		if !contains(allowed, clientChannel) {
			return MatchResult{Matched: false, Reason: "channel not in grayscale list: " + clientChannel}
		}
	}

	// 5. 地区过滤
	if req.Version.GrayscaleRegions != "" {
		allowed := ParseList(req.Version.GrayscaleRegions)
		if req.Region == "" || !contains(allowed, req.Region) {
			return MatchResult{Matched: false, Reason: "region not in grayscale list: " + req.Region}
		}
	}

	// 6. 灰度比例判断
	rate := req.Version.GrayscaleRate
	if rate <= 0 {
		return MatchResult{Matched: false, Reason: "grayscale rate <= 0"}
	}
	if rate >= 100 {
		return MatchResult{Matched: true, Reason: "grayscale rate >= 100", Rate: rate, Bucket: 0}
	}

	// 7. 计算哈希桶
	salt := cfgCache.GetString(ctx, CfgKeyHashSalt, "keyauth-grayscale-v040")
	clientID := req.ClientID
	if clientID == "" {
		clientID = "anonymous"
	}
	bucket := HashBucket(salt, req.Version.AppID, clientID)

	if bucket < int(rate) {
		return MatchResult{Matched: true, Reason: "bucket hit", Rate: rate, Bucket: bucket}
	}
	return MatchResult{Matched: false, Reason: "bucket miss", Rate: rate, Bucket: bucket}
}

// HashBucket 计算客户端在灰度分桶中的位置（0-99）
//
// 算法：SHA-256(salt + ":" + appID + ":" + clientID) 取前 4 字节小端 uint32，mod 100
//
// 保证：
//   - 同一 (salt, appID, clientID) 三元组恒定返回同一桶（客户端多次检查结果稳定）
//   - 不同 clientID 在 0-99 范围内近似均匀分布
//   - 修改 salt 会导致全量用户重新分桶（用于紧急回滚灰度）
func HashBucket(salt string, appID uint64, clientID string) int {
	raw := salt + ":" + strconv.FormatUint(appID, 10) + ":" + clientID
	sum := sha256.Sum256([]byte(raw))
	// 取前 4 字节小端 uint32
	v := binary.LittleEndian.Uint32(sum[:4])
	return int(v % 100)
}

// ParseList 解析逗号分隔的字符串列表（trim + 小写化 + 去空）
//
// 用例：
//   "windows,macOS,Linux" → ["windows", "macos", "linux"]
//   "" → []
//   " , " → []
func ParseList(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(strings.ToLower(p))
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// contains 判断切片是否包含字符串（大小写敏感，调用方需先统一大小写）
func contains(s []string, v string) bool {
	v = strings.ToLower(strings.TrimSpace(v))
	for _, item := range s {
		if item == v {
			return true
		}
	}
	return false
}

// DefaultRate 读取 sys_config 中的默认灰度比例（新建版本时使用）
func DefaultRate(ctx context.Context, cfgCache *config.ConfigCache) float64 {
	return cfgCache.GetFloat64(ctx, CfgKeyDefaultRate, 10.00)
}

// IsEnabled 读取灰度发布全局开关
func IsEnabled(ctx context.Context, cfgCache *config.ConfigCache) bool {
	return cfgCache.GetBool(ctx, CfgKeyEnabled, true)
}
