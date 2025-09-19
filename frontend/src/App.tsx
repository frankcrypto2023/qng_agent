import { useState, useEffect, useRef } from 'react'
import { MessageSquare, Bot } from 'lucide-react'
import { Sidebar } from './components/Sidebar'
import { ChatMessage } from './components/ChatMessage'
import { ChatInput } from './components/ChatInput'
import { SettingsModal } from './components/SettingsModal'
import { apiClient } from './api/client'
import { ChatSession, ChatMessage as ChatMessageType } from './types'
import { generateId } from './utils'

function App() {
  // State management
  const [sessions, setSessions] = useState<ChatSession[]>([])
  const [currentSession, setCurrentSession] = useState<ChatSession | null>(null)
  const [isSettingsOpen, setIsSettingsOpen] = useState(false)
  const [isLoading, setIsLoading] = useState(false)
  const [streamingMessage, setStreamingMessage] = useState<string>('')
  const [isStreaming, setIsStreaming] = useState(false)
  
  // Refs
  const messagesEndRef = useRef<HTMLDivElement>(null)

  // Load sessions on component mount
  useEffect(() => {
    loadSessions()
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
      setSessions(loadedSessions)
    } catch (error) {
      console.error('Failed to load sessions:', error)
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

    // Send message and handle streaming response
    await apiClient.sendMessage(
      {
        sessionId: currentSession.id,
        message: content
      },
      // onChunk: Accumulate streaming content
      (chunk: string) => {
        setStreamingMessage(prev => prev + chunk)
      },
      // onComplete: Finalize the assistant message
      (messageId: string) => {
        const assistantMessage: ChatMessageType = {
          id: messageId,
          role: 'assistant',
          content: streamingMessage,
          timestamp: new Date()
        }

        // Update current session with assistant message
        const finalSession = {
          ...updatedSession,
          messages: [...updatedSession.messages, assistantMessage],
          updatedAt: new Date()
        }
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
      <div className="flex-1 flex flex-col">
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
                    <ChatMessage key={message.id} message={message} />
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
            <ChatInput
              onSendMessage={handleSendMessage}
              disabled={isLoading}
              isLoading={isStreaming}
            />
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
    </div>
  )
}

export default App