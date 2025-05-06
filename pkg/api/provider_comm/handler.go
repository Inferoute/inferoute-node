package provider_comm

import (
	"context"
	"fmt"
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

	// Check if request wants streaming
	isStreaming := false
	if reqData, ok := req.RequestData["stream"]; ok {
		if streamBool, ok := reqData.(bool); ok {
			isStreaming = streamBool
		}
	}

	if isStreaming {
		// Set streaming headers
		c.Response().Header().Set("Content-Type", "text/event-stream")
		c.Response().Header().Set("Cache-Control", "no-cache")
		c.Response().Header().Set("Connection", "keep-alive")
		c.Response().WriteHeader(http.StatusOK)

		// Create a done channel for cleanup
		done := make(chan error)

		// Start streaming in a goroutine
		go func() {
			buffer := make([]byte, 1024)
			for {
				select {
				case <-c.Request().Context().Done():
					done <- c.Request().Context().Err()
					return
				default:
					n, err := responseBody.Read(buffer)
					if n > 0 {
						// Write chunk to response
						if _, writeErr := c.Response().Write(buffer[:n]); writeErr != nil {
							done <- writeErr
							return
						}
						c.Response().Flush()
					}
					if err == io.EOF {
						done <- nil
						return
					}
					if err != nil {
						done <- err
						return
					}
				}
			}
		}()

		// Wait for streaming to complete or context to cancel
		if err := <-done; err != nil {
			if err == context.Canceled {
				h.logger.Info("Client closed connection")
				return nil
			}
			h.logger.Error("Streaming error: %v", err)
			return common.ErrInternalServer(fmt.Errorf("streaming error: %w", err))
		}
		return nil
	}

	// For non-streaming requests, just copy the response
	c.Response().Header().Set("Content-Type", "application/json")
	_, err = io.Copy(c.Response(), responseBody)
	return err
}
