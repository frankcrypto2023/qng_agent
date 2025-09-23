package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"qng-agent/internal/harmony"
	"qng-agent/internal/types"
	"strings"
	"time"

	"github.com/Qitmeer/qng/log"
)

// Global harmony parser instance for efficiency
var harmonyParser = harmony.NewHarmonyParser()

// filterHarmonyMetadata filters out GPT-OSS Harmony format metadata from message content
// Uses the new harmony parser for better accuracy and performance
func filterHarmonyMetadata(content string) string {
	log.Debug("llm", "action", "Filtering harmony metadata", "content", content)
	return harmonyParser.FilterMetadata(content)
}

// StreamBuffer handles buffering and filtering for streaming responses
type StreamBuffer struct {
	buffer   strings.Builder
	onChunk  func(string)
	lastSent int
}

// NewStreamBuffer creates a new stream buffer
func NewStreamBuffer(onChunk func(string)) *StreamBuffer {
	return &StreamBuffer{
		onChunk: onChunk,
	}
}

// Add adds content to the buffer and processes it
func (sb *StreamBuffer) Add(content string) {
	sb.buffer.WriteString(content)
	sb.processBuffer()
}

// Flush processes any remaining content in the buffer
func (sb *StreamBuffer) Flush() {
	if sb.buffer.Len() > sb.lastSent {
		remaining := sb.buffer.String()[sb.lastSent:]
		filtered := filterHarmonyMetadata(remaining)
		if filtered != "" {
			sb.onChunk(filtered)
		}
	}
}

// processBuffer processes the buffer content and sends filtered chunks
func (sb *StreamBuffer) processBuffer() {
	content := sb.buffer.String()

	// Look for potential harmony markers in the buffer
	// We need to be careful not to send partial harmony tags
	harmonyStart := strings.LastIndex(content, "<|")
	harmonyEnd := strings.LastIndex(content, "|>")

	// If we have a potential harmony start but no end, keep buffering
	if harmonyStart > harmonyEnd && harmonyStart >= 0 {
		// Check if this looks like a harmony tag start
		potentialTag := content[harmonyStart:]
		if strings.Contains(potentialTag, "channel") || strings.Contains(potentialTag, "end") {
			// Likely a harmony tag, don't send yet
			return
		}
	}

	// If we have a complete harmony tag, filter it out
	if harmonyStart >= 0 && harmonyEnd >= 0 && harmonyEnd > harmonyStart {
		// We have a complete tag, filter the content up to this point
		contentToSend := content[:harmonyStart] + content[harmonyEnd+2:]
		if len(contentToSend) > sb.lastSent {
			newContent := contentToSend[sb.lastSent:]
			filtered := filterHarmonyMetadata(newContent)
			if filtered != "" {
				sb.onChunk(filtered)
			}
			sb.lastSent = len(contentToSend)
		}
		return
	}

	// No harmony tags detected, send new content
	if len(content) > sb.lastSent {
		newContent := content[sb.lastSent:]
		// Check if the new content contains harmony markers
		if strings.Contains(newContent, "<|") || strings.Contains(newContent, "|>") {
			// Contains potential harmony markers, be more conservative
			// Only send content up to the first potential marker
			markerPos := strings.Index(newContent, "<|")
			if markerPos >= 0 {
				safeContent := newContent[:markerPos]
				if safeContent != "" {
					sb.onChunk(safeContent)
					sb.lastSent += len(safeContent)
				}
			} else {
				// No markers, safe to send
				sb.onChunk(newContent)
				sb.lastSent = len(content)
			}
		} else {
			// No markers, safe to send
			sb.onChunk(newContent)
			sb.lastSent = len(content)
		}
	}
}

// Client interface for LLM providers
type Client interface {
	StreamCompletion(ctx context.Context, messages []types.ChatMessage, onChunk func(string), onComplete func()) error
	GetCompletion(ctx context.Context, messages []types.ChatMessage) (string, error)
}

// OpenAIClient implements Client for OpenAI-compatible APIs
type OpenAIClient struct {
	baseURL   string
	apiKey    string
	model     string
	maxTokens int
	temp      float64
	timeout   int // Request timeout in seconds
}

// OpenRouterClient implements Client for OpenRouter API
type OpenRouterClient struct {
	baseURL   string
	apiKey    string
	model     string
	maxTokens int
	temp      float64
	appName   string
	appURL    string
	timeout   int // Request timeout in seconds
}

// NewOpenAIClient creates a new OpenAI client
func NewOpenAIClient(baseURL, apiKey, model string, maxTokens int, temp float64, timeout int) *OpenAIClient {
	return &OpenAIClient{
		baseURL:   strings.TrimSuffix(baseURL, "/"),
		apiKey:    apiKey,
		model:     model,
		maxTokens: maxTokens,
		temp:      temp,
		timeout:   timeout,
	}
}

// NewOpenRouterClient creates a new OpenRouter client
func NewOpenRouterClient(baseURL, apiKey, model string, maxTokens int, temp float64, appName, appURL string, timeout int) *OpenRouterClient {
	return &OpenRouterClient{
		baseURL:   strings.TrimSuffix(baseURL, "/"),
		apiKey:    apiKey,
		model:     model,
		maxTokens: maxTokens,
		temp:      temp,
		appName:   appName,
		appURL:    appURL,
		timeout:   timeout,
	}
}

// OpenAI API types
type openAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openAIRequest struct {
	Model       string          `json:"model"`
	Messages    []openAIMessage `json:"messages"`
	MaxTokens   int             `json:"max_tokens"`
	Temperature float64         `json:"temperature"`
	Stream      bool            `json:"stream"`
}

type openAIResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
}

// StreamCompletion streams completion from OpenAI API
func (c *OpenAIClient) StreamCompletion(ctx context.Context, messages []types.ChatMessage, onChunk func(string), onComplete func()) error {
	// Convert messages to OpenAI format
	apiMessages := make([]openAIMessage, len(messages))
	for i, msg := range messages {
		apiMessages[i] = openAIMessage{
			Role:    msg.Role,
			Content: msg.Content,
		}
	}

	request := openAIRequest{
		Model:       c.model,
		Messages:    apiMessages,
		MaxTokens:   c.maxTokens,
		Temperature: c.temp,
		Stream:      true,
	}

	jsonData, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/chat/completions", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	client := &http.Client{Timeout: time.Duration(c.timeout) * time.Second}
	log.Debug("llm", "action", "StreamCompletion HTTP client created", "timeout_seconds", c.timeout)
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	// Read streaming response with buffering for harmony filtering
	decoder := json.NewDecoder(resp.Body)
	buffer := NewStreamBuffer(onChunk)

	for {
		var response openAIResponse
		if err := decoder.Decode(&response); err != nil {
			if err == io.EOF {
				break
			}
			return fmt.Errorf("failed to decode response: %w", err)
		}

		if len(response.Choices) > 0 {
			choice := response.Choices[0]
			if choice.Delta.Content != "" {
				buffer.Add(choice.Delta.Content)
			}
			if choice.FinishReason == "stop" {
				break
			}
		}
	}

	// Flush any remaining buffered content
	buffer.Flush()
	onComplete()
	return nil
}

// GetCompletion gets a complete response from OpenAI API
func (c *OpenAIClient) GetCompletion(ctx context.Context, messages []types.ChatMessage) (string, error) {
	// Convert messages to OpenAI format
	apiMessages := make([]openAIMessage, len(messages))
	for i, msg := range messages {
		apiMessages[i] = openAIMessage{
			Role:    msg.Role,
			Content: msg.Content,
		}
	}

	request := openAIRequest{
		Model:       c.model,
		Messages:    apiMessages,
		MaxTokens:   c.maxTokens,
		Temperature: c.temp,
		Stream:      false,
	}

	jsonData, err := json.Marshal(request)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/chat/completions", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	client := &http.Client{Timeout: time.Duration(c.timeout) * time.Second}
	log.Debug("llm", "action", "GetCompletion HTTP client created", "timeout_seconds", c.timeout)
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var response openAIResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	if len(response.Choices) == 0 {
		return "", fmt.Errorf("no choices in response")
	}

	content := response.Choices[0].Message.Content
	return filterHarmonyMetadata(content), nil
}

// Manager handles LLM client management
type Manager struct {
	defaultClient Client
	clients       map[string]Client
}

// NewManager creates a new LLM manager
func NewManager() *Manager {
	return &Manager{
		clients: make(map[string]Client),
	}
}

// SetDefaultClient sets the default LLM client
func (m *Manager) SetDefaultClient(client Client) {
	m.defaultClient = client
}

// GetClient returns the default LLM client
func (m *Manager) GetClient() Client {
	if m.defaultClient == nil {
		// Return a dummy client for testing if no client is configured
		return &DummyClient{}
	}
	return m.defaultClient
}

// UpdateClientFromConfig updates the LLM client from configuration
func (m *Manager) UpdateClientFromConfig(config types.LLMProviderConfig) {
	var client Client

	// Set default timeout if not specified
	timeout := config.Timeout
	if timeout <= 0 {
		timeout = 120 // Default 2 minutes
		log.Debug("llm", "action", "LLM timeout not specified in config, using default", "timeout_seconds", timeout)
	} else {
		log.Debug("llm", "action", "LLM timeout configured", "timeout_seconds", timeout)
	}

	switch config.Type {
	case types.ProviderTypeOpenRouter:
		client = NewOpenRouterClient(
			config.URL,
			config.Token,
			config.ModelName,
			2048, // maxTokens
			0.7,  // temperature
			config.AppName,
			config.AppURL,
			timeout,
		)
	case types.ProviderTypeGroq:
		// Groq uses OpenAI-compatible API
		client = NewOpenAIClient(
			config.URL,
			config.Token,
			config.ModelName,
			2048, // maxTokens
			0.7,  // temperature
			timeout,
		)
	case types.ProviderTypeOpenAI, types.ProviderTypeCustom:
		// OpenAI and custom providers use the same client
		client = NewOpenAIClient(
			config.URL,
			config.Token,
			config.ModelName,
			2048, // maxTokens
			0.7,  // temperature
			timeout,
		)
	case types.ProviderTypeAnthropic:
		// For now, treat Anthropic as OpenAI-compatible
		// TODO: Implement proper Anthropic client when needed
		client = NewOpenAIClient(
			config.URL,
			config.Token,
			config.ModelName,
			2048, // maxTokens
			0.7,  // temperature
			timeout,
		)
	default:
		// Default to OpenAI client for backward compatibility
		client = NewOpenAIClient(
			config.URL,
			config.Token,
			config.ModelName,
			2048, // maxTokens
			0.7,  // temperature
			timeout,
		)
	}

	m.SetDefaultClient(client)
}

// DummyClient is a simple client for testing when no LLM is configured
type DummyClient struct{}

// StreamCompletion implements Client interface with dummy responses
func (d *DummyClient) StreamCompletion(ctx context.Context, messages []types.ChatMessage, onChunk func(string), onComplete func()) error {
	response := "Hello! I'm a QNG Intelligent Agent. Currently, no LLM provider is configured, so I'm running in demo mode. Please configure your LLM settings in the settings panel to enable full functionality."

	// Filter harmony metadata before streaming
	filteredResponse := filterHarmonyMetadata(response)

	// Simulate streaming by sending chunks
	for _, char := range filteredResponse {
		onChunk(string(char))
		time.Sleep(50 * time.Millisecond) // Simulate typing speed
	}

	onComplete()
	return nil
}

// GetCompletion implements Client interface with dummy responses
func (d *DummyClient) GetCompletion(ctx context.Context, messages []types.ChatMessage) (string, error) {
	content := "Hello! I'm a QNG Intelligent Agent. Currently, no LLM provider is configured, so I'm running in demo mode. Please configure your LLM settings in the settings panel to enable full functionality."
	return filterHarmonyMetadata(content), nil
}

// StreamCompletion streams completion from OpenRouter API
func (c *OpenRouterClient) StreamCompletion(ctx context.Context, messages []types.ChatMessage, onChunk func(string), onComplete func()) error {
	// Convert messages to OpenAI format
	apiMessages := make([]openAIMessage, len(messages))
	for i, msg := range messages {
		apiMessages[i] = openAIMessage{
			Role:    msg.Role,
			Content: msg.Content,
		}
	}

	request := openAIRequest{
		Model:       c.model,
		Messages:    apiMessages,
		MaxTokens:   c.maxTokens,
		Temperature: c.temp,
		Stream:      true,
	}

	jsonData, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}
	log.Debug("llm", "action", "StreamCompletion Request", "url", c.baseURL+"/chat/completions", "data", string(jsonData))
	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/chat/completions", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	// Add OpenRouter specific headers
	if c.appName != "" {
		req.Header.Set("X-Title", c.appName)
	}
	if c.appURL != "" {
		req.Header.Set("HTTP-Referer", c.appURL)
	}

	client := &http.Client{Timeout: time.Duration(c.timeout) * time.Second}
	log.Debug("llm", "action", "OpenRouter StreamCompletion HTTP client created", "timeout_seconds", c.timeout)
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	// Read streaming response with buffering for harmony filtering
	decoder := json.NewDecoder(resp.Body)
	buffer := NewStreamBuffer(onChunk)

	for {
		var response openAIResponse
		if err := decoder.Decode(&response); err != nil {
			if err == io.EOF {
				break
			}
			return fmt.Errorf("failed to decode response: %w", err)
		}

		if len(response.Choices) > 0 {
			choice := response.Choices[0]
			if choice.Delta.Content != "" {
				buffer.Add(choice.Delta.Content)
			}
			if choice.FinishReason == "stop" {
				break
			}
		}
	}

	// Flush any remaining buffered content
	buffer.Flush()
	onComplete()
	return nil
}

// GetCompletion gets a complete response from OpenRouter API
func (c *OpenRouterClient) GetCompletion(ctx context.Context, messages []types.ChatMessage) (string, error) {
	// Convert messages to OpenAI format
	apiMessages := make([]openAIMessage, len(messages))
	for i, msg := range messages {
		apiMessages[i] = openAIMessage{
			Role:    msg.Role,
			Content: msg.Content,
		}
	}

	request := openAIRequest{
		Model:       c.model,
		Messages:    apiMessages,
		MaxTokens:   c.maxTokens,
		Temperature: c.temp,
		Stream:      false,
	}

	jsonData, err := json.Marshal(request)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}
	log.Debug("llm", "action", "GetCompletion Request", "url", c.baseURL+"/chat/completions", "data", string(jsonData))
	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/chat/completions", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	// Add OpenRouter specific headers
	if c.appName != "" {
		req.Header.Set("X-Title", c.appName)
	}
	if c.appURL != "" {
		req.Header.Set("HTTP-Referer", c.appURL)
	}

	client := &http.Client{Timeout: time.Duration(c.timeout) * time.Second}
	log.Debug("llm", "action", "OpenRouter GetCompletion HTTP client created", "timeout_seconds", c.timeout)
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var response openAIResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	if len(response.Choices) == 0 {
		return "", fmt.Errorf("no choices in response")
	}

	content := response.Choices[0].Message.Content
	return filterHarmonyMetadata(content), nil
}
