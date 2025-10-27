# 生成一份QNG Agent的特定模板

## 生成要求

- 1. 基于gpt-oss-20b模型对应的harmony标签模板语言生成
- 2. QNG Agent是一个基于QNG 区块链网络的AI 智能体
- 3. 模板需要支持function call
- 4. QNG Agent 包含了QNG MCP Tool，特定模板可以识别是mcp tool执行报错还是正常执行
- 5. 创建template模板文件
- 6. 这个模板文件可以支持llama.cpp --chat-template 指定加载配置
- 7. 模板包含以下初始描述
```
You are a helpful QNG blockchain agent assistant
```