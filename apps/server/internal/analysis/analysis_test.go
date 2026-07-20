// v0.6.0 高级分析单元测试
//
// 测试覆盖：
//   1. 工具函数：DecayScore / statDateStr / statDateRange / maskCardKey / normalizePage / CalcRiskLevel
//   2. 用户行为分析：AggregateUserBehaviorForDate / GetBehaviorOverview / ListUserBehaviors / GetUserBehaviorDetail / GetBehaviorTrend
//   3. 卡密画像：AggregateCardProfileForDate / GetCardProfileOverview / ListCardProfiles / GetCardProfileDetail / GetCardProfileTrend
//   4. 风险用户：ReevaluateUserRiskScore / ReevaluateAllRiskScores / GetRiskUserOverview / ListRiskUsers / GetRiskUserDetail / BanUser / UnbanUser
//   5. Worker：RunAggregationOnceSync 端到端
//
// 严格遵循铁律 06：所有断言基于已知固定输入，无随机/不确定性
package analysis

import (
	"context"
	"fmt"
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

// setupTestDB 构造测试用 SQLite 内存数据库
// 自动 migrate 所有相关表
func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	// 每个测试独立 DB：用唯一 DSN
	dsn := fmt.Sprintf("file:analysis_test_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&model.SysConfig{},
		&model.EndUser{},
		&model.AppCard{},
		&model.AppDevice{},
		&model.LogVerify{},
		&model.RiskEvent{},
		&model.UserBehaviorProfile{},
		&model.CardUsageProfile{},
		&model.UserRiskScore{},
	))
	return db
}

// setupTestCfgCache 构造测试用 ConfigCache（含默认配置）
func setupTestCfgCache(t *testing.T, db *gorm.DB, overrides map[string]string) *config.ConfigCache {
	t.Helper()
	mr, err := miniredis.Run()
	require.NoError(t, err)
	t.Cleanup(mr.Close)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	// 默认配置（与 migration 032 一致）
	defaults := map[string]string{
		CfgKeyEnabled:               "1",
		CfgKeyBehaviorEnabled:       "1",
		CfgKeyCardProfileEnabled:    "1",
		CfgKeyRiskScoreEnabled:      "1",
		CfgKeyRiskHighThreshold:     "70",
		CfgKeyRiskMediumThreshold:   "40",
		CfgKeyRiskCriticalThreshold: "100",
		CfgKeyAggregateInterval:     "3600",
		CfgKeyTopN:                  "20",
		CfgKeyLookbackDays:          "30",
		CfgKeyWeightHighFreq:        "25",
		CfgKeyWeightGeoAnomaly:      "20",
		CfgKeyWeightNewDevice:       "10",
		CfgKeyWeightAbnormalUA:     "15",
		CfgKeyWeightFailRateHigh:    "20",
		CfgKeyWeightMultiIP:         "15",
		CfgKeyWeightMultiDev:        "15",
		CfgKeyThresholdFailRate:     "50",
		CfgKeyThresholdMultiIPCount: "3",
		CfgKeyThresholdMultiDevCount: "5",
		CfgKeyDecayDays:             "7",
	}
	if overrides == nil {
	overrides = map[string]string{}
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
			ConfigGroup: "analysis",
		}).Error)
	}
	return config.NewConfigCache(db, rdb)
}

// newTestManager 构造测试用 Manager
func newTestManager(t *testing.T, cfgOverrides map[string]string) (*Manager, *gorm.DB) {
	t.Helper()
	db := setupTestDB(t)
	cfg := setupTestCfgCache(t, db, cfgOverrides)
	return NewManager(db, cfg), db
}

// fixedTime 返回固定时间（用于断言稳定）
func fixedTime() time.Time {
	// 2026-07-20 12:00:00 UTC
	return time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC)
}

// ============== 1. 工具函数测试 ==============

func TestDecayScore_ZeroScore(t *testing.T) {
	assert.Equal(t, 0, DecayScore(0, 3, 7))
}

func TestDecayScore_DayZero(t *testing.T) {
	// 0 天前的事件，无衰减
	assert.Equal(t, 100, DecayScore(100, 0, 7))
}

func TestDecayScore_FullDecay(t *testing.T) {
	// 7 天前的事件，decay_days=7 → 完全衰减为 0
	assert.Equal(t, 0, DecayScore(100, 7, 7))
	assert.Equal(t, 0, DecayScore(100, 10, 7)) // 超过 decayDays 也为 0
}

func TestDecayScore_PartialDecay(t *testing.T) {
	// score=70, daysSince=3, decayDays=7 → 70 - 70*3/7 = 70 - 30 = 40
	assert.Equal(t, 40, DecayScore(70, 3, 7))
	// score=100, daysSince=1, decayDays=10 → 100 - 100*1/10 = 90
	assert.Equal(t, 90, DecayScore(100, 1, 10))
}

func TestDecayScore_InvalidDecayDays(t *testing.T) {
	// decayDays=0 时不衰减
	assert.Equal(t, 50, DecayScore(50, 5, 0))
}

func TestStatDateStr(t *testing.T) {
	tm := time.Date(2026, 7, 20, 23, 59, 59, 0, time.UTC)
	assert.Equal(t, "2026-07-20", statDateStr(tm))
}

func TestStatDateRange(t *testing.T) {
	start := time.Date(2026, 7, 18, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC)
	dates := statDateRange(start, end)
	assert.Equal(t, []string{"2026-07-18", "2026-07-19", "2026-07-20"}, dates)
}

func TestStatDateRange_EndBeforeStart(t *testing.T) {
	start := time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 7, 18, 0, 0, 0, 0, time.UTC)
	assert.Nil(t, statDateRange(start, end))
}

func TestMaskCardKey_Long(t *testing.T) {
	assert.Equal(t, "ABCD****EFGH", maskCardKey("ABCDEFGH1234EFGH"))
}

func TestMaskCardKey_Short(t *testing.T) {
	assert.Equal(t, "****", maskCardKey("abc"))
	assert.Equal(t, "****", maskCardKey("12345678"))
}

func TestNormalizePage_Defaults(t *testing.T) {
	page, size := normalizePage(0, 0)
	assert.Equal(t, 1, page)
	assert.Equal(t, 20, size)
}

func TestNormalizePage_Valid(t *testing.T) {
	page, size := normalizePage(3, 50)
	assert.Equal(t, 3, page)
	assert.Equal(t, 50, size)
}

func TestNormalizePage_OverLimit(t *testing.T) {
	page, size := normalizePage(5, 200)
	assert.Equal(t, 5, page)
	assert.Equal(t, 20, size) // 超过 100 限制回退默认
}

func TestCalcRiskLevel_Low(t *testing.T) {
	m, _ := newTestManager(t, nil)
	assert.Equal(t, RiskLevelLow, m.CalcRiskLevel(context.Background(), 0))
	assert.Equal(t, RiskLevelLow, m.CalcRiskLevel(context.Background(), 39))
}

func TestCalcRiskLevel_Medium(t *testing.T) {
	m, _ := newTestManager(t, nil)
	assert.Equal(t, RiskLevelMedium, m.CalcRiskLevel(context.Background(), 40))
	assert.Equal(t, RiskLevelMedium, m.CalcRiskLevel(context.Background(), 69))
}

func TestCalcRiskLevel_High(t *testing.T) {
	m, _ := newTestManager(t, nil)
	assert.Equal(t, RiskLevelHigh, m.CalcRiskLevel(context.Background(), 70))
	assert.Equal(t, RiskLevelHigh, m.CalcRiskLevel(context.Background(), 99))
}

func TestCalcRiskLevel_Critical(t *testing.T) {
	m, _ := newTestManager(t, nil)
	assert.Equal(t, RiskLevelCritical, m.CalcRiskLevel(context.Background(), 100))
	assert.Equal(t, RiskLevelCritical, m.CalcRiskLevel(context.Background(), 200))
}

// ============== 2. 用户行为分析测试 ==============

func TestAggregateUserBehavior_NoData(t *testing.T) {
	m, _ := newTestManager(t, nil)
	n, err := m.AggregateUserBehaviorForDate(context.Background(), "2026-07-20")
	require.NoError(t, err)
	assert.Equal(t, 0, n)
}

func TestAggregateUserBehavior_NoCardID(t *testing.T) {
	m, db := newTestManager(t, nil)
	// 无 card_id 的 log_verify 应被跳过
	require.NoError(t, db.Create(&model.LogVerify{
		TenantID: 1, AppID: 10, Action: "login", Result: "success",
		ClientIP: "1.1.1.1", CreatedAt: fixedTime(),
	}).Error)
	n, err := m.AggregateUserBehaviorForDate(context.Background(), "2026-07-20")
	require.NoError(t, err)
	assert.Equal(t, 0, n)
}

func TestAggregateUserBehavior_CardNotBound(t *testing.T) {
	m, db := newTestManager(t, nil)
	// 卡密未绑终端用户：end_user_id IS NULL
	require.NoError(t, db.Create(&model.AppCard{
		BaseModel: model.BaseModel{ID: 100}, TenantID: 1, AppID: 10, Status: "active",
	}).Error)
	require.NoError(t, db.Create(&model.LogVerify{
		TenantID: 1, AppID: 10, CardID: uint64Ptr(100), Action: "verify",
		Result: "success", ClientIP: "1.1.1.1", CreatedAt: fixedTime(),
	}).Error)
	n, err := m.AggregateUserBehaviorForDate(context.Background(), "2026-07-20")
	require.NoError(t, err)
	assert.Equal(t, 0, n) // 卡密未绑用户，跳过
}

func TestAggregateUserBehavior_Success(t *testing.T) {
	m, db := newTestManager(t, nil)
	// 准备：1 个终端用户 + 1 张绑卡 + 多条 log_verify
	uid := uint64(500)
	require.NoError(t, db.Create(&model.EndUser{
		ID: uid, TenantID: 1, AppID: 10, Username: "user1", Status: "active",
	}).Error)
	require.NoError(t, db.Create(&model.AppCard{
		BaseModel: model.BaseModel{ID: 100}, TenantID: 1, AppID: 10, Status: "active", EndUserID: &uid,
	}).Error)
	// 3 条 log_verify：1 login + 2 verify，2 success + 1 fail，2 个不同 IP
	t0 := fixedTime()
	require.NoError(t, db.Create(&model.LogVerify{
		TenantID: 1, AppID: 10, CardID: uint64Ptr(100), Action: "login", Result: "success",
		ClientIP: "1.1.1.1", CreatedAt: t0,
	}).Error)
	require.NoError(t, db.Create(&model.LogVerify{
		TenantID: 1, AppID: 10, CardID: uint64Ptr(100), Action: "verify", Result: "success",
		ClientIP: "1.1.1.1", CreatedAt: t0.Add(1 * time.Hour),
	}).Error)
	require.NoError(t, db.Create(&model.LogVerify{
		TenantID: 1, AppID: 10, CardID: uint64Ptr(100), Action: "verify", Result: "fail",
		ClientIP: "2.2.2.2", CreatedAt: t0.Add(2 * time.Hour),
	}).Error)

	n, err := m.AggregateUserBehaviorForDate(context.Background(), "2026-07-20")
	require.NoError(t, err)
	assert.Equal(t, 1, n)

	// 验证聚合结果
	var profile model.UserBehaviorProfile
	require.NoError(t, db.Where("end_user_id = ?", uid).First(&profile).Error)
	assert.Equal(t, 1, profile.LoginCount)
	assert.Equal(t, 2, profile.VerifyCount)
	assert.Equal(t, 2, profile.SuccessCount)
	assert.Equal(t, 1, profile.FailCount)
	assert.Equal(t, 2, profile.DistinctIPCount) // 1.1.1.1 + 2.2.2.2
	assert.Equal(t, "2026-07-20", profile.StatDate)
}

func TestAggregateUserBehavior_Idempotent(t *testing.T) {
	m, db := newTestManager(t, nil)
	uid := uint64(501)
	require.NoError(t, db.Create(&model.EndUser{ID: uid, TenantID: 1, AppID: 10, Username: "u", Status: "active"}).Error)
	require.NoError(t, db.Create(&model.AppCard{BaseModel: model.BaseModel{ID: 101}, TenantID: 1, AppID: 10, Status: "active", EndUserID: &uid}).Error)
	require.NoError(t, db.Create(&model.LogVerify{
		TenantID: 1, AppID: 10, CardID: uint64Ptr(101), Action: "login", Result: "success",
		ClientIP: "1.1.1.1", CreatedAt: fixedTime(),
	}).Error)

	// 第一次聚合
	n1, err := m.AggregateUserBehaviorForDate(context.Background(), "2026-07-20")
	require.NoError(t, err)
	assert.Equal(t, 1, n1)

	// 第二次聚合（幂等：不应产生重复记录）
	n2, err := m.AggregateUserBehaviorForDate(context.Background(), "2026-07-20")
	require.NoError(t, err)
	assert.Equal(t, 1, n2)

	var count int64
	db.Model(&model.UserBehaviorProfile{}).Where("end_user_id = ?", uid).Count(&count)
	assert.Equal(t, int64(1), count) // 仍只有 1 条记录
}

func TestGetBehaviorOverview(t *testing.T) {
	m, db := newTestManager(t, nil)
	uid := uint64(502)
	require.NoError(t, db.Create(&model.EndUser{ID: uid, TenantID: 1, AppID: 10, Username: "u", Status: "active"}).Error)
	require.NoError(t, db.Create(&model.AppCard{BaseModel: model.BaseModel{ID: 102}, TenantID: 1, AppID: 10, Status: "active", EndUserID: &uid}).Error)
	require.NoError(t, db.Create(&model.LogVerify{
		TenantID: 1, AppID: 10, CardID: uint64Ptr(102), Action: "login", Result: "success",
		ClientIP: "1.1.1.1", CreatedAt: fixedTime(),
	}).Error)
	_, err := m.AggregateUserBehaviorForDate(context.Background(), "2026-07-20")
	require.NoError(t, err)

	ov, err := m.GetBehaviorOverview(context.Background(), Filter{})
	require.NoError(t, err)
	assert.Equal(t, int64(1), ov.TotalActiveUsers)
	assert.Equal(t, int64(1), ov.TotalLoginCount)
	assert.Equal(t, 30, ov.LookbackDays)
}

func TestListUserBehaviors(t *testing.T) {
	m, db := newTestManager(t, nil)
	uid := uint64(503)
	require.NoError(t, db.Create(&model.EndUser{ID: uid, TenantID: 1, AppID: 10, Username: "user503", Status: "active"}).Error)
	require.NoError(t, db.Create(&model.AppCard{BaseModel: model.BaseModel{ID: 103}, TenantID: 1, AppID: 10, Status: "active", EndUserID: &uid}).Error)
	require.NoError(t, db.Create(&model.LogVerify{
		TenantID: 1, AppID: 10, CardID: uint64Ptr(103), Action: "verify", Result: "success",
		ClientIP: "1.1.1.1", CreatedAt: fixedTime(),
	}).Error)
	_, err := m.AggregateUserBehaviorForDate(context.Background(), "2026-07-20")
	require.NoError(t, err)

	users, total, err := m.ListUserBehaviors(context.Background(), Filter{Page: 1, PageSize: 10})
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	require.Len(t, users, 1)
	assert.Equal(t, uid, users[0].EndUserID)
	assert.Equal(t, "user503", users[0].Username)
	assert.Equal(t, int64(1), users[0].VerifyCount)
}

func TestGetUserBehaviorDetail(t *testing.T) {
	m, db := newTestManager(t, nil)
	uid := uint64(504)
	require.NoError(t, db.Create(&model.EndUser{ID: uid, TenantID: 1, AppID: 10, Username: "d_user", Status: "active"}).Error)
	require.NoError(t, db.Create(&model.AppCard{BaseModel: model.BaseModel{ID: 104}, TenantID: 1, AppID: 10, Status: "active", EndUserID: &uid}).Error)
	require.NoError(t, db.Create(&model.LogVerify{
		TenantID: 1, AppID: 10, CardID: uint64Ptr(104), Action: "login", Result: "success",
		ClientIP: "1.1.1.1", CreatedAt: fixedTime(),
	}).Error)
	_, err := m.AggregateUserBehaviorForDate(context.Background(), "2026-07-20")
	require.NoError(t, err)

	detail, err := m.GetUserBehaviorDetail(context.Background(), uid, 30)
	require.NoError(t, err)
	assert.Equal(t, uid, detail.Summary.EndUserID)
	assert.Equal(t, "d_user", detail.Summary.Username)
	require.Len(t, detail.Daily, 1)
	assert.Equal(t, "2026-07-20", detail.Daily[0].StatDate)
	assert.Equal(t, 1, detail.Daily[0].LoginCount)
}

func TestGetBehaviorTrend(t *testing.T) {
	m, db := newTestManager(t, nil)
	uid := uint64(505)
	require.NoError(t, db.Create(&model.EndUser{ID: uid, TenantID: 1, AppID: 10, Username: "u", Status: "active"}).Error)
	require.NoError(t, db.Create(&model.AppCard{BaseModel: model.BaseModel{ID: 105}, TenantID: 1, AppID: 10, Status: "active", EndUserID: &uid}).Error)
	require.NoError(t, db.Create(&model.LogVerify{
		TenantID: 1, AppID: 10, CardID: uint64Ptr(105), Action: "login", Result: "success",
		ClientIP: "1.1.1.1", CreatedAt: fixedTime(),
	}).Error)
	_, err := m.AggregateUserBehaviorForDate(context.Background(), "2026-07-20")
	require.NoError(t, err)

	trend, err := m.GetBehaviorTrend(context.Background(), Filter{}, 30)
	require.NoError(t, err)
	require.Len(t, trend, 1)
	assert.Equal(t, "2026-07-20", trend[0].StatDate)
	assert.Equal(t, int64(1), trend[0].ActiveUsers)
	assert.Equal(t, int64(1), trend[0].LoginCount)
}

// ============== 3. 卡密画像测试 ==============

func TestAggregateCardProfile_NoCardID(t *testing.T) {
	m, db := newTestManager(t, nil)
	// log_verify 无 card_id，应跳过
	require.NoError(t, db.Create(&model.LogVerify{
		TenantID: 1, AppID: 10, Action: "login", Result: "success",
		ClientIP: "1.1.1.1", CreatedAt: fixedTime(),
	}).Error)
	n, err := m.AggregateCardProfileForDate(context.Background(), "2026-07-20")
	require.NoError(t, err)
	assert.Equal(t, 0, n)
}

func TestAggregateCardProfile_Success(t *testing.T) {
	m, db := newTestManager(t, nil)
	require.NoError(t, db.Create(&model.AppCard{
		BaseModel: model.BaseModel{ID: 200}, TenantID: 1, AppID: 10, Status: "active", CardKey: "CARD1234CARD5678",
	}).Error)
	t0 := fixedTime()
	require.NoError(t, db.Create(&model.LogVerify{
		TenantID: 1, AppID: 10, CardID: uint64Ptr(200), Action: "verify", Result: "success",
		ClientIP: "1.1.1.1", CreatedAt: t0,
	}).Error)
	require.NoError(t, db.Create(&model.LogVerify{
		TenantID: 1, AppID: 10, CardID: uint64Ptr(200), Action: "heartbeat", Result: "success",
		ClientIP: "1.1.1.1", CreatedAt: t0.Add(1 * time.Hour),
	}).Error)
	require.NoError(t, db.Create(&model.LogVerify{
		TenantID: 1, AppID: 10, CardID: uint64Ptr(200), Action: "verify", Result: "device_mismatch",
		ClientIP: "2.2.2.2", CreatedAt: t0.Add(2 * time.Hour),
	}).Error)

	n, err := m.AggregateCardProfileForDate(context.Background(), "2026-07-20")
	require.NoError(t, err)
	assert.Equal(t, 1, n)

	var profile model.CardUsageProfile
	require.NoError(t, db.Where("card_id = ?", 200).First(&profile).Error)
	assert.Equal(t, 2, profile.VerifyCount)
	assert.Equal(t, 1, profile.HeartbeatCount)
	assert.Equal(t, 1, profile.DeviceMismatchCount)
	assert.Equal(t, 2, profile.DistinctIPCount)
}

func TestGetCardProfileOverview(t *testing.T) {
	m, db := newTestManager(t, nil)
	require.NoError(t, db.Create(&model.AppCard{BaseModel: model.BaseModel{ID: 201}, TenantID: 1, AppID: 10, Status: "active"}).Error)
	require.NoError(t, db.Create(&model.LogVerify{
		TenantID: 1, AppID: 10, CardID: uint64Ptr(201), Action: "verify", Result: "success",
		ClientIP: "1.1.1.1", CreatedAt: fixedTime(),
	}).Error)
	_, err := m.AggregateCardProfileForDate(context.Background(), "2026-07-20")
	require.NoError(t, err)

	ov, err := m.GetCardProfileOverview(context.Background(), Filter{})
	require.NoError(t, err)
	assert.Equal(t, int64(1), ov.TotalActiveCards)
	assert.Equal(t, int64(1), ov.TotalVerifyCount)
}

func TestListCardProfiles_MaskCardKey(t *testing.T) {
	m, db := newTestManager(t, nil)
	require.NoError(t, db.Create(&model.AppCard{
		BaseModel: model.BaseModel{ID: 202}, TenantID: 1, AppID: 10, Status: "active", CardKey: "ABCDEFGHIJKLMNOP",
	}).Error)
	require.NoError(t, db.Create(&model.LogVerify{
		TenantID: 1, AppID: 10, CardID: uint64Ptr(202), Action: "verify", Result: "success",
		ClientIP: "1.1.1.1", CreatedAt: fixedTime(),
	}).Error)
	_, err := m.AggregateCardProfileForDate(context.Background(), "2026-07-20")
	require.NoError(t, err)

	cards, total, err := m.ListCardProfiles(context.Background(), Filter{Page: 1, PageSize: 10})
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	require.Len(t, cards, 1)
	assert.Equal(t, uint64(202), cards[0].CardID)
	assert.Equal(t, "ABCD****MNOP", cards[0].CardKey) // 脱敏
	assert.Equal(t, "active", cards[0].Status)
}

func TestGetCardProfileDetail(t *testing.T) {
	m, db := newTestManager(t, nil)
	require.NoError(t, db.Create(&model.AppCard{
		BaseModel: model.BaseModel{ID: 203}, TenantID: 1, AppID: 10, Status: "active", CardKey: "KEY1234KEY5678",
	}).Error)
	require.NoError(t, db.Create(&model.LogVerify{
		TenantID: 1, AppID: 10, CardID: uint64Ptr(203), Action: "verify", Result: "success",
		ClientIP: "1.1.1.1", CreatedAt: fixedTime(),
	}).Error)
	_, err := m.AggregateCardProfileForDate(context.Background(), "2026-07-20")
	require.NoError(t, err)

	detail, err := m.GetCardProfileDetail(context.Background(), 203, 30)
	require.NoError(t, err)
	assert.Equal(t, uint64(203), detail.Summary.CardID)
	assert.Equal(t, "KEY1****5678", detail.Summary.CardKey)
	require.Len(t, detail.Daily, 1)
	assert.Equal(t, 1, detail.Daily[0].VerifyCount)
}

func TestGetCardProfileTrend(t *testing.T) {
	m, db := newTestManager(t, nil)
	require.NoError(t, db.Create(&model.AppCard{BaseModel: model.BaseModel{ID: 204}, TenantID: 1, AppID: 10, Status: "active"}).Error)
	require.NoError(t, db.Create(&model.LogVerify{
		TenantID: 1, AppID: 10, CardID: uint64Ptr(204), Action: "verify", Result: "success",
		ClientIP: "1.1.1.1", CreatedAt: fixedTime(),
	}).Error)
	_, err := m.AggregateCardProfileForDate(context.Background(), "2026-07-20")
	require.NoError(t, err)

	trend, err := m.GetCardProfileTrend(context.Background(), Filter{}, 30)
	require.NoError(t, err)
	require.Len(t, trend, 1)
	assert.Equal(t, int64(1), trend[0].ActiveCards)
	assert.Equal(t, int64(1), trend[0].VerifyCount)
}

// ============== 4. 风险用户测试 ==============

func TestReevaluateUserRisk_NoEvents(t *testing.T) {
	m, _ := newTestManager(t, nil)
	// 无 risk_event 也无绑卡：评分应为 0，等级 low
	r, err := m.ReevaluateUserRiskScore(context.Background(), UserTypeEndUser, 600)
	require.NoError(t, err)
	assert.Equal(t, 0, r.DecayedScore)
	assert.Equal(t, RiskLevelLow, r.RiskLevel)
}

func TestReevaluateUserRisk_WithRiskEvents(t *testing.T) {
	m, db := newTestManager(t, nil)
	// 1 次 high_frequency + 1 次 geo_login → 25 + 20 = 45 → medium
	now := time.Now()
	require.NoError(t, db.Create(&model.RiskEvent{
		RuleType: "high_frequency", RuleName: "高频请求",
		UserType: UserTypeEndUser, UserID: 601, Username: "u1",
		ClientIP: "1.1.1.1", RiskScore: 25, ActionTaken: "alert",
		Detail: "{}", CreatedAt: now,
	}).Error)
	require.NoError(t, db.Create(&model.RiskEvent{
		RuleType: "geo_login", RuleName: "异地登录",
		UserType: UserTypeEndUser, UserID: 601, Username: "u1",
		ClientIP: "1.1.1.1", RiskScore: 20, ActionTaken: "alert",
		Detail: "{}", CreatedAt: now,
	}).Error)

	r, err := m.ReevaluateUserRiskScore(context.Background(), UserTypeEndUser, 601)
	require.NoError(t, err)
	assert.Equal(t, 45, r.RawScore)
	assert.Equal(t, 45, r.DecayedScore) // 0 天前，无衰减
	assert.Equal(t, RiskLevelMedium, r.RiskLevel)
	assert.Equal(t, 1, r.HighFreqHits)
	assert.Equal(t, 1, r.GeoAnomalyHits)
}

func TestReevaluateUserRisk_Decay(t *testing.T) {
	m, db := newTestManager(t, nil)
	// 3 天前的事件：score=25, daysSince=3, decayDays=7 → 25 - 25*3/7 = 25-10 = 15
	oldTime := time.Now().AddDate(0, 0, -3)
	require.NoError(t, db.Create(&model.RiskEvent{
		RuleType: "high_frequency", RuleName: "高频",
		UserType: UserTypeEndUser, UserID: 602, Username: "u2",
		ClientIP: "1.1.1.1", RiskScore: 25, ActionTaken: "alert",
		Detail: "{}", CreatedAt: oldTime,
	}).Error)

	r, err := m.ReevaluateUserRiskScore(context.Background(), UserTypeEndUser, 602)
	require.NoError(t, err)
	assert.Equal(t, 25, r.RawScore)
	assert.Equal(t, 15, r.DecayedScore) // 衰减后
}

func TestReevaluateUserRisk_EndUserAnomalies_FailRate(t *testing.T) {
	m, db := newTestManager(t, nil)
	uid := uint64(603)
	require.NoError(t, db.Create(&model.EndUser{ID: uid, TenantID: 1, AppID: 10, Username: "u", Status: "active"}).Error)
	require.NoError(t, db.Create(&model.AppCard{BaseModel: model.BaseModel{ID: 300}, TenantID: 1, AppID: 10, Status: "active", EndUserID: &uid}).Error)
	// 10 条 log_verify，6 fail + 4 success → 失败率 60% > 50% → +20
	now := time.Now()
	for i := 0; i < 4; i++ {
		require.NoError(t, db.Create(&model.LogVerify{
			TenantID: 1, AppID: 10, CardID: uint64Ptr(300), Action: "verify", Result: "success",
			ClientIP: "1.1.1.1", CreatedAt: now.Add(-time.Duration(i) * time.Minute),
		}).Error)
	}
	for i := 0; i < 6; i++ {
		require.NoError(t, db.Create(&model.LogVerify{
			TenantID: 1, AppID: 10, CardID: uint64Ptr(300), Action: "verify", Result: "fail",
			ClientIP: "1.1.1.1", CreatedAt: now.Add(-time.Duration(i+10) * time.Minute),
		}).Error)
	}

	r, err := m.ReevaluateUserRiskScore(context.Background(), UserTypeEndUser, uid)
	require.NoError(t, err)
	assert.Equal(t, 1, r.FailRateHigh) // 触发失败率告警
	assert.Equal(t, 20, r.DecayedScore) // 20 × 1 = 20
	assert.Equal(t, RiskLevelLow, r.RiskLevel) // 20 < 40 → low
}

func TestReevaluateUserRisk_EndUserAnomalies_MultiIP(t *testing.T) {
	m, db := newTestManager(t, nil)
	uid := uint64(604)
	require.NoError(t, db.Create(&model.EndUser{ID: uid, TenantID: 1, AppID: 10, Username: "u", Status: "active"}).Error)
	require.NoError(t, db.Create(&model.AppCard{BaseModel: model.BaseModel{ID: 301}, TenantID: 1, AppID: 10, Status: "active", EndUserID: &uid}).Error)
	// 3 个不同 IP → 触发 multi_ip
	now := time.Now()
	for _, ip := range []string{"1.1.1.1", "2.2.2.2", "3.3.3.3"} {
		require.NoError(t, db.Create(&model.LogVerify{
			TenantID: 1, AppID: 10, CardID: uint64Ptr(301), Action: "verify", Result: "success",
			ClientIP: ip, CreatedAt: now,
		}).Error)
	}

	r, err := m.ReevaluateUserRiskScore(context.Background(), UserTypeEndUser, uid)
	require.NoError(t, err)
	assert.Equal(t, 1, r.MultiIPHits) // 触发多 IP
	assert.Equal(t, 15, r.DecayedScore) // 15 × 1 = 15
}

func TestReevaluateUserRisk_AutoBan(t *testing.T) {
	m, db := newTestManager(t, nil)
	// 4 次 high_frequency → 4 × 25 = 100 → 达到 critical_threshold
	now := time.Now()
	for i := 0; i < 4; i++ {
		require.NoError(t, db.Create(&model.RiskEvent{
			RuleType: "high_frequency", RuleName: "高频",
			UserType: UserTypeEndUser, UserID: 605, Username: "u",
			ClientIP: "1.1.1.1", RiskScore: 25, ActionTaken: "alert",
			Detail: "{}", CreatedAt: now,
		}).Error)
	}

	r, err := m.ReevaluateUserRiskScore(context.Background(), UserTypeEndUser, 605)
	require.NoError(t, err)
	assert.Equal(t, 100, r.DecayedScore)
	assert.Equal(t, RiskLevelCritical, r.RiskLevel)

	// 验证 user_risk_score 表中 banned=true
	var score model.UserRiskScore
	require.NoError(t, db.Where("user_type = ? AND user_id = ?", UserTypeEndUser, 605).First(&score).Error)
	assert.True(t, score.Banned)
	assert.Equal(t, "评分达到致命阈值，自动封禁候选", score.BannedReason)
}

func TestReevaluateAllRiskScores(t *testing.T) {
	m, db := newTestManager(t, nil)
	// 创建 2 个用户的风控事件
	now := time.Now()
	for _, uid := range []uint64{606, 607} {
		require.NoError(t, db.Create(&model.RiskEvent{
			RuleType: "high_frequency", RuleName: "高频",
			UserType: UserTypeEndUser, UserID: uid, Username: "u",
			ClientIP: "1.1.1.1", RiskScore: 25, ActionTaken: "alert",
			Detail: "{}", CreatedAt: now,
		}).Error)
	}

	count, err := m.ReevaluateAllRiskScores(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 2, count)

	// 验证 user_risk_score 表中有 2 条记录
	var total int64
	db.Model(&model.UserRiskScore{}).Count(&total)
	assert.Equal(t, int64(2), total)
}

func TestGetRiskUserOverview(t *testing.T) {
	m, db := newTestManager(t, nil)
	// 预置 1 个 high + 1 个 critical 用户
	require.NoError(t, db.Create(&model.UserRiskScore{
		UserType: UserTypeEndUser, UserID: 700, RiskScore: 75, RiskLevel: RiskLevelHigh,
	}).Error)
	require.NoError(t, db.Create(&model.UserRiskScore{
		UserType: UserTypeEndUser, UserID: 701, RiskScore: 100, RiskLevel: RiskLevelCritical, Banned: true,
	}).Error)

	ov, err := m.GetRiskUserOverview(context.Background(), Filter{})
	require.NoError(t, err)
	assert.Equal(t, int64(2), ov.TotalUsers)
	assert.Equal(t, int64(1), ov.HighRiskCount)
	assert.Equal(t, int64(1), ov.CriticalCount)
	assert.Equal(t, int64(1), ov.BannedCount)
}

func TestListRiskUsers_FilterByLevel(t *testing.T) {
	m, db := newTestManager(t, nil)
	require.NoError(t, db.Create(&model.UserRiskScore{
		UserType: UserTypeEndUser, UserID: 710, RiskScore: 75, RiskLevel: RiskLevelHigh,
	}).Error)
	require.NoError(t, db.Create(&model.UserRiskScore{
		UserType: UserTypeEndUser, UserID: 711, RiskScore: 30, RiskLevel: RiskLevelLow,
	}).Error)

	users, total, err := m.ListRiskUsers(context.Background(), Filter{Level: RiskLevelHigh, Page: 1, PageSize: 10})
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	require.Len(t, users, 1)
	assert.Equal(t, uint64(710), users[0].UserID)
	assert.Equal(t, RiskLevelHigh, users[0].RiskLevel)
}

func TestGetRiskUserDetail(t *testing.T) {
	m, db := newTestManager(t, nil)
	require.NoError(t, db.Create(&model.UserRiskScore{
		UserType: UserTypeEndUser, UserID: 720, Username: "detail_user",
		RiskScore: 50, RiskLevel: RiskLevelMedium, EventCount: 3,
	}).Error)
	require.NoError(t, db.Create(&model.RiskEvent{
		RuleType: "high_frequency", RuleName: "高频",
		UserType: UserTypeEndUser, UserID: 720, Username: "detail_user",
		ClientIP: "1.1.1.1", RiskScore: 25, ActionTaken: "alert",
		Detail: "{}", CreatedAt: time.Now(),
	}).Error)

	detail, err := m.GetRiskUserDetail(context.Background(), UserTypeEndUser, 720, 20)
	require.NoError(t, err)
	assert.Equal(t, uint64(720), detail.Summary.UserID)
	assert.Equal(t, "detail_user", detail.Summary.Username)
	assert.Equal(t, RiskLevelMedium, detail.Summary.RiskLevel)
	require.Len(t, detail.RecentEvents, 1)
	assert.Equal(t, "high_frequency", detail.RecentEvents[0].RuleType)
}

func TestBanUser_ExistingRecord(t *testing.T) {
	m, db := newTestManager(t, nil)
	require.NoError(t, db.Create(&model.UserRiskScore{
		UserType: UserTypeEndUser, UserID: 730, RiskScore: 30, RiskLevel: RiskLevelLow,
	}).Error)

	require.NoError(t, m.BanUser(context.Background(), UserTypeEndUser, 730, "手动封禁-违规"))

	var score model.UserRiskScore
	require.NoError(t, db.Where("user_type = ? AND user_id = ?", UserTypeEndUser, 730).First(&score).Error)
	assert.True(t, score.Banned)
	assert.Equal(t, "手动封禁-违规", score.BannedReason)
	assert.NotNil(t, score.BannedAt)
}

func TestBanUser_NewRecord(t *testing.T) {
	m, _ := newTestManager(t, nil)
	// 用户无评分记录，直接封禁
	require.NoError(t, m.BanUser(context.Background(), UserTypeEndUser, 731, "黑名单"))

	var score model.UserRiskScore
	require.NoError(t, m.db.Where("user_type = ? AND user_id = ?", UserTypeEndUser, 731).First(&score).Error)
	assert.True(t, score.Banned)
	assert.Equal(t, "黑名单", score.BannedReason)
}

func TestUnbanUser(t *testing.T) {
	m, db := newTestManager(t, nil)
	now := time.Now()
	require.NoError(t, db.Create(&model.UserRiskScore{
		UserType: UserTypeEndUser, UserID: 740, RiskScore: 50, RiskLevel: RiskLevelMedium,
		Banned: true, BannedReason: "违规", BannedAt: &now,
	}).Error)

	require.NoError(t, m.UnbanUser(context.Background(), UserTypeEndUser, 740))

	var score model.UserRiskScore
	require.NoError(t, db.Where("user_type = ? AND user_id = ?", UserTypeEndUser, 740).First(&score).Error)
	assert.False(t, score.Banned)
	assert.Equal(t, "", score.BannedReason)
	assert.Nil(t, score.BannedAt)
	// 风险评分保留
	assert.Equal(t, 50, score.RiskScore)
}

// ============== 5. Worker 测试 ==============

func TestRunAggregationOnceSync(t *testing.T) {
	m, db := newTestManager(t, nil)
	// 准备数据：1 个用户 + 1 张卡 + 1 条 log_verify + 1 条 risk_event
	uid := uint64(800)
	require.NoError(t, db.Create(&model.EndUser{ID: uid, TenantID: 1, AppID: 10, Username: "w", Status: "active"}).Error)
	require.NoError(t, db.Create(&model.AppCard{BaseModel: model.BaseModel{ID: 400}, TenantID: 1, AppID: 10, Status: "active", EndUserID: &uid}).Error)
	require.NoError(t, db.Create(&model.LogVerify{
		TenantID: 1, AppID: 10, CardID: uint64Ptr(400), Action: "login", Result: "success",
		ClientIP: "1.1.1.1", CreatedAt: time.Now(),
	}).Error)
	require.NoError(t, db.Create(&model.RiskEvent{
		RuleType: "geo_login", RuleName: "异地",
		UserType: UserTypeEndUser, UserID: uid, Username: "w",
		ClientIP: "1.1.1.1", RiskScore: 20, ActionTaken: "alert",
		Detail: "{}", CreatedAt: time.Now(),
	}).Error)

	// 执行聚合
	users, cards, risk, err := m.RunAggregationOnceSync(context.Background())
	require.NoError(t, err)
	assert.Greater(t, users, 0) // 至少聚合 1 个用户
	assert.Greater(t, cards, 0) // 至少聚合 1 张卡
	assert.Greater(t, risk, 0)  // 至少重算 1 个用户

	// 验证 user_behavior_profile 表
	var behaviorCount int64
	db.Model(&model.UserBehaviorProfile{}).Where("end_user_id = ?", uid).Count(&behaviorCount)
	assert.Equal(t, int64(1), behaviorCount)

	// 验证 card_usage_profile 表
	var cardCount int64
	db.Model(&model.CardUsageProfile{}).Where("card_id = ?", 400).Count(&cardCount)
	assert.Equal(t, int64(1), cardCount)

	// 验证 user_risk_score 表
	var riskScore model.UserRiskScore
	require.NoError(t, db.Where("user_type = ? AND user_id = ?", UserTypeEndUser, uid).First(&riskScore).Error)
	assert.Equal(t, 20, riskScore.RiskScore) // geo_login × 1 = 20
	assert.Equal(t, 1, riskScore.GeoAnomalyHits)
}

// ============== 辅助函数 ==============

// uint64Ptr 返回 *uint64
func uint64Ptr(v uint64) *uint64 {
	return &v
}
