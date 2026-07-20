// v0.6.0 高级分析子模块 3：风险用户识别
//
// 数据源：
//   1. risk_event 表（风控引擎已记录的事件）
//   2. log_verify 表（24h 内异常模式：失败率/多 IP/多设备）
//
// 评分算法：
//   raw_score = Σ(hits × weight)  -- 仅统计最近 decay_days 天
//   decayed_score = DecayScore(raw_score, days_since_last_event, decay_days)
//   risk_level = CalcRiskLevel(decayed_score)
//
// 自动封禁：当 decayed_score >= critical_threshold 时标记 banned=true
//   实际账号封禁由调用方（admin handler / 中间件）根据 banned 标志决定
package analysis

import (
	"context"
	"fmt"
	"time"

	"github.com/your-org/keyauth-saas/apps/server/internal/model"
	"gorm.io/gorm"
)

// ============== 输出结构 ==============

// RiskUserSummary 风险用户列表项
type RiskUserSummary struct {
	UserID         uint64     `json:"user_id"`
	UserType       string     `json:"user_type"`
	Username       string     `json:"username"`
	TenantID       uint64     `json:"tenant_id"`
	AppID          uint64     `json:"app_id"`
	RiskScore      int        `json:"risk_score"`
	RiskLevel      string     `json:"risk_level"`
	EventCount     int        `json:"event_count"`
	HighFreqHits   int        `json:"high_freq_hits"`
	GeoAnomalyHits int        `json:"geo_anomaly_hits"`
	NewDeviceHits  int        `json:"new_device_hits"`
	AbnormalUAHits int        `json:"abnormal_ua_hits"`
	FailRateHigh   int        `json:"fail_rate_high_hits"`
	MultiIPHits    int        `json:"multi_ip_hits"`
	MultiDevHits   int        `json:"multi_dev_hits"`
	Banned         bool       `json:"banned"`
	BannedReason   string     `json:"banned_reason"`
	LastEventAt    *time.Time `json:"last_event_at"`
	LastEvalAt     *time.Time `json:"last_eval_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

// RiskUserDetail 风险用户详情（含最近事件）
type RiskUserDetail struct {
	Summary     RiskUserSummary  `json:"summary"`
	RecentEvents []RiskEventBrief `json:"recent_events"` // 最近 N 条风控事件
}

// RiskEventBrief 风控事件简要
type RiskEventBrief struct {
	ID          uint64    `json:"id"`
	RuleType    string    `json:"rule_type"`
	RuleName    string    `json:"rule_name"`
	RiskScore   int       `json:"risk_score"`
	ActionTaken string    `json:"action_taken"`
	ClientIP    string    `json:"client_ip"`
	Detail      string    `json:"detail"`
	CreatedAt   time.Time `json:"created_at"`
}

// RiskUserOverview 风险用户总览
type RiskUserOverview struct {
	TotalUsers      int64 `json:"total_users"`       // user_risk_score 表总记录数
	HighRiskCount   int64 `json:"high_risk_count"`   // risk_level=high
	CriticalCount   int64 `json:"critical_count"`    // risk_level=critical
	MediumRiskCount int64 `json:"medium_risk_count"` // risk_level=medium
	BannedCount     int64 `json:"banned_count"`      // banned=true
	TotalEvents     int64 `json:"total_events"`      // 最近 30 天风控事件总数
}

// ============== 评分重算 ==============

// reevaluateResult 评分重算中间结果
type reevaluateResult struct {
	RawScore        int
	DecayedScore    int
	RiskLevel       string
	HighFreqHits    int
	GeoAnomalyHits  int
	NewDeviceHits   int
	AbnormalUAHits  int
	FailRateHigh    int
	MultiIPHits     int
	MultiDevHits    int
	EventCount      int
	LastEventAt     *time.Time
	TenantID        uint64
	AppID           uint64
	Username        string
}

// ReevaluateUserRiskScore 重算指定用户的评分
// 步骤：
//  1. 查询 risk_event 最近 decay_days 天的事件，按 rule_type 分组统计
//  2. 对于 enduser，查询 log_verify 最近 24h 异常模式（失败率/多 IP/多设备）
//  3. 计算 raw_score = Σ(hits × weight)
//  4. 应用时间衰减：DecayScore(raw_score, days_since_last_event, decay_days)
//  5. upsert user_risk_score 表
//  6. 当 decayed_score >= critical_threshold 且未 banned 时，标记 banned=true
func (m *Manager) ReevaluateUserRiskScore(ctx context.Context, userType string, userID uint64) (*reevaluateResult, error) {
	decayDays := m.cfg.GetInt(ctx, CfgKeyDecayDays, 7)
	cutoff := time.Now().AddDate(0, 0, -decayDays)

	// 1. 查询 risk_event
	var events []model.RiskEvent
	if err := m.db.WithContext(ctx).
		Where("user_type = ? AND user_id = ? AND created_at >= ?", userType, userID, cutoff).
		Order("created_at DESC").
		Find(&events).Error; err != nil {
		return nil, fmt.Errorf("查询 risk_event 失败: %w", err)
	}

	// 2. 按 rule_type 统计
	r := &reevaluateResult{}
	r.EventCount = len(events)
	var lastEventAt *time.Time
	for _, e := range events {
		switch e.RuleType {
		case "high_frequency":
			r.HighFreqHits++
		case "geo_login":
			r.GeoAnomalyHits++
		case "new_device":
			r.NewDeviceHits++
		case "abnormal_ua":
			r.AbnormalUAHits++
		}
		// RiskEvent 表无 tenant_id 字段，TenantID 由 evaluateEndUserAnomalies 中从 app_card 反查填充
		if e.Username != "" {
			r.Username = e.Username
		}
		if lastEventAt == nil || e.CreatedAt.After(*lastEventAt) {
			t := e.CreatedAt
			lastEventAt = &t
		}
	}
	r.LastEventAt = lastEventAt

	// 3. 对于 enduser，查询 log_verify 24h 异常模式
	if userType == UserTypeEndUser {
		if err := m.evaluateEndUserAnomalies(ctx, userID, r); err != nil {
			return nil, err
		}
	}

	// 4. 计算原始评分
	r.RawScore = r.HighFreqHits*m.cfg.GetInt(ctx, CfgKeyWeightHighFreq, 25) +
		r.GeoAnomalyHits*m.cfg.GetInt(ctx, CfgKeyWeightGeoAnomaly, 20) +
		r.NewDeviceHits*m.cfg.GetInt(ctx, CfgKeyWeightNewDevice, 10) +
		r.AbnormalUAHits*m.cfg.GetInt(ctx, CfgKeyWeightAbnormalUA, 15) +
		r.FailRateHigh*m.cfg.GetInt(ctx, CfgKeyWeightFailRateHigh, 20) +
		r.MultiIPHits*m.cfg.GetInt(ctx, CfgKeyWeightMultiIP, 15) +
		r.MultiDevHits*m.cfg.GetInt(ctx, CfgKeyWeightMultiDev, 15)

	// 5. 应用时间衰减
	daysSinceLast := 0
	if lastEventAt != nil {
		daysSinceLast = int(time.Since(*lastEventAt).Hours() / 24)
	}
	r.DecayedScore = DecayScore(r.RawScore, daysSinceLast, decayDays)

	// 6. 计算等级
	r.RiskLevel = m.CalcRiskLevel(ctx, r.DecayedScore)

	// 7. upsert user_risk_score
	score := model.UserRiskScore{
		TenantID:        r.TenantID,
		AppID:           r.AppID,
		UserType:        userType,
		UserID:          userID,
		Username:        r.Username,
		RiskScore:       r.DecayedScore,
		RiskLevel:       r.RiskLevel,
		EventCount:      r.EventCount,
		HighFreqHits:    r.HighFreqHits,
		GeoAnomalyHits:  r.GeoAnomalyHits,
		NewDeviceHits:   r.NewDeviceHits,
		AbnormalUAHits:  r.AbnormalUAHits,
		FailRateHigh:    r.FailRateHigh,
		MultiIPHits:     r.MultiIPHits,
		MultiDevHits:    r.MultiDevHits,
		LastEventAt:     r.LastEventAt,
	}
	now := time.Now()
	score.LastEvalAt = &now

	// 8. 自动封禁检查
	critical := m.cfg.GetInt(ctx, CfgKeyRiskCriticalThreshold, 100)
	if r.DecayedScore >= critical {
		score.Banned = true
		if score.BannedReason == "" {
			score.BannedReason = "评分达到致命阈值，自动封禁候选"
			score.BannedAt = &now
		}
	}

	if err := m.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 先查询是否已存在（保留原 banned_reason/banned_at）
		var existing model.UserRiskScore
		result := tx.Where("user_type = ? AND user_id = ?", userType, userID).First(&existing)
		if result.Error == nil {
			// 已存在：更新评分相关字段，但保留 banned_reason/banned_at（除非新触发）
			if score.Banned && !existing.Banned {
				// 新触发封禁
				score.BannedReason = "评分达到致命阈值，自动封禁候选"
				score.BannedAt = &now
			} else if existing.Banned {
				// 已封禁：保留原封禁信息
				score.Banned = true
				score.BannedReason = existing.BannedReason
				score.BannedAt = existing.BannedAt
			}
			return tx.Model(&existing).Where("id = ?", existing.ID).Updates(map[string]interface{}{
				"tenant_id":         score.TenantID,
				"app_id":            score.AppID,
				"username":          score.Username,
				"risk_score":        score.RiskScore,
				"risk_level":        score.RiskLevel,
				"event_count":       score.EventCount,
				"high_freq_hits":    score.HighFreqHits,
				"geo_anomaly_hits":  score.GeoAnomalyHits,
				"new_device_hits":   score.NewDeviceHits,
				"abnormal_ua_hits":  score.AbnormalUAHits,
				"fail_rate_high_hits": score.FailRateHigh,
				"multi_ip_hits":     score.MultiIPHits,
				"multi_dev_hits":    score.MultiDevHits,
				"last_event_at":     score.LastEventAt,
				"last_eval_at":      score.LastEvalAt,
				"banned":            score.Banned,
				"banned_reason":     score.BannedReason,
				"banned_at":         score.BannedAt,
			}).Error
		}
		// 不存在：插入
		return tx.Create(&score).Error
	}); err != nil {
		return nil, fmt.Errorf("写入 user_risk_score 失败: %w", err)
	}

	return r, nil
}

// evaluateEndUserAnomalies 评估终端用户的 log_verify 24h 异常模式
// 修改 reevaluateResult 的 FailRateHigh / MultiIPHits / MultiDevHits 字段
// 同时填充 TenantID / AppID（如未从 risk_event 中获取）
func (m *Manager) evaluateEndUserAnomalies(ctx context.Context, userID uint64, r *reevaluateResult) error {
	// 查询该用户绑定的所有卡密
	var cards []model.AppCard
	if err := m.db.WithContext(ctx).
		Select("id, tenant_id, app_id").
		Where("end_user_id = ?", userID).
		Find(&cards).Error; err != nil {
		return fmt.Errorf("查询 app_card 失败: %w", err)
	}
	if len(cards) == 0 {
		return nil // 未绑卡，无 log_verify 数据可分析
	}

	cardIDs := make([]uint64, 0, len(cards))
	for _, c := range cards {
		cardIDs = append(cardIDs, c.ID)
		if r.TenantID == 0 {
			r.TenantID = c.TenantID
		}
		if r.AppID == 0 {
			r.AppID = c.AppID
		}
	}

	// 查询最近 24h 的 log_verify
	cutoff := time.Now().Add(-24 * time.Hour)
	var logs []model.LogVerify
	if err := m.db.WithContext(ctx).
		Where("card_id IN ? AND created_at >= ?", cardIDs, cutoff).
		Find(&logs).Error; err != nil {
		return fmt.Errorf("查询 log_verify 失败: %w", err)
	}
	if len(logs) == 0 {
		return nil
	}

	// 失败率检测
	totalOps := 0
	failCount := 0
	ipSet := make(map[string]struct{})
	devSet := make(map[uint64]struct{})
	for _, lv := range logs {
		if lv.Result == "success" || lv.Result == "fail" {
			totalOps++
			if lv.Result == "fail" {
				failCount++
			}
		}
		if lv.ClientIP != "" {
			ipSet[lv.ClientIP] = struct{}{}
		}
		if lv.DeviceID != nil {
			devSet[*lv.DeviceID] = struct{}{}
		}
	}

	failRateThreshold := m.cfg.GetInt(ctx, CfgKeyThresholdFailRate, 50)
	if totalOps > 0 {
		failRate := failCount * 100 / totalOps
		if failRate > failRateThreshold {
			r.FailRateHigh = 1
		}
	}

	// 多 IP 检测
	multiIPThreshold := m.cfg.GetInt(ctx, CfgKeyThresholdMultiIPCount, 3)
	if len(ipSet) >= multiIPThreshold {
		r.MultiIPHits = 1
	}

	// 多设备检测
	multiDevThreshold := m.cfg.GetInt(ctx, CfgKeyThresholdMultiDevCount, 5)
	if len(devSet) >= multiDevThreshold {
		r.MultiDevHits = 1
	}

	return nil
}

// ReevaluateAllRiskScores 批量重算所有用户的评分
// 调用方：worker 定时调用
// 策略：扫描所有 user_risk_score + risk_event 中存在但未在 user_risk_score 中的用户
// 返回：处理的用户数 + 错误
func (m *Manager) ReevaluateAllRiskScores(ctx context.Context) (int, error) {
	// 1. 收集所有需要评估的 (user_type, user_id) 对
	type userKey struct {
		UserType string
		UserID   uint64
	}
	userSet := make(map[userKey]struct{})

	// 1.1 已在 user_risk_score 表中的
	var existing []model.UserRiskScore
	if err := m.db.WithContext(ctx).Select("user_type, user_id").Find(&existing).Error; err != nil {
		return 0, fmt.Errorf("查询 user_risk_score 失败: %w", err)
	}
	for _, u := range existing {
		userSet[userKey{u.UserType, u.UserID}] = struct{}{}
	}

	// 1.2 risk_event 中存在但未在 user_risk_score 表中的
	decayDays := m.cfg.GetInt(ctx, CfgKeyDecayDays, 7)
	cutoff := time.Now().AddDate(0, 0, -decayDays)
	var events []model.RiskEvent
	if err := m.db.WithContext(ctx).
		Select("DISTINCT user_type, user_id").
		Where("created_at >= ?", cutoff).
		Find(&events).Error; err != nil {
		return 0, fmt.Errorf("查询 risk_event 失败: %w", err)
	}
	for _, e := range events {
		userSet[userKey{e.UserType, e.UserID}] = struct{}{}
	}

	// 2. 逐个重算
	count := 0
	for k := range userSet {
		if _, err := m.ReevaluateUserRiskScore(ctx, k.UserType, k.UserID); err != nil {
			// 单用户失败不中断整体流程，记录日志继续
			continue
		}
		count++
	}
	return count, nil
}

// ============== 查询接口 ==============

// GetRiskUserOverview 风险用户总览
func (m *Manager) GetRiskUserOverview(ctx context.Context, f Filter) (*RiskUserOverview, error) {
	// baseWhere 构造基础过滤条件（避免链式 Where 共享 Statement 状态）
	baseWhere := func(q *gorm.DB) *gorm.DB {
		if f.TenantID > 0 {
			q = q.Where("tenant_id = ?", f.TenantID)
		}
		if f.AppID > 0 {
			q = q.Where("app_id = ?", f.AppID)
		}
		return q
	}

	ov := &RiskUserOverview{}
	if err := baseWhere(m.db.WithContext(ctx).Model(&model.UserRiskScore{})).Count(&ov.TotalUsers).Error; err != nil {
		return nil, err
	}
	if err := baseWhere(m.db.WithContext(ctx).Model(&model.UserRiskScore{})).Where("risk_level = ?", RiskLevelHigh).Count(&ov.HighRiskCount).Error; err != nil {
		return nil, err
	}
	if err := baseWhere(m.db.WithContext(ctx).Model(&model.UserRiskScore{})).Where("risk_level = ?", RiskLevelCritical).Count(&ov.CriticalCount).Error; err != nil {
		return nil, err
	}
	if err := baseWhere(m.db.WithContext(ctx).Model(&model.UserRiskScore{})).Where("risk_level = ?", RiskLevelMedium).Count(&ov.MediumRiskCount).Error; err != nil {
		return nil, err
	}
	if err := baseWhere(m.db.WithContext(ctx).Model(&model.UserRiskScore{})).Where("banned = ?", true).Count(&ov.BannedCount).Error; err != nil {
		return nil, err
	}

	// 最近 30 天风控事件总数（RiskEvent 表无 tenant_id，无法按租户过滤）
	days := m.cfg.GetInt(ctx, CfgKeyLookbackDays, 30)
	startDate := time.Now().AddDate(0, 0, -days)
	if err := m.db.WithContext(ctx).Model(&model.RiskEvent{}).
		Where("created_at >= ?", startDate).
		Count(&ov.TotalEvents).Error; err != nil {
		return nil, err
	}
	return ov, nil
}

// ListRiskUsers 列出风险用户
// 排序：按 risk_score 降序
// 可按 risk_level / banned 过滤
func (m *Manager) ListRiskUsers(ctx context.Context, f Filter) ([]RiskUserSummary, int64, error) {
	page, pageSize := normalizePage(f.Page, f.PageSize)

	q := m.db.WithContext(ctx).Model(&model.UserRiskScore{})
	if f.TenantID > 0 {
		q = q.Where("tenant_id = ?", f.TenantID)
	}
	if f.AppID > 0 {
		q = q.Where("app_id = ?", f.AppID)
	}
	if f.UserType != "" {
		q = q.Where("user_type = ?", f.UserType)
	}
	if f.Level != "" {
		q = q.Where("risk_level = ?", f.Level)
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var scores []model.UserRiskScore
	if err := q.Order("risk_score DESC, last_event_at DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&scores).Error; err != nil {
		return nil, 0, err
	}

	out := make([]RiskUserSummary, 0, len(scores))
	for _, s := range scores {
		out = append(out, RiskUserSummary{
			UserID:         s.UserID,
			UserType:       s.UserType,
			Username:       s.Username,
			TenantID:       s.TenantID,
			AppID:          s.AppID,
			RiskScore:      s.RiskScore,
			RiskLevel:      s.RiskLevel,
			EventCount:     s.EventCount,
			HighFreqHits:   s.HighFreqHits,
			GeoAnomalyHits: s.GeoAnomalyHits,
			NewDeviceHits:  s.NewDeviceHits,
			AbnormalUAHits: s.AbnormalUAHits,
			FailRateHigh:   s.FailRateHigh,
			MultiIPHits:    s.MultiIPHits,
			MultiDevHits:   s.MultiDevHits,
			Banned:         s.Banned,
			BannedReason:   s.BannedReason,
			LastEventAt:    s.LastEventAt,
			LastEvalAt:     s.LastEvalAt,
			UpdatedAt:      s.UpdatedAt,
		})
	}
	return out, total, nil
}

// GetRiskUserDetail 单用户风险详情
// recentEventsLimit：最近事件返回数量（默认 20）
func (m *Manager) GetRiskUserDetail(ctx context.Context, userType string, userID uint64, recentEventsLimit int) (*RiskUserDetail, error) {
	if recentEventsLimit <= 0 || recentEventsLimit > 100 {
		recentEventsLimit = 20
	}

	// 查询评分
	var score model.UserRiskScore
	err := m.db.WithContext(ctx).
		Where("user_type = ? AND user_id = ?", userType, userID).
		First(&score).Error
	if err != nil {
		return nil, fmt.Errorf("用户评分记录不存在: %w", err)
	}

	// 查询最近事件
	var events []model.RiskEvent
	if err := m.db.WithContext(ctx).
		Where("user_type = ? AND user_id = ?", userType, userID).
		Order("created_at DESC").
		Limit(recentEventsLimit).
		Find(&events).Error; err != nil {
		return nil, fmt.Errorf("查询 risk_event 失败: %w", err)
	}

	detail := &RiskUserDetail{
		Summary: RiskUserSummary{
			UserID:         score.UserID,
			UserType:       score.UserType,
			Username:       score.Username,
			TenantID:       score.TenantID,
			AppID:          score.AppID,
			RiskScore:      score.RiskScore,
			RiskLevel:      score.RiskLevel,
			EventCount:     score.EventCount,
			HighFreqHits:   score.HighFreqHits,
			GeoAnomalyHits: score.GeoAnomalyHits,
			NewDeviceHits:  score.NewDeviceHits,
			AbnormalUAHits: score.AbnormalUAHits,
			FailRateHigh:   score.FailRateHigh,
			MultiIPHits:    score.MultiIPHits,
			MultiDevHits:   score.MultiDevHits,
			Banned:         score.Banned,
			BannedReason:   score.BannedReason,
			LastEventAt:    score.LastEventAt,
			LastEvalAt:     score.LastEvalAt,
			UpdatedAt:      score.UpdatedAt,
		},
		RecentEvents: make([]RiskEventBrief, 0, len(events)),
	}
	for _, e := range events {
		detail.RecentEvents = append(detail.RecentEvents, RiskEventBrief{
			ID:          e.ID,
			RuleType:    e.RuleType,
			RuleName:    e.RuleName,
			RiskScore:   e.RiskScore,
			ActionTaken: e.ActionTaken,
			ClientIP:    e.ClientIP,
			Detail:      e.Detail,
			CreatedAt:   e.CreatedAt,
		})
	}
	return detail, nil
}

// ============== 手动操作 ==============

// BanUser 手动封禁用户
// reason：封禁原因（写入 banned_reason）
// 操作会立即标记 banned=true，但不会改变 risk_score
func (m *Manager) BanUser(ctx context.Context, userType string, userID uint64, reason string) error {
	now := time.Now()
	result := m.db.WithContext(ctx).Model(&model.UserRiskScore{}).
		Where("user_type = ? AND user_id = ?", userType, userID).
		Updates(map[string]interface{}{
			"banned":        true,
			"banned_reason": reason,
			"banned_at":     &now,
		})
	if result.Error != nil {
		return fmt.Errorf("封禁失败: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		// 不存在评分记录：创建一条仅含封禁信息的记录
		score := &model.UserRiskScore{
			UserType:      userType,
			UserID:        userID,
			RiskLevel:     RiskLevelLow,
			Banned:        true,
			BannedReason:  reason,
			BannedAt:      &now,
			LastEvalAt:    &now,
		}
		if err := m.db.WithContext(ctx).Create(score).Error; err != nil {
			return fmt.Errorf("创建封禁记录失败: %w", err)
		}
	}
	return nil
}

// UnbanUser 手动解封
// 清除 banned/banned_reason/banned_at，但保留 risk_score（需要 worker 重算）
func (m *Manager) UnbanUser(ctx context.Context, userType string, userID uint64) error {
	result := m.db.WithContext(ctx).Model(&model.UserRiskScore{}).
		Where("user_type = ? AND user_id = ?", userType, userID).
		Updates(map[string]interface{}{
			"banned":        false,
			"banned_reason": "",
			"banned_at":     nil,
		})
	if result.Error != nil {
		return fmt.Errorf("解封失败: %w", result.Error)
	}
	return nil
}
