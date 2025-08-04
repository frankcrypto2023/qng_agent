// internal/sop/feedback.go
package sop

import (
	"context"
	"fmt"
	"sync"
	"time"
	
	"qng_agent/internal/protocol"
)

// FeedbackType 反馈类型
type FeedbackType string

const (
	FeedbackTypeTransaction FeedbackType = "transaction"
	FeedbackTypeValidation  FeedbackType = "validation"
	FeedbackTypeExecution   FeedbackType = "execution"
	FeedbackTypeRollback    FeedbackType = "rollback"
	FeedbackTypeError       FeedbackType = "error"
)

// FeedbackLevel 反馈级别
type FeedbackLevel string

const (
	FeedbackLevelInfo    FeedbackLevel = "info"
	FeedbackLevelWarning FeedbackLevel = "warning"
	FeedbackLevelError   FeedbackLevel = "error"
	FeedbackLevelCritical FeedbackLevel = "critical"
)

// FeedbackMessage 反馈消息
type FeedbackMessage struct {
	ID          string                 `json:"id"`
	Type        FeedbackType           `json:"type"`
	Level       FeedbackLevel          `json:"level"`
	Source      string                 `json:"source"`
	Message     string                 `json:"message"`
	Details     map[string]interface{} `json:"details,omitempty"`
	Timestamp   time.Time              `json:"timestamp"`
	WorkflowID  string                 `json:"workflow_id,omitempty"`
	StepID      string                 `json:"step_id,omitempty"`
	Suggestions []string               `json:"suggestions,omitempty"`
}

// FeedbackHandler 反馈处理器接口
type FeedbackHandler interface {
	HandleFeedback(ctx context.Context, feedback *FeedbackMessage) error
}

// FeedbackSystem 可执行反馈系统
type FeedbackSystem struct {
	handlers    map[FeedbackType][]FeedbackHandler
	messagePool *protocol.MessagePool
	stateManager *StateManager
	mu          sync.RWMutex
	
	// 反馈缓冲区
	feedbackBuffer []*FeedbackMessage
	bufferSize     int
	bufferMu       sync.Mutex
}

// NewFeedbackSystem 创建反馈系统
func NewFeedbackSystem(messagePool *protocol.MessagePool, stateManager *StateManager) *FeedbackSystem {
	return &FeedbackSystem{
		handlers:       make(map[FeedbackType][]FeedbackHandler),
		messagePool:    messagePool,
		stateManager:   stateManager,
		feedbackBuffer: make([]*FeedbackMessage, 0),
		bufferSize:     100,
	}
}

// RegisterHandler 注册反馈处理器
func (fs *FeedbackSystem) RegisterHandler(feedbackType FeedbackType, handler FeedbackHandler) {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	
	fs.handlers[feedbackType] = append(fs.handlers[feedbackType], handler)
}

// SendFeedback 发送反馈
func (fs *FeedbackSystem) SendFeedback(ctx context.Context, feedback *FeedbackMessage) error {
	// 添加时间戳
	feedback.Timestamp = time.Now()
	feedback.ID = fmt.Sprintf("feedback_%d", time.Now().UnixNano())
	
	// 添加到缓冲区
	fs.addToBuffer(feedback)
	
	// 记录到状态管理器
	fs.stateManager.LogExecution(
		string(feedback.Type),
		string(feedback.Level), 
		feedback.Message,
		feedback.Details,
	)
	
	// 获取处理器
	fs.mu.RLock()
	handlers, exists := fs.handlers[feedback.Type]
	fs.mu.RUnlock()
	
	if !exists {
		return fmt.Errorf("no handlers registered for feedback type: %s", feedback.Type)
	}
	
	// 并发处理反馈
	var wg sync.WaitGroup
	errorChan := make(chan error, len(handlers))
	
	for _, handler := range handlers {
		wg.Add(1)
		go func(h FeedbackHandler) {
			defer wg.Done()
			if err := h.HandleFeedback(ctx, feedback); err != nil {
				errorChan <- err
			}
		}(handler)
	}
	
	wg.Wait()
	close(errorChan)
	
	// 收集错误
	var errors []error
	for err := range errorChan {
		errors = append(errors, err)
	}
	
	if len(errors) > 0 {
		return fmt.Errorf("feedback handling errors: %v", errors)
	}
	
	return nil
}

// addToBuffer 添加到反馈缓冲区
func (fs *FeedbackSystem) addToBuffer(feedback *FeedbackMessage) {
	fs.bufferMu.Lock()
	defer fs.bufferMu.Unlock()
	
	fs.feedbackBuffer = append(fs.feedbackBuffer, feedback)
	
	// 限制缓冲区大小
	if len(fs.feedbackBuffer) > fs.bufferSize {
		fs.feedbackBuffer = fs.feedbackBuffer[1:]
	}
}

// GetRecentFeedback 获取最近的反馈
func (fs *FeedbackSystem) GetRecentFeedback(limit int) []*FeedbackMessage {
	fs.bufferMu.Lock()
	defer fs.bufferMu.Unlock()
	
	if limit <= 0 || limit > len(fs.feedbackBuffer) {
		limit = len(fs.feedbackBuffer)
	}
	
	start := len(fs.feedbackBuffer) - limit
	if start < 0 {
		start = 0
	}
	
	result := make([]*FeedbackMessage, limit)
	copy(result, fs.feedbackBuffer[start:])
	
	return result
}

// TransactionFeedbackHandler 交易反馈处理器
type TransactionFeedbackHandler struct {
	onChainValidator OnChainValidator
}

// OnChainValidator 链上验证器接口
type OnChainValidator interface {
	ValidateTransaction(ctx context.Context, txHash string) (*TransactionValidationResult, error)
	GetTransactionStatus(ctx context.Context, txHash string) (string, error)
}

// TransactionValidationResult 交易验证结果
type TransactionValidationResult struct {
	Valid       bool                   `json:"valid"`
	TxHash      string                 `json:"tx_hash"`
	Status      string                 `json:"status"`
	GasUsed     uint64                 `json:"gas_used"`
	BlockNumber uint64                 `json:"block_number"`
	Details     map[string]interface{} `json:"details"`
	Issues      []string               `json:"issues,omitempty"`
}

// NewTransactionFeedbackHandler 创建交易反馈处理器
func NewTransactionFeedbackHandler(validator OnChainValidator) *TransactionFeedbackHandler {
	return &TransactionFeedbackHandler{
		onChainValidator: validator,
	}
}

// HandleFeedback 处理交易反馈
func (tfh *TransactionFeedbackHandler) HandleFeedback(ctx context.Context, feedback *FeedbackMessage) error {
	if feedback.Type != FeedbackTypeTransaction {
		return nil
	}
	
	// 提取交易哈希
	txHash, ok := feedback.Details["tx_hash"].(string)
	if !ok || txHash == "" {
		return fmt.Errorf("transaction hash not found in feedback details")
	}
	
	// 验证交易
	result, err := tfh.onChainValidator.ValidateTransaction(ctx, txHash)
	if err != nil {
		return fmt.Errorf("failed to validate transaction: %w", err)
	}
	
	// 如果交易无效，发送警告反馈
	if !result.Valid {
		warningFeedback := &FeedbackMessage{
			Type:    FeedbackTypeValidation,
			Level:   FeedbackLevelWarning,
			Source:  "TransactionValidator",
			Message: fmt.Sprintf("Transaction validation failed: %s", txHash),
			Details: map[string]interface{}{
				"tx_hash":      txHash,
				"validation":   result,
				"original_feedback": feedback.ID,
			},
			Suggestions: []string{
				"Check transaction parameters",
				"Verify contract interaction",
				"Review gas settings",
			},
		}
		
		// 这里应该有方法将反馈发送回系统
		fmt.Printf("Validation warning: %+v\n", warningFeedback)
	}
	
	return nil
}

// ValidationFeedbackHandler 验证反馈处理器
type ValidationFeedbackHandler struct {
	autoFix       bool
	maxRetries    int
	retryStrategy RetryStrategy
}

// RetryStrategy 重试策略
type RetryStrategy interface {
	ShouldRetry(attempt int, err error) bool
	GetRetryDelay(attempt int) time.Duration
}

// ExponentialBackoffStrategy 指数退避重试策略
type ExponentialBackoffStrategy struct {
	BaseDelay time.Duration
	MaxDelay  time.Duration
	Factor    float64
}

// ShouldRetry 是否应该重试
func (ebs *ExponentialBackoffStrategy) ShouldRetry(attempt int, err error) bool {
	return attempt < 3 // 最多重试3次
}

// GetRetryDelay 获取重试延迟
func (ebs *ExponentialBackoffStrategy) GetRetryDelay(attempt int) time.Duration {
	delay := time.Duration(float64(ebs.BaseDelay) * (ebs.Factor * float64(attempt)))
	if delay > ebs.MaxDelay {
		delay = ebs.MaxDelay
	}
	return delay
}

// NewValidationFeedbackHandler 创建验证反馈处理器
func NewValidationFeedbackHandler(autoFix bool, maxRetries int) *ValidationFeedbackHandler {
	return &ValidationFeedbackHandler{
		autoFix:    autoFix,
		maxRetries: maxRetries,
		retryStrategy: &ExponentialBackoffStrategy{
			BaseDelay: time.Second,
			MaxDelay:  30 * time.Second,
			Factor:    2.0,
		},
	}
}

// HandleFeedback 处理验证反馈
func (vfh *ValidationFeedbackHandler) HandleFeedback(ctx context.Context, feedback *FeedbackMessage) error {
	if feedback.Type != FeedbackTypeValidation {
		return nil
	}
	
	// 根据反馈级别决定处理策略
	switch feedback.Level {
	case FeedbackLevelError, FeedbackLevelCritical:
		if vfh.autoFix {
			return vfh.attemptAutoFix(ctx, feedback)
		}
		return vfh.escalateIssue(ctx, feedback)
		
	case FeedbackLevelWarning:
		return vfh.logWarning(ctx, feedback)
		
	default:
		return vfh.logInfo(ctx, feedback)
	}
}

// attemptAutoFix 尝试自动修复
func (vfh *ValidationFeedbackHandler) attemptAutoFix(ctx context.Context, feedback *FeedbackMessage) error {
	fmt.Printf("Attempting auto-fix for feedback: %s\n", feedback.ID)
	
	// 这里实现自动修复逻辑
	// 例如：调整参数、重新计算、修复配置等
	
	return nil
}

// escalateIssue 升级问题
func (vfh *ValidationFeedbackHandler) escalateIssue(ctx context.Context, feedback *FeedbackMessage) error {
	fmt.Printf("Escalating issue: %s - %s\n", feedback.ID, feedback.Message)
	
	// 这里实现问题升级逻辑
	// 例如：发送通知、暂停操作、记录日志等
	
	return nil
}

// logWarning 记录警告
func (vfh *ValidationFeedbackHandler) logWarning(ctx context.Context, feedback *FeedbackMessage) error {
	fmt.Printf("Warning logged: %s - %s\n", feedback.ID, feedback.Message)
	return nil
}

// logInfo 记录信息
func (vfh *ValidationFeedbackHandler) logInfo(ctx context.Context, feedback *FeedbackMessage) error {
	fmt.Printf("Info logged: %s - %s\n", feedback.ID, feedback.Message)
	return nil
}

// ExecutionFeedbackHandler 执行反馈处理器
type ExecutionFeedbackHandler struct {
	performanceTracker PerformanceTracker
	alertManager       AlertManager
}

// PerformanceTracker 性能跟踪器接口
type PerformanceTracker interface {
	RecordExecution(workflowID string, stepID string, duration time.Duration, success bool)
	GetPerformanceMetrics(workflowID string) *PerformanceMetrics
}

// AlertManager 告警管理器接口
type AlertManager interface {
	SendAlert(ctx context.Context, alert *Alert) error
}

// PerformanceMetrics 性能指标
type PerformanceMetrics struct {
	TotalExecutions    int           `json:"total_executions"`
	SuccessRate        float64       `json:"success_rate"`
	AverageLatency     time.Duration `json:"average_latency"`
	MaxLatency         time.Duration `json:"max_latency"`
	MinLatency         time.Duration `json:"min_latency"`
	ErrorRate          float64       `json:"error_rate"`
	ThroughputPerHour  float64       `json:"throughput_per_hour"`
}

// Alert 告警
type Alert struct {
	ID          string                 `json:"id"`
	Type        string                 `json:"type"`
	Severity    string                 `json:"severity"`
	Title       string                 `json:"title"`
	Description string                 `json:"description"`
	Data        map[string]interface{} `json:"data"`
	Timestamp   time.Time              `json:"timestamp"`
}

// NewExecutionFeedbackHandler 创建执行反馈处理器
func NewExecutionFeedbackHandler(tracker PerformanceTracker, alertMgr AlertManager) *ExecutionFeedbackHandler {
	return &ExecutionFeedbackHandler{
		performanceTracker: tracker,
		alertManager:       alertMgr,
	}
}

// HandleFeedback 处理执行反馈
func (efh *ExecutionFeedbackHandler) HandleFeedback(ctx context.Context, feedback *FeedbackMessage) error {
	if feedback.Type != FeedbackTypeExecution {
		return nil
	}
	
	// 记录性能指标
	if efh.performanceTracker != nil {
		workflowID := feedback.WorkflowID
		stepID := feedback.StepID
		
		if duration, ok := feedback.Details["duration"].(time.Duration); ok {
			success := feedback.Level != FeedbackLevelError && feedback.Level != FeedbackLevelCritical
			efh.performanceTracker.RecordExecution(workflowID, stepID, duration, success)
		}
	}
	
	// 发送告警（如果需要）
	if efh.alertManager != nil && (feedback.Level == FeedbackLevelError || feedback.Level == FeedbackLevelCritical) {
		alert := &Alert{
			ID:          fmt.Sprintf("alert_%d", time.Now().UnixNano()),
			Type:        "execution_failure",
			Severity:    string(feedback.Level),
			Title:       "Workflow Execution Issue",
			Description: feedback.Message,
			Data: map[string]interface{}{
				"feedback_id":  feedback.ID,
				"workflow_id":  feedback.WorkflowID,
				"step_id":      feedback.StepID,
				"source":       feedback.Source,
			},
			Timestamp: time.Now(),
		}
		
		if err := efh.alertManager.SendAlert(ctx, alert); err != nil {
			return fmt.Errorf("failed to send alert: %w", err)
		}
	}
	
	return nil
}