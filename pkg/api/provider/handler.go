package provider

import (
	"database/sql"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/sentnl/inferoute-node/pkg/common"
)

// Handler handles HTTP requests for provider management
type Handler struct {
	service   *Service
	logger    *common.Logger
	db        *sql.DB
	validator *validator.Validate
}

// NewHandler creates a new provider management handler
func NewHandler(service *Service, logger *common.Logger, db *sql.DB) *Handler {
	return &Handler{
		service:   service,
		logger:    logger,
		db:        db,
		validator: validator.New(),
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

	// Health push endpoint
	g.POST("/health", h.PushHealth)

	// Pause management
	g.PUT("/pause", h.UpdatePauseStatus)

	// HMAC validation
	g.POST("/validate_hmac", h.ValidateHMAC)

	// Add the new filter route
	g.GET("/health/providers/filter", h.FilterProviders)
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

// FilterProvidersRequest represents the query parameters for filtering providers
type FilterProvidersRequest struct {
	ModelName string  `query:"model_name" validate:"required"`
	Tier      *int    `query:"tier"`
	MaxCost   float64 `query:"max_cost" validate:"required,gt=0"`
}

// FilterProvidersResponse represents a provider in the filtered list
type FilterProvidersResponse struct {
	ProviderID   string  `json:"provider_id"`
	Username     string  `json:"username"`
	Tier         int     `json:"tier"`
	HealthStatus string  `json:"health_status"`
	Latency      int     `json:"latency_ms"`
	InputCost    float64 `json:"input_cost"`
	OutputCost   float64 `json:"output_cost"`
}

// @Summary Filter providers by model, tier, health status, and cost
// @Description Get a list of healthy providers offering a specific model within cost constraints
// @Tags providers
// @Accept json
// @Produce json
// @Param model_name query string true "Name of the model to filter by"
// @Param tier query int false "Optional tier to filter by"
// @Param max_cost query number true "Maximum cost per token (applies to both input and output)"
// @Success 200 {array} FilterProvidersResponse
// @Failure 400 {object} common.ErrorResponse
// @Failure 500 {object} common.ErrorResponse
// @Router /api/health/providers/filter [get]
func (h *Handler) FilterProviders(c echo.Context) error {
	var req FilterProvidersRequest
	if err := c.Bind(&req); err != nil {
		return common.NewBadRequestError("invalid request parameters")
	}

	if err := h.validator.Struct(req); err != nil {
		return common.NewBadRequestError("validation failed")
	}

	// Build the query based on parameters
	query := `
		WITH healthy_providers AS (
			SELECT ps.provider_id, ps.tier, ps.health_status, 
				   u.username,
				   COALESCE((
					   SELECT latency_ms 
					   FROM provider_health_history 
					   WHERE provider_id = ps.provider_id 
					   ORDER BY health_check_time DESC 
					   LIMIT 1
				   ), 0) as latency_ms
			FROM provider_status ps
			JOIN users u ON u.id = ps.provider_id
			WHERE ps.health_status = 'green'
			AND ps.is_available = true
			AND ($1::int IS NULL OR ps.tier = $1)
		)
		SELECT 
			hp.provider_id,
			hp.username,
			hp.tier,
			hp.health_status,
			hp.latency_ms,
			pm.input_price_per_token,
			pm.output_price_per_token
		FROM healthy_providers hp
		JOIN provider_models pm ON pm.provider_id = hp.provider_id
		WHERE pm.model_name = $2
		AND pm.input_price_per_token <= $3
		AND pm.output_price_per_token <= $3
		ORDER BY hp.tier ASC, hp.latency_ms ASC;
	`

	rows, err := h.db.QueryContext(c.Request().Context(), query, req.Tier, req.ModelName, req.MaxCost)
	if err != nil {
		return common.NewInternalError("database error", err)
	}
	defer rows.Close()

	var providers []FilterProvidersResponse
	for rows.Next() {
		var p FilterProvidersResponse
		err := rows.Scan(
			&p.ProviderID,
			&p.Username,
			&p.Tier,
			&p.HealthStatus,
			&p.Latency,
			&p.InputCost,
			&p.OutputCost,
		)
		if err != nil {
			return common.NewInternalError("error scanning results", err)
		}
		providers = append(providers, p)
	}

	if err = rows.Err(); err != nil {
		return common.NewInternalError("error iterating results", err)
	}

	return c.JSON(http.StatusOK, providers)
}

// @Summary Validate HMAC
// @Description Validates an HMAC for a provider and returns the associated request data
// @Tags Provider
// @Accept json
// @Produce json
// @Param request body ValidateHMACRequest true "HMAC to validate"
// @Success 200 {object} ValidateHMACResponse
// @Failure 400 {object} common.ErrorResponse
// @Failure 401 {object} common.ErrorResponse
// @Failure 500 {object} common.ErrorResponse
// @Router /api/provider/validate_hmac [post]
func (h *Handler) ValidateHMAC(c echo.Context) error {
	var req ValidateHMACRequest
	if err := c.Bind(&req); err != nil {
		return common.NewBadRequestError("invalid request body")
	}

	if err := h.validator.Struct(req); err != nil {
		return common.NewBadRequestError("validation failed")
	}

	// Get provider ID from auth context
	providerID, ok := c.Get("user_id").(uuid.UUID)
	if !ok {
		return common.NewUnauthorizedError("provider ID not found in context")
	}

	response, err := h.service.ValidateHMAC(c.Request().Context(), providerID, req)
	if err != nil {
		return err // Service errors are already properly formatted
	}

	if !response.Valid {
		return c.JSON(http.StatusUnauthorized, response)
	}

	return c.JSON(http.StatusOK, response)
}
