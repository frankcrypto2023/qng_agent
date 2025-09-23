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

export type LLMProviderType = 'openai' | 'openrouter' | 'groq' | 'anthropic' | 'custom';

export interface LLMProviderConfig {
  type: LLMProviderType;
  name: string;
  url: string;
  token: string;
  model_name: string;
  timeout?: number;     // Request timeout in seconds
  app_name?: string;    // For OpenRouter X-Title header
  app_url?: string;     // For OpenRouter HTTP-Referer header
}

export interface AppSettings {
  mcp_servers: MCPServerConfig[];
  llm_provider: LLMProviderConfig;
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