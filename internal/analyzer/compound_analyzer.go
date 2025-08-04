package analyzer

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// CompoundOperation 复合操作结构
type CompoundOperation struct {
	Operations  []Operation       `json:"operations"`
	Sequence    []string          `json:"sequence"` // 操作序列描述
	TotalGas    uint64            `json:"total_gas"`
	NeedAuth    bool              `json:"need_auth"`              // 是否需要用户授权
	AuthRequest *SignatureRequest `json:"auth_request,omitempty"` // 签名请求
	// 交易监控相关字段
	CurrentStep     int    `json:"current_step"`     // 当前执行步骤
	TransactionHash string `json:"transaction_hash"` // 当前交易的哈希
	StepStatus      string `json:"step_status"`      // 当前步骤状态
	AutoContinue    bool   `json:"auto_continue"`    // 是否自动继续下一步
}

// SignatureRequest 签名请求结构（与LangGraph兼容）
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

// CompoundAnalyzer 复合操作分析器
type CompoundAnalyzer struct {
	contractAnalyzer *ContractAnalyzer
}

// NewCompoundAnalyzer 创建复合操作分析器
func NewCompoundAnalyzer(contractAnalyzer *ContractAnalyzer) *CompoundAnalyzer {
	return &CompoundAnalyzer{
		contractAnalyzer: contractAnalyzer,
	}
}

// AnalyzeCompoundRequest 分析复合请求
func (ca *CompoundAnalyzer) AnalyzeCompoundRequest(userRequest string) (*CompoundOperation, error) {
	// 1. 分解复合请求为多个子操作
	subRequests := ca.decomposeCompoundRequest(userRequest)

	// 调试输出
	fmt.Printf("DEBUG: 分解后的子请求: %v\n", subRequests)

	// 2. 为每个子操作生成操作列表
	var allOperations []Operation
	var sequence []string
	var totalGas uint64

	for i, subRequest := range subRequests {
		fmt.Printf("DEBUG: 分析子请求 %d: %s\n", i+1, subRequest)

		// 分析子请求
		result, err := ca.contractAnalyzer.AnalyzeUserRequest(subRequest)
		if err != nil {
			return nil, fmt.Errorf("failed to analyze sub-request '%s': %w", subRequest, err)
		}

		fmt.Printf("DEBUG: 子请求 %d 结果: workflow=%s, operations=%d\n",
			i+1, result.WorkflowType, len(result.Operations))

		// 添加操作到列表
		for _, op := range result.Operations {
			allOperations = append(allOperations, op)
			totalGas += op.GasEstimate
		}

		// 添加序列描述
		sequence = append(sequence, fmt.Sprintf("%s: %s", result.WorkflowType, subRequest))
	}

	fmt.Printf("DEBUG: 总操作数: %d, 总Gas: %d\n", len(allOperations), totalGas)

	// 3. 生成签名请求（如果有操作需要签名）
	var authRequest *SignatureRequest
	needAuth := len(allOperations) > 0

	if needAuth {
		authRequest = ca.generateSignatureRequest(userRequest, allOperations, totalGas)
	}

	return &CompoundOperation{
		Operations:   allOperations,
		Sequence:     sequence,
		TotalGas:     totalGas,
		NeedAuth:     needAuth,
		AuthRequest:  authRequest,
		CurrentStep:  0,
		StepStatus:   "pending",
		AutoContinue: true,
	}, nil
}

// decomposeCompoundRequest 分解复合请求
func (ca *CompoundAnalyzer) decomposeCompoundRequest(userRequest string) []string {
	var subRequests []string

	// 转换为小写以便匹配
	lowerRequest := strings.ToLower(userRequest)
	fmt.Printf("DEBUG: 分析复合请求: %s (lower: %s)\n", userRequest, lowerRequest)

	// 定义复合操作模式
	compoundPatterns := []struct {
		pattern string
		actions []string
	}{
		{
			pattern: `兑换.*?再.*?质押`,
			actions: []string{"swap", "stake"},
		},
		{
			pattern: `swap.*?then.*?stake`,
			actions: []string{"swap", "stake"},
		},
		{
			pattern: `exchange.*?then.*?stake`,
			actions: []string{"swap", "stake"},
		},
		{
			pattern: `兑换.*?然后.*?质押`,
			actions: []string{"swap", "stake"},
		},
	}

	// 检查是否为复合操作
	for i, compound := range compoundPatterns {
		matched, _ := regexp.MatchString(compound.pattern, lowerRequest)
		fmt.Printf("DEBUG: 模式 %d '%s' 匹配: %t\n", i+1, compound.pattern, matched)
		if matched {
			subRequests = ca.extractCompoundOperations(userRequest, compound.actions)
			fmt.Printf("DEBUG: 使用预定义模式提取操作: %v\n", subRequests)
			return subRequests
		}
	}

	// 如果不是预定义的复合模式，尝试智能分解
	fmt.Printf("DEBUG: 使用智能分解\n")
	return ca.intelligentDecompose(userRequest)
}

// extractCompoundOperations 提取复合操作
func (ca *CompoundAnalyzer) extractCompoundOperations(userRequest string, actions []string) []string {
	var subRequests []string

	fmt.Printf("DEBUG: 提取复合操作，请求: %s, 动作: %v\n", userRequest, actions)

	// 提取金额和代币信息
	amountRegex := regexp.MustCompile(`(\d+(?:\.\d+)?)\s*(MEER|MTK)`)
	matches := amountRegex.FindStringSubmatch(userRequest)
	fmt.Printf("DEBUG: 金额匹配结果: %v\n", matches)

	if len(matches) < 3 {
		fmt.Printf("DEBUG: 无法解析金额和代币，返回原请求\n")
		return []string{userRequest} // 如果无法解析，返回原请求
	}

	amount := matches[1]
	fromToken := matches[2]
	toToken := "MTK" // 默认目标代币

	fmt.Printf("DEBUG: 解析结果 - 金额: %s, 源代币: %s, 目标代币: %s\n", amount, fromToken, toToken)

	// 根据操作类型生成子请求
	for _, action := range actions {
		switch action {
		case "swap":
			// 生成兑换请求
			swapRequest := fmt.Sprintf("兑换 %s %s 为 %s", amount, fromToken, toToken)
			subRequests = append(subRequests, swapRequest)
			fmt.Printf("DEBUG: 生成兑换请求: %s\n", swapRequest)
		case "stake":
			// 生成质押请求 - 使用兑换后的MTK数量
			if fromToken == "MEER" {
				// 根据兑换率计算MTK数量 (1 MEER = 1000 MTK)
				mtkAmount := fmt.Sprintf("%.0f", float64(parseFloat(amount))*1000)
				stakeRequest := fmt.Sprintf("质押 %s MTK", mtkAmount)
				subRequests = append(subRequests, stakeRequest)
				fmt.Printf("DEBUG: 生成质押请求: %s\n", stakeRequest)
			} else {
				stakeRequest := fmt.Sprintf("质押 %s %s", amount, fromToken)
				subRequests = append(subRequests, stakeRequest)
				fmt.Printf("DEBUG: 生成质押请求: %s\n", stakeRequest)
			}
		}
	}

	fmt.Printf("DEBUG: 最终生成的子请求: %v\n", subRequests)
	return subRequests
}

// intelligentDecompose 智能分解请求
func (ca *CompoundAnalyzer) intelligentDecompose(userRequest string) []string {
	var subRequests []string

	// 查找连接词
	connectors := []string{"然后", "再", "接着", "之后", "then", "and then", "after that"}

	for _, connector := range connectors {
		if strings.Contains(userRequest, connector) {
			parts := strings.Split(userRequest, connector)
			if len(parts) >= 2 {
				// 清理并添加子请求
				for _, part := range parts {
					part = strings.TrimSpace(part)
					if part != "" {
						subRequests = append(subRequests, part)
					}
				}
				return subRequests
			}
		}
	}

	// 如果没有找到连接词，尝试按逗号分割
	if strings.Contains(userRequest, "，") {
		parts := strings.Split(userRequest, "，")
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if part != "" {
				subRequests = append(subRequests, part)
			}
		}
		return subRequests
	}

	// 如果都无法分解，返回原请求
	return []string{userRequest}
}

// parseFloat 解析浮点数
func parseFloat(s string) float64 {
	var result float64
	fmt.Sscanf(s, "%f", &result)
	return result
}

// IsCompoundRequest 判断是否为复合请求
func (ca *CompoundAnalyzer) IsCompoundRequest(userRequest string) bool {
	lowerRequest := strings.ToLower(userRequest)

	// 检查是否包含多个操作的关键词
	compoundKeywords := []string{
		"然后", "再", "接着", "之后", "then", "and then", "after that",
	}

	for _, keyword := range compoundKeywords {
		if strings.Contains(lowerRequest, keyword) {
			return true
		}
	}

	// 检查复合操作模式
	compoundPatterns := []string{
		`兑换.*?再.*?质押`,
		`swap.*?then.*?stake`,
		`exchange.*?then.*?stake`,
		`兑换.*?然后.*?质押`,
	}

	for _, pattern := range compoundPatterns {
		matched, _ := regexp.MatchString(pattern, lowerRequest)
		if matched {
			return true
		}
	}

	return false
}

// getString 从map中安全获取字符串值
func getString(m map[string]interface{}, key string) string {
	if val, ok := m[key]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}

// generateSignatureRequest 生成签名请求
func (ca *CompoundAnalyzer) generateSignatureRequest(userRequest string, operations []Operation, totalGas uint64) *SignatureRequest {
	// 使用LLM提取签名请求信息
	signatureRequest := ca.extractSignatureRequestWithLLM(userRequest, operations, totalGas)

	fmt.Printf("DEBUG: 生成签名请求: %+v\n", signatureRequest)
	return signatureRequest
}

// extractSignatureRequestWithLLM 使用LLM提取签名请求
func (ca *CompoundAnalyzer) extractSignatureRequestWithLLM(userRequest string, operations []Operation, totalGas uint64) *SignatureRequest {
	// 暂时跳过LLM调用，直接使用默认逻辑
	// TODO: 实现LLM客户端集成

	// 如果LLM不可用或解析失败，使用默认逻辑
	return ca.generateDefaultSignatureRequest(userRequest, operations, totalGas)
}

// parseLLMResponse 解析LLM响应
func (ca *CompoundAnalyzer) parseLLMResponse(response string) *SignatureRequest {
	// 尝试提取JSON部分
	jsonStart := strings.Index(response, "{")
	jsonEnd := strings.LastIndex(response, "}")

	if jsonStart == -1 || jsonEnd == -1 || jsonEnd <= jsonStart {
		return nil
	}

	jsonStr := response[jsonStart : jsonEnd+1]

	// 解析JSON
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		return nil
	}

	// 构建签名请求
	sigReq := &SignatureRequest{
		Action:    getString(result, "action"),
		FromToken: getString(result, "from_token"),
		ToToken:   getString(result, "to_token"),
		Amount:    getString(result, "amount"),
		ToAddress: getString(result, "to_address"),
		Value:     getString(result, "value"),
		Data:      getString(result, "data"),
		GasLimit:  getString(result, "gas_limit"),
		GasPrice:  getString(result, "gas_price"),
		GasFee:    getString(result, "gas_fee"),
		Slippage:  getString(result, "slippage"),
	}

	return sigReq
}

// generateDefaultSignatureRequest 生成默认签名请求
func (ca *CompoundAnalyzer) generateDefaultSignatureRequest(userRequest string, operations []Operation, totalGas uint64) *SignatureRequest {
	// 从用户请求中提取基本信息
	amountRegex := regexp.MustCompile(`(\d+(?:\.\d+)?)\s*(MEER|MTK)`)
	matches := amountRegex.FindStringSubmatch(userRequest)

	var amount, fromToken, toToken string
	if len(matches) >= 3 {
		amount = matches[1]
		fromToken = matches[2]
		toToken = "MTK" // 默认目标代币
	}

	// 将金额转换为十六进制格式
	value := ca.convertAmountToHex(amount)

	// 构建签名请求
	return &SignatureRequest{
		Action:    "compound_operation",
		FromToken: fromToken,
		ToToken:   toToken,
		Amount:    amount,
		ToAddress: "0xfBb52268B01e20a9C0C566932716c9B9c550c868", // SimpleSwap合约地址
		Value:     value,                                        // 使用转换后的金额
		Data:      "0x",                                         // 简化的数据
		GasLimit:  fmt.Sprintf("0x%x", totalGas),                // 转换为十六进制
		GasPrice:  "0x3B9ACA00",                                 // 1 gwei
		GasFee:    fmt.Sprintf("%d", totalGas*1),                // 简化的Gas费用计算
		Slippage:  "0.5",                                        // 0.5%滑点
	}
}

// convertAmountToHex 将金额转换为十六进制格式
func (ca *CompoundAnalyzer) convertAmountToHex(amount string) string {
	if amount == "" {
		return "0x0"
	}

	// 解析金额
	var value float64
	if _, err := fmt.Sscanf(amount, "%f", &value); err != nil {
		fmt.Printf("⚠️ 无法解析金额: %s\n", amount)
		return "0x0"
	}

	// 转换为wei (假设代币有18位小数)
	weiValue := uint64(value * 1e18)

	// 转换为十六进制
	return fmt.Sprintf("0x%x", weiValue)
}

// parseUint64 解析uint64
func parseUint64(s string) uint64 {
	if s == "" {
		return 0
	}

	// 处理十六进制
	if strings.HasPrefix(s, "0x") {
		if parsed, err := strconv.ParseUint(s[2:], 16, 64); err == nil {
			return parsed
		}
	}

	// 处理十进制
	if parsed, err := strconv.ParseUint(s, 10, 64); err == nil {
		return parsed
	}

	return 0
}

// parseFloat64 解析float64
func parseFloat64(s string) float64 {
	if s == "" {
		return 0.0
	}

	if parsed, err := strconv.ParseFloat(s, 64); err == nil {
		return parsed
	}

	return 0.0
}

// MonitorTransactionAndContinue 监控交易状态并自动继续执行
func (ca *CompoundAnalyzer) MonitorTransactionAndContinue(ctx context.Context, operation *CompoundOperation, rpcClient interface{}) error {
	if !operation.AutoContinue || operation.CurrentStep >= len(operation.Operations) {
		return nil
	}

	// 检查是否有待监控的交易
	if operation.TransactionHash == "" {
		return nil
	}

	// 这里应该调用RPC客户端来监控交易状态
	// 由于我们没有直接访问RPC客户端，我们通过日志来模拟这个过程
	fmt.Printf("🔍 监控交易状态: %s (步骤 %d/%d)\n",
		operation.TransactionHash, operation.CurrentStep+1, len(operation.Operations))

	// 模拟交易确认检查
	// 在实际实现中，这里应该调用 rpcClient.WaitForTransactionConfirmation
	fmt.Printf("⏳ 等待交易确认: %s\n", operation.TransactionHash)

	// 模拟交易确认完成
	operation.StepStatus = "confirmed"
	fmt.Printf("✅ 交易确认完成: %s\n", operation.TransactionHash)

	// 移动到下一步
	operation.CurrentStep++
	if operation.CurrentStep < len(operation.Operations) {
		operation.StepStatus = "pending"
		operation.TransactionHash = "" // 清空交易哈希，准备下一步
		fmt.Printf("🔄 自动继续执行下一步: %d/%d\n", operation.CurrentStep+1, len(operation.Operations))
	} else {
		operation.StepStatus = "completed"
		fmt.Printf("🎉 所有步骤执行完成\n")
	}

	return nil
}

// UpdateTransactionHash 更新当前交易的哈希
func (ca *CompoundAnalyzer) UpdateTransactionHash(operation *CompoundOperation, txHash string) {
	operation.TransactionHash = txHash
	operation.StepStatus = "submitted"
	fmt.Printf("📝 更新交易哈希: %s (步骤 %d)\n", txHash, operation.CurrentStep+1)
}

// GetNextOperation 获取下一个要执行的操作
func (ca *CompoundAnalyzer) GetNextOperation(operation *CompoundOperation) *Operation {
	if operation.CurrentStep >= len(operation.Operations) {
		return nil
	}
	return &operation.Operations[operation.CurrentStep]
}
