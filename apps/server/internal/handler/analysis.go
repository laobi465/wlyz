// Package handler v0.6.0 高级分析 API
// 严格遵循铁律 04：所有路由路径与字段名以常量声明
// 严格遵循铁律 05：所有阈值/开关走 sys_config 后台可视化编辑
//
// 路由组 /admin/analysis/*：
//   - /behavior/overview          用户行为总览（KPI）
//   - /behavior/users             用户行为列表（分页）
//   - /behavior/users/:id         单用户行为详情
//   - /behavior/trend             全局行为趋势
//   - /card_profile/overview      卡密画像总览
//   - /card_profile/cards         卡密画像列表（分页，卡密脱敏）
//   - /card_profile/cards/:id     单卡密画像详情
//   - /card_profile/trend         全局卡密趋势
//   - /risk/overview              风险用户总览
//   - /risk/users                 风险用户列表（分页 + level 过滤）
//   - /risk/users/:user_type/:id  风险用户详情
//   - /risk/users/:user_type/:id/ban    手动封禁
//   - /risk/users/:user_type/:id/unban  手动解封
//   - /risk/reevaluate/:user_type/:id   手动触发评分重算
//   - /risk/reevaluate_all              批量重算（运维场景）
//   - /aggregate/trigger                手动触发一次聚合（运维场景）
package handler

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/your-org/keyauth-saas/apps/server/internal/analysis"
	"github.com/your-org/keyauth-saas/apps/server/internal/logger"
	"github.com/your-org/keyauth-saas/apps/server/internal/middleware"
)

// ============== 内部辅助 ==============

// requireAnalysisMgr 校验 AnalysisMgr 是否注入
func requireAnalysisMgr(deps *Deps, c *gin.Context) bool {
	if deps.AnalysisMgr == nil {
		middleware.Fail(c, http.StatusServiceUnavailable, 5001, "高级分析模块未启用")
		return false
	}
	return true
}

// parseAnalysisFilter 从 query 解析通用过滤参数
// tenant_id / app_id / user_type / level / page / page_size
func parseAnalysisFilter(c *gin.Context) analysis.Filter {
	f := analysis.Filter{
		UserType: strings.TrimSpace(c.Query("user_type")),
		Level:    strings.TrimSpace(c.Query("level")),
	}
	if v, err := strconv.ParseUint(c.Query("tenant_id"), 10, 64); err == nil {
		f.TenantID = v
	}
	if v, err := strconv.ParseUint(c.Query("app_id"), 10, 64); err == nil {
		f.AppID = v
	}
	if v, err := strconv.Atoi(c.DefaultQuery("page", "1")); err == nil {
		f.Page = v
	}
	if v, err := strconv.Atoi(c.DefaultQuery("page_size", "20")); err == nil {
		f.PageSize = v
	}
	return f
}

// ============== 1. 用户行为分析 ==============

// AdminBehaviorOverview GET /admin/analysis/behavior/overview
// 返回回溯周期内终端用户行为 KPI
func AdminBehaviorOverview(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !requireAnalysisMgr(deps, c) {
			return
		}
		f := parseAnalysisFilter(c)
		ov, err := deps.AnalysisMgr.GetBehaviorOverview(c.Request.Context(), f)
		if err != nil {
			logger.Error("analysis: get behavior overview failed", "err", err)
			middleware.Fail(c, http.StatusInternalServerError, 5002, "查询行为总览失败")
			return
		}
		middleware.Success(c, ov)
	}
}

// AdminListUserBehaviors GET /admin/analysis/behavior/users
// 分页列出终端用户行为汇总（按活跃度降序）
func AdminListUserBehaviors(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !requireAnalysisMgr(deps, c) {
			return
		}
		f := parseAnalysisFilter(c)
		users, total, err := deps.AnalysisMgr.ListUserBehaviors(c.Request.Context(), f)
		if err != nil {
			middleware.Fail(c, http.StatusInternalServerError, 5002, "查询用户行为列表失败: "+err.Error())
			return
		}
		middleware.Success(c, gin.H{
			"list":      users,
			"total":     total,
			"page":      f.Page,
			"page_size": f.PageSize,
		})
	}
}

// AdminGetUserBehaviorDetail GET /admin/analysis/behavior/users/:id
// 单用户行为详情（按日序列 + 汇总）
// query: days（回溯天数，默认走 sys_config analysis.lookback_days）
func AdminGetUserBehaviorDetail(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !requireAnalysisMgr(deps, c) {
			return
		}
		uid, err := strconv.ParseUint(c.Param("id"), 10, 64)
		if err != nil || uid == 0 {
			middleware.Fail(c, http.StatusBadRequest, 4001, "无效的用户 ID")
			return
		}
		days, _ := strconv.Atoi(c.DefaultQuery("days", "0"))
		detail, err := deps.AnalysisMgr.GetUserBehaviorDetail(c.Request.Context(), uid, days)
		if err != nil {
			middleware.Fail(c, http.StatusInternalServerError, 5002, "查询用户行为详情失败: "+err.Error())
			return
		}
		middleware.Success(c, detail)
	}
}

// AdminBehaviorTrend GET /admin/analysis/behavior/trend
// 全局行为趋势（按日序列）
// query: days（默认走 sys_config analysis.lookback_days）
func AdminBehaviorTrend(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !requireAnalysisMgr(deps, c) {
			return
		}
		f := parseAnalysisFilter(c)
		days, _ := strconv.Atoi(c.DefaultQuery("days", "0"))
		trend, err := deps.AnalysisMgr.GetBehaviorTrend(c.Request.Context(), f, days)
		if err != nil {
			middleware.Fail(c, http.StatusInternalServerError, 5002, "查询行为趋势失败: "+err.Error())
			return
		}
		middleware.Success(c, gin.H{"list": trend})
	}
}

// ============== 2. 卡密使用画像 ==============

// AdminCardProfileOverview GET /admin/analysis/card_profile/overview
func AdminCardProfileOverview(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !requireAnalysisMgr(deps, c) {
			return
		}
		f := parseAnalysisFilter(c)
		ov, err := deps.AnalysisMgr.GetCardProfileOverview(c.Request.Context(), f)
		if err != nil {
			middleware.Fail(c, http.StatusInternalServerError, 5002, "查询卡密画像总览失败: "+err.Error())
			return
		}
		middleware.Success(c, ov)
	}
}

// AdminListCardProfiles GET /admin/analysis/card_profile/cards
// 分页列出卡密画像（卡密号脱敏）
func AdminListCardProfiles(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !requireAnalysisMgr(deps, c) {
			return
		}
		f := parseAnalysisFilter(c)
		cards, total, err := deps.AnalysisMgr.ListCardProfiles(c.Request.Context(), f)
		if err != nil {
			middleware.Fail(c, http.StatusInternalServerError, 5002, "查询卡密画像列表失败: "+err.Error())
			return
		}
		middleware.Success(c, gin.H{
			"list":      cards,
			"total":     total,
			"page":      f.Page,
			"page_size": f.PageSize,
		})
	}
}

// AdminGetCardProfileDetail GET /admin/analysis/card_profile/cards/:id
func AdminGetCardProfileDetail(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !requireAnalysisMgr(deps, c) {
			return
		}
		cid, err := strconv.ParseUint(c.Param("id"), 10, 64)
		if err != nil || cid == 0 {
			middleware.Fail(c, http.StatusBadRequest, 4001, "无效的卡密 ID")
			return
		}
		days, _ := strconv.Atoi(c.DefaultQuery("days", "0"))
		detail, err := deps.AnalysisMgr.GetCardProfileDetail(c.Request.Context(), cid, days)
		if err != nil {
			middleware.Fail(c, http.StatusInternalServerError, 5002, "查询卡密画像详情失败: "+err.Error())
			return
		}
		middleware.Success(c, detail)
	}
}

// AdminCardProfileTrend GET /admin/analysis/card_profile/trend
func AdminCardProfileTrend(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !requireAnalysisMgr(deps, c) {
			return
		}
		f := parseAnalysisFilter(c)
		days, _ := strconv.Atoi(c.DefaultQuery("days", "0"))
		trend, err := deps.AnalysisMgr.GetCardProfileTrend(c.Request.Context(), f, days)
		if err != nil {
			middleware.Fail(c, http.StatusInternalServerError, 5002, "查询卡密趋势失败: "+err.Error())
			return
		}
		middleware.Success(c, gin.H{"list": trend})
	}
}

// ============== 3. 风险用户识别 ==============

// AdminRiskUserOverview GET /admin/analysis/risk/overview
func AdminRiskUserOverview(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !requireAnalysisMgr(deps, c) {
			return
		}
		f := parseAnalysisFilter(c)
		ov, err := deps.AnalysisMgr.GetRiskUserOverview(c.Request.Context(), f)
		if err != nil {
			middleware.Fail(c, http.StatusInternalServerError, 5002, "查询风险用户总览失败: "+err.Error())
			return
		}
		middleware.Success(c, ov)
	}
}

// AdminListRiskUsers GET /admin/analysis/risk/users
// 分页列出风险用户，支持 level 过滤（low/medium/high/critical）
func AdminListRiskUsers(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !requireAnalysisMgr(deps, c) {
			return
		}
		f := parseAnalysisFilter(c)
		users, total, err := deps.AnalysisMgr.ListRiskUsers(c.Request.Context(), f)
		if err != nil {
			middleware.Fail(c, http.StatusInternalServerError, 5002, "查询风险用户列表失败: "+err.Error())
			return
		}
		middleware.Success(c, gin.H{
			"list":      users,
			"total":     total,
			"page":      f.Page,
			"page_size": f.PageSize,
		})
	}
}

// AdminGetRiskUserDetail GET /admin/analysis/risk/users/:user_type/:id
// 单风险用户详情（含最近 N 条风控事件）
// query: recent_events（默认 20，上限 100）
func AdminGetRiskUserDetail(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !requireAnalysisMgr(deps, c) {
			return
		}
		userType := strings.TrimSpace(c.Param("user_type"))
		uid, err := strconv.ParseUint(c.Param("id"), 10, 64)
		if err != nil || uid == 0 || userType == "" {
			middleware.Fail(c, http.StatusBadRequest, 4001, "参数错误：user_type 或 id 非法")
			return
		}
		recent, _ := strconv.Atoi(c.DefaultQuery("recent_events", "20"))
		detail, err := deps.AnalysisMgr.GetRiskUserDetail(c.Request.Context(), userType, uid, recent)
		if err != nil {
			middleware.Fail(c, http.StatusInternalServerError, 5002, "查询风险用户详情失败: "+err.Error())
			return
		}
		middleware.Success(c, detail)
	}
}

// adminBanReq 手动封禁请求体
type adminBanReq struct {
	Reason string `json:"reason" binding:"required,max=255"`
}

// AdminBanRiskUser POST /admin/analysis/risk/users/:user_type/:id/ban
// 手动封禁（写入 user_risk_score.banned=true + 封禁原因）
// 注意：仅修改分析表标记，实际账号封禁需调用 admin/enduser 接口
func AdminBanRiskUser(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !requireAnalysisMgr(deps, c) {
			return
		}
		userType := strings.TrimSpace(c.Param("user_type"))
		uid, err := strconv.ParseUint(c.Param("id"), 10, 64)
		if err != nil || uid == 0 || userType == "" {
			middleware.Fail(c, http.StatusBadRequest, 4001, "参数错误：user_type 或 id 非法")
			return
		}
		var req adminBanReq
		if err := c.ShouldBindJSON(&req); err != nil {
			middleware.Fail(c, http.StatusBadRequest, 4001, "参数错误: "+err.Error())
			return
		}
		if err := deps.AnalysisMgr.BanUser(c.Request.Context(), userType, uid, req.Reason); err != nil {
			middleware.Fail(c, http.StatusInternalServerError, 5002, "封禁失败: "+err.Error())
			return
		}
		middleware.Success(c, gin.H{
			"user_type":     userType,
			"user_id":       uid,
			"banned":        true,
			"banned_reason": req.Reason,
		})
	}
}

// AdminUnbanRiskUser POST /admin/analysis/risk/users/:user_type/:id/unban
// 手动解封
func AdminUnbanRiskUser(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !requireAnalysisMgr(deps, c) {
			return
		}
		userType := strings.TrimSpace(c.Param("user_type"))
		uid, err := strconv.ParseUint(c.Param("id"), 10, 64)
		if err != nil || uid == 0 || userType == "" {
			middleware.Fail(c, http.StatusBadRequest, 4001, "参数错误：user_type 或 id 非法")
			return
		}
		if err := deps.AnalysisMgr.UnbanUser(c.Request.Context(), userType, uid); err != nil {
			middleware.Fail(c, http.StatusInternalServerError, 5002, "解封失败: "+err.Error())
			return
		}
		middleware.Success(c, gin.H{
			"user_type": userType,
			"user_id":   uid,
			"banned":    false,
		})
	}
}

// AdminReevaluateRiskUser POST /admin/analysis/risk/reevaluate/:user_type/:id
// 手动触发单个用户的风险评分重算
func AdminReevaluateRiskUser(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !requireAnalysisMgr(deps, c) {
			return
		}
		userType := strings.TrimSpace(c.Param("user_type"))
		uid, err := strconv.ParseUint(c.Param("id"), 10, 64)
		if err != nil || uid == 0 || userType == "" {
			middleware.Fail(c, http.StatusBadRequest, 4001, "参数错误：user_type 或 id 非法")
			return
		}
		if _, err := deps.AnalysisMgr.ReevaluateUserRiskScore(c.Request.Context(), userType, uid); err != nil {
			middleware.Fail(c, http.StatusInternalServerError, 5002, "评分重算失败: "+err.Error())
			return
		}
		// 重算后回读最新评分
		detail, err := deps.AnalysisMgr.GetRiskUserDetail(c.Request.Context(), userType, uid, 10)
		if err != nil {
			middleware.Success(c, gin.H{
				"user_type":   userType,
				"user_id":     uid,
				"reevaluated": true,
			})
			return
		}
		middleware.Success(c, gin.H{
			"user_type":   userType,
			"user_id":     uid,
			"reevaluated": true,
			"summary":     detail.Summary,
		})
	}
}

// AdminReevaluateAllRiskUsers POST /admin/analysis/risk/reevaluate_all
// 批量重算所有用户的风险评分（运维场景，按需触发）
func AdminReevaluateAllRiskUsers(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !requireAnalysisMgr(deps, c) {
			return
		}
		count, err := deps.AnalysisMgr.ReevaluateAllRiskScores(c.Request.Context())
		if err != nil {
			middleware.Fail(c, http.StatusInternalServerError, 5002, "批量重算失败: "+err.Error())
			return
		}
		middleware.Success(c, gin.H{
			"reevaluated": true,
			"count":       count,
		})
	}
}

// AdminTriggerAggregation POST /admin/analysis/aggregate/trigger
// 手动触发一次聚合（运维场景，立即聚合昨日+今日数据 + 重算风险评分）
func AdminTriggerAggregation(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !requireAnalysisMgr(deps, c) {
			return
		}
		users, cards, risk, err := deps.AnalysisMgr.RunAggregationOnceSync(c.Request.Context())
		if err != nil {
			middleware.Fail(c, http.StatusInternalServerError, 5002, "聚合失败: "+err.Error())
			return
		}
		middleware.Success(c, gin.H{
			"triggered":        true,
			"users_aggregated": users,
			"cards_aggregated": cards,
			"risk_reevaluated": risk,
		})
	}
}
