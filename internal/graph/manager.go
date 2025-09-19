package graph

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"qng-agent/internal/llm"
	"qng-agent/internal/types"
	"strings"

	"github.com/Qitmeer/qng/graph"
	"github.com/tmc/langchaingo/llms"
)

// LLMGraphManager manages the LLM graph workflow using official QNG graph library
type LLMGraphManager struct {
	llmClient llm.Client
}

// NewLLMGraphManager creates a new LLM graph manager
func NewLLMGraphManager(llmClient llm.Client) *LLMGraphManager {
	return &LLMGraphManager{
		llmClient: llmClient,
	}
}

// ProcessUserMessage processes a user message through the official QNG graph
func (m *LLMGraphManager) ProcessUserMessage(ctx context.Context, userMessage string, history []types.ChatMessage) (string, bool, error) {
	// Create a new message graph using official QNG graph library
	messageGraph := graph.NewMessageGraph()

	// Add nodes to the graph
	messageGraph.AddNode("intent_analysis", m.intentAnalysisNode)
	messageGraph.AddNode("mcp_tool_execution", m.mcpToolExecutionNode)
	messageGraph.AddNode("web3_workflow_execution", m.web3WorkflowExecutionNode)
	messageGraph.AddNode("response_generation", m.responseGenerationNode)

	// Set entry point
	messageGraph.SetEntryPoint("intent_analysis")

	// Add conditional edges based on intent
	messageGraph.AddConditionalEdge("intent_analysis", m.routeAfterIntent)

	// Add direct edges from execution nodes to response generation
	messageGraph.AddEdge("mcp_tool_execution", "response_generation")
	messageGraph.AddEdge("web3_workflow_execution", "response_generation")

	// Compile the graph
	runnable, err := messageGraph.Compile()
	if err != nil {
		return "", false, fmt.Errorf("failed to compile graph: %w", err)
	}

	// Prepare initial messages using llms.MessageContent format
	initialMessages := []llms.MessageContent{
		{
			Role: llms.ChatMessageTypeSystem,
			Parts: []llms.ContentPart{
				llms.TextPart("You are a QNG blockchain agent. Process user requests through graph workflow."),
			},
		},
		{
			Role: llms.ChatMessageTypeHuman,
			Parts: []llms.ContentPart{
				llms.TextPart(userMessage),
			},
		},
	}

	// Add history context as system message
	if len(history) > 0 {
		historyText := "Previous conversation context:\n"
		for _, msg := range history {
			historyText += fmt.Sprintf("%s: %s\n", msg.Role, msg.Content)
		}
		
		contextMessage := llms.MessageContent{
			Role: llms.ChatMessageTypeSystem,
			Parts: []llms.ContentPart{
				llms.TextPart(historyText),
			},
		}
		initialMessages = append([]llms.MessageContent{contextMessage}, initialMessages...)
	}

	// Execute the graph
	result, err := runnable.Invoke(ctx, initialMessages)
	if err != nil {
		return "", false, fmt.Errorf("graph execution failed: %w", err)
	}

	// Extract final response from result messages
	finalResponse := ""
	needsAuth := false
	
	for _, msg := range result {
		for _, part := range msg.Parts {
			if textPart, ok := part.(llms.TextContent); ok {
				content := textPart.Text
				if strings.Contains(content, "FINAL_RESPONSE:") {
					finalResponse = strings.TrimPrefix(content, "FINAL_RESPONSE:")
				}
				if strings.Contains(content, "NEEDS_AUTH:true") {
					needsAuth = true
				}
			}
		}
	}

	if finalResponse == "" {
		return "I encountered an error processing your request.", false, nil
	}

	return finalResponse, needsAuth, nil
}

// intentAnalysisNode analyzes user intent using LLM
func (m *LLMGraphManager) intentAnalysisNode(ctx context.Context, state []llms.MessageContent, options graph.Options) ([]llms.MessageContent, error) {
	// Extract user message from state
	userMessage := ""
	for _, msg := range state {
		if msg.Role == llms.ChatMessageTypeHuman {
			for _, part := range msg.Parts {
				if textPart, ok := part.(llms.TextContent); ok {
					userMessage = textPart.Text
					break
				}
			}
			break
		}
	}

	if userMessage == "" {
		return state, fmt.Errorf("user message not found in state")
	}

	prompt := fmt.Sprintf(`Analyze the user's intent in the following message:
"%s"

You are a QNG blockchain agent. Determine if this message requires:
1. MCP Tool execution (like stateroot queries, balance checks, node status)
2. Web3 workflow execution (like token swaps, staking operations, contract interactions)  
3. General information or conversation

Respond with a JSON object containing:
{
  "intent": "mcp_tool|web3_workflow|general_conversation",
  "confidence": 0.0-1.0,
  "mcp_tool": "tool_name_if_applicable", 
  "parameters": {"param1": "value1"},
  "workflow_name": "workflow_name_if_applicable",
  "requires_auth": true/false
}`, userMessage)

	chatMessages := []types.ChatMessage{
		{Role: "user", Content: prompt},
	}

	response, err := m.llmClient.GetCompletion(ctx, chatMessages)
	if err != nil {
		return state, fmt.Errorf("LLM completion failed: %w", err)
	}

	// Parse the JSON response
	var result types.IntentAnalysisResult
	if err := json.Unmarshal([]byte(response), &result); err != nil {
		// Fallback: try to extract intent from response text
		response = strings.ToLower(response)
		if strings.Contains(response, "stateroot") || strings.Contains(response, "node") || strings.Contains(response, "balance") {
			result = types.IntentAnalysisResult{
				Intent:     "mcp_tool",
				Confidence: 0.8,
				MCPTool:    m.extractToolFromMessage(userMessage),
			}
		} else if strings.Contains(response, "swap") || strings.Contains(response, "stake") || strings.Contains(response, "workflow") {
			result = types.IntentAnalysisResult{
				Intent:       "web3_workflow",
				Confidence:   0.8,
				WorkflowName: "token_swap",
			}
		} else {
			result = types.IntentAnalysisResult{
				Intent:     "general_conversation",
				Confidence: 0.9,
			}
		}
	}

	log.Printf("Intent analysis result: %+v", result)

	// Add intent result to state as a system message
	intentData, _ := json.Marshal(result)
	intentMessage := llms.MessageContent{
		Role: llms.ChatMessageTypeSystem,
		Parts: []llms.ContentPart{
			llms.TextPart(fmt.Sprintf("INTENT_RESULT:%s", string(intentData))),
		},
	}

	return append(state, intentMessage), nil
}

// extractToolFromMessage extracts likely MCP tool from user message
func (m *LLMGraphManager) extractToolFromMessage(message string) string {
	message = strings.ToLower(message)
	if strings.Contains(message, "stateroot") {
		return "stateroot"
	}
	if strings.Contains(message, "balance") {
		return "balance"
	}
	if strings.Contains(message, "status") || strings.Contains(message, "node") {
		return "node_status"
	}
	return "unknown"
}

// mcpToolExecutionNode executes MCP tools
func (m *LLMGraphManager) mcpToolExecutionNode(ctx context.Context, state []llms.MessageContent, options graph.Options) ([]llms.MessageContent, error) {
	// Extract intent result from state
	var intentResult types.IntentAnalysisResult
	var userMessage string
	
	for _, msg := range state {
		if msg.Role == llms.ChatMessageTypeHuman {
			for _, part := range msg.Parts {
				if textPart, ok := part.(llms.TextContent); ok {
					userMessage = textPart.Text
					break
				}
			}
		}
		
		if msg.Role == llms.ChatMessageTypeSystem {
			for _, part := range msg.Parts {
				if textPart, ok := part.(llms.TextContent); ok {
					content := textPart.Text
					if strings.HasPrefix(content, "INTENT_RESULT:") {
						intentData := strings.TrimPrefix(content, "INTENT_RESULT:")
						json.Unmarshal([]byte(intentData), &intentResult)
						break
					}
				}
			}
		}
	}

	// Simulate MCP tool execution based on requirements
	var result string
	switch intentResult.MCPTool {
	case "stateroot":
		// Extract parameters from user message as specified in requirements
		node := m.extractNodeURL(userMessage)
		order := m.extractOrder(userMessage)
		
		result = fmt.Sprintf(`{
			"stateroot": "0x%x",
			"order": %d,
			"node": "%s",
			"timestamp": "2024-01-01T12:00:00Z",
			"block_height": %d
		}`, 
			0x1234567890abcdef+order, // Simulate different stateroot based on order
			order,
			node,
			10000+order,
		)
	case "balance":
		result = `{
			"balance": "1000.5",
			"token": "MEER",
			"address": "0xabcdef123456", 
			"decimals": 18
		}`
	case "node_status":
		result = `{
			"status": "online",
			"version": "2.1.0",
			"peers": 25,
			"height": 150000,
			"sync_progress": 100.0
		}`
	default:
		result = fmt.Sprintf(`{"error": "Unknown tool: %s"}`, intentResult.MCPTool)
	}

	log.Printf("MCP tool execution result: %s", result)

	// Add tool result to state
	toolMessage := llms.MessageContent{
		Role: llms.ChatMessageTypeSystem,
		Parts: []llms.ContentPart{
			llms.TextPart(fmt.Sprintf("TOOL_RESULT:%s", result)),
		},
	}

	return append(state, toolMessage), nil
}

// extractNodeURL extracts node URL from user message (as per requirements)
func (m *LLMGraphManager) extractNodeURL(message string) string {
	// Simple extraction logic for http URLs as specified in requirements
	if strings.Contains(message, "http://") {
		start := strings.Index(message, "http://")
		end := start + 7 // "http://"
		for end < len(message) && (message[end] != ' ' && message[end] != '?' && message[end] != ',' && message[end] != '.') {
			end++
		}
		if end < len(message) && message[end] == ':' {
			// Include port
			end++
			for end < len(message) && message[end] >= '0' && message[end] <= '9' {
				end++
			}
		}
		return message[start:end]
	}
	return "http://127.0.0.1:8545" // Default as per requirements
}

// extractOrder extracts order parameter from user message (as per requirements)
func (m *LLMGraphManager) extractOrder(message string) int {
	// Look for "order=NUMBER" as specified in requirements
	if idx := strings.Index(message, "order="); idx != -1 {
		start := idx + 6
		end := start
		for end < len(message) && message[end] >= '0' && message[end] <= '9' {
			end++
		}
		if end > start {
			var order int
			fmt.Sscanf(message[start:end], "%d", &order)
			return order
		}
	}
	return 10000 // Default as per requirements
}

// web3WorkflowExecutionNode executes Web3 workflows as per requirements
func (m *LLMGraphManager) web3WorkflowExecutionNode(ctx context.Context, state []llms.MessageContent, options graph.Options) ([]llms.MessageContent, error) {
	// Extract intent result from state
	var intentResult types.IntentAnalysisResult
	
	for _, msg := range state {
		if msg.Role == llms.ChatMessageTypeSystem {
			for _, part := range msg.Parts {
				if textPart, ok := part.(llms.TextContent); ok {
					content := textPart.Text
					if strings.HasPrefix(content, "INTENT_RESULT:") {
						intentData := strings.TrimPrefix(content, "INTENT_RESULT:")
						json.Unmarshal([]byte(intentData), &intentResult)
						break
					}
				}
			}
		}
	}

	// Load and execute workflow based on workflow name as per requirements
	workflow := m.getWorkflowConfig(intentResult.WorkflowName)
	if workflow == nil {
		return state, fmt.Errorf("workflow %s not found", intentResult.WorkflowName)
	}

	// Simulate workflow execution as specified in requirements (wallet connection, etc.)
	result := map[string]interface{}{
		"workflow":    intentResult.WorkflowName,
		"status":      "requires_user_confirmation",
		"message":     "Please connect your wallet to continue with the operation",
		"next_step":   "wallet_connection",
		"workflow_id": fmt.Sprintf("wf_%s_%d", intentResult.WorkflowName, len(workflow.Nodes)),
	}

	resultData, _ := json.Marshal(result)
	log.Printf("Web3 workflow execution result: %s", string(resultData))

	// Add workflow result to state
	workflowMessage := llms.MessageContent{
		Role: llms.ChatMessageTypeSystem,
		Parts: []llms.ContentPart{
			llms.TextPart(fmt.Sprintf("WORKFLOW_RESULT:%s", string(resultData))),
			llms.TextPart("NEEDS_AUTH:true"),
		},
	}

	return append(state, workflowMessage), nil
}

// getWorkflowConfig returns a predefined workflow configuration as per requirements
func (m *LLMGraphManager) getWorkflowConfig(name string) *types.WorkflowConfig {
	switch name {
	case "token_swap":
		// Token swap workflow as specified in requirements (MEER to USDT example)
		return &types.WorkflowConfig{
			Name:        "Token Swap Workflow",
			Description: "Swap tokens using smart contracts",
			Nodes: []types.WorkflowNode{
				{ID: "wallet_connect", Type: "wallet", Name: "Connect Wallet"},
				{ID: "balance_check", Type: "contract_read", Name: "Check Balance"},
				{ID: "approve_token", Type: "contract_write", Name: "Approve Token"},
				{ID: "swap_execute", Type: "contract_write", Name: "Execute Swap"},
			},
			Edges: []types.WorkflowEdge{
				{Source: "wallet_connect", Target: "balance_check"},
				{Source: "balance_check", Target: "approve_token"},
				{Source: "approve_token", Target: "swap_execute"},
			},
		}
	case "staking":
		return &types.WorkflowConfig{
			Name:        "Token Staking Workflow",
			Description: "Stake tokens for rewards",
			Nodes: []types.WorkflowNode{
				{ID: "wallet_connect", Type: "wallet", Name: "Connect Wallet"},
				{ID: "balance_check", Type: "contract_read", Name: "Check Balance"},
				{ID: "stake_execute", Type: "contract_write", Name: "Execute Staking"},
			},
		}
	default:
		return nil
	}
}

// responseGenerationNode generates natural language response
func (m *LLMGraphManager) responseGenerationNode(ctx context.Context, state []llms.MessageContent, options graph.Options) ([]llms.MessageContent, error) {
	// Extract data from state
	var userMessage string
	var intentResult types.IntentAnalysisResult
	var toolResult string
	var workflowResult string
	
	for _, msg := range state {
		if msg.Role == llms.ChatMessageTypeHuman {
			for _, part := range msg.Parts {
				if textPart, ok := part.(llms.TextContent); ok {
					userMessage = textPart.Text
					break
				}
			}
		}
		
		if msg.Role == llms.ChatMessageTypeSystem {
			for _, part := range msg.Parts {
				if textPart, ok := part.(llms.TextContent); ok {
					content := textPart.Text
					if strings.HasPrefix(content, "INTENT_RESULT:") {
						intentData := strings.TrimPrefix(content, "INTENT_RESULT:")
						json.Unmarshal([]byte(intentData), &intentResult)
					} else if strings.HasPrefix(content, "TOOL_RESULT:") {
						toolResult = strings.TrimPrefix(content, "TOOL_RESULT:")
					} else if strings.HasPrefix(content, "WORKFLOW_RESULT:") {
						workflowResult = strings.TrimPrefix(content, "WORKFLOW_RESULT:")
					}
				}
			}
		}
	}

	var dataContext string
	if toolResult != "" {
		dataContext = fmt.Sprintf("Tool execution result: %s\n", toolResult)
	}
	if workflowResult != "" {
		dataContext += fmt.Sprintf("Workflow execution result: %s\n", workflowResult)
	}

	prompt := fmt.Sprintf(`You are a helpful QNG blockchain agent assistant.

User asked: "%s"

Intent analysis determined: %s (confidence: %.2f)

%s

Please provide a clear, natural language response to the user. Guidelines:
- Be helpful and informative
- If there are tool results, explain them clearly  
- If there are workflow steps, guide the user through them
- For blockchain data, explain what it means
- Use simple language that both technical and non-technical users can understand
- If authentication is required, explain what the user needs to do next`, 
		userMessage,
		intentResult.Intent,
		intentResult.Confidence,
		dataContext)

	chatMessages := []types.ChatMessage{
		{Role: "user", Content: prompt},
	}

	response, err := m.llmClient.GetCompletion(ctx, chatMessages)
	if err != nil {
		return state, fmt.Errorf("LLM completion failed: %w", err)
	}

	log.Printf("Generated response: %s", response[:min(len(response), 100)]+"...")

	// Add final response to state
	responseMessage := llms.MessageContent{
		Role: llms.ChatMessageTypeSystem,
		Parts: []llms.ContentPart{
			llms.TextPart(fmt.Sprintf("FINAL_RESPONSE:%s", response)),
		},
	}

	return append(state, responseMessage), nil
}

// routeAfterIntent determines the next node after intent analysis
func (m *LLMGraphManager) routeAfterIntent(ctx context.Context, state []llms.MessageContent, options graph.Options) string {
	// Extract intent result from state
	var intentResult types.IntentAnalysisResult
	
	for _, msg := range state {
		if msg.Role == llms.ChatMessageTypeSystem {
			for _, part := range msg.Parts {
				if textPart, ok := part.(llms.TextContent); ok {
					content := textPart.Text
					if strings.HasPrefix(content, "INTENT_RESULT:") {
						intentData := strings.TrimPrefix(content, "INTENT_RESULT:")
						json.Unmarshal([]byte(intentData), &intentResult)
						break
					}
				}
			}
		}
	}

	log.Printf("Routing based on intent: %s", intentResult.Intent)

	switch intentResult.Intent {
	case "mcp_tool":
		return "mcp_tool_execution"
	case "web3_workflow":
		return "web3_workflow_execution"
	default:
		return "response_generation"
	}
}

// min helper function
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}