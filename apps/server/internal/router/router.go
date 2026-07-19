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
		DB:       container.DB,
		Redis:    container.Redis,
		Crypto:   container.Crypto,
		Config:   container.Config,
		CfgCache: container.ConfigCache(),
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
		// 工作台
		adminAuth.GET("/dashboard", handler.AdminDashboard(deps))

		// 租户管理
		adminAuth.GET("/tenants", handler.AdminListTenants(deps))
		adminAuth.POST("/tenants", handler.AdminCreateTenant(deps))
		adminAuth.PUT("/tenants/:id", handler.AdminUpdateTenant(deps))

		// 套餐管理
		adminAuth.GET("/packages", handler.AdminListPackages(deps))
		adminAuth.POST("/packages", handler.AdminCreatePackage(deps))
		adminAuth.PUT("/packages/:id", handler.AdminUpdatePackage(deps))

		// 系统配置
		adminAuth.GET("/config", handler.AdminListConfig(deps))
		adminAuth.PUT("/config/:key", handler.AdminUpdateConfig(deps))

		// 平台代理管理
		adminAuth.GET("/agents", handler.AdminListAgents(deps))
		adminAuth.PUT("/agents/:id", handler.AdminUpdateAgent(deps))

		// 公告管理
		adminAuth.GET("/notices", handler.AdminListNotices(deps))
		adminAuth.POST("/notices", handler.AdminCreateNotice(deps))
		adminAuth.PUT("/notices/:id", handler.AdminUpdateNotice(deps))
		adminAuth.DELETE("/notices/:id", handler.AdminDeleteNotice(deps))

		// 日志审计（v0.3.3 升级：3 表独立查询 + CSV 导出）
		adminAuth.GET("/logs", handler.AdminListLogs(deps))                           // 兼容旧接口（仅 operation）
		adminAuth.GET("/logs/operations", handler.AdminListOperationLogs(deps))       // 操作日志
		adminAuth.GET("/logs/verify", handler.AdminListVerifyLogs(deps))              // 验证日志
		adminAuth.GET("/logs/login_failed", handler.AdminListLoginFailedLogs(deps))   // 登录失败日志
		adminAuth.GET("/logs/export", handler.AdminExportLogs(deps))                  // CSV 导出

		// 安全中心
		adminAuth.GET("/security/stats", handler.AdminSecurityStats(deps))
		adminAuth.GET("/security/ip_blacklist", handler.AdminListIPBlacklist(deps))
		adminAuth.POST("/security/ip_blacklist", handler.AdminAddIPBlacklist(deps))
		adminAuth.DELETE("/security/ip_blacklist/:id", handler.AdminRemoveIPBlacklist(deps))

		// 支付结算管理（v0.2.3 + v0.3.4 升级）
		adminAuth.GET("/settlements", handler.AdminListSettlements(deps))
		adminAuth.POST("/settlements/:id/settle", handler.AdminSettleOrder(deps))
		adminAuth.POST("/settlements/batch_settle", handler.AdminBatchSettle(deps)) // v0.3.4 批量结算
		adminAuth.POST("/pay/test", handler.AdminTestPayConfig(deps))

		// 开发者提现审核（v0.3.4 新增）
		adminAuth.GET("/tenant_withdrawals", handler.AdminListTenantWithdrawals(deps))
		adminAuth.POST("/tenant_withdrawals/:id/pay", handler.AdminPayTenantWithdraw(deps))
		adminAuth.POST("/tenant_withdrawals/:id/reject", handler.AdminRejectTenantWithdraw(deps))

		// 对账报表（v0.3.4 新增）
		adminAuth.GET("/reconciliation", handler.AdminReconciliation(deps))

		// 账号设置（v0.3.0 三角色统一）
		adminAuth.GET("/auth/me", handler.ProfileMe(deps))
		adminAuth.PUT("/auth/profile", handler.UpdateProfile(deps))
		adminAuth.POST("/auth/change_password", handler.ChangePassword(deps))
		adminAuth.POST("/auth/2fa/setup", handler.Setup2FA(deps))
		adminAuth.POST("/auth/2fa/verify", handler.Verify2FA(deps))
		adminAuth.POST("/auth/2fa/disable", handler.Disable2FA(deps))
		adminAuth.GET("/auth/devices", handler.ListLoginDevices(deps))
		adminAuth.POST("/auth/devices/:id/kick", handler.KickDevice(deps))
		adminAuth.POST("/auth/logout", handler.Logout(deps))
	}

	// ----- 开发者控制台 API（JWT 鉴权 + 多租户隔离） -----
	tenantAuth := v1.Group("/tenant")
	tenantAuth.Use(middleware.JWTAuth(cfg.JWT.Secret, "tenant"))
	tenantAuth.Use(middleware.TenantScope(container.DB))
	{
		// 工作台
		tenantAuth.GET("/dashboard", handler.TenantDashboard(deps))

		// 应用管理
		tenantAuth.GET("/apps", handler.TenantListApps(deps))
		tenantAuth.POST("/apps", handler.TenantCreateApp(deps))
		tenantAuth.GET("/apps/:id", handler.TenantGetApp(deps))
		tenantAuth.PUT("/apps/:id", handler.TenantUpdateApp(deps))
		tenantAuth.DELETE("/apps/:id", handler.TenantDeleteApp(deps))
		tenantAuth.POST("/apps/:id/reset_key", handler.TenantResetAppKey(deps))

		// 卡类管理
		tenantAuth.GET("/card_types", handler.TenantListCardTypes(deps))
		tenantAuth.POST("/card_types", handler.TenantCreateCardType(deps))
		tenantAuth.PUT("/card_types/:id", handler.TenantUpdateCardType(deps))

		// 卡密管理
		tenantAuth.GET("/cards", handler.TenantListCards(deps))
		tenantAuth.GET("/cards/:id", handler.TenantGetCard(deps))
		tenantAuth.POST("/cards/generate", handler.TenantGenerateCards(deps))
		tenantAuth.POST("/cards/:id/ban", handler.TenantBanCard(deps))
		tenantAuth.POST("/cards/:id/unban", handler.TenantUnbanCard(deps))
		tenantAuth.DELETE("/cards/:id", handler.TenantDeleteCard(deps))

		// 设备管理（v0.3.0）
		tenantAuth.GET("/devices", handler.TenantListDevices(deps))
		tenantAuth.POST("/devices/:id/kick", handler.TenantKickDevice(deps))

		// 订单管理（v0.3.0）
		tenantAuth.GET("/orders", handler.TenantListOrders(deps))

		// 云变量（v0.3.0）
		tenantAuth.GET("/cloud_vars", handler.TenantListCloudVars(deps))
		tenantAuth.POST("/cloud_vars", handler.TenantUpsertCloudVar(deps))
		tenantAuth.PUT("/cloud_vars/:id", handler.TenantUpsertCloudVar(deps))
		tenantAuth.DELETE("/cloud_vars/:id", handler.TenantDeleteCloudVar(deps))

		// 版本管理（v0.3.0）
		tenantAuth.GET("/versions", handler.TenantListVersions(deps))
		tenantAuth.POST("/versions", handler.TenantCreateVersion(deps))
		tenantAuth.DELETE("/versions/:id", handler.TenantDeleteVersion(deps))

		// 代理管理（v0.3.0）
		tenantAuth.GET("/agents", handler.TenantListAgents(deps))
		tenantAuth.PUT("/agents/:id", handler.TenantUpdateAgent(deps))

		// 邀请码（v0.3.0）
		tenantAuth.GET("/agents/invite_codes", handler.TenantListInviteCodes(deps))
		tenantAuth.POST("/agents/invite_codes", handler.TenantGenInviteCode(deps))
		tenantAuth.POST("/agents/invite_codes/:id/disable", handler.TenantDisableInviteCode(deps))

		// 开发者支付配置（v0.3.0）
		tenantAuth.GET("/pay_config", handler.TenantListPayConfig(deps))
		tenantAuth.POST("/pay_config", handler.TenantSavePayConfig(deps))
		tenantAuth.PUT("/pay_config/:channel", handler.TenantSavePayConfig(deps))
		tenantAuth.POST("/pay_config/test", handler.TenantTestPayConfig(deps))

		// 开发者公告（v0.3.0）
		tenantAuth.GET("/notices", handler.TenantListNotices(deps))
		tenantAuth.POST("/notices", handler.TenantCreateNotice(deps))
		tenantAuth.PUT("/notices/:id", handler.TenantUpdateNotice(deps))
		tenantAuth.DELETE("/notices/:id", handler.TenantDeleteNotice(deps))

		// 财务审核（v0.3.2 代理充值/提现审核闭环）
		tenantAuth.GET("/recharge_requests", handler.TenantListRechargeRequests(deps))
		tenantAuth.POST("/recharge_requests/:id/approve", handler.TenantApproveRecharge(deps))
		tenantAuth.POST("/recharge_requests/:id/reject", handler.TenantRejectRecharge(deps))
		tenantAuth.GET("/withdrawals", handler.TenantListWithdrawals(deps))
		tenantAuth.POST("/withdrawals/:id/pay", handler.TenantPayWithdraw(deps))
		tenantAuth.POST("/withdrawals/:id/reject", handler.TenantRejectWithdraw(deps))

		// 开发者结算与提现（v0.3.4 新增）
		tenantAuth.GET("/settlements", handler.TenantListSettlements(deps))           // 自己的结算记录
		tenantAuth.GET("/balance_overview", handler.TenantBalanceOverview(deps))      // 余额概览
		tenantAuth.GET("/balance_logs", handler.TenantListBalanceLogs(deps))          // 余额流水
		tenantAuth.GET("/withdrawals/mine", handler.TenantListOwnWithdrawals(deps))   // 自己的提现申请
		tenantAuth.POST("/withdraw", handler.TenantWithdraw(deps))                    // 发起提现申请

		// 账号设置（v0.3.0 三角色统一）
		tenantAuth.GET("/auth/me", handler.ProfileMe(deps))
		tenantAuth.PUT("/auth/profile", handler.UpdateProfile(deps))
		tenantAuth.POST("/auth/change_password", handler.ChangePassword(deps))
		tenantAuth.POST("/auth/2fa/setup", handler.Setup2FA(deps))
		tenantAuth.POST("/auth/2fa/verify", handler.Verify2FA(deps))
		tenantAuth.POST("/auth/2fa/disable", handler.Disable2FA(deps))
		tenantAuth.GET("/auth/devices", handler.ListLoginDevices(deps))
		tenantAuth.POST("/auth/devices/:id/kick", handler.KickDevice(deps))
		tenantAuth.POST("/auth/logout", handler.Logout(deps))
	}

	// ----- 代理商控制台 API（JWT 鉴权 + 多租户隔离） -----
	agentAuth := v1.Group("/agent")
	agentAuth.Use(middleware.JWTAuth(cfg.JWT.Secret, "agent"))
	agentAuth.Use(middleware.TenantScope(container.DB))
	{
		// 工作台
		agentAuth.GET("/dashboard", handler.AgentDashboard(deps))

		// 账号信息（v0.3.0 扩展，覆盖原 CurrentUser）
		agentAuth.GET("/auth/me", handler.AgentMe(deps))
		agentAuth.PUT("/auth/profile", handler.UpdateProfile(deps))
		agentAuth.POST("/auth/change_password", handler.ChangePassword(deps))
		agentAuth.POST("/auth/2fa/setup", handler.Setup2FA(deps))
		agentAuth.POST("/auth/2fa/verify", handler.Verify2FA(deps))
		agentAuth.POST("/auth/2fa/disable", handler.Disable2FA(deps))
		agentAuth.GET("/auth/devices", handler.ListLoginDevices(deps))
		agentAuth.POST("/auth/devices/:id/kick", handler.KickDevice(deps))
		agentAuth.POST("/auth/logout", handler.Logout(deps))

		// 卡类与卡密
		agentAuth.GET("/card_types", handler.AgentListCardTypes(deps))
		agentAuth.GET("/cards", handler.AgentListCards(deps))
		agentAuth.POST("/cards/generate", handler.AgentGenerateCards(deps))

		// 订单
		agentAuth.GET("/orders", handler.AgentListOrders(deps))

		// 佣金与提现
		agentAuth.GET("/commission", handler.AgentListCommission(deps))
		agentAuth.POST("/withdraw", handler.AgentWithdraw(deps))
		agentAuth.POST("/recharge", handler.AgentRecharge(deps))

		// 消息通知
		agentAuth.GET("/notices", handler.AgentListNotices(deps))
		agentAuth.POST("/notices/:id/read", handler.AgentReadNotice(deps))
	}

	// ----- 公共 API（无需鉴权） -----
	publicGroup := v1.Group("/public")
	{
		publicGroup.POST("/auth/admin/login", handler.AdminLogin(deps))
		publicGroup.POST("/auth/tenant/register", handler.TenantRegister(deps))
		publicGroup.POST("/auth/tenant/login", handler.TenantLogin(deps))
		publicGroup.POST("/auth/agent/login", handler.AgentLogin(deps))
		publicGroup.POST("/auth/agent/register", handler.AgentRegister(deps))
		publicGroup.POST("/auth/refresh", handler.RefreshToken(deps)) // 三角色共用
		publicGroup.GET("/notices/platform", handler.PublicPlatformNotices(deps))
	}

	// ----- 支付回调（无鉴权，靠签名校验） -----
	payGroup := v1.Group("/pay")
	{
		// 终端用户下单（无鉴权，任何人可下单）
		payGroup.POST("/order", handler.CreatePayOrder(deps))
		// 终端用户查询订单
		payGroup.GET("/order/:order_no", handler.GetPayOrder(deps))
		// 易支付异步回调
		payGroup.POST("/notify/epay", handler.EpayNotify(deps))
		// 易支付同步跳转（用户浏览器 302）
		payGroup.GET("/return/epay", handler.EpayReturn(deps))
		// 开发者自有易支付回调（v0.3.0）
		payGroup.POST("/notify/tenant/:tenant_id", handler.EpayTenantNotify(deps))
	}

	return r
}
