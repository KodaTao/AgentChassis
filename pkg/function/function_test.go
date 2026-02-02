package function

import (
	"context"
	"reflect"
	"testing"
	"time"
)

// MockFunction 测试用的 Mock 函数
type MockFunction struct {
	name        string
	description string
	paramsType  reflect.Type
	executeFunc func(ctx context.Context, params any) (Result, error)
}

func (m *MockFunction) Name() string        { return m.name }
func (m *MockFunction) Description() string { return m.description }
func (m *MockFunction) ParamsType() reflect.Type { return m.paramsType }
func (m *MockFunction) Execute(ctx context.Context, params any) (Result, error) {
	if m.executeFunc != nil {
		return m.executeFunc(ctx, params)
	}
	return Result{Message: "executed"}, nil
}

// TestParams 测试用的参数结构
type TestParams struct {
	Name    string `json:"name" desc:"名称" required:"true"`
	Count   int    `json:"count" desc:"数量" default:"10"`
	Enabled bool   `json:"enabled" desc:"是否启用"`
}

func TestRegistry_Register(t *testing.T) {
	registry := NewRegistry()

	fn := &MockFunction{
		name:        "test_func",
		description: "A test function",
	}

	// 正常注册
	err := registry.Register(fn)
	if err != nil {
		t.Errorf("Register() error = %v", err)
	}

	// 验证注册成功
	if !registry.Has("test_func") {
		t.Error("Registry should have the registered function")
	}

	// 注册 nil 应该失败
	err = registry.Register(nil)
	if err != ErrNilFunction {
		t.Errorf("Register(nil) should return ErrNilFunction, got %v", err)
	}

	// 注册空名称应该失败
	emptyNameFn := &MockFunction{name: ""}
	err = registry.Register(emptyNameFn)
	if err != ErrEmptyFunctionName {
		t.Errorf("Register(empty name) should return ErrEmptyFunctionName, got %v", err)
	}
}

func TestRegistry_Get(t *testing.T) {
	registry := NewRegistry()

	fn := &MockFunction{
		name:        "get_test",
		description: "Test get",
	}
	registry.Register(fn)

	// 获取存在的函数
	got, ok := registry.Get("get_test")
	if !ok {
		t.Error("Get() should return true for existing function")
	}
	if got.Name() != "get_test" {
		t.Errorf("Get() returned wrong function, got %s", got.Name())
	}

	// 获取不存在的函数
	_, ok = registry.Get("not_exist")
	if ok {
		t.Error("Get() should return false for non-existing function")
	}
}

func TestRegistry_List(t *testing.T) {
	registry := NewRegistry()

	registry.Register(&MockFunction{name: "func1"})
	registry.Register(&MockFunction{name: "func2"})
	registry.Register(&MockFunction{name: "func3"})

	list := registry.List()
	if len(list) != 3 {
		t.Errorf("List() returned %d items, want 3", len(list))
	}
}

func TestRegistry_ListInfo(t *testing.T) {
	registry := NewRegistry()

	registry.Register(&MockFunction{
		name:        "info_test",
		description: "Test description",
		paramsType:  reflect.TypeOf(TestParams{}),
	})

	infos := registry.ListInfo()
	if len(infos) != 1 {
		t.Fatalf("ListInfo() returned %d items, want 1", len(infos))
	}

	info := infos[0]
	if info.Name != "info_test" {
		t.Errorf("ListInfo()[0].Name = %s, want info_test", info.Name)
	}
	if info.Description != "Test description" {
		t.Errorf("ListInfo()[0].Description = %s, want 'Test description'", info.Description)
	}
	if len(info.Parameters) != 3 {
		t.Errorf("ListInfo()[0].Parameters has %d items, want 3", len(info.Parameters))
	}
}

func TestRegistry_Unregister(t *testing.T) {
	registry := NewRegistry()

	registry.Register(&MockFunction{name: "to_remove"})

	// 注销存在的函数
	removed := registry.Unregister("to_remove")
	if !removed {
		t.Error("Unregister() should return true for existing function")
	}

	// 验证已删除
	if registry.Has("to_remove") {
		t.Error("Function should be removed after Unregister()")
	}

	// 注销不存在的函数
	removed = registry.Unregister("not_exist")
	if removed {
		t.Error("Unregister() should return false for non-existing function")
	}
}

func TestExtractParamInfo(t *testing.T) {
	fn := &MockFunction{
		name:       "param_test",
		paramsType: reflect.TypeOf(TestParams{}),
	}

	params := ExtractParamInfo(fn)

	if len(params) != 3 {
		t.Fatalf("ExtractParamInfo() returned %d params, want 3", len(params))
	}

	// 验证 Name 参数
	nameParam := findParam(params, "name")
	if nameParam == nil {
		t.Fatal("Should have 'name' parameter")
	}
	if nameParam.Type != "string" {
		t.Errorf("name.Type = %s, want string", nameParam.Type)
	}
	if !nameParam.Required {
		t.Error("name should be required")
	}
	if nameParam.Description != "名称" {
		t.Errorf("name.Description = %s, want '名称'", nameParam.Description)
	}

	// 验证 Count 参数
	countParam := findParam(params, "count")
	if countParam == nil {
		t.Fatal("Should have 'count' parameter")
	}
	if countParam.Type != "integer" {
		t.Errorf("count.Type = %s, want integer", countParam.Type)
	}
	if countParam.Default != "10" {
		t.Errorf("count.Default = %s, want 10", countParam.Default)
	}
}

func findParam(params []ParamInfo, name string) *ParamInfo {
	for i := range params {
		if params[i].Name == name {
			return &params[i]
		}
	}
	return nil
}

func TestParseParams(t *testing.T) {
	rawParams := map[string]string{
		"name":    "test",
		"count":   "42",
		"enabled": "true",
	}

	var params TestParams
	err := ParseParams(rawParams, &params)
	if err != nil {
		t.Fatalf("ParseParams() error = %v", err)
	}

	if params.Name != "test" {
		t.Errorf("params.Name = %s, want test", params.Name)
	}
	if params.Count != 42 {
		t.Errorf("params.Count = %d, want 42", params.Count)
	}
	if !params.Enabled {
		t.Error("params.Enabled should be true")
	}
}

func TestParseParams_Default(t *testing.T) {
	rawParams := map[string]string{
		"name": "test",
		// count 使用默认值
	}

	var params TestParams
	err := ParseParams(rawParams, &params)
	if err != nil {
		t.Fatalf("ParseParams() error = %v", err)
	}

	if params.Count != 10 {
		t.Errorf("params.Count = %d, want 10 (default)", params.Count)
	}
}

func TestExecutor_Execute(t *testing.T) {
	registry := NewRegistry()

	fn := &MockFunction{
		name:       "exec_test",
		paramsType: reflect.TypeOf(TestParams{}),
		executeFunc: func(ctx context.Context, params any) (Result, error) {
			p := params.(TestParams)
			return Result{
				Message: "Hello " + p.Name,
				Data:    map[string]int{"count": p.Count},
			}, nil
		},
	}
	registry.Register(fn)

	executor := NewExecutor(registry, 5*time.Second)

	resp := executor.Execute(context.Background(), ExecuteRequest{
		FunctionName: "exec_test",
		Params: map[string]string{
			"name":  "World",
			"count": "5",
		},
	})

	if resp.Error != nil {
		t.Fatalf("Execute() error = %v", resp.Error)
	}

	if resp.Result.Message != "Hello World" {
		t.Errorf("Result.Message = %s, want 'Hello World'", resp.Result.Message)
	}
}

func TestExecutor_Timeout(t *testing.T) {
	registry := NewRegistry()

	fn := &MockFunction{
		name: "slow_func",
		executeFunc: func(ctx context.Context, params any) (Result, error) {
			// 模拟慢函数
			select {
			case <-time.After(5 * time.Second):
				return Result{Message: "done"}, nil
			case <-ctx.Done():
				return Result{}, ctx.Err()
			}
		},
	}
	registry.Register(fn)

	executor := NewExecutor(registry, 100*time.Millisecond) // 100ms 超时

	resp := executor.Execute(context.Background(), ExecuteRequest{
		FunctionName: "slow_func",
	})

	if resp.Error == nil {
		t.Error("Execute() should return timeout error")
	}
}

func TestExecutor_NotFound(t *testing.T) {
	registry := NewRegistry()
	executor := NewExecutor(registry, 5*time.Second)

	resp := executor.Execute(context.Background(), ExecuteRequest{
		FunctionName: "not_exist",
	})

	if resp.Error == nil {
		t.Error("Execute() should return error for non-existing function")
	}
}

func TestToSnakeCase(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Name", "name"},
		{"UserName", "user_name"},
		{"APIKey", "a_p_i_key"},
		{"simple", "simple"},
		{"XMLParser", "x_m_l_parser"},
	}

	for _, tt := range tests {
		got := toSnakeCase(tt.input)
		if got != tt.want {
			t.Errorf("toSnakeCase(%s) = %s, want %s", tt.input, got, tt.want)
		}
	}
}
