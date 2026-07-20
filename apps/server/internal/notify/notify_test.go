// Package notify v0.4.0 通知系统单元测试
// 严格遵循铁律 06：所有断言基于已知固定输入，无随机/不确定性
// 测试覆盖：
//   1. Render（变量替换 / 空变量 / 多变量 / 占位符未提供保留）
//   2. ValidateChannel（sms/email/inapp/非法）
//   3. ParseVariables（空 / 数组 / 非法 JSON）
//   4. GenerateVerifyCode（长度 / 全数字）
//   5. IsChannelEnabled（开关从 sys_config 读取）
//   6. CheckRateLimit（未超限 / 超限 / limit=0 不限）
//   7. GetTemplate（租户自定义优先 / 回退平台通用 / 不存在）
//   8. Send（通道关闭 / 限流 / 模板未找到 / mock provider 成功 / mock provider 失败）
//   9. Retry（成功 / 非 failed 状态 / 超过最大重试次数）
//  10. GetStats（按状态/渠道统计）
//  11. 模板 CRUD（Create / Update / Delete / List）
//  12. mock provider 注入（SetSMSProvider / SetEmailProvider）
package notify

import (
	"context"
	"encoding/base64"
	"errors"
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
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:notify_test_%d?mode=memory&cache=shared", time.Now().UnixNano())), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&model.NotifyTemplate{},
		&model.NotifyLog{},
		&model.SysConfig{},
	))
	db.Exec("DELETE FROM notify_template")
	db.Exec("DELETE FROM notify_log")
	db.Exec("DELETE FROM sys_config")
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
		CfgKeySMSEnabled:           "0",
		CfgKeySMSProvider:          "none",
		CfgKeySMSAccessKeyID:       "",
		CfgKeySMSAccessSecretEnc:   "",
		CfgKeySMSSignName:          "",
		CfgKeyEmailEnabled:         "0",
		CfgKeyEmailSMTPHost:         "",
		CfgKeyEmailSMTPPort:         "465",
		CfgKeyEmailSMTPUsername:     "",
		CfgKeyEmailSMTPPasswordEnc:  "",
		CfgKeyEmailFromAddress:      "",
		CfgKeyEmailFromName:         "KeyAuth SaaS",
		CfgKeyInAppEnabled:          "1",
		CfgKeyRetryTimes:            "3",
		CfgKeyRetryIntervalSeconds:  "60",
		CfgKeyRateLimitPerMinute:    "60",
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
			ConfigGroup: "notify",
		}).Error)
	}
	return config.NewConfigCache(db, rdb), mr
}

// mockSMSProvider 测试用 mock 短信服务商
type mockSMSProvider struct {
	sendError error
	msgID     string
	calls     int
}

func (m *mockSMSProvider) Send(ctx context.Context, phone, signName, templateCode string, params map[string]interface{}) (string, error) {
	m.calls++
	if m.sendError != nil {
		return "", m.sendError
	}
	if m.msgID != "" {
		return m.msgID, nil
	}
	return fmt.Sprintf("mock-sms-%d", time.Now().UnixNano()), nil
}

// mockEmailProvider 测试用 mock 邮件服务商
type mockEmailProvider struct {
	sendError error
	msgID     string
	calls     int
}

func (m *mockEmailProvider) Send(ctx context.Context, to, subject, htmlBody string) (string, error) {
	m.calls++
	if m.sendError != nil {
		return "", m.sendError
	}
	if m.msgID != "" {
		return m.msgID, nil
	}
	return fmt.Sprintf("mock-email-%d", time.Now().UnixNano()), nil
}

// ============== 1. Render ==============

func TestRender_NoVars(t *testing.T) {
	assert.Equal(t, "hello world", Render("hello world", nil))
	assert.Equal(t, "hello world", Render("hello world", map[string]interface{}{}))
}

func TestRender_SingleVar(t *testing.T) {
	out := Render("您的验证码是 {{code}}", map[string]interface{}{"code": "123456"})
	assert.Equal(t, "您的验证码是 123456", out)
}

func TestRender_MultiVars(t *testing.T) {
	out := Render("{{name}} 您好，订单 {{order_no}} 已支付 {{amount}} 元", map[string]interface{}{
		"name":     "张三",
		"order_no": "ORD20250720001",
		"amount":   99.5,
	})
	assert.Equal(t, "张三 您好，订单 ORD20250720001 已支付 99.5 元", out)
}

func TestRender_VarNotProvided(t *testing.T) {
	// 未提供的变量保留原占位符（便于排查）
	out := Render("您的验证码是 {{code}}, 用户：{{name}}", map[string]interface{}{"code": "123456"})
	assert.Equal(t, "您的验证码是 123456, 用户：{{name}}", out)
}

func TestRender_SSTISafe(t *testing.T) {
	// 铁律 06：strings.NewReplacer 不解析模板语法，杜绝 SSTI
	out := Render("{{user}}", map[string]interface{}{"user": "{{admin}}"})
	assert.Equal(t, "{{admin}}", out)
}

// ============== 2. ValidateChannel ==============

func TestValidateChannel_Valid(t *testing.T) {
	assert.True(t, ValidateChannel(ChannelSMS))
	assert.True(t, ValidateChannel(ChannelEmail))
	assert.True(t, ValidateChannel(ChannelInApp))
}

func TestValidateChannel_Invalid(t *testing.T) {
	assert.False(t, ValidateChannel(""))
	assert.False(t, ValidateChannel("wechat"))
	assert.False(t, ValidateChannel("SMS")) // 大小写敏感
}

// ============== 3. ParseVariables ==============

func TestParseVariables_Empty(t *testing.T) {
	assert.Equal(t, []string{}, ParseVariables(""))
	assert.Equal(t, []string{}, ParseVariables("[]"))
}

func TestParseVariables_Array(t *testing.T) {
	vars := ParseVariables(`["code","name","amount"]`)
	assert.Equal(t, []string{"code", "name", "amount"}, vars)
}

func TestParseVariables_InvalidJSON(t *testing.T) {
	assert.Equal(t, []string{}, ParseVariables("not-json"))
	assert.Equal(t, []string{}, ParseVariables(`{"key":"val"}`)) // 不是数组
}

// ============== 4. GenerateVerifyCode ==============

func TestGenerateVerifyCode_DefaultLength(t *testing.T) {
	code := GenerateVerifyCode(0) // 0 回退默认 6
	assert.Equal(t, 6, len(code))
	for _, c := range code {
		assert.True(t, c >= '0' && c <= '9', "应全为数字")
	}
}

func TestGenerateVerifyCode_CustomLength(t *testing.T) {
	code := GenerateVerifyCode(4)
	assert.Equal(t, 4, len(code))
	code8 := GenerateVerifyCode(8)
	assert.Equal(t, 8, len(code8))
}

func TestGenerateVerifyCode_AllDigits(t *testing.T) {
	code := GenerateVerifyCode(10)
	for _, c := range code {
		assert.True(t, c >= '0' && c <= '9')
	}
}

// ============== 5. IsChannelEnabled ==============

func TestIsChannelEnabled_Defaults(t *testing.T) {
	db := setupTestDB(t)
	cache, _ := setupTestCfgCache(t, db, nil)
	mgr := NewManager(db, cache, nil)
	ctx := context.Background()

	// 默认 SMS/Email 关闭，InApp 开启
	assert.False(t, mgr.IsChannelEnabled(ctx, ChannelSMS))
	assert.False(t, mgr.IsChannelEnabled(ctx, ChannelEmail))
	assert.True(t, mgr.IsChannelEnabled(ctx, ChannelInApp))
}

func TestIsChannelEnabled_Enabled(t *testing.T) {
	db := setupTestDB(t)
	cache, _ := setupTestCfgCache(t, db, map[string]string{
		CfgKeySMSEnabled:   "1",
		CfgKeyEmailEnabled: "1",
	})
	mgr := NewManager(db, cache, nil)
	ctx := context.Background()

	assert.True(t, mgr.IsChannelEnabled(ctx, ChannelSMS))
	assert.True(t, mgr.IsChannelEnabled(ctx, ChannelEmail))
	assert.True(t, mgr.IsChannelEnabled(ctx, ChannelInApp))
}

func TestIsChannelEnabled_UnknownChannel(t *testing.T) {
	db := setupTestDB(t)
	cache, _ := setupTestCfgCache(t, db, nil)
	mgr := NewManager(db, cache, nil)
	assert.False(t, mgr.IsChannelEnabled(context.Background(), "wechat"))
}

// ============== 6. CheckRateLimit ==============

func TestCheckRateLimit_NoLimit(t *testing.T) {
	// limit=0 表示不限
	db := setupTestDB(t)
	cache, _ := setupTestCfgCache(t, db, map[string]string{CfgKeyRateLimitPerMinute: "0"})
	mgr := NewManager(db, cache, nil)
	assert.True(t, mgr.CheckRateLimit(context.Background(), 100))
}

func TestCheckRateLimit_BelowLimit(t *testing.T) {
	db := setupTestDB(t)
	cache, _ := setupTestCfgCache(t, db, map[string]string{CfgKeyRateLimitPerMinute: "3"})
	mgr := NewManager(db, cache, nil)
	ctx := context.Background()

	// 预置 2 条日志
	for i := 0; i < 2; i++ {
		require.NoError(t, db.Create(&model.NotifyLog{
			TemplateCode: "test", Channel: ChannelSMS, Recipient: "13900000000",
			Status: LogStatusSent, TenantID: 100,
		}).Error)
	}
	assert.True(t, mgr.CheckRateLimit(ctx, 100)) // 2 < 3
}

func TestCheckRateLimit_Exceeded(t *testing.T) {
	db := setupTestDB(t)
	cache, _ := setupTestCfgCache(t, db, map[string]string{CfgKeyRateLimitPerMinute: "2"})
	mgr := NewManager(db, cache, nil)
	ctx := context.Background()

	for i := 0; i < 2; i++ {
		require.NoError(t, db.Create(&model.NotifyLog{
			TemplateCode: "test", Channel: ChannelSMS, Recipient: "13900000000",
			Status: LogStatusSent, TenantID: 200,
		}).Error)
	}
	assert.False(t, mgr.CheckRateLimit(ctx, 200)) // 2 >= 2
}

// ============== 7. GetTemplate ==============

func TestGetTemplate_PlatformFallback(t *testing.T) {
	db := setupTestDB(t)
	cache, _ := setupTestCfgCfgCacheSimple(t, db)
	mgr := NewManager(db, cache, nil)
	ctx := context.Background()

	// 仅插入平台通用模板（tenant_id=0）
	require.NoError(t, db.Create(&model.NotifyTemplate{
		Code: TemplateVerifyCode, Name: "验证码", Channel: ChannelSMS,
		Content: "您的验证码：{{code}}", Variables: "[\"code\"]",
		TenantID: 0, Status: TemplateStatusEnabled,
	}).Error)

	// 租户 999 没有，应回退到平台通用
	tmpl, err := mgr.GetTemplate(ctx, TemplateVerifyCode, ChannelSMS, 999)
	require.NoError(t, err)
	assert.Equal(t, uint64(0), tmpl.TenantID)
	assert.Equal(t, "您的验证码：{{code}}", tmpl.Content)
}

func TestGetTemplate_TenantPreferred(t *testing.T) {
	db := setupTestDB(t)
	cache, _ := setupTestCfgCfgCacheSimple(t, db)
	mgr := NewManager(db, cache, nil)
	ctx := context.Background()

	// 平台 + 租户自定义同时存在
	require.NoError(t, db.Create(&model.NotifyTemplate{
		Code: TemplateVerifyCode, Name: "验证码", Channel: ChannelSMS,
		Content: "平台：{{code}}", TenantID: 0, Status: TemplateStatusEnabled,
	}).Error)
	require.NoError(t, db.Create(&model.NotifyTemplate{
		Code: TemplateVerifyCode, Name: "验证码", Channel: ChannelSMS,
		Content: "租户：{{code}}", TenantID: 999, Status: TemplateStatusEnabled,
	}).Error)

	tmpl, err := mgr.GetTemplate(ctx, TemplateVerifyCode, ChannelSMS, 999)
	require.NoError(t, err)
	assert.Equal(t, uint64(999), tmpl.TenantID)
	assert.Equal(t, "租户：{{code}}", tmpl.Content)
}

func TestGetTemplate_NotFound(t *testing.T) {
	db := setupTestDB(t)
	cache, _ := setupTestCfgCfgCacheSimple(t, db)
	mgr := NewManager(db, cache, nil)

	_, err := mgr.GetTemplate(context.Background(), "nonexistent", ChannelSMS, 0)
	assert.ErrorIs(t, err, ErrTemplateNotFound)
}

func TestGetTemplate_DisabledSkipped(t *testing.T) {
	db := setupTestDB(t)
	cache, _ := setupTestCfgCfgCacheSimple(t, db)
	mgr := NewManager(db, cache, nil)
	ctx := context.Background()

	// 平台模板被禁用
	require.NoError(t, db.Create(&model.NotifyTemplate{
		Code: TemplateVerifyCode, Name: "验证码", Channel: ChannelSMS,
		Content: "disabled", TenantID: 0, Status: TemplateStatusDisabled,
	}).Error)

	_, err := mgr.GetTemplate(ctx, TemplateVerifyCode, ChannelSMS, 0)
	assert.ErrorIs(t, err, ErrTemplateNotFound)
}

// setupTestCfgCfgCacheSimple 简化版 setup（不依赖 notify.* 配置）
func setupTestCfgCfgCacheSimple(t *testing.T, db *gorm.DB) (*config.ConfigCache, *miniredis.Miniredis) {
	t.Helper()
	mr, err := miniredis.Run()
	require.NoError(t, err)
	t.Cleanup(mr.Close)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	return config.NewConfigCache(db, rdb), mr
}

// ============== 8. Send ==============

func TestSend_ChannelDisabled(t *testing.T) {
	db := setupTestDB(t)
	cache, _ := setupTestCfgCache(t, db, map[string]string{
		CfgKeySMSEnabled: "0",
	})
	mgr := NewManager(db, cache, nil)

	result, err := mgr.Send(context.Background(), SendRequest{
		TemplateCode: TemplateVerifyCode,
		Channel:      ChannelSMS,
		Recipient:    "13900000000",
		TenantID:     0,
	})
	assert.ErrorIs(t, err, ErrChannelDisabled)
	assert.Equal(t, LogStatusFailed, result.Status)
}

func TestSend_RateLimitExceeded(t *testing.T) {
	db := setupTestDB(t)
	cache, _ := setupTestCfgCache(t, db, map[string]string{
		CfgKeyInAppEnabled:       "1",
		CfgKeyRateLimitPerMinute: "1",
	})
	mgr := NewManager(db, cache, nil)
	ctx := context.Background()

	// 预置 1 条日志使限流触发
	require.NoError(t, db.Create(&model.NotifyLog{
		TemplateCode: "test", Channel: ChannelInApp, Recipient: "u1",
		Status: LogStatusSent, TenantID: 0,
	}).Error)

	result, err := mgr.Send(ctx, SendRequest{
		TemplateCode: TemplateOrderPaid,
		Channel:      ChannelInApp,
		Recipient:    "u2",
		TenantID:     0,
	})
	assert.ErrorIs(t, err, ErrRateLimitExceeded)
	assert.Equal(t, LogStatusFailed, result.Status)
}

func TestSend_TemplateNotFound(t *testing.T) {
	db := setupTestDB(t)
	cache, _ := setupTestCfgCache(t, db, map[string]string{
		CfgKeyInAppEnabled: "1",
	})
	mgr := NewManager(db, cache, nil)

	_, err := mgr.Send(context.Background(), SendRequest{
		TemplateCode: "nonexistent",
		Channel:      ChannelInApp,
		Recipient:    "u1",
		TenantID:     0,
	})
	assert.ErrorIs(t, err, ErrTemplateNotFound)
}

func TestSend_InAppSuccess(t *testing.T) {
	db := setupTestDB(t)
	cache, _ := setupTestCfgCache(t, db, map[string]string{
		CfgKeyInAppEnabled: "1",
	})
	mgr := NewManager(db, cache, nil)
	ctx := context.Background()

	require.NoError(t, db.Create(&model.NotifyTemplate{
		Code: TemplateOrderPaid, Name: "订单支付成功", Channel: ChannelInApp,
		Subject: "订单已支付", Content: "订单 {{order_no}} 已支付",
		Variables: "[\"order_no\"]", TenantID: 0, Status: TemplateStatusEnabled,
	}).Error)

	result, err := mgr.Send(ctx, SendRequest{
		TemplateCode: TemplateOrderPaid,
		Channel:      ChannelInApp,
		Recipient:    "user-001",
		Variables:    map[string]interface{}{"order_no": "ORD20250720001"},
		TenantID:     0,
	})
	require.NoError(t, err)
	assert.Equal(t, LogStatusSent, result.Status)
	assert.NotEmpty(t, result.ProviderMsgID)
	assert.True(t, result.LogID > 0)

	// 验证日志已写入
	var log model.NotifyLog
	require.NoError(t, db.First(&log, result.LogID).Error)
	assert.Equal(t, "订单 ORD20250720001 已支付", log.Content)
	assert.Equal(t, "订单已支付", log.Subject)
	assert.Equal(t, LogStatusSent, log.Status)
	assert.NotNil(t, log.SentAt)
}

func TestSend_SMSWithMockProvider(t *testing.T) {
	db := setupTestDB(t)
	cache, _ := setupTestCfgCache(t, db, map[string]string{
		CfgKeySMSEnabled:  "1",
		CfgKeySMSProvider: "aliyun",
	})
	mgr := NewManager(db, cache, nil)
	mock := &mockSMSProvider{msgID: "aliyun-test-msgid"}
	mgr.SetSMSProvider(mock)
	ctx := context.Background()

	require.NoError(t, db.Create(&model.NotifyTemplate{
		Code: TemplateVerifyCode, Name: "验证码", Channel: ChannelSMS,
		Content: "验证码 {{code}}", Variables: "[\"code\"]",
		TenantID: 0, Status: TemplateStatusEnabled,
	}).Error)

	result, err := mgr.Send(ctx, SendRequest{
		TemplateCode: TemplateVerifyCode,
		Channel:      ChannelSMS,
		Recipient:    "13900000000",
		Variables:    map[string]interface{}{"code": "888888"},
		TenantID:     0,
	})
	require.NoError(t, err)
	assert.Equal(t, LogStatusSent, result.Status)
	assert.Equal(t, "aliyun-test-msgid", result.ProviderMsgID)
	assert.Equal(t, 1, mock.calls)

	// 验证日志
	var log model.NotifyLog
	require.NoError(t, db.First(&log, result.LogID).Error)
	assert.Equal(t, "验证码 888888", log.Content)
	assert.Equal(t, "13900000000", log.Recipient)
}

func TestSend_SMSProviderFailed(t *testing.T) {
	db := setupTestDB(t)
	cache, _ := setupTestCfgCache(t, db, map[string]string{
		CfgKeySMSEnabled:  "1",
		CfgKeySMSProvider: "aliyun",
	})
	mgr := NewManager(db, cache, nil)
	mgr.SetSMSProvider(&mockSMSProvider{sendError: errors.New("upstream timeout")})

	require.NoError(t, db.Create(&model.NotifyTemplate{
		Code: TemplateVerifyCode, Name: "验证码", Channel: ChannelSMS,
		Content: "验证码 {{code}}", TenantID: 0, Status: TemplateStatusEnabled,
	}).Error)

	result, err := mgr.Send(context.Background(), SendRequest{
		TemplateCode: TemplateVerifyCode,
		Channel:      ChannelSMS,
		Recipient:    "13900000000",
		TenantID:     0,
	})
	require.NoError(t, err) // Send 本身不报错，错误体现在 result.Status
	assert.Equal(t, LogStatusFailed, result.Status)
	assert.Contains(t, result.ErrorMessage, "upstream timeout")
}

func TestSend_EmailWithMockProvider(t *testing.T) {
	db := setupTestDB(t)
	cache, _ := setupTestCfgCache(t, db, map[string]string{
		CfgKeyEmailEnabled: "1",
	})
	mgr := NewManager(db, cache, nil)
	mgr.SetEmailProvider(&mockEmailProvider{msgID: "smtp-test-msgid"})
	ctx := context.Background()

	require.NoError(t, db.Create(&model.NotifyTemplate{
		Code: TemplateVerifyCodeEmail, Name: "邮箱验证码", Channel: ChannelEmail,
		Subject: "验证码", Content: "您的邮箱验证码 {{code}}",
		TenantID: 0, Status: TemplateStatusEnabled,
	}).Error)

	result, err := mgr.Send(ctx, SendRequest{
		TemplateCode: TemplateVerifyCodeEmail,
		Channel:      ChannelEmail,
		Recipient:    "user@example.com",
		Variables:    map[string]interface{}{"code": "999999"},
		TenantID:     0,
	})
	require.NoError(t, err)
	assert.Equal(t, LogStatusSent, result.Status)
	assert.Equal(t, "smtp-test-msgid", result.ProviderMsgID)
}

func TestSend_EmptyRecipient(t *testing.T) {
	db := setupTestDB(t)
	cache, _ := setupTestCfgCache(t, db, map[string]string{CfgKeyInAppEnabled: "1"})
	mgr := NewManager(db, cache, nil)

	_, err := mgr.Send(context.Background(), SendRequest{
		TemplateCode: TemplateOrderPaid,
		Channel:      ChannelInApp,
		Recipient:    "",
		TenantID:     0,
	})
	assert.ErrorIs(t, err, ErrInvalidRecipient)
}

// ============== 9. Retry ==============

func TestRetry_Success(t *testing.T) {
	db := setupTestDB(t)
	cache, _ := setupTestCfgCache(t, db, map[string]string{
		CfgKeySMSEnabled:  "1",
		CfgKeySMSProvider: "aliyun",
	})
	mgr := NewManager(db, cache, nil)
	mgr.SetSMSProvider(&mockSMSProvider{msgID: "retry-msgid"})
	ctx := context.Background()

	// 预置一条 failed 日志
	logEntry := &model.NotifyLog{
		TemplateCode: TemplateVerifyCode, Channel: ChannelSMS,
		Recipient: "13900000000", Subject: "", Content: "验证码 123",
		Status: LogStatusFailed, RetryCount: 0, TenantID: 0,
	}
	require.NoError(t, db.Create(logEntry).Error)

	result, err := mgr.Retry(ctx, logEntry.ID)
	require.NoError(t, err)
	assert.Equal(t, LogStatusSent, result.Status)
	assert.Equal(t, "retry-msgid", result.ProviderMsgID)

	// 验证 retry_count 递增
	var updated model.NotifyLog
	require.NoError(t, db.First(&updated, logEntry.ID).Error)
	assert.Equal(t, 1, updated.RetryCount)
	assert.Equal(t, LogStatusSent, updated.Status)
}

func TestRetry_NotFailedStatus(t *testing.T) {
	db := setupTestDB(t)
	cache, _ := setupTestCfgCache(t, db, nil)
	mgr := NewManager(db, cache, nil)

	logEntry := &model.NotifyLog{
		TemplateCode: "test", Channel: ChannelInApp, Recipient: "u1",
		Content: "x", Status: LogStatusSent, TenantID: 0,
	}
	require.NoError(t, db.Create(logEntry).Error)

	result, err := mgr.Retry(context.Background(), logEntry.ID)
	require.NoError(t, err) // 非 failed 不报错，仅返回原状态
	assert.Equal(t, LogStatusSent, result.Status)
	assert.Contains(t, result.ErrorMessage, "only failed log can retry")
}

func TestRetry_MaxExceeded(t *testing.T) {
	db := setupTestDB(t)
	cache, _ := setupTestCfgCache(t, db, map[string]string{
		CfgKeySMSEnabled:  "1",
		CfgKeySMSProvider: "aliyun",
		CfgKeyRetryTimes:  "2",
	})
	mgr := NewManager(db, cache, nil)
	mgr.SetSMSProvider(&mockSMSProvider{})

	logEntry := &model.NotifyLog{
		TemplateCode: TemplateVerifyCode, Channel: ChannelSMS,
		Recipient: "13900000000", Content: "x",
		Status: LogStatusFailed, RetryCount: 2, TenantID: 0,
	}
	require.NoError(t, db.Create(logEntry).Error)

	result, err := mgr.Retry(context.Background(), logEntry.ID)
	require.NoError(t, err)
	assert.Equal(t, LogStatusFailed, result.Status)
	assert.Contains(t, result.ErrorMessage, "max retry exceeded")
}

// ============== 10. GetStats ==============

func TestGetStats_All(t *testing.T) {
	db := setupTestDB(t)
	cache, _ := setupTestCfgCache(t, db, nil)
	mgr := NewManager(db, cache, nil)
	ctx := context.Background()

	// 写入混合日志
	entries := []model.NotifyLog{
		{TemplateCode: "t1", Channel: ChannelSMS, Recipient: "13900000000", Status: LogStatusSent, TenantID: 0},
		{TemplateCode: "t2", Channel: ChannelSMS, Recipient: "13900000001", Status: LogStatusFailed, TenantID: 0},
		{TemplateCode: "t3", Channel: ChannelEmail, Recipient: "u@e.com", Status: LogStatusSent, TenantID: 0},
		{TemplateCode: "t4", Channel: ChannelInApp, Recipient: "u1", Status: LogStatusPending, TenantID: 0},
		{TemplateCode: "t5", Channel: ChannelInApp, Recipient: "u2", Status: LogStatusSent, TenantID: 0},
	}
	for i := range entries {
		require.NoError(t, db.Create(&entries[i]).Error)
	}

	stats, err := mgr.GetStats(ctx, 0)
	require.NoError(t, err)
	assert.Equal(t, int64(5), stats.Total)
	assert.Equal(t, int64(3), stats.Sent)
	assert.Equal(t, int64(1), stats.Failed)
	assert.Equal(t, int64(1), stats.Pending)
	assert.Equal(t, int64(2), stats.SMSCount)
	assert.Equal(t, int64(1), stats.EmailCount)
	assert.Equal(t, int64(2), stats.InAppCount)
}

func TestGetStats_ByTenant(t *testing.T) {
	db := setupTestDB(t)
	cache, _ := setupTestCfgCache(t, db, nil)
	mgr := NewManager(db, cache, nil)
	ctx := context.Background()

	require.NoError(t, db.Create(&model.NotifyLog{
		TemplateCode: "t1", Channel: ChannelSMS, Recipient: "1",
		Status: LogStatusSent, TenantID: 100,
	}).Error)
	require.NoError(t, db.Create(&model.NotifyLog{
		TemplateCode: "t2", Channel: ChannelSMS, Recipient: "2",
		Status: LogStatusSent, TenantID: 200,
	}).Error)

	stats, err := mgr.GetStats(ctx, 100)
	require.NoError(t, err)
	assert.Equal(t, int64(1), stats.Total)
}

// ============== 11. 模板 CRUD ==============

func TestTemplateCRUD(t *testing.T) {
	db := setupTestDB(t)
	cache, _ := setupTestCfgCfgCacheSimple(t, db)
	mgr := NewManager(db, cache, nil)
	ctx := context.Background()

	// Create
	tmpl := &model.NotifyTemplate{
		Code: "custom_code", Name: "自定义模板", Channel: ChannelSMS,
		Content: "测试 {{var}}", Variables: "[\"var\"]",
		TenantID: 0, Status: TemplateStatusEnabled,
	}
	require.NoError(t, mgr.CreateTemplate(ctx, tmpl))
	assert.True(t, tmpl.ID > 0)

	// Get
	fetched, err := mgr.GetTemplate(ctx, "custom_code", ChannelSMS, 0)
	require.NoError(t, err)
	assert.Equal(t, "自定义模板", fetched.Name)

	// Update
	require.NoError(t, mgr.UpdateTemplate(ctx, tmpl.ID, map[string]interface{}{
		"name": "更新后的模板",
	}))
	fetched2, err := mgr.GetTemplate(ctx, "custom_code", ChannelSMS, 0)
	require.NoError(t, err)
	assert.Equal(t, "更新后的模板", fetched2.Name)

	// List
	items, total, err := mgr.ListTemplates(ctx, 0, ChannelSMS, 1, 20)
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Equal(t, 1, len(items))

	// Delete
	require.NoError(t, mgr.DeleteTemplate(ctx, tmpl.ID))
	_, err = mgr.GetTemplate(ctx, "custom_code", ChannelSMS, 0)
	assert.ErrorIs(t, err, ErrTemplateNotFound)
}

// ============== 12. ListLogs ==============

func TestListLogs_Filters(t *testing.T) {
	db := setupTestDB(t)
	cache, _ := setupTestCfgCfgCacheSimple(t, db)
	mgr := NewManager(db, cache, nil)
	ctx := context.Background()

	entries := []model.NotifyLog{
		{TemplateCode: "t1", Channel: ChannelSMS, Recipient: "1", Status: LogStatusSent, TenantID: 0},
		{TemplateCode: "t2", Channel: ChannelEmail, Recipient: "2", Status: LogStatusFailed, TenantID: 0},
		{TemplateCode: "t3", Channel: ChannelInApp, Recipient: "3", Status: LogStatusSent, TenantID: 0},
	}
	for i := range entries {
		require.NoError(t, db.Create(&entries[i]).Error)
	}

	// 按 channel 过滤
	items, total, err := mgr.ListLogs(ctx, 0, ChannelSMS, "", 1, 20)
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Equal(t, 1, len(items))
	assert.Equal(t, ChannelSMS, items[0].Channel)

	// 按 status 过滤
	items2, total2, err := mgr.ListLogs(ctx, 0, "", LogStatusFailed, 1, 20)
	require.NoError(t, err)
	assert.Equal(t, int64(1), total2)
	assert.Equal(t, LogStatusFailed, items2[0].Status)

	// 全部
	_, total3, err := mgr.ListLogs(ctx, 0, "", "", 1, 20)
	require.NoError(t, err)
	assert.Equal(t, int64(3), total3)
}

// ============== 13. TestDispatch ==============

func TestTestDispatch(t *testing.T) {
	db := setupTestDB(t)
	cache, _ := setupTestCfgCache(t, db, map[string]string{CfgKeyInAppEnabled: "1"})
	mgr := NewManager(db, cache, nil)

	result := mgr.TestDispatch(context.Background(), ChannelInApp, "user1", "标题", "内容", nil)
	assert.Equal(t, LogStatusSent, result.Status)
	assert.NotEmpty(t, result.ProviderMsgID)
}

// ============== 14. 状态机常量 ==============

func TestConstants(t *testing.T) {
	// 铁律 06：常量值固定，确保不被改动
	assert.Equal(t, "sms", ChannelSMS)
	assert.Equal(t, "email", ChannelEmail)
	assert.Equal(t, "inapp", ChannelInApp)

	assert.Equal(t, "verify_code", TemplateVerifyCode)
	assert.Equal(t, "verify_code_email", TemplateVerifyCodeEmail)
	assert.Equal(t, "order_paid", TemplateOrderPaid)
	assert.Equal(t, "agent_commission", TemplateAgentCommission)

	assert.Equal(t, "enabled", TemplateStatusEnabled)
	assert.Equal(t, "disabled", TemplateStatusDisabled)

	assert.Equal(t, "pending", LogStatusPending)
	assert.Equal(t, "sent", LogStatusSent)
	assert.Equal(t, "failed", LogStatusFailed)

	assert.Equal(t, 0, PriorityNormal)
	assert.Equal(t, 1, PriorityHigh)
	assert.Equal(t, 2, PriorityUrgent)
}

// ============== 16. 阿里云短信签名 ==============

// TestSignAliyunRequest 测试阿里云 RPC API 签名算法
// 铁律 06：所有断言基于已知固定输入，签名结果确定性可重现
func TestSignAliyunRequest(t *testing.T) {
	// 阿里云官方文档示例参数
	params := map[string]string{
		"AccessKeyId":      "testid",
		"Action":           "SendSms",
		"Format":           "JSON",
		"PhoneNumbers":     "15300000000",
		"RegionId":         "cn-hangzhou",
		"SignName":         "阿里云短信测试专用",
		"SignatureMethod":  "HMAC-SHA1",
		"SignatureNonce":   "45eac111-9f9d-4f07-8c2b-3c2f5c5d4b01",
		"SignatureVersion": "1.0",
		"TemplateCode":     "SMS_123456789",
		"TemplateParam":    `{"code":"1234"}`,
		"Timestamp":        "2017-07-12T02:42:19Z",
		"Version":          "2017-05-25",
	}
	signature := signAliyunRequest(params, "testsecret")
	// 验证签名格式正确（Base64 编码，长度 > 0）
	if signature == "" {
		t.Fatal("signature should not be empty")
	}
	// 验证签名可解码
	decoded, err := base64.StdEncoding.DecodeString(signature)
	if err != nil {
		t.Fatalf("signature is not valid base64: %v", err)
	}
	if len(decoded) != 20 { // SHA1 输出 20 字节
		t.Fatalf("signature should be 20 bytes (SHA1), got %d", len(decoded))
	}

	// 验证同一输入同一输出（确定性）
	sig2 := signAliyunRequest(params, "testsecret")
	if signature != sig2 {
		t.Fatal("signature should be deterministic")
	}

	// 验证不同 secret 产生不同签名
	sig3 := signAliyunRequest(params, "different_secret")
	if signature == sig3 {
		t.Fatal("different secret should produce different signature")
	}

	// 验证不同参数产生不同签名
	params2 := make(map[string]string)
	for k, v := range params {
		params2[k] = v
	}
	params2["PhoneNumbers"] = "15900000000"
	sig4 := signAliyunRequest(params2, "testsecret")
	if signature == sig4 {
		t.Fatal("different params should produce different signature")
	}
}

// ============== 15. dialSMTPClient 加密分支 ==============

// TestDialSMTPClient_EncryptionBranch 验证 dialSMTPClient 在三种 encryption 模式下
// 对无效主机均能正确返回错误（无真实 SMTP 服务器，仅验证分支选择与错误传播）
func TestDialSMTPClient_EncryptionBranch(t *testing.T) {
	// SSL 模式（465 隐式 TLS）：tls.DialWithDialer 应在超时内失败
	_, err := dialSMTPClient("invalid.host.does.not.exist", 465, "ssl", 2*time.Second)
	if err == nil {
		t.Fatal("dialSMTPClient with invalid host (ssl) should return error")
	}
	assert.Contains(t, err.Error(), "smtp ssl dial")

	// TLS 模式（587 STARTTLS）：net.DialTimeout 应在超时内失败
	_, err = dialSMTPClient("invalid.host.does.not.exist", 587, "tls", 2*time.Second)
	if err == nil {
		t.Fatal("dialSMTPClient with invalid host (tls) should return error")
	}
	assert.Contains(t, err.Error(), "smtp dial")

	// None 模式（25 明文）：net.DialTimeout 应在超时内失败
	_, err = dialSMTPClient("invalid.host.does.not.exist", 25, "none", 2*time.Second)
	if err == nil {
		t.Fatal("dialSMTPClient with invalid host (none) should return error")
	}
	assert.Contains(t, err.Error(), "smtp dial")
}
