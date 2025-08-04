// internal/sop/state_manager.go
package sop

import (
	"fmt"
	"sync"
	"time"
)

// StateManager 状态管理器
type StateManager struct {
	states       map[string]interface{}
	executionLog []ExecutionLogEntry
	mu           sync.RWMutex
}

// ExecutionLogEntry 执行日志条目
type ExecutionLogEntry struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"`
	Status    string    `json:"status"`
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
	Data      map[string]interface{} `json:"data,omitempty"`
}

// NewStateManager 创建状态管理器
func NewStateManager() *StateManager {
	return &StateManager{
		states:       make(map[string]interface{}),
		executionLog: make([]ExecutionLogEntry, 0),
	}
}

// SetState 设置状态
func (sm *StateManager) SetState(key string, value interface{}) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	
	sm.states[key] = value
	
	// 记录状态变更日志
	entry := ExecutionLogEntry{
		ID:        fmt.Sprintf("state_%d", time.Now().UnixNano()),
		Type:      "state_change",
		Status:    "completed",
		Message:   fmt.Sprintf("State '%s' updated", key),
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"key":   key,
			"value": value,
		},
	}
	
	sm.executionLog = append(sm.executionLog, entry)
}

// GetState 获取状态
func (sm *StateManager) GetState(key string) (interface{}, bool) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	
	value, exists := sm.states[key]
	return value, exists
}

// DeleteState 删除状态
func (sm *StateManager) DeleteState(key string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	
	delete(sm.states, key)
	
	// 记录删除日志
	entry := ExecutionLogEntry{
		ID:        fmt.Sprintf("state_%d", time.Now().UnixNano()),
		Type:      "state_delete",
		Status:    "completed",
		Message:   fmt.Sprintf("State '%s' deleted", key),
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"key": key,
		},
	}
	
	sm.executionLog = append(sm.executionLog, entry)
}

// ListStates 列出所有状态
func (sm *StateManager) ListStates() map[string]interface{} {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	
	// 创建状态副本
	states := make(map[string]interface{})
	for k, v := range sm.states {
		states[k] = v
	}
	
	return states
}

// LogExecution 记录执行日志
func (sm *StateManager) LogExecution(logType, status, message string, data map[string]interface{}) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	
	entry := ExecutionLogEntry{
		ID:        fmt.Sprintf("exec_%d", time.Now().UnixNano()),
		Type:      logType,
		Status:    status,
		Message:   message,
		Timestamp: time.Now(),
		Data:      data,
	}
	
	sm.executionLog = append(sm.executionLog, entry)
	
	// 限制日志大小
	if len(sm.executionLog) > 1000 {
		sm.executionLog = sm.executionLog[100:]
	}
}

// GetExecutionLog 获取执行日志
func (sm *StateManager) GetExecutionLog(limit int) []ExecutionLogEntry {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	
	if limit <= 0 || limit > len(sm.executionLog) {
		limit = len(sm.executionLog)
	}
	
	// 返回最近的日志
	start := len(sm.executionLog) - limit
	if start < 0 {
		start = 0
	}
	
	log := make([]ExecutionLogEntry, limit)
	copy(log, sm.executionLog[start:])
	
	return log
}

// ClearLog 清空日志
func (sm *StateManager) ClearLog() {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	
	sm.executionLog = make([]ExecutionLogEntry, 0)
}

// GetStats 获取统计信息
func (sm *StateManager) GetStats() map[string]interface{} {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	
	stats := map[string]interface{}{
		"total_states": len(sm.states),
		"total_logs":   len(sm.executionLog),
	}
	
	// 统计日志类型
	logTypes := make(map[string]int)
	statusCounts := make(map[string]int)
	
	for _, entry := range sm.executionLog {
		logTypes[entry.Type]++
		statusCounts[entry.Status]++
	}
	
	stats["log_types"] = logTypes
	stats["status_counts"] = statusCounts
	
	return stats
}