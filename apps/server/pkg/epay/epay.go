// Package epay 彩虹易支付协议封装
// 文档参考：https://epay.com/docs
// 协议要点：
//  1. 签名算法固定 MD5，参与签名的参数按 ASCII 升序排序，末尾追加商户密钥
//  2. 下单：GET submit.php（页面跳转）或 POST mapi.php（API 模式）
//  3. 异步回调：POST/GET 均可能，参数包含 sign 与 sign_type，trade_status=TRADE_SUCCESS 表示成功
//  4. 同步跳转：用户浏览器 302 跳转，参数同异步回调
//  5. 商户响应异步回调需返回字符串 "success"（小写），否则支付平台会重试
package epay

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/your-org/keyauth-saas/apps/server/pkg/crypto"
)

// Config 彩虹易支付配置
type Config struct {
	GatewayURL string // 易支付网关地址（如 https://pay.example.com）
	PID        string // 商户 PID
	Secret     string // 商户密钥（明文，已通过 AES 解密）
	SignType   string // 签名类型，固定 MD5
}

// OrderParams 下单参数
type OrderParams struct {
	OutTradeNo  string // 商户订单号
	Name        string // 商品名称
	Money       string // 金额（元，保留 2 位小数字符串）
	PayType     string // alipay/wxpay/qqpay
	NotifyURL   string // 异步通知地址
	ReturnURL   string // 同步跳转地址
	ClientIP    string // 客户端 IP
}

// BuildSubmitURL 构造 GET 跳转 URL（submit.php）
// 返回完整的支付页面地址，前端直接 location.href 即可
func BuildSubmitURL(cfg *Config, p *OrderParams) (string, error) {
	if cfg == nil || cfg.GatewayURL == "" || cfg.PID == "" || cfg.Secret == "" {
		return "", fmt.Errorf("易支付配置不完整")
	}
	if p == nil || p.OutTradeNo == "" || p.Money == "" {
		return "", fmt.Errorf("订单参数不完整")
	}

	params := map[string]string{
		"pid":          cfg.PID,
		"type":         p.PayType,
		"out_trade_no": p.OutTradeNo,
		"notify_url":   p.NotifyURL,
		"return_url":   p.ReturnURL,
		"name":         p.Name,
		"money":        p.Money,
	}
	if p.ClientIP != "" {
		params["clientip"] = p.ClientIP
	}

	sign := crypto.SignEpayParams(params, cfg.Secret)
	params["sign"] = sign
	params["sign_type"] = defaultIfEmpty(cfg.SignType, "MD5")

	// 拼接 URL
	q := url.Values{}
	for k, v := range params {
		q.Set(k, v)
	}

	gateway := strings.TrimRight(cfg.GatewayURL, "/")
	return gateway + "/submit.php?" + q.Encode(), nil
}

// NotifyParams 异步/同步回调参数
type NotifyParams struct {
	PID         string
	TradeNo     string // 易支付流水号
	OutTradeNo  string // 商户订单号
	Type        string // 支付方式
	Name        string
	Money       string
	TradeStatus string // TRADE_SUCCESS 表示成功
	Sign        string
	SignType    string
}

// ParseNotify 从 map[string]string 解析回调参数
func ParseNotify(params map[string]string) *NotifyParams {
	return &NotifyParams{
		PID:         params["pid"],
		TradeNo:     params["trade_no"],
		OutTradeNo:  params["out_trade_no"],
		Type:        params["type"],
		Name:        params["name"],
		Money:       params["money"],
		TradeStatus: params["trade_status"],
		Sign:        params["sign"],
		SignType:    params["sign_type"],
	}
}

// VerifyNotify 验证回调签名
// params 是原始回调参数 map（包含 sign/sign_type），secret 为商户密钥明文
func VerifyNotify(params map[string]string, secret string) bool {
	if secret == "" {
		return false
	}
	sign, ok := params["sign"]
	if !ok || sign == "" {
		return false
	}
	return crypto.VerifyEpaySign(params, secret, sign)
}

// IsSuccess 判断回调是否支付成功
func (n *NotifyParams) IsSuccess() bool {
	return n.TradeStatus == "TRADE_SUCCESS"
}

// defaultIfEmpty 简单辅助
func defaultIfEmpty(s, def string) string {
	if s == "" {
		return def
	}
	return s
}
