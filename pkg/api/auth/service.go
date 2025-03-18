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
	"github.com/sentnl/inferoute-node/pkg/common/apikey"
)

// Config holds service configuration
type Config struct {
	InternalKey string
}

// Service handles authentication business logic
type Service struct {
	db     *db.DB
	logger *common.Logger
	config Config
}

// NewService creates a new authentication service
func NewService(db *db.DB, logger *common.Logger, config Config) *Service {
	return &Service{
		db:     db,
		logger: logger,
		config: config,
	}
}

// CreateUser creates a new user
func (s *Service) CreateUser(ctx context.Context, req CreateUserRequest) (*CreateUserResponse, error) {
	var response CreateUserResponse

	err := s.db.ExecuteTx(ctx, func(tx *sql.Tx) error {
		// Create user
		user := User{
			ID:        uuid.New(),
			Username:  req.Username,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}

		_, err := tx.ExecContext(ctx,
			`INSERT INTO users (id, username, created_at, updated_at)
			VALUES ($1, $2, $3, $4)`,
			user.ID, user.Username, user.CreatedAt, user.UpdatedAt,
		)
		if err != nil {
			return fmt.Errorf("error creating user: %v", err)
		}

		response.User = user
		return nil
	})

	if err != nil {
		return nil, common.ErrInternalServer(err)
	}

	return &response, nil
}

// ValidateAPIKey validates an API key and returns user information
func (s *Service) ValidateAPIKey(ctx context.Context, req ValidateAPIKeyRequest) (*ValidateAPIKeyResponse, error) {
	s.logger.Info("Starting API key validation for key length: %d", len(req.APIKey))

	// Generate lookup key for fast comparison
	lookupKey := apikey.GenerateLookupKey(req.APIKey)
	s.logger.Debug("Generated lookup key: %s", lookupKey)

	// Log connection pool stats
	stats := s.db.Stats()
	s.logger.Info("DB Pool Stats - Open: %d, InUse: %d, Idle: %d",
		stats.OpenConnections,
		stats.InUse,
		stats.Idle)

	rows, err := s.db.QueryContext(ctx,
		`SELECT ak.id, ak.provider_id, ak.consumer_id, ak.api_key, u.id as user_id,
		COALESCE(b.available_amount, 0) as available_balance,
		COALESCE(b.held_amount, 0) as held_balance
		FROM api_keys ak
		LEFT JOIN providers p ON p.id = ak.provider_id
		LEFT JOIN consumers c ON c.id = ak.consumer_id
		JOIN users u ON u.id = COALESCE(p.user_id, c.user_id)
		LEFT JOIN balances b ON b.user_id = u.id
		WHERE ak.is_active = true AND ak.lookup_key = $1`,
		lookupKey)
	if err != nil {
		s.logger.Error("Database query failed: %v", err)
		return nil, fmt.Errorf("error querying consumer: %w", err)
	}
	defer rows.Close()

	s.logger.Info("Successfully queried API keys table")

	for rows.Next() {
		var id uuid.UUID
		var providerID, consumerID *uuid.UUID
		var bcryptHash string
		var userID uuid.UUID
		var availableBalance, heldBalance float64

		if err := rows.Scan(&id, &providerID, &consumerID, &bcryptHash, &userID, &availableBalance, &heldBalance); err != nil {
			s.logger.Error("Error scanning row: %v", err)
			continue
		}

		s.logger.Debug("Comparing API key - Hash length: %d, Format: %s",
			len(bcryptHash),
			bcryptHash[:10]+"...")

		if apikey.CompareAPIKey(req.APIKey, bcryptHash) {
			s.logger.Info("Found matching API key")

			// Determine user type
			userType := "consumer"
			if providerID != nil {
				userType = "provider"
			}

			return &ValidateAPIKeyResponse{
				Valid:            true,
				UserID:           userID,
				ProviderID:       providerID,
				ConsumerID:       consumerID,
				UserType:         userType,
				AvailableBalance: availableBalance,
				HeldBalance:      heldBalance,
			}, nil
		}
	}

	s.logger.Info("No matching API key found")
	return &ValidateAPIKeyResponse{Valid: false}, nil
}

// HoldDeposit places a hold on a user's balance
func (s *Service) HoldDeposit(ctx context.Context, req HoldDepositRequest) (*HoldDepositResponse, error) {
	var response HoldDepositResponse

	err := s.db.ExecuteTx(ctx, func(tx *sql.Tx) error {
		var balance Balance
		err := tx.QueryRowContext(ctx,
			`SELECT id, user_id, available_amount, held_amount, created_at, updated_at
			FROM balances
			WHERE user_id = $1
			FOR UPDATE`,
			req.UserID,
		).Scan(&balance.ID, &balance.UserID, &balance.AvailableAmount,
			&balance.HeldAmount, &balance.CreatedAt, &balance.UpdatedAt)

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

// ReleaseHold releases a hold on a user's balance
func (s *Service) ReleaseHold(ctx context.Context, req ReleaseHoldRequest) (*ReleaseHoldResponse, error) {
	var response ReleaseHoldResponse

	err := s.db.ExecuteTx(ctx, func(tx *sql.Tx) error {
		var balance Balance
		err := tx.QueryRowContext(ctx,
			`SELECT id, user_id, available_amount, held_amount, created_at, updated_at
			FROM balances
			WHERE user_id = $1
			FOR UPDATE`,
			req.UserID,
		).Scan(&balance.ID, &balance.UserID, &balance.AvailableAmount,
			&balance.HeldAmount, &balance.CreatedAt, &balance.UpdatedAt)

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

// CreateEntity creates a new consumer or provider for an existing user
func (s *Service) CreateEntity(ctx context.Context, req CreateEntityRequest) (*CreateEntityResponse, error) {
	var response CreateEntityResponse

	err := s.db.ExecuteTx(ctx, func(tx *sql.Tx) error {
		// Verify user exists
		var exists bool
		err := tx.QueryRowContext(ctx,
			`SELECT EXISTS(SELECT 1 FROM users WHERE id = $1)`,
			req.UserID,
		).Scan(&exists)
		if err != nil {
			return fmt.Errorf("error checking user existence: %v", err)
		}
		if !exists {
			return common.ErrNotFound(fmt.Errorf("user not found"))
		}

		if req.Type == "provider" {
			provider := Provider{
				ID:          uuid.New(),
				UserID:      req.UserID,
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
		} else {
			consumer := Consumer{
				ID:                   uuid.New(),
				UserID:               req.UserID,
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
		}

		return nil
	})

	if err != nil {
		return nil, common.ErrInternalServer(err)
	}

	return &response, nil
}

// CreateAPIKey creates a new API key for a consumer or provider
func (s *Service) CreateAPIKey(ctx context.Context, req CreateAPIKeyRequest) (*CreateAPIKeyResponse, error) {
	plainTextKey := apikey.GenerateAPIKey()
	hashedKey := apikey.HashAPIKey(plainTextKey)

	err := s.db.ExecuteTx(ctx, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx,
			`INSERT INTO api_keys (id, provider_id, consumer_id, api_key, lookup_key, is_active, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, NOW(), NOW())`,
			uuid.New(),
			req.ProviderID,
			req.ConsumerID,
			hashedKey.BcryptHash,
			hashedKey.LookupKey,
			true,
		)
		if err != nil {
			return fmt.Errorf("error creating API key: %w", err)
		}
		return nil
	})

	if err != nil {
		return nil, common.ErrInternalServer(err)
	}

	return &CreateAPIKeyResponse{
		APIKey: plainTextKey,
	}, nil
}
