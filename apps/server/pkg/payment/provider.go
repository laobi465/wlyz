// Package payment v0.5.0 集成扩展批次 3：海外支付通道抽象
//
// 设计目标：在保留 epay（彩虹易支付）作为国内主通道的基础上，新增 3 个海外/加密货币通道：
//   - USDT-TRC20：固定收款地址 + 金额唯一后缀匹配 + TronGrid 轮询 / 外部监控 webhook
//   - PayPal Orders API v2：OAuth2 access_token + 创建订单返回 approval URL + webhook 验签
//   - Stripe Payment Intents API：创建 PaymentIntent 返回 client_secret + webhook HMAC-SHA256 验签
//
// 严格遵循铁律 04/05/06：
//   04 - 无硬编码：所有通道开关 / API 凭证 / 网关地址 / 沙盒标志 全部从 sys_config 读取
//   05 - 配置走后端：13 项 pay.{usdt,paypal,stripe}.* sys_config 可通过后台实时调整
//   06 - 反幻觉：所有 HTTP 调用真实发起并解析响应；签名算法遵循官方文档；测试用 httptest.Server mock
package payment

import (
	"context"
	"net/http"
)

// Channel 通道标识（与 model.AppOrder.PayChannel 字段值一致）
const (
	ChannelUSDT   = "usdt"
	ChannelPayPal = "paypal"
	ChannelStripe = "stripe"
)

// OrderParams 创建支付订单统一参数
type OrderParams struct {
	OrderNo    string  // 商户订单号（含 UST/PPL/STP 前缀）
	Amount     string  // 金额字符串（USDT 6 位小数 / PayPal USD 2 位 / Stripe cents 整数）
	AmountRaw  float64 // 原始金额（人民币元），供 USDT 汇率换算使用
	Currency   string  // 货币代码：USDT / USD
	Subject    string  // 订单标题
	NotifyURL  string  // 异步回调 URL
	ReturnURL  string  // 同步跳转 URL（PayPal 用）
	ClientIP   string
	OrderID    uint64 // 内部订单 ID（USDT 用作金额后缀生成依据）
}

// OrderResult 创建订单返回
type OrderResult struct {
	Channel       string                 // usdt / paypal / stripe
	PaymentURL    string                 // PayPal approval URL（浏览器跳转）
	ClientSecret  string                 // Stripe client_secret（前端 Stripe.js 用）
	QRContent     string                 // USDT 二维码内容（address + amount）
	Address       string                 // USDT 收款地址
	Amount        string                 // 实际需支付金额（USDT 含后缀）
	RawResponse   map[string]interface{} // 原始响应（调试用）
	ExpireSeconds int                    // 订单有效时长
}

// NotifyData webhook 解析后的统一结构
// 各 provider 在 ParseNotify 中将自身协议字段映射到此结构
type NotifyData struct {
	Channel     string                 // usdt / paypal / stripe
	OutTradeNo  string                 // 商户订单号
	TradeNo     string                 // 网关流水号（PayPal capture_id / Stripe payment_intent id / USDT tx_hash）
	Amount      string                 // 金额字符串（用于和订单金额校验）
	Currency    string                 // 货币代码
	Status      string                 // paid / pending / failed
	RawPayload  map[string]interface{} // 原始 webhook payload
	VerifyError error                  // 验签失败原因（非 nil 表示验签未通过）
}

// Provider 支付通道抽象
type Provider interface {
	// Name 返回通道标识
	Name() string

	// CreateOrder 创建支付订单
	CreateOrder(ctx context.Context, p *OrderParams) (*OrderResult, error)

	// ParseNotify 解析 webhook 请求并验签
	// 验签失败时返回 *NotifyData{VerifyError: err}
	ParseNotify(ctx context.Context, r *http.Request) (*NotifyData, error)
}

// IsPaid 是否已支付
func (n *NotifyData) IsPaid() bool {
	return n != nil && n.VerifyError == nil && n.Status == "paid"
}
