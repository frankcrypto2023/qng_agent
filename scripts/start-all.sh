#!/bin/bash

echo "🚀 启动QNG Agent完整服务栈..."

# 启动Chain服务（合约管理）
echo "📋 启动Chain服务..."
go run cmd/chain/main.go &
CHAIN_PID=$!
echo "✅ Chain服务已启动 (PID: $CHAIN_PID)"

# 等待Chain服务启动
sleep 3

# 启动MCP服务
echo "🔧 启动MCP服务..."
go run cmd/mcp/main.go &
MCP_PID=$!
echo "✅ MCP服务已启动 (PID: $MCP_PID)"

# 等待MCP服务启动
sleep 3

# 启动Agent服务
echo "🤖 启动Agent服务..."
go run cmd/agent/main.go &
AGENT_PID=$!
echo "✅ Agent服务已启动 (PID: $AGENT_PID)"

echo ""
echo "🎉 所有服务已启动！"
echo "📊 服务状态："
echo "  - Chain服务 (合约管理): http://localhost:9092"
echo "  - MCP服务 (模型上下文): http://localhost:9091"
echo "  - Agent服务 (智能代理): http://localhost:9090"
echo ""
echo "🔗 访问地址："
echo "  - Agent Web界面: http://localhost:9090"
echo "  - Chain API: http://localhost:9092/api/chain"
echo "  - MCP API: http://localhost:9091/api/mcp"
echo ""
echo "按 Ctrl+C 停止所有服务"

# 等待中断信号
trap 'echo "正在停止服务..."; kill $CHAIN_PID $MCP_PID $AGENT_PID; exit' INT

# 保持脚本运行
wait 