package tokenizer

import (
	"fmt"

	"github.com/labstack/echo/v4"
	"github.com/sentnl/inferoute-node/pkg/common"
)

// Handler handles HTTP requests for tokenization
type Handler struct {
	service *Service
}

// NewHandler creates a new handler
func NewHandler(service *Service) *Handler {
	return &Handler{
		service: service,
	}
}

// Register registers the routes
func (h *Handler) Register(e *echo.Echo) {
	e.POST("/api/tokenize", h.handleTokenize)
}

// handleTokenize handles the HTTP request for tokenization
func (h *Handler) handleTokenize(c echo.Context) error {
	var req TokenizeRequest
	if err := c.Bind(&req); err != nil {
		return common.ErrBadRequest(fmt.Errorf("invalid request body: %w", err))
	}

	// Validate request
	if req.InputText == "" && req.OutputText == "" {
		return common.ErrBadRequest(fmt.Errorf("at least one of input_text or output_text must be provided"))
	}

	// Process request
	response, err := h.service.Tokenize(c.Request().Context(), &req)
	if err != nil {
		return fmt.Errorf("failed to tokenize text: %w", err)
	}

	return c.JSON(200, response)
}
