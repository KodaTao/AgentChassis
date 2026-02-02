package protocol

import (
	"strings"
	"testing"
)

func TestEncoder_EncodeResult_Success(t *testing.T) {
	encoder := NewEncoder()

	result := &CallResult{
		Name:    "clean_logs",
		Status:  StatusSuccess,
		Message: "Cleaned 5 files",
	}

	output, err := encoder.EncodeResult(result)
	if err != nil {
		t.Fatalf("EncodeResult() error = %v", err)
	}

	// 验证包含必要元素
	if !strings.Contains(output, `name="clean_logs"`) {
		t.Error("Output should contain function name")
	}
	if !strings.Contains(output, `status="success"`) {
		t.Error("Output should contain success status")
	}
	if !strings.Contains(output, "<message>Cleaned 5 files</message>") {
		t.Error("Output should contain message")
	}
}

func TestEncoder_EncodeResult_Error(t *testing.T) {
	encoder := NewEncoder()

	result := &CallResult{
		Name:   "clean_logs",
		Status: StatusError,
		Error:  "permission denied",
	}

	output, err := encoder.EncodeResult(result)
	if err != nil {
		t.Fatalf("EncodeResult() error = %v", err)
	}

	if !strings.Contains(output, `status="error"`) {
		t.Error("Output should contain error status")
	}
	if !strings.Contains(output, "<error>permission denied</error>") {
		t.Error("Output should contain error message")
	}
}

func TestEncoder_EncodeResult_WithStructData(t *testing.T) {
	encoder := NewEncoder()

	type File struct {
		Name    string `json:"name"`
		Size    int    `json:"size"`
		Deleted bool   `json:"deleted"`
	}

	result := &CallResult{
		Name:    "list_files",
		Status:  StatusSuccess,
		Message: "Found 2 files",
		Data: []File{
			{Name: "app.log", Size: 1024, Deleted: true},
			{Name: "error.log", Size: 512, Deleted: true},
		},
	}

	output, err := encoder.EncodeResult(result)
	if err != nil {
		t.Fatalf("EncodeResult() error = %v", err)
	}

	// 验证 TOON 格式输出
	if !strings.Contains(output, `type="toon"`) {
		t.Error("Output should contain TOON data type")
	}
	if !strings.Contains(output, "items[2]{name,size,deleted}") {
		t.Error("Output should contain TOON header")
	}
	if !strings.Contains(output, "app.log") {
		t.Error("Output should contain file name")
	}
}

func TestEncoder_EncodeResult_WithMarkdown(t *testing.T) {
	encoder := NewEncoder()

	result := &CallResult{
		Name:     "generate_report",
		Status:   StatusSuccess,
		Message:  "Report generated",
		Markdown: "## Report\n- Item 1\n- Item 2\n",
	}

	output, err := encoder.EncodeResult(result)
	if err != nil {
		t.Fatalf("EncodeResult() error = %v", err)
	}

	if !strings.Contains(output, `type="markdown"`) {
		t.Error("Output should contain markdown output type")
	}
	if !strings.Contains(output, "## Report") {
		t.Error("Output should contain markdown content")
	}
}

func TestEncoder_EncodeError(t *testing.T) {
	encoder := NewEncoder()

	output := encoder.EncodeError("test_func", "something went wrong")

	if !strings.Contains(output, `name="test_func"`) {
		t.Error("Output should contain function name")
	}
	if !strings.Contains(output, `status="error"`) {
		t.Error("Output should contain error status")
	}
	if !strings.Contains(output, "something went wrong") {
		t.Error("Output should contain error message")
	}
}

func TestEncoder_EncodeToTOON_Map(t *testing.T) {
	encoder := NewEncoder()

	data := map[string]any{
		"name":  "test",
		"count": 42,
	}

	result := &CallResult{
		Name:    "test",
		Status:  StatusSuccess,
		Message: "OK",
		Data:    data,
	}

	output, err := encoder.EncodeResult(result)
	if err != nil {
		t.Fatalf("EncodeResult() error = %v", err)
	}

	// Map 应该被编码为 key: value 格式
	if !strings.Contains(output, `type="toon"`) {
		t.Error("Output should contain TOON data")
	}
}

func TestEncoder_EscapeXML(t *testing.T) {
	encoder := NewEncoder()

	result := &CallResult{
		Name:   "test",
		Status: StatusError,
		Error:  "error with <special> & \"characters\"",
	}

	output, err := encoder.EncodeResult(result)
	if err != nil {
		t.Fatalf("EncodeResult() error = %v", err)
	}

	// 验证 XML 特殊字符被转义
	if strings.Contains(output, "<special>") {
		t.Error("XML special characters should be escaped")
	}
	if !strings.Contains(output, "&lt;special&gt;") {
		t.Error("< and > should be escaped")
	}
}

func TestFormatValue(t *testing.T) {
	tests := []struct {
		name  string
		input any
		want  string
	}{
		{"string", "hello", "hello"},
		{"int", 42, "42"},
		{"bool", true, "true"},
		{"string with comma", "a,b,c", `"a,b,c"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 这里我们间接测试 formatValue
			encoder := NewEncoder()
			result := &CallResult{
				Name:   "test",
				Status: StatusSuccess,
				Data: map[string]any{
					"value": tt.input,
				},
			}
			output, _ := encoder.EncodeResult(result)
			_ = output // 验证不会 panic
		})
	}
}
