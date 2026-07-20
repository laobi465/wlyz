// RiskEngine 风控评估中间件（v0.4.0 第十五项迁移）
// 严格遵循铁律 04：所有配置键名以常量声明
// 严格遵循铁律 05：所有开关/阈值走 sys_config 后台可视化编辑
//
// 设计说明：
//   - 登录评估：不作为中间件，而在 doLogin 登录成功后调用 EvaluateLogin
//     （因为需要 user_id，登录前无法评估）
//   - 验证评估：作为 client API 的可选中间件，对 verify/login 端点评估
//   - 中间件形式仅用于"匿名请求"风控（如未登录的扫描、爆破）
package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/your-org/keyauth-saas/apps/server/internal/risk"
)

// RiskEngineForAnonymous 匿名请求风控评估中间件
// 仅评估 abnormal_ua / high_frequency / abnormal_time（不依赖 user_id）
// 用于：登录前置评估、未鉴权的扫描接口等
// 命中 block 动作直接 403 拒绝；命中 challenge 不阻塞（登录流程后续处理）
func RiskEngineForAnonymous(mgr *risk.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		if mgr == nil {
			c.Next()
			return
		}

		ctx := c.Request.Context()
		ec := risk.EvalContext{
			ClientIP:  RealIP(c),
			UserAgent: c.Request.Header.Get("User-Agent"),
			Operation: c.Request.URL.Path,
		}

		out := mgr.EvaluateLogin(ctx, ec)
		if out.ShouldBlock {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"code":    1006,
				"message": "请求已被风控引擎拦截",
				"action":  risk.ActionBlock,
			})
			return
		}

		// 记录风控事件（异步由调用方完成，避免阻塞请求）
		if len(out.HitRules) > 0 {
			c.Set("risk_eval_output", out)
		}

		c.Next()
	}
}
