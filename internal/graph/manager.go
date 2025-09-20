package graph

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"qng-agent/internal/config"
	"qng-agent/internal/llm"
	"qng-agent/internal/mcp"
	"qng-agent/internal/storage"
	"qng-agent/internal/types"
	"regexp"
	"strings"
	"time"

	"github.com/Qitmeer/qng/graph"
	"github.com/tmc/langchaingo/llms"
)

// LLMGraphManager manages the LLM graph workflow using official QNG graph library
type LLMGraphManager struct {
	llmClient llm.Client
	config    *config.Config
	storage   storage.Storage
	mcpClient *mcp.Client
}

// NewLLMGraphManager creates a new LLM graph manager
func NewLLMGraphManager(llmClient llm.Client, cfg *config.Config, storage storage.Storage) *LLMGraphManager {
	mcpClient := mcp.NewClient()

	// Initialize MCP client with configured servers
	mcpClient.UpdateServersFromConfig(cfg.MCP.DefaultServers)

	// Also load user settings if available
	if settings, err := storage.LoadSettings(); err == nil && settings.MCPServers != nil {
		mcpClient.UpdateServers(settings.MCPServers)
	}

	return &LLMGraphManager{
		llmClient: llmClient,
		config:    cfg,
		storage:   storage,
		mcpClient: mcpClient,
	}
}

// ProcessUserMessage processes a user message through the official QNG graph
func (m *LLMGraphManager) ProcessUserMessage(ctx context.Context, userMessage string, history []types.ChatMessage) (string, bool, error) {
	// Create a new message graph using official QNG graph library
	messageGraph := graph.NewMessageGraph()

	// Add nodes to the graph
	messageGraph.AddNode("intent_analysis", m.intentAnalysisNode)
	messageGraph.AddNode("mcp_tool_execution", m.mcpToolExecutionNode)
	messageGraph.AddNode("sub_workflow_execution", m.subWorkflowExecutionNode) // New node for sub-workflows
	messageGraph.AddNode("web3_workflow_execution", m.web3WorkflowExecutionNode)
	messageGraph.AddNode("response_generation", m.responseGenerationNode)

	// Set entry point
	messageGraph.SetEntryPoint("intent_analysis")

	// Add conditional edges based on intent
	messageGraph.AddConditionalEdge("intent_analysis", m.routeAfterIntent)

	// Add direct edges from execution nodes to response generation
	messageGraph.AddEdge("mcp_tool_execution", "response_generation")
	messageGraph.AddEdge("sub_workflow_execution", "response_generation") // New edge
	messageGraph.AddEdge("web3_workflow_execution", "response_generation")

	// Set response_generation as terminal node
	messageGraph.AddEdge("response_generation", graph.END)

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

// intentAnalysisNode analyzes user intent using LLM and generates sub-workflows for multi-task requests
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

	// Use LLM to analyze intent and determine if sub-workflow is needed
	prompt := m.buildIntentAnalysisPrompt(userMessage)

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
				Parameters: m.extractParametersFromMessage(userMessage),
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

	// If LLM determined this is a sub-workflow, try to use LLM-generated workflow first
	if result.Intent == "sub_workflow" {
		// First check if LLM provided a complete sub_workflow structure
		if result.SubWorkflow != nil {
			log.Printf("Using LLM-generated sub-workflow: %+v", result.SubWorkflow)
			// Ensure all tasks have proper TaskType set
			for i := range result.SubWorkflow.Tasks {
				if result.SubWorkflow.Tasks[i].TaskType == "" {
					// Default to mcp_tool for tasks with tool_name
					if result.SubWorkflow.Tasks[i].ToolName != "" {
						result.SubWorkflow.Tasks[i].TaskType = "mcp_tool"
					}
				}
			}
		} else if result.MultiTask {
			// Fallback: generate using legacy method if LLM didn't provide sub_workflow
			subWorkflow := m.generateSubWorkflowFromAnalysis(userMessage, &result)
			if subWorkflow != nil {
				result.SubWorkflow = subWorkflow
				log.Printf("Generated sub-workflow using legacy method: %+v", result.SubWorkflow)
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

// generateSubWorkflowFromAnalysis generates a sub-workflow based on LLM analysis result
func (m *LLMGraphManager) generateSubWorkflowFromAnalysis(userMessage string, intentResult *types.IntentAnalysisResult) *types.SubWorkflow {
	// Extract information from LLM analysis
	rpcURLs := m.extractRPCURLsFromIntentResult(intentResult)
	if len(rpcURLs) == 0 {
		// Fallback: extract URLs from message directly
		rpcURLs = m.extractRPCURLs(userMessage)
	}

	if len(rpcURLs) < 2 {
		log.Printf("Not enough RPC URLs for sub-workflow: %v", rpcURLs)
		return nil
	}

	// Determine operation type and tool from LLM analysis or message content
	operation, toolName := m.determineOperationFromAnalysis(intentResult, userMessage)
	if toolName == "" {
		log.Printf("Could not determine tool for sub-workflow")
		return nil
	}

	// Extract common parameters
	parameters := m.extractParametersFromAnalysis(intentResult, userMessage)

	// Generate sub-workflow with dynamic naming based on operation
	operationDisplayName := m.getOperationDisplayName(operation)
	subWorkflow := &types.SubWorkflow{
		ID:                  fmt.Sprintf("%s_workflow_%d", operation, time.Now().Unix()),
		Name:                fmt.Sprintf("Multi-RPC %s Operation", operationDisplayName),
		Description:         fmt.Sprintf("Execute %s operation across %d RPC endpoints", operationDisplayName, len(rpcURLs)),
		ExecutionMode:       "sequential", // Default to sequential for better logging and debugging
		AggregationStrategy: "compare",    // Default to comparison for multi-RPC operations
		Tasks:               make([]types.TaskExecution, 0, len(rpcURLs)),
	}

	// Create tasks for each RPC URL
	for i, rpcURL := range rpcURLs {
		taskParams := make(map[string]interface{})
		// Copy common parameters
		for k, v := range parameters {
			taskParams[k] = v
		}
		// Add RPC-specific parameter
		taskParams["rpc"] = rpcURL

		task := types.TaskExecution{
			ID:          fmt.Sprintf("%s_task_%d", operation, i+1),
			TaskType:    "mcp_tool",
			ToolName:    toolName,
			Parameters:  taskParams,
			RPC:         rpcURL,
			Order:       i + 1,
			Description: fmt.Sprintf("Execute %s on %s", operation, rpcURL),
		}
		subWorkflow.Tasks = append(subWorkflow.Tasks, task)
	}

	return subWorkflow
}

// extractRPCURLsFromIntentResult extracts RPC URLs from LLM intent analysis result
func (m *LLMGraphManager) extractRPCURLsFromIntentResult(intentResult *types.IntentAnalysisResult) []string {
	// Check if LLM provided rpc_urls in the analysis
	if intentResult.Parameters != nil {
		if rpcURLsInterface, exists := intentResult.Parameters["rpc_urls"]; exists {
			if rpcURLsArray, ok := rpcURLsInterface.([]interface{}); ok {
				var urls []string
				for _, url := range rpcURLsArray {
					if urlStr, ok := url.(string); ok {
						urls = append(urls, urlStr)
					}
				}
				return urls
			}
		}
	}
	return []string{}
}

// determineOperationFromAnalysis determines the operation type and tool name from LLM analysis
func (m *LLMGraphManager) determineOperationFromAnalysis(intentResult *types.IntentAnalysisResult, userMessage string) (string, string) {
	// First check if LLM provided task_parameters with operation
	if intentResult.Parameters != nil {
		if taskParams, exists := intentResult.Parameters["task_parameters"]; exists {
			if taskParamsMap, ok := taskParams.(map[string]interface{}); ok {
				if operation, exists := taskParamsMap["operation"]; exists {
					if operationStr, ok := operation.(string); ok {
						toolName := m.getToolForOperation(operationStr)
						return operationStr, toolName
					}
				}
			}
		}
	}

	// Fallback: analyze message content to determine operation
	message := strings.ToLower(userMessage)
	if strings.Contains(message, "stateroot") {
		return "stateroot", "get_block_stateroot"
	}
	if strings.Contains(message, "区块总数") || strings.Contains(message, "区块总量") || strings.Contains(message, "block count") || strings.Contains(message, "block_count") {
		return "block_count", "get_block_count"
	}
	if strings.Contains(message, "block") && !strings.Contains(message, "stateroot") && !strings.Contains(message, "count") {
		return "block", "get_block_by_order"
	}
	if strings.Contains(message, "balance") || strings.Contains(message, "余额") {
		return "balance", "get_balance"
	}

	// Default fallback - try to infer from available context
	log.Printf("Could not determine operation from message: %s", userMessage)
	return "unknown", ""
}

// getToolForOperation maps operation names to MCP tool names
func (m *LLMGraphManager) getToolForOperation(operation string) string {
	switch strings.ToLower(operation) {
	case "stateroot":
		return "get_block_stateroot"
	case "block_count", "count":
		return "get_block_count"
	case "block":
		return "get_block_by_order"
	case "balance":
		return "get_balance"
	default:
		log.Printf("Unknown operation: %s", operation)
		return "" // Return empty instead of default
	}
}

// getOperationDisplayName returns a user-friendly display name for operations
func (m *LLMGraphManager) getOperationDisplayName(operation string) string {
	switch strings.ToLower(operation) {
	case "stateroot":
		return "StateRoot Query"
	case "block_count", "count":
		return "Block Count Query"
	case "block":
		return "Block Data Query"
	case "balance":
		return "Balance Query"
	default:
		return strings.Title(operation) + " Query"
	}
}

// extractParametersFromAnalysis extracts common parameters for all tasks
func (m *LLMGraphManager) extractParametersFromAnalysis(intentResult *types.IntentAnalysisResult, userMessage string) map[string]interface{} {
	parameters := make(map[string]interface{})

	// First check LLM analysis results
	if intentResult.Parameters != nil {
		if taskParams, exists := intentResult.Parameters["task_parameters"]; exists {
			if taskParamsMap, ok := taskParams.(map[string]interface{}); ok {
				for k, v := range taskParamsMap {
					if k != "operation" { // Don't include operation as a parameter
						parameters[k] = v
					}
				}
			}
		}
	}

	// Fallback: extract from message content
	if len(parameters) == 0 {
		// Extract order if present
		if order := m.extractOrderFromMessage(userMessage); order > 0 {
			parameters["order"] = order
		}
	}

	return parameters
}

// extractParametersFromMessage extracts parameters from user message (fallback method)
func (m *LLMGraphManager) extractParametersFromMessage(userMessage string) map[string]interface{} {
	parameters := make(map[string]interface{})

	// Extract RPC URL
	rpcURLs := m.extractRPCURLs(userMessage)
	if len(rpcURLs) > 0 {
		parameters["rpc"] = rpcURLs[0] // Use first RPC URL for single tool calls
	}

	// Extract order if present
	if order := m.extractOrderFromMessage(userMessage); order > 0 {
		parameters["order"] = order
	}

	// Extract address if present
	if address := m.extractAddress(userMessage); address != "" {
		parameters["address"] = address
	}

	return parameters
}

// extractRPCURLs extracts all HTTP RPC URLs from the user message
func (m *LLMGraphManager) extractRPCURLs(message string) []string {
	// Regular expression to match HTTP URLs
	urlRegex := regexp.MustCompile(`https?://[^\s]+`)
	matches := urlRegex.FindAllString(message, -1)

	var rpcURLs []string
	for _, match := range matches {
		// Clean up URL (remove trailing punctuation)
		url := strings.TrimRight(match, ".,;!?")
		// Add trailing slash if not present and ends with port
		if strings.Contains(url, ":") && !strings.HasSuffix(url, "/") {
			url += "/"
		}
		rpcURLs = append(rpcURLs, url)
	}

	return rpcURLs
}

// extractOrderFromMessage extracts order number from user message
func (m *LLMGraphManager) extractOrderFromMessage(message string) int {
	// Look for "order" followed by number
	orderRegex := regexp.MustCompile(`order\s+(\d+)`)
	matches := orderRegex.FindStringSubmatch(strings.ToLower(message))
	if len(matches) > 1 {
		var order int
		fmt.Sscanf(matches[1], "%d", &order)
		return order
	}
	return 0
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

// mcpToolExecutionNode executes MCP tools via real MCP server calls
func (m *LLMGraphManager) mcpToolExecutionNode(ctx context.Context, state []llms.MessageContent, options graph.Options) ([]llms.MessageContent, error) {
	// Extract intent result from state
	var intentResult types.IntentAnalysisResult

	for _, msg := range state {
		// if msg.Role == llms.ChatMessageTypeHuman {
		// 	for _, part := range msg.Parts {
		// 		if textPart, ok := part.(llms.TextContent); ok {
		// 			userMessage = textPart.Text
		// 			break
		// 		}
		// 	}
		// }

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

	// Prepare parameters for MCP tool call
	parameters := intentResult.Parameters
	if parameters == nil {
		parameters = make(map[string]interface{})
	}

	log.Printf("Calling MCP tool '%s' with parameters: %+v", intentResult.MCPTool, parameters)

	// Call the real MCP server
	result, err := m.mcpClient.CallTool(ctx, intentResult.MCPTool, parameters)
	if err != nil {
		// If MCP call fails, create error response
		errorResult := map[string]interface{}{
			"error":  err.Error(),
			"tool":   intentResult.MCPTool,
			"status": "failed",
		}
		resultData, _ := json.Marshal(errorResult)

		log.Printf("MCP tool execution failed: %v", err)

		// Add error result to state
		toolMessage := llms.MessageContent{
			Role: llms.ChatMessageTypeSystem,
			Parts: []llms.ContentPart{
				llms.TextPart(fmt.Sprintf("TOOL_RESULT:%s", string(resultData))),
			},
		}
		return append(state, toolMessage), nil
	}

	// Convert result to JSON string
	resultData, err := json.Marshal(result)
	if err != nil {
		resultData = []byte(fmt.Sprintf(`{"error": "Failed to marshal result", "raw_result": "%v"}`, result))
	}

	log.Printf("MCP tool execution successful: %s", string(resultData))

	// Add tool result to state
	toolMessage := llms.MessageContent{
		Role: llms.ChatMessageTypeSystem,
		Parts: []llms.ContentPart{
			llms.TextPart(fmt.Sprintf("TOOL_RESULT:%s", string(resultData))),
		},
	}

	return append(state, toolMessage), nil
}

// subWorkflowExecutionNode executes sub-workflows for multi-task requests
func (m *LLMGraphManager) subWorkflowExecutionNode(ctx context.Context, state []llms.MessageContent, options graph.Options) ([]llms.MessageContent, error) {
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

	if intentResult.SubWorkflow == nil {
		return state, fmt.Errorf("no sub-workflow found in intent result")
	}

	subWorkflow := intentResult.SubWorkflow
	log.Printf("Executing sub-workflow: %s with %d tasks", subWorkflow.Name, len(subWorkflow.Tasks))

	// Execute tasks based on execution mode
	var taskResults []types.TaskResult
	var err error

	startTime := time.Now()
	if subWorkflow.ExecutionMode == "parallel" {
		taskResults, err = m.executeTasksInParallel(ctx, subWorkflow.Tasks)
	} else {
		taskResults, err = m.executeTasksSequentially(ctx, subWorkflow.Tasks)
	}

	if err != nil {
		log.Printf("Sub-workflow execution failed: %v", err)
		// Create error result
		errorResult := types.SubWorkflowResult{
			WorkflowID:  subWorkflow.ID,
			Success:     false,
			TaskResults: taskResults,
			Summary:     fmt.Sprintf("Sub-workflow execution failed: %v", err),
			TotalTime:   time.Since(startTime),
		}

		resultData, _ := json.Marshal(errorResult)
		errorMessage := llms.MessageContent{
			Role: llms.ChatMessageTypeSystem,
			Parts: []llms.ContentPart{
				llms.TextPart(fmt.Sprintf("SUB_WORKFLOW_RESULT:%s", string(resultData))),
			},
		}
		return append(state, errorMessage), nil
	}

	// Aggregate results based on strategy
	summary, aggregatedSuccess := m.aggregateResults(taskResults, subWorkflow.AggregationStrategy)

	// Create sub-workflow result
	subWorkflowResult := types.SubWorkflowResult{
		WorkflowID:  subWorkflow.ID,
		Success:     aggregatedSuccess,
		TaskResults: taskResults,
		Summary:     summary,
		TotalTime:   time.Since(startTime),
	}

	log.Printf("Sub-workflow completed: %s, Success: %v, Total time: %v",
		subWorkflow.Name, aggregatedSuccess, subWorkflowResult.TotalTime)

	// Add sub-workflow result to state
	resultData, _ := json.Marshal(subWorkflowResult)
	resultMessage := llms.MessageContent{
		Role: llms.ChatMessageTypeSystem,
		Parts: []llms.ContentPart{
			llms.TextPart(fmt.Sprintf("SUB_WORKFLOW_RESULT:%s", string(resultData))),
		},
	}

	return append(state, resultMessage), nil
}

// executeTasksSequentially executes tasks one by one with dependency resolution
func (m *LLMGraphManager) executeTasksSequentially(ctx context.Context, tasks []types.TaskExecution) ([]types.TaskResult, error) {
	var results []types.TaskResult
	resultMap := make(map[string]map[string]interface{}) // taskID -> result data

	for _, task := range tasks {
		log.Printf("Executing task sequentially: %s (%s)", task.ID, task.Description)

		// Check for parameter dependencies and resolve them
		resolvedTask := task
		if len(task.DependsOn) > 0 {
			var err error
			resolvedTask, err = m.resolveDependentParameters(ctx, task, results, resultMap)
			if err != nil {
				log.Printf("Failed to resolve parameters for task %s: %v", task.ID, err)
				return results, fmt.Errorf("parameter resolution failed for task %s: %w", task.ID, err)
			}
		}

		startTime := time.Now()
		result, err := m.executeTask(ctx, resolvedTask)
		executionTime := time.Since(startTime)

		taskResult := types.TaskResult{
			TaskID:        task.ID,
			Success:       err == nil,
			ExecutionTime: executionTime,
			RPC:           task.RPC,
		}

		if err != nil {
			taskResult.Error = err.Error()
			log.Printf("Task %s failed: %v", task.ID, err)
		} else {
			taskResult.Result = result
			resultMap[task.ID] = result
			log.Printf("Task %s completed successfully in %v", task.ID, executionTime)
		}

		results = append(results, taskResult)
	}

	return results, nil
}

// resolveDependentParameters resolves parameters that depend on previous task results using LLM
func (m *LLMGraphManager) resolveDependentParameters(ctx context.Context, task types.TaskExecution, previousResults []types.TaskResult, resultMap map[string]map[string]interface{}) (types.TaskExecution, error) {
	log.Printf("Resolving dependent parameters for task %s, depends on: %v", task.ID, task.DependsOn)
	
	// Collect dependency results
	dependencyContext := make(map[string]interface{})
	for _, depTaskID := range task.DependsOn {
		if result, exists := resultMap[depTaskID]; exists {
			dependencyContext[depTaskID] = result
		}
	}
	
	// Use LLM to extract parameters from dependency results
	resolvedParameters, err := m.extractParametersWithLLM(ctx, task, dependencyContext)
	if err != nil {
		return task, fmt.Errorf("LLM parameter extraction failed: %w", err)
	}
	
	// Create resolved task with updated parameters
	resolvedTask := task
	resolvedTask.Parameters = resolvedParameters
	
	log.Printf("Task %s parameters resolved: %+v", task.ID, resolvedParameters)
	return resolvedTask, nil
}

// extractParametersWithLLM uses LLM to extract parameters from previous task results
func (m *LLMGraphManager) extractParametersWithLLM(ctx context.Context, task types.TaskExecution, dependencyResults map[string]interface{}) (map[string]interface{}, error) {
	// Build context information for LLM
	contextStr := ""
	for taskID, result := range dependencyResults {
		resultJSON, _ := json.Marshal(result)
		contextStr += fmt.Sprintf("Task %s result: %s\n", taskID, string(resultJSON))
	}
	
	// Create LLM prompt for parameter extraction
	prompt := fmt.Sprintf(`You are a blockchain data extraction assistant. 

CONTEXT: Previous task results:
%s

CURRENT TASK: %s (tool: %s)
Task Description: %s
Original Parameters: %s

YOUR TASK: Extract the required parameters for the current task from the previous task results.

INSTRUCTIONS:
1. Look at the previous task results above
2. Extract the specific data needed for the current task parameters
3. For blockchain queries, common parameter mappings:
   - block_count result → "order" parameter for stateroot/block queries
   - rpc URLs should be preserved from original parameters
   - other static parameters should be kept

RULES:
- If you see a block count number like 13078095 in previous results, use it as "order" parameter
- Always preserve "rpc" parameter from original task parameters
- Return ONLY a JSON object with the resolved parameters
- Do not include explanations, just the JSON

Expected JSON format:
{"rpc": "extracted_or_preserved_rpc_url", "order": extracted_block_number_or_other_params}`, 
		contextStr, 
		task.ID, 
		task.ToolName, 
		task.Description,
		fmt.Sprintf("%+v", task.Parameters))

	// Call LLM for parameter extraction
	chatMessages := []types.ChatMessage{
		{Role: "user", Content: prompt},
	}

	response, err := m.llmClient.GetCompletion(ctx, chatMessages)
	if err != nil {
		return nil, fmt.Errorf("LLM completion failed: %w", err)
	}

	// Parse LLM response as JSON
	var extractedParams map[string]interface{}
	if err := json.Unmarshal([]byte(response), &extractedParams); err != nil {
		// Fallback: try to extract manually if LLM response is not valid JSON
		log.Printf("LLM response not valid JSON, falling back to manual extraction: %s", response)
		return m.fallbackParameterExtraction(task, dependencyResults)
	}
	
	return extractedParams, nil
}

// fallbackParameterExtraction provides a fallback mechanism for parameter extraction
func (m *LLMGraphManager) fallbackParameterExtraction(task types.TaskExecution, dependencyResults map[string]interface{}) (map[string]interface{}, error) {
	resolvedParams := make(map[string]interface{})
	
	// Copy original parameters first
	for k, v := range task.Parameters {
		resolvedParams[k] = v
	}
	
	// Try to extract block count from dependency results
	for _, result := range dependencyResults {
		if resultMap, ok := result.(map[string]interface{}); ok {
			if content, exists := resultMap["content"]; exists {
				if contentArray, ok := content.([]interface{}); ok && len(contentArray) > 0 {
					if contentItem, ok := contentArray[0].(map[string]interface{}); ok {
						if text, exists := contentItem["text"]; exists {
							if textStr, ok := text.(string); ok {
								// Try to extract block number from JSON response
								var jsonResp map[string]interface{}
								if err := json.Unmarshal([]byte(textStr), &jsonResp); err == nil {
									if result, exists := jsonResp["result"]; exists {
										if blockCount, ok := result.(float64); ok {
											resolvedParams["order"] = int(blockCount)
											log.Printf("Extracted block count %d for task %s", int(blockCount), task.ID)
											return resolvedParams, nil
										}
									}
								}
							}
						}
					}
				}
			}
		}
	}
	
	return resolvedParams, nil
}

// executeTasksInParallel executes tasks concurrently
func (m *LLMGraphManager) executeTasksInParallel(ctx context.Context, tasks []types.TaskExecution) ([]types.TaskResult, error) {
	var results []types.TaskResult
	resultsChan := make(chan types.TaskResult, len(tasks))
	errorsChan := make(chan error, len(tasks))

	// Start all tasks concurrently
	for _, task := range tasks {
		go func(t types.TaskExecution) {
			log.Printf("Executing task in parallel: %s (%s)", t.ID, t.Description)

			startTime := time.Now()
			result, err := m.executeTask(ctx, t)
			executionTime := time.Since(startTime)

			taskResult := types.TaskResult{
				TaskID:        t.ID,
				Success:       err == nil,
				ExecutionTime: executionTime,
				RPC:           t.RPC,
			}

			if err != nil {
				taskResult.Error = err.Error()
				log.Printf("Parallel task %s failed: %v", t.ID, err)
				errorsChan <- err
			} else {
				taskResult.Result = result
				log.Printf("Parallel task %s completed successfully in %v", t.ID, executionTime)
			}

			resultsChan <- taskResult
		}(task)
	}

	// Collect all results
	for i := 0; i < len(tasks); i++ {
		result := <-resultsChan
		results = append(results, result)
	}

	// Check for any errors
	select {
	case err := <-errorsChan:
		return results, err
	default:
		return results, nil
	}
}

// executeTask executes a single task based on its type
func (m *LLMGraphManager) executeTask(ctx context.Context, task types.TaskExecution) (map[string]interface{}, error) {
	switch task.TaskType {
	case "mcp_tool":
		return m.executeMCPTask(ctx, task)
	case "web3_workflow":
		return m.executeWeb3Task(ctx, task)
	default:
		return nil, fmt.Errorf("unsupported task type: %s", task.TaskType)
	}
}

// executeMCPTask executes an MCP tool task
func (m *LLMGraphManager) executeMCPTask(ctx context.Context, task types.TaskExecution) (map[string]interface{}, error) {
	log.Printf("Calling MCP tool '%s' with parameters: %+v for RPC: %s",
		task.ToolName, task.Parameters, task.RPC)

	// Call the MCP tool with the specific parameters
	result, err := m.mcpClient.CallTool(ctx, task.ToolName, task.Parameters)
	if err != nil {
		return nil, fmt.Errorf("MCP tool call failed for %s: %w", task.ToolName, err)
	}

	// Add RPC information to the result for tracking
	if result == nil {
		result = make(map[string]interface{})
	}
	result["_rpc"] = task.RPC
	result["_task_id"] = task.ID

	return result, nil
}

// executeWeb3Task executes a Web3 workflow task (placeholder)
func (m *LLMGraphManager) executeWeb3Task(ctx context.Context, task types.TaskExecution) (map[string]interface{}, error) {
	// This is a placeholder for Web3 task execution
	return map[string]interface{}{
		"status":    "not_implemented",
		"task_type": "web3_workflow",
		"message":   "Web3 workflow execution not yet implemented",
	}, nil
}

// aggregateResults combines task results based on the aggregation strategy
func (m *LLMGraphManager) aggregateResults(results []types.TaskResult, strategy string) (string, bool) {
	if len(results) == 0 {
		return "No tasks were executed", false
	}

	successCount := 0
	var summaryParts []string

	for _, result := range results {
		if result.Success {
			successCount++
		}

		// Build summary for each result
		rpcInfo := ""
		if result.RPC != "" {
			rpcInfo = fmt.Sprintf(" (RPC: %s)", result.RPC)
		}

		if result.Success {
			if result.Result != nil {
				// Let LLM handle data extraction and formatting - just pass raw result
				log.Printf("Task result (raw for LLM): %+v", result.Result)
				summaryParts = append(summaryParts,
					fmt.Sprintf("✅ %s%s: Raw result: %+v (took %v)",
						result.TaskID, rpcInfo, result.Result, result.ExecutionTime))
			} else {
				log.Printf("Task result is nil")
				summaryParts = append(summaryParts,
					fmt.Sprintf("✅ %s%s: Success but no data (took %v)",
						result.TaskID, rpcInfo, result.ExecutionTime))
			}
		} else {
			summaryParts = append(summaryParts,
				fmt.Sprintf("❌ %s%s: Failed - %s (took %v)",
					result.TaskID, rpcInfo, result.Error, result.ExecutionTime))
		}
	}

	overallSuccess := successCount > 0

	// Create strategy-specific summary
	switch strategy {
	case "compare":
		header := fmt.Sprintf("Comparison Results (%d/%d tasks successful):\n", successCount, len(results))
		if successCount > 1 {
			header += "Multiple endpoints queried - comparing results:\n"
		}
		return header + strings.Join(summaryParts, "\n"), overallSuccess

	case "summarize":
		header := fmt.Sprintf("Summary (%d/%d tasks successful):\n", successCount, len(results))
		return header + strings.Join(summaryParts, "\n"), overallSuccess

	case "merge":
		header := fmt.Sprintf("Merged Results (%d/%d tasks successful):\n", successCount, len(results))
		return header + strings.Join(summaryParts, "\n"), overallSuccess

	default:
		header := fmt.Sprintf("Results (%d/%d tasks successful):\n", successCount, len(results))
		return header + strings.Join(summaryParts, "\n"), overallSuccess
	}
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

// extractAddress extracts wallet/contract address from user message
func (m *LLMGraphManager) extractAddress(message string) string {
	// Look for 0x prefixed addresses
	if idx := strings.Index(message, "0x"); idx != -1 {
		start := idx
		end := start + 2
		for end < len(message) && ((message[end] >= '0' && message[end] <= '9') ||
			(message[end] >= 'a' && message[end] <= 'f') ||
			(message[end] >= 'A' && message[end] <= 'F')) {
			end++
		}
		if end > start+2 {
			return message[start:end]
		}
	}
	return ""
}

// extractToken extracts token symbol from user message
func (m *LLMGraphManager) extractToken(message string) string {
	message = strings.ToLower(message)
	tokens := []string{"meer", "usdt", "btc", "eth", "qng"}
	for _, token := range tokens {
		if strings.Contains(message, token) {
			return strings.ToUpper(token)
		}
	}
	return "MEER" // Default token
}

// extractBlockID extracts block height or hash from user message
func (m *LLMGraphManager) extractBlockID(message string) string {
	// Look for "block" followed by number or hash
	if strings.Contains(message, "block") {
		words := strings.Fields(message)
		for i, word := range words {
			if strings.Contains(word, "block") && i+1 < len(words) {
				next := words[i+1]
				// Check if it's a number (block height) or hash
				if len(next) > 0 && (next[0] >= '0' && next[0] <= '9') {
					return next
				}
				if strings.HasPrefix(next, "0x") {
					return next
				}
			}
		}
	}
	return "latest"
}

// extractTxHash extracts transaction hash from user message
func (m *LLMGraphManager) extractTxHash(message string) string {
	// Look for transaction hash patterns
	if strings.Contains(message, "tx") || strings.Contains(message, "transaction") {
		if idx := strings.Index(message, "0x"); idx != -1 {
			start := idx
			end := start + 2
			for end < len(message) && ((message[end] >= '0' && message[end] <= '9') ||
				(message[end] >= 'a' && message[end] <= 'f') ||
				(message[end] >= 'A' && message[end] <= 'F')) {
				end++
			}
			if end > start+2 && end-start >= 10 { // At least reasonable hash length
				return message[start:end]
			}
		}
	}
	return ""
}

// extractSymbol extracts token symbol for price queries
func (m *LLMGraphManager) extractSymbol(message string) string {
	message = strings.ToLower(message)
	symbols := []string{"meer", "btc", "eth", "usdt", "usdc", "qng"}
	for _, symbol := range symbols {
		if strings.Contains(message, symbol) {
			return strings.ToUpper(symbol)
		}
	}
	return "MEER" // Default symbol
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
	var subWorkflowResult string

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
					} else if strings.HasPrefix(content, "SUB_WORKFLOW_RESULT:") {
						subWorkflowResult = strings.TrimPrefix(content, "SUB_WORKFLOW_RESULT:")
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
	if subWorkflowResult != "" {
		// Parse sub-workflow result to provide better context
		var subResult types.SubWorkflowResult
		if err := json.Unmarshal([]byte(subWorkflowResult), &subResult); err == nil {
			log.Printf("SubWorkflow result for LLM: %s", subResult.Summary)
			dataContext += fmt.Sprintf("Sub-workflow execution result: %s\n", subResult.Summary)
			dataContext += fmt.Sprintf("Total execution time: %v\n", subResult.TotalTime)
			dataContext += fmt.Sprintf("Tasks completed: %d/%d successful\n",
				len(subResult.TaskResults), len(subResult.TaskResults))
		} else {
			log.Printf("SubWorkflow result (raw) for LLM: %s", subWorkflowResult)
			dataContext += fmt.Sprintf("Sub-workflow execution result: %s\n", subWorkflowResult)
		}
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
- If authentication is required, explain what the user needs to do next
- If multiple RPC endpoints were queried, compare and explain the differences or similarities
- For sub-workflow results, summarize the findings from multiple tasks clearly

LANGUAGE CONSISTENCY REQUIREMENT:
- CRITICAL: Analyze the user's question language (Chinese, English, etc.)
- Respond in the SAME language that the user used in their question
- If user asked in Chinese (中文), respond entirely in Chinese
- If user asked in English, respond entirely in English
- If user mixed languages, prioritize the primary language used
- Keep technical terms consistent with the user's language preference
- This language consistency rule overrides all other formatting preferences

FORMATTING AND PRESENTATION GUIDELINES:
- Use proper Markdown formatting for better readability
- Create tables using Markdown syntax when comparing multiple data points
- Use emojis and formatting to make the response engaging
- Structure information with headers (##, ###) for organization
- Use bullet points and numbered lists for clarity
- Add visual separators (---) between sections
- For comparisons, use side-by-side tables or clear comparisons
- Highlight important values with **bold** or backtick code blocks

Example of good table formatting:
| Field | Node 1 Value | Node 2 Value | Status |
|-------|-------------|-------------|---------|
| StateRoot | 0x123... | 0x123... | ✅ Match |
| Response Time | 36ms | 33ms | ✅ Both fast |

Example of good section structure:
## 🔍 **Analysis Results**
### ✅ Summary
### 📊 Detailed Comparison
### 💡 What This Means

CRITICAL DATA EXTRACTION INSTRUCTIONS:
- The raw results may contain JSON-RPC responses like: {"jsonrpc":"2.0","id":1,"result":13077047}
- Extract the actual values from "result" fields in JSON-RPC responses
- If you see content arrays with text fields, parse the JSON inside the text
- Present the extracted numbers in a user-friendly format with commas (e.g., 13,077,047)
- Compare values across different RPC endpoints using tables when appropriate
- Do NOT invent numbers - extract them from the provided raw data
- If you cannot extract clear numbers, say so and show the raw data

For blockchain data presentation:
- Use tables for comparing multiple nodes/endpoints
- Add visual indicators (✅❌🔍💡) for status and emphasis
- Explain technical terms in simple language
- Provide context about what the data means
- Include response times and performance metrics when available`, userMessage, intentResult.Intent, intentResult.Confidence, dataContext)

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
	case "sub_workflow":
		return "sub_workflow_execution"
	case "web3_workflow":
		return "web3_workflow_execution"
	default:
		return "response_generation"
	}
}

// buildIntentAnalysisPrompt constructs a dynamic prompt with current MCP tools and workflows
func (m *LLMGraphManager) buildIntentAnalysisPrompt(userMessage string) string {
	// Get available MCP tools from configuration and settings
	mcpTools := m.getAvailableMCPTools()
	web3Workflows := m.getAvailableWorkflows()

	prompt := fmt.Sprintf(`Analyze the user's intent in the following message:
"%s"

You are a QNG blockchain agent. Available MCP Tools:
%s

Available Web3 Workflows:
%s

ANALYSIS PROCESS:
1. TOOL IDENTIFICATION: First, identify ALL tools that need to be executed to fulfill the user's request
2. DEPENDENCY ANALYSIS: Then, analyze which tools depend on outputs from other tools  
3. PARAMETER EXTRACTION: Extract all parameters from user message and identify which ones are static vs dynamic (from previous tool outputs)
4. WORKFLOW CONSTRUCTION: Build the execution plan with proper sequencing

DECISION LOGIC:
- Single tool needed → use "mcp_tool"
- Multiple tools needed → use "sub_workflow"
- Pre-defined workflow exists → use "web3_workflow"
- General conversation → use "general_conversation"

WORKFLOW ANALYSIS STEPS:
Step 1: List all required tools
Step 2: Identify parameter dependencies between tools
Step 3: Determine execution order based on dependencies
Step 4: Extract static parameters from user input
Step 5: Define dynamic parameter mappings between tools

EXAMPLES (as reference, don't hardcode these patterns):

Example 1: "查询 http://rpc1/ 最新区块数"
- Tools needed: [get_block_count]
- Dependencies: none
- Result: mcp_tool

Example 2: "查询 http://rpc1/ 与 http://rpc2/ 最新区块数" 
- Tools needed: [get_block_count, get_block_count]
- Dependencies: none (parallel execution)
- Result: sub_workflow with parallel tasks

Example 3: "查询 http://rpc1/ 与 http://rpc2/ 对应最新区块数对应的stateroot"
- Tools needed: [get_block_count, get_block_count, get_block_stateroot, get_block_stateroot]
- Dependencies: stateroot tools depend on block_count results
- Parameter flow: block_count_result → stateroot_order_parameter
- Result: sub_workflow with sequential dependencies

YOUR TASK:
Analyze the user request following these steps:
1. Identify what tools are needed to complete the request
2. Determine if any tool needs output from another tool as input
3. Extract static parameters from user message
4. Design parameter flow between dependent tools
5. Choose appropriate intent based on complexity

Respond with JSON:
{
  "intent": "mcp_tool|sub_workflow|web3_workflow|general_conversation",
  "confidence": 0.0-1.0,
  "analysis": {
    "tools_needed": ["list", "of", "required", "tools"],
    "dependencies": [
      {"tool": "tool_name", "depends_on": ["previous_tool"], "parameter_mapping": {"output_field": "input_parameter"}}
    ],
    "static_parameters": {"extracted": "from_user_message"},
    "execution_strategy": "parallel|sequential|mixed"
  },
  "mcp_tool": "tool_name_if_single_tool",
  "parameters": {"for": "single_tool_execution"},
  "workflow_name": "name_if_predefined_workflow",
  "sub_workflow": {
    "id": "generated_workflow_id", 
    "name": "descriptive_name_based_on_user_request",
    "description": "what_this_accomplishes",
    "execution_mode": "sequential|parallel|mixed",
    "aggregation_strategy": "compare|summarize|merge|raw",
    "tasks": [
      {
        "id": "task_id",
        "tool_name": "identified_tool",
        "parameters": {"static_param": "value", "dynamic_param": "{{previous_task_output}}"},
        "depends_on": ["task_ids_this_depends_on"],
        "output_mapping": {"tool_output_field": "variable_name_for_next_task"}
      }
    ]
  },
  "rpc_urls": ["extracted_from_message"],
  "analysis_request": "what_analysis_user_wants"
}

CRITICAL INSTRUCTIONS:
- DON'T hardcode workflow patterns - analyze each request dynamically
- ALWAYS identify the minimal set of tools needed
- CAREFULLY trace parameter dependencies between tools
- Use {{variable_name}} for parameters that come from previous tool outputs
- Design tasks based on actual tool requirements, not predefined templates
- Consider user's intent for result analysis (comparison, summarization, etc.)`, userMessage, mcpTools, web3Workflows)

	log.Printf("Intent analysis prompt: %s", prompt)
	return prompt
}

// getAvailableMCPTools returns a formatted string of available MCP tools
func (m *LLMGraphManager) getAvailableMCPTools() string {
	var toolDescriptions []string

	// Get MCP servers from configuration
	for _, server := range m.config.MCP.DefaultServers {
		if server.Enabled {
			tools := m.getMCPServerTools(server.Name)
			for _, tool := range tools {
				toolDescriptions = append(toolDescriptions, tool)
			}
		}
	}

	// Get additional tools from user settings if available
	if settings, err := m.getUserSettings(); err == nil && settings.MCPServers != nil {
		for _, server := range settings.MCPServers {
			if server.Enabled {
				tools := m.getMCPServerTools(server.Name)
				for _, tool := range tools {
					toolDescriptions = append(toolDescriptions, tool)
				}
			}
		}
	}

	if len(toolDescriptions) == 0 {
		return "  - No MCP tools currently configured"
	}

	return strings.Join(toolDescriptions, "\n")
}

// getMCPServerTools returns tool descriptions for a specific MCP server
func (m *LLMGraphManager) getMCPServerTools(serverName string) []string {
	// Try to get tools dynamically from the MCP server
	if serverURL, exists := m.getServerURL(serverName); exists {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		tools, err := m.mcpClient.GetServerTools(ctx, serverURL)
		if err == nil && len(tools) > 0 {
			// log.Printf("Got tools from server %s: %v", serverName, tools)
			return m.formatToolsFromServer(tools)
		}

		log.Printf("Failed to get tools from server %s: %v, using fallback", serverName, err)
	}
	// Fallback to hardcoded tools if server is not accessible
	return m.getFallbackTools(serverName)
}

// getServerURL gets the URL for a server name
func (m *LLMGraphManager) getServerURL(serverName string) (string, bool) {
	// Check configuration servers
	for _, server := range m.config.MCP.DefaultServers {
		if server.Name == serverName && server.Enabled {
			return server.URL, true
		}
	}

	// Check user settings
	if settings, err := m.getUserSettings(); err == nil && settings.MCPServers != nil {
		for _, server := range settings.MCPServers {
			if server.Name == serverName && server.Enabled {
				return server.URL, true
			}
		}
	}

	return "", false
}

// formatToolsFromServer formats tools received from MCP server
func (m *LLMGraphManager) formatToolsFromServer(tools []map[string]interface{}) []string {
	var descriptions []string

	for _, tool := range tools {
		name, _ := tool["name"].(string)
		description, _ := tool["description"].(string)

		if name == "" {
			continue
		}

		// Format parameters if available
		var paramStr string
		if params, ok := tool["parameters"].(map[string]interface{}); ok && len(params) > 0 {
			var paramNames []string
			for paramName := range params {
				paramNames = append(paramNames, paramName)
			}
			if len(paramNames) > 0 {
				paramStr = fmt.Sprintf(" (params: %s)", strings.Join(paramNames, ", "))
			}
		}

		// Build description string
		if description != "" {
			descriptions = append(descriptions, fmt.Sprintf("  - %s: %s%s", name, description, paramStr))
		} else {
			descriptions = append(descriptions, fmt.Sprintf("  - %s%s", name, paramStr))
		}
	}

	return descriptions
}

// getFallbackTools returns hardcoded tools when server is not accessible
func (m *LLMGraphManager) getFallbackTools(serverName string) []string {
	switch serverName {
	case "QNG Tools":
		return []string{
			"  - stateroot: Query QNG node stateroot information (params: rpc, order)",
			"  - balance: Check account balance (params: address, token)",
			"  - node_status: Get QNG node status and health (params: rpc)",
			"  - block_info: Get block information by height or hash (params: rpc, block_id)",
			"  - transaction_info: Get transaction details (params: rpc, tx_hash)",
		}
	case "Metamask Tools":
		return []string{
			"  - wallet_connect: Connect to user's wallet",
			"  - wallet_balance: Get wallet token balances",
			"  - send_transaction: Send blockchain transaction (requires auth)",
			"  - sign_message: Sign arbitrary message (requires auth)",
		}
	case "Market Data":
		return []string{
			"  - token_price: Get current token price (params: symbol)",
			"  - market_cap: Get token market cap data (params: symbol)",
			"  - price_history: Get historical price data (params: symbol, timeframe)",
		}
	default:
		// Return generic tools based on server name patterns
		if strings.Contains(strings.ToLower(serverName), "qng") {
			return []string{
				"  - " + strings.ToLower(serverName) + "_query: Generic QNG node query tool",
			}
		}
		return []string{
			"  - " + strings.ToLower(serverName) + "_tool: Generic tool from " + serverName,
		}
	}
}

// getAvailableWorkflows returns a formatted string of available Web3 workflows
func (m *LLMGraphManager) getAvailableWorkflows() string {
	workflows := []string{
		"  - token_swap: Swap tokens using DEX smart contracts (MEER <-> USDT, etc.)",
		"  - staking: Stake tokens for rewards (requires wallet connection)",
		"  - liquidity_provision: Add/remove liquidity from pools",
		"  - governance_voting: Participate in DAO governance (requires auth)",
		"  - nft_trading: Buy/sell/transfer NFTs (requires auth)",
		"  - contract_deployment: Deploy smart contracts (requires auth)",
		"  - multi_sig_operations: Multi-signature wallet operations (requires auth)",
	}

	return strings.Join(workflows, "\n")
}

// getUserSettings retrieves user settings from storage
func (m *LLMGraphManager) getUserSettings() (*types.AppSettings, error) {
	// Use the LoadSettings method from the storage interface
	return m.storage.LoadSettings()
}

// min helper function
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
