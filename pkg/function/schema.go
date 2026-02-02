// Package function 提供 Function 接口定义和相关类型
package function

import (
	"reflect"
	"strings"
)

// ExtractParamInfo 从 Function 中提取参数信息
// 使用反射读取参数结构体的字段和 tag
func ExtractParamInfo(fn Function) []ParamInfo {
	paramType := fn.ParamsType()
	if paramType == nil {
		return nil
	}

	// 如果是指针，获取元素类型
	if paramType.Kind() == reflect.Ptr {
		paramType = paramType.Elem()
	}

	// 只处理结构体类型
	if paramType.Kind() != reflect.Struct {
		return nil
	}

	return extractStructParams(paramType)
}

// extractStructParams 从结构体类型提取参数信息
func extractStructParams(t reflect.Type) []ParamInfo {
	var params []ParamInfo

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)

		// 跳过非导出字段
		if field.PkgPath != "" {
			continue
		}

		// 跳过嵌入字段（匿名字段）
		if field.Anonymous {
			// 递归处理嵌入的结构体
			if field.Type.Kind() == reflect.Struct {
				embedded := extractStructParams(field.Type)
				params = append(params, embedded...)
			}
			continue
		}

		param := ParamInfo{
			Name:        getFieldName(field),
			Type:        getTypeName(field.Type),
			Description: field.Tag.Get("desc"),
			Required:    isRequired(field),
			Default:     field.Tag.Get("default"),
		}

		params = append(params, param)
	}

	return params
}

// getFieldName 获取字段名称
// 优先使用 json tag，否则使用字段名（转小写）
func getFieldName(field reflect.StructField) string {
	// 尝试 json tag
	jsonTag := field.Tag.Get("json")
	if jsonTag != "" && jsonTag != "-" {
		parts := strings.Split(jsonTag, ",")
		if parts[0] != "" {
			return parts[0]
		}
	}

	// 尝试 toon tag
	toonTag := field.Tag.Get("toon")
	if toonTag != "" && toonTag != "-" {
		parts := strings.Split(toonTag, ",")
		if parts[0] != "" {
			return parts[0]
		}
	}

	// 默认使用字段名（转小写下划线）
	return toSnakeCase(field.Name)
}

// getTypeName 获取类型的可读名称
func getTypeName(t reflect.Type) string {
	switch t.Kind() {
	case reflect.String:
		return "string"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return "integer"
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return "integer"
	case reflect.Float32, reflect.Float64:
		return "number"
	case reflect.Bool:
		return "boolean"
	case reflect.Slice, reflect.Array:
		elemType := getTypeName(t.Elem())
		return "array[" + elemType + "]"
	case reflect.Map:
		keyType := getTypeName(t.Key())
		valType := getTypeName(t.Elem())
		return "map[" + keyType + "]" + valType
	case reflect.Ptr:
		return getTypeName(t.Elem())
	case reflect.Struct:
		return "object"
	default:
		return t.String()
	}
}

// isRequired 判断字段是否必填
func isRequired(field reflect.StructField) bool {
	// 检查 required tag
	requiredTag := field.Tag.Get("required")
	if requiredTag == "true" || requiredTag == "1" {
		return true
	}

	// 检查 validate tag (常见的验证库格式)
	validateTag := field.Tag.Get("validate")
	if strings.Contains(validateTag, "required") {
		return true
	}

	// 检查 binding tag (gin 框架格式)
	bindingTag := field.Tag.Get("binding")
	if strings.Contains(bindingTag, "required") {
		return true
	}

	return false
}

// toSnakeCase 将驼峰命名转换为下划线命名
func toSnakeCase(s string) string {
	var result strings.Builder
	for i, r := range s {
		if i > 0 && r >= 'A' && r <= 'Z' {
			result.WriteByte('_')
		}
		if r >= 'A' && r <= 'Z' {
			result.WriteByte(byte(r + 32)) // 转小写
		} else {
			result.WriteRune(r)
		}
	}
	return result.String()
}

// ParseParams 将 map[string]string 解析为目标结构体
// 使用反射填充结构体字段
func ParseParams(params map[string]string, target any) error {
	v := reflect.ValueOf(target)
	if v.Kind() != reflect.Ptr || v.IsNil() {
		return ErrInvalidTarget
	}

	v = v.Elem()
	if v.Kind() != reflect.Struct {
		return ErrInvalidTarget
	}

	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		fieldValue := v.Field(i)

		if !fieldValue.CanSet() {
			continue
		}

		// 获取参数名
		paramName := getFieldName(field)
		paramValue, ok := params[paramName]
		if !ok {
			// 检查是否有默认值
			defaultVal := field.Tag.Get("default")
			if defaultVal != "" {
				paramValue = defaultVal
			} else {
				continue
			}
		}

		// 设置字段值
		if err := setFieldValue(fieldValue, paramValue); err != nil {
			return err
		}
	}

	return nil
}

// setFieldValue 设置字段值
func setFieldValue(field reflect.Value, value string) error {
	switch field.Kind() {
	case reflect.String:
		field.SetString(value)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		var intVal int64
		_, err := parseNumber(value, &intVal)
		if err != nil {
			return err
		}
		field.SetInt(intVal)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		var intVal int64
		_, err := parseNumber(value, &intVal)
		if err != nil {
			return err
		}
		field.SetUint(uint64(intVal))
	case reflect.Float32, reflect.Float64:
		var floatVal float64
		_, err := parseNumber(value, &floatVal)
		if err != nil {
			return err
		}
		field.SetFloat(floatVal)
	case reflect.Bool:
		boolVal := value == "true" || value == "1" || value == "yes"
		field.SetBool(boolVal)
	default:
		// 对于复杂类型，暂不处理
		return nil
	}
	return nil
}

// parseNumber 解析数字字符串
func parseNumber(s string, target any) (bool, error) {
	switch v := target.(type) {
	case *int64:
		var n int64
		for _, c := range s {
			if c >= '0' && c <= '9' {
				n = n*10 + int64(c-'0')
			} else if c == '-' && n == 0 {
				continue
			} else {
				break
			}
		}
		if len(s) > 0 && s[0] == '-' {
			n = -n
		}
		*v = n
		return true, nil
	case *float64:
		var n float64
		var decimal float64 = 0
		var decimalPlace float64 = 1
		inDecimal := false
		negative := false

		for i, c := range s {
			if c == '-' && i == 0 {
				negative = true
			} else if c == '.' {
				inDecimal = true
			} else if c >= '0' && c <= '9' {
				if inDecimal {
					decimalPlace *= 10
					decimal += float64(c-'0') / decimalPlace
				} else {
					n = n*10 + float64(c-'0')
				}
			}
		}
		n += decimal
		if negative {
			n = -n
		}
		*v = n
		return true, nil
	}
	return false, nil
}

// 错误定义
var (
	ErrInvalidTarget = &SchemaError{Message: "target must be a non-nil pointer to struct"}
)

// SchemaError Schema 相关错误
type SchemaError struct {
	Message string
}

func (e *SchemaError) Error() string {
	return e.Message
}
