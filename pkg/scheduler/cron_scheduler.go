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
	"github.com/robfig/cron/v3"
	"gorm.io/gorm"
)

// CronScheduler Cron 定时任务调度器
type CronScheduler struct {
	db       *gorm.DB
	taskRepo *CronTaskRepository
	execRepo *CronExecutionRepository
	registry *function.Registry
	logger   *slog.Logger

	cron     *cron.Cron
	mu       sync.RWMutex
	entryMap map[uint]cron.EntryID // 任务ID -> cron EntryID

	ctx    context.Context
	cancel context.CancelFunc
}

// NewCronScheduler 创建 Cron 调度器
func NewCronScheduler(db *gorm.DB, registry *function.Registry, logger *slog.Logger) *CronScheduler {
	ctx, cancel := context.WithCancel(context.Background())

	// 创建支持秒级的 cron 调度器（6 字段格式）
	c := cron.New(cron.WithSeconds())

	return &CronScheduler{
		db:       db,
		taskRepo: NewCronTaskRepository(db),
		execRepo: NewCronExecutionRepository(db),
		registry: registry,
		logger:   logger,
		cron:     c,
		entryMap: make(map[uint]cron.EntryID),
		ctx:      ctx,
		cancel:   cancel,
	}
}

// Start 启动调度器
func (s *CronScheduler) Start() error {
	s.logger.Info("starting cron scheduler")

	// 自动迁移表
	if err := s.db.AutoMigrate(&CronTask{}, &CronExecution{}); err != nil {
		return fmt.Errorf("failed to migrate cron tables: %w", err)
	}

	// 恢复所有任务
	if err := s.recoverTasks(); err != nil {
		return fmt.Errorf("failed to recover cron tasks: %w", err)
	}

	// 启动 cron 调度器
	s.cron.Start()

	s.logger.Info("cron scheduler started")
	return nil
}

// Stop 停止调度器
func (s *CronScheduler) Stop() {
	s.logger.Info("stopping cron scheduler")
	s.cancel()

	// 停止 cron 调度器
	ctx := s.cron.Stop()
	<-ctx.Done()

	s.mu.Lock()
	s.entryMap = make(map[uint]cron.EntryID)
	s.mu.Unlock()

	s.logger.Info("cron scheduler stopped")
}

// recoverTasks 恢复所有任务
func (s *CronScheduler) recoverTasks() error {
	tasks, err := s.taskRepo.ListAll()
	if err != nil {
		return err
	}

	s.logger.Info("recovering cron tasks", "count", len(tasks))

	for _, task := range tasks {
		if err := s.scheduleTask(&task); err != nil {
			s.logger.Error("failed to recover cron task", "task_id", task.ID, "name", task.Name, "error", err)
		} else {
			s.logger.Info("cron task recovered", "task_id", task.ID, "name", task.Name, "cron_expr", task.CronExpr)
		}
	}

	return nil
}

// CreateTask 创建定时任务
func (s *CronScheduler) CreateTask(name, cronExpr, functionName string, params map[string]any, description string) (*CronTask, error) {
	// 验证 cron 表达式
	parser := cron.NewParser(cron.Second | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	schedule, err := parser.Parse(cronExpr)
	if err != nil {
		return nil, fmt.Errorf("invalid cron expression: %w", err)
	}

	// 验证函数是否存在
	if _, ok := s.registry.Get(functionName); !ok {
		return nil, fmt.Errorf("function not found: %s", functionName)
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

	// 计算下次执行时间
	nextRun := schedule.Next(time.Now())

	// 创建任务
	task := &CronTask{
		Name:         name,
		CronExpr:     cronExpr,
		FunctionName: functionName,
		Params:       paramsJSON,
		Description:  description,
		NextRunAt:    &nextRun,
	}

	if err := s.taskRepo.Create(task); err != nil {
		return nil, fmt.Errorf("failed to create cron task: %w", err)
	}

	// 调度任务
	if err := s.scheduleTask(task); err != nil {
		// 如果调度失败，删除任务
		_ = s.taskRepo.DeleteByID(task.ID)
		return nil, fmt.Errorf("failed to schedule cron task: %w", err)
	}

	s.logger.Info("cron task created",
		"task_id", task.ID,
		"name", name,
		"cron_expr", cronExpr,
		"function", functionName,
		"next_run", nextRun,
	)

	return task, nil
}

// scheduleTask 调度单个任务
func (s *CronScheduler) scheduleTask(task *CronTask) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 如果已有同 ID 的调度，先移除
	if entryID, ok := s.entryMap[task.ID]; ok {
		s.cron.Remove(entryID)
		delete(s.entryMap, task.ID)
	}

	// 创建任务执行闭包
	taskID := task.ID
	entryID, err := s.cron.AddFunc(task.CronExpr, func() {
		s.executeTask(taskID)
	})
	if err != nil {
		return fmt.Errorf("failed to add cron entry: %w", err)
	}

	s.entryMap[task.ID] = entryID

	// 更新下次执行时间
	entry := s.cron.Entry(entryID)
	if !entry.Next.IsZero() {
		_ = s.taskRepo.UpdateNextRunAt(task.ID, entry.Next)
	}

	s.logger.Debug("cron task scheduled",
		"task_id", task.ID,
		"name", task.Name,
		"cron_expr", task.CronExpr,
		"entry_id", entryID,
	)

	return nil
}

// executeTask 执行任务
func (s *CronScheduler) executeTask(taskID uint) {
	// 检查调度器是否已停止
	select {
	case <-s.ctx.Done():
		return
	default:
	}

	scheduledAt := time.Now()
	s.logger.Info("executing cron task", "task_id", taskID, "scheduled_at", scheduledAt)

	// 获取任务信息
	task, err := s.taskRepo.GetByID(taskID)
	if err != nil {
		s.logger.Error("failed to get cron task", "task_id", taskID, "error", err)
		return
	}

	// 创建执行记录
	startedAt := time.Now()
	exec := &CronExecution{
		CronTaskID:  taskID,
		ScheduledAt: scheduledAt,
		StartedAt:   startedAt,
		Status:      CronStatusRunning,
	}
	if err := s.execRepo.Create(exec); err != nil {
		s.logger.Error("failed to create execution record", "task_id", taskID, "error", err)
		// 继续执行，只是没有记录
	}

	// 获取函数
	_, ok := s.registry.Get(task.FunctionName)
	if !ok {
		errMsg := fmt.Sprintf("function not found: %s", task.FunctionName)
		s.logger.Error(errMsg, "task_id", taskID)
		s.finishExecution(exec, CronStatusFailed, "", errMsg)
		return
	}

	// 解析参数
	var params map[string]any
	if task.Params != "" {
		if err := json.Unmarshal([]byte(task.Params), &params); err != nil {
			errMsg := fmt.Sprintf("failed to unmarshal params: %v", err)
			s.logger.Error(errMsg, "task_id", taskID)
			s.finishExecution(exec, CronStatusFailed, "", errMsg)
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
	execErr := execResp.Error

	// 更新执行记录
	if execErr != nil {
		errMsg := execErr.Error()
		s.logger.Error("cron task execution failed", "task_id", taskID, "error", errMsg)
		s.finishExecution(exec, CronStatusFailed, "", errMsg)
	} else {
		resultJSON, _ := json.Marshal(result)
		s.logger.Info("cron task execution completed", "task_id", taskID, "result", string(resultJSON))
		s.finishExecution(exec, CronStatusCompleted, string(resultJSON), "")
	}

	// 更新下次执行时间
	s.mu.RLock()
	entryID, ok := s.entryMap[taskID]
	s.mu.RUnlock()

	if ok {
		entry := s.cron.Entry(entryID)
		if !entry.Next.IsZero() {
			_ = s.taskRepo.UpdateNextRunAt(taskID, entry.Next)
		}
	}
}

// finishExecution 完成执行记录
func (s *CronScheduler) finishExecution(exec *CronExecution, status CronExecutionStatus, result, errMsg string) {
	if exec == nil || exec.ID == 0 {
		return
	}

	finishedAt := time.Now()
	exec.FinishedAt = &finishedAt
	exec.Status = status
	exec.Result = result
	exec.Error = errMsg
	exec.Duration = finishedAt.Sub(exec.StartedAt).Milliseconds()

	if err := s.execRepo.Update(exec); err != nil {
		s.logger.Error("failed to update execution record", "exec_id", exec.ID, "error", err)
	}
}

// DeleteTaskByID 根据 ID 删除任务
func (s *CronScheduler) DeleteTaskByID(id uint) error {
	s.mu.Lock()
	// 从 cron 中移除
	if entryID, ok := s.entryMap[id]; ok {
		s.cron.Remove(entryID)
		delete(s.entryMap, id)
	}
	s.mu.Unlock()

	// 删除执行历史
	if err := s.execRepo.DeleteByTaskID(id); err != nil {
		s.logger.Warn("failed to delete execution history", "task_id", id, "error", err)
	}

	// 删除任务
	if err := s.taskRepo.DeleteByID(id); err != nil {
		return err
	}

	s.logger.Info("cron task deleted", "task_id", id)
	return nil
}

// GetTaskByID 根据 ID 获取任务
func (s *CronScheduler) GetTaskByID(id uint) (*CronTask, error) {
	return s.taskRepo.GetByID(id)
}

// ListTasks 列出任务
func (s *CronScheduler) ListTasks(limit, offset int) ([]CronTask, error) {
	return s.taskRepo.List(limit, offset)
}

// CountTasks 统计任务数量
func (s *CronScheduler) CountTasks() (int64, error) {
	return s.taskRepo.Count()
}

// GetExecutionHistory 获取任务的执行历史
func (s *CronScheduler) GetExecutionHistory(taskID uint, limit, offset int) ([]CronExecution, error) {
	return s.execRepo.ListByTaskID(taskID, limit, offset)
}

// CountExecutionHistory 统计任务的执行历史数量
func (s *CronScheduler) CountExecutionHistory(taskID uint) (int64, error) {
	return s.execRepo.CountByTaskID(taskID)
}

// GetTaskRepository 获取 Task Repository
func (s *CronScheduler) GetTaskRepository() *CronTaskRepository {
	return s.taskRepo
}

// GetExecutionRepository 获取 Execution Repository
func (s *CronScheduler) GetExecutionRepository() *CronExecutionRepository {
	return s.execRepo
}
