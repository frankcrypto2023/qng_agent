// internal/roles/transaction_builder.go
package roles

import (
	"context"
	"qng_agent/internal/protocol"
)

// TransactionBuilder 交易构建角色
type TransactionBuilder struct {
	AbstractRole
}

// NewTransactionBuilder 创建交易构建器
func NewTransactionBuilder() *TransactionBuilder {
	return &TransactionBuilder{
		AbstractRole: AbstractRole{
			Profile: RoleProfile{
				Type:        RoleTransactionBuilder,
				Name:        "Transaction Builder",
				Description: "Constructs and optimizes blockchain transactions",
				Goal:        "Build efficient and secure transactions",
				Constraints: []string{"Gas optimization", "Security validation"},
				Skills:      []string{"Transaction construction", "Gas estimation", "Smart contracts"},
			},
		},
	}
}

// Act 执行交易构建
func (tb *TransactionBuilder) Act(ctx context.Context, input *protocol.StructuredMessage) (*protocol.StructuredMessage, error) {
	// 模拟交易构建处理
	output := protocol.NewStructuredMessage(
		protocol.MessageTypeTransaction,
		map[string]interface{}{
			"transactions": []map[string]interface{}{
				{
					"type":      "mock_transaction",
					"to":        "0x1234567890abcdef",
					"value":     "1000000000000000000",
					"gas_limit": 21000,
				},
			},
			"total_gas":       21000,
			"execution_order": []int{0},
		},
	)
	output.SenderRole = string(tb.Profile.Type)
	return output, nil
}