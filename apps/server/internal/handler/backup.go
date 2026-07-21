// v0.4.0 数据备份恢复 Handler
// 严格遵循铁律 04/05/06：
//
//	04 - 备份目录 / 保留天数 / 加密密钥 / 压缩开关 全部从 sys_config 读取
//	05 - 6 项 backup.* 配置可通过后台实时调整
//	06 - 下载前 checksum 校验；恢复前严格校验文件完整性
package handler

import (
	"net/http"
	"os"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/your-org/keyauth-saas/apps/server/internal/backup"
	"github.com/your-org/keyauth-saas/apps/server/internal/logger"
	"github.com/your-org/keyauth-saas/apps/server/internal/middleware"
	"github.com/your-org/keyauth-saas/apps/server/internal/model"
)

// ============== 1. 创建备份 ==============

// adminCreateBackupReq 创建备份请求
type adminCreateBackupReq struct {
	BackupType string `json:"backup_type" binding:"omitempty,oneof=manual auto"` // 默认 manual
}

// AdminCreateBackup POST /admin/backup/create
// 管理员手动创建备份（异步执行，立即返回 log_id）
func AdminCreateBackup(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		var req adminCreateBackupReq
		_ = c.ShouldBindJSON(&req)
		if req.BackupType == "" {
			req.BackupType = backup.BackupTypeManual
		}

		adminID := getUserID(c)
		mgr := backup.NewManager(deps.DB, deps.CfgCache)

		// 异步执行（备份可能耗时较长）
		go func() {
			opts := backup.BackupOptions{
				BackupType: req.BackupType,
				TriggerBy:  adminID,
				TriggerIP:  c.ClientIP(),
			}
			_, _ = mgr.CreateBackup(ctx, opts)
		}()

		// 记录操作日志
		uid := adminID
		RecordOperation(deps, c, "backup", "create_backup", "success", "system", &uid, map[string]interface{}{
			"backup_type": req.BackupType,
		})

		middleware.Success(c, gin.H{
			"triggered":   true,
			"backup_type": req.BackupType,
			"message":     "备份已异步触发，请通过 /admin/backup/list 查看进度",
		})
	}
}

// ============== 2. 备份列表 ==============

// AdminListBackups GET /admin/backup/list?page=&page_size=&status=&backup_type=
// 分页查询备份审计日志
func AdminListBackups(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		page, pageSize := parsePagination(c)

		q := deps.DB.Model(&model.SystemBackupLog{})
		if status := c.Query("status"); status != "" {
			q = q.Where("status = ?", status)
		}
		if bt := c.Query("backup_type"); bt != "" {
			q = q.Where("backup_type = ?", bt)
		}

		var total int64
		q.Count(&total)

		var logs []model.SystemBackupLog
		if err := q.Order("id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&logs).Error; err != nil {
			logger.Error("backup: list logs failed", "err", err)
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

// ============== 3. 备份详情 ==============

// AdminGetBackup GET /admin/backup/:id
// 查询单条备份日志详情
func AdminGetBackup(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseUint(c.Param("id"), 10, 64)
		if err != nil {
			middleware.Fail(c, http.StatusBadRequest, 1001, "ID 格式错误")
			return
		}

		var log model.SystemBackupLog
		if err := deps.DB.First(&log, id).Error; err != nil {
			middleware.Fail(c, http.StatusNotFound, 1008, "未找到指定的备份日志")
			return
		}

		middleware.Success(c, log)
	}
}

// ============== 4. 下载备份 ==============

// AdminDownloadBackup GET /admin/backup/:id/download
// 下载备份文件（强制校验 checksum）
func AdminDownloadBackup(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		id, err := strconv.ParseUint(c.Param("id"), 10, 64)
		if err != nil {
			middleware.Fail(c, http.StatusBadRequest, 1001, "ID 格式错误")
			return
		}

		mgr := backup.NewManager(deps.DB, deps.CfgCache)

		// 校验 checksum
		ok, err := mgr.VerifyChecksum(ctx, id)
		if err != nil {
			middleware.Fail(c, http.StatusNotFound, 1008, "校验失败: "+err.Error())
			return
		}
		if !ok {
			middleware.Fail(c, http.StatusConflict, 1011, "备份文件 checksum 不匹配，拒绝下载")
			return
		}

		// 获取文件路径
		filePath, err := mgr.GetBackupFilePath(ctx, id)
		if err != nil {
			middleware.Fail(c, http.StatusNotFound, 1008, "获取文件路径失败: "+err.Error())
			return
		}

		// 检查文件存在
		if _, err := os.Stat(filePath); err != nil {
			middleware.Fail(c, http.StatusNotFound, 1008, "备份文件不存在: "+err.Error())
			return
		}

		// 记录操作日志
		fid := id
		RecordOperation(deps, c, "backup", "download_backup", "success", "system", &fid, nil)

		c.Header("Content-Description", "File Transfer")
		c.Header("Content-Transfer-Encoding", "binary")
		c.Header("Content-Disposition", "attachment; filename=\"backup_"+strconv.FormatUint(id, 10)+".bak\"")
		c.Header("Content-Type", "application/octet-stream")
		c.File(filePath)
	}
}

// ============== 5. 恢复备份 ==============

// adminRestoreBackupReq 恢复备份请求
type adminRestoreBackupReq struct {
	BackupLogID uint64 `json:"backup_log_id" binding:"required"`
}

// AdminRestoreBackup POST /admin/backup/restore
// 管理员手动恢复指定备份（异步执行）
func AdminRestoreBackup(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		var req adminRestoreBackupReq
		if err := c.ShouldBindJSON(&req); err != nil {
			middleware.Fail(c, http.StatusBadRequest, 1001, "参数错误: "+err.Error())
			return
		}

		// 校验目标备份存在
		var backupLog model.SystemBackupLog
		if err := deps.DB.First(&backupLog, req.BackupLogID).Error; err != nil {
			middleware.Fail(c, http.StatusNotFound, 1008, "未找到指定的备份日志")
			return
		}
		if backupLog.Status != backup.StatusSuccess {
			middleware.Fail(c, http.StatusBadRequest, 1001, "目标备份状态非 success，无法恢复")
			return
		}

		adminID := getUserID(c)
		mgr := backup.NewManager(deps.DB, deps.CfgCache)

		// 异步执行（恢复可能耗时较长）
		go func() {
			opts := backup.RestoreOptions{
				BackupLogID: req.BackupLogID,
				TriggerBy:   adminID,
				TriggerIP:   c.ClientIP(),
			}
			_, _ = mgr.RestoreBackup(ctx, opts)
		}()

		// 记录操作日志
		bid := req.BackupLogID
		uid := adminID
		RecordOperation(deps, c, "backup", "restore_backup", "success", "system", &uid, map[string]interface{}{
			"backup_log_id": bid,
			"trigger_by":    adminID,
		})

		middleware.Success(c, gin.H{
			"triggered":     true,
			"backup_log_id": req.BackupLogID,
			"message":       "恢复已异步触发，请通过 /admin/backup/list 查看进度（restored_from 字段关联原备份）",
		})
	}
}

// ============== 6. 清理过期备份 ==============

// AdminCleanupBackups POST /admin/backup/cleanup
// 手动触发清理过期备份
func AdminCleanupBackups(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		mgr := backup.NewManager(deps.DB, deps.CfgCache)

		deleted, err := mgr.CleanupExpired(ctx)
		if err != nil {
			logger.Error("backup: cleanup expired failed", "err", err)
			middleware.Fail(c, http.StatusInternalServerError, 5001, "清理失败")
			return
		}

		// 记录操作日志
		uid := getUserID(c)
		RecordOperation(deps, c, "backup", "cleanup_expired", "success", "system", &uid, map[string]interface{}{
			"deleted_count": deleted,
		})

		middleware.Success(c, gin.H{
			"deleted_count": deleted,
			"message":       "清理完成",
		})
	}
}

// ============== 7. 备份状态概览 ==============

// AdminBackupStatus GET /admin/backup/status
// 返回备份配置 + 统计信息
func AdminBackupStatus(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		mgr := backup.NewManager(deps.DB, deps.CfgCache)

		// 统计
		var totalBackups, successCount, failedCount int64
		deps.DB.Model(&model.SystemBackupLog{}).Where("backup_type != ?", backup.BackupTypeRestoreSource).Count(&totalBackups)
		deps.DB.Model(&model.SystemBackupLog{}).Where("backup_type != ? AND status = ?", backup.BackupTypeRestoreSource, backup.StatusSuccess).Count(&successCount)
		deps.DB.Model(&model.SystemBackupLog{}).Where("backup_type != ? AND status = ?", backup.BackupTypeRestoreSource, backup.StatusFailed).Count(&failedCount)

		// 最近一次成功备份
		var latestSuccess model.SystemBackupLog
		hasLatest := true
		if err := deps.DB.Where("backup_type != ? AND status = ?", backup.BackupTypeRestoreSource, backup.StatusSuccess).
			Order("id DESC").First(&latestSuccess).Error; err != nil {
			hasLatest = false
		}

		// 备份目录磁盘占用
		var totalSize int64
		deps.DB.Model(&model.SystemBackupLog{}).
			Where("status = ?", backup.StatusSuccess).
			Select("COALESCE(SUM(file_size), 0)").Scan(&totalSize)

		resp := gin.H{
			"auto_enabled":   mgr.IsAutoBackupEnabled(ctx),
			"backup_dir":     deps.CfgCache.GetString(ctx, backup.CfgKeyBackupDir, "data/backups"),
			"retention_days": deps.CfgCache.GetInt(ctx, backup.CfgKeyRetentionDays, 30),
			"compress":       deps.CfgCache.GetBool(ctx, backup.CfgKeyCompress, true),
			"encrypted":      deps.CfgCache.GetString(ctx, backup.CfgKeyEncryptionKey, "") != "",
			"total_backups":  totalBackups,
			"success_count":  successCount,
			"failed_count":   failedCount,
			"total_size":     totalSize,
			"latest_success": nil,
		}
		if hasLatest {
			resp["latest_success"] = latestSuccess
		}

		middleware.Success(c, resp)
	}
}
