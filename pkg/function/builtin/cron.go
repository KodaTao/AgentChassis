// Package builtin 提供内置的 Function 实现
package builtin

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"github.com/KodaTao/AgentChassis/pkg/function"
	"github.com/KodaTao/AgentChassis/pkg/scheduler"
)

// CronCreateParams 创建定时任务的参数
type CronCreateParams struct {
	Name         string         `json:"name" desc:"任务名称（描述性，可重复）" required:"true"`
	CronExpr     string         `json:"cron_expr" desc:"Cron表达式（6字段，支持秒级），格式：秒 分 时 日 月 周。例如：'0 30 9 * * *' 每天9:30执行，'*/10 * * * * *' 每10秒执行" required:"true"`
	FunctionName string         `json:"function_name" desc:"要执行的函数名" required:"true"`
	Params       map[string]any `json:"params" desc:"传递给函数的参数"`
	Description  string         `json:"description" desc:"任务描述"`
}

// CronCreateFunction 创建定时任务的函数
type CronCreateFunction struct {
	scheduler *scheduler.CronScheduler
}

// NewCronCreateFunction 创建 CronCreateFunction
func NewCronCreateFunction(s *scheduler.CronScheduler) *CronCreateFunction {
	return &CronCreateFunction{scheduler: s}
}

func (f *CronCreateFunction) Name() string {
	return "cron_create"
}

func (f *CronCreateFunction) Description() string {
	return "创建一个定时任务，按照 Cron 表达式周期性执行指定的函数。使用6字段格式（秒 分 时 日 月 周），例如 '0 30 9 * * *' 表示每天9:30执行。"
}

func (f *CronCreateFunction) ParamsType() reflect.Type {
	return reflect.TypeOf(CronCreateParams{})
}

func (f *CronCreateFunction) Execute(ctx context.Context, params any) (function.Result, error) {
	p := params.(CronCreateParams)

	// 创建任务
	task, err := f.scheduler.CreateTask(p.Name, p.CronExpr, p.FunctionName, p.Params, p.Description)
	if err != nil {
		return function.Result{}, err
	}

	nextRunStr := ""
	if task.NextRunAt != nil {
		nextRunStr = task.NextRunAt.Format("2006-01-02 15:04:05")
	}

	return function.Result{
		Message: fmt.Sprintf("定时任务创建成功（ID: %d），下次执行时间: %s", task.ID, nextRunStr),
		Data: map[string]any{
			"id":            task.ID,
			"name":          task.Name,
			"cron_expr":     task.CronExpr,
			"function_name": task.FunctionName,
			"description":   task.Description,
			"next_run_at":   nextRunStr,
		},
	}, nil
}

// CronListParams 列出定时任务的参数
type CronListParams struct {
	Limit  int `json:"limit" desc:"返回数量限制，默认20" default:"20"`
	Offset int `json:"offset" desc:"偏移量，用于分页" default:"0"`
}

// CronListFunction 列出定时任务的函数
type CronListFunction struct {
	scheduler *scheduler.CronScheduler
}

// NewCronListFunction 创建 CronListFunction
func NewCronListFunction(s *scheduler.CronScheduler) *CronListFunction {
	return &CronListFunction{scheduler: s}
}

func (f *CronListFunction) Name() string {
	return "cron_list"
}

func (f *CronListFunction) Description() string {
	return "列出所有定时任务"
}

func (f *CronListFunction) ParamsType() reflect.Type {
	return reflect.TypeOf(CronListParams{})
}

func (f *CronListFunction) Execute(ctx context.Context, params any) (function.Result, error) {
	p := params.(CronListParams)

	limit := p.Limit
	if limit <= 0 {
		limit = 20
	}

	tasks, err := f.scheduler.ListTasks(limit, p.Offset)
	if err != nil {
		return function.Result{}, err
	}

	total, err := f.scheduler.CountTasks()
	if err != nil {
		return function.Result{}, err
	}

	// 转换为输出格式
	taskList := make([]map[string]any, len(tasks))
	for i, task := range tasks {
		nextRunStr := ""
		if task.NextRunAt != nil {
			nextRunStr = task.NextRunAt.Format(time.RFC3339)
		}
		taskList[i] = map[string]any{
			"id":            task.ID,
			"name":          task.Name,
			"cron_expr":     task.CronExpr,
			"function_name": task.FunctionName,
			"description":   task.Description,
			"next_run_at":   nextRunStr,
			"created_at":    task.CreatedAt.Format(time.RFC3339),
		}
	}

	return function.Result{
		Message: fmt.Sprintf("找到 %d 个定时任务（共 %d 个）", len(tasks), total),
		Data: map[string]any{
			"total":  total,
			"limit":  limit,
			"offset": p.Offset,
			"tasks":  taskList,
		},
	}, nil
}

// CronDeleteParams 删除定时任务的参数
type CronDeleteParams struct {
	ID uint `json:"id" desc:"要删除的任务ID" required:"true"`
}

// CronDeleteFunction 删除定时任务的函数
type CronDeleteFunction struct {
	scheduler *scheduler.CronScheduler
}

// NewCronDeleteFunction 创建 CronDeleteFunction
func NewCronDeleteFunction(s *scheduler.CronScheduler) *CronDeleteFunction {
	return &CronDeleteFunction{scheduler: s}
}

func (f *CronDeleteFunction) Name() string {
	return "cron_delete"
}

func (f *CronDeleteFunction) Description() string {
	return "根据ID删除一个定时任务，同时删除其所有执行历史"
}

func (f *CronDeleteFunction) ParamsType() reflect.Type {
	return reflect.TypeOf(CronDeleteParams{})
}

func (f *CronDeleteFunction) Execute(ctx context.Context, params any) (function.Result, error) {
	p := params.(CronDeleteParams)

	if err := f.scheduler.DeleteTaskByID(p.ID); err != nil {
		return function.Result{}, err
	}

	return function.Result{
		Message: fmt.Sprintf("定时任务（ID: %d）已删除", p.ID),
		Data: map[string]any{
			"id":      p.ID,
			"deleted": true,
		},
	}, nil
}

// CronGetParams 获取定时任务详情的参数
type CronGetParams struct {
	ID uint `json:"id" desc:"任务ID" required:"true"`
}

// CronGetFunction 获取定时任务详情的函数
type CronGetFunction struct {
	scheduler *scheduler.CronScheduler
}

// NewCronGetFunction 创建 CronGetFunction
func NewCronGetFunction(s *scheduler.CronScheduler) *CronGetFunction {
	return &CronGetFunction{scheduler: s}
}

func (f *CronGetFunction) Name() string {
	return "cron_get"
}

func (f *CronGetFunction) Description() string {
	return "根据ID获取定时任务的详细信息"
}

func (f *CronGetFunction) ParamsType() reflect.Type {
	return reflect.TypeOf(CronGetParams{})
}

func (f *CronGetFunction) Execute(ctx context.Context, params any) (function.Result, error) {
	p := params.(CronGetParams)

	task, err := f.scheduler.GetTaskByID(p.ID)
	if err != nil {
		return function.Result{}, err
	}

	nextRunStr := ""
	if task.NextRunAt != nil {
		nextRunStr = task.NextRunAt.Format(time.RFC3339)
	}

	data := map[string]any{
		"id":            task.ID,
		"name":          task.Name,
		"cron_expr":     task.CronExpr,
		"function_name": task.FunctionName,
		"description":   task.Description,
		"params":        task.Params,
		"next_run_at":   nextRunStr,
		"created_at":    task.CreatedAt.Format(time.RFC3339),
		"updated_at":    task.UpdatedAt.Format(time.RFC3339),
	}

	return function.Result{
		Message: fmt.Sprintf("定时任务（ID: %d）: %s，下次执行时间: %s", task.ID, task.Name, nextRunStr),
		Data:    data,
	}, nil
}

// CronHistoryParams 获取执行历史的参数
type CronHistoryParams struct {
	ID     uint `json:"id" desc:"任务ID" required:"true"`
	Limit  int  `json:"limit" desc:"返回数量限制，默认20" default:"20"`
	Offset int  `json:"offset" desc:"偏移量，用于分页" default:"0"`
}

// CronHistoryFunction 获取执行历史的函数
type CronHistoryFunction struct {
	scheduler *scheduler.CronScheduler
}

// NewCronHistoryFunction 创建 CronHistoryFunction
func NewCronHistoryFunction(s *scheduler.CronScheduler) *CronHistoryFunction {
	return &CronHistoryFunction{scheduler: s}
}

func (f *CronHistoryFunction) Name() string {
	return "cron_history"
}

func (f *CronHistoryFunction) Description() string {
	return "获取定时任务的执行历史记录"
}

func (f *CronHistoryFunction) ParamsType() reflect.Type {
	return reflect.TypeOf(CronHistoryParams{})
}

func (f *CronHistoryFunction) Execute(ctx context.Context, params any) (function.Result, error) {
	p := params.(CronHistoryParams)

	limit := p.Limit
	if limit <= 0 {
		limit = 20
	}

	// 先验证任务存在
	task, err := f.scheduler.GetTaskByID(p.ID)
	if err != nil {
		return function.Result{}, err
	}

	executions, err := f.scheduler.GetExecutionHistory(p.ID, limit, p.Offset)
	if err != nil {
		return function.Result{}, err
	}

	total, err := f.scheduler.CountExecutionHistory(p.ID)
	if err != nil {
		return function.Result{}, err
	}

	// 转换为输出格式
	execList := make([]map[string]any, len(executions))
	for i, exec := range executions {
		item := map[string]any{
			"id":           exec.ID,
			"scheduled_at": exec.ScheduledAt.Format(time.RFC3339),
			"started_at":   exec.StartedAt.Format(time.RFC3339),
			"status":       exec.Status,
			"duration_ms":  exec.Duration,
		}
		if exec.FinishedAt != nil {
			item["finished_at"] = exec.FinishedAt.Format(time.RFC3339)
		}
		if exec.Result != "" {
			item["result"] = exec.Result
		}
		if exec.Error != "" {
			item["error"] = exec.Error
		}
		execList[i] = item
	}

	return function.Result{
		Message: fmt.Sprintf("任务 '%s'（ID: %d）的执行历史：共 %d 条记录", task.Name, task.ID, total),
		Data: map[string]any{
			"task_id":    p.ID,
			"task_name":  task.Name,
			"total":      total,
			"limit":      limit,
			"offset":     p.Offset,
			"executions": execList,
		},
	}, nil
}
