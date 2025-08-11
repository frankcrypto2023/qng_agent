// scripts/deploy-register-split.js
const { ethers } = require("hardhat");
const fs = require("fs");
const path = require("path");

async function main() {
  console.log("开始部署 Register 分体合约...");

  // 获取部署账户
  const [deployer] = await ethers.getSigners();
  console.log("部署账户:", deployer.address);
  console.log("账户余额:", ethers.formatEther(await ethers.provider.getBalance(deployer.address)), "ETH");

  // 读取合约ABI文件
  function readABI(contractName) {
    try {
      const abiPath = path.join(__dirname, "..", "artifacts", "contracts", `${contractName}.sol`, `${contractName}.json`);
      const artifact = JSON.parse(fs.readFileSync(abiPath, "utf8"));
      return JSON.stringify(artifact.abi);
    } catch (error) {
      console.log(`⚠️  无法读取 ${contractName} 的ABI: ${error.message}`);
      return "";
    }
  }

  // 1. 部署 RegisterCore 合约
  console.log("\n1. 部署 RegisterCore 合约...");
  const RegisterCore = await ethers.getContractFactory("RegisterCore");
  const registerCore = await RegisterCore.deploy();
  await registerCore.waitForDeployment();
  
  const coreAddress = await registerCore.getAddress();
  console.log("RegisterCore 合约部署成功!");
  console.log("核心合约地址:", coreAddress);
  console.log("合约版本:", await registerCore.getVersion());
  console.log("注册费用:", ethers.formatEther(await registerCore.registrationFee()), "ETH");
  console.log("合约所有者:", await registerCore.owner());

  // 2. 部署 RegisterExtended 合约
  console.log("\n2. 部署 RegisterExtended 合约...");
  const RegisterExtended = await ethers.getContractFactory("RegisterExtended");
  const registerExtended = await RegisterExtended.deploy(coreAddress);
  await registerExtended.waitForDeployment();
  
  const extendedAddress = await registerExtended.getAddress();
  console.log("RegisterExtended 合约部署成功!");
  console.log("扩展合约地址:", extendedAddress);
  console.log("关联的核心合约:", await registerExtended.coreContract());

  // 验证部署
  console.log("\n验证部署信息:");
  console.log("- 核心合约是否暂停:", await registerCore.paused());
  console.log("- 核心合约总代理数量:", await registerCore.totalAgents());
  console.log("- 扩展合约服务请求计数器:", await registerExtended.serviceRequestCounter());

  // 3. 注册合约到Register
  console.log("\n3. 注册合约到Register...");
  
  // 准备要注册的合约列表
  const contractsToRegister = [
    {
      name: "MyToken",
      address: "0x1859Bd4e1d2Ba470b1E6D9C8d14dF785e533E3A0",
      services: ["ERC20", "Token"],
      protocols: ["ERC20"],
      tags: ["token", "erc20", "defi"],
      metadata: "Standard ERC20 token contract"
    },
    {
      name: "SimpleSwap",
      address: "0xfBb52268B01e20a9C0C566932716c9B9c550c868",
      services: ["Swap", "DEX"],
      protocols: ["ERC20"],
      tags: ["swap", "dex", "defi"],
      metadata: "Simple token swap contract for ETH <-> MTK"
    },
    {
      name: "MTKStaking",
      address: "0x85ed17629F364381ccEd92F701c028bfDEE501EC",
      services: ["Staking", "Rewards"],
      protocols: ["ERC20"],
      tags: ["staking", "rewards", "defi"],
      metadata: "MTK token staking contract with rewards"
    }
  ];

  for (const contract of contractsToRegister) {
    try {
      console.log(`\n注册合约: ${contract.name}`);
      
      // 读取ABI
      const abi = readABI(contract.name);
      if (!abi) {
        console.log(`⚠️  跳过 ${contract.name}，无法读取ABI`);
        continue;
      }
      
      // 准备端点（使用合约地址作为端点）
      const endpoints = [contract.address];
      const weights = [100]; // 权重为100
      
      // 注册到RegisterCore
      const tx = await registerCore.registerAgent(
        contract.name,
        endpoints,
        weights,
        contract.services,
        contract.protocols,
        contract.tags,
        contract.metadata,
        abi, // 这个参数对应contractABI
        { value: ethers.parseEther("0.01") }
      );
      
      await tx.wait();
      console.log(`✅ ${contract.name} 注册成功!`);
      console.log(`   - 地址: ${contract.address}`);
      console.log(`   - 服务: ${contract.services.join(", ")}`);
      console.log(`   - 协议: ${contract.protocols.join(", ")}`);
      console.log(`   - 标签: ${contract.tags.join(", ")}`);
      console.log(`   - ABI长度: ${abi.length} 字符`);
      
    } catch (error) {
      console.log(`❌ 注册 ${contract.name} 失败: ${error.message}`);
    }
  }

  console.log("\n部署完成! 🎉");
  console.log("=== 合约地址汇总 ===");
  console.log("RegisterCore:", coreAddress);
  console.log("RegisterExtended:", extendedAddress);
  console.log("=====================");
  console.log("你可以在区块链浏览器中查看这些合约");
  
  // 保存部署信息到文件
  const deploymentInfo = {
    network: "customnet",
    deployer: deployer.address,
    coreContract: coreAddress,
    extendedContract: extendedAddress,
    deploymentTime: new Date().toISOString(),
    version: await registerCore.getVersion()
  };
  
  console.log("\n部署信息已保存到 deployment-info.json");
  require('fs').writeFileSync(
    'deployment-info.json', 
    JSON.stringify(deploymentInfo, null, 2)
  );
}

main().catch((error) => {
  console.error("部署失败:", error);
  process.exitCode = 1;
});
