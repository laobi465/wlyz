// Package risk v0.4.0 高级安全风控规则引擎单元测试
// 严格遵循铁律 06：所有断言基于已知固定输入，无随机/不确定性
package risk

import (
	"context"
	"encoding/json"
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

func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:risk_test_%d?mode=memory&cache=shared", time.Now().UnixNano())), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&model.RiskRule{},
		&model.RiskEvent{},
		&model.LoginGeoAlert{},
		&model.RefreshTokenDevice{},
		&model.SysConfig{},
		&model.AppDevice{},
	))
	db.Exec("DELETE FROM risk_rule")
	db.Exec("DELETE FROM risk_event")
	db.Exec("DELETE FROM login_geo_alert")
	db.Exec("DELETE FROM refresh_token_device")
	db.Exec("DELETE FROM sys_config")
	return db
}

func setupCfgCache(t *testing.T, db *gorm.DB, overrides map[string]string) (*config.ConfigCache, *miniredis.Miniredis) {
	t.Helper()
	mr, err := miniredis.Run()
	require.NoError(t, err)
	t.Cleanup(mr.Close)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	defaults := map[string]string{
		CfgKeyEngineEnabled:        "1",
		CfgKeyEngineScoreThreshold: "80",
		CfgKeyEngineDefaultAction:  "alert",
		CfgKeyGeoLoginEnabled:      "1",
		CfgKeyGeoLoginIPv4Prefix:   "24",
		CfgKeyGeoLoginIPv6Prefix:   "64",
		CfgKeyGeoLoginNotifyChans:  "inapp,email",
		CfgKeyNewDeviceEnabled:     "1",
		CfgKeyAbnormalUAEnabled:    "1",
		CfgKeyAbnormalTimeEnabled:  "0",
		CfgKeyAbnormalTimeStart:    "02:00",
		CfgKeyAbnormalTimeEnd:      "05:00",
	}
	if overrides != nil {
		for k, v := range overrides {
			defaults[k] = v
		}
	}
	for k, v := range defaults {
		require.NoError(t, db.Create(&model.SysConfig{
			ConfigKey:   k,
			ConfigValue: v,
			ConfigType:  "string",
			ConfigGroup: "security",
		}).Error)
	}
	cfg := config.NewConfigCache(db, rdb)
	require.NoError(t, cfg.Preload(context.Background()))
	return cfg, mr
}

// seedBuiltinRules 写入 5 条内置规则（status=active，abnormal_time=disabled）
func seedBuiltinRules(t *testing.T, db *gorm.DB) {
	t.Helper()
	rules := []model.RiskRule{
		{Name: "异地登录检测", RuleType: RuleTypeGeoLogin, Condition: `{"ipv4_prefix":24,"ipv6_prefix":64}`, Score: 60, Action: ActionAlert, Priority: 10, Status: StatusActive, CreatedBy: "system"},
		{Name: "新设备登录检测", RuleType: RuleTypeNewDevice, Condition: `{"check_fields":["hwid"]}`, Score: 40, Action: ActionAlert, Priority: 20, Status: StatusActive, CreatedBy: "system"},
		{Name: "异常 UA 检测", RuleType: RuleTypeAbnormalUA, Condition: `{"block_bots":true}`, Score: 30, Action: ActionAlert, Priority: 30, Status: StatusActive, CreatedBy: "system"},
		{Name: "异常时段检测", RuleType: RuleTypeAbnormalTime, Condition: `{"start":"02:00","end":"05:00"}`, Score: 20, Action: ActionAlert, Priority: 40, Status: StatusDisabled, CreatedBy: "system"},
		{Name: "高频请求检测", RuleType: RuleTypeHighFrequency, Condition: `{"window_seconds":60,"threshold":10}`, Score: 50, Action: ActionChallenge, Priority: 50, Status: StatusActive, CreatedBy: "system"},
	}
	for i := range rules {
		require.NoError(t, db.Create(&rules[i]).Error)
	}
}

// ============== 1. NetworkOf IP 网段计算 ==============

func TestNetworkOf_IPv4(t *testing.T) {
	net, ok := NetworkOf("1.2.3.4", 24, 64)
	require.True(t, ok)
	assert.Equal(t, "1.2.3.0/24", net)
}

func TestNetworkOf_IPv4_DifferentPrefix(t *testing.T) {
	net, ok := NetworkOf("1.2.3.4", 16, 64)
	require.True(t, ok)
	assert.Equal(t, "1.2.0.0/16", net)
}

func TestNetworkOf_IPv6(t *testing.T) {
	net, ok := NetworkOf("2001:db8::1", 24, 64)
	require.True(t, ok)
	assert.Equal(t, "2001:db8::/64", net)
}

func TestNetworkOf_InvalidIP(t *testing.T) {
	_, ok := NetworkOf("invalid-ip", 24, 64)
	assert.False(t, ok)
}

func TestNetworkOf_SameNetwork(t *testing.T) {
	net1, _ := NetworkOf("1.2.3.4", 24, 64)
	net2, _ := NetworkOf("1.2.3.100", 24, 64)
	assert.Equal(t, net1, net2, "同 /24 应视为同网段")
}

func TestNetworkOf_DifferentNetwork(t *testing.T) {
	net1, _ := NetworkOf("1.2.3.4", 24, 64)
	net2, _ := NetworkOf("1.2.4.4", 24, 64)
	assert.NotEqual(t, net1, net2, "不同 /24 应视为不同网段")
}

// ============== 2. parseHHMM 时间解析 ==============

func TestParseHHMM_Valid(t *testing.T) {
	min, ok := parseHHMM("02:30")
	require.True(t, ok)
	assert.Equal(t, 150, min)
}

func TestParseHHMM_Invalid(t *testing.T) {
	_, ok := parseHHMM("25:00")
	assert.False(t, ok)
	_, ok = parseHHMM("12:60")
	assert.False(t, ok)
	_, ok = parseHHMM("invalid")
	assert.False(t, ok)
}

// ============== 3. actionLevel 动作级别 ==============

func TestActionLevel(t *testing.T) {
	assert.Equal(t, 3, actionLevel(ActionBlock))
	assert.Equal(t, 2, actionLevel(ActionChallenge))
	assert.Equal(t, 1, actionLevel(ActionAlert))
	assert.Equal(t, 0, actionLevel("unknown"))
}

// ============== 4. 异常 UA 检测 ==============

func TestEvalAbnormalUA_Empty(t *testing.T) {
	db := setupTestDB(t)
	cfg, _ := setupCfgCache(t, db, nil)
	seedBuiltinRules(t, db)
	m := NewManager(db, cfg)

	var rule model.RiskRule
	require.NoError(t, db.Where("rule_type = ?", RuleTypeAbnormalUA).First(&rule).Error)

	hit, detail, score := m.evalAbnormalUA(context.Background(), rule, EvalContext{UserAgent: ""})
	assert.True(t, hit)
	assert.Equal(t, rule.Score, score)
	var d map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(detail), &d))
	assert.Equal(t, "empty_ua", d["reason"])
}

func TestEvalAbnormalUA_Curl(t *testing.T) {
	db := setupTestDB(t)
	cfg, _ := setupCfgCache(t, db, nil)
	seedBuiltinRules(t, db)
	m := NewManager(db, cfg)

	var rule model.RiskRule
	require.NoError(t, db.Where("rule_type = ?", RuleTypeAbnormalUA).First(&rule).Error)

	hit, _, _ := m.evalAbnormalUA(context.Background(), rule, EvalContext{UserAgent: "curl/7.81.0"})
	assert.True(t, hit, "curl 应触发异常 UA")
}

func TestEvalAbnormalUA_PythonRequests(t *testing.T) {
	db := setupTestDB(t)
	cfg, _ := setupCfgCache(t, db, nil)
	seedBuiltinRules(t, db)
	m := NewManager(db, cfg)
	var rule model.RiskRule
	require.NoError(t, db.Where("rule_type = ?", RuleTypeAbnormalUA).First(&rule).Error)

	hit, _, _ := m.evalAbnormalUA(context.Background(), rule, EvalContext{UserAgent: "python-requests/2.28.1"})
	assert.True(t, hit, "python-requests 应触发异常 UA")
}

func TestEvalAbnormalUA_NormalBrowser(t *testing.T) {
	db := setupTestDB(t)
	cfg, _ := setupCfgCache(t, db, nil)
	seedBuiltinRules(t, db)
	m := NewManager(db, cfg)
	var rule model.RiskRule
	require.NoError(t, db.Where("rule_type = ?", RuleTypeAbnormalUA).First(&rule).Error)

	hit, _, _ := m.evalAbnormalUA(context.Background(), rule, EvalContext{
		UserAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
	})
	assert.False(t, hit, "正常 Chrome UA 不应触发")
}

// ============== 5. 异常时段检测 ==============

func TestEvalAbnormalTime_InRange(t *testing.T) {
	db := setupTestDB(t)
	cfg, _ := setupCfgCache(t, db, nil)
	seedBuiltinRules(t, db)
	m := NewManager(db, cfg)
	var rule model.RiskRule
	require.NoError(t, db.Where("rule_type = ?", RuleTypeAbnormalTime).First(&rule).Error)

	// 03:30 在 02:00-05:00 范围内
	ec := EvalContext{OccurredAt: time.Date(2026, 7, 20, 3, 30, 0, 0, time.Local)}
	hit, _, _ := m.evalAbnormalTime(context.Background(), rule, ec)
	assert.True(t, hit)
}

func TestEvalAbnormalTime_OutOfRange(t *testing.T) {
	db := setupTestDB(t)
	cfg, _ := setupCfgCache(t, db, nil)
	seedBuiltinRules(t, db)
	m := NewManager(db, cfg)
	var rule model.RiskRule
	require.NoError(t, db.Where("rule_type = ?", RuleTypeAbnormalTime).First(&rule).Error)

	// 14:00 不在 02:00-05:00 范围内
	ec := EvalContext{OccurredAt: time.Date(2026, 7, 20, 14, 0, 0, 0, time.Local)}
	hit, _, _ := m.evalAbnormalTime(context.Background(), rule, ec)
	assert.False(t, hit)
}

func TestEvalAbnormalTime_CrossMidnight(t *testing.T) {
	db := setupTestDB(t)
	// 覆盖规则 condition：23:00-04:00（跨午夜）
	cfg, _ := setupCfgCache(t, db, nil)
	// 先 seed 默认规则，再更新 condition
	seedBuiltinRules(t, db)
	require.NoError(t, db.Model(&model.RiskRule{}).
		Where("rule_type = ?", RuleTypeAbnormalTime).
		Update("condition", `{"start":"23:00","end":"04:00"}`).Error)
	m := NewManager(db, cfg)

	var rule model.RiskRule
	require.NoError(t, db.Where("rule_type = ?", RuleTypeAbnormalTime).First(&rule).Error)

	// 23:30 在 23:00-04:00 范围内
	ec1 := EvalContext{OccurredAt: time.Date(2026, 7, 20, 23, 30, 0, 0, time.Local)}
	hit, _, _ := m.evalAbnormalTime(context.Background(), rule, ec1)
	assert.True(t, hit, "23:30 应在 23:00-04:00 范围内")

	// 02:00 也在范围内
	ec2 := EvalContext{OccurredAt: time.Date(2026, 7, 20, 2, 0, 0, 0, time.Local)}
	hit2, _, _ := m.evalAbnormalTime(context.Background(), rule, ec2)
	assert.True(t, hit2, "02:00 应在跨午夜范围 23:00-04:00 内")

	// 12:00 不在范围
	ec3 := EvalContext{OccurredAt: time.Date(2026, 7, 20, 12, 0, 0, 0, time.Local)}
	hit3, _, _ := m.evalAbnormalTime(context.Background(), rule, ec3)
	assert.False(t, hit3, "12:00 不在 23:00-04:00 范围")
}

// ============== 6. 异地登录检测 ==============

func TestEvalGeoLogin_NoHistory(t *testing.T) {
	db := setupTestDB(t)
	cfg, _ := setupCfgCache(t, db, nil)
	seedBuiltinRules(t, db)
	m := NewManager(db, cfg)
	var rule model.RiskRule
	require.NoError(t, db.Where("rule_type = ?", RuleTypeGeoLogin).First(&rule).Error)

	// 无任何历史会话：不触发
	ec := EvalContext{
		UserType: UserTypeAdmin, UserID: 1, Username: "admin",
		ClientIP: "1.2.3.4", UserAgent: "Mozilla/5.0",
	}
	hit, _, _ := m.evalGeoLogin(context.Background(), rule, ec)
	assert.False(t, hit, "无历史会话不应触发异地告警")
}

func TestEvalGeoLogin_SameNetwork(t *testing.T) {
	db := setupTestDB(t)
	cfg, _ := setupCfgCache(t, db, nil)
	seedBuiltinRules(t, db)
	m := NewManager(db, cfg)
	var rule model.RiskRule
	require.NoError(t, db.Where("rule_type = ?", RuleTypeGeoLogin).First(&rule).Error)

	// 写入历史会话：同 /24 不同 IP
	require.NoError(t, db.Create(&model.RefreshTokenDevice{
		UserRole: UserTypeAdmin, UserID: 1, RefreshJTI: "jti-1",
		ClientIP: "1.2.3.100", UserAgent: "Mozilla/5.0",
		ExpiresAt: time.Now().Add(time.Hour),
	}).Error)

	ec := EvalContext{
		UserType: UserTypeAdmin, UserID: 1, Username: "admin",
		ClientIP: "1.2.3.4", UserAgent: "Mozilla/5.0",
	}
	hit, _, _ := m.evalGeoLogin(context.Background(), rule, ec)
	assert.False(t, hit, "同 /24 网段不应触发异地告警")
}

func TestEvalGeoLogin_DifferentNetwork(t *testing.T) {
	db := setupTestDB(t)
	cfg, _ := setupCfgCache(t, db, nil)
	seedBuiltinRules(t, db)
	m := NewManager(db, cfg)
	var rule model.RiskRule
	require.NoError(t, db.Where("rule_type = ?", RuleTypeGeoLogin).First(&rule).Error)

	// 历史会话：1.2.3.100（/24）
	require.NoError(t, db.Create(&model.RefreshTokenDevice{
		UserRole: UserTypeAdmin, UserID: 1, RefreshJTI: "jti-1",
		ClientIP: "1.2.3.100", UserAgent: "Mozilla/5.0",
		ExpiresAt: time.Now().Add(time.Hour),
	}).Error)

	// 当前登录：5.6.7.8（不同 /24）
	ec := EvalContext{
		UserType: UserTypeAdmin, UserID: 1, Username: "admin",
		ClientIP: "5.6.7.8", UserAgent: "Mozilla/5.0",
	}
	hit, detail, _ := m.evalGeoLogin(context.Background(), rule, ec)
	assert.True(t, hit, "不同 /24 应触发异地告警")

	var d map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(detail), &d))
	assert.Equal(t, "1.2.3.0/24", d["previous_network"])
	assert.Equal(t, "5.6.7.0/24", d["current_network"])
}

func TestEvalGeoLogin_SameIP(t *testing.T) {
	db := setupTestDB(t)
	cfg, _ := setupCfgCache(t, db, nil)
	seedBuiltinRules(t, db)
	m := NewManager(db, cfg)
	var rule model.RiskRule
	require.NoError(t, db.Where("rule_type = ?", RuleTypeGeoLogin).First(&rule).Error)

	require.NoError(t, db.Create(&model.RefreshTokenDevice{
		UserRole: UserTypeAdmin, UserID: 1, RefreshJTI: "jti-1",
		ClientIP: "1.2.3.4", UserAgent: "Mozilla/5.0",
		ExpiresAt: time.Now().Add(time.Hour),
	}).Error)

	ec := EvalContext{
		UserType: UserTypeAdmin, UserID: 1, Username: "admin",
		ClientIP: "1.2.3.4", UserAgent: "Mozilla/5.0",
	}
	hit, _, _ := m.evalGeoLogin(context.Background(), rule, ec)
	assert.False(t, hit, "同 IP 不应触发异地告警")
}

// ============== 7. 新设备检测 ==============

func TestEvalNewDevice_FirstLogin(t *testing.T) {
	db := setupTestDB(t)
	cfg, _ := setupCfgCache(t, db, nil)
	seedBuiltinRules(t, db)
	m := NewManager(db, cfg)
	var rule model.RiskRule
	require.NoError(t, db.Where("rule_type = ?", RuleTypeNewDevice).First(&rule).Error)

	// 无任何历史会话：不触发（首次登录）
	ec := EvalContext{
		UserType: UserTypeAdmin, UserID: 1, Username: "admin",
		HWID: "abc123", UserAgent: "Mozilla/5.0",
	}
	hit, _, _ := m.evalNewDevice(context.Background(), rule, ec)
	assert.False(t, hit, "首次登录不应触发新设备告警")
}

func TestEvalNewDevice_SameUA(t *testing.T) {
	db := setupTestDB(t)
	cfg, _ := setupCfgCache(t, db, nil)
	seedBuiltinRules(t, db)
	m := NewManager(db, cfg)
	var rule model.RiskRule
	require.NoError(t, db.Where("rule_type = ?", RuleTypeNewDevice).First(&rule).Error)

	require.NoError(t, db.Create(&model.RefreshTokenDevice{
		UserRole: UserTypeAdmin, UserID: 1, RefreshJTI: "jti-1",
		UserAgent: "Mozilla/5.0 Chrome",
		ExpiresAt: time.Now().Add(time.Hour),
	}).Error)

	ec := EvalContext{
		UserType: UserTypeAdmin, UserID: 1, Username: "admin",
		HWID: "abc123", UserAgent: "Mozilla/5.0 Chrome",
	}
	hit, _, _ := m.evalNewDevice(context.Background(), rule, ec)
	assert.False(t, hit, "同 UA 应不触发")
}

func TestEvalNewDevice_DifferentUA(t *testing.T) {
	db := setupTestDB(t)
	cfg, _ := setupCfgCache(t, db, nil)
	seedBuiltinRules(t, db)
	m := NewManager(db, cfg)
	var rule model.RiskRule
	require.NoError(t, db.Where("rule_type = ?", RuleTypeNewDevice).First(&rule).Error)

	require.NoError(t, db.Create(&model.RefreshTokenDevice{
		UserRole: UserTypeAdmin, UserID: 1, RefreshJTI: "jti-1",
		UserAgent: "Mozilla/5.0 Chrome",
		ExpiresAt: time.Now().Add(time.Hour),
	}).Error)

	ec := EvalContext{
		UserType: UserTypeAdmin, UserID: 1, Username: "admin",
		HWID: "abc123", UserAgent: "Mozilla/5.0 Firefox",
	}
	hit, _, _ := m.evalNewDevice(context.Background(), rule, ec)
	assert.True(t, hit, "不同 UA 应触发新设备告警")
}

// ============== 8. 高频请求检测 ==============

func TestEvalHighFrequency_BelowThreshold(t *testing.T) {
	db := setupTestDB(t)
	cfg, _ := setupCfgCache(t, db, nil)
	seedBuiltinRules(t, db)
	m := NewManager(db, cfg)
	var rule model.RiskRule
	require.NoError(t, db.Where("rule_type = ?", RuleTypeHighFrequency).First(&rule).Error)

	// 写入 5 条事件（阈值 10）
	for i := 0; i < 5; i++ {
		require.NoError(t, db.Create(&model.RiskEvent{
			RuleType: RuleTypeHighFrequency, ClientIP: "1.2.3.4",
			Username: "admin", ActionTaken: ActionAlert,
		}).Error)
	}

	ec := EvalContext{ClientIP: "1.2.3.4", Username: "admin"}
	hit, _, _ := m.evalHighFrequency(context.Background(), rule, ec)
	assert.False(t, hit, "5 条 < 10 阈值不应触发")
}

func TestEvalHighFrequency_AtThreshold(t *testing.T) {
	db := setupTestDB(t)
	cfg, _ := setupCfgCache(t, db, nil)
	seedBuiltinRules(t, db)
	m := NewManager(db, cfg)
	var rule model.RiskRule
	require.NoError(t, db.Where("rule_type = ?", RuleTypeHighFrequency).First(&rule).Error)

	// 写入 10 条事件（达到阈值）
	for i := 0; i < 10; i++ {
		require.NoError(t, db.Create(&model.RiskEvent{
			RuleType: RuleTypeHighFrequency, ClientIP: "1.2.3.4",
			Username: "admin", ActionTaken: ActionAlert,
		}).Error)
	}

	ec := EvalContext{ClientIP: "1.2.3.4", Username: "admin"}
	hit, _, _ := m.evalHighFrequency(context.Background(), rule, ec)
	assert.True(t, hit, "10 条达到阈值应触发")
}

// ============== 9. EvaluateLogin 引擎总入口 ==============

func TestEvaluateLogin_EngineDisabled(t *testing.T) {
	db := setupTestDB(t)
	cfg, _ := setupCfgCache(t, db, map[string]string{CfgKeyEngineEnabled: "0"})
	seedBuiltinRules(t, db)
	m := NewManager(db, cfg)

	ec := EvalContext{
		UserType: UserTypeAdmin, UserID: 1, Username: "admin",
		ClientIP: "5.6.7.8", UserAgent: "curl/7.81.0",
	}
	out := m.EvaluateLogin(context.Background(), ec)
	assert.Equal(t, 0, out.TotalScore)
	assert.False(t, out.ShouldBlock)
}

func TestEvaluateLogin_HitMultipleRules(t *testing.T) {
	db := setupTestDB(t)
	cfg, _ := setupCfgCache(t, db, nil)
	seedBuiltinRules(t, db)
	m := NewManager(db, cfg)

	// 准备历史会话（异地登录会触发）
	require.NoError(t, db.Create(&model.RefreshTokenDevice{
		UserRole: UserTypeAdmin, UserID: 1, RefreshJTI: "jti-1",
		ClientIP: "1.2.3.100", UserAgent: "Mozilla/5.0 Chrome",
		ExpiresAt: time.Now().Add(time.Hour),
	}).Error)

	// 命中：异常 UA（curl）+ 异地登录（5.6.7.8 vs 1.2.3.100）+ 新设备（不同 UA）
	ec := EvalContext{
		UserType: UserTypeAdmin, UserID: 1, Username: "admin",
		ClientIP: "5.6.7.8", UserAgent: "curl/7.81.0", HWID: "hwid-1",
	}
	out := m.EvaluateLogin(context.Background(), ec)
	assert.GreaterOrEqual(t, len(out.HitRules), 2, "应至少命中 2 条规则")
	assert.Greater(t, out.TotalScore, 0)
}

// ============== 10. RecordEvent 事件落盘 ==============

func TestRecordEvent_GeoAlertCreated(t *testing.T) {
	db := setupTestDB(t)
	cfg, _ := setupCfgCache(t, db, nil)
	seedBuiltinRules(t, db)
	m := NewManager(db, cfg)

	// 准备历史会话
	require.NoError(t, db.Create(&model.RefreshTokenDevice{
		UserRole: UserTypeAdmin, UserID: 1, RefreshJTI: "jti-1",
		ClientIP: "1.2.3.100", UserAgent: "Mozilla/5.0",
		ExpiresAt: time.Now().Add(time.Hour),
	}).Error)

	ec := EvalContext{
		UserType: UserTypeAdmin, UserID: 1, Username: "admin",
		ClientIP: "5.6.7.8", UserAgent: "Mozilla/5.0",
	}
	out := m.EvaluateLogin(context.Background(), ec)
	require.NotEmpty(t, out.HitRules)

	m.RecordEvent(context.Background(), ec, out)

	// 验证 risk_event 表
	var eventCount int64
	db.Model(&model.RiskEvent{}).Count(&eventCount)
	assert.Greater(t, eventCount, int64(0), "应有风控事件落盘")

	// 验证 login_geo_alert 表
	var alertCount int64
	db.Model(&model.LoginGeoAlert{}).Count(&alertCount)
	assert.Equal(t, int64(1), alertCount, "应有 1 条异地登录告警")
}

// ============== 11. 规则 CRUD ==============

func TestCreateRule_BuiltinForbidden(t *testing.T) {
	db := setupTestDB(t)
	cfg, _ := setupCfgCache(t, db, nil)
	m := NewManager(db, cfg)

	err := m.CreateRule(context.Background(), &model.RiskRule{
		Name: "test", RuleType: RuleTypeGeoLogin, Condition: "{}", Score: 10,
	})
	assert.Error(t, err, "内置规则类型应禁止创建")
}

func TestCreateRule_CustomOK(t *testing.T) {
	db := setupTestDB(t)
	cfg, _ := setupCfgCache(t, db, nil)
	m := NewManager(db, cfg)

	err := m.CreateRule(context.Background(), &model.RiskRule{
		Name: "自定义规则", RuleType: RuleTypeCustom,
		Condition: `{"foo":"bar"}`, Score: 30, Action: ActionAlert,
	})
	assert.NoError(t, err)

	var rule model.RiskRule
	require.NoError(t, db.Where("rule_type = ?", RuleTypeCustom).First(&rule).Error)
	assert.Equal(t, "自定义规则", rule.Name)
	assert.Equal(t, ActionAlert, rule.Action)
	assert.Equal(t, StatusActive, rule.Status)
}

func TestDeleteRule_BuiltinForbidden(t *testing.T) {
	db := setupTestDB(t)
	cfg, _ := setupCfgCache(t, db, nil)
	seedBuiltinRules(t, db)
	m := NewManager(db, cfg)

	var rule model.RiskRule
	require.NoError(t, db.Where("rule_type = ?", RuleTypeGeoLogin).First(&rule).Error)

	err := m.DeleteRule(context.Background(), rule.ID)
	assert.Error(t, err, "内置规则应禁止删除")
}

func TestDeleteRule_CustomOK(t *testing.T) {
	db := setupTestDB(t)
	cfg, _ := setupCfgCache(t, db, nil)
	m := NewManager(db, cfg)

	require.NoError(t, m.CreateRule(context.Background(), &model.RiskRule{
		Name: "test", RuleType: RuleTypeCustom, Condition: "{}", Score: 10,
	}))

	var rule model.RiskRule
	require.NoError(t, db.Where("rule_type = ?", RuleTypeCustom).First(&rule).Error)
	err := m.DeleteRule(context.Background(), rule.ID)
	assert.NoError(t, err)

	var count int64
	db.Model(&model.RiskRule{}).Where("rule_type = ?", RuleTypeCustom).Count(&count)
	assert.Equal(t, int64(0), count)
}

func TestUpdateRule_BuiltinCannotChangeType(t *testing.T) {
	db := setupTestDB(t)
	cfg, _ := setupCfgCache(t, db, nil)
	seedBuiltinRules(t, db)
	m := NewManager(db, cfg)

	var rule model.RiskRule
	require.NoError(t, db.Where("rule_type = ?", RuleTypeGeoLogin).First(&rule).Error)

	err := m.UpdateRule(context.Background(), rule.ID, map[string]interface{}{
		"rule_type": RuleTypeCustom,
		"status":    StatusDisabled,
	})
	assert.NoError(t, err)

	// rule_type 应未变，status 应变更
	var updated model.RiskRule
	require.NoError(t, db.First(&updated, rule.ID).Error)
	assert.Equal(t, RuleTypeGeoLogin, updated.RuleType, "rule_type 应未变")
	assert.Equal(t, StatusDisabled, updated.Status, "status 应已变更")
}

// ============== 12. 事件/告警查询与确认 ==============

func TestAcknowledgeEvent(t *testing.T) {
	db := setupTestDB(t)
	cfg, _ := setupCfgCache(t, db, nil)
	m := NewManager(db, cfg)

	require.NoError(t, db.Create(&model.RiskEvent{
		RuleType: RuleTypeAbnormalUA, RuleName: "test", ActionTaken: ActionAlert,
	}).Error)

	var ev model.RiskEvent
	require.NoError(t, db.First(&ev).Error)

	err := m.AcknowledgeEvent(context.Background(), ev.ID, "admin1")
	assert.NoError(t, err)

	var updated model.RiskEvent
	require.NoError(t, db.First(&updated, ev.ID).Error)
	assert.True(t, updated.Acknowledged)
	assert.Equal(t, "admin1", updated.AcknowledgedBy)
}

func TestAcknowledgeGeoAlert(t *testing.T) {
	db := setupTestDB(t)
	cfg, _ := setupCfgCache(t, db, nil)
	m := NewManager(db, cfg)

	require.NoError(t, db.Create(&model.LoginGeoAlert{
		UserType: UserTypeAdmin, UserID: 1, Username: "admin",
		CurrentIP: "1.2.3.4", CurrentNetwork: "1.2.3.0/24",
		PreviousIP: "5.6.7.8", PreviousNetwork: "5.6.7.0/24",
		AlertStatus: "pending",
	}).Error)

	var alert model.LoginGeoAlert
	require.NoError(t, db.First(&alert).Error)

	err := m.AcknowledgeGeoAlert(context.Background(), alert.ID, "admin1")
	assert.NoError(t, err)

	var updated model.LoginGeoAlert
	require.NoError(t, db.First(&updated, alert.ID).Error)
	assert.Equal(t, "acknowledged", updated.AlertStatus)
	assert.Equal(t, "admin1", updated.AcknowledgedBy)
}

func TestCloseGeoAlert_AlreadyAcknowledged(t *testing.T) {
	db := setupTestDB(t)
	cfg, _ := setupCfgCache(t, db, nil)
	m := NewManager(db, cfg)

	require.NoError(t, db.Create(&model.LoginGeoAlert{
		UserType: UserTypeAdmin, UserID: 1, Username: "admin",
		CurrentIP: "1.2.3.4", CurrentNetwork: "1.2.3.0/24",
		PreviousIP: "5.6.7.8", PreviousNetwork: "5.6.7.0/24",
		AlertStatus: "acknowledged",
	}).Error)

	var alert model.LoginGeoAlert
	require.NoError(t, db.First(&alert).Error)

	err := m.CloseGeoAlert(context.Background(), alert.ID, "admin1")
	assert.NoError(t, err)

	var updated model.LoginGeoAlert
	require.NoError(t, db.First(&updated, alert.ID).Error)
	assert.Equal(t, "closed", updated.AlertStatus)
	assert.NotNil(t, updated.ClosedAt)
}

// ============== 13. 统计 ==============

func TestGetStats_Empty(t *testing.T) {
	db := setupTestDB(t)
	cfg, _ := setupCfgCache(t, db, nil)
	m := NewManager(db, cfg)

	stats, err := m.GetStats(context.Background())
	require.NoError(t, err)
	assert.Equal(t, int64(0), stats.RiskEventsToday)
	assert.Equal(t, int64(0), stats.GeoAlertsToday)
	assert.Equal(t, int64(0), stats.PendingAlerts)
	assert.Empty(t, stats.TopAbnormalIPs)
	assert.Empty(t, stats.RecentEvents)
}

func TestGetStats_WithEvents(t *testing.T) {
	db := setupTestDB(t)
	cfg, _ := setupCfgCache(t, db, nil)
	m := NewManager(db, cfg)

	require.NoError(t, db.Create(&model.RiskEvent{
		RuleType: RuleTypeAbnormalUA, RuleName: "异常 UA", ActionTaken: ActionAlert,
		ClientIP: "1.2.3.4", RiskScore: 30,
	}).Error)
	require.NoError(t, db.Create(&model.RiskEvent{
		RuleType: RuleTypeGeoLogin, RuleName: "异地登录", ActionTaken: ActionBlock,
		ClientIP: "5.6.7.8", RiskScore: 60,
	}).Error)
	require.NoError(t, db.Create(&model.LoginGeoAlert{
		UserType: UserTypeAdmin, UserID: 1, Username: "admin",
		CurrentIP: "1.2.3.4", CurrentNetwork: "1.2.3.0/24",
		PreviousIP: "5.6.7.8", PreviousNetwork: "5.6.7.0/24",
		AlertStatus: "pending",
	}).Error)

	stats, err := m.GetStats(context.Background())
	require.NoError(t, err)
	assert.Equal(t, int64(2), stats.RiskEventsToday)
	assert.Equal(t, int64(1), stats.BlockedRequests)
	assert.Equal(t, int64(1), stats.AlertRequests)
	assert.Equal(t, int64(1), stats.GeoAlertsToday)
	assert.Equal(t, int64(1), stats.GeoAlertsPending)
	assert.Len(t, stats.TopAbnormalIPs, 2)
	assert.Len(t, stats.RecentEvents, 2)
}

// ============== 14. 配置键常量正确性 ==============

func TestCfgKeyConstants(t *testing.T) {
	assert.Equal(t, "cloudflare.enabled", CfgKeyCloudflareEnabled)
	assert.Equal(t, "cloudflare.real_ip_header", CfgKeyCloudflareRealIPHeader)
	assert.Equal(t, "risk.engine.enabled", CfgKeyEngineEnabled)
	assert.Equal(t, "risk.engine.score_threshold", CfgKeyEngineScoreThreshold)
	assert.Equal(t, "risk.geo_login_alert.enabled", CfgKeyGeoLoginEnabled)
	assert.Equal(t, "risk.geo_login_alert.ipv4_prefix", CfgKeyGeoLoginIPv4Prefix)
	assert.Equal(t, "risk.new_device_alert.enabled", CfgKeyNewDeviceEnabled)
	assert.Equal(t, "risk.abnormal_ua_alert.enabled", CfgKeyAbnormalUAEnabled)
	assert.Equal(t, "risk.abnormal_time_alert.enabled", CfgKeyAbnormalTimeEnabled)
}

// ============== 15. 规则类型与动作常量 ==============

func TestRuleTypeConstants(t *testing.T) {
	assert.Equal(t, "geo_login", RuleTypeGeoLogin)
	assert.Equal(t, "new_device", RuleTypeNewDevice)
	assert.Equal(t, "abnormal_ua", RuleTypeAbnormalUA)
	assert.Equal(t, "abnormal_time", RuleTypeAbnormalTime)
	assert.Equal(t, "high_frequency", RuleTypeHighFrequency)
	assert.Equal(t, "custom", RuleTypeCustom)
}

func TestActionConstants(t *testing.T) {
	assert.Equal(t, "alert", ActionAlert)
	assert.Equal(t, "challenge", ActionChallenge)
	assert.Equal(t, "block", ActionBlock)
}
