package provider

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/sentnl/inferoute-node/internal/db"
	"github.com/sentnl/inferoute-node/pkg/common"
	"github.com/sentnl/inferoute-node/pkg/rabbitmq"
)

// Service handles provider management and health checks
type Service struct {
	db     *db.DB
	logger *common.Logger
	rmq    *rabbitmq.Client
	client *http.Client
}

// NewService creates a new provider management service
func NewService(db *db.DB, logger *common.Logger, rmq *rabbitmq.Client) *Service {
	return &Service{
		db:     db,
		logger: logger,
		rmq:    rmq,
		client: &http.Client{
			Timeout: 5 * time.Second, // 5 second timeout for health checks
		},
	}
}

// AddModel adds a new model for a provider
func (s *Service) AddModel(ctx context.Context, providerID uuid.UUID, req AddModelRequest) (*ProviderModel, error) {
	var model ProviderModel

	err := s.db.ExecuteTx(ctx, func(tx *sql.Tx) error {
		// Verify provider exists
		var exists bool
		err := tx.QueryRowContext(ctx,
			`SELECT EXISTS(
				SELECT 1 FROM users 
				WHERE id = $1 
				AND type = 'provider'
			)`,
			providerID,
		).Scan(&exists)
		if err != nil {
			return fmt.Errorf("error checking provider: %v", err)
		}
		if !exists {
			return common.ErrNotFound(fmt.Errorf("provider not found"))
		}

		// Create model
		model = ProviderModel{
			ID:                  uuid.New(),
			ProviderID:          providerID,
			ModelName:           req.ModelName,
			ServiceType:         req.ServiceType,
			InputPricePerToken:  req.InputPricePerToken,
			OutputPricePerToken: req.OutputPricePerToken,
			IsActive:            true,
			CreatedAt:           time.Now(),
			UpdatedAt:           time.Now(),
		}

		_, err = tx.ExecContext(ctx,
			`INSERT INTO provider_models (
				id, provider_id, model_name, service_type,
				input_price_per_token, output_price_per_token,
				is_active, created_at, updated_at
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
			model.ID, model.ProviderID, model.ModelName, model.ServiceType,
			model.InputPricePerToken, model.OutputPricePerToken,
			model.IsActive, model.CreatedAt, model.UpdatedAt,
		)
		return err
	})

	if err != nil {
		return nil, fmt.Errorf("error adding model: %v", err)
	}

	return &model, nil
}

// ListModels lists all models for a provider
func (s *Service) ListModels(ctx context.Context, providerID uuid.UUID) (*ListModelsResponse, error) {
	query := `
		WITH provider_info AS (
			SELECT username FROM users WHERE id = $1
		)
		SELECT 
			pi.username,
			COALESCE(json_agg(
				json_build_object(
					'id', pm.id,
					'provider_id', pm.provider_id,
					'model_name', pm.model_name,
					'service_type', pm.service_type,
					'input_price_per_token', pm.input_price_per_token,
					'output_price_per_token', pm.output_price_per_token,
					'is_active', pm.is_active,
					'created_at', pm.created_at AT TIME ZONE 'UTC',
					'updated_at', pm.updated_at AT TIME ZONE 'UTC'
				) ORDER BY pm.created_at DESC
			) FILTER (WHERE pm.id IS NOT NULL), '[]'::json) as models
		FROM provider_info pi
		LEFT JOIN provider_models pm ON pm.provider_id = $1
		GROUP BY pi.username`

	var response ListModelsResponse
	var modelsJSON []byte
	err := s.db.QueryRowContext(ctx, query, providerID).Scan(&response.Username, &modelsJSON)
	if err == sql.ErrNoRows {
		return nil, common.ErrNotFound(fmt.Errorf("provider not found"))
	}
	if err != nil {
		return nil, common.ErrInternalServer(fmt.Errorf("error listing models: %w", err))
	}

	err = json.Unmarshal(modelsJSON, &response.Models)
	if err != nil {
		return nil, common.ErrInternalServer(fmt.Errorf("error parsing models: %w", err))
	}

	return &response, nil
}

// UpdateModel updates an existing model
func (s *Service) UpdateModel(ctx context.Context, providerID, modelID uuid.UUID, req UpdateModelRequest) (*ProviderModel, error) {
	query := `
		UPDATE provider_models 
		SET model_name = $1, 
			service_type = $2,
			input_price_per_token = $3, 
			output_price_per_token = $4,
			updated_at = NOW()
		WHERE id = $5 AND provider_id = $6
		RETURNING id, provider_id, model_name, service_type, input_price_per_token, output_price_per_token, is_active, created_at, updated_at`

	model := &ProviderModel{}
	err := s.db.QueryRowContext(ctx, query,
		req.ModelName,
		req.ServiceType,
		req.InputPricePerToken,
		req.OutputPricePerToken,
		modelID,
		providerID,
	).Scan(
		&model.ID,
		&model.ProviderID,
		&model.ModelName,
		&model.ServiceType,
		&model.InputPricePerToken,
		&model.OutputPricePerToken,
		&model.IsActive,
		&model.CreatedAt,
		&model.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, common.ErrNotFound(fmt.Errorf("model not found"))
	}
	if err != nil {
		return nil, common.ErrInternalServer(fmt.Errorf("error updating model: %w", err))
	}

	return model, nil
}

// DeleteModel deletes a model
func (s *Service) DeleteModel(ctx context.Context, providerID, modelID uuid.UUID) error {
	query := `DELETE FROM provider_models WHERE id = $1 AND provider_id = $2`
	result, err := s.db.ExecContext(ctx, query, modelID, providerID)
	if err != nil {
		return common.ErrInternalServer(fmt.Errorf("error deleting model: %w", err))
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return common.ErrInternalServer(fmt.Errorf("error checking deleted rows: %w", err))
	}

	if rowsAffected == 0 {
		return common.ErrNotFound(fmt.Errorf("model not found"))
	}

	return nil
}

// GetStatus gets the current status of a provider
func (s *Service) GetStatus(ctx context.Context, providerID uuid.UUID) (*GetStatusResponse, error) {
	query := `
		SELECT 
			is_available,
			paused,
			health_check_status,
			last_health_check
		FROM provider_status
		WHERE provider_id = $1`

	status := &GetStatusResponse{}
	err := s.db.QueryRowContext(ctx, query, providerID).Scan(
		&status.IsAvailable,
		&status.Paused,
		&status.HealthCheckStatus,
		&status.LastHealthCheck,
	)

	if err == sql.ErrNoRows {
		// If no status exists yet, return default values
		return &GetStatusResponse{
			IsAvailable:       false,
			Paused:            false,
			HealthCheckStatus: false,
			LastHealthCheck:   time.Now(),
		}, nil
	}
	if err != nil {
		return nil, common.ErrInternalServer(fmt.Errorf("error getting provider status: %w", err))
	}

	return status, nil
}

// PublishHealthUpdate publishes a health update message to RabbitMQ
func (s *Service) PublishHealthUpdate(ctx context.Context, message ProviderHealthMessage) error {
	// Convert message to JSON
	messageBytes, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("error marshaling health message: %w", err)
	}

	// Publish to RabbitMQ
	err = s.rmq.Publish(
		"provider_health", // exchange
		"health_updates",  // routing key
		messageBytes,
	)
	if err != nil {
		return fmt.Errorf("error publishing to RabbitMQ: %w", err)
	}

	s.logger.Info("Published health update for provider with %d models", len(message.Models))
	return nil
}

// UpdatePauseStatus updates the pause status of a provider
func (s *Service) UpdatePauseStatus(ctx context.Context, providerID uuid.UUID, paused bool) (*UpdatePauseResponse, error) {
	query := `
		UPDATE provider_status 
		SET paused = $1
		WHERE provider_id = $2
		RETURNING provider_id, paused`

	var response UpdatePauseResponse
	err := s.db.QueryRowContext(ctx, query, paused, providerID).Scan(&response.ProviderID, &response.Paused)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, common.ErrNotFound(fmt.Errorf("provider not found"))
		}
		return nil, common.ErrInternalServer(fmt.Errorf("failed to update pause status: %w", err))
	}

	s.logger.Info("Updated provider %s pause status to %v", providerID, paused)
	return &response, nil
}
