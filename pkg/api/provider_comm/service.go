package provider_comm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/sentnl/inferoute-node/internal/db"
	"github.com/sentnl/inferoute-node/pkg/common"
)

// Service handles provider communication
type Service struct {
	db     *db.DB
	logger *common.Logger
	client *http.Client
}

// NewService creates a new provider communication service
func NewService(db *db.DB, logger *common.Logger) *Service {
	return &Service{
		db:     db,
		logger: logger,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// SendRequest sends a request to a provider and returns the raw response
func (s *Service) SendRequest(ctx context.Context, req SendRequestRequest) (io.ReadCloser, error) {
	// Log the request body
	reqBody, err := json.Marshal(req.RequestData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}
	s.logger.Info("  Body: %s", string(reqBody))

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, "POST", req.ProviderURL, bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add headers
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Request-ID", req.HMAC)
	httpReq.Header.Set("X-Model-Name", req.ModelName)

	// Send request
	start := time.Now()
	resp, err := s.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	s.logger.Info("  Network time (just HTTP request): %dms", time.Since(start).Milliseconds())

	// Check response status
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("provider returned non-200 status: %d, body: %s", resp.StatusCode, string(body))
	}

	s.logger.Info("Response received from provider")
	s.logger.Info("Response headers:")
	for k, v := range resp.Header {
		s.logger.Info("  %s: %v", k, v)
	}

	return resp.Body, nil
}
