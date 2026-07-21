// Package handler 管理员弹窗通知轮询单元测试
// v0.4.0：覆盖 AdminUpdatePoll 轻量轮询端点核心路径
// 严格遵循铁律 06：所有断言基于已知固定输入，无随机/不确定性
package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/your-org/keyauth-saas/apps/server/internal/config"
	"github.com/your-org/keyauth-saas/apps/server/internal/model"
	"github.com/your-org/keyauth-saas/apps/server/internal/update"
	"github.com/your-org/keyauth-saas/apps/server/pkg/crypto"
)

// setupGin 启动测试模式 gin engine
// 注：使用 sync.Once 保证 gin.SetMode 只被调用一次，避免并发测试中
// 多 goroutine 同时调用 setupGin 触发 DATA RACE（gin.SetMode 写包级变量）
var ginModeOnce sync.Once

func setupGin() *gin.Engine {
	ginModeOnce.Do(func() {
		gin.SetMode(gin.TestMode)
	})
	return gin.New()
}

// setupPollTestDB 启动 SQLite 内存库 + AutoMigrate SystemUpdateLog + SysConfig
func setupPollTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared&_pragma=foreign_keys(1)"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.SystemUpdateLog{}, &model.SysConfig{}))
	// 清空表（cache=shared 模式下可能残留旧数据）
	db.Exec("DELETE FROM system_update_log")
	db.Exec("DELETE FROM sys_config")
	return db
}

// setupPollCrypto 启动 AES-256 crypto manager（ConfigCache.Set 加密配置值需要）
func setupPollCrypto(t *testing.T) *crypto.Manager {
	t.Helper()
	mgr, err := crypto.NewManager("0123456789abcdef0123456789abcdef", "", "")
	require.NoError(t, err)
	return mgr
}

// setupPollMiniRedis 启动 miniredis + 返回 redis.Client
func setupPollMiniRedis(t *testing.T) (*redis.Client, *miniredis.Miniredis) {
	t.Helper()
	mr, err := miniredis.Run()
	require.NoError(t, err)
	t.Cleanup(mr.Close)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	return rdb, mr
}

// setupPollDeps 构造测试 Deps（含真实 ConfigCache，便于测试 sys_config 读取）
func setupPollDeps(t *testing.T) *Deps {
	t.Helper()
	rdb, _ := setupPollMiniRedis(t)
	db := setupPollTestDB(t)
	cfgCache := config.NewConfigCache(db, rdb)
	// Preload 确保配置缓存初始化
	require.NoError(t, cfgCache.Preload(context.Background()))
	return &Deps{
		DB:       db,
		Redis:    rdb,
		Crypto:   setupPollCrypto(t),
		CfgCache: cfgCache,
	}
}

// seedUpdateLog 插入一条 SystemUpdateLog 记录
func seedUpdateLog(t *testing.T, db *gorm.DB, status, trigger, commitAfter string, createdAt time.Time) {
	t.Helper()
	log := model.SystemUpdateLog{
		Status:        status,
		TriggerSource: trigger,
		CommitAfter:   commitAfter,
		CreatedAt:     createdAt,
	}
	require.NoError(t, db.Create(&log).Error)
}

// callPoll 调用 AdminUpdatePoll 并返回响应 body
func callPoll(t *testing.T, deps *Deps) map[string]interface{} {
	t.Helper()
	gin := setupGin()
	gin.GET("/admin/update/poll", AdminUpdatePoll(deps))

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/admin/update/poll", nil)
	gin.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code, "poll 接口应返回 200")

	var resp struct {
		Code int                    `json:"code"`
		Data map[string]interface{} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Equal(t, 0, resp.Code, "code 应为 0")
	return resp.Data
}

// ============== 测试用例 ==============

// TestAdminUpdatePoll_DefaultConfig 默认配置（无 sys_config）应返回 enabled=true + interval=30
func TestAdminUpdatePoll_DefaultConfig(t *testing.T) {
	deps := setupPollDeps(t)

	data := callPoll(t, deps)

	assert.Equal(t, true, data["enabled"], "默认 enabled=true")
	// JSON 数字默认解析为 float64
	interval, ok := data["interval_seconds"].(float64)
	require.True(t, ok, "interval_seconds 应为数字")
	assert.Equal(t, int64(30), int64(interval), "默认 interval_seconds=30")
	// current_commit 取自 git rev-parse HEAD，测试环境在 git 仓库内会返回实际 hash
	// 这里仅断言类型为 string（不强制断言具体值，避免依赖运行环境）
	_, ok = data["current_commit"].(string)
	assert.True(t, ok, "current_commit 应为 string 类型")
	assert.Equal(t, false, data["is_locked"], "默认未锁定")
	// 无审计日志时 last_* 应为 nil
	assert.Nil(t, data["last_update_at"])
	assert.Nil(t, data["last_status"])
	assert.Nil(t, data["last_trigger"])
	assert.Nil(t, data["last_commit"])
}

// TestAdminUpdatePoll_WithLatestLog 有审计日志时返回最近一次更新元信息
func TestAdminUpdatePoll_WithLatestLog(t *testing.T) {
	deps := setupPollDeps(t)
	now := time.Now().Truncate(time.Second)
	seedUpdateLog(t, deps.DB, update.StatusSuccess, update.TriggerSourceWebhook, "abc123def456", now)

	data := callPoll(t, deps)

	lastUpdateAt, ok := data["last_update_at"].(float64)
	require.True(t, ok, "last_update_at 应为数字")
	assert.Equal(t, now.Unix(), int64(lastUpdateAt), "last_update_at 时间戳应匹配")
	assert.Equal(t, update.StatusSuccess, data["last_status"])
	assert.Equal(t, update.TriggerSourceWebhook, data["last_trigger"])
	assert.Equal(t, "abc123def456", data["last_commit"])
}

// TestAdminUpdatePoll_MultipleLogs 多条日志时返回最新一条
func TestAdminUpdatePoll_MultipleLogs(t *testing.T) {
	deps := setupPollDeps(t)
	old := time.Now().Add(-2 * time.Hour).Truncate(time.Second)
	new := time.Now().Add(-30 * time.Minute).Truncate(time.Second)
	seedUpdateLog(t, deps.DB, update.StatusFailed, update.TriggerSourceManual, "old_commit", old)
	seedUpdateLog(t, deps.DB, update.StatusSuccess, update.TriggerSourceWebhook, "new_commit", new)

	data := callPoll(t, deps)

	assert.Equal(t, update.StatusSuccess, data["last_status"], "应返回最新一条")
	assert.Equal(t, "new_commit", data["last_commit"])
}

// TestAdminUpdatePoll_Disabled 配置 update.poll.enabled=0 时 enabled 应返回 false
func TestAdminUpdatePoll_Disabled(t *testing.T) {
	deps := setupPollDeps(t)
	require.NoError(t, deps.CfgCache.Set(context.Background(), update.CfgKeyPollEnabled, "0", "弹窗通知开关", "update", "测试"))

	data := callPoll(t, deps)

	assert.Equal(t, false, data["enabled"], "配置关闭后 enabled=false")
}

// TestAdminUpdatePoll_IntervalCustom 配置自定义间隔应生效
func TestAdminUpdatePoll_IntervalCustom(t *testing.T) {
	deps := setupPollDeps(t)
	require.NoError(t, deps.CfgCache.Set(context.Background(), update.CfgKeyPollInterval, "60", "轮询间隔", "update", "测试"))

	data := callPoll(t, deps)

	interval, ok := data["interval_seconds"].(float64)
	require.True(t, ok)
	assert.Equal(t, int64(60), int64(interval), "自定义 interval_seconds=60 应生效")
}

// TestAdminUpdatePoll_IntervalBelowMin 配置间隔低于下限应被强制提升到 PollIntervalMin
func TestAdminUpdatePoll_IntervalBelowMin(t *testing.T) {
	deps := setupPollDeps(t)
	require.NoError(t, deps.CfgCache.Set(context.Background(), update.CfgKeyPollInterval, "5", "轮询间隔", "update", "测试"))

	data := callPoll(t, deps)

	interval, ok := data["interval_seconds"].(float64)
	require.True(t, ok)
	assert.Equal(t, int64(update.PollIntervalMin), int64(interval), "低于下限应强制提升到 PollIntervalMin=10")
}

// TestAdminUpdatePoll_IntervalEqualMin 配置间隔等于下限应保留
func TestAdminUpdatePoll_IntervalEqualMin(t *testing.T) {
	deps := setupPollDeps(t)
	require.NoError(t, deps.CfgCache.Set(context.Background(), update.CfgKeyPollInterval, "10", "轮询间隔", "update", "测试"))

	data := callPoll(t, deps)

	interval, ok := data["interval_seconds"].(float64)
	require.True(t, ok)
	assert.Equal(t, int64(10), int64(interval), "等于下限应保留")
}

// TestAdminUpdatePoll_EmptyLogTable 空审计日志表时 last_* 全部为 nil
func TestAdminUpdatePoll_EmptyLogTable(t *testing.T) {
	deps := setupPollDeps(t)

	data := callPoll(t, deps)

	assert.Nil(t, data["last_update_at"])
	assert.Nil(t, data["last_status"])
	assert.Nil(t, data["last_trigger"])
	assert.Nil(t, data["last_commit"])
}

// TestAdminUpdatePoll_RespFieldsNotContainLogText 响应不应包含 log_text 重字段
func TestAdminUpdatePoll_RespFieldsNotContainLogText(t *testing.T) {
	deps := setupPollDeps(t)
	now := time.Now().Truncate(time.Second)
	// 即使 log_text 字段有内容，poll 接口也不应返回
	log := model.SystemUpdateLog{
		Status:        update.StatusSuccess,
		TriggerSource: update.TriggerSourceWebhook,
		CommitAfter:   "abc123",
		LogText:       "VERY_LONG_LOG_TEXT_CONTENT_SHOULD_NOT_BE_RETURNED_BY_POLL_ENDPOINT",
		CreatedAt:     now,
	}
	require.NoError(t, deps.DB.Create(&log).Error)

	data := callPoll(t, deps)

	// 验证关键字段存在
	assert.Equal(t, update.StatusSuccess, data["last_status"])
	// 验证 log_text 不在响应中
	_, exists := data["log_text"]
	assert.False(t, exists, "poll 响应不应包含 log_text 字段")
	_, exists = data["LogText"]
	assert.False(t, exists, "poll 响应不应包含 LogText 字段（大小写）")
	_, exists = data["steps_json"]
	assert.False(t, exists, "poll 响应不应包含 steps_json 字段")
}

// TestAdminUpdatePoll_RolledBackStatus 回滚状态应正确返回
func TestAdminUpdatePoll_RolledBackStatus(t *testing.T) {
	deps := setupPollDeps(t)
	now := time.Now().Truncate(time.Second)
	seedUpdateLog(t, deps.DB, update.StatusRolledBack, update.TriggerSourceRollback, "rolled_back_commit", now)

	data := callPoll(t, deps)

	assert.Equal(t, update.StatusRolledBack, data["last_status"])
	assert.Equal(t, update.TriggerSourceRollback, data["last_trigger"])
}

// TestAdminUpdatePoll_ConfigCacheDynamicChange 配置动态变更应即时生效
func TestAdminUpdatePoll_ConfigCacheDynamicChange(t *testing.T) {
	deps := setupPollDeps(t)

	// 初始默认 enabled=true
	data1 := callPoll(t, deps)
	assert.Equal(t, true, data1["enabled"])

	// 动态关闭
	require.NoError(t, deps.CfgCache.Set(context.Background(), update.CfgKeyPollEnabled, "0", "开关", "update", "测试"))
	data2 := callPoll(t, deps)
	assert.Equal(t, false, data2["enabled"], "动态关闭后应立即生效")

	// 动态开启
	require.NoError(t, deps.CfgCache.Set(context.Background(), update.CfgKeyPollEnabled, "1", "开关", "update", "测试"))
	data3 := callPoll(t, deps)
	assert.Equal(t, true, data3["enabled"], "动态开启后应立即生效")
}

// TestCfgKeyPollConstants 配置键常量应正确
func TestCfgKeyPollConstants(t *testing.T) {
	assert.Equal(t, "update.poll.enabled", update.CfgKeyPollEnabled)
	assert.Equal(t, "update.poll.interval_seconds", update.CfgKeyPollInterval)
	assert.Equal(t, 10, update.PollIntervalMin, "下限应为 10 秒")
}

// TestAdminUpdatePoll_AllFieldsPresent 响应应包含所有预期字段
func TestAdminUpdatePoll_AllFieldsPresent(t *testing.T) {
	deps := setupPollDeps(t)
	now := time.Now().Truncate(time.Second)
	seedUpdateLog(t, deps.DB, update.StatusSuccess, update.TriggerSourceWebhook, "abc123", now)

	data := callPoll(t, deps)

	expectedFields := []string{
		"enabled", "interval_seconds", "current_commit", "is_locked",
		"last_update_at", "last_status", "last_trigger", "last_commit",
	}
	for _, field := range expectedFields {
		_, exists := data[field]
		assert.True(t, exists, "响应应包含字段: %s", field)
	}
}
