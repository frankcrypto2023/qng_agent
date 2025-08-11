// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

/**
 * @title IRegisterCore
 * @dev Interface for Register Core contract
 */
interface IRegisterCore {
    // Events
    event AgentRegistered(
        string indexed agentId,
        address indexed owner,
        string[] services,
        string[] protocols
    );
    
    event AgentUpdated(
        string indexed agentId,
        address indexed owner,
        string[] endpoints,
        uint256[] weights
    );
    
    event AgentDeregistered(
        string indexed agentId,
        address indexed owner
    );
    
    event ServiceRegistered(
        string indexed agentId,
        string indexed service
    );
    
    event RegistrationFeeUpdated(
        uint256 oldFee,
        uint256 newFee
    );
}

/**
 * @title RegisterCore
 * @dev Core implementation of Register with basic agent registration and discovery
 */
contract RegisterCore is IRegisterCore {
    
    // Constants
    uint256 public constant VERSION = 2;
    uint256 public constant MAX_SERVICES_PER_AGENT = 50;
    uint256 public constant MAX_PROTOCOLS_PER_AGENT = 20;
    uint256 public constant MAX_TAGS_PER_AGENT = 30;
    uint256 public constant BASE_REPUTATION = 50;
    
    // State variables
    address public owner;
    uint256 public registrationFee = 0.01 ether;
    bool public paused = false;
    
    // Agent information structure
    struct AgentInfo {
        address owner;
        string[] endpoints;
        uint256[] weights;
        uint256 registrationTime;
        uint256 lastUpdateTime;
        bool isActive;
        string metadata;
        string[] services;
        string[] protocols;
        uint256 reputation;
        string abi; // 存储合约的ABI
        mapping(string => bool) hasService;
        mapping(string => bool) supportsProtocol;
    }
    
    // Main registries
    mapping(string => AgentInfo) private agents;
    mapping(uint256 => string) private agentIdByIndex;
    uint256 public totalAgents;
    
    // Service discovery indices
    mapping(string => string[]) private agentsByService;
    mapping(string => string[]) private agentsByProtocol;
    mapping(string => string[]) private agentsByTag;
    mapping(string => string[]) private agentTags;
    
    // Owner tracking
    mapping(address => string[]) private agentsByOwner;
    
    // Modifiers
    modifier onlyOwner() {
        require(msg.sender == owner, "Only owner can call this function");
        _;
    }
    
    modifier whenNotPaused() {
        require(!paused, "Contract is paused");
        _;
    }
    
    modifier onlyAgentOwner(string memory agentId) {
        require(agents[agentId].owner == msg.sender, "Only agent owner can perform this action");
        _;
    }
    
    modifier agentExists(string memory agentId) {
        require(agents[agentId].isActive, "Agent does not exist");
        _;
    }
    
    modifier validEndpoints(string[] memory endpoints, uint256[] memory weights) {
        require(endpoints.length > 0, "At least one endpoint required");
        require(endpoints.length == weights.length, "Endpoints and weights length mismatch");
        
        uint256 totalWeight = 0;
        for (uint256 i = 0; i < weights.length; i++) {
            require(weights[i] > 0, "Weight must be greater than 0");
            totalWeight += weights[i];
            require(bytes(endpoints[i]).length > 0, "Empty endpoint");
        }
        require(totalWeight > 0, "Total weight must be greater than 0");
        _;
    }
    
    // Constructor
    constructor() {
        owner = msg.sender;
    }
    
    // Main registration function
    function registerAgent(
        string memory agentId,
        string[] memory endpoints,
        uint256[] memory weights,
        string[] memory services,
        string[] memory protocols,
        string[] memory tags,
        string memory metadata,
        string memory contractABI
    ) 
        external 
        payable 
        whenNotPaused
        validEndpoints(endpoints, weights)
    {
        require(bytes(agentId).length > 0, "Empty agent ID");
        require(!agents[agentId].isActive, "Agent already registered");
        require(msg.value >= registrationFee, "Insufficient fee");
        require(services.length <= MAX_SERVICES_PER_AGENT, "Too many services");
        require(protocols.length <= MAX_PROTOCOLS_PER_AGENT, "Too many protocols");
        require(tags.length <= MAX_TAGS_PER_AGENT, "Too many tags");
        
        // Store basic agent info
        AgentInfo storage newAgent = agents[agentId];
        newAgent.owner = msg.sender;
        newAgent.endpoints = endpoints;
        newAgent.weights = weights;
        newAgent.registrationTime = block.timestamp;
        newAgent.lastUpdateTime = block.timestamp;
        newAgent.isActive = true;
        newAgent.metadata = metadata;
        newAgent.abi = contractABI;
        newAgent.services = services;
        newAgent.protocols = protocols;
        newAgent.reputation = BASE_REPUTATION;
        
        // Index services
        for (uint256 i = 0; i < services.length; i++) {
            require(bytes(services[i]).length > 0, "Empty service name");
            newAgent.hasService[services[i]] = true;
            agentsByService[services[i]].push(agentId);
        }
        
        // Index protocols
        for (uint256 i = 0; i < protocols.length; i++) {
            require(bytes(protocols[i]).length > 0, "Empty protocol name");
            newAgent.supportsProtocol[protocols[i]] = true;
            agentsByProtocol[protocols[i]].push(agentId);
        }
        
        // Index tags
        agentTags[agentId] = tags;
        for (uint256 i = 0; i < tags.length; i++) {
            if (bytes(tags[i]).length > 0) {
                agentsByTag[tags[i]].push(agentId);
            }
        }
        
        // Update global indices
        agentIdByIndex[totalAgents] = agentId;
        totalAgents++;
        agentsByOwner[msg.sender].push(agentId);
        
        // Refund excess payment
        if (msg.value > registrationFee) {
            (bool success, ) = payable(msg.sender).call{value: msg.value - registrationFee}("");
            require(success, "Refund failed");
        }
        
        emit AgentRegistered(agentId, msg.sender, services, protocols);
    }
    
    // Update agent endpoints and weights
    function updateAgent(
        string memory agentId,
        string[] memory endpoints,
        uint256[] memory weights,
        string memory metadata
    ) 
        external 
        whenNotPaused
        agentExists(agentId)
        onlyAgentOwner(agentId)
        validEndpoints(endpoints, weights)
    {
        AgentInfo storage agent = agents[agentId];
        agent.endpoints = endpoints;
        agent.weights = weights;
        agent.metadata = metadata;
        agent.lastUpdateTime = block.timestamp;
        
        emit AgentUpdated(agentId, msg.sender, endpoints, weights);
    }
    
    // Deregister an agent
    function deregisterAgent(string memory agentId) 
        external 
        agentExists(agentId)
        onlyAgentOwner(agentId)
    {
        AgentInfo storage agent = agents[agentId];
        agent.isActive = false;
        
        // Remove from service indices
        for (uint256 i = 0; i < agent.services.length; i++) {
            _removeFromArray(agentsByService[agent.services[i]], agentId);
        }
        
        // Remove from protocol indices
        for (uint256 i = 0; i < agent.protocols.length; i++) {
            _removeFromArray(agentsByProtocol[agent.protocols[i]], agentId);
        }
        
        // Remove from tag indices
        string[] memory tags = agentTags[agentId];
        for (uint256 i = 0; i < tags.length; i++) {
            _removeFromArray(agentsByTag[tags[i]], agentId);
        }
        delete agentTags[agentId];
        
        // Remove from owner's list
        _removeFromArray(agentsByOwner[msg.sender], agentId);
        
        emit AgentDeregistered(agentId, msg.sender);
    }
    
    // Discovery functions
    function discoverAgentsByService(string memory service) 
        external 
        view 
        returns (string[] memory)
    {
        return agentsByService[service];
    }
    
    function discoverAgentsByProtocol(string memory protocol) 
        external 
        view 
        returns (string[] memory)
    {
        return agentsByProtocol[protocol];
    }
    
    function discoverAgentsByTag(string memory tag) 
        external 
        view 
        returns (string[] memory)
    {
        return agentsByTag[tag];
    }
    
    // Add service to existing agent
    function addService(
        string memory agentId,
        string memory service
    ) 
        external 
        whenNotPaused
        agentExists(agentId)
        onlyAgentOwner(agentId)
    {
        require(bytes(service).length > 0, "Empty service name");
        require(!agents[agentId].hasService[service], "Service already registered");
        require(agents[agentId].services.length < MAX_SERVICES_PER_AGENT, "Too many services");
        
        agents[agentId].services.push(service);
        agents[agentId].hasService[service] = true;
        agentsByService[service].push(agentId);
        agents[agentId].lastUpdateTime = block.timestamp;
        
        emit ServiceRegistered(agentId, service);
    }
    
    // Query functions
    function getAgent(string memory agentId) 
        external 
        view 
        agentExists(agentId)
        returns (
            address agentOwner,
            string[] memory endpoints,
            uint256[] memory weights,
            uint256 registrationTime,
            uint256 lastUpdateTime,
            string memory metadata
        )
    {
        AgentInfo storage agent = agents[agentId];
        return (
            agent.owner,
            agent.endpoints,
            agent.weights,
            agent.registrationTime,
            agent.lastUpdateTime,
            agent.metadata
        );
    }
    
    function getAgentDetails(string memory agentId) 
        external 
        view 
        agentExists(agentId)
        returns (
            address agentOwner,
            string[] memory endpoints,
            string[] memory services,
            string[] memory protocols,
            uint256 reputation,
            string memory metadata,
            string memory contractABI
        )
    {
        AgentInfo storage agent = agents[agentId];
        return (
            agent.owner,
            agent.endpoints,
            agent.services,
            agent.protocols,
            agent.reputation,
            agent.metadata,
            agent.abi
        );
    }
    
    function getAgentABI(string memory agentId) 
        external 
        view 
        agentExists(agentId)
        returns (string memory contractABI)
    {
        return agents[agentId].abi;
    }
    
    function getAgentEndpoint(string memory agentId) 
        external 
        view 
        agentExists(agentId)
        returns (string memory endpoint)
    {
        AgentInfo storage agent = agents[agentId];
        
        if (agent.endpoints.length == 1) {
            return agent.endpoints[0];
        }
        
        // Calculate total weight
        uint256 totalWeight = 0;
        for (uint256 i = 0; i < agent.weights.length; i++) {
            totalWeight += agent.weights[i];
        }
        
        // Generate pseudo-random number
        uint256 random = uint256(keccak256(abi.encodePacked(
            block.timestamp,
            block.prevrandao,
            msg.sender,
            agentId
        ))) % totalWeight;
        
        // Select endpoint based on weighted random
        uint256 weightSum = 0;
        for (uint256 i = 0; i < agent.endpoints.length; i++) {
            weightSum += agent.weights[i];
            if (random < weightSum) {
                return agent.endpoints[i];
            }
        }
        
        // Fallback
        return agent.endpoints[0];
    }
    
    function getAgentsByOwner(address agentOwner) 
        external 
        view 
        returns (string[] memory)
    {
        return agentsByOwner[agentOwner];
    }
    
    function isAgentActive(string memory agentId) 
        external 
        view 
        returns (bool)
    {
        return agents[agentId].isActive;
    }
    
    function getAgentIdByIndex(uint256 index) 
        external 
        view 
        returns (string memory)
    {
        require(index < totalAgents, "Index out of bounds");
        return agentIdByIndex[index];
    }
    
    function getVersion() external pure returns (uint256) {
        return VERSION;
    }
    
    // Admin functions
    function updateRegistrationFee(uint256 newFee) 
        external 
        onlyOwner 
    {
        uint256 oldFee = registrationFee;
        registrationFee = newFee;
        emit RegistrationFeeUpdated(oldFee, newFee);
    }
    
    function withdrawFees() 
        external 
        onlyOwner 
    {
        uint256 balance = address(this).balance;
        require(balance > 0, "No fees to withdraw");
        (bool success, ) = payable(owner).call{value: balance}("");
        require(success, "Withdrawal failed");
    }
    
    function pause() external onlyOwner {
        paused = true;
    }
    
    function unpause() external onlyOwner {
        paused = false;
    }
    
    function transferOwnership(address newOwner) external onlyOwner {
        require(newOwner != address(0), "Invalid new owner");
        owner = newOwner;
    }
    
    // Internal helper functions
    function _removeFromArray(string[] storage array, string memory value) private {
        for (uint256 i = 0; i < array.length; i++) {
            if (keccak256(bytes(array[i])) == keccak256(bytes(value))) {
                array[i] = array[array.length - 1];
                array.pop();
                break;
            }
        }
    }
    
    // Receive function to accept ETH
    receive() external payable {}
    
    // Fallback function
    fallback() external payable {}
}
