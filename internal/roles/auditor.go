// internal/roles/auditor.go
package roles

import (
	"context"
	"qng_agent/internal/protocol"
)

// Auditor 审计角色
type Auditor struct {
	AbstractRole
}

// NewAuditor 创建审计员
func NewAuditor() *Auditor {
	return &Auditor{
		AbstractRole: AbstractRole{
			Profile: RoleProfile{
				Type:        RoleAuditor,
				Name:        "Auditor",
				Description: "Audits execution results and ensures compliance",
				Goal:        "Verify transaction execution and compliance",
				Constraints: []string{"Thorough verification", "Compliance checking"},
				Skills:      []string{"Transaction auditing", "Compliance verification", "Report generation"},
			},
		},
	}
}

// Act 执行审计
func (a *Auditor) Act(ctx context.Context, input *protocol.StructuredMessage) (*protocol.StructuredMessage, error) {
	// 模拟审计处理
	output := protocol.NewStructuredMessage(
		protocol.MessageTypeFinalReport,
		map[string]interface{}{
			"audit_report": map[string]interface{}{
				"execution_summary": map[string]interface{}{
					"total_steps":      5,
					"successful_steps": 5,
					"failed_steps":     0,
				},
				"compliance": map[string]interface{}{
					"user_intent_matched":    true,
					"risk_limits_respected":  true,
					"slippage_within_bounds": true,
				},
			},
			"overall_status": "success",
		},
	)
	output.SenderRole = string(a.Profile.Type)
	return output, nil
}