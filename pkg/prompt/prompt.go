// Package prompt 提供提示词生成和管理功能
package prompt

import (
	"bytes"
	"text/template"

	"github.com/KodaTao/AgentChassis/pkg/function"
	"github.com/KodaTao/AgentChassis/pkg/prompt/templates"
)

// Generator 提示词生成器
type Generator struct {
	systemTemplate *template.Template
	minimalTemplate *template.Template
}

// NewGenerator 创建提示词生成器
func NewGenerator() *Generator {
	return &Generator{
		systemTemplate:  template.Must(template.New("system").Parse(templates.SystemPrompt)),
		minimalTemplate: template.Must(template.New("minimal").Parse(templates.SystemPromptMinimal)),
	}
}

// TemplateData 模板数据
type TemplateData struct {
	Functions    []function.FunctionInfo
	HasFunctions bool
}

// GenerateSystemPrompt 生成完整的系统提示词
func (g *Generator) GenerateSystemPrompt(functions []function.FunctionInfo) (string, error) {
	var buf bytes.Buffer
	data := TemplateData{
		Functions:    functions,
		HasFunctions: len(functions) > 0,
	}
	if err := g.systemTemplate.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// GenerateMinimalPrompt 生成精简版系统提示词
func (g *Generator) GenerateMinimalPrompt(functions []function.FunctionInfo) (string, error) {
	var buf bytes.Buffer
	data := TemplateData{
		Functions:    functions,
		HasFunctions: len(functions) > 0,
	}
	if err := g.minimalTemplate.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// GenerateWithCustomTemplate 使用自定义模板生成提示词
func (g *Generator) GenerateWithCustomTemplate(tmplStr string, data any) (string, error) {
	tmpl, err := template.New("custom").Parse(tmplStr)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// DefaultGenerator 默认生成器实例
var DefaultGenerator = NewGenerator()

// GenerateSystemPrompt 使用默认生成器生成系统提示词
func GenerateSystemPrompt(functions []function.FunctionInfo) (string, error) {
	return DefaultGenerator.GenerateSystemPrompt(functions)
}

// GenerateMinimalPrompt 使用默认生成器生成精简版系统提示词
func GenerateMinimalPrompt(functions []function.FunctionInfo) (string, error) {
	return DefaultGenerator.GenerateMinimalPrompt(functions)
}
