// Package scheduler 提供定时任务调度功能
package scheduler

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/KodaTao/AgentChassis/pkg/function"
	"gorm.io/gorm"
)

// DelayScheduler 延时任务调度器
type DelayScheduler struct {
	db       *gorm.DB
	repo     *DelayTaskRepository
	registry *function.Registry
	logger   *slog.Logger

	mu     sync.RWMutex
	timers map[string]*time.Timer // 任务名 -> 定时器

	ctx    context.Context
	cancel context.CancelFunc
}

// NewDelayScheduler 创建延时任务调度器
func NewDelayScheduler(db *gorm.DB, registry *function.Registry, logger *slog.Logger) *DelayScheduler {
	ctx, cancel := context.WithCancel(context.Background())

	return &DelayScheduler{
		db:       db,
		repo:     NewDelayTaskRepository(db),
		registry: registry,
		logger:   logger,
		timers:   make(map[string]*time.Timer),
		ctx:      ctx,
		cancel:   cancel,
	}
}

// Start 启动调度器，恢复待执行的任务
func (s *DelayScheduler) Start() error {
	s.logger.Info("starting delay scheduler")

	// 自动迁移表
	if err := s.db.AutoMigrate(&DelayTask{}); err != nil {
		return fmt.Errorf("failed to migrate delay_tasks table: %w", err)
	}

	// 恢复待执行的任务
	if err := s.recoverTasks(); err != nil {
		return fmt.Errorf("failed to recover tasks: %w", err)
	}

	s.logger.Info("delay scheduler started")
	return nil
}

// Stop 停止调度器
func (s *DelayScheduler) Stop() {
	s.logger.Info("stopping delay scheduler")
	s.cancel()

	s.mu.Lock()
	defer s.mu.Unlock()

	// 停止所有定时器
	for name, timer := range s.timers {
		timer.Stop()
		s.logger.Debug("stopped timer", "task", name)
	}
	s.timers = make(map[string]*time.Timer)

	s.logger.Info("delay scheduler stopped")
}

// recoverTasks 恢复待执行的任务
func (s *DelayScheduler) recoverTasks() error {
	tasks, err := s.repo.ListPending()
	if err != nil {
		return err
	}

	s.logger.Info("recovering pending tasks", "count", len(tasks))

	for _, task := range tasks {
		if task.IsExpired() {
			// 已过期的任务标记为 missed
			if err := s.repo.MarkAsMissed(task.Name); err != nil {
				s.logger.Error("failed to mark task as missed", "task", task.Name, "error", err)
			} else {
				s.logger.Warn("task missed due to server restart", "task", task.Name, "run_at", task.RunAt)
			}
		} else {
			// 未过期的任务重新调度
			if err := s.scheduleTask(&task); err != nil {
				s.logger.Error("failed to reschedule task", "task", task.Name, "error", err)
			} else {
				s.logger.Info("task rescheduled", "task", task.Name, "run_at", task.RunAt)
			}
		}
	}

	return nil
}

// CreateTask 创建并调度延时任务
func (s *DelayScheduler) CreateTask(name, functionName string, runAt time.Time, params map[string]any) (*DelayTask, error) {
	// 验证函数是否存在
	if _, ok := s.registry.Get(functionName); !ok {
		return nil, fmt.Errorf("function not found: %s", functionName)
	}

	// 检查执行时间是否在未来
	if runAt.Before(time.Now()) {
		return nil, fmt.Errorf("run_at must be in the future")
	}

	// 检查任务名是否已存在
	exists, err := s.repo.Exists(name)
	if err != nil {
		return nil, fmt.Errorf("failed to check task existence: %w", err)
	}
	if exists {
		return nil, ErrTaskExists
	}

	// 序列化参数
	paramsJSON := ""
	if params != nil {
		data, err := json.Marshal(params)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal params: %w", err)
		}
		paramsJSON = string(data)
	}

	// 创建任务
	task := &DelayTask{
		Name:         name,
		RunAt:        runAt,
		FunctionName: functionName,
		Params:       paramsJSON,
		Status:       StatusPending,
	}

	if err := s.repo.Create(task); err != nil {
		return nil, fmt.Errorf("failed to create task: %w", err)
	}

	// 调度任务
	if err := s.scheduleTask(task); err != nil {
		// 如果调度失败，删除任务
		_ = s.repo.Delete(name)
		return nil, fmt.Errorf("failed to schedule task: %w", err)
	}

	s.logger.Info("task created and scheduled",
		"task", name,
		"function", functionName,
		"run_at", runAt,
	)

	return task, nil
}

// scheduleTask 调度单个任务
func (s *DelayScheduler) scheduleTask(task *DelayTask) error {
	delay := time.Until(task.RunAt)
	if delay < 0 {
		delay = 0
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// 如果已有同名定时器，先停止
	if existingTimer, ok := s.timers[task.Name]; ok {
		existingTimer.Stop()
	}

	// 创建新定时器
	timer := time.AfterFunc(delay, func() {
		s.executeTask(task.Name)
	})

	s.timers[task.Name] = timer

	s.logger.Debug("task scheduled",
		"task", task.Name,
		"delay", delay,
		"run_at", task.RunAt,
	)

	return nil
}

// executeTask 执行任务
func (s *DelayScheduler) executeTask(taskName string) {
	// 检查调度器是否已停止
	select {
	case <-s.ctx.Done():
		return
	default:
	}

	s.logger.Info("executing task", "task", taskName)

	// 获取任务信息
	task, err := s.repo.GetByName(taskName)
	if err != nil {
		s.logger.Error("failed to get task", "task", taskName, "error", err)
		return
	}

	// 检查任务状态
	if task.Status != StatusPending {
		s.logger.Warn("task is not pending, skipping", "task", taskName, "status", task.Status)
		return
	}

	// 更新状态为 running
	if err := s.repo.UpdateStatus(taskName, StatusRunning, "", ""); err != nil {
		s.logger.Error("failed to update task status to running", "task", taskName, "error", err)
		return
	}

	// 获取函数
	_, ok := s.registry.Get(task.FunctionName)
	if !ok {
		errMsg := fmt.Sprintf("function not found: %s", task.FunctionName)
		s.logger.Error(errMsg, "task", taskName)
		_ = s.repo.UpdateStatus(taskName, StatusFailed, "", errMsg)
		return
	}

	// 解析参数
	var params map[string]any
	if task.Params != "" {
		if err := json.Unmarshal([]byte(task.Params), &params); err != nil {
			errMsg := fmt.Sprintf("failed to unmarshal params: %v", err)
			s.logger.Error(errMsg, "task", taskName)
			_ = s.repo.UpdateStatus(taskName, StatusFailed, "", errMsg)
			return
		}
	}

	// 执行函数
	ctx, cancel := context.WithTimeout(s.ctx, 5*time.Minute)
	defer cancel()

	executor := function.NewExecutor(s.registry, 5*time.Minute)
	execResp := executor.Execute(ctx, function.ExecuteRequest{
		FunctionName: task.FunctionName,
		Params:       convertToStringParams(params),
	})
	result := execResp.Result
	err = execResp.Error

	// 更新任务状态
	if err != nil {
		errMsg := err.Error()
		s.logger.Error("task execution failed", "task", taskName, "error", errMsg)
		_ = s.repo.UpdateStatus(taskName, StatusFailed, "", errMsg)
	} else {
		resultJSON, _ := json.Marshal(result)
		s.logger.Info("task execution completed", "task", taskName, "result", string(resultJSON))
		_ = s.repo.UpdateStatus(taskName, StatusCompleted, string(resultJSON), "")
	}

	// 从定时器映射中移除
	s.mu.Lock()
	delete(s.timers, taskName)
	s.mu.Unlock()
}

// CancelTask 取消任务
func (s *DelayScheduler) CancelTask(name string) error {
	// 先取消定时器
	s.mu.Lock()
	if timer, ok := s.timers[name]; ok {
		timer.Stop()
		delete(s.timers, name)
	}
	s.mu.Unlock()

	// 更新数据库状态
	if err := s.repo.Cancel(name); err != nil {
		return err
	}

	s.logger.Info("task cancelled", "task", name)
	return nil
}

// GetTask 获取任务信息
func (s *DelayScheduler) GetTask(name string) (*DelayTask, error) {
	return s.repo.GetByName(name)
}

// ListTasks 列出任务
func (s *DelayScheduler) ListTasks(status *TaskStatus, limit int) ([]DelayTask, error) {
	return s.repo.List(status, limit)
}

// ListPendingTasks 列出待执行的任务
func (s *DelayScheduler) ListPendingTasks() ([]DelayTask, error) {
	return s.repo.ListPending()
}

// GetRepository 获取 Repository（供外部使用）
func (s *DelayScheduler) GetRepository() *DelayTaskRepository {
	return s.repo
}

// convertToStringParams 将 map[string]any 转换为 map[string]string
func convertToStringParams(params map[string]any) map[string]string {
	if params == nil {
		return nil
	}
	result := make(map[string]string, len(params))
	for k, v := range params {
		result[k] = fmt.Sprintf("%v", v)
	}
	return result
}
