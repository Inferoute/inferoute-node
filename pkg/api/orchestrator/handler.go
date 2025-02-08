package orchestrator

import (
	"context"
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

	// Parse request body
	var req OpenAIRequest
	if err := c.Bind(&req); err != nil {
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
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to process request")
	}

	return c.JSON(http.StatusOK, response)
}
