package common

import (
	"fmt"
	"net"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/sentnl/inferoute-node/internal/config"
)

// InternalOnly returns a middleware function that ensures the request is coming from an internal service
func InternalOnly() echo.MiddlewareFunc {
	// Load config
	cfg, err := config.LoadConfig("")
	if err != nil {
		panic("Failed to load config: " + err.Error())
	}

	logger := NewLogger("internal-security")
	logger.Info("Initializing internal security middleware with CIDR: %s", cfg.InternalNetworkCIDR)

	// Parse CIDR once during middleware initialization
	_, internalNet, err := net.ParseCIDR(cfg.InternalNetworkCIDR)
	if err != nil {
		logger.Error("Invalid CIDR %s, defaulting to localhost network: %v", cfg.InternalNetworkCIDR, err)
		_, internalNet, _ = net.ParseCIDR("127.0.0.0/8")
	}

	// Also parse IPv6 localhost
	_, internalNetV6, _ := net.ParseCIDR("::1/128")

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// Log request details
			logger.Info("Request details:")
			logger.Info("  Remote IP: %s", c.Request().RemoteAddr)
			logger.Info("  X-Forwarded-For: %s", c.Request().Header.Get("X-Forwarded-For"))
			logger.Info("  X-Real-IP: %s", c.Request().Header.Get("X-Real-IP"))

			// Check internal API key
			apiKey := c.Request().Header.Get("X-Internal-Key")
			if apiKey != cfg.InternalAPIKey {
				logger.Error("Invalid internal API key provided")
				return echo.NewHTTPError(http.StatusUnauthorized, "Invalid internal API key")
			}

			// Check if request is from internal network
			ip := c.RealIP()
			logger.Info("  RealIP(): %s", ip)

			if strings.Contains(ip, ",") {
				// If multiple IPs (X-Forwarded-For), take the first one
				ip = strings.TrimSpace(strings.Split(ip, ",")[0])
				logger.Info("  Multiple IPs found, using first one: %s", ip)
			}

			ipAddr := net.ParseIP(ip)
			if ipAddr == nil {
				logger.Error("Invalid IP address: %s", ip)
				return echo.NewHTTPError(http.StatusUnauthorized, "Invalid IP address")
			}

			logger.Info("  Parsed IP: %s", ipAddr.String())
			logger.Info("  Internal Network IPv4: %s", internalNet.String())
			logger.Info("  Internal Network IPv6: %s", internalNetV6.String())

			// Check both IPv4 and IPv6 networks
			isInternal := internalNet.Contains(ipAddr) || internalNetV6.Contains(ipAddr)
			logger.Info("  Is Internal: %v", isInternal)

			if !isInternal {
				logger.Error("IP %s not in internal networks", ipAddr.String())
				return echo.NewHTTPError(http.StatusUnauthorized, fmt.Sprintf("Request not from internal network (IP: %s)", ipAddr.String()))
			}

			return next(c)
		}
	}
}
