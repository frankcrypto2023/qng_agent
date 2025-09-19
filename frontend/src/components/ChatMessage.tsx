import { User, Bot } from 'lucide-react'
import ReactMarkdown from 'react-markdown'
import remarkGfm from 'remark-gfm'
import rehypeHighlight from 'rehype-highlight'
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
        
        <div className="prose prose-sm max-w-none prose-headings:text-gray-900 prose-code:text-purple-600 prose-code:bg-gray-100 prose-code:px-1 prose-code:py-0.5 prose-code:rounded prose-pre:bg-gray-900 prose-pre:text-gray-100">
          {isUser ? (
            // User messages: simple text display
            <div className="whitespace-pre-wrap break-words">
              {message.content}
            </div>
          ) : (
            // Assistant messages: markdown rendering
            <div className={cn(
              isStreaming && "after:content-['▊'] after:animate-pulse after:ml-1"
            )}>
              <ReactMarkdown
                remarkPlugins={[remarkGfm]}
                rehypePlugins={[rehypeHighlight]}
                components={{
                  // Custom styling for different elements
                  h1: ({children}) => <h1 className="text-xl font-bold text-gray-900 mb-4">{children}</h1>,
                  h2: ({children}) => <h2 className="text-lg font-semibold text-gray-800 mb-3">{children}</h2>,
                  h3: ({children}) => <h3 className="text-base font-medium text-gray-800 mb-2">{children}</h3>,
                  h4: ({children}) => <h4 className="text-sm font-medium text-gray-700 mb-2">{children}</h4>,
                  p: ({children}) => <p className="mb-3 text-gray-700 leading-relaxed">{children}</p>,
                  ul: ({children}) => <ul className="list-disc list-inside mb-3 text-gray-700 space-y-1">{children}</ul>,
                  ol: ({children}) => <ol className="list-decimal list-inside mb-3 text-gray-700 space-y-1">{children}</ol>,
                  li: ({children}) => <li className="ml-2">{children}</li>,
                  blockquote: ({children}) => <blockquote className="border-l-4 border-blockchain-primary pl-4 italic text-gray-600 my-3">{children}</blockquote>,
                  code: ({className, children, ...props}) => {
                    const match = /language-(\w+)/.exec(className || '')
                    return match ? (
                      <pre className="bg-gray-900 text-gray-100 rounded-lg p-4 overflow-x-auto my-3">
                        <code className={className} {...props}>
                          {children}
                        </code>
                      </pre>
                    ) : (
                      <code className="bg-gray-100 text-purple-600 px-1 py-0.5 rounded text-sm" {...props}>
                        {children}
                      </code>
                    )
                  },
                  table: ({children}) => (
                    <div className="overflow-x-auto my-4">
                      <table className="min-w-full border-collapse border border-gray-300">
                        {children}
                      </table>
                    </div>
                  ),
                  thead: ({children}) => <thead className="bg-gray-50">{children}</thead>,
                  th: ({children}) => <th className="border border-gray-300 px-4 py-2 text-left font-medium text-gray-900">{children}</th>,
                  td: ({children}) => <td className="border border-gray-300 px-4 py-2 text-gray-700">{children}</td>,
                  a: ({children, href}) => <a href={href} className="text-blockchain-primary hover:underline" target="_blank" rel="noopener noreferrer">{children}</a>,
                  strong: ({children}) => <strong className="font-semibold text-gray-900">{children}</strong>,
                  em: ({children}) => <em className="italic text-gray-700">{children}</em>,
                }}
              >
                {message.content}
              </ReactMarkdown>
            </div>
          )}
        </div>
      </div>
    </div>
  )
}