package provider_comm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
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

	s.logger.Info("  Network time (just HTTP request): %dms", networkTime)

	if err != nil {
		s.logger.Error("Provider request failed after %dms: %v", networkTime, err)
		return nil, common.ErrInternalServer(fmt.Errorf("error sending request to provider: %w", err))
	}
	defer resp.Body.Close()

	s.logger.Info("Response received from provider")
	s.logger.Info("Response headers:")
	for k, v := range resp.Header {
		s.logger.Info("  %s: %v", k, v)
	}

	// Check response status code first
	if resp.StatusCode != http.StatusOK {
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			s.logger.Error("Failed to read error response body: %v", err)
		} else {
			s.logger.Error("Provider returned non-200 status code %d: %s", resp.StatusCode, string(bodyBytes))
		}
		return nil, common.ErrInternalServer(fmt.Errorf("provider returned status %d", resp.StatusCode))
	}

	// Peek at the first few bytes to check for SSE format
	bodyReader := bufio.NewReader(resp.Body)
	peek, err := bodyReader.Peek(5) // Look for "data:" prefix
	if err != nil && err != io.EOF {
		s.logger.Error("Failed to peek response: %v", err)
		return nil, common.ErrInternalServer(fmt.Errorf("error reading response: %w", err))
	}

	// Check if response is SSE format either by content type or content
	contentType := resp.Header.Get("Content-Type")
	isSSEContentType := strings.Contains(strings.ToLower(contentType), "text/event-stream") ||
		strings.Contains(strings.ToLower(contentType), "application/x-ndjson") ||
		strings.Contains(strings.ToLower(contentType), "application/stream+json")
	isSSEContent := len(peek) >= 5 && string(peek[:5]) == "data:"
	isSSE := isSSEContentType || isSSEContent

	s.logger.Info("SSE detection:")
	s.logger.Info("  Content-Type: %s", contentType)
	s.logger.Info("  Content peek: %q", string(peek))
	s.logger.Info("  Is SSE by Content-Type: %v", isSSEContentType)
	s.logger.Info("  Is SSE by content: %v", isSSEContent)
	s.logger.Info("  Final SSE detection: %v", isSSE)

	if isSSE {
		s.logger.Info("Handling as SSE response")
		return s.handleSSEResponse(bodyReader)
	}

	s.logger.Info("Handling as regular JSON response")

	// For non-SSE responses, handle as regular JSON
	bodyBytes, err := io.ReadAll(bodyReader)
	if err != nil {
		s.logger.Error("Failed to read response body: %v", err)
		return nil, common.ErrInternalServer(fmt.Errorf("error reading response body: %w", err))
	}

	var responseData map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &responseData); err != nil {
		s.logger.Error("Failed to parse JSON response: %v", err)
		s.logger.Error("Raw response that failed to parse: %s", string(bodyBytes))
		return nil, common.ErrInternalServer(fmt.Errorf("error parsing provider response: %w", err))
	}

	return responseData, nil
}

// handleSSEResponse processes a Server-Sent Events response and combines the chunks
func (s *Service) handleSSEResponse(body io.Reader) (map[string]interface{}, error) {
	scanner := bufio.NewScanner(body)
	var fullContent string
	var lastChunk map[string]interface{}
	var role string
	var usage map[string]interface{}
	var totalTokens int

	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}

		// Skip empty lines
		if strings.TrimSpace(data) == "" {
			continue
		}

		var chunk map[string]interface{}
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			s.logger.Error("Failed to parse SSE chunk: %v", err)
			s.logger.Error("Raw chunk that failed to parse: %s", data)
			continue
		}

		// Store the last valid chunk for metadata
		lastChunk = chunk

		// Extract usage if present
		if u, ok := chunk["usage"].(map[string]interface{}); ok {
			usage = u
		}

		// Extract content from the chunk
		if choices, ok := chunk["choices"].([]interface{}); ok && len(choices) > 0 {
			if choice, ok := choices[0].(map[string]interface{}); ok {
				if delta, ok := choice["delta"].(map[string]interface{}); ok {
					// Check for role in the first chunk
					if r, ok := delta["role"].(string); ok {
						role = r
					}
					// Accumulate content
					if content, ok := delta["content"].(string); ok {
						fullContent += content
					}
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, common.ErrInternalServer(fmt.Errorf("error reading SSE stream: %w", err))
	}

	if lastChunk == nil {
		return nil, common.ErrInternalServer(fmt.Errorf("no valid chunks received"))
	}

	// If no role was found in deltas, default to "assistant"
	if role == "" {
		role = "assistant"
	}

	// If no usage was found or if it's empty, create estimated usage
	if usage == nil || (usage["total_tokens"] == 0 && usage["completion_tokens"] == 0) {
		promptTokens := 0
		if lastChunk["usage"] != nil {
			if u, ok := lastChunk["usage"].(map[string]interface{}); ok {
				if pt, ok := u["prompt_tokens"].(float64); ok {
					promptTokens = int(pt)
				}
			}
		}
		usage = map[string]interface{}{
			"prompt_tokens":     promptTokens,
			"completion_tokens": totalTokens,
			"total_tokens":      promptTokens + totalTokens,
		}
	}

	// Clean up the content
	fullContent = strings.TrimSpace(fullContent)

	// Construct final response in OpenAI format
	response := map[string]interface{}{
		"id":      lastChunk["id"],
		"object":  "chat.completion",
		"created": lastChunk["created"],
		"model":   lastChunk["model"],
		"choices": []map[string]interface{}{
			{
				"index": 0,
				"message": map[string]interface{}{
					"role":    role,
					"content": fullContent,
				},
				"finish_reason": "stop",
			},
		},
		"usage": usage,
	}

	s.logger.Info("Constructed OpenAI-compatible response: %+v", response)
	return response, nil
}
