package main

import (
	"context"
	"database/sql"
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
	"github.com/sentnl/inferoute-node/pkg/api/provider"
	"github.com/sentnl/inferoute-node/pkg/common"
	"github.com/sentnl/inferoute-node/pkg/rabbitmq"
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
	logger := common.NewLogger("provider-management")

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

	// Initialize RabbitMQ
	rmqURL := fmt.Sprintf("amqp://%s:%s@%s:%d/%s",
		cfg.RabbitMQUser,
		cfg.RabbitMQPassword,
		cfg.RabbitMQHost,
		cfg.RabbitMQPort,
		cfg.RabbitMQVHost)
	rmq, err := rabbitmq.NewClient(rmqURL)
	if err != nil {
		logger.Error("Failed to connect to RabbitMQ: %v", err)
		os.Exit(1)
	}
	defer rmq.Close()

	// Create Echo instance
	e := echo.New()

	// Set custom validator
	e.Validator = &CustomValidator{validator: validator.New()}

	// Add middleware
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	e.Use(middleware.CORS())

	// Set custom error handler
	customMiddleware := common.NewMiddleware(logger)
	e.HTTPErrorHandler = customMiddleware.ErrorHandler()

	// Add internal security middleware for internal endpoints
	internalGroup := e.Group("/api/provider/internal")
	internalGroup.Use(common.InternalOnly())

	// Add auth middleware to extract provider ID from API key
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
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

			// Query the database to get the provider ID associated with this API key
			var providerID uuid.UUID
			query := `SELECT p.id 
				FROM providers p
				JOIN api_keys ak ON ak.provider_id = p.id
				WHERE ak.api_key = $1 AND ak.is_active = true`

			err := database.QueryRowContext(c.Request().Context(), query, apiKey).Scan(&providerID)
			if err != nil {
				if err == sql.ErrNoRows {
					return common.ErrUnauthorized(fmt.Errorf("invalid API key"))
				}
				return common.ErrInternalServer(fmt.Errorf("error validating API key: %w", err))
			}

			// Set provider ID in context
			c.Set("provider_id", providerID)
			return next(c)
		}
	})

	// Initialize services and handlers
	providerService := provider.NewService(database, logger, rmq)
	providerHandler := provider.NewHandler(providerService, logger, database.DB)

	// Register routes
	providerHandler.Register(e)

	// Start server
	go func() {
		servicePort := cfg.ServerPort // Default to configured port
		if cfg.IsDevelopment() {
			servicePort = 8082 // Development port for Provider Management service
		}
		addr := fmt.Sprintf("%s:%d", cfg.ServerHost, servicePort)
		logger.Info("Starting provider management service on %s (env: %s)", addr, cfg.Environment)
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
