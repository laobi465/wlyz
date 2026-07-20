// Package analysis v0.6.0 高级分析模块
//
// 三大子模块：
//   1. 用户行为分析（Behavior）：基于 log_verify 按终端用户 + 日聚合，输出活跃度/失败率/多设备多 IP 等
//   2. 卡密使用画像（CardProfile）：基于 log_verify 按卡密 + 日聚合，输出使用频次/失败率/异常设备等
//   3. 风险用户识别（RiskUser）：综合 risk_event + log_verify 实时异常模式，累计风险评分 + 衰减 + 自动分级
//
// 严格遵循铁律 04：所有配置键名以常量声明，禁止硬编码字符串
// 严格遵循铁律 05：所有阈值/开关/权重走 sys_config 后台可视化编辑
// 严格遵循铁律 06：所有断言基于已知固定输入，无随机/不确定性
package analysis

import (
	"context"
	"time"

	"github.com/your-org/keyauth-saas/apps/server/internal/config"
	"gorm.io/gorm"
)

// ============== 配置键常量（铁律 04） ==============

const (
	// 总开关
	CfgKeyEnabled = "analysis.enabled"

	// 三大模块独立开关
	CfgKeyBehaviorEnabled    = "analysis.behavior.enabled"
	CfgKeyCardProfileEnabled = "analysis.card_profile.enabled"
	CfgKeyRiskScoreEnabled   = "analysis.risk_score.enabled"

	// 风险评分阈值
	CfgKeyRiskHighThreshold     = "analysis.risk_score.high_threshold"
	CfgKeyRiskMediumThreshold   = "analysis.risk_score.medium_threshold"
	CfgKeyRiskCriticalThreshold = "analysis.risk_score.critical_threshold"

	// 聚合参数
	CfgKeyAggregateInterval = "analysis.aggregate_interval_seconds"
	CfgKeyTopN              = "analysis.top_n"
	CfgKeyLookbackDays      = "analysis.lookback_days"

	// 风险评分权重（7 项）
	CfgKeyWeightHighFreq      = "analysis.risk_score.weight.high_freq"
	CfgKeyWeightGeoAnomaly    = "analysis.risk_score.weight.geo_anomaly"
	CfgKeyWeightNewDevice     = "analysis.risk_score.weight.new_device"
	CfgKeyWeightAbnormalUA    = "analysis.risk_score.weight.abnormal_ua"
	CfgKeyWeightFailRateHigh  = "analysis.risk_score.weight.fail_rate_high"
	CfgKeyWeightMultiIP       = "analysis.risk_score.weight.multi_ip"
	CfgKeyWeightMultiDev      = "analysis.risk_score.weight.multi_dev"

	// 异常模式阈值
	CfgKeyThresholdFailRate     = "analysis.risk_score.threshold.fail_rate"
	CfgKeyThresholdMultiIPCount = "analysis.risk_score.threshold.multi_ip_count"
	CfgKeyThresholdMultiDevCount = "analysis.risk_score.threshold.multi_dev_count"
	CfgKeyDecayDays             = "analysis.risk_score.decay_days"
)

// ============== 风险等级常量 ==============

const (
	RiskLevelLow      = "low"
	RiskLevelMedium   = "medium"
	RiskLevelHigh     = "high"
	RiskLevelCritical = "critical"
)

// 用户类型常量（与 risk 包保持一致）
const (
	UserTypeAdmin   = "admin"
	UserTypeTenant  = "tenant"
	UserTypeAgent   = "agent"
	UserTypeEndUser = "enduser"
)

// ============== 通用过滤/查询参数 ==============

// Filter 列表查询通用过滤参数
type Filter struct {
	TenantID uint64 // 0=不限
	AppID    uint64 // 0=不限
	UserType string // 空串=不限（仅风险用户列表使用）
	Level    string // 空串=不限（仅风险用户列表使用）
	Page     int
	PageSize int
}

// normalizePage 规范化分页参数
func normalizePage(page, pageSize int) (int, int) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	return page, pageSize
}

// ============== Manager ==============

// Manager 高级分析管理器
// 持有 db + cfg，提供三大子模块的方法
type Manager struct {
	db  *gorm.DB
	cfg *config.ConfigCache
}

// NewManager 构造
func NewManager(db *gorm.DB, cfg *config.ConfigCache) *Manager {
	return &Manager{db: db, cfg: cfg}
}

// ============== 公共工具函数 ==============

// CalcRiskLevel 根据评分计算风险等级
// 阈值从 sys_config 读取，支持后台动态调整
//   0 ~ medium_threshold-1 → low
//   medium_threshold ~ high_threshold-1 → medium
//   high_threshold ~ critical_threshold-1 → high
//   >= critical_threshold → critical
func (m *Manager) CalcRiskLevel(ctx context.Context, score int) string {
	medium := m.cfg.GetInt(ctx, CfgKeyRiskMediumThreshold, 40)
	high := m.cfg.GetInt(ctx, CfgKeyRiskHighThreshold, 70)
	critical := m.cfg.GetInt(ctx, CfgKeyRiskCriticalThreshold, 100)
	if score >= critical {
		return RiskLevelCritical
	}
	if score >= high {
		return RiskLevelHigh
	}
	if score >= medium {
		return RiskLevelMedium
	}
	return RiskLevelLow
}

// DecayScore 风险评分衰减
// 算法：score = score * (1 - daysSinceLastEvent / decayDays)
// 当 daysSinceLastEvent >= decayDays 时，score 衰减为 0
// 衰减后不会变成负数
func DecayScore(score int, daysSinceLastEvent int, decayDays int) int {
	if decayDays <= 0 || score <= 0 {
		return score
	}
	if daysSinceLastEvent >= decayDays {
		return 0
	}
	// 整数衰减：每天衰减 score/decayDays
	decay := score * daysSinceLastEvent / decayDays
	result := score - decay
	if result < 0 {
		return 0
	}
	return result
}

// statDateStr 返回 YYYY-MM-DD 格式
func statDateStr(t time.Time) string {
	return t.Format("2006-01-02")
}

// statDateRange 返回 [start, end] 闭区间的日期字符串切片
// 用于按日聚合时遍历日期范围
func statDateRange(start, end time.Time) []string {
	if end.Before(start) {
		return nil
	}
	var dates []string
	cur := start
	for !cur.After(end) {
		dates = append(dates, statDateStr(cur))
		cur = cur.AddDate(0, 0, 1)
	}
	return dates
}

// parseTimePtr 解析数据库返回的时间字符串（兼容 SQLite/MySQL 差异）
// SQLite 经 MIN/MAX 等聚合函数返回的时间字段为字符串，无法直接 Scan 到 *time.Time
// 此函数兼容多种常见时间格式
func parseTimePtr(s string) *time.Time {
	if s == "" {
		return nil
	}
	for _, layout := range []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02 15:04:05.999999999-07:00",
		"2006-01-02 15:04:05.999999999",
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05.999999999Z",
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05",
	} {
		if t, err := time.Parse(layout, s); err == nil {
			return &t
		}
	}
	return nil
}
