import { useState, useEffect, useRef } from 'react';
import { Sidebar } from './components/Sidebar';
import { ChatMessage } from './components/ChatMessage';
import { ChatInput } from './components/ChatInput';
import { Conversation, Message } from './types';
import { harmonyAPI } from './api';

function App() {
  const [conversations, setConversations] = useState<Conversation[]>([]);
  const [currentConversationId, setCurrentConversationId] = useState<string | null>(null);
  const [selectedMode, setSelectedMode] = useState<'basic' | 'tools' | 'stream'>('basic');
  const [isLoading, setIsLoading] = useState(false);
  const messagesEndRef = useRef<HTMLDivElement>(null);

  const currentConversation = conversations.find(c => c.id === currentConversationId);

  // 滚动到底部
  const scrollToBottom = () => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' });
  };

  useEffect(() => {
    scrollToBottom();
  }, [currentConversation?.messages]);

  // 创建新对话
  const createNewConversation = () => {
    const newConversation: Conversation = {
      id: Date.now().toString(),
      title: '新对话',
      messages: [],
      mode: selectedMode,
      createdAt: new Date(),
      updatedAt: new Date(),
    };

    setConversations(prev => [newConversation, ...prev]);
    setCurrentConversationId(newConversation.id);
  };

  // 选择对话
  const selectConversation = (id: string) => {
    setCurrentConversationId(id);
    const conversation = conversations.find(c => c.id === id);
    if (conversation) {
      setSelectedMode(conversation.mode);
    }
  };

  // 删除对话
  const deleteConversation = (id: string) => {
    setConversations(prev => prev.filter(c => c.id !== id));
    if (currentConversationId === id) {
      setCurrentConversationId(null);
    }
  };

  // 模式改变
  const handleModeChange = (mode: 'basic' | 'tools' | 'stream') => {
    setSelectedMode(mode);
    if (currentConversationId) {
      setConversations(prev => prev.map(c => 
        c.id === currentConversationId 
          ? { ...c, mode, updatedAt: new Date() }
          : c
      ));
    }
  };

  // 发送消息
  const sendMessage = async (content: string) => {
    if (!currentConversationId) {
      createNewConversation();
      // 等待状态更新后再发送消息
      setTimeout(() => sendMessage(content), 0);
      return;
    }

    const userMessage: Message = {
      id: Date.now().toString(),
      role: 'user',
      content,
      timestamp: new Date(),
    };

    // 添加用户消息
    setConversations(prev => prev.map(c => 
      c.id === currentConversationId 
        ? {
            ...c,
            messages: [...c.messages, userMessage],
            title: c.messages.length === 0 ? content.slice(0, 50) + '...' : c.title,
            updatedAt: new Date(),
          }
        : c
    ));

    setIsLoading(true);

    try {
      const request = {
        message: content,
        mode: selectedMode,
      };

      if (selectedMode === 'stream') {
        // 流式响应
        let assistantMessage: Message = {
          id: (Date.now() + 1).toString(),
          role: 'assistant',
          content: '',
          timestamp: new Date(),
        };

        // 添加空的助手消息
        setConversations(prev => prev.map(c => 
          c.id === currentConversationId 
            ? { ...c, messages: [...c.messages, assistantMessage] }
            : c
        ));

        await harmonyAPI.chatStream(
          request,
          (chunk) => {
            // 更新助手消息内容
            setConversations(prev => prev.map(c => 
              c.id === currentConversationId 
                ? {
                    ...c,
                    messages: c.messages.map(m => 
                      m.id === assistantMessage.id 
                        ? { ...m, content: m.content + (chunk.response || '') }
                        : m
                    ),
                  }
                : c
            ));
          },
          () => {
            setIsLoading(false);
          },
          (error) => {
            console.error('流式聊天错误:', error);
            setIsLoading(false);
          }
        );
      } else {
        // 普通响应
        const response = await harmonyAPI.chat(request);
        
        const assistantMessage: Message = {
          id: (Date.now() + 1).toString(),
          role: 'assistant',
          content: response.response,
          timestamp: new Date(),
        };

        setConversations(prev => prev.map(c => 
          c.id === currentConversationId 
            ? { ...c, messages: [...c.messages, assistantMessage] }
            : c
        ));

        setIsLoading(false);
      }
    } catch (error) {
      console.error('发送消息错误:', error);
      
      const errorMessage: Message = {
        id: (Date.now() + 1).toString(),
        role: 'assistant',
        content: `错误: ${error instanceof Error ? error.message : '未知错误'}`,
        timestamp: new Date(),
      };

      setConversations(prev => prev.map(c => 
        c.id === currentConversationId 
          ? { ...c, messages: [...c.messages, errorMessage] }
          : c
      ));

      setIsLoading(false);
    }
  };

  return (
    <div className="flex h-screen bg-gray-900">
      <Sidebar
        conversations={conversations}
        currentConversationId={currentConversationId}
        onNewConversation={createNewConversation}
        onSelectConversation={selectConversation}
        onDeleteConversation={deleteConversation}
        selectedMode={selectedMode}
        onModeChange={handleModeChange}
      />
      
      <div className="flex-1 flex flex-col">
        {currentConversation ? (
          <>
            {/* 聊天区域 */}
            <div className="flex-1 overflow-y-auto p-4">
              {currentConversation.messages.length === 0 ? (
                <div className="text-center text-gray-500 py-20">
                  <h2 className="text-2xl font-bold mb-4">欢迎使用 Harmony GPT-OSS</h2>
                  <p className="text-lg mb-8">开始你的对话吧！</p>
                  <div className="space-y-4 text-left max-w-2xl mx-auto">
                    <div className="bg-gray-800 p-4 rounded-lg">
                      <h3 className="font-semibold mb-2">💡 使用提示</h3>
                      <ul className="space-y-1 text-sm">
                        <li>• 基础模式：适合一般对话和问答</li>
                        <li>• 工具模式：支持函数调用和复杂任务</li>
                        <li>• 流式模式：实时流式响应，体验更流畅</li>
                      </ul>
                    </div>
                  </div>
                </div>
              ) : (
                <div className="space-y-4">
                  {currentConversation.messages.map((message) => (
                    <ChatMessage key={message.id} message={message} />
                  ))}
                  <div ref={messagesEndRef} />
                </div>
              )}
            </div>
            
            {/* 输入区域 */}
            <ChatInput
              onSendMessage={sendMessage}
              isLoading={isLoading}
            />
          </>
        ) : (
          <div className="flex-1 flex items-center justify-center">
            <div className="text-center text-gray-500">
              <h2 className="text-2xl font-bold mb-4">选择或创建对话</h2>
              <p>开始使用 Harmony GPT-OSS 进行对话</p>
            </div>
          </div>
        )}
      </div>
    </div>
  );
}

export default App;
