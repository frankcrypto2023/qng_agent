package qng

import (
	"context"
	"fmt"
	"github.com/Qitmeer/qng/graph"
	"log"
	"qng_agent/internal/contracts"
	"qng_agent/internal/llm"
	"qng_agent/internal/rpc"
	"time"
)

// getString 从map中安全获取字符串值
func getString(m map[string]any, key string) string {
	if val, ok := m[key]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}

// LangGraph 节点系统
type LangGraph struct {
	nodes           map[string]Node
	llm             llm.Client
	contractManager *contracts.ContractManager
	rpcClient       *rpc.Client
	txConfig        TransactionConfig

	g *graph.Graph
	r *graph.Runnable
}

// Node 节点接口
type Node interface {
	Execute(ctx context.Context, input NodeInput) (*NodeOutput, error)
	GetName() string
	GetType() string
}

// NodeInput 节点输入
type NodeInput struct {
	Data    map[string]any `json:"data"`
	Context map[string]any `json:"context"`
}

// NodeOutput 节点输出
type NodeOutput struct {
	Data         map[string]any `json:"data"`
	NextNodes    []string       `json:"next_nodes"`
	NeedUserAuth bool           `json:"need_user_auth"`
	AuthRequest  any            `json:"auth_request,omitempty"`
	Completed    bool           `json:"completed"`
}

// NewLangGraph 创建LangGraph实例
func NewLangGraph(llmClient llm.Client, contractManager *contracts.ContractManager, rpcClient *rpc.Client, txConfig TransactionConfig) *LangGraph {
	lg := &LangGraph{
		nodes:           make(map[string]Node),
		llm:             llmClient,
		contractManager: contractManager,
		rpcClient:       rpcClient,
		txConfig:        txConfig,
	}
	lg.g = graph.NewGraph()
	// 注册节点
	lg.registerNodes()

	// 构建图结构
	lg.buildGraph()

	return lg
}

// registerNodes 注册所有节点
func (lg *LangGraph) registerNodes() {
	nodes := []Node{
		NewTaskDecomposerNode(lg.llm, lg.contractManager),    // 任务分解节点
		NewSwapExecutorNode(lg.contractManager),              // 交易执行节点
		NewStakeExecutorNode(lg.contractManager),             // 质押执行节点
		NewSignatureValidatorNode(lg.rpcClient, lg.txConfig), // 签名验证节点
		NewResultAggregatorNode(),                            // 结果聚合节点
	}

	for _, node := range nodes {
		lg.g.AddNode(node.GetName(), func(ctx context.Context, name string, state graph.State) (graph.State, error) {
			log.Printf("🔄 执行节点: %s (类型: %s)", node.GetName(), node.GetType())
			input := state["input"].(*NodeInput)
			// 执行节点
			output, err := node.Execute(ctx, *input)
			if err != nil {
				log.Printf("❌ 节点执行失败: %v", err)
				return nil, fmt.Errorf("node %s execution failed: %w", node.GetName(), err)
			}
			log.Printf("✅ 节点执行成功")
			log.Printf("📊 输出数据: %+v", output.Data)

			state["output"] = output
			state[node.GetName()] = node

			return state, nil
		})
	}
}

// buildGraph 构建图结构
func (lg *LangGraph) buildGraph() {
	edgeFunc := func(ctx context.Context, name string, state graph.State) string {
		input := state["input"].(*NodeInput)
		output := state["output"].(*NodeOutput)

		if name == "task_decomposer" {
			n := state["task_decomposer"].(*TaskDecomposerNode)
			output.NextNodes = n.determineNextNodes(output.Data["tasks"].([]map[string]any))
		} else if name == "signature_validator" {
			n := state["signature_validator"].(*SignatureValidatorNode)
			output.NextNodes = n.checkDependentTasks(input.Data, output.Data["transaction_hash"].(string))
		}

		log.Printf("➡️  下一个节点: %v", output.NextNodes)
		log.Printf("🔐 需要用户授权: %v", output.NeedUserAuth)
		log.Printf("✅ 是否完成: %v", output.Completed)

		// 检查是否需要用户授权
		if output.NeedUserAuth {
			log.Printf("✍️  需要用户签名授权")
			log.Printf("📋 授权请求: %+v", output.AuthRequest)

			// 将AuthRequest转换为SignatureRequest
			var signatureRequest *SignatureRequest
			if authReq, ok := output.AuthRequest.(*SignatureRequest); ok {
				signatureRequest = authReq
			} else {
				log.Printf("⚠️  AuthRequest类型不正确，尝试转换")
				// 如果类型不正确，尝试从map转换
				if authMap, ok := output.AuthRequest.(map[string]any); ok {
					signatureRequest = &SignatureRequest{
						Action:    getString(authMap, "action"),
						FromToken: getString(authMap, "from_token"),
						ToToken:   getString(authMap, "to_token"),
						Amount:    getString(authMap, "amount"),
						ToAddress: getString(authMap, "to_address"),
						Value:     getString(authMap, "value"),
						Data:      getString(authMap, "data"),
						GasLimit:  getString(authMap, "gas_limit"),
						GasPrice:  getString(authMap, "gas_price"),
						GasFee:    getString(authMap, "gas_fee"),
						Slippage:  getString(authMap, "slippage"),
					}
				}
			}

			state["result"] = &ProcessResult{
				NeedSignature:    true,
				SignatureRequest: signatureRequest,
				WorkflowContext: map[string]any{
					"current_node": name,
					"node_output":  output,
					"input":        *input,
				},
			}
			return graph.END
		}

		// 检查是否已完成
		if output.Completed {
			log.Printf("✅ 工作流执行完成")
			state["result"] = &ProcessResult{
				FinalResult: output.Data,
			}
			return graph.END
		}

		// 继续执行下一个节点
		if len(output.NextNodes) > 0 {
			nextNode := output.NextNodes[0] // 简化处理，取第一个
			log.Printf("➡️  继续执行下一个节点: %s", nextNode)

			nextInput := &NodeInput{
				Data:    output.Data,
				Context: input.Context,
			}
			state["input"] = nextInput
			return nextNode
		}

		// 没有下一个节点，工作流完成
		log.Printf("✅ 没有下一个节点，工作流完成")
		state["result"] = &ProcessResult{
			FinalResult: output.Data,
		}
		return graph.END
	}

	lg.g.AddConditionalEdge("task_decomposer", edgeFunc)
	lg.g.AddConditionalEdge("swap_executor", edgeFunc)
	lg.g.AddConditionalEdge("stake_executor", edgeFunc)
	lg.g.AddConditionalEdge("signature_validator", edgeFunc)
	lg.g.AddEdge("result_aggregator", graph.END)

	lg.g.SetEntryPoint("task_decomposer")

	r, err := lg.g.Compile()
	if err != nil {
		log.Printf("❌ compile error: %v", err)
	}
	lg.r = r
}

// ExecuteWorkflow 执行工作流
func (lg *LangGraph) ExecuteWorkflow(ctx context.Context, message string) (*ProcessResult, error) {
	log.Printf("🔄 LangGraph开始执行工作流")
	log.Printf("📝 用户消息: %s", message)

	// 初始化输入
	input := &NodeInput{
		Data: map[string]any{
			"user_message": message,
			"timestamp":    time.Now(),
		},
		Context: map[string]any{
			"workflow_id": ctx.Value("workflow_id"),
			"session_id":  ctx.Value("session_id"),
		},
	}

	// 确保工作流ID被正确传递到数据中
	if workflowID := ctx.Value("workflow_id"); workflowID != nil {
		input.Data["workflow_id"] = workflowID
	}
	if sessionID := ctx.Value("session_id"); sessionID != nil {
		input.Data["session_id"] = sessionID
	}

	state, err := lg.r.Invoke(ctx, map[string]interface{}{"input": input})
	if err != nil {
		return nil, err
	}

	// 安全地处理结果
	if result, exists := state["result"]; exists && result != nil {
		if processResult, ok := result.(*ProcessResult); ok {
			return processResult, nil
		}
	}

	// 如果没有结果或结果类型不正确，返回默认的ProcessResult
	log.Printf("⚠️  状态中没有有效的ProcessResult，返回默认结果")
	return &ProcessResult{
		FinalResult: map[string]any{
			"message": "工作流执行完成",
			"status":  "completed",
		},
	}, nil
}

// ContinueWithSignature 使用签名继续工作流
func (lg *LangGraph) ContinueWithSignature(ctx context.Context, workflowContext any, signature string) (*ProcessResult, error) {
	log.Printf("🔄 使用签名继续工作流")
	log.Printf("🔐 签名长度: %d", len(signature))

	// 从工作流上下文恢复执行状态
	contextMap, ok := workflowContext.(map[string]any)
	if !ok {
		log.Printf("❌ 无效的工作流上下文类型")
		return nil, fmt.Errorf("invalid workflow context")
	}

	currentNode, ok := contextMap["current_node"].(string)
	if !ok {
		log.Printf("❌ 上下文中缺少当前节点信息")
		return nil, fmt.Errorf("invalid current node in context")
	}

	log.Printf("🔄 从节点恢复: %s", currentNode)

	nodeOutput, ok := contextMap["node_output"].(*NodeOutput)
	if !ok {
		log.Printf("❌ 上下文中缺少节点输出信息")
		return nil, fmt.Errorf("invalid node output in context")
	}

	nodeInput, ok := contextMap["input"].(NodeInput)
	if !ok {
		log.Printf("❌ 上下文中缺少节点输入信息")
		return nil, fmt.Errorf("invalid node input in context")
	}

	// 将签名添加到数据中
	log.Printf("🔐 将签名添加到节点数据中")
	nodeOutput.Data["signature"] = signature
	nodeOutput.NeedUserAuth = false

	// 继续执行下一个节点
	if len(nodeOutput.NextNodes) > 0 {
		nextNode := nodeOutput.NextNodes[0]
		log.Printf("➡️  继续执行下一个节点: %s", nextNode)

		nextInput := &NodeInput{
			Data:    nodeOutput.Data,
			Context: nodeInput.Context,
		}
		lg.g.SetEntryPoint(nextNode)
		state, err := lg.r.Invoke(ctx, map[string]interface{}{"input": nextInput})
		if err != nil {
			log.Printf("❌ 继续执行失败: %v", err)
			return nil, err
		}
		log.Printf("✅ 继续执行成功")

		// 安全地处理结果
		if result, exists := state["result"]; exists && result != nil {
			if processResult, ok := result.(*ProcessResult); ok {
				return processResult, nil
			}
		}

		// 如果没有结果或结果类型不正确，返回基于当前数据的ProcessResult
		log.Printf("⚠️  状态中没有有效的ProcessResult，返回基于当前数据的结果")
		return &ProcessResult{
			FinalResult: nodeOutput.Data,
		}, nil
	}

	log.Printf("✅ 没有下一个节点，返回当前数据")
	return &ProcessResult{
		FinalResult: nodeOutput.Data,
	}, nil
}
