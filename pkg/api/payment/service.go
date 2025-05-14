package payment

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/sentnl/inferoute-node/internal/db"
	"github.com/sentnl/inferoute-node/pkg/common"
	"github.com/sentnl/inferoute-node/pkg/rabbitmq"
)

const (
	defaultServiceFeePercentage = 0.05 // 5% default service fee
)

// Service handles payment processing
type Service struct {
	db     *db.DB
	logger *common.Logger
	rmq    *rabbitmq.Client
}

// NewService creates a new payment processing service
func NewService(db *db.DB, logger *common.Logger, rmq *rabbitmq.Client) *Service {
	return &Service{
		db:     db,
		logger: logger,
		rmq:    rmq,
	}
}

// StartPaymentProcessor starts consuming payment messages from RabbitMQ
func (s *Service) StartPaymentProcessor(ctx context.Context) error {
	s.logger.Info("Starting payment processor")
	return s.rmq.Consume(
		"transactions_exchange", // exchange
		"transactions",          // routing key
		"transactions_queue",    // queue name
		func(msg []byte) error {
			var paymentMsg PaymentMessage
			if err := json.Unmarshal(msg, &paymentMsg); err != nil {
				s.logger.Error("Failed to unmarshal payment message: %v", err)
				return fmt.Errorf("error unmarshaling payment message: %w", err)
			}

			s.logger.Info("Processing payment for HMAC: %s", paymentMsg.HMAC)
			if err := s.processPayment(ctx, paymentMsg); err != nil {
				s.logger.Error("Failed to process payment: %v", err)
				return err
			}
			s.logger.Info("Successfully processed payment for HMAC: %s", paymentMsg.HMAC)
			return nil
		},
	)
}

// processPayment handles the payment processing logic
func (s *Service) processPayment(ctx context.Context, msg PaymentMessage) error {
	// Start transaction
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Get fee percentage from system settings
	var feePercentage float64
	err = tx.QueryRowContext(ctx,
		`SELECT CAST(setting_value AS FLOAT) / 100.0 FROM system_settings WHERE setting_key = 'fee_percentage'`,
	).Scan(&feePercentage)
	if err != nil {
		return fmt.Errorf("failed to get fee percentage: %w", err)
	}

	// Get consumer and provider user IDs
	var consumerUserID, providerUserID uuid.UUID
	err = tx.QueryRowContext(ctx,
		`SELECT c.user_id, p.user_id 
		FROM consumers c 
		JOIN providers p ON p.id = $1
		WHERE c.id = $2`,
		msg.ProviderID, msg.ConsumerID,
	).Scan(&consumerUserID, &providerUserID)
	if err != nil {
		return fmt.Errorf("failed to get user IDs: %w", err)
	}

	// Get initial pricing for the successful provider
	var initialInputPrice, initialOutputPrice float64
	err = tx.QueryRowContext(ctx,
		`SELECT input_price_tokens, output_price_tokens 
		FROM transaction_provider_prices 
		WHERE transaction_id = (SELECT id FROM transactions WHERE hmac = $1)
		AND provider_id = $2`,
		msg.HMAC, msg.ProviderID,
	).Scan(&initialInputPrice, &initialOutputPrice)
	if err != nil {
		return fmt.Errorf("failed to get initial pricing: %w", err)
	}

	// Calculate tokens per second
	var tokensPerSecond float64
	if msg.Latency > 0 {
		totalTokens := float64(msg.TotalInputTokens + msg.TotalOutputTokens)
		tokensPerSecond = totalTokens / (float64(msg.Latency) / 1000.0) // Convert latency to seconds
	}

	var totalCost, serviceFee, providerEarnings float64

	if consumerUserID == providerUserID {
		// If same user, no fees or costs
		serviceFee = 0
		providerEarnings = 0
		totalCost = 0
		s.logger.Info("Consumer and provider belong to same user - no fees or costs applied")
	} else {
		// Calculate costs using initial prices (per million tokens)
		inputCost := float64(msg.TotalInputTokens) * (initialInputPrice / 1_000_000.0)
		outputCost := float64(msg.TotalOutputTokens) * (initialOutputPrice / 1_000_000.0)
		totalCost = inputCost + outputCost
		serviceFee = totalCost * feePercentage
		providerEarnings = totalCost - serviceFee

		// Process the actual payments
		if err := s.processConsumerDebitTx(tx, msg.ConsumerID, totalCost); err != nil {
			return fmt.Errorf("failed to process consumer debit: %w", err)
		}

		if err := s.processProviderCreditTx(tx, msg.ProviderID, providerEarnings); err != nil {
			return fmt.Errorf("failed to process provider credit: %w", err)
		}
	}

	// Update transaction with payment details
	query := `
		UPDATE transactions 
		SET tokens_per_second = $1,
			consumer_cost = $2,
			provider_earnings = $3,
			service_fee = $4,
			status = 'completed'
		WHERE hmac = $5
	`

	if _, err := tx.Exec(
		query,
		tokensPerSecond,
		totalCost,
		providerEarnings,
		serviceFee,
		msg.HMAC,
	); err != nil {
		return fmt.Errorf("failed to update transaction: %w", err)
	}

	// Update the provider model's average TPS and transaction count
	if err := s.updateProviderModelStats(tx, msg.ProviderID, msg.ModelName, tokensPerSecond); err != nil {
		return fmt.Errorf("failed to update provider model stats: %w", err)
	}

	// Commit the transaction
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// processConsumerDebitTx handles the debiting of funds from the consumer's balance within a transaction
func (s *Service) processConsumerDebitTx(tx *sql.Tx, consumerID uuid.UUID, amount float64) error {
	// First get the user_id for this consumer
	var userID uuid.UUID
	err := tx.QueryRowContext(context.Background(),
		`SELECT user_id FROM consumers WHERE id = $1`,
		consumerID,
	).Scan(&userID)
	if err != nil {
		return fmt.Errorf("error getting user_id for consumer: %w", err)
	}

	// Now update the user's balance
	_, err = tx.Exec(
		`UPDATE balances 
		SET available_amount = available_amount - $1,
			updated_at = NOW()
		WHERE user_id = $2`,
		amount,
		userID,
	)
	return err
}

// processProviderCreditTx handles the crediting of funds to the provider's balance within a transaction
func (s *Service) processProviderCreditTx(tx *sql.Tx, providerID uuid.UUID, amount float64) error {
	// First get the user_id for this provider
	var userID uuid.UUID
	err := tx.QueryRowContext(context.Background(),
		`SELECT user_id FROM providers WHERE id = $1`,
		providerID,
	).Scan(&userID)
	if err != nil {
		return fmt.Errorf("error getting user_id for provider: %w", err)
	}

	// Now update the user's balance
	_, err = tx.Exec(
		`UPDATE balances 
		SET available_amount = available_amount + $1,
			updated_at = NOW()
		WHERE user_id = $2`,
		amount,
		userID,
	)
	return err
}

// updateProviderModelStats updates the provider model's average TPS and transaction count
func (s *Service) updateProviderModelStats(tx *sql.Tx, providerID uuid.UUID, modelName string, tokensPerSecond float64) error {
	// Get current values
	var currentAvgTPS float64
	var currentCount int
	err := tx.QueryRow(`
		SELECT average_tps, transaction_count 
		FROM provider_models 
		WHERE provider_id = $1 AND model_name = $2`,
		providerID, modelName,
	).Scan(&currentAvgTPS, &currentCount)

	if err == sql.ErrNoRows {
		// Try with ":latest" suffix if exact match not found
		err = tx.QueryRow(`
			SELECT average_tps, transaction_count 
			FROM provider_models 
			WHERE provider_id = $1 AND model_name = $2`,
			providerID, modelName+":latest",
		).Scan(&currentAvgTPS, &currentCount)
	}

	if err != nil {
		return fmt.Errorf("failed to get current TPS stats: %w", err)
	}

	// Calculate new average using the running average formula
	newCount := currentCount + 1
	newAvgTPS := ((currentAvgTPS * float64(currentCount)) + tokensPerSecond) / float64(newCount)

	// Update the provider model
	result, err := tx.Exec(`
		UPDATE provider_models 
		SET average_tps = $1, 
			transaction_count = $2,
			updated_at = NOW()
		WHERE provider_id = $3 AND model_name = $4`,
		newAvgTPS, newCount, providerID, modelName,
	)

	if err != nil {
		return fmt.Errorf("failed to update provider model stats: %w", err)
	}

	// Check if any rows were affected
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	// If no rows were affected, try with ":latest" suffix
	if rowsAffected == 0 {
		_, err = tx.Exec(`
			UPDATE provider_models 
			SET average_tps = $1, 
				transaction_count = $2,
				updated_at = NOW()
			WHERE provider_id = $3 AND model_name = $4`,
			newAvgTPS, newCount, providerID, modelName+":latest",
		)
		if err != nil {
			return fmt.Errorf("failed to update provider model stats with :latest suffix: %w", err)
		}
	}

	return nil
}
