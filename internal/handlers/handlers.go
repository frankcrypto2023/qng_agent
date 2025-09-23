package handlers

import (
	"context"
	"fmt"
	"net/http"
	"qng-agent/internal/config"
	"qng-agent/internal/graph"
	"qng-agent/internal/llm"
	"qng-agent/internal/middleware"
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
	graphManager   *graph.LLMGraphManager
	config         *config.Config
	storage        storage.Storage
}

// NewHandler creates a new handler
func NewHandler(sessionManager *session.Manager, llmManager *llm.Manager, cfg *config.Config, storage storage.Storage) *Handler {
	graphManager := graph.NewLLMGraphManager(llmManager.GetClient(), llmManager, cfg, storage)

	return &Handler{
		sessionManager: sessionManager,
		llmManager:     llmManager,
		graphManager:   graphManager,
		config:         cfg,
		storage:        storage,
	}
}

// CreateSession creates a new chat session
func (h *Handler) CreateSession(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "User ID not found"})
		return
	}

	var req types.CreateSessionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	session, err := h.sessionManager.CreateSession(userID, req.Title)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create session"})
		return
	}

	c.JSON(http.StatusOK, types.CreateSessionResponse{
		Session: *session,
	})
}

// GetSessions retrieves all chat sessions for the user
func (h *Handler) GetSessions(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "User ID not found"})
		return
	}

	sessions, err := h.sessionManager.GetSessions(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get sessions"})
		return
	}

	// Ensure we return an empty array instead of null
	if sessions == nil {
		sessions = []types.ChatSession{}
	}

	c.JSON(http.StatusOK, sessions)
}

// GetSession retrieves a specific chat session
func (h *Handler) GetSession(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "User ID not found"})
		return
	}

	sessionID := c.Param("id")

	session, err := h.sessionManager.GetSession(userID, sessionID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Session not found"})
		return
	}

	c.JSON(http.StatusOK, session)
}

// DeleteSession deletes a chat session
func (h *Handler) DeleteSession(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "User ID not found"})
		return
	}

	sessionID := c.Param("id")

	err := h.sessionManager.DeleteSession(userID, sessionID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete session"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Session deleted"})
}

// UpdateSession updates a chat session
func (h *Handler) UpdateSession(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "User ID not found"})
		return
	}

	sessionID := c.Param("id")

	var req struct {
		Title string `json:"title"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Update session title
	err := h.sessionManager.UpdateSessionTitle(userID, sessionID, req.Title)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update session"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Session updated"})
}

// StreamChat handles streaming chat responses
func (h *Handler) StreamChat(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "User ID not found"})
		return
	}

	var req types.SendMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate session exists
	_, err := h.sessionManager.GetSession(userID, req.SessionID)
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
	if err := h.sessionManager.AddMessage(userID, req.SessionID, userMessage); err != nil {
		c.SSEvent("error", gin.H{"error": "Failed to save user message"})
		return
	}

	// Get session history for context
	history, err := h.sessionManager.GetSessionHistory(userID, req.SessionID)
	if err != nil {
		c.SSEvent("error", gin.H{"error": "Failed to get session history"})
		return
	}

	// Process through QNG Graph workflow
	// Use configured timeout with a reasonable minimum
	timeoutSeconds := h.config.LLM.Timeout
	if timeoutSeconds <= 0 {
		timeoutSeconds = 120 // Default 2 minutes if not configured
	}
	// Add some buffer for processing overhead
	timeoutSeconds += 30
	ctx, cancel := context.WithTimeout(c.Request.Context(), time.Duration(timeoutSeconds)*time.Second)
	defer cancel()

	// Create assistant message
	assistantMessageID := uuid.New().String()

	// Send message ID first using standard SSE format
	fmt.Fprintf(c.Writer, "data: %s\n\n", `{"message_id":"`+assistantMessageID+`"}`)
	c.Writer.Flush()

	// Process message through QNG Graph workflow
	response, needsAuth, err := h.graphManager.ProcessUserMessage(ctx, userID, req.Message, history[:len(history)-1])
	if err != nil {
		fmt.Fprintf(c.Writer, "data: %s\n\n", `{"error":"Failed to process message: `+err.Error()+`"}`)
		c.Writer.Flush()
		return
	}

	// Stream the response character by character
	for i, char := range response {
		fmt.Fprintf(c.Writer, "data: %s\n\n", `{"content":"`+string(char)+`"}`)
		c.Writer.Flush()

		// Add small delay for realistic streaming effect
		if i%10 == 0 {
			time.Sleep(50 * time.Millisecond)
		}
	}

	// If authentication is needed, send auth request
	if needsAuth {
		authData := `{"need_auth":true,"auth_type":"wallet_connection","message":"Please connect your wallet to continue"}`
		fmt.Fprintf(c.Writer, "data: %s\n\n", authData)
		c.Writer.Flush()
	}

	// Send completion signal
	fmt.Fprintf(c.Writer, "data: [DONE]\n\n")
	c.Writer.Flush()

	// Save assistant message
	assistantMessage := &types.ChatMessage{
		ID:        assistantMessageID,
		Role:      "assistant",
		Content:   response,
		Timestamp: time.Now(),
	}

	if err := h.sessionManager.AddMessage(userID, req.SessionID, assistantMessage); err != nil {
		// Log error but don't fail the request
		fmt.Printf("Failed to save assistant message: %v\n", err)
	}
}

// GetSettings retrieves application settings for the user
func (h *Handler) GetSettings(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "User ID not found"})
		return
	}

	settings, err := h.storage.LoadSettings(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load settings"})
		return
	}

	c.JSON(http.StatusOK, settings)
}

// UpdateSettings updates application settings for the user
func (h *Handler) UpdateSettings(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "User ID not found"})
		return
	}

	var settings types.AppSettings
	if err := c.ShouldBindJSON(&settings); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Update LLM client if configuration changed
	if settings.LLMProvider.URL != "" && settings.LLMProvider.Token != "" {
		h.llmManager.UpdateClientFromConfig(settings.LLMProvider)
		h.graphManager = graph.NewLLMGraphManager(h.llmManager.GetClient(), h.llmManager, h.config, h.storage)
	}

	// Save settings
	if err := h.storage.SaveSettings(userID, &settings); err != nil {
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
