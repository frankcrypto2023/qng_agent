package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"strings"
	"time"
)

// ClientInterface 定义MCP客户端接口
type ClientInterface interface {
	Call(ctx context.Context, method string, params map[string]any) (any, error)
	GetCapabilities() []string
	Close() error
}

// DetailedCapabilityClient 支持详细能力信息的MCP客户端接口
type DetailedCapabilityClient interface {
	ClientInterface
	GetDetailedCapabilities() []Capability
}

// SSEClient SSE协议客户端
type SSEClient struct {
	url          string
	timeout      time.Duration
	client       *http.Client
	capabilities []string
}

// StdioClient stdio协议客户端
type StdioClient struct {
	command      []string
	timeout      time.Duration
	cmd          *exec.Cmd
	stdin        io.WriteCloser
	stdout       io.ReadCloser
	capabilities []string
}

// NewSSEClient 创建SSE客户端
func NewSSEClient(url string, timeout int, capabilities []string) *SSEClient {
	return &SSEClient{
		url:          url,
		timeout:      time.Duration(timeout) * time.Second,
		capabilities: capabilities,
		client: &http.Client{
			Timeout: time.Duration(timeout) * time.Second,
		},
	}
}

// NewStdioClient 创建stdio客户端
func NewStdioClient(command []string, timeout int, capabilities []string) *StdioClient {
	return &StdioClient{
		command:      command,
		timeout:      time.Duration(timeout) * time.Second,
		capabilities: capabilities,
	}
}

// Call SSE客户端调用方法
func (c *SSEClient) Call(ctx context.Context, method string, params map[string]any) (any, error) {
	// 从URL中提取服务器名称
	serverName := "qng" // 默认值
	if strings.Contains(c.url, "qng") {
		serverName = "qng"
	} else if strings.Contains(c.url, "metamask") {
		serverName = "metamask"
	} else if strings.Contains(c.url, "file_system") {
		serverName = "file_system"
	}

	requestBody := map[string]any{
		"server": serverName,
		"method": method,
		"params": params,
	}

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.url+"/call", strings.NewReader(string(jsonData)))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP error: %d", resp.StatusCode)
	}

	var response map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if errorMsg, exists := response["error"]; exists {
		return nil, fmt.Errorf("MCP error: %v", errorMsg)
	}

	return response["result"], nil
}

// GetCapabilities SSE客户端获取能力
func (c *SSEClient) GetCapabilities() []string {
	return c.capabilities
}

// Close SSE客户端关闭
func (c *SSEClient) Close() error {
	// SSE客户端不需要特殊清理
	return nil
}

// Call stdio客户端调用方法
func (c *StdioClient) Call(ctx context.Context, method string, params map[string]any) (any, error) {
	if c.cmd == nil {
		if err := c.start(); err != nil {
			return nil, fmt.Errorf("failed to start stdio client: %w", err)
		}
	}

	request := map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  method,
		"params":  params,
	}

	jsonData, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// 发送请求
	_, err = c.stdin.Write(append(jsonData, '\n'))
	if err != nil {
		return nil, fmt.Errorf("failed to write to stdin: %w", err)
	}

	// 读取响应
	var response map[string]any
	if err := json.NewDecoder(c.stdout).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if errorMsg, exists := response["error"]; exists {
		return nil, fmt.Errorf("MCP error: %v", errorMsg)
	}

	return response["result"], nil
}

// GetCapabilities stdio客户端获取能力
func (c *StdioClient) GetCapabilities() []string {
	return c.capabilities
}

// Close stdio客户端关闭
func (c *StdioClient) Close() error {
	if c.cmd != nil && c.cmd.Process != nil {
		return c.cmd.Process.Kill()
	}
	return nil
}

// start 启动stdio进程
func (c *StdioClient) start() error {
	if len(c.command) == 0 {
		return fmt.Errorf("no command specified")
	}

	c.cmd = exec.Command(c.command[0], c.command[1:]...)

	var err error
	c.stdin, err = c.cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	c.stdout, err = c.cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	if err := c.cmd.Start(); err != nil {
		return fmt.Errorf("failed to start command: %w", err)
	}

	return nil
}
