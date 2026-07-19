// 心跳保活服务
// 用 Redis Sorted Set 维护在线设备，score 为最近心跳时间戳
// 严格遵循铁律 05：心跳周期/超时/宽限期全部从 sys_config 读取
package heartbeat

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

// Redis Key 规范：
// 在线设备集合：heartbeat:online:{app_id}  -> ZSET, member=device_id, score=last_heartbeat_ts
// 设备心跳详情：heartbeat:detail:{app_id}:{device_id} -> Hash, 含 ip/user_agent/last_ts

const (
	keyOnlinePrefix = "heartbeat:online:"
	keyDetailPrefix = "heartbeat:detail:"
)

func onlineKey(appID uint64) string  { return fmt.Sprintf("%s%d", keyOnlinePrefix, appID) }
func detailKey(appID, deviceID uint64) string {
	return fmt.Sprintf("%s%d:%d", keyDetailPrefix, appID, deviceID)
}

// Record 记录一次心跳
// 同时更新 ZSET（score=当前时间戳）和 Hash（详情）
func Record(
	ctx context.Context,
	rdb *redis.Client,
	appID, deviceID uint64,
	ip, userAgent string,
) error {
	if rdb == nil {
		return fmt.Errorf("redis 未初始化")
	}
	now := time.Now().Unix()
	pipe := rdb.TxPipeline()
	// 1. 更新在线 ZSET
	pipe.ZAdd(ctx, onlineKey(appID), redis.Z{
		Score:  float64(now),
		Member: strconv.FormatUint(deviceID, 10),
	})
	// 2. 更新设备详情
	pipe.HSet(ctx, detailKey(appID, deviceID), map[string]interface{}{
		"last_ts":    now,
		"ip":         ip,
		"user_agent": userAgent,
	})
	// 3. 设置详情 Hash 的 TTL（30 天，超时自动清理孤儿数据）
	pipe.Expire(ctx, detailKey(appID, deviceID), 30*24*time.Hour)
	_, err := pipe.Exec(ctx)
	return err
}

// IsOnline 检查设备是否在线
// heartbeatTimeout：超过该秒数未心跳则视为离线
func IsOnline(ctx context.Context, rdb *redis.Client, appID, deviceID uint64, heartbeatTimeout int) (bool, error) {
	if rdb == nil {
		return false, fmt.Errorf("redis 未初始化")
	}
	if heartbeatTimeout <= 0 {
		heartbeatTimeout = 180
	}
	member := strconv.FormatUint(deviceID, 10)
	score, err := rdb.ZScore(ctx, onlineKey(appID), member).Result()
	if err == redis.Nil {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	now := time.Now().Unix()
	return now-int64(score) < int64(heartbeatTimeout), nil
}

// Remove 移除设备心跳（用于解绑/封禁时）
func Remove(ctx context.Context, rdb *redis.Client, appID, deviceID uint64) error {
	if rdb == nil {
		return nil
	}
	pipe := rdb.TxPipeline()
	pipe.ZRem(ctx, onlineKey(appID), strconv.FormatUint(deviceID, 10))
	pipe.Del(ctx, detailKey(appID, deviceID))
	_, err := pipe.Exec(ctx)
	return err
}

// CountOnline 统计应用当前在线设备数
func CountOnline(ctx context.Context, rdb *redis.Client, appID uint64, heartbeatTimeout int) (int64, error) {
	if rdb == nil {
		return 0, fmt.Errorf("redis 未初始化")
	}
	if heartbeatTimeout <= 0 {
		heartbeatTimeout = 180
	}
	cutoff := float64(time.Now().Add(-time.Duration(heartbeatTimeout) * time.Second).Unix())
	// 统计 score > cutoff 的成员数
	return rdb.ZCount(ctx, onlineKey(appID), strconv.FormatInt(int64(cutoff), 10), "+inf").Result()
}

// ListOnline 列出应用在线设备 ID
// 用于看板展示
func ListOnline(ctx context.Context, rdb *redis.Client, appID uint64, heartbeatTimeout int, offset, count int64) ([]uint64, error) {
	if rdb == nil {
		return nil, fmt.Errorf("redis 未初始化")
	}
	if heartbeatTimeout <= 0 {
		heartbeatTimeout = 180
	}
	cutoff := time.Now().Add(-time.Duration(heartbeatTimeout) * time.Second).Unix()
	// ZRANGEBYSCORE 返回 score >= cutoff 的成员
	members, err := rdb.ZRangeByScore(ctx, onlineKey(appID), &redis.ZRangeBy{
		Min:    strconv.FormatInt(cutoff, 10),
		Max:    "+inf",
		Offset: offset,
		Count:  count,
	}).Result()
	if err != nil {
		return nil, err
	}
	ids := make([]uint64, 0, len(members))
	for _, m := range members {
		if id, err := strconv.ParseUint(m, 10, 64); err == nil {
			ids = append(ids, id)
		}
	}
	return ids, nil
}

// GetLastHeartbeatAt 获取设备最后一次心跳时间
// 用于客户端 verify 接口返回
func GetLastHeartbeatAt(ctx context.Context, rdb *redis.Client, appID, deviceID uint64) (time.Time, error) {
	if rdb == nil {
		return time.Time{}, fmt.Errorf("redis 未初始化")
	}
	member := strconv.FormatUint(deviceID, 10)
	score, err := rdb.ZScore(ctx, onlineKey(appID), member).Result()
	if err == redis.Nil {
		return time.Time{}, nil // 从未心跳
	}
	if err != nil {
		return time.Time{}, err
	}
	return time.Unix(int64(score), 0), nil
}
