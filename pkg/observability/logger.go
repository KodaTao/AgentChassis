// Package observability 提供可观测性功能：日志、指标、链路追踪
package observability

import (
	"context"
	"io"
	"log/slog"
	"os"
	"strings"
)

// Logger 全局日志实例
var Logger *slog.Logger

// LogConfig 日志配置
type LogConfig struct {
	Level    string // debug, info, warn, error
	Format   string // text, json
	Output   string // stdout, file
	FilePath string // 日志文件路径
}

// InitLogger 初始化日志系统
func InitLogger(cfg LogConfig) error {
	var (
		writer  io.Writer
		handler slog.Handler
		level   slog.Level
	)

	// 解析日志级别
	switch strings.ToLower(cfg.Level) {
	case "debug":
		level = slog.LevelDebug
	case "info":
		level = slog.LevelInfo
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	// 设置输出目标
	switch strings.ToLower(cfg.Output) {
	case "file":
		if cfg.FilePath == "" {
			cfg.FilePath = "agentchassis.log"
		}
		file, err := os.OpenFile(cfg.FilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return err
		}
		writer = file
	default:
		writer = os.Stdout
	}

	// 设置日志格式
	opts := &slog.HandlerOptions{
		Level:     level,
		AddSource: level == slog.LevelDebug, // Debug 模式下添加源码位置
	}

	switch strings.ToLower(cfg.Format) {
	case "json":
		handler = slog.NewJSONHandler(writer, opts)
	default:
		handler = slog.NewTextHandler(writer, opts)
	}

	Logger = slog.New(handler)
	slog.SetDefault(Logger)

	return nil
}

// DefaultLogger 返回默认日志实例
func DefaultLogger() *slog.Logger {
	if Logger == nil {
		Logger = slog.Default()
	}
	return Logger
}

// WithContext 创建带有上下文信息的日志器
func WithContext(ctx context.Context) *slog.Logger {
	logger := DefaultLogger()

	// 从 context 中提取 trace_id 等信息
	if traceID := ctx.Value("trace_id"); traceID != nil {
		logger = logger.With("trace_id", traceID)
	}
	if sessionID := ctx.Value("session_id"); sessionID != nil {
		logger = logger.With("session_id", sessionID)
	}

	return logger
}

// Debug 记录 Debug 级别日志
func Debug(msg string, args ...any) {
	DefaultLogger().Debug(msg, args...)
}

// Info 记录 Info 级别日志
func Info(msg string, args ...any) {
	DefaultLogger().Info(msg, args...)
}

// Warn 记录 Warn 级别日志
func Warn(msg string, args ...any) {
	DefaultLogger().Warn(msg, args...)
}

// Error 记录 Error 级别日志
func Error(msg string, args ...any) {
	DefaultLogger().Error(msg, args...)
}

// DebugContext 记录带上下文的 Debug 日志
func DebugContext(ctx context.Context, msg string, args ...any) {
	WithContext(ctx).Debug(msg, args...)
}

// InfoContext 记录带上下文的 Info 日志
func InfoContext(ctx context.Context, msg string, args ...any) {
	WithContext(ctx).Info(msg, args...)
}

// WarnContext 记录带上下文的 Warn 日志
func WarnContext(ctx context.Context, msg string, args ...any) {
	WithContext(ctx).Warn(msg, args...)
}

// ErrorContext 记录带上下文的 Error 日志
func ErrorContext(ctx context.Context, msg string, args ...any) {
	WithContext(ctx).Error(msg, args...)
}

// LLMRequestLog 记录 LLM 请求日志
func LLMRequestLog(ctx context.Context, provider, model string, messageCount int) {
	WithContext(ctx).Info("LLM request",
		"provider", provider,
		"model", model,
		"message_count", messageCount,
	)
}

// LLMResponseLog 记录 LLM 响应日志
func LLMResponseLog(ctx context.Context, provider string, durationMs int64, tokenUsage map[string]int) {
	WithContext(ctx).Info("LLM response",
		"provider", provider,
		"duration_ms", durationMs,
		"prompt_tokens", tokenUsage["prompt"],
		"completion_tokens", tokenUsage["completion"],
		"total_tokens", tokenUsage["total"],
	)
}

// FunctionCallLog 记录 Function 调用日志
func FunctionCallLog(ctx context.Context, funcName string, status string, durationMs int64) {
	WithContext(ctx).Info("Function call",
		"function", funcName,
		"status", status,
		"duration_ms", durationMs,
	)
}
