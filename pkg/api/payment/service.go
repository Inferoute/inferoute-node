package payment

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"

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
func (s *Service) processPayment(ctx context.Context, msg PaymentMessage) error {
	return s.db.ExecuteTx(ctx, func(tx *sql.Tx) error {
		// Get current fee percentage
		feePercentage, err := s.getFeePercentage(ctx, tx)
		if err != nil {
			s.logger.Error("Error getting fee percentage: %v", err)
			feePercentage = defaultServiceFeePercentage
		}

		// Get provider's pricing for the model
		var inputPrice, outputPrice float64
		err = tx.QueryRowContext(ctx,
			`SELECT input_price_per_token, output_price_per_token 
			FROM provider_models 
			WHERE provider_id = $1 AND model_name = $2`,
			msg.ProviderID,
			msg.ModelName,
		).Scan(&inputPrice, &outputPrice)
		if err != nil {
			return fmt.Errorf("error getting provider pricing: %w", err)
		}

		// Calculate costs
		inputCost := float64(msg.TotalInputTokens) * inputPrice
		outputCost := float64(msg.TotalOutputTokens) * outputPrice
		totalCost := inputCost + outputCost
		serviceFee := totalCost * feePercentage
		providerEarnings := totalCost - serviceFee

		// Calculate tokens per second
		tokensPerSecond := float64(msg.TotalOutputTokens) / (float64(msg.Latency) / 1000.0)

		// Update transaction record
		_, err = tx.ExecContext(ctx,
			`UPDATE transactions 
			SET tokens_per_second = $1,
				consumer_cost = $2,
				provider_earnings = $3,
				service_fee = $4,
				status = 'completed',
				updated_at = NOW()
			WHERE hmac = $5`,
			tokensPerSecond,
			totalCost,
			providerEarnings,
			serviceFee,
			msg.HMAC,
		)
		if err != nil {
			return fmt.Errorf("error updating transaction: %w", err)
		}

		// Update balances
		// Debit consumer
		_, err = tx.ExecContext(ctx,
			`UPDATE balances 
			SET available_amount = available_amount - $1,
				updated_at = NOW()
			WHERE user_id = $2`,
			totalCost,
			msg.ConsumerID,
		)
		if err != nil {
			return fmt.Errorf("error debiting consumer: %w", err)
		}

		// Credit provider
		_, err = tx.ExecContext(ctx,
			`UPDATE balances 
			SET available_amount = available_amount + $1,
				updated_at = NOW()
			WHERE user_id = $2`,
			providerEarnings,
			msg.ProviderID,
		)
		if err != nil {
			return fmt.Errorf("error crediting provider: %w", err)
		}

		return nil
	})
}
