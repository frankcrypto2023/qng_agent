package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"qng_agent/internal/config"
	"strings"
	"time"
)

type AnthropicClient struct {
	config config.AnthropicConfig
	client *http.Client
}

type AnthropicRequest struct {
	Model     string    `json:"model"`
	Messages  []Message `json:"messages"`
	MaxTokens int       `json:"max_tokens,omitempty"`
}

type AnthropicResponse struct {
	Content []struct {
		Text string `json:"text"`
	} `json:"content"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func NewAnthropicClient(config config.AnthropicConfig) (Client, error) {
	if config.APIKey == "" {
		return NewMockClient(), nil
	}

	client := &http.Client{
		Timeout: time.Duration(config.Timeout) * time.Second,
	}

	return &AnthropicClient{
		config: config,
		client: client,
	}, nil
}

func (c *AnthropicClient) Chat(ctx context.Context, messages []Message) (string, error) {
	if c.config.APIKey == "" {
		// 如果没有API密钥，使用模拟客户端
		mockClient := NewMockClient()
		return mockClient.Chat(ctx, messages)
	}

	requestBody := AnthropicRequest{
		Model:     c.config.Model,
		Messages:  messages,
		MaxTokens: 2000,
	}

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	url := "https://api.anthropic.com/v1/messages"
	req, err := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(string(jsonData)))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.config.APIKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := c.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	var response AnthropicResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	if response.Error != nil {
		return "", fmt.Errorf("Anthropic API error: %s", response.Error.Message)
	}

	if len(response.Content) == 0 {
		return "", fmt.Errorf("no response from Anthropic")
	}

	return response.Content[0].Text, nil
}

func (c *AnthropicClient) ChatStream(ctx context.Context, messages []Message) (<-chan string, error) {
	// 使用模拟客户端进行流式响应
	mockClient := NewMockClient()
	return mockClient.ChatStream(ctx, messages)
}

func (c *AnthropicClient) GetModelInfo() map[string]interface{} {
	return map[string]interface{}{
		"provider": "anthropic",
		"model":    c.config.Model,
		"timeout":  c.config.Timeout,
	}
}
