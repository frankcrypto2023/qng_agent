package mcp

import (
	"context"
	"fmt"
	"log"
	"qng_agent/internal/config"
	"qng_agent/internal/qng"
	"sync"
	"time"
)

type QNGServer struct {
	config     config.QNGConfig
	chain      *qng.Chain
	sessions   map[string]*Session
	sessionsMu sync.RWMutex
}

// Session和SessionUpdate类型已在types.go中定义

func NewQNGServer(config config.QNGConfig) *QNGServer {
	// 转换为qng.ChainConfig
	chainConfig := qng.ChainConfig{
		Enabled: config.Enabled,
		Host:    "localhost", // 默认值
		Port:    9091,        // 默认值
		Timeout: 30,          // 默认值
		Network: config.Network,
		RPCURL:  config.RPCURL,
		Transaction: qng.TransactionConfig{
			ConfirmationTimeout:   config.Transaction.ConfirmationTimeout,
			PollingInterval:       config.Transaction.PollingInterval,
			RequiredConfirmations: config.Transaction.RequiredConfirmations,
		},
		LangGraph: qng.LangGraphConfig{
			Enabled: config.LangGraph.Enabled,
			Nodes:   config.LangGraph.Nodes,
		},
		LLM: qng.LLMConfig{
			Provider: config.LLM.Provider,
			OpenAI: qng.OpenAIConfig{
				APIKey:    config.LLM.OpenAI.APIKey,
				Model:     config.LLM.OpenAI.Model,
				BaseURL:   config.LLM.OpenAI.BaseURL,
				Timeout:   config.LLM.OpenAI.Timeout,
				MaxTokens: config.LLM.OpenAI.MaxTokens,
			},
			Gemini: qng.GeminiConfig{
				APIKey:  config.LLM.Gemini.APIKey,
				Model:   config.LLM.Gemini.Model,
				Timeout: config.LLM.Gemini.Timeout,
			},
			Anthropic: qng.AnthropicConfig{
				APIKey:  config.LLM.Anthropic.APIKey,
				Model:   config.LLM.Anthropic.Model,
				Timeout: config.LLM.Anthropic.Timeout,
			},
		},
		Chain: qng.ChainSubConfig{
			Enabled: config.Enabled,
			Network: config.Network,
			RPCURL:  config.RPCURL,
			Transaction: qng.TransactionConfig{
				ConfirmationTimeout:   config.Transaction.ConfirmationTimeout,
				PollingInterval:       config.Transaction.PollingInterval,
				RequiredConfirmations: config.Transaction.RequiredConfirmations,
			},
			LangGraph: qng.LangGraphConfig{
				Enabled: config.LangGraph.Enabled,
				Nodes:   config.LangGraph.Nodes,
			},
			LLM: qng.LLMConfig{
				Provider: config.LLM.Provider,
				OpenAI: qng.OpenAIConfig{
					APIKey:    config.LLM.OpenAI.APIKey,
					Model:     config.LLM.OpenAI.Model,
					BaseURL:   config.LLM.OpenAI.BaseURL,
					Timeout:   config.LLM.OpenAI.Timeout,
					MaxTokens: config.LLM.OpenAI.MaxTokens,
				},
				Gemini: qng.GeminiConfig{
					APIKey:  config.LLM.Gemini.APIKey,
					Model:   config.LLM.Gemini.Model,
					Timeout: config.LLM.Gemini.Timeout,
				},
				Anthropic: qng.AnthropicConfig{
					APIKey:  config.LLM.Anthropic.APIKey,
					Model:   config.LLM.Anthropic.Model,
					Timeout: config.LLM.Anthropic.Timeout,
				},
			},
		},
	}

	chain := qng.NewChain(chainConfig)

	server := &QNGServer{
		config:   config,
		chain:    chain,
		sessions: make(map[string]*Session),
	}

	return server
}

func (s *QNGServer) Start() error {
	log.Printf("🚀 QNG MCP服务器启动")

	// 启动QNG Chain
	if err := s.chain.Start(); err != nil {
		log.Printf("❌ 启动QNG Chain失败: %v", err)
		return err
	}

	log.Printf("✅ QNG MCP服务器启动成功")
	return nil
}

func (s *QNGServer) Stop() error {
	log.Printf("🛑 QNG MCP服务器停止")

	// 停止所有会话
	s.sessionsMu.Lock()
	for _, session := range s.sessions {
		close(session.CancelChan)
	}
	s.sessionsMu.Unlock()

	// 停止QNG Chain
	if err := s.chain.Stop(); err != nil {
		log.Printf("❌ 停止QNG Chain失败: %v", err)
		return err
	}

	log.Printf("✅ QNG MCP服务器停止成功")
	return nil
}

func (s *QNGServer) Call(ctx context.Context, method string, params map[string]any) (any, error) {
	log.Printf("🔄 QNG MCP服务器调用")
	log.Printf("🛠️  方法: %s", method)
	log.Printf("📋 参数: %+v", params)

	switch method {
	case "execute_workflow":
		return s.executeWorkflow(ctx, params)
	case "get_session_status":
		return s.getSessionStatus(ctx, params)
	case "submit_signature":
		return s.submitSignature(ctx, params)
	case "poll_session":
		return s.pollSession(ctx, params)
	default:
		log.Printf("❌ 未知方法: %s", method)
		return nil, fmt.Errorf("unknown method: %s", method)
	}
}

func (s *QNGServer) executeWorkflow(ctx context.Context, params map[string]any) (any, error) {
	log.Printf("🔄 执行工作流")

	message, ok := params["message"].(string)
	if !ok {
		log.Printf("❌ 缺少message参数")
		return nil, fmt.Errorf("message parameter required")
	}

	log.Printf("📝 用户消息: %s", message)

	// 创建新会话
	sessionID := generateSessionID()
	workflowID := generateWorkflowID()

	session := &Session{
		ID:          sessionID,
		WorkflowID:  workflowID,
		Status:      "pending",
		Message:     message,
		CreatedAt:   time.Now().Format(time.RFC3339),
		UpdatedAt:   time.Now().Format(time.RFC3339),
		PollingChan: make(chan *SessionUpdate, 10),
		CancelChan:  make(chan bool, 1),
	}

	// 保存会话（同时使用 sessionID 和 workflowID 作为 key）
	s.sessionsMu.Lock()
	s.sessions[sessionID] = session
	s.sessions[workflowID] = session // 允许通过 workflowID 查询
	s.sessionsMu.Unlock()

	log.Printf("✅ 创建会话: %s", sessionID)
	log.Printf("📋 工作流ID: %s", workflowID)

	// 异步执行工作流
	go s.executeWorkflowAsync(session, message)

	return map[string]any{
		"session_id":  sessionID,
		"workflow_id": workflowID,
		"status":      "pending",
		"message":     "工作流已提交，正在处理中...",
	}, nil
}

func (s *QNGServer) executeWorkflowAsync(session *Session, message string) {
	log.Printf("🔄 异步执行工作流")
	log.Printf("📋 会话ID: %s", session.ID)
	log.Printf("📝 消息: %s", message)

	// 更新状态为运行中
	s.updateSessionStatus(session, "running", "正在执行工作流...")

	// 创建上下文
	ctx := context.WithValue(context.Background(), "workflow_id", session.WorkflowID)
	ctx = context.WithValue(ctx, "session_id", session.ID)

	// 执行工作流
	result, err := s.chain.ProcessMessage(ctx, message)
	if err != nil {
		log.Printf("❌ 工作流执行失败: %v", err)
		s.updateSessionStatus(session, "failed", fmt.Sprintf("执行失败: %v", err))
		return
	}

	// 检查是否需要签名
	if result.NeedSignature {
		log.Printf("✍️  需要用户签名")
		session.Context = result.WorkflowContext

		// 将签名请求转换为正确的类型并保存
		if result.SignatureRequest != nil {
			qngSigReq := result.SignatureRequest
			signatureRequest := &SignatureRequest{
				Action:    qngSigReq.Action,
				FromToken: qngSigReq.FromToken,
				ToToken:   qngSigReq.ToToken,
				Amount:    qngSigReq.Amount,
				ToAddress: qngSigReq.ToAddress,
				Value:     qngSigReq.Value,
				Data:      qngSigReq.Data,
				GasLimit:  qngSigReq.GasLimit,
				GasPrice:  qngSigReq.GasPrice,
				GasFee:    qngSigReq.GasFee,
				Slippage:  qngSigReq.Slippage,
			}
			session.SignatureRequest = signatureRequest

			log.Printf("✅ 签名请求已保存到会话")
			log.Printf("📋 签名请求详情: action=%s, from=%s->%s, amount=%s",
				signatureRequest.Action, signatureRequest.FromToken, signatureRequest.ToToken, signatureRequest.Amount)
			log.Printf("📋 交易数据: to=%s, value=%s, data=%s",
				signatureRequest.ToAddress, signatureRequest.Value, signatureRequest.Data)
		}

		s.updateSessionStatus(session, "waiting_signature", "等待用户签名授权")

		// 发送签名请求
		s.sendSessionUpdate(session, "signature_request", result.SignatureRequest)
		return
	}

	// 工作流完成
	log.Printf("✅ 工作流执行完成")
	session.Result = result.FinalResult
	s.updateSessionStatus(session, "completed", "工作流执行完成")

	// 发送结果
	s.sendSessionUpdate(session, "result", result.FinalResult)
}

func (s *QNGServer) getSessionStatus(ctx context.Context, params map[string]any) (any, error) {
	log.Printf("📋 获取会话状态")

	sessionID, ok := params["session_id"].(string)
	if !ok {
		log.Printf("❌ 缺少session_id参数")
		return nil, fmt.Errorf("session_id parameter required")
	}

	s.sessionsMu.RLock()
	session, exists := s.sessions[sessionID]
	s.sessionsMu.RUnlock()

	if !exists {
		log.Printf("❌ 会话不存在: %s", sessionID)
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}

	log.Printf("✅ 返回会话状态: %s", session.Status)

	result := map[string]any{
		"session_id":     session.ID,
		"workflow_id":    session.WorkflowID,
		"status":         session.Status,
		"message":        session.Message,
		"created_at":     session.CreatedAt,
		"updated_at":     session.UpdatedAt,
		"need_signature": session.Status == "waiting_signature",
	}

	// 如果需要签名，添加签名请求数据
	if session.Status == "waiting_signature" && session.SignatureRequest != nil {
		result["signature_request"] = session.SignatureRequest
	}

	return result, nil
}

func (s *QNGServer) submitSignature(ctx context.Context, params map[string]any) (any, error) {
	log.Printf("✍️  提交签名")

	sessionID, ok := params["session_id"].(string)
	if !ok {
		log.Printf("❌ 缺少session_id参数")
		return nil, fmt.Errorf("session_id parameter required")
	}

	signature, ok := params["signature"].(string)
	if !ok {
		log.Printf("❌ 缺少signature参数")
		return nil, fmt.Errorf("signature parameter required")
	}

	log.Printf("🔐 签名长度: %d", len(signature))

	s.sessionsMu.RLock()
	session, exists := s.sessions[sessionID]
	s.sessionsMu.RUnlock()

	if !exists {
		log.Printf("❌ 会话不存在: %s", sessionID)
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}

	if session.Status != "waiting_signature" {
		log.Printf("❌ 会话状态不正确: %s", session.Status)
		return nil, fmt.Errorf("session not in waiting_signature status")
	}

	log.Printf("✅ 验证签名并继续工作流")

	// 更新状态为运行中
	s.updateSessionStatus(session, "running", "正在处理签名...")

	// 异步继续工作流
	go s.continueWorkflowWithSignature(session, signature)

	return map[string]any{
		"session_id": session.ID,
		"status":     "processing",
		"message":    "签名已提交，正在处理...",
	}, nil
}

func (s *QNGServer) continueWorkflowWithSignature(session *Session, signature string) {
	log.Printf("🔄 使用签名继续工作流")
	log.Printf("📋 会话ID: %s", session.ID)

	// 创建上下文
	ctx := context.WithValue(context.Background(), "workflow_id", session.WorkflowID)
	ctx = context.WithValue(ctx, "session_id", session.ID)

	// 继续工作流
	result, err := s.chain.ContinueWithSignature(ctx, session.Context, signature)
	if err != nil {
		log.Printf("❌ 继续工作流失败: %v", err)
		s.updateSessionStatus(session, "failed", fmt.Sprintf("继续执行失败: %v", err))
		return
	}

	// 检查是否需要新的签名请求
	if result.NeedSignature {
		log.Printf("🔔 检测到新的签名请求")

		// 保存工作流上下文
		session.Context = result.WorkflowContext

		// 处理签名请求
		if result.SignatureRequest != nil {
			qngSigReq := result.SignatureRequest
			signatureRequest := &SignatureRequest{
				Action:    qngSigReq.Action,
				FromToken: qngSigReq.FromToken,
				ToToken:   qngSigReq.ToToken,
				Amount:    qngSigReq.Amount,
				ToAddress: qngSigReq.ToAddress,
				Value:     qngSigReq.Value,
				Data:      qngSigReq.Data,
				GasLimit:  qngSigReq.GasLimit,
				GasPrice:  qngSigReq.GasPrice,
				GasFee:    qngSigReq.GasFee,
				Slippage:  qngSigReq.Slippage,
			}
			session.SignatureRequest = signatureRequest

			log.Printf("✅ 新签名请求已保存到会话")
			log.Printf("📋 签名请求详情: action=%s, from=%s->%s, amount=%s",
				signatureRequest.Action, signatureRequest.FromToken, signatureRequest.ToToken, signatureRequest.Amount)
			log.Printf("📋 交易数据: to=%s, value=%s, data=%s",
				signatureRequest.ToAddress, signatureRequest.Value, signatureRequest.Data)
		}

		s.updateSessionStatus(session, "waiting_signature", "等待用户签名授权")

		// 发送签名请求
		s.sendSessionUpdate(session, "signature_request", result.SignatureRequest)
		return
	}

	// 工作流完成
	log.Printf("✅ 工作流执行完成")
	session.Result = result.FinalResult
	s.updateSessionStatus(session, "completed", "工作流执行完成")

	// 发送结果
	s.sendSessionUpdate(session, "result", result.FinalResult)
}

func (s *QNGServer) pollSession(ctx context.Context, params map[string]any) (any, error) {
	log.Printf("🔄 Long Polling会话")

	sessionID, ok := params["session_id"].(string)
	if !ok {
		log.Printf("❌ 缺少session_id参数")
		return nil, fmt.Errorf("session_id parameter required")
	}

	timeout, ok := params["timeout"].(int)
	if !ok {
		timeout = 30 // 默认30秒
	}

	s.sessionsMu.RLock()
	session, exists := s.sessions[sessionID]
	s.sessionsMu.RUnlock()

	if !exists {
		log.Printf("❌ 会话不存在: %s", sessionID)
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}

	log.Printf("⏰ 等待会话更新，超时时间: %d秒", timeout)

	// 等待会话更新
	select {
	case update := <-session.PollingChan:
		log.Printf("✅ 收到会话更新: %s", update.Type)
		return map[string]any{
			"session_id": session.ID,
			"update":     update,
		}, nil
	case <-time.After(time.Duration(timeout) * time.Second):
		log.Printf("⏰ 会话轮询超时")
		return map[string]any{
			"session_id": session.ID,
			"timeout":    true,
			"message":    "轮询超时，请重试",
		}, nil
	case <-session.CancelChan:
		log.Printf("🛑 会话已取消")
		return map[string]any{
			"session_id": session.ID,
			"cancelled":  true,
			"message":    "会话已取消",
		}, nil
	}
}

func (s *QNGServer) updateSessionStatus(session *Session, status, message string) {
	log.Printf("🔄 更新会话状态: %s -> %s", session.Status, status)

	session.Status = status
	session.Message = message
	session.UpdatedAt = time.Now().Format(time.RFC3339)
	session.CreatedAt = time.Now().Format(time.RFC3339)

	log.Printf("✅ 会话状态已更新")
}

func (s *QNGServer) sendSessionUpdate(session *Session, updateType string, data any) {
	log.Printf("📤 发送会话更新: %s", updateType)

	update := &SessionUpdate{
		Type:    updateType,
		Data:    data,
		Session: session,
	}

	// 非阻塞发送
	select {
	case session.PollingChan <- update:
		log.Printf("✅ 会话更新已发送")
	default:
		log.Printf("⚠️  会话更新通道已满，跳过发送")
	}
}

// GetCapabilities 获取QNG服务器能力（字符串列表）
func (s *QNGServer) GetCapabilities() []string {
	return []string{
		"execute_workflow",
		"get_session_status",
		"submit_signature",
		"poll_session",
	}
}

// GetDetailedCapabilities 获取QNG服务器详细能力
func (s *QNGServer) GetDetailedCapabilities() []Capability {
	return []Capability{
		{
			Name:        "execute_workflow",
			Description: "执行QNG工作流，支持代币兑换、质押等操作",
			Parameters: []Parameter{
				{
					Name:        "message",
					Type:        "string",
					Description: "用户请求消息，例如：'我要将1 MEER兑换成MTK，再将对应的MTK质押'",
					Required:    true,
				},
			},
		},
		{
			Name:        "get_session_status",
			Description: "获取工作流会话状态",
			Parameters: []Parameter{
				{
					Name:        "session_id",
					Type:        "string",
					Description: "会话ID，用于查询特定会话的状态",
					Required:    true,
				},
			},
		},
		{
			Name:        "submit_signature",
			Description: "提交用户签名以继续工作流执行",
			Parameters: []Parameter{
				{
					Name:        "session_id",
					Type:        "string",
					Description: "会话ID，用于标识要继续的工作流",
					Required:    true,
				},
				{
					Name:        "signature",
					Type:        "string",
					Description: "用户签名，用于授权交易执行",
					Required:    true,
				},
			},
		},
		{
			Name:        "poll_session",
			Description: "Long Polling会话更新，实时获取会话状态变化",
			Parameters: []Parameter{
				{
					Name:        "session_id",
					Type:        "string",
					Description: "会话ID，用于监听特定会话的状态变化",
					Required:    true,
				},
				{
					Name:        "timeout",
					Type:        "int",
					Description: "超时时间（秒），默认30秒",
					Required:    false,
				},
			},
		},
	}
}

func generateSessionID() string {
	return fmt.Sprintf("session_%d", time.Now().UnixNano())
}

func generateWorkflowID() string {
	return fmt.Sprintf("workflow_%d", time.Now().UnixNano())
}
