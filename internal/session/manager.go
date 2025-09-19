package session

import (
	"fmt"
	"qng-agent/internal/storage"
	"qng-agent/internal/types"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Manager handles chat session management
type Manager struct {
	storage  storage.Storage
	sessions map[string]*SessionContext
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
		sessions: make(map[string]*SessionContext),
	}
}

// CreateSession creates a new chat session
func (m *Manager) CreateSession(title string) (*types.ChatSession, error) {
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

	if err := m.storage.CreateSession(session); err != nil {
		return nil, fmt.Errorf("failed to create session in storage: %w", err)
	}

	// Add to active sessions
	m.mutex.Lock()
	m.sessions[session.ID] = &SessionContext{
		Session:  session,
		LastUsed: time.Now(),
	}
	m.mutex.Unlock()

	return session, nil
}

// GetSession retrieves a session by ID
func (m *Manager) GetSession(sessionID string) (*types.ChatSession, error) {
	// Check active sessions first
	m.mutex.RLock()
	if ctx, exists := m.sessions[sessionID]; exists {
		ctx.mutex.RLock()
		session := ctx.Session
		ctx.mutex.RUnlock()
		ctx.LastUsed = time.Now()
		m.mutex.RUnlock()
		return session, nil
	}
	m.mutex.RUnlock()

	// Load from storage
	session, err := m.storage.GetSession(sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get session from storage: %w", err)
	}

	// Add to active sessions
	m.mutex.Lock()
	m.sessions[sessionID] = &SessionContext{
		Session:  session,
		LastUsed: time.Now(),
	}
	m.mutex.Unlock()

	return session, nil
}

// GetSessions retrieves all sessions
func (m *Manager) GetSessions() ([]types.ChatSession, error) {
	sessions, err := m.storage.GetSessions()
	if err != nil {
		return nil, fmt.Errorf("failed to get sessions from storage: %w", err)
	}

	return sessions, nil
}

// DeleteSession deletes a session
func (m *Manager) DeleteSession(sessionID string) error {
	// Remove from active sessions
	m.mutex.Lock()
	delete(m.sessions, sessionID)
	m.mutex.Unlock()

	// Delete from storage
	if err := m.storage.DeleteSession(sessionID); err != nil {
		return fmt.Errorf("failed to delete session from storage: %w", err)
	}

	return nil
}

// AddMessage adds a message to a session
func (m *Manager) AddMessage(sessionID string, message *types.ChatMessage) error {
	// Get session context
	m.mutex.RLock()
	ctx, exists := m.sessions[sessionID]
	m.mutex.RUnlock()

	if !exists {
		// Load session if not in memory
		_, err := m.GetSession(sessionID)
		if err != nil {
			return fmt.Errorf("session not found: %w", err)
		}
		
		m.mutex.RLock()
		ctx = m.sessions[sessionID]
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
	if err := m.storage.CreateMessage(message); err != nil {
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
func (m *Manager) GetSessionContext(sessionID string) (*SessionContext, error) {
	m.mutex.RLock()
	ctx, exists := m.sessions[sessionID]
	m.mutex.RUnlock()

	if !exists {
		// Load session if not in memory
		_, err := m.GetSession(sessionID)
		if err != nil {
			return nil, fmt.Errorf("session not found: %w", err)
		}
		
		m.mutex.RLock()
		ctx = m.sessions[sessionID]
		m.mutex.RUnlock()
	}

	if ctx == nil {
		return nil, fmt.Errorf("session context not found")
	}

	ctx.LastUsed = time.Now()
	return ctx, nil
}

// GetSessionHistory returns the message history for a session
func (m *Manager) GetSessionHistory(sessionID string) ([]types.ChatMessage, error) {
	session, err := m.GetSession(sessionID)
	if err != nil {
		return nil, err
	}

	return session.Messages, nil
}

// UpdateSessionTitle updates the title of a session
func (m *Manager) UpdateSessionTitle(sessionID, title string) error {
	// Get session context
	m.mutex.RLock()
	ctx, exists := m.sessions[sessionID]
	m.mutex.RUnlock()

	if !exists {
		return fmt.Errorf("session not found")
	}

	// Update in memory
	ctx.mutex.Lock()
	ctx.Session.Title = title
	ctx.Session.UpdatedAt = time.Now()
	ctx.mutex.Unlock()

	// Update in storage
	if err := m.storage.UpdateSession(ctx.Session); err != nil {
		return fmt.Errorf("failed to update session in storage: %w", err)
	}

	return nil
}

// CleanupInactiveSessions removes inactive sessions from memory
func (m *Manager) CleanupInactiveSessions(maxAge time.Duration) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	cutoff := time.Now().Add(-maxAge)
	for sessionID, ctx := range m.sessions {
		if ctx.LastUsed.Before(cutoff) {
			delete(m.sessions, sessionID)
		}
	}
}