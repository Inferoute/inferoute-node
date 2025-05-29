package cloudflare

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/cloudflare/cloudflare-go"
	"github.com/google/uuid"
	"github.com/sentnl/inferoute-node/internal/config"
	"github.com/sentnl/inferoute-node/internal/db"
	"github.com/sentnl/inferoute-node/pkg/common"
)

const (
	cloudFlareAPIBaseURL = "https://api.cloudflare.com/client/v4"
	inferouteDomain      = "inferoute.net" // Or load from config if it can vary
)

// Service handles business logic for Cloudflare tunnels.
// It requires a database connection, logger, application config,
// and an HTTP client for making calls to other internal services (like auth)
// and to the Cloudflare API.
type Service struct {
	db               *db.DB
	logger           *common.Logger
	config           *config.Config
	httpClient       *http.Client    // For internal auth calls
	cloudflareClient *cloudflare.API // Official Cloudflare client
}

// NewService creates a new Cloudflare service.
func NewService(db *db.DB, logger *common.Logger, cfg *config.Config) (*Service, error) {
	// Initialize Cloudflare API client
	if cfg.CloudflareAPIKey == "" || cfg.CloudflareEmail == "" {
		return nil, fmt.Errorf("Cloudflare API Key and Email must be configured")
	}

	// Log the internal API key to ensure it's loaded
	if cfg.InternalAPIKey == "" {
		logger.Warn("InternalAPIKey is not set in the configuration!")
	} else {
		logger.Info("InternalAPIKey loaded successfully, length: %d", len(cfg.InternalAPIKey))
	}

	// Note: UsingAccount is an option for NewClient, not New.
	// Client operations will need to specify the account or zone identifier.
	cfClient, err := cloudflare.New(cfg.CloudflareAPIKey, cfg.CloudflareEmail)
	if err != nil {
		return nil, fmt.Errorf("error creating Cloudflare client: %w", err)
	}

	return &Service{
		db:               db,
		logger:           logger,
		config:           cfg,
		httpClient:       &http.Client{Timeout: 10 * time.Second},
		cloudflareClient: cfClient,
	}, nil
}

// AuthResponse represents the expected response from the internal auth service,
// augmented with details fetched subsequently.
type AuthResponse struct {
	Valid        bool       `json:"valid"`
	UserID       uuid.UUID  `json:"user_id,omitempty"`
	ProviderID   *uuid.UUID `json:"provider_id,omitempty"`
	Username     string     `json:"username,omitempty"`
	ProviderName string     `json:"provider_name,omitempty"`
}

// validateAPIKeyAndGetDetails validates an API key via the internal auth service
// and fetches associated Username and ProviderName.
// It now accepts apiKey as a string parameter.
func (s *Service) validateAPIKeyAndGetDetails(ctx context.Context, apiKey string) (*AuthResponse, error) {
	s.logger.Info("Validating API key and fetching details for key ending: ...%s", apiKey[max(0, len(apiKey)-4):])

	// Prepare context for internal request (should include logger and internal API key)
	// Assuming common.MakeInternalRequest handles this if ctx is properly set up by middleware/caller.
	// If not, ensure s.config.InternalAPIKey is added to a new context for the call.
	internalReqCtx := context.WithValue(ctx, common.ContextKeyInternalAPIKey, s.config.InternalAPIKey)
	internalReqCtx = context.WithValue(internalReqCtx, common.ContextKeyLogger, s.logger)

	authServiceReq := map[string]string{"api_key": apiKey}
	authServiceRespMap, err := common.MakeInternalRequest(
		internalReqCtx, // Use the context with internal key and logger
		"POST",
		common.AuthService, // Assumes this constant (e.g., "auth-service") is defined in pkg/common
		"/api/auth/validate",
		authServiceReq,
	)

	if err != nil {
		s.logger.Error("Failed to call auth service for API key validation: %v", err)
		// Check if the error is already a common.AppError, if so, return it directly
		if appErr, ok := err.(*common.AppError); ok {
			return nil, appErr
		}
		return nil, common.ErrInternalServer(fmt.Errorf("auth service communication error: %w", err))
	}

	valid, _ := authServiceRespMap["valid"].(bool)
	if !valid {
		s.logger.Warn("API key validation failed by auth service for key ending: ...%s", apiKey[max(0, len(apiKey)-4):])
		return &AuthResponse{Valid: false}, common.ErrUnauthorized(fmt.Errorf("invalid API key"))
	}

	userIDStr, _ := authServiceRespMap["user_id"].(string)
	providerIDStr, providerIDExists := authServiceRespMap["provider_id"].(string)

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		s.logger.Error("Failed to parse user_id from auth response ('%s'): %v", userIDStr, err)
		return nil, common.ErrInternalServer(fmt.Errorf("invalid user_id format from auth service"))
	}

	var providerIDPtr *uuid.UUID
	if providerIDExists && providerIDStr != "" {
		pid, err := uuid.Parse(providerIDStr)
		if err != nil {
			s.logger.Error("Failed to parse provider_id from auth response ('%s'): %v", providerIDStr, err)
			// This might be an issue, but proceed to see if it's a consumer key perhaps.
			// For tunnel creation, we strictly need a provider.
		} else {
			providerIDPtr = &pid
		}
	}

	if providerIDPtr == nil {
		s.logger.Warn("No ProviderID returned from auth service for API key of user %s", userID)
		return nil, common.ErrUnauthorized(fmt.Errorf("API key is not associated with a provider"))
	}

	// Fetch Username from users table
	var username string
	err = s.db.QueryRowContext(ctx, "SELECT username FROM users WHERE id = $1", userID).Scan(&username)
	if err != nil {
		if err == sql.ErrNoRows {
			s.logger.Error("No user found in DB for UserID %s (from auth service)", userID)
			return nil, common.ErrInternalServer(fmt.Errorf("user %s not found after auth validation", userID))
		} else {
			s.logger.Error("Failed to query username for UserID %s: %v", userID, err)
			return nil, common.ErrInternalServer(fmt.Errorf("database error fetching username: %w", err))
		}
	}

	// Fetch ProviderName from providers table
	var providerName string
	err = s.db.QueryRowContext(ctx, "SELECT name FROM providers WHERE id = $1", *providerIDPtr).Scan(&providerName)
	if err != nil {
		if err == sql.ErrNoRows {
			s.logger.Error("No provider found in DB for ProviderID %s (from auth service)", *providerIDPtr)
			return nil, common.ErrInternalServer(fmt.Errorf("provider %s not found after auth validation", *providerIDPtr))
		} else {
			s.logger.Error("Failed to query provider name for ProviderID %s: %v", *providerIDPtr, err)
			return nil, common.ErrInternalServer(fmt.Errorf("database error fetching provider name: %w", err))
		}
	}

	s.logger.Info("Successfully validated API key and fetched details: UserID %s, ProviderID %s, Username %s, ProviderName %s",
		userID, *providerIDPtr, username, providerName)

	return &AuthResponse{
		Valid:        true,
		UserID:       userID,
		ProviderID:   providerIDPtr,
		Username:     username,
		ProviderName: providerName,
	}, nil
}

// RequestTunnel handles the logic for creating or updating a Cloudflare tunnel.
// It now accepts apiKey and serviceURL as string parameters.
func (s *Service) RequestTunnel(ctx context.Context, apiKey string, serviceURL string) (*RequestTunnelResponse, error) {
	s.logger.Info("RequestTunnel called for service URL: %s", serviceURL)

	authDetails, err := s.validateAPIKeyAndGetDetails(ctx, apiKey)
	if err != nil {
		s.logger.Error("API key validation failed: %v", err)
		return nil, err
	}
	if !authDetails.Valid || authDetails.ProviderID == nil || authDetails.Username == "" || authDetails.ProviderName == "" {
		return nil, common.ErrUnauthorized(fmt.Errorf("API key is not valid for a provider or missing required user/provider details for DNS"))
	}

	providerID := *authDetails.ProviderID
	s.logger.Info("Requesting tunnel for provider ID: %s (User: %s, ProviderName: %s)",
		providerID.String(), authDetails.Username, authDetails.ProviderName)

	accountRC := cloudflare.AccountIdentifier(s.config.CloudflareAccountID)
	zoneRC := cloudflare.ZoneIdentifier(s.config.CloudflareZoneID)

	var tunnel ProviderTunnel
	dbErr := s.db.QueryRowContext(ctx,
		`SELECT provider_id, tunnel_id, tunnel_name, hostname, service_url, last_token_issued, created_at, updated_at
		 FROM provider_tunnels WHERE provider_id = $1`,
		providerID,
	).Scan(
		&tunnel.ProviderID, &tunnel.TunnelID, &tunnel.TunnelName,
		&tunnel.Hostname, &tunnel.ServiceURL, &tunnel.LastTokenIssued,
		&tunnel.CreatedAt, &tunnel.UpdatedAt,
	)

	if dbErr == sql.ErrNoRows {
		s.logger.Info("No existing tunnel for provider %s in DB. Checking Cloudflare directly or creating new.", providerID)

		cfTunnelName := fmt.Sprintf("%s-%s-tunnel", authDetails.Username, authDetails.ProviderName)
		subdomain := strings.ToLower(fmt.Sprintf("%s-%s", authDetails.Username, authDetails.ProviderName))
		hostname := fmt.Sprintf("%s.%s", subdomain, s.config.DomainCloudflare)

		createdTunnel, creationErr := s.createCloudflareTunnel(ctx, accountRC, cfTunnelName)
		if creationErr != nil {
			if errors.Is(creationErr, ErrTunnelAlreadyExists) {
				s.logger.Info("Tunnel name '%s' already exists in Cloudflare. Attempting to fetch its details.", cfTunnelName)
				existingCFTunnel, fetchErr := s.getCloudflareTunnelByName(ctx, accountRC, cfTunnelName)
				if fetchErr != nil {
					s.logger.Error("Failed to fetch existing tunnel '%s' by name from Cloudflare after conflict: %v", cfTunnelName, fetchErr)
					return nil, common.ErrInternalServer(fmt.Errorf("failed to fetch existing tunnel by name after conflict: %w", fetchErr))
				}
				// Use this existing Cloudflare tunnel's details
				createdTunnel = existingCFTunnel // Assign to createdTunnel to flow through the rest of the logic
				s.logger.Info("Using details from existing Cloudflare tunnel ID %s, Name: %s", createdTunnel.ID, createdTunnel.Name)
			} else {
				// Creation failed for a reason other than conflict
				return nil, common.ErrInternalServer(fmt.Errorf("failed to create cloudflare tunnel: %w", creationErr))
			}
		}

		// If createdTunnel is nil here, it means fetching after conflict also failed or some other initial error not yet caught.
		// This check is a safeguard, though the logic above should handle it.
		if createdTunnel == nil {
			s.logger.Error("Failed to obtain tunnel details for '%s' after creation/fetch attempt.", cfTunnelName)
			return nil, common.ErrInternalServer(fmt.Errorf("could not obtain tunnel details for %s", cfTunnelName))
		}

		s.logger.Info("Cloudflare tunnel obtained/created: ID %s, Name: %s", createdTunnel.ID, createdTunnel.Name)

		cnameTarget := fmt.Sprintf("%s.cfargotunnel.com", createdTunnel.ID)
		err = s.createDNSRecord(ctx, zoneRC, hostname, cnameTarget)
		if err != nil {
			s.logger.Error("Failed to create DNS record for %s, attempting to delete tunnel %s", hostname, createdTunnel.ID)
			delErr := s.deleteCloudflareTunnel(ctx, accountRC, createdTunnel.ID)
			if delErr != nil {
				s.logger.Error("Failed to delete Cloudflare tunnel %s after DNS failure: %v", createdTunnel.ID, delErr)
			}
			return nil, common.ErrInternalServer(fmt.Errorf("failed to create DNS record: %w", err))
		}
		s.logger.Info("DNS CNAME record created: %s -> %s", hostname, cnameTarget)

		tunnelClientToken, err := s.getCloudflareTunnelClientToken(ctx, accountRC, createdTunnel.ID)
		if err != nil {
			s.logger.Error("Failed to get client token for tunnel %s: %v", createdTunnel.ID, err)
			// Note: Tunnel and DNS remain created - token retrieval failure might be temporary
			// Use the cleanup API endpoint to remove stale tunnels if needed
			return nil, common.ErrInternalServer(fmt.Errorf("failed to get client token for tunnel: %w", err))
		}

		newTunnelEntry := ProviderTunnel{
			ProviderID:      providerID,
			TunnelID:        createdTunnel.ID,
			TunnelName:      createdTunnel.Name,
			Hostname:        hostname,
			ServiceURL:      serviceURL,
			LastTokenIssued: time.Now(),
		}
		_, err = s.db.ExecContext(ctx, `INSERT INTO provider_tunnels
			(provider_id, tunnel_id, tunnel_name, hostname, service_url, last_token_issued, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, NOW(), NOW())`,
			newTunnelEntry.ProviderID, newTunnelEntry.TunnelID, newTunnelEntry.TunnelName,
			newTunnelEntry.Hostname, newTunnelEntry.ServiceURL, newTunnelEntry.LastTokenIssued)
		if err != nil {
			s.logger.Error("Failed to save tunnel details to DB for tunnel %s, attempting cleanup", createdTunnel.ID)
			s.deleteDNSRecordByContent(ctx, zoneRC, hostname, cnameTarget)
			s.deleteCloudflareTunnel(ctx, accountRC, createdTunnel.ID)
			return nil, common.ErrInternalServer(fmt.Errorf("failed to save tunnel details: %w", err))
		}
		// After successful creation and DB storage, configure ingress for the new tunnel
		err = s.updateCloudflareTunnelIngress(ctx, accountRC, createdTunnel.ID, hostname, serviceURL)
		if err != nil {
			s.logger.Error("Failed to configure ingress for new tunnel %s, manual check required: %v", createdTunnel.ID, err)
			// This might not be fatal for returning the token, but it's a problem.
			// Depending on policy, you might want to return an error or just log heavily.
		}

		return &RequestTunnelResponse{
			Token:    tunnelClientToken,
			Hostname: hostname,
		}, nil

	} else if dbErr != nil {
		return nil, common.ErrInternalServer(fmt.Errorf("failed to query provider tunnel: %w", dbErr))
	}

	s.logger.Info("Existing tunnel found for provider %s. ID: %s, Hostname: %s", providerID, tunnel.TunnelID, tunnel.Hostname)
	if tunnel.ServiceURL != serviceURL {
		s.logger.Info("ServiceURL differs. Current: '%s', New: '%s'. Updating tunnel configuration.", tunnel.ServiceURL, serviceURL)
		err = s.updateCloudflareTunnelIngress(ctx, accountRC, tunnel.TunnelID, tunnel.Hostname, serviceURL)
		if err != nil {
			return nil, common.ErrInternalServer(fmt.Errorf("failed to update tunnel ingress/configuration: %w", err))
		}
		tunnel.ServiceURL = serviceURL
		_, err = s.db.ExecContext(ctx, `UPDATE provider_tunnels SET service_url = $1, updated_at = NOW()
			WHERE provider_id = $2`,
			tunnel.ServiceURL, tunnel.ProviderID)
		if err != nil {
			return nil, common.ErrInternalServer(fmt.Errorf("failed to update tunnel service URL in DB: %w", err))
		}
		s.logger.Info("Tunnel configuration updated for %s", tunnel.TunnelID)
	}

	cfTunnelClientToken, err := s.getCloudflareTunnelClientToken(ctx, accountRC, tunnel.TunnelID)
	if err != nil {
		return nil, common.ErrInternalServer(fmt.Errorf("failed to get existing tunnel client token: %w", err))
	}

	_, err = s.db.ExecContext(ctx, "UPDATE provider_tunnels SET last_token_issued = NOW() WHERE provider_id = $1", providerID)
	if err != nil {
		s.logger.Error("Failed to update last_token_issued for provider %s: %v", providerID, err)
	}

	return &RequestTunnelResponse{
		Token:    cfTunnelClientToken,
		Hostname: tunnel.Hostname,
	}, nil
}

// RefreshToken handles refreshing or re-fetching a tunnel token.
// It now accepts apiKey as a string parameter.
func (s *Service) RefreshToken(ctx context.Context, apiKey string) (*RefreshTokenResponse, error) {
	s.logger.Info("Refreshing token for provider ID: %s", apiKey)

	authDetails, err := s.validateAPIKeyAndGetDetails(ctx, apiKey)
	if err != nil {
		s.logger.Error("API key validation failed: %v", err)
		return nil, err
	}
	if !authDetails.Valid || authDetails.ProviderID == nil {
		return nil, common.ErrUnauthorized(fmt.Errorf("API key is not valid for a provider"))
	}

	providerID := *authDetails.ProviderID
	accountRC := cloudflare.AccountIdentifier(s.config.CloudflareAccountID)

	var existingTunnel ProviderTunnel
	err = s.db.QueryRowContext(ctx,
		"SELECT tunnel_id FROM provider_tunnels WHERE provider_id = $1",
		providerID,
	).Scan(&existingTunnel.TunnelID)

	if err == sql.ErrNoRows {
		return nil, common.ErrNotFound(fmt.Errorf("no tunnel found for this provider to refresh token"))
	}
	if err != nil {
		return nil, common.ErrInternalServer(fmt.Errorf("failed to query provider tunnel for refresh: %w", err))
	}

	cfTunnelClientToken, err := s.getCloudflareTunnelClientToken(ctx, accountRC, existingTunnel.TunnelID)
	if err != nil {
		return nil, common.ErrInternalServer(fmt.Errorf("failed to get/refresh tunnel client token: %w", err))
	}

	_, err = s.db.ExecContext(ctx, "UPDATE provider_tunnels SET last_token_issued = NOW() WHERE provider_id = $1", providerID)
	if err != nil {
		s.logger.Error("Failed to update last_token_issued on refresh for provider %s: %v", providerID, err)
	}

	return &RefreshTokenResponse{
		Token: cfTunnelClientToken,
	}, nil
}

// CleanupTunnel handles cleanup of inactive tunnels for the authenticated provider
func (s *Service) CleanupTunnel(ctx context.Context, apiKey string, days int) (*CleanupTunnelResponse, error) {
	s.logger.Info("CleanupTunnel called with %d days threshold for all provider tunnels", days)

	// Default to 30 days if not specified or invalid
	if days <= 0 {
		days = 30
		s.logger.Info("Using default threshold of %d days for tunnel cleanup", days)
	}

	authDetails, err := s.validateAPIKeyAndGetDetails(ctx, apiKey)
	if err != nil {
		s.logger.Error("API key validation failed for cleanup: %v", err)
		return nil, err
	}
	if !authDetails.Valid || authDetails.ProviderID == nil {
		return nil, common.ErrUnauthorized(fmt.Errorf("API key is not valid for a provider"))
	}

	providerID := *authDetails.ProviderID
	accountRC := cloudflare.AccountIdentifier(s.config.CloudflareAccountID)
	zoneRC := cloudflare.ZoneIdentifier(s.config.CloudflareZoneID)

	// Get all tunnels for this provider
	rows, err := s.db.QueryContext(ctx,
		"SELECT tunnel_id, hostname FROM provider_tunnels WHERE provider_id = $1",
		providerID,
	)
	if err != nil {
		s.logger.Error("Failed to query provider tunnels: %v", err)
		return &CleanupTunnelResponse{
			Message: fmt.Sprintf("Database error: %v", err),
		}, nil
	}
	defer rows.Close()

	var allTunnels []struct {
		TunnelID string
		Hostname string
	}

	for rows.Next() {
		var tunnel struct {
			TunnelID string
			Hostname string
		}
		if err := rows.Scan(&tunnel.TunnelID, &tunnel.Hostname); err != nil {
			s.logger.Error("Failed to scan tunnel row: %v", err)
			continue
		}
		allTunnels = append(allTunnels, tunnel)
	}

	if len(allTunnels) == 0 {
		s.logger.Info("No tunnels found for provider %s", providerID)
		return &CleanupTunnelResponse{
			TotalChecked: 0,
			TotalDeleted: 0,
			Message:      "No tunnels found for this provider",
		}, nil
	}

	s.logger.Info("Found %d tunnels for provider %s, checking activity status", len(allTunnels), providerID)

	thresholdDate := time.Now().AddDate(0, 0, -days)
	var deletedTunnels []string
	var activeTunnels []string
	var failedCleanups []string

	for _, tunnel := range allTunnels {
		s.logger.Info("Checking tunnel %s status", tunnel.TunnelID)

		// Get tunnel status from Cloudflare
		tunnelStatus, err := s.getTunnelStatus(ctx, accountRC, tunnel.TunnelID)
		if err != nil {
			s.logger.Error("Failed to get status for tunnel %s: %v", tunnel.TunnelID, err)
			failedCleanups = append(failedCleanups, tunnel.TunnelID+" (status check failed)")
			continue
		}

		// Check if tunnel is inactive
		if tunnelStatus.LastSeen.After(thresholdDate) {
			s.logger.Info("Tunnel %s is still active (last seen: %v)", tunnel.TunnelID, tunnelStatus.LastSeen)
			activeTunnels = append(activeTunnels, tunnel.TunnelID)
			continue
		}

		s.logger.Info("Tunnel %s inactive for >%d days (last seen: %v), proceeding with cleanup", tunnel.TunnelID, days, tunnelStatus.LastSeen)

		// Delete DNS record
		cnameTarget := fmt.Sprintf("%s.cfargotunnel.com", tunnel.TunnelID)
		err = s.deleteDNSRecordByContent(ctx, zoneRC, tunnel.Hostname, cnameTarget)
		if err != nil {
			s.logger.Error("Failed to delete DNS record for tunnel %s: %v", tunnel.TunnelID, err)
			// Continue with tunnel deletion even if DNS fails
		}

		// Delete Cloudflare tunnel
		err = s.deleteCloudflareTunnel(ctx, accountRC, tunnel.TunnelID)
		if err != nil {
			s.logger.Error("Failed to delete Cloudflare tunnel %s: %v", tunnel.TunnelID, err)
			failedCleanups = append(failedCleanups, tunnel.TunnelID+" (cloudflare deletion failed)")
			continue
		}

		// Remove from local DB
		_, err = s.db.ExecContext(ctx, "DELETE FROM provider_tunnels WHERE tunnel_id = $1 AND provider_id = $2", tunnel.TunnelID, providerID)
		if err != nil {
			s.logger.Error("Failed to delete tunnel %s from local DB: %v", tunnel.TunnelID, err)
			failedCleanups = append(failedCleanups, tunnel.TunnelID+" (database deletion failed)")
			continue
		}

		deletedTunnels = append(deletedTunnels, tunnel.TunnelID)
		s.logger.Info("Successfully cleaned up tunnel %s", tunnel.TunnelID)
	}

	message := fmt.Sprintf("Cleanup completed: checked %d tunnels, deleted %d inactive tunnels (threshold: %d days)",
		len(allTunnels), len(deletedTunnels), days)

	if len(failedCleanups) > 0 {
		message += fmt.Sprintf(", %d cleanups failed", len(failedCleanups))
	}

	s.logger.Info("Tunnel cleanup summary: %s", message)

	return &CleanupTunnelResponse{
		TotalChecked:   len(allTunnels),
		TotalDeleted:   len(deletedTunnels),
		DeletedTunnels: deletedTunnels,
		ActiveTunnels:  activeTunnels,
		FailedCleanups: failedCleanups,
		Message:        message,
	}, nil
}

// TunnelStatus represents tunnel status information from Cloudflare
type TunnelStatus struct {
	ID       string    `json:"id"`
	Name     string    `json:"name"`
	Status   string    `json:"status"`
	LastSeen time.Time `json:"last_seen"`
}

// getTunnelStatus gets tunnel status from Cloudflare API
func (s *Service) getTunnelStatus(ctx context.Context, rc *cloudflare.ResourceContainer, tunnelID string) (*TunnelStatus, error) {
	s.logger.Info("Getting tunnel status for ID: %s under account %s", tunnelID, rc.Identifier)

	apiURL := fmt.Sprintf("%s/accounts/%s/cfd_tunnel/%s", cloudFlareAPIBaseURL, rc.Identifier, tunnelID)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request for tunnel status: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+s.config.CloudflareAPIKey)
	req.Header.Set("Content-Type", "application/json")

	httpClient := &http.Client{Timeout: 15 * time.Second}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP client error getting tunnel status: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read tunnel status response body: %w", err)
	}

	var cfResponse struct {
		Success bool               `json:"success"`
		Errors  []cloudflare.Error `json:"errors"`
		Result  struct {
			ID          string     `json:"id"`
			Name        string     `json:"name"`
			Status      string     `json:"status"`
			CreatedAt   time.Time  `json:"created_at"`
			DeletedAt   *time.Time `json:"deleted_at"`
			Connections []struct {
				ColoName           string    `json:"colo_name"`
				UUID               string    `json:"uuid"`
				IsPendingReconnect bool      `json:"is_pending_reconnect"`
				ClientID           string    `json:"client_id"`
				ClientVersion      string    `json:"client_version"`
				OpenedAt           time.Time `json:"opened_at"`
				OriginIP           string    `json:"origin_ip"`
			} `json:"connections"`
		} `json:"result"`
	}

	if err := json.Unmarshal(bodyBytes, &cfResponse); err != nil {
		return nil, fmt.Errorf("failed to unmarshal tunnel status response: %w. Body: %s", err, string(bodyBytes))
	}

	if !cfResponse.Success {
		var errorStrings []string
		for _, apiErr := range cfResponse.Errors {
			errorStrings = append(errorStrings, apiErr.Error())
		}
		return nil, fmt.Errorf("Cloudflare API error getting tunnel status: %s", strings.Join(errorStrings, "; "))
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("Cloudflare API tunnel status request failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	// Determine last seen time - use the most recent connection or created_at if no connections
	lastSeen := cfResponse.Result.CreatedAt
	for _, conn := range cfResponse.Result.Connections {
		if conn.OpenedAt.After(lastSeen) {
			lastSeen = conn.OpenedAt
		}
	}

	return &TunnelStatus{
		ID:       cfResponse.Result.ID,
		Name:     cfResponse.Result.Name,
		Status:   cfResponse.Result.Status,
		LastSeen: lastSeen,
	}, nil
}

// --- Cloudflare API Interaction Methods ---

// createCloudflareTunnel now manually makes the HTTP request to include config_src: "cloudflare"
func (s *Service) createCloudflareTunnel(ctx context.Context, rc *cloudflare.ResourceContainer, tunnelName string) (*cloudflare.Tunnel, error) {
	s.logger.Info("Manually creating Cloudflare tunnel with name: %s, config_src: cloudflare, under account %s", tunnelName, rc.Identifier)

	apiURL := fmt.Sprintf("%s/accounts/%s/cfd_tunnel", cloudFlareAPIBaseURL, rc.Identifier) // cloudFlareAPIBaseURL defined earlier

	payload := map[string]string{
		"name":       tunnelName,
		"config_src": "cloudflare",
	}

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		s.logger.Error("Failed to marshal payload for create tunnel '%s': %v", tunnelName, err)
		return nil, fmt.Errorf("failed to marshal create tunnel payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewBuffer(jsonPayload))
	if err != nil {
		s.logger.Error("Failed to create HTTP request for create tunnel '%s': %v", tunnelName, err)
		return nil, fmt.Errorf("failed to create HTTP request for tunnel: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+s.config.CloudflareAPIKey)
	req.Header.Set("Content-Type", "application/json")

	httpClient := &http.Client{Timeout: 15 * time.Second} // Use a reasonable timeout
	resp, err := httpClient.Do(req)
	if err != nil {
		s.logger.Error("HTTP client error creating tunnel '%s': %v", tunnelName, err)
		return nil, fmt.Errorf("HTTP client error creating tunnel: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		s.logger.Error("Failed to read response body for create tunnel '%s': %v", tunnelName, err)
		return nil, fmt.Errorf("failed to read create tunnel response body: %w", err)
	}

	// Define a structure to unmarshal the full Cloudflare API response
	// This typically includes a 'result' field for the actual data and success/error fields.
	// Based on common Cloudflare API structure, assuming cloudflare.TunnelResponse is appropriate.
	// If cloudflare.TunnelResponse is not suitable, a custom struct will be needed here.
	var cfResponse struct {
		Success  bool               `json:"success"`
		Errors   []cloudflare.Error `json:"errors"`   // SDK's generic Error type
		Messages []json.RawMessage  `json:"messages"` // Use json.RawMessage for less structured messages
		Result   cloudflare.Tunnel  `json:"result"`
	}

	if err := json.Unmarshal(bodyBytes, &cfResponse); err != nil {
		// If unmarshalling fails, but status code indicates conflict, treat as conflict
		if resp.StatusCode == http.StatusConflict {
			s.logger.Warn("Conflict (HTTP 409) creating tunnel '%s', name likely already exists. Body: %s", tunnelName, string(bodyBytes))
			return nil, ErrTunnelAlreadyExists
		}
		s.logger.Error("Failed to unmarshal JSON response for create tunnel '%s': %v. Body: %s", tunnelName, err, string(bodyBytes))
		return nil, fmt.Errorf("failed to unmarshal create tunnel response: %w. Body: %s", err, string(bodyBytes))
	}

	if !cfResponse.Success {
		// If success is false, also check for HTTP 409 Conflict as a primary indicator
		if resp.StatusCode == http.StatusConflict {
			s.logger.Warn("Conflict (HTTP 409) with success=false creating tunnel '%s', name likely already exists. Errors: %+v, Messages: %+v", tunnelName, cfResponse.Errors, cfResponse.Messages)
			return nil, ErrTunnelAlreadyExists
		}
		s.logger.Error("Cloudflare API returned non-success for create tunnel '%s': Status: %d, Errors: %+v, Messages: %+v, Body: %s",
			tunnelName, resp.StatusCode, cfResponse.Errors, cfResponse.Messages, string(bodyBytes))
		var errorStrings []string
		for _, apiErr := range cfResponse.Errors {
			errorStrings = append(errorStrings, apiErr.Error())
		}
		for _, rawMsg := range cfResponse.Messages {
			errorStrings = append(errorStrings, string(rawMsg))
		}

		if len(errorStrings) == 0 && (resp.StatusCode < 200 || resp.StatusCode >= 300) {
			errorStrings = append(errorStrings, fmt.Sprintf("HTTP status %d: %s", resp.StatusCode, string(bodyBytes)))
		} else if len(errorStrings) == 0 {
			errorStrings = append(errorStrings, "Unknown Cloudflare API error with success=false")
		}
		return nil, fmt.Errorf("Cloudflare API error creating tunnel: %s", strings.Join(errorStrings, "; "))
	}

	// Double check status code even if success is true, as a safeguard.
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		s.logger.Error("Cloudflare API returned non-2xx status code %d for create tunnel '%s' despite success=true. Body: %s", resp.StatusCode, tunnelName, string(bodyBytes))
		return nil, fmt.Errorf("Cloudflare API request failed with status %d despite success=true: %s", resp.StatusCode, string(bodyBytes))
	}

	s.logger.Info("Successfully created Cloudflare Tunnel ID: %s, Name: %s via manual HTTP request", cfResponse.Result.ID, cfResponse.Result.Name)
	return &cfResponse.Result, nil
}

func (s *Service) getCloudflareTunnelClientToken(ctx context.Context, rc *cloudflare.ResourceContainer, tunnelID string) (string, error) {
	s.logger.Info("Manually getting Cloudflare client token for tunnel ID: %s under account %s", tunnelID, rc.Identifier)

	apiURL := fmt.Sprintf("%s/accounts/%s/cfd_tunnel/%s/token", cloudFlareAPIBaseURL, rc.Identifier, tunnelID)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil) // GET for token request
	if err != nil {
		return "", fmt.Errorf("failed to create HTTP request for tunnel token: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+s.config.CloudflareAPIKey)
	req.Header.Set("Content-Type", "application/json") // Important even for GET requests

	httpClient := &http.Client{Timeout: 15 * time.Second}
	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("HTTP client error getting tunnel token: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read tunnel token response body: %w", err)
	}

	// Cloudflare API for tunnel token typically returns a raw string token in the result, not a complex JSON object.
	// However, the overall response is still JSON structured with success, errors, messages, result.
	var cfResponse struct {
		Success  bool               `json:"success"`
		Errors   []cloudflare.Error `json:"errors"`
		Messages []json.RawMessage  `json:"messages"`
		Result   string             `json:"result"` // The token itself is usually the result
	}

	if err := json.Unmarshal(bodyBytes, &cfResponse); err != nil {
		// It's possible the token endpoint on success returns *only* the token string directly, not wrapped in JSON.
		// Let's check for that scenario if status is 200 OK.
		if resp.StatusCode >= 200 && resp.StatusCode < 300 && len(bodyBytes) > 0 {
			// Attempt to use the body directly as the token, assuming it might be a raw string.
			// This is less common for CF but good to be defensive.
			// Clean up potential quotes if it's a JSON string literal returned directly.
			token := strings.Trim(string(bodyBytes), "\"")
			if token != "" {
				s.logger.Info("Successfully fetched tunnel token (treated as raw string) for tunnel %s", tunnelID)
				return token, nil
			}
		}
		s.logger.Error("Failed to unmarshal JSON response for tunnel token '%s': %v. Body: %s", tunnelID, err, string(bodyBytes))
		return "", fmt.Errorf("failed to unmarshal tunnel token response: %w. Body: %s", err, string(bodyBytes))
	}

	if !cfResponse.Success {
		s.logger.Error("Cloudflare API returned non-success for get tunnel token '%s': Errors: %+v, Messages: %+v, Body: %s",
			tunnelID, cfResponse.Errors, cfResponse.Messages, string(bodyBytes))
		var errorStrings []string
		for _, apiErr := range cfResponse.Errors {
			errorStrings = append(errorStrings, apiErr.Error())
		}
		for _, rawMsg := range cfResponse.Messages {
			errorStrings = append(errorStrings, string(rawMsg))
		}
		if len(errorStrings) == 0 {
			errorStrings = append(errorStrings, fmt.Sprintf("HTTP status %d: %s", resp.StatusCode, string(bodyBytes)))
		}
		return "", fmt.Errorf("Cloudflare API error getting tunnel token: %s", strings.Join(errorStrings, "; "))
	}

	// Double check status code even if success is true.
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		s.logger.Error("Cloudflare API returned non-2xx status code %d for get tunnel token '%s' despite success=true. Body: %s", resp.StatusCode, tunnelID, string(bodyBytes))
		return "", fmt.Errorf("Cloudflare API request failed with status %d for tunnel token despite success=true: %s", resp.StatusCode, string(bodyBytes))
	}

	if cfResponse.Result == "" {
		s.logger.Error("Cloudflare API returned success but an empty token for tunnel %s. Body: %s", tunnelID, string(bodyBytes))
		return "", fmt.Errorf("Cloudflare API returned an empty token for tunnel %s", tunnelID)
	}

	s.logger.Info("Successfully fetched Cloudflare Tunnel Token for ID: %s via manual HTTP request", tunnelID)
	return cfResponse.Result, nil
}

func (s *Service) createDNSRecord(ctx context.Context, rc *cloudflare.ResourceContainer, hostname, cnameTarget string) error {
	s.logger.Info("Manually creating DNS CNAME record: %s -> %s (Zone ID: %s)", hostname, cnameTarget, rc.Identifier)

	apiURL := fmt.Sprintf("%s/zones/%s/dns_records", cloudFlareAPIBaseURL, rc.Identifier)

	payload := map[string]interface{}{
		"type":    "CNAME",
		"name":    hostname, // Use the full hostname as per your working curl
		"content": cnameTarget,
		"ttl":     1,
		"proxied": true,
	}

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal create DNS payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewBuffer(jsonPayload))
	if err != nil {
		return fmt.Errorf("failed to create HTTP request for DNS: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+s.config.CloudflareAPIKey)
	req.Header.Set("Content-Type", "application/json")

	httpClient := &http.Client{Timeout: 15 * time.Second}
	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("HTTP client error creating DNS: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read DNS response body: %w", err)
	}

	var cfResponse struct {
		Success bool               `json:"success"`
		Errors  []cloudflare.Error `json:"errors"`
		Result  interface{}        `json:"result"` // DNS result is different from tunnel
	}

	if err := json.Unmarshal(bodyBytes, &cfResponse); err != nil {
		return fmt.Errorf("failed to unmarshal DNS response: %w. Body: %s", err, string(bodyBytes))
	}

	if !cfResponse.Success {
		var errorMessages []string
		for _, apiErr := range cfResponse.Errors {
			errorMessages = append(errorMessages, apiErr.Error())
		}
		return fmt.Errorf("Cloudflare API error creating DNS: %s", strings.Join(errorMessages, "; "))
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("Cloudflare API DNS request failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	s.logger.Info("Successfully created DNS record for %s via manual HTTP request", hostname)
	return nil
}

func (s *Service) updateCloudflareTunnelIngress(ctx context.Context, rc *cloudflare.ResourceContainer, tunnelID, publicHostname, serviceURL string) error {
	s.logger.Info("Manually updating Cloudflare tunnel %s configuration for hostname %s to service %s under account %s", tunnelID, publicHostname, serviceURL, rc.Identifier)

	apiURL := fmt.Sprintf("%s/accounts/%s/cfd_tunnel/%s/configurations", cloudFlareAPIBaseURL, rc.Identifier, tunnelID)

	// Construct ingress rules payload
	payload := map[string]interface{}{
		"config": map[string]interface{}{
			"ingress": []map[string]interface{}{
				{
					"hostname":      publicHostname,
					"service":       serviceURL,
					"originRequest": map[string]interface{}{},
				},
				{
					"service": "http_status:404",
				},
			},
		},
	}

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal tunnel configuration payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, apiURL, bytes.NewBuffer(jsonPayload))
	if err != nil {
		return fmt.Errorf("failed to create HTTP request for tunnel configuration: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+s.config.CloudflareAPIKey)
	req.Header.Set("Content-Type", "application/json")

	httpClient := &http.Client{Timeout: 15 * time.Second}
	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("HTTP client error updating tunnel configuration: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read tunnel configuration response body: %w", err)
	}

	var cfResponse struct {
		Success bool               `json:"success"`
		Errors  []cloudflare.Error `json:"errors"`
		Result  interface{}        `json:"result"` // Configuration result can vary
	}

	if err := json.Unmarshal(bodyBytes, &cfResponse); err != nil {
		return fmt.Errorf("failed to unmarshal tunnel configuration response: %w. Body: %s", err, string(bodyBytes))
	}

	if !cfResponse.Success {
		var errorStrings []string
		for _, apiErr := range cfResponse.Errors {
			errorStrings = append(errorStrings, apiErr.Error())
		}
		return fmt.Errorf("Cloudflare API error updating tunnel configuration: %s", strings.Join(errorStrings, "; "))
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("Cloudflare API tunnel configuration request failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	s.logger.Info("Successfully updated tunnel configuration for tunnel ID: %s via manual HTTP request", tunnelID)
	return nil
}

func (s *Service) deleteCloudflareTunnel(ctx context.Context, rc *cloudflare.ResourceContainer, tunnelID string) error {
	s.logger.Warn("Manually attempting to delete Cloudflare tunnel ID: %s under account %s", tunnelID, rc.Identifier)

	apiURL := fmt.Sprintf("%s/accounts/%s/cfd_tunnel/%s", cloudFlareAPIBaseURL, rc.Identifier, tunnelID)

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, apiURL, nil) // No body for DELETE
	if err != nil {
		return fmt.Errorf("failed to create HTTP request for delete tunnel: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+s.config.CloudflareAPIKey)
	req.Header.Set("Content-Type", "application/json") // Good practice, though no body

	httpClient := &http.Client{Timeout: 15 * time.Second}
	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("HTTP client error deleting tunnel: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read delete tunnel response body: %w", err)
	}

	var cfResponse struct {
		Success bool               `json:"success"`
		Errors  []cloudflare.Error `json:"errors"`
		Result  cloudflare.Tunnel  `json:"result"` // Tunnel is usually returned on delete
	}

	if err := json.Unmarshal(bodyBytes, &cfResponse); err != nil {
		// If unmarshalling fails, it might be a non-JSON error or an empty body on success for DELETE
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			s.logger.Info("Successfully deleted Cloudflare tunnel ID: %s (non-JSON or empty response body, relying on status code)", tunnelID)
			return nil
		}
		return fmt.Errorf("failed to unmarshal delete tunnel response: %w. Body: %s", err, string(bodyBytes))
	}

	if !cfResponse.Success {
		var errorMessages []string
		for _, apiErr := range cfResponse.Errors {
			errorMessages = append(errorMessages, apiErr.Error())
		}
		// If success is false but no errors in the JSON, and status code is an error, use that info.
		if len(errorMessages) == 0 && (resp.StatusCode < 200 || resp.StatusCode >= 300) {
			errorMessages = append(errorMessages, fmt.Sprintf("HTTP status %d: %s", resp.StatusCode, string(bodyBytes)))
		} else if len(errorMessages) == 0 {
			errorMessages = append(errorMessages, "Unknown Cloudflare API error deleting tunnel with success=false")
		}
		return fmt.Errorf("Cloudflare API error deleting tunnel: %s", strings.Join(errorMessages, "; "))
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("Cloudflare API delete tunnel request failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	s.logger.Info("Successfully deleted Cloudflare tunnel ID: %s via manual HTTP request", cfResponse.Result.ID)
	return nil
}

func (s *Service) deleteDNSRecordByContent(ctx context.Context, rc *cloudflare.ResourceContainer, hostname, cnameContent string) error {
	s.logger.Warn("Manually attempting to delete DNS CNAME record for %s pointing to %s in zone %s", hostname, cnameContent, rc.Identifier)

	// Step 1: List DNS records to find the one to delete
	listAPIURL := fmt.Sprintf("%s/zones/%s/dns_records", cloudFlareAPIBaseURL, rc.Identifier)
	listReq, err := http.NewRequestWithContext(ctx, http.MethodGet, listAPIURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create HTTP request for list DNS: %w", err)
	}

	q := listReq.URL.Query()
	q.Add("type", "CNAME")
	// Cloudflare API expects the name to be the FQDN for listing/filtering
	q.Add("name", hostname)
	q.Add("content", cnameContent)
	listReq.URL.RawQuery = q.Encode()

	listReq.Header.Set("Authorization", "Bearer "+s.config.CloudflareAPIKey)
	listReq.Header.Set("Content-Type", "application/json")

	httpClient := &http.Client{Timeout: 15 * time.Second}
	listResp, err := httpClient.Do(listReq)
	if err != nil {
		return fmt.Errorf("HTTP client error listing DNS records: %w", err)
	}
	defer listResp.Body.Close()

	listBodyBytes, err := io.ReadAll(listResp.Body)
	if err != nil {
		return fmt.Errorf("failed to read list DNS response body: %w", err)
	}

	var listCFResponse struct {
		Success bool                   `json:"success"`
		Errors  []cloudflare.Error     `json:"errors"`
		Result  []cloudflare.DNSRecord `json:"result"` // Expecting a list of DNS records
	}

	if err := json.Unmarshal(listBodyBytes, &listCFResponse); err != nil {
		return fmt.Errorf("failed to unmarshal list DNS response: %w. Body: %s", err, string(listBodyBytes))
	}

	if !listCFResponse.Success {
		var errorMessages []string
		for _, apiErr := range listCFResponse.Errors {
			errorMessages = append(errorMessages, apiErr.Error())
		}
		return fmt.Errorf("Cloudflare API error listing DNS: %s", strings.Join(errorMessages, "; "))
	}
	if listResp.StatusCode < 200 || listResp.StatusCode >= 300 {
		return fmt.Errorf("Cloudflare API list DNS request failed with status %d: %s", listResp.StatusCode, string(listBodyBytes))
	}

	if len(listCFResponse.Result) == 0 {
		s.logger.Info("No DNS record found for %s pointing to %s via manual list. Nothing to delete.", hostname, cnameContent)
		return nil
	}

	for _, record := range listCFResponse.Result {
		s.logger.Info("Found DNS record ID %s for %s. Deleting via manual HTTP request.", record.ID, record.Name)

		// Step 2: Delete the found DNS record by its ID
		deleteAPIURL := fmt.Sprintf("%s/zones/%s/dns_records/%s", cloudFlareAPIBaseURL, rc.Identifier, record.ID)
		deleteReq, err := http.NewRequestWithContext(ctx, http.MethodDelete, deleteAPIURL, nil)
		if err != nil {
			return fmt.Errorf("failed to create HTTP request for delete DNS record %s: %w", record.ID, err)
		}
		deleteReq.Header.Set("Authorization", "Bearer "+s.config.CloudflareAPIKey)
		deleteReq.Header.Set("Content-Type", "application/json")

		deleteResp, err := httpClient.Do(deleteReq) // Re-use httpClient
		if err != nil {
			return fmt.Errorf("HTTP client error deleting DNS record %s: %w", record.ID, err)
		}
		defer deleteResp.Body.Close()

		deleteBodyBytes, err := io.ReadAll(deleteResp.Body)
		if err != nil {
			s.logger.Error("Failed to read response body for delete DNS record %s: %v", record.ID, err)
			// Continue to next record if there are multiple, or return error if this is the only one.
			// For simplicity, we'll return error here, but in a multi-match scenario, might log and continue.
			return fmt.Errorf("failed to read delete DNS response body for record %s: %w", record.ID, err)
		}

		var deleteCFResponse struct {
			Success bool               `json:"success"`
			Errors  []cloudflare.Error `json:"errors"`
			Result  struct {
				ID string `json:"id"`
			} `json:"result"` // Delete often returns the ID of deleted object
		}

		if err := json.Unmarshal(deleteBodyBytes, &deleteCFResponse); err != nil {
			// If unmarshalling fails, check status code for success on DELETE
			if deleteResp.StatusCode >= 200 && deleteResp.StatusCode < 300 {
				s.logger.Info("Successfully deleted DNS record ID: %s (non-JSON or empty response, relying on status code)", record.ID)
				continue // Successfully deleted this one, move to next if any
			}
			return fmt.Errorf("failed to unmarshal delete DNS response for record %s: %w. Body: %s", record.ID, err, string(deleteBodyBytes))
		}

		if !deleteCFResponse.Success {
			var errorMessages []string
			for _, apiErr := range deleteCFResponse.Errors {
				errorMessages = append(errorMessages, apiErr.Error())
			}
			return fmt.Errorf("Cloudflare API error deleting DNS record %s: %s", record.ID, strings.Join(errorMessages, "; "))
		}
		if deleteResp.StatusCode < 200 || deleteResp.StatusCode >= 300 {
			return fmt.Errorf("Cloudflare API delete DNS record %s request failed with status %d: %s", record.ID, deleteResp.StatusCode, string(deleteBodyBytes))
		}
		s.logger.Info("Successfully deleted DNS record ID: %s for %s via manual HTTP request", record.ID, record.Name)
	}
	return nil
}

// Helper function min
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// TODO:
// - Review Cloudflare API rate limits and add appropriate backoff/retry logic if the SDK's default isn't sufficient (though it's usually good).
// - Consider atomicity for multi-step operations (e.g., if DB write fails after CF Tunnel/DNS creation, ensure cleanup). The current code has some basic cleanup attempts.
// - Enhance logging with more contextual information.
// - Ensure the `common.InternalOnly` middleware in main.go is correctly configured for this service.
// - The placeholder logic in `validateAPIKeyAndGetDetails` has been replaced. Double-check `common.AuthService` and `common.ContextKeyInternalAPIKey`

// Define new error type at package level or in a common errors file
var ErrTunnelAlreadyExists = errors.New("tunnel with this name already exists in Cloudflare")
var ErrTunnelNotFoundByName = errors.New("tunnel with this name not found in Cloudflare listing")

// getCloudflareTunnelByName tries to find an existing tunnel by its name.
func (s *Service) getCloudflareTunnelByName(ctx context.Context, rc *cloudflare.ResourceContainer, tunnelName string) (*cloudflare.Tunnel, error) {
	s.logger.Info("Attempting to find existing Cloudflare tunnel by name: %s under account %s", tunnelName, rc.Identifier)

	// The endpoint for listing tunnels. Note: CF API might use slightly different path for Zero Trust Tunnels listing.
	// Let's assume /cfd_tunnel can be listed with a name filter.
	apiURL := fmt.Sprintf("%s/accounts/%s/cfd_tunnel", cloudFlareAPIBaseURL, rc.Identifier)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request to list tunnels: %w", err)
	}

	q := req.URL.Query()
	q.Add("name", tunnelName)
	// q.Add("is_deleted", "false") // Check if API supports this filter
	req.URL.RawQuery = q.Encode()

	req.Header.Set("Authorization", "Bearer "+s.config.CloudflareAPIKey)
	req.Header.Set("Content-Type", "application/json")

	httpClient := &http.Client{Timeout: 15 * time.Second}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP client error listing tunnels by name '%s': %w", tunnelName, err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read list tunnels response body: %w", err)
	}

	var cfListResponse struct {
		Success bool                `json:"success"`
		Errors  []cloudflare.Error  `json:"errors"`
		Result  []cloudflare.Tunnel `json:"result"` // Expecting an array of tunnels
	}

	if err := json.Unmarshal(bodyBytes, &cfListResponse); err != nil {
		return nil, fmt.Errorf("failed to unmarshal list tunnels response: %w. Body: %s", err, string(bodyBytes))
	}

	if !cfListResponse.Success {
		var errorStrings []string
		for _, apiErr := range cfListResponse.Errors {
			errorStrings = append(errorStrings, apiErr.Error())
		}
		return nil, fmt.Errorf("Cloudflare API error listing tunnels by name '%s': %s", tunnelName, strings.Join(errorStrings, "; "))
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("Cloudflare API list tunnels request failed with status %d for name '%s': %s", resp.StatusCode, tunnelName, string(bodyBytes))
	}

	for _, tunnel := range cfListResponse.Result {
		if tunnel.Name == tunnelName {
			s.logger.Info("Found existing tunnel ID %s by name '%s' in Cloudflare listing.", tunnel.ID, tunnelName)
			return &tunnel, nil
		}
	}

	s.logger.Warn("Tunnel with name '%s' not found in Cloudflare listing for account %s.", tunnelName, rc.Identifier)
	return nil, ErrTunnelNotFoundByName
}

// BulkCleanupTunnels handles cleanup of inactive tunnels across all providers (internal only)
func (s *Service) BulkCleanupTunnels(ctx context.Context, days int) (*CleanupTunnelResponse, error) {
	s.logger.Info("BulkCleanupTunnels called with %d days threshold for all provider tunnels", days)

	// Default to 30 days if not specified or invalid
	if days <= 0 {
		days = 30
		s.logger.Info("Using default threshold of %d days for tunnel cleanup", days)
	}

	accountRC := cloudflare.AccountIdentifier(s.config.CloudflareAccountID)
	zoneRC := cloudflare.ZoneIdentifier(s.config.CloudflareZoneID)

	// Get all tunnels across all providers
	rows, err := s.db.QueryContext(ctx,
		"SELECT tunnel_id, hostname, provider_id FROM provider_tunnels",
	)
	if err != nil {
		s.logger.Error("Failed to query all provider tunnels: %v", err)
		return &CleanupTunnelResponse{
			Message: fmt.Sprintf("Database error: %v", err),
		}, nil
	}
	defer rows.Close()

	var allTunnels []struct {
		TunnelID   string
		Hostname   string
		ProviderID uuid.UUID
	}

	for rows.Next() {
		var tunnel struct {
			TunnelID   string
			Hostname   string
			ProviderID uuid.UUID
		}
		if err := rows.Scan(&tunnel.TunnelID, &tunnel.Hostname, &tunnel.ProviderID); err != nil {
			s.logger.Error("Failed to scan tunnel row: %v", err)
			continue
		}
		allTunnels = append(allTunnels, tunnel)
	}

	var deletedTunnels []string
	var activeTunnels []string
	var failedCleanups []string
	totalChecked := len(allTunnels)

	s.logger.Info("Found %d total tunnels to check for cleanup", totalChecked)

	for _, tunnel := range allTunnels {
		// Get tunnel status from Cloudflare
		status, err := s.getTunnelStatus(ctx, accountRC, tunnel.TunnelID)
		if err != nil {
			s.logger.Error("Failed to get tunnel status for %s: %v", tunnel.TunnelID, err)
			failedCleanups = append(failedCleanups, fmt.Sprintf("%s (status check failed: %v)", tunnel.TunnelID, err))
			continue
		}

		// Check if tunnel is inactive for more than specified days
		daysSinceLastSeen := int(time.Since(status.LastSeen).Hours() / 24)
		if daysSinceLastSeen > days {
			s.logger.Info("Tunnel %s inactive for %d days (threshold: %d), deleting", tunnel.TunnelID, daysSinceLastSeen, days)

			// Delete DNS record first
			if err := s.deleteDNSRecordByContent(ctx, zoneRC, tunnel.Hostname, fmt.Sprintf("%s.cfargotunnel.com", tunnel.TunnelID)); err != nil {
				s.logger.Error("Failed to delete DNS record for tunnel %s: %v", tunnel.TunnelID, err)
				failedCleanups = append(failedCleanups, fmt.Sprintf("%s (DNS deletion failed: %v)", tunnel.TunnelID, err))
				continue
			}

			// Delete Cloudflare tunnel
			if err := s.deleteCloudflareTunnel(ctx, accountRC, tunnel.TunnelID); err != nil {
				s.logger.Error("Failed to delete Cloudflare tunnel %s: %v", tunnel.TunnelID, err)
				failedCleanups = append(failedCleanups, fmt.Sprintf("%s (tunnel deletion failed: %v)", tunnel.TunnelID, err))
				continue
			}

			// Remove from database
			if _, err := s.db.ExecContext(ctx, "DELETE FROM provider_tunnels WHERE tunnel_id = $1", tunnel.TunnelID); err != nil {
				s.logger.Error("Failed to delete tunnel %s from database: %v", tunnel.TunnelID, err)
				failedCleanups = append(failedCleanups, fmt.Sprintf("%s (DB deletion failed: %v)", tunnel.TunnelID, err))
				continue
			}

			deletedTunnels = append(deletedTunnels, tunnel.TunnelID)
			s.logger.Info("Successfully deleted tunnel %s", tunnel.TunnelID)
		} else {
			s.logger.Debug("Tunnel %s is still active (last seen %d days ago)", tunnel.TunnelID, daysSinceLastSeen)
			activeTunnels = append(activeTunnels, tunnel.TunnelID)
		}
	}

	totalDeleted := len(deletedTunnels)
	message := fmt.Sprintf("Cleanup completed: checked %d tunnels, deleted %d inactive tunnels (threshold: %d days)", totalChecked, totalDeleted, days)

	if len(failedCleanups) > 0 {
		message += fmt.Sprintf(", %d cleanups failed", len(failedCleanups))
	}

	s.logger.Info(message)

	return &CleanupTunnelResponse{
		TotalChecked:   totalChecked,
		TotalDeleted:   totalDeleted,
		DeletedTunnels: deletedTunnels,
		ActiveTunnels:  activeTunnels,
		FailedCleanups: failedCleanups,
		Message:        message,
	}, nil
}
