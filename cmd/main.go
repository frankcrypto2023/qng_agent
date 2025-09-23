package main

import (
	"os"
	"qng-agent/internal/config"
	"qng-agent/internal/server"

	"github.com/Qitmeer/qng/log"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Debug("main", "error", "Failed to load configuration", "details", err)
		os.Exit(1)
	}

	// Set log level from configuration
	lvl, err := log.LvlFromString(cfg.Log.Level)
	if err != nil {
		log.Debug("main", "error", "Invalid log level", "level", cfg.Log.Level, "details", err)
		os.Exit(1)
	}
	log.Glogger().Verbosity(lvl)
	// Create and start server
	srv := server.New(cfg)
	if err := srv.Start(); err != nil {
		log.Debug("main", "error", "Failed to start server", "details", err)
	}
}
