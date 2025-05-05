package ai_applications

import (
	"context"
	"time"

	"github.com/sentnl/inferoute-node/internal/db"
	"github.com/sentnl/inferoute-node/pkg/common"
)

// Service handles business logic for AI applications
type Service struct {
	db     *db.DB
	logger *common.Logger
}

// NewService creates a new AI applications service
func NewService(db *db.DB, logger *common.Logger) *Service {
	return &Service{
		db:     db,
		logger: logger,
	}
}

// GetTopModels retrieves the top 10 models by transaction count
func (s *Service) GetTopModels(ctx context.Context) (*ModelResponse, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT DISTINCT pm.model_name, COUNT(t.id) as transaction_count
		FROM provider_models pm
		LEFT JOIN transactions t ON t.model_name = pm.model_name
		GROUP BY pm.model_name
		ORDER BY transaction_count DESC
		LIMIT 10
	`)
	if err != nil {
		s.logger.Error("Error querying models: %v", err)
		return nil, err
	}
	defer rows.Close()

	var models []ModelDetail
	currentTime := time.Now().Unix()

	for rows.Next() {
		var modelName string
		var transactionCount int
		if err := rows.Scan(&modelName, &transactionCount); err != nil {
			s.logger.Error("Error scanning row: %v", err)
			continue
		}

		model := ModelDetail{
			ID:          modelName,
			Object:      "model",
			Created:     currentTime,
			OwnedBy:     "vllm",
			Root:        modelName,
			Parent:      nil,
			MaxModelLen: 32768,
		}
		models = append(models, model)
	}

	if err = rows.Err(); err != nil {
		s.logger.Error("Error iterating rows: %v", err)
		return nil, err
	}

	return &ModelResponse{
		Object: "list",
		Data:   models,
	}, nil
}
