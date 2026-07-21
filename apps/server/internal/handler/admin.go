// 平台超管系统配置 Handler
// 注：业务接口（Dashboard / Tenants / Packages / Agents / Notices / Logs / Security）
//
//	已迁移至 admin_business.go（v0.3.0）
//	登录/注册/刷新/登出 接口已迁移至 auth.go
package handler

import (
	"strconv"

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
// v0.6.7 P0 修复：对关键安全参数增加最小值校验
// 防止管理员配置异常值导致前端死循环（如 jwt.access_ttl_seconds=60 触发 scheduleRefresh 死循环）
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

		// v0.6.7 P0 修复：关键参数最小值校验
		if err := validateSysConfigValue(key, req.Value); err != nil {
			middleware.Fail(c, 400, 1002, err.Error())
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

// validateSysConfigValue 校验关键 sys_config 值的合法性
// v0.6.7 P0 修复：防止异常值导致前端 scheduleRefresh 死循环（管理员登录后死机 bug）
// jwt.access_ttl_seconds 必须 >= 600（10 分钟），否则前端「提前 5 分钟续期」逻辑会立即触发刷新
// jwt.refresh_ttl_seconds 必须 >= access_ttl_seconds + 300，且 >= 3600
func validateSysConfigValue(key, value string) error {
	switch key {
	case "jwt.access_ttl_seconds":
		v, err := strconv.Atoi(value)
		if err != nil {
			return errInvalidConfigValue("jwt.access_ttl_seconds 必须是整数")
		}
		if v < 600 {
			return errInvalidConfigValue("jwt.access_ttl_seconds 不能小于 600 秒（10 分钟），否则会触发前端高频刷新")
		}
		if v > 86400*30 {
			return errInvalidConfigValue("jwt.access_ttl_seconds 不能大于 30 天")
		}
	case "jwt.refresh_ttl_seconds":
		v, err := strconv.Atoi(value)
		if err != nil {
			return errInvalidConfigValue("jwt.refresh_ttl_seconds 必须是整数")
		}
		if v < 3600 {
			return errInvalidConfigValue("jwt.refresh_ttl_seconds 不能小于 3600 秒（1 小时）")
		}
		if v > 86400*90 {
			return errInvalidConfigValue("jwt.refresh_ttl_seconds 不能大于 90 天")
		}
	}
	return nil
}

// errInvalidConfigValue 构造配置值非法错误
type configValueError struct{ msg string }

func (e *configValueError) Error() string { return e.msg }

func errInvalidConfigValue(msg string) error { return &configValueError{msg: msg} }
