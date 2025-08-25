package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"qng_agent/internal/config"
	"qng_agent/internal/harmony"
	"strings"
	"time"
)

// WebServer Harmony Web 服务器
type WebServer struct {
	harmonyClient *harmony.LlamaCppHarmonyClient
	port          string
	webuiPath     string
}

// ChatRequest Web 聊天请求
type ChatRequest struct {
	Message string `json:"message"`
	Mode    string `json:"mode"` // basic, tools, stream
}

// ChatResponse Web 聊天响应
type ChatResponse struct {
	Response string         `json:"response"`
	Error    string         `json:"error,omitempty"`
	Usage    *harmony.Usage `json:"usage,omitempty"`
}

// NewWebServer 创建新的 Web 服务器
func NewWebServer(baseURL, port string) *WebServer {
	if port == "" {
		port = "8080"
	}

	// 设置 Web UI 路径
	webuiPath := "cmd/harmony_demo/frontend"

	// 创建默认配置
	defaultConfig := &config.LlamaCppConfig{
		Temperature:      0.8,
		TopP:             0.95,
		TopK:             40,
		MaxTokens:        2000,
		RepeatPenalty:    1.1,
		CachePrompt:      true,
		ReasoningFormat:  "none",
		Samplers:         "edkypmxt",
		DynatempRange:    0,
		DynatempExponent: 1,
		MinP:             0.05,
		TypicalP:         1,
		XtcProbability:   0,
		XtcThreshold:     0.1,
		RepeatLastN:      64,
		PresencePenalty:  0,
		FrequencyPenalty: 0,
		DryMultiplier:    0,
		DryBase:          1.75,
		DryAllowedLength: 2,
		DryPenaltyLastN:  -1,
		TimingsPerToken:  true,
	}

	return &WebServer{
		harmonyClient: harmony.NewLlamaCppHarmonyClient(baseURL, defaultConfig),
		port:          port,
		webuiPath:     webuiPath,
	}
}

// Start 启动 Web 服务器
func (s *WebServer) Start() error {
	// 构建 Web UI
	if err := s.buildWebUI(); err != nil {
		log.Printf("⚠️  Web UI 构建失败，使用备用界面: %v", err)
	} else {
		log.Printf("✅ Web UI 构建成功")
	}

	// 设置路由
	http.HandleFunc("/", s.handleHome)
	http.HandleFunc("/chat", s.handleChat)
	http.HandleFunc("/stream", s.handleStream)
	http.HandleFunc("/static/", s.handleStatic)
	http.HandleFunc("/assets/", s.handleAssets)

	// 启动服务器
	addr := ":" + s.port
	log.Printf("🚀 Harmony Web 服务器启动在 http://localhost%s", addr)
	log.Printf("📝 支持的模式: basic, tools, stream")

	return http.ListenAndServe(addr, nil)
}

// buildWebUI 构建 Web UI
func (s *WebServer) buildWebUI() error {
	// 检查 Web UI 目录是否存在
	if _, err := os.Stat(s.webuiPath); os.IsNotExist(err) {
		return fmt.Errorf("Web UI 目录不存在: %s", s.webuiPath)
	}

	// 检查 package.json 是否存在
	packagePath := filepath.Join(s.webuiPath, "package.json")
	if _, err := os.Stat(packagePath); os.IsNotExist(err) {
		return fmt.Errorf("package.json 不存在: %s", packagePath)
	}

	// 切换到 Web UI 目录
	originalDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("获取当前目录失败: %w", err)
	}
	defer os.Chdir(originalDir)

	if err := os.Chdir(s.webuiPath); err != nil {
		return fmt.Errorf("切换到 Web UI 目录失败: %w", err)
	}

	// 安装依赖
	log.Printf("📦 安装 Web UI 依赖...")
	cmd := exec.Command("npm", "install")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("安装依赖失败: %w", err)
	}

	// 构建项目
	log.Printf("🔨 构建 Web UI...")
	cmd = exec.Command("npm", "run", "build")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("构建失败: %w", err)
	}

	return nil
}

// handleHome 处理主页
func (s *WebServer) handleHome(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	// 尝试提供构建后的 index.html
	distIndexPath := filepath.Join(s.webuiPath, "dist", "index.html")
	if _, err := os.Stat(distIndexPath); err == nil {
		// 构建后的文件存在，直接提供
		http.ServeFile(w, r, distIndexPath)
		return
	}

	// 如果构建后的文件不存在，使用备用界面
	s.serveFallbackUI(w, r)
}

// serveFallbackUI 提供备用界面
func (s *WebServer) serveFallbackUI(w http.ResponseWriter, r *http.Request) {
	// 设置 HTML 模板 - 基于 llama.cpp 官方实现
	tmpl := `
<!DOCTYPE html>
<html lang="zh-CN" data-theme="dark">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <meta name="color-scheme" content="light dark">
    <title>🦙 Harmony GPT-OSS Chat</title>
    <script src="https://cdn.tailwindcss.com"></script>
    <link href="https://cdn.jsdelivr.net/npm/daisyui@4.7.2/dist/full.min.css" rel="stylesheet" type="text/css" />
    <link href="https://cdn.jsdelivr.net/npm/heroicons@2.0.18/24/outline/index.css" rel="stylesheet" />
    <script>
        tailwind.config = {
            theme: {
                extend: {
                    colors: {
                        'base-100': '#1a1a1a',
                        'base-200': '#2d2d2d',
                        'base-300': '#404040',
                        'base-content': '#e0e0e0',
                    }
                }
            }
        }
    </script>
    <style>
        * {
            margin: 0;
            padding: 0;
            box-sizing: border-box;
        }
        
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif;
            background: #1a1a1a;
            color: #e0e0e0;
            min-height: 100vh;
        }
        
        html {
            scrollbar-gutter: auto;
        }
        
        .chat-screen {
            max-width: 900px;
        }
        
        .chat-bubble {
            word-break: break-words;
        }
        
        .chat-bubble-base-300 {
            --tw-bg-opacity: 1;
            --tw-text-opacity: 1;
            background: #404040;
            color: #e0e0e0;
        }
        
        .message {
            margin-bottom: 20px;
            display: flex;
            align-items: flex-start;
            gap: 12px;
        }
        
        .message.user {
            flex-direction: row-reverse;
        }
        
        .message.assistant {
            flex-direction: row;
        }
        
        .message-avatar {
            width: 32px;
            height: 32px;
            border-radius: 50%;
            display: flex;
            align-items: center;
            justify-content: center;
            font-size: 14px;
            font-weight: 600;
            flex-shrink: 0;
        }
        
        .message.user .message-avatar {
            background: #007acc;
            color: white;
        }
        
        .message.assistant .message-avatar {
            background: #4caf50;
            color: white;
        }
        
        .message-content {
            max-width: 70%;
            padding: 12px 16px;
            border-radius: 12px;
            font-size: 14px;
            line-height: 1.5;
            word-wrap: break-word;
        }
        
        .message.user .message-content {
            background: #007acc;
            color: white;
            border-bottom-right-radius: 4px;
        }
        
        .message.assistant .message-content {
            background: #2d2d2d;
            color: #e0e0e0;
            border: 1px solid #404040;
            border-bottom-left-radius: 4px;
        }
        
        .message-time {
            font-size: 11px;
            color: #888888;
            margin-top: 4px;
        }
        
        .message.user .message-time {
            text-align: right;
        }
        
        .message.assistant .message-time {
            text-align: left;
        }
        
        .typing-indicator {
            display: none;
            padding: 12px 16px;
            background: #2d2d2d;
            border: 1px solid #404040;
            border-radius: 12px;
            color: #a0a0a0;
            font-style: italic;
            margin: 10px 0;
            border-bottom-left-radius: 4px;
        }
        
        .typing-indicator.show {
            display: block;
        }
        
        .usage-info {
            font-size: 12px;
            color: #888888;
            margin-top: 10px;
            text-align: center;
            padding: 8px;
            background: #2d2d2d;
            border-radius: 6px;
            border: 1px solid #404040;
        }
        
        .error {
            color: #ff6b6b;
            background: #2d1a1a;
            border: 1px solid #4a2a2a;
            padding: 12px 16px;
            border-radius: 8px;
            margin: 10px 0;
            font-size: 14px;
        }
        
        .welcome-message {
            text-align: center;
            padding: 40px 20px;
            color: #a0a0a0;
        }
        
        .welcome-message h2 {
            color: #ffffff;
            margin-bottom: 10px;
            font-size: 24px;
        }
        
        .welcome-message p {
            font-size: 16px;
            line-height: 1.6;
        }
        
        .features {
            display: flex;
            justify-content: center;
            gap: 30px;
            margin-top: 30px;
            flex-wrap: wrap;
        }
        
        .feature {
            text-align: center;
            padding: 20px;
            background: #2d2d2d;
            border-radius: 12px;
            border: 1px solid #404040;
            min-width: 200px;
        }
        
        .feature h3 {
            color: #007acc;
            margin-bottom: 10px;
            font-size: 16px;
        }
        
        .feature p {
            color: #a0a0a0;
            font-size: 14px;
        }
        
        @media (max-width: 768px) {
            .message-content {
                max-width: 85%;
                font-size: 13px;
            }
            
            .features {
                gap: 15px;
            }
            
            .feature {
                min-width: 150px;
                padding: 15px;
            }
        }
    </style>
</head>
<body>
    <div class="flex flex-row drawer lg:drawer-open">
        <!-- 侧边栏 -->
        <input
            id="toggle-drawer"
            type="checkbox"
            class="drawer-toggle"
            aria-label="Toggle sidebar"
            defaultChecked
        />

        <div
            class="drawer-side h-screen lg:h-screen z-50 lg:max-w-64"
            role="complementary"
            aria-label="Sidebar"
            tabIndex="0"
        >
            <label
                htmlFor="toggle-drawer"
                aria-label="Close sidebar"
                class="drawer-overlay"
            ></label>

            <div class="flex flex-col bg-base-200 min-h-full max-w-64 py-4 px-4">
                <div class="flex flex-row items-center justify-between mb-4 mt-4">
                    <h2 class="font-bold ml-4" role="heading">
                        🦙 Harmony GPT-OSS
                    </h2>

                    <!-- close sidebar button -->
                    <label
                        htmlFor="toggle-drawer"
                        class="btn btn-ghost lg:hidden"
                        aria-label="Close sidebar"
                        role="button"
                        tabIndex="0"
                    >
                        <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12"></path>
                        </svg>
                    </label>
                </div>

                <!-- 模式选择 -->
                <div class="form-control mb-4">
                    <label class="label">
                        <span class="label-text">对话模式</span>
                    </label>
                    <select class="select select-bordered w-full" id="modeSelector">
                        <option value="basic">基本对话</option>
                        <option value="tools">工具调用</option>
                        <option value="stream">流式对话</option>
                    </select>
                </div>

                <div class="divider"></div>

                <!-- 系统信息 -->
                <div class="text-sm text-base-content/70 mt-4">
                    <p>基于 OpenAI Harmony 格式的智能对话系统</p>
                    <p class="mt-2">支持多种对话模式和工具调用功能</p>
                </div>

                <!-- 功能特性 -->
                <div class="mt-6 space-y-2">
                    <div class="flex items-center gap-2 text-sm">
                        <div class="w-2 h-2 bg-primary rounded-full"></div>
                        <span>基本对话</span>
                    </div>
                    <div class="flex items-center gap-2 text-sm">
                        <div class="w-2 h-2 bg-secondary rounded-full"></div>
                        <span>工具调用</span>
                    </div>
                    <div class="flex items-center gap-2 text-sm">
                        <div class="w-2 h-2 bg-accent rounded-full"></div>
                        <span>流式响应</span>
                    </div>
                </div>
            </div>
        </div>
        
        <!-- 主内容区域 -->
        <div class="drawer-content flex flex-col">
            <!-- 头部 -->
            <div class="flex flex-row items-center pt-6 pb-6 sticky top-0 z-10 bg-base-100">
                <!-- open sidebar button -->
                <label htmlFor="toggle-drawer" class="btn btn-ghost lg:hidden">
                    <svg class="h-5 w-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 6h16M4 12h16M4 18h16"></path>
                    </svg>
                </label>

                <div class="grow text-2xl font-bold ml-2">Harmony GPT-OSS</div>

                <!-- action buttons (top right) -->
                <div class="flex items-center">
                    <div class="tooltip tooltip-bottom" data-tip="设置">
                        <button class="btn" id="settingsBtn">
                            <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z"></path>
                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z"></path>
                            </svg>
                        </button>
                    </div>

                    <!-- theme controller -->
                    <div class="tooltip tooltip-bottom" data-tip="主题">
                        <div class="dropdown dropdown-end dropdown-bottom">
                            <div tabIndex="0" role="button" class="btn m-1">
                                <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M20.354 15.354A9 9 0 018.646 3.646 9.003 9.003 0 0012 21a9.003 9.003 0 008.354-5.646z"></path>
                                </svg>
                            </div>
                            <ul tabIndex="0" class="dropdown-content bg-base-300 rounded-box z-[1] w-52 p-2 shadow-2xl">
                                <li>
                                    <button class="btn btn-sm btn-block btn-ghost justify-start" onclick="setTheme('light')">
                                        light
                                    </button>
                                </li>
                                <li>
                                    <button class="btn btn-sm btn-block btn-ghost justify-start" onclick="setTheme('dark')">
                                        dark
                                    </button>
                                </li>
                                <li>
                                    <button class="btn btn-sm btn-block btn-ghost justify-start" onclick="setTheme('auto')">
                                        auto
                                    </button>
                                </li>
                            </ul>
                        </div>
                    </div>
                </div>
            </div>
            
            <!-- 聊天区域 -->
            <main class="drawer-content grow flex flex-col h-screen mx-auto px-4 overflow-auto bg-base-100" id="main-scroll">
                <div class="chat-screen">
                    <div class="flex-1 overflow-y-auto p-4" id="chatContainer">
                        <div class="welcome-message">
                            <h2>欢迎使用 Harmony GPT-OSS</h2>
                            <p>这是一个基于 OpenAI Harmony 格式的智能对话系统，支持多种对话模式和工具调用功能。</p>
                            <div class="features">
                                <div class="feature">
                                    <h3>🎯 基本对话</h3>
                                    <p>普通的 AI 对话模式，适合日常交流</p>
                                </div>
                                <div class="feature">
                                    <h3>🔧 工具调用</h3>
                                    <p>支持函数调用的智能对话</p>
                                </div>
                                <div class="feature">
                                    <h3>⚡ 流式对话</h3>
                                    <p>实时流式响应，更好的用户体验</p>
                                </div>
                            </div>
                        </div>
                    </div>
                    
                    <div class="typing-indicator" id="typingIndicator">
                        AI 正在思考中...
                    </div>
                    
                    <!-- 输入区域 -->
                    <div class="bg-base-200 border-t border-base-300 p-4">
                        <div class="flex gap-3 items-end max-w-4xl mx-auto">
                            <textarea 
                                class="textarea textarea-bordered flex-1 min-h-[44px] max-h-32 resize-none" 
                                id="messageInput" 
                                placeholder="输入你的消息..." 
                                rows="1"
                            ></textarea>
                            <button class="btn btn-primary" id="sendButton" onclick="sendMessage()">
                                <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 19l9 2-9-18-9 18 9-2zm0 0v-8"></path>
                                </svg>
                            </button>
                        </div>
                        <div class="usage-info" id="usageInfo"></div>
                    </div>
                </div>
            </main>
        </div>
    </div>

    <script>
        let isStreaming = false;
        let currentTheme = 'dark';
        
        // 主题管理
        function setTheme(theme) {
            currentTheme = theme;
            document.documentElement.setAttribute('data-theme', theme);
            localStorage.setItem('harmony-theme', theme);
        }
        
        // 初始化主题
        function initTheme() {
            const savedTheme = localStorage.getItem('harmony-theme') || 'dark';
            setTheme(savedTheme);
        }
        
        function addMessage(content, isUser = false) {
            const chatContainer = document.getElementById('chatContainer');
            
            // 如果是第一条消息，清除欢迎界面
            if (chatContainer.querySelector('.welcome-message')) {
                chatContainer.innerHTML = '';
            }
            
            const messageDiv = document.createElement('div');
            messageDiv.className = 'message ' + (isUser ? 'user' : 'assistant');
            
            // 添加头像
            const avatarDiv = document.createElement('div');
            avatarDiv.className = 'message-avatar';
            avatarDiv.textContent = isUser ? 'U' : 'A';
            
            // 添加消息内容
            const contentDiv = document.createElement('div');
            contentDiv.className = 'message-content';
            contentDiv.innerHTML = content;
            
            // 添加时间戳
            const timeDiv = document.createElement('div');
            timeDiv.className = 'message-time';
            timeDiv.textContent = new Date().toLocaleTimeString();
            
            // 组装消息
            messageDiv.appendChild(avatarDiv);
            messageDiv.appendChild(contentDiv);
            contentDiv.appendChild(timeDiv);
            
            chatContainer.appendChild(messageDiv);
            chatContainer.scrollTop = chatContainer.scrollHeight;
        }
        
        function showTypingIndicator() {
            document.getElementById('typingIndicator').classList.add('show');
        }
        
        function hideTypingIndicator() {
            document.getElementById('typingIndicator').classList.remove('show');
        }
        
                 function updateUsageInfo(usage) {
             if (usage) {
                 const usageInfo = document.getElementById('usageInfo');
                 usageInfo.innerHTML = 'Token 使用: 提示=' + usage.prompt_tokens + ', 完成=' + usage.completion_tokens + ', 总计=' + usage.total_tokens;
             }
         }
        
        function showError(message) {
            const chatContainer = document.getElementById('chatContainer');
            const errorDiv = document.createElement('div');
            errorDiv.className = 'error';
            errorDiv.textContent = '错误: ' + message;
            chatContainer.appendChild(errorDiv);
            chatContainer.scrollTop = chatContainer.scrollHeight;
        }
        
        async function sendMessage() {
            const messageInput = document.getElementById('messageInput');
            const sendButton = document.getElementById('sendButton');
            const modeSelector = document.getElementById('modeSelector');
            
            const message = messageInput.value.trim();
            if (!message) return;
            
            const mode = modeSelector.value;
            
            // 禁用输入
            messageInput.disabled = true;
            sendButton.disabled = true;
            
            // 添加用户消息
            addMessage(message, true);
            messageInput.value = '';
            
            // 显示加载状态
            showTypingIndicator();
            
            try {
                if (mode === 'stream') {
                    await sendStreamMessage(message);
                } else {
                    await sendNormalMessage(message, mode);
                }
            } catch (error) {
                console.error('发送消息失败:', error);
                showError(error.message);
            } finally {
                // 恢复输入
                messageInput.disabled = false;
                sendButton.disabled = false;
                messageInput.focus();
                hideTypingIndicator();
            }
        }
        
        async function sendNormalMessage(message, mode) {
            const response = await fetch('/chat', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                },
                body: JSON.stringify({
                    message: message,
                    mode: mode
                })
            });
            
            if (!response.ok) {
                throw new Error('网络请求失败');
            }
            
            const data = await response.json();
            
            if (data.error) {
                throw new Error(data.error);
            }
            
            addMessage(data.response);
            updateUsageInfo(data.usage);
        }
        
        async function sendStreamMessage(message) {
            console.log('🔄 开始流式请求...');
            
            try {
                const response = await fetch('/stream', {
                    method: 'POST',
                    headers: {
                        'Content-Type': 'application/json',
                    },
                    body: JSON.stringify({
                        message: message,
                        mode: 'stream'
                    })
                });
                
                if (!response.ok) {
                    throw new Error('Request failed: ' + response.status + ' ' + response.statusText);
                }
                
                console.log('✅ 流式请求成功，开始读取数据...');
                
                const reader = response.body.getReader();
                const decoder = new TextDecoder();
                let fullResponse = '';
                let isFirstChunk = true;
                let chunkCount = 0;
                
                while (true) {
                    const { done, value } = await reader.read();
                    
                    if (done) {
                        console.log('✅ 流式响应完成');
                        break;
                    }
                    
                    const chunk = decoder.decode(value);
                    const lines = chunk.split('\n');
                    
                    for (const line of lines) {
                        if (line.trim() === '') continue;
                        
                        if (line.startsWith('data: ')) {
                            const data = line.slice(6);
                            
                            if (data === '[DONE]') {
                                console.log('✅ 收到完成信号');
                                return;
                            }
                            
                            try {
                                const parsed = JSON.parse(data);
                                chunkCount++;
                                console.log('📦 收到第 ' + chunkCount + ' 个块:', parsed.response);
                                
                                if (isFirstChunk) {
                                    addMessage(parsed.response);
                                    fullResponse = parsed.response;
                                    isFirstChunk = false;
                                } else {
                                    // 累积内容以实现真正的流式效果
                                    fullResponse += parsed.response;
                                    const messages = document.querySelectorAll('.message.assistant .message-content');
                                    const lastMessage = messages[messages.length - 1];
                                    if (lastMessage) {
                                        lastMessage.innerHTML = fullResponse;
                                        // 滚动到底部
                                        const chatContainer = document.getElementById('chatContainer');
                                        chatContainer.scrollTop = chatContainer.scrollHeight;
                                    }
                                }
                                
                            } catch (e) {
                                console.error('❌ 解析流数据失败:', e, '原始数据:', data);
                            }
                        }
                    }
                }
            } catch (error) {
                console.error('❌ 流式请求失败:', error);
                showError(error.message);
            }
        }
        
        // 回车发送消息
        document.getElementById('messageInput').addEventListener('keypress', function(e) {
            if (e.key === 'Enter' && !e.shiftKey) {
                e.preventDefault();
                sendMessage();
            }
        });
        
        // 自动调整文本框高度
        document.getElementById('messageInput').addEventListener('input', function() {
            this.style.height = 'auto';
            this.style.height = Math.min(this.scrollHeight, 120) + 'px';
        });
        
        // 模式切换时清空聊天记录
        document.getElementById('modeSelector').addEventListener('change', function() {
            const chatContainer = document.getElementById('chatContainer');
            chatContainer.innerHTML = '<div class="welcome-message"><h2>模式已切换</h2><p>开始新的对话吧！</p></div>';
            document.getElementById('usageInfo').innerHTML = '';
        });
        
        // 初始化
        document.addEventListener('DOMContentLoaded', function() {
            initTheme();
        });
    </script>
</body>
</html>`

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(tmpl))
}

// handleChat 处理聊天请求
func (s *WebServer) handleChat(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// 创建对话
	conv := s.createConversation(req.Message, req.Mode)

	// 发送请求
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	resp, err := s.harmonyClient.Chat(ctx, conv, &harmony.ChatOptions{
		Temperature: 0.7,
		MaxTokens:   800,
	})

	response := ChatResponse{}
	if err != nil {
		response.Error = err.Error()
	} else {
		response.Response = getContentString(resp.Content)
		response.Usage = resp.Usage
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleStream 处理流式聊天请求
func (s *WebServer) handleStream(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// 设置 SSE 头
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// 创建对话
	conv := s.createConversation(req.Message, req.Mode)

	// 发送流式请求
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	log.Printf("🔄 开始流式请求...")
	stream, err := s.harmonyClient.ChatStream(ctx, conv, &harmony.ChatOptions{
		Temperature: 0.7,
		MaxTokens:   800,
	})

	if err != nil {
		log.Printf("❌ 流式请求失败: %v", err)
		fmt.Fprintf(w, "data: %s\n\n", err.Error())
		return
	}
	log.Printf("✅ 流式请求成功")

	var fullContent strings.Builder
	chunkCount := 0

	for {
		select {
		case <-ctx.Done():
			log.Printf("⏰ 流式请求超时或被取消")
			fmt.Fprintf(w, "data: [DONE]\n\n")
			return
		case resp := <-stream:
			chunkCount++
			log.Printf("📦 收到第 %d 个流式块", chunkCount)

			if resp.Error != nil {
				log.Printf("❌ 流式响应错误: %v", resp.Error)
				fmt.Fprintf(w, "data: %s\n\n", resp.Error.Error())
				return
			}

			if resp.Done {
				log.Printf("✅ 流式响应完成，总共 %d 个块", chunkCount)
				fmt.Fprintf(w, "data: [DONE]\n\n")
				return
			}

			// 检查 Content 是否为空
			if resp.Content.Content == nil {
				log.Printf("⚠️ 收到空的 Content.Content，跳过")
				continue
			}

			// 获取增量内容
			content := getContentString(resp.Content)
			fullContent.WriteString(content)
			log.Printf("📝 增量内容: %q", content)

			// 发送 SSE 数据 - 发送增量内容以实现真正的流式效果
			response := ChatResponse{
				Response: content, // 只发送增量内容，不是累积内容
			}

			jsonData, _ := json.Marshal(response)
			fmt.Fprintf(w, "data: %s\n\n", string(jsonData))

			// 刷新缓冲区
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
		}
	}
}

// handleStatic 处理静态文件
func (s *WebServer) handleStatic(w http.ResponseWriter, r *http.Request) {
	http.NotFound(w, r)
}

// handleAssets 处理构建后的静态资源
func (s *WebServer) handleAssets(w http.ResponseWriter, r *http.Request) {
	// 尝试从构建目录提供文件
	distPath := filepath.Join(s.webuiPath, "dist")
	// 正确处理 assets 路径，文件在 dist/assets/ 目录下
	filePath := filepath.Join(distPath, "assets", strings.TrimPrefix(r.URL.Path, "/assets/"))

	// 添加调试日志
	log.Printf("🔍 请求静态资源: %s", r.URL.Path)
	log.Printf("📁 文件路径: %s", filePath)

	// 检查文件是否存在
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		log.Printf("❌ 文件不存在: %s", filePath)
		http.NotFound(w, r)
		return
	}

	log.Printf("✅ 文件存在: %s", filePath)

	// 设置正确的 MIME 类型
	ext := filepath.Ext(filePath)
	switch ext {
	case ".js":
		w.Header().Set("Content-Type", "application/javascript")
	case ".css":
		w.Header().Set("Content-Type", "text/css")
	case ".html":
		w.Header().Set("Content-Type", "text/html")
	case ".json":
		w.Header().Set("Content-Type", "application/json")
	case ".woff2":
		w.Header().Set("Content-Type", "font/woff2")
	case ".woff":
		w.Header().Set("Content-Type", "font/woff")
	case ".ttf":
		w.Header().Set("Content-Type", "font/ttf")
	case ".png":
		w.Header().Set("Content-Type", "image/png")
	case ".jpg", ".jpeg":
		w.Header().Set("Content-Type", "image/jpeg")
	case ".svg":
		w.Header().Set("Content-Type", "image/svg+xml")
	default:
		w.Header().Set("Content-Type", "application/octet-stream")
	}

	http.ServeFile(w, r, filePath)
}

// createConversation 根据模式和消息创建对话
func (s *WebServer) createConversation(message, mode string) harmony.Conversation {
	conv := harmony.Conversation{
		Messages: []harmony.Message{
			harmony.NewSystemMessage(harmony.SystemContent{
				Instructions: "你是一个有用的AI助手，请用中文回答问题。",
				Knowledge:    "2024-06",
				CurrentDate:  "2025-01-28",
				Reasoning:    "high",
				ValidChannels: []harmony.Channel{
					harmony.ChannelAnalysis,
					harmony.ChannelCommentary,
					harmony.ChannelFinal,
				},
			}),
			harmony.NewUserMessage(message),
		},
	}

	// 根据模式添加不同的配置
	switch mode {
	case "tools":
		// 添加工具定义
		tools := map[string]interface{}{
			"functions": map[string]interface{}{
				"get_weather": "(_: {location: string, format?: 'celsius' | 'fahrenheit'}) => any",
				"get_time":    "() => any",
				"calculate":   "(_: {expression: string}) => any",
				"search":      "(_: {query: string}) => any",
			},
		}

		conv.Messages[0] = harmony.NewSystemMessage(harmony.SystemContent{
			Instructions: "你是一个有用的AI助手，可以使用工具来帮助用户。",
			Knowledge:    "2024-06",
			CurrentDate:  "2025-01-28",
			Reasoning:    "high",
			ValidChannels: []harmony.Channel{
				harmony.ChannelAnalysis,
				harmony.ChannelCommentary,
				harmony.ChannelFinal,
			},
			ToolChannels: map[string]harmony.Channel{
				"functions": harmony.ChannelCommentary,
			},
		})

		// 在用户消息前插入开发者消息
		devMsg := harmony.NewDeveloperMessage(harmony.DeveloperContent{
			Instructions: "你可以使用以下工具来帮助用户：",
			Tools:        tools,
		})

		conv.Messages = append(conv.Messages[:1], devMsg, conv.Messages[1])

	case "stream":
		// 流式模式使用基本配置
		conv.Messages[0] = harmony.NewSystemMessage(harmony.SystemContent{
			Instructions: "你是一个有用的AI助手，请用中文回答问题。",
			Knowledge:    "2024-06",
			CurrentDate:  "2025-01-28",
			Reasoning:    "high",
			ValidChannels: []harmony.Channel{
				harmony.ChannelAnalysis,
				harmony.ChannelCommentary,
				harmony.ChannelFinal,
			},
		})
	}

	return conv
}
