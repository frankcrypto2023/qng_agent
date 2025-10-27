# Qng Agent 代码优化

## 优化要求

- 1. 请学习当前代码框架，当前代码是一个QNG 区块链网络的智能体
- 2. 主要优化是主流程以及子流程的系统提示词，主要是代码文件[manager.go](../internal/graph/manager.go)
    - 当前系统提示词比较臃肿，结合[模板](../qng_agent_template)优化
    - 当前mcp tool不智能，将mcp tool转成标准function tool的协议来进行对话
    - 增加对话历史，messages组装出以下标准结构,来与大模型对话
    - 结合现有系统提示词进行优化，保留提示词的目的，不影响现有功能的前提下，将tool做成mcp tool，精简优化提示词
```
[
      {
        "role": "system",
        "content": "You are a tools-calling assistant. You express numerical values with at most two decimals."
      },
      {
        "role": "user",
        "content": "What'\''s the y coordinate of a point on the unit sphere at angle 30 degrees?"
      },
      {
        "role": "assistant",
        "content": null,
        "tool_calls": [
          {
            "id": "call_6789",
            "type": "function",
            "function": {
              "name": "calculate",
              "arguments": "{\"expression\":\"sin(30 * pi / 180)\"}"
            }
          }
        ]
      },
      {
        "role": "tool",
        "name": "calculate",
        "content": "0.55644242476",
        "tool_call_id": "call_6789"
      }
    ],
    "tools": [
      {
        "type": "function",
        "function": {
          "name": "calculate",
          "description": "A calculator function that computes values of arithmetic expressions in the Python syntax",
          "parameters": {
            "type": "object",
            "properties": {
              "expression": {
                "type": "string",
                "description": "An arithmetic expression to compute the value of (Python syntad, assuming all floats)"
              }
            },
            "required": ["expression"]
          }
        }
      }
    ]
```