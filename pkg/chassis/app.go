// Package chassis 提供 AgentChassis 核心框架
package chassis

import (
	"fmt"
	"log/slog"

	"github.com/KodaTao/AgentChassis/pkg/function"
	"github.com/KodaTao/AgentChassis/pkg/function/builtin"
	"github.com/KodaTao/AgentChassis/pkg/llm"
	"github.com/KodaTao/AgentChassis/pkg/llm/openai"
	"github.com/KodaTao/AgentChassis/pkg/observability"
	"github.com/KodaTao/AgentChassis/pkg/scheduler"
	"github.com/KodaTao/AgentChassis/pkg/storage"
	"github.com/KodaTao/AgentChassis/pkg/telegram"
)

// App AgentChassis 应用实例
// 这是整个框架的入口点
type App struct {
	config              *Config
	registry            *function.Registry
	agent               *Agent
	provider            llm.Provider
	delayScheduler      *scheduler.DelayScheduler
	cronScheduler       *scheduler.CronScheduler
	telegramBot         *telegram.Bot
	sendMessageFunction *builtin.SendMessageFunction // 保存引用以便后续注入 Telegram 发送器
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

	// 4. 初始化 DelayScheduler（此时还没有 AgentExecutor，后续设置）
	db := storage.GetDB()
	logger := slog.Default()
	a.delayScheduler = scheduler.NewDelayScheduler(db, logger)
	if err := a.delayScheduler.Start(); err != nil {
		return fmt.Errorf("failed to start delay scheduler: %w", err)
	}

	observability.Info("DelayScheduler started")

	// 5. 初始化 CronScheduler（此时还没有 AgentExecutor，后续设置）
	a.cronScheduler = scheduler.NewCronScheduler(db, logger)
	if err := a.cronScheduler.Start(); err != nil {
		return fmt.Errorf("failed to start cron scheduler: %w", err)
	}

	observability.Info("CronScheduler started")

	// 6. 注册内置调度函数
	a.registerBuiltinSchedulerFunctions()

	// 7. 创建 Agent
	a.agent = NewAgent(a.provider, a.registry, nil)

	// 8. 设置 AgentExecutor 到调度器（解决循环依赖）
	// Agent 创建完成后，将其适配为 AgentExecutor 并注入到调度器
	executor := NewAgentExecutorAdapter(a.agent)
	a.delayScheduler.SetAgentExecutor(executor)
	a.cronScheduler.SetAgentExecutor(executor)

	observability.Info("AgentExecutor injected to schedulers")

	observability.Info("AgentChassis initialized",
		"registered_functions", a.registry.Count(),
	)

	// 9. 初始化 Telegram Bot（可选）
	if a.config.Telegram.Enabled {
		if err := a.initTelegramBot(); err != nil {
			return fmt.Errorf("failed to initialize telegram bot: %w", err)
		}
	}

	return nil
}

// initTelegramBot 初始化 Telegram Bot
func (a *App) initTelegramBot() error {
	logger := slog.Default()

	botConfig := telegram.Config{
		Enabled:    a.config.Telegram.Enabled,
		Token:      a.config.Telegram.Token,
		SessionTTL: a.config.Telegram.SessionTTL,
	}

	bot, err := telegram.NewBot(botConfig, a.agent, logger)
	if err != nil {
		return err
	}

	a.telegramBot = bot

	// 将 Telegram Bot 注入到 SendMessageFunction
	if a.sendMessageFunction != nil {
		a.sendMessageFunction.SetTelegramSender(bot)
		observability.Info("Telegram sender injected to SendMessageFunction")
	}

	// 启动 Bot（异步接收消息）
	bot.Start()

	observability.Info("Telegram Bot started")
	return nil
}

// registerBuiltinSchedulerFunctions 注册内置函数
func (a *App) registerBuiltinSchedulerFunctions() {
	// 注册消息发送函数（通用的外部通知函数，可直接调用或被延时任务调用）
	// 保存引用以便后续注入 Telegram 发送器
	a.sendMessageFunction = builtin.NewSendMessageFunction()
	_ = a.registry.Register(a.sendMessageFunction)

	// 注册延时任务管理函数
	_ = a.registry.Register(builtin.NewDelayCreateFunction(a.delayScheduler))
	_ = a.registry.Register(builtin.NewDelayListFunction(a.delayScheduler))
	_ = a.registry.Register(builtin.NewDelayCancelFunction(a.delayScheduler))
	_ = a.registry.Register(builtin.NewDelayGetFunction(a.delayScheduler))

	// 注册定时任务管理函数
	_ = a.registry.Register(builtin.NewCronCreateFunction(a.cronScheduler))
	_ = a.registry.Register(builtin.NewCronListFunction(a.cronScheduler))
	_ = a.registry.Register(builtin.NewCronDeleteFunction(a.cronScheduler))
	_ = a.registry.Register(builtin.NewCronGetFunction(a.cronScheduler))
	_ = a.registry.Register(builtin.NewCronHistoryFunction(a.cronScheduler))

	observability.Info("Registered builtin functions",
		"delay_functions", []string{"send_message", "delay_create", "delay_list", "delay_cancel", "delay_get"},
		"cron_functions", []string{"cron_create", "cron_list", "cron_delete", "cron_get", "cron_history"},
	)
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

	// 停止 Telegram Bot
	if a.telegramBot != nil {
		a.telegramBot.Stop()
		observability.Info("Telegram Bot stopped")
	}

	// 停止调度器
	if a.delayScheduler != nil {
		a.delayScheduler.Stop()
	}
	if a.cronScheduler != nil {
		a.cronScheduler.Stop()
	}

	// 关闭数据库
	if err := storage.Close(); err != nil {
		observability.Error("Failed to close database", "error", err)
		return err
	}

	observability.Info("AgentChassis shutdown complete")
	return nil
}

// GetDelayScheduler 获取延时任务调度器
func (a *App) GetDelayScheduler() *scheduler.DelayScheduler {
	return a.delayScheduler
}

// GetCronScheduler 获取定时任务调度器
func (a *App) GetCronScheduler() *scheduler.CronScheduler {
	return a.cronScheduler
}

// GetTelegramBot 获取 Telegram Bot 实例
func (a *App) GetTelegramBot() *telegram.Bot {
	return a.telegramBot
}
