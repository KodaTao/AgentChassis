package telegram

import (
	"fmt"
	"sync"
	"time"
)

// SessionEntry 会话条目
type SessionEntry struct {
	SessionID string    // Agent session ID
	CreatedAt time.Time // 创建时间
}

// SessionStore 存储 message_id 到 session_id 的映射
// 用于实现基于 Reply 机制的多会话管理
type SessionStore struct {
	mu sync.RWMutex
	// chat_id -> message_id -> SessionEntry
	sessions map[int64]map[int]*SessionEntry
	ttl      time.Duration
}

// NewSessionStore 创建 SessionStore
func NewSessionStore(ttl time.Duration) *SessionStore {
	store := &SessionStore{
		sessions: make(map[int64]map[int]*SessionEntry),
		ttl:      ttl,
	}
	// 启动定期清理
	go store.cleanupLoop()
	return store
}

// Set 记录映射关系
func (s *SessionStore) Set(chatID int64, msgID int, sessionID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.sessions[chatID]; !ok {
		s.sessions[chatID] = make(map[int]*SessionEntry)
	}

	s.sessions[chatID][msgID] = &SessionEntry{
		SessionID: sessionID,
		CreatedAt: time.Now(),
	}
}

// Get 查找 session_id
// 如果找不到返回空字符串
func (s *SessionStore) Get(chatID int64, msgID int) string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if chatSessions, ok := s.sessions[chatID]; ok {
		if entry, ok := chatSessions[msgID]; ok {
			// 检查是否过期
			if time.Since(entry.CreatedAt) <= s.ttl {
				return entry.SessionID
			}
		}
	}
	return ""
}

// GenerateSessionID 生成新的 session ID
func GenerateSessionID(chatID int64) string {
	return fmt.Sprintf("tg_%d_%d", chatID, time.Now().UnixNano())
}

// cleanupLoop 定期清理过期的映射
func (s *SessionStore) cleanupLoop() {
	ticker := time.NewTicker(time.Hour) // 每小时清理一次
	defer ticker.Stop()

	for range ticker.C {
		s.cleanup()
	}
}

// cleanup 清理过期映射
func (s *SessionStore) cleanup() {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	for chatID, chatSessions := range s.sessions {
		for msgID, entry := range chatSessions {
			if now.Sub(entry.CreatedAt) > s.ttl {
				delete(chatSessions, msgID)
			}
		}
		// 如果该 chat 的所有映射都被清理，删除整个 chat
		if len(chatSessions) == 0 {
			delete(s.sessions, chatID)
		}
	}
}

// Stats 返回统计信息（用于调试）
func (s *SessionStore) Stats() (chatCount, entryCount int) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	chatCount = len(s.sessions)
	for _, chatSessions := range s.sessions {
		entryCount += len(chatSessions)
	}
	return
}
