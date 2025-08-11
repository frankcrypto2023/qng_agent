// scripts/register-contracts.js
const { ethers } = require("hardhat");
const fs = require("fs");
const path = require("path");

async function main() {
  console.log("开始注册合约到 Register 注册中心...");

  // 获取部署账户
  const [deployer] = await ethers.getSigners();
  console.log("注册账户:", deployer.address);

  // 读取 deployed.json 获取合约地址
  const deployedData = JSON.parse(fs.readFileSync("deployed.json", "utf8"));
  
  // 读取 contracts.json 获取合约配置
  const contractsConfig = JSON.parse(fs.readFileSync("config/contracts.json", "utf8"));
  
  // 连接到 Register 注册中心
  const RegisterCore = await ethers.getContractFactory("RegisterCore");
  const registerCore = RegisterCore.attach(deployedData.RegisterCore);
  
  const RegisterExtended = await ethers.getContractFactory("RegisterExtended");
  const registerExtended = RegisterExtended.attach(deployedData.RegisterExtended);

  console.log("连接到注册中心合约:");
  console.log("- RegisterCore:", deployedData.RegisterCore);
  console.log("- RegisterExtended:", deployedData.RegisterExtended);

  // 定义合约服务类型
  const contractServices = {
    "MyToken": ["ERC20", "Token", "Transfer"],
    "SimpleSwap": ["DEX", "Swap", "Exchange"],
    "MTKStaking": ["Staking", "Rewards", "Yield"]
  };

  // 定义合约协议
  const contractProtocols = {
    "MyToken": ["ERC20", "Ethereum"],
    "SimpleSwap": ["Custom", "DEX"],
    "MTKStaking": ["Custom", "Staking"]
  };

  // 定义合约标签
  const contractTags = {
    "MyToken": ["token", "erc20", "fungible"],
    "SimpleSwap": ["dex", "swap", "exchange", "liquidity"],
    "MTKStaking": ["staking", "rewards", "yield", "defi"]
  };

  // 注册每个合约
  for (const [contractName, contractAddress] of Object.entries(deployedData)) {
    // 跳过注册中心合约本身
    if (contractName === "RegisterCore" || contractName === "RegisterExtended") {
      continue;
    }

    console.log(`\n注册合约: ${contractName}`);
    console.log(`地址: ${contractAddress}`);

    // 获取合约配置
    const contractConfig = contractsConfig.contracts[contractName];
    if (!contractConfig) {
      console.log(`⚠️  警告: ${contractName} 在 contracts.json 中未找到配置`);
      continue;
    }

    // 准备注册参数
    const agentId = contractName.toLowerCase();
    const endpoints = [contractAddress]; // 合约地址作为端点
    const weights = [100]; // 权重为100
    const services = contractServices[contractName] || ["Contract"];
    const protocols = contractProtocols[contractName] || ["Custom"];
    const tags = contractTags[contractName] || ["contract"];
    
    // 构建元数据
    const metadata = JSON.stringify({
      name: contractConfig.name,
      type: contractConfig.type,
      description: contractConfig.description,
      artifactPath: contractConfig.artifactPath,
      functions: Object.keys(contractConfig.functions || {}),
      version: contractsConfig.version,
      network: contractsConfig.network.name
    });

    // 读取合约ABI
    let contractABI = "";
    try {
      const artifactPath = path.join(__dirname, "..", contractConfig.artifactPath);
      if (fs.existsSync(artifactPath)) {
        const artifact = JSON.parse(fs.readFileSync(artifactPath, "utf8"));
        contractABI = JSON.stringify(artifact.abi);
        console.log(`   - 读取ABI成功，长度: ${contractABI.length} 字符`);
      } else {
        console.log(`   - 警告: ABI文件不存在: ${artifactPath}`);
      }
    } catch (error) {
      console.log(`   - 警告: 读取ABI失败: ${error.message}`);
    }

    try {
      // 注册到 Register 核心合约
      const registrationFee = await registerCore.registrationFee();
      const tx = await registerCore.registerAgent(
        agentId,
        endpoints,
        weights,
        services,
        protocols,
        tags,
        metadata,
        contractABI,
        { value: registrationFee }
      );
      
      await tx.wait();
      console.log(`✅ ${contractName} 注册成功!`);
      console.log(`   - 服务: ${services.join(", ")}`);
      console.log(`   - 协议: ${protocols.join(", ")}`);
      console.log(`   - 标签: ${tags.join(", ")}`);

      // 如果合约支持工作流，创建服务请求
      if (contractConfig.type === "DEX" || contractConfig.type === "Staking") {
        console.log(`   - 创建服务请求示例...`);
        
        // 为 DEX 创建交换服务请求
        if (contractConfig.type === "DEX") {
          const swapRequestTx = await registerExtended.requestService(
            agentId,
            "swap",
            JSON.stringify({
              from: "MEER",
              to: "MTK",
              amount: "1.0"
            }),
            3600 // 1小时过期
          );
          await swapRequestTx.wait();
          console.log(`   - 创建了交换服务请求`);
        }

        // 为 Staking 创建质押服务请求
        if (contractConfig.type === "Staking") {
          const stakeRequestTx = await registerExtended.requestService(
            agentId,
            "stake",
            JSON.stringify({
              token: "MTK",
              amount: "100.0"
            }),
            3600 // 1小时过期
          );
          await stakeRequestTx.wait();
          console.log(`   - 创建了质押服务请求`);
        }
      }

    } catch (error) {
      console.error(`❌ ${contractName} 注册失败:`, error.message);
      
      // 检查是否已经注册
      if (error.message.includes("Agent already registered")) {
        console.log(`   - ${contractName} 已经注册，跳过...`);
      }
    }
  }

  // 验证注册结果
  console.log("\n=== 验证注册结果 ===");
  const totalAgents = await registerCore.totalAgents();
  console.log(`总注册代理数: ${totalAgents}`);

  // 查询每个服务类型的代理
  const serviceTypes = ["ERC20", "DEX", "Staking", "Token", "Swap", "Exchange"];
  for (const serviceType of serviceTypes) {
    try {
      const agents = await registerCore.discoverAgentsByService(serviceType);
      if (agents.length > 0) {
        console.log(`${serviceType} 服务代理: ${agents.join(", ")}`);
      }
    } catch (error) {
      // 忽略不存在的服务类型
    }
  }

  // 查询活跃的服务请求
  console.log("\n=== 活跃服务请求 ===");
  const requestTypes = ["swap", "stake", "transfer"];
  for (const requestType of requestTypes) {
    try {
      const requests = await registerExtended.getActiveServiceRequests(requestType);
      if (requests.length > 0) {
        console.log(`${requestType} 类型请求数: ${requests.length}`);
      }
    } catch (error) {
      // 忽略不存在的请求类型
    }
  }

  console.log("\n✅ 合约注册完成!");
  console.log("现在可以通过 Register 注册中心发现和使用这些合约了。");
}

main().catch((error) => {
  console.error("注册失败:", error);
  process.exitCode = 1;
});
