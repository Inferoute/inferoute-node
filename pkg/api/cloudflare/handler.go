package cloudflare

import (
	"errors"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/sentnl/inferoute-node/pkg/common"
)

// Handler handles HTTP requests for the Cloudflare tunnel service.
type Handler struct {
	service *Service
	logger  *common.Logger
	// validator *validator.Validate // Assuming you have a custom validator setup as in main.go
}

// NewHandler creates a new Cloudflare tunnel handler.
func NewHandler(service *Service, logger *common.Logger) *Handler {
	return &Handler{
		service: service,
		logger:  logger,
		// validator: validator.New(), // Initialize if needed
	}
}

// RegisterRoutes registers the Cloudflare tunnel routes.
func (h *Handler) RegisterRoutes(e *echo.Echo, internalKeyMiddleware echo.MiddlewareFunc) {
	// Group for external Cloudflare tunnel APIs (provider API key protected)
	// These endpoints can be accessed from outside the network by provider clients
	providerGroup := e.Group("/api/cloudflare")
	providerGroup.POST("/tunnel/request", h.RequestTunnel)
	providerGroup.POST("/tunnel/refresh-token", h.RefreshToken)

	// Group for internal Cloudflare tunnel APIs (internal API key protected)
	// These endpoints can only be accessed from within the network
	internalGroup := e.Group("/api/cloudflare", internalKeyMiddleware)
	internalGroup.POST("/tunnel/cleanup", h.CleanupTunnel)
}

// RequestTunnel handles POST /api/cloudflare/tunnel/request
// @Summary Request or get details of a Cloudflare tunnel for a provider
// @Description Authenticates provider via API key, then creates/updates a Cloudflare tunnel and DNS, returning token and hostname.
// @Tags cloudflare
// @Accept json
// @Produce json
// @Security InternalKey
// @Param request body RequestTunnelRequest true "Tunnel request details including API key and service URL"
// @Success 200 {object} RequestTunnelResponse
// @Failure 400 {object} common.ErrorResponse "Invalid input"
// @Failure 401 {object} common.ErrorResponse "Unauthorized - API key invalid or not a provider"
// @Failure 500 {object} common.ErrorResponse "Internal server error"
// @Router /api/cloudflare/tunnel/request [post]
func (h *Handler) RequestTunnel(c echo.Context) error {
	req := new(RequestTunnelRequest)
	if err := c.Bind(req); err != nil {
		return common.ErrInvalidInput(err)
	}
	if err := c.Validate(req); err != nil {
		return common.ErrInvalidInput(err)
	}

	authHeader := c.Request().Header.Get("Authorization")
	if authHeader == "" {
		return common.ErrUnauthorized(errors.New("missing Authorization header"))
	}

	splitToken := strings.Split(authHeader, "Bearer ")
	if len(splitToken) != 2 {
		return common.ErrUnauthorized(errors.New("invalid Authorization header format"))
	}
	apiKey := splitToken[1]

	resp, err := h.service.RequestTunnel(c.Request().Context(), apiKey, req.ServiceURL)
	if err != nil {
		// Specific error handling based on service error type can be added here
		// For now, pass through the error from the service layer, which should be an AppError
		return err
	}

	return c.JSON(http.StatusOK, resp)
}

// RefreshToken handles POST /api/cloudflare/tunnel/refresh-token
// @Summary Refresh or retrieve the token for an existing Cloudflare tunnel
// @Description Allows a provider client to get a new/current token for their tunnel.
// @Tags cloudflare
// @Accept json
// @Produce json
// @Security InternalKey
// @Param request body RefreshTokenRequest true "Refresh token request including API key and refresh flag"
// @Success 200 {object} RefreshTokenResponse
// @Failure 400 {object} common.ErrorResponse "Invalid input"
// @Failure 401 {object} common.ErrorResponse "Unauthorized - API key invalid or not a provider"
// @Failure 404 {object} common.ErrorResponse "Not Found - Tunnel does not exist for this provider"
// @Failure 500 {object} common.ErrorResponse "Internal server error"
// @Router /api/cloudflare/tunnel/refresh-token [post]
func (h *Handler) RefreshToken(c echo.Context) error {
	req := new(RefreshTokenRequest)
	if err := c.Bind(req); err != nil {
		return common.ErrInvalidInput(err)
	}

	// Note: Validation for RefreshTokenRequest might be minimal if only 'refresh' is present
	// and its usage is just a boolean flag. If it becomes more complex, add c.Validate(req).

	authHeader := c.Request().Header.Get("Authorization")
	if authHeader == "" {
		return common.ErrUnauthorized(errors.New("missing Authorization header"))
	}

	splitToken := strings.Split(authHeader, "Bearer ")
	if len(splitToken) != 2 {
		return common.ErrUnauthorized(errors.New("invalid Authorization header format"))
	}
	apiKey := splitToken[1]

	resp, err := h.service.RefreshToken(c.Request().Context(), apiKey)
	if err != nil {
		return err
	}

	return c.JSON(http.StatusOK, resp)
}

// CleanupTunnel handles tunnel cleanup requests (internal only)
func (h *Handler) CleanupTunnel(c echo.Context) error {
	req := new(CleanupTunnelRequest)
	if err := c.Bind(req); err != nil {
		return common.ErrInvalidInput(err)
	}
	if err := c.Validate(req); err != nil {
		return common.ErrInvalidInput(err)
	}

	// No need to check Authorization header since this is protected by internal middleware
	// and should only be called by internal services like the scheduler

	resp, err := h.service.BulkCleanupTunnels(c.Request().Context(), req.Days)
	if err != nil {
		return err
	}

	return c.JSON(http.StatusOK, resp)
}
