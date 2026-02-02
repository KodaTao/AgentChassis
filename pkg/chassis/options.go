// Package chassis 提供 AgentChassis 核心框架
package chassis

import (
	"time"

	"github.com/KodaTao/AgentChassis/pkg/llm"
)

// Config 应用配置
type Config struct {
	Server        ServerConfig        `mapstructure:"server"`
	LLM           llm.Config          `mapstructure:"llm"`
	Database      DatabaseConfig      `mapstructure:"database"`
	Log           LogConfig           `mapstructure:"log"`
	Observability ObservabilityConfig `mapstructure:"observability"`
	Telegram      TelegramConfig      `mapstructure:"telegram"`
}

// TelegramConfig Telegram Bot 配置
type TelegramConfig struct {
	// Enabled 是否启用 Telegram Bot
	Enabled bool `mapstructure:"enabled"`

	// Token Bot Token
	Token string `mapstructure:"token"`

	// SessionTTL Session 映射保留时间
	SessionTTL time.Duration `mapstructure:"session_ttl"`
}

// ServerConfig 服务器配置
type ServerConfig struct {
	// Host 监听地址
	Host string `mapstructure:"host"`

	// Port 监听端口
	Port int `mapstructure:"port"`

	// Mode 运行模式：debug, release, test
	Mode string `mapstructure:"mode"`
}

// DatabaseConfig 数据库配置
type DatabaseConfig struct {
	// Path 数据库文件路径
	Path string `mapstructure:"path"`
}

// LogConfig 日志配置
type LogConfig struct {
	// Level 日志级别：debug, info, warn, error
	Level string `mapstructure:"level"`

	// Format 日志格式：text, json
	Format string `mapstructure:"format"`

	// Output 输出目标：stdout, file
	Output string `mapstructure:"output"`

	// FilePath 日志文件路径（当 Output 为 file 时生效）
	FilePath string `mapstructure:"file_path"`
}

// ObservabilityConfig 可观测性配置
type ObservabilityConfig struct {
	Metrics MetricsConfig `mapstructure:"metrics"`
	Tracing TracingConfig `mapstructure:"tracing"`
}

// MetricsConfig 指标配置
type MetricsConfig struct {
	// Enabled 是否启用
	Enabled bool `mapstructure:"enabled"`

	// Path 指标暴露路径
	Path string `mapstructure:"path"`
}

// TracingConfig 链路追踪配置
type TracingConfig struct {
	// Enabled 是否启用
	Enabled bool `mapstructure:"enabled"`

	// Endpoint 追踪数据上报地址
	Endpoint string `mapstructure:"endpoint"`
}

// DefaultConfig 返回默认配置
func DefaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Host: "0.0.0.0",
			Port: 8080,
			Mode: "debug",
		},
		LLM: llm.Config{
			Provider:    "openai",
			BaseURL:     "https://api.openai.com/v1",
			Model:       "gpt-4",
			Timeout:     60,
			MaxTokens:   4096,
			Temperature: 0.7,
		},
		Database: DatabaseConfig{
			Path: "~/.agentchassis/data.db",
		},
		Log: LogConfig{
			Level:  "info",
			Format: "text",
			Output: "stdout",
		},
		Observability: ObservabilityConfig{
			Metrics: MetricsConfig{
				Enabled: false,
				Path:    "/metrics",
			},
			Tracing: TracingConfig{
				Enabled: false,
			},
		},
		Telegram: TelegramConfig{
			Enabled:    false,
			Token:      "",
			SessionTTL: 24 * time.Hour,
		},
	}
}

// Option 配置选项函数
type Option func(*Config)

// WithServerPort 设置服务器端口
func WithServerPort(port int) Option {
	return func(c *Config) {
		c.Server.Port = port
	}
}

// WithServerMode 设置运行模式
func WithServerMode(mode string) Option {
	return func(c *Config) {
		c.Server.Mode = mode
	}
}

// WithLLMConfig 设置 LLM 配置
func WithLLMConfig(cfg llm.Config) Option {
	return func(c *Config) {
		c.LLM = cfg
	}
}

// WithLogLevel 设置日志级别
func WithLogLevel(level string) Option {
	return func(c *Config) {
		c.Log.Level = level
	}
}

// WithDatabasePath 设置数据库路径
func WithDatabasePath(path string) Option {
	return func(c *Config) {
		c.Database.Path = path
	}
}

// WithTelegram 设置telegram设置
func WithTelegram(t TelegramConfig) Option {
	return func(c *Config) {
		c.Telegram = t
	}
}

// SessionConfig 会话配置
type SessionConfig struct {
	// MaxHistory 最大历史消息数
	MaxHistory int

	// TTL 会话过期时间
	TTL time.Duration
}

// DefaultSessionConfig 返回默认会话配置
func DefaultSessionConfig() *SessionConfig {
	return &SessionConfig{
		MaxHistory: 20,
		TTL:        30 * time.Minute,
	}
}
