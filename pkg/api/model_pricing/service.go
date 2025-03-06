package model_pricing

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/lib/pq"
	"github.com/sentnl/inferoute-node/internal/db"
	"github.com/sentnl/inferoute-node/pkg/common"
)

// Service handles model pricing operations
type Service struct {
	db     *db.DB
	logger *common.Logger
}

// NewService creates a new model pricing service
func NewService(db *db.DB, logger *common.Logger) *Service {
	return &Service{
		db:     db,
		logger: logger,
	}
}

// GetModelPrices retrieves pricing information for the specified models
func (s *Service) GetModelPrices(ctx context.Context, models []string) (*GetPricesResponse, error) {
	// Get default prices first
	var defaultPricing ModelPricing
	err := s.db.QueryRowContext(ctx,
		`SELECT model_name, avg_input_price_tokens, avg_output_price_tokens, sample_size 
		FROM average_model_costs WHERE model_name = 'default'`).
		Scan(&defaultPricing.ModelName, &defaultPricing.AvgInputPrice, &defaultPricing.AvgOutputPrice, &defaultPricing.SampleSize)
	if err != nil {
		s.logger.Error("Failed to get default pricing: %v", err)
		return nil, fmt.Errorf("failed to get default pricing: %v", err)
	}

	// Query for all requested models
	rows, err := s.db.QueryContext(ctx,
		`SELECT model_name, avg_input_price_tokens, avg_output_price_tokens, sample_size 
		FROM average_model_costs 
		WHERE model_name = ANY($1)`, pq.Array(models))
	if err != nil {
		return nil, fmt.Errorf("failed to query model prices: %v", err)
	}
	defer rows.Close()

	// Create a map for found models
	foundModels := make(map[string]ModelPricing)
	for rows.Next() {
		var pricing ModelPricing
		err := rows.Scan(&pricing.ModelName, &pricing.AvgInputPrice, &pricing.AvgOutputPrice, &pricing.SampleSize)
		if err != nil {
			return nil, fmt.Errorf("failed to scan model prices: %v", err)
		}
		foundModels[pricing.ModelName] = pricing
	}

	// Build response using default pricing for missing models
	response := &GetPricesResponse{
		ModelPrices: make([]ModelPricing, len(models)),
	}

	for i, modelName := range models {
		if pricing, exists := foundModels[modelName]; exists {
			response.ModelPrices[i] = pricing
		} else {
			response.ModelPrices[i] = ModelPricing{
				ModelName:      modelName,
				AvgInputPrice:  defaultPricing.AvgInputPrice,
				AvgOutputPrice: defaultPricing.AvgOutputPrice,
				SampleSize:     0,
			}
		}
	}

	return response, nil
}

// UpdateModelCosts updates the average costs for all models and updates default pricing
func (s *Service) UpdateModelCosts(ctx context.Context) error {
	// Start a transaction since we'll be doing multiple operations
	err := s.db.ExecuteTx(ctx, func(tx *sql.Tx) error {
		// First, update individual model costs
		_, err := tx.ExecContext(ctx, `
			INSERT INTO average_model_costs (model_name, avg_input_price_tokens, avg_output_price_tokens, sample_size)
			SELECT 
				model_name,
				AVG(input_price_tokens) as avg_input_price,
				AVG(output_price_tokens) as avg_output_price,
				COUNT(*) as sample_size
			FROM provider_models
			WHERE is_active = true
			GROUP BY model_name
			ON CONFLICT (model_name) DO UPDATE
			SET 
				avg_input_price_tokens = EXCLUDED.avg_input_price_tokens,
				avg_output_price_tokens = EXCLUDED.avg_output_price_tokens,
				sample_size = EXCLUDED.sample_size,
				updated_at = CURRENT_TIMESTAMP`)
		if err != nil {
			s.logger.Error("Failed to update model costs: %v", err)
			return fmt.Errorf("failed to update model costs: %v", err)
		}

		// Then, update the default pricing based on average of all models (excluding the default entry)
		_, err = tx.ExecContext(ctx, `
			INSERT INTO average_model_costs (
				model_name, 
				avg_input_price_tokens, 
				avg_output_price_tokens, 
				sample_size
			)
			SELECT 
				'default' as model_name,
				AVG(avg_input_price_tokens) as avg_input_price,
				AVG(avg_output_price_tokens) as avg_output_price,
				SUM(sample_size) as sample_size
			FROM average_model_costs
			WHERE model_name != 'default'
			ON CONFLICT (model_name) DO UPDATE
			SET 
				avg_input_price_tokens = EXCLUDED.avg_input_price_tokens,
				avg_output_price_tokens = EXCLUDED.avg_output_price_tokens,
				sample_size = EXCLUDED.sample_size,
				updated_at = CURRENT_TIMESTAMP`)
		if err != nil {
			s.logger.Error("Failed to update default pricing: %v", err)
			return fmt.Errorf("failed to update default pricing: %v", err)
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to update model costs: %v", err)
	}

	return nil
}
