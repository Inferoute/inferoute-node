package provider

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
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

	// Process model name - remove ":latest" suffix if present
	modelName := req.ModelName
	if strings.HasSuffix(modelName, ":latest") {
		modelName = strings.TrimSuffix(modelName, ":latest")
	}

	err := s.db.ExecuteTx(ctx, func(tx *sql.Tx) error {
		// Verify provider exists
		var exists bool
		err := tx.QueryRowContext(ctx,
			`SELECT EXISTS(
				SELECT 1 FROM users 
				WHERE id = $1
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
			ID:                uuid.New(),
			ProviderID:        providerID,
			ModelName:         modelName,
			ServiceType:       req.ServiceType,
			InputPriceTokens:  req.InputPriceTokens,
			OutputPriceTokens: req.OutputPriceTokens,
			IsActive:          true,
			CreatedAt:         time.Now(),
			UpdatedAt:         time.Now(),
		}

		_, err = tx.ExecContext(ctx,
			`INSERT INTO provider_models (
				id, provider_id, model_name, service_type,
				input_price_tokens, output_price_tokens,
				is_active, created_at, updated_at
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
			model.ID, model.ProviderID, model.ModelName, model.ServiceType,
			model.InputPriceTokens, model.OutputPriceTokens,
			model.IsActive, model.CreatedAt, model.UpdatedAt,
		)
		return err
	})

	if err != nil {
		// Check if this is a unique constraint violation
		if strings.Contains(err.Error(), "provider_models_provider_id_model_name_key") {
			return nil, common.ErrInvalidInput(fmt.Errorf("model '%s' already exists for this provider. To update the model's configuration, use the PUT /api/provider/models/{model_id} endpoint", modelName))
		}
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
					'input_price_tokens', pm.input_price_tokens,
					'output_price_tokens', pm.output_price_tokens,
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
	// Process model name - remove ":latest" suffix if present
	modelName := req.ModelName
	if strings.HasSuffix(modelName, ":latest") {
		modelName = strings.TrimSuffix(modelName, ":latest")
	}

	query := `
		UPDATE provider_models 
		SET model_name = $1, 
			service_type = $2,
			input_price_tokens = $3, 
			output_price_tokens = $4,
			updated_at = NOW()
		WHERE id = $5 AND provider_id = $6
		RETURNING id, provider_id, model_name, service_type, input_price_tokens, output_price_tokens, is_active, created_at, updated_at`

	model := &ProviderModel{}
	err := s.db.QueryRowContext(ctx, query,
		modelName,
		req.ServiceType,
		req.InputPriceTokens,
		req.OutputPriceTokens,
		modelID,
		providerID,
	).Scan(
		&model.ID,
		&model.ProviderID,
		&model.ModelName,
		&model.ServiceType,
		&model.InputPriceTokens,
		&model.OutputPriceTokens,
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
		UPDATE providers 
		SET paused = $1,
		    updated_at = NOW()
		WHERE id = $2
		RETURNING paused`

	var response UpdatePauseResponse
	err := s.db.QueryRowContext(ctx, query, paused, providerID).Scan(&response.Paused)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, common.ErrNotFound(fmt.Errorf("provider not found"))
		}
		return nil, common.ErrInternalServer(fmt.Errorf("failed to update pause status: %w", err))
	}

	s.logger.Info("Updated provider %s pause status to %v", providerID, paused)
	return &response, nil
}

// ValidateHMAC validates an HMAC for a provider
func (s *Service) ValidateHMAC(ctx context.Context, providerID uuid.UUID, req ValidateHMACRequest) (*ValidateHMACResponse, error) {
	// Query the transaction table to validate the HMAC
	var transactionID uuid.UUID
	var modelName string
	err := s.db.QueryRowContext(ctx,
		`SELECT id, model_name
		FROM transactions 
		WHERE hmac = $1 
		AND status = 'pending'`,
		req.HMAC,
	).Scan(&transactionID, &modelName)

	if err != nil {
		if err == sql.ErrNoRows {
			return &ValidateHMACResponse{
				Valid: false,
				Error: "invalid or expired HMAC",
			}, nil
		}
		return nil, common.ErrInternalServer(fmt.Errorf("error validating HMAC: %w", err))
	}

	return &ValidateHMACResponse{
		Valid:         true,
		TransactionID: transactionID,
		RequestData: map[string]interface{}{
			"model_name": modelName,
		},
	}, nil
}

// UpdateAPIURL updates the provider's API URL
func (s *Service) UpdateAPIURL(ctx context.Context, providerID uuid.UUID, apiURL string) error {
	query := `
		UPDATE providers 
		SET api_url = $1,
			updated_at = NOW()
		WHERE id = $2`

	result, err := s.db.ExecContext(ctx, query, apiURL, providerID)
	if err != nil {
		return common.ErrInternalServer(fmt.Errorf("failed to update API URL: %w", err))
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return common.ErrInternalServer(fmt.Errorf("failed to get rows affected: %w", err))
	}

	if rowsAffected == 0 {
		return common.ErrNotFound(fmt.Errorf("provider not found"))
	}

	return nil
}

// UpdateProviderInfo updates the provider information with GPU and ngrok data
func (s *Service) UpdateProviderInfo(ctx context.Context, providerID uuid.UUID, req ProviderHealthPushRequest) error {
	query := `
		UPDATE providers 
		SET 
			api_url = CASE WHEN $2::text IS NOT NULL THEN $2 ELSE api_url END,
			product_name = CASE WHEN $3::text IS NOT NULL THEN $3 ELSE product_name END,
			driver_version = CASE WHEN $4::text IS NOT NULL THEN $4 ELSE driver_version END,
			cuda_version = CASE WHEN $5::text IS NOT NULL THEN $5 ELSE cuda_version END,
			gpu_count = CASE WHEN $6::int IS NOT NULL THEN $6 ELSE gpu_count END,
			memory_total = CASE WHEN $7::int IS NOT NULL THEN $7 ELSE memory_total END,
			memory_free = CASE WHEN $8::int IS NOT NULL THEN $8 ELSE memory_free END,
			provider_type = CASE WHEN $9::text IS NOT NULL THEN $9 ELSE provider_type END,
			updated_at = NOW()
		WHERE id = $1
	`

	var ngrokURL *string
	if req.Ngrok != nil && req.Ngrok.URL != "" {
		ngrokURL = &req.Ngrok.URL
	}

	var productName, driverVersion, cudaVersion *string
	var gpuCount, memoryTotal, memoryFree *int
	if req.GPU != nil {
		if req.GPU.ProductName != "" {
			productName = &req.GPU.ProductName
		}
		if req.GPU.DriverVersion != "" {
			driverVersion = &req.GPU.DriverVersion
		}
		if req.GPU.CudaVersion != "" {
			cudaVersion = &req.GPU.CudaVersion
		}
		if req.GPU.GPUCount > 0 {
			gpuCount = &req.GPU.GPUCount
		}
		if req.GPU.MemoryTotal > 0 {
			memoryTotal = &req.GPU.MemoryTotal
		}
		if req.GPU.MemoryFree > 0 {
			memoryFree = &req.GPU.MemoryFree
		}
	}

	var providerType *string
	if req.ProviderType != "" {
		providerType = &req.ProviderType
	}

	_, err := s.db.ExecContext(ctx, query,
		providerID,
		ngrokURL,
		productName,
		driverVersion,
		cudaVersion,
		gpuCount,
		memoryTotal,
		memoryFree,
		providerType,
	)
	if err != nil {
		return fmt.Errorf("error updating provider info: %w", err)
	}

	return nil
}
