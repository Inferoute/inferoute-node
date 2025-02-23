package auth

import (
	"time"

	"github.com/google/uuid"
)

// User represents a user in the system
type User struct {
	ID        uuid.UUID `json:"id"`
	Username  string    `json:"username"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Provider represents a provider instance
type Provider struct {
	ID              uuid.UUID `json:"id"`
	UserID          uuid.UUID `json:"user_id"`
	Name            string    `json:"name"`
	IsAvailable     bool      `json:"is_available"`
	LastHealthCheck time.Time `json:"last_health_check"`
	HealthStatus    string    `json:"health_status"`
	Tier            int       `json:"tier"`
	Paused          bool      `json:"paused"`
	APIURL          string    `json:"api_url"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// Consumer represents a consumer instance
type Consumer struct {
	ID                   uuid.UUID `json:"id"`
	UserID               uuid.UUID `json:"user_id"`
	Name                 string    `json:"name"`
	MaxInputPriceTokens  float64   `json:"max_input_price_tokens"`
	MaxOutputPriceTokens float64   `json:"max_output_price_tokens"`
	CreatedAt            time.Time `json:"created_at"`
	UpdatedAt            time.Time `json:"updated_at"`
}

// Balance represents a provider or consumer balance
type Balance struct {
	ID              uuid.UUID  `json:"id"`
	ProviderID      *uuid.UUID `json:"provider_id,omitempty"`
	ConsumerID      *uuid.UUID `json:"consumer_id,omitempty"`
	AvailableAmount float64    `json:"available_amount"`
	HeldAmount      float64    `json:"held_amount"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

// APIKey represents an API key for a provider or consumer
type APIKey struct {
	ID          uuid.UUID  `json:"id"`
	ProviderID  *uuid.UUID `json:"provider_id,omitempty"`
	ConsumerID  *uuid.UUID `json:"consumer_id,omitempty"`
	APIKey      string     `json:"api_key"`
	Description string     `json:"description"`
	IsActive    bool       `json:"is_active"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// CreateUserRequest represents a request to create a new user
type CreateUserRequest struct {
	Username string `json:"username" validate:"required"`
}

// CreateUserResponse represents the response to a create user request
type CreateUserResponse struct {
	User User `json:"user"`
}

// CreateEntityRequest represents a request to create a consumer or provider
type CreateEntityRequest struct {
	UserID uuid.UUID `json:"user_id" validate:"required"`
	Type   string    `json:"type" validate:"required,oneof=consumer provider"`
	Name   string    `json:"name" validate:"required"`
	APIURL string    `json:"api_url,omitempty"` // Only for providers
}

// CreateEntityResponse represents the response to create a consumer or provider
type CreateEntityResponse struct {
	Provider *Provider `json:"provider,omitempty"`
	Consumer *Consumer `json:"consumer,omitempty"`
}

// CreateAPIKeyRequest represents a request to create a new API key
type CreateAPIKeyRequest struct {
	UserID      uuid.UUID  `json:"user_id" validate:"required"`
	ProviderID  *uuid.UUID `json:"provider_id,omitempty"`
	ConsumerID  *uuid.UUID `json:"consumer_id,omitempty"`
	Type        string     `json:"type" validate:"required,oneof=consumer provider"`
	Description string     `json:"description" validate:"required"`
}

// CreateAPIKeyResponse represents the response to create an API key
type CreateAPIKeyResponse struct {
	ID          uuid.UUID  `json:"id"`
	APIKey      string     `json:"api_key"`
	Description string     `json:"description"`
	ProviderID  *uuid.UUID `json:"provider_id,omitempty"`
	ConsumerID  *uuid.UUID `json:"consumer_id,omitempty"`
}

// ValidateAPIKeyRequest represents a request to validate an API key
type ValidateAPIKeyRequest struct {
	APIKey string `json:"api_key" validate:"required"`
}

// ValidateAPIKeyResponse represents the response to a validate API key request
type ValidateAPIKeyResponse struct {
	Valid            bool       `json:"valid"`
	UserID           uuid.UUID  `json:"user_id,omitempty"`
	ProviderID       *uuid.UUID `json:"provider_id,omitempty"`
	ConsumerID       *uuid.UUID `json:"consumer_id,omitempty"`
	UserType         string     `json:"user_type,omitempty"`
	AvailableBalance float64    `json:"available_balance,omitempty"`
	HeldBalance      float64    `json:"held_balance,omitempty"`
}

// HoldDepositRequest represents a request to place a hold on a balance
type HoldDepositRequest struct {
	ConsumerID uuid.UUID `json:"consumer_id" validate:"required"`
	Amount     float64   `json:"amount" validate:"required,min=0"`
}

// HoldDepositResponse represents the response to a hold deposit request
type HoldDepositResponse struct {
	Success bool    `json:"success"`
	Balance Balance `json:"balance"`
}

// ReleaseHoldRequest represents a request to release a hold on a balance
type ReleaseHoldRequest struct {
	ConsumerID uuid.UUID `json:"consumer_id" validate:"required"`
	Amount     float64   `json:"amount" validate:"required,min=0"`
}

// ReleaseHoldResponse represents the response to a release hold request
type ReleaseHoldResponse struct {
	Success bool    `json:"success"`
	Balance Balance `json:"balance"`
}
