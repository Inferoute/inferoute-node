package common

import (
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"
)

// AppError represents an application error
type AppError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Err     error  `json:"-"` // Internal error details, not exposed to API
}

// Error implements the error interface
func (e *AppError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Err)
	}
	return e.Message
}

// Unwrap returns the wrapped error
func (e *AppError) Unwrap() error {
	return e.Err
}

// NewAppError creates a new AppError
func NewAppError(code int, message string, err error) *AppError {
	return &AppError{
		Code:    code,
		Message: message,
		Err:     err,
	}
}

// Common error types
var (
	ErrInvalidInput = func(err error) *AppError {
		return NewAppError(http.StatusBadRequest, "Invalid input", err)
	}

	ErrUnauthorized = func(err error) *AppError {
		return NewAppError(http.StatusUnauthorized, "Unauthorized", err)
	}

	ErrForbidden = func(err error) *AppError {
		return NewAppError(http.StatusForbidden, "Forbidden", err)
	}

	ErrNotFound = func(err error) *AppError {
		return NewAppError(http.StatusNotFound, "Resource not found", err)
	}

	ErrConflict = func(err error) *AppError {
		return NewAppError(http.StatusConflict, "Resource conflict", err)
	}

	ErrInternalServer = func(err error) *AppError {
		return NewAppError(http.StatusInternalServerError, "Internal server error", err)
	}

	ErrServiceUnavailable = func(err error) *AppError {
		return NewAppError(http.StatusServiceUnavailable, "Service unavailable", err)
	}

	ErrTimeout = func(err error) *AppError {
		return NewAppError(http.StatusRequestTimeout, "Request timeout", err)
	}

	ErrInsufficientFunds = func(err error) *AppError {
		return NewAppError(http.StatusPaymentRequired, "Insufficient funds", err)
	}

	ErrProviderUnavailable = func(err error) *AppError {
		return NewAppError(http.StatusServiceUnavailable, "Provider unavailable", err)
	}

	ErrInvalidHMAC = func(err error) *AppError {
		return NewAppError(http.StatusUnauthorized, "Invalid HMAC", err)
	}
)

// IsAppError checks if an error is an AppError
func IsAppError(err error) (*AppError, bool) {
	appErr, ok := err.(*AppError)
	return appErr, ok
}

// GetStatusCode returns the HTTP status code for an error
func GetStatusCode(err error) int {
	if appErr, ok := IsAppError(err); ok {
		return appErr.Code
	}
	return http.StatusInternalServerError
}

// GetErrorResponse returns a standardized error response
func GetErrorResponse(err error) map[string]interface{} {
	if appErr, ok := IsAppError(err); ok {
		return map[string]interface{}{
			"error": map[string]interface{}{
				"code":    appErr.Code,
				"message": appErr.Message,
			},
		}
	}

	return map[string]interface{}{
		"error": map[string]interface{}{
			"code":    http.StatusInternalServerError,
			"message": "Internal server error",
		},
	}
}

type ErrorResponse struct {
	Message string `json:"message"`
	Error   string `json:"error,omitempty"`
}

func NewErrorResponse(message string, err error) *ErrorResponse {
	errStr := ""
	if err != nil {
		errStr = err.Error()
	}
	return &ErrorResponse{
		Message: message,
		Error:   errStr,
	}
}

func NewBadRequestError(message string) error {
	return echo.NewHTTPError(http.StatusBadRequest, NewErrorResponse(message, nil))
}

func NewInternalError(message string, err error) error {
	return echo.NewHTTPError(http.StatusInternalServerError, NewErrorResponse(message, err))
}

func NewNotFoundError(message string) error {
	return echo.NewHTTPError(http.StatusNotFound, NewErrorResponse(message, nil))
}

func NewUnauthorizedError(message string) error {
	return echo.NewHTTPError(http.StatusUnauthorized, NewErrorResponse(message, nil))
}

// ErrBadRequest returns an HTTP error with status code 400
func ErrBadRequest(err error) error {
	return echo.NewHTTPError(http.StatusBadRequest, err.Error())
}
