package orchestrator

import (
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/sentnl/inferoute-node/pkg/common"
)

// AuthMiddleware validates API keys and sets user info in context
func AuthMiddleware(logger *common.Logger) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// Extract API key from Authorization header
			authHeader := c.Request().Header.Get("Authorization")
			if authHeader == "" {
				return common.ErrUnauthorized(fmt.Errorf("missing Authorization header"))
			}

			parts := strings.Split(authHeader, " ")
			if len(parts) != 2 || parts[0] != "Bearer" {
				return common.ErrUnauthorized(fmt.Errorf("invalid Authorization header format"))
			}

			apiKey := parts[1]

			// Validate API key with auth service
			resp, err := common.MakeInternalRequest(
				c.Request().Context(),
				"POST",
				common.AuthService,
				"/api/auth/validate",
				ValidateAPIKeyRequest{APIKey: apiKey},
			)
			if err != nil {
				return common.ErrInternalServer(fmt.Errorf("error validating API key: %w", err))
			}

			if !resp["valid"].(bool) {
				return common.ErrUnauthorized(fmt.Errorf("invalid API key"))
			}

			// Extract user info from response
			userID, err := uuid.Parse(resp["user_id"].(string))
			if err != nil {
				return common.ErrInternalServer(fmt.Errorf("invalid user ID in response"))
			}

			userType := resp["user_type"].(string)
			if userType != "consumer" {
				return common.ErrUnauthorized(fmt.Errorf("only consumer API keys are allowed"))
			}

			consumerID, err := uuid.Parse(resp["consumer_id"].(string))
			if err != nil {
				return common.ErrInternalServer(fmt.Errorf("invalid consumer ID in response"))
			}

			// Set user info in context
			c.Set("user_id", userID)
			c.Set("user_type", userType)
			c.Set("consumer_id", consumerID)
			c.Set("available_balance", resp["available_balance"].(float64))
			c.Set("held_balance", resp["held_balance"].(float64))

			return next(c)
		}
	}
}
