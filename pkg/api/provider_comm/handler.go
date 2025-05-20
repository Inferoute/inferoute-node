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

	// Create a new context with a stream count value
	ctx := context.WithValue(c.Request().Context(), "stream_count", 0)

	responseBody, err := h.service.SendRequest(ctx, req)
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
			ctx = context.WithValue(ctx, "stream_count", chunkCount)
			// Update the request context
			c.SetRequest(c.Request().WithContext(ctx))
		},
		buffer: make([]byte, 0),
	}

	// Create a custom writer that maintains context
	writer := &contextWriter{
		writer: c.Response().Writer,
		ctx:    ctx,
	}

	// Stream the response body directly to the client while also logging and counting chunks
	_, err = io.Copy(writer, countingReader)

	// Log what was sent to the client
	h.logger.Info("Raw response sent to client: %s", logBuffer.String())
	h.logger.Info("Total chunks streamed: %d", chunkCount)

	return err
}

// contextWriter is a custom writer that maintains context
type contextWriter struct {
	writer io.Writer
	ctx    context.Context
}

func (w *contextWriter) Write(p []byte) (n int, err error) {
	return w.writer.Write(p)
}

// countingReader is a custom reader that counts chunks
type countingReader struct {
	reader  io.Reader
	onChunk func()
	buffer  []byte
}

func (r *countingReader) Read(p []byte) (n int, err error) {
	n, err = r.reader.Read(p)
	if n > 0 {
		// Append to buffer
		r.buffer = append(r.buffer, p[:n]...)

		// Process complete chunks
		for {
			// Find next chunk boundary
			chunkEnd := bytes.Index(r.buffer, []byte("\n\n"))
			if chunkEnd == -1 {
				break // No complete chunk found
			}

			// Extract chunk
			chunk := r.buffer[:chunkEnd+2]
			r.buffer = r.buffer[chunkEnd+2:]

			// Only count if it's a data chunk and not [DONE]
			if bytes.HasPrefix(chunk, []byte("data: ")) && !bytes.Contains(chunk, []byte("[DONE]")) {
				r.onChunk()
			}
		}
	}
	return n, err
}
