// Package server 提供 HTTP Server 功能
package server

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/KodaTao/AgentChassis/pkg/chassis"
	"github.com/KodaTao/AgentChassis/pkg/observability"
	scheduler_pkg "github.com/KodaTao/AgentChassis/pkg/scheduler"
)

// Server HTTP 服务器
type Server struct {
	app    *chassis.App
	engine *gin.Engine
	config *ServerConfig
}

// ServerConfig 服务器配置
type ServerConfig struct {
	Host string
	Port int
	Mode string // debug, release, test
}

// NewServer 创建 HTTP 服务器
func NewServer(app *chassis.App, config *ServerConfig) *Server {
	// 设置 Gin 模式
	switch config.Mode {
	case "release":
		gin.SetMode(gin.ReleaseMode)
	case "test":
		gin.SetMode(gin.TestMode)
	default:
		gin.SetMode(gin.DebugMode)
	}

	engine := gin.New()

	// 添加中间件
	engine.Use(gin.Recovery())
	engine.Use(LoggerMiddleware())
	engine.Use(CORSMiddleware())

	server := &Server{
		app:    app,
		engine: engine,
		config: config,
	}

	// 注册路由
	server.setupRoutes()

	return server
}

// setupRoutes 设置路由
func (s *Server) setupRoutes() {
	// 健康检查
	s.engine.GET("/health", s.healthCheck)

	// API v1
	v1 := s.engine.Group("/api/v1")
	{
		// 对话接口
		v1.POST("/chat", s.chat)

		// Function 管理
		v1.GET("/functions", s.listFunctions)
		v1.GET("/functions/:name", s.getFunction)

		// Session 管理
		v1.GET("/sessions", s.listSessions)
		v1.DELETE("/sessions/:id", s.deleteSession)

		// 延时任务管理
		v1.GET("/delay-tasks", s.listDelayTasks)
		v1.POST("/delay-tasks", s.createDelayTask)
		v1.GET("/delay-tasks/:id", s.getDelayTask)
		v1.DELETE("/delay-tasks/:id", s.cancelDelayTask)

		// 定时任务管理
		v1.GET("/crons", s.listCronTasks)
		v1.POST("/crons", s.createCronTask)
		v1.GET("/crons/:id", s.getCronTask)
		v1.DELETE("/crons/:id", s.deleteCronTask)
		v1.GET("/crons/:id/history", s.getCronTaskHistory)
	}
}

// Run 启动服务器
func (s *Server) Run() error {
	addr := s.config.Host + ":" + itoa(s.config.Port)
	observability.Info("Starting HTTP server", "address", addr)
	return s.engine.Run(addr)
}

// GetEngine 获取 Gin 引擎（用于测试）
func (s *Server) GetEngine() *gin.Engine {
	return s.engine
}

// 健康检查
func (s *Server) healthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":    "healthy",
		"timestamp": time.Now().Unix(),
	})
}

// 对话接口
func (s *Server) chat(c *gin.Context) {
	var req chassis.ChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request: " + err.Error(),
		})
		return
	}

	if req.Message == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "message is required",
		})
		return
	}

	// 执行对话
	resp, err := s.app.GetAgent().Chat(c.Request.Context(), req)
	if err != nil {
		observability.Error("Chat failed", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Chat failed: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, resp)
}

// 列出所有 Function
func (s *Server) listFunctions(c *gin.Context) {
	functions := s.app.GetRegistry().ListInfo()
	c.JSON(http.StatusOK, gin.H{
		"functions": functions,
		"count":     len(functions),
	})
}

// 获取单个 Function
func (s *Server) getFunction(c *gin.Context) {
	name := c.Param("name")

	fn, ok := s.app.GetRegistry().Get(name)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Function not found: " + name,
		})
		return
	}

	info := struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}{
		Name:        fn.Name(),
		Description: fn.Description(),
	}

	c.JSON(http.StatusOK, info)
}

// 列出所有 Session
func (s *Server) listSessions(c *gin.Context) {
	sessions := s.app.GetAgent().ListSessions()
	c.JSON(http.StatusOK, gin.H{
		"sessions": sessions,
		"count":    len(sessions),
	})
}

// 删除 Session
func (s *Server) deleteSession(c *gin.Context) {
	id := c.Param("id")

	if s.app.GetAgent().DeleteSession(id) {
		c.JSON(http.StatusOK, gin.H{
			"message": "Session deleted",
		})
	} else {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Session not found: " + id,
		})
	}
}

// CreateDelayTaskRequest 创建延时任务请求
type CreateDelayTaskRequest struct {
	Name   string `json:"name" binding:"required"`
	RunAt  string `json:"run_at" binding:"required"` // ISO8601 格式
	Prompt string `json:"prompt" binding:"required"` // 触发时发给AI的提示词
}

// 列出延时任务
func (s *Server) listDelayTasks(c *gin.Context) {
	scheduler := s.app.GetDelayScheduler()
	if scheduler == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "DelayScheduler not initialized",
		})
		return
	}

	// 获取查询参数
	statusStr := c.Query("status")
	var status *scheduler_pkg.TaskStatus
	if statusStr != "" {
		st := scheduler_pkg.TaskStatus(statusStr)
		status = &st
	}

	// 分页参数
	limit := 20
	offset := 0
	if l := c.Query("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	if o := c.Query("offset"); o != "" {
		if parsed, err := strconv.Atoi(o); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	tasks, err := scheduler.ListTasks(status, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to list tasks: " + err.Error(),
		})
		return
	}

	total, err := scheduler.CountTasks(status)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to count tasks: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"tasks":  tasks,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

// 创建延时任务
func (s *Server) createDelayTask(c *gin.Context) {
	scheduler := s.app.GetDelayScheduler()
	if scheduler == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "DelayScheduler not initialized",
		})
		return
	}

	var req CreateDelayTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request: " + err.Error(),
		})
		return
	}

	// 解析时间
	runAt, err := time.Parse(time.RFC3339, req.RunAt)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid run_at format, expected ISO8601/RFC3339: " + err.Error(),
		})
		return
	}

	// 创建任务
	task, err := scheduler.CreateTask(req.Name, runAt, req.Prompt)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, task)
}

// 获取延时任务详情
func (s *Server) getDelayTask(c *gin.Context) {
	scheduler := s.app.GetDelayScheduler()
	if scheduler == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "DelayScheduler not initialized",
		})
		return
	}

	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid task ID",
		})
		return
	}

	task, err := scheduler.GetTaskByID(uint(id))
	if err != nil {
		status := http.StatusInternalServerError
		if err == scheduler_pkg.ErrTaskNotFound {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, task)
}

// 取消延时任务
func (s *Server) cancelDelayTask(c *gin.Context) {
	scheduler := s.app.GetDelayScheduler()
	if scheduler == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "DelayScheduler not initialized",
		})
		return
	}

	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid task ID",
		})
		return
	}

	err = scheduler.CancelTaskByID(uint(id))
	if err != nil {
		status := http.StatusInternalServerError
		if err == scheduler_pkg.ErrTaskNotFound {
			status = http.StatusNotFound
		} else if err == scheduler_pkg.ErrTaskNotPending {
			status = http.StatusBadRequest
		}
		c.JSON(status, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Task cancelled successfully",
		"id":      id,
	})
}

// LoggerMiddleware 日志中间件
func LoggerMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path

		c.Next()

		latency := time.Since(start)
		status := c.Writer.Status()

		observability.Info("HTTP request",
			"method", c.Request.Method,
			"path", path,
			"status", status,
			"latency_ms", latency.Milliseconds(),
			"client_ip", c.ClientIP(),
		)
	}
}

// CORSMiddleware 跨域中间件
func CORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Authorization")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

// itoa 简单的整数转字符串
func itoa(n int) string {
	if n == 0 {
		return "0"
	}

	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}

// CreateCronTaskRequest 创建定时任务请求
type CreateCronTaskRequest struct {
	Name        string `json:"name" binding:"required"`
	CronExpr    string `json:"cron_expr" binding:"required"`
	Prompt      string `json:"prompt" binding:"required"` // 触发时发给AI的提示词
	Description string `json:"description"`
}

// 列出定时任务
func (s *Server) listCronTasks(c *gin.Context) {
	scheduler := s.app.GetCronScheduler()
	if scheduler == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "CronScheduler not initialized",
		})
		return
	}

	// 分页参数
	limit := 20
	offset := 0
	if l := c.Query("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	if o := c.Query("offset"); o != "" {
		if parsed, err := strconv.Atoi(o); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	tasks, err := scheduler.ListTasks(limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to list cron tasks: " + err.Error(),
		})
		return
	}

	total, err := scheduler.CountTasks()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to count cron tasks: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"tasks":  tasks,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

// 创建定时任务
func (s *Server) createCronTask(c *gin.Context) {
	scheduler := s.app.GetCronScheduler()
	if scheduler == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "CronScheduler not initialized",
		})
		return
	}

	var req CreateCronTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request: " + err.Error(),
		})
		return
	}

	// 创建任务
	task, err := scheduler.CreateTask(req.Name, req.CronExpr, req.Prompt, req.Description)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, task)
}

// 获取定时任务详情
func (s *Server) getCronTask(c *gin.Context) {
	scheduler := s.app.GetCronScheduler()
	if scheduler == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "CronScheduler not initialized",
		})
		return
	}

	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid task ID",
		})
		return
	}

	task, err := scheduler.GetTaskByID(uint(id))
	if err != nil {
		status := http.StatusInternalServerError
		if err == scheduler_pkg.ErrCronTaskNotFound {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, task)
}

// 删除定时任务
func (s *Server) deleteCronTask(c *gin.Context) {
	scheduler := s.app.GetCronScheduler()
	if scheduler == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "CronScheduler not initialized",
		})
		return
	}

	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid task ID",
		})
		return
	}

	err = scheduler.DeleteTaskByID(uint(id))
	if err != nil {
		status := http.StatusInternalServerError
		if err == scheduler_pkg.ErrCronTaskNotFound {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Cron task deleted successfully",
		"id":      id,
	})
}

// 获取定时任务执行历史
func (s *Server) getCronTaskHistory(c *gin.Context) {
	scheduler := s.app.GetCronScheduler()
	if scheduler == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "CronScheduler not initialized",
		})
		return
	}

	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid task ID",
		})
		return
	}

	// 先验证任务存在
	task, err := scheduler.GetTaskByID(uint(id))
	if err != nil {
		status := http.StatusInternalServerError
		if err == scheduler_pkg.ErrCronTaskNotFound {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{
			"error": err.Error(),
		})
		return
	}

	// 分页参数
	limit := 20
	offset := 0
	if l := c.Query("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	if o := c.Query("offset"); o != "" {
		if parsed, err := strconv.Atoi(o); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	executions, err := scheduler.GetExecutionHistory(uint(id), limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to get execution history: " + err.Error(),
		})
		return
	}

	total, err := scheduler.CountExecutionHistory(uint(id))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to count execution history: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"task_id":    id,
		"task_name":  task.Name,
		"executions": executions,
		"total":      total,
		"limit":      limit,
		"offset":     offset,
	})
}
