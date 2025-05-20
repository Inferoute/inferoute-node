package main

import (
	"fmt"
	"os"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/sentnl/inferoute-node/internal/config"
	"github.com/sentnl/inferoute-node/pkg/api/tokenizer"
	"github.com/sentnl/inferoute-node/pkg/common"
)

func main() {
	// Initialize logger
	logger := common.NewLogger("tokenizer-service")

	// Load configuration
	cfg, err := config.LoadConfig("")
	if err != nil {
		logger.Fatal("Failed to load configuration: %v", err)
	}

	// Create Echo instance
	e := echo.New()

	// Add middleware
	e.Use(middleware.Recover())
	e.Use(middleware.Logger())
	e.Use(middleware.CORS())
	e.Use(common.InternalOnly())

	// Initialize tokenizer service
	service, err := tokenizer.NewService(logger)
	if err != nil {
		logger.Fatal("Failed to initialize tokenizer service: %v", err)
	}

	// Initialize handler
	handler := tokenizer.NewHandler(service)

	// Register routes
	handler.Register(e)

	// Start server
	servicePort := cfg.ServerPort // Default to configured port
	if cfg.IsDevelopment() {
		servicePort = 8088 // Development port for Tokenizer service
	}
	addr := fmt.Sprintf("%s:%d", cfg.ServerHost, servicePort)
	logger.Info("Starting Tokenizer service on %s (env: %s)", addr, cfg.Environment)
	if err := e.Start(addr); err != nil {
		logger.Error("Failed to start server: %v", err)
		os.Exit(1)
	}
}
