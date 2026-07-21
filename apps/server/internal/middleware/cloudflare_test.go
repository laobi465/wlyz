// Package middleware v0.4.0 Cloudflare 中间件单元测试
// 严格遵循铁律 06：所有断言基于已知固定输入，无随机/不确定性
package middleware

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
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
)

func setupCFTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:cf_test_%d?mode=memory&cache=shared", time.Now().UnixNano())), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.SysConfig{}))
	db.Exec("DELETE FROM sys_config")
	return db
}

func setupCFCfgCache(t *testing.T, db *gorm.DB, overrides map[string]string) *config.ConfigCache {
	t.Helper()
	mr, err := miniredis.Run()
	require.NoError(t, err)
	t.Cleanup(mr.Close)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	defaults := map[string]string{
		"cloudflare.enabled":          "0",
		"cloudflare.real_ip_header":   "CF-Connecting-IP",
		"cloudflare.ip_country_header": "CF-IPCountry",
		"cloudflare.trusted_cidrs":    "",
	}
	if overrides != nil {
		for k, v := range overrides {
			defaults[k] = v
		}
	}
	for k, v := range defaults {
		require.NoError(t, db.Create(&model.SysConfig{
			ConfigKey: k, ConfigValue: v, ConfigType: "string", ConfigGroup: "security",
		}).Error)
	}
	cfg := config.NewConfigCache(db, rdb)
	require.NoError(t, cfg.Preload(context.Background()))
	return cfg
}

func setupCFEngine() *gin.Engine {
	gin.SetMode(gin.TestMode)
	return gin.New()
}

// ============== 测试 1：未启用 CF 时回退到 c.ClientIP() ==============

func TestCloudflareRealIP_DisabledFallback(t *testing.T) {
	db := setupCFTestDB(t)
	cfg := setupCFCfgCache(t, db, nil)

	r := setupCFEngine()
	r.Use(CloudflareRealIP(cfg))
	r.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ip": RealIP(c)})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "1.2.3.4:5678"
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	// 默认 c.ClientIP() 在测试环境返回 RemoteAddr 的 host
	assert.Contains(t, w.Body.String(), "1.2.3.4")
}

// ============== 测试 2：启用 CF，从 CF-Connecting-IP 取真实 IP ==============

func TestCloudflareRealIP_EnabledWithHeader(t *testing.T) {
	db := setupCFTestDB(t)
	// P1-04 修复后：必须配置 trusted_cidrs 且 RemoteAddr 命中受信 CIDR 才会信任 CF 头
	// 此处使用 Cloudflare 官方 CIDR（173.245.48.0/20），并将 RemoteAddr 设为该网段内的地址
	cfg := setupCFCfgCache(t, db, map[string]string{
		"cloudflare.enabled":       "1",
		"cloudflare.trusted_cidrs": "173.245.48.0/20,103.21.244.0/22,103.22.200.0/22",
	})

	r := setupCFEngine()
	r.Use(CloudflareRealIP(cfg))
	r.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ip": RealIP(c), "country": IPCountry(c)})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "173.245.48.10:5678" // 命中 173.245.48.0/20 受信 CIDR
	req.Header.Set("CF-Connecting-IP", "203.0.113.50")
	req.Header.Set("CF-IPCountry", "CN")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "203.0.113.50")
	assert.Contains(t, w.Body.String(), "CN")
}

// ============== 测试 3：受信 CIDR 校验通过 ==============

func TestCloudflareRealIP_TrustedCIDRMatch(t *testing.T) {
	db := setupCFTestDB(t)
	cfg := setupCFCfgCache(t, db, map[string]string{
		"cloudflare.enabled":       "1",
		"cloudflare.trusted_cidrs": "10.0.0.0/8",
	})

	r := setupCFEngine()
	r.Use(CloudflareRealIP(cfg))
	r.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ip": RealIP(c)})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "10.0.0.1:5678" // 在 10.0.0.0/8 内
	req.Header.Set("CF-Connecting-IP", "203.0.113.50")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "203.0.113.50")
}

// ============== 测试 4：受信 CIDR 校验失败，回退 c.ClientIP() ==============

func TestCloudflareRealIP_UntrustedCIDR(t *testing.T) {
	db := setupCFTestDB(t)
	cfg := setupCFCfgCache(t, db, map[string]string{
		"cloudflare.enabled":       "1",
		"cloudflare.trusted_cidrs": "10.0.0.0/8",
	})

	r := setupCFEngine()
	r.Use(CloudflareRealIP(cfg))
	r.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ip": RealIP(c)})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "203.0.113.99:5678" // 不在 10.0.0.0/8 内
	req.Header.Set("CF-Connecting-IP", "203.0.113.50")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	// 应回退到 RemoteAddr 的 host，而非信任 CF 头
	assert.Contains(t, w.Body.String(), "203.0.113.99")
	assert.NotContains(t, w.Body.String(), "203.0.113.50")
}

// ============== 测试 5：自定义头名 ==============

func TestCloudflareRealIP_CustomHeaderName(t *testing.T) {
	db := setupCFTestDB(t)
	cfg := setupCFCfgCache(t, db, map[string]string{
		"cloudflare.enabled":       "1",
		"cloudflare.real_ip_header": "X-Real-IP",
	})

	r := setupCFEngine()
	r.Use(CloudflareRealIP(cfg))
	r.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ip": RealIP(c)})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "10.0.0.1:5678"
	req.Header.Set("X-Real-IP", "198.51.100.10")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Contains(t, w.Body.String(), "198.51.100.10")
}

// ============== 测试 6：CIDR 列表解析（单 IP 无前缀） ==============

func TestIPInCIDRList_SingleIP(t *testing.T) {
	assert.True(t, ipInCIDRList("1.2.3.4", "1.2.3.4"))
	assert.False(t, ipInCIDRList("1.2.3.5", "1.2.3.4"))
}

func TestIPInCIDRList_CIDR(t *testing.T) {
	assert.True(t, ipInCIDRList("10.0.0.5", "10.0.0.0/8"))
	assert.False(t, ipInCIDRList("192.168.0.1", "10.0.0.0/8"))
}

func TestIPInCIDRList_MultipleEntries(t *testing.T) {
	assert.True(t, ipInCIDRList("10.0.0.5", "10.0.0.0/8,192.168.0.0/16"))
	assert.True(t, ipInCIDRList("192.168.1.1", "10.0.0.0/8,192.168.0.0/16"))
	assert.False(t, ipInCIDRList("172.16.0.1", "10.0.0.0/8,192.168.0.0/16"))
}

func TestIPInCIDRList_InvalidIP(t *testing.T) {
	assert.False(t, ipInCIDRList("invalid", "10.0.0.0/8"))
}

// ============== 测试 7：hostFromAddr ==============

func TestHostFromAddr(t *testing.T) {
	assert.Equal(t, "1.2.3.4", hostFromAddr("1.2.3.4:5678"))
	assert.Equal(t, "1.2.3.4", hostFromAddr("1.2.3.4")) // 无端口
	assert.Equal(t, "", hostFromAddr(""))
}

// ============== 测试 8：IPCountry 未注入时返回空串 ==============

func TestIPCountry_EmptyWhenNotSet(t *testing.T) {
	db := setupCFTestDB(t)
	cfg := setupCFCfgCache(t, db, map[string]string{"cloudflare.enabled": "0"})

	r := setupCFEngine()
	r.Use(CloudflareRealIP(cfg))
	r.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"country": IPCountry(c)})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "1.2.3.4:5678"
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Contains(t, w.Body.String(), `"country":""`)
}
