package main

import (
	"context"
	"fmt"
	"log"
	"qng_agent/internal/harmony"
	"strings"
)

func main() {
	fmt.Println("🤖 Harmony GPT-OSS 示例")
	fmt.Println(strings.Repeat("=", 50))

	// 创建客户端
	client := harmony.NewLlamaCppHarmonyClient("http://localhost:8081")

	// 示例 1: 基本对话
	fmt.Println("\n📝 示例 1: 基本对话")
	fmt.Println(strings.Repeat("-", 30))

	conv1 := harmony.Conversation{
		Messages: []harmony.Message{
			harmony.NewSystemMessage(harmony.SystemContent{
				Instructions: "你是一个有用的AI助手，请用中文回答问题。",
				Knowledge:    "2024-06",
				CurrentDate:  "2025-01-28",
				Reasoning:    "high",
				ValidChannels: []harmony.Channel{
					harmony.ChannelAnalysis,
					harmony.ChannelCommentary,
					harmony.ChannelFinal,
				},
			}),
			harmony.NewUserMessage("你好！请介绍一下你自己。"),
		},
	}

	ctx := context.Background()
	resp1, err := client.Chat(ctx, conv1, &harmony.ChatOptions{
		Temperature: 0.7,
		MaxTokens:   300,
	})

	if err != nil {
		log.Printf("❌ 对话失败: %v", err)
	} else {
		fmt.Printf("✅ 助手回复:\n%s\n", getContentString(resp1.Content))
	}

	// 示例 2: 工具调用
	fmt.Println("\n🔧 示例 2: 工具调用")
	fmt.Println(strings.Repeat("-", 30))

	tools := map[string]interface{}{
		"functions": map[string]interface{}{
			"get_weather": "(_: {location: string, format?: 'celsius' | 'fahrenheit'}) => any",
			"get_time":    "() => any",
			"calculate":   "(_: {expression: string}) => any",
		},
	}

	conv2 := harmony.Conversation{
		Messages: []harmony.Message{
			harmony.NewSystemMessage(harmony.SystemContent{
				Instructions: "你是一个有用的AI助手，可以使用工具来帮助用户。",
				Knowledge:    "2024-06",
				CurrentDate:  "2025-01-28",
				Reasoning:    "high",
				ValidChannels: []harmony.Channel{
					harmony.ChannelAnalysis,
					harmony.ChannelCommentary,
					harmony.ChannelFinal,
				},
				ToolChannels: map[string]harmony.Channel{
					"functions": harmony.ChannelCommentary,
				},
			}),
			harmony.NewDeveloperMessage(harmony.DeveloperContent{
				Instructions: "你可以使用以下工具来帮助用户：",
				Tools:        tools,
			}),
			harmony.NewUserMessage("请帮我计算 15 * 23 的结果。"),
		},
	}

	resp2, err := client.Chat(ctx, conv2, &harmony.ChatOptions{
		Temperature: 0.7,
		MaxTokens:   500,
	})

	if err != nil {
		log.Printf("❌ 工具调用失败: %v", err)
	} else {
		fmt.Printf("✅ 助手回复:\n%s\n", getContentString(resp2.Content))
	}

	// 示例 3: 多通道输出
	fmt.Println("\n📡 示例 3: 多通道输出")
	fmt.Println(strings.Repeat("-", 30))

	conv3 := harmony.Conversation{
		Messages: []harmony.Message{
			harmony.NewSystemMessage(harmony.SystemContent{
				Instructions: "你是一个有用的AI助手，请使用多个通道来组织你的回答。",
				Knowledge:    "2024-06",
				CurrentDate:  "2025-01-28",
				Reasoning:    "high",
				ValidChannels: []harmony.Channel{
					harmony.ChannelAnalysis,
					harmony.ChannelCommentary,
					harmony.ChannelFinal,
				},
			}),
			harmony.NewUserMessage("请分析一下区块链技术的优缺点。"),
		},
	}

	resp3, err := client.Chat(ctx, conv3, &harmony.ChatOptions{
		Temperature: 0.7,
		MaxTokens:   800,
	})

	if err != nil {
		log.Printf("❌ 多通道输出失败: %v", err)
	} else {
		fmt.Printf("✅ 助手回复:\n%s\n", getContentString(resp3.Content))
	}

	fmt.Println("\n✅ 所有示例完成！")
}

// getContentString 获取消息内容的字符串表示
func getContentString(msg harmony.Message) string {
	switch content := msg.Content.(type) {
	case harmony.AssistantContent:
		// 只返回纯文本内容，不包含格式标记
		return content.Content
	case harmony.UserContent:
		return content.Text
	case harmony.SystemContent:
		return content.Instructions
	case harmony.DeveloperContent:
		return content.Instructions
	default:
		return fmt.Sprintf("%v", content)
	}
}
