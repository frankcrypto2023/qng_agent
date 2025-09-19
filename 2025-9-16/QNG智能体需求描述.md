# QNG 智能体前端框架实现

按照以下需求点,实现QNG智能体前端框架

## 一. 具体需求以及场景描述
- 用一个QNG智能体页面对话框
- 用户可以新建聊天或者打开一个历史聊天会话
- 会话在后台存储上下文
- 用户在某个会话提出问题,调用接口返回内容,接口使用stream流式一个字一个字跳动显示
- 用户可以设置自定义的 MCP Server，按照标准的MCP SSE 协议
- 用户可以设置选择自定义模型Provider 具体包含url,token,modelName
- 用户提问之后接口的处理逻辑如下:
    - 1. 后端先New一个LLM Graph出来，通过https://github.com/Qitmeer/qng/blob/dev/2.1/graph/graph.go 这个github库里的包来创建,这是一个工作流图框架,你可以先学习下代码以及用法
    - 2. 这个LLM Graph 主要作用是理解用户问题的意图分析，请帮我设计一套工作流来丰富这个Graph 里面应该包含的节点，针对这个工作流我提以下基础的要求: 1)可以识别用户session的上下文历史信息 2)可以分析出是否需要调用用户设置的MCP tools 
    - 3. 根据LLM Graph 识别的结果 去做对应的事情 比如调用tools 或者需要用户确认等,针对这里 我提一点要求，就是有一类是识别工作流，也就是在web3 某个合约 配置了某一种工作流，这边识别后可以自动加载工作流配置执行对应的工作流，工作流配置如下:
```
{
    "name": "QNG-AI-Web3-Workflow",
    "description": "An QNG-AI-powered Web3 workflow for processing and analyzing blockchain data",
    "nodes": [
        {
            "id": "input_node",
            "type": "data_processor",
            "name": "Input Processor",
            "config": {
                "validation": true
            }
        },
        {
            "id": "ai_analyzer",
            "type": "ai_model",
            "name": "AI Data Analyzer",
            "config": {
                "model": "gpt-4",
                "temperature": 0.7,
                "max_tokens": 1000
            }
        },
        {
            "id": "web3_reader",
            "type": "web3_contract",
            "name": "Smart Contract Reader",
            "web3_config": {
                "contract_address": "0x742d35Cc6634C0532925a3b844Bc9e7595f0bEb",
                "chain_id": 1,
                "method": "getData",
                "parameters": ["param1", "param2"]
            }
        },
        {
            "id": "decision_node",
            "type": "conditional",
            "name": "Decision Maker",
            "config": {
                "condition": "score > 0.8",
                "true_branch": "web3_writer",
                "false_branch": "output_node"
            }
        },
        {
            "id": "web3_writer",
            "type": "web3_contract",
            "name": "Smart Contract Writer",
            "web3_config": {
                "contract_address": "0x742d35Cc6634C0532925a3b844Bc9e7595f0bEb",
                "chain_id": 1,
                "method": "setData",
                "parameters": ["result"]
            }
        },
        {
            "id": "output_node",
            "type": "data_processor",
            "name": "Output Formatter",
            "config": {
                "format": "json"
            }
        }
    ],
    "edges": [
        {
            "id": "edge1",
            "source": "input_node",
            "target": "web3_reader"
        },
        {
            "id": "edge2",
            "source": "web3_reader",
            "target": "ai_analyzer"
        },
        {
            "id": "edge3",
            "source": "ai_analyzer",
            "target": "decision_node"
        },
        {
            "id": "edge4",
            "source": "decision_node",
            "target": "web3_writer",
            "condition": "score > 0.8"
        },
        {
            "id": "edge5",
            "source": "decision_node",
            "target": "output_node",
            "condition": "score <= 0.8"
        },
        {
            "id": "edge6",
            "source": "web3_writer",
            "target": "output_node"
        }
    ]
}
```

- 自动识别工作流这里可以先预留 写一个固定的 比如配置一个 AI analyzing blockchain data 然后自动加载以上的 工作流配置
- 如果 没有匹配上 mcp tools 定义好的工作流配置等，则根据用户意图看是否需要自动生成一个根据用户意图的工作流，也是使用上面讲的 qng graph 来去创建子图
- 所有工作流执行结束后返回给用户
- 上诉会话管理可以使用gpt-oss 对应框架 harmony ，就是使用channel 的模式来区分不同会话，组装会话模板来调用LLM 
- 具体调用的LLM server是用户设置的 LLM provider信息
- 比如如下场景:
    1. 用户输入: 'http://127.0.0.1:8545节点下对应order=10000的stateroot信息'，这时候接口会创建一个LLM的GRAPH，根据已有的意图分析工作流,发现有MCP tools对应上 比如是stateroot 然后提取 http://127.0.0.1:8545 调用tool 获取结果，将结果再调用LLM使用自然语言 返回给用户
    2. 用户输入: 'http://127.0.0.1:8545节点与http://127.0.0.2:8545节点下对应order=10000的stateroot信息 是否一致'，这时候接口会创建一个LLM的GRAPH，根据已有的意图分析工作流,发现有MCP tools对应上 比如是stateroot 然后提取 http://127.0.0.1:8545 调用tool 获取结果，再提取http://127.0.0.2:8545 调用tool获取结果 将2个结果再调用LLM 看是否一致，不一致有哪些不一致，使用自然语言 返回给用户
    3. 用户输入: '我需要将Memamask中的1MEER兑换成USDT',这时候接口会创建一个LLM的GRAPH，根据已有的意图分析工作流,发现有子图可以做这件事，那么就load 子图的config json 形式工作流，比如子图优先需要用户连接钱包操作，需要返回用户点击授权，获取到用户地址后，子图查询余额，如果余额不足提示用户，子图结束，如果余额充足再组装对应的交易再返回给用户，需要用户签名，用户签好名之后再发送交易，返回交易id 再交易LLM 返回自然语言给到用户

## 二.页面设计要求
- 标题 QNG智能体
- 页面简洁大方
- 注入区块链元素
- 类似CHATGPT的对话框页面
- 左侧有创建新聊天按钮
- 左侧下方有会话记录
- 选择其中某个会话，将历史会话记录加载出来
- 左侧下方有设置按钮,可以设置MCP Servers信息,MCP 使用标准的SSE,MCP协议,并且设置模型源 包含模型URL,token,具体模型名称。

## 三.前端技术要求
- 使用react框架开发,代码注释全一点,便于后续多次迭代
- 前端接口模块定义明确以及需要参考后端接口文档保持字段一致
- 注释使用英文
- 所有接口都走自己的接口实现，不要直接用第三方接口，比如调用LLM

## 四.后端技术要求
- 使用golang框架开发接口
- 注释使用英文
- 做好接口文档