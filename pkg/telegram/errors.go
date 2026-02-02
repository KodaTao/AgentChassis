package telegram

import "errors"

var (
	// ErrTokenRequired Token 未配置
	ErrTokenRequired = errors.New("telegram bot token is required")

	// ErrBotNotInitialized Bot 未初始化
	ErrBotNotInitialized = errors.New("telegram bot is not initialized")

	// ErrSessionNotFound Session 未找到
	ErrSessionNotFound = errors.New("session not found")
)
