# AgentChassis 需求规格说明书

## 1. 项目概述

### 1.1 项目定位
AgentChassis (AC) 是一个轻量级、可插拔的 Go 语言 AI Agent 框架，专注于以最省 Token 的方式让 AI 稳定地调用本地函数。

### 1.2 核心价值
- **极简扩展**：热插拔 Function，新增功能只需实现一个 Interface
- **XML + TOON 协议**：首个原生支持 `XML 嵌套 TOON` 的框架，比 JSON 更省 Token
- **跨平台分发**：编译成单一二进制文件（<20MB），支持 Linux/macOS/Windows
- **任务编排**：内置 Cron 定时任务，AI 可动态创建和管理定时任务

---

## 2. 技术需求

### 2.1 LLM 集成

#### 2.1.1 支持范围
| 优先级 | LLM 提供商 | 说明 |
|--------|-----------|------|
| P0 (初期) | OpenAI | 包括 GPT-4、GPT-3.5 等 |
| P0 (初期) | OpenAI 兼容 API | 如 Azure OpenAI、LocalAI、Ollama、vLLM 等 |
| P1 (后期) | Claude | Anthropic Claude API |
| P1 (后期) | 本地模型 | 通过兼容 API 支持 |

#### 2.1.2 配置项
```yaml
llm:
  provider: "openai"           # openai | azure | custom
  api_key: "${OPENAI_API_KEY}"
  base_url: "https://api.openai.com/v1"  # 可自定义 endpoint
  model: "gpt-4"
  timeout: 60s
  max_retries: 3
```

### 2.2 Function 接口设计

#### 2.2.1 核心接口
```go
// Function 是所有可调用函数的基础接口
type Function interface {
    // Name 返回函数的唯一标识符，AI 通过此名称调用
    Name() string

    // Description 返回函数描述，用于 AI 理解函数用途
    Description() string

    // Execute 执行函数，返回结果或错误
    // ctx 用于超时控制和取消
    // params 是通过反射解析的结构化参数
    Execute(ctx context.Context, params any) (Result, error)
}

// Result 函数执行结果
type Result struct {
    Data     any    // 结构化数据，将被编码为 TOON
    Markdown string // 可选的 Markdown 格式输出
    Message  string // 简短的文本消息
}
```

#### 2.2.2 参数 Schema 定义
- **使用 Go 反射机制**自动生成参数 Schema
- 通过 struct tag 定义参数元信息：
```go
type CleanLogsParams struct {
    Path      string `json:"path" desc:"要清理的目录路径" required:"true"`
    Days      int    `json:"days" desc:"保留最近N天的日志" default:"7"`
    DryRun    bool   `json:"dry_run" desc:"仅预览，不实际删除"`
}

func (f *FileCleaner) ParamsType() reflect.Type {
    return reflect.TypeOf(CleanLogsParams{})
}
```

#### 2.2.3 异步执行
- 所有 Function 执行都是异步的，通过 `context.Context` 控制
- 支持超时设置和手动取消
- 执行结果通过 channel 或 callback 返回

#### 2.2.4 错误处理
- Function 执行失败**不重试**
- 错误信息直接返回给 AI，让 AI 决定下一步操作
- 错误格式统一：
```xml
<error>
  <function>clean_logs</function>
  <message>permission denied: /var/log</message>
</error>
```

### 2.3 XML + TOON 协议规范

#### 2.3.1 协议设计原则
- **最外层使用 XML**：便于 AI 解析和生成
- **结构化多行数据使用 TOON**：显著节省 Token
- **Markdown 内容原样保留**：适合展示给用户

#### 2.3.2 AI 调用格式（AI → Agent）
```xml
<call name="function_name">
  <p>key1: value1</p>
  <p>key2: value2</p>
  <data type="toon">
items[3]{id,name,price}:
  1,Apple,2.5
  2,Banana,1.8
  3,Orange,3.0
  </data>
</call>
```

#### 2.3.3 Agent 响应格式（Agent → AI）
```xml
<result name="function_name" status="success">
  <message>操作完成</message>
  <data type="toon">
files[2]{name,size,deleted}:
  app.log,1024,true
  error.log,512,true
  </data>
  <output type="markdown">
## 清理结果
- 删除文件：2 个
- 释放空间：1.5 KB
  </output>
</result>
```

#### 2.3.4 错误响应格式
```xml
<result name="function_name" status="error">
  <error>permission denied: /var/log</error>
</result>
```

#### 2.3.5 TOON 协议参考
- 官方规范：https://github.com/toon-format/toon
- Go 实现：https://github.com/toon-format/toon-go
- 特点：
  - 表格数据使用 `[N]{field1,field2}:` 语法
  - 比 JSON 节省约 40% Token
  - 支持与 JSON 无损转换

### 2.4 定时任务系统

定时任务分为两种类型：
1. **DelayTask（一次性延时任务）**：在指定时间点执行一次
2. **CronTask（重复性定时任务）**：按 Cron 表达式周期执行

#### 2.4.1 DelayTask（一次性延时任务）

##### 功能要求
- AI 可通过内置 Function 动态创建/查询/取消延时任务
- 任务持久化到 SQLite，重启后自动恢复
- 已过期未执行的任务标记为 `missed` 状态
- 保留已完成任务的历史记录

##### 数据模型
```go
type DelayTask struct {
    gorm.Model
    Name         string     `gorm:"uniqueIndex;not null"` // 任务名称（唯一标识）
    RunAt        time.Time  `gorm:"not null;index"`       // 执行时间点（绝对时间）
    FunctionName string     `gorm:"not null"`             // 要执行的函数名
    Params       string     `gorm:"type:text"`            // 函数参数（JSON 格式）
    Status       string     `gorm:"default:pending"`      // pending/running/completed/failed/cancelled/missed
    Result       string     `gorm:"type:text"`            // 执行结果
    Error        string     `gorm:"type:text"`            // 错误信息
    ExecutedAt   *time.Time                               // 实际执行时间
}
```

##### 状态流转
```
pending -> running -> completed
                   -> failed
pending -> cancelled（手动取消）
pending -> missed（重启时已过期）
```

##### 内置 Function

**消息发送函数** - 通用外部通知函数，可直接调用或被延时任务调用
```go
// 发送消息
type SendMessageParams struct {
    To      string `json:"to" desc:"接收者（人名、邮箱、手机号等）" required:"true"`
    Message string `json:"message" desc:"消息内容" required:"true"`
    Channel string `json:"channel" desc:"通知渠道：console（默认）/email/sms/wechat" default:"console"`
}
```

**延时任务管理函数**
```go
// 创建延时任务
type DelayCreateParams struct {
    Name     string `json:"name" desc:"任务名称，唯一标识" required:"true"`
    RunAt    string `json:"run_at" desc:"执行时间，ISO8601格式，如 2024-12-25T09:00:00+08:00" required:"true"`
    Function string `json:"function" desc:"要执行的函数名" required:"true"`
    Params   string `json:"params" desc:"函数参数，JSON格式"`
}

// 列出延时任务
type DelayListParams struct {
    Status string `json:"status" desc:"按状态筛选：pending/completed/failed/cancelled/missed"`
}

// 取消延时任务
type DelayCancelParams struct {
    Name string `json:"name" desc:"要取消的任务名称" required:"true"`
}
```

##### 重启恢复逻辑
1. 启动时查询所有 `pending` 状态的任务
2. 检查 `run_at` 是否已过期：
   - 已过期：更新状态为 `missed`
   - 未过期：使用 `time.AfterFunc` 重新注册

#### 2.4.2 CronTask（重复性定时任务）

##### 功能要求
- AI 可通过内置 Function 动态创建/查询/删除定时任务
- 任务持久化到 SQLite，重启后自动恢复
- 支持失效时间（until）和最大执行次数（max_runs）

##### 数据模型
```go
type CronTask struct {
    gorm.Model
    Name         string     `gorm:"uniqueIndex;not null"` // 任务名称
    CronExpr     string     `gorm:"not null"`             // Cron 表达式
    FunctionName string     `gorm:"not null"`             // 要执行的函数
    Params       string     `gorm:"type:text"`            // 参数（JSON 格式）
    Enabled      bool       `gorm:"default:true"`         // 是否启用
    Until        *time.Time                               // 失效时间（可选）
    MaxRuns      int        `gorm:"default:0"`            // 最大执行次数，0表示无限
    RunCount     int        `gorm:"default:0"`            // 已执行次数
    LastRunAt    *time.Time                               // 最后执行时间
    LastStatus   string                                   // 最后执行状态
}
```

##### 内置 Function
```go
// 创建定时任务
type CronCreateParams struct {
    Name     string `json:"name" desc:"任务名称" required:"true"`
    CronExpr string `json:"cron_expr" desc:"Cron 表达式，如 '0 9 * * *'" required:"true"`
    Function string `json:"function" desc:"要执行的函数名" required:"true"`
    Params   string `json:"params" desc:"函数参数（JSON 格式）"`
    Until    string `json:"until" desc:"失效时间，ISO8601格式（可选）"`
    MaxRuns  int    `json:"max_runs" desc:"最大执行次数，0表示无限（可选）"`
}

// 列出定时任务
type CronListParams struct {
    Name string `json:"name" desc:"按名称筛选（可选）"`
}

// 删除定时任务
type CronDeleteParams struct {
    Name string `json:"name" desc:"要删除的任务名称" required:"true"`
}
```

#### 2.4.3 持久化方案
- 使用 **GORM + SQLite**（`~/.agentchassis/data.db`）
- DelayTask 和 CronTask 使用独立的表

#### 2.4.4 REST API
```
# 延时任务
GET    /api/v1/delays            # 获取所有延时任务
POST   /api/v1/delays            # 创建延时任务
DELETE /api/v1/delays/:name      # 取消延时任务

# 定时任务
GET    /api/v1/crons             # 获取所有定时任务
POST   /api/v1/crons             # 创建定时任务
DELETE /api/v1/crons/:name       # 删除定时任务
```

### 2.5 可观测性

#### 2.5.1 日志记录
- 使用 `slog`（Go 1.21+ 标准库）作为日志框架
- 支持 JSON 和 Text 两种输出格式
- 日志级别：DEBUG、INFO、WARN、ERROR
- 关键日志点：
  - LLM 请求/响应
  - Function 调用/结果
  - Cron 任务触发/执行
  - 错误和异常

#### 2.5.2 指标（Metrics）
- 使用 Prometheus 格式暴露指标
- 核心指标：
  - `ac_llm_requests_total`：LLM 请求总数
  - `ac_llm_request_duration_seconds`：LLM 请求延迟
  - `ac_llm_tokens_total`：Token 使用量
  - `ac_function_calls_total`：Function 调用总数
  - `ac_function_duration_seconds`：Function 执行延迟
  - `ac_cron_executions_total`：Cron 任务执行次数

#### 2.5.3 链路追踪（Tracing）
- 支持 OpenTelemetry 标准
- 每次对话生成唯一 TraceID
- Span 覆盖：
  - 完整对话流程
  - 单次 LLM 调用
  - 单次 Function 执行

---

## 3. 系统架构

### 3.1 核心模块
```
┌─────────────────────────────────────────────────────────────┐
│                      AgentChassis                            │
├─────────────────────────────────────────────────────────────┤
│  ┌─────────┐  ┌─────────┐  ┌─────────┐  ┌─────────────────┐ │
│  │   CLI   │  │  HTTP   │  │  REPL   │  │  Embedded SDK   │ │
│  │ Runner  │  │ Server  │  │  Mode   │  │    (库模式)     │ │
│  └────┬────┘  └────┬────┘  └────┬────┘  └────────┬────────┘ │
│       └────────────┴────────────┴────────────────┘          │
│                           │                                  │
│  ┌────────────────────────▼────────────────────────────┐    │
│  │                   Agent Core                         │    │
│  │  ┌──────────┐  ┌──────────┐  ┌────────────────────┐ │    │
│  │  │ Executor │  │ Protocol │  │  Context Manager   │ │    │
│  │  │  Loop    │  │  Parser  │  │  (对话历史管理)    │ │    │
│  │  └──────────┘  └──────────┘  └────────────────────┘ │    │
│  └─────────────────────────────────────────────────────┘    │
│                           │                                  │
│       ┌───────────────────┼───────────────────┐             │
│       ▼                   ▼                   ▼             │
│  ┌─────────┐       ┌─────────────┐     ┌───────────┐        │
│  │   LLM   │       │  Function   │     │   Cron    │        │
│  │ Adapter │       │  Registry   │     │ Scheduler │        │
│  └─────────┘       └─────────────┘     └───────────┘        │
│       │                   │                   │              │
│       ▼                   ▼                   ▼              │
│  ┌─────────┐       ┌─────────────┐     ┌───────────┐        │
│  │ OpenAI  │       │  Built-in   │     │  SQLite   │        │
│  │ Claude  │       │   Custom    │     │  Storage  │        │
│  │  ...    │       │  Functions  │     │           │        │
│  └─────────┘       └─────────────┘     └───────────┘        │
├─────────────────────────────────────────────────────────────┤
│                    Observability Layer                       │
│  ┌──────────┐  ┌────────────────┐  ┌──────────────────┐     │
│  │   slog   │  │   Prometheus   │  │  OpenTelemetry   │     │
│  │  Logger  │  │    Metrics     │  │     Tracing      │     │
│  └──────────┘  └────────────────┘  └──────────────────┘     │
└─────────────────────────────────────────────────────────────┘
```

### 3.2 运行模式
| 模式 | 说明 | 使用场景 | 优先级 |
|------|------|----------|--------|
| HTTP Server | 提供 REST API | Web 控制台集成 | P0 (初期) |
| CLI Runner | 命令行执行单次任务 | 脚本自动化 | P1 (后期) |
| REPL Mode | 交互式命令行 | 开发调试 | P1 (后期) |
| Embedded SDK | 作为库嵌入其他应用 | 二次开发 | P2 (后期) |

### 3.3 REST API 设计（初期）

#### 3.3.1 对话接口
```
POST /api/v1/chat
Content-Type: application/json

{
  "session_id": "optional-session-id",  // 可选，用于多轮对话
  "message": "用户输入的消息"
}

Response:
{
  "session_id": "uuid",
  "reply": "AI 的回复",
  "function_calls": [
    {
      "name": "clean_logs",
      "status": "success",
      "result": "清理了 5 个文件"
    }
  ]
}
```

#### 3.3.2 Function 管理接口
```
GET  /api/v1/functions          # 获取已注册的 Function 列表
GET  /api/v1/functions/:name    # 获取单个 Function 详情
```

#### 3.3.3 Cron 管理接口
```
GET    /api/v1/crons            # 获取所有定时任务
POST   /api/v1/crons            # 创建定时任务（也可通过 AI 创建）
DELETE /api/v1/crons/:name      # 删除定时任务
```

#### 3.3.4 健康检查
```
GET /health                     # 健康检查
GET /metrics                    # Prometheus 指标（后期）
```

---

## 4. 目录结构规划

```
AgentChassis/
├── cmd/
│   └── agent/
│       └── main.go              # CLI 入口
├── pkg/
│   ├── chassis/                 # 核心框架
│   │   ├── app.go               # 应用入口
│   │   ├── options.go           # 配置选项
│   │   └── context.go           # 执行上下文
│   ├── llm/                     # LLM 适配层
│   │   ├── provider.go          # Provider 接口
│   │   ├── openai/              # OpenAI 实现
│   │   └── config.go            # LLM 配置
│   ├── function/                # Function 管理
│   │   ├── registry.go          # 函数注册表
│   │   ├── interface.go         # 函数接口定义
│   │   ├── schema.go            # 参数 Schema 生成（反射）
│   │   └── builtin/             # 内置函数
│   │       └── cron.go          # Cron 管理函数
│   ├── protocol/                # 协议解析
│   │   ├── parser.go            # XML + TOON 解析器
│   │   ├── encoder.go           # 响应编码器
│   │   └── prompt.go            # System Prompt 生成
│   ├── cron/                    # 定时任务
│   │   ├── scheduler.go         # 调度器
│   │   └── storage.go           # 持久化
│   └── observability/           # 可观测性
│       ├── logger.go            # 日志
│       ├── metrics.go           # 指标
│       └── tracing.go           # 链路追踪
├── internal/                    # 内部实现
│   └── util/                    # 工具函数
├── examples/                    # 示例
│   └── file_cleaner/
├── configs/                     # 配置文件示例
│   └── config.example.yaml
├── CLAUDE.md                    # 开发指南
├── REQUIREMENT.md               # 本文档
├── TODO.md                      # 开发计划
├── README.md                    # 项目介绍
├── go.mod
└── go.sum
```

---

## 5. 依赖选型

| 功能 | 库 | 说明 |
|------|-----|------|
| TOON 解析 | `github.com/toon-format/toon-go` | 官方 Go 实现 |
| HTTP Client | `net/http` | 标准库 |
| HTTP Server | `github.com/gin-gonic/gin` | 高性能 Web 框架 |
| ORM | `gorm.io/gorm` | Go ORM 框架 |
| SQLite 驱动 | `gorm.io/driver/sqlite` | GORM SQLite 驱动 |
| Cron 调度 | `github.com/robfig/cron/v3` | 成熟的 Cron 库 |
| 日志 | `log/slog` | Go 1.21+ 标准库 |
| 指标 | `github.com/prometheus/client_golang` | Prometheus SDK |
| 链路追踪 | `go.opentelemetry.io/otel` | OpenTelemetry SDK |
| 配置管理 | `github.com/spf13/viper` | 配置文件解析（YAML） |
| CLI | `github.com/spf13/cobra` | 命令行框架 |

---

## 6. 非功能需求

### 6.1 性能目标
- Function 调用延迟：< 10ms（不含 LLM 时间）
- 内存占用：< 50MB（空载）
- 并发支持：至少 100 个并发对话

### 6.2 兼容性
- Go 版本：1.21+
- 操作系统：Linux、macOS、Windows
- 架构：amd64、arm64

### 6.3 安全性
- API Key 不在日志中明文打印
- 支持环境变量配置敏感信息
- Function 执行有超时保护

---

## 7. 待确认事项

- [x] LLM 支持范围（初期 OpenAI）
- [x] 参数 Schema 方案（Go 反射）
- [x] TOON 协议细节（XML 包裹 TOON）
- [x] Cron 持久化方案（SQLite）
- [x] 可观测性需求（日志+指标+链路追踪）

---

## 更新记录

| 日期 | 版本 | 说明 |
|------|------|------|
| 2024-XX-XX | v0.1 | 初始版本，确定核心需求 |
| 2024-XX-XX | v0.2 | 完善定时任务系统设计：区分 DelayTask（一次性）和 CronTask（重复性） |
