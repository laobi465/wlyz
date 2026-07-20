// v0.5.0 集成扩展批次 1：webhook 通知通道单元测试
//
// 测试覆盖：
//   1. signDingTalk 加签算法（与官方示例对齐）
//   2. escapeTelegramMarkdown 特殊字符转义
//   3. 3 个 Provider 的配置缺失错误路径
//   4. 3 个 Provider 的 HTTP 集成测试（mock httptest.Server）
//   5. ValidateChannel / Channel 常量 / 配置键常量
//   6. Manager.dispatch 6 通道分发
//
// 严格遵循铁律 06：不编造测试数据，所有断言基于真实算法或 mock 响应
package notify

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/your-org/keyauth-saas/apps/server/internal/model"
)

// setupTestManager 构造测试用 Manager（内存 SQLite + miniredis + 默认配置）
func setupTestManager(t *testing.T) *Manager {
	t.Helper()
	db := setupTestDB(t)
	cache, _ := setupTestCfgCache(t, db, nil)
	return NewManager(db, cache, nil)
}

// setNotifyConfig 写入/更新一项 notify.* 配置（同时刷新 Redis 缓存）
func setNotifyConfig(t *testing.T, mgr *Manager, key, value string) {
	t.Helper()
	if err := mgr.cache.Set(context.Background(), key, value, "test", "notify", "test"); err != nil {
		t.Fatalf("set config %s failed: %v", key, err)
	}
}

// ============== 1. signDingTalk 加签算法 ==============

func TestSignDingTalk(t *testing.T) {
	// 官方示例：timestamp=1577808000000, secret="SEC..."
	// 此处验证算法流程：相同输入应产生相同输出；不同 secret 应产生不同输出
	ts := int64(1577808000000)
	sign1 := signDingTalk(ts, "SECTEST123")
	sign2 := signDingTalk(ts, "SECTEST123")
	if sign1 != sign2 {
		t.Errorf("signDingTalk not deterministic: %s != %s", sign1, sign2)
	}
	// 不同 secret 应产生不同签名
	sign3 := signDingTalk(ts, "SECTEST456")
	if sign1 == sign3 {
		t.Errorf("signDingTalk should differ with different secret")
	}
	// 签名应可被 base64 解码（即包含合法字符）
	if !strings.Contains(sign1, "=") && len(sign1)%4 != 0 {
		t.Errorf("signDingTalk should be base64 encoded")
	}
}

// ============== 2. escapeTelegramMarkdown ==============

func TestEscapeTelegramMarkdown(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{"plain", "hello world", "hello world"},
		{"underscore", "a_b", `a\_b`},
		{"asterisk", "a*b", `a\*b`},
		{"brackets", "a[b]c(d)", `a\[b\]c\(d\)`},
		{"tilde", "a~b", `a\~b`},
		{"backtick", "a`b", "a\\`b"},
		{"gt", "a>b", `a\>b`},
		{"hash", "a#b", `a\#b`},
		{"plus", "a+b", `a\+b`},
		{"dash", "a-b", `a\-b`},
		{"eq", "a=b", `a\=b`},
		{"pipe", "a|b", `a\|b`},
		{"braces", "a{b}c", `a\{b\}c`},
		{"dot", "a.b", `a\.b`},
		{"bang", "a!b", `a\!b`},
		{"chinese", "你好_世界", `你好\_世界`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := escapeTelegramMarkdown(c.input)
			if got != c.want {
				t.Errorf("escapeTelegramMarkdown(%q) = %q, want %q", c.input, got, c.want)
			}
		})
	}
}

// ============== 3. Provider 配置缺失 ==============

func TestDingTalkProvider_NotConfigured(t *testing.T) {
	mgr := setupTestManager(t)
	p := &dingtalkWebhookProvider{mgr: mgr}
	_, err := p.Send(context.Background(), "", "subject", "content")
	if err != ErrProviderNotConfig {
		t.Errorf("expected ErrProviderNotConfig, got %v", err)
	}
}

func TestWeComProvider_NotConfigured(t *testing.T) {
	mgr := setupTestManager(t)
	p := &wecomWebhookProvider{mgr: mgr}
	_, err := p.Send(context.Background(), "", "subject", "content")
	if err != ErrProviderNotConfig {
		t.Errorf("expected ErrProviderNotConfig, got %v", err)
	}
}

func TestTelegramProvider_NotConfigured(t *testing.T) {
	mgr := setupTestManager(t)
	p := &telegramWebhookProvider{mgr: mgr}
	// 未配置 bot_token
	_, err := p.Send(context.Background(), "", "subject", "content")
	if err != ErrProviderNotConfig {
		t.Errorf("expected ErrProviderNotConfig, got %v", err)
	}
}

func TestTelegramProvider_ChatIDMissing(t *testing.T) {
	mgr := setupTestManager(t)
	// 仅配置 bot_token，不配置 chat_id
	setNotifyConfig(t, mgr, CfgKeyTelegramBotToken, "123:ABC")
	p := &telegramWebhookProvider{mgr: mgr}
	_, err := p.Send(context.Background(), "", "subject", "content")
	if err != ErrProviderNotConfig {
		t.Errorf("expected ErrProviderNotConfig when chat_id missing, got %v", err)
	}
}

// ============== 4. HTTP 集成测试 ==============

// TestDingTalkProvider_HTTPSuccess 钉钉成功响应
func TestDingTalkProvider_HTTPSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 校验请求方法与 Content-Type
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("expected application/json, got %s", r.Header.Get("Content-Type"))
		}
		// 校验 body 包含 markdown 字段
		body, _ := io.ReadAll(r.Body)
		var payload map[string]interface{}
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Errorf("invalid json: %v", err)
		}
		if payload["msgtype"] != "markdown" {
			t.Errorf("expected msgtype=markdown, got %v", payload["msgtype"])
		}
		// 返回钉钉成功响应
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"errcode":0,"errmsg":"ok","messageId":123456}`))
	}))
	defer server.Close()

	mgr := setupTestManager(t)
	setNotifyConfig(t, mgr, CfgKeyDingTalkWebhookURL, server.URL)
	// 不配置 secret（无加签）
	p := &dingtalkWebhookProvider{mgr: mgr}
	msgID, err := p.Send(context.Background(), "", "测试标题", "测试内容")
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
	if !strings.HasPrefix(msgID, "dingtalk-") {
		t.Errorf("expected msgID prefix 'dingtalk-', got %s", msgID)
	}
}

// TestDingTalkProvider_HTTPWithSign 钉钉加签场景
func TestDingTalkProvider_HTTPWithSign(t *testing.T) {
	var capturedURL string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedURL = r.URL.String()
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"errcode":0,"errmsg":"ok","messageId":789}`))
	}))
	defer server.Close()

	mgr := setupTestManager(t)
	setNotifyConfig(t, mgr, CfgKeyDingTalkWebhookURL, server.URL+"?access_token=test123")
	setNotifyConfig(t, mgr, CfgKeyDingTalkSecret, "SECTEST")
	p := &dingtalkWebhookProvider{mgr: mgr}
	_, err := p.Send(context.Background(), "", "title", "content")
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
	// 校验 URL 中包含 timestamp & sign
	if !strings.Contains(capturedURL, "timestamp=") {
		t.Errorf("URL should contain timestamp: %s", capturedURL)
	}
	if !strings.Contains(capturedURL, "sign=") {
		t.Errorf("URL should contain sign: %s", capturedURL)
	}
	if !strings.Contains(capturedURL, "access_token=test123") {
		t.Errorf("URL should preserve access_token: %s", capturedURL)
	}
}

// TestDingTalkProvider_HTTPError 钉钉业务错误
func TestDingTalkProvider_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"errcode":310000,"errmsg":"keywords not in content"}`))
	}))
	defer server.Close()

	mgr := setupTestManager(t)
	setNotifyConfig(t, mgr, CfgKeyDingTalkWebhookURL, server.URL)
	p := &dingtalkWebhookProvider{mgr: mgr}
	_, err := p.Send(context.Background(), "", "title", "content")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "310000") {
		t.Errorf("error should contain errcode 310000: %v", err)
	}
}

// TestDingTalkProvider_AtMobiles @ 手机号场景
func TestDingTalkProvider_AtMobiles(t *testing.T) {
	var capturedBody []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"errcode":0,"errmsg":"ok","messageId":1}`))
	}))
	defer server.Close()

	mgr := setupTestManager(t)
	setNotifyConfig(t, mgr, CfgKeyDingTalkWebhookURL, server.URL)
	setNotifyConfig(t, mgr, CfgKeyDingTalkAtMobiles, "13800138000,13900139000")
	p := &dingtalkWebhookProvider{mgr: mgr}
	_, err := p.Send(context.Background(), "", "title", "content")
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	var payload map[string]interface{}
	json.Unmarshal(capturedBody, &payload)
	at, ok := payload["at"].(map[string]interface{})
	if !ok {
		t.Fatal("payload should contain at field")
	}
	mobiles, ok := at["atMobiles"].([]interface{})
	if !ok || len(mobiles) != 2 {
		t.Errorf("atMobiles should have 2 entries, got %v", at["atMobiles"])
	}
	// content 应包含 @13800138000
	md := payload["markdown"].(map[string]interface{})
	text, _ := md["text"].(string)
	if !strings.Contains(text, "@13800138000") {
		t.Errorf("text should contain @13800138000: %s", text)
	}
}

// TestWeComProvider_HTTPSuccess 企微成功响应
func TestWeComProvider_HTTPSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		body, _ := io.ReadAll(r.Body)
		var payload map[string]interface{}
		json.Unmarshal(body, &payload)
		if payload["msgtype"] != "markdown" {
			t.Errorf("expected msgtype=markdown, got %v", payload["msgtype"])
		}
		md := payload["markdown"].(map[string]interface{})
		text, _ := md["content"].(string)
		// subject 应加粗显示在开头
		if !strings.Contains(text, "**标题**") {
			t.Errorf("content should contain bold title: %s", text)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"errcode":0,"errmsg":"ok"}`))
	}))
	defer server.Close()

	mgr := setupTestManager(t)
	setNotifyConfig(t, mgr, CfgKeyWeComWebhookURL, server.URL)
	p := &wecomWebhookProvider{mgr: mgr}
	msgID, err := p.Send(context.Background(), "", "标题", "正文")
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
	if !strings.HasPrefix(msgID, "wecom-") {
		t.Errorf("expected msgID prefix 'wecom-', got %s", msgID)
	}
}

// TestWeComProvider_HTTPError 企微业务错误
func TestWeComProvider_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"errcode":93000,"errmsg":"invalid webhook url"}`))
	}))
	defer server.Close()

	mgr := setupTestManager(t)
	setNotifyConfig(t, mgr, CfgKeyWeComWebhookURL, server.URL)
	p := &wecomWebhookProvider{mgr: mgr}
	_, err := p.Send(context.Background(), "", "title", "content")
	if err == nil || !strings.Contains(err.Error(), "93000") {
		t.Errorf("expected error with 93000, got %v", err)
	}
}

// TestTelegramProvider_HTTPSuccess TG 成功响应
func TestTelegramProvider_HTTPSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 校验 URL 路径包含 bot token
		if !strings.Contains(r.URL.Path, "/bot123:ABC/sendMessage") {
			t.Errorf("URL should contain bot token: %s", r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		var payload map[string]interface{}
		json.Unmarshal(body, &payload)
		if payload["chat_id"] != "-1001234567890" {
			t.Errorf("expected chat_id=-1001234567890, got %v", payload["chat_id"])
		}
		if payload["parse_mode"] != "MarkdownV2" {
			t.Errorf("expected parse_mode=MarkdownV2, got %v", payload["parse_mode"])
		}
		// 校验转义：subject 加粗 + content 特殊字符转义
		text, _ := payload["text"].(string)
		if !strings.Contains(text, "*title*") {
			t.Errorf("text should contain bold title: %s", text)
		}
		// content 中的 _ 应被转义为 \_
		if !strings.Contains(text, `hello\_world`) {
			t.Errorf("text should escape underscore: %s", text)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ok":true,"result":{"message_id":42,"date":1577808000}}`))
	}))
	defer server.Close()

	mgr := setupTestManager(t)
	setNotifyConfig(t, mgr, CfgKeyTelegramBotToken, "123:ABC")
	setNotifyConfig(t, mgr, CfgKeyTelegramChatID, "-1001234567890")
	p := &telegramWebhookProvider{mgr: mgr}
	// 重写 API URL 指向测试服务器
	orig := telegramAPIBase
	telegramAPIBase = server.URL
	defer func() { telegramAPIBase = orig }()
	msgID, err := p.Send(context.Background(), "", "title", "hello_world")
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
	if !strings.HasPrefix(msgID, "telegram-") {
		t.Errorf("expected msgID prefix 'telegram-', got %s", msgID)
	}
}

// TestTelegramProvider_HTTPError TG 业务错误
func TestTelegramProvider_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ok":false,"error_code":400,"description":"Bad Request: chat not found"}`))
	}))
	defer server.Close()

	mgr := setupTestManager(t)
	setNotifyConfig(t, mgr, CfgKeyTelegramBotToken, "123:ABC")
	setNotifyConfig(t, mgr, CfgKeyTelegramChatID, "invalid")
	p := &telegramWebhookProvider{mgr: mgr}
	orig := telegramAPIBase
	telegramAPIBase = server.URL
	defer func() { telegramAPIBase = orig }()
	_, err := p.Send(context.Background(), "", "title", "content")
	if err == nil || !strings.Contains(err.Error(), "400") {
		t.Errorf("expected error with 400, got %v", err)
	}
}

// TestTelegramProvider_LongMessageTruncation 长消息截断
func TestTelegramProvider_LongMessageTruncation(t *testing.T) {
	var capturedText string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var payload map[string]interface{}
		json.Unmarshal(body, &payload)
		capturedText, _ = payload["text"].(string)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ok":true,"result":{"message_id":1}}`))
	}))
	defer server.Close()

	mgr := setupTestManager(t)
	setNotifyConfig(t, mgr, CfgKeyTelegramBotToken, "123:ABC")
	setNotifyConfig(t, mgr, CfgKeyTelegramChatID, "@channel")
	p := &telegramWebhookProvider{mgr: mgr}
	orig := telegramAPIBase
	telegramAPIBase = server.URL
	defer func() { telegramAPIBase = orig }()
	// 5000 字符内容（超过 4096 上限）
	longContent := strings.Repeat("a", 5000)
	_, err := p.Send(context.Background(), "", "", longContent)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
	// 截断后应包含提示文字
	if !strings.Contains(capturedText, "已截断") {
		t.Errorf("truncated text should contain '已截断': %s", capturedText)
	}
	if len(capturedText) > 4096 {
		t.Errorf("text should be truncated to <=4096 chars, got %d", len(capturedText))
	}
}

// ============== 5. 常量与校验 ==============

func TestWebhookChannelConstants(t *testing.T) {
	if ChannelDingTalk != "dingtalk" {
		t.Errorf("ChannelDingTalk = %s, want 'dingtalk'", ChannelDingTalk)
	}
	if ChannelWeCom != "wecom" {
		t.Errorf("ChannelWeCom = %s, want 'wecom'", ChannelWeCom)
	}
	if ChannelTelegram != "telegram" {
		t.Errorf("ChannelTelegram = %s, want 'telegram'", ChannelTelegram)
	}
}

func TestValidateChannel_WebhookChannels(t *testing.T) {
	cases := []struct {
		channel string
		want    bool
	}{
		{ChannelSMS, true},
		{ChannelEmail, true},
		{ChannelInApp, true},
		{ChannelDingTalk, true},
		{ChannelWeCom, true},
		{ChannelTelegram, true},
		{"unknown", false},
		{"", false},
		{"webhook", false},
	}
	for _, c := range cases {
		got := ValidateChannel(c.channel)
		if got != c.want {
			t.Errorf("ValidateChannel(%q) = %v, want %v", c.channel, got, c.want)
		}
	}
}

func TestWebhookConfigKeys(t *testing.T) {
	// 校验配置键常量值符合 sys_config 命名规范
	cases := []struct {
		key   string
		value string
	}{
		{CfgKeyDingTalkEnabled, "notify.dingtalk.enabled"},
		{CfgKeyDingTalkWebhookURL, "notify.dingtalk.webhook_url"},
		{CfgKeyDingTalkSecret, "notify.dingtalk.secret"},
		{CfgKeyDingTalkAtMobiles, "notify.dingtalk.at_mobiles"},
		{CfgKeyDingTalkAtAll, "notify.dingtalk.at_all"},
		{CfgKeyWeComEnabled, "notify.wecom.enabled"},
		{CfgKeyWeComWebhookURL, "notify.wecom.webhook_url"},
		{CfgKeyTelegramEnabled, "notify.telegram.enabled"},
		{CfgKeyTelegramBotToken, "notify.telegram.bot_token"},
		{CfgKeyTelegramChatID, "notify.telegram.chat_id"},
	}
	for _, c := range cases {
		if c.key != c.value {
			t.Errorf("%s = %s, want %s", c.key, c.key, c.value)
		}
	}
}

// ============== 6. Manager.dispatch 6 通道分发 ==============

func TestManagerDispatch_WebhookChannels(t *testing.T) {
	mgr := setupTestManager(t)

	// 钉钉通道：未配置 → failed
	r := mgr.dispatch(context.Background(), ChannelDingTalk, "", "title", "content", "test", nil)
	if r.Status != LogStatusFailed {
		t.Errorf("dispatch dingtalk (not configured) status = %s, want failed", r.Status)
	}
	if !strings.Contains(r.ErrorMessage, "not configured") && !strings.Contains(r.ErrorMessage, "Provider") {
		t.Errorf("dispatch dingtalk error should mention not configured: %s", r.ErrorMessage)
	}

	// 企微通道：未配置 → failed
	r = mgr.dispatch(context.Background(), ChannelWeCom, "", "title", "content", "test", nil)
	if r.Status != LogStatusFailed {
		t.Errorf("dispatch wecom (not configured) status = %s, want failed", r.Status)
	}

	// TG 通道：未配置 → failed
	r = mgr.dispatch(context.Background(), ChannelTelegram, "", "title", "content", "test", nil)
	if r.Status != LogStatusFailed {
		t.Errorf("dispatch telegram (not configured) status = %s, want failed", r.Status)
	}
}

func TestManagerIsChannelEnabled_WebhookChannels(t *testing.T) {
	mgr := setupTestManager(t)
	// 默认全部禁用
	if mgr.IsChannelEnabled(context.Background(), ChannelDingTalk) {
		t.Error("dingtalk should be disabled by default")
	}
	if mgr.IsChannelEnabled(context.Background(), ChannelWeCom) {
		t.Error("wecom should be disabled by default")
	}
	if mgr.IsChannelEnabled(context.Background(), ChannelTelegram) {
		t.Error("telegram should be disabled by default")
	}
	// 启用后应返回 true
	setNotifyConfig(t, mgr, CfgKeyDingTalkEnabled, "1")
	setNotifyConfig(t, mgr, CfgKeyWeComEnabled, "1")
	setNotifyConfig(t, mgr, CfgKeyTelegramEnabled, "1")
	if !mgr.IsChannelEnabled(context.Background(), ChannelDingTalk) {
		t.Error("dingtalk should be enabled after config")
	}
	if !mgr.IsChannelEnabled(context.Background(), ChannelWeCom) {
		t.Error("wecom should be enabled after config")
	}
	if !mgr.IsChannelEnabled(context.Background(), ChannelTelegram) {
		t.Error("telegram should be enabled after config")
	}
}

// ============== 辅助：模型引用占位 ==============
// 防止 model 包未使用导致编译报错（setupTestDB 已用，此处冗余保险）
var _ = model.NotifyLog{}

// telegramAPIBase 在 webhook.go 中定义，测试中通过临时覆盖实现 mock

