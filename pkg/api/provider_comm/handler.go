package provider_comm

import (
	"bytes"
	"io"
	"net/http"
	"strings"

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

	responseBody, err := h.service.SendRequest(c.Request().Context(), req)
	if err != nil {
		return err // Service errors are already properly formatted
	}
	defer responseBody.Close()

	// Create a buffer to store chunks for logging
	var logBuffer bytes.Buffer
	teeReader := io.TeeReader(responseBody, &logBuffer)

	// Set Content-Type based on what the orchestrator (our client) accepts
	acceptHeader := c.Request().Header.Get("Accept")
	h.logger.Info("Provider-Comms Handler: Received Accept header from orchestrator: '%s'", acceptHeader)

	if strings.Contains(acceptHeader, "text/event-stream") {
		c.Response().Header().Set("Content-Type", "text/event-stream")
		h.logger.Info("Provider-Comms Handler: Setting response Content-Type to text/event-stream")
	} else {
		c.Response().Header().Set("Content-Type", "application/json") // Default or copy from provider if known
		h.logger.Info("Provider-Comms Handler: Setting response Content-Type to application/json")
	}
	c.Response().Header().Set("Transfer-Encoding", "chunked")
	c.Response().WriteHeader(http.StatusOK)
	h.logger.Info("Provider-Comms Handler: Final response headers set. Streaming to orchestrator.")

	// Stream the response body directly to the client while also logging
	bytesCopied, err := io.Copy(c.Response(), teeReader)
	h.logger.Info("Provider-Comms Handler: Finished streaming to orchestrator. Bytes copied: %d", bytesCopied)
	if err != nil {
		h.logger.Error("Provider-Comms Handler: Error during io.Copy to orchestrator: %v", err)
	}

	// Log the captured stream data (be careful with large responses in production)
	// if logBuffer.Len() > 0 {
	// 	h.logger.Debug("Provider-Comms Handler: Captured stream data for orchestrator: %s", logBuffer.String())
	// } else {
	// 	h.logger.Debug("Provider-Comms Handler: No data captured in logBuffer for orchestrator (stream might have been empty or read directly). Bytes copied: %d", bytesCopied)
	// }

	return err
}
