package provider_comm

import (
	"github.com/google/uuid"
)

// SendRequestRequest represents the request to send to a provider
type SendRequestRequest struct {
	ProviderID  uuid.UUID              `json:"provider_id" validate:"required"`
	HMAC        string                 `json:"hmac" validate:"required"`
	RequestData map[string]interface{} `json:"request_data" validate:"required"`
	ModelName   string                 `json:"model_name" validate:"required"`
	ProviderURL string                 `json:"provider_url"`
}

// SendRequestResponse represents the response from sending a request to a provider
type SendRequestResponse struct {
	Success      bool                   `json:"success"`
	ResponseData map[string]interface{} `json:"response_data,omitempty"`
	Error        string                 `json:"error,omitempty"`
	Latency      int64                  `json:"latency_ms"`
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
