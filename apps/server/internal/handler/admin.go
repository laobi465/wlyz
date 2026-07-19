// 平台超管系统配置 Handler
// 注：业务接口（Dashboard / Tenants / Packages / Agents / Notices / Logs / Security）
//     已迁移至 admin_business.go（v0.3.0）
//     登录/注册/刷新/登出 接口已迁移至 auth.go
package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/your-org/keyauth-saas/apps/server/internal/middleware"
)

// AdminListConfig 系统配置列表（S-07）
// 按 config_group 分组返回
func AdminListConfig(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
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
			Value  string `json:"value" binding:"required"`
			Name   string `json:"name"`
			Group  string `json:"group"`
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
