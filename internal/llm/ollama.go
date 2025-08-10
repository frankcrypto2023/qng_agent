package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"qng_agent/internal/config"
	"strings"
	"time"
)

type OllamaClient struct {
	config  config.LlamaCppConfig
	client  *http.Client
	baseURL string
}

// OllamaRequest 请求结构 (参考Ollama API)
type OllamaRequest struct {
	Model       string         `json:"model"`
	Prompt      string         `json:"prompt"`
	Messages    []Message      `json:"messages,omitempty"`
	Temperature float64        `json:"temperature,omitempty"`
	TopP        float64        `json:"top_p,omitempty"`
	TopK        int            `json:"top_k,omitempty"`
	MaxTokens   int            `json:"max_tokens,omitempty"`
	StopWords   []string       `json:"stop,omitempty"`
	Stream      bool           `json:"stream,omitempty"`
	Options     *OllamaOptions `json:"options,omitempty"`
}

// OllamaOptions 模型选项
type OllamaOptions struct {
	Temperature   float64 `json:"temperature,omitempty"`
	TopP          float64 `json:"top_p,omitempty"`
	TopK          int     `json:"top_k,omitempty"`
	RepeatPenalty float64 `json:"repeat_penalty,omitempty"`
	Seed          int     `json:"seed,omitempty"`
	NumCtx        int     `json:"num_ctx,omitempty"`
	NumThread     int     `json:"num_thread,omitempty"`
	NumGPU        int     `json:"num_gpu,omitempty"`
	NumGQA        int     `json:"num_gqa,omitempty"`
	NumKeep       int     `json:"num_keep,omitempty"`
	F16KV         bool    `json:"f16_kv,omitempty"`
	LogitsAll     bool    `json:"logits_all,omitempty"`
	VocabOnly     bool    `json:"vocab_only,omitempty"`
	UseMlock      bool    `json:"use_mlock,omitempty"`
	UseMMap       bool    `json:"use_mmap,omitempty"`
	Embedding     bool    `json:"embedding,omitempty"`
	RopeFreqBase  float64 `json:"rope_freq_base,omitempty"`
	RopeFreqScale float64 `json:"rope_freq_scale,omitempty"`
	MulMatQ       bool    `json:"mul_mat_q,omitempty"`
}

// OllamaResponse 响应结构
type OllamaResponse struct {
	Model              string `json:"model"`
	CreatedAt          string `json:"created_at"`
	Response           string `json:"response"`
	Done               bool   `json:"done"`
	Context            []int  `json:"context"`
	TotalDuration      int64  `json:"total_duration"`
	LoadDuration       int64  `json:"load_duration"`
	PromptEvalCount    int    `json:"prompt_eval_count"`
	PromptEvalDuration int64  `json:"prompt_eval_duration"`
	EvalCount          int    `json:"eval_count"`
	EvalDuration       int64  `json:"eval_duration"`
}

// OllamaError 错误响应结构
type OllamaError struct {
	Error string `json:"error"`
}

func NewOllamaClient(config config.LlamaCppConfig) (Client, error) {
	// 设置默认的Ollama服务地址
	baseURL := "http://localhost:11434"
	if config.BaseURL != "" {
		baseURL = config.BaseURL
	}

	client := &http.Client{
		Timeout: time.Duration(config.Timeout) * time.Second,
	}

	ollamaClient := &OllamaClient{
		config:  config,
		client:  client,
		baseURL: baseURL,
	}

	// 验证服务是否可用
	if err := ollamaClient.validateService(); err != nil {
		log.Printf("⚠️  Ollama服务不可用: %v", err)
		return NewMockClient(), nil
	}

	return ollamaClient, nil
}

func (c *OllamaClient) validateService() error {
	// 检查Ollama服务健康状态
	resp, err := c.client.Get(c.baseURL + "/api/tags")
	if err != nil {
		return fmt.Errorf("failed to connect to Ollama service: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Ollama service returned status: %d", resp.StatusCode)
	}

	log.Printf("✅ Ollama服务连接成功: %s", c.baseURL)
	return nil
}

func (c *OllamaClient) Chat(ctx context.Context, messages []Message) (string, error) {
	log.Printf("🔍 Ollama客户端诊断信息:")
	log.Printf("  - 服务地址: %s", c.baseURL)
	log.Printf("  - 模型名称: %s", c.config.ModelPath)
	log.Printf("  - 温度: %.2f", c.config.Temperature)
	log.Printf("  - TopP: %.2f", c.config.TopP)
	log.Printf("  - TopK: %d", c.config.TopK)
	log.Printf("  - 最大令牌数: %d", c.config.MaxTokens)
	log.Printf("  - 超时: %d秒", c.config.Timeout)

	// 构建Ollama选项
	options := &OllamaOptions{
		Temperature:   c.config.Temperature,
		TopP:          c.config.TopP,
		TopK:          c.config.TopK,
		RepeatPenalty: c.config.RepeatPenalty,
		NumCtx:        c.config.ContextSize,
		NumThread:     c.config.Threads,
		UseMlock:      c.config.MemoryLock,
		UseMMap:       c.config.MemoryMap,
		F16KV:         c.config.MemoryF16,
	}

	// GPU相关设置
	if c.config.GPU {
		options.NumGPU = c.config.GPULayers
	}

	// 构建请求 - 使用messages格式 (推荐)
	request := OllamaRequest{
		Model:     c.config.ModelPath,
		Messages:  messages,
		Options:   options,
		Stream:    false,
		StopWords: []string{"</s>", "<|endoftext|>", "<|im_end|>", "Human:", "Assistant:"},
	}

	jsonData, err := json.Marshal(request)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	// 发送请求到Ollama API
	url := c.baseURL + "/api/chat"
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

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	log.Printf("🔍 响应状态码: %d", resp.StatusCode)
	log.Printf("🔍 响应内容: %s", string(body))

	if resp.StatusCode != http.StatusOK {
		var errorResp OllamaError
		if err := json.Unmarshal(body, &errorResp); err == nil {
			return "", fmt.Errorf("Ollama API error: %s", errorResp.Error)
		}
		return "", fmt.Errorf("Ollama API returned status %d: %s", resp.StatusCode, string(body))
	}

	var response OllamaResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	// 清理响应文本
	cleanedResponse := c.cleanResponse(response.Response)

	log.Printf("✅ Ollama响应生成成功，长度: %d", len(cleanedResponse))

	return cleanedResponse, nil
}

func (c *OllamaClient) cleanResponse(response string) string {
	// 移除停止词
	stopWords := []string{"</s>", "<|endoftext|>", "<|im_end|>", "Human:", "Assistant:"}
	cleaned := response

	for _, stopWord := range stopWords {
		if idx := strings.Index(cleaned, stopWord); idx != -1 {
			cleaned = cleaned[:idx]
		}
	}

	// 清理空白字符
	cleaned = strings.TrimSpace(cleaned)

	return cleaned
}

// GetModelInfo 获取模型信息
func (c *OllamaClient) GetModelInfo() map[string]interface{} {
	return map[string]interface{}{
		"provider":     "ollama",
		"base_url":     c.baseURL,
		"model_name":   c.config.ModelPath,
		"temperature":  c.config.Temperature,
		"top_p":        c.config.TopP,
		"top_k":        c.config.TopK,
		"max_tokens":   c.config.MaxTokens,
		"timeout":      c.config.Timeout,
		"context_size": c.config.ContextSize,
		"threads":      c.config.Threads,
		"gpu":          c.config.GPU,
	}
}

// ValidateService 验证服务状态
func (c *OllamaClient) ValidateService() error {
	return c.validateService()
}

// ListModels 列出可用模型
func (c *OllamaClient) ListModels() ([]string, error) {
	resp, err := c.client.Get(c.baseURL + "/api/tags")
	if err != nil {
		return nil, fmt.Errorf("failed to get models: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get models, status: %d", resp.StatusCode)
	}

	var result struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	var models []string
	for _, model := range result.Models {
		models = append(models, model.Name)
	}

	return models, nil
}

// PullModel 拉取模型
func (c *OllamaClient) PullModel(modelName string) error {
	request := map[string]string{
		"name": modelName,
	}

	jsonData, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := c.client.Post(c.baseURL+"/api/pull", "application/json", bytes.NewReader(jsonData))
	if err != nil {
		return fmt.Errorf("failed to pull model: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to pull model, status: %d, body: %s", resp.StatusCode, string(body))
	}

	log.Printf("✅ 模型 %s 拉取成功", modelName)
	return nil
}

// GetModelStatus 获取模型状态
func (c *OllamaClient) GetModelStatus(modelName string) (map[string]interface{}, error) {
	resp, err := c.client.Get(fmt.Sprintf("%s/api/show", c.baseURL))
	if err != nil {
		return nil, fmt.Errorf("failed to get model status: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get model status, status: %d", resp.StatusCode)
	}

	var result map[string]interface{}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return result, nil
}
