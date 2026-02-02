// Package scheduler 提供定时任务调度功能
package scheduler

import (
	"errors"
	"time"

	"gorm.io/gorm"
)

// DelayTaskRepository DelayTask 数据访问层
type DelayTaskRepository struct {
	db *gorm.DB
}

// NewDelayTaskRepository 创建 Repository
func NewDelayTaskRepository(db *gorm.DB) *DelayTaskRepository {
	return &DelayTaskRepository{db: db}
}

// Create 创建延时任务
func (r *DelayTaskRepository) Create(task *DelayTask) error {
	return r.db.Create(task).Error
}

// GetByName 根据名称获取任务
func (r *DelayTaskRepository) GetByName(name string) (*DelayTask, error) {
	var task DelayTask
	err := r.db.Where("name = ?", name).First(&task).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrTaskNotFound
		}
		return nil, err
	}
	return &task, nil
}

// GetByID 根据 ID 获取任务
func (r *DelayTaskRepository) GetByID(id uint) (*DelayTask, error) {
	var task DelayTask
	err := r.db.First(&task, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrTaskNotFound
		}
		return nil, err
	}
	return &task, nil
}

// Update 更新任务
func (r *DelayTaskRepository) Update(task *DelayTask) error {
	return r.db.Save(task).Error
}

// Delete 删除任务
func (r *DelayTaskRepository) Delete(name string) error {
	result := r.db.Where("name = ?", name).Delete(&DelayTask{})
	if result.RowsAffected == 0 {
		return ErrTaskNotFound
	}
	return result.Error
}

// List 列出任务
func (r *DelayTaskRepository) List(status *TaskStatus, limit int) ([]DelayTask, error) {
	var tasks []DelayTask
	query := r.db.Model(&DelayTask{})

	if status != nil {
		query = query.Where("status = ?", *status)
	}

	if limit > 0 {
		query = query.Limit(limit)
	}

	err := query.Order("run_at ASC").Find(&tasks).Error
	return tasks, err
}

// ListPending 列出所有待执行的任务
func (r *DelayTaskRepository) ListPending() ([]DelayTask, error) {
	status := StatusPending
	return r.List(&status, 0)
}

// ListByStatus 根据状态列出任务
func (r *DelayTaskRepository) ListByStatus(status TaskStatus) ([]DelayTask, error) {
	return r.List(&status, 0)
}

// UpdateStatus 更新任务状态
func (r *DelayTaskRepository) UpdateStatus(name string, status TaskStatus, result, errMsg string) error {
	updates := map[string]interface{}{
		"status": status,
	}

	if result != "" {
		updates["result"] = result
	}

	if errMsg != "" {
		updates["error"] = errMsg
	}

	if status == StatusCompleted || status == StatusFailed {
		now := time.Now()
		updates["executed_at"] = &now
	}

	res := r.db.Model(&DelayTask{}).Where("name = ?", name).Updates(updates)
	if res.RowsAffected == 0 {
		return ErrTaskNotFound
	}
	return res.Error
}

// MarkAsMissed 标记过期任务为 missed
func (r *DelayTaskRepository) MarkAsMissed(name string) error {
	return r.UpdateStatus(name, StatusMissed, "", "task missed due to server restart")
}

// Cancel 取消任务
func (r *DelayTaskRepository) Cancel(name string) error {
	res := r.db.Model(&DelayTask{}).
		Where("name = ? AND status = ?", name, StatusPending).
		Update("status", StatusCancelled)

	if res.RowsAffected == 0 {
		// 检查是否存在
		var count int64
		r.db.Model(&DelayTask{}).Where("name = ?", name).Count(&count)
		if count == 0 {
			return ErrTaskNotFound
		}
		return ErrTaskNotPending
	}
	return res.Error
}

// Exists 检查任务是否存在
func (r *DelayTaskRepository) Exists(name string) (bool, error) {
	var count int64
	err := r.db.Model(&DelayTask{}).Where("name = ?", name).Count(&count).Error
	return count > 0, err
}

// 错误定义
var (
	ErrTaskNotFound   = errors.New("task not found")
	ErrTaskNotPending = errors.New("task is not in pending status")
	ErrTaskExists     = errors.New("task with this name already exists")
)
