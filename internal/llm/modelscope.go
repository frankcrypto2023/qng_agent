package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"qng_agent/internal/config"
	"strings"
	"time"
)

type ModelScopeClient struct {
	config config.ModelScopeConfig
	client *http.Client
}

// ModelScope 请求结构
type ModelScopeRequest struct {
	Model       string               `json:"model"`
	Messages    []ModelScopeMessage  `json:"messages"`
	MaxTokens   int                  `json:"max_tokens,omitempty"`
	Temperature float64              `json:"temperature,omitempty"`
	Stream      bool                 `json:"stream,omitempty"`
	Parameters  ModelScopeParameters `json:"parameters,omitempty"`
}

type ModelScopeMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ModelScopeParameters struct {
	MaxTokens   int     `json:"max_tokens,omitempty"`
	Temperature float64 `json:"temperature,omitempty"`
	TopP        float64 `json:"top_p,omitempty"`
}

// ModelScope 响应结构
type ModelScopeResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index   int `json:"index"`
		Message struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
	} `json:"error,omitempty"`
}

func NewModelScopeClient(config config.ModelScopeConfig) (Client, error) {
	if config.APIKey == "" {
		return NewMockClient(), nil
	}

	// 设置默认值
	if config.BaseURL == "" {
		config.BaseURL = "https://api.modelscope.cn/v1"
	}
	if config.Model == "" {
		config.Model = "qwen/Qwen2.5-7B-Instruct"
	}
	if config.Timeout == 0 {
		config.Timeout = 30
	}
	if config.MaxTokens == 0 {
		config.MaxTokens = 2000
	}

	client := &http.Client{
		Timeout: time.Duration(config.Timeout) * time.Second,
	}

	return &ModelScopeClient{
		config: config,
		client: client,
	}, nil
}

func (c *ModelScopeClient) Chat(ctx context.Context, messages []Message) (string, error) {
	if c.config.APIKey == "" {
		// fmt.Println("-------------------", c.config.APIKey, "mock")
		// 如果没有API密钥，使用模拟客户端
		mockClient := NewMockClient()
		return mockClient.Chat(ctx, messages)
	}

	// 转换消息格式
	var modelScopeMessages []ModelScopeMessage
	for _, msg := range messages {
		modelScopeMessages = append(modelScopeMessages, ModelScopeMessage{
			Role:    msg.Role,
			Content: msg.Content,
		})
	}

	// 构建请求
	requestBody := ModelScopeRequest{
		Model:     c.config.Model,
		Messages:  modelScopeMessages,
		MaxTokens: c.config.MaxTokens,
		Parameters: ModelScopeParameters{
			MaxTokens:   c.config.MaxTokens,
			Temperature: 0.7,
			TopP:        0.9,
		},
	}

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	// 构建请求URL
	url := fmt.Sprintf("%s/chat/completions", c.config.BaseURL)

	req, err := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(string(jsonData)))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	// 设置请求头
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.config.APIKey))
	req.Header.Set("User-Agent", "QNG-Agent/1.0")

	// 发送请求
	resp, err := c.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// 读取响应体
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	// 检查HTTP状态码
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("ModelScope API error: HTTP %d - %s", resp.StatusCode, string(body))
	}

	// 解析响应
	var response ModelScopeResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	// 检查API错误
	if response.Error != nil {
		return "", fmt.Errorf("ModelScope API error: %s (type: %s, code: %s)",
			response.Error.Message, response.Error.Type, response.Error.Code)
	}

	// 检查是否有响应内容
	if len(response.Choices) == 0 {
		return "", fmt.Errorf("no response from ModelScope")
	}

	// 返回第一个选择的响应内容
	content := response.Choices[0].Message.Content
	if content == "" {
		return "", fmt.Errorf("empty response from ModelScope")
	}
	// fmt.Println("-------------------", content)
	return content, nil
}

func (c *ModelScopeClient) ChatStream(ctx context.Context, messages []Message) (<-chan string, error) {
	// 使用模拟客户端进行流式响应
	mockClient := NewMockClient()
	return mockClient.ChatStream(ctx, messages)
}

func (c *ModelScopeClient) GetModelInfo() map[string]interface{} {
	return map[string]interface{}{
		"provider":   "modelscope",
		"model":      c.config.Model,
		"base_url":   c.config.BaseURL,
		"timeout":    c.config.Timeout,
		"max_tokens": c.config.MaxTokens,
	}
}
