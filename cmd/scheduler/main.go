package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/sentnl/inferoute-node/internal/config"
	"github.com/sentnl/inferoute-node/pkg/api/scheduler"
	"github.com/sentnl/inferoute-node/pkg/common"
)

func main() {
	// Load config
	cfg, err := config.LoadConfig(".env")
	if err != nil {
		fmt.Printf("Error loading config: %v\n", err)
		os.Exit(1)
	}

	// Initialize logger
	logger := common.NewLogger("scheduler")

	// Create scheduler service
	logger.Info("Scheduler configured with internal key: %s", cfg.InternalAPIKey)
	service := scheduler.NewService(cfg.InternalAPIKey, logger)

	// Create context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start the scheduler
	if err := service.Start(ctx); err != nil {
		logger.Error("Failed to start scheduler: %v", err)
		os.Exit(1)
	}

	logger.Info("Scheduler service started successfully")

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	// Graceful shutdown
	logger.Info("Shutting down scheduler service...")
	service.Stop()

	logger.Info("Scheduler service stopped")
}
