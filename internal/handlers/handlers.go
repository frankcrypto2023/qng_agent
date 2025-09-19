package handlers

import (
	"context"
	"fmt"
	"net/http"
	"qng-agent/internal/llm"
	"qng-agent/internal/session"
	"qng-agent/internal/storage"
	"qng-agent/internal/types"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// Handler contains all HTTP handlers
type Handler struct {
	sessionManager *session.Manager
	llmManager     *llm.Manager
	storage        storage.Storage
}

// NewHandler creates a new handler
func NewHandler(sessionManager *session.Manager, llmManager *llm.Manager, storage storage.Storage) *Handler {
	return &Handler{
		sessionManager: sessionManager,
		llmManager:     llmManager,
		storage:        storage,
	}
}

// CreateSession creates a new chat session
func (h *Handler) CreateSession(c *gin.Context) {
	var req types.CreateSessionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	session, err := h.sessionManager.CreateSession(req.Title)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create session"})
		return
	}

	c.JSON(http.StatusOK, types.CreateSessionResponse{
		Session: *session,
	})
}

// GetSessions retrieves all chat sessions
func (h *Handler) GetSessions(c *gin.Context) {
	sessions, err := h.sessionManager.GetSessions()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get sessions"})
		return
	}

	c.JSON(http.StatusOK, sessions)
}

// GetSession retrieves a specific chat session
func (h *Handler) GetSession(c *gin.Context) {
	sessionID := c.Param("id")
	
	session, err := h.sessionManager.GetSession(sessionID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Session not found"})
		return
	}

	c.JSON(http.StatusOK, session)
}

// DeleteSession deletes a chat session
func (h *Handler) DeleteSession(c *gin.Context) {
	sessionID := c.Param("id")
	
	err := h.sessionManager.DeleteSession(sessionID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete session"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Session deleted"})
}

// StreamChat handles streaming chat responses
func (h *Handler) StreamChat(c *gin.Context) {
	var req types.SendMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate session exists
	_, err := h.sessionManager.GetSession(req.SessionID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Session not found"})
		return
	}

	// Set headers for SSE
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Access-Control-Allow-Origin", "*")

	// Create user message
	userMessage := &types.ChatMessage{
		ID:        uuid.New().String(),
		Role:      "user",
		Content:   req.Message,
		Timestamp: time.Now(),
	}

	// Save user message
	if err := h.sessionManager.AddMessage(req.SessionID, userMessage); err != nil {
		c.SSEvent("error", gin.H{"error": "Failed to save user message"})
		return
	}

	// Get session history for context (currently not used in simplified implementation)
	_, err = h.sessionManager.GetSessionHistory(req.SessionID)
	if err != nil {
		c.SSEvent("error", gin.H{"error": "Failed to get session history"})
		return
	}

	// Process through LLM graph
	ctx, cancel := context.WithTimeout(c.Request.Context(), 60*time.Second)
	defer cancel()

	// Create assistant message
	assistantMessageID := uuid.New().String()
	
	// Send message ID first
	c.SSEvent("data", gin.H{"message_id": assistantMessageID})

	// Process message through LLM and stream response
	chatMessages := []types.ChatMessage{
		{Role: "user", Content: req.Message},
	}
	
	response, err := h.llmManager.GetClient().GetCompletion(ctx, chatMessages)
	if err != nil {
		c.SSEvent("error", gin.H{"error": "Failed to process message"})
		return
	}

	// Stream the response character by character
	for i, char := range response {
		c.SSEvent("data", gin.H{"content": string(char)})
		c.Writer.Flush()
		
		// Add small delay for realistic streaming effect
		if i%10 == 0 {
			time.Sleep(50 * time.Millisecond)
		}
	}

	// If authentication is needed, send auth request (simplified logic)
	// This could be enhanced with more sophisticated intent analysis
	if false { // Disabled for now since we removed graph workflow
		c.SSEvent("data", gin.H{
			"need_auth": true,
			"auth_type": "wallet_connection",
			"message":   "Please connect your wallet to continue",
		})
	}

	// Send completion signal
	c.SSEvent("data", "[DONE]")

	// Save assistant message
	assistantMessage := &types.ChatMessage{
		ID:        assistantMessageID,
		Role:      "assistant", 
		Content:   response,
		Timestamp: time.Now(),
	}

	if err := h.sessionManager.AddMessage(req.SessionID, assistantMessage); err != nil {
		// Log error but don't fail the request
		fmt.Printf("Failed to save assistant message: %v\n", err)
	}
}

// GetSettings retrieves application settings
func (h *Handler) GetSettings(c *gin.Context) {
	settings, err := h.storage.LoadSettings()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load settings"})
		return
	}

	c.JSON(http.StatusOK, settings)
}

// UpdateSettings updates application settings
func (h *Handler) UpdateSettings(c *gin.Context) {
	var settings types.AppSettings
	if err := c.ShouldBindJSON(&settings); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Update LLM client if configuration changed
	if settings.LLMProvider.URL != "" && settings.LLMProvider.Token != "" {
		h.llmManager.UpdateClientFromConfig(settings.LLMProvider)
	}

	// Save settings
	if err := h.storage.SaveSettings(&settings); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save settings"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Settings updated"})
}

// Health check endpoint
func (h *Handler) Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":    "healthy",
		"timestamp": time.Now(),
	})
}