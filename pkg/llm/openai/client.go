// Package openai 提供 OpenAI API 客户端实现
package openai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/KodaTao/AgentChassis/pkg/llm"
	"github.com/KodaTao/AgentChassis/pkg/observability"
)

// Provider OpenAI 提供商实现
type Provider struct {
	config     *Config
	httpClient *http.Client
}

// Config OpenAI 配置
type Config struct {
	APIKey      string
	BaseURL     string
	Model       string
	Timeout     time.Duration
	MaxTokens   int
	Temperature float64
}

// DefaultConfig 返回默认配置
func DefaultConfig() *Config {
	return &Config{
		BaseURL:     "https://api.openai.com/v1",
		Model:       "gpt-4",
		Timeout:     60 * time.Second,
		MaxTokens:   4096,
		Temperature: 0.7,
	}
}

// NewProvider 创建 OpenAI Provider
func NewProvider(cfg *Config) *Provider {
	if cfg == nil {
		cfg = DefaultConfig()
	}
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://api.openai.com/v1"
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 60 * time.Second
	}

	return &Provider{
		config: cfg,
		httpClient: &http.Client{
			Timeout: cfg.Timeout,
		},
	}
}

// NewProviderFromLLMConfig 从通用 LLM 配置创建 Provider
func NewProviderFromLLMConfig(cfg llm.Config) *Provider {
	return NewProvider(&Config{
		APIKey:      cfg.APIKey,
		BaseURL:     cfg.BaseURL,
		Model:       cfg.Model,
		Timeout:     time.Duration(cfg.Timeout) * time.Second,
		MaxTokens:   cfg.MaxTokens,
		Temperature: cfg.Temperature,
	})
}

// Name 返回提供商名称
func (p *Provider) Name() string {
	return "openai"
}

// Chat 发送对话请求
func (p *Provider) Chat(ctx context.Context, messages []llm.Message) (string, error) {
	start := time.Now()
	observability.LLMRequestLog(ctx, p.Name(), p.config.Model, len(messages))

	// 构建请求
	reqBody := chatRequest{
		Model:       p.config.Model,
		Messages:    convertMessages(messages),
		MaxTokens:   p.config.MaxTokens,
		Temperature: p.config.Temperature,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	// 创建 HTTP 请求
	req, err := http.NewRequestWithContext(ctx, "POST", p.config.BaseURL+"/chat/completions", bytes.NewReader(bodyBytes))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.config.APIKey)

	// 发送请求
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// 读取响应
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	// 检查状态码
	if resp.StatusCode != http.StatusOK {
		var errResp errorResponse
		json.Unmarshal(respBody, &errResp)
		return "", fmt.Errorf("API error (status %d): %s", resp.StatusCode, errResp.Error.Message)
	}

	// 解析响应
	var chatResp chatResponse
	if err := json.Unmarshal(respBody, &chatResp); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	if len(chatResp.Choices) == 0 {
		return "", fmt.Errorf("no choices in response")
	}

	content := chatResp.Choices[0].Message.Content
	duration := time.Since(start)

	// 记录响应日志
	observability.LLMResponseLog(ctx, p.Name(), duration.Milliseconds(), map[string]int{
		"prompt":     chatResp.Usage.PromptTokens,
		"completion": chatResp.Usage.CompletionTokens,
		"total":      chatResp.Usage.TotalTokens,
	})

	return content, nil
}

// ChatStream 发送流式对话请求
func (p *Provider) ChatStream(ctx context.Context, messages []llm.Message) (<-chan llm.StreamChunk, error) {
	observability.LLMRequestLog(ctx, p.Name(), p.config.Model, len(messages))

	// 构建请求
	reqBody := chatRequest{
		Model:       p.config.Model,
		Messages:    convertMessages(messages),
		MaxTokens:   p.config.MaxTokens,
		Temperature: p.config.Temperature,
		Stream:      true,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// 创建 HTTP 请求（流式不设置超时，由 context 控制）
	req, err := http.NewRequestWithContext(ctx, "POST", p.config.BaseURL+"/chat/completions", bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.config.APIKey)
	req.Header.Set("Accept", "text/event-stream")

	// 创建不带超时的 client（流式响应需要长时间保持连接）
	streamClient := &http.Client{}

	// 发送请求
	resp, err := streamClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	// 检查状态码
	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		var errResp errorResponse
		json.Unmarshal(respBody, &errResp)
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, errResp.Error.Message)
	}

	// 创建输出 channel
	ch := make(chan llm.StreamChunk, 100)

	// 启动 goroutine 处理流式响应
	go func() {
		defer close(ch)
		defer resp.Body.Close()

		reader := bufio.NewReader(resp.Body)
		for {
			select {
			case <-ctx.Done():
				ch <- llm.StreamChunk{Error: ctx.Err(), Done: true}
				return
			default:
			}

			line, err := reader.ReadString('\n')
			if err != nil {
				if err == io.EOF {
					ch <- llm.StreamChunk{Done: true}
					return
				}
				ch <- llm.StreamChunk{Error: err, Done: true}
				return
			}

			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}

			// SSE 格式：data: {...}
			if !strings.HasPrefix(line, "data: ") {
				continue
			}

			data := strings.TrimPrefix(line, "data: ")
			if data == "[DONE]" {
				ch <- llm.StreamChunk{Done: true}
				return
			}

			var streamResp streamResponse
			if err := json.Unmarshal([]byte(data), &streamResp); err != nil {
				continue
			}

			if len(streamResp.Choices) > 0 && streamResp.Choices[0].Delta.Content != "" {
				ch <- llm.StreamChunk{
					Content: streamResp.Choices[0].Delta.Content,
					Done:    false,
				}
			}
		}
	}()

	return ch, nil
}

// convertMessages 转换消息格式
func convertMessages(messages []llm.Message) []chatMessage {
	result := make([]chatMessage, len(messages))
	for i, m := range messages {
		result[i] = chatMessage{
			Role:    string(m.Role),
			Content: m.Content,
		}
	}
	return result
}

// API 请求/响应结构

type chatRequest struct {
	Model       string        `json:"model"`
	Messages    []chatMessage `json:"messages"`
	MaxTokens   int           `json:"max_tokens,omitempty"`
	Temperature float64       `json:"temperature,omitempty"`
	Stream      bool          `json:"stream,omitempty"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index   int `json:"index"`
		Message struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

type streamResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index int `json:"index"`
		Delta struct {
			Role    string `json:"role,omitempty"`
			Content string `json:"content,omitempty"`
		} `json:"delta"`
		FinishReason *string `json:"finish_reason"`
	} `json:"choices"`
}

type errorResponse struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
	} `json:"error"`
}
