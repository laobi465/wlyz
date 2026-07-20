// Package handler v0.4.0 高级安全管理 API
// 严格遵循铁律 04：所有路由路径与字段名以常量声明
// 严格遵循铁律 05：所有阈值/开关走 sys_config 后台可视化编辑
//
// 路由组 /admin/security/risk/* 与 /admin/security/geo_alerts/*
// - 风控规则 CRUD（仅 custom 类型可创建/删除；内置规则仅可调阈值/启停）
// - 风控事件查询 + 确认
// - 异地登录告警查询 + 确认 + 关闭
// - 风控看板统计
package handler

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/your-org/keyauth-saas/apps/server/internal/middleware"
	"github.com/your-org/keyauth-saas/apps/server/internal/model"
	"github.com/your-org/keyauth-saas/apps/server/internal/risk"
)

// ============== 风控规则 CRUD ==============

// AdminListRiskRules GET /admin/security/risk/rules
// 列出所有风控规则（含 disabled）
func AdminListRiskRules(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if deps.RiskMgr == nil {
			middleware.Fail(c, http.StatusServiceUnavailable, 5001, "风控引擎未启用")
			return
		}
		rules, err := deps.RiskMgr.ListRules(c.Request.Context())
		if err != nil {
			middleware.Fail(c, http.StatusInternalServerError, 5002, "查询规则失败: "+err.Error())
			return
		}
		middleware.Success(c, gin.H{"list": rules, "total": len(rules)})
	}
}

// AdminGetRiskRule GET /admin/security/risk/rules/:id
func AdminGetRiskRule(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if deps.RiskMgr == nil {
			middleware.Fail(c, http.StatusServiceUnavailable, 5001, "风控引擎未启用")
			return
		}
		id, err := strconv.ParseUint(c.Param("id"), 10, 64)
		if err != nil {
			middleware.Fail(c, http.StatusBadRequest, 4001, "无效的规则 ID")
			return
		}
		rule, err := deps.RiskMgr.GetRule(c.Request.Context(), id)
		if err != nil {
			middleware.Fail(c, http.StatusNotFound, 4041, "规则不存在")
			return
		}
		middleware.Success(c, rule)
	}
}

// adminCreateRiskRuleReq 创建规则请求体
type adminCreateRiskRuleReq struct {
	Name        string `json:"name" binding:"required,max=64"`
	Description string `json:"description" binding:"max=255"`
	RuleType    string `json:"rule_type" binding:"required,oneof=custom"`
	Condition   string `json:"condition" binding:"required"`
	Score       int    `json:"score" binding:"min=0,max=100"`
	Action      string `json:"action" binding:"omitempty,oneof=alert challenge block"`
	Priority    int    `json:"priority" binding:"min=0"`
	Status      string `json:"status" binding:"omitempty,oneof=active disabled"`
}

// AdminCreateRiskRule POST /admin/security/risk/rules
// 仅允许创建 custom 类型规则（内置规则由 migration seed）
func AdminCreateRiskRule(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if deps.RiskMgr == nil {
			middleware.Fail(c, http.StatusServiceUnavailable, 5001, "风控引擎未启用")
			return
		}
		var req adminCreateRiskRuleReq
		if err := c.ShouldBindJSON(&req); err != nil {
			middleware.Fail(c, http.StatusBadRequest, 4001, "参数错误: "+err.Error())
			return
		}

		// 当前管理员用户名（用于审计）
		creator := "admin"
		if v, ok := c.Get("username"); ok {
			if s, ok := v.(string); ok && s != "" {
				creator = s
			}
		}

		rule := &model.RiskRule{
			Name:        req.Name,
			Description: req.Description,
			RuleType:    req.RuleType,
			Condition:   req.Condition,
			Score:       req.Score,
			Action:      req.Action,
			Priority:    req.Priority,
			Status:      req.Status,
			CreatedBy:   creator,
		}
		if err := deps.RiskMgr.CreateRule(c.Request.Context(), rule); err != nil {
			middleware.Fail(c, http.StatusBadRequest, 4002, "创建规则失败: "+err.Error())
			return
		}
		middleware.Success(c, rule)
	}
}

// adminUpdateRiskRuleReq 更新规则请求体
type adminUpdateRiskRuleReq struct {
	Name        *string `json:"name" binding:"omitempty,max=64"`
	Description *string `json:"description" binding:"omitempty,max=255"`
	Condition   *string `json:"condition"`
	Score       *int    `json:"score" binding:"omitempty,min=0,max=100"`
	Action      *string `json:"action" binding:"omitempty,oneof=alert challenge block"`
	Priority    *int    `json:"priority" binding:"omitempty,min=0"`
	Status      *string `json:"status" binding:"omitempty,oneof=active disabled"`
}

// AdminUpdateRiskRule PUT /admin/security/risk/rules/:id
func AdminUpdateRiskRule(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if deps.RiskMgr == nil {
			middleware.Fail(c, http.StatusServiceUnavailable, 5001, "风控引擎未启用")
			return
		}
		id, err := strconv.ParseUint(c.Param("id"), 10, 64)
		if err != nil {
			middleware.Fail(c, http.StatusBadRequest, 4001, "无效的规则 ID")
			return
		}

		var req adminUpdateRiskRuleReq
		if err := c.ShouldBindJSON(&req); err != nil {
			middleware.Fail(c, http.StatusBadRequest, 4001, "参数错误: "+err.Error())
			return
		}

		updates := map[string]interface{}{}
		if req.Name != nil {
			updates["name"] = *req.Name
		}
		if req.Description != nil {
			updates["description"] = *req.Description
		}
		if req.Condition != nil {
			updates["condition"] = *req.Condition
		}
		if req.Score != nil {
			updates["score"] = *req.Score
		}
		if req.Action != nil {
			updates["action"] = *req.Action
		}
		if req.Priority != nil {
			updates["priority"] = *req.Priority
		}
		if req.Status != nil {
			updates["status"] = *req.Status
		}

		if err := deps.RiskMgr.UpdateRule(c.Request.Context(), id, updates); err != nil {
			middleware.Fail(c, http.StatusBadRequest, 4002, "更新规则失败: "+err.Error())
			return
		}
		middleware.Success(c, gin.H{"id": id, "updated": true})
	}
}

// AdminDeleteRiskRule DELETE /admin/security/risk/rules/:id
// 内置规则（created_by=system）禁止删除
func AdminDeleteRiskRule(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if deps.RiskMgr == nil {
			middleware.Fail(c, http.StatusServiceUnavailable, 5001, "风控引擎未启用")
			return
		}
		id, err := strconv.ParseUint(c.Param("id"), 10, 64)
		if err != nil {
			middleware.Fail(c, http.StatusBadRequest, 4001, "无效的规则 ID")
			return
		}
		if err := deps.RiskMgr.DeleteRule(c.Request.Context(), id); err != nil {
			middleware.Fail(c, http.StatusBadRequest, 4003, "删除规则失败: "+err.Error())
			return
		}
		middleware.Success(c, gin.H{"id": id, "deleted": true})
	}
}

// ============== 风控事件查询 + 确认 ==============

// AdminListRiskEvents GET /admin/security/risk/events
// 支持 user_type/rule_type/action/client_ip/acknowledged/start_date/end_date 筛选
func AdminListRiskEvents(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if deps.RiskMgr == nil {
			middleware.Fail(c, http.StatusServiceUnavailable, 5001, "风控引擎未启用")
			return
		}
		page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
		if page < 1 {
			page = 1
		}
		pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
		if pageSize < 1 || pageSize > 200 {
			pageSize = 20
		}

		params := risk.ListEventsParams{
			Page:       page,
			PageSize:   pageSize,
			UserType:   strings.TrimSpace(c.Query("user_type")),
			RuleType:   strings.TrimSpace(c.Query("rule_type")),
			Action:     strings.TrimSpace(c.Query("action")),
			ClientIP:   strings.TrimSpace(c.Query("client_ip")),
			StartDate:  strings.TrimSpace(c.Query("start_date")),
			EndDate:    strings.TrimSpace(c.Query("end_date")),
		}
		if ack := c.Query("acknowledged"); ack != "" {
			b := ack == "1" || ack == "true"
			params.Acknowledged = &b
		}

		events, total, err := deps.RiskMgr.ListEvents(c.Request.Context(), params)
		if err != nil {
			middleware.Fail(c, http.StatusInternalServerError, 5002, "查询事件失败: "+err.Error())
			return
		}
		middleware.Success(c, gin.H{
			"list":      events,
			"total":     total,
			"page":      page,
			"page_size": pageSize,
		})
	}
}

// AdminAckRiskEvent POST /admin/security/risk/events/:id/acknowledge
func AdminAckRiskEvent(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if deps.RiskMgr == nil {
			middleware.Fail(c, http.StatusServiceUnavailable, 5001, "风控引擎未启用")
			return
		}
		id, err := strconv.ParseUint(c.Param("id"), 10, 64)
		if err != nil {
			middleware.Fail(c, http.StatusBadRequest, 4001, "无效的事件 ID")
			return
		}

		acknowledger := "admin"
		if v, ok := c.Get("username"); ok {
			if s, ok := v.(string); ok && s != "" {
				acknowledger = s
			}
		}

		if err := deps.RiskMgr.AcknowledgeEvent(c.Request.Context(), id, acknowledger); err != nil {
			middleware.Fail(c, http.StatusInternalServerError, 5002, "确认事件失败: "+err.Error())
			return
		}
		middleware.Success(c, gin.H{"id": id, "acknowledged": true})
	}
}

// ============== 异地登录告警查询 + 确认 + 关闭 ==============

// AdminListGeoAlerts GET /admin/security/geo_alerts
func AdminListGeoAlerts(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if deps.RiskMgr == nil {
			middleware.Fail(c, http.StatusServiceUnavailable, 5001, "风控引擎未启用")
			return
		}
		page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
		if page < 1 {
			page = 1
		}
		pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
		if pageSize < 1 || pageSize > 200 {
			pageSize = 20
		}

		params := risk.ListGeoAlertParams{
			Page:        page,
			PageSize:    pageSize,
			UserType:    strings.TrimSpace(c.Query("user_type")),
			AlertStatus: strings.TrimSpace(c.Query("alert_status")),
			StartDate:   strings.TrimSpace(c.Query("start_date")),
			EndDate:     strings.TrimSpace(c.Query("end_date")),
		}

		alerts, total, err := deps.RiskMgr.ListGeoAlerts(c.Request.Context(), params)
		if err != nil {
			middleware.Fail(c, http.StatusInternalServerError, 5002, "查询告警失败: "+err.Error())
			return
		}
		middleware.Success(c, gin.H{
			"list":      alerts,
			"total":     total,
			"page":      page,
			"page_size": pageSize,
		})
	}
}

// AdminAckGeoAlert POST /admin/security/geo_alerts/:id/acknowledge
func AdminAckGeoAlert(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if deps.RiskMgr == nil {
			middleware.Fail(c, http.StatusServiceUnavailable, 5001, "风控引擎未启用")
			return
		}
		id, err := strconv.ParseUint(c.Param("id"), 10, 64)
		if err != nil {
			middleware.Fail(c, http.StatusBadRequest, 4001, "无效的告警 ID")
			return
		}

		acknowledger := "admin"
		if v, ok := c.Get("username"); ok {
			if s, ok := v.(string); ok && s != "" {
				acknowledger = s
			}
		}

		if err := deps.RiskMgr.AcknowledgeGeoAlert(c.Request.Context(), id, acknowledger); err != nil {
			middleware.Fail(c, http.StatusInternalServerError, 5002, "确认告警失败: "+err.Error())
			return
		}
		middleware.Success(c, gin.H{"id": id, "acknowledged": true})
	}
}

// AdminCloseGeoAlert POST /admin/security/geo_alerts/:id/close
func AdminCloseGeoAlert(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if deps.RiskMgr == nil {
			middleware.Fail(c, http.StatusServiceUnavailable, 5001, "风控引擎未启用")
			return
		}
		id, err := strconv.ParseUint(c.Param("id"), 10, 64)
		if err != nil {
			middleware.Fail(c, http.StatusBadRequest, 4001, "无效的告警 ID")
			return
		}

		acknowledger := "admin"
		if v, ok := c.Get("username"); ok {
			if s, ok := v.(string); ok && s != "" {
				acknowledger = s
			}
		}

		if err := deps.RiskMgr.CloseGeoAlert(c.Request.Context(), id, acknowledger); err != nil {
			middleware.Fail(c, http.StatusInternalServerError, 5002, "关闭告警失败: "+err.Error())
			return
		}
		middleware.Success(c, gin.H{"id": id, "closed": true})
	}
}

// ============== 风控看板 ==============

// AdminRiskStats GET /admin/security/risk/stats
// 返回今日/本周风控事件统计 + 待处理告警数 + TOP 异常 IP + 最近 10 条事件
func AdminRiskStats(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if deps.RiskMgr == nil {
			middleware.Fail(c, http.StatusServiceUnavailable, 5001, "风控引擎未启用")
			return
		}
		stats, err := deps.RiskMgr.GetStats(c.Request.Context())
		if err != nil {
			middleware.Fail(c, http.StatusInternalServerError, 5002, "获取统计失败: "+err.Error())
			return
		}
		middleware.Success(c, stats)
	}
}
