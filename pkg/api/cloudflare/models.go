package cloudflare

import (
	"time"

	"github.com/google/uuid"
)

// ProviderTunnel represents the database model for a Cloudflare tunnel associated with a provider.
type ProviderTunnel struct {
	ProviderID      uuid.UUID `json:"provider_id" db:"provider_id"`
	TunnelID        string    `json:"tunnel_id" db:"tunnel_id"`
	TunnelName      string    `json:"tunnel_name" db:"tunnel_name"`
	Hostname        string    `json:"hostname" db:"hostname"`
	ServiceURL      string    `json:"service_url" db:"service_url"`
	LastTokenIssued time.Time `json:"last_token_issued" db:"last_token_issued"`
	CreatedAt       time.Time `json:"created_at" db:"created_at"`
	UpdatedAt       time.Time `json:"updated_at" db:"updated_at"`
}

// RequestTunnelRequest defines the structure for requesting a new tunnel.
// It now expects the API key to be passed via Authorization header, so APIKey field is removed.
type RequestTunnelRequest struct {
	ServiceURL string `json:"service_url" validate:"required,url"`
}

// RequestTunnelResponse defines the structure for the response of a tunnel request.
type RequestTunnelResponse struct {
	Token    string `json:"token"`
	Hostname string `json:"hostname"`
}

// RefreshTokenRequest defines the structure for refreshing a tunnel token.
// It now expects the API key to be passed via Authorization header, so APIKey field is removed.
// The Refresh field was a placeholder and can be removed if not used for other logic.
type RefreshTokenRequest struct {
	Refresh bool `json:"refresh"` // Kept for now, can be removed if truly unused.
}

// RefreshTokenResponse defines the structure for the response of a refresh token request.
type RefreshTokenResponse struct {
	Token string `json:"token"`
}

// CleanupTunnelRequest represents a request to cleanup/delete inactive tunnels
type CleanupTunnelRequest struct {
	Days int `json:"days,omitempty" validate:"omitempty,min=1,max=365"` // Optional, defaults to 30 days, max 1 year
}

// CleanupTunnelResponse represents the response from tunnel cleanup
type CleanupTunnelResponse struct {
	TotalChecked   int      `json:"total_checked"`
	TotalDeleted   int      `json:"total_deleted"`
	DeletedTunnels []string `json:"deleted_tunnels"`
	ActiveTunnels  []string `json:"active_tunnels"`
	FailedCleanups []string `json:"failed_cleanups,omitempty"`
	Message        string   `json:"message"`
}
