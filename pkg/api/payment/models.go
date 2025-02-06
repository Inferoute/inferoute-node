package payment

import (
	"github.com/google/uuid"
)

// PaymentMessage represents the message received from RabbitMQ
type PaymentMessage struct {
	ConsumerID        uuid.UUID `json:"consumer_id"`
	ProviderID        uuid.UUID `json:"provider_id"`
	HMAC              string    `json:"hmac"`
	ModelName         string    `json:"model_name"`
	TotalInputTokens  int       `json:"total_input_tokens"`
	TotalOutputTokens int       `json:"total_output_tokens"`
	Latency           int64     `json:"latency"`
}

// ProcessedPayment represents the result of payment processing
type ProcessedPayment struct {
	TransactionID    uuid.UUID `json:"transaction_id"`
	ConsumerID       uuid.UUID `json:"consumer_id"`
	ProviderID       uuid.UUID `json:"provider_id"`
	TokensPerSecond  float64   `json:"tokens_per_second"`
	ConsumerCost     float64   `json:"consumer_cost"`
	ProviderEarnings float64   `json:"provider_earnings"`
	ServiceFee       float64   `json:"service_fee"`
	Status           string    `json:"status"`
}
