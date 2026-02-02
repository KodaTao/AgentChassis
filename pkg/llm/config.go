// Package llm 提供 LLM 适配层接口和实现
package llm

import (
	"os"
	"strings"
)

// LoadConfigFromEnv 从环境变量加载配置
func LoadConfigFromEnv() Config {
	cfg := Config{
		Provider:    getEnv("AC_LLM_PROVIDER", "openai"),
		APIKey:      getEnv("AC_LLM_API_KEY", os.Getenv("OPENAI_API_KEY")),
		BaseURL:     getEnv("AC_LLM_BASE_URL", "https://api.openai.com/v1"),
		Model:       getEnv("AC_LLM_MODEL", "gpt-4"),
		Timeout:     getEnvInt("AC_LLM_TIMEOUT", 60),
		MaxTokens:   getEnvInt("AC_LLM_MAX_TOKENS", 4096),
		Temperature: getEnvFloat("AC_LLM_TEMPERATURE", 0.7),
	}
	return cfg
}

// ResolveAPIKey 解析 API Key（支持环境变量引用）
// 如果值以 ${} 包裹，则从环境变量读取
func ResolveAPIKey(key string) string {
	if strings.HasPrefix(key, "${") && strings.HasSuffix(key, "}") {
		envName := key[2 : len(key)-1]
		return os.Getenv(envName)
	}
	return key
}

// MaskAPIKey 脱敏 API Key，用于日志输出
func MaskAPIKey(key string) string {
	if len(key) <= 8 {
		return "****"
	}
	return key[:4] + "****" + key[len(key)-4:]
}

// getEnv 获取环境变量，支持默认值
func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

// getEnvInt 获取整数环境变量
func getEnvInt(key string, defaultVal int) int {
	val := os.Getenv(key)
	if val == "" {
		return defaultVal
	}
	// 简单解析
	var result int
	for _, c := range val {
		if c >= '0' && c <= '9' {
			result = result*10 + int(c-'0')
		}
	}
	if result == 0 {
		return defaultVal
	}
	return result
}

// getEnvFloat 获取浮点数环境变量
func getEnvFloat(key string, defaultVal float64) float64 {
	val := os.Getenv(key)
	if val == "" {
		return defaultVal
	}
	// 简单解析
	var result float64
	var decimal float64 = 0
	var decimalPlace float64 = 1
	inDecimal := false

	for _, c := range val {
		if c == '.' {
			inDecimal = true
		} else if c >= '0' && c <= '9' {
			if inDecimal {
				decimalPlace *= 10
				decimal += float64(c-'0') / decimalPlace
			} else {
				result = result*10 + float64(c-'0')
			}
		}
	}
	result += decimal
	if result == 0 {
		return defaultVal
	}
	return result
}

// Validate 验证配置
func (c *Config) Validate() error {
	if c.APIKey == "" {
		return ErrMissingAPIKey
	}
	if c.Model == "" {
		return ErrMissingModel
	}
	return nil
}

// WithAPIKey 设置 API Key
func (c *Config) WithAPIKey(key string) *Config {
	c.APIKey = ResolveAPIKey(key)
	return c
}

// WithBaseURL 设置 Base URL
func (c *Config) WithBaseURL(url string) *Config {
	c.BaseURL = url
	return c
}

// WithModel 设置模型
func (c *Config) WithModel(model string) *Config {
	c.Model = model
	return c
}

// 配置相关错误
type ConfigError struct {
	Message string
}

func (e *ConfigError) Error() string {
	return e.Message
}

var (
	ErrMissingAPIKey = &ConfigError{Message: "API key is required"}
	ErrMissingModel  = &ConfigError{Message: "model is required"}
)
