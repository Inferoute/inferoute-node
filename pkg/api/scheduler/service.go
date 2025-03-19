package scheduler

import (
	"context"
	"fmt"
	"net/http"

	"github.com/robfig/cron/v3"
	"github.com/sentnl/inferoute-node/pkg/common"
)

// NewService creates a new scheduler service
func NewService(internalKey string, logger *common.Logger) *Service {
	if internalKey == "" {
		logger.Error("WARNING: Internal key is empty!")
	}
	return &Service{
		logger:      logger,
		cron:        cron.New(cron.WithSeconds()),
		internalKey: internalKey,
	}
}

func (s *Service) Start(ctx context.Context) error {
	if s.isInitialized {
		return fmt.Errorf("scheduler already initialized")
	}

	// Update model pricing every minute
	_, err := s.cron.AddFunc("0 * * * * *", func() {
		if err := s.updateModelPricing(ctx); err != nil {
			s.logger.Error("Failed to update model pricing: %v", err)
		}
	})
	if err != nil {
		return fmt.Errorf("failed to add model pricing update job: %w", err)
	}

	// Update provider tiers every 5 minutes  - 0 */5 * * * *
	_, err = s.cron.AddFunc("0 */5 * * * *", func() {
		if err := s.updateProviderTiers(ctx); err != nil {
			s.logger.Error("Failed to update provider tiers: %v", err)
		}
	})
	if err != nil {
		return fmt.Errorf("failed to add provider tiers update job: %w", err)
	}

	// Check stale providers every 5 minutes
	_, err = s.cron.AddFunc("0 */5 * * * *", func() {
		if err := s.checkStaleProviders(ctx); err != nil {
			s.logger.Error("Failed to check stale providers: %v", err)
		}
	})
	if err != nil {
		return fmt.Errorf("failed to add stale providers check job: %w", err)
	}

	s.cron.Start()
	s.isInitialized = true
	s.logger.Info("Scheduler started successfully")
	return nil
}

func (s *Service) Stop() {
	if s.cron != nil {
		s.cron.Stop()
	}
	s.isInitialized = false
	s.logger.Info("Scheduler stopped")
}

func (s *Service) makeRequest(ctx context.Context, method string, endpoint common.ServiceEndpoint, path string) error {

	// Add internal key to context
	ctx = context.WithValue(ctx, "internal_key", s.internalKey)
	ctx = context.WithValue(ctx, "logger", s.logger)

	_, err := common.MakeInternalRequest(
		ctx,
		method,
		endpoint,
		path,
		nil,
	)
	if err != nil {
		s.logger.Error("Request failed with error: %v", err)
		return fmt.Errorf("request failed: %w", err)
	}

	return nil
}

func (s *Service) updateModelPricing(ctx context.Context) error {
	s.logger.Info("Running scheduled model pricing update")
	return s.makeRequest(ctx, http.MethodPost, common.ModelPricingService, "/api/model-pricing/update-pricing-data")
}

func (s *Service) updateProviderTiers(ctx context.Context) error {
	s.logger.Info("Running scheduled provider tiers update")
	s.logger.Info("Provider tiers endpoint: /api/health/providers/update-tiers")
	return s.makeRequest(ctx, http.MethodPost, common.ProviderHealthService, "/api/health/providers/update-tiers")
}

func (s *Service) checkStaleProviders(ctx context.Context) error {
	s.logger.Info("Running scheduled stale providers check")
	s.logger.Info("Stale providers endpoint: /api/health/providers/check-stale")
	return s.makeRequest(ctx, http.MethodPost, common.ProviderHealthService, "/api/health/providers/check-stale")
}
