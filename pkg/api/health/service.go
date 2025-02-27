package health

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/sentnl/inferoute-node/internal/db"
	"github.com/sentnl/inferoute-node/pkg/common"
	"github.com/sentnl/inferoute-node/pkg/rabbitmq"
)

const (
	healthCheckTimeout = 30 * time.Minute
	slaWindow          = 30 * 24 * time.Hour // 30 days
	tier1Threshold     = 0.99                // 99%
	tier2Threshold     = 0.95                // 95%
)

// Service handles provider health management
type Service struct {
	db     *db.DB
	logger *common.Logger
	rmq    *rabbitmq.Client
}

// NewService creates a new provider health service
func NewService(db *db.DB, logger *common.Logger, rmq *rabbitmq.Client) *Service {
	return &Service{
		db:     db,
		logger: logger,
		rmq:    rmq,
	}
}

// StartHealthCheckConsumer starts consuming health check messages from RabbitMQ
func (s *Service) StartHealthCheckConsumer(ctx context.Context) error {
	s.logger.Info("Starting health check consumer")
	return s.rmq.Consume(
		"provider_health",         // exchange
		"health_updates",          // routing key
		"provider_health_updates", // queue name
		func(msg []byte) error {
			var healthMsg ProviderHealthMessage
			if err := json.Unmarshal(msg, &healthMsg); err != nil {
				s.logger.Error("Failed to unmarshal health message: %v", err)
				return fmt.Errorf("error unmarshaling health message: %w", err)
			}

			s.logger.Info("Processing health check for API key: %s", healthMsg.APIKey)
			if err := s.processHealthCheck(ctx, healthMsg); err != nil {
				s.logger.Error("Failed to process health check: %v", err)
				return err
			}
			s.logger.Info("Successfully processed health check for API key: %s", healthMsg.APIKey)
			return nil
		},
	)
}

// processHealthCheck processes a health check message from a provider
func (s *Service) processHealthCheck(ctx context.Context, msg ProviderHealthMessage) error {
	// Get provider ID from API key
	var providerID uuid.UUID
	var serviceType string
	err := s.db.QueryRowContext(ctx,
		`SELECT p.id, p.provider_type
		FROM providers p
		JOIN api_keys ak ON ak.provider_id = p.id
		WHERE ak.api_key = $1 AND ak.is_active = true`,
		msg.APIKey,
	).Scan(&providerID, &serviceType)
	if err != nil {
		return fmt.Errorf("error getting provider ID: %w", err)
	}

	return s.db.ExecuteTx(ctx, func(tx *sql.Tx) error {
		// Update provider information if GPU or Ngrok data is provided
		if msg.GPU != nil || msg.Ngrok != nil || msg.ProviderType != "" {
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
			if msg.Ngrok != nil && msg.Ngrok.URL != "" {
				ngrokURL = &msg.Ngrok.URL
			}

			var productName, driverVersion, cudaVersion *string
			var gpuCount, memoryTotal, memoryFree *int
			if msg.GPU != nil {
				if msg.GPU.ProductName != "" {
					productName = &msg.GPU.ProductName
				}
				if msg.GPU.DriverVersion != "" {
					driverVersion = &msg.GPU.DriverVersion
				}
				if msg.GPU.CudaVersion != "" {
					cudaVersion = &msg.GPU.CudaVersion
				}
				if msg.GPU.GPUCount > 0 {
					gpuCount = &msg.GPU.GPUCount
				}
				if msg.GPU.MemoryTotal > 0 {
					memoryTotal = &msg.GPU.MemoryTotal
				}
				if msg.GPU.MemoryFree > 0 {
					memoryFree = &msg.GPU.MemoryFree
				}
			}

			var providerType *string
			if msg.ProviderType != "" {
				providerType = &msg.ProviderType
				serviceType = msg.ProviderType
			}

			_, err := tx.ExecContext(ctx, query,
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
		}

		// Get provider's models from database
		rows, err := tx.QueryContext(ctx,
			`SELECT id, model_name 
			FROM provider_models 
			WHERE provider_id = $1`,
			providerID,
		)
		if err != nil {
			return fmt.Errorf("error getting provider models: %w", err)
		}
		defer rows.Close()

		// Map to store existing models in the database
		dbModels := make(map[string]uuid.UUID)
		for rows.Next() {
			var id uuid.UUID
			var modelName string
			if err := rows.Scan(&id, &modelName); err != nil {
				return fmt.Errorf("error scanning model: %w", err)
			}
			dbModels[modelName] = id
		}

		// Process models from health update
		healthModels := make(map[string]ProviderHealthPushModel)
		for _, model := range msg.Models {
			// Process model name - remove ":latest" suffix if present
			modelName := model.ID
			if strings.HasSuffix(modelName, ":latest") {
				modelName = strings.TrimSuffix(modelName, ":latest")
			}

			healthModels[modelName] = model

			// Check if model exists in database
			if modelID, exists := dbModels[modelName]; exists {
				// Update existing model
				_, err := tx.ExecContext(ctx,
					`UPDATE provider_models 
					SET model_created = $1, 
						model_owned_by = $2,
						is_active = true,
						updated_at = NOW()
					WHERE id = $3`,
					time.Unix(model.Created, 0),
					model.OwnedBy,
					modelID,
				)
				if err != nil {
					s.logger.Error("Failed to update model metadata: %v", err)
					// Continue processing even if this update fails
				}

				// Remove from dbModels map to track which models need to be deleted
				delete(dbModels, modelName)
			} else {
				// Add new model to database with default pricing
				_, err := tx.ExecContext(ctx,
					`INSERT INTO provider_models (
						id, provider_id, model_name, service_type,
						input_price_tokens, output_price_tokens,
						is_active, model_created, model_owned_by,
						created_at, updated_at
					) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, NOW(), NOW())`,
					uuid.New(),
					providerID,
					modelName,
					serviceType,
					0.0001, // Default input price
					0.0001, // Default output price
					true,
					time.Unix(model.Created, 0),
					model.OwnedBy,
				)
				if err != nil {
					s.logger.Error("Failed to add new model: %v", err)
					// Continue processing even if this update fails
				} else {
					s.logger.Info("Added new model: %s for provider %s", modelName, providerID)
				}
			}
		}

		// Delete models that weren't in the health update
		for modelName, modelID := range dbModels {
			_, err := tx.ExecContext(ctx,
				`DELETE FROM provider_models WHERE id = $1`,
				modelID,
			)
			if err != nil {
				s.logger.Error("Failed to delete model %s: %v", modelName, err)
				// Continue processing even if this deletion fails
			} else {
				s.logger.Info("Deleted model: %s for provider %s", modelName, providerID)
			}
		}

		// Determine health status - simplified to just green or red
		// Green = at least one model is presented
		// Red = no models are presented
		var healthStatus HealthStatus
		if len(msg.Models) > 0 {
			healthStatus = HealthStatusGreen
		} else {
			healthStatus = HealthStatusRed
		}

		s.logger.Info("Health check status: %s (provider has %d models)",
			healthStatus, len(msg.Models))

		// Update provider status
		_, err = tx.ExecContext(ctx,
			`UPDATE providers 
			SET health_status = $1,
				last_health_check = NOW(),
				is_available = $2,
				updated_at = NOW()
			WHERE id = $3`,
			healthStatus,
			healthStatus != HealthStatusRed,
			providerID,
		)
		if err != nil {
			return fmt.Errorf("error updating provider status: %w", err)
		}

		// Record health history with GPU metrics if available
		var gpuUtilization, memoryUsed, memoryTotal *int
		if msg.GPU != nil {
			gpuUtilization = &msg.GPU.Utilization
			memoryUsed = &msg.GPU.MemoryUsed
			memoryTotal = &msg.GPU.MemoryTotal
		}

		_, err = tx.ExecContext(ctx,
			`INSERT INTO provider_health_history 
			(id, provider_id, health_status, latency_ms, gpu_utilization, memory_used, memory_total, health_check_time)
			VALUES ($1, $2, $3, $4, $5, $6, $7, NOW())`,
			uuid.New(),
			providerID,
			healthStatus,
			0, // latency_ms - we don't have this in the health check message
			gpuUtilization,
			memoryUsed,
			memoryTotal,
		)
		if err != nil {
			return fmt.Errorf("error recording health history: %w", err)
		}

		return nil
	})
}

// CheckStaleProviders checks and updates providers that haven't sent a health check recently
func (s *Service) CheckStaleProviders(ctx context.Context) (int, error) {
	return s.db.ExecuteTxInt(ctx, func(tx *sql.Tx) (int, error) {
		// Update stale providers to unavailable
		result, err := tx.ExecContext(ctx,
			`UPDATE providers
			SET is_available = false,
				updated_at = NOW()
			WHERE last_health_check < NOW() - INTERVAL '5 minutes'
				AND is_available = true
				AND NOT paused`)
		if err != nil {
			return 0, fmt.Errorf("error updating stale providers: %w", err)
		}

		affected, err := result.RowsAffected()
		if err != nil {
			return 0, fmt.Errorf("error getting affected rows: %w", err)
		}

		return int(affected), nil
	})
}

// UpdateProviderTiers updates provider tiers based on their health history
func (s *Service) UpdateProviderTiers(ctx context.Context) (int, error) {
	s.logger.Info("Starting provider tier update process")

	// Single query to calculate and update tiers
	query := `
	WITH health_stats AS (
		SELECT 
			provider_id,
			COUNT(*) as total_checks,
			COUNT(*) FILTER (WHERE health_status = 'green') as green_checks,
			CASE
				WHEN COUNT(*) > 0 THEN 
					(COUNT(*) FILTER (WHERE health_status = 'green')::float / COUNT(*)::float)
				ELSE 0
			END as health_percentage
		FROM provider_health_history
		WHERE health_check_time > NOW() - INTERVAL '720 hour'
		GROUP BY provider_id
	)
	UPDATE providers p
	SET 
		tier = CASE
			WHEN hs.health_percentage >= 0.99 THEN 1
			WHEN hs.health_percentage >= 0.95 THEN 2
			ELSE 3
		END,
		updated_at = NOW()
	FROM health_stats hs
	WHERE p.id = hs.provider_id
	AND hs.total_checks > 0
	AND (
		CASE
			WHEN hs.health_percentage >= 0.99 THEN 1
			WHEN hs.health_percentage >= 0.95 THEN 2
			ELSE 3
		END != p.tier
	)
	RETURNING p.id, 
		hs.health_percentage,
		p.tier as new_tier,
		hs.green_checks,
		hs.total_checks`

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		s.logger.Error("Failed to update provider tiers: %v", err)
		return 0, fmt.Errorf("error updating provider tiers: %w", err)
	}
	defer rows.Close()

	var updatedCount int
	for rows.Next() {
		var (
			providerID    uuid.UUID
			healthPercent float64
			newTier       int
			greenChecks   int
			totalChecks   int
		)

		if err := rows.Scan(&providerID, &healthPercent, &newTier, &greenChecks, &totalChecks); err != nil {
			s.logger.Error("Failed to scan update result: %v", err)
			return updatedCount, fmt.Errorf("error scanning update result: %w", err)
		}

		s.logger.Info("Updated provider %s: Health %.2f%% (%d/%d green) -> Tier %d",
			providerID, healthPercent*100, greenChecks, totalChecks, newTier)

		updatedCount++
	}

	if err = rows.Err(); err != nil {
		s.logger.Error("Error iterating results: %v", err)
		return updatedCount, fmt.Errorf("error iterating results: %w", err)
	}

	s.logger.Info("Completed provider tier updates. Total providers updated: %d", updatedCount)
	return updatedCount, nil
}

// GetHealthyNodes returns a list of healthy nodes that match the criteria
func (s *Service) GetHealthyNodes(ctx context.Context, req GetHealthyNodesRequest) (*GetHealthyNodesResponse, error) {
	var response GetHealthyNodesResponse

	// Query healthy providers that match the criteria
	rows, err := s.db.QueryContext(ctx,
		`SELECT 
			p.id,
			u.username,
			pm.input_price_tokens,
			pm.output_price_tokens,
			p.tier,
			p.health_status
		FROM providers p
		JOIN users u ON u.id = p.user_id
		JOIN provider_models pm ON pm.provider_id = p.id
		WHERE p.is_available = true
			AND NOT p.paused
			AND p.health_status = 'green'
			AND p.tier <= $1
			AND pm.model_name = $2
			AND pm.is_active = true
			AND (pm.input_price_tokens <= $3 AND pm.output_price_tokens <= $3)
		ORDER BY p.tier ASC, p.last_health_check DESC`,
		req.Tier,
		req.ModelName,
		req.MaxCost,
	)
	if err != nil {
		return nil, fmt.Errorf("error querying healthy nodes: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var node HealthyNode
		err := rows.Scan(
			&node.ProviderID,
			&node.Username,
			&node.InputPriceTokens,
			&node.OutputPriceTokens,
			&node.Tier,
			&node.HealthStatus,
		)
		if err != nil {
			return nil, fmt.Errorf("error scanning row: %w", err)
		}
		response.Nodes = append(response.Nodes, node)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return &response, nil
}
