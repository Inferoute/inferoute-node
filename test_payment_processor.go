package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/sentnl/inferoute-node/internal/config"
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
	logger := common.NewLogger("payment-test")

	// Connect to database
	connStr := fmt.Sprintf(
		"postgresql://%s:%s@%s:%d/%s?sslmode=%s",
		cfg.DatabaseUser,
		cfg.DatabasePassword,
		cfg.DatabaseHost,
		cfg.DatabasePort,
		cfg.DatabaseDBName,
		cfg.DatabaseSSLMode,
	)

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		logger.Error("Failed to connect to database: %v", err)
		os.Exit(1)
	}
	defer db.Close()

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

	// Query all payment transactions
	rows, err := db.QueryContext(context.Background(),
		`SELECT 
			consumer_id, provider_id, hmac, model_name,
			total_input_tokens, total_output_tokens, latency,
			input_price_tokens, output_price_tokens
		FROM transactions 
		WHERE status = 'payment'
		ORDER BY created_at ASC`)
	if err != nil {
		logger.Error("Failed to query transactions: %v", err)
		os.Exit(1)
	}
	defer rows.Close()

	// Process each transaction
	var count int
	startTime := time.Now()

	for rows.Next() {
		var msg payment.PaymentMessage
		err := rows.Scan(
			&msg.ConsumerID,
			&msg.ProviderID,
			&msg.HMAC,
			&msg.ModelName,
			&msg.TotalInputTokens,
			&msg.TotalOutputTokens,
			&msg.Latency,
			&msg.InputPriceTokens,
			&msg.OutputPriceTokens,
		)
		if err != nil {
			logger.Error("Failed to scan row: %v", err)
			continue
		}

		// Convert message to JSON
		msgBytes, err := json.Marshal(msg)
		if err != nil {
			logger.Error("Failed to marshal message: %v", err)
			continue
		}

		// Publish to RabbitMQ
		err = rmq.Publish(
			"transactions_exchange",
			"transactions",
			msgBytes,
		)
		if err != nil {
			logger.Error("Failed to publish message: %v", err)
			continue
		}

		count++
		if count%10 == 0 {
			logger.Info("Published %d messages...", count)
		}
	}

	duration := time.Since(startTime)
	logger.Info("Published %d messages in %v (%.2f messages/second)",
		count,
		duration,
		float64(count)/duration.Seconds())
}
