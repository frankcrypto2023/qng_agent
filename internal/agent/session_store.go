package agent

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// SessionStore 会话存储接口
type SessionStore interface {
	GetSession(ctx context.Context, sessionID string) (*Session, error)
	SaveSession(ctx context.Context, session *Session) error
	DeleteSession(ctx context.Context, sessionID string) error
	Close() error
}

// SQLiteSessionStore SQLite会话存储实现
type SQLiteSessionStore struct {
	db *sql.DB
}

// NewSQLiteSessionStore 创建SQLite会话存储
func NewSQLiteSessionStore(dbPath string) (*SQLiteSessionStore, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// 创建会话表
	createTableSQL := `
	CREATE TABLE IF NOT EXISTS sessions (
		id TEXT PRIMARY KEY,
		messages TEXT,
		current_state TEXT,
		created_at DATETIME,
		updated_at DATETIME
	)`

	_, err = db.Exec(createTableSQL)
	if err != nil {
		return nil, fmt.Errorf("failed to create sessions table: %w", err)
	}

	return &SQLiteSessionStore{db: db}, nil
}

// GetSession 获取会话
func (s *SQLiteSessionStore) GetSession(ctx context.Context, sessionID string) (*Session, error) {
	query := `SELECT messages, current_state, created_at FROM sessions WHERE id = ?`
	
	var messagesJSON, currentState string
	var createdAt time.Time
	
	err := s.db.QueryRowContext(ctx, query, sessionID).Scan(&messagesJSON, &currentState, &createdAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // 会话不存在
		}
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	// 解析消息
	var messages []Message
	if messagesJSON != "" {
		if err := json.Unmarshal([]byte(messagesJSON), &messages); err != nil {
			log.Printf("⚠️  解析会话消息失败: %v", err)
			messages = make([]Message, 0)
		}
	}

	return &Session{
		ID:           sessionID,
		Messages:     messages,
		CurrentState: currentState,
		CreatedAt:    createdAt,
	}, nil
}

// SaveSession 保存会话
func (s *SQLiteSessionStore) SaveSession(ctx context.Context, session *Session) error {
	// 序列化消息
	messagesJSON, err := json.Marshal(session.Messages)
	if err != nil {
		return fmt.Errorf("failed to marshal messages: %w", err)
	}

	// 使用UPSERT语法
	query := `
	INSERT INTO sessions (id, messages, current_state, created_at, updated_at)
	VALUES (?, ?, ?, ?, ?)
	ON CONFLICT(id) DO UPDATE SET
		messages = excluded.messages,
		current_state = excluded.current_state,
		updated_at = excluded.updated_at`

	now := time.Now()
	_, err = s.db.ExecContext(ctx, query, 
		session.ID, 
		string(messagesJSON), 
		session.CurrentState, 
		session.CreatedAt, 
		now)

	if err != nil {
		return fmt.Errorf("failed to save session: %w", err)
	}

	return nil
}

// DeleteSession 删除会话
func (s *SQLiteSessionStore) DeleteSession(ctx context.Context, sessionID string) error {
	query := `DELETE FROM sessions WHERE id = ?`
	_, err := s.db.ExecContext(ctx, query, sessionID)
	if err != nil {
		return fmt.Errorf("failed to delete session: %w", err)
	}
	return nil
}

// Close 关闭数据库连接
func (s *SQLiteSessionStore) Close() error {
	return s.db.Close()
}

// MemorySessionStore 内存会话存储（用于测试）
type MemorySessionStore struct {
	sessions map[string]*Session
}

// NewMemorySessionStore 创建内存会话存储
func NewMemorySessionStore() *MemorySessionStore {
	return &MemorySessionStore{
		sessions: make(map[string]*Session),
	}
}

// GetSession 获取会话
func (m *MemorySessionStore) GetSession(ctx context.Context, sessionID string) (*Session, error) {
	if session, exists := m.sessions[sessionID]; exists {
		return session, nil
	}
	return nil, nil
}

// SaveSession 保存会话
func (m *MemorySessionStore) SaveSession(ctx context.Context, session *Session) error {
	m.sessions[session.ID] = session
	return nil
}

// DeleteSession 删除会话
func (m *MemorySessionStore) DeleteSession(ctx context.Context, sessionID string) error {
	delete(m.sessions, sessionID)
	return nil
}

// Close 关闭存储
func (m *MemorySessionStore) Close() error {
	return nil
} 