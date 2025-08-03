// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

import "@openzeppelin/contracts/access/Ownable.sol";
import "@openzeppelin/contracts/security/ReentrancyGuard.sol";
import "@openzeppelin/contracts/security/Pausable.sol";

/**
 * @title Almanac
 * @dev A decentralized registry for AI agents in the Fetch.ai ecosystem
 */
contract Almanac is Ownable, ReentrancyGuard, Pausable {
    
    // Contract version for compatibility checking
    uint256 public constant VERSION = 1;
    
    // Registration fee (can be adjusted by owner)
    uint256 public registrationFee = 0.01 ether;
    
    // Struct to store agent information
    struct AgentInfo {
        address owner;              // Owner's wallet address
        string[] endpoints;         // Service endpoints
        uint256[] weights;          // Weights for each endpoint
        uint256 registrationTime;   // When the agent was registered
        uint256 lastUpdateTime;     // Last update timestamp
        bool isActive;              // Whether the agent is active
        string metadata;            // Additional metadata (JSON format)
    }
    
    // Mapping from agent ID to agent information
    mapping(string => AgentInfo) private agents;
    
    // Mapping to track registered agent IDs for enumeration
    mapping(uint256 => string) private agentIdByIndex;
    uint256 public totalAgents;
    
    // Mapping to track agents owned by an address
    mapping(address => string[]) private agentsByOwner;
    
    // Events
    event AgentRegistered(
        string indexed agentId,
        address indexed owner,
        string[] endpoints,
        uint256[] weights
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
    
    event RegistrationFeeUpdated(
        uint256 oldFee,
        uint256 newFee
    );
    
    event EndpointSelected(
        string indexed agentId,
        string endpoint,
        uint256 weight
    );
    
    // Modifiers
    modifier onlyAgentOwner(string memory agentId) {
        require(
            agents[agentId].owner == msg.sender,
            "Almanac: Only agent owner can perform this action"
        );
        _;
    }
    
    modifier agentExists(string memory agentId) {
        require(agents[agentId].isActive, "Almanac: Agent does not exist");
        _;
    }
    
    modifier validEndpoints(string[] memory endpoints, uint256[] memory weights) {
        require(endpoints.length > 0, "Almanac: At least one endpoint required");
        require(endpoints.length == weights.length, "Almanac: Endpoints and weights length mismatch");
        
        uint256 totalWeight = 0;
        for (uint256 i = 0; i < weights.length; i++) {
            require(weights[i] > 0, "Almanac: Weight must be greater than 0");
            totalWeight += weights[i];
            
            // Validate endpoint format (basic check)
            require(bytes(endpoints[i]).length > 0, "Almanac: Empty endpoint");
        }
        require(totalWeight > 0, "Almanac: Total weight must be greater than 0");
        _;
    }
    
    constructor() {
        // Constructor
    }
    
    /**
     * @dev Register a new agent
     * @param agentId Unique identifier for the agent
     * @param endpoints Array of service endpoints
     * @param weights Array of weights for each endpoint
     * @param metadata Optional metadata in JSON format
     */
    function registerAgent(
        string memory agentId,
        string[] memory endpoints,
        uint256[] memory weights,
        string memory metadata
    ) 
        external 
        payable 
        nonReentrant 
        whenNotPaused
        validEndpoints(endpoints, weights)
    {
        require(bytes(agentId).length > 0, "Almanac: Agent ID cannot be empty");
        require(!agents[agentId].isActive, "Almanac: Agent already registered");
        require(msg.value >= registrationFee, "Almanac: Insufficient registration fee");
        
        // Create new agent info
        AgentInfo storage newAgent = agents[agentId];
        newAgent.owner = msg.sender;
        newAgent.endpoints = endpoints;
        newAgent.weights = weights;
        newAgent.registrationTime = block.timestamp;
        newAgent.lastUpdateTime = block.timestamp;
        newAgent.isActive = true;
        newAgent.metadata = metadata;
        
        // Add to indices
        agentIdByIndex[totalAgents] = agentId;
        totalAgents++;
        agentsByOwner[msg.sender].push(agentId);
        
        // Refund excess payment
        if (msg.value > registrationFee) {
            payable(msg.sender).transfer(msg.value - registrationFee);
        }
        
        emit AgentRegistered(agentId, msg.sender, endpoints, weights);
    }
    
    /**
     * @dev Update agent information
     * @param agentId Agent identifier
     * @param endpoints New array of service endpoints
     * @param weights New array of weights
     * @param metadata Updated metadata
     */
    function updateAgent(
        string memory agentId,
        string[] memory endpoints,
        uint256[] memory weights,
        string memory metadata
    ) 
        external 
        nonReentrant
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
    
    /**
     * @dev Deregister an agent
     * @param agentId Agent identifier
     */
    function deregisterAgent(string memory agentId) 
        external 
        nonReentrant
        agentExists(agentId)
        onlyAgentOwner(agentId)
    {
        agents[agentId].isActive = false;
        
        // Remove from owner's list
        string[] storage ownerAgents = agentsByOwner[msg.sender];
        for (uint256 i = 0; i < ownerAgents.length; i++) {
            if (keccak256(bytes(ownerAgents[i])) == keccak256(bytes(agentId))) {
                ownerAgents[i] = ownerAgents[ownerAgents.length - 1];
                ownerAgents.pop();
                break;
            }
        }
        
        emit AgentDeregistered(agentId, msg.sender);
    }
    
    /**
     * @dev Get agent information
     * @param agentId Agent identifier
     * @return owner Agent owner address
     * @return endpoints Service endpoints
     * @return weights Endpoint weights
     * @return registrationTime Registration timestamp
     * @return lastUpdateTime Last update timestamp
     * @return metadata Agent metadata
     */
    function getAgent(string memory agentId) 
        external 
        view 
        agentExists(agentId)
        returns (
            address owner,
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
    
    /**
     * @dev Get a weighted random endpoint for an agent
     * @param agentId Agent identifier
     * @return endpoint Selected endpoint based on weights
     */
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
        
        // Generate pseudo-random number based on block data
        uint256 random = uint256(keccak256(abi.encodePacked(
            block.timestamp,
            block.difficulty,
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
        
        // Fallback (should never reach here)
        return agent.endpoints[0];
    }
    
    /**
     * @dev Get all agents owned by an address
     * @param owner Address to query
     * @return agentIds Array of agent IDs owned by the address
     */
    function getAgentsByOwner(address owner) 
        external 
        view 
        returns (string[] memory agentIds)
    {
        return agentsByOwner[owner];
    }
    
    /**
     * @dev Check if an agent is registered and active
     * @param agentId Agent identifier
     * @return isActive Whether the agent is active
     */
    function isAgentActive(string memory agentId) 
        external 
        view 
        returns (bool)
    {
        return agents[agentId].isActive;
    }
    
    /**
     * @dev Get agent at specific index (for enumeration)
     * @param index Index to query
     * @return agentId Agent identifier at the index
     */
    function getAgentIdByIndex(uint256 index) 
        external 
        view 
        returns (string memory)
    {
        require(index < totalAgents, "Almanac: Index out of bounds");
        return agentIdByIndex[index];
    }
    
    /**
     * @dev Update registration fee (only owner)
     * @param newFee New registration fee in wei
     */
    function updateRegistrationFee(uint256 newFee) 
        external 
        onlyOwner 
    {
        uint256 oldFee = registrationFee;
        registrationFee = newFee;
        emit RegistrationFeeUpdated(oldFee, newFee);
    }
    
    /**
     * @dev Withdraw collected fees (only owner)
     */
    function withdrawFees() 
        external 
        onlyOwner 
        nonReentrant
    {
        uint256 balance = address(this).balance;
        require(balance > 0, "Almanac: No fees to withdraw");
        payable(owner()).transfer(balance);
    }
    
    /**
     * @dev Pause contract (only owner)
     */
    function pause() external onlyOwner {
        _pause();
    }
    
    /**
     * @dev Unpause contract (only owner)
     */
    function unpause() external onlyOwner {
        _unpause();
    }
    
    /**
     * @dev Get contract version
     * @return version Contract version number
     */
    function getVersion() external pure returns (uint256) {
        return VERSION;
    }
    
    /**
     * @dev Batch register multiple agents (gas optimization)
     * @param agentIds Array of agent identifiers
     * @param endpointsArray Array of endpoint arrays
     * @param weightsArray Array of weight arrays
     * @param metadataArray Array of metadata strings
     */
    function batchRegisterAgents(
        string[] memory agentIds,
        string[][] memory endpointsArray,
        uint256[][] memory weightsArray,
        string[] memory metadataArray
    ) 
        external 
        payable 
        nonReentrant 
        whenNotPaused
    {
        require(
            agentIds.length == endpointsArray.length &&
            agentIds.length == weightsArray.length &&
            agentIds.length == metadataArray.length,
            "Almanac: Array length mismatch"
        );
        
        uint256 totalFee = registrationFee * agentIds.length;
        require(msg.value >= totalFee, "Almanac: Insufficient fee for batch registration");
        
        for (uint256 i = 0; i < agentIds.length; i++) {
            // Validate each registration
            require(bytes(agentIds[i]).length > 0, "Almanac: Empty agent ID");
            require(!agents[agentIds[i]].isActive, "Almanac: Agent already registered");
            require(endpointsArray[i].length > 0, "Almanac: No endpoints provided");
            require(endpointsArray[i].length == weightsArray[i].length, "Almanac: Endpoints/weights mismatch");
            
            // Register agent
            AgentInfo storage newAgent = agents[agentIds[i]];
            newAgent.owner = msg.sender;
            newAgent.endpoints = endpointsArray[i];
            newAgent.weights = weightsArray[i];
            newAgent.registrationTime = block.timestamp;
            newAgent.lastUpdateTime = block.timestamp;
            newAgent.isActive = true;
            newAgent.metadata = metadataArray[i];
            
            agentIdByIndex[totalAgents] = agentIds[i];
            totalAgents++;
            agentsByOwner[msg.sender].push(agentIds[i]);
            
            emit AgentRegistered(agentIds[i], msg.sender, endpointsArray[i], weightsArray[i]);
        }
        
        // Refund excess
        if (msg.value > totalFee) {
            payable(msg.sender).transfer(msg.value - totalFee);
        }
    }
}