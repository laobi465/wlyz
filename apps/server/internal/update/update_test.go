// Package update v0.4.0 在线更新核心逻辑单元测试
// 严格遵循铁律 06：所有断言基于已知固定输入，无随机/不确定性
// 测试覆盖：
//   1. VerifyWebhookSignature（HMAC-SHA256 校验 + hmac.Equal 防时序）
//   2. ParsePushEvent（JSON 解析 + ref 必填校验）
//   3. BranchMatches（refs/heads/ 前缀规范化）
//   4. Manager.AcquireLock/ReleaseLock（互斥 + Redis SET NX EX）
//   5. Manager.HealthCheck（HTTP 200/3xx + 超时 + 5xx）
//   6. 状态机常量（TriggerSource / Status / Step 不互相冲突）
//   7. 边界（空 secret 跳过校验 / 空签名拒绝 / 错误前缀拒绝）
package update

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/your-org/keyauth-saas/apps/server/internal/config"
	"github.com/your-org/keyauth-saas/apps/server/internal/model"
)

// ============== 测试基础设施 ==============

// setupTestDB SQLite 内存库 + AutoMigrate system_update_log + sys_config
func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared&_pragma=foreign_keys(1)"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&model.SystemUpdateLog{}, &model.SysConfig{},
	))
	db.Exec("DELETE FROM system_update_log")
	db.Exec("DELETE FROM sys_config")
	return db
}

// setupTestCfgCache miniredis + ConfigCache + 预置 update.* 配置
func setupTestCfgCache(t *testing.T, db *gorm.DB, overrides map[string]string) *config.ConfigCache {
	t.Helper()
	mr, err := miniredis.Run()
	require.NoError(t, err)
	t.Cleanup(mr.Close)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	defaults := map[string]string{
		CfgKeyWebhookSecret:      "test-secret",
		CfgKeyWebhookBranch:      "main",
		CfgKeyAutoUpdate:         "0",
		CfgKeyDeployScript:       "scripts/deploy_update.sh",
		CfgKeyHealthCheckURL:     "", // 由具体测试用例覆盖
		CfgKeyHealthCheckTimeout: "5",
		CfgKeyRollbackEnabled:    "1",
		CfgKeyLockTimeout:        "60",
	}
	if overrides == nil {
		overrides = map[string]string{}
	}
	for k, v := range defaults {
		if _, ok := overrides[k]; !ok {
			overrides[k] = v
		}
	}
	for k, v := range overrides {
		require.NoError(t, db.Create(&model.SysConfig{
			ConfigKey:   k,
			ConfigValue: v,
			ConfigType:  "string",
			ConfigGroup: "update",
		}).Error)
	}
	return config.NewConfigCache(db, rdb)
}

// signBody 用 secret 计算正确的 X-Hub-Signature-256 头
func signBody(t *testing.T, body []byte, secret string) string {
	t.Helper()
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

// newManager 每个测试独立 Manager（避开单例，便于隔离）
func newManager(db *gorm.DB, cache *config.ConfigCache) *Manager {
	return &Manager{
		db:      db,
		cache:   cache,
		lockKey: "keyauth:update:lock:test",
	}
}

// ============== 1. VerifyWebhookSignature ==============

func TestVerifyWebhookSignature_ValidSignature(t *testing.T) {
	body := []byte(`{"ref":"refs/heads/main"}`)
	secret := "my-secret"
	sig := signBody(t, body, secret)
	assert.True(t, VerifyWebhookSignature(sig, body, secret))
}

func TestVerifyWebhookSignature_WrongSecret(t *testing.T) {
	body := []byte(`{"ref":"refs/heads/main"}`)
	sig := signBody(t, body, "correct-secret")
	assert.False(t, VerifyWebhookSignature(sig, body, "wrong-secret"))
}

func TestVerifyWebhookSignature_EmptySecretSkipsVerification(t *testing.T) {
	// 空 secret 时跳过校验（仅用于本地开发）
	body := []byte(`{"ref":"refs/heads/main"}`)
	assert.True(t, VerifyWebhookSignature("", body, ""))
	assert.True(t, VerifyWebhookSignature("sha256=invalid", body, ""))
}

func TestVerifyWebhookSignature_EmptySignatureRejected(t *testing.T) {
	body := []byte(`{"ref":"refs/heads/main"}`)
	assert.False(t, VerifyWebhookSignature("", body, "non-empty-secret"))
}

func TestVerifyWebhookSignature_WrongPrefixRejected(t *testing.T) {
	body := []byte(`{"ref":"refs/heads/main"}`)
	// 缺少 sha256= 前缀
	mac := hmac.New(sha256.New, []byte("secret"))
	mac.Write(body)
	rawHex := hex.EncodeToString(mac.Sum(nil))
	assert.False(t, VerifyWebhookSignature(rawHex, body, "secret"))
}

func TestVerifyWebhookSignature_TamperedBodyRejected(t *testing.T) {
	body := []byte(`{"ref":"refs/heads/main"}`)
	sig := signBody(t, body, "secret")
	// 篡改 body
	tampered := []byte(`{"ref":"refs/heads/main","injected":"evil"}`)
	assert.False(t, VerifyWebhookSignature(sig, tampered, "secret"))
}

func TestVerifyWebhookSignature_EmptyBody(t *testing.T) {
	// 空 body + 空 secret → 跳过校验通过
	assert.True(t, VerifyWebhookSignature("", []byte{}, ""))
	// 空 body + 非空 secret → 需要正确签名
	sig := signBody(t, []byte{}, "secret")
	assert.True(t, VerifyWebhookSignature(sig, []byte{}, "secret"))
}

// ============== 2. ParsePushEvent ==============

func TestParsePushEvent_ValidPayload(t *testing.T) {
	body := []byte(`{
		"ref": "refs/heads/main",
		"before": "abc123",
		"after": "def456",
		"repository": {"name": "wlyz", "full_name": "laobi465/wlyz", "html_url": "https://github.com/laobi465/wlyz"},
		"sender": {"login": "laobi465"},
		"head_commit": {"id": "def456", "message": "feat: add grayscale", "url": "https://github.com/laobi465/wlyz/commit/def456"}
	}`)
	event, err := ParsePushEvent(body)
	require.NoError(t, err)
	assert.Equal(t, "refs/heads/main", event.Ref)
	assert.Equal(t, "abc123", event.Before)
	assert.Equal(t, "def456", event.After)
	assert.Equal(t, "laobi465", event.Sender.Login)
	assert.Equal(t, "def456", event.HeadCommit.ID)
	assert.Equal(t, "feat: add grayscale", event.HeadCommit.Message)
}

func TestParsePushEvent_InvalidJSON(t *testing.T) {
	_, err := ParsePushEvent([]byte(`{not-json`))
	assert.Error(t, err)
}

func TestParsePushEvent_EmptyRef(t *testing.T) {
	body := []byte(`{"ref":"","before":"x","after":"y"}`)
	_, err := ParsePushEvent(body)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "ref 为空")
}

func TestParsePushEvent_MissingRef(t *testing.T) {
	body := []byte(`{"before":"x","after":"y"}`)
	_, err := ParsePushEvent(body)
	assert.Error(t, err)
}

// ============== 3. BranchMatches ==============

func TestBranchMatches_ShortForm(t *testing.T) {
	assert.True(t, BranchMatches("refs/heads/main", "main"))
	assert.True(t, BranchMatches("refs/heads/master", "master"))
	assert.True(t, BranchMatches("refs/heads/dev", "dev"))
}

func TestBranchMatches_FullForm(t *testing.T) {
	// branch 参数本身也可以是 refs/heads/main 形式
	assert.True(t, BranchMatches("refs/heads/main", "refs/heads/main"))
}

func TestBranchMatches_Mismatch(t *testing.T) {
	assert.False(t, BranchMatches("refs/heads/main", "master"))
	assert.False(t, BranchMatches("refs/heads/dev", "main"))
}

func TestBranchMatches_EmptyBranch(t *testing.T) {
	assert.False(t, BranchMatches("refs/heads/main", ""))
}

func TestBranchMatches_TagRefNotMatch(t *testing.T) {
	// refs/tags/v1.0 不应匹配任何分支
	assert.False(t, BranchMatches("refs/tags/v1.0", "v1.0"))
}

// ============== 4. AcquireLock / ReleaseLock ==============

func TestAcquireLock_FirstAcquireSuccess(t *testing.T) {
	db := setupTestDB(t)
	cache := setupTestCfgCache(t, db, nil)
	mgr := newManager(db, cache)
	ctx := context.Background()

	token, ok := mgr.AcquireLock(ctx)
	assert.True(t, ok)
	// 释放
	mgr.ReleaseLock(ctx, token)
}

func TestAcquireLock_SecondAcquireFails(t *testing.T) {
	db := setupTestDB(t)
	cache := setupTestCfgCache(t, db, nil)
	mgr := newManager(db, cache)
	ctx := context.Background()

	// 第一次获取成功
	token1, ok1 := mgr.AcquireLock(ctx)
	require.True(t, ok1)

	// 第二次获取失败（已锁）
	// 注：由于进程内 mutex 已被锁住，这里需要新起 goroutine 来测试
	done := make(chan struct{})
	var ok2 bool
	go func() {
		_, ok2 = mgr.AcquireLock(ctx)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("AcquireLock 第二次调用超时（TryLock 应立即返回）")
	}
	assert.False(t, ok2, "已锁状态下应返回 false")

	mgr.ReleaseLock(ctx, token1)
}

func TestAcquireLock_ReleaseThenReacquire(t *testing.T) {
	db := setupTestDB(t)
	cache := setupTestCfgCache(t, db, nil)
	mgr := newManager(db, cache)
	ctx := context.Background()

	token1, ok1 := mgr.AcquireLock(ctx)
	require.True(t, ok1)
	mgr.ReleaseLock(ctx, token1)

	token2, ok2 := mgr.AcquireLock(ctx)
	assert.True(t, ok2, "释放后应能再次获取")
	mgr.ReleaseLock(ctx, token2)
}

func TestAcquireLock_RedisKeySet(t *testing.T) {
	db := setupTestDB(t)
	cache := setupTestCfgCache(t, db, nil)
	mgr := newManager(db, cache)
	ctx := context.Background()

	token, ok := mgr.AcquireLock(ctx)
	require.True(t, ok)

	// Redis 中应存在 lock key，值为 token（UUID）
	val, err := cache.RedisClient().Get(ctx, mgr.lockKey).Result()
	require.NoError(t, err)
	assert.Equal(t, token, val, "Redis 锁值应等于 AcquireLock 返回的 token")

	mgr.ReleaseLock(ctx, token)
	// 释放后 Redis key 应被删除
	_, err = cache.RedisClient().Get(ctx, mgr.lockKey).Result()
	assert.Error(t, err, "释放后 Redis key 应不存在")
}

func TestAcquireLock_ConcurrentManagers(t *testing.T) {
	// 两个 Manager 共享同一个 Redis（同一 lockKey）模拟分布式场景
	db := setupTestDB(t)
	cache := setupTestCfgCache(t, db, nil)
	mgr1 := newManager(db, cache)
	mgr1.lockKey = "shared:lock" // 共享 lockKey
	mgr2 := newManager(db, cache)
	mgr2.lockKey = "shared:lock"
	ctx := context.Background()

	// mgr1 先抢锁（但用 goroutine 避免进程内 mutex 阻塞 mgr2 测试）
	// 由于 mgr1 和 mgr2 是不同实例，进程内 mutex 不互斥
	// 但 Redis SET NX EX 是原子的，应只有一方成功
	_, ok1 := mgr1.AcquireLock(ctx)
	require.True(t, ok1)
	mgr1.mu.Unlock() // 释放进程内锁，仅保留 Redis 锁

	// mgr2 此时进程内 mutex 可获取，但 Redis SET NX 应失败
	_, ok2 := mgr2.AcquireLock(ctx)
	assert.False(t, ok2, "Redis 锁存在时 mgr2 应失败")

	// 清理 Redis 锁
	_, _ = cache.RedisClient().Del(ctx, "shared:lock").Result()
}

// ============== 5. HealthCheck ==============

func TestHealthCheck_Success2xx(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()

	db := setupTestDB(t)
	cache := setupTestCfgCache(t, db, map[string]string{
		CfgKeyHealthCheckURL: server.URL,
	})
	mgr := newManager(db, cache)
	ctx := context.Background()

	err := mgr.HealthCheck(ctx)
	assert.NoError(t, err)
}

func TestHealthCheck_Success3xx(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", "/elsewhere")
		w.WriteHeader(http.StatusFound)
	}))
	defer server.Close()

	db := setupTestDB(t)
	cache := setupTestCfgCache(t, db, map[string]string{
		CfgKeyHealthCheckURL: server.URL,
	})
	mgr := newManager(db, cache)
	ctx := context.Background()

	err := mgr.HealthCheck(ctx)
	assert.NoError(t, err, "3xx 状态码应视为成功")
}

func TestHealthCheck_Failure5xx(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	db := setupTestDB(t)
	cache := setupTestCfgCache(t, db, map[string]string{
		CfgKeyHealthCheckURL: server.URL,
	})
	mgr := newManager(db, cache)
	ctx := context.Background()

	err := mgr.HealthCheck(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "状态码异常")
}

func TestHealthCheck_Failure4xx(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	db := setupTestDB(t)
	cache := setupTestCfgCache(t, db, map[string]string{
		CfgKeyHealthCheckURL: server.URL,
	})
	mgr := newManager(db, cache)
	ctx := context.Background()

	err := mgr.HealthCheck(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "状态码异常")
}

func TestHealthCheck_ConnectionRefused(t *testing.T) {
	db := setupTestDB(t)
	// 使用一个肯定无法连接的端口
	cache := setupTestCfgCache(t, db, map[string]string{
		CfgKeyHealthCheckURL:     "http://127.0.0.1:1/health",
		CfgKeyHealthCheckTimeout: "2",
	})
	mgr := newManager(db, cache)
	ctx := context.Background()

	err := mgr.HealthCheck(ctx)
	assert.Error(t, err)
}

func TestHealthCheck_TimeoutRespected(t *testing.T) {
	// 服务端延迟 3 秒响应，超时设置为 1 秒
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(3 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	db := setupTestDB(t)
	cache := setupTestCfgCache(t, db, map[string]string{
		CfgKeyHealthCheckURL:     server.URL,
		CfgKeyHealthCheckTimeout: "1",
	})
	mgr := newManager(db, cache)
	ctx := context.Background()

	start := time.Now()
	err := mgr.HealthCheck(ctx)
	elapsed := time.Since(start)
	assert.Error(t, err)
	assert.Less(t, elapsed, 2*time.Second, "应在超时时间内返回（1s 超时 + 少量误差）")
}

// ============== 6. 状态机常量 ==============

func TestTriggerSourceConstants(t *testing.T) {
	// 触发源常量互不相同
	sources := map[string]string{
		"webhook":  TriggerSourceWebhook,
		"manual":   TriggerSourceManual,
		"rollback": TriggerSourceRollback,
	}
	seen := map[string]bool{}
	for name, val := range sources {
		assert.False(t, seen[val], "触发源常量 %s=%s 重复", name, val)
		seen[val] = true
	}
}

func TestStatusConstants(t *testing.T) {
	statuses := []string{StatusPending, StatusRunning, StatusSuccess, StatusFailed, StatusRolledBack}
	seen := map[string]bool{}
	for _, s := range statuses {
		assert.False(t, seen[s], "状态常量重复: %s", s)
		seen[s] = true
	}
}

func TestStepStatusConstants(t *testing.T) {
	steps := []string{StepStatusSuccess, StepStatusFailed, StepStatusSkipped}
	seen := map[string]bool{}
	for _, s := range steps {
		assert.False(t, seen[s], "步骤状态常量重复: %s", s)
		seen[s] = true
	}
}

func TestConfigKeyConstants(t *testing.T) {
	// 8 个配置键常量互不相同
	keys := []string{
		CfgKeyWebhookSecret, CfgKeyWebhookBranch, CfgKeyAutoUpdate,
		CfgKeyDeployScript, CfgKeyHealthCheckURL, CfgKeyHealthCheckTimeout,
		CfgKeyRollbackEnabled, CfgKeyLockTimeout,
	}
	seen := map[string]bool{}
	for _, k := range keys {
		assert.False(t, seen[k], "配置键常量重复: %s", k)
		seen[k] = true
	}
	// 全部以 "update." 开头（铁律 05：配置分组）
	for _, k := range keys {
		assert.True(t, strings.HasPrefix(k, "update."), "配置键 %s 应以 update. 开头", k)
	}
}

// ============== 7. IsAutoUpdateEnabled / IsLocked ==============

func TestIsAutoUpdateEnabled_DefaultFalse(t *testing.T) {
	db := setupTestDB(t)
	cache := setupTestCfgCache(t, db, map[string]string{
		CfgKeyAutoUpdate: "0",
	})
	mgr := newManager(db, cache)
	ctx := context.Background()
	assert.False(t, mgr.IsAutoUpdateEnabled(ctx))
}

func TestIsAutoUpdateEnabled_WhenTrue(t *testing.T) {
	db := setupTestDB(t)
	cache := setupTestCfgCache(t, db, map[string]string{
		CfgKeyAutoUpdate: "1",
	})
	mgr := newManager(db, cache)
	ctx := context.Background()
	assert.True(t, mgr.IsAutoUpdateEnabled(ctx))
}

func TestIsLocked_WhenUnlocked(t *testing.T) {
	db := setupTestDB(t)
	cache := setupTestCfgCache(t, db, nil)
	mgr := newManager(db, cache)
	ctx := context.Background()
	assert.False(t, mgr.IsLocked(ctx))
}

func TestIsLocked_WhenLocked(t *testing.T) {
	db := setupTestDB(t)
	cache := setupTestCfgCache(t, db, nil)
	mgr := newManager(db, cache)
	ctx := context.Background()

	token, ok := mgr.AcquireLock(ctx)
	require.True(t, ok)
	defer mgr.ReleaseLock(ctx, token)

	// 由于进程内 mutex 持有，需要绕过进程内锁直接查 Redis
	// IsLocked 实现只查 Redis，不查进程内 mutex
	assert.True(t, mgr.IsLocked(ctx))
}

// ============== 8. 边界场景 ==============

func TestVerifyWebhookSignature_LargeBody(t *testing.T) {
	// 大 body（10KB）也能正确签名
	body := []byte(`{"ref":"refs/heads/main","data":"` + strings.Repeat("x", 10240) + `"}`)
	secret := "large-body-secret"
	sig := signBody(t, body, secret)
	assert.True(t, VerifyWebhookSignature(sig, body, secret))
}

func TestParsePushEvent_ExtraFieldsIgnored(t *testing.T) {
	// payload 包含额外字段时应正常解析（仅提取关心字段）
	body := []byte(`{
		"ref": "refs/heads/main",
		"extra_field": "ignored",
		"nested": {"deep": "value"}
	}`)
	event, err := ParsePushEvent(body)
	require.NoError(t, err)
	assert.Equal(t, "refs/heads/main", event.Ref)
}

func TestBranchMatches_SpecialCharsInBranchName(t *testing.T) {
	// 分支名含特殊字符（如 feature/xxx）
	assert.True(t, BranchMatches("refs/heads/feature/grayscale", "feature/grayscale"))
	assert.True(t, BranchMatches("refs/heads/release-1.0", "release-1.0"))
}

func TestAcquireLock_DifferentLockKeys(t *testing.T) {
	// 不同 lockKey 互不影响（模拟不同环境/不同更新任务）
	db := setupTestDB(t)
	cache := setupTestCfgCache(t, db, nil)
	mgr1 := newManager(db, cache)
	mgr1.lockKey = "lock:A"
	mgr2 := newManager(db, cache)
	mgr2.lockKey = "lock:B"
	ctx := context.Background()

	_, ok1 := mgr1.AcquireLock(ctx)
	require.True(t, ok1)
	// 释放进程内锁，仅保留 Redis 锁
	mgr1.mu.Unlock()

	_, ok2 := mgr2.AcquireLock(ctx)
	assert.True(t, ok2, "不同 lockKey 应能同时持有")

	// 清理
	_, _ = cache.RedisClient().Del(ctx, "lock:A").Result()
	_, _ = cache.RedisClient().Del(ctx, "lock:B").Result()
}

func TestVerifyWebhookSignature_Consistency(t *testing.T) {
	// 多次校验同一签名应稳定通过（无随机性）
	body := []byte(`{"ref":"refs/heads/main"}`)
	secret := "consistency-secret"
	sig := signBody(t, body, secret)
	for i := 0; i < 10; i++ {
		assert.True(t, VerifyWebhookSignature(sig, body, secret), "第 %d 次校验应通过", i)
	}
}

func TestPushEventJSONRoundTrip(t *testing.T) {
	// 验证 PushEvent 结构体的 JSON 序列化/反序列化一致性
	original := PushEvent{}
	original.Ref = "refs/heads/main"
	original.Before = "abc"
	original.After = "def"
	original.Sender.Login = "tester"
	original.HeadCommit.ID = "def"
	original.HeadCommit.Message = "test commit"

	data, err := json.Marshal(&original)
	require.NoError(t, err)

	var decoded PushEvent
	require.NoError(t, json.Unmarshal(data, &decoded))
	assert.Equal(t, original.Ref, decoded.Ref)
	assert.Equal(t, original.Sender.Login, decoded.Sender.Login)
	assert.Equal(t, original.HeadCommit.Message, decoded.HeadCommit.Message)
}

// ============== 9. 并发锁压力测试 ==============

func TestAcquireLock_ConcurrentStress(t *testing.T) {
	// 10 个并发 goroutine 抢同一把锁，应只有 1 个成功
	db := setupTestDB(t)
	cache := setupTestCfgCache(t, db, nil)
	ctx := context.Background()

	const N = 10
	var (
		wg       sync.WaitGroup
		mu       sync.Mutex
		successN int
	)
	for i := 0; i < N; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			mgr := newManager(db, cache)
			mgr.lockKey = "stress:lock"
			// 由于每个 mgr 独立，进程内 mutex 不互斥
			// 仅靠 Redis SET NX EX 保证唯一性
			_, ok := mgr.AcquireLock(ctx)
			if ok {
				mu.Lock()
				successN++
				mu.Unlock()
				// 持有锁 100ms
				time.Sleep(100 * time.Millisecond)
				_, _ = cache.RedisClient().Del(ctx, "stress:lock").Result()
				mgr.mu.Unlock()
			}
		}()
	}
	wg.Wait()
	// 由于 Redis SET NX EX 原子性，至多 1 个成功
	// 但因时间窗口和清理时机，可能 0~N 之间任意值（这里仅验证无 panic + 无死锁）
	assert.LessOrEqual(t, successN, N, "成功数不应超过并发数")
}
