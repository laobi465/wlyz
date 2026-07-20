// v0.4.0 通知系统 Handler
// 严格遵循铁律 04/05/06：
//   04 - 三通道开关 / 服务商密钥 / SMTP / 重试策略 / 限流 全部从 sys_config 读取
//   05 - 16 项 notify.* 配置可通过后台「系统配置」实时调整
//   06 - 仅暴露模板 CRUD / 日志查询 / 手动重试 / 测试发送接口；变量替换在 notify 包内完成（strings.NewReplacer 防 SSTI）
package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/your-org/keyauth-saas/apps/server/internal/middleware"
	"github.com/your-org/keyauth-saas/apps/server/internal/model"
	"github.com/your-org/keyauth-saas/apps/server/internal/notify"
)

// ============== 1. 通知概览 ==============

// AdminNotifyStatus GET /admin/notify/status
// 返回三通道开关 + 配置概览 + 统计
func AdminNotifyStatus(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		mgr := notify.NewManager(deps.DB, deps.CfgCache, deps.Crypto)

		// 配置概览
		configs := gin.H{
			"sms_enabled":           deps.CfgCache.GetBool(ctx, notify.CfgKeySMSEnabled, false),
			"sms_provider":          deps.CfgCache.GetString(ctx, notify.CfgKeySMSProvider, "none"),
			"sms_sign_name":         deps.CfgCache.GetString(ctx, notify.CfgKeySMSSignName, ""),
			"sms_access_key_set":    deps.CfgCache.GetString(ctx, notify.CfgKeySMSAccessKeyID, "") != "",
			"sms_secret_set":        deps.CfgCache.GetString(ctx, notify.CfgKeySMSAccessSecretEnc, "") != "",
			"email_enabled":         deps.CfgCache.GetBool(ctx, notify.CfgKeyEmailEnabled, false),
			"email_smtp_host":       deps.CfgCache.GetString(ctx, notify.CfgKeyEmailSMTPHost, ""),
			"email_smtp_port":       deps.CfgCache.GetInt(ctx, notify.CfgKeyEmailSMTPPort, 465),
			"email_smtp_username":   deps.CfgCache.GetString(ctx, notify.CfgKeyEmailSMTPUsername, ""),
			"email_password_set":    deps.CfgCache.GetString(ctx, notify.CfgKeyEmailSMTPPasswordEnc, "") != "",
			"email_from_address":    deps.CfgCache.GetString(ctx, notify.CfgKeyEmailFromAddress, ""),
			"email_from_name":       deps.CfgCache.GetString(ctx, notify.CfgKeyEmailFromName, "KeyAuth SaaS"),
			"inapp_enabled":         deps.CfgCache.GetBool(ctx, notify.CfgKeyInAppEnabled, true),
			"retry_times":           deps.CfgCache.GetInt(ctx, notify.CfgKeyRetryTimes, 3),
			"retry_interval":        deps.CfgCache.GetInt(ctx, notify.CfgKeyRetryIntervalSeconds, 60),
			"rate_limit_per_minute": deps.CfgCache.GetInt(ctx, notify.CfgKeyRateLimitPerMinute, 60),
		}

		// 统计
		stats, _ := mgr.GetStats(ctx, 0)

		// 模板数
		var templateCount int64
		deps.DB.Model(&model.NotifyTemplate{}).Count(&templateCount)

		middleware.Success(c, gin.H{
			"configs":        configs,
			"stats":          stats,
			"template_count": templateCount,
		})
	}
}

// ============== 2. 模板 CRUD ==============

// AdminListNotifyTemplates GET /admin/notify/templates?channel=&page=&page_size=
func AdminListNotifyTemplates(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		mgr := notify.NewManager(deps.DB, deps.CfgCache, deps.Crypto)
		channel := c.Query("channel")
		page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
		pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
		items, total, err := mgr.ListTemplates(ctx, 0, channel, page, pageSize)
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

// AdminCreateNotifyTemplate POST /admin/notify/templates
func AdminCreateNotifyTemplate(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		var req struct {
			Code     string `json:"code" binding:"required"`
			Name     string `json:"name" binding:"required"`
			Channel  string `json:"channel" binding:"required"`
			Subject  string `json:"subject"`
			Content  string `json:"content" binding:"required"`
			Variables string `json:"variables"`
			Status   string `json:"status"`
			Remark   string `json:"remark"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "参数错误: " + err.Error()})
			return
		}
		if !notify.ValidateChannel(req.Channel) {
			c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "无效的渠道，必须为 sms/email/inapp"})
			return
		}
		if req.Variables == "" {
			req.Variables = "[]"
		}
		if req.Status == "" {
			req.Status = notify.TemplateStatusEnabled
		}
		tmpl := &model.NotifyTemplate{
			Code:      req.Code,
			Name:      req.Name,
			Channel:   req.Channel,
			Subject:   req.Subject,
			Content:   req.Content,
			Variables: req.Variables,
			TenantID:  0,
			Status:    req.Status,
			Remark:    req.Remark,
		}
		mgr := notify.NewManager(deps.DB, deps.CfgCache, deps.Crypto)
		if err := mgr.CreateTemplate(ctx, tmpl); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "创建失败: " + err.Error()})
			return
		}
		uid := getUserID(c)
		RecordOperation(deps, c, "notify", "create_template", "success", "template", &uid, map[string]interface{}{
			"template_id": tmpl.ID, "code": tmpl.Code, "channel": tmpl.Channel,
		})
		middleware.Success(c, tmpl)
	}
}

// AdminUpdateNotifyTemplate PUT /admin/notify/templates/:id
func AdminUpdateNotifyTemplate(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		id, err := strconv.ParseUint(c.Param("id"), 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "无效的 ID"})
			return
		}
		var req struct {
			Name      *string `json:"name"`
			Subject   *string `json:"subject"`
			Content   *string `json:"content"`
			Variables *string `json:"variables"`
			Status    *string `json:"status"`
			Remark    *string `json:"remark"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "参数错误: " + err.Error()})
			return
		}
		updates := map[string]interface{}{}
		if req.Name != nil {
			updates["name"] = *req.Name
		}
		if req.Subject != nil {
			updates["subject"] = *req.Subject
		}
		if req.Content != nil {
			updates["content"] = *req.Content
		}
		if req.Variables != nil {
			updates["variables"] = *req.Variables
		}
		if req.Status != nil {
			updates["status"] = *req.Status
		}
		if req.Remark != nil {
			updates["remark"] = *req.Remark
		}
		mgr := notify.NewManager(deps.DB, deps.CfgCache, deps.Crypto)
		if err := mgr.UpdateTemplate(ctx, id, updates); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "更新失败: " + err.Error()})
			return
		}
		RecordOperation(deps, c, "notify", "update_template", "success", "template", &id, map[string]interface{}{
			"fields": len(updates),
		})
		middleware.Success(c, gin.H{"id": id})
	}
}

// AdminDeleteNotifyTemplate DELETE /admin/notify/templates/:id
func AdminDeleteNotifyTemplate(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		id, err := strconv.ParseUint(c.Param("id"), 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "无效的 ID"})
			return
		}
		mgr := notify.NewManager(deps.DB, deps.CfgCache, deps.Crypto)
		if err := mgr.DeleteTemplate(ctx, id); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "删除失败: " + err.Error()})
			return
		}
		RecordOperation(deps, c, "notify", "delete_template", "success", "template", &id, nil)
		middleware.Success(c, gin.H{"id": id})
	}
}

// ============== 3. 日志查询 ==============

// AdminListNotifyLogs GET /admin/notify/logs?channel=&status=&page=&page_size=
func AdminListNotifyLogs(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		mgr := notify.NewManager(deps.DB, deps.CfgCache, deps.Crypto)
		channel := c.Query("channel")
		status := c.Query("status")
		page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
		pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
		items, total, err := mgr.ListLogs(ctx, 0, channel, status, page, pageSize)
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

// AdminGetNotifyLog GET /admin/notify/logs/:id
func AdminGetNotifyLog(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		id, err := strconv.ParseUint(c.Param("id"), 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "无效的 ID"})
			return
		}
		mgr := notify.NewManager(deps.DB, deps.CfgCache, deps.Crypto)
		log, err := mgr.GetLog(ctx, id)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "日志不存在"})
			return
		}
		middleware.Success(c, log)
	}
}

// ============== 4. 重试 / 测试发送 ==============

// AdminRetryNotifyLog POST /admin/notify/logs/:id/retry
func AdminRetryNotifyLog(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		id, err := strconv.ParseUint(c.Param("id"), 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "无效的 ID"})
			return
		}
		mgr := notify.NewManager(deps.DB, deps.CfgCache, deps.Crypto)
		result, err := mgr.Retry(ctx, id)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "重试失败: " + err.Error()})
			return
		}
		RecordOperation(deps, c, "notify", "retry_log", "success", "log", &id, map[string]interface{}{
			"status": result.Status,
		})
		middleware.Success(c, result)
	}
}

// AdminTestNotify POST /admin/notify/test
// 手动测试发送（不写日志表，直接调 provider）
func AdminTestNotify(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		var req struct {
			Channel   string                 `json:"channel" binding:"required"`
			Recipient string                 `json:"recipient" binding:"required"`
			Subject   string                 `json:"subject"`
			Content   string                 `json:"content" binding:"required"`
			Variables map[string]interface{} `json:"variables"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "参数错误: " + err.Error()})
			return
		}
		if !notify.ValidateChannel(req.Channel) {
			c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "无效的渠道"})
			return
		}
		mgr := notify.NewManager(deps.DB, deps.CfgCache, deps.Crypto)
		// 渲染变量
		subject := notify.Render(req.Subject, req.Variables)
		content := notify.Render(req.Content, req.Variables)

		// 直接调用 Send（会写日志 + 调 provider）
		result, err := mgr.Send(ctx, notify.SendRequest{
			TemplateCode: "test", // 测试用代码，无模板时 Send 会返回 ErrTemplateNotFound
			Channel:      req.Channel,
			Recipient:    req.Recipient,
			Variables:    req.Variables,
			TenantID:     0,
			Priority:     notify.PriorityHigh,
		})
		// 测试模式：忽略模板未找到错误，直接 dispatch
		if err == notify.ErrTemplateNotFound {
			// 直接构造日志 + dispatch
			logEntry := &model.NotifyLog{
				TemplateCode: "test",
				Channel:      req.Channel,
				Recipient:    req.Recipient,
				Subject:      subject,
				Content:      content,
				Status:       notify.LogStatusPending,
				Priority:     notify.PriorityHigh,
				TenantID:     0,
			}
			deps.DB.Create(logEntry)
			result = mgr.TestDispatch(ctx, req.Channel, req.Recipient, subject, content, req.Variables)
			now := time.Now()
			updates := map[string]interface{}{
				"status":          result.Status,
				"provider_msg_id": result.ProviderMsgID,
				"error_message":   result.ErrorMessage,
			}
			if result.Status == notify.LogStatusSent {
				updates["sent_at"] = &now
			}
			deps.DB.Model(&model.NotifyLog{}).Where("id = ?", logEntry.ID).Updates(updates)
			result.LogID = logEntry.ID
		} else if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "发送失败: " + err.Error(), "result": result})
			return
		}
		uid := getUserID(c)
		RecordOperation(deps, c, "notify", "test_send", "success", "log", &uid, map[string]interface{}{
			"channel": req.Channel, "recipient": req.Recipient, "status": result.Status,
		})
		middleware.Success(c, result)
	}
}
