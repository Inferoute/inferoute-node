package health

import (
	"database/sql"
	"net/http"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/sentnl/inferoute-node/internal/db"
	"github.com/sentnl/inferoute-node/pkg/common"
)

// Handler handles HTTP requests for provider health management
type Handler struct {
	service   *Service
	validator *validator.Validate
	logger    *common.Logger
}

// NewHandler creates a new provider health handler
func NewHandler(sqlDB *sql.DB, logger *common.Logger) *Handler {
	dbWrapper := &db.DB{DB: sqlDB}
	return &Handler{
		service:   NewService(dbWrapper, logger, nil),
		validator: validator.New(),
		logger:    logger,
	}
}

// Register registers the provider health routes
func (h *Handler) Register(e *echo.Echo) {
	g := e.Group("/api/health")

	// Get healthy providers
	g.GET("/providers/healthy", h.GetHealthyNodes)

	// Get provider health status
	g.GET("/provider/:provider_id", h.GetProviderHealth)

	// Filter for Healthy providers
	g.GET("/providers/filter", h.FilterProviders)

	// Manual triggers - internal only
	internalGroup := g.Group("/providers", common.InternalOnly())
	internalGroup.POST("/update-tiers", h.TriggerUpdateTiers)
	internalGroup.POST("/check-stale", h.TriggerCheckStale)
}

// @Summary Get all healthy nodes
// @Description Get a list of all providers with green health status
// @Tags Health
// @Produce json
// @Success 200 {array} HealthyNodeResponse
// @Failure 500 {object} common.ErrorResponse
// @Router /api/health/providers/healthy [get]
func (h *Handler) GetHealthyNodes(c echo.Context) error {
	query := `
		SELECT 
			ps.provider_id,
			u.username,
			ps.tier,
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
		ORDER BY ps.tier ASC, latency_ms ASC`

	rows, err := h.service.db.QueryContext(c.Request().Context(), query)
	if err != nil {
		return common.NewInternalError("database error", err)
	}
	defer rows.Close()

	var nodes []HealthyNodeResponse
	for rows.Next() {
		var node HealthyNodeResponse
		err := rows.Scan(
			&node.ProviderID,
			&node.Username,
			&node.Tier,
			&node.Latency,
		)
		if err != nil {
			return common.NewInternalError("error scanning results", err)
		}
		nodes = append(nodes, node)
	}

	return c.JSON(http.StatusOK, nodes)
}

// @Summary Get provider health status
// @Description Get detailed health status for a specific provider
// @Tags Health
// @Produce json
// @Param provider_id path string true "Provider ID"
// @Success 200 {object} ProviderHealthResponse
// @Failure 404 {object} common.ErrorResponse
// @Failure 500 {object} common.ErrorResponse
// @Router /api/health/provider/{provider_id} [get]
func (h *Handler) GetProviderHealth(c echo.Context) error {
	providerID, err := uuid.Parse(c.Param("provider_id"))
	if err != nil {
		return common.NewBadRequestError("invalid provider ID")
	}

	query := `
		SELECT 
			ps.provider_id,
			u.username,
			ps.health_status,
			ps.tier,
			ps.is_available,
			COALESCE((
				SELECT latency_ms 
				FROM provider_health_history 
				WHERE provider_id = ps.provider_id 
				ORDER BY health_check_time DESC 
				LIMIT 1
			), 0) as latency_ms,
			ps.last_health_check
		FROM provider_status ps
		JOIN users u ON u.id = ps.provider_id
		WHERE ps.provider_id = $1`

	var health ProviderHealthResponse
	err = h.service.db.QueryRowContext(c.Request().Context(), query, providerID).Scan(
		&health.ProviderID,
		&health.Username,
		&health.HealthStatus,
		&health.Tier,
		&health.IsAvailable,
		&health.Latency,
		&health.LastHealthCheck,
	)

	if err == sql.ErrNoRows {
		return common.NewNotFoundError("provider not found")
	}
	if err != nil {
		return common.NewInternalError("database error", err)
	}

	return c.JSON(http.StatusOK, health)
}

// Types for responses
type HealthyNodeResponse struct {
	ProviderID string `json:"provider_id"`
	Username   string `json:"username"`
	Tier       int    `json:"tier"`
	Latency    int    `json:"latency_ms"`
}

type ProviderHealthResponse struct {
	ProviderID      string    `json:"provider_id"`
	Username        string    `json:"username"`
	HealthStatus    string    `json:"health_status"`
	Tier            int       `json:"tier"`
	IsAvailable     bool      `json:"is_available"`
	Latency         int       `json:"latency_ms"`
	LastHealthCheck time.Time `json:"last_health_check"`
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
// @Tags Health
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
			pm.input_price_tokens,
			pm.output_price_tokens
		FROM healthy_providers hp
		JOIN provider_models pm ON pm.provider_id = hp.provider_id
		WHERE pm.model_name = $2
		AND pm.input_price_tokens <= $3
		AND pm.output_price_tokens <= $3
		ORDER BY hp.tier ASC, hp.latency_ms ASC;
	`

	rows, err := h.service.db.QueryContext(c.Request().Context(), query, req.Tier, req.ModelName, req.MaxCost)
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

// @Summary Manually trigger provider tier updates
// @Description Updates provider tiers based on their health history
// @Tags Health
// @Produce json
// @Success 200 {object} TriggerHealthChecksResponse
// @Failure 500 {object} common.ErrorResponse
// @Router /api/health/providers/update-tiers [post]
func (h *Handler) TriggerUpdateTiers(c echo.Context) error {
	updatedCount, err := h.service.UpdateProviderTiers(c.Request().Context())
	if err != nil {
		return common.NewInternalError("failed to update provider tiers", err)
	}

	return c.JSON(http.StatusOK, TriggerHealthChecksResponse{
		TiersUpdated: updatedCount,
	})
}

// @Summary Manually trigger stale provider checks
// @Description Checks and updates providers that haven't sent a health check recently
// @Tags Health
// @Produce json
// @Success 200 {object} TriggerHealthChecksResponse
// @Failure 500 {object} common.ErrorResponse
// @Router /api/health/providers/check-stale [post]
func (h *Handler) TriggerCheckStale(c echo.Context) error {
	updatedCount, err := h.service.CheckStaleProviders(c.Request().Context())
	if err != nil {
		return common.NewInternalError("failed to check stale providers", err)
	}

	return c.JSON(http.StatusOK, TriggerHealthChecksResponse{
		ProvidersUpdated: updatedCount,
	})
}
