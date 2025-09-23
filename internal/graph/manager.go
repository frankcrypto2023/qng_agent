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
	"strings"
	"time"

	"github.com/Qitmeer/qng/graph"
	"github.com/tmc/langchaingo/llms"
)

// LLMGraphManager manages the LLM graph workflow using official QNG graph library
type LLMGraphManager struct {
	llmClient  llm.Client
	llmManager *llm.Manager // Add LLM manager reference for configuration updates
	config     *config.Config
	storage    storage.Storage
	mcpClient  *mcp.Client
}

// NewLLMGraphManager creates a new LLM graph manager
func NewLLMGraphManager(llmClient llm.Client, llmManager *llm.Manager, cfg *config.Config, storage storage.Storage) *LLMGraphManager {
	mcpClient := mcp.NewClient()

	// Initialize MCP client with configured servers
	mcpClient.UpdateServersFromConfig(cfg.MCP.DefaultServers)

	// Also load user settings if available (use default user for global MCP settings)
	if settings, err := storage.LoadSettings("default"); err == nil && settings.MCPServers != nil {
		mcpClient.UpdateServers(settings.MCPServers)
	}

	return &LLMGraphManager{
		llmClient:  llmClient,
		llmManager: llmManager,
		config:     cfg,
		storage:    storage,
		mcpClient:  mcpClient,
	}
}

// ProcessUserMessage processes a user message through the official QNG graph
func (m *LLMGraphManager) ProcessUserMessage(ctx context.Context, userID, userMessage string, history []types.ChatMessage) (string, bool, error) {
	// Update LLM client and MCP client with user-specific configuration if available
	if userID != "" {
		if settings, err := m.getUserSettings(userID); err == nil {
			// Update LLM configuration if available
			if settings.LLMProvider.URL != "" {
				log.Printf("Updated LLM configuration for user %s: %+v", userID, settings.LLMProvider)
				// Update LLM manager with user's configuration
				m.llmManager.UpdateClientFromConfig(settings.LLMProvider)
				// Get the updated client
				m.llmClient = m.llmManager.GetClient()
			}

			// Update MCP servers if available
			if len(settings.MCPServers) > 0 {
				m.mcpClient.UpdateServers(settings.MCPServers)
				log.Printf("Updated MCP servers for user %s: %d servers configured", userID, len(settings.MCPServers))
			} else {
				// Fallback to default servers if user has no specific MCP configuration
				m.mcpClient.UpdateServersFromConfig(m.config.MCP.DefaultServers)
				log.Printf("Using default MCP servers for user %s", userID)
			}
		}
	}

	// Create a new message graph using official QNG graph library
	messageGraph := graph.NewGraph[map[string]interface{}, map[string]interface{}]()

	// Add nodes to the graph
	messageGraph.AddNode("intent_analysis", m.wrapIntentAnalysisNode)
	messageGraph.AddNode("mcp_tool_execution", m.wrapMcpToolExecutionNode)
	messageGraph.AddNode("sub_workflow_execution", m.wrapSubWorkflowExecutionNode) // New node for sub-workflows
	messageGraph.AddNode("web3_workflow_execution", m.wrapWeb3WorkflowExecutionNode)
	messageGraph.AddNode("response_generation", m.wrapResponseGenerationNode)

	// Set entry point
	messageGraph.SetEntryPoint("intent_analysis")

	// Add conditional edges based on intent
	messageGraph.AddConditionalEdge("intent_analysis", m.wrapRouteAfterIntent)

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
	state := graph.State{
		"messages": initialMessages,
	}
	result, err := runnable.Invoke(ctx, state)
	if err != nil {
		return "", false, fmt.Errorf("graph execution failed: %w", err)
	}

	// Extract final response from result messages
	finalResponse := ""
	needsAuth := false

	resultMessages, ok := result["messages"].([]llms.MessageContent)
	if !ok {
		return "", false, fmt.Errorf("invalid result: messages not found or wrong type")
	}

	for _, msg := range resultMessages {
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
func (m *LLMGraphManager) intentAnalysisNode(ctx context.Context, state []llms.MessageContent, options graph.Option) ([]llms.MessageContent, error) {
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
		// If JSON parsing fails, try to extract JSON from the response
		jsonStart := strings.Index(response, "{")
		jsonEnd := strings.LastIndex(response, "}")
		if jsonStart != -1 && jsonEnd != -1 && jsonEnd > jsonStart {
			jsonStr := response[jsonStart : jsonEnd+1]
			if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
				log.Printf("Failed to parse LLM response as JSON: %v", err)
				// Return a general conversation intent as fallback
				result = types.IntentAnalysisResult{
					Intent:     "general_conversation",
					Confidence: 0.5,
				}
			}
		} else {
			log.Printf("No valid JSON found in LLM response: %s", response)
			// Return a general conversation intent as fallback
			result = types.IntentAnalysisResult{
				Intent:     "general_conversation",
				Confidence: 0.5,
			}
		}
	}

	// If LLM determined this is a sub-workflow, try to use LLM-generated workflow first
	if result.Intent == "sub_workflow" {
		// First check if LLM provided a complete sub_workflow structure
		if result.SubWorkflow != nil {
			log.Printf("=== LLM GENERATED SUB-WORKFLOW DETAILS ===")
			log.Printf("Workflow ID: %s", result.SubWorkflow.ID)
			log.Printf("Workflow Name: %s", result.SubWorkflow.Name)
			log.Printf("Workflow Description: %s", result.SubWorkflow.Description)
			log.Printf("Execution Mode: %s", result.SubWorkflow.ExecutionMode)
			log.Printf("Aggregation Strategy: %s", result.SubWorkflow.AggregationStrategy)
			log.Printf("Number of Tasks: %d", len(result.SubWorkflow.Tasks))

			for i, task := range result.SubWorkflow.Tasks {
				log.Printf("--- Task %d ---", i+1)
				log.Printf("  Task ID: %s", task.ID)
				log.Printf("  Task Type: %s", task.TaskType)
				log.Printf("  Tool Name: %s", task.ToolName)
				log.Printf("  Description: %s", task.Description)
				log.Printf("  RPC: %s", task.RPC)
				log.Printf("  Depends On: %v", task.DependsOn)
				log.Printf("  Parameters: %+v", task.Parameters)
				// Note: OutputMapping field not available in TaskExecution type
			}
			log.Printf("=== END SUB-WORKFLOW DETAILS ===")

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
			log.Printf("LLM did not provide sub-workflow structure, generating using legacy method...")
			subWorkflow := m.generateSubWorkflowFromAnalysis(userMessage, &result)
			if subWorkflow != nil {
				result.SubWorkflow = subWorkflow
				log.Printf("=== GENERATED SUB-WORKFLOW DETAILS (LEGACY) ===")
				log.Printf("Workflow ID: %s", result.SubWorkflow.ID)
				log.Printf("Workflow Name: %s", result.SubWorkflow.Name)
				log.Printf("Workflow Description: %s", result.SubWorkflow.Description)
				log.Printf("Execution Mode: %s", result.SubWorkflow.ExecutionMode)
				log.Printf("Aggregation Strategy: %s", result.SubWorkflow.AggregationStrategy)
				log.Printf("Number of Tasks: %d", len(result.SubWorkflow.Tasks))

				for i, task := range result.SubWorkflow.Tasks {
					log.Printf("--- Task %d ---", i+1)
					log.Printf("  Task ID: %s", task.ID)
					log.Printf("  Task Type: %s", task.TaskType)
					log.Printf("  Tool Name: %s", task.ToolName)
					log.Printf("  Description: %s", task.Description)
					log.Printf("  RPC: %s", task.RPC)
					log.Printf("  Depends On: %v", task.DependsOn)
					log.Printf("  Parameters: %+v", task.Parameters)
					// Note: OutputMapping field not available in TaskExecution type
				}
				log.Printf("=== END GENERATED SUB-WORKFLOW DETAILS ===")
			} else {
				log.Printf("Failed to generate sub-workflow using legacy method")
			}
		} else {
			log.Printf("LLM determined sub-workflow intent but provided no sub-workflow structure and MultiTask is false")
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
	// Use LLM to generate the sub-workflow structure
	subWorkflow, err := m.generateSubWorkflowWithLLM(userMessage, intentResult)
	if err != nil {
		log.Printf("Failed to generate sub-workflow with LLM: %v", err)
		return nil
	}

	return subWorkflow
}

// generateSubWorkflowWithLLM uses LLM to generate a complete sub-workflow structure
func (m *LLMGraphManager) generateSubWorkflowWithLLM(userMessage string, intentResult *types.IntentAnalysisResult) (*types.SubWorkflow, error) {
	// Get available MCP tools
	mcpTools := m.getAvailableMCPTools()

	// Create LLM prompt for sub-workflow generation
	prompt := fmt.Sprintf(`You are a blockchain workflow generation assistant.

USER REQUEST: "%s"
INTENT ANALYSIS: %s

AVAILABLE MCP TOOLS:
%s

YOUR TASK: Generate a complete sub-workflow structure based on the user request and intent analysis.

INSTRUCTIONS:
1. Analyze the user request to understand what needs to be done
2. Identify all required tools and their dependencies
3. Create tasks with proper parameter mappings
4. Set appropriate execution mode (parallel/sequential/mixed)
5. Define aggregation strategy for results

WORKFLOW GENERATION RULES:
- Each task should have a unique ID
- Tasks that depend on others should specify depends_on
- Use {{previous_task_output}} for dynamic parameters
- RPC URLs should be extracted from the user message
- Tool names should match available MCP tools exactly

PARAMETER MAPPING RULES:
- For qng_get_stateroot tool: use "block_order" and "rpc_url" parameters
- For qng_get_block_by_order tool: use "block_order" and "rpc_url" parameters
- For qng_get_block_count tool: only "rpc_url" parameter needed
- Always use "rpc_url" for RPC endpoint (not "rpc")
- Use "block_order" for block numbers (not "order")

COMMON PATTERNS:
- Multiple RPC queries: parallel execution, compare aggregation
- Sequential operations: sequential execution, merge aggregation
- Data analysis: mixed execution, summarize aggregation

Return ONLY a JSON object with the complete sub-workflow structure:
{
  "id": "generated_workflow_id",
  "name": "descriptive_name",
  "description": "what_this_accomplishes",
  "execution_mode": "parallel|sequential|mixed",
  "aggregation_strategy": "compare|summarize|merge|raw",
  "tasks": [
    {
      "id": "task_id",
      "task_type": "mcp_tool",
      "tool_name": "exact_tool_name",
      "parameters": {"rpc_url": "http://example.com", "block_order": "{{previous_task_output}}"},
      "depends_on": ["task_ids_this_depends_on"],
      "description": "task_description"
    }
  ]
}`,
		userMessage,
		fmt.Sprintf("%+v", intentResult),
		mcpTools)

	// Call LLM for sub-workflow generation
	chatMessages := []types.ChatMessage{
		{Role: "user", Content: prompt},
	}

	ctx := context.Background()
	response, err := m.llmClient.GetCompletion(ctx, chatMessages)
	if err != nil {
		return nil, fmt.Errorf("LLM completion failed: %w", err)
	}

	// Parse LLM response as JSON
	var subWorkflow types.SubWorkflow
	if err := json.Unmarshal([]byte(response), &subWorkflow); err != nil {
		// Try to extract JSON from response
		jsonStart := strings.Index(response, "{")
		jsonEnd := strings.LastIndex(response, "}")
		if jsonStart != -1 && jsonEnd != -1 && jsonEnd > jsonStart {
			jsonStr := response[jsonStart : jsonEnd+1]
			if err := json.Unmarshal([]byte(jsonStr), &subWorkflow); err != nil {
				return nil, fmt.Errorf("failed to parse LLM response as JSON: %w", err)
			}
		} else {
			return nil, fmt.Errorf("no valid JSON found in LLM response")
		}
	}

	// Validate and enhance the generated sub-workflow
	if subWorkflow.ID == "" {
		subWorkflow.ID = fmt.Sprintf("sub_workflow_%d", time.Now().Unix())
	}

	// Ensure all tasks have proper task_type
	for i := range subWorkflow.Tasks {
		if subWorkflow.Tasks[i].TaskType == "" {
			subWorkflow.Tasks[i].TaskType = "mcp_tool"
		}
	}

	return &subWorkflow, nil
}

// mcpToolExecutionNode executes MCP tools via real MCP server calls
func (m *LLMGraphManager) mcpToolExecutionNode(ctx context.Context, state []llms.MessageContent, options graph.Option) ([]llms.MessageContent, error) {
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

	// Validate intent result
	if intentResult.MCPTool == "" {
		return state, fmt.Errorf("no MCP tool specified in intent result")
	}

	// Prepare parameters for MCP tool call
	parameters := intentResult.Parameters
	if parameters == nil {
		parameters = make(map[string]interface{})
	}

	// Use LLM to validate and enhance parameters if needed
	enhancedParams, err := m.enhanceParametersWithLLM(ctx, intentResult.MCPTool, parameters, state)
	if err != nil {
		log.Printf("Failed to enhance parameters with LLM: %v", err)
		// Continue with original parameters
	} else {
		parameters = enhancedParams
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

	// Format the result using LLM for better user experience
	formattedResult, err := m.formatToolResultWithLLM(ctx, intentResult.MCPTool, result, state)
	if err != nil {
		log.Printf("Failed to format tool result with LLM: %v", err)
		// Use raw result if formatting fails
		formattedResult = string(resultData)
	}

	// Add formatted tool result to state
	toolMessage := llms.MessageContent{
		Role: llms.ChatMessageTypeSystem,
		Parts: []llms.ContentPart{
			llms.TextPart(fmt.Sprintf("TOOL_RESULT:%s", formattedResult)),
		},
	}

	return append(state, toolMessage), nil
}

// subWorkflowExecutionNode executes sub-workflows for multi-task requests
func (m *LLMGraphManager) subWorkflowExecutionNode(ctx context.Context, state []llms.MessageContent, options graph.Option) ([]llms.MessageContent, error) {
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
	log.Printf("=== EXECUTING SUB-WORKFLOW ===")
	log.Printf("Workflow Name: %s", subWorkflow.Name)
	log.Printf("Number of Tasks: %d", len(subWorkflow.Tasks))
	log.Printf("Execution Mode: %s", subWorkflow.ExecutionMode)
	log.Printf("Aggregation Strategy: %s", subWorkflow.AggregationStrategy)

	// Log task details before execution
	for i, task := range subWorkflow.Tasks {
		log.Printf("Pre-execution Task %d: %s", i+1, task.ID)
		log.Printf("  Tool: %s", task.ToolName)
		log.Printf("  Parameters: %+v", task.Parameters)
		log.Printf("  Depends On: %v", task.DependsOn)
		log.Printf("  RPC: %s", task.RPC)
	}

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
	log.Printf("=== RESOLVING DEPENDENT PARAMETERS ===")
	log.Printf("Task ID: %s", task.ID)
	log.Printf("Tool Name: %s", task.ToolName)
	log.Printf("Original Parameters: %+v", task.Parameters)
	log.Printf("Depends On: %v", task.DependsOn)

	// Collect dependency results
	dependencyContext := make(map[string]interface{})
	for _, depTaskID := range task.DependsOn {
		if result, exists := resultMap[depTaskID]; exists {
			dependencyContext[depTaskID] = result
			log.Printf("Found dependency result for %s: %+v", depTaskID, result)
		} else {
			log.Printf("WARNING: No result found for dependency %s", depTaskID)
		}
	}

	log.Printf("Dependency Context: %+v", dependencyContext)

	// Use LLM to extract parameters from dependency results
	log.Printf("Calling LLM to extract parameters...")
	resolvedParameters, err := m.extractParametersWithLLM(ctx, task, dependencyContext)
	if err != nil {
		log.Printf("LLM parameter extraction failed: %v", err)
		return task, fmt.Errorf("LLM parameter extraction failed: %w", err)
	}

	log.Printf("LLM extracted parameters: %+v", resolvedParameters)

	// Create resolved task with updated parameters
	resolvedTask := task
	resolvedTask.Parameters = resolvedParameters

	log.Printf("Final resolved task parameters: %+v", resolvedTask.Parameters)
	log.Printf("=== END PARAMETER RESOLUTION ===")
	return resolvedTask, nil
}

// enhanceParametersWithLLM uses LLM to validate and enhance parameters for MCP tool execution
func (m *LLMGraphManager) enhanceParametersWithLLM(ctx context.Context, toolName string, parameters map[string]interface{}, state []llms.MessageContent) (map[string]interface{}, error) {
	// Get user message from state
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

	// Get available MCP tools to understand parameter requirements
	mcpTools := m.getAvailableMCPTools()

	// Create LLM prompt for parameter enhancement
	prompt := fmt.Sprintf(`You are a blockchain parameter validation assistant.

USER REQUEST: "%s"
TOOL TO EXECUTE: %s
CURRENT PARAMETERS: %s

AVAILABLE MCP TOOLS:
%s

YOUR TASK: Validate and enhance the parameters for the specified tool.

INSTRUCTIONS:
1. Review the user request and current parameters
2. Check if all required parameters are present and correctly formatted
3. Extract any missing parameters from the user message
4. Ensure parameter types and formats are correct
5. Add any additional context that might be helpful

COMMON PARAMETER PATTERNS:
- RPC URLs: Should be complete HTTP/HTTPS URLs (use "rpc_url" parameter)
- Block numbers: Should be integers (extract from "order", "height", "block" keywords, use "block_order" parameter)
- Addresses: Should be valid blockchain addresses
- Token symbols: Should be uppercase (e.g., "MEER", "USDT")

PARAMETER MAPPING RULES:
- For qng_get_stateroot tool: use "block_order" and "rpc_url" parameters
- For qng_get_block_by_order tool: use "block_order" and "rpc_url" parameters
- For qng_get_block_count tool: only "rpc_url" parameter needed
- Always use "rpc_url" for RPC endpoint (not "rpc")

RULES:
- Always preserve existing parameters unless they are clearly wrong
- Extract missing parameters from the user message
- Return ONLY a JSON object with the enhanced parameters
- Do not include explanations, just the JSON

Expected JSON format:
{"rpc_url": "extracted_or_preserved_rpc_url", "block_order": extracted_block_number, "other_param": "value"}`,
		userMessage,
		toolName,
		fmt.Sprintf("%+v", parameters),
		mcpTools)

	// Call LLM for parameter enhancement
	chatMessages := []types.ChatMessage{
		{Role: "user", Content: prompt},
	}

	response, err := m.llmClient.GetCompletion(ctx, chatMessages)
	if err != nil {
		return nil, fmt.Errorf("LLM completion failed: %w", err)
	}

	// Parse LLM response as JSON
	var enhancedParams map[string]interface{}
	if err := json.Unmarshal([]byte(response), &enhancedParams); err != nil {
		// Try to extract JSON from response
		jsonStart := strings.Index(response, "{")
		jsonEnd := strings.LastIndex(response, "}")
		if jsonStart != -1 && jsonEnd != -1 && jsonEnd > jsonStart {
			jsonStr := response[jsonStart : jsonEnd+1]
			if err := json.Unmarshal([]byte(jsonStr), &enhancedParams); err != nil {
				return parameters, fmt.Errorf("failed to parse LLM response as JSON: %w", err)
			}
		} else {
			return parameters, fmt.Errorf("no valid JSON found in LLM response")
		}
	}

	return enhancedParams, nil
}

// extractParametersWithLLM uses LLM to extract parameters from previous task results
func (m *LLMGraphManager) extractParametersWithLLM(ctx context.Context, task types.TaskExecution, dependencyResults map[string]interface{}) (map[string]interface{}, error) {
	log.Printf("=== EXTRACTING PARAMETERS WITH LLM ===")
	log.Printf("Current Task: %s (%s)", task.ID, task.ToolName)
	log.Printf("Dependency Results Count: %d", len(dependencyResults))

	// Build context information for LLM
	contextStr := ""
	for taskID, result := range dependencyResults {
		resultJSON, _ := json.Marshal(result)
		contextStr += fmt.Sprintf("Task %s result: %s\n", taskID, string(resultJSON))
		log.Printf("Dependency %s result: %s", taskID, string(resultJSON))
	}

	log.Printf("Context string for LLM: %s", contextStr)

	// Get available MCP tools to understand parameter requirements
	mcpTools := m.getAvailableMCPTools()

	// Create LLM prompt for parameter extraction
	prompt := fmt.Sprintf(`You are a blockchain data extraction and transformation assistant. 

CONTEXT: Previous task results:
%s

CURRENT TASK: %s (tool: %s)
Task Description: %s
Original Parameters: %s

AVAILABLE MCP TOOLS:
%s

YOUR TASK: Extract and transform the required parameters for the current task from the previous task results.

INSTRUCTIONS:
1. Analyze the previous task results to understand the data structure
2. Identify the specific data needed for the current task parameters
3. Transform data formats as needed (e.g., string to int, extract nested values)
4. Handle different data sources and formats intelligently

COMMON DATA TRANSFORMATIONS:
- JSON-RPC responses: Extract "result" field values
- Block count numbers: Convert to integer for "block_order" parameter
- RPC URLs: Preserve from original parameters or extract from results
- Addresses: Validate and format blockchain addresses
- Timestamps: Convert to appropriate format if needed
- Nested objects: Extract specific fields using dot notation

ADVANCED EXTRACTION RULES:
- If result contains {"jsonrpc":"2.0","id":1,"result":13077047}, extract 13077047 as "block_order"
- If result contains nested data, use appropriate field paths
- Handle arrays by selecting relevant elements
- Convert string numbers to integers when needed
- Preserve original parameter structure unless transformation is needed

PARAMETER MAPPING RULES:
- For qng_get_stateroot tool: use "block_order" parameter (not "order")
- For qng_get_block_by_order tool: use "block_order" parameter (not "order")
- For qng_get_block_count tool: only "rpc_url" parameter needed
- Always use "rpc_url" for RPC endpoint (not "rpc")

RULES:
- Always preserve "rpc_url" parameter from original task parameters
- Transform data types appropriately (string numbers to int, etc.)
- Handle missing or malformed data gracefully
- Return ONLY a JSON object with the resolved parameters
- Do not include explanations, just the JSON

Expected JSON format:
{"rpc_url": "extracted_or_preserved_rpc_url", "block_order": extracted_block_number, "other_param": "transformed_value"}`,
		contextStr,
		task.ID,
		task.ToolName,
		task.Description,
		fmt.Sprintf("%+v", task.Parameters),
		mcpTools)

	// Call LLM for parameter extraction
	chatMessages := []types.ChatMessage{
		{Role: "user", Content: prompt},
	}

	log.Printf("Sending prompt to LLM for parameter extraction...")
	log.Printf("Prompt length: %d characters", len(prompt))

	response, err := m.llmClient.GetCompletion(ctx, chatMessages)
	if err != nil {
		log.Printf("LLM completion failed: %v", err)
		return nil, fmt.Errorf("LLM completion failed: %w", err)
	}

	log.Printf("LLM response received: %s", response)

	// Parse LLM response as JSON
	var extractedParams map[string]interface{}
	if err := json.Unmarshal([]byte(response), &extractedParams); err != nil {
		log.Printf("Failed to parse LLM response as JSON: %v", err)
		// Try to extract JSON from response
		jsonStart := strings.Index(response, "{")
		jsonEnd := strings.LastIndex(response, "}")
		if jsonStart != -1 && jsonEnd != -1 && jsonEnd > jsonStart {
			jsonStr := response[jsonStart : jsonEnd+1]
			log.Printf("Extracted JSON string: %s", jsonStr)
			if err := json.Unmarshal([]byte(jsonStr), &extractedParams); err != nil {
				log.Printf("Failed to parse extracted JSON: %v", err)
				return nil, fmt.Errorf("failed to parse LLM response as JSON: %w", err)
			}
		} else {
			log.Printf("No valid JSON found in LLM response")
			return nil, fmt.Errorf("no valid JSON found in LLM response: %s", response)
		}
	}

	log.Printf("Successfully extracted parameters: %+v", extractedParams)
	log.Printf("=== END LLM PARAMETER EXTRACTION ===")
	return extractedParams, nil
}

// formatToolResultWithLLM uses LLM to format tool execution results for better user experience
func (m *LLMGraphManager) formatToolResultWithLLM(ctx context.Context, toolName string, result interface{}, state []llms.MessageContent) (string, error) {
	// Get user message from state
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

	// Convert result to JSON string for LLM processing
	resultJSON, err := json.Marshal(result)
	if err != nil {
		resultJSON = []byte(fmt.Sprintf(`{"error": "Failed to marshal result", "raw_result": "%v"}`, result))
	}

	// Create LLM prompt for result formatting
	prompt := fmt.Sprintf(`You are a blockchain data presentation assistant.

USER REQUEST: "%s"
TOOL EXECUTED: %s
RAW RESULT: %s

YOUR TASK: Format the tool execution result into a user-friendly response.

INSTRUCTIONS:
1. Analyze the raw result data structure
2. Extract meaningful information from the result
3. Present the data in a clear, organized format
4. Use appropriate language based on the user's request language
5. Add context and explanations where helpful

FORMATTING GUIDELINES:
- Use Markdown formatting for better readability
- Create tables for comparing multiple data points
- Use emojis and formatting to make the response engaging
- Structure information with headers (##, ###) for organization
- Use bullet points and numbered lists for clarity
- Add visual separators (---) between sections

COMMON DATA PATTERNS:
- JSON-RPC responses: Extract "result" field values
- Block data: Present block number, hash, timestamp, transactions
- Balance data: Show token amounts with proper formatting
- Error responses: Explain the error in user-friendly terms

LANGUAGE CONSISTENCY:
- Respond in the SAME language as the user's request
- If user asked in Chinese, respond in Chinese
- If user asked in English, respond in English
- Keep technical terms consistent with user's language

RULES:
- Always provide context about what the data means
- Use clear, simple language that both technical and non-technical users can understand
- Highlight important values with **bold** or backtick code blocks
- For blockchain data, explain what it means in practical terms
- If there are errors, explain what went wrong and suggest solutions

Return ONLY the formatted response, no additional explanations.`,
		userMessage,
		toolName,
		string(resultJSON))

	// Call LLM for result formatting
	chatMessages := []types.ChatMessage{
		{Role: "user", Content: prompt},
	}

	response, err := m.llmClient.GetCompletion(ctx, chatMessages)
	if err != nil {
		return "", fmt.Errorf("LLM completion failed: %w", err)
	}

	return response, nil
}

// executeTasksInParallel executes tasks concurrently with dependency resolution
func (m *LLMGraphManager) executeTasksInParallel(ctx context.Context, tasks []types.TaskExecution) ([]types.TaskResult, error) {
	// First, check if any tasks have dependencies
	hasDependencies := false
	for _, task := range tasks {
		if len(task.DependsOn) > 0 {
			hasDependencies = true
			break
		}
	}

	// If no dependencies, execute all tasks in parallel
	if !hasDependencies {
		return m.executeTasksInParallelSimple(ctx, tasks)
	}

	// If there are dependencies, use a hybrid approach:
	// 1. Execute independent tasks in parallel
	// 2. Execute dependent tasks after their dependencies complete
	return m.executeTasksWithDependencies(ctx, tasks)
}

// executeTasksInParallelSimple executes tasks concurrently without dependency resolution
func (m *LLMGraphManager) executeTasksInParallelSimple(ctx context.Context, tasks []types.TaskExecution) ([]types.TaskResult, error) {
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

// executeTasksWithDependencies executes tasks with proper dependency resolution
func (m *LLMGraphManager) executeTasksWithDependencies(ctx context.Context, tasks []types.TaskExecution) ([]types.TaskResult, error) {
	var results []types.TaskResult
	resultMap := make(map[string]map[string]interface{}) // taskID -> result data
	completedTasks := make(map[string]bool)

	// Create a map for quick task lookup
	taskMap := make(map[string]types.TaskExecution)
	for _, task := range tasks {
		taskMap[task.ID] = task
	}

	// Execute tasks in rounds based on dependencies
	for len(completedTasks) < len(tasks) {
		var readyTasks []types.TaskExecution

		// Find tasks that are ready to execute (no dependencies or all dependencies completed)
		for _, task := range tasks {
			if completedTasks[task.ID] {
				continue
			}

			ready := true
			for _, depID := range task.DependsOn {
				if !completedTasks[depID] {
					ready = false
					break
				}
			}

			if ready {
				readyTasks = append(readyTasks, task)
			}
		}

		if len(readyTasks) == 0 {
			// This shouldn't happen if the dependency graph is valid
			return results, fmt.Errorf("circular dependency or invalid task dependencies detected")
		}

		// Resolve parameters for ready tasks that have dependencies
		var resolvedTasks []types.TaskExecution
		for _, task := range readyTasks {
			if len(task.DependsOn) > 0 {
				log.Printf("=== RESOLVING PARAMETERS FOR TASK: %s ===", task.ID)
				log.Printf("Original parameters: %+v", task.Parameters)
				log.Printf("Dependencies: %v", task.DependsOn)
				log.Printf("Available results: %+v", resultMap)

				// Resolve dependent parameters
				resolvedTask, err := m.resolveDependentParameters(ctx, task, results, resultMap)
				if err != nil {
					log.Printf("Failed to resolve parameters for task %s: %v", task.ID, err)
					// Add failed task to results
					results = append(results, types.TaskResult{
						TaskID:  task.ID,
						Success: false,
						Error:   err.Error(),
						RPC:     task.RPC,
					})
					completedTasks[task.ID] = true
					continue
				}

				log.Printf("Resolved parameters: %+v", resolvedTask.Parameters)
				log.Printf("=== END PARAMETER RESOLUTION FOR TASK: %s ===", task.ID)
				resolvedTasks = append(resolvedTasks, resolvedTask)
			} else {
				log.Printf("Task %s has no dependencies, using original parameters: %+v", task.ID, task.Parameters)
				resolvedTasks = append(resolvedTasks, task)
			}
		}

		// Execute resolved tasks in parallel
		roundResults, err := m.executeTasksInParallelSimple(ctx, resolvedTasks)
		if err != nil {
			return results, err
		}

		// Process results and update state
		for _, taskResult := range roundResults {
			results = append(results, taskResult)
			completedTasks[taskResult.TaskID] = true

			if taskResult.Success && taskResult.Result != nil {
				resultMap[taskResult.TaskID] = taskResult.Result
			}
		}
	}

	return results, nil
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

// web3WorkflowExecutionNode executes Web3 workflows as per requirements
func (m *LLMGraphManager) web3WorkflowExecutionNode(ctx context.Context, state []llms.MessageContent, options graph.Option) ([]llms.MessageContent, error) {
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
func (m *LLMGraphManager) responseGenerationNode(ctx context.Context, state []llms.MessageContent, options graph.Option) ([]llms.MessageContent, error) {
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
func (m *LLMGraphManager) routeAfterIntent(ctx context.Context, state []llms.MessageContent, options graph.Option) string {
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

	// log.Printf("Intent analysis prompt: %s", prompt)
	return prompt
}

// getAvailableMCPTools returns a formatted string of available MCP tools
func (m *LLMGraphManager) getAvailableMCPTools() string {
	var toolDescriptions []string

	// Get MCP servers from configuration
	for _, server := range m.config.MCP.DefaultServers {
		if server.Enabled {
			tools := m.getMCPServerTools(server.Name)
			toolDescriptions = append(toolDescriptions, tools...)
		}
	}

	// Get additional tools from user settings if available
	// Note: This would need userID parameter passed through - skipping for now to simplify

	if len(toolDescriptions) == 0 {
		return "  - No MCP tools currently configured"
	}

	return strings.Join(toolDescriptions, "\n")
}

// getMCPServerTools returns tool descriptions for a specific MCP server
func (m *LLMGraphManager) getMCPServerTools(serverName string) []string {
	// Try to get tools dynamically from the MCP server
	if serverURL, exists := m.getServerURL(serverName, ""); exists {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		tools, err := m.mcpClient.GetServerTools(ctx, serverURL)
		if err == nil && len(tools) > 0 {
			// log.Printf("Got tools from server %s: %v", serverName, tools)
			return m.formatToolsFromServer(tools)
		}

		log.Printf("Failed to get tools from server %s: %v", serverName, err)
	}

	// Return empty if server is not accessible - no hardcoded fallback
	log.Printf("No tools available for server %s", serverName)
	return []string{}
}

// getServerURL gets the URL for a server name
func (m *LLMGraphManager) getServerURL(serverName, userID string) (string, bool) {
	// Check configuration servers
	for _, server := range m.config.MCP.DefaultServers {
		if server.Name == serverName && server.Enabled {
			return server.URL, true
		}
	}

	// Check user settings
	if userID != "" {
		if settings, err := m.getUserSettings(userID); err == nil && settings.MCPServers != nil {
			for _, server := range settings.MCPServers {
				if server.Name == serverName && server.Enabled {
					return server.URL, true
				}
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
		// Check for inputSchema (MCP standard) or parameters field
		var params map[string]interface{}
		if inputSchema, ok := tool["inputSchema"].(map[string]interface{}); ok {
			if properties, ok := inputSchema["properties"].(map[string]interface{}); ok {
				params = properties
			}
		} else if paramMap, ok := tool["parameters"].(map[string]interface{}); ok {
			params = paramMap
		}

		if len(params) > 0 {
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
func (m *LLMGraphManager) getUserSettings(userID string) (*types.AppSettings, error) {
	// Use the LoadSettings method from the storage interface with user-specific ID
	return m.storage.LoadSettings(userID)
}

// min helper function
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Wrapper functions to adapt to new graph API
func (m *LLMGraphManager) wrapIntentAnalysisNode(ctx context.Context, name string, state graph.State) (graph.State, error) {
	// Convert state to []llms.MessageContent
	messages, ok := state["messages"].([]llms.MessageContent)
	if !ok {
		return nil, fmt.Errorf("invalid state: messages not found or wrong type")
	}

	// Call original function
	result, err := m.intentAnalysisNode(ctx, messages, nil)
	if err != nil {
		return nil, err
	}

	// Convert result back to state
	newState := make(graph.State)
	for k, v := range state {
		newState[k] = v
	}
	newState["messages"] = result
	return newState, nil
}

func (m *LLMGraphManager) wrapMcpToolExecutionNode(ctx context.Context, name string, state graph.State) (graph.State, error) {
	messages, ok := state["messages"].([]llms.MessageContent)
	if !ok {
		return nil, fmt.Errorf("invalid state: messages not found or wrong type")
	}

	result, err := m.mcpToolExecutionNode(ctx, messages, nil)
	if err != nil {
		return nil, err
	}

	newState := make(graph.State)
	for k, v := range state {
		newState[k] = v
	}
	newState["messages"] = result
	return newState, nil
}

func (m *LLMGraphManager) wrapSubWorkflowExecutionNode(ctx context.Context, name string, state graph.State) (graph.State, error) {
	messages, ok := state["messages"].([]llms.MessageContent)
	if !ok {
		return nil, fmt.Errorf("invalid state: messages not found or wrong type")
	}

	result, err := m.subWorkflowExecutionNode(ctx, messages, nil)
	if err != nil {
		return nil, err
	}

	newState := make(graph.State)
	for k, v := range state {
		newState[k] = v
	}
	newState["messages"] = result
	return newState, nil
}

func (m *LLMGraphManager) wrapWeb3WorkflowExecutionNode(ctx context.Context, name string, state graph.State) (graph.State, error) {
	messages, ok := state["messages"].([]llms.MessageContent)
	if !ok {
		return nil, fmt.Errorf("invalid state: messages not found or wrong type")
	}

	result, err := m.web3WorkflowExecutionNode(ctx, messages, nil)
	if err != nil {
		return nil, err
	}

	newState := make(graph.State)
	for k, v := range state {
		newState[k] = v
	}
	newState["messages"] = result
	return newState, nil
}

func (m *LLMGraphManager) wrapResponseGenerationNode(ctx context.Context, name string, state graph.State) (graph.State, error) {
	messages, ok := state["messages"].([]llms.MessageContent)
	if !ok {
		return nil, fmt.Errorf("invalid state: messages not found or wrong type")
	}

	result, err := m.responseGenerationNode(ctx, messages, nil)
	if err != nil {
		return nil, err
	}

	newState := make(graph.State)
	for k, v := range state {
		newState[k] = v
	}
	newState["messages"] = result
	return newState, nil
}

func (m *LLMGraphManager) wrapRouteAfterIntent(ctx context.Context, name string, state graph.State) string {
	messages, ok := state["messages"].([]llms.MessageContent)
	if !ok {
		// Default to response generation if state is invalid
		return "response_generation"
	}

	return m.routeAfterIntent(ctx, messages, nil)
}
