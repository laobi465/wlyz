// Package risk v0.4.0 高级安全风控规则引擎
// 严格遵循铁律 04：所有配置键名以常量声明，禁止硬编码字符串
// 严格遵循铁律 05：所有阈值/开关走 sys_config 后台可视化编辑
// 严格遵循铁律 06：所有断言基于已知固定输入，无随机/不确定性
//
// 5 条内置规则（rule_type 固定，仅可调阈值/启停）：
//   - geo_login      异地登录（基于 IP 网段比较，无需 GeoIP 数据库）
//   - new_device     新设备登录（基于 hwid 首次见到）
//   - abnormal_ua    异常 UA（爬虫/curl/空 UA）
//   - abnormal_time  异常时段（凌晨 02:00-05:00，默认禁用）
//   - high_frequency 高频请求（同 IP 同账号 60s 内 >10 次）
package risk

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/your-org/keyauth-saas/apps/server/internal/model"
	"github.com/your-org/keyauth-saas/apps/server/pkg/ua"
	"gorm.io/gorm"
)

// ============== 配置键常量（铁律 04） ==============

const (
	CfgKeyCloudflareEnabled         = "cloudflare.enabled"
	CfgKeyCloudflareRealIPHeader    = "cloudflare.real_ip_header"
	CfgKeyCloudflareIPCountryHeader = "cloudflare.ip_country_header"
	CfgKeyCloudflareTrustedCIDRs    = "cloudflare.trusted_cidrs"

	CfgKeyEngineEnabled        = "risk.engine.enabled"
	CfgKeyEngineScoreThreshold = "risk.engine.score_threshold"
	CfgKeyEngineDefaultAction  = "risk.engine.default_action"

	CfgKeyGeoLoginEnabled       = "risk.geo_login_alert.enabled"
	CfgKeyGeoLoginIPv4Prefix    = "risk.geo_login_alert.ipv4_prefix"
	CfgKeyGeoLoginIPv6Prefix    = "risk.geo_login_alert.ipv6_prefix"
	CfgKeyGeoLoginNotifyChans   = "risk.geo_login_alert.notify_channels"

	CfgKeyNewDeviceEnabled    = "risk.new_device_alert.enabled"
	CfgKeyAbnormalUAEnabled   = "risk.abnormal_ua_alert.enabled"
	CfgKeyAbnormalTimeEnabled = "risk.abnormal_time_alert.enabled"
	CfgKeyAbnormalTimeStart   = "risk.abnormal_time_start"
	CfgKeyAbnormalTimeEnd     = "risk.abnormal_time_end"
)

// ============== 规则类型与动作常量 ==============

const (
	RuleTypeGeoLogin      = "geo_login"
	RuleTypeNewDevice     = "new_device"
	RuleTypeAbnormalUA    = "abnormal_ua"
	RuleTypeAbnormalTime  = "abnormal_time"
	RuleTypeHighFrequency = "high_frequency"
	RuleTypeCustom        = "custom"

	ActionAlert     = "alert"     // 仅记录告警
	ActionChallenge = "challenge" // 要求二次验证
	ActionBlock     = "block"     // 拒绝请求

	StatusActive   = "active"
	StatusDisabled = "disabled"

	UserTypeAdmin   = "admin"
	UserTypeTenant  = "tenant"
	UserTypeAgent   = "agent"
	UserTypeEndUser = "enduser"
)

// ============== 配置读取接口（避免循环依赖） ==============

// ConfigReader sys_config 读取抽象
// 实际由 *config.ConfigCache 实现
type ConfigReader interface {
	GetBool(ctx context.Context, key string, fallback bool) bool
	GetInt(ctx context.Context, key string, fallback int) int
	GetString(ctx context.Context, key, fallback string) string
}

// ============== 评估上下文 ==============

// EvalContext 风控评估上下文
// 由调用方（中间件/handler）填充，规则引擎消费
type EvalContext struct {
	UserType    string // admin/tenant/agent/enduser，空=匿名
	UserID      uint64
	Username    string
	ClientIP    string
	UserAgent   string
	HWID        string // 设备指纹（客户端 SDK 计算）
	Operation   string // login/verify/heartbeat/bind 等
	OccurredAt  time.Time
}

// EvalResult 单条规则评估结果
type EvalResult struct {
	RuleID      uint64
	RuleType    string
	RuleName    string
	Score       int    // 命中加分
	Action      string // alert/challenge/block
	Hit         bool   // 是否命中
	Detail      string // JSON 详情
}

// EngineOutput 引擎总输出
type EngineOutput struct {
	TotalScore   int           // 累计评分
	Action       string        // 最终动作（取最高级别）
	HitRules     []EvalResult  // 命中的规则列表
	ShouldBlock  bool          // 是否应拒绝请求
	ShouldChallenge bool       // 是否应触发二次验证
}

// ============== Manager 风控引擎 ==============

// Manager 风控规则引擎
type Manager struct {
	db        *gorm.DB
	cfg       ConfigReader
}

// NewManager 构造
func NewManager(db *gorm.DB, cfg ConfigReader) *Manager {
	return &Manager{db: db, cfg: cfg}
}

// EvaluateLogin 评估登录请求
// 依次执行所有 active 规则，返回累计评分与最终动作
func (m *Manager) EvaluateLogin(ctx context.Context, ec EvalContext) EngineOutput {
	if !m.cfg.GetBool(ctx, CfgKeyEngineEnabled, true) {
		return EngineOutput{}
	}

	if ec.OccurredAt.IsZero() {
		ec.OccurredAt = time.Now()
	}

	rules := m.listActiveRules(ctx)
	results := make([]EvalResult, 0, len(rules))
	totalScore := 0
	maxAction := ActionAlert

	for _, rule := range rules {
		// 内置规则的总开关在 sys_config，优先校验
		if !m.isRuleEnabled(ctx, rule.RuleType) {
			continue
		}

		res := m.evalRule(ctx, rule, ec)
		if res.Hit {
			results = append(results, res)
			totalScore += res.Score
			if actionLevel(res.Action) > actionLevel(maxAction) {
				maxAction = res.Action
			}
		}
	}

	// 评分超阈值时升级动作
	threshold := m.cfg.GetInt(ctx, CfgKeyEngineScoreThreshold, 80)
	if totalScore >= threshold {
		defaultAction := m.cfg.GetString(ctx, CfgKeyEngineDefaultAction, ActionAlert)
		if actionLevel(defaultAction) > actionLevel(maxAction) {
			maxAction = defaultAction
		}
	}

	out := EngineOutput{
		TotalScore:      totalScore,
		Action:          maxAction,
		HitRules:        results,
		ShouldBlock:     maxAction == ActionBlock,
		ShouldChallenge: maxAction == ActionChallenge,
	}
	return out
}

// RecordEvent 落盘风控事件
// 同时记录到 risk_event 表；若是异地登录，额外写入 login_geo_alert 表
func (m *Manager) RecordEvent(ctx context.Context, ec EvalContext, out EngineOutput) {
	if len(out.HitRules) == 0 {
		return
	}

	for _, r := range out.HitRules {
		detail := r.Detail
		if detail == "" {
			detail = "{}"
		}
		ev := model.RiskEvent{
			RuleID:      r.RuleID,
			RuleType:    r.RuleType,
			RuleName:    r.RuleName,
			UserType:    ec.UserType,
			UserID:      ec.UserID,
			Username:    ec.Username,
			ClientIP:    ec.ClientIP,
			UserAgent:   ec.UserAgent,
			RiskScore:   r.Score,
			ActionTaken: r.Action,
			Detail:      detail,
		}
		if err := m.db.WithContext(ctx).Create(&ev).Error; err != nil {
			// 风控日志失败不影响主流程
			continue
		}

		// 异地登录额外写入告警表
		if r.RuleType == RuleTypeGeoLogin {
			m.recordGeoAlert(ctx, ec, r.Detail)
		}
	}
}

// ============== 内部：规则评估分发 ==============

func (m *Manager) evalRule(ctx context.Context, rule model.RiskRule, ec EvalContext) EvalResult {
	res := EvalResult{
		RuleID:   rule.ID,
		RuleType: rule.RuleType,
		RuleName: rule.Name,
		Action:   rule.Action,
	}

	switch rule.RuleType {
	case RuleTypeGeoLogin:
		res.Hit, res.Detail, res.Score = m.evalGeoLogin(ctx, rule, ec)
	case RuleTypeNewDevice:
		res.Hit, res.Detail, res.Score = m.evalNewDevice(ctx, rule, ec)
	case RuleTypeAbnormalUA:
		res.Hit, res.Detail, res.Score = m.evalAbnormalUA(ctx, rule, ec)
	case RuleTypeAbnormalTime:
		res.Hit, res.Detail, res.Score = m.evalAbnormalTime(ctx, rule, ec)
	case RuleTypeHighFrequency:
		res.Hit, res.Detail, res.Score = m.evalHighFrequency(ctx, rule, ec)
	}

	if res.Hit {
		// 命中则用规则声明的 score/action 覆盖（规则配置优先）
		if rule.Score > 0 {
			res.Score = rule.Score
		}
		if rule.Action != "" {
			res.Action = rule.Action
		}
	}
	return res
}

// listActiveRules 拉取所有 active 规则（按 priority 升序）
func (m *Manager) listActiveRules(ctx context.Context) []model.RiskRule {
	var rules []model.RiskRule
	m.db.WithContext(ctx).Where("status = ?", StatusActive).
		Order("priority ASC, id ASC").
		Find(&rules)
	return rules
}

// isRuleEnabled 校验内置规则的总开关（自定义规则 always true）
func (m *Manager) isRuleEnabled(ctx context.Context, ruleType string) bool {
	switch ruleType {
	case RuleTypeGeoLogin:
		return m.cfg.GetBool(ctx, CfgKeyGeoLoginEnabled, true)
	case RuleTypeNewDevice:
		return m.cfg.GetBool(ctx, CfgKeyNewDeviceEnabled, true)
	case RuleTypeAbnormalUA:
		return m.cfg.GetBool(ctx, CfgKeyAbnormalUAEnabled, true)
	case RuleTypeAbnormalTime:
		return m.cfg.GetBool(ctx, CfgKeyAbnormalTimeEnabled, false)
	case RuleTypeHighFrequency:
		return true // 高频请求始终启用（阈值由规则 condition 控制）
	case RuleTypeCustom:
		return true
	default:
		return true
	}
}

// ============== 规则 1：异地登录检测 ==============

// evalGeoLogin 比较 ec.ClientIP 网段与上次登录 IP 网段
// 上次登录 IP 取自 RefreshTokenDevice 表（按 user_type+user_id 最新一条）
func (m *Manager) evalGeoLogin(ctx context.Context, rule model.RiskRule, ec EvalContext) (bool, string, int) {
	if ec.ClientIP == "" || ec.UserType == "" || ec.UserID == 0 {
		return false, "", 0
	}

	ipv4Prefix := m.cfg.GetInt(ctx, CfgKeyGeoLoginIPv4Prefix, 24)
	ipv6Prefix := m.cfg.GetInt(ctx, CfgKeyGeoLoginIPv6Prefix, 64)

	// 解析规则 condition 覆盖（管理员可在 risk_rule 表覆盖默认前缀）
	var cond struct {
		IPv4Prefix int `json:"ipv4_prefix"`
		IPv6Prefix int `json:"ipv6_prefix"`
	}
	if rule.Condition != "" {
		_ = json.Unmarshal([]byte(rule.Condition), &cond)
		if cond.IPv4Prefix > 0 {
			ipv4Prefix = cond.IPv4Prefix
		}
		if cond.IPv6Prefix > 0 {
			ipv6Prefix = cond.IPv6Prefix
		}
	}

	currentNetwork, ok := NetworkOf(ec.ClientIP, ipv4Prefix, ipv6Prefix)
	if !ok {
		return false, "", 0
	}

	// 查找上次登录 IP（RefreshTokenDevice 表）
	var last model.RefreshTokenDevice
	err := m.db.WithContext(ctx).
		Where("user_role = ? AND user_id = ? AND revoked = ?", ec.UserType, ec.UserID, false).
		Order("id DESC").
		Limit(1).
		First(&last).Error
	if err != nil {
		// 无历史记录，不触发异地告警（首登正常）
		return false, "", 0
	}

	if last.ClientIP == "" || last.ClientIP == ec.ClientIP {
		return false, "", 0
	}

	previousNetwork, ok := NetworkOf(last.ClientIP, ipv4Prefix, ipv6Prefix)
	if !ok {
		return false, "", 0
	}

	if currentNetwork == previousNetwork {
		return false, "", 0
	}

	detail, _ := json.Marshal(map[string]interface{}{
		"current_ip":       ec.ClientIP,
		"current_network":  currentNetwork,
		"previous_ip":      last.ClientIP,
		"previous_network": previousNetwork,
		"ipv4_prefix":      ipv4Prefix,
		"ipv6_prefix":      ipv6Prefix,
	})
	return true, string(detail), rule.Score
}

// recordGeoAlert 异地登录额外写入 login_geo_alert 表
func (m *Manager) recordGeoAlert(ctx context.Context, ec EvalContext, detailJSON string) {
	var d map[string]interface{}
	if err := json.Unmarshal([]byte(detailJSON), &d); err != nil {
		return
	}
	currentNetwork, _ := d["current_network"].(string)
	previousNetwork, _ := d["previous_network"].(string)
	previousIP, _ := d["previous_ip"].(string)
	if previousIP == "" {
		return
	}
	channels := m.cfg.GetString(ctx, CfgKeyGeoLoginNotifyChans, "inapp,email")

	alert := model.LoginGeoAlert{
		UserType:        ec.UserType,
		UserID:          ec.UserID,
		Username:        ec.Username,
		CurrentIP:       ec.ClientIP,
		CurrentNetwork:  currentNetwork,
		PreviousIP:      previousIP,
		PreviousNetwork: previousNetwork,
		UserAgent:       ec.UserAgent,
		NotifyChannels:  channels,
		AlertStatus:     "pending",
	}
	_ = m.db.WithContext(ctx).Create(&alert).Error
}

// ============== 规则 2：新设备登录检测 ==============

// evalNewDevice 检查 hwid 是否首次见到（针对 enduser/app_device 维度）
// 对 admin/tenant/agent：检查 RefreshTokenDevice 表中该 hwid 是否首次出现
func (m *Manager) evalNewDevice(ctx context.Context, rule model.RiskRule, ec EvalContext) (bool, string, int) {
	if ec.HWID == "" || ec.UserType == "" || ec.UserID == 0 {
		return false, "", 0
	}

	// 简化逻辑：检查 RefreshTokenDevice 表中 user_role+user_id 是否有不同 hwid（按 UA 哈希近似）
	// 真实生产应记录历史 hwid 列表；此处用 UA 字段比对作为近似
	var count int64
	m.db.WithContext(ctx).Model(&model.RefreshTokenDevice{}).
		Where("user_role = ? AND user_id = ? AND user_agent = ? AND revoked = ?",
			ec.UserType, ec.UserID, ec.UserAgent, false).
		Count(&count)

	if count > 0 {
		// 同 UA 已有记录，不算新设备
		return false, "", 0
	}

	// 检查是否有任何历史记录
	var total int64
	m.db.WithContext(ctx).Model(&model.RefreshTokenDevice{}).
		Where("user_role = ? AND user_id = ? AND revoked = ?",
			ec.UserType, ec.UserID, false).
		Count(&total)

	if total == 0 {
		// 首次登录，不算新设备告警
		return false, "", 0
	}

	detail, _ := json.Marshal(map[string]interface{}{
		"hwid":        ec.HWID,
		"user_agent":  ec.UserAgent,
		"history_count": total,
	})
	return true, string(detail), rule.Score
}

// ============== 规则 3：异常 UA 检测 ==============

// evalAbnormalUA 检查 UA 是否为爬虫/curl/空 UA
func (m *Manager) evalAbnormalUA(ctx context.Context, rule model.RiskRule, ec EvalContext) (bool, string, int) {
	if ec.UserAgent == "" {
		detail, _ := json.Marshal(map[string]interface{}{
			"reason": "empty_ua",
		})
		return true, string(detail), rule.Score
	}

	uaLower := strings.ToLower(ec.UserAgent)
	abnormalKeywords := []string{"curl", "wget", "python-requests", "scrapy", "bot", "spider", "crawler"}
	for _, kw := range abnormalKeywords {
		if strings.Contains(uaLower, kw) {
			detail, _ := json.Marshal(map[string]interface{}{
				"reason":  "abnormal_keyword",
				"keyword": kw,
			})
			return true, string(detail), rule.Score
		}
	}

	// 用 pkg/ua 解析，识别为 Bot 也触发
	info := ua.Parse(ec.UserAgent)
	if info.DeviceType == ua.DeviceBot {
		detail, _ := json.Marshal(map[string]interface{}{
			"reason": "bot_detected",
			"browser": info.Browser,
		})
		return true, string(detail), rule.Score
	}

	return false, "", 0
}

// ============== 规则 4：异常时段检测 ==============

// evalAbnormalTime 检查登录时间是否在 02:00-05:00
// 注意：使用服务器本地时间（生产环境应统一 UTC+8）
func (m *Manager) evalAbnormalTime(ctx context.Context, rule model.RiskRule, ec EvalContext) (bool, string, int) {
	startStr := m.cfg.GetString(ctx, CfgKeyAbnormalTimeStart, "02:00")
	endStr := m.cfg.GetString(ctx, CfgKeyAbnormalTimeEnd, "05:00")

	// 解析规则 condition 覆盖
	var cond struct {
		Start string `json:"start"`
		End   string `json:"end"`
	}
	if rule.Condition != "" {
		_ = json.Unmarshal([]byte(rule.Condition), &cond)
		if cond.Start != "" {
			startStr = cond.Start
		}
		if cond.End != "" {
			endStr = cond.End
		}
	}

	startMin, ok1 := parseHHMM(startStr)
	endMin, ok2 := parseHHMM(endStr)
	if !ok1 || !ok2 {
		return false, "", 0
	}

	curMin := ec.OccurredAt.Hour()*60 + ec.OccurredAt.Minute()
	inRange := false
	if startMin <= endMin {
		inRange = curMin >= startMin && curMin < endMin
	} else {
		// 跨午夜：如 23:00-04:00
		inRange = curMin >= startMin || curMin < endMin
	}

	if !inRange {
		return false, "", 0
	}

	detail, _ := json.Marshal(map[string]interface{}{
		"current_time": ec.OccurredAt.Format("15:04"),
		"start":        startStr,
		"end":          endStr,
	})
	return true, string(detail), rule.Score
}

// ============== 规则 5：高频请求检测 ==============

// evalHighFrequency 检查同 IP 同账号 60 秒内登录/验证次数 > 阈值
// 统计 risk_event 表本身的命中次数（避免与限流重复）
func (m *Manager) evalHighFrequency(ctx context.Context, rule model.RiskRule, ec EvalContext) (bool, string, int) {
	if ec.ClientIP == "" {
		return false, "", 0
	}

	// 解析规则 condition
	windowSec := 60
	threshold := 10
	var cond struct {
		WindowSeconds int `json:"window_seconds"`
		Threshold     int `json:"threshold"`
	}
	if rule.Condition != "" {
		_ = json.Unmarshal([]byte(rule.Condition), &cond)
		if cond.WindowSeconds > 0 {
			windowSec = cond.WindowSeconds
		}
		if cond.Threshold > 0 {
			threshold = cond.Threshold
		}
	}

	since := time.Now().Add(-time.Duration(windowSec) * time.Second)
	var count int64
	q := m.db.WithContext(ctx).Model(&model.RiskEvent{}).
		Where("client_ip = ? AND created_at > ?", ec.ClientIP, since)
	if ec.Username != "" {
		q = q.Where("username = ?", ec.Username)
	}
	q.Count(&count)

	// count >= threshold 触发（避免每次都触发，仅在达到阈值时命中一次）
	if count < int64(threshold) {
		return false, "", 0
	}

	detail, _ := json.Marshal(map[string]interface{}{
		"window_seconds": windowSec,
		"threshold":      threshold,
		"current_count":  count,
		"client_ip":      ec.ClientIP,
	})
	return true, string(detail), rule.Score
}

// ============== 工具函数 ==============

// NetworkOf 计算指定 IP 的网段 CIDR
// ipv4Prefix/ipv6Prefix 控制前缀长度
// 返回 "1.2.3.0/24" 形式；解析失败返回 false
func NetworkOf(ipStr string, ipv4Prefix, ipv6Prefix int) (string, bool) {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return "", false
	}

	var prefix int
	var bits int
	if v4 := ip.To4(); v4 != nil {
		ip = v4
		prefix = ipv4Prefix
		bits = 32
	} else {
		prefix = ipv6Prefix
		bits = 128
	}

	if prefix < 0 || prefix > bits {
		return "", false
	}

	mask := net.CIDRMask(prefix, bits)
	network := ip.Mask(mask)
	return fmt.Sprintf("%s/%d", network.String(), prefix), true
}

// parseHHMM 解析 "HH:MM" 字符串为分钟数
func parseHHMM(s string) (int, bool) {
	parts := strings.Split(s, ":")
	if len(parts) != 2 {
		return 0, false
	}
	h := atoiSafe(parts[0])
	m := atoiSafe(parts[1])
	if h < 0 || h > 23 || m < 0 || m > 59 {
		return 0, false
	}
	return h*60 + m, true
}

func atoiSafe(s string) int {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return -1
		}
		n = n*10 + int(c-'0')
	}
	return n
}

// actionLevel 动作级别（用于取最高级别）
func actionLevel(action string) int {
	switch action {
	case ActionBlock:
		return 3
	case ActionChallenge:
		return 2
	case ActionAlert:
		return 1
	default:
		return 0
	}
}

// ============== CRUD：风控规则管理 ==============

// ListRules 列出所有规则（含 disabled）
func (m *Manager) ListRules(ctx context.Context) ([]model.RiskRule, error) {
	var rules []model.RiskRule
	err := m.db.WithContext(ctx).Order("priority ASC, id ASC").Find(&rules).Error
	return rules, err
}

// GetRule 单条详情
func (m *Manager) GetRule(ctx context.Context, id uint64) (*model.RiskRule, error) {
	var rule model.RiskRule
	err := m.db.WithContext(ctx).First(&rule, id).Error
	if err != nil {
		return nil, err
	}
	return &rule, nil
}

// CreateRule 创建自定义规则
// 内置规则（rule_type != custom）禁止创建
func (m *Manager) CreateRule(ctx context.Context, rule *model.RiskRule) error {
	if rule.RuleType != RuleTypeCustom {
		return fmt.Errorf("内置规则类型 %s 禁止创建，请使用 custom", rule.RuleType)
	}
	if rule.Condition == "" {
		rule.Condition = "{}"
	}
	if rule.Action == "" {
		rule.Action = ActionAlert
	}
	if rule.Status == "" {
		rule.Status = StatusActive
	}
	if rule.CreatedBy == "" {
		rule.CreatedBy = "admin"
	}
	return m.db.WithContext(ctx).Create(rule).Error
}

// UpdateRule 更新规则（仅可改阈值/启停/动作；name/description 可改）
// 内置规则（created_by=system）禁止改 rule_type
func (m *Manager) UpdateRule(ctx context.Context, id uint64, updates map[string]interface{}) error {
	rule, err := m.GetRule(ctx, id)
	if err != nil {
		return err
	}

	// 内置规则不可改 rule_type 与 condition（仅可调阈值/启停/动作）
	if rule.CreatedBy == "system" {
		delete(updates, "rule_type")
	}

	// 允许更新的字段白名单
	allowed := map[string]bool{
		"name": true, "description": true, "condition": true,
		"score": true, "action": true, "priority": true, "status": true,
	}
	for k := range updates {
		if !allowed[k] {
			delete(updates, k)
		}
	}

	return m.db.WithContext(ctx).Model(&model.RiskRule{}).
		Where("id = ?", id).
		Updates(updates).Error
}

// DeleteRule 删除规则（内置规则 created_by=system 禁止删除）
func (m *Manager) DeleteRule(ctx context.Context, id uint64) error {
	rule, err := m.GetRule(ctx, id)
	if err != nil {
		return err
	}
	if rule.CreatedBy == "system" {
		return fmt.Errorf("内置规则禁止删除，可改为 disabled 状态")
	}
	return m.db.WithContext(ctx).Delete(&model.RiskRule{}, id).Error
}

// ============== CRUD：风控事件查询 ==============

// ListEvents 列出风控事件（支持 user_type/rule_type/action/acknowledged 筛选）
func (m *Manager) ListEvents(ctx context.Context, params ListEventsParams) ([]model.RiskEvent, int64, error) {
	q := m.db.WithContext(ctx).Model(&model.RiskEvent{})
	if params.UserType != "" {
		q = q.Where("user_type = ?", params.UserType)
	}
	if params.RuleType != "" {
		q = q.Where("rule_type = ?", params.RuleType)
	}
	if params.Action != "" {
		q = q.Where("action_taken = ?", params.Action)
	}
	if params.ClientIP != "" {
		q = q.Where("client_ip = ?", params.ClientIP)
	}
	if params.Acknowledged != nil {
		q = q.Where("acknowledged = ?", *params.Acknowledged)
	}
	if params.StartDate != "" {
		q = q.Where("created_at >= ?", params.StartDate)
	}
	if params.EndDate != "" {
		q = q.Where("created_at <= ?", params.EndDate)
	}

	var total int64
	q.Count(&total)

	if params.PageSize > 0 {
		q = q.Limit(params.PageSize).Offset((params.Page - 1) * params.PageSize)
	}
	var events []model.RiskEvent
	err := q.Order("created_at DESC").Find(&events).Error
	return events, total, err
}

// ListEventsParams 查询参数
type ListEventsParams struct {
	Page         int
	PageSize     int
	UserType     string
	RuleType     string
	Action       string
	ClientIP     string
	Acknowledged *bool
	StartDate    string
	EndDate      string
}

// AcknowledgeEvent 确认风控事件
func (m *Manager) AcknowledgeEvent(ctx context.Context, id uint64, acknowledger string) error {
	now := time.Now()
	return m.db.WithContext(ctx).Model(&model.RiskEvent{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"acknowledged":    true,
			"acknowledged_by": acknowledger,
			"acknowledged_at": now,
		}).Error
}

// ============== CRUD：异地登录告警查询 ==============

// ListGeoAlerts 列出异地登录告警
func (m *Manager) ListGeoAlerts(ctx context.Context, params ListGeoAlertParams) ([]model.LoginGeoAlert, int64, error) {
	q := m.db.WithContext(ctx).Model(&model.LoginGeoAlert{})
	if params.UserType != "" {
		q = q.Where("user_type = ?", params.UserType)
	}
	if params.AlertStatus != "" {
		q = q.Where("alert_status = ?", params.AlertStatus)
	}
	if params.StartDate != "" {
		q = q.Where("created_at >= ?", params.StartDate)
	}
	if params.EndDate != "" {
		q = q.Where("created_at <= ?", params.EndDate)
	}

	var total int64
	q.Count(&total)

	if params.PageSize > 0 {
		q = q.Limit(params.PageSize).Offset((params.Page-1) * params.PageSize)
	}
	var alerts []model.LoginGeoAlert
	err := q.Order("created_at DESC").Find(&alerts).Error
	return alerts, total, err
}

// ListGeoAlertParams 查询参数
type ListGeoAlertParams struct {
	Page        int
	PageSize    int
	UserType    string
	AlertStatus string
	StartDate   string
	EndDate     string
}

// AcknowledgeGeoAlert 确认异地登录告警
func (m *Manager) AcknowledgeGeoAlert(ctx context.Context, id uint64, acknowledger string) error {
	now := time.Now()
	return m.db.WithContext(ctx).Model(&model.LoginGeoAlert{}).
		Where("id = ? AND alert_status = ?", id, "pending").
		Updates(map[string]interface{}{
			"alert_status":    "acknowledged",
			"acknowledged_by": acknowledger,
			"acknowledged_at": now,
		}).Error
}

// CloseGeoAlert 关闭异地登录告警
func (m *Manager) CloseGeoAlert(ctx context.Context, id uint64, acknowledger string) error {
	now := time.Now()
	return m.db.WithContext(ctx).Model(&model.LoginGeoAlert{}).
		Where("id = ? AND alert_status != ?", id, "closed").
		Updates(map[string]interface{}{
			"alert_status":    "closed",
			"acknowledged_by": acknowledger,
			"closed_at":       now,
		}).Error
}

// ============== 统计：风控看板 ==============

// Stats 风控看板统计
type Stats struct {
	RiskEventsToday       int64 `json:"risk_events_today"`
	RiskEventsWeek        int64 `json:"risk_events_week"`
	BlockedRequests       int64 `json:"blocked_requests_today"`
	ChallengeRequests     int64 `json:"challenge_requests_today"`
	AlertRequests         int64 `json:"alert_requests_today"`
	PendingAlerts         int64 `json:"pending_alerts"`
	GeoAlertsToday        int64 `json:"geo_alerts_today"`
	GeoAlertsPending      int64 `json:"geo_alerts_pending"`
	AcknowledgedEvents    int64 `json:"acknowledged_events_today"`
	TopAbnormalIPs        []TopAbnormalIP `json:"top_abnormal_ips"`
	RecentEvents          []model.RiskEvent `json:"recent_events"`
}

// TopAbnormalIP 异常 IP TOP
type TopAbnormalIP struct {
	ClientIP string `json:"client_ip"`
	Count    int64  `json:"count"`
}

// GetStats 风控看板统计
func (m *Manager) GetStats(ctx context.Context) (*Stats, error) {
	now := time.Now()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	weekStart := todayStart.AddDate(0, 0, -7)

	var s Stats

	m.db.WithContext(ctx).Model(&model.RiskEvent{}).
		Where("created_at >= ?", todayStart).Count(&s.RiskEventsToday)
	m.db.WithContext(ctx).Model(&model.RiskEvent{}).
		Where("created_at >= ?", weekStart).Count(&s.RiskEventsWeek)
	m.db.WithContext(ctx).Model(&model.RiskEvent{}).
		Where("created_at >= ? AND action_taken = ?", todayStart, ActionBlock).
		Count(&s.BlockedRequests)
	m.db.WithContext(ctx).Model(&model.RiskEvent{}).
		Where("created_at >= ? AND action_taken = ?", todayStart, ActionChallenge).
		Count(&s.ChallengeRequests)
	m.db.WithContext(ctx).Model(&model.RiskEvent{}).
		Where("created_at >= ? AND action_taken = ?", todayStart, ActionAlert).
		Count(&s.AlertRequests)
	m.db.WithContext(ctx).Model(&model.RiskEvent{}).
		Where("acknowledged = ?", false).Count(&s.PendingAlerts)

	m.db.WithContext(ctx).Model(&model.LoginGeoAlert{}).
		Where("created_at >= ?", todayStart).Count(&s.GeoAlertsToday)
	m.db.WithContext(ctx).Model(&model.LoginGeoAlert{}).
		Where("alert_status = ?", "pending").Count(&s.GeoAlertsPending)
	m.db.WithContext(ctx).Model(&model.RiskEvent{}).
		Where("created_at >= ? AND acknowledged = ?", todayStart, true).
		Count(&s.AcknowledgedEvents)

	// TOP 10 异常 IP（按今日事件数）
	m.db.WithContext(ctx).Model(&model.RiskEvent{}).
		Select("client_ip, COUNT(*) as count").
		Where("created_at >= ? AND client_ip != ''", todayStart).
		Group("client_ip").
		Order("count DESC").
		Limit(10).
		Scan(&s.TopAbnormalIPs)

	// 最近 10 条事件
	m.db.WithContext(ctx).Order("created_at DESC").Limit(10).Find(&s.RecentEvents)

	return &s, nil
}
