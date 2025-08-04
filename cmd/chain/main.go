package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"qng_agent/internal/config"
	"qng_agent/internal/contracts"
	"qng_agent/internal/llm"
	"qng_agent/internal/protocol"
	"qng_agent/internal/qng"
	"qng_agent/internal/roles"
	"qng_agent/internal/rpc"
	"qng_agent/internal/service"
	"qng_agent/internal/sop"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
)

// 全局SOP系统变量
var (
	sopEngine   *sop.WorkflowEngineImpl
	roleManager *sop.RoleManagerImpl
	messagePool *protocol.MessagePool
)

// registerStandardRoles 注册标准角色
func registerStandardRoles() {
	roleDefinitions := []struct {
		roleType    roles.RoleType
		name        string
		description string
	}{
		{roles.RoleStrategyAnalyst, "Strategy Analyst", "Analyzes user requests and formulates Web3 strategies"},
		{roles.RoleRiskManager, "Risk Manager", "Assesses and mitigates risks in Web3 operations"},
		{roles.RoleTransactionBuilder, "Transaction Builder", "Constructs and optimizes blockchain transactions"},
		{roles.RoleExecutor, "Executor", "Executes transactions on blockchain networks"},
		{roles.RoleAuditor, "Auditor", "Audits execution results and ensures compliance"},
	}

	for _, roleInfo := range roleDefinitions {
		var demoRole roles.BaseRole
		switch roleInfo.roleType {
		case roles.RoleStrategyAnalyst:
			demoRole = roles.NewStrategyAnalyst()
		case roles.RoleRiskManager:
			demoRole = roles.NewRiskManager()
		case roles.RoleTransactionBuilder:
			demoRole = roles.NewTransactionBuilder()
		case roles.RoleExecutor:
			demoRole = roles.NewExecutor()
		case roles.RoleAuditor:
			demoRole = roles.NewAuditor()
		default:
			log.Printf("Warning: Unknown role type: %s", roleInfo.roleType)
			continue
		}

		if err := roleManager.RegisterRole(demoRole); err != nil {
			log.Printf("Failed to register role %s: %v", roleInfo.name, err)
		} else {
			log.Printf("✅ Registered role: %s", roleInfo.name)
		}
	}
}

// registerStandardRolesWithLLM 注册基于LLM的标准角色
func registerStandardRolesWithLLM(llmClient llm.Client) {
	if llmClient == nil {
		log.Printf("⚠️  LLM客户端为空，使用模拟角色")
		registerStandardRoles()
		return
	}

	log.Printf("🤖 使用LLM集成的角色系统")

	// 注册LLM集成的策略分析师
	llmStrategyAnalyst := roles.NewLLMStrategyAnalyst(llmClient)
	if err := roleManager.RegisterRole(llmStrategyAnalyst); err != nil {
		log.Printf("Failed to register LLM Strategy Analyst: %v", err)
	} else {
		log.Printf("✅ Registered LLM Strategy Analyst")
	}

	// 注册其他角色（暂时使用模拟版本）
	roleDefinitions := []struct {
		roleType    roles.RoleType
		name        string
		description string
	}{
		{roles.RoleRiskManager, "Risk Manager", "Assesses and mitigates risks in Web3 operations"},
		{roles.RoleTransactionBuilder, "Transaction Builder", "Constructs and optimizes blockchain transactions"},
		{roles.RoleExecutor, "Executor", "Executes transactions on blockchain networks"},
		{roles.RoleAuditor, "Auditor", "Audits execution results and ensures compliance"},
	}

	for _, roleInfo := range roleDefinitions {
		var demoRole roles.BaseRole
		switch roleInfo.roleType {
		case roles.RoleRiskManager:
			demoRole = roles.NewRiskManager()
		case roles.RoleTransactionBuilder:
			demoRole = roles.NewTransactionBuilder()
		case roles.RoleExecutor:
			demoRole = roles.NewExecutor()
		case roles.RoleAuditor:
			demoRole = roles.NewAuditor()
		default:
			log.Printf("Warning: Unknown role type: %s", roleInfo.roleType)
			continue
		}

		if err := roleManager.RegisterRole(demoRole); err != nil {
			log.Printf("Failed to register role %s: %v", roleInfo.name, err)
		} else {
			log.Printf("✅ Registered role: %s", roleInfo.name)
		}
	}
}

// convertSOPSignatureRequest 将SOP签名请求转换为QNG签名请求
func convertSOPSignatureRequest(sopReq *sop.SignatureRequest) *qng.SignatureRequest {
	if sopReq == nil {
		return nil
	}

	return &qng.SignatureRequest{
		Action:    sopReq.Action,
		FromToken: sopReq.FromToken,
		ToToken:   sopReq.ToToken,
		Amount:    sopReq.Amount,
		ToAddress: sopReq.ToAddress,
		Value:     sopReq.Value,
		Data:      sopReq.Data,
		GasLimit:  sopReq.GasLimit,
		GasPrice:  sopReq.GasPrice,
		GasFee:    sopReq.GasFee,
		Slippage:  sopReq.Slippage,
	}
}

func main() {
	log.Println("=== QNG Chain 服务启动 ===")

	// 加载配置
	cfg := config.LoadConfig("config/config.yaml")
	if cfg == nil {
		log.Fatal("Failed to load config")
	}

	// 获取服务注册中心
	registry := service.GetRegistry()

	// 注册自己为Chain服务
	chainService := &service.ServiceInfo{
		Name:    "chain",
		Address: "localhost",
		Port:    9092,
		Endpoints: []string{
			"/api/chain/process",
			"/api/chain/status",
			"/api/chain/nodes",
			"/health",
		},
		Metadata: map[string]string{
			"type":    "qng_chain",
			"version": "1.0.0",
		},
	}

	if err := registry.RegisterService(chainService); err != nil {
		log.Fatal("Failed to register Chain service:", err)
	}

	// 检查QNG服务是否启用
	if qngServerConfig, exists := cfg.MCP.Servers["qng"]; exists && qngServerConfig.Enabled {
		log.Printf("✅ QNG服务已启用")
	} else {
		log.Fatal("QNG服务未在配置中启用")
	}

	// 创建Chain配置，使用配置文件中的LLM设置
	chainConfig := qng.ChainConfig{
		Enabled: true,
		Host:    "localhost",
		Port:    9091,
		Timeout: 30,
		Chain: qng.ChainSubConfig{
			Enabled: true,
			Network: "mainnet",
			RPCURL:  "http://47.242.255.132:1234/",
			Transaction: qng.TransactionConfig{
				ConfirmationTimeout:   60,
				PollingInterval:       2,
				RequiredConfirmations: 1,
			},
			LangGraph: qng.LangGraphConfig{
				Enabled: true,
				Nodes: []string{
					"task_decomposer",
					"swap_executor",
					"stake_executor",
					"signature_validator",
					"result_aggregator",
				},
			},
			LLM: qng.LLMConfig{
				Provider: cfg.LLM.Provider,
				OpenAI: qng.OpenAIConfig{
					APIKey:    cfg.LLM.OpenAI.APIKey,
					Model:     cfg.LLM.OpenAI.Model,
					BaseURL:   cfg.LLM.OpenAI.BaseURL,
					Timeout:   cfg.LLM.OpenAI.Timeout,
					MaxTokens: cfg.LLM.OpenAI.MaxTokens,
				},
				Gemini: qng.GeminiConfig{
					APIKey:  cfg.LLM.Gemini.APIKey,
					Model:   cfg.LLM.Gemini.Model,
					Timeout: cfg.LLM.Gemini.Timeout,
				},
				Anthropic: qng.AnthropicConfig{
					APIKey:  cfg.LLM.Anthropic.APIKey,
					Model:   cfg.LLM.Anthropic.Model,
					Timeout: cfg.LLM.Anthropic.Timeout,
				},
				ModelScope: qng.ModelScopeConfig{
					APIKey:    cfg.LLM.ModelScope.APIKey,
					Model:     cfg.LLM.ModelScope.Model,
					BaseURL:   cfg.LLM.ModelScope.BaseURL,
					Timeout:   cfg.LLM.ModelScope.Timeout,
					MaxTokens: cfg.LLM.ModelScope.MaxTokens,
				},
			},
		},
	}

	// 初始化QNG Chain
	chain := qng.NewChain(chainConfig)
	log.Printf("🔗 初始化QNG链，RPC: %s", chainConfig.Chain.RPCURL)
	log.Printf("🤖 使用LLM提供商: %s", chainConfig.Chain.LLM.Provider)

	// 工作流会话存储
	workflowSessions := make(map[string]*qng.ProcessResult)
	var sessionsMu sync.RWMutex

	// 启动Chain服务
	if err := chain.Start(); err != nil {
		log.Fatal("Failed to start chain:", err)
	}
	log.Println("✅ QNG Chain已启动")

	// 初始化SOP系统
	log.Println("🔧 初始化SOP工作流系统...")

	// 创建合约管理器
	contractManager, err := contracts.NewContractManager("config/contracts.json")
	if err != nil {
		log.Printf("⚠️ 无法创建合约管理器: %v，将使用默认配置", err)
		contractManager = nil
	} else {
		log.Printf("✅ 合约管理器初始化成功")
	}

	// 创建RPC客户端
	var rpcClient *rpc.Client
	if chainConfig.Chain.RPCURL != "" {
		rpcClient = rpc.NewClient(chainConfig.Chain.RPCURL)
		log.Printf("✅ RPC客户端已创建: %s", chainConfig.Chain.RPCURL)
	} else {
		log.Printf("⚠️  未配置RPC URL，使用模拟确认")
	}

	// 创建SOP工作流引擎
	sopConfig := sop.DefaultEngineConfig()
	sopConfig.EnableFeedback = true
	sopConfig.EnableMetrics = true

	sopEngine = sop.NewWorkflowEngineImpl(sopConfig, contractManager)

	// 更新复合操作处理器的RPC客户端
	if rpcClient != nil {
		sopEngine.CompoundHandler = sop.NewCompoundHandler(sopEngine, rpcClient, sop.TransactionConfig{
			ConfirmationTimeout:   chainConfig.Chain.Transaction.ConfirmationTimeout,
			PollingInterval:       chainConfig.Chain.Transaction.PollingInterval,
			RequiredConfirmations: chainConfig.Chain.Transaction.RequiredConfirmations,
		})
	}
	roleManager = sopEngine.GetRoleManager().(*sop.RoleManagerImpl)

	// 注册标准角色
	registerStandardRolesWithLLM(chain.GetLLMClient())

	log.Println("✅ SOP工作流系统初始化完成")

	// 创建HTTP服务器
	gin.SetMode(gin.ReleaseMode)
	router := gin.Default()

	// 健康检查端点
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":    "healthy",
			"service":   "chain",
			"timestamp": time.Now().Unix(),
		})
	})

	// Chain API端点
	api := router.Group("/api/chain")
	{
		// 处理消息 - 集成SOP工作流
		api.POST("/process", func(c *gin.Context) {
			var req struct {
				Message string `json:"message"`
				Type    string `json:"type,omitempty"` // 工作流类型：swap, stake, compound
			}

			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}

			ctx := context.Background()

			// 确定工作流类型
			workflowType := sop.WorkflowSwap // 默认swap
			if req.Type != "" {
				switch req.Type {
				case "swap":
					workflowType = sop.WorkflowSwap
				case "stake":
					workflowType = sop.WorkflowStake
				case "compound":
					workflowType = sop.WorkflowCompound
				default:
					c.JSON(http.StatusBadRequest, gin.H{"error": "unsupported workflow type"})
					return
				}
			} else {
				// 自动检测工作流类型
				message := req.Message
				if strings.Contains(message, "兑换") && strings.Contains(message, "质押") {
					workflowType = sop.WorkflowCompound
				} else if strings.Contains(message, "兑换") || strings.Contains(message, "swap") {
					workflowType = sop.WorkflowSwap
				} else if strings.Contains(message, "质押") || strings.Contains(message, "stake") {
					workflowType = sop.WorkflowStake
				}
			}

			// 创建用户请求消息
			userRequest := protocol.NewStructuredMessage(
				protocol.MessageTypeUserRequest,
				map[string]interface{}{
					"request":      req.Message,
					"user_address": "0x1234567890123456789012345678901234567890", // 从请求中获取
					"timestamp":    time.Now(),
					"preferences": map[string]interface{}{
						"max_slippage": 0.5,
						"gas_priority": "medium",
						"deadline":     30,
					},
				},
			)

			// 根据配置选择执行引擎
			executionEngine := cfg.Agent.ExecutionEngine
			if executionEngine == "" {
				executionEngine = "auto" // 默认使用auto
			}

			log.Printf("🔧 执行引擎配置: %s", executionEngine)

			var result *qng.ProcessResult
			var err error
			var workflowID string

			switch executionEngine {
			case "sop":
				// 强制使用SOP工作流引擎
				log.Printf("🚀 强制使用SOP工作流引擎执行: %s", workflowType)
				sopResult, err := sopEngine.ExecuteWorkflow(ctx, workflowType, userRequest)
				if err != nil {
					log.Printf("SOP workflow execution failed: %v", err)
					c.JSON(http.StatusInternalServerError, gin.H{"error": "SOP execution failed", "details": err.Error()})
					return
				}
				log.Printf("✅ SOP工作流执行成功")

				workflowID = fmt.Sprintf("sop_workflow_%d", time.Now().UnixNano())

				// 检查SOP结果是否需要签名
				needSignature := false
				var signatureRequest *qng.SignatureRequest
				var workflowContext interface{}

				if sopResult != nil {
					log.Printf("🔍 检查SOP结果: NeedSignature=%v", sopResult.NeedSignature)
					// 检查SOP结果中的签名信息
					if sopResult.NeedSignature && sopResult.SignatureRequest != nil {
						log.Printf("✅ 检测到需要签名，提取签名请求")
						needSignature = true

						// 类型转换：从sop.SignatureRequest转换为qng.SignatureRequest
						sopSigReq := sopResult.SignatureRequest
						signatureRequest = &qng.SignatureRequest{
							Action:    sopSigReq.Action,
							FromToken: sopSigReq.FromToken,
							ToToken:   sopSigReq.ToToken,
							Amount:    sopSigReq.Amount,
							ToAddress: sopSigReq.ToAddress,
							Value:     sopSigReq.Value,
							Data:      sopSigReq.Data,
							GasLimit:  sopSigReq.GasLimit,
							GasPrice:  sopSigReq.GasPrice,
							GasFee:    sopSigReq.GasFee,
							Slippage:  sopSigReq.Slippage,
						}
						workflowContext = sopResult.WorkflowContext
						log.Printf("📝 签名请求: %+v", signatureRequest)
					} else {
						log.Printf("❌ SOP结果中没有检测到签名需求或签名请求为空")
					}
				} else {
					log.Printf("❌ SOP结果为nil")
				}

				result = &qng.ProcessResult{
					NeedSignature:    needSignature,
					SignatureRequest: signatureRequest,
					WorkflowContext:  workflowContext,
					FinalResult:      sopResult,
				}

			case "langgraph":
				// 强制使用LangGraph引擎
				log.Printf("🚀 强制使用LangGraph引擎执行")
				result, err = chain.ProcessMessage(ctx, req.Message)
				if err != nil {
					log.Printf("LangGraph execution failed: %v", err)
					c.JSON(http.StatusInternalServerError, gin.H{"error": "LangGraph execution failed", "details": err.Error()})
					return
				}
				log.Printf("✅ LangGraph执行成功")

				workflowID = fmt.Sprintf("langgraph_workflow_%d", time.Now().UnixNano())

			case "auto", "":
				// 自动选择：优先使用SOP，失败时回退到LangGraph
				log.Printf("🚀 自动选择执行引擎，优先使用SOP: %s", workflowType)
				sopResult, sopErr := sopEngine.ExecuteWorkflow(ctx, workflowType, userRequest)
				if sopErr != nil {
					log.Printf("SOP workflow execution failed: %v", sopErr)
					log.Printf("🔄 回退到LangGraph流程")
					result, err = chain.ProcessMessage(ctx, req.Message)
					if err != nil {
						c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
						return
					}
					workflowID = fmt.Sprintf("langgraph_workflow_%d", time.Now().UnixNano())
					log.Printf("✅ LangGraph执行成功")
				} else {
					log.Printf("✅ SOP工作流执行成功")
					workflowID = fmt.Sprintf("sop_workflow_%d", time.Now().UnixNano())

					// 检查SOP结果是否需要签名
					needSignature := false
					var signatureRequest *qng.SignatureRequest
					var workflowContext interface{}

					if sopResult != nil {
						log.Printf("🔍 检查SOP结果: NeedSignature=%v", sopResult.NeedSignature)
						// 检查SOP结果中的签名信息
						if sopResult.NeedSignature && sopResult.SignatureRequest != nil {
							log.Printf("✅ 检测到需要签名，提取签名请求")
							needSignature = true

							// 类型转换：从sop.SignatureRequest转换为qng.SignatureRequest
							sopSigReq := sopResult.SignatureRequest
							signatureRequest = &qng.SignatureRequest{
								Action:    sopSigReq.Action,
								FromToken: sopSigReq.FromToken,
								ToToken:   sopSigReq.ToToken,
								Amount:    sopSigReq.Amount,
								ToAddress: sopSigReq.ToAddress,
								Value:     sopSigReq.Value,
								Data:      sopSigReq.Data,
								GasLimit:  sopSigReq.GasLimit,
								GasPrice:  sopSigReq.GasPrice,
								GasFee:    sopSigReq.GasFee,
								Slippage:  sopSigReq.Slippage,
							}
							workflowContext = sopResult.WorkflowContext
							log.Printf("📝 签名请求: %+v", signatureRequest)
						} else {
							log.Printf("❌ SOP结果中没有检测到签名需求或签名请求为空")
						}
					} else {
						log.Printf("❌ SOP结果为nil")
					}

					result = &qng.ProcessResult{
						NeedSignature:    needSignature,
						SignatureRequest: signatureRequest,
						WorkflowContext:  workflowContext,
						FinalResult:      sopResult,
					}
				}

			default:
				c.JSON(http.StatusBadRequest, gin.H{"error": "unsupported execution engine", "supported": []string{"sop", "langgraph", "auto"}})
				return
			}

			// 保存工作流结果
			sessionsMu.Lock()
			workflowSessions[workflowID] = result
			sessionsMu.Unlock()

			// 根据是否需要签名确定返回状态
			var status, message string
			if result.NeedSignature {
				status = "waiting_signature"
				message = "等待用户签名授权"
			} else {
				status = "completed"
				message = "工作流执行完成"
			}

			// 返回工作流结果
			c.JSON(http.StatusOK, gin.H{
				"workflow_id": workflowID,
				"session_id":  workflowID,
				"status":      status,
				"message":     message,
				"engine":      executionEngine,
				"result":      result,
			})

		})

		// 获取链状态
		api.GET("/status", func(c *gin.Context) {
			status := map[string]interface{}{
				"running":       true,
				"chain_rpc":     chainConfig.Chain.RPCURL,
				"graph_nodes":   len(chainConfig.Chain.LangGraph.Nodes),
				"poll_interval": 5000, // 默认5秒
				"timestamp":     time.Now().Unix(),
			}

			c.JSON(http.StatusOK, gin.H{"status": status})
		})

		// 获取SOP系统状态
		api.GET("/sop/status", func(c *gin.Context) {
			if sopEngine == nil {
				c.JSON(http.StatusServiceUnavailable, gin.H{"error": "SOP engine not initialized"})
				return
			}

			stats := sopEngine.GetSystemStats()
			recentFeedback := sopEngine.GetRecentFeedback(5)

			status := gin.H{
				"sop_enabled":     true,
				"active_roles":    len(roleManager.ListRoles()),
				"system_stats":    stats,
				"recent_feedback": recentFeedback,
				"timestamp":       time.Now().Unix(),
			}

			c.JSON(http.StatusOK, status)
		})

		// 获取SOP工作流状态
		api.POST("/sop/status", func(c *gin.Context) {
			var req struct {
				ExecutionID string `json:"execution_id"`
			}

			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}

			if sopEngine == nil {
				c.JSON(http.StatusServiceUnavailable, gin.H{"error": "SOP engine not initialized"})
				return
			}

			execCtx, err := sopEngine.GetExecutionStatus(req.ExecutionID)
			if err != nil {
				c.JSON(http.StatusNotFound, gin.H{"error": "execution not found"})
				return
			}

			// 检查是否需要签名
			needsSignature := false
			var signatureRequest *sop.SignatureRequest

			for _, stepState := range execCtx.State {
				if stepState.Status == "waiting_signature" {
					needsSignature = true
					// 构建签名请求
					signatureRequest = &sop.SignatureRequest{
						Action:    "swap",
						FromToken: "MEER",
						ToToken:   "MTK",
						Amount:    "1.0",
						GasLimit:  "1",
						GasPrice:  "10000",
						GasFee:    "0.00021",
						Slippage:  "0.5",
					}
					break
				}
			}

			status := gin.H{
				"execution_id":    req.ExecutionID,
				"status":          "running",
				"needs_signature": needsSignature,
				"steps":           execCtx.State,
			}

			if needsSignature && signatureRequest != nil {
				status["signature_request"] = signatureRequest
			}

			c.JSON(http.StatusOK, status)
		})

		// 继续SOP工作流（带签名）
		api.POST("/sop/continue", func(c *gin.Context) {
			var req struct {
				ExecutionID string `json:"execution_id"`
				Signature   string `json:"signature"`
			}

			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}

			if sopEngine == nil {
				c.JSON(http.StatusServiceUnavailable, gin.H{"error": "SOP engine not initialized"})
				return
			}

			ctx := context.Background()
			result, err := sopEngine.ContinueWithSignatureAndTransaction(ctx, req.ExecutionID, req.Signature, "")
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}

			c.JSON(http.StatusOK, gin.H{
				"execution_id": req.ExecutionID,
				"status":       "completed",
				"result":       result,
			})
		})

		// 获取节点信息
		api.GET("/nodes", func(c *gin.Context) {
			nodes := map[string]interface{}{
				"task_decomposer": map[string]interface{}{
					"name":   "task_decomposer",
					"type":   "llm_processor",
					"status": "active",
				},
				"swap_executor": map[string]interface{}{
					"name":   "swap_executor",
					"type":   "transaction_executor",
					"status": "active",
				},
				"stake_executor": map[string]interface{}{
					"name":   "stake_executor",
					"type":   "transaction_executor",
					"status": "active",
				},
				"signature_validator": map[string]interface{}{
					"name":   "signature_validator",
					"type":   "validator",
					"status": "active",
				},
				"result_aggregator": map[string]interface{}{
					"name":   "result_aggregator",
					"type":   "aggregator",
					"status": "active",
				},
			}

			c.JSON(http.StatusOK, gin.H{"nodes": nodes})
		})

		// 获取工作流状态
		api.POST("/status", func(c *gin.Context) {
			var req struct {
				SessionID string `json:"session_id"`
			}

			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}

			// 从存储中获取工作流状态
			sessionsMu.RLock()
			result, exists := workflowSessions[req.SessionID]
			sessionsMu.RUnlock()
			if !exists {
				c.JSON(http.StatusNotFound, gin.H{"error": "workflow not found"})
				return
			}

			// 根据工作流状态确定响应
			var status gin.H
			if result.NeedSignature {
				status = gin.H{
					"session_id":     req.SessionID,
					"workflow_id":    req.SessionID,
					"status":         "waiting_signature",
					"message":        "等待用户签名授权",
					"need_signature": result.NeedSignature,
				}
			} else {
				// 工作流已完成
				status = gin.H{
					"session_id":  req.SessionID,
					"workflow_id": req.SessionID,
					"status":      "completed",
					"message":     "工作流执行完成",
					"result":      result.FinalResult,
				}
			}

			// 如果需要签名，添加签名请求数据
			if result.NeedSignature && result.SignatureRequest != nil {
				log.Printf("📝 添加签名请求到状态响应: Action=%s, GasLimit=%s, Slippage=%s",
					result.SignatureRequest.Action, result.SignatureRequest.GasLimit, result.SignatureRequest.Slippage)
				status["signature_request"] = gin.H{
					"action":     result.SignatureRequest.Action,
					"from_token": result.SignatureRequest.FromToken,
					"to_token":   result.SignatureRequest.ToToken,
					"amount":     result.SignatureRequest.Amount,
					"to_address": result.SignatureRequest.ToAddress,
					"value":      result.SignatureRequest.Value,
					"data":       result.SignatureRequest.Data,
					"gas_limit":  result.SignatureRequest.GasLimit,
					"gas_price":  result.SignatureRequest.GasPrice,
					"gas_fee":    result.SignatureRequest.GasFee,
					"slippage":   result.SignatureRequest.Slippage,
				}
			} else {
				log.Printf("⚠️  需要签名但签名请求为空: NeedSignature=%v, SignatureRequest=%v",
					result.NeedSignature, result.SignatureRequest != nil)
			}

			c.JSON(http.StatusOK, status)
		})

		// Agent签名端点（转发到chain/continue）
		api.POST("/agent/signature", func(c *gin.Context) {
			var req struct {
				SessionID       string `json:"session_id"`
				Signature       string `json:"signature"`
				TransactionHash string `json:"transaction_hash,omitempty"` // 新增：交易哈希
			}

			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}

			ctx := context.Background()

			// 从存储中获取工作流结果
			sessionsMu.RLock()
			workflowResult, exists := workflowSessions[req.SessionID]
			sessionsMu.RUnlock()
			if !exists {
				c.JSON(http.StatusNotFound, gin.H{"error": "workflow not found"})
				return
			}

			log.Printf("🔍 处理签名请求: session_id=%s, workflowResult=%+v", req.SessionID, workflowResult)

			// 根据工作流类型决定调用哪个引擎
			var result *qng.ProcessResult
			var err error

			// 检查工作流ID前缀来确定执行引擎
			if strings.HasPrefix(req.SessionID, "sop_workflow_") {
				// 使用SOP引擎继续执行
				log.Printf("🔄 使用SOP引擎继续执行: %s", req.SessionID)
				if sopEngine == nil {
					c.JSON(http.StatusServiceUnavailable, gin.H{"error": "SOP engine not initialized"})
					return
				}

				// 从工作流上下文中提取SOP引擎的执行ID
				var executionID string
				if workflowContext, ok := workflowResult.WorkflowContext.(map[string]interface{}); ok {
					if execID, ok := workflowContext["execution_id"].(string); ok {
						executionID = execID
						log.Printf("📝 从工作流上下文中提取SOP执行ID: %s", executionID)
					} else {
						log.Printf("❌ 工作流上下文中没有execution_id字段")
						c.JSON(http.StatusInternalServerError, gin.H{"error": "missing execution_id in workflow context"})
						return
					}
				} else {
					log.Printf("❌ 工作流上下文格式错误")
					c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid workflow context format"})
					return
				}

				sopResult, err := sopEngine.ContinueWithSignatureAndTransaction(ctx, executionID, req.Signature, req.TransactionHash)
				if err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
					return
				}

				// 将SOP结果转换为ProcessResult
				result = &qng.ProcessResult{
					NeedSignature:    sopResult.NeedSignature,
					SignatureRequest: convertSOPSignatureRequest(sopResult.SignatureRequest), // 转换签名请求类型
					WorkflowContext:  sopResult.WorkflowContext,
					FinalResult:      sopResult,
				}
			} else {
				// 使用LangGraph引擎继续执行
				log.Printf("🔄 使用LangGraph引擎继续执行: %s", req.SessionID)
				result, err = chain.ContinueWithSignature(ctx, workflowResult.WorkflowContext, req.Signature)
				if err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
					return
				}
			}

			// 更新工作流会话存储
			sessionsMu.Lock()
			workflowSessions[req.SessionID] = result
			sessionsMu.Unlock()

			c.JSON(http.StatusOK, gin.H{
				"session_id": req.SessionID,
				"status":     "completed",
				"message":    "工作流执行完成",
				"result":     result,
			})
		})

	}

	// Agent API端点
	agentAPI := router.Group("/api/agent")
	{
		// Agent处理端点（转发到chain/process）
		agentAPI.POST("/process", func(c *gin.Context) {
			var req struct {
				Message string `json:"message"`
			}

			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}

			log.Printf("🤖 Agent处理请求: %s", req.Message)

			// 转发到chain/process端点
			ctx := context.Background()

			// 使用自动选择引擎
			var result *qng.ProcessResult
			var err error
			var workflowID string

			// 自动选择：优先使用SOP，失败时回退到LangGraph
			log.Printf("🚀 自动选择执行引擎，优先使用SOP")
			sopResult, sopErr := sopEngine.ExecuteWorkflow(ctx, "compound", &protocol.StructuredMessage{
				Content: map[string]interface{}{
					"user_request": req.Message,
				},
			})

			if sopErr != nil {
				log.Printf("SOP workflow execution failed: %v", sopErr)
				log.Printf("🔄 回退到LangGraph流程")
				result, err = chain.ProcessMessage(ctx, req.Message)
				if err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
					return
				}
				workflowID = fmt.Sprintf("langgraph_workflow_%d", time.Now().UnixNano())
				log.Printf("✅ LangGraph执行成功")
			} else {
				log.Printf("✅ SOP工作流执行成功")
				workflowID = fmt.Sprintf("sop_workflow_%d", time.Now().UnixNano())

				// 检查SOP结果是否需要签名
				needSignature := false
				var signatureRequest *qng.SignatureRequest
				var workflowContext interface{}

				if sopResult != nil {
					log.Printf("🔍 检查SOP结果: NeedSignature=%v", sopResult.NeedSignature)
					if sopResult.NeedSignature && sopResult.SignatureRequest != nil {
						log.Printf("✅ 检测到需要签名，提取签名请求")
						needSignature = true

						// 类型转换：从sop.SignatureRequest转换为qng.SignatureRequest
						sopSigReq := sopResult.SignatureRequest
						signatureRequest = &qng.SignatureRequest{
							Action:    sopSigReq.Action,
							FromToken: sopSigReq.FromToken,
							ToToken:   sopSigReq.ToToken,
							Amount:    sopSigReq.Amount,
							ToAddress: sopSigReq.ToAddress,
							Value:     sopSigReq.Value,
							Data:      sopSigReq.Data,
							GasLimit:  sopSigReq.GasLimit,
							GasPrice:  sopSigReq.GasPrice,
							GasFee:    sopSigReq.GasFee,
							Slippage:  sopSigReq.Slippage,
						}
						workflowContext = sopResult.WorkflowContext
						log.Printf("📝 签名请求: %+v", signatureRequest)
					}
				}

				result = &qng.ProcessResult{
					NeedSignature:    needSignature,
					SignatureRequest: signatureRequest,
					WorkflowContext:  workflowContext,
					FinalResult:      sopResult,
				}
			}

			// 保存工作流结果
			sessionsMu.Lock()
			workflowSessions[workflowID] = result
			sessionsMu.Unlock()

			// 根据是否需要签名确定返回状态
			var status, message string
			if result.NeedSignature {
				status = "waiting_signature"
				message = "等待用户签名授权"
			} else {
				status = "completed"
				message = "工作流执行完成"
			}

			// 返回工作流结果
			c.JSON(http.StatusOK, gin.H{
				"workflow_id": workflowID,
				"session_id":  workflowID,
				"status":      status,
				"message":     message,
				"engine":      "auto",
				"result":      result,
			})
		})

		// Agent轮询端点（转发到chain/status）
		agentAPI.GET("/poll/:sessionId", func(c *gin.Context) {
			sessionId := c.Param("sessionId")

			log.Printf("🔄 Agent轮询状态: %s", sessionId)

			// 从存储中获取工作流结果
			sessionsMu.RLock()
			workflowResult, exists := workflowSessions[sessionId]
			sessionsMu.RUnlock()

			if !exists {
				c.JSON(http.StatusNotFound, gin.H{"error": "session not found"})
				return
			}

			// 构建状态响应
			status := gin.H{
				"session_id": sessionId,
				"status":     "running",
				"result":     workflowResult,
			}

			// 如果需要签名，添加签名请求到状态响应
			if workflowResult.NeedSignature && workflowResult.SignatureRequest != nil {
				status["signature_request"] = workflowResult.SignatureRequest
				log.Printf("📝 添加签名请求到状态响应: %+v", workflowResult.SignatureRequest)
			}

			c.JSON(http.StatusOK, status)
		})

		// Agent签名端点（转发到chain/continue）
		agentAPI.POST("/signature", func(c *gin.Context) {
			var req struct {
				SessionID       string `json:"session_id"`
				Signature       string `json:"signature"`
				TransactionHash string `json:"transaction_hash,omitempty"` // 新增：交易哈希
			}

			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}

			ctx := context.Background()

			// 从存储中获取工作流结果
			sessionsMu.RLock()
			workflowResult, exists := workflowSessions[req.SessionID]
			sessionsMu.RUnlock()
			if !exists {
				c.JSON(http.StatusNotFound, gin.H{"error": "workflow not found"})
				return
			}

			log.Printf("🔍 处理签名请求: session_id=%s, workflowResult=%+v", req.SessionID, workflowResult)

			// 根据工作流类型决定调用哪个引擎
			var result *qng.ProcessResult
			var err error

			// 检查工作流ID前缀来确定执行引擎
			if strings.HasPrefix(req.SessionID, "sop_workflow_") {
				// 使用SOP引擎继续执行
				log.Printf("🔄 使用SOP引擎继续执行: %s", req.SessionID)
				if sopEngine == nil {
					c.JSON(http.StatusServiceUnavailable, gin.H{"error": "SOP engine not initialized"})
					return
				}

				// 从工作流上下文中提取SOP引擎的执行ID
				var executionID string
				if workflowContext, ok := workflowResult.WorkflowContext.(map[string]interface{}); ok {
					if execID, ok := workflowContext["execution_id"].(string); ok {
						executionID = execID
						log.Printf("📝 从工作流上下文中提取SOP执行ID: %s", executionID)
					} else {
						log.Printf("❌ 工作流上下文中没有execution_id字段")
						c.JSON(http.StatusInternalServerError, gin.H{"error": "missing execution_id in workflow context"})
						return
					}
				} else {
					log.Printf("❌ 工作流上下文格式错误")
					c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid workflow context format"})
					return
				}

				sopResult, err := sopEngine.ContinueWithSignatureAndTransaction(ctx, executionID, req.Signature, req.TransactionHash)
				if err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
					return
				}

				// 将SOP结果转换为ProcessResult
				result = &qng.ProcessResult{
					NeedSignature:    sopResult.NeedSignature,
					SignatureRequest: convertSOPSignatureRequest(sopResult.SignatureRequest), // 转换签名请求类型
					WorkflowContext:  sopResult.WorkflowContext,
					FinalResult:      sopResult,
				}
			} else {
				// 使用LangGraph引擎继续执行
				log.Printf("🔄 使用LangGraph引擎继续执行: %s", req.SessionID)
				result, err = chain.ContinueWithSignature(ctx, workflowResult.WorkflowContext, req.Signature)
				if err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
					return
				}
			}

			// 更新工作流会话存储
			sessionsMu.Lock()
			workflowSessions[req.SessionID] = result
			sessionsMu.Unlock()

			c.JSON(http.StatusOK, gin.H{
				"session_id": req.SessionID,
				"status":     "completed",
				"message":    "工作流执行完成",
				"result":     result,
			})
		})

		// 继续工作流（带签名）
		api.POST("/continue", func(c *gin.Context) {
			var req struct {
				SessionID       string `json:"session_id"`
				Signature       string `json:"signature"`
				TransactionHash string `json:"transaction_hash,omitempty"` // 新增：交易哈希
			}

			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}

			ctx := context.Background()

			// 从存储中获取工作流结果
			sessionsMu.RLock()
			workflowResult, exists := workflowSessions[req.SessionID]
			sessionsMu.RUnlock()
			if !exists {
				c.JSON(http.StatusNotFound, gin.H{"error": "workflow not found"})
				return
			}

			// 根据工作流类型决定调用哪个引擎
			var result *qng.ProcessResult
			var err error

			// 检查工作流ID前缀来确定执行引擎
			if strings.HasPrefix(req.SessionID, "sop_workflow_") {
				// 使用SOP引擎继续执行
				log.Printf("🔄 使用SOP引擎继续执行: %s", req.SessionID)
				if sopEngine == nil {
					c.JSON(http.StatusServiceUnavailable, gin.H{"error": "SOP engine not initialized"})
					return
				}

				// 从工作流上下文中提取SOP引擎的执行ID
				var executionID string
				if workflowContext, ok := workflowResult.WorkflowContext.(map[string]interface{}); ok {
					if execID, ok := workflowContext["execution_id"].(string); ok {
						executionID = execID
						log.Printf("📝 从工作流上下文中提取SOP执行ID: %s", executionID)
					} else {
						log.Printf("❌ 工作流上下文中没有execution_id字段")
						c.JSON(http.StatusInternalServerError, gin.H{"error": "missing execution_id in workflow context"})
						return
					}
				} else {
					log.Printf("❌ 工作流上下文格式错误")
					c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid workflow context format"})
					return
				}

				sopResult, err := sopEngine.ContinueWithSignatureAndTransaction(ctx, executionID, req.Signature, req.TransactionHash)
				if err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
					return
				}

				// 将SOP结果转换为ProcessResult
				result = &qng.ProcessResult{
					NeedSignature:    sopResult.NeedSignature,
					SignatureRequest: convertSOPSignatureRequest(sopResult.SignatureRequest), // 转换签名请求类型
					WorkflowContext:  sopResult.WorkflowContext,
					FinalResult:      sopResult,
				}
			} else {
				// 使用LangGraph引擎继续执行
				log.Printf("🔄 使用LangGraph引擎继续执行: %s", req.SessionID)
				result, err = chain.ContinueWithSignature(ctx, workflowResult.WorkflowContext, req.Signature)
				if err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
					return
				}
			}

			// 更新工作流会话存储
			sessionsMu.Lock()
			workflowSessions[req.SessionID] = result
			sessionsMu.Unlock()

			c.JSON(http.StatusOK, gin.H{
				"session_id": req.SessionID,
				"status":     "completed",
				"message":    "工作流执行完成",
				"result":     result,
			})
		})
	}

	// 启动HTTP服务器
	server := &http.Server{
		Addr:    ":9092",
		Handler: router,
	}

	go func() {
		log.Printf("🚀 Chain服务启动在端口: %d", 9092)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("Failed to start Chain server:", err)
		}
	}()

	// 启动状态监控
	log.Println("🎯 启动状态监控...")
	go func() {
		pollInterval := 5000 // 默认5秒
		ticker := time.NewTicker(time.Duration(pollInterval) * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				log.Printf("📊 链状态监控 - 间隔: %dms", pollInterval)
				// 这里可以添加更多的监控逻辑
			}
		}
	}()

	log.Println("✅ QNG Chain服务已启动")
	log.Printf("📡 监控间隔: %dms", 5000)
	log.Printf("🌐 图节点数: %d", len(chainConfig.Chain.LangGraph.Nodes))

	// 启动健康检查
	registry.StartHealthCheck()

	// 等待中断信号
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	log.Println("Chain服务正在运行，按 Ctrl+C 停止")
	<-c

	log.Println("正在关闭Chain服务...")

	// 注销服务
	registry.UnregisterService("chain")

	// 关闭Chain
	if err := chain.Stop(); err != nil {
		log.Printf("关闭Chain服务时出错: %v", err)
	}

	// 关闭HTTP服务器
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Printf("Chain服务关闭失败: %v", err)
	}

	log.Println("Chain服务已关闭")
}
