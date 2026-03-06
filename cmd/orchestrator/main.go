// @title Inferoute API
// @version 1.0
// @description API for Inferoute orchestration and providers
// @host api.inferoute.com
// @BasePath /
// @schemes https http
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
	"github.com/sentnl/inferoute-node/pkg/api/orchestrator"
	"github.com/sentnl/inferoute-node/pkg/common"
	"github.com/sentnl/inferoute-node/pkg/common/apikey"
	"github.com/sentnl/inferoute-node/pkg/rabbitmq"

	_ "github.com/sentnl/inferoute-node/docs"
	echoswagger "github.com/swaggo/echo-swagger"
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
	logger := common.NewLogger("orchestrator")

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

	// Add auth middleware to extract consumer ID from API key
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

			plainTextKey := strings.TrimSpace(parts[1])
			if plainTextKey == "" {
				return common.ErrUnauthorized(fmt.Errorf("empty API key"))
			}

			// Query all API keys and compare hashes
			var consumerID uuid.UUID
			rows, err := database.QueryContext(c.Request().Context(),
				`SELECT c.id, ak.api_key
				FROM consumers c
				JOIN api_keys ak ON ak.consumer_id = c.id
				WHERE ak.is_active = true`)
			if err != nil {
				return common.ErrInternalServer(fmt.Errorf("error querying API keys: %w", err))
			}
			defer rows.Close()

			found := false
			for rows.Next() {
				var id uuid.UUID
				var hashedKey string
				if err := rows.Scan(&id, &hashedKey); err != nil {
					return common.ErrInternalServer(fmt.Errorf("error scanning API key: %w", err))
				}

				if apikey.CompareAPIKey(plainTextKey, hashedKey) {
					consumerID = id
					found = true
					break
				}
			}

			if !found {
				return common.ErrUnauthorized(fmt.Errorf("invalid API key"))
			}

			// Set consumer ID in context
			c.Set("consumer_id", consumerID)
			return next(c)
		}
	})

	// Initialize services and handlers
	service := orchestrator.NewService(database, logger, rmq, cfg.InternalAPIKey)
	handler := orchestrator.NewHandler(service, logger)

	// Register routes
	handler.RegisterRoutes(e)
	e.GET("/swagger/*", echoswagger.WrapHandler)

	// Start server
	go func() {
		servicePort := cfg.ServerPort // Default to configured port
		if cfg.IsDevelopment() {
			servicePort = 8080 // Development port for Orchestrator service
		}
		addr := fmt.Sprintf("%s:%d", cfg.ServerHost, servicePort)
		logger.Info("Starting orchestrator service on %s (env: %s)", addr, cfg.Environment)
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
