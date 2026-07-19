// 限流中间件（基于 Redis 滑动窗口）
// 配置项走 sys_config（铁律 05）
package middleware

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

// 限流相关配置键
const (
	configKeyRateLimitGlobal      = "security.rate.limit_global"      // 全局限流（每秒 QPS）
	configKeyRateLimitSensitive   = "security.rate.limit_sensitive"   // 敏感接口每分钟
	configKeyBanThreshold         = "security.ban.threshold"          // 卡密错误 N 次封 IP
	configKeyBanDuration          = "security.ban.duration_seconds"   // 封禁时长（秒）
)

// RateLimitByIP 按 IP 限流
// limitKey 标识限流策略（如 verify/login/pay）
// cfgReader 用于读取 sys_config 中的限流阈值
func RateLimitByIP(rdb *redis.Client, cfgReader ConfigReader, limitKey string) gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := c.ClientIP()
		ctx := c.Request.Context()

		// 读取限流配置
		limit := cfgReader.GetInt(ctx, configKeyRateLimitGlobal, 100)
		if limitKey == "sensitive" {
			limit = cfgReader.GetInt(ctx, configKeyRateLimitSensitive, 10)
		}

		// 滑动窗口：1 秒内同 IP 最多 N 次
		key := fmt.Sprintf("rate:%s:%s", limitKey, ip)
		count, err := rdb.Incr(ctx, key).Result()
		if err != nil {
			// Redis 故障时放行（fail-open，避免 Redis 故障导致服务不可用）
			c.Next()
			return
		}
		if count == 1 {
			// 首次访问设置过期时间
			window := time.Second
			if limitKey == "sensitive" {
				window = time.Minute
			}
			_ = rdb.Expire(ctx, key, window).Err()
		}
		if count > int64(limit) {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"code":    1005,
				"message": "请求过于频繁，请稍后再试",
			})
			return
		}

		c.Next()
	}
}

// IPBlacklist IP 黑名单检查
func IPBlacklist(rdb *redis.Client, db interface{}) gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := c.ClientIP()
		ctx := c.Request.Context()

		// 1. 查 Redis 缓存
		key := "ip:blacklist:" + ip
		exists, err := rdb.Exists(ctx, key).Result()
		if err == nil && exists > 0 {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"code":    1003,
				"message": "IP 已被加入黑名单",
			})
			return
		}

		// 2. 注：MySQL 持久化黑名单查询应由 Service 层提供（此处省略）
		// 需验证：Redis 黑名单与 MySQL 同步策略

		c.Next()
	}
}

// RecordCardFailure 记录卡密验证失败次数
// 达到阈值自动封禁 IP
func RecordCardFailure(ctx context.Context, rdb *redis.Client, cfgReader ConfigReader, ip string) error {
	key := "fail:card:" + ip
	count, err := rdb.Incr(ctx, key).Result()
	if err != nil {
		return err
	}
	if count == 1 {
		// 1 小时窗口
		_ = rdb.Expire(ctx, key, time.Hour).Err()
	}

	threshold := cfgReader.GetInt(ctx, configKeyBanThreshold, 5)
	if int(count) >= threshold {
		// 自动封禁
		banDuration := cfgReader.GetInt(ctx, configKeyBanDuration, 3600)
		banKey := "ip:blacklist:" + ip
		_ = rdb.Set(ctx, banKey, "auto", time.Duration(banDuration)*time.Second).Err()
		// 同时写入 MySQL 持久化（需 Service 层调用）
	}
	return nil
}

// ClearCardFailure 验证成功时清除失败计数
func ClearCardFailure(ctx context.Context, rdb *redis.Client, ip string) {
	_ = rdb.Del(ctx, "fail:card:"+ip).Err()
}
