import React from 'react';
import { Plus, Settings, MessageSquare } from 'lucide-react';
import { Conversation } from '../types';

interface SidebarProps {
  conversations: Conversation[];
  currentConversationId: string | null;
  onNewConversation: () => void;
  onSelectConversation: (id: string) => void;
  onDeleteConversation: (id: string) => void;
  selectedMode: 'basic' | 'tools' | 'stream';
  onModeChange: (mode: 'basic' | 'tools' | 'stream') => void;
}

export const Sidebar: React.FC<SidebarProps> = ({
  conversations,
  currentConversationId,
  onNewConversation,
  onSelectConversation,
  onDeleteConversation,
  selectedMode,
  onModeChange,
}) => {
  const modeOptions = [
    { value: 'basic', label: '基础模式', description: '简单的对话模式' },
    { value: 'tools', label: '工具模式', description: '支持函数调用' },
    { value: 'stream', label: '流式模式', description: '实时流式响应' },
  ] as const;

  return (
    <div className="w-80 bg-gray-800 border-r border-gray-700 flex flex-col h-full">
      {/* 头部 */}
      <div className="p-4 border-b border-gray-700">
        <div className="flex items-center justify-between mb-4">
          <h1 className="text-xl font-bold text-white">🦙 Harmony Chat</h1>
          <button className="text-gray-400 hover:text-white">
            <Settings className="w-5 h-5" />
          </button>
        </div>
        
        <button
          onClick={onNewConversation}
          className="w-full btn-primary flex items-center justify-center gap-2"
        >
          <Plus className="w-4 h-4" />
          新对话
        </button>
      </div>

      {/* 模式选择 */}
      <div className="p-4 border-b border-gray-700">
        <h3 className="text-sm font-medium text-gray-300 mb-3">对话模式</h3>
        <div className="space-y-2">
          {modeOptions.map((option) => (
            <label
              key={option.value}
              className={`flex items-start p-3 rounded-lg cursor-pointer transition-colors ${
                selectedMode === option.value
                  ? 'bg-blue-600 text-white'
                  : 'bg-gray-700 hover:bg-gray-600 text-gray-300'
              }`}
            >
              <input
                type="radio"
                name="mode"
                value={option.value}
                checked={selectedMode === option.value}
                onChange={(e) => onModeChange(e.target.value as 'basic' | 'tools' | 'stream')}
                className="sr-only"
              />
              <div className="flex-1">
                <div className="font-medium">{option.label}</div>
                <div className="text-xs opacity-80">{option.description}</div>
              </div>
            </label>
          ))}
        </div>
      </div>

      {/* 对话列表 */}
      <div className="flex-1 overflow-y-auto">
        <div className="p-4">
          <h3 className="text-sm font-medium text-gray-300 mb-3">对话历史</h3>
          {conversations.length === 0 ? (
            <div className="text-center text-gray-500 py-8">
              <MessageSquare className="w-12 h-12 mx-auto mb-2 opacity-50" />
              <p>暂无对话</p>
            </div>
          ) : (
            <div className="space-y-2">
              {conversations.map((conversation) => (
                <div
                  key={conversation.id}
                  className={`group flex items-center justify-between p-3 rounded-lg cursor-pointer transition-colors ${
                    currentConversationId === conversation.id
                      ? 'bg-blue-600 text-white'
                      : 'bg-gray-700 hover:bg-gray-600 text-gray-300'
                  }`}
                  onClick={() => onSelectConversation(conversation.id)}
                >
                  <div className="flex-1 min-w-0">
                    <div className="font-medium truncate">{conversation.title}</div>
                    <div className="text-xs opacity-80">
                      {conversation.updatedAt.toLocaleDateString()}
                    </div>
                  </div>
                  
                  {currentConversationId === conversation.id && (
                    <button
                      onClick={(e) => {
                        e.stopPropagation();
                        onDeleteConversation(conversation.id);
                      }}
                      className="opacity-0 group-hover:opacity-100 text-red-400 hover:text-red-300 transition-opacity"
                    >
                      ×
                    </button>
                  )}
                </div>
              ))}
            </div>
          )}
        </div>
      </div>
    </div>
  );
};
