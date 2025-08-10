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
	case "llamacpp":
		return NewLlamaCppLocalClient(config.LlamaCpp)
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
	if contains(lastMessage, "兑换") && contains(lastMessage, "质押") {
		return `{
			"intent_type": "compound",
			"actions": ["兑换MEER为MTK", "质押MTK"],
			"operations": [
				{
					"type": "swap",
					"from_token": "MEER",
					"to_token": "MTK",
					"amount": "1.0",
					"description": "兑换MEER为MTK"
				},
				{
					"type": "stake",
					"token": "MTK",
					"amount": "兑换获得的MTK数量",
					"description": "质押MTK"
				}
			],
			"analysis": "用户请求进行复合操作：先兑换MEER为MTK，然后质押MTK",
			"recommendations": ["确保兑换比例合理", "检查质押池的安全性"],
			"risk_assessment": "中等风险，需要验证兑换比例和质押池",
			"estimated_gas": "0.001 MEER",
			"estimated_cost": "1.001 MEER"
		}`, nil
	}

	if contains(lastMessage, "兑换") || contains(lastMessage, "swap") {
		return `{
			"intent_type": "swap",
			"actions": ["兑换MEER为MTK"],
			"operations": [
				{
					"type": "swap",
					"from_token": "MEER",
					"to_token": "MTK",
					"amount": "1.0",
					"description": "兑换MEER为MTK"
				}
			],
			"analysis": "用户请求兑换MEER为MTK",
			"recommendations": ["检查兑换比例", "确认滑点设置"],
			"risk_assessment": "低风险，标准兑换操作",
			"estimated_gas": "0.0005 MEER",
			"estimated_cost": "1.0005 MEER"
		}`, nil
	}

	if contains(lastMessage, "质押") || contains(lastMessage, "stake") {
		return `{
			"intent_type": "stake",
			"actions": ["质押MTK"],
			"operations": [
				{
					"type": "stake",
					"token": "MTK",
					"amount": "1.0",
					"description": "质押MTK"
				}
			],
			"analysis": "用户请求质押MTK",
			"recommendations": ["检查质押池APY", "确认质押期限"],
			"risk_assessment": "低风险，标准质押操作",
			"estimated_gas": "0.0003 MEER",
			"estimated_cost": "0.0003 MEER"
		}`, nil
	}

	// 默认响应
	return `{
		"intent_type": "swap",
		"actions": ["兑换操作"],
		"operations": [
			{
				"type": "swap",
				"from_token": "MEER",
				"to_token": "MTK",
				"amount": "1.0",
				"description": "兑换MEER为MTK"
			}
		],
		"analysis": "标准兑换操作",
		"recommendations": ["检查兑换比例"],
		"risk_assessment": "低风险",
		"estimated_gas": "0.0005 MEER",
		"estimated_cost": "1.0005 MEER"
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
