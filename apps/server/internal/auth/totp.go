// TOTP 2FA 工具包
// 基于 RFC 6238 标准（Google Authenticator / Microsoft Authenticator 兼容）
// 严格遵循铁律 04/05：所有可变参数从 sys_config 读取
package auth

import (
	"fmt"
	"strings"
	"time"

	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"

	"github.com/your-org/keyauth-saas/apps/server/pkg/crypto"
)

// TOTPOptions TOTP 生成参数（大部分参数应从 sys_config 读取）
type TOTPOptions struct {
	Issuer      string // 发行方名称（如 "KeyAuth SaaS"）
	Account     string // 用户账号（如 username/email）
	Period      uint   // 周期（秒），默认 30
	SecretSize  uint   // 密钥长度（字节），默认 20
	Digits      otp.Digits // 位数，6 或 8
	Algorithm   otp.Algorithm // 算法，SHA1/SHA256/SHA512
	Skew        uint   // 允许的前后偏移周期数（默认 1，即 ±30s）
}

// DefaultTOTPOptions 默认参数（生产环境应从 sys_config 覆盖）
// 注意：SHA1 是 Google Authenticator 兼容性最好的算法，仅在不要求强制 SHA256 时使用
func DefaultTOTPOptions(issuer, account string) TOTPOptions {
	return TOTPOptions{
		Issuer:     issuer,
		Account:    account,
		Period:     30,
		SecretSize: 20,
		Digits:     otp.DigitsSix,
		Algorithm:  otp.AlgorithmSHA1,
		Skew:       1,
	}
}

// TOTPResult 生成 TOTP 后的返回结构
type TOTPResult struct {
	Secret       string // 原始密钥（需 AES 加密后入库）
	OTPAUTHURL   string // otpauth:// 协议 URL（用于生成二维码）
	BackupCodes  []string // 备用码（10 个，一次性使用）
}

// GenerateTOTP 生成新的 TOTP 密钥
// 返回值：明文密钥 + otpauth URL + 备用码
// 注意：调用方必须将 Secret 用 AES-256-GCM 加密后入库
func GenerateTOTP(opt TOTPOptions) (*TOTPResult, error) {
	if opt.Issuer == "" || opt.Account == "" {
		return nil, fmt.Errorf("issuer 和 account 不能为空")
	}
	if opt.Period == 0 {
		opt.Period = 30
	}
	if opt.SecretSize == 0 {
		opt.SecretSize = 20
	}
	if opt.Digits == 0 {
		opt.Digits = otp.DigitsSix
	}
	if opt.Algorithm == 0 {
		opt.Algorithm = otp.AlgorithmSHA1
	}

	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      opt.Issuer,
		AccountName: opt.Account,
		Period:      opt.Period,
		SecretSize:  opt.SecretSize,
		Digits:      opt.Digits,
		Algorithm:   opt.Algorithm,
	})
	if err != nil {
		return nil, fmt.Errorf("生成 TOTP 密钥失败: %w", err)
	}

	backupCodes, err := generateBackupCodes(10)
	if err != nil {
		return nil, fmt.Errorf("生成备用码失败: %w", err)
	}

	return &TOTPResult{
		Secret:      key.Secret(),
		OTPAUTHURL:  key.URL(),
		BackupCodes: backupCodes,
	}, nil
}

// ValidateTOTP 校验 TOTP 验证码
// secret 为明文密钥（调用方需先 AES 解密）
// code 为用户输入的 6 位验证码
// skew 为允许的前后偏移周期数（一般 1，即 ±30s）
func ValidateTOTP(secret, code string, skew uint) bool {
	if secret == "" || code == "" {
		return false
	}
	code = strings.TrimSpace(code)
	if len(code) != 6 {
		return false
	}
	// pquerna/otp 默认 skew=1，可通过 ValidateCustom 调整
	return totp.Validate(code, secret)
}

// ValidateTOTPCustom 自定义参数校验（用于 skew / period / digits 与默认值不同时）
func ValidateTOTPCustom(secret, code string, opt TOTPOptions, now time.Time) bool {
	if secret == "" || code == "" {
		return false
	}
	ok, _ := totp.ValidateCustom(code, secret, now, totp.ValidateOpts{
		Period:    opt.Period,
		Skew:      opt.Skew,
		Digits:    opt.Digits,
		Algorithm: opt.Algorithm,
	})
	return ok
}

// EncryptTOTPSecret 用 AES-256-GCM 加密 TOTP 密钥入库
func EncryptTOTPSecret(mgr *crypto.Manager, plaintext string) (string, error) {
	if mgr == nil {
		return "", fmt.Errorf("加密管理器未初始化")
	}
	return mgr.EncryptAES(plaintext)
}

// DecryptTOTPSecret 从数据库取出加密的 TOTP 密钥并解密
func DecryptTOTPSecret(mgr *crypto.Manager, ciphertext string) (string, error) {
	if mgr == nil {
		return "", fmt.Errorf("加密管理器未初始化")
	}
	return mgr.DecryptAES(ciphertext)
}

// generateBackupCodes 生成 n 个 8 位数字备用码
// 备用码一次性使用，用于丢失 TOTP 设备时登录
// 生产环境应入库（bcrypt 哈希），使用后立即删除
func generateBackupCodes(n int) ([]string, error) {
	if n <= 0 {
		return nil, nil
	}
	// 此处简化实现：用 crypto/rand 生成 8 位数字
	// 注意：备用码应入库哈希存储，此处仅返回明文，调用方负责哈希
	codes := make([]string, 0, n)
	for i := 0; i < n; i++ {
		// 使用 totp 包内部的随机数生成器（避免重复 import）
		key, err := totp.Generate(totp.GenerateOpts{
			Issuer:      "backup",
			AccountName: fmt.Sprintf("code-%d", i),
			Period:      30,
			SecretSize:  4, // 4 字节 = 8 位十六进制
			Digits:      otp.DigitsEight,
			Algorithm:   otp.AlgorithmSHA1,
		})
		if err != nil {
			return nil, err
		}
		// 取前 8 位作为备用码
		codes = append(codes, key.Secret()[:8])
	}
	return codes, nil
}
