// Package enduser v0.4.0 终端用户体系单元测试
// 严格遵循铁律 06：所有断言基于已知固定输入，无随机/不确定性
// 测试覆盖：
//   1. Register（成功 / 注册关闭 / 用户名空 / 密码过短 / 重复用户名）
//   2. Login（成功 / 用户不存在 / 密码错误 / 用户封禁）
//   3. ValidateAccessToken（合法 / 签名错误 / 过期 / 格式错误）
//   4. parseUA（pc / mobile / bot / 过长截断）
//   5. RefreshToken 轮换（旧 token 失效 / 新 token 可用）
//   6. Logout / RevokeSession / RevokeAllSessions / ListSessions
//   7. BindCard（成功 / 卡密不存在 / 卡密封禁 / 卡密已绑他人 / 达上限 / 幂等 / 重新激活）
//   8. UnbindCard（成功 / 未绑定）
//   9. ListMyCards / GetCardDetail
//  10. UpdateProfile（字段白名单过滤）
//  11. ChangePassword（旧密码错误 / 新密码过短 / 撤销所有会话）
//  12. ResetPassword
//  13. 状态机常量
package enduser

import (
	"context"
	"fmt"
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

const testJWTSecret = "test-jwt-secret-key-for-enduser"

func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:enduser_test_%d?mode=memory&cache=shared", time.Now().UnixNano())), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&model.EndUser{},
		&model.EndUserCard{},
		&model.EndUserToken{},
		&model.AppCard{},
		&model.SysConfig{},
	))
	db.Exec("DELETE FROM end_user")
	db.Exec("DELETE FROM end_user_card")
	db.Exec("DELETE FROM end_user_token")
	db.Exec("DELETE FROM app_card")
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
		CfgKeyRegisterEnabled:      "1",
		CfgKeyLoginMethod:          "username",
		CfgKeyPasswordMinLength:    "6",
		CfgKeyVerifyCodeTTL:        "300",
		CfgKeyVerifyCodeLength:     "6",
		CfgKeyAccessTokenTTL:       "2",
		CfgKeyRefreshTokenTTL:      "30",
		CfgKeyBindCardPerUserMax:   "3",
		CfgKeyAllowAnonymousQuery:  "1",
		CfgKeyIPRateLimitPerMinute: "60",
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
			ConfigGroup: "enduser",
		}).Error)
	}
	return config.NewConfigCache(db, rdb), mr
}

// ============== 1. Register ==============

func TestRegister_Success(t *testing.T) {
	db := setupTestDB(t)
	cache, _ := setupTestCfgCache(t, db, nil)
	mgr := NewManager(db, cache)

	user, err := mgr.Register(context.Background(), RegisterRequest{
		TenantID: 1, AppID: 10,
		Username: "alice", Password: "password123",
		Email: "alice@example.com", Nickname: "Alice",
	})
	require.NoError(t, err)
	assert.True(t, user.ID > 0)
	assert.Equal(t, "alice", user.Username)
	assert.Equal(t, "alice@example.com", user.Email)
	assert.Equal(t, UserStatusActive, user.Status)
	assert.NotEmpty(t, user.PasswordHash)
	assert.NotEqual(t, "password123", user.PasswordHash) // 必须已 hash
}

func TestRegister_Disabled(t *testing.T) {
	db := setupTestDB(t)
	cache, _ := setupTestCfgCache(t, db, map[string]string{CfgKeyRegisterEnabled: "0"})
	mgr := NewManager(db, cache)

	_, err := mgr.Register(context.Background(), RegisterRequest{
		TenantID: 1, AppID: 10, Username: "u", Password: "p",
	})
	assert.ErrorIs(t, err, ErrRegisterDisabled)
}

func TestRegister_EmptyUsername(t *testing.T) {
	db := setupTestDB(t)
	cache, _ := setupTestCfgCache(t, db, nil)
	mgr := NewManager(db, cache)

	_, err := mgr.Register(context.Background(), RegisterRequest{
		TenantID: 1, AppID: 10, Username: "   ", Password: "password123",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "username required")
}

func TestRegister_PasswordTooShort(t *testing.T) {
	db := setupTestDB(t)
	cache, _ := setupTestCfgCache(t, db, map[string]string{CfgKeyPasswordMinLength: "8"})
	mgr := NewManager(db, cache)

	_, err := mgr.Register(context.Background(), RegisterRequest{
		TenantID: 1, AppID: 10, Username: "u", Password: "short",
	})
	assert.ErrorIs(t, err, ErrPasswordTooShort)
}

func TestRegister_DuplicateUsername(t *testing.T) {
	db := setupTestDB(t)
	cache, _ := setupTestCfgCache(t, db, nil)
	mgr := NewManager(db, cache)
	ctx := context.Background()

	_, err := mgr.Register(ctx, RegisterRequest{
		TenantID: 1, AppID: 10, Username: "alice", Password: "password123",
	})
	require.NoError(t, err)

	_, err = mgr.Register(ctx, RegisterRequest{
		TenantID: 1, AppID: 10, Username: "alice", Password: "password456",
	})
	assert.ErrorIs(t, err, ErrUserExists)
}

func TestRegister_SameUsernameDifferentTenant(t *testing.T) {
	// 不同租户 + 应用下用户名可重复
	db := setupTestDB(t)
	cache, _ := setupTestCfgCache(t, db, nil)
	mgr := NewManager(db, cache)
	ctx := context.Background()

	_, err := mgr.Register(ctx, RegisterRequest{
		TenantID: 1, AppID: 10, Username: "alice", Password: "password123",
	})
	require.NoError(t, err)

	_, err = mgr.Register(ctx, RegisterRequest{
		TenantID: 2, AppID: 20, Username: "alice", Password: "password123",
	})
	assert.NoError(t, err) // 不同租户允许
}

// ============== 2. Login ==============

func TestLogin_Success(t *testing.T) {
	db := setupTestDB(t)
	cache, _ := setupTestCfgCache(t, db, nil)
	mgr := NewManager(db, cache)
	ctx := context.Background()

	_, err := mgr.Register(ctx, RegisterRequest{
		TenantID: 1, AppID: 10, Username: "alice", Password: "password123",
	})
	require.NoError(t, err)

	tokenPair, user, err := mgr.Login(ctx, LoginRequest{
		TenantID: 1, AppID: 10, Username: "alice", Password: "password123",
		IP: "127.0.0.1", UserAgent: "Mozilla/5.0",
	}, testJWTSecret)
	require.NoError(t, err)
	assert.NotEmpty(t, tokenPair.AccessToken)
	assert.NotEmpty(t, tokenPair.RefreshToken)
	assert.Equal(t, "Bearer", tokenPair.TokenType)
	assert.Equal(t, int64(2*3600), tokenPair.ExpiresIn)
	assert.Equal(t, "alice", user.Username)

	// access token 可通过 ValidateAccessToken 校验
	uid, appID, err := ValidateAccessToken(tokenPair.AccessToken, testJWTSecret)
	require.NoError(t, err)
	assert.Equal(t, user.ID, uid)
	assert.Equal(t, uint64(10), appID)
}

func TestLogin_UserNotFound(t *testing.T) {
	db := setupTestDB(t)
	cache, _ := setupTestCfgCache(t, db, nil)
	mgr := NewManager(db, cache)

	_, _, err := mgr.Login(context.Background(), LoginRequest{
		TenantID: 1, AppID: 10, Username: "ghost", Password: "x",
	}, testJWTSecret)
	assert.ErrorIs(t, err, ErrUserNotFound)
}

func TestLogin_PasswordIncorrect(t *testing.T) {
	db := setupTestDB(t)
	cache, _ := setupTestCfgCache(t, db, nil)
	mgr := NewManager(db, cache)
	ctx := context.Background()

	_, err := mgr.Register(ctx, RegisterRequest{
		TenantID: 1, AppID: 10, Username: "alice", Password: "password123",
	})
	require.NoError(t, err)

	_, _, err = mgr.Login(ctx, LoginRequest{
		TenantID: 1, AppID: 10, Username: "alice", Password: "wrong-password",
	}, testJWTSecret)
	assert.ErrorIs(t, err, ErrPasswordIncorrect)
}

func TestLogin_UserBanned(t *testing.T) {
	db := setupTestDB(t)
	cache, _ := setupTestCfgCache(t, db, nil)
	mgr := NewManager(db, cache)
	ctx := context.Background()

	user, err := mgr.Register(ctx, RegisterRequest{
		TenantID: 1, AppID: 10, Username: "alice", Password: "password123",
	})
	require.NoError(t, err)

	// 封禁
	require.NoError(t, db.Model(&model.EndUser{}).Where("id = ?", user.ID).
		Update("status", UserStatusBanned).Error)

	_, _, err = mgr.Login(ctx, LoginRequest{
		TenantID: 1, AppID: 10, Username: "alice", Password: "password123",
	}, testJWTSecret)
	assert.ErrorIs(t, err, ErrUserBanned)
}

// ============== 3. ValidateAccessToken ==============

func TestValidateAccessToken_Valid(t *testing.T) {
	db := setupTestDB(t)
	cache, _ := setupTestCfgCache(t, db, nil)
	mgr := NewManager(db, cache)
	ctx := context.Background()

	user, err := mgr.Register(ctx, RegisterRequest{
		TenantID: 1, AppID: 10, Username: "alice", Password: "password123",
	})
	require.NoError(t, err)
	tokenPair, _, err := mgr.Login(ctx, LoginRequest{
		TenantID: 1, AppID: 10, Username: "alice", Password: "password123",
	}, testJWTSecret)
	require.NoError(t, err)

	uid, appID, err := ValidateAccessToken(tokenPair.AccessToken, testJWTSecret)
	require.NoError(t, err)
	assert.Equal(t, user.ID, uid)
	assert.Equal(t, uint64(10), appID)
}

func TestValidateAccessToken_WrongSecret(t *testing.T) {
	db := setupTestDB(t)
	cache, _ := setupTestCfgCache(t, db, nil)
	mgr := NewManager(db, cache)
	ctx := context.Background()

	_, err := mgr.Register(ctx, RegisterRequest{
		TenantID: 1, AppID: 10, Username: "alice", Password: "password123",
	})
	require.NoError(t, err)
	tokenPair, _, err := mgr.Login(ctx, LoginRequest{
		TenantID: 1, AppID: 10, Username: "alice", Password: "password123",
	}, testJWTSecret)
	require.NoError(t, err)

	// 用错误的 secret 校验
	_, _, err = ValidateAccessToken(tokenPair.AccessToken, "wrong-secret")
	assert.ErrorIs(t, err, ErrTokenInvalid)
}

func TestValidateAccessToken_Expired(t *testing.T) {
	// 构造一个已过期的 token
	user := &model.EndUser{ID: 100, AppID: 10}
	// 直接调用 generateAccessToken，传入负 TTL
	// generateAccessToken 用 time.Now().Add(ttlHours * time.Hour)，传 -1 即 1 小时前
	token, err := generateAccessToken(user, testJWTSecret, -1)
	require.NoError(t, err)

	_, _, err = ValidateAccessToken(token, testJWTSecret)
	assert.ErrorIs(t, err, ErrTokenExpired)
}

func TestValidateAccessToken_Malformed(t *testing.T) {
	_, _, err := ValidateAccessToken("no-dot-in-token", testJWTSecret)
	assert.ErrorIs(t, err, ErrTokenInvalid)

	_, _, err = ValidateAccessToken("", testJWTSecret)
	assert.ErrorIs(t, err, ErrTokenInvalid)
}

// ============== 4. parseUA ==============

func TestParseUA_PC(t *testing.T) {
	name, dtype := parseUA("Mozilla/5.0 (Windows NT 10.0; Win64; x64)")
	assert.Equal(t, "pc", dtype)
	assert.Contains(t, name, "mozilla")
}

func TestParseUA_Mobile(t *testing.T) {
	_, dtype := parseUA("Mozilla/5.0 (iPhone; CPU iPhone OS 14_0 like Mac OS X) Mobile/15E148")
	assert.Equal(t, "mobile", dtype)

	_, dtype = parseUA("Android SDK built for x86")
	assert.Equal(t, "mobile", dtype)
}

func TestParseUA_Bot(t *testing.T) {
	_, dtype := parseUA("Googlebot/2.1 (+http://www.google.com/bot.html)")
	assert.Equal(t, "bot", dtype)

	_, dtype = parseUA("Baiduspider+(+http://www.baidu.com)")
	assert.Equal(t, "bot", dtype)
}

func TestParseUA_Truncation(t *testing.T) {
	longUA := ""
	for i := 0; i < 200; i++ {
		longUA += "a"
	}
	name, _ := parseUA(longUA)
	assert.True(t, len(name) <= 128)
}

// ============== 5. RefreshToken 轮换 ==============

func TestRefreshToken_RotateSuccess(t *testing.T) {
	db := setupTestDB(t)
	cache, _ := setupTestCfgCache(t, db, nil)
	mgr := NewManager(db, cache)
	ctx := context.Background()

	_, err := mgr.Register(ctx, RegisterRequest{
		TenantID: 1, AppID: 10, Username: "alice", Password: "password123",
	})
	require.NoError(t, err)
	tokenPair, _, err := mgr.Login(ctx, LoginRequest{
		TenantID: 1, AppID: 10, Username: "alice", Password: "password123",
	}, testJWTSecret)
	require.NoError(t, err)

	// 用旧 refresh token 换新
	newPair, err := mgr.RefreshToken(ctx, tokenPair.RefreshToken, testJWTSecret, "127.0.0.1", "ua")
	require.NoError(t, err)
	assert.NotEmpty(t, newPair.AccessToken)
	assert.NotEmpty(t, newPair.RefreshToken)
	assert.NotEqual(t, tokenPair.RefreshToken, newPair.RefreshToken)

	// 旧 token 失效
	_, _, err = mgr.VerifyRefreshToken(ctx, tokenPair.RefreshToken)
	assert.ErrorIs(t, err, ErrTokenRevoked)

	// 新 token 可用
	_, _, err = mgr.VerifyRefreshToken(ctx, newPair.RefreshToken)
	assert.NoError(t, err)
}

func TestRefreshToken_InvalidToken(t *testing.T) {
	db := setupTestDB(t)
	cache, _ := setupTestCfgCache(t, db, nil)
	mgr := NewManager(db, cache)

	_, err := mgr.RefreshToken(context.Background(), "invalid-token", testJWTSecret, "", "")
	assert.ErrorIs(t, err, ErrTokenInvalid)
}

func TestVerifyRefreshToken_Empty(t *testing.T) {
	db := setupTestDB(t)
	cache, _ := setupTestCfgCache(t, db, nil)
	mgr := NewManager(db, cache)

	_, _, err := mgr.VerifyRefreshToken(context.Background(), "")
	assert.ErrorIs(t, err, ErrTokenInvalid)
}

// ============== 6. Logout / Revoke ==============

func TestLogout_Success(t *testing.T) {
	db := setupTestDB(t)
	cache, _ := setupTestCfgCache(t, db, nil)
	mgr := NewManager(db, cache)
	ctx := context.Background()

	_, err := mgr.Register(ctx, RegisterRequest{
		TenantID: 1, AppID: 10, Username: "alice", Password: "password123",
	})
	require.NoError(t, err)
	tokenPair, _, err := mgr.Login(ctx, LoginRequest{
		TenantID: 1, AppID: 10, Username: "alice", Password: "password123",
	}, testJWTSecret)
	require.NoError(t, err)

	// 注销前可验证
	_, _, err = mgr.VerifyRefreshToken(ctx, tokenPair.RefreshToken)
	require.NoError(t, err)

	// 注销
	require.NoError(t, mgr.Logout(ctx, tokenPair.RefreshToken))

	// 注销后失效
	_, _, err = mgr.VerifyRefreshToken(ctx, tokenPair.RefreshToken)
	assert.ErrorIs(t, err, ErrTokenRevoked)
}

func TestRevokeSession_ByJTI(t *testing.T) {
	db := setupTestDB(t)
	cache, _ := setupTestCfgCache(t, db, nil)
	mgr := NewManager(db, cache)
	ctx := context.Background()

	user, err := mgr.Register(ctx, RegisterRequest{
		TenantID: 1, AppID: 10, Username: "alice", Password: "password123",
	})
	require.NoError(t, err)
	tokenPair, _, err := mgr.Login(ctx, LoginRequest{
		TenantID: 1, AppID: 10, Username: "alice", Password: "password123",
	}, testJWTSecret)
	require.NoError(t, err)

	// 查 jti
	sessions, err := mgr.ListSessions(ctx, user.ID)
	require.NoError(t, err)
	require.Equal(t, 1, len(sessions))
	jti := sessions[0].JTI

	// 踢出
	require.NoError(t, mgr.RevokeSession(ctx, user.ID, jti))

	// 失效
	_, _, err = mgr.VerifyRefreshToken(ctx, tokenPair.RefreshToken)
	assert.ErrorIs(t, err, ErrTokenRevoked)

	// ListSessions 不再返回
	sessions2, err := mgr.ListSessions(ctx, user.ID)
	require.NoError(t, err)
	assert.Equal(t, 0, len(sessions2))
}

func TestRevokeAllSessions(t *testing.T) {
	db := setupTestDB(t)
	cache, _ := setupTestCfgCache(t, db, nil)
	mgr := NewManager(db, cache)
	ctx := context.Background()

	user, err := mgr.Register(ctx, RegisterRequest{
		TenantID: 1, AppID: 10, Username: "alice", Password: "password123",
	})
	require.NoError(t, err)

	// 多设备登录
	_, _, err = mgr.Login(ctx, LoginRequest{
		TenantID: 1, AppID: 10, Username: "alice", Password: "password123",
		IP: "1.1.1.1", UserAgent: "ua1",
	}, testJWTSecret)
	require.NoError(t, err)
	_, _, err = mgr.Login(ctx, LoginRequest{
		TenantID: 1, AppID: 10, Username: "alice", Password: "password123",
		IP: "2.2.2.2", UserAgent: "ua2",
	}, testJWTSecret)
	require.NoError(t, err)

	sessions, err := mgr.ListSessions(ctx, user.ID)
	require.NoError(t, err)
	assert.Equal(t, 2, len(sessions))

	// 撤销所有
	require.NoError(t, mgr.RevokeAllSessions(ctx, user.ID))

	sessions2, err := mgr.ListSessions(ctx, user.ID)
	require.NoError(t, err)
	assert.Equal(t, 0, len(sessions2))
}

func TestListSessions_ExcludesExpired(t *testing.T) {
	db := setupTestDB(t)
	cache, _ := setupTestCfgCache(t, db, nil)
	mgr := NewManager(db, cache)
	ctx := context.Background()

	user, err := mgr.Register(ctx, RegisterRequest{
		TenantID: 1, AppID: 10, Username: "alice", Password: "password123",
	})
	require.NoError(t, err)

	// 手动插入一条已过期的 token
	pastTime := time.Now().Add(-time.Hour)
	require.NoError(t, db.Create(&model.EndUserToken{
		UserID:       user.ID,
		JTI:          "expired-jti",
		DeviceType:   "pc",
		RefreshToken: "expired-hash",
		ExpiresAt:    pastTime,
	}).Error)

	sessions, err := mgr.ListSessions(ctx, user.ID)
	require.NoError(t, err)
	assert.Equal(t, 0, len(sessions)) // 不应包含已过期的
}

// ============== 7. BindCard ==============

func TestBindCard_Success(t *testing.T) {
	db := setupTestDB(t)
	cache, _ := setupTestCfgCache(t, db, nil)
	mgr := NewManager(db, cache)
	ctx := context.Background()

	user, err := mgr.Register(ctx, RegisterRequest{
		TenantID: 1, AppID: 10, Username: "alice", Password: "password123",
	})
	require.NoError(t, err)

	// 创建一张卡
	card := &model.AppCard{
		TenantID: 1, AppID: 10, CardTypeID: 1, CardKey: "K-001",
		CardKeyHash: "hash-001", Checksum: "ab", Status: "unused",
		DurationSeconds: 86400, MaxUses: 1, CreatedBy: 1,
	}
	require.NoError(t, db.Create(card).Error)

	entry, err := mgr.BindCard(ctx, user.ID, card.ID)
	require.NoError(t, err)
	assert.Equal(t, user.ID, entry.UserID)
	assert.Equal(t, card.ID, entry.CardID)
	assert.Equal(t, BindStatusActive, entry.Status)

	// 验证卡密 end_user_id 已更新
	var updatedCard model.AppCard
	require.NoError(t, db.First(&updatedCard, card.ID).Error)
	require.NotNil(t, updatedCard.EndUserID)
	assert.Equal(t, user.ID, *updatedCard.EndUserID)
}

func TestBindCard_CardNotFound(t *testing.T) {
	db := setupTestDB(t)
	cache, _ := setupTestCfgCache(t, db, nil)
	mgr := NewManager(db, cache)
	ctx := context.Background()

	user, err := mgr.Register(ctx, RegisterRequest{
		TenantID: 1, AppID: 10, Username: "alice", Password: "password123",
	})
	require.NoError(t, err)

	_, err = mgr.BindCard(ctx, user.ID, 999999)
	assert.ErrorIs(t, err, ErrCardNotFound)
}

func TestBindCard_CardBanned(t *testing.T) {
	db := setupTestDB(t)
	cache, _ := setupTestCfgCache(t, db, nil)
	mgr := NewManager(db, cache)
	ctx := context.Background()

	user, err := mgr.Register(ctx, RegisterRequest{
		TenantID: 1, AppID: 10, Username: "alice", Password: "password123",
	})
	require.NoError(t, err)

	card := &model.AppCard{
		TenantID: 1, AppID: 10, CardTypeID: 1, CardKey: "K-banned",
		CardKeyHash: "hash-banned", Checksum: "ab", Status: "banned",
		DurationSeconds: 86400, MaxUses: 1, CreatedBy: 1,
	}
	require.NoError(t, db.Create(card).Error)

	_, err = mgr.BindCard(ctx, user.ID, card.ID)
	assert.ErrorIs(t, err, ErrCardStatusInvalid)
}

func TestBindCard_CardDisabled(t *testing.T) {
	db := setupTestDB(t)
	cache, _ := setupTestCfgCache(t, db, nil)
	mgr := NewManager(db, cache)
	ctx := context.Background()

	user, err := mgr.Register(ctx, RegisterRequest{
		TenantID: 1, AppID: 10, Username: "alice", Password: "password123",
	})
	require.NoError(t, err)

	card := &model.AppCard{
		TenantID: 1, AppID: 10, CardTypeID: 1, CardKey: "K-dis",
		CardKeyHash: "hash-dis", Checksum: "ab", Status: "disabled",
		DurationSeconds: 86400, MaxUses: 1, CreatedBy: 1,
	}
	require.NoError(t, db.Create(card).Error)

	_, err = mgr.BindCard(ctx, user.ID, card.ID)
	assert.ErrorIs(t, err, ErrCardStatusInvalid)
}

func TestBindCard_AlreadyBoundToOther(t *testing.T) {
	db := setupTestDB(t)
	cache, _ := setupTestCfgCache(t, db, nil)
	mgr := NewManager(db, cache)
	ctx := context.Background()

	user1, err := mgr.Register(ctx, RegisterRequest{
		TenantID: 1, AppID: 10, Username: "alice", Password: "password123",
	})
	require.NoError(t, err)
	user2, err := mgr.Register(ctx, RegisterRequest{
		TenantID: 1, AppID: 10, Username: "bob", Password: "password123",
	})
	require.NoError(t, err)

	card := &model.AppCard{
		TenantID: 1, AppID: 10, CardTypeID: 1, CardKey: "K-shared",
		CardKeyHash: "hash-shared", Checksum: "ab", Status: "unused",
		DurationSeconds: 86400, MaxUses: 1, CreatedBy: 1,
	}
	require.NoError(t, db.Create(card).Error)

	// user1 先绑
	_, err = mgr.BindCard(ctx, user1.ID, card.ID)
	require.NoError(t, err)

	// user2 再绑失败
	_, err = mgr.BindCard(ctx, user2.ID, card.ID)
	assert.ErrorIs(t, err, ErrCardBoundToOther)
}

func TestBindCard_Idempotent(t *testing.T) {
	// 同一用户重复绑定同一张卡应幂等返回
	db := setupTestDB(t)
	cache, _ := setupTestCfgCache(t, db, nil)
	mgr := NewManager(db, cache)
	ctx := context.Background()

	user, err := mgr.Register(ctx, RegisterRequest{
		TenantID: 1, AppID: 10, Username: "alice", Password: "password123",
	})
	require.NoError(t, err)

	card := &model.AppCard{
		TenantID: 1, AppID: 10, CardTypeID: 1, CardKey: "K-idem",
		CardKeyHash: "hash-idem", Checksum: "ab", Status: "unused",
		DurationSeconds: 86400, MaxUses: 1, CreatedBy: 1,
	}
	require.NoError(t, db.Create(card).Error)

	entry1, err := mgr.BindCard(ctx, user.ID, card.ID)
	require.NoError(t, err)
	entry2, err := mgr.BindCard(ctx, user.ID, card.ID)
	require.NoError(t, err)
	assert.Equal(t, entry1.ID, entry2.ID) // 同一条记录
}

func TestBindCard_RebindAfterUnbind(t *testing.T) {
	db := setupTestDB(t)
	cache, _ := setupTestCfgCache(t, db, nil)
	mgr := NewManager(db, cache)
	ctx := context.Background()

	user, err := mgr.Register(ctx, RegisterRequest{
		TenantID: 1, AppID: 10, Username: "alice", Password: "password123",
	})
	require.NoError(t, err)

	card := &model.AppCard{
		TenantID: 1, AppID: 10, CardTypeID: 1, CardKey: "K-rebind",
		CardKeyHash: "hash-rebind", Checksum: "ab", Status: "unused",
		DurationSeconds: 86400, MaxUses: 1, CreatedBy: 1,
	}
	require.NoError(t, db.Create(card).Error)

	// 绑定 → 解绑 → 再绑
	_, err = mgr.BindCard(ctx, user.ID, card.ID)
	require.NoError(t, err)
	require.NoError(t, mgr.UnbindCard(ctx, user.ID, card.ID))

	entry, err := mgr.BindCard(ctx, user.ID, card.ID)
	require.NoError(t, err)
	assert.Equal(t, BindStatusActive, entry.Status)
}

func TestBindCard_LimitExceeded(t *testing.T) {
	db := setupTestDB(t)
	cache, _ := setupTestCfgCache(t, db, map[string]string{CfgKeyBindCardPerUserMax: "2"})
	mgr := NewManager(db, cache)
	ctx := context.Background()

	user, err := mgr.Register(ctx, RegisterRequest{
		TenantID: 1, AppID: 10, Username: "alice", Password: "password123",
	})
	require.NoError(t, err)

	// 创建 3 张卡
	for i := 0; i < 3; i++ {
		card := &model.AppCard{
			TenantID: 1, AppID: 10, CardTypeID: 1,
			CardKey: fmt.Sprintf("K-limit-%d", i),
			CardKeyHash: fmt.Sprintf("hash-limit-%d", i),
			Checksum: "ab", Status: "unused",
			DurationSeconds: 86400, MaxUses: 1, CreatedBy: 1,
		}
		require.NoError(t, db.Create(card).Error)

		if i < 2 {
			_, err := mgr.BindCard(ctx, user.ID, card.ID)
			require.NoError(t, err)
		} else {
			// 第 3 张应失败
			_, err := mgr.BindCard(ctx, user.ID, card.ID)
			assert.ErrorIs(t, err, ErrBindLimitExceeded)
		}
	}
}

// ============== 8. UnbindCard ==============

func TestUnbindCard_Success(t *testing.T) {
	db := setupTestDB(t)
	cache, _ := setupTestCfgCache(t, db, nil)
	mgr := NewManager(db, cache)
	ctx := context.Background()

	user, err := mgr.Register(ctx, RegisterRequest{
		TenantID: 1, AppID: 10, Username: "alice", Password: "password123",
	})
	require.NoError(t, err)

	card := &model.AppCard{
		TenantID: 1, AppID: 10, CardTypeID: 1, CardKey: "K-unbind",
		CardKeyHash: "hash-unbind", Checksum: "ab", Status: "unused",
		DurationSeconds: 86400, MaxUses: 1, CreatedBy: 1,
	}
	require.NoError(t, db.Create(card).Error)

	_, err = mgr.BindCard(ctx, user.ID, card.ID)
	require.NoError(t, err)

	require.NoError(t, mgr.UnbindCard(ctx, user.ID, card.ID))

	// 卡密 end_user_id 应清空
	var updatedCard model.AppCard
	require.NoError(t, db.First(&updatedCard, card.ID).Error)
	assert.Nil(t, updatedCard.EndUserID)

	// 绑定记录应标记为 unbound
	var entry model.EndUserCard
	require.NoError(t, db.Where("card_id = ?", card.ID).First(&entry).Error)
	assert.Equal(t, BindStatusUnbound, entry.Status)
	assert.NotNil(t, entry.UnboundAt)
}

func TestUnbindCard_NotBound(t *testing.T) {
	db := setupTestDB(t)
	cache, _ := setupTestCfgCache(t, db, nil)
	mgr := NewManager(db, cache)
	ctx := context.Background()

	user, err := mgr.Register(ctx, RegisterRequest{
		TenantID: 1, AppID: 10, Username: "alice", Password: "password123",
	})
	require.NoError(t, err)

	card := &model.AppCard{
		TenantID: 1, AppID: 10, CardTypeID: 1, CardKey: "K-notbound",
		CardKeyHash: "hash-notbound", Checksum: "ab", Status: "unused",
		DurationSeconds: 86400, MaxUses: 1, CreatedBy: 1,
	}
	require.NoError(t, db.Create(card).Error)

	err = mgr.UnbindCard(ctx, user.ID, card.ID)
	assert.ErrorIs(t, err, ErrCardNotFound)
}

// ============== 9. ListMyCards / GetCardDetail ==============

func TestListMyCards(t *testing.T) {
	db := setupTestDB(t)
	cache, _ := setupTestCfgCache(t, db, nil)
	mgr := NewManager(db, cache)
	ctx := context.Background()

	user, err := mgr.Register(ctx, RegisterRequest{
		TenantID: 1, AppID: 10, Username: "alice", Password: "password123",
	})
	require.NoError(t, err)

	// 绑 3 张
	for i := 0; i < 3; i++ {
		card := &model.AppCard{
			TenantID: 1, AppID: 10, CardTypeID: 1,
			CardKey: fmt.Sprintf("K-list-%d", i),
			CardKeyHash: fmt.Sprintf("hash-list-%d", i),
			Checksum: "ab", Status: "unused",
			DurationSeconds: 86400, MaxUses: 1, CreatedBy: 1,
		}
		require.NoError(t, db.Create(card).Error)
		_, err := mgr.BindCard(ctx, user.ID, card.ID)
		require.NoError(t, err)
	}

	cards, total, err := mgr.ListMyCards(ctx, user.ID, 1, 20)
	require.NoError(t, err)
	assert.Equal(t, int64(3), total)
	assert.Equal(t, 3, len(cards))

	// 分页
	cards2, total2, err := mgr.ListMyCards(ctx, user.ID, 1, 2)
	require.NoError(t, err)
	assert.Equal(t, int64(3), total2)
	assert.Equal(t, 2, len(cards2))

	cards3, _, err := mgr.ListMyCards(ctx, user.ID, 2, 2)
	require.NoError(t, err)
	assert.Equal(t, 1, len(cards3))
}

func TestListMyCards_Empty(t *testing.T) {
	db := setupTestDB(t)
	cache, _ := setupTestCfgCache(t, db, nil)
	mgr := NewManager(db, cache)

	cards, total, err := mgr.ListMyCards(context.Background(), 999, 1, 20)
	require.NoError(t, err)
	assert.Equal(t, int64(0), total)
	assert.Equal(t, 0, len(cards))
}

func TestGetCardDetail_Success(t *testing.T) {
	db := setupTestDB(t)
	cache, _ := setupTestCfgCache(t, db, nil)
	mgr := NewManager(db, cache)
	ctx := context.Background()

	user, err := mgr.Register(ctx, RegisterRequest{
		TenantID: 1, AppID: 10, Username: "alice", Password: "password123",
	})
	require.NoError(t, err)

	card := &model.AppCard{
		TenantID: 1, AppID: 10, CardTypeID: 1, CardKey: "K-detail",
		CardKeyHash: "hash-detail", Checksum: "ab", Status: "unused",
		DurationSeconds: 86400, MaxUses: 1, CreatedBy: 1,
	}
	require.NoError(t, db.Create(card).Error)
	_, err = mgr.BindCard(ctx, user.ID, card.ID)
	require.NoError(t, err)

	detail, err := mgr.GetCardDetail(ctx, user.ID, card.ID)
	require.NoError(t, err)
	assert.Equal(t, card.ID, detail.ID)
	assert.Equal(t, "K-detail", detail.CardKey)
}

func TestGetCardDetail_NotOwned(t *testing.T) {
	db := setupTestDB(t)
	cache, _ := setupTestCfgCache(t, db, nil)
	mgr := NewManager(db, cache)
	ctx := context.Background()

	user, err := mgr.Register(ctx, RegisterRequest{
		TenantID: 1, AppID: 10, Username: "alice", Password: "password123",
	})
	require.NoError(t, err)

	card := &model.AppCard{
		TenantID: 1, AppID: 10, CardTypeID: 1, CardKey: "K-other",
		CardKeyHash: "hash-other", Checksum: "ab", Status: "unused",
		DurationSeconds: 86400, MaxUses: 1, CreatedBy: 1,
	}
	require.NoError(t, db.Create(card).Error)

	// 未绑定 → 不属于
	_, err = mgr.GetCardDetail(ctx, user.ID, card.ID)
	assert.ErrorIs(t, err, ErrCardNotFound)
}

// ============== 10. UpdateProfile ==============

func TestUpdateProfile_Whitelist(t *testing.T) {
	db := setupTestDB(t)
	cache, _ := setupTestCfgCache(t, db, nil)
	mgr := NewManager(db, cache)
	ctx := context.Background()

	user, err := mgr.Register(ctx, RegisterRequest{
		TenantID: 1, AppID: 10, Username: "alice", Password: "password123",
	})
	require.NoError(t, err)

	// 仅允许 nickname/avatar_url/email/phone
	require.NoError(t, mgr.UpdateProfile(ctx, user.ID, map[string]interface{}{
		"nickname":    "新昵称",
		"avatar_url":  "https://example.com/avatar.png",
		"email":       "new@example.com",
		"phone":       "13900000000",
		"password_hash": "should-be-ignored", // 应被过滤
		"status":       "banned",             // 应被过滤
	}))

	updated, err := mgr.GetProfile(ctx, user.ID)
	require.NoError(t, err)
	assert.Equal(t, "新昵称", updated.Nickname)
	assert.Equal(t, "https://example.com/avatar.png", updated.AvatarURL)
	assert.Equal(t, "new@example.com", updated.Email)
	assert.Equal(t, "13900000000", updated.Phone)
	assert.Equal(t, UserStatusActive, updated.Status) // 未被改为 banned
	assert.NotEqual(t, "should-be-ignored", updated.PasswordHash)
}

func TestUpdateProfile_EmptyFilter(t *testing.T) {
	db := setupTestDB(t)
	cache, _ := setupTestCfgCache(t, db, nil)
	mgr := NewManager(db, cache)
	ctx := context.Background()

	user, err := mgr.Register(ctx, RegisterRequest{
		TenantID: 1, AppID: 10, Username: "alice", Password: "password123",
	})
	require.NoError(t, err)

	// 全是非法字段 → 不更新，但也不报错
	require.NoError(t, mgr.UpdateProfile(ctx, user.ID, map[string]interface{}{
		"forbidden_field": "value",
	}))
}

// ============== 11. ChangePassword ==============

func TestChangePassword_Success(t *testing.T) {
	db := setupTestDB(t)
	cache, _ := setupTestCfgCache(t, db, nil)
	mgr := NewManager(db, cache)
	ctx := context.Background()

	user, err := mgr.Register(ctx, RegisterRequest{
		TenantID: 1, AppID: 10, Username: "alice", Password: "old-password",
	})
	require.NoError(t, err)
	tokenPair, _, err := mgr.Login(ctx, LoginRequest{
		TenantID: 1, AppID: 10, Username: "alice", Password: "old-password",
	}, testJWTSecret)
	require.NoError(t, err)

	// 修改密码
	require.NoError(t, mgr.ChangePassword(ctx, user.ID, "old-password", "new-password"))

	// 旧密码登录失败
	_, _, err = mgr.Login(ctx, LoginRequest{
		TenantID: 1, AppID: 10, Username: "alice", Password: "old-password",
	}, testJWTSecret)
	assert.ErrorIs(t, err, ErrPasswordIncorrect)

	// 新密码登录成功
	_, _, err = mgr.Login(ctx, LoginRequest{
		TenantID: 1, AppID: 10, Username: "alice", Password: "new-password",
	}, testJWTSecret)
	assert.NoError(t, err)

	// 旧 refresh token 应已撤销
	_, _, err = mgr.VerifyRefreshToken(ctx, tokenPair.RefreshToken)
	assert.ErrorIs(t, err, ErrTokenRevoked)
}

func TestChangePassword_WrongOld(t *testing.T) {
	db := setupTestDB(t)
	cache, _ := setupTestCfgCache(t, db, nil)
	mgr := NewManager(db, cache)
	ctx := context.Background()

	user, err := mgr.Register(ctx, RegisterRequest{
		TenantID: 1, AppID: 10, Username: "alice", Password: "old-password",
	})
	require.NoError(t, err)

	err = mgr.ChangePassword(ctx, user.ID, "wrong-old", "new-password")
	assert.ErrorIs(t, err, ErrPasswordIncorrect)
}

func TestChangePassword_NewTooShort(t *testing.T) {
	db := setupTestDB(t)
	cache, _ := setupTestCfgCache(t, db, map[string]string{CfgKeyPasswordMinLength: "8"})
	mgr := NewManager(db, cache)
	ctx := context.Background()

	user, err := mgr.Register(ctx, RegisterRequest{
		TenantID: 1, AppID: 10, Username: "alice", Password: "password123",
	})
	require.NoError(t, err)

	err = mgr.ChangePassword(ctx, user.ID, "password123", "short")
	assert.ErrorIs(t, err, ErrPasswordTooShort)
}

// ============== 12. ResetPassword ==============

func TestResetPassword_Success(t *testing.T) {
	db := setupTestDB(t)
	cache, _ := setupTestCfgCache(t, db, nil)
	mgr := NewManager(db, cache)
	ctx := context.Background()

	user, err := mgr.Register(ctx, RegisterRequest{
		TenantID: 1, AppID: 10, Username: "alice", Password: "old-password",
	})
	require.NoError(t, err)

	require.NoError(t, mgr.ResetPassword(ctx, user.ID, "reset-password"))

	// 新密码可登录
	_, _, err = mgr.Login(ctx, LoginRequest{
		TenantID: 1, AppID: 10, Username: "alice", Password: "reset-password",
	}, testJWTSecret)
	assert.NoError(t, err)
}

func TestResetPassword_TooShort(t *testing.T) {
	db := setupTestDB(t)
	cache, _ := setupTestCfgCache(t, db, map[string]string{CfgKeyPasswordMinLength: "8"})
	mgr := NewManager(db, cache)
	ctx := context.Background()

	user, err := mgr.Register(ctx, RegisterRequest{
		TenantID: 1, AppID: 10, Username: "alice", Password: "password123",
	})
	require.NoError(t, err)

	err = mgr.ResetPassword(ctx, user.ID, "short")
	assert.ErrorIs(t, err, ErrPasswordTooShort)
}

// ============== 13. 辅助函数 ==============

func TestIsRegisterEnabled(t *testing.T) {
	db := setupTestDB(t)
	cache, _ := setupTestCfgCache(t, db, map[string]string{CfgKeyRegisterEnabled: "0"})
	mgr := NewManager(db, cache)
	assert.False(t, mgr.IsRegisterEnabled(context.Background()))

	db2 := setupTestDB(t)
	cache2, _ := setupTestCfgCache(t, db2, map[string]string{CfgKeyRegisterEnabled: "1"})
	mgr2 := NewManager(db2, cache2)
	assert.True(t, mgr2.IsRegisterEnabled(context.Background()))
}

func TestIsAnonymousQueryAllowed(t *testing.T) {
	db := setupTestDB(t)
	cache, _ := setupTestCfgCache(t, db, map[string]string{CfgKeyAllowAnonymousQuery: "0"})
	mgr := NewManager(db, cache)
	assert.False(t, mgr.IsAnonymousQueryAllowed(context.Background()))

	db2 := setupTestDB(t)
	cache2, _ := setupTestCfgCache(t, db2, map[string]string{CfgKeyAllowAnonymousQuery: "1"})
	mgr2 := NewManager(db2, cache2)
	assert.True(t, mgr2.IsAnonymousQueryAllowed(context.Background()))
}

// ============== 14. 状态机常量 ==============

func TestConstants(t *testing.T) {
	// 铁律 06：常量值固定
	assert.Equal(t, "active", UserStatusActive)
	assert.Equal(t, "banned", UserStatusBanned)
	assert.Equal(t, "deleted", UserStatusDeleted)

	assert.Equal(t, "active", BindStatusActive)
	assert.Equal(t, "unbound", BindStatusUnbound)

	// 配置键
	assert.Equal(t, "enduser.register_enabled", CfgKeyRegisterEnabled)
	assert.Equal(t, "enduser.login_method", CfgKeyLoginMethod)
	assert.Equal(t, "enduser.password_min_length", CfgKeyPasswordMinLength)
	assert.Equal(t, "enduser.verify_code_ttl", CfgKeyVerifyCodeTTL)
	assert.Equal(t, "enduser.verify_code_length", CfgKeyVerifyCodeLength)
	assert.Equal(t, "enduser.access_token_ttl", CfgKeyAccessTokenTTL)
	assert.Equal(t, "enduser.refresh_token_ttl", CfgKeyRefreshTokenTTL)
	assert.Equal(t, "enduser.bind_card_per_user_max", CfgKeyBindCardPerUserMax)
	assert.Equal(t, "enduser.allow_anonymous_query", CfgKeyAllowAnonymousQuery)
	assert.Equal(t, "enduser.ip_rate_limit_per_minute", CfgKeyIPRateLimitPerMinute)
}

// ============== 15. GetProfile / UserNotFound ==============

func TestGetProfile_NotFound(t *testing.T) {
	db := setupTestDB(t)
	cache, _ := setupTestCfgCache(t, db, nil)
	mgr := NewManager(db, cache)

	_, err := mgr.GetProfile(context.Background(), 999999)
	assert.ErrorIs(t, err, ErrUserNotFound)
}

func TestChangePassword_UserNotFound(t *testing.T) {
	db := setupTestDB(t)
	cache, _ := setupTestCfgCache(t, db, nil)
	mgr := NewManager(db, cache)

	err := mgr.ChangePassword(context.Background(), 999999, "old", "new-password")
	assert.ErrorIs(t, err, ErrUserNotFound)
}

// ============== 16. bcrypt 集成 ==============

func TestBcryptIntegration(t *testing.T) {
	// 端到端验证 bcrypt cost=12，与 crypto 包集成
	hash, err := crypto.HashPassword("test-password")
	require.NoError(t, err)
	assert.True(t, crypto.CheckPassword(hash, "test-password"))
	assert.False(t, crypto.CheckPassword(hash, "wrong-password"))
	// bcrypt hash 以 $2a$ 开头
	assert.Equal(t, "$2a$", hash[:4])
}
