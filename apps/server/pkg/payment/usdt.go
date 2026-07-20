// USDT-TRC20 支付通道
//
// 设计要点（铁律 06：基于 TronGrid API 真实协议，不编造字段）：
//  1. 固定收款地址：管理员在 sys_config 配置一个 TRON 地址（T 开头 34 位）
//  2. 金额唯一后缀：每笔订单的实际金额 = base_amount + (order_id % 100) / 100
//     例：订单 #1234 基础金额 9.99 → 实付 9.99 + 0.34 = 10.33 USDT
//     使每笔订单金额在 1 USDT 范围内唯一，便于从区块链交易中匹配
//  3. webhook 两种来源：
//     - 外部监控服务（推荐）：第三方监听区块链后回调 /pay/notify/usdt，HMAC-SHA256 签名
//     - 内置 TronGrid 轮询 worker（可选，pay.usdt.polling_enabled=1 时启用）
//  4. TronGrid API：GET https://api.trongrid.io/v1/accounts/{address}/transactions/trc20
//     参数：contract_address=TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t（USDT 主网合约）
//           limit=50 / only_to=true / min_timestamp=毫秒
package payment

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/your-org/keyauth-saas/apps/server/internal/config"
	"github.com/your-org/keyauth-saas/apps/server/pkg/crypto"
)

// USDT 配置键常量（铁律 04：禁止硬编码配置键名）
const (
	CfgKeyUSDTEnabled          = "pay.usdt.enabled"             // bool
	CfgKeyUSDTTrc20Address     = "pay.usdt.trc20_address"       // string T 开头 34 位
	CfgKeyUSDTContractAddress  = "pay.usdt.contract_address"    // string 默认主网 USDT 合约
	CfgKeyUSDTHMACSecretEnc    = "pay.usdt.hmac_secret_enc"     // string AES 加密的 webhook 签名密钥
	CfgKeyUSDTExchangeRate     = "pay.usdt.exchange_rate"       // number CNY→USDT 汇率（1 CNY = X USDT）
	CfgKeyUSDTPollingEnabled   = "pay.usdt.polling_enabled"     // bool
	CfgKeyUSDTPollingInterval  = "pay.usdt.polling_interval_seconds" // number 默认 60
	CfgKeyUSDTTronGridAPIKey   = "pay.usdt.trongrid_api_key"    // string 可选
	CfgKeyUSDTExpireSeconds    = "pay.usdt.expire_seconds"      // number 默认 1800
)

// USDT-TRC20 主网合约地址（铁律 04：作为默认值参数，可通过 sys_config 覆盖）
const defaultUSDTContract = "TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t"

// trongridAPIBase 可在测试中覆盖
var trongridAPIBase = "https://api.trongrid.io"

// USDTProvider USDT-TRC20 支付通道
type USDTProvider struct {
	cfg    *config.ConfigCache
	crypto *crypto.Manager
}

// NewUSDTProvider 构造
func NewUSDTProvider(cfg *config.ConfigCache, cryptoMgr *crypto.Manager) *USDTProvider {
	return &USDTProvider{cfg: cfg, crypto: cryptoMgr}
}

// Name 通道标识
func (p *USDTProvider) Name() string { return ChannelUSDT }

// CreateOrder 创建 USDT 支付订单
// 返回 QRContent（地址 + 金额）供前端生成二维码
func (p *USDTProvider) CreateOrder(ctx context.Context, params *OrderParams) (*OrderResult, error) {
	if p.cfg == nil {
		return nil, errors.New("USDT provider config cache 未初始化")
	}
	address := p.cfg.GetString(ctx, CfgKeyUSDTTrc20Address, "")
	if address == "" {
		return nil, errors.New("USDT 收款地址未配置")
	}
	if !strings.HasPrefix(address, "T") || len(address) != 34 {
		return nil, fmt.Errorf("USDT 收款地址格式非法：%s（应为 T 开头 34 位）", address)
	}

	// 汇率换算：CNY → USDT
	rate := p.cfg.GetFloat64(ctx, CfgKeyUSDTExchangeRate, 0.14) // 默认 1 CNY ≈ 0.14 USDT
	if rate <= 0 {
		return nil, errors.New("USDT 汇率配置非法")
	}

	// 金额后缀：order_id % 100 / 100（0.00 ~ 0.99）
	// 注意：baseAmountUSDT 已包含两位小数，后缀额外加在第三位小数之后避免覆盖
	// 最终金额格式：XX.XX + 0.YY = XX.YY + 0.0Y → 实际为 baseUSDT + (orderID%100)/100
	baseUSDT := params.AmountRaw * rate
	suffix := float64(params.OrderID%100) / 100.0
	actualUSDT := baseUSDT + suffix
	// USDT 精度 6 位小数
	amountStr := strconv.FormatFloat(actualUSDT, 'f', 6, 64)

	expireSec := p.cfg.GetInt(ctx, CfgKeyUSDTExpireSeconds, 1800)

	// QRContent 遵循 BIP21 风格：usdt://{address}?amount={amount}&contract=...
	contract := p.cfg.GetString(ctx, CfgKeyUSDTContractAddress, defaultUSDTContract)
	qrContent := fmt.Sprintf("usdt://%s?amount=%s&contract=%s", address, amountStr, contract)

	return &OrderResult{
		Channel:       ChannelUSDT,
		QRContent:     qrContent,
		Address:       address,
		Amount:        amountStr,
		ExpireSeconds: expireSec,
		RawResponse: map[string]interface{}{
			"base_usdt":      strconv.FormatFloat(baseUSDT, 'f', 6, 64),
			"suffix":         suffix,
			"exchange_rate":  rate,
			"contract":       contract,
			"original_cny":   params.AmountRaw,
		},
	}, nil
}

// usdtNotifyPayload 外部监控 webhook 请求体
type usdtNotifyPayload struct {
	TxHash    string `json:"tx_hash"`     // 链上交易哈希
	From      string `json:"from"`        // 付款地址
	To        string `json:"to"`          // 收款地址
	Amount    string `json:"amount"`      // 金额（6 位小数 USDT）
	OutTradeNo string `json:"out_trade_no"` // 商户订单号
	Timestamp int64  `json:"timestamp"`   // 交易时间戳（秒）
	Signature string `json:"signature"`   // HMAC-SHA256(out_trade_no + amount + timestamp, secret)
}

// ParseNotify 解析外部监控 webhook
// 验签：HMAC-SHA256(out_trade_no + amount + timestamp, hmac_secret)
func (p *USDTProvider) ParseNotify(ctx context.Context, r *http.Request) (*NotifyData, error) {
	if p.cfg == nil {
		return nil, errors.New("USDT provider config cache 未初始化")
	}
	if r == nil || r.Body == nil {
		return nil, errors.New("请求体为空")
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, fmt.Errorf("读取请求体失败: %w", err)
	}

	var payload usdtNotifyPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("JSON 解析失败: %w", err)
	}
	if payload.OutTradeNo == "" || payload.Amount == "" || payload.TxHash == "" {
		return nil, errors.New("缺少必填字段：out_trade_no/amount/tx_hash")
	}

	// 验签
	secretEnc := p.cfg.GetString(ctx, CfgKeyUSDTHMACSecretEnc, "")
	if secretEnc == "" {
		return &NotifyData{
			Channel:     ChannelUSDT,
			OutTradeNo:  payload.OutTradeNo,
			TradeNo:     payload.TxHash,
			Amount:      payload.Amount,
			Currency:    "USDT",
			Status:      "failed",
			VerifyError: errors.New("HMAC 密钥未配置"),
		}, nil
	}
	secret, err := p.crypto.DecryptAES(secretEnc)
	if err != nil {
		return &NotifyData{
			Channel:     ChannelUSDT,
			OutTradeNo:  payload.OutTradeNo,
			TradeNo:     payload.TxHash,
			Amount:      payload.Amount,
			Currency:    "USDT",
			Status:      "failed",
			VerifyError: fmt.Errorf("HMAC 密钥解密失败: %w", err),
		}, nil
	}
	expectedSig := crypto.HMACSHA256Hex(
		payload.OutTradeNo+payload.Amount+strconv.FormatInt(payload.Timestamp, 10),
		secret,
	)
	if !crypto.ConstantTimeEqualString(expectedSig, payload.Signature) {
		return &NotifyData{
			Channel:     ChannelUSDT,
			OutTradeNo:  payload.OutTradeNo,
			TradeNo:     payload.TxHash,
			Amount:      payload.Amount,
			Currency:    "USDT",
			Status:      "failed",
			VerifyError: errors.New("签名校验失败"),
		}, nil
	}

	return &NotifyData{
		Channel:    ChannelUSDT,
		OutTradeNo: payload.OutTradeNo,
		TradeNo:    payload.TxHash,
		Amount:     payload.Amount,
		Currency:   "USDT",
		Status:     "paid",
		RawPayload: map[string]interface{}{
			"tx_hash":   payload.TxHash,
			"from":      payload.From,
			"to":        payload.To,
			"timestamp": payload.Timestamp,
		},
	}, nil
}

// MatchOrderAmount 校验回调金额是否与订单预期金额匹配
// baseUSDT：订单基础 USDT 金额；orderID：订单 ID；actualAmount：链上实付金额字符串
func MatchOrderAmount(baseUSDT float64, orderID uint64, actualAmount string) error {
	suffix := float64(orderID%100) / 100.0
	expected := baseUSDT + suffix
	expectedStr := strconv.FormatFloat(expected, 'f', 6, 64)
	if actualAmount != expectedStr {
		return fmt.Errorf("金额不匹配：期望 %s，实际 %s", expectedStr, actualAmount)
	}
	return nil
}

// trongridTxResponse TronGrid API 响应结构（仅取必要字段，铁律 06：基于真实 API 文档）
type trongridTxResponse struct {
	Data []struct {
		TransactionID string `json:"transaction_id"`
		BlockTimestamp int64 `json:"block_timestamp"` // 毫秒
		Value         string `json:"value"`           // 6 位小数字符串（USDT 内部存储乘以 10^6）
		From          string `json:"from"`
		To            string `json:"to"`
		TokenInfo     struct {
			Symbol   string `json:"symbol"`
			Address  string `json:"address"`
			Decimals int    `json:"decimals"`
		} `json:"token_info"`
	} `json:"data"`
	Success bool `json:"success"`
}

// PollTronGrid 拉取最近入账的 USDT-TRC20 交易
// 返回的 Amount 已转换为 6 位小数字符串（USDT 单位）
// fromTimestamp：起始时间戳（毫秒），仅返回此时间之后的交易
func (p *USDTProvider) PollTronGrid(ctx context.Context, fromTimestamp int64) ([]TronGridTx, error) {
	if p.cfg == nil {
		return nil, errors.New("USDT provider config cache 未初始化")
	}
	address := p.cfg.GetString(ctx, CfgKeyUSDTTrc20Address, "")
	if address == "" {
		return nil, errors.New("USDT 收款地址未配置")
	}
	contract := p.cfg.GetString(ctx, CfgKeyUSDTContractAddress, defaultUSDTContract)
	apiKey := p.cfg.GetString(ctx, CfgKeyUSDTTronGridAPIKey, "")

	// GET https://api.trongrid.io/v1/accounts/{address}/transactions/trc20
	// 参数：limit=50&only_to=true&contract_address={contract}&min_timestamp={ms}
	url := fmt.Sprintf("%s/v1/accounts/%s/transactions/trc20?limit=50&only_to=true&contract_address=%s&min_timestamp=%d&order_by=block_timestamp,asc",
		trongridAPIBase, address, contract, fromTimestamp)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("构造 TronGrid 请求失败: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	if apiKey != "" {
		req.Header.Set("TRON-PRO-API-KEY", apiKey)
	}

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("TronGrid 请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("TronGrid 非 200 响应: %d %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取 TronGrid 响应失败: %w", err)
	}

	var txResp trongridTxResponse
	if err := json.Unmarshal(body, &txResp); err != nil {
		return nil, fmt.Errorf("解析 TronGrid 响应失败: %w", err)
	}
	if !txResp.Success {
		return nil, errors.New("TronGrid success=false")
	}

	txs := make([]TronGridTx, 0, len(txResp.Data))
	for _, d := range txResp.Data {
		// USDT decimals=6，TronGrid 返回 value 为最小单位整数（乘以 10^6）
		// 需转换为 6 位小数字符串
		amount, err := convertTokenValue(d.Value, d.TokenInfo.Decimals)
		if err != nil {
			continue
		}
		txs = append(txs, TronGridTx{
			TxHash:      d.TransactionID,
			From:        d.From,
			To:          d.To,
			Amount:      amount,
			TimestampMs: d.BlockTimestamp,
		})
	}
	return txs, nil
}

// TronGridTx TronGrid 返回的单笔交易（已规范化）
type TronGridTx struct {
	TxHash      string
	From        string
	To          string
	Amount      string // 6 位小数字符串 USDT
	TimestampMs int64
}

// convertTokenValue 将最小单位整数转成 6 位小数字符串
// USDT decimals=6，value="10330000" → "10.330000"
func convertTokenValue(rawValue string, decimals int) (string, error) {
	if decimals <= 0 {
		return rawValue, nil
	}
	// 用 big.Float 精确除以 10^decimals
	bigVal, ok := new(big.Float).SetString(rawValue)
	if !ok {
		return "", fmt.Errorf("invalid value: %s", rawValue)
	}
	divisor := new(big.Float).SetInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(decimals)), nil))
	result := new(big.Float).Quo(bigVal, divisor)
	return result.Text('f', decimals), nil
}

// SignUSDTWebhook 工具函数：为外部监控 payload 生成签名（供测试和文档参考）
func SignUSDTWebhook(outTradeNo, amount string, timestamp int64, secret string) string {
	return crypto.HMACSHA256Hex(outTradeNo+amount+strconv.FormatInt(timestamp, 10), secret)
}
