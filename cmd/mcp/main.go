package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"qng_agent/internal/config"
	"qng_agent/internal/mcp"
	"qng_agent/internal/protocol"
	"qng_agent/internal/service"
	"time"

	"github.com/gin-gonic/gin"
)

// 全局协议系统变量
var (
	messagePool *protocol.MessagePool
	validator   *protocol.Web3MessageValidator
	serializer  *protocol.MessageSerializer
)

func main() {
	log.Println("=== QNG MCP 服务启动 (SSE模式) ===")

	// 加载配置
	cfg := config.LoadConfig("config/config.yaml")
	if cfg == nil {
		log.Fatal("Failed to load config")
	}

	// 初始化协议系统
	log.Println("🔧 初始化协议系统...")
	messagePool = protocol.NewMessagePool()
	validator = protocol.NewWeb3MessageValidator()
	serializer = protocol.NewMessageSerializer(1024) // 1KB压缩阈值
	log.Println("✅ 协议系统初始化完成")

	// 获取服务注册中心
	registry := service.GetRegistry()

	// 注册MCP服务
	mcpService := &service.ServiceInfo{
		Name:     "mcp",
		Address:  "localhost",
		Port:     9091,
		Status:   "running",
		LastSeen: time.Now(),
		Endpoints: []string{
			"/api/mcp/call",
			"/api/mcp/qng/workflow",
			"/api/mcp/capabilities",
			"/api/mcp/events", // SSE端点
		},
		Metadata: map[string]string{
			"type":     "mcp_service",
			"version":  "1.0.0",
			"protocol": "sse",
		},
	}

	if err := registry.RegisterService(mcpService); err != nil {
		log.Fatal("Failed to register MCP service:", err)
	}

	log.Println("✅ MCP服务已注册到服务注册中心")

	// 初始化MCP服务器
	mcpServer := mcp.NewServer(cfg.MCP)
	log.Println("✅ MCP服务器初始化成功")

	// 启动MCP服务器
	if err := mcpServer.Start(); err != nil {
		log.Fatal("Failed to start MCP server:", err)
	}
	defer mcpServer.Stop()

	log.Println("📋 服务架构说明:")
	log.Println("  - MCP服务以SSE模式运行")
	log.Println("  - 支持实时事件推送")
	log.Println("  - 兼容标准MCP协议")

	// 创建HTTP服务器
	gin.SetMode(gin.ReleaseMode)
	router := gin.Default()

	// 添加CORS中间件
	router.Use(func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	})

	// 健康检查端点
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":    "healthy",
			"service":   "mcp",
			"protocol":  "sse",
			"timestamp": time.Now().Unix(),
		})
	})

	// API路由
	api := router.Group("/api/mcp")
	{
		// 通用MCP调用
		api.POST("/call", func(c *gin.Context) {
			var req struct {
				Server string                 `json:"server"`
				Method string                 `json:"method"`
				Params map[string]interface{} `json:"params"`
			}

			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}

			ctx := c.Request.Context()
			result, err := mcpServer.Call(ctx, req.Server, req.Method, req.Params)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}

			c.JSON(http.StatusOK, gin.H{"result": result})
		})

		// QNG工作流
		api.POST("/qng/workflow", func(c *gin.Context) {
			var req struct {
				Message string `json:"message"`
			}

			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}

			ctx := c.Request.Context()
			result, err := mcpServer.Call(ctx, "qng", "execute_workflow", map[string]any{
				"message": req.Message,
			})
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}

			resMap, _ := result.(map[string]any)
			workflowID, _ := resMap["workflow_id"].(string)
			c.JSON(http.StatusOK, gin.H{"workflow_id": workflowID})
		})

		// 获取工作流状态
		api.GET("/qng/workflow/:id/status", func(c *gin.Context) {
			workflowID := c.Param("id")

			ctx := c.Request.Context()
			result, err := mcpServer.Call(ctx, "qng", "get_session_status", map[string]any{
				"session_id": workflowID,
			})
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}

			c.JSON(http.StatusOK, result)
		})

		// 提交签名
		api.POST("/qng/workflow/:id/signature", func(c *gin.Context) {
			workflowID := c.Param("id")

			var req struct {
				Signature string `json:"signature"`
			}

			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}

			ctx := c.Request.Context()
			result, err := mcpServer.Call(ctx, "qng", "submit_signature", map[string]any{
				"session_id": workflowID,
				"signature":  req.Signature,
			})
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}

			c.JSON(http.StatusOK, gin.H{"result": result})
		})

		// 获取能力
		api.GET("/capabilities", func(c *gin.Context) {
			capabilities := mcpServer.GetCapabilities()
			c.JSON(http.StatusOK, gin.H{"capabilities": capabilities})
		})

		// 处理结构化消息
		api.POST("/protocol/message", func(c *gin.Context) {
			var req struct {
				Type    string                 `json:"type"`
				Content map[string]interface{} `json:"content"`
			}

			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}

			// 创建结构化消息
			messageType := protocol.MessageType(req.Type)
			message := protocol.NewStructuredMessage(messageType, req.Content)

			// 验证消息
			if err := validator.ValidateMessage(message); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "message validation failed", "details": err.Error()})
				return
			}

			// 发布消息到消息池
			if err := messagePool.Publish(message); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to publish message"})
				return
			}

			// 序列化消息
			serialized, err := serializer.Serialize(message)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to serialize message"})
				return
			}

			c.JSON(http.StatusOK, gin.H{
				"message_id": message.ID,
				"status":     "published",
				"size":       len(serialized),
				"timestamp":  message.Timestamp.Unix(),
			})
		})

		// 获取消息统计
		api.GET("/protocol/stats", func(c *gin.Context) {
			stats := gin.H{
				"total_messages": len(messagePool.GetMessages([]protocol.MessageType{})),
				"message_types": map[string]int{
					"UserRequest":      len(messagePool.GetMessages([]protocol.MessageType{protocol.MessageTypeUserRequest})),
					"Strategy":         len(messagePool.GetMessages([]protocol.MessageType{protocol.MessageTypeStrategy})),
					"RiskAssessment":   len(messagePool.GetMessages([]protocol.MessageType{protocol.MessageTypeRiskAssessment})),
					"Transaction":      len(messagePool.GetMessages([]protocol.MessageType{protocol.MessageTypeTransaction})),
					"ExecutionResult":  len(messagePool.GetMessages([]protocol.MessageType{protocol.MessageTypeExecutionResult})),
					"FinalReport":      len(messagePool.GetMessages([]protocol.MessageType{protocol.MessageTypeFinalReport})),
				},
				"timestamp": time.Now().Unix(),
			}

			c.JSON(http.StatusOK, stats)
		})

		// SSE事件流端点
		api.GET("/events", func(c *gin.Context) {
			// 设置SSE头部
			c.Header("Content-Type", "text/event-stream")
			c.Header("Cache-Control", "no-cache")
			c.Header("Connection", "keep-alive")
			c.Header("Access-Control-Allow-Origin", "*")
			c.Header("Access-Control-Allow-Headers", "Cache-Control")

			// 创建事件通道
			eventChan := make(chan string)
			defer close(eventChan)

			// 监听客户端断开连接
			notify := c.Writer.CloseNotify()
			go func() {
				<-notify
				log.Println("SSE客户端断开连接")
			}()

			// 发送初始连接事件
			initialEvent := map[string]interface{}{
				"type":    "connected",
				"message": "SSE连接已建立",
				"time":    time.Now().Unix(),
			}
			initialData, _ := json.Marshal(initialEvent)
			fmt.Fprintf(c.Writer, "data: %s\n\n", initialData)
			c.Writer.Flush()

			// 持续发送事件
			ticker := time.NewTicker(30 * time.Second)
			defer ticker.Stop()

			for {
				select {
				case <-c.Request.Context().Done():
					return
				case <-ticker.C:
					// 发送心跳事件
					heartbeatEvent := map[string]interface{}{
						"type": "heartbeat",
						"time": time.Now().Unix(),
					}
					heartbeatData, _ := json.Marshal(heartbeatEvent)
					fmt.Fprintf(c.Writer, "data: %s\n\n", heartbeatData)
					c.Writer.Flush()
				case event := <-eventChan:
					// 发送自定义事件
					fmt.Fprintf(c.Writer, "data: %s\n\n", event)
					c.Writer.Flush()
				}
			}
		})

		// 工作流状态SSE端点
		api.GET("/workflow/:id/events", func(c *gin.Context) {
			workflowID := c.Param("id")

			// 设置SSE头部
			c.Header("Content-Type", "text/event-stream")
			c.Header("Cache-Control", "no-cache")
			c.Header("Connection", "keep-alive")
			c.Header("Access-Control-Allow-Origin", "*")

			// 监听客户端断开连接
			notify := c.Writer.CloseNotify()
			go func() {
				<-notify
				log.Printf("工作流 %s 的SSE客户端断开连接", workflowID)
			}()

			// 定期检查工作流状态
			ticker := time.NewTicker(2 * time.Second)
			defer ticker.Stop()

			for {
				select {
				case <-c.Request.Context().Done():
					return
				case <-ticker.C:
					ctx := c.Request.Context()
					result, err := mcpServer.Call(ctx, "qng", "get_session_status", map[string]any{
						"session_id": workflowID,
					})
					if err != nil {
						errorEvent := map[string]interface{}{
							"type":  "error",
							"error": err.Error(),
							"time":  time.Now().Unix(),
						}
						errorData, _ := json.Marshal(errorEvent)
						fmt.Fprintf(c.Writer, "data: %s\n\n", errorData)
						c.Writer.Flush()
						continue
					}

					statusEvent := map[string]interface{}{
						"type":        "status_update",
						"workflow_id": workflowID,
						"status":      result,
						"time":        time.Now().Unix(),
					}
					statusData, _ := json.Marshal(statusEvent)
					fmt.Fprintf(c.Writer, "data: %s\n\n", statusData)
					c.Writer.Flush()

					// 如果工作流完成，停止发送事件
					if statusMap, ok := result.(map[string]interface{}); ok {
						if status, exists := statusMap["status"]; exists {
							if statusStr, ok := status.(string); ok {
								if statusStr == "completed" || statusStr == "failed" {
									completionEvent := map[string]interface{}{
										"type":        "workflow_completed",
										"workflow_id": workflowID,
										"status":      statusStr,
										"time":        time.Now().Unix(),
									}
									completionData, _ := json.Marshal(completionEvent)
									fmt.Fprintf(c.Writer, "data: %s\n\n", completionData)
									c.Writer.Flush()
									return
								}
							}
						}
					}
				}
			}
		})
	}

	// 启动服务器
	addr := ":9091"
	log.Printf("MCP服务启动在 %s (SSE模式)", addr)
	if err := router.Run(addr); err != nil {
		log.Fatal("Failed to start MCP server:", err)
	}
}
