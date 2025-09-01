package qngai

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/cloudwego/eino/compose"
)

// WorkflowConfig 定义工作流的JSON配置结构
type WorkflowConfig struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Nodes       []NodeConfig           `json:"nodes"`
	Edges       []EdgeConfig           `json:"edges"`
	Metadata    map[string]interface{} `json:"metadata"`
}

// NodeConfig 定义节点配置
type NodeConfig struct {
	ID         string                 `json:"id"`
	Type       string                 `json:"type"` // 节点类型：ai_model, web3_contract, data_processor等
	Name       string                 `json:"name"`
	Config     map[string]interface{} `json:"config"`      // 节点特定配置
	Web3Config *Web3NodeConfig        `json:"web3_config"` // Web3相关配置
}

// Web3NodeConfig Web3节点特定配置
type Web3NodeConfig struct {
	ContractAddress string   `json:"contract_address"`
	ChainID         int      `json:"chain_id"`
	ABI             string   `json:"abi"`
	Method          string   `json:"method"`
	Parameters      []string `json:"parameters"`
}

// EdgeConfig 定义边（节点间关系）配置
type EdgeConfig struct {
	ID         string                 `json:"id"`
	Source     string                 `json:"source"`      // 源节点ID
	Target     string                 `json:"target"`      // 目标节点ID
	Condition  string                 `json:"condition"`   // 可选：条件表达式
	DataMapper map[string]interface{} `json:"data_mapper"` // 数据映射规则
}

// WorkflowEngine 工作流引擎
type WorkflowEngine struct {
	config *WorkflowConfig
	graph  *compose.Graph[map[string]interface{}, map[string]interface{}]
}

// NewWorkflowEngine 创建工作流引擎实例
func NewWorkflowEngine(configPath string) (*WorkflowEngine, error) {
	// 读取JSON配置文件
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config WorkflowConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	engine := &WorkflowEngine{
		config: &config,
	}

	// 构建工作流
	if err := engine.buildWorkflow(); err != nil {
		return nil, fmt.Errorf("failed to build workflow: %w", err)
	}

	return engine, nil
}

// buildWorkflow 根据配置构建Eino工作流
func (e *WorkflowEngine) buildWorkflow() error {
	// 创建Graph实例
	graph := compose.NewGraph[map[string]interface{}, map[string]interface{}]()

	// 创建所有节点
	for _, nodeConfig := range e.config.Nodes {
		node, err := e.createNode(nodeConfig)
		if err != nil {
			return fmt.Errorf("failed to create node %s: %w", nodeConfig.ID, err)
		}

		// 添加节点到图中
		if err := graph.AddLambdaNode(nodeConfig.ID, node); err != nil {
			return fmt.Errorf("failed to add node %s: %w", nodeConfig.ID, err)
		}
	}

	// 添加边（节点间的连接关系）
	for _, edge := range e.config.Edges {
		// 基础连接
		if err := graph.AddEdge(edge.Source, edge.Target); err != nil {
			return fmt.Errorf("failed to add edge %s->%s: %w", edge.Source, edge.Target, err)
		}
	}

	// 设置起始和结束节点
	// 找到起始节点（没有入边的节点）
	startNodes := findStartNodes(e.config.Nodes, e.config.Edges)
	if len(startNodes) == 0 {
		// 如果没有找到起始节点，使用第一个节点
		if len(e.config.Nodes) > 0 {
			startNodes = []string{e.config.Nodes[0].ID}
		}
	}

	// 找到结束节点（没有出边的节点）
	endNodes := findEndNodes(e.config.Nodes, e.config.Edges)
	if len(endNodes) == 0 {
		// 如果没有找到结束节点，使用最后一个节点
		if len(e.config.Nodes) > 0 {
			endNodes = []string{e.config.Nodes[len(e.config.Nodes)-1].ID}
		}
	}

	// 添加起始边
	for _, startNode := range startNodes {
		if err := graph.AddEdge(compose.START, startNode); err != nil {
			return fmt.Errorf("failed to add start edge to %s: %w", startNode, err)
		}
	}

	// 添加结束边
	for _, endNode := range endNodes {
		if err := graph.AddEdge(endNode, compose.END); err != nil {
			return fmt.Errorf("failed to add end edge from %s: %w", endNode, err)
		}
	}

	// 保存编译后的图
	e.graph = graph
	return nil
}

// createNode 根据配置创建节点
func (e *WorkflowEngine) createNode(config NodeConfig) (*compose.Lambda, error) {
	switch config.Type {
	case "ai_model":
		log.Printf("createAIModelNode: %v", config)
		return e.createAIModelNode(config)
	case "web3_contract":
		log.Printf("createWeb3Node: %v", config)
		return e.createWeb3Node(config)
	case "data_processor":
		log.Printf("createDataProcessorNode: %v", config)
		return e.createDataProcessorNode(config)
	case "conditional":
		log.Printf("createConditionalNode: %v", config)
		return e.createConditionalNode(config)
	default:
		return nil, fmt.Errorf("unknown node type: %s", config.Type)
	}
}

// createAIModelNode 创建AI模型节点
func (e *WorkflowEngine) createAIModelNode(config NodeConfig) (*compose.Lambda, error) {
	// 这里根据配置创建相应的AI模型节点
	// 可以集成OpenAI、Claude等模型
	return compose.InvokableLambda(func(ctx context.Context, input map[string]interface{}) (output map[string]interface{}, err error) {
		// AI模型调用逻辑
		// 例如：调用LLM进行文本生成、分析等

		// 处理逻辑...
		result := processWithAI(input, config.Config)

		// 返回结果
		return result, nil
	}), nil
}

// createWeb3Node 创建Web3交互节点
func (e *WorkflowEngine) createWeb3Node(config NodeConfig) (*compose.Lambda, error) {
	if config.Web3Config == nil {
		return nil, fmt.Errorf("web3 config is required for web3 node")
	}

	return compose.InvokableLambda(func(ctx context.Context, input map[string]interface{}) (output map[string]interface{}, err error) {
		// Web3交互逻辑
		// 例如：调用智能合约、读取链上数据、发送交易等

		// 与智能合约交互
		result := interactWithContract(
			config.Web3Config.ContractAddress,
			config.Web3Config.Method,
			config.Web3Config.Parameters,
			input,
		)

		return result, nil
	}), nil
}

// createDataProcessorNode 创建数据处理节点
func (e *WorkflowEngine) createDataProcessorNode(config NodeConfig) (*compose.Lambda, error) {
	return compose.InvokableLambda(func(ctx context.Context, input map[string]interface{}) (output map[string]interface{}, err error) {
		// 数据处理逻辑
		// 例如：数据转换、聚合、过滤等

		// 处理数据
		result := processData(input, config.Config)

		return result, nil
	}), nil
}

// createConditionalNode 创建条件节点
func (e *WorkflowEngine) createConditionalNode(config NodeConfig) (*compose.Lambda, error) {
	return compose.InvokableLambda(func(ctx context.Context, input map[string]interface{}) (output map[string]interface{}, err error) {
		// 条件判断逻辑

		// 根据条件决定下一步
		condition := config.Config["condition"].(string)
		if evaluateSimpleCondition(condition, input) {
			input["next_node"] = config.Config["true_branch"]
		} else {
			input["next_node"] = config.Config["false_branch"]
		}

		return input, nil
	}), nil
}

// Execute 执行工作流
func (e *WorkflowEngine) Execute(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
	if e.graph == nil {
		return nil, fmt.Errorf("workflow not initialized")
	}

	// 编译图并执行
	runnable, err := e.graph.Compile(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to compile graph: %w", err)
	}

	// 执行工作流
	output, err := runnable.Invoke(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("workflow execution failed: %w", err)
	}

	return output, nil
}

// 辅助函数
func evaluateCondition(condition string, state map[string]interface{}) bool {
	// 实现条件表达式的评估逻辑
	// 可以使用表达式引擎如govaluate等
	return true
}

func evaluateSimpleCondition(condition string, data map[string]interface{}) bool {
	// 简单条件评估
	return true
}

func processWithAI(data map[string]interface{}, config map[string]interface{}) map[string]interface{} {
	// AI处理逻辑实现
	return map[string]interface{}{
		"processed": true,
		"result":    "AI processed data",
	}
}

func interactWithContract(address, method string, params []string, data map[string]interface{}) map[string]interface{} {
	// Web3合约交互逻辑实现
	log.Printf("interactWithContract: %v", data)
	return map[string]interface{}{
		"tx_hash": "0x123...",
		"status":  "success",
	}
}

func processData(data map[string]interface{}, config map[string]interface{}) map[string]interface{} {
	// 数据处理逻辑实现
	return data
}

// findStartNodes 找到起始节点（没有入边的节点）
func findStartNodes(nodes []NodeConfig, edges []EdgeConfig) []string {
	nodeIDs := make(map[string]bool)
	for _, node := range nodes {
		nodeIDs[node.ID] = true
	}

	// 移除所有有入边的节点
	for _, edge := range edges {
		delete(nodeIDs, edge.Target)
	}

	// 返回剩余的节点（没有入边的节点）
	var startNodes []string
	for nodeID := range nodeIDs {
		startNodes = append(startNodes, nodeID)
	}
	return startNodes
}

// findEndNodes 找到结束节点（没有出边的节点）
func findEndNodes(nodes []NodeConfig, edges []EdgeConfig) []string {
	nodeIDs := make(map[string]bool)
	for _, node := range nodes {
		nodeIDs[node.ID] = true
	}

	// 移除所有有出边的节点
	for _, edge := range edges {
		delete(nodeIDs, edge.Source)
	}

	// 返回剩余的节点（没有出边的节点）
	var endNodes []string
	for nodeID := range nodeIDs {
		endNodes = append(endNodes, nodeID)
	}
	return endNodes
}
