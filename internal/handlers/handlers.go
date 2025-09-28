package handlers

import (
	"context"
	"fmt"
	"net/http"
	"qng-agent/internal/config"
	"qng-agent/internal/graph"
	"qng-agent/internal/llm"
	"qng-agent/internal/middleware"
	"qng-agent/internal/session"
	"qng-agent/internal/storage"
	"qng-agent/internal/types"
	"qng-agent/internal/websocket"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// Handler contains all HTTP handlers
type Handler struct {
	sessionManager *session.Manager
	llmManager     *llm.Manager
	graphManager   *graph.LLMGraphManager
	config         *config.Config
	storage        storage.Storage
	wsHub          *websocket.Hub
}

// NewHandler creates a new handler
func NewHandler(sessionManager *session.Manager, llmManager *llm.Manager, cfg *config.Config, storage storage.Storage) *Handler {
	graphManager := graph.NewLLMGraphManager(llmManager.GetClient(), llmManager, cfg, storage)
	wsHub := websocket.NewHub()

	return &Handler{
		sessionManager: sessionManager,
		llmManager:     llmManager,
		graphManager:   graphManager,
		config:         cfg,
		storage:        storage,
		wsHub:          wsHub,
	}
}

// CreateSession creates a new chat session
func (h *Handler) CreateSession(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "User ID not found"})
		return
	}

	var req types.CreateSessionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	session, err := h.sessionManager.CreateSession(userID, req.Title)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create session"})
		return
	}

	c.JSON(http.StatusOK, types.CreateSessionResponse{
		Session: *session,
	})
}

// GetSessions retrieves all chat sessions for the user
func (h *Handler) GetSessions(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "User ID not found"})
		return
	}

	sessions, err := h.sessionManager.GetSessions(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get sessions"})
		return
	}

	// Ensure we return an empty array instead of null
	if sessions == nil {
		sessions = []types.ChatSession{}
	}

	c.JSON(http.StatusOK, sessions)
}

// GetSession retrieves a specific chat session
func (h *Handler) GetSession(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "User ID not found"})
		return
	}

	sessionID := c.Param("id")

	session, err := h.sessionManager.GetSession(userID, sessionID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Session not found"})
		return
	}

	c.JSON(http.StatusOK, session)
}

// DeleteSession deletes a chat session
func (h *Handler) DeleteSession(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "User ID not found"})
		return
	}

	sessionID := c.Param("id")

	err := h.sessionManager.DeleteSession(userID, sessionID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete session"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Session deleted"})
}

// UpdateSession updates a chat session
func (h *Handler) UpdateSession(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "User ID not found"})
		return
	}

	sessionID := c.Param("id")

	var req struct {
		Title string `json:"title"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Update session title
	err := h.sessionManager.UpdateSessionTitle(userID, sessionID, req.Title)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update session"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Session updated"})
}

// StreamChat handles streaming chat responses
func (h *Handler) StreamChat(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "User ID not found"})
		return
	}

	var req types.SendMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate session exists
	_, err := h.sessionManager.GetSession(userID, req.SessionID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Session not found"})
		return
	}

	// Set headers for SSE
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Access-Control-Allow-Origin", "*")

	// Create user message
	userMessage := &types.ChatMessage{
		ID:        uuid.New().String(),
		Role:      "user",
		Content:   req.Message,
		Timestamp: time.Now(),
	}

	// Save user message
	if err := h.sessionManager.AddMessage(userID, req.SessionID, userMessage); err != nil {
		c.SSEvent("error", gin.H{"error": "Failed to save user message"})
		return
	}

	// Get session history for context
	history, err := h.sessionManager.GetSessionHistory(userID, req.SessionID)
	if err != nil {
		c.SSEvent("error", gin.H{"error": "Failed to get session history"})
		return
	}

	// Process through QNG Graph workflow
	// Use configured timeout with a reasonable minimum
	timeoutSeconds := h.config.LLM.Timeout
	if timeoutSeconds <= 0 {
		timeoutSeconds = 120 // Default 2 minutes if not configured
	}
	// Add some buffer for processing overhead
	timeoutSeconds += 30
	ctx, cancel := context.WithTimeout(c.Request.Context(), time.Duration(timeoutSeconds)*time.Second)
	defer cancel()

	// Create assistant message
	assistantMessageID := uuid.New().String()

	// Send message ID first using standard SSE format
	fmt.Fprintf(c.Writer, "data: %s\n\n", `{"message_id":"`+assistantMessageID+`"}`)
	c.Writer.Flush()

	// Check if this message should trigger workflow visualization
	shouldTriggerWorkflow := h.shouldTriggerWorkflow(req.Message)
	if shouldTriggerWorkflow {
		// Send workflow status updates via WebSocket
		h.simulateWorkflowExecution(req.Message, req.SessionID)
	}

	// Process message through QNG Graph workflow
	response, needsAuth, err := h.graphManager.ProcessUserMessage(ctx, userID, req.Message, history[:len(history)-1])
	if err != nil {
		fmt.Fprintf(c.Writer, "data: %s\n\n", `{"error":"Failed to process message: `+err.Error()+`"}`)
		c.Writer.Flush()
		return
	}

	// Stream the response character by character
	for i, char := range response {
		fmt.Fprintf(c.Writer, "data: %s\n\n", `{"content":"`+string(char)+`"}`)
		c.Writer.Flush()

		// Add small delay for realistic streaming effect
		if i%10 == 0 {
			time.Sleep(50 * time.Millisecond)
		}
	}

	// If authentication is needed, send auth request
	if needsAuth {
		authData := `{"need_auth":true,"auth_type":"wallet_connection","message":"Please connect your wallet to continue"}`
		fmt.Fprintf(c.Writer, "data: %s\n\n", authData)
		c.Writer.Flush()
	}

	// Send completion signal
	fmt.Fprintf(c.Writer, "data: [DONE]\n\n")
	c.Writer.Flush()

	// Save assistant message
	assistantMessage := &types.ChatMessage{
		ID:        assistantMessageID,
		Role:      "assistant",
		Content:   response,
		Timestamp: time.Now(),
	}

	if err := h.sessionManager.AddMessage(userID, req.SessionID, assistantMessage); err != nil {
		// Log error but don't fail the request
		fmt.Printf("Failed to save assistant message: %v\n", err)
	}
}

// GetSettings retrieves application settings for the user
func (h *Handler) GetSettings(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "User ID not found"})
		return
	}

	settings, err := h.storage.LoadSettings(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load settings"})
		return
	}

	c.JSON(http.StatusOK, settings)
}

// UpdateSettings updates application settings for the user
func (h *Handler) UpdateSettings(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "User ID not found"})
		return
	}

	var settings types.AppSettings
	if err := c.ShouldBindJSON(&settings); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Update LLM client if configuration changed
	if settings.LLMProvider.URL != "" && settings.LLMProvider.Token != "" {
		h.llmManager.UpdateClientFromConfig(settings.LLMProvider)
		h.graphManager = graph.NewLLMGraphManager(h.llmManager.GetClient(), h.llmManager, h.config, h.storage)
	}

	// Save settings
	if err := h.storage.SaveSettings(userID, &settings); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save settings"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Settings updated"})
}

// Health check endpoint
func (h *Handler) Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":    "healthy",
		"timestamp": time.Now(),
	})
}

// InitiateWorkflow initiates a new workflow and returns the visualization graph
func (h *Handler) InitiateWorkflow(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "User ID not found"})
		return
	}

	var req types.WorkflowInitiateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate session exists
	_, err := h.sessionManager.GetSession(userID, req.SessionID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Session not found"})
		return
	}

	// Generate workflow ID
	workflowID := fmt.Sprintf("wf-%s", uuid.New().String())

	// Create workflow graph based on the message
	graph := h.createWorkflowGraph(req.Message, workflowID)

	response := types.WorkflowInitiateResponse{
		WorkflowID: workflowID,
		Graph:      graph,
	}

	c.JSON(http.StatusOK, response)
}

// createWorkflowGraph creates a workflow graph based on the user message
func (h *Handler) createWorkflowGraph(message, workflowID string) types.WorkflowVisualizationGraph {
	// 智能工作流分析 - 根据消息内容动态生成节点
	message = strings.ToLower(message)

	nodes := []types.WorkflowVisualizationNode{}
	edges := []types.WorkflowVisualizationEdge{}
	nodeCounter := 1

	// 1. 总是从意图分析开始
	nodes = append(nodes, types.WorkflowVisualizationNode{
		ID:     fmt.Sprintf("node-%d", nodeCounter),
		Label:  "意图分析",
		Type:   "IntentAnalysis",
		Status: types.NodeStatusPending,
	})
	intentNodeID := fmt.Sprintf("node-%d", nodeCounter)
	nodeCounter++

	// 2. 根据消息内容分析需要哪些操作
	operations := h.analyzeMessageIntent(message)

	// 3. 为每个操作创建节点
	var lastNodeID string
	for i, op := range operations {
		nodeID := fmt.Sprintf("node-%d", nodeCounter)
		nodeCounter++

		nodes = append(nodes, types.WorkflowVisualizationNode{
			ID:     nodeID,
			Label:  op.Label,
			Type:   op.Type,
			Status: types.NodeStatusPending,
		})

		// 连接节点
		if i == 0 {
			// 第一个操作节点连接到意图分析
			edges = append(edges, types.WorkflowVisualizationEdge{
				Source: intentNodeID,
				Target: nodeID,
			})
		} else {
			// 后续节点连接到前一个节点
			edges = append(edges, types.WorkflowVisualizationEdge{
				Source: lastNodeID,
				Target: nodeID,
			})
		}
		lastNodeID = nodeID
	}

	// 4. 添加结果格式化节点（如果需要）
	if len(operations) > 0 {
		formatNodeID := fmt.Sprintf("node-%d", nodeCounter)
		nodes = append(nodes, types.WorkflowVisualizationNode{
			ID:     formatNodeID,
			Label:  "格式化最终结果",
			Type:   "LLMBeautify",
			Status: types.NodeStatusPending,
		})

		edges = append(edges, types.WorkflowVisualizationEdge{
			Source: lastNodeID,
			Target: formatNodeID,
		})
	}

	return types.WorkflowVisualizationGraph{
		Nodes: nodes,
		Edges: edges,
	}
}

// WorkflowOperation represents a single operation in the workflow
type WorkflowOperation struct {
	Label string
	Type  string
}

// analyzeMessageIntent analyzes the message and returns required operations
func (h *Handler) analyzeMessageIntent(message string) []WorkflowOperation {
	operations := []WorkflowOperation{}

	// 检查是否需要查询区块信息
	if containsKeywords(message, []string{"block", "区块", "height", "高度", "latest", "最新"}) {
		operations = append(operations, WorkflowOperation{
			Label: "查询区块信息",
			Type:  "APICall",
		})
	}

	// 检查是否需要查询交易信息
	if containsKeywords(message, []string{"transaction", "交易", "tx", "txid"}) {
		operations = append(operations, WorkflowOperation{
			Label: "查询交易信息",
			Type:  "APICall",
		})
	}

	// 检查是否需要查询余额信息
	if containsKeywords(message, []string{"balance", "余额", "address", "地址"}) {
		operations = append(operations, WorkflowOperation{
			Label: "查询余额信息",
			Type:  "APICall",
		})
	}

	// 检查是否需要查询状态根信息
	if containsKeywords(message, []string{"stateroot", "state", "状态", "root"}) {
		operations = append(operations, WorkflowOperation{
			Label: "查询状态根信息",
			Type:  "APICall",
		})
	}

	// 检查是否需要参数提取
	if containsKeywords(message, []string{"参数", "parameter", "提取", "extract"}) {
		operations = append(operations, WorkflowOperation{
			Label: "提取参数",
			Type:  "LLMParameterExtraction",
		})
	}

	// 检查是否需要数据分析
	if containsKeywords(message, []string{"分析", "analyze", "统计", "statistics", "比较", "compare"}) {
		operations = append(operations, WorkflowOperation{
			Label: "数据分析",
			Type:  "LLMAnalysis",
		})
	}

	// 检查是否需要智能合约调用
	if containsKeywords(message, []string{"contract", "合约", "call", "调用", "execute", "执行"}) {
		operations = append(operations, WorkflowOperation{
			Label: "智能合约调用",
			Type:  "ContractCall",
		})
	}

	// 如果没有匹配到任何操作，添加默认的查询操作
	if len(operations) == 0 {
		operations = append(operations, WorkflowOperation{
			Label: "执行查询操作",
			Type:  "APICall",
		})
	}

	return operations
}

// containsKeywords checks if message contains any of the specified keywords
func containsKeywords(message string, keywords []string) bool {
	messageLower := strings.ToLower(message)
	for _, keyword := range keywords {
		if strings.Contains(messageLower, strings.ToLower(keyword)) {
			return true
		}
	}
	return false
}

// ServeWebSocket handles WebSocket connections for workflow status updates
func (h *Handler) ServeWebSocket(c *gin.Context) {
	h.wsHub.ServeWS(c)
}

// GetWebSocketHub returns the WebSocket hub for external access
func (h *Handler) GetWebSocketHub() *websocket.Hub {
	return h.wsHub
}

// shouldTriggerWorkflow determines if a message should trigger workflow visualization
func (h *Handler) shouldTriggerWorkflow(message string) bool {
	workflowKeywords := []string{
		"查询", "获取", "调用", "执行", "分析", "比较", "统计",
		"query", "get", "call", "execute", "analyze", "compare", "statistics",
		"rpc", "api", "block", "transaction", "balance", "state", "stateroot",
	}

	messageLower := strings.ToLower(message)
	for _, keyword := range workflowKeywords {
		if strings.Contains(messageLower, strings.ToLower(keyword)) {
			return true
		}
	}
	return false
}

// simulateWorkflowExecution simulates workflow execution and sends status updates
func (h *Handler) simulateWorkflowExecution(message, sessionID string) {
	// Generate a workflow ID
	workflowID := fmt.Sprintf("wf-%s", uuid.New().String())

	// Create workflow graph based on message analysis
	graph := h.createWorkflowGraph(message, workflowID)

	// Extract node information for simulation
	nodeInfos := make([]struct {
		ID    string
		Label string
		Type  string
	}, len(graph.Nodes))

	for i, node := range graph.Nodes {
		nodeInfos[i] = struct {
			ID    string
			Label string
			Type  string
		}{
			ID:    node.ID,
			Label: node.Label,
			Type:  node.Type,
		}
	}

	// Start workflow execution simulation in a goroutine
	go func() {
		// Simulate node execution with delays
		for i, nodeInfo := range nodeInfos {
			// Set node to executing
			update := types.WorkflowStatusUpdate{
				WorkflowID: workflowID,
				NodeID:     nodeInfo.ID,
				Status:     types.NodeStatusExecuting,
				Data:       map[string]interface{}{"label": nodeInfo.Label, "type": nodeInfo.Type},
			}
			h.wsHub.BroadcastWorkflowUpdate(update)

			// Simulate execution time based on node type
			executionTime := h.getExecutionTimeForNodeType(nodeInfo.Type)
			time.Sleep(executionTime)

			// 智能失败逻辑 - 根据实际业务场景模拟失败
			success := h.shouldNodeSucceed(nodeInfo, i, len(nodeInfos))
			status := types.NodeStatusSuccess
			var errorMsg string

			if !success {
				status = types.NodeStatusFailure
				errorMsg = h.getFailureReason(nodeInfo)
			}

			update = types.WorkflowStatusUpdate{
				WorkflowID: workflowID,
				NodeID:     nodeInfo.ID,
				Status:     status,
				Data:       map[string]interface{}{"label": nodeInfo.Label, "type": nodeInfo.Type, "result": fmt.Sprintf("节点 %s 执行完成", nodeInfo.Label)},
				Error:      errorMsg,
			}
			h.wsHub.BroadcastWorkflowUpdate(update)
		}

		// Send workflow completion
		complete := types.WorkflowComplete{
			WorkflowID: workflowID,
			Success:    true,
			Summary:    "工作流执行完成",
			TotalTime:  int64(len(nodeInfos) * 2 * 1000), // 2 seconds per node in milliseconds
		}
		h.wsHub.BroadcastWorkflowComplete(complete)
	}()
}

// getExecutionTimeForNodeType returns execution time based on node type
func (h *Handler) getExecutionTimeForNodeType(nodeType string) time.Duration {
	switch nodeType {
	case "IntentAnalysis":
		return 1 * time.Second
	case "APICall":
		return 2 * time.Second
	case "LLMParameterExtraction":
		return 1 * time.Second
	case "LLMAnalysis":
		return 3 * time.Second
	case "ContractCall":
		return 2 * time.Second
	case "LLMBeautify":
		return 1 * time.Second
	default:
		return 1 * time.Second
	}
}

// shouldNodeSucceed determines if a node should succeed based on business logic
func (h *Handler) shouldNodeSucceed(nodeInfo struct {
	ID    string
	Label string
	Type  string
}, index, totalNodes int) bool {
	// 默认所有节点都成功
	success := true

	// 可以根据具体业务场景设置失败条件
	// 例如：模拟网络问题、API限制等

	// 模拟状态根查询的失败情况（10%概率）
	if nodeInfo.Type == "APICall" && strings.Contains(nodeInfo.Label, "查询状态根信息") {
		// 这里可以添加更复杂的失败逻辑
		// 例如：检查网络状态、API限制等
		success = true // 默认成功，可以根据实际需求调整
	}

	// 模拟API调用失败（5%概率）
	if nodeInfo.Type == "APICall" && index > 0 {
		// 可以添加随机失败逻辑
		// success = rand.Float32() > 0.05 // 5%失败率
	}

	return success
}

// getFailureReason returns a realistic failure reason for a node
func (h *Handler) getFailureReason(nodeInfo struct {
	ID    string
	Label string
	Type  string
}) string {
	switch nodeInfo.Type {
	case "APICall":
		return "API调用失败：网络连接超时"
	case "LLMParameterExtraction":
		return "参数提取失败：无法识别有效参数"
	case "LLMAnalysis":
		return "数据分析失败：数据格式不正确"
	case "ContractCall":
		return "智能合约调用失败：合约执行错误"
	default:
		return "节点执行失败：未知错误"
	}
}
