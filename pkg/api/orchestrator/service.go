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

type Service struct {
	db     *db.DB
	logger *common.Logger
	rmq    *rabbitmq.Client
}

func NewService(db *db.DB, logger *common.Logger, rmq *rabbitmq.Client) *Service {
	return &Service{
		db:     db,
		logger: logger,
		rmq:    rmq,
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
	selectedProviders := s.selectBestProviders(providers)
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

func (s *Service) getConsumerSettings(ctx context.Context, consumerID uuid.UUID, model string) (*ConsumerSettings, error) {
	s.logger.Info("Getting consumer settings for user %s and model %s", consumerID, model)

	var settings ConsumerSettings

	// First try to get model-specific settings
	err := s.db.QueryRowContext(ctx, `
		SELECT max_input_price_tokens, max_output_price_tokens 
		FROM consumer_models 
		WHERE consumer_id = $1 AND model_name = $2`,
		consumerID, model,
	).Scan(&settings.MaxInputPriceTokens, &settings.MaxOutputPriceTokens)

	if err == nil {
		s.logger.Info("Found model-specific price settings: input=%v, output=%v",
			settings.MaxInputPriceTokens, settings.MaxOutputPriceTokens)
		return &settings, nil
	}

	if err != sql.ErrNoRows {
		return nil, fmt.Errorf("error querying model-specific settings: %w", err)
	}

	// If no model-specific settings found, get global settings
	s.logger.Info("No model-specific settings found, falling back to global settings")
	err = s.db.QueryRowContext(ctx, `
		SELECT max_input_price_tokens, max_output_price_tokens 
		FROM consumers 
		WHERE user_id = $1`,
		consumerID,
	).Scan(&settings.MaxInputPriceTokens, &settings.MaxOutputPriceTokens)

	if err == sql.ErrNoRows {
		s.logger.Info("No global settings found, using system defaults")
		// Use system defaults if no settings found
		settings.MaxInputPriceTokens = 1.0 // $1.00 per million tokens default
		settings.MaxOutputPriceTokens = 1.0
		return &settings, nil
	}

	if err != nil {
		return nil, fmt.Errorf("error querying global consumer settings: %w", err)
	}

	s.logger.Info("Found global price settings: input=%v, output=%v",
		settings.MaxInputPriceTokens, settings.MaxOutputPriceTokens)
	return &settings, nil
}

func (s *Service) getHealthyProviders(ctx context.Context, model string, settings *ConsumerSettings) ([]ProviderInfo, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT ps.provider_id, ps.api_url, pm.input_price_tokens, pm.output_price_tokens, ps.tier, 
			COALESCE((
				SELECT latency_ms 
				FROM provider_health_history 
				WHERE provider_id = ps.provider_id 
				ORDER BY health_check_time DESC 
				LIMIT 1
			), 0) as latency, ps.health_status
		FROM provider_status ps
		JOIN provider_models pm ON ps.provider_id = pm.provider_id
		WHERE pm.model_name = $1 
		AND ps.health_status IN ('green', 'orange')
		AND ps.is_available = true
		AND NOT ps.paused
		AND pm.is_active = true
		AND pm.input_price_tokens <= $2 
		AND pm.output_price_tokens <= $3`,
		model, settings.MaxInputPriceTokens, settings.MaxOutputPriceTokens,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get healthy providers: %w", err)
	}
	defer rows.Close()

	var providers []ProviderInfo
	for rows.Next() {
		var p ProviderInfo
		if err := rows.Scan(&p.ProviderID, &p.URL, &p.InputPriceTokens, &p.OutputPriceTokens, &p.Tier, &p.Latency, &p.HealthStatus); err != nil {
			return nil, fmt.Errorf("failed to scan provider info: %w", err)
		}
		providers = append(providers, p)
	}

	return providers, nil
}

func (s *Service) selectBestProviders(providers []ProviderInfo) []ProviderInfo {
	// Create a slice to store provider scores
	type scoredProvider struct {
		provider ProviderInfo
		score    float64
	}

	scored := make([]scoredProvider, 0, len(providers))

	// Calculate scores for all providers
	for _, p := range providers {
		// Lower price and latency is better
		priceScore := 1.0 / (p.InputPriceTokens + p.OutputPriceTokens)
		latencyScore := 1.0 / float64(p.Latency)

		// Weight price more heavily than latency (70/30 split)
		score := (priceScore * 0.7) + (latencyScore * 0.3)

		scored = append(scored, scoredProvider{p, score})
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

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO transactions (
			id, consumer_id, provider_id, hmac, model_name, 
			input_price_tokens, output_price_tokens, status
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		tx.ID, tx.ConsumerID, tx.ProviderID, tx.HMAC, tx.ModelName,
		tx.InputPriceTokens, tx.OutputPriceTokens, tx.Status,
	)

	if err != nil {
		return nil, err
	}

	return tx, nil
}

func (s *Service) placeHoldingDeposit(ctx context.Context, consumerID uuid.UUID) error {
	s.logger.Info("Starting holding deposit process for consumer %s", consumerID)

	// Create request body
	reqBody := map[string]interface{}{
		"user_id": consumerID.String(),
		"amount":  1.0, // $1 holding deposit
	}

	s.logger.Info("Making auth service request with body: %+v", reqBody)

	// Make internal request to auth service
	response, err := common.MakeInternalRequest(
		ctx,
		"POST",
		common.AuthService,
		"/api/auth/hold",
		reqBody,
	)

	if err != nil {
		s.logger.Error("Failed to place holding deposit: %v", err)
		if response != nil {
			s.logger.Error("Response data: %+v", response)
		}
		return fmt.Errorf("failed to place holding deposit: %w", err)
	}

	s.logger.Info("Auth service response: %+v", response)
	s.logger.Info("Successfully placed holding deposit for consumer %s", consumerID)
	return nil
}

func (s *Service) releaseHoldingDeposit(ctx context.Context, consumerID uuid.UUID) error {
	// Create request body
	reqBody := map[string]interface{}{
		"user_id": consumerID.String(),
		"amount":  1.0, // $1 holding deposit
	}

	s.logger.Info("Releasing holding deposit for consumer %s", consumerID)

	// Make internal request to auth service
	_, err := common.MakeInternalRequest(
		ctx,
		"POST",
		common.AuthService,
		"/api/auth/release",
		reqBody,
	)
	if err != nil {
		return fmt.Errorf("failed to release holding deposit: %w", err)
	}

	s.logger.Info("Successfully released holding deposit for consumer %s", consumerID)
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

func (s *Service) finalizeTransaction(ctx context.Context, tx *TransactionRecord, response interface{}, latency int64) error {
	// Extract token counts from response
	responseMap, ok := response.(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid response format")
	}

	usage, ok := responseMap["usage"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("missing usage information in response")
	}

	promptTokens, ok := usage["prompt_tokens"].(float64)
	if !ok {
		return fmt.Errorf("invalid prompt tokens format")
	}

	completionTokens, ok := usage["completion_tokens"].(float64)
	if !ok {
		return fmt.Errorf("invalid completion tokens format")
	}

	// Update transaction with token counts and status
	_, err := s.db.ExecContext(ctx, `
		UPDATE transactions 
		SET total_input_tokens = $1,
			total_output_tokens = $2,
			latency = $3,
			status = 'payment'
		WHERE id = $4`,
		int(promptTokens),
		int(completionTokens),
		latency,
		tx.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update transaction: %w", err)
	}

	// Create payment message
	paymentMsg := PaymentMessage{
		ConsumerID:        tx.ConsumerID,
		ProviderID:        tx.ProviderID,
		HMAC:              tx.HMAC,
		ModelName:         tx.ModelName,
		TotalInputTokens:  int(promptTokens),
		TotalOutputTokens: int(completionTokens),
		InputPriceTokens:  tx.InputPriceTokens,
		OutputPriceTokens: tx.OutputPriceTokens,
		Latency:           latency,
	}

	// Convert message to JSON
	msgBytes, err := json.Marshal(paymentMsg)
	if err != nil {
		return fmt.Errorf("failed to marshal payment message: %w", err)
	}

	// Publish to RabbitMQ
	err = s.rmq.Publish(
		"transactions_exchange",
		"transactions",
		msgBytes,
	)
	if err != nil {
		return fmt.Errorf("failed to publish payment message: %w", err)
	}

	s.logger.Info("Successfully finalized transaction %s with %d input tokens and %d output tokens",
		tx.ID, int(promptTokens), int(completionTokens))
	return nil
}

// min returns the smaller of x or y
func min(x, y int) int {
	if x < y {
		return x
	}
	return y
}
