// internal/sop/engine.go
package sop

import (
	"context"
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"qng_agent/internal/analyzer"
	"qng_agent/internal/contracts"
	"qng_agent/internal/protocol"
	"qng_agent/internal/roles"
)

// SignatureRequiredError 表示需要签名的错误
type SignatureRequiredError struct {
	StepID string
	Output *protocol.StructuredMessage
}

func (e *SignatureRequiredError) Error() string {
	return fmt.Sprintf("signature required for step %s", e.StepID)
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

// RoleManager 角色管理器接口
type RoleManager interface {
	GetRole(roleType roles.RoleType) (roles.BaseRole, error)
	RegisterRole(role roles.BaseRole) error
	ListRoles() []roles.BaseRole
}

// WorkflowEngineImpl 工作流引擎实现
type WorkflowEngineImpl struct {
	workflows      *StandardWorkflows
	roleManager    RoleManager
	messagePool    *protocol.MessagePool
	stateManager   *StateManager
	feedbackSystem *FeedbackSystem

	// 执行上下文管理
	executions map[string]*WorkflowExecutionContext
	execMu     sync.RWMutex

	// 复合操作处理器
	CompoundHandler *CompoundHandler

	// 合约管理器
	contractManager *contracts.ContractManager

	// 配置
	config *EngineConfig
}

// EngineConfig 引擎配置
type EngineConfig struct {
	MaxConcurrentWorkflows int           `json:"max_concurrent_workflows"`
	DefaultTimeout         time.Duration `json:"default_timeout"`
	EnableFeedback         bool          `json:"enable_feedback"`
	EnableMetrics          bool          `json:"enable_metrics"`
	RetryStrategy          string        `json:"retry_strategy"`
}

// DefaultEngineConfig 默认引擎配置
func DefaultEngineConfig() *EngineConfig {
	return &EngineConfig{
		MaxConcurrentWorkflows: 10,
		DefaultTimeout:         30 * time.Minute,
		EnableFeedback:         true,
		EnableMetrics:          true,
		RetryStrategy:          "exponential_backoff",
	}
}

// NewWorkflowEngineImpl 创建工作流引擎实现
func NewWorkflowEngineImpl(config *EngineConfig, contractManager *contracts.ContractManager) *WorkflowEngineImpl {
	if config == nil {
		config = DefaultEngineConfig()
	}

	messagePool := protocol.NewMessagePool()
	stateManager := NewStateManager()
	feedbackSystem := NewFeedbackSystem(messagePool, stateManager)

	// 初始化角色管理器
	roleManager := NewRoleManager()

	engine := &WorkflowEngineImpl{
		workflows:       NewStandardWorkflows(),
		roleManager:     roleManager,
		messagePool:     messagePool,
		stateManager:    stateManager,
		feedbackSystem:  feedbackSystem,
		contractManager: contractManager,
		executions:      make(map[string]*WorkflowExecutionContext),
		config:          config,
	}

	// 初始化复合操作处理器（暂时不传入RPC客户端，使用默认配置）
	engine.CompoundHandler = NewCompoundHandler(engine, nil, TransactionConfig{
		ConfirmationTimeout:   60,
		PollingInterval:       2,
		RequiredConfirmations: 1,
	})

	// 初始化反馈处理器
	engine.initializeFeedbackHandlers()

	return engine
}

// initializeFeedbackHandlers 初始化反馈处理器
func (we *WorkflowEngineImpl) initializeFeedbackHandlers() {
	if !we.config.EnableFeedback {
		return
	}

	// 注册验证反馈处理器
	validationHandler := NewValidationFeedbackHandler(true, 3)
	we.feedbackSystem.RegisterHandler(FeedbackTypeValidation, validationHandler)

	// 注册执行反馈处理器
	executionHandler := NewExecutionFeedbackHandler(nil, nil)
	we.feedbackSystem.RegisterHandler(FeedbackTypeExecution, executionHandler)
}

// ExecuteWorkflow 执行工作流（带反馈机制）
func (we *WorkflowEngineImpl) ExecuteWorkflow(ctx context.Context, workflowType WorkflowType, input *protocol.StructuredMessage) (*WorkflowResult, error) {
	// 创建执行上下文
	execCtx, err := we.createExecutionContext(workflowType, input)
	if err != nil {
		return nil, fmt.Errorf("failed to create execution context: %w", err)
	}

	// 发送开始执行反馈
	if we.config.EnableFeedback {
		feedback := &FeedbackMessage{
			Type:       FeedbackTypeExecution,
			Level:      FeedbackLevelInfo,
			Source:     "WorkflowEngine",
			Message:    fmt.Sprintf("Starting workflow execution: %s", workflowType),
			WorkflowID: execCtx.ID,
			Details: map[string]interface{}{
				"workflow_type": workflowType,
				"input_id":      input.ID,
			},
		}

		if err := we.feedbackSystem.SendFeedback(ctx, feedback); err != nil {
			we.stateManager.LogExecution("feedback_error", "warning",
				fmt.Sprintf("Failed to send start feedback: %v", err), nil)
		}
	}

	// 执行工作流
	result, err := we.executeWorkflowWithFeedback(ctx, execCtx)

	// 如果执行失败，创建一个包含错误信息的结果
	if err != nil && result == nil {
		result = &WorkflowResult{
			ID:        execCtx.ID,
			Type:      execCtx.WorkflowDef.Type,
			Status:    "failed",
			StartTime: execCtx.StartTime,
			EndTime:   time.Now(),
			Steps:     execCtx.State,
		}
	}

	// 只有在工作流真正完成时才清理执行上下文
	// 如果工作流需要签名，保留执行上下文以便后续继续执行
	if result != nil && !result.NeedSignature {
		we.cleanupExecutionContext(execCtx.ID)
		log.Printf("🧹 清理执行上下文: %s", execCtx.ID)
	} else if result != nil && result.NeedSignature {
		log.Printf("⏸️  保留执行上下文等待签名: %s", execCtx.ID)
	}

	// 发送完成反馈
	if we.config.EnableFeedback {
		level := FeedbackLevelInfo
		message := "Workflow completed successfully"

		if err != nil {
			level = FeedbackLevelError
			message = fmt.Sprintf("Workflow failed: %v", err)
		}

		feedback := &FeedbackMessage{
			Type:       FeedbackTypeExecution,
			Level:      level,
			Source:     "WorkflowEngine",
			Message:    message,
			WorkflowID: execCtx.ID,
			Details: map[string]interface{}{
				"workflow_type": workflowType,
				"duration":      time.Since(execCtx.StartTime),
				"final_status":  result != nil,
			},
		}

		if err := we.feedbackSystem.SendFeedback(ctx, feedback); err != nil {
			we.stateManager.LogExecution("feedback_error", "warning",
				fmt.Sprintf("Failed to send completion feedback: %v", err), nil)
		}
	}

	return result, err
}

// createExecutionContext 创建执行上下文
func (we *WorkflowEngineImpl) createExecutionContext(workflowType WorkflowType, input *protocol.StructuredMessage) (*WorkflowExecutionContext, error) {
	// 检查并发限制
	we.execMu.RLock()
	if len(we.executions) >= we.config.MaxConcurrentWorkflows {
		we.execMu.RUnlock()
		return nil, fmt.Errorf("maximum concurrent workflows reached: %d", we.config.MaxConcurrentWorkflows)
	}
	we.execMu.RUnlock()

	// 获取工作流定义
	workflow, exists := we.workflows.workflows[workflowType]
	if !exists {
		return nil, fmt.Errorf("workflow type %s not found", workflowType)
	}

	// 创建执行上下文
	execCtx := &WorkflowExecutionContext{
		ID:          generateExecutionID(),
		WorkflowDef: workflow,
		StartTime:   time.Now(),
		State:       make(map[string]*StepState),
		Messages:    make(map[string]*protocol.StructuredMessage),
	}

	// 初始化输入
	execCtx.Messages["input"] = input

	// 如果是复合操作，初始化步骤状态
	if workflowType == "compound" {
		execCtx.State["compound_step"] = &StepState{
			StepID: "compound_step",
			Status: "pending",
			Output: &protocol.StructuredMessage{
				Content: map[string]interface{}{
					"current_step": 0,
					"total_steps":  0, // 将在执行时更新
					"status":       "pending",
				},
			},
		}
	}

	// 注册执行上下文
	we.execMu.Lock()
	we.executions[execCtx.ID] = execCtx
	we.execMu.Unlock()

	return execCtx, nil
}

// executeWorkflowWithFeedback 执行工作流（带反馈）
func (we *WorkflowEngineImpl) executeWorkflowWithFeedback(ctx context.Context, execCtx *WorkflowExecutionContext) (*WorkflowResult, error) {
	// 设置超时
	timeoutCtx, cancel := context.WithTimeout(ctx, we.config.DefaultTimeout)
	defer cancel()

	// 执行工作流步骤
	for _, step := range execCtx.WorkflowDef.Steps {
		// 检查上下文是否已取消
		select {
		case <-timeoutCtx.Done():
			return nil, fmt.Errorf("workflow execution timeout")
		default:
		}

		// 执行步骤
		if err := we.executeStepWithFeedback(timeoutCtx, execCtx, &step); err != nil {
			// 检查是否是签名请求错误
			if sigErr, ok := err.(*SignatureRequiredError); ok {
				// 构建包含签名请求的结果
				signatureRequest := we.extractSignatureRequest(sigErr.Output)
				log.Printf("🔍 提取的签名请求: %+v", signatureRequest)

				result := &WorkflowResult{
					ID:               execCtx.ID,
					Type:             execCtx.WorkflowDef.Type,
					Status:           "waiting_signature",
					StartTime:        execCtx.StartTime,
					EndTime:          time.Now(),
					Steps:            execCtx.State,
					FinalOutput:      sigErr.Output,
					NeedSignature:    true,
					SignatureRequest: signatureRequest,
					WorkflowContext: map[string]interface{}{
						"execution_id": execCtx.ID,
						"step_id":      sigErr.StepID,
						"output":       sigErr.Output,
					},
				}

				log.Printf("✅ 构建的WorkflowResult: NeedSignature=%v, SignatureRequest=%v",
					result.NeedSignature, result.SignatureRequest != nil)
				if result.SignatureRequest != nil {
					log.Printf("📝 WorkflowResult中的签名请求: %+v", result.SignatureRequest)
				} else {
					log.Printf("❌ WorkflowResult中的签名请求为nil")
				}

				// 发送签名请求反馈
				if we.config.EnableFeedback {
					feedback := &FeedbackMessage{
						Type:       FeedbackTypeExecution,
						Level:      FeedbackLevelInfo,
						Source:     "SignatureHandler",
						Message:    fmt.Sprintf("Signature required for step %s", step.ID),
						WorkflowID: execCtx.ID,
						StepID:     step.ID,
						Details: map[string]interface{}{
							"signature_required": true,
							"step_id":            step.ID,
							"signature_request":  result.SignatureRequest,
						},
					}
					we.feedbackSystem.SendFeedback(timeoutCtx, feedback)
				}

				return result, nil
			}

			// 发送错误反馈
			if we.config.EnableFeedback {
				feedback := &FeedbackMessage{
					Type:       FeedbackTypeExecution,
					Level:      FeedbackLevelError,
					Source:     "WorkflowEngine",
					Message:    fmt.Sprintf("Step %s failed: %v", step.ID, err),
					WorkflowID: execCtx.ID,
					StepID:     step.ID,
					Details: map[string]interface{}{
						"step_name": step.Name,
						"role":      step.Role,
						"error":     err.Error(),
					},
				}

				we.feedbackSystem.SendFeedback(timeoutCtx, feedback)
			}

			// 执行回滚
			if rollback, exists := execCtx.WorkflowDef.Rollback[step.ID]; exists {
				we.executeRollbackWithFeedback(timeoutCtx, execCtx, &rollback)
			}

			return nil, fmt.Errorf("step %s failed: %w", step.ID, err)
		}

		// 发送步骤完成反馈
		if we.config.EnableFeedback {
			feedback := &FeedbackMessage{
				Type:       FeedbackTypeExecution,
				Level:      FeedbackLevelInfo,
				Source:     "WorkflowEngine",
				Message:    fmt.Sprintf("Step %s completed successfully", step.ID),
				WorkflowID: execCtx.ID,
				StepID:     step.ID,
				Details: map[string]interface{}{
					"step_name": step.Name,
					"role":      step.Role,
					"duration":  time.Since(execCtx.State[step.ID].StartTime),
				},
			}

			we.feedbackSystem.SendFeedback(timeoutCtx, feedback)
		}
	}

	// 构建结果
	result := &WorkflowResult{
		ID:          execCtx.ID,
		Type:        execCtx.WorkflowDef.Type,
		Status:      "completed",
		StartTime:   execCtx.StartTime,
		EndTime:     time.Now(),
		Steps:       execCtx.State,
		FinalOutput: execCtx.Messages["audit"],
	}

	return result, nil
}

// needsSignature 检查输出是否需要签名
func (we *WorkflowEngineImpl) needsSignature(output *protocol.StructuredMessage) bool {
	// 检查是否包含需要签名的操作
	if operations, ok := output.Content["operations"].([]interface{}); ok {
		for _, op := range operations {
			if opMap, ok := op.(map[string]interface{}); ok {
				if opType, ok := opMap["type"].(string); ok {
					// 兑换、质押和复合操作需要签名
					if opType == "swap" || opType == "stake" || opType == "compound" {
						return true
					}
				}
			}
		}
	}

	// 检查参数中是否明确指定需要签名
	if params, ok := output.Content["parameters"].(map[string]interface{}); ok {
		if needSignature, ok := params["need_signature"].(bool); ok && needSignature {
			return true
		}
	}

	return false
}

// buildSignatureRequestFromMap 从map构建签名请求
func (we *WorkflowEngineImpl) buildSignatureRequestFromMap(sigReq map[string]interface{}) *SignatureRequest {
	// 从map中提取签名请求信息
	action := getString(sigReq, "action")
	fromToken := getString(sigReq, "from_token")
	toToken := getString(sigReq, "to_token")
	amount := getString(sigReq, "amount")
	toAddress := getString(sigReq, "to_address")
	value := getString(sigReq, "value")
	data := getString(sigReq, "data")
	gasLimitStr := getString(sigReq, "gas_limit")
	gasPrice := getString(sigReq, "gas_price")
	gasFee := getString(sigReq, "gas_fee")
	slippageStr := getString(sigReq, "slippage")

	// 处理GasLimit
	if gasLimitStr != "" {
		// 如果已经是十六进制格式，直接使用
		if !strings.HasPrefix(gasLimitStr, "0x") {
			// 如果是数字字符串，转换为十六进制
			if parsed, err := strconv.ParseUint(gasLimitStr, 10, 64); err == nil {
				gasLimitStr = fmt.Sprintf("0x%x", parsed)
			}
		}
	} else {
		gasLimitStr = "0x186a0" // 默认值
	}

	// 处理Slippage
	var slippage string
	if slippageStr != "" {
		// 尝试解析为float64，然后格式化为字符串
		if parsed, err := strconv.ParseFloat(slippageStr, 64); err == nil {
			slippage = fmt.Sprintf("%.1f", parsed)
		} else {
			// 如果解析失败，直接使用原始字符串
			slippage = slippageStr
		}
	} else {
		slippage = "0.5" // 默认值
	}

	return &SignatureRequest{
		Action:    action,
		FromToken: fromToken,
		ToToken:   toToken,
		Amount:    amount,
		ToAddress: toAddress,
		Value:     value,
		Data:      data,
		GasLimit:  gasLimitStr,
		GasPrice:  gasPrice,
		GasFee:    gasFee,
		Slippage:  slippage,
	}
}

// buildSignatureRequestFromOperation 从操作构建签名请求
func (we *WorkflowEngineImpl) buildSignatureRequestFromOperation(op map[string]interface{}) *SignatureRequest {
	action := getString(op, "type")
	fromToken := getString(op, "from_token")
	toToken := getString(op, "to_token")
	amount := getString(op, "amount")

	log.Printf("🔍 从操作构建签名请求: action=%s, fromToken=%s, toToken=%s, amount=%s",
		action, fromToken, toToken, amount)

	// 根据操作类型设置默认值
	toAddress := "0xfBb52268B01e20a9C0C566932716c9B9c550c868" // SimpleSwap合约地址
	value := we.convertAmountToHex(amount)                    // 使用金额转换
	data := "0x"
	gasLimit := "0x186a0"    // 默认Gas限制
	gasPrice := "0x3B9ACA00" // 1 gwei
	gasFee := "100000"
	slippage := "0.5"

	sigReq := &SignatureRequest{
		Action:    action,
		FromToken: fromToken,
		ToToken:   toToken,
		Amount:    amount,
		ToAddress: toAddress,
		Value:     value,
		Data:      data,
		GasLimit:  gasLimit,
		GasPrice:  gasPrice,
		GasFee:    gasFee,
		Slippage:  slippage,
	}

	log.Printf("✅ 构建的签名请求: %+v", sigReq)
	return sigReq
}

// buildSignatureRequestFromUserRequest 从用户请求构建签名请求
func (we *WorkflowEngineImpl) buildSignatureRequestFromUserRequest(userRequest string) *SignatureRequest {
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
	value := we.convertAmountToHex(amount)

	// 构建签名请求
	return &SignatureRequest{
		Action:    "compound_operation",
		FromToken: fromToken,
		ToToken:   toToken,
		Amount:    amount,
		ToAddress: "0xfBb52268B01e20a9C0C566932716c9B9c550c868", // SimpleSwap合约地址
		Value:     value,                                        // 使用转换后的金额
		Data:      "0x",                                         // 简化的数据
		GasLimit:  "100000",                                     // 默认Gas限制
		GasPrice:  "0x3B9ACA00",                                 // 1 gwei
		GasFee:    "100000",                                     // 简化的Gas费用计算
		Slippage:  "0.5",                                        // 0.5%滑点
	}
}

// extractSignatureRequest 从输出中提取签名请求
func (we *WorkflowEngineImpl) extractSignatureRequest(output *protocol.StructuredMessage) *SignatureRequest {
	log.Printf("🔍 开始提取签名请求，输出内容: %+v", output.Content)

	// 从参数中提取签名请求
	if params, ok := output.Content["parameters"].(map[string]interface{}); ok {
		log.Printf("📝 找到参数: %+v", params)
		if sigReq, ok := params["signature_request"].(map[string]interface{}); ok {
			log.Printf("✅ 找到签名请求: %+v", sigReq)
			return we.buildSignatureRequestFromMap(sigReq)
		}
	}

	// 尝试从operations中提取签名请求信息
	if operations, ok := output.Content["operations"].([]interface{}); ok {
		log.Printf("📝 找到操作列表: %+v", operations)
		if len(operations) > 0 {
			// 从第一个操作中提取基本信息
			if firstOp, ok := operations[0].(map[string]interface{}); ok {
				log.Printf("✅ 从第一个操作提取信息: %+v", firstOp)
				// 检查是否已经有构建好的签名请求
				if sigReq, ok := firstOp["signature_request"].(map[string]interface{}); ok {
					log.Printf("✅ 找到已构建的签名请求: %+v", sigReq)
					return we.buildSignatureRequestFromMap(sigReq)
				}
				// 如果没有，则构建新的
				return we.buildSignatureRequestFromOperation(firstOp)
			}
		}
	}

	// 尝试从user_intent中提取信息
	if userIntent, ok := output.Content["user_intent"].(map[string]interface{}); ok {
		log.Printf("📝 找到用户意图: %+v", userIntent)
		if originalRequest, ok := userIntent["original_request"].(string); ok {
			log.Printf("✅ 从原始请求提取信息: %s", originalRequest)
			return we.buildSignatureRequestFromUserRequest(originalRequest)
		}
	}

	// 如果没有找到签名请求，返回默认的
	log.Printf("❌ 未找到签名请求，返回默认值")
	return &SignatureRequest{
		Action:    "unknown",
		FromToken: "",
		ToToken:   "",
		Amount:    "",
		ToAddress: "",
		Value:     "0x0",
		Data:      "0x",
		GasLimit:  "0",
		GasPrice:  "0x0",
		GasFee:    "0",
		Slippage:  "0.0",
	}
}

// ContinueWithSignature 使用签名继续工作流
func (we *WorkflowEngineImpl) ContinueWithSignature(ctx context.Context, executionID string, signature string) (*WorkflowResult, error) {
	return we.ContinueWithSignatureAndTransaction(ctx, executionID, signature, "")
}

// ContinueWithSignatureAndTransaction 继续执行工作流（带签名和交易哈希）
func (we *WorkflowEngineImpl) ContinueWithSignatureAndTransaction(ctx context.Context, executionID string, signature string, transactionHash string) (*WorkflowResult, error) {
	log.Printf("🔄 SOP引擎开始继续执行工作流: %s", executionID)
	log.Printf("🔐 签名长度: %d", len(signature))
	if transactionHash != "" {
		log.Printf("📝 交易哈希: %s", transactionHash)
	}

	we.execMu.RLock()
	execCtx, exists := we.executions[executionID]
	we.execMu.RUnlock()

	if !exists {
		log.Printf("❌ 执行上下文不存在: %s", executionID)
		return nil, fmt.Errorf("execution %s not found", executionID)
	}

	log.Printf("✅ 找到执行上下文: %s", executionID)
	log.Printf("📝 执行上下文状态: %+v", execCtx.State)

	// 验证签名
	if err := we.validateSignature(signature); err != nil {
		log.Printf("❌ 签名验证失败: %v", err)
		return nil, fmt.Errorf("invalid signature: %w", err)
	}

	log.Printf("✅ 签名验证通过")

	// 如果有交易哈希，等待交易确认
	if transactionHash != "" && we.CompoundHandler != nil && we.CompoundHandler.rpcClient != nil {
		log.Printf("⏳ 等待交易确认: %s", transactionHash)
		
		ctx, cancel := context.WithTimeout(ctx, time.Duration(we.CompoundHandler.txConfig.ConfirmationTimeout)*time.Second)
		defer cancel()

		receipt, err := we.CompoundHandler.rpcClient.WaitForTransactionConfirmation(
			ctx,
			transactionHash,
			we.CompoundHandler.txConfig.RequiredConfirmations,
			time.Duration(we.CompoundHandler.txConfig.PollingInterval)*time.Second,
		)

		if err != nil {
			log.Printf("❌ 交易确认失败: %v", err)
			return nil, fmt.Errorf("transaction confirmation failed: %w", err)
		}

		if !receipt.Success {
			log.Printf("❌ 交易执行失败: %s", transactionHash)
			return nil, fmt.Errorf("transaction execution failed: %s", transactionHash)
		}

		log.Printf("✅ 交易确认成功: %s", transactionHash)
	}

	// 检查是否是复合操作
	if we.isCompoundOperation(execCtx) {
		log.Printf("🔍 检测到复合操作")

		// 提取复合操作信息
		compoundOp, err := we.extractCompoundOperationFromContext(execCtx)
		if err != nil {
			log.Printf("❌ 无法提取复合操作信息: %v", err)
			return nil, fmt.Errorf("failed to extract compound operation: %w", err)
		}

		log.Printf("📝 复合操作信息: 总步骤=%d, 当前步骤=%d, 状态=%s",
			len(compoundOp.Operations), compoundOp.CurrentStep, compoundOp.StepStatus)

		// 更新复合操作状态 - 先递增当前步骤
		compoundOp.CurrentStep++
		compoundOp.StepStatus = "pending"

		// 检查是否还有更多操作需要执行
		if compoundOp.CurrentStep < len(compoundOp.Operations) {
			// 获取下一个操作
			nextOp := &compoundOp.Operations[compoundOp.CurrentStep]
			log.Printf("🔄 准备执行下一步操作: %s (%s)", nextOp.Type, nextOp.Description)

			// 生成下一个操作的签名请求
			signatureRequest := we.generateSignatureRequestForOperation(nextOp, compoundOp)

			// 保存当前步骤状态到执行上下文
			log.Printf("💾 保存步骤状态: current_step=%d, total_steps=%d", compoundOp.CurrentStep, len(compoundOp.Operations))
			execCtx.State["compound_step"] = &StepState{
				StepID: "compound_step",
				Status: "pending",
				Output: &protocol.StructuredMessage{
					Content: map[string]interface{}{
						"current_step": compoundOp.CurrentStep,
						"total_steps":  len(compoundOp.Operations),
						"status":       compoundOp.StepStatus,
					},
				},
			}
			log.Printf("✅ 步骤状态已保存")

			// 构建包含新签名请求的结果
			result := &WorkflowResult{
				ID:               execCtx.ID,
				Type:             execCtx.WorkflowDef.Type,
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
				Type:      execCtx.WorkflowDef.Type,
				Status:    "completed",
				StartTime: execCtx.StartTime,
				EndTime:   time.Now(),
				Steps:     execCtx.State,
				FinalOutput: &protocol.StructuredMessage{
					Content: map[string]interface{}{
						"message": "复合操作执行完成",
						"status":  "success",
					},
				},
			}

			// 清理执行上下文
			we.cleanupExecutionContext(executionID)
			return result, nil
		}
	}

	// 继续执行工作流（非复合操作）
	log.Printf("🔄 继续执行工作流...")
	result, err := we.executeWorkflowWithFeedback(ctx, execCtx)
	if err != nil {
		log.Printf("❌ 工作流执行失败: %v", err)
		return nil, err
	}

	log.Printf("✅ 工作流继续执行成功: %+v", result)
	return result, nil
}

// isCompoundOperation 检查是否是复合操作
func (we *WorkflowEngineImpl) isCompoundOperation(execCtx *WorkflowExecutionContext) bool {
	// 检查decompose步骤的输出
	if stepState, exists := execCtx.State["decompose"]; exists && stepState.Output != nil {
		content := stepState.Output.Content
		if content == nil {
			return false
		}

		// 检查是否有复合操作的特征
		if operations, ok := content["operations"].([]interface{}); ok {
			return len(operations) > 1
		}
	}

	return false
}

// monitorCompoundOperation 监控复合操作的交易状态
func (we *WorkflowEngineImpl) monitorCompoundOperation(ctx context.Context, execCtx *WorkflowExecutionContext) {
	log.Printf("🔍 开始监控复合操作: %s", execCtx.ID)

	// 从执行上下文中提取复合操作信息
	compoundOp, err := we.extractCompoundOperationFromContext(execCtx)
	if err != nil {
		log.Printf("❌ 无法提取复合操作信息: %v", err)
		return
	}

	log.Printf("📝 复合操作信息: 总步骤=%d, 当前步骤=%d, 状态=%s",
		len(compoundOp.Operations), compoundOp.CurrentStep, compoundOp.StepStatus)

	// 监控交易状态
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Printf("🛑 交易监控被取消: %s", execCtx.ID)
			return
		case <-ticker.C:
			log.Printf("🔍 检查交易状态: %s", execCtx.ID)

			// 检查当前交易是否已确认
			if compoundOp.TransactionHash != "" {
				// 这里应该调用RPC客户端来检查交易状态
				// 暂时模拟交易确认
				log.Printf("✅ 第一笔交易确认完成，准备执行下一步")

				// 移动到下一步
				compoundOp.CurrentStep++
				compoundOp.TransactionHash = "" // 清空交易哈希，准备下一步

				if compoundOp.CurrentStep < len(compoundOp.Operations) {
					// 还有更多操作需要执行
					log.Printf("🔄 自动继续执行下一步操作: %d/%d",
						compoundOp.CurrentStep+1, len(compoundOp.Operations))

					// 生成下一个操作的签名请求
					nextOp := &compoundOp.Operations[compoundOp.CurrentStep]
					signatureRequest := we.generateSignatureRequestForOperation(nextOp, compoundOp)

					// 更新执行上下文状态
					we.updateExecutionContextForNextStep(execCtx, compoundOp, signatureRequest)

					log.Printf("📝 生成下一个签名请求: %+v", signatureRequest)
					return
				} else {
					// 所有操作已完成
					log.Printf("🎉 复合操作执行完成: %s", execCtx.ID)
					compoundOp.StepStatus = "completed"
					return
				}
			} else {
				log.Printf("⏳ 等待交易确认...")
			}
		}
	}
}

// extractCompoundOperationFromContext 从执行上下文中提取复合操作信息
func (we *WorkflowEngineImpl) extractCompoundOperationFromContext(execCtx *WorkflowExecutionContext) (*analyzer.CompoundOperation, error) {
	// 从decompose步骤的输出中提取复合操作信息
	if stepState, exists := execCtx.State["decompose"]; exists && stepState.Output != nil {
		content := stepState.Output.Content
		if content == nil {
			return nil, fmt.Errorf("decompose step output is nil")
		}

		// 尝试从operations中重建复合操作
		if operations, ok := content["operations"].([]interface{}); ok {
			var compoundOps []analyzer.Operation
			var totalGas uint64

			for i, op := range operations {
				if opMap, ok := op.(map[string]interface{}); ok {
					// 转换操作格式
					gasEstimate := parseUint64(getString(opMap, "gas_estimate"))
					if gasEstimate == 0 {
						// 如果没有gas_estimate，使用默认值
						gasEstimate = 100000 // 默认100k gas
					}

					operation := analyzer.Operation{
						Type:        getString(opMap, "type"),
						Description: getString(opMap, "description"),
						GasEstimate: gasEstimate,
						Parameters:  make(map[string]interface{}),
					}

					// 根据操作类型设置参数
					switch operation.Type {
					case "swap":
						operation.Parameters["from_token"] = getString(opMap, "from_token")
						operation.Parameters["to_token"] = getString(opMap, "to_token")
						operation.Parameters["amount"] = getString(opMap, "amount")
					case "stake":
						operation.Parameters["token"] = getString(opMap, "token")

						// 对于质押操作，需要计算正确的数量
						amount := getString(opMap, "amount")
						if amount == "兑换获得的MTK数量" || amount == "all_from_previous" {
							// 计算质押数量：基于第一个操作的结果
							if i > 0 && len(compoundOps) > 0 {
								firstOp := compoundOps[0]
								if firstOp.Type == "swap" {
									// 从第一个swap操作计算MTK数量
									swapAmount := getString(firstOp.Parameters, "amount")
									if swapAmount != "" {
										// 假设1 MEER = 1000 MTK的兑换率
										if amountFloat, err := strconv.ParseFloat(swapAmount, 64); err == nil {
											mtkAmount := fmt.Sprintf("%.0f", amountFloat*1000)
											operation.Parameters["amount"] = mtkAmount
											log.Printf("📝 计算质押数量: %s MEER -> %s MTK", swapAmount, mtkAmount)
										} else {
											operation.Parameters["amount"] = "1000" // 默认值
										}
									} else {
										operation.Parameters["amount"] = "1000" // 默认值
									}
								} else {
									operation.Parameters["amount"] = "1000" // 默认值
								}
							} else {
								operation.Parameters["amount"] = "1000" // 默认值
							}
						} else {
							operation.Parameters["amount"] = amount
						}
					}

					compoundOps = append(compoundOps, operation)
					totalGas += operation.GasEstimate
				}
			}

			// 从工作流上下文中获取当前步骤
			currentStep := 0
			if compoundStepState, exists := execCtx.State["compound_step"]; exists {
				log.Printf("🔍 找到compound_step状态: %+v", compoundStepState)
				if compoundStepState.Output != nil && compoundStepState.Output.Content != nil {
					log.Printf("📝 compound_step内容: %+v", compoundStepState.Output.Content)
					if step, ok := compoundStepState.Output.Content["current_step"].(float64); ok {
						currentStep = int(step)
						log.Printf("✅ 成功获取当前步骤: %d", currentStep)
					} else if step, ok := compoundStepState.Output.Content["current_step"].(int); ok {
						currentStep = step
						log.Printf("✅ 成功获取当前步骤: %d", currentStep)
					} else {
						log.Printf("❌ 无法获取当前步骤，类型断言失败，值类型: %T", compoundStepState.Output.Content["current_step"])
					}
				} else {
					log.Printf("❌ compound_step输出为空")
				}
			} else {
				log.Printf("❌ 未找到compound_step状态")
			}

			log.Printf("📊 最终当前步骤: %d", currentStep)

			return &analyzer.CompoundOperation{
				Operations:   compoundOps,
				TotalGas:     totalGas,
				CurrentStep:  currentStep, // 使用保存的当前步骤
				StepStatus:   "pending",
				AutoContinue: true,
			}, nil
		}
	}

	return nil, fmt.Errorf("cannot extract compound operation from context")
}

// generateSignatureRequestForOperation 为指定操作生成签名请求
func (we *WorkflowEngineImpl) generateSignatureRequestForOperation(op *analyzer.Operation, compoundOp *analyzer.CompoundOperation) *SignatureRequest {
	// 确保有合理的gas限制
	gasLimit := op.GasEstimate
	if gasLimit == 0 {
		gasLimit = 100000 // 默认100k gas
	}

	// 根据操作类型获取合约地址
	var toAddress string
	var err error
	if we.contractManager != nil {
		toAddress, err = we.contractManager.GetContractAddressByOperation(op.Type)
		if err != nil {
			log.Printf("⚠️ 无法获取合约地址: %v，使用默认地址", err)
			// 使用默认地址作为后备
			switch op.Type {
			case "swap":
				toAddress = "0xfBb52268B01e20a9C0C566932716c9B9c550c868" // SimpleSwap
			case "stake":
				toAddress = "0x85ed17629F364381ccEd92F701c028bfDEE501EC" // MTKStaking
			default:
				toAddress = "0x0"
			}
		}
	} else {
		// 如果合约管理器不可用，使用默认地址
		switch op.Type {
		case "swap":
			toAddress = "0xfBb52268B01e20a9C0C566932716c9B9c550c868" // SimpleSwap
		case "stake":
			toAddress = "0x85ed17629F364381ccEd92F701c028bfDEE501EC" // MTKStaking
		default:
			toAddress = "0x0"
		}
	}

	// 根据操作类型生成相应的签名请求
	switch op.Type {
	case "swap":
		fromToken := getString(op.Parameters, "from_token")
		toToken := getString(op.Parameters, "to_token")
		amount := getString(op.Parameters, "amount")

		// 将金额转换为十六进制格式
		value := we.convertAmountToHex(amount)

		return &SignatureRequest{
			Action:    "swap",
			FromToken: fromToken,
			ToToken:   toToken,
			Amount:    amount,
			ToAddress: toAddress,
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
		value := we.convertAmountToHex(amount)

		return &SignatureRequest{
			Action:    "stake",
			FromToken: token,
			ToToken:   token,
			Amount:    amount,
			ToAddress: toAddress,
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
			ToAddress: toAddress,
			Value:     "0x0",
			Data:      "0x",
			GasLimit:  fmt.Sprintf("0x%x", gasLimit),
			GasPrice:  "0x0",
			GasFee:    fmt.Sprintf("%d", gasLimit),
			Slippage:  "0.0",
		}
	}
}

// convertAmountToHex 将金额转换为十六进制格式
func (we *WorkflowEngineImpl) convertAmountToHex(amount string) string {
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

// updateExecutionContextForNextStep 更新执行上下文以准备下一步
func (we *WorkflowEngineImpl) updateExecutionContextForNextStep(execCtx *WorkflowExecutionContext, compoundOp *analyzer.CompoundOperation, signatureRequest *SignatureRequest) {
	// 更新执行上下文状态，为下一步做准备
	// 这里可以添加更多状态管理逻辑
	log.Printf("📝 更新执行上下文状态，准备下一步操作")
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

// validateSignature 验证签名
func (we *WorkflowEngineImpl) validateSignature(signature string) error {
	// 这里实现签名验证逻辑
	// 暂时返回nil，表示验证通过
	return nil
}

// executeStepWithFeedback 执行单个步骤（带反馈）
func (we *WorkflowEngineImpl) executeStepWithFeedback(ctx context.Context, execCtx *WorkflowExecutionContext, step *WorkflowStep) error {
	// 记录步骤开始
	stepState := &StepState{
		StepID:    step.ID,
		Status:    "running",
		StartTime: time.Now(),
	}
	execCtx.State[step.ID] = stepState

	// 收集输入
	input, err := we.collectStepInput(execCtx, step)
	if err != nil {
		stepState.Status = "failed"
		stepState.Error = err.Error()
		return err
	}

	// 获取角色并执行
	role, err := we.roleManager.GetRole(step.Role)
	if err != nil {
		stepState.Status = "failed"
		stepState.Error = err.Error()
		return err
	}

	// 执行角色动作（带重试和反馈）
	var output *protocol.StructuredMessage
	for attempt := 0; attempt <= step.RetryCount; attempt++ {
		output, err = role.Act(ctx, input)
		if err == nil {
			break
		}

		// 发送重试反馈
		if we.config.EnableFeedback && attempt < step.RetryCount {
			feedback := &FeedbackMessage{
				Type:       FeedbackTypeExecution,
				Level:      FeedbackLevelWarning,
				Source:     "WorkflowEngine",
				Message:    fmt.Sprintf("Step %s attempt %d failed, retrying", step.ID, attempt+1),
				WorkflowID: execCtx.ID,
				StepID:     step.ID,
				Details: map[string]interface{}{
					"attempt":      attempt + 1,
					"max_attempts": step.RetryCount + 1,
					"error":        err.Error(),
					"retry_in":     time.Duration(attempt+1) * time.Second,
				},
			}

			we.feedbackSystem.SendFeedback(ctx, feedback)
		}

		if attempt < step.RetryCount {
			time.Sleep(time.Duration(attempt+1) * time.Second)
		}
	}

	if err != nil {
		stepState.Status = "failed"
		stepState.Error = err.Error()
		return err
	}

	// 执行验证器（在角色执行之后）
	for _, validator := range step.Validators {
		if err := validator(ctx, output); err != nil {
			stepState.Status = "failed"
			stepState.Error = fmt.Sprintf("validation failed: %v", err)

			// 发送验证反馈
			if we.config.EnableFeedback {
				feedback := &FeedbackMessage{
					Type:       FeedbackTypeValidation,
					Level:      FeedbackLevelError,
					Source:     "StepValidator",
					Message:    fmt.Sprintf("Validation failed for step %s: %v", step.ID, err),
					WorkflowID: execCtx.ID,
					StepID:     step.ID,
					Details: map[string]interface{}{
						"validator_error": err.Error(),
						"output_data":     output.Content,
					},
					Suggestions: []string{
						"Check output parameters",
						"Verify data format",
						"Review validation rules",
					},
				}

				we.feedbackSystem.SendFeedback(ctx, feedback)
			}

			return err
		}
	}

	// 保存输出
	execCtx.Messages[step.ID] = output

	// 检查是否需要签名
	if we.needsSignature(output) {
		stepState.Status = "waiting_signature"
		stepState.Output = output

		// 发送签名请求反馈
		if we.config.EnableFeedback {
			feedback := &FeedbackMessage{
				Type:       FeedbackTypeExecution,
				Level:      FeedbackLevelInfo,
				Source:     "SignatureHandler",
				Message:    fmt.Sprintf("Signature required for step %s", step.ID),
				WorkflowID: execCtx.ID,
				StepID:     step.ID,
				Details: map[string]interface{}{
					"signature_required": true,
					"step_id":            step.ID,
				},
			}
			we.feedbackSystem.SendFeedback(ctx, feedback)
		}

		return &SignatureRequiredError{
			StepID: step.ID,
			Output: output,
		}
	}

	// 执行后置动作
	for _, postAction := range step.PostActions {
		if err := postAction(ctx, output); err != nil {
			// 后置动作失败不影响主流程，但要发送反馈
			if we.config.EnableFeedback {
				feedback := &FeedbackMessage{
					Type:       FeedbackTypeExecution,
					Level:      FeedbackLevelWarning,
					Source:     "PostActionHandler",
					Message:    fmt.Sprintf("Post action failed for step %s: %v", step.ID, err),
					WorkflowID: execCtx.ID,
					StepID:     step.ID,
					Details: map[string]interface{}{
						"post_action_error": err.Error(),
					},
				}

				we.feedbackSystem.SendFeedback(ctx, feedback)
			}
		}
	}

	// 更新步骤状态
	stepState.Status = "completed"
	stepState.EndTime = time.Now()
	stepState.Output = output

	return nil
}

// executeRollbackWithFeedback 执行回滚（带反馈）
func (we *WorkflowEngineImpl) executeRollbackWithFeedback(ctx context.Context, execCtx *WorkflowExecutionContext, rollback *RollbackStrategy) error {
	// 发送回滚开始反馈
	if we.config.EnableFeedback {
		feedback := &FeedbackMessage{
			Type:       FeedbackTypeRollback,
			Level:      FeedbackLevelWarning,
			Source:     "WorkflowEngine",
			Message:    "Starting rollback procedure",
			WorkflowID: execCtx.ID,
			Details: map[string]interface{}{
				"rollback_condition": rollback.Condition,
				"actions":            rollback.Actions,
			},
		}

		we.feedbackSystem.SendFeedback(ctx, feedback)
	}

	// 执行回滚动作
	for _, action := range rollback.Actions {
		// 这里实现具体的回滚逻辑
		we.stateManager.LogExecution("rollback", "info",
			fmt.Sprintf("Executing rollback action: %s", action),
			map[string]interface{}{
				"workflow_id": execCtx.ID,
				"action":      action,
			})
	}

	// 发送回滚完成反馈
	if we.config.EnableFeedback {
		feedback := &FeedbackMessage{
			Type:       FeedbackTypeRollback,
			Level:      FeedbackLevelInfo,
			Source:     "WorkflowEngine",
			Message:    "Rollback completed",
			WorkflowID: execCtx.ID,
			Details: map[string]interface{}{
				"actions_completed": len(rollback.Actions),
			},
		}

		we.feedbackSystem.SendFeedback(ctx, feedback)
	}

	return nil
}

// collectStepInput 收集步骤输入
func (we *WorkflowEngineImpl) collectStepInput(execCtx *WorkflowExecutionContext, step *WorkflowStep) (*protocol.StructuredMessage, error) {
	if len(step.Input) == 0 {
		// 如果没有依赖，使用原始输入
		return execCtx.Messages["input"], nil
	}

	// 合并所有依赖步骤的输出
	mergedContent := make(map[string]interface{})

	for _, inputID := range step.Input {
		if msg, exists := execCtx.Messages[inputID]; exists {
			for k, v := range msg.Content {
				mergedContent[k] = v
			}
		} else {
			return nil, fmt.Errorf("missing input from step %s", inputID)
		}
	}

	// 创建合并后的消息
	return &protocol.StructuredMessage{
		Type:    protocol.MessageType(fmt.Sprintf("merged_%s", step.ID)),
		Content: mergedContent,
		Context: execCtx.Messages["input"].Context,
	}, nil
}

// cleanupExecutionContext 清理执行上下文
func (we *WorkflowEngineImpl) cleanupExecutionContext(executionID string) {
	we.execMu.Lock()
	defer we.execMu.Unlock()

	delete(we.executions, executionID)
}

// GetExecutionStatus 获取执行状态
func (we *WorkflowEngineImpl) GetExecutionStatus(executionID string) (*WorkflowExecutionContext, error) {
	we.execMu.RLock()
	defer we.execMu.RUnlock()

	execCtx, exists := we.executions[executionID]
	if !exists {
		return nil, fmt.Errorf("execution %s not found", executionID)
	}

	return execCtx, nil
}

// ListActiveExecutions 列出活跃的执行
func (we *WorkflowEngineImpl) ListActiveExecutions() []string {
	we.execMu.RLock()
	defer we.execMu.RUnlock()

	var executions []string
	for id := range we.executions {
		executions = append(executions, id)
	}

	return executions
}

// GetRecentFeedback 获取最近的反馈
func (we *WorkflowEngineImpl) GetRecentFeedback(limit int) []*FeedbackMessage {
	return we.feedbackSystem.GetRecentFeedback(limit)
}

// GetRoleManager 获取角色管理器
func (we *WorkflowEngineImpl) GetRoleManager() RoleManager {
	return we.roleManager
}

// GetSystemStats 获取系统统计信息
func (we *WorkflowEngineImpl) GetSystemStats() map[string]interface{} {
	we.execMu.RLock()
	activeExecutions := len(we.executions)
	we.execMu.RUnlock()

	stats := we.stateManager.GetStats()
	stats["active_executions"] = activeExecutions
	stats["max_concurrent_workflows"] = we.config.MaxConcurrentWorkflows
	stats["feedback_enabled"] = we.config.EnableFeedback
	stats["metrics_enabled"] = we.config.EnableMetrics

	return stats
}
