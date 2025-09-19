import React, { useState, useRef, useEffect } from 'react'
import { Send, Loader2 } from 'lucide-react'
import { cn } from '../utils'

interface ChatInputProps {
  onSendMessage: (message: string) => void
  disabled?: boolean
  isLoading?: boolean
}

export function ChatInput({ onSendMessage, disabled = false, isLoading = false }: ChatInputProps) {
  const [message, setMessage] = useState('')
  const textareaRef = useRef<HTMLTextAreaElement>(null)

  // Auto-resize textarea
  useEffect(() => {
    const textarea = textareaRef.current
    if (textarea) {
      textarea.style.height = 'auto'
      textarea.style.height = `${Math.min(textarea.scrollHeight, 150)}px`
    }
  }, [message])

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    
    if (!message.trim() || disabled || isLoading) return
    
    onSendMessage(message.trim())
    setMessage('')
  }

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault()
      handleSubmit(e)
    }
  }

  return (
    <div className="border-t border-gray-200 bg-white p-4">
      <form onSubmit={handleSubmit} className="flex gap-3 items-end">
        <div className="flex-1 relative">
          <textarea
            ref={textareaRef}
            value={message}
            onChange={(e) => setMessage(e.target.value)}
            onKeyDown={handleKeyDown}
            placeholder="Ask QNG Agent anything about blockchain, Web3, or cryptocurrency..."
            disabled={disabled || isLoading}
            className={cn(
              "w-full px-4 py-3 border border-gray-300 rounded-xl resize-none",
              "focus:ring-2 focus:ring-blockchain-primary focus:border-transparent",
              "disabled:bg-gray-50 disabled:cursor-not-allowed",
              "placeholder-gray-500"
            )}
            rows={1}
            style={{ maxHeight: '150px' }}
          />
        </div>
        
        <button
          type="submit"
          disabled={!message.trim() || disabled || isLoading}
          className={cn(
            "p-3 rounded-xl transition-colors flex-shrink-0",
            "disabled:bg-gray-300 disabled:cursor-not-allowed",
            message.trim() && !disabled && !isLoading
              ? "bg-blockchain-primary hover:bg-blockchain-primary/90 text-white"
              : "bg-gray-300 text-gray-500"
          )}
        >
          {isLoading ? (
            <Loader2 className="w-5 h-5 animate-spin" />
          ) : (
            <Send className="w-5 h-5" />
          )}
        </button>
      </form>
      
      <div className="mt-2 text-xs text-gray-500 text-center">
        QNG Agent can help with blockchain queries, Web3 operations, and smart contract interactions
      </div>
    </div>
  )
}