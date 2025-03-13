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

// ModelPricingData represents candlestick chart data for a model's pricing
type ModelPricingData struct {
	ModelName   string  `json:"model_name"`
	Timestamp   string  `json:"timestamp"`
	InputOpen   float64 `json:"input_open"`
	InputHigh   float64 `json:"input_high"`
	InputLow    float64 `json:"input_low"`
	InputClose  float64 `json:"input_close"`
	OutputOpen  float64 `json:"output_open"`
	OutputHigh  float64 `json:"output_high"`
	OutputLow   float64 `json:"output_low"`
	OutputClose float64 `json:"output_close"`
	Volume      int     `json:"volume"`
}

// UpdatePricingDataResponse represents the response for updating model pricing data
type UpdatePricingDataResponse struct {
	Status string `json:"status"`
	Count  int    `json:"count"`
}

// GetPricingDataRequest represents the request for getting model pricing data
type GetPricingDataRequest struct {
	ModelName string `json:"model_name"`
	Limit     int    `json:"limit"`
}

// GetPricingDataResponse represents the response for getting model pricing data
type GetPricingDataResponse struct {
	Data []ModelPricingData `json:"data"`
}
