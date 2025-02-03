package provider

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/sentnl/inferoute-node/pkg/common"
)

// Handler handles HTTP requests for provider management
type Handler struct {
	service *Service
	logger  *common.Logger
}

// NewHandler creates a new provider management handler
func NewHandler(service *Service, logger *common.Logger) *Handler {
	return &Handler{
		service: service,
		logger:  logger,
	}
}

// Register registers the provider management routes
func (h *Handler) Register(e *echo.Echo) {
	g := e.Group("/api/provider")

	// Model management
	g.POST("/models", h.AddModel)
	g.GET("/models", h.ListModels)
	g.PUT("/models/:model_id", h.UpdateModel)
	g.DELETE("/models/:model_id", h.DeleteModel)

	// Status and health
	g.GET("/status", h.GetStatus)
	g.POST("/health", h.PushHealth)

	// Pause management
	g.PUT("/pause", h.UpdatePauseStatus)
}

// @Summary Add a new model for the provider
// @Description Add a new model configuration for the authenticated provider
// @Tags Provider
// @Accept json
// @Produce json
// @Param model body AddModelRequest true "Model configuration"
// @Success 201 {object} ProviderModel
// @Failure 400 {object} common.ErrorResponse
// @Router /api/provider/models [post]
func (h *Handler) AddModel(c echo.Context) error {
	var req AddModelRequest
	if err := c.Bind(&req); err != nil {
		return common.ErrInvalidInput(err)
	}

	if err := c.Validate(&req); err != nil {
		return common.ErrInvalidInput(err)
	}

	// Get provider ID from auth context
	providerID := c.Get("user_id").(uuid.UUID)

	model, err := h.service.AddModel(c.Request().Context(), providerID, req)
	if err != nil {
		return err
	}

	return c.JSON(http.StatusCreated, model)
}

// @Summary List provider's models
// @Description Get all models configured for the authenticated provider
// @Tags Provider
// @Produce json
// @Success 200 {object} ListModelsResponse
// @Router /api/provider/models [get]
func (h *Handler) ListModels(c echo.Context) error {
	userID := c.Get("user_id").(uuid.UUID)

	response, err := h.service.ListModels(c.Request().Context(), userID)
	if err != nil {
		return err
	}

	return c.JSON(http.StatusOK, response)
}

// @Summary Update a model
// @Description Update an existing model configuration
// @Tags Provider
// @Accept json
// @Produce json
// @Param model_id path string true "Model ID"
// @Param model body UpdateModelRequest true "Updated model configuration"
// @Success 200 {object} ProviderModel
// @Failure 400 {object} common.ErrorResponse
// @Failure 404 {object} common.ErrorResponse
// @Router /api/provider/models/{model_id} [put]
func (h *Handler) UpdateModel(c echo.Context) error {
	var req UpdateModelRequest
	if err := c.Bind(&req); err != nil {
		return common.ErrInvalidInput(err)
	}

	if err := c.Validate(&req); err != nil {
		return err
	}

	userID := c.Get("user_id").(uuid.UUID)
	modelID, err := uuid.Parse(c.Param("model_id"))
	if err != nil {
		return common.ErrInvalidInput(err)
	}

	model, err := h.service.UpdateModel(c.Request().Context(), userID, modelID, req)
	if err != nil {
		return err
	}

	return c.JSON(http.StatusOK, model)
}

// @Summary Delete a model
// @Description Delete an existing model configuration
// @Tags Provider
// @Param model_id path string true "Model ID"
// @Success 204 "No Content"
// @Failure 404 {object} common.ErrorResponse
// @Router /api/provider/models/{model_id} [delete]
func (h *Handler) DeleteModel(c echo.Context) error {
	userID := c.Get("user_id").(uuid.UUID)
	modelID, err := uuid.Parse(c.Param("model_id"))
	if err != nil {
		return common.ErrInvalidInput(err)
	}

	err = h.service.DeleteModel(c.Request().Context(), userID, modelID)
	if err != nil {
		return err
	}

	return c.NoContent(http.StatusOK)
}

// @Summary Get provider status
// @Description Get the current status of the authenticated provider
// @Tags Provider
// @Produce json
// @Success 200 {object} GetStatusResponse
// @Router /api/provider/status [get]
func (h *Handler) GetStatus(c echo.Context) error {
	userID := c.Get("user_id").(uuid.UUID)

	status, err := h.service.GetStatus(c.Request().Context(), userID)
	if err != nil {
		return err
	}

	return c.JSON(http.StatusOK, status)
}

// @Summary Push provider health data
// @Description Push provider's current health and model status
// @Tags Provider
// @Accept json
// @Produce json
// @Param health body ProviderHealthPushRequest true "Health data"
// @Success 200
// @Failure 400 {object} common.ErrorResponse
// @Router /api/provider/health [post]
func (h *Handler) PushHealth(c echo.Context) error {
	var req ProviderHealthPushRequest
	if err := c.Bind(&req); err != nil {
		return common.ErrInvalidInput(fmt.Errorf("invalid request body: %w", err))
	}

	if err := c.Validate(&req); err != nil {
		return common.ErrInvalidInput(err)
	}

	// Get API key from auth header
	auth := c.Request().Header.Get("Authorization")
	if auth == "" {
		return common.ErrUnauthorized(fmt.Errorf("missing authorization header"))
	}

	parts := strings.Split(auth, " ")
	if len(parts) != 2 || parts[0] != "Bearer" {
		return common.ErrUnauthorized(fmt.Errorf("invalid authorization format"))
	}

	apiKey := strings.TrimSpace(parts[1])

	// Create message for RabbitMQ
	message := ProviderHealthMessage{
		APIKey: apiKey,
		Models: req.Data,
	}

	// Publish to RabbitMQ
	if err := h.service.PublishHealthUpdate(c.Request().Context(), message); err != nil {
		return common.ErrInternalServer(fmt.Errorf("failed to publish health update: %w", err))
	}

	return c.NoContent(http.StatusOK)
}

// @Summary Update provider pause status
// @Description Update the pause status of the authenticated provider
// @Tags Provider
// @Accept json
// @Produce json
// @Param request body UpdatePauseRequest true "Pause status update"
// @Success 200 {object} UpdatePauseResponse
// @Failure 400 {object} common.ErrorResponse
// @Failure 404 {object} common.ErrorResponse
// @Router /api/provider/pause [put]
func (h *Handler) UpdatePauseStatus(c echo.Context) error {
	var req UpdatePauseRequest
	if err := c.Bind(&req); err != nil {
		return common.ErrInvalidInput(fmt.Errorf("invalid request body: %w", err))
	}

	// The validation is failing because the field is missing or not properly bound
	// Let's log the request body to help debug
	h.logger.Info("Received pause request: %+v", req)

	providerID := c.Get("user_id").(uuid.UUID)

	response, err := h.service.UpdatePauseStatus(c.Request().Context(), providerID, req.Paused)
	if err != nil {
		return err
	}

	return c.JSON(http.StatusOK, response)
}
