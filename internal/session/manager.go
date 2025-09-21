package session

import (
	"fmt"
	"qng-agent/internal/storage"
	"qng-agent/internal/types"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Manager handles chat session management with user isolation
type Manager struct {
	storage  storage.Storage
	// sessions map[userID][sessionID]*SessionContext for user isolation
	sessions map[string]map[string]*SessionContext
	mutex    sync.RWMutex
}

// SessionContext represents an active session context
type SessionContext struct {
	Session   *types.ChatSession
	LastUsed  time.Time
	mutex     sync.RWMutex
}

// NewManager creates a new session manager
func NewManager(storage storage.Storage) *Manager {
	return &Manager{
		storage:  storage,
		sessions: make(map[string]map[string]*SessionContext),
	}
}

// CreateSession creates a new chat session for a user
func (m *Manager) CreateSession(userID, title string) (*types.ChatSession, error) {
	if title == "" {
		title = "New Conversation"
	}

	session := &types.ChatSession{
		ID:        uuid.New().String(),
		Title:     title,
		Messages:  []types.ChatMessage{},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := m.storage.CreateSession(userID, session); err != nil {
		return nil, fmt.Errorf("failed to create session in storage: %w", err)
	}

	// Add to active sessions
	m.mutex.Lock()
	if m.sessions[userID] == nil {
		m.sessions[userID] = make(map[string]*SessionContext)
	}
	m.sessions[userID][session.ID] = &SessionContext{
		Session:  session,
		LastUsed: time.Now(),
	}
	m.mutex.Unlock()

	return session, nil
}

// GetSession retrieves a session by userID and sessionID
func (m *Manager) GetSession(userID, sessionID string) (*types.ChatSession, error) {
	// Check active sessions first
	m.mutex.RLock()
	if userSessions, exists := m.sessions[userID]; exists {
		if ctx, exists := userSessions[sessionID]; exists {
			ctx.mutex.RLock()
			session := ctx.Session
			ctx.mutex.RUnlock()
			ctx.LastUsed = time.Now()
			m.mutex.RUnlock()
			return session, nil
		}
	}
	m.mutex.RUnlock()

	// Load from storage
	session, err := m.storage.GetSession(userID, sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get session from storage: %w", err)
	}

	// Add to active sessions
	m.mutex.Lock()
	if m.sessions[userID] == nil {
		m.sessions[userID] = make(map[string]*SessionContext)
	}
	m.sessions[userID][sessionID] = &SessionContext{
		Session:  session,
		LastUsed: time.Now(),
	}
	m.mutex.Unlock()

	return session, nil
}

// GetSessions retrieves all sessions for a user
func (m *Manager) GetSessions(userID string) ([]types.ChatSession, error) {
	sessions, err := m.storage.GetSessions(userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get sessions from storage: %w", err)
	}

	return sessions, nil
}

// DeleteSession deletes a session for a user
func (m *Manager) DeleteSession(userID, sessionID string) error {
	// Remove from active sessions
	m.mutex.Lock()
	if userSessions, exists := m.sessions[userID]; exists {
		delete(userSessions, sessionID)
		if len(userSessions) == 0 {
			delete(m.sessions, userID)
		}
	}
	m.mutex.Unlock()

	// Delete from storage
	if err := m.storage.DeleteSession(userID, sessionID); err != nil {
		return fmt.Errorf("failed to delete session from storage: %w", err)
	}

	return nil
}

// AddMessage adds a message to a user's session
func (m *Manager) AddMessage(userID, sessionID string, message *types.ChatMessage) error {
	// Get session context
	m.mutex.RLock()
	var ctx *SessionContext
	if userSessions, exists := m.sessions[userID]; exists {
		ctx = userSessions[sessionID]
	}
	m.mutex.RUnlock()
	if ctx == nil {
		// Load session if not in memory
		_, err := m.GetSession(userID, sessionID)
		if err != nil {
			return fmt.Errorf("session not found: %w", err)
		}

		m.mutex.RLock()
		if userSessions, exists := m.sessions[userID]; exists {
			ctx = userSessions[sessionID]
		}
		m.mutex.RUnlock()

		if ctx == nil {
			return fmt.Errorf("failed to load session context")
		}
	}

	// Set session ID and timestamp
	message.SessionID = sessionID
	if message.Timestamp.IsZero() {
		message.Timestamp = time.Now()
	}

	// Save to storage
	if err := m.storage.CreateMessage(userID, message); err != nil {
		return fmt.Errorf("failed to save message to storage: %w", err)
	}

	// Update session in memory
	ctx.mutex.Lock()
	ctx.Session.Messages = append(ctx.Session.Messages, *message)
	ctx.Session.UpdatedAt = message.Timestamp
	ctx.LastUsed = time.Now()
	ctx.mutex.Unlock()

	return nil
}

// GetSessionContext retrieves the session context for workflow execution
func (m *Manager) GetSessionContext(userID, sessionID string) (*SessionContext, error) {
	m.mutex.RLock()
	var ctx *SessionContext
	if userSessions, exists := m.sessions[userID]; exists {
		ctx = userSessions[sessionID]
	}
	m.mutex.RUnlock()
	if ctx == nil {
		// Load session if not in memory
		_, err := m.GetSession(userID, sessionID)
		if err != nil {
			return nil, fmt.Errorf("session not found: %w", err)
		}

		m.mutex.RLock()
		if userSessions, exists := m.sessions[userID]; exists {
			ctx = userSessions[sessionID]
		}
		m.mutex.RUnlock()
	}

	if ctx == nil {
		return nil, fmt.Errorf("session context not found")
	}

	ctx.LastUsed = time.Now()
	return ctx, nil
}

// GetSessionHistory returns the message history for a user's session
func (m *Manager) GetSessionHistory(userID, sessionID string) ([]types.ChatMessage, error) {
	session, err := m.GetSession(userID, sessionID)
	if err != nil {
		return nil, err
	}

	return session.Messages, nil
}

// UpdateSessionTitle updates the title of a user's session
func (m *Manager) UpdateSessionTitle(userID, sessionID, title string) error {
	// Get session context
	m.mutex.RLock()
	var ctx *SessionContext
	if userSessions, exists := m.sessions[userID]; exists {
		ctx = userSessions[sessionID]
	}
	m.mutex.RUnlock()
	if ctx == nil {
		return fmt.Errorf("session not found")
	}

	// Update in memory
	ctx.mutex.Lock()
	ctx.Session.Title = title
	ctx.Session.UpdatedAt = time.Now()
	ctx.mutex.Unlock()

	// Update in storage
	if err := m.storage.UpdateSession(userID, ctx.Session); err != nil {
		return fmt.Errorf("failed to update session in storage: %w", err)
	}

	return nil
}

// CleanupInactiveSessions removes inactive sessions from memory
func (m *Manager) CleanupInactiveSessions(maxAge time.Duration) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	cutoff := time.Now().Add(-maxAge)
	for userID, userSessions := range m.sessions {
		for sessionID, ctx := range userSessions {
			if ctx.LastUsed.Before(cutoff) {
				delete(userSessions, sessionID)
			}
		}
		// Remove empty user sessions map
		if len(userSessions) == 0 {
			delete(m.sessions, userID)
		}
	}
}