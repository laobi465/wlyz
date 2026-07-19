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

	"github.com/your-org/keyauth-saas/apps/server/internal/middleware"
	"github.com/your-org/keyauth-saas/apps/server/internal/model"
	"github.com/your-org/keyauth-saas/apps/server/pkg/crypto"
	"github.com/your-org/keyauth-saas/apps/server/pkg/epay"
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

		// 6. 加载易支付配置
		cfg, err := loadPlatformPayConfig(deps)
		if err != nil {
			middleware.Fail(c, http.StatusInternalServerError, 4006, "支付配置错误: "+err.Error())
			return
		}

		// 7. 创建订单
		orderNo := snowflake.OrderNo("ORD")
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
			NotifyURL:  resolveNotifyURL(deps, c),
			ReturnURL:  resolveReturnURL(deps, c),
			ClientIP:   c.ClientIP(),
		}
		payURL, err := epay.BuildSubmitURL(cfg, orderParams)
		if err != nil {
			middleware.Fail(c, http.StatusInternalServerError, 4008, "构造支付链接失败: "+err.Error())
			return
		}

		middleware.Success(c, gin.H{
			"order_no":   orderNo,
			"order_id":   order.ID,
			"pay_url":    payURL,
			"total_amount": totalAmount,
			"money":      moneyStr,
			"pay_type":   req.PayType,
			"expire_at":  time.Now().Add(time.Duration(orderExpire) * time.Second).Unix(),
			"quantity":   req.Quantity,
			"card_type":  ct.Name,
		})
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

		middleware.Success(c, gin.H{
			"order_no":       order.OrderNo,
			"pay_status":     order.PayStatus,
			"pay_channel":    order.PayChannel,
			"total_amount":   order.TotalAmount,
			"quantity":       order.Quantity,
			"card_ids":       cardIDs,
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

		// 4. 处理订单（事务内完成：状态校验 + 自动发卡 + 抽成记录）
		if err := processPaidOrder(deps, notify); err != nil {
			// 处理失败：释放锁以便重试
			deps.Redis.Del(c.Request.Context(), lockKey)
			c.String(200, "fail")
			return
		}

		c.String(200, "success")
	}
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

	return txErr
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

// ============== EpayTenantNotify 开发者自有易支付回调（v0.3.0） ==============

// EpayTenantNotify 开发者自有易支付回调
// POST /api/v1/pay/notify/tenant/:tenant_id
// TODO(v0.3.0): 按租户隔离回调路由，从 tenant_pay_config 读取该租户的易支付配置
func EpayTenantNotify(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.String(200, "fail")
	}
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

// AdminSettleOrder 手动标记订单已结算
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
		if err := deps.DB.Model(&s).Updates(map[string]interface{}{
			"status":           "settled",
			"settled_at":       now,
			"settle_batch_no":  batchNo,
			"settle_method":    req.Method,
			"settle_remark":    req.Remark,
		}).Error; err != nil {
			middleware.Fail(c, http.StatusInternalServerError, 5002, "结算失败: "+err.Error())
			return
		}

		middleware.Success(c, gin.H{
			"id":               s.ID,
			"status":           "settled",
			"settle_batch_no":  batchNo,
			"settled_at":       now,
		})
	}
}

// ============== 标记未使用导入（防编译报错） ==============

var _ = redis.Client{}
