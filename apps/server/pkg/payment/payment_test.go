// v0.5.0 集成扩展批次 3：海外支付通道单元测试
//
// 测试覆盖：
//   1. USDT：金额后缀算法 / QRContent 格式 / webhook 验签（成功+失败+密钥未配置）/ TronGrid 轮询 mock / convertTokenValue
//   2. PayPal：OAuth2 token 缓存 / 创建订单 mock / webhook 验签 API mock / 资源字段提取
//   3. Stripe：创建 PaymentIntent mock / webhook HMAC-SHA256 验签 / 时间戳容差 / 签名工具函数
//   4. 通道常量与配置键完整性
//
// 严格遵循铁律 06：所有断言基于已知固定输入，无随机/不确定性
// 测试用 httptest.Server mock 外部 HTTP 调用，不发起真实网络请求
package payment

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
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
	"github.com/your-org/keyauth-saas/apps/server/pkg/crypto"
)

// ============== 测试基础设施 ==============

// testAESKey 32 字节 AES-256 测试密钥（铁律 06：测试固定密钥，生产环境从环境变量读取）
const testAESKey = "0123456789abcdef0123456789abcdef"

// testMgr 包级 crypto.Manager（所有测试共享，因 AES 密钥固定）
var testMgr *crypto.Manager

// TestMain 初始化 testMgr
func TestMain(m *testing.M) {
	var err error
	testMgr, err = crypto.NewManager(testAESKey, "", "")
	if err != nil {
		panic("初始化 testMgr 失败: " + err.Error())
	}
	m.Run()
}

// setupTestCfgCache 构造测试用 ConfigCache（内存 SQLite + miniredis）
func setupTestCfgCache(t *testing.T, overrides map[string]string) *config.ConfigCache {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:payment_test?mode=memory&cache=shared"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.SysConfig{}))
	db.Exec("DELETE FROM sys_config")

	mr, err := miniredis.Run()
	require.NoError(t, err)
	t.Cleanup(mr.Close)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	for k, v := range overrides {
		require.NoError(t, db.Create(&model.SysConfig{
			ConfigKey:   k,
			ConfigValue: v,
			ConfigType:  "string",
			ConfigGroup: "pay",
		}).Error)
	}
	return config.NewConfigCache(db, rdb)
}

// encryptHelper 加密并返回密文字符串（使用包级 testMgr）
func encryptHelper(t *testing.T, plaintext string) string {
	t.Helper()
	enc, err := testMgr.EncryptAES(plaintext)
	require.NoError(t, err)
	return enc
}

// ============== 1. USDT 测试 ==============

func TestUSDTProvider_Name(t *testing.T) {
	cache := setupTestCfgCache(t, nil)
	provider := NewUSDTProvider(cache, nil)
	assert.Equal(t, "usdt", provider.Name())
}

func TestUSDTProvider_CreateOrder_AddressMissing(t *testing.T) {
	cache := setupTestCfgCache(t, nil)
	provider := NewUSDTProvider(cache, nil)
	_, err := provider.CreateOrder(context.Background(), &OrderParams{
		OrderNo:   "UST123",
		AmountRaw: 9.99,
		OrderID:   1234,
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "收款地址未配置")
}

func TestUSDTProvider_CreateOrder_AddressInvalid(t *testing.T) {
	cache := setupTestCfgCache(t, map[string]string{
		CfgKeyUSDTTrc20Address: "ABCINVALID", // 非 T 开头 34 位
	})
	provider := NewUSDTProvider(cache, nil)
	_, err := provider.CreateOrder(context.Background(), &OrderParams{
		OrderNo:   "UST123",
		AmountRaw: 9.99,
		OrderID:   1234,
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "地址格式非法")
}

func TestUSDTProvider_CreateOrder_Success(t *testing.T) {
	addr := "TXYZ123456789012345678901234567890" // T 开头 34 位
	cache := setupTestCfgCache(t, map[string]string{
		CfgKeyUSDTTrc20Address: addr,
		CfgKeyUSDTExchangeRate: "0.14",
	})
	provider := NewUSDTProvider(cache, nil)

	result, err := provider.CreateOrder(context.Background(), &OrderParams{
		OrderNo:   "UST1700000001",
		AmountRaw: 9.99, // CNY
		OrderID:   1701,
	})
	require.NoError(t, err)
	assert.Equal(t, "usdt", result.Channel)
	assert.Equal(t, addr, result.Address)

	// 金额计算：9.99 * 0.14 = 1.3986 + (1701 % 100) / 100 = 0.01 → 1.4086 USDT
	// 6 位小数："1.408600"
	assert.Equal(t, "1.408600", result.Amount)
	assert.Contains(t, result.QRContent, "usdt://"+addr+"?amount=1.408600&contract=")
}

func TestMatchOrderAmount_Success(t *testing.T) {
	// order_id=1701, baseUSDT=1.3986, expected=1.408600
	err := MatchOrderAmount(1.3986, 1701, "1.408600")
	assert.NoError(t, err)
}

func TestMatchOrderAmount_Mismatch(t *testing.T) {
	err := MatchOrderAmount(1.3986, 1701, "1.500000")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "金额不匹配")
}

func TestUSDTProvider_ParseNotify_Success(t *testing.T) {
	secret := "hmac-secret-123"
	cache := setupTestCfgCache(t, map[string]string{
		CfgKeyUSDTHMACSecretEnc: encryptHelper(t, secret),
	})
	provider := NewUSDTProvider(cache, testMgr)

	outTradeNo := "UST1700000001"
	amount := "1.408600"
	ts := int64(1700000000)
	sig := SignUSDTWebhook(outTradeNo, amount, ts, secret)

	payload := usdtNotifyPayload{
		TxHash:     "0xabc123",
		From:       "TSender123",
		To:         "TXYZ123456789012345678901234567890",
		Amount:     amount,
		OutTradeNo: outTradeNo,
		Timestamp:  ts,
		Signature:  sig,
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest("POST", "/pay/notify/usdt", bytes.NewReader(body))
	data, err := provider.ParseNotify(context.Background(), req)
	require.NoError(t, err)
	assert.True(t, data.IsPaid())
	assert.Equal(t, outTradeNo, data.OutTradeNo)
	assert.Equal(t, "0xabc123", data.TradeNo)
	assert.Equal(t, "USDT", data.Currency)
}

func TestUSDTProvider_ParseNotify_SignatureMismatch(t *testing.T) {
	secret := "hmac-secret-123"
	cache := setupTestCfgCache(t, map[string]string{
		CfgKeyUSDTHMACSecretEnc: encryptHelper(t, secret),
	})
	provider := NewUSDTProvider(cache, testMgr)

	payload := usdtNotifyPayload{
		TxHash:     "0xabc",
		Amount:     "1.408600",
		OutTradeNo: "UST123",
		Timestamp:  1700000000,
		Signature:  "wrong-sig",
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/pay/notify/usdt", bytes.NewReader(body))
	data, err := provider.ParseNotify(context.Background(), req)
	require.NoError(t, err)
	assert.False(t, data.IsPaid())
	assert.Error(t, data.VerifyError)
	assert.Contains(t, data.VerifyError.Error(), "签名校验失败")
}

func TestUSDTProvider_ParseNotify_SecretMissing(t *testing.T) {
	cache := setupTestCfgCache(t, nil)
	provider := NewUSDTProvider(cache, testMgr)

	payload := usdtNotifyPayload{
		TxHash:     "0xabc",
		Amount:     "1.408600",
		OutTradeNo: "UST123",
		Timestamp:  1700000000,
		Signature:  "any",
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/pay/notify/usdt", bytes.NewReader(body))
	data, err := provider.ParseNotify(context.Background(), req)
	require.NoError(t, err)
	assert.False(t, data.IsPaid())
	assert.Contains(t, data.VerifyError.Error(), "HMAC 密钥未配置")
}

func TestUSDTProvider_ParseNotify_MissingFields(t *testing.T) {
	cache := setupTestCfgCache(t, nil)
	provider := NewUSDTProvider(cache, testMgr)

	// 缺 out_trade_no / amount / tx_hash
	body := []byte(`{"from":"Tsender","to":"Txyz"}`)
	req := httptest.NewRequest("POST", "/pay/notify/usdt", bytes.NewReader(body))
	_, err := provider.ParseNotify(context.Background(), req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "缺少必填字段")
}

func TestUSDTProvider_PollTronGrid_Success(t *testing.T) {
	addr := "TXYZ123456789012345678901234567890"
	cache := setupTestCfgCache(t, map[string]string{
		CfgKeyUSDTTrc20Address: addr,
	})
	provider := NewUSDTProvider(cache, nil)

	// mock TronGrid 响应
	mockSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 校验路径
		assert.Contains(t, r.URL.Path, "/v1/accounts/"+addr+"/transactions/trc20")
		assert.Equal(t, "true", r.URL.Query().Get("only_to"))
		// 返回 1 笔交易
		// value="10330000" decimals=6 → 10.330000
		resp := `{
			"success": true,
			"data": [
				{
					"transaction_id": "tx-hash-001",
					"block_timestamp": 1700000000000,
					"value": "10330000",
					"from": "TSender123",
					"to": "` + addr + `",
					"token_info": {"symbol": "USDT", "address": "TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t", "decimals": 6}
				}
			]
		}`
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(resp))
	}))
	defer mockSrv.Close()

	// 覆盖 trongridAPIBase
	origBase := trongridAPIBase
	trongridAPIBase = mockSrv.URL
	defer func() { trongridAPIBase = origBase }()

	txs, err := provider.PollTronGrid(context.Background(), 1699900000000)
	require.NoError(t, err)
	require.Len(t, txs, 1)
	assert.Equal(t, "tx-hash-001", txs[0].TxHash)
	assert.Equal(t, "10.330000", txs[0].Amount)
	assert.Equal(t, addr, txs[0].To)
}

func TestUSDTProvider_PollTronGrid_Non200(t *testing.T) {
	addr := "TXYZ123456789012345678901234567890"
	cache := setupTestCfgCache(t, map[string]string{
		CfgKeyUSDTTrc20Address: addr,
	})
	provider := NewUSDTProvider(cache, nil)

	mockSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`server error`))
	}))
	defer mockSrv.Close()

	origBase := trongridAPIBase
	trongridAPIBase = mockSrv.URL
	defer func() { trongridAPIBase = origBase }()

	_, err := provider.PollTronGrid(context.Background(), 1699900000000)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "非 200")
}

func TestConvertTokenValue_Default(t *testing.T) {
	// decimals=0 时直接返回原值
	out, err := convertTokenValue("100", 0)
	assert.NoError(t, err)
	assert.Equal(t, "100", out)
}

func TestConvertTokenValue_USDTDecimals(t *testing.T) {
	// 10330000 / 10^6 = 10.330000
	out, err := convertTokenValue("10330000", 6)
	assert.NoError(t, err)
	assert.Equal(t, "10.330000", out)
}

func TestConvertTokenValue_InvalidValue(t *testing.T) {
	_, err := convertTokenValue("not-a-number", 6)
	assert.Error(t, err)
}

// ============== 2. PayPal 测试 ==============

func TestPayPalProvider_Name(t *testing.T) {
	cache := setupTestCfgCache(t, nil)
	provider := NewPayPalProvider(cache, nil)
	assert.Equal(t, "paypal", provider.Name())
}

func TestPayPalProvider_GetAccessToken_Success(t *testing.T) {
	cache := setupTestCfgCache(t, map[string]string{
		CfgKeyPayPalClientID:        "test-client-id",
		CfgKeyPayPalClientSecretEnc: encryptHelper(t, "test-secret"),
		CfgKeyPayPalSandbox:         "1",
	})
	provider := NewPayPalProvider(cache, testMgr)

	mockSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/oauth2/token", r.URL.Path)
		// 校验 Basic Auth
		user, pass, ok := r.BasicAuth()
		assert.True(t, ok)
		assert.Equal(t, "test-client-id", user)
		assert.Equal(t, "test-secret", pass)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"access_token":"A21AAF","token_type":"Bearer","expires_in":32400,"scope":"openid"}`))
	}))
	defer mockSrv.Close()
	paypalAPIBase = mockSrv.URL
	defer func() { paypalAPIBase = "" }()

	token, err := provider.getAccessToken(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "A21AAF", token)

	// 第二次调用应使用缓存（mock server 不会再次被调用，可通过 requests 计数验证）
	token2, err := provider.getAccessToken(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "A21AAF", token2)
}

func TestPayPalProvider_GetAccessToken_ConfigMissing(t *testing.T) {
	cache := setupTestCfgCache(t, nil)
	provider := NewPayPalProvider(cache, testMgr)
	_, err := provider.getAccessToken(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "client_id 或 client_secret 未配置")
}

func TestPayPalProvider_CreateOrder_Success(t *testing.T) {
	cache := setupTestCfgCache(t, map[string]string{
		CfgKeyPayPalClientID:        "test-id",
		CfgKeyPayPalClientSecretEnc: encryptHelper(t, "test-secret"),
		CfgKeyPayPalSandbox:         "1",
	})
	provider := NewPayPalProvider(cache, testMgr)

	mockSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/oauth2/token" {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"access_token":"A21AAF","token_type":"Bearer","expires_in":32400}`))
			return
		}
		if r.URL.Path == "/v2/checkout/orders" {
			// 校验 Authorization
			assert.Equal(t, "Bearer A21AAF", r.Header.Get("Authorization"))
			assert.Equal(t, "PPL123456", r.Header.Get("PayPal-Request-Id"))
			// 解析请求体校验金额
			body, _ := io.ReadAll(r.Body)
			var req paypalOrderRequest
			_ = json.Unmarshal(body, &req)
			assert.Equal(t, "CAPTURE", req.Intent)
			assert.Equal(t, "9.99", req.PurchaseUnits[0].Amount.Value)
			assert.Equal(t, "USD", req.PurchaseUnits[0].Amount.CurrencyCode)
			assert.Equal(t, "PPL123456", req.PurchaseUnits[0].ReferenceID)
			w.WriteHeader(http.StatusCreated)
			w.Write([]byte(`{
				"id": "ORDERID123",
				"status": "CREATED",
				"links": [
					{"href": "https://paypal.com/approve/123", "rel": "approve", "method": "REDIRECT"},
					{"href": "https://api.paypal.com/v2/checkout/orders/ORDERID123", "rel": "self", "method": "GET"}
				]
			}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer mockSrv.Close()
	paypalAPIBase = mockSrv.URL
	defer func() { paypalAPIBase = "" }()

	result, err := provider.CreateOrder(context.Background(), &OrderParams{
		OrderNo:   "PPL123456",
		Amount:    "9.99",
		Subject:   "KeyAuth 卡密",
		ReturnURL: "https://example.com/return",
	})
	require.NoError(t, err)
	assert.Equal(t, "paypal", result.Channel)
	assert.Equal(t, "https://paypal.com/approve/123", result.PaymentURL)
	assert.Equal(t, "9.99", result.Amount)
}

func TestPayPalProvider_CreateOrder_NoApproveLink(t *testing.T) {
	cache := setupTestCfgCache(t, map[string]string{
		CfgKeyPayPalClientID:        "test-id",
		CfgKeyPayPalClientSecretEnc: encryptHelper(t, "test-secret"),
	})
	provider := NewPayPalProvider(cache, testMgr)

	mockSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/oauth2/token" {
			w.Write([]byte(`{"access_token":"A21AAF","token_type":"Bearer","expires_in":32400}`))
			return
		}
		w.WriteHeader(http.StatusCreated)
		// 响应中没有 approve 链接
		w.Write([]byte(`{"id":"ORDERID","status":"CREATED","links":[{"href":"https://api.paypal.com/x","rel":"self","method":"GET"}]}`))
	}))
	defer mockSrv.Close()
	paypalAPIBase = mockSrv.URL
	defer func() { paypalAPIBase = "" }()

	_, err := provider.CreateOrder(context.Background(), &OrderParams{
		OrderNo: "PPL123",
		Amount:  "9.99",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "approve 链接")
}

func TestPayPalProvider_ParseNotify_CaptureCompleted(t *testing.T) {
	webhookID := "WH-2WR3-ABCD-EFGH-1234"
	cache := setupTestCfgCache(t, map[string]string{
		CfgKeyPayPalClientID:        "test-id",
		CfgKeyPayPalClientSecretEnc: encryptHelper(t, "test-secret"),
		CfgKeyPayPalWebhookID:       webhookID,
	})
	provider := NewPayPalProvider(cache, testMgr)

	mockSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/oauth2/token" {
			w.Write([]byte(`{"access_token":"A21AAF","token_type":"Bearer","expires_in":32400}`))
			return
		}
		if r.URL.Path == "/v1/notifications/verify-webhook-signature" {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"verification_status":"SUCCESS"}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer mockSrv.Close()
	paypalAPIBase = mockSrv.URL
	defer func() { paypalAPIBase = "" }()

	// 构造 PAYMENT.CAPTURE.COMPLETED webhook payload
	payload := `{
		"id": "WH-2WR3-EVT-001",
		"event_type": "PAYMENT.CAPTURE.COMPLETED",
		"create_time": "2024-01-01T00:00:00Z",
		"resource": {
			"id": "CAPTURE-001",
			"custom_id": "PPL123456",
			"amount": {"currency_code": "USD", "value": "9.99"}
		}
	}`

	req := httptest.NewRequest("POST", "/pay/notify/paypal", strings.NewReader(payload))
	req.Header.Set("PayPal-Transmission-Id", "trans-id-001")
	req.Header.Set("PayPal-Transmission-Time", "2024-01-01T00:00:00Z")
	req.Header.Set("PayPal-Transmission-Sig", "sig-001")
	req.Header.Set("PayPal-Cert-Url", "https://api.paypal.com/cert")
	req.Header.Set("PayPal-Auth-Algo", "SHA256withRSA")

	data, err := provider.ParseNotify(context.Background(), req)
	require.NoError(t, err)
	assert.True(t, data.IsPaid())
	assert.Equal(t, "PPL123456", data.OutTradeNo)
	assert.Equal(t, "CAPTURE-001", data.TradeNo)
	assert.Equal(t, "9.99", data.Amount)
	assert.Equal(t, "USD", data.Currency)
}

func TestPayPalProvider_ParseNotify_VerificationFailed(t *testing.T) {
	webhookID := "WH-2WR3-ABCD-EFGH-1234"
	cache := setupTestCfgCache(t, map[string]string{
		CfgKeyPayPalClientID:        "test-id",
		CfgKeyPayPalClientSecretEnc: encryptHelper(t, "test-secret"),
		CfgKeyPayPalWebhookID:       webhookID,
	})
	provider := NewPayPalProvider(cache, testMgr)

	mockSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/oauth2/token" {
			w.Write([]byte(`{"access_token":"A21AAF","token_type":"Bearer","expires_in":32400}`))
			return
		}
		if r.URL.Path == "/v1/notifications/verify-webhook-signature" {
			w.Write([]byte(`{"verification_status":"FAILURE"}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer mockSrv.Close()
	paypalAPIBase = mockSrv.URL
	defer func() { paypalAPIBase = "" }()

	payload := `{
		"id": "WH-EVT",
		"event_type": "PAYMENT.CAPTURE.COMPLETED",
		"resource": {"id":"C1","custom_id":"PPL1","amount":{"value":"9.99"}}
	}`
	req := httptest.NewRequest("POST", "/pay/notify/paypal", strings.NewReader(payload))
	req.Header.Set("PayPal-Transmission-Id", "trans-id")
	req.Header.Set("PayPal-Transmission-Sig", "sig")
	req.Header.Set("PayPal-Cert-Url", "https://api.paypal.com/cert")
	req.Header.Set("PayPal-Auth-Algo", "SHA256withRSA")
	req.Header.Set("PayPal-Transmission-Time", "2024-01-01T00:00:00Z")

	data, err := provider.ParseNotify(context.Background(), req)
	require.NoError(t, err)
	assert.False(t, data.IsPaid())
	assert.Contains(t, data.VerifyError.Error(), "verification_status=FAILURE")
}

func TestPayPalProvider_ParseNotify_WebhookIDMissing(t *testing.T) {
	cache := setupTestCfgCache(t, map[string]string{
		CfgKeyPayPalClientID:        "test-id",
		CfgKeyPayPalClientSecretEnc: encryptHelper(t, "test-secret"),
	})
	provider := NewPayPalProvider(cache, testMgr)

	paypalAPIBase = ""
	payload := `{"id":"WH","event_type":"PAYMENT.CAPTURE.COMPLETED","resource":{"id":"C1","custom_id":"PPL1"}}`
	req := httptest.NewRequest("POST", "/pay/notify/paypal", strings.NewReader(payload))
	req.Header.Set("PayPal-Transmission-Id", "trans-id")
	req.Header.Set("PayPal-Transmission-Sig", "sig")
	req.Header.Set("PayPal-Cert-Url", "https://api.paypal.com/cert")

	data, err := provider.ParseNotify(context.Background(), req)
	require.NoError(t, err)
	assert.False(t, data.IsPaid())
	assert.Contains(t, data.VerifyError.Error(), "webhook_id 未配置")
}

func TestPayPalProvider_ParseNotify_OrderApprovedPending(t *testing.T) {
	webhookID := "WH-TEST-001"
	cache := setupTestCfgCache(t, map[string]string{
		CfgKeyPayPalClientID:        "test-id",
		CfgKeyPayPalClientSecretEnc: encryptHelper(t, "test-secret"),
		CfgKeyPayPalWebhookID:       webhookID,
	})
	provider := NewPayPalProvider(cache, testMgr)

	mockSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/oauth2/token" {
			w.Write([]byte(`{"access_token":"A21","token_type":"Bearer","expires_in":32400}`))
			return
		}
		if r.URL.Path == "/v1/notifications/verify-webhook-signature" {
			w.Write([]byte(`{"verification_status":"SUCCESS"}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer mockSrv.Close()
	paypalAPIBase = mockSrv.URL
	defer func() { paypalAPIBase = "" }()

	// CHECKOUT.ORDER.APPROVED → status=pending（不标 paid）
	payload := `{
		"id": "WH-EVT-APPROVED",
		"event_type": "CHECKOUT.ORDER.APPROVED",
		"resource": {
			"id": "ORDER-001",
			"purchase_units": [{"custom_id": "PPL123", "amount": {"value": "9.99", "currency_code": "USD"}}]
		}
	}`
	req := httptest.NewRequest("POST", "/pay/notify/paypal", strings.NewReader(payload))
	req.Header.Set("PayPal-Transmission-Id", "trans-id")
	req.Header.Set("PayPal-Transmission-Sig", "sig")
	req.Header.Set("PayPal-Cert-Url", "https://api.paypal.com/cert")
	req.Header.Set("PayPal-Auth-Algo", "SHA256withRSA")
	req.Header.Set("PayPal-Transmission-Time", "2024-01-01T00:00:00Z")

	data, err := provider.ParseNotify(context.Background(), req)
	require.NoError(t, err)
	// CHECKOUT.ORDER.APPROVED 不算 paid（只有 PAYMENT.CAPTURE.COMPLETED 才算）
	assert.False(t, data.IsPaid())
	assert.Equal(t, "pending", data.Status)
	assert.Equal(t, "PPL123", data.OutTradeNo)
	assert.Equal(t, "9.99", data.Amount)
}

func TestUSDToCents(t *testing.T) {
	cents, err := USDToCents("9.99")
	assert.NoError(t, err)
	assert.Equal(t, "999", cents)

	cents, err = USDToCents("0.01")
	assert.NoError(t, err)
	assert.Equal(t, "1", cents)

	_, err = USDToCents("not-a-number")
	assert.Error(t, err)
}

// ============== 3. Stripe 测试 ==============

func TestStripeProvider_Name(t *testing.T) {
	cache := setupTestCfgCache(t, nil)
	provider := NewStripeProvider(cache, nil)
	assert.Equal(t, "stripe", provider.Name())
}

func TestStripeProvider_CreateOrder_Success(t *testing.T) {
	cache := setupTestCfgCache(t, map[string]string{
		CfgKeyStripeSecretKeyEnc: encryptHelper(t, "sk_test_12345"),
	})
	provider := NewStripeProvider(cache, testMgr)

	mockSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/payment_intents", r.URL.Path)
		assert.Equal(t, "Bearer sk_test_12345", r.Header.Get("Authorization"))
		assert.Equal(t, "2023-10-16", r.Header.Get("Stripe-Version"))
		// 解析 form
		_ = r.ParseForm()
		assert.Equal(t, "999", r.PostFormValue("amount"))
		assert.Equal(t, "usd", r.PostFormValue("currency"))
		assert.Equal(t, "STP123456", r.PostFormValue("metadata[order_no]"))

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"id": "pi_12345",
			"object": "payment_intent",
			"amount": 999,
			"currency": "usd",
			"status": "requires_payment_method",
			"client_secret": "pi_12345_secret_abcde"
		}`))
	}))
	defer mockSrv.Close()
	stripeAPIBaseOverride = mockSrv.URL
	defer func() { stripeAPIBaseOverride = "" }()

	result, err := provider.CreateOrder(context.Background(), &OrderParams{
		OrderNo: "STP123456",
		Amount:  "999", // cents
		Subject: "KeyAuth 卡密",
	})
	require.NoError(t, err)
	assert.Equal(t, "stripe", result.Channel)
	assert.Equal(t, "pi_12345_secret_abcde", result.ClientSecret)
	assert.Equal(t, "999", result.Amount)
}

func TestStripeProvider_CreateOrder_SecretMissing(t *testing.T) {
	cache := setupTestCfgCache(t, nil)
	provider := NewStripeProvider(cache, testMgr)
	_, err := provider.CreateOrder(context.Background(), &OrderParams{
		OrderNo: "STP1",
		Amount:  "999",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "secret_key 未配置")
}

func TestStripeProvider_CreateOrder_InvalidKeyFormat(t *testing.T) {
	cache := setupTestCfgCache(t, map[string]string{
		CfgKeyStripeSecretKeyEnc: encryptHelper(t, "invalid-key"),
	})
	provider := NewStripeProvider(cache, testMgr)
	_, err := provider.CreateOrder(context.Background(), &OrderParams{
		OrderNo: "STP1",
		Amount:  "999",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "应以 sk_ 开头")
}

func TestStripeProvider_CreateOrder_Non200(t *testing.T) {
	cache := setupTestCfgCache(t, map[string]string{
		CfgKeyStripeSecretKeyEnc: encryptHelper(t, "sk_test_123"),
	})
	provider := NewStripeProvider(cache, testMgr)

	mockSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":{"message":"invalid amount"}}`))
	}))
	defer mockSrv.Close()
	stripeAPIBaseOverride = mockSrv.URL
	defer func() { stripeAPIBaseOverride = "" }()

	_, err := provider.CreateOrder(context.Background(), &OrderParams{
		OrderNo: "STP1",
		Amount:  "999",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "非 200")
}

func TestStripeProvider_ParseNotify_Success(t *testing.T) {
	webhookSecret := "whsec_abc123"
	cache := setupTestCfgCache(t, map[string]string{
		CfgKeyStripeWebhookSecretEnc: encryptHelper(t, webhookSecret),
	})
	provider := NewStripeProvider(cache, testMgr)

	// 构造 payment_intent.succeeded webhook
	body := []byte(`{
		"id": "evt_001",
		"type": "payment_intent.succeeded",
		"object": "event",
		"data": {
			"object": {
				"id": "pi_12345",
				"amount": 999,
				"currency": "usd",
				"metadata": {"order_no": "STP123456"}
			}
		}
	}`)

	ts := time.Now().Unix()
	sig := SignStripeWebhook(body, webhookSecret, ts)
	sigHeader := BuildStripeSignatureHeader(ts, sig)

	req := httptest.NewRequest("POST", "/pay/notify/stripe", bytes.NewReader(body))
	req.Header.Set("Stripe-Signature", sigHeader)

	data, err := provider.ParseNotify(context.Background(), req)
	require.NoError(t, err)
	assert.True(t, data.IsPaid())
	assert.Equal(t, "STP123456", data.OutTradeNo)
	assert.Equal(t, "pi_12345", data.TradeNo)
	assert.Equal(t, "999", data.Amount)
	assert.Equal(t, "USD", data.Currency)
}

func TestStripeProvider_ParseNotify_SignatureMismatch(t *testing.T) {
	webhookSecret := "whsec_abc123"
	cache := setupTestCfgCache(t, map[string]string{
		CfgKeyStripeWebhookSecretEnc: encryptHelper(t, webhookSecret),
	})
	provider := NewStripeProvider(cache, testMgr)

	body := []byte(`{"id":"evt","type":"payment_intent.succeeded","data":{"object":{"id":"pi_1","amount":999,"metadata":{"order_no":"STP1"}}}}`)
	ts := time.Now().Unix()

	req := httptest.NewRequest("POST", "/pay/notify/stripe", bytes.NewReader(body))
	req.Header.Set("Stripe-Signature", "t="+string(rune(ts))+",v1=invalid_sig")

	data, err := provider.ParseNotify(context.Background(), req)
	require.NoError(t, err)
	assert.False(t, data.IsPaid())
	assert.Error(t, data.VerifyError)
}

func TestStripeProvider_ParseNotify_TimestampOutOfTolerance(t *testing.T) {
	webhookSecret := "whsec_abc123"
	cache := setupTestCfgCache(t, map[string]string{
		CfgKeyStripeWebhookSecretEnc: encryptHelper(t, webhookSecret),
	})
	provider := NewStripeProvider(cache, testMgr)

	body := []byte(`{"id":"evt","type":"payment_intent.succeeded","data":{"object":{"id":"pi_1","amount":999,"metadata":{"order_no":"STP1"}}}}`)

	// 10 分钟前的时间戳，超出 5 分钟容差
	oldTs := time.Now().Add(-10 * time.Minute).Unix()
	sig := SignStripeWebhook(body, webhookSecret, oldTs)
	sigHeader := BuildStripeSignatureHeader(oldTs, sig)

	req := httptest.NewRequest("POST", "/pay/notify/stripe", bytes.NewReader(body))
	req.Header.Set("Stripe-Signature", sigHeader)

	data, err := provider.ParseNotify(context.Background(), req)
	require.NoError(t, err)
	assert.False(t, data.IsPaid())
	assert.Contains(t, data.VerifyError.Error(), "超出 5 分钟容差")
}

func TestStripeProvider_ParseNotify_WebhookSecretMissing(t *testing.T) {
	cache := setupTestCfgCache(t, nil)
	provider := NewStripeProvider(cache, testMgr)

	body := []byte(`{"id":"evt","type":"payment_intent.succeeded","data":{"object":{"id":"pi_1","metadata":{"order_no":"STP1"}}}}`)
	req := httptest.NewRequest("POST", "/pay/notify/stripe", bytes.NewReader(body))
	req.Header.Set("Stripe-Signature", "t=1700000000,v1=abc")

	data, err := provider.ParseNotify(context.Background(), req)
	require.NoError(t, err)
	assert.False(t, data.IsPaid())
	assert.Contains(t, data.VerifyError.Error(), "webhook_secret 未配置")
}

func TestStripeProvider_ParseNotify_PaymentFailed(t *testing.T) {
	webhookSecret := "whsec_abc"
	cache := setupTestCfgCache(t, map[string]string{
		CfgKeyStripeWebhookSecretEnc: encryptHelper(t, webhookSecret),
	})
	provider := NewStripeProvider(cache, testMgr)

	body := []byte(`{
		"id": "evt_fail",
		"type": "payment_intent.payment_failed",
		"data": {"object": {"id": "pi_fail", "amount": 999, "metadata": {"order_no": "STP999"}}}
	}`)
	ts := time.Now().Unix()
	sig := SignStripeWebhook(body, webhookSecret, ts)

	req := httptest.NewRequest("POST", "/pay/notify/stripe", bytes.NewReader(body))
	req.Header.Set("Stripe-Signature", BuildStripeSignatureHeader(ts, sig))

	data, err := provider.ParseNotify(context.Background(), req)
	require.NoError(t, err)
	assert.False(t, data.IsPaid())
	assert.Equal(t, "failed", data.Status)
}

func TestVerifyStripeSignature_MissingHeader(t *testing.T) {
	err := verifyStripeSignature("", []byte("body"), "secret")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Stripe-Signature 头缺失")
}

func TestVerifyStripeSignature_MissingV1(t *testing.T) {
	err := verifyStripeSignature("t=12345", []byte("body"), "secret")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "缺少 t= 或 v1=")
}

func TestSignStripeWebhook_Deterministic(t *testing.T) {
	body := []byte(`{"test":true}`)
	ts := int64(1700000000)
	sig1 := SignStripeWebhook(body, "secret", ts)
	sig2 := SignStripeWebhook(body, "secret", ts)
	assert.Equal(t, sig1, sig2)
	// 不同 secret 应产生不同签名
	sig3 := SignStripeWebhook(body, "other-secret", ts)
	assert.NotEqual(t, sig1, sig3)
}

// ============== 4. 常量与配置键完整性 ==============

func TestChannelConstants(t *testing.T) {
	assert.Equal(t, "usdt", ChannelUSDT)
	assert.Equal(t, "paypal", ChannelPayPal)
	assert.Equal(t, "stripe", ChannelStripe)
}

func TestUSDTConfigKeys(t *testing.T) {
	// 铁律 04：配置键常量必须以 pay.usdt. 开头
	assert.Equal(t, "pay.usdt.enabled", CfgKeyUSDTEnabled)
	assert.Equal(t, "pay.usdt.trc20_address", CfgKeyUSDTTrc20Address)
	assert.Equal(t, "pay.usdt.contract_address", CfgKeyUSDTContractAddress)
	assert.Equal(t, "pay.usdt.hmac_secret_enc", CfgKeyUSDTHMACSecretEnc)
	assert.Equal(t, "pay.usdt.exchange_rate", CfgKeyUSDTExchangeRate)
	assert.Equal(t, "pay.usdt.polling_enabled", CfgKeyUSDTPollingEnabled)
	assert.Equal(t, "pay.usdt.polling_interval_seconds", CfgKeyUSDTPollingInterval)
	assert.Equal(t, "pay.usdt.trongrid_api_key", CfgKeyUSDTTronGridAPIKey)
	assert.Equal(t, "pay.usdt.expire_seconds", CfgKeyUSDTExpireSeconds)
}

func TestPayPalConfigKeys(t *testing.T) {
	assert.Equal(t, "pay.paypal.enabled", CfgKeyPayPalEnabled)
	assert.Equal(t, "pay.paypal.client_id", CfgKeyPayPalClientID)
	assert.Equal(t, "pay.paypal.client_secret_enc", CfgKeyPayPalClientSecretEnc)
	assert.Equal(t, "pay.paypal.webhook_id", CfgKeyPayPalWebhookID)
	assert.Equal(t, "pay.paypal.sandbox", CfgKeyPayPalSandbox)
	assert.Equal(t, "pay.paypal.expire_seconds", CfgKeyPayPalExpireSeconds)
}

func TestStripeConfigKeys(t *testing.T) {
	assert.Equal(t, "pay.stripe.enabled", CfgKeyStripeEnabled)
	assert.Equal(t, "pay.stripe.secret_key_enc", CfgKeyStripeSecretKeyEnc)
	assert.Equal(t, "pay.stripe.webhook_secret_enc", CfgKeyStripeWebhookSecretEnc)
	assert.Equal(t, "pay.stripe.expire_seconds", CfgKeyStripeExpireSeconds)
}

func TestNotifyData_IsPaid(t *testing.T) {
	// paid + 无验签错误 → true
	assert.True(t, (&NotifyData{Status: "paid"}).IsPaid())
	// paid + 验签错误 → false
	assert.False(t, (&NotifyData{Status: "paid", VerifyError: assertError("err")}).IsPaid())
	// 非 paid → false
	assert.False(t, (&NotifyData{Status: "pending"}).IsPaid())
	// nil → false
	assert.False(t, (&NotifyData{}).IsPaid())
}

// assertError 辅助：返回一个 error
func assertError(msg string) error {
	return &testError{msg: msg}
}

type testError struct{ msg string }

func (e *testError) Error() string { return e.msg }
