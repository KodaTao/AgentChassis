// Package function 提供 Function 接口定义和相关类型
package function

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/KodaTao/AgentChassis/pkg/observability"
)

// Registry 函数注册表
// 线程安全，支持并发读写
type Registry struct {
	mu        sync.RWMutex
	functions map[string]Function
}

// NewRegistry 创建新的注册表
func NewRegistry() *Registry {
	return &Registry{
		functions: make(map[string]Function),
	}
}

// Register 注册一个 Function
// 如果同名 Function 已存在，会被覆盖
func (r *Registry) Register(fn Function) error {
	if fn == nil {
		return ErrNilFunction
	}
	name := fn.Name()
	if name == "" {
		return ErrEmptyFunctionName
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	r.functions[name] = fn
	observability.Info("Function registered", "name", name)
	return nil
}

// RegisterAll 批量注册 Functions
func (r *Registry) RegisterAll(fns ...Function) error {
	for _, fn := range fns {
		if err := r.Register(fn); err != nil {
			return err
		}
	}
	return nil
}

// Get 获取指定名称的 Function
func (r *Registry) Get(name string) (Function, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	fn, ok := r.functions[name]
	return fn, ok
}

// Has 检查是否存在指定名称的 Function
func (r *Registry) Has(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	_, ok := r.functions[name]
	return ok
}

// List 列出所有已注册的 Function 名称
func (r *Registry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.functions))
	for name := range r.functions {
		names = append(names, name)
	}
	return names
}

// ListInfo 列出所有 Function 的详细信息
func (r *Registry) ListInfo() []FunctionInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	infos := make([]FunctionInfo, 0, len(r.functions))
	for _, fn := range r.functions {
		info := FunctionInfo{
			Name:        fn.Name(),
			Description: fn.Description(),
			Parameters:  ExtractParamInfo(fn),
		}
		infos = append(infos, info)
	}
	return infos
}

// Unregister 注销一个 Function
func (r *Registry) Unregister(name string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.functions[name]; ok {
		delete(r.functions, name)
		observability.Info("Function unregistered", "name", name)
		return true
	}
	return false
}

// Count 返回已注册的 Function 数量
func (r *Registry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return len(r.functions)
}

// Execute 执行指定的 Function
// 这是一个便捷方法，内部处理超时和错误捕获
func (r *Registry) Execute(ctx context.Context, name string, params any) (Result, error) {
	fn, ok := r.Get(name)
	if !ok {
		return Result{}, fmt.Errorf("%w: %s", ErrFunctionNotFound, name)
	}

	start := time.Now()
	result, err := fn.Execute(ctx, params)
	duration := time.Since(start)

	// 记录执行日志
	status := "success"
	if err != nil {
		status = "error"
	}
	observability.FunctionCallLog(ctx, name, status, duration.Milliseconds())

	return result, err
}

// 错误定义
var (
	ErrNilFunction       = fmt.Errorf("function cannot be nil")
	ErrEmptyFunctionName = fmt.Errorf("function name cannot be empty")
	ErrFunctionNotFound  = fmt.Errorf("function not found")
)

// DefaultRegistry 默认的全局注册表
var DefaultRegistry = NewRegistry()

// Register 向默认注册表注册 Function
func Register(fn Function) error {
	return DefaultRegistry.Register(fn)
}

// Get 从默认注册表获取 Function
func Get(name string) (Function, bool) {
	return DefaultRegistry.Get(name)
}

// List 列出默认注册表中的所有 Function
func List() []string {
	return DefaultRegistry.List()
}

// ListInfo 列出默认注册表中所有 Function 的信息
func ListInfo() []FunctionInfo {
	return DefaultRegistry.ListInfo()
}
