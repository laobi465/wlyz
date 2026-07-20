// Package openapi v0.4.0 API 开放平台单元测试
// 严格遵循铁律 06：所有断言基于已知固定输入，无随机/不确定性
// 测试覆盖：
//   1. hashToken / signWebhook / VerifyWebhookSignature（哈希 + 签名算法）
//   2. generateRandomString（长度 / 唯一性）
//   3. generateUUID（格式 / 唯一性）
//   4. ValidateScopes（合法 / 非法 / 空 / 空 available）
//   5. HasScope（命中 / 未命中 / 空 scopes）
//   6. ParseScopes（解析 / 空 / 多空格）
//   7. isSubscribed（订阅 / 未订阅 / 空事件）
//   8. truncate（短 / 长 / 等长）
//   9. TokenManager.GenerateToken（正常 / 无效 scope / 数量上限 / TTL 永久 / TTL 有限）
//  10. TokenManager.ValidateToken（有效 / 撤销 / 过期 / 不存在 / last_used 更新）
//  11. TokenManager.RevokeToken（成功 / 不存在 / 已撤销）
//  12. TokenManager.ListTokens / GetToken
//  13. WebhookManager.CreateEndpoint（正常 / 非法 URL / 带 secret 加密）
//  14. WebhookManager.UpdateEndpoint / DeleteEndpoint / GetEndpoint / ListEndpoints
//  15. WebhookManager.DispatchEvent（无订阅 / 有订阅 / 多端点）
//  16. WebhookManager.RetryDelivery（成功 / 已成功不可重试 / 超过重试次数）
//  17. WebhookManager.ListDeliveries / GetDelivery
//  18. WebhookManager.ProcessPendingRetries
package openapi

import (
	"context"
	"crypto/sha512"
	"encoding/hex"
	"fmt"
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
	"github.com/your-org/keyauth-saas/apps/server/pkg/crypto"
)

// ============== 测试基础设施 ==============

func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:openapi_test_%d?mode=memory&cache=shared", time.Now().UnixNano())), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&model.DeveloperAPIToken{},
		&model.WebhookEndpoint{},
		&model.WebhookDelivery{},
		&model.SysConfig{},
	))
	db.Exec("DELETE FROM developer_api_token")
	db.Exec("DELETE FROM webhook_endpoint")
	db.Exec("DELETE FROM webhook_delivery")
	db.Exec("DELETE FROM sys_config")
	return db
}

func setupTestCfgCache(t *testing.T, db *gorm.DB, overrides map[string]string) (*config.ConfigCache, *miniredis.Miniredis) {
	t.Helper()
	mr, err := miniredis.Run()
	require.NoError(t, err)
	t.Cleanup(mr.Close)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	defaults := map[string]string{
		CfgKeyTokenPrefix:          "pat_",
		CfgKeyTokenLength:          "40",
		CfgKeyTokenMaxPerTenant:    "10",
		CfgKeyTokenDefaultTTLDays:  "365",
		CfgKeyScopeAvailable:       "card.read,card.write,order.read,order.write,agent.read,agent.write,webhook.read,webhook.write",
		CfgKeyWebhookTimeout:       "10",
		CfgKeyWebhookMaxRetry:      "3",
		CfgKeyWebhookFailThreshold: "10",
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
			ConfigGroup: "openapi",
		}).Error)
	}
	return config.NewConfigCache(db, rdb), mr
}

func setupTestCrypto(t *testing.T) *crypto.Manager {
	t.Helper()
	mgr, err := crypto.NewManager("0123456789abcdef0123456789abcdef", "", "")
	require.NoError(t, err)
	return mgr
}

// ============== 1. 哈希 + 签名 ==============

func TestHashToken_Deterministic(t *testing.T) {
	token := "pat_abcdef1234567890"
	h1 := hashToken(token)
	h2 := hashToken(token)
	assert.Equal(t, h1, h2, "same token should produce same hash")
	// SHA-512 hex = 128 字符
	assert.Len(t, h1, 128, "SHA-512 hex length should be 128")
	// 不同 token 不同哈希
	assert.NotEqual(t, h1, hashToken("pat_different_token"))
}

func TestSignWebhook_EmptySecret(t *testing.T) {
	sig := signWebhook("", "evt-1", 1234567890, `{"foo":"bar"}`)
	assert.Empty(t, sig, "empty secret should produce empty signature")
}

func TestSignWebhook_Deterministic(t *testing.T) {
	secret := "test-secret"
	sig1 := signWebhook(secret, "evt-1", 1234567890, `{"foo":"bar"}`)
	sig2 := signWebhook(secret, "evt-1", 1234567890, `{"foo":"bar"}`)
	assert.Equal(t, sig1, sig2, "same inputs should produce same signature")
	assert.NotEmpty(t, sig1)
	// SHA-512/256 hex = 64 字符
	assert.Len(t, sig1, 64, "SHA-512/256 hex length should be 64")
}

func TestVerifyWebhookSignature(t *testing.T) {
	secret := "test-secret"
	eventID := "evt-12345"
	timestamp := int64(1700000000)
	payload := `{"order_id":42,"amount":99.5}`
	sig := signWebhook(secret, eventID, timestamp, payload)

	tests := []struct {
		name      string
		secret    string
		eventID   string
		timestamp int64
		payload   string
		sig       string
		want      bool
	}{
		{"valid", secret, eventID, timestamp, payload, sig, true},
		{"wrong secret", "other-secret", eventID, timestamp, payload, sig, false},
		{"wrong event_id", secret, "other-evt", timestamp, payload, sig, false},
		{"wrong timestamp", secret, eventID, timestamp + 1, payload, sig, false},
		{"wrong payload", secret, eventID, timestamp, `{"other":true}`, sig, false},
		{"empty secret", "", eventID, timestamp, payload, sig, false},
		{"empty sig", secret, eventID, timestamp, payload, "", false},
		{"tampered sig", secret, eventID, timestamp, payload, sig[:len(sig)-1] + "a", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := VerifyWebhookSignature(tt.secret, tt.eventID, tt.timestamp, tt.payload, tt.sig)
			assert.Equal(t, tt.want, got)
		})
	}
}

// ============== 2. generateRandomString ==============

func TestGenerateRandomString(t *testing.T) {
	s1, err := generateRandomString(32)
	require.NoError(t, err)
	assert.Len(t, s1, 32, "length should match request")

	s2, err := generateRandomString(32)
	require.NoError(t, err)
	assert.NotEqual(t, s1, s2, "two random strings should differ")

	// 字符集校验
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	for _, c := range s1 {
		assert.Contains(t, charset, string(c), "char should be in charset")
	}

	// 零长度
	_, err = generateRandomString(0)
	assert.Error(t, err)
}

func TestGenerateRandomString_Concurrent(t *testing.T) {
	// 100 并发生成，验证无重复
	const n = 100
	var wg sync.WaitGroup
	results := make(chan string, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s, _ := generateRandomString(20)
			results <- s
		}()
	}
	wg.Wait()
	close(results)
	seen := make(map[string]bool)
	for s := range results {
		assert.False(t, seen[s], "concurrent generation should not produce duplicates")
		seen[s] = true
	}
}

// ============== 3. generateUUID ==============

func TestGenerateUUID(t *testing.T) {
	u1 := generateUUID()
	u2 := generateUUID()
	assert.NotEqual(t, u1, u2, "two UUIDs should differ")
	// UUID v4 格式：8-4-4-4-12
	assert.Len(t, u1, 36, "UUID v4 length should be 36")
	parts := strings.Split(u1, "-")
	assert.Len(t, parts, 5)
	assert.Len(t, parts[0], 8)
	assert.Len(t, parts[1], 4)
	assert.Len(t, parts[2], 4)
	assert.Len(t, parts[3], 4)
	assert.Len(t, parts[4], 12)
	// version 4
	assert.Equal(t, "4", string(parts[2][0]), "UUID should be version 4")
}

// ============== 4. ValidateScopes ==============

func TestValidateScopes(t *testing.T) {
	available := "card.read,card.write,order.read,order.write,agent.read"

	tests := []struct {
		name      string
		scopes    string
		available string
		wantErr   bool
	}{
		{"empty scopes", "", available, false},
		{"single valid", "card.read", available, false},
		{"multiple valid", "card.read,order.write", available, false},
		{"with spaces", "card.read, order.write", available, false},
		{"invalid scope", "card.read,invalid.scope", available, true},
		{"empty available", "card.read", "", true},
		{"empty both", "", "", false},
		{"trailing comma", "card.read,", available, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateScopes(tt.scopes, tt.available)
			if tt.wantErr {
				assert.Error(t, err)
				assert.ErrorIs(t, err, ErrInvalidScope)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// ============== 5. HasScope ==============

func TestHasScope(t *testing.T) {
	tests := []struct {
		name     string
		scopes   string
		required string
		want     bool
	}{
		{"single match", "card.read", "card.read", true},
		{"multiple match first", "card.read,order.read", "card.read", true},
		{"multiple match last", "card.read,order.read", "order.read", true},
		{"multiple match middle", "card.read,order.read,agent.read", "order.read", true},
		{"no match", "card.read", "order.read", false},
		{"empty scopes", "", "card.read", false},
		{"empty required", "card.read", "", false},
		{"with spaces", "card.read, order.read", "order.read", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, HasScope(tt.scopes, tt.required))
		})
	}
}

// ============== 6. ParseScopes ==============

func TestParseScopes(t *testing.T) {
	tests := []struct {
		name   string
		scopes string
		want   []string
	}{
		{"empty", "", nil},
		{"single", "card.read", []string{"card.read"}},
		{"multiple", "card.read,order.write", []string{"card.read", "order.write"}},
		{"with spaces", "card.read, order.write", []string{"card.read", "order.write"}},
		{"trailing comma", "card.read,", []string{"card.read"}},
		{"all empty", ",,,", []string{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseScopes(tt.scopes)
			if tt.want == nil {
				assert.Nil(t, got)
			} else {
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

// ============== 7. isSubscribed ==============

func TestIsSubscribed(t *testing.T) {
	tests := []struct {
		name      string
		events    string
		eventType string
		want      bool
	}{
		{"empty events", "", "order.paid", false},
		{"match", "order.paid,card.generated", "order.paid", true},
		{"no match", "order.paid,card.generated", "agent.registered", false},
		{"empty event type", "order.paid,card.generated", "", false},
		{"single match", "order.paid", "order.paid", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, isSubscribed(tt.events, tt.eventType))
		})
	}
}

// ============== 8. truncate ==============

func TestTruncate(t *testing.T) {
	assert.Equal(t, "abc", truncate("abc", 10), "short string unchanged")
	assert.Equal(t, "hello", truncate("hello world", 5), "long string truncated")
	assert.Equal(t, "hello", truncate("hello", 5), "exactly max len unchanged")
	assert.Equal(t, "", truncate("", 10), "empty string unchanged")
}

// ============== 9. TokenManager.GenerateToken ==============

func TestTokenManager_GenerateToken_Success(t *testing.T) {
	db := setupTestDB(t)
	cache, _ := setupTestCfgCache(t, db, nil)
	mgr := NewTokenManager(db, cache)
	ctx := context.Background()

	plain, token, err := mgr.GenerateToken(ctx, 1001, "测试 Token", "card.read,order.read", 30)
	require.NoError(t, err)
	assert.NotEmpty(t, plain, "should return plaintext")
	assert.True(t, strings.HasPrefix(plain, "pat_"), "should have prefix pat_")
	assert.Equal(t, uint64(1001), token.TenantID)
	assert.Equal(t, "测试 Token", token.Name)
	assert.Equal(t, "card.read,order.read", token.Scopes)
	assert.Equal(t, TokenStatusActive, token.Status)
	assert.NotNil(t, token.ExpiresAt, "TTL > 0 should set expires_at")
	assert.Equal(t, plain[:8], token.Prefix, "prefix should be first 8 chars")
	// hash 存库
	var dbToken model.DeveloperAPIToken
	require.NoError(t, db.Where("id = ?", token.ID).First(&dbToken).Error)
	assert.NotEqual(t, plain, dbToken.TokenHash, "should not store plaintext")
	assert.Len(t, dbToken.TokenHash, 128, "SHA-512 hex = 128 chars")
}

func TestTokenManager_GenerateToken_InvalidScope(t *testing.T) {
	db := setupTestDB(t)
	cache, _ := setupTestCfgCache(t, db, nil)
	mgr := NewTokenManager(db, cache)

	_, _, err := mgr.GenerateToken(context.Background(), 1001, "test", "invalid.scope", 30)
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrInvalidScope)
}

func TestTokenManager_GenerateToken_LimitExceeded(t *testing.T) {
	db := setupTestDB(t)
	cache, _ := setupTestCfgCache(t, db, map[string]string{
		CfgKeyTokenMaxPerTenant: "2",
	})
	mgr := NewTokenManager(db, cache)

	_, _, err := mgr.GenerateToken(context.Background(), 1001, "t1", "", 30)
	require.NoError(t, err)
	_, _, err = mgr.GenerateToken(context.Background(), 1001, "t2", "", 30)
	require.NoError(t, err)
	_, _, err = mgr.GenerateToken(context.Background(), 1001, "t3", "", 30)
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrTokenLimitExceeded)
}

func TestTokenManager_GenerateToken_LimitZero_Unlimited(t *testing.T) {
	db := setupTestDB(t)
	cache, _ := setupTestCfgCache(t, db, map[string]string{
		CfgKeyTokenMaxPerTenant: "0",
	})
	mgr := NewTokenManager(db, cache)

	for i := 0; i < 20; i++ {
		_, _, err := mgr.GenerateToken(context.Background(), 1001, fmt.Sprintf("t%d", i), "", 30)
		require.NoError(t, err)
	}
}

func TestTokenManager_GenerateToken_TTLZero_NeverExpire(t *testing.T) {
	db := setupTestDB(t)
	cache, _ := setupTestCfgCache(t, db, map[string]string{
		CfgKeyTokenDefaultTTLDays: "0",
	})
	mgr := NewTokenManager(db, cache)

	plain, token, err := mgr.GenerateToken(context.Background(), 1001, "永久", "", 0)
	require.NoError(t, err)
	assert.Nil(t, token.ExpiresAt, "TTL=0 should never expire")
	assert.NotEmpty(t, plain)
}

func TestTokenManager_GenerateToken_DifferentTenants_NotShared(t *testing.T) {
	db := setupTestDB(t)
	cache, _ := setupTestCfgCache(t, db, map[string]string{
		CfgKeyTokenMaxPerTenant: "2",
	})
	mgr := NewTokenManager(db, cache)

	_, _, err := mgr.GenerateToken(context.Background(), 1001, "t1", "", 30)
	require.NoError(t, err)
	_, _, err = mgr.GenerateToken(context.Background(), 1001, "t2", "", 30)
	require.NoError(t, err)
	// 租户 1002 不受 1001 的限制影响
	_, _, err = mgr.GenerateToken(context.Background(), 1002, "t1", "", 30)
	require.NoError(t, err)
}

// ============== 10. TokenManager.ValidateToken ==============

func TestTokenManager_ValidateToken_Success(t *testing.T) {
	db := setupTestDB(t)
	cache, _ := setupTestCfgCache(t, db, nil)
	mgr := NewTokenManager(db, cache)
	ctx := context.Background()

	plain, _, err := mgr.GenerateToken(ctx, 1001, "test", "card.read", 30)
	require.NoError(t, err)

	// 等待异步更新完成
	time.Sleep(100 * time.Millisecond)

	token, err := mgr.ValidateToken(ctx, plain, "127.0.0.1")
	require.NoError(t, err)
	assert.Equal(t, uint64(1001), token.TenantID)
	assert.Equal(t, "card.read", token.Scopes)

	// 等待 last_used 异步更新
	time.Sleep(100 * time.Millisecond)
	var dbToken model.DeveloperAPIToken
	require.NoError(t, db.Where("id = ?", token.ID).First(&dbToken).Error)
	assert.NotNil(t, dbToken.LastUsedAt, "last_used_at should be updated")
	assert.Equal(t, "127.0.0.1", dbToken.LastUsedIP)
}

func TestTokenManager_ValidateToken_NotFound(t *testing.T) {
	db := setupTestDB(t)
	cache, _ := setupTestCfgCache(t, db, nil)
	mgr := NewTokenManager(db, cache)

	_, err := mgr.ValidateToken(context.Background(), "pat_nonexistent_token_xxxxxxxxxxxxxxxxx", "1.1.1.1")
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrTokenNotFound)
}

func TestTokenManager_ValidateToken_Empty(t *testing.T) {
	db := setupTestDB(t)
	cache, _ := setupTestCfgCache(t, db, nil)
	mgr := NewTokenManager(db, cache)

	_, err := mgr.ValidateToken(context.Background(), "", "1.1.1.1")
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrTokenNotFound)
}

func TestTokenManager_ValidateToken_Revoked(t *testing.T) {
	db := setupTestDB(t)
	cache, _ := setupTestCfgCache(t, db, nil)
	mgr := NewTokenManager(db, cache)
	ctx := context.Background()

	plain, token, err := mgr.GenerateToken(ctx, 1001, "test", "card.read", 30)
	require.NoError(t, err)

	require.NoError(t, mgr.RevokeToken(ctx, 1001, token.ID))

	_, err = mgr.ValidateToken(ctx, plain, "1.1.1.1")
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrTokenRevoked)
}

func TestTokenManager_ValidateToken_Expired(t *testing.T) {
	db := setupTestDB(t)
	cache, _ := setupTestCfgCache(t, db, nil)
	mgr := NewTokenManager(db, cache)
	ctx := context.Background()

	// 生成一个已过期的 Token
	plain, _, err := mgr.GenerateToken(ctx, 1001, "test", "card.read", 30)
	require.NoError(t, err)

	// 直接更新 expires_at 为过去
	pastTime := time.Now().Add(-1 * time.Hour)
	require.NoError(t, db.Model(&model.DeveloperAPIToken{}).
		Where("token_hash = ?", hashToken(plain)).
		Update("expires_at", pastTime).Error)

	_, err = mgr.ValidateToken(ctx, plain, "1.1.1.1")
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrTokenExpired)
}

// ============== 11. TokenManager.RevokeToken ==============

func TestTokenManager_RevokeToken_Success(t *testing.T) {
	db := setupTestDB(t)
	cache, _ := setupTestCfgCache(t, db, nil)
	mgr := NewTokenManager(db, cache)
	ctx := context.Background()

	_, token, err := mgr.GenerateToken(ctx, 1001, "test", "", 30)
	require.NoError(t, err)

	require.NoError(t, mgr.RevokeToken(ctx, 1001, token.ID))

	var dbToken model.DeveloperAPIToken
	require.NoError(t, db.Where("id = ?", token.ID).First(&dbToken).Error)
	assert.Equal(t, TokenStatusRevoked, dbToken.Status)
	assert.NotNil(t, dbToken.RevokedAt)
}

func TestTokenManager_RevokeToken_NotFound(t *testing.T) {
	db := setupTestDB(t)
	cache, _ := setupTestCfgCache(t, db, nil)
	mgr := NewTokenManager(db, cache)

	err := mgr.RevokeToken(context.Background(), 1001, 99999)
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrTokenNotFound)
}

func TestTokenManager_RevokeToken_WrongTenant(t *testing.T) {
	db := setupTestDB(t)
	cache, _ := setupTestCfgCache(t, db, nil)
	mgr := NewTokenManager(db, cache)
	ctx := context.Background()

	_, token, err := mgr.GenerateToken(ctx, 1001, "test", "", 30)
	require.NoError(t, err)

	// 用其他租户 ID 撤销应该失败
	err = mgr.RevokeToken(ctx, 1002, token.ID)
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrTokenNotFound)
}

func TestTokenManager_RevokeToken_AlreadyRevoked(t *testing.T) {
	db := setupTestDB(t)
	cache, _ := setupTestCfgCache(t, db, nil)
	mgr := NewTokenManager(db, cache)
	ctx := context.Background()

	_, token, err := mgr.GenerateToken(ctx, 1001, "test", "", 30)
	require.NoError(t, err)

	require.NoError(t, mgr.RevokeToken(ctx, 1001, token.ID))
	// 再次撤销应失败
	err = mgr.RevokeToken(ctx, 1001, token.ID)
	assert.Error(t, err)
}

// ============== 12. TokenManager.ListTokens / GetToken ==============

func TestTokenManager_ListTokens(t *testing.T) {
	db := setupTestDB(t)
	cache, _ := setupTestCfgCache(t, db, nil)
	mgr := NewTokenManager(db, cache)
	ctx := context.Background()

	// 创建 5 个 token
	for i := 0; i < 5; i++ {
		_, _, err := mgr.GenerateToken(ctx, 1001, fmt.Sprintf("t%d", i), "", 30)
		require.NoError(t, err)
	}
	// 撤销一个
	_, token, _ := mgr.GenerateToken(ctx, 1001, "revoked", "", 30)
	require.NoError(t, mgr.RevokeToken(ctx, 1001, token.ID))

	// 全部
	items, total, err := mgr.ListTokens(ctx, 1001, "", 1, 100)
	require.NoError(t, err)
	assert.Equal(t, int64(6), total)
	assert.Len(t, items, 6)

	// 仅 active
	items, total, err = mgr.ListTokens(ctx, 1001, TokenStatusActive, 1, 100)
	require.NoError(t, err)
	assert.Equal(t, int64(5), total)
	assert.Len(t, items, 5)
	for _, it := range items {
		assert.Equal(t, TokenStatusActive, it.Status)
	}

	// 分页
	items, total, err = mgr.ListTokens(ctx, 1001, "", 1, 2)
	require.NoError(t, err)
	assert.Equal(t, int64(6), total)
	assert.Len(t, items, 2)

	items, total, err = mgr.ListTokens(ctx, 1001, "", 3, 2)
	require.NoError(t, err)
	assert.Equal(t, int64(6), total)
	assert.Len(t, items, 2)
}

func TestTokenManager_ListTokens_TenantIsolation(t *testing.T) {
	db := setupTestDB(t)
	cache, _ := setupTestCfgCache(t, db, nil)
	mgr := NewTokenManager(db, cache)
	ctx := context.Background()

	_, _, err := mgr.GenerateToken(ctx, 1001, "t1", "", 30)
	require.NoError(t, err)
	_, _, err = mgr.GenerateToken(ctx, 1002, "t2", "", 30)
	require.NoError(t, err)

	items1001, total1001, err := mgr.ListTokens(ctx, 1001, "", 1, 100)
	require.NoError(t, err)
	assert.Equal(t, int64(1), total1001)
	assert.Len(t, items1001, 1)
	assert.Equal(t, uint64(1001), items1001[0].TenantID)

	items1002, total1002, err := mgr.ListTokens(ctx, 1002, "", 1, 100)
	require.NoError(t, err)
	assert.Equal(t, int64(1), total1002)
	assert.Equal(t, uint64(1002), items1002[0].TenantID)
}

func TestTokenManager_GetToken(t *testing.T) {
	db := setupTestDB(t)
	cache, _ := setupTestCfgCache(t, db, nil)
	mgr := NewTokenManager(db, cache)
	ctx := context.Background()

	_, token, err := mgr.GenerateToken(ctx, 1001, "test", "card.read", 30)
	require.NoError(t, err)

	got, err := mgr.GetToken(ctx, 1001, token.ID)
	require.NoError(t, err)
	assert.Equal(t, token.ID, got.ID)
	assert.Equal(t, "test", got.Name)

	// 错误租户
	_, err = mgr.GetToken(ctx, 1002, token.ID)
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrTokenNotFound)

	// 不存在
	_, err = mgr.GetToken(ctx, 1001, 99999)
	assert.Error(t, err)
}

// ============== 13. WebhookManager.CreateEndpoint ==============

func TestWebhookManager_CreateEndpoint_Success(t *testing.T) {
	db := setupTestDB(t)
	cache, _ := setupTestCfgCache(t, db, nil)
	cry := setupTestCrypto(t)
	mgr := NewWebhookManager(db, cache, cry)

	ep := &model.WebhookEndpoint{
		TenantID: 1001,
		Name:     "test endpoint",
		URL:      "https://example.com/webhook",
		Events:   "order.paid,card.generated",
	}
	err := mgr.CreateEndpoint(context.Background(), ep, "my-secret")
	require.NoError(t, err)
	assert.NotZero(t, ep.ID)
	assert.Equal(t, EndpointStatusActive, ep.Status)
	assert.NotEmpty(t, ep.SecretEnc, "secret should be encrypted stored")
	// 解密验证
	dec, err := cry.DecryptAES(ep.SecretEnc)
	require.NoError(t, err)
	assert.Equal(t, "my-secret", dec)
}

func TestWebhookManager_CreateEndpoint_InvalidURL(t *testing.T) {
	db := setupTestDB(t)
	cache, _ := setupTestCfgCache(t, db, nil)
	cry := setupTestCrypto(t)
	mgr := NewWebhookManager(db, cache, cry)

	ep := &model.WebhookEndpoint{
		TenantID: 1001,
		Name:     "test",
		URL:      "ftp://invalid.com",
		Events:   "order.paid",
	}
	err := mgr.CreateEndpoint(context.Background(), ep, "")
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrInvalidURL)
}

func TestWebhookManager_CreateEndpoint_HTTPAllowed(t *testing.T) {
	db := setupTestDB(t)
	cache, _ := setupTestCfgCache(t, db, nil)
	cry := setupTestCrypto(t)
	mgr := NewWebhookManager(db, cache, cry)

	// http:// 也允许（开发环境）
	ep := &model.WebhookEndpoint{
		TenantID: 1001,
		Name:     "dev endpoint",
		URL:      "http://localhost:9000/webhook",
		Events:   "order.paid",
	}
	err := mgr.CreateEndpoint(context.Background(), ep, "")
	require.NoError(t, err)
	assert.NotZero(t, ep.ID)
}

// ============== 14. WebhookManager CRUD ==============

func TestWebhookManager_UpdateEndpoint(t *testing.T) {
	db := setupTestDB(t)
	cache, _ := setupTestCfgCache(t, db, nil)
	cry := setupTestCrypto(t)
	mgr := NewWebhookManager(db, cache, cry)
	ctx := context.Background()

	ep := &model.WebhookEndpoint{
		TenantID: 1001,
		Name:     "ep1",
		URL:      "https://example.com/wh",
		Events:   "order.paid",
	}
	require.NoError(t, mgr.CreateEndpoint(ctx, ep, "secret1"))

	err := mgr.UpdateEndpoint(ctx, ep.ID, 1001, map[string]interface{}{
		"name":   "ep1-updated",
		"events": "order.paid,card.generated",
	}, "secret2")
	require.NoError(t, err)

	got, err := mgr.GetEndpoint(ctx, ep.ID, 1001)
	require.NoError(t, err)
	assert.Equal(t, "ep1-updated", got.Name)
	assert.Equal(t, "order.paid,card.generated", got.Events)
	// secret 也应被更新
	dec, err := cry.DecryptAES(got.SecretEnc)
	require.NoError(t, err)
	assert.Equal(t, "secret2", dec)
}

func TestWebhookManager_UpdateEndpoint_InvalidURL(t *testing.T) {
	db := setupTestDB(t)
	cache, _ := setupTestCfgCache(t, db, nil)
	cry := setupTestCrypto(t)
	mgr := NewWebhookManager(db, cache, cry)
	ctx := context.Background()

	ep := &model.WebhookEndpoint{
		TenantID: 1001,
		Name:     "ep1",
		URL:      "https://example.com/wh",
		Events:   "order.paid",
	}
	require.NoError(t, mgr.CreateEndpoint(ctx, ep, ""))

	err := mgr.UpdateEndpoint(ctx, ep.ID, 1001, map[string]interface{}{
		"url": "not-a-url",
	}, "")
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrInvalidURL)
}

func TestWebhookManager_UpdateEndpoint_NotFound(t *testing.T) {
	db := setupTestDB(t)
	cache, _ := setupTestCfgCache(t, db, nil)
	cry := setupTestCrypto(t)
	mgr := NewWebhookManager(db, cache, cry)

	err := mgr.UpdateEndpoint(context.Background(), 99999, 1001, map[string]interface{}{
		"name": "x",
	}, "")
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrEndpointNotFound)
}

func TestWebhookManager_DeleteEndpoint(t *testing.T) {
	db := setupTestDB(t)
	cache, _ := setupTestCfgCache(t, db, nil)
	cry := setupTestCrypto(t)
	mgr := NewWebhookManager(db, cache, cry)
	ctx := context.Background()

	ep := &model.WebhookEndpoint{
		TenantID: 1001,
		Name:     "ep1",
		URL:      "https://example.com/wh",
		Events:   "order.paid",
	}
	require.NoError(t, mgr.CreateEndpoint(ctx, ep, ""))

	require.NoError(t, mgr.DeleteEndpoint(ctx, ep.ID, 1001))

	_, err := mgr.GetEndpoint(ctx, ep.ID, 1001)
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrEndpointNotFound)
}

func TestWebhookManager_DeleteEndpoint_NotFound(t *testing.T) {
	db := setupTestDB(t)
	cache, _ := setupTestCfgCache(t, db, nil)
	cry := setupTestCrypto(t)
	mgr := NewWebhookManager(db, cache, cry)

	err := mgr.DeleteEndpoint(context.Background(), 99999, 1001)
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrEndpointNotFound)
}

func TestWebhookManager_ListEndpoints(t *testing.T) {
	db := setupTestDB(t)
	cache, _ := setupTestCfgCache(t, db, nil)
	cry := setupTestCrypto(t)
	mgr := NewWebhookManager(db, cache, cry)
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		ep := &model.WebhookEndpoint{
			TenantID: 1001,
			Name:     fmt.Sprintf("ep%d", i),
			URL:      fmt.Sprintf("https://example.com/wh%d", i),
			Events:   "order.paid",
		}
		require.NoError(t, mgr.CreateEndpoint(ctx, ep, ""))
	}

	items, total, err := mgr.ListEndpoints(ctx, 1001, "", 1, 100)
	require.NoError(t, err)
	assert.Equal(t, int64(3), total)
	assert.Len(t, items, 3)

	// 过滤 active
	items, total, err = mgr.ListEndpoints(ctx, 1001, EndpointStatusActive, 1, 100)
	require.NoError(t, err)
	assert.Equal(t, int64(3), total)

	// tenant 隔离
	items, total, err = mgr.ListEndpoints(ctx, 1002, "", 1, 100)
	require.NoError(t, err)
	assert.Equal(t, int64(0), total)
}

// ============== 15. WebhookManager.DispatchEvent ==============

func TestWebhookManager_DispatchEvent_NoSubscribers(t *testing.T) {
	db := setupTestDB(t)
	cache, _ := setupTestCfgCache(t, db, nil)
	cry := setupTestCrypto(t)
	mgr := NewWebhookManager(db, cache, cry)
	ctx := context.Background()

	// 创建不订阅的端点
	ep := &model.WebhookEndpoint{
		TenantID: 1001,
		Name:     "ep1",
		URL:      "https://example.com/wh",
		Events:   "card.generated",
	}
	require.NoError(t, mgr.CreateEndpoint(ctx, ep, ""))

	deliveries, err := mgr.DispatchEvent(ctx, 1001, EventOrderPaid, map[string]interface{}{"order_id": 1})
	require.NoError(t, err)
	assert.Empty(t, deliveries, "no matching subscribers should produce no deliveries")
}

func TestWebhookManager_DispatchEvent_Success(t *testing.T) {
	db := setupTestDB(t)
	cache, _ := setupTestCfgCache(t, db, nil)
	cry := setupTestCrypto(t)
	mgr := NewWebhookManager(db, cache, cry)
	ctx := context.Background()

	// 起一个 mock HTTP server
	var receivedReq *http.Request
	var receivedBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedReq = r
		buf := make([]byte, 1024)
		n, _ := r.Body.Read(buf)
		receivedBody = string(buf[:n])
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	ep := &model.WebhookEndpoint{
		TenantID: 1001,
		Name:     "ep1",
		URL:      srv.URL,
		Events:   "order.paid",
	}
	require.NoError(t, mgr.CreateEndpoint(ctx, ep, "test-secret"))

	deliveries, err := mgr.DispatchEvent(ctx, 1001, EventOrderPaid, map[string]interface{}{
		"order_id": 42,
		"amount":   99.5,
	})
	require.NoError(t, err)
	assert.Len(t, deliveries, 1)

	// 验证 server 收到请求
	require.NotNil(t, receivedReq)
	assert.Equal(t, "POST", receivedReq.Method)
	assert.Equal(t, "application/json", receivedReq.Header.Get("Content-Type"))
	assert.Equal(t, EventOrderPaid, receivedReq.Header.Get("X-KeyAuth-Event"))
	assert.NotEmpty(t, receivedReq.Header.Get("X-KeyAuth-Event-ID"))
	assert.NotEmpty(t, receivedReq.Header.Get("X-KeyAuth-Signature"))
	assert.Contains(t, receivedBody, `"order_id":42`)
	assert.Contains(t, receivedBody, `"amount":99.5`)

	// 验证 delivery 状态
	var d model.WebhookDelivery
	require.NoError(t, db.Where("id = ?", deliveries[0].ID).First(&d).Error)
	assert.Equal(t, DeliveryStatusSuccess, d.Status)
	assert.Equal(t, 200, d.ResponseCode)
	assert.NotNil(t, d.DeliveredAt)

	// 验证端点失败计数清零
	var ep2 model.WebhookEndpoint
	require.NoError(t, db.Where("id = ?", ep.ID).First(&ep2).Error)
	assert.Equal(t, 0, ep2.FailureCount)
	assert.Equal(t, 200, ep2.LastResponseCode)
}

func TestWebhookManager_DispatchEvent_Failure(t *testing.T) {
	db := setupTestDB(t)
	cache, _ := setupTestCfgCache(t, db, nil)
	cry := setupTestCrypto(t)
	mgr := NewWebhookManager(db, cache, cry)
	ctx := context.Background()

	// 起一个返回 500 的 server
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"err":"server error"}`))
	}))
	defer srv.Close()

	ep := &model.WebhookEndpoint{
		TenantID: 1001,
		Name:     "ep1",
		URL:      srv.URL,
		Events:   "order.paid",
	}
	require.NoError(t, mgr.CreateEndpoint(ctx, ep, ""))

	deliveries, err := mgr.DispatchEvent(ctx, 1001, EventOrderPaid, map[string]interface{}{"order_id": 1})
	require.NoError(t, err)
	assert.Len(t, deliveries, 1)

	// 验证 delivery 失败
	var d model.WebhookDelivery
	require.NoError(t, db.Where("id = ?", deliveries[0].ID).First(&d).Error)
	assert.Equal(t, DeliveryStatusFailed, d.Status)
	assert.Equal(t, 500, d.ResponseCode)
	assert.NotNil(t, d.NextRetryAt, "should schedule retry")
	assert.Equal(t, 1, d.AttemptCount)

	// 端点失败计数 +1
	var ep2 model.WebhookEndpoint
	require.NoError(t, db.Where("id = ?", ep.ID).First(&ep2).Error)
	assert.Equal(t, 1, ep2.FailureCount)
	assert.Equal(t, 500, ep2.LastResponseCode)
}

func TestWebhookManager_DispatchEvent_Unreachable(t *testing.T) {
	db := setupTestDB(t)
	cache, _ := setupTestCfgCache(t, db, map[string]string{
		CfgKeyWebhookTimeout: "1", // 1 秒超时
	})
	cry := setupTestCrypto(t)
	mgr := NewWebhookManager(db, cache, cry)
	ctx := context.Background()

	// 用一个肯定连不上的地址（0.0.0.0:1 不可路由）
	ep := &model.WebhookEndpoint{
		TenantID: 1001,
		Name:     "ep1",
		URL:      "http://127.0.0.1:1/wh", // 端口 1 通常无服务
		Events:   "order.paid",
	}
	require.NoError(t, mgr.CreateEndpoint(ctx, ep, ""))

	deliveries, err := mgr.DispatchEvent(ctx, 1001, EventOrderPaid, map[string]interface{}{"order_id": 1})
	require.NoError(t, err)
	assert.Len(t, deliveries, 1)

	var d model.WebhookDelivery
	require.NoError(t, db.Where("id = ?", deliveries[0].ID).First(&d).Error)
	assert.Equal(t, DeliveryStatusFailed, d.Status)
	assert.Equal(t, 0, d.ResponseCode)
	assert.NotEmpty(t, d.ResponseBody)
}

func TestWebhookManager_DispatchEvent_AutoDisable(t *testing.T) {
	db := setupTestDB(t)
	cache, _ := setupTestCfgCache(t, db, map[string]string{
		CfgKeyWebhookFailThreshold: "3", // 3 次失败就 disable
	})
	cry := setupTestCrypto(t)
	mgr := NewWebhookManager(db, cache, cry)
	ctx := context.Background()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	ep := &model.WebhookEndpoint{
		TenantID: 1001,
		Name:     "ep1",
		URL:      srv.URL,
		Events:   "order.paid",
	}
	require.NoError(t, mgr.CreateEndpoint(ctx, ep, ""))

	// 连续 3 次失败
	for i := 0; i < 3; i++ {
		_, err := mgr.DispatchEvent(ctx, 1001, EventOrderPaid, map[string]interface{}{"i": i})
		require.NoError(t, err)
	}

	// 端点应被自动 disable
	var ep2 model.WebhookEndpoint
	require.NoError(t, db.Where("id = ?", ep.ID).First(&ep2).Error)
	assert.Equal(t, EndpointStatusDisabled, ep2.Status)
	assert.Equal(t, 3, ep2.FailureCount)

	// 再次 dispatch 应不命中（端点 disabled 不在 active 查询范围内）
	deliveries, err := mgr.DispatchEvent(ctx, 1001, EventOrderPaid, map[string]interface{}{"i": 99})
	require.NoError(t, err)
	assert.Empty(t, deliveries)
}

func TestWebhookManager_DispatchEvent_MultipleEndpoints(t *testing.T) {
	db := setupTestDB(t)
	cache, _ := setupTestCfgCache(t, db, nil)
	cry := setupTestCrypto(t)
	mgr := NewWebhookManager(db, cache, cry)
	ctx := context.Background()

	// 3 个端点，2 个订阅 order.paid，1 个订阅其他
	srv1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) }))
	defer srv1.Close()
	srv2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) }))
	defer srv2.Close()
	srv3 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) }))
	defer srv3.Close()

	for i, srv := range []*httptest.Server{srv1, srv2, srv3} {
		events := "order.paid"
		if i == 2 {
			events = "card.generated"
		}
		ep := &model.WebhookEndpoint{
			TenantID: 1001,
			Name:     fmt.Sprintf("ep%d", i),
			URL:      srv.URL,
			Events:   events,
		}
		require.NoError(t, mgr.CreateEndpoint(ctx, ep, ""))
	}

	deliveries, err := mgr.DispatchEvent(ctx, 1001, EventOrderPaid, map[string]interface{}{"order_id": 1})
	require.NoError(t, err)
	assert.Len(t, deliveries, 2, "2 endpoints subscribed to order.paid")
}

// ============== 16. WebhookManager.RetryDelivery ==============

func TestWebhookManager_RetryDelivery_Success(t *testing.T) {
	db := setupTestDB(t)
	cache, _ := setupTestCfgCache(t, db, nil)
	cry := setupTestCrypto(t)
	mgr := NewWebhookManager(db, cache, cry)
	ctx := context.Background()

	// 第一次：失败
	failCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		failCount++
		if failCount == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	ep := &model.WebhookEndpoint{
		TenantID: 1001,
		Name:     "ep1",
		URL:      srv.URL,
		Events:   "order.paid",
	}
	require.NoError(t, mgr.CreateEndpoint(ctx, ep, ""))

	deliveries, err := mgr.DispatchEvent(ctx, 1001, EventOrderPaid, map[string]interface{}{"order_id": 1})
	require.NoError(t, err)
	require.Len(t, deliveries, 1)
	assert.Equal(t, DeliveryStatusFailed, deliveries[0].Status)

	// 重试
	retried, err := mgr.RetryDelivery(ctx, deliveries[0].ID, 1001)
	require.NoError(t, err)
	assert.Equal(t, DeliveryStatusSuccess, retried.Status)
}

func TestWebhookManager_RetryDelivery_AlreadySuccess(t *testing.T) {
	db := setupTestDB(t)
	cache, _ := setupTestCfgCache(t, db, nil)
	cry := setupTestCrypto(t)
	mgr := NewWebhookManager(db, cache, cry)
	ctx := context.Background()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) }))
	defer srv.Close()

	ep := &model.WebhookEndpoint{
		TenantID: 1001,
		Name:     "ep1",
		URL:      srv.URL,
		Events:   "order.paid",
	}
	require.NoError(t, mgr.CreateEndpoint(ctx, ep, ""))

	deliveries, err := mgr.DispatchEvent(ctx, 1001, EventOrderPaid, map[string]interface{}{"order_id": 1})
	require.NoError(t, err)
	require.Len(t, deliveries, 1)
	require.Equal(t, DeliveryStatusSuccess, deliveries[0].Status)

	_, err = mgr.RetryDelivery(ctx, deliveries[0].ID, 1001)
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrDeliveryNotRetryable)
}

func TestWebhookManager_RetryDelivery_NotFound(t *testing.T) {
	db := setupTestDB(t)
	cache, _ := setupTestCfgCache(t, db, nil)
	cry := setupTestCrypto(t)
	mgr := NewWebhookManager(db, cache, cry)

	_, err := mgr.RetryDelivery(context.Background(), 99999, 1001)
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrDeliveryNotFound)
}

func TestWebhookManager_RetryDelivery_ExceedMaxRetry(t *testing.T) {
	db := setupTestDB(t)
	cache, _ := setupTestCfgCache(t, db, map[string]string{
		CfgKeyWebhookMaxRetry: "1", // 仅 1 次重试
	})
	cry := setupTestCrypto(t)
	mgr := NewWebhookManager(db, cache, cry)
	ctx := context.Background()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	ep := &model.WebhookEndpoint{
		TenantID: 1001,
		Name:     "ep1",
		URL:      srv.URL,
		Events:   "order.paid",
	}
	require.NoError(t, mgr.CreateEndpoint(ctx, ep, ""))

	deliveries, err := mgr.DispatchEvent(ctx, 1001, EventOrderPaid, map[string]interface{}{"order_id": 1})
	require.NoError(t, err)
	require.Len(t, deliveries, 1)
	assert.Equal(t, 1, deliveries[0].MaxRetry)
	assert.Equal(t, 1, deliveries[0].AttemptCount)

	// attempt_count >= max_retry 应失败
	_, err = mgr.RetryDelivery(ctx, deliveries[0].ID, 1001)
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrDeliveryNotRetryable)
}

func TestWebhookManager_RetryDelivery_DisabledEndpoint(t *testing.T) {
	db := setupTestDB(t)
	cache, _ := setupTestCfgCache(t, db, nil)
	cry := setupTestCrypto(t)
	mgr := NewWebhookManager(db, cache, cry)
	ctx := context.Background()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	ep := &model.WebhookEndpoint{
		TenantID: 1001,
		Name:     "ep1",
		URL:      srv.URL,
		Events:   "order.paid",
	}
	require.NoError(t, mgr.CreateEndpoint(ctx, ep, ""))

	deliveries, err := mgr.DispatchEvent(ctx, 1001, EventOrderPaid, map[string]interface{}{"order_id": 1})
	require.NoError(t, err)

	// 手动 disable 端点
	require.NoError(t, mgr.UpdateEndpoint(ctx, ep.ID, 1001, map[string]interface{}{
		"status": EndpointStatusDisabled,
	}, ""))

	_, err = mgr.RetryDelivery(ctx, deliveries[0].ID, 1001)
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrEndpointDisabled)
}

// ============== 17. WebhookManager.ListDeliveries / GetDelivery ==============

func TestWebhookManager_ListDeliveries(t *testing.T) {
	db := setupTestDB(t)
	cache, _ := setupTestCfgCache(t, db, nil)
	cry := setupTestCrypto(t)
	mgr := NewWebhookManager(db, cache, cry)
	ctx := context.Background()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) }))
	defer srv.Close()

	ep := &model.WebhookEndpoint{
		TenantID: 1001,
		Name:     "ep1",
		URL:      srv.URL,
		Events:   "order.paid,card.generated",
	}
	require.NoError(t, mgr.CreateEndpoint(ctx, ep, ""))

	// 触发 3 个事件
	_, err := mgr.DispatchEvent(ctx, 1001, EventOrderPaid, map[string]interface{}{"i": 1})
	require.NoError(t, err)
	_, err = mgr.DispatchEvent(ctx, 1001, EventOrderPaid, map[string]interface{}{"i": 2})
	require.NoError(t, err)
	_, err = mgr.DispatchEvent(ctx, 1001, EventCardGenerated, map[string]interface{}{"i": 3})
	require.NoError(t, err)

	// 全部
	items, total, err := mgr.ListDeliveries(ctx, 1001, 0, "", "", 1, 100)
	require.NoError(t, err)
	assert.Equal(t, int64(3), total)
	assert.Len(t, items, 3)

	// 按 endpoint
	items, total, err = mgr.ListDeliveries(ctx, 1001, ep.ID, "", "", 1, 100)
	require.NoError(t, err)
	assert.Equal(t, int64(3), total)

	// 按 event_type
	items, total, err = mgr.ListDeliveries(ctx, 1001, 0, "", EventOrderPaid, 1, 100)
	require.NoError(t, err)
	assert.Equal(t, int64(2), total)
	for _, d := range items {
		assert.Equal(t, EventOrderPaid, d.EventType)
	}

	// 按 status
	items, total, err = mgr.ListDeliveries(ctx, 1001, 0, DeliveryStatusSuccess, "", 1, 100)
	require.NoError(t, err)
	assert.Equal(t, int64(3), total)
}

func TestWebhookManager_GetDelivery(t *testing.T) {
	db := setupTestDB(t)
	cache, _ := setupTestCfgCache(t, db, nil)
	cry := setupTestCrypto(t)
	mgr := NewWebhookManager(db, cache, cry)
	ctx := context.Background()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) }))
	defer srv.Close()

	ep := &model.WebhookEndpoint{
		TenantID: 1001,
		Name:     "ep1",
		URL:      srv.URL,
		Events:   "order.paid",
	}
	require.NoError(t, mgr.CreateEndpoint(ctx, ep, ""))

	deliveries, err := mgr.DispatchEvent(ctx, 1001, EventOrderPaid, map[string]interface{}{"order_id": 42})
	require.NoError(t, err)
	require.Len(t, deliveries, 1)

	got, err := mgr.GetDelivery(ctx, deliveries[0].ID, 1001)
	require.NoError(t, err)
	assert.Equal(t, deliveries[0].ID, got.ID)
	assert.Equal(t, EventOrderPaid, got.EventType)
	assert.NotEmpty(t, got.EventID)
	assert.Len(t, got.EventID, 36, "event_id should be UUID v4")

	// 不存在
	_, err = mgr.GetDelivery(ctx, 99999, 1001)
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrDeliveryNotFound)

	// 错误租户
	_, err = mgr.GetDelivery(ctx, deliveries[0].ID, 1002)
	assert.Error(t, err)
}

// ============== 18. WebhookManager.ProcessPendingRetries ==============

func TestWebhookManager_ProcessPendingRetries(t *testing.T) {
	db := setupTestDB(t)
	cache, _ := setupTestCfgCache(t, db, nil)
	cry := setupTestCrypto(t)
	mgr := NewWebhookManager(db, cache, cry)
	ctx := context.Background()

	// 第 1 次失败
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount <= 1 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	ep := &model.WebhookEndpoint{
		TenantID: 1001,
		Name:     "ep1",
		URL:      srv.URL,
		Events:   "order.paid",
	}
	require.NoError(t, mgr.CreateEndpoint(ctx, ep, ""))

	deliveries, err := mgr.DispatchEvent(ctx, 1001, EventOrderPaid, map[string]interface{}{"i": 1})
	require.NoError(t, err)
	require.Len(t, deliveries, 1)
	assert.Equal(t, DeliveryStatusFailed, deliveries[0].Status)

	// 将 next_retry_at 设为过去（立即重试）
	pastTime := time.Now().Add(-1 * time.Minute)
	require.NoError(t, db.Model(&model.WebhookDelivery{}).
		Where("id = ?", deliveries[0].ID).
		Update("next_retry_at", pastTime).Error)

	// 处理重试
	processed, err := mgr.ProcessPendingRetries(ctx, 10)
	require.NoError(t, err)
	assert.Equal(t, 1, processed)

	// 验证成功
	var d model.WebhookDelivery
	require.NoError(t, db.Where("id = ?", deliveries[0].ID).First(&d).Error)
	assert.Equal(t, DeliveryStatusSuccess, d.Status)
}

func TestWebhookManager_ProcessPendingRetries_NotDueYet(t *testing.T) {
	db := setupTestDB(t)
	cache, _ := setupTestCfgCache(t, db, nil)
	cry := setupTestCrypto(t)
	mgr := NewWebhookManager(db, cache, cry)
	ctx := context.Background()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	ep := &model.WebhookEndpoint{
		TenantID: 1001,
		Name:     "ep1",
		URL:      srv.URL,
		Events:   "order.paid",
	}
	require.NoError(t, mgr.CreateEndpoint(ctx, ep, ""))

	_, err := mgr.DispatchEvent(ctx, 1001, EventOrderPaid, map[string]interface{}{"i": 1})
	require.NoError(t, err)

	// next_retry_at 是未来时间，不会重试
	processed, err := mgr.ProcessPendingRetries(ctx, 10)
	require.NoError(t, err)
	assert.Equal(t, 0, processed)
}

func TestWebhookManager_ProcessPendingRetries_Empty(t *testing.T) {
	db := setupTestDB(t)
	cache, _ := setupTestCfgCache(t, db, nil)
	cry := setupTestCrypto(t)
	mgr := NewWebhookManager(db, cache, cry)

	processed, err := mgr.ProcessPendingRetries(context.Background(), 10)
	require.NoError(t, err)
	assert.Equal(t, 0, processed)
}

// ============== 常量校验 ==============

func TestConstants_NoConflict(t *testing.T) {
	// 配置键唯一
	keys := []string{
		CfgKeyTokenPrefix, CfgKeyTokenLength, CfgKeyTokenMaxPerTenant,
		CfgKeyTokenDefaultTTLDays, CfgKeyScopeAvailable,
		CfgKeyWebhookTimeout, CfgKeyWebhookMaxRetry, CfgKeyWebhookFailThreshold,
	}
	seen := make(map[string]bool)
	for _, k := range keys {
		assert.False(t, seen[k], "duplicate config key: %s", k)
		seen[k] = true
	}

	// 事件类型
	events := []string{
		EventOrderPaid, EventCardGenerated, EventAgentRegistered,
		EventAgentRechargeApproved, EventAgentWithdrawPaid,
	}
	seenEvents := make(map[string]bool)
	for _, e := range events {
		assert.False(t, seenEvents[e], "duplicate event: %s", e)
		seenEvents[e] = true
	}

	// 状态
	assert.Equal(t, "active", TokenStatusActive)
	assert.Equal(t, "revoked", TokenStatusRevoked)
	assert.Equal(t, "active", EndpointStatusActive)
	assert.Equal(t, "disabled", EndpointStatusDisabled)
	assert.Equal(t, "pending", DeliveryStatusPending)
	assert.Equal(t, "success", DeliveryStatusSuccess)
	assert.Equal(t, "failed", DeliveryStatusFailed)
}

// ============== 集成场景：Token + Scope 验证 ==============

func TestIntegration_TokenValidateAndScopeCheck(t *testing.T) {
	db := setupTestDB(t)
	cache, _ := setupTestCfgCache(t, db, nil)
	mgr := NewTokenManager(db, cache)
	ctx := context.Background()

	// 生成有 card.read / order.read scope 的 token
	plain, _, err := mgr.GenerateToken(ctx, 1001, "test", "card.read,order.read", 30)
	require.NoError(t, err)

	token, err := mgr.ValidateToken(ctx, plain, "1.1.1.1")
	require.NoError(t, err)

	// scope 校验
	assert.True(t, HasScope(token.Scopes, ScopeCardRead))
	assert.True(t, HasScope(token.Scopes, ScopeOrderRead))
	assert.False(t, HasScope(token.Scopes, ScopeCardWrite))
	assert.False(t, HasScope(token.Scopes, ScopeAgentRead))

	// ParseScopes
	scopes := ParseScopes(token.Scopes)
	assert.Len(t, scopes, 2)
	assert.Contains(t, scopes, "card.read")
	assert.Contains(t, scopes, "order.read")
}

// ============== 边界场景 ==============

func TestWebhookManager_DispatchEvent_LargePayload(t *testing.T) {
	db := setupTestDB(t)
	cache, _ := setupTestCfgCache(t, db, nil)
	cry := setupTestCrypto(t)
	mgr := NewWebhookManager(db, cache, cry)
	ctx := context.Background()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer srv.Close()

	ep := &model.WebhookEndpoint{
		TenantID: 1001,
		Name:     "ep1",
		URL:      srv.URL,
		Events:   "order.paid",
	}
	require.NoError(t, mgr.CreateEndpoint(ctx, ep, ""))

	// 大 payload（10KB）
	bigData := map[string]interface{}{
		"data": strings.Repeat("a", 10240),
	}
	deliveries, err := mgr.DispatchEvent(ctx, 1001, EventOrderPaid, bigData)
	require.NoError(t, err)
	assert.Len(t, deliveries, 1)
	assert.Equal(t, DeliveryStatusSuccess, deliveries[0].Status)
}

func TestWebhookManager_DispatchEvent_NilPayload(t *testing.T) {
	db := setupTestDB(t)
	cache, _ := setupTestCfgCache(t, db, nil)
	cry := setupTestCrypto(t)
	mgr := NewWebhookManager(db, cache, cry)
	ctx := context.Background()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer srv.Close()

	ep := &model.WebhookEndpoint{
		TenantID: 1001,
		Name:     "ep1",
		URL:      srv.URL,
		Events:   "order.paid",
	}
	require.NoError(t, mgr.CreateEndpoint(ctx, ep, ""))

	// nil payload
	deliveries, err := mgr.DispatchEvent(ctx, 1001, EventOrderPaid, nil)
	require.NoError(t, err)
	assert.Len(t, deliveries, 1)
	assert.Equal(t, DeliveryStatusSuccess, deliveries[0].Status)
	assert.Equal(t, "null", deliveries[0].Payload)
}

func TestTokenManager_GenerateToken_DifferentLengths(t *testing.T) {
	db := setupTestDB(t)
	cache, _ := setupTestCfgCache(t, db, map[string]string{
		CfgKeyTokenLength: "16", // 最小长度
	})
	mgr := NewTokenManager(db, cache)

	plain, _, err := mgr.GenerateToken(context.Background(), 1001, "test", "", 30)
	require.NoError(t, err)
	// pat_(4) + 16 chars = 20 chars
	assert.Len(t, plain, 20)
}

func TestHex_Decoding(t *testing.T) {
	// 辅助测试：验证 hex 编码的 hash 可被解码回字节
	original := []byte("test data")
	hash := sha512.Sum512(original)
	hexStr := hex.EncodeToString(hash[:])
	decoded, err := hex.DecodeString(hexStr)
	require.NoError(t, err)
	assert.Equal(t, hash[:], decoded)
	assert.Len(t, decoded, 64, "SHA-512 = 64 bytes")
}

// 引用 sha512 包，避免未使用导入
var _ = sha512.Sum512
