// internal/auth 单元测试
// 覆盖 v0.4.0 jti 黑名单核心逻辑 + Token 生成/解析
// 铁律 06：所有断言基于固定输入，无随机/不确定性
package auth

import (
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/your-org/keyauth-saas/apps/server/internal/middleware"
)

// setupMiniRedis 启动 miniredis 并返回 client + cleanup
func setupMiniRedis(t *testing.T) (*redis.Client, *miniredis.Miniredis) {
	t.Helper()
	mr, err := miniredis.Run()
	require.NoError(t, err)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() {
		_ = rdb.Close()
		mr.Close()
	})
	return rdb, mr
}

// ============== Token 生成与解析 ==============

// TestGenerateTokenPair_JTI写入 access + refresh 都携带同一 jti
func TestGenerateTokenPair_JTI写入(t *testing.T) {
	jti := "test-jti-12345"
	pair, err := GenerateTokenPair(TokenOptions{
		Secret:     "test-jwt-secret-32-bytes-needed!",
		Issuer:     "keyauth-test",
		UserID:     1001,
		Username:   "tester",
		Role:       RoleTenant,
		TenantID:   1001,
		AccessTTL:  time.Hour,
		RefreshTTL: 24 * time.Hour,
		JTI:        jti,
	})
	require.NoError(t, err)
	require.NotEmpty(t, pair.AccessToken)
	require.NotEmpty(t, pair.RefreshToken)

	// 解析 access token
	accessClaims, accessType, err := ParseToken("test-jwt-secret-32-bytes-needed!", pair.AccessToken)
	require.NoError(t, err)
	assert.Equal(t, TokenTypeAccess, accessType)
	assert.Equal(t, jti, accessClaims.ID, "access token 应携带 jti")
	assert.Equal(t, "tester", accessClaims.Username)

	// 解析 refresh token
	refreshClaims, refreshType, err := ParseToken("test-jwt-secret-32-bytes-needed!", pair.RefreshToken)
	require.NoError(t, err)
	assert.Equal(t, TokenTypeRefresh, refreshType)
	assert.Equal(t, jti, refreshClaims.ID, "refresh token 应携带同一 jti")
	assert.Equal(t, accessClaims.ID, refreshClaims.ID, "access + refresh 的 jti 必须一致")
}

// TestGenerateTokenPair_空JTI JTI 为空时仍可生成（兼容旧调用方）
func TestGenerateTokenPair_空JTI(t *testing.T) {
	pair, err := GenerateTokenPair(TokenOptions{
		Secret:     "test-jwt-secret-32-bytes-needed!",
		UserID:     1,
		Role:       RoleAdmin,
		AccessTTL:  time.Hour,
		RefreshTTL: 24 * time.Hour,
		// JTI 故意留空
	})
	require.NoError(t, err)
	require.NotEmpty(t, pair.AccessToken)

	claims, _, err := ParseToken("test-jwt-secret-32-bytes-needed!", pair.AccessToken)
	require.NoError(t, err)
	assert.Empty(t, claims.ID, "JTI 为空时 claims.ID 也应为空")
}

// TestGenerateTokenPair_空Secret 应返回错误
func TestGenerateTokenPair_空Secret(t *testing.T) {
	_, err := GenerateTokenPair(TokenOptions{
		Secret: "",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "密钥")
}

// TestParseToken_错误签名 应返回错误
func TestParseToken_错误签名(t *testing.T) {
	pair, err := GenerateTokenPair(TokenOptions{
		Secret:     "correct-secret-32-bytes-needed!!",
		UserID:     1,
		Role:       RoleAdmin,
		AccessTTL:  time.Hour,
		RefreshTTL: 24 * time.Hour,
	})
	require.NoError(t, err)

	// 用错误密钥解析
	_, _, err = ParseToken("wrong-secret-32-bytes-needed!!!!", pair.AccessToken)
	require.Error(t, err)
}

// TestParseToken_过期Token 应返回错误
func TestParseToken_过期Token(t *testing.T) {
	pair, err := GenerateTokenPair(TokenOptions{
		Secret:     "test-jwt-secret-32-bytes-needed!",
		UserID:     1,
		Role:       RoleAdmin,
		AccessTTL:  -time.Hour, // 已过期
		RefreshTTL: -time.Hour,
	})
	require.NoError(t, err)

	_, _, err = ParseToken("test-jwt-secret-32-bytes-needed!", pair.AccessToken)
	require.Error(t, err)
}

// ============== jti 黑名单 ==============

// TestBlacklistRefreshTokenByJTI_基本功能 按 jti 加入黑名单
func TestBlacklistRefreshTokenByJTI_基本功能(t *testing.T) {
	rdb, _ := setupMiniRedis(t)
	jti := "jti-to-blacklist-001"

	// 1. 初始未黑名单
	blacklisted, err := IsRefreshTokenBlacklisted(rdb, 1001, RoleTenant, jti)
	require.NoError(t, err)
	assert.False(t, blacklisted)

	// 2. 加入黑名单
	err = BlacklistRefreshTokenByJTI(rdb, jti, time.Hour)
	require.NoError(t, err)

	// 3. 检查已黑名单
	blacklisted, err = IsRefreshTokenBlacklisted(rdb, 1001, RoleTenant, jti)
	require.NoError(t, err)
	assert.True(t, blacklisted, "jti 加入黑名单后应识别为已黑名单")
}

// TestBlacklistRefreshTokenByJTI_不影响其他JTI 不同 jti 互不影响
func TestBlacklistRefreshTokenByJTI_不影响其他JTI(t *testing.T) {
	rdb, _ := setupMiniRedis(t)
	jti1 := "jti-device-1"
	jti2 := "jti-device-2"

	// 黑名单 jti1
	err := BlacklistRefreshTokenByJTI(rdb, jti1, time.Hour)
	require.NoError(t, err)

	// jti1 黑名单
	b1, _ := IsRefreshTokenBlacklisted(rdb, 1001, RoleTenant, jti1)
	assert.True(t, b1, "jti1 应被黑名单")

	// jti2 不受影响（关键：精准单点踢出）
	b2, _ := IsRefreshTokenBlacklisted(rdb, 1001, RoleTenant, jti2)
	assert.False(t, b2, "jti2 不应被黑名单（精准单点踢出）")
}

// TestBlacklistRefreshTokenByJTI_同一用户不同设备 该用户其他设备不受影响
func TestBlacklistRefreshTokenByJTI_同一用户不同设备(t *testing.T) {
	rdb, _ := setupMiniRedis(t)
	userID := uint64(2002)
	role := RoleTenant

	// 模拟同一用户两个设备的 jti
	jtiPhone := "jti-phone-001"
	jtiLaptop := "jti-laptop-002"

	// 手机先登录
	err := BlacklistRefreshTokenByJTI(rdb, jtiPhone, time.Hour)
	require.NoError(t, err)

	// 踢出手机（jtiPhone 已黑名单）
	bPhone, _ := IsRefreshTokenBlacklisted(rdb, userID, role, jtiPhone)
	assert.True(t, bPhone, "手机 jti 应被黑名单")

	// 笔记本不受影响（关键：单点踢出 vs v0.3.x 踢整个用户）
	bLaptop, _ := IsRefreshTokenBlacklisted(rdb, userID, role, jtiLaptop)
	assert.False(t, bLaptop, "笔记本 jti 不应被黑名单（v0.4.0 单点踢出）")
}

// TestIsRefreshTokenBlacklisted_兼容旧Token 旧 token 无 jti 时回退 user 维度
func TestIsRefreshTokenBlacklisted_兼容旧Token(t *testing.T) {
	rdb, _ := setupMiniRedis(t)
	userID := uint64(3003)
	role := RoleAgent

	// 1. 旧 token 无 jti，user 维度也未黑名单 → 未黑名单
	b, err := IsRefreshTokenBlacklisted(rdb, userID, role, "")
	require.NoError(t, err)
	assert.False(t, b)

	// 2. user 维度黑名单（修改密码场景）
	err = BlacklistRefreshToken(rdb, userID, role, time.Hour)
	require.NoError(t, err)

	// 3. 旧 token（jti=""）应通过 user 维度回退识别为已黑名单
	b, err = IsRefreshTokenBlacklisted(rdb, userID, role, "")
	require.NoError(t, err)
	assert.True(t, b, "旧 token 无 jti 时应回退 user 维度检查")

	// 4. 新 token（有 jti）不应受 user 维度黑名单影响（除非该 jti 也被黑名单）
	newJTI := "new-jti-after-pwd-change"
	b, err = IsRefreshTokenBlacklisted(rdb, userID, role, newJTI)
	require.NoError(t, err)
	// 注：IsRefreshTokenBlacklisted 会同时检查 jti 和 user 维度，user 维度黑名单时新 token 也会被拦
	// 这是预期行为：修改密码后强制所有设备重登
	assert.True(t, b, "user 维度黑名单时，所有 token（含新 jti）都应被拦截（修改密码场景）")
}

// TestBlacklistRefreshTokenByJTI_空参数 应直接返回 nil 不操作
func TestBlacklistRefreshTokenByJTI_空参数(t *testing.T) {
	rdb, _ := setupMiniRedis(t)

	err := BlacklistRefreshTokenByJTI(rdb, "", time.Hour)
	require.NoError(t, err, "空 jti 应直接返回 nil")

	err = BlacklistRefreshTokenByJTI(rdb, "some-jti", 0)
	require.NoError(t, err, "ttl<=0 应直接返回 nil")

	err = BlacklistRefreshTokenByJTI(nil, "some-jti", time.Hour)
	require.NoError(t, err, "rdb=nil 应直接返回 nil")
}

// TestBlacklistRefreshTokenByJTI_TTL过期 黑名单 TTL 过期后自动失效
func TestBlacklistRefreshTokenByJTI_TTL过期(t *testing.T) {
	rdb, mr := setupMiniRedis(t)
	jti := "jti-ttl-test"

	// 加入黑名单 1 小时
	err := BlacklistRefreshTokenByJTI(rdb, jti, time.Hour)
	require.NoError(t, err)

	// 立即检查：已黑名单
	b, _ := IsRefreshTokenBlacklisted(rdb, 1, RoleAdmin, jti)
	assert.True(t, b)

	// 快进 2 小时（miniredis FastForward 控制 Redis TTL，不影响 Go time）
	mr.FastForward(2 * time.Hour)

	// TTL 过期后：未黑名单
	b, _ = IsRefreshTokenBlacklisted(rdb, 1, RoleAdmin, jti)
	assert.False(t, b, "TTL 过期后黑名单应自动失效")
}

// TestClearRefreshBlacklist 清除 user 维度黑名单
func TestClearRefreshBlacklist(t *testing.T) {
	rdb, _ := setupMiniRedis(t)
	userID := uint64(4004)
	role := RoleAdmin

	// 1. 加入 user 维度黑名单
	err := BlacklistRefreshToken(rdb, userID, role, time.Hour)
	require.NoError(t, err)

	// 2. 已黑名单
	b, _ := IsRefreshTokenBlacklisted(rdb, userID, role, "")
	assert.True(t, b)

	// 3. 清除
	err = ClearRefreshBlacklist(rdb, userID, role)
	require.NoError(t, err)

	// 4. 未黑名单
	b, _ = IsRefreshTokenBlacklisted(rdb, userID, role, "")
	assert.False(t, b)
}

// ============== ExtractBearer ==============

// TestExtractBearer 正常提取
func TestExtractBearer(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{"正常 Bearer", "Bearer abc.def.ghi", "abc.def.ghi", false},
		{"空字符串", "", "", true},
		{"无 Bearer 前缀", "abc.def.ghi", "", true},
		{"Basic 前缀", "Basic abc", "", true},
		{"Bearer 后空", "Bearer ", "", false}, // TrimPrefix 后为空字符串，不报错
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ExtractBearer(tc.input)
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.want, got)
			}
		})
	}
}

// ============== 端到端：登录 → 踢设备 → 重检 ==============

// TestJTI黑名单端到端 模拟登录 → 踢设备 → 该 jti 失效但其他 jti 不受影响
func TestJTI黑名单端到端(t *testing.T) {
	rdb, _ := setupMiniRedis(t)
	userID := uint64(5005)
	role := RoleTenant
	secret := "e2e-test-secret-32-bytes-needed!!"

	// 1. 用户在手机登录（生成 jti1）
	jti1 := "phone-jti-e2e-001"
	pair1, err := GenerateTokenPair(TokenOptions{
		Secret:     secret,
		UserID:     userID,
		Username:   "e2e-user",
		Role:       role,
		TenantID:   userID,
		AccessTTL:  time.Hour,
		RefreshTTL: 24 * time.Hour,
		JTI:        jti1,
	})
	require.NoError(t, err)

	// 2. 用户在笔记本登录（生成 jti2）
	jti2 := "laptop-jti-e2e-002"
	pair2, err := GenerateTokenPair(TokenOptions{
		Secret:     secret,
		UserID:     userID,
		Username:   "e2e-user",
		Role:       role,
		TenantID:   userID,
		AccessTTL:  time.Hour,
		RefreshTTL: 24 * time.Hour,
		JTI:        jti2,
	})
	require.NoError(t, err)

	// 3. 两个 token 都未黑名单
	b1, _ := IsRefreshTokenBlacklisted(rdb, userID, role, jti1)
	assert.False(t, b1)
	b2, _ := IsRefreshTokenBlacklisted(rdb, userID, role, jti2)
	assert.False(t, b2)

	// 4. 解析 token 验证 jti 正确写入
	claims1, _, err := ParseToken(secret, pair1.AccessToken)
	require.NoError(t, err)
	assert.Equal(t, jti1, claims1.ID)

	claims2, _, err := ParseToken(secret, pair2.AccessToken)
	require.NoError(t, err)
	assert.Equal(t, jti2, claims2.ID)

	// 5. 踢出手机（jti1 加入黑名单）
	err = BlacklistRefreshTokenByJTI(rdb, jti1, 24*time.Hour)
	require.NoError(t, err)

	// 6. 验证：手机 jti1 已黑名单，笔记本 jti2 不受影响
	b1, _ = IsRefreshTokenBlacklisted(rdb, userID, role, jti1)
	assert.True(t, b1, "手机 jti1 应被黑名单")
	b2, _ = IsRefreshTokenBlacklisted(rdb, userID, role, jti2)
	assert.False(t, b2, "笔记本 jti2 不应受影响（v0.4.0 精准单点踢出）")

	// 7. 模拟修改密码场景：BlacklistRefreshToken(user 维度) 踢所有设备
	err = BlacklistRefreshToken(rdb, userID, role, 24*time.Hour)
	require.NoError(t, err)

	// 8. 验证：两个 jti 都被拦截（user 维度黑名单覆盖所有）
	b1, _ = IsRefreshTokenBlacklisted(rdb, userID, role, jti1)
	assert.True(t, b1, "修改密码后 jti1 应被拦截")
	b2, _ = IsRefreshTokenBlacklisted(rdb, userID, role, jti2)
	assert.True(t, b2, "修改密码后 jti2 也应被拦截（强制所有设备重登）")
}

// ============== middleware.JWTClaims 集成验证 ==============

// TestJWTClaims_JTI通过RegisteredClaims middleware.JWTClaims 通过 RegisteredClaims.ID 携带 jti
func TestJWTClaims_JTI通过RegisteredClaims(t *testing.T) {
	// 验证 JWTClaims 嵌入了 jwt.RegisteredClaims，ID 字段可访问
	claims := middleware.JWTClaims{
		UserID:   1,
		Username: "test",
		Role:     RoleAdmin,
	}
	claims.ID = "test-jti-via-registered"

	assert.Equal(t, "test-jti-via-registered", claims.ID, "JWTClaims 应能通过 RegisteredClaims.ID 携带 jti")
	assert.Equal(t, uint64(1), claims.UserID)
}
