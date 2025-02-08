package orchestrator

import (
	"fmt"

	"github.com/google/uuid"
)

// OpenAIRequest represents the incoming request from the consumer
type OpenAIRequest struct {
	Model       string                 `json:"model" validate:"required"`
	Messages    []Message              `json:"messages,omitempty"`
	Prompt      string                 `json:"prompt,omitempty"`
	MaxTokens   int                    `json:"max_tokens,omitempty"`
	Temperature float64                `json:"temperature,omitempty"`
	Stream      bool                   `json:"stream,omitempty"`
	ExtraParams map[string]interface{} `json:"extra_params,omitempty"`
}

// Message represents a chat message
type Message struct {
	Role    string `json:"role" validate:"required,oneof=system user assistant"`
	Content string `json:"content" validate:"required"`
}

// ConsumerSettings represents the consumer's price settings
type ConsumerSettings struct {
	MaxInputPriceTokens  float64 `json:"max_input_price_tokens"`
	MaxOutputPriceTokens float64 `json:"max_output_price_tokens"`
}

// ConsumerModelSettings represents model-specific price settings for a consumer
type ConsumerModelSettings struct {
	ModelName            string  `json:"model_name"`
	MaxInputPriceTokens  float64 `json:"max_input_price_tokens"`
	MaxOutputPriceTokens float64 `json:"max_output_price_tokens"`
}

// ProviderInfo represents a provider's information and pricing
type ProviderInfo struct {
	ProviderID        uuid.UUID `json:"provider_id"`
	URL               string    `json:"url"`
	InputPriceTokens  float64   `json:"input_price_tokens"`
	OutputPriceTokens float64   `json:"output_price_tokens"`
	Tier              int       `json:"tier"`
	Latency           int       `json:"latency_ms"`
	HealthStatus      string    `json:"health_status"`
}

// TransactionRecord represents a transaction in the database
type TransactionRecord struct {
	ID                uuid.UUID `json:"id"`
	ConsumerID        uuid.UUID `json:"consumer_id"`
	ProviderID        uuid.UUID `json:"provider_id"`
	HMAC              string    `json:"hmac"`
	ModelName         string    `json:"model_name"`
	InputPriceTokens  float64   `json:"input_price_tokens"`
	OutputPriceTokens float64   `json:"output_price_tokens"`
	TotalInputTokens  int       `json:"total_input_tokens"`
	TotalOutputTokens int       `json:"total_output_tokens"`
	TokensPerSecond   float64   `json:"tokens_per_second"`
	Latency           int       `json:"latency"`
	ConsumerCost      float64   `json:"consumer_cost"`
	ProviderEarnings  float64   `json:"provider_earnings"`
	ServiceFee        float64   `json:"service_fee"`
	Status            string    `json:"status"`
}

// PaymentMessage represents the message to be sent to RabbitMQ
type PaymentMessage struct {
	ConsumerID        uuid.UUID `json:"consumer_id"`
	ProviderID        uuid.UUID `json:"provider_id"`
	HMAC              string    `json:"hmac"`
	ModelName         string    `json:"model_name"`
	TotalInputTokens  int       `json:"total_input_tokens"`
	TotalOutputTokens int       `json:"total_output_tokens"`
	InputPriceTokens  float64   `json:"input_price_tokens"`
	OutputPriceTokens float64   `json:"output_price_tokens"`
	Latency           int64     `json:"latency"`
}

// Validate ensures either messages or prompt is provided
func (r *OpenAIRequest) Validate() error {
	if r.Messages == nil && r.Prompt == "" {
		return fmt.Errorf("either messages or prompt must be provided")
	}
	if r.Messages != nil && r.Prompt != "" {
		return fmt.Errorf("cannot provide both messages and prompt")
	}
	return nil
}
