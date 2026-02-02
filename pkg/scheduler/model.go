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
	Name         string     `gorm:"uniqueIndex;not null" json:"name"`          // 任务名称（唯一标识）
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

// CronTask 重复性定时任务（预留，后续实现）
type CronTask struct {
	gorm.Model
	Name         string     `gorm:"uniqueIndex;not null" json:"name"`    // 任务名称
	CronExpr     string     `gorm:"not null" json:"cron_expr"`           // Cron 表达式
	FunctionName string     `gorm:"not null" json:"function_name"`       // 要执行的函数
	Params       string     `gorm:"type:text" json:"params,omitempty"`   // 参数（JSON 格式）
	Enabled      bool       `gorm:"default:true" json:"enabled"`         // 是否启用
	Until        *time.Time `json:"until,omitempty"`                     // 失效时间（可选）
	MaxRuns      int        `gorm:"default:0" json:"max_runs"`           // 最大执行次数，0表示无限
	RunCount     int        `gorm:"default:0" json:"run_count"`          // 已执行次数
	LastRunAt    *time.Time `json:"last_run_at,omitempty"`               // 最后执行时间
	LastStatus   string     `json:"last_status,omitempty"`               // 最后执行状态
}

// TableName 指定表名
func (CronTask) TableName() string {
	return "cron_tasks"
}
