package harmony

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Role 定义消息角色
type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleDeveloper Role = "developer"
)

// Channel 定义输出通道
type Channel string

const (
	ChannelAnalysis   Channel = "analysis"
	ChannelCommentary Channel = "commentary"
	ChannelFinal      Channel = "final"
)

// Message 表示 Harmony 格式的消息
type Message struct {
	Role    Role        `json:"role"`
	Content interface{} `json:"content"`
}

// SystemContent 系统消息内容
type SystemContent struct {
	Instructions  string             `json:"instructions,omitempty"`
	Knowledge     string             `json:"knowledge,omitempty"`
	CurrentDate   string             `json:"current_date,omitempty"`
	Reasoning     string             `json:"reasoning,omitempty"`
	ValidChannels []Channel          `json:"valid_channels,omitempty"`
	ToolChannels  map[string]Channel `json:"tool_channels,omitempty"`
}

// DeveloperContent 开发者消息内容
type DeveloperContent struct {
	Instructions string                 `json:"instructions,omitempty"`
	Tools        map[string]interface{} `json:"tools,omitempty"`
}

// UserContent 用户消息内容
type UserContent struct {
	Text string `json:"text"`
}

// AssistantContent 助手消息内容
type AssistantContent struct {
	Channel Channel `json:"channel"`
	Content string  `json:"content"`
}

// Conversation 表示完整的对话
type Conversation struct {
	Messages []Message `json:"messages"`
}

// HarmonyEncoder Harmony 格式编码器
type HarmonyEncoder struct{}

// NewHarmonyEncoder 创建新的 Harmony 编码器
func NewHarmonyEncoder() *HarmonyEncoder {
	return &HarmonyEncoder{}
}

// EncodeMessage 编码单个消息
func (e *HarmonyEncoder) EncodeMessage(msg Message) string {
	var contentStr string

	switch content := msg.Content.(type) {
	case SystemContent:
		contentStr = e.encodeSystemContent(content)
	case DeveloperContent:
		contentStr = e.encodeDeveloperContent(content)
	case UserContent:
		contentStr = content.Text
	case AssistantContent:
		contentStr = fmt.Sprintf("Channel: %s\n%s", content.Channel, content.Content)
	default:
		if jsonBytes, err := json.Marshal(content); err == nil {
			contentStr = string(jsonBytes)
		} else {
			contentStr = fmt.Sprintf("%v", content)
		}
	}

	return fmt.Sprintf("<|start|>%s<|message|>%s<|end|>", msg.Role, contentStr)
}

// encodeSystemContent 编码系统消息内容
func (e *HarmonyEncoder) encodeSystemContent(content SystemContent) string {
	var parts []string

	if content.Instructions != "" {
		parts = append(parts, content.Instructions)
	}

	if content.Knowledge != "" {
		parts = append(parts, fmt.Sprintf("Knowledge cutoff: %s", content.Knowledge))
	}

	if content.CurrentDate != "" {
		parts = append(parts, fmt.Sprintf("Current date: %s", content.CurrentDate))
	}

	if content.Reasoning != "" {
		parts = append(parts, fmt.Sprintf("Reasoning: %s", content.Reasoning))
	}

	if len(content.ValidChannels) > 0 {
		channels := make([]string, len(content.ValidChannels))
		for i, ch := range content.ValidChannels {
			channels[i] = string(ch)
		}
		parts = append(parts, fmt.Sprintf("# Valid channels: %s. Channel must be included for every message.", strings.Join(channels, ", ")))
	}

	if len(content.ToolChannels) > 0 {
		for tool, channel := range content.ToolChannels {
			parts = append(parts, fmt.Sprintf("Calls to these tools must go to the %s channel: '%s'.", channel, tool))
		}
	}

	return strings.Join(parts, "\n")
}

// encodeDeveloperContent 编码开发者消息内容
func (e *HarmonyEncoder) encodeDeveloperContent(content DeveloperContent) string {
	var parts []string

	if content.Instructions != "" {
		parts = append(parts, fmt.Sprintf("# Instructions\n\n%s", content.Instructions))
	}

	if len(content.Tools) > 0 {
		parts = append(parts, "# Tools\n")
		for namespace, tools := range content.Tools {
			parts = append(parts, fmt.Sprintf("## %s\n", namespace))
			if toolMap, ok := tools.(map[string]interface{}); ok {
				for toolName, toolDef := range toolMap {
					parts = append(parts, fmt.Sprintf("type %s = %v;", toolName, toolDef))
				}
			}
			parts = append(parts, fmt.Sprintf("} // namespace %s", namespace))
		}
	}

	return strings.Join(parts, "\n\n")
}

// EncodeConversation 编码完整对话
func (e *HarmonyEncoder) EncodeConversation(conv Conversation) string {
	var parts []string

	for _, msg := range conv.Messages {
		parts = append(parts, e.EncodeMessage(msg))
	}

	// 添加助手开始标记
	parts = append(parts, "<|start|>assistant<|message|>")

	return strings.Join(parts, "\n")
}

// DecodeMessage 解码消息
func (e *HarmonyEncoder) DecodeMessage(text string) (Message, error) {
	// 简单的解码实现，实际使用时可能需要更复杂的解析
	startIdx := strings.Index(text, "<|start|>")
	endIdx := strings.Index(text, "<|end|>")

	if startIdx == -1 || endIdx == -1 {
		return Message{}, fmt.Errorf("invalid message format")
	}

	roleStart := startIdx + 9 // "<|start|>" 的长度，应该是 9 而不是 8
	roleEnd := strings.Index(text[roleStart:], "<|message|>")
	if roleEnd == -1 {
		return Message{}, fmt.Errorf("invalid message format")
	}

	roleText := text[roleStart : roleStart+roleEnd]
	role := Role(strings.TrimSpace(roleText))
	contentStart := roleStart + roleEnd + 10 // "<|message|>" 的长度，应该是 10
	content := strings.TrimSpace(text[contentStart:endIdx])

	// 如果内容以 '>' 开头，去掉它
	if len(content) > 0 && content[0] == '>' {
		content = content[1:]
	}

	var msgContent interface{}
	switch role {
	case RoleSystem:
		msgContent = e.decodeSystemContent(content)
	case RoleDeveloper:
		msgContent = e.decodeDeveloperContent(content)
	case RoleUser:
		msgContent = UserContent{Text: content}
	case RoleAssistant:
		msgContent = e.decodeAssistantContent(content)
	default:
		msgContent = content
	}

	return Message{
		Role:    role,
		Content: msgContent,
	}, nil
}

// decodeSystemContent 解码系统消息内容
func (e *HarmonyEncoder) decodeSystemContent(content string) SystemContent {
	// 简化实现，实际使用时需要更复杂的解析
	return SystemContent{
		Instructions: content,
	}
}

// decodeDeveloperContent 解码开发者消息内容
func (e *HarmonyEncoder) decodeDeveloperContent(content string) DeveloperContent {
	// 简化实现，实际使用时需要更复杂的解析
	return DeveloperContent{
		Instructions: content,
	}
}

// decodeAssistantContent 解码助手消息内容
func (e *HarmonyEncoder) decodeAssistantContent(content string) AssistantContent {
	// 处理复杂的多通道响应格式
	// 例如: "analysis<|message|>We need to respond politely.<|start|>assistant<|channel|>commentary..."

	// 首先尝试提取最后一个有效的助手消息
	if strings.Contains(content, "<|start|>assistant<|channel|>") {
		// 找到最后一个助手消息的开始位置
		lastAssistantStart := strings.LastIndex(content, "<|start|>assistant<|channel|>")
		if lastAssistantStart != -1 {
			// 提取通道信息
			channelStart := lastAssistantStart + len("<|start|>assistant<|channel|>")
			channelEnd := strings.Index(content[channelStart:], "<|message|>")
			if channelEnd != -1 {
				channel := Channel(strings.TrimSpace(content[channelStart : channelStart+channelEnd]))
				messageStart := channelStart + channelEnd + len("<|message|>")
				messageContent := strings.TrimSpace(content[messageStart:])

				return AssistantContent{
					Channel: channel,
					Content: messageContent,
				}
			}
		}
	}

	// 如果没有找到复杂的格式，尝试简单的 Channel: 格式
	lines := strings.Split(content, "\n")
	if len(lines) > 0 && strings.HasPrefix(lines[0], "Channel: ") {
		channel := Channel(strings.TrimPrefix(lines[0], "Channel: "))
		contentText := strings.Join(lines[1:], "\n")
		return AssistantContent{
			Channel: channel,
			Content: contentText,
		}
	}

	// 默认返回 final 通道和原始内容
	return AssistantContent{
		Channel: ChannelFinal,
		Content: content,
	}
}

// NewSystemMessage 创建系统消息
func NewSystemMessage(content SystemContent) Message {
	return Message{
		Role:    RoleSystem,
		Content: content,
	}
}

// NewDeveloperMessage 创建开发者消息
func NewDeveloperMessage(content DeveloperContent) Message {
	return Message{
		Role:    RoleDeveloper,
		Content: content,
	}
}

// NewUserMessage 创建用户消息
func NewUserMessage(text string) Message {
	return Message{
		Role:    RoleUser,
		Content: UserContent{Text: text},
	}
}

// NewAssistantMessage 创建助手消息
func NewAssistantMessage(channel Channel, content string) Message {
	return Message{
		Role:    RoleAssistant,
		Content: AssistantContent{Channel: channel, Content: content},
	}
}
