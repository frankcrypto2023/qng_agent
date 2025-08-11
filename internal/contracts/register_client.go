package contracts

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
)

// RegisterCore ABI 定义
const registerCoreABI = `[
	{
		"inputs": [],
		"name": "totalAgents",
		"outputs": [
			{
				"internalType": "uint256",
				"name": "",
				"type": "uint256"
			}
		],
		"stateMutability": "view",
		"type": "function"
	},
	{
		"inputs": [
			{
				"internalType": "string",
				"name": "agentId",
				"type": "string"
			}
		],
		"name": "getAgentABI",
		"outputs": [
			{
				"internalType": "string",
				"name": "contractABI",
				"type": "string"
			}
		],
		"stateMutability": "view",
		"type": "function"
	},
	{
		"inputs": [
			{
				"internalType": "string",
				"name": "agentId",
				"type": "string"
			}
		],
		"name": "getAgentDetails",
		"outputs": [
			{
				"internalType": "address",
				"name": "agentOwner",
				"type": "address"
			},
			{
				"internalType": "string[]",
				"name": "endpoints",
				"type": "string[]"
			},
			{
				"internalType": "string[]",
				"name": "services",
				"type": "string[]"
			},
			{
				"internalType": "string[]",
				"name": "protocols",
				"type": "string[]"
			},
			{
				"internalType": "uint256",
				"name": "reputation",
				"type": "uint256"
			},
			{
				"internalType": "string",
				"name": "metadata",
				"type": "string"
			},
			{
				"internalType": "string",
				"name": "contractABI",
				"type": "string"
			}
		],
		"stateMutability": "view",
		"type": "function"
	},
	{
		"inputs": [
			{
				"internalType": "string",
				"name": "service",
				"type": "string"
			}
		],
		"name": "discoverAgentsByService",
		"outputs": [
			{
				"internalType": "string[]",
				"name": "",
				"type": "string[]"
			}
		],
		"stateMutability": "view",
		"type": "function"
	}
]`

// RegisterClient Register合约客户端
type RegisterClient struct {
	registerCoreAddress     string
	registerExtendedAddress string
	rpcURL                  string
	client                  *ethclient.Client
	ctx                     context.Context
	registerCoreABI         abi.ABI
}

// NewRegisterClient 创建Register客户端
func NewRegisterClient(coreAddress, extendedAddress, rpcURL string) *RegisterClient {
	// 连接到以太坊客户端
	var client *ethclient.Client
	var err error

	if rpcURL != "" {
		client, err = ethclient.Dial(rpcURL)
		if err != nil {
			log.Printf("⚠️  无法连接到以太坊客户端 %s: %v，将使用模拟数据", rpcURL, err)
			client = nil
		} else {
			log.Printf("✅ 成功连接到以太坊客户端: %s", rpcURL)
		}
	} else {
		log.Printf("⚠️  未配置RPC URL，将使用模拟数据")
	}

	// 解析RegisterCore ABI
	parsedABI, err := abi.JSON(strings.NewReader(registerCoreABI))
	if err != nil {
		log.Printf("⚠️  解析RegisterCore ABI失败: %v", err)
	}

	return &RegisterClient{
		registerCoreAddress:     coreAddress,
		registerExtendedAddress: extendedAddress,
		rpcURL:                  rpcURL,
		client:                  client,
		ctx:                     context.Background(),
		registerCoreABI:         parsedABI,
	}
}

// GetAgentDetails 获取代理详细信息
func (rc *RegisterClient) GetAgentDetails(agentId string) (*RegistryContractInfo, error) {
	log.Printf("🔍 从Register合约获取代理详情: %s", agentId)

	if rc.client == nil {
		log.Printf("⚠️  以太坊客户端未连接，使用模拟数据")
		return rc.getSimulatedAgentDetails(agentId)
	}

	// 尝试从真实合约获取数据
	log.Printf("🔍 尝试从RegisterCore合约获取代理详情: %s", agentId)

	// 检查ABI是否已解析
	if rc.registerCoreABI.Methods == nil {
		log.Printf("⚠️  RegisterCore ABI未正确解析，使用模拟数据")
		return rc.getSimulatedAgentDetails(agentId)
	}

	// 构建调用数据
	data, err := rc.registerCoreABI.Pack("getAgentDetails", agentId)
	if err != nil {
		log.Printf("⚠️  ABI编码失败: %v，使用模拟数据", err)
		return rc.getSimulatedAgentDetails(agentId)
	}

	// 调用合约
	contractAddress := common.HexToAddress(rc.registerCoreAddress)
	msg := ethereum.CallMsg{
		To:   &contractAddress,
		Data: data,
	}

	result, err := rc.client.CallContract(rc.ctx, msg, nil)
	if err != nil {
		log.Printf("⚠️  调用getAgentDetails失败: %v，使用模拟数据", err)
		return rc.getSimulatedAgentDetails(agentId)
	}

	log.Printf("🔧 getAgentDetails调用成功，返回数据长度: %d", len(result))
	if len(result) == 0 {
		log.Printf("⚠️  getAgentDetails返回空数据，代理 %s 可能不存在", agentId)
		return rc.getSimulatedAgentDetails(agentId)
	}

	// 解析返回结果 - 使用更安全的方式
	var results []interface{}
	err = rc.registerCoreABI.UnpackIntoInterface(&results, "getAgentDetails", result)
	if err != nil {
		log.Printf("⚠️  解析getAgentDetails结果失败: %v，使用模拟数据", err)
		return rc.getSimulatedAgentDetails(agentId)
	}

	// 检查结果数组长度
	if len(results) < 7 {
		log.Printf("⚠️  getAgentDetails返回结果不完整，使用模拟数据")
		return rc.getSimulatedAgentDetails(agentId)
	}

	// 构建返回结果
	info := &RegistryContractInfo{
		AgentID: agentId,
	}

	// 安全地提取各个字段
	if owner, ok := results[0].(common.Address); ok {
		info.Address = owner.Hex()
	}

	if endpoints, ok := results[1].([]string); ok {
		info.Endpoints = endpoints
	}

	if services, ok := results[2].([]string); ok {
		info.Services = services
	}

	if protocols, ok := results[3].([]string); ok {
		info.Protocols = protocols
	}

	if reputation, ok := results[4].(*big.Int); ok {
		info.Reputation = reputation
	}

	if metadata, ok := results[5].(string); ok && metadata != "" {
		// 尝试解析JSON元数据
		var metaData map[string]interface{}
		if err := json.Unmarshal([]byte(metadata), &metaData); err == nil {
			if name, ok := metaData["name"].(string); ok {
				info.Name = name
			} else {
				info.Name = agentId
			}
			if contractType, ok := metaData["type"].(string); ok {
				info.Type = contractType
			} else {
				info.Type = "Contract"
			}
			if description, ok := metaData["description"].(string); ok {
				info.Description = description
			} else {
				info.Description = fmt.Sprintf("Contract registered as %s", agentId)
			}
		} else {
			info.Name = agentId
			info.Type = "Contract"
			info.Description = fmt.Sprintf("Contract registered as %s", agentId)
		}
	} else {
		info.Name = agentId
		info.Type = "Contract"
		info.Description = fmt.Sprintf("Contract registered as %s", agentId)
	}

	log.Printf("✅ 成功从链上获取代理详情")
	return info, nil
}

// getSimulatedAgentDetails 返回模拟的代理详情
func (rc *RegisterClient) getSimulatedAgentDetails(agentId string) (*RegistryContractInfo, error) {
	switch agentId {
	case "simpleswap":
		return &RegistryContractInfo{
			AgentID:     "simpleswap",
			Address:     "0xfBb52268B01e20a9C0C566932716c9B9c550c868",
			Name:        "SimpleSwap",
			Type:        "DEX",
			Description: "Simple token swap contract for ETH <-> MTK",
			Services:    []string{"Swap", "DEX", "Exchange"},
			Protocols:   []string{"ERC20"},
			Tags:        []string{"swap", "dex", "defi"},
			Reputation:  big.NewInt(80),
			Endpoints:   []string{"0xfBb52268B01e20a9C0C566932716c9B9c550c868"},
		}, nil
	case "mtkstaking":
		return &RegistryContractInfo{
			AgentID:     "mtkstaking",
			Address:     "0x85ed17629F364381ccEd92F701c028bfDEE501EC",
			Name:        "MTKStaking",
			Type:        "Staking",
			Description: "MTK token staking contract with rewards",
			Services:    []string{"Staking", "Rewards"},
			Protocols:   []string{"ERC20"},
			Tags:        []string{"staking", "rewards", "defi"},
			Reputation:  big.NewInt(75),
			Endpoints:   []string{"0x85ed17629F364381ccEd92F701c028bfDEE501EC"},
		}, nil
	case "mytoken":
		return &RegistryContractInfo{
			AgentID:     "mytoken",
			Address:     "0x1859Bd4e1d2Ba470b1E6D9C8d14dF785e533E3A0",
			Name:        "MyToken",
			Type:        "ERC20",
			Description: "Standard ERC20 token contract",
			Services:    []string{"ERC20", "Token"},
			Protocols:   []string{"ERC20"},
			Tags:        []string{"token", "erc20", "defi"},
			Reputation:  big.NewInt(90),
			Endpoints:   []string{"0x1859Bd4e1d2Ba470b1E6D9C8d14dF785e533E3A0"},
		}, nil
	default:
		return nil, fmt.Errorf("agent %s not found", agentId)
	}
}

// GetAgentABI 获取代理的ABI
func (rc *RegisterClient) GetAgentABI(agentId string) (string, error) {
	log.Printf("🔍 从Register合约获取ABI: %s", agentId)

	if rc.client == nil {
		log.Printf("⚠️  以太坊客户端未连接，使用模拟数据")
		return rc.getSimulatedAgentABI(agentId)
	}

	// 尝试从真实合约获取数据
	log.Printf("🔍 尝试从RegisterCore合约获取ABI: %s", agentId)

	// 检查ABI是否已解析
	if rc.registerCoreABI.Methods == nil {
		log.Printf("⚠️  RegisterCore ABI未正确解析，使用模拟数据")
		return rc.getSimulatedAgentABI(agentId)
	}

	// 构建调用数据
	data, err := rc.registerCoreABI.Pack("getAgentABI", agentId)
	if err != nil {
		log.Printf("⚠️  ABI编码失败: %v，使用模拟数据", err)
		return rc.getSimulatedAgentABI(agentId)
	}

	// 调用合约
	contractAddress := common.HexToAddress(rc.registerCoreAddress)
	msg := ethereum.CallMsg{
		To:   &contractAddress,
		Data: data,
	}

	result, err := rc.client.CallContract(rc.ctx, msg, nil)
	if err != nil {
		log.Printf("⚠️  调用getAgentABI失败: %v，使用模拟数据", err)
		return rc.getSimulatedAgentABI(agentId)
	}

	// 解析返回结果
	var contractABI string
	err = rc.registerCoreABI.UnpackIntoInterface(&contractABI, "getAgentABI", result)
	if err != nil {
		log.Printf("⚠️  解析getAgentABI结果失败: %v，使用模拟数据", err)
		return rc.getSimulatedAgentABI(agentId)
	}

	if contractABI == "" {
		log.Printf("⚠️  合约返回的ABI为空，使用模拟数据")
		return rc.getSimulatedAgentABI(agentId)
	}

	log.Printf("✅ 成功从链上获取ABI，长度: %d 字符", len(contractABI))
	return contractABI, nil
}

// getSimulatedAgentABI 返回模拟的ABI数据
func (rc *RegisterClient) getSimulatedAgentABI(agentId string) (string, error) {
	switch agentId {
	case "simpleswap":
		// 返回SimpleSwap的ABI（简化版）
		return `[
			{
				"type": "function",
				"name": "buyToken",
				"inputs": [],
				"outputs": [],
				"stateMutability": "payable"
			},
			{
				"type": "function", 
				"name": "sellToken",
				"inputs": [{"name": "tokenAmount", "type": "uint256"}],
				"outputs": [],
				"stateMutability": "nonpayable"
			}
		]`, nil
	case "mtkstaking":
		// 返回MTKStaking的ABI（简化版）
		return `[
			{
				"type": "function",
				"name": "stake",
				"inputs": [{"name": "amount", "type": "uint256"}],
				"outputs": [],
				"stateMutability": "nonpayable"
			},
			{
				"type": "function",
				"name": "unstake", 
				"inputs": [{"name": "amount", "type": "uint256"}],
				"outputs": [],
				"stateMutability": "nonpayable"
			}
		]`, nil
	case "mytoken":
		// 返回MyToken的ABI（简化版）
		return `[
			{
				"type": "function",
				"name": "transfer",
				"inputs": [
					{"name": "to", "type": "address"},
					{"name": "amount", "type": "uint256"}
				],
				"outputs": [{"name": "", "type": "bool"}],
				"stateMutability": "nonpayable"
			}
		]`, nil
	default:
		return "", fmt.Errorf("ABI not found for agent %s", agentId)
	}
}

// DiscoverAgentsByService 根据服务发现代理
func (rc *RegisterClient) DiscoverAgentsByService(service string) ([]string, error) {
	log.Printf("🔍 根据服务发现代理: %s", service)

	if rc.client == nil {
		log.Printf("⚠️  以太坊客户端未连接，使用模拟数据")
		// 返回模拟的注册数据
		switch service {
		case "Swap", "DEX":
			return []string{"simpleswap"}, nil
		case "Staking", "Rewards":
			return []string{"mtkstaking"}, nil
		case "ERC20", "Token":
			return []string{"mytoken"}, nil
		default:
			return []string{}, nil
		}
	}

	// 检查ABI是否已解析
	if rc.registerCoreABI.Methods == nil {
		log.Printf("⚠️  RegisterCore ABI未正确解析，使用模拟数据")
		// 返回模拟数据
		switch service {
		case "Swap", "DEX":
			return []string{"simpleswap"}, nil
		case "Staking", "Rewards":
			return []string{"mtkstaking"}, nil
		case "ERC20", "Token":
			return []string{"mytoken"}, nil
		default:
			return []string{}, nil
		}
	}

	// 尝试从真实合约获取数据
	log.Printf("🔍 尝试从RegisterCore合约获取服务: %s", service)

	// 构建调用数据
	data, err := rc.registerCoreABI.Pack("discoverAgentsByService", service)
	if err != nil {
		log.Printf("⚠️  ABI编码失败: %v，使用模拟数据", err)
		// 返回模拟数据
		switch service {
		case "Swap", "DEX":
			return []string{"simpleswap"}, nil
		case "Staking", "Rewards":
			return []string{"mtkstaking"}, nil
		case "ERC20", "Token":
			return []string{"mytoken"}, nil
		default:
			return []string{}, nil
		}
	}

	// 调用合约
	contractAddress := common.HexToAddress(rc.registerCoreAddress)
	msg := ethereum.CallMsg{
		To:   &contractAddress,
		Data: data,
	}

	result, err := rc.client.CallContract(rc.ctx, msg, nil)
	if err != nil {
		log.Printf("⚠️  调用discoverAgentsByService失败: %v，使用模拟数据", err)
		// 返回模拟数据
		switch service {
		case "Swap", "DEX":
			return []string{"simpleswap"}, nil
		case "Staking", "Rewards":
			return []string{"mtkstaking"}, nil
		case "ERC20", "Token":
			return []string{"mytoken"}, nil
		default:
			return []string{}, nil
		}
	}

	// 解析返回结果
	var agents []string
	err = rc.registerCoreABI.UnpackIntoInterface(&agents, "discoverAgentsByService", result)
	if err != nil {
		log.Printf("⚠️  解析discoverAgentsByService结果失败: %v，使用模拟数据", err)
		// 返回模拟数据
		switch service {
		case "Swap", "DEX":
			return []string{"simpleswap"}, nil
		case "Staking", "Rewards":
			return []string{"mtkstaking"}, nil
		case "ERC20", "Token":
			return []string{"mytoken"}, nil
		default:
			return []string{}, nil
		}
	}

	log.Printf("✅ 成功从链上发现 %d 个代理: %v", len(agents), agents)

	// 过滤和清理代理ID，只保留小写的代理ID
	var cleanedAgents []string
	seen := make(map[string]bool)

	for _, agent := range agents {
		// 移除可能的空格和特殊字符
		cleanedAgent := strings.TrimSpace(agent)
		if cleanedAgent != "" {
			// 转换为小写，因为注册脚本使用 contractName.toLowerCase()
			lowerAgent := strings.ToLower(cleanedAgent)
			if !seen[lowerAgent] {
				cleanedAgents = append(cleanedAgents, lowerAgent)
				seen[lowerAgent] = true
			}
		}
	}

	log.Printf("🔧 清理后的代理列表: %v", cleanedAgents)
	return cleanedAgents, nil
}

// GetTotalAgents 获取总代理数
func (rc *RegisterClient) GetTotalAgents() (*big.Int, error) {
	log.Printf("🔍 获取总代理数")

	if rc.client == nil {
		// 如果无法连接到以太坊客户端，返回模拟数据
		return big.NewInt(3), nil
	}

	// 检查ABI是否已解析
	if rc.registerCoreABI.Methods == nil {
		log.Printf("⚠️  RegisterCore ABI未正确解析，使用模拟数据")
		return big.NewInt(3), nil
	}

	// 构建调用数据
	data, err := rc.registerCoreABI.Pack("totalAgents")
	if err != nil {
		log.Printf("⚠️  ABI编码失败: %v，使用模拟数据", err)
		return big.NewInt(3), nil
	}

	// 调用合约
	contractAddress := common.HexToAddress(rc.registerCoreAddress)
	msg := ethereum.CallMsg{
		To:   &contractAddress,
		Data: data,
	}

	result, err := rc.client.CallContract(rc.ctx, msg, nil)
	if err != nil {
		log.Printf("⚠️  调用totalAgents失败: %v，使用模拟数据", err)
		return big.NewInt(3), nil
	}

	// 解析返回结果
	var totalAgents *big.Int
	err = rc.registerCoreABI.UnpackIntoInterface(&totalAgents, "totalAgents", result)
	if err != nil {
		log.Printf("⚠️  解析totalAgents结果失败: %v，使用模拟数据", err)
		return big.NewInt(3), nil
	}

	log.Printf("✅ 成功从链上获取总代理数: %s", totalAgents.String())
	return totalAgents, nil
}
