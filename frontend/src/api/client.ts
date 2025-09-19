import { 
  ChatSession, 
  SendMessageRequest,
  CreateSessionRequest,
  CreateSessionResponse,
  AppSettings
} from '../types'

const API_BASE_URL = '/api'

// API client class for handling all backend communication
class APIClient {
  // Session management
  async createSession(request: CreateSessionRequest): Promise<CreateSessionResponse> {
    const response = await fetch(`${API_BASE_URL}/sessions`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify(request),
    })
    
    if (!response.ok) {
      throw new Error('Failed to create session')
    }
    
    return response.json()
  }

  async getSessions(): Promise<ChatSession[]> {
    const response = await fetch(`${API_BASE_URL}/sessions`)
    
    if (!response.ok) {
      throw new Error('Failed to fetch sessions')
    }
    
    return response.json()
  }

  async getSession(sessionId: string): Promise<ChatSession> {
    const response = await fetch(`${API_BASE_URL}/sessions/${sessionId}`)
    
    if (!response.ok) {
      throw new Error('Failed to fetch session')
    }
    
    return response.json()
  }

  async deleteSession(sessionId: string): Promise<void> {
    const response = await fetch(`${API_BASE_URL}/sessions/${sessionId}`, {
      method: 'DELETE',
    })
    
    if (!response.ok) {
      throw new Error('Failed to delete session')
    }
  }

  // Message streaming with Server-Sent Events
  async sendMessage(
    request: SendMessageRequest,
    onChunk: (chunk: string) => void,
    onComplete: (messageId: string) => void,
    onError: (error: Error) => void
  ): Promise<void> {
    try {
      const response = await fetch(`${API_BASE_URL}/chat/stream`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify(request),
      })

      if (!response.ok) {
        throw new Error('Failed to send message')
      }

      const reader = response.body?.getReader()
      if (!reader) {
        throw new Error('No response body')
      }

      const decoder = new TextDecoder()
      let messageId = ''

      while (true) {
        const { done, value } = await reader.read()
        
        if (done) break

        const chunk = decoder.decode(value)
        const lines = chunk.split('\n')

        for (const line of lines) {
          // Handle Server-Sent Events format: "data: content"
          if (line.startsWith('data: ')) {
            const data = line.slice(6) // Remove "data: " (including space)
            
            if (data === '[DONE]') {
              onComplete(messageId)
              return
            }

            try {
              const parsed = JSON.parse(data)
              if (parsed.message_id) {
                messageId = parsed.message_id
              }
              if (parsed.content) {
                onChunk(parsed.content)
              }
            } catch (e) {
              // Handle non-JSON chunks - ignore empty lines and events
              if (data && data !== '') {
                console.log('Non-JSON SSE data:', data)
              }
            }
          }
        }
      }
    } catch (error) {
      onError(error instanceof Error ? error : new Error('Unknown error'))
    }
  }

  // Settings management
  async getSettings(): Promise<AppSettings> {
    const response = await fetch(`${API_BASE_URL}/settings`)
    
    if (!response.ok) {
      throw new Error('Failed to fetch settings')
    }
    
    return response.json()
  }

  async updateSettings(settings: AppSettings): Promise<void> {
    const response = await fetch(`${API_BASE_URL}/settings`, {
      method: 'PUT',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify(settings),
    })
    
    if (!response.ok) {
      throw new Error('Failed to update settings')
    }
  }
}

export const apiClient = new APIClient()