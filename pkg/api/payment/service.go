package payment

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"

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

// processPayment handles the payment processing logic
func (s *Service) processPayment(msg *PaymentMessage) error {
	// Start a transaction
	var feePercentage float64
	err := s.db.ExecuteTx(context.Background(), func(tx *sql.Tx) error {
		var err error
		feePercentage, err = s.getFeePercentage(context.Background(), tx)
		if err != nil {
			return fmt.Errorf("failed to get fee percentage: %w", err)
		}
		return nil
	})
	if err != nil {
		return err
	}

	// Calculate tokens per second
	tokensPerSecond := float64(msg.TotalOutputTokens) / float64(msg.Latency)

	// Calculate costs using prices from the message (per million tokens)
	inputCost := float64(msg.TotalInputTokens) * (msg.InputPriceTokens / 1_000_000.0)
	outputCost := float64(msg.TotalOutputTokens) * (msg.OutputPriceTokens / 1_000_000.0)
	totalCost := inputCost + outputCost

	// Calculate service fee using the percentage from the database
	serviceFee := totalCost * feePercentage
	providerEarnings := totalCost - serviceFee

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

	if _, err := s.db.Exec(
		query,
		tokensPerSecond,
		totalCost,
		providerEarnings,
		serviceFee,
		msg.HMAC,
	); err != nil {
		return fmt.Errorf("failed to update transaction: %w", err)
	}

	// Process the actual payments
	if err := s.processConsumerDebit(msg.ConsumerID, totalCost); err != nil {
		return fmt.Errorf("failed to process consumer debit: %w", err)
	}

	if err := s.processProviderCredit(msg.ProviderID, providerEarnings); err != nil {
		return fmt.Errorf("failed to process provider credit: %w", err)
	}

	return nil
}

// processConsumerDebit handles the debiting of funds from the consumer's balance
func (s *Service) processConsumerDebit(consumerID uuid.UUID, amount float64) error {
	_, err := s.db.Exec(
		`UPDATE balances 
		SET available_amount = available_amount - $1,
			updated_at = NOW()
		WHERE user_id = $2`,
		amount,
		consumerID,
	)
	return err
}

// processProviderCredit handles the crediting of funds to the provider's balance
func (s *Service) processProviderCredit(providerID uuid.UUID, amount float64) error {
	_, err := s.db.Exec(
		`UPDATE balances 
		SET available_amount = available_amount + $1,
			updated_at = NOW()
		WHERE user_id = $2`,
		amount,
		providerID,
	)
	return err
}
