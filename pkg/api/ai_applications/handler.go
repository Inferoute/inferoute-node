package ai_applications

import (
	"database/sql"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/sentnl/inferoute-node/internal/db"
	"github.com/sentnl/inferoute-node/pkg/common"
)

// Handler handles HTTP requests for AI applications
type Handler struct {
	service *Service
	logger  *common.Logger
}

// NewHandler creates a new AI applications handler
func NewHandler(sqlDB *sql.DB, logger *common.Logger) *Handler {
	dbWrapper := &db.DB{DB: sqlDB}
	return &Handler{
		service: NewService(dbWrapper, logger),
		logger:  logger,
	}
}

// Register registers the AI applications routes
func (h *Handler) Register(e *echo.Echo) {
	// OpenAI-compatible models endpoint
	e.GET("/v1/models", h.GetModels)
}

// @Summary Get available models
// @Description Get a list of available models in OpenAI-compatible format
// @Tags Models
// @Produce json
// @Success 200 {object} ModelResponse
// @Failure 500 {object} common.ErrorResponse
// @Router /v1/models [get]
func (h *Handler) GetModels(c echo.Context) error {
	response, err := h.service.GetTopModels(c.Request().Context())
	if err != nil {
		h.logger.Error("Error getting models: %v", err)
		return common.NewInternalError("failed to fetch models", err)
	}

	return c.JSON(http.StatusOK, response)
}
