package analyzer

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path/filepath"
)

// ContractArtifact 合约编译产物结构
type ContractArtifact struct {
	ContractName string          `json:"contractName"`
	SourceName   string          `json:"sourceName"`
	ABI          json.RawMessage `json:"abi"`
	Bytecode     string          `json:"bytecode"`
}

// DeployedContracts 已部署合约地址映射
type DeployedContracts struct {
	MyToken    string `json:"MyToken"`
	SimpleSwap string `json:"SimpleSwap"`
	MTKStaking string `json:"MTKStaking"`
}

// ContractLoader 合约配置加载器
type ContractLoader struct {
	artifactsDir  string
	deployedPath  string
	contractsPath string
	llmAnalyzer   *LLMContractAnalyzer
}

// LLMContractAnalyzer LLM合约分析器
type LLMContractAnalyzer struct {
	// 这里可以集成实际的LLM客户端
	// 暂时使用模拟分析
}

// NewContractLoader 创建合约加载器
func NewContractLoader(artifactsDir, deployedPath, contractsPath string) *ContractLoader {
	return &ContractLoader{
		artifactsDir:  artifactsDir,
		deployedPath:  deployedPath,
		contractsPath: contractsPath,
		llmAnalyzer:   &LLMContractAnalyzer{},
	}
}

// LoadContractsConfig 从文件动态加载合约配置
func (cl *ContractLoader) LoadContractsConfig() (*ContractsConfig, error) {
	// 1. 加载已部署合约地址
	deployed, err := cl.loadDeployedContracts()
	if err != nil {
		return nil, fmt.Errorf("failed to load deployed contracts: %w", err)
	}

	// 2. 加载网络配置
	networkConfig := NetworkConfig{
		ChainID: 8134,
		Name:    "Custom Network",
		RPCURL:  "http://47.242.255.132:1234/",
	}

	// 3. 加载代币配置
	tokens, err := cl.loadTokenConfigs(deployed)
	if err != nil {
		return nil, fmt.Errorf("failed to load token configs: %w", err)
	}

	// 4. 加载合约配置
	contracts, err := cl.loadContractConfigs(deployed)
	if err != nil {
		return nil, fmt.Errorf("failed to load contract configs: %w", err)
	}

	// 5. 生成工作流配置
	workflows := cl.generateWorkflowConfigs(contracts)

	config := &ContractsConfig{
		Version:   "1.0.0",
		Network:   networkConfig,
		Tokens:    tokens,
		Contracts: contracts,
		Workflows: workflows,
	}

	return config, nil
}

// loadDeployedContracts 加载已部署合约地址
func (cl *ContractLoader) loadDeployedContracts() (*DeployedContracts, error) {
	data, err := ioutil.ReadFile(cl.deployedPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read deployed.json: %w", err)
	}

	var deployed DeployedContracts
	if err := json.Unmarshal(data, &deployed); err != nil {
		return nil, fmt.Errorf("failed to parse deployed.json: %w", err)
	}

	return &deployed, nil
}

// loadTokenConfigs 加载代币配置
func (cl *ContractLoader) loadTokenConfigs(deployed *DeployedContracts) (map[string]TokenConfig, error) {
	tokens := make(map[string]TokenConfig)

	// 添加原生代币
	tokens["MEER"] = TokenConfig{
		Name:        "MEER",
		Symbol:      "MEER",
		Decimals:    18,
		IsNative:    true,
		Description: "Native token of the network",
	}

	// 添加MTK代币
	tokens["MTK"] = TokenConfig{
		Name:            "MyToken",
		Symbol:          "MTK",
		Decimals:        18,
		IsNative:        false,
		ContractAddress: deployed.MyToken,
		ContractName:    "MyToken",
		Description:     "ERC20 token deployed on custom network",
	}

	return tokens, nil
}

// loadContractConfigs 加载合约配置
func (cl *ContractLoader) loadContractConfigs(deployed *DeployedContracts) (map[string]ContractConfig, error) {
	contracts := make(map[string]ContractConfig)

	// 加载SimpleSwap合约
	simpleSwapConfig, err := cl.loadContractConfig("SimpleSwap", deployed.SimpleSwap)
	if err != nil {
		return nil, fmt.Errorf("failed to load SimpleSwap config: %w", err)
	}
	contracts["SimpleSwap"] = *simpleSwapConfig

	// 加载MTKStaking合约
	mtkStakingConfig, err := cl.loadContractConfig("MTKStaking", deployed.MTKStaking)
	if err != nil {
		return nil, fmt.Errorf("failed to load MTKStaking config: %w", err)
	}
	contracts["MTKStaking"] = *mtkStakingConfig

	return contracts, nil
}

// loadContractConfig 加载单个合约配置
func (cl *ContractLoader) loadContractConfig(contractName, address string) (*ContractConfig, error) {
	// 1. 读取合约源码
	sourceCode, err := cl.readContractSource(contractName)
	if err != nil {
		return nil, fmt.Errorf("failed to read contract source: %w", err)
	}

	// 2. 读取合约ABI
	abi, err := cl.readContractABI(contractName)
	if err != nil {
		return nil, fmt.Errorf("failed to read contract ABI: %w", err)
	}

	// 3. 使用LLM分析合约
	analysis, err := cl.llmAnalyzer.AnalyzeContract(contractName, sourceCode, abi)
	if err != nil {
		return nil, fmt.Errorf("failed to analyze contract: %w", err)
	}

	// 4. 构建合约配置
	config := &ContractConfig{
		Name:         contractName,
		Address:      address,
		ArtifactPath: fmt.Sprintf("artifacts/contracts/%s.sol/%s.json", contractName, contractName),
		Type:         analysis.Type,
		Description:  analysis.Description,
		Functions:    analysis.Functions,
	}

	// 根据合约类型添加特定配置
	switch contractName {
	case "SimpleSwap":
		config.SupportedPairs = analysis.SupportedPairs
	case "MTKStaking":
		config.StakingInfo = analysis.StakingInfo
	}

	return config, nil
}

// readContractSource 读取合约源码
func (cl *ContractLoader) readContractSource(contractName string) (string, error) {
	sourcePath := filepath.Join(cl.contractsPath, contractName+".sol")
	data, err := ioutil.ReadFile(sourcePath)
	if err != nil {
		return "", fmt.Errorf("failed to read contract source %s: %w", sourcePath, err)
	}
	return string(data), nil
}

// readContractABI 读取合约ABI
func (cl *ContractLoader) readContractABI(contractName string) (json.RawMessage, error) {
	artifactPath := filepath.Join(cl.artifactsDir, "contracts", contractName+".sol", contractName+".json")
	data, err := ioutil.ReadFile(artifactPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read contract artifact %s: %w", artifactPath, err)
	}

	var artifact ContractArtifact
	if err := json.Unmarshal(data, &artifact); err != nil {
		return nil, fmt.Errorf("failed to parse contract artifact: %w", err)
	}

	return artifact.ABI, nil
}

// generateWorkflowConfigs 生成工作流配置
func (cl *ContractLoader) generateWorkflowConfigs(contracts map[string]ContractConfig) map[string]WorkflowConfig {
	workflows := make(map[string]WorkflowConfig)

	// 为每个合约生成对应的工作流
	for contractName := range contracts {
		switch contractName {
		case "SimpleSwap":
			workflows["swap"] = WorkflowConfig{
				Description:    "Token swap operations",
				SupportedPairs: []string{"MEER-MTK", "MTK-MEER"},
				Contract:       contractName,
				Patterns: []string{
					"兑换 {amount} {fromToken} 为 {toToken}",
					"兑换 {amount} {fromToken}",
					"将 {amount} {fromToken} 换成 {toToken}",
					"swap {amount} {fromToken} to {toToken}",
					"exchange {amount} {fromToken} for {toToken}",
				},
			}
		case "MTKStaking":
			workflows["stake"] = WorkflowConfig{
				Description:     "Token staking operations",
				SupportedTokens: []string{"MTK"},
				Contract:        contractName,
				Patterns: []string{
					"质押 {amount} {token}",
					"质押 {amount} MTK",
					"stake {amount} {token}",
					"stake {amount} MTK",
					"将 {amount} {token} 质押",
					"抵押 {amount} {token}",
				},
			}
			workflows["unstake"] = WorkflowConfig{
				Description:     "Token unstaking operations",
				SupportedTokens: []string{"MTK"},
				Contract:        contractName,
				Patterns: []string{
					"取消质押 {amount} {token}",
					"解质押 {amount} MTK",
					"unstake {amount} {token}",
					"提取质押 {amount} {token}",
					"赎回 {amount} {token}",
				},
			}
			workflows["claimRewards"] = WorkflowConfig{
				Description:     "Claim staking rewards",
				SupportedTokens: []string{"MTK"},
				Contract:        contractName,
				Patterns: []string{
					"领取奖励",
					"领取收益",
					"claim rewards",
					"提取奖励",
					"收取奖励",
				},
			}
		}
	}

	// 添加复合操作工作流
	workflows["compound"] = WorkflowConfig{
		Description: "Compound operations combining multiple actions",
		Contract:    "Multiple",
		Patterns: []string{
			"兑换 {amount} {fromToken} 为 {toToken} 再 质押",
			"兑换 {amount} {fromToken} 然后 质押",
			"swap {amount} {fromToken} to {toToken} then stake",
			"exchange {amount} {fromToken} for {toToken} then stake",
		},
	}

	return workflows
}
