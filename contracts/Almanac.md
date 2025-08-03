# Almanac V2 部署和使用指南

## 1. 部署合约

### 使用 Hardhat 部署

```javascript
// scripts/deploy.js
const hre = require("hardhat");

async function main() {
  const AlmanacV2 = await hre.ethers.getContractFactory("AlmanacV2");
  const almanac = await AlmanacV2.deploy();
  
  await almanac.deployed();
  
  console.log("AlmanacV2 deployed to:", almanac.address);
  
  // 设置初始参数
  await almanac.updateRegistrationFee(ethers.utils.parseEther("0.01"));
  console.log("Registration fee set to 0.01 ETH");
}

main()
  .then(() => process.exit(0))
  .catch((error) => {
    console.error(error);
    process.exit(1);
  });
```

### 使用 Truffle 部署

```javascript
// migrations/2_deploy_almanac.js
const AlmanacV2 = artifacts.require("AlmanacV2");

module.exports = async function(deployer) {
  await deployer.deploy(AlmanacV2);
  const almanac = await AlmanacV2.deployed();
  
  // 设置初始参数
  await almanac.updateRegistrationFee(web3.utils.toWei("0.01", "ether"));
};
```

## 2. 与合约交互

### 2.1 注册代理

```javascript
// 使用 ethers.js
const almanac = new ethers.Contract(almanacAddress, almanacABI, signer);

// 注册一个 AI 代理
const tx = await almanac.registerAgent(
  "agent1qww3ju3h6kfcuqf54gkghvt2pqe8qp97a7nzm2vp8plfxflc0epzcjsv79t", // agentId
  ["https://agent1.example.com:8000", "https://backup.example.com:8000"], // endpoints
  [70, 30], // weights
  ["nlp_processing", "sentiment_analysis", "text_summarization"], // services
  ["http", "https", "grpc"], // protocols
  ["ai", "nlp", "machine_learning"], // tags
  JSON.stringify({
    name: "NLP Expert Agent",
    version: "1.0.0",
    description: "Specialized in natural language processing tasks"
  }),
  { value: ethers.utils.parseEther("0.01") } // 注册费
);

await tx.wait();
console.log("Agent registered successfully");
```

### 2.2 发现代理

```javascript
// 按服务发现
const nlpAgents = await almanac.discoverAgentsByService("nlp_processing");
console.log("NLP Agents:", nlpAgents);

// 按协议发现
const grpcAgents = await almanac.discoverAgentsByProtocol("grpc");
console.log("gRPC Agents:", grpcAgents);

// 高级搜索
const qualifiedAgents = await almanac.searchAgents(
  "sentiment_analysis",  // service
  "https",              // protocol
  80                    // minimum reputation
);
console.log("Qualified Agents:", qualifiedAgents);
```

### 2.3 请求服务

```javascript
// 发布服务请求
const requestTx = await almanac.requestService(
  "agent1qww3ju3h6kfcuqf54gkghvt2pqe8qp97a7nzm2vp8plfxflc0epzcjsv79t",
  "image_recognition",
  JSON.stringify({
    format: ["jpeg", "png"],
    maxSize: "5MB",
    requirements: "real-time processing",
    budget: "50 FET"
  }),
  86400 // 24小时有效期
);

const receipt = await requestTx.wait();
const requestId = receipt.events[0].args.requestId;
console.log("Service request created:", requestId);

// 响应服务请求
await almanac.respondToServiceRequest(
  requestId,
  "agent2xyz..." // 响应者的 agentId
);
```

### 2.4 创建协作组

```javascript
// 创建任务组
await almanac.createCollaborationGroup(
  "data_analysis_team_001",
  "task_force",
  [
    "agent1qww3ju3h6kfcuqf54gkghvt2pqe8qp97a7nzm2vp8plfxflc0epzcjsv79t",
    "agent2abc...",
    "agent3def..."
  ],
  JSON.stringify({
    task: "Analyze Q4 market trends",
    deadline: "2024-12-31",
    coordinator: "agent1qww3ju3h6kfcuqf54gkghvt2pqe8qp97a7nzm2vp8plfxflc0epzcjsv79t"
  })
);

// 加入现有组
await almanac.joinCollaborationGroup(
  "data_analysis_team_001",
  "agent4ghi..."
);
```

### 2.5 记录交互和评分

```javascript
// 记录交互
await almanac.recordInteraction(
  "agent1qww3ju3h6kfcuqf54gkghvt2pqe8qp97a7nzm2vp8plfxflc0epzcjsv79t",
  "agent2abc..."
);

// 评分（1-5分）
await almanac.rateAgent(
  "agent1qww3ju3h6kfcuqf54gkghvt2pqe8qp97a7nzm2vp8plfxflc0epzcjsv79t", // rater
  "agent2abc...", // rated
  5 // score
);
```

## 3. 集成到 uAgents 框架

### 3.1 扩展 Agent 类

```python
from uagents import Agent, Context
from web3 import Web3
import json

class AlmanacAgent(Agent):
    def __init__(self, name, seed, almanac_address, private_key, **kwargs):
        super().__init__(name=name, seed=seed, **kwargs)
        
        # Web3 设置
        self.w3 = Web3(Web3.HTTPProvider('https://rpc.fetch.ai'))
        self.almanac = self.w3.eth.contract(
            address=almanac_address,
            abi=ALMANAC_ABI
        )
        self.account = self.w3.eth.account.from_key(private_key)
        
    async def register_in_almanac(self, services, protocols, tags):
        """在 Almanac 合约中注册"""
        tx = self.almanac.functions.registerAgent(
            self.address,  # agentId
            [self.endpoint],  # endpoints
            [100],  # weights
            services,
            protocols,
            tags,
            json.dumps({"name": self.name})
        ).build_transaction({
            'from': self.account.address,
            'value': self.w3.toWei(0.01, 'ether'),
            'nonce': self.w3.eth.get_transaction_count(self.account.address),
        })
        
        signed_tx = self.account.sign_transaction(tx)
        tx_hash = self.w3.eth.send_raw_transaction(signed_tx.rawTransaction)
        receipt = self.w3.eth.wait_for_transaction_receipt(tx_hash)
        
        return receipt
    
    async def discover_agents(self, service):
        """发现提供特定服务的代理"""
        agents = self.almanac.functions.discoverAgentsByService(service).call()
        return agents
    
    async def request_service(self, service_type, requirements, duration=86400):
        """请求服务"""
        tx = self.almanac.functions.requestService(
            self.address,
            service_type,
            json.dumps(requirements),
            duration
        ).build_transaction({
            'from': self.account.address,
            'nonce': self.w3.eth.get_transaction_count(self.account.address),
        })
        
        signed_tx = self.account.sign_transaction(tx)
        tx_hash = self.w3.eth.send_raw_transaction(signed_tx.rawTransaction)
        receipt = self.w3.eth.wait_for_transaction_receipt(tx_hash)
        
        # 从事件中获取 requestId
        request_id = receipt['logs'][0]['topics'][1]
        return int(request_id, 16)
```

### 3.2 使用示例

```python
# 创建支持 Almanac 的代理
agent = AlmanacAgent(
    name="data_analyst",
    seed="analyst recovery phrase",
    almanac_address="0x...",  # Almanac 合约地址
    private_key="0x...",  # 用于链上交易的私钥
    port=8000,
    endpoint=["http://127.0.0.1:8000/submit"]
)

@agent.on_event("startup")
async def register_agent(ctx: Context):
    # 在 Almanac 中注册
    receipt = await agent.register_in_almanac(
        services=["data_analysis", "visualization"],
        protocols=["http", "websocket"],
        tags=["analytics", "bigdata"]
    )
    ctx.logger.info(f"Registered in Almanac: {receipt['transactionHash'].hex()}")
    
    # 发现其他数据分析代理
    analysts = await agent.discover_agents("data_analysis")
    ctx.logger.info(f"Found {len(analysts)} data analysis agents")

@agent.on_interval(period=300.0)  # 每5分钟
async def check_service_requests(ctx: Context):
    # 检查数据分析服务请求
    active_requests = await agent.almanac.functions.getActiveServiceRequests(
        "data_analysis"
    ).call()
    
    for request_id in active_requests:
        request_info = await agent.almanac.functions.getServiceRequest(
            request_id
        ).call()
        
        ctx.logger.info(f"Found request {request_id}: {request_info}")
        
        # 决定是否响应
        requirements = json.loads(request_info[2])  # requirements
        if can_handle_requirements(requirements):
            await respond_to_request(request_id)

if __name__ == "__main__":
    agent.run()
```

## 4. 监控和维护

### 4.1 事件监听

```javascript
// 监听新代理注册
almanac.on("AgentRegistered", (agentId, owner, services, protocols) => {
  console.log(`New agent registered: ${agentId}`);
  console.log(`Services: ${services.join(", ")}`);
});

// 监听服务请求
almanac.on("ServiceRequested", (requestId, serviceType, requesterAgentId) => {
  console.log(`New service request #${requestId} for ${serviceType}`);
});

// 监听协作组创建
almanac.on("CollaborationGroupCreated", (groupId, groupType, members) => {
  console.log(`New ${groupType} group: ${groupId} with ${members.length} members`);
});
```

### 4.2 管理功能

```javascript
// 管理员功能
const owner = await almanac.owner();
if (account === owner) {
  // 更新注册费
  await almanac.updateRegistrationFee(ethers.utils.parseEther("0.02"));
  
  // 提取费用
  await almanac.withdrawFees();
  
  // 紧急暂停
  await almanac.pause();
}
```

## 5. 最佳实践

1. **服务命名规范**：使用清晰、标准化的服务名称（如 `nlp_processing`, `image_recognition`）
2. **协议标准化**：使用标准协议名称（如 `http`, `https`, `grpc`, `websocket`）
3. **元数据格式**：使用 JSON 格式存储结构化元数据
4. **错误处理**：始终处理交易失败和合约调用错误
5. **Gas 优化**：批量操作时使用 `batchRegisterAgents`
6. **安全考虑**：保护私钥，使用硬件钱包或安全的密钥管理系统