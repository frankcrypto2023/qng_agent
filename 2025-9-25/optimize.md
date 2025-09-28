QNG 智能体前端工作流实时可视化需求文档
1. 概述
为了提升用户体验，直观地展示 QNG 智能体在处理复杂任务时的内部执行逻辑，本项目旨在开发一个前端工作流实时可视化功能。当用户输入一个需要多步骤执行的任务时，系统将在网页右侧以流程图的形式，动态展示任务的执行图谱、每个节点的当前状态以及最终结果，实现对智能体工作状态的实时追踪。

2. 功能需求
2.1 前端界面 (Frontend)
布局与视图:

在网页主界面的右侧开辟一个专门的工作流显示区域，该区域宽度可调整。

此区域固定展现，内容自动跟着滑动在页面底部；只显示最后一个对话的工作流。

工作流图谱渲染:

初始化: 当后端返回工作流的完整结构后，前端需要根据节点（nodes）和边（edges）信息，绘制一个自上而下的有向无环图 (DAG)。

节点 (Node): 每个节点应以带圆角的矩形框表示，框内清晰展示节点的标题或摘要信息（如：“意图分析”）。

连接 (Edge): 节点之间使用箭头连接，明确指示工作流的执行方向。

节点状态实时更新:

前端需要实时响应 WebSocket 推送的节点状态更新，并通过改变节点的视觉样式来反馈。定义以下几种核心状态：

待执行 (Pending): 默认状态，如灰色背景。

执行中 (Executing): 节点正在处理，如蓝色背景并附带一个动态加载图标。

执行成功 (Success): 节点处理完成且成功，如绿色背景，并在一侧显示一个 "✔" (check) 图标。

执行失败 (Failure): 节点处理失败，如红色背景，并在一侧显示一个 "✖" (cross) 图标。点击失败节点可查看简要错误信息。

2.2 后端服务 (Backend)
HTTP API:

需要一个 API 端点（例如 POST /api/workflow/initiate）接收用户的查询请求。

接收到请求后，后端首先解析并构建出完整的执行工作流图谱结构，然后将该结构（包含所有节点和连接关系）通过 HTTP 响应立即返回给前端。

WebSocket 服务:

提供一个 WebSocket 服务端点（例如 /ws/workflow/status），用于与前端建立长连接。

在工作流的执行过程中，每当一个节点的状态发生变化（如：开始执行、执行成功、执行失败），后端都应通过 WebSocket 向前端推送一条更新消息。

所有节点执行完毕后，推送一条工作流结束的最终消息。

3. 数据结构定义 (Data Structures)
3.1 初始工作流图谱结构 (HTTP Response)
后端通过 HTTP API 返回给前端的整体图谱结构。

{
  "workflowId": "wf-1688c2f2-e175-4f33-913a-912f2778b17b",
  "graph": {
    "nodes": [
      {
        "id": "node-1",
        "label": "意图分析",
        "type": "IntentAnalysis",
        "status": "Pending" 
      },
      {
        "id": "node-2",
        "label": "查询最新区块数量",
        "type": "APICall",
        "status": "Pending"
      },
      {
        "id": "node-3",
        "label": "提取区块号参数",
        "type": "LLMParameterExtraction",
        "status": "Pending"
      },
      {
        "id": "node-4",
        "label": "查询 StateRoot 信息",
        "type": "APICall",
        "status": "Pending"
      },
      {
        "id": "node-5",
        "label": "格式化最终结果",
        "type": "LLMBeautify",
        "status": "Pending"
      }
    ],
    "edges": [
      { "source": "node-1", "target": "node-2" },
      { "source": "node-2", "target": "node-3" },
      { "source": "node-3", "target": "node-4" },
      { "source": "node-4", "target": "node-5" }
    ]
  }
}

3.2 节点状态更新消息 (WebSocket Message)
后端通过 WebSocket 推送给前端的实时状态更新消息。

{
  "eventType": "NODE_STATUS_UPDATE",
  "payload": {
    "workflowId": "wf-1688c2f2-e175-4f33-913a-912f2778b17b",
    "nodeId": "node-2",
    "status": "Success", 
    "data": {
      "result": "Block number is 1000"
    },
    "error": null
  }
}

eventType: 消息类型，例如 NODE_STATUS_UPDATE, WORKFLOW_COMPLETE。

status: 节点更新后的状态，值为 Executing, Success, Failure 之一。

data: 节点成功执行后返回的数据（可选）。

error: 节点执行失败时的错误信息（可选）。

4. 示例工作流 (Example Workflow)
用户输入: '查询 http://120.79.90.24:18545 最新区块数量，然后减1 例如区块数量为1000, 取出999对应的 stateroot 信息'

[前端] 发送用户输入到 POST /api/workflow/initiate。

[后端] 接收请求，生成图谱结构。

[前端] 接收到 3.1 中定义的 JSON 结构，并在右侧区域渲染出包含5个灰色“待执行”节点的流程图。

[后端] 开始执行 node-1。

[后端] node-1 执行状态变为 Executing，通过 WebSocket 推送消息。

{ "eventType": "NODE_STATUS_UPDATE", "payload": { "nodeId": "node-1", "status": "Executing" }}

[前端] 接收消息，将 node-1 的背景变为蓝色并显示加载图标。

[后端] node-1 执行成功，通过 WebSocket 推送消息。

{ "eventType": "NODE_STATUS_UPDATE", "payload": { "nodeId": "node-1", "status": "Success" }}

[前端] 接收消息，将 node-1 的背景变为绿色并显示 "✔"。

... 后续节点 node-2 至 node-5 重复步骤 4-8。

[后端] 所有节点执行完毕，推送工作流完成消息。

[前端] 接收完成消息，可以高亮整个流程图或显示最终结果。

5. 做好接口测试以及页面测试