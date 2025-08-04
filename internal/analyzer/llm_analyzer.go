package analyzer

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// ContractAnalysis 合约分析结果
type ContractAnalysis struct {
	Type            string                    `json:"type"`
	Description     string                    `json:"description"`
	Functions       map[string]FunctionConfig `json:"functions"`
	SupportedPairs  []PairConfig              `json:"supportedPairs,omitempty"`
	StakingInfo     *StakingInfoConfig        `json:"stakingInfo,omitempty"`
}

// AnalyzeContract 分析合约源码和ABI
func (lca *LLMContractAnalyzer) AnalyzeContract(contractName, sourceCode string, abi json.RawMessage) (*ContractAnalysis, error) {
	// 解析ABI
	var abiData []map[string]interface{}
	if err := json.Unmarshal(abi, &abiData); err != nil {
		return nil, fmt.Errorf("failed to parse ABI: %w", err)
	}

	// 分析合约类型和描述
	contractType, description := lca.analyzeContractType(contractName, sourceCode)

	// 分析函数
	functions := lca.analyzeFunctions(abiData, sourceCode)

	analysis := &ContractAnalysis{
		Type:        contractType,
		Description: description,
		Functions:   functions,
	}

	// 根据合约类型添加特定分析
	switch contractName {
	case "SimpleSwap":
		analysis.SupportedPairs = lca.analyzeSwapPairs(sourceCode)
	case "MTKStaking":
		analysis.StakingInfo = lca.analyzeStakingInfo(sourceCode)
	}

	return analysis, nil
}

// analyzeContractType 分析合约类型
func (lca *LLMContractAnalyzer) analyzeContractType(contractName, sourceCode string) (string, string) {
	switch contractName {
	case "SimpleSwap":
		return "DEX", "Simple token swap contract for ETH <-> MTK"
	case "MTKStaking":
		return "Staking", "MTK token staking contract with rewards"
	default:
		return "Unknown", "Unknown contract type"
	}
}

// analyzeFunctions 分析合约函数
func (lca *LLMContractAnalyzer) analyzeFunctions(abi []map[string]interface{}, sourceCode string) map[string]FunctionConfig {
	functions := make(map[string]FunctionConfig)

	for _, item := range abi {
		if item["type"] == "function" {
			name := item["name"].(string)
			stateMutability := item["stateMutability"].(string)
			
			// 跳过构造函数和内部函数
			if name == "" || strings.HasPrefix(name, "_") {
				continue
			}

			// 分析函数参数
			parameters := lca.analyzeFunctionParameters(item)
			
			// 分析函数描述
			description := lca.analyzeFunctionDescription(name, sourceCode)
			
			// 分析是否为payable
			payable := stateMutability == "payable"
			
			// 分析兑换率（针对swap合约）
			exchangeRate := lca.analyzeExchangeRate(name, sourceCode)

			functions[name] = FunctionConfig{
				Signature:    lca.generateFunctionSignature(item),
				Description:  description,
				Parameters:   parameters,
				Payable:      payable,
				ExchangeRate: exchangeRate,
			}
		}
	}

	return functions
}

// analyzeFunctionParameters 分析函数参数
func (lca *LLMContractAnalyzer) analyzeFunctionParameters(function map[string]interface{}) []ParameterConfig {
	var parameters []ParameterConfig

	if inputs, ok := function["inputs"].([]interface{}); ok {
		for _, input := range inputs {
			inputMap := input.(map[string]interface{})
			paramName := inputMap["name"].(string)
			paramType := inputMap["type"].(string)
			
			// 生成参数描述
			description := lca.generateParameterDescription(paramName, paramType)
			
			parameters = append(parameters, ParameterConfig{
				Name:        paramName,
				Type:        paramType,
				Description: description,
			})
		}
	}

	return parameters
}

// generateFunctionSignature 生成函数签名
func (lca *LLMContractAnalyzer) generateFunctionSignature(function map[string]interface{}) string {
	name := function["name"].(string)
	
	var params []string
	if inputs, ok := function["inputs"].([]interface{}); ok {
		for _, input := range inputs {
			inputMap := input.(map[string]interface{})
			paramType := inputMap["type"].(string)
			params = append(params, paramType)
		}
	}
	
	if len(params) == 0 {
		return name + "()"
	}
	
	return name + "(" + strings.Join(params, ",") + ")"
}

// analyzeFunctionDescription 分析函数描述
func (lca *LLMContractAnalyzer) analyzeFunctionDescription(functionName, sourceCode string) string {
	// 基于函数名和源码分析描述
	switch functionName {
	case "buyToken":
		return "Buy MTK tokens with ETH"
	case "sellToken":
		return "Sell MTK tokens for ETH"
	case "stake":
		return "Stake MTK tokens to earn rewards"
	case "unstake":
		return "Unstake MTK tokens"
	case "claimRewards":
		return "Claim accumulated staking rewards"
	case "depositToken":
		return "Deposit tokens to contract (admin function)"
	case "withdrawETH":
		return "Withdraw ETH from contract (admin function)"
	default:
		return "Contract function"
	}
}

// generateParameterDescription 生成参数描述
func (lca *LLMContractAnalyzer) generateParameterDescription(paramName, paramType string) string {
	switch paramName {
	case "amount":
		return "Amount of tokens"
	case "tokenAmount":
		return "Amount of tokens to sell"
	case "tokenAddress":
		return "Token contract address"
	default:
		return fmt.Sprintf("%s parameter of type %s", paramName, paramType)
	}
}

// analyzeExchangeRate 分析兑换率
func (lca *LLMContractAnalyzer) analyzeExchangeRate(functionName, sourceCode string) string {
	if functionName == "buyToken" {
		// 从源码中提取兑换率
		rateRegex := regexp.MustCompile(`rate\s*=\s*(\d+)`)
		if match := rateRegex.FindStringSubmatch(sourceCode); len(match) > 1 {
			rate := match[1]
			return fmt.Sprintf("1 ETH = %s MTK", rate)
		}
		return "1 ETH = 1000 MTK"
	}
	
	if functionName == "sellToken" {
		return "1000 MTK = 1 ETH"
	}
	
	return ""
}

// analyzeSwapPairs 分析交换对
func (lca *LLMContractAnalyzer) analyzeSwapPairs(sourceCode string) []PairConfig {
	// 从源码中提取兑换率
	rateRegex := regexp.MustCompile(`rate\s*=\s*(\d+)`)
	var rate float64 = 1000 // 默认值
	if match := rateRegex.FindStringSubmatch(sourceCode); len(match) > 1 {
		if parsed, err := fmt.Sscanf(match[1], "%f", &rate); err == nil && parsed > 0 {
			// 使用解析的值
		}
	}

	return []PairConfig{
		{
			From:        "MEER",
			To:          "MTK",
			Method:      "buyToken",
			Rate:        rate,
			Description: fmt.Sprintf("Convert MEER to MTK at 1:%g rate", rate),
		},
		{
			From:        "MTK",
			To:          "MEER",
			Method:      "sellToken",
			Rate:        1.0 / rate,
			Description: fmt.Sprintf("Convert MTK to MEER at %g:1 rate", rate),
		},
	}
}

// analyzeStakingInfo 分析质押信息
func (lca *LLMContractAnalyzer) analyzeStakingInfo(sourceCode string) *StakingInfoConfig {
	// 从源码中提取质押信息
	minStakeRegex := regexp.MustCompile(`minStakeAmount\s*=\s*(\d+)`)
	rewardRateRegex := regexp.MustCompile(`rewardRate\s*=\s*(\d+)`)
	
	var minStakeAmount string = "1.0"
	var rewardRate string = "100 wei per day per MTK"
	
	if match := minStakeRegex.FindStringSubmatch(sourceCode); len(match) > 1 {
		minStakeAmount = match[1]
	}
	
	if match := rewardRateRegex.FindStringSubmatch(sourceCode); len(match) > 1 {
		rewardRate = match[1] + " wei per day per MTK"
	}

	return &StakingInfoConfig{
		StakingToken:   "MTK",
		RewardToken:    "MTK",
		MinStakeAmount: minStakeAmount,
		RewardRate:     rewardRate,
		Description:    "Stake MTK tokens to earn MTK rewards",
	}
} 