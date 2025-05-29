package main

import (
	"fmt"
	"os"

	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/sentnl/inferoute-node/internal/config"
	"github.com/sentnl/inferoute-node/internal/db"
	"github.com/sentnl/inferoute-node/pkg/api/cloudflare" // Import the new cloudflare package
	"github.com/sentnl/inferoute-node/pkg/common"
)

// CustomValidator is a custom validator for Echo (can be shared or duplicated)
type CustomValidator struct {
	validator *validator.Validate
}

// Validate validates the input
func (cv *CustomValidator) Validate(i interface{}) error {
	if err := cv.validator.Struct(i); err != nil {
		// TODO: Consider if common.ErrInvalidInput should be directly returned
		// or if a more specific error for validation context is needed here.
		return common.ErrInvalidInput(err) // Using common.ErrInvalidInput as per existing auth service
	}
	return nil
}

func main() {
	// Initialize logger
	logger := common.NewLogger("cloudflare-service")

	// Load configuration
	cfg, err := config.LoadConfig("") // Pass path if .env is not in root
	if err != nil {
		logger.Fatal("Failed to load configuration: %v", err)
	}

	// Validate essential Cloudflare configuration
	if cfg.CloudflareAPIKey == "" || cfg.CloudflareAccountID == "" || cfg.CloudflareZoneID == "" {
		logger.Fatal("Cloudflare API Key, Account ID, and Zone ID must be configured.")
	}

	// Initialize database connection
	database, err := db.New(
		cfg.DatabaseHost,
		cfg.DatabasePort,
		cfg.DatabaseUser,
		cfg.DatabasePassword,
		cfg.DatabaseDBName,
		cfg.DatabaseSSLMode,
	)
	if err != nil {
		logger.Fatal("Failed to connect to database: %v", err)
	}
	defer database.Close()

	// Create Echo instance
	e := echo.New()

	// Set custom validator
	e.Validator = &CustomValidator{validator: validator.New()}

	// Add middleware
	e.Use(middleware.Recover())
	e.Use(middleware.Logger()) // Echo's built-in logger
	e.Use(middleware.CORS())   // Basic CORS setup

	// Create the internal API key middleware using common.InternalOnlyWithConfig
	// This assumes common.InternalOnlyWithConfig is adapted or a new one is created
	// that takes the key directly from cfg.InternalAPIKey.
	// For now, let's use the existing common.InternalOnly() and assume it works as intended or will be updated.
	// If common.InternalOnly relies on specific service config not present here, this needs adjustment.
	internalKeyAuthMiddleware := common.InternalOnly() // This will use the key from its own config loading or a shared one.
	// A more direct way if InternalOnly is refactored:
	// internalKeyAuthMiddleware := common.NewInternalKeyMiddleware(cfg.InternalAPIKey, logger)

	// Initialize services and handlers
	cloudflareService, err := cloudflare.NewService(database, logger, cfg)
	if err != nil {
		logger.Fatal("Failed to create cloudflare service: %v", err)
	}
	cloudflareHandler := cloudflare.NewHandler(cloudflareService, logger)

	// Register routes, passing the middleware
	cloudflareHandler.RegisterRoutes(e, internalKeyAuthMiddleware)

	// Add health check endpoint
	e.GET("/health", func(c echo.Context) error {
		err := database.HealthCheck()
		if err != nil {
			return c.JSON(500, map[string]string{
				"status": "unhealthy",
				"error":  err.Error(),
			})
		}
		return c.JSON(200, map[string]string{
			"status": "healthy",
		})
	})

	// Start server
	servicePort := cfg.ServerPort // Default to configured port
	if cfg.IsDevelopment() {

		servicePort = 8089
	}
	addr := fmt.Sprintf("%s:%d", cfg.ServerHost, servicePort)
	logger.Info("Starting Cloudflare service on %s (env: %s)", addr, cfg.Environment)
	if err := e.Start(addr); err != nil {
		logger.Error("Failed to start server: %v", err)
		os.Exit(1)
	}
}
