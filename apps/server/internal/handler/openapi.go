// v0.4.0 API 开放平台 Handler
// 严格遵循铁律 04/05/06：
//   04 - Token 前缀/长度/TTL/Webhook 超时/重试次数/失败阈值 全部从 sys_config 读取
//   05 - 8 项 openapi.* / webhook.* 配置可通过后台「系统配置」实时调整
//   06 - Token SHA-512 哈希存储（明文仅生成时返回一次）；Webhook secret AES-256-GCM 加密；HMAC-SHA256 签名 + hmac.Equal 常量时间比较
//
// 接口分三组：
//   1. adminAuth 下：开放平台配置概览（只读）
//   2. tenantAuth 下：Token / Webhook / Delivery 全套 CRUD（租户隔离）
//   3. openapiAuth 下（API Token 鉴权）：第三方调用方信息查询 + 业务数据只读访问
package handler

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/your-org/keyauth-saas/apps/server/internal/middleware"
	"github.com/your-org/keyauth-saas/apps/server/internal/model"
	"github.com/your-org/keyauth-saas/apps/server/internal/openapi"
)

// ============== 平台管理端（adminAuth） ==============

// AdminOpenAPIStatus GET /admin/openapi/status
// 开放平台配置概览 + 全局统计
func AdminOpenAPIStatus(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		// 配置概览
		configs := gin.H{
			"token_prefix":           deps.CfgCache.GetString(ctx, openapi.CfgKeyTokenPrefix, "pat_"),
			"token_length":           deps.CfgCache.GetInt(ctx, openapi.CfgKeyTokenLength, 40),
			"token_max_per_tenant":   deps.CfgCache.GetInt(ctx, openapi.CfgKeyTokenMaxPerTenant, 10),
			"token_default_ttl_days": deps.CfgCache.GetInt(ctx, openapi.CfgKeyTokenDefaultTTLDays, 365),
			"scope_available":        deps.CfgCache.GetString(ctx, openapi.CfgKeyScopeAvailable, ""),
			"webhook_timeout":        deps.CfgCache.GetInt(ctx, openapi.CfgKeyWebhookTimeout, 10),
			"webhook_max_retry":      deps.CfgCache.GetInt(ctx, openapi.CfgKeyWebhookMaxRetry, 3),
			"webhook_fail_threshold": deps.CfgCache.GetInt(ctx, openapi.CfgKeyWebhookFailThreshold, 10),
		}

		// 全局统计
		var tokenTotal, tokenActive, endpointTotal, endpointActive, deliveryTotal, deliveryFailed int64
		deps.DB.Model(&model.DeveloperAPIToken{}).Count(&tokenTotal)
		deps.DB.Model(&model.DeveloperAPIToken{}).Where("status = ?", openapi.TokenStatusActive).Count(&tokenActive)
		deps.DB.Model(&model.WebhookEndpoint{}).Count(&endpointTotal)
		deps.DB.Model(&model.WebhookEndpoint{}).Where("status = ?", openapi.EndpointStatusActive).Count(&endpointActive)
		deps.DB.Model(&model.WebhookDelivery{}).Count(&deliveryTotal)
		deps.DB.Model(&model.WebhookDelivery{}).Where("status = ?", openapi.DeliveryStatusFailed).Count(&deliveryFailed)

		middleware.Success(c, gin.H{
			"configs": configs,
			"stats": gin.H{
				"token_total":         tokenTotal,
				"token_active":        tokenActive,
				"endpoint_total":      endpointTotal,
				"endpoint_active":     endpointActive,
				"delivery_total":      deliveryTotal,
				"delivery_failed":     deliveryFailed,
			},
		})
	}
}

// ============== 租户控制台（tenantAuth） ==============

// ----- Token 管理 -----

// TenantListAPITokens GET /tenant/openapi/tokens?status=&page=&page_size=
func TenantListAPITokens(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		tenantID := getTenantID(c)
		status := c.Query("status")
		page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
		pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

		mgr := openapi.NewTokenManager(deps.DB, deps.CfgCache)
		items, total, err := mgr.ListTokens(ctx, tenantID, status, page, pageSize)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "查询失败: " + err.Error()})
			return
		}
		middleware.Success(c, gin.H{
			"items": items,
			"total": total,
			"page":  page,
		})
	}
}

// TenantCreateAPIToken POST /tenant/openapi/tokens
// 创建后返回明文 token（仅此一次，后续无法再获取）
func TenantCreateAPIToken(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		tenantID := getTenantID(c)
		var req struct {
			Name     string `json:"name" binding:"required"`
			Scopes   string `json:"scopes"`
			TTLDays  int    `json:"ttl_days"` // 0=永久, -1=用默认, >0=指定天数
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "参数错误: " + err.Error()})
			return
		}
		mgr := openapi.NewTokenManager(deps.DB, deps.CfgCache)
		plainToken, token, err := mgr.GenerateToken(ctx, tenantID, req.Name, req.Scopes, req.TTLDays)
		if err != nil {
			code := 500
			if errors.Is(err, openapi.ErrInvalidScope) {
				code = 400
			} else if errors.Is(err, openapi.ErrTokenLimitExceeded) {
				code = 409
			}
			c.JSON(code, gin.H{"code": code, "message": err.Error()})
			return
		}
		uid := getUserID(c)
		RecordOperation(deps, c, "openapi", "create_token", "success", "token", &uid, map[string]interface{}{
			"token_id": token.ID, "name": token.Name, "scopes": token.Scopes,
		})
		// 返回明文 token（仅此一次）+ token 元信息
		middleware.Success(c, gin.H{
			"plain_token": plainToken, // 警告：仅此一次返回
			"token":       token,
		})
	}
}

// TenantGetAPIToken GET /tenant/openapi/tokens/:id
func TenantGetAPIToken(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		tenantID := getTenantID(c)
		id, err := strconv.ParseUint(c.Param("id"), 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "无效的 ID"})
			return
		}
		mgr := openapi.NewTokenManager(deps.DB, deps.CfgCache)
		token, err := mgr.GetToken(ctx, tenantID, id)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "Token 不存在"})
			return
		}
		middleware.Success(c, token)
	}
}

// TenantRevokeAPIToken DELETE /tenant/openapi/tokens/:id
func TenantRevokeAPIToken(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		tenantID := getTenantID(c)
		id, err := strconv.ParseUint(c.Param("id"), 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "无效的 ID"})
			return
		}
		mgr := openapi.NewTokenManager(deps.DB, deps.CfgCache)
		if err := mgr.RevokeToken(ctx, tenantID, id); err != nil {
			c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "Token 不存在或已撤销"})
			return
		}
		RecordOperation(deps, c, "openapi", "revoke_token", "success", "token", &id, nil)
		middleware.Success(c, gin.H{"id": id, "status": openapi.TokenStatusRevoked})
	}
}

// ----- Webhook 端点管理 -----

// TenantListWebhookEndpoints GET /tenant/openapi/webhooks?status=&page=&page_size=
func TenantListWebhookEndpoints(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		tenantID := getTenantID(c)
		status := c.Query("status")
		page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
		pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

		mgr := openapi.NewWebhookManager(deps.DB, deps.CfgCache, deps.Crypto)
		items, total, err := mgr.ListEndpoints(ctx, tenantID, status, page, pageSize)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "查询失败: " + err.Error()})
			return
		}
		middleware.Success(c, gin.H{
			"items": items,
			"total": total,
			"page":  page,
		})
	}
}

// TenantCreateWebhookEndpoint POST /tenant/openapi/webhooks
func TenantCreateWebhookEndpoint(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		tenantID := getTenantID(c)
		var req struct {
			Name   string `json:"name" binding:"required"`
			URL    string `json:"url" binding:"required"`
			Secret string `json:"secret"`   // 可选，用于签名校验；明文传入，AES 加密存储
			Events string `json:"events" binding:"required"` // 逗号分隔，如 "order.paid,card.generated"
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "参数错误: " + err.Error()})
			return
		}
		ep := &model.WebhookEndpoint{
			TenantID: tenantID,
			Name:     req.Name,
			URL:      req.URL,
			Events:   req.Events,
		}
		mgr := openapi.NewWebhookManager(deps.DB, deps.CfgCache, deps.Crypto)
		if err := mgr.CreateEndpoint(ctx, ep, req.Secret); err != nil {
			code := 500
			if errors.Is(err, openapi.ErrInvalidURL) {
				code = 400
			}
			c.JSON(code, gin.H{"code": code, "message": err.Error()})
			return
		}
		uid := getUserID(c)
		RecordOperation(deps, c, "openapi", "create_webhook", "success", "webhook", &uid, map[string]interface{}{
			"endpoint_id": ep.ID, "url": ep.URL, "events": ep.Events,
		})
		middleware.Success(c, ep)
	}
}

// TenantGetWebhookEndpoint GET /tenant/openapi/webhooks/:id
func TenantGetWebhookEndpoint(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		tenantID := getTenantID(c)
		id, err := strconv.ParseUint(c.Param("id"), 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "无效的 ID"})
			return
		}
		mgr := openapi.NewWebhookManager(deps.DB, deps.CfgCache, deps.Crypto)
		ep, err := mgr.GetEndpoint(ctx, id, tenantID)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "Webhook 端点不存在"})
			return
		}
		middleware.Success(c, ep)
	}
}

// TenantUpdateWebhookEndpoint PUT /tenant/openapi/webhooks/:id
func TenantUpdateWebhookEndpoint(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		tenantID := getTenantID(c)
		id, err := strconv.ParseUint(c.Param("id"), 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "无效的 ID"})
			return
		}
		var req struct {
			Name       *string `json:"name"`
			URL        *string `json:"url"`
			Secret     *string `json:"secret"`  // 传入则更新；不传保持原值
			Events     *string `json:"events"`
			Status     *string `json:"status"` // active / disabled
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "参数错误: " + err.Error()})
			return
		}
		updates := map[string]interface{}{}
		if req.Name != nil {
			updates["name"] = *req.Name
		}
		if req.URL != nil {
			updates["url"] = *req.URL
		}
		if req.Events != nil {
			updates["events"] = *req.Events
		}
		if req.Status != nil {
			updates["status"] = *req.Status
		}
		newSecret := ""
		if req.Secret != nil {
			newSecret = *req.Secret
		}
		mgr := openapi.NewWebhookManager(deps.DB, deps.CfgCache, deps.Crypto)
		if err := mgr.UpdateEndpoint(ctx, id, tenantID, updates, newSecret); err != nil {
			code := 500
			if errors.Is(err, openapi.ErrInvalidURL) {
				code = 400
			} else if errors.Is(err, openapi.ErrEndpointNotFound) {
				code = 404
			}
			c.JSON(code, gin.H{"code": code, "message": err.Error()})
			return
		}
		RecordOperation(deps, c, "openapi", "update_webhook", "success", "webhook", &id, map[string]interface{}{
			"fields": len(updates),
		})
		middleware.Success(c, gin.H{"id": id})
	}
}

// TenantDeleteWebhookEndpoint DELETE /tenant/openapi/webhooks/:id
func TenantDeleteWebhookEndpoint(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		tenantID := getTenantID(c)
		id, err := strconv.ParseUint(c.Param("id"), 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "无效的 ID"})
			return
		}
		mgr := openapi.NewWebhookManager(deps.DB, deps.CfgCache, deps.Crypto)
		if err := mgr.DeleteEndpoint(ctx, id, tenantID); err != nil {
			c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "Webhook 端点不存在"})
			return
		}
		RecordOperation(deps, c, "openapi", "delete_webhook", "success", "webhook", &id, nil)
		middleware.Success(c, gin.H{"id": id})
	}
}

// ----- Webhook 推送日志（Delivery） -----

// TenantListWebhookDeliveries GET /tenant/openapi/webhooks/deliveries?endpoint_id=&status=&event_type=&page=&page_size=
func TenantListWebhookDeliveries(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		tenantID := getTenantID(c)
		endpointID, _ := strconv.ParseUint(c.Query("endpoint_id"), 10, 64)
		status := c.Query("status")
		eventType := c.Query("event_type")
		page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
		pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

		mgr := openapi.NewWebhookManager(deps.DB, deps.CfgCache, deps.Crypto)
		items, total, err := mgr.ListDeliveries(ctx, tenantID, endpointID, status, eventType, page, pageSize)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "查询失败: " + err.Error()})
			return
		}
		middleware.Success(c, gin.H{
			"items": items,
			"total": total,
			"page":  page,
		})
	}
}

// TenantGetWebhookDelivery GET /tenant/openapi/webhooks/deliveries/:id
func TenantGetWebhookDelivery(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		tenantID := getTenantID(c)
		id, err := strconv.ParseUint(c.Param("id"), 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "无效的 ID"})
			return
		}
		mgr := openapi.NewWebhookManager(deps.DB, deps.CfgCache, deps.Crypto)
		d, err := mgr.GetDelivery(ctx, id, tenantID)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "推送日志不存在"})
			return
		}
		middleware.Success(c, d)
	}
}

// TenantRetryWebhookDelivery POST /tenant/openapi/webhooks/deliveries/:id/retry
func TenantRetryWebhookDelivery(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		tenantID := getTenantID(c)
		id, err := strconv.ParseUint(c.Param("id"), 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "无效的 ID"})
			return
		}
		mgr := openapi.NewWebhookManager(deps.DB, deps.CfgCache, deps.Crypto)
		d, err := mgr.RetryDelivery(ctx, id, tenantID)
		if err != nil {
			code := 500
			if errors.Is(err, openapi.ErrDeliveryNotFound) {
				code = 404
			} else if errors.Is(err, openapi.ErrDeliveryNotRetryable) {
				code = 409
			} else if errors.Is(err, openapi.ErrEndpointDisabled) || errors.Is(err, openapi.ErrEndpointNotFound) {
				code = 409
			}
			c.JSON(code, gin.H{"code": code, "message": err.Error()})
			return
		}
		RecordOperation(deps, c, "openapi", "retry_delivery", "success", "delivery", &id, map[string]interface{}{
			"status": d.Status,
		})
		middleware.Success(c, d)
	}
}

// ----- 可用 scope / 事件类型查询（前端表单用） -----

// TenantOpenAPIMeta GET /tenant/openapi/meta
// 返回可用 scope 列表 + 支持的事件类型列表（供前端表单勾选）
func TenantOpenAPIMeta(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		availableScopes := deps.CfgCache.GetString(ctx, openapi.CfgKeyScopeAvailable, "")
		scopes := openapi.ParseScopes(availableScopes)

		eventTypes := []string{
			openapi.EventOrderPaid,
			openapi.EventCardGenerated,
			openapi.EventAgentRegistered,
			openapi.EventAgentRechargeApproved,
			openapi.EventAgentWithdrawPaid,
		}
		middleware.Success(c, gin.H{
			"scopes":      scopes,
			"event_types": eventTypes,
			"token_prefix":         deps.CfgCache.GetString(ctx, openapi.CfgKeyTokenPrefix, "pat_"),
			"token_max_per_tenant": deps.CfgCache.GetInt(ctx, openapi.CfgKeyTokenMaxPerTenant, 10),
			"token_default_ttl":    deps.CfgCache.GetInt(ctx, openapi.CfgKeyTokenDefaultTTLDays, 365),
		})
	}
}

// ============== 第三方调用方（openapiAuth - API Token 鉴权） ==============

// OpenAPIWhoami GET /api/v1/openapi/whoami
// 第三方通过 API Token 查询自身身份信息（用于调试 Token 是否生效）
func OpenAPIWhoami(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenID, _ := c.Get("api_token_id")
		tenantID, _ := c.Get("api_tenant_id")
		scopes, _ := c.Get("api_scopes")
		name, _ := c.Get("api_token_name")
		middleware.Success(c, gin.H{
			"token_id":  tokenID,
			"tenant_id": tenantID,
			"scopes":    scopes,
			"name":      name,
			"scope_list": openapi.ParseScopes(strings.TrimSpace(scopes.(string))),
		})
	}
}

// ============== Webhook 事件分发辅助（业务点接入） ==============

// DispatchWebhookEvent 异步分发 Webhook 事件到租户的所有订阅端点
// 设计要点（铁律 06）：
//   1. 异步执行（goroutine），不阻塞业务主流程；Webhook 推送失败不影响业务结果
//   2. panic recover，确保任何异常不会导致业务 goroutine 泄漏
//   3. 使用 context.Background()，因业务请求的 ctx 可能已结束
//   4. 任何错误仅记录日志，不返回（业务点已成功，Webhook 是 best-effort 通知）
//
// 使用示例：
//   DispatchWebhookEvent(deps, tenantID, openapi.EventOrderPaid, gin.H{"order_no": "ORD123", "amount": 99.5})
func DispatchWebhookEvent(deps *Deps, tenantID uint64, eventType string, payload interface{}) {
	if deps == nil || deps.DB == nil || deps.CfgCache == nil || tenantID == 0 || eventType == "" {
		return
	}
	go func() {
		defer func() {
			if r := recover(); r != nil {
				// 静默吞掉异常，不影响业务（铁律 06：Webhook 是 best-effort）
				_ = r
			}
		}()
		ctx := context.Background()
		mgr := openapi.NewWebhookManager(deps.DB, deps.CfgCache, deps.Crypto)
		_, _ = mgr.DispatchEvent(ctx, tenantID, eventType, payload)
	}()
}
