package auth

import (
	"time"

	"github.com/google/uuid"
)

// User represents a user in the system
type User struct {
	ID        uuid.UUID `json:"id"`
	Type      string    `json:"type"` // "consumer" or "provider"
	Username  string    `json:"username"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Balance represents a user's balance
type Balance struct {
	UserID          uuid.UUID `json:"user_id"`
	AvailableAmount float64   `json:"available_amount"`
	HeldAmount      float64   `json:"held_amount"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// APIKey represents an API key
type APIKey struct {
	ID        uuid.UUID `json:"id"`
	UserID    uuid.UUID `json:"user_id"`
	APIKey    string    `json:"api_key"`
	IsActive  bool      `json:"is_active"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// CreateUserRequest represents a request to create a new user
type CreateUserRequest struct {
	Type     string  `json:"type" validate:"required,oneof=consumer provider"`
	Username string  `json:"username" validate:"required"`
	Balance  float64 `json:"balance" validate:"required,min=0"`
}

// CreateUserResponse represents the response to a create user request
type CreateUserResponse struct {
	User    User    `json:"user"`
	APIKey  string  `json:"api_key"`
	Balance Balance `json:"balance"`
}

// ValidateAPIKeyRequest represents a request to validate an API key
type ValidateAPIKeyRequest struct {
	APIKey string `json:"api_key" validate:"required"`
}

// ValidateAPIKeyResponse represents the response to a validate API key request
type ValidateAPIKeyResponse struct {
	Valid    bool      `json:"valid"`
	UserID   uuid.UUID `json:"user_id,omitempty"`
	UserType string    `json:"user_type,omitempty"`
}

// HoldDepositRequest represents a request to place a hold on a user's balance
type HoldDepositRequest struct {
	UserID uuid.UUID `json:"user_id" validate:"required"`
	Amount float64   `json:"amount" validate:"required,min=0"`
}

// HoldDepositResponse represents the response to a hold deposit request
type HoldDepositResponse struct {
	Success bool    `json:"success"`
	Balance Balance `json:"balance"`
}

// ReleaseHoldRequest represents a request to release a hold on a user's balance
type ReleaseHoldRequest struct {
	UserID uuid.UUID `json:"user_id" validate:"required"`
	Amount float64   `json:"amount" validate:"required,min=0"`
}

// ReleaseHoldResponse represents the response to a release hold request
type ReleaseHoldResponse struct {
	Success bool    `json:"success"`
	Balance Balance `json:"balance"`
}
