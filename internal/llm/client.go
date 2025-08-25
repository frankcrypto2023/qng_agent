package llm

import (
	"context"
	"fmt"
	"qng_agent/internal/config"
	"strings"
	"time"
)

type Client interface {
	Chat(ctx context.Context, messages []Message) (string, error)
	ChatStream(ctx context.Context, messages []Message) (<-chan string, error)
	GetModelInfo() map[string]interface{}
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
	fmt.Printf("🔍 MockClient收到消息: %s\n", lastMessage)

	// 检查是否包含不支持的代币
	unsupportedTokens := []string{"usdt", "btc", "eth", "bnb", "ada", "dot", "link", "uni", "aave", "comp"}
	fmt.Printf("🔍 检查不支持的代币: %v\n", unsupportedTokens)

	for _, token := range unsupportedTokens {
		fmt.Printf("🔍 检查代币 '%s' 是否在消息中...\n", token)
		if contains(lastMessage, token) {
			fmt.Printf("❌ 检测到不支持的代币: %s\n", token)
			unsupportedToken := strings.ToUpper(token)
			return fmt.Sprintf(`{
				"intent_type": "error",
				"error": "不支持的代币: %s",
				"message": "抱歉，当前系统不支持 %s 代币。当前系统只支持 MEER 和 MTK 之间的兑换。",
				"recommendations": ["尝试兑换 MEER 为 MTK", "直接质押 MTK"],
				"tasks": [
					{
						"id": "error_1",
						"type": "error",
						"error": "不支持的代币: %s",
						"description": "当前系统只支持 MEER 和 MTK 之间的兑换",
						"message": "❌ 抱歉，当前系统不支持 %s 代币。\\n\\n💡 建议：\\n- 当前系统只支持 MEER 和 MTK 之间的兑换\\n- 您可以尝试：兑换 1 MEER 为 MTK，然后质押 MTK\\n- 或者直接质押您现有的 MTK"
					}
				]
			}`, unsupportedToken, unsupportedToken, unsupportedToken, unsupportedToken), nil
		}
	}
	fmt.Printf("✅ 未检测到不支持的代币\n")

	// 根据消息内容返回模拟响应
	// 注意：不支持的代币检查必须在其他检查之前
	if contains(lastMessage, "兑换") && contains(lastMessage, "质押") {
		// 再次检查是否包含不支持的代币（双重保险）
		for _, token := range unsupportedTokens {
			if contains(lastMessage, token) {
				unsupportedToken := strings.ToUpper(token)
				return fmt.Sprintf(`{
					"intent_type": "error",
					"error": "不支持的代币: %s",
					"message": "抱歉，当前系统不支持 %s 代币。当前系统只支持 MEER 和 MTK 之间的兑换。",
					"recommendations": ["尝试兑换 MEER 为 MTK", "直接质押 MTK"],
					"tasks": [
						{
							"id": "error_1",
							"type": "error",
							"error": "不支持的代币: %s",
							"description": "当前系统只支持 MEER 和 MTK 之间的兑换",
							"message": "❌ 抱歉，当前系统不支持 %s 代币。\\n\\n💡 建议：\\n- 当前系统只支持 MEER 和 MTK 之间的兑换\\n- 您可以尝试：兑换 1 MEER 为 MTK，然后质押 MTK\\n- 或者直接质押您现有的 MTK"
						}
					]
				}`, unsupportedToken, unsupportedToken, unsupportedToken, unsupportedToken), nil
			}
		}

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

func (c *MockClient) ChatStream(ctx context.Context, messages []Message) (<-chan string, error) {
	// 模拟流式响应
	contentChan := make(chan string, 10)

	go func() {
		defer close(contentChan)

		// 模拟流式输出
		response := "这是一个模拟的流式响应。"
		for _, char := range response {
			select {
			case <-ctx.Done():
				return
			case contentChan <- string(char):
				// 模拟延迟
				time.Sleep(50 * time.Millisecond)
			}
		}
	}()

	return contentChan, nil
}

func (c *MockClient) GetModelInfo() map[string]interface{} {
	return map[string]interface{}{
		"provider": "mock",
		"model":    "mock-model",
		"version":  "1.0.0",
	}
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
