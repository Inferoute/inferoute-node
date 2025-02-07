package health

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
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
	err := s.db.QueryRowContext(ctx,
		`SELECT u.id 
		FROM users u
		JOIN api_keys ak ON u.id = ak.user_id
		WHERE ak.api_key = $1 AND u.type = 'provider'`,
		msg.APIKey,
	).Scan(&providerID)
	if err != nil {
		return fmt.Errorf("error getting provider ID: %w", err)
	}

	return s.db.ExecuteTx(ctx, func(tx *sql.Tx) error {
		// Get provider's models from database (all models, not just active ones)
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

		dbModels := make(map[string]struct{})
		for rows.Next() {
			var id uuid.UUID
			var modelName string
			if err := rows.Scan(&id, &modelName); err != nil {
				return fmt.Errorf("error scanning model: %w", err)
			}
			dbModels[modelName] = struct{}{}
		}

		// Check which models from health check exist in database
		healthModels := make(map[string]struct{})
		for _, model := range msg.Models {
			healthModels[model.ID] = struct{}{}
		}

		// Determine health status
		var healthStatus HealthStatus
		var matchCount int
		if len(msg.Models) == 0 {
			healthStatus = HealthStatusRed
		} else {
			for modelName := range dbModels {
				if _, exists := healthModels[modelName]; exists {
					matchCount++
				}
			}

			if matchCount == len(dbModels) {
				healthStatus = HealthStatusGreen
			} else if matchCount > 0 {
				healthStatus = HealthStatusOrange
			} else {
				healthStatus = HealthStatusRed
			}
		}

		s.logger.Info("Health check status: %s (matched %d out of %d models)",
			healthStatus, matchCount, len(dbModels))

		// Update provider status
		_, err = tx.ExecContext(ctx,
			`UPDATE provider_status 
			SET health_status = $1,
				last_health_check = NOW(),
				is_available = $2,
				updated_at = NOW()
			WHERE provider_id = $3`,
			healthStatus,
			healthStatus != HealthStatusRed,
			providerID,
		)
		if err != nil {
			return fmt.Errorf("error updating provider status: %w", err)
		}

		// Record health history
		_, err = tx.ExecContext(ctx,
			`INSERT INTO provider_health_history 
			(provider_id, health_status, latency_ms, health_check_time)
			VALUES ($1, $2, $3, NOW())`,
			providerID,
			healthStatus,
			0, // TODO: Add latency measurement
		)
		if err != nil {
			return fmt.Errorf("error recording health history: %w", err)
		}

		// Update model active status based on health check
		for modelName := range dbModels {
			isActive := false
			if _, exists := healthModels[modelName]; exists {
				isActive = true
			}
			_, err = tx.ExecContext(ctx,
				`UPDATE provider_models 
				SET is_active = $1,
					updated_at = NOW()
				WHERE provider_id = $2 AND model_name = $3`,
				isActive,
				providerID,
				modelName,
			)
			if err != nil {
				return fmt.Errorf("error updating model status: %w", err)
			}
		}

		return nil
	})
}

// CheckStaleProviders checks and updates providers that haven't sent a health check recently
func (s *Service) CheckStaleProviders(ctx context.Context) (int, error) {
	result, err := s.db.ExecContext(ctx,
		`UPDATE provider_status 
		SET health_status = $1,
			is_available = false,
			updated_at = NOW()
		WHERE last_health_check < NOW() - $2::interval
		AND health_status != $1`,
		HealthStatusRed,
		healthCheckTimeout.String(),
	)
	if err != nil {
		return 0, fmt.Errorf("error updating stale providers: %w", err)
	}

	count, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("error getting affected rows: %w", err)
	}

	return int(count), nil
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
	UPDATE provider_status ps
	SET 
		tier = CASE
			WHEN hs.health_percentage >= 0.99 THEN 1
			WHEN hs.health_percentage >= 0.95 THEN 2
			ELSE 3
		END,
		updated_at = NOW()
	FROM health_stats hs
	WHERE ps.provider_id = hs.provider_id
	AND hs.total_checks > 0
	AND (
		CASE
			WHEN hs.health_percentage >= 0.99 THEN 1
			WHEN hs.health_percentage >= 0.95 THEN 2
			ELSE 3
		END != ps.tier
	)
	RETURNING ps.provider_id, 
		hs.health_percentage,
		ps.tier as new_tier,
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
	rows, err := s.db.QueryContext(ctx,
		`SELECT 
			u.id,
			u.username,
			pm.input_price_tokens,
			pm.output_price_tokens,
			ps.tier,
			ps.health_status
		FROM users u
		JOIN provider_status ps ON u.id = ps.provider_id
		JOIN provider_models pm ON u.id = pm.provider_id
		WHERE pm.model_name = $1
		AND pm.is_active = true
		AND ps.is_available = true
		AND ps.paused = false
		AND ps.tier <= $2
		AND pm.input_price_tokens <= $3
		AND pm.output_price_tokens <= $3
		ORDER BY ps.tier ASC, ps.last_health_check DESC`,
		req.ModelName,
		req.Tier,
		req.MaxCost,
	)
	if err != nil {
		return nil, fmt.Errorf("error getting healthy nodes: %w", err)
	}
	defer rows.Close()

	var nodes []HealthyNode
	for rows.Next() {
		var node HealthyNode
		if err := rows.Scan(
			&node.ProviderID,
			&node.Username,
			&node.InputPriceTokens,
			&node.OutputPriceTokens,
			&node.Tier,
			&node.HealthStatus,
		); err != nil {
			return nil, fmt.Errorf("error scanning healthy node: %w", err)
		}
		nodes = append(nodes, node)
	}

	return &GetHealthyNodesResponse{Nodes: nodes}, nil
}
