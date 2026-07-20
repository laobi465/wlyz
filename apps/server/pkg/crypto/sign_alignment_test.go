// Package crypto 客户端 SDK 签名对齐测试
// 验证 Python / Node.js / PHP 三语言 SDK 与后端 HMACSHA256 输出完全一致
// 严格遵循铁律 06：所有断言基于已知固定输入，无随机/不确定性
//
// 运行时若缺少 python3/node/php，对应子测试自动 t.Skip
// Node.js 若编译的 OpenSSL 不支持 sha512/256（脚本退出码 2），对应子测试 t.Skip
package crypto

import (
	"errors"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// signTestCases 一组固定输入（secret + signString），用于跨语言对齐
// signString 格式遵循 SPEC 3.2：METHOD\nPATH?QUERY\nTIMESTAMP\nNONCE\nBODY
var signTestCases = []struct {
	name   string
	secret string
	msg    string
}{
	{
		name:   "login_最小用例",
		secret: "test-sign-secret-12345",
		msg:    "POST\n/api/v1/client/login\n1721374800\na1b2c3d4e5f6\n{\"card_key\":\"K2X9-AB7C-MN4P-QR8S\",\"hwid\":\"abc123\"}",
	},
	{
		name:   "heartbeat_中文+长 body",
		secret: "另一个密钥",
		msg:    "POST\n/api/v1/client/heartbeat?ts=1721374800\n1721374900\nnonce-xyz\n{\"card_key\":\"VIP-测试-卡密\",\"hwid\":\"hw-中文-指纹\",\"extra\":\"数据\"}",
	},
	{
		name:   "get_var_空 body",
		secret: "secret-key",
		msg:    "GET\n/api/v1/client/get_var?key=pro_feature\n1721375000\nn0nc3-aaa\n",
	},
}

// runSignScript 调用外部脚本计算签名
// 返回 (signature, error)。脚本退出码 2 视为环境限制（如 Node OpenSSL 不支持 sha512/256）
func runSignScript(t *testing.T, interpreter, scriptPath, secret, msg string) (string, error) {
	t.Helper()
	cmd := exec.Command(interpreter, scriptPath, secret, msg)
	out, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 2 {
			t.Skipf("环境限制：%s 不支持 sha512/256（stderr: %s）", interpreter, strings.TrimSpace(string(exitErr.Stderr)))
		}
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// sdkScriptsDir 返回 sdks/tests 目录的绝对路径
func sdkScriptsDir() string {
	_, file, _, _ := runtime.Caller(0)
	// file = .../apps/server/pkg/crypto/sign_alignment_test.go
	// sdks/tests = ../../../../sdks/tests
	return filepath.Join(filepath.Dir(file), "..", "..", "..", "..", "sdks", "tests")
}

// TestSignAlignment_AllLanguages 三语言 SDK 签名 vs 后端 HMACSHA256 完全一致
// 这是 v0.3.6 客户端 SDK 接入规范（SPEC 7.4）的核心保障测试
func TestSignAlignment_AllLanguages(t *testing.T) {
	scriptsDir := sdkScriptsDir()

	interpreters := []struct {
		name       string
		binary     string
		scriptName string
	}{
		{"Python", "python3", "sign.py"},
		{"Node.js", "node", "sign.js"},
		{"PHP", "php", "sign.php"},
	}

	for _, tc := range signTestCases {
		t.Run(tc.name, func(t *testing.T) {
			// 1. 后端基准签名
			backendSig := HMACSHA256(tc.secret, []byte(tc.msg))
			require.NotEmpty(t, backendSig)
			require.Len(t, backendSig, 64, "后端签名应为 64 位 hex")

			// 2. 三语言 SDK 签名对比
			for _, interp := range interpreters {
				interp := interp
				t.Run(interp.name, func(t *testing.T) {
					// 检查运行时是否可用，不可用则跳过（不强制依赖）
					if _, err := exec.LookPath(interp.binary); err != nil {
						t.Skipf("运行时 %s 不可用，跳过签名对齐测试", interp.binary)
					}

					scriptPath := filepath.Join(scriptsDir, interp.scriptName)
					sdkSig, err := runSignScript(t, interp.binary, scriptPath, tc.secret, tc.msg)
					require.NoError(t, err, "%s 脚本执行失败", interp.name)
					require.Len(t, sdkSig, 64, "%s 签名应为 64 位 hex", interp.name)

					assert.Equal(t, backendSig, sdkSig,
						"%s SDK 签名与后端不一致\nsecret=%s\nmsg=%s",
						interp.name, tc.secret, tc.msg)
				})
			}
		})
	}
}

// TestSignAlignment_BackendDeterministic 后端 HMACSHA256 对同一输入应确定性输出
func TestSignAlignment_BackendDeterministic(t *testing.T) {
	secret := "deterministic-secret"
	msg := "POST\n/api/v1/client/login\n1\nn\n{}"
	sig1 := HMACSHA256(secret, []byte(msg))
	sig2 := HMACSHA256(secret, []byte(msg))
	assert.Equal(t, sig1, sig2, "同一输入应产生相同签名")
	assert.Len(t, sig1, 64)
}
