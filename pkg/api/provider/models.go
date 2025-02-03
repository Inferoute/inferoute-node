package provider

import (
	"time"

	"github.com/google/uuid"
)

// ServiceType represents the type of service a provider uses
type ServiceType string

const (
	ServiceTypeOllama   ServiceType = "ollama"
	ServiceTypeExolabs  ServiceType = "exolabs"
	ServiceTypeLlamaCPP ServiceType = "llama_cpp"
)

// ProviderModel represents a model configuration for a provider
type ProviderModel struct {
	ID                  uuid.UUID   `json:"id"`
	ProviderID          uuid.UUID   `json:"provider_id"`
	ModelName           string      `json:"model_name"`
	ServiceType         ServiceType `json:"service_type"`
	InputPricePerToken  float64     `json:"input_price_per_token"`
	OutputPricePerToken float64     `json:"output_price_per_token"`
	IsActive            bool        `json:"is_active"`
	CreatedAt           time.Time   `json:"created_at"`
	UpdatedAt           time.Time   `json:"updated_at"`
}

// ProviderStatus represents the current status of a provider
type ProviderStatus struct {
	ProviderID        uuid.UUID `json:"provider_id"`
	IsAvailable       bool      `json:"is_available"`
	LastHealthCheck   time.Time `json:"last_health_check"`
	HealthCheckStatus bool      `json:"health_check_status"`
	LatencyMs         int       `json:"latency_ms"`
	SuccessRate       float64   `json:"success_rate"`
	IsHealthy         bool      `json:"is_healthy"`
	LastError         string    `json:"last_error,omitempty"`
	Paused            bool      `json:"paused"`
}

// AddModelRequest represents a request to add a new model for a provider
type AddModelRequest struct {
	ModelName           string      `json:"model_name" validate:"required"`
	ServiceType         ServiceType `json:"service_type" validate:"required,oneof=ollama exolabs llama_cpp"`
	InputPricePerToken  float64     `json:"input_price_per_token" validate:"required,min=0"`
	OutputPricePerToken float64     `json:"output_price_per_token" validate:"required,min=0"`
}

// UpdateModelRequest represents the request to update a model
type UpdateModelRequest struct {
	ModelName           string  `json:"model_name" validate:"required"`
	ServiceType         string  `json:"service_type" validate:"required,oneof=ollama exolabs llama_cpp"`
	InputPricePerToken  float64 `json:"input_price_per_token" validate:"required,gte=0"`
	OutputPricePerToken float64 `json:"output_price_per_token" validate:"required,gte=0"`
}

// ListModelsResponse represents the response for listing provider models
type ListModelsResponse struct {
	Username string          `json:"username"`
	Models   []ProviderModel `json:"models"`
}

// GetStatusResponse represents the provider's current status
type GetStatusResponse struct {
	IsAvailable       bool      `json:"is_available"`
	Paused            bool      `json:"paused"`
	HealthCheckStatus bool      `json:"health_check_status"`
	LastHealthCheck   time.Time `json:"last_health_check"`
}

// UpdatePauseRequest represents the request to update a provider's pause status
type UpdatePauseRequest struct {
	Paused bool `json:"paused" validate:"required"`
}

// UpdatePauseResponse represents the response after updating a provider's pause status
type UpdatePauseResponse struct {
	ProviderID uuid.UUID `json:"provider_id"`
	Paused     bool      `json:"paused"`
}

// ProviderHealthPushModel represents a model in the health push data
type ProviderHealthPushModel struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	OwnedBy string `json:"owned_by"`
}

// ProviderHealthPushRequest represents the request body for health push
type ProviderHealthPushRequest struct {
	Object string                    `json:"object" validate:"required,eq=list"`
	Data   []ProviderHealthPushModel `json:"data" validate:"required,dive"`
}

// ProviderHealthMessage represents the message that will be sent to RabbitMQ
type ProviderHealthMessage struct {
	APIKey string                    `json:"api_key"`
	Models []ProviderHealthPushModel `json:"models"`
}
