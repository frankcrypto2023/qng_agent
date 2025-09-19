package storage

import "qng-agent/internal/types"

// Storage defines the interface for data persistence
type Storage interface {
	// Session management
	CreateSession(session *types.ChatSession) error
	GetSession(sessionID string) (*types.ChatSession, error)
	GetSessions() ([]types.ChatSession, error)
	UpdateSession(session *types.ChatSession) error
	DeleteSession(sessionID string) error

	// Message management
	CreateMessage(message *types.ChatMessage) error
	GetMessages(sessionID string) ([]types.ChatMessage, error)

	// Settings management
	SaveSettings(settings *types.AppSettings) error
	LoadSettings() (*types.AppSettings, error)

	// Database operations
	Close() error
}