package config

import (
	"fmt"
	"github.com/spf13/viper"
	"log"
)

type Config struct {
	Server      ServerConfig      `mapstructure:"server" yaml:"server"`
	Logging     LoggingConfig     `mapstructure:"logging" yaml:"logging"`
	LLM         LLMConfig         `mapstructure:"llm" yaml:"llm"`
	MCP         MCPConfig         `mapstructure:"mcp" yaml:"mcp"`
	Agent       AgentConfig       `mapstructure:"agent" yaml:"agent"`
	Frontend    FrontendConfig    `mapstructure:"frontend" yaml:"frontend"`
	Database    DatabaseConfig    `mapstructure:"database" yaml:"database"`
	Cache       CacheConfig       `mapstructure:"cache" yaml:"cache"`
	Security    SecurityConfig    `mapstructure:"security" yaml:"security"`
	Monitoring  MonitoringConfig  `mapstructure:"monitoring" yaml:"monitoring"`
	Development DevelopmentConfig `mapstructure:"development" yaml:"development"`
}

type ServerConfig struct {
	Host string `mapstructure:"host"`
	Port int    `mapstructure:"port"`
	Mode string `mapstructure:"mode"`
}

type LoggingConfig struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"`
	Output string `mapstructure:"output"`
	File   string `mapstructure:"file"`
}

type LLMConfig struct {
	Provider   string           `mapstructure:"provider" yaml:"provider"`
	OpenAI     OpenAIConfig     `mapstructure:"openai" yaml:"openai"`
	Gemini     GeminiConfig     `mapstructure:"gemini" yaml:"gemini"`
	Anthropic  AnthropicConfig  `mapstructure:"anthropic" yaml:"anthropic"`
	ModelScope ModelScopeConfig `mapstructure:"modelscope" yaml:"modelscope"`
	LlamaCpp   LlamaCppConfig   `mapstructure:"llamacpp" yaml:"llamacpp"`
}

type OpenAIConfig struct {
	APIKey    string `mapstructure:"api_key" yaml:"api_key"`
	Model     string `mapstructure:"model" yaml:"model"`
	BaseURL   string `mapstructure:"base_url" yaml:"base_url"`
	Timeout   int    `mapstructure:"timeout" yaml:"timeout"`
	MaxTokens int    `mapstructure:"max_tokens" yaml:"max_tokens"`
}

type GeminiConfig struct {
	APIKey  string `mapstructure:"api_key"`
	Model   string `mapstructure:"model"`
	Timeout int    `mapstructure:"timeout"`
}

type AnthropicConfig struct {
	APIKey  string `mapstructure:"api_key"`
	Model   string `mapstructure:"model"`
	Timeout int    `mapstructure:"timeout"`
}

type ModelScopeConfig struct {
	APIKey    string `mapstructure:"api_key"`
	Model     string `mapstructure:"model"`
	BaseURL   string `mapstructure:"base_url"`
	Timeout   int    `mapstructure:"timeout"`
	MaxTokens int    `mapstructure:"max_tokens"`
}

type LlamaCppConfig struct {
	ExecutablePath string  `mapstructure:"executable_path" yaml:"executable_path"`
	BaseURL        string  `mapstructure:"base_url" yaml:"base_url"`
	ServerPort     int     `mapstructure:"server_port" yaml:"server_port"`
	ModelPath      string  `mapstructure:"model_path" yaml:"model_path"`
	ContextSize    int     `mapstructure:"context_size" yaml:"context_size"`
	Threads        int     `mapstructure:"threads" yaml:"threads"`
	Temperature    float64 `mapstructure:"temperature" yaml:"temperature"`
	TopP           float64 `mapstructure:"top_p" yaml:"top_p"`
	TopK           int     `mapstructure:"top_k" yaml:"top_k"`
	MaxTokens      int     `mapstructure:"max_tokens" yaml:"max_tokens"`
	Timeout        int     `mapstructure:"timeout" yaml:"timeout"`
	GPU            bool    `mapstructure:"gpu" yaml:"gpu"`
	GPUThreads     int     `mapstructure:"gpu_threads" yaml:"gpu_threads"`
	GPULayers      int     `mapstructure:"gpu_layers" yaml:"gpu_layers"`
	MemoryMap      bool    `mapstructure:"memory_map" yaml:"memory_map"`
	MemoryF16      bool    `mapstructure:"memory_f16" yaml:"memory_f16"`
	MemoryLock     bool    `mapstructure:"memory_lock" yaml:"memory_lock"`
	RepeatPenalty  float64 `mapstructure:"repeat_penalty" yaml:"repeat_penalty"`
}

type MCPConfig struct {
	Servers map[string]MCPServerConfig `mapstructure:"servers"`
}

// 注意：QNGConfig、ChainConfig、TransactionConfig、LangGraphConfig、MetaMaskConfig
// 已移除，因为现在使用外部Chain服务

type AgentConfig struct {
	Name            string         `mapstructure:"name"`
	Version         string         `mapstructure:"version"`
	ExecutionEngine string         `mapstructure:"execution_engine"` // sop, langgraph, auto
	Workflow        WorkflowConfig `mapstructure:"workflow"`
	Polling         PollingConfig  `mapstructure:"polling"`
	LLM             LLMConfig      `mapstructure:"llm"`
	MCP             MCPConfig      `mapstructure:"mcp"`
}

type WorkflowConfig struct {
	Timeout    int `mapstructure:"timeout"`
	MaxRetries int `mapstructure:"max_retries"`
	RetryDelay int `mapstructure:"retry_delay"`
}

type PollingConfig struct {
	Interval    int `mapstructure:"interval"`
	Timeout     int `mapstructure:"timeout"`
	MaxAttempts int `mapstructure:"max_attempts"`
}

type FrontendConfig struct {
	Enabled   bool            `mapstructure:"enabled"`
	Host      string          `mapstructure:"host"`
	Port      int             `mapstructure:"port"`
	BuildDir  string          `mapstructure:"build_dir"`
	API       APIConfig       `mapstructure:"api"`
	WebSocket WebSocketConfig `mapstructure:"websocket"`
}

type APIConfig struct {
	BaseURL string `mapstructure:"base_url"`
	Timeout int    `mapstructure:"timeout"`
}

type WebSocketConfig struct {
	Enabled bool   `mapstructure:"enabled"`
	URL     string `mapstructure:"url"`
}

type DatabaseConfig struct {
	Driver   string         `mapstructure:"driver"`
	SQLite   SQLiteConfig   `mapstructure:"sqlite"`
	Postgres PostgresConfig `mapstructure:"postgres"`
	MySQL    MySQLConfig    `mapstructure:"mysql"`
}

type SQLiteConfig struct {
	Path string `mapstructure:"path"`
}

type PostgresConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	User     string `mapstructure:"user"`
	Password string `mapstructure:"password"`
	Database string `mapstructure:"database"`
	SSLMode  string `mapstructure:"ssl_mode"`
}

type MySQLConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	User     string `mapstructure:"user"`
	Password string `mapstructure:"password"`
	Database string `mapstructure:"database"`
}

type CacheConfig struct {
	Driver string      `mapstructure:"driver"`
	Redis  RedisConfig `mapstructure:"redis"`
}

type RedisConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Password string `mapstructure:"password"`
	Database int    `mapstructure:"database"`
	Timeout  int    `mapstructure:"timeout"`
}

type SecurityConfig struct {
	JWTSecret string     `mapstructure:"jwt_secret"`
	JWTExpiry string     `mapstructure:"jwt_expiry"`
	CORS      CORSConfig `mapstructure:"cors"`
}

type CORSConfig struct {
	Enabled bool     `mapstructure:"enabled"`
	Origins []string `mapstructure:"origins"`
	Methods []string `mapstructure:"methods"`
	Headers []string `mapstructure:"headers"`
}

type MonitoringConfig struct {
	Enabled     bool              `mapstructure:"enabled"`
	Metrics     MetricsConfig     `mapstructure:"metrics"`
	HealthCheck HealthCheckConfig `mapstructure:"health_check"`
}

type MetricsConfig struct {
	Enabled bool `mapstructure:"enabled"`
	Port    int  `mapstructure:"port"`
}

type HealthCheckConfig struct {
	Enabled bool   `mapstructure:"enabled"`
	Port    int    `mapstructure:"port"`
	Path    string `mapstructure:"path"`
}

type DevelopmentConfig struct {
	HotReload bool `mapstructure:"hot_reload"`
	Debug     bool `mapstructure:"debug"`
	CORS      bool `mapstructure:"cors"`
}

type MCPServerConfig struct {
	Enabled      bool     `mapstructure:"enabled"`
	Protocol     string   `mapstructure:"protocol"` // sse or stdio
	URL          string   `mapstructure:"url"`      // for sse protocol
	Command      []string `mapstructure:"command"`  // for stdio protocol
	Timeout      int      `mapstructure:"timeout"`
	Capabilities []string `mapstructure:"capabilities"`
}

// MetaMaskConfig MetaMask配置
type MetaMaskConfig struct {
	Enabled bool   `mapstructure:"enabled"`
	Network string `mapstructure:"network"`
	Timeout int    `mapstructure:"timeout"`
}

// QNGConfig QNG配置
type QNGConfig struct {
	Enabled     bool                 `mapstructure:"enabled"`
	Network     string               `mapstructure:"network"`
	RPCURL      string               `mapstructure:"rpc_url"`
	Transaction QNGTransactionConfig `mapstructure:"transaction"`
	LangGraph   QNGLangGraphConfig   `mapstructure:"langgraph"`
	LLM         LLMConfig            `mapstructure:"llm"`
}

// QNGTransactionConfig QNG交易配置
type QNGTransactionConfig struct {
	ConfirmationTimeout   int `mapstructure:"confirmation_timeout"`
	PollingInterval       int `mapstructure:"polling_interval"`
	RequiredConfirmations int `mapstructure:"required_confirmations"`
}

// QNGLangGraphConfig QNG LangGraph配置
type QNGLangGraphConfig struct {
	Enabled bool     `mapstructure:"enabled"`
	Nodes   []string `mapstructure:"nodes"`
}

func LoadConfig(configPath string) *Config {
	viper.SetConfigFile(configPath)

	// 设置默认值
	setDefaults()

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			log.Printf("⚠️  配置文件读取失败: %v", err)
		}
	}

	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		log.Printf("❌ 配置解析失败: %v", err)
		return nil
	}

	return &config
}

// Load 函数，使用默认配置文件路径
func Load() (*Config, error) {
	return LoadFromFile("config/config.yaml")
}

// LoadFromFile 从指定文件加载配置
func LoadFromFile(configPath string) (*Config, error) {
	viper.SetConfigFile(configPath)

	// 设置默认值
	setDefaults()

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, err
		}
	}

	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, err
	}

	return &config, nil
}

// Save 保存配置到文件
func Save(cfg *Config) error {
	return SaveToFile(cfg, "config/config.yaml")
}

// SaveToFile 保存配置到指定文件
func SaveToFile(cfg *Config, configPath string) error {
	// 创建新的viper实例避免冲突
	v := viper.New()
	v.SetConfigFile(configPath)
	if err := v.ReadInConfig(); err != nil {
		return fmt.Errorf("read existing config: %w", err)
	}

	// 更新LLM配置
	v.Set("llm.provider", cfg.LLM.Provider)
	if cfg.LLM.OpenAI.APIKey != "" {
		v.Set("llm.openai.api_key", cfg.LLM.OpenAI.APIKey)
	}
	if cfg.LLM.OpenAI.Model != "" {
		v.Set("llm.openai.model", cfg.LLM.OpenAI.Model)
	}
	if cfg.LLM.OpenAI.BaseURL != "" {
		v.Set("llm.openai.base_url", cfg.LLM.OpenAI.BaseURL)
	}
	if cfg.LLM.OpenAI.Timeout > 0 {
		v.Set("llm.openai.timeout", cfg.LLM.OpenAI.Timeout)
	}
	if cfg.LLM.OpenAI.MaxTokens > 0 {
		v.Set("llm.openai.max_tokens", cfg.LLM.OpenAI.MaxTokens)
	}

	if cfg.LLM.Gemini.APIKey != "" {
		v.Set("llm.gemini.api_key", cfg.LLM.Gemini.APIKey)
	}
	if cfg.LLM.Gemini.Model != "" {
		v.Set("llm.gemini.model", cfg.LLM.Gemini.Model)
	}
	if cfg.LLM.Gemini.Timeout > 0 {
		v.Set("llm.gemini.timeout", cfg.LLM.Gemini.Timeout)
	}

	if cfg.LLM.Anthropic.APIKey != "" {
		v.Set("llm.anthropic.api_key", cfg.LLM.Anthropic.APIKey)
	}
	if cfg.LLM.Anthropic.Model != "" {
		v.Set("llm.anthropic.model", cfg.LLM.Anthropic.Model)
	}
	if cfg.LLM.Anthropic.Timeout > 0 {
		v.Set("llm.anthropic.timeout", cfg.LLM.Anthropic.Timeout)
	}

	if cfg.LLM.ModelScope.APIKey != "" {
		v.Set("llm.modelscope.api_key", cfg.LLM.ModelScope.APIKey)
	}
	if cfg.LLM.ModelScope.Model != "" {
		v.Set("llm.modelscope.model", cfg.LLM.ModelScope.Model)
	}
	if cfg.LLM.ModelScope.BaseURL != "" {
		v.Set("llm.modelscope.base_url", cfg.LLM.ModelScope.BaseURL)
	}
	if cfg.LLM.ModelScope.Timeout > 0 {
		v.Set("llm.modelscope.timeout", cfg.LLM.ModelScope.Timeout)
	}
	if cfg.LLM.ModelScope.MaxTokens > 0 {
		v.Set("llm.modelscope.max_tokens", cfg.LLM.ModelScope.MaxTokens)
	}

	// 更新LlamaCpp配置
	if cfg.LLM.LlamaCpp.BaseURL != "" {
		v.Set("llm.llamacpp.base_url", cfg.LLM.LlamaCpp.BaseURL)
	}
	if cfg.LLM.LlamaCpp.ServerPort > 0 {
		v.Set("llm.llamacpp.server_port", cfg.LLM.LlamaCpp.ServerPort)
	}
	if cfg.LLM.LlamaCpp.ModelPath != "" {
		v.Set("llm.llamacpp.model_path", cfg.LLM.LlamaCpp.ModelPath)
	}
	if cfg.LLM.LlamaCpp.ContextSize > 0 {
		v.Set("llm.llamacpp.context_size", cfg.LLM.LlamaCpp.ContextSize)
	}
	if cfg.LLM.LlamaCpp.Threads > 0 {
		v.Set("llm.llamacpp.threads", cfg.LLM.LlamaCpp.Threads)
	}
	if cfg.LLM.LlamaCpp.Temperature > 0 {
		v.Set("llm.llamacpp.temperature", cfg.LLM.LlamaCpp.Temperature)
	}
	if cfg.LLM.LlamaCpp.TopP > 0 {
		v.Set("llm.llamacpp.top_p", cfg.LLM.LlamaCpp.TopP)
	}
	if cfg.LLM.LlamaCpp.TopK > 0 {
		v.Set("llm.llamacpp.top_k", cfg.LLM.LlamaCpp.TopK)
	}
	if cfg.LLM.LlamaCpp.MaxTokens > 0 {
		v.Set("llm.llamacpp.max_tokens", cfg.LLM.LlamaCpp.MaxTokens)
	}
	if cfg.LLM.LlamaCpp.Timeout > 0 {
		v.Set("llm.llamacpp.timeout", cfg.LLM.LlamaCpp.Timeout)
	}
	v.Set("llm.llamacpp.gpu", cfg.LLM.LlamaCpp.GPU)
	if cfg.LLM.LlamaCpp.GPUThreads > 0 {
		v.Set("llm.llamacpp.gpu_threads", cfg.LLM.LlamaCpp.GPUThreads)
	}
	if cfg.LLM.LlamaCpp.GPULayers > 0 {
		v.Set("llm.llamacpp.gpu_layers", cfg.LLM.LlamaCpp.GPULayers)
	}
	v.Set("llm.llamacpp.memory_map", cfg.LLM.LlamaCpp.MemoryMap)
	v.Set("llm.llamacpp.memory_f16", cfg.LLM.LlamaCpp.MemoryF16)
	v.Set("llm.llamacpp.memory_lock", cfg.LLM.LlamaCpp.MemoryLock)
	if cfg.LLM.LlamaCpp.RepeatPenalty > 0 {
		v.Set("llm.llamacpp.repeat_penalty", cfg.LLM.LlamaCpp.RepeatPenalty)
	}

	// 更新MCP服务器配置
	for serverName, serverConfig := range cfg.MCP.Servers {
		v.Set(fmt.Sprintf("mcp.servers.%s.enabled", serverName), serverConfig.Enabled)
		v.Set(fmt.Sprintf("mcp.servers.%s.protocol", serverName), serverConfig.Protocol)
		v.Set(fmt.Sprintf("mcp.servers.%s.timeout", serverName), serverConfig.Timeout)

		if serverConfig.URL != "" {
			v.Set(fmt.Sprintf("mcp.servers.%s.url", serverName), serverConfig.URL)
		}
		if len(serverConfig.Command) > 0 {
			v.Set(fmt.Sprintf("mcp.servers.%s.command", serverName), serverConfig.Command)
		}
		if len(serverConfig.Capabilities) > 0 {
			v.Set(fmt.Sprintf("mcp.servers.%s.capabilities", serverName), serverConfig.Capabilities)
		}
	}

	// 写回配置文件
	return v.WriteConfig()
}

func setDefaults() {
	// 服务器默认值
	viper.SetDefault("server.host", "0.0.0.0")
	viper.SetDefault("server.port", 8080)
	viper.SetDefault("server.mode", "release")

	// 日志默认值
	viper.SetDefault("logging.level", "info")
	viper.SetDefault("logging.format", "json")
	viper.SetDefault("logging.output", "stdout")

	// LLM默认值
	viper.SetDefault("llm.provider", "openai")
	viper.SetDefault("llm.openai.model", "gpt-4")
	viper.SetDefault("llm.openai.base_url", "https://api.openai.com/v1")
	viper.SetDefault("llm.openai.timeout", 30)
	viper.SetDefault("llm.openai.max_tokens", 2000)
	viper.SetDefault("llm.modelscope.model", "qwen/Qwen2.5-7B-Instruct")
	viper.SetDefault("llm.modelscope.base_url", "https://api.modelscope.cn/v1")
	viper.SetDefault("llm.modelscope.timeout", 30)
	viper.SetDefault("llm.modelscope.max_tokens", 2000)

	// LlamaCpp默认值
	viper.SetDefault("llm.llamacpp.executable_path", "llama-server")
	viper.SetDefault("llm.llamacpp.base_url", "http://localhost:8081")
	viper.SetDefault("llm.llamacpp.server_port", 8081)
	viper.SetDefault("llm.llamacpp.model_path", "~/models/llama-2-7b-chat.gguf")
	viper.SetDefault("llm.llamacpp.context_size", 4096)
	viper.SetDefault("llm.llamacpp.threads", 4)
	viper.SetDefault("llm.llamacpp.temperature", 0.7)
	viper.SetDefault("llm.llamacpp.top_p", 0.9)
	viper.SetDefault("llm.llamacpp.top_k", 40)
	viper.SetDefault("llm.llamacpp.max_tokens", 2000)
	viper.SetDefault("llm.llamacpp.timeout", 60)
	viper.SetDefault("llm.llamacpp.gpu", false)
	viper.SetDefault("llm.llamacpp.gpu_threads", 1)
	viper.SetDefault("llm.llamacpp.gpu_layers", 0)
	viper.SetDefault("llm.llamacpp.memory_map", true)
	viper.SetDefault("llm.llamacpp.memory_f16", false)
	viper.SetDefault("llm.llamacpp.memory_lock", false)
	viper.SetDefault("llm.llamacpp.repeat_penalty", 1.1)

	// MCP服务器默认值
	viper.SetDefault("mcp.servers.qng.enabled", true)
	viper.SetDefault("mcp.servers.qng.protocol", "sse")
	viper.SetDefault("mcp.servers.qng.url", "http://localhost:9091/api/mcp")
	viper.SetDefault("mcp.servers.qng.timeout", 30)

	viper.SetDefault("mcp.servers.metamask.enabled", true)
	viper.SetDefault("mcp.servers.metamask.protocol", "stdio")
	viper.SetDefault("mcp.servers.metamask.timeout", 30)

	viper.SetDefault("mcp.servers.file_system.enabled", true)
	viper.SetDefault("mcp.servers.file_system.protocol", "sse")
	viper.SetDefault("mcp.servers.file_system.url", "http://localhost:8084/api/mcp")
	viper.SetDefault("mcp.servers.file_system.timeout", 30)

	// 智能体默认值
	viper.SetDefault("agent.name", "QNG Agent")
	viper.SetDefault("agent.version", "1.0.0")
	viper.SetDefault("agent.workflow.timeout", 300)
	viper.SetDefault("agent.workflow.max_retries", 3)
	viper.SetDefault("agent.workflow.retry_delay", 5)
	viper.SetDefault("agent.polling.interval", 2)
	viper.SetDefault("agent.polling.timeout", 30)
	viper.SetDefault("agent.polling.max_attempts", 15)

	// 前端默认值
	viper.SetDefault("frontend.enabled", true)
	viper.SetDefault("frontend.host", "localhost")
	viper.SetDefault("frontend.port", 3000)
	viper.SetDefault("frontend.api.base_url", "http://localhost:8080/api")
	viper.SetDefault("frontend.api.timeout", 30)
	viper.SetDefault("frontend.websocket.enabled", true)
	viper.SetDefault("frontend.websocket.url", "ws://localhost:8080/ws")

	// 数据库默认值
	viper.SetDefault("database.driver", "sqlite")
	viper.SetDefault("database.sqlite.path", "data/qng_agent.db")

	// 缓存默认值
	viper.SetDefault("cache.driver", "memory")

	// 安全默认值
	viper.SetDefault("security.jwt_expiry", "24h")
	viper.SetDefault("security.cors.enabled", true)

	// 监控默认值
	viper.SetDefault("monitoring.enabled", true)
	viper.SetDefault("monitoring.metrics.enabled", true)
	viper.SetDefault("monitoring.metrics.port", 9090)
	viper.SetDefault("monitoring.health_check.enabled", true)
	viper.SetDefault("monitoring.health_check.port", 8080)
	viper.SetDefault("monitoring.health_check.path", "/health")

	// 开发默认值
	viper.SetDefault("development.hot_reload", true)
	viper.SetDefault("development.debug", true)
	viper.SetDefault("development.cors", true)
}
