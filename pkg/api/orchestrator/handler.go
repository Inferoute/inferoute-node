package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/sentnl/inferoute-node/pkg/common"
)

type Handler struct {
	service *Service
	logger  *common.Logger
}

func NewHandler(service *Service, logger *common.Logger) *Handler {
	return &Handler{
		service: service,
		logger:  logger,
	}
}

// RegisterRoutes registers the orchestrator routes
func (h *Handler) RegisterRoutes(e *echo.Echo) {
	// OpenAI-compatible routes
	e.POST("/v1/chat/completions", h.ProcessRequest)
	e.POST("/v1/completions", h.ProcessRequest)

	// Custom API routes
	g := e.Group("/api")
	g.POST("/chat", h.ProcessRequest)
	g.POST("/generate", h.ProcessRequest)
}

// ProcessRequest godoc
// @Summary Process an OpenAI-compatible API request
// @Description Handles the complete request flow from consumer to provider
// @Tags orchestrator
// @Accept json
// @Produce json
// @Param request body OpenAIRequest true "OpenAI request"
// @Success 200 {object} interface{} "Provider response"
// @Failure 400 {object} common.ErrorResponse "Invalid request"
// @Failure 401 {object} common.ErrorResponse "Unauthorized"
// @Failure 500 {object} common.ErrorResponse "Internal server error"
// @Router /api/v1/process [post]
func (h *Handler) ProcessRequest(c echo.Context) error {
	// Extract original request path from X-Original-URI header
	originalPath := c.Request().Header.Get("X-Original-URI")
	if originalPath == "" {
		// If no original path is provided, use a default path based on the request type
		if strings.Contains(c.Path(), "chat") {
			originalPath = "/v1/chat/completions"
		} else {
			originalPath = "/v1/completions"
		}
	}

	// Create context with original path
	ctx := context.WithValue(c.Request().Context(), "original_path", originalPath)
	c.SetRequest(c.Request().WithContext(ctx))

	// Get API key from Authorization header
	apiKey := c.Request().Header.Get("Authorization")
	if apiKey == "" {
		return common.ErrUnauthorized(fmt.Errorf("missing API key"))
	}

	// Remove "Bearer " prefix if present
	apiKey = strings.TrimPrefix(apiKey, "Bearer ")

	// Add API key to context
	ctx = context.WithValue(ctx, "api_key", apiKey)
	c.SetRequest(c.Request().WithContext(ctx))

	// Debug: Log raw request body
	//var rawBody map[string]interface{}
	//rawBytes, err := io.ReadAll(c.Request().Body)
	//if err != nil {
	//	h.logger.Error("Failed to read request body: %v", err)
	//	return common.ErrBadRequest(fmt.Errorf("failed to read request body: %w", err))
	//}
	//h.logger.Info("Raw request body: %s", string(rawBytes))

	// Restore the request body for later use
	//c.Request().Body = io.NopCloser(bytes.NewBuffer(rawBytes))

	//if err := json.Unmarshal(rawBytes, &rawBody); err != nil {
	//	h.logger.Error("Failed to parse raw request body: %v", err)
	//	return common.ErrBadRequest(fmt.Errorf("invalid request body: %w", err))
	//}
	//h.logger.Info("Parsed request body: %+v", rawBody)

	// Parse request body into OpenAIRequest
	var req OpenAIRequest
	if err := json.NewDecoder(c.Request().Body).Decode(&req); err != nil {
		h.logger.Error("Failed to parse request into OpenAIRequest: %v", err)
		return common.ErrBadRequest(fmt.Errorf("invalid request body: %w", err))
	}

	// Custom validation for request format
	if err := req.Validate(); err != nil {
		return common.ErrBadRequest(err)
	}

	// Extract consumer ID from context (set by auth middleware)
	consumerID, ok := c.Get("consumer_id").(uuid.UUID)
	if !ok {
		return echo.NewHTTPError(http.StatusUnauthorized, "Invalid or missing consumer ID")
	}

	// Process the request
	h.logger.Info("Orchestrator Handler: Processing request. Stream flag from parsed request: %v", req.Stream)
	response, err := h.service.ProcessRequest(c.Request().Context(), consumerID, &req)
	if err != nil {
		h.logger.Error("Orchestrator Handler: Error from service.ProcessRequest: %v. Stream flag: %v", err, req.Stream)

		// For streaming requests, we need to send the error in SSE format
		if req.Stream {
			h.logger.Info("Orchestrator Handler: Handling error for a stream request.")
			c.Response().Header().Set("Content-Type", "text/event-stream")
			c.Response().Header().Set("Cache-Control", "no-cache")
			c.Response().Header().Set("Connection", "keep-alive")

			errorEvent := map[string]interface{}{
				"error": map[string]interface{}{
					"message": err.Error(),
					"type":    "inferoute_error",
					"code":    http.StatusInternalServerError,
				},
			}

			// Convert to SSE format
			jsonData, _ := json.Marshal(errorEvent)
			errorMsg := fmt.Sprintf("data: %s\n\n", string(jsonData))

			_, writeErr := c.Response().Write([]byte(errorMsg))
			if writeErr != nil {
				h.logger.Error("Error writing error event: %v", writeErr)
			}
			c.Response().Flush()
			return nil
		}

		// Check if this is an AppError
		if appErr, ok := common.IsAppError(err); ok {
			// Return the AppError with its status code and message
			return echo.NewHTTPError(appErr.Code, map[string]interface{}{
				"error": map[string]interface{}{
					"message": err.Error(),
					"type":    "inferoute_error",
					"code":    appErr.Code,
				},
			})
		}

		// For other errors, return a generic error
		return echo.NewHTTPError(http.StatusInternalServerError, map[string]interface{}{
			"error": map[string]interface{}{
				"message": err.Error(),
				"type":    "inferoute_error",
				"code":    http.StatusInternalServerError,
			},
		})
	}

	// Handle streaming response
	if req.Stream {
		h.logger.Info("Orchestrator Handler: Entering stream handling block.")
		// Set streaming headers
		c.Response().Header().Set("Content-Type", "text/event-stream")
		c.Response().Header().Set("Cache-Control", "no-cache")
		c.Response().Header().Set("Connection", "keep-alive")
		c.Response().Header().Set("Transfer-Encoding", "chunked")
		h.logger.Info("Orchestrator Handler: Set client response Content-Type to text/event-stream.")

		// Type assert response to io.ReadCloser
		responseBody, ok := response.(io.ReadCloser)
		if !ok {
			h.logger.Error("Orchestrator Handler: Invalid streaming response type from service: %T. Expected io.ReadCloser.", response)
			return echo.NewHTTPError(http.StatusInternalServerError, "Invalid streaming response from service")
		}
		defer responseBody.Close()
		h.logger.Info("Orchestrator Handler: Successfully type-asserted service response to io.ReadCloser.")

		// Stream the response
		buffer := make([]byte, 1024) // Standard buffer size for streaming
		for {
			n, readErr := responseBody.Read(buffer)
			if n > 0 {
				// Log first few bytes of the chunk for inspection
				// Be cautious with logging potentially large/sensitive data in production
				// logData := buffer[:n]
				// if n > 64 { // Log only a snippet
				// 	logData = buffer[:64]
				// }
				// h.logger.Debug("Orchestrator Handler: Streaming %d bytes to client. Data snippet: %s", n, string(logData))

				_, writeErr := c.Response().Write(buffer[:n])
				if writeErr != nil {
					h.logger.Error("Orchestrator Handler: Error writing to client response: %v", writeErr)
					// It's hard to recover here as headers are already sent.
					// The connection might be closed by the client.
					return writeErr // or simply break
				}
				c.Response().Flush() // Ensure data is sent to the client
			}

			if readErr == io.EOF {
				h.logger.Info("Orchestrator Handler: Reached EOF from service response stream.")
				break
			}
			if readErr != nil {
				h.logger.Error("Orchestrator Handler: Error reading from service response stream: %v", readErr)
				// We might want to send an SSE error event to the client here if possible,
				// but the stream might be broken.
				return readErr // or break
			}
		}
		h.logger.Info("Orchestrator Handler: Finished streaming response to client.")
		return nil
	}

	// Handle non-streaming response
	h.logger.Info("Orchestrator Handler: Handling non-streaming response. Response type from service: %T", response)
	// Extract response_data from provider response
	if responseMap, ok := response.(map[string]interface{}); ok {
		if responseData, ok := responseMap["response_data"]; ok {
			return c.JSON(http.StatusOK, responseData)
		}
	}

	return c.JSON(http.StatusOK, response)
}
