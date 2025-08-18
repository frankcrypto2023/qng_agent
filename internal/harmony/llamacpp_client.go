package harmony

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

// LlamaCppRequest llama.cpp API 请求结构
type LlamaCppRequest struct {
	Prompt                   string   `json:"prompt"`
	Stream                   bool     `json:"stream"`
	Temperature              float64  `json:"temperature,omitempty"`
	TopP                     float64  `json:"top_p,omitempty"`
	TopK                     int      `json:"top_k,omitempty"`
	RepeatPenalty            float64  `json:"repeat_penalty,omitempty"`
	MaxTokens                int      `json:"n_predict,omitempty"`
	Stop                     []string `json:"stop,omitempty"`
	Grammar                  string   `json:"grammar,omitempty"`
	Seed                     int      `json:"seed,omitempty"`
	Mirostat                 int      `json:"mirostat,omitempty"`
	MirostatTau              float64  `json:"mirostat_tau,omitempty"`
	MirostatEta              float64  `json:"mirostat_eta,omitempty"`
	TypicalP                 float64  `json:"typical_p,omitempty"`
	FrequencyPenalty         float64  `json:"frequency_penalty,omitempty"`
	PresencePenalty          float64  `json:"presence_penalty,omitempty"`
	PenaltyPrompt            string   `json:"penalty_prompt,omitempty"`
	PenaltyRepeat            float64  `json:"penalty_repeat,omitempty"`
	PenaltyPresent           float64  `json:"penalty_present,omitempty"`
	PenaltyFrequency         float64  `json:"penalty_frequency,omitempty"`
	PenaltyLastN             int      `json:"penalty_last_n,omitempty"`
	PenaltyRange             int      `json:"penalty_range,omitempty"`
	PenaltySlope             float64  `json:"penalty_slope,omitempty"`
	PenaltyWindow            int      `json:"penalty_window,omitempty"`
	PenaltyWindowLast        int      `json:"penalty_window_last,omitempty"`
	PenaltyWindowMin         int      `json:"penalty_window_min,omitempty"`
	PenaltyWindowMax         int      `json:"penalty_window_max,omitempty"`
	PenaltyWindowStep        int      `json:"penalty_window_step,omitempty"`
	PenaltyWindowType        int      `json:"penalty_window_type,omitempty"`
	PenaltyWindowSize        int      `json:"penalty_window_size,omitempty"`
	PenaltyWindowTemp        float64  `json:"penalty_window_temp,omitempty"`
	PenaltyWindowTopK        int      `json:"penalty_window_top_k,omitempty"`
	PenaltyWindowTopP        float64  `json:"penalty_window_top_p,omitempty"`
	PenaltyWindowRep         float64  `json:"penalty_window_rep,omitempty"`
	PenaltyWindowFreq        float64  `json:"penalty_window_freq,omitempty"`
	PenaltyWindowPres        float64  `json:"penalty_window_pres,omitempty"`
	PenaltyWindowLastN       int      `json:"penalty_window_last_n,omitempty"`
	PenaltyWindowRange       int      `json:"penalty_window_range,omitempty"`
	PenaltyWindowSlope       float64  `json:"penalty_window_slope,omitempty"`
	PenaltyWindowWindow      int      `json:"penalty_window_window,omitempty"`
	PenaltyWindowWindowLast  int      `json:"penalty_window_window_last,omitempty"`
	PenaltyWindowWindowMin   int      `json:"penalty_window_window_min,omitempty"`
	PenaltyWindowWindowMax   int      `json:"penalty_window_window_max,omitempty"`
	PenaltyWindowWindowStep  int      `json:"penalty_window_window_step,omitempty"`
	PenaltyWindowWindowType  int      `json:"penalty_window_window_type,omitempty"`
	PenaltyWindowWindowSize  int      `json:"penalty_window_window_size,omitempty"`
	PenaltyWindowWindowTemp  float64  `json:"penalty_window_window_temp,omitempty"`
	PenaltyWindowWindowTopK  int      `json:"penalty_window_window_top_k,omitempty"`
	PenaltyWindowWindowTopP  float64  `json:"penalty_window_window_top_p,omitempty"`
	PenaltyWindowWindowRep   float64  `json:"penalty_window_window_rep,omitempty"`
	PenaltyWindowWindowFreq  float64  `json:"penalty_window_window_freq,omitempty"`
	PenaltyWindowWindowPres  float64  `json:"penalty_window_window_pres,omitempty"`
	PenaltyWindowWindowLastN int      `json:"penalty_window_window_last_n,omitempty"`
	PenaltyWindowWindowRange int      `json:"penalty_window_window_range,omitempty"`
	PenaltyWindowWindowSlope float64  `json:"penalty_window_window_slope,omitempty"`
}

// LlamaCppResponse llama.cpp API 响应结构
type LlamaCppResponse struct {
	Content            string `json:"content"`
	GenerationSettings struct {
		FrequencyPenalty float64  `json:"frequency_penalty"`
		Grammar          string   `json:"grammar"`
		IgnoreEos        bool     `json:"ignore_eos"`
		LogitBias        []int    `json:"logit_bias"`
		Mirostat         int      `json:"mirostat"`
		MirostatEta      float64  `json:"mirostat_eta"`
		MirostatTau      float64  `json:"mirostat_tau"`
		Model            string   `json:"model"`
		NCtx             int      `json:"n_ctx"`
		NKeep            int      `json:"n_keep"`
		NPredict         int      `json:"n_predict"`
		NProbs           int      `json:"n_probs"`
		PenaltyLastN     int      `json:"penalty_last_n"`
		PenaltyPresent   float64  `json:"penalty_present"`
		PenaltyRepeat    float64  `json:"penalty_repeat"`
		PenaltyTemp      float64  `json:"penalty_temp"`
		RepeatLastN      int      `json:"repeat_last_n"`
		RepeatPenalty    float64  `json:"repeat_penalty"`
		Seed             int      `json:"seed"`
		Stop             []string `json:"stop"`
		Stream           bool     `json:"stream"`
		Temperature      float64  `json:"temperature"`
		TfsZ             float64  `json:"tfs_z"`
		TopK             int      `json:"top_k"`
		TopP             float64  `json:"top_p"`
		TypicalP         float64  `json:"typical_p"`
	} `json:"generation_settings"`
	Model              string `json:"model"`
	PromptEvalCount    int    `json:"prompt_eval_count"`
	PromptEvalDuration int64  `json:"prompt_eval_duration"`
	EvalCount          int    `json:"eval_count"`
	EvalDuration       int64  `json:"eval_duration"`
	Timings            struct {
		PredN  int   `json:"pred_n"`
		PredMS int64 `json:"pred_ms"`
		PromN  int   `json:"prom_n"`
		PromMS int64 `json:"prom_ms"`
	} `json:"timings"`
}

// LlamaCppStreamResponse 流式响应结构
type LlamaCppStreamResponse struct {
	Content string `json:"content"`
	Stop    bool   `json:"stop"`
	Timings struct {
		PredN  int   `json:"pred_n"`
		PredMS int64 `json:"pred_ms"`
		PromN  int   `json:"prom_n"`
		PromMS int64 `json:"prom_ms"`
	} `json:"timings"`
}

// LlamaCppHarmonyClient Harmony 格式的 llama.cpp 客户端
type LlamaCppHarmonyClient struct {
	baseURL    string
	httpClient *http.Client
	encoder    *HarmonyEncoder
}

// NewLlamaCppHarmonyClient 创建新的 Harmony llama.cpp 客户端
func NewLlamaCppHarmonyClient(baseURL string) *LlamaCppHarmonyClient {
	if baseURL == "" {
		baseURL = "http://localhost:8081"
	}

	return &LlamaCppHarmonyClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		encoder: NewHarmonyEncoder(),
	}
}

// Chat 发送对话请求
func (c *LlamaCppHarmonyClient) Chat(ctx context.Context, conv Conversation, options *ChatOptions) (*ChatResponse, error) {
	// 编码对话为 Harmony 格式
	prompt := c.encoder.EncodeConversation(conv)

	// 构建请求
	req := &LlamaCppRequest{
		Prompt:      prompt,
		Stream:      false,
		Temperature: 0.7,
		TopP:        0.9,
		MaxTokens:   2048,
		Stop:        []string{"<|end|>"},
	}

	// 应用选项
	if options != nil {
		if options.Temperature > 0 {
			req.Temperature = options.Temperature
		}
		if options.TopP > 0 {
			req.TopP = options.TopP
		}
		if options.MaxTokens > 0 {
			req.MaxTokens = options.MaxTokens
		}
		if len(options.Stop) > 0 {
			req.Stop = options.Stop
		}
	}

	// 发送请求
	resp, err := c.sendRequest(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	// 解析响应
	content := resp.Content
	if content == "" {
		return nil, fmt.Errorf("empty response from model")
	}

	// 解码助手消息
	assistantMsg, err := c.encoder.DecodeMessage("<|start|>assistant<|message|>" + content + "<|end|>")
	if err != nil {
		// 如果解码失败，直接使用原始内容
		assistantMsg = NewAssistantMessage(ChannelFinal, content)
	}

	return &ChatResponse{
		Content: assistantMsg,
		Usage: &Usage{
			PromptTokens:     resp.PromptEvalCount,
			CompletionTokens: resp.EvalCount,
			TotalTokens:      resp.PromptEvalCount + resp.EvalCount,
		},
	}, nil
}

// ChatStream 发送流式对话请求
func (c *LlamaCppHarmonyClient) ChatStream(ctx context.Context, conv Conversation, options *ChatOptions) (<-chan *ChatStreamResponse, error) {
	// 编码对话为 Harmony 格式
	prompt := c.encoder.EncodeConversation(conv)

	// 构建请求
	req := &LlamaCppRequest{
		Prompt:      prompt,
		Stream:      true,
		Temperature: 0.7,
		TopP:        0.9,
		MaxTokens:   2048,
		Stop:        []string{"<|end|>"},
	}

	// 应用选项
	if options != nil {
		if options.Temperature > 0 {
			req.Temperature = options.Temperature
		}
		if options.TopP > 0 {
			req.TopP = options.TopP
		}
		if options.MaxTokens > 0 {
			req.MaxTokens = options.MaxTokens
		}
		if len(options.Stop) > 0 {
			req.Stop = options.Stop
		}
	}

	// 发送流式请求
	resp, err := c.sendStreamRequest(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to send stream request: %w", err)
	}

	// 创建响应通道
	responseChan := make(chan *ChatStreamResponse)

			go func() {
			defer close(responseChan)
			defer resp.Body.Close()

			reader := bufio.NewReader(resp.Body)
			var fullContent strings.Builder
			chunkCount := 0

			log.Printf("🔄 开始读取流式响应...")

			for {
			select {
			case <-ctx.Done():
				return
			default:
				line, err := reader.ReadString('\n')
				if err != nil {
					log.Printf("❌ 读取流式数据失败: %v", err)
					if err == io.EOF {
						log.Printf("📄 到达文件末尾")
						// 发送最终响应
						if fullContent.Len() > 0 {
							content := fullContent.String()
							assistantMsg, _ := c.encoder.DecodeMessage("<|start|>assistant<|message|>" + content + "<|end|>")
							if assistantMsg.Content == nil {
								assistantMsg = NewAssistantMessage(ChannelFinal, content)
							}

							responseChan <- &ChatStreamResponse{
								Content: assistantMsg,
								Done:    true,
							}
						}
						return
					}
					responseChan <- &ChatStreamResponse{
						Error: fmt.Errorf("failed to read stream: %w", err),
						Done:  true,
					}
					return
				}

				line = strings.TrimSpace(line)
				if line == "" {
					continue
				}

				log.Printf("📝 收到原始行: %q", line)

				// 解析流式响应
				var streamResp LlamaCppStreamResponse
				if err := json.Unmarshal([]byte(line), &streamResp); err != nil {
					log.Printf("⚠️ JSON 解析失败: %v, 原始数据: %s", err, line)
					continue
				}

				chunkCount++

				if streamResp.Content != "" {
					fullContent.WriteString(streamResp.Content)

					// 发送增量响应 - 只发送增量内容，不包含完整的消息结构
					responseChan <- &ChatStreamResponse{
						Content: NewAssistantMessage(ChannelFinal, streamResp.Content),
						Done:    false,
					}
				}

				if streamResp.Stop {
					// 发送完成信号
					responseChan <- &ChatStreamResponse{
						Done: true,
					}
					return
				}
			}
		}
	}()

	return responseChan, nil
}

// sendRequest 发送普通请求
func (c *LlamaCppHarmonyClient) sendRequest(ctx context.Context, req *LlamaCppRequest) (*LlamaCppResponse, error) {
	jsonData, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/completion", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var llamaResp LlamaCppResponse
	if err := json.NewDecoder(resp.Body).Decode(&llamaResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &llamaResp, nil
}

// sendStreamRequest 发送流式请求
func (c *LlamaCppHarmonyClient) sendStreamRequest(ctx context.Context, req *LlamaCppRequest) (*http.Response, error) {
	jsonData, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/completion", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(body))
	}

	return resp, nil
}

// ChatOptions 聊天选项
type ChatOptions struct {
	Temperature float64  `json:"temperature,omitempty"`
	TopP        float64  `json:"top_p,omitempty"`
	MaxTokens   int      `json:"max_tokens,omitempty"`
	Stop        []string `json:"stop,omitempty"`
}

// ChatResponse 聊天响应
type ChatResponse struct {
	Content Message `json:"content"`
	Usage   *Usage  `json:"usage,omitempty"`
}

// ChatStreamResponse 流式聊天响应
type ChatStreamResponse struct {
	Content Message `json:"content,omitempty"`
	Error   error   `json:"error,omitempty"`
	Done    bool    `json:"done"`
}

// Usage 使用统计
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}
