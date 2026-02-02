// Package scheduler 提供定时任务调度功能
package scheduler

import (
	"context"
	"log/slog"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/KodaTao/AgentChassis/pkg/function"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// MockParams 测试参数
type MockParams struct {
	Message string `json:"message"`
}

// MockFunction 测试用的 Function
type MockFunction struct {
	name        string
	executed    bool
	executedAt  time.Time
	executeFunc func(ctx context.Context, params any) (function.Result, error)
}

func (f *MockFunction) Name() string {
	return f.name
}

func (f *MockFunction) Description() string {
	return "Test function"
}

func (f *MockFunction) ParamsType() reflect.Type {
	return reflect.TypeOf(MockParams{})
}

func (f *MockFunction) Execute(ctx context.Context, params any) (function.Result, error) {
	f.executed = true
	f.executedAt = time.Now()
	if f.executeFunc != nil {
		return f.executeFunc(ctx, params)
	}
	return function.Result{Message: "executed"}, nil
}

// setupTestDB 创建测试数据库
func setupTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	// 自动迁移
	if err := db.AutoMigrate(&DelayTask{}); err != nil {
		t.Fatalf("Failed to migrate: %v", err)
	}

	return db
}

// setupTestScheduler 创建测试调度器
func setupTestScheduler(t *testing.T) (*DelayScheduler, *gorm.DB, *function.Registry) {
	db := setupTestDB(t)
	registry := function.NewRegistry()
	testLogger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))

	scheduler := NewDelayScheduler(db, registry, testLogger)
	return scheduler, db, registry
}

func TestDelayScheduler_CreateTask(t *testing.T) {
	scheduler, _, registry := setupTestScheduler(t)
	defer scheduler.Stop()

	// 注册测试函数
	mockFn := &MockFunction{name: "test_func"}
	_ = registry.Register(mockFn)

	// 启动调度器
	if err := scheduler.Start(); err != nil {
		t.Fatalf("Failed to start scheduler: %v", err)
	}

	// 创建任务
	runAt := time.Now().Add(1 * time.Hour)
	task, err := scheduler.CreateTask("test_task", "test_func", runAt, map[string]any{"message": "hello"})
	if err != nil {
		t.Fatalf("Failed to create task: %v", err)
	}

	// 验证任务
	if task.Name != "test_task" {
		t.Errorf("Expected name 'test_task', got '%s'", task.Name)
	}
	if task.FunctionName != "test_func" {
		t.Errorf("Expected function_name 'test_func', got '%s'", task.FunctionName)
	}
	if task.Status != StatusPending {
		t.Errorf("Expected status 'pending', got '%s'", task.Status)
	}
}

func TestDelayScheduler_CreateTask_FunctionNotFound(t *testing.T) {
	scheduler, _, _ := setupTestScheduler(t)
	defer scheduler.Stop()

	if err := scheduler.Start(); err != nil {
		t.Fatalf("Failed to start scheduler: %v", err)
	}

	// 尝试创建任务，但函数不存在
	runAt := time.Now().Add(1 * time.Hour)
	_, err := scheduler.CreateTask("test_task", "nonexistent_func", runAt, nil)
	if err == nil {
		t.Error("Expected error when function not found")
	}
}

func TestDelayScheduler_CreateTask_PastTime(t *testing.T) {
	scheduler, _, registry := setupTestScheduler(t)
	defer scheduler.Stop()

	// 注册测试函数
	mockFn := &MockFunction{name: "test_func"}
	_ = registry.Register(mockFn)

	if err := scheduler.Start(); err != nil {
		t.Fatalf("Failed to start scheduler: %v", err)
	}

	// 尝试创建过去时间的任务
	runAt := time.Now().Add(-1 * time.Hour)
	_, err := scheduler.CreateTask("test_task", "test_func", runAt, nil)
	if err == nil {
		t.Error("Expected error when run_at is in the past")
	}
}

func TestDelayScheduler_CreateTask_DuplicateName(t *testing.T) {
	scheduler, _, registry := setupTestScheduler(t)
	defer scheduler.Stop()

	// 注册测试函数
	mockFn := &MockFunction{name: "test_func"}
	_ = registry.Register(mockFn)

	if err := scheduler.Start(); err != nil {
		t.Fatalf("Failed to start scheduler: %v", err)
	}

	// 创建第一个任务
	runAt := time.Now().Add(1 * time.Hour)
	_, err := scheduler.CreateTask("test_task", "test_func", runAt, nil)
	if err != nil {
		t.Fatalf("Failed to create first task: %v", err)
	}

	// 尝试创建同名任务
	_, err = scheduler.CreateTask("test_task", "test_func", runAt.Add(time.Hour), nil)
	if err != ErrTaskExists {
		t.Errorf("Expected ErrTaskExists, got %v", err)
	}
}

func TestDelayScheduler_CancelTask(t *testing.T) {
	scheduler, _, registry := setupTestScheduler(t)
	defer scheduler.Stop()

	// 注册测试函数
	mockFn := &MockFunction{name: "test_func"}
	_ = registry.Register(mockFn)

	if err := scheduler.Start(); err != nil {
		t.Fatalf("Failed to start scheduler: %v", err)
	}

	// 创建任务
	runAt := time.Now().Add(1 * time.Hour)
	_, err := scheduler.CreateTask("test_task", "test_func", runAt, nil)
	if err != nil {
		t.Fatalf("Failed to create task: %v", err)
	}

	// 取消任务
	if err := scheduler.CancelTask("test_task"); err != nil {
		t.Fatalf("Failed to cancel task: %v", err)
	}

	// 验证状态
	task, err := scheduler.GetTask("test_task")
	if err != nil {
		t.Fatalf("Failed to get task: %v", err)
	}
	if task.Status != StatusCancelled {
		t.Errorf("Expected status 'cancelled', got '%s'", task.Status)
	}
}

func TestDelayScheduler_ExecuteTask(t *testing.T) {
	scheduler, _, registry := setupTestScheduler(t)
	defer scheduler.Stop()

	// 注册测试函数
	mockFn := &MockFunction{name: "test_func"}
	_ = registry.Register(mockFn)

	if err := scheduler.Start(); err != nil {
		t.Fatalf("Failed to start scheduler: %v", err)
	}

	// 创建一个马上执行的任务
	runAt := time.Now().Add(100 * time.Millisecond)
	_, err := scheduler.CreateTask("test_task", "test_func", runAt, nil)
	if err != nil {
		t.Fatalf("Failed to create task: %v", err)
	}

	// 等待任务执行
	time.Sleep(500 * time.Millisecond)

	// 验证函数被执行
	if !mockFn.executed {
		t.Error("Expected function to be executed")
	}

	// 验证状态变为 completed
	task, err := scheduler.GetTask("test_task")
	if err != nil {
		t.Fatalf("Failed to get task: %v", err)
	}
	if task.Status != StatusCompleted {
		t.Errorf("Expected status 'completed', got '%s'", task.Status)
	}
}

func TestDelayScheduler_ListTasks(t *testing.T) {
	scheduler, _, registry := setupTestScheduler(t)
	defer scheduler.Stop()

	// 注册测试函数
	mockFn := &MockFunction{name: "test_func"}
	_ = registry.Register(mockFn)

	if err := scheduler.Start(); err != nil {
		t.Fatalf("Failed to start scheduler: %v", err)
	}

	// 创建多个任务
	runAt := time.Now().Add(1 * time.Hour)
	for i := 0; i < 3; i++ {
		name := "task_" + string(rune('a'+i))
		_, err := scheduler.CreateTask(name, "test_func", runAt.Add(time.Duration(i)*time.Minute), nil)
		if err != nil {
			t.Fatalf("Failed to create task %s: %v", name, err)
		}
	}

	// 列出所有任务
	tasks, err := scheduler.ListTasks(nil, 0)
	if err != nil {
		t.Fatalf("Failed to list tasks: %v", err)
	}
	if len(tasks) != 3 {
		t.Errorf("Expected 3 tasks, got %d", len(tasks))
	}

	// 列出 pending 任务
	status := StatusPending
	tasks, err = scheduler.ListTasks(&status, 0)
	if err != nil {
		t.Fatalf("Failed to list pending tasks: %v", err)
	}
	if len(tasks) != 3 {
		t.Errorf("Expected 3 pending tasks, got %d", len(tasks))
	}
}

func TestDelayScheduler_RecoverTasks(t *testing.T) {
	db := setupTestDB(t)
	registry := function.NewRegistry()
	testLogger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))

	// 注册测试函数
	mockFn := &MockFunction{name: "test_func"}
	_ = registry.Register(mockFn)

	// 预先创建一个过期任务
	expiredTask := &DelayTask{
		Name:         "expired_task",
		FunctionName: "test_func",
		RunAt:        time.Now().Add(-1 * time.Hour), // 过去 1 小时
		Status:       StatusPending,
	}
	if err := db.Create(expiredTask).Error; err != nil {
		t.Fatalf("Failed to create expired task: %v", err)
	}

	// 预先创建一个未过期任务
	futureTask := &DelayTask{
		Name:         "future_task",
		FunctionName: "test_func",
		RunAt:        time.Now().Add(1 * time.Hour), // 未来 1 小时
		Status:       StatusPending,
	}
	if err := db.Create(futureTask).Error; err != nil {
		t.Fatalf("Failed to create future task: %v", err)
	}

	// 创建调度器并启动（会触发恢复）
	scheduler := NewDelayScheduler(db, registry, testLogger)
	if err := scheduler.Start(); err != nil {
		t.Fatalf("Failed to start scheduler: %v", err)
	}
	defer scheduler.Stop()

	// 验证过期任务被标记为 missed
	task1, err := scheduler.GetTask("expired_task")
	if err != nil {
		t.Fatalf("Failed to get expired task: %v", err)
	}
	if task1.Status != StatusMissed {
		t.Errorf("Expected expired task status 'missed', got '%s'", task1.Status)
	}

	// 验证未过期任务仍为 pending
	task2, err := scheduler.GetTask("future_task")
	if err != nil {
		t.Fatalf("Failed to get future task: %v", err)
	}
	if task2.Status != StatusPending {
		t.Errorf("Expected future task status 'pending', got '%s'", task2.Status)
	}
}

func TestRepository_CRUD(t *testing.T) {
	db := setupTestDB(t)
	repo := NewDelayTaskRepository(db)

	// Create
	task := &DelayTask{
		Name:         "test_task",
		FunctionName: "test_func",
		RunAt:        time.Now().Add(1 * time.Hour),
		Status:       StatusPending,
	}
	if err := repo.Create(task); err != nil {
		t.Fatalf("Failed to create task: %v", err)
	}

	// Read by name
	retrieved, err := repo.GetByName("test_task")
	if err != nil {
		t.Fatalf("Failed to get task by name: %v", err)
	}
	if retrieved.Name != "test_task" {
		t.Errorf("Expected name 'test_task', got '%s'", retrieved.Name)
	}

	// Read by ID
	retrievedByID, err := repo.GetByID(task.ID)
	if err != nil {
		t.Fatalf("Failed to get task by ID: %v", err)
	}
	if retrievedByID.ID != task.ID {
		t.Errorf("Expected ID %d, got %d", task.ID, retrievedByID.ID)
	}

	// Update
	task.Status = StatusRunning
	if err := repo.Update(task); err != nil {
		t.Fatalf("Failed to update task: %v", err)
	}
	updated, _ := repo.GetByName("test_task")
	if updated.Status != StatusRunning {
		t.Errorf("Expected status 'running', got '%s'", updated.Status)
	}

	// UpdateStatus
	if err := repo.UpdateStatus("test_task", StatusCompleted, "done", ""); err != nil {
		t.Fatalf("Failed to update status: %v", err)
	}
	completed, _ := repo.GetByName("test_task")
	if completed.Status != StatusCompleted {
		t.Errorf("Expected status 'completed', got '%s'", completed.Status)
	}

	// Delete
	if err := repo.Delete("test_task"); err != nil {
		t.Fatalf("Failed to delete task: %v", err)
	}
	_, err = repo.GetByName("test_task")
	if err != ErrTaskNotFound {
		t.Errorf("Expected ErrTaskNotFound after delete, got %v", err)
	}
}

func TestRepository_Cancel(t *testing.T) {
	db := setupTestDB(t)
	repo := NewDelayTaskRepository(db)

	// 创建 pending 任务
	task := &DelayTask{
		Name:         "test_task",
		FunctionName: "test_func",
		RunAt:        time.Now().Add(1 * time.Hour),
		Status:       StatusPending,
	}
	_ = repo.Create(task)

	// 取消任务
	if err := repo.Cancel("test_task"); err != nil {
		t.Fatalf("Failed to cancel task: %v", err)
	}

	// 验证状态
	cancelled, _ := repo.GetByName("test_task")
	if cancelled.Status != StatusCancelled {
		t.Errorf("Expected status 'cancelled', got '%s'", cancelled.Status)
	}

	// 尝试再次取消（应该失败，因为已经不是 pending）
	err := repo.Cancel("test_task")
	if err != ErrTaskNotPending {
		t.Errorf("Expected ErrTaskNotPending, got %v", err)
	}
}

func TestRepository_List(t *testing.T) {
	db := setupTestDB(t)
	repo := NewDelayTaskRepository(db)

	// 创建多个不同状态的任务
	tasks := []DelayTask{
		{Name: "task1", FunctionName: "f1", RunAt: time.Now().Add(1 * time.Hour), Status: StatusPending},
		{Name: "task2", FunctionName: "f2", RunAt: time.Now().Add(2 * time.Hour), Status: StatusPending},
		{Name: "task3", FunctionName: "f3", RunAt: time.Now().Add(3 * time.Hour), Status: StatusCompleted},
	}
	for i := range tasks {
		_ = repo.Create(&tasks[i])
	}

	// 列出所有
	all, err := repo.List(nil, 0)
	if err != nil {
		t.Fatalf("Failed to list all tasks: %v", err)
	}
	if len(all) != 3 {
		t.Errorf("Expected 3 tasks, got %d", len(all))
	}

	// 列出 pending
	pending, err := repo.ListPending()
	if err != nil {
		t.Fatalf("Failed to list pending tasks: %v", err)
	}
	if len(pending) != 2 {
		t.Errorf("Expected 2 pending tasks, got %d", len(pending))
	}

	// 带 limit
	limited, err := repo.List(nil, 2)
	if err != nil {
		t.Fatalf("Failed to list with limit: %v", err)
	}
	if len(limited) != 2 {
		t.Errorf("Expected 2 tasks with limit, got %d", len(limited))
	}
}
