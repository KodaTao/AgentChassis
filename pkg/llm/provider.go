// Package llm 提供 LLM 适配层接口和实现
package llm

import (
	"context"
)

// Provider LLM 提供商接口
// 所有 LLM 实现（OpenAI、Claude 等）都需要实现此接口
type Provider interface {
	// Chat 发送对话请求
	// messages 是对话历史
	// 返回 AI 的回复内容
	Chat(ctx context.Context, messages []Message) (string, error)

	// ChatStream 发送流式对话请求
	// messages 是对话历史
	// 返回一个 channel，逐步返回 AI 回复的内容片段
	ChatStream(ctx context.Context, messages []Message) (<-chan StreamChunk, error)

	// Name 返回提供商名称
	Name() string
}

// Message 对话消息
type Message struct {
	Role    Role   `json:"role"`
	Content string `json:"content"`
}

// Role 消息角色
type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
)

// StreamChunk 流式响应片段
type StreamChunk struct {
	// Content 内容片段
	Content string `json:"content"`

	// Done 是否完成
	Done bool `json:"done"`

	// Error 错误信息（如果有）
	Error error `json:"error,omitempty"`
}

// Config LLM 通用配置
type Config struct {
	// Provider 提供商类型：openai, azure, custom
	Provider string `mapstructure:"provider"`

	// APIKey API 密钥
	APIKey string `mapstructure:"api_key"`

	// BaseURL API 基础 URL（用于自定义 endpoint）
	BaseURL string `mapstructure:"base_url"`

	// Model 模型名称
	Model string `mapstructure:"model"`

	// Timeout 请求超时时间（秒）
	Timeout int `mapstructure:"timeout"`

	// MaxTokens 最大 Token 数
	MaxTokens int `mapstructure:"max_tokens"`

	// Temperature 温度参数（0-2）
	Temperature float64 `mapstructure:"temperature"`
}

// Usage Token 使用统计
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}
