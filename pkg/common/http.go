package common

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"
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
	ModelPricingService          = ServiceEndpoint{Host: "model-pricing", Port: 8085}
	TokenizerService             = ServiceEndpoint{Host: "tokenizer", Port: 8088}
)

// MakeInternalRequest makes a request to another internal service
func MakeInternalRequest(ctx context.Context, method string, endpoint ServiceEndpoint, path string, body interface{}) (map[string]interface{}, error) {
	totalStartTime := time.Now()

	// Get logger from context if available
	var logger *Logger
	if l, ok := ctx.Value("logger").(*Logger); ok {
		logger = l
	}

	// Get internal key from context
	internalKey, ok := ctx.Value("internal_key").(string)
	if !ok || internalKey == "" {
		if logger != nil {
			logger.Error("Internal key missing from context")
		}
		return nil, fmt.Errorf("internal key missing from context")
	}

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	// Convert body to JSON if it exists
	marshalStartTime := time.Now()
	var bodyReader io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		bodyReader = bytes.NewBuffer(jsonBody)
	}
	marshalTime := time.Since(marshalStartTime).Milliseconds()

	// Create URL
	url := fmt.Sprintf("http://%s:%d%s", endpoint.Host, endpoint.Port, path)

	// Create request
	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Internal-Key", internalKey)

	// Make request
	httpStartTime := time.Now()
	resp, err := client.Do(req)
	httpTime := time.Since(httpStartTime).Milliseconds()
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	decodeStartTime := time.Now()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Log or inspect the raw response body
	slog.InfoContext(ctx, "Raw response body", "body", string(respBody))

	// Decode the response body
	var result map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	decodeTime := time.Since(decodeStartTime).Milliseconds()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("request failed with status %d: %v", resp.StatusCode, result["error"])
	}

	totalTime := time.Since(totalStartTime).Milliseconds()

	if logger != nil {
		logger.Info("Internal request to %s:%d%s - Total: %dms, HTTP: %dms, Marshal: %dms, Decode: %dms",
			endpoint.Host, endpoint.Port, path, totalTime, httpTime, marshalTime, decodeTime)
	}

	return result, nil
}

// MakeInternalRequestRaw is similar to MakeInternalRequest but returns the raw response body
func MakeInternalRequestRaw(ctx context.Context, method string, endpoint ServiceEndpoint, path string, body interface{}) ([]byte, error) {
	// Get logger from context if available
	var logger *Logger
	if l, ok := ctx.Value("logger").(*Logger); ok {
		logger = l
	}

	// Get internal key from context
	internalKey, ok := ctx.Value("internal_key").(string)
	if !ok || internalKey == "" {
		if logger != nil {
			logger.Error("Internal key missing from context")
		}
		return nil, fmt.Errorf("internal key missing from context")
	}

	if logger != nil {
		logger.Info("Making internal request with key length: %d", len(internalKey))
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
	req.Header.Set("X-Internal-Key", internalKey)

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

func MakeInternalRequestStream(ctx context.Context, method string, endpoint ServiceEndpoint, path string, body interface{}) (*http.Response, error) {
	// Get logger from context if available
	var logger *Logger
	if l, ok := ctx.Value("logger").(*Logger); ok {
		logger = l
	}

	// Get internal key from context
	internalKey, ok := ctx.Value("internal_key").(string)
	if !ok || internalKey == "" {
		if logger != nil {
			logger.Error("Internal key missing from context")
		}
		return nil, fmt.Errorf("internal key missing from context")
	}

	// Create HTTP client without timeout for streaming
	client := &http.Client{}

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
	req.Header.Set("X-Internal-Key", internalKey)
	req.Header.Set("Accept", "text/event-stream") // Add this for SSE support

	// Make request
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}

	// Check response status before returning
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(body))
	}

	return resp, nil
}
