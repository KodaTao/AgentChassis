// Package builtin 提供内置的 Function 实现
package builtin

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"github.com/KodaTao/AgentChassis/pkg/function"
	"github.com/KodaTao/AgentChassis/pkg/observability"
)

// NotificationChannel 通知渠道类型
type NotificationChannel string

const (
	ChannelConsole NotificationChannel = "console" // 控制台输出（默认）
	ChannelEmail   NotificationChannel = "email"   // 邮件（待实现）
	ChannelSMS     NotificationChannel = "sms"     // 短信（待实现）
	ChannelWeChat  NotificationChannel = "wechat"  // 微信（待实现）
)

// SendMessageParams 发送消息的参数
type SendMessageParams struct {
	To      string `json:"to" desc:"接收者（人名、邮箱、手机号等，根据渠道而定）" required:"true"`
	Message string `json:"message" desc:"消息内容" required:"true"`
	Channel string `json:"channel" desc:"通知渠道：console（控制台，默认）、email、sms、wechat" default:"console"`
}

// SendMessageFunction 发送消息的函数
// 这是一个通用的外部通知函数，用于向他人发送消息
// 目前支持控制台输出，未来可扩展为邮件、短信、微信等渠道
type SendMessageFunction struct {
	// 可以在这里注入不同渠道的发送器
	// emailSender EmailSender
	// smsSender   SMSSender
	// wechatSender WeChatSender
}

func (f *SendMessageFunction) Name() string {
	return "send_message"
}

func (f *SendMessageFunction) Description() string {
	return "向指定的人发送消息通知。可以直接调用，也可以配合延时任务在指定时间发送。目前支持控制台输出，未来可扩展邮件、短信、微信等渠道。"
}

func (f *SendMessageFunction) ParamsType() reflect.Type {
	return reflect.TypeOf(SendMessageParams{})
}

func (f *SendMessageFunction) Execute(ctx context.Context, params any) (function.Result, error) {
	p := params.(SendMessageParams)

	// 确定通知渠道
	channel := NotificationChannel(p.Channel)
	if channel == "" {
		channel = ChannelConsole
	}

	now := time.Now()
	timestamp := now.Format("2006-01-02 15:04:05")

	// 根据渠道发送消息
	var deliveryStatus string
	var deliveryError error

	switch channel {
	case ChannelConsole:
		// 控制台输出
		fmt.Printf("\n")
		fmt.Printf("╔══════════════════════════════════════════════════════════════╗\n")
		fmt.Printf("║  📬 新消息通知                                                ║\n")
		fmt.Printf("╠══════════════════════════════════════════════════════════════╣\n")
		fmt.Printf("║  收件人: %-52s ║\n", p.To)
		fmt.Printf("║  时间:   %-52s ║\n", timestamp)
		fmt.Printf("╠══════════════════════════════════════════════════════════════╣\n")
		fmt.Printf("║  内容: %-54s ║\n", truncateString(p.Message, 54))
		if len(p.Message) > 54 {
			// 长消息换行显示
			remaining := p.Message[54:]
			for len(remaining) > 0 {
				lineLen := 54
				if len(remaining) < lineLen {
					lineLen = len(remaining)
				}
				fmt.Printf("║         %-54s ║\n", remaining[:lineLen])
				remaining = remaining[lineLen:]
			}
		}
		fmt.Printf("╚══════════════════════════════════════════════════════════════╝\n")
		fmt.Printf("\n")

		deliveryStatus = "delivered"

		// 同时记录到日志
		observability.Info("Message sent",
			"channel", "console",
			"to", p.To,
			"message", p.Message,
			"time", timestamp,
		)

	case ChannelEmail:
		// TODO: 实现邮件发送
		deliveryStatus = "unsupported"
		deliveryError = fmt.Errorf("email channel is not implemented yet")

	case ChannelSMS:
		// TODO: 实现短信发送
		deliveryStatus = "unsupported"
		deliveryError = fmt.Errorf("sms channel is not implemented yet")

	case ChannelWeChat:
		// TODO: 实现微信发送
		deliveryStatus = "unsupported"
		deliveryError = fmt.Errorf("wechat channel is not implemented yet")

	default:
		deliveryStatus = "unsupported"
		deliveryError = fmt.Errorf("unknown channel: %s", channel)
	}

	if deliveryError != nil {
		return function.Result{
			Message: fmt.Sprintf("消息发送失败: %s", deliveryError.Error()),
			Data: map[string]any{
				"to":      p.To,
				"message": p.Message,
				"channel": string(channel),
				"status":  deliveryStatus,
				"error":   deliveryError.Error(),
			},
		}, deliveryError
	}

	return function.Result{
		Message: fmt.Sprintf("已向 %s 发送消息: %s", p.To, truncateString(p.Message, 30)),
		Data: map[string]any{
			"to":         p.To,
			"message":    p.Message,
			"channel":    string(channel),
			"status":     deliveryStatus,
			"sent_at":    now.Format(time.RFC3339),
		},
	}, nil
}

// truncateString 截断字符串
func truncateString(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen-3]) + "..."
}

// NewSendMessageFunction 创建 SendMessageFunction
func NewSendMessageFunction() *SendMessageFunction {
	return &SendMessageFunction{}
}
