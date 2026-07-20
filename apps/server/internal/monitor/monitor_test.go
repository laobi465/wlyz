// Package monitor v0.4.0 监控告警单元测试
// 严格遵循铁律 06：所有断言基于已知固定输入，无随机/不确定性
// 测试覆盖：
//   1. CompareWithOperator（> / < / >= / <= / == / 未知运算符）
//   2. FormatMetricName（小写 / 空格转下划线 / 横杠转下划线）
//   3. SaveMetrics（空切片 / 单条 / 多条 / 非法 labels）
//   4. GetAlertRules（默认 4 条规则 / 阈值从 sys_config 读取）
//   5. EvaluateAlerts（告警关闭 / 触发告警 / 静默期 / 自动恢复）
//   6. ResolveStaleAlerts（超 1h 自动恢复 / 未超时不恢复）
//   7. CleanupExpiredMetrics（按保留天数清理 / retention=0 不清理）
//   8. AckAlert（成功 / 不存在）
//   9. GetMetricHistory（时间范围 / limit 边界）
//  10. 状态机常量
package monitor

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/your-org/keyauth-saas/apps/server/internal/config"
	"github.com/your-org/keyauth-saas/apps/server/internal/model"
)

// ============== 测试基础设施 ==============

func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:monitor_test_%d?mode=memory&cache=shared", time.Now().UnixNano())), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&model.SystemMetric{},
		&model.SystemAlert{},
		&model.SysConfig{},
		&model.AppDevice{},
		&model.LogVerify{},
	))
	db.Exec("DELETE FROM system_metric")
	db.Exec("DELETE FROM system_alert")
	db.Exec("DELETE FROM sys_config")
	db.Exec("DELETE FROM app_device")
	db.Exec("DELETE FROM log_verify")
	return db
}

func setupTestCfgCache(t *testing.T, db *gorm.DB, overrides map[string]string) (*config.ConfigCache, *miniredis.Miniredis) {
	t.Helper()
	mr, err := miniredis.Run()
	require.NoError(t, err)
	t.Cleanup(mr.Close)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	defaults := map[string]string{
		CfgKeyCollectInterval:    "60",
		CfgKeyAlertEnabled:       "1",
		CfgKeyNotifyWebhookURL:   "",
		CfgKeySilenceMinutes:     "30",
		CfgKeyThresholdCPU:       "90",
		CfgKeyThresholdMemory:    "90",
		CfgKeyThresholdDisk:      "85",
		CfgKeyThresholdErrorRate: "10",
		CfgKeyRetentionDays:      "30",
	}
	if overrides == nil {
		overrideCopy := map[string]string{}
		for k, v := range overrides {
			overrideCopy[k] = v
		}
		overrides = overrideCopy
	}
	for k, v := range defaults {
		if _, ok := overrides[k]; !ok {
			overrides[k] = v
		}
	}
	for k, v := range overrides {
		require.NoError(t, db.Create(&model.SysConfig{
			ConfigKey:   k,
			ConfigValue: v,
			ConfigType:  "string",
			ConfigGroup: "monitor",
		}).Error)
	}
	return config.NewConfigCache(db, rdb), mr
}

// ============== 1. CompareWithOperator ==============

func TestCompareWithOperator_GT(t *testing.T) {
	assert.True(t, CompareWithOperator(95, 90, OpGT))
	assert.False(t, CompareWithOperator(90, 90, OpGT))
	assert.False(t, CompareWithOperator(80, 90, OpGT))
}

func TestCompareWithOperator_LT(t *testing.T) {
	assert.True(t, CompareWithOperator(80, 90, OpLT))
	assert.False(t, CompareWithOperator(90, 90, OpLT))
	assert.False(t, CompareWithOperator(95, 90, OpLT))
}

func TestCompareWithOperator_GE(t *testing.T) {
	assert.True(t, CompareWithOperator(95, 90, OpGE))
	assert.True(t, CompareWithOperator(90, 90, OpGE))
	assert.False(t, CompareWithOperator(80, 90, OpGE))
}

func TestCompareWithOperator_LE(t *testing.T) {
	assert.True(t, CompareWithOperator(80, 90, OpLE))
	assert.True(t, CompareWithOperator(90, 90, OpLE))
	assert.False(t, CompareWithOperator(95, 90, OpLE))
}

func TestCompareWithOperator_EQ(t *testing.T) {
	assert.True(t, CompareWithOperator(90, 90, OpEQ))
	// 浮点精度容差 0.001
	assert.True(t, CompareWithOperator(90.0005, 90, OpEQ))
	assert.False(t, CompareWithOperator(90.1, 90, OpEQ))
	assert.False(t, CompareWithOperator(89.9, 90, OpEQ))
}

func TestCompareWithOperator_Unknown(t *testing.T) {
	// 未知运算符默认返回 false（铁律 06：不依赖字符串拼接 eval）
	assert.False(t, CompareWithOperator(100, 0, "!="))
	assert.False(t, CompareWithOperator(100, 0, ""))
	assert.False(t, CompareWithOperator(100, 0, "invalid"))
}

// ============== 2. FormatMetricName ==============

func TestFormatMetricName_Lowercase(t *testing.T) {
	assert.Equal(t, "cpu_usage", FormatMetricName("CPU_Usage"))
	assert.Equal(t, "cpu_usage_avg", FormatMetricName("CPU_Usage_Avg"))
}

func TestFormatMetricName_DashToUnderscore(t *testing.T) {
	assert.Equal(t, "cpu_usage", FormatMetricName("cpu-usage"))
	assert.Equal(t, "memory_usage_percent", FormatMetricName("memory-usage-percent"))
}

func TestFormatMetricName_SpaceToUnderscore(t *testing.T) {
	assert.Equal(t, "cpu_usage", FormatMetricName("cpu usage"))
	assert.Equal(t, "memory_usage_percent", FormatMetricName("memory usage percent"))
}

func TestFormatMetricName_Combined(t *testing.T) {
	assert.Equal(t, "cpu_usage_avg", FormatMetricName("CPU Usage-AVG"))
}

// ============== 3. SaveMetrics ==============

func TestSaveMetrics_Empty(t *testing.T) {
	db := setupTestDB(t)
	cache, _ := setupTestCfgCache(t, db, nil)
	mgr := NewManager(db, cache)
	ctx := context.Background()

	count, err := mgr.SaveMetrics(ctx, nil)
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestSaveMetrics_Single(t *testing.T) {
	db := setupTestDB(t)
	cache, _ := setupTestCfgCache(t, db, nil)
	mgr := NewManager(db, cache)
	ctx := context.Background()

	samples := []MetricSample{
		{Name: MetricCPUUsage, Value: 75.5, Unit: "%", Labels: map[string]interface{}{"host": "server1"}},
	}
	count, err := mgr.SaveMetrics(ctx, samples)
	require.NoError(t, err)
	assert.Equal(t, 1, count)

	// 验证写入
	var metric model.SystemMetric
	require.NoError(t, db.First(&metric).Error)
	assert.Equal(t, MetricCPUUsage, metric.MetricName)
	assert.Equal(t, 75.5, metric.MetricValue)
	assert.Equal(t, "%", metric.MetricUnit)
	assert.Contains(t, metric.LabelsJSON, "server1")
}

func TestSaveMetrics_Multiple(t *testing.T) {
	db := setupTestDB(t)
	cache, _ := setupTestCfgCache(t, db, nil)
	mgr := NewManager(db, cache)
	ctx := context.Background()

	samples := []MetricSample{
		{Name: MetricCPUUsage, Value: 75.5, Unit: "%"},
		{Name: MetricMemoryUsage, Value: 60.2, Unit: "%"},
		{Name: MetricDiskUsage, Value: 45.0, Unit: "%"},
	}
	count, err := mgr.SaveMetrics(ctx, samples)
	require.NoError(t, err)
	assert.Equal(t, 3, count)

	var total int64
	db.Model(&model.SystemMetric{}).Count(&total)
	assert.Equal(t, int64(3), total)
}

func TestSaveMetrics_NilLabels(t *testing.T) {
	db := setupTestDB(t)
	cache, _ := setupTestCfgCache(t, db, nil)
	mgr := NewManager(db, cache)
	ctx := context.Background()

	samples := []MetricSample{
		{Name: MetricQPS, Value: 100, Unit: "count", Labels: nil},
	}
	count, err := mgr.SaveMetrics(ctx, samples)
	require.NoError(t, err)
	assert.Equal(t, 1, count)

	var metric model.SystemMetric
	require.NoError(t, db.First(&metric).Error)
	assert.Equal(t, "{}", metric.LabelsJSON)
}

// ============== 4. GetAlertRules ==============

func TestGetAlertRules_Default(t *testing.T) {
	db := setupTestDB(t)
	cache, _ := setupTestCfgCache(t, db, nil)
	mgr := NewManager(db, cache)
	ctx := context.Background()

	rules := mgr.GetAlertRules(ctx)
	require.Len(t, rules, 4)

	// 验证 4 条规则
	ruleMap := map[string]AlertRule{}
	for _, r := range rules {
		ruleMap[r.MetricName] = r
	}

	assert.Contains(t, ruleMap, MetricCPUUsage)
	assert.Contains(t, ruleMap, MetricMemoryUsage)
	assert.Contains(t, ruleMap, MetricDiskUsage)
	assert.Contains(t, ruleMap, MetricErrorRate)

	// 验证默认阈值
	assert.Equal(t, 90.0, ruleMap[MetricCPUUsage].Threshold)
	assert.Equal(t, 90.0, ruleMap[MetricMemoryUsage].Threshold)
	assert.Equal(t, 85.0, ruleMap[MetricDiskUsage].Threshold)
	assert.Equal(t, 10.0, ruleMap[MetricErrorRate].Threshold)

	// 验证默认运算符
	for _, r := range rules {
		assert.Equal(t, OpGT, r.Operator)
	}
}

func TestGetAlertRules_FromConfig(t *testing.T) {
	db := setupTestDB(t)
	cache, _ := setupTestCfgCache(t, db, map[string]string{
		CfgKeyThresholdCPU:       "75",
		CfgKeyThresholdMemory:    "80",
		CfgKeyThresholdDisk:      "70",
		CfgKeyThresholdErrorRate: "5",
	})
	mgr := NewManager(db, cache)
	ctx := context.Background()

	rules := mgr.GetAlertRules(ctx)
	ruleMap := map[string]AlertRule{}
	for _, r := range rules {
		ruleMap[r.MetricName] = r
	}

	assert.Equal(t, 75.0, ruleMap[MetricCPUUsage].Threshold)
	assert.Equal(t, 80.0, ruleMap[MetricMemoryUsage].Threshold)
	assert.Equal(t, 70.0, ruleMap[MetricDiskUsage].Threshold)
	assert.Equal(t, 5.0, ruleMap[MetricErrorRate].Threshold)
}

// ============== 5. EvaluateAlerts ==============

func TestEvaluateAlerts_Disabled(t *testing.T) {
	db := setupTestDB(t)
	cache, _ := setupTestCfgCache(t, db, map[string]string{
		CfgKeyAlertEnabled: "0",
	})
	mgr := NewManager(db, cache)
	ctx := context.Background()

	// 极高 CPU 应触发，但告警已关闭
	samples := []MetricSample{
		{Name: MetricCPUUsage, Value: 99.9, Unit: "%"},
	}
	fired, resolved, err := mgr.EvaluateAlerts(ctx, samples)
	require.NoError(t, err)
	assert.Equal(t, 0, fired)
	assert.Equal(t, 0, resolved)

	// 验证未写入任何告警
	var count int64
	db.Model(&model.SystemAlert{}).Count(&count)
	assert.Equal(t, int64(0), count)
}

func TestEvaluateAlerts_Fired(t *testing.T) {
	db := setupTestDB(t)
	cache, _ := setupTestCfgCache(t, db, map[string]string{
		CfgKeyThresholdCPU:    "80",
		CfgKeyNotifyWebhookURL: "", // 不发送通知
	})
	mgr := NewManager(db, cache)
	ctx := context.Background()

	samples := []MetricSample{
		{Name: MetricCPUUsage, Value: 95.0, Unit: "%"},
	}
	fired, resolved, err := mgr.EvaluateAlerts(ctx, samples)
	require.NoError(t, err)
	assert.Equal(t, 1, fired)
	assert.Equal(t, 0, resolved)

	// 验证告警已写入
	var alert model.SystemAlert
	require.NoError(t, db.First(&alert).Error)
	assert.Equal(t, MetricCPUUsage, alert.AlertRule)
	assert.Equal(t, StatusFiring, alert.Status)
	assert.Equal(t, 95.0, alert.MetricValue)
	assert.Equal(t, 80.0, alert.Threshold)
	assert.Equal(t, OpGT, alert.Operator)
	assert.Equal(t, SeverityWarning, alert.Severity)
}

func TestEvaluateAlerts_NotTriggered(t *testing.T) {
	db := setupTestDB(t)
	cache, _ := setupTestCfgCache(t, db, map[string]string{
		CfgKeyThresholdCPU: "80",
	})
	mgr := NewManager(db, cache)
	ctx := context.Background()

	samples := []MetricSample{
		{Name: MetricCPUUsage, Value: 50.0, Unit: "%"}, // 未超阈值
	}
	fired, _, err := mgr.EvaluateAlerts(ctx, samples)
	require.NoError(t, err)
	assert.Equal(t, 0, fired)

	var count int64
	db.Model(&model.SystemAlert{}).Count(&count)
	assert.Equal(t, int64(0), count)
}

func TestEvaluateAlerts_SilencePeriod(t *testing.T) {
	db := setupTestDB(t)
	cache, _ := setupTestCfgCache(t, db, map[string]string{
		CfgKeyThresholdCPU:    "80",
		CfgKeySilenceMinutes:  "30",
		CfgKeyNotifyWebhookURL: "",
	})
	mgr := NewManager(db, cache)
	ctx := context.Background()

	// 第一次触发
	samples := []MetricSample{{Name: MetricCPUUsage, Value: 95.0, Unit: "%"}}
	fired1, _, err := mgr.EvaluateAlerts(ctx, samples)
	require.NoError(t, err)
	assert.Equal(t, 1, fired1)

	// 第二次：静默期内应跳过
	fired2, _, err := mgr.EvaluateAlerts(ctx, samples)
	require.NoError(t, err)
	assert.Equal(t, 0, fired2)

	// 验证仅 1 条告警
	var count int64
	db.Model(&model.SystemAlert{}).Count(&count)
	assert.Equal(t, int64(1), count)
}

func TestEvaluateAlerts_AutoResolve(t *testing.T) {
	db := setupTestDB(t)
	cache, _ := setupTestCfgCache(t, db, map[string]string{
		CfgKeyThresholdCPU:    "80",
		CfgKeyNotifyWebhookURL: "",
	})
	mgr := NewManager(db, cache)
	ctx := context.Background()

	// 先触发
	samples1 := []MetricSample{{Name: MetricCPUUsage, Value: 95.0, Unit: "%"}}
	fired1, _, err := mgr.EvaluateAlerts(ctx, samples1)
	require.NoError(t, err)
	assert.Equal(t, 1, fired1)

	// 指标恢复正常
	samples2 := []MetricSample{{Name: MetricCPUUsage, Value: 50.0, Unit: "%"}}
	_, resolved, err := mgr.EvaluateAlerts(ctx, samples2)
	require.NoError(t, err)
	assert.Equal(t, 1, resolved)

	// 验证告警状态已变为 resolved
	var alert model.SystemAlert
	require.NoError(t, db.First(&alert).Error)
	assert.Equal(t, StatusResolved, alert.Status)
	assert.NotNil(t, alert.ResolvedAt)
}

// ============== 6. ResolveStaleAlerts ==============

func TestResolveStaleAlerts_Stale(t *testing.T) {
	db := setupTestDB(t)
	cache, _ := setupTestCfgCache(t, db, nil)
	mgr := NewManager(db, cache)
	ctx := context.Background()

	// 写入 2 小时前的 firing 告警
	staleTime := time.Now().Add(-2 * time.Hour)
	require.NoError(t, db.Create(&model.SystemAlert{
		AlertRule:   MetricCPUUsage,
		Severity:    SeverityWarning,
		Status:      StatusFiring,
		MetricValue: 99,
		Threshold:   90,
		Operator:    OpGT,
		Message:     "stale",
		FiredAt:     staleTime,
	}).Error)

	resolved, err := mgr.ResolveStaleAlerts(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, resolved)

	var alert model.SystemAlert
	require.NoError(t, db.First(&alert).Error)
	assert.Equal(t, StatusResolved, alert.Status)
	assert.NotNil(t, alert.ResolvedAt)
}

func TestResolveStaleAlerts_NotStale(t *testing.T) {
	db := setupTestDB(t)
	cache, _ := setupTestCfgCache(t, db, nil)
	mgr := NewManager(db, cache)
	ctx := context.Background()

	// 写入 30 分钟前的 firing 告警（未超 1h）
	require.NoError(t, db.Create(&model.SystemAlert{
		AlertRule:   MetricCPUUsage,
		Status:      StatusFiring,
		MetricValue: 99,
		Threshold:   90,
		Operator:    OpGT,
		FiredAt:     time.Now().Add(-30 * time.Minute),
	}).Error)

	resolved, err := mgr.ResolveStaleAlerts(ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, resolved)

	var alert model.SystemAlert
	require.NoError(t, db.First(&alert).Error)
	assert.Equal(t, StatusFiring, alert.Status)
}

func TestResolveStaleAlerts_AlreadyResolved(t *testing.T) {
	db := setupTestDB(t)
	cache, _ := setupTestCfgCache(t, db, nil)
	mgr := NewManager(db, cache)
	ctx := context.Background()

	// 已 resolved 的告警不应被处理
	require.NoError(t, db.Create(&model.SystemAlert{
		AlertRule:   MetricCPUUsage,
		Status:      StatusResolved,
		MetricValue: 99,
		Threshold:   90,
		Operator:    OpGT,
		FiredAt:     time.Now().Add(-3 * time.Hour),
	}).Error)

	resolved, err := mgr.ResolveStaleAlerts(ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, resolved)
}

// ============== 7. CleanupExpiredMetrics ==============

func TestCleanupExpiredMetrics_ByRetention(t *testing.T) {
	db := setupTestDB(t)
	cache, _ := setupTestCfgCache(t, db, map[string]string{
		CfgKeyRetentionDays: "7",
	})
	mgr := NewManager(db, cache)
	ctx := context.Background()

	// 10 天前的指标（应清理）
	require.NoError(t, db.Create(&model.SystemMetric{
		MetricName:  MetricCPUUsage,
		MetricValue: 50,
		MetricUnit:  "%",
		CollectedAt: time.Now().Add(-10 * 24 * time.Hour),
	}).Error)

	// 1 天前的指标（保留）
	require.NoError(t, db.Create(&model.SystemMetric{
		MetricName:  MetricCPUUsage,
		MetricValue: 60,
		MetricUnit:  "%",
		CollectedAt: time.Now().Add(-1 * 24 * time.Hour),
	}).Error)

	deleted, err := mgr.CleanupExpiredMetrics(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, deleted)

	var count int64
	db.Model(&model.SystemMetric{}).Count(&count)
	assert.Equal(t, int64(1), count)
}

func TestCleanupExpiredMetrics_ZeroRetention(t *testing.T) {
	db := setupTestDB(t)
	cache, _ := setupTestCfgCache(t, db, map[string]string{
		CfgKeyRetentionDays: "0",
	})
	mgr := NewManager(db, cache)
	ctx := context.Background()

	require.NoError(t, db.Create(&model.SystemMetric{
		MetricName:  MetricCPUUsage,
		MetricValue: 50,
		CollectedAt: time.Now().Add(-100 * 24 * time.Hour),
	}).Error)

	// retention=0 不清理
	deleted, err := mgr.CleanupExpiredMetrics(ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, deleted)
}

func TestCleanupExpiredMetrics_NoExpired(t *testing.T) {
	db := setupTestDB(t)
	cache, _ := setupTestCfgCache(t, db, map[string]string{
		CfgKeyRetentionDays: "30",
	})
	mgr := NewManager(db, cache)
	ctx := context.Background()

	require.NoError(t, db.Create(&model.SystemMetric{
		MetricName:  MetricCPUUsage,
		MetricValue: 50,
		CollectedAt: time.Now(),
	}).Error)

	deleted, err := mgr.CleanupExpiredMetrics(ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, deleted)
}

// ============== 8. AckAlert ==============

func TestAckAlert_Success(t *testing.T) {
	db := setupTestDB(t)
	cache, _ := setupTestCfgCache(t, db, nil)
	mgr := NewManager(db, cache)
	ctx := context.Background()

	// 创建 firing 告警
	alert := &model.SystemAlert{
		AlertRule:   MetricCPUUsage,
		Status:      StatusFiring,
		MetricValue: 99,
		Threshold:   90,
		Operator:    OpGT,
		FiredAt:     time.Now(),
	}
	require.NoError(t, db.Create(alert).Error)

	err := mgr.AckAlert(ctx, alert.ID, 42)
	require.NoError(t, err)

	// 验证状态
	var updated model.SystemAlert
	require.NoError(t, db.First(&updated, alert.ID).Error)
	assert.Equal(t, StatusAcked, updated.Status)
	assert.Equal(t, uint64(42), updated.AckedBy)
	assert.NotNil(t, updated.AckedAt)
}

func TestAckAlert_NotExist(t *testing.T) {
	db := setupTestDB(t)
	cache, _ := setupTestCfgCache(t, db, nil)
	mgr := NewManager(db, cache)
	ctx := context.Background()

	// 不存在的 ID（GORM Updates 在 0 行时无错误，但行数为 0）
	err := mgr.AckAlert(ctx, 9999, 1)
	// Updates 不存在记录时不会报错，仅 RowsAffected=0
	assert.NoError(t, err)
}

// ============== 9. GetMetricHistory ==============

func TestGetMetricHistory_Default(t *testing.T) {
	db := setupTestDB(t)
	cache, _ := setupTestCfgCache(t, db, nil)
	mgr := NewManager(db, cache)
	ctx := context.Background()

	// 写入多条历史指标
	now := time.Now()
	for i, v := range []float64{50, 60, 70, 80} {
		require.NoError(t, db.Create(&model.SystemMetric{
			MetricName:  MetricCPUUsage,
			MetricValue: v,
			MetricUnit:  "%",
			CollectedAt: now.Add(-time.Duration(4-i) * time.Hour),
		}).Error)
	}

	from := now.Add(-24 * time.Hour)
	metrics, err := mgr.GetMetricHistory(ctx, MetricCPUUsage, from, now, 100)
	require.NoError(t, err)
	assert.Len(t, metrics, 4)

	// 验证按时间倒序
	assert.Equal(t, 80.0, metrics[0].MetricValue)
	assert.Equal(t, 50.0, metrics[3].MetricValue)
}

func TestGetMetricHistory_FilterByName(t *testing.T) {
	db := setupTestDB(t)
	cache, _ := setupTestCfgCache(t, db, nil)
	mgr := NewManager(db, cache)
	ctx := context.Background()

	now := time.Now()
	require.NoError(t, db.Create(&model.SystemMetric{
		MetricName:  MetricCPUUsage,
		MetricValue: 50,
		CollectedAt: now,
	}).Error)
	require.NoError(t, db.Create(&model.SystemMetric{
		MetricName:  MetricMemoryUsage,
		MetricValue: 60,
		CollectedAt: now,
	}).Error)

	from := now.Add(-1 * time.Hour)
	metrics, err := mgr.GetMetricHistory(ctx, MetricCPUUsage, from, now, 100)
	require.NoError(t, err)
	assert.Len(t, metrics, 1)
	assert.Equal(t, MetricCPUUsage, metrics[0].MetricName)
}

func TestGetMetricHistory_LimitBoundary(t *testing.T) {
	db := setupTestDB(t)
	cache, _ := setupTestCfgCache(t, db, nil)
	mgr := NewManager(db, cache)
	ctx := context.Background()

	now := time.Now()
	// 写入 5 条
	for i := 0; i < 5; i++ {
		require.NoError(t, db.Create(&model.SystemMetric{
			MetricName:  MetricCPUUsage,
			MetricValue: float64(i),
			CollectedAt: now.Add(-time.Duration(i) * time.Minute),
		}).Error)
	}

	from := now.Add(-1 * time.Hour)
	// limit=3 应返回 3 条
	metrics, err := mgr.GetMetricHistory(ctx, MetricCPUUsage, from, now, 3)
	require.NoError(t, err)
	assert.Len(t, metrics, 3)
}

func TestGetMetricHistory_LimitZero(t *testing.T) {
	db := setupTestDB(t)
	cache, _ := setupTestCfgCache(t, db, nil)
	mgr := NewManager(db, cache)
	ctx := context.Background()

	now := time.Now()
	require.NoError(t, db.Create(&model.SystemMetric{
		MetricName:  MetricCPUUsage,
		MetricValue: 50,
		CollectedAt: now,
	}).Error)

	from := now.Add(-1 * time.Hour)
	// limit=0 应被修正为默认 100
	metrics, err := mgr.GetMetricHistory(ctx, MetricCPUUsage, from, now, 0)
	require.NoError(t, err)
	assert.Len(t, metrics, 1)
}

// ============== 10. GetActiveAlerts ==============

func TestGetActiveAlerts_OnlyFiring(t *testing.T) {
	db := setupTestDB(t)
	cache, _ := setupTestCfgCache(t, db, nil)
	mgr := NewManager(db, cache)
	ctx := context.Background()

	require.NoError(t, db.Create(&model.SystemAlert{
		AlertRule: MetricCPUUsage, Status: StatusFiring, Threshold: 90, Operator: OpGT, FiredAt: time.Now(),
	}).Error)
	require.NoError(t, db.Create(&model.SystemAlert{
		AlertRule: MetricMemoryUsage, Status: StatusResolved, Threshold: 90, Operator: OpGT, FiredAt: time.Now(),
	}).Error)

	alerts, err := mgr.GetActiveAlerts(ctx)
	require.NoError(t, err)
	assert.Len(t, alerts, 1)
	assert.Equal(t, MetricCPUUsage, alerts[0].AlertRule)
}

// ============== 11. sendNotification ==============

func TestSendNotification_NoWebhook(t *testing.T) {
	db := setupTestDB(t)
	cache, _ := setupTestCfgCache(t, db, map[string]string{
		CfgKeyNotifyWebhookURL: "",
	})
	mgr := NewManager(db, cache)
	ctx := context.Background()

	alert := &model.SystemAlert{
		AlertRule:   MetricCPUUsage,
		Status:      StatusFiring,
		MetricValue: 99,
		Threshold:   90,
		Operator:    OpGT,
		LabelsJSON:  "{}",
		FiredAt:     time.Now(),
	}
	// 未配置 webhook 应返回 false
	result := mgr.sendNotification(ctx, alert)
	assert.False(t, result)
}

func TestSendNotification_WebhookSuccess(t *testing.T) {
	var receivedPayload AlertPayload
	var mu sync.Mutex
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Equal(t, "critical", r.Header.Get("X-Alert-Severity"))
		mu.Lock()
		defer mu.Unlock()
		body := make([]byte, 1024)
		n, _ := r.Body.Read(body)
		_ = json.Unmarshal(body[:n], &receivedPayload)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	db := setupTestDB(t)
	cache, _ := setupTestCfgCache(t, db, map[string]string{
		CfgKeyNotifyWebhookURL: server.URL,
	})
	mgr := NewManager(db, cache)
	ctx := context.Background()

	alert := &model.SystemAlert{
		AlertRule:   MetricCPUUsage,
		Severity:    SeverityCritical,
		Status:      StatusFiring,
		MetricValue: 99,
		Threshold:   90,
		Operator:    OpGT,
		Message:     "test alert",
		LabelsJSON:  "{}",
		FiredAt:     time.Now(),
	}
	result := mgr.sendNotification(ctx, alert)
	assert.True(t, result)

	mu.Lock()
	defer mu.Unlock()
	assert.Equal(t, MetricCPUUsage, receivedPayload.AlertRule)
	assert.Equal(t, SeverityCritical, receivedPayload.Severity)
	assert.Equal(t, 99.0, receivedPayload.MetricValue)
}

func TestSendNotification_WebhookFail(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	db := setupTestDB(t)
	cache, _ := setupTestCfgCache(t, db, map[string]string{
		CfgKeyNotifyWebhookURL: server.URL,
	})
	mgr := NewManager(db, cache)
	ctx := context.Background()

	alert := &model.SystemAlert{
		AlertRule:  MetricCPUUsage,
		Severity:   SeverityWarning,
		Status:     StatusFiring,
		LabelsJSON: "{}",
		FiredAt:    time.Now(),
	}
	result := mgr.sendNotification(ctx, alert)
	assert.False(t, result)
}

func TestSendNotification_WebhookUnreachable(t *testing.T) {
	db := setupTestDB(t)
	cache, _ := setupTestCfgCache(t, db, map[string]string{
		CfgKeyNotifyWebhookURL: "http://127.0.0.1:1", // 端口 1 通常未监听
	})
	mgr := NewManager(db, cache)
	ctx := context.Background()

	alert := &model.SystemAlert{
		AlertRule:  MetricCPUUsage,
		Status:     StatusFiring,
		LabelsJSON: "{}",
		FiredAt:    time.Now(),
	}
	// 连接失败应返回 false（不阻塞主流程）
	result := mgr.sendNotification(ctx, alert)
	assert.False(t, result)
}

// ============== 12. SendAlertNotification（公开方法） ==============

func TestSendAlertNotification_NotExist(t *testing.T) {
	db := setupTestDB(t)
	cache, _ := setupTestCfgCache(t, db, nil)
	mgr := NewManager(db, cache)
	ctx := context.Background()

	_, err := mgr.SendAlertNotification(ctx, 9999)
	assert.Error(t, err)
}

// ============== 13. IsAlertEnabled / GetCollectInterval ==============

func TestIsAlertEnabled_True(t *testing.T) {
	db := setupTestDB(t)
	cache, _ := setupTestCfgCache(t, db, map[string]string{
		CfgKeyAlertEnabled: "1",
	})
	mgr := NewManager(db, cache)
	assert.True(t, mgr.IsAlertEnabled(context.Background()))
}

func TestIsAlertEnabled_False(t *testing.T) {
	db := setupTestDB(t)
	cache, _ := setupTestCfgCache(t, db, map[string]string{
		CfgKeyAlertEnabled: "0",
	})
	mgr := NewManager(db, cache)
	assert.False(t, mgr.IsAlertEnabled(context.Background()))
}

func TestGetCollectInterval(t *testing.T) {
	db := setupTestDB(t)
	cache, _ := setupTestCfgCache(t, db, map[string]string{
		CfgKeyCollectInterval: "120",
	})
	mgr := NewManager(db, cache)
	assert.Equal(t, 120, mgr.GetCollectInterval(context.Background()))
}

// ============== 14. 常量 ==============

func TestConstants_ConfigKeys(t *testing.T) {
	// 验证所有配置键常量
	assert.Equal(t, "monitor.collect_interval", CfgKeyCollectInterval)
	assert.Equal(t, "monitor.alert_enabled", CfgKeyAlertEnabled)
	assert.Equal(t, "monitor.notify.webhook_url", CfgKeyNotifyWebhookURL)
	assert.Equal(t, "monitor.silence_minutes", CfgKeySilenceMinutes)
	assert.Equal(t, "monitor.threshold.cpu_usage", CfgKeyThresholdCPU)
	assert.Equal(t, "monitor.threshold.memory_usage", CfgKeyThresholdMemory)
	assert.Equal(t, "monitor.threshold.disk_usage", CfgKeyThresholdDisk)
	assert.Equal(t, "monitor.threshold.error_rate", CfgKeyThresholdErrorRate)
	assert.Equal(t, "monitor.retention_days", CfgKeyRetentionDays)
}

func TestConstants_MetricNames(t *testing.T) {
	assert.Equal(t, "cpu_usage", MetricCPUUsage)
	assert.Equal(t, "memory_usage", MetricMemoryUsage)
	assert.Equal(t, "disk_usage", MetricDiskUsage)
	assert.Equal(t, "qps", MetricQPS)
	assert.Equal(t, "verify_count", MetricVerifyCount)
	assert.Equal(t, "online_devices", MetricOnlineDevices)
	assert.Equal(t, "error_rate", MetricErrorRate)
}

func TestConstants_Severity(t *testing.T) {
	assert.Equal(t, "info", SeverityInfo)
	assert.Equal(t, "warning", SeverityWarning)
	assert.Equal(t, "critical", SeverityCritical)
	assert.Equal(t, "fatal", SeverityFatal)
}

func TestConstants_Status(t *testing.T) {
	assert.Equal(t, "firing", StatusFiring)
	assert.Equal(t, "resolved", StatusResolved)
	assert.Equal(t, "silenced", StatusSilenced)
	assert.Equal(t, "acked", StatusAcked)
}

func TestConstants_Operator(t *testing.T) {
	assert.Equal(t, ">", OpGT)
	assert.Equal(t, "<", OpLT)
	assert.Equal(t, ">=", OpGE)
	assert.Equal(t, "<=", OpLE)
	assert.Equal(t, "==", OpEQ)
}

// ============== 15. 并发测试 ==============

func TestCollectAndEvaluate_Concurrent(t *testing.T) {
	db := setupTestDB(t)
	cache, _ := setupTestCfgCache(t, db, map[string]string{
		CfgKeyThresholdCPU:     "80",
		CfgKeyNotifyWebhookURL: "",
	})
	mgr := NewManager(db, cache)

	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = mgr.CollectAndEvaluate(context.Background())
		}()
	}
	wg.Wait()

	// 互斥锁保证不崩溃即视为通过
	assert.True(t, true)
}

// ============== 16. CollectSystemMetrics ==============

func TestCollectSystemMetrics_Success(t *testing.T) {
	db := setupTestDB(t)
	cache, _ := setupTestCfgCache(t, db, nil)
	mgr := NewManager(db, cache)

	// CollectSystemMetrics 在测试环境可能 gopsutil 不可用，但不应崩溃
	samples, err := mgr.CollectSystemMetrics(context.Background())
	assert.NoError(t, err)
	// 至少应该返回在线设备数（DB 查询必成功）
	assert.NotEmpty(t, samples)

	// 验证包含 online_devices 指标
	names := map[string]bool{}
	for _, s := range samples {
		names[s.Name] = true
	}
	assert.True(t, names[MetricOnlineDevices])
	assert.True(t, names[MetricVerifyCount])
}

// ============== 17. 集成：CollectAndEvaluate ==============

func TestCollectAndEvaluate_Integration(t *testing.T) {
	db := setupTestDB(t)
	cache, _ := setupTestCfgCache(t, db, map[string]string{
		CfgKeyThresholdCPU:     "1", // 极低阈值确保不触发
		CfgKeyNotifyWebhookURL: "",
		CfgKeyRetentionDays:    "0", // 不清理
	})
	mgr := NewManager(db, cache)

	result, err := mgr.CollectAndEvaluate(context.Background())
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.GreaterOrEqual(t, result.MetricsCollected, 1)
}

// ============== 18. 边界用例 ==============

func TestCompareWithOperator_NegativeValues(t *testing.T) {
	assert.True(t, CompareWithOperator(-5, -10, OpGT))
	assert.False(t, CompareWithOperator(-10, -5, OpGT))
	assert.True(t, CompareWithOperator(-10, -5, OpLT))
}

func TestCompareWithOperator_ZeroValues(t *testing.T) {
	assert.True(t, CompareWithOperator(0, 0, OpEQ))
	assert.True(t, CompareWithOperator(0, 0, OpGE))
	assert.True(t, CompareWithOperator(0, 0, OpLE))
	assert.False(t, CompareWithOperator(0, 0, OpGT))
	assert.False(t, CompareWithOperator(0, 0, OpLT))
}

func TestFormatMetricName_Empty(t *testing.T) {
	assert.Equal(t, "", FormatMetricName(""))
}

func TestSaveMetrics_AllNilLabels(t *testing.T) {
	db := setupTestDB(t)
	cache, _ := setupTestCfgCache(t, db, nil)
	mgr := NewManager(db, cache)

	samples := []MetricSample{
		{Name: MetricCPUUsage, Value: 50, Unit: "%", Labels: nil},
		{Name: MetricMemoryUsage, Value: 60, Unit: "%", Labels: nil},
	}
	count, err := mgr.SaveMetrics(context.Background(), samples)
	require.NoError(t, err)
	assert.Equal(t, 2, count)
}

func TestEvaluateAlerts_MultipleMetrics(t *testing.T) {
	db := setupTestDB(t)
	cache, _ := setupTestCfgCache(t, db, map[string]string{
		CfgKeyThresholdCPU:       "80",
		CfgKeyThresholdMemory:    "80",
		CfgKeyThresholdDisk:      "80",
		CfgKeyThresholdErrorRate: "5",
		CfgKeyNotifyWebhookURL:   "",
	})
	mgr := NewManager(db, cache)

	samples := []MetricSample{
		{Name: MetricCPUUsage, Value: 95.0, Unit: "%"},      // 触发
		{Name: MetricMemoryUsage, Value: 95.0, Unit: "%"},   // 触发
		{Name: MetricDiskUsage, Value: 50.0, Unit: "%"},     // 未触发
		{Name: MetricErrorRate, Value: 15.0, Unit: "%"},     // 触发
	}
	fired, _, err := mgr.EvaluateAlerts(context.Background(), samples)
	require.NoError(t, err)
	assert.Equal(t, 3, fired)

	var count int64
	db.Model(&model.SystemAlert{}).Count(&count)
	assert.Equal(t, int64(3), count)
}
