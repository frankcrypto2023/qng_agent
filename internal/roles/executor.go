// internal/roles/executor.go
package roles

import (
	"context"
	"qng_agent/internal/protocol"
)

// Executor 交易执行角色
type Executor struct {
	AbstractRole
}

// NewExecutor 创建执行器
func NewExecutor() *Executor {
	return &Executor{
		AbstractRole: AbstractRole{
			Profile: RoleProfile{
				Type:        RoleExecutor,
				Name:        "Executor",
				Description: "Executes blockchain transactions",
				Goal:        "Execute transactions safely and efficiently",
				Constraints: []string{"Transaction validation", "Gas management"},
				Skills:      []string{"Transaction execution", "Blockchain interaction", "Error handling"},
			},
		},
	}
}

// Act 执行交易
func (e *Executor) Act(ctx context.Context, input *protocol.StructuredMessage) (*protocol.StructuredMessage, error) {
	// 模拟交易执行处理
	output := protocol.NewStructuredMessage(
		protocol.MessageTypeExecutionResult,
		map[string]interface{}{
			"results": []map[string]interface{}{
				{
					"status":       "success",
					"tx_hash":      "0xabcdef1234567890",
					"gas_used":     21000,
					"block_number": 18500000,
				},
			},
			"summary": map[string]interface{}{
				"total_executed": 1,
				"successful":     1,
				"failed":         0,
			},
		},
	)
	output.SenderRole = string(e.Profile.Type)
	return output, nil
}