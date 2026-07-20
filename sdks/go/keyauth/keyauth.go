// Package keyauth KeyAuth SaaS Go 客户端 SDK
//
// 面向终端软件的客户端 SDK，封装 9 个验证 API：
//
//	Login / Verify / Heartbeat / Bind / Unbind / GetVar / Notice / Version / Logout
//
// 依赖：仅 Go 标准库（net/http / crypto/sha512 / encoding/json / time）
//
// 签名算法（与后端 internal/middleware/signature.go 一致）：
//	原文 = METHOD\nPATH?QUERY\nTIMESTAMP\nNONCE\nBODY
//	签名 = HMAC-SHA512/256(secret, 原文) → 64 位小写 hex
//	注：Go 标准库 crypto/sha512 原生提供 New512_256 变体，与后端完全一致，无需回退
//
// 铁律 04：API 地址 / AppKey / SignSecret 由调用方传入，SDK 内不硬编码
// 铁律 06：所有接口错误返回 *KeyAuthError(code, message)，不静默吞异常
package keyauth

import (
	"bytes"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Version SDK 版本
const Version = "0.4.0"

// KeyAuthError KeyAuth API 错误（含 code 与 message）
type KeyAuthError struct {
	Code      int
	Message   string
	HTTPStatus int
}

func (e *KeyAuthError) Error() string {
	return fmt.Sprintf("[%d] %s", e.Code, e.Message)
}

// CardInfo 卡密信息（Login/Verify 返回）
type CardInfo struct {
	Type             string `json:"type"`
	Status           string `json:"status"`
	ExpiresAt        *int64 `json:"expires_at"`
	RemainingSeconds int64  `json:"remaining_seconds"`
	BoundDevices     int    `json:"bound_devices"`
	MaxDevices       int    `json:"max_devices"`
	UsedCount        int    `json:"used_count"`
	MaxUses          int    `json:"max_uses"`
}

// DeviceInfo 设备信息（Login 返回）
type DeviceInfo struct {
	ID      uint64 `json:"id"`
	HWID    string `json:"hwid"`
	Name    string `json:"name"`
	BoundAt int64  `json:"bound_at"`
}

// Client KeyAuth SaaS 客户端 SDK
//
// 用法：
//
//	c := keyauth.NewClient("https://yourdomain.com", "ak_xxx", "sk_xxx")
//	result, err := c.Login("ABCD-1234-EFGH-5678", "cpu-mac-disk-hash")
type Client struct {
	apiBase    string
	appKey     string
	signSecret string
	httpClient *http.Client
}

// NewClient 构造客户端
//
// apiBase 后端 API 根地址（如 https://yourdomain.com）
// appKey 应用 AppKey（ak_ 开头）
// signSecret 应用 SignSecret（sk_ 开头，AES 解密后的明文）
func NewClient(apiBase, appKey, signSecret string) *Client {
	return &Client{
		apiBase:    strings.TrimRight(apiBase, "/"),
		appKey:     appKey,
		signSecret: signSecret,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// SetTimeout 设置 HTTP 请求超时
func (c *Client) SetTimeout(d time.Duration) {
	c.httpClient.Timeout = d
}

// ============== 公共 API ==============

// LoginResult Login 返回
type LoginResult struct {
	Token             string      `json:"token"`
	ExpiresAt         int64       `json:"expires_at"`
	Card              CardInfo    `json:"card"`
	Device            DeviceInfo  `json:"device"`
	HeartbeatInterval int         `json:"heartbeat_interval"`
	HeartbeatTimeout  int         `json:"heartbeat_timeout"`
}

// Login 登录（首次自动绑定设备）
func (c *Client) Login(cardKey, hwid string, deviceName, deviceType string) (*LoginResult, error) {
	payload := map[string]string{
		"card_key":    cardKey,
		"hwid":        hwid,
		"device_name": deviceName,
		"device_type": deviceType,
	}
	var out LoginResult
	return &out, c.post("/api/v1/client/login", payload, &out)
}

// VerifyResult Verify 返回
type VerifyResult struct {
	Card              CardInfo   `json:"card"`
	Device            DeviceInfo `json:"device"`
	LastHeartbeatAt   int64      `json:"last_heartbeat_at"`
	HeartbeatInterval int        `json:"heartbeat_interval"`
	HeartbeatTimeout  int        `json:"heartbeat_timeout"`
}

// Verify 验证卡密有效性（不绑定，不增加使用次数）
func (c *Client) Verify(cardKey, hwid string) (*VerifyResult, error) {
	payload := map[string]string{"card_key": cardKey, "hwid": hwid}
	var out VerifyResult
	return &out, c.post("/api/v1/client/verify", payload, &out)
}

// HeartbeatResult Heartbeat 返回
type HeartbeatResult struct {
	NextHeartbeat    int64 `json:"next_heartbeat"`
	HeartbeatTimeout int   `json:"heartbeat_timeout"`
	ServerTime       int64 `json:"server_time"`
}

// Heartbeat 心跳保活（按 heartbeat_interval 周期调用）
func (c *Client) Heartbeat(cardKey, hwid string) (*HeartbeatResult, error) {
	payload := map[string]string{"card_key": cardKey, "hwid": hwid}
	var out HeartbeatResult
	return &out, c.post("/api/v1/client/heartbeat", payload, &out)
}

// BindResult Bind 返回
type BindResult struct {
	DeviceID   uint64 `json:"device_id"`
	BoundAt    int64  `json:"bound_at"`
	BoundCount int    `json:"bound_count"`
	MaxDevices int    `json:"max_devices"`
}

// Bind 手动绑定设备（MaxDevices > 1 多机场景）
func (c *Client) Bind(cardKey, hwid, deviceName, deviceType string) (*BindResult, error) {
	payload := map[string]string{
		"card_key":    cardKey,
		"hwid":        hwid,
		"device_name": deviceName,
		"device_type": deviceType,
	}
	var out BindResult
	return &out, c.post("/api/v1/client/bind", payload, &out)
}

// UnbindResult Unbind 返回
type UnbindResult struct {
	Unbound         bool  `json:"unbound"`
	DeductedSeconds int64 `json:"deducted_seconds"`
	Message         string `json:"message"`
}

// Unbind 解绑设备（扣时 UnbindDeductSeconds）
func (c *Client) Unbind(cardKey, hwid string) (*UnbindResult, error) {
	payload := map[string]string{"card_key": cardKey, "hwid": hwid}
	var out UnbindResult
	return &out, c.post("/api/v1/client/unbind", payload, &out)
}

// GetVarResult GetVar 返回
type GetVarResult struct {
	VarKey    string `json:"var_key"`
	VarValue  string `json:"var_value"`
	VarType   string `json:"var_type"`
	UpdatedAt int64  `json:"updated_at"`
}

// GetVar 获取云变量
func (c *Client) GetVar(cardKey, varKey string) (*GetVarResult, error) {
	payload := map[string]string{"card_key": cardKey, "var_key": varKey}
	var out GetVarResult
	return &out, c.post("/api/v1/client/get_var", payload, &out)
}

// NoticeItem 公告条目
type NoticeItem struct {
	ID        uint64 `json:"id"`
	Title     string `json:"title"`
	Content   string `json:"content"`
	IsPinned  bool   `json:"is_pinned"`
	CreatedAt int64  `json:"created_at"`
}

// NoticeResult Notice 返回
type NoticeResult struct {
	Notices []NoticeItem `json:"notices"`
}

// Notice 获取应用公告
func (c *Client) Notice() (*NoticeResult, error) {
	var out NoticeResult
	return &out, c.post("/api/v1/client/notice", map[string]string{}, &out)
}

// VersionResult Version 返回
type VersionResult struct {
	HasUpdate         bool   `json:"has_update"`
	ForceUpdate       bool   `json:"force_update"`
	LatestVersion     string `json:"latest_version"`
	CurrentVersion    string `json:"current_version"`
	DownloadURL       string `json:"download_url"`
	BackupURL         string `json:"backup_url"`
	UpdateDescription string `json:"update_description"`
	MinVersion        string `json:"min_version"`
	ReleasedAt        int64  `json:"released_at"`
}

// Version 检查版本更新
func (c *Client) Version(currentVersion, platform string) (*VersionResult, error) {
	payload := map[string]string{
		"current_version": currentVersion,
		"platform":        platform,
	}
	var out VersionResult
	return &out, c.post("/api/v1/client/version", payload, &out)
}

// LogoutResult Logout 返回
type LogoutResult struct {
	LoggedOut bool `json:"logged_out"`
}

// Logout 退出登录（仅记录日志，不影响设备绑定状态）
func (c *Client) Logout(cardKey, hwid string) (*LogoutResult, error) {
	payload := map[string]string{"card_key": cardKey, "hwid": hwid}
	var out LogoutResult
	return &out, c.post("/api/v1/client/logout", payload, &out)
}

// ============== 内部方法 ==============

// post 发送带签名的 POST 请求，自动校验 code=0 并把 data 反序列化到 out
func (c *Client) post(path string, payload interface{}, out interface{}) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return &KeyAuthError{Code: 1006, Message: "JSON 编码失败: " + err.Error()}
	}

	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	nonce, err := randomNonce()
	if err != nil {
		return &KeyAuthError{Code: 1006, Message: "生成 nonce 失败: " + err.Error()}
	}

	// 签名原文：METHOD\nPATH?QUERY\nTIMESTAMP\nNONCE\nBODY（与后端 signature.go:88 一致）
	signString := strings.Join([]string{"POST", path, timestamp, nonce, string(body)}, "\n")
	signature := hmacSHA512_256Hex(c.signSecret, signString)

	url := c.apiBase + path
	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return &KeyAuthError{Code: 1006, Message: "构造请求失败: " + err.Error()}
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-App-Key", c.appKey)
	req.Header.Set("X-Timestamp", timestamp)
	req.Header.Set("X-Nonce", nonce)
	req.Header.Set("X-Signature", signature)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return &KeyAuthError{Code: 1006, Message: "网络请求失败: " + err.Error()}
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return &KeyAuthError{Code: 1006, Message: "读取响应失败: " + err.Error()}
	}

	var envelope struct {
		Code    int             `json:"code"`
		Message string          `json:"message"`
		Data    json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return &KeyAuthError{
			Code:       1006,
			Message:    "响应非 JSON: " + truncate(string(raw), 200),
			HTTPStatus: resp.StatusCode,
		}
	}

	if resp.StatusCode != 200 || envelope.Code != 0 {
		return &KeyAuthError{
			Code:       envelope.Code,
			Message:    envelope.Message,
			HTTPStatus: resp.StatusCode,
		}
	}

	if out != nil && len(envelope.Data) > 0 {
		if err := json.Unmarshal(envelope.Data, out); err != nil {
			return &KeyAuthError{Code: 1006, Message: "解析 data 字段失败: " + err.Error()}
		}
	}
	return nil
}

// hmacSHA512_256Hex HMAC-SHA512/256 → 64 位小写 hex（与后端 crypto.HMACSHA256 完全一致）
// Go 标准库 crypto/sha512 原生提供 New512_256，无需任何回退
func hmacSHA512_256Hex(secret, msg string) string {
	mac := hmac.New(sha512.New512_256, []byte(secret))
	mac.Write([]byte(msg))
	return hex.EncodeToString(mac.Sum(nil))
}

// randomNonce 生成 16 字节随机 hex nonce
func randomNonce() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// truncate 字符串截断（错误信息显示用）
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

// ErrEmptyData 后端 data 字段为空
var ErrEmptyData = errors.New("empty data")

// Sign 计算签名（供 sdks/tests/sign.go 调用，与后端 HMACSHA256 对齐）
// 导出函数，便于其他模块复用签名逻辑
func Sign(secret, msg string) string {
	return hmacSHA512_256Hex(secret, msg)
}
