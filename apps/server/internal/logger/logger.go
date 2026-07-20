// Package logger 结构化日志封装（v0.4.0）
// 基于 Go 标准库 log/slog，零第三方依赖
// 严格遵循铁律 04/05：日志级别 / 输出格式 / 输出路径 均可通过 sys_config 覆盖
// 严格遵循铁律 06：所有日志字段为强类型，禁止 fmt.Sprintf 拼接敏感信息
//
// 用法：
//
//	logger.Init(logger.Options{Level: "info", Format: "json", Output: "stdout"})
//	logger.Info("user login", "user_id", 123, "ip", "1.2.3.4")
//	logger.Error("db write failed", "err", err, "table", "log_login_failed")
package logger

import (
	"context"
	"io"
	"log/slog"
	"os"
	"strings"
	"sync/atomic"
)

// Options 日志配置（v0.4.0：main.go 启动时从 config / sys_config 注入）
type Options struct {
	Level  string // debug / info / warn / error（默认 info）
	Format string // json / text（默认 json，生产推荐 json 便于 ELK 采集）
	Output string // stdout / stderr / 文件路径（默认 stdout）
}

// 全局 logger（atomic.Value 保证并发安全切换）
// 默认指向 slog.Default()，确保 Init 前调用 logger.Info 等不 panic
var globalLogger atomic.Value

func init() {
	globalLogger.Store(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))
}

// Init 初始化全局 logger（应在 main.go 启动时调用一次）
// 重复调用会替换既有 logger（用于运行时热更新日志级别）
func Init(opt Options) {
	level := parseLevel(opt.Level)
	format := strings.ToLower(strings.TrimSpace(opt.Format))
	if format == "" {
		format = "json"
	}
	var output io.Writer = os.Stdout
	switch strings.ToLower(strings.TrimSpace(opt.Output)) {
	case "", "stdout":
		output = os.Stdout
	case "stderr":
		output = os.Stderr
	default:
		// 文件路径：以 append 模式打开（0644）
		f, err := os.OpenFile(opt.Output, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			// 回退到 stdout，避免因日志路径错误导致服务无法启动
			slog.Error("logger open file failed, fallback to stdout", "err", err, "path", opt.Output)
			output = os.Stdout
		} else {
			output = f
		}
	}

	var handler slog.Handler
	opts := &slog.HandlerOptions{Level: level}
	switch format {
	case "text":
		handler = slog.NewTextHandler(output, opts)
	default:
		handler = slog.NewJSONHandler(output, opts)
	}
	globalLogger.Store(slog.New(handler))
	slog.SetDefault(globalLogger.Load().(*slog.Logger))
}

// parseLevel 字符串转 slog.Level（默认 info）
func parseLevel(s string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// L 返回当前全局 logger（供调用方使用 With 等方法派生子 logger）
func L() *slog.Logger {
	return globalLogger.Load().(*slog.Logger)
}

// Debug / Info / Warn / Error 便捷封装
func Debug(msg string, args ...any) { L().Debug(msg, args...) }
func Info(msg string, args ...any)  { L().Info(msg, args...) }
func Warn(msg string, args ...any)  { L().Warn(msg, args...) }
func Error(msg string, args ...any) { L().Error(msg, args...) }

// DebugCtx / InfoCtx / WarnCtx / ErrorCtx 带 context 的便捷封装（用于链路追踪）
func DebugCtx(ctx context.Context, msg string, args ...any) { L().DebugContext(ctx, msg, args...) }
func InfoCtx(ctx context.Context, msg string, args ...any)  { L().InfoContext(ctx, msg, args...) }
func WarnCtx(ctx context.Context, msg string, args ...any)  { L().WarnContext(ctx, msg, args...) }
func ErrorCtx(ctx context.Context, msg string, args ...any) { L().ErrorContext(ctx, msg, args...) }
