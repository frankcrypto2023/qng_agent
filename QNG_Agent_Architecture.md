# QNG智能体系统架构图

## 📋 系统概览

QNG智能体是一个基于LLM驱动的区块链操作系统，支持多RPC节点查询、智能工作流编排和动态参数解析。
![image](./2025-9-16/workflow.png)
```mermaid
flowchart TD
    %% 用户界面层
    User[👤 用户] --> Frontend[🌐 React前端界面]
    Frontend --> |HTTP POST /api/chat| WebServer[🖥️ Web服务器]
    
    %% 核心处理层
    WebServer --> SessionManager[📝 会话管理器]
    SessionManager --> GraphManager[🧠 LLM图管理器]
    
    %% 主要处理流程
    GraphManager --> |1. 意图分析| IntentAnalysis[🔍 意图分析节点]
    IntentAnalysis --> |动态提示词| LLMClient[🤖 LLM客户端]
    LLMClient --> |JSON响应| IntentResult{意图结果}
    
    %% 路由决策
    IntentResult --> |单工具调用| MCPExecution[⚡ MCP工具执行]
    IntentResult --> |复杂工作流| SubWorkflowExecution[🔄 子工作流执行]
    IntentResult --> |Web3操作| Web3Execution[🌐 Web3工作流执行]
    IntentResult --> |常规对话| ResponseGeneration[💬 响应生成]
    
    %% MCP工具执行分支
    MCPExecution --> MCPClient[🔌 MCP客户端]
    MCPClient --> |SSE连接| MCPServer[📡 MCP服务器]
    MCPServer --> |JSON-RPC| QNGNode1[🟢 QNG节点1]
    MCPServer --> |JSON-RPC| QNGNode2[🟢 QNG节点2]
    
    %% 子工作流执行分支
    SubWorkflowExecution --> TaskScheduler[📅 任务调度器]
    TaskScheduler --> |并行/顺序| TaskExecution[⚙️ 任务执行器]
    TaskExecution --> |依赖检测| DependencyResolver[🔗 依赖解析器]
    DependencyResolver --> |LLM参数提取| ParameterExtractor[📊 参数提取器]
    ParameterExtractor --> MCPClient
    
    %% 最终响应生成
    MCPExecution --> ResponseGeneration
    SubWorkflowExecution --> ResponseGeneration  
    Web3Execution --> ResponseGeneration
    ResponseGeneration --> |语言一致性| LLMClient
    LLMClient --> |Markdown格式| FinalResponse[📝 最终响应]
    
    %% 响应返回
    FinalResponse --> |SSE流式传输| WebServer
    WebServer --> |实时显示| Frontend
    Frontend --> |自动刷新| User
    
    %% 存储层
    SessionManager -.-> LevelDB[(💾 LevelDB存储)]
    
    %% 配置层
    GraphManager -.-> ConfigManager[⚙️ 配置管理]
    ConfigManager -.-> MCPConfig[📋 MCP服务器配置]
    ConfigManager -.-> LLMConfig[🤖 LLM配置]
    
    %% 样式定义
    classDef userLayer fill:#e1f5fe
    classDef frontendLayer fill:#f3e5f5
    classDef coreLayer fill:#e8f5e8
    classDef executionLayer fill:#fff3e0
    classDef dataLayer fill:#fce4ec
    classDef externalLayer fill:#f1f8e9
    
    class User,Frontend userLayer
    class WebServer,SessionManager frontendLayer
    class GraphManager,IntentAnalysis,LLMClient coreLayer
    class MCPExecution,SubWorkflowExecution,Web3Execution,TaskScheduler,TaskExecution executionLayer
    class LevelDB,ConfigManager dataLayer
    class QNGNode1,QNGNode2,MCPServer externalLayer
```

## 🔄 详细消息处理流程

### 1. 用户输入阶段
```mermaid
sequenceDiagram
    participant U as 用户
    participant F as 前端
    participant W as Web服务器
    participant S as 会话管理器
    
    U->>F: 输入消息
    F->>W: POST /api/chat
    W->>S: 创建/获取会话
    S->>W: 返回会话ID
    Note over W: 准备流式响应
```

### 2. 意图分析阶段
```mermaid
sequenceDiagram
    participant G as 图管理器
    participant I as 意图分析节点
    participant L as LLM客户端
    
    G->>I: 用户消息
    I->>I: 构建动态提示词
    Note over I: 包含可用MCP工具<br/>和工作流信息
    I->>L: 分析请求
    L->>I: JSON结果
    Note over I: {intent, confidence,<br/>tool_name, sub_workflow}
```

### 3. 工作流路由决策
```mermaid
flowchart TD
    IntentResult{意图分析结果}
    IntentResult --> |intent: mcp_tool| SingleTool[单工具执行]
    IntentResult --> |intent: sub_workflow| MultiTool[子工作流执行]
    IntentResult --> |intent: web3_workflow| Web3Flow[Web3工作流]
    IntentResult --> |intent: general| DirectResponse[直接响应]
    
    SingleTool --> MCPCall[调用MCP工具]
    MultiTool --> TaskGeneration[生成任务序列]
    TaskGeneration --> DependencyCheck{检查任务依赖}
    DependencyCheck --> |有依赖| Sequential[顺序执行]
    DependencyCheck --> |无依赖| Parallel[并行执行]
```

### 4. 子工作流执行详细流程
```mermaid
sequenceDiagram
    participant S as 子工作流执行器
    participant T as 任务调度器
    participant D as 依赖解析器
    participant P as 参数提取器
    participant M as MCP客户端
    participant Q as QNG节点
    
    S->>T: 解析工作流任务
    T->>T: 识别任务依赖关系
    
    loop 对每个任务
        T->>D: 检查依赖
        alt 有依赖任务
            D->>P: 提取前置任务结果
            P->>P: LLM分析参数
            Note over P: 从JSON结果中提取<br/>block_count → order参数
            P->>T: 返回解析参数
        end
        T->>M: 执行任务调用
        M->>Q: JSON-RPC请求
        Q->>M: 返回结果
        M->>T: 存储任务结果
    end
    
    T->>S: 聚合所有结果
```

### 5. MCP客户端并发处理
```mermaid
sequenceDiagram
    participant T1 as 任务1
    participant T2 as 任务2
    participant M as MCP客户端
    participant S1 as SSE会话1
    participant S2 as SSE会话2
    participant Q1 as QNG节点1
    participant Q2 as QNG节点2
    
    par 并发执行
        T1->>M: 调用get_block_count
        M->>S1: 创建会话A
        S1->>Q1: JSON-RPC请求
    and
        T2->>M: 调用get_block_count  
        M->>S2: 创建会话B
        S2->>Q2: JSON-RPC请求
    end
    
    par 并发响应
        Q1->>S1: 返回结果A
        S1->>M: 通过sessionID=A
        M->>T1: 结果A
    and
        Q2->>S2: 返回结果B
        S2->>M: 通过sessionID=B
        M->>T2: 结果B
    end
```

## 🏗️ 核心组件架构

### 1. LLM图管理器 (LLMGraphManager)
```
职责：
- 消息图的创建和编排
- 节点间的路由决策
- 上下文状态管理

关键方法：
- ProcessUserMessage() - 主入口
- intentAnalysisNode() - 意图分析
- routeAfterIntent() - 路由决策
```

### 2. MCP客户端 (MCP Client)
```
职责：
- 与MCP服务器的SSE通信
- 会话管理和连接池
- 并发请求处理

关键特性：
- sessionID-based连接管理
- 线程安全的并发访问
- 自动重连和错误处理
```

### 3. 子工作流执行器 (SubWorkflow Executor)
```
职责：
- 复杂任务的编排和执行
- 任务间依赖关系处理
- 结果聚合和分析

关键功能：
- 动态工作流生成
- LLM驱动的参数提取
- 并行/顺序执行模式
```

## 🔍 系统提示词架构

### 1. 意图分析提示词
```
功能：动态分析用户意图，生成结构化工作流
输入：用户消息 + 可用工具列表
输出：JSON格式的意图分析结果

关键特性：
- 自适应工具识别
- 依赖关系分析
- 工作流任务生成
```

### 2. 参数提取提示词
```
功能：从前置任务结果中提取后续任务所需参数
输入：前置任务JSON结果 + 当前任务定义
输出：解析后的参数映射

示例：
block_count: 13078095 → order: 13078095
```

### 3. 响应生成提示词
```
功能：生成用户友好的最终响应
特性：
- 语言一致性检测
- Markdown格式化
- 数据可视化表格
- 技术术语本地化
```

## 📊 数据流转架构

```mermaid
flowchart LR
    Input[用户输入] --> Analysis[意图分析]
    Analysis --> |单工具| DirectExec[直接执行]
    Analysis --> |多工具| WorkflowGen[工作流生成]
    
    WorkflowGen --> TaskSeq[任务序列]
    TaskSeq --> Dependency[依赖分析]
    Dependency --> Execution[并发执行]
    
    DirectExec --> Results[工具结果]
    Execution --> Results
    Results --> Aggregation[结果聚合]
    Aggregation --> Format[格式化响应]
    Format --> Stream[流式输出]
    Stream --> Display[用户显示]
```

## 🚀 关键技术特性

1. **动态工作流生成** - LLM分析用户意图，自动生成执行计划
2. **智能参数传递** - 自动解析任务间的数据依赖关系  
3. **并发执行优化** - sessionID隔离的并发MCP调用
4. **语言一致性** - 自动匹配用户语言进行响应
5. **流式用户体验** - 实时显示执行进度和结果
6. **容错恢复机制** - 自动重试和降级处理
7. **可扩展架构** - 插件化的工具和工作流系统

---

*此架构图基于实际代码分析生成，反映了QNG智能体的完整技术栈和数据流转过程。*