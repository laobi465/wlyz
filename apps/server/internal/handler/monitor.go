// v0.4.0 监控告警 Handler
// 严格遵循铁律 04/05/06：
//   04 - 采集间隔 / 阈值 / webhook URL / 静默期 / 告警开关 全部从 sys_config 读取
//   05 - 9 项 monitor.* 配置可通过后台「系统配置」实时调整
//   06 - 仅暴露查询/触发/确认接口；指标采集与阈值比较在 monitor 包内完成（显式 switch 不 eval）
package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/your-org/keyauth-saas/apps/server/internal/middleware"
	"github.com/your-org/keyauth-saas/apps/server/internal/model"
	"github.com/your-org/keyauth-saas/apps/server/internal/monitor"
)

// ============== 1. 监控状态概览 ==============

// AdminMonitorStatus GET /admin/monitor/status
// 返回监控配置 + 当前活跃告警 + 最近一次采集结果
func AdminMonitorStatus(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		mgr := monitor.NewManager(deps.DB, deps.CfgCache)

		// 配置概览
		configs := gin.H{
			"collect_interval":      mgr.GetCollectInterval(ctx),
			"alert_enabled":         mgr.IsAlertEnabled(ctx),
			"silence_minutes":       deps.CfgCache.GetInt(ctx, monitor.CfgKeySilenceMinutes, 30),
			"retention_days":        deps.CfgCache.GetInt(ctx, monitor.CfgKeyRetentionDays, 30),
			"webhook_configured":    deps.CfgCache.GetString(ctx, monitor.CfgKeyNotifyWebhookURL, "") != "",
			"threshold_cpu":         deps.CfgCache.GetFloat64(ctx, monitor.CfgKeyThresholdCPU, 90),
			"threshold_memory":      deps.CfgCache.GetFloat64(ctx, monitor.CfgKeyThresholdMemory, 90),
			"threshold_disk":        deps.CfgCache.GetFloat64(ctx, monitor.CfgKeyThresholdDisk, 85),
			"threshold_error_rate":  deps.CfgCache.GetFloat64(ctx, monitor.CfgKeyThresholdErrorRate, 10),
		}

		// 当前活跃告警
		activeAlerts, _ := mgr.GetActiveAlerts(ctx)

		// 统计
		var totalAlerts, firingCount, resolvedCount int64
		deps.DB.Model(&model.SystemAlert{}).Count(&totalAlerts)
		deps.DB.Model(&model.SystemAlert{}).Where("status = ?", monitor.StatusFiring).Count(&firingCount)
		deps.DB.Model(&model.SystemAlert{}).Where("status = ?", monitor.StatusResolved).Count(&resolvedCount)

		// 最近一次采集
		var latestMetric model.SystemMetric
		hasLatest := true
		if err := deps.DB.Order("collected_at DESC").First(&latestMetric).Error; err != nil {
			hasLatest = false
		}

		// 最近 24h 各指标平均值
		type metricAgg struct {
			MetricName string  `json:"metric_name"`
			AvgValue   float64 `json:"avg_value"`
			MaxValue   float64 `json:"max_value"`
			Count      int64   `json:"count"`
		}
		var aggs []metricAgg
		since := time.Now().Add(-24 * time.Hour)
		deps.DB.Model(&model.SystemMetric{}).
			Select("metric_name, AVG(metric_value) as avg_value, MAX(metric_value) as max_value, COUNT(*) as count").
			Where("collected_at >= ?", since).
			Group("metric_name").
			Scan(&aggs)

		resp := gin.H{
			"configs":         configs,
			"active_alerts":   activeAlerts,
			"stats": gin.H{
				"total_alerts":   totalAlerts,
				"firing_count":   firingCount,
				"resolved_count": resolvedCount,
			},
			"last_24h_summary": aggs,
			"latest_metric":    nil,
		}
		if hasLatest {
			resp["latest_metric"] = latestMetric
		}

		middleware.Success(c, resp)
	}
}

// ============== 2. 立即采集 ==============

// AdminCollectNow POST /admin/monitor/collect
// 管理员手动触发一次采集 + 评估
func AdminCollectNow(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		mgr := monitor.NewManager(deps.DB, deps.CfgCache)

		result, err := mgr.CollectAndEvaluate(ctx)
		if err != nil {
			middleware.Fail(c, http.StatusInternalServerError, 5001, "采集失败: "+err.Error())
			return
		}

		// 记录操作日志
		uid := getUserID(c)
		RecordOperation(deps, c, "monitor", "collect_now", "success", "system", &uid, map[string]interface{}{
			"metrics_collected": result.MetricsCollected,
			"alerts_fired":      result.AlertsFired,
			"alerts_resolved":   result.AlertsResolved,
		})

		middleware.Success(c, gin.H{
			"triggered":         true,
			"metrics_collected": result.MetricsCollected,
			"alerts_fired":      result.AlertsFired,
			"alerts_resolved":   result.AlertsResolved,
			"notify_sent":       result.NotifySent,
			"error":             result.ErrorMessage,
			"message":           "采集完成",
		})
	}
}

// ============== 3. 指标历史查询 ==============

// AdminMetricHistory GET /admin/monitor/metrics?name=&hours=&limit=
// 查询指定指标的历史数据
func AdminMetricHistory(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		name := c.Query("name")
		if name == "" {
			middleware.Fail(c, http.StatusBadRequest, 1001, "缺少 name 参数")
			return
		}

		hours := 24
		if h := c.Query("hours"); h != "" {
			if v, err := strconv.Atoi(h); err == nil && v > 0 && v <= 720 {
				hours = v
			}
		}
		limit := 100
		if l := c.Query("limit"); l != "" {
			if v, err := strconv.Atoi(l); err == nil && v > 0 && v <= 1000 {
				limit = v
			}
		}

		mgr := monitor.NewManager(deps.DB, deps.CfgCache)
		from := time.Now().Add(-time.Duration(hours) * time.Hour)
		to := time.Now()

		metrics, err := mgr.GetMetricHistory(ctx, name, from, to, limit)
		if err != nil {
			middleware.Fail(c, http.StatusInternalServerError, 5001, "查询失败: "+err.Error())
			return
		}

		middleware.Success(c, gin.H{
			"list":  metrics,
			"name":  name,
			"from":  from,
			"to":    to,
			"count": len(metrics),
		})
	}
}

// ============== 4. 告警列表 ==============

// AdminListAlerts GET /admin/monitor/alerts?status=&severity=&page=&page_size=
// 分页查询告警事件
func AdminListAlerts(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		page, pageSize := parsePagination(c)

		q := deps.DB.Model(&model.SystemAlert{})
		if status := c.Query("status"); status != "" {
			q = q.Where("status = ?", status)
		}
		if severity := c.Query("severity"); severity != "" {
			q = q.Where("severity = ?", severity)
		}

		var total int64
		q.Count(&total)

		var alerts []model.SystemAlert
		if err := q.Order("id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&alerts).Error; err != nil {
			middleware.Fail(c, http.StatusInternalServerError, 5001, "查询失败: "+err.Error())
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

// ============== 5. 确认告警 ==============

// adminAckAlertReq 确认告警请求
type adminAckAlertReq struct {
	AlertID uint64 `json:"alert_id" binding:"required"`
}

// AdminAckAlert POST /admin/monitor/alerts/ack
// 管理员确认告警（标记为 acked，停止通知）
func AdminAckAlert(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		var req adminAckAlertReq
		if err := c.ShouldBindJSON(&req); err != nil {
			middleware.Fail(c, http.StatusBadRequest, 1001, "参数错误: "+err.Error())
			return
		}

		// 校验告警存在
		var alert model.SystemAlert
		if err := deps.DB.First(&alert, req.AlertID).Error; err != nil {
			middleware.Fail(c, http.StatusNotFound, 1008, "未找到指定的告警")
			return
		}
		if alert.Status != monitor.StatusFiring {
			middleware.Fail(c, http.StatusBadRequest, 1001, "仅 firing 状态的告警可确认")
			return
		}

		adminID := getUserID(c)
		mgr := monitor.NewManager(deps.DB, deps.CfgCache)
		if err := mgr.AckAlert(ctx, req.AlertID, adminID); err != nil {
			middleware.Fail(c, http.StatusInternalServerError, 5001, "确认失败: "+err.Error())
			return
		}

		// 记录操作日志
		aid := req.AlertID
		uid := adminID
		RecordOperation(deps, c, "monitor", "ack_alert", "success", "system", &uid, map[string]interface{}{
			"alert_id": aid,
			"rule":     alert.AlertRule,
		})

		middleware.Success(c, gin.H{
			"acked":    true,
			"alert_id": req.AlertID,
			"message":  "告警已确认",
		})
	}
}

// ============== 6. 重发告警通知 ==============

// adminResendAlertReq 重发通知请求
type adminResendAlertReq struct {
	AlertID uint64 `json:"alert_id" binding:"required"`
}

// AdminResendAlert POST /admin/monitor/alerts/resend
// 手动重发告警通知到 webhook
func AdminResendAlert(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		var req adminResendAlertReq
		if err := c.ShouldBindJSON(&req); err != nil {
			middleware.Fail(c, http.StatusBadRequest, 1001, "参数错误: "+err.Error())
			return
		}

		mgr := monitor.NewManager(deps.DB, deps.CfgCache)
		sent, err := mgr.SendAlertNotification(ctx, req.AlertID)
		if err != nil {
			middleware.Fail(c, http.StatusNotFound, 1008, "告警不存在: "+err.Error())
			return
		}

		// 记录操作日志
		aid := req.AlertID
		uid := getUserID(c)
		RecordOperation(deps, c, "monitor", "resend_alert", "success", "system", &uid, map[string]interface{}{
			"alert_id":   aid,
			"sent":       sent,
		})

		middleware.Success(c, gin.H{
			"sent":      sent,
			"alert_id":  req.AlertID,
			"message":   "通知已发送",
		})
	}
}

// ============== 7. 清理过期指标 ==============

// AdminCleanupMetrics POST /admin/monitor/cleanup
// 手动触发清理过期指标
func AdminCleanupMetrics(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		mgr := monitor.NewManager(deps.DB, deps.CfgCache)

		deleted, err := mgr.CleanupExpiredMetrics(ctx)
		if err != nil {
			middleware.Fail(c, http.StatusInternalServerError, 5001, "清理失败: "+err.Error())
			return
		}

		// 记录操作日志
		uid := getUserID(c)
		RecordOperation(deps, c, "monitor", "cleanup_metrics", "success", "system", &uid, map[string]interface{}{
			"deleted_count": deleted,
		})

		middleware.Success(c, gin.H{
			"deleted_count": deleted,
			"message":       "清理完成",
		})
	}
}
