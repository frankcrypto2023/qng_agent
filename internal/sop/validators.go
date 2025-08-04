// internal/sop/validators.go
package sop

import (
	"context"
	"fmt"
	"strconv"
	"time"
	
	"qng_agent/internal/protocol"
)

// 验证器实现
func validateStakeProtocol(ctx context.Context, msg *protocol.StructuredMessage) error {
	protocol, ok := msg.Content["protocol"].(string)
	if !ok || protocol == "" {
		return fmt.Errorf("invalid or missing protocol")
	}
	
	// 验证支持的质押协议
	supportedProtocols := []string{"compound", "aave", "lido", "rocketpool"}
	for _, supported := range supportedProtocols {
		if protocol == supported {
			return nil
		}
	}
	
	return fmt.Errorf("unsupported protocol: %s", protocol)
}

func validateStakeAmount(ctx context.Context, msg *protocol.StructuredMessage) error {
	amountStr, ok := msg.Content["amount"].(string)
	if !ok || amountStr == "" {
		return fmt.Errorf("invalid or missing amount")
	}
	
	amount, err := strconv.ParseFloat(amountStr, 64)
	if err != nil {
		return fmt.Errorf("invalid amount format: %v", err)
	}
	
	if amount <= 0 {
		return fmt.Errorf("amount must be positive")
	}
	
	// 检查最小质押金额
	if amount < 0.01 {
		return fmt.Errorf("amount below minimum threshold")
	}
	
	return nil
}

func validateProtocolSafety(ctx context.Context, msg *protocol.StructuredMessage) error {
	safetyData, ok := msg.Content["safety_analysis"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("missing safety analysis")
	}
	
	score, ok := safetyData["safety_score"].(float64)
	if !ok {
		return fmt.Errorf("missing safety score")
	}
	
	// 安全分数必须高于阈值
	if score < 0.7 {
		return fmt.Errorf("protocol safety score too low: %f", score)
	}
	
	return nil
}

func validateAPY(ctx context.Context, msg *protocol.StructuredMessage) error {
	apyStr, ok := msg.Content["apy"].(string)
	if !ok || apyStr == "" {
		return fmt.Errorf("missing APY information")
	}
	
	apy, err := strconv.ParseFloat(apyStr, 64)
	if err != nil {
		return fmt.Errorf("invalid APY format: %v", err)
	}
	
	// APY应该在合理范围内
	if apy < 0 || apy > 50 {
		return fmt.Errorf("suspicious APY value: %f", apy)
	}
	
	return nil
}

func validateComplexOperation(ctx context.Context, msg *protocol.StructuredMessage) error {
	// 调试：打印消息内容
	fmt.Printf("DEBUG: Message content: %+v\n", msg.Content)
	
	operations, ok := msg.Content["operations"].([]interface{})
	if !ok || len(operations) == 0 {
		// 尝试从其他位置查找operations
		if strategy, ok := msg.Content["strategy"].(map[string]interface{}); ok {
			if ops, ok := strategy["operations"].([]interface{}); ok && len(ops) > 0 {
				operations = ops
			}
		}
		
		if len(operations) == 0 {
			return fmt.Errorf("no operations defined for complex operation")
		}
	}
	
	// 验证每个子操作
	for i, op := range operations {
		opMap, ok := op.(map[string]interface{})
		if !ok {
			return fmt.Errorf("invalid operation format at index %d", i)
		}
		
		opType, ok := opMap["type"].(string)
		if !ok || opType == "" {
			return fmt.Errorf("missing operation type at index %d", i)
		}
		
		// 验证操作类型
		validTypes := []string{"swap", "stake", "unstake", "approve"}
		valid := false
		for _, validType := range validTypes {
			if opType == validType {
				valid = true
				break
			}
		}
		
		if !valid {
			return fmt.Errorf("invalid operation type '%s' at index %d", opType, i)
		}
	}
	
	return nil
}

// 后置动作实现
func notifyExecution(ctx context.Context, msg *protocol.StructuredMessage) error {
	// 发送执行通知
	fmt.Printf("Execution notification: %s\n", msg.ID)
	return nil
}

func updateState(ctx context.Context, msg *protocol.StructuredMessage) error {
	// 更新系统状态
	fmt.Printf("State updated for message: %s\n", msg.ID)
	return nil
}

// 生成执行ID
func generateExecutionID() string {
	return fmt.Sprintf("exec_%d", time.Now().UnixNano())
}