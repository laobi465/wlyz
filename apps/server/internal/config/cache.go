// Package config - sys_config 缓存实现
// 严格遵循铁律 05：所有配置走 sys_config 表 + Redis 缓存 + 后台可视化编辑
package config

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/your-org/keyauth-saas/apps/server/internal/model"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

// 配置缓存相关常量
const (
	configCacheTTL      = 3600 * time.Second // 1 小时
	configCacheKeyPrefix = "config:"
)

// ConfigCache sys_config 配置缓存
// - 优先从 Redis 取
// - 未命中则查 MySQL，回写 Redis
// - 后台修改时主动清除缓存
type ConfigCache struct {
	db    *gorm.DB
	redis *redis.Client
}

// NewConfigCache 构造
func NewConfigCache(db *gorm.DB, rdb *redis.Client) *ConfigCache {
	return &ConfigCache{db: db, redis: rdb}
}

// GetString 读取字符串配置
// fallback 为缓存/DB 全未命中时的兜底默认值（铁律 04：默认值作参数，不硬编码在业务代码里）
func (c *ConfigCache) GetString(ctx context.Context, key, fallback string) string {
	val, ok := c.getRaw(ctx, key)
	if !ok {
		return fallback
	}
	return val
}

// GetInt 读取整型配置
func (c *ConfigCache) GetInt(ctx context.Context, key string, fallback int) int {
	val, ok := c.getRaw(ctx, key)
	if !ok {
		return fallback
	}
	n, err := strconv.Atoi(val)
	if err != nil {
		return fallback
	}
	return n
}

// GetInt64 读取 int64 配置
func (c *ConfigCache) GetInt64(ctx context.Context, key string, fallback int64) int64 {
	val, ok := c.getRaw(ctx, key)
	if !ok {
		return fallback
	}
	n, err := strconv.ParseInt(val, 10, 64)
	if err != nil {
		return fallback
	}
	return n
}

// GetBool 读取布尔配置
func (c *ConfigCache) GetBool(ctx context.Context, key string, fallback bool) bool {
	val, ok := c.getRaw(ctx, key)
	if !ok {
		return fallback
	}
	b, err := strconv.ParseBool(val)
	if err != nil {
		return fallback
	}
	return b
}

// GetFloat64 读取 float64 配置
func (c *ConfigCache) GetFloat64(ctx context.Context, key string, fallback float64) float64 {
	val, ok := c.getRaw(ctx, key)
	if !ok {
		return fallback
	}
	f, err := strconv.ParseFloat(val, 64)
	if err != nil {
		return fallback
	}
	return f
}

// GetJSON 读取 JSON 配置（反序列化到 out）
func (c *ConfigCache) GetJSON(ctx context.Context, key string, out interface{}) error {
	val, ok := c.getRaw(ctx, key)
	if !ok {
		return fmt.Errorf("配置项 %s 不存在", key)
	}
	return json.Unmarshal([]byte(val), out)
}

// Set 写入配置（后台编辑时调用）
// 同时更新 MySQL + 刷新 Redis 缓存
func (c *ConfigCache) Set(ctx context.Context, key, value, name, group, remark string) error {
	// 1. 写入 MySQL（upsert）
	row := model.SysConfig{
		ConfigKey:    key,
		ConfigValue:  value,
		ConfigType:   "string",
		ConfigName:   name,
		ConfigGroup:  group,
		Remark:       remark,
	}
	result := c.db.WithContext(ctx).Where("config_key = ?", key).
		Assign(model.SysConfig{
			ConfigValue: value,
			ConfigName:  name,
			ConfigGroup: group,
			Remark:      remark,
		}).
		FirstOrCreate(&row)
	if result.Error != nil {
		return fmt.Errorf("写入 sys_config 失败: %w", result.Error)
	}

	// 2. 删除 Redis 缓存（下次读取自动重建）
	return c.invalidate(ctx, key)
}

// invalidate 清除指定 key 的缓存
func (c *ConfigCache) invalidate(ctx context.Context, key string) error {
	return c.redis.Del(ctx, configCacheKeyPrefix+key).Err()
}

// InvalidateAll 清除全部配置缓存（用于后台"刷新缓存"按钮）
func (c *ConfigCache) InvalidateAll(ctx context.Context) error {
	pattern := configCacheKeyPrefix + "*"
	iter := c.redis.Scan(ctx, 0, pattern, 100).Iterator()
	keys := []string{}
	for iter.Next(ctx) {
		keys = append(keys, iter.Val())
	}
	if err := iter.Err(); err != nil {
		return err
	}
	if len(keys) > 0 {
		return c.redis.Del(ctx, keys...).Err()
	}
	return nil
}

// getRaw 内部读取原始字符串值
func (c *ConfigCache) getRaw(ctx context.Context, key string) (string, bool) {
	cacheKey := configCacheKeyPrefix + key

	// 1. 查 Redis
	if val, err := c.redis.Get(ctx, cacheKey).Result(); err == nil {
		if val == "__NULL__" {
			// 穿透保护标记
			return "", false
		}
		return val, true
	}

	// 2. 查 MySQL
	var row model.SysConfig
	err := c.db.WithContext(ctx).Select("config_value").Where("config_key = ?", key).First(&row).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			// 写入穿透保护标记，60s 过期
			_ = c.redis.Set(ctx, cacheKey, "__NULL__", 60*time.Second).Err()
			return "", false
		}
		return "", false
	}

	// 3. 回写 Redis 缓存
	_ = c.redis.Set(ctx, cacheKey, row.ConfigValue, configCacheTTL).Err()
	return row.ConfigValue, true
}

// Preload 启动时预加载全部配置到缓存
func (c *ConfigCache) Preload(ctx context.Context) error {
	var rows []model.SysConfig
	if err := c.db.WithContext(ctx).Find(&rows).Error; err != nil {
		return err
	}
	pipe := c.redis.Pipeline()
	for _, r := range rows {
		pipe.Set(ctx, configCacheKeyPrefix+r.ConfigKey, r.ConfigValue, configCacheTTL)
	}
	_, err := pipe.Exec(ctx)
	return err
}

// ListByGroup 后台按分组查询配置
func (c *ConfigCache) ListByGroup(ctx context.Context, group string) ([]model.SysConfig, error) {
	var rows []model.SysConfig
	q := c.db.WithContext(ctx).Order("config_group ASC, config_key ASC")
	if group != "" {
		q = q.Where("config_group = ?", group)
	}
	err := q.Find(&rows).Error
	return rows, err
}
