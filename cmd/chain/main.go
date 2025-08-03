package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"qng_agent/internal/config"
	"qng_agent/internal/qng"
	"qng_agent/internal/service"
	"sync"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
)

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
		// 处理消息
		api.POST("/process", func(c *gin.Context) {
			var req struct {
				Message string `json:"message"`
			}

			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}

			ctx := context.Background()
			result, err := chain.ProcessMessage(ctx, req.Message)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}

			// 生成工作流ID
			workflowID := fmt.Sprintf("workflow_%d", time.Now().UnixNano())

			// 保存工作流结果
			sessionsMu.Lock()
			workflowSessions[workflowID] = result
			sessionsMu.Unlock()

			// 返回包含工作流ID的响应
			response := gin.H{
				"workflow_id": workflowID,
				"session_id":  workflowID, // 为兼容性添加
				"status":      "pending",
				"message":     "工作流已提交，正在处理中...",
				"result":      result,
			}

			c.JSON(http.StatusOK, response)
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
			}

			c.JSON(http.StatusOK, status)
		})

		// 继续工作流（带签名）
		api.POST("/continue", func(c *gin.Context) {
			var req struct {
				SessionID string `json:"session_id"`
				Signature string `json:"signature"`
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

			// 使用保存的工作流上下文
			result, err := chain.ContinueWithSignature(ctx, workflowResult.WorkflowContext, req.Signature)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
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
