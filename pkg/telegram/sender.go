package telegram

import (
	"fmt"
	"log/slog"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// Sender 消息发送器
// 封装 Telegram Bot API 的消息发送功能
type Sender struct {
	bot    *tgbotapi.BotAPI
	logger *slog.Logger
}

// NewSender 创建消息发送器
func NewSender(bot *tgbotapi.BotAPI, logger *slog.Logger) *Sender {
	return &Sender{
		bot:    bot,
		logger: logger,
	}
}

// SendReply 发送回复消息（reply 指定的消息）
// 返回发送的消息 ID
func (s *Sender) SendReply(chatID int64, replyToMsgID int, text string) (int, error) {
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ReplyToMessageID = replyToMsgID
	msg.ParseMode = tgbotapi.ModeMarkdownV2

	sent, err := s.bot.Send(msg)
	if err != nil {
		// 如果 Markdown 解析失败，尝试以纯文本发送
		s.logger.Warn("failed to send markdown message, retrying as plain text",
			"chat_id", chatID,
			"error", err,
		)
		msg.ParseMode = ""
		sent, err = s.bot.Send(msg)
		if err != nil {
			s.logger.Error("failed to send message",
				"chat_id", chatID,
				"error", err,
			)
			return 0, fmt.Errorf("failed to send message: %w", err)
		}
	}

	s.logger.Debug("message sent",
		"chat_id", chatID,
		"message_id", sent.MessageID,
		"reply_to", replyToMsgID,
	)

	return sent.MessageID, nil
}

// SendMessage 发送消息（不 reply）
// 用于任务触发时的通知
func (s *Sender) SendMessage(chatID int64, text string) (int, error) {
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = tgbotapi.ModeMarkdownV2

	sent, err := s.bot.Send(msg)
	if err != nil {
		// 如果 Markdown 解析失败，尝试以纯文本发送
		s.logger.Warn("failed to send markdown message, retrying as plain text",
			"chat_id", chatID,
			"error", err,
		)
		msg.ParseMode = ""
		sent, err = s.bot.Send(msg)
		if err != nil {
			s.logger.Error("failed to send notification",
				"chat_id", chatID,
				"error", err,
			)
			return 0, fmt.Errorf("failed to send notification: %w", err)
		}
	}

	s.logger.Debug("notification sent",
		"chat_id", chatID,
		"message_id", sent.MessageID,
	)

	return sent.MessageID, nil
}
