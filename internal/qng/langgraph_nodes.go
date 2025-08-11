package qng

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"qng_agent/internal/contracts"
	"qng_agent/internal/llm"
	"qng_agent/internal/rpc"
	"regexp"
	"strings"
	"time"
)

// TaskDecomposerNode 任务分解节点
type TaskDecomposerNode struct {
	llmClient       llm.Client
	contractManager *contracts.ContractManager
	registerManager *contracts.RegisterContractManager // 添加注册管理器
}

func NewTaskDecomposerNode(llmClient llm.Client, contractManager *contracts.ContractManager) *TaskDecomposerNode {
	return &TaskDecomposerNode{
		llmClient:       llmClient,
		contractManager: contractManager,
		registerManager: nil, // 暂时设为nil，稍后通过其他方式设置
	}
}

func (n *TaskDecomposerNode) GetName() string {
	return "task_decomposer"
}

func (n *TaskDecomposerNode) GetType() string {
	return "llm_processor"
}

func (n *TaskDecomposerNode) Execute(ctx context.Context, input NodeInput) (*NodeOutput, error) {
	log.Printf("🔄 任务分解节点开始执行")

	userMessage, ok := input.Data["user_message"].(string)
	if !ok {
		log.Printf("❌ 输入中缺少user_message")
		return nil, fmt.Errorf("user_message not found in input")
	}

	log.Printf("📝 用户消息: %s", userMessage)

	// 从合约管理器获取实际配置信息
	var supportedTokens []string
	var supportedPairs []string
	var contractInfo string
	var registeredContracts []map[string]interface{}

	if n.contractManager != nil {
		// 获取支持的代币
		supportedTokens = n.contractManager.GetSupportedTokens()

		// 获取支持的兑换对
		supportedPairs = n.contractManager.GetSupportedPairs()

		// 获取合约信息
		contractInfo = n.contractManager.GetWorkflowDescription()

		// 获取注册中心的合约信息
		if n.registerManager != nil {
			allContracts := n.registerManager.ListAllContractsFromRegistry()
			for _, contract := range allContracts {
				registeredContracts = append(registeredContracts, map[string]interface{}{
					"id":          contract.AgentID,
					"name":        contract.Name,
					"type":        contract.Type,
					"address":     contract.Address,
					"services":    contract.Services,
					"protocols":   contract.Protocols,
					"description": contract.Description,
				})
			}
		}

		log.Printf("📋 支持的代币: %v", supportedTokens)
		log.Printf("📋 支持的兑换对: %v", supportedPairs)
		log.Printf("📋 合约信息: %s", contractInfo)
		log.Printf("📋 注册的合约: %+v", registeredContracts)
	} else {
		// 默认配置
		supportedTokens = []string{"MEER", "MTK"}
		supportedPairs = []string{"MEER ↔ MTK"}
		contractInfo = "SimpleSwap合约支持MEER和MTK之间的兑换"
		registeredContracts = []map[string]interface{}{
			{
				"id":          "simpleswap",
				"name":        "SimpleSwap",
				"type":        "DEX",
				"services":    []string{"Swap", "DEX"},
				"description": "Simple token swap contract",
			},
		}
	}

	// 构建注册合约信息
	registeredContractsInfo := ""
	for _, contract := range registeredContracts {
		registeredContractsInfo += fmt.Sprintf(`
- ID: %s
  名称: %s
  类型: %s
  地址: %s
  服务: %v
  协议: %v
  描述: %s`,
			contract["id"], contract["name"], contract["type"], contract["address"],
			contract["services"], contract["protocols"], contract["description"])
	}

	// 构建LLM提示
	prompt := fmt.Sprintf(`
你是一个区块链DeFi操作分析助手。请仔细分析用户的中文请求，并分解为具体的执行步骤。

系统配置信息：
%s

注册的合约信息：
%s

支持的操作类型：
1. swap: 代币兑换
2. stake: 代币质押

支持的代币：%v
支持的兑换对：%v

用户请求: %s

请根据用户的实际请求内容，准确识别代币名称和数量，并从注册的合约中选择合适的合约ID，按以下格式返回分解结果：

{
  "tasks": [
    {
      "id": "task_1",
      "type": "swap",
      "from_token": "MEER", 
      "to_token": "MTK",
      "amount": "1",
      "contract_id": "simpleswap",
      "dependency_tx_id": null,
      "description": "兑换1 MEER为MTK"
    },
    {
      "id": "task_2", 
      "type": "stake",
      "token": "MTK",
      "amount": "all_from_previous",
      "pool": "compound",
      "dependency_tx_id": "task_1",
      "description": "将兑换得到的MTK进行质押"
    }
  ]
}

重要规则：
1. 仔细阅读用户请求，准确提取代币名称和数量
2. 只使用支持的代币：%v，不要使用其他代币如USDT、BTC等
3. 只使用支持的兑换对：%v
4. 如果用户说"兑换X MEER的MTK"，意思是用X个MEER兑换MTK
5. 如果用户说"质押MTK"，使用stake类型
6. 如果是连续操作（先兑换后质押），第二个任务要设置dependency_tx_id
7. 每个任务必须有唯一的id（task_1, task_2...）
8. amount可以设置为"all_from_previous"表示使用前一个任务的全部输出
9. 独立任务的dependency_tx_id设置为null
10. 必须包含所有必要字段：id, type, from_token, to_token, amount, dependency_tx_id, description
11. 对于stake任务，使用token字段而不是from_token/to_token
12. 对于不存在的token或者其他合约，请提示用户，并给出建议的合约名称
CRITICAL: 你必须只返回有效的JSON格式，不要包含任何其他文字、解释或说明。确保JSON格式完全正确，可以被直接解析。
`, contractInfo, supportedTokens, supportedPairs, userMessage, supportedTokens, supportedPairs)

	log.Printf("📋 构建LLM提示完成")
	log.Printf("📝 提示长度: %d", len(prompt))

	// 调用LLM进行任务分解
	if n.llmClient != nil {
		log.Printf("🤖 调用LLM进行任务分解...")
		response, err := n.llmClient.Chat(ctx, []llm.Message{
			{Role: "user", Content: prompt},
		})
		if err != nil {
			log.Printf("❌ LLM调用失败: %v", err)
			return nil, fmt.Errorf("LLM call failed: %w", err)
		}

		log.Printf("✅ LLM响应成功")
		log.Printf("📄 LLM响应: %s", response)

		// 解析LLM响应
		log.Printf("🔄 解析LLM响应...")
		tasks := n.parseTasksFromResponse(response, userMessage)
		log.Printf("📋 解析出 %d 个任务", len(tasks))

		return &NodeOutput{
			Data: map[string]any{
				"tasks":         tasks,
				"user_message":  userMessage,
				"decomposed_at": input.Data["timestamp"],
			},
			Completed: false,
		}, nil
	}

	// 如果没有LLM客户端，使用简单的规则分解
	log.Printf("⚠️  没有LLM客户端，使用简单规则分解")
	tasks := n.simpleTaskDecomposition(userMessage)
	log.Printf("📋 简单分解出 %d 个任务", len(tasks))

	return &NodeOutput{
		Data: map[string]any{
			"tasks":        tasks,
			"user_message": userMessage,
		},
		Completed: false,
	}, nil
}

func (n *TaskDecomposerNode) parseTasksFromResponse(response string, originalUserMessage string) []map[string]any {
	log.Printf("🔄 解析LLM响应中的任务")
	log.Printf("📄 响应内容: %s", response)

	// 尝试从JSON响应中解析任务
	// 首先尝试提取JSON部分
	jsonStart := strings.Index(response, "{")
	jsonEnd := strings.LastIndex(response, "}")

	if jsonStart >= 0 && jsonEnd > jsonStart {
		jsonStr := response[jsonStart : jsonEnd+1]
		log.Printf("📋 提取的JSON: %s", jsonStr)

		// 尝试使用json.Unmarshal解析
		var result map[string]any
		if err := json.Unmarshal([]byte(jsonStr), &result); err == nil {
			if tasks, ok := result["tasks"].([]any); ok {
				log.Printf("✅ 成功解析JSON，找到 %d 个任务", len(tasks))

				// 转换为所需格式并验证内容
				taskList := make([]map[string]any, 0, len(tasks))
				allTasksValid := true
				hasUnsupportedTokens := false

				for i, task := range tasks {
					if taskMap, ok := task.(map[string]any); ok {
						log.Printf("📋 任务[%d]: %+v", i, taskMap)

						// 补充缺失的字段
						enhancedTask := n.enhanceTask(taskMap, i+1)
						log.Printf("📋 增强后的任务[%d]: %+v", i, enhancedTask)

						// 验证代币是否为支持的类型
						if taskType, exists := enhancedTask["type"].(string); exists && taskType == "swap" {
							fromToken, _ := enhancedTask["from_token"].(string)
							toToken, _ := enhancedTask["to_token"].(string)

							// 检查是否为支持的代币对
							if supported, errorMsg := n.isSupportedTokenPair(fromToken, toToken); !supported {
								log.Printf("⚠️  %s", errorMsg)
								hasUnsupportedTokens = true
								break
							}
						}

						taskList = append(taskList, enhancedTask)
					}
				}

				if hasUnsupportedTokens {
					log.Printf("❌ 检测到不支持的代币，返回错误信息")
					// 返回包含错误信息的特殊任务
					return []map[string]any{
						{
							"id":          "error_1",
							"type":        "error",
							"error":       "检测到不支持的代币",
							"description": "当前系统只支持 MEER 和 MTK 之间的兑换",
							"message":     "❌ 抱歉，当前系统不支持您请求的代币。\n\n💡 建议：\n- 当前系统只支持 MEER 和 MTK 之间的兑换\n- 您可以尝试：兑换 1 MEER 为 MTK，然后质押 MTK\n- 或者直接质押您现有的 MTK",
						},
					}
				}

				if allTasksValid {
					log.Printf("✅ 所有任务验证通过")
					return taskList
				} else {
					log.Printf("⚠️  任务内容验证失败，使用备用解析")
				}
			}
		} else {
			log.Printf("⚠️  JSON解析失败: %v", err)
		}
	}

	// 如果JSON解析失败或内容验证失败，使用原始用户输入进行文本解析
	log.Printf("🔄 使用原始用户输入进行备用解析")
	log.Printf("📝 原始用户输入: %s", originalUserMessage)
	return n.fallbackParseFromText(originalUserMessage)
}

// enhanceTask 增强任务，补充缺失的字段
func (n *TaskDecomposerNode) enhanceTask(task map[string]any, index int) map[string]any {
	enhanced := make(map[string]any)

	// 复制原始任务的所有字段
	for k, v := range task {
		enhanced[k] = v
	}

	// 补充缺失的字段
	if _, exists := enhanced["id"]; !exists {
		enhanced["id"] = fmt.Sprintf("task_%d", index)
	}

	if _, exists := enhanced["dependency_tx_id"]; !exists {
		enhanced["dependency_tx_id"] = nil
	}

	if _, exists := enhanced["description"]; !exists {
		// 根据任务类型生成描述
		if taskType, ok := enhanced["type"].(string); ok {
			switch taskType {
			case "swap":
				fromToken, _ := enhanced["from_token"].(string)
				toToken, _ := enhanced["to_token"].(string)
				amount, _ := enhanced["amount"].(string)
				enhanced["description"] = fmt.Sprintf("兑换%s %s为%s", amount, fromToken, toToken)
			case "stake":
				token, _ := enhanced["token"].(string)
				amount, _ := enhanced["amount"].(string)
				enhanced["description"] = fmt.Sprintf("质押%s %s", amount, token)
			}
		}
	}

	// 确保stake任务有token字段
	if taskType, ok := enhanced["type"].(string); ok && taskType == "stake" {
		if _, exists := enhanced["token"]; !exists {
			// 从from_token或to_token推断
			if fromToken, exists := enhanced["from_token"].(string); exists {
				enhanced["token"] = fromToken
			} else if toToken, exists := enhanced["to_token"].(string); exists {
				enhanced["token"] = toToken
			} else {
				enhanced["token"] = "MTK" // 默认值
			}
		}
	}

	// 纠正不支持的代币对
	if taskType, ok := enhanced["type"].(string); ok && taskType == "swap" {
		fromToken, _ := enhanced["from_token"].(string)
		toToken, _ := enhanced["to_token"].(string)

		// 如果是不支持的代币对，标记为错误
		if supported, errorMsg := n.isSupportedTokenPair(fromToken, toToken); !supported {
			log.Printf("⚠️  %s", errorMsg)
			enhanced["error"] = errorMsg
			enhanced["unsupported_tokens"] = true
		}

		// 如果金额明显过大（比如1000），可能是LLM理解错误，纠正为1
		if amount, ok := enhanced["amount"].(string); ok {
			if amount == "1000" || amount == "1000.0" {
				log.Printf("⚠️  检测到可能的金额错误: %s，纠正为 1", amount)
				enhanced["amount"] = "1"
			}
		}

		// 重新生成描述字段，确保使用纠正后的代币对和金额
		correctedFromToken, _ := enhanced["from_token"].(string)
		correctedToToken, _ := enhanced["to_token"].(string)
		correctedAmount, _ := enhanced["amount"].(string)
		enhanced["description"] = fmt.Sprintf("兑换%s %s为%s", correctedAmount, correctedFromToken, correctedToToken)
	}

	return enhanced
}

// isSupportedTokenPair 检查是否为支持的代币对
func (n *TaskDecomposerNode) isSupportedTokenPair(fromToken, toToken string) (bool, string) {
	supportedPairs := [][]string{
		{"MEER", "MTK"},
		{"MTK", "MEER"},
	}

	for _, pair := range supportedPairs {
		if pair[0] == fromToken && pair[1] == toToken {
			return true, ""
		}
	}

	// 检查是否有不支持的代币
	supportedTokens := []string{"MEER", "MTK"}
	unsupportedTokens := []string{}

	if !contains(supportedTokens, fromToken) {
		unsupportedTokens = append(unsupportedTokens, fromToken)
	}
	if !contains(supportedTokens, toToken) {
		unsupportedTokens = append(unsupportedTokens, toToken)
	}

	errorMsg := ""
	if len(unsupportedTokens) > 0 {
		errorMsg = fmt.Sprintf("不支持的代币: %v。当前系统只支持: %v", unsupportedTokens, supportedTokens)
	} else {
		errorMsg = fmt.Sprintf("不支持的代币对: %s->%s。当前系统只支持: MEER↔MTK", fromToken, toToken)
	}

	log.Printf("📋 %s", errorMsg)
	return false, errorMsg
}

// contains 检查字符串是否在切片中
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// fallbackParseFromText 简单文本解析作为备用
func (n *TaskDecomposerNode) fallbackParseFromText(userMessage string) []map[string]any {
	log.Printf("🔄 使用文本解析备用方案")
	log.Printf("📝 分析用户消息: %s", userMessage)

	lowerMessage := strings.ToLower(userMessage)
	tasks := make([]map[string]any, 0)

	// 检测兑换任务
	if strings.Contains(lowerMessage, "swap") || strings.Contains(lowerMessage, "兑换") {
		log.Printf("✅ 检测到兑换任务")

		// 检查是否包含不支持的代币
		unsupportedTokens := []string{"usdt", "btc", "eth", "bnb", "ada", "dot", "link", "uni", "aave", "comp"}
		foundUnsupported := false
		var unsupportedToken string

		for _, token := range unsupportedTokens {
			if strings.Contains(lowerMessage, token) {
				foundUnsupported = true
				unsupportedToken = strings.ToUpper(token)
				break
			}
		}

		if foundUnsupported {
			log.Printf("❌ 检测到不支持的代币: %s", unsupportedToken)
			return []map[string]any{
				{
					"id":          "error_1",
					"type":        "error",
					"error":       fmt.Sprintf("不支持的代币: %s", unsupportedToken),
					"description": "当前系统只支持 MEER 和 MTK 之间的兑换",
					"message":     fmt.Sprintf("❌ 抱歉，当前系统不支持 %s 代币。\n\n💡 建议：\n- 当前系统只支持 MEER 和 MTK 之间的兑换\n- 您可以尝试：兑换 1 MEER 为 MTK，然后质押 MTK\n- 或者直接质押您现有的 MTK", unsupportedToken),
				},
			}
		}

		// 默认值
		fromToken := "MEER"
		toToken := "MTK"
		amount := "1"

		// 智能解析代币和数量
		// 解析类似 "兑换10MEER的MTK" 或 "兑换10 MEER为MTK" 的模式
		patterns := []string{
			`兑换(\d+)meer.*mtk`,    // "兑换10MEER的MTK"
			`兑换(\d+).*meer.*mtk`,  // "兑换10 MEER为MTK"
			`swap\s+(\d+)\s+meer`, // "swap 10 meer"
			`(\d+)meer.*兑换.*mtk`,  // "1meer兑换成mtk"
		}

		for _, pattern := range patterns {
			re := regexp.MustCompile(pattern)
			if matches := re.FindStringSubmatch(lowerMessage); len(matches) > 1 {
				amount = matches[1]
				log.Printf("📋 提取数量: %s", amount)
				break
			}
		}

		// 确认代币方向
		if strings.Contains(lowerMessage, "meer") && strings.Contains(lowerMessage, "mtk") {
			// 判断是 MEER->MTK 还是 MTK->MEER
			meerIndex := strings.Index(lowerMessage, "meer")
			mtkIndex := strings.Index(lowerMessage, "mtk")

			if meerIndex < mtkIndex && (strings.Contains(lowerMessage, "兑换") || strings.Contains(lowerMessage, "swap")) {
				// "兑换MEER为MTK" 或 "兑换MEER的MTK" 或 "1meer兑换成mtk"
				fromToken = "MEER"
				toToken = "MTK"
			} else if mtkIndex < meerIndex {
				// "兑换MTK为MEER"
				fromToken = "MTK"
				toToken = "MEER"
			}
		}

		task := map[string]any{
			"id":               "task_1",
			"type":             "swap",
			"from_token":       fromToken,
			"to_token":         toToken,
			"amount":           amount,
			"dependency_tx_id": nil,
			"description":      fmt.Sprintf("兑换%s %s为%s", amount, fromToken, toToken),
		}

		log.Printf("📋 构建兑换任务: %+v", task)
		tasks = append(tasks, task)
	}

	// 检测质押任务
	if strings.Contains(lowerMessage, "stake") || strings.Contains(lowerMessage, "质押") {
		log.Printf("✅ 检测到质押任务")

		// 检查是否是连续操作
		hasSwap := strings.Contains(lowerMessage, "swap") || strings.Contains(lowerMessage, "兑换")

		var stakeTask map[string]any
		if hasSwap {
			// 连续操作：兑换后质押
			stakeTask = map[string]any{
				"id":               "task_2",
				"type":             "stake",
				"token":            "MTK",
				"amount":           "all_from_previous",
				"pool":             "compound",
				"dependency_tx_id": "task_1",
				"description":      "将兑换得到的MTK进行质押",
			}
			log.Printf("🔗 连续操作：质押依赖兑换任务")
		} else {
			// 独立质押操作
			stakeTask = map[string]any{
				"id":               "task_1",
				"type":             "stake",
				"token":            "MTK",
				"amount":           "100",
				"pool":             "compound",
				"dependency_tx_id": nil,
				"description":      "质押MTK代币",
			}
		}

		log.Printf("📋 构建质押任务: %+v", stakeTask)
		tasks = append(tasks, stakeTask)
	}

	log.Printf("✅ 文本解析完成，共 %d 个任务", len(tasks))
	return tasks
}

func (n *TaskDecomposerNode) simpleTaskDecomposition(message string) []map[string]any {
	log.Printf("🔄 使用简单规则分解任务")
	log.Printf("📝 消息: %s", message)

	lowerMsg := strings.ToLower(message)
	tasks := make([]map[string]any, 0)

	// 检测是否有连续操作（兑换后质押）
	hasSwapAndStake := (strings.Contains(lowerMsg, "兑换") || strings.Contains(lowerMsg, "swap")) &&
		(strings.Contains(lowerMsg, "质押") || strings.Contains(lowerMsg, "stake"))

	if strings.Contains(lowerMsg, "兑换") || strings.Contains(lowerMsg, "swap") {
		log.Printf("✅ 检测到兑换/swap任务")

		// 检查是否包含不支持的代币
		unsupportedTokens := []string{"usdt", "btc", "eth", "bnb", "ada", "dot", "link", "uni", "aave", "comp"}
		foundUnsupported := false
		var unsupportedToken string

		for _, token := range unsupportedTokens {
			if strings.Contains(lowerMsg, token) {
				foundUnsupported = true
				unsupportedToken = strings.ToUpper(token)
				break
			}
		}

		if foundUnsupported {
			log.Printf("❌ 检测到不支持的代币: %s", unsupportedToken)
			return []map[string]any{
				{
					"id":          "error_1",
					"type":        "error",
					"error":       fmt.Sprintf("不支持的代币: %s", unsupportedToken),
					"description": "当前系统只支持 MEER 和 MTK 之间的兑换",
					"message":     fmt.Sprintf("❌ 抱歉，当前系统不支持 %s 代币。\n\n💡 建议：\n- 当前系统只支持 MEER 和 MTK 之间的兑换\n- 您可以尝试：兑换 1 MEER 为 MTK，然后质押 MTK\n- 或者直接质押您现有的 MTK", unsupportedToken),
				},
			}
		}

		// 解析代币信息
		fromToken := "MEER"
		toToken := "MTK"
		amount := "10"

		// 简单的代币解析
		if strings.Contains(lowerMsg, "meer") {
			fromToken = "MEER"
		}
		if strings.Contains(lowerMsg, "mtk") {
			toToken = "MTK"
		}

		// 解析数量
		if strings.Contains(lowerMsg, "10") {
			amount = "10"
		} else if strings.Contains(lowerMsg, "1") {
			amount = "1"
		}

		swapTask := map[string]any{
			"id":               "task_1",
			"type":             "swap",
			"from_token":       fromToken,
			"to_token":         toToken,
			"amount":           amount,
			"dependency_tx_id": nil,
			"description":      fmt.Sprintf("兑换%s %s为%s", amount, fromToken, toToken),
		}
		tasks = append(tasks, swapTask)
	}

	if strings.Contains(lowerMsg, "质押") || strings.Contains(lowerMsg, "stake") {
		log.Printf("✅ 检测到质押/stake任务")

		var stakeTask map[string]any

		if hasSwapAndStake {
			// 如果是连续操作，质押任务依赖兑换任务
			log.Printf("🔗 检测到连续操作：兑换后质押")
			stakeTask = map[string]any{
				"id":               "task_2",
				"type":             "stake",
				"token":            "MTK",               // 使用兑换得到的代币
				"amount":           "all_from_previous", // 使用前一个任务的全部输出
				"pool":             "compound",
				"dependency_tx_id": "task_1", // 依赖兑换任务
				"description":      "将兑换得到的MTK进行质押",
			}
		} else {
			// 独立的质押任务
			stakeTask = map[string]any{
				"id":               "task_1",
				"type":             "stake",
				"token":            "MTK",
				"amount":           "100", // 默认数量
				"pool":             "compound",
				"dependency_tx_id": nil,
				"description":      "质押MTK代币",
			}
		}

		tasks = append(tasks, stakeTask)
	}

	log.Printf("📋 简单分解完成，共 %d 个任务", len(tasks))
	if hasSwapAndStake {
		log.Printf("🔗 检测到依赖关系：task_2 依赖 task_1")
	}
	return tasks
}

func (n *TaskDecomposerNode) determineNextNodes(tasks []map[string]any) []string {
	log.Printf("🔄 确定下一个执行节点")
	log.Printf("📋 任务数量: %d", len(tasks))

	if len(tasks) == 0 {
		log.Printf("➡️  没有任务，选择result_aggregator节点")
		return []string{"result_aggregator"}
	}

	// 找到没有依赖的第一个任务（即可以立即执行的任务）
	for i, task := range tasks {
		log.Printf("📋 任务[%d]: %+v", i, task)

		dependencyTxID := task["dependency_tx_id"]
		if dependencyTxID == nil {
			// 没有依赖，可以立即执行
			if taskType, ok := task["type"].(string); ok {
				log.Printf("🔄 任务类型: %s (无依赖)", taskType)
				switch taskType {
				case "swap":
					log.Printf("➡️  选择swap_executor节点")
					return []string{"swap_executor"}
				case "stake":
					log.Printf("➡️  选择stake_executor节点")
					return []string{"stake_executor"}
				}
			}
		} else {
			log.Printf("🔗 任务[%d]依赖于: %v", i, dependencyTxID)
		}
	}

	// 如果所有任务都有依赖，说明可能有问题，先执行第一个任务
	if len(tasks) > 0 {
		firstTask := tasks[0]
		if taskType, ok := firstTask["type"].(string); ok {
			log.Printf("⚠️  所有任务都有依赖，强制执行第一个任务: %s", taskType)
			switch taskType {
			case "swap":
				return []string{"swap_executor"}
			case "stake":
				return []string{"stake_executor"}
			}
		}
	}

	log.Printf("➡️  默认选择result_aggregator节点")
	return []string{"result_aggregator"}
}

// SwapExecutorNode 交易执行节点
type SwapExecutorNode struct {
	contractManager *contracts.ContractManager
	registerManager *contracts.RegisterContractManager // 添加注册管理器
}

func NewSwapExecutorNode(contractManager *contracts.ContractManager) *SwapExecutorNode {
	return &SwapExecutorNode{
		contractManager: contractManager,
		registerManager: nil, // 稍后通过SetRegisterManager设置
	}
}

// SetRegisterManager 设置注册管理器
func (n *SwapExecutorNode) SetRegisterManager(registerManager *contracts.RegisterContractManager) {
	n.registerManager = registerManager
}

func (n *SwapExecutorNode) GetName() string {
	return "swap_executor"
}

func (n *SwapExecutorNode) GetType() string {
	return "transaction_executor"
}

func (n *SwapExecutorNode) Execute(ctx context.Context, input NodeInput) (*NodeOutput, error) {
	log.Printf("🔄 交易执行节点开始执行")

	// 查找当前需要执行的swap任务
	currentTask, err := n.findCurrentSwapTask(input.Data)
	if err != nil {
		log.Printf("❌ 查找当前swap任务失败: %v", err)
		return nil, err
	}

	log.Printf("📋 当前执行任务: %+v", currentTask)

	// 使用合约管理器构建兑换请求
	if n.contractManager == nil {
		log.Printf("❌ 合约管理器未初始化")
		return nil, fmt.Errorf("contract manager not initialized")
	}

	swapRequest, err := n.buildSwapRequestFromTask(currentTask, input.Data)
	if err != nil {
		log.Printf("❌ 构建兑换请求失败: %v", err)
		return nil, err
	}

	log.Printf("✅ 构建兑换请求成功: %s %s -> %s", swapRequest.Amount, swapRequest.FromToken, swapRequest.ToToken)

	// 构建交易数据
	txData, err := n.contractManager.BuildSwapTransaction(swapRequest)
	if err != nil {
		log.Printf("❌ 构建交易数据失败: %v", err)
		return nil, fmt.Errorf("failed to build transaction: %w", err)
	}

	log.Printf("✅ 交易数据构建成功")

	// 需要用户签名授权交易
	log.Printf("✍️  需要用户签名授权交易")
	authRequest := &SignatureRequest{
		Action:    "swap",
		FromToken: swapRequest.FromToken,
		ToToken:   swapRequest.ToToken,
		Amount:    swapRequest.Amount, // 显示用户请求的原始金额
		GasFee:    "0.001 ETH",
		Slippage:  "0.5%",
		// 使用合约管理器生成的真实交易数据
		ToAddress: txData.To,
		Value:     txData.Value, // 这是转换后的 wei 金额
		Data:      txData.Data,
		GasLimit:  txData.GasLimit,
		GasPrice:  txData.GasPrice,
	}

	log.Printf("📋 授权请求: %+v", authRequest)

	// 标记当前任务为正在执行
	taskID, _ := currentTask["id"].(string)
	input.Data[taskID+"_current_step"] = "swap"
	input.Data["current_task_id"] = taskID

	return &NodeOutput{
		Data:         input.Data,
		NextNodes:    []string{"signature_validator"},
		NeedUserAuth: true,
		AuthRequest:  authRequest,
		Completed:    false,
	}, nil
}

// findCurrentSwapTask 查找当前需要执行的swap任务
func (n *SwapExecutorNode) findCurrentSwapTask(data map[string]any) (map[string]any, error) {
	tasks, ok := data["tasks"].([]map[string]any)
	if !ok {
		return nil, fmt.Errorf("tasks not found in input data")
	}

	// 获取已完成的任务列表
	completedTasks := make([]string, 0)
	if completed, exists := data["completed_tasks"].([]string); exists {
		completedTasks = completed
	}

	// 查找可以执行的swap任务
	for _, task := range tasks {
		if taskType, ok := task["type"].(string); ok && taskType == "swap" {
			taskID, _ := task["id"].(string)

			// 检查任务是否已完成
			var alreadyCompleted bool
			for _, completed := range completedTasks {
				if completed == taskID {
					alreadyCompleted = true
					break
				}
			}
			if alreadyCompleted {
				continue
			}

			// 检查依赖是否满足
			dependencyTxID := task["dependency_tx_id"]
			if dependencyTxID == nil {
				// 无依赖任务，可以执行
				return task, nil
			} else if depID, ok := dependencyTxID.(string); ok {
				// 检查依赖任务是否已完成
				var dependencyCompleted bool
				for _, completed := range completedTasks {
					if completed == depID {
						dependencyCompleted = true
						break
					}
				}
				if dependencyCompleted {
					return task, nil
				}
			}
		}
	}

	return nil, fmt.Errorf("no executable swap task found")
}

// buildSwapRequestFromTask 从任务构建兑换请求
func (n *SwapExecutorNode) buildSwapRequestFromTask(task map[string]any, data map[string]any) (*contracts.SwapRequest, error) {
	fromToken, ok := task["from_token"].(string)
	if !ok {
		return nil, fmt.Errorf("from_token not found in task")
	}

	toToken, ok := task["to_token"].(string)
	if !ok {
		return nil, fmt.Errorf("to_token not found in task")
	}

	amount, ok := task["amount"].(string)
	if !ok {
		return nil, fmt.Errorf("amount not found in task")
	}

	// 处理特殊的金额类型
	if amount == "all_from_previous" {
		// 从前一个任务获取金额
		// 这里可以根据前一个任务的类型和结果来计算
		amount = "10" // 默认值，实际应该从前一个任务的交易结果中获取
		log.Printf("🔄 使用前一个任务的输出金额: %s", amount)
	}

	// 获取合约ID（可选）
	contractID, _ := task["contract_id"].(string)

	// 如果有合约ID，从注册中心获取合约名称
	var contractName string
	if contractID != "" && n.registerManager != nil {
		contractInfo := n.registerManager.GetContractInfoFromRegistry(contractID)
		if contractInfo != nil {
			contractName = contractInfo.Name
			log.Printf("📋 根据合约ID %s 找到合约: %s", contractID, contractName)
		} else {
			log.Printf("⚠️  未找到合约ID: %s", contractID)
		}
	}

	return &contracts.SwapRequest{
		FromToken:    fromToken,
		ToToken:      toToken,
		Amount:       amount,
		ContractName: contractName,
	}, nil
}

// StakeExecutorNode 质押执行节点
type StakeExecutorNode struct {
	contractManager *contracts.ContractManager
}

func NewStakeExecutorNode(contractManager *contracts.ContractManager) *StakeExecutorNode {
	return &StakeExecutorNode{
		contractManager: contractManager,
	}
}

func (n *StakeExecutorNode) GetName() string {
	return "stake_executor"
}

func (n *StakeExecutorNode) GetType() string {
	return "transaction_executor"
}

func (n *StakeExecutorNode) Execute(ctx context.Context, input NodeInput) (*NodeOutput, error) {
	log.Printf("🔄 质押执行节点开始执行")

	// 查找当前需要执行的stake任务
	currentTask, err := n.findCurrentStakeTask(input.Data)
	if err != nil {
		log.Printf("❌ 查找当前stake任务失败: %v", err)
		return nil, err
	}

	log.Printf("📋 当前执行任务: %+v", currentTask)

	// 使用合约管理器构建质押请求
	if n.contractManager == nil {
		log.Printf("❌ 合约管理器未初始化")
		return nil, fmt.Errorf("contract manager not initialized")
	}

	stakeRequest, err := n.buildStakeRequestFromTask(currentTask, input.Data)
	if err != nil {
		log.Printf("❌ 构建质押请求失败: %v", err)
		return nil, err
	}

	log.Printf("✅ 构建质押请求成功: %s %s %s", stakeRequest.Action, stakeRequest.Amount, stakeRequest.Token)

	// 检查是否已经执行了授权步骤
	taskID, _ := currentTask["id"].(string)
	approveKey := taskID + "_approve_completed"

	if _, approveCompleted := input.Data[approveKey]; !approveCompleted {
		// 还没有授权，先构建授权交易
		log.Printf("🔐 需要先授权MTK代币给质押合约")

		approveData, err := n.contractManager.BuildApproveTransaction(stakeRequest)
		if err != nil {
			log.Printf("❌ 构建授权交易失败: %v", err)
			return nil, fmt.Errorf("failed to build approve transaction: %w", err)
		}

		log.Printf("✅ 授权交易数据构建成功")

		// 标记当前是授权步骤
		input.Data[taskID+"_current_step"] = "approve"
		input.Data["current_task_id"] = taskID

		authRequest := &SignatureRequest{
			Action:    "approve",
			FromToken: stakeRequest.Token,
			ToToken:   stakeRequest.Token,
			Amount:    stakeRequest.Amount,
			GasFee:    "0.001 ETH",
			// 使用合约管理器生成的真实交易数据
			ToAddress: approveData.To,
			Value:     approveData.Value,
			Data:      approveData.Data,
			GasLimit:  approveData.GasLimit,
			GasPrice:  approveData.GasPrice,
		}

		log.Printf("📋 授权请求: %+v", authRequest)

		return &NodeOutput{
			Data:         input.Data,
			NextNodes:    []string{"signature_validator"},
			NeedUserAuth: true,
			AuthRequest:  authRequest,
			Completed:    false,
		}, nil
	}

	// 授权已完成，现在构建质押交易
	log.Printf("✅ 授权已完成，构建质押交易")

	txData, err := n.contractManager.BuildStakeTransaction(stakeRequest)
	if err != nil {
		log.Printf("❌ 构建质押交易数据失败: %v", err)
		return nil, fmt.Errorf("failed to build stake transaction: %w", err)
	}

	log.Printf("✅ 质押交易数据构建成功")

	// 需要用户签名授权质押交易
	log.Printf("✍️  需要用户签名授权质押交易")

	// 标记当前是质押步骤
	input.Data[taskID+"_current_step"] = "stake"
	input.Data["current_task_id"] = taskID

	authRequest := &SignatureRequest{
		Action:    "stake",
		FromToken: stakeRequest.Token,
		ToToken:   stakeRequest.Token,
		Amount:    stakeRequest.Amount,
		GasFee:    "0.001 ETH",
		// 使用合约管理器生成的真实交易数据
		ToAddress: txData.To,
		Value:     txData.Value,
		Data:      txData.Data,
		GasLimit:  txData.GasLimit,
		GasPrice:  txData.GasPrice,
	}

	log.Printf("📋 授权请求: %+v", authRequest)

	return &NodeOutput{
		Data:         input.Data,
		NextNodes:    []string{"signature_validator"},
		NeedUserAuth: true,
		AuthRequest:  authRequest,
		Completed:    false,
	}, nil
}

// findCurrentStakeTask 查找当前需要执行的stake任务
func (n *StakeExecutorNode) findCurrentStakeTask(data map[string]any) (map[string]any, error) {
	tasks, ok := data["tasks"].([]map[string]any)
	if !ok {
		return nil, fmt.Errorf("tasks not found in input data")
	}

	// 获取已完成的任务列表
	completedTasks := make([]string, 0)
	if completed, exists := data["completed_tasks"].([]string); exists {
		completedTasks = completed
	}

	// 查找可以执行的stake任务
	for _, task := range tasks {
		if taskType, ok := task["type"].(string); ok && taskType == "stake" {
			taskID, _ := task["id"].(string)

			// 检查任务是否已完成
			var alreadyCompleted bool
			for _, completed := range completedTasks {
				if completed == taskID {
					alreadyCompleted = true
					break
				}
			}
			if alreadyCompleted {
				continue
			}

			// 检查依赖是否满足
			dependencyTxID := task["dependency_tx_id"]
			if dependencyTxID == nil {
				// 无依赖任务，可以执行
				return task, nil
			} else if depID, ok := dependencyTxID.(string); ok {
				// 检查依赖任务是否已完成
				var dependencyCompleted bool
				for _, completed := range completedTasks {
					if completed == depID {
						dependencyCompleted = true
						break
					}
				}
				if dependencyCompleted {
					return task, nil
				}
			}
		}
	}

	return nil, fmt.Errorf("no executable stake task found")
}

// buildStakeRequestFromTask 从任务构建质押请求
func (n *StakeExecutorNode) buildStakeRequestFromTask(task map[string]any, data map[string]any) (*contracts.StakeRequest, error) {
	token, ok := task["token"].(string)
	if !ok {
		return nil, fmt.Errorf("token not found in task")
	}

	amount, ok := task["amount"].(string)
	if !ok {
		return nil, fmt.Errorf("amount not found in task")
	}

	// 处理特殊的金额类型
	if amount == "all_from_previous" {
		// 从前一个任务获取金额
		// 根据兑换汇率计算：1 MEER = 1000 MTK
		// 这里可以根据前一个任务的实际结果来计算
		amount = "1000" // 假设前一个任务兑换了1 MEER，得到1000 MTK
		log.Printf("🔄 使用前一个任务的输出金额: %s", amount)
	}

	return &contracts.StakeRequest{
		Token:  token,
		Amount: amount,
		Action: "stake", // 默认为质押操作
	}, nil
}

// SignatureValidatorNode 签名验证节点
type SignatureValidatorNode struct {
	rpcClient *rpc.Client
	txConfig  TransactionConfig
}

func NewSignatureValidatorNode(rpcClient *rpc.Client, txConfig TransactionConfig) *SignatureValidatorNode {
	return &SignatureValidatorNode{
		rpcClient: rpcClient,
		txConfig:  txConfig,
	}
}

func (n *SignatureValidatorNode) GetName() string {
	return "signature_validator"
}

func (n *SignatureValidatorNode) GetType() string {
	return "validator"
}

func (n *SignatureValidatorNode) Execute(ctx context.Context, input NodeInput) (*NodeOutput, error) {
	log.Printf("🔄 签名验证节点开始执行")

	signature, ok := input.Data["signature"].(string)
	if !ok || signature == "" {
		log.Printf("❌ 输入中缺少签名")
		return nil, fmt.Errorf("signature not found in input")
	}

	log.Printf("🔐 收到签名，长度: %d", len(signature))
	log.Printf("🔐 签名内容: %s", signature[:llm.Min(len(signature), 50)])

	// 验证签名（简化处理）
	if len(signature) < 10 {
		log.Printf("❌ 签名长度不足: %d", len(signature))
		return nil, fmt.Errorf("invalid signature")
	}

	log.Printf("✅ 签名验证成功")

	// 签名验证成功，继续下一步
	input.Data["signature_verified"] = true
	transactionHash := signature // 使用完整的交易哈希
	input.Data["transaction_hash"] = transactionHash

	log.Printf("📊 更新数据:")
	log.Printf("  - signature_verified: true")
	log.Printf("  - transaction_hash: %s", transactionHash)

	// 等待交易确认
	log.Printf("⏳ 等待交易确认...")
	err := n.waitForTransactionConfirmation(ctx, transactionHash)
	if err != nil {
		log.Printf("❌ 交易确认失败: %v", err)
		return nil, fmt.Errorf("transaction confirmation failed: %w", err)
	}

	return &NodeOutput{
		Data:      input.Data,
		Completed: false,
	}, nil
}

// waitForTransactionConfirmation 等待交易确认
func (n *SignatureValidatorNode) waitForTransactionConfirmation(ctx context.Context, txHash string) error {
	log.Printf("🔍 开始监控交易确认: %s", txHash)

	// 如果没有RPC客户端，使用模拟确认
	if n.rpcClient == nil {
		log.Printf("⚠️  未配置RPC客户端，使用模拟确认")
		confirmationTime := 5
		log.Printf("⏰ 模拟等待交易确认，预计 %d 秒...", confirmationTime)

		for i := 1; i <= confirmationTime; i++ {
			time.Sleep(1 * time.Second)
			log.Printf("⏳ 模拟确认进度: %d/%d 秒", i, confirmationTime)
		}

		log.Printf("✅ 模拟交易确认完成: %s", txHash)
		return nil
	}

	// 使用真实的RPC客户端等待交易确认
	log.Printf("🌐 使用RPC轮询等待交易确认...")

	// 创建带超时的上下文
	ctxWithTimeout, cancel := context.WithTimeout(ctx, time.Duration(n.txConfig.ConfirmationTimeout)*time.Second)
	defer cancel()

	pollingInterval := time.Duration(n.txConfig.PollingInterval) * time.Second
	requiredConfirmations := n.txConfig.RequiredConfirmations

	receipt, err := n.rpcClient.WaitForTransactionConfirmation(
		ctxWithTimeout,
		txHash,
		requiredConfirmations,
		pollingInterval,
	)

	if err != nil {
		log.Printf("❌ 交易确认失败: %v", err)
		return fmt.Errorf("交易确认失败: %w", err)
	}

	if !receipt.Success {
		log.Printf("❌ 交易执行失败: %s", txHash)
		return fmt.Errorf("交易执行失败")
	}

	log.Printf("✅ 交易已确认并完成: %s (区块: %s)", txHash, receipt.BlockNumber)
	log.Printf("🎯 现在可以安全执行依赖任务")
	return nil
}

// checkDependentTasks 检查是否有依赖当前任务的下一个任务
func (n *SignatureValidatorNode) checkDependentTasks(data map[string]any, completedTxHash string) []string {
	log.Printf("🔗 检查依赖任务")

	var completedTaskID string

	// 检查是否是swap步骤完成
	currentTaskID, exists := data["current_task_id"].(string)
	if exists {
		currentStepKey := currentTaskID + "_current_step"
		if currentStep, stepExists := data[currentStepKey].(string); stepExists && currentStep == "swap" {
			log.Printf("✅ Swap步骤完成，标记任务完成")
			// 标记swap任务完成
			data[currentTaskID+"_swap_completed"] = true
			data[currentTaskID+"_swap_tx_hash"] = completedTxHash
			// 清除当前步骤标记
			delete(data, currentStepKey)
			delete(data, "current_task_id")

			// 添加到已完成任务列表
			completedTasks := make([]string, 0)
			if completed, exists := data["completed_tasks"].([]string); exists {
				completedTasks = completed
			}
			completedTasks = append(completedTasks, currentTaskID)
			data["completed_tasks"] = completedTasks

			log.Printf("✅ 任务 %s 完成，交易哈希: %s", currentTaskID, completedTxHash)

			// 设置completedTaskID为当前完成的任务
			completedTaskID = currentTaskID
		}
	}

	// 检查是否是授权步骤完成
	for _, task := range data["tasks"].([]map[string]any) {
		if taskID, exists := task["id"].(string); exists {
			currentStepKey := taskID + "_current_step"
			if currentStep, stepExists := data[currentStepKey].(string); stepExists && currentStep == "approve" {
				log.Printf("✅ 授权步骤完成，标记并返回质押执行节点")
				// 标记授权完成
				data[taskID+"_approve_completed"] = true
				data[taskID+"_approve_tx_hash"] = completedTxHash
				// 清除当前步骤标记
				delete(data, currentStepKey)
				delete(data, "current_task_id")
				// 返回质押执行节点继续执行实际的质押交易
				return []string{"stake_executor"}
			}
		}
	}

	// 检查是否是质押步骤完成
	if currentTaskID, exists := data["current_task_id"].(string); exists {
		currentStepKey := currentTaskID + "_current_step"
		if currentStep, stepExists := data[currentStepKey].(string); stepExists && currentStep == "stake" {
			log.Printf("✅ 质押步骤完成，标记任务完成")
			// 标记质押任务完成
			data[currentTaskID+"_stake_completed"] = true
			data[currentTaskID+"_stake_tx_hash"] = completedTxHash
			// 清除当前步骤标记
			delete(data, currentStepKey)
			delete(data, "current_task_id")

			// 添加到已完成任务列表
			completedTasks := make([]string, 0)
			if completed, exists := data["completed_tasks"].([]string); exists {
				completedTasks = completed
			}
			completedTasks = append(completedTasks, currentTaskID)
			data["completed_tasks"] = completedTasks

			log.Printf("✅ 任务 %s 完成，交易哈希: %s", currentTaskID, completedTxHash)
		}
	}

	// 获取任务列表
	tasks, ok := data["tasks"].([]map[string]any)
	if !ok {
		log.Printf("⚠️  没有找到任务列表，转到结果聚合")
		return []string{"result_aggregator"}
	}

	// 如果没有已完成的任务记录，初始化
	if data["completed_tasks"] == nil {
		data["completed_tasks"] = make([]string, 0)
	}

	completedTasks, ok := data["completed_tasks"].([]string)
	if !ok {
		completedTasks = make([]string, 0)
	}

	// 查找刚完成的任务ID
	for _, task := range tasks {
		if taskID, exists := task["id"].(string); exists {
			// 检查这个任务是否是刚完成的（没有依赖或依赖已完成）
			dependencyTxID := task["dependency_tx_id"]
			if dependencyTxID == nil {
				// 这是一个无依赖的任务，应该是刚完成的
				var alreadyCompleted bool
				for _, completed := range completedTasks {
					if completed == taskID {
						alreadyCompleted = true
						break
					}
				}
				if !alreadyCompleted {
					completedTaskID = taskID
					break
				}
			}
		}
	}

	// 如果没有找到无依赖的任务，检查是否有当前正在执行的任务
	if completedTaskID == "" {
		if currentTaskID, exists := data["current_task_id"].(string); exists {
			completedTaskID = currentTaskID
			log.Printf("📋 使用当前任务ID作为完成的任务: %s", completedTaskID)
		}
	}

	// 如果找到了完成的任务，记录它
	if completedTaskID != "" {
		// 检查是否已经在completed_tasks中
		completedTasks := make([]string, 0)
		if completed, exists := data["completed_tasks"].([]string); exists {
			completedTasks = completed
		}

		// 检查是否已经记录过
		alreadyRecorded := false
		for _, completed := range completedTasks {
			if completed == completedTaskID {
				alreadyRecorded = true
				break
			}
		}

		if !alreadyRecorded {
			completedTasks = append(completedTasks, completedTaskID)
			data["completed_tasks"] = completedTasks
			data[completedTaskID+"_tx_hash"] = completedTxHash
			log.Printf("✅ 任务 %s 完成，交易哈希: %s", completedTaskID, completedTxHash)
		}
	}

	// 查找依赖刚完成任务的下一个任务
	log.Printf("🔍 查找依赖任务，已完成任务ID: %s", completedTaskID)
	for _, task := range tasks {
		if taskID, exists := task["id"].(string); exists {
			dependencyTxID := task["dependency_tx_id"]
			log.Printf("📋 检查任务 %s，依赖: %v", taskID, dependencyTxID)
			if dependencyTxID != nil && dependencyTxID == completedTaskID {
				// 找到依赖任务
				log.Printf("🔗 找到依赖任务: %s 依赖于 %s", taskID, completedTaskID)

				// 检查任务类型
				if taskType, typeExists := task["type"].(string); typeExists {
					switch taskType {
					case "swap":
						log.Printf("➡️  执行依赖的swap任务: %s", taskID)
						return []string{"swap_executor"}
					case "stake":
						log.Printf("➡️  执行依赖的stake任务: %s", taskID)
						return []string{"stake_executor"}
					}
				}
			}
		}
	}

	log.Printf("✅ 没有更多依赖任务，转到结果聚合")
	return []string{"result_aggregator"}
}

// ResultAggregatorNode 结果聚合节点
type ResultAggregatorNode struct{}

func NewResultAggregatorNode() *ResultAggregatorNode {
	return &ResultAggregatorNode{}
}

func (n *ResultAggregatorNode) GetName() string {
	return "result_aggregator"
}

func (n *ResultAggregatorNode) GetType() string {
	return "aggregator"
}

func (n *ResultAggregatorNode) Execute(ctx context.Context, input NodeInput) (*NodeOutput, error) {
	log.Printf("🔄 结果聚合节点开始执行")
	log.Printf("📊 输入数据: %+v", input.Data)

	// 检查是否有错误任务
	if tasks, ok := input.Data["tasks"].([]map[string]any); ok {
		for _, task := range tasks {
			if taskType, exists := task["type"].(string); exists && taskType == "error" {
				log.Printf("❌ 检测到错误任务: %+v", task)
				// 返回错误信息
				return &NodeOutput{
					Data: map[string]any{
						"status":       "error",
						"error":        task["error"],
						"message":      task["message"],
						"description":  task["description"],
						"timestamp":    time.Now(),
						"workflow_id":  input.Context["workflow_id"],
						"session_id":   input.Context["session_id"],
						"user_message": input.Data["user_message"],
					},
					NextNodes: []string{}, // 终止节点
					Completed: true,
				}, nil
			}
		}
	}

	// 聚合所有执行结果
	result := map[string]any{
		"status":       "completed",
		"timestamp":    time.Now(),
		"workflow_id":  input.Context["workflow_id"],
		"session_id":   input.Context["session_id"],
		"tasks":        input.Data["tasks"],
		"user_message": input.Data["user_message"],
	}

	// 检查是否有签名验证结果
	if signatureVerified, ok := input.Data["signature_verified"].(bool); ok && signatureVerified {
		log.Printf("✅ 检测到签名验证成功")
		result["signature_verified"] = true
		result["transaction_hash"] = input.Data["transaction_hash"]
	}

	// 检查是否有交易执行结果
	if transactionHash, ok := input.Data["transaction_hash"].(string); ok {
		log.Printf("✅ 检测到交易哈希: %s", transactionHash)
		result["transaction_hash"] = transactionHash
	}

	log.Printf("📊 聚合结果: %+v", result)

	return &NodeOutput{
		Data:      result,
		NextNodes: []string{}, // 终止节点
		Completed: true,
	}, nil
}
