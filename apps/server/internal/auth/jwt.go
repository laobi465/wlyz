// Package auth 认证工具集
// 提供 JWT 双 Token 机制 / TOTP 2FA / 登录失败计数
// 严格遵循铁律 04/05：所有可变参数从 sys_config 读取，禁止硬编码
package auth

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/redis/go-redis/v9"

	"github.com/your-org/keyauth-saas/apps/server/internal/middleware"
)

// TokenType Token 类型
type TokenType string

const (
	TokenTypeAccess  TokenType = "access"
	TokenTypeRefresh TokenType = "refresh"
)

// 角色常量（与 middleware.JWTClaims.Role 一致）
const (
	RoleAdmin  = "admin"
	RoleTenant = "tenant"
	RoleAgent  = "agent"
)

// TokenPair 双 Token 返回结构
type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`             // access token 有效期（秒）
	RefreshIn    int64  `json:"refresh_in"`             // refresh token 有效期（秒）
	TokenType    string `json:"token_type"`             // 固定 "Bearer"
}

// TokenOptions 生成 Token 的参数
type TokenOptions struct {
	Secret        string // JWT 签名密钥
	Issuer        string // 签发者
	UserID        uint64
	Username      string
	Role          string
	TenantID      uint64
	AccessTTL     time.Duration // access token 有效期
	RefreshTTL    time.Duration // refresh token 有效期
	JTI           string        // v0.4.0：JWT ID，用于精准单点踢出（同一会话的 access + refresh 共享同一 jti）
}

// GenerateTokenPair 生成 access + refresh 双 Token
// refresh token 的 subject 固定为 "refresh"，无法用于业务接口
// v0.4.0：access + refresh 携带同一 jti，登出/踢设备时按 jti 黑名单（不再踢整个用户）
func GenerateTokenPair(opt TokenOptions) (*TokenPair, error) {
	if opt.Secret == "" {
		return nil, errors.New("JWT 密钥不能为空")
	}
	now := time.Now()

	// access token
	accessClaims := &middleware.JWTClaims{
		UserID:   opt.UserID,
		Username: opt.Username,
		Role:     opt.Role,
		TenantID: opt.TenantID,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        opt.JTI,
			Issuer:    opt.Issuer,
			Subject:   string(TokenTypeAccess),
			ExpiresAt: jwt.NewNumericDate(now.Add(opt.AccessTTL)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
		},
	}
	accessToken := jwt.NewWithClaims(jwt.SigningMethodHS256, accessClaims)
	accessStr, err := accessToken.SignedString([]byte(opt.Secret))
	if err != nil {
		return nil, fmt.Errorf("签发 access token 失败: %w", err)
	}

	// refresh token（不携带业务字段，仅携带 user_id + role + jti）
	refreshClaims := &middleware.JWTClaims{
		UserID: opt.UserID,
		Role:   opt.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        opt.JTI,
			Issuer:    opt.Issuer,
			Subject:   string(TokenTypeRefresh),
			ExpiresAt: jwt.NewNumericDate(now.Add(opt.RefreshTTL)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
		},
	}
	refreshToken := jwt.NewWithClaims(jwt.SigningMethodHS256, refreshClaims)
	refreshStr, err := refreshToken.SignedString([]byte(opt.Secret))
	if err != nil {
		return nil, fmt.Errorf("签发 refresh token 失败: %w", err)
	}

	return &TokenPair{
		AccessToken:  accessStr,
		RefreshToken: refreshStr,
		ExpiresIn:    int64(opt.AccessTTL.Seconds()),
		RefreshIn:    int64(opt.RefreshTTL.Seconds()),
		TokenType:    "Bearer",
	}, nil
}

// ParseToken 解析并校验 Token
// 返回 claims 和 token 类型；如果 token 过期或签名错误返回 error
func ParseToken(secret, tokenStr string) (*middleware.JWTClaims, TokenType, error) {
	claims := &middleware.JWTClaims{}
	token, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrTokenSignatureInvalid
		}
		return []byte(secret), nil
	})
	if err != nil || !token.Valid {
		return nil, "", fmt.Errorf("token 无效: %w", err)
	}
	if claims.Subject == string(TokenTypeRefresh) {
		return claims, TokenTypeRefresh, nil
	}
	return claims, TokenTypeAccess, nil
}

// ====================== Refresh Token 黑名单 ======================
// 用途：登出 / 修改密码 / 踢设备后让指定 refresh token 失效
// Redis Key:
//   - 按 jti 维度（v0.4.0 新增，精准单点踢出）：auth:refresh:blacklist:jti:{jti}
//   - 按 user 维度（v0.3.x 旧接口，踢整个用户所有会话）：auth:refresh:blacklist:{role}:{user_id}
// TTL: 与 refresh token 剩余有效期一致（避免无限占用内存）

// BlacklistRefreshToken 将指定用户的 refresh token 加入黑名单（按 user 维度）
// v0.3.x 行为：踢出该用户所有设备（用于修改密码 / 关闭 2FA 等需要强制所有设备重登的场景）
// v0.4.0 起单点踢出请用 BlacklistRefreshTokenByJTI
func BlacklistRefreshToken(rdb *redis.Client, userID uint64, role string, ttl time.Duration) error {
	if rdb == nil || ttl <= 0 {
		return nil
	}
	key := fmt.Sprintf("auth:refresh:blacklist:%s:%d", role, userID)
	return rdb.Set(nilCtx, key, "1", ttl).Err()
}

// IsRefreshTokenBlacklisted 检查 refresh token 是否已被吊销
// v0.4.0：优先按 jti 检查；jti 为空时回退 user 维度（兼容旧 token）
func IsRefreshTokenBlacklisted(rdb *redis.Client, userID uint64, role, jti string) (bool, error) {
	if rdb == nil {
		return false, nil
	}
	// 1. 按 jti 检查（v0.4.0 新增）
	if jti != "" {
		jtiKey := fmt.Sprintf("auth:refresh:blacklist:jti:%s", jti)
		n, err := rdb.Exists(nilCtx, jtiKey).Result()
		if err != nil {
			return false, err
		}
		if n > 0 {
			return true, nil
		}
	}
	// 2. 回退 user 维度（兼容 v0.3.x 签发的旧 token，无 jti）
	key := fmt.Sprintf("auth:refresh:blacklist:%s:%d", role, userID)
	n, err := rdb.Exists(nilCtx, key).Result()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// BlacklistRefreshTokenByJTI 按 jti 黑名单单个 refresh token（v0.4.0 新增）
// 用于登出 / 踢设备：仅让指定会话失效，不影响该用户其他设备
func BlacklistRefreshTokenByJTI(rdb *redis.Client, jti string, ttl time.Duration) error {
	if rdb == nil || jti == "" || ttl <= 0 {
		return nil
	}
	key := fmt.Sprintf("auth:refresh:blacklist:jti:%s", jti)
	return rdb.Set(nilCtx, key, "1", ttl).Err()
}

// ClearRefreshBlacklist 清除黑名单（用于登录成功后清理旧的会话标记）
func ClearRefreshBlacklist(rdb *redis.Client, userID uint64, role string) error {
	if rdb == nil {
		return nil
	}
	key := fmt.Sprintf("auth:refresh:blacklist:%s:%d", role, userID)
	return rdb.Del(nilCtx, key).Err()
}

// ExtractBearer 从 Authorization 头提取纯 Token 字符串
func ExtractBearer(authHeader string) (string, error) {
	if authHeader == "" {
		return "", errors.New("未提供 Token")
	}
	if !strings.HasPrefix(authHeader, "Bearer ") {
		return "", errors.New("Token 格式错误")
	}
	return strings.TrimPrefix(authHeader, "Bearer "), nil
}
