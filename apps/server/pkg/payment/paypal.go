// PayPal Orders API v2 支付通道
//
// 设计要点（铁律 06：基于 PayPal 官方文档 https://developer.paypal.com/api/orders/v2/）：
//  1. OAuth2：POST /v1/oauth2/token with Basic Auth (client_id:client_secret)
//     scope=openid profile email address https://uri.paypal.com/services/payments/realtimeapi
//     返回 access_token（TTL ~32400s 即 9 小时）+ expires_in + token_type=Bearer
//  2. 创建订单：POST /v2/checkout/orders
//     intent=CAPTURE / purchase_units[].amount.{currency_code,value} / application_context.{return_url,cancel_url}
//     返回 id（订单 ID）+ status=CREATED + links[].href（含 approve 跳转 URL，rel=approve）
//  3. webhook 验签（PayPal-Transmission-* 头 + CERT URL）：
//     简化方案：调用 /v1/notifications/verify-webhook-signature API 验证
//     完整流程：拼装 auth_algo / cert_url / transmission_id / transmission_sig / transmission_time / webhook_id / body
//     POST 后返回 verification_status=SUCCESS
//  4. webhook 事件：CHECKOUT.ORDER.APPROVED → 用户已批准；PAYMENT.CAPTURE.COMPLETED → 资金已入账
//     仅当 PAYMENT.CAPTURE.COMPLETED 触发时标记订单为 paid
package payment

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/your-org/keyauth-saas/apps/server/internal/config"
	"github.com/your-org/keyauth-saas/apps/server/pkg/crypto"
)

// PayPal 配置键常量
const (
	CfgKeyPayPalEnabled        = "pay.paypal.enabled"          // bool
	CfgKeyPayPalClientID       = "pay.paypal.client_id"        // string
	CfgKeyPayPalClientSecretEnc = "pay.paypal.client_secret_enc" // string AES 加密
	CfgKeyPayPalWebhookID      = "pay.paypal.webhook_id"       // string WH-开头
	CfgKeyPayPalSandbox        = "pay.paypal.sandbox"          // bool 默认 true
	CfgKeyPayPalExpireSeconds  = "pay.paypal.expire_seconds"   // number 默认 1800
)

// PayPal API 基础地址（铁律 04：sandbox/live 由 sys_config 控制）
const (
	paypalSandboxAPI = "https://api-m.sandbox.paypal.com"
	paypalLiveAPI    = "https://api-m.paypal.com"
)

// paypalAPIBase 可在测试中覆盖
var paypalAPIBase = ""

// getPayPalAPIBase 根据 sandbox 配置返回 API 基础地址
func getPayPalAPIBase(ctx context.Context, cfg *config.ConfigCache) string {
	if paypalAPIBase != "" {
		return paypalAPIBase
	}
	if cfg.GetBool(ctx, CfgKeyPayPalSandbox, true) {
		return paypalSandboxAPI
	}
	return paypalLiveAPI
}

// PayPalProvider PayPal 支付通道
type PayPalProvider struct {
	cfg    *config.ConfigCache
	crypto *crypto.Manager

	// access_token 缓存（避免每次请求都换取新 token）
	mu          sync.Mutex
	accessToken string
	tokenExpiry time.Time
}

// NewPayPalProvider 构造
func NewPayPalProvider(cfg *config.ConfigCache, cryptoMgr *crypto.Manager) *PayPalProvider {
	return &PayPalProvider{cfg: cfg, crypto: cryptoMgr}
}

// Name 通道标识
func (p *PayPalProvider) Name() string { return ChannelPayPal }

// paypalTokenResponse OAuth2 token 响应
type paypalTokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"` // 秒
	Scope       string `json:"scope"`
}

// getAccessToken 获取 PayPal OAuth2 access_token（带缓存，提前 60s 过期）
func (p *PayPalProvider) getAccessToken(ctx context.Context) (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// 缓存命中（提前 60s 过期避免边界失效）
	if p.accessToken != "" && time.Now().Add(60*time.Second).Before(p.tokenExpiry) {
		return p.accessToken, nil
	}

	if p.cfg == nil {
		return "", errors.New("PayPal provider config cache 未初始化")
	}
	clientID := p.cfg.GetString(ctx, CfgKeyPayPalClientID, "")
	secretEnc := p.cfg.GetString(ctx, CfgKeyPayPalClientSecretEnc, "")
	if clientID == "" || secretEnc == "" {
		return "", errors.New("PayPal client_id 或 client_secret 未配置")
	}
	secret, err := p.crypto.DecryptAES(secretEnc)
	if err != nil {
		return "", fmt.Errorf("PayPal client_secret 解密失败: %w", err)
	}

	apiBase := getPayPalAPIBase(ctx, p.cfg)
	url := apiBase + "/v1/oauth2/token"

	body := "grant_type=client_credentials"
	req, err := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("构造 OAuth2 请求失败: %w", err)
	}
	req.SetBasicAuth(clientID, secret)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("OAuth2 请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("OAuth2 非 200 响应: %d %s", resp.StatusCode, string(respBody))
	}

	var tok paypalTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tok); err != nil {
		return "", fmt.Errorf("OAuth2 响应解析失败: %w", err)
	}

	p.accessToken = tok.AccessToken
	p.tokenExpiry = time.Now().Add(time.Duration(tok.ExpiresIn) * time.Second)
	return tok.AccessToken, nil
}

// paypalOrderRequest 创建订单请求体
type paypalOrderRequest struct {
	Intent       string                  `json:"intent"` // CAPTURE
	PurchaseUnits []paypalPurchaseUnit   `json:"purchase_units"`
	ApplicationContext *paypalAppContext `json:"application_context,omitempty"`
}

type paypalPurchaseUnit struct {
	ReferenceID string         `json:"reference_id,omitempty"` // 商户订单号
	Amount      paypalAmount   `json:"amount"`
}

type paypalAmount struct {
	CurrencyCode string `json:"currency_code"` // USD
	Value        string `json:"value"`         // 金额字符串（2 位小数）
}

type paypalAppContext struct {
	ReturnURL string `json:"return_url"`
	CancelURL string `json:"cancel_url"`
}

// paypalOrderResponse 创建订单响应
type paypalOrderResponse struct {
	ID     string         `json:"id"`     // PayPal 订单 ID
	Status string         `json:"status"` // CREATED / APPROVED / COMPLETED
	Links  []paypalLink   `json:"links"`
}

type paypalLink struct {
	HRef   string `json:"href"`
	Rel    string `json:"rel"`     // self / approve / capture / refund
	Method string `json:"method"`  // GET / POST / REDIRECT
}

// CreateOrder 创建 PayPal 订单
// 返回 PaymentURL（approve 链接，前端 location.href 跳转）
func (p *PayPalProvider) CreateOrder(ctx context.Context, params *OrderParams) (*OrderResult, error) {
	if p.cfg == nil {
		return nil, errors.New("PayPal provider config cache 未初始化")
	}
	if params.Amount == "" {
		return nil, errors.New("金额不能为空")
	}
	token, err := p.getAccessToken(ctx)
	if err != nil {
		return nil, err
	}

	apiBase := getPayPalAPIBase(ctx, p.cfg)
	url := apiBase + "/v2/checkout/orders"

	orderReq := paypalOrderRequest{
		Intent: "CAPTURE",
		PurchaseUnits: []paypalPurchaseUnit{
			{
				ReferenceID: params.OrderNo,
				Amount: paypalAmount{
					CurrencyCode: "USD",
					Value:        params.Amount,
				},
			},
		},
	}
	if params.ReturnURL != "" {
		orderReq.ApplicationContext = &paypalAppContext{
			ReturnURL: params.ReturnURL,
			CancelURL: params.ReturnURL, // 复用 return URL 作为取消 URL
		}
	}

	reqBody, _ := json.Marshal(orderReq)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("构造创建订单请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("PayPal-Request-Id", params.OrderNo) // 幂等键

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("创建订单请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("创建订单非 201 响应: %d %s", resp.StatusCode, string(respBody))
	}

	var orderResp paypalOrderResponse
	if err := json.NewDecoder(resp.Body).Decode(&orderResp); err != nil {
		return nil, fmt.Errorf("创建订单响应解析失败: %w", err)
	}

	// 找到 approve 链接
	approveURL := ""
	for _, link := range orderResp.Links {
		if link.Rel == "approve" {
			approveURL = link.HRef
			break
		}
	}
	if approveURL == "" {
		return nil, fmt.Errorf("PayPal 响应缺少 approve 链接: %+v", orderResp.Links)
	}

	expireSec := p.cfg.GetInt(ctx, CfgKeyPayPalExpireSeconds, 1800)

	return &OrderResult{
		Channel:       ChannelPayPal,
		PaymentURL:    approveURL,
		Amount:        params.Amount,
		ExpireSeconds: expireSec,
		RawResponse: map[string]interface{}{
			"order_id":    orderResp.ID,
			"status":      orderResp.Status,
			"approve_url": approveURL,
		},
	}, nil
}

// paypalWebhookPayload PayPal webhook 事件载荷
type paypalWebhookPayload struct {
	ID         string                 `json:"id"`          // 事件 ID（WH-2WR3-XXXX）
	EventType  string                 `json:"event_type"`  // PAYMENT.CAPTURE.COMPLETED / CHECKOUT.ORDER.APPROVED
	CreateTime string                 `json:"create_time"` // ISO8601
	Resource   map[string]interface{} `json:"resource"`    // 资源对象（订单或捕获）
}

// ParseNotify 解析 PayPal webhook
// 验签流程（简化版，铁律 06：基于官方 verify-webhook-signature API）：
//   1. 读取 PayPal-Transmission-* 头 + webhook_id + raw body
//   2. 调用 /v1/notifications/verify-webhook-signature API
//   3. verification_status=SUCCESS 表示通过
//   4. 仅 PAYMENT.CAPTURE.COMPLETED 事件标记订单为 paid
func (p *PayPalProvider) ParseNotify(ctx context.Context, r *http.Request) (*NotifyData, error) {
	if p.cfg == nil {
		return nil, errors.New("PayPal provider config cache 未初始化")
	}
	if r == nil || r.Body == nil {
		return nil, errors.New("请求体为空")
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, fmt.Errorf("读取请求体失败: %w", err)
	}

	// 还原 body 供后续解析
	r.Body = io.NopCloser(bytes.NewReader(body))

	var payload paypalWebhookPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("JSON 解析失败: %w", err)
	}

	// webhook_id 校验
	webhookID := p.cfg.GetString(ctx, CfgKeyPayPalWebhookID, "")
	outTradeNo := ""
	amount := ""
	tradeNo := ""
	status := "pending"

	// 从 resource 中提取订单号 + 金额 + 流水号
	// resource 结构因事件类型不同：
	//   PAYMENT.CAPTURE.COMPLETED: resource 是 capture 对象，含 custom_id（订单号）/ amount.value / id
	//   CHECKOUT.ORDER.APPROVED: resource 是 order 对象，含 purchase_units[0].custom_id / id
	if payload.Resource != nil {
		if v, ok := payload.Resource["custom_id"].(string); ok {
			outTradeNo = v
		}
		// purchase_units[0].custom_id 兜底
		if outTradeNo == "" {
			if units, ok := payload.Resource["purchase_units"].([]interface{}); ok && len(units) > 0 {
				if unit, ok := units[0].(map[string]interface{}); ok {
					if v, ok := unit["custom_id"].(string); ok {
						outTradeNo = v
					}
					if amt, ok := unit["amount"].(map[string]interface{}); ok {
						if v, ok := amt["value"].(string); ok {
							amount = v
						}
					}
				}
			}
		}
		// capture 对象 amount
		if amount == "" {
			if amt, ok := payload.Resource["amount"].(map[string]interface{}); ok {
				if v, ok := amt["value"].(string); ok {
					amount = v
				}
			}
		}
		if id, ok := payload.Resource["id"].(string); ok {
			tradeNo = id
		}
	}

	// 验签
	verifyErr := p.verifyWebhookSignature(ctx, r, body, webhookID)
	if verifyErr != nil {
		return &NotifyData{
			Channel:     ChannelPayPal,
			OutTradeNo:  outTradeNo,
			TradeNo:     tradeNo,
			Amount:      amount,
			Currency:    "USD",
			Status:      "failed",
			VerifyError: verifyErr,
			RawPayload:  payload.Resource,
		}, nil
	}

	// 仅 PAYMENT.CAPTURE.COMPLETED 标记为 paid
	if payload.EventType == "PAYMENT.CAPTURE.COMPLETED" {
		status = "paid"
	} else if payload.EventType == "CHECKOUT.ORDER.APPROVED" {
		status = "pending" // 用户已批准，等待 capture
	} else {
		status = "pending"
	}

	return &NotifyData{
		Channel:    ChannelPayPal,
		OutTradeNo: outTradeNo,
		TradeNo:    tradeNo,
		Amount:     amount,
		Currency:   "USD",
		Status:     status,
		RawPayload: map[string]interface{}{
			"event_id":   payload.ID,
			"event_type": payload.EventType,
			"create_time": payload.CreateTime,
		},
	}, nil
}

// verifyWebhookSignature 调用 PayPal verify-webhook-signature API 验签
// 详见：https://developer.paypal.com/api/rest/webhooks/
func (p *PayPalProvider) verifyWebhookSignature(ctx context.Context, r *http.Request, body []byte, webhookID string) error {
	if webhookID == "" {
		return errors.New("PayPal webhook_id 未配置")
	}
	if webhookID != "" && !strings.HasPrefix(webhookID, "WH-") {
		return fmt.Errorf("PayPal webhook_id 格式非法：%s（应以 WH- 开头）", webhookID)
	}

	token, err := p.getAccessToken(ctx)
	if err != nil {
		return err
	}

	// 拼装验签请求体
	// 铁律 06：字段名严格遵循 PayPal 官方文档
	transmissionID := r.Header.Get("PayPal-Transmission-Id")
	transmissionTime := r.Header.Get("PayPal-Transmission-Time")
	transmissionSig := r.Header.Get("PayPal-Transmission-Sig")
	certURL := r.Header.Get("PayPal-Cert-Url")
	authAlgo := r.Header.Get("PayPal-Auth-Algo")

	if transmissionID == "" || transmissionSig == "" || certURL == "" {
		return errors.New("PayPal webhook 头缺少必要字段：PayPal-Transmission-Id/Sig/Cert-Url")
	}

	verifyReq := map[string]interface{}{
		"transmission_id":   transmissionID,
		"transmission_time": transmissionTime,
		"transmission_sig":  transmissionSig,
		"cert_url":          certURL,
		"auth_algo":         authAlgo,
		"webhook_id":        webhookID,
		"webhook_event":     json.RawMessage(body),
	}
	reqBody, _ := json.Marshal(verifyReq)

	apiBase := getPayPalAPIBase(ctx, p.cfg)
	url := apiBase + "/v1/notifications/verify-webhook-signature"

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(reqBody))
	if err != nil {
		return fmt.Errorf("构造验签请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("验签请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("验签非 200 响应: %d %s", resp.StatusCode, string(respBody))
	}

	var verifyResp struct {
		VerificationStatus string `json:"verification_status"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&verifyResp); err != nil {
		return fmt.Errorf("验签响应解析失败: %w", err)
	}

	if verifyResp.VerificationStatus != "SUCCESS" {
		return fmt.Errorf("PayPal 验签失败：verification_status=%s", verifyResp.VerificationStatus)
	}
	return nil
}

// USDToCents 将美元金额字符串（如 "9.99"）转为 cents 整数字符串（如 "999"）
// 用于和订单金额校验时的归一化
func USDToCents(usdStr string) (string, error) {
	f, err := strconv.ParseFloat(usdStr, 64)
	if err != nil {
		return "", fmt.Errorf("非法 USD 金额: %s", usdStr)
	}
	cents := int64(f * 100 + 0.5) // 四舍五入
	return strconv.FormatInt(cents, 10), nil
}
