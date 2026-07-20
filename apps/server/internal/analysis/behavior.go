// v0.6.0 高级分析子模块 1：用户行为分析
//
// 数据流：
//   log_verify JOIN app_card（end_user_id 反查）→ 按 (end_user_id, stat_date) 聚合
//   → 写入 user_behavior_profile 表 → 查询接口直接读聚合表
//
// 严格遵循铁律 06：聚合算法确定性强，无随机
package analysis

import (
	"context"
	"fmt"
	"time"

	"github.com/your-org/keyauth-saas/apps/server/internal/model"
)

// ============== 输出结构 ==============

// BehaviorOverview 用户行为总览（KPI 卡片数据）
type BehaviorOverview struct {
	TotalActiveUsers   int64   `json:"total_active_users"`    // 回溯周期内活跃终端用户数
	TotalLoginCount    int64   `json:"total_login_count"`     // 回溯周期内总登录次数
	TotalVerifyCount   int64   `json:"total_verify_count"`    // 回溯周期内总验证次数
	TotalHeartbeatCount int64  `json:"total_heartbeat_count"` // 回溯周期内总心跳次数
	TotalBindCount     int64   `json:"total_bind_count"`      // 回溯周期内总绑卡次数
	TotalFailCount     int64   `json:"total_fail_count"`      // 回溯周期内总失败次数
	AvgFailRate        float64 `json:"avg_fail_rate"`         // 平均失败率（百分比）
	AvgIPPerUser       float64 `json:"avg_ip_per_user"`       // 平均每用户 IP 数
	AvgDevicePerUser   float64 `json:"avg_device_per_user"`   // 平均每用户设备数
	LookbackDays       int     `json:"lookback_days"`
}

// UserBehaviorSummary 单个终端用户的行为汇总（列表项）
type UserBehaviorSummary struct {
	EndUserID       uint64     `json:"end_user_id"`
	Username        string     `json:"username"`
	TenantID        uint64     `json:"tenant_id"`
	AppID           uint64     `json:"app_id"`
	LoginCount      int64      `json:"login_count"`
	VerifyCount     int64      `json:"verify_count"`
	HeartbeatCount  int64      `json:"heartbeat_count"`
	BindCount       int64      `json:"bind_count"`
	SuccessCount    int64      `json:"success_count"`
	FailCount       int64      `json:"fail_count"`
	BannedCount     int64      `json:"banned_count"`
	DistinctIPCount int64      `json:"distinct_ip_count"`
	DistinctDevCount int64     `json:"distinct_device_count"`
	FailRate        float64    `json:"fail_rate"` // 百分比
	FirstActiveAt   *time.Time `json:"first_active_at"`
	LastActiveAt    *time.Time `json:"last_active_at"`
}

// UserBehaviorDetail 单用户详情（按日序列 + 汇总）
type UserBehaviorDetail struct {
	Summary UserBehaviorSummary   `json:"summary"`
	Daily   []UserBehaviorDaily   `json:"daily"` // 按日序列（最近 days 天）
}

// UserBehaviorDaily 单日行为数据
type UserBehaviorDaily struct {
	StatDate         string     `json:"stat_date"`
	LoginCount       int        `json:"login_count"`
	VerifyCount      int        `json:"verify_count"`
	HeartbeatCount   int        `json:"heartbeat_count"`
	BindCount        int        `json:"bind_count"`
	SuccessCount     int        `json:"success_count"`
	FailCount        int        `json:"fail_count"`
	DistinctIPCount  int        `json:"distinct_ip_count"`
	DistinctDevCount int        `json:"distinct_device_count"`
	LastActiveAt     *time.Time `json:"last_active_at"`
}

// BehaviorTrendPoint 行为趋势单日数据点
type BehaviorTrendPoint struct {
	StatDate         string `json:"stat_date"`
	ActiveUsers      int64  `json:"active_users"` // 当日活跃用户数
	LoginCount       int64  `json:"login_count"`
	VerifyCount      int64  `json:"verify_count"`
	HeartbeatCount   int64  `json:"heartbeat_count"`
	FailCount        int64  `json:"fail_count"`
}

// ============== 聚合函数（由 worker 调用） ==============

// AggregateUserBehaviorForDate 聚合指定日期的终端用户行为
// date 格式：YYYY-MM-DD（本地时区）
// 步骤：
//   1. 查询当日 log_verify 所有记录
//   2. 通过 app_card 反查 end_user_id（end_user_id IS NOT NULL）
//   3. 按 end_user_id 内存聚合（action 计数 / result 计数 / distinct IP/device）
//   4. upsert 到 user_behavior_profile 表
// 返回：聚合的用户数
func (m *Manager) AggregateUserBehaviorForDate(ctx context.Context, date string) (int, error) {
	// 1. 解析日期范围
	day, err := time.ParseInLocation("2006-01-02", date, time.Local)
	if err != nil {
		return 0, fmt.Errorf("日期格式非法（应为 YYYY-MM-DD）: %w", err)
	}
	start := day
	end := day.AddDate(0, 0, 1)

	// 2. 查询当日 log_verify
	var logs []model.LogVerify
	if err := m.db.WithContext(ctx).
		Where("created_at >= ? AND created_at < ?", start, end).
		Find(&logs).Error; err != nil {
		return 0, fmt.Errorf("查询 log_verify 失败: %w", err)
	}

	// 3. 收集所有 card_id，批量查询 app_card.end_user_id
	cardIDSet := make(map[uint64]struct{})
	for _, lv := range logs {
		if lv.CardID != nil {
			cardIDSet[*lv.CardID] = struct{}{}
		}
	}
	cardToUser := make(map[uint64]uint64)
	if len(cardIDSet) > 0 {
		cardIDs := make([]uint64, 0, len(cardIDSet))
		for id := range cardIDSet {
			cardIDs = append(cardIDs, id)
		}
		var cards []model.AppCard
		if err := m.db.WithContext(ctx).
			Select("id, end_user_id").
			Where("id IN ? AND end_user_id IS NOT NULL", cardIDs).
			Find(&cards).Error; err != nil {
			return 0, fmt.Errorf("查询 app_card 失败: %w", err)
		}
		for _, c := range cards {
			if c.EndUserID != nil {
				cardToUser[c.ID] = *c.EndUserID
			}
		}
	}

	// 4. 内存聚合
	type aggKey struct {
		EndUserID uint64
	}
	type aggVal struct {
		TenantID        uint64
		AppID           uint64
		LoginCount      int
		VerifyCount     int
		HeartbeatCount  int
		BindCount       int
		UnbindCount     int
		SuccessCount    int
		FailCount       int
		BannedCount     int
		DistinctIPs     map[string]struct{}
		DistinctDevices map[uint64]struct{}
		FirstActiveAt   *time.Time
		LastActiveAt    *time.Time
	}
	aggregates := make(map[aggKey]*aggVal)

	for _, lv := range logs {
		if lv.CardID == nil {
			continue
		}
		uid, ok := cardToUser[*lv.CardID]
		if !ok {
			continue // 卡密未绑定终端用户
		}
		key := aggKey{EndUserID: uid}
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
		// action 计数
		switch lv.Action {
		case "login":
			a.LoginCount++
		case "verify":
			a.VerifyCount++
		case "heartbeat":
			a.HeartbeatCount++
		case "bind":
			a.BindCount++
		case "unbind":
			a.UnbindCount++
		}
		// result 计数
		switch lv.Result {
		case "success":
			a.SuccessCount++
		case "fail":
			a.FailCount++
		case "banned":
			a.BannedCount++
		}
		// distinct
		if lv.ClientIP != "" {
			a.DistinctIPs[lv.ClientIP] = struct{}{}
		}
		if lv.DeviceID != nil {
			a.DistinctDevices[*lv.DeviceID] = struct{}{}
		}
		// 时间范围
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

	// 5. upsert 到 user_behavior_profile
	for key, a := range aggregates {
		profile := model.UserBehaviorProfile{
			TenantID:         a.TenantID,
			AppID:            a.AppID,
			EndUserID:        key.EndUserID,
			StatDate:         date,
			LoginCount:       a.LoginCount,
			VerifyCount:      a.VerifyCount,
			HeartbeatCount:   a.HeartbeatCount,
			BindCount:        a.BindCount,
			UnbindCount:      a.UnbindCount,
			SuccessCount:     a.SuccessCount,
			FailCount:        a.FailCount,
			BannedCount:      a.BannedCount,
			DistinctIPCount:  len(a.DistinctIPs),
			DistinctDevCount: len(a.DistinctDevices),
			FirstActiveAt:    a.FirstActiveAt,
			LastActiveAt:     a.LastActiveAt,
		}
		// upsert：按 (end_user_id, stat_date) 唯一索引
		if err := m.db.WithContext(ctx).
			Where("end_user_id = ? AND stat_date = ?", key.EndUserID, date).
			Assign(profile).
			FirstOrCreate(&profile).Error; err != nil {
			return 0, fmt.Errorf("写入 user_behavior_profile 失败: %w", err)
		}
	}

	return len(aggregates), nil
}

// ============== 查询接口 ==============

// GetBehaviorOverview 用户行为总览
// 回溯天数从 sys_config 读取（默认 30 天）
func (m *Manager) GetBehaviorOverview(ctx context.Context, f Filter) (*BehaviorOverview, error) {
	days := m.cfg.GetInt(ctx, CfgKeyLookbackDays, 30)
	start := time.Now().AddDate(0, 0, -days)
	startDate := statDateStr(start)

	q := m.db.WithContext(ctx).Model(&model.UserBehaviorProfile{}).
		Where("stat_date >= ?", startDate)
	if f.TenantID > 0 {
		q = q.Where("tenant_id = ?", f.TenantID)
	}
	if f.AppID > 0 {
		q = q.Where("app_id = ?", f.AppID)
	}

	var totalUsers int64
	if err := q.Distinct("end_user_id").Count(&totalUsers).Error; err != nil {
		return nil, err
	}

	type sumRow struct {
		TotalLogin      int64
		TotalVerify     int64
		TotalHeartbeat  int64
		TotalBind       int64
		TotalFail       int64
		TotalSuccess    int64
		SumIP           int64
		SumDevice       int64
	}
	var sum sumRow
	err := q.Select(`
		COALESCE(SUM(login_count), 0) as total_login,
		COALESCE(SUM(verify_count), 0) as total_verify,
		COALESCE(SUM(heartbeat_count), 0) as total_heartbeat,
		COALESCE(SUM(bind_count), 0) as total_bind,
		COALESCE(SUM(fail_count), 0) as total_fail,
		COALESCE(SUM(success_count), 0) as total_success,
		COALESCE(SUM(distinct_ip_count), 0) as sum_ip,
		COALESCE(SUM(distinct_device_count), 0) as sum_device
	`).Scan(&sum).Error
	if err != nil {
		return nil, err
	}

	ov := &BehaviorOverview{
		TotalActiveUsers:    totalUsers,
		TotalLoginCount:     sum.TotalLogin,
		TotalVerifyCount:    sum.TotalVerify,
		TotalHeartbeatCount: sum.TotalHeartbeat,
		TotalBindCount:      sum.TotalBind,
		TotalFailCount:      sum.TotalFail,
		LookbackDays:        days,
	}
	totalOps := sum.TotalSuccess + sum.TotalFail
	if totalOps > 0 {
		ov.AvgFailRate = float64(sum.TotalFail) / float64(totalOps) * 100
	}
	if totalUsers > 0 {
		ov.AvgIPPerUser = float64(sum.SumIP) / float64(totalUsers)
		ov.AvgDevicePerUser = float64(sum.SumDevice) / float64(totalUsers)
	}
	return ov, nil
}

// ListUserBehaviors 列出终端用户行为汇总（按用户聚合回溯周期数据）
// 排序：按总活跃度（login+verify+heartbeat）降序
func (m *Manager) ListUserBehaviors(ctx context.Context, f Filter) ([]UserBehaviorSummary, int64, error) {
	page, pageSize := normalizePage(f.Page, f.PageSize)
	days := m.cfg.GetInt(ctx, CfgKeyLookbackDays, 30)
	startDate := statDateStr(time.Now().AddDate(0, 0, -days))

	q := m.db.WithContext(ctx).Model(&model.UserBehaviorProfile{}).
		Where("stat_date >= ?", startDate)
	if f.TenantID > 0 {
		q = q.Where("tenant_id = ?", f.TenantID)
	}
	if f.AppID > 0 {
		q = q.Where("app_id = ?", f.AppID)
	}

	var total int64
	// 统计 distinct end_user_id 数量
	countQ := m.db.WithContext(ctx).Model(&model.UserBehaviorProfile{}).
		Where("stat_date >= ?", startDate)
	if f.TenantID > 0 {
		countQ = countQ.Where("tenant_id = ?", f.TenantID)
	}
	if f.AppID > 0 {
		countQ = countQ.Where("app_id = ?", f.AppID)
	}
	if err := countQ.Distinct("end_user_id").Count(&total).Error; err != nil {
		return nil, 0, err
	}

	type row struct {
		EndUserID        uint64
		TenantID         uint64
		AppID            uint64
		LoginCount       int64
		VerifyCount      int64
		HeartbeatCount   int64
		BindCount        int64
		SuccessCount     int64
		FailCount        int64
		BannedCount      int64
		DistinctIPCount  int64
		DistinctDevCount int64
		FirstActiveAt    string
		LastActiveAt     string
	}
	var rows []row
	err := q.Select(`
		end_user_id,
		MAX(tenant_id) as tenant_id,
		MAX(app_id) as app_id,
		SUM(login_count) as login_count,
		SUM(verify_count) as verify_count,
		SUM(heartbeat_count) as heartbeat_count,
		SUM(bind_count) as bind_count,
		SUM(success_count) as success_count,
		SUM(fail_count) as fail_count,
		SUM(banned_count) as banned_count,
		MAX(distinct_ip_count) as distinct_ip_count,
		MAX(distinct_device_count) as distinct_dev_count,
		MIN(first_active_at) as first_active_at,
		MAX(last_active_at) as last_active_at
	`).
		Group("end_user_id").
		Order("(SUM(login_count) + SUM(verify_count) + SUM(heartbeat_count)) DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Scan(&rows).Error
	if err != nil {
		return nil, 0, err
	}

	// 批量查询 username
	userIDs := make([]uint64, 0, len(rows))
	for _, r := range rows {
		userIDs = append(userIDs, r.EndUserID)
	}
	usernames := make(map[uint64]string)
	if len(userIDs) > 0 {
		var users []model.EndUser
		m.db.WithContext(ctx).Select("id, username").Where("id IN ?", userIDs).Find(&users)
		for _, u := range users {
			usernames[u.ID] = u.Username
		}
	}

	out := make([]UserBehaviorSummary, 0, len(rows))
	for _, r := range rows {
		s := UserBehaviorSummary{
			EndUserID:        r.EndUserID,
			Username:         usernames[r.EndUserID],
			TenantID:         r.TenantID,
			AppID:            r.AppID,
			LoginCount:       r.LoginCount,
			VerifyCount:      r.VerifyCount,
			HeartbeatCount:   r.HeartbeatCount,
			BindCount:        r.BindCount,
			SuccessCount:     r.SuccessCount,
			FailCount:        r.FailCount,
			BannedCount:      r.BannedCount,
			DistinctIPCount:  r.DistinctIPCount,
			DistinctDevCount: r.DistinctDevCount,
			FirstActiveAt:    parseTimePtr(r.FirstActiveAt),
			LastActiveAt:     parseTimePtr(r.LastActiveAt),
		}
		total := s.SuccessCount + s.FailCount
		if total > 0 {
			s.FailRate = float64(s.FailCount) / float64(total) * 100
		}
		out = append(out, s)
	}
	return out, total, nil
}

// GetUserBehaviorDetail 单用户行为详情（按日序列 + 汇总）
func (m *Manager) GetUserBehaviorDetail(ctx context.Context, userID uint64, days int) (*UserBehaviorDetail, error) {
	if days <= 0 {
		days = m.cfg.GetInt(ctx, CfgKeyLookbackDays, 30)
	}
	startDate := statDateStr(time.Now().AddDate(0, 0, -days))

	var profiles []model.UserBehaviorProfile
	err := m.db.WithContext(ctx).
		Where("end_user_id = ? AND stat_date >= ?", userID, startDate).
		Order("stat_date ASC").
		Find(&profiles).Error
	if err != nil {
		return nil, err
	}

	detail := &UserBehaviorDetail{
		Daily: make([]UserBehaviorDaily, 0, len(profiles)),
	}
	// 汇总
	var sum UserBehaviorSummary
	sum.EndUserID = userID
	for _, p := range profiles {
		detail.Daily = append(detail.Daily, UserBehaviorDaily{
			StatDate:         p.StatDate,
			LoginCount:       p.LoginCount,
			VerifyCount:      p.VerifyCount,
			HeartbeatCount:   p.HeartbeatCount,
			BindCount:        p.BindCount,
			SuccessCount:     p.SuccessCount,
			FailCount:        p.FailCount,
			DistinctIPCount:  p.DistinctIPCount,
			DistinctDevCount: p.DistinctDevCount,
			LastActiveAt:     p.LastActiveAt,
		})
		sum.TenantID = p.TenantID
		sum.AppID = p.AppID
		sum.LoginCount += int64(p.LoginCount)
		sum.VerifyCount += int64(p.VerifyCount)
		sum.HeartbeatCount += int64(p.HeartbeatCount)
		sum.BindCount += int64(p.BindCount)
		sum.SuccessCount += int64(p.SuccessCount)
		sum.FailCount += int64(p.FailCount)
		sum.BannedCount += int64(p.BannedCount)
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

	// 查 username
	var u model.EndUser
	if err := m.db.WithContext(ctx).Select("id, username").First(&u, userID).Error; err == nil {
		sum.Username = u.Username
	}
	detail.Summary = sum
	return detail, nil
}

// GetBehaviorTrend 行为趋势（按日聚合所有用户）
func (m *Manager) GetBehaviorTrend(ctx context.Context, f Filter, days int) ([]BehaviorTrendPoint, error) {
	if days <= 0 {
		days = m.cfg.GetInt(ctx, CfgKeyLookbackDays, 30)
	}
	startDate := statDateStr(time.Now().AddDate(0, 0, -days))

	q := m.db.WithContext(ctx).Model(&model.UserBehaviorProfile{}).
		Where("stat_date >= ?", startDate)
	if f.TenantID > 0 {
		q = q.Where("tenant_id = ?", f.TenantID)
	}
	if f.AppID > 0 {
		q = q.Where("app_id = ?", f.AppID)
	}

	type row struct {
		StatDate       string
		ActiveUsers    int64
		LoginCount     int64
		VerifyCount    int64
		HeartbeatCount int64
		FailCount      int64
	}
	var rows []row
	err := q.Select(`
		stat_date,
		COUNT(DISTINCT end_user_id) as active_users,
		SUM(login_count) as login_count,
		SUM(verify_count) as verify_count,
		SUM(heartbeat_count) as heartbeat_count,
		SUM(fail_count) as fail_count
	`).
		Group("stat_date").
		Order("stat_date ASC").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	out := make([]BehaviorTrendPoint, 0, len(rows))
	for _, r := range rows {
		out = append(out, BehaviorTrendPoint{
			StatDate:       r.StatDate,
			ActiveUsers:    r.ActiveUsers,
			LoginCount:     r.LoginCount,
			VerifyCount:    r.VerifyCount,
			HeartbeatCount: r.HeartbeatCount,
			FailCount:      r.FailCount,
		})
	}
	return out, nil
}
