// 平台总支付（彩虹易支付）处理器
// 对应路由：/api/v1/pay/*
// 严格遵循铁律 04/05：所有可变参数从 sys_config 读取
// 严格遵循铁律 06：不确定处标注「需验证」或「待核实」
package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"github.com/your-org/keyauth-saas/apps/server/internal/metrics"
	"github.com/your-org/keyauth-saas/apps/server/internal/middleware"
	"github.com/your-org/keyauth-saas/apps/server/internal/model"
	"github.com/your-org/keyauth-saas/apps/server/internal/multilevel"
	"github.com/your-org/keyauth-saas/apps/server/internal/openapi"
	"github.com/your-org/keyauth-saas/apps/server/pkg/crypto"
	"github.com/your-org/keyauth-saas/apps/server/pkg/epay"
	"github.com/your-org/keyauth-saas/apps/server/pkg/payment"
	"github.com/your-org/keyauth-saas/apps/server/pkg/snowflake"
)

// ============== DTO ==============

type createOrderReq struct {
	AppID       uint64 `json:"app_id" binding:"required"`
	CardTypeID  uint64 `json:"card_type_id" binding:"required"`
	Quantity    int    `json:"quantity" binding:"required,min=1,max=100"`
	PayType     string `json:"pay_type" binding:"required,oneof=alipay wxpay qqpay"`
	BuyerContact string `json:"buyer_contact" binding:"omitempty,max=128"`
	AgentID     uint64 `json:"agent_id" binding:"omitempty"` // 推广代理 ID（可选）
	PayChannel  string `json:"pay_channel" binding:"omitempty,oneof=epay usdt paypal stripe"` // v0.5.0 海外支付通道，留空默认 epay
}

// ============== 平台易支付配置加载 ==============

// loadPlatformPayConfig 从 sys_config 加载平台易支付配置并解密商户密钥
func loadPlatformPayConfig(deps *Deps) (*epay.Config, error) {
	ctx := context.Background()
	gatewayURL := deps.CfgCache.GetString(ctx, "pay.platform.gateway_url", "")
	pid := deps.CfgCache.GetString(ctx, "pay.platform.pid", "")
	keyEncrypted := deps.CfgCache.GetString(ctx, "pay.platform.key_encrypted", "")
	signType := deps.CfgCache.GetString(ctx, "pay.platform.sign_type", "MD5")

	if gatewayURL == "" || pid == "" || keyEncrypted == "" {
		return nil, fmt.Errorf("平台易支付未配置完整（gateway_url/pid/key_encrypted）")
	}

	// AES 解密商户密钥
	secret, err := deps.Crypto.DecryptAES(keyEncrypted)
	if err != nil {
		return nil, fmt.Errorf("商户密钥解密失败: %w", err)
	}
	if secret == "" {
		return nil, fmt.Errorf("商户密钥为空")
	}

	return &epay.Config{
		GatewayURL: gatewayURL,
		PID:        pid,
		Secret:     secret,
		SignType:   signType,
	}, nil
}

// resolveNotifyURL 拼接完整异步回调 URL
func resolveNotifyURL(deps *Deps, c *gin.Context) string {
	ctx := context.Background()
	notifyPath := deps.CfgCache.GetString(ctx, "pay.platform.notify_path", "/api/v1/pay/notify/epay")
	// 优先用请求头中的 Host（保证公网域名），否则回退到配置
	scheme := "https"
	if r := c.Request; r.TLS == nil {
		if xfp := r.Header.Get("X-Forwarded-Proto"); xfp != "" {
			scheme = xfp
		} else {
			scheme = "http"
		}
	}
	host := c.Request.Host
	if host == "" {
		// 兜底：用配置中的网关地址域名（待核实：是否应单独配置 notify_base_url）
		return strings.TrimRight(deps.CfgCache.GetString(ctx, "pay.platform.gateway_url", ""), "/") + notifyPath
	}
	return scheme + "://" + host + notifyPath
}

// resolveReturnURL 拼接同步跳转 URL
func resolveReturnURL(deps *Deps, c *gin.Context) string {
	ctx := context.Background()
	returnPath := deps.CfgCache.GetString(ctx, "pay.platform.return_path", "/api/v1/pay/return/epay")
	scheme := "https"
	if r := c.Request; r.TLS == nil {
		if xfp := r.Header.Get("X-Forwarded-Proto"); xfp != "" {
			scheme = xfp
		} else {
			scheme = "http"
		}
	}
	host := c.Request.Host
	if host == "" {
		return strings.TrimRight(deps.CfgCache.GetString(ctx, "pay.platform.gateway_url", ""), "/") + returnPath
	}
	return scheme + "://" + host + returnPath
}

// ============== CreatePayOrder 创建支付订单 ==============

// CreatePayOrder 终端用户下单
// POST /api/v1/pay/order
// 流程：
//  1. 校验应用/卡类状态
//  2. 校验平台支付是否启用
//  3. 计算订单金额（card_type.price × quantity）
//  4. 创建 AppOrder（pending）
//  5. 构造易支付跳转 URL
//  6. 返回 pay_url 给前端
func CreatePayOrder(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req createOrderReq
		if err := c.ShouldBindJSON(&req); err != nil {
			middleware.Fail(c, http.StatusBadRequest, 1001, "参数错误: "+err.Error())
			return
		}

		ctx := c.Request.Context()

		// 1. 校验平台支付总开关
		if !deps.CfgCache.GetBool(ctx, "pay.platform.enabled", true) {
			middleware.Fail(c, http.StatusForbidden, 4001, "平台总支付未启用")
			return
		}

		// 2. 校验应用
		var app model.App
		if err := deps.DB.Where("id = ? AND status = ?", req.AppID, "active").First(&app).Error; err != nil {
			middleware.Fail(c, http.StatusForbidden, 4002, "应用不存在或已禁用")
			return
		}

		// 3. 校验卡类
		var ct model.AppCardType
		if err := deps.DB.Where("id = ? AND app_id = ? AND tenant_id = ? AND status = ?",
			req.CardTypeID, app.ID, app.TenantID, "active").First(&ct).Error; err != nil {
			middleware.Fail(c, http.StatusForbidden, 4003, "卡类不存在或已下架")
			return
		}

		// 4. 校验开发者账号状态
		var tenant model.SysTenant
		if err := deps.DB.First(&tenant, app.TenantID).Error; err != nil {
			middleware.Fail(c, http.StatusForbidden, 4004, "开发者账号不存在")
			return
		}
		if tenant.Status != "active" {
			middleware.Fail(c, http.StatusForbidden, 4004, "开发者账号已被禁用")
			return
		}
		if tenant.ExpiresAt != nil && tenant.ExpiresAt.Before(time.Now()) {
			middleware.Fail(c, http.StatusForbidden, 4004, "开发者账号已过期")
			return
		}

		// 5. 计算订单金额
		totalAmount := ct.Price * float64(req.Quantity)
		if totalAmount <= 0 {
			middleware.Fail(c, http.StatusBadRequest, 4005, "订单金额必须大于 0")
			return
		}

		// 5.1 v0.5.0 海外支付通道分流（usdt/paypal/stripe）
		//     海外通道不走双层支付模式，统一走平台总支付（独立 webhook 回调）
		//     国内 epay 通道仍走 6. 双层支付模式
		if req.PayChannel == payment.ChannelUSDT ||
			req.PayChannel == payment.ChannelPayPal ||
			req.PayChannel == payment.ChannelStripe {
			handleOverseasPayOrder(deps, c, &req, &app, &ct, &tenant, totalAmount)
			return
		}

		// 6. 双层支付模式切换（v0.3.6）：
		//    优先级：套餐 AllowCustomPay=true + 开发者 TenantPayConfig.Enabled=true → 走自有支付
		//    否则：回退平台总支付（需 pay.platform.enabled=true）
		var pkg model.SysPackage
		_ = deps.DB.First(&pkg, tenant.PackageID).Error

		useTenantPay := false
		if pkg.AllowCustomPay {
			var tpc model.TenantPayConfig
			if err := deps.DB.Where("tenant_id = ? AND channel = ? AND enabled = ?",
				app.TenantID, "epay", true).First(&tpc).Error; err == nil {
				useTenantPay = true
			}
		}

		var (
			payCfg    *epay.Config
			orderNo   string
			payNotify string
			payReturn string
			payErr    error
		)
		if useTenantPay {
			// 6.1 走开发者自有易支付
			payCfg, payErr = loadTenantPayConfig(deps, app.TenantID)
			if payErr != nil {
				middleware.Fail(c, http.StatusInternalServerError, 4006, "自有支付配置错误: "+payErr.Error())
				return
			}
			orderNo = snowflake.OrderNo("TOP")
			// 回调 URL 携带 tenant_id 以便 EpayTenantNotify 加载对应密钥
			payNotify = resolveTenantNotifyURL(deps, c, app.TenantID)
			payReturn = resolveTenantReturnURL(deps, c)
		} else {
			// 6.2 走平台总支付
			if !deps.CfgCache.GetBool(ctx, "pay.platform.enabled", true) {
				middleware.Fail(c, http.StatusForbidden, 4001, "平台总支付未启用且开发者未开通自有支付")
				return
			}
			payCfg, payErr = loadPlatformPayConfig(deps)
			if payErr != nil {
				middleware.Fail(c, http.StatusInternalServerError, 4006, "平台支付配置错误: "+payErr.Error())
				return
			}
			orderNo = snowflake.OrderNo("ORD")
			payNotify = resolveNotifyURL(deps, c)
			payReturn = resolveReturnURL(deps, c)
		}

		// 7. 创建订单
		var agentID *uint64
		if req.AgentID > 0 {
			agentID = &req.AgentID
		}
		order := &model.AppOrder{
			TenantID:     app.TenantID,
			AppID:        app.ID,
			CardTypeID:   ct.ID,
			OrderNo:      orderNo,
			BuyerContact: req.BuyerContact,
			AgentID:      agentID,
			Quantity:     req.Quantity,
			UnitPrice:    ct.Price,
			TotalAmount:  totalAmount,
			PayChannel:   "epay_" + req.PayType,
			PayStatus:    "pending",
			ClientIP:     c.ClientIP(),
		}
		if err := deps.DB.Create(order).Error; err != nil {
			middleware.Fail(c, http.StatusInternalServerError, 4007, "创建订单失败: "+err.Error())
			return
		}

		// 8. 构造易支付跳转 URL
		orderExpire := deps.CfgCache.GetInt(ctx, "pay.order_expire_seconds", 1800)
		namePrefix := deps.CfgCache.GetString(ctx, "pay.platform.order_name_prefix", "KeyAuth卡密")
		moneyStr := strconv.FormatFloat(totalAmount, 'f', 2, 64)
		orderParams := &epay.OrderParams{
			OutTradeNo: orderNo,
			Name:       fmt.Sprintf("%s·%s×%d", namePrefix, ct.Name, req.Quantity),
			Money:      moneyStr,
			PayType:    req.PayType,
			NotifyURL:  payNotify,
			ReturnURL:  payReturn,
			ClientIP:   c.ClientIP(),
		}
		payURL, buildErr := epay.BuildSubmitURL(payCfg, orderParams)
		if buildErr != nil {
			middleware.Fail(c, http.StatusInternalServerError, 4008, "构造支付链接失败: "+buildErr.Error())
			return
		}

		middleware.Success(c, gin.H{
			"order_no":       orderNo,
			"order_id":       order.ID,
			"pay_url":        payURL,
			"total_amount":   totalAmount,
			"money":          moneyStr,
			"pay_type":       req.PayType,
			"pay_mode":       ternary(useTenantPay, "tenant", "platform"), // v0.3.6 标识支付通道
			"expire_at":      time.Now().Add(time.Duration(orderExpire) * time.Second).Unix(),
			"quantity":       req.Quantity,
			"card_type":      ct.Name,
		})
	}
}

// resolveTenantNotifyURL 拼接开发者自有易支付异步回调 URL（携带 tenant_id）
func resolveTenantNotifyURL(deps *Deps, c *gin.Context, tenantID uint64) string {
	ctx := context.Background()
	notifyPath := deps.CfgCache.GetString(ctx, "pay.tenant.notify_path", "/api/v1/pay/notify/tenant/")
	scheme := "https"
	if r := c.Request; r.TLS == nil {
		if xfp := r.Header.Get("X-Forwarded-Proto"); xfp != "" {
			scheme = xfp
		} else {
			scheme = "http"
		}
	}
	host := c.Request.Host
	if host == "" {
		return strings.TrimRight(deps.CfgCache.GetString(ctx, "pay.platform.gateway_url", ""), "/") +
			notifyPath + strconv.FormatUint(tenantID, 10)
	}
	return scheme + "://" + host + notifyPath + strconv.FormatUint(tenantID, 10)
}

// resolveTenantReturnURL 拼接开发者自有易支付同步跳转 URL（复用平台 return 配置）
func resolveTenantReturnURL(deps *Deps, c *gin.Context) string {
	return resolveReturnURL(deps, c)
}

// ternary 简化的三元运算符辅助
func ternary(cond bool, a, b string) string {
	if cond {
		return a
	}
	return b
}

// ============== v0.5.0 海外支付通道（USDT/PayPal/Stripe） ==============

// overseasChannelPrefix 海外通道对应的订单号前缀（3 位，与 dispatchPaidOrder 一致）
// 铁律 04：前缀集中定义，避免散落
var overseasChannelPrefix = map[string]string{
	payment.ChannelUSDT:   "UST",
	payment.ChannelPayPal: "PPL",
	payment.ChannelStripe: "STP",
}

// overseasNotifyPath 海外通道 webhook 路径（从 sys_config 读取，留空使用默认）
var overseasNotifyPath = map[string]string{
	payment.ChannelUSDT:   "pay.usdt.notify_path",
	payment.ChannelPayPal: "pay.paypal.notify_path",
	payment.ChannelStripe: "pay.stripe.notify_path",
}

var overseasNotifyPathDefault = map[string]string{
	payment.ChannelUSDT:   "/api/v1/pay/notify/usdt",
	payment.ChannelPayPal: "/api/v1/pay/notify/paypal",
	payment.ChannelStripe: "/api/v1/pay/notify/stripe",
}

// resolveOverseasNotifyURL 拼接海外通道异步回调 URL
func resolveOverseasNotifyURL(deps *Deps, c *gin.Context, channel string) string {
	ctx := context.Background()
	notifyPath := deps.CfgCache.GetString(ctx, overseasNotifyPath[channel], overseasNotifyPathDefault[channel])
	scheme := "https"
	if r := c.Request; r.TLS == nil {
		if xfp := r.Header.Get("X-Forwarded-Proto"); xfp != "" {
			scheme = xfp
		} else {
			scheme = "http"
		}
	}
	host := c.Request.Host
	if host == "" {
		return strings.TrimRight(deps.CfgCache.GetString(ctx, "pay.platform.gateway_url", ""), "/") + notifyPath
	}
	return scheme + "://" + host + notifyPath
}

// handleOverseasPayOrder 处理海外支付通道下单
// 流程：
//  1. 校验对应通道启用
//  2. 创建订单（UST/PPL/STP 前缀）
//  3. 调用 payment.Provider.CreateOrder
//  4. 返回通道特定字段（QRContent / PaymentURL / ClientSecret）
func handleOverseasPayOrder(deps *Deps, c *gin.Context, req *createOrderReq, app *model.App, ct *model.AppCardType, tenant *model.SysTenant, totalAmount float64) {
	ctx := c.Request.Context()

	// 1. 校验通道启用
	var enabledKey string
	switch req.PayChannel {
	case payment.ChannelUSDT:
		enabledKey = payment.CfgKeyUSDTEnabled
	case payment.ChannelPayPal:
		enabledKey = payment.CfgKeyPayPalEnabled
	case payment.ChannelStripe:
		enabledKey = payment.CfgKeyStripeEnabled
	default:
		middleware.Fail(c, http.StatusBadRequest, 1001, "未知支付通道: "+req.PayChannel)
		return
	}
	if !deps.CfgCache.GetBool(ctx, enabledKey, false) {
		middleware.Fail(c, http.StatusForbidden, 4012, "支付通道未启用: "+req.PayChannel)
		return
	}

	// 2. 创建订单（pending）
	prefix := overseasChannelPrefix[req.PayChannel]
	orderNo := snowflake.OrderNo(prefix)
	var agentID *uint64
	if req.AgentID > 0 {
		agentID = &req.AgentID
	}
	order := &model.AppOrder{
		TenantID:     app.TenantID,
		AppID:        app.ID,
		CardTypeID:   ct.ID,
		OrderNo:      orderNo,
		BuyerContact: req.BuyerContact,
		AgentID:      agentID,
		Quantity:     req.Quantity,
		UnitPrice:    ct.Price,
		TotalAmount:  totalAmount,
		PayChannel:   req.PayChannel,
		PayStatus:    "pending",
		ClientIP:     c.ClientIP(),
	}
	if err := deps.DB.Create(order).Error; err != nil {
		middleware.Fail(c, http.StatusInternalServerError, 4007, "创建订单失败: "+err.Error())
		return
	}

	// 3. 构造 provider 调用参数
	//    - USDT：Amount 留空，provider 内部根据 AmountRaw × 汇率 + 后缀计算
	//    - PayPal：Amount = USD 2 位小数（CNY × exchange_rate）
	//    - Stripe：Amount = cents 整数（CNY × exchange_rate × 100）
	var amountStr string
	switch req.PayChannel {
	case payment.ChannelUSDT:
		amountStr = "" // USDT 由 provider 内部计算
	case payment.ChannelPayPal:
		rate := deps.CfgCache.GetFloat64(ctx, "pay.paypal.exchange_rate", 0.14)
		usdAmount := totalAmount * rate
		amountStr = strconv.FormatFloat(usdAmount, 'f', 2, 64)
	case payment.ChannelStripe:
		rate := deps.CfgCache.GetFloat64(ctx, "pay.stripe.exchange_rate", 0.14)
		usdAmount := totalAmount * rate
		cents := int64(usdAmount*100 + 0.5) // 四舍五入
		amountStr = strconv.FormatInt(cents, 10)
	}

	namePrefix := deps.CfgCache.GetString(ctx, "pay.platform.order_name_prefix", "KeyAuth卡密")
	orderParams := &payment.OrderParams{
		OrderNo:    orderNo,
		Amount:     amountStr,
		AmountRaw:  totalAmount,
		Subject:    fmt.Sprintf("%s·%s×%d", namePrefix, ct.Name, req.Quantity),
		NotifyURL:  resolveOverseasNotifyURL(deps, c, req.PayChannel),
		ReturnURL:  resolveReturnURL(deps, c),
		ClientIP:   c.ClientIP(),
		OrderID:    order.ID,
	}

	// 4. 初始化 provider 并创建订单
	var provider payment.Provider
	switch req.PayChannel {
	case payment.ChannelUSDT:
		provider = payment.NewUSDTProvider(deps.CfgCache, deps.Crypto)
	case payment.ChannelPayPal:
		provider = payment.NewPayPalProvider(deps.CfgCache, deps.Crypto)
	case payment.ChannelStripe:
		provider = payment.NewStripeProvider(deps.CfgCache, deps.Crypto)
	}

	result, err := provider.CreateOrder(ctx, orderParams)
	if err != nil {
		// 创建失败：回滚订单状态为 closed（避免占用订单号）
		deps.DB.Model(order).Update("pay_status", "closed")
		middleware.Fail(c, http.StatusInternalServerError, 4008, "创建支付订单失败: "+err.Error())
		return
	}

	// 5. 返回通道特定字段
	resp := gin.H{
		"order_no":     orderNo,
		"order_id":     order.ID,
		"pay_channel":  req.PayChannel,
		"pay_mode":     "platform", // 海外通道统一走平台
		"total_amount": totalAmount,
		"expire_at":    time.Now().Add(time.Duration(result.ExpireSeconds) * time.Second).Unix(),
		"quantity":     req.Quantity,
		"card_type":    ct.Name,
	}
	switch req.PayChannel {
	case payment.ChannelUSDT:
		resp["qr_content"] = result.QRContent
		resp["address"] = result.Address
		resp["amount"] = result.Amount
		resp["currency"] = "USDT"
	case payment.ChannelPayPal:
		resp["pay_url"] = result.PaymentURL
		resp["amount"] = result.Amount
		resp["currency"] = "USD"
	case payment.ChannelStripe:
		resp["client_secret"] = result.ClientSecret
		resp["amount"] = result.Amount
		resp["currency"] = "USD"
	}
	middleware.Success(c, resp)
}

// notifyToEpayNotify 将 payment.NotifyData 转换为 epay.NotifyParams 以复用 dispatchPaidOrder
// 金额字段直接使用订单 TotalAmount（已通过 provider 验签 + 通道金额校验保证真实性）
func notifyToEpayNotify(data *payment.NotifyData, order *model.AppOrder) *epay.NotifyParams {
	return &epay.NotifyParams{
		OutTradeNo:  data.OutTradeNo,
		TradeNo:     data.TradeNo,
		Money:       strconv.FormatFloat(order.TotalAmount, 'f', 2, 64),
		TradeStatus: "TRADE_SUCCESS",
	}
}

// handleOverseasNotify 海外通道 webhook 通用处理
// 流程：
//  1. provider.ParseNotify 验签
//  2. Redis 防重入
//  3. 查订单 + 通道特定金额校验
//  4. dispatchPaidOrder 处理
func handleOverseasNotify(deps *Deps, c *gin.Context, provider payment.Provider) {
	ctx := c.Request.Context()

	// 1. 验签
	data, err := provider.ParseNotify(ctx, c.Request)
	if err != nil {
		c.String(http.StatusOK, "fail")
		return
	}
	if !data.IsPaid() {
		// 验签失败或状态非 paid：返回 success 避免重试（PayPal/Stripe 会持续重试失败回调）
		// 但如果是验签失败，应返回 fail 让对端排查
		if data.VerifyError != nil {
			c.String(http.StatusOK, "fail")
			return
		}
		c.String(http.StatusOK, "success")
		return
	}

	// 2. Redis 防重入（60s 锁）
	lockKey := "pay:notify:lock:" + data.OutTradeNo
	ok, err := deps.Redis.SetNX(ctx, lockKey, "1", 60*time.Second).Result()
	if err != nil || !ok {
		c.String(http.StatusOK, "success")
		return
	}

	// 3. 查订单
	var order model.AppOrder
	if err := deps.DB.Where("order_no = ?", data.OutTradeNo).First(&order).Error; err != nil {
		deps.Redis.Del(ctx, lockKey)
		c.String(http.StatusOK, "fail")
		return
	}

	// 4. 通道特定金额校验（防伪造回调金额）
	switch provider.Name() {
	case payment.ChannelUSDT:
		// USDT：用 MatchOrderAmount 严格校验链上金额（baseUSDT + order_id 后缀）
		rate := deps.CfgCache.GetFloat64(ctx, payment.CfgKeyUSDTExchangeRate, 0.14)
		baseUSDT := order.TotalAmount * rate
		if err := payment.MatchOrderAmount(baseUSDT, order.ID, data.Amount); err != nil {
			deps.Redis.Del(ctx, lockKey)
			c.String(http.StatusOK, "fail")
			return
		}
	case payment.ChannelPayPal:
		// PayPal：USD 金额校验（data.Amount 与订单预期 USD 金额一致）
		rate := deps.CfgCache.GetFloat64(ctx, "pay.paypal.exchange_rate", 0.14)
		expectedUSD := strconv.FormatFloat(order.TotalAmount*rate, 'f', 2, 64)
		if data.Amount != expectedUSD {
			deps.Redis.Del(ctx, lockKey)
			c.String(http.StatusOK, "fail")
			return
		}
	case payment.ChannelStripe:
		// Stripe：cents 金额校验
		rate := deps.CfgCache.GetFloat64(ctx, "pay.stripe.exchange_rate", 0.14)
		expectedCents := int64(order.TotalAmount*rate*100 + 0.5)
		if data.Amount != strconv.FormatInt(expectedCents, 10) {
			deps.Redis.Del(ctx, lockKey)
			c.String(http.StatusOK, "fail")
			return
		}
	}

	// 5. 构造 epay.NotifyParams 复用 dispatchPaidOrder
	notify := notifyToEpayNotify(data, &order)
	if err := dispatchPaidOrder(deps, notify); err != nil {
		deps.Redis.Del(ctx, lockKey)
		c.String(http.StatusOK, "fail")
		return
	}

	c.String(http.StatusOK, "success")
}

// USDTNotify USDT-TRC20 异步回调
// POST /api/v1/pay/notify/usdt
// 由外部区块链监控服务调用，HMAC-SHA256 签名
func USDTNotify(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		provider := payment.NewUSDTProvider(deps.CfgCache, deps.Crypto)
		handleOverseasNotify(deps, c, provider)
	}
}

// PayPalNotify PayPal webhook 回调
// POST /api/v1/pay/notify/paypal
// 事件：PAYMENT.CAPTURE.COMPLETED → 标记 paid
func PayPalNotify(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		provider := payment.NewPayPalProvider(deps.CfgCache, deps.Crypto)
		handleOverseasNotify(deps, c, provider)
	}
}

// StripeNotify Stripe webhook 回调
// POST /api/v1/pay/notify/stripe
// 事件：payment_intent.succeeded → 标记 paid
func StripeNotify(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		provider := payment.NewStripeProvider(deps.CfgCache, deps.Crypto)
		handleOverseasNotify(deps, c, provider)
	}
}

// ============== GetPayOrder 查询订单状态 ==============

// GetPayOrder 终端用户查询订单
// GET /api/v1/pay/order/:order_no
// 同时执行超时关单逻辑
func GetPayOrder(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		orderNo := strings.TrimSpace(c.Param("order_no"))
		if orderNo == "" {
			middleware.Fail(c, http.StatusBadRequest, 1001, "订单号不能为空")
			return
		}

		var order model.AppOrder
		if err := deps.DB.Where("order_no = ?", orderNo).First(&order).Error; err != nil {
			middleware.Fail(c, http.StatusNotFound, 4009, "订单不存在")
			return
		}

		// 超时关单检查（pending 状态 + 超过 pay.order_expire_seconds）
		if order.PayStatus == "pending" {
			orderExpire := deps.CfgCache.GetInt(c.Request.Context(), "pay.order_expire_seconds", 1800)
			if time.Since(order.CreatedAt) > time.Duration(orderExpire)*time.Second {
				deps.DB.Model(&order).Update("pay_status", "closed")
				order.PayStatus = "closed"
			}
		}

		// 解析卡密 ID
		var cardIDs []uint64
		if order.CardIDs != "" {
			_ = json.Unmarshal([]byte(order.CardIDs), &cardIDs)
		}

		// v0.3.5：订单已支付时返回卡密明文，供 H5 终端用户直接查看
		// 安全：仅返回该订单关联的卡密；卡密 ID 数组从订单 card_ids 字段解析
		var cardKeys []string
		if order.PayStatus == "paid" && len(cardIDs) > 0 {
			deps.DB.Model(&model.AppCard{}).
				Where("id IN ?", cardIDs).
				Order("id ASC").
				Pluck("card_key", &cardKeys)
		}

		middleware.Success(c, gin.H{
			"order_no":       order.OrderNo,
			"pay_status":     order.PayStatus,
			"pay_channel":    order.PayChannel,
			"total_amount":   order.TotalAmount,
			"quantity":       order.Quantity,
			"card_ids":       cardIDs,
			"card_keys":      cardKeys, // v0.3.5：仅在 paid 时非空
			"pay_trade_no":   order.PayTradeNo,
			"paid_at":        order.PaidAt,
			"created_at":     order.CreatedAt,
		})
	}
}

// ============== EpayNotify 异步回调 ==============

// EpayNotify 平台总支付异步回调
// POST/GET /api/v1/pay/notify/epay
// 流程：
//  1. 收集所有参数（GET + POST 合并）
//  2. 验签
//  3. Redis 防重入（SETNX）
//  4. 查订单 → 校验金额
//  5. 事务：更新订单 paid + 自动发卡 + 写抽成记录
//  6. 返回 "success"
func EpayNotify(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 1. 收集所有参数（彩虹易支付可能 POST 或 GET 回调）
		params := collectNotifyParams(c)

		// 2. 加载易支付配置 + 验签
		cfg, err := loadPlatformPayConfig(deps)
		if err != nil {
			c.String(200, "fail")
			return
		}
		if !epay.VerifyNotify(params, cfg.Secret) {
			c.String(200, "fail")
			return
		}

		notify := epay.ParseNotify(params)
		if !notify.IsSuccess() {
			c.String(200, "fail")
			return
		}

		// 3. Redis 防重入（同一订单号 60 秒内只处理一次）
		lockKey := "pay:notify:lock:" + notify.OutTradeNo
		ok, err := deps.Redis.SetNX(c.Request.Context(), lockKey, "1", 60*time.Second).Result()
		if err != nil || !ok {
			// 已被处理或锁定中，直接返回 success 避免支付平台重试
			c.String(200, "success")
			return
		}

			// 4. 处理订单（按订单号前缀分发，v0.3.6 新增 REG 代理注册分支）
		//    ORD → 卡密购买（processPaidOrder）
		//    REG → 代理注册（processAgentRegisterPaid）
		if err := dispatchPaidOrder(deps, notify); err != nil {
			// 处理失败：释放锁以便重试
			deps.Redis.Del(c.Request.Context(), lockKey)
			c.String(200, "fail")
			return
		}

		c.String(200, "success")
	}
}

// dispatchPaidOrder 按订单号前缀分发到对应业务处理器
// 前缀定义（铁律 04：集中分发，避免散落）：
//   - ORD → 平台总支付卡密购买（processPaidOrder）
//   - TOP → 开发者自有易支付卡密购买（processTenantOwnPaidOrder，v0.3.6）
//   - REG → 代理注册付费（processAgentRegisterPaid）
//   - MFD → 开发者月度服务费（processMonthlyFeePaid，v0.4.x）
//   - UST → USDT-TRC20 卡密购买（v0.5.0，复用 processPaidOrder）
//   - PPL → PayPal 卡密购买（v0.5.0，复用 processPaidOrder）
//   - STP → Stripe 卡密购买（v0.5.0，复用 processPaidOrder）
func dispatchPaidOrder(deps *Deps, notify *epay.NotifyParams) error {
	// v0.4.x Prometheus 业务埋点：按订单前缀 + 处理结果统计支付订单总数
	// 铁律 06：status=success/fail 由 err 是否为 nil 决定
	var prefix string
	var err error
	switch {
	case strings.HasPrefix(notify.OutTradeNo, "REG"):
		prefix = "REG"
		err = processAgentRegisterPaid(deps, notify)
	case strings.HasPrefix(notify.OutTradeNo, "MFD"):
		prefix = "MFD"
		err = processMonthlyFeePaid(deps, notify)
	case strings.HasPrefix(notify.OutTradeNo, "TOP"):
		prefix = "TOP"
		err = processTenantOwnPaidOrder(deps, notify)
	case strings.HasPrefix(notify.OutTradeNo, "ORD"),
		strings.HasPrefix(notify.OutTradeNo, "UST"),
		strings.HasPrefix(notify.OutTradeNo, "PPL"),
		strings.HasPrefix(notify.OutTradeNo, "STP"):
		// v0.5.0：USDT/PayPal/Stripe 均为平台级卡密购买，复用 processPaidOrder
		prefix = notify.OutTradeNo[:3]
		err = processPaidOrder(deps, notify)
	default:
		prefix = "UNKNOWN"
		err = fmt.Errorf("未知订单前缀: %s", notify.OutTradeNo)
	}

	status := "success"
	if err != nil {
		status = "fail"
	}
	metrics.IncPayOrder(prefix, status)

	// 金额累计（成功时按分计入 PayOrderAmountTotal）
	if err == nil {
		if moneyFloat, e := strconv.ParseFloat(notify.Money, 64); e == nil {
			metrics.AddPayOrderAmount(prefix, moneyFloat*100)
		}
	}

	return err
}

// collectNotifyParams 合并 GET + POST + Form 参数
func collectNotifyParams(c *gin.Context) map[string]string {
	params := make(map[string]string)

	// GET query
	for k, v := range c.Request.URL.Query() {
		if len(v) > 0 {
			params[k] = v[0]
		}
	}

	// POST form
	if err := c.Request.ParseForm(); err == nil {
		for k, v := range c.Request.PostForm {
			if len(v) > 0 {
				params[k] = v[0]
			}
		}
	}

	// 兼容 application/x-www-form-urlencoded 之外的 POST body
	if c.Request.Method == "POST" && c.Request.Header.Get("Content-Type") != "" {
		// gin 的 PostForm 已在 ParseForm 中处理
		for k, v := range c.Request.PostForm {
			if _, exists := params[k]; !exists && len(v) > 0 {
				params[k] = v[0]
			}
		}
	}

	return params
}

// processPaidOrder 处理支付成功的订单（事务）
func processPaidOrder(deps *Deps, notify *epay.NotifyParams) error {
	// 1. 查订单
	var order model.AppOrder
	if err := deps.DB.Where("order_no = ?", notify.OutTradeNo).First(&order).Error; err != nil {
		return fmt.Errorf("订单不存在: %w", err)
	}

	// 2. 校验金额（防止伪造回调）
	expectedMoney := strconv.FormatFloat(order.TotalAmount, 'f', 2, 64)
	if notify.Money != expectedMoney {
		return fmt.Errorf("金额不匹配: 期望 %s，实际 %s", expectedMoney, notify.Money)
	}

	// 3. 校验订单状态（幂等：已支付直接返回成功）
	if order.PayStatus == "paid" {
		return nil
	}
	if order.PayStatus != "pending" {
		return fmt.Errorf("订单状态异常: %s", order.PayStatus)
	}

	// 4. 查询卡类（自动发卡参数来源）
	var ct model.AppCardType
	if err := deps.DB.First(&ct, order.CardTypeID).Error; err != nil {
		return fmt.Errorf("卡类不存在: %w", err)
	}

	// 5. 查询开发者套餐（计算抽成）
	var tenant model.SysTenant
	if err := deps.DB.First(&tenant, order.TenantID).Error; err != nil {
		return fmt.Errorf("开发者不存在: %w", err)
	}
	var pkg model.SysPackage
	_ = deps.DB.First(&pkg, tenant.PackageID).Error

	// 6. 计算抽成
	ctx := context.Background()
	commissionRate := pkg.PlatformCommissionRate
	if commissionRate <= 0 {
		commissionRate = deps.CfgCache.GetFloat64(ctx, "pay.platform.commission_default", 5.00)
	}
	commissionAmount := order.TotalAmount * commissionRate / 100
	netAmount := order.TotalAmount - commissionAmount

	// 7. 事务：更新订单 + 自动发卡 + 写抽成记录
	now := time.Now()
	txErr := deps.DB.Transaction(func(tx *gorm.DB) error {
		// 7.1 更新订单状态
		if err := tx.Model(&order).Updates(map[string]interface{}{
			"pay_status":    "paid",
			"pay_trade_no":  notify.TradeNo,
			"paid_at":       now,
			"commission_amount": commissionAmount,
		}).Error; err != nil {
			return fmt.Errorf("更新订单状态失败: %w", err)
		}

		// 7.2 自动发卡（生成 N 张卡密，关联订单）
		cardIDs := make([]uint64, 0, order.Quantity)
		batchNo := fmt.Sprintf("P%s%06d", now.Format("20060102"), order.ID%1000000)
		for i := 0; i < order.Quantity; i++ {
			key, hash, checksum, err := crypto.GenerateCardKey("")
			if err != nil {
				return fmt.Errorf("生成第 %d 张卡密失败: %w", i+1, err)
			}
			card := &model.AppCard{
				TenantID:        order.TenantID,
				AppID:           order.AppID,
				CardTypeID:      order.CardTypeID,
				CardKey:         key,
				CardKeyHash:     hash,
				Checksum:        checksum,
				Status:          "unused",
				BatchNo:         batchNo,
				DurationSeconds: ct.DurationSeconds,
				MaxUses:         ct.MaxUses,
				CreatedBy:       order.TenantID, // 系统代发，记录租户 ID
				CreatorType:     "auto",
				OrderID:         &order.ID,
			}
			if err := tx.Create(card).Error; err != nil {
				return fmt.Errorf("入库第 %d 张卡密失败: %w", i+1, err)
			}
			cardIDs = append(cardIDs, card.ID)
		}

		// 7.3 回填订单 card_ids
		cardIDsJSON, _ := json.Marshal(cardIDs)
		if err := tx.Model(&order).Update("card_ids", string(cardIDsJSON)).Error; err != nil {
			return fmt.Errorf("回填 card_ids 失败: %w", err)
		}

		// 7.4 写抽成结算记录
		settlement := &model.PlatformSettlement{
			TenantID:         order.TenantID,
			OrderID:          order.ID,
			OrderNo:          order.OrderNo,
			GrossAmount:      order.TotalAmount,
			CommissionRate:   commissionRate,
			CommissionAmount: commissionAmount,
			NetAmount:        netAmount,
			Status:           "pending",
		}
		if err := tx.Create(settlement).Error; err != nil {
			return fmt.Errorf("写入抽成记录失败: %w", err)
		}

		return nil
	})

	if txErr != nil {
		return txErr
	}

	// v0.4.0 Webhook：异步分发 order.paid 事件（通知开发者订单已支付 + 已自动发卡）
	DispatchWebhookEvent(deps, order.TenantID, openapi.EventOrderPaid, gin.H{
		"order_no":         order.OrderNo,
		"order_id":         order.ID,
		"app_id":           order.AppID,
		"card_type_id":     order.CardTypeID,
		"quantity":         order.Quantity,
		"total_amount":     order.TotalAmount,
		"commission_amount": commissionAmount,
		"net_amount":       netAmount,
		"pay_trade_no":     notify.TradeNo,
		"paid_at":          now.Unix(),
	})

	return nil
}

// processAgentRegisterPaid 处理代理注册支付成功（v0.3.6 新增）
// 流程（事务内，方案 B：先支付后建 Agent）：
//  1. 查 AgentRegistrationOrder，校验金额 + 状态（pending → paid）
//  2. 查邀请码 + 套餐，事务内重复 quota 校验防 TOCTOU
//  3. 计算 bcrypt 哈希，INSERT Agent{Status: active, CommissionRate: 邀请码.DefaultCommissionRate}
//  4. 回填 AgentRegistrationOrder.AgentID + PayStatus=paid + PaidAt + PayTradeNo
//  5. 邀请码 used_count++，达 max_uses 时 status=exhausted，写 used_by_agent_id
//  6. 注册费不进 PlatformSettlement（直接归平台，与卡密抽成解耦）
func processAgentRegisterPaid(deps *Deps, notify *epay.NotifyParams) error {
	// 1. 查订单
	var order model.AgentRegistrationOrder
	if err := deps.DB.Where("order_no = ?", notify.OutTradeNo).First(&order).Error; err != nil {
		return fmt.Errorf("代理注册订单不存在: %w", err)
	}

	// 2. 校验金额
	expectedMoney := strconv.FormatFloat(order.Amount, 'f', 2, 64)
	if notify.Money != expectedMoney {
		return fmt.Errorf("代理注册订单金额不匹配: 期望 %s，实际 %s", expectedMoney, notify.Money)
	}

	// 3. 幂等校验
	if order.PayStatus == "paid" {
		return nil
	}
	if order.PayStatus != "pending" {
		return fmt.Errorf("代理注册订单状态异常: %s", order.PayStatus)
	}

	// 4. 查邀请码（含默认佣金比例）
	var ic model.AgentInviteCode
	if err := deps.DB.First(&ic, order.InviteCodeID).Error; err != nil {
		return fmt.Errorf("邀请码不存在: %w", err)
	}

	// 5. 查开发者 + 套餐（事务内重复 quota 校验防 TOCTOU）
	var tenant model.SysTenant
	if err := deps.DB.First(&tenant, order.TenantID).Error; err != nil {
		return fmt.Errorf("开发者不存在: %w", err)
	}
	var pkg model.SysPackage
	if err := deps.DB.First(&pkg, tenant.PackageID).Error; err != nil {
		return fmt.Errorf("套餐不存在: %w", err)
	}

	// 6. 计算密码 bcrypt 哈希（cost=12）
	// 注意：密码明文存在订单创建时的会话里，但 AgentRegistrationOrder 未存（铁律 04：不存明文密码）
	//       因此这里需要从订单 ClientIP + 时间戳反推不行——解决方案：在 AgentRegister handler 创建订单时，
	//       把密码哈希缓存到 Redis（key=agent_register:pwd:{order_no}，TTL 30min），支付成功后取出
	ctx := context.Background()
	pwdHashKey := "agent_register:pwd:" + order.OrderNo
	storedHash, err := deps.Redis.Get(ctx, pwdHashKey).Result()
	if err == redis.Nil {
		return fmt.Errorf("密码哈希缓存已过期，请联系开发者重新发起注册")
	}
	if err != nil {
		return fmt.Errorf("读取密码哈希缓存失败: %w", err)
	}

	// 7. 事务：建 Agent + 回填订单 + 邀请码闭环
	now := time.Now()
	// 7.0 计算多级代理层级（v0.4.0）
	//   - 邀请码 creator_type='tenant' → 一级代理（parent_id=0, level=1）
	//   - 邀请码 creator_type='agent'  → 下级代理（parent_id=创建者ID, level=创建者.level+1）
	parentID, agentLevel, levelErr := multilevel.ComputeSubordinateLevel(ctx, deps.DB, deps.CfgCache, &ic)
	if levelErr != nil {
		return levelErr
	}
	var agentID uint64 // v0.4.0：事务外捕获 agent.ID 供 Webhook 使用
	txErr := deps.DB.Transaction(func(tx *gorm.DB) error {
		// 7.1 事务内重复 quota 校验（防 TOCTOU）
		var agentCount int64
		if err := tx.Model(&model.Agent{}).Where("tenant_id = ?", order.TenantID).Count(&agentCount).Error; err != nil {
			return fmt.Errorf("查询代理数失败: %w", err)
		}
		if pkg.MaxAgents <= 0 || int(agentCount) >= pkg.MaxAgents {
			return fmt.Errorf("开发者代理数已达上限 %d，注册失败", pkg.MaxAgents)
		}

		// 7.2 重复用户名校验（防并发）
		var nameExists int64
		tx.Model(&model.Agent{}).Where("tenant_id = ? AND username = ?", order.TenantID, order.Username).Count(&nameExists)
		if nameExists > 0 {
			return fmt.Errorf("用户名 %s 已被占用", order.Username)
		}

		// 7.3 INSERT Agent（Status=active，可直接登录）
		agent := &model.Agent{
			TenantID:        order.TenantID,
			Username:        order.Username,
			PasswordHash:    storedHash,
			Phone:           order.Phone,
			Status:          "active",
			Balance:         0,
			CommissionRate:  ic.DefaultCommissionRate,
			CommissionMode:  "percentage",
			ParentID:        parentID,   // v0.4.0：多级代理上级 ID（0=一级代理）
			Level:           agentLevel, // v0.4.0：代理层级（1/2/3）
			LastLoginAt:     nil,
			LastLoginIP:     "",
		}
		if err := tx.Create(agent).Error; err != nil {
			return fmt.Errorf("创建代理账号失败: %w", err)
		}
		agentID = agent.ID

		// 7.4 回填 AgentRegistrationOrder
		if err := tx.Model(&order).Updates(map[string]interface{}{
			"agent_id":      agent.ID,
			"pay_status":    "paid",
			"pay_trade_no":  notify.TradeNo,
			"paid_at":       now,
		}).Error; err != nil {
			return fmt.Errorf("回填订单失败: %w", err)
		}

		// 7.5 邀请码状态机闭环：原子自增 used_count + 达 max_uses 时置 exhausted
		// P0 高危 7：使用 SQL 原子操作 + WHERE 守门，防并发 TOCTOU 突破 max_uses
		incrRes := tx.Model(&ic).
			Where("id = ? AND used_count < max_uses", ic.ID).
			Updates(map[string]interface{}{
				"used_count":       gorm.Expr("used_count + 1"),
				"used_by_agent_id": agent.ID,
			})
		if incrRes.Error != nil {
			return fmt.Errorf("更新邀请码使用次数失败: %w", incrRes.Error)
		}
		if incrRes.RowsAffected == 0 {
			return fmt.Errorf("邀请码使用次数已用尽")
		}
		// 达 max_uses 时置 exhausted（基于自增后的真实值，二次 UPDATE 避免表达式歧义）
		if err := tx.Model(&ic).Where("id = ? AND used_count >= max_uses", ic.ID).
			Update("status", "exhausted").Error; err != nil {
			return fmt.Errorf("更新邀请码状态为 exhausted 失败: %w", err)
		}

		return nil
	})

	if txErr != nil {
		return txErr
	}

	// 8. 删除 Redis 中的密码哈希缓存（已用过，安全清理）
	deps.Redis.Del(ctx, pwdHashKey)

	// 9. 审计追溯：AgentRegistrationOrder 自身已记录订单号/username/tenant_id/amount/agent_id/paid_at，
	//    邀请码 used_by_agent_id 也已写入，无需额外日志（避免回调上下文缺 OperatorID 等字段写入失败）

	// 10. v0.4.0 Webhook：异步分发 agent.registered 事件（通知开发者新代理注册成功）
	DispatchWebhookEvent(deps, order.TenantID, openapi.EventAgentRegistered, gin.H{
		"order_no":       order.OrderNo,
		"agent_id":       agentID,
		"agent_username": order.Username,
		"agent_phone":    order.Phone,
		"invite_code":    ic.Code,
		"level":          agentLevel,
		"parent_id":      parentID,
		"amount":         order.Amount,
		"pay_trade_no":   notify.TradeNo,
		"registered_at":  now.Unix(),
	})

	// v0.4.x Prometheus 业务埋点：代理注册成功 +1
	metrics.IncAgentRegistered()

	return nil
}

// cacheAgentRegisterPassword 临时缓存代理注册密码哈希到 Redis
// TTL 30min（与 pay.order_expire_seconds 配置项联动可调）
// 铁律 04：不存明文密码到 DB，仅短期缓存 bcrypt 哈希等支付回调使用
func cacheAgentRegisterPassword(deps *Deps, orderNo, pwdHash string) error {
	ctx := context.Background()
	ttl := time.Duration(deps.CfgCache.GetInt64(ctx, "pay.order_expire_seconds", 1800)) * time.Second
	return deps.Redis.Set(ctx, "agent_register:pwd:"+orderNo, pwdHash, ttl).Err()
}

// ============== EpayReturn 同步跳转 ==============

// EpayReturn 平台总支付同步跳转
// GET /api/v1/pay/return/epay
// 用户支付完成后浏览器 302 跳转到前端结果页
func EpayReturn(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := context.Background()
		// 提取订单号（易支付同步跳转会带 out_trade_no 参数）
		orderNo := strings.TrimSpace(c.Query("out_trade_no"))
		if orderNo == "" {
			orderNo = strings.TrimSpace(c.Query("order_no"))
		}

		// 前端结果页地址（默认 /pay/result?order_no=xxx）
		frontURL := deps.CfgCache.GetString(ctx, "pay.platform.return_front_url", "/pay/result")
		if orderNo == "" {
			c.Redirect(http.StatusFound, frontURL)
			return
		}
		sep := "?"
		if strings.Contains(frontURL, "?") {
			sep = "&"
		}
		c.Redirect(http.StatusFound, frontURL+sep+"order_no="+orderNo)
	}
}

// ============== EpayTenantNotify 开发者自有易支付回调（v0.3.6 实现） ==============

// EpayTenantNotify 开发者自有易支付回调
// POST /api/v1/pay/notify/tenant/:tenant_id
// 流程：
//  1. 从 URL 取 tenant_id，加载该租户的 TenantPayConfig（channel=epay, enabled=true）
//  2. AES 解密 key_encrypted
//  3. 收集回调参数 + 验签（用该租户密钥）
//  4. Redis 防重入（key 含 tenant_id 命名空间隔离）
//  5. dispatchPaidOrder 按订单号前缀分发：TOP → processTenantOwnPaidOrder
//  6. 返回 "success"
func EpayTenantNotify(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 1. 取 tenant_id
		tenantIDStr := c.Param("tenant_id")
		tenantID, err := strconv.ParseUint(tenantIDStr, 10, 64)
		if err != nil || tenantID == 0 {
			c.String(200, "fail")
			return
		}

		// 2. 加载该租户的易支付配置
		cfg, err := loadTenantPayConfig(deps, tenantID)
		if err != nil {
			c.String(200, "fail")
			return
		}

		// 3. 收集参数 + 验签
		params := collectNotifyParams(c)
		if !epay.VerifyNotify(params, cfg.Secret) {
			c.String(200, "fail")
			return
		}
		notify := epay.ParseNotify(params)
		if !notify.IsSuccess() {
			c.String(200, "fail")
			return
		}

		// 4. Redis 防重入（按 tenant_id 命名空间隔离）
		lockKey := fmt.Sprintf("pay:notify:tenant:%d:lock:%s", tenantID, notify.OutTradeNo)
		ok, err := deps.Redis.SetNX(c.Request.Context(), lockKey, "1", 60*time.Second).Result()
		if err != nil || !ok {
			c.String(200, "success")
			return
		}

		// 5. 处理订单（TOP 前缀走自有支付分支，ORD 走平台总支付分支）
		if err := dispatchPaidOrder(deps, notify); err != nil {
			deps.Redis.Del(c.Request.Context(), lockKey)
			c.String(200, "fail")
			return
		}

		c.String(200, "success")
	}
}

// loadTenantPayConfig 加载租户自有易支付配置并解密密钥
// 铁律 05：密钥 AES-256-GCM 加密入库，运行时解密
func loadTenantPayConfig(deps *Deps, tenantID uint64) (*epay.Config, error) {
	var cfg model.TenantPayConfig
	if err := deps.DB.Where("tenant_id = ? AND channel = ? AND enabled = ?",
		tenantID, "epay", true).First(&cfg).Error; err != nil {
		return nil, fmt.Errorf("开发者自有易支付未启用: %w", err)
	}
	if cfg.GatewayURL == "" || cfg.PID == "" || cfg.KeyEncrypted == "" {
		return nil, fmt.Errorf("开发者自有易支付配置不完整")
	}
	secret, err := deps.Crypto.DecryptAES(cfg.KeyEncrypted)
	if err != nil {
		return nil, fmt.Errorf("开发者商户密钥解密失败: %w", err)
	}
	if secret == "" {
		return nil, fmt.Errorf("开发者商户密钥为空")
	}
	return &epay.Config{
		GatewayURL: cfg.GatewayURL,
		PID:        cfg.PID,
		Secret:     secret,
		SignType:   "MD5",
	}, nil
}

// processTenantOwnPaidOrder 处理开发者自有易支付成功订单（事务）
// 与 processPaidOrder 区别：
//  1. 不写 PlatformSettlement（资金直接进开发者易支付账户，平台不抽成）
//  2. 平台通过套餐 CustomPayFee 月费模式收取开通费，不在每单抽成
//  3. 写 TenantBalanceLog 记录订单已结算（type=settle，amount=订单总额）
//     → 开发者 balance 累加订单总额，对应"已直接收款"的事实
func processTenantOwnPaidOrder(deps *Deps, notify *epay.NotifyParams) error {
	// 1. 查订单
	var order model.AppOrder
	if err := deps.DB.Where("order_no = ?", notify.OutTradeNo).First(&order).Error; err != nil {
		return fmt.Errorf("订单不存在: %w", err)
	}

	// 2. 校验金额（防伪造回调）
	expectedMoney := strconv.FormatFloat(order.TotalAmount, 'f', 2, 64)
	if notify.Money != expectedMoney {
		return fmt.Errorf("金额不匹配: 期望 %s，实际 %s", expectedMoney, notify.Money)
	}

	// 3. 幂等校验
	if order.PayStatus == "paid" {
		return nil
	}
	if order.PayStatus != "pending" {
		return fmt.Errorf("订单状态异常: %s", order.PayStatus)
	}

	// 4. 查卡类（自动发卡参数来源）
	var ct model.AppCardType
	if err := deps.DB.First(&ct, order.CardTypeID).Error; err != nil {
		return fmt.Errorf("卡类不存在: %w", err)
	}

	// 5. 事务：更新订单 + 自动发卡 + 写开发者流水（balance 累加）
	now := time.Now()
	txErr := deps.DB.Transaction(func(tx *gorm.DB) error {
		// 5.1 更新订单状态
		if err := tx.Model(&order).Updates(map[string]interface{}{
			"pay_status":   "paid",
			"pay_trade_no": notify.TradeNo,
			"paid_at":      now,
		}).Error; err != nil {
			return fmt.Errorf("更新订单状态失败: %w", err)
		}

		// 5.2 自动发卡
		cardIDs := make([]uint64, 0, order.Quantity)
		batchNo := fmt.Sprintf("T%s%06d", now.Format("20060102"), order.ID%1000000)
		for i := 0; i < order.Quantity; i++ {
			key, hash, checksum, err := crypto.GenerateCardKey("")
			if err != nil {
				return fmt.Errorf("生成第 %d 张卡密失败: %w", i+1, err)
			}
			card := &model.AppCard{
				TenantID:        order.TenantID,
				AppID:           order.AppID,
				CardTypeID:      order.CardTypeID,
				CardKey:         key,
				CardKeyHash:     hash,
				Checksum:        checksum,
				Status:          "unused",
				BatchNo:         batchNo,
				DurationSeconds: ct.DurationSeconds,
				MaxUses:         ct.MaxUses,
				CreatedBy:       order.TenantID,
				CreatorType:     "auto",
				OrderID:         &order.ID,
			}
			if err := tx.Create(card).Error; err != nil {
				return fmt.Errorf("入库第 %d 张卡密失败: %w", i+1, err)
			}
			cardIDs = append(cardIDs, card.ID)
		}

		// 5.3 回填订单 card_ids
		cardIDsJSON, _ := json.Marshal(cardIDs)
		if err := tx.Model(&order).Update("card_ids", string(cardIDsJSON)).Error; err != nil {
			return fmt.Errorf("回填 card_ids 失败: %w", err)
		}

		// 5.4 开发者余额累加 + 流水（自有支付资金已直接到开发者账户）
		var tenant model.SysTenant
		if err := tx.Set("gorm:query_option", "FOR UPDATE").First(&tenant, order.TenantID).Error; err != nil {
			return fmt.Errorf("查询开发者失败: %w", err)
		}
		newBalance := tenant.Balance + order.TotalAmount
		if err := tx.Model(&tenant).Update("balance", newBalance).Error; err != nil {
			return fmt.Errorf("更新开发者余额失败: %w", err)
		}

		log := &model.TenantBalanceLog{
			TenantID:       order.TenantID,
			Type:           "settle",
			Amount:         order.TotalAmount,
			BalanceAfter:   newBalance,
			RelatedOrderID: &order.ID,
			PayMethod:      "tenant_epay",
			Status:         "settled",
			Remark:         fmt.Sprintf("自有易支付订单 %s 自动结算", order.OrderNo),
		}
		if err := tx.Create(log).Error; err != nil {
			return fmt.Errorf("写入开发者流水失败: %w", err)
		}

		return nil
	})

	return txErr
}

// processMonthlyFeePaid 处理开发者月度服务费支付成功（v0.4.x 新增）
// 流程（事务）：
//  1. 查 TenantMonthlyFeeOrder，校验金额 + 状态（pending → paid）
//  2. 事务内更新 pay_status=paid + pay_mode=platform_epay + paid_at
//  3. 不写抽成记录（月费直接归平台，与卡密抽成解耦）
// 铁律 04/05：金额从订单自身读取（创建时已从 sys_config 读取并入库）；订单号前缀 MFD
// 铁律 06：幂等校验，已支付直接返回成功
// 注：model 当前未单独存 pay_trade_no 字段，平台交易号通过 Webhook 事件 payload 保留追溯链路（待核实：后续如需对账可加列）
func processMonthlyFeePaid(deps *Deps, notify *epay.NotifyParams) error {
	// 1. 查订单
	var order model.TenantMonthlyFeeOrder
	if err := deps.DB.Where("order_no = ?", notify.OutTradeNo).First(&order).Error; err != nil {
		return fmt.Errorf("月费订单不存在: %w", err)
	}

	// 2. 校验金额（防伪造回调）
	expectedMoney := strconv.FormatFloat(order.Amount, 'f', 2, 64)
	if notify.Money != expectedMoney {
		return fmt.Errorf("月费订单金额不匹配: 期望 %s，实际 %s", expectedMoney, notify.Money)
	}

	// 3. 幂等校验
	if order.PayStatus == "paid" {
		return nil
	}
	if order.PayStatus != "pending" {
		return fmt.Errorf("月费订单状态异常: %s", order.PayStatus)
	}

	// 4. 事务：更新订单状态
	now := time.Now()
	txErr := deps.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&order).Updates(map[string]interface{}{
			"pay_status": "paid",
			"pay_mode":   "platform_epay",
			"paid_at":    now,
		}).Error; err != nil {
			return fmt.Errorf("更新月费订单状态失败: %w", err)
		}
		return nil
	})

	if txErr != nil {
		return txErr
	}

	// 5. v0.4.0 Webhook：异步分发 order.paid 事件（复用订单已支付事件，payload 标注 order_type=monthly_fee）
	DispatchWebhookEvent(deps, order.TenantID, openapi.EventOrderPaid, gin.H{
		"order_no":     order.OrderNo,
		"order_id":     order.ID,
		"order_type":   "monthly_fee",
		"tenant_id":    order.TenantID,
		"amount":       order.Amount,
		"period_start": order.PeriodStart.Unix(),
		"period_end":   order.PeriodEnd.Unix(),
		"pay_trade_no": notify.TradeNo,
		"paid_at":      now.Unix(),
	})

	return nil
}

// ============== 超管：结算记录列表 ==============

// AdminListSettlements 平台抽成结算记录列表
// GET /api/v1/admin/settlements?tenant_id=&status=&page=&page_size=
func AdminListSettlements(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
		pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
		if page < 1 {
			page = 1
		}
		if pageSize < 1 || pageSize > 100 {
			pageSize = 20
		}

		q := deps.DB.Model(&model.PlatformSettlement{})
		if tenantIDStr := c.Query("tenant_id"); tenantIDStr != "" {
			if tid, err := strconv.ParseUint(tenantIDStr, 10, 64); err == nil && tid > 0 {
				q = q.Where("tenant_id = ?", tid)
			}
		}
		if status := c.Query("status"); status != "" {
			q = q.Where("status = ?", status)
		}

		var total int64
		q.Count(&total)

		var rows []model.PlatformSettlement
		if err := q.Order("id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&rows).Error; err != nil {
			middleware.Fail(c, http.StatusInternalServerError, 5001, "查询失败")
			return
		}

		middleware.Success(c, gin.H{
			"list":      rows,
			"total":     total,
			"page":      page,
			"page_size": pageSize,
		})
	}
}

// ============== 超管：平台支付配置测试 ==============

// AdminTestPayConfig 测试平台易支付配置是否可用
// POST /api/v1/admin/pay/test
// 仅校验配置完整性 + 解密是否成功，不发起真实支付
func AdminTestPayConfig(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := context.Background()
		gatewayURL := deps.CfgCache.GetString(ctx, "pay.platform.gateway_url", "")
		pid := deps.CfgCache.GetString(ctx, "pay.platform.pid", "")
		keyEncrypted := deps.CfgCache.GetString(ctx, "pay.platform.key_encrypted", "")

		issues := make([]string, 0)
		if gatewayURL == "" {
			issues = append(issues, "网关地址未配置")
		}
		if pid == "" {
			issues = append(issues, "商户 PID 未配置")
		}
		if keyEncrypted == "" {
			issues = append(issues, "商户密钥未配置")
		}

		if len(issues) == 0 {
			// 尝试解密
			if _, err := deps.Crypto.DecryptAES(keyEncrypted); err != nil {
				issues = append(issues, "商户密钥解密失败: "+err.Error())
			}
		}

		if len(issues) > 0 {
			middleware.Fail(c, http.StatusBadRequest, 4010, "配置异常: "+strings.Join(issues, "; "))
			return
		}

		middleware.Success(c, gin.H{
			"ok":          true,
			"gateway_url": gatewayURL,
			"pid":         pid,
			"sign_type":   deps.CfgCache.GetString(ctx, "pay.platform.sign_type", "MD5"),
		})
	}
}

// ============== 超管：手动结算 ==============

// AdminSettleOrder 手动标记订单已结算（v0.3.4 升级：事务保护 + 累加 tenant balance + 写 tenant_balance_log）
// POST /api/v1/admin/settlements/:id/settle
func AdminSettleOrder(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		idStr := c.Param("id")
		id, err := strconv.ParseUint(idStr, 10, 64)
		if err != nil || id == 0 {
			middleware.Fail(c, http.StatusBadRequest, 1001, "结算记录 ID 无效")
			return
		}

		var req struct {
			Method string `json:"method" binding:"omitempty,oneof=manual alipay wechat bank"`
			Remark string `json:"remark" binding:"omitempty,max=255"`
		}
		_ = c.ShouldBindJSON(&req)
		if req.Method == "" {
			req.Method = "manual"
		}

		var s model.PlatformSettlement
		if err := deps.DB.First(&s, id).Error; err != nil {
			middleware.Fail(c, http.StatusNotFound, 4009, "结算记录不存在")
			return
		}
		if s.Status == "settled" {
			middleware.Fail(c, http.StatusBadRequest, 4011, "该记录已结算")
			return
		}

		now := time.Now()
		batchNo := fmt.Sprintf("STL%s%06d", now.Format("20060102"), id%1000000)

		// 事务：1) 更新 settlement 2) 累加 tenant balance 3) 写 tenant_balance_log
		var balanceAfter float64
		txErr := deps.DB.Transaction(func(tx *gorm.DB) error {
			// 1. 更新结算记录
			if err := tx.Model(&s).Updates(map[string]interface{}{
				"status":          "settled",
				"settled_at":      now,
				"settle_batch_no": batchNo,
				"settle_method":   req.Method,
				"settle_remark":   req.Remark,
			}).Error; err != nil {
				return fmt.Errorf("更新结算记录失败: %w", err)
			}

			// 2. 累加开发者可提现余额（FOR UPDATE 防并发）
			var tenant model.SysTenant
			if err := tx.Set("gorm:query_option", "FOR UPDATE").First(&tenant, s.TenantID).Error; err != nil {
				return fmt.Errorf("查询开发者失败: %w", err)
			}
			newBalance := tenant.Balance + s.NetAmount
			if err := tx.Model(&tenant).Update("balance", newBalance).Error; err != nil {
				return fmt.Errorf("更新开发者余额失败: %w", err)
			}
			balanceAfter = newBalance

			// 3. 写开发者余额流水
			log := &model.TenantBalanceLog{
				TenantID:            s.TenantID,
				Type:                "settle",
				Amount:              s.NetAmount,
				BalanceAfter:        newBalance,
				RelatedOrderID:      &s.OrderID,
				RelatedSettlementID: &s.ID,
				PayMethod:           req.Method,
				SettleBatchNo:       batchNo,
				Status:              "settled",
				Remark:              req.Remark,
			}
			if err := tx.Create(log).Error; err != nil {
				return fmt.Errorf("写入开发者流水失败: %w", err)
			}
			return nil
		})
		if txErr != nil {
			middleware.Fail(c, http.StatusInternalServerError, 5002, "结算失败: "+txErr.Error())
			return
		}

		// 异步记录操作日志（v0.3.3 RecordOperation）
		RecordOperation(deps, c, "settlement", "settle_order", "success", "platform_settlement", &s.ID, map[string]interface{}{
			"tenant_id":  s.TenantID,
			"order_id":   s.OrderID,
			"net_amount": s.NetAmount,
			"batch_no":   batchNo,
		})

		middleware.Success(c, gin.H{
			"id":              s.ID,
			"status":          "settled",
			"settle_batch_no": batchNo,
			"settled_at":      now,
			"balance_after":   balanceAfter,
		})
	}
}

// ============== 标记未使用导入（防编译报错） ==============

var _ = redis.Client{}
