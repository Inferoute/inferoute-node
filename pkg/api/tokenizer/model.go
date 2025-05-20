package tokenizer

// TokenizeRequest represents the request body for tokenization
type TokenizeRequest struct {
	InputText  string `json:"input_text"`
	OutputText string `json:"output_text"`
}

// TokenizeResponse represents the response body for tokenization
type TokenizeResponse struct {
	InputTokenCount  int    `json:"input_token_count"`
	OutputTokenCount int    `json:"output_token_count"`
	TotalTokenCount  int    `json:"total_token_count"`
	Method           string `json:"method"`
}
