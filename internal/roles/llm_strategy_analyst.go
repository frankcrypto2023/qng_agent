// internal/roles/llm_strategy_analyst.go
package roles

import (
	"context"
	"encoding/json"
	"fmt"
	"qng_agent/internal/analyzer"
	"qng_agent/internal/llm"
	"qng_agent/internal/protocol"
	"strings"
	"time"
)

// LLMStrategyAnalyst 基于LLM的策略分析师角色
type LLMStrategyAnalyst struct {
	AbstractRole
	llmClient        llm.Client
	contractAnalyzer *analyzer.ContractAnalyzer
}

// NewLLMStrategyAnalyst 创建基于LLM的策略分析师
func NewLLMStrategyAnalyst(llmClient llm.Client) *LLMStrategyAnalyst {
	// 创建合约分析器
	contractAnalyzer, err := analyzer.NewContractAnalyzer("config/contracts.json")
	if err != nil {
		// 如果创建失败，使用nil，后续会回退到LLM分析
		contractAnalyzer = nil
	}

	return &LLMStrategyAnalyst{
		AbstractRole: AbstractRole{
			Profile: RoleProfile{
				Type:        RoleStrategyAnalyst,
				Name:        "LLM Strategy Analyst",
				Description: "Analyzes user requests using LLM and formulates Web3 strategies",
				Goal:        "Provide intelligent strategy recommendations for Web3 operations",
				Constraints: []string{"Follow security best practices", "Consider gas optimization", "Validate user intent"},
				Skills:      []string{"LLM-powered analysis", "DeFi protocols analysis", "Token economics", "Market analysis"},
			},
		},
		llmClient:        llmClient,
		contractAnalyzer: contractAnalyzer,
	}
}

// Act 执行策略分析
func (sa *LLMStrategyAnalyst) Act(ctx context.Context, input *protocol.StructuredMessage) (*protocol.StructuredMessage, error) {
	// 提取用户请求
	userRequest, ok := input.Content["request"].(string)
	if !ok {
		return nil, fmt.Errorf("invalid user request format")
	}

	// 优先使用合约分析器
	if sa.contractAnalyzer != nil {
		analysis, err := sa.contractAnalyzer.AnalyzeUserRequest(userRequest)
		if err == nil && analysis.Validation.Valid {
			// 使用合约分析器的结果
			return sa.buildOutputFromAnalysis(userRequest, analysis)
		}
	}

	// 回退到LLM分析
	return sa.analyzeWithLLM(ctx, userRequest)
}

// buildOutputFromAnalysis 从合约分析结果构建输出
func (sa *LLMStrategyAnalyst) buildOutputFromAnalysis(userRequest string, analysis *analyzer.AnalysisResult) (*protocol.StructuredMessage, error) {
	// 转换操作为interface{}格式
	operations := make([]interface{}, len(analysis.Operations))
	for i, op := range analysis.Operations {
		operations[i] = map[string]interface{}{
			"type":         op.Type,
			"contract":     op.Contract,
			"method":       op.Method,
			"parameters":   op.Parameters,
			"description":  op.Description,
			"gas_estimate": op.GasEstimate,
		}
	}

	output := protocol.NewStructuredMessage(
		protocol.MessageTypeStrategy,
		map[string]interface{}{
			"user_intent": map[string]interface{}{
				"original_request": userRequest,
				"analyzed_at":      time.Now(),
				"intent_type":      analysis.WorkflowType,
				"actions":          []string{analysis.WorkflowType},
			},
			"strategy": map[string]interface{}{
				"analysis":        fmt.Sprintf("基于合约ABI分析，识别为%s操作", analysis.WorkflowType),
				"recommendations": []string{"操作已验证", "合约地址有效", "方法签名正确"},
				"risk_assessment": "低风险，基于已验证的合约ABI",
				"estimated_gas":   "基于合约方法预估",
				"estimated_cost":  "基于当前Gas价格预估",
			},
			"operations":      operations,
			"recommendations": []string{"操作已验证", "合约地址有效", "方法签名正确"},
		},
	)
	output.SenderRole = string(sa.Profile.Type)

	return output, nil
}

// analyzeWithLLM 使用LLM进行分析
func (sa *LLMStrategyAnalyst) analyzeWithLLM(ctx context.Context, userRequest string) (*protocol.StructuredMessage, error) {
	// 构建LLM提示
	prompt := sa.buildAnalysisPrompt(userRequest)

	// 调用LLM进行分析
	messages := []llm.Message{
		{
			Role:    "user",
			Content: prompt,
		},
	}
	response, err := sa.llmClient.Chat(ctx, messages)
	if err != nil {
		return nil, fmt.Errorf("LLM analysis failed: %w", err)
	}

	// 解析LLM响应
	strategy, err := sa.parseLLMResponse(response)
	if err != nil {
		return nil, fmt.Errorf("failed to parse LLM response: %w", err)
	}

	// 构建输出消息
	output := protocol.NewStructuredMessage(
		protocol.MessageTypeStrategy,
		map[string]interface{}{
			"user_intent": map[string]interface{}{
				"original_request": userRequest,
				"analyzed_at":      time.Now(),
				"intent_type":      strategy.IntentType,
				"actions":          strategy.Actions,
			},
			"strategy": map[string]interface{}{
				"analysis":        strategy.Analysis,
				"recommendations": strategy.Recommendations,
				"risk_assessment": strategy.RiskAssessment,
				"estimated_gas":   strategy.EstimatedGas,
				"estimated_cost":  strategy.EstimatedCost,
			},
			"operations":      convertToInterfaceSlice(strategy.Operations),
			"recommendations": strategy.Recommendations,
		},
	)
	output.SenderRole = string(sa.Profile.Type)

	return output, nil
}

// buildAnalysisPrompt 构建分析提示
func (sa *LLMStrategyAnalyst) buildAnalysisPrompt(userRequest string) string {
	return fmt.Sprintf(`你是一个专业的Web3策略分析师。请分析以下用户请求并提供详细的策略建议：

用户请求: %s

请以JSON格式返回分析结果，包含以下字段：
{
  "intent_type": "操作类型 (swap/stake/compound)",
  "actions": ["具体操作步骤"],
  "operations": [
    {
      "type": "swap",
      "from_token": "MEER",
      "to_token": "MTK",
      "amount": "1.0",
      "description": "兑换MEER为MTK"
    },
    {
      "type": "stake",
      "token": "MTK",
      "amount": "兑换获得的MTK数量",
      "description": "质押MTK"
    }
  ],
  "analysis": "详细分析",
  "recommendations": ["建议列表"],
  "risk_assessment": "风险评估",
  "estimated_gas": "预估Gas费用",
  "estimated_cost": "预估总成本"
}

请根据用户请求的具体内容生成相应的operations数组。对于复合操作（如兑换+质押），请包含多个操作步骤。
请确保分析准确、安全且实用。`, userRequest)
}

// StrategyAnalysisResult 策略分析结果
type StrategyAnalysisResult struct {
	IntentType      string                   `json:"intent_type"`
	Actions         []string                 `json:"actions"`
	Operations      []map[string]interface{} `json:"operations"`
	Analysis        string                   `json:"analysis"`
	Recommendations []string                 `json:"recommendations"`
	RiskAssessment  string                   `json:"risk_assessment"`
	EstimatedGas    string                   `json:"estimated_gas"`
	EstimatedCost   string                   `json:"estimated_cost"`
}

// parseLLMResponse 解析LLM响应
func (sa *LLMStrategyAnalyst) parseLLMResponse(response string) (*StrategyAnalysisResult, error) {
	// 尝试提取JSON部分
	jsonStart := strings.Index(response, "{")
	jsonEnd := strings.LastIndex(response, "}")

	if jsonStart == -1 || jsonEnd == -1 {
		return nil, fmt.Errorf("no JSON found in LLM response")
	}

	jsonStr := response[jsonStart : jsonEnd+1]

	var result StrategyAnalysisResult
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	return &result, nil
}

// convertToInterfaceSlice 将[]map[string]interface{}转换为[]interface{}
func convertToInterfaceSlice(operations []map[string]interface{}) []interface{} {
	result := make([]interface{}, len(operations))
	for i, op := range operations {
		result[i] = op
	}
	return result
}
