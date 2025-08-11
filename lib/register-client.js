// lib/register-client.js
const { ethers } = require("ethers");
const fs = require("fs");
const path = require("path");

/**
 * Register 客户端类
 * 用于从 Register 注册中心获取合约信息
 */
class RegisterClient {
  constructor(provider, registerCoreAddress, registerExtendedAddress) {
    this.provider = provider;
    this.registerCoreAddress = registerCoreAddress;
    this.registerExtendedAddress = registerExtendedAddress;
    
    // 缓存合约实例
    this.registerCore = null;
    this.registerExtended = null;
    
    // 缓存合约信息
    this.contractCache = new Map();
    this.serviceCache = new Map();
  }

  /**
   * 初始化客户端
   */
  async initialize() {
    try {
      // 读取合约 ABI
      const coreArtifact = JSON.parse(fs.readFileSync("artifacts/contracts/RegisterCore.sol/RegisterCore.json", "utf8"));
      const extendedArtifact = JSON.parse(fs.readFileSync("artifacts/contracts/RegisterExtended.sol/RegisterExtended.json", "utf8"));
      
      // 创建合约实例
      this.registerCore = new ethers.Contract(this.registerCoreAddress, coreArtifact.abi, this.provider);
      this.registerExtended = new ethers.Contract(this.registerExtendedAddress, extendedArtifact.abi, this.provider);
      
      console.log("Register 客户端初始化成功");
      return true;
    } catch (error) {
      console.error("Register 客户端初始化失败:", error);
      return false;
    }
  }

  /**
   * 发现提供指定服务的代理
   */
  async discoverAgentsByService(serviceType) {
    try {
      const agents = await this.registerCore.discoverAgentsByService(serviceType);
      return agents;
    } catch (error) {
      console.error(`发现 ${serviceType} 服务失败:`, error);
      return [];
    }
  }

  /**
   * 发现支持指定协议的代理
   */
  async discoverAgentsByProtocol(protocol) {
    try {
      const agents = await this.registerCore.discoverAgentsByProtocol(protocol);
      return agents;
    } catch (error) {
      console.error(`发现 ${protocol} 协议失败:`, error);
      return [];
    }
  }

  /**
   * 发现带有指定标签的代理
   */
  async discoverAgentsByTag(tag) {
    try {
      const agents = await this.registerCore.discoverAgentsByTag(tag);
      return agents;
    } catch (error) {
      console.error(`发现 ${tag} 标签失败:`, error);
      return [];
    }
  }

  /**
   * 获取代理详细信息
   */
  async getAgentDetails(agentId) {
    try {
      const details = await this.registerCore.getAgentDetails(agentId);
      return {
        owner: details[0],
        endpoints: details[1],
        services: details[2],
        protocols: details[3],
        reputation: details[4],
        metadata: details[5],
        abi: details[6] // 新增的ABI字段
      };
    } catch (error) {
      console.error(`获取代理 ${agentId} 详情失败:`, error);
      return null;
    }
  }

  /**
   * 获取代理端点
   */
  async getAgentEndpoint(agentId) {
    try {
      const endpoint = await this.registerCore.getAgentEndpoint(agentId);
      return endpoint;
    } catch (error) {
      console.error(`获取代理 ${agentId} 端点失败:`, error);
      return null;
    }
  }

  /**
   * 检查代理是否活跃
   */
  async isAgentActive(agentId) {
    try {
      return await this.registerCore.isAgentActive(agentId);
    } catch (error) {
      console.error(`检查代理 ${agentId} 状态失败:`, error);
      return false;
    }
  }

  /**
   * 获取活跃的服务请求
   */
  async getActiveServiceRequests(serviceType) {
    try {
      const requestIds = await this.registerExtended.getActiveServiceRequests(serviceType);
      const requests = [];
      
      for (const requestId of requestIds) {
        const request = await this.registerExtended.getServiceRequest(requestId);
        requests.push({
          requestId: requestId.toString(),
          requesterAgentId: request[0],
          serviceType: request[1],
          requirements: request[2],
          timestamp: request[3].toString(),
          expiryTime: request[4].toString(),
          isActive: request[5],
          respondents: request[6]
        });
      }
      
      return requests;
    } catch (error) {
      console.error(`获取 ${serviceType} 服务请求失败:`, error);
      return [];
    }
  }

  /**
   * 根据合约类型获取合约信息
   */
  async getContractsByType(contractType) {
    const agents = await this.discoverAgentsByService(contractType);
    const contracts = [];
    
    for (const agentId of agents) {
      const details = await this.getAgentDetails(agentId);
      if (details) {
        const metadata = JSON.parse(details.metadata);
        if (metadata.type === contractType) {
          contracts.push({
            agentId,
            address: details.endpoints[0], // 第一个端点作为合约地址
            name: metadata.name,
            type: metadata.type,
            description: metadata.description,
            functions: metadata.functions,
            reputation: details.reputation,
            services: details.services,
            protocols: details.protocols
          });
        }
      }
    }
    
    return contracts;
  }

  /**
   * 获取所有 ERC20 代币合约
   */
  async getERC20Tokens() {
    return await this.getContractsByType("ERC20");
  }

  /**
   * 获取所有 DEX 合约
   */
  async getDEXContracts() {
    return await this.getContractsByType("DEX");
  }

  /**
   * 获取所有 Staking 合约
   */
  async getStakingContracts() {
    return await this.getContractsByType("Staking");
  }

  /**
   * 搜索合约
   */
  async searchContracts(criteria) {
    const { service, protocol, minReputation = 0 } = criteria;
    const contracts = [];
    
    let agents = [];
    if (service) {
      agents = await this.discoverAgentsByService(service);
    } else if (protocol) {
      agents = await this.discoverAgentsByProtocol(protocol);
    }
    
    for (const agentId of agents) {
      const details = await this.getAgentDetails(agentId);
      if (details && details.reputation >= minReputation) {
        const metadata = JSON.parse(details.metadata);
        contracts.push({
          agentId,
          address: details.endpoints[0],
          name: metadata.name,
          type: metadata.type,
          description: metadata.description,
          reputation: details.reputation,
          services: details.services,
          protocols: details.protocols
        });
      }
    }
    
    return contracts;
  }

  /**
   * 获取合约 ABI
   */
  async getContractABI(agentId) {
    try {
      // 首先尝试从Register合约直接获取ABI
      const abi = await this.registerCore.getAgentABI(agentId);
      if (abi && abi.length > 0) {
        console.log(`✅ 从Register合约获取到 ${agentId} 的ABI，长度: ${abi.length}`);
        return JSON.parse(abi);
      }
      
      // 如果Register合约中没有ABI，回退到从文件系统读取
      console.log(`⚠️  Register合约中没有 ${agentId} 的ABI，尝试从文件系统读取`);
      const details = await this.getAgentDetails(agentId);
      if (details) {
        const metadata = JSON.parse(details.metadata);
        const artifactPath = metadata.artifactPath;
        
        if (artifactPath && fs.existsSync(artifactPath)) {
          const artifact = JSON.parse(fs.readFileSync(artifactPath, "utf8"));
          return artifact.abi;
        }
      }
      return null;
    } catch (error) {
      console.error(`获取合约 ${agentId} ABI 失败:`, error);
      return null;
    }
  }

  /**
   * 创建合约实例
   */
  async createContractInstance(agentId, signer = null) {
    try {
      const endpoint = await this.getAgentEndpoint(agentId);
      const abi = await this.getContractABI(agentId);
      
      if (endpoint && abi) {
        const provider = signer || this.provider;
        return new ethers.Contract(endpoint, abi, provider);
      }
      return null;
    } catch (error) {
      console.error(`创建合约实例失败:`, error);
      return null;
    }
  }

  /**
   * 获取推荐合约
   */
  async getRecommendedContracts(serviceType, excludeAgentId = null) {
    try {
      const agents = await this.discoverAgentsByService(serviceType);
      const recommendations = [];
      
      for (const agentId of agents) {
        if (excludeAgentId && agentId === excludeAgentId) {
          continue;
        }
        
        const details = await this.getAgentDetails(agentId);
        if (details && details.reputation >= 70) {
          const metadata = JSON.parse(details.metadata);
          recommendations.push({
            agentId,
            address: details.endpoints[0],
            name: metadata.name,
            type: metadata.type,
            reputation: details.reputation,
            description: metadata.description
          });
        }
      }
      
      return recommendations.sort((a, b) => b.reputation - a.reputation);
    } catch (error) {
      console.error(`获取推荐合约失败:`, error);
      return [];
    }
  }

  /**
   * 刷新缓存
   */
  clearCache() {
    this.contractCache.clear();
    this.serviceCache.clear();
  }

  /**
   * 获取注册中心统计信息
   */
  async getRegistryStats() {
    try {
      const totalAgents = await this.registerCore.totalAgents();
      const registrationFee = await this.registerCore.registrationFee();
      const isPaused = await this.registerCore.paused();
      
      return {
        totalAgents: totalAgents.toString(),
        registrationFee: ethers.formatEther(registrationFee),
        isPaused,
        registerCoreAddress: this.registerCoreAddress,
        registerExtendedAddress: this.registerExtendedAddress
      };
    } catch (error) {
      console.error("获取注册中心统计信息失败:", error);
      return null;
    }
  }
}

module.exports = RegisterClient;
