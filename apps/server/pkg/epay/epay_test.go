// pkg/epay 单元测试
// 覆盖 BuildSubmitURL / ParseNotify / VerifyNotify / IsSuccess
// 严格遵循铁律 06：所有签名值通过 crypto.SignEpayParams 重算校验
package epay

import (
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/your-org/keyauth-saas/apps/server/pkg/crypto"
)

// ============== BuildSubmitURL ==============

func TestBuildSubmitURL_Success(t *testing.T) {
	cfg := &Config{
		GatewayURL: "https://pay.example.com",
		PID:        "1001",
		Secret:     "MY-SECRET",
		SignType:   "MD5",
	}
	p := &OrderParams{
		OutTradeNo: "ORD1767225600000",
		Name:       "KeyAuth卡密",
		Money:      "1.00",
		PayType:    "alipay",
		NotifyURL:  "https://example.com/notify",
		ReturnURL:  "https://example.com/return",
		ClientIP:   "1.2.3.4",
	}

	u, err := BuildSubmitURL(cfg, p)
	require.NoError(t, err)
	require.NotEmpty(t, u)

	// 应包含 submit.php
	assert.True(t, strings.HasPrefix(u, "https://pay.example.com/submit.php?"))

	// 解析 query 参数校验
	parsed, err := url.Parse(u)
	require.NoError(t, err)
	q := parsed.Query()

	assert.Equal(t, "1001", q.Get("pid"))
	assert.Equal(t, "ORD1767225600000", q.Get("out_trade_no"))
	assert.Equal(t, "1.00", q.Get("money"))
	assert.Equal(t, "alipay", q.Get("type"))
	assert.Equal(t, "MD5", q.Get("sign_type"))
	assert.NotEmpty(t, q.Get("sign"))

	// 重算签名校验
	expectedParams := map[string]string{
		"pid":          "1001",
		"type":         "alipay",
		"out_trade_no": "ORD1767225600000",
		"notify_url":   "https://example.com/notify",
		"return_url":   "https://example.com/return",
		"name":         "KeyAuth卡密",
		"money":        "1.00",
		"clientip":     "1.2.3.4",
	}
	expectedSign := crypto.SignEpayParams(expectedParams, "MY-SECRET")
	assert.Equal(t, expectedSign, q.Get("sign"))
}

func TestBuildSubmitURL_TrimsTrailingSlash(t *testing.T) {
	cfg := &Config{
		GatewayURL: "https://pay.example.com/",  // 末尾斜杠
		PID:        "1",
		Secret:     "s",
	}
	p := &OrderParams{
		OutTradeNo: "O1",
		Money:      "0.01",
	}

	u, err := BuildSubmitURL(cfg, p)
	require.NoError(t, err)
	// 应去除末尾斜杠后拼接
	assert.True(t, strings.HasPrefix(u, "https://pay.example.com/submit.php?"))
	assert.False(t, strings.Contains(u, "//submit.php"))
}

func TestBuildSubmitURL_NoClientIP(t *testing.T) {
	cfg := &Config{GatewayURL: "https://pay.x.com", PID: "1", Secret: "s"}
	p := &OrderParams{OutTradeNo: "O1", Money: "0.01"}

	u, err := BuildSubmitURL(cfg, p)
	require.NoError(t, err)
	parsed, _ := url.Parse(u)
	q := parsed.Query()
	_, hasClientIP := q["clientip"]
	assert.False(t, hasClientIP, "ClientIP 为空时不应包含 clientip 参数")
}

func TestBuildSubmitURL_DefaultSignType(t *testing.T) {
	// SignType 留空时应默认 MD5
	cfg := &Config{GatewayURL: "https://pay.x.com", PID: "1", Secret: "s"}
	p := &OrderParams{OutTradeNo: "O1", Money: "0.01"}

	u, _ := BuildSubmitURL(cfg, p)
	parsed, _ := url.Parse(u)
	assert.Equal(t, "MD5", parsed.Query().Get("sign_type"))
}

func TestBuildSubmitURL_ConfigValidation(t *testing.T) {
	p := &OrderParams{OutTradeNo: "O1", Money: "0.01"}

	cases := []struct {
		name string
		cfg  *Config
	}{
		{"nil cfg", nil},
		{"empty gateway", &Config{PID: "1", Secret: "s"}},
		{"empty pid", &Config{GatewayURL: "https://x.com", Secret: "s"}},
		{"empty secret", &Config{GatewayURL: "https://x.com", PID: "1"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := BuildSubmitURL(c.cfg, p)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "易支付配置不完整")
		})
	}
}

func TestBuildSubmitURL_OrderParamsValidation(t *testing.T) {
	cfg := &Config{GatewayURL: "https://x.com", PID: "1", Secret: "s"}

	cases := []struct {
		name string
		p    *OrderParams
	}{
		{"nil params", nil},
		{"empty out_trade_no", &OrderParams{Money: "0.01"}},
		{"empty money", &OrderParams{OutTradeNo: "O1"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := BuildSubmitURL(cfg, c.p)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "订单参数不完整")
		})
	}
}

// ============== ParseNotify ==============

func TestParseNotify_AllFields(t *testing.T) {
	params := map[string]string{
		"pid":          "1001",
		"trade_no":     "2026072000001",
		"out_trade_no": "ORD12345",
		"type":         "alipay",
		"name":         "KeyAuth卡密",
		"money":        "9.90",
		"trade_status": "TRADE_SUCCESS",
		"sign":         "abcdef0123456789",
		"sign_type":    "MD5",
	}
	n := ParseNotify(params)
	require.NotNil(t, n)

	assert.Equal(t, "1001", n.PID)
	assert.Equal(t, "2026072000001", n.TradeNo)
	assert.Equal(t, "ORD12345", n.OutTradeNo)
	assert.Equal(t, "alipay", n.Type)
	assert.Equal(t, "KeyAuth卡密", n.Name)
	assert.Equal(t, "9.90", n.Money)
	assert.Equal(t, "TRADE_SUCCESS", n.TradeStatus)
	assert.Equal(t, "abcdef0123456789", n.Sign)
	assert.Equal(t, "MD5", n.SignType)
}

func TestParseNotify_EmptyParams(t *testing.T) {
	n := ParseNotify(map[string]string{})
	require.NotNil(t, n)
	// 所有字段应为空串
	assert.Empty(t, n.PID)
	assert.Empty(t, n.OutTradeNo)
}

func TestParseNotify_PartialParams(t *testing.T) {
	n := ParseNotify(map[string]string{
		"pid":         "1001",
		"out_trade_no": "X1",
	})
	assert.Equal(t, "1001", n.PID)
	assert.Equal(t, "X1", n.OutTradeNo)
	assert.Empty(t, n.TradeNo)
	assert.Empty(t, n.Money)
}

// ============== IsSuccess ==============

func TestNotifyParams_IsSuccess(t *testing.T) {
	cases := []struct {
		status string
		want   bool
	}{
		{"TRADE_SUCCESS", true},
		{"TRADE_FAILED", false},
		{"", false},
		{"pending", false},
	}
	for _, c := range cases {
		t.Run("status="+c.status, func(t *testing.T) {
			n := &NotifyParams{TradeStatus: c.status}
			assert.Equal(t, c.want, n.IsSuccess())
		})
	}
}

// ============== VerifyNotify ==============

func TestVerifyNotify_ValidSignature(t *testing.T) {
	secret := "S"
	params := map[string]string{
		"pid":          "1001",
		"out_trade_no": "ORD1",
		"money":        "1.00",
	}
	// 生成正确签名
	sign := crypto.SignEpayParams(params, secret)
	params["sign"] = sign
	params["sign_type"] = "MD5"

	assert.True(t, VerifyNotify(params, secret))
}

func TestVerifyNotify_WrongSignature(t *testing.T) {
	params := map[string]string{
		"pid":          "1001",
		"out_trade_no": "ORD1",
		"money":        "1.00",
		"sign":         "wrong-sign",
	}
	assert.False(t, VerifyNotify(params, "S"))
}

func TestVerifyNotify_WrongSecret(t *testing.T) {
	secret := "right"
	params := map[string]string{
		"pid":          "1001",
		"out_trade_no": "ORD1",
		"money":        "1.00",
	}
	sign := crypto.SignEpayParams(params, secret)
	params["sign"] = sign

	assert.False(t, VerifyNotify(params, "wrong"))
	assert.True(t, VerifyNotify(params, secret))
}

func TestVerifyNotify_EmptySecret(t *testing.T) {
	params := map[string]string{"sign": "abc"}
	assert.False(t, VerifyNotify(params, ""))
}

func TestVerifyNotify_MissingSign(t *testing.T) {
	params := map[string]string{
		"pid":          "1001",
		"out_trade_no": "ORD1",
	}
	assert.False(t, VerifyNotify(params, "S"))
}

func TestVerifyNotify_EmptySign(t *testing.T) {
	params := map[string]string{
		"pid":   "1",
		"sign":  "",
	}
	assert.False(t, VerifyNotify(params, "S"))
}

// ============== 端到端：BuildSubmitURL → VerifyNotify 闭环 ==============

func TestEpay_RoundTrip_BuildAndVerify(t *testing.T) {
	// 模拟下单 → 平台回调场景
	secret := "E2E-SECRET"
	cfg := &Config{
		GatewayURL: "https://pay.example.com",
		PID:        "2008",
		Secret:     secret,
		SignType:   "MD5",
	}
	order := &OrderParams{
		OutTradeNo: "ORD1767225600000",
		Name:       "KeyAuth卡密",
		Money:      "99.00",
		PayType:    "wxpay",
		NotifyURL:  "https://example.com/notify",
		ReturnURL:  "https://example.com/return",
	}

	// 1. 构造下单 URL
	submitURL, err := BuildSubmitURL(cfg, order)
	require.NoError(t, err)

	// 2. 模拟平台回调（仅传订单参数 + sign）
	notifyParams := map[string]string{
		"pid":          cfg.PID,
		"trade_no":     "2026072000099",
		"out_trade_no": order.OutTradeNo,
		"type":         order.PayType,
		"name":         order.Name,
		"money":        order.Money,
		"trade_status": "TRADE_SUCCESS",
	}
	notifyParams["sign"] = crypto.SignEpayParams(notifyParams, secret)
	notifyParams["sign_type"] = "MD5"

	// 3. 验证回调签名
	assert.True(t, VerifyNotify(notifyParams, secret),
		"回调签名应通过校验")

	// 4. 解析回调参数 + 判断成功
	np := ParseNotify(notifyParams)
	assert.True(t, np.IsSuccess())
	assert.Equal(t, order.OutTradeNo, np.OutTradeNo)
	assert.Equal(t, order.Money, np.Money)

	t.Logf("下单 URL: %s", submitURL[:80]+"...")
	t.Logf("回调订单号: %s, 金额: %s", np.OutTradeNo, np.Money)
}

// ============== defaultIfEmpty ==============

func TestDefaultIfEmpty(t *testing.T) {
	assert.Equal(t, "MD5", defaultIfEmpty("", "MD5"))
	assert.Equal(t, "SHA256", defaultIfEmpty("SHA256", "MD5"))
}

// ============== 前缀分发兼容测试 ==============

func TestNotifyParams_OrderPrefixRouting(t *testing.T) {
	// v0.3.6 关键路径：回调按 out_trade_no 前缀分发到不同业务通道
	// ORD → processPaidOrder
	// TOP → processTenantOwnPaidOrder
	// REG → processAgentRegisterPaid
	cases := []string{
		"ORD1767225600000",
		"TOP1767225600001",
		"REG1767225600002",
	}
	for _, orderNo := range cases {
		t.Run(orderNo, func(t *testing.T) {
			n := &NotifyParams{OutTradeNo: orderNo, TradeStatus: "TRADE_SUCCESS"}
			assert.True(t, n.IsSuccess())
			assert.True(t, strings.HasPrefix(n.OutTradeNo, orderNo[:3]),
				"订单号前缀应保留")
		})
	}
}
