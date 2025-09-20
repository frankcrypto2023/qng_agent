import { useState } from 'react'
import { MessageSquare, Plus, Settings, Trash2, Edit2, Check, X } from 'lucide-react'
import { ChatSession } from '../types'
import { apiClient } from '../api/client'
import { formatTimestamp, cn } from '../utils'

interface SidebarProps {
  sessions: ChatSession[]
  currentSessionId: string | null
  onSessionSelect: (sessionId: string) => void
  onNewChat: () => void
  onOpenSettings: () => void
  onDeleteSession: (sessionId: string) => void
  onSessionsUpdate: () => void
}

export function Sidebar({
  sessions,
  currentSessionId,
  onSessionSelect,
  onNewChat,
  onOpenSettings,
  onDeleteSession,
  onSessionsUpdate
}: SidebarProps) {
  const [deletingSessionId, setDeletingSessionId] = useState<string | null>(null)
  const [editingSessionId, setEditingSessionId] = useState<string | null>(null)
  const [editingTitle, setEditingTitle] = useState<string>('')

  const handleDeleteSession = async (sessionId: string, e: React.MouseEvent) => {
    e.stopPropagation()
    
    if (deletingSessionId) return
    
    setDeletingSessionId(sessionId)
    
    try {
      await apiClient.deleteSession(sessionId)
      onDeleteSession(sessionId)
      onSessionsUpdate()
    } catch (error) {
      console.error('Failed to delete session:', error)
    } finally {
      setDeletingSessionId(null)
    }
  }

  const handleEditSession = (sessionId: string, currentTitle: string, e: React.MouseEvent) => {
    e.stopPropagation()
    setEditingSessionId(sessionId)
    setEditingTitle(currentTitle || 'New Conversation')
  }

  const handleSaveTitle = async (sessionId: string, e: React.MouseEvent) => {
    e.stopPropagation()
    
    try {
      await apiClient.updateSession(sessionId, { title: editingTitle.trim() })
      onSessionsUpdate() // Refresh sessions to show updated title
    } catch (error) {
      console.error('Failed to update session title:', error)
    } finally {
      setEditingSessionId(null)
      setEditingTitle('')
    }
  }

  const handleCancelEdit = (e: React.MouseEvent) => {
    e.stopPropagation()
    setEditingSessionId(null)
    setEditingTitle('')
  }

  const handleTitleKeyPress = (e: React.KeyboardEvent, sessionId: string) => {
    if (e.key === 'Enter') {
      e.preventDefault()
      handleSaveTitle(sessionId, e as any)
    } else if (e.key === 'Escape') {
      e.preventDefault()
      handleCancelEdit(e as any)
    }
  }

  return (
    <div className="w-80 bg-white border-r border-gray-200 flex flex-col">
      {/* Header */}
      <div className="p-4 border-b border-gray-200">
        <div className="flex items-center gap-3 mb-4">
          <div className="w-8 h-8 bg-blockchain-gradient rounded-lg flex items-center justify-center">
            <MessageSquare className="w-5 h-5 text-white" />
          </div>
          <h1 className="text-xl font-bold text-gray-900">QNG Intelligent Agent</h1>
        </div>
        
        <button
          onClick={onNewChat}
          className="w-full flex items-center gap-3 px-4 py-3 bg-blockchain-primary text-white rounded-lg hover:bg-blockchain-primary/90 transition-colors"
        >
          <Plus className="w-5 h-5" />
          New Chat
        </button>
      </div>

      {/* Sessions List */}
      <div className="flex-1 overflow-y-auto p-4 space-y-2">
        {!sessions || sessions.length === 0 ? (
          <div className="text-center text-gray-500 py-8">
            <MessageSquare className="w-12 h-12 mx-auto mb-3 text-gray-300" />
            <p>No conversations yet</p>
            <p className="text-sm">Start a new chat to begin</p>
          </div>
        ) : (
          sessions.map((session) => (
            <div
              key={session.id}
              onClick={() => onSessionSelect(session.id)}
              className={cn(
                "group relative p-3 rounded-lg cursor-pointer transition-colors",
                currentSessionId === session.id
                  ? "bg-blockchain-primary/10 border border-blockchain-primary/20"
                  : "hover:bg-gray-50"
              )}
            >
              <div className="flex items-start gap-3">
                <MessageSquare className="w-5 h-5 text-blockchain-primary mt-0.5 flex-shrink-0" />
                <div className="flex-1 min-w-0">
                  {editingSessionId === session.id ? (
                    <div className="flex items-center gap-2">
                      <input
                        type="text"
                        value={editingTitle}
                        onChange={(e) => setEditingTitle(e.target.value)}
                        onKeyPress={(e) => handleTitleKeyPress(e, session.id)}
                        className="flex-1 px-2 py-1 text-sm border border-gray-300 rounded focus:outline-none focus:border-blockchain-primary"
                        autoFocus
                        onClick={(e) => e.stopPropagation()}
                      />
                      <button
                        onClick={(e) => handleSaveTitle(session.id, e)}
                        className="p-1 hover:bg-green-100 rounded transition-colors"
                      >
                        <Check className="w-4 h-4 text-green-600" />
                      </button>
                      <button
                        onClick={handleCancelEdit}
                        className="p-1 hover:bg-gray-100 rounded transition-colors"
                      >
                        <X className="w-4 h-4 text-gray-600" />
                      </button>
                    </div>
                  ) : (
                    <div className="flex items-center gap-2">
                      <h3 className="font-medium text-gray-900 truncate flex-1">
                        {session.title || 'New Conversation'}
                      </h3>
                      <button
                        onClick={(e) => handleEditSession(session.id, session.title || 'New Conversation', e)}
                        className="opacity-0 group-hover:opacity-100 p-1 hover:bg-gray-100 rounded transition-all"
                      >
                        <Edit2 className="w-4 h-4 text-gray-600" />
                      </button>
                    </div>
                  )}
                  <p className="text-sm text-gray-500 mt-1">
                    {formatTimestamp(new Date(session.updatedAt))}
                  </p>
                  {session.messages && session.messages.length > 0 && (
                    <p className="text-sm text-gray-400 mt-1 truncate">
                      {session.messages[session.messages.length - 1].content}
                    </p>
                  )}
                </div>
                <button
                  onClick={(e) => handleDeleteSession(session.id, e)}
                  disabled={deletingSessionId === session.id}
                  className="opacity-0 group-hover:opacity-100 p-1 hover:bg-red-100 rounded transition-all"
                >
                  <Trash2 className="w-4 h-4 text-red-500" />
                </button>
              </div>
            </div>
          ))
        )}
      </div>

      {/* Settings */}
      <div className="p-4 border-t border-gray-200">
        <button
          onClick={onOpenSettings}
          className="w-full flex items-center gap-3 px-4 py-3 text-gray-700 hover:bg-gray-50 rounded-lg transition-colors"
        >
          <Settings className="w-5 h-5" />
          Settings
        </button>
      </div>
    </div>
  )
}