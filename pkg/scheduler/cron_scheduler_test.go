// Package scheduler 提供定时任务调度功能
package scheduler

import (
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/KodaTao/AgentChassis/pkg/function"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

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
func setupCronTestScheduler(t *testing.T) (*CronScheduler, *gorm.DB, *function.Registry) {
	db := setupCronTestDB(t)
	registry := function.NewRegistry()
	testLogger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))

	scheduler := NewCronScheduler(db, registry, testLogger)
	return scheduler, db, registry
}

func TestCronScheduler_CreateTask(t *testing.T) {
	scheduler, _, registry := setupCronTestScheduler(t)
	defer scheduler.Stop()

	// 注册测试函数
	mockFn := &MockFunction{name: "test_func"}
	_ = registry.Register(mockFn)

	// 启动调度器
	if err := scheduler.Start(); err != nil {
		t.Fatalf("Failed to start scheduler: %v", err)
	}

	// 创建任务（每分钟执行）
	task, err := scheduler.CreateTask("test_cron", "0 * * * * *", "test_func", map[string]any{"message": "hello"}, "测试任务")
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
	if task.FunctionName != "test_func" {
		t.Errorf("Expected function_name 'test_func', got '%s'", task.FunctionName)
	}
	if task.ID == 0 {
		t.Error("Expected task to have a valid ID")
	}
	if task.NextRunAt == nil {
		t.Error("Expected task to have next_run_at")
	}
}

func TestCronScheduler_CreateTask_InvalidCronExpr(t *testing.T) {
	scheduler, _, registry := setupCronTestScheduler(t)
	defer scheduler.Stop()

	// 注册测试函数
	mockFn := &MockFunction{name: "test_func"}
	_ = registry.Register(mockFn)

	if err := scheduler.Start(); err != nil {
		t.Fatalf("Failed to start scheduler: %v", err)
	}

	// 使用无效的 cron 表达式
	_, err := scheduler.CreateTask("test_cron", "invalid", "test_func", nil, "")
	if err == nil {
		t.Error("Expected error for invalid cron expression")
	}
}

func TestCronScheduler_CreateTask_FunctionNotFound(t *testing.T) {
	scheduler, _, _ := setupCronTestScheduler(t)
	defer scheduler.Stop()

	if err := scheduler.Start(); err != nil {
		t.Fatalf("Failed to start scheduler: %v", err)
	}

	// 尝试创建任务，但函数不存在
	_, err := scheduler.CreateTask("test_cron", "0 * * * * *", "nonexistent_func", nil, "")
	if err == nil {
		t.Error("Expected error when function not found")
	}
}

func TestCronScheduler_CreateTask_DuplicateName(t *testing.T) {
	scheduler, _, registry := setupCronTestScheduler(t)
	defer scheduler.Stop()

	// 注册测试函数
	mockFn := &MockFunction{name: "test_func"}
	_ = registry.Register(mockFn)

	if err := scheduler.Start(); err != nil {
		t.Fatalf("Failed to start scheduler: %v", err)
	}

	// 创建第一个任务
	task1, err := scheduler.CreateTask("test_cron", "0 * * * * *", "test_func", nil, "")
	if err != nil {
		t.Fatalf("Failed to create first task: %v", err)
	}

	// 创建同名任务（允许重复名称）
	task2, err := scheduler.CreateTask("test_cron", "30 * * * * *", "test_func", nil, "")
	if err != nil {
		t.Fatalf("Should allow duplicate name, but got error: %v", err)
	}

	// 验证两个任务有不同的 ID
	if task1.ID == task2.ID {
		t.Error("Expected different IDs for tasks with duplicate names")
	}
}

func TestCronScheduler_DeleteTask(t *testing.T) {
	scheduler, _, registry := setupCronTestScheduler(t)
	defer scheduler.Stop()

	// 注册测试函数
	mockFn := &MockFunction{name: "test_func"}
	_ = registry.Register(mockFn)

	if err := scheduler.Start(); err != nil {
		t.Fatalf("Failed to start scheduler: %v", err)
	}

	// 创建任务
	task, err := scheduler.CreateTask("test_cron", "0 * * * * *", "test_func", nil, "")
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
	scheduler, _, registry := setupCronTestScheduler(t)
	defer scheduler.Stop()

	// 注册测试函数
	mockFn := &MockFunction{name: "test_func"}
	_ = registry.Register(mockFn)

	if err := scheduler.Start(); err != nil {
		t.Fatalf("Failed to start scheduler: %v", err)
	}

	// 创建一个每秒执行的任务
	task, err := scheduler.CreateTask("test_cron", "* * * * * *", "test_func", nil, "")
	if err != nil {
		t.Fatalf("Failed to create task: %v", err)
	}

	// 等待任务执行
	time.Sleep(1500 * time.Millisecond)

	// 验证函数被执行
	if !mockFn.executed {
		t.Error("Expected function to be executed")
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
	scheduler, _, registry := setupCronTestScheduler(t)
	defer scheduler.Stop()

	// 注册测试函数
	mockFn := &MockFunction{name: "test_func"}
	_ = registry.Register(mockFn)

	if err := scheduler.Start(); err != nil {
		t.Fatalf("Failed to start scheduler: %v", err)
	}

	// 创建多个任务
	for i := 0; i < 3; i++ {
		name := "task_" + string(rune('a'+i))
		_, err := scheduler.CreateTask(name, "0 * * * * *", "test_func", nil, "")
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
	scheduler, _, registry := setupCronTestScheduler(t)
	defer scheduler.Stop()

	// 注册测试函数
	mockFn := &MockFunction{name: "test_func"}
	_ = registry.Register(mockFn)

	if err := scheduler.Start(); err != nil {
		t.Fatalf("Failed to start scheduler: %v", err)
	}

	// 创建5个任务
	for i := 0; i < 5; i++ {
		name := "task_" + string(rune('a'+i))
		_, err := scheduler.CreateTask(name, "0 * * * * *", "test_func", nil, "")
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
	registry := function.NewRegistry()
	testLogger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))

	// 注册测试函数
	mockFn := &MockFunction{name: "test_func"}
	_ = registry.Register(mockFn)

	// 预先创建一个任务（直接写入数据库）
	task := &CronTask{
		Name:         "existing_task",
		CronExpr:     "0 * * * * *",
		FunctionName: "test_func",
	}
	if err := db.Create(task).Error; err != nil {
		t.Fatalf("Failed to create task: %v", err)
	}

	// 创建调度器并启动（会触发恢复）
	scheduler := NewCronScheduler(db, registry, testLogger)
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
		Name:         "test_cron",
		CronExpr:     "0 * * * * *",
		FunctionName: "test_func",
		Description:  "Test task",
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
		Name:         "test_cron",
		CronExpr:     "0 * * * * *",
		FunctionName: "test_func",
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
	scheduler, _, registry := setupCronTestScheduler(t)
	defer scheduler.Stop()

	// 注册测试函数
	mockFn := &MockFunction{name: "test_func"}
	_ = registry.Register(mockFn)

	if err := scheduler.Start(); err != nil {
		t.Fatalf("Failed to start scheduler: %v", err)
	}

	// 创建一个每5秒执行的任务（使用秒级表达式）
	task, err := scheduler.CreateTask("test_cron", "*/5 * * * * *", "test_func", nil, "每5秒执行")
	if err != nil {
		t.Fatalf("Failed to create task: %v", err)
	}

	// 验证任务创建成功
	if task.CronExpr != "*/5 * * * * *" {
		t.Errorf("Expected cron_expr '*/5 * * * * *', got '%s'", task.CronExpr)
	}
}
