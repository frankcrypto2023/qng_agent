package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"qng_agent/internal/config"
	"qng_agent/internal/harmony"
	"strings"
	"time"
)

func main() {
	// 定义命令行参数
	var (
		mode    = flag.String("mode", "basic", "运行模式: basic, custom, tools, stream, web")
		prompt  = flag.String("prompt", "", "自定义提示 (用于 custom 模式)")
		baseURL = flag.String("url", "http://localhost:8081", "llama.cpp 服务器地址")
		port    = flag.String("port", "8080", "Web 服务器端口 (用于 web 模式)")
		help    = flag.Bool("help", false, "显示帮助信息")
	)

	flag.Parse()

	if *help {
		showHelp()
		return
	}

	fmt.Println("🤖 Harmony GPT-OSS Demo Tool")
	fmt.Println(strings.Repeat("=", 50))

	switch *mode {
	case "basic":
		fmt.Println("运行基本 demo...")
		harmony.Demo()

	case "custom":
		if *prompt == "" {
			fmt.Println("❌ 自定义模式需要提供 -prompt 参数")
			os.Exit(1)
		}
		fmt.Printf("运行自定义提示 demo: %s\n", *prompt)
		harmony.DemoWithCustomPrompt(*prompt)

	case "tools":
		fmt.Println("运行工具调用 demo...")
		tools := map[string]interface{}{
			"functions": map[string]interface{}{
				"get_weather": "(_: {location: string, format?: 'celsius' | 'fahrenheit'}) => any",
				"get_time":    "() => any",
				"calculate":   "(_: {expression: string}) => any",
				"search":      "(_: {query: string}) => any",
			},
		}
		harmony.DemoWithTools(tools, "请帮我计算 15 * 23 的结果，并告诉我现在的时间。")

	case "stream":
		fmt.Println("运行流式对话 demo...")
		runStreamDemo(*baseURL)

	case "web":
		fmt.Println("启动 Web 服务器...")
		server := NewWebServer(*baseURL, *port)
		if err := server.Start(); err != nil {
			log.Fatalf("❌ Web 服务器启动失败: %v", err)
		}

	default:
		fmt.Printf("❌ 未知的运行模式: %s\n", *mode)
		showHelp()
		os.Exit(1)
	}
}

func showHelp() {
	fmt.Println("Harmony GPT-OSS Demo Tool")
	fmt.Println()
	fmt.Println("用法:")
	fmt.Println("  harmony_demo [选项]")
	fmt.Println()
	fmt.Println("选项:")
	fmt.Println("  -mode string")
	fmt.Println("        运行模式 (默认: basic)")
	fmt.Println("        basic  - 运行基本 demo")
	fmt.Println("        custom - 运行自定义提示 demo")
	fmt.Println("        tools  - 运行工具调用 demo")
	fmt.Println("        stream - 运行流式对话 demo")
	fmt.Println("        web    - 启动 Web 服务器")
	fmt.Println("  -prompt string")
	fmt.Println("        自定义提示 (用于 custom 模式)")
	fmt.Println("  -url string")
	fmt.Println("        llama.cpp 服务器地址 (默认: http://localhost:8081)")
	fmt.Println("  -port string")
	fmt.Println("        Web 服务器端口 (默认: 8080)")
	fmt.Println("  -help")
	fmt.Println("        显示帮助信息")
	fmt.Println()
	fmt.Println("示例:")
	fmt.Println("  harmony_demo")
	fmt.Println("  harmony_demo -mode custom -prompt '请介绍一下量子计算'")
	fmt.Println("  harmony_demo -mode tools")
	fmt.Println("  harmony_demo -mode stream")
	fmt.Println("  harmony_demo -mode web -port 8080")
	fmt.Println("  harmony_demo -url http://localhost:8080")
}

func runStreamDemo(baseURL string) {
	// 创建默认配置
	defaultConfig := &config.LlamaCppConfig{
		Temperature:      0.8,
		TopP:             0.95,
		TopK:             40,
		MaxTokens:        2000,
		RepeatPenalty:    1.1,
		CachePrompt:      true,
		ReasoningFormat:  "none",
		Samplers:         "edkypmxt",
		DynatempRange:    0,
		DynatempExponent: 1,
		MinP:             0.05,
		TypicalP:         1,
		XtcProbability:   0,
		XtcThreshold:     0.1,
		RepeatLastN:      64,
		PresencePenalty:  0,
		FrequencyPenalty: 0,
		DryMultiplier:    0,
		DryBase:          1.75,
		DryAllowedLength: 2,
		DryPenaltyLastN:  -1,
		TimingsPerToken:  true,
	}

	client := harmony.NewLlamaCppHarmonyClient(baseURL, defaultConfig)

	conv := harmony.Conversation{
		Messages: []harmony.Message{
			harmony.NewSystemMessage(harmony.SystemContent{
				Instructions:  "你是一个有用的AI助手，请用中文回答问题。",
				Knowledge:     "2024-06",
				CurrentDate:   "2025-01-28",
				Reasoning:     "high",
				ValidChannels: []harmony.Channel{harmony.ChannelAnalysis, harmony.ChannelCommentary, harmony.ChannelFinal},
			}),
			harmony.NewUserMessage("请写一个关于机器学习的短文，大约150字。"),
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	stream, err := client.ChatStream(ctx, conv, &harmony.ChatOptions{
		Temperature: 0.7,
		MaxTokens:   400,
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
