export interface Message {
  id: string;
  role: 'user' | 'assistant' | 'system';
  content: string;
  timestamp: Date;
}

export interface ChatRequest {
  message: string;
  mode: 'basic' | 'tools' | 'stream';
}

export interface ChatResponse {
  response: string;
  error?: string;
  usage?: {
    prompt_tokens: number;
    completion_tokens: number;
    total_tokens: number;
  };
}

export interface Conversation {
  id: string;
  title: string;
  messages: Message[];
  mode: 'basic' | 'tools' | 'stream';
  createdAt: Date;
  updatedAt: Date;
}
