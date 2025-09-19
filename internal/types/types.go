package types

import (
	"time"
)

// ChatMessage represents a message in a chat session
type ChatMessage struct {
	ID        string    `json:"id" db:"id"`
	SessionID string    `json:"session_id" db:"session_id"`
	Role      string    `json:"role" db:"role"` // "user" or "assistant"
	Content   string    `json:"content" db:"content"`
	Timestamp time.Time `json:"timestamp" db:"timestamp"`
}

// ChatSession represents a chat session
type ChatSession struct {
	ID        string        `json:"id" db:"id"`
	Title     string        `json:"title" db:"title"`
	Messages  []ChatMessage `json:"messages"`
	CreatedAt time.Time     `json:"created_at" db:"created_at"`
	UpdatedAt time.Time     `json:"updated_at" db:"updated_at"`
}

// MCPServerConfig represents MCP server configuration
type MCPServerConfig struct {
	Name    string `json:"name"`
	URL     string `json:"url"`
	Enabled bool   `json:"enabled"`
}

// LLMProviderConfig represents LLM provider configuration
type LLMProviderConfig struct {
	Name      string `json:"name"`
	URL       string `json:"url"`
	Token     string `json:"token"`
	ModelName string `json:"model_name"`
}

// AppSettings represents application settings
type AppSettings struct {
	MCPServers  []MCPServerConfig `json:"mcp_servers"`
	LLMProvider LLMProviderConfig `json:"llm_provider"`
}

// SendMessageRequest represents a request to send a message
type SendMessageRequest struct {
	SessionID string `json:"session_id" binding:"required"`
	Message   string `json:"message" binding:"required"`
}

// SendMessageResponse represents the response when sending a message
type SendMessageResponse struct {
	MessageID string `json:"message_id"`
	Content   string `json:"content"`
}

// CreateSessionRequest represents a request to create a new session
type CreateSessionRequest struct {
	Title string `json:"title"`
}

// CreateSessionResponse represents the response when creating a session
type CreateSessionResponse struct {
	Session ChatSession `json:"session"`
}

// StreamChunk represents a chunk of streaming response
type StreamChunk struct {
	MessageID string `json:"message_id,omitempty"`
	Content   string `json:"content,omitempty"`
	Done      bool   `json:"done,omitempty"`
}

// WorkflowNode represents a node in a workflow graph
type WorkflowNode struct {
	ID          string                 `json:"id"`
	Type        string                 `json:"type"`
	Name        string                 `json:"name"`
	Config      map[string]interface{} `json:"config,omitempty"`
	Web3Config  map[string]interface{} `json:"web3_config,omitempty"`
}

// WorkflowEdge represents an edge in a workflow graph
type WorkflowEdge struct {
	ID        string `json:"id"`
	Source    string `json:"source"`
	Target    string `json:"target"`
	Condition string `json:"condition,omitempty"`
}

// WorkflowConfig represents a complete workflow configuration
type WorkflowConfig struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Nodes       []WorkflowNode `json:"nodes"`
	Edges       []WorkflowEdge `json:"edges"`
}

// LLMGraphState represents the state during LLM graph execution
type LLMGraphState struct {
	Input       map[string]interface{} `json:"input"`
	Context     map[string]interface{} `json:"context"`
	History     []ChatMessage          `json:"history"`
	Intent      string                 `json:"intent"`
	MCPTools    []string               `json:"mcp_tools"`
	Workflow    *WorkflowConfig        `json:"workflow,omitempty"`
	NeedAuth    bool                   `json:"need_auth"`
	AuthRequest interface{}            `json:"auth_request,omitempty"`
}

// IntentAnalysisResult represents the result of intent analysis
type IntentAnalysisResult struct {
	Intent       string                 `json:"intent"`
	Confidence   float64                `json:"confidence"`
	MCPTool      string                 `json:"mcp_tool,omitempty"`
	Parameters   map[string]interface{} `json:"parameters,omitempty"`
	WorkflowName string                 `json:"workflow_name,omitempty"`
	RequiresAuth bool                   `json:"requires_auth"`
}