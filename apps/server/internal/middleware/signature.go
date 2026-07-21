// HMAC-SHA256 签名校验中间件
// 用于客户端验证 API（/api/v1/client/*）
// 严格按 references/07 + 布丁卡密规范实现
package middleware

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/your-org/keyauth-saas/apps/server/internal/model"
	"github.com/your-org/keyauth-saas/apps/server/pkg/crypto"
	"gorm.io/gorm"
)

// 签名相关常量（铁律 04：不硬编码，从 sys_config 读取）
const (
	// 时间戳允许偏差（秒），默认 300 秒（±5 分钟）
	configKeyTimestampTolerance = "security.sign.timestamp_tolerance"
	// Nonce 防重放过期（秒），默认 300 秒
	configKeyNonceExpire = "security.sign.nonce_expire"
)

// SignatureAuth HMAC 签名验证中间件
// 流程：
// 1. 提取 X-App-Key / X-Timestamp / X-Nonce / X-Signature
// 2. 查应用，获取 sign_secret（解密）
// 3. 校验时间戳偏差
// 4. 校验 Nonce 未被使用过（Redis 5 分钟去重）
// 5. 重算签名，常量时间比较
func SignatureAuth(db *gorm.DB, rdb *redis.Client, cfgReader ConfigReader) gin.HandlerFunc {
	return func(c *gin.Context) {
		appKey := c.GetHeader("X-App-Key")
		timestamp := c.GetHeader("X-Timestamp")
		nonce := c.GetHeader("X-Nonce")
		signature := c.GetHeader("X-Signature")

		// 1. 参数完整性
		if appKey == "" || timestamp == "" || nonce == "" || signature == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": 1002, "message": "签名参数缺失"})
			return
		}

		// 2. 查应用（含 SignSecret 与 SignSecretPrev）
		var app model.App
		if err := db.Where("app_key = ? AND status = ?", appKey, "active").First(&app).Error; err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": 3001, "message": "应用不存在或已禁用"})
			return
		}

		// 3. 时间戳校验
		ts, err := strconv.ParseInt(timestamp, 10, 64)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": 1001, "message": "时间戳格式错误"})
			return
		}
		tolerance := cfgReader.GetInt(c.Request.Context(), configKeyTimestampTolerance, 300)
		now := time.Now().Unix()
		if now-ts > int64(tolerance) || ts-now > int64(tolerance) {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": 1001, "message": "时间戳超出允许范围"})
			return
		}

		// 4. 读取请求体（用于签名）
		bodyBytes, _ := io.ReadAll(c.Request.Body)
		c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes)) // 重置 body

		// 5. 构造签名原文：METHOD\nPATH?QUERY\nTIMESTAMP\nNONCE\nBODY
		pathQuery := c.Request.URL.Path
		if c.Request.URL.RawQuery != "" {
			pathQuery += "?" + c.Request.URL.RawQuery
		}
		signString := strings.Join([]string{
			c.Request.Method,
			pathQuery,
			timestamp,
			nonce,
			string(bodyBytes),
		}, "\n")

		// 6. 解密 SignSecret 并计算签名
		// 注：crypto.Manager 需通过依赖注入获取（简化版直接全局，正式版应注入）
		cryptoMgr := GetCryptoManager()
		if cryptoMgr == nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"code": 1006, "message": "加密管理器未初始化"})
			return
		}
		signSecret, err := cryptoMgr.DecryptAES(app.SignSecret)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"code": 1006, "message": "签名密钥解密失败"})
			return
		}
		// 注：使用 HMACSHA512_256（SHA-512/256 变体）与客户端 SDK 对齐
		// SDK（csharp/java/php/go/python/node/cpp）均基于 sha512.New512_256 实现
		expectedSig := crypto.HMACSHA512_256(signSecret, []byte(signString))

		// 7. 常量时间比较（防时序攻击），失败时尝试旧密钥（轮换期）
		sigOK := crypto.HMACEqual(expectedSig, signature)
		if !sigOK && app.SignSecretPrev != "" {
			if oldSecret, err := cryptoMgr.DecryptAES(app.SignSecretPrev); err == nil {
				oldSig := crypto.HMACSHA512_256(oldSecret, []byte(signString))
				sigOK = crypto.HMACEqual(oldSig, signature)
			}
		}
		if !sigOK {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": 1002, "message": "签名校验失败"})
			return
		}

		// 8. Nonce 防重放（P1-02 修复：移到签名校验通过后再 SetNX）
		// 原实现先 SetNX 再校验签名，攻击者构造大量随机 nonce + 错误签名即可污染 Redis nonce 命名空间。
		// 调整顺序：签名校验通过后再写 nonce，确保只有合法请求才会占用 nonce 槽位。
		nonceExpire := cfgReader.GetInt(c.Request.Context(), configKeyNonceExpire, 300)
		nonceKey := "nonce:" + nonce
		ok, err := rdb.SetNX(c.Request.Context(), nonceKey, 1, time.Duration(nonceExpire)*time.Second).Result()
		if err != nil || !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": 1001, "message": "请求已过期或重复"})
			return
		}

		// 9. 注入应用上下文
		c.Set("app_id", app.ID)
		c.Set("tenant_id", app.TenantID)
		c.Set("app", &app)
		c.Next()
	}
}

// ConfigReader 配置读取接口（避免循环依赖）
type ConfigReader interface {
	GetString(ctx context.Context, key, fallback string) string
	GetInt(ctx context.Context, key string, fallback int) int
	GetInt64(ctx context.Context, key string, fallback int64) int64
	GetBool(ctx context.Context, key string, fallback bool) bool
}

// 全局 CryptoManager 引用（由 main 注入）
// 注：正式实现应通过依赖注入，这里简化为全局变量
var globalCryptoMgr *crypto.Manager

// SetCryptoManager 由 main 启动时注入
func SetCryptoManager(m *crypto.Manager) {
	globalCryptoMgr = m
}

// GetCryptoManager 获取
func GetCryptoManager() *crypto.Manager {
	return globalCryptoMgr
}
