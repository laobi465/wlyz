// 平台超管 / 开发者 / 代理 处理器骨架
// 实际业务逻辑在各模块的 service 层实现，handler 仅做参数转发
package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/your-org/keyauth-saas/apps/server/internal/middleware"
)

// ============== 公共认证 ==============

// AdminLogin 超管登录
func AdminLogin(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		// TODO(v0.2.0): 用户名密码校验 + 2FA + 生成 JWT
		middleware.Fail(c, 501, 1006, "接口待实现：v0.2.0 交付")
	}
}

// TenantRegister 开发者注册
func TenantRegister(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		// TODO(v0.2.0): 注册 + 套餐分配 + 验证码
		middleware.Fail(c, 501, 1006, "接口待实现：v0.2.0 交付")
	}
}

// TenantLogin 开发者登录
func TenantLogin(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		// TODO(v0.2.0): 登录 + 2FA + JWT
		middleware.Fail(c, 501, 1006, "接口待实现：v0.2.0 交付")
	}
}

// AgentLogin 代理登录
func AgentLogin(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		// TODO(v0.2.0): 登录 + JWT
		middleware.Fail(c, 501, 1006, "接口待实现：v0.2.0 交付")
	}
}

// AgentRegister 代理注册（邀请码 + 支付注册费）
func AgentRegister(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		// TODO(v0.3.0): 邀请码校验 + 支付注册费 + 创建代理账号
		middleware.Fail(c, 501, 1006, "接口待实现：v0.3.0 交付")
	}
}

// PublicPlatformNotices 公开平台公告
func PublicPlatformNotices(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		// TODO(v0.3.0): 查询 type=platform AND status=published
		middleware.Fail(c, 501, 1006, "接口待实现：v0.3.0 交付")
	}
}

// ============== 平台超管 ==============

// AdminDashboard 平台看板（S-01）
func AdminDashboard(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		// TODO(v0.3.0): 全局统计
		middleware.Fail(c, 501, 1006, "接口待实现：v0.3.0 交付")
	}
}

// AdminListTenants 租户列表（S-02）
func AdminListTenants(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		// TODO(v0.2.0): 分页查询
		middleware.Fail(c, 501, 1006, "接口待实现：v0.2.0 交付")
	}
}

// AdminCreateTenant 创建租户
func AdminCreateTenant(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		middleware.Fail(c, 501, 1006, "接口待实现：v0.2.0 交付")
	}
}

// AdminUpdateTenant 更新租户
func AdminUpdateTenant(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		middleware.Fail(c, 501, 1006, "接口待实现：v0.2.0 交付")
	}
}

// AdminListPackages 套餐列表（S-03）
func AdminListPackages(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		middleware.Fail(c, 501, 1006, "接口待实现：v0.2.0 交付")
	}
}

// AdminCreatePackage 创建套餐
func AdminCreatePackage(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		middleware.Fail(c, 501, 1006, "接口待实现：v0.2.0 交付")
	}
}

// AdminListConfig 系统配置列表（S-07）
// 按 config_group 分组返回
func AdminListConfig(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		// TODO(v0.2.0): 按 group 分组查询 sys_config
		group := c.Query("group")
		rows, err := deps.CfgCache.ListByGroup(c.Request.Context(), group)
		if err != nil {
			middleware.Fail(c, 500, 1006, "查询配置失败")
			return
		}
		middleware.Success(c, gin.H{"list": rows})
	}
}

// AdminUpdateConfig 更新配置（铁律 05：保存即清缓存）
func AdminUpdateConfig(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		key := c.Param("key")
		var req struct {
			Value string `json:"value" binding:"required"`
			Name  string `json:"name"`
			Group string `json:"group"`
			Remark string `json:"remark"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			middleware.Fail(c, 400, 1001, "参数错误")
			return
		}
		// 保存 + 自动清缓存
		if err := deps.CfgCache.Set(c.Request.Context(), key, req.Value, req.Name, req.Group, req.Remark); err != nil {
			middleware.Fail(c, 500, 1006, "保存配置失败: "+err.Error())
			return
		}
		middleware.Success(c, nil)
	}
}

// AdminListNotices 公告列表（S-15/S-16）
func AdminListNotices(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		middleware.Fail(c, 501, 1006, "接口待实现：v0.3.0 交付")
	}
}

// AdminCreateNotice 创建公告
func AdminCreateNotice(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		middleware.Fail(c, 501, 1006, "接口待实现：v0.3.0 交付")
	}
}

// ============== 开发者控制台 ==============

// TenantDashboard 开发者工作台（D-01）
func TenantDashboard(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		// TODO(v0.3.0): 统计卡密数/在线数/今日销量/本月收入
		middleware.Fail(c, 501, 1006, "接口待实现：v0.3.0 交付")
	}
}

// TenantListApps 应用列表（D-02）
func TenantListApps(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		// TODO(v0.2.0): 按 tenant_id 查询
		middleware.Fail(c, 501, 1006, "接口待实现：v0.2.0 交付")
	}
}

// TenantCreateApp 创建应用
func TenantCreateApp(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		// TODO(v0.2.0): 生成 AppKey/AppSecret/SignSecret
		middleware.Fail(c, 501, 1006, "接口待实现：v0.2.0 交付")
	}
}

// TenantUpdateApp 更新应用
func TenantUpdateApp(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		middleware.Fail(c, 501, 1006, "接口待实现：v0.2.0 交付")
	}
}

// TenantListCards 卡密列表（D-03）
func TenantListCards(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		middleware.Fail(c, 501, 1006, "接口待实现：v0.2.0 交付")
	}
}

// TenantGenerateCards 批量生成卡密
// 调用 pkg/crypto.GenerateCardKey + 批量 INSERT
func TenantGenerateCards(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		// TODO(v0.2.0): 实现批量生成（参考卡密生成器）
		middleware.Fail(c, 501, 1006, "接口待实现：v0.2.0 交付")
	}
}

// TenantListAgents 代理列表（D-08）
func TenantListAgents(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		middleware.Fail(c, 501, 1006, "接口待实现：v0.3.0 交付")
	}
}

// TenantGenInviteCode 生成代理邀请码
func TenantGenInviteCode(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		middleware.Fail(c, 501, 1006, "接口待实现：v0.3.0 交付")
	}
}

// ============== 代理商控制台 ==============

// AgentDashboard 代理工作台（P-01）
func AgentDashboard(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		middleware.Fail(c, 501, 1006, "接口待实现：v0.3.0 交付")
	}
}

// AgentListCards 代理卡密列表（P-03）
func AgentListCards(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		middleware.Fail(c, 501, 1006, "接口待实现：v0.3.0 交付")
	}
}

// AgentGenerateCards 代理生成卡密（扣余额）
func AgentGenerateCards(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		// TODO(v0.3.0): 校验授权范围 → 扣余额 → 生成卡密
		middleware.Fail(c, 501, 1006, "接口待实现：v0.3.0 交付")
	}
}

// AgentListCommission 佣金明细（P-05）
func AgentListCommission(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		middleware.Fail(c, 501, 1006, "接口待实现：v0.3.0 交付")
	}
}

// AgentWithdraw 提现申请
func AgentWithdraw(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		middleware.Fail(c, 501, 1006, "接口待实现：v0.3.0 交付")
	}
}

// ============== 支付回调 ==============

// EpayNotify 平台总支付回调
func EpayNotify(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		// TODO(v0.2.0): 验签 + 处理订单 + 自动发卡
		middleware.Fail(c, 501, 1006, "接口待实现：v0.2.0 交付")
	}
}

// EpayReturn 平台总支付同步跳转
func EpayReturn(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Redirect(302, "/pay/result")
	}
}

// EpayTenantNotify 开发者自有易支付回调（按租户路由）
func EpayTenantNotify(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		// TODO(v0.3.0): 按租户隔离回调路由
		middleware.Fail(c, 501, 1006, "接口待实现：v0.3.0 交付")
	}
}
