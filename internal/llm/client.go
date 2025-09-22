package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"qng-agent/internal/types"
	"strings"
	"time"
)

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
}

// NewOpenAIClient creates a new OpenAI client
func NewOpenAIClient(baseURL, apiKey, model string, maxTokens int, temp float64) *OpenAIClient {
	return &OpenAIClient{
		baseURL:   strings.TrimSuffix(baseURL, "/"),
		apiKey:    apiKey,
		model:     model,
		maxTokens: maxTokens,
		temp:      temp,
	}
}

// NewOpenRouterClient creates a new OpenRouter client
func NewOpenRouterClient(baseURL, apiKey, model string, maxTokens int, temp float64, appName, appURL string) *OpenRouterClient {
	return &OpenRouterClient{
		baseURL:   strings.TrimSuffix(baseURL, "/"),
		apiKey:    apiKey,
		model:     model,
		maxTokens: maxTokens,
		temp:      temp,
		appName:   appName,
		appURL:    appURL,
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

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	// Read streaming response
	decoder := json.NewDecoder(resp.Body)
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
				onChunk(choice.Delta.Content)
			}
			if choice.FinishReason == "stop" {
				break
			}
		}
	}

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

	client := &http.Client{Timeout: 60 * time.Second}
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

	return response.Choices[0].Message.Content, nil
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
		)
	case types.ProviderTypeGroq:
		// Groq uses OpenAI-compatible API
		client = NewOpenAIClient(
			config.URL,
			config.Token,
			config.ModelName,
			2048, // maxTokens
			0.7,  // temperature
		)
	case types.ProviderTypeOpenAI, types.ProviderTypeCustom:
		// OpenAI and custom providers use the same client
		client = NewOpenAIClient(
			config.URL,
			config.Token,
			config.ModelName,
			2048, // maxTokens
			0.7,  // temperature
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
		)
	default:
		// Default to OpenAI client for backward compatibility
		client = NewOpenAIClient(
			config.URL,
			config.Token,
			config.ModelName,
			2048, // maxTokens
			0.7,  // temperature
		)
	}

	m.SetDefaultClient(client)
}

// DummyClient is a simple client for testing when no LLM is configured
type DummyClient struct{}

// StreamCompletion implements Client interface with dummy responses
func (d *DummyClient) StreamCompletion(ctx context.Context, messages []types.ChatMessage, onChunk func(string), onComplete func()) error {
	response := "Hello! I'm a QNG Intelligent Agent. Currently, no LLM provider is configured, so I'm running in demo mode. Please configure your LLM settings in the settings panel to enable full functionality."

	// Simulate streaming by sending chunks
	for _, char := range response {
		onChunk(string(char))
		time.Sleep(50 * time.Millisecond) // Simulate typing speed
	}

	onComplete()
	return nil
}

// GetCompletion implements Client interface with dummy responses
func (d *DummyClient) GetCompletion(ctx context.Context, messages []types.ChatMessage) (string, error) {
	return "Hello! I'm a QNG Intelligent Agent. Currently, no LLM provider is configured, so I'm running in demo mode. Please configure your LLM settings in the settings panel to enable full functionality.", nil
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
	log.Printf("Request: %s %s", c.baseURL+"/chat/completions", string(jsonData))
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

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	// Read streaming response
	decoder := json.NewDecoder(resp.Body)
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
				onChunk(choice.Delta.Content)
			}
			if choice.FinishReason == "stop" {
				break
			}
		}
	}

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
	log.Printf("Request: %s %s", c.baseURL+"/chat/completions", string(jsonData))
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

	client := &http.Client{Timeout: 60 * time.Second}
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

	return response.Choices[0].Message.Content, nil
}
