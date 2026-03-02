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
					return common.ErrUnauthorized(fmt.Errorf("invalid internal key"))
				}
				return next(c)
			}
			return next(c)
		}
	})

	// Register internal routes first
	logger.Info("Registering internal route: POST /api/model-pricing/update-pricing-data")
	internalGroup.POST("/update-pricing-data", handler.UpdateModelPricingData)

	// Authenticated routes (supports both provider and consumer API keys)
	authenticatedGroup := api.Group("/model-pricing", func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if c.Request().URL.Path == "/api/model-pricing/update-pricing-data" {
				return next(c)
			}

			auth := c.Request().Header.Get("Authorization")
			if auth == "" {
				return common.ErrUnauthorized(fmt.Errorf("missing authorization header"))
			}

			parts := strings.Split(auth, " ")
			if len(parts) != 2 || parts[0] != "Bearer" {
				return common.ErrUnauthorized(fmt.Errorf("invalid authorization format"))
			}

			apiKey := strings.TrimSpace(parts[1])
			if apiKey == "" {
				return common.ErrUnauthorized(fmt.Errorf("empty API key"))
			}

			ctx := context.WithValue(c.Request().Context(), common.ContextKeyInternalAPIKey, cfg.InternalAPIKey)
			ctx = context.WithValue(ctx, common.ContextKeyLogger, logger)

			resp, err := common.MakeInternalRequest(
				ctx,
				"POST",
				common.AuthService,
				"/api/auth/validate",
				map[string]string{"api_key": apiKey},
			)
			if err != nil {
				return common.ErrInternalServer(fmt.Errorf("error validating API key: %w", err))
			}

			if !resp["valid"].(bool) {
				return common.ErrUnauthorized(fmt.Errorf("invalid API key"))
			}

			c.Set("user_type", resp["user_type"].(string))
			return next(c)
		}
	})

	// Register authenticated routes
	logger.Info("Registering authenticated routes")
	authenticatedGroup.POST("/get-prices", handler.GetModelPrices)
	authenticatedGroup.GET("/pricing-data/:model_name", handler.GetModelPricingData)

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
