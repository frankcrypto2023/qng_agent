package harmony

import (
	"testing"
)

func TestHarmonyEncoder_EncodeMessage(t *testing.T) {
	encoder := NewHarmonyEncoder()

	// 测试系统消息编码
	systemMsg := NewSystemMessage(SystemContent{
		Instructions: "你是一个有用的AI助手。",
		Knowledge:    "2024-06",
		CurrentDate:  "2025-01-28",
		Reasoning:    "high",
	})

	encoded := encoder.EncodeMessage(systemMsg)
	expected := "<|start|>system<|message|>你是一个有用的AI助手。\nKnowledge cutoff: 2024-06\nCurrent date: 2025-01-28\nReasoning: high<|end|>"

	if encoded != expected {
		t.Errorf("系统消息编码错误\n期望: %s\n实际: %s", expected, encoded)
	}

	// 测试用户消息编码
	userMsg := NewUserMessage("你好！")
	encoded = encoder.EncodeMessage(userMsg)
	expected = "<|start|>user<|message|>你好！<|end|>"

	if encoded != expected {
		t.Errorf("用户消息编码错误\n期望: %s\n实际: %s", expected, encoded)
	}
}

func TestHarmonyEncoder_EncodeConversation(t *testing.T) {
	encoder := NewHarmonyEncoder()

	conv := Conversation{
		Messages: []Message{
			NewSystemMessage(SystemContent{
				Instructions: "你是一个有用的AI助手。",
			}),
			NewUserMessage("你好！"),
		},
	}

	encoded := encoder.EncodeConversation(conv)
	expected := "<|start|>system<|message|>你是一个有用的AI助手。<|end|>\n<|start|>user<|message|>你好！<|end|>\n<|start|>assistant<|message|>"

	if encoded != expected {
		t.Errorf("对话编码错误\n期望: %s\n实际: %s", expected, encoded)
	}
}

func TestHarmonyEncoder_DecodeMessage(t *testing.T) {
	encoder := NewHarmonyEncoder()

	// 测试解码系统消息
	encoded := "<|start|>system<|message|>你是一个有用的AI助手。<|end|>"

	msg, err := encoder.DecodeMessage(encoded)

	if err != nil {
		t.Errorf("解码系统消息失败: %v", err)
	}

	if msg.Role != RoleSystem {
		t.Errorf("角色错误，期望: %s, 实际: %s", RoleSystem, msg.Role)
	}

	// 测试解码用户消息
	encoded = "<|start|>user<|message|>你好！<|end|>"

	msg, err = encoder.DecodeMessage(encoded)

	if err != nil {
		t.Errorf("解码用户消息失败: %v", err)
	}

	if msg.Role != RoleUser {
		t.Errorf("角色错误，期望: %s, 实际: %s", RoleUser, msg.Role)
	}

	if content, ok := msg.Content.(UserContent); !ok || content.Text != "你好！" {
		t.Errorf("内容错误，期望: 你好！, 实际: %v", msg.Content)
	}
}

func TestNewLlamaCppHarmonyClient(t *testing.T) {
	client := NewLlamaCppHarmonyClient("http://localhost:8081")

	if client == nil {
		t.Error("客户端创建失败")
	}

	if client.baseURL != "http://localhost:8081" {
		t.Errorf("baseURL 错误，期望: http://localhost:8081, 实际: %s", client.baseURL)
	}

	if client.encoder == nil {
		t.Error("编码器未初始化")
	}
}

func TestGetContentString(t *testing.T) {
	// 测试助手消息
	assistantMsg := NewAssistantMessage(ChannelFinal, "这是一个测试回复")
	content := getContentString(assistantMsg)
	expected := "这是一个测试回复"

	if content != expected {
		t.Errorf("助手消息内容错误\n期望: %s\n实际: %s", expected, content)
	}

	// 测试用户消息
	userMsg := NewUserMessage("用户输入")
	content = getContentString(userMsg)
	expected = "用户输入"

	if content != expected {
		t.Errorf("用户消息内容错误\n期望: %s\n实际: %s", expected, content)
	}
}

func TestChannelConstants(t *testing.T) {
	// 测试通道常量
	if ChannelAnalysis != "analysis" {
		t.Errorf("ChannelAnalysis 错误，期望: analysis, 实际: %s", ChannelAnalysis)
	}

	if ChannelCommentary != "commentary" {
		t.Errorf("ChannelCommentary 错误，期望: commentary, 实际: %s", ChannelCommentary)
	}

	if ChannelFinal != "final" {
		t.Errorf("ChannelFinal 错误，期望: final, 实际: %s", ChannelFinal)
	}
}

func TestRoleConstants(t *testing.T) {
	// 测试角色常量
	if RoleSystem != "system" {
		t.Errorf("RoleSystem 错误，期望: system, 实际: %s", RoleSystem)
	}

	if RoleUser != "user" {
		t.Errorf("RoleUser 错误，期望: user, 实际: %s", RoleUser)
	}

	if RoleAssistant != "assistant" {
		t.Errorf("RoleAssistant 错误，期望: assistant, 实际: %s", RoleAssistant)
	}

	if RoleDeveloper != "developer" {
		t.Errorf("RoleDeveloper 错误，期望: developer, 实际: %s", RoleDeveloper)
	}
}

func TestSystemContentWithChannels(t *testing.T) {
	encoder := NewHarmonyEncoder()

	systemMsg := NewSystemMessage(SystemContent{
		Instructions:  "你是一个有用的AI助手。",
		ValidChannels: []Channel{ChannelAnalysis, ChannelCommentary, ChannelFinal},
		ToolChannels: map[string]Channel{
			"functions": ChannelCommentary,
		},
	})

	encoded := encoder.EncodeMessage(systemMsg)

	// 检查是否包含通道信息
	if !contains(encoded, "Valid channels: analysis, commentary, final") {
		t.Error("编码结果中缺少有效通道信息")
	}

	if !contains(encoded, "Calls to these tools must go to the commentary channel: 'functions'") {
		t.Error("编码结果中缺少工具通道信息")
	}
}

func TestDeveloperContentWithTools(t *testing.T) {
	encoder := NewHarmonyEncoder()

	tools := map[string]interface{}{
		"functions": map[string]interface{}{
			"get_weather": "(_: {location: string}) => any",
			"get_time":    "() => any",
		},
	}

	devMsg := NewDeveloperMessage(DeveloperContent{
		Instructions: "你可以使用以下工具：",
		Tools:        tools,
	})

	encoded := encoder.EncodeMessage(devMsg)

	// 检查是否包含工具信息
	if !contains(encoded, "# Tools") {
		t.Error("编码结果中缺少工具标题")
	}

	if !contains(encoded, "## functions") {
		t.Error("编码结果中缺少函数命名空间")
	}

	if !contains(encoded, "type get_weather = (_: {location: string}) => any;") {
		t.Error("编码结果中缺少工具定义")
	}
}

func TestDecodeComplexAssistantContent(t *testing.T) {
	encoder := NewHarmonyEncoder()
	
	// 测试复杂的多通道响应格式
	complexContent := `analysis<|message|>We need to respond politely.<|start|>assistant<|channel|>commentary<|message|>中文回复.<|start|>assistant<|channel|>final<|message|>你好！有什么可以帮到你的吗？`
	
	assistantMsg, err := encoder.DecodeMessage("<|start|>assistant<|message|>" + complexContent + "<|end|>")
	if err != nil {
		t.Errorf("解码复杂内容失败: %v", err)
	}
	
	if assistantMsg.Role != RoleAssistant {
		t.Errorf("角色错误，期望: %s, 实际: %s", RoleAssistant, assistantMsg.Role)
	}
	
	if content, ok := assistantMsg.Content.(AssistantContent); ok {
		if content.Channel != ChannelFinal {
			t.Errorf("通道错误，期望: %s, 实际: %s", ChannelFinal, content.Channel)
		}
		
		expectedContent := "你好！有什么可以帮到你的吗？"
		if content.Content != expectedContent {
			t.Errorf("内容错误\n期望: %s\n实际: %s", expectedContent, content.Content)
		}
	} else {
		t.Error("内容类型错误，期望 AssistantContent")
	}
}

// 辅助函数
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) &&
		(s[:len(substr)] == substr || s[len(s)-len(substr):] == substr ||
			func() bool {
				for i := 1; i <= len(s)-len(substr); i++ {
					if s[i:i+len(substr)] == substr {
						return true
					}
				}
				return false
			}()))
}
