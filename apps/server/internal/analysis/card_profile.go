// v0.6.0 高级分析子模块 2：卡密使用画像
//
// 数据流：
//   log_verify（按 card_id 直接聚合，无需 JOIN app_card）
//   → 按 (card_id, stat_date) 聚合 → 写入 card_usage_profile 表
//   → 查询接口直接读聚合表
//
// 与用户行为分析的区别：
//   1. 聚合维度是 card_id（即使未绑终端用户也纳入画像）
//   2. 额外有 device_mismatch_count（设备不匹配次数，卡密共享特征）
//   3. 排序按 verify_count + heartbeat_count（卡密使用强度）
package analysis

import (
	"context"
	"fmt"
	"time"

	"github.com/your-org/keyauth-saas/apps/server/internal/model"
)

// ============== 输出结构 ==============

// CardProfileOverview 卡密使用画像总览
type CardProfileOverview struct {
	TotalActiveCards     int64   `json:"total_active_cards"`     // 回溯周期内有使用的卡密数
	TotalVerifyCount     int64   `json:"total_verify_count"`
	TotalHeartbeatCount  int64   `json:"total_heartbeat_count"`
	TotalBindCount       int64   `json:"total_bind_count"`
	TotalFailCount       int64   `json:"total_fail_count"`
	TotalBannedCount     int64   `json:"total_banned_count"`
	TotalDevMismatch     int64   `json:"total_device_mismatch_count"` // 设备不匹配总数（卡密共享特征）
	AvgFailRate          float64 `json:"avg_fail_rate"`
	AvgIPPerCard         float64 `json:"avg_ip_per_card"`
	AvgDevicePerCard     float64 `json:"avg_device_per_card"`
	LookbackDays         int     `json:"lookback_days"`
}

// CardProfileSummary 单卡密使用画像汇总（列表项）
type CardProfileSummary struct {
	CardID               uint64     `json:"card_id"`
	TenantID             uint64     `json:"tenant_id"`
	AppID                uint64     `json:"app_id"`
	CardKey              string     `json:"card_key"` // 脱敏后显示（前 4 + **** + 后 4）
	Status               string     `json:"status"`
	VerifyCount          int64      `json:"verify_count"`
	HeartbeatCount       int64      `json:"heartbeat_count"`
	BindCount            int64      `json:"bind_count"`
	SuccessCount         int64      `json:"success_count"`
	FailCount            int64      `json:"fail_count"`
	BannedCount          int64      `json:"banned_count"`
	DeviceMismatchCount  int64      `json:"device_mismatch_count"`
	DistinctIPCount      int64      `json:"distinct_ip_count"`
	DistinctDevCount     int64      `json:"distinct_device_count"`
	FailRate             float64    `json:"fail_rate"`
	FirstActiveAt        *time.Time `json:"first_active_at"`
	LastActiveAt         *time.Time `json:"last_active_at"`
}

// CardProfileDetail 单卡密详情（按日序列 + 汇总）
type CardProfileDetail struct {
	Summary CardProfileSummary   `json:"summary"`
	Daily   []CardProfileDaily   `json:"daily"`
}

// CardProfileDaily 单日卡密使用数据
type CardProfileDaily struct {
	StatDate            string     `json:"stat_date"`
	VerifyCount         int        `json:"verify_count"`
	HeartbeatCount      int        `json:"heartbeat_count"`
	BindCount           int        `json:"bind_count"`
	SuccessCount        int        `json:"success_count"`
	FailCount           int        `json:"fail_count"`
	DeviceMismatchCount int        `json:"device_mismatch_count"`
	DistinctIPCount     int        `json:"distinct_ip_count"`
	DistinctDevCount    int        `json:"distinct_device_count"`
	LastActiveAt        *time.Time `json:"last_active_at"`
}

// CardProfileTrendPoint 卡密使用趋势单日数据点
type CardProfileTrendPoint struct {
	StatDate        string `json:"stat_date"`
	ActiveCards     int64  `json:"active_cards"` // 当日使用的卡密数
	VerifyCount     int64  `json:"verify_count"`
	HeartbeatCount  int64  `json:"heartbeat_count"`
	FailCount       int64  `json:"fail_count"`
	DevMismatch     int64  `json:"device_mismatch_count"`
}

// ============== 聚合函数 ==============

// AggregateCardProfileForDate 聚合指定日期的卡密使用画像
// 与 AggregateUserBehaviorForDate 的区别：直接按 card_id 聚合，不依赖 end_user_id
func (m *Manager) AggregateCardProfileForDate(ctx context.Context, date string) (int, error) {
	day, err := time.ParseInLocation("2006-01-02", date, time.Local)
	if err != nil {
		return 0, fmt.Errorf("日期格式非法（应为 YYYY-MM-DD）: %w", err)
	}
	start := day
	end := day.AddDate(0, 0, 1)

	var logs []model.LogVerify
	if err := m.db.WithContext(ctx).
		Where("created_at >= ? AND created_at < ?", start, end).
		Find(&logs).Error; err != nil {
		return 0, fmt.Errorf("查询 log_verify 失败: %w", err)
	}

	// 内存聚合（按 card_id 分组）
	type aggKey struct {
		CardID uint64
	}
	type aggVal struct {
		TenantID             uint64
		AppID                uint64
		VerifyCount          int
		HeartbeatCount       int
		BindCount            int
		SuccessCount         int
		FailCount            int
		BannedCount          int
		DeviceMismatchCount  int
		DistinctIPs          map[string]struct{}
		DistinctDevices      map[uint64]struct{}
		FirstActiveAt        *time.Time
		LastActiveAt         *time.Time
	}
	aggregates := make(map[aggKey]*aggVal)

	for _, lv := range logs {
		if lv.CardID == nil {
			continue // 未关联卡密的日志（如登录失败）不纳入卡密画像
		}
		cardID := *lv.CardID
		key := aggKey{CardID: cardID}
		a := aggregates[key]
		if a == nil {
			a = &aggVal{
				TenantID:        lv.TenantID,
				AppID:           lv.AppID,
				DistinctIPs:     make(map[string]struct{}),
				DistinctDevices: make(map[uint64]struct{}),
			}
			aggregates[key] = a
		}
		switch lv.Action {
		case "verify":
			a.VerifyCount++
		case "heartbeat":
			a.HeartbeatCount++
		case "bind":
			a.BindCount++
		}
		switch lv.Result {
		case "success":
			a.SuccessCount++
		case "fail":
			a.FailCount++
		case "banned":
			a.BannedCount++
		case "device_mismatch":
			a.DeviceMismatchCount++
		}
		if lv.ClientIP != "" {
			a.DistinctIPs[lv.ClientIP] = struct{}{}
		}
		if lv.DeviceID != nil {
			a.DistinctDevices[*lv.DeviceID] = struct{}{}
		}
		t := lv.CreatedAt
		if a.FirstActiveAt == nil || t.Before(*a.FirstActiveAt) {
			tCopy := t
			a.FirstActiveAt = &tCopy
		}
		if a.LastActiveAt == nil || t.After(*a.LastActiveAt) {
			tCopy := t
			a.LastActiveAt = &tCopy
		}
	}

	for key, a := range aggregates {
		profile := model.CardUsageProfile{
			TenantID:            a.TenantID,
			AppID:               a.AppID,
			CardID:              key.CardID,
			StatDate:            date,
			VerifyCount:         a.VerifyCount,
			HeartbeatCount:      a.HeartbeatCount,
			BindCount:           a.BindCount,
			SuccessCount:        a.SuccessCount,
			FailCount:           a.FailCount,
			BannedCount:         a.BannedCount,
			DeviceMismatchCount: a.DeviceMismatchCount,
			DistinctIPCount:     len(a.DistinctIPs),
			DistinctDevCount:    len(a.DistinctDevices),
			FirstActiveAt:       a.FirstActiveAt,
			LastActiveAt:        a.LastActiveAt,
		}
		if err := m.db.WithContext(ctx).
			Where("card_id = ? AND stat_date = ?", key.CardID, date).
			Assign(profile).
			FirstOrCreate(&profile).Error; err != nil {
			return 0, fmt.Errorf("写入 card_usage_profile 失败: %w", err)
		}
	}

	return len(aggregates), nil
}

// ============== 查询接口 ==============

// GetCardProfileOverview 卡密使用画像总览
func (m *Manager) GetCardProfileOverview(ctx context.Context, f Filter) (*CardProfileOverview, error) {
	days := m.cfg.GetInt(ctx, CfgKeyLookbackDays, 30)
	startDate := statDateStr(time.Now().AddDate(0, 0, -days))

	q := m.db.WithContext(ctx).Model(&model.CardUsageProfile{}).
		Where("stat_date >= ?", startDate)
	if f.TenantID > 0 {
		q = q.Where("tenant_id = ?", f.TenantID)
	}
	if f.AppID > 0 {
		q = q.Where("app_id = ?", f.AppID)
	}

	var totalCards int64
	if err := q.Distinct("card_id").Count(&totalCards).Error; err != nil {
		return nil, err
	}

	type sumRow struct {
		TotalVerify    int64
		TotalHeartbeat int64
		TotalBind      int64
		TotalFail      int64
		TotalSuccess   int64
		TotalBanned    int64
		TotalMismatch  int64
		SumIP          int64
		SumDevice      int64
	}
	var sum sumRow
	err := q.Select(`
		COALESCE(SUM(verify_count), 0) as total_verify,
		COALESCE(SUM(heartbeat_count), 0) as total_heartbeat,
		COALESCE(SUM(bind_count), 0) as total_bind,
		COALESCE(SUM(fail_count), 0) as total_fail,
		COALESCE(SUM(success_count), 0) as total_success,
		COALESCE(SUM(banned_count), 0) as total_banned,
		COALESCE(SUM(device_mismatch_count), 0) as total_mismatch,
		COALESCE(SUM(distinct_ip_count), 0) as sum_ip,
		COALESCE(SUM(distinct_device_count), 0) as sum_device
	`).Scan(&sum).Error
	if err != nil {
		return nil, err
	}

	ov := &CardProfileOverview{
		TotalActiveCards:    totalCards,
		TotalVerifyCount:    sum.TotalVerify,
		TotalHeartbeatCount: sum.TotalHeartbeat,
		TotalBindCount:      sum.TotalBind,
		TotalFailCount:      sum.TotalFail,
		TotalBannedCount:    sum.TotalBanned,
		TotalDevMismatch:    sum.TotalMismatch,
		LookbackDays:        days,
	}
	totalOps := sum.TotalSuccess + sum.TotalFail
	if totalOps > 0 {
		ov.AvgFailRate = float64(sum.TotalFail) / float64(totalOps) * 100
	}
	if totalCards > 0 {
		ov.AvgIPPerCard = float64(sum.SumIP) / float64(totalCards)
		ov.AvgDevicePerCard = float64(sum.SumDevice) / float64(totalCards)
	}
	return ov, nil
}

// ListCardProfiles 列出卡密使用画像
// 排序：按 verify_count + heartbeat_count 降序（卡密使用强度）
func (m *Manager) ListCardProfiles(ctx context.Context, f Filter) ([]CardProfileSummary, int64, error) {
	page, pageSize := normalizePage(f.Page, f.PageSize)
	days := m.cfg.GetInt(ctx, CfgKeyLookbackDays, 30)
	startDate := statDateStr(time.Now().AddDate(0, 0, -days))

	q := m.db.WithContext(ctx).Model(&model.CardUsageProfile{}).
		Where("stat_date >= ?", startDate)
	if f.TenantID > 0 {
		q = q.Where("tenant_id = ?", f.TenantID)
	}
	if f.AppID > 0 {
		q = q.Where("app_id = ?", f.AppID)
	}

	var total int64
	countQ := m.db.WithContext(ctx).Model(&model.CardUsageProfile{}).
		Where("stat_date >= ?", startDate)
	if f.TenantID > 0 {
		countQ = countQ.Where("tenant_id = ?", f.TenantID)
	}
	if f.AppID > 0 {
		countQ = countQ.Where("app_id = ?", f.AppID)
	}
	if err := countQ.Distinct("card_id").Count(&total).Error; err != nil {
		return nil, 0, err
	}

	type row struct {
		CardID              uint64
		TenantID            uint64
		AppID               uint64
		VerifyCount         int64
		HeartbeatCount      int64
		BindCount           int64
		SuccessCount        int64
		FailCount           int64
		BannedCount         int64
		DeviceMismatchCount int64
		DistinctIPCount     int64
		DistinctDevCount    int64
		FirstActiveAt       string
		LastActiveAt        string
	}
	var rows []row
	err := q.Select(`
		card_id,
		MAX(tenant_id) as tenant_id,
		MAX(app_id) as app_id,
		SUM(verify_count) as verify_count,
		SUM(heartbeat_count) as heartbeat_count,
		SUM(bind_count) as bind_count,
		SUM(success_count) as success_count,
		SUM(fail_count) as fail_count,
		SUM(banned_count) as banned_count,
		SUM(device_mismatch_count) as device_mismatch_count,
		MAX(distinct_ip_count) as distinct_ip_count,
		MAX(distinct_device_count) as distinct_dev_count,
		MIN(first_active_at) as first_active_at,
		MAX(last_active_at) as last_active_at
	`).
		Group("card_id").
		Order("(SUM(verify_count) + SUM(heartbeat_count)) DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Scan(&rows).Error
	if err != nil {
		return nil, 0, err
	}

	// 批量查询卡密状态 + 脱敏 card_key
	cardIDs := make([]uint64, 0, len(rows))
	for _, r := range rows {
		cardIDs = append(cardIDs, r.CardID)
	}
	cardInfo := make(map[uint64]struct {
		CardKey string
		Status  string
	})
	if len(cardIDs) > 0 {
		var cards []model.AppCard
		m.db.WithContext(ctx).Select("id, card_key, status").Where("id IN ?", cardIDs).Find(&cards)
		for _, c := range cards {
			cardInfo[c.ID] = struct {
				CardKey string
				Status  string
			}{CardKey: maskCardKey(c.CardKey), Status: c.Status}
		}
	}

	out := make([]CardProfileSummary, 0, len(rows))
	for _, r := range rows {
		s := CardProfileSummary{
			CardID:              r.CardID,
			TenantID:            r.TenantID,
			AppID:               r.AppID,
			VerifyCount:         r.VerifyCount,
			HeartbeatCount:      r.HeartbeatCount,
			BindCount:           r.BindCount,
			SuccessCount:        r.SuccessCount,
			FailCount:           r.FailCount,
			BannedCount:         r.BannedCount,
			DeviceMismatchCount: r.DeviceMismatchCount,
			DistinctIPCount:     r.DistinctIPCount,
		DistinctDevCount:    r.DistinctDevCount,
		FirstActiveAt:       parseTimePtr(r.FirstActiveAt),
		LastActiveAt:        parseTimePtr(r.LastActiveAt),
	}
		if info, ok := cardInfo[r.CardID]; ok {
			s.CardKey = info.CardKey
			s.Status = info.Status
		}
		total := s.SuccessCount + s.FailCount
		if total > 0 {
			s.FailRate = float64(s.FailCount) / float64(total) * 100
		}
		out = append(out, s)
	}
	return out, total, nil
}

// GetCardProfileDetail 单卡密使用画像详情
func (m *Manager) GetCardProfileDetail(ctx context.Context, cardID uint64, days int) (*CardProfileDetail, error) {
	if days <= 0 {
		days = m.cfg.GetInt(ctx, CfgKeyLookbackDays, 30)
	}
	startDate := statDateStr(time.Now().AddDate(0, 0, -days))

	var profiles []model.CardUsageProfile
	err := m.db.WithContext(ctx).
		Where("card_id = ? AND stat_date >= ?", cardID, startDate).
		Order("stat_date ASC").
		Find(&profiles).Error
	if err != nil {
		return nil, err
	}

	detail := &CardProfileDetail{
		Daily: make([]CardProfileDaily, 0, len(profiles)),
	}
	var sum CardProfileSummary
	sum.CardID = cardID
	for _, p := range profiles {
		detail.Daily = append(detail.Daily, CardProfileDaily{
			StatDate:            p.StatDate,
			VerifyCount:         p.VerifyCount,
			HeartbeatCount:      p.HeartbeatCount,
			BindCount:           p.BindCount,
			SuccessCount:        p.SuccessCount,
			FailCount:           p.FailCount,
			DeviceMismatchCount: p.DeviceMismatchCount,
			DistinctIPCount:     p.DistinctIPCount,
			DistinctDevCount:    p.DistinctDevCount,
			LastActiveAt:        p.LastActiveAt,
		})
		sum.TenantID = p.TenantID
		sum.AppID = p.AppID
		sum.VerifyCount += int64(p.VerifyCount)
		sum.HeartbeatCount += int64(p.HeartbeatCount)
		sum.BindCount += int64(p.BindCount)
		sum.SuccessCount += int64(p.SuccessCount)
		sum.FailCount += int64(p.FailCount)
		sum.BannedCount += int64(p.BannedCount)
		sum.DeviceMismatchCount += int64(p.DeviceMismatchCount)
		if p.FirstActiveAt != nil {
			if sum.FirstActiveAt == nil || p.FirstActiveAt.Before(*sum.FirstActiveAt) {
				sum.FirstActiveAt = p.FirstActiveAt
			}
		}
		if p.LastActiveAt != nil {
			if sum.LastActiveAt == nil || p.LastActiveAt.After(*sum.LastActiveAt) {
				sum.LastActiveAt = p.LastActiveAt
			}
		}
		if p.DistinctIPCount > int(sum.DistinctIPCount) {
			sum.DistinctIPCount = int64(p.DistinctIPCount)
		}
		if p.DistinctDevCount > int(sum.DistinctDevCount) {
			sum.DistinctDevCount = int64(p.DistinctDevCount)
		}
	}
	totalOps := sum.SuccessCount + sum.FailCount
	if totalOps > 0 {
		sum.FailRate = float64(sum.FailCount) / float64(totalOps) * 100
	}

	// 查 card_key + status
	var c model.AppCard
	if err := m.db.WithContext(ctx).Select("id, card_key, status").First(&c, cardID).Error; err == nil {
		sum.CardKey = maskCardKey(c.CardKey)
		sum.Status = c.Status
	}
	detail.Summary = sum
	return detail, nil
}

// GetCardProfileTrend 卡密使用趋势（按日聚合）
func (m *Manager) GetCardProfileTrend(ctx context.Context, f Filter, days int) ([]CardProfileTrendPoint, error) {
	if days <= 0 {
		days = m.cfg.GetInt(ctx, CfgKeyLookbackDays, 30)
	}
	startDate := statDateStr(time.Now().AddDate(0, 0, -days))

	q := m.db.WithContext(ctx).Model(&model.CardUsageProfile{}).
		Where("stat_date >= ?", startDate)
	if f.TenantID > 0 {
		q = q.Where("tenant_id = ?", f.TenantID)
	}
	if f.AppID > 0 {
		q = q.Where("app_id = ?", f.AppID)
	}

	type row struct {
		StatDate       string
		ActiveCards    int64
		VerifyCount    int64
		HeartbeatCount int64
		FailCount      int64
		DevMismatch    int64
	}
	var rows []row
	err := q.Select(`
		stat_date,
		COUNT(DISTINCT card_id) as active_cards,
		SUM(verify_count) as verify_count,
		SUM(heartbeat_count) as heartbeat_count,
		SUM(fail_count) as fail_count,
		SUM(device_mismatch_count) as dev_mismatch
	`).
		Group("stat_date").
		Order("stat_date ASC").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	out := make([]CardProfileTrendPoint, 0, len(rows))
	for _, r := range rows {
		out = append(out, CardProfileTrendPoint{
			StatDate:       r.StatDate,
			ActiveCards:    r.ActiveCards,
			VerifyCount:    r.VerifyCount,
			HeartbeatCount: r.HeartbeatCount,
			FailCount:      r.FailCount,
			DevMismatch:    r.DevMismatch,
		})
	}
	return out, nil
}

// maskCardKey 卡密脱敏：前 4 + **** + 后 4
// 长度 <= 8 时全部替换为 ****
func maskCardKey(key string) string {
	if len(key) <= 8 {
		return "****"
	}
	return key[:4] + "****" + key[len(key)-4:]
}
