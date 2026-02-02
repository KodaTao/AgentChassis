// Package chassis 提供 AgentChassis 核心框架
package chassis

import (
	"context"
	"fmt"
	"time"

	"github.com/KodaTao/AgentChassis/pkg/function"
	"github.com/KodaTao/AgentChassis/pkg/llm"
	"github.com/KodaTao/AgentChassis/pkg/observability"
	"github.com/KodaTao/AgentChassis/pkg/prompt"
	"github.com/KodaTao/AgentChassis/pkg/protocol"
)

// Agent AI 智能体
// 核心执行引擎，负责与 LLM 交互并调用 Function
type Agent struct {
	provider        llm.Provider
	registry        *function.Registry
	executor        *function.Executor
	sessionManager  *SessionManager
	parser          *protocol.Parser
	encoder         *protocol.Encoder
	promptGenerator *prompt.Generator
	config          *AgentConfig
}

// AgentConfig Agent 配置
type AgentConfig struct {
	MaxIterations int           // 最大迭代次数（防止无限循环）
	Timeout       time.Duration // 单次执行超时
}

// DefaultAgentConfig 返回默认 Agent 配置
func DefaultAgentConfig() *AgentConfig {
	return &AgentConfig{
		MaxIterations: 10,
		Timeout:       5 * time.Minute,
	}
}

// NewAgent 创建 Agent
func NewAgent(provider llm.Provider, registry *function.Registry, config *AgentConfig) *Agent {
	if config == nil {
		config = DefaultAgentConfig()
	}
	return &Agent{
		provider:        provider,
		registry:        registry,
		executor:        function.NewExecutor(registry, 30*time.Second),
		sessionManager:  NewSessionManager(nil),
		parser:          protocol.NewParser(),
		encoder:         protocol.NewEncoder(),
		promptGenerator: prompt.NewGenerator(),
		config:          config,
	}
}

// ChatRequest 对话请求
type ChatRequest struct {
	SessionID string `json:"session_id"`
	Message   string `json:"message"`
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

// Chat 处理对话请求
// 这是主要的交互入口，负责：
// 1. 获取/创建会话
// 2. 添加用户消息
// 3. 调用 LLM
// 4. 解析并执行 Function 调用
// 5. 循环直到 LLM 给出最终回复
func (a *Agent) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	// 生成或使用提供的 session ID
	sessionID := req.SessionID
	if sessionID == "" {
		sessionID = generateSessionID()
	}

	// 添加 session ID 到 context
	ctx = WithSessionID(ctx, sessionID)

	// 获取或创建会话
	session := a.sessionManager.GetOrCreate(sessionID)

	// 如果是新会话，添加系统提示
	if len(session.Messages) == 0 {
		systemPrompt, err := a.promptGenerator.GenerateSystemPrompt(a.registry.ListInfo())
		if err != nil {
			return nil, fmt.Errorf("failed to generate system prompt: %w", err)
		}
		session.AddMessage(llm.RoleSystem, systemPrompt)
	}

	// 添加用户消息
	session.AddMessage(llm.RoleUser, req.Message)

	// 执行对话循环
	var functionCalls []FunctionCall
	var finalReply string

	for i := 0; i < a.config.MaxIterations; i++ {
		// 调用 LLM
		observability.InfoContext(ctx, "Calling LLM", "iteration", i+1)

		reply, err := a.provider.Chat(ctx, session.GetMessages())
		if err != nil {
			return nil, fmt.Errorf("LLM call failed: %w", err)
		}

		// 添加 AI 回复到会话
		session.AddMessage(llm.RoleAssistant, reply)

		// 检查是否包含函数调用
		if !a.parser.HasCall(reply) {
			// 没有函数调用，这是最终回复
			finalReply = reply
			break
		}

		// 解析函数调用
		calls, err := a.parser.ParseCalls(reply)
		if err != nil {
			observability.WarnContext(ctx, "Failed to parse function calls", "error", err)
			finalReply = reply
			break
		}

		// 执行每个函数调用
		var results []string
		for _, call := range calls {
			observability.InfoContext(ctx, "Executing function", "name", call.Name)

			// 执行函数
			execResp := a.executor.Execute(ctx, function.ExecuteRequest{
				FunctionName: call.Name,
				Params:       call.Params,
				Data:         call.Data,
			})

			// 记录调用结果
			fc := FunctionCall{
				Name:   call.Name,
				Status: "success",
			}

			var resultStr string
			if execResp.Error != nil {
				fc.Status = "error"
				fc.Result = execResp.Error.Error()
				resultStr = a.encoder.EncodeError(call.Name, execResp.Error.Error())
			} else {
				fc.Result = execResp.Result.Message
				result := &protocol.CallResult{
					Name:     call.Name,
					Status:   protocol.StatusSuccess,
					Message:  execResp.Result.Message,
					Data:     execResp.Result.Data,
					Markdown: execResp.Result.Markdown,
				}
				resultStr, _ = a.encoder.EncodeResult(result)
			}

			functionCalls = append(functionCalls, fc)
			results = append(results, resultStr)
		}

		// 将函数结果添加到会话（作为用户消息，因为这是给 AI 看的）
		combinedResults := ""
		for _, r := range results {
			combinedResults += r + "\n"
		}
		session.AddMessage(llm.RoleUser, combinedResults)
	}

	// 提取 AI 回复中的纯文本部分（去掉函数调用）
	if finalReply == "" {
		finalReply = "I've completed the requested operations."
	} else {
		// 如果回复中包含函数调用，提取调用前后的文本
		textBefore := a.parser.ExtractTextBeforeCall(finalReply)
		textAfter := a.parser.ExtractTextAfterCall(finalReply)
		if textBefore != "" || textAfter != "" {
			finalReply = textBefore + " " + textAfter
		}
	}

	// 截断会话历史（防止 token 超限）
	session.Truncate(a.sessionManager.config.MaxHistory)

	return &ChatResponse{
		SessionID:     sessionID,
		Reply:         finalReply,
		FunctionCalls: functionCalls,
	}, nil
}

// ChatStream 流式对话（返回 channel）
func (a *Agent) ChatStream(ctx context.Context, req ChatRequest) (<-chan StreamResponse, error) {
	ch := make(chan StreamResponse, 100)

	go func() {
		defer close(ch)

		// 使用普通 Chat 处理（简化实现）
		// 后续可以改为真正的流式实现
		resp, err := a.Chat(ctx, req)
		if err != nil {
			ch <- StreamResponse{Error: err, Done: true}
			return
		}

		// 逐字符发送回复（模拟流式）
		for _, char := range resp.Reply {
			ch <- StreamResponse{Content: string(char)}
		}

		ch <- StreamResponse{
			SessionID:     resp.SessionID,
			FunctionCalls: resp.FunctionCalls,
			Done:          true,
		}
	}()

	return ch, nil
}

// StreamResponse 流式响应
type StreamResponse struct {
	SessionID     string         `json:"session_id,omitempty"`
	Content       string         `json:"content,omitempty"`
	FunctionCalls []FunctionCall `json:"function_calls,omitempty"`
	Error         error          `json:"error,omitempty"`
	Done          bool           `json:"done"`
}

// GetSession 获取会话
func (a *Agent) GetSession(id string) *Session {
	return a.sessionManager.Get(id)
}

// DeleteSession 删除会话
func (a *Agent) DeleteSession(id string) bool {
	return a.sessionManager.Delete(id)
}

// ListSessions 列出所有会话 ID
func (a *Agent) ListSessions() []string {
	return a.sessionManager.List()
}

// GetRegistry 获取函数注册表
func (a *Agent) GetRegistry() *function.Registry {
	return a.registry
}

// generateSessionID 生成会话 ID
func generateSessionID() string {
	return fmt.Sprintf("session_%d", time.Now().UnixNano())
}
