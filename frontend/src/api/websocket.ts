import { WorkflowStatusUpdate, WorkflowComplete } from '../types'

export type WebSocketEventHandler = (event: WorkflowStatusUpdate | WorkflowComplete) => void

export class WorkflowWebSocketClient {
  private ws: WebSocket | null = null
  private reconnectAttempts = 0
  private maxReconnectAttempts = 5
  private reconnectDelay = 1000
  private eventHandlers: WebSocketEventHandler[] = []
  private isConnected = false
  private currentWorkflowId: string | null = null

  constructor(private baseUrl: string = '') {
    // Use relative URL for WebSocket connection
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    const wsUrl = `${protocol}//${window.location.host}/ws/workflow/status`
    this.connect(wsUrl)
  }

  private connect(url: string) {
    try {
      this.ws = new WebSocket(url)
      
      this.ws.onopen = () => {
        console.log('WebSocket connected')
        this.isConnected = true
        this.reconnectAttempts = 0
        
        // Resubscribe to current workflow if any
        if (this.currentWorkflowId) {
          this.subscribeToWorkflow(this.currentWorkflowId)
        }
      }

      this.ws.onmessage = (event) => {
        try {
          const data = JSON.parse(event.data)
          this.handleMessage(data)
        } catch (error) {
          console.error('Failed to parse WebSocket message:', error)
        }
      }

      this.ws.onclose = () => {
        console.log('WebSocket disconnected')
        this.isConnected = false
        this.attemptReconnect()
      }

      this.ws.onerror = (error) => {
        console.error('WebSocket error:', error)
      }
    } catch (error) {
      console.error('Failed to create WebSocket connection:', error)
      this.attemptReconnect()
    }
  }

  private attemptReconnect() {
    if (this.reconnectAttempts < this.maxReconnectAttempts) {
      this.reconnectAttempts++
      console.log(`Attempting to reconnect... (${this.reconnectAttempts}/${this.maxReconnectAttempts})`)
      
      setTimeout(() => {
        const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
        const wsUrl = `${protocol}//${window.location.host}/ws/workflow/status`
        this.connect(wsUrl)
      }, this.reconnectDelay * this.reconnectAttempts)
    } else {
      console.error('Max reconnection attempts reached')
    }
  }

  private handleMessage(data: any) {
    // Handle different message types
    if (data.eventType === 'NODE_STATUS_UPDATE' || data.eventType === 'WORKFLOW_COMPLETE') {
      this.notifyHandlers(data)
    }
  }

  private notifyHandlers(event: WorkflowStatusUpdate | WorkflowComplete) {
    this.eventHandlers.forEach(handler => {
      try {
        handler(event)
      } catch (error) {
        console.error('Error in WebSocket event handler:', error)
      }
    })
  }

  public subscribeToWorkflow(workflowId: string) {
    this.currentWorkflowId = workflowId
    
    if (this.isConnected && this.ws) {
      const message = {
        type: 'subscribe_workflow',
        workflow_id: workflowId
      }
      this.ws.send(JSON.stringify(message))
      console.log(`Subscribed to workflow: ${workflowId}`)
    }
  }

  public unsubscribeFromWorkflow() {
    this.currentWorkflowId = null
    
    if (this.isConnected && this.ws) {
      const message = {
        type: 'unsubscribe_workflow'
      }
      this.ws.send(JSON.stringify(message))
      console.log('Unsubscribed from workflow')
    }
  }

  public addEventHandler(handler: WebSocketEventHandler) {
    this.eventHandlers.push(handler)
  }

  public removeEventHandler(handler: WebSocketEventHandler) {
    const index = this.eventHandlers.indexOf(handler)
    if (index > -1) {
      this.eventHandlers.splice(index, 1)
    }
  }

  public disconnect() {
    if (this.ws) {
      this.ws.close()
      this.ws = null
    }
    this.isConnected = false
    this.currentWorkflowId = null
    this.eventHandlers = []
  }

  public getConnectionStatus(): boolean {
    return this.isConnected
  }

  public getCurrentWorkflowId(): string | null {
    return this.currentWorkflowId
  }
}

// Singleton instance
export const workflowWebSocket = new WorkflowWebSocketClient()
