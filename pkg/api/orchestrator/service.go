package orchestrator

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/sentnl/inferoute-node/internal/config"
	"github.com/sentnl/inferoute-node/internal/db"
	"github.com/sentnl/inferoute-node/pkg/common"
	"github.com/sentnl/inferoute-node/pkg/rabbitmq"
)

// Service handles orchestration of requests
type Service struct {
	db             *db.DB
	logger         *common.Logger
	rmq            *rabbitmq.Client
	internalAPIKey string
	userID         uuid.UUID // Stores the current user's ID
}

// NewService creates a new orchestration service
func NewService(db *db.DB, logger *common.Logger, rmq *rabbitmq.Client, internalAPIKey string) *Service {
	return &Service{
		db:             db,
		logger:         logger,
		rmq:            rmq,
		internalAPIKey: internalAPIKey,
	}
}

// ProcessRequest handles the main orchestration flow
func (s *Service) ProcessRequest(ctx context.Context, consumerID uuid.UUID, req *OpenAIRequest) (interface{}, error) {
	totalStartTime := time.Now()

	// Add max_tokens and temperature to context if they exist in the request
	if req.MaxTokens > 0 {
		ctx = context.WithValue(ctx, "max_tokens", req.MaxTokens)
		s.logger.Info("Added max_tokens=%d to context", req.MaxTokens)
	}

	if req.Temperature != 0 {
		ctx = context.WithValue(ctx, "temperature", req.Temperature)
		s.logger.Info("Added temperature=%f to context", req.Temperature)
	}

	// 1. Validate API key
	authStartTime := time.Now()
	authReq := map[string]interface{}{
		"api_key": ctx.Value("api_key").(string),
	}

	s.logger.Info("Validating API key and checking balance")
	authResp, err := common.MakeInternalRequest(
		ctx,
		"POST",
		common.AuthService,
		"/api/auth/validate",
		authReq,
	)
	s.logger.Info("Auth validation took: %dms", time.Since(authStartTime).Milliseconds())
	if err != nil {
		s.logger.Error("Failed to validate API key: %v", err)
		return nil, fmt.Errorf("failed to validate API key: %w", err)
	}

	// Check if API key is valid
	if valid, ok := authResp["valid"].(bool); !ok || !valid {
		s.logger.Error("API key validation failed")
		return nil, common.ErrUnauthorized(fmt.Errorf("invalid API key"))
	}

	// Store user_id from auth response
	userIDStr, ok := authResp["user_id"].(string)
	if !ok {
		s.logger.Error("Failed to get user_id from auth response")
		return nil, common.ErrInternalServer(fmt.Errorf("failed to get user information"))
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		s.logger.Error("Failed to parse user_id: %v", err)
		return nil, common.ErrInternalServer(fmt.Errorf("invalid user_id format"))
	}
	s.userID = userID

	// Check if user has sufficient balance
	availableBalance, ok := authResp["available_balance"].(float64)
	if !ok {
		s.logger.Error("Failed to get available balance from auth response")
		return nil, common.ErrInternalServer(fmt.Errorf("failed to get balance information"))
	}

	s.logger.Info("Current balance - Available: %v", availableBalance)
	if availableBalance < 1.0 {
		s.logger.Error("Insufficient funds: available_balance=%v", availableBalance)
		return nil, common.ErrInsufficientFunds(fmt.Errorf("insufficient funds: minimum $1.00 required"))
	}

	// //1. Get user settings
	userSettingsStartTime := time.Now()
	userSettings, err := s.getUserSettings(ctx)
	s.logger.Info("Getting user settings took: %dms", time.Since(userSettingsStartTime).Milliseconds())
	if err != nil {
		s.logger.Error("Failed to get user settings: %v", err)
		return nil, fmt.Errorf("failed to get user settings: %w", err)
	}

	// 2. Get consumer settings (global and model-specific)
	consumerSettingsStartTime := time.Now()
	settings, err := s.getConsumerSettings(ctx, consumerID, req.Model)
	s.logger.Info("Getting consumer settings took: %dms", time.Since(consumerSettingsStartTime).Milliseconds())
	if err != nil {
		return nil, fmt.Errorf("failed to get consumer settings: %w", err)
	}

	var userProviders []ProviderInfo
	if userSettings.DefaultToOwnModels {
		// Only get user providers if DefaultToOwnModels is true
		userProvidersStartTime := time.Now()
		userProviders, err = s.getUserProviders(ctx, req.Model)
		s.logger.Info("Getting user providers took: %dms", time.Since(userProvidersStartTime).Milliseconds())
		if err != nil {
			return nil, fmt.Errorf("failed to get user providers: %w", err)
		}
		s.logger.Info("Found %d user providers with DefaultToOwnModels=true", len(userProviders))
	}

	// 3.b Get healthy providers within price constraints
	providersStartTime := time.Now()
	providers, err := s.getHealthyProviders(ctx, req.Model, settings)
	s.logger.Info("Getting healthy providers took: %dms", time.Since(providersStartTime).Milliseconds())
	if err != nil {
		return nil, fmt.Errorf("failed to get healthy providers: %w", err)
	}

	if len(providers) == 0 {
		return nil, fmt.Errorf("no healthy providers available for model %s within price constraints", req.Model)
	}

	// 4. Select best providers based on price and latency
	selectProvidersStartTime := time.Now()
	selectedProviders := s.selectBestProviders(providers, req.Sort)
	s.logger.Info("Selecting best providers took: %dms", time.Since(selectProvidersStartTime).Milliseconds())
	if len(selectedProviders) == 0 {
		return nil, fmt.Errorf("no suitable providers found for model %s", req.Model)
	}

	// If we have user providers, prioritize them
	if len(userProviders) > 0 {
		s.logger.Info("GERT: %v", userProviders)
		// Get up to 3 user providers
		userProviderCount := min(3, len(userProviders))
		userProvidersList := userProviders[:userProviderCount]

		// Get up to 3 non-user providers
		nonUserProviderCount := min(3, len(selectedProviders))
		nonUserProvidersList := selectedProviders[:nonUserProviderCount]

		// Combine lists with user providers first
		combinedProviders := make([]ProviderInfo, 0, userProviderCount+nonUserProviderCount)
		combinedProviders = append(combinedProviders, userProvidersList...)
		combinedProviders = append(combinedProviders, nonUserProvidersList...)

		s.logger.Info("Combined %d user providers with %d non-user providers",
			userProviderCount, nonUserProviderCount)

		selectedProviders = combinedProviders
	}

	// Use first provider for transaction record
	selectedProvider := selectedProviders[0]

	// 5. Generate HMAC
	hmacStartTime := time.Now()
	hmac, err := s.generateHMAC(ctx, consumerID, req)
	s.logger.Info("Generating HMAC took: %dms", time.Since(hmacStartTime).Milliseconds())
	if err != nil {
		return nil, fmt.Errorf("failed to generate HMAC: %w", err)
	}

	// 6. Create transaction record
	txStartTime := time.Now()
	tx, err := s.createTransaction(ctx, consumerID, selectedProvider, req.Model, hmac)
	s.logger.Info("Creating transaction took: %dms", time.Since(txStartTime).Milliseconds())
	if err != nil {
		return nil, fmt.Errorf("failed to create transaction: %w", err)
	}

	// 7. Place holding deposit
	holdingStartTime := time.Now()
	if err := s.placeHoldingDeposit(ctx); err != nil {
		return nil, fmt.Errorf("failed to place holding deposit: %w", err)
	}
	s.logger.Info("Placing holding deposit took: %dms", time.Since(holdingStartTime).Milliseconds())

	// 8. Send request to provider
	startTime := time.Now()
	response, successfulProvider, err := s.sendRequestToProvider(ctx, selectedProviders, req, hmac)
	providerRequestTime := time.Since(startTime).Milliseconds()
	s.logger.Info("Provider request took: %dms", providerRequestTime)
	if err != nil {
		// Release holding deposit on error
		_ = s.releaseHoldingDeposit(ctx)

		// Update transaction status to 'canceled' since all providers failed
		if cancelErr := s.cancelTransaction(ctx, tx.ID); cancelErr != nil {
			s.logger.Error("Failed to cancel transaction: %v", cancelErr)
		} else {
			s.logger.Info("Transaction %s canceled: all providers failed", tx.ID)
		}

		return nil, fmt.Errorf("failed to send request to provider: %w", err)
	}
	latency := time.Since(startTime).Milliseconds()

	// 9. Release holding deposit
	releaseStartTime := time.Now()
	if err := s.releaseHoldingDeposit(ctx); err != nil {
		s.logger.Error("Failed to release holding deposit: %v", err)
	}
	s.logger.Info("Releasing holding deposit took: %dms", time.Since(releaseStartTime).Milliseconds())

	// 10. Update transaction and publish payment message
	finalizeStartTime := time.Now()
	if err := s.finalizeTransaction(ctx, tx, response, latency, successfulProvider); err != nil {
		s.logger.Error("Failed to finalize transaction: %v", err)
	}
	s.logger.Info("Finalizing transaction took: %dms", time.Since(finalizeStartTime).Milliseconds())

	totalTime := time.Since(totalStartTime).Milliseconds()
	s.logger.Info("Total orchestration time: %dms (Provider request: %dms, Overhead: %dms)",
		totalTime, providerRequestTime, totalTime-providerRequestTime)

	return response, nil
}

// UserSettings represents user-specific settings
type UserSettings struct {
	DefaultToOwnModels bool
}

// getUserSettings fetches the user's settings from the database
func (s *Service) getUserSettings(ctx context.Context) (*UserSettings, error) {
	var settings UserSettings
	err := s.db.QueryRowContext(ctx,
		`SELECT default_to_own_models
		FROM user_settings
		WHERE user_id = $1`,
		s.userID,
	).Scan(&settings.DefaultToOwnModels)

	if err == sql.ErrNoRows {
		// If no settings found, return default values
		return &UserSettings{
			DefaultToOwnModels: true, // Default value as per schema
		}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("error getting user settings: %w", err)
	}

	s.logger.Info("Got user settings - DefaultToOwnModels: %v", settings.DefaultToOwnModels)
	return &settings, nil
}

// getConsumerSettings gets the consumer's price settings, including any model-specific overrides
func (s *Service) getConsumerSettings(ctx context.Context, consumerID uuid.UUID, model string) (*ConsumerSettings, error) {
	// First check for model-specific settings
	var settings ConsumerSettings
	err := s.db.QueryRowContext(ctx,
		`SELECT max_input_price_tokens, max_output_price_tokens
		FROM consumer_models
		WHERE consumer_id = $1 AND model_name = $2`,
		consumerID, model,
	).Scan(&settings.MaxInputPriceTokens, &settings.MaxOutputPriceTokens)

	if err == sql.ErrNoRows {
		// If no model-specific settings, get global settings
		err = s.db.QueryRowContext(ctx,
			`SELECT max_input_price_tokens, max_output_price_tokens
			FROM consumers
			WHERE id = $1`,
			consumerID,
		).Scan(&settings.MaxInputPriceTokens, &settings.MaxOutputPriceTokens)

		if err == sql.ErrNoRows {
			return nil, common.ErrNotFound(fmt.Errorf("consumer not found"))
		}
		if err != nil {
			return nil, fmt.Errorf("error getting consumer settings: %w", err)
		}
	} else if err != nil {
		return nil, fmt.Errorf("error getting model-specific settings: %w", err)
	}

	return &settings, nil
}

// getUserProviders gets a list of providers that belong to the user
func (s *Service) getUserProviders(ctx context.Context, model string) ([]ProviderInfo, error) {
	// Make request to health service with query parameters
	response, err := common.MakeInternalRequestRaw(
		ctx,
		"GET",
		common.ProviderHealthService,
		fmt.Sprintf("/api/health/providers/user?user_id=%s&model_name=%s", s.userID, model),
		nil, // No body for GET request
	)
	if err != nil {
		return nil, fmt.Errorf("error getting user providers from health service: %w", err)
	}

	// Parse response as array
	var providersData []map[string]interface{}
	if err := json.Unmarshal(response, &providersData); err != nil {
		return nil, fmt.Errorf("error decoding response: %w", err)
	}

	s.logger.Info("Got response from health service: %v", string(response))
	s.logger.Info("Number of user providers returned: %d", len(providersData))

	// Convert response to ProviderInfo slice
	providers := make([]ProviderInfo, 0, len(providersData))
	for _, providerMap := range providersData {
		// Skip if any required field is nil
		if providerMap["provider_id"] == nil ||
			providerMap["input_cost"] == nil ||
			providerMap["output_cost"] == nil ||
			providerMap["tier"] == nil ||
			providerMap["health_status"] == nil ||
			providerMap["average_tps"] == nil ||
			providerMap["api_url"] == nil {
			s.logger.Error("Skipping provider with nil fields: %v", providerMap)
			continue
		}

		// Skip if API URL is empty
		apiURL := providerMap["api_url"].(string)
		if apiURL == "" {
			s.logger.Error("Skipping provider %s with empty API URL", providerMap["provider_id"])
			continue
		}

		provider := ProviderInfo{
			ProviderID:        uuid.MustParse(providerMap["provider_id"].(string)),
			URL:               apiURL,
			InputPriceTokens:  providerMap["input_cost"].(float64),
			OutputPriceTokens: providerMap["output_cost"].(float64),
			Tier:              int(providerMap["tier"].(float64)),
			HealthStatus:      providerMap["health_status"].(string),
			AverageTPS:        providerMap["average_tps"].(float64),
		}
		providers = append(providers, provider)
	}

	s.logger.Info("Found %d valid user providers after filtering", len(providers))
	return providers, nil
}

// getHealthyProviders gets a list of healthy providers that support the requested model
func (s *Service) getHealthyProviders(ctx context.Context, model string, settings *ConsumerSettings) ([]ProviderInfo, error) {
	// Calculate max cost from input and output prices
	maxCost := settings.MaxInputPriceTokens + settings.MaxOutputPriceTokens

	// Make request to health service with query parameters
	response, err := common.MakeInternalRequestRaw(
		ctx,
		"GET",
		common.ProviderHealthService,
		fmt.Sprintf("/api/health/providers/filter?model_name=%s&max_cost=%f", model, maxCost),
		nil, // No body for GET request
	)
	if err != nil {
		return nil, fmt.Errorf("error getting providers from health service: %w", err)
	}

	// Parse response as array
	var providersData []map[string]interface{}
	if err := json.Unmarshal(response, &providersData); err != nil {
		return nil, fmt.Errorf("error decoding response: %w", err)
	}

	s.logger.Info("Got response from health service: %v", string(response))
	s.logger.Info("Number of providers returned: %d", len(providersData))

	// Convert response to ProviderInfo slice
	providers := make([]ProviderInfo, 0, len(providersData))
	for _, providerMap := range providersData {
		// Skip if any required field is nil
		if providerMap["provider_id"] == nil ||
			providerMap["input_cost"] == nil ||
			providerMap["output_cost"] == nil ||
			providerMap["tier"] == nil ||
			providerMap["health_status"] == nil ||
			providerMap["average_tps"] == nil ||
			providerMap["api_url"] == nil {
			s.logger.Error("Skipping provider with nil fields: %v", providerMap)
			continue
		}

		// Skip if API URL is empty
		apiURL := providerMap["api_url"].(string)
		if apiURL == "" {
			s.logger.Error("Skipping provider %s with empty API URL", providerMap["provider_id"])
			continue
		}

		provider := ProviderInfo{
			ProviderID:        uuid.MustParse(providerMap["provider_id"].(string)),
			URL:               apiURL,
			InputPriceTokens:  providerMap["input_cost"].(float64),
			OutputPriceTokens: providerMap["output_cost"].(float64),
			Tier:              int(providerMap["tier"].(float64)),
			HealthStatus:      providerMap["health_status"].(string),
			AverageTPS:        providerMap["average_tps"].(float64),
		}
		providers = append(providers, provider)
	}

	if len(providers) == 0 {
		s.logger.Error("No valid providers found after filtering. Raw response: %v", string(response))
		return nil, fmt.Errorf("no valid providers available for model %s within price constraints", model)
	}

	s.logger.Info("Found %d valid providers after filtering", len(providers))
	return providers, nil
}

func (s *Service) selectBestProviders(providers []ProviderInfo, sortBy string) []ProviderInfo {
	if len(providers) == 0 {
		s.logger.Error("selectBestProviders called with empty providers list")
		return nil
	}

	// Create a slice to store provider scores
	type scoredProvider struct {
		provider ProviderInfo
		score    float64
	}

	scored := make([]scoredProvider, 0, len(providers))
	s.logger.Info("Scoring %d providers", len(providers))

	// Find max TPS for normalization
	var maxTPS float64
	for _, p := range providers {
		if p.AverageTPS > maxTPS {
			maxTPS = p.AverageTPS
		}
	}

	// Set weights based on sort preference
	var priceWeight, tpsWeight float64
	switch sortBy {
	case "throughput":
		s.logger.Info("Sorting by throughput")
		priceWeight = 0.2 // 20% price
		tpsWeight = 0.8   // 80% throughput
	default: // "cost" or empty
		s.logger.Info("Sorting by cost")
		priceWeight = 0.7 // 70% price
		tpsWeight = 0.3   // 30% throughput
	}

	// Calculate scores for all providers
	for _, p := range providers {
		totalPrice := p.InputPriceTokens + p.OutputPriceTokens

		// Normalize scores to 0-1 range
		// For price: lower is better, so we invert the relationship
		priceScore := 1.0 - (totalPrice / (totalPrice + 1.0)) // Asymptotic normalization

		// For TPS: higher is better, normalize against max TPS
		tpsScore := p.AverageTPS / maxTPS

		// Apply weights and combine scores
		score := (priceScore * priceWeight) + (tpsScore * tpsWeight)

		scored = append(scored, scoredProvider{p, score})
		s.logger.Info("Provider %s scored %f (price: %f, tps: %f, normalized_price_score: %f, normalized_tps_score: %f, sort: %s)",
			p.ProviderID, score, totalPrice, p.AverageTPS, priceScore, tpsScore, sortBy)
	}

	// Sort by score in descending order
	sort.Slice(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})

	// Return top 3 providers (or all if less than 3)
	resultCount := min(3, len(scored))
	result := make([]ProviderInfo, resultCount)
	for i := 0; i < resultCount; i++ {
		result[i] = scored[i].provider
	}

	return result
}

func (s *Service) generateHMAC(_ context.Context, consumerID uuid.UUID, req *OpenAIRequest) (string, error) {
	// Get HMAC secret from config
	cfg, err := config.LoadConfig("")
	if err != nil {
		return "", fmt.Errorf("failed to load config: %w", err)
	}

	// Create unique request identifier combining:
	// - Consumer ID
	// - Timestamp (nanoseconds for uniqueness)
	// - Model name
	// - First few characters of the first message (if available)
	uniqueData := struct {
		ConsumerID uuid.UUID `json:"consumer_id"`
		Timestamp  int64     `json:"timestamp"`
		Model      string    `json:"model"`
		MessageID  string    `json:"message_id,omitempty"`
	}{
		ConsumerID: consumerID,
		Timestamp:  time.Now().UnixNano(),
		Model:      req.Model,
	}

	// Add first message hash if available
	if len(req.Messages) > 0 {
		// Take first 32 chars of the first message as an identifier
		content := req.Messages[0].Content
		if len(content) > 32 {
			content = content[:32]
		}
		uniqueData.MessageID = content
	}

	// Create HMAC generator with secret
	hmacGen := common.NewHMACGenerator(cfg.AuthHMACSecret)

	// Generate HMAC using the structured data
	hmac, err := hmacGen.GenerateWithData(uniqueData)
	if err != nil {
		return "", fmt.Errorf("failed to generate HMAC: %w", err)
	}

	s.logger.Info("Generated HMAC for consumer %s with model %s", consumerID, req.Model)
	return hmac, nil
}

// createTransaction creates a new transaction record
func (s *Service) createTransaction(ctx context.Context, consumerID uuid.UUID, provider ProviderInfo, model, hmac string) (*TransactionRecord, error) {
	tx := &TransactionRecord{
		ID:                uuid.New(),
		ConsumerID:        consumerID,
		ProviderID:        provider.ProviderID,
		HMAC:              hmac,
		ModelName:         model,
		InputPriceTokens:  provider.InputPriceTokens,
		OutputPriceTokens: provider.OutputPriceTokens,
		Status:            "pending",
	}

	_, err := s.db.ExecContext(ctx,
		`INSERT INTO transactions (
			id, consumer_id, provider_id, hmac, model_name,
			input_price_tokens, output_price_tokens, status,
			created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW(), NOW())`,
		tx.ID, tx.ConsumerID, tx.ProviderID, tx.HMAC, tx.ModelName,
		tx.InputPriceTokens, tx.OutputPriceTokens, tx.Status,
	)
	if err != nil {
		return nil, fmt.Errorf("error creating transaction: %w", err)
	}

	return tx, nil
}

// placeHoldingDeposit places a holding deposit for a consumer
func (s *Service) placeHoldingDeposit(ctx context.Context) error {
	// Make internal request to auth service using stored userID
	resp, err := common.MakeInternalRequest(
		ctx,
		"POST",
		common.AuthService,
		"/api/auth/hold",
		HoldDepositRequest{
			UserID: s.userID,
			Amount: 1.0, // $1 holding deposit
		},
	)
	if err != nil {
		return fmt.Errorf("error placing holding deposit: %w", err)
	}

	if !resp["success"].(bool) {
		return fmt.Errorf("failed to place holding deposit")
	}

	return nil
}

// releaseHoldingDeposit releases a holding deposit for a consumer
func (s *Service) releaseHoldingDeposit(ctx context.Context) error {
	// Make internal request to auth service using stored userID
	resp, err := common.MakeInternalRequest(
		ctx,
		"POST",
		common.AuthService,
		"/api/auth/release",
		ReleaseHoldRequest{
			UserID: s.userID,
			Amount: 1.0, // $1 holding deposit
		},
	)
	if err != nil {
		return fmt.Errorf("error releasing holding deposit: %w", err)
	}

	if !resp["success"].(bool) {
		return fmt.Errorf("failed to release holding deposit")
	}

	return nil
}

func (s *Service) sendRequestToProvider(ctx context.Context, providers []ProviderInfo, req *OpenAIRequest, hmac string) (interface{}, *ProviderInfo, error) {
	var lastErr error

	// Get original request path from context
	originalPath, ok := ctx.Value("original_path").(string)
	if !ok || originalPath == "" {
		return nil, nil, common.ErrBadRequest(fmt.Errorf("missing request path"))
	}

	for i, provider := range providers {
		providerStartTime := time.Now()
		s.logger.Info("Attempting request with provider %d/%d (ID: %s)", i+1, len(providers), provider.ProviderID)

		// Construct full provider URL with the original path
		providerFullURL := fmt.Sprintf("%s%s", provider.URL, originalPath)

		// Prepare request body
		prepStartTime := time.Now()

		// Convert the OpenAIRequest to a map to ensure all fields are included
		reqMap := make(map[string]interface{})
		reqBytes, err := json.Marshal(req)
		if err != nil {
			s.logger.Error("Failed to marshal request: %v", err)
			continue
		}
		if err := json.Unmarshal(reqBytes, &reqMap); err != nil {
			s.logger.Error("Failed to unmarshal request to map: %v", err)
			continue
		}

		// Log the request map to verify all fields are included
		s.logger.Info("Request map: %+v", reqMap)

		providerReq := map[string]interface{}{
			"provider_id":  provider.ProviderID,
			"hmac":         hmac,
			"provider_url": providerFullURL,
			"model_name":   req.Model,
			"request_data": reqMap,
		}
		s.logger.Info("Request preparation took: %dms", time.Since(prepStartTime).Milliseconds())

		// Send request to provider communication service
		commStartTime := time.Now()
		response, err := common.MakeInternalRequest(
			ctx,
			"POST",
			common.ProviderCommunicationService,
			"/api/provider-comms/send_requests",
			providerReq,
		)
		commTime := time.Since(commStartTime).Milliseconds()
		s.logger.Info("Provider communication service call took: %dms", commTime)

		if err != nil {
			lastErr = err
			s.logger.Error("Provider %s failed after %dms: %v", provider.ProviderID, commTime, err)

			// Log that we're trying the next provider if there are more
			if i < len(providers)-1 {
				s.logger.Info("Trying next provider (%d/%d remaining)", len(providers)-i-1, len(providers))
			}
			continue // Try next provider
		}

		// Check if the response indicates success
		success, ok := response["success"].(bool)
		if !ok || !success {
			errMsg := "unknown error"
			if errStr, ok := response["error"].(string); ok {
				errMsg = errStr
			}
			lastErr = fmt.Errorf("provider request failed: %s", errMsg)
			s.logger.Error("Provider %s request failed after %dms: %s", provider.ProviderID, commTime, errMsg)

			// Log that we're trying the next provider if there are more
			if i < len(providers)-1 {
				s.logger.Info("Trying next provider (%d/%d remaining)", len(providers)-i-1, len(providers))
			}
			continue // Try next provider
		}

		// If we get here, the request was successful
		totalProviderTime := time.Since(providerStartTime).Milliseconds()
		s.logger.Info("Request successful with provider %s (total time: %dms, comm time: %dms)",
			provider.ProviderID, totalProviderTime, commTime)
		return response, &provider, nil
	}

	// If we get here, all providers failed
	s.logger.Error("All %d providers failed. Last error: %v", len(providers), lastErr)
	return nil, nil, fmt.Errorf("all providers failed. Last error: %v", lastErr)
}

// finalizeTransaction updates the transaction with completion details
func (s *Service) finalizeTransaction(ctx context.Context, tx *TransactionRecord, response interface{}, latency int64, provider *ProviderInfo) error {
	// Extract response data from interface
	responseMap, ok := response.(map[string]interface{})
	if !ok {
		s.logger.Error("DEBUG: Response is not a map: %T", response)
		return fmt.Errorf("invalid response format")
	}

	// Extract usage information from response_data
	responseData, ok := responseMap["response_data"].(map[string]interface{})
	if !ok {
		s.logger.Error("DEBUG: response_data is not a map: %T", responseMap["response_data"])
		return fmt.Errorf("invalid response_data format")
	}

	// Extract usage information
	usage, ok := responseData["usage"].(map[string]interface{})
	if !ok {
		s.logger.Error("DEBUG: usage is not a map: %T", responseData["usage"])
		return fmt.Errorf("missing usage information in response")
	}

	// Extract token counts
	totalInputTokens := int(usage["prompt_tokens"].(float64))
	totalOutputTokens := int(usage["completion_tokens"].(float64))

	// Update transaction with completion details and the successful provider's information
	query := `
		UPDATE transactions 
		SET total_input_tokens = $1,
			total_output_tokens = $2,
			latency = $3,
			status = 'payment',
			provider_id = $4,           -- Update to successful provider
			input_price_tokens = $5,    -- Update to successful provider's pricing
			output_price_tokens = $6    -- Update to successful provider's pricing
		WHERE id = $7
		RETURNING id`

	var transactionID uuid.UUID
	err := s.db.QueryRowContext(ctx, query,
		totalInputTokens,
		totalOutputTokens,
		latency,
		provider.ProviderID,        // Use successful provider's ID
		provider.InputPriceTokens,  // Use successful provider's pricing
		provider.OutputPriceTokens, // Use successful provider's pricing
		tx.ID,
	).Scan(&transactionID)

	if err != nil {
		s.logger.Error("Failed to update transaction with successful provider info: %v", err)
		return fmt.Errorf("failed to update transaction: %w", err)
	}

	s.logger.Info("Updated transaction %s with successful provider %s", tx.ID, provider.ProviderID)

	// Create payment message with the successful provider's information
	paymentMsg := PaymentMessage{
		ConsumerID:        tx.ConsumerID,
		ProviderID:        provider.ProviderID,
		HMAC:              tx.HMAC,
		ModelName:         tx.ModelName,
		TotalInputTokens:  totalInputTokens,
		TotalOutputTokens: totalOutputTokens,
		InputPriceTokens:  provider.InputPriceTokens,  // Use successful provider's pricing
		OutputPriceTokens: provider.OutputPriceTokens, // Use successful provider's pricing
		Latency:           latency,
	}

	// Convert to JSON
	msgBytes, err := json.Marshal(paymentMsg)
	if err != nil {
		return fmt.Errorf("failed to marshal payment message: %w", err)
	}

	// Publish to RabbitMQ
	err = s.rmq.Publish(
		"transactions_exchange", // exchange
		"transactions",          // routing key
		msgBytes,
	)
	if err != nil {
		return fmt.Errorf("failed to publish payment message: %w", err)
	}

	s.logger.Info("Published payment message for transaction %s with successful provider %s", tx.ID, provider.ProviderID)
	return nil
}

// cancelTransaction updates a transaction's status to 'canceled'
func (s *Service) cancelTransaction(ctx context.Context, transactionID uuid.UUID) error {
	query := `
		UPDATE transactions 
		SET status = 'canceled',
		    updated_at = NOW()
		WHERE id = $1
		RETURNING id`

	var id uuid.UUID
	err := s.db.QueryRowContext(ctx, query, transactionID).Scan(&id)
	if err != nil {
		return fmt.Errorf("error canceling transaction: %w", err)
	}

	s.logger.Info("Transaction %s status updated to 'canceled'", id)
	return nil
}

// min returns the smaller of x or y
func min(x, y int) int {
	if x < y {
		return x
	}
	return y
}
