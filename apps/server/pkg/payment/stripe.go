// Stripe Payment Intents API 支付通道
//
// 设计要点（铁律 06：基于 Stripe 官方文档 https://stripe.com/docs/api/payment_intents）：
//  1. 创建 PaymentIntent：POST /v1/payment_intents
//     表单参数：amount=cents(整数) / currency=usd / metadata.order_no=商户订单号
//     鉴权：Authorization: Bearer sk_live_xxx 或 sk_test_xxx
//     返回 id（pi_xxx）+ client_secret（pi_xxx_secret_xxx）+ status=requires_payment_method
//  2. webhook 验签（Stripe-Signature 头）：
//     格式：t=时间戳,v1=HMAC-SHA256(signed_payload, webhook_secret)
//     signed_payload = "{t}.{raw_body}"
//     校验：用 webhook_secret 重新计算 HMAC-SHA256(t.body)，与 v1 比较（容差 5 分钟）
//  3. webhook 事件：
//     payment_intent.succeeded → 资金已入账，标记订单 paid
//     payment_intent.payment_failed → 失败
package payment

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/your-org/keyauth-saas/apps/server/internal/config"
	"github.com/your-org/keyauth-saas/apps/server/pkg/crypto"
)

// Stripe 配置键常量
const (
	CfgKeyStripeEnabled           = "pay.stripe.enabled"             // bool
	CfgKeyStripeSecretKeyEnc      = "pay.stripe.secret_key_enc"      // string AES 加密的 sk_live_xxx
	CfgKeyStripeWebhookSecretEnc  = "pay.stripe.webhook_secret_enc"  // string AES 加密的 whsec_xxx
	CfgKeyStripeExpireSeconds     = "pay.stripe.expire_seconds"      // number 默认 1800
)

// Stripe API 基础地址
const stripeAPIBase = "https://api.stripe.com"

// stripeAPIBaseOverride 可在测试中覆盖
var stripeAPIBaseOverride = ""

// getStripeAPIBase 返回 Stripe API 基础地址
func getStripeAPIBase() string {
	if stripeAPIBaseOverride != "" {
		return stripeAPIBaseOverride
	}
	return stripeAPIBase
}

// StripeProvider Stripe 支付通道
type StripeProvider struct {
	cfg    *config.ConfigCache
	crypto *crypto.Manager
}

// NewStripeProvider 构造
func NewStripeProvider(cfg *config.ConfigCache, cryptoMgr *crypto.Manager) *StripeProvider {
	return &StripeProvider{cfg: cfg, crypto: cryptoMgr}
}

// Name 通道标识
func (p *StripeProvider) Name() string { return ChannelStripe }

// stripePaymentIntentResponse PaymentIntent 创建响应
type stripePaymentIntentResponse struct {
	ID           string `json:"id"`            // pi_xxx
	Object       string `json:"object"`        // payment_intent
	Amount       int64  `json:"amount"`        // cents
	Currency     string `json:"currency"`      // usd
	Status       string `json:"status"`        // requires_payment_method / succeeded / canceled
	ClientSecret string `json:"client_secret"` // pi_xxx_secret_xxx
}

// CreateOrder 创建 Stripe PaymentIntent
// 返回 ClientSecret 供前端 Stripe.js 调用 confirmCardPayment
func (p *StripeProvider) CreateOrder(ctx context.Context, params *OrderParams) (*OrderResult, error) {
	if p.cfg == nil {
		return nil, errors.New("Stripe provider config cache 未初始化")
	}
	if params.Amount == "" {
		return nil, errors.New("金额不能为空")
	}
	secretKeyEnc := p.cfg.GetString(ctx, CfgKeyStripeSecretKeyEnc, "")
	if secretKeyEnc == "" {
		return nil, errors.New("Stripe secret_key 未配置")
	}
	secretKey, err := p.crypto.DecryptAES(secretKeyEnc)
	if err != nil {
		return nil, fmt.Errorf("Stripe secret_key 解密失败: %w", err)
	}
	if !strings.HasPrefix(secretKey, "sk_") {
		return nil, fmt.Errorf("Stripe secret_key 格式非法（应以 sk_ 开头）")
	}

	// 金额转 cents（params.Amount 已是 cents 整数字符串）
	amountCents, err := strconv.ParseInt(params.Amount, 10, 64)
	if err != nil || amountCents <= 0 {
		return nil, fmt.Errorf("非法金额（应为 cents 整数）：%s", params.Amount)
	}

	// POST https://api.stripe.com/v1/payment_intents
	// Content-Type: application/x-www-form-urlencoded
	form := url.Values{}
	form.Set("amount", strconv.FormatInt(amountCents, 10))
	form.Set("currency", "usd")
	form.Set("metadata[order_no]", params.OrderNo)
	form.Set("description", params.Subject)
	// 自动确认模式：前端用 Stripe.js 触发 confirmCardPayment
	form.Set("automatic_payment_methods[enabled]", "true")

	urlStr := getStripeAPIBase() + "/v1/payment_intents"
	req, err := http.NewRequestWithContext(ctx, "POST", urlStr, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("构造 PaymentIntent 请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Authorization", "Bearer "+secretKey)
	req.Header.Set("Stripe-Version", "2023-10-16") // 铁律 06：固定 API 版本

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("PaymentIntent 请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("PaymentIntent 非 200 响应: %d %s", resp.StatusCode, string(respBody))
	}

	var piResp stripePaymentIntentResponse
	if err := json.NewDecoder(resp.Body).Decode(&piResp); err != nil {
		return nil, fmt.Errorf("PaymentIntent 响应解析失败: %w", err)
	}

	if piResp.ClientSecret == "" {
		return nil, fmt.Errorf("Stripe 响应缺少 client_secret: %+v", piResp)
	}

	expireSec := p.cfg.GetInt(ctx, CfgKeyStripeExpireSeconds, 1800)

	return &OrderResult{
		Channel:       ChannelStripe,
		ClientSecret:  piResp.ClientSecret,
		Amount:        strconv.FormatInt(piResp.Amount, 10), // cents
		ExpireSeconds: expireSec,
		RawResponse: map[string]interface{}{
			"payment_intent_id": piResp.ID,
			"status":            piResp.Status,
			"currency":          piResp.Currency,
		},
	}, nil
}

// stripeWebhookPayload Stripe webhook 事件载荷
// 文档：https://stripe.com/docs/api/events/object
type stripeWebhookPayload struct {
	ID      string                 `json:"id"`       // evt_xxx
	Type    string                 `json:"type"`     // payment_intent.succeeded / payment_intent.payment_failed
	Object  string                 `json:"object"`   // event
	Data    struct {
		Object map[string]interface{} `json:"object"` // PaymentIntent 对象
	} `json:"data"`
}

// ParseNotify 解析 Stripe webhook
// 验签流程（铁律 06：基于官方 https://stripe.com/docs/webhooks/signatures）：
//   1. 提取 Stripe-Signature 头，格式："t=1234567890,v1=abcd..."
//   2. 用 webhook_secret 计算 HMAC-SHA256(t.body)，与 v1 比较
//   3. 时间戳容差 5 分钟（防重放）
//   4. 仅 payment_intent.succeeded 标记为 paid
func (p *StripeProvider) ParseNotify(ctx context.Context, r *http.Request) (*NotifyData, error) {
	if p.cfg == nil {
		return nil, errors.New("Stripe provider config cache 未初始化")
	}
	if r == nil || r.Body == nil {
		return nil, errors.New("请求体为空")
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, fmt.Errorf("读取请求体失败: %w", err)
	}

	// 验签
	webhookSecretEnc := p.cfg.GetString(ctx, CfgKeyStripeWebhookSecretEnc, "")
	outTradeNo := ""
	amount := ""
	tradeNo := ""
	status := "pending"

	// 从 body 解析事件
	var payload stripeWebhookPayload
	_ = json.Unmarshal(body, &payload) // 即使验签失败也尝试解析，用于错误响应中包含订单号

	// 从 data.object 提取订单号 + 金额 + 流水号
	if pi := payload.Data.Object; pi != nil {
		if id, ok := pi["id"].(string); ok {
			tradeNo = id
		}
		if amt, ok := pi["amount"].(float64); ok {
			amount = strconv.FormatInt(int64(amt), 10)
		}
		if meta, ok := pi["metadata"].(map[string]interface{}); ok {
			if v, ok := meta["order_no"].(string); ok {
				outTradeNo = v
			}
		}
	}

	if webhookSecretEnc == "" {
		return &NotifyData{
			Channel:     ChannelStripe,
			OutTradeNo:  outTradeNo,
			TradeNo:     tradeNo,
			Amount:      amount,
			Currency:    "USD",
			Status:      "failed",
			VerifyError: errors.New("Stripe webhook_secret 未配置"),
		}, nil
	}
	webhookSecret, err := p.crypto.DecryptAES(webhookSecretEnc)
	if err != nil {
		return &NotifyData{
			Channel:     ChannelStripe,
			OutTradeNo:  outTradeNo,
			TradeNo:     tradeNo,
			Amount:      amount,
			Currency:    "USD",
			Status:      "failed",
			VerifyError: fmt.Errorf("Stripe webhook_secret 解密失败: %w", err),
		}, nil
	}

	verifyErr := verifyStripeSignature(r.Header.Get("Stripe-Signature"), body, webhookSecret)
	if verifyErr != nil {
		return &NotifyData{
			Channel:     ChannelStripe,
			OutTradeNo:  outTradeNo,
			TradeNo:     tradeNo,
			Amount:      amount,
			Currency:    "USD",
			Status:      "failed",
			VerifyError: verifyErr,
		}, nil
	}

	// 仅 payment_intent.succeeded 标记为 paid
	if payload.Type == "payment_intent.succeeded" {
		status = "paid"
	} else if payload.Type == "payment_intent.payment_failed" {
		status = "failed"
	} else {
		status = "pending"
	}

	return &NotifyData{
		Channel:    ChannelStripe,
		OutTradeNo: outTradeNo,
		TradeNo:    tradeNo,
		Amount:     amount,
		Currency:   "USD",
		Status:     status,
		RawPayload: map[string]interface{}{
			"event_id":   payload.ID,
			"event_type": payload.Type,
		},
	}, nil
}

// verifyStripeSignature 验证 Stripe webhook 签名
// 算法（铁律 06：严格遵循官方文档）：
//   1. 解析 Stripe-Signature 头，提取 t= 和 v1=
//   2. signed_payload = "{t}.{body}"
//   3. expected_sig = HMAC-SHA256(signed_payload, webhook_secret) → hex
//   4. 常量时间比较 expected_sig 与 v1
//   5. 时间戳容差 5 分钟（防重放）
func verifyStripeSignature(sigHeader string, body []byte, webhookSecret string) error {
	if sigHeader == "" {
		return errors.New("Stripe-Signature 头缺失")
	}
	if webhookSecret == "" {
		return errors.New("webhook_secret 为空")
	}

	// 解析头：t=xxx,v1=xxx
	var timestampStr, sigV1 string
	for _, part := range strings.Split(sigHeader, ",") {
		part = strings.TrimSpace(part)
		if strings.HasPrefix(part, "t=") {
			timestampStr = strings.TrimPrefix(part, "t=")
		} else if strings.HasPrefix(part, "v1=") {
			sigV1 = strings.TrimPrefix(part, "v1=")
		}
	}
	if timestampStr == "" || sigV1 == "" {
		return errors.New("Stripe-Signature 缺少 t= 或 v1=")
	}

	timestamp, err := strconv.ParseInt(timestampStr, 10, 64)
	if err != nil {
		return fmt.Errorf("非法时间戳: %s", timestampStr)
	}

	// 时间容差 5 分钟（防重放）
	if time.Now().Unix()-timestamp > 300 {
		return fmt.Errorf("Stripe webhook 时间戳超出 5 分钟容差: %d", timestamp)
	}
	// 同时允许未来时间不超过 5 分钟（防时钟漂移）
	if timestamp-time.Now().Unix() > 300 {
		return fmt.Errorf("Stripe webhook 时间戳超出 5 分钟容差（未来时间）: %d", timestamp)
	}

	// 计算 expected signature
	signedPayload := timestampStr + "." + string(body)
	expectedSig := crypto.HMACSHA256Hex(webhookSecret, signedPayload)

	if !crypto.ConstantTimeEqualString(expectedSig, sigV1) {
		return errors.New("Stripe webhook 签名校验失败")
	}
	return nil
}

// SignStripeWebhook 工具函数：为 webhook payload 生成签名（供测试和文档参考）
func SignStripeWebhook(body []byte, webhookSecret string, timestamp int64) string {
	signedPayload := strconv.FormatInt(timestamp, 10) + "." + string(body)
	return crypto.HMACSHA256Hex(webhookSecret, signedPayload)
}

// BuildStripeSignatureHeader 拼装 Stripe-Signature 头
func BuildStripeSignatureHeader(timestamp int64, signature string) string {
	return fmt.Sprintf("t=%d,v1=%s", timestamp, signature)
}
