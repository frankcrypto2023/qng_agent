#!/bin/bash

# QNG Agent 网络连接诊断脚本

echo "=== QNG Agent 网络连接诊断 ==="
echo

# 检查基本网络连接
echo "1. 检查基本网络连接..."
if ping -c 3 8.8.8.8 > /dev/null 2>&1; then
    echo "✅ 基本网络连接正常"
else
    echo "❌ 基本网络连接失败"
fi

echo

# 检查DNS解析
echo "2. 检查DNS解析..."
if nslookup api.groq.com > /dev/null 2>&1; then
    echo "✅ Groq DNS解析正常"
else
    echo "❌ Groq DNS解析失败"
fi

if nslookup api.openai.com > /dev/null 2>&1; then
    echo "✅ OpenAI DNS解析正常"
else
    echo "❌ OpenAI DNS解析失败"
fi

echo

# 检查HTTPS连接
echo "3. 检查HTTPS连接..."

echo "测试 Groq API 连接..."
if curl -s --connect-timeout 10 --max-time 30 -I https://api.groq.com/openai/v1/models > /dev/null 2>&1; then
    echo "✅ Groq API HTTPS连接正常"
else
    echo "❌ Groq API HTTPS连接失败"
    echo "   尝试详细连接测试..."
    curl -v --connect-timeout 10 --max-time 30 https://api.groq.com/openai/v1/models 2>&1 | head -20
fi

echo
echo "测试 OpenAI API 连接..."
if curl -s --connect-timeout 10 --max-time 30 -I https://api.openai.com/v1/models > /dev/null 2>&1; then
    echo "✅ OpenAI API HTTPS连接正常"
else
    echo "❌ OpenAI API HTTPS连接失败"
    echo "   尝试详细连接测试..."
    curl -v --connect-timeout 10 --max-time 30 https://api.openai.com/v1/models 2>&1 | head -20
fi

echo

# 检查代理设置
echo "4. 检查代理设置..."
if [ ! -z "$HTTP_PROXY" ]; then
    echo "ℹ️  HTTP_PROXY 已设置: $HTTP_PROXY"
else
    echo "ℹ️  HTTP_PROXY 未设置"
fi

if [ ! -z "$HTTPS_PROXY" ]; then
    echo "ℹ️  HTTPS_PROXY 已设置: $HTTPS_PROXY"
else
    echo "ℹ️  HTTPS_PROXY 未设置"
fi

echo

# 检查防火墙
echo "5. 检查防火墙状态..."
if command -v ufw &> /dev/null; then
    ufw_status=$(ufw status 2>/dev/null | head -1)
    echo "ℹ️  UFW状态: $ufw_status"
fi

if command -v iptables &> /dev/null; then
    iptables_rules=$(iptables -L | wc -l)
    echo "ℹ️  iptables规则数量: $iptables_rules"
fi

echo

# 检查系统资源
echo "6. 检查系统资源..."
echo "内存使用情况:"
free -h | head -2

echo
echo "磁盘使用情况:"
df -h / | tail -1

echo

# 提供解决方案
echo "=== 常见网络问题解决方案 ==="
echo
echo "1. unexpected EOF 错误:"
echo "   - 检查网络连接稳定性"
echo "   - 尝试增加timeout设置"
echo "   - 检查是否有代理或防火墙阻止连接"
echo "   - 尝试使用不同的网络环境"
echo
echo "2. 连接超时:"
echo "   - 检查网络速度"
echo "   - 尝试使用VPN或代理"
echo "   - 检查DNS设置"
echo
echo "3. SSL/TLS错误:"
echo "   - 更新系统证书"
echo "   - 检查系统时间是否正确"
echo "   - 尝试禁用SSL验证(仅测试用)"
echo
echo "4. 代理配置:"
echo "   - 如果使用代理，确保正确配置"
echo "   - 检查代理服务器是否正常工作"
echo "   - 尝试绕过代理进行测试"
echo

echo "诊断完成！"
