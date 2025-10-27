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
	GetCompletionWithTools(ctx context.Context, messages []map[string]interface{}, tools []types.Tool) (*types.ChatMessage, error)
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

	// Create HTTP client with retry and better error handling
	client := &http.Client{
		Timeout: time.Duration(c.timeout) * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:          100,
			MaxIdleConnsPerHost:   10,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		},
	}
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

	log.Debug("llm", "action", "OpenAI GetCompletion Request",
		"url", c.baseURL+"/chat/completions",
		"model", c.model,
		"max_tokens", c.maxTokens,
		"temperature", c.temp,
		"request_size", len(jsonData))

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/chat/completions", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	// Safely truncate API key for logging
	keyPrefix := c.apiKey
	if len(keyPrefix) > 10 {
		keyPrefix = keyPrefix[:10]
	}

	log.Debug("llm", "action", "OpenAI Request Headers",
		"content_type", req.Header.Get("Content-Type"),
		"authorization", "Bearer "+keyPrefix+"...")

	// Create HTTP client with retry and better error handling
	client := &http.Client{
		Timeout: time.Duration(c.timeout) * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:          100,
			MaxIdleConnsPerHost:   10,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		},
	}
	log.Debug("llm", "action", "GetCompletion HTTP client created", "timeout_seconds", c.timeout)

	log.Debug("llm", "action", "Sending HTTP request",
		"method", req.Method,
		"url", req.URL.String(),
		"timeout", c.timeout)

	// Retry mechanism for network issues
	var resp *http.Response
	var err error
	maxRetries := 3

	for attempt := 1; attempt <= maxRetries; attempt++ {
		log.Debug("llm", "action", "HTTP request attempt", "attempt", attempt, "max_retries", maxRetries)

		resp, err = client.Do(req)
		if err == nil {
			break // Success
		}

		log.Debug("llm", "action", "HTTP request failed", "attempt", attempt, "error", err.Error())

		if attempt < maxRetries {
			// Wait before retry (exponential backoff)
			waitTime := time.Duration(attempt) * time.Second
			log.Debug("llm", "action", "Retrying after delay", "wait_seconds", waitTime.Seconds())
			time.Sleep(waitTime)

			// Recreate request body for retry
			req.Body = io.NopCloser(bytes.NewBuffer(jsonData))
		}
	}

	if err != nil {
		log.Debug("llm", "action", "HTTP request failed after all retries", "error", err.Error())
		return "", fmt.Errorf("failed to send request after %d attempts: %w", maxRetries, err)
	}
	defer resp.Body.Close()

	log.Debug("llm", "action", "HTTP response received",
		"status_code", resp.StatusCode,
		"status", resp.Status,
		"content_length", resp.ContentLength,
		"headers", fmt.Sprintf("%v", resp.Header))

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		errorMsg := fmt.Sprintf("API request failed with status %d: %s", resp.StatusCode, string(body))

		// Provide more specific error messages for common status codes
		switch resp.StatusCode {
		case 401:
			errorMsg += "\n\nPossible solutions:\n- Check if your API key is correct\n- Ensure your API key has proper permissions\n- Verify the API key format (should start with 'sk-' for OpenAI)"
		case 403:
			errorMsg += "\n\nPossible solutions:\n- Check if your API key has sufficient permissions\n- Verify your account has enough credits/quota\n- Ensure the API endpoint URL is correct"
		case 429:
			errorMsg += "\n\nPossible solutions:\n- You have exceeded the rate limit\n- Wait a moment and try again\n- Consider upgrading your API plan"
		case 500, 502, 503, 504:
			errorMsg += "\n\nPossible solutions:\n- The API service is temporarily unavailable\n- Try again in a few moments\n- Check the API provider's status page"
		}

		return "", fmt.Errorf(errorMsg)
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

// GetCompletionWithTools gets a completion with function calling support for OpenRouter
func (c *OpenRouterClient) GetCompletionWithTools(ctx context.Context, messages []map[string]interface{}, tools []types.Tool) (*types.ChatMessage, error) {
	// Convert tools to OpenAI format
	apiTools := make([]map[string]interface{}, len(tools))
	for i, tool := range tools {
		apiTools[i] = map[string]interface{}{
			"type": "function",
			"function": map[string]interface{}{
				"name":        tool.Function.Name,
				"description": tool.Function.Description,
				"parameters":  tool.Function.Parameters,
			},
		}
	}

	request := map[string]interface{}{
		"model":       c.model,
		"messages":    messages,
		"max_tokens":  c.maxTokens,
		"temperature": c.temp,
		"tools":       apiTools,
		"tool_choice": "auto",
	}

	jsonData, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/chat/completions", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
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

	// Create HTTP client with retry and better error handling
	client := &http.Client{
		Timeout: time.Duration(c.timeout) * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:          100,
			MaxIdleConnsPerHost:   10,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		},
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var response map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	choices, ok := response["choices"].([]interface{})
	if !ok || len(choices) == 0 {
		return nil, fmt.Errorf("no choices in response")
	}

	choice, ok := choices[0].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid choice format")
	}

	message, ok := choice["message"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid message format")
	}

	content, _ := message["content"].(string)
	toolCalls, _ := message["tool_calls"].([]interface{})

	result := &types.ChatMessage{
		Role:    "assistant",
		Content: content,
	}

	// Convert tool calls if present
	if toolCalls != nil {
		for _, tc := range toolCalls {
			toolCallMap, ok := tc.(map[string]interface{})
			if !ok {
				continue
			}

			toolCall := types.ToolCall{
				ID:   toolCallMap["id"].(string),
				Type: toolCallMap["type"].(string),
			}

			function, ok := toolCallMap["function"].(map[string]interface{})
			if ok {
				toolCall.Function = types.FunctionCall{
					Name:      function["name"].(string),
					Arguments: function["arguments"].(map[string]interface{}),
				}
			}

			result.ToolCalls = append(result.ToolCalls, toolCall)
		}
	}

	return result, nil
}

// GetCompletionWithTools gets a completion with function calling support
func (c *OpenAIClient) GetCompletionWithTools(ctx context.Context, messages []map[string]interface{}, tools []types.Tool) (*types.ChatMessage, error) {
	// Convert tools to OpenAI format
	apiTools := make([]map[string]interface{}, len(tools))
	for i, tool := range tools {
		apiTools[i] = map[string]interface{}{
			"type": "function",
			"function": map[string]interface{}{
				"name":        tool.Function.Name,
				"description": tool.Function.Description,
				"parameters":  tool.Function.Parameters,
			},
		}
	}

	request := map[string]interface{}{
		"model":       c.model,
		"messages":    messages,
		"max_tokens":  c.maxTokens,
		"temperature": c.temp,
		"tools":       apiTools,
		"tool_choice": "auto",
	}

	jsonData, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	log.Debug("llm", "action", "OpenAI GetCompletionWithTools Request",
		"url", c.baseURL+"/chat/completions",
		"model", c.model,
		"max_tokens", c.maxTokens,
		"temperature", c.temp,
		"tools_count", len(tools),
		"request_size", len(jsonData))

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/chat/completions", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	// Safely truncate API key for logging
	keyPrefix := c.apiKey
	if len(keyPrefix) > 10 {
		keyPrefix = keyPrefix[:10]
	}

	log.Debug("llm", "action", "OpenAI Tools Request Headers",
		"content_type", req.Header.Get("Content-Type"),
		"authorization", "Bearer "+keyPrefix+"...")

	// Create HTTP client with retry and better error handling
	client := &http.Client{
		Timeout: time.Duration(c.timeout) * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:          100,
			MaxIdleConnsPerHost:   10,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		},
	}

	log.Debug("llm", "action", "Sending Tools HTTP request",
		"method", req.Method,
		"url", req.URL.String(),
		"timeout", c.timeout)

	// Retry mechanism for network issues
	var resp *http.Response
	var err error
	maxRetries := 3

	for attempt := 1; attempt <= maxRetries; attempt++ {
		log.Debug("llm", "action", "Tools HTTP request attempt", "attempt", attempt, "max_retries", maxRetries)

		resp, err = client.Do(req)
		if err == nil {
			break // Success
		}

		log.Debug("llm", "action", "Tools HTTP request failed", "attempt", attempt, "error", err.Error())

		if attempt < maxRetries {
			// Wait before retry (exponential backoff)
			waitTime := time.Duration(attempt) * time.Second
			log.Debug("llm", "action", "Retrying Tools request after delay", "wait_seconds", waitTime.Seconds())
			time.Sleep(waitTime)

			// Recreate request body for retry
			req.Body = io.NopCloser(bytes.NewBuffer(jsonData))
		}
	}

	if err != nil {
		log.Debug("llm", "action", "Tools HTTP request failed after all retries", "error", err.Error())
		return nil, fmt.Errorf("failed to send request after %d attempts: %w", maxRetries, err)
	}
	defer resp.Body.Close()

	log.Debug("llm", "action", "Tools HTTP response received",
		"status_code", resp.StatusCode,
		"status", resp.Status,
		"content_length", resp.ContentLength,
		"headers", fmt.Sprintf("%v", resp.Header))

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		errorMsg := fmt.Sprintf("API request failed with status %d: %s", resp.StatusCode, string(body))

		// Provide more specific error messages for common status codes
		switch resp.StatusCode {
		case 401:
			errorMsg += "\n\nPossible solutions:\n- Check if your API key is correct\n- Ensure your API key has proper permissions\n- Verify the API key format (should start with 'sk-' for OpenAI)"
		case 403:
			errorMsg += "\n\nPossible solutions:\n- Check if your API key has sufficient permissions\n- Verify your account has enough credits/quota\n- Ensure the API endpoint URL is correct"
		case 429:
			errorMsg += "\n\nPossible solutions:\n- You have exceeded the rate limit\n- Wait a moment and try again\n- Consider upgrading your API plan"
		case 500, 502, 503, 504:
			errorMsg += "\n\nPossible solutions:\n- The API service is temporarily unavailable\n- Try again in a few moments\n- Check the API provider's status page"
		}

		return nil, fmt.Errorf(errorMsg)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	choices, ok := response["choices"].([]interface{})
	if !ok || len(choices) == 0 {
		return nil, fmt.Errorf("no choices in response")
	}

	choice, ok := choices[0].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid choice format")
	}

	message, ok := choice["message"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid message format")
	}

	content, _ := message["content"].(string)
	toolCalls, _ := message["tool_calls"].([]interface{})

	result := &types.ChatMessage{
		Role:    "assistant",
		Content: content,
	}

	// Convert tool calls if present
	if toolCalls != nil {
		for _, tc := range toolCalls {
			toolCallMap, ok := tc.(map[string]interface{})
			if !ok {
				continue
			}

			toolCall := types.ToolCall{
				ID:   toolCallMap["id"].(string),
				Type: toolCallMap["type"].(string),
			}

			function, ok := toolCallMap["function"].(map[string]interface{})
			if ok {
				toolCall.Function = types.FunctionCall{
					Name:      function["name"].(string),
					Arguments: function["arguments"].(map[string]interface{}),
				}
			}

			result.ToolCalls = append(result.ToolCalls, toolCall)
		}
	}

	return result, nil
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

// GetCompletionWithTools implements Client interface with dummy responses
func (d *DummyClient) GetCompletionWithTools(ctx context.Context, messages []map[string]interface{}, tools []types.Tool) (*types.ChatMessage, error) {
	return &types.ChatMessage{
		Role:    "assistant",
		Content: "Hello! I'm a QNG Intelligent Agent. Currently, no LLM provider is configured, so I'm running in demo mode. Please configure your LLM settings in the settings panel to enable full functionality.",
	}, nil
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

	// Create HTTP client with retry and better error handling
	client := &http.Client{
		Timeout: time.Duration(c.timeout) * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:          100,
			MaxIdleConnsPerHost:   10,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		},
	}
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

	log.Debug("llm", "action", "OpenRouter GetCompletion Request",
		"url", c.baseURL+"/chat/completions",
		"model", c.model,
		"max_tokens", c.maxTokens,
		"temperature", c.temp,
		"request_size", len(jsonData))

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/chat/completions", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	// Safely truncate API key for logging
	keyPrefix := c.apiKey
	if len(keyPrefix) > 10 {
		keyPrefix = keyPrefix[:10]
	}

	log.Debug("llm", "action", "OpenRouter Request Headers",
		"content_type", req.Header.Get("Content-Type"),
		"authorization", "Bearer "+keyPrefix+"...")

	// Add OpenRouter specific headers
	if c.appName != "" {
		req.Header.Set("X-Title", c.appName)
	}
	if c.appURL != "" {
		req.Header.Set("HTTP-Referer", c.appURL)
	}

	// Create HTTP client with retry and better error handling
	client := &http.Client{
		Timeout: time.Duration(c.timeout) * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:          100,
			MaxIdleConnsPerHost:   10,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		},
	}
	log.Debug("llm", "action", "OpenRouter GetCompletion HTTP client created", "timeout_seconds", c.timeout)

	log.Debug("llm", "action", "Sending OpenRouter HTTP request",
		"method", req.Method,
		"url", req.URL.String(),
		"timeout", c.timeout)

	// Retry mechanism for network issues
	var resp *http.Response
	var err error
	maxRetries := 3

	for attempt := 1; attempt <= maxRetries; attempt++ {
		log.Debug("llm", "action", "OpenRouter HTTP request attempt", "attempt", attempt, "max_retries", maxRetries)

		resp, err = client.Do(req)
		if err == nil {
			break // Success
		}

		log.Debug("llm", "action", "OpenRouter HTTP request failed", "attempt", attempt, "error", err.Error())

		if attempt < maxRetries {
			// Wait before retry (exponential backoff)
			waitTime := time.Duration(attempt) * time.Second
			log.Debug("llm", "action", "Retrying OpenRouter request after delay", "wait_seconds", waitTime.Seconds())
			time.Sleep(waitTime)

			// Recreate request body for retry
			req.Body = io.NopCloser(bytes.NewBuffer(jsonData))
		}
	}

	if err != nil {
		log.Debug("llm", "action", "OpenRouter HTTP request failed after all retries", "error", err.Error())
		return "", fmt.Errorf("failed to send request after %d attempts: %w", maxRetries, err)
	}
	defer resp.Body.Close()

	log.Debug("llm", "action", "OpenRouter HTTP response received",
		"status_code", resp.StatusCode,
		"status", resp.Status,
		"content_length", resp.ContentLength,
		"headers", fmt.Sprintf("%v", resp.Header))

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
