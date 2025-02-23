package main

import (
	"fmt"
	"os"

	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/sentnl/inferoute-node/internal/config"
	"github.com/sentnl/inferoute-node/internal/db"
	"github.com/sentnl/inferoute-node/pkg/api/auth"
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
	// Initialize logger
	logger := common.NewLogger("auth-service")

	// Load configuration
	cfg, err := config.LoadConfig("")
	if err != nil {
		logger.Fatal("Failed to load configuration: %v", err)
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
	e.Use(middleware.Logger())
	e.Use(middleware.CORS())
	e.Use(common.InternalOnly())

	// Initialize services and handlers
	authService := auth.NewService(database, logger, auth.Config{
		InternalKey: cfg.InternalAPIKey,
	})
	authHandler := auth.NewHandler(authService, logger)

	// Register routes
	authHandler.RegisterRoutes(e)

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
		servicePort = 8081 // Development port for Auth service
	}
	addr := fmt.Sprintf("%s:%d", cfg.ServerHost, servicePort)
	logger.Info("Starting Auth service on %s (env: %s)", addr, cfg.Environment)
	if err := e.Start(addr); err != nil {
		logger.Error("Failed to start server: %v", err)
		os.Exit(1)
	}
}
