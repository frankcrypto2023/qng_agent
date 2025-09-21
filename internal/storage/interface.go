package storage

import "qng-agent/internal/types"

// Storage defines the interface for data persistence with user isolation
type Storage interface {
	// Session management - all operations scoped by userID
	CreateSession(userID string, session *types.ChatSession) error
	GetSession(userID, sessionID string) (*types.ChatSession, error)
	GetSessions(userID string) ([]types.ChatSession, error)
	UpdateSession(userID string, session *types.ChatSession) error
	DeleteSession(userID, sessionID string) error

	// Message management - scoped by userID
	CreateMessage(userID string, message *types.ChatMessage) error
	GetMessages(userID, sessionID string) ([]types.ChatMessage, error)

	// Settings management - scoped by userID
	SaveSettings(userID string, settings *types.AppSettings) error
	LoadSettings(userID string) (*types.AppSettings, error)

	// Database operations
	Close() error
}