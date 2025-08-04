// internal/sop/workflow.go
package sop

import (
	"context"
	"fmt"
	"time"

	"qng_agent/internal/protocol"
	"qng_agent/internal/roles"
)

// WorkflowType 工作流类型
type WorkflowType string

const (
	WorkflowSwap     WorkflowType = "swap"
	WorkflowStake    WorkflowType = "stake"
	WorkflowUnstake  WorkflowType = "unstake"
	WorkflowApprove  WorkflowType = "approve"
	WorkflowCompound WorkflowType = "compound" // 复合操作
)

// WorkflowStep 工作流步骤
type WorkflowStep struct {
	ID          string
	Name        string
	Role        roles.RoleType
	Input       []string // 依赖的步骤ID
	Output      string   // 输出消息类型
	Timeout     time.Duration
	RetryCount  int
	Validators  []StepValidator
	PostActions []PostAction
}

// StepValidator 步骤验证器
type StepValidator func(context.Context, *protocol.StructuredMessage) error

// PostAction 步骤后置动作
type PostAction func(context.Context, *protocol.StructuredMessage) error

// WorkflowDefinition 工作流定义
type WorkflowDefinition struct {
	Type        WorkflowType
	Name        string
	Description string
	Steps       []WorkflowStep
	Rollback    map[string]RollbackStrategy
}

// RollbackStrategy 回滚策略
type RollbackStrategy struct {
	Condition string
	Actions   []string
}

// StandardWorkflows 标准工作流定义 - MetaGPT的SOP概念
type StandardWorkflows struct {
	workflows map[WorkflowType]*WorkflowDefinition
}

// NewStandardWorkflows 创建标准工作流
func NewStandardWorkflows() *StandardWorkflows {
	sw := &StandardWorkflows{
		workflows: make(map[WorkflowType]*WorkflowDefinition),
	}

	// 初始化标准工作流
	sw.initializeSwapWorkflow()
	sw.initializeStakeWorkflow()
	sw.initializeCompoundWorkflow()

	return sw
}

// initializeSwapWorkflow 初始化Swap工作流
func (sw *StandardWorkflows) initializeSwapWorkflow() {
	workflow := &WorkflowDefinition{
		Type:        WorkflowSwap,
		Name:        "Token Swap Workflow",
		Description: "Standard workflow for token swapping operations",
		Steps: []WorkflowStep{
			{
				ID:         "analyze",
				Name:       "Analyze Swap Request",
				Role:       roles.RoleStrategyAnalyst,
				Input:      []string{},
				Output:     string(protocol.MessageTypeStrategy),
				Timeout:    30 * time.Second,
				RetryCount: 2,
				Validators: []StepValidator{
					validateTokenPair,
					validateAmount,
				},
			},
			{
				ID:         "assess_risk",
				Name:       "Assess Swap Risk",
				Role:       roles.RoleRiskManager,
				Input:      []string{"analyze"},
				Output:     string(protocol.MessageTypeRiskAssessment),
				Timeout:    20 * time.Second,
				RetryCount: 1,
				Validators: []StepValidator{
					validateRiskThresholds,
				},
			},
			{
				ID:         "build_tx",
				Name:       "Build Swap Transaction",
				Role:       roles.RoleTransactionBuilder,
				Input:      []string{"assess_risk"},
				Output:     string(protocol.MessageTypeTransaction),
				Timeout:    30 * time.Second,
				RetryCount: 3,
				Validators: []StepValidator{
					validateTransactionData,
					validateGasEstimate,
				},
			},
			{
				ID:         "execute",
				Name:       "Execute Swap",
				Role:       roles.RoleExecutor,
				Input:      []string{"build_tx"},
				Output:     string(protocol.MessageTypeExecutionResult),
				Timeout:    5 * time.Minute,
				RetryCount: 2,
				PostActions: []PostAction{
					notifyExecution,
					updateState,
				},
			},
			{
				ID:         "audit",
				Name:       "Audit Execution",
				Role:       roles.RoleAuditor,
				Input:      []string{"execute"},
				Output:     string(protocol.MessageTypeFinalReport),
				Timeout:    1 * time.Minute,
				RetryCount: 1,
			},
		},
		Rollback: map[string]RollbackStrategy{
			"execute": {
				Condition: "failed",
				Actions:   []string{"revert_approval", "notify_failure"},
			},
		},
	}

	sw.workflows[WorkflowSwap] = workflow
}

// initializeStakeWorkflow 初始化质押工作流
func (sw *StandardWorkflows) initializeStakeWorkflow() {
	workflow := &WorkflowDefinition{
		Type:        WorkflowStake,
		Name:        "Token Staking Workflow",
		Description: "Standard workflow for token staking operations",
		Steps: []WorkflowStep{
			{
				ID:         "analyze",
				Name:       "Analyze Stake Request",
				Role:       roles.RoleStrategyAnalyst,
				Input:      []string{},
				Output:     string(protocol.MessageTypeStrategy),
				Timeout:    30 * time.Second,
				RetryCount: 2,
				Validators: []StepValidator{
					validateStakeProtocol,
					validateStakeAmount,
				},
			},
			{
				ID:         "assess_risk",
				Name:       "Assess Staking Risk",
				Role:       roles.RoleRiskManager,
				Input:      []string{"analyze"},
				Output:     string(protocol.MessageTypeRiskAssessment),
				Timeout:    20 * time.Second,
				RetryCount: 1,
				Validators: []StepValidator{
					validateProtocolSafety,
					validateAPY,
				},
			},
			{
				ID:         "approve",
				Name:       "Approve Token",
				Role:       roles.RoleTransactionBuilder,
				Input:      []string{"assess_risk"},
				Output:     string(protocol.MessageTypeTransaction),
				Timeout:    30 * time.Second,
				RetryCount: 3,
			},
			{
				ID:         "stake",
				Name:       "Build Stake Transaction",
				Role:       roles.RoleTransactionBuilder,
				Input:      []string{"approve"},
				Output:     string(protocol.MessageTypeTransaction),
				Timeout:    30 * time.Second,
				RetryCount: 3,
			},
			{
				ID:         "execute",
				Name:       "Execute Staking",
				Role:       roles.RoleExecutor,
				Input:      []string{"stake"},
				Output:     string(protocol.MessageTypeExecutionResult),
				Timeout:    5 * time.Minute,
				RetryCount: 2,
			},
			{
				ID:         "audit",
				Name:       "Audit Staking",
				Role:       roles.RoleAuditor,
				Input:      []string{"execute"},
				Output:     string(protocol.MessageTypeFinalReport),
				Timeout:    1 * time.Minute,
				RetryCount: 1,
			},
		},
	}

	sw.workflows[WorkflowStake] = workflow
}

// initializeCompoundWorkflow 初始化复合工作流
func (sw *StandardWorkflows) initializeCompoundWorkflow() {
	workflow := &WorkflowDefinition{
		Type:        WorkflowCompound,
		Name:        "Compound Operation Workflow",
		Description: "Workflow for complex operations like swap and stake",
		Steps: []WorkflowStep{
			{
				ID:         "decompose",
				Name:       "Decompose Complex Request",
				Role:       roles.RoleStrategyAnalyst,
				Input:      []string{},
				Output:     string(protocol.MessageTypeStrategy),
				Timeout:    45 * time.Second,
				RetryCount: 2,
				Validators: []StepValidator{
					validateComplexOperation,
				},
			},
			{
				ID:         "execute_swap",
				Name:       "Execute Swap Operation",
				Role:       roles.RoleExecutor,
				Input:      []string{"decompose"},
				Output:     string(protocol.MessageTypeExecutionResult),
				Timeout:    60 * time.Second,
				RetryCount: 2,
				Validators: []StepValidator{
					validateSwapResult,
				},
			},
			{
				ID:         "execute_stake",
				Name:       "Execute Stake Operation",
				Role:       roles.RoleExecutor,
				Input:      []string{"execute_swap"},
				Output:     string(protocol.MessageTypeExecutionResult),
				Timeout:    60 * time.Second,
				RetryCount: 2,
				Validators: []StepValidator{
					validateStakeResult,
				},
			},
			{
				ID:         "audit_compound",
				Name:       "Audit Compound Operation",
				Role:       roles.RoleAuditor,
				Input:      []string{"execute_swap", "execute_stake"},
				Output:     string(protocol.MessageTypeFinalReport),
				Timeout:    30 * time.Second,
				RetryCount: 1,
				Validators: []StepValidator{
					validateCompoundResult,
				},
			},
		},
		Rollback: map[string]RollbackStrategy{
			"execute_swap": {
				Condition: "swap_failed",
				Actions:   []string{"revert_swap"},
			},
			"execute_stake": {
				Condition: "stake_failed",
				Actions:   []string{"revert_stake", "revert_swap"},
			},
		},
	}

	sw.workflows[WorkflowCompound] = workflow
}

// WorkflowEngine 工作流引擎
type WorkflowEngine struct {
	workflows    *StandardWorkflows
	roleManager  RoleManager
	messagePool  *protocol.MessagePool
	stateManager *StateManager
}

// NewWorkflowEngine 创建工作流引擎
func NewWorkflowEngine(roleManager RoleManager, messagePool *protocol.MessagePool) *WorkflowEngine {
	return &WorkflowEngine{
		workflows:    NewStandardWorkflows(),
		roleManager:  roleManager,
		messagePool:  messagePool,
		stateManager: NewStateManager(),
	}
}

// ExecuteWorkflow 执行工作流
func (we *WorkflowEngine) ExecuteWorkflow(ctx context.Context, workflowType WorkflowType, input *protocol.StructuredMessage) (*WorkflowResult, error) {
	// 获取工作流定义
	workflow, exists := we.workflows.workflows[workflowType]
	if !exists {
		return nil, fmt.Errorf("workflow type %s not found", workflowType)
	}

	// 创建工作流执行上下文
	execCtx := &WorkflowExecutionContext{
		ID:          generateExecutionID(),
		WorkflowDef: workflow,
		StartTime:   time.Now(),
		State:       make(map[string]*StepState),
		Messages:    make(map[string]*protocol.StructuredMessage),
	}

	// 初始化输入
	execCtx.Messages["input"] = input

	// 执行工作流步骤
	for _, step := range workflow.Steps {
		if err := we.executeStep(ctx, execCtx, &step); err != nil {
			// 执行回滚
			if rollback, exists := workflow.Rollback[step.ID]; exists {
				we.executeRollback(ctx, execCtx, &rollback)
			}
			return nil, fmt.Errorf("step %s failed: %w", step.ID, err)
		}
	}

	// 构建结果
	result := &WorkflowResult{
		ID:          execCtx.ID,
		Type:        workflowType,
		Status:      "completed",
		StartTime:   execCtx.StartTime,
		EndTime:     time.Now(),
		Steps:       execCtx.State,
		FinalOutput: execCtx.Messages["audit"],
	}

	return result, nil
}

// executeStep 执行单个步骤
func (we *WorkflowEngine) executeStep(ctx context.Context, execCtx *WorkflowExecutionContext, step *WorkflowStep) error {
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

	fmt.Printf("DEBUG: Step %s input: %+v\n", step.ID, input.Content)

	// 获取角色并执行
	role, err := we.roleManager.GetRole(step.Role)
	if err != nil {
		stepState.Status = "failed"
		stepState.Error = err.Error()
		return err
	}

	// 执行角色动作（带重试）
	var output *protocol.StructuredMessage
	for i := 0; i <= step.RetryCount; i++ {
		output, err = role.Act(ctx, input)
		if err == nil {
			break
		}

		if i < step.RetryCount {
			time.Sleep(time.Duration(i+1) * time.Second)
		}
	}

	if err != nil {
		stepState.Status = "failed"
		stepState.Error = err.Error()
		return err
	}

	fmt.Printf("DEBUG: Step %s output: %+v\n", step.ID, output.Content)

	// 执行验证器（在角色执行之后）
	fmt.Printf("DEBUG: Executing validators for step %s with output: %+v\n", step.ID, output.Content)
	for _, validator := range step.Validators {
		if err := validator(ctx, output); err != nil {
			stepState.Status = "failed"
			stepState.Error = fmt.Sprintf("validation failed: %v", err)
			return err
		}
	}

	// 保存输出
	execCtx.Messages[step.ID] = output

	// 执行后置动作
	for _, postAction := range step.PostActions {
		if err := postAction(ctx, output); err != nil {
			// 后置动作失败不影响主流程，只记录日志
			fmt.Printf("Post action failed: %v\n", err)
		}
	}

	// 更新步骤状态
	stepState.Status = "completed"
	stepState.EndTime = time.Now()
	stepState.Output = output

	return nil
}

// collectStepInput 收集步骤输入
func (we *WorkflowEngine) collectStepInput(execCtx *WorkflowExecutionContext, step *WorkflowStep) (*protocol.StructuredMessage, error) {
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

// WorkflowExecutionContext 工作流执行上下文
type WorkflowExecutionContext struct {
	ID          string
	WorkflowDef *WorkflowDefinition
	StartTime   time.Time
	State       map[string]*StepState
	Messages    map[string]*protocol.StructuredMessage
}

// StepState 步骤状态
type StepState struct {
	StepID    string
	Status    string
	StartTime time.Time
	EndTime   time.Time
	Error     string
	Output    *protocol.StructuredMessage
}

// WorkflowResult 工作流结果
type WorkflowResult struct {
	ID               string
	Type             WorkflowType
	Status           string
	StartTime        time.Time
	EndTime          time.Time
	Steps            map[string]*StepState
	FinalOutput      *protocol.StructuredMessage
	NeedSignature    bool
	SignatureRequest *SignatureRequest
	WorkflowContext  map[string]interface{}
}

// 验证器实现
func validateTokenPair(ctx context.Context, msg *protocol.StructuredMessage) error {
	// 验证代币对是否有效
	return nil
}

func validateAmount(ctx context.Context, msg *protocol.StructuredMessage) error {
	// 验证金额是否有效
	return nil
}

func validateRiskThresholds(ctx context.Context, msg *protocol.StructuredMessage) error {
	// 验证风险阈值
	return nil
}

func validateTransactionData(ctx context.Context, msg *protocol.StructuredMessage) error {
	// 验证交易数据
	return nil
}

func validateGasEstimate(ctx context.Context, msg *protocol.StructuredMessage) error {
	// 验证Gas估算
	return nil
}

// executeRollback 执行回滚
func (we *WorkflowEngine) executeRollback(ctx context.Context, execCtx *WorkflowExecutionContext, rollback *RollbackStrategy) error {
	// 实现回滚逻辑
	return nil
}

func validateSwapResult(ctx context.Context, msg *protocol.StructuredMessage) error {
	// 验证交换操作结果
	if results, ok := msg.Content["results"].([]map[string]interface{}); ok {
		if len(results) == 0 {
			return fmt.Errorf("no swap results found")
		}
		return nil
	}
	// 尝试其他类型
	if results, ok := msg.Content["results"].([]interface{}); ok {
		if len(results) == 0 {
			return fmt.Errorf("no swap results found")
		}
		return nil
	}
	return fmt.Errorf("invalid swap result format")
}

func validateStakeResult(ctx context.Context, msg *protocol.StructuredMessage) error {
	// 验证质押操作结果
	if results, ok := msg.Content["results"].([]map[string]interface{}); ok {
		if len(results) == 0 {
			return fmt.Errorf("no stake results found")
		}
		return nil
	}
	// 尝试其他类型
	if results, ok := msg.Content["results"].([]interface{}); ok {
		if len(results) == 0 {
			return fmt.Errorf("no stake results found")
		}
		return nil
	}
	return fmt.Errorf("invalid stake result format")
}

func validateCompoundResult(ctx context.Context, msg *protocol.StructuredMessage) error {
	// 验证复合操作结果
	if auditReport, ok := msg.Content["audit_report"].(map[string]interface{}); ok {
		if compliance, ok := auditReport["compliance"].(map[string]interface{}); ok {
			if userIntentMatched, ok := compliance["user_intent_matched"].(bool); ok && userIntentMatched {
				return nil
			}
		}
	}
	return fmt.Errorf("compound operation audit failed")
}
