// Package chassis 提供 AgentChassis 核心框架
package chassis

import (
	"context"
	"fmt"

	"github.com/KodaTao/AgentChassis/pkg/scheduler"
)

// taskExecutionPromptPrefix 任务执行时的提示前缀
// 明确告诉 AI 这是一个定时任务被触发了，需要立即执行，而不是创建新任务
const taskExecutionPromptPrefix = `【系统提示：这是一个定时任务被触发了，请立即执行以下任务内容。
重要：不要创建新的定时任务或延时任务，而是直接执行任务。
如果任务需要通知用户，请使用 send_message 函数发送消息。】

任务内容：`

// agentExecutorAdapter 实现 scheduler.AgentExecutor 接口
// 用于将 Agent 适配到 scheduler 包，避免循环依赖
type agentExecutorAdapter struct {
	agent *Agent
}

// NewAgentExecutorAdapter 创建 AgentExecutor 适配器
func NewAgentExecutorAdapter(agent *Agent) scheduler.AgentExecutor {
	return &agentExecutorAdapter{agent: agent}
}

// Execute 执行一次独立的对话，返回 LLM 的最终回复
// 每次执行使用独立的 session，不保留历史上下文
func (a *agentExecutorAdapter) Execute(ctx context.Context, prompt string) (string, error) {
	// 添加任务执行前缀，明确告诉 AI 这是任务触发时刻
	// 防止 AI 误解并递归创建新任务
	fullPrompt := fmt.Sprintf("%s%s", taskExecutionPromptPrefix, prompt)

	// 使用空的 session ID 触发创建新会话
	// 这确保每次任务执行都是独立的，不会受到之前对话的影响
	req := ChatRequest{
		SessionID: "", // 空 session ID 会触发创建新会话
		Message:   fullPrompt,
	}

	resp, err := a.agent.Chat(ctx, req)
	if err != nil {
		return "", err
	}

	return resp.Reply, nil
}
