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
func (s *Service) SendRequest(ctx context.Context, req SendRequestRequest) (map[string]interface{}, error) {
	totalStartTime := time.Now()

	// Send the request_data directly as it's already in the correct format
	marshalStartTime := time.Now()
	// Instead of marshaling req.RequestData directly, we need to ensure all fields are preserved
	// Create a copy of the request data to ensure we don't modify the original
	requestData := make(map[string]interface{})
	for k, v := range req.RequestData {
		requestData[k] = v
	}

	// Log the request data to verify all fields are included
	s.logger.Info("Original request data: %+v", requestData)

	// Check if max_tokens exists in the request data
	if _, exists := requestData["max_tokens"]; !exists {
		// If not in request data, try to get from context
		if maxTokens, ok := ctx.Value("max_tokens").(int); ok && maxTokens > 0 {
			requestData["max_tokens"] = maxTokens
			s.logger.Info("Added max_tokens=%d from context", maxTokens)
		}
	} else {
		s.logger.Info("max_tokens already exists in request data: %v", requestData["max_tokens"])
	}

	// Check if temperature exists in the request data
	if _, exists := requestData["temperature"]; !exists {
		// If not in request data, try to get from context
		if temperature, ok := ctx.Value("temperature").(float64); ok {
			requestData["temperature"] = temperature
			s.logger.Info("Added temperature=%f from context", temperature)
		}
	} else {
		s.logger.Info("temperature already exists in request data: %v", requestData["temperature"])
	}

	requestBody, err := json.Marshal(requestData)
	marshalTime := time.Since(marshalStartTime).Milliseconds()
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
	s.logger.Info("  Request preparation took: %dms", time.Since(totalStartTime).Milliseconds())

	startTime := time.Now()
	resp, err := s.client.Do(httpReq)
	networkTime := time.Since(startTime).Milliseconds()
	latency := time.Since(totalStartTime).Milliseconds()

	s.logger.Info("  Network time (just HTTP request): %dms", networkTime)

	if err != nil {
		s.logger.Error("Provider request failed after %dms: %v", networkTime, err)
		return nil, common.ErrInternalServer(fmt.Errorf("error sending request to provider: %w", err))
	}
	defer resp.Body.Close()

	// Parse the response
	decodeStartTime := time.Now()
	var responseData map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&responseData); err != nil {
		decodeTime := time.Since(decodeStartTime).Milliseconds()
		s.logger.Error("Failed to parse provider response after %dms: %v", decodeTime, err)
		return nil, common.ErrInternalServer(fmt.Errorf("error parsing provider response: %w", err))
	}
	decodeTime := time.Since(decodeStartTime).Milliseconds()

	// Log timing metrics
	s.logger.Info("Provider Response:")
	s.logger.Info("  Status Code: %d", resp.StatusCode)
	s.logger.Info("  Latency: %dms", latency)
	s.logger.Info("  Network Time: %dms", networkTime)
	s.logger.Info("  Response Decode Time: %dms", decodeTime)
	s.logger.Info("  Marshal Request Time: %dms", marshalTime)
	s.logger.Info("  Total Provider Comm Time: %dms", time.Since(totalStartTime).Milliseconds())
	s.logger.Info("  Response Data: %+v", responseData)

	if resp.StatusCode != http.StatusOK {
		return nil, common.ErrInternalServer(fmt.Errorf("provider returned status %d", resp.StatusCode))
	}

	return responseData, nil
}
