// Package telegram 提供 Telegram Bot 集成功能
package telegram

import "time"

// Config Telegram Bot 配置
type Config struct {
	Enabled    bool          `mapstructure:"enabled"`     // 是否启用 Telegram Bot
	Token      string        `mapstructure:"token"`       // Bot Token
	SessionTTL time.Duration `mapstructure:"session_ttl"` // Session 映射保留时间
}

// DefaultConfig 返回默认配置
func DefaultConfig() *Config {
	return &Config{
		Enabled:    false,
		Token:      "",
		SessionTTL: 24 * time.Hour,
	}
}

// Validate 验证配置
func (c Config) Validate() error {
	if c.Enabled && c.Token == "" {
		return ErrTokenRequired
	}
	return nil
}
