package common

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/sentnl/inferoute-node/internal/config"
)

// ServiceEndpoint represents different internal service endpoints
type ServiceEndpoint struct {
	Host string
	Port int
}

var (
	// Define service endpoints
	AuthService                  = ServiceEndpoint{Host: "auth", Port: 8081}
	OrchestratorService          = ServiceEndpoint{Host: "orchestrator", Port: 8080}
	ProviderManagementService    = ServiceEndpoint{Host: "provider-management", Port: 8082}
	ProviderCommunicationService = ServiceEndpoint{Host: "provider-communication", Port: 8083}
	ProviderHealthService        = ServiceEndpoint{Host: "provider-health", Port: 8084}
	PaymentService               = ServiceEndpoint{Host: "payment", Port: 8085}
)

// MakeInternalRequest makes a request to another internal service
func MakeInternalRequest(ctx context.Context, method string, endpoint ServiceEndpoint, path string, body interface{}) (map[string]interface{}, error) {
	// Get config for internal key
	cfg, err := config.LoadConfig("")
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	// Convert body to JSON if it exists
	var bodyReader io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		bodyReader = bytes.NewBuffer(jsonBody)
	}

	// Create URL
	url := fmt.Sprintf("http://%s:%d%s", endpoint.Host, endpoint.Port, path)

	// Create request
	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Internal-Key", cfg.InternalAPIKey)

	// Make request
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Check response status
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("request failed with status %d: %v", resp.StatusCode, result["error"])
	}

	return result, nil
}

// MakeInternalRequestRaw is similar to MakeInternalRequest but returns the raw response body
func MakeInternalRequestRaw(ctx context.Context, method string, endpoint ServiceEndpoint, path string, body interface{}) ([]byte, error) {
	// Get config for internal key
	cfg, err := config.LoadConfig("")
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	// Convert body to JSON if it exists
	var bodyReader io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		bodyReader = bytes.NewBuffer(jsonBody)
	}

	// Create URL
	url := fmt.Sprintf("http://%s:%d%s", endpoint.Host, endpoint.Port, path)

	// Create request
	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Internal-Key", cfg.InternalAPIKey)

	// Make request
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Check response status
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("request failed with status %d: %s", resp.StatusCode, respBody)
	}

	return respBody, nil
}
