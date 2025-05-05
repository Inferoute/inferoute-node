package orchestrator

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

// OpenAIRequest represents the incoming request from the consumer
type OpenAIRequest struct {
	Model            string             `json:"model" validate:"required"`
	Messages         []Message          `json:"messages,omitempty"`
	Prompt           string             `json:"prompt,omitempty"`
	Sort             string             `json:"sort,omitempty" validate:"omitempty,oneof=cost throughput"`
	MaxTokens        int                `json:"max_tokens,omitempty"`
	Temperature      float64            `json:"temperature,omitempty"`
	TopP             float64            `json:"top_p,omitempty"`
	N                int                `json:"n,omitempty"`
	Stream           bool               `json:"stream,omitempty"`
	Stop             []string           `json:"stop,omitempty"`
	PresencePenalty  float64            `json:"presence_penalty,omitempty"`
	FrequencyPenalty float64            `json:"frequency_penalty,omitempty"`
	LogitBias        map[string]float64 `json:"logit_bias,omitempty"`
	User             string             `json:"user,omitempty"`
}

// Message represents a chat message - only used for basic validation
type Message struct {
	Role    string      `json:"role" validate:"required,oneof=system user assistant"`
	Content interface{} `json:"content" validate:"required"` // Can be string, array, or map
}

// GetContent returns the content as a string, handling various content types
func (m *Message) GetContent() string {
	switch v := m.Content.(type) {
	case string:
		return v
	case []interface{}:
		parts := make([]string, len(v))
		for i, part := range v {
			switch p := part.(type) {
			case string:
				parts[i] = p
			case map[string]interface{}:
				if text, ok := p["text"].(string); ok {
					parts[i] = text
				} else {
					parts[i] = fmt.Sprintf("%v", p)
				}
			default:
				parts[i] = fmt.Sprintf("%v", p)
			}
		}
		return strings.Join(parts, "")
	case map[string]interface{}:
		if text, ok := v["text"].(string); ok {
			return text
		}
		return fmt.Sprintf("%v", v)
	default:
		return fmt.Sprintf("%v", m.Content)
	}
}

// UnmarshalJSON implements custom unmarshaling for Message
func (m *Message) UnmarshalJSON(data []byte) error {
	// First try to unmarshal into a temporary struct
	type TempMessage struct {
		Role    string      `json:"role"`
		Content interface{} `json:"content"`
	}
	var temp TempMessage
	if err := json.Unmarshal(data, &temp); err != nil {
		return err
	}

	// Set the role
	m.Role = temp.Role
	m.Content = temp.Content

	return nil
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

// ProviderInfo contains information about a provider
type ProviderInfo struct {
	ProviderID        uuid.UUID `json:"provider_id"`
	URL               string    `json:"url"`
	InputPriceTokens  float64   `json:"input_price_tokens"`
	OutputPriceTokens float64   `json:"output_price_tokens"`
	Tier              int       `json:"tier"`
	HealthStatus      string    `json:"health_status"`
	AverageTPS        float64   `json:"average_tps"`
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

// HoldDepositRequest represents a request to place a hold on a balance
type HoldDepositRequest struct {
	UserID uuid.UUID `json:"user_id"`
	Amount float64   `json:"amount"`
}

// ReleaseHoldRequest represents a request to release a hold on a balance
type ReleaseHoldRequest struct {
	UserID uuid.UUID `json:"user_id"`
	Amount float64   `json:"amount"`
}

// ValidateAPIKeyRequest represents a request to validate an API key
type ValidateAPIKeyRequest struct {
	APIKey string `json:"api_key"`
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
