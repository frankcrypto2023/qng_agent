// internal/roles/base_role.go
package roles

import (
	"context"
	"fmt"
	"time"
	"qng_agent/internal/protocol"
)

// RoleType 定义角色类型
type RoleType string

const (
	RoleStrategyAnalyst    RoleType = "StrategyAnalyst"
	RoleRiskManager        RoleType = "RiskManager"
	RoleTransactionBuilder RoleType = "TransactionBuilder"
	RoleExecutor           RoleType = "Executor"
	RoleAuditor            RoleType = "Auditor"
)

// RoleProfile 角色配置文件
type RoleProfile struct {
	Name        string
	Type        RoleType
	Description string
	Goal        string
	Constraints []string
	Skills      []string
}

// BaseRole 基础角色接口 - 借鉴 MetaGPT 的设计
type BaseRole interface {
	GetProfile() RoleProfile
	Act(ctx context.Context, input *protocol.StructuredMessage) (*protocol.StructuredMessage, error)
	Subscribe(topics []protocol.MessageType)
	Publish(message *protocol.StructuredMessage) error
}

// AbstractRole 抽象角色实现
type AbstractRole struct {
	Profile       RoleProfile
	MessagePool   *protocol.MessagePool
	Memory        []protocol.StructuredMessage
	Subscriptions []protocol.MessageType
}

// GetProfile 获取角色配置
func (r *AbstractRole) GetProfile() RoleProfile {
	return r.Profile
}

// Subscribe 订阅消息类型
func (r *AbstractRole) Subscribe(topics []protocol.MessageType) {
	r.Subscriptions = append(r.Subscriptions, topics...)
}

// Publish 发布消息
func (r *AbstractRole) Publish(message *protocol.StructuredMessage) error {
	if r.MessagePool == nil {
		return fmt.Errorf("message pool not initialized")
	}

	message.SenderRole = string(r.Profile.Type)
	message.Timestamp = time.Now()

	return r.MessagePool.Publish(message)
}

// GetRelevantMessages 获取相关消息
func (r *AbstractRole) GetRelevantMessages() []protocol.StructuredMessage {
	if r.MessagePool == nil {
		return []protocol.StructuredMessage{}
	}

	return r.MessagePool.GetMessages(r.Subscriptions)
}
