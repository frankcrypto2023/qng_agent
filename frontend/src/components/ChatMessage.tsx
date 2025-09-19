import { User, Bot } from 'lucide-react'
import { ChatMessage as ChatMessageType } from '../types'
import { formatTimestamp, cn } from '../utils'

interface ChatMessageProps {
  message: ChatMessageType
  isStreaming?: boolean
}

export function ChatMessage({ message, isStreaming = false }: ChatMessageProps) {
  const isUser = message.role === 'user'

  return (
    <div className={cn(
      "flex gap-4 p-4",
      isUser ? "bg-gray-50" : "bg-white"
    )}>
      {/* Avatar */}
      <div className={cn(
        "w-8 h-8 rounded-full flex items-center justify-center flex-shrink-0",
        isUser 
          ? "bg-blockchain-primary text-white" 
          : "bg-blockchain-secondary text-white"
      )}>
        {isUser ? (
          <User className="w-5 h-5" />
        ) : (
          <Bot className="w-5 h-5" />
        )}
      </div>

      {/* Message Content */}
      <div className="flex-1 min-w-0">
        <div className="flex items-center gap-2 mb-2">
          <span className="font-medium text-gray-900">
            {isUser ? 'You' : 'QNG Agent'}
          </span>
          <span className="text-sm text-gray-500">
            {formatTimestamp(message.timestamp)}
          </span>
        </div>
        
        <div className="prose prose-sm max-w-none">
          <div className={cn(
            "whitespace-pre-wrap break-words",
            isStreaming && "after:content-['▊'] after:animate-pulse after:ml-1"
          )}>
            {message.content}
          </div>
        </div>
      </div>
    </div>
  )
}