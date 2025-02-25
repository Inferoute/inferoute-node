package payment

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

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
			if err := s.processPayment(&paymentMsg); err != nil {
				s.logger.Error("Failed to process payment: %v", err)
				return err
			}
			s.logger.Info("Successfully processed payment for HMAC: %s", paymentMsg.HMAC)
			return nil
		},
	)
}

// getFeePercentage gets the current service fee percentage from system_settings
func (s *Service) getFeePercentage(ctx context.Context, tx *sql.Tx) (float64, error) {
	var feeStr string
	err := tx.QueryRowContext(ctx,
		`SELECT setting_value 
		FROM system_settings 
		WHERE setting_key = 'fee_percentage'`,
	).Scan(&feeStr)

	if err == sql.ErrNoRows {
		s.logger.Info("No fee percentage found in system_settings, using default: %v", defaultServiceFeePercentage)
		return defaultServiceFeePercentage, nil
	}
	if err != nil {
		return defaultServiceFeePercentage, fmt.Errorf("error getting fee percentage: %w", err)
	}

	fee, err := strconv.ParseFloat(feeStr, 64)
	if err != nil {
		s.logger.Error("Invalid fee percentage in system_settings: %v, using default", feeStr)
		return defaultServiceFeePercentage, nil
	}

	return fee / 100.0, nil // Convert percentage to decimal
}

// checkForPricingCheating checks if a provider updated their pricing during transaction processing
func (s *Service) checkForPricingCheating(ctx context.Context, tx *sql.Tx, msg *PaymentMessage) (bool, uuid.UUID, error) {
	var providerModelID uuid.UUID
	var modelUpdatedAt time.Time
	var transactionCreatedAt, transactionUpdatedAt time.Time

	// Get the provider_model ID and its updated_at timestamp
	err := tx.QueryRowContext(ctx, `
		SELECT id, updated_at 
		FROM provider_models 
		WHERE provider_id = $1 AND model_name = $2`,
		msg.ProviderID, msg.ModelName,
	).Scan(&providerModelID, &modelUpdatedAt)
	if err != nil {
		return false, uuid.Nil, fmt.Errorf("failed to get provider model details: %w", err)
	}

	// Get transaction timestamps
	err = tx.QueryRowContext(ctx, `
		SELECT created_at, updated_at 
		FROM transactions 
		WHERE hmac = $1`,
		msg.HMAC,
	).Scan(&transactionCreatedAt, &transactionUpdatedAt)
	if err != nil {
		return false, uuid.Nil, fmt.Errorf("failed to get transaction timestamps: %w", err)
	}

	// Check if model was updated during transaction processing
	if modelUpdatedAt.After(transactionCreatedAt) && modelUpdatedAt.Before(transactionUpdatedAt) {
		// Record the cheating incident
		_, err = tx.ExecContext(ctx, `
			INSERT INTO provider_cheating_incidents (
				provider_id, provider_model_id, transaction_hmac,
				transaction_created_at, transaction_updated_at, model_updated_at,
				input_price_tokens, output_price_tokens,
				total_input_tokens, total_output_tokens
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
			msg.ProviderID, providerModelID, msg.HMAC,
			transactionCreatedAt, transactionUpdatedAt, modelUpdatedAt,
			msg.InputPriceTokens, msg.OutputPriceTokens,
			msg.TotalInputTokens, msg.TotalOutputTokens,
		)
		if err != nil {
			return true, providerModelID, fmt.Errorf("failed to record cheating incident: %w", err)
		}
		return true, providerModelID, nil
	}

	return false, providerModelID, nil
}

// processPayment handles the payment processing logic
func (s *Service) processPayment(msg *PaymentMessage) error {
	// Start a transaction
	var feePercentage float64
	var isCheating bool

	err := s.db.ExecuteTx(context.Background(), func(tx *sql.Tx) error {
		var err error
		feePercentage, err = s.getFeePercentage(context.Background(), tx)
		if err != nil {
			return fmt.Errorf("failed to get fee percentage: %w", err)
		}

		// Check for pricing cheating
		isCheating, _, err = s.checkForPricingCheating(context.Background(), tx, msg)
		if err != nil {
			return fmt.Errorf("failed to check for pricing cheating: %w", err)
		}

		return nil
	})
	if err != nil {
		return err
	}

	// Calculate tokens per second
	tokensPerSecond := float64(msg.TotalOutputTokens) / (float64(msg.Latency) / 1000.0)

	// Get user IDs for both consumer and provider
	var consumerUserID, providerUserID uuid.UUID
	err = s.db.QueryRowContext(context.Background(),
		`SELECT user_id FROM consumers WHERE id = $1`,
		msg.ConsumerID,
	).Scan(&consumerUserID)
	if err != nil {
		return fmt.Errorf("failed to get consumer user_id: %w", err)
	}

	err = s.db.QueryRowContext(context.Background(),
		`SELECT user_id FROM providers WHERE id = $1`,
		msg.ProviderID,
	).Scan(&providerUserID)
	if err != nil {
		return fmt.Errorf("failed to get provider user_id: %w", err)
	}

	var serviceFee, providerEarnings, totalCost float64

	if isCheating {
		s.logger.Warn("Provider %s attempted to change pricing during transaction %s - applying penalty", msg.ProviderID, msg.HMAC)
		// In case of cheating, charge $1 service fee and no provider earnings
		serviceFee = 1.0
		providerEarnings = 0
		totalCost = serviceFee
	} else if consumerUserID == providerUserID {
		// If same user, no fees or costs
		serviceFee = 0
		providerEarnings = 0
		totalCost = 0
		s.logger.Info("Consumer and provider belong to same user - no fees or costs applied")
	} else {
		// Calculate costs using prices from the message (per million tokens)
		inputCost := float64(msg.TotalInputTokens) * (msg.InputPriceTokens / 1_000_000.0)
		outputCost := float64(msg.TotalOutputTokens) * (msg.OutputPriceTokens / 1_000_000.0)
		totalCost = inputCost + outputCost
		serviceFee = totalCost * feePercentage
		providerEarnings = totalCost - serviceFee
	}

	// Update the provider model's average TPS and transaction count
	err = s.db.ExecuteTx(context.Background(), func(tx *sql.Tx) error {
		// Get current values
		var currentAvgTPS float64
		var currentCount int
		err := tx.QueryRow(`
			SELECT average_tps, transaction_count 
			FROM provider_models 
			WHERE provider_id = $1 AND model_name = $2`,
			msg.ProviderID, msg.ModelName,
		).Scan(&currentAvgTPS, &currentCount)
		if err != nil {
			return fmt.Errorf("failed to get current TPS stats: %w", err)
		}

		// Calculate new average using the running average formula
		newCount := currentCount + 1
		newAvgTPS := ((currentAvgTPS * float64(currentCount)) + tokensPerSecond) / float64(newCount)

		// Update the provider model
		_, err = tx.Exec(`
			UPDATE provider_models 
			SET average_tps = $1, 
				transaction_count = $2,
				updated_at = NOW()
			WHERE provider_id = $3 AND model_name = $4`,
			newAvgTPS, newCount, msg.ProviderID, msg.ModelName,
		)
		if err != nil {
			return fmt.Errorf("failed to update provider model TPS stats: %w", err)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to update provider model stats: %w", err)
	}

	// Update transaction with payment details
	query := `
		UPDATE transactions 
		SET tokens_per_second = $1,
			consumer_cost = $2,
			provider_earnings = $3,
			service_fee = $4,
			status = CASE WHEN $5 THEN 'cheating_detected' ELSE 'completed' END
		WHERE hmac = $6
	`

	if _, err := s.db.Exec(
		query,
		tokensPerSecond,
		totalCost,
		providerEarnings,
		serviceFee,
		isCheating,
		msg.HMAC,
	); err != nil {
		return fmt.Errorf("failed to update transaction: %w", err)
	}

	// Only process actual payments if not a cheating case
	if !isCheating && consumerUserID != providerUserID {
		if err := s.processConsumerDebit(msg.ConsumerID, totalCost); err != nil {
			return fmt.Errorf("failed to process consumer debit: %w", err)
		}

		if err := s.processProviderCredit(msg.ProviderID, providerEarnings); err != nil {
			return fmt.Errorf("failed to process provider credit: %w", err)
		}
	}

	return nil
}

// processConsumerDebit handles the debiting of funds from the consumer's balance
func (s *Service) processConsumerDebit(consumerID uuid.UUID, amount float64) error {
	// First get the user_id for this consumer
	var userID uuid.UUID
	err := s.db.QueryRowContext(context.Background(),
		`SELECT user_id FROM consumers WHERE id = $1`,
		consumerID,
	).Scan(&userID)
	if err != nil {
		return fmt.Errorf("error getting user_id for consumer: %w", err)
	}

	// Now update the user's balance
	_, err = s.db.Exec(
		`UPDATE balances 
		SET available_amount = available_amount - $1,
			updated_at = NOW()
		WHERE user_id = $2`,
		amount,
		userID,
	)
	return err
}

// processProviderCredit handles the crediting of funds to the provider's balance
func (s *Service) processProviderCredit(providerID uuid.UUID, amount float64) error {
	// First get the user_id for this provider
	var userID uuid.UUID
	err := s.db.QueryRowContext(context.Background(),
		`SELECT user_id FROM providers WHERE id = $1`,
		providerID,
	).Scan(&userID)
	if err != nil {
		return fmt.Errorf("error getting user_id for provider: %w", err)
	}

	// Now update the user's balance
	_, err = s.db.Exec(
		`UPDATE balances 
		SET available_amount = available_amount + $1,
			updated_at = NOW()
		WHERE user_id = $2`,
		amount,
		userID,
	)
	return err
}
