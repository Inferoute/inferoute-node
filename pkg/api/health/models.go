package health

import (
	"time"

	"github.com/google/uuid"
)

// HealthStatus represents the health status of a provider
type HealthStatus string

const (
	HealthStatusGreen HealthStatus = "green"
	HealthStatusRed   HealthStatus = "red"
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

// ProviderHealthMessage represents the message received from RabbitMQ
type ProviderHealthMessage struct {
	APIKey       string                    `json:"api_key"`
	Models       []ProviderHealthPushModel `json:"models"`
	GPU          *GPUInfo                  `json:"gpu,omitempty"`
	Ngrok        *NgrokInfo                `json:"ngrok,omitempty"`
	ProviderType string                    `json:"provider_type,omitempty"`
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
	ProviderID        uuid.UUID `json:"provider_id"`
	Username          string    `json:"username"`
	InputPriceTokens  float64   `json:"input_price_tokens"`
	OutputPriceTokens float64   `json:"output_price_tokens"`
	Tier              int       `json:"tier"`
	HealthStatus      string    `json:"health_status"`
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

// HealthyNodeResponse represents a healthy node in the response
type HealthyNodeResponse struct {
	ProviderID string `json:"provider_id"`
	Username   string `json:"username"`
	Tier       int    `json:"tier"`
	Latency    int    `json:"latency_ms"`
}

// ProviderHealthResponse represents the health status of a provider
type ProviderHealthResponse struct {
	ProviderID      string    `json:"provider_id"`
	Username        string    `json:"username"`
	HealthStatus    string    `json:"health_status"`
	Tier            int       `json:"tier"`
	IsAvailable     bool      `json:"is_available"`
	Latency         int       `json:"latency_ms"`
	LastHealthCheck time.Time `json:"last_health_check"`
}

// FilterProvidersRequest represents the query parameters for filtering providers
type FilterProvidersRequest struct {
	ModelName string  `query:"model_name"`
	Tier      *int    `query:"tier"`
	MaxCost   float64 `query:"max_cost" validate:"required,gt=0"`
}

// FilterProvidersResponse represents a provider in the filtered list
type FilterProvidersResponse struct {
	ProviderID   string  `json:"provider_id"`
	Username     string  `json:"username"`
	Tier         int     `json:"tier"`
	HealthStatus string  `json:"health_status"`
	APIURL       string  `json:"api_url"`
	ModelName    string  `json:"model_name"`
	InputCost    float64 `json:"input_cost"`
	OutputCost   float64 `json:"output_cost"`
	AverageTPS   float64 `json:"average_tps"`
}
