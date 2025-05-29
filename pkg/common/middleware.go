package common

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

// ContextKey is a type for context keys to avoid collisions.
type ContextKey string

const (
	// ContextKeyInternalAPIKey is the key for the internal API key in context.
	ContextKeyInternalAPIKey ContextKey = "internal_api_key"
	// ContextKeyLogger is the key for the logger in context.
	ContextKeyLogger ContextKey = "logger"
	// ContextKeyAPIKey is the key for the client's API key in context.
	ContextKeyAPIKey ContextKey = "api_key"
	// ContextKeyOriginalPath is the key for the original request path in context.
	ContextKeyOriginalPath ContextKey = "original_path"
	// ContextKeyOriginalRequest is the key for the original request body/struct in context.
	ContextKeyOriginalRequest ContextKey = "original_request"
)

// Middleware holds all middleware dependencies
type Middleware struct {
	logger *Logger
}

// NewMiddleware creates a new middleware instance
func NewMiddleware(logger *Logger) *Middleware {
	return &Middleware{
		logger: logger,
	}
}

// Logger returns a middleware function for logging requests
func (m *Middleware) Logger() echo.MiddlewareFunc {
	return middleware.LoggerWithConfig(middleware.LoggerConfig{
		Format: "method=${method}, uri=${uri}, status=${status}, latency=${latency_human}\n",
	})
}

// Recover returns a middleware function for recovering from panics
func (m *Middleware) Recover() echo.MiddlewareFunc {
	return middleware.RecoverWithConfig(middleware.RecoverConfig{
		StackSize: 4 << 10, // 4 KB
		LogLevel:  0,       // DEBUG level
	})
}

// CORS returns a middleware function for handling CORS
func (m *Middleware) CORS() echo.MiddlewareFunc {
	return middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: []string{"*"},
		AllowMethods: []string{http.MethodGet, http.MethodPut, http.MethodPost, http.MethodDelete},
	})
}

// RequestID returns a middleware function for adding request IDs
func (m *Middleware) RequestID() echo.MiddlewareFunc {
	return middleware.RequestIDWithConfig(middleware.RequestIDConfig{
		Generator: func() string {
			return fmt.Sprintf("%d", time.Now().UnixNano())
		},
	})
}

// APIKeyAuth returns a middleware function for API key authentication
func (m *Middleware) APIKeyAuth(validateAPIKey func(string) (bool, error)) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			auth := c.Request().Header.Get("Authorization")
			if auth == "" {
				return echo.NewHTTPError(http.StatusUnauthorized, "missing API key")
			}

			// Extract API key from Bearer token
			parts := strings.Split(auth, " ")
			if len(parts) != 2 || parts[0] != "Bearer" {
				return echo.NewHTTPError(http.StatusUnauthorized, "invalid authorization format")
			}

			apiKey := parts[1]
			valid, err := validateAPIKey(apiKey)
			if err != nil {
				return echo.NewHTTPError(http.StatusInternalServerError, "error validating API key")
			}

			if !valid {
				return echo.NewHTTPError(http.StatusUnauthorized, "invalid API key")
			}

			return next(c)
		}
	}
}

// Timeout returns a middleware function for request timeout
func (m *Middleware) Timeout(timeout time.Duration) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			ctx, cancel := context.WithTimeout(c.Request().Context(), timeout)
			defer cancel()

			c.SetRequest(c.Request().WithContext(ctx))

			done := make(chan error)
			go func() {
				done <- next(c)
			}()

			select {
			case <-ctx.Done():
				return echo.NewHTTPError(http.StatusRequestTimeout, "request timeout")
			case err := <-done:
				return err
			}
		}
	}
}

// ErrorHandler returns a custom error handler
func (m *Middleware) ErrorHandler() echo.HTTPErrorHandler {
	return func(err error, c echo.Context) {
		var (
			code    = http.StatusInternalServerError
			message interface{}
		)

		if appErr, ok := IsAppError(err); ok {
			code = appErr.Code
			message = appErr.Message
			if appErr.Err != nil {
				message = appErr.Err.Error()
			}
		} else if he, ok := err.(*echo.HTTPError); ok {
			code = he.Code
			message = he.Message
		} else {
			message = err.Error()
		}

		// Log the error
		m.logger.Printf("Error: %v", err)

		if !c.Response().Committed {
			if c.Request().Method == http.MethodHead {
				err = c.NoContent(code)
			} else {
				err = c.JSON(code, map[string]interface{}{
					"error": message,
				})
			}
			if err != nil {
				m.logger.Printf("Error sending error response: %v", err)
			}
		}
	}
}
