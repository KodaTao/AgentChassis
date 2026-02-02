// Package types 提供跨包共享的类型定义
package types

import "context"

// ChannelContext 渠道上下文
// 用于标识消息来源渠道，任务执行时也会使用此信息进行通知
type ChannelContext struct {
	Type   string            `json:"type"`              // 渠道类型: console, telegram, wechat, email...
	ChatID string            `json:"chat_id,omitempty"` // 聊天ID，如 telegram chat id
	Extra  map[string]string `json:"extra,omitempty"`   // 其他扩展参数
}

// ChatRequest 对话请求
type ChatRequest struct {
	SessionID string          `json:"session_id"`
	Message   string          `json:"message"`
	Channel   *ChannelContext `json:"channel,omitempty"` // 渠道上下文
}

// ChatResponse 对话响应
type ChatResponse struct {
	SessionID     string         `json:"session_id"`
	Reply         string         `json:"reply"`
	FunctionCalls []FunctionCall `json:"function_calls,omitempty"`
}

// FunctionCall 函数调用记录
type FunctionCall struct {
	Name   string `json:"name"`
	Status string `json:"status"` // success, error
	Result string `json:"result"`
}

// Agent 接口定义
// 用于解耦 telegram 包对 chassis 包的直接依赖
type Agent interface {
	Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error)
}
