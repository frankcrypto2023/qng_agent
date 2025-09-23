import { useState, useEffect } from 'react'
import { X, Plus, Trash2, Save } from 'lucide-react'
import { AppSettings, MCPServerConfig, LLMProviderConfig, LLMProviderType } from '../types'
import { apiClient } from '../api/client'
import { cn } from '../utils'

interface SettingsModalProps {
  isOpen: boolean
  onClose: () => void
}

export function SettingsModal({ isOpen, onClose }: SettingsModalProps) {
  const [settings, setSettings] = useState<AppSettings>({
    mcp_servers: [],
    llm_provider: {
      type: 'openai',
      name: '',
      url: '',
      token: '',
      model_name: '',
      timeout: 120,
      app_name: '',
      app_url: ''
    }
  })
  const [isLoading, setIsLoading] = useState(false)
  const [isSaving, setIsSaving] = useState(false)

  // Load settings when modal opens
  useEffect(() => {
    if (isOpen) {
      loadSettings()
    }
  }, [isOpen])

  const loadSettings = async () => {
    setIsLoading(true)
    try {
      const loadedSettings = await apiClient.getSettings()
      setSettings(loadedSettings)
    } catch (error) {
      console.error('Failed to load settings:', error)
    } finally {
      setIsLoading(false)
    }
  }

  const handleSave = async () => {
    setIsSaving(true)
    try {
      await apiClient.updateSettings(settings)
      onClose()
    } catch (error) {
      console.error('Failed to save settings:', error)
    } finally {
      setIsSaving(false)
    }
  }

  // MCP Server management
  const addMCPServer = () => {
    setSettings(prev => ({
      ...prev,
      mcp_servers: [
        ...prev.mcp_servers,
        { name: '', url: '', enabled: true }
      ]
    }))
  }

  const updateMCPServer = (index: number, updates: Partial<MCPServerConfig>) => {
    setSettings(prev => ({
      ...prev,
      mcp_servers: prev.mcp_servers.map((server, i) => 
        i === index ? { ...server, ...updates } : server
      )
    }))
  }

  const removeMCPServer = (index: number) => {
    setSettings(prev => ({
      ...prev,
      mcp_servers: prev.mcp_servers.filter((_, i) => i !== index)
    }))
  }

  // LLM Provider management
  const updateLLMProvider = (updates: Partial<LLMProviderConfig>) => {
    setSettings(prev => ({
      ...prev,
      llm_provider: { ...prev.llm_provider, ...updates }
    }))
  }

  // Provider type options
  const providerTypes: { value: LLMProviderType; label: string; description: string; defaultUrl: string }[] = [
    {
      value: 'openai',
      label: 'OpenAI',
      description: 'OpenAI API or compatible services',
      defaultUrl: 'https://api.openai.com/v1'
    },
    {
      value: 'openrouter',
      label: 'OpenRouter',
      description: 'Access to multiple AI models through OpenRouter',
      defaultUrl: 'https://openrouter.ai/api/v1'
    },
    {
      value: 'groq',
      label: 'Groq',
      description: 'Fast inference with Groq API',
      defaultUrl: 'https://api.groq.com/openai/v1'
    },
    {
      value: 'anthropic',
      label: 'Anthropic',
      description: 'Anthropic Claude API',
      defaultUrl: 'https://api.anthropic.com/v1'
    },
    {
      value: 'custom',
      label: 'Custom',
      description: 'Custom OpenAI-compatible API endpoint',
      defaultUrl: ''
    }
  ]

  const getDefaultUrl = (type: LLMProviderType) => {
    const provider = providerTypes.find(p => p.value === type)
    return provider?.defaultUrl || ''
  }

  if (!isOpen) return null

  return (
    <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50">
      <div className="bg-white rounded-xl shadow-xl w-full max-w-4xl max-h-[90vh] overflow-hidden">
        {/* Header */}
        <div className="flex items-center justify-between p-6 border-b border-gray-200">
          <h2 className="text-2xl font-bold text-gray-900">Settings</h2>
          <button
            onClick={onClose}
            className="p-2 hover:bg-gray-100 rounded-lg transition-colors"
          >
            <X className="w-5 h-5" />
          </button>
        </div>

        {/* Content */}
        <div className="p-6 overflow-y-auto max-h-[calc(90vh-120px)]">
          {isLoading ? (
            <div className="flex items-center justify-center py-12">
              <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-blockchain-primary"></div>
            </div>
          ) : (
            <div className="space-y-8">
              {/* LLM Provider Section */}
              <section>
                <h3 className="text-lg font-semibold text-gray-900 mb-4">LLM Provider Configuration</h3>
                <div className="bg-gray-50 rounded-lg p-4 space-y-4">
                  {/* Provider Type Selection */}
                  <div>
                    <label className="block text-sm font-medium text-gray-700 mb-2">
                      Provider Type
                    </label>
                    <select
                      value={settings.llm_provider.type}
                      onChange={(e) => {
                        const newType = e.target.value as LLMProviderType
                        updateLLMProvider({ 
                          type: newType,
                          url: getDefaultUrl(newType)
                        })
                      }}
                      className="w-full px-3 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-blockchain-primary focus:border-transparent"
                    >
                      {providerTypes.map((provider) => (
                        <option key={provider.value} value={provider.value}>
                          {provider.label} - {provider.description}
                        </option>
                      ))}
                    </select>
                  </div>

                  <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                    <div>
                      <label className="block text-sm font-medium text-gray-700 mb-2">
                        Provider Name
                      </label>
                      <input
                        type="text"
                        value={settings.llm_provider.name}
                        onChange={(e) => updateLLMProvider({ name: e.target.value })}
                        placeholder="e.g., OpenAI, Anthropic, OpenRouter"
                        className="w-full px-3 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-blockchain-primary focus:border-transparent"
                      />
                    </div>
                    <div>
                      <label className="block text-sm font-medium text-gray-700 mb-2">
                        Model Name
                      </label>
                      <input
                        type="text"
                        value={settings.llm_provider.model_name}
                        onChange={(e) => updateLLMProvider({ model_name: e.target.value })}
                        placeholder={
                          settings.llm_provider.type === 'openrouter' 
                            ? 'e.g., openai/gpt-4o, anthropic/claude-3-opus' 
                            : settings.llm_provider.type === 'groq'
                            ? 'e.g., llama-3.1-70b-versatile, mixtral-8x7b-32768'
                            : 'e.g., gpt-4, claude-3-opus'
                        }
                        className="w-full px-3 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-blockchain-primary focus:border-transparent"
                      />
                    </div>
                  </div>
                  
                  <div>
                    <label className="block text-sm font-medium text-gray-700 mb-2">
                      API URL
                    </label>
                    <input
                      type="url"
                      value={settings.llm_provider.url}
                      onChange={(e) => updateLLMProvider({ url: e.target.value })}
                      placeholder={getDefaultUrl(settings.llm_provider.type)}
                      className="w-full px-3 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-blockchain-primary focus:border-transparent"
                    />
                  </div>
                  
                  <div>
                    <label className="block text-sm font-medium text-gray-700 mb-2">
                      API Token
                    </label>
                    <input
                      type="password"
                      value={settings.llm_provider.token}
                      onChange={(e) => updateLLMProvider({ token: e.target.value })}
                      placeholder="Your API token"
                      className="w-full px-3 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-blockchain-primary focus:border-transparent"
                    />
                  </div>

                  <div>
                    <label className="block text-sm font-medium text-gray-700 mb-2">
                      Request Timeout (seconds)
                    </label>
                    <input
                      type="number"
                      min="30"
                      max="600"
                      value={settings.llm_provider.timeout || 120}
                      onChange={(e) => updateLLMProvider({ timeout: parseInt(e.target.value) || 120 })}
                      placeholder="120"
                      className="w-full px-3 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-blockchain-primary focus:border-transparent"
                    />
                    <p className="text-xs text-gray-500 mt-1">
                      Timeout for LLM requests in seconds. Recommended: 30-60 for fast responses, 120-180 for complex tasks.
                    </p>
                  </div>

                  {/* OpenRouter specific fields */}
                  {settings.llm_provider.type === 'openrouter' && (
                    <div className="border-t pt-4 space-y-4">
                      <div className="text-sm text-gray-600">
                        <strong>OpenRouter Configuration:</strong> These fields are optional but help with app attribution on OpenRouter leaderboards.
                      </div>
                      <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                        <div>
                          <label className="block text-sm font-medium text-gray-700 mb-2">
                            App Name (Optional)
                          </label>
                          <input
                            type="text"
                            value={settings.llm_provider.app_name || ''}
                            onChange={(e) => updateLLMProvider({ app_name: e.target.value })}
                            placeholder="QNG Agent"
                            className="w-full px-3 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-blockchain-primary focus:border-transparent"
                          />
                        </div>
                        <div>
                          <label className="block text-sm font-medium text-gray-700 mb-2">
                            App URL (Optional)
                          </label>
                          <input
                            type="url"
                            value={settings.llm_provider.app_url || ''}
                            onChange={(e) => updateLLMProvider({ app_url: e.target.value })}
                            placeholder="https://your-app.com"
                            className="w-full px-3 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-blockchain-primary focus:border-transparent"
                          />
                        </div>
                      </div>
                    </div>
                  )}
                </div>
              </section>

              {/* MCP Servers Section */}
              <section>
                <div className="flex items-center justify-between mb-4">
                  <h3 className="text-lg font-semibold text-gray-900">MCP Servers</h3>
                  <button
                    onClick={addMCPServer}
                    className="flex items-center gap-2 px-3 py-2 bg-blockchain-primary text-white rounded-lg hover:bg-blockchain-primary/90 transition-colors"
                  >
                    <Plus className="w-4 h-4" />
                    Add Server
                  </button>
                </div>
                
                {settings.mcp_servers.length === 0 ? (
                  <div className="text-center py-8 text-gray-500 bg-gray-50 rounded-lg">
                    No MCP servers configured. Add one to enable tool integrations.
                  </div>
                ) : (
                  <div className="space-y-4">
                    {settings.mcp_servers.map((server, index) => (
                      <div key={index} className="bg-gray-50 rounded-lg p-4">
                        <div className="flex items-start gap-4">
                          <div className="flex-1 space-y-3">
                            <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
                              <input
                                type="text"
                                value={server.name}
                                onChange={(e) => updateMCPServer(index, { name: e.target.value })}
                                placeholder="Server name"
                                className="px-3 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-blockchain-primary focus:border-transparent"
                              />
                              <input
                                type="url"
                                value={server.url}
                                onChange={(e) => updateMCPServer(index, { url: e.target.value })}
                                placeholder="Server URL (SSE endpoint)"
                                className="px-3 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-blockchain-primary focus:border-transparent"
                              />
                            </div>
                            <div className="flex items-center gap-2">
                              <input
                                type="checkbox"
                                id={`enabled-${index}`}
                                checked={server.enabled}
                                onChange={(e) => updateMCPServer(index, { enabled: e.target.checked })}
                                className="rounded border-gray-300 text-blockchain-primary focus:ring-blockchain-primary"
                              />
                              <label htmlFor={`enabled-${index}`} className="text-sm text-gray-700">
                                Enabled
                              </label>
                            </div>
                          </div>
                          <button
                            onClick={() => removeMCPServer(index)}
                            className="p-2 text-red-500 hover:bg-red-50 rounded-lg transition-colors"
                          >
                            <Trash2 className="w-4 h-4" />
                          </button>
                        </div>
                      </div>
                    ))}
                  </div>
                )}
              </section>
            </div>
          )}
        </div>

        {/* Footer */}
        <div className="flex items-center justify-end gap-3 p-6 border-t border-gray-200">
          <button
            onClick={onClose}
            className="px-4 py-2 text-gray-700 hover:bg-gray-100 rounded-lg transition-colors"
          >
            Cancel
          </button>
          <button
            onClick={handleSave}
            disabled={isSaving}
            className={cn(
              "flex items-center gap-2 px-4 py-2 rounded-lg transition-colors",
              isSaving
                ? "bg-gray-300 text-gray-500 cursor-not-allowed"
                : "bg-blockchain-primary text-white hover:bg-blockchain-primary/90"
            )}
          >
            {isSaving ? (
              <>
                <div className="w-4 h-4 border-2 border-gray-400 border-t-transparent rounded-full animate-spin" />
                Saving...
              </>
            ) : (
              <>
                <Save className="w-4 h-4" />
                Save Settings
              </>
            )}
          </button>
        </div>
      </div>
    </div>
  )
}