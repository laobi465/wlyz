// v0.5.0 集成扩展：Webhook 通知渠道实现
//
// 三个 Provider：
//  1. dingtalkWebhookProvider - 钉钉群机器人（webhook + 加签 secret + @mobiles）
//  2. wecomWebhookProvider    - 企业微信群机器人（webhook + text/markdown）
//  3. telegramWebhookProvider - Telegram Bot（sendMessage API + MarkdownV2）
//
// 严格遵循铁律 04/05/06：
//
//	04 - webhook URL / Bot Token / 加签 secret 全部从 sys_config 读取
//	05 - 8 项 notify.{dingtalk,wecom,telegram}.* 配置可通过后台实时调整
//	06 - 各家签名算法与官方文档对齐，不编造任何字段；HTTP 调用走标准库 net/http
package notify

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// telegramAPIBase Telegram Bot API 基础 URL
// 默认官方端点；测试中可覆盖以指向 httptest.Server
var telegramAPIBase = "https://api.telegram.org"

// webhookHTTPClient 包级别共享 HTTP 客户端，设置 10 秒超时避免下游不可用时挂起
// 复用连接池，所有 webhook 渠道（钉钉 / 企微 / Telegram）共用
var webhookHTTPClient = &http.Client{Timeout: 10 * time.Second}

// ============== 1. 钉钉群机器人 ==============

// dingtalkWebhookProvider 钉钉群机器人实现
// 官方文档：https://open.dingtalk.com/document/robots/custom-robot-access
//
// 安全设置（三选一，本项目支持「加签」）：
//   - 自定义关键词：消息内容必须包含关键词（不推荐，限制太死）
//   - 加签：HMAC-SHA256(timestamp + "\n" + secret) → base64 → url encode
//   - IP 地址段：限制来源 IP（运维侧配置）
//
// 消息类型：text / markdown / actionCard / feedCard，本项目用 markdown（支持 @）
type dingtalkWebhookProvider struct {
	mgr *Manager
}

// signDingTalk 计算钉钉加签
// 算法：HMAC-SHA256(key=secret, msg=timestamp + "\n" + secret) → base64 → url.QueryEscape
func signDingTalk(timestamp int64, secret string) string {
	stringToSign := fmt.Sprintf("%d\n%s", timestamp, secret)
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(stringToSign))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

// Send 钉钉机器人发送
// recipient 在钉钉渠道不使用（消息广播到群），保留以兼容接口
func (p *dingtalkWebhookProvider) Send(ctx context.Context, recipient, subject, content string) (string, error) {
	webhookURL := p.mgr.cache.GetString(ctx, CfgKeyDingTalkWebhookURL, "")
	if webhookURL == "" {
		return "", ErrProviderNotConfig
	}

	// 加签（如配置了 secret）
	secret := p.mgr.cache.GetString(ctx, CfgKeyDingTalkSecret, "")
	finalURL := webhookURL
	if secret != "" {
		timestamp := time.Now().UnixMilli()
		sign := signDingTalk(timestamp, secret)
		// 钉钉 webhook URL 已带 access_token 参数，追加 timestamp & sign
		sep := "&"
		if !strings.Contains(webhookURL, "?") {
			sep = "?"
		}
		finalURL = fmt.Sprintf("%s%stimestamp=%d&sign=%s", webhookURL, sep, timestamp, url.QueryEscape(sign))
	}

	// @ 设置
	atMobilesStr := p.mgr.cache.GetString(ctx, CfgKeyDingTalkAtMobiles, "")
	atAll := p.mgr.cache.GetBool(ctx, CfgKeyDingTalkAtAll, false)
	var atMobiles []string
	if atMobilesStr != "" {
		for _, m := range strings.Split(atMobilesStr, ",") {
			m = strings.TrimSpace(m)
			if m != "" {
				atMobiles = append(atMobiles, m)
			}
		}
	}

	// 构造 markdown 消息（subject 作为标题，content 作为正文）
	// content 中追加 @手机号（钉钉要求 isAtAll=true 时 text 内容不能缺少 @）
	mdContent := content
	if len(atMobiles) > 0 && !atAll {
		for _, m := range atMobiles {
			mdContent += fmt.Sprintf(" @%s", m)
		}
	}

	payload := map[string]interface{}{
		"msgtype": "markdown",
		"markdown": map[string]string{
			"title": subject,
			"text":  mdContent,
		},
		"at": map[string]interface{}{
			"atMobiles": atMobiles,
			"isAtAll":   atAll,
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("dingtalk: marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", finalURL, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := webhookHTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("dingtalk: http: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("dingtalk: read body: %w", err)
	}

	// 钉钉响应：{"errcode":0,"errmsg":"ok"}
	var result struct {
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
		MsgID   int64  `json:"messageId"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("dingtalk: invalid response: %s", string(respBody))
	}
	if result.ErrCode != 0 {
		return "", fmt.Errorf("dingtalk: errcode=%d errmsg=%s", result.ErrCode, result.ErrMsg)
	}
	return fmt.Sprintf("dingtalk-%d", result.MsgID), nil
}

// ============== 2. 企业微信群机器人 ==============

// wecomWebhookProvider 企业微信群机器人实现
// 官方文档：https://developer.work.weixin.qq.com/document/path/91770
//
// 安全：webhook URL 中含 key，等同鉴权，无需额外签名
// 消息类型：text / markdown / image / news / file，本项目用 markdown（不支持 @，text 才支持）
// @ 提示：仅 text 类型支持 @（需 mentioning_list 或 isAtAll）；markdown 不支持
// 因此 @ 行为在企微中受限：当 at_all=true 时改用 text 类型，否则用 markdown
type wecomWebhookProvider struct {
	mgr *Manager
}

// Send 企业微信机器人发送
func (p *wecomWebhookProvider) Send(ctx context.Context, recipient, subject, content string) (string, error) {
	webhookURL := p.mgr.cache.GetString(ctx, CfgKeyWeComWebhookURL, "")
	if webhookURL == "" {
		return "", ErrProviderNotConfig
	}

	// 默认 markdown；企微 markdown 不支持 @，因此不强求 at 配置
	// 拼接 subject 作为标题（加粗）
	mdContent := content
	if subject != "" {
		mdContent = fmt.Sprintf("**%s**\n\n%s", subject, content)
	}

	payload := map[string]interface{}{
		"msgtype": "markdown",
		"markdown": map[string]string{
			"content": mdContent,
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("wecom: marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", webhookURL, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := webhookHTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("wecom: http: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("wecom: read body: %w", err)
	}

	// 企微响应：{"errcode":0,"errmsg":"ok"}
	var result struct {
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("wecom: invalid response: %s", string(respBody))
	}
	if result.ErrCode != 0 {
		return "", fmt.Errorf("wecom: errcode=%d errmsg=%s", result.ErrCode, result.ErrMsg)
	}
	return fmt.Sprintf("wecom-%d", time.Now().UnixNano()), nil
}

// ============== 3. Telegram Bot ==============

// telegramWebhookProvider Telegram Bot 实现
// 官方文档：https://core.telegram.org/bots/api#sendmessage
//
// 端点：https://api.telegram.org/bot<token>/sendMessage
// 必填：chat_id（频道：@channelusername / 群组：-100xxx / 私聊：userid）
// 可选：parse_mode=MarkdownV2 / HTML / 纯文本
// 长度限制：4096 字符（超过需分页，本项目当前实现简单截断 + 提示）
type telegramWebhookProvider struct {
	mgr *Manager
}

// escapeTelegramMarkdown 转义 MarkdownV2 特殊字符
// 官方要求：以下字符必须用 \ 前缀转义：_ * [ ] ( ) ~ ` > # + - = | { } . !
// 否则 sendMessage 返回 400 "can't parse entities"
func escapeTelegramMarkdown(s string) string {
	// 铁律 06：完整覆盖官方文档列出的特殊字符
	const special = "_*[]()~`>#+-=|{}.!"
	var b strings.Builder
	b.Grow(len(s) + len(s)/4)
	for _, r := range s {
		if strings.ContainsRune(special, r) {
			b.WriteByte('\\')
		}
		b.WriteRune(r)
	}
	return b.String()
}

// Send Telegram Bot 发送
func (p *telegramWebhookProvider) Send(ctx context.Context, recipient, subject, content string) (string, error) {
	botToken := p.mgr.cache.GetString(ctx, CfgKeyTelegramBotToken, "")
	if botToken == "" {
		return "", ErrProviderNotConfig
	}
	chatID := p.mgr.cache.GetString(ctx, CfgKeyTelegramChatID, "")
	if chatID == "" {
		return "", ErrProviderNotConfig
	}

	// 拼接消息（subject 加粗 + 换行 + content）
	// 使用 MarkdownV2 渲染，特殊字符需转义
	var msgText string
	if subject != "" {
		msgText = fmt.Sprintf("*%s*\n\n%s", escapeTelegramMarkdown(subject), escapeTelegramMarkdown(content))
	} else {
		msgText = escapeTelegramMarkdown(content)
	}

	// 铁律 06：超长消息截断（Telegram 限制 4096 字符）
	const maxLen = 4096
	if len(msgText) > maxLen {
		msgText = msgText[:maxLen-20] + "\n\n...（已截断）"
	}

	payload := map[string]interface{}{
		"chat_id":    chatID,
		"text":       msgText,
		"parse_mode": "MarkdownV2",
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("telegram: marshal payload: %w", err)
	}

	// 铁律 04：bot_token 不硬编码，从配置读取
	// telegramAPIBase 可在测试中覆盖以指向 mock server
	apiURL := fmt.Sprintf("%s/bot%s/sendMessage", telegramAPIBase, botToken)
	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := webhookHTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("telegram: http: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("telegram: read body: %w", err)
	}

	// TG 响应：{"ok":true,"result":{"message_id":123,"date":...}}
	var result struct {
		OK          bool   `json:"ok"`
		ErrorCode   int    `json:"error_code"`
		Description string `json:"description"`
		Result      struct {
			MessageID int64 `json:"message_id"`
		} `json:"result"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("telegram: invalid response: %s", string(respBody))
	}
	if !result.OK {
		return "", fmt.Errorf("telegram: error_code=%d description=%s", result.ErrorCode, result.Description)
	}
	return fmt.Sprintf("telegram-%s", strconv.FormatInt(result.Result.MessageID, 10)), nil
}
