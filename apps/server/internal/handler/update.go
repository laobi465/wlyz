// v0.4.0 在线更新 Handler
// 包含 Webhook 接收 + 管理后台状态/触发/历史/回滚
// 严格遵循铁律 04/05/06：
//
//	04 - webhook 密钥 / 分支 / 自动开关 / 部署脚本路径 全部从 sys_config 读取
//	05 - 8 项 update.* 配置可通过后台「系统配置」实时调整
//	06 - webhook 签名校验用 hmac.Equal 防时序攻击；shell 命令显式组合不拼接用户输入
package handler

import (
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/your-org/keyauth-saas/apps/server/internal/logger"
	"github.com/your-org/keyauth-saas/apps/server/internal/middleware"
	"github.com/your-org/keyauth-saas/apps/server/internal/model"
	"github.com/your-org/keyauth-saas/apps/server/internal/update"
)

// ============== 1. Webhook 接收（无鉴权，靠签名校验） ==============

// GitHubWebhook POST /api/v1/public/update/webhook
// 接收 GitHub push event，校验签名 + 分支匹配后触发更新
func GitHubWebhook(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		// 1. 读取 raw body（签名校验需要原始字节）
		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			middleware.Fail(c, http.StatusBadRequest, 1001, "读取请求体失败")
			return
		}

		// 2. 签名校验（铁律 06：HMAC-SHA256 + hmac.Equal 防时序攻击）
		signature := c.GetHeader("X-Hub-Signature-256")
		secret := deps.CfgCache.GetString(ctx, update.CfgKeyWebhookSecret, "")
		if !update.VerifyWebhookSignature(signature, body, secret) {
			middleware.Fail(c, http.StatusUnauthorized, 1002, "Webhook 签名校验失败")
			return
		}

		// 3. 事件类型校验（仅处理 push）
		eventType := c.GetHeader("X-GitHub-Event")
		if eventType != "push" {
			middleware.Success(c, gin.H{"skipped": true, "reason": "non-push event: " + eventType})
			return
		}

		// 4. 解析 push event
		event, err := update.ParsePushEvent(body)
		if err != nil {
			middleware.Fail(c, http.StatusBadRequest, 1001, "解析 push event 失败: "+err.Error())
			return
		}

		// 5. 分支匹配
		targetBranch := deps.CfgCache.GetString(ctx, update.CfgKeyWebhookBranch, "main")
		if !update.BranchMatches(event.Ref, targetBranch) {
			middleware.Success(c, gin.H{
				"skipped":     true,
				"reason":      "branch mismatch",
				"event_ref":   event.Ref,
				"target":      targetBranch,
				"head_commit": event.HeadCommit.ID,
			})
			return
		}

		// 6. 记录 webhook 通知（无论是否自动触发都写审计日志的 pending）
		// 7. 是否自动触发更新
		autoUpdate := deps.CfgCache.GetBool(ctx, update.CfgKeyAutoUpdate, false)
		if !autoUpdate {
			// 仅记录通知，等待管理员手动触发
			middleware.Success(c, gin.H{
				"received":       true,
				"auto_update":    false,
				"head_commit":    event.HeadCommit.ID,
				"commit_message": event.HeadCommit.Message,
				"sender":         event.Sender.Login,
				"branch":         event.Ref,
				"message":        "已收到推送通知，需管理员手动触发更新",
			})
			return
		}

		// 8. 异步触发更新（不阻塞 webhook 响应）
		mgr := update.NewManager(deps.DB, deps.CfgCache)
		go func() {
			opts := update.UpdateOptions{
				TriggerSource: update.TriggerSourceWebhook,
				TriggerBy:     0,
				TriggerIP:     c.ClientIP(),
				Branch:        targetBranch,
			}
			_, _ = mgr.ExecuteUpdate(ctx, opts)
		}()

		middleware.Success(c, gin.H{
			"received":       true,
			"auto_update":    true,
			"head_commit":    event.HeadCommit.ID,
			"commit_message": event.HeadCommit.Message,
			"sender":         event.Sender.Login,
			"branch":         event.Ref,
			"message":        "已触发自动更新",
		})
	}
}

// ============== 2. 管理后台：更新状态 ==============

// AdminUpdateStatus GET /admin/update/status
// 返回当前部署版本 / 锁状态 / 自动开关 / 最新审计日志
func AdminUpdateStatus(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		mgr := update.NewManager(deps.DB, deps.CfgCache)

		// 当前 commit hash
		currentCommit := mgr.GetLatestCommit(ctx)

		// 锁状态
		isLocked := mgr.IsLocked(ctx)

		// 自动更新开关
		autoUpdate := deps.CfgCache.GetBool(ctx, update.CfgKeyAutoUpdate, false)

		// 目标分支
		branch := deps.CfgCache.GetString(ctx, update.CfgKeyWebhookBranch, "main")

		// 最近一次审计日志
		var latestLog model.SystemUpdateLog
		hasLatest := true
		if err := deps.DB.Order("id DESC").First(&latestLog).Error; err != nil {
			hasLatest = false
		}

		// 最近 5 次成功 / 失败统计
		var successCount, failedCount int64
		deps.DB.Model(&model.SystemUpdateLog{}).Where("status = ?", update.StatusSuccess).Count(&successCount)
		deps.DB.Model(&model.SystemUpdateLog{}).Where("status IN ?", []string{update.StatusFailed, update.StatusRolledBack}).Count(&failedCount)

		resp := gin.H{
			"current_commit": currentCommit,
			"is_locked":      isLocked,
			"auto_update":    autoUpdate,
			"branch":         branch,
			"success_count":  successCount,
			"failed_count":   failedCount,
			"latest_log":     nil,
		}
		if hasLatest {
			resp["latest_log"] = latestLog
		}

		middleware.Success(c, resp)
	}
}

// ============== 3. 管理后台：手动触发更新 ==============

// adminTriggerUpdateReq 手动触发更新请求
type adminTriggerUpdateReq struct {
	Branch string `json:"branch" binding:"omitempty,max=64"` // 可选，空则用 sys_config 默认分支
}

// AdminTriggerUpdate POST /admin/update/trigger
// 管理员手动触发更新（异步执行，立即返回 log_id）
func AdminTriggerUpdate(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		var req adminTriggerUpdateReq
		_ = c.ShouldBindJSON(&req)

		branch := req.Branch
		if branch == "" {
			branch = deps.CfgCache.GetString(ctx, update.CfgKeyWebhookBranch, "main")
		}

		adminID := getUserID(c)
		mgr := update.NewManager(deps.DB, deps.CfgCache)

		// 快速检查锁状态（避免无意义的异步等待）
		if mgr.IsLocked(ctx) {
			middleware.Fail(c, http.StatusConflict, 1011, "已有更新在进行中，请等待完成")
			return
		}

		// 异步触发（更新可能耗时较长，不阻塞 HTTP 响应）
		opts := update.UpdateOptions{
			TriggerSource: update.TriggerSourceManual,
			TriggerBy:     adminID,
			TriggerIP:     c.ClientIP(),
			Branch:        branch,
		}

		// 先创建一条 pending 审计日志（让前端能立即看到状态）
		// 实际执行由 ExecuteUpdate 内部完成（会更新该日志）
		go func() {
			_, _ = mgr.ExecuteUpdate(ctx, opts)
		}()

		// 记录操作日志
		RecordOperation(deps, c, "update", "trigger_update", "success", "system", nil, map[string]interface{}{
			"branch":     branch,
			"trigger_by": adminID,
		})

		middleware.Success(c, gin.H{
			"triggered": true,
			"branch":    branch,
			"message":   "更新已异步触发，请通过 /admin/update/status 或 /admin/update/history 查看进度",
		})
	}
}

// ============== 4. 管理后台：更新历史 ==============

// AdminListUpdateHistory GET /admin/update/history?page=&page_size=&status=
// 分页查询更新审计日志
func AdminListUpdateHistory(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		page, pageSize := parsePagination(c)

		q := deps.DB.Model(&model.SystemUpdateLog{})
		if status := c.Query("status"); status != "" {
			q = q.Where("status = ?", status)
		}
		if source := c.Query("trigger_source"); source != "" {
			q = q.Where("trigger_source = ?", source)
		}

		var total int64
		q.Count(&total)

		var logs []model.SystemUpdateLog
		if err := q.Order("id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&logs).Error; err != nil {
			logger.Error("update: list logs failed", "err", err)
			middleware.Fail(c, http.StatusInternalServerError, 5001, "查询失败")
			return
		}

		middleware.Success(c, gin.H{
			"list":      logs,
			"total":     total,
			"page":      page,
			"page_size": pageSize,
		})
	}
}

// ============== 5. 管理后台：回滚 ==============

// adminRollbackReq 回滚请求
type adminRollbackReq struct {
	FailedLogID uint64 `json:"failed_log_id" binding:"required"` // 要回滚的失败更新日志 id
}

// AdminRollbackUpdate POST /admin/update/rollback
// 管理员手动回滚到指定失败更新前的 commit
func AdminRollbackUpdate(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		var req adminRollbackReq
		if err := c.ShouldBindJSON(&req); err != nil {
			middleware.Fail(c, http.StatusBadRequest, 1001, "参数错误: "+err.Error())
			return
		}

		// 校验目标日志存在
		var failedLog model.SystemUpdateLog
		if err := deps.DB.First(&failedLog, req.FailedLogID).Error; err != nil {
			middleware.Fail(c, http.StatusNotFound, 1008, "未找到指定的更新日志")
			return
		}

		if failedLog.CommitBefore == "" {
			middleware.Fail(c, http.StatusBadRequest, 1001, "目标日志未记录 commit_before，无法回滚")
			return
		}

		adminID := getUserID(c)
		mgr := update.NewManager(deps.DB, deps.CfgCache)

		// 锁状态检查
		if mgr.IsLocked(ctx) {
			middleware.Fail(c, http.StatusConflict, 1011, "已有更新在进行中，请等待完成")
			return
		}

		// 异步执行回滚
		opts := update.UpdateOptions{
			TriggerSource: update.TriggerSourceRollback,
			TriggerBy:     adminID,
			TriggerIP:     c.ClientIP(),
		}
		go func() {
			_, _ = mgr.Rollback(ctx, req.FailedLogID, opts)
		}()

		// 记录操作日志
		fid := req.FailedLogID
		RecordOperation(deps, c, "update", "rollback_update", "success", "system", &fid, map[string]interface{}{
			"failed_log_id": req.FailedLogID,
			"target_commit": failedLog.CommitBefore,
			"trigger_by":    adminID,
		})

		middleware.Success(c, gin.H{
			"triggered":     true,
			"failed_log_id": req.FailedLogID,
			"target_commit": failedLog.CommitBefore,
			"message":       "回滚已异步触发，请通过 /admin/update/history 查看进度",
		})
	}
}

// ============== 6. 管理后台：单条详情 ==============

// AdminGetUpdateLog GET /admin/update/logs/:id
// 查询指定更新日志详情（含完整 log_text）
func AdminGetUpdateLog(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseUint(c.Param("id"), 10, 64)
		if err != nil {
			middleware.Fail(c, http.StatusBadRequest, 1001, "ID 格式错误")
			return
		}

		var log model.SystemUpdateLog
		if err := deps.DB.First(&log, id).Error; err != nil {
			middleware.Fail(c, http.StatusNotFound, 1008, "未找到指定的更新日志")
			return
		}

		middleware.Success(c, log)
	}
}

// ============== 7. 管理后台：轻量轮询（v0.4.0 弹窗通知） ==============

// AdminUpdatePoll GET /admin/update/poll
// v0.4.0 管理员弹窗通知专用轻量轮询端点
// 仅返回 commit + 锁状态 + 最近一次更新元信息（不含 log_text 重字段）
// 前端 AdminLayout 中的 UpdateNotifier 组件按 update.poll.interval_seconds 间隔轮询
// 检测到 current_commit 与 localStorage 中记录的 last_known_commit 不一致时弹窗提示
// 严格遵循铁律 04/05：轮询开关 / 间隔 全部从 sys_config 读取
func AdminUpdatePoll(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		// 弹窗通知总开关（关闭时前端不轮询，但接口仍可调用便于排查）
		enabled := deps.CfgCache.GetBool(ctx, update.CfgKeyPollEnabled, true)

		// 建议轮询间隔（秒），强制下限 PollIntervalMin 防配置错误打爆后端
		intervalSec := deps.CfgCache.GetInt(ctx, update.CfgKeyPollInterval, 30)
		if intervalSec < update.PollIntervalMin {
			intervalSec = update.PollIntervalMin
		}

		mgr := update.NewManager(deps.DB, deps.CfgCache)
		currentCommit := mgr.GetLatestCommit(ctx)
		isLocked := mgr.IsLocked(ctx)

		// 仅 SELECT 轻量字段（避免 log_text 大字段）
		var latest struct {
			Status        string    `gorm:"column:status"`
			TriggerSource string    `gorm:"column:trigger_source"`
			CommitAfter   string    `gorm:"column:commit_after"`
			CreatedAt     time.Time `gorm:"column:created_at"`
		}
		hasLatest := true
		if err := deps.DB.Table("system_update_log").
			Order("id DESC").
			Limit(1).
			Select("status, trigger_source, commit_after, created_at").
			Scan(&latest).Error; err != nil || latest.Status == "" {
			hasLatest = false
		}

		resp := gin.H{
			"enabled":          enabled,
			"interval_seconds": intervalSec,
			"current_commit":   currentCommit,
			"is_locked":        isLocked,
			"last_update_at":   nil,
			"last_status":      nil,
			"last_trigger":     nil,
			"last_commit":      nil,
		}
		if hasLatest {
			resp["last_update_at"] = latest.CreatedAt.Unix()
			resp["last_status"] = latest.Status
			resp["last_trigger"] = latest.TriggerSource
			resp["last_commit"] = latest.CommitAfter
		}

		middleware.Success(c, resp)
	}
}

// ============== 8. 管理后台：检查 GitHub 是否有新版本（v0.9.0 新增） ==============

// AdminCheckUpdate GET /admin/update/check
// v0.9.0 主动调用 GitHub API 查询最新 release，对比当前部署版本
//
// 与 /admin/update/poll 的区别：
//   - poll: 对比本地 current_commit 变化（更新已部署后弹窗提示管理员刷新）
//   - check: 对比 GitHub 远端 release 与本地配置的 current_version（部署前主动检查）
//
// 严格遵循铁律：
//
//	04 - GitHub owner/repo/token/current_version 全部从 sys_config 读取
//	05 - 4 项 update.github.* 配置可通过后台「系统配置」实时调整
//	06 - 网络错误显式返回错误信息，不编造数据；token 不回显
//
// 缓存：进程内缓存 60 秒（GitHubAPILimitMinInterval），防止管理员频繁点击触发 GitHub 限流
func AdminCheckUpdate(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		mgr := update.NewManager(deps.DB, deps.CfgCache)

		result, err := mgr.CheckUpdate(ctx)
		if err != nil {
			// 错误已包含可读信息（配置缺失 / 网络错误 / 限流 / 仓库无 release 等）
			logger.Warn("update: check github release failed", "err", err)
			middleware.Fail(c, http.StatusBadGateway, 5002, err.Error())
			return
		}

		// 记录操作日志（检查更新动作）
		adminID := getUserID(c)
		RecordOperation(deps, c, "update", "check_update", "success", "system", nil, map[string]interface{}{
			"trigger_by": adminID,
			"current":    result.CurrentVersion,
			"latest":     result.LatestVersion,
			"has_update": result.HasUpdate,
			"repo":       result.RepoOwner + "/" + result.RepoName,
		})

		middleware.Success(c, result)
	}
}
