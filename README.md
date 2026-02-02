# 🚀 AgentChassis (AC)

**The Lightweight, Pluggable Agent Framework for Go.**

AgentChassis 是一个专为 Golang 开发者设计的智能体基座。它不像现有的 Agent 框架那样笨重，它专注于解决一件事：**如何以最省 Token 的方式，让 AI 稳定地调用你的本地函数。**

---

## ✨ 为什么选择 AgentChassis?

* **极简扩展**：支持“热插拔” Function。新增一个功能只需实现一个 Interface。
* **XML + TOON 协议**：首个原生支持 `XML 嵌套 TOON` 的框架。比 JSON 更省 Token，比纯文本更易被 AI 解析。
* **跨平台分发**：利用 Go 的优势，编译后是一个不到 20MB 的二进制文件，可在 Linux, macOS, Windows 任意部署。
* **任务编排**：内置 Cron 定时任务调度，AI 不仅能即时响应，还能帮你打理未来。

## 🛠️ 开发者指南：快速新增功能

```go
// 1. 实现一个简单的功能
type FileCleaner struct{}

func (f FileCleaner) Name() string { return "clean_logs" }
func (f FileCleaner) Description() string { return "清理指定目录的日志文件" }

func (f FileCleaner) Execute(params map[string]string, content string) (string, error) {
    // 你的业务逻辑：比如删除文件
    return "清理成功", nil
}

// 2. 注册进框架
func main() {
    app := chassis.New()
    app.Register(FileCleaner{})
    app.Run()
}

```

## 📜 协议规范

AgentChassis 强制引导 AI 使用以下高效格式：

```xml
<call name="clean_logs">
<params>path: "/var/log"</params>
</call>

```

## ⚖️ 开源协议

本项目采用 **Apache 2.0** 协议，鼓励企业级定制。

---