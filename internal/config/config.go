package config

import (
	"fmt"
	"log"
	"github.com/spf13/viper"
)

type Config struct {
	Server   ServerConfig   `mapstructure:"server" yaml:"server"`
	Logging  LoggingConfig  `mapstructure:"logging" yaml:"logging"`
	LLM      LLMConfig      `mapstructure:"llm" yaml:"llm"`
	MCP      MCPConfig      `mapstructure:"mcp" yaml:"mcp"`
	Agent    AgentConfig    `mapstructure:"agent" yaml:"agent"`
	Frontend FrontendConfig `mapstructure:"frontend" yaml:"frontend"`
	Database DatabaseConfig `mapstructure:"database" yaml:"database"`
	Cache    CacheConfig    `mapstructure:"cache" yaml:"cache"`
	Security SecurityConfig `mapstructure:"security" yaml:"security"`
	Monitoring MonitoringConfig `mapstructure:"monitoring" yaml:"monitoring"`
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
	Provider string                 `mapstructure:"provider" yaml:"provider"`
	OpenAI   OpenAIConfig          `mapstructure:"openai" yaml:"openai"`
	Gemini   GeminiConfig          `mapstructure:"gemini" yaml:"gemini"`
	Anthropic AnthropicConfig      `mapstructure:"anthropic" yaml:"anthropic"`
}

type OpenAIConfig struct {
	APIKey   string `mapstructure:"api_key" yaml:"api_key"`
	Model    string `mapstructure:"model" yaml:"model"`
	BaseURL  string `mapstructure:"base_url" yaml:"base_url"`
	Timeout  int    `mapstructure:"timeout" yaml:"timeout"`
	MaxTokens int   `mapstructure:"max_tokens" yaml:"max_tokens"`
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

type MCPConfig struct {
	Mode     string      `mapstructure:"mode"`
	Host     string      `mapstructure:"host"`
	Port     int         `mapstructure:"port"`
	Timeout  int         `mapstructure:"timeout"`
	QNG      QNGConfig   `mapstructure:"qng"`
	MetaMask MetaMaskConfig `mapstructure:"metamask"`
}

type QNGConfig struct {
	Enabled bool        `mapstructure:"enabled"`
	Host    string      `mapstructure:"host"`
	Port    int         `mapstructure:"port"`
	Timeout int         `mapstructure:"timeout"`
	Chain   ChainConfig `mapstructure:"chain"`
}

type ChainConfig struct {
	Enabled     bool               `mapstructure:"enabled"`
	Network     string             `mapstructure:"network"`
	RPCURL      string             `mapstructure:"rpc_url"`
	Transaction TransactionConfig  `mapstructure:"transaction"`
	LangGraph   LangGraphConfig    `mapstructure:"langgraph"`
	LLM         LLMConfig          `mapstructure:"llm"`
}

type TransactionConfig struct {
	ConfirmationTimeout    int `mapstructure:"confirmation_timeout"`
	PollingInterval        int `mapstructure:"polling_interval"`
	RequiredConfirmations  int `mapstructure:"required_confirmations"`
}

type LangGraphConfig struct {
	Enabled bool     `mapstructure:"enabled"`
	Nodes   []string `mapstructure:"nodes"`
}

type MetaMaskConfig struct {
	Enabled  bool   `mapstructure:"enabled"`
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Timeout  int    `mapstructure:"timeout"`
	Network  string `mapstructure:"network"`
	ChainID  string `mapstructure:"chain_id"`
}

type AgentConfig struct {
	Name     string           `mapstructure:"name"`
	Version  string           `mapstructure:"version"`
	Workflow WorkflowConfig   `mapstructure:"workflow"`
	Polling  PollingConfig    `mapstructure:"polling"`
	LLM      LLMConfig        `mapstructure:"llm"`
	MCP      MCPConfig        `mapstructure:"mcp"`
}

type WorkflowConfig struct {
	Timeout     int `mapstructure:"timeout"`
	MaxRetries  int `mapstructure:"max_retries"`
	RetryDelay  int `mapstructure:"retry_delay"`
}

type PollingConfig struct {
	Interval     int `mapstructure:"interval"`
	Timeout      int `mapstructure:"timeout"`
	MaxAttempts  int `mapstructure:"max_attempts"`
}

type FrontendConfig struct {
	Enabled  bool         `mapstructure:"enabled"`
	Host     string       `mapstructure:"host"`
	Port     int          `mapstructure:"port"`
	BuildDir string       `mapstructure:"build_dir"`
	API      APIConfig    `mapstructure:"api"`
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
	Driver  string            `mapstructure:"driver"`
	SQLite  SQLiteConfig      `mapstructure:"sqlite"`
	Postgres PostgresConfig   `mapstructure:"postgres"`
	MySQL   MySQLConfig       `mapstructure:"mysql"`
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
	Driver string       `mapstructure:"driver"`
	Redis  RedisConfig  `mapstructure:"redis"`
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
	Enabled   bool         `mapstructure:"enabled"`
	Metrics   MetricsConfig `mapstructure:"metrics"`
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
	
	// 更新MCP配置
	if cfg.MCP.Host != "" {
		v.Set("mcp.host", cfg.MCP.Host)
	}
	if cfg.MCP.Port > 0 {
		v.Set("mcp.port", cfg.MCP.Port)
	}
	if cfg.MCP.Timeout > 0 {
		v.Set("mcp.timeout", cfg.MCP.Timeout)
	}
	
	// QNG MCP配置
	v.Set("mcp.qng.enabled", cfg.MCP.QNG.Enabled)
	if cfg.MCP.QNG.Host != "" {
		v.Set("mcp.qng.host", cfg.MCP.QNG.Host)
	}
	if cfg.MCP.QNG.Port > 0 {
		v.Set("mcp.qng.port", cfg.MCP.QNG.Port)
	}
	if cfg.MCP.QNG.Timeout > 0 {
		v.Set("mcp.qng.timeout", cfg.MCP.QNG.Timeout)
	}
	
	// MetaMask MCP配置
	v.Set("mcp.metamask.enabled", cfg.MCP.MetaMask.Enabled)
	if cfg.MCP.MetaMask.Host != "" {
		v.Set("mcp.metamask.host", cfg.MCP.MetaMask.Host)
	}
	if cfg.MCP.MetaMask.Port > 0 {
		v.Set("mcp.metamask.port", cfg.MCP.MetaMask.Port)
	}
	if cfg.MCP.MetaMask.Timeout > 0 {
		v.Set("mcp.metamask.timeout", cfg.MCP.MetaMask.Timeout)
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
	
	// MCP默认值
	viper.SetDefault("mcp.mode", "distributed")
	viper.SetDefault("mcp.host", "localhost")
	viper.SetDefault("mcp.port", 8081)
	viper.SetDefault("mcp.timeout", 30)
	
	// QNG默认值
	viper.SetDefault("mcp.qng.enabled", true)
	viper.SetDefault("mcp.qng.host", "localhost")
	viper.SetDefault("mcp.qng.port", 8082)
	viper.SetDefault("mcp.qng.timeout", 30)
	viper.SetDefault("mcp.qng.chain.enabled", true)
	viper.SetDefault("mcp.qng.chain.network", "mainnet")
	viper.SetDefault("mcp.qng.chain.langgraph.enabled", true)
	
	// MetaMask默认值
	viper.SetDefault("mcp.metamask.enabled", true)
	viper.SetDefault("mcp.metamask.network", "Ethereum Mainnet")
	viper.SetDefault("mcp.metamask.chain_id", "1")
	
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
