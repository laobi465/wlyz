// pkg/crypto 单元测试
// 覆盖 AES-256-GCM / HMAC-SHA512/256 / bcrypt / SHA-512 / MD5 / 彩虹易支付签名 / 卡密生成器
// 严格遵循铁律 06：所有断言基于已知向量或可验证性质，禁编造
package crypto

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/hex"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============== Manager 构造 ==============

func TestNewManager_AESKeyLength(t *testing.T) {
	t.Run("32 字节 AES 密钥 - 通过", func(t *testing.T) {
		m, err := NewManager("0123456789abcdef0123456789abcdef", "", "")
		require.NoError(t, err)
		require.NotNil(t, m)
	})
	t.Run("非 32 字节 - 拒绝", func(t *testing.T) {
		_, err := NewManager("short", "", "")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "AES 密钥必须 32 字节")
	})
}

// ============== AES-256-GCM ==============

func TestAES_EncryptDecrypt_RoundTrip(t *testing.T) {
	m, err := NewManager("0123456789abcdef0123456789abcdef", "", "")
	require.NoError(t, err)

	cases := []string{
		"",
		"hello",
		"中文测试",
		"特殊字符 !@#$%^&*()_+-=",
		strings.Repeat("a", 4096), // 大文本
	}
	for _, plain := range cases {
		t.Run("roundtrip", func(t *testing.T) {
			cipher, err := m.EncryptAES(plain)
			require.NoError(t, err)
			// 空串原样返回空
			if plain == "" {
				assert.Equal(t, "", cipher)
				return
			}
			// 密文应不等于明文
			assert.NotEqual(t, plain, cipher)
			// 解密应还原
			decrypted, err := m.DecryptAES(cipher)
			require.NoError(t, err)
			assert.Equal(t, plain, decrypted)
		})
	}
}

func TestAES_DifferentCipherForSamePlain(t *testing.T) {
	// GCM 模式下 nonce 随机，相同明文应产生不同密文
	m, _ := NewManager("0123456789abcdef0123456789abcdef", "", "")
	c1, _ := m.EncryptAES("hello")
	c2, _ := m.EncryptAES("hello")
	assert.NotEqual(t, c1, c2)
}

func TestAES_DecryptInvalidBase64(t *testing.T) {
	m, _ := NewManager("0123456789abcdef0123456789abcdef", "", "")
	_, err := m.DecryptAES("!!!invalid base64!!!")
	require.Error(t, err)
}

func TestAES_DecryptTruncatedCipher(t *testing.T) {
	m, _ := NewManager("0123456789abcdef0123456789abcdef", "", "")
	// 3 字节 base64 编码后仍短于 GCM nonce（12 字节）
	short := base64.StdEncoding.EncodeToString([]byte{1, 2, 3})
	_, err := m.DecryptAES(short)
	require.Error(t, err)
	// 应返回「密文长度不足」错误
	assert.Contains(t, err.Error(), "密文长度不足")
}

// ============== HMAC-SHA512/256 ==============

func TestHMACSHA256_OutputFormat(t *testing.T) {
	sig := HMACSHA256("secret", []byte("data"))
	// 输出应为 64 位小写 hex
	assert.Len(t, sig, 64)
	for _, c := range sig {
		assert.True(t, (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f'),
			"应全为小写 hex 字符，实际: %s", sig)
	}
}

func TestHMACSHA256_Deterministic(t *testing.T) {
	// 相同输入应产生相同输出
	sig1 := HMACSHA256("secret", []byte("data"))
	sig2 := HMACSHA256("secret", []byte("data"))
	assert.Equal(t, sig1, sig2)
}

func TestHMACSHA256_DifferentInput(t *testing.T) {
	// 不同输入应产生不同输出
	sig1 := HMACSHA256("secret1", []byte("data"))
	sig2 := HMACSHA256("secret2", []byte("data"))
	sig3 := HMACSHA256("secret", []byte("data1"))
	assert.NotEqual(t, sig1, sig2)
	assert.NotEqual(t, sig1, sig3)
}

func TestHMACSHA256_MatchesStandardSHA512_256(t *testing.T) {
	// 验证后端使用的是 SHA-512/256 变体（与标准 SHA-256 不同）
	// 标准 SHA-256 HMAC 输出：
	stdSHA256 := hex.EncodeToString(hmacSHA256Std([]byte("secret"), []byte("data")))
	// 后端 SHA-512/256 HMAC 输出：
	backend := HMACSHA256("secret", []byte("data"))

	// 两者长度都是 64 hex，但内容应不同
	assert.Len(t, stdSHA256, 64)
	assert.Len(t, backend, 64)
	// 待核实：sha512.New512_256 vs sha256 应产生不同结果
	assert.NotEqual(t, stdSHA256, backend,
		"后端 HMACSHA256 应使用 sha512.New512_256 变体，与标准 SHA-256 不同")
}

// hmacSHA256Std 标准库 SHA-256 HMAC（用于对比测试）
func hmacSHA256Std(key, data []byte) []byte {
	h := sha256.New()
	// 手动 HMAC 实现（避免引入 crypto/hmac 后再被测试）
	blockSize := 64
	if len(key) > blockSize {
		h.Write(key)
		key = h.Sum(nil)
		h.Reset()
	}
	ipad := make([]byte, blockSize)
	opad := make([]byte, blockSize)
	copy(ipad, key)
	copy(opad, key)
	for i := range ipad {
		ipad[i] ^= 0x36
		opad[i] ^= 0x5c
	}
	h.Write(ipad)
	h.Write(data)
	inner := h.Sum(nil)
	h.Reset()
	h.Write(opad)
	h.Write(inner)
	return h.Sum(nil)
}

// ============== HMACEqual ==============

func TestHMACEqual(t *testing.T) {
	assert.True(t, HMACEqual("abc", "abc"))
	assert.False(t, HMACEqual("abc", "abd"))
	assert.False(t, HMACEqual("", "abc"))
	assert.True(t, HMACEqual("", ""))
}

// ============== bcrypt ==============

func TestHashPassword_AndCheck(t *testing.T) {
	hash, err := HashPassword("MyStrong@2026")
	require.NoError(t, err)
	// bcrypt 哈希应以 $2 开头
	assert.True(t, strings.HasPrefix(hash, "$2"))
	// 校验正确密码
	assert.True(t, CheckPassword(hash, "MyStrong@2026"))
	// 校验错误密码
	assert.False(t, CheckPassword(hash, "wrong"))
}

func TestHashPassword_EmptyRejected(t *testing.T) {
	_, err := HashPassword("")
	require.Error(t, err)
}

func TestHashPassword_DifferentHashForSamePassword(t *testing.T) {
	// bcrypt 每次哈希应不同（随机 salt）
	h1, _ := HashPassword("same")
	h2, _ := HashPassword("same")
	assert.NotEqual(t, h1, h2)
	// 但都能匹配
	assert.True(t, CheckPassword(h1, "same"))
	assert.True(t, CheckPassword(h2, "same"))
}

func TestCheckPassword_Empty(t *testing.T) {
	assert.False(t, CheckPassword("", "any"))
	assert.False(t, CheckPassword("anyhash", ""))
}

// ============== SHA-512 ==============

func TestSHA512Hex_KnownVector(t *testing.T) {
	// SHA-512("") = cf83e1357eefb8bdf1542850d66d8007d620e4050b5715dc83f4a921d36ce9ce47d0d13c5d85f2b0ff8318d2877eec2f63b931bd47417a81a538327af927da3e
	empty := SHA512Hex("")
	assert.Equal(t, "cf83e1357eefb8bdf1542850d66d8007d620e4050b5715dc83f4a921d36ce9ce47d0d13c5d85f2b0ff8318d2877eec2f63b931bd47417a81a538327af927da3e", empty)

	// SHA-512("abc") = ddaf35a193617aba...
	abc := SHA512Hex("abc")
	expected := "ddaf35a193617abacc417349ae20413112e6fa4e89a97ea20a9eeee64b55d39a2192992a274fc1a836ba3c23a3feebbd454d4423643ce80e2a9ac94fa54ca49f"
	assert.Equal(t, expected, abc)
}

func TestSHA512Hex_Length(t *testing.T) {
	// SHA-512 输出 128 位 hex
	assert.Len(t, SHA512Hex("test"), 128)
}

func TestSHA512Checksum8(t *testing.T) {
	// 校验位取 SHA-512 前 8 字符
	sig := SHA512Hex("data")
	c8 := SHA512Checksum8("data")
	assert.Len(t, c8, 8)
	assert.Equal(t, sig[:8], c8)
}

// ============== MD5 ==============

func TestMD5Hex_KnownVector(t *testing.T) {
	// MD5("") = d41d8cd98f00b204e9800998ecf8427e
	assert.Equal(t, "d41d8cd98f00b204e9800998ecf8427e", MD5Hex(""))
	// MD5("abc") = 900150983cd24fb0d6963f7d28e17f72
	assert.Equal(t, "900150983cd24fb0d6963f7d28e17f72", MD5Hex("abc"))
}

// ============== SignEpayParams ==============

func TestSignEpayParams_KnownVector(t *testing.T) {
	// 彩虹易支付签名规则：
	// 1. 排除 sign / sign_type / 空值
	// 2. key ASCII 升序
	// 3. key1=value1&key2=value2&...
	// 4. 末尾追加 secret（不加 &key=）
	// 5. MD5
	// 已知向量（手动计算）：
	// params = {pid: "1001", type: "alipay", out_trade_no: "ORD123", money: "1.00"}
	// 拼接串 = "money=1.00&out_trade_no=ORD123&pid=1001&type=alipay" + "SECRET"
	// MD5(...) 应为确定值
	params := map[string]string{
		"pid":          "1001",
		"type":         "alipay",
		"out_trade_no": "ORD123",
		"money":        "1.00",
	}
	secret := "SECRET"
	sig := SignEpayParams(params, secret)

	// 手动重算校验
	manual := MD5Hex("money=1.00&out_trade_no=ORD123&pid=1001&type=alipay" + secret)
	assert.Equal(t, manual, sig)
}

func TestSignEpayParams_ExcludesSignFields(t *testing.T) {
	params := map[string]string{
		"pid":       "1001",
		"sign":      "should-be-excluded",
		"sign_type": "MD5",
		"money":     "1.00",
	}
	secret := "S"
	sig := SignEpayParams(params, secret)
	// 应等价于只含 pid + money 的签名
	expected := SignEpayParams(map[string]string{
		"pid":   "1001",
		"money": "1.00",
	}, secret)
	assert.Equal(t, expected, sig)
}

func TestSignEpayParams_ExcludesEmptyValue(t *testing.T) {
	params := map[string]string{
		"pid":   "1001",
		"empty": "",
		"money": "1.00",
	}
	sig := SignEpayParams(params, "S")
	expected := SignEpayParams(map[string]string{
		"pid":   "1001",
		"money": "1.00",
	}, "S")
	assert.Equal(t, expected, sig)
}

func TestVerifyEpaySign(t *testing.T) {
	params := map[string]string{
		"pid":   "1001",
		"money": "1.00",
	}
	secret := "S"
	sig := SignEpayParams(params, secret)
	assert.True(t, VerifyEpaySign(params, secret, sig))
	assert.False(t, VerifyEpaySign(params, secret, "wrong-sign"))
	assert.False(t, VerifyEpaySign(params, "wrong-secret", sig))
}

// ============== GenerateCardKey ==============

func TestGenerateCardKey_Format(t *testing.T) {
	key, hash, checksum, err := GenerateCardKey("VIP")
	require.NoError(t, err)

	// 格式应为 VIP-XXXX-XXXX-XXXX-XXXX
	assert.True(t, strings.HasPrefix(key, "VIP-"))
	parts := strings.Split(key, "-")
	assert.Len(t, parts, 5, "应为 5 段（前缀 + 4 段）")
	assert.Equal(t, "VIP", parts[0])
	for i := 1; i < 5; i++ {
		assert.Len(t, parts[i], 4, "每段 4 字符")
	}
	// 字符集应不含易混淆字符 0/O/1/I/L（仅校验随机段，不校验前缀）
	for i := 1; i < 5; i++ {
		for _, c := range parts[i] {
			assert.False(t, c == '0' || c == 'O' || c == '1' || c == 'I' || c == 'L',
				"卡密随机段含易混淆字符: %c", c)
		}
	}

	// hash 应等于 SHA512Hex(key)
	assert.Equal(t, SHA512Hex(key), hash)

	// checksum 应等于 SHA512Checksum8(key + hash)
	assert.Equal(t, SHA512Checksum8(key+hash), checksum)
	assert.Len(t, checksum, 8)
}

func TestGenerateCardKey_NoPrefix(t *testing.T) {
	key, _, _, err := GenerateCardKey("")
	require.NoError(t, err)
	// 无前缀时为 XXXX-XXXX-XXXX-XXXX
	parts := strings.Split(key, "-")
	assert.Len(t, parts, 4)
}

func TestGenerateCardKey_Uniqueness(t *testing.T) {
	// 连续生成 100 张应全部不同
	seen := make(map[string]bool, 100)
	for i := 0; i < 100; i++ {
		key, _, _, err := GenerateCardKey("U")
		require.NoError(t, err)
		assert.False(t, seen[key], "重复生成卡密: %s", key)
		seen[key] = true
	}
}

// ============== RandomHex ==============

func TestRandomHex(t *testing.T) {
	h, err := RandomHex(16)
	require.NoError(t, err)
	assert.Len(t, h, 32, "16 字节 = 32 hex")

	h2, _ := RandomHex(16)
	assert.NotEqual(t, h, h2, "应随机")
}

// ============== AppKey/Secret 生成 ==============

func TestGenerateAppKey(t *testing.T) {
	k, err := GenerateAppKey()
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(k, "ak_"))
	assert.Len(t, k, 3+32, "ak_ 前缀 + 32 hex")
}

func TestGenerateAppSecret(t *testing.T) {
	s, err := GenerateAppSecret()
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(s, "as_"))
	assert.Len(t, s, 3+64, "as_ 前缀 + 64 hex")
}

func TestGenerateSignSecret(t *testing.T) {
	s, err := GenerateSignSecret()
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(s, "ss_"))
	assert.Len(t, s, 3+64)
}

// ============== GenerateHWID ==============

func TestGenerateHWID(t *testing.T) {
	hwid := GenerateHWID("cpu-i7", "mb-serial", "AA:BB:CC:DD:EE:FF", "disk-sn")
	// 应为 SHA-512 hex
	assert.Len(t, hwid, 128)
	// 应等于 SHA512Hex("cpu-i7|mb-serial|AA:BB:CC:DD:EE:FF|disk-sn")
	expected := SHA512Hex("cpu-i7|mb-serial|AA:BB:CC:DD:EE:FF|disk-sn")
	assert.Equal(t, expected, hwid)
	// 不同输入应不同
	hwid2 := GenerateHWID("cpu-i7", "different", "AA:BB:CC:DD:EE:FF", "disk-sn")
	assert.NotEqual(t, hwid, hwid2)
}

// ============== SHA-512/256 验证（防 SDK 签名不一致 bug） ==============

func TestSHA512_256_Variant(t *testing.T) {
	// 验证后端 crypto.HMACSHA256 使用 sha512.New512_256 变体
	// 与标准 sha256 HMAC 应不同
	backend := HMACSHA256("test-secret", []byte("test-body"))

	// 标准 SHA-256 HMAC（对比组）
	h := hmacSHA256Std([]byte("test-secret"), []byte("test-body"))
	stdSHA256 := hex.EncodeToString(h)

	// 标准 SHA-512/256 HMAC（对比组）
	mac2 := hmac.New(sha512.New512_256, []byte("test-secret"))
	mac2.Write([]byte("test-body"))
	sha512_256 := hex.EncodeToString(mac2.Sum(nil))

	// 后端 HMACSHA256 应等于 sha512/256，不等于 sha256
	assert.Equal(t, sha512_256, backend, "后端 HMACSHA256 应等于 sha512/256 HMAC")
	assert.NotEqual(t, stdSHA256, backend, "应与标准 sha256 HMAC 不同")
}
