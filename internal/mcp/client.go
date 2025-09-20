package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"qng-agent/internal/config"
	"qng-agent/internal/types"
	"strings"
	"sync"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
)

// Client represents an MCP client for communicating with MCP servers
type Client struct {
	httpClient    *http.Client
	servers       map[string]string // serverName -> URL mapping
	sessions      map[string]string // serverURL -> sessionID mapping for reuse
	sseConnections map[string]*http.Response // serverURL -> active SSE connection
	mutex         sync.RWMutex // Protect concurrent access to sessions and connections
	requestCounter int64        // Counter for unique request IDs
}

// NewClient creates a new MCP client
func NewClient() *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		servers:        make(map[string]string),
		sessions:       make(map[string]string),
		sseConnections: make(map[string]*http.Response),
		requestCounter: 0,
	}
}

// UpdateServers updates the list of MCP servers
func (c *Client) UpdateServers(servers []types.MCPServerConfig) {
	c.servers = make(map[string]string)
	for _, server := range servers {
		if server.Enabled {
			c.servers[server.Name] = server.URL
		}
	}
}

// UpdateServersFromConfig updates the list of MCP servers from config
func (c *Client) UpdateServersFromConfig(servers []config.MCPServerConfig) {
	c.servers = make(map[string]string)
	for _, server := range servers {
		if server.Enabled {
			c.servers[server.Name] = server.URL
		}
	}
}

// CallTool calls a specific tool on an MCP server using mcp-go types
func (c *Client) CallTool(ctx context.Context, toolName string, parameters map[string]interface{}) (map[string]interface{}, error) {
	// Find which server provides this tool
	serverURL, err := c.findServerForTool(toolName)
	if err != nil {
		return nil, err
	}

	// Check if this is an SSE endpoint
	if strings.Contains(serverURL, "/sse") {
		return c.callSSETool(ctx, serverURL, toolName, parameters)
	}

	// Standard MCP JSON-RPC call - not implemented yet for this use case
	return nil, fmt.Errorf("non-SSE MCP servers not yet supported")
}

// callSSETool calls a tool via SSE endpoint using improved MCP protocol handling with concurrency safety
func (c *Client) callSSETool(ctx context.Context, serverURL string, toolName string, parameters map[string]interface{}) (map[string]interface{}, error) {
	// Generate unique request ID with thread safety
	c.mutex.Lock()
	c.requestCounter++
	requestID := fmt.Sprintf("tool_call_%d_%d", time.Now().UnixNano(), c.requestCounter)
	c.mutex.Unlock()

	// Try up to 2 times: first with cached session, then with fresh session
	for attempt := 0; attempt < 2; attempt++ {
		// For MCP SSE protocol, we need to get session and use message endpoint
		_, messageEndpoint, err := c.initMCPSessionSafe(ctx, serverURL)
		if err != nil {
			log.Printf("Failed to initialize MCP session for tool call (attempt %d): %v", attempt+1, err)
			if attempt == 0 {
				// Clear cached session and try again
				c.mutex.Lock()
				delete(c.sessions, serverURL)
				c.mutex.Unlock()
				continue
			}
			return nil, fmt.Errorf("failed to initialize MCP session: %w", err)
		}

		// Use mcp-go structures for the tool call request
		var callToolRequest mcp.CallToolRequest
		callToolRequest.Method = "tools/call"
		callToolRequest.Params.Name = toolName
		callToolRequest.Params.Arguments = parameters

		// Marshal to JSON-RPC 2.0 format with unique request ID
		request := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      requestID,
			"method":  callToolRequest.Method,
			"params": map[string]interface{}{
				"name":      callToolRequest.Params.Name,
				"arguments": callToolRequest.Params.Arguments,
			},
		}

		requestBody, err := json.Marshal(request)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request: %w", err)
		}

		// Make request to message endpoint
		req, err := http.NewRequestWithContext(ctx, "POST", messageEndpoint, bytes.NewBuffer(requestBody))
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "text/event-stream")
		req.Header.Set("Cache-Control", "no-cache")
		req.Header.Set("User-Agent", "QNG-Agent/1.0")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("failed to call message endpoint: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
			body, _ := io.ReadAll(resp.Body)
			// If session is invalid, clear cache and retry
			if strings.Contains(string(body), "Invalid session ID") {
				sseURL := strings.Replace(messageEndpoint, "/message", "/sse", 1)
				if idx := strings.Index(sseURL, "?"); idx != -1 {
					sseURL = sseURL[:idx]
				}
				delete(c.sessions, sseURL)
				log.Printf("Session expired for tool call, cleared from cache")
				if attempt == 0 {
					continue
				}
			}
			return nil, fmt.Errorf("message endpoint returned status %d: %s", resp.StatusCode, string(body))
		}

		// Read response from the original SSE connection instead of the response body
		return c.readSSEToolCallResponse(requestID)
	}

	return nil, fmt.Errorf("failed to call tool after 2 attempts")
}

// readSSEToolCallResponse reads tool call response from the active SSE connection
func (c *Client) readSSEToolCallResponse(expectedID string) (map[string]interface{}, error) {
	// Find the active SSE connection for this session
	var sseConn *http.Response
	for _, conn := range c.sseConnections {
		if conn != nil {
			sseConn = conn
			break
		}
	}

	if sseConn == nil {
		return nil, fmt.Errorf("no active SSE connection found")
	}

	scanner := bufio.NewScanner(sseConn.Body)
	timeout := time.NewTimer(30 * time.Second)
	defer timeout.Stop()

	done := make(chan map[string]interface{})
	errChan := make(chan error)

	go func() {
		defer close(done)
		defer close(errChan)
		
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			
			// Parse SSE format: "data: {...}"
			if strings.HasPrefix(line, "data: ") {
				data := strings.TrimPrefix(line, "data: ")
				
				// Skip heartbeat and empty data
				if data == "" || data == "heartbeat" {
					continue
				}
				
				// Try to parse as JSON-RPC response
				var jsonRpcResponse map[string]interface{}
				if err := json.Unmarshal([]byte(data), &jsonRpcResponse); err == nil {
					// Check if this is our response (handle both string and number IDs)
					var responseID string
					if id, ok := jsonRpcResponse["id"].(string); ok {
						responseID = id
					} else if id, ok := jsonRpcResponse["id"].(float64); ok {
						responseID = fmt.Sprintf("%.0f", id)
					}
					
					if responseID == expectedID {
						// Check for error
						if errorField, ok := jsonRpcResponse["error"]; ok {
							errChan <- fmt.Errorf("MCP error: %v", errorField)
							return
						}
						
						// Extract result
						if result, ok := jsonRpcResponse["result"]; ok {
							log.Printf("MCP raw result: %v (type: %T)", result, result)
							// Convert result to map using mcp-go compatible format
							resultMap := c.convertMCPResult(result)
							log.Printf("MCP converted result: %+v", resultMap)
							done <- resultMap
							return
						}
						
						// Empty result
						done <- map[string]interface{}{}
						return
					}
				}
			}
		}
		
		if err := scanner.Err(); err != nil {
			errChan <- fmt.Errorf("error reading SSE stream: %w", err)
		}
	}()

	select {
	case result := <-done:
		return result, nil
	case err := <-errChan:
		return nil, err
	case <-timeout.C:
		return nil, fmt.Errorf("timeout waiting for tool call response")
	}
}

// convertMCPResult converts MCP result to our expected format
func (c *Client) convertMCPResult(result interface{}) map[string]interface{} {
	resultMap := make(map[string]interface{})
	
	// If result is already a map, use it directly
	if resultMap, ok := result.(map[string]interface{}); ok {
		return resultMap
	}
	
	// If result is not a map, wrap it
	resultMap["data"] = result
	return resultMap
}

// findServerForTool determines which MCP server provides a specific tool
func (c *Client) findServerForTool(toolName string) (string, error) {
	// Map tools to servers based on tool naming patterns
	switch {
	case toolName == "get_block_stateroot" || toolName == "get_block_by_order" || toolName == "get_block_count" || 
		 strings.HasPrefix(toolName, "get_") || strings.HasPrefix(toolName, "banlist") || 
		 strings.HasPrefix(toolName, "estimate_") || strings.HasPrefix(toolName, "tips") ||
		 strings.HasPrefix(toolName, "is_"):
		if url, ok := c.servers["QNG Tools"]; ok {
			return url, nil
		}
		// Fallback to any available server for QNG tools
		for serverName, url := range c.servers {
			if strings.Contains(strings.ToLower(serverName), "qng") {
				return url, nil
			}
		}
	case toolName == "wallet_connect" || toolName == "wallet_balance" || toolName == "send_transaction" || toolName == "sign_message":
		if url, ok := c.servers["Metamask Tools"]; ok {
			return url, nil
		}
	case toolName == "token_price" || toolName == "market_cap" || toolName == "price_history":
		if url, ok := c.servers["Market Data"]; ok {
			return url, nil
		}
	}

	// Try to find any server that might handle this tool
	for serverName, url := range c.servers {
		if serverName != "" {
			return url, nil
		}
	}

	return "", fmt.Errorf("no MCP server found for tool: %s", toolName)
}

// initMCPSessionSafe initializes MCP session with thread safety
func (c *Client) initMCPSessionSafe(ctx context.Context, sseURL string) (string, string, error) {
	// Check existing session with read lock
	c.mutex.RLock()
	if existingSessionID, exists := c.sessions[sseURL]; exists {
		if sseConn, connExists := c.sseConnections[sseURL]; connExists && sseConn != nil {
			messageEndpoint := strings.Replace(sseURL, "/sse", "/message", 1) + "?sessionId=" + existingSessionID
			c.mutex.RUnlock()
			return existingSessionID, messageEndpoint, nil
		}
	}
	c.mutex.RUnlock()

	// Acquire write lock for session creation
	c.mutex.Lock()
	defer c.mutex.Unlock()

	// Double-check pattern - another goroutine might have created the session
	if existingSessionID, exists := c.sessions[sseURL]; exists {
		if sseConn, connExists := c.sseConnections[sseURL]; connExists && sseConn != nil {
			messageEndpoint := strings.Replace(sseURL, "/sse", "/message", 1) + "?sessionId=" + existingSessionID
			return existingSessionID, messageEndpoint, nil
		}
	}

	// Clean up any stale connections
	if oldConn, exists := c.sseConnections[sseURL]; exists && oldConn != nil {
		oldConn.Body.Close()
		delete(c.sseConnections, sseURL)
		delete(c.sessions, sseURL)
	}

	// Call the existing initMCPSession method (unlock first to avoid deadlock)
	c.mutex.Unlock()
	sessionID, messageEndpoint, err := c.initMCPSession(ctx, sseURL)
	c.mutex.Lock()
	
	return sessionID, messageEndpoint, err
}

// initMCPSession initializes MCP session and returns session ID and message endpoint
func (c *Client) initMCPSession(ctx context.Context, sseURL string) (string, string, error) {
	// Check if we have an active SSE connection and session for this URL
	if existingSessionID, exists := c.sessions[sseURL]; exists {
		if sseConn, connExists := c.sseConnections[sseURL]; connExists && sseConn != nil {
			// Test if SSE connection is still alive by checking if we can read from it
			messageEndpoint := strings.Replace(sseURL, "/sse", "/message", 1) + "?sessionId=" + existingSessionID
			return existingSessionID, messageEndpoint, nil
		}
	}

	// Clean up any stale connections
	if oldConn, exists := c.sseConnections[sseURL]; exists && oldConn != nil {
		oldConn.Body.Close()
		delete(c.sseConnections, sseURL)
		delete(c.sessions, sseURL)
	}

	req, err := http.NewRequestWithContext(ctx, "GET", sseURL, nil)
	if err != nil {
		return "", "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("User-Agent", "QNG-Agent/1.0")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("failed to connect to SSE: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return "", "", fmt.Errorf("SSE returned status %d", resp.StatusCode)
	}

	// Read the first SSE message to get session info
	scanner := bufio.NewScanner(resp.Body)
	timeout := time.NewTimer(5 * time.Second)
	defer timeout.Stop()

	done := make(chan struct{})
	var sessionID, messageEndpoint string

	go func() {
		defer close(done)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			log.Printf("SSE received: %s", line)
			
			if strings.HasPrefix(line, "data: ") {
				data := strings.TrimPrefix(line, "data: ")
				
				// Look for message endpoint URL
				if strings.Contains(data, "/message?sessionId=") {
					messageEndpoint = data
					// Extract session ID from URL
					if idx := strings.Index(data, "sessionId="); idx != -1 {
						sessionID = data[idx+10:] // "sessionId=" is 10 chars
						// Remove any trailing parameters
						if ampIdx := strings.Index(sessionID, "&"); ampIdx != -1 {
							sessionID = sessionID[:ampIdx]
						}
						return
					}
				}
			}
		}
	}()

	select {
	case <-done:
		if sessionID != "" && messageEndpoint != "" {
			// Cache the session and KEEP the SSE connection alive
			c.sessions[sseURL] = sessionID
			c.sseConnections[sseURL] = resp
			log.Printf("Established active SSE connection for session: %s", sessionID)
			return sessionID, messageEndpoint, nil
		}
		resp.Body.Close()
		return "", "", fmt.Errorf("failed to get session ID from SSE stream")
	case <-timeout.C:
		resp.Body.Close()
		return "", "", fmt.Errorf("timeout waiting for session ID")
	}
}

// GetServerTools gets the list of available tools from a specific server
func (c *Client) GetServerTools(ctx context.Context, serverURL string) ([]map[string]interface{}, error) {
	// Check if this is an SSE endpoint
	if strings.Contains(serverURL, "/sse") {
		return c.getSSEServerTools(ctx, serverURL)
	}

	// Standard MCP tools/list call - not implemented yet for this use case
	return c.getDefaultTools(serverURL), nil
}

// getSSEServerTools gets tools from SSE endpoint using proper MCP protocol
func (c *Client) getSSEServerTools(ctx context.Context, serverURL string) ([]map[string]interface{}, error) {
	// Try up to 2 times: first with cached session, then with fresh session
	for attempt := 0; attempt < 2; attempt++ {
		// Step 1: Connect to SSE endpoint to get session ID
		sessionID, messageEndpoint, err := c.initMCPSession(ctx, serverURL)
		if err != nil {
			log.Printf("Failed to initialize MCP session (attempt %d): %v", attempt+1, err)
			if attempt == 0 {
				// Clear cached session and try again
				delete(c.sessions, serverURL)
				continue
			}
			return c.getDefaultTools(serverURL), nil
		}

		// Step 2: Use the message endpoint to call tools/list
		tools, err := c.callMCPToolsList(ctx, messageEndpoint, sessionID)
		if err != nil {
			log.Printf("Failed to call tools/list (attempt %d): %v", attempt+1, err)
			// If session was invalid, the cache was already cleared, retry once
			if attempt == 0 && strings.Contains(err.Error(), "Invalid session ID") {
				continue
			}
			return c.getDefaultTools(serverURL), nil
		}

		log.Printf("Successfully got %d tools from MCP server", len(tools))
		return tools, nil
	}

	// If both attempts failed, return default tools
	return c.getDefaultTools(serverURL), nil
}

// callMCPToolsList calls the MCP tools/list method and reads response from original SSE connection
func (c *Client) callMCPToolsList(ctx context.Context, messageEndpoint, sessionID string) ([]map[string]interface{}, error) {
	// Extract server URL to get the SSE connection
	sseURL := strings.Replace(messageEndpoint, "/message", "/sse", 1)
	if idx := strings.Index(sseURL, "?"); idx != -1 {
		sseURL = sseURL[:idx]
	}

	// Get the active SSE connection
	sseConn, exists := c.sseConnections[sseURL]
	if !exists || sseConn == nil {
		return nil, fmt.Errorf("no active SSE connection for %s", sseURL)
	}

	// Prepare MCP JSON-RPC request
	request := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      "tools_list_1",
		"method":  "tools/list",
		"params":  map[string]interface{}{},
	}

	requestBody, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Send request to message endpoint (will get 202 Accepted)
	req, err := http.NewRequestWithContext(ctx, "POST", messageEndpoint, bytes.NewBuffer(requestBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("User-Agent", "QNG-Agent/1.0")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call message endpoint: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		body, _ := io.ReadAll(resp.Body)
		// If session is invalid, clear cache and return error for retry with new session
		if strings.Contains(string(body), "Invalid session ID") {
			delete(c.sessions, sseURL)
			if sseConn != nil {
				sseConn.Body.Close()
			}
			delete(c.sseConnections, sseURL)
			log.Printf("Session expired for %s, cleared from cache", sseURL)
		}
		return nil, fmt.Errorf("message endpoint returned status %d: %s", resp.StatusCode, string(body))
	}

	log.Printf("Request sent, status: %d, now reading response from SSE connection...", resp.StatusCode)

	// Read response from the original SSE connection
	return c.readSSEResponseFromConnection(sseConn, "tools_list_1", 10*time.Second)
}

// readSSEResponseFromConnection reads JSON-RPC response from active SSE connection
func (c *Client) readSSEResponseFromConnection(sseConn *http.Response, expectedID string, timeout time.Duration) ([]map[string]interface{}, error) {
	scanner := bufio.NewScanner(sseConn.Body)
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	done := make(chan []map[string]interface{})
	errChan := make(chan error)

	go func() {
		defer close(done)
		defer close(errChan)

		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			log.Printf("SSE response: %s", line)

			// Parse SSE format: "data: {...}"
			if strings.HasPrefix(line, "data: ") {
				data := strings.TrimPrefix(line, "data: ")

				// Skip heartbeat and empty data
				if data == "" || data == "heartbeat" {
					continue
				}

				// Try to parse as JSON-RPC response
				var jsonRpcResponse map[string]interface{}
				if err := json.Unmarshal([]byte(data), &jsonRpcResponse); err == nil {
					// Check if this is our response
					if id, ok := jsonRpcResponse["id"].(string); ok && id == expectedID {
						// Check for error
						if errorField, ok := jsonRpcResponse["error"]; ok {
							errChan <- fmt.Errorf("MCP error: %v", errorField)
							return
						}

						// Extract tools from result
						if result, ok := jsonRpcResponse["result"].(map[string]interface{}); ok {
							if toolsField, ok := result["tools"].([]interface{}); ok {
								var tools []map[string]interface{}
								for _, tool := range toolsField {
									if toolMap, ok := tool.(map[string]interface{}); ok {
										tools = append(tools, toolMap)
									}
								}
								log.Printf("Successfully parsed %d tools from SSE response", len(tools))
								done <- tools
								return
							}
						}

						// No tools found, return empty
						done <- []map[string]interface{}{}
						return
					}
				}
			}
		}

		if err := scanner.Err(); err != nil {
			errChan <- fmt.Errorf("error reading SSE stream: %w", err)
		}
	}()

	select {
	case tools := <-done:
		return tools, nil
	case err := <-errChan:
		return nil, err
	case <-timer.C:
		return nil, fmt.Errorf("timeout waiting for JSON-RPC response")
	}
}

// getDefaultTools returns default tools when server doesn't provide tools list
func (c *Client) getDefaultTools(serverURL string) []map[string]interface{} {
	// Determine server type from URL
	if strings.Contains(serverURL, "/sse") || strings.Contains(strings.ToLower(serverURL), "qng") {
		return []map[string]interface{}{
			{
				"name":        "get_block_stateroot",
				"description": "Retrieves a qng block stateroot by its order and rpc",
				"inputSchema": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"order": map[string]interface{}{
							"description": "Order of the qng block stateroot to retrieve",
							"type":        "number",
						},
						"rpc": map[string]interface{}{
							"description": "Rpc of the qng block stateroot to retrieve",
							"type":        "string",
						},
					},
					"required": []string{"order", "rpc"},
				},
			},
			{
				"name":        "get_block_by_order",
				"description": "Retrieves a qng block by its order",
				"inputSchema": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"order": map[string]interface{}{
							"description": "Order of the qng block to retrieve",
							"type":        "number",
						},
					},
					"required": []string{"order"},
				},
			},
			{
				"name":        "get_block_count",
				"description": "Retrieves a qng block total count",
				"inputSchema": map[string]interface{}{
					"type":       "object",
					"properties": map[string]interface{}{},
				},
			},
		}
	}

	return []map[string]interface{}{
		{
			"name":        "generic_tool",
			"description": "Generic tool from " + serverURL,
			"inputSchema": map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
	}
}

