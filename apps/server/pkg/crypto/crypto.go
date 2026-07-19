// Package crypto 加密工具集
// 包含 AES-256-GCM / RSA-4096 / HMAC-SHA256 / bcrypt / 卡密生成器
// 所有敏感字段加密 / 签名校验均经过此包
package crypto

import (
	"crypto"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha512"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

// Manager 加密管理器（持有 AES 密钥 + RSA 密钥对）
type Manager struct {
	aesKey     []byte
	privateKey *rsa.PrivateKey
	publicKey  *rsa.PublicKey
}

// NewManager 构造加密管理器
// aesKey 必须 32 字节（AES-256-GCM）
func NewManager(aesKey, rsaPrivateKeyPath, rsaPublicKeyPath string) (*Manager, error) {
	if len(aesKey) != 32 {
		return nil, fmt.Errorf("AES 密钥必须 32 字节，当前 %d 字节", len(aesKey))
	}
	m := &Manager{aesKey: []byte(aesKey)}

	// 加载 RSA 私钥（可选，未配置时跳过）
	if rsaPrivateKeyPath != "" {
		if _, err := os.Stat(rsaPrivateKeyPath); err == nil {
			priv, err := loadPrivateKey(rsaPrivateKeyPath)
			if err != nil {
				return nil, fmt.Errorf("加载 RSA 私钥失败: %w", err)
			}
			m.privateKey = priv
			m.publicKey = &priv.PublicKey
		}
	}

	// 单独加载公钥（如配置了）
	if rsaPublicKeyPath != "" && m.publicKey == nil {
		if _, err := os.Stat(rsaPublicKeyPath); err == nil {
			pub, err := loadPublicKey(rsaPublicKeyPath)
			if err != nil {
				return nil, fmt.Errorf("加载 RSA 公钥失败: %w", err)
			}
			m.publicKey = pub
		}
	}

	return m, nil
}

// ============== AES-256-GCM ==============

// EncryptAES 对称加密（用于敏感字段如商户密钥、AppSecret 等）
func (m *Manager) EncryptAES(plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}
	block, err := aes.NewCipher(m.aesKey)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	// 拼接 nonce + 密文
	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// DecryptAES 对称解密
func (m *Manager) DecryptAES(ciphertext string) (string, error) {
	if ciphertext == "" {
		return "", nil
	}
	data, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(m.aesKey)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(data) < gcm.NonceSize() {
		return "", errors.New("密文长度不足")
	}
	nonce := data[:gcm.NonceSize()]
	ciphertext = data[gcm.NonceSize():]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}

// ============== RSA-4096 响应签名 ==============

// SignWithRSA 用 RSA-4096 + SHA-512 签名
// 用于服务端响应签名（防伪造服务端）
func (m *Manager) SignWithRSA(data []byte) (string, error) {
	if m.privateKey == nil {
		return "", errors.New("RSA 私钥未加载")
	}
	// 注：实际应使用 rsa.SignPSS（更安全），此处简化为 PKCS1v15
	// 需验证：PKCS1v15 vs PSS 在 SDK 端兼容性
	hashed := sha512.Sum512(data)
	sig, err := rsa.SignPKCS1v15(rand.Reader, m.privateKey, crypto.SHA512, hashed[:])
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(sig), nil
}

// VerifyRSASignature 校验 RSA 签名
func (m *Manager) VerifyRSASignature(data []byte, signature string) error {
	if m.publicKey == nil {
		return errors.New("RSA 公钥未加载")
	}
	sig, err := base64.StdEncoding.DecodeString(signature)
	if err != nil {
		return err
	}
	hashed := sha512.Sum512(data)
	return rsa.VerifyPKCS1v15(m.publicKey, crypto.SHA512, hashed[:], sig)
}

// ============== HMAC-SHA256 ==============

// HMACSHA256 计算 HMAC-SHA256 签名
// 用于客户端请求签名（X-Signature）
// 签名原文：METHOD\nPATH?QUERY\nTIMESTAMP\nNONCE\nBODY
// 输出 64 位小写 hex
func HMACSHA256(secret string, data []byte) string {
	mac := hmac.New(sha512.New512_256, []byte(secret))
	mac.Write(data)
	return hex.EncodeToString(mac.Sum(nil))
}

// 注：sha512.New512_256 是 SHA-512/256 变体（与 SHA-256 输出长度相同但更安全）。
// 如 SDK 端用标准 SHA-256，可改用 sha256.New()。需验证：当前实现兼容性。

// HMACEqual 常量时间比较（防时序攻击）
func HMACEqual(a, b string) bool {
	return hmac.Equal([]byte(a), []byte(b))
}

// ============== bcrypt 密码哈希 ==============

// HashPassword bcrypt 加密（cost=12，参考布丁卡密）
const bcryptCost = 12

func HashPassword(password string) (string, error) {
	if password == "" {
		return "", errors.New("密码不能为空")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// CheckPassword 校验密码
func CheckPassword(hashed, password string) bool {
	if hashed == "" || password == "" {
		return false
	}
	return bcrypt.CompareHashAndPassword([]byte(hashed), []byte(password)) == nil
}

// ============== SHA-512 / 卡密哈希 ==============

// SHA512Hex 返回 SHA-512 十六进制
func SHA512Hex(data string) string {
	h := sha512.Sum512([]byte(data))
	return hex.EncodeToString(h[:])
}

// SHA512Checksum8 卡密校验位（前 8 位，防伪）
func SHA512Checksum8(data string) string {
	h := sha512.Sum512([]byte(data))
	return hex.EncodeToString(h[:])[:8]
}

// ============== 卡密生成器 ==============

// 卡密字符集（去除易混淆字符 0/O/1/I/L）
const cardKeyCharset = "ABCDEFGHJKMNPQRSTUVWXYZ23456789"

// GenerateCardKey 生成单张卡密
// 格式：PREFIX-XXXX-XXXX-XXXX-XXXX（4 段，每段 4 字符）
// 使用 crypto/rand 系统熵源（铁律 04：SecureRandom）
func GenerateCardKey(prefix string) (key, hash, checksum string, err error) {
	segments := make([]string, 4)
	for i := 0; i < 4; i++ {
		seg, err := randomString(4, cardKeyCharset)
		if err != nil {
			return "", "", "", err
		}
		segments[i] = seg
	}
	if prefix != "" {
		key = prefix + "-" + strings.Join(segments, "-")
	} else {
		key = strings.Join(segments, "-")
	}
	hash = SHA512Hex(key)
	checksum = SHA512Checksum8(key + hash)
	return key, hash, checksum, nil
}

// randomString 用 crypto/rand 生成随机字符串
func randomString(n int, charset string) (string, error) {
	if n <= 0 {
		return "", errors.New("长度必须大于 0")
	}
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	for i := 0; i < n; i++ {
		buf[i] = charset[int(buf[i])%len(charset)]
	}
	return string(buf), nil
}

// RandomHex 生成指定长度的随机十六进制字符串
func RandomHex(n int) (string, error) {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

// ============== RSA 密钥文件加载 ==============

func loadPrivateKey(path string) (*rsa.PrivateKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, errors.New("PEM 解码失败")
	}
	// 优先尝试 PKCS8，回退 PKCS1
	if key, err := x509.ParsePKCS8PrivateKey(block.Bytes); err == nil {
		if rsaKey, ok := key.(*rsa.PrivateKey); ok {
			return rsaKey, nil
		}
		return nil, errors.New("非 RSA 私钥")
	}
	return x509.ParsePKCS1PrivateKey(block.Bytes)
}

func loadPublicKey(path string) (*rsa.PublicKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, errors.New("PEM 解码失败")
	}
	// 优先 PKIX，回退 PKCS1
	if key, err := x509.ParsePKIXPublicKey(block.Bytes); err == nil {
		if rsaKey, ok := key.(*rsa.PublicKey); ok {
			return rsaKey, nil
		}
		return nil, errors.New("非 RSA 公钥")
	}
	return x509.ParsePKCS1PublicKey(block.Bytes)
}
