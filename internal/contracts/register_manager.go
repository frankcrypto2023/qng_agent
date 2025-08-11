package contracts

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"math/big"
)

// RegisterContractManager 基于注册中心的合约管理器
type RegisterContractManager struct {
	*ContractManager
	registerCoreAddress     string
	registerExtendedAddress string
	contractsFromRegistry   map[string]*RegistryContractInfo
}

// RegistryContractInfo 从注册中心获取的合约信息
type RegistryContractInfo struct {
	AgentID     string     `json:"agentId"`
	Address     string     `json:"address"`
	Name        string     `json:"name"`
	Type        string     `json:"type"`
	Description string     `json:"description"`
	Functions   []string   `json:"functions"`
	Services    []string   `json:"services"`
	Protocols   []string   `json:"protocols"`
	Tags        []string   `json:"tags"`
	Metadata    string     `json:"metadata"`
	Reputation  *big.Int   `json:"reputation"`
	Endpoints   []string   `json:"endpoints"`
	Weights     []*big.Int `json:"weights"`
}

// NewRegisterContractManager 创建基于注册中心的合约管理器
func NewRegisterContractManager(configPath, registerCoreAddress, registerExtendedAddress, rpcURL string) (*RegisterContractManager, error) {
	// 如果configPath为空，使用默认路径
	// if configPath == "" {
	// 	configPath = "config/contracts.json"
	// 	log.Printf("📋 使用默认配置文件路径: %s", configPath)
	// }

	// 首先创建基础的合约管理器
	baseManager, err := NewContractManager(configPath)
	if err != nil {
		log.Printf("⚠️  无法创建基础合约管理器: %v", err)
		// 创建一个空的合约管理器作为后备
		baseManager = &ContractManager{
			config:    &ContractConfig{},
			artifacts: make(map[string]*ContractArtifact),
		}
	}

	registerManager := &RegisterContractManager{
		ContractManager:         baseManager,
		registerCoreAddress:     registerCoreAddress,
		registerExtendedAddress: registerExtendedAddress,
		contractsFromRegistry:   make(map[string]*RegistryContractInfo),
	}

	// 尝试从注册中心加载合约信息
	if err := registerManager.loadContractsFromRegistry(rpcURL); err != nil {
		log.Printf("⚠️  无法从注册中心加载合约信息: %v", err)
		log.Printf("📋 将使用本地配置文件作为后备")
	}

	return registerManager, nil
}

// loadContractsFromRegistry 从注册中心加载合约信息
func (registerManager *RegisterContractManager) loadContractsFromRegistry(rpcURL string) error {
	log.Printf("🔧 从注册中心加载合约信息...")

	// 检查Register合约地址是否配置
	if registerManager.registerCoreAddress == "" || registerManager.registerExtendedAddress == "" {
		log.Printf("⚠️  Register合约地址未配置，使用模拟数据")
		return registerManager.loadContractsFromRegistrySimulated()
	}

	// 尝试从真实的Register合约获取数据
	if err := registerManager.loadContractsFromRegistryReal(rpcURL); err != nil {
		log.Printf("⚠️  从真实Register合约获取数据失败: %v", err)
		log.Printf("📋 回退到模拟数据")
		return registerManager.loadContractsFromRegistrySimulated()
	}

	log.Printf("✅ 从注册中心加载了 %d 个合约", len(registerManager.contractsFromRegistry))
	return nil
}

// loadContractsFromRegistryReal 从真实的Register合约加载数据
func (registerManager *RegisterContractManager) loadContractsFromRegistryReal(rpcURL string) error {
	log.Printf("🔗 连接到Register合约: %s", registerManager.registerCoreAddress)
	log.Printf("🔗 使用RPC URL: %s", rpcURL)

	// 确保contractsFromRegistry被正确初始化
	if registerManager.contractsFromRegistry == nil {
		registerManager.contractsFromRegistry = make(map[string]*RegistryContractInfo)
		log.Printf("✅ 初始化contractsFromRegistry")
	}

	// 创建Register客户端
	client := NewRegisterClient(registerManager.registerCoreAddress, registerManager.registerExtendedAddress, rpcURL)

	// 获取总代理数
	totalAgents, err := client.GetTotalAgents()
	if err != nil {
		return fmt.Errorf("failed to get total agents: %w", err)
	}

	log.Printf("📊 注册中心总代理数: %s", totalAgents.String())

	// 根据服务类型发现代理
	services := []string{"ERC20", "DEX", "Swap", "Staking", "Token"}
	for _, service := range services {
		agents, err := client.DiscoverAgentsByService(service)
		if err != nil {
			log.Printf("⚠️  发现 %s 服务失败: %v", service, err)
			continue
		}

		log.Printf("📋 %s 服务代理: %v", service, agents)

		// 获取每个代理的详细信息
		for _, agentId := range agents {
			details, err := client.GetAgentDetails(agentId)
			if err != nil {
				log.Printf("⚠️  获取代理 %s 详情失败: %v", agentId, err)
				// 即使获取详情失败，也要尝试获取ABI
				details = &RegistryContractInfo{
					AgentID: agentId,
					Name:    agentId,
					Type:    "Contract",
					Address: "0x0000000000000000000000000000000000000000",
				}
			}

			// 获取ABI
			abi, err := client.GetAgentABI(agentId)
			if err != nil {
				log.Printf("⚠️  获取代理 %s ABI失败: %v", agentId, err)
				continue
			}

			// 解析ABI获取函数信息
			var abiData []interface{}
			if err := json.Unmarshal([]byte(abi), &abiData); err != nil {
				log.Printf("⚠️  解析ABI失败: %v", err)
				continue
			}

			// 提取函数信息
			functions := make(map[string]FunctionInfo)
			for _, item := range abiData {
				if funcMap, ok := item.(map[string]interface{}); ok {
					if funcType, ok := funcMap["type"].(string); ok && funcType == "function" {
						if funcName, ok := funcMap["name"].(string); ok {
							functions[funcName] = FunctionInfo{
								Signature:   funcName,
								Description: fmt.Sprintf("Function %s", funcName),
							}
						}
					}
				}
			}

			// 确保基础合约管理器存在
			if registerManager.ContractManager == nil {
				log.Printf("⚠️  ContractManager为nil，跳过合约加载")
				continue
			}

			// 确保config不为nil
			if registerManager.ContractManager.config == nil {
				registerManager.ContractManager.config = &ContractConfig{
					Contracts: make(map[string]ContractInfo),
				}
				log.Printf("✅ 初始化ContractManager.config")
			}

			// 使用合约名称作为键，而不是agentId
			contractKey := details.Name

			// 为不同类型的合约添加SupportedPairs
			var supportedPairs []SwapPair
			if details.Type == "DEX" || details.Name == "SimpleSwap" {
				supportedPairs = []SwapPair{
					{
						From:        "MEER",
						To:          "MTK",
						Method:      "buyToken",
						Rate:        1000,
						Description: "Convert MEER to MTK at 1:1000 rate",
					},
					{
						From:        "MTK",
						To:          "MEER",
						Method:      "sellToken",
						Rate:        0.001,
						Description: "Convert MTK to MEER at 1000:1 rate",
					},
				}
			}

			// 确保基础合约管理器存在且config不为nil
			if registerManager.ContractManager != nil {
				if registerManager.ContractManager.config == nil {
					registerManager.ContractManager.config = &ContractConfig{
						Contracts: make(map[string]ContractInfo),
					}
					log.Printf("✅ 初始化ContractManager.config")
				}

				// 确保Contracts map不为nil
				if registerManager.ContractManager.config.Contracts == nil {
					registerManager.ContractManager.config.Contracts = make(map[string]ContractInfo)
					log.Printf("✅ 初始化ContractManager.config.Contracts")
				}

				registerManager.ContractManager.config.Contracts[contractKey] = ContractInfo{
					Name:           details.Name,
					Address:        details.Address,
					Type:           details.Type,
					Description:    details.Description,
					Functions:      functions,
					SupportedPairs: supportedPairs,
				}

				log.Printf("✅ 从Register合约加载: %s (%s) -> %s", contractKey, agentId, details.Address)
			} else {
				log.Printf("⚠️  ContractManager为nil，跳过基础管理器更新")
			}

			// 确保contractsFromRegistry不为nil
			if registerManager.contractsFromRegistry == nil {
				registerManager.contractsFromRegistry = make(map[string]*RegistryContractInfo)
				log.Printf("✅ 初始化contractsFromRegistry (延迟初始化)")
			}

			// 保存到注册中心缓存
			registerManager.contractsFromRegistry[agentId] = details
		}
	}

	return nil
}

// loadContractsFromRegistrySimulated 使用模拟数据加载合约信息
func (registerManager *RegisterContractManager) loadContractsFromRegistrySimulated() error {
	// 确保contractsFromRegistry被正确初始化
	if registerManager.contractsFromRegistry == nil {
		registerManager.contractsFromRegistry = make(map[string]*RegistryContractInfo)
		log.Printf("✅ 初始化contractsFromRegistry (模拟模式)")
	}

	// 读取 deployed.json 获取合约地址
	deployedData, err := registerManager.readDeployedData()
	if err != nil {
		return fmt.Errorf("failed to read deployed data: %w", err)
	}

	// 读取 contracts.json 获取合约配置
	contractsConfig, err := registerManager.readContractsConfig()
	if err != nil {
		return fmt.Errorf("failed to read contracts config: %w", err)
	}

	// 模拟从注册中心获取的合约信息
	registerManager.simulateRegistryContracts(deployedData, contractsConfig)
	return nil
}

// readDeployedData 读取 deployed.json
func (registerManager *RegisterContractManager) readDeployedData() (map[string]string, error) {
	data, err := ioutil.ReadFile("deployed.json")
	if err != nil {
		return nil, err
	}

	var deployedData map[string]string
	if err := json.Unmarshal(data, &deployedData); err != nil {
		return nil, err
	}

	return deployedData, nil
}

// readContractsConfig 读取 contracts.json
func (registerManager *RegisterContractManager) readContractsConfig() (*ContractConfig, error) {
	data, err := ioutil.ReadFile("config/contracts.json")
	if err != nil {
		return nil, err
	}

	var config ContractConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

// simulateRegistryContracts 模拟从注册中心获取合约信息
func (registerManager *RegisterContractManager) simulateRegistryContracts(deployedData map[string]string, contractsConfig *ContractConfig) {
	// 定义合约服务类型映射
	contractServices := map[string][]string{
		"MyToken":    {"ERC20", "Token", "Transfer"},
		"SimpleSwap": {"DEX", "Swap", "Exchange"},
		"MTKStaking": {"Staking", "Rewards", "Yield"},
	}

	// 定义合约协议映射
	contractProtocols := map[string][]string{
		"MyToken":    {"ERC20", "Ethereum"},
		"SimpleSwap": {"Custom", "DEX"},
		"MTKStaking": {"Custom", "Staking"},
	}

	// 定义合约标签映射
	contractTags := map[string][]string{
		"MyToken":    {"token", "erc20", "fungible"},
		"SimpleSwap": {"dex", "swap", "exchange", "liquidity"},
		"MTKStaking": {"staking", "rewards", "yield", "defi"},
	}

	// 处理每个合约
	for contractName, contractAddress := range deployedData {
		// 跳过注册中心合约本身
		if contractName == "RegisterCore" || contractName == "RegisterExtended" {
			continue
		}

		// 获取合约配置
		contractConfig, exists := contractsConfig.Contracts[contractName]
		if !exists {
			log.Printf("⚠️  警告: %s 在 contracts.json 中未找到配置", contractName)
			continue
		}

		// 构建元数据
		metadata := map[string]interface{}{
			"name":         contractConfig.Name,
			"type":         contractConfig.Type,
			"description":  contractConfig.Description,
			"artifactPath": contractConfig.ArtifactPath,
			"functions":    registerManager.getFunctionNames(contractConfig.Functions),
			"version":      contractsConfig.Version,
			"network":      contractsConfig.Network.Name,
		}

		metadataJSON, _ := json.Marshal(metadata)

		// 创建合约信息
		contractInfo := &RegistryContractInfo{
			AgentID:     contractName,
			Address:     contractAddress,
			Name:        contractConfig.Name,
			Type:        contractConfig.Type,
			Description: contractConfig.Description,
			Functions:   registerManager.getFunctionNames(contractConfig.Functions),
			Services:    contractServices[contractName],
			Protocols:   contractProtocols[contractName],
			Tags:        contractTags[contractName],
			Metadata:    string(metadataJSON),
			Reputation:  big.NewInt(50), // 默认声誉值
			Endpoints:   []string{contractAddress},
		}

		registerManager.contractsFromRegistry[contractName] = contractInfo
		log.Printf("📋 注册合约: %s -> %s", contractName, contractAddress)

		// 同时更新基础合约管理器中的合约信息
		if registerManager.ContractManager != nil {
			// 确保合约配置存在
			if registerManager.ContractManager.config == nil {
				registerManager.ContractManager.config = &ContractConfig{
					Contracts: make(map[string]ContractInfo),
				}
			}

			// 添加合约到基础管理器
			registerManager.ContractManager.config.Contracts[contractName] = ContractInfo{
				Name:           contractConfig.Name,
				Address:        contractAddress,
				ArtifactPath:   contractConfig.ArtifactPath,
				Type:           contractConfig.Type,
				Description:    contractConfig.Description,
				Functions:      contractConfig.Functions,
				SupportedPairs: contractConfig.SupportedPairs, // 添加支持的交换对
			}

			log.Printf("✅ 已添加到基础合约管理器: %s", contractName)
		}
	}

	// 加载所有合约的artifacts
	if registerManager.ContractManager != nil {
		if err := registerManager.ContractManager.LoadArtifacts(); err != nil {
			log.Printf("⚠️  加载artifacts失败: %v", err)
		} else {
			log.Printf("✅ 所有合约artifacts加载成功")
		}
	}
}

// getFunctionNames 获取函数名称列表
func (registerManager *RegisterContractManager) getFunctionNames(functions map[string]FunctionInfo) []string {
	names := make([]string, 0, len(functions))
	for name := range functions {
		names = append(names, name)
	}
	return names
}

// GetContractInfoFromRegistry 从注册中心获取合约信息
func (registerManager *RegisterContractManager) GetContractInfoFromRegistry(name string) *RegistryContractInfo {
	if contract, exists := registerManager.contractsFromRegistry[name]; exists {
		return contract
	}
	return nil
}

// GetContractsByTypeFromRegistry 从注册中心根据类型获取合约
func (registerManager *RegisterContractManager) GetContractsByTypeFromRegistry(contractType string) []*RegistryContractInfo {
	var contracts []*RegistryContractInfo
	for _, contract := range registerManager.contractsFromRegistry {
		if contract.Type == contractType {
			contracts = append(contracts, contract)
		}
	}
	return contracts
}

// GetERC20TokensFromRegistry 从注册中心获取 ERC20 代币
func (registerManager *RegisterContractManager) GetERC20TokensFromRegistry() []*RegistryContractInfo {
	return registerManager.GetContractsByTypeFromRegistry("ERC20")
}

// GetDEXContractsFromRegistry 从注册中心获取 DEX 合约
func (registerManager *RegisterContractManager) GetDEXContractsFromRegistry() []*RegistryContractInfo {
	return registerManager.GetContractsByTypeFromRegistry("DEX")
}

// GetStakingContractsFromRegistry 从注册中心获取 Staking 合约
func (registerManager *RegisterContractManager) GetStakingContractsFromRegistry() []*RegistryContractInfo {
	return registerManager.GetContractsByTypeFromRegistry("Staking")
}

// SearchContractsFromRegistry 从注册中心搜索合约
func (registerManager *RegisterContractManager) SearchContractsFromRegistry(criteria map[string]interface{}) []*RegistryContractInfo {
	var contracts []*RegistryContractInfo

	for _, contract := range registerManager.contractsFromRegistry {
		// 检查服务类型
		if service, ok := criteria["service"].(string); ok {
			found := false
			for _, s := range contract.Services {
				if s == service {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		// 检查协议类型
		if protocol, ok := criteria["protocol"].(string); ok {
			found := false
			for _, p := range contract.Protocols {
				if p == protocol {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		// 检查标签
		if tag, ok := criteria["tag"].(string); ok {
			found := false
			for _, t := range contract.Tags {
				if t == tag {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		contracts = append(contracts, contract)
	}

	return contracts
}

// GetContractAddressByOperationFromRegistry 从注册中心根据操作类型获取合约地址
func (registerManager *RegisterContractManager) GetContractAddressByOperationFromRegistry(operationType string) (string, error) {
	log.Printf("🔍 从注册中心根据操作类型查找合约: %s", operationType)

	// 定义操作类型与服务的映射
	operationServiceMapping := map[string][]string{
		"swap":         {"Swap", "DEX", "Exchange"},
		"stake":        {"Staking", "Rewards"},
		"unstake":      {"Staking", "Rewards"},
		"claimRewards": {"Staking", "Rewards"},
	}

	// 获取支持的操作类型
	supportedServices, exists := operationServiceMapping[operationType]
	if !exists {
		return "", fmt.Errorf("unsupported operation type: %s", operationType)
	}

	// 遍历支持的服务，查找合约
	for _, service := range supportedServices {
		contracts := registerManager.GetContractByServiceFromRegistry(service)
		if len(contracts) > 0 {
			// 选择声誉最高的合约
			var bestContract *RegistryContractInfo
			var highestReputation *big.Int

			for _, contract := range contracts {
				if contract.Reputation != nil && (highestReputation == nil || contract.Reputation.Cmp(highestReputation) > 0) {
					highestReputation = contract.Reputation
					bestContract = contract
				}
			}

			if bestContract != nil {
				log.Printf("✅ 从注册中心找到支持 %s 操作的合约: %s (%s), 声誉: %s",
					operationType, bestContract.AgentID, bestContract.Address, bestContract.Reputation.String())
				return bestContract.Address, nil
			}

			// 如果没有声誉信息，使用第一个合约
			log.Printf("✅ 从注册中心找到支持 %s 操作的合约: %s (%s)",
				operationType, contracts[0].AgentID, contracts[0].Address)
			return contracts[0].Address, nil
		}
	}

	log.Printf("❌ 从注册中心未找到支持 %s 操作的合约", operationType)
	return "", fmt.Errorf("no contract found in registry supporting operation: %s", operationType)
}

// GetContractAddressByOperation 重写基础方法，从注册中心获取合约地址
func (registerManager *RegisterContractManager) GetContractAddressByOperation(operationType string) (string, error) {
	log.Printf("🔍 RegisterContractManager: 根据操作类型查找合约: %s", operationType)

	// 首先尝试从注册中心获取
	if address, err := registerManager.GetContractAddressByOperationFromRegistry(operationType); err == nil {
		return address, nil
	}

	// 如果注册中心没有找到，回退到基础方法
	log.Printf("⚠️  注册中心未找到合约，回退到基础方法")
	return registerManager.ContractManager.GetContractAddressByOperation(operationType)
}

// RefreshFromRegistry 从注册中心刷新合约信息
func (registerManager *RegisterContractManager) RefreshFromRegistry() error {
	log.Printf("🔄 从注册中心刷新合约信息...")
	registerManager.contractsFromRegistry = make(map[string]*RegistryContractInfo)
	// 使用默认RPC URL进行刷新
	return registerManager.loadContractsFromRegistry("http://47.242.255.132:1234/")
}

// GetRegistryStats 获取注册中心统计信息
func (registerManager *RegisterContractManager) GetRegistryStats() map[string]interface{} {
	return map[string]interface{}{
		"totalAgents":             len(registerManager.contractsFromRegistry),
		"registerCoreAddress":     registerManager.registerCoreAddress,
		"registerExtendedAddress": registerManager.registerExtendedAddress,
		"contractTypes": map[string]int{
			"ERC20":   len(registerManager.GetERC20TokensFromRegistry()),
			"DEX":     len(registerManager.GetDEXContractsFromRegistry()),
			"Staking": len(registerManager.GetStakingContractsFromRegistry()),
		},
	}
}

// GetWorkflowDescriptionFromRegistry 从注册中心获取工作流描述
func (registerManager *RegisterContractManager) GetWorkflowDescriptionFromRegistry() string {
	description := "基于注册中心的合约系统信息:\n\n"

	description += "注册的合约:\n"
	for name, contract := range registerManager.contractsFromRegistry {
		description += fmt.Sprintf("- %s (%s): %s\n", name, contract.Type, contract.Description)
		description += fmt.Sprintf("  地址: %s\n", contract.Address)
		description += fmt.Sprintf("  服务: %v\n", contract.Services)
		description += fmt.Sprintf("  协议: %v\n", contract.Protocols)
		description += fmt.Sprintf("  标签: %v\n", contract.Tags)
	}

	description += "\n支持的操作:\n"
	description += "- 代币交换: 通过 DEX 合约进行代币交换\n"
	description += "- 代币质押: 通过 Staking 合约进行代币质押和奖励领取\n"
	description += "- 代币转账: 通过 ERC20 合约进行代币转账\n"

	return description
}

// GetContractCapabilitiesFromRegistry 从注册中心获取合约能力
func (registerManager *RegisterContractManager) GetContractCapabilitiesFromRegistry() map[string]interface{} {
	capabilities := make(map[string]interface{})

	// 获取所有合约类型的能力
	erc20Tokens := registerManager.GetERC20TokensFromRegistry()
	dexContracts := registerManager.GetDEXContractsFromRegistry()
	stakingContracts := registerManager.GetStakingContractsFromRegistry()

	// ERC20 代币能力
	if len(erc20Tokens) > 0 {
		capabilities["erc20"] = map[string]interface{}{
			"count":       len(erc20Tokens),
			"contracts":   erc20Tokens,
			"operations":  []string{"transfer", "approve", "balanceOf", "totalSupply"},
			"description": "ERC20 标准代币合约，支持代币转账、授权、查询余额等操作",
		}
	}

	// DEX 合约能力
	if len(dexContracts) > 0 {
		capabilities["dex"] = map[string]interface{}{
			"count":       len(dexContracts),
			"contracts":   dexContracts,
			"operations":  []string{"swap", "addLiquidity", "removeLiquidity", "getAmountsOut"},
			"description": "去中心化交易所合约，支持代币交换、流动性管理等功能",
		}
	}

	// Staking 合约能力
	if len(stakingContracts) > 0 {
		capabilities["staking"] = map[string]interface{}{
			"count":       len(stakingContracts),
			"contracts":   stakingContracts,
			"operations":  []string{"stake", "unstake", "claimRewards", "getStakedAmount"},
			"description": "质押合约，支持代币质押、解质押、奖励领取等功能",
		}
	}

	return capabilities
}

// GetContractByServiceFromRegistry 根据服务类型获取合约
func (registerManager *RegisterContractManager) GetContractByServiceFromRegistry(service string) []*RegistryContractInfo {
	var contracts []*RegistryContractInfo

	for _, contract := range registerManager.contractsFromRegistry {
		for _, s := range contract.Services {
			if s == service {
				contracts = append(contracts, contract)
				break
			}
		}
	}

	return contracts
}

// GetContractByProtocolFromRegistry 根据协议类型获取合约
func (registerManager *RegisterContractManager) GetContractByProtocolFromRegistry(protocol string) []*RegistryContractInfo {
	var contracts []*RegistryContractInfo

	for _, contract := range registerManager.contractsFromRegistry {
		for _, p := range contract.Protocols {
			if p == protocol {
				contracts = append(contracts, contract)
				break
			}
		}
	}

	return contracts
}

// GetContractByTagFromRegistry 根据标签获取合约
func (registerManager *RegisterContractManager) GetContractByTagFromRegistry(tag string) []*RegistryContractInfo {
	var contracts []*RegistryContractInfo

	for _, contract := range registerManager.contractsFromRegistry {
		for _, t := range contract.Tags {
			if t == tag {
				contracts = append(contracts, contract)
				break
			}
		}
	}

	return contracts
}

// GetContractMetadataFromRegistry 从注册中心获取合约元数据
func (registerManager *RegisterContractManager) GetContractMetadataFromRegistry(contractName string) (map[string]interface{}, error) {
	contract := registerManager.GetContractInfoFromRegistry(contractName)
	if contract == nil {
		return nil, fmt.Errorf("contract %s not found in registry", contractName)
	}

	var metadata map[string]interface{}
	if err := json.Unmarshal([]byte(contract.Metadata), &metadata); err != nil {
		return nil, fmt.Errorf("failed to parse metadata: %w", err)
	}

	return metadata, nil
}

// GetContractFunctionsFromRegistry 从注册中心获取合约函数列表
func (registerManager *RegisterContractManager) GetContractFunctionsFromRegistry(contractName string) ([]string, error) {
	contract := registerManager.GetContractInfoFromRegistry(contractName)
	if contract == nil {
		return nil, fmt.Errorf("contract %s not found in registry", contractName)
	}

	return contract.Functions, nil
}

// GetContractEndpointsFromRegistry 从注册中心获取合约端点
func (registerManager *RegisterContractManager) GetContractEndpointsFromRegistry(contractName string) ([]string, error) {
	contract := registerManager.GetContractInfoFromRegistry(contractName)
	if contract == nil {
		return nil, fmt.Errorf("contract %s not found in registry", contractName)
	}

	return contract.Endpoints, nil
}

// GetContractReputationFromRegistry 从注册中心获取合约声誉值
func (registerManager *RegisterContractManager) GetContractReputationFromRegistry(contractName string) (*big.Int, error) {
	contract := registerManager.GetContractInfoFromRegistry(contractName)
	if contract == nil {
		return nil, fmt.Errorf("contract %s not found in registry", contractName)
	}

	return contract.Reputation, nil
}

// ListAllContractsFromRegistry 列出注册中心所有合约
func (registerManager *RegisterContractManager) ListAllContractsFromRegistry() []*RegistryContractInfo {
	var contracts []*RegistryContractInfo
	for _, contract := range registerManager.contractsFromRegistry {
		contracts = append(contracts, contract)
	}
	return contracts
}

// GetContractSummaryFromRegistry 获取注册中心合约摘要
func (registerManager *RegisterContractManager) GetContractSummaryFromRegistry() map[string]interface{} {
	summary := make(map[string]interface{})

	// 按类型统计
	typeStats := make(map[string]int)
	serviceStats := make(map[string]int)
	protocolStats := make(map[string]int)

	for _, contract := range registerManager.contractsFromRegistry {
		// 类型统计
		typeStats[contract.Type]++

		// 服务统计
		for _, service := range contract.Services {
			serviceStats[service]++
		}

		// 协议统计
		for _, protocol := range contract.Protocols {
			protocolStats[protocol]++
		}
	}

	summary["totalContracts"] = len(registerManager.contractsFromRegistry)
	summary["typeStats"] = typeStats
	summary["serviceStats"] = serviceStats
	summary["protocolStats"] = protocolStats
	summary["registerCoreAddress"] = registerManager.registerCoreAddress
	summary["registerExtendedAddress"] = registerManager.registerExtendedAddress

	return summary
}
