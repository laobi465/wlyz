// Package middleware HTTP 中间件集合
// 包含 JWT 鉴权 / 多租户隔离 / HMAC 签名校验 / 限流 / 日志 / 跨域
package middleware

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
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

		// 3. 角色校验
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

		// 4. 注入上下文（v0.4.0：注入 jti 供下游单点踢出使用）
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
// 调用方如需携带 jti，应在传入 claims 前设置 claims.ID
func GenerateToken(secret, issuer string, expireHours int, claims JWTClaims) (string, error) {
	// 保留 jti（claims.ID），仅覆盖签发相关字段
	jti := claims.ID
	claims.RegisteredClaims = jwt.RegisteredClaims{
		ID:        jti,
		Issuer:    issuer,
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Duration(expireHours) * time.Hour)),
		IssuedAt:  jwt.NewNumericDate(time.Now()),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}
