package tokenizer

import (
	"context"
	"fmt"

	"github.com/pkoukk/tiktoken-go"
	"github.com/sentnl/inferoute-node/pkg/common"
)

// Service handles tokenization requests
type Service struct {
	logger *common.Logger
	Enc    *tiktoken.Tiktoken
}

// NewService creates a new tokenizer service
func NewService(logger *common.Logger) (*Service, error) {
	// Initialize GPT-2 tokenizer
	enc, err := tiktoken.GetEncoding("cl100k_base") // Using OpenAI's encoding
	if err != nil {
		return nil, fmt.Errorf("failed to initialize tokenizer: %w", err)
	}

	return &Service{
		logger: logger,
		Enc:    enc,
	}, nil
}

// Tokenize handles token counting for input and output text
func (s *Service) Tokenize(ctx context.Context, req *TokenizeRequest) (*TokenizeResponse, error) {
	// Count input tokens
	inputTokens := s.Enc.Encode(req.InputText, nil, nil)
	inputCount := len(inputTokens)

	// Count output tokens
	outputTokens := s.Enc.Encode(req.OutputText, nil, nil)
	outputCount := len(outputTokens)

	// Create response
	response := &TokenizeResponse{
		InputTokenCount:  inputCount,
		OutputTokenCount: outputCount,
		TotalTokenCount:  inputCount + outputCount,
		Method:           "bpe",
	}

	s.logger.Info("Tokenized text - Input: %d tokens, Output: %d tokens, Total: %d tokens",
		inputCount, outputCount, response.TotalTokenCount)

	return response, nil
}
