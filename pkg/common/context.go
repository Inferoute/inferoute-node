package common

import (
	"context"
)

type CtxKey string

const (
	CtxKeyLogger          CtxKey = "logger"
	CtxKeyOriginalRequest CtxKey = "originalRequest"
	CtxKeyInternalAPIKey  CtxKey = "internalAPIKey"
	CtxKeyUserID          CtxKey = "userID"
	CtxKeyTransactionID   CtxKey = "transactionID"
	CtxKeyProvider        CtxKey = "provider"
	CtxKeyModel           CtxKey = "model"
	CtxKeyRawRequest      CtxKey = "rawRequest"
	CtxKeyEndpoint        CtxKey = "endpoint"
	CtxKeyPath            CtxKey = "path"
)

// GetLoggerFromContext retrieves the logger from the context.
// If not found, it returns a new default logger.
func GetLoggerFromContext(ctx context.Context) *Logger {
	if logger, ok := ctx.Value(CtxKeyLogger).(*Logger); ok && logger != nil {
		return logger
	}
	// Fallback to a default logger if not found in context or if nil
	return NewLogger("default-context-logger")
}
