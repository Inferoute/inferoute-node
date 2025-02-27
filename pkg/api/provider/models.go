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
	ID                uuid.UUID   `json:"id"`
	ProviderID        uuid.UUID   `json:"provider_id"`
	ModelName         string      `json:"model_name"`
	ServiceType       ServiceType `json:"service_type"`
	InputPriceTokens  float64     `json:"input_price_tokens"`
	OutputPriceTokens float64     `json:"output_price_tokens"`
	IsActive          bool        `json:"is_active"`
	CreatedAt         time.Time   `json:"created_at"`
	UpdatedAt         time.Time   `json:"updated_at"`
}

// ProviderStatus represents the current status of a provider
type ProviderStatus struct {
	ProviderID      uuid.UUID `json:"provider_id"`
	IsAvailable     bool      `json:"is_available"`
	LastHealthCheck time.Time `json:"last_health_check"`
	HealthStatus    bool      `json:"health_status"`
	IsHealthy       bool      `json:"is_healthy"`
	LastError       string    `json:"last_error,omitempty"`
	Paused          bool      `json:"paused"`
}

// AddModelRequest represents a request to add a new model for a provider
type AddModelRequest struct {
	ModelName         string      `json:"model_name" validate:"required"`
	ServiceType       ServiceType `json:"service_type" validate:"required,oneof=ollama exolabs llama_cpp"`
	InputPriceTokens  float64     `json:"input_price_tokens" validate:"required,min=0"`
	OutputPriceTokens float64     `json:"output_price_tokens" validate:"required,min=0"`
}

// UpdateModelRequest represents the request to update a model
type UpdateModelRequest struct {
	ModelName         string  `json:"model_name" validate:"required"`
	ServiceType       string  `json:"service_type" validate:"required,oneof=ollama exolabs llama_cpp"`
	InputPriceTokens  float64 `json:"input_price_tokens" validate:"required,gte=0"`
	OutputPriceTokens float64 `json:"output_price_tokens" validate:"required,gte=0"`
}

// ListModelsResponse represents the response for listing provider models
type ListModelsResponse struct {
	Username string          `json:"username"`
	Models   []ProviderModel `json:"models"`
}

// GetStatusResponse represents the provider's current status
type GetStatusResponse struct {
	IsAvailable     bool      `json:"is_available"`
	Paused          bool      `json:"paused"`
	HealthStatus    bool      `json:"health_status"`
	LastHealthCheck time.Time `json:"last_health_check"`
}

// UpdatePauseRequest represents the request to update a provider's pause status
type UpdatePauseRequest struct {
	Paused bool `json:"paused" validate:"required"`
}

// UpdatePauseResponse represents the response after updating a provider's pause status
type UpdatePauseResponse struct {
	Paused bool `json:"paused"`
}

// ProviderHealthPushModel represents a model in the health push data
type ProviderHealthPushModel struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	OwnedBy string `json:"owned_by"`
}

// GPUInfo represents GPU information in the health push data
type GPUInfo struct {
	ProductName   string `json:"product_name"`
	DriverVersion string `json:"driver_version"`
	CudaVersion   string `json:"cuda_version"`
	GPUCount      int    `json:"gpu_count"`
	UUID          string `json:"uuid"`
	Utilization   int    `json:"utilization"`
	MemoryTotal   int    `json:"memory_total"`
	MemoryUsed    int    `json:"memory_used"`
	MemoryFree    int    `json:"memory_free"`
	IsBusy        bool   `json:"is_busy"`
}

// NgrokInfo represents ngrok tunnel information in the health push data
type NgrokInfo struct {
	URL string `json:"url"`
}

// ProviderHealthPushRequest represents the request body for health push
type ProviderHealthPushRequest struct {
	Object       string                    `json:"object" validate:"required,eq=list"`
	Data         []ProviderHealthPushModel `json:"data" validate:"required,dive"`
	GPU          *GPUInfo                  `json:"gpu,omitempty"`
	Ngrok        *NgrokInfo                `json:"ngrok,omitempty"`
	ProviderType string                    `json:"provider_type,omitempty"`
}

// ProviderHealthMessage represents the message that will be sent to RabbitMQ
type ProviderHealthMessage struct {
	APIKey       string                    `json:"api_key"`
	Models       []ProviderHealthPushModel `json:"models"`
	GPU          *GPUInfo                  `json:"gpu,omitempty"`
	Ngrok        *NgrokInfo                `json:"ngrok,omitempty"`
	ProviderType string                    `json:"provider_type,omitempty"`
}

// ValidateHMACRequest represents a request to validate an HMAC
type ValidateHMACRequest struct {
	HMAC string `json:"hmac" validate:"required"`
}

// ValidateHMACResponse represents the response to an HMAC validation request
type ValidateHMACResponse struct {
	Valid         bool                   `json:"valid"`
	RequestData   map[string]interface{} `json:"request_data,omitempty"`
	Error         string                 `json:"error,omitempty"`
	TransactionID uuid.UUID              `json:"transaction_id,omitempty"`
}

// GetProviderHealthResponse represents the response for getting provider health
type GetProviderHealthResponse struct {
	ProviderID      string    `json:"provider_id"`
	Username        string    `json:"username"`
	HealthStatus    string    `json:"health_status"`
	Tier            int       `json:"tier"`
	IsAvailable     bool      `json:"is_available"`
	LatencyMs       int       `json:"latency_ms"`
	LastHealthCheck time.Time `json:"last_health_check"`
}
