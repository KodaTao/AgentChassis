// Package scheduler 提供定时任务调度功能
package scheduler

import "context"

// AgentExecutor 定义 Agent 执行接口
// 用于解耦 scheduler 和 chassis 包，避免循环依赖
type AgentExecutor interface {
	// Execute 执行一次对话，返回 LLM 的最终回复
	// prompt: 发送给 LLM 的提示词
	// 返回: LLM 的最终回复文本
	Execute(ctx context.Context, prompt string) (string, error)
}
