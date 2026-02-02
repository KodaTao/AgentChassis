// Package chassis 提供 AgentChassis 核心框架
package chassis

import (
	"fmt"

	"github.com/KodaTao/AgentChassis/pkg/function"
	"github.com/KodaTao/AgentChassis/pkg/llm"
	"github.com/KodaTao/AgentChassis/pkg/llm/openai"
	"github.com/KodaTao/AgentChassis/pkg/observability"
	"github.com/KodaTao/AgentChassis/pkg/storage"
)

// App AgentChassis 应用实例
// 这是整个框架的入口点
type App struct {
	config   *Config
	registry *function.Registry
	agent    *Agent
	provider llm.Provider
}

// New 创建新的 App 实例
func New(opts ...Option) *App {
	// 应用默认配置
	config := DefaultConfig()

	// 应用选项
	for _, opt := range opts {
		opt(config)
	}

	return &App{
		config:   config,
		registry: function.NewRegistry(),
	}
}

// Register 注册一个 Function
func (a *App) Register(fn function.Function) error {
	return a.registry.Register(fn)
}

// RegisterAll 批量注册 Functions
func (a *App) RegisterAll(fns ...function.Function) error {
	return a.registry.RegisterAll(fns...)
}

// Initialize 初始化应用
// 包括：日志、数据库、LLM Provider、Agent
func (a *App) Initialize() error {
	// 1. 初始化日志
	if err := observability.InitLogger(observability.LogConfig{
		Level:    a.config.Log.Level,
		Format:   a.config.Log.Format,
		Output:   a.config.Log.Output,
		FilePath: a.config.Log.FilePath,
	}); err != nil {
		return fmt.Errorf("failed to initialize logger: %w", err)
	}

	observability.Info("Initializing AgentChassis",
		"server_port", a.config.Server.Port,
		"llm_provider", a.config.LLM.Provider,
		"llm_model", a.config.LLM.Model,
	)

	// 2. 初始化数据库
	if err := storage.InitDB(storage.Config{
		Path: a.config.Database.Path,
	}); err != nil {
		return fmt.Errorf("failed to initialize database: %w", err)
	}

	// 3. 初始化 LLM Provider
	// 解析 API Key（支持环境变量）
	apiKey := llm.ResolveAPIKey(a.config.LLM.APIKey)
	if apiKey == "" {
		return fmt.Errorf("LLM API key is required")
	}

	// 根据 provider 类型创建实例
	switch a.config.LLM.Provider {
	case "openai", "azure", "custom":
		a.provider = openai.NewProviderFromLLMConfig(llm.Config{
			Provider:    a.config.LLM.Provider,
			APIKey:      apiKey,
			BaseURL:     a.config.LLM.BaseURL,
			Model:       a.config.LLM.Model,
			Timeout:     a.config.LLM.Timeout,
			MaxTokens:   a.config.LLM.MaxTokens,
			Temperature: a.config.LLM.Temperature,
		})
	default:
		return fmt.Errorf("unsupported LLM provider: %s", a.config.LLM.Provider)
	}

	observability.Info("LLM Provider initialized",
		"provider", a.provider.Name(),
		"model", a.config.LLM.Model,
		"api_key", llm.MaskAPIKey(apiKey),
	)

	// 4. 创建 Agent
	a.agent = NewAgent(a.provider, a.registry, nil)

	observability.Info("AgentChassis initialized",
		"registered_functions", a.registry.Count(),
	)

	return nil
}

// GetAgent 获取 Agent 实例
func (a *App) GetAgent() *Agent {
	return a.agent
}

// GetRegistry 获取函数注册表
func (a *App) GetRegistry() *function.Registry {
	return a.registry
}

// GetConfig 获取配置
func (a *App) GetConfig() *Config {
	return a.config
}

// GetProvider 获取 LLM Provider
func (a *App) GetProvider() llm.Provider {
	return a.provider
}

// Shutdown 关闭应用
func (a *App) Shutdown() error {
	observability.Info("Shutting down AgentChassis")

	// 关闭数据库
	if err := storage.Close(); err != nil {
		observability.Error("Failed to close database", "error", err)
		return err
	}

	observability.Info("AgentChassis shutdown complete")
	return nil
}
