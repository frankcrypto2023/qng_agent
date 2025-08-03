package llm

import (
	"context"
	"fmt"
	"qng_agent/internal/config"
)

type Client interface {
	Chat(ctx context.Context, messages []Message) (string, error)
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

func NewClient(config config.LLMConfig) (Client, error) {
	switch config.Provider {
	case "openai":
		return NewOpenAIClient(config.OpenAI)
	case "anthropic":
		return NewAnthropicClient(config.Anthropic)
	case "gemini":
		return NewGeminiClient(config.Gemini)
	case "modelscope":
		return NewModelScopeClient(config.ModelScope)
	default:
		return NewOpenAIClient(config.OpenAI)
	}
}

// MockClient 模拟LLM客户端，用于测试
type MockClient struct{}

func NewMockClient() Client {
	return &MockClient{}
}

func (c *MockClient) Chat(ctx context.Context, messages []Message) (string, error) {
	// 模拟LLM响应
	if len(messages) == 0 {
		return "", fmt.Errorf("no messages provided")
	}

	lastMessage := messages[len(messages)-1].Content

	// 根据消息内容返回模拟响应
	if contains(lastMessage, "兑换") || contains(lastMessage, "swap") {
		return `{
			"tasks": [
				{
					"type": "swap",
					"from_token": "USDT",
					"to_token": "BTC",
					"amount": "1000"
				}
			]
		}`, nil
	}

	if contains(lastMessage, "质押") || contains(lastMessage, "stake") {
		return `{
			"tasks": [
				{
					"type": "stake",
					"token": "BTC",
					"amount": "0.1",
					"pool": "compound"
				}
			]
		}`, nil
	}

	// 默认响应
	return `{
		"tasks": [
			{
				"type": "swap",
				"from_token": "USDT",
				"to_token": "BTC",
				"amount": "500"
			}
		]
	}`, nil
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr ||
		(len(s) > len(substr) && (s[:len(substr)] == substr ||
			s[len(s)-len(substr):] == substr ||
			containsSubstring(s, substr))))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
