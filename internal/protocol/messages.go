// internal/protocol/messages.go
package protocol

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// MessageType 消息类型
type MessageType string

const (
	MessageTypeUserRequest      MessageType = "UserRequest"
	MessageTypeStrategy         MessageType = "Strategy"
	MessageTypeRiskAssessment   MessageType = "RiskAssessment"
	MessageTypeTransaction      MessageType = "Transaction"
	MessageTypeSignatureRequest MessageType = "SignatureRequest"
	MessageTypeExecutionResult  MessageType = "ExecutionResult"
	MessageTypeFinalReport      MessageType = "FinalReport"
	MessageTypeError            MessageType = "Error"
)

// StructuredMessage 结构化消息 - MetaGPT 的核心创新
type StructuredMessage struct {
	ID             string                 `json:"id"`
	Type           MessageType            `json:"type"`
	SenderRole     string                 `json:"sender_role"`
	RecipientRoles []string               `json:"recipient_roles"`
	Content        map[string]interface{} `json:"content"`
	Schema         string                 `json:"schema"`
	Version        string                 `json:"version"`
	Timestamp      time.Time              `json:"timestamp"`
	Dependencies   []string               `json:"dependencies"`
	Status         MessageStatus          `json:"status"`
	Context        *MessageContext        `json:"context,omitempty"`
}

// MessageStatus 消息状态
type MessageStatus string

const (
	StatusPending    MessageStatus = "pending"
	StatusProcessing MessageStatus = "processing"
	StatusCompleted  MessageStatus = "completed"
	StatusFailed     MessageStatus = "failed"
	StatusCancelled  MessageStatus = "cancelled"
)

// MessageContext 消息上下文
type MessageContext struct {
	SessionID   string            `json:"session_id"`
	UserAddress string            `json:"user_address"`
	ChainID     int64             `json:"chain_id"`
	Priority    int               `json:"priority"`
	Metadata    map[string]string `json:"metadata"`
}

// NewStructuredMessage 创建新的结构化消息
func NewStructuredMessage(msgType MessageType, content map[string]interface{}) *StructuredMessage {
	return &StructuredMessage{
		ID:        uuid.New().String(),
		Type:      msgType,
		Content:   content,
		Version:   "1.0",
		Timestamp: time.Now(),
		Status:    StatusPending,
	}
}

// Validate 验证消息格式
func (sm *StructuredMessage) Validate() error {
	if sm.ID == "" {
		return fmt.Errorf("message ID is required")
	}

	if sm.Type == "" {
		return fmt.Errorf("message type is required")
	}

	// 根据消息类型验证必需字段
	requiredFields := sm.getRequiredFields()
	for _, field := range requiredFields {
		if _, exists := sm.Content[field]; !exists {
			return fmt.Errorf("required field '%s' is missing for message type '%s'", field, sm.Type)
		}
	}

	return nil
}

// getRequiredFields 获取消息类型的必需字段
func (sm *StructuredMessage) getRequiredFields() []string {
	requiredFieldsMap := map[MessageType][]string{
		MessageTypeUserRequest:      {"request", "user_address", "timestamp"},
		MessageTypeStrategy:         {"user_intent", "strategy", "recommendations"},
		MessageTypeRiskAssessment:   {"assessment", "approved", "mitigations"},
		MessageTypeTransaction:      {"transactions", "total_gas", "execution_order"},
		MessageTypeSignatureRequest: {"transaction", "message", "request_type"},
		MessageTypeExecutionResult:  {"results", "summary", "recommendations"},
		MessageTypeFinalReport:      {"audit_report", "state_verification", "overall_status"},
	}

	return requiredFieldsMap[sm.Type]
}

// ToJSON 转换为JSON
func (sm *StructuredMessage) ToJSON() ([]byte, error) {
	return json.MarshalIndent(sm, "", "  ")
}

// FromJSON 从JSON解析
func FromJSON(data []byte) (*StructuredMessage, error) {
	var msg StructuredMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, err
	}
	return &msg, nil
}

// MessagePool 消息池 - 实现发布订阅机制
type MessagePool struct {
	messages    []StructuredMessage
	subscribers map[MessageType][]chan StructuredMessage
	mu          sync.RWMutex
}

// NewMessagePool 创建消息池
func NewMessagePool() *MessagePool {
	return &MessagePool{
		messages:    make([]StructuredMessage, 0),
		subscribers: make(map[MessageType][]chan StructuredMessage),
	}
}

// Publish 发布消息
func (mp *MessagePool) Publish(msg *StructuredMessage) error {
	mp.mu.Lock()
	defer mp.mu.Unlock()

	// 验证消息
	if err := msg.Validate(); err != nil {
		return fmt.Errorf("invalid message: %w", err)
	}

	// 添加到消息历史
	mp.messages = append(mp.messages, *msg)

	// 通知订阅者
	if subscribers, exists := mp.subscribers[msg.Type]; exists {
		for _, ch := range subscribers {
			select {
			case ch <- *msg:
			default:
				// 通道满了，跳过
			}
		}
	}

	return nil
}

// Subscribe 订阅消息类型
func (mp *MessagePool) Subscribe(msgType MessageType, buffer int) <-chan StructuredMessage {
	mp.mu.Lock()
	defer mp.mu.Unlock()

	ch := make(chan StructuredMessage, buffer)
	mp.subscribers[msgType] = append(mp.subscribers[msgType], ch)

	return ch
}

// GetMessages 获取特定类型的历史消息
func (mp *MessagePool) GetMessages(types []MessageType) []StructuredMessage {
	mp.mu.RLock()
	defer mp.mu.RUnlock()

	var filtered []StructuredMessage
	for _, msg := range mp.messages {
		for _, t := range types {
			if msg.Type == t {
				filtered = append(filtered, msg)
				break
			}
		}
	}

	return filtered
}

// MessageTemplates 消息模板定义
type MessageTemplates struct{}

// UserRequestTemplate 用户请求模板
func (mt *MessageTemplates) UserRequestTemplate() map[string]interface{} {
	return map[string]interface{}{
		"request":      "",         // 用户原始请求
		"user_address": "",         // 用户钱包地址
		"timestamp":    time.Now(), // 请求时间
		"preferences": map[string]interface{}{
			"max_slippage": 0.05,     // 最大滑点
			"gas_priority": "medium", // gas优先级
			"deadline":     30,       // 交易截止时间（分钟）
		},
	}
}

// StrategyTemplate Web3策略模板
func (mt *MessageTemplates) StrategyTemplate() map[string]interface{} {
	return map[string]interface{}{
		"user_intent": map[string]interface{}{
			"action":     "",                       // swap/stake/unstake/approve
			"from_token": "",                       // 源代币
			"to_token":   "",                       // 目标代币
			"amount":     "",                       // 金额
			"additional": map[string]interface{}{}, // 额外参数
		},
		"strategy": map[string]interface{}{
			"route": []map[string]interface{}{
				{
					"protocol":         "", // Uniswap/Sushiswap/Curve等
					"pool":             "", // 流动性池地址
					"input_token":      "", // 输入代币
					"output_token":     "", // 输出代币
					"estimated_output": "", // 预估输出
				},
			},
			"estimated_gas":  0,
			"estimated_cost": "",
			"execution_time": 0,
		},
		"recommendations": []string{},
		"alternatives":    []map[string]interface{}{},
	}
}

// RiskAssessmentTemplate 风险评估模板
func (mt *MessageTemplates) RiskAssessmentTemplate() map[string]interface{} {
	return map[string]interface{}{
		"assessment": map[string]interface{}{
			"slippage_risk": map[string]interface{}{
				"score":       0.0, // 0-1 风险分数
				"max_impact":  "",  // 最大影响金额
				"probability": "",  // 概率
			},
			"liquidity_risk": map[string]interface{}{
				"score":      0.0,
				"pool_depth": "",
				"24h_volume": "",
			},
			"contract_risk": map[string]interface{}{
				"score":        0.0,
				"audit_status": "",
				"known_issues": []string{},
			},
			"gas_risk": map[string]interface{}{
				"score":             0.0,
				"current_gas_price": "",
				"estimated_cost":    "",
			},
			"overall_score": 0.0,
		},
		"approved": false,
		"mitigations": []map[string]interface{}{
			{
				"risk_type": "",
				"action":    "",
				"impact":    "",
			},
		},
	}
}

// TransactionTemplate 交易模板
func (mt *MessageTemplates) TransactionTemplate() map[string]interface{} {
	return map[string]interface{}{
		"transactions": []map[string]interface{}{
			{
				"type":      "", // approve/swap/stake等
				"to":        "", // 合约地址
				"from":      "", // 发送方地址
				"value":     "", // ETH数量
				"data":      "", // 交易数据
				"gas_limit": 0,  // Gas限制
				"gas_price": "", // Gas价格
				"nonce":     0,  // Nonce
				"chain_id":  0,  // 链ID
			},
		},
		"total_gas":       0,
		"execution_order": []int{},               // 执行顺序
		"dependencies":    map[string][]string{}, // 交易依赖关系
	}
}

// ExecutionResultTemplate 执行结果模板
func (mt *MessageTemplates) ExecutionResultTemplate() map[string]interface{} {
	return map[string]interface{}{
		"results": []map[string]interface{}{
			{
				"index":        0,
				"status":       "", // success/failed/pending
				"tx_hash":      "", // 交易哈希
				"gas_used":     0,  // 实际使用的Gas
				"block_number": 0,  // 区块号
				"timestamp":    time.Time{},
				"error":        "", // 错误信息（如果有）
			},
		},
		"summary": map[string]interface{}{
			"total_executed": 0,
			"successful":     0,
			"failed":         0,
			"total_gas_used": 0,
			"total_cost":     "",
		},
		"recommendations": []string{},
	}
}

// FinalReportTemplate 最终报告模板
func (mt *MessageTemplates) FinalReportTemplate() map[string]interface{} {
	return map[string]interface{}{
		"audit_report": map[string]interface{}{
			"execution_summary": map[string]interface{}{},
			"performance_metrics": map[string]interface{}{
				"total_time":      0,
				"gas_efficiency":  0.0,
				"cost_efficiency": 0.0,
			},
			"compliance": map[string]interface{}{
				"user_intent_matched":    false,
				"risk_limits_respected":  false,
				"slippage_within_bounds": false,
			},
		},
		"state_verification": map[string]interface{}{
			"expected_state": map[string]interface{}{},
			"actual_state":   map[string]interface{}{},
			"discrepancies":  []string{},
		},
		"recommendations": []string{},
		"overall_status":  "", // success/partial_success/failed
	}
}

// Web3MessageValidator 消息验证器
type Web3MessageValidator struct {
	schemas map[MessageType]interface{}
}

// NewWeb3MessageValidator 创建验证器
func NewWeb3MessageValidator() *Web3MessageValidator {
	return &Web3MessageValidator{
		schemas: make(map[MessageType]interface{}),
	}
}

// ValidateMessage 验证消息内容
func (v *Web3MessageValidator) ValidateMessage(msg *StructuredMessage) error {
	// 基础验证
	if err := msg.Validate(); err != nil {
		return err
	}

	// 类型特定验证
	switch msg.Type {
	case MessageTypeUserRequest:
		return v.validateUserRequest(msg)
	case MessageTypeStrategy:
		return v.validateStrategy(msg)
	case MessageTypeRiskAssessment:
		return v.validateRiskAssessment(msg)
	case MessageTypeTransaction:
		return v.validateTransaction(msg)
	case MessageTypeExecutionResult:
		return v.validateExecutionResult(msg)
	case MessageTypeFinalReport:
		return v.validateFinalReport(msg)
	default:
		return fmt.Errorf("unknown message type: %s", msg.Type)
	}
}

// validateUserRequest 验证用户请求
func (v *Web3MessageValidator) validateUserRequest(msg *StructuredMessage) error {
	userAddress, ok := msg.Content["user_address"].(string)
	if !ok || userAddress == "" {
		return fmt.Errorf("invalid user address")
	}

	// 验证以太坊地址格式
	if !isValidEthereumAddress(userAddress) {
		return fmt.Errorf("invalid Ethereum address format")
	}

	return nil
}

// validateStrategy 验证策略消息
func (v *Web3MessageValidator) validateStrategy(msg *StructuredMessage) error {
	strategy, ok := msg.Content["strategy"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("strategy field is required")
	}

	// 验证路由信息
	route, ok := strategy["route"].([]map[string]interface{})
	if !ok || len(route) == 0 {
		return fmt.Errorf("valid route is required in strategy")
	}

	return nil
}

// validateRiskAssessment 验证风险评估
func (v *Web3MessageValidator) validateRiskAssessment(msg *StructuredMessage) error {
	assessment, ok := msg.Content["assessment"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("assessment field is required")
	}

	// 验证风险分数
	overallScore, ok := assessment["overall_score"].(float64)
	if !ok || overallScore < 0 || overallScore > 1 {
		return fmt.Errorf("invalid overall risk score")
	}

	return nil
}

// validateTransaction 验证交易消息
func (v *Web3MessageValidator) validateTransaction(msg *StructuredMessage) error {
	transactions, ok := msg.Content["transactions"].([]map[string]interface{})
	if !ok || len(transactions) == 0 {
		return fmt.Errorf("transactions field is required and must not be empty")
	}

	// 验证每个交易
	for i, tx := range transactions {
		if _, ok := tx["to"].(string); !ok {
			return fmt.Errorf("transaction %d missing 'to' address", i)
		}
		if _, ok := tx["data"].(string); !ok {
			return fmt.Errorf("transaction %d missing 'data' field", i)
		}
	}

	return nil
}

// validateExecutionResult 验证执行结果
func (v *Web3MessageValidator) validateExecutionResult(msg *StructuredMessage) error {
	results, ok := msg.Content["results"].([]map[string]interface{})
	if !ok {
		return fmt.Errorf("results field is required")
	}

	// 验证每个结果
	for i, result := range results {
		status, ok := result["status"].(string)
		if !ok {
			return fmt.Errorf("result %d missing status", i)
		}

		if status == "success" {
			if _, ok := result["tx_hash"].(string); !ok {
				return fmt.Errorf("successful result %d missing tx_hash", i)
			}
		}
	}

	return nil
}

// validateFinalReport 验证最终报告
func (v *Web3MessageValidator) validateFinalReport(msg *StructuredMessage) error {
	overallStatus, ok := msg.Content["overall_status"].(string)
	if !ok || overallStatus == "" {
		return fmt.Errorf("overall_status is required")
	}

	validStatuses := []string{"success", "partial_success", "failed"}
	valid := false
	for _, status := range validStatuses {
		if overallStatus == status {
			valid = true
			break
		}
	}

	if !valid {
		return fmt.Errorf("invalid overall_status: %s", overallStatus)
	}

	return nil
}

// isValidEthereumAddress 验证以太坊地址
func isValidEthereumAddress(address string) bool {
	// 简单的格式验证
	if len(address) != 42 {
		return false
	}
	if address[:2] != "0x" {
		return false
	}
	// 这里应该添加更多验证逻辑
	return true
}

// MessageSerializer 消息序列化器
type MessageSerializer struct {
	compressThreshold int // 压缩阈值（字节）
}

// NewMessageSerializer 创建序列化器
func NewMessageSerializer(compressThreshold int) *MessageSerializer {
	return &MessageSerializer{
		compressThreshold: compressThreshold,
	}
}

// Serialize 序列化消息
func (ms *MessageSerializer) Serialize(msg *StructuredMessage) ([]byte, error) {
	data, err := json.Marshal(msg)
	if err != nil {
		return nil, err
	}

	// 如果消息太大，进行压缩
	if len(data) > ms.compressThreshold {
		return ms.compress(data)
	}

	return data, nil
}

// Deserialize 反序列化消息
func (ms *MessageSerializer) Deserialize(data []byte) (*StructuredMessage, error) {
	// 检查是否是压缩数据
	if ms.isCompressed(data) {
		var err error
		data, err = ms.decompress(data)
		if err != nil {
			return nil, err
		}
	}

	var msg StructuredMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, err
	}

	return &msg, nil
}

// compress 压缩数据
func (ms *MessageSerializer) compress(data []byte) ([]byte, error) {
	// 实现压缩逻辑（例如使用gzip）
	return data, nil
}

// decompress 解压数据
func (ms *MessageSerializer) decompress(data []byte) ([]byte, error) {
	// 实现解压逻辑
	return data, nil
}

// isCompressed 检查数据是否被压缩
func (ms *MessageSerializer) isCompressed(data []byte) bool {
	// 检查压缩标识
	return false
}
