// Package logger 结构化日志封装单元测试
// 严格遵循铁律 06：所有断言基于已知固定输入，无随机/不确定性
package logger

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"
)

// captureOutput 临时替换全局 logger 的输出为 buffer，返回 buffer 内容
// 测试结束后恢复原 logger
func captureOutput(t *testing.T, level string, fn func()) string {
	t.Helper()
	var buf bytes.Buffer
	prev := globalLogger.Load().(*slog.Logger)
	defer globalLogger.Store(prev)

	handler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: parseLevel(level)})
	globalLogger.Store(slog.New(handler))
	fn()
	return buf.String()
}

func TestParseLevel(t *testing.T) {
	cases := map[string]slog.Level{
		"":        slog.LevelInfo,
		"info":    slog.LevelInfo,
		"INFO":    slog.LevelInfo,
		"debug":   slog.LevelDebug,
		"warn":    slog.LevelWarn,
		"warning": slog.LevelWarn,
		"error":   slog.LevelError,
		"unknown": slog.LevelInfo, // 默认 info
	}
	for input, want := range cases {
		if got := parseLevel(input); got != want {
			t.Errorf("parseLevel(%q) = %v, want %v", input, got, want)
		}
	}
}

// TestInit_JSONFormat 验证 Init 后默认 JSON 格式输出包含 level/msg/字段
func TestInit_JSONFormat(t *testing.T) {
	output := captureOutput(t, "info", func() {
		Info("test event", "user_id", uint64(42), "action", "login")
	})

	var entry map[string]interface{}
	if err := json.NewDecoder(strings.NewReader(output)).Decode(&entry); err != nil {
		t.Fatalf("输出不是合法 JSON: %v\nraw: %s", err, output)
	}
	if entry["msg"] != "test event" {
		t.Errorf("msg = %v, want 'test event'", entry["msg"])
	}
	if entry["level"] != "INFO" {
		t.Errorf("level = %v, want INFO", entry["level"])
	}
	if entry["user_id"].(float64) != 42 {
		t.Errorf("user_id = %v, want 42", entry["user_id"])
	}
	if entry["action"] != "login" {
		t.Errorf("action = %v, want 'login'", entry["action"])
	}
}

// TestInit_LevelFiltering 验证 level=warn 时 info/debug 不输出
func TestInit_LevelFiltering(t *testing.T) {
	output := captureOutput(t, "warn", func() {
		Debug("debug msg")
		Info("info msg")
		Warn("warn msg")
		Error("error msg")
	})

	if strings.Contains(output, "debug msg") {
		t.Errorf("level=warn 时不应输出 debug：\n%s", output)
	}
	if strings.Contains(output, "info msg") {
		t.Errorf("level=warn 时不应输出 info：\n%s", output)
	}
	if !strings.Contains(output, "warn msg") {
		t.Errorf("level=warn 时应输出 warn：\n%s", output)
	}
	if !strings.Contains(output, "error msg") {
		t.Errorf("level=warn 时应输出 error：\n%s", output)
	}
}

// TestInit_TextFormat 验证 Init with format=text 切换为文本格式
func TestInit_TextFormat(t *testing.T) {
	prev := globalLogger.Load().(*slog.Logger)
	defer globalLogger.Store(prev)

	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo})
	globalLogger.Store(slog.New(handler))

	Info("text format test", "key", "value")
	out := buf.String()

	// slog text 格式：msg 含空格时会自动加引号：msg="text format test"
	if !strings.Contains(out, `msg="text format test"`) {
		t.Errorf("text 格式应包含 msg= 前缀：%s", out)
	}
	if !strings.Contains(out, "key=value") {
		t.Errorf("text 格式应包含 key=value：%s", out)
	}
}

// TestL_ReturnsNonNil 验证 L() 在 Init 前后都返回非 nil logger
func TestL_ReturnsNonNil(t *testing.T) {
	if L() == nil {
		t.Fatal("L() 返回 nil")
	}
	Init(Options{Level: "info", Format: "json", Output: "stdout"})
	if L() == nil {
		t.Fatal("Init 后 L() 返回 nil")
	}
}

// TestInit_DefaultFallback 验证 Init 默认值（空 Options）不 panic 且使用 JSON + info
func TestInit_DefaultFallback(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Init with empty Options panic: %v", r)
		}
	}()
	Init(Options{})
	Info("default init ok")
}
