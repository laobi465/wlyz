// Package notify v0.4.0 通知系统核心包
// 严格遵循铁律 04/05/06：
//   04 - 无硬编码：三通道开关 / 服务商密钥 / SMTP / 重试策略 / 限流 全部从 sys_config 读取
//   05 - 配置走后端：16 项 notify.* 配置可通过后台实时调整
//   06 - 反幻觉：模板变量替换用 strings.NewReplacer 安全替换（不用 text/template 防 SSTI）；测试覆盖正/负/边界全场景
//
// 核心能力：
//   1. Manager.Render - 模板变量替换（{{var}} → 实际值）
//   2. Manager.GetTemplate - 按代码 + 渠道 + 租户查询模板
//   3. Manager.Send - 同步发送（短信/邮件/站内信）
//   4. Manager.Enqueue - 异步发送（写日志后由 worker 处理）
//   5. Manager.SendByTemplateCode - 按模板代码发送（最常用入口）
//   6. Manager.ListLogs - 查询发送日志
//   7. Manager.Retry - 重试失败日志
//   8. SMSProvider / EmailProvider - 服务商接口（aliyun/tencent/smtp 实现）
package notify

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/smtp"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"gorm.io/gorm"

	"github.com/your-org/keyauth-saas/apps/server/internal/config"
	"github.com/your-org/keyauth-saas/apps/server/internal/model"
	"github.com/your-org/keyauth-saas/apps/server/pkg/crypto"
)

// ============== 常量 ==============

// 配置键常量（铁律 04：禁止硬编码配置键名）
const (
	CfgKeySMSEnabled            = "notify.sms.enabled"
	CfgKeySMSProvider           = "notify.sms.provider"
	CfgKeySMSAccessKeyID        = "notify.sms.access_key_id"
	CfgKeySMSAccessSecretEnc    = "notify.sms.access_secret_enc"
	CfgKeySMSSignName           = "notify.sms.sign_name"
	CfgKeySMSRegion             = "notify.sms.region"       // 默认 "cn-hangzhou"
	CfgKeySMSEndpoint           = "notify.sms.endpoint"     // 默认 "dysmsapi.aliyuncs.com"
	CfgKeyEmailEnabled          = "notify.email.enabled"
	CfgKeyEmailSMTPHost         = "notify.email.smtp_host"
	CfgKeyEmailSMTPPort         = "notify.email.smtp_port"
	CfgKeyEmailSMTPUsername     = "notify.email.smtp_username"
	CfgKeyEmailSMTPPasswordEnc  = "notify.email.smtp_password_enc"
	CfgKeyEmailFromAddress      = "notify.email.from_address"
	CfgKeyEmailFromName         = "notify.email.from_name"
	CfgKeyInAppEnabled          = "notify.inapp.enabled"
	CfgKeyRetryTimes            = "notify.retry.times"
	CfgKeyRetryIntervalSeconds  = "notify.retry.interval_seconds"
	CfgKeyRateLimitPerMinute    = "notify.rate_limit.per_minute"
	CfgKeyEmailSMTPEncryption     = "notify.email.smtp_encryption"       // 默认 "ssl"，可选 none/ssl/tls
	CfgKeyEmailSMTPTimeoutSeconds = "notify.email.smtp_timeout_seconds"  // 默认 10
)

// Channel 通知渠道
const (
	ChannelSMS   = "sms"
	ChannelEmail = "email"
	ChannelInApp = "inapp"
)

// TemplateCode 预置模板代码
const (
	TemplateVerifyCode       = "verify_code"
	TemplateVerifyCodeEmail  = "verify_code_email"
	TemplateOrderPaid        = "order_paid"
	TemplateAgentCommission  = "agent_commission"
	TemplatePayModeChanged   = "pay_mode_changed" // v0.4.x 收尾项 D：开发者切换支付配置时通知代理
)

// 配置键常量（v0.4.x 收尾项 D）
const (
	CfgKeyPayModeChangedEnabled = "notify.pay_mode_changed.enabled"
)

// TemplateStatus 模板状态
const (
	TemplateStatusEnabled  = "enabled"
	TemplateStatusDisabled = "disabled"
)

// LogStatus 日志状态
const (
	LogStatusPending = "pending"
	LogStatusSent    = "sent"
	LogStatusFailed  = "failed"
)

// Priority 优先级
const (
	PriorityNormal = 0
	PriorityHigh   = 1
	PriorityUrgent = 2
)

// 错误
var (
	ErrTemplateNotFound   = errors.New("notify: template not found")
	ErrTemplateDisabled   = errors.New("notify: template disabled")
	ErrChannelDisabled    = errors.New("notify: channel disabled")
	ErrRateLimitExceeded  = errors.New("notify: rate limit exceeded")
	ErrInvalidRecipient   = errors.New("notify: invalid recipient")
	ErrProviderNotConfig  = errors.New("notify: provider not configured")
)

// ============== 类型 ==============

// SendRequest 发送请求
type SendRequest struct {
	TemplateCode string                 // 模板代码（与 Channel 配合查找）
	Channel      string                 // 渠道：sms/email/inapp
	Recipient    string                 // 接收人：手机号/邮箱/user_id
	Variables    map[string]interface{} // 模板变量
	TenantID     uint64                 // 租户 ID（0=平台）
	Priority     int                    // 优先级
}

// SendResult 发送结果
type SendResult struct {
	LogID         uint64
	Status        string
	ProviderMsgID string
	ErrorMessage  string
}

// SMSProvider 短信服务商接口
type SMSProvider interface {
	Send(ctx context.Context, phone, signName, templateCode string, params map[string]interface{}) (msgID string, err error)
}

// EmailProvider 邮件服务商接口
type EmailProvider interface {
	Send(ctx context.Context, to, subject, htmlBody string) (msgID string, err error)
}

// Manager 通知管理器
type Manager struct {
	db          *gorm.DB
	cache       *config.ConfigCache
	crypto      *crypto.Manager
	smsProvider SMSProvider
	emailProvider EmailProvider
	mu          sync.Mutex
}

// NewManager 创建通知管理器
func NewManager(db *gorm.DB, cache *config.ConfigCache, cry *crypto.Manager) *Manager {
	m := &Manager{
		db:     db,
		cache:  cache,
		crypto: cry,
	}
	m.smsProvider = &aliyunSMSProvider{mgr: m}
	m.emailProvider = &smtpEmailProvider{mgr: m}
	return m
}

// ============== 1. 模板变量替换 ==============

// Render 模板变量替换（铁律 06：用 strings.NewReplacer 安全替换，不用 text/template 防 SSTI）
// 占位符格式：{{var}}，未提供的变量保留原占位符（便于排查）
func Render(template string, vars map[string]interface{}) string {
	if vars == nil || len(vars) == 0 {
		return template
	}
	pairs := make([]string, 0, len(vars)*2)
	for k, v := range vars {
		pairs = append(pairs, "{{"+k+"}}", fmt.Sprintf("%v", v))
	}
	return strings.NewReplacer(pairs...).Replace(template)
}

// ============== 2. 模板查询 ==============

// GetTemplate 按代码 + 渠道 + 租户查询模板
// 优先返回租户自定义模板，找不到则返回平台通用模板（tenant_id=0）
func (m *Manager) GetTemplate(ctx context.Context, code, channel string, tenantID uint64) (*model.NotifyTemplate, error) {
	// 1. 先查租户自定义模板
	var tmpl model.NotifyTemplate
	err := m.db.Where("code = ? AND channel = ? AND tenant_id = ? AND status = ?", code, channel, tenantID, TemplateStatusEnabled).
		First(&tmpl).Error
	if err == nil {
		return &tmpl, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	// 2. 回退平台通用模板
	err = m.db.Where("code = ? AND channel = ? AND tenant_id = 0 AND status = ?", code, channel, TemplateStatusEnabled).
		First(&tmpl).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrTemplateNotFound
		}
		return nil, err
	}
	return &tmpl, nil
}

// ListTemplates 列模板（admin/tenant 共用）
func (m *Manager) ListTemplates(ctx context.Context, tenantID uint64, channel string, page, pageSize int) ([]model.NotifyTemplate, int64, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}
	q := m.db.Model(&model.NotifyTemplate{})
	// 平台可看全部；租户只能看自己的 + 平台通用的
	if tenantID > 0 {
		q = q.Where("tenant_id = 0 OR tenant_id = ?", tenantID)
	}
	if channel != "" {
		q = q.Where("channel = ?", channel)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []model.NotifyTemplate
	if err := q.Order("id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// CreateTemplate / UpdateTemplate / DeleteTemplate 模板 CRUD
func (m *Manager) CreateTemplate(ctx context.Context, t *model.NotifyTemplate) error {
	return m.db.Create(t).Error
}

func (m *Manager) UpdateTemplate(ctx context.Context, id uint64, updates map[string]interface{}) error {
	return m.db.Model(&model.NotifyTemplate{}).Where("id = ?", id).Updates(updates).Error
}

func (m *Manager) DeleteTemplate(ctx context.Context, id uint64) error {
	return m.db.Delete(&model.NotifyTemplate{}, id).Error
}

// ============== 3. 通道开关检查 ==============

// IsChannelEnabled 检查通道是否启用
func (m *Manager) IsChannelEnabled(ctx context.Context, channel string) bool {
	switch channel {
	case ChannelSMS:
		return m.cache.GetBool(ctx, CfgKeySMSEnabled, false)
	case ChannelEmail:
		return m.cache.GetBool(ctx, CfgKeyEmailEnabled, false)
	case ChannelInApp:
		return m.cache.GetBool(ctx, CfgKeyInAppEnabled, true)
	}
	return false
}

// ============== 4. 限流检查 ==============

// CheckRateLimit 单租户每分钟限流检查
// 返回 true 表示通过，false 表示超限
func (m *Manager) CheckRateLimit(ctx context.Context, tenantID uint64) bool {
	limit := m.cache.GetInt(ctx, CfgKeyRateLimitPerMinute, 60)
	if limit <= 0 {
		return true
	}
	oneMinuteAgo := time.Now().Add(-time.Minute)
	var count int64
	m.db.Model(&model.NotifyLog{}).
		Where("tenant_id = ? AND created_at >= ?", tenantID, oneMinuteAgo).
		Count(&count)
	return count < int64(limit)
}

// ============== 5. 同步发送 ==============

// Send 同步发送通知
// 流程：① 检查通道开关 ② 限流 ③ 查模板 ④ 渲染变量 ⑤ 调用 provider ⑥ 写日志
func (m *Manager) Send(ctx context.Context, req SendRequest) (*SendResult, error) {
	// 1. 通道开关
	if !m.IsChannelEnabled(ctx, req.Channel) {
		return &SendResult{Status: LogStatusFailed, ErrorMessage: "channel disabled"}, ErrChannelDisabled
	}
	// 2. 限流
	if !m.CheckRateLimit(ctx, req.TenantID) {
		return &SendResult{Status: LogStatusFailed, ErrorMessage: "rate limit exceeded"}, ErrRateLimitExceeded
	}
	// 3. 接收人校验
	if req.Recipient == "" {
		return &SendResult{Status: LogStatusFailed, ErrorMessage: "empty recipient"}, ErrInvalidRecipient
	}
	// 4. 查模板
	tmpl, err := m.GetTemplate(ctx, req.TemplateCode, req.Channel, req.TenantID)
	if err != nil {
		return &SendResult{Status: LogStatusFailed, ErrorMessage: err.Error()}, err
	}
	// 5. 渲染变量
	subject := Render(tmpl.Subject, req.Variables)
	content := Render(tmpl.Content, req.Variables)
	// 6. 写 pending 日志
	logEntry := &model.NotifyLog{
		TemplateID:   tmpl.ID,
		TemplateCode: tmpl.Code,
		Channel:      req.Channel,
		Recipient:    req.Recipient,
		Subject:      subject,
		Content:      content,
		Status:       LogStatusPending,
		Priority:     req.Priority,
		TenantID:     req.TenantID,
	}
	if err := m.db.Create(logEntry).Error; err != nil {
		return &SendResult{Status: LogStatusFailed, ErrorMessage: err.Error()}, err
	}
	// 7. 调用 provider 发送
	result := m.dispatch(ctx, req.Channel, req.Recipient, subject, content, req.TemplateCode, req.Variables)
	// 8. 更新日志
	now := time.Now()
	updates := map[string]interface{}{
		"status":          result.Status,
		"provider_msg_id": result.ProviderMsgID,
		"error_message":   result.ErrorMessage,
	}
	if result.Status == LogStatusSent {
		updates["sent_at"] = &now
	}
	m.db.Model(&model.NotifyLog{}).Where("id = ?", logEntry.ID).Updates(updates)
	result.LogID = logEntry.ID
	return result, nil
}

// dispatch 根据 channel 调用对应 provider
func (m *Manager) dispatch(ctx context.Context, channel, recipient, subject, content, templateCode string, vars map[string]interface{}) *SendResult {
	switch channel {
	case ChannelSMS:
		provider := m.cache.GetString(ctx, CfgKeySMSProvider, "none")
		if provider == "none" {
			return &SendResult{Status: LogStatusFailed, ErrorMessage: "sms provider not configured"}
		}
		signName := m.cache.GetString(ctx, CfgKeySMSSignName, "")
		msgID, err := m.smsProvider.Send(ctx, recipient, signName, templateCode, vars)
		if err != nil {
			return &SendResult{Status: LogStatusFailed, ErrorMessage: err.Error()}
		}
		return &SendResult{Status: LogStatusSent, ProviderMsgID: msgID}
	case ChannelEmail:
		msgID, err := m.emailProvider.Send(ctx, recipient, subject, content)
		if err != nil {
			return &SendResult{Status: LogStatusFailed, ErrorMessage: err.Error()}
		}
		return &SendResult{Status: LogStatusSent, ProviderMsgID: msgID}
	case ChannelInApp:
		// 站内信直接写日志即视为发送成功（前端拉取日志展示）
		return &SendResult{Status: LogStatusSent, ProviderMsgID: fmt.Sprintf("inapp-%d", time.Now().UnixNano())}
	}
	return &SendResult{Status: LogStatusFailed, ErrorMessage: "unknown channel: " + channel}
}

// TestDispatch 暴露 dispatch 供测试发送 handler 使用（绕过模板查找）
func (m *Manager) TestDispatch(ctx context.Context, channel, recipient, subject, content string, vars map[string]interface{}) *SendResult {
	return m.dispatch(ctx, channel, recipient, subject, content, "test", vars)
}

// SetSMSProvider / SetEmailProvider 暴露注入点供测试 mock
func (m *Manager) SetSMSProvider(p SMSProvider) { m.smsProvider = p }
func (m *Manager) SetEmailProvider(p EmailProvider) { m.emailProvider = p }

// ============== 6. 日志查询 / 重试 ==============

// ListLogs 分页查询日志
func (m *Manager) ListLogs(ctx context.Context, tenantID uint64, channel, status string, page, pageSize int) ([]model.NotifyLog, int64, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}
	q := m.db.Model(&model.NotifyLog{})
	if tenantID > 0 {
		q = q.Where("tenant_id = ?", tenantID)
	}
	if channel != "" {
		q = q.Where("channel = ?", channel)
	}
	if status != "" {
		q = q.Where("status = ?", status)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []model.NotifyLog
	if err := q.Order("id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// GetLog 查询单条日志
func (m *Manager) GetLog(ctx context.Context, id uint64) (*model.NotifyLog, error) {
	var log model.NotifyLog
	if err := m.db.First(&log, id).Error; err != nil {
		return nil, err
	}
	return &log, nil
}

// Retry 重试失败日志
func (m *Manager) Retry(ctx context.Context, logID uint64) (*SendResult, error) {
	var log model.NotifyLog
	if err := m.db.First(&log, logID).Error; err != nil {
		return nil, err
	}
	if log.Status != LogStatusFailed {
		return &SendResult{LogID: log.ID, Status: log.Status, ErrorMessage: "only failed log can retry"}, nil
	}
	maxRetry := m.cache.GetInt(ctx, CfgKeyRetryTimes, 3)
	if log.RetryCount >= maxRetry {
		return &SendResult{LogID: log.ID, Status: LogStatusFailed, ErrorMessage: "max retry exceeded"}, nil
	}
	// 调用 provider 重发
	var vars map[string]interface{}
	// 注意：日志中不存储原始 vars，重试时仅用已渲染的 content 重新发送
	result := m.dispatch(ctx, log.Channel, log.Recipient, log.Subject, log.Content, log.TemplateCode, vars)
	now := time.Now()
	updates := map[string]interface{}{
		"status":          result.Status,
		"provider_msg_id": result.ProviderMsgID,
		"error_message":   result.ErrorMessage,
		"retry_count":     log.RetryCount + 1,
	}
	if result.Status == LogStatusSent {
		updates["sent_at"] = &now
	}
	if err := m.db.Model(&model.NotifyLog{}).Where("id = ?", logID).Updates(updates).Error; err != nil {
		return nil, fmt.Errorf("notify: update log failed: %w", err)
	}
	result.LogID = logID
	return result, nil
}

// ============== 7. 辅助：验证码生成 ==============

// GenerateVerifyCode 生成数字验证码
func GenerateVerifyCode(length int) string {
	if length <= 0 {
		length = 6
	}
	digits := "0123456789"
	code := make([]byte, length)
	max := big.NewInt(int64(len(digits)))
	for i := 0; i < length; i++ {
		n, err := rand.Int(rand.Reader, max)
		if err != nil {
			// 回退到时间戳（极小概率）
			code[i] = digits[time.Now().UnixNano()%int64(len(digits))]
			continue
		}
		code[i] = digits[n.Int64()]
	}
	return string(code)
}

// ============== 8. 短信服务商实现（阿里云） ==============

// aliyunSMSProvider 阿里云短信实现
// 铁律 06：实现真实 HTTP 调用签名规则；测试时通过 mock provider 替换
type aliyunSMSProvider struct {
	mgr *Manager
}

// signAliyunRequest 计算阿里云 RPC API 签名
// 算法：HMAC-SHA1(secret + "&", "POST&" + url.QueryEscape("/") + "&" + url.QueryEscape(sortedQueryString))
// 返回 Base64 编码的签名
func signAliyunRequest(params map[string]string, secret string) string {
	// 1. 按 key 字典序排序
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// 2. 拼接 sortedQueryString（key=value 用 & 连接，key/value 都 URL encode）
	var buf strings.Builder
	for i, k := range keys {
		if i > 0 {
			buf.WriteByte('&')
		}
		buf.WriteString(url.QueryEscape(k))
		buf.WriteByte('=')
		buf.WriteString(url.QueryEscape(params[k]))
	}
	sortedQueryString := buf.String()

	// 3. 构造 stringToSign: POST&%2F&<url-encoded sortedQueryString>
	stringToSign := "POST&" + url.QueryEscape("/") + "&" + url.QueryEscape(sortedQueryString)

	// 4. HMAC-SHA1(secret + "&", stringToSign)
	h := hmac.New(sha1.New, []byte(secret+"&"))
	h.Write([]byte(stringToSign))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

// Send 阿里云短信发送（v0.4.0 收尾项 A：完整签名 + HTTP POST）
// 铁律 06：实现真实阿里云 Dysms API 调用，零第三方依赖
func (p *aliyunSMSProvider) Send(ctx context.Context, phone, signName, templateCode string, params map[string]interface{}) (string, error) {
	accessKeyID := p.mgr.cache.GetString(ctx, CfgKeySMSAccessKeyID, "")
	if accessKeyID == "" {
		return "", ErrProviderNotConfig
	}
	accessSecretEnc := p.mgr.cache.GetString(ctx, CfgKeySMSAccessSecretEnc, "")
	if accessSecretEnc == "" {
		return "", ErrProviderNotConfig
	}
	// 解密 AccessSecret（铁律 04：密钥 AES 加密存储）
	accessSecret := accessSecretEnc
	if p.mgr.crypto != nil {
		if dec, err := p.mgr.crypto.DecryptAES(accessSecretEnc); err == nil {
			accessSecret = dec
		}
	}
	region := p.mgr.cache.GetString(ctx, CfgKeySMSRegion, "cn-hangzhou")
	endpoint := p.mgr.cache.GetString(ctx, CfgKeySMSEndpoint, "dysmsapi.aliyuncs.com")
	if signName == "" {
		signName = p.mgr.cache.GetString(ctx, CfgKeySMSSignName, "")
	}

	// 序列化模板参数
	templateParamJSON := "{}"
	if len(params) > 0 {
		if b, err := json.Marshal(params); err == nil {
			templateParamJSON = string(b)
		}
	}

	// 拼装公共参数 + 业务参数
	// SignatureNonce 复用时间戳生成器（Manager 未提供 nonce 生成器，使用 UnixNano）
	nonce := strconv.FormatInt(time.Now().UnixNano(), 10)
	reqParams := map[string]string{
		"SignatureMethod":  "HMAC-SHA1",
		"SignatureNonce":   nonce,
		"AccessKeyId":      accessKeyID,
		"SignatureVersion": "1.0",
		"Timestamp":        time.Now().UTC().Format("2006-01-02T15:04:05Z"),
		"Format":           "JSON",
		"Version":          "2017-05-25",
		"RegionId":         region,
		"Action":           "SendSms",
		"PhoneNumbers":     phone,
		"SignName":         signName,
		"TemplateCode":     templateCode,
		"TemplateParam":    templateParamJSON,
	}

	// 计算签名
	signature := signAliyunRequest(reqParams, accessSecret)
	reqParams["Signature"] = signature

	// 构造 POST form body
	form := url.Values{}
	for k, v := range reqParams {
		form.Set(k, v)
	}

	// 发起 HTTP POST
	httpReq, err := http.NewRequestWithContext(ctx, "POST", "https://"+endpoint+"/", strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	httpReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	// 解析响应
	var result struct {
		Code      string `json:"Code"`
		Message   string `json:"Message"`
		BizId     string `json:"BizId"`
		RequestId string `json:"RequestId"`
	}
	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		return "", fmt.Errorf("aliyun sms: invalid response: %s", string(bodyBytes))
	}
	if result.Code != "OK" {
		return "", fmt.Errorf("aliyun sms: %s: %s", result.Code, result.Message)
	}
	return result.BizId, nil
}

// ============== 9. 邮件服务商实现（SMTP） ==============

// smtpEmailProvider SMTP 邮件实现
type smtpEmailProvider struct {
	mgr *Manager
}

// dialSMTPClient 根据 encryption 模式建立 SMTP 客户端
// encryption: "ssl"=465 隐式 TLS / "tls"=587 STARTTLS / "none"=25 明文
// 零第三方依赖：仅用标准库 crypto/tls + net/smtp
func dialSMTPClient(host string, port int, encryption string, timeout time.Duration) (*smtp.Client, error) {
	// 用 net.JoinHostPort 而非 fmt.Sprintf("%s:%d")：正确处理 IPv6 主机（如 ::1）
	addr := net.JoinHostPort(host, strconv.Itoa(port))
	switch encryption {
	case "ssl":
		// 465 端口隐式 TLS（直接 TLS 握手）
		tlsConfig := &tls.Config{ServerName: host}
		conn, err := tls.DialWithDialer(&net.Dialer{Timeout: timeout}, "tcp", addr, tlsConfig)
		if err != nil {
			return nil, fmt.Errorf("smtp ssl dial: %w", err)
		}
		client, err := smtp.NewClient(conn, host)
		if err != nil {
			conn.Close()
			return nil, fmt.Errorf("smtp new client: %w", err)
		}
		return client, nil
	case "tls":
		// 587 端口 STARTTLS（先明文连接再升级）
		conn, err := net.DialTimeout("tcp", addr, timeout)
		if err != nil {
			return nil, fmt.Errorf("smtp dial: %w", err)
		}
		client, err := smtp.NewClient(conn, host)
		if err != nil {
			conn.Close()
			return nil, fmt.Errorf("smtp new client: %w", err)
		}
		if err := client.StartTLS(&tls.Config{ServerName: host}); err != nil {
			client.Close()
			return nil, fmt.Errorf("smtp starttls: %w", err)
		}
		return client, nil
	default:
		// 25 端口明文（不推荐生产使用）
		conn, err := net.DialTimeout("tcp", addr, timeout)
		if err != nil {
			return nil, fmt.Errorf("smtp dial: %w", err)
		}
		client, err := smtp.NewClient(conn, host)
		if err != nil {
			conn.Close()
			return nil, fmt.Errorf("smtp new client: %w", err)
		}
		return client, nil
	}
}

// Send 通过 SMTP 发送邮件
// 支持 465 隐式 TLS / 587 STARTTLS / 25 明文三种加密模式
func (p *smtpEmailProvider) Send(ctx context.Context, to, subject, htmlBody string) (string, error) {
	host := p.mgr.cache.GetString(ctx, CfgKeyEmailSMTPHost, "")
	if host == "" {
		return "", ErrProviderNotConfig
	}
	port := p.mgr.cache.GetInt(ctx, CfgKeyEmailSMTPPort, 465)
	username := p.mgr.cache.GetString(ctx, CfgKeyEmailSMTPUsername, "")
	passwordEnc := p.mgr.cache.GetString(ctx, CfgKeyEmailSMTPPasswordEnc, "")
	fromAddr := p.mgr.cache.GetString(ctx, CfgKeyEmailFromAddress, "")
	fromName := p.mgr.cache.GetString(ctx, CfgKeyEmailFromName, "KeyAuth SaaS")
	if username == "" || fromAddr == "" {
		return "", ErrProviderNotConfig
	}
	encryption := p.mgr.cache.GetString(ctx, CfgKeyEmailSMTPEncryption, "ssl")
	timeoutSec := p.mgr.cache.GetInt(ctx, CfgKeyEmailSMTPTimeoutSeconds, 10)

	// 解密 SMTP 密码（铁律 04：密钥 AES 加密存储）
	password := passwordEnc
	if p.mgr.crypto != nil && passwordEnc != "" {
		if dec, err := p.mgr.crypto.DecryptAES(passwordEnc); err == nil {
			password = dec
		}
	}

	// 构造邮件
	msgID := fmt.Sprintf("<%d.keyauth@%s>", time.Now().UnixNano(), strings.Split(fromAddr, "@")[0])
	headers := fmt.Sprintf("From: %s <%s>\r\nTo: %s\r\nSubject: %s\r\nMessage-ID: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/html; charset=UTF-8\r\n\r\n",
		fromName, fromAddr, to, subject, msgID)
	body := headers + htmlBody

	// 向后兼容：未配置 encryption 时按 port 推断（465=ssl / 587=tls / 其他=none）
	if encryption == "" {
		switch {
		case port == 465:
			encryption = "ssl"
		case port == 587:
			encryption = "tls"
		default:
			encryption = "none"
		}
	}

	timeout := time.Duration(timeoutSec) * time.Second

	// 建立 SMTP 连接
	client, err := dialSMTPClient(host, port, encryption, timeout)
	if err != nil {
		return "", err
	}
	defer client.Close()

	// SMTP 认证
	if password != "" {
		auth := smtp.PlainAuth("", username, password, host)
		if err := client.Auth(auth); err != nil {
			return "", fmt.Errorf("smtp auth: %w", err)
		}
	}

	// 设置发件人
	if err := client.Mail(fromAddr); err != nil {
		return "", fmt.Errorf("smtp mail: %w", err)
	}

	// 设置收件人
	if err := client.Rcpt(to); err != nil {
		return "", fmt.Errorf("smtp rcpt: %w", err)
	}

	// 写入邮件正文
	w, err := client.Data()
	if err != nil {
		return "", fmt.Errorf("smtp data: %w", err)
	}
	if _, err := w.Write([]byte(body)); err != nil {
		return "", fmt.Errorf("smtp write: %w", err)
	}
	if err := w.Close(); err != nil {
		return "", fmt.Errorf("smtp close: %w", err)
	}

	return msgID, nil
}

// ============== 10. 统计 ==============

// Stats 通知统计
type Stats struct {
	Total      int64 `json:"total"`
	Sent       int64 `json:"sent"`
	Failed     int64 `json:"failed"`
	Pending    int64 `json:"pending"`
	SMSCount   int64 `json:"sms_count"`
	EmailCount int64 `json:"email_count"`
	InAppCount int64 `json:"inapp_count"`
}

// GetStats 统计
// 铁律 06：每次 Count 用新 session 避免Where 累积污染（GORM 已知陷阱）
func (m *Manager) GetStats(ctx context.Context, tenantID uint64) (*Stats, error) {
	stats := &Stats{}
	baseWhere := func(q *gorm.DB) *gorm.DB {
		if tenantID > 0 {
			return q.Where("tenant_id = ?", tenantID)
		}
		return q
	}
	baseWhere(m.db.Model(&model.NotifyLog{})).Count(&stats.Total)
	baseWhere(m.db.Model(&model.NotifyLog{})).Where("status = ?", LogStatusSent).Count(&stats.Sent)
	baseWhere(m.db.Model(&model.NotifyLog{})).Where("status = ?", LogStatusFailed).Count(&stats.Failed)
	baseWhere(m.db.Model(&model.NotifyLog{})).Where("status = ?", LogStatusPending).Count(&stats.Pending)
	baseWhere(m.db.Model(&model.NotifyLog{})).Where("channel = ?", ChannelSMS).Count(&stats.SMSCount)
	baseWhere(m.db.Model(&model.NotifyLog{})).Where("channel = ?", ChannelEmail).Count(&stats.EmailCount)
	baseWhere(m.db.Model(&model.NotifyLog{})).Where("channel = ?", ChannelInApp).Count(&stats.InAppCount)
	return stats, nil
}

// ============== 11. 辅助函数 ==============

// ParseVariables 解析模板变量 JSON
func ParseVariables(varsJSON string) []string {
	if varsJSON == "" || varsJSON == "[]" {
		return []string{}
	}
	var vars []string
	if err := json.Unmarshal([]byte(varsJSON), &vars); err != nil {
		return []string{}
	}
	return vars
}

// ValidateChannel 校验渠道
func ValidateChannel(channel string) bool {
	return channel == ChannelSMS || channel == ChannelEmail || channel == ChannelInApp
}

// ============== 12. v0.4.x 收尾项 D：支付方式变更通知代理 ==============

// NotifyAgentsByTenant 给指定开发者名下所有启用代理发送支付方式变更通知
// 流程：① 读 sys_config 开关 ② 查启用代理 ③ 渲染模板 ④ 创建 Notice ⑤ 创建 NoticeTarget ⑥ 批量创建 notify_log
//
// 创建产物：
//   - 1 条 Notice（type=agent_notify, status=published, show_badge=true）
//   - 1 条 NoticeTarget（target_type=all_agents, target_id=tenantID）
//   - N 条 notify_log（每个代理一条，channel=inapp, status=sent, priority=high）
//
// 通知失败不阻塞主流程，调用方应自行处理返回的 error（通常仅记录日志）。
// 严格遵循铁律 04：配置开关 notify.pay_mode_changed.enabled 走 sys_config，无硬编码
func NotifyAgentsByTenant(ctx context.Context, db *gorm.DB, tenantID uint64, variables map[string]interface{}) error {
	// 1. 读配置开关 notify.pay_mode_changed.enabled
	//    显式 "0" 才跳过；未配置或其它值均视为启用（与 migration 默认 '1' 一致）
	var cfgValue string
	db.Raw("SELECT config_value FROM sys_config WHERE config_key = ? LIMIT 1", CfgKeyPayModeChangedEnabled).Scan(&cfgValue)
	if cfgValue == "0" {
		return nil
	}

	// 2. 查询该开发者名下所有启用代理
	var agents []model.Agent
	if err := db.Where("tenant_id = ? AND status = ?", tenantID, "active").Find(&agents).Error; err != nil {
		return err
	}
	if len(agents) == 0 {
		return nil
	}

	// 3. 提取变量（用于 Notice 标题/正文渲染）
	tenantName := ""
	if v, ok := variables["tenant_name"].(string); ok {
		tenantName = v
	}
	channelStr := ""
	if v, ok := variables["channel"].(string); ok {
		channelStr = v
	}
	actionStr := ""
	if v, ok := variables["action"].(string); ok {
		actionStr = v
	}
	timeStr := ""
	if v, ok := variables["time"].(string); ok {
		timeStr = v
	}

	// 4. 渲染通知正文（优先用 notify_template 平台模板，找不到则用兜底文案）
	subject := "支付方式变更通知"
	content := fmt.Sprintf("开发者 %s 的支付通道 %s 已%s，请知悉。", tenantName, channelStr, actionStr)
	if timeStr != "" {
		content = fmt.Sprintf("开发者 %s 的支付通道 %s 已%s（%s），请知悉。如有疑问请联系开发者。", tenantName, channelStr, actionStr, timeStr)
	}
	var tmpl model.NotifyTemplate
	if err := db.Where("code = ? AND channel = ? AND tenant_id = 0 AND status = ?", TemplatePayModeChanged, ChannelInApp, TemplateStatusEnabled).First(&tmpl).Error; err == nil {
		subject = Render(tmpl.Subject, variables)
		content = Render(tmpl.Content, variables)
	}

	// 5. 创建 Notice 记录（type=agent_notify, status=published, show_badge=true）
	notice := model.Notice{
		Type:          "agent_notify",
		TenantID:      &tenantID,
		Title:         fmt.Sprintf("支付方式变更通知 - %s", channelStr),
		Content:       content,
		ContentFormat: "text",
		IsPinned:      false,
		IsPopup:       false,
		ShowBadge:     true,
		StartAt:       time.Now(),
		Status:        "published",
		CreatedBy:     0, // 系统创建
	}
	if err := db.Create(&notice).Error; err != nil {
		return err
	}

	// 6. 创建 NoticeTarget（target_type=all_agents, target_id=tenantID）
	target := model.NoticeTarget{
		NoticeID:   notice.ID,
		TargetType: "all_agents",
		TargetID:   &tenantID,
	}
	if err := db.Create(&target).Error; err != nil {
		return err
	}

	// 7. 批量创建 notify_log（每个代理一条，channel=inapp, status=sent, priority=high）
	logs := make([]model.NotifyLog, 0, len(agents))
	now := time.Now()
	for _, agent := range agents {
		logs = append(logs, model.NotifyLog{
			TenantID:     tenantID,
			Channel:      ChannelInApp,
			Recipient:    fmt.Sprintf("%d", agent.ID),
			TemplateCode: TemplatePayModeChanged,
			Subject:      subject,
			Content:      content,
			Status:       LogStatusSent,
			Priority:     PriorityHigh,
			SentAt:       &now,
			CreatedAt:    now,
		})
	}
	if err := db.CreateInBatches(logs, 100).Error; err != nil {
		return err
	}

	return nil
}
