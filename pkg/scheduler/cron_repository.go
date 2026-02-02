// Package scheduler 提供定时任务调度功能
package scheduler

import (
	"errors"

	"gorm.io/gorm"
)

// CronTask 相关错误定义
var (
	ErrCronTaskNotFound = errors.New("cron task not found")
	ErrExecutionNotFound = errors.New("cron execution not found")
)

// CronTaskRepository Cron 任务 Repository
type CronTaskRepository struct {
	db *gorm.DB
}

// NewCronTaskRepository 创建 CronTaskRepository
func NewCronTaskRepository(db *gorm.DB) *CronTaskRepository {
	return &CronTaskRepository{db: db}
}

// Create 创建任务
func (r *CronTaskRepository) Create(task *CronTask) error {
	return r.db.Create(task).Error
}

// GetByID 根据 ID 获取任务
func (r *CronTaskRepository) GetByID(id uint) (*CronTask, error) {
	var task CronTask
	if err := r.db.First(&task, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrCronTaskNotFound
		}
		return nil, err
	}
	return &task, nil
}

// Update 更新任务
func (r *CronTaskRepository) Update(task *CronTask) error {
	return r.db.Save(task).Error
}

// UpdateNextRunAt 更新下次执行时间
func (r *CronTaskRepository) UpdateNextRunAt(id uint, nextRunAt interface{}) error {
	return r.db.Model(&CronTask{}).Where("id = ?", id).Update("next_run_at", nextRunAt).Error
}

// DeleteByID 根据 ID 删除任务
func (r *CronTaskRepository) DeleteByID(id uint) error {
	result := r.db.Delete(&CronTask{}, id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrCronTaskNotFound
	}
	return nil
}

// List 列出任务
func (r *CronTaskRepository) List(limit, offset int) ([]CronTask, error) {
	var tasks []CronTask
	query := r.db.Order("id DESC")
	if limit > 0 {
		query = query.Limit(limit)
	}
	if offset > 0 {
		query = query.Offset(offset)
	}
	if err := query.Find(&tasks).Error; err != nil {
		return nil, err
	}
	return tasks, nil
}

// Count 统计任务数量
func (r *CronTaskRepository) Count() (int64, error) {
	var count int64
	if err := r.db.Model(&CronTask{}).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

// ListAll 列出所有任务（用于恢复调度）
func (r *CronTaskRepository) ListAll() ([]CronTask, error) {
	var tasks []CronTask
	if err := r.db.Find(&tasks).Error; err != nil {
		return nil, err
	}
	return tasks, nil
}

// CronExecutionRepository Cron 执行历史 Repository
type CronExecutionRepository struct {
	db *gorm.DB
}

// NewCronExecutionRepository 创建 CronExecutionRepository
func NewCronExecutionRepository(db *gorm.DB) *CronExecutionRepository {
	return &CronExecutionRepository{db: db}
}

// Create 创建执行记录
func (r *CronExecutionRepository) Create(exec *CronExecution) error {
	return r.db.Create(exec).Error
}

// GetByID 根据 ID 获取执行记录
func (r *CronExecutionRepository) GetByID(id uint) (*CronExecution, error) {
	var exec CronExecution
	if err := r.db.First(&exec, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrExecutionNotFound
		}
		return nil, err
	}
	return &exec, nil
}

// Update 更新执行记录
func (r *CronExecutionRepository) Update(exec *CronExecution) error {
	return r.db.Save(exec).Error
}

// ListByTaskID 根据任务 ID 列出执行历史
func (r *CronExecutionRepository) ListByTaskID(taskID uint, limit, offset int) ([]CronExecution, error) {
	var execs []CronExecution
	query := r.db.Where("cron_task_id = ?", taskID).Order("id DESC")
	if limit > 0 {
		query = query.Limit(limit)
	}
	if offset > 0 {
		query = query.Offset(offset)
	}
	if err := query.Find(&execs).Error; err != nil {
		return nil, err
	}
	return execs, nil
}

// CountByTaskID 统计任务的执行历史数量
func (r *CronExecutionRepository) CountByTaskID(taskID uint) (int64, error) {
	var count int64
	if err := r.db.Model(&CronExecution{}).Where("cron_task_id = ?", taskID).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

// DeleteByTaskID 删除任务的所有执行历史
func (r *CronExecutionRepository) DeleteByTaskID(taskID uint) error {
	return r.db.Where("cron_task_id = ?", taskID).Delete(&CronExecution{}).Error
}

// ListByTaskIDWithStatus 根据任务 ID 和状态列出执行历史
func (r *CronExecutionRepository) ListByTaskIDWithStatus(taskID uint, status CronExecutionStatus, limit, offset int) ([]CronExecution, error) {
	var execs []CronExecution
	query := r.db.Where("cron_task_id = ? AND status = ?", taskID, status).Order("id DESC")
	if limit > 0 {
		query = query.Limit(limit)
	}
	if offset > 0 {
		query = query.Offset(offset)
	}
	if err := query.Find(&execs).Error; err != nil {
		return nil, err
	}
	return execs, nil
}
