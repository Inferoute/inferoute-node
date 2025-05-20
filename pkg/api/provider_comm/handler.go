package provider_comm

import (
	"bytes"
	"context"
	"io"
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

	responseBody, err := h.service.SendRequest(c.Request().Context(), req)
	if err != nil {
		return err // Service errors are already properly formatted
	}
	defer responseBody.Close()

	// Create a buffer to store chunks for logging
	var logBuffer bytes.Buffer
	teeReader := io.TeeReader(responseBody, &logBuffer)

	// Copy headers from provider response
	c.Response().Header().Set("Content-Type", "application/json")
	c.Response().Header().Set("Transfer-Encoding", "chunked")
	c.Response().WriteHeader(http.StatusOK)

	// Create a custom reader that counts chunks
	chunkCount := 0
	countingReader := &countingReader{
		reader: teeReader,
		onChunk: func() {
			chunkCount++
			// Update the stream count in the context
			ctx := c.Request().Context()
			ctx = context.WithValue(ctx, "stream_count", chunkCount)
			c.SetRequest(c.Request().WithContext(ctx))
		},
	}

	// Stream the response body directly to the client while also logging and counting chunks
	_, err = io.Copy(c.Response(), countingReader)

	// Log what was sent to the client
	h.logger.Info("Raw response sent to client: %s", logBuffer.String())
	h.logger.Info("Total chunks streamed: %d", chunkCount)

	return err
}

// countingReader is a custom reader that counts chunks
type countingReader struct {
	reader  io.Reader
	onChunk func()
}

func (r *countingReader) Read(p []byte) (n int, err error) {
	n, err = r.reader.Read(p)
	if n > 0 {
		// Only count actual data chunks, not empty ones
		if !bytes.Equal(p[:n], []byte("data: [DONE]\n\n")) {
			r.onChunk()
		}
	}
	return n, err
}
