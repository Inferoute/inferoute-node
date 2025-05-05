package provider_comm

import (
	"net/http"

	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
	"github.com/sentnl/inferoute-node/internal/db"
	"github.com/sentnl/inferoute-node/pkg/common"
)

// Handler handles HTTP requests for provider communication
type Handler struct {
	service   *Service
	validator *validator.Validate
	logger    *common.Logger
}

// NewHandler creates a new provider communication handler
func NewHandler(db *db.DB, logger *common.Logger) *Handler {
	return &Handler{
		service:   NewService(db, logger),
		validator: validator.New(),
		logger:    logger,
	}
}

// Register registers the provider communication routes
func (h *Handler) Register(e *echo.Echo) {
	g := e.Group("/api/provider-comms")

	// Routes for provider communication
	g.POST("/send_requests", h.SendRequest)
}

// @Summary Send request to provider
// @Description Sends a request to a specific provider and waits for response
// @Tags Provider Communication
// @Accept json
// @Produce json
// @Param request body SendRequestRequest true "Request details"
// @Success 200 {object} interface{} "Provider response"
// @Failure 400 {object} common.ErrorResponse
// @Failure 404 {object} common.ErrorResponse
// @Failure 500 {object} common.ErrorResponse
// @Router /api/send_requests [post]
func (h *Handler) SendRequest(c echo.Context) error {
	var req SendRequestRequest
	if err := c.Bind(&req); err != nil {
		return common.NewBadRequestError("invalid request body")
	}

	if err := h.validator.Struct(req); err != nil {
		return common.NewBadRequestError("validation failed")
	}

	response, err := h.service.SendRequest(c.Request().Context(), req)
	if err != nil {
		return err // Service errors are already properly formatted
	}

	return c.JSON(http.StatusOK, response)
}
