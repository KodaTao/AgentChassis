// Package scheduler 提供定时任务调度功能
package scheduler

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"gorm.io/gorm"
)

// DelayScheduler 延时任务调度器
type DelayScheduler struct {
	db            *gorm.DB
	repo          *DelayTaskRepository
	agentExecutor AgentExecutor
	logger        *slog.Logger

	mu     sync.RWMutex
	timers map[uint]*time.Timer // 任务ID -> 定时器

	ctx    context.Context
	cancel context.CancelFunc
}

// NewDelayScheduler 创建延时任务调度器
func NewDelayScheduler(db *gorm.DB, logger *slog.Logger) *DelayScheduler {
	ctx, cancel := context.WithCancel(context.Background())

	return &DelayScheduler{
		db:     db,
		repo:   NewDelayTaskRepository(db),
		logger: logger,
		timers: make(map[uint]*time.Timer),
		ctx:    ctx,
		cancel: cancel,
	}
}

// SetAgentExecutor 设置 Agent 执行器（用于依赖注入，避免循环依赖）
func (s *DelayScheduler) SetAgentExecutor(executor AgentExecutor) {
	s.agentExecutor = executor
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
	for id, timer := range s.timers {
		timer.Stop()
		s.logger.Debug("stopped timer", "task_id", id)
	}
	s.timers = make(map[uint]*time.Timer)

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
			if err := s.repo.MarkAsMissedByID(task.ID); err != nil {
				s.logger.Error("failed to mark task as missed", "task_id", task.ID, "name", task.Name, "error", err)
			} else {
				s.logger.Warn("task missed due to server restart", "task_id", task.ID, "name", task.Name, "run_at", task.RunAt)
			}
		} else {
			// 未过期的任务重新调度
			if err := s.scheduleTask(&task); err != nil {
				s.logger.Error("failed to reschedule task", "task_id", task.ID, "name", task.Name, "error", err)
			} else {
				s.logger.Info("task rescheduled", "task_id", task.ID, "name", task.Name, "run_at", task.RunAt)
			}
		}
	}

	return nil
}

// CreateTask 创建并调度延时任务
// channel 参数为可选的渠道上下文 JSON 字符串
func (s *DelayScheduler) CreateTask(name string, runAt time.Time, prompt string, channel ...string) (*DelayTask, error) {
	// 检查执行时间是否在未来
	if runAt.Before(time.Now()) {
		return nil, fmt.Errorf("run_at must be in the future")
	}

	// 检查 prompt 不能为空
	if prompt == "" {
		return nil, fmt.Errorf("prompt cannot be empty")
	}

	// 创建任务
	task := &DelayTask{
		Name:   name,
		RunAt:  runAt,
		Prompt: prompt,
		Status: StatusPending,
	}

	// 设置渠道信息（如果提供）
	if len(channel) > 0 && channel[0] != "" {
		task.Channel = channel[0]
	}

	if err := s.repo.Create(task); err != nil {
		return nil, fmt.Errorf("failed to create task: %w", err)
	}

	// 调度任务
	if err := s.scheduleTask(task); err != nil {
		// 如果调度失败，删除任务
		_ = s.repo.DeleteByID(task.ID)
		return nil, fmt.Errorf("failed to schedule task: %w", err)
	}

	s.logger.Info("task created and scheduled",
		"task_id", task.ID,
		"name", name,
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

	// 如果已有同 ID 定时器，先停止
	if existingTimer, ok := s.timers[task.ID]; ok {
		existingTimer.Stop()
	}

	// 创建新定时器
	taskID := task.ID
	timer := time.AfterFunc(delay, func() {
		s.executeTask(taskID)
	})

	s.timers[task.ID] = timer

	s.logger.Debug("task scheduled",
		"task_id", task.ID,
		"name", task.Name,
		"delay", delay,
		"run_at", task.RunAt,
	)

	return nil
}

// executeTask 执行任务
func (s *DelayScheduler) executeTask(taskID uint) {
	// 检查调度器是否已停止
	select {
	case <-s.ctx.Done():
		return
	default:
	}

	s.logger.Info("executing task", "task_id", taskID)

	// 获取任务信息
	task, err := s.repo.GetByID(taskID)
	if err != nil {
		s.logger.Error("failed to get task", "task_id", taskID, "error", err)
		return
	}

	// 检查任务状态
	if task.Status != StatusPending {
		s.logger.Warn("task is not pending, skipping", "task_id", taskID, "status", task.Status)
		return
	}

	// 更新状态为 running
	if err := s.repo.UpdateStatusByID(taskID, StatusRunning, "", ""); err != nil {
		s.logger.Error("failed to update task status to running", "task_id", taskID, "error", err)
		return
	}

	// 检查 AgentExecutor 是否已设置
	if s.agentExecutor == nil {
		errMsg := "agent executor not set"
		s.logger.Error(errMsg, "task_id", taskID)
		_ = s.repo.UpdateStatusByID(taskID, StatusFailed, "", errMsg)
		return
	}

	// 执行：调用 Agent
	ctx, cancel := context.WithTimeout(s.ctx, 5*time.Minute)
	defer cancel()

	result, err := s.agentExecutor.Execute(ctx, task.Prompt)

	// 更新任务状态
	if err != nil {
		errMsg := err.Error()
		s.logger.Error("task execution failed", "task_id", taskID, "error", errMsg)
		_ = s.repo.UpdateStatusByID(taskID, StatusFailed, "", errMsg)
	} else {
		s.logger.Info("task execution completed", "task_id", taskID, "result", result)
		_ = s.repo.UpdateStatusByID(taskID, StatusCompleted, result, "")
	}

	// 从定时器映射中移除
	s.mu.Lock()
	delete(s.timers, taskID)
	s.mu.Unlock()
}

// CancelTaskByID 根据 ID 取消任务
func (s *DelayScheduler) CancelTaskByID(id uint) error {
	// 先取消定时器
	s.mu.Lock()
	if timer, ok := s.timers[id]; ok {
		timer.Stop()
		delete(s.timers, id)
	}
	s.mu.Unlock()

	// 更新数据库状态
	if err := s.repo.CancelByID(id); err != nil {
		return err
	}

	s.logger.Info("task cancelled", "task_id", id)
	return nil
}

// GetTaskByID 根据 ID 获取任务信息
func (s *DelayScheduler) GetTaskByID(id uint) (*DelayTask, error) {
	return s.repo.GetByID(id)
}

// ListTasks 列出任务
func (s *DelayScheduler) ListTasks(status *TaskStatus, limit, offset int) ([]DelayTask, error) {
	return s.repo.List(status, limit, offset)
}

// CountTasks 统计任务数量
func (s *DelayScheduler) CountTasks(status *TaskStatus) (int64, error) {
	return s.repo.Count(status)
}

// ListPendingTasks 列出待执行的任务
func (s *DelayScheduler) ListPendingTasks() ([]DelayTask, error) {
	return s.repo.ListPending()
}

// GetRepository 获取 Repository（供外部使用）
func (s *DelayScheduler) GetRepository() *DelayTaskRepository {
	return s.repo
}
