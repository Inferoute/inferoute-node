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
			`INSERT INTO users (id, type, username, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5)`,
			user.ID, user.Type, user.Username, user.CreatedAt, user.UpdatedAt,
		)
		if err != nil {
			return fmt.Errorf("error creating user: %v", err)
		}

		response.User = user

		// Create provider or consumer based on user type
		if user.Type == "provider" {
			provider := Provider{
				ID:          uuid.New(),
				UserID:      user.ID,
				Name:        req.Name,
				IsAvailable: false,
				APIURL:      req.APIURL,
				CreatedAt:   time.Now(),
				UpdatedAt:   time.Now(),
			}

			_, err = tx.ExecContext(ctx,
				`INSERT INTO providers (id, user_id, name, is_available, api_url, created_at, updated_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7)`,
				provider.ID, provider.UserID, provider.Name, provider.IsAvailable,
				provider.APIURL, provider.CreatedAt, provider.UpdatedAt,
			)
			if err != nil {
				return fmt.Errorf("error creating provider: %v", err)
			}

			response.Provider = &provider

			// Generate API key for provider
			apiKey := APIKey{
				ID:         uuid.New(),
				ProviderID: &provider.ID,
				APIKey:     generateAPIKey(),
				IsActive:   true,
				CreatedAt:  time.Now(),
				UpdatedAt:  time.Now(),
			}

			_, err = tx.ExecContext(ctx,
				`INSERT INTO api_keys (id, provider_id, api_key, is_active, created_at, updated_at)
				VALUES ($1, $2, $3, $4, $5, $6)`,
				apiKey.ID, apiKey.ProviderID, apiKey.APIKey, apiKey.IsActive,
				apiKey.CreatedAt, apiKey.UpdatedAt,
			)
			if err != nil {
				return fmt.Errorf("error creating API key: %v", err)
			}

			response.APIKey = apiKey.APIKey

			// Create balance for provider
			balance := Balance{
				ID:              uuid.New(),
				ProviderID:      &provider.ID,
				AvailableAmount: req.Balance,
				HeldAmount:      0,
				CreatedAt:       time.Now(),
				UpdatedAt:       time.Now(),
			}

			_, err = tx.ExecContext(ctx,
				`INSERT INTO balances (id, provider_id, available_amount, held_amount, created_at, updated_at)
				VALUES ($1, $2, $3, $4, $5, $6)`,
				balance.ID, balance.ProviderID, balance.AvailableAmount,
				balance.HeldAmount, balance.CreatedAt, balance.UpdatedAt,
			)
			if err != nil {
				return fmt.Errorf("error creating balance: %v", err)
			}

			response.Balance = balance

		} else {
			consumer := Consumer{
				ID:                   uuid.New(),
				UserID:               user.ID,
				Name:                 req.Name,
				MaxInputPriceTokens:  1.0, // Default values
				MaxOutputPriceTokens: 1.0,
				CreatedAt:            time.Now(),
				UpdatedAt:            time.Now(),
			}

			_, err = tx.ExecContext(ctx,
				`INSERT INTO consumers (id, user_id, name, max_input_price_tokens, max_output_price_tokens, created_at, updated_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7)`,
				consumer.ID, consumer.UserID, consumer.Name,
				consumer.MaxInputPriceTokens, consumer.MaxOutputPriceTokens,
				consumer.CreatedAt, consumer.UpdatedAt,
			)
			if err != nil {
				return fmt.Errorf("error creating consumer: %v", err)
			}

			response.Consumer = &consumer

			// Generate API key for consumer
			apiKey := APIKey{
				ID:         uuid.New(),
				ConsumerID: &consumer.ID,
				APIKey:     generateAPIKey(),
				IsActive:   true,
				CreatedAt:  time.Now(),
				UpdatedAt:  time.Now(),
			}

			_, err = tx.ExecContext(ctx,
				`INSERT INTO api_keys (id, consumer_id, api_key, is_active, created_at, updated_at)
				VALUES ($1, $2, $3, $4, $5, $6)`,
				apiKey.ID, apiKey.ConsumerID, apiKey.APIKey, apiKey.IsActive,
				apiKey.CreatedAt, apiKey.UpdatedAt,
			)
			if err != nil {
				return fmt.Errorf("error creating API key: %v", err)
			}

			response.APIKey = apiKey.APIKey

			// Create balance for consumer
			balance := Balance{
				ID:              uuid.New(),
				ConsumerID:      &consumer.ID,
				AvailableAmount: req.Balance,
				HeldAmount:      0,
				CreatedAt:       time.Now(),
				UpdatedAt:       time.Now(),
			}

			_, err = tx.ExecContext(ctx,
				`INSERT INTO balances (id, consumer_id, available_amount, held_amount, created_at, updated_at)
				VALUES ($1, $2, $3, $4, $5, $6)`,
				balance.ID, balance.ConsumerID, balance.AvailableAmount,
				balance.HeldAmount, balance.CreatedAt, balance.UpdatedAt,
			)
			if err != nil {
				return fmt.Errorf("error creating balance: %v", err)
			}

			response.Balance = balance
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
		// First try to find the API key in the api_keys table
		var apiKey APIKey
		err := tx.QueryRowContext(ctx,
			`SELECT id, provider_id, consumer_id, is_active 
			FROM api_keys 
			WHERE api_key = $1`,
			req.APIKey,
		).Scan(&apiKey.ID, &apiKey.ProviderID, &apiKey.ConsumerID, &apiKey.IsActive)

		if err == sql.ErrNoRows {
			response = ValidateAPIKeyResponse{Valid: false}
			return nil
		}
		if err != nil {
			return fmt.Errorf("error querying API key: %v", err)
		}

		// API key exists but is not active
		if !apiKey.IsActive {
			response = ValidateAPIKeyResponse{Valid: false}
			return nil
		}

		// If it's a provider API key
		if apiKey.ProviderID != nil {
			var provider Provider
			var userID uuid.UUID
			var userType string
			var availableBalance, heldBalance float64

			err = tx.QueryRowContext(ctx,
				`SELECT p.id, p.user_id, u.type, COALESCE(b.available_amount, 0), COALESCE(b.held_amount, 0)
				FROM providers p
				JOIN users u ON u.id = p.user_id
				LEFT JOIN balances b ON b.provider_id = p.id
				WHERE p.id = $1`,
				apiKey.ProviderID,
			).Scan(&provider.ID, &userID, &userType, &availableBalance, &heldBalance)

			if err == sql.ErrNoRows {
				response = ValidateAPIKeyResponse{Valid: false}
				return nil
			}
			if err != nil {
				return fmt.Errorf("error querying provider: %v", err)
			}

			response = ValidateAPIKeyResponse{
				Valid:            true,
				UserID:           userID,
				ProviderID:       &provider.ID,
				UserType:         userType,
				AvailableBalance: availableBalance,
				HeldBalance:      heldBalance,
			}
			return nil
		}

		// If it's a consumer API key
		if apiKey.ConsumerID != nil {
			var consumer Consumer
			var userID uuid.UUID
			var userType string
			var availableBalance, heldBalance float64

			err = tx.QueryRowContext(ctx,
				`SELECT c.id, c.user_id, u.type, COALESCE(b.available_amount, 0), COALESCE(b.held_amount, 0)
				FROM consumers c
				JOIN users u ON u.id = c.user_id
				LEFT JOIN balances b ON b.consumer_id = c.id
				WHERE c.id = $1`,
				apiKey.ConsumerID,
			).Scan(&consumer.ID, &userID, &userType, &availableBalance, &heldBalance)

			if err == sql.ErrNoRows {
				response = ValidateAPIKeyResponse{Valid: false}
				return nil
			}
			if err != nil {
				return fmt.Errorf("error querying consumer: %v", err)
			}

			response = ValidateAPIKeyResponse{
				Valid:            true,
				UserID:           userID,
				ConsumerID:       &consumer.ID,
				UserType:         userType,
				AvailableBalance: availableBalance,
				HeldBalance:      heldBalance,
			}
			return nil
		}

		// This should never happen as we have a CHECK constraint in the database
		return fmt.Errorf("API key has neither provider_id nor consumer_id")
	})

	if err != nil {
		return nil, common.ErrInternalServer(err)
	}

	return &response, nil
}

// HoldDeposit places a hold on a consumer's balance
func (s *Service) HoldDeposit(ctx context.Context, req HoldDepositRequest) (*HoldDepositResponse, error) {
	var response HoldDepositResponse

	err := s.db.ExecuteTx(ctx, func(tx *sql.Tx) error {
		var balance Balance
		err := tx.QueryRowContext(ctx,
			`SELECT id, consumer_id, available_amount, held_amount, created_at, updated_at
			FROM balances
			WHERE consumer_id = $1
			FOR UPDATE`,
			req.ConsumerID,
		).Scan(&balance.ID, &balance.ConsumerID, &balance.AvailableAmount,
			&balance.HeldAmount, &balance.CreatedAt, &balance.UpdatedAt)

		if err == sql.ErrNoRows {
			return common.ErrNotFound(fmt.Errorf("balance not found for consumer %s", req.ConsumerID))
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
			WHERE id = $4`,
			balance.AvailableAmount, balance.HeldAmount, balance.UpdatedAt, balance.ID,
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

// ReleaseHold releases a hold on a consumer's balance
func (s *Service) ReleaseHold(ctx context.Context, req ReleaseHoldRequest) (*ReleaseHoldResponse, error) {
	var response ReleaseHoldResponse

	err := s.db.ExecuteTx(ctx, func(tx *sql.Tx) error {
		var balance Balance
		err := tx.QueryRowContext(ctx,
			`SELECT id, consumer_id, available_amount, held_amount, created_at, updated_at
			FROM balances
			WHERE consumer_id = $1
			FOR UPDATE`,
			req.ConsumerID,
		).Scan(&balance.ID, &balance.ConsumerID, &balance.AvailableAmount,
			&balance.HeldAmount, &balance.CreatedAt, &balance.UpdatedAt)

		if err == sql.ErrNoRows {
			return common.ErrNotFound(fmt.Errorf("balance not found for consumer %s", req.ConsumerID))
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
			WHERE id = $4`,
			balance.AvailableAmount, balance.HeldAmount, balance.UpdatedAt, balance.ID,
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
