package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"qng_agent/internal/config"
	"qng_agent/internal/llm"
	"qng_agent/internal/mcp"
	"strings"
	"time"
)

type Manager struct {
	mcpManager   *mcp.Manager
	llmClient    llm.Client
	sessionStore SessionStore
}

type Session struct {
	ID           string
	Messages     []Message
	CurrentState string
	CreatedAt    time.Time
}

type Message struct {
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
}

type ProcessRequest struct {
	SessionID string `json:"session_id"`
	Message   string `json:"message"`
}

type ProcessResponse struct {
	Response   string `json:"response"`
	NeedAction bool   `json:"need_action"`
	ActionType string `json:"action_type,omitempty"`
	ActionData any    `json:"action_data,omitempty"`
	WorkflowID string `json:"workflow_id,omitempty"`
}

func NewManager(mcpConfig config.MCPConfig, llmConfig config.LLMConfig) *Manager {
	llmClient, err := llm.NewClient(llmConfig)
	if err != nil {
		log.Fatal("Failed to create LLM client:", err)
	}

	mcpManager := mcp.NewManager(mcpConfig)

	// 创建会话存储
	var sessionStore SessionStore
	if dbPath := "data/sessions.db"; dbPath != "" {
		sqliteStore, err := NewSQLiteSessionStore(dbPath)
		if err != nil {
			log.Printf("⚠️  无法创建SQLite会话存储，使用内存存储: %v", err)
			sessionStore = NewMemorySessionStore()
		} else {
			sessionStore = sqliteStore
			log.Printf("✅ SQLite会话存储已初始化: %s", dbPath)
		}
	} else {
		sessionStore = NewMemorySessionStore()
		log.Printf("✅ 使用内存会话存储")
	}

	return &Manager{
		mcpManager:   mcpManager,
		llmClient:    llmClient,
		sessionStore: sessionStore,
	}
}

func (m *Manager) ProcessMessage(ctx context.Context, req ProcessRequest) (*ProcessResponse, error) {
	session := m.getOrCreateSession(req.SessionID)

	// 添加用户消息到会话
	userMsg := Message{
		Role:      "user",
		Content:   req.Message,
		Timestamp: time.Now(),
	}
	session.Messages = append(session.Messages, userMsg)

	// 立即保存会话
	if err := m.sessionStore.SaveSession(ctx, session); err != nil {
		log.Printf("⚠️  保存用户消息失败: %v", err)
	}

	// 通过LLM分析意图
	intent, err := m.analyzeIntent(ctx, req.Message)
	if err != nil {
		return nil, fmt.Errorf("intent analysis failed: %w", err)
	}

	// 如果没有检测到需要工具调用，直接使用LLM回复
	if !intent.NeedsTool {
		response, err := m.llmClient.Chat(ctx, m.buildLLMMessages(session))
		if err != nil {
			return nil, fmt.Errorf("LLM call failed: %w", err)
		}

		// 添加助手回复到会话
		assistantMsg := Message{
			Role:      "assistant",
			Content:   response,
			Timestamp: time.Now(),
		}
		session.Messages = append(session.Messages, assistantMsg)

		return &ProcessResponse{
			Response: response,
		}, nil
	}

	// 调用对应的MCP服务器工具
	log.Printf("🔄 调用MCP服务器: %s, 工具: %s", intent.ServerName, intent.ToolName)
	result, err := m.mcpManager.Call(ctx, intent.ServerName, intent.ToolName, intent.Parameters)
	if err != nil {
		log.Printf("❌ MCP工具调用失败: %v", err)
		return nil, fmt.Errorf("MCP tool call failed: %w", err)
	}

	// 将结果传给LLM格式化
	llmMessages := m.buildLLMMessages(session)
	llmMessages = append(llmMessages, llm.Message{
		Role:    "system",
		Content: fmt.Sprintf("Tool result: %s", result),
	})

	response, err := m.llmClient.Chat(ctx, llmMessages)
	if err != nil {
		return nil, fmt.Errorf("LLM formatting failed: %w", err)
	}

	assistantMsg := Message{
		Role:      "assistant",
		Content:   response,
		Timestamp: time.Now(),
	}
	session.Messages = append(session.Messages, assistantMsg)

	// 立即保存会话
	if err := m.sessionStore.SaveSession(ctx, session); err != nil {
		log.Printf("⚠️  保存助手回复失败: %v", err)
	}

	// 如果是工作流，返回特殊响应
	if intent.ServerName == "qng" && intent.ToolName == "execute_workflow" {
		if resultMap, ok := result.(map[string]any); ok {
			if workflowID, exists := resultMap["workflow_id"]; exists {
				return &ProcessResponse{
					Response:   "任务正在执行中，请等待...",
					NeedAction: true,
					ActionType: "workflow_running",
					WorkflowID: fmt.Sprintf("%v", workflowID),
				}, nil
			}
		}
		// 如果没有 workflow_id，返回错误
		return &ProcessResponse{
			Response: "工作流执行失败：未收到有效的工作流ID",
		}, nil
	}

	return &ProcessResponse{
		Response: response,
	}, nil
}

type Intent struct {
	NeedsTool  bool           `json:"needs_tool"`
	ServerName string         `json:"server_name"`
	ToolName   string         `json:"tool_name"`
	Parameters map[string]any `json:"parameters"`
	Confidence float64        `json:"confidence"`
}

func (m *Manager) analyzeIntent(ctx context.Context, message string) (*Intent, error) {
	// 获取所有可用的MCP服务器详细能力
	capabilities := m.mcpManager.GetDetailedCapabilities()

	// 构建详细的能力描述
	var capabilityDescriptions []string
	for serverName, serverCaps := range capabilities {
		for _, cap := range serverCaps {
			// 构建详细的能力描述
			desc := fmt.Sprintf("🔧 %s.%s: %s", serverName, cap.Name, cap.Description)

			// 添加参数信息
			if len(cap.Parameters) > 0 {
				var paramDescs []string
				for _, param := range cap.Parameters {
					required := ""
					if param.Required {
						required = " (必需)"
					}
					paramDescs = append(paramDescs, fmt.Sprintf("    - %s (%s)%s: %s",
						param.Name, param.Type, required, param.Description))
				}
				desc += "\n" + strings.Join(paramDescs, "\n")
			}

			capabilityDescriptions = append(capabilityDescriptions, desc)
		}
	}

	log.Printf("🔍 MCP能力分析:")
	log.Printf("  - 服务器数量: %d", len(capabilities))
	log.Printf("  - 能力描述: %v", capabilityDescriptions)

	if len(capabilityDescriptions) == 0 {
		log.Printf("⚠️  警告: 没有找到任何MCP能力")
	}
	// 构建LLM提示
	prompt := fmt.Sprintf(`分析用户意图并选择合适的MCP工具。

可用工具:
%s

用户消息: %s

请分析用户意图并返回JSON格式的响应:
{
  "needs_tool": true/false,
  "server_name": "服务器名称",
  "tool_name": "工具名称", 
  "parameters": {"参数": "值"},
  "confidence": 0.95
}

如果不需要工具调用，设置needs_tool为false，其他字段可以为空。`,
		strings.Join(capabilityDescriptions, "\n"),
		message)

	// 调用LLM分析意图
	response, err := m.llmClient.Chat(ctx, []llm.Message{
		{
			Role:    "user",
			Content: "你是一个意图分析助手，负责分析用户消息并选择合适的MCP工具。\n\n" + prompt,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("LLM intent analysis failed: %w", err)
	}
	fmt.Println("response: ", response)
	// 解析LLM响应
	var intent Intent
	if err := json.Unmarshal([]byte(response), &intent); err != nil {
		// 如果解析失败，使用简单的关键词匹配作为后备
		return m.fallbackIntentAnalysis(message), nil
	}

	return &intent, nil
}

func (m *Manager) fallbackIntentAnalysis(message string) *Intent {
	lowerMsg := strings.ToLower(message)

	// 检查是否是工作流相关消息
	workflowKeywords := []string{
		"兑换", "质押", "交易", "swap", "stake",
		"transfer", "转账", "usdt", "btc", "eth",
	}

	for _, keyword := range workflowKeywords {
		if strings.Contains(lowerMsg, keyword) {
			return &Intent{
				NeedsTool:  true,
				ServerName: "qng",
				ToolName:   "execute_workflow",
				Parameters: map[string]any{
					"message": message,
				},
				Confidence: 0.8,
			}
		}
	}

	// 检查MetaMask相关操作
	if strings.Contains(lowerMsg, "钱包") || strings.Contains(lowerMsg, "metamask") ||
		strings.Contains(lowerMsg, "连接") || strings.Contains(lowerMsg, "签名") {
		return &Intent{
			NeedsTool:  true,
			ServerName: "metamask",
			ToolName:   "connect_wallet",
			Parameters: map[string]any{},
			Confidence: 0.8,
		}
	}

	// 检查文件系统操作
	if strings.Contains(lowerMsg, "文件") || strings.Contains(lowerMsg, "读取") ||
		strings.Contains(lowerMsg, "写入") || strings.Contains(lowerMsg, "目录") {
		return &Intent{
			NeedsTool:  true,
			ServerName: "file_system",
			ToolName:   "list_directory",
			Parameters: map[string]any{
				"path": ".",
			},
			Confidence: 0.7,
		}
	}

	return &Intent{
		NeedsTool:  false,
		Confidence: 0.9,
	}
}

func (m *Manager) getOrCreateSession(sessionID string) *Session {
	ctx := context.Background()

	// 尝试从存储中获取现有会话
	session, err := m.sessionStore.GetSession(ctx, sessionID)
	if err != nil {
		log.Printf("⚠️  获取会话失败: %v", err)
	}

	if session != nil {
		return session
	}

	// 创建新会话
	session = &Session{
		ID:           sessionID,
		Messages:     make([]Message, 0),
		CurrentState: "active",
		CreatedAt:    time.Now(),
	}

	// 保存到存储
	if err := m.sessionStore.SaveSession(ctx, session); err != nil {
		log.Printf("⚠️  保存会话失败: %v", err)
	}

	return session
}

func (m *Manager) buildLLMMessages(session *Session) []llm.Message {
	messages := make([]llm.Message, 0, len(session.Messages)+1)

	// 添加系统提示（合并到用户消息中）
	systemPrompt := `你是一个智能区块链助手，可以帮助用户进行各种DeFi操作。
你可以调用以下工具：
1. QNG工作流 - 处理复杂的DeFi操作流程
2. MetaMask - 钱包连接和签名操作
3. 文件系统 - 文件读写操作

请根据用户需求提供准确的帮助。`

	// 构建完整的对话历史
	var conversation strings.Builder
	conversation.WriteString(systemPrompt)
	conversation.WriteString("\n\n对话历史:\n")

	// 转换会话消息
	for _, msg := range session.Messages {
		role := "用户"
		if msg.Role == "assistant" {
			role = "助手"
		}
		conversation.WriteString(fmt.Sprintf("%s: %s\n", role, msg.Content))
	}

	messages = append(messages, llm.Message{
		Role:    "user",
		Content: conversation.String(),
	})

	return messages
}

func (m *Manager) GetWorkflowStatus(ctx context.Context, workflowID string) (*mcp.WorkflowStatus, error) {
	result, err := m.mcpManager.Call(ctx, "qng", "get_session_status", map[string]any{"session_id": workflowID})
	if err != nil {
		return nil, err
	}

	// 处理从 HTTP 响应解析的结果
	var status mcp.WorkflowStatus
	if resultMap, ok := result.(map[string]interface{}); ok {
		// 从 map 转换为 WorkflowStatus 结构体
		if statusStr, exists := resultMap["status"]; exists {
			if s, ok := statusStr.(string); ok {
				status.Status = s
			}
		}
		if progressFloat, exists := resultMap["progress"]; exists {
			if p, ok := progressFloat.(float64); ok {
				status.Progress = int(p)
			}
		}
		if message, exists := resultMap["message"]; exists {
			if m, ok := message.(string); ok {
				status.Message = m
			}
		}
		if sessionID, exists := resultMap["session_id"]; exists {
			if sid, ok := sessionID.(string); ok {
				status.SessionID = sid
			}
		}
		if needSig, exists := resultMap["need_signature"]; exists {
			if ns, ok := needSig.(bool); ok {
				status.NeedSignature = ns
			}
		}
		if sigReq, exists := resultMap["signature_request"]; exists {
			if sr, ok := sigReq.(map[string]interface{}); ok {
				sigRequest := &mcp.SignatureRequest{}
				if action, ok := sr["action"].(string); ok {
					sigRequest.Action = action
				}
				if fromToken, ok := sr["from_token"].(string); ok {
					sigRequest.FromToken = fromToken
				}
				if toToken, ok := sr["to_token"].(string); ok {
					sigRequest.ToToken = toToken
				}
				if amount, ok := sr["amount"].(string); ok {
					sigRequest.Amount = amount
				}
				if toAddr, ok := sr["to_address"].(string); ok {
					sigRequest.ToAddress = toAddr
				}
				if value, ok := sr["value"].(string); ok {
					sigRequest.Value = value
				}
				if data, ok := sr["data"].(string); ok {
					sigRequest.Data = data
				}
				if gasLimit, ok := sr["gas_limit"].(string); ok {
					sigRequest.GasLimit = gasLimit
				}
				if gasPrice, ok := sr["gas_price"].(string); ok {
					sigRequest.GasPrice = gasPrice
				}
				if gasFee, ok := sr["gas_fee"].(string); ok {
					sigRequest.GasFee = gasFee
				}
				if slippage, ok := sr["slippage"].(string); ok {
					sigRequest.Slippage = slippage
				}
				status.SignatureRequest = sigRequest
			}
		}
		if resultData, exists := resultMap["result"]; exists {
			if rd, ok := resultData.(map[string]interface{}); ok {
				status.Result = rd
			}
		}
		if errorStr, exists := resultMap["error"]; exists {
			if e, ok := errorStr.(string); ok {
				status.Error = e
			}
		}
		return &status, nil
	}

	return nil, fmt.Errorf("invalid workflow status type: %T", result)
}

func (m *Manager) ContinueWorkflowWithSignature(ctx context.Context, workflowID, signature string) (any, error) {
	return m.mcpManager.Call(ctx, "qng", "submit_signature", map[string]any{"session_id": workflowID, "signature": signature})
}

func (m *Manager) GetCapabilities() map[string]any {
	capabilities := m.mcpManager.GetCapabilities()

	result := map[string]any{
		"llm": map[string]any{
			"enabled":   true,
			"providers": []string{"openai", "anthropic", "gemini"},
		},
		"mcp_servers": make(map[string]any),
		"features": []string{
			"natural_language_processing",
			"intent_analysis",
			"workflow_execution",
			"wallet_integration",
			"transaction_signing",
			"file_system_operations",
		},
	}

	// 添加MCP服务器能力
	for serverName, serverCaps := range capabilities {
		result["mcp_servers"].(map[string]any)[serverName] = map[string]any{
			"enabled":      true,
			"capabilities": serverCaps,
		}
	}

	return result
}

// Close 关闭管理器
func (m *Manager) Close() error {
	var errors []error

	// 关闭MCP管理器
	if m.mcpManager != nil {
		if err := m.mcpManager.Close(); err != nil {
			errors = append(errors, fmt.Errorf("close mcp manager: %w", err))
		}
	}

	// 关闭会话存储
	if m.sessionStore != nil {
		if err := m.sessionStore.Close(); err != nil {
			errors = append(errors, fmt.Errorf("close session store: %w", err))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("close manager errors: %v", errors)
	}
	return nil
}
