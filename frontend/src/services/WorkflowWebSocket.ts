// WebSocket连接管理器
export class WorkflowWebSocket {
  private ws: WebSocket | null = null
  private reconnectAttempts = 0
  private maxReconnectAttempts = 5
  private reconnectDelay = 1000
  private isConnecting = false
  private listeners: Map<string, Set<Function>> = new Map()

  constructor(
    private baseUrl: string = '',
    private onConnect?: () => void,
    private onDisconnect?: () => void,
    private onError?: (error: Event) => void
  ) {}

  // 连接WebSocket
  connect(workflowId?: string): Promise<void> {
    return new Promise((resolve, reject) => {
      if (this.isConnecting || (this.ws && this.ws.readyState === WebSocket.OPEN)) {
        resolve()
        return
      }

      this.isConnecting = true
      
      try {
        const wsUrl = `ws://localhost:8081/api/ws/workflow/status${workflowId ? `?workflowId=${workflowId}` : ''}`
        this.ws = new WebSocket(wsUrl)

        this.ws.onopen = () => {
          console.log('WebSocket connected')
          this.isConnecting = false
          this.reconnectAttempts = 0
          this.onConnect?.()
          resolve()
        }

        this.ws.onmessage = (event) => {
          try {
            const data = JSON.parse(event.data)
            this.handleMessage(data)
          } catch (error) {
            console.error('Failed to parse WebSocket message:', error)
          }
        }

        this.ws.onclose = (event) => {
          console.log('WebSocket disconnected:', event.code, event.reason)
          this.isConnecting = false
          this.onDisconnect?.()
          
          // 如果不是正常关闭，尝试重连
          if (event.code !== 1000 && this.reconnectAttempts < this.maxReconnectAttempts) {
            this.scheduleReconnect()
          }
        }

        this.ws.onerror = (error) => {
          console.error('WebSocket error:', error)
          this.isConnecting = false
          this.onError?.(error)
          reject(error)
        }
      } catch (error) {
        this.isConnecting = false
        reject(error)
      }
    })
  }

  // 断开连接
  disconnect(): void {
    if (this.ws) {
      this.ws.close(1000, 'Manual disconnect')
      this.ws = null
    }
  }

  // 发送消息
  send(message: any): void {
    if (this.ws && this.ws.readyState === WebSocket.OPEN) {
      this.ws.send(JSON.stringify(message))
    } else {
      console.warn('WebSocket is not connected')
    }
  }

  // 添加事件监听器
  addEventListener(eventType: string, callback: Function): void {
    if (!this.listeners.has(eventType)) {
      this.listeners.set(eventType, new Set())
    }
    this.listeners.get(eventType)!.add(callback)
  }

  // 移除事件监听器
  removeEventListener(eventType: string, callback: Function): void {
    const listeners = this.listeners.get(eventType)
    if (listeners) {
      listeners.delete(callback)
      if (listeners.size === 0) {
        this.listeners.delete(eventType)
      }
    }
  }

  // 处理接收到的消息
  private handleMessage(data: any): void {
    console.log('WebSocket message received:', data)
    const eventType = data.eventType
    const listeners = this.listeners.get(eventType)
    
    console.log('Event type:', eventType, 'Listeners:', listeners?.size || 0)
    
    if (listeners) {
      listeners.forEach(callback => {
        try {
          console.log('Calling listener for event:', eventType)
          callback(data.payload || data)
        } catch (error) {
          console.error('Error in WebSocket event listener:', error)
        }
      })
    } else {
      console.warn('No listeners found for event type:', eventType)
    }
  }

  // 安排重连
  private scheduleReconnect(): void {
    if (this.reconnectAttempts >= this.maxReconnectAttempts) {
      console.error('Max reconnection attempts reached')
      return
    }

    this.reconnectAttempts++
    const delay = this.reconnectDelay * Math.pow(2, this.reconnectAttempts - 1)
    
    console.log(`Attempting to reconnect in ${delay}ms (attempt ${this.reconnectAttempts}/${this.maxReconnectAttempts})`)
    
    setTimeout(() => {
      this.connect().catch(error => {
        console.error('Reconnection failed:', error)
      })
    }, delay)
  }

  // 获取连接状态
  get isConnected(): boolean {
    return this.ws?.readyState === WebSocket.OPEN
  }

  // 获取重连状态
  get isReconnecting(): boolean {
    return this.isConnecting
  }
}

// 工作流状态更新事件类型
export interface WorkflowStatusUpdate {
  workflowId: string
  nodeId: string
  status: 'Pending' | 'Executing' | 'Success' | 'Failure'
  data?: any
  error?: string
}

// 工作流完成事件类型
export interface WorkflowComplete {
  workflowId: string
  success: boolean
  summary?: string
  totalTime?: number
}

// 导出单例实例
export const workflowWebSocket = new WorkflowWebSocket(
  '', // 将在使用时设置baseUrl
  () => console.log('Workflow WebSocket connected'),
  () => console.log('Workflow WebSocket disconnected'),
  (error) => console.error('Workflow WebSocket error:', error)
)
