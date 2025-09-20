package config

import (
	"encoding/json"
	"os"
)

// Config represents the application configuration
type Config struct {
	Server   ServerConfig   `json:"server"`
	Database DatabaseConfig `json:"database"`
	LLM      LLMConfig      `json:"llm"`
	MCP      MCPConfig      `json:"mcp"`
}

// ServerConfig contains HTTP server configuration
type ServerConfig struct {
	Port           string   `json:"port"`
	Host           string   `json:"host"`
	AllowedOrigins []string `json:"allowed_origins"`
	ReadTimeout    int      `json:"read_timeout"`
	WriteTimeout   int      `json:"write_timeout"`
	MaxRequestSize int64    `json:"max_request_size"`
}

// DatabaseConfig contains database configuration
type DatabaseConfig struct {
	Path string `json:"path"`
	Type string `json:"type"` // "leveldb"
}

// LLMConfig contains default LLM configuration
type LLMConfig struct {
	DefaultProvider string  `json:"default_provider"`
	DefaultModel    string  `json:"default_model"`
	MaxTokens       int     `json:"max_tokens"`
	Temperature     float64 `json:"temperature"`
}

// MCPConfig contains MCP server configuration
type MCPConfig struct {
	DefaultServers []MCPServerConfig `json:"default_servers"`
}

// MCPServerConfig represents an MCP server configuration
type MCPServerConfig struct {
	Name    string `json:"name"`
	URL     string `json:"url"`
	Enabled bool   `json:"enabled"`
}

// Load loads configuration from file or environment variables
func Load() (*Config, error) {
	cfg := &Config{
		Server: ServerConfig{
			Port:           getEnv("PORT", "8081"),
			Host:           getEnv("HOST", "0.0.0.0"),
			AllowedOrigins: []string{"http://localhost:3000"},
			ReadTimeout:    30,
			WriteTimeout:   30,
			MaxRequestSize: 1024 * 1024, // 1MB
		},
		Database: DatabaseConfig{
			Path: getEnv("DB_PATH", "./data/qng_agent.leveldb"),
			Type: "leveldb",
		},
		LLM: LLMConfig{
			DefaultProvider: "openai",
			DefaultModel:    "gpt-4",
			MaxTokens:       2048,
			Temperature:     0.7,
		},
		MCP: MCPConfig{
			DefaultServers: []MCPServerConfig{
				{
					Name:    "QNG Tools",
					URL:     "http://localhost:8080/sse",
					Enabled: true,
				},
			},
		},
	}

	// Try to load from config file
	if configFile := getEnv("CONFIG_FILE", ""); configFile != "" {
		if err := loadFromFile(cfg, configFile); err != nil {
			return nil, err
		}
	}

	return cfg, nil
}

// loadFromFile loads configuration from JSON file
func loadFromFile(cfg *Config, filename string) error {
	data, err := os.ReadFile(filename)
	if err != nil {
		return err
	}

	return json.Unmarshal(data, cfg)
}

// getEnv gets environment variable with default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
