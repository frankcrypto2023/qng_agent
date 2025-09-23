package server

import (
	"fmt"
	"net/http"
	"qng-agent/internal/config"
	"qng-agent/internal/handlers"
	"qng-agent/internal/llm"
	"qng-agent/internal/middleware"
	"qng-agent/internal/session"
	"qng-agent/internal/storage"
	"time"

	"github.com/gin-gonic/gin"
)

// Server represents the HTTP server
type Server struct {
	config  *config.Config
	handler *handlers.Handler
	engine  *gin.Engine
}

// New creates a new server instance
func New(cfg *config.Config) *Server {
	// Initialize LevelDB storage
	storage, err := storage.NewLevelDBStorage(cfg.Database.Path)
	if err != nil {
		panic(fmt.Sprintf("Failed to initialize LevelDB storage: %v", err))
	}

	// Initialize session manager
	sessionManager := session.NewManager(storage)

	// Initialize LLM manager
	llmManager := llm.NewManager()
	
	// Load settings and configure LLM client (use default user for initial settings)
	settings, err := storage.LoadSettings("default")
	if err == nil && settings.LLMProvider.URL != "" && settings.LLMProvider.Token != "" {
		llmManager.UpdateClientFromConfig(settings.LLMProvider)
	}

	// Initialize handlers
	handler := handlers.NewHandler(sessionManager, llmManager, cfg, storage)

	// Setup Gin
	if cfg.Server.Host == "0.0.0.0" {
		gin.SetMode(gin.ReleaseMode)
	}

	engine := gin.Default()

	// Add CORS middleware
	engine.Use(func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")
		
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		
		c.Next()
	})

	// Add user identification middleware
	engine.Use(middleware.UserMiddleware())

	server := &Server{
		config:  cfg,
		handler: handler,
		engine:  engine,
	}

	server.setupRoutes()
	return server
}

// setupRoutes configures the API routes
func (s *Server) setupRoutes() {
	api := s.engine.Group("/api")
	
	// Health check
	api.GET("/health", s.handler.Health)
	
	// Session management
	api.POST("/sessions", s.handler.CreateSession)
	api.GET("/sessions", s.handler.GetSessions)
	api.GET("/sessions/:id", s.handler.GetSession)
	api.PUT("/sessions/:id", s.handler.UpdateSession)
	api.DELETE("/sessions/:id", s.handler.DeleteSession)
	
	// Chat streaming
	api.POST("/chat/stream", s.handler.StreamChat)
	
	// Settings
	api.GET("/settings", s.handler.GetSettings)
	api.PUT("/settings", s.handler.UpdateSettings)

	// Serve static files in development
	s.engine.Static("/static", "./frontend/dist")
	s.engine.NoRoute(func(c *gin.Context) {
		c.File("./frontend/dist/index.html")
	})
}

// Start starts the HTTP server
func (s *Server) Start() error {
	addr := fmt.Sprintf("%s:%s", s.config.Server.Host, s.config.Server.Port)
	
	// Create HTTP server with configured timeouts
	server := &http.Server{
		Addr:         addr,
		Handler:      s.engine,
		ReadTimeout:  time.Duration(s.config.Server.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(s.config.Server.WriteTimeout) * time.Second,
	}
	
	fmt.Printf("🚀 QNG Intelligent Agent server starting on %s\n", addr)
	fmt.Printf("📊 Health check: http://%s/api/health\n", addr)
	fmt.Printf("🌐 Frontend: http://%s\n", addr)
	fmt.Printf("⏱️  Server timeouts - Read: %ds, Write: %ds\n", s.config.Server.ReadTimeout, s.config.Server.WriteTimeout)
	
	return server.ListenAndServe()
}