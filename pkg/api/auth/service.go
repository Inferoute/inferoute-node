package auth

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/sentnl/inferoute-node/internal/db"
	"github.com/sentnl/inferoute-node/pkg/common"
)

// Service handles authentication business logic
type Service struct {
	db     *db.DB
	logger *common.Logger
}

// NewService creates a new authentication service
func NewService(db *db.DB, logger *common.Logger) *Service {
	return &Service{
		db:     db,
		logger: logger,
	}
}

// CreateUser creates a new user with an API key and initial balance
func (s *Service) CreateUser(ctx context.Context, req CreateUserRequest) (*CreateUserResponse, error) {
	var response CreateUserResponse

	err := s.db.ExecuteTx(ctx, func(tx *sql.Tx) error {
		// Create user
		user := User{
			ID:        uuid.New(),
			Type:      req.Type,
			Username:  req.Username,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}

		_, err := tx.ExecContext(ctx,
			`INSERT INTO public.users (id, type, username, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5)`,
			user.ID, user.Type, user.Username, user.CreatedAt, user.UpdatedAt,
		)
		if err != nil {
			return fmt.Errorf("error creating user: %v", err)
		}

		// Generate API key
		apiKey := APIKey{
			ID:        uuid.New(),
			UserID:    user.ID,
			APIKey:    generateAPIKey(),
			IsActive:  true,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}

		_, err = tx.ExecContext(ctx,
			`INSERT INTO api_keys (id, user_id, api_key, is_active, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6)`,
			apiKey.ID, apiKey.UserID, apiKey.APIKey, apiKey.IsActive, apiKey.CreatedAt, apiKey.UpdatedAt,
		)
		if err != nil {
			return fmt.Errorf("error creating API key: %v", err)
		}

		// Create initial balance
		balance := Balance{
			UserID:          user.ID,
			AvailableAmount: req.Balance,
			HeldAmount:      0,
			CreatedAt:       time.Now(),
			UpdatedAt:       time.Now(),
		}

		_, err = tx.ExecContext(ctx,
			`INSERT INTO balances (user_id, available_amount, held_amount, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5)`,
			balance.UserID, balance.AvailableAmount, balance.HeldAmount, balance.CreatedAt, balance.UpdatedAt,
		)
		if err != nil {
			return fmt.Errorf("error creating balance: %v", err)
		}

		response = CreateUserResponse{
			User:    user,
			APIKey:  apiKey.APIKey,
			Balance: balance,
		}

		return nil
	})

	if err != nil {
		return nil, common.ErrInternalServer(err)
	}

	return &response, nil
}

// ValidateAPIKey validates an API key and returns user information
func (s *Service) ValidateAPIKey(ctx context.Context, req ValidateAPIKeyRequest) (*ValidateAPIKeyResponse, error) {
	var response ValidateAPIKeyResponse

	err := s.db.ExecuteTx(ctx, func(tx *sql.Tx) error {
		var userID uuid.UUID
		var userType string
		var isActive bool

		err := tx.QueryRowContext(ctx,
			`SELECT ak.user_id, u.type, ak.is_active
			FROM api_keys ak
			JOIN users u ON u.id = ak.user_id
			WHERE ak.api_key = $1`,
			req.APIKey,
		).Scan(&userID, &userType, &isActive)

		if err == sql.ErrNoRows {
			response = ValidateAPIKeyResponse{Valid: false}
			return nil
		}
		if err != nil {
			return fmt.Errorf("error validating API key: %v", err)
		}

		response = ValidateAPIKeyResponse{
			Valid:    isActive,
			UserID:   userID,
			UserType: userType,
		}

		return nil
	})

	if err != nil {
		return nil, common.ErrInternalServer(err)
	}

	return &response, nil
}

// HoldDeposit places a hold on a user's balance
func (s *Service) HoldDeposit(ctx context.Context, req HoldDepositRequest) (*HoldDepositResponse, error) {
	var response HoldDepositResponse

	err := s.db.ExecuteTx(ctx, func(tx *sql.Tx) error {
		var balance Balance
		err := tx.QueryRowContext(ctx,
			`SELECT user_id, available_amount, held_amount, created_at, updated_at
			FROM balances
			WHERE user_id = $1
			FOR UPDATE`,
			req.UserID,
		).Scan(&balance.UserID, &balance.AvailableAmount, &balance.HeldAmount,
			&balance.CreatedAt, &balance.UpdatedAt)

		if err == sql.ErrNoRows {
			return common.ErrNotFound(fmt.Errorf("balance not found for user %s", req.UserID))
		}
		if err != nil {
			return fmt.Errorf("error fetching balance: %v", err)
		}

		if balance.AvailableAmount < req.Amount {
			return common.ErrInsufficientFunds(fmt.Errorf("insufficient funds"))
		}

		balance.AvailableAmount -= req.Amount
		balance.HeldAmount += req.Amount
		balance.UpdatedAt = time.Now()

		_, err = tx.ExecContext(ctx,
			`UPDATE balances
			SET available_amount = $1, held_amount = $2, updated_at = $3
			WHERE user_id = $4`,
			balance.AvailableAmount, balance.HeldAmount, balance.UpdatedAt, balance.UserID,
		)
		if err != nil {
			return fmt.Errorf("error updating balance: %v", err)
		}

		response = HoldDepositResponse{
			Success: true,
			Balance: balance,
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return &response, nil
}

// ReleaseHold releases a hold on a user's balance
func (s *Service) ReleaseHold(ctx context.Context, req ReleaseHoldRequest) (*ReleaseHoldResponse, error) {
	var response ReleaseHoldResponse

	err := s.db.ExecuteTx(ctx, func(tx *sql.Tx) error {
		var balance Balance
		err := tx.QueryRowContext(ctx,
			`SELECT user_id, available_amount, held_amount, created_at, updated_at
			FROM balances
			WHERE user_id = $1
			FOR UPDATE`,
			req.UserID,
		).Scan(&balance.UserID, &balance.AvailableAmount, &balance.HeldAmount,
			&balance.CreatedAt, &balance.UpdatedAt)

		if err == sql.ErrNoRows {
			return common.ErrNotFound(fmt.Errorf("balance not found for user %s", req.UserID))
		}
		if err != nil {
			return fmt.Errorf("error fetching balance: %v", err)
		}

		if balance.HeldAmount < req.Amount {
			return common.ErrInvalidInput(fmt.Errorf("held amount is less than release amount"))
		}

		balance.AvailableAmount += req.Amount
		balance.HeldAmount -= req.Amount
		balance.UpdatedAt = time.Now()

		_, err = tx.ExecContext(ctx,
			`UPDATE balances
			SET available_amount = $1, held_amount = $2, updated_at = $3
			WHERE user_id = $4`,
			balance.AvailableAmount, balance.HeldAmount, balance.UpdatedAt, balance.UserID,
		)
		if err != nil {
			return fmt.Errorf("error updating balance: %v", err)
		}

		response = ReleaseHoldResponse{
			Success: true,
			Balance: balance,
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return &response, nil
}

// generateAPIKey generates a random API key in OpenAI format (sk-...)
func generateAPIKey() string {
	b := make([]byte, 24) // 32 bytes of base64 is too long, OpenAI uses shorter keys
	_, err := rand.Read(b)
	if err != nil {
		panic(err) // This should never happen
	}
	// Use RawURLEncoding to avoid / and + characters
	return fmt.Sprintf("sk-%s", base64.RawURLEncoding.EncodeToString(b))
}
