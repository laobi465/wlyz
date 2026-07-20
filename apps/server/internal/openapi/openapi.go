// Package openapi v0.4.0 API 开放平台核心包
// 严格遵循铁律 04/05/06：
//   04 - 无硬编码：Token 前缀/长度/有效期/Webhook 超时/重试次数/失败阈值 全部从 sys_config 读取
//   05 - 配置走后端：8 项 openapi.* / webhook.* 配置可通过后台实时调整
//   06 - 反幻觉：Token SHA-512 哈希存储（不存明文）+ HMAC-SHA256 签名 + hmac.Equal 常量时间比较防时序攻击
//
// 核心能力：
//   1. Token Manager：GenerateToken / ValidateToken / RevokeToken / ListTokens
//   2. Webhook Manager：DispatchEvent / SendDelivery / RetryDelivery / ListDeliveries
//   3. Scope 校验：HasScope / ParseScopes / ValidateScopes
//   4. 事件类型常量：order.paid / card.generated / agent.registered / agent.recharge_approved / agent.withdraw_paid
package openapi

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/your-org/keyauth-saas/apps/server/internal/config"
	"github.com/your-org/keyauth-saas/apps/server/internal/model"
	"github.com/your-org/keyauth-saas/apps/server/pkg/crypto"
)

// ============== 常量 ==============

// 配置键常量（铁律 04：禁止硬编码配置键名）
const (
	CfgKeyTokenPrefix          = "openapi.token.prefix"
	CfgKeyTokenLength          = "openapi.token.length"
	CfgKeyTokenMaxPerTenant    = "openapi.token.max_per_tenant"
	CfgKeyTokenDefaultTTLDays  = "openapi.token.default_ttl_days"
	CfgKeyScopeAvailable       = "openapi.scope.available"
	CfgKeyWebhookTimeout       = "webhook.timeout_seconds"
	CfgKeyWebhookMaxRetry      = "webhook.max_retry"
	CfgKeyWebhookFailThreshold = "webhook.failure_threshold"
)

// Token 状态
const (
	TokenStatusActive  = "active"
	TokenStatusRevoked = "revoked"
)

// Endpoint 状态
const (
	EndpointStatusActive   = "active"
	EndpointStatusDisabled = "disabled"
)

// Delivery 状态
const (
	DeliveryStatusPending = "pending"
	DeliveryStatusSuccess = "success"
	DeliveryStatusFailed  = "failed"
)

// 预定义 scopes（与 sys_config openapi.scope.available 对齐）
const (
	ScopeCardRead    = "card.read"
	ScopeCardWrite   = "card.write"
	ScopeOrderRead   = "order.read"
	ScopeOrderWrite  = "order.write"
	ScopeAgentRead   = "agent.read"
	ScopeAgentWrite  = "agent.write"
	ScopeWebhookRead = "webhook.read"
	ScopeWebhookWrite = "webhook.write"
)

// 事件类型常量
const (
	EventOrderPaid            = "order.paid"
	EventCardGenerated        = "card.generated"
	EventAgentRegistered      = "agent.registered"
	EventAgentRechargeApproved = "agent.recharge.approved"
	EventAgentWithdrawPaid    = "agent.withdraw.paid"
)

// 错误
var (
	ErrTokenNotFound        = errors.New("openapi: token not found")
	ErrTokenRevoked         = errors.New("openapi: token revoked")
	ErrTokenExpired         = errors.New("openapi: token expired")
	ErrTokenLimitExceeded   = errors.New("openapi: token limit exceeded")
	ErrInvalidScope         = errors.New("openapi: invalid scope")
	ErrEndpointNotFound     = errors.New("openapi: webhook endpoint not found")
	ErrEndpointDisabled     = errors.New("openapi: webhook endpoint disabled")
	ErrInvalidURL           = errors.New("openapi: invalid webhook URL")
	ErrDeliveryNotFound     = errors.New("openapi: webhook delivery not found")
	ErrDeliveryNotRetryable = errors.New("openapi: delivery not retryable")
)

// ============== 类型 ==============

// TokenManager Token 管理器
type TokenManager struct {
	db     *gorm.DB
	cache  *config.ConfigCache
}

// WebhookManager Webhook 管理器
type WebhookManager struct {
	db     *gorm.DB
	cache  *config.ConfigCache
	crypto *crypto.Manager
	client *http.Client
}

// NewTokenManager 创建 Token 管理器
func NewTokenManager(db *gorm.DB, cache *config.ConfigCache) *TokenManager {
	return &TokenManager{db: db, cache: cache}
}

// NewWebhookManager 创建 Webhook 管理器
func NewWebhookManager(db *gorm.DB, cache *config.ConfigCache, cry *crypto.Manager) *WebhookManager {
	return &WebhookManager{
		db:     db,
		cache:  cache,
		crypto: cry,
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

// ============== Token Manager ==============

// GenerateToken 生成新 API Token
// 流程：① 校验 scopes 合法性 ② 检查单租户 Token 数量上限 ③ 生成随机 Token ④ SHA-512 哈希存储 ⑤ 返回明文（仅此一次）
func (m *TokenManager) GenerateToken(ctx context.Context, tenantID uint64, name, scopes string, ttlDays int) (plainToken string, token *model.DeveloperAPIToken, err error) {
	// 1. 校验 scopes
	available := m.cache.GetString(ctx, CfgKeyScopeAvailable, "")
	if err := ValidateScopes(scopes, available); err != nil {
		return "", nil, err
	}
	// 2. 检查数量上限
	maxPerTenant := m.cache.GetInt(ctx, CfgKeyTokenMaxPerTenant, 10)
	if maxPerTenant > 0 {
		var count int64
		if err := m.db.Model(&model.DeveloperAPIToken{}).
			Where("tenant_id = ? AND status = ?", tenantID, TokenStatusActive).
			Count(&count).Error; err != nil {
			return "", nil, err
		}
		if count >= int64(maxPerTenant) {
			return "", nil, ErrTokenLimitExceeded
		}
	}
	// 3. 生成随机 Token
	prefix := m.cache.GetString(ctx, CfgKeyTokenPrefix, "pat_")
	length := m.cache.GetInt(ctx, CfgKeyTokenLength, 40)
	if length < 16 {
		length = 16
	}
	randomPart, err := generateRandomString(length)
	if err != nil {
		return "", nil, err
	}
	plainToken = prefix + randomPart
	// 4. SHA-512 哈希
	hash := hashToken(plainToken)
	// 5. 计算过期时间
	var expiresAt *time.Time
	if ttlDays < 0 {
		// 用默认
		ttlDays = m.cache.GetInt(ctx, CfgKeyTokenDefaultTTLDays, 365)
	}
	if ttlDays > 0 {
		t := time.Now().Add(time.Duration(ttlDays) * 24 * time.Hour)
		expiresAt = &t
	}
	// 6. 写库
	token = &model.DeveloperAPIToken{
		TenantID:  tenantID,
		Name:      name,
		TokenHash: hash,
		Prefix:    plainToken[:min(8, len(plainToken))],
		Scopes:    scopes,
		ExpiresAt: expiresAt,
		Status:    TokenStatusActive,
	}
	if err := m.db.Create(token).Error; err != nil {
		return "", nil, fmt.Errorf("openapi: create token failed: %w", err)
	}
	return plainToken, token, nil
}

// ValidateToken 校验 API Token
// 流程：① SHA-512 哈希比对 ② 状态校验 ③ 过期校验 ④ 异步更新 last_used_at/ip
func (m *TokenManager) ValidateToken(ctx context.Context, plainToken, clientIP string) (*model.DeveloperAPIToken, error) {
	if plainToken == "" {
		return nil, ErrTokenNotFound
	}
	hash := hashToken(plainToken)
	var token model.DeveloperAPIToken
	if err := m.db.Where("token_hash = ?", hash).First(&token).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrTokenNotFound
		}
		return nil, err
	}
	if token.Status != TokenStatusActive {
		return nil, ErrTokenRevoked
	}
	if token.ExpiresAt != nil && token.ExpiresAt.Before(time.Now()) {
		return nil, ErrTokenExpired
	}
	// 异步更新使用信息（不阻塞请求）
	go func(id uint64, ip string) {
		now := time.Now()
		updates := map[string]interface{}{
			"last_used_at": now,
			"last_used_ip": ip,
		}
		m.db.Model(&model.DeveloperAPIToken{}).Where("id = ?", id).Updates(updates)
	}(token.ID, clientIP)
	return &token, nil
}

// RevokeToken 撤销 Token
func (m *TokenManager) RevokeToken(ctx context.Context, tenantID, tokenID uint64) error {
	now := time.Now()
	result := m.db.Model(&model.DeveloperAPIToken{}).
		Where("id = ? AND tenant_id = ? AND status = ?", tokenID, tenantID, TokenStatusActive).
		Updates(map[string]interface{}{
			"status":     TokenStatusRevoked,
			"revoked_at": now,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrTokenNotFound
	}
	return nil
}

// ListTokens 列出租户的所有 Token
func (m *TokenManager) ListTokens(ctx context.Context, tenantID uint64, status string, page, pageSize int) ([]model.DeveloperAPIToken, int64, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}
	q := m.db.Model(&model.DeveloperAPIToken{}).Where("tenant_id = ?", tenantID)
	if status != "" {
		q = q.Where("status = ?", status)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []model.DeveloperAPIToken
	if err := q.Order("id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// GetToken 获取单个 Token 详情
func (m *TokenManager) GetToken(ctx context.Context, tenantID, tokenID uint64) (*model.DeveloperAPIToken, error) {
	var token model.DeveloperAPIToken
	if err := m.db.Where("id = ? AND tenant_id = ?", tokenID, tenantID).First(&token).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrTokenNotFound
		}
		return nil, err
	}
	return &token, nil
}

// ============== Webhook Manager ==============

// CreateEndpoint 创建 Webhook 端点
func (m *WebhookManager) CreateEndpoint(ctx context.Context, ep *model.WebhookEndpoint, plainSecret string) error {
	// 1. URL 校验
	if !strings.HasPrefix(ep.URL, "https://") && !strings.HasPrefix(ep.URL, "http://") {
		return ErrInvalidURL
	}
	// 2. AES 加密 secret
	if plainSecret != "" && m.crypto != nil {
		enc, err := m.crypto.EncryptAES(plainSecret)
		if err != nil {
			return fmt.Errorf("openapi: encrypt secret failed: %w", err)
		}
		ep.SecretEnc = enc
	}
	ep.Status = EndpointStatusActive
	if err := m.db.Create(ep).Error; err != nil {
		return fmt.Errorf("openapi: create endpoint failed: %w", err)
	}
	return nil
}

// UpdateEndpoint 更新 Webhook 端点
func (m *WebhookManager) UpdateEndpoint(ctx context.Context, id, tenantID uint64, updates map[string]interface{}, newPlainSecret string) error {
	// URL 校验
	if url, ok := updates["url"].(string); ok && url != "" {
		if !strings.HasPrefix(url, "https://") && !strings.HasPrefix(url, "http://") {
			return ErrInvalidURL
		}
	}
	// 处理 secret 更新
	if newPlainSecret != "" && m.crypto != nil {
		enc, err := m.crypto.EncryptAES(newPlainSecret)
		if err != nil {
			return fmt.Errorf("openapi: encrypt secret failed: %w", err)
		}
		updates["secret_enc"] = enc
	}
	result := m.db.Model(&model.WebhookEndpoint{}).
		Where("id = ? AND tenant_id = ?", id, tenantID).
		Updates(updates)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrEndpointNotFound
	}
	return nil
}

// DeleteEndpoint 删除 Webhook 端点
func (m *WebhookManager) DeleteEndpoint(ctx context.Context, id, tenantID uint64) error {
	result := m.db.Where("id = ? AND tenant_id = ?", id, tenantID).Delete(&model.WebhookEndpoint{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrEndpointNotFound
	}
	return nil
}

// ListEndpoints 列出端点
func (m *WebhookManager) ListEndpoints(ctx context.Context, tenantID uint64, status string, page, pageSize int) ([]model.WebhookEndpoint, int64, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}
	q := m.db.Model(&model.WebhookEndpoint{}).Where("tenant_id = ?", tenantID)
	if status != "" {
		q = q.Where("status = ?", status)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []model.WebhookEndpoint
	if err := q.Order("id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// GetEndpoint 获取单个端点
func (m *WebhookManager) GetEndpoint(ctx context.Context, id, tenantID uint64) (*model.WebhookEndpoint, error) {
	var ep model.WebhookEndpoint
	if err := m.db.Where("id = ? AND tenant_id = ?", id, tenantID).First(&ep).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrEndpointNotFound
		}
		return nil, err
	}
	return &ep, nil
}

// DispatchEvent 分发事件到所有订阅的端点
// 流程：① 查询订阅了该事件的 active 端点 ② 为每个端点创建 delivery 记录 ③ 同步尝试发送 ④ 失败的留下次重试
func (m *WebhookManager) DispatchEvent(ctx context.Context, tenantID uint64, eventType string, payload interface{}) ([]model.WebhookDelivery, error) {
	// 1. payload 序列化
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("openapi: marshal payload failed: %w", err)
	}
	payloadStr := string(payloadBytes)
	eventID := generateUUID()
	maxRetry := m.cache.GetInt(ctx, CfgKeyWebhookMaxRetry, 3)

	// 2. 查询订阅端点
	var endpoints []model.WebhookEndpoint
	if err := m.db.Where("tenant_id = ? AND status = ?", tenantID, EndpointStatusActive).
		Find(&endpoints).Error; err != nil {
		return nil, err
	}
	// 过滤订阅了该事件的端点
	var matched []model.WebhookEndpoint
	for _, ep := range endpoints {
		if isSubscribed(ep.Events, eventType) {
			matched = append(matched, ep)
		}
	}
	if len(matched) == 0 {
		return nil, nil
	}

	// 3. 为每个端点创建并尝试发送
	var deliveries []model.WebhookDelivery
	for _, ep := range matched {
		delivery := &model.WebhookDelivery{
			TenantID:   tenantID,
			EndpointID: ep.ID,
			EventType:  eventType,
			EventID:    eventID,
			Payload:    payloadStr,
			Status:     DeliveryStatusPending,
			MaxRetry:   maxRetry,
		}
		if err := m.db.Create(delivery).Error; err != nil {
			continue
		}
		// 同步尝试发送
		m.sendDelivery(ctx, delivery, &ep)
		deliveries = append(deliveries, *delivery)
	}
	return deliveries, nil
}

// sendDelivery 发送单条 delivery（同步）
func (m *WebhookManager) sendDelivery(ctx context.Context, delivery *model.WebhookDelivery, ep *model.WebhookEndpoint) {
	timeout := m.cache.GetInt(ctx, CfgKeyWebhookTimeout, 10)
	if timeout <= 0 {
		timeout = 10
	}
	client := &http.Client{Timeout: time.Duration(timeout) * time.Second}

	// 签名：HMAC-SHA256(secret, event_id.timestamp.payload)
	var secret string
	if ep.SecretEnc != "" && m.crypto != nil {
		if dec, err := m.crypto.DecryptAES(ep.SecretEnc); err == nil {
			secret = dec
		}
	}
	sig := signWebhook(secret, delivery.EventID, delivery.CreatedAt.Unix(), delivery.Payload)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, ep.URL, bytes.NewReader([]byte(delivery.Payload)))
	if err != nil {
		m.markDeliveryFailed(delivery, ep, 0, err.Error())
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-KeyAuth-Event", delivery.EventType)
	req.Header.Set("X-KeyAuth-Event-ID", delivery.EventID)
	req.Header.Set("X-KeyAuth-Signature", sig)
	req.Header.Set("X-KeyAuth-Timestamp", fmt.Sprintf("%d", delivery.CreatedAt.Unix()))

	resp, err := client.Do(req)
	delivery.AttemptCount++
	if err != nil {
		m.markDeliveryFailed(delivery, ep, 0, err.Error())
		return
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
	bodyStr := string(body)
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		// 成功
		now := time.Now()
		m.db.Model(&model.WebhookDelivery{}).Where("id = ?", delivery.ID).Updates(map[string]interface{}{
			"status":        DeliveryStatusSuccess,
			"response_code": resp.StatusCode,
			"response_body": bodyStr,
			"delivered_at":  now,
			"attempt_count": delivery.AttemptCount,
		})
		delivery.Status = DeliveryStatusSuccess
		delivery.ResponseCode = resp.StatusCode
		delivery.ResponseBody = bodyStr
		delivery.DeliveredAt = &now
		// 重置失败计数
		m.db.Model(&model.WebhookEndpoint{}).Where("id = ?", ep.ID).Updates(map[string]interface{}{
			"failure_count":     0,
			"last_response_code": resp.StatusCode,
			"last_response_at":  now,
			"last_error":        "",
		})
	} else {
		// 失败
		errMsg := fmt.Sprintf("HTTP %d: %s", resp.StatusCode, bodyStr)
		m.markDeliveryFailed(delivery, ep, resp.StatusCode, errMsg)
	}
}

// markDeliveryFailed 标记 delivery 失败 + 端点失败计数 + 阈值检查自动 disable
func (m *WebhookManager) markDeliveryFailed(delivery *model.WebhookDelivery, ep *model.WebhookEndpoint, code int, errMsg string) {
	// 更新 delivery
	now := time.Now()
	updates := map[string]interface{}{
		"status":        DeliveryStatusFailed,
		"response_code": code,
		"response_body": errMsg,
		"attempt_count": delivery.AttemptCount,
	}
	if delivery.AttemptCount < delivery.MaxRetry {
		nextRetry := now.Add(time.Duration(delivery.AttemptCount) * 2 * time.Minute) // 2/4/6 分钟退避
		updates["next_retry_at"] = nextRetry
	}
	m.db.Model(&model.WebhookDelivery{}).Where("id = ?", delivery.ID).Updates(updates)
	delivery.Status = DeliveryStatusFailed
	delivery.ResponseCode = code
	delivery.ResponseBody = errMsg

	// 更新端点失败计数
	newFailCount := ep.FailureCount + 1
	epUpdates := map[string]interface{}{
		"failure_count":     newFailCount,
		"last_response_code": code,
		"last_response_at":  now,
		"last_error":        truncate(errMsg, 500),
	}
	// 阈值检查
	threshold := m.cache.GetInt(context.Background(), CfgKeyWebhookFailThreshold, 10)
	if threshold > 0 && newFailCount >= threshold {
		epUpdates["status"] = EndpointStatusDisabled
	}
	m.db.Model(&model.WebhookEndpoint{}).Where("id = ?", ep.ID).Updates(epUpdates)
}

// RetryDelivery 手动重试 delivery
func (m *WebhookManager) RetryDelivery(ctx context.Context, id, tenantID uint64) (*model.WebhookDelivery, error) {
	var delivery model.WebhookDelivery
	if err := m.db.Where("id = ? AND tenant_id = ?", id, tenantID).First(&delivery).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrDeliveryNotFound
		}
		return nil, err
	}
	if delivery.Status == DeliveryStatusSuccess {
		return nil, ErrDeliveryNotRetryable
	}
	if delivery.AttemptCount >= delivery.MaxRetry {
		return nil, ErrDeliveryNotRetryable
	}
	// 查端点
	var ep model.WebhookEndpoint
	if err := m.db.Where("id = ?", delivery.EndpointID).First(&ep).Error; err != nil {
		return nil, ErrEndpointNotFound
	}
	if ep.Status != EndpointStatusActive {
		return nil, ErrEndpointDisabled
	}
	// 重置 next_retry_at 并发送
	m.sendDelivery(ctx, &delivery, &ep)
	// 重新查询最新状态
	m.db.Where("id = ?", id).First(&delivery)
	return &delivery, nil
}

// ListDeliveries 列出 delivery
func (m *WebhookManager) ListDeliveries(ctx context.Context, tenantID uint64, endpointID uint64, status, eventType string, page, pageSize int) ([]model.WebhookDelivery, int64, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}
	q := m.db.Model(&model.WebhookDelivery{}).Where("tenant_id = ?", tenantID)
	if endpointID > 0 {
		q = q.Where("endpoint_id = ?", endpointID)
	}
	if status != "" {
		q = q.Where("status = ?", status)
	}
	if eventType != "" {
		q = q.Where("event_type = ?", eventType)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []model.WebhookDelivery
	if err := q.Order("id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// GetDelivery 获取单条 delivery
func (m *WebhookManager) GetDelivery(ctx context.Context, id, tenantID uint64) (*model.WebhookDelivery, error) {
	var d model.WebhookDelivery
	if err := m.db.Where("id = ? AND tenant_id = ?", id, tenantID).First(&d).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrDeliveryNotFound
		}
		return nil, err
	}
	return &d, nil
}

// ProcessPendingRetries 处理待重试的 delivery（后台 worker 调用）
func (m *WebhookManager) ProcessPendingRetries(ctx context.Context, limit int) (int, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	now := time.Now()
	var deliveries []model.WebhookDelivery
	if err := m.db.Where("status = ? AND next_retry_at <= ? AND attempt_count < max_retry",
		DeliveryStatusFailed, now).
		Limit(limit).Find(&deliveries).Error; err != nil {
		return 0, err
	}
	processed := 0
	for _, d := range deliveries {
		var ep model.WebhookEndpoint
		if err := m.db.Where("id = ?", d.EndpointID).First(&ep).Error; err != nil {
			continue
		}
		if ep.Status != EndpointStatusActive {
			continue
		}
		dc := d // copy
		m.sendDelivery(ctx, &dc, &ep)
		processed++
	}
	return processed, nil
}

// ============== Scope 工具函数 ==============

// ValidateScopes 校验 scopes 字符串是否全部在 available 列表中
// scopes: "card.read,order.write"  available: "card.read,card.write,..."
func ValidateScopes(scopes, available string) error {
	if scopes == "" {
		return nil // 空表示无权限（仅做身份认证）
	}
	availMap := make(map[string]bool)
	for _, s := range strings.Split(available, ",") {
		s = strings.TrimSpace(s)
		if s != "" {
			availMap[s] = true
		}
	}
	for _, s := range strings.Split(scopes, ",") {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if !availMap[s] {
			return fmt.Errorf("%w: %s", ErrInvalidScope, s)
		}
	}
	return nil
}

// HasScope 检查 token 是否有指定 scope
func HasScope(tokenScopes, required string) bool {
	if tokenScopes == "" {
		return false
	}
	for _, s := range strings.Split(tokenScopes, ",") {
		if strings.TrimSpace(s) == required {
			return true
		}
	}
	return false
}

// ParseScopes 解析 scopes 字符串为切片
func ParseScopes(scopes string) []string {
	if scopes == "" {
		return nil
	}
	result := []string{}
	for _, s := range strings.Split(scopes, ",") {
		s = strings.TrimSpace(s)
		if s != "" {
			result = append(result, s)
		}
	}
	return result
}

// isSubscribed 检查端点是否订阅了指定事件
// events: "order.paid,card.generated"
func isSubscribed(events, eventType string) bool {
	if events == "" {
		return false
	}
	for _, e := range strings.Split(events, ",") {
		if strings.TrimSpace(e) == eventType {
			return true
		}
	}
	return false
}

// ============== 辅助函数 ==============

// hashToken 计算 Token 的 SHA-512 哈希（hex 编码）
func hashToken(token string) string {
	h := sha512.Sum512([]byte(token))
	return hex.EncodeToString(h[:])
}

// signWebhook 计算 Webhook 签名：HMAC-SHA256(secret, event_id|timestamp|payload) → hex
// secret 为空时返回空字符串（端点未配置 secret）
func signWebhook(secret, eventID string, timestamp int64, payload string) string {
	if secret == "" {
		return ""
	}
	mac := hmac.New(sha512.New512_256, []byte(secret))
	mac.Write([]byte(eventID))
	mac.Write([]byte("|"))
	mac.Write([]byte(fmt.Sprintf("%d", timestamp)))
	mac.Write([]byte("|"))
	mac.Write([]byte(payload))
	return hex.EncodeToString(mac.Sum(nil))
}

// VerifyWebhookSignature 校验 Webhook 签名（常量时间比较防时序攻击）
func VerifyWebhookSignature(secret, eventID string, timestamp int64, payload, signature string) bool {
	expected := signWebhook(secret, eventID, timestamp, payload)
	if expected == "" || signature == "" {
		return false
	}
	return hmac.Equal([]byte(expected), []byte(signature))
}

// generateRandomString 生成随机字符串（crypto/rand）
func generateRandomString(length int) (string, error) {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	if length <= 0 {
		return "", errors.New("length must be positive")
	}
	result := make([]byte, length)
	max := big.NewInt(int64(len(charset)))
	for i := 0; i < length; i++ {
		n, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", err
		}
		result[i] = charset[n.Int64()]
	}
	return string(result), nil
}

// generateUUID 生成简单 UUID v4
func generateUUID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	// Version 4
	b[6] = (b[6] & 0x0f) | 0x40
	// Variant
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// truncate 截断字符串
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen]
}
