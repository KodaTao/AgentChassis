// Package scheduler 提供定时任务调度功能
package scheduler

import (
	"context"
	"log/slog"
	"os"
	"sync"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// MockAgentExecutor 模拟 AgentExecutor
type MockAgentExecutor struct {
	mu         sync.Mutex
	executions []string
	result     string
	err        error
}

func (m *MockAgentExecutor) Execute(ctx context.Context, prompt string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.executions = append(m.executions, prompt)
	if m.err != nil {
		return "", m.err
	}
	if m.result != "" {
		return m.result, nil
	}
	return "执行完成: " + prompt, nil
}

func (m *MockAgentExecutor) ExecutionCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.executions)
}

func (m *MockAgentExecutor) LastPrompt() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.executions) == 0 {
		return ""
	}
	return m.executions[len(m.executions)-1]
}

// setupCronTestDB 创建 Cron 测试数据库
func setupCronTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	// 自动迁移
	if err := db.AutoMigrate(&CronTask{}, &CronExecution{}); err != nil {
		t.Fatalf("Failed to migrate: %v", err)
	}

	return db
}

// setupCronTestScheduler 创建测试调度器
func setupCronTestScheduler(t *testing.T) (*CronScheduler, *gorm.DB, *MockAgentExecutor) {
	db := setupCronTestDB(t)
	testLogger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	mockExecutor := &MockAgentExecutor{}

	scheduler := NewCronScheduler(db, testLogger)
	scheduler.SetAgentExecutor(mockExecutor)
	return scheduler, db, mockExecutor
}

func TestCronScheduler_CreateTask(t *testing.T) {
	scheduler, _, _ := setupCronTestScheduler(t)
	defer scheduler.Stop()

	// 启动调度器
	if err := scheduler.Start(); err != nil {
		t.Fatalf("Failed to start scheduler: %v", err)
	}

	// 创建任务（每分钟执行）
	task, err := scheduler.CreateTask("test_cron", "0 * * * * *", "请发送一条问候消息", "测试任务")
	if err != nil {
		t.Fatalf("Failed to create task: %v", err)
	}

	// 验证任务
	if task.Name != "test_cron" {
		t.Errorf("Expected name 'test_cron', got '%s'", task.Name)
	}
	if task.CronExpr != "0 * * * * *" {
		t.Errorf("Expected cron_expr '0 * * * * *', got '%s'", task.CronExpr)
	}
	if task.Prompt != "请发送一条问候消息" {
		t.Errorf("Expected prompt '请发送一条问候消息', got '%s'", task.Prompt)
	}
	if task.ID == 0 {
		t.Error("Expected task to have a valid ID")
	}
	if task.NextRunAt == nil {
		t.Error("Expected task to have next_run_at")
	}
}

func TestCronScheduler_CreateTask_InvalidCronExpr(t *testing.T) {
	scheduler, _, _ := setupCronTestScheduler(t)
	defer scheduler.Stop()

	if err := scheduler.Start(); err != nil {
		t.Fatalf("Failed to start scheduler: %v", err)
	}

	// 使用无效的 cron 表达式
	_, err := scheduler.CreateTask("test_cron", "invalid", "测试提示词", "")
	if err == nil {
		t.Error("Expected error for invalid cron expression")
	}
}

func TestCronScheduler_CreateTask_EmptyPrompt(t *testing.T) {
	scheduler, _, _ := setupCronTestScheduler(t)
	defer scheduler.Stop()

	if err := scheduler.Start(); err != nil {
		t.Fatalf("Failed to start scheduler: %v", err)
	}

	// 尝试创建任务，但 prompt 为空
	_, err := scheduler.CreateTask("test_cron", "0 * * * * *", "", "")
	if err == nil {
		t.Error("Expected error when prompt is empty")
	}
}

func TestCronScheduler_CreateTask_DuplicateName(t *testing.T) {
	scheduler, _, _ := setupCronTestScheduler(t)
	defer scheduler.Stop()

	if err := scheduler.Start(); err != nil {
		t.Fatalf("Failed to start scheduler: %v", err)
	}

	// 创建第一个任务
	task1, err := scheduler.CreateTask("test_cron", "0 * * * * *", "测试提示词1", "")
	if err != nil {
		t.Fatalf("Failed to create first task: %v", err)
	}

	// 创建同名任务（允许重复名称）
	task2, err := scheduler.CreateTask("test_cron", "30 * * * * *", "测试提示词2", "")
	if err != nil {
		t.Fatalf("Should allow duplicate name, but got error: %v", err)
	}

	// 验证两个任务有不同的 ID
	if task1.ID == task2.ID {
		t.Error("Expected different IDs for tasks with duplicate names")
	}
}

func TestCronScheduler_DeleteTask(t *testing.T) {
	scheduler, _, _ := setupCronTestScheduler(t)
	defer scheduler.Stop()

	if err := scheduler.Start(); err != nil {
		t.Fatalf("Failed to start scheduler: %v", err)
	}

	// 创建任务
	task, err := scheduler.CreateTask("test_cron", "0 * * * * *", "测试提示词", "")
	if err != nil {
		t.Fatalf("Failed to create task: %v", err)
	}

	// 删除任务
	if err := scheduler.DeleteTaskByID(task.ID); err != nil {
		t.Fatalf("Failed to delete task: %v", err)
	}

	// 验证任务已删除
	_, err = scheduler.GetTaskByID(task.ID)
	if err != ErrCronTaskNotFound {
		t.Errorf("Expected ErrCronTaskNotFound, got %v", err)
	}
}

func TestCronScheduler_ExecuteTask(t *testing.T) {
	scheduler, _, mockExecutor := setupCronTestScheduler(t)
	defer scheduler.Stop()

	if err := scheduler.Start(); err != nil {
		t.Fatalf("Failed to start scheduler: %v", err)
	}

	// 创建一个每秒执行的任务
	task, err := scheduler.CreateTask("test_cron", "* * * * * *", "请问候用户", "")
	if err != nil {
		t.Fatalf("Failed to create task: %v", err)
	}

	// 等待任务执行
	time.Sleep(1500 * time.Millisecond)

	// 验证 AgentExecutor 被调用
	if mockExecutor.ExecutionCount() == 0 {
		t.Error("Expected AgentExecutor to be called")
	}

	// 验证执行时传递的 prompt
	if mockExecutor.LastPrompt() != "请问候用户" {
		t.Errorf("Expected prompt '请问候用户', got '%s'", mockExecutor.LastPrompt())
	}

	// 验证执行历史被记录
	executions, err := scheduler.GetExecutionHistory(task.ID, 10, 0)
	if err != nil {
		t.Fatalf("Failed to get execution history: %v", err)
	}
	if len(executions) == 0 {
		t.Error("Expected at least one execution record")
	}
	if len(executions) > 0 && executions[0].Status != CronStatusCompleted {
		t.Errorf("Expected execution status 'completed', got '%s'", executions[0].Status)
	}
}

func TestCronScheduler_ListTasks(t *testing.T) {
	scheduler, _, _ := setupCronTestScheduler(t)
	defer scheduler.Stop()

	if err := scheduler.Start(); err != nil {
		t.Fatalf("Failed to start scheduler: %v", err)
	}

	// 创建多个任务
	for i := 0; i < 3; i++ {
		name := "task_" + string(rune('a'+i))
		_, err := scheduler.CreateTask(name, "0 * * * * *", "测试提示词", "")
		if err != nil {
			t.Fatalf("Failed to create task %s: %v", name, err)
		}
	}

	// 列出所有任务
	tasks, err := scheduler.ListTasks(20, 0)
	if err != nil {
		t.Fatalf("Failed to list tasks: %v", err)
	}
	if len(tasks) != 3 {
		t.Errorf("Expected 3 tasks, got %d", len(tasks))
	}
}

func TestCronScheduler_ListTasks_Pagination(t *testing.T) {
	scheduler, _, _ := setupCronTestScheduler(t)
	defer scheduler.Stop()

	if err := scheduler.Start(); err != nil {
		t.Fatalf("Failed to start scheduler: %v", err)
	}

	// 创建5个任务
	for i := 0; i < 5; i++ {
		name := "task_" + string(rune('a'+i))
		_, err := scheduler.CreateTask(name, "0 * * * * *", "测试提示词", "")
		if err != nil {
			t.Fatalf("Failed to create task %s: %v", name, err)
		}
	}

	// 测试分页：limit=2, offset=0
	tasks, err := scheduler.ListTasks(2, 0)
	if err != nil {
		t.Fatalf("Failed to list tasks: %v", err)
	}
	if len(tasks) != 2 {
		t.Errorf("Expected 2 tasks with limit=2, got %d", len(tasks))
	}

	// 测试总数
	count, err := scheduler.CountTasks()
	if err != nil {
		t.Fatalf("Failed to count tasks: %v", err)
	}
	if count != 5 {
		t.Errorf("Expected count 5, got %d", count)
	}
}

func TestCronScheduler_RecoverTasks(t *testing.T) {
	db := setupCronTestDB(t)
	testLogger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	mockExecutor := &MockAgentExecutor{}

	// 预先创建一个任务（直接写入数据库）
	task := &CronTask{
		Name:     "existing_task",
		CronExpr: "0 * * * * *",
		Prompt:   "恢复测试提示词",
	}
	if err := db.Create(task).Error; err != nil {
		t.Fatalf("Failed to create task: %v", err)
	}

	// 创建调度器并启动（会触发恢复）
	scheduler := NewCronScheduler(db, testLogger)
	scheduler.SetAgentExecutor(mockExecutor)
	if err := scheduler.Start(); err != nil {
		t.Fatalf("Failed to start scheduler: %v", err)
	}
	defer scheduler.Stop()

	// 验证任务被恢复
	tasks, err := scheduler.ListTasks(20, 0)
	if err != nil {
		t.Fatalf("Failed to list tasks: %v", err)
	}
	if len(tasks) != 1 {
		t.Errorf("Expected 1 recovered task, got %d", len(tasks))
	}
}

func TestCronTaskRepository_CRUD(t *testing.T) {
	db := setupCronTestDB(t)
	repo := NewCronTaskRepository(db)

	// Create
	task := &CronTask{
		Name:        "test_cron",
		CronExpr:    "0 * * * * *",
		Prompt:      "测试提示词",
		Description: "Test task",
	}
	if err := repo.Create(task); err != nil {
		t.Fatalf("Failed to create task: %v", err)
	}

	// Read by ID
	retrieved, err := repo.GetByID(task.ID)
	if err != nil {
		t.Fatalf("Failed to get task by ID: %v", err)
	}
	if retrieved.Name != "test_cron" {
		t.Errorf("Expected name 'test_cron', got '%s'", retrieved.Name)
	}

	// Update
	task.Description = "Updated description"
	if err := repo.Update(task); err != nil {
		t.Fatalf("Failed to update task: %v", err)
	}
	updated, _ := repo.GetByID(task.ID)
	if updated.Description != "Updated description" {
		t.Errorf("Expected description 'Updated description', got '%s'", updated.Description)
	}

	// DeleteByID
	if err := repo.DeleteByID(task.ID); err != nil {
		t.Fatalf("Failed to delete task: %v", err)
	}
	_, err = repo.GetByID(task.ID)
	if err != ErrCronTaskNotFound {
		t.Errorf("Expected ErrCronTaskNotFound after delete, got %v", err)
	}
}

func TestCronExecutionRepository_CRUD(t *testing.T) {
	db := setupCronTestDB(t)
	taskRepo := NewCronTaskRepository(db)
	execRepo := NewCronExecutionRepository(db)

	// 先创建一个任务
	task := &CronTask{
		Name:     "test_cron",
		CronExpr: "0 * * * * *",
		Prompt:   "测试提示词",
	}
	_ = taskRepo.Create(task)

	// Create execution
	now := time.Now()
	exec := &CronExecution{
		CronTaskID:  task.ID,
		ScheduledAt: now,
		StartedAt:   now,
		Status:      CronStatusRunning,
	}
	if err := execRepo.Create(exec); err != nil {
		t.Fatalf("Failed to create execution: %v", err)
	}

	// Read by ID
	retrieved, err := execRepo.GetByID(exec.ID)
	if err != nil {
		t.Fatalf("Failed to get execution by ID: %v", err)
	}
	if retrieved.CronTaskID != task.ID {
		t.Errorf("Expected cron_task_id %d, got %d", task.ID, retrieved.CronTaskID)
	}

	// Update
	finishedAt := time.Now()
	exec.FinishedAt = &finishedAt
	exec.Status = CronStatusCompleted
	exec.Result = "success"
	if err := execRepo.Update(exec); err != nil {
		t.Fatalf("Failed to update execution: %v", err)
	}
	updated, _ := execRepo.GetByID(exec.ID)
	if updated.Status != CronStatusCompleted {
		t.Errorf("Expected status 'completed', got '%s'", updated.Status)
	}

	// List by TaskID
	executions, err := execRepo.ListByTaskID(task.ID, 10, 0)
	if err != nil {
		t.Fatalf("Failed to list executions: %v", err)
	}
	if len(executions) != 1 {
		t.Errorf("Expected 1 execution, got %d", len(executions))
	}

	// Count by TaskID
	count, err := execRepo.CountByTaskID(task.ID)
	if err != nil {
		t.Fatalf("Failed to count executions: %v", err)
	}
	if count != 1 {
		t.Errorf("Expected count 1, got %d", count)
	}

	// Delete by TaskID
	if err := execRepo.DeleteByTaskID(task.ID); err != nil {
		t.Fatalf("Failed to delete executions by task ID: %v", err)
	}
	executions, _ = execRepo.ListByTaskID(task.ID, 10, 0)
	if len(executions) != 0 {
		t.Errorf("Expected 0 executions after delete, got %d", len(executions))
	}
}

func TestCronScheduler_SecondLevelPrecision(t *testing.T) {
	scheduler, _, _ := setupCronTestScheduler(t)
	defer scheduler.Stop()

	if err := scheduler.Start(); err != nil {
		t.Fatalf("Failed to start scheduler: %v", err)
	}

	// 创建一个每5秒执行的任务（使用秒级表达式）
	task, err := scheduler.CreateTask("test_cron", "*/5 * * * * *", "每5秒执行的提示词", "每5秒执行")
	if err != nil {
		t.Fatalf("Failed to create task: %v", err)
	}

	// 验证任务创建成功
	if task.CronExpr != "*/5 * * * * *" {
		t.Errorf("Expected cron_expr '*/5 * * * * *', got '%s'", task.CronExpr)
	}
}
