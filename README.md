# AgentChassis

**轻量级、可插拔的 Go 语言 AI Agent 框架**

AgentChassis 是一个专为 Golang 开发者设计的智能体基座。它专注于解决一件事：**如何以最省 Token 的方式，让 AI 稳定地调用你的本地函数。**

---

## 特性

- **极简扩展**：热插拔 Function，新增功能只需实现一个 Interface
- **XML + TOON 协议**：首个原生支持 `XML 嵌套 TOON` 的框架，比 JSON 更省 Token
- **跨平台分发**：编译后是一个不到 20MB 的二进制文件，支持 Linux/macOS/Windows
- **任务编排**：内置延时任务和 Cron 定时任务，AI 可动态创建和管理
- **多渠道支持**：支持 REST API 和 Telegram Bot 作为交互入口

---

## 交互截图

![Telegram截图1.png](images/Telegram%E6%88%AA%E5%9B%BE1.png)

---

## 快速开始

### 安装

```bash
go get github.com/KodaTao/AgentChassis
```

### 最小示例

```go
package main

import (
    "context"
    "fmt"
    "reflect"

    "github.com/KodaTao/AgentChassis/pkg/chassis"
    "github.com/KodaTao/AgentChassis/pkg/function"
    "github.com/KodaTao/AgentChassis/pkg/llm"
    "github.com/KodaTao/AgentChassis/pkg/server"
)

// 1. 定义参数结构体
type GreetParams struct {
    Name string `json:"name" desc:"要问候的人名" required:"true"`
}

// 2. 实现 Function 接口
type GreetFunction struct{}

func (f *GreetFunction) Name() string        { return "greet" }
func (f *GreetFunction) Description() string { return "向指定的人打招呼" }
func (f *GreetFunction) ParamsType() reflect.Type { return reflect.TypeOf(GreetParams{}) }

func (f *GreetFunction) Execute(ctx context.Context, params any) (function.Result, error) {
    p := params.(GreetParams)
    return function.Result{
        Message: fmt.Sprintf("你好，%s！", p.Name),
    }, nil
}

func main() {
    // 3. 创建应用
    app := chassis.New(
        chassis.WithServerPort(8080),
        chassis.WithLLMConfig(llm.Config{
            Provider: "openai",
            APIKey:   "${OPENAI_API_KEY}",  // 支持环境变量
            Model:    "gpt-4",
        }),
    )

    // 4. 注册 Function
    app.Register(&GreetFunction{})

    // 5. 初始化并启动
    app.Initialize()

    srv := server.NewServer(app, &server.ServerConfig{Port: 8080})
    srv.Run()
}
```

### 或运行 Agent

```bash
go run cmd/agent/main.go serve \
  --config configs/config.yaml # 你的配置文件路径
```

### 测试调用

```bash
curl -X POST http://localhost:8080/api/v1/chat \
     -H "Content-Type: application/json" \
     -d '{"message": "请向张三打个招呼"}'
```

---

## 配置

### 配置文件

创建 `configs/config.yaml`：

```yaml
# 服务器配置
server:
  host: "0.0.0.0"
  port: 8080

# LLM 配置
llm:
  provider: "openai"
  api_key: "${OPENAI_API_KEY}"  # 支持环境变量
  base_url: "https://api.openai.com/v1"
  model: "gpt-4"
  timeout: 60
  max_tokens: 4096
  temperature: 0.7

# 数据库配置
database:
  path: "~/.agentchassis/data.db"

# 日志配置
log:
  level: "info"    # debug, info, warn, error
  format: "text"   # text, json

# Telegram Bot 配置（可选）
telegram:
  enabled: false
  token: "${TELEGRAM_BOT_TOKEN}"
  session_ttl: "24h"
```

### 环境变量

```bash
export OPENAI_API_KEY="your-api-key"
export OPENAI_BASE_URL="https://api.openai.com/v1"  # 可选，支持自定义 endpoint
export TELEGRAM_BOT_TOKEN="your-bot-token"          # 可选，启用 Telegram Bot
```

---

## 核心概念

### Function 接口

所有可被 AI 调用的函数都需要实现 `Function` 接口：

```go
type Function interface {
    Name() string                                              // 函数名称
    Description() string                                       // 函数描述
    ParamsType() reflect.Type                                  // 参数类型
    Execute(ctx context.Context, params any) (Result, error)   // 执行逻辑
}
```

### 参数定义

使用 struct tag 定义参数元信息：

```go
type CleanLogsParams struct {
    Path   string `json:"path" desc:"要清理的目录路径" required:"true"`
    Days   int    `json:"days" desc:"保留最近N天的日志" default:"7"`
    DryRun bool   `json:"dry_run" desc:"仅预览，不实际删除"`
}
```

支持的 tag：
- `desc`: 参数描述（给 AI 看）
- `required`: 是否必填
- `default`: 默认值

---

## 内置功能

### 延时任务

AI 可以创建一次性延时任务：

```
用户: "1小时后提醒我开会"
AI: 好的，我已创建延时任务，将在 1 小时后提醒您开会。
```

内置 Function：
- `delay_create` - 创建延时任务
- `delay_list` - 列出任务
- `delay_cancel` - 取消任务
- `delay_get` - 获取任务详情

### Cron 定时任务

AI 可以创建周期性定时任务：

```
用户: "每天早上9点提醒我喝水"
AI: 好的，我已创建定时任务，将在每天早上 9 点提醒您喝水。
```

内置 Function：
- `cron_create` - 创建定时任务
- `cron_list` - 列出任务
- `cron_delete` - 删除任务
- `cron_get` - 获取任务详情
- `cron_history` - 查看执行历史

### 消息通知

支持多渠道消息发送：

```go
// 控制台输出（默认）
send_message(to: "张三", message: "开会了", channel: "console")

// Telegram 消息
send_message(to: "123456789", message: "开会了", channel: "telegram")
```

---

## Telegram Bot

AgentChassis 支持 Telegram Bot 作为交互入口，采用**纯 Reply 机制**管理多会话：

- **新消息** = 新对话
- **Reply 消息** = 继续对应的对话
- 支持并行多个独立对话

### 启用 Telegram Bot

1. 从 @BotFather 获取 Bot Token
2. 配置环境变量或配置文件：

```bash
export TELEGRAM_BOT_TOKEN="your-bot-token"
```

```yaml
telegram:
  enabled: true
  token: "${TELEGRAM_BOT_TOKEN}"
  session_ttl: "24h"
```

3. 启动应用，Bot 会自动运行

---

## REST API

### 对话接口

```
POST /api/v1/chat
Content-Type: application/json

{
  "session_id": "optional-session-id",
  "message": "用户输入的消息"
}
```

响应：
```json
{
  "session_id": "uuid",
  "reply": "AI 的回复",
  "function_calls": [
    {
      "name": "greet",
      "status": "success",
      "result": "你好，张三！"
    }
  ]
}
```

### Function 管理

```
GET /api/v1/functions         # 列出所有 Function
GET /api/v1/functions/:name   # 获取 Function 详情
```

### 延时任务管理

```
GET    /api/v1/delay-tasks      # 列出任务
POST   /api/v1/delay-tasks      # 创建任务
GET    /api/v1/delay-tasks/:id  # 获取详情
DELETE /api/v1/delay-tasks/:id  # 取消任务
```

### Cron 任务管理

```
GET    /api/v1/crons              # 列出任务
POST   /api/v1/crons              # 创建任务
GET    /api/v1/crons/:id          # 获取详情
DELETE /api/v1/crons/:id          # 删除任务
GET    /api/v1/crons/:id/history  # 执行历史
```

### 健康检查

```
GET /health
```

---

## 项目结构

```
AgentChassis/
├── cmd/agent/           # CLI 入口
├── pkg/
│   ├── chassis/         # 核心框架（App、Agent）
│   ├── llm/             # LLM 适配层（OpenAI）
│   ├── function/        # Function 系统
│   │   └── builtin/     # 内置 Function
│   ├── protocol/        # XML + TOON 协议
│   ├── scheduler/       # 任务调度器
│   ├── telegram/        # Telegram Bot
│   ├── server/          # HTTP Server
│   ├── storage/         # 数据持久化
│   ├── prompt/          # System Prompt 生成
│   ├── observability/   # 日志
│   └── types/           # 共享类型
├── examples/            # 示例代码
└── configs/             # 配置文件
```

---

## 运行示例

```bash
# 克隆项目
git clone https://github.com/KodaTao/AgentChassis.git
cd AgentChassis

# 设置环境变量
export OPENAI_API_KEY="your-api-key"

# 运行示例
go run examples/hello/main.go
```

---

## 开源协议

本项目采用 **Apache 2.0** 协议。

---

## 贡献

欢迎提交 Issue 和 Pull Request！
