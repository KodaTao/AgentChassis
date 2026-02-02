// Package scheduler 提供定时任务调度功能
package scheduler

import (
	"time"

	"gorm.io/gorm"
)

// TaskStatus 任务状态
type TaskStatus string

const (
	StatusPending   TaskStatus = "pending"   // 等待执行
	StatusRunning   TaskStatus = "running"   // 正在执行
	StatusCompleted TaskStatus = "completed" // 执行完成
	StatusFailed    TaskStatus = "failed"    // 执行失败
	StatusCancelled TaskStatus = "cancelled" // 已取消
	StatusMissed    TaskStatus = "missed"    // 错过执行（重启时已过期）
)

// DelayTask 一次性延时任务
type DelayTask struct {
	gorm.Model
	Name         string     `gorm:"not null" json:"name"`                      // 任务名称（描述性，可重复）
	RunAt        time.Time  `gorm:"not null;index" json:"run_at"`              // 执行时间点（绝对时间）
	FunctionName string     `gorm:"not null" json:"function_name"`             // 要执行的函数名
	Params       string     `gorm:"type:text" json:"params,omitempty"`         // 函数参数（JSON 格式）
	Status       TaskStatus `gorm:"default:pending;index" json:"status"`       // 任务状态
	Result       string     `gorm:"type:text" json:"result,omitempty"`         // 执行结果
	Error        string     `gorm:"type:text" json:"error,omitempty"`          // 错误信息
	ExecutedAt   *time.Time `json:"executed_at,omitempty"`                     // 实际执行时间
}

// TableName 指定表名
func (DelayTask) TableName() string {
	return "delay_tasks"
}

// IsExpired 检查任务是否已过期
func (t *DelayTask) IsExpired() bool {
	return time.Now().After(t.RunAt)
}

// IsPending 检查任务是否处于等待状态
func (t *DelayTask) IsPending() bool {
	return t.Status == StatusPending
}

// CanCancel 检查任务是否可以取消
func (t *DelayTask) CanCancel() bool {
	return t.Status == StatusPending
}

// TimeUntilRun 返回距离执行的时间
func (t *DelayTask) TimeUntilRun() time.Duration {
	return time.Until(t.RunAt)
}

// CronTask 重复性定时任务
type CronTask struct {
	gorm.Model
	Name         string `gorm:"not null" json:"name"`                // 任务名称（描述性，可重复）
	CronExpr     string `gorm:"not null" json:"cron_expr"`           // Cron 表达式（6字段，支持秒级）
	FunctionName string `gorm:"not null" json:"function_name"`       // 要执行的函数名
	Params       string `gorm:"type:text" json:"params,omitempty"`   // 函数参数（JSON 格式）
	Description  string `gorm:"type:text" json:"description"`        // 任务描述
	NextRunAt    *time.Time `json:"next_run_at,omitempty"`           // 下次执行时间
}

// TableName 指定表名
func (CronTask) TableName() string {
	return "cron_tasks"
}

// CronExecutionStatus 定时任务执行状态
type CronExecutionStatus string

const (
	CronStatusRunning   CronExecutionStatus = "running"   // 正在执行
	CronStatusCompleted CronExecutionStatus = "completed" // 执行完成
	CronStatusFailed    CronExecutionStatus = "failed"    // 执行失败
)

// CronExecution 定时任务执行历史记录
type CronExecution struct {
	gorm.Model
	CronTaskID  uint                `gorm:"not null;index" json:"cron_task_id"` // 关联的 CronTask ID
	ScheduledAt time.Time           `gorm:"not null;index" json:"scheduled_at"` // 计划执行时间
	StartedAt   time.Time           `gorm:"not null" json:"started_at"`         // 开始执行时间
	FinishedAt  *time.Time          `json:"finished_at,omitempty"`              // 结束时间
	Status      CronExecutionStatus `gorm:"not null;index" json:"status"`       // 执行状态
	Result      string              `gorm:"type:text" json:"result,omitempty"`  // 执行结果
	Error       string              `gorm:"type:text" json:"error,omitempty"`   // 错误信息
	Duration    int64               `json:"duration_ms,omitempty"`              // 执行耗时（毫秒）
}

// TableName 指定表名
func (CronExecution) TableName() string {
	return "cron_executions"
}
