package health

import (
	"time"

	"github.com/google/uuid"
)

// HealthStatus represents the health status of a provider
type HealthStatus string

const (
	HealthStatusGreen  HealthStatus = "green"
	HealthStatusOrange HealthStatus = "orange"
	HealthStatusRed    HealthStatus = "red"
)

// ProviderHealthHistory represents a health check record
type ProviderHealthHistory struct {
	ID              uuid.UUID    `json:"id"`
	ProviderID      uuid.UUID    `json:"provider_id"`
	HealthStatus    HealthStatus `json:"health_status"`
	LatencyMs       int          `json:"latency_ms"`
	HealthCheckTime time.Time    `json:"health_check_time"`
	CreatedAt       time.Time    `json:"created_at"`
}

// ProviderStatus represents the current status of a provider
type ProviderStatus struct {
	ProviderID      uuid.UUID    `json:"provider_id"`
	IsAvailable     bool         `json:"is_available"`
	LastHealthCheck time.Time    `json:"last_health_check"`
	HealthStatus    HealthStatus `json:"health_status"`
	Tier            int          `json:"tier"`
	Paused          bool         `json:"paused"`
}

// ProviderHealthMessage represents the message received from RabbitMQ
type ProviderHealthMessage struct {
	APIKey string                    `json:"api_key"`
	Models []ProviderHealthPushModel `json:"models"`
}

// ProviderHealthPushModel represents a model in the health push data
type ProviderHealthPushModel struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	OwnedBy string `json:"owned_by"`
}

// GetHealthyNodesRequest represents the request for getting healthy nodes
type GetHealthyNodesRequest struct {
	ModelName string  `query:"model_name" validate:"required"`
	MaxCost   float64 `query:"max_cost" validate:"required,gt=0"`
	Tier      int     `query:"tier" validate:"required,min=1,max=3"`
}

// HealthyNode represents a healthy provider node
type HealthyNode struct {
	ProviderID          uuid.UUID `json:"provider_id"`
	Username            string    `json:"username"`
	InputPricePerToken  float64   `json:"input_price_per_token"`
	OutputPricePerToken float64   `json:"output_price_per_token"`
	Tier                int       `json:"tier"`
	HealthStatus        string    `json:"health_status"`
}

// GetHealthyNodesResponse represents the response for getting healthy nodes
type GetHealthyNodesResponse struct {
	Nodes []HealthyNode `json:"nodes"`
}

// TriggerHealthChecksResponse represents the response for triggering health checks
type TriggerHealthChecksResponse struct {
	ProvidersUpdated int `json:"providers_updated"`
	TiersUpdated     int `json:"tiers_updated"`
}
