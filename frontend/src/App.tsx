import { useState, useEffect, useRef } from 'react'
import { MessageSquare, Bot } from 'lucide-react'
import { Sidebar } from './components/Sidebar'
import { ChatMessage } from './components/ChatMessage'
import { ChatInput } from './components/ChatInput'
import { SettingsModal } from './components/SettingsModal'
import { WorkflowVisualization } from './components/WorkflowVisualization'
import { apiClient } from './api/client'
import { ChatSession, ChatMessage as ChatMessageType, WorkflowVisualizationGraph } from './types'
import { generateId } from './utils'
import { workflowWebSocket, WorkflowStatusUpdate } from './services/WorkflowWebSocket'

function App() {
  // State management
  const [sessions, setSessions] = useState<ChatSession[]>([])
  const [currentSession, setCurrentSession] = useState<ChatSession | null>(null)
  const [isSettingsOpen, setIsSettingsOpen] = useState(false)
  const [isLoading, setIsLoading] = useState(false)
  const [streamingMessage, setStreamingMessage] = useState<string>('')
  const [isStreaming, setIsStreaming] = useState(false)
  
  // 工作流可视化状态
  const [workflowGraph, setWorkflowGraph] = useState<WorkflowVisualizationGraph | null>(null)
  const [currentWorkflowId, setCurrentWorkflowId] = useState<string | null>(null)
  const [showWorkflow, setShowWorkflow] = useState(false)

  // 更新工作流节点状态
  const updateWorkflowNodeStatus = (update: WorkflowStatusUpdate) => {
    console.log('Updating workflow node status:', update)
    console.log('Current workflow graph before update:', workflowGraph)
    
    setWorkflowGraph(prevGraph => {
      if (!prevGraph) {
        console.log('No previous graph to update')
        return prevGraph
      }
      
      const updatedGraph = {
        ...prevGraph,
        nodes: prevGraph.nodes.map(node => {
          if (node.id === update.nodeId) {
            console.log(`Updating node ${node.id} from ${node.status} to ${update.status}`)
            return {
              ...node,
              status: update.status,
              error: update.error
            }
          }
          return node
        })
      }
      
      console.log('Updated workflow graph:', updatedGraph)
      return updatedGraph
    })
  }
  
  // Refs
  const messagesEndRef = useRef<HTMLDivElement>(null)

  // Load sessions on component mount
  useEffect(() => {
    loadSessions()
  }, [])

  // 初始化WebSocket连接
  useEffect(() => {
    const initWebSocket = async () => {
      try {
        await workflowWebSocket.connect()
        
        // 监听工作流状态更新
        workflowWebSocket.addEventListener('NODE_STATUS_UPDATE', (data: WorkflowStatusUpdate) => {
          console.log('Node status update received:', data)
          updateWorkflowNodeStatus(data)
        })
        
        // 监听工作流完成
        workflowWebSocket.addEventListener('WORKFLOW_COMPLETE', (data: any) => {
          console.log('Workflow complete received:', data)
          // 保持工作流面板显示，不自动隐藏
          // setShowWorkflow(false)
        })
        
        console.log('WebSocket event listeners added')
      } catch (error) {
        console.error('Failed to connect WebSocket:', error)
      }
    }

    initWebSocket()

    return () => {
      workflowWebSocket.disconnect()
    }
  }, [])


  // Auto-scroll to bottom when messages change
  useEffect(() => {
    scrollToBottom()
  }, [currentSession?.messages, streamingMessage])

  const scrollToBottom = () => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' })
  }

  const loadSessions = async () => {
    try {
      const loadedSessions = await apiClient.getSessions()
      setSessions(loadedSessions || [])
    } catch (error) {
      console.error('Failed to load sessions:', error)
      setSessions([]) // Ensure sessions is always an array
    }
  }

  const handleNewChat = async () => {
    try {
      const response = await apiClient.createSession({
        title: 'New Conversation'
      })
      
      const newSession = response.session
      setSessions(prev => [newSession, ...prev])
      setCurrentSession(newSession)
    } catch (error) {
      console.error('Failed to create new session:', error)
    }
  }

  const handleSessionSelect = async (sessionId: string) => {
    try {
      const session = await apiClient.getSession(sessionId)
      setCurrentSession(session)
    } catch (error) {
      console.error('Failed to load session:', error)
    }
  }

  const handleDeleteSession = (sessionId: string) => {
    setSessions(prev => prev.filter(session => session.id !== sessionId))
    
    if (currentSession?.id === sessionId) {
      setCurrentSession(null)
    }
  }

  const handleSendMessage = async (content: string) => {
    if (!currentSession || isLoading || isStreaming) return

    // Create user message
    const userMessage: ChatMessageType = {
      id: generateId(),
      role: 'user',
      content,
      timestamp: new Date()
    }

    // Update current session with user message
    const updatedSession = {
      ...currentSession,
      messages: [...currentSession.messages, userMessage],
      updatedAt: new Date()
    }
    setCurrentSession(updatedSession)

    // Update sessions list
    setSessions(prev => 
      prev.map(session => 
        session.id === currentSession.id ? updatedSession : session
      )
    )

    setIsLoading(true)
    setIsStreaming(true)
    setStreamingMessage('')

    // 检查是否需要启动工作流可视化
    const shouldInitiateWorkflow = checkIfWorkflowNeeded(content)
    console.log('Should initiate workflow:', shouldInitiateWorkflow)
    
    if (shouldInitiateWorkflow) {
      try {
        console.log('Attempting to initiate workflow...')
        // 初始化工作流
        const workflowResponse = await apiClient.initiateWorkflow({
          message: content,
          session_id: currentSession.id
        })
        
        console.log('Workflow response received:', workflowResponse)
        
        setCurrentWorkflowId(workflowResponse.workflowId)
        setWorkflowGraph(workflowResponse.graph)
        setShowWorkflow(true)
        
        console.log('Workflow state updated:', {
          workflowId: workflowResponse.workflowId,
          graph: workflowResponse.graph,
          showWorkflow: true
        })
      } catch (error) {
        console.error('Failed to initiate workflow:', error)
        // 继续正常流程，不显示工作流
      }
    }

    // Use a ref to track the accumulated streaming content
    let accumulatedContent = ''

    // Send message and handle streaming response
    await apiClient.sendMessage(
      {
        session_id: currentSession.id,
        message: content
      },
      // onChunk: Accumulate streaming content
      (chunk: string) => {
        accumulatedContent += chunk
        setStreamingMessage(accumulatedContent)
      },
      // onComplete: Finalize the assistant message
      (messageId: string) => {
        console.log('Message completed with ID:', messageId)
        console.log('Accumulated content:', accumulatedContent)
        
        const assistantMessage: ChatMessageType = {
          id: messageId,
          role: 'assistant',
          content: accumulatedContent, // Use the accumulated content
          timestamp: new Date()
        }

        // Update current session with assistant message
        const finalSession = {
          ...updatedSession,
          messages: [...updatedSession.messages, assistantMessage],
          updatedAt: new Date()
        }
        
        console.log('Final session messages:', finalSession.messages)
        setCurrentSession(finalSession)

        // Update sessions list
        setSessions(prev => 
          prev.map(session => 
            session.id === currentSession.id ? finalSession : session
          )
        )

        setIsStreaming(false)
        setStreamingMessage('')
        setIsLoading(false)
        
        // Simulate clicking current session to refresh the message display and ensure proper Markdown rendering
        setTimeout(() => {
          if (currentSession?.id) {
            handleSessionSelect(currentSession.id)
          }
        }, 500) // 500ms delay to ensure state updates are complete
      },
      // onError: Handle errors
      (error: Error) => {
        console.error('Failed to send message:', error)
        setIsStreaming(false)
        setStreamingMessage('')
        setIsLoading(false)
      }
    )
  }

  // 检查是否需要启动工作流可视化
  const checkIfWorkflowNeeded = (message: string): boolean => {
    const workflowKeywords = [
      '查询', '获取', '调用', '执行', '分析', '比较', '统计',
      'query', 'get', 'call', 'execute', 'analyze', 'compare', 'statistics',
      'rpc', 'api', 'block', 'transaction', 'balance', 'state', 'stateroot'
    ]
    
    const shouldTrigger = workflowKeywords.some(keyword => 
      message.toLowerCase().includes(keyword.toLowerCase())
    )
    
    console.log('Workflow trigger check:', {
      message,
      shouldTrigger,
      keywords: workflowKeywords
    })
    
    return shouldTrigger
  }

  // 测试函数：强制显示工作流
  const testShowWorkflow = () => {
    const testGraph = {
      nodes: [
        { id: 'node-1', label: '意图分析', type: 'IntentAnalysis', status: 'Pending' as const },
        { id: 'node-2', label: '查询最新区块数量', type: 'APICall', status: 'Pending' as const },
        { id: 'node-3', label: '提取区块号参数', type: 'LLMParameterExtraction', status: 'Pending' as const },
        { id: 'node-4', label: '查询 StateRoot 信息', type: 'APICall', status: 'Pending' as const },
        { id: 'node-5', label: '格式化最终结果', type: 'LLMBeautify', status: 'Pending' as const }
      ],
      edges: [
        { source: 'node-1', target: 'node-2' },
        { source: 'node-2', target: 'node-3' },
        { source: 'node-3', target: 'node-4' },
        { source: 'node-4', target: 'node-5' }
      ]
    }
    
    setCurrentWorkflowId('test-wf-123')
    setWorkflowGraph(testGraph)
    setShowWorkflow(true)
    console.log('Test workflow displayed')
  }

  return (
    <div className="flex h-screen bg-gray-50">
      {/* Sidebar */}
      <Sidebar
        sessions={sessions}
        currentSessionId={currentSession?.id || null}
        onSessionSelect={handleSessionSelect}
        onNewChat={handleNewChat}
        onOpenSettings={() => setIsSettingsOpen(true)}
        onDeleteSession={handleDeleteSession}
        onSessionsUpdate={loadSessions}
      />

      {/* Main Chat Area */}
      <div className={`flex-1 flex flex-col ${showWorkflow ? 'mr-[500px]' : ''}`}>
        {currentSession ? (
          <>
            {/* Messages */}
            <div className="flex-1 overflow-y-auto">
              {currentSession.messages.length === 0 && !isStreaming ? (
                // Welcome screen
                <div className="flex items-center justify-center h-full">
                  <div className="text-center max-w-md px-4">
                    <div className="w-16 h-16 bg-blockchain-gradient rounded-full flex items-center justify-center mx-auto mb-6">
                      <Bot className="w-8 h-8 text-white" />
                    </div>
                    <h2 className="text-2xl font-bold text-gray-900 mb-4">
                      Welcome to QNG Intelligent Agent
                    </h2>
                    <p className="text-gray-600 mb-6">
                      I can help you with blockchain queries, Web3 operations, smart contract interactions, and cryptocurrency analysis. 
                      Ask me anything!
                    </p>
                    <div className="text-sm text-gray-500 space-y-2">
                      <p>• Query stateroot information from QNG nodes</p>
                      <p>• Execute Web3 workflows and smart contracts</p>
                      <p>• Analyze blockchain data with AI</p>
                      <p>• Manage cryptocurrency transactions</p>
                    </div>
                  </div>
                </div>
              ) : (
                // Messages list
                <div className="space-y-0">
                  {currentSession.messages.map((message) => (
                    <ChatMessage 
                      key={message.id} 
                      message={message} 
                      isStreaming={false}
                    />
                  ))}
                  
                  {/* Streaming message */}
                  {isStreaming && streamingMessage && (
                    <ChatMessage
                      message={{
                        id: 'streaming',
                        role: 'assistant',
                        content: streamingMessage,
                        timestamp: new Date()
                      }}
                      isStreaming={true}
                    />
                  )}
                  
                  <div ref={messagesEndRef} />
                </div>
              )}
            </div>

            {/* Chat Input */}
            <div className="p-4 border-t border-gray-200">
              <div className="flex items-center space-x-2 mb-2">
                <button
                  onClick={testShowWorkflow}
                  className="px-3 py-1 text-sm bg-blue-100 text-blue-700 rounded hover:bg-blue-200 transition-colors"
                >
                  测试显示工作流
                </button>
                <span className="text-xs text-gray-500">点击此按钮测试工作流面板显示</span>
              </div>
              <ChatInput
                onSendMessage={handleSendMessage}
                disabled={isLoading}
                isLoading={isStreaming}
              />
            </div>
          </>
        ) : (
          // No session selected
          <div className="flex items-center justify-center h-full">
            <div className="text-center max-w-md px-4">
              <MessageSquare className="w-16 h-16 text-gray-300 mx-auto mb-6" />
              <h2 className="text-xl font-semibold text-gray-700 mb-2">
                Select a conversation
              </h2>
              <p className="text-gray-500 mb-6">
                Choose an existing conversation from the sidebar or start a new chat to begin.
              </p>
              <button
                onClick={handleNewChat}
                className="bg-blockchain-primary text-white px-6 py-3 rounded-lg hover:bg-blockchain-primary/90 transition-colors"
              >
                Start New Chat
              </button>
            </div>
          </div>
        )}
      </div>

      {/* Settings Modal */}
      <SettingsModal
        isOpen={isSettingsOpen}
        onClose={() => setIsSettingsOpen(false)}
      />

      {/* Workflow Visualization */}
      <WorkflowVisualization
        workflowId={currentWorkflowId || undefined}
        graph={workflowGraph || undefined}
        isVisible={showWorkflow}
        onNodeClick={(node) => {
          console.log('Node clicked:', node)
          if (node.error) {
            alert(`节点错误: ${node.error}`)
          }
        }}
        onClose={() => {
          setShowWorkflow(false)
          setWorkflowGraph(null)
          setCurrentWorkflowId(null)
        }}
      />
    </div>
  )
}

export default App