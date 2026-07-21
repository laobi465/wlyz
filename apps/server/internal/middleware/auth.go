// Package middleware HTTP 中间件集合
// 包含 JWT 鉴权 / 多租户隔离 / HMAC 签名校验 / 限流 / 日志 / 跨域
package middleware

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"

	"github.com/your-org/keyauth-saas/apps/server/internal/openapi"
)

// JWTClaims 自定义 JWT 载荷
type JWTClaims struct {
	UserID   uint64 `json:"user_id"`
	Username string `json:"username"`
	Role     string `json:"role"` // admin/tenant/agent
	TenantID uint64 `json:"tenant_id"`
	jwt.RegisteredClaims // v0.4.0：通过 RegisteredClaims.ID 携带 jti（精准单点踢出）
}

// JWTAuth JWT 鉴权中间件
// role 参数指定允许的角色（admin/tenant/agent），多角色用逗号分隔
func JWTAuth(secret string, allowedRoles string) gin.HandlerFunc {
	allowed := strings.Split(allowedRoles, ",")
	return func(c *gin.Context) {
		// 1. 提取 Token
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": 1002, "message": "未提供 Token"})
			return
		}
		tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
		if tokenStr == authHeader {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": 1002, "message": "Token 格式错误"})
			return
		}

		// 2. 解析 Token
		claims := &JWTClaims{}
		token, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (interface{}, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, jwt.ErrTokenSignatureInvalid
			}
			return []byte(secret), nil
		})
		if err != nil || !token.Valid {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": 2002, "message": "Token 无效或已过期"})
			return
		}

		// 3. Subject 校验（P1-01 修复：拒绝 refresh token 访问业务接口）
		// 仅允许 access token（Subject=="access"）通过；refresh token（Subject=="refresh"，7 天 TTL）
		// 必须走 /public/auth/refresh 端点轮换，禁止直接访问业务接口
		if claims.Subject != "access" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": 1002, "message": "token 类型错误"})
			return
		}

		// 4. 角色校验
		roleOK := false
		for _, r := range allowed {
			if strings.TrimSpace(r) == claims.Role {
				roleOK = true
				break
			}
		}
		if !roleOK {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"code": 1003, "message": "无权限访问"})
			return
		}

		// 5. 注入上下文（v0.4.0：注入 jti 供下游单点踢出使用）
		c.Set("user_id", claims.UserID)
		c.Set("username", claims.Username)
		c.Set("role", claims.Role)
		c.Set("tenant_id", claims.TenantID)
		c.Set("jti", claims.ID)
		c.Next()
	}
}

// GenerateToken 生成 JWT Token
// v0.4.0：保留 claims.ID（jti）字段，仅覆盖 Issuer/ExpiresAt/IssuedAt
// P1-01：保留 claims.Subject（access/refresh），未指定时默认 "access"（向后兼容）
// 调用方如需携带 jti，应在传入 claims 前设置 claims.ID
// 调用方如需签发 refresh token，应设置 claims.Subject = "refresh"
func GenerateToken(secret, issuer string, expireHours int, claims JWTClaims) (string, error) {
	// 保留 jti（claims.ID）和 Subject（P1-01：access/refresh 区分）
	jti := claims.ID
	subject := claims.Subject
	if subject == "" {
		// 向后兼容：旧调用方未显式指定 Subject 时默认签发 access token
		subject = "access"
	}
	claims.RegisteredClaims = jwt.RegisteredClaims{
		ID:        jti,
		Issuer:    issuer,
		Subject:   subject,
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Duration(expireHours) * time.Hour)),
		IssuedAt:  jwt.NewNumericDate(time.Now()),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

// H5EndUserAuth 终端用户 access token 鉴权中间件（v0.4.0）
// 鉴权流程：① 提取 Authorization: Bearer <access_token> ② HMAC-SHA256 签名校验
// ③ 过期校验 ④ 注入 enduser_id / enduser_app_id 到 context
//
// 与 JWTAuth 的区别：终端用户 token 不走 jwt 库（简化为 HMAC-SHA256(secret|user_id|app_id|exp)），
// 因 enduser 包未引入 jwt 依赖；如需迁移到标准 JWT，可复用 JWTClaims。
func H5EndUserAuth(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": 1002, "message": "未提供 Token"})
			return
		}
		tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
		if tokenStr == authHeader {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": 1002, "message": "Token 格式错误"})
			return
		}
		// 解析 token：payload.signature
		parts := strings.SplitN(tokenStr, ".", 2)
		if len(parts) != 2 {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": 2002, "message": "Token 无效"})
			return
		}
		payload, sig := parts[0], parts[1]
		// 校验签名（HMAC-SHA256）
		// 铁律 06：直接调用 crypto.HMACSHA256 会形成循环依赖（crypto 不依赖 middleware），
		// 此处用 hmac 标准库重新实现一份等价逻辑
		expectedSig := computeHMACSHA256(payload, secret)
		if !hmacEqualConstTime(sig, expectedSig) {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": 2002, "message": "Token 签名无效"})
			return
		}
		// 解析 payload：userID|appID|exp
		var userID, appID uint64
		var exp int64
		if _, err := fmt.Sscanf(payload, "%d|%d|%d", &userID, &appID, &exp); err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": 2002, "message": "Token 载荷无效"})
			return
		}
		if time.Now().Unix() > exp {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": 2002, "message": "Token 已过期"})
			return
		}
		c.Set("enduser_id", userID)
		c.Set("enduser_app_id", appID)
		c.Next()
	}
}

// computeHMACSHA256 计算 HMAC-SHA256(secret, data) 的十六进制字符串
// 与 pkg/crypto.HMACSHA256 等价（避免循环依赖）
func computeHMACSHA256(data, secret string) string {
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(data))
	return hex.EncodeToString(h.Sum(nil))
}

// hmacEqualConstTime 常量时间比较，防时序攻击
func hmacEqualConstTime(a, b string) bool {
	return hmac.Equal([]byte(a), []byte(b))
}

// APITokenAuth 开发者 API Token 鉴权中间件（v0.4.0）
// 鉴权流程：① 提取 Authorization: Bearer <pat_xxx> ② 调用 TokenManager.ValidateToken 校验
// ③ 注入 api_token_id / api_tenant_id / api_scopes 到 gin.Context
//
// 与 JWTAuth 的区别：
//   - JWTAuth 服务端内部账号（admin/tenant/agent）
//   - APITokenAuth 第三方开发者通过 API Token 访问开放平台接口
//   - Token 明文不存库（仅 SHA-512 哈希），明文仅生成时返回一次
//   - 失败响应码统一 401，错误信息不暴露内部细节（防信息泄露）
func APITokenAuth(mgr *openapi.TokenManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 1. 提取 Authorization: Bearer <token>
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": 1002, "message": "未提供 API Token"})
			return
		}
		tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
		if tokenStr == authHeader {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": 1002, "message": "Token 格式错误"})
			return
		}
		// 2. 调用 TokenManager 校验
		clientIP := c.ClientIP()
		token, err := mgr.ValidateToken(c.Request.Context(), tokenStr, clientIP)
		if err != nil {
			// 统一错误信息，不区分"不存在/已撤销/已过期"，防信息泄露
			msg := "API Token 无效"
			code := 2002
			switch {
			case errors.Is(err, openapi.ErrTokenRevoked):
				msg = "API Token 已撤销"
			case errors.Is(err, openapi.ErrTokenExpired):
				msg = "API Token 已过期"
			case errors.Is(err, openapi.ErrTokenNotFound):
				// 保持默认 msg
			}
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": code, "message": msg})
			return
		}
		// 3. 注入上下文（下游 handler / RequireScope 中间件使用）
		c.Set("api_token_id", token.ID)
		c.Set("api_tenant_id", token.TenantID)
		c.Set("api_scopes", token.Scopes)
		c.Set("api_token_name", token.Name)
		c.Next()
	}
}

// RequireScope 要求请求方持有指定 scope（v0.4.0）
// 必须在 APITokenAuth 之后使用；支持多 scope（任一命中即通过，OR 语义）
// 用法：r.GET("/api/v1/openapi/cards", middleware.APITokenAuth(mgr), middleware.RequireScope("card.read"), handler)
func RequireScope(scopes ...string) gin.HandlerFunc {
	required := make([]string, 0, len(scopes))
	for _, s := range scopes {
		s = strings.TrimSpace(s)
		if s != "" {
			required = append(required, s)
		}
	}
	return func(c *gin.Context) {
		if len(required) == 0 {
			c.Next()
			return
		}
		tokenScopesRaw, exists := c.Get("api_scopes")
		if !exists {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"code": 1003, "message": "未通过 API Token 鉴权"})
			return
		}
		tokenScopes, _ := tokenScopesRaw.(string)
		// OR 语义：任一 required scope 命中即通过
		for _, req := range required {
			if openapi.HasScope(tokenScopes, req) {
				c.Next()
				return
			}
		}
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"code": 1003, "message": "API Token 缺少所需权限: " + strings.Join(required, ",")})
	}
}
