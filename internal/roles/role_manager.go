// internal/roles/role_manager.go
package roles

import (
	"fmt"
	"sync"
)

// RoleManager 角色管理器接口
type RoleManager interface {
	GetRole(roleType RoleType) (BaseRole, error)
	RegisterRole(role BaseRole) error
	ListRoles() []BaseRole
	GetRoleProfile(roleType RoleType) (RoleProfile, error)
	InitializeDefaultRoles() error
}

// RoleManagerImpl 角色管理器实现
type RoleManagerImpl struct {
	roles map[RoleType]BaseRole
	mu    sync.RWMutex
}

// NewRoleManager 创建角色管理器
func NewRoleManager() *RoleManagerImpl {
	return &RoleManagerImpl{
		roles: make(map[RoleType]BaseRole),
	}
}

// RegisterRole 注册角色
func (rm *RoleManagerImpl) RegisterRole(role BaseRole) error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	profile := role.GetProfile()
	rm.roles[profile.Type] = role

	return nil
}

// GetRole 获取角色
func (rm *RoleManagerImpl) GetRole(roleType RoleType) (BaseRole, error) {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	role, exists := rm.roles[roleType]
	if !exists {
		return nil, fmt.Errorf("role %s not found", roleType)
	}

	return role, nil
}

// ListRoles 列出所有角色
func (rm *RoleManagerImpl) ListRoles() []BaseRole {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	var roleList []BaseRole
	for _, role := range rm.roles {
		roleList = append(roleList, role)
	}

	return roleList
}

// GetRoleProfile 获取角色配置
func (rm *RoleManagerImpl) GetRoleProfile(roleType RoleType) (RoleProfile, error) {
	role, err := rm.GetRole(roleType)
	if err != nil {
		return RoleProfile{}, err
	}

	return role.GetProfile(), nil
}

// InitializeDefaultRoles 初始化默认角色
func (rm *RoleManagerImpl) InitializeDefaultRoles() error {
	// 创建策略分析师
	strategyAnalyst := NewStrategyAnalyst()
	if err := rm.RegisterRole(strategyAnalyst); err != nil {
		return fmt.Errorf("failed to register strategy analyst: %w", err)
	}

	// 创建风险管理员
	riskManager := NewRiskManager()
	if err := rm.RegisterRole(riskManager); err != nil {
		return fmt.Errorf("failed to register risk manager: %w", err)
	}

	// 创建交易构建器
	transactionBuilder := NewTransactionBuilder()
	if err := rm.RegisterRole(transactionBuilder); err != nil {
		return fmt.Errorf("failed to register transaction builder: %w", err)
	}

	// 创建执行器
	executor := NewExecutor()
	if err := rm.RegisterRole(executor); err != nil {
		return fmt.Errorf("failed to register executor: %w", err)
	}

	// 创建审计员
	auditor := NewAuditor()
	if err := rm.RegisterRole(auditor); err != nil {
		return fmt.Errorf("failed to register auditor: %w", err)
	}

	return nil
}
