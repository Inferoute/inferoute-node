package auth

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/sentnl/inferoute-node/pkg/common"
)

// Handler handles HTTP requests for the authentication service
type Handler struct {
	service *Service
	logger  *common.Logger
}

// NewHandler creates a new authentication handler
func NewHandler(service *Service, logger *common.Logger) *Handler {
	return &Handler{
		service: service,
		logger:  logger,
	}
}

// RegisterRoutes registers the authentication routes
func (h *Handler) RegisterRoutes(e *echo.Echo) {
	g := e.Group("/api/auth")

	g.POST("/users", h.CreateUser)
	g.POST("/validate", h.ValidateAPIKey)
	g.POST("/hold", h.HoldDeposit)
	g.POST("/release", h.ReleaseHold)
}

// CreateUser handles the creation of a new user
// @Summary Create a new user
// @Description Create a new user with a username, API key and initial balance
// @Tags auth
// @Accept json
// @Produce json
// @Param request body CreateUserRequest true "User creation request with username and initial balance"
// @Success 201 {object} CreateUserResponse
// @Failure 400 {object} common.ErrorResponse
// @Failure 500 {object} common.ErrorResponse
// @Router /api/auth/users [post]
func (h *Handler) CreateUser(c echo.Context) error {
	var req CreateUserRequest
	if err := c.Bind(&req); err != nil {
		return common.ErrInvalidInput(err)
	}

	if err := c.Validate(&req); err != nil {
		return common.ErrInvalidInput(err)
	}

	resp, err := h.service.CreateUser(c.Request().Context(), req)
	if err != nil {
		return err
	}

	return c.JSON(http.StatusCreated, resp)
}

// ValidateAPIKey handles API key validation
// @Summary Validate an API key
// @Description Validate an API key and return user information
// @Tags auth
// @Accept json
// @Produce json
// @Param request body ValidateAPIKeyRequest true "API key validation request"
// @Success 200 {object} ValidateAPIKeyResponse
// @Failure 400 {object} common.ErrorResponse
// @Failure 500 {object} common.ErrorResponse
// @Router /api/auth/validate [post]
func (h *Handler) ValidateAPIKey(c echo.Context) error {
	var req ValidateAPIKeyRequest
	if err := c.Bind(&req); err != nil {
		return common.ErrInvalidInput(err)
	}

	if err := c.Validate(&req); err != nil {
		return common.ErrInvalidInput(err)
	}

	resp, err := h.service.ValidateAPIKey(c.Request().Context(), req)
	if err != nil {
		return err
	}

	return c.JSON(http.StatusOK, resp)
}

// HoldDeposit handles placing a hold on a user's balance
// @Summary Place a hold on a user's balance
// @Description Place a hold on a user's balance for a pending transaction
// @Tags auth
// @Accept json
// @Produce json
// @Param request body HoldDepositRequest true "Hold deposit request"
// @Success 200 {object} HoldDepositResponse
// @Failure 400 {object} common.ErrorResponse
// @Failure 402 {object} common.ErrorResponse
// @Failure 404 {object} common.ErrorResponse
// @Failure 500 {object} common.ErrorResponse
// @Router /api/auth/hold [post]
func (h *Handler) HoldDeposit(c echo.Context) error {
	var req HoldDepositRequest
	if err := c.Bind(&req); err != nil {
		return common.ErrInvalidInput(err)
	}

	if err := c.Validate(&req); err != nil {
		return common.ErrInvalidInput(err)
	}

	resp, err := h.service.HoldDeposit(c.Request().Context(), req)
	if err != nil {
		return err
	}

	return c.JSON(http.StatusOK, resp)
}

// ReleaseHold handles releasing a hold on a user's balance
// @Summary Release a hold on a user's balance
// @Description Release a hold on a user's balance and return the funds to available
// @Tags auth
// @Accept json
// @Produce json
// @Param request body ReleaseHoldRequest true "Release hold request"
// @Success 200 {object} ReleaseHoldResponse
// @Failure 400 {object} common.ErrorResponse
// @Failure 404 {object} common.ErrorResponse
// @Failure 500 {object} common.ErrorResponse
// @Router /api/auth/release [post]
func (h *Handler) ReleaseHold(c echo.Context) error {
	var req ReleaseHoldRequest
	if err := c.Bind(&req); err != nil {
		return common.ErrInvalidInput(err)
	}

	if err := c.Validate(&req); err != nil {
		return common.ErrInvalidInput(err)
	}

	resp, err := h.service.ReleaseHold(c.Request().Context(), req)
	if err != nil {
		return err
	}

	return c.JSON(http.StatusOK, resp)
}
