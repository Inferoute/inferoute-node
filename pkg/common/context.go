package common

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
