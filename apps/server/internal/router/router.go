// Package router 路由注册
package router

import (
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/your-org/keyauth-saas/apps/server/internal/config"
	"github.com/your-org/keyauth-saas/apps/server/internal/handler"
	"github.com/your-org/keyauth-saas/apps/server/internal/middleware"
)

// Register 注册所有路由
func Register(container *config.Container) *gin.Engine {
	cfg := container.Config
	if cfg.App.Mode == "release" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()

	// 全局中间件
	r.Use(gin.Recovery())
	r.Use(gin.Logger())
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"}, // 生产环境应限制为已知域名
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization", "X-App-Key", "X-Timestamp", "X-Nonce", "X-Signature"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
	}))
	r.Use(middleware.IPBlacklist(container.Redis, container.DB))

	// 注入全局加密管理器
	middleware.SetCryptoManager(container.Crypto)

	// 健康检查
	r.GET("/health", func(c *gin.Context) {
		middleware.Success(c, gin.H{"status": "ok"})
	})

	// ============== API v1 ==============
	v1 := r.Group("/api/v1")

	// 初始化依赖注入（简化版直接构造，正式版应用 wire/fx）
	deps := &handler.Deps{
		DB:        container.DB,
		Redis:     container.Redis,
		Crypto:    container.Crypto,
		Config:    container.Config,
		CfgCache:  container.ConfigCache(),
	}

	// ----- 客户端验证 API（HMAC 签名鉴权） -----
	clientGroup := v1.Group("/client")
	clientGroup.Use(middleware.SignatureAuth(container.DB, container.Redis, container.ConfigCache()))
	clientGroup.Use(middleware.RateLimitByIP(container.Redis, container.ConfigCache(), "verify"))
	{
		clientGroup.POST("/login", handler.ClientLogin(deps))
		clientGroup.POST("/verify", handler.ClientVerify(deps))
		clientGroup.POST("/heartbeat", handler.ClientHeartbeat(deps))
		clientGroup.POST("/bind", handler.ClientBind(deps))
		clientGroup.POST("/unbind", handler.ClientUnbind(deps))
		clientGroup.POST("/get_var", handler.ClientGetVar(deps))
		clientGroup.POST("/notice", handler.ClientNotice(deps))
		clientGroup.POST("/version", handler.ClientVersion(deps))
		clientGroup.POST("/logout", handler.ClientLogout(deps))
	}

	// ----- 平台超管 API（JWT 鉴权） -----
	adminAuth := v1.Group("/admin")
	adminAuth.Use(middleware.JWTAuth(cfg.JWT.Secret, "admin"))
	{
		adminAuth.GET("/dashboard", handler.AdminDashboard(deps))
		adminAuth.GET("/tenants", handler.AdminListTenants(deps))
		adminAuth.POST("/tenants", handler.AdminCreateTenant(deps))
		adminAuth.PUT("/tenants/:id", handler.AdminUpdateTenant(deps))
		adminAuth.GET("/packages", handler.AdminListPackages(deps))
		adminAuth.POST("/packages", handler.AdminCreatePackage(deps))
		adminAuth.GET("/config", handler.AdminListConfig(deps))
		adminAuth.PUT("/config/:key", handler.AdminUpdateConfig(deps))
		adminAuth.GET("/notices", handler.AdminListNotices(deps))
		adminAuth.POST("/notices", handler.AdminCreateNotice(deps))
	}

	// ----- 开发者控制台 API（JWT 鉴权 + 多租户隔离） -----
	tenantAuth := v1.Group("/tenant")
	tenantAuth.Use(middleware.JWTAuth(cfg.JWT.Secret, "tenant"))
	tenantAuth.Use(middleware.TenantScope(container.DB))
	{
		tenantAuth.GET("/dashboard", handler.TenantDashboard(deps))
		tenantAuth.GET("/apps", handler.TenantListApps(deps))
		tenantAuth.POST("/apps", handler.TenantCreateApp(deps))
		tenantAuth.PUT("/apps/:id", handler.TenantUpdateApp(deps))
		tenantAuth.GET("/cards", handler.TenantListCards(deps))
		tenantAuth.POST("/cards/generate", handler.TenantGenerateCards(deps))
		tenantAuth.GET("/agents", handler.TenantListAgents(deps))
		tenantAuth.POST("/agents/invite_codes", handler.TenantGenInviteCode(deps))
	}

	// ----- 代理商控制台 API（JWT 鉴权 + 多租户隔离） -----
	agentAuth := v1.Group("/agent")
	agentAuth.Use(middleware.JWTAuth(cfg.JWT.Secret, "agent"))
	agentAuth.Use(middleware.TenantScope(container.DB))
	{
		agentAuth.GET("/dashboard", handler.AgentDashboard(deps))
		agentAuth.GET("/cards", handler.AgentListCards(deps))
		agentAuth.POST("/cards/generate", handler.AgentGenerateCards(deps))
		agentAuth.GET("/commission", handler.AgentListCommission(deps))
		agentAuth.POST("/withdraw", handler.AgentWithdraw(deps))
	}

	// ----- 公共 API（无需鉴权） -----
	publicGroup := v1.Group("/public")
	{
		publicGroup.POST("/auth/admin/login", handler.AdminLogin(deps))
		publicGroup.POST("/auth/tenant/register", handler.TenantRegister(deps))
		publicGroup.POST("/auth/tenant/login", handler.TenantLogin(deps))
		publicGroup.POST("/auth/agent/login", handler.AgentLogin(deps))
		publicGroup.POST("/auth/agent/register", handler.AgentRegister(deps))
		publicGroup.GET("/notices/platform", handler.PublicPlatformNotices(deps))
	}

	// ----- 支付回调（无鉴权，靠签名校验） -----
	payGroup := v1.Group("/pay")
	{
		payGroup.POST("/notify/epay", handler.EpayNotify(deps))
		payGroup.GET("/return/epay", handler.EpayReturn(deps))
		payGroup.POST("/notify/tenant/:tenant_id", handler.EpayTenantNotify(deps))
	}

	return r
}
