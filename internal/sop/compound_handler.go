// internal/sop/compound_handler.go
package sop

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"qng_agent/internal/analyzer"
	"qng_agent/internal/protocol"
	"qng_agent/internal/rpc"
)

// TransactionConfig 交易配置
type TransactionConfig struct {
	ConfirmationTimeout   int
	PollingInterval       int
	RequiredConfirmations int
}

// CompoundHandler 复合操作处理器
type CompoundHandler struct {
	engine    *WorkflowEngineImpl
	rpcClient *rpc.Client
	txConfig  TransactionConfig
}

// NewCompoundHandler 创建复合操作处理器
func NewCompoundHandler(engine *WorkflowEngineImpl, rpcClient *rpc.Client, txConfig TransactionConfig) *CompoundHandler {
	return &CompoundHandler{
		engine:    engine,
		rpcClient: rpcClient,
		txConfig:  txConfig,
	}
}

// ProcessCompoundOperation 处理复合操作
func (ch *CompoundHandler) ProcessCompoundOperation(ctx context.Context, userRequest string) (*WorkflowResult, error) {
	log.Printf("🔍 开始处理复合操作: %s", userRequest)

	// 1. 分析复合请求
	compoundOp, err := ch.analyzeCompoundRequest(userRequest)
	if err != nil {
		return nil, fmt.Errorf("failed to analyze compound request: %w", err)
	}

	log.Printf("📝 复合操作分析结果: 总步骤=%d, 当前步骤=%d",
		len(compoundOp.Operations), compoundOp.CurrentStep)

	// 2. 创建执行上下文
	execCtx, err := ch.createCompoundExecutionContext(compoundOp, userRequest)
	if err != nil {
		return nil, fmt.Errorf("failed to create execution context: %w", err)
	}

	// 3. 生成第一个签名请求
	if compoundOp.CurrentStep < len(compoundOp.Operations) {
		firstOp := &compoundOp.Operations[compoundOp.CurrentStep]
		signatureRequest := ch.generateSignatureRequestForOperation(firstOp, compoundOp)

		// 构建需要签名的结果
		result := &WorkflowResult{
			ID:               execCtx.ID,
			Type:             "compound",
			Status:           "waiting_signature",
			StartTime:        time.Now(),
			EndTime:          time.Now(),
			Steps:            execCtx.State,
			NeedSignature:    true,
			SignatureRequest: signatureRequest,
			WorkflowContext: map[string]interface{}{
				"execution_id": execCtx.ID,
				"step_id":      "compound_step",
				"compound_op":  compoundOp,
				"current_step": compoundOp.CurrentStep,
				"total_steps":  len(compoundOp.Operations),
			},
		}

		log.Printf("✅ 生成复合操作签名请求: %+v", signatureRequest)
		return result, nil
	}

	return nil, fmt.Errorf("no operations to execute")
}

// ContinueCompoundOperation 继续复合操作
func (ch *CompoundHandler) ContinueCompoundOperation(ctx context.Context, executionID string, signature string) (*WorkflowResult, error) {
	log.Printf("🔄 继续复合操作: %s", executionID)

	// 获取执行上下文
	execCtx, err := ch.engine.GetExecutionStatus(executionID)
	if err != nil {
		return nil, fmt.Errorf("execution context not found: %w", err)
	}

	// 从上下文中提取复合操作信息
	compoundOp, err := ch.extractCompoundOperationFromContext(execCtx)
	if err != nil {
		return nil, fmt.Errorf("failed to extract compound operation: %w", err)
	}

	log.Printf("📝 复合操作状态: 当前步骤=%d/%d, 状态=%s",
		compoundOp.CurrentStep+1, len(compoundOp.Operations), compoundOp.StepStatus)

	// 验证签名
	if err := ch.engine.validateSignature(signature); err != nil {
		return nil, fmt.Errorf("invalid signature: %w", err)
	}

	// 从签名中提取交易哈希并等待确认
	if ch.rpcClient != nil {
		// 这里需要从签名中提取交易哈希
		// 在实际实现中，签名应该包含交易哈希信息
		txHash := ch.extractTransactionHashFromSignature(signature)
		if txHash != "" {
			log.Printf("⏳ 等待交易确认: %s", txHash)

			// 等待交易确认
			ctx, cancel := context.WithTimeout(ctx, time.Duration(ch.txConfig.ConfirmationTimeout)*time.Second)
			defer cancel()

			receipt, err := ch.rpcClient.WaitForTransactionConfirmation(
				ctx,
				txHash,
				ch.txConfig.RequiredConfirmations,
				time.Duration(ch.txConfig.PollingInterval)*time.Second,
			)

			if err != nil {
				log.Printf("❌ 交易确认失败: %v", err)
				return nil, fmt.Errorf("transaction confirmation failed: %w", err)
			}

			if !receipt.Success {
				log.Printf("❌ 交易执行失败: %s", txHash)
				return nil, fmt.Errorf("transaction execution failed: %s", txHash)
			}

			log.Printf("✅ 交易确认成功: %s", txHash)
		}
	}

	// 更新当前步骤状态
	compoundOp.StepStatus = "completed"
	compoundOp.CurrentStep++

	// 检查是否还有更多操作
	if compoundOp.CurrentStep < len(compoundOp.Operations) {
		// 生成下一个操作的签名请求
		nextOp := &compoundOp.Operations[compoundOp.CurrentStep]
		signatureRequest := ch.generateSignatureRequestForOperation(nextOp, compoundOp)

		// 更新执行上下文状态
		ch.updateCompoundStepState(execCtx, compoundOp)

		// 构建下一个签名请求
		result := &WorkflowResult{
			ID:               execCtx.ID,
			Type:             "compound",
			Status:           "waiting_signature",
			StartTime:        execCtx.StartTime,
			EndTime:          time.Now(),
			Steps:            execCtx.State,
			NeedSignature:    true,
			SignatureRequest: signatureRequest,
			WorkflowContext: map[string]interface{}{
				"execution_id": execCtx.ID,
				"step_id":      "compound_continue",
				"compound_op":  compoundOp,
				"current_step": compoundOp.CurrentStep,
				"total_steps":  len(compoundOp.Operations),
			},
		}

		log.Printf("✅ 生成下一个签名请求: %+v", signatureRequest)
		return result, nil
	} else {
		// 所有操作已完成
		log.Printf("🎉 复合操作执行完成: %s", executionID)
		compoundOp.StepStatus = "completed"

		// 构建完成结果
		result := &WorkflowResult{
			ID:        execCtx.ID,
			Type:      "compound",
			Status:    "completed",
			StartTime: execCtx.StartTime,
			EndTime:   time.Now(),
			Steps:     execCtx.State,
			FinalOutput: &protocol.StructuredMessage{
				Content: map[string]interface{}{
					"message":    "复合操作执行完成",
					"status":     "success",
					"operations": compoundOp.Operations,
					"total_gas":  compoundOp.TotalGas,
				},
			},
		}

		// 清理执行上下文
		ch.engine.cleanupExecutionContext(executionID)
		return result, nil
	}
}

// extractTransactionHashFromSignature 从签名中提取交易哈希
// 注意：这是一个简化的实现，实际应用中需要根据具体的签名格式来解析
func (ch *CompoundHandler) extractTransactionHashFromSignature(signature string) string {
	// 在实际实现中，签名应该包含交易哈希信息
	// 这里我们假设签名格式为: "txHash:signature" 或者从其他地方获取

	// 检查签名是否包含交易哈希（格式：txHash:signature）
	if len(signature) > 66 && strings.Contains(signature, ":") {
		parts := strings.Split(signature, ":")
		if len(parts) == 2 && len(parts[0]) == 66 {
			txHash := parts[0]
			log.Printf("📝 从签名中提取交易哈希: %s", txHash)
			return txHash
		}
	}

	// 如果没有找到交易哈希，返回空字符串
	log.Printf("⚠️  无法从签名中提取交易哈希，跳过交易确认")
	return ""
}

// analyzeCompoundRequest 分析复合请求
func (ch *CompoundHandler) analyzeCompoundRequest(userRequest string) (*analyzer.CompoundOperation, error) {
	// 创建复合分析器
	contractAnalyzer, err := analyzer.NewContractAnalyzer("config/contracts.json")
	if err != nil {
		return nil, fmt.Errorf("failed to create contract analyzer: %w", err)
	}
	compoundAnalyzer := analyzer.NewCompoundAnalyzer(contractAnalyzer)

	// 分析复合请求
	compoundOp, err := compoundAnalyzer.AnalyzeCompoundRequest(userRequest)
	if err != nil {
		return nil, fmt.Errorf("failed to analyze compound request: %w", err)
	}

	return compoundOp, nil
}

// createCompoundExecutionContext 创建复合操作执行上下文
func (ch *CompoundHandler) createCompoundExecutionContext(compoundOp *analyzer.CompoundOperation, userRequest string) (*WorkflowExecutionContext, error) {
	// 创建执行上下文
	execCtx := &WorkflowExecutionContext{
		ID:        generateExecutionID(),
		StartTime: time.Now(),
		State:     make(map[string]*StepState),
		Messages:  make(map[string]*protocol.StructuredMessage),
	}

	// 初始化复合步骤状态
	execCtx.State["compound_step"] = &StepState{
		StepID:    "compound_step",
		Status:    "pending",
		StartTime: time.Now(),
		Output: &protocol.StructuredMessage{
			Content: map[string]interface{}{
				"current_step": compoundOp.CurrentStep,
				"total_steps":  len(compoundOp.Operations),
				"status":       "pending",
				"operations":   compoundOp.Operations,
			},
		},
	}

	// 保存复合操作信息到上下文
	execCtx.Messages["compound_operation"] = &protocol.StructuredMessage{
		Content: map[string]interface{}{
			"compound_op":  compoundOp,
			"user_request": userRequest,
		},
	}

	// 注册执行上下文
	ch.engine.execMu.Lock()
	ch.engine.executions[execCtx.ID] = execCtx
	ch.engine.execMu.Unlock()

	return execCtx, nil
}

// generateSignatureRequestForOperation 为指定操作生成签名请求
func (ch *CompoundHandler) generateSignatureRequestForOperation(op *analyzer.Operation, compoundOp *analyzer.CompoundOperation) *SignatureRequest {
	// 确保有合理的gas限制
	gasLimit := op.GasEstimate
	if gasLimit == 0 {
		gasLimit = 100000 // 默认100k gas
	}

	// 根据操作类型生成相应的签名请求
	switch op.Type {
	case "swap":
		fromToken := getString(op.Parameters, "from_token")
		toToken := getString(op.Parameters, "to_token")
		amount := getString(op.Parameters, "amount")

		// 将金额转换为十六进制格式
		value := ch.convertAmountToHex(amount)

		return &SignatureRequest{
			Action:    "swap",
			FromToken: fromToken,
			ToToken:   toToken,
			Amount:    amount,
			ToAddress: "0xfBb52268B01e20a9C0C566932716c9B9c550c868",
			Value:     value,
			Data:      "0x",
			GasLimit:  fmt.Sprintf("0x%x", gasLimit),
			GasPrice:  "0x3B9ACA00",
			GasFee:    fmt.Sprintf("%d", gasLimit),
			Slippage:  "0.5",
		}
	case "stake":
		token := getString(op.Parameters, "token")
		amount := getString(op.Parameters, "amount")

		// 将金额转换为十六进制格式
		value := ch.convertAmountToHex(amount)

		return &SignatureRequest{
			Action:    "stake",
			FromToken: token,
			ToToken:   token,
			Amount:    amount,
			ToAddress: "0xfBb52268B01e20a9C0C566932716c9B9c550c868",
			Value:     value,
			Data:      "0x",
			GasLimit:  fmt.Sprintf("0x%x", gasLimit),
			GasPrice:  "0x3B9ACA00",
			GasFee:    fmt.Sprintf("%d", gasLimit),
			Slippage:  "0.5",
		}
	default:
		return &SignatureRequest{
			Action:    "unknown",
			FromToken: "",
			ToToken:   "",
			Amount:    "",
			ToAddress: "",
			Value:     "0x0",
			Data:      "0x",
			GasLimit:  fmt.Sprintf("0x%x", gasLimit),
			GasPrice:  "0x0",
			GasFee:    fmt.Sprintf("%d", gasLimit),
			Slippage:  "0.0",
		}
	}
}

// extractCompoundOperationFromContext 从执行上下文中提取复合操作信息
func (ch *CompoundHandler) extractCompoundOperationFromContext(execCtx *WorkflowExecutionContext) (*analyzer.CompoundOperation, error) {
	// 从compound_operation消息中提取复合操作信息
	if msg, exists := execCtx.Messages["compound_operation"]; exists && msg.Content != nil {
		if compoundOpData, ok := msg.Content["compound_op"].(*analyzer.CompoundOperation); ok {
			return compoundOpData, nil
		}
	}

	// 从compound_step状态中提取信息
	if stepState, exists := execCtx.State["compound_step"]; exists && stepState.Output != nil {
		if content := stepState.Output.Content; content != nil {
			if operations, ok := content["operations"].([]analyzer.Operation); ok {
				// 重建复合操作
				var totalGas uint64
				for _, op := range operations {
					totalGas += op.GasEstimate
				}

				currentStep := 0
				if step, ok := content["current_step"].(float64); ok {
					currentStep = int(step)
				}

				return &analyzer.CompoundOperation{
					Operations:   operations,
					TotalGas:     totalGas,
					CurrentStep:  currentStep,
					StepStatus:   "pending",
					AutoContinue: true,
				}, nil
			}
		}
	}

	return nil, fmt.Errorf("cannot extract compound operation from context")
}

// updateCompoundStepState 更新复合步骤状态
func (ch *CompoundHandler) updateCompoundStepState(execCtx *WorkflowExecutionContext, compoundOp *analyzer.CompoundOperation) {
	// 更新复合步骤状态
	if stepState, exists := execCtx.State["compound_step"]; exists {
		stepState.Status = "running"
		stepState.Output = &protocol.StructuredMessage{
			Content: map[string]interface{}{
				"current_step": compoundOp.CurrentStep,
				"total_steps":  len(compoundOp.Operations),
				"status":       compoundOp.StepStatus,
				"operations":   compoundOp.Operations,
			},
		}
	}

	// 更新复合操作消息
	execCtx.Messages["compound_operation"] = &protocol.StructuredMessage{
		Content: map[string]interface{}{
			"compound_op": compoundOp,
		},
	}
}

// convertAmountToHex 将金额转换为十六进制格式
func (ch *CompoundHandler) convertAmountToHex(amount string) string {
	if amount == "" {
		return "0x0"
	}

	// 解析金额
	var value float64
	if _, err := fmt.Sscanf(amount, "%f", &value); err != nil {
		log.Printf("⚠️ 无法解析金额: %s", amount)
		return "0x0"
	}

	// 转换为wei (假设代币有18位小数)
	weiValue := uint64(value * 1e18)

	// 转换为十六进制
	return fmt.Sprintf("0x%x", weiValue)
}

// getStringFromMap 从map中安全获取字符串值
func getStringFromMap(m map[string]interface{}, key string) string {
	if val, ok := m[key]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}
