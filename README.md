# QNG Intelligent Agent

A full-stack conversational AI interface powered by a dynamic, graph-based workflow engine for Web3 and blockchain operations.

## 🚀 Features

- **ChatGPT-style Interface**: Clean, modern UI for natural conversation
- **LLM Graph Workflow Engine**: Dynamic intent analysis and execution
- **Session Management**: Persistent chat history with context awareness  
- **Streaming Responses**: Real-time character-by-character response streaming
- **MCP Tool Integration**: Support for Model Context Protocol tools
- **Web3 Workflows**: Smart contract interactions and blockchain operations
- **Settings Panel**: Configurable LLM providers and MCP servers

## 🏗️ Architecture

### Frontend (React + TypeScript)
- Modern React with TypeScript and Tailwind CSS
- Real-time streaming chat interface
- Session management and history
- Settings configuration for LLM and MCP servers
- Blockchain-themed design elements

### Backend (Golang)
- Gin HTTP framework with SQLite storage
- Channel-based session management (harmony-style)
- LLM Graph workflow engine using QNG graph library
- Server-Sent Events (SSE) for response streaming
- Intent analysis and dynamic workflow routing

### Workflow Engine
The core LLM Graph uses **官方 QNG graph 库**直接从 `github.com/Qitmeer/qng/blob/dev/2.1/graph/graph.go` 并处理每个用户消息：

1. **Intent Analysis**: 使用 LLM 理解用户请求
2. **Tool Routing**: 通过条件边路由到 MCP 工具或 Web3 工作流  
3. **Execution**: 执行工具/工作流，支持用户确认
4. **Response Generation**: 生成自然语言响应

#### 真正的 QNG Graph 集成
- 使用官方 QNG graph API: `graph.NewMessageGraph()`, `AddNode()`, `AddConditionalEdge()`
- 基于消息的图执行使用 `llms.MessageContent` (LangChain 标准)
- 支持动态工作流创建和条件路由执行
- 完全兼容 QNG 生态系统架构

## 📋 Supported Scenarios

### 1. MCP Tool Queries
```
User: "What is the stateroot for order=10000 on node http://127.0.0.1:8545?"
→ Extracts parameters → Calls stateroot tool → Returns formatted result
```

### 2. Comparative Analysis  
```
User: "Compare stateroot for order=10000 between http://127.0.0.1:8545 and http://127.0.0.2:8545"
→ Parallel tool execution → Comparison analysis → Difference report
```

### 3. Web3 Workflows
```
User: "I want to swap 1 MEER for USDT using Metamask"
→ Loads token swap workflow → Wallet connection → Balance check → Transaction signing
```

## 🛠️ Setup

### Prerequisites
- Node.js 18+ 
- Go 1.21+
- Git

### Quick Start

1. **Clone and setup**:
```bash
git clone <repository>
cd qng_agent
./setup.sh
```

2. **Start development servers**:
```bash
./scripts/start.sh
```

3. **Access the application**:
- Frontend: http://localhost:3000
- Backend API: http://localhost:8080
- Health check: http://localhost:8080/api/health

### Manual Setup

**Frontend**:
```bash
cd frontend
npm install
npm run dev
```

**Backend**:
```bash
go mod tidy
go build -o bin/qng-agent cmd/main.go
./bin/qng-agent
```

## ⚙️ Configuration

### Environment Variables
- `PORT`: Server port (default: 8080)
- `HOST`: Server host (default: 0.0.0.0)  
- `DB_PATH`: SQLite database path (default: ./data/qng_agent.db)
- `CONFIG_FILE`: JSON config file path (optional)

### LLM Provider Setup
Configure in the Settings panel:
- **Provider Name**: OpenAI, Anthropic, etc.
- **API URL**: https://api.openai.com/v1
- **Token**: Your API key
- **Model**: gpt-4, claude-3-opus, etc.

### MCP Servers
Add MCP servers in Settings:
- **Name**: Descriptive name
- **URL**: SSE endpoint URL
- **Enabled**: Toggle on/off

## 🔌 API Reference

### Sessions
- `POST /api/sessions` - Create new session
- `GET /api/sessions` - List all sessions  
- `GET /api/sessions/:id` - Get session details
- `DELETE /api/sessions/:id` - Delete session

### Chat
- `POST /api/chat/stream` - Send message with streaming response

### Settings  
- `GET /api/settings` - Get current settings
- `PUT /api/settings` - Update settings

## 📊 Workflow Configuration

Web3 workflows are defined in JSON format:

```json
{
  "name": "Token Swap Workflow",
  "description": "Swap tokens using smart contracts",
  "nodes": [
    {
      "id": "wallet_connect",
      "type": "wallet",
      "name": "Connect Wallet"
    },
    {
      "id": "balance_check", 
      "type": "contract_read",
      "name": "Check Balance"
    },
    {
      "id": "swap_execute",
      "type": "contract_write", 
      "name": "Execute Swap"
    }
  ],
  "edges": [
    {"source": "wallet_connect", "target": "balance_check"},
    {"source": "balance_check", "target": "swap_execute"}
  ]
}
```

## 🧪 Development

### Project Structure
```
qng_agent/
├── frontend/          # React frontend
│   ├── src/
│   │   ├── components/    # React components
│   │   ├── api/          # API client
│   │   └── types.ts      # TypeScript types
├── cmd/               # Go main entry point
├── internal/          # Go backend code
│   ├── config/           # Configuration
│   ├── handlers/         # HTTP handlers
│   ├── session/          # Session management
│   ├── storage/          # Database layer
│   ├── llm/             # LLM client
│   ├── graph/           # Workflow engine (使用官方 QNG graph)
│   └── types/           # Go types
├── scripts/           # Utility scripts
└── data/             # SQLite database
```

### 官方 QNG Graph 工作流架构

工作流引擎使用真正的 QNG graph 执行：

```go
// 使用官方 QNG graph API
messageGraph := graph.NewMessageGraph()
messageGraph.AddNode("intent_analysis", intentAnalysisNode)
messageGraph.AddNode("mcp_tool_execution", mcpToolExecutionNode)
messageGraph.AddNode("web3_workflow_execution", web3WorkflowExecutionNode)
messageGraph.AddNode("response_generation", responseGenerationNode)

// 添加条件路由
messageGraph.AddConditionalEdge("intent_analysis", routeAfterIntent)
messageGraph.AddEdge("mcp_tool_execution", "response_generation")
messageGraph.AddEdge("web3_workflow_execution", "response_generation")

// 使用 llms.MessageContent 执行
runnable, _ := messageGraph.Compile()
result, _ := runnable.Invoke(ctx, initialMessages)
```

### Adding New MCP Tools
1. Update intent analysis in `internal/graph/manager.go`
2. Add tool execution logic in `MCPToolNode`
3. Configure tool parameters and response handling

### Adding New Web3 Workflows  
1. Define workflow JSON configuration
2. Add workflow loading in `Web3WorkflowNode`
3. Implement workflow-specific execution logic

## 🚨 Security Notes

- All LLM API calls are proxied through the backend
- API keys are stored securely and not exposed to frontend
- Database includes proper foreign key constraints
- CORS is configured for development (update for production)

## 📝 License

[Add your license here]

## 🤝 Contributing

[Add contribution guidelines here]