// Package chassis 提供 AgentChassis 核心框架
package chassis

import (
	"context"
	"sync"
	"time"

	"github.com/KodaTao/AgentChassis/pkg/llm"
)

// Session 对话会话
type Session struct {
	ID        string        `json:"id"`
	Messages  []llm.Message `json:"messages"`
	CreatedAt time.Time     `json:"created_at"`
	UpdatedAt time.Time     `json:"updated_at"`
}

// AddMessage 添加消息到会话
func (s *Session) AddMessage(role llm.Role, content string) {
	s.Messages = append(s.Messages, llm.Message{
		Role:    role,
		Content: content,
	})
	s.UpdatedAt = time.Now()
}

// GetMessages 获取所有消息
func (s *Session) GetMessages() []llm.Message {
	return s.Messages
}

// Truncate 截断消息历史，保留最近的 n 条
// 始终保留第一条系统消息（如果有）
func (s *Session) Truncate(maxMessages int) {
	if len(s.Messages) <= maxMessages {
		return
	}

	// 检查第一条是否是系统消息
	if len(s.Messages) > 0 && s.Messages[0].Role == llm.RoleSystem {
		// 保留系统消息 + 最近的消息
		systemMsg := s.Messages[0]
		recentMessages := s.Messages[len(s.Messages)-(maxMessages-1):]
		s.Messages = append([]llm.Message{systemMsg}, recentMessages...)
	} else {
		// 只保留最近的消息
		s.Messages = s.Messages[len(s.Messages)-maxMessages:]
	}
}

// Clear 清空消息历史（保留系统消息）
func (s *Session) Clear() {
	if len(s.Messages) > 0 && s.Messages[0].Role == llm.RoleSystem {
		s.Messages = s.Messages[:1]
	} else {
		s.Messages = nil
	}
	s.UpdatedAt = time.Now()
}

// SessionManager 会话管理器
// 管理多个会话，支持并发访问
type SessionManager struct {
	mu       sync.RWMutex
	sessions map[string]*Session
	config   *SessionConfig
}

// NewSessionManager 创建会话管理器
func NewSessionManager(config *SessionConfig) *SessionManager {
	if config == nil {
		config = DefaultSessionConfig()
	}
	return &SessionManager{
		sessions: make(map[string]*Session),
		config:   config,
	}
}

// Get 获取会话，如果不存在则返回 nil
func (m *SessionManager) Get(id string) *Session {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.sessions[id]
}

// GetOrCreate 获取或创建会话
func (m *SessionManager) GetOrCreate(id string) *Session {
	m.mu.Lock()
	defer m.mu.Unlock()

	if session, ok := m.sessions[id]; ok {
		return session
	}

	session := &Session{
		ID:        id,
		Messages:  make([]llm.Message, 0),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	m.sessions[id] = session
	return session
}

// Delete 删除会话
func (m *SessionManager) Delete(id string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.sessions[id]; ok {
		delete(m.sessions, id)
		return true
	}
	return false
}

// List 列出所有会话 ID
func (m *SessionManager) List() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ids := make([]string, 0, len(m.sessions))
	for id := range m.sessions {
		ids = append(ids, id)
	}
	return ids
}

// CleanExpired 清理过期会话
func (m *SessionManager) CleanExpired() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	count := 0
	expireTime := time.Now().Add(-m.config.TTL)

	for id, session := range m.sessions {
		if session.UpdatedAt.Before(expireTime) {
			delete(m.sessions, id)
			count++
		}
	}
	return count
}

// ContextKey 上下文键类型
type ContextKey string

const (
	// SessionIDKey 会话 ID 上下文键
	SessionIDKey ContextKey = "session_id"
	// TraceIDKey 追踪 ID 上下文键
	TraceIDKey ContextKey = "trace_id"
)

// WithSessionID 将会话 ID 添加到 context
func WithSessionID(ctx context.Context, sessionID string) context.Context {
	return context.WithValue(ctx, SessionIDKey, sessionID)
}

// GetSessionID 从 context 获取会话 ID
func GetSessionID(ctx context.Context) string {
	if id, ok := ctx.Value(SessionIDKey).(string); ok {
		return id
	}
	return ""
}

// WithTraceID 将追踪 ID 添加到 context
func WithTraceID(ctx context.Context, traceID string) context.Context {
	return context.WithValue(ctx, TraceIDKey, traceID)
}

// GetTraceID 从 context 获取追踪 ID
func GetTraceID(ctx context.Context) string {
	if id, ok := ctx.Value(TraceIDKey).(string); ok {
		return id
	}
	return ""
}
