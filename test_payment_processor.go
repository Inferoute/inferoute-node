package main

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/sentnl/inferoute-node/internal/config"
	"github.com/sentnl/inferoute-node/pkg/api/payment"
	"github.com/sentnl/inferoute-node/pkg/common"
	"github.com/sentnl/inferoute-node/pkg/rabbitmq"
)

// TestMessage represents a test message configuration
type TestMessage struct {
	ConsumerID uuid.UUID
	ProviderID uuid.UUID
	ModelName  string
	InputCost  float64
	OutputCost float64
}

// getTestMessages returns a slice of test messages to process
func getTestMessages() []TestMessage {
	return []TestMessage{
		{
			ConsumerID: uuid.MustParse("cccccccc-cccc-cccc-cccc-cccccccccccc"), // Enterprise consumer
			ProviderID: uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"), // Tier 1 provider
			ModelName:  "deepseek-r1:8b",
			InputCost:  0.15,
			OutputCost: 0.3,
		},
		{
			ConsumerID: uuid.MustParse("dddddddd-dddd-dddd-dddd-dddddddddddd"), // Business consumer
			ProviderID: uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"), // Tier 2 provider
			ModelName:  "claude-2",
			InputCost:  0.6,
			OutputCost: 0.18,
		},
		{
			ConsumerID: uuid.MustParse("77777777-7777-7777-7777-777777777777"), // Startup consumer
			ProviderID: uuid.MustParse("cccccccc-cccc-cccc-cccc-cccccccccccc"), // Tier 2 provider
			ModelName:  "mistral-small",
			InputCost:  0.3,
			OutputCost: 0.9,
		},
	}
}

func main() {
	// Load config
	cfg, err := config.LoadConfig(".env")
	if err != nil {
		fmt.Printf("Error loading config: %v\n", err)
		os.Exit(1)
	}

	// Initialize logger
	logger := common.NewLogger("payment-test")

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

	// Get test messages
	testMessages := getTestMessages()

	// Process each test message
	var count int
	startTime := time.Now()
	rand.Seed(time.Now().UnixNano())

	for i := 0; i < 100; i++ { // Send 100 test messages
		// Pick a random test message configuration
		testMsg := testMessages[rand.Intn(len(testMessages))]

		// Create payment message
		msg := payment.PaymentMessage{
			ConsumerID:        testMsg.ConsumerID,
			ProviderID:        testMsg.ProviderID,
			HMAC:              fmt.Sprintf("test_hmac_%d", i),
			ModelName:         testMsg.ModelName,
			TotalInputTokens:  rand.Intn(1000) + 100,      // 100-1100 tokens
			TotalOutputTokens: rand.Intn(1500) + 200,      // 200-1700 tokens
			Latency:           int64(rand.Intn(200) + 50), // 50-250ms latency
			InputPriceTokens:  testMsg.InputCost,
			OutputPriceTokens: testMsg.OutputCost,
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

		// Add small delay between messages
		time.Sleep(100 * time.Millisecond)
	}

	duration := time.Since(startTime)
	logger.Info("Published %d messages in %v (%.2f messages/second)",
		count,
		duration,
		float64(count)/duration.Seconds())
}
