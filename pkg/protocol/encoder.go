// Package protocol 提供 XML + TOON 协议解析和编码
package protocol

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
)

// ResultStatus 结果状态
type ResultStatus string

const (
	StatusSuccess ResultStatus = "success"
	StatusError   ResultStatus = "error"
)

// CallResult Function 执行结果
type CallResult struct {
	Name     string       `xml:"name,attr"`
	Status   ResultStatus `xml:"status,attr"`
	Message  string       // 简短消息
	Data     any          // 结构化数据（将编码为 TOON）
	Markdown string       // Markdown 输出
	Error    string       // 错误信息
}

// Encoder 响应编码器
type Encoder struct{}

// NewEncoder 创建编码器实例
func NewEncoder() *Encoder {
	return &Encoder{}
}

// EncodeResult 将执行结果编码为 XML + TOON 格式
// 输出格式：
// <result name="function_name" status="success">
//   <message>操作完成</message>
//   <data type="toon">TOON_CONTENT</data>
//   <output type="markdown">MARKDOWN_CONTENT</output>
// </result>
func (e *Encoder) EncodeResult(result *CallResult) (string, error) {
	var buf bytes.Buffer

	// 写入开始标签
	buf.WriteString(fmt.Sprintf(`<result name="%s" status="%s">`, result.Name, result.Status))
	buf.WriteString("\n")

	// 处理错误情况
	if result.Status == StatusError {
		buf.WriteString(fmt.Sprintf("  <error>%s</error>\n", escapeXML(result.Error)))
		buf.WriteString("</result>")
		return buf.String(), nil
	}

	// 写入消息
	if result.Message != "" {
		buf.WriteString(fmt.Sprintf("  <message>%s</message>\n", escapeXML(result.Message)))
	}

	// 写入数据（TOON 格式）
	if result.Data != nil {
		toonContent, err := e.encodeToTOON(result.Data)
		if err != nil {
			// 如果 TOON 编码失败，退回 JSON
			jsonContent, _ := json.Marshal(result.Data)
			buf.WriteString(fmt.Sprintf("  <data type=\"json\">%s</data>\n", string(jsonContent)))
		} else if toonContent != "" {
			buf.WriteString("  <data type=\"toon\">\n")
			// 缩进 TOON 内容
			for _, line := range strings.Split(toonContent, "\n") {
				buf.WriteString("    " + line + "\n")
			}
			buf.WriteString("  </data>\n")
		}
	}

	// 写入 Markdown 输出
	if result.Markdown != "" {
		buf.WriteString("  <output type=\"markdown\">\n")
		buf.WriteString(result.Markdown)
		if !strings.HasSuffix(result.Markdown, "\n") {
			buf.WriteString("\n")
		}
		buf.WriteString("  </output>\n")
	}

	buf.WriteString("</result>")
	return buf.String(), nil
}

// EncodeError 编码错误响应
func (e *Encoder) EncodeError(funcName string, errMsg string) string {
	return fmt.Sprintf(`<result name="%s" status="error">
  <error>%s</error>
</result>`, funcName, escapeXML(errMsg))
}

// encodeToTOON 将数据编码为 TOON 格式
// 简化实现：对于 slice of struct，生成表格格式
// 对于单个 struct，生成 key: value 格式
func (e *Encoder) encodeToTOON(data any) (string, error) {
	if data == nil {
		return "", nil
	}

	v := reflect.ValueOf(data)

	// 处理指针
	if v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return "", nil
		}
		v = v.Elem()
	}

	switch v.Kind() {
	case reflect.Slice, reflect.Array:
		return e.encodeSliceToTOON(v)
	case reflect.Struct:
		return e.encodeStructToTOON(v)
	case reflect.Map:
		return e.encodeMapToTOON(v)
	default:
		// 简单类型直接返回字符串
		return fmt.Sprintf("%v", v.Interface()), nil
	}
}

// encodeSliceToTOON 将 slice 编码为 TOON 表格格式
// 格式：items[N]{field1,field2}: row1 row2 ...
func (e *Encoder) encodeSliceToTOON(v reflect.Value) (string, error) {
	if v.Len() == 0 {
		return "", nil
	}

	var buf bytes.Buffer

	// 获取第一个元素，确定字段
	elem := v.Index(0)
	if elem.Kind() == reflect.Ptr {
		elem = elem.Elem()
	}

	if elem.Kind() != reflect.Struct {
		// 非结构体 slice，简单列出
		buf.WriteString(fmt.Sprintf("items[%d]:", v.Len()))
		for i := 0; i < v.Len(); i++ {
			if i > 0 {
				buf.WriteString(",")
			}
			buf.WriteString(fmt.Sprintf("%v", v.Index(i).Interface()))
		}
		return buf.String(), nil
	}

	// 结构体 slice，生成表格格式
	t := elem.Type()
	var fields []string
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if field.PkgPath != "" { // 跳过非导出字段
			continue
		}
		// 获取 json tag 或字段名
		name := field.Tag.Get("json")
		if name == "" || name == "-" {
			name = strings.ToLower(field.Name)
		} else {
			name = strings.Split(name, ",")[0]
		}
		fields = append(fields, name)
	}

	// 写入头部：items[N]{field1,field2}:
	buf.WriteString(fmt.Sprintf("items[%d]{%s}:\n", v.Len(), strings.Join(fields, ",")))

	// 写入每一行数据
	for i := 0; i < v.Len(); i++ {
		item := v.Index(i)
		if item.Kind() == reflect.Ptr {
			item = item.Elem()
		}

		var values []string
		for j := 0; j < t.NumField(); j++ {
			if t.Field(j).PkgPath != "" {
				continue
			}
			val := item.Field(j)
			values = append(values, formatValue(val))
		}
		buf.WriteString("  " + strings.Join(values, ",") + "\n")
	}

	return strings.TrimSuffix(buf.String(), "\n"), nil
}

// encodeStructToTOON 将单个 struct 编码为 key: value 格式
func (e *Encoder) encodeStructToTOON(v reflect.Value) (string, error) {
	var buf bytes.Buffer
	t := v.Type()

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if field.PkgPath != "" { // 跳过非导出字段
			continue
		}

		// 获取字段名
		name := field.Tag.Get("json")
		if name == "" || name == "-" {
			name = strings.ToLower(field.Name)
		} else {
			name = strings.Split(name, ",")[0]
		}

		val := v.Field(i)
		buf.WriteString(fmt.Sprintf("%s: %s\n", name, formatValue(val)))
	}

	return strings.TrimSuffix(buf.String(), "\n"), nil
}

// encodeMapToTOON 将 map 编码为 key: value 格式
func (e *Encoder) encodeMapToTOON(v reflect.Value) (string, error) {
	var buf bytes.Buffer

	iter := v.MapRange()
	for iter.Next() {
		key := iter.Key()
		val := iter.Value()
		buf.WriteString(fmt.Sprintf("%v: %s\n", key.Interface(), formatValue(val)))
	}

	return strings.TrimSuffix(buf.String(), "\n"), nil
}

// formatValue 格式化单个值
func formatValue(v reflect.Value) string {
	if !v.IsValid() {
		return ""
	}

	switch v.Kind() {
	case reflect.String:
		s := v.String()
		// 如果包含逗号或换行，需要引用
		if strings.ContainsAny(s, ",\n") {
			return fmt.Sprintf(`"%s"`, strings.ReplaceAll(s, `"`, `\"`))
		}
		return s
	case reflect.Ptr, reflect.Interface:
		if v.IsNil() {
			return ""
		}
		return formatValue(v.Elem())
	default:
		return fmt.Sprintf("%v", v.Interface())
	}
}

// escapeXML 转义 XML 特殊字符
func escapeXML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	s = strings.ReplaceAll(s, "'", "&apos;")
	return s
}
