// 登录失败计数器 + 账号锁定逻辑
// 用 Redis 滑动窗口记录失败次数，达到阈值后锁定账号一段时间
// 严格遵循铁律 05：阈值/锁定时长/窗口期全部从 sys_config 读取
package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// LockStatus 账号锁定状态
type LockStatus struct {
	Locked      bool          `json:"locked"`
	FailedCount int           `json:"failed_count"`
	MaxAttempts int           `json:"max_attempts"`    // 最大失败次数阈值
	LockTTL     time.Duration `json:"lock_ttl"`        // 锁定剩余时间
	WindowTTL   time.Duration `json:"window_ttl"`      // 统计窗口剩余时间
}

// Redis Key 规范：
// 失败计数：auth:fail:{role}:{identifier}  -> 计数值（带窗口 TTL）
// 锁定标记：auth:lock:{role}:{identifier}  -> "1"（带锁定 TTL）

// failKey 失败计数 Redis Key
func failKey(role, identifier string) string {
	return fmt.Sprintf("auth:fail:%s:%s", role, identifier)
}

// lockKey 锁定标记 Redis Key
func lockKey(role, identifier string) string {
	return fmt.Sprintf("auth:lock:%s:%s", role, identifier)
}

// RecordLoginFailure 记录一次登录失败
// 参数：
//   - rdb: Redis 客户端
//   - role: 角色（admin/tenant/agent）
//   - identifier: 账号唯一标识（username 或 email）
//   - maxAttempts: 最大失败次数阈值（从 sys_config 读取）
//   - windowSeconds: 失败计数窗口（秒，从 sys_config 读取）
//   - lockSeconds: 锁定时长（秒，从 sys_config 读取）
//
// 返回当前累计失败次数
func RecordLoginFailure(
	ctx context.Context,
	rdb *redis.Client,
	role, identifier string,
	maxAttempts int,
	windowSeconds, lockSeconds int,
) (int, error) {
	if rdb == nil {
		return 0, fmt.Errorf("redis 未初始化")
	}
	if maxAttempts <= 0 || windowSeconds <= 0 || lockSeconds <= 0 {
		return 0, fmt.Errorf("参数错误: maxAttempts=%d windowSeconds=%d lockSeconds=%d",
			maxAttempts, windowSeconds, lockSeconds)
	}

	fk := failKey(role, identifier)
	// INCR 后如果是第一次失败，设置窗口 TTL
	count, err := rdb.Incr(ctx, fk).Result()
	if err != nil {
		return 0, fmt.Errorf("记录登录失败次数失败: %w", err)
	}
	if count == 1 {
		_ = rdb.Expire(ctx, fk, time.Duration(windowSeconds)*time.Second).Err()
	}

	// 达到阈值则锁定
	if int(count) >= maxAttempts {
		lk := lockKey(role, identifier)
		if err := rdb.Set(ctx, lk, "1", time.Duration(lockSeconds)*time.Second).Err(); err != nil {
			return int(count), fmt.Errorf("锁定账号失败: %w", err)
		}
		// 锁定后清除计数（避免锁定期内继续累加）
		_ = rdb.Del(ctx, fk).Err()
	}
	return int(count), nil
}

// IsAccountLocked 检查账号是否被锁定
// 返回：是否锁定 + 剩余锁定时间
func IsAccountLocked(ctx context.Context, rdb *redis.Client, role, identifier string) (bool, time.Duration, error) {
	if rdb == nil {
		return false, 0, fmt.Errorf("redis 未初始化")
	}
	lk := lockKey(role, identifier)
	ttl, err := rdb.TTL(ctx, lk).Result()
	if err != nil {
		return false, 0, fmt.Errorf("检查锁定状态失败: %w", err)
	}
	if ttl > 0 {
		return true, ttl, nil
	}
	return false, 0, nil
}

// GetLockStatus 获取账号锁定详情（前端用于提示「X 分钟后重试」）
func GetLockStatus(
	ctx context.Context,
	rdb *redis.Client,
	role, identifier string,
	maxAttempts int,
) (*LockStatus, error) {
	if rdb == nil {
		return nil, fmt.Errorf("redis 未初始化")
	}

	// 1. 检查锁定
	lk := lockKey(role, identifier)
	lockTTL, err := rdb.TTL(ctx, lk).Result()
	if err != nil {
		return nil, err
	}
	if lockTTL > 0 {
		return &LockStatus{
			Locked:      true,
			FailedCount: maxAttempts,
			MaxAttempts: maxAttempts,
			LockTTL:     lockTTL,
		}, nil
	}

	// 2. 检查失败计数
	fk := failKey(role, identifier)
	count, _ := rdb.Get(ctx, fk).Int()
	windowTTL, _ := rdb.TTL(ctx, fk).Result()

	return &LockStatus{
		Locked:      false,
		FailedCount: count,
		MaxAttempts: maxAttempts,
		WindowTTL:   windowTTL,
	}, nil
}

// ClearLoginFailure 登录成功后清除失败计数
// 注意：不会清除锁定标记（一旦锁定必须等 TTL 到期，防止暴力重试绕过）
func ClearLoginFailure(ctx context.Context, rdb *redis.Client, role, identifier string) error {
	if rdb == nil {
		return nil
	}
	return rdb.Del(ctx, failKey(role, identifier)).Err()
}

// FormatLockRemaining 格式化锁定剩余时间为人类可读字符串
// 例如：90s → "1分30秒"，3700s → "1小时1分钟"
func FormatLockRemaining(d time.Duration) string {
	if d <= 0 {
		return "0秒"
	}
	minutes := int(d.Minutes())
	seconds := int(d.Seconds()) % 60
	if minutes == 0 {
		return fmt.Sprintf("%d秒", seconds)
	}
	if minutes < 60 {
		return fmt.Sprintf("%d分%d秒", minutes, seconds)
	}
	hours := minutes / 60
	minutes = minutes % 60
	return fmt.Sprintf("%d小时%d分钟", hours, minutes)
}
