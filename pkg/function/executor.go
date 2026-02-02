// Package function 提供 Function 接口定义和相关类型
package function

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"github.com/KodaTao/AgentChassis/pkg/observability"
)

// Executor 函数执行器
// 封装函数执行的通用逻辑，包括超时控制、错误处理等
type Executor struct {
	registry *Registry
	timeout  time.Duration
}

// NewExecutor 创建函数执行器
func NewExecutor(registry *Registry, timeout time.Duration) *Executor {
	if timeout == 0 {
		timeout = 30 * time.Second // 默认超时 30 秒
	}
	return &Executor{
		registry: registry,
		timeout:  timeout,
	}
}

// ExecuteRequest 执行请求
type ExecuteRequest struct {
	FunctionName string
	Params       map[string]string // 原始参数（key: value 字符串）
	Data         string            // TOON 格式的数据（可选）
}

// ExecuteResponse 执行响应
type ExecuteResponse struct {
	Result   Result
	Duration time.Duration
	Error    error
}

// Execute 执行函数
// 自动处理参数解析、超时控制和错误捕获
func (e *Executor) Execute(ctx context.Context, req ExecuteRequest) ExecuteResponse {
	start := time.Now()

	// 获取函数
	fn, ok := e.registry.Get(req.FunctionName)
	if !ok {
		return ExecuteResponse{
			Error:    fmt.Errorf("%w: %s", ErrFunctionNotFound, req.FunctionName),
			Duration: time.Since(start),
		}
	}

	// 创建带超时的 context
	execCtx, cancel := context.WithTimeout(ctx, e.timeout)
	defer cancel()

	// 解析参数
	params, err := e.parseParams(fn, req.Params)
	if err != nil {
		return ExecuteResponse{
			Error:    fmt.Errorf("failed to parse params: %w", err),
			Duration: time.Since(start),
		}
	}

	// 执行函数（带 panic 恢复）
	result, execErr := e.executeWithRecover(execCtx, fn, params)
	duration := time.Since(start)

	// 记录日志
	status := "success"
	if execErr != nil {
		status = "error"
	}
	observability.FunctionCallLog(ctx, req.FunctionName, status, duration.Milliseconds())

	return ExecuteResponse{
		Result:   result,
		Duration: duration,
		Error:    execErr,
	}
}

// parseParams 解析参数
func (e *Executor) parseParams(fn Function, rawParams map[string]string) (any, error) {
	paramType := fn.ParamsType()
	if paramType == nil {
		return nil, nil
	}

	// 创建参数实例
	var paramValue reflect.Value
	if paramType.Kind() == reflect.Ptr {
		paramValue = reflect.New(paramType.Elem())
	} else {
		paramValue = reflect.New(paramType)
	}

	// 填充参数
	if err := ParseParams(rawParams, paramValue.Interface()); err != nil {
		return nil, err
	}

	// 如果原始类型不是指针，返回值而非指针
	if paramType.Kind() != reflect.Ptr {
		return paramValue.Elem().Interface(), nil
	}
	return paramValue.Interface(), nil
}

// executeWithRecover 执行函数并恢复 panic
func (e *Executor) executeWithRecover(ctx context.Context, fn Function, params any) (result Result, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("function panicked: %v", r)
			observability.Error("Function panicked",
				"function", fn.Name(),
				"panic", r,
			)
		}
	}()

	// 使用 channel 实现超时控制
	done := make(chan struct{})
	var execResult Result
	var execErr error

	go func() {
		defer close(done)
		execResult, execErr = fn.Execute(ctx, params)
	}()

	select {
	case <-done:
		return execResult, execErr
	case <-ctx.Done():
		return Result{}, fmt.Errorf("function execution timeout: %w", ctx.Err())
	}
}

// ExecuteAsync 异步执行函数
// 返回一个 channel，完成后会收到结果
func (e *Executor) ExecuteAsync(ctx context.Context, req ExecuteRequest) <-chan ExecuteResponse {
	ch := make(chan ExecuteResponse, 1)

	go func() {
		defer close(ch)
		resp := e.Execute(ctx, req)
		ch <- resp
	}()

	return ch
}

// SetTimeout 设置超时时间
func (e *Executor) SetTimeout(timeout time.Duration) {
	e.timeout = timeout
}

// GetTimeout 获取超时时间
func (e *Executor) GetTimeout() time.Duration {
	return e.timeout
}
