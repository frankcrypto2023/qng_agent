# Harmony GPT-OSS 模块

这个模块实现了基于 OpenAI Harmony 格式的 GPT-OSS 模型调用，通过 llama.cpp 服务器与 gpt-oss 模型进行交互。

## 功能特性

- ✅ **Harmony 格式支持**: 完整的 OpenAI Harmony 响应格式实现
- ✅ **多通道输出**: 支持 analysis、commentary、final 等输出通道
- ✅ **工具调用**: 支持函数调用和工具定义
- ✅ **流式响应**: 支持实时流式对话
- ✅ **llama.cpp 集成**: 与 llama.cpp 服务器无缝集成
- ✅ **中文支持**: 完整的中文对话支持

## 安装和配置

### 1. 安装 llama.cpp

首先需要安装并配置 llama.cpp 服务器：

```bash
# 克隆 llama.cpp 仓库
git clone https://github.com/ggerganov/llama.cpp.git
cd llama.cpp

# 编译
make

# 下载 gpt-oss 模型 (需要自行获取模型文件)
# 启动服务器
./server -m models/gpt-oss.gguf -p 8081
```

### 2. 使用 Harmony 模块

```go
package main

import (
    "context"
    "fmt"
    "qng_agent/internal/harmony"
)

func main() {
    // 创建客户端
    client := harmony.NewLlamaCppHarmonyClient("http://localhost:8081")
    
    // 创建对话
    conv := harmony.Conversation{
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
    
    // 发送请求
    ctx := context.Background()
    resp, err := client.Chat(ctx, conv, &harmony.ChatOptions{
        Temperature: 0.7,
        MaxTokens:   500,
    })
    
    if err != nil {
        panic(err)
    }
    
    fmt.Printf("助手回复: %s\n", harmony.GetContentString(resp.Content))
}
```

## 命令行工具

项目提供了命令行工具来快速测试 Harmony 功能：

```bash
# 编译
go build -o harmony_demo cmd/harmony_demo/main.go

# 运行基本 demo
./harmony_demo

# 运行自定义提示 demo
./harmony_demo -mode custom -prompt "请介绍一下量子计算"

# 运行工具调用 demo
./harmony_demo -mode tools

# 运行流式对话 demo
./harmony_demo -mode stream

# 指定不同的服务器地址
./harmony_demo -url http://localhost:8080
```

## API 参考

### 核心类型

#### Message
```go
type Message struct {
    Role    Role        `json:"role"`
    Content interface{} `json:"content"`
}
```

#### Conversation
```go
type Conversation struct {
    Messages []Message `json:"messages"`
}
```

#### SystemContent
```go
type SystemContent struct {
    Instructions   string            `json:"instructions,omitempty"`
    Knowledge      string            `json:"knowledge,omitempty"`
    CurrentDate    string            `json:"current_date,omitempty"`
    Reasoning      string            `json:"reasoning,omitempty"`
    ValidChannels  []Channel         `json:"valid_channels,omitempty"`
    ToolChannels   map[string]Channel `json:"tool_channels,omitempty"`
}
```

#### DeveloperContent
```go
type DeveloperContent struct {
    Instructions string                 `json:"instructions,omitempty"`
    Tools        map[string]interface{} `json:"tools,omitempty"`
}
```

### 主要函数

#### NewLlamaCppHarmonyClient
```go
func NewLlamaCppHarmonyClient(baseURL string) *LlamaCppHarmonyClient
```
创建新的 Harmony llama.cpp 客户端。

#### Chat
```go
func (c *LlamaCppHarmonyClient) Chat(ctx context.Context, conv Conversation, options *ChatOptions) (*ChatResponse, error)
```
发送普通对话请求。

#### ChatStream
```go
func (c *LlamaCppHarmonyClient) ChatStream(ctx context.Context, conv Conversation, options *ChatOptions) (<-chan *ChatStreamResponse, error)
```
发送流式对话请求。

### 工具调用示例

```go
// 定义工具
tools := map[string]interface{}{
    "functions": map[string]interface{}{
        "get_weather": "(_: {location: string, format?: 'celsius' | 'fahrenheit'}) => any",
        "get_time":    "() => any",
        "calculate":   "(_: {expression: string}) => any",
    },
}

// 创建带工具的对话
conv := harmony.Conversation{
    Messages: []harmony.Message{
        harmony.NewSystemMessage(harmony.SystemContent{
            Instructions: "你是一个有用的AI助手，可以使用工具来帮助用户。",
            Knowledge:    "2024-06",
            CurrentDate:  "2025-01-28",
            Reasoning:    "high",
            ValidChannels: []harmony.Channel{harmony.ChannelAnalysis, harmony.ChannelCommentary, harmony.ChannelFinal},
            ToolChannels: map[string]harmony.Channel{
                "functions": harmony.ChannelCommentary,
            },
        }),
        harmony.NewDeveloperMessage(harmony.DeveloperContent{
            Instructions: "你可以使用以下工具来帮助用户：",
            Tools:        tools,
        }),
        harmony.NewUserMessage("请告诉我北京的天气怎么样？"),
    },
}
```

## Harmony 格式说明

Harmony 格式是 OpenAI 为 gpt-oss 模型设计的特殊响应格式，包含以下特点：

### 消息结构
```
<|start|>role<|message|>content<|end|>
```

### 支持的角色
- `system`: 系统消息
- `user`: 用户消息
- `assistant`: 助手消息
- `developer`: 开发者消息

### 输出通道
- `analysis`: 分析通道
- `commentary`: 评论通道
- `final`: 最终输出通道

### 工具调用
工具调用通过 `developer` 角色定义，并在 `system` 消息中指定工具应该输出到哪个通道。

## 注意事项

1. **模型要求**: 需要 gpt-oss 模型才能正确使用 Harmony 格式
2. **服务器配置**: 确保 llama.cpp 服务器正确启动并加载了 gpt-oss 模型
3. **网络连接**: 确保客户端能够连接到 llama.cpp 服务器
4. **内存要求**: gpt-oss 模型较大，确保有足够的内存

## 故障排除

### 常见问题

1. **连接失败**
   ```
   错误: failed to send request: dial tcp localhost:8081: connect: connection refused
   ```
   解决: 检查 llama.cpp 服务器是否正在运行

2. **模型加载失败**
   ```
   错误: model not found
   ```
   解决: 确保模型文件路径正确且文件存在

3. **格式错误**
   ```
   错误: invalid message format
   ```
   解决: 检查 Harmony 格式是否正确

### 调试模式

启用详细日志：
```go
log.SetLevel(log.DebugLevel)
```

## 贡献

欢迎提交 Issue 和 Pull Request 来改进这个模块。

## 许可证

本项目采用 MIT 许可证。
