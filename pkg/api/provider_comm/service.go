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
	// Send the request_data directly as it's already in the correct format
	requestBody, err := json.Marshal(req.RequestData)
	if err != nil {
		return nil, common.ErrInternalServer(fmt.Errorf("error marshaling request: %w", err))
	}

	// Create the request with proper headers
	httpReq, err := http.NewRequestWithContext(ctx, "POST", req.ProviderURL, bytes.NewBuffer(requestBody))
	if err != nil {
		return nil, common.ErrInternalServer(fmt.Errorf("error creating request: %w", err))
	}

	// Set required headers
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Request-ID", req.HMAC) // Use HMAC as request ID
	httpReq.Header.Set("X-Model-Name", req.ModelName)

	// Log the outgoing request for debugging
	s.logger.Info("Sending request to provider:")
	s.logger.Info("  URL: %s", req.ProviderURL)
	s.logger.Info("  Headers: %v", httpReq.Header)
	s.logger.Info("  Body: %s", string(requestBody))

	startTime := time.Now()
	resp, err := s.client.Do(httpReq)
	networkTime := time.Since(startTime).Milliseconds()

	s.logger.Info("  Network time (just HTTP request): %dms", networkTime)

	if err != nil {
		s.logger.Error("Provider request failed after %dms: %v", networkTime, err)
		return nil, common.ErrInternalServer(fmt.Errorf("error sending request to provider: %w", err))
	}

	s.logger.Info("Response received from provider")
	s.logger.Info("Response headers:")
	for k, v := range resp.Header {
		s.logger.Info("  %s: %v", k, v)
	}

	// Check response status code first
	if resp.StatusCode != http.StatusOK {
		bodyBytes, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			s.logger.Error("Failed to read error response body: %v", err)
		} else {
			s.logger.Error("Provider returned non-200 status code %d: %s", resp.StatusCode, string(bodyBytes))
		}
		return nil, common.ErrInternalServer(fmt.Errorf("provider returned status %d", resp.StatusCode))
	}

	// Return the raw response body - let the caller handle streaming/parsing
	return resp.Body, nil
}
