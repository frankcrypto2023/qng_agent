package harmony

import (
	"context"
	"fmt"
	"log"
	"qng_agent/internal/config"
	"strings"
	"time"
)

// Demo 运行 Harmony 测试 demo
func Demo() {
	fmt.Println("🤖 Harmony GPT-OSS Demo")
	fmt.Println(strings.Repeat("=", 50))

	// 创建默认配置
	defaultConfig := &config.LlamaCppConfig{
		Temperature:    0.8,
		TopP:           0.95,
		TopK:           40,
		MaxTokens:      2000,
		RepeatPenalty:  1.1,
		CachePrompt:    true,
		ReasoningFormat: "none",
		Samplers:       "edkypmxt",
		DynatempRange:  0,
		DynatempExponent: 1,
		MinP:           0.05,
		TypicalP:       1,
		XtcProbability: 0,
		XtcThreshold:   0.1,
		RepeatLastN:    64,
		PresencePenalty: 0,
		FrequencyPenalty: 0,
		DryMultiplier:  0,
		DryBase:        1.75,
		DryAllowedLength: 2,
		DryPenaltyLastN: -1,
		TimingsPerToken: true,
	}

	// 创建客户端
	client := NewLlamaCppHarmonyClient("http://localhost:8081", defaultConfig)

	// 测试基本对话
	testBasicChat(client)

	// 测试工具调用
	testToolCalling(client)

	// 测试流式响应
	testStreamingChat(client)

	// 测试多通道输出
	testMultiChannel(client)
}

// testBasicChat 测试基本对话
func testBasicChat(client *LlamaCppHarmonyClient) {
	fmt.Println("\n📝 测试基本对话")
	fmt.Println(strings.Repeat("-", 30))

	// 创建对话
	conv := Conversation{
		Messages: []Message{
			NewSystemMessage(SystemContent{
				Instructions:  "你是一个有用的AI助手，请用中文回答问题。",
				Knowledge:     "2024-06",
				CurrentDate:   "2025-01-28",
				Reasoning:     "high",
				ValidChannels: []Channel{ChannelAnalysis, ChannelCommentary, ChannelFinal},
			}),
			NewUserMessage("你好！请介绍一下你自己。"),
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := client.Chat(ctx, conv, &ChatOptions{
		Temperature: 0.7,
		MaxTokens:   500,
	})

	if err != nil {
		log.Printf("❌ 对话失败: %v", err)
		return
	}

	fmt.Printf("✅ 助手回复:\n%s\n", getContentString(resp.Content))
	if resp.Usage != nil {
		fmt.Printf("📊 Token 使用: 提示=%d, 完成=%d, 总计=%d\n",
			resp.Usage.PromptTokens, resp.Usage.CompletionTokens, resp.Usage.TotalTokens)
	}
}

// testToolCalling 测试工具调用
func testToolCalling(client *LlamaCppHarmonyClient) {
	fmt.Println("\n🔧 测试工具调用")
	fmt.Println(strings.Repeat("-", 30))

	// 创建带工具定义的对话
	conv := Conversation{
		Messages: []Message{
			NewSystemMessage(SystemContent{
				Instructions:  "你是一个有用的AI助手，可以使用工具来帮助用户。",
				Knowledge:     "2024-06",
				CurrentDate:   "2025-01-28",
				Reasoning:     "high",
				ValidChannels: []Channel{ChannelAnalysis, ChannelCommentary, ChannelFinal},
				ToolChannels: map[string]Channel{
					"functions": ChannelCommentary,
				},
			}),
			NewDeveloperMessage(DeveloperContent{
				Instructions: "你可以使用以下工具来帮助用户：",
				Tools: map[string]interface{}{
					"functions": map[string]interface{}{
						"get_weather": "(_: {location: string, format?: 'celsius' | 'fahrenheit'}) => any",
						"get_time":    "() => any",
						"calculate":   "(_: {expression: string}) => any",
					},
				},
			}),
			NewUserMessage("请告诉我北京的天气怎么样？"),
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := client.Chat(ctx, conv, &ChatOptions{
		Temperature: 0.7,
		MaxTokens:   800,
	})

	if err != nil {
		log.Printf("❌ 工具调用失败: %v", err)
		return
	}

	fmt.Printf("✅ 助手回复:\n%s\n", getContentString(resp.Content))
}

// testStreamingChat 测试流式对话
func testStreamingChat(client *LlamaCppHarmonyClient) {
	fmt.Println("\n🌊 测试流式对话")
	fmt.Println(strings.Repeat("-", 30))

	conv := Conversation{
		Messages: []Message{
			NewSystemMessage(SystemContent{
				Instructions:  "你是一个有用的AI助手，请用中文回答问题。",
				Knowledge:     "2024-06",
				CurrentDate:   "2025-01-28",
				Reasoning:     "high",
				ValidChannels: []Channel{ChannelAnalysis, ChannelCommentary, ChannelFinal},
			}),
			NewUserMessage("请写一个关于人工智能的短文，大约100字。"),
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	stream, err := client.ChatStream(ctx, conv, &ChatOptions{
		Temperature: 0.7,
		MaxTokens:   300,
	})

	if err != nil {
		log.Printf("❌ 流式对话失败: %v", err)
		return
	}

	fmt.Print("✅ 助手回复 (流式):\n")
	var fullContent strings.Builder

	for {
		select {
		case <-ctx.Done():
			fmt.Println("\n⏰ 超时")
			return
		case resp := <-stream:
			if resp.Error != nil {
				log.Printf("❌ 流式响应错误: %v", resp.Error)
				return
			}

			if resp.Done {
				fmt.Println("\n✅ 流式对话完成")
				return
			}

			content := getContentString(resp.Content)
			fullContent.WriteString(content)
			fmt.Print(content)
		}
	}
}

// testMultiChannel 测试多通道输出
func testMultiChannel(client *LlamaCppHarmonyClient) {
	fmt.Println("\n📡 测试多通道输出")
	fmt.Println(strings.Repeat("-", 30))

	conv := Conversation{
		Messages: []Message{
			NewSystemMessage(SystemContent{
				Instructions:  "你是一个有用的AI助手，请使用多个通道来组织你的回答。",
				Knowledge:     "2024-06",
				CurrentDate:   "2025-01-28",
				Reasoning:     "high",
				ValidChannels: []Channel{ChannelAnalysis, ChannelCommentary, ChannelFinal},
			}),
			NewUserMessage("请分析一下区块链技术的优缺点，并给出你的建议。"),
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := client.Chat(ctx, conv, &ChatOptions{
		Temperature: 0.7,
		MaxTokens:   1000,
	})

	if err != nil {
		log.Printf("❌ 多通道输出失败: %v", err)
		return
	}

	fmt.Printf("✅ 助手回复:\n%s\n", getContentString(resp.Content))
}

// getContentString 获取消息内容的字符串表示
func getContentString(msg Message) string {
	switch content := msg.Content.(type) {
	case AssistantContent:
		// 只返回纯文本内容，不包含格式标记
		return content.Content
	case UserContent:
		return content.Text
	case SystemContent:
		return content.Instructions
	case DeveloperContent:
		return content.Instructions
	default:
		return fmt.Sprintf("%v", content)
	}
}

// DemoWithCustomPrompt 使用自定义提示的 demo
func DemoWithCustomPrompt(prompt string) {
	fmt.Printf("\n🎯 自定义提示 Demo: %s\n", prompt)
	fmt.Println(strings.Repeat("=", 60))

	// 创建默认配置
	defaultConfig := &config.LlamaCppConfig{
		Temperature:    0.8,
		TopP:           0.95,
		TopK:           40,
		MaxTokens:      2000,
		RepeatPenalty:  1.1,
		CachePrompt:    true,
		ReasoningFormat: "none",
		Samplers:       "edkypmxt",
		DynatempRange:  0,
		DynatempExponent: 1,
		MinP:           0.05,
		TypicalP:       1,
		XtcProbability: 0,
		XtcThreshold:   0.1,
		RepeatLastN:    64,
		PresencePenalty: 0,
		FrequencyPenalty: 0,
		DryMultiplier:  0,
		DryBase:        1.75,
		DryAllowedLength: 2,
		DryPenaltyLastN: -1,
		TimingsPerToken: true,
	}

	client := NewLlamaCppHarmonyClient("http://localhost:8081", defaultConfig)

	// 创建对话
	conv := Conversation{
		Messages: []Message{
			NewSystemMessage(SystemContent{
				Instructions:  "你是一个专业的AI助手，请用中文回答问题。",
				Knowledge:     "2024-06",
				CurrentDate:   "2025-01-28",
				Reasoning:     "high",
				ValidChannels: []Channel{ChannelAnalysis, ChannelCommentary, ChannelFinal},
			}),
			NewUserMessage(prompt),
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := client.Chat(ctx, conv, &ChatOptions{
		Temperature: 0.7,
		MaxTokens:   800,
	})

	if err != nil {
		log.Printf("❌ 自定义提示失败: %v", err)
		return
	}

	fmt.Printf("✅ 助手回复:\n%s\n", getContentString(resp.Content))
	if resp.Usage != nil {
		fmt.Printf("📊 Token 使用: 提示=%d, 完成=%d, 总计=%d\n",
			resp.Usage.PromptTokens, resp.Usage.CompletionTokens, resp.Usage.TotalTokens)
	}
}

// DemoWithTools 使用工具的 demo
func DemoWithTools(tools map[string]interface{}, userPrompt string) {
	fmt.Printf("\n🔧 工具调用 Demo: %s\n", userPrompt)
	fmt.Println(strings.Repeat("=", 60))

	// 创建默认配置
	defaultConfig := &config.LlamaCppConfig{
		Temperature:    0.8,
		TopP:           0.95,
		TopK:           40,
		MaxTokens:      2000,
		RepeatPenalty:  1.1,
		CachePrompt:    true,
		ReasoningFormat: "none",
		Samplers:       "edkypmxt",
		DynatempRange:  0,
		DynatempExponent: 1,
		MinP:           0.05,
		TypicalP:       1,
		XtcProbability: 0,
		XtcThreshold:   0.1,
		RepeatLastN:    64,
		PresencePenalty: 0,
		FrequencyPenalty: 0,
		DryMultiplier:  0,
		DryBase:        1.75,
		DryAllowedLength: 2,
		DryPenaltyLastN: -1,
		TimingsPerToken: true,
	}

	client := NewLlamaCppHarmonyClient("http://localhost:8081", defaultConfig)

	// 创建对话
	conv := Conversation{
		Messages: []Message{
			NewSystemMessage(SystemContent{
				Instructions:  "你是一个有用的AI助手，可以使用工具来帮助用户。",
				Knowledge:     "2024-06",
				CurrentDate:   "2025-01-28",
				Reasoning:     "high",
				ValidChannels: []Channel{ChannelAnalysis, ChannelCommentary, ChannelFinal},
				ToolChannels: map[string]Channel{
					"functions": ChannelCommentary,
				},
			}),
			NewDeveloperMessage(DeveloperContent{
				Instructions: "你可以使用以下工具来帮助用户：",
				Tools:        tools,
			}),
			NewUserMessage(userPrompt),
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := client.Chat(ctx, conv, &ChatOptions{
		Temperature: 0.7,
		MaxTokens:   1000,
	})

	if err != nil {
		log.Printf("❌ 工具调用失败: %v", err)
		return
	}

	fmt.Printf("✅ 助手回复:\n%s\n", getContentString(resp.Content))
}
