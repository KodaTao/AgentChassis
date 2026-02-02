# AgentChassis 开发计划

## 阶段一：基础架构 (Foundation) ✅

### 1.1 项目初始化
- [x] 初始化 Go module，配置 go.mod
- [x] 创建目录结构
- [x] 添加基础依赖（toon-go, gin, gorm, viper 等）
- [x] 创建配置文件示例 `configs/config.example.yaml`

### 1.2 核心接口定义
- [x] 定义 `Function` 接口 (`pkg/function/interface.go`)
- [x] 定义 `Result` 结构体
- [x] 定义 `LLMProvider` 接口 (`pkg/llm/provider.go`)
- [x] 定义配置结构体 (`pkg/chassis/options.go`)

### 1.3 日志系统
- [x] 封装 slog 日志器 (`pkg/observability/logger.go`)
- [x] 支持 JSON/Text 格式切换
- [x] 支持日志级别配置

### 1.4 数据库初始化
- [x] 配置 GORM + SQLite (`pkg/storage/db.go`)
- [x] 实现数据库自动迁移
- [x] 定义基础 Model

---

## 阶段二：协议层 (Protocol) ✅

### 2.1 XML + TOON 解析器
- [x] 实现 XML 调用解析 (`pkg/protocol/parser.go`)
- [x] 集成 toon-go 解析 TOON 内容
- [x] 支持 `<call>`, `<p>`, `<data>` 标签解析
- [x] 单元测试

### 2.2 响应编码器
- [x] 实现 `<result>` 响应编码 (`pkg/protocol/encoder.go`)
- [x] 支持 TOON 数据编码
- [x] 支持 Markdown 输出
- [x] 支持错误响应编码
- [x] 单元测试

### 2.3 System Prompt 生成器
- [x] 根据已注册 Function 生成 System Prompt (`pkg/prompt/prompt.go`)
- [x] 包含协议格式说明
- [x] 包含 Function 列表和参数说明
- [x] 提示词模板单独管理 (`pkg/prompt/templates/`)

---

## 阶段三：Function 系统 (Function) ✅

### 3.1 函数注册表
- [x] 实现 `Registry` 结构 (`pkg/function/registry.go`)
- [x] 支持注册/获取/列出 Function
- [x] 线程安全设计

### 3.2 参数 Schema 生成
- [x] 使用反射生成参数 Schema (`pkg/function/schema.go`)
- [x] 支持 struct tag: `desc`, `required`, `default`
- [x] 生成 AI 可读的参数描述
- [x] 单元测试

### 3.3 函数执行器
- [x] 实现带 Context 的异步执行 (`pkg/function/executor.go`)
- [x] 超时控制
- [x] 错误捕获和格式化

---

## 阶段四：LLM 集成 (LLM) ✅

### 4.1 OpenAI Provider
- [x] 实现 OpenAI Chat Completion API 调用 (`pkg/llm/openai/`)
- [x] 支持流式响应 (Stream)
- [x] 支持自定义 Base URL（兼容其他 API）
- [x] 支持超时配置

### 4.2 LLM 配置管理
- [x] 实现 YAML 配置文件加载 (`pkg/llm/config.go`)
- [x] 支持环境变量覆盖
- [x] API Key 脱敏处理

---

## 阶段五：Agent 核心 (Core) ✅

### 5.1 执行循环
- [x] 实现 Agent 执行主循环 (`pkg/chassis/agent.go`)
- [x] 对话上下文管理
- [x] Function 调用解析和执行
- [x] 结果反馈给 LLM

### 5.2 上下文管理
- [x] 实现对话历史管理 (`pkg/chassis/context.go`)
- [x] 实现会话存储（内存）
- [x] Token 截断策略
- [x] 支持多轮对话

### 5.3 应用入口
- [x] 实现 `chassis.New()` 工厂方法
- [x] 实现 `app.Register()` 函数注册
- [x] 实现 `app.Initialize()` 初始化方法

---

## 阶段六：HTTP Server (API) ✅

### 6.1 Gin 框架搭建
- [x] 初始化 Gin 路由 (`pkg/server/router.go`)
- [x] 配置中间件（日志、Recovery、CORS）
- [x] 健康检查接口 `GET /health`

### 6.2 对话 API
- [x] 实现 `POST /api/v1/chat` 对话接口
- [x] 支持 session_id 多轮对话
- [x] 返回 Function 调用结果

### 6.3 Function 管理 API
- [x] 实现 `GET /api/v1/functions` 列出所有 Function
- [x] 实现 `GET /api/v1/functions/:name` 获取 Function 详情

### 6.4 延时任务管理 API ✅
- [x] 实现 `GET /api/v1/delay-tasks` 列出延时任务
- [x] 实现 `POST /api/v1/delay-tasks` 创建延时任务
- [x] 实现 `GET /api/v1/delay-tasks/:name` 获取任务详情
- [x] 实现 `DELETE /api/v1/delay-tasks/:name` 取消任务

### 6.5 Cron 管理 API（待开发）
- [ ] 实现 `GET /api/v1/crons` 列出所有定时任务
- [ ] 实现 `POST /api/v1/crons` 创建定时任务
- [ ] 实现 `DELETE /api/v1/crons/:name` 删除定时任务

---

## 阶段七：定时任务 (Scheduler) 🔄

### 7.1 延时任务 (DelayTask) ✅
- [x] 定义 DelayTask GORM Model (`pkg/scheduler/model.go`)
- [x] 实现 CRUD Repository (`pkg/scheduler/repository.go`)
- [x] 实现 DelayScheduler (`pkg/scheduler/delay_scheduler.go`)
  - [x] 使用 time.AfterFunc 实现调度
  - [x] 重启恢复：已过期任务标记为 missed
  - [x] 保留已完成任务历史记录
- [x] 内置 Function
  - [x] `send_message` - 发送消息通知（支持多渠道：console/email/sms/wechat）(`pkg/function/builtin/reminder.go`)
  - [x] `delay_create` - 创建延时任务 (`pkg/function/builtin/delay.go`)
  - [x] `delay_list` - 列出延时任务
  - [x] `delay_cancel` - 取消延时任务
  - [x] `delay_get` - 获取任务详情
- [x] REST API
  - [x] `GET /api/v1/delay-tasks` - 列出任务
  - [x] `POST /api/v1/delay-tasks` - 创建任务
  - [x] `GET /api/v1/delay-tasks/:name` - 获取任务详情
  - [x] `DELETE /api/v1/delay-tasks/:name` - 取消任务
- [x] 系统提示词增强
  - [x] 添加当前时间信息，支持 AI 计算未来时间
  - [x] 强调使用用户语言回复
- [x] 单元测试

### 7.2 Cron 定时任务 (CronTask) - 待开发
- [ ] 定义 CronTask 调度器 (`pkg/scheduler/cron_scheduler.go`)
- [ ] 集成 robfig/cron
- [ ] 支持动态添加/删除任务
- [ ] 任务执行回调，更新状态
- [ ] 启动时从数据库加载任务
- [ ] 任务执行后更新 LastRunAt、LastStatus
- [ ] 内置 Function (`pkg/function/builtin/cron.go`)
  - [ ] `cron_create` 函数
  - [ ] `cron_list` 函数
  - [ ] `cron_delete` 函数
- [ ] 单元测试

---

## 阶段八：CLI 入口 (CLI) ✅

### 8.1 命令行框架
- [x] 使用 Cobra 搭建 CLI (`cmd/agent/main.go`)
- [x] 实现 `agent serve` 命令（启动 HTTP Server）
- [x] 支持 `--config` 指定配置文件
- [x] 支持 `--port` 指定端口

---

## 阶段九：可观测性增强 (Observability)

### 9.1 Prometheus 指标
- [ ] 实现指标收集器 (`pkg/observability/metrics.go`)
- [ ] LLM 调用指标
- [ ] Function 执行指标
- [ ] 暴露 `GET /metrics` 端点

### 9.2 OpenTelemetry 链路追踪
- [ ] 集成 OTel SDK (`pkg/observability/tracing.go`)
- [ ] 为 LLM 调用添加 Span
- [ ] 为 Function 执行添加 Span
- [ ] 支持导出到 Jaeger/OTLP

---

## 阶段十：示例和文档 (Examples) 🔄

### 10.1 示例项目
- [x] 创建 `hello` 示例（greet, get_time, calculate）
- [ ] 创建 `file_cleaner` 示例 Function
- [ ] 创建 `http_request` 示例 Function
- [ ] 创建完整的 Cron 使用示例

### 10.2 文档完善
- [ ] 完善 README.md 快速开始
- [ ] 添加 API 文档（Swagger 或手写）
- [ ] 添加最佳实践指南

---

## 开发优先级与排期

| 优先级 | 阶段 | 内容 | 状态 |
|--------|------|------|------|
| **P0** | 阶段一 | 基础架构 | ✅ 完成 |
| **P0** | 阶段二 | 协议层 | ✅ 完成 |
| **P0** | 阶段三 | Function 系统 | ✅ 完成 |
| **P0** | 阶段四 | LLM 集成 | ✅ 完成 |
| **P0** | 阶段五 | Agent 核心 | ✅ 完成 |
| **P0** | 阶段六 | HTTP Server | ✅ 完成 |
| **P1** | 阶段七 | 定时任务（DelayTask ✅ / CronTask 待开发） | 🔄 进行中 |
| **P1** | 阶段八 | CLI 入口 | ✅ 完成 |
| **P2** | 阶段九 | 可观测性增强 | 待开发 |
| **P2** | 阶段十 | 示例和文档 | 🔄 进行中 |

---

## ✅ MVP 里程碑（最小可用版本）已达成！

已完成的功能：
- ✅ 用户可通过 REST API 与 Agent 对话
- ✅ Agent 能够调用已注册的 Function
- ✅ 支持多轮对话
- ✅ 基础日志记录
- ✅ CLI 工具（agent serve）
- ✅ 示例项目

---

## 状态说明

- `[ ]` 待完成
- `[x]` 已完成并测试通过
- 🔄 开发中
- ⏳ 等待用户测试

---

## 更新记录

| 日期 | 说明 |
|------|------|
| 2024-XX-XX | 创建初始开发计划 |
| 2024-XX-XX | 更新：使用 GORM，初期专注 HTTP Server 模式 |
| 2024-XX-XX | **MVP 完成**：阶段一到六、阶段八已完成 |
