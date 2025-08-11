// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

import "./RegisterCore.sol";

/**
 * @title IRegisterExtended
 * @dev Interface for Register Extended contract
 */
interface IRegisterExtended {
    // Events
    event ServiceRequested(
        uint256 indexed requestId,
        string indexed serviceType,
        string requesterAgentId
    );
    
    event ServiceResponseSubmitted(
        uint256 indexed requestId,
        string indexed respondentAgentId
    );
}

/**
 * @title RegisterExtended
 * @dev Simplified extended implementation with service requests only
 */
contract RegisterExtended is IRegisterExtended {
    
    // Reference to core contract
    RegisterCore public immutable coreContract;
    
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
    
    // Service request system
    mapping(uint256 => ServiceRequest) public serviceRequests;
    uint256 public serviceRequestCounter;
    mapping(string => uint256[]) private requestsByService;
    
    // Constructor
    constructor(address payable _coreContract) {
        require(_coreContract != address(0), "Invalid core contract address");
        coreContract = RegisterCore(_coreContract);
    }
    
    // Service request system
    function requestService(
        string memory requesterAgentId,
        string memory serviceType,
        string memory requirements,
        uint256 duration
    ) 
        external 
        returns (uint256 requestId)
    {
        require(coreContract.isAgentActive(requesterAgentId), "Agent does not exist");
        (address agentOwner,,,,,) = coreContract.getAgent(requesterAgentId);
        require(agentOwner == msg.sender, "Only agent owner can request service");
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
        
        emit ServiceRequested(requestId, serviceType, requesterAgentId);
    }
    
    function respondToServiceRequest(
        uint256 requestId,
        string memory respondentAgentId
    ) 
        external 
    {
        require(coreContract.isAgentActive(respondentAgentId), "Agent does not exist");
        (address agentOwner,,,,,) = coreContract.getAgent(respondentAgentId);
        require(agentOwner == msg.sender, "Only agent owner can respond");
        
        ServiceRequest storage request = serviceRequests[requestId];
        require(request.isActive, "Request not active");
        require(block.timestamp < request.expiryTime, "Request expired");
        require(!request.hasResponded[respondentAgentId], "Already responded");
        
        // Check if agent provides the requested service
        (address agentOwner2, string[] memory endpoints, string[] memory services, string[] memory protocols, uint256 reputation, string memory metadata, string memory contractABI) = coreContract.getAgentDetails(respondentAgentId);
        bool providesService = false;
        for (uint256 i = 0; i < services.length; i++) {
            if (keccak256(bytes(services[i])) == keccak256(bytes(request.serviceType))) {
                providesService = true;
                break;
            }
        }
        require(providesService, "Agent doesn't provide this service");
        
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
    
    // Query functions
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
}
