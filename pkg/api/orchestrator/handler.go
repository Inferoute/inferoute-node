package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
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
	response, err := h.service.ProcessRequest(c.Request().Context(), consumerID, &req)
	if err != nil {
		h.logger.Error("Failed to process request: %v", err)

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

	// Extract response_data from provider response
	if responseMap, ok := response.(map[string]interface{}); ok {
		if responseData, ok := responseMap["response_data"]; ok {
			return c.JSON(http.StatusOK, responseData)
		}
	}

	return c.JSON(http.StatusOK, response)
}
