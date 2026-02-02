// Package protocol 提供 XML + TOON 协议解析和编码
package protocol

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"

	"github.com/KodaTao/AgentChassis/pkg/function"
	"github.com/KodaTao/AgentChassis/pkg/prompt/templates"
)

// PromptGenerator System Prompt 生成器
// Deprecated: 请使用 pkg/prompt 包中的 Generator
type PromptGenerator struct {
	template *template.Template
}

// NewPromptGenerator 创建 Prompt 生成器
// Deprecated: 请使用 prompt.NewGenerator()
func NewPromptGenerator() *PromptGenerator {
	tmpl := template.Must(template.New("system_prompt").Parse(templates.SystemPrompt))
	return &PromptGenerator{
		template: tmpl,
	}
}

// Generate 根据已注册的 Function 生成 System Prompt
func (g *PromptGenerator) Generate(functions []function.FunctionInfo) (string, error) {
	var buf bytes.Buffer

	data := promptData{
		Functions:    functions,
		HasFunctions: len(functions) > 0,
	}

	if err := g.template.Execute(&buf, data); err != nil {
		return "", err
	}

	return buf.String(), nil
}

// GenerateMinimal 生成简化版 System Prompt（节省 Token）
func (g *PromptGenerator) GenerateMinimal(functions []function.FunctionInfo) string {
	var buf bytes.Buffer

	buf.WriteString("You are an AI assistant with access to the following functions:\n\n")

	for _, fn := range functions {
		buf.WriteString(fmt.Sprintf("## %s\n%s\n", fn.Name, fn.Description))
		if len(fn.Parameters) > 0 {
			buf.WriteString("Parameters:\n")
			for _, p := range fn.Parameters {
				required := ""
				if p.Required {
					required = " (required)"
				}
				buf.WriteString(fmt.Sprintf("- %s: %s%s\n", p.Name, p.Type, required))
			}
		}
		buf.WriteString("\n")
	}

	buf.WriteString("To call a function, use XML format:\n")
	buf.WriteString("<call name=\"function_name\"><p>param: value</p></call>\n")

	return buf.String()
}

// promptData 模板数据
type promptData struct {
	Functions    []function.FunctionInfo
	HasFunctions bool
}

// FormatFunctionList 格式化 Function 列表为可读文本
func FormatFunctionList(functions []function.FunctionInfo) string {
	var buf bytes.Buffer

	for i, fn := range functions {
		if i > 0 {
			buf.WriteString("\n")
		}
		buf.WriteString(fmt.Sprintf("### %s\n", fn.Name))
		buf.WriteString(fmt.Sprintf("%s\n", fn.Description))

		if len(fn.Parameters) > 0 {
			buf.WriteString("\n**Parameters:**\n")
			for _, p := range fn.Parameters {
				required := ""
				if p.Required {
					required = " *(required)*"
				}
				defaultVal := ""
				if p.Default != "" {
					defaultVal = fmt.Sprintf(" (default: %s)", p.Default)
				}
				desc := ""
				if p.Description != "" {
					desc = fmt.Sprintf(" - %s", p.Description)
				}
				buf.WriteString(fmt.Sprintf("- %s (%s)%s%s%s\n", p.Name, p.Type, required, defaultVal, desc))
			}
		}
	}

	return buf.String()
}

// BuildFunctionSignature 构建函数签名字符串
func BuildFunctionSignature(fn function.FunctionInfo) string {
	var params []string
	for _, p := range fn.Parameters {
		param := p.Name + ": " + p.Type
		if p.Required {
			param += " (required)"
		}
		params = append(params, param)
	}
	return fmt.Sprintf("%s(%s)", fn.Name, strings.Join(params, ", "))
}
