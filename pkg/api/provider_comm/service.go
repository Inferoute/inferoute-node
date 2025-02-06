package provider_comm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
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

// SendRequest sends a request to a provider and waits for the response
func (s *Service) SendRequest(ctx context.Context, req SendRequestRequest) (*SendRequestResponse, error) {
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
	latency := time.Since(startTime).Milliseconds()

	if err != nil {
		return &SendRequestResponse{
			Success: false,
			Error:   fmt.Sprintf("error sending request to provider: %v", err),
			Latency: latency,
		}, nil
	}
	defer resp.Body.Close()

	// Parse the response
	var responseData map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&responseData); err != nil {
		s.logger.Error("Failed to parse provider response: %v", err)
		return &SendRequestResponse{
			Success: false,
			Error:   fmt.Sprintf("error parsing provider response: %v", err),
			Latency: latency,
		}, nil
	}

	response := &SendRequestResponse{
		Success:      resp.StatusCode == http.StatusOK,
		ResponseData: responseData,
		Latency:      latency,
	}

	// Log the complete response for debugging
	s.logger.Info("Provider Response:")
	s.logger.Info("  Status Code: %d", resp.StatusCode)
	s.logger.Info("  Success: %v", response.Success)
	s.logger.Info("  Latency: %dms", response.Latency)
	s.logger.Info("  Response Data: %+v", response.ResponseData)

	return response, nil
}
