// Package main 是 AgentChassis 的 CLI 入口
package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/KodaTao/AgentChassis/pkg/chassis"
	"github.com/KodaTao/AgentChassis/pkg/observability"
	"github.com/KodaTao/AgentChassis/pkg/server"
)

var (
	cfgFile string
	app     *chassis.App
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "agent",
		Short: "AgentChassis - Lightweight AI Agent Framework",
		Long: `AgentChassis is a lightweight, pluggable AI Agent framework for Go.
It focuses on efficient Function calling with minimal token usage.`,
	}

	// 全局 flags
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is ./config.yaml)")

	// 添加子命令
	rootCmd.AddCommand(serveCmd())
	rootCmd.AddCommand(versionCmd())

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// serveCmd 启动 HTTP 服务器
func serveCmd() *cobra.Command {
	var port int
	var host string

	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the HTTP server",
		Long:  `Start the AgentChassis HTTP server to handle API requests.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// 加载配置
			config, err := loadConfig()
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			// 命令行参数覆盖配置
			if port != 0 {
				config.Server.Port = port
			}
			if host != "" {
				config.Server.Host = host
			}

			// 创建应用
			app = chassis.New(
				chassis.WithServerPort(config.Server.Port),
				chassis.WithServerMode(config.Server.Mode),
				chassis.WithLLMConfig(config.LLM),
				chassis.WithLogLevel(config.Log.Level),
				chassis.WithDatabasePath(config.Database.Path),
				chassis.WithTelegram(config.Telegram),
			)

			// 初始化
			if err := app.Initialize(); err != nil {
				return fmt.Errorf("failed to initialize: %w", err)
			}

			// 创建 HTTP 服务器
			srv := server.NewServer(app, &server.ServerConfig{
				Host: config.Server.Host,
				Port: config.Server.Port,
				Mode: config.Server.Mode,
			})

			// 优雅关闭
			go func() {
				sigCh := make(chan os.Signal, 1)
				signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
				<-sigCh

				observability.Info("Received shutdown signal")
				app.Shutdown()
				os.Exit(0)
			}()

			// 启动服务器
			return srv.Run()
		},
	}

	cmd.Flags().IntVarP(&port, "port", "p", 0, "Server port (default 8080)")
	cmd.Flags().StringVarP(&host, "host", "H", "", "Server host (default 0.0.0.0)")

	return cmd
}

// versionCmd 显示版本信息
func versionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("AgentChassis v0.1.0")
			fmt.Println("The Lightweight, Pluggable Agent Framework for Go")
		},
	}
}

// loadConfig 加载配置文件
func loadConfig() (*chassis.Config, error) {
	v := viper.New()

	// 设置默认值
	v.SetDefault("server.host", "0.0.0.0")
	v.SetDefault("server.port", 8080)
	v.SetDefault("server.mode", "debug")

	v.SetDefault("llm.provider", "openai")
	v.SetDefault("llm.base_url", "https://api.openai.com/v1")
	v.SetDefault("llm.model", "gpt-4")
	v.SetDefault("llm.timeout", 60)
	v.SetDefault("llm.max_tokens", 4096)
	v.SetDefault("llm.temperature", 0.7)

	v.SetDefault("database.path", "~/.agentchassis/data.db")

	v.SetDefault("log.level", "info")
	v.SetDefault("log.format", "text")
	v.SetDefault("log.output", "stdout")

	// 配置文件
	if cfgFile != "" {
		v.SetConfigFile(cfgFile)
	} else {
		v.SetConfigName("config")
		v.SetConfigType("yaml")
		v.AddConfigPath(".")
		v.AddConfigPath("./configs")
		v.AddConfigPath("$HOME/.agentchassis")
	}

	// 环境变量
	v.SetEnvPrefix("AC")
	v.AutomaticEnv()

	// 读取配置文件（如果存在）
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, err
		}
		// 配置文件不存在时使用默认值
	}

	// 解析配置
	config := &chassis.Config{}
	if err := v.Unmarshal(config); err != nil {
		return nil, err
	}

	return config, nil
}
