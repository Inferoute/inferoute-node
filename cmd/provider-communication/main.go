package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/sentnl/inferoute-node/internal/config"
	"github.com/sentnl/inferoute-node/internal/db"
	"github.com/sentnl/inferoute-node/pkg/api/provider_comm"
	"github.com/sentnl/inferoute-node/pkg/common"
)

// CustomValidator is a custom validator for Echo
type CustomValidator struct {
	validator *validator.Validate
}

// Validate validates the input
func (cv *CustomValidator) Validate(i interface{}) error {
	if err := cv.validator.Struct(i); err != nil {
		return common.ErrInvalidInput(err)
	}
	return nil
}

func main() {
	// Load config
	cfg, err := config.LoadConfig(".env")
	if err != nil {
		fmt.Printf("Error loading config: %v\n", err)
		os.Exit(1)
	}

	// Initialize logger
	logger := common.NewLogger("provider-communication")

	// Connect to database
	database, err := db.New(
		cfg.DatabaseHost,
		cfg.DatabasePort,
		cfg.DatabaseUser,
		cfg.DatabasePassword,
		cfg.DatabaseDBName,
		cfg.DatabaseSSLMode,
	)
	if err != nil {
		logger.Error("Failed to connect to database: %v", err)
		os.Exit(1)
	}
	defer database.Close()

	// Create Echo instance
	e := echo.New()

	// Set custom validator
	e.Validator = &CustomValidator{validator: validator.New()}

	// Add middleware
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	e.Use(middleware.CORS())

	// Add auth middleware for provider endpoints
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// Skip auth for send_requests endpoint (used by orchestrator)
			if c.Path() == "/api/provider-comms/send_requests" {
				return next(c)
			}

			auth := c.Request().Header.Get("Authorization")
			if auth == "" {
				return common.ErrUnauthorized(fmt.Errorf("missing authorization header"))
			}

			// Extract API key from Bearer token
			parts := strings.Split(auth, " ")
			if len(parts) != 2 || parts[0] != "Bearer" {
				return common.ErrUnauthorized(fmt.Errorf("invalid authorization format"))
			}

			apiKey := strings.TrimSpace(parts[1])
			if apiKey == "" {
				return common.ErrUnauthorized(fmt.Errorf("empty API key"))
			}

			// Query the database to get the provider associated with this API key
			var userID uuid.UUID
			var username string
			query := `SELECT u.id, u.username 
				FROM users u
				JOIN api_keys ak ON u.id = ak.user_id
				WHERE ak.api_key = $1 AND u.type = 'provider' AND ak.is_active = true`

			err := database.QueryRowContext(c.Request().Context(), query, apiKey).Scan(&userID, &username)
			if err != nil {
				return common.ErrUnauthorized(fmt.Errorf("invalid API key"))
			}

			// Set user info in context
			c.Set("user_id", userID)
			c.Set("username", username)
			return next(c)
		}
	})

	// Initialize handler
	handler := provider_comm.NewHandler(database, logger)

	// Register routes
	handler.Register(e)

	// Start server
	go func() {
		servicePort := cfg.ServerPort // Default to configured port
		if cfg.IsDevelopment() {
			servicePort = 8083 // Development port for Provider Communication service
		}
		addr := fmt.Sprintf("%s:%d", cfg.ServerHost, servicePort)
		logger.Info("Starting provider communication service on %s (env: %s)", addr, cfg.Environment)
		if err := e.Start(addr); err != nil {
			logger.Error("Failed to start server: %v", err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := e.Shutdown(ctx); err != nil {
		logger.Error("Failed to shutdown server: %v", err)
	}
}
