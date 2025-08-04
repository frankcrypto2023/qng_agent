package qng

import (
	"context"
	"fmt"
	"log"
	"qng_agent/internal/config"
	"qng_agent/internal/contracts"
	"qng_agent/internal/llm"
	"qng_agent/internal/rpc"
	"sync"
)

// ChainConfig 链配置结构
type ChainConfig struct {
	Enabled     bool
	Host        string
	Port        int
	Timeout     int
	Network     string
	RPCURL      string
	Transaction TransactionConfig
	LangGraph   LangGraphConfig
	LLM         LLMConfig
	Chain       ChainSubConfig
}

// ChainSubConfig 链子配置
type ChainSubConfig struct {
	Enabled     bool
	Network     string
	RPCURL      string
	Transaction TransactionConfig
	LangGraph   LangGraphConfig
	LLM         LLMConfig
}

// TransactionConfig 交易配置
type TransactionConfig struct {
	ConfirmationTimeout   int
	PollingInterval       int
	RequiredConfirmations int
}

// LangGraphConfig LangGraph配置
type LangGraphConfig struct {
	Enabled bool
	Nodes   []string
}

// LLMConfig LLM配置
type LLMConfig struct {
	Provider   string
	OpenAI     OpenAIConfig
	Gemini     GeminiConfig
	Anthropic  AnthropicConfig
	ModelScope ModelScopeConfig
}

// OpenAIConfig OpenAI配置
type OpenAIConfig struct {
	APIKey    string
	Model     string
	BaseURL   string
	Timeout   int
	MaxTokens int
}

// GeminiConfig Gemini配置
type GeminiConfig struct {
	APIKey  string
	Model   string
	Timeout int
}

// AnthropicConfig Anthropic配置
type AnthropicConfig struct {
	APIKey  string
	Model   string
	Timeout int
}

// ModelScopeConfig ModelScope配置
type ModelScopeConfig struct {
	APIKey    string
	Model     string
	BaseURL   string
	Timeout   int
	MaxTokens int
}

type Chain struct {
	config          ChainConfig
	llmClient       llm.Client
	contractManager *contracts.ContractManager
	rpcClient       *rpc.Client
	langGraph       *LangGraph
	mu              sync.RWMutex
	running         bool
}

// SignatureRequest 签名请求结构
type SignatureRequest struct {
	Action    string `json:"action"`
	FromToken string `json:"from_token"`
	ToToken   string `json:"to_token"`
	Amount    string `json:"amount"`
	ToAddress string `json:"to_address"`
	Value     string `json:"value"`
	Data      string `json:"data"`
	GasLimit  string `json:"gas_limit"`
	GasPrice  string `json:"gas_price"`
	GasFee    string `json:"gas_fee"`
	Slippage  string `json:"slippage"`
}

type ProcessResult struct {
	NeedSignature    bool              `json:"need_signature"`
	SignatureRequest *SignatureRequest `json:"signature_request,omitempty"`
	WorkflowContext  any               `json:"workflow_context,omitempty"`
	FinalResult      any               `json:"final_result,omitempty"`
}

func NewChain(chainConfig ChainConfig) *Chain {
	// 创建LLM客户端
	var llmClient llm.Client
	var err error

	// 从配置中获取LLM配置
	if chainConfig.Chain.LLM.Provider != "" {
		// 转换为config.LLMConfig
		llmConfig := config.LLMConfig{
			Provider: chainConfig.Chain.LLM.Provider,
			OpenAI: config.OpenAIConfig{
				APIKey:    chainConfig.Chain.LLM.OpenAI.APIKey,
				Model:     chainConfig.Chain.LLM.OpenAI.Model,
				BaseURL:   chainConfig.Chain.LLM.OpenAI.BaseURL,
				Timeout:   chainConfig.Chain.LLM.OpenAI.Timeout,
				MaxTokens: chainConfig.Chain.LLM.OpenAI.MaxTokens,
			},
			Gemini: config.GeminiConfig{
				APIKey:  chainConfig.Chain.LLM.Gemini.APIKey,
				Model:   chainConfig.Chain.LLM.Gemini.Model,
				Timeout: chainConfig.Chain.LLM.Gemini.Timeout,
			},
			Anthropic: config.AnthropicConfig{
				APIKey:  chainConfig.Chain.LLM.Anthropic.APIKey,
				Model:   chainConfig.Chain.LLM.Anthropic.Model,
				Timeout: chainConfig.Chain.LLM.Anthropic.Timeout,
			},
			ModelScope: config.ModelScopeConfig{
				APIKey:    chainConfig.Chain.LLM.ModelScope.APIKey,
				Model:     chainConfig.Chain.LLM.ModelScope.Model,
				BaseURL:   chainConfig.Chain.LLM.ModelScope.BaseURL,
				Timeout:   chainConfig.Chain.LLM.ModelScope.Timeout,
				MaxTokens: chainConfig.Chain.LLM.ModelScope.MaxTokens,
			},
		}
		llmClient, err = llm.NewClient(llmConfig)
		if err != nil {
			log.Printf("⚠️  无法创建LLM客户端: %v", err)
			llmClient = nil
		}
	}

	// 创建合约管理器
	contractManager, err := contracts.NewContractManager("config/contracts.json")
	if err != nil {
		log.Printf("⚠️  无法创建合约管理器: %v", err)
		contractManager = nil
	}

	// 创建RPC客户端
	var rpcClient *rpc.Client
	if chainConfig.Chain.RPCURL != "" {
		rpcClient = rpc.NewClient(chainConfig.Chain.RPCURL)
		log.Printf("✅ RPC客户端已创建: %s", chainConfig.Chain.RPCURL)
	} else {
		log.Printf("⚠️  未配置RPC URL，使用模拟确认")
	}
	// fmt.Println("------------", chainConfig.Chain.LLM.Provider)
	// 创建LangGraph
	langGraph := NewLangGraph(llmClient, contractManager, rpcClient, chainConfig.Chain.Transaction)

	chain := &Chain{
		config:          chainConfig,
		llmClient:       llmClient,
		contractManager: contractManager,
		rpcClient:       rpcClient,
		langGraph:       langGraph,
	}

	return chain
}

func (c *Chain) Start() error {
	log.Printf("🚀 QNG Chain启动")
	c.running = true
	return nil
}

func (c *Chain) Stop() error {
	log.Printf("🛑 QNG Chain停止")
	c.running = false
	return nil
}

// GetLLMClient 获取LLM客户端
func (c *Chain) GetLLMClient() llm.Client {
	return c.llmClient
}

func (c *Chain) ProcessMessage(ctx context.Context, message string) (*ProcessResult, error) {
	log.Printf("🔄 QNG Chain开始处理消息")
	log.Printf("📝 消息内容: %s", message)

	if !c.running {
		log.Printf("❌ Chain未运行")
		return nil, fmt.Errorf("chain is not running")
	}

	// 使用LangGraph执行工作流
	result, err := c.langGraph.ExecuteWorkflow(ctx, message)
	if err != nil {
		log.Printf("❌ LangGraph执行失败: %v", err)
		return nil, fmt.Errorf("langgraph execution failed: %w", err)
	}

	log.Printf("✅ LangGraph执行成功")
	return result, nil
}

func (c *Chain) ContinueWithSignature(ctx context.Context, workflowContext any, signature string) (*ProcessResult, error) {
	log.Printf("🔄 QNG Chain使用签名继续工作流")
	log.Printf("🔐 签名长度: %d", len(signature))

	result, err := c.langGraph.ContinueWithSignature(ctx, workflowContext, signature)
	if err != nil {
		log.Printf("❌ 继续执行失败: %v", err)
		return nil, fmt.Errorf("continue with signature failed: %w", err)
	}

	log.Printf("✅ 继续执行成功")
	return result, nil
}
