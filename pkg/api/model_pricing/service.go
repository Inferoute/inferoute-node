package model_pricing

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

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
		`SELECT model_name, input_close, output_close, 1 as sample_size 
		FROM model_pricing_data 
		WHERE model_name = 'default' 
		ORDER BY timestamp DESC 
		LIMIT 1`).
		Scan(&defaultPricing.ModelName, &defaultPricing.AvgInputPrice, &defaultPricing.AvgOutputPrice, &defaultPricing.SampleSize)
	if err != nil {
		s.logger.Error("Failed to get default pricing: %v", err)
		return nil, fmt.Errorf("failed to get default pricing: %v", err)
	}

	// Normalize model names by removing ":latest" suffix if present
	normalizedModels := make([]string, len(models))
	for i, model := range models {
		normalizedModels[i] = normalizeModelName(model)
	}

	// Add 'default' to the list of models to query
	allModels := append(normalizedModels, "default")

	// Query for all requested models
	rows, err := s.db.QueryContext(ctx,
		`WITH latest_pricing AS (
			SELECT DISTINCT ON (model_name) 
				model_name, 
				input_close as avg_input_price, 
				output_close as avg_output_price,
				1 as sample_size
			FROM model_pricing_data
			WHERE model_name = ANY($1)
			ORDER BY model_name, timestamp DESC
		)
		SELECT * FROM latest_pricing`, pq.Array(allModels))
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

	// Build response including all models and default
	response := &GetPricesResponse{
		ModelPrices: make([]ModelPricing, len(allModels)),
	}

	// Add all models including default
	for i, modelName := range allModels {
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

// UpdateModelPricingData collects and stores pricing data for candlestick charts
// This function is designed to run every minute
func (s *Service) UpdateModelPricingData(ctx context.Context) (int, error) {
	s.logger.Info("Starting to update model pricing data for candlestick charts")

	// Get the last processed transaction timestamp
	var lastProcessedTime time.Time
	err := s.db.QueryRowContext(ctx, `
		SELECT setting_value 
		FROM system_settings 
		WHERE setting_key = 'last_processed_transaction_time'`).
		Scan(&lastProcessedTime)
	if err != nil {
		if err == sql.ErrNoRows {
			// If no record exists, use a very old timestamp
			lastProcessedTime = time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC)
		} else {
			s.logger.Error("Failed to get last processed transaction time: %v", err)
			return 0, fmt.Errorf("failed to get last processed transaction time: %v", err)
		}
	}

	// Get current time to use as the new last processed time
	currentTime := time.Now()

	// Get all unique model names from provider_models
	rows, err := s.db.QueryContext(ctx, `
		SELECT DISTINCT model_name 
		FROM provider_models 
		WHERE is_active = true`)
	if err != nil {
		s.logger.Error("Failed to query unique model names: %v", err)
		return 0, fmt.Errorf("failed to query unique model names: %v", err)
	}
	defer rows.Close()

	// Collect all model names
	var modelNames []string
	for rows.Next() {
		var modelName string
		if err := rows.Scan(&modelName); err != nil {
			s.logger.Error("Failed to scan model name: %v", err)
			return 0, fmt.Errorf("failed to scan model name: %v", err)
		}
		// Normalize model name by removing ":latest" suffix if present
		normalizedModelName := normalizeModelName(modelName)
		modelNames = append(modelNames, normalizedModelName)
	}

	// Count of models processed
	processedCount := 0

	// Collect metrics for default calculation
	var inputHighs, inputLows, inputCloses []float64
	var outputHighs, outputLows, outputCloses []float64
	var totalVolumeInput, totalVolumeOutput int

	// Process each model
	for _, modelName := range modelNames {
		err := s.db.ExecuteTx(ctx, func(tx *sql.Tx) error {
			// Get the previous entry for this model to use as input_open
			var prevInputClose, prevOutputClose float64
			var hasLastEntry bool

			err := tx.QueryRowContext(ctx, `
				SELECT input_close, output_close 
				FROM model_pricing_data 
				WHERE model_name = $1 
				ORDER BY timestamp DESC 
				LIMIT 1`,
				modelName).Scan(&prevInputClose, &prevOutputClose)

			if err != nil {
				if err == sql.ErrNoRows {
					// No previous entry, we'll use current averages for open values
					hasLastEntry = false
				} else {
					s.logger.Error("Failed to get previous pricing data for model %s: %v", modelName, err)
					return fmt.Errorf("failed to get previous pricing data for model %s: %v", modelName, err)
				}
			} else {
				hasLastEntry = true
			}

			// Get current pricing metrics for the model
			// We need to query with both the normalized model name and with ":latest" suffix
			var inputHigh, inputLow, inputClose, outputHigh, outputLow, outputClose float64

			// First try with the normalized model name
			err = tx.QueryRowContext(ctx, `
				SELECT 
					MAX(input_price_tokens), 
					MIN(input_price_tokens), 
					AVG(input_price_tokens),
					MAX(output_price_tokens), 
					MIN(output_price_tokens), 
					AVG(output_price_tokens)
				FROM provider_models 
				WHERE (model_name = $1 OR model_name = $2) AND is_active = true`,
				modelName, modelName+":latest").
				Scan(&inputHigh, &inputLow, &inputClose, &outputHigh, &outputLow, &outputClose)

			if err != nil {
				if err == sql.ErrNoRows {
					s.logger.Info("No active providers for model %s, skipping", modelName)
					return nil
				}
				s.logger.Error("Failed to calculate pricing metrics for model %s: %v", modelName, err)
				return fmt.Errorf("failed to calculate pricing metrics for model %s: %v", modelName, err)
			}

			// If we don't have a previous entry, use current values for open
			if !hasLastEntry {
				prevInputClose = inputClose
				prevOutputClose = outputClose
			}

			// Get volume data from transactions table
			// We need to query with both the normalized model name and with ":latest" suffix
			var volumeInput, volumeOutput int
			err = tx.QueryRowContext(ctx, `
				SELECT 
					COALESCE(SUM(total_input_tokens), 0),
					COALESCE(SUM(total_output_tokens), 0)
				FROM transactions 
				WHERE (model_name = $1 OR model_name = $2)
				AND status = 'completed' 
				AND updated_at > $3 
				AND updated_at <= $4`,
				modelName, modelName+":latest", lastProcessedTime, currentTime).
				Scan(&volumeInput, &volumeOutput)

			if err != nil && err != sql.ErrNoRows {
				s.logger.Error("Failed to get volume data for model %s: %v", modelName, err)
				return fmt.Errorf("failed to get volume data for model %s: %v", modelName, err)
			}

			// Insert the new pricing data
			_, err = tx.ExecContext(ctx, `
				INSERT INTO model_pricing_data (
					model_name, timestamp,
					input_open, input_high, input_low, input_close,
					output_open, output_high, output_low, output_close,
					volume_input, volume_output
				) VALUES (
					$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12
				)`,
				modelName, currentTime,
				prevInputClose, inputHigh, inputLow, inputClose,
				prevOutputClose, outputHigh, outputLow, outputClose,
				volumeInput, volumeOutput)

			if err != nil {
				s.logger.Error("Failed to insert pricing data for model %s: %v", modelName, err)
				return fmt.Errorf("failed to insert pricing data for model %s: %v", modelName, err)
			}

			// Collect metrics for default calculation
			inputHighs = append(inputHighs, inputHigh)
			inputLows = append(inputLows, inputLow)
			inputCloses = append(inputCloses, inputClose)
			outputHighs = append(outputHighs, outputHigh)
			outputLows = append(outputLows, outputLow)
			outputCloses = append(outputCloses, outputClose)
			totalVolumeInput += volumeInput
			totalVolumeOutput += volumeOutput

			return nil
		})

		if err != nil {
			s.logger.Error("Failed to process model %s: %v", modelName, err)
			continue
		}

		processedCount++
	}

	// Update default entry based on current batch averages
	if len(inputCloses) > 0 {
		err := s.db.ExecuteTx(ctx, func(tx *sql.Tx) error {
			// Calculate averages for default entry
			var defaultInputHigh, defaultInputLow, defaultInputClose float64
			var defaultOutputHigh, defaultOutputLow, defaultOutputClose float64
			var defaultPrevInputClose, defaultPrevOutputClose float64

			// Calculate averages from current batch
			defaultInputHigh = calculateAverage(inputHighs)
			defaultInputLow = calculateAverage(inputLows)
			defaultInputClose = calculateAverage(inputCloses)
			defaultOutputHigh = calculateAverage(outputHighs)
			defaultOutputLow = calculateAverage(outputLows)
			defaultOutputClose = calculateAverage(outputCloses)

			// Get previous default entry
			err := tx.QueryRowContext(ctx, `
				SELECT input_close, output_close 
				FROM model_pricing_data 
				WHERE model_name = 'default' 
				ORDER BY timestamp DESC 
				LIMIT 1`).Scan(&defaultPrevInputClose, &defaultPrevOutputClose)

			if err != nil {
				if err == sql.ErrNoRows {
					// No previous entry, use current values
					defaultPrevInputClose = defaultInputClose
					defaultPrevOutputClose = defaultOutputClose
				} else {
					s.logger.Error("Failed to get previous default pricing data: %v", err)
					return fmt.Errorf("failed to get previous default pricing data: %v", err)
				}
			}

			// Insert default entry
			_, err = tx.ExecContext(ctx, `
				INSERT INTO model_pricing_data (
					model_name, timestamp,
					input_open, input_high, input_low, input_close,
					output_open, output_high, output_low, output_close,
					volume_input, volume_output
				) VALUES (
					'default', $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11
				)`,
				currentTime,
				defaultPrevInputClose, defaultInputHigh, defaultInputLow, defaultInputClose,
				defaultPrevOutputClose, defaultOutputHigh, defaultOutputLow, defaultOutputClose,
				totalVolumeInput, totalVolumeOutput)

			if err != nil {
				s.logger.Error("Failed to insert default pricing data: %v", err)
				return fmt.Errorf("failed to insert default pricing data: %v", err)
			}

			return nil
		})

		if err != nil {
			s.logger.Error("Failed to update default pricing data: %v", err)
		} else {
			processedCount++ // Count default as processed
			s.logger.Info("Updated default pricing data based on current batch averages")
		}
	}

	// Update the last processed transaction time
	_, err = s.db.ExecContext(ctx, `
		UPDATE system_settings 
		SET setting_value = $1, updated_at = NOW() 
		WHERE setting_key = 'last_processed_transaction_time'`,
		currentTime.Format(time.RFC3339))
	if err != nil {
		s.logger.Error("Failed to update last processed transaction time: %v", err)
	}

	s.logger.Info("Successfully updated pricing data for %d models", processedCount)
	return processedCount, nil
}

// Helper function to normalize model names by removing ":latest" suffix if present
func normalizeModelName(modelName string) string {
	const latestSuffix = ":latest"
	if strings.HasSuffix(modelName, latestSuffix) {
		return modelName[:len(modelName)-len(latestSuffix)]
	}
	return modelName
}

// Helper function to calculate average of a slice of float64
func calculateAverage(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}

	var sum float64
	for _, v := range values {
		sum += v
	}

	return sum / float64(len(values))
}

// GetModelPricingData retrieves candlestick chart data for a specific model
func (s *Service) GetModelPricingData(ctx context.Context, modelName string, limit int) (*GetPricingDataResponse, error) {
	if limit <= 0 {
		limit = 60 // Default to 1 hour of data (assuming 1-minute intervals)
	}

	// Normalize model name by removing ":latest" suffix if present
	normalizedModelName := normalizeModelName(modelName)

	rows, err := s.db.QueryContext(ctx, `
		SELECT 
			model_name, timestamp,
			input_open, input_high, input_low, input_close,
			output_open, output_high, output_low, output_close,
			volume_input, volume_output
		FROM model_pricing_data
		WHERE model_name = $1
		ORDER BY timestamp DESC
		LIMIT $2`,
		normalizedModelName, limit)

	if err != nil {
		s.logger.Error("Failed to query pricing data for model %s: %v", normalizedModelName, err)
		return nil, fmt.Errorf("failed to query pricing data for model %s: %v", normalizedModelName, err)
	}
	defer rows.Close()

	var data []ModelPricingDataResponse
	for rows.Next() {
		var item ModelPricingDataResponse
		var timestamp time.Time

		err := rows.Scan(
			&item.ModelName, &timestamp,
			&item.InputOpen, &item.InputHigh, &item.InputLow, &item.InputClose,
			&item.OutputOpen, &item.OutputHigh, &item.OutputLow, &item.OutputClose,
			&item.VolumeInput, &item.VolumeOutput)

		if err != nil {
			s.logger.Error("Failed to scan pricing data: %v", err)
			return nil, fmt.Errorf("failed to scan pricing data: %v", err)
		}

		item.Timestamp = timestamp.Format(time.RFC3339)
		data = append(data, item)
	}

	return &GetPricingDataResponse{Data: data}, nil
}
