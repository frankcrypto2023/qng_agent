package analyzer

import (
	"fmt"
	"regexp"
	"strings"
)

// ContractAnalyzer 基于ABI的合约分析器
type ContractAnalyzer struct {
	contractsConfig  *ContractsConfig
	compoundAnalyzer *CompoundAnalyzer
}

// ContractsConfig 合约配置结构
type ContractsConfig struct {
	Version   string                    `json:"version"`
	Network   NetworkConfig             `json:"network"`
	Tokens    map[string]TokenConfig    `json:"tokens"`
	Contracts map[string]ContractConfig `json:"contracts"`
	Workflows map[string]WorkflowConfig `json:"workflows"`
}

type NetworkConfig struct {
	ChainID int    `json:"chainId"`
	Name    string `json:"name"`
	RPCURL  string `json:"rpcUrl"`
}

type TokenConfig struct {
	Name            string `json:"name"`
	Symbol          string `json:"symbol"`
	Decimals        int    `json:"decimals"`
	IsNative        bool   `json:"isNative"`
	ContractAddress string `json:"contractAddress,omitempty"`
	ContractName    string `json:"contractName,omitempty"`
	Description     string `json:"description"`
}

type ContractConfig struct {
	Name           string                    `json:"name"`
	Address        string                    `json:"address"`
	ArtifactPath   string                    `json:"artifactPath"`
	Type           string                    `json:"type"`
	Description    string                    `json:"description"`
	Functions      map[string]FunctionConfig `json:"functions"`
	SupportedPairs []PairConfig              `json:"supportedPairs,omitempty"`
	StakingInfo    *StakingInfoConfig        `json:"stakingInfo,omitempty"`
}

type FunctionConfig struct {
	Signature    string            `json:"signature"`
	Description  string            `json:"description"`
	Parameters   []ParameterConfig `json:"parameters"`
	Payable      bool              `json:"payable,omitempty"`
	ExchangeRate string            `json:"exchangeRate,omitempty"`
}

type ParameterConfig struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Description string `json:"description"`
}

type PairConfig struct {
	From        string  `json:"from"`
	To          string  `json:"to"`
	Method      string  `json:"method"`
	Rate        float64 `json:"rate"`
	Description string  `json:"description"`
}

type StakingInfoConfig struct {
	StakingToken   string `json:"stakingToken"`
	RewardToken    string `json:"rewardToken"`
	MinStakeAmount string `json:"minStakeAmount"`
	RewardRate     string `json:"rewardRate"`
	Description    string `json:"description"`
}

type WorkflowConfig struct {
	Description     string   `json:"description"`
	SupportedPairs  []string `json:"supportedPairs,omitempty"`
	SupportedTokens []string `json:"supportedTokens,omitempty"`
	Contract        string   `json:"contract"`
	Patterns        []string `json:"patterns"`
}

// NewContractAnalyzer 创建合约分析器
func NewContractAnalyzer(configPath string) (*ContractAnalyzer, error) {
	config, err := loadContractsConfig(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load contracts config: %w", err)
	}

	analyzer := &ContractAnalyzer{
		contractsConfig: config,
	}

	// 初始化复合操作分析器
	analyzer.compoundAnalyzer = NewCompoundAnalyzer(analyzer)

	return analyzer, nil
}

// loadContractsConfig 加载合约配置
func loadContractsConfig(configPath string) (*ContractsConfig, error) {
	// 使用动态合约加载器
	loader := NewContractLoader("artifacts", "deployed.json", "contracts", configPath)
	config, err := loader.LoadContractsConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load contracts config: %w", err)
	}
	return config, nil
}

// AnalyzeUserRequest 分析用户请求
func (ca *ContractAnalyzer) AnalyzeUserRequest(userRequest string) (*AnalysisResult, error) {
	// 检查是否为复合操作
	if ca.compoundAnalyzer.IsCompoundRequest(userRequest) {
		return ca.analyzeCompoundRequest(userRequest)
	}

	result := &AnalysisResult{
		UserRequest: userRequest,
		Operations:  []Operation{},
	}

	// 1. 识别工作流类型
	workflowType := ca.identifyWorkflowType(userRequest)
	result.WorkflowType = workflowType

	// 2. 提取操作参数
	params := ca.extractParameters(userRequest, workflowType)
	result.Parameters = params

	// 3. 生成操作列表
	operations := ca.generateOperations(workflowType, params)
	result.Operations = operations

	// 4. 验证操作可行性
	validation := ca.validateOperations(operations)
	result.Validation = validation

	return result, nil
}

// analyzeCompoundRequest 分析复合请求
func (ca *ContractAnalyzer) analyzeCompoundRequest(userRequest string) (*AnalysisResult, error) {
	// 使用复合分析器分析请求
	compoundOp, err := ca.compoundAnalyzer.AnalyzeCompoundRequest(userRequest)
	if err != nil {
		return nil, fmt.Errorf("failed to analyze compound request: %w", err)
	}

	// 验证所有操作
	validation := ca.validateOperations(compoundOp.Operations)

	// 构建参数，包含签名请求信息
	params := map[string]interface{}{
		"sequence": compoundOp.Sequence,
	}

	// 如果需要签名，添加签名请求信息
	if compoundOp.NeedAuth && compoundOp.AuthRequest != nil {
		params["need_signature"] = true
		params["signature_request"] = compoundOp.AuthRequest
		params["total_gas"] = compoundOp.TotalGas
	}

	// 确保操作列表被正确设置，这样SOP引擎的needsSignature方法才能检测到
	operations := compoundOp.Operations
	if len(operations) == 0 {
		// 如果没有生成操作，创建一个占位操作以确保签名请求被处理
		operations = []Operation{
			{
				Type:        "compound",
				Description: "复合操作需要签名",
				GasEstimate: compoundOp.TotalGas,
			},
		}
	}

	result := &AnalysisResult{
		UserRequest:  userRequest,
		WorkflowType: "compound",
		Parameters:   params,
		Operations:   operations,
		Validation:   validation,
	}

	return result, nil
}

// AnalysisResult 分析结果
type AnalysisResult struct {
	UserRequest  string                 `json:"user_request"`
	WorkflowType string                 `json:"workflow_type"`
	Parameters   map[string]interface{} `json:"parameters"`
	Operations   []Operation            `json:"operations"`
	Validation   *ValidationResult      `json:"validation"`
}

// Operation 操作定义
type Operation struct {
	Type        string                 `json:"type"`
	Contract    string                 `json:"contract"`
	Method      string                 `json:"method"`
	Parameters  map[string]interface{} `json:"parameters"`
	Description string                 `json:"description"`
	GasEstimate uint64                 `json:"gas_estimate"`
}

// ValidationResult 验证结果
type ValidationResult struct {
	Valid    bool     `json:"valid"`
	Errors   []string `json:"errors"`
	Warnings []string `json:"warnings"`
}

// identifyWorkflowType 识别工作流类型
func (ca *ContractAnalyzer) identifyWorkflowType(userRequest string) string {
	userRequest = strings.ToLower(userRequest)

	fmt.Printf("DEBUG: 识别工作流类型，请求: %s\n", userRequest)
	fmt.Printf("DEBUG: 可用工作流数量: %d\n", len(ca.contractsConfig.Workflows))

	for workflowType, config := range ca.contractsConfig.Workflows {
		fmt.Printf("DEBUG: 检查工作流 '%s'，模式数量: %d\n", workflowType, len(config.Patterns))
		for i, pattern := range config.Patterns {
			// 不要将模式转换为小写，保持原始大小写
			matched := ca.matchPattern(userRequest, pattern)
			fmt.Printf("DEBUG: 工作流 '%s' 模式 %d '%s' 匹配: %t\n", workflowType, i+1, pattern, matched)
			if matched {
				fmt.Printf("DEBUG: 匹配成功，返回工作流类型: %s\n", workflowType)
				return workflowType
			}
		}
	}

	fmt.Printf("DEBUG: 未找到匹配的工作流，返回 unknown\n")
	return "unknown"
}

// matchPattern 匹配模式
func (ca *ContractAnalyzer) matchPattern(userRequest, pattern string) bool {
	// 将模式转换为正则表达式，处理占位符
	regexPattern := ca.convertPatternToRegex(pattern)

	// 编译正则表达式，设置不区分大小写标志
	re, err := regexp.Compile("(?i)" + regexPattern)
	if err != nil {
		fmt.Printf("DEBUG: 正则表达式编译错误: %v\n", err)
		return false
	}

	matched := re.MatchString(userRequest)
	fmt.Printf("DEBUG: 正则匹配 '%s' 对 '%s': %t\n", regexPattern, userRequest, matched)
	return matched
}

// convertPatternToRegex 将模式转换为正则表达式
func (ca *ContractAnalyzer) convertPatternToRegex(pattern string) string {
	// 先转义特殊字符
	regex := regexp.QuoteMeta(pattern)

	// 然后替换占位符为正确的正则表达式
	regex = strings.ReplaceAll(regex, `\{amount\}`, `\d+(?:\.\d+)?`)
	regex = strings.ReplaceAll(regex, `\{fromToken\}`, `(MEER|MTK)`)
	regex = strings.ReplaceAll(regex, `\{toToken\}`, `(MEER|MTK)`)
	regex = strings.ReplaceAll(regex, `\{token\}`, `(MEER|MTK)`)

	fmt.Printf("DEBUG: 模式转换 '%s' -> '%s'\n", pattern, regex)
	return regex
}

// extractParameters 提取参数
func (ca *ContractAnalyzer) extractParameters(userRequest, workflowType string) map[string]interface{} {
	params := make(map[string]interface{})

	// 提取金额
	amountRegex := regexp.MustCompile(`(\d+(?:\.\d+)?)\s*(MEER|MTK)`)
	matches := amountRegex.FindStringSubmatch(userRequest)
	if len(matches) >= 3 {
		params["amount"] = matches[1]
		params["token"] = matches[2]
	}

	// 提取代币对
	pairRegex := regexp.MustCompile(`(MEER|MTK)\s*(?:兑换|换成|to|for)\s*(MEER|MTK)`)
	matches = pairRegex.FindStringSubmatch(userRequest)
	if len(matches) >= 3 {
		params["fromToken"] = matches[1]
		params["toToken"] = matches[2]
	}

	return params
}

// generateOperations 生成操作列表
func (ca *ContractAnalyzer) generateOperations(workflowType string, params map[string]interface{}) []Operation {
	var operations []Operation

	workflow, exists := ca.contractsConfig.Workflows[workflowType]
	if !exists {
		return operations
	}

	contract, exists := ca.contractsConfig.Contracts[workflow.Contract]
	if !exists {
		return operations
	}

	switch workflowType {
	case "swap":
		operations = ca.generateSwapOperations(contract, params)
	case "stake":
		operations = ca.generateStakeOperations(contract, params)
	case "unstake":
		operations = ca.generateUnstakeOperations(contract, params)
	case "claimRewards":
		operations = ca.generateClaimRewardsOperations(contract, params)
	}

	return operations
}

// generateSwapOperations 生成兑换操作
func (ca *ContractAnalyzer) generateSwapOperations(contract ContractConfig, params map[string]interface{}) []Operation {
	var operations []Operation

	fromToken, _ := params["fromToken"].(string)
	toToken, _ := params["toToken"].(string)
	amount, _ := params["amount"].(string)

	// 查找匹配的代币对
	for _, pair := range contract.SupportedPairs {
		if pair.From == fromToken && pair.To == toToken {
			operation := Operation{
				Type:        "swap",
				Contract:    contract.Address,
				Method:      pair.Method,
				Parameters:  map[string]interface{}{"amount": amount},
				Description: pair.Description,
				GasEstimate: 150000, // 预估Gas
			}
			operations = append(operations, operation)
			break
		}
	}

	return operations
}

// generateStakeOperations 生成质押操作
func (ca *ContractAnalyzer) generateStakeOperations(contract ContractConfig, params map[string]interface{}) []Operation {
	var operations []Operation

	amount, _ := params["amount"].(string)

	if function, exists := contract.Functions["stake"]; exists {
		operation := Operation{
			Type:        "stake",
			Contract:    contract.Address,
			Method:      function.Signature,
			Parameters:  map[string]interface{}{"amount": amount},
			Description: function.Description,
			GasEstimate: 100000, // 预估Gas
		}
		operations = append(operations, operation)
	}

	return operations
}

// generateUnstakeOperations 生成解质押操作
func (ca *ContractAnalyzer) generateUnstakeOperations(contract ContractConfig, params map[string]interface{}) []Operation {
	var operations []Operation

	amount, _ := params["amount"].(string)

	if function, exists := contract.Functions["unstake"]; exists {
		operation := Operation{
			Type:        "unstake",
			Contract:    contract.Address,
			Method:      function.Signature,
			Parameters:  map[string]interface{}{"amount": amount},
			Description: function.Description,
			GasEstimate: 80000, // 预估Gas
		}
		operations = append(operations, operation)
	}

	return operations
}

// generateClaimRewardsOperations 生成领取奖励操作
func (ca *ContractAnalyzer) generateClaimRewardsOperations(contract ContractConfig, params map[string]interface{}) []Operation {
	var operations []Operation

	if function, exists := contract.Functions["claimRewards"]; exists {
		operation := Operation{
			Type:        "claimRewards",
			Contract:    contract.Address,
			Method:      function.Signature,
			Parameters:  map[string]interface{}{},
			Description: function.Description,
			GasEstimate: 60000, // 预估Gas
		}
		operations = append(operations, operation)
	}

	return operations
}

// validateOperations 验证操作可行性
func (ca *ContractAnalyzer) validateOperations(operations []Operation) *ValidationResult {
	result := &ValidationResult{
		Valid:    true,
		Errors:   []string{},
		Warnings: []string{},
	}

	for _, op := range operations {
		// 检查合约是否存在
		if _, exists := ca.contractsConfig.Contracts[op.Contract]; !exists {
			result.Valid = false
			result.Errors = append(result.Errors, fmt.Sprintf("Contract %s not found", op.Contract))
		}

		// 检查方法是否存在
		contract := ca.contractsConfig.Contracts[op.Contract]
		if _, exists := contract.Functions[op.Method]; !exists {
			result.Valid = false
			result.Errors = append(result.Errors, fmt.Sprintf("Method %s not found in contract %s", op.Method, op.Contract))
		}
	}

	return result
}
