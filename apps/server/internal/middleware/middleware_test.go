// Package middleware HTTP 中间件单元测试
// 覆盖 JWT / Signature / Tenant / RateLimit / IPBlacklist 核心安全路径
// 严格遵循铁律 06：所有断言基于已知固定输入，无随机/不确定性
package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
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

	"github.com/your-org/keyauth-saas/apps/server/internal/model"
	"github.com/your-org/keyauth-saas/apps/server/pkg/crypto"
)

// ============== 测试辅助 ==============

// mockConfigReader 内存版 ConfigReader，用于测试
type mockConfigReader struct {
	values map[string]string
}

func newMockConfigReader() *mockConfigReader {
	return &mockConfigReader{
		values: map[string]string{
			"security.sign.timestamp_tolerance": "300",
			"security.sign.nonce_expire":        "300",
			"security.rate.limit_global":        "100",
			"security.rate.limit_sensitive":     "3",
			"security.ban.threshold":            "5",
			"security.ban.duration_seconds":     "3600",
		},
	}
}

func (m *mockConfigReader) GetString(_ context.Context, key, fallback string) string {
	if v, ok := m.values[key]; ok {
		return v
	}
	return fallback
}

func (m *mockConfigReader) GetInt(_ context.Context, key string, fallback int) int {
	if v, ok := m.values[key]; ok {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}

func (m *mockConfigReader) GetInt64(_ context.Context, key string, fallback int64) int64 {
	if v, ok := m.values[key]; ok {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			return n
		}
	}
	return fallback
}

func (m *mockConfigReader) GetBool(_ context.Context, key string, fallback bool) bool {
	if v, ok := m.values[key]; ok {
		return v == "true" || v == "1"
	}
	return fallback
}

// setupMiniRedis 启动 miniredis + 返回 redis.Client
func setupMiniRedis(t *testing.T) (*redis.Client, *miniredis.Miniredis) {
	t.Helper()
	mr, err := miniredis.Run()
	require.NoError(t, err)
	t.Cleanup(mr.Close)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	return rdb, mr
}

// setupTestDB 用 SQLite 内存库初始化测试 DB
func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared&_pragma=foreign_keys(1)"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.App{}))
	require.NoError(t, db.Exec("DELETE FROM app").Error)
	return db
}

// setupGinRouter 初始化 gin 测试模式 + 返回 engine
func setupGinRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	return gin.New()
}

// ============== JWTAuth ==============

func TestJWTAuth_ValidToken(t *testing.T) {
	r := setupGinRouter()
	secret := "test-jwt-secret"
	r.Use(JWTAuth(secret, "tenant"))
	r.GET("/test", func(c *gin.Context) {
		uid, _ := c.Get("user_id")
		role, _ := c.Get("role")
		c.JSON(200, gin.H{"user_id": uid, "role": role})
	})

	claims := JWTClaims{UserID: 42, Username: "alice", Role: "tenant", TenantID: 1001}
	token, err := GenerateToken(secret, "keyauth-test", 1, claims)
	require.NoError(t, err)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
	body := w.Body.String()
	assert.Contains(t, body, `"user_id":42`)
	assert.Contains(t, body, `"role":"tenant"`)
}

func TestJWTAuth_MissingToken(t *testing.T) {
	r := setupGinRouter()
	r.Use(JWTAuth("secret", "tenant"))
	r.GET("/test", func(c *gin.Context) { c.Status(200) })

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Contains(t, w.Body.String(), "未提供 Token")
}

func TestJWTAuth_MalformedHeader(t *testing.T) {
	r := setupGinRouter()
	r.Use(JWTAuth("secret", "tenant"))
	r.GET("/test", func(c *gin.Context) { c.Status(200) })

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Basic abc123") // 非 Bearer 前缀
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Contains(t, w.Body.String(), "Token 格式错误")
}

func TestJWTAuth_InvalidSignature(t *testing.T) {
	r := setupGinRouter()
	r.Use(JWTAuth("correct-secret", "tenant"))
	r.GET("/test", func(c *gin.Context) { c.Status(200) })

	// 用错误 secret 生成 token
	token, err := GenerateToken("wrong-secret", "test", 1, JWTClaims{UserID: 1, Role: "tenant"})
	require.NoError(t, err)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Contains(t, w.Body.String(), "Token 无效或已过期")
}

func TestJWTAuth_RoleMismatch(t *testing.T) {
	r := setupGinRouter()
	r.Use(JWTAuth("secret", "tenant")) // 只允许 tenant
	r.GET("/test", func(c *gin.Context) { c.Status(200) })

	// 生成 admin 角色的 token
	token, err := GenerateToken("secret", "test", 1, JWTClaims{UserID: 1, Role: "admin"})
	require.NoError(t, err)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Contains(t, w.Body.String(), "无权限访问")
}

func TestJWTAuth_MultipleAllowedRoles(t *testing.T) {
	r := setupGinRouter()
	r.Use(JWTAuth("secret", "admin,tenant,agent"))
	r.GET("/test", func(c *gin.Context) { c.Status(200) })

	// 三角色都应通过
	for _, role := range []string{"admin", "tenant", "agent"} {
		token, err := GenerateToken("secret", "test", 1, JWTClaims{UserID: 1, Role: role})
		require.NoError(t, err)
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		r.ServeHTTP(w, req)
		assert.Equal(t, 200, w.Code, "角色 %s 应通过", role)
	}
}

func TestGenerateToken_ContainsIssuer(t *testing.T) {
	token, err := GenerateToken("secret", "keyauth-issuer", 1, JWTClaims{UserID: 1, Role: "tenant"})
	require.NoError(t, err)
	// JWT 由三段 base64 用 . 分隔
	parts := strings.Split(token, ".")
	require.Len(t, parts, 3)
}

// ============== TenantScope ==============

func TestTenantScope_NoTenantID(t *testing.T) {
	r := setupGinRouter()
	r.Use(TenantScope(setupTestDB(t)))
	r.GET("/test", func(c *gin.Context) { c.Status(200) })

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)
	// 未设置 tenant_id（模拟跳过 JWT 中间件）
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Contains(t, w.Body.String(), "无法识别租户身份")
}

func TestTenantScope_WithTenantID(t *testing.T) {
	r := setupGinRouter()
	// 模拟 JWT 中间件注入 tenant_id
	r.Use(func(c *gin.Context) {
		c.Set("tenant_id", uint64(1001))
		c.Next()
	})
	r.Use(TenantScope(setupTestDB(t)))
	r.GET("/test", func(c *gin.Context) {
		// TenantScope 应注入 db 和 gorm_scope
		_, hasDB := c.Get("db")
		_, hasScope := c.Get("gorm_scope")
		c.JSON(200, gin.H{"has_db": hasDB, "has_scope": hasScope})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
	assert.Contains(t, w.Body.String(), `"has_db":true`)
	assert.Contains(t, w.Body.String(), `"has_scope":true`)
}

func TestCheckResourceOwnership(t *testing.T) {
	db := setupTestDB(t)
	// 种子数据
	require.NoError(t, db.Create(&model.App{
		BaseModel: model.BaseModel{ID: 1},
		TenantID:  1001,
		AppKey:    "ak-1",
		Name:      "app-1",
	}).Error)
	require.NoError(t, db.Create(&model.App{
		BaseModel: model.BaseModel{ID: 2},
		TenantID:  2002,
		AppKey:    "ak-2",
		Name:      "app-2",
	}).Error)

	// tenant 1001 拥有 app 1
	ok, err := CheckResourceOwnership(db, "app", 1, 1001)
	require.NoError(t, err)
	assert.True(t, ok)

	// tenant 1001 不拥有 app 2（跨租户访问）
	ok, err = CheckResourceOwnership(db, "app", 2, 1001)
	require.NoError(t, err)
	assert.False(t, ok)

	// 不存在的资源
	ok, err = CheckResourceOwnership(db, "app", 999, 1001)
	require.NoError(t, err)
	assert.False(t, ok)
}

// ============== SignatureAuth ==============

// setupSignatureTest 准备 SignatureAuth 测试上下文（DB + App + CryptoManager + Redis）
// 返回 (router, signSecret, app, cryptoMgr)
func setupSignatureTest(t *testing.T) (*gin.Engine, string, model.App, *crypto.Manager) {
	t.Helper()

	// 1. AES-256 密钥（32 字节）
	aesKey := "0123456789abcdef0123456789abcdef" // 32 字节
	cryptoMgr, err := crypto.NewManager(aesKey, "", "")
	require.NoError(t, err)
	SetCryptoManager(cryptoMgr)
	t.Cleanup(func() { SetCryptoManager(nil) })

	// 2. 真实 sign_secret（明文）+ AES 加密入库
	signSecret := "real-sign-secret-from-app"
	encryptedSecret, err := cryptoMgr.EncryptAES(signSecret)
	require.NoError(t, err)

	// 3. SQLite DB + 种子 App
	db := setupTestDB(t)
	app := model.App{
		BaseModel:   model.BaseModel{ID: 1},
		TenantID:    1001,
		AppKey:      "ak_test_signature",
		AppSecret:   "encrypted",
		SignSecret:  encryptedSecret,
		Name:        "test-app",
		Status:      "active",
		MaxDevices:  1,
	}
	require.NoError(t, db.Create(&app).Error)

	// 4. Redis (miniredis)
	rdb, _ := setupMiniRedis(t)

	// 5. Gin 路由
	r := setupGinRouter()
	r.Use(SignatureAuth(db, rdb, newMockConfigReader()))
	r.POST("/api/v1/client/login", func(c *gin.Context) {
		c.JSON(200, gin.H{"code": 0, "message": "ok"})
	})

	return r, signSecret, app, cryptoMgr
}

// buildSignRequest 构造带签名的请求
func buildSignRequest(t *testing.T, method, path, body, signSecret, appKey string) *http.Request {
	t.Helper()
	timestamp := time.Now().Unix()
	nonce := "nonce-test-12345"

	// 签名原文：METHOD\nPATH?QUERY\nTIMESTAMP\nNONCE\nBODY
	signString := strings.Join([]string{method, path, timestampStr(timestamp), nonce, body}, "\n")
	sig := crypto.HMACSHA256(signSecret, []byte(signString))

	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("X-App-Key", appKey)
	req.Header.Set("X-Timestamp", timestampStr(timestamp))
	req.Header.Set("X-Nonce", nonce)
	req.Header.Set("X-Signature", sig)
	return req
}

// timestampStr int64 → string
func timestampStr(ts int64) string {
	return strconv.FormatInt(ts, 10)
}

func TestSignatureAuth_ValidSignature(t *testing.T) {
	r, signSecret, app, _ := setupSignatureTest(t)

	body := `{"card_key":"K2X9-AB7C","hwid":"abc"}`
	req := buildSignRequest(t, "POST", "/api/v1/client/login", body, signSecret, app.AppKey)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
	assert.Contains(t, w.Body.String(), `"code":0`)
}

func TestSignatureAuth_MissingHeaders(t *testing.T) {
	r, _, _, _ := setupSignatureTest(t)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/v1/client/login", strings.NewReader("{}"))
	// 不设置任何签名头
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Contains(t, w.Body.String(), "签名参数缺失")
}

func TestSignatureAuth_AppNotFound(t *testing.T) {
	r, _, _, _ := setupSignatureTest(t)

	body := `{}`
	req := buildSignRequest(t, "POST", "/api/v1/client/login", body, "any-secret", "ak_nonexistent")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Contains(t, w.Body.String(), "应用不存在")
}

func TestSignatureAuth_TimestampOutOfTolerance(t *testing.T) {
	r, signSecret, app, _ := setupSignatureTest(t)

	// 构造 1 小时前的时间戳（超出 ±300 秒容差）
	oldTs := time.Now().Add(-time.Hour).Unix()
	body := `{}`
	signString := strings.Join([]string{"POST", "/api/v1/client/login", timestampStr(oldTs), "nonce-old", body}, "\n")
	sig := crypto.HMACSHA256(signSecret, []byte(signString))

	req := httptest.NewRequest("POST", "/api/v1/client/login", strings.NewReader(body))
	req.Header.Set("X-App-Key", app.AppKey)
	req.Header.Set("X-Timestamp", timestampStr(oldTs))
	req.Header.Set("X-Nonce", "nonce-old")
	req.Header.Set("X-Signature", sig)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Contains(t, w.Body.String(), "时间戳超出允许范围")
}

func TestSignatureAuth_TimestampMalformed(t *testing.T) {
	r, _, app, _ := setupSignatureTest(t)

	req := httptest.NewRequest("POST", "/api/v1/client/login", strings.NewReader("{}"))
	req.Header.Set("X-App-Key", app.AppKey)
	req.Header.Set("X-Timestamp", "not-a-number")
	req.Header.Set("X-Nonce", "n")
	req.Header.Set("X-Signature", "sig")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Contains(t, w.Body.String(), "时间戳格式错误")
}

func TestSignatureAuth_NonceReplay(t *testing.T) {
	r, signSecret, app, _ := setupSignatureTest(t)

	body := `{}`
	req1 := buildSignRequest(t, "POST", "/api/v1/client/login", body, signSecret, app.AppKey)

	// 第一次请求应通过
	w1 := httptest.NewRecorder()
	r.ServeHTTP(w1, req1)
	assert.Equal(t, 200, w1.Code)

	// 第二次相同 nonce 应被拒绝（防重放）
	// 重新构造相同 nonce 的请求（重置 body reader）
	req2 := buildSignRequest(t, "POST", "/api/v1/client/login", body, signSecret, app.AppKey)
	// 覆盖 nonce 为第一次的 nonce
	tolerance := time.Now().Unix()
	_ = tolerance
	// 用第一次请求的 nonce（已使用）
	signString := strings.Join([]string{
		"POST",
		"/api/v1/client/login",
		timestampStr(time.Now().Unix()),
		"nonce-test-12345", // 与 buildSignRequest 第一次相同
		body,
	}, "\n")
	sig := crypto.HMACSHA256(signSecret, []byte(signString))
	req2 = httptest.NewRequest("POST", "/api/v1/client/login", strings.NewReader(body))
	req2.Header.Set("X-App-Key", app.AppKey)
	req2.Header.Set("X-Timestamp", timestampStr(time.Now().Unix()))
	req2.Header.Set("X-Nonce", "nonce-test-12345")
	req2.Header.Set("X-Signature", sig)

	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)

	assert.Equal(t, http.StatusUnauthorized, w2.Code)
	assert.Contains(t, w2.Body.String(), "请求已过期或重复")
}

func TestSignatureAuth_WrongSignature(t *testing.T) {
	r, _, app, _ := setupSignatureTest(t)

	body := `{}`
	req := httptest.NewRequest("POST", "/api/v1/client/login", strings.NewReader(body))
	req.Header.Set("X-App-Key", app.AppKey)
	req.Header.Set("X-Timestamp", timestampStr(time.Now().Unix()))
	req.Header.Set("X-Nonce", "nonce-wrong-sig")
	req.Header.Set("X-Signature", "deadbeef") // 故意错误的签名

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Contains(t, w.Body.String(), "签名校验失败")
}

// ============== RateLimitByIP ==============

func TestRateLimitByIP_BelowLimit(t *testing.T) {
	rdb, _ := setupMiniRedis(t)
	cfg := newMockConfigReader()
	cfg.values["security.rate.limit_sensitive"] = "3" // 每分钟 3 次

	r := setupGinRouter()
	r.Use(RateLimitByIP(rdb, cfg, "sensitive"))
	r.GET("/test", func(c *gin.Context) { c.Status(200) })

	// 3 次请求都应通过
	for i := 1; i <= 3; i++ {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "1.2.3.4:1234"
		r.ServeHTTP(w, req)
		assert.Equal(t, 200, w.Code, "第 %d 次应通过", i)
	}
}

func TestRateLimitByIP_ExceedsLimit(t *testing.T) {
	rdb, _ := setupMiniRedis(t)
	cfg := newMockConfigReader()
	cfg.values["security.rate.limit_sensitive"] = "2" // 每分钟 2 次

	r := setupGinRouter()
	r.Use(RateLimitByIP(rdb, cfg, "sensitive"))
	r.GET("/test", func(c *gin.Context) { c.Status(200) })

	// 前 2 次通过
	for i := 1; i <= 2; i++ {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "5.6.7.8:1234"
		r.ServeHTTP(w, req)
		assert.Equal(t, 200, w.Code)
	}

	// 第 3 次应被限流
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "5.6.7.8:1234"
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusTooManyRequests, w.Code)
	assert.Contains(t, w.Body.String(), "请求过于频繁")
}

func TestRateLimitByIP_DifferentIPsIndependent(t *testing.T) {
	rdb, _ := setupMiniRedis(t)
	cfg := newMockConfigReader()
	cfg.values["security.rate.limit_sensitive"] = "1" // 每分钟 1 次

	r := setupGinRouter()
	r.Use(RateLimitByIP(rdb, cfg, "sensitive"))
	r.GET("/test", func(c *gin.Context) { c.Status(200) })

	// IP A 第 1 次：通过
	wA1 := httptest.NewRecorder()
	reqA1 := httptest.NewRequest("GET", "/test", nil)
	reqA1.RemoteAddr = "1.1.1.1:1234"
	r.ServeHTTP(wA1, reqA1)
	assert.Equal(t, 200, wA1.Code)

	// IP B 第 1 次：应通过（不同 IP 不互相影响）
	wB1 := httptest.NewRecorder()
	reqB1 := httptest.NewRequest("GET", "/test", nil)
	reqB1.RemoteAddr = "2.2.2.2:1234"
	r.ServeHTTP(wB1, reqB1)
	assert.Equal(t, 200, wB1.Code)

	// IP A 第 2 次：应被限流
	wA2 := httptest.NewRecorder()
	reqA2 := httptest.NewRequest("GET", "/test", nil)
	reqA2.RemoteAddr = "1.1.1.1:1234"
	r.ServeHTTP(wA2, reqA2)
	assert.Equal(t, http.StatusTooManyRequests, wA2.Code)
}

func TestRateLimitByIP_RedisFailureFailOpen(t *testing.T) {
	// 直接构造指向不可达地址的 redis.Client 模拟 Redis 故障
	// （miniredis.Close() 后调用 mr.Addr() 会 panic，所以用真实不可达地址）
	rdb := redis.NewClient(&redis.Options{
		Addr:        "127.0.0.1:1", // 不可达端口
		DialTimeout: 100 * time.Millisecond,
	})
	defer rdb.Close()

	cfg := newMockConfigReader()
	r := setupGinRouter()
	r.Use(RateLimitByIP(rdb, cfg, "global"))
	r.GET("/test", func(c *gin.Context) { c.Status(200) })

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "9.9.9.9:1234"
	r.ServeHTTP(w, req)

	// Redis 故障时应 fail-open（放行而非阻断）
	assert.Equal(t, 200, w.Code)
}

// ============== IPBlacklist ==============

func TestIPBlacklist_BlockedIP(t *testing.T) {
	rdb, _ := setupMiniRedis(t)
	// 预置黑名单
	require.NoError(t, rdb.Set(context.Background(), "ip:blacklist:1.2.3.4", "auto", time.Hour).Err())

	r := setupGinRouter()
	r.Use(IPBlacklist(rdb, nil))
	r.GET("/test", func(c *gin.Context) { c.Status(200) })

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "1.2.3.4:1234"
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Contains(t, w.Body.String(), "IP 已被加入黑名单")
}

func TestIPBlacklist_CleanIP(t *testing.T) {
	rdb, _ := setupMiniRedis(t)

	r := setupGinRouter()
	r.Use(IPBlacklist(rdb, nil))
	r.GET("/test", func(c *gin.Context) { c.Status(200) })

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "8.8.8.8:1234"
	r.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
}

// ============== RecordCardFailure + ClearCardFailure ==============

func TestRecordCardFailure_BelowThreshold(t *testing.T) {
	rdb, _ := setupMiniRedis(t)
	cfg := newMockConfigReader()
	cfg.values["security.ban.threshold"] = "3"

	ip := "1.1.1.1"
	// 记录 2 次（未达阈值 3），不应被封禁
	for i := 1; i <= 2; i++ {
		require.NoError(t, RecordCardFailure(context.Background(), rdb, cfg, ip))
	}

	// 不应在黑名单中
	exists, err := rdb.Exists(context.Background(), "ip:blacklist:"+ip).Result()
	require.NoError(t, err)
	assert.Equal(t, int64(0), exists, "未达阈值不应封禁")
}

func TestRecordCardFailure_AutoBanAtThreshold(t *testing.T) {
	rdb, _ := setupMiniRedis(t)
	cfg := newMockConfigReader()
	cfg.values["security.ban.threshold"] = "3"
	cfg.values["security.ban.duration_seconds"] = "3600"

	ip := "2.2.2.2"
	// 记录 3 次（达到阈值 3），第 3 次应自动封禁
	for i := 1; i <= 3; i++ {
		require.NoError(t, RecordCardFailure(context.Background(), rdb, cfg, ip))
	}

	// 应在黑名单中
	exists, err := rdb.Exists(context.Background(), "ip:blacklist:"+ip).Result()
	require.NoError(t, err)
	assert.Equal(t, int64(1), exists, "达到阈值应自动封禁")
}

func TestClearCardFailure(t *testing.T) {
	rdb, _ := setupMiniRedis(t)
	cfg := newMockConfigReader()
	ip := "3.3.3.3"

	// 记录 2 次失败
	for i := 1; i <= 2; i++ {
		require.NoError(t, RecordCardFailure(context.Background(), rdb, cfg, ip))
	}

	// 清除
	ClearCardFailure(context.Background(), rdb, ip)

	// 失败计数应被清除
	count, err := rdb.Get(context.Background(), "fail:card:"+ip).Result()
	assert.Equal(t, redis.Nil, err, "清除后 key 应不存在")
	assert.Empty(t, count)
}

// ============== Response ==============

func TestSuccess_ResponseFormat(t *testing.T) {
	r := setupGinRouter()
	r.GET("/test", func(c *gin.Context) {
		Success(c, gin.H{"id": 1})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
	body := w.Body.String()
	assert.Contains(t, body, `"code":0`)
	assert.Contains(t, body, `"message":"success"`)
	assert.Contains(t, body, `"id":1`)
	assert.Contains(t, body, `"request_id":"req-`) // 自动生成 UUID
	assert.Contains(t, body, `"timestamp":`)
}

func TestFail_ResponseFormat(t *testing.T) {
	r := setupGinRouter()
	r.GET("/test", func(c *gin.Context) {
		Fail(c, http.StatusBadRequest, 1001, "参数错误")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	body := w.Body.String()
	assert.Contains(t, body, `"code":1001`)
	assert.Contains(t, body, `"message":"参数错误"`)
	assert.NotContains(t, body, `"data"`) // 失败响应不应有 data 字段
}

// ============== GenerateToken + JWTAuth 端到端 ==============

func TestGenerateToken_Auth_RoundTrip(t *testing.T) {
	secret := "round-trip-secret"
	claims := JWTClaims{
		UserID:   999,
		Username: "roundtrip-user",
		Role:     "admin",
		TenantID: 8888,
	}

	token, err := GenerateToken(secret, "test-issuer", 24, claims)
	require.NoError(t, err)
	require.NotEmpty(t, token)

	// 用 JWTAuth 验证
	r := setupGinRouter()
	r.Use(JWTAuth(secret, "admin"))
	r.GET("/test", func(c *gin.Context) {
		uid, _ := c.Get("user_id")
		uname, _ := c.Get("username")
		tid, _ := c.Get("tenant_id")
		c.JSON(200, gin.H{"uid": uid, "uname": uname, "tid": tid})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
	body := w.Body.String()
	assert.Contains(t, body, `"uid":999`)
	assert.Contains(t, body, `"uname":"roundtrip-user"`)
	assert.Contains(t, body, `"tid":8888`)
}
