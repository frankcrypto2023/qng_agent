package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"qng_agent/internal/config"
	"runtime"
	"strings"
	"sync"
	"time"
)

// StreamResponse 流式响应结构
type StreamResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index int `json:"index"`
		Delta struct {
			Content string `json:"content,omitempty"`
			Role    string `json:"role,omitempty"`
		} `json:"delta"`
		FinishReason string `json:"finish_reason,omitempty"`
	} `json:"choices"`
	Usage *struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage,omitempty"`
}

type LlamaCppLocalClient struct {
	config      config.LlamaCppConfig
	client      *http.Client
	baseURL     string
	process     *exec.Cmd
	mu          sync.RWMutex
	initialized bool
}

func NewLlamaCppLocalClient(config config.LlamaCppConfig) (Client, error) {
	// 使用配置的端口，如果没有配置则使用默认端口
	port := 8081
	if config.ServerPort > 0 {
		port = config.ServerPort
	}

	baseURL := fmt.Sprintf("http://localhost:%d", port)
	if config.BaseURL != "" {
		baseURL = config.BaseURL
	}

	client := &http.Client{
		Timeout: time.Duration(config.Timeout) * time.Second,
	}

	llamaClient := &LlamaCppLocalClient{
		config:  config,
		client:  client,
		baseURL: baseURL,
	}

	if err := llamaClient.startLocalService(); err != nil {
		log.Printf("⚠️  无法启动本地llama.cpp服务: %v", err)
		return NewMockClient(), nil
	}

	return llamaClient, nil
}

func (c *LlamaCppLocalClient) startLocalService() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.initialized {
		return nil
	}

	if c.config.ModelPath == "" {
		return fmt.Errorf("model path is required for local llama.cpp")
	}

	if _, err := os.Stat(c.config.ModelPath); os.IsNotExist(err) {
		return fmt.Errorf("model file not found: %s", c.config.ModelPath)
	}
	go func() {
		args := c.buildLlamaCppArgs()
		executablePath := c.config.ExecutablePath
		if executablePath == "" {
			executablePath = "llama-server"
		}
		cmd := exec.Command(executablePath, args...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		log.Printf("🔧 启动本地llama.cpp服务: %s", strings.Join(cmd.Args, " "))

		if err := cmd.Start(); err != nil {
			return
		}

		c.process = cmd
		c.initialized = true
	}()

	time.Sleep(5 * time.Second)

	if err := c.validateService(); err != nil {
		c.stopLocalService()
		return fmt.Errorf("service validation failed: %w", err)
	}

	log.Printf("✅ 本地llama.cpp服务启动成功")
	return nil
}

func (c *LlamaCppLocalClient) buildLlamaCppArgs() []string {
	// 使用配置的端口，如果没有配置则使用默认端口
	port := 8081
	if c.config.ServerPort > 0 {
		port = c.config.ServerPort
	}

	args := []string{
		"--model", c.config.ModelPath,
		"--port", fmt.Sprintf("%d", port),
		"--host", "0.0.0.0",
	}

	if c.config.ContextSize > 0 {
		args = append(args, "--ctx-size", fmt.Sprintf("%d", c.config.ContextSize))
	}
	if c.config.Threads > 0 {
		args = append(args, "--threads", fmt.Sprintf("%d", c.config.Threads))
	}

	if c.config.GPU {
		// args = append(args, "--n-gpu-layers", fmt.Sprintf("%d", c.config.GPULayers))
		// if c.config.GPUThreads > 0 {
		// 	args = append(args, "--n-gpu-threads", fmt.Sprintf("%d", c.config.GPUThreads))
		// }
	}

	if c.config.MemoryF16 {
		args = append(args, "--f16-kv")
	}
	if c.config.MemoryLock {
		args = append(args, "--mlock")
	}

	args = append(args, "--batch-size", "512")
	args = append(args, "--repeat-penalty", fmt.Sprintf("%.2f", c.config.RepeatPenalty))

	return args
}

func (c *LlamaCppLocalClient) validateService() error {
	resp, err := c.client.Get(c.baseURL + "/health")
	if err != nil {
		return fmt.Errorf("failed to connect to local llama.cpp service: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("local llama.cpp service returned status: %d", resp.StatusCode)
	}

	log.Printf("✅ 本地llama.cpp服务连接成功: %s", c.baseURL)
	return nil
}

func (c *LlamaCppLocalClient) stopLocalService() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.process != nil {
		log.Printf("🛑 停止本地llama.cpp服务")
		c.process.Process.Kill()
		c.process.Wait()
		c.process = nil
	}

	c.initialized = false
}

func (c *LlamaCppLocalClient) Chat(ctx context.Context, messages []Message) (string, error) {
	if !c.initialized {
		if err := c.startLocalService(); err != nil {
			return "", fmt.Errorf("failed to start local service: %w", err)
		}
	}

	log.Printf("🔍 本地LlamaCpp客户端诊断信息:")
	log.Printf("  - 服务地址: %s", c.baseURL)
	log.Printf("  - 模型路径: %s", c.config.ModelPath)
	log.Printf("  - 上下文大小: %d", c.config.ContextSize)
	log.Printf("  - 线程数: %d", c.config.Threads)
	log.Printf("  - 温度: %.2f", c.config.Temperature)
	log.Printf("  - GPU: %v", c.config.GPU)

	// 构建请求
	request := map[string]interface{}{
		"model":       filepath.Base(c.config.ModelPath),
		"messages":    messages,
		"temperature": c.config.Temperature,
		"top_p":       c.config.TopP,
		"top_k":       c.config.TopK,
		"max_tokens":  c.config.MaxTokens,
		"stream":      true, // 启用流式响应
		"stop":        []string{"<|im_end|>", "</s>"},
	}

	jsonData, err := json.Marshal(request)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	url := c.baseURL + "/v1/chat/completions"
	log.Printf("🌐 请求URL: %s", url)
	log.Printf("📤 请求数据: %s", string(jsonData))

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	log.Printf("🔍 响应状态码: %d", resp.StatusCode)

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("local llama.cpp API returned status %d: %s", resp.StatusCode, string(body))
	}

	// 处理流式响应
	return c.handleStreamResponse(resp.Body)
}

// handleStreamResponse 处理流式响应
func (c *LlamaCppLocalClient) handleStreamResponse(body io.ReadCloser) (string, error) {
	reader := bufio.NewReader(body)
	var fullContent strings.Builder
	var totalTokens int

	log.Printf("🔄 开始接收流式响应...")

	for {
		// 读取一行数据
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			return "", fmt.Errorf("failed to read stream: %w", err)
		}

		// 去除行尾的换行符
		line = strings.TrimSpace(line)

		// 跳过空行
		if line == "" {
			continue
		}

		// 检查是否是数据行 (以 "data: " 开头)
		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		// 提取JSON数据
		jsonData := strings.TrimPrefix(line, "data: ")

		// 检查是否是结束标记
		if jsonData == "[DONE]" {
			log.Printf("✅ 流式响应接收完成")
			break
		}

		// 解析流式响应
		var streamResp StreamResponse
		if err := json.Unmarshal([]byte(jsonData), &streamResp); err != nil {
			log.Printf("⚠️  解析流式响应失败: %v, 数据: %s", err, jsonData)
			continue
		}
		// fmt.Println("")
		// 处理选择内容
		for _, choice := range streamResp.Choices {
			if choice.Delta.Content != "" {
				fullContent.WriteString(choice.Delta.Content)
				// fmt.Print(choice.Delta.Content)
			}

			// 检查是否完成
			if choice.FinishReason != "" {
				log.Printf("🏁 响应完成，原因: %s", choice.FinishReason)
			}
		}
		// fmt.Println("")

		// 更新token计数
		if streamResp.Usage != nil {
			totalTokens = streamResp.Usage.TotalTokens
		}
	}

	content := fullContent.String()
	log.Printf("✅ 流式响应处理完成，总长度: %d, 总tokens: %d", len(content), totalTokens)

	return content, nil
}

// ChatStream 提供流式聊天接口，返回内容通道
func (c *LlamaCppLocalClient) ChatStream(ctx context.Context, messages []Message) (<-chan string, error) {
	if !c.initialized {
		if err := c.startLocalService(); err != nil {
			return nil, fmt.Errorf("failed to start local service: %w", err)
		}
	}

	// 构建请求
	request := map[string]interface{}{
		"model":       filepath.Base(c.config.ModelPath),
		"messages":    messages,
		"temperature": c.config.Temperature,
		"top_p":       c.config.TopP,
		"top_k":       c.config.TopK,
		"max_tokens":  c.config.MaxTokens,
		"stream":      true,
	}

	jsonData, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := c.baseURL + "/v1/chat/completions"
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("local llama.cpp API returned status %d: %s", resp.StatusCode, string(body))
	}

	// 创建内容通道
	contentChan := make(chan string, 100)

	// 在goroutine中处理流式响应
	go func() {
		defer resp.Body.Close()
		defer close(contentChan)

		reader := bufio.NewReader(resp.Body)

		for {
			select {
			case <-ctx.Done():
				log.Printf("⚠️  上下文取消，停止流式接收")
				return
			default:
			}

			// 读取一行数据
			line, err := reader.ReadString('\n')
			if err != nil {
				if err == io.EOF {
					log.Printf("✅ 流式响应接收完成")
					break
				}
				log.Printf("❌ 读取流式响应失败: %v", err)
				break
			}

			// 去除行尾的换行符
			line = strings.TrimSpace(line)

			// 跳过空行
			if line == "" {
				continue
			}

			// 检查是否是数据行
			if !strings.HasPrefix(line, "data: ") {
				continue
			}

			// 提取JSON数据
			jsonData := strings.TrimPrefix(line, "data: ")

			// 检查是否是结束标记
			if jsonData == "[DONE]" {
				log.Printf("✅ 流式响应接收完成")
				break
			}

			// 解析流式响应
			var streamResp StreamResponse
			if err := json.Unmarshal([]byte(jsonData), &streamResp); err != nil {
				log.Printf("⚠️  解析流式响应失败: %v, 数据: %s", err, jsonData)
				continue
			}

			// 处理选择内容
			for _, choice := range streamResp.Choices {
				if choice.Delta.Content != "" {
					// 发送内容到通道
					select {
					case contentChan <- choice.Delta.Content:
					case <-ctx.Done():
						return
					}
				}

				// 检查是否完成
				if choice.FinishReason != "" {
					log.Printf("🏁 响应完成，原因: %s", choice.FinishReason)
				}
			}
		}
	}()

	return contentChan, nil
}

func (c *LlamaCppLocalClient) Close() error {
	c.stopLocalService()
	return nil
}

func (c *LlamaCppLocalClient) GetModelInfo() map[string]interface{} {
	return map[string]interface{}{
		"provider":     "llamacpp_local",
		"base_url":     c.baseURL,
		"model_path":   c.config.ModelPath,
		"context_size": c.config.ContextSize,
		"threads":      c.config.Threads,
		"temperature":  c.config.Temperature,
		"gpu":          c.config.GPU,
		"initialized":  c.initialized,
	}
}

func (c *LlamaCppLocalClient) CheckLlamaCppInstallation() error {
	cmd := exec.Command("llama.cpp", "--help")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("llama.cpp not found or not executable: %w", err)
	}

	log.Printf("✅ llama.cpp 已安装")
	return nil
}

func (c *LlamaCppLocalClient) GetSystemInfo() map[string]interface{} {
	return map[string]interface{}{
		"os":                  runtime.GOOS,
		"arch":                runtime.GOARCH,
		"cpu_count":           runtime.NumCPU(),
		"go_version":          runtime.Version(),
		"llama_cpp_installed": c.CheckLlamaCppInstallation() == nil,
	}
}
