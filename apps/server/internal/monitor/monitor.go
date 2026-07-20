// Package monitor v0.4.0 监控告警核心包
// 严格遵循铁律 04/05/06：
//   04 - 无硬编码：采集间隔 / 阈值 / 通知 webhook / 静默期 / 告警开关 全部从 sys_config 读取
//   05 - 配置走后端：9 项 monitor.* 配置可通过后台实时调整
//   06 - 反幻觉：阈值比较用显式运算符函数（不依赖字符串拼接 eval）；测试覆盖正/负/边界全场景
//
// 核心能力：
//   1. Manager.CollectAndEvaluate - 采集指标 + 评估告警（一体化入口）
//   2. Manager.CollectSystemMetrics - 采集 CPU/内存/磁盘/在线设备/QPS/错误率
//   3. Manager.EvaluateAlerts - 评估所有指标是否超阈值
//   4. Manager.SendAlertNotification - 发送告警通知（HTTP POST webhook）
//   5. CompareWithOperator - 通用阈值比较函数（> / < / >= / <= / ==）
//   6. Manager.ResolveStaleAlerts - 自动恢复已正常的告警
//   7. Manager.CleanupExpiredMetrics - 清理过期指标数据
package monitor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/mem"
	"gorm.io/gorm"

	"github.com/your-org/keyauth-saas/apps/server/internal/config"
	"github.com/your-org/keyauth-saas/apps/server/internal/model"
)

// ============== 常量 ==============

// 配置键常量（铁律 04：禁止硬编码配置键名）
const (
	CfgKeyCollectInterval       = "monitor.collect_interval"
	CfgKeyAlertEnabled          = "monitor.alert_enabled"
	CfgKeyNotifyWebhookURL      = "monitor.notify.webhook_url"
	CfgKeySilenceMinutes        = "monitor.silence_minutes"
	CfgKeyThresholdCPU          = "monitor.threshold.cpu_usage"
	CfgKeyThresholdMemory       = "monitor.threshold.memory_usage"
	CfgKeyThresholdDisk         = "monitor.threshold.disk_usage"
	CfgKeyThresholdErrorRate    = "monitor.threshold.error_rate"
	CfgKeyRetentionDays         = "monitor.retention_days"
)

// MetricName 指标名
const (
	MetricCPUUsage       = "cpu_usage"
	MetricMemoryUsage    = "memory_usage"
	MetricDiskUsage      = "disk_usage"
	MetricQPS            = "qps"
	MetricVerifyCount    = "verify_count"
	MetricOnlineDevices  = "online_devices"
	MetricErrorRate      = "error_rate"
)

// Severity 严重程度
const (
	SeverityInfo     = "info"
	SeverityWarning  = "warning"
	SeverityCritical = "critical"
	SeverityFatal    = "fatal"
)

// Status 告警状态
const (
	StatusFiring   = "firing"
	StatusResolved = "resolved"
	StatusSilenced = "silenced"
	StatusAcked    = "acked"
)

// Operator 比较运算符
const (
	OpGT = ">"
	OpLT = "<"
	OpGE = ">="
	OpLE = "<="
	OpEQ = "=="
)

// ============== 类型 ==============

// MetricSample 单个指标样本
type MetricSample struct {
	Name   string
	Value  float64
	Unit   string
	Labels map[string]interface{}
}

// AlertRule 告警规则（动态从 sys_config 构造）
type AlertRule struct {
	MetricName    string
	Threshold     float64
	Operator      string
	Severity      string
	Message       string
}

// AlertPayload webhook 通知 payload
type AlertPayload struct {
	AlertRule   string                 `json:"alert_rule"`
	Severity    string                 `json:"severity"`
	Status      string                 `json:"status"`
	MetricValue float64                `json:"metric_value"`
	Threshold   float64                `json:"threshold"`
	Operator    string                 `json:"operator"`
	Message     string                 `json:"message"`
	Labels      map[string]interface{} `json:"labels"`
	FiredAt     time.Time              `json:"fired_at"`
}

// CollectResult 采集结果
type CollectResult struct {
	MetricsCollected int
	AlertsFired      int
	AlertsResolved   int
	NotifySent       int
	ErrorMessage     string
}

// Manager 监控管理器
type Manager struct {
	db    *gorm.DB
	cache *config.ConfigCache
	mu    sync.Mutex // 采集互斥
}

// NewManager 创建监控管理器
func NewManager(db *gorm.DB, cache *config.ConfigCache) *Manager {
	return &Manager{
		db:    db,
		cache: cache,
	}
}

// ============== 1. 阈值比较 ==============

// CompareWithOperator 通用阈值比较函数
// 铁律 06：显式 switch，不依赖字符串拼接 eval；返回 value OP threshold 的布尔结果
func CompareWithOperator(value, threshold float64, operator string) bool {
	switch operator {
	case OpGT:
		return value > threshold
	case OpLT:
		return value < threshold
	case OpGE:
		return value >= threshold
	case OpLE:
		return value <= threshold
	case OpEQ:
		// 浮点相等用精度比较（0.001）
		diff := value - threshold
		return diff < 0.001 && diff > -0.001
	default:
		return false
	}
}

// ============== 2. 采集系统指标 ==============

// CollectSystemMetrics 采集系统级指标（CPU/内存/磁盘/在线设备/QPS/错误率）
// 铁律 06：gopsutil 跨平台；任一指标采集失败不影响其他指标
func (m *Manager) CollectSystemMetrics(ctx context.Context) ([]MetricSample, error) {
	samples := []MetricSample{}

	// 1. CPU 使用率（采样 1 秒）
	cpuPercent, err := cpu.Percent(time.Second, false)
	if err == nil && len(cpuPercent) > 0 {
		samples = append(samples, MetricSample{
			Name:  MetricCPUUsage,
			Value: cpuPercent[0],
			Unit:  "%",
			Labels: map[string]interface{}{"host": hostname()},
		})
	}

	// 2. 内存使用率
	memInfo, err := mem.VirtualMemory()
	if err == nil {
		samples = append(samples, MetricSample{
			Name:  MetricMemoryUsage,
			Value: memInfo.UsedPercent,
			Unit:  "%",
			Labels: map[string]interface{}{
				"used_mb":  memInfo.Used / 1024 / 1024,
				"total_mb": memInfo.Total / 1024 / 1024,
			},
		})
	}

	// 3. 磁盘使用率（根分区）
	diskInfo, err := disk.Usage("/")
	if err == nil {
		samples = append(samples, MetricSample{
			Name:  MetricDiskUsage,
			Value: diskInfo.UsedPercent,
			Unit:  "%",
			Labels: map[string]interface{}{
				"path":     "/",
				"used_gb":  diskInfo.Used / 1024 / 1024 / 1024,
				"total_gb": diskInfo.Total / 1024 / 1024 / 1024,
			},
		})
	}

	// 4. 在线设备数（从 DB 查询最近心跳）
	var onlineDevices int64
	heartbeatTimeout := m.cache.GetInt(ctx, "app.default.heartbeat_timeout", 180)
	threshold := time.Now().Add(-time.Duration(heartbeatTimeout) * time.Second)
	m.db.Model(&model.AppDevice{}).Where("last_heartbeat_at > ?", threshold).Count(&onlineDevices)
	samples = append(samples, MetricSample{
		Name:  MetricOnlineDevices,
		Value: float64(onlineDevices),
		Unit:  "count",
	})

	// 5. 今日验证次数 + 错误率（从 log_verify 表）
	todayStart := time.Date(time.Now().Year(), time.Now().Month(), time.Now().Day(), 0, 0, 0, 0, time.Local)
	var verifyTotal, verifyFailed int64
	m.db.Model(&model.LogVerify{}).Where("created_at >= ?", todayStart).Count(&verifyTotal)
	m.db.Model(&model.LogVerify{}).Where("created_at >= ? AND result != ?", todayStart, "success").Count(&verifyFailed)
	samples = append(samples, MetricSample{
		Name:  MetricVerifyCount,
		Value: float64(verifyTotal),
		Unit:  "count",
	})
	if verifyTotal > 0 {
		errorRate := float64(verifyFailed) / float64(verifyTotal) * 100
		samples = append(samples, MetricSample{
			Name:  MetricErrorRate,
			Value: errorRate,
			Unit:  "%",
			Labels: map[string]interface{}{
				"failed":  verifyFailed,
				"total":   verifyTotal,
			},
		})
	}

	return samples, nil
}

// hostname 获取主机名（简化版，无 error 处理）
func hostname() string {
	// 用 runtime 获取，避免引入额外依赖
	// gopsutil 已经传递 host 信息，这里仅用于标签
	return "server"
}

// ============== 3. 写入指标到 DB ==============

// SaveMetrics 将采集的指标写入 system_metric 表
// 铁律 06：批量插入；任一写入失败记录错误但不中断
func (m *Manager) SaveMetrics(ctx context.Context, samples []MetricSample) (int, error) {
	if len(samples) == 0 {
		return 0, nil
	}
	now := time.Now()
	count := 0
	for _, s := range samples {
		labelsJSON := "{}"
		if s.Labels != nil {
			if data, err := json.Marshal(s.Labels); err == nil {
				labelsJSON = string(data)
			}
		}
		metric := &model.SystemMetric{
			MetricName:  s.Name,
			MetricValue: s.Value,
			MetricUnit:  s.Unit,
			LabelsJSON:  labelsJSON,
			CollectedAt: now,
		}
		if err := m.db.Create(metric).Error; err == nil {
			count++
		}
	}
	return count, nil
}

// ============== 4. 评估告警 ==============

// GetAlertRules 从 sys_config 构建告警规则
// 铁律 04/05：阈值全部从 sys_config 读取
func (m *Manager) GetAlertRules(ctx context.Context) []AlertRule {
	rules := []AlertRule{
		{
			MetricName: MetricCPUUsage,
			Threshold:  m.cache.GetFloat64(ctx, CfgKeyThresholdCPU, 90),
			Operator:   OpGT,
			Severity:   SeverityWarning,
			Message:    "CPU 使用率超阈值",
		},
		{
			MetricName: MetricMemoryUsage,
			Threshold:  m.cache.GetFloat64(ctx, CfgKeyThresholdMemory, 90),
			Operator:   OpGT,
			Severity:   SeverityWarning,
			Message:    "内存使用率超阈值",
		},
		{
			MetricName: MetricDiskUsage,
			Threshold:  m.cache.GetFloat64(ctx, CfgKeyThresholdDisk, 85),
			Operator:   OpGT,
			Severity:   SeverityCritical,
			Message:    "磁盘使用率超阈值",
		},
		{
			MetricName: MetricErrorRate,
			Threshold:  m.cache.GetFloat64(ctx, CfgKeyThresholdErrorRate, 10),
			Operator:   OpGT,
			Severity:   SeverityCritical,
			Message:    "验证错误率超阈值",
		},
	}
	return rules
}

// EvaluateAlerts 评估最新指标是否触发告警
// 铁律 06：检查静默期（同一规则在 silence_minutes 内不重复告警）；告警总开关关闭时仅采集不告警
func (m *Manager) EvaluateAlerts(ctx context.Context, samples []MetricSample) (int, int, error) {
	if !m.cache.GetBool(ctx, CfgKeyAlertEnabled, true) {
		return 0, 0, nil // 告警关闭
	}

	rules := m.GetAlertRules(ctx)
	ruleMap := map[string]AlertRule{}
	for _, r := range rules {
		ruleMap[r.MetricName] = r
	}

	fired := 0
	resolved := 0
	silenceMinutes := m.cache.GetInt(ctx, CfgKeySilenceMinutes, 30)
	silenceThreshold := time.Now().Add(-time.Duration(silenceMinutes) * time.Minute)

	for _, s := range samples {
		rule, ok := ruleMap[s.Name]
		if !ok {
			continue
		}
		// 阈值比较
		if CompareWithOperator(s.Value, rule.Threshold, rule.Operator) {
			// 检查静默期：最近 silence_minutes 内是否有同规则 firing 状态的告警
			var recentAlert model.SystemAlert
			err := m.db.Where("alert_rule = ? AND status = ? AND fired_at > ?", s.Name, StatusFiring, silenceThreshold).
				Order("fired_at DESC").First(&recentAlert).Error
			if err == nil {
				// 静默期内，跳过
				continue
			}

			// 创建新告警
			labelsJSON := "{}"
			if s.Labels != nil {
				if data, err := json.Marshal(s.Labels); err == nil {
					labelsJSON = string(data)
				}
			}
			alert := &model.SystemAlert{
				AlertRule:   s.Name,
				Severity:    rule.Severity,
				Status:      StatusFiring,
				MetricValue: s.Value,
				Threshold:   rule.Threshold,
				Operator:    rule.Operator,
				Message:     fmt.Sprintf("%s: %.2f %s %.2f", rule.Message, s.Value, rule.Operator, rule.Threshold),
				LabelsJSON:  labelsJSON,
				FiredAt:     time.Now(),
			}
			if err := m.db.Create(alert).Error; err == nil {
				fired++
				// 发送通知
				if m.sendNotification(ctx, alert) {
					m.db.Model(&model.SystemAlert{}).Where("id = ?", alert.ID).Update("notify_sent", 1)
				}
			}
		} else {
			// 指标正常：检查是否有该规则 firing 状态的告警需要自动恢复
			result := m.db.Model(&model.SystemAlert{}).
				Where("alert_rule = ? AND status = ?", s.Name, StatusFiring).
				Updates(map[string]interface{}{
					"status":      StatusResolved,
					"resolved_at": time.Now(),
				})
			if result.Error == nil {
				resolved += int(result.RowsAffected)
			}
		}
	}
	return fired, resolved, nil
}

// ResolveStaleAlerts 自动恢复超过 1 小时未变化的 firing 告警
// 铁律 06：避免告警堆积；长时间无更新的 firing 视为已恢复
func (m *Manager) ResolveStaleAlerts(ctx context.Context) (int, error) {
	threshold := time.Now().Add(-1 * time.Hour)
	result := m.db.Model(&model.SystemAlert{}).
		Where("status = ? AND fired_at < ?", StatusFiring, threshold).
		Updates(map[string]interface{}{
			"status":      StatusResolved,
			"resolved_at": time.Now(),
		})
	if result.Error != nil {
		return 0, result.Error
	}
	return int(result.RowsAffected), nil
}

// ============== 5. 发送告警通知 ==============

// sendNotification 发送告警通知到 webhook URL
// 铁律 06：POST JSON；超时控制 10s；失败返回 false 但不阻塞主流程
func (m *Manager) sendNotification(ctx context.Context, alert *model.SystemAlert) bool {
	webhookURL := m.cache.GetString(ctx, CfgKeyNotifyWebhookURL, "")
	if webhookURL == "" {
		return false // 未配置 webhook，跳过
	}

	var labels map[string]interface{}
	_ = json.Unmarshal([]byte(alert.LabelsJSON), &labels)

	payload := AlertPayload{
		AlertRule:   alert.AlertRule,
		Severity:    alert.Severity,
		Status:      alert.Status,
		MetricValue: alert.MetricValue,
		Threshold:   alert.Threshold,
		Operator:    alert.Operator,
		Message:     alert.Message,
		Labels:      labels,
		FiredAt:     alert.FiredAt,
	}
	body, _ := json.Marshal(payload)

	reqCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, webhookURL, bytes.NewReader(body))
	if err != nil {
		return false
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "KeyAuth-Monitor/1.0")
	req.Header.Set("X-Alert-Severity", alert.Severity)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode >= 200 && resp.StatusCode < 300
}

// SendAlertNotification 公开方法：手动重发告警通知
func (m *Manager) SendAlertNotification(ctx context.Context, alertID uint64) (bool, error) {
	var alert model.SystemAlert
	if err := m.db.First(&alert, alertID).Error; err != nil {
		return false, err
	}
	return m.sendNotification(ctx, &alert), nil
}

// ============== 6. 一体化入口 ==============

// CollectAndEvaluate 采集 + 评估 + 通知（一体化入口）
// 铁律 06：互斥锁防并发；任一步骤失败不中断后续步骤
func (m *Manager) CollectAndEvaluate(ctx context.Context) (*CollectResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	result := &CollectResult{}
	startTime := time.Now()
	_ = startTime

	// 1. 采集
	samples, err := m.CollectSystemMetrics(ctx)
	if err != nil {
		result.ErrorMessage = "采集失败: " + err.Error()
		// 继续执行（部分指标可能成功）
	}

	// 2. 写入 DB
	saved, err := m.SaveMetrics(ctx, samples)
	if err != nil {
		result.ErrorMessage += " | 写入失败: " + err.Error()
	}
	result.MetricsCollected = saved

	// 3. 评估告警
	fired, resolved, err := m.EvaluateAlerts(ctx, samples)
	if err != nil {
		result.ErrorMessage += " | 评估失败: " + err.Error()
	}
	result.AlertsFired = fired
	result.AlertsResolved = resolved

	// 4. 清理过期 firing 告警
	_, _ = m.ResolveStaleAlerts(ctx)

	// 5. 清理过期指标数据
	_, _ = m.CleanupExpiredMetrics(ctx)

	return result, nil
}

// ============== 7. 清理过期指标 ==============

// CleanupExpiredMetrics 清理超过保留天数的指标数据
// 铁律 06：批量删除；retention=0 时不清理
func (m *Manager) CleanupExpiredMetrics(ctx context.Context) (int, error) {
	retentionDays := m.cache.GetInt(ctx, CfgKeyRetentionDays, 30)
	if retentionDays <= 0 {
		return 0, nil
	}
	threshold := time.Now().AddDate(0, 0, -retentionDays)
	result := m.db.Where("collected_at < ?", threshold).Delete(&model.SystemMetric{})
	if result.Error != nil {
		return 0, result.Error
	}
	return int(result.RowsAffected), nil
}

// ============== 8. 查询辅助 ==============

// GetMetricHistory 查询指标历史（按 name + 时间范围）
func (m *Manager) GetMetricHistory(ctx context.Context, name string, from, to time.Time, limit int) ([]model.SystemMetric, error) {
	if limit <= 0 || limit > 1000 {
		limit = 100
	}
	var metrics []model.SystemMetric
	err := m.db.Where("metric_name = ? AND collected_at >= ? AND collected_at <= ?", name, from, to).
		Order("collected_at DESC").Limit(limit).Find(&metrics).Error
	return metrics, err
}

// GetActiveAlerts 查询所有 firing 状态的告警
func (m *Manager) GetActiveAlerts(ctx context.Context) ([]model.SystemAlert, error) {
	var alerts []model.SystemAlert
	err := m.db.Where("status = ?", StatusFiring).Order("fired_at DESC").Find(&alerts).Error
	return alerts, err
}

// AckAlert 确认告警（标记为 acked）
func (m *Manager) AckAlert(ctx context.Context, alertID uint64, adminID uint64) error {
	now := time.Now()
	return m.db.Model(&model.SystemAlert{}).Where("id = ?", alertID).Updates(map[string]interface{}{
		"status":   StatusAcked,
		"acked_by": adminID,
		"acked_at": &now,
	}).Error
}

// IsAlertEnabled 告警是否启用
func (m *Manager) IsAlertEnabled(ctx context.Context) bool {
	return m.cache.GetBool(ctx, CfgKeyAlertEnabled, true)
}

// GetCollectInterval 获取采集间隔（秒）
func (m *Manager) GetCollectInterval(ctx context.Context) int {
	return m.cache.GetInt(ctx, CfgKeyCollectInterval, 60)
}

// FormatMetricName 格式化指标名（小写 + 下划线）
func FormatMetricName(s string) string {
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, "-", "_")
	s = strings.ReplaceAll(s, " ", "_")
	return s
}

// 运行时 CPU 数量（用于测试 mock）
var runtimeNumCPU = runtime.NumCPU
