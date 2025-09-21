package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"qng-agent/internal/types"
	"sort"
	"time"

	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/util"
)

// LevelDBStorage implements storage using LevelDB
type LevelDBStorage struct {
	db *leveldb.DB
}

// Key prefixes for different data types with user isolation
const (
	UserPrefix     = "user:"
	SessionPrefix  = "session:"
	MessagePrefix  = "message:"
	SettingsPrefix = "settings:"
	IndexPrefix    = "index:"
)

// Helper functions for generating user-scoped keys
func userSessionKey(userID, sessionID string) string {
	return UserPrefix + userID + ":" + SessionPrefix + sessionID
}

func userMessageKey(userID, sessionID, timestamp, messageID string) string {
	return UserPrefix + userID + ":" + MessagePrefix + sessionID + ":" + timestamp + ":" + messageID
}

func userSettingsKey(userID string) string {
	return UserPrefix + userID + ":" + SettingsPrefix + "app_settings"
}

func userSessionIndexKey(userID, timestamp, sessionID string) string {
	return UserPrefix + userID + ":" + IndexPrefix + "sessions:" + timestamp + ":" + sessionID
}

func userSessionIndexPrefix(userID string) string {
	return UserPrefix + userID + ":" + IndexPrefix + "sessions:"
}

func userMessagePrefix(userID, sessionID string) string {
	return UserPrefix + userID + ":" + MessagePrefix + sessionID + ":"
}

// NewLevelDBStorage creates a new LevelDB storage instance
func NewLevelDBStorage(dbPath string) (*LevelDBStorage, error) {
	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return nil, fmt.Errorf("failed to create database directory: %w", err)
	}

	db, err := leveldb.OpenFile(dbPath, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to open LevelDB: %w", err)
	}

	storage := &LevelDBStorage{db: db}
	return storage, nil
}

// CreateSession creates a new chat session
func (s *LevelDBStorage) CreateSession(userID string, session *types.ChatSession) error {
	sessionKey := userSessionKey(userID, session.ID)
	
	data, err := json.Marshal(session)
	if err != nil {
		return fmt.Errorf("failed to marshal session: %w", err)
	}

	// Store session data
	if err := s.db.Put([]byte(sessionKey), data, nil); err != nil {
		return fmt.Errorf("failed to store session: %w", err)
	}

	// Update session index for quick listing
	indexKey := userSessionIndexKey(userID, session.UpdatedAt.Format(time.RFC3339), session.ID)
	if err := s.db.Put([]byte(indexKey), []byte(session.ID), nil); err != nil {
		return fmt.Errorf("failed to update session index: %w", err)
	}

	return nil
}

// GetSession retrieves a chat session with its messages
func (s *LevelDBStorage) GetSession(userID, sessionID string) (*types.ChatSession, error) {
	sessionKey := userSessionKey(userID, sessionID)
	
	data, err := s.db.Get([]byte(sessionKey), nil)
	if err != nil {
		if err == leveldb.ErrNotFound {
			return nil, fmt.Errorf("session not found")
		}
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	var session types.ChatSession
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, fmt.Errorf("failed to unmarshal session: %w", err)
	}

	// Get messages for this session
	messages, err := s.GetMessages(userID, sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get messages: %w", err)
	}
	
	session.Messages = messages
	return &session, nil
}

// GetSessions retrieves all chat sessions for a user
func (s *LevelDBStorage) GetSessions(userID string) ([]types.ChatSession, error) {
	var sessions []types.ChatSession
	
	// Use index to get sessions ordered by update time (newest first)
	iter := s.db.NewIterator(util.BytesPrefix([]byte(userSessionIndexPrefix(userID))), nil)
	defer iter.Release()

	// Collect session IDs in reverse order (newest first)
	var sessionIDs []string
	for iter.Next() {
		sessionID := string(iter.Value())
		sessionIDs = append(sessionIDs, sessionID)
	}

	if err := iter.Error(); err != nil {
		return nil, fmt.Errorf("failed to iterate session index: %w", err)
	}

	// Reverse the slice to get newest first
	for i := len(sessionIDs) - 1; i >= 0; i-- {
		sessionID := sessionIDs[i]
		
		session, err := s.GetSession(userID, sessionID)
		if err != nil {
			// Skip sessions that can't be loaded
			continue
		}
		
		sessions = append(sessions, *session)
	}

	return sessions, nil
}

// UpdateSession updates a chat session
func (s *LevelDBStorage) UpdateSession(userID string, session *types.ChatSession) error {
	// Remove old index entry by finding it
	iter := s.db.NewIterator(util.BytesPrefix([]byte(userSessionIndexPrefix(userID))), nil)
	defer iter.Release()

	for iter.Next() {
		if string(iter.Value()) == session.ID {
			// Delete old index entry
			s.db.Delete(iter.Key(), nil)
			break
		}
	}

	// Create session (will update existing and create new index)
	return s.CreateSession(userID, session)
}

// DeleteSession deletes a chat session and its messages
func (s *LevelDBStorage) DeleteSession(userID, sessionID string) error {
	// Delete session
	sessionKey := userSessionKey(userID, sessionID)
	if err := s.db.Delete([]byte(sessionKey), nil); err != nil {
		return fmt.Errorf("failed to delete session: %w", err)
	}

	// Delete all messages for this session
	messagePrefix := userMessagePrefix(userID, sessionID)
	iter := s.db.NewIterator(util.BytesPrefix([]byte(messagePrefix)), nil)
	defer iter.Release()

	for iter.Next() {
		if err := s.db.Delete(iter.Key(), nil); err != nil {
			return fmt.Errorf("failed to delete message: %w", err)
		}
	}

	if err := iter.Error(); err != nil {
		return fmt.Errorf("failed to iterate messages for deletion: %w", err)
	}

	// Delete session from index
	indexPrefix := userSessionIndexPrefix(userID)
	indexIter := s.db.NewIterator(util.BytesPrefix([]byte(indexPrefix)), nil)
	defer indexIter.Release()

	for indexIter.Next() {
		if string(indexIter.Value()) == sessionID {
			if err := s.db.Delete(indexIter.Key(), nil); err != nil {
				return fmt.Errorf("failed to delete session from index: %w", err)
			}
			break
		}
	}

	if err := indexIter.Error(); err != nil {
		return fmt.Errorf("failed to iterate session index for deletion: %w", err)
	}

	return nil
}

// CreateMessage creates a new chat message
func (s *LevelDBStorage) CreateMessage(userID string, message *types.ChatMessage) error {
	// Use timestamp in key for ordering
	messageKey := userMessageKey(userID, message.SessionID, message.Timestamp.Format(time.RFC3339Nano), message.ID)
	
	data, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	if err := s.db.Put([]byte(messageKey), data, nil); err != nil {
		return fmt.Errorf("failed to store message: %w", err)
	}

	// Update session timestamp
	session, err := s.GetSession(userID, message.SessionID)
	if err != nil {
		return fmt.Errorf("failed to get session for update: %w", err)
	}

	session.UpdatedAt = message.Timestamp
	if err := s.UpdateSession(userID, session); err != nil {
		return fmt.Errorf("failed to update session timestamp: %w", err)
	}

	return nil
}

// GetMessages retrieves all messages for a session
func (s *LevelDBStorage) GetMessages(userID, sessionID string) ([]types.ChatMessage, error) {
	var messages []types.ChatMessage

	messagePrefix := userMessagePrefix(userID, sessionID)
	iter := s.db.NewIterator(util.BytesPrefix([]byte(messagePrefix)), nil)
	defer iter.Release()

	for iter.Next() {
		var message types.ChatMessage
		if err := json.Unmarshal(iter.Value(), &message); err != nil {
			return nil, fmt.Errorf("failed to unmarshal message: %w", err)
		}
		messages = append(messages, message)
	}

	if err := iter.Error(); err != nil {
		return nil, fmt.Errorf("failed to iterate messages: %w", err)
	}

	// Ensure we return an empty slice instead of nil
	if messages == nil {
		messages = []types.ChatMessage{}
	}

	// Sort messages by timestamp
	sort.Slice(messages, func(i, j int) bool {
		return messages[i].Timestamp.Before(messages[j].Timestamp)
	})

	return messages, nil
}

// SaveSettings saves application settings for a user
func (s *LevelDBStorage) SaveSettings(userID string, settings *types.AppSettings) error {
	data, err := json.Marshal(settings)
	if err != nil {
		return fmt.Errorf("failed to marshal settings: %w", err)
	}

	settingsKey := userSettingsKey(userID)
	if err := s.db.Put([]byte(settingsKey), data, nil); err != nil {
		return fmt.Errorf("failed to save settings: %w", err)
	}

	return nil
}

// LoadSettings loads application settings for a user
func (s *LevelDBStorage) LoadSettings(userID string) (*types.AppSettings, error) {
	settingsKey := userSettingsKey(userID)
	
	data, err := s.db.Get([]byte(settingsKey), nil)
	if err != nil {
		if err == leveldb.ErrNotFound {
			// Return default settings
			return &types.AppSettings{
				MCPServers: []types.MCPServerConfig{},
				LLMProvider: types.LLMProviderConfig{
					Name:      "OpenAI",
					URL:       "https://api.openai.com/v1",
					Token:     "",
					ModelName: "gpt-4",
				},
			}, nil
		}
		return nil, fmt.Errorf("failed to load settings: %w", err)
	}

	var settings types.AppSettings
	if err := json.Unmarshal(data, &settings); err != nil {
		return nil, fmt.Errorf("failed to unmarshal settings: %w", err)
	}

	return &settings, nil
}

// GetStats returns database statistics
func (s *LevelDBStorage) GetStats() (map[string]interface{}, error) {
	stats := make(map[string]interface{})
	
	// Count sessions
	sessionCount := 0
	sessionIter := s.db.NewIterator(util.BytesPrefix([]byte(SessionPrefix)), nil)
	for sessionIter.Next() {
		sessionCount++
	}
	sessionIter.Release()
	
	// Count messages
	messageCount := 0
	messageIter := s.db.NewIterator(util.BytesPrefix([]byte(MessagePrefix)), nil)
	for messageIter.Next() {
		messageCount++
	}
	messageIter.Release()

	stats["sessions"] = sessionCount
	stats["messages"] = messageCount
	stats["database_type"] = "LevelDB"
	
	return stats, nil
}

// CompactDatabase compacts the LevelDB database
func (s *LevelDBStorage) CompactDatabase() error {
	return s.db.CompactRange(util.Range{Start: nil, Limit: nil})
}

// Close closes the database connection
func (s *LevelDBStorage) Close() error {
	return s.db.Close()
}

// Backup creates a backup of the database
func (s *LevelDBStorage) Backup(backupPath string) error {
	// Ensure backup directory exists
	if err := os.MkdirAll(filepath.Dir(backupPath), 0755); err != nil {
		return fmt.Errorf("failed to create backup directory: %w", err)
	}

	// Create snapshot
	snapshot, err := s.db.GetSnapshot()
	if err != nil {
		return fmt.Errorf("failed to create snapshot: %w", err)
	}
	defer snapshot.Release()

	// Create backup database
	backupDB, err := leveldb.OpenFile(backupPath, nil)
	if err != nil {
		return fmt.Errorf("failed to create backup database: %w", err)
	}
	defer backupDB.Close()

	// Copy all data
	iter := snapshot.NewIterator(nil, nil)
	defer iter.Release()

	for iter.Next() {
		if err := backupDB.Put(iter.Key(), iter.Value(), nil); err != nil {
			return fmt.Errorf("failed to write backup data: %w", err)
		}
	}

	if err := iter.Error(); err != nil {
		return fmt.Errorf("failed to iterate during backup: %w", err)
	}

	return nil
}