package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/sentnl/inferoute-node/internal/config"
	"github.com/sentnl/inferoute-node/internal/db"
	"github.com/sentnl/inferoute-node/pkg/api/payment"
	"github.com/sentnl/inferoute-node/pkg/common"
	"github.com/sentnl/inferoute-node/pkg/rabbitmq"
)

func main() {
	// Load config
	cfg, err := config.LoadConfig(".env")
	if err != nil {
		fmt.Printf("Error loading config: %v\n", err)
		os.Exit(1)
	}

	// Initialize logger
	logger := common.NewLogger("payment-processing")

	// Connect to database
	database, err := db.New(
		cfg.DatabaseHost,
		cfg.DatabasePort,
		cfg.DatabaseUser,
		cfg.DatabasePassword,
		cfg.DatabaseDBName,
		cfg.DatabaseSSLMode,
	)
	if err != nil {
		logger.Error("Failed to connect to database: %v", err)
		os.Exit(1)
	}
	defer database.Close()

	// Initialize RabbitMQ
	rmqURL := fmt.Sprintf("amqp://%s:%s@%s:%d/%s",
		cfg.RabbitMQUser,
		cfg.RabbitMQPassword,
		cfg.RabbitMQHost,
		cfg.RabbitMQPort,
		cfg.RabbitMQVHost)
	rmq, err := rabbitmq.NewClient(rmqURL)
	if err != nil {
		logger.Error("Failed to connect to RabbitMQ: %v", err)
		os.Exit(1)
	}
	defer rmq.Close()

	// Declare RabbitMQ exchange and queue
	if err := rmq.DeclareExchange("transactions_exchange", "topic"); err != nil {
		logger.Error("Failed to declare exchange: %v", err)
		os.Exit(1)
	}

	if err := rmq.DeclareQueue(
		"transactions_queue",
		"transactions_exchange",
		"transactions",
	); err != nil {
		logger.Error("Failed to declare queue: %v", err)
		os.Exit(1)
	}

	// Initialize payment service
	paymentService := payment.NewService(database, logger, rmq)

	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start payment processor
	go func() {
		logger.Info("Starting payment processor service")
		if err := paymentService.StartPaymentProcessor(ctx); err != nil {
			logger.Error("Payment processor failed: %v", err)
			os.Exit(1)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	// Graceful shutdown
	logger.Info("Shutting down payment processor service...")
	cancel()
}
