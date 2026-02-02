// Package protocol 提供 XML + TOON 协议解析和编码
package protocol

import (
	"encoding/xml"
	"errors"
	"regexp"
	"strings"
)

// CallRequest AI 调用请求（解析自 AI 输出）
type CallRequest struct {
	Name   string            `xml:"name,attr"`
	Params map[string]string // 从 <p> 标签解析
	Data   string            // 从 <data type="toon"> 解析的 TOON 内容
}

// RawCall XML 原始调用结构
type RawCall struct {
	XMLName xml.Name  `xml:"call"`
	Name    string    `xml:"name,attr"`
	Items   []RawItem `xml:",any"`
}

// RawItem XML 内部元素
type RawItem struct {
	XMLName xml.Name
	Type    string `xml:"type,attr"`
	Content string `xml:",chardata"`
}

// Parser 协议解析器
type Parser struct{}

// NewParser 创建解析器实例
func NewParser() *Parser {
	return &Parser{}
}

// ParseCall 从 AI 输出中解析 Function 调用
// 支持格式：
// <call name="function_name">
//   <p>key: value</p>
//   <data type="toon">TOON_CONTENT</data>
// </call>
func (p *Parser) ParseCall(content string) (*CallRequest, error) {
	// 提取 <call>...</call> 内容
	callContent, err := extractCallXML(content)
	if err != nil {
		return nil, err
	}

	// 解析 XML
	var rawCall RawCall
	if err := xml.Unmarshal([]byte(callContent), &rawCall); err != nil {
		return nil, &ParseError{
			Message: "failed to parse call XML",
			Cause:   err,
		}
	}

	// 构建 CallRequest
	req := &CallRequest{
		Name:   rawCall.Name,
		Params: make(map[string]string),
	}

	// 处理内部元素
	for _, item := range rawCall.Items {
		switch item.XMLName.Local {
		case "p":
			// 解析 key: value 格式的参数
			key, value := parseKeyValue(strings.TrimSpace(item.Content))
			if key != "" {
				req.Params[key] = value
			}
		case "data":
			if item.Type == "toon" {
				req.Data = strings.TrimSpace(item.Content)
			}
		}
	}

	if req.Name == "" {
		return nil, &ParseError{Message: "call name is required"}
	}

	return req, nil
}

// ParseCalls 从 AI 输出中解析所有 Function 调用
// AI 可能在一次输出中调用多个 Function
func (p *Parser) ParseCalls(content string) ([]*CallRequest, error) {
	var calls []*CallRequest

	// 使用正则找到所有 <call>...</call>
	re := regexp.MustCompile(`(?s)<call[^>]*>.*?</call>`)
	matches := re.FindAllString(content, -1)

	for _, match := range matches {
		call, err := p.ParseCall(match)
		if err != nil {
			// 记录错误但继续解析其他调用
			continue
		}
		calls = append(calls, call)
	}

	return calls, nil
}

// HasCall 检查内容中是否包含 Function 调用
func (p *Parser) HasCall(content string) bool {
	return strings.Contains(content, "<call") && strings.Contains(content, "</call>")
}

// ExtractTextBeforeCall 提取调用之前的文本内容
// AI 可能在调用前有一些说明文字
func (p *Parser) ExtractTextBeforeCall(content string) string {
	idx := strings.Index(content, "<call")
	if idx == -1 {
		return content
	}
	return strings.TrimSpace(content[:idx])
}

// ExtractTextAfterCall 提取调用之后的文本内容
func (p *Parser) ExtractTextAfterCall(content string) string {
	idx := strings.LastIndex(content, "</call>")
	if idx == -1 {
		return ""
	}
	return strings.TrimSpace(content[idx+7:])
}

// extractCallXML 从内容中提取 <call>...</call> XML
func extractCallXML(content string) (string, error) {
	// 查找 <call 开始位置
	startIdx := strings.Index(content, "<call")
	if startIdx == -1 {
		return "", &ParseError{Message: "no <call> tag found"}
	}

	// 查找 </call> 结束位置
	endIdx := strings.Index(content[startIdx:], "</call>")
	if endIdx == -1 {
		return "", &ParseError{Message: "no </call> closing tag found"}
	}

	return content[startIdx : startIdx+endIdx+7], nil
}

// parseKeyValue 解析 "key: value" 格式
func parseKeyValue(s string) (string, string) {
	idx := strings.Index(s, ":")
	if idx == -1 {
		return "", ""
	}
	key := strings.TrimSpace(s[:idx])
	value := strings.TrimSpace(s[idx+1:])
	return key, value
}

// ParseError 解析错误
type ParseError struct {
	Message string
	Cause   error
}

func (e *ParseError) Error() string {
	if e.Cause != nil {
		return e.Message + ": " + e.Cause.Error()
	}
	return e.Message
}

func (e *ParseError) Unwrap() error {
	return e.Cause
}

// 预定义错误
var (
	ErrNoCallFound = errors.New("no function call found in content")
)
