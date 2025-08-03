# ModelScope LLM 集成使用说明

## 概述

本项目已集成 ModelScope API 支持，可以使用 ModelScope 提供的各种大语言模型。

## 配置

### 1. 配置文件设置

在 `config/config.yaml` 中添加 ModelScope 配置：

```yaml
llm:
  provider: modelscope  # 设置为 modelscope
  modelscope:
    api_key: "your-modelscope-api-key"
    model: "qwen/Qwen2.5-7B-Instruct"
    base_url: "https://api.modelscope.cn/v1"
    timeout: 30
    max_tokens: 2000
```

### 2. 支持的模型

ModelScope 支持多种模型，包括但不限于：

- `qwen/Qwen2.5-7B-Instruct` - 通义千问 2.5 7B 指令模型
- `qwen/Qwen2.5-14B-Instruct` - 通义千问 2.5 14B 指令模型
- `qwen/Qwen2.5-72B-Instruct` - 通义千问 2.5 72B 指令模型
- `qwen/Qwen2.5-VL-72B-Instruct` - 通义千问 2.5 72B 多模态模型
- `baichuan-inc/Baichuan2-7B-Chat` - 百川 2 7B 对话模型
- `THUDM/chatglm3-6b` - ChatGLM3 6B 模型

### 3. API Key 获取

1. 访问 [ModelScope](https://www.modelscope.cn/)
2. 注册并登录账号
3. 在个人中心获取 API Key
4. 将 API Key 配置到配置文件中

## 使用方法

### 1. 代码中使用

```go
import (
    "context"
    "qng_agent/internal/config"
    "qng_agent/internal/llm"
)

// 创建配置
config := config.LLMConfig{
    Provider: "modelscope",
    ModelScope: config.ModelScopeConfig{
        APIKey:    "your-api-key",
        Model:     "qwen/Qwen2.5-7B-Instruct",
        BaseURL:   "https://api.modelscope.cn/v1",
        Timeout:   30,
        MaxTokens: 2000,
    },
}

// 创建客户端
client, err := llm.NewClient(config)
if err != nil {
    // 处理错误
}

// 发送消息
ctx := context.Background()
messages := []llm.Message{
    {
        Role:    "user",
        Content: "你好，请介绍一下自己",
    },
}

response, err := client.Chat(ctx, messages)
if err != nil {
    // 处理错误
}

fmt.Println("响应:", response)
```

### 2. 在 LangGraph 中使用

ModelScope 客户端已集成到 LangGraph 系统中，可以在任务分解节点中使用：

```go
// 在 TaskDecomposerNode 中会自动使用配置的 ModelScope 客户端
// 无需额外配置，系统会根据配置文件自动选择
```

## 特性

### 1. 自动降级

- 如果没有配置 API Key，系统会自动使用 MockClient
- 如果 API 调用失败，会返回详细的错误信息

### 2. 参数配置

- `api_key`: ModelScope API 密钥
- `model`: 要使用的模型名称
- `base_url`: API 基础 URL（默认为 https://api.modelscope.cn/v1）
- `timeout`: 请求超时时间（秒）
- `max_tokens`: 最大生成 token 数

### 3. 错误处理

- HTTP 状态码检查
- API 错误响应解析
- 网络超时处理
- 响应内容验证

## 注意事项

1. **API 限制**: 注意 ModelScope API 的调用频率和 token 限制
2. **模型选择**: 根据任务需求选择合适的模型
3. **网络环境**: 确保能够访问 ModelScope API
4. **成本控制**: 不同模型的调用成本不同，请合理选择

## 故障排除

### 常见问题

1. **API Key 无效**
   - 检查 API Key 是否正确
   - 确认 API Key 是否已激活

2. **网络连接问题**
   - 检查网络连接
   - 确认防火墙设置

3. **模型不可用**
   - 检查模型名称是否正确
   - 确认模型是否在可用列表中

4. **超时错误**
   - 增加 timeout 配置值
   - 检查网络延迟

### 调试方法

1. 启用详细日志
2. 检查 HTTP 响应状态码
3. 查看 API 错误信息
4. 测试网络连接

## 更新日志

- v1.0.0: 初始版本，支持基本的 ModelScope API 调用
- 支持多种模型
- 集成到 LangGraph 系统
- 自动降级到 MockClient 