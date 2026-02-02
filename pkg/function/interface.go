// Package function 提供 Function 接口定义和相关类型
package function

import (
	"context"
	"reflect"
)

// Function 是所有可调用函数的基础接口
// AI 通过 Name() 识别函数，通过 Description() 理解函数用途
type Function interface {
	// Name 返回函数的唯一标识符，AI 通过此名称调用
	// 命名规范：小写字母、数字、下划线，如 "clean_logs", "send_email"
	Name() string

	// Description 返回函数描述，用于 AI 理解函数用途
	// 描述应简洁明了，说明函数的功能和使用场景
	Description() string

	// Execute 执行函数
	// ctx 用于超时控制和取消
	// params 是通过反射解析的结构化参数，类型由 ParamsType() 决定
	Execute(ctx context.Context, params any) (Result, error)

	// ParamsType 返回参数的反射类型
	// 框架通过此方法获取参数结构，自动生成 Schema
	// 返回 nil 表示该函数不需要参数
	ParamsType() reflect.Type
}

// Result 函数执行结果
type Result struct {
	// Data 结构化数据，将被编码为 TOON 格式
	// 支持 struct、slice、map 等类型
	Data any `json:"data,omitempty"`

	// Markdown 可选的 Markdown 格式输出
	// 用于需要富文本展示的场景
	Markdown string `json:"markdown,omitempty"`

	// Message 简短的文本消息
	// 用于向 AI 简要说明执行结果
	Message string `json:"message,omitempty"`
}

// FunctionInfo 函数元信息，用于 API 返回和 Prompt 生成
type FunctionInfo struct {
	Name        string       `json:"name"`
	Description string       `json:"description"`
	Parameters  []ParamInfo  `json:"parameters,omitempty"`
}

// ParamInfo 参数元信息
type ParamInfo struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Description string `json:"description,omitempty"`
	Required    bool   `json:"required"`
	Default     string `json:"default,omitempty"`
}
