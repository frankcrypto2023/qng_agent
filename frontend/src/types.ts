// Chat message types
export interface ChatMessage {
  id: string;
  role: 'user' | 'assistant';
  content: string;
  timestamp: Date;
}

// Chat session types
export interface ChatSession {
  id: string;
  title: string;
  messages: ChatMessage[];
  createdAt: Date;
  updatedAt: Date;
}

// Settings types
export interface MCPServerConfig {
  name: string;
  url: string;
  enabled: boolean;
}

export interface LLMProviderConfig {
  name: string;
  url: string;
  token: string;
  modelName: string;
}

export interface AppSettings {
  mcpServers: MCPServerConfig[];
  llmProvider: LLMProviderConfig;
}

// API types
export interface SendMessageRequest {
  session_id: string;
  message: string;
}

export interface SendMessageResponse {
  messageId: string;
  content: string;
}

export interface CreateSessionRequest {
  title?: string;
}

export interface CreateSessionResponse {
  session: ChatSession;
}

// Workflow types
export interface WorkflowNode {
  id: string;
  type: string;
  name: string;
  config?: Record<string, any>;
  web3_config?: Record<string, any>;
}

export interface WorkflowEdge {
  id: string;
  source: string;
  target: string;
  condition?: string;
}

export interface WorkflowConfig {
  name: string;
  description: string;
  nodes: WorkflowNode[];
  edges: WorkflowEdge[];
}