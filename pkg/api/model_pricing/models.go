package model_pricing

// ModelPricing represents the pricing information for a specific model
type ModelPricing struct {
	ModelName      string  `json:"model_name"`
	AvgInputPrice  float64 `json:"avg_input_price"`
	AvgOutputPrice float64 `json:"avg_output_price"`
	SampleSize     int     `json:"sample_size"`
}

// GetPricesRequest represents the request body for getting model prices
type GetPricesRequest struct {
	Models []string `json:"models"`
}

// GetPricesResponse represents the response body for model prices
type GetPricesResponse struct {
	ModelPrices []ModelPricing `json:"model_prices"`
}

// UpdateCostsResponse represents the response for updating model costs
type UpdateCostsResponse struct {
	Status string `json:"status"`
}
