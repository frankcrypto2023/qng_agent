// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

/**
 * @title IAlmanacV2
 * @dev Interface for Almanac V2 contract
 */
interface IAlmanacV2 {
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
    
    event ServiceRequested(
        uint256 indexed requestId,
        string indexed serviceType,
        string requesterAgentId
    );
    
    event ServiceResponseSubmitted(
        uint256 indexed requestId,
        string indexed respondentAgentId
    );
    
    event CollaborationGroupCreated(
        string indexed groupId,
        string groupType,
        string[] initialMembers
    );
    
    event AgentJoinedGroup(
        string indexed groupId,
        string indexed agentId
    );
    
    event InteractionRecorded(
        string indexed agent1,
        string indexed agent2,
        uint256 interactionCount
    );
    
    event ReputationUpdated(
        string indexed agentId,
        uint256 newReputation
    );
    
    event RegistrationFeeUpdated(
        uint256 oldFee,
        uint256 newFee
    );
}

/**
 * @title AlmanacV2
 * @dev Complete implementation of Almanac V2 with discovery and collaboration features
 */
contract AlmanacV2 is IAlmanacV2 {
    
    // Constants
    uint256 public constant VERSION = 2;
    uint256 public constant MAX_SERVICES_PER_AGENT = 50;
    uint256 public constant MAX_PROTOCOLS_PER_AGENT = 20;
    uint256 public constant MAX_TAGS_PER_AGENT = 30;
    uint256 public constant MAX_GROUP_MEMBERS = 100;
    uint256 public constant MIN_REPUTATION = 0;
    uint256 public constant MAX_REPUTATION = 100;
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
        mapping(string => bool) hasService;
        mapping(string => bool) supportsProtocol;
    }
    
    // Service request structure
    struct ServiceRequest {
        string requesterAgentId;
        string serviceType;
        string requirements;
        uint256 timestamp;
        uint256 expiryTime;
        bool isActive;
        string[] respondents;
        mapping(string => bool) hasResponded;
    }
    
    // Collaboration group structure
    struct CollaborationGroup {
        string groupId;
        string[] members;
        string groupType;
        string metadata;
        uint256 createdAt;
        bool isActive;
        address creator;
        mapping(string => bool) isMember;
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
    
    // Service request system
    mapping(uint256 => ServiceRequest) public serviceRequests;
    uint256 public serviceRequestCounter;
    mapping(string => uint256[]) private requestsByService;
    mapping(string => uint256[]) private activeRequestsByAgent;
    
    // Collaboration groups
    mapping(string => CollaborationGroup) public collaborationGroups;
    string[] public groupIds;
    mapping(string => string[]) private agentGroups;
    
    // Interaction and reputation
    mapping(string => mapping(string => uint256)) public interactions;
    mapping(string => mapping(string => uint256)) public ratings;
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
    
    // Main registration function with extended parameters
    function registerAgent(
        string memory agentId,
        string[] memory endpoints,
        uint256[] memory weights,
        string[] memory services,
        string[] memory protocols,
        string[] memory tags,
        string memory metadata
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
    
    // Advanced search with multiple criteria
    function searchAgents(
        string memory service,
        string memory protocol,
        uint256 minReputation
    ) 
        external 
        view 
        returns (string[] memory matchingAgents)
    {
        string[] memory serviceAgents = agentsByService[service];
        uint256 matchCount = 0;
        
        // First pass: count matches
        for (uint256 i = 0; i < serviceAgents.length; i++) {
            string memory agentId = serviceAgents[i];
            AgentInfo storage agent = agents[agentId];
            if (agent.isActive &&
                agent.supportsProtocol[protocol] &&
                agent.reputation >= minReputation) {
                matchCount++;
            }
        }
        
        // Second pass: collect matches
        matchingAgents = new string[](matchCount);
        uint256 index = 0;
        for (uint256 i = 0; i < serviceAgents.length; i++) {
            string memory agentId = serviceAgents[i];
            AgentInfo storage agent = agents[agentId];
            if (agent.isActive &&
                agent.supportsProtocol[protocol] &&
                agent.reputation >= minReputation) {
                matchingAgents[index] = agentId;
                index++;
            }
        }
    }
    
    // Service request system
    function requestService(
        string memory requesterAgentId,
        string memory serviceType,
        string memory requirements,
        uint256 duration
    ) 
        external 
        agentExists(requesterAgentId)
        onlyAgentOwner(requesterAgentId)
        returns (uint256 requestId)
    {
        require(bytes(serviceType).length > 0, "Empty service type");
        require(duration > 0, "Duration must be positive");
        
        requestId = serviceRequestCounter++;
        
        ServiceRequest storage request = serviceRequests[requestId];
        request.requesterAgentId = requesterAgentId;
        request.serviceType = serviceType;
        request.requirements = requirements;
        request.timestamp = block.timestamp;
        request.expiryTime = block.timestamp + duration;
        request.isActive = true;
        
        requestsByService[serviceType].push(requestId);
        activeRequestsByAgent[requesterAgentId].push(requestId);
        
        emit ServiceRequested(requestId, serviceType, requesterAgentId);
    }
    
    function respondToServiceRequest(
        uint256 requestId,
        string memory respondentAgentId
    ) 
        external 
        agentExists(respondentAgentId)
        onlyAgentOwner(respondentAgentId)
    {
        ServiceRequest storage request = serviceRequests[requestId];
        require(request.isActive, "Request not active");
        require(block.timestamp < request.expiryTime, "Request expired");
        require(!request.hasResponded[respondentAgentId], "Already responded");
        require(
            agents[respondentAgentId].hasService[request.serviceType],
            "Agent doesn't provide this service"
        );
        
        request.respondents.push(respondentAgentId);
        request.hasResponded[respondentAgentId] = true;
        
        emit ServiceResponseSubmitted(requestId, respondentAgentId);
    }
    
    function getActiveServiceRequests(string memory serviceType) 
        external 
        view 
        returns (uint256[] memory activeRequests)
    {
        uint256[] memory allRequests = requestsByService[serviceType];
        uint256 activeCount = 0;
        
        // Count active requests
        for (uint256 i = 0; i < allRequests.length; i++) {
            ServiceRequest storage request = serviceRequests[allRequests[i]];
            if (request.isActive && block.timestamp < request.expiryTime) {
                activeCount++;
            }
        }
        
        // Collect active requests
        activeRequests = new uint256[](activeCount);
        uint256 index = 0;
        for (uint256 i = 0; i < allRequests.length; i++) {
            ServiceRequest storage request = serviceRequests[allRequests[i]];
            if (request.isActive && block.timestamp < request.expiryTime) {
                activeRequests[index] = allRequests[i];
                index++;
            }
        }
    }
    
    // Collaboration group management
    function createCollaborationGroup(
        string memory groupId,
        string memory groupType,
        string[] memory initialMembers,
        string memory metadata
    ) 
        external 
        whenNotPaused
    {
        require(bytes(groupId).length > 0, "Empty group ID");
        require(!collaborationGroups[groupId].isActive, "Group already exists");
        require(initialMembers.length > 0, "No initial members");
        require(initialMembers.length <= MAX_GROUP_MEMBERS, "Too many members");
        
        // Verify all members exist and creator is included
        bool creatorIncluded = false;
        for (uint256 i = 0; i < initialMembers.length; i++) {
            require(agents[initialMembers[i]].isActive, "Member agent doesn't exist");
            if (agents[initialMembers[i]].owner == msg.sender) {
                creatorIncluded = true;
            }
        }
        require(creatorIncluded, "Creator must be member");
        
        CollaborationGroup storage group = collaborationGroups[groupId];
        group.groupId = groupId;
        group.members = initialMembers;
        group.groupType = groupType;
        group.metadata = metadata;
        group.createdAt = block.timestamp;
        group.isActive = true;
        group.creator = msg.sender;
        
        for (uint256 i = 0; i < initialMembers.length; i++) {
            group.isMember[initialMembers[i]] = true;
            agentGroups[initialMembers[i]].push(groupId);
        }
        
        groupIds.push(groupId);
        
        emit CollaborationGroupCreated(groupId, groupType, initialMembers);
    }
    
    function joinCollaborationGroup(
        string memory groupId,
        string memory agentId
    ) 
        external 
        whenNotPaused
        agentExists(agentId)
        onlyAgentOwner(agentId)
    {
        require(collaborationGroups[groupId].isActive, "Group doesn't exist");
        require(!collaborationGroups[groupId].isMember[agentId], "Already a member");
        require(collaborationGroups[groupId].members.length < MAX_GROUP_MEMBERS, "Group is full");
        
        collaborationGroups[groupId].members.push(agentId);
        collaborationGroups[groupId].isMember[agentId] = true;
        agentGroups[agentId].push(groupId);
        
        emit AgentJoinedGroup(groupId, agentId);
    }
    
    // Interaction and reputation management
    function recordInteraction(
        string memory agent1,
        string memory agent2
    ) 
        external 
        agentExists(agent1)
        agentExists(agent2)
    {
        require(
            agents[agent1].owner == msg.sender || agents[agent2].owner == msg.sender,
            "Must own one of the agents"
        );
        require(
            keccak256(bytes(agent1)) != keccak256(bytes(agent2)),
            "Cannot interact with self"
        );
        
        interactions[agent1][agent2]++;
        interactions[agent2][agent1]++;
        
        emit InteractionRecorded(agent1, agent2, interactions[agent1][agent2]);
    }
    
    function rateAgent(
        string memory raterAgentId,
        string memory ratedAgentId,
        uint256 score
    ) 
        external 
        agentExists(raterAgentId)
        agentExists(ratedAgentId)
        onlyAgentOwner(raterAgentId)
    {
        require(score >= 1 && score <= 5, "Score must be 1-5");
        require(interactions[raterAgentId][ratedAgentId] > 0, "No prior interaction");
        require(
            keccak256(bytes(raterAgentId)) != keccak256(bytes(ratedAgentId)),
            "Cannot rate self"
        );
        
        uint256 oldRating = ratings[raterAgentId][ratedAgentId];
        ratings[raterAgentId][ratedAgentId] = score;
        
        // Update reputation if rating changed
        if (oldRating != score) {
            _updateReputation(ratedAgentId);
        }
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
            string[] memory groups,
            string memory metadata
        )
    {
        AgentInfo storage agent = agents[agentId];
        return (
            agent.owner,
            agent.endpoints,
            agent.services,
            agent.protocols,
            agent.reputation,
            agentGroups[agentId],
            agent.metadata
        );
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
    
    function getAgentGroups(string memory agentId) 
        external 
        view 
        returns (string[] memory)
    {
        return agentGroups[agentId];
    }
    
    function getRecommendedAgents(
        string memory agentId,
        string memory serviceType
    ) 
        external 
        view 
        agentExists(agentId)
        returns (string[] memory recommendations)
    {
        string[] memory serviceProviders = agentsByService[serviceType];
        uint256 recommendCount = 0;
        
        // Count suitable agents
        for (uint256 i = 0; i < serviceProviders.length; i++) {
            string memory providerId = serviceProviders[i];
            if (agents[providerId].isActive &&
                agents[providerId].reputation >= 70 &&
                keccak256(bytes(providerId)) != keccak256(bytes(agentId))) {
                recommendCount++;
            }
        }
        
        recommendations = new string[](recommendCount);
        uint256 index = 0;
        
        // Collect recommendations
        for (uint256 i = 0; i < serviceProviders.length; i++) {
            string memory providerId = serviceProviders[i];
            if (agents[providerId].isActive &&
                agents[providerId].reputation >= 70 &&
                keccak256(bytes(providerId)) != keccak256(bytes(agentId))) {
                recommendations[index] = providerId;
                index++;
            }
        }
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
    
    function getServiceRequest(uint256 requestId)
        external
        view
        returns (
            string memory requesterAgentId,
            string memory serviceType,
            string memory requirements,
            uint256 timestamp,
            uint256 expiryTime,
            bool isActive,
            string[] memory respondents
        )
    {
        ServiceRequest storage request = serviceRequests[requestId];
        return (
            request.requesterAgentId,
            request.serviceType,
            request.requirements,
            request.timestamp,
            request.expiryTime,
            request.isActive,
            request.respondents
        );
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
    function _updateReputation(string memory agentId) private {
        uint256 totalScore = 0;
        uint256 ratingCount = 0;
        
        // Calculate average rating from all raters
        for (uint256 i = 0; i < totalAgents; i++) {
            string memory raterId = agentIdByIndex[i];
            if (agents[raterId].isActive) {
                uint256 rating = ratings[raterId][agentId];
                if (rating > 0) {
                    totalScore += rating * 20; // Convert 1-5 to 20-100
                    ratingCount++;
                }
            }
        }
        
        if (ratingCount > 0) {
            uint256 newReputation = totalScore / ratingCount;
            // Apply bounds
            if (newReputation > MAX_REPUTATION) {
                newReputation = MAX_REPUTATION;
            } else if (newReputation < MIN_REPUTATION) {
                newReputation = MIN_REPUTATION;
            }
            
            agents[agentId].reputation = newReputation;
            emit ReputationUpdated(agentId, newReputation);
        }
    }
    
    function _removeFromArray(string[] storage array, string memory value) private {
        for (uint256 i = 0; i < array.length; i++) {
            if (keccak256(bytes(array[i])) == keccak256(bytes(value))) {
                array[i] = array[array.length - 1];
                array.pop();
                break;
            }
        }
    }
    
    // Batch operations for gas optimization
    function batchRegisterAgents(
        string[] memory agentIds,
        string[][] memory endpointsArray,
        uint256[][] memory weightsArray,
        string[][] memory servicesArray,
        string[][] memory protocolsArray,
        string[][] memory tagsArray,
        string[] memory metadataArray
    ) 
        external 
        payable 
        whenNotPaused
    {
        require(agentIds.length > 0, "No agents to register");
        require(
            agentIds.length == endpointsArray.length &&
            agentIds.length == weightsArray.length &&
            agentIds.length == servicesArray.length &&
            agentIds.length == protocolsArray.length &&
            agentIds.length == tagsArray.length &&
            agentIds.length == metadataArray.length,
            "Array length mismatch"
        );
        
        uint256 totalFee = registrationFee * agentIds.length;
        require(msg.value >= totalFee, "Insufficient fee for batch");
        
        for (uint256 i = 0; i < agentIds.length; i++) {
            // Validate each registration
            require(bytes(agentIds[i]).length > 0, "Empty agent ID");
            require(!agents[agentIds[i]].isActive, "Agent already exists");
            require(endpointsArray[i].length > 0, "No endpoints");
            require(endpointsArray[i].length == weightsArray[i].length, "Endpoints/weights mismatch");
            
            // Create agent
            AgentInfo storage newAgent = agents[agentIds[i]];
            newAgent.owner = msg.sender;
            newAgent.endpoints = endpointsArray[i];
            newAgent.weights = weightsArray[i];
            newAgent.registrationTime = block.timestamp;
            newAgent.lastUpdateTime = block.timestamp;
            newAgent.isActive = true;
            newAgent.metadata = metadataArray[i];
            newAgent.services = servicesArray[i];
            newAgent.protocols = protocolsArray[i];
            newAgent.reputation = BASE_REPUTATION;
            
            // Index services
            for (uint256 j = 0; j < servicesArray[i].length; j++) {
                newAgent.hasService[servicesArray[i][j]] = true;
                agentsByService[servicesArray[i][j]].push(agentIds[i]);
            }
            
            // Index protocols
            for (uint256 j = 0; j < protocolsArray[i].length; j++) {
                newAgent.supportsProtocol[protocolsArray[i][j]] = true;
                agentsByProtocol[protocolsArray[i][j]].push(agentIds[i]);
            }
            
            // Index tags
            agentTags[agentIds[i]] = tagsArray[i];
            for (uint256 j = 0; j < tagsArray[i].length; j++) {
                if (bytes(tagsArray[i][j]).length > 0) {
                    agentsByTag[tagsArray[i][j]].push(agentIds[i]);
                }
            }
            
            // Update indices
            agentIdByIndex[totalAgents] = agentIds[i];
            totalAgents++;
            agentsByOwner[msg.sender].push(agentIds[i]);
            
            emit AgentRegistered(agentIds[i], msg.sender, servicesArray[i], protocolsArray[i]);
        }
        
        // Refund excess
        if (msg.value > totalFee) {
            (bool success, ) = payable(msg.sender).call{value: msg.value - totalFee}("");
            require(success, "Refund failed");
        }
    }
    
    // Emergency functions
    function emergencyWithdraw() external onlyOwner {
        require(paused, "Must be paused for emergency withdrawal");
        (bool success, ) = payable(owner).call{value: address(this).balance}("");
        require(success, "Emergency withdrawal failed");
    }
    
    // Receive function to accept ETH
    receive() external payable {}
    
    // Fallback function
    fallback() external payable {}
}