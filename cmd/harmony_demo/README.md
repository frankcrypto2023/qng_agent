# Harmony GPT-OSS Web 服务器

这是一个基于 OpenAI Harmony 格式的 Web 对话界面，集成了 llama.cpp 服务器，提供美观的聊天界面。

## 功能特性

- 🎨 **美观的 Web 界面**: 现代化的聊天界面设计
- 🤖 **多模式支持**: 基本对话、工具调用、流式对话
- ⚡ **实时流式响应**: 支持 Server-Sent Events (SSE) 流式输出
- 🔧 **工具调用**: 内置多种工具函数
- 📱 **响应式设计**: 支持桌面和移动设备
- 🌐 **中文界面**: 完整的中文用户界面

## 快速开始

### 1. 启动 llama.cpp 服务器

首先需要启动 llama.cpp 服务器：

```bash
# 克隆并编译 llama.cpp
git clone https://github.com/ggml-org/llama.cpp.git
cd llama.cpp
make

# 启动服务器 (需要 gpt-oss 模型)
./server -m models/gpt-oss.gguf -p 8081
```

### 2. 启动 Harmony Web 服务器

```bash
# 编译并启动
go build -o harmony_demo cmd/harmony_demo/main.go cmd/harmony_demo/web_server.go
./harmony_demo -mode web -port 8080

# 或者使用启动脚本
./scripts/start_harmony_web.sh
```

### 3. 访问 Web 界面

打开浏览器访问: http://localhost:8080

## 使用说明

### 对话模式

1. **基本对话**: 普通的 AI 对话模式
2. **工具调用**: 支持函数调用的智能对话
3. **流式对话**: 实时流式响应的对话模式

### 界面功能

- **模式选择**: 在输入框上方选择对话模式
- **消息输入**: 在底部输入框输入消息
- **发送消息**: 点击发送按钮或按回车键
- **实时响应**: 流式模式下可以看到实时输出
- **Token 统计**: 显示每次对话的 Token 使用情况

## API 接口

### POST /chat

普通聊天接口

**请求体:**
```json
{
    "message": "你好！",
    "mode": "basic"
}
```

**响应:**
```json
{
    "response": "你好！我是基于 Harmony 格式的 AI 助手...",
    "usage": {
        "prompt_tokens": 150,
        "completion_tokens": 50,
        "total_tokens": 200
    }
}
```

### POST /stream

流式聊天接口 (Server-Sent Events)

**请求体:**
```json
{
    "message": "请介绍一下人工智能",
    "mode": "stream"
}
```

**响应格式:**
```
data: {"response": "人工智能是..."}

data: {"response": "人工智能是计算机科学的一个分支..."}

data: [DONE]
```

## 配置选项

### 命令行参数

- `-mode web`: 启动 Web 服务器模式
- `-port 8080`: 设置 Web 服务器端口 (默认: 8080)
- `-url http://localhost:8081`: 设置 llama.cpp 服务器地址

### 环境变量

- `HARMONY_PORT`: Web 服务器端口
- `LLAMA_URL`: llama.cpp 服务器地址

## 开发说明

### 项目结构

```
cmd/harmony_demo/
├── main.go          # 主程序入口
├── web_server.go    # Web 服务器实现
└── README.md        # 说明文档
```

### 核心组件

- **WebServer**: Web 服务器主类
- **handleHome**: 主页处理函数
- **handleChat**: 聊天接口处理
- **handleStream**: 流式聊天接口处理
- **createConversation**: 对话创建函数

### 前端特性

- **响应式设计**: 使用 CSS Flexbox 和 Grid
- **实时更新**: 使用 Server-Sent Events
- **错误处理**: 完整的错误提示和处理
- **加载状态**: 优雅的加载动画

## 故障排除

### 常见问题

1. **连接失败**
   ```
   错误: 无法连接到 llama.cpp 服务器
   ```
   解决: 检查 llama.cpp 服务器是否正在运行

2. **端口占用**
   ```
   错误: 端口 8080 已被占用
   ```
   解决: 使用 `-port` 参数指定其他端口

3. **模型加载失败**
   ```
   错误: 模型文件不存在
   ```
   解决: 确保 gpt-oss 模型文件路径正确

### 调试模式

启用详细日志：
```bash
export HARMONY_DEBUG=1
./harmony_demo -mode web
```

## 部署说明

### Docker 部署

```dockerfile
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o harmony_demo cmd/harmony_demo/main.go cmd/harmony_demo/web_server.go

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/harmony_demo .
EXPOSE 8080
CMD ["./harmony_demo", "-mode", "web", "-port", "8080"]
```

### 生产环境

1. **反向代理**: 使用 Nginx 或 Apache 作为反向代理
2. **SSL 证书**: 配置 HTTPS 支持
3. **负载均衡**: 使用多个实例进行负载均衡
4. **监控**: 添加健康检查和监控

## 贡献

欢迎提交 Issue 和 Pull Request 来改进这个 Web 界面。

## 许可证

本项目采用 MIT 许可证。
