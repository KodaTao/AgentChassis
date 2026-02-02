// Package scheduler 提供定时任务调度功能
package scheduler

import (
	"log/slog"
	"os"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

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
func setupTestScheduler(t *testing.T) (*DelayScheduler, *gorm.DB, *MockAgentExecutor) {
	db := setupTestDB(t)
	testLogger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	mockExecutor := &MockAgentExecutor{}

	scheduler := NewDelayScheduler(db, testLogger)
	scheduler.SetAgentExecutor(mockExecutor)
	return scheduler, db, mockExecutor
}

func TestDelayScheduler_CreateTask(t *testing.T) {
	scheduler, _, _ := setupTestScheduler(t)
	defer scheduler.Stop()

	// 启动调度器
	if err := scheduler.Start(); err != nil {
		t.Fatalf("Failed to start scheduler: %v", err)
	}

	// 创建任务
	runAt := time.Now().Add(1 * time.Hour)
	task, err := scheduler.CreateTask("test_task", runAt, "请发送一条问候消息")
	if err != nil {
		t.Fatalf("Failed to create task: %v", err)
	}

	// 验证任务
	if task.Name != "test_task" {
		t.Errorf("Expected name 'test_task', got '%s'", task.Name)
	}
	if task.Prompt != "请发送一条问候消息" {
		t.Errorf("Expected prompt '请发送一条问候消息', got '%s'", task.Prompt)
	}
	if task.Status != StatusPending {
		t.Errorf("Expected status 'pending', got '%s'", task.Status)
	}
	if task.ID == 0 {
		t.Error("Expected task to have a valid ID")
	}
}

func TestDelayScheduler_CreateTask_EmptyPrompt(t *testing.T) {
	scheduler, _, _ := setupTestScheduler(t)
	defer scheduler.Stop()

	if err := scheduler.Start(); err != nil {
		t.Fatalf("Failed to start scheduler: %v", err)
	}

	// 尝试创建任务，但 prompt 为空
	runAt := time.Now().Add(1 * time.Hour)
	_, err := scheduler.CreateTask("test_task", runAt, "")
	if err == nil {
		t.Error("Expected error when prompt is empty")
	}
}

func TestDelayScheduler_CreateTask_PastTime(t *testing.T) {
	scheduler, _, _ := setupTestScheduler(t)
	defer scheduler.Stop()

	if err := scheduler.Start(); err != nil {
		t.Fatalf("Failed to start scheduler: %v", err)
	}

	// 尝试创建过去时间的任务
	runAt := time.Now().Add(-1 * time.Hour)
	_, err := scheduler.CreateTask("test_task", runAt, "测试提示词")
	if err == nil {
		t.Error("Expected error when run_at is in the past")
	}
}

func TestDelayScheduler_CreateTask_DuplicateName(t *testing.T) {
	scheduler, _, _ := setupTestScheduler(t)
	defer scheduler.Stop()

	if err := scheduler.Start(); err != nil {
		t.Fatalf("Failed to start scheduler: %v", err)
	}

	// 创建第一个任务
	runAt := time.Now().Add(1 * time.Hour)
	task1, err := scheduler.CreateTask("test_task", runAt, "测试提示词1")
	if err != nil {
		t.Fatalf("Failed to create first task: %v", err)
	}

	// 创建同名任务（现在应该成功，因为允许重复名称）
	task2, err := scheduler.CreateTask("test_task", runAt.Add(time.Hour), "测试提示词2")
	if err != nil {
		t.Fatalf("Should allow duplicate name, but got error: %v", err)
	}

	// 验证两个任务有不同的 ID
	if task1.ID == task2.ID {
		t.Error("Expected different IDs for tasks with duplicate names")
	}
}

func TestDelayScheduler_CancelTaskByID(t *testing.T) {
	scheduler, _, _ := setupTestScheduler(t)
	defer scheduler.Stop()

	if err := scheduler.Start(); err != nil {
		t.Fatalf("Failed to start scheduler: %v", err)
	}

	// 创建任务
	runAt := time.Now().Add(1 * time.Hour)
	task, err := scheduler.CreateTask("test_task", runAt, "测试提示词")
	if err != nil {
		t.Fatalf("Failed to create task: %v", err)
	}

	// 取消任务
	if err := scheduler.CancelTaskByID(task.ID); err != nil {
		t.Fatalf("Failed to cancel task: %v", err)
	}

	// 验证状态
	retrieved, err := scheduler.GetTaskByID(task.ID)
	if err != nil {
		t.Fatalf("Failed to get task: %v", err)
	}
	if retrieved.Status != StatusCancelled {
		t.Errorf("Expected status 'cancelled', got '%s'", retrieved.Status)
	}
}

func TestDelayScheduler_ExecuteTask(t *testing.T) {
	scheduler, _, mockExecutor := setupTestScheduler(t)
	defer scheduler.Stop()

	if err := scheduler.Start(); err != nil {
		t.Fatalf("Failed to start scheduler: %v", err)
	}

	// 创建一个马上执行的任务
	runAt := time.Now().Add(100 * time.Millisecond)
	task, err := scheduler.CreateTask("test_task", runAt, "请问候用户")
	if err != nil {
		t.Fatalf("Failed to create task: %v", err)
	}

	// 等待任务执行
	time.Sleep(500 * time.Millisecond)

	// 验证 AgentExecutor 被调用
	if mockExecutor.ExecutionCount() == 0 {
		t.Error("Expected AgentExecutor to be called")
	}

	// 验证执行时传递的 prompt
	if mockExecutor.LastPrompt() != "请问候用户" {
		t.Errorf("Expected prompt '请问候用户', got '%s'", mockExecutor.LastPrompt())
	}

	// 验证状态变为 completed
	retrieved, err := scheduler.GetTaskByID(task.ID)
	if err != nil {
		t.Fatalf("Failed to get task: %v", err)
	}
	if retrieved.Status != StatusCompleted {
		t.Errorf("Expected status 'completed', got '%s'", retrieved.Status)
	}
}

func TestDelayScheduler_ListTasks(t *testing.T) {
	scheduler, _, _ := setupTestScheduler(t)
	defer scheduler.Stop()

	if err := scheduler.Start(); err != nil {
		t.Fatalf("Failed to start scheduler: %v", err)
	}

	// 创建多个任务
	runAt := time.Now().Add(1 * time.Hour)
	for i := 0; i < 3; i++ {
		name := "task_" + string(rune('a'+i))
		_, err := scheduler.CreateTask(name, runAt.Add(time.Duration(i)*time.Minute), "测试提示词")
		if err != nil {
			t.Fatalf("Failed to create task %s: %v", name, err)
		}
	}

	// 列出所有任务
	tasks, err := scheduler.ListTasks(nil, 20, 0)
	if err != nil {
		t.Fatalf("Failed to list tasks: %v", err)
	}
	if len(tasks) != 3 {
		t.Errorf("Expected 3 tasks, got %d", len(tasks))
	}

	// 列出 pending 任务
	status := StatusPending
	tasks, err = scheduler.ListTasks(&status, 20, 0)
	if err != nil {
		t.Fatalf("Failed to list pending tasks: %v", err)
	}
	if len(tasks) != 3 {
		t.Errorf("Expected 3 pending tasks, got %d", len(tasks))
	}
}

func TestDelayScheduler_ListTasks_Pagination(t *testing.T) {
	scheduler, _, _ := setupTestScheduler(t)
	defer scheduler.Stop()

	if err := scheduler.Start(); err != nil {
		t.Fatalf("Failed to start scheduler: %v", err)
	}

	// 创建5个任务
	runAt := time.Now().Add(1 * time.Hour)
	for i := 0; i < 5; i++ {
		name := "task_" + string(rune('a'+i))
		_, err := scheduler.CreateTask(name, runAt.Add(time.Duration(i)*time.Minute), "测试提示词")
		if err != nil {
			t.Fatalf("Failed to create task %s: %v", name, err)
		}
	}

	// 测试分页：limit=2, offset=0
	tasks, err := scheduler.ListTasks(nil, 2, 0)
	if err != nil {
		t.Fatalf("Failed to list tasks: %v", err)
	}
	if len(tasks) != 2 {
		t.Errorf("Expected 2 tasks with limit=2, got %d", len(tasks))
	}

	// 测试分页：limit=2, offset=2
	tasks, err = scheduler.ListTasks(nil, 2, 2)
	if err != nil {
		t.Fatalf("Failed to list tasks: %v", err)
	}
	if len(tasks) != 2 {
		t.Errorf("Expected 2 tasks with limit=2,offset=2, got %d", len(tasks))
	}

	// 测试分页：limit=2, offset=4
	tasks, err = scheduler.ListTasks(nil, 2, 4)
	if err != nil {
		t.Fatalf("Failed to list tasks: %v", err)
	}
	if len(tasks) != 1 {
		t.Errorf("Expected 1 task with limit=2,offset=4, got %d", len(tasks))
	}

	// 测试总数
	count, err := scheduler.CountTasks(nil)
	if err != nil {
		t.Fatalf("Failed to count tasks: %v", err)
	}
	if count != 5 {
		t.Errorf("Expected count 5, got %d", count)
	}
}

func TestDelayScheduler_RecoverTasks(t *testing.T) {
	db := setupTestDB(t)
	testLogger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	mockExecutor := &MockAgentExecutor{}

	// 预先创建一个过期任务
	expiredTask := &DelayTask{
		Name:   "expired_task",
		Prompt: "过期任务提示词",
		RunAt:  time.Now().Add(-1 * time.Hour), // 过去 1 小时
		Status: StatusPending,
	}
	if err := db.Create(expiredTask).Error; err != nil {
		t.Fatalf("Failed to create expired task: %v", err)
	}
	expiredID := expiredTask.ID

	// 预先创建一个未过期任务
	futureTask := &DelayTask{
		Name:   "future_task",
		Prompt: "未来任务提示词",
		RunAt:  time.Now().Add(1 * time.Hour), // 未来 1 小时
		Status: StatusPending,
	}
	if err := db.Create(futureTask).Error; err != nil {
		t.Fatalf("Failed to create future task: %v", err)
	}
	futureID := futureTask.ID

	// 创建调度器并启动（会触发恢复）
	scheduler := NewDelayScheduler(db, testLogger)
	scheduler.SetAgentExecutor(mockExecutor)
	if err := scheduler.Start(); err != nil {
		t.Fatalf("Failed to start scheduler: %v", err)
	}
	defer scheduler.Stop()

	// 验证过期任务被标记为 missed
	task1, err := scheduler.GetTaskByID(expiredID)
	if err != nil {
		t.Fatalf("Failed to get expired task: %v", err)
	}
	if task1.Status != StatusMissed {
		t.Errorf("Expected expired task status 'missed', got '%s'", task1.Status)
	}

	// 验证未过期任务仍为 pending
	task2, err := scheduler.GetTaskByID(futureID)
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
		Name:   "test_task",
		Prompt: "测试提示词",
		RunAt:  time.Now().Add(1 * time.Hour),
		Status: StatusPending,
	}
	if err := repo.Create(task); err != nil {
		t.Fatalf("Failed to create task: %v", err)
	}

	// Read by ID
	retrieved, err := repo.GetByID(task.ID)
	if err != nil {
		t.Fatalf("Failed to get task by ID: %v", err)
	}
	if retrieved.Name != "test_task" {
		t.Errorf("Expected name 'test_task', got '%s'", retrieved.Name)
	}

	// Update
	task.Status = StatusRunning
	if err := repo.Update(task); err != nil {
		t.Fatalf("Failed to update task: %v", err)
	}
	updated, _ := repo.GetByID(task.ID)
	if updated.Status != StatusRunning {
		t.Errorf("Expected status 'running', got '%s'", updated.Status)
	}

	// UpdateStatusByID
	if err := repo.UpdateStatusByID(task.ID, StatusCompleted, "done", ""); err != nil {
		t.Fatalf("Failed to update status: %v", err)
	}
	completed, _ := repo.GetByID(task.ID)
	if completed.Status != StatusCompleted {
		t.Errorf("Expected status 'completed', got '%s'", completed.Status)
	}

	// DeleteByID
	if err := repo.DeleteByID(task.ID); err != nil {
		t.Fatalf("Failed to delete task: %v", err)
	}
	_, err = repo.GetByID(task.ID)
	if err != ErrTaskNotFound {
		t.Errorf("Expected ErrTaskNotFound after delete, got %v", err)
	}
}

func TestRepository_CancelByID(t *testing.T) {
	db := setupTestDB(t)
	repo := NewDelayTaskRepository(db)

	// 创建 pending 任务
	task := &DelayTask{
		Name:   "test_task",
		Prompt: "测试提示词",
		RunAt:  time.Now().Add(1 * time.Hour),
		Status: StatusPending,
	}
	_ = repo.Create(task)

	// 取消任务
	if err := repo.CancelByID(task.ID); err != nil {
		t.Fatalf("Failed to cancel task: %v", err)
	}

	// 验证状态
	cancelled, _ := repo.GetByID(task.ID)
	if cancelled.Status != StatusCancelled {
		t.Errorf("Expected status 'cancelled', got '%s'", cancelled.Status)
	}

	// 尝试再次取消（应该失败，因为已经不是 pending）
	err := repo.CancelByID(task.ID)
	if err != ErrTaskNotPending {
		t.Errorf("Expected ErrTaskNotPending, got %v", err)
	}
}

func TestRepository_List(t *testing.T) {
	db := setupTestDB(t)
	repo := NewDelayTaskRepository(db)

	// 创建多个不同状态的任务
	tasks := []DelayTask{
		{Name: "task1", Prompt: "prompt1", RunAt: time.Now().Add(1 * time.Hour), Status: StatusPending},
		{Name: "task2", Prompt: "prompt2", RunAt: time.Now().Add(2 * time.Hour), Status: StatusPending},
		{Name: "task3", Prompt: "prompt3", RunAt: time.Now().Add(3 * time.Hour), Status: StatusCompleted},
	}
	for i := range tasks {
		_ = repo.Create(&tasks[i])
	}

	// 列出所有
	all, err := repo.List(nil, 20, 0)
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

	// 带 limit 和 offset
	limited, err := repo.List(nil, 2, 0)
	if err != nil {
		t.Fatalf("Failed to list with limit: %v", err)
	}
	if len(limited) != 2 {
		t.Errorf("Expected 2 tasks with limit, got %d", len(limited))
	}

	// 测试 Count
	count, err := repo.Count(nil)
	if err != nil {
		t.Fatalf("Failed to count tasks: %v", err)
	}
	if count != 3 {
		t.Errorf("Expected count 3, got %d", count)
	}

	// 测试 Count with status
	statusPending := StatusPending
	countPending, err := repo.Count(&statusPending)
	if err != nil {
		t.Fatalf("Failed to count pending tasks: %v", err)
	}
	if countPending != 2 {
		t.Errorf("Expected pending count 2, got %d", countPending)
	}
}
