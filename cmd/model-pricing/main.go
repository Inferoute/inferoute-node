package main

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

	"github.com/google/uuid"
	"github.com/sentnl/inferoute-node/internal/config"
	"github.com/sentnl/inferoute-node/internal/db"
	"github.com/sentnl/inferoute-node/pkg/api/model_pricing"
	"github.com/sentnl/inferoute-node/pkg/common"
)

func main() {
	// Load config
	cfg, err := config.LoadConfig(".env")
	if err != nil {
		fmt.Printf("Error loading config: %v\n", err)
		os.Exit(1)
	}

	// Initialize logger
	logger := common.NewLogger("model-pricing")

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
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	e.Use(middleware.CORS())

	logger.Info("Setting up middleware chain")

	// Add provider auth middleware
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			logger.Info("Provider Auth Middleware - Processing request to: %s", c.Request().URL.Path)

			// Skip provider auth for internal routes
			if strings.HasSuffix(c.Request().URL.Path, "/update-costs") {
				logger.Info("Provider Auth Middleware - Skipping for internal route")
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

	// Initialize service and handler
	service := model_pricing.NewService(database, logger)
	handler := model_pricing.NewHandler(service)

	logger.Info("Registering routes")

	// Register internal routes first
	internalGroup := e.Group("/api/model-pricing")
	internalGroup.Use(common.InternalOnly())
	logger.Info("Registering internal route: POST /api/model-pricing/update-costs")
	internalGroup.POST("/update-costs", handler.UpdateModelCosts)

	// Then register public routes
	publicGroup := e.Group("/api/model-pricing")
	logger.Info("Registering public route: POST /api/model-pricing/get-prices")
	publicGroup.POST("/get-prices", handler.GetModelPrices)

	// Start server
	go func() {
		servicePort := cfg.ServerPort // Default to configured port
		if cfg.IsDevelopment() {
			servicePort = 8085 // Development port for Model Pricing service
		}
		addr := fmt.Sprintf("%s:%d", cfg.ServerHost, servicePort)
		logger.Info("Starting model-pricing service on %s (env: %s)", addr, cfg.Environment)
		if err := e.Start(addr); err != nil && err != http.ErrServerClosed {
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
