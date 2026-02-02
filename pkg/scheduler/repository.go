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

// DeleteByID 根据 ID 删除任务
func (r *DelayTaskRepository) DeleteByID(id uint) error {
	result := r.db.Delete(&DelayTask{}, id)
	if result.RowsAffected == 0 {
		return ErrTaskNotFound
	}
	return result.Error
}

// List 列出任务
func (r *DelayTaskRepository) List(status *TaskStatus, limit, offset int) ([]DelayTask, error) {
	var tasks []DelayTask
	query := r.db.Model(&DelayTask{})

	if status != nil {
		query = query.Where("status = ?", *status)
	}

	if limit > 0 {
		query = query.Limit(limit)
	}

	if offset > 0 {
		query = query.Offset(offset)
	}

	err := query.Order("run_at ASC").Find(&tasks).Error
	return tasks, err
}

// ListPending 列出所有待执行的任务
func (r *DelayTaskRepository) ListPending() ([]DelayTask, error) {
	status := StatusPending
	return r.List(&status, 0, 0)
}

// ListByStatus 根据状态列出任务
func (r *DelayTaskRepository) ListByStatus(status TaskStatus) ([]DelayTask, error) {
	return r.List(&status, 0, 0)
}

// Count 统计任务数量
func (r *DelayTaskRepository) Count(status *TaskStatus) (int64, error) {
	var count int64
	query := r.db.Model(&DelayTask{})
	if status != nil {
		query = query.Where("status = ?", *status)
	}
	err := query.Count(&count).Error
	return count, err
}

// UpdateStatusByID 根据 ID 更新任务状态
func (r *DelayTaskRepository) UpdateStatusByID(id uint, status TaskStatus, result, errMsg string) error {
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

	res := r.db.Model(&DelayTask{}).Where("id = ?", id).Updates(updates)
	if res.RowsAffected == 0 {
		return ErrTaskNotFound
	}
	return res.Error
}

// MarkAsMissedByID 根据 ID 标记过期任务为 missed
func (r *DelayTaskRepository) MarkAsMissedByID(id uint) error {
	return r.UpdateStatusByID(id, StatusMissed, "", "task missed due to server restart")
}

// CancelByID 根据 ID 取消任务
func (r *DelayTaskRepository) CancelByID(id uint) error {
	res := r.db.Model(&DelayTask{}).
		Where("id = ? AND status = ?", id, StatusPending).
		Update("status", StatusCancelled)

	if res.RowsAffected == 0 {
		// 检查是否存在
		var count int64
		r.db.Model(&DelayTask{}).Where("id = ?", id).Count(&count)
		if count == 0 {
			return ErrTaskNotFound
		}
		return ErrTaskNotPending
	}
	return res.Error
}

// 错误定义
var (
	ErrTaskNotFound   = errors.New("task not found")
	ErrTaskNotPending = errors.New("task is not in pending status")
)
