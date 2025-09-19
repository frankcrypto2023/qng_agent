# Project: QNG Intelligent Agent - Full-Stack Implementation

## I. High-Level Objective

Build a full-stack web application, the "QNG Intelligent Agent." It will feature a conversational AI interface powered by a dynamic, graph-based workflow engine on the backend. The agent is designed to understand user intent, interact with Web3 systems, execute complex workflows, and provide natural language responses.

## II. Frontend Requirements (React)

### A. UI/UX Design

  - **Application Title:** QNG Intelligent Agent
  - **Aesthetics:** Clean, modern, and incorporate blockchain-themed visual elements.
  - **Layout:** A ChatGPT-style interface.
      - **Left Sidebar:**
          - "New Chat" button at the top.
          - A scrollable list of historical chat sessions.
          - A "Settings" button at the bottom.
      - **Main Content Area:**
          - Displays the current conversation.
          - User input text box at the bottom.

### B. Core Functionality

  - **Session Management:** Users can create new chats or select an existing one from the history to load the conversation.
  - **Real-time Interaction:** AI responses must be streamed character by character to the UI.
  - **Settings Panel:** A modal or page to configure:
      - **MCP Server:** A user-defined server endpoint that adheres to the standard MCP SSE protocol.
      - **LLM Provider:**
          - `url`: The API endpoint of the language model.
          - `token`: The authentication token.
          - `modelName`: The specific model to be used (e.g., "gpt-4").

### C. Technical Specifications

  - **Framework:** React.
  - **Code Style:** All code and comments must be in **English**. Comments should be comprehensive to facilitate future development.
  - **API Communication:**
      - Create a dedicated and clearly defined API service module.
      - **Crucially, all external API calls (especially to the LLM Provider) must be proxied through the Golang backend.** The frontend should not call any third-party services directly.

## III. Backend Requirements (Golang)

### A. Core Technology

  - **Framework:** Golang.
  - **Code Style:** All code and comments must be in **English**.
  - **API Documentation:** Provide a clear and comprehensive API specification for the frontend.

### B. Session and Context Management

  - Manage user conversation state, including context and history.
  - Implement a channel-based session management system (similar to the 'harmony' framework from gpt-oss) to isolate different conversations and correctly assemble prompts for the LLM.

### C. The LLM Graph Workflow Engine

The core of the backend is a workflow engine built on the `github.com/Qitmeer/qng/blob/dev/2.1/graph/graph.go` library.

**Processing Logic for Every User Message:**

1.  **Instantiate Graph:** Create a new `LLM Graph` instance.
2.  **Intent Analysis Workflow:** Execute a primary workflow on this graph to analyze the user's intent. This workflow must:
      - Analyze the user's current prompt.
      - Consider the historical context of the session.
      - **Decision Point 1:** Determine if the intent maps to a registered **MCP Tool**.
      - **Decision Point 2:** Determine if the intent maps to a predefined **Web3 Workflow** loaded from a contract or configuration.
3.  **Execution Path:** Based on the analysis, proceed as follows:
      - **If an MCP Tool is matched:**
        a. Extract necessary parameters from the user's prompt.
        b. Execute the tool.
        c. Pass the tool's output to the configured LLM to generate a natural language response.
      - **If a Web3 Workflow is matched:**
        a. Load the corresponding workflow configuration JSON.
        b. Execute the graph as defined by its `nodes` and `edges`.
        c. This workflow may involve multi-step interactions requiring user confirmation (e.g., "Connect Wallet," "Sign Transaction"), which must be communicated back to the frontend.
        d. Provide a default "AI analyzing blockchain data" workflow using the structure below for initial implementation.
      - **If NO predefined match is found:**
        a. Attempt to dynamically generate a new, ad-hoc sub-graph to fulfill the user's unique request.
      - **Final Step:** Stream the final result from any path back to the user.

## IV. Web3 Workflow Configuration (JSON Schema)

Workflows are defined using the following JSON structure. The backend must be able to parse and execute these graphs.

```json
{
    "name": "QNG-AI-Web3-Workflow",
    "description": "An QNG-AI-powered Web3 workflow for processing and analyzing blockchain data",
    "nodes": [
        { "id": "input_node", "type": "data_processor", "name": "Input Processor", "config": { "validation": true } },
        { "id": "ai_analyzer", "type": "ai_model", "name": "AI Data Analyzer", "config": { "model": "gpt-4", "temperature": 0.7, "max_tokens": 1000 } },
        { "id": "web3_reader", "type": "web3_contract", "name": "Smart Contract Reader", "web3_config": { "contract_address": "0x...", "chain_id": 1, "method": "getData", "parameters": ["param1"] } },
        { "id": "decision_node", "type": "conditional", "name": "Decision Maker", "config": { "condition": "score > 0.8", "true_branch": "web3_writer", "false_branch": "output_node" } },
        { "id": "web3_writer", "type": "web3_contract", "name": "Smart Contract Writer", "web3_config": { "contract_address": "0x...", "chain_id": 1, "method": "setData", "parameters": ["result"] } },
        { "id": "output_node", "type": "data_processor", "name": "Output Formatter", "config": { "format": "json" } }
    ],
    "edges": [
        { "id": "edge1", "source": "input_node", "target": "web3_reader" },
        { "id": "edge2", "source": "web3_reader", "target": "ai_analyzer" },
        { "id": "edge3", "source": "ai_analyzer", "target": "decision_node" },
        { "id": "edge4", "source": "decision_node", "target": "web3_writer" },
        { "id": "edge5", "source": "decision_node", "target": "output_node" },
        { "id": "edge6", "source": "web3_writer", "target": "output_node" }
    ]
}
```

## V. Illustrative Scenarios

### Scenario 1: Single Tool Execution

  - **User Input:** "What is the stateroot for order=10000 on node [http://127.0.0.1:8545](http://127.0.0.1:8545)?"
  - **Expected Action:**
    1.  Graph identifies the intent as a `stateroot` query (an MCP Tool).
    2.  It extracts `http://127.0.0.1:8545` and `order=10000` as parameters.
    3.  It calls the `stateroot` tool with these parameters.
    4.  The tool's raw result is passed to the LLM to generate a human-readable sentence.
    5.  The final sentence is streamed to the user.

### Scenario 2: Multi-Tool Execution & Comparison

  - **User Input:** "Compare the stateroot for order=10000 on node [http://127.0.0.1:8545](http://127.0.0.1:8545) and [http://127.0.0.2:8545](http://127.0.0.2:8545). Are they the same?"
  - **Expected Action:**
    1.  Graph identifies two `stateroot` tool calls are needed.
    2.  It creates two parallel execution branches in the graph.
    3.  Branch 1: Calls `stateroot` tool for the first node.
    4.  Branch 2: Calls `stateroot` tool for the second node.
    5.  A subsequent node receives the results from both branches.
    6.  This combined result is passed to the LLM with the instruction to compare them and highlight differences.
    7.  The LLM's comparison is streamed to the user.

### Scenario 3: Predefined Web3 Workflow (Token Swap)

  - **User Input:** "I want to swap 1 MEER for USDT using my Metamask."
  - **Expected Action:**
    1.  Graph identifies the intent matches the "Token Swap" Web3 workflow.
    2.  It loads the workflow's JSON configuration.
    3.  **Step 1:** The first node requires a wallet connection. The backend sends a request to the frontend to trigger a Metamask connection prompt.
    4.  **Step 2:** After the user connects and the wallet address is received, a `checkBalance` node is executed.
    5.  **Step 3 (Conditional):** If the balance is insufficient, the workflow terminates and informs the user. If sufficient, it proceeds.
    6.  **Step 4:** An `assembleTransaction` node creates the swap transaction data. The backend sends this to the frontend, requesting a user signature.
    7.  **Step 5:** After the user signs, a `sendTransaction` node executes.
    8.  **Step 6:** The final node formats the transaction hash and result, which is then passed to the LLM to generate a confirmation message for the user.