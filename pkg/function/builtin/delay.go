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

// DelayCreateParams 创建延时任务的参数
type DelayCreateParams struct {
	Name    string `json:"name" desc:"任务名称（描述性，可重复）" required:"true"`
	RunAt   string `json:"run_at" desc:"执行时间，ISO8601格式，如 2024-01-15T10:30:00+08:00" required:"true"`
	Prompt  string `json:"prompt" desc:"任务触发时发送给AI的提示词，AI会根据提示词决定执行什么操作" required:"true"`
	Channel string `json:"channel" desc:"渠道上下文JSON，如 {\"type\":\"console\"} 或 {\"type\":\"telegram\",\"chat_id\":\"123\"}"`
}

// DelayCreateFunction 创建延时任务的函数
type DelayCreateFunction struct {
	scheduler *scheduler.DelayScheduler
}

// NewDelayCreateFunction 创建 DelayCreateFunction
func NewDelayCreateFunction(s *scheduler.DelayScheduler) *DelayCreateFunction {
	return &DelayCreateFunction{scheduler: s}
}

func (f *DelayCreateFunction) Name() string {
	return "delay_create"
}

func (f *DelayCreateFunction) Description() string {
	return "创建一个延时任务，在指定时间触发AI执行任务。执行时间必须是未来时间点（ISO8601格式）。触发时AI会根据prompt决定执行什么操作。创建成功后返回任务ID。"
}

func (f *DelayCreateFunction) ParamsType() reflect.Type {
	return reflect.TypeOf(DelayCreateParams{})
}

func (f *DelayCreateFunction) Execute(ctx context.Context, params any) (function.Result, error) {
	p := params.(DelayCreateParams)

	// 解析时间
	runAt, err := time.Parse(time.RFC3339, p.RunAt)
	if err != nil {
		return function.Result{}, fmt.Errorf("invalid run_at format, expected ISO8601/RFC3339: %v", err)
	}

	// 构建完整的 prompt（包含渠道信息）
	fullPrompt := p.Prompt
	if p.Channel != "" {
		fullPrompt = fmt.Sprintf("【渠道信息：%s】\n%s", p.Channel, p.Prompt)
	}

	// 创建任务（传递渠道信息）
	task, err := f.scheduler.CreateTask(p.Name, runAt, fullPrompt, p.Channel)
	if err != nil {
		return function.Result{}, err
	}

	data := map[string]any{
		"id":     task.ID,
		"name":   task.Name,
		"prompt": task.Prompt,
		"run_at": task.RunAt.Format(time.RFC3339),
		"status": task.Status,
	}
	if task.Channel != "" {
		data["channel"] = task.Channel
	}

	return function.Result{
		Message: fmt.Sprintf("延时任务创建成功（ID: %d），将在 %s 触发AI执行",
			task.ID, task.RunAt.Format("2006-01-02 15:04:05")),
		Data: data,
	}, nil
}

// DelayListParams 列出延时任务的参数
type DelayListParams struct {
	Status string `json:"status" desc:"按状态筛选：pending/running/completed/failed/cancelled/missed，不填则返回所有"`
	Limit  int    `json:"limit" desc:"返回数量限制，默认20" default:"20"`
	Offset int    `json:"offset" desc:"偏移量，用于分页" default:"0"`
}

// DelayListFunction 列出延时任务的函数
type DelayListFunction struct {
	scheduler *scheduler.DelayScheduler
}

// NewDelayListFunction 创建 DelayListFunction
func NewDelayListFunction(s *scheduler.DelayScheduler) *DelayListFunction {
	return &DelayListFunction{scheduler: s}
}

func (f *DelayListFunction) Name() string {
	return "delay_list"
}

func (f *DelayListFunction) Description() string {
	return "列出延时任务，可按状态筛选。状态包括：pending（待执行）、running（执行中）、completed（已完成）、failed（失败）、cancelled（已取消）、missed（错过执行）"
}

func (f *DelayListFunction) ParamsType() reflect.Type {
	return reflect.TypeOf(DelayListParams{})
}

func (f *DelayListFunction) Execute(ctx context.Context, params any) (function.Result, error) {
	p := params.(DelayListParams)

	limit := p.Limit
	if limit <= 0 {
		limit = 20
	}

	var status *scheduler.TaskStatus
	if p.Status != "" {
		s := scheduler.TaskStatus(p.Status)
		// 验证状态值
		switch s {
		case scheduler.StatusPending, scheduler.StatusRunning, scheduler.StatusCompleted,
			scheduler.StatusFailed, scheduler.StatusCancelled, scheduler.StatusMissed:
			status = &s
		default:
			return function.Result{}, fmt.Errorf("invalid status: %s", p.Status)
		}
	}

	tasks, err := f.scheduler.ListTasks(status, limit, p.Offset)
	if err != nil {
		return function.Result{}, err
	}

	total, err := f.scheduler.CountTasks(status)
	if err != nil {
		return function.Result{}, err
	}

	// 转换为输出格式
	taskList := make([]map[string]any, len(tasks))
	for i, task := range tasks {
		taskList[i] = map[string]any{
			"id":         task.ID,
			"name":       task.Name,
			"prompt":     task.Prompt,
			"run_at":     task.RunAt.Format(time.RFC3339),
			"status":     task.Status,
			"created_at": task.CreatedAt.Format(time.RFC3339),
		}
		if task.ExecutedAt != nil {
			taskList[i]["executed_at"] = task.ExecutedAt.Format(time.RFC3339)
		}
		if task.Error != "" {
			taskList[i]["error"] = task.Error
		}
	}

	message := fmt.Sprintf("找到 %d 个延时任务（共 %d 个）", len(tasks), total)
	if status != nil {
		message = fmt.Sprintf("找到 %d 个状态为 %s 的延时任务（共 %d 个）", len(tasks), *status, total)
	}

	return function.Result{
		Message: message,
		Data: map[string]any{
			"total":  total,
			"limit":  limit,
			"offset": p.Offset,
			"tasks":  taskList,
		},
	}, nil
}

// DelayCancelParams 取消延时任务的参数
type DelayCancelParams struct {
	ID uint `json:"id" desc:"要取消的任务ID" required:"true"`
}

// DelayCancelFunction 取消延时任务的函数
type DelayCancelFunction struct {
	scheduler *scheduler.DelayScheduler
}

// NewDelayCancelFunction 创建 DelayCancelFunction
func NewDelayCancelFunction(s *scheduler.DelayScheduler) *DelayCancelFunction {
	return &DelayCancelFunction{scheduler: s}
}

func (f *DelayCancelFunction) Name() string {
	return "delay_cancel"
}

func (f *DelayCancelFunction) Description() string {
	return "根据ID取消一个待执行的延时任务。只有状态为 pending 的任务才能被取消。"
}

func (f *DelayCancelFunction) ParamsType() reflect.Type {
	return reflect.TypeOf(DelayCancelParams{})
}

func (f *DelayCancelFunction) Execute(ctx context.Context, params any) (function.Result, error) {
	p := params.(DelayCancelParams)

	if err := f.scheduler.CancelTaskByID(p.ID); err != nil {
		return function.Result{}, err
	}

	return function.Result{
		Message: fmt.Sprintf("延时任务（ID: %d）已取消", p.ID),
		Data: map[string]any{
			"id":     p.ID,
			"status": "cancelled",
		},
	}, nil
}

// DelayGetParams 获取延时任务详情的参数
type DelayGetParams struct {
	ID uint `json:"id" desc:"任务ID" required:"true"`
}

// DelayGetFunction 获取延时任务详情的函数
type DelayGetFunction struct {
	scheduler *scheduler.DelayScheduler
}

// NewDelayGetFunction 创建 DelayGetFunction
func NewDelayGetFunction(s *scheduler.DelayScheduler) *DelayGetFunction {
	return &DelayGetFunction{scheduler: s}
}

func (f *DelayGetFunction) Name() string {
	return "delay_get"
}

func (f *DelayGetFunction) Description() string {
	return "根据ID获取延时任务的详细信息"
}

func (f *DelayGetFunction) ParamsType() reflect.Type {
	return reflect.TypeOf(DelayGetParams{})
}

func (f *DelayGetFunction) Execute(ctx context.Context, params any) (function.Result, error) {
	p := params.(DelayGetParams)

	task, err := f.scheduler.GetTaskByID(p.ID)
	if err != nil {
		return function.Result{}, err
	}

	data := map[string]any{
		"id":         task.ID,
		"name":       task.Name,
		"prompt":     task.Prompt,
		"run_at":     task.RunAt.Format(time.RFC3339),
		"status":     task.Status,
		"created_at": task.CreatedAt.Format(time.RFC3339),
		"updated_at": task.UpdatedAt.Format(time.RFC3339),
	}

	if task.ExecutedAt != nil {
		data["executed_at"] = task.ExecutedAt.Format(time.RFC3339)
	}
	if task.Result != "" {
		data["result"] = task.Result
	}
	if task.Error != "" {
		data["error"] = task.Error
	}

	statusDesc := ""
	switch task.Status {
	case scheduler.StatusPending:
		statusDesc = fmt.Sprintf("任务将在 %s 执行", task.RunAt.Format("2006-01-02 15:04:05"))
	case scheduler.StatusRunning:
		statusDesc = "任务正在执行中"
	case scheduler.StatusCompleted:
		statusDesc = "任务已完成"
	case scheduler.StatusFailed:
		statusDesc = fmt.Sprintf("任务执行失败: %s", task.Error)
	case scheduler.StatusCancelled:
		statusDesc = "任务已取消"
	case scheduler.StatusMissed:
		statusDesc = "任务错过执行（服务重启时已过期）"
	}

	return function.Result{
		Message: fmt.Sprintf("任务（ID: %d）的状态: %s", task.ID, statusDesc),
		Data:    data,
	}, nil
}
