package main

import (
	"context"
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
	"github.com/sentnl/inferoute-node/pkg/common/apikey"
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

	// Initialize service and handler
	service := model_pricing.NewService(database, logger)
	handler := model_pricing.NewHandler(service)

	logger.Info("Registering routes")

	// Create base API group
	api := e.Group("/api")

	// Internal endpoints (protected by X-Internal-Key)
	internalGroup := api.Group("/model-pricing", func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if c.Request().URL.Path == "/api/model-pricing/update-pricing-data" {

				key := c.Request().Header.Get("X-Internal-Key")
				if key != cfg.InternalAPIKey {
					logger.Error("Internal Auth Middleware - Invalid or missing internal key: %s", key)
					return common.ErrUnauthorized(fmt.Errorf("invalid internal key"))
				}
				logger.Info("Internal Auth Middleware - Valid internal key")
				return next(c)
			}
			return next(c)
		}
	})

	// Register internal routes first
	logger.Info("Registering internal route: POST /api/model-pricing/update-pricing-data")
	internalGroup.POST("/update-pricing-data", handler.UpdateModelPricingData)

	// Provider authenticated routes
	providerGroup := api.Group("/model-pricing", func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// Skip provider auth for internal routes
			if c.Request().URL.Path == "/api/model-pricing/update-pricing-data" {
				return next(c)
			}

			logger.Info("Provider Auth Middleware - Processing request to: %s", c.Request().URL.Path)

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
			var providerID uuid.UUID
			rows, err := database.QueryContext(c.Request().Context(),
				`SELECT p.id, ak.api_key
				FROM providers p
				JOIN api_keys ak ON ak.provider_id = p.id
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
					providerID = id
					found = true
					break
				}
			}

			if !found {
				return common.ErrUnauthorized(fmt.Errorf("invalid API key"))
			}

			// Set provider ID in context
			c.Set("provider_id", providerID)
			return next(c)
		}
	})

	// Register provider routes
	logger.Info("Registering provider-authenticated routes")
	providerGroup.POST("/get-prices", handler.GetModelPrices)
	providerGroup.GET("/pricing-data/:model_name", handler.GetModelPricingData)

	// Start the scheduler to update pricing data every minute
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Run the pricing data update immediately on startup
	go func() {
		logger.Info("Running initial pricing data update")
		count, err := service.UpdateModelPricingData(ctx)
		if err != nil {
			logger.Error("Failed to update pricing data: %v", err)
		} else {
			logger.Info("Initial pricing data update completed for %d models", count)
		}
	}()

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
	ctx, cancel = context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := e.Shutdown(ctx); err != nil {
		logger.Error("Failed to shutdown server: %v", err)
	}
}
