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

type GeminiClient struct {
	config config.GeminiConfig
	client *http.Client
}

type GeminiRequest struct {
	Contents []struct {
		Parts []struct {
			Text string `json:"text"`
		} `json:"parts"`
	} `json:"contents"`
}

type GeminiResponse struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"content"`
	} `json:"candidates"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func NewGeminiClient(config config.GeminiConfig) (Client, error) {
	if config.APIKey == "" {
		return NewMockClient(), nil
	}

	client := &http.Client{
		Timeout: time.Duration(config.Timeout) * time.Second,
	}

	return &GeminiClient{
		config: config,
		client: client,
	}, nil
}

func (c *GeminiClient) Chat(ctx context.Context, messages []Message) (string, error) {
	if c.config.APIKey == "" {
		// 如果没有API密钥，使用模拟客户端
		mockClient := NewMockClient()
		return mockClient.Chat(ctx, messages)
	}

	// 转换消息格式
	var contents []struct {
		Parts []struct {
			Text string `json:"text"`
		} `json:"parts"`
	}

	for _, msg := range messages {
		contents = append(contents, struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		}{
			Parts: []struct {
				Text string `json:"text"`
			}{
				{Text: msg.Content},
			},
		})
	}

	requestBody := GeminiRequest{
		Contents: contents,
	}

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s", 
		c.config.Model, c.config.APIKey)
	
	req, err := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(string(jsonData)))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	var response GeminiResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	if response.Error != nil {
		return "", fmt.Errorf("Gemini API error: %s", response.Error.Message)
	}

	if len(response.Candidates) == 0 {
		return "", fmt.Errorf("no response from Gemini")
	}

	if len(response.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("no content in Gemini response")
	}

	return response.Candidates[0].Content.Parts[0].Text, nil
}
