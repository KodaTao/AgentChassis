package telegram

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"github.com/KodaTao/AgentChassis/pkg/types"
)

// Bot Telegram Bot 封装
type Bot struct {
	api          *tgbotapi.BotAPI
	config       Config
	sessionStore *SessionStore
	sender       *Sender
	agent        types.Agent
	logger       *slog.Logger

	ctx    context.Context
	cancel context.CancelFunc
}

// NewBot 创建 Telegram Bot
func NewBot(config Config, agent types.Agent, logger *slog.Logger) (*Bot, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}

	api, err := tgbotapi.NewBotAPI(config.Token)
	if err != nil {
		return nil, fmt.Errorf("failed to create telegram bot: %w", err)
	}

	me, err := api.GetMe()
	if err != nil {
		return nil, fmt.Errorf("failed to get bot info: %w", err)
	}

	logger.Info("telegram bot created",
		"username", me.UserName)

	ctx, cancel := context.WithCancel(context.Background())

	bot := &Bot{
		api:          api,
		config:       config,
		sessionStore: NewSessionStore(config.SessionTTL),
		agent:        agent,
		logger:       logger,
		ctx:          ctx,
		cancel:       cancel,
	}
	bot.sender = NewSender(api, logger)

	logger.Info("telegram bot created",
		"username", api.Self.UserName,
	)

	return bot, nil
}

// Start 启动 Bot，开始接收消息
func (b *Bot) Start() {
	b.logger.Info("starting telegram bot")

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := b.api.GetUpdatesChan(u)

	go func() {
		for {
			select {
			case <-b.ctx.Done():
				b.logger.Info("telegram bot stopped")
				return
			case update := <-updates:
				if update.Message != nil {
					if update.Message.Chat.IsGroup() || update.Message.Chat.IsChannel() {
						// 群聊必须@才生效
						if !strings.Contains(update.Message.Text, "@"+b.api.Self.UserName+" ") {
							continue
						}
					}
					go b.handleMessage(update.Message)
				}
			}
		}
	}()

	b.logger.Info("telegram bot started")
}

// Stop 停止 Bot
func (b *Bot) Stop() {
	b.logger.Info("stopping telegram bot")
	b.cancel()
	b.api.StopReceivingUpdates()
}

// handleMessage 处理收到的消息
func (b *Bot) handleMessage(msg *tgbotapi.Message) {
	// 忽略非文本消息
	if msg.Text == "" {
		return
	}

	chatID := msg.Chat.ID
	userMsgID := msg.MessageID

	b.logger.Info("received message",
		"chat_id", chatID,
		"message_id", userMsgID,
		"from", msg.From.UserName,
		"text", truncateText(msg.Text, 50),
	)

	// 确定 session ID
	var sessionID string
	if msg.ReplyToMessage != nil && msg.ReplyToMessage.From.ID == b.api.Self.ID {
		// 用户 reply 了 Bot 的消息，查找对应的 session
		replyToMsgID := msg.ReplyToMessage.MessageID
		sessionID = b.sessionStore.Get(chatID, replyToMsgID)
		if sessionID == "" {
			b.logger.Debug("session not found for reply, creating new session",
				"chat_id", chatID,
				"reply_to", replyToMsgID,
			)
			sessionID = GenerateSessionID(chatID)
		} else {
			b.logger.Debug("found session from reply",
				"chat_id", chatID,
				"session_id", sessionID,
				"reply_to", replyToMsgID,
			)
		}
	} else {
		// 新消息，创建新 session
		sessionID = GenerateSessionID(chatID)
		b.logger.Debug("new message, creating new session",
			"chat_id", chatID,
			"session_id", sessionID,
		)
	}

	// 构建渠道上下文
	channel := &types.ChannelContext{
		Type:   "telegram",
		ChatID: strconv.FormatInt(chatID, 10),
	}

	channelJSON, _ := json.Marshal(channel)

	// 调用 Agent 处理消息
	// 添加渠道信息提示到消息中（用于任务创建时 AI 知道渠道）
	// 这样 AI 在创建任务时会把渠道信息包含在 channel 参数中
	req := types.ChatRequest{
		SessionID: sessionID,
		Message:   fmt.Sprintf("【当前渠道：%s】\n%s", string(channelJSON), msg.Text),
		Channel:   channel,
	}

	resp, err := b.agent.Chat(b.ctx, req)
	if err != nil {
		b.logger.Error("agent chat failed",
			"chat_id", chatID,
			"session_id", sessionID,
			"error", err,
		)
		// 发送错误提示给用户
		_, _ = b.sender.SendReply(chatID, userMsgID, "抱歉，处理消息时出现了错误，请稍后重试。")
		return
	}

	// 发送回复（reply 用户的消息）
	botMsgID, err := b.sender.SendReply(chatID, userMsgID, resp.Reply)
	if err != nil {
		b.logger.Error("failed to send reply",
			"chat_id", chatID,
			"error", err,
		)
		return
	}

	// 记录映射：bot 消息 ID -> session ID
	b.sessionStore.Set(chatID, botMsgID, resp.SessionID)

	b.logger.Info("message handled",
		"chat_id", chatID,
		"session_id", resp.SessionID,
		"user_msg_id", userMsgID,
		"bot_msg_id", botMsgID,
		"function_calls", len(resp.FunctionCalls),
	)

	// 同时记录用户消息 ID 的映射（方便调试和某些场景）
	b.sessionStore.Set(chatID, userMsgID, resp.SessionID)
}

// GetSender 获取消息发送器（供外部使用，如任务触发通知）
func (b *Bot) GetSender() *Sender {
	return b.sender
}

// SendNotification 发送通知消息到指定 chat
// 用于任务触发时的通知
func (b *Bot) SendNotification(chatID int64, text string) error {
	_, err := b.sender.SendMessage(chatID, text)
	return err
}

// truncateText 截断文本（用于日志）
func truncateText(text string, maxLen int) string {
	runes := []rune(text)
	if len(runes) <= maxLen {
		return text
	}
	return string(runes[:maxLen]) + "..."
}
