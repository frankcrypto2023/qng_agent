package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"qng_agent/internal/config"
	"sync"
	"time"
)

type Server struct {
	config         config.MCPConfig
	qngServer      *QNGServer
	metamaskServer *MetaMaskServer
	mu             sync.RWMutex
	running        bool
}

func NewServer(config config.MCPConfig) *Server {
	log.Printf("🔧 创建MCP服务器")

	server := &Server{
		config: config,
	}

	// 使用新的servers配置
	if config.Servers != nil {
		log.Printf("📋 使用新的servers配置")

		// 初始化QNG服务器
		if qngConfig, exists := config.Servers["qng"]; exists && qngConfig.Enabled {
			log.Printf("🔧 初始化QNG MCP服务器")
			log.Printf("📋 将调用外部Chain服务 (http://localhost:9092)")
			// 不创建内部QNG服务器，而是调用外部Chain服务
		} else {
			log.Printf("⚠️  QNG服务未启用")
		}

		// 初始化MetaMask服务器
		if metamaskConfig, exists := config.Servers["metamask"]; exists && metamaskConfig.Enabled {
			log.Printf("🔧 初始化MetaMask MCP服务器")
			log.Printf("⚠️  暂时跳过MetaMask服务器初始化（配置类型问题）")
		} else {
			log.Printf("⚠️  MetaMask服务未启用")
		}
	} else {
		log.Printf("⚠️  没有配置任何MCP服务器")
	}

	return server
}

func (s *Server) Start() error {
	log.Printf("🚀 MCP服务器启动")

	s.mu.Lock()
	s.running = true
	s.mu.Unlock()

	// 启动QNG服务器
	if s.qngServer != nil {
		log.Printf("🚀 启动QNG MCP服务器")
		if err := s.qngServer.Start(); err != nil {
			log.Printf("❌ 启动QNG服务器失败: %v", err)
			return fmt.Errorf("failed to start QNG server: %w", err)
		}
		log.Printf("✅ QNG MCP服务器启动成功")
	}

	// 启动MetaMask服务器
	if s.metamaskServer != nil {
		log.Printf("🚀 启动MetaMask MCP服务器")
		if err := s.metamaskServer.Start(); err != nil {
			log.Printf("❌ 启动MetaMask服务器失败: %v", err)
			return fmt.Errorf("failed to start MetaMask server: %w", err)
		}
		log.Printf("✅ MetaMask MCP服务器启动成功")
	}

	log.Printf("✅ MCP服务器启动完成")
	return nil
}

func (s *Server) Stop() error {
	log.Printf("🛑 MCP服务器停止")

	s.mu.Lock()
	s.running = false
	s.mu.Unlock()

	// 停止QNG服务器
	if s.qngServer != nil {
		log.Printf("🛑 停止QNG MCP服务器")
		if err := s.qngServer.Stop(); err != nil {
			log.Printf("❌ 停止QNG服务器失败: %v", err)
		} else {
			log.Printf("✅ QNG MCP服务器停止成功")
		}
	}

	// 停止MetaMask服务器
	if s.metamaskServer != nil {
		log.Printf("🛑 停止MetaMask MCP服务器")
		if err := s.metamaskServer.Stop(); err != nil {
			log.Printf("❌ 停止MetaMask服务器失败: %v", err)
		} else {
			log.Printf("✅ MetaMask MCP服务器停止成功")
		}
	}

	log.Printf("✅ MCP服务器停止完成")
	return nil
}

func (s *Server) Call(ctx context.Context, service string, method string, params map[string]any) (any, error) {
	log.Printf("🔄 MCP服务器调用")
	log.Printf("🔧 服务: %s", service)
	log.Printf("🛠️  方法: %s", method)
	log.Printf("📋 参数: %+v", params)

	s.mu.RLock()
	if !s.running {
		s.mu.RUnlock()
		log.Printf("❌ MCP服务器未运行")
		return nil, fmt.Errorf("MCP server is not running")
	}
	s.mu.RUnlock()

	switch service {
	case "qng":
		log.Printf("🔄 调用外部Chain服务")
		return s.callChainService(ctx, method, params)

	case "metamask":
		if s.metamaskServer == nil {
			log.Printf("❌ MetaMask服务未启用")
			return nil, fmt.Errorf("MetaMask service not enabled")
		}
		log.Printf("🔄 调用MetaMask服务")
		return s.metamaskServer.Call(ctx, method, params)

	default:
		log.Printf("❌ 未知服务: %s", service)
		return nil, fmt.Errorf("unknown service: %s", service)
	}
}

func (s *Server) GetCapabilities() map[string][]Capability {
	log.Printf("📋 获取MCP服务器能力")

	capabilities := make(map[string][]Capability)

	// QNG服务能力（通过外部Chain服务）
	if qngConfig, exists := s.config.Servers["qng"]; exists && qngConfig.Enabled {
		log.Printf("📋 获取QNG服务能力（外部Chain服务）")
		capabilities["qng"] = []Capability{
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
		}
	}

	// MetaMask服务能力
	if s.metamaskServer != nil {
		log.Printf("📋 获取MetaMask服务能力")
		capabilities["metamask"] = s.metamaskServer.GetDetailedCapabilities()
	}

	log.Printf("✅ 返回 %d 个服务的能力", len(capabilities))
	return capabilities
}

func (s *Server) GetServices() []string {
	log.Printf("📋 获取可用服务列表")

	services := make([]string, 0)

	if s.qngServer != nil {
		services = append(services, "qng")
		log.Printf("✅ QNG服务可用")
	}

	if s.metamaskServer != nil {
		services = append(services, "metamask")
		log.Printf("✅ MetaMask服务可用")
	}

	log.Printf("📋 可用服务: %v", services)
	return services
}

// callChainService 调用外部Chain服务
func (s *Server) callChainService(ctx context.Context, method string, params map[string]any) (any, error) {
	log.Printf("🌐 调用Chain服务: %s", method)

	// 构建请求URL
	chainURL := "http://localhost:9092/api/chain"

	// 根据方法构建不同的请求
	var reqBody map[string]any
	var endpoint string

	switch method {
	case "execute_workflow":
		message, ok := params["message"].(string)
		if !ok {
			return nil, fmt.Errorf("message parameter required")
		}
		reqBody = map[string]any{
			"message": message,
		}
		endpoint = "/process"

	case "get_session_status":
		sessionID, ok := params["session_id"].(string)
		if !ok {
			return nil, fmt.Errorf("session_id parameter required")
		}
		reqBody = map[string]any{
			"session_id": sessionID,
		}
		endpoint = "/status"

	case "submit_signature":
		sessionID, ok := params["session_id"].(string)
		if !ok {
			return nil, fmt.Errorf("session_id parameter required")
		}
		signature, ok := params["signature"].(string)
		if !ok {
			return nil, fmt.Errorf("signature parameter required")
		}
		reqBody = map[string]any{
			"session_id": sessionID,
			"signature":  signature,
		}
		endpoint = "/continue"

	default:
		return nil, fmt.Errorf("unsupported method: %s", method)
	}

	// 发送HTTP请求
	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	log.Printf("🌐 请求URL: %s%s", chainURL, endpoint)
	log.Printf("📤 请求数据: %s", string(jsonData))

	req, err := http.NewRequestWithContext(ctx, "POST", chainURL+endpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call chain service: %w", err)
	}
	defer resp.Body.Close()

	log.Printf("🔍 响应状态码: %d", resp.StatusCode)

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		log.Printf("❌ Chain服务错误响应: %s", string(body))
		return nil, fmt.Errorf("chain service error: %s - %s", resp.Status, string(body))
	}

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	log.Printf("✅ Chain服务调用成功")
	return result, nil
}
