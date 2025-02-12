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
	// 1. Validate API key
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
	if err != nil {
		s.logger.Error("Failed to validate API key: %v", err)
		return nil, fmt.Errorf("failed to validate API key: %w", err)
	}

	// Check if API key is valid
	if valid, ok := authResp["valid"].(bool); !ok || !valid {
		s.logger.Error("API key validation failed")
		return nil, common.ErrUnauthorized(fmt.Errorf("invalid API key"))
	}

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

	// 2. Get consumer settings (global and model-specific)
	settings, err := s.getConsumerSettings(ctx, consumerID, req.Model)
	if err != nil {
		return nil, fmt.Errorf("failed to get consumer settings: %w", err)
	}

	// 3. Get healthy providers within price constraints
	providers, err := s.getHealthyProviders(ctx, req.Model, settings)
	if err != nil {
		return nil, fmt.Errorf("failed to get healthy providers: %w", err)
	}

	if len(providers) == 0 {
		return nil, fmt.Errorf("no healthy providers available for model %s within price constraints", req.Model)
	}

	// 4. Select best providers based on price and latency
	selectedProviders := s.selectBestProviders(providers, req.Sort)
	if len(selectedProviders) == 0 {
		return nil, fmt.Errorf("no suitable providers found for model %s", req.Model)
	}

	// Use first provider for transaction record
	selectedProvider := selectedProviders[0]

	// 5. Generate HMAC
	hmac, err := s.generateHMAC(ctx, consumerID, req)
	if err != nil {
		return nil, fmt.Errorf("failed to generate HMAC: %w", err)
	}

	// 6. Create transaction record
	tx, err := s.createTransaction(ctx, consumerID, selectedProvider, req.Model, hmac)
	if err != nil {
		return nil, fmt.Errorf("failed to create transaction: %w", err)
	}

	// 7. Place holding deposit
	if err := s.placeHoldingDeposit(ctx, consumerID); err != nil {
		return nil, fmt.Errorf("failed to place holding deposit: %w", err)
	}

	// 8. Send request to provider
	startTime := time.Now()
	response, err := s.sendRequestToProvider(ctx, providers, req, hmac)
	if err != nil {
		// Release holding deposit on error
		_ = s.releaseHoldingDeposit(ctx, consumerID)
		return nil, fmt.Errorf("failed to send request to provider: %w", err)
	}
	latency := time.Since(startTime).Milliseconds()

	// 9. Release holding deposit
	if err := s.releaseHoldingDeposit(ctx, consumerID); err != nil {
		s.logger.Error("Failed to release holding deposit: %v", err)
	}

	// 10. Update transaction and publish payment message
	if err := s.finalizeTransaction(ctx, tx, response, latency); err != nil {
		s.logger.Error("Failed to finalize transaction: %v", err)
	}

	return response, nil
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

	// Set weights based on sort preference
	var priceWeight, tpsWeight float64
	switch sortBy {
	case "throughput":
		s.logger.Info("GERT Sorting by throughput")
		priceWeight = 0.2 // 20% price
		tpsWeight = 0.8   // 80% throughput
	default: // "cost" or empty
		s.logger.Info("GERT Sorting by cost")
		priceWeight = 0.7 // 70% price
		tpsWeight = 0.3   // 30% throughput
	}

	// Calculate scores for all providers
	for _, p := range providers {
		// Lower price and higher TPS is better
		priceScore := 1.0 / (p.InputPriceTokens + p.OutputPriceTokens)
		tpsScore := p.AverageTPS // Higher TPS is directly better

		// Apply weights based on sort preference
		score := (priceScore * priceWeight) + (tpsScore * tpsWeight)

		scored = append(scored, scoredProvider{p, score})
		s.logger.Info("Provider %s scored %f (price: %f, tps: %f, sort: %s)",
			p.ProviderID, score, p.InputPriceTokens+p.OutputPriceTokens, p.AverageTPS, sortBy)
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
func (s *Service) placeHoldingDeposit(ctx context.Context, consumerID uuid.UUID) error {
	// Make internal request to auth service
	resp, err := common.MakeInternalRequest(
		ctx,
		"POST",
		common.AuthService,
		"/api/auth/hold",
		HoldDepositRequest{
			ConsumerID: consumerID,
			Amount:     1.0, // $1 holding deposit
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
func (s *Service) releaseHoldingDeposit(ctx context.Context, consumerID uuid.UUID) error {
	// Make internal request to auth service
	resp, err := common.MakeInternalRequest(
		ctx,
		"POST",
		common.AuthService,
		"/api/auth/release",
		ReleaseHoldRequest{
			ConsumerID: consumerID,
			Amount:     1.0, // $1 holding deposit
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

func (s *Service) sendRequestToProvider(ctx context.Context, providers []ProviderInfo, req *OpenAIRequest, hmac string) (interface{}, error) {
	var lastErr error

	// Get original request path from context
	originalPath, ok := ctx.Value("original_path").(string)
	if !ok || originalPath == "" {
		return nil, common.ErrBadRequest(fmt.Errorf("missing request path"))
	}

	for i, provider := range providers {
		s.logger.Info("Attempting request with provider %d/%d (ID: %s)", i+1, len(providers), provider.ProviderID)

		// Construct full provider URL with the original path
		providerFullURL := fmt.Sprintf("%s%s", provider.URL, originalPath)

		// Prepare request body
		providerReq := map[string]interface{}{
			"provider_id":  provider.ProviderID,
			"hmac":         hmac,
			"provider_url": providerFullURL,
			"model_name":   req.Model,
			"request_data": req,
		}

		// Send request to provider communication service
		response, err := common.MakeInternalRequest(
			ctx,
			"POST",
			common.ProviderCommunicationService,
			"/api/provider-comms/send_requests",
			providerReq,
		)

		if err != nil {
			lastErr = err
			s.logger.Error("Provider %s failed: %v", provider.ProviderID, err)
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
			s.logger.Error("Provider %s request failed: %s", provider.ProviderID, errMsg)
			continue // Try next provider
		}

		// If we get here, the request was successful
		s.logger.Info("Request successful with provider %s", provider.ProviderID)
		return response, nil
	}

	// If we get here, all providers failed
	return nil, fmt.Errorf("all providers failed. Last error: %v", lastErr)
}

// finalizeTransaction updates the transaction with completion details
func (s *Service) finalizeTransaction(ctx context.Context, tx *TransactionRecord, response interface{}, latency int64) error {

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

	// Update transaction with completion details
	query := `
		UPDATE transactions 
		SET total_input_tokens = $1,
			total_output_tokens = $2,
			latency = $3,
			status = 'payment'
		WHERE id = $4
		RETURNING id`

	var transactionID uuid.UUID
	err := s.db.QueryRowContext(ctx, query,
		totalInputTokens,
		totalOutputTokens,
		latency,
		tx.ID,
	).Scan(&transactionID)

	if err != nil {
		return fmt.Errorf("failed to update transaction: %w", err)
	}

	// Create payment message
	paymentMsg := PaymentMessage{
		ConsumerID:        tx.ConsumerID,
		ProviderID:        tx.ProviderID,
		HMAC:              tx.HMAC,
		ModelName:         tx.ModelName,
		TotalInputTokens:  totalInputTokens,
		TotalOutputTokens: totalOutputTokens,
		InputPriceTokens:  tx.InputPriceTokens,
		OutputPriceTokens: tx.OutputPriceTokens,
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

	s.logger.Info("Published payment message for transaction %s", tx.ID)
	return nil
}

// min returns the smaller of x or y
func min(x, y int) int {
	if x < y {
		return x
	}
	return y
}
