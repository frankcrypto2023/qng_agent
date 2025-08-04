# QNG Agent - SOP vs LangGraph 工作流引擎对比

## 概述

QNG Agent 支持两种不同的工作流引擎来处理区块链操作：

1. **SOP (Standard Operating Procedure)** - 基于角色的结构化工作流
2. **LangGraph** - 基于图的动态工作流

## 架构对比

### SOP 架构

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   Role Manager  │    │  Workflow Def   │    │  State Manager  │
│   (角色管理器)    │    │  (工作流定义)    │    │  (状态管理器)    │
└─────────────────┘    └─────────────────┘    └─────────────────┘
         │                       │                       │
         ▼                       ▼                       ▼
┌─────────────────────────────────────────────────────────────────┐
│              SOP Workflow Engine                               │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐          │
│  │   Step 1    │  │   Step 2    │  │   Step 3    │          │
│  │ (Analyze)   │  │ (Risk Mgmt) │  │ (Execute)   │          │
│  └─────────────┘  └─────────────┘  └─────────────┘          │
└─────────────────────────────────────────────────────────────────┘
```

### LangGraph 架构

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   LLM Client    │    │ Contract Mgr    │    │   RPC Client    │
│   (LLM客户端)    │    │ (合约管理器)     │    │  (RPC客户端)     │
└─────────────────┘    └─────────────────┘    └─────────────────┘
         │                       │                       │
         ▼                       ▼                       ▼
┌─────────────────────────────────────────────────────────────────┐
│                    LangGraph Engine                            │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐          │
│  │   Node 1    │◄─┤   Node 2    │◄─┤   Node 3    │          │
│  │(Decomposer) │  │(Executor)   │  │(Validator)  │          │
│  └─────────────┘  └─────────────┘  └─────────────┘          │
└─────────────────────────────────────────────────────────────────┘
```

## 详细对比

### 1. 设计理念

| 特性 | SOP | LangGraph |
|------|-----|-----------|
| **设计理念** | 基于角色的结构化流程 | 基于图的动态流程 |
| **核心概念** | 标准操作程序 (Standard Operating Procedure) | 语言图 (Language Graph) |
| **灵感来源** | MetaGPT 的 SOP 概念 | LangChain 的图执行模型 |
| **流程控制** | 预定义的步骤序列 | 动态节点执行 |

### 2. 工作流定义

#### SOP 工作流定义
```go
// 预定义的工作流步骤
workflow := &WorkflowDefinition{
    Type: WorkflowCompound,
    Steps: []WorkflowStep{
        {
            ID:     "analyze",
            Name:   "Analyze Request",
            Role:   roles.RoleStrategyAnalyst,
            Input:  []string{},
            Output: string(protocol.MessageTypeStrategy),
        },
        {
            ID:     "assess_risk",
            Name:   "Assess Risk",
            Role:   roles.RoleRiskManager,
            Input:  []string{"analyze"},
            Output: string(protocol.MessageTypeRiskAssessment),
        },
        // ... 更多步骤
    },
}
```

#### LangGraph 工作流定义
```go
// 动态节点注册
nodes := []Node{
    NewTaskDecomposerNode(llm, contractManager),
    NewSwapExecutorNode(contractManager),
    NewStakeExecutorNode(contractManager),
    NewSignatureValidatorNode(rpcClient, txConfig),
    NewResultAggregatorNode(),
}

// 图结构构建
lg.buildGraph()
```

### 3. 执行流程对比

#### SOP 执行流程
```
1. 用户请求 → 2. 工作流类型识别 → 3. 创建执行上下文
4. 按步骤执行 → 5. 角色分配 → 6. 消息传递
7. 签名请求 → 8. 用户签名 → 9. 继续下一步
10. 完成工作流
```

#### LangGraph 执行流程
```
1. 用户请求 → 2. 任务分解节点 → 3. 动态路由
4. 执行节点 → 5. 状态更新 → 6. 下一个节点
7. 签名验证 → 8. 结果聚合 → 9. 完成
```

### 4. 复合操作处理

#### SOP 复合操作
```go
// 复合操作处理器
func (ch *CompoundHandler) ProcessCompoundOperation(ctx context.Context, userRequest string) (*WorkflowResult, error) {
    // 1. 分析复合请求
    compoundOp, err := ch.analyzeCompoundRequest(userRequest)
    
    // 2. 创建执行上下文
    execCtx, err := ch.createCompoundExecutionContext(compoundOp, userRequest)
    
    // 3. 生成第一个签名请求
    if compoundOp.CurrentStep < len(compoundOp.Operations) {
        firstOp := &compoundOp.Operations[compoundOp.CurrentStep]
        signatureRequest := ch.generateSignatureRequestForOperation(firstOp, compoundOp)
        // ...
    }
}
```

#### LangGraph 复合操作
```go
// 动态节点执行
func (lg *LangGraph) ExecuteWorkflow(ctx context.Context, message string) (*ProcessResult, error) {
    // 1. 创建初始状态
    initialState := graph.State{
        "input": &NodeInput{
            Data: map[string]any{"message": message},
        },
    }
    
    // 2. 执行图
    result, err := lg.r.Invoke(ctx, initialState)
    
    // 3. 处理结果
    return lg.processResult(result)
}
```

### 5. 状态管理

#### SOP 状态管理
```go
type WorkflowExecutionContext struct {
    ID          string
    WorkflowDef *WorkflowDefinition
    StartTime   time.Time
    State       map[string]*StepState
    Messages    map[string]*protocol.StructuredMessage
}

type StepState struct {
    StepID    string
    Status    string
    StartTime time.Time
    EndTime   time.Time
    Error     string
    Output    *protocol.StructuredMessage
}
```

#### LangGraph 状态管理
```go
type NodeInput struct {
    Data    map[string]any `json:"data"`
    Context map[string]any `json:"context"`
}

type NodeOutput struct {
    Data         map[string]any `json:"data"`
    NextNodes    []string       `json:"next_nodes"`
    NeedUserAuth bool           `json:"need_user_auth"`
    AuthRequest  any            `json:"auth_request,omitempty"`
    Completed    bool           `json:"completed"`
}
```

## 优缺点对比

### SOP 优势

✅ **结构化强**
- 预定义的工作流步骤
- 明确的角色分工
- 可预测的执行路径

✅ **易于理解和维护**
- 清晰的步骤定义
- 直观的角色分配
- 简单的状态管理

✅ **适合标准化操作**
- 复合操作处理
- 分步签名流程
- 错误处理和回滚

✅ **性能稳定**
- 无 LLM 依赖
- 快速执行
- 资源消耗低

### SOP 劣势

❌ **灵活性有限**
- 固定的工作流结构
- 难以动态调整
- 扩展性受限

❌ **智能程度较低**
- 缺乏 LLM 推理
- 无法处理复杂逻辑
- 依赖预定义规则

### LangGraph 优势

✅ **高度灵活**
- 动态节点执行
- 可配置的图结构
- 支持复杂逻辑

✅ **智能推理**
- LLM 驱动的决策
- 自然语言理解
- 上下文感知

✅ **可扩展性强**
- 易于添加新节点
- 支持自定义逻辑
- 模块化设计

✅ **适应性强**
- 处理复杂场景
- 动态路由
- 智能错误处理

### LangGraph 劣势

❌ **复杂性高**
- 学习曲线陡峭
- 调试困难
- 状态管理复杂

❌ **性能开销**
- LLM 调用延迟
- 资源消耗较高
- 执行时间较长

❌ **稳定性挑战**
- LLM 响应不稳定
- 错误处理复杂
- 依赖外部服务

## 使用场景建议

### 选择 SOP 的场景

🟢 **标准化操作**
- 代币兑换 (swap)
- 代币质押 (stake)
- 复合操作 (compound)

🟢 **性能要求高**
- 高频交易
- 实时响应
- 资源受限环境

🟢 **可预测流程**
- 固定的业务逻辑
- 明确的步骤序列
- 简单的错误处理

### 选择 LangGraph 的场景

🟢 **复杂业务逻辑**
- 多步骤决策
- 动态路由
- 智能分析

🟢 **自然语言交互**
- 用户意图理解
- 上下文感知
- 智能推荐

🟢 **创新性功能**
- 实验性功能
- 快速原型
- 灵活扩展

## 实际应用示例

### SOP 复合操作示例
```
用户请求: "我要将1MEER兑换成MTK，再将对应的MTK质押"

SOP 处理流程:
1. 分析请求 → 识别为复合操作
2. 分解操作 → [swap, stake]
3. 生成第一个签名请求 → swap 操作
4. 用户签名 → 验证签名
5. 生成第二个签名请求 → stake 操作
6. 用户签名 → 验证签名
7. 完成复合操作
```

### LangGraph 智能分析示例
```
用户请求: "我想投资一些代币，但担心风险"

LangGraph 处理流程:
1. 任务分解节点 → 分析用户意图
2. 风险评估节点 → 评估投资风险
3. 策略推荐节点 → 生成投资建议
4. 执行计划节点 → 制定执行计划
5. 签名验证节点 → 验证用户授权
6. 结果聚合节点 → 生成最终报告
```

## 总结

SOP 和 LangGraph 各有其适用场景：

- **SOP** 适合标准化、高性能、可预测的场景
- **LangGraph** 适合复杂、智能、灵活的场景

在实际应用中，可以根据具体需求选择合适的引擎，或者结合两者的优势来构建混合解决方案。 