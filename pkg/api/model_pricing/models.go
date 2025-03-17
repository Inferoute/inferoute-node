package model_pricing

import "time"

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

// ModelPricingData represents candlestick chart data for database operations
type ModelPricingData struct {
	ID           int64     `json:"id"`
	ModelName    string    `json:"model_name"`
	Timestamp    time.Time `json:"timestamp"`
	InputOpen    float64   `json:"input_open"`
	InputHigh    float64   `json:"input_high"`
	InputLow     float64   `json:"input_low"`
	InputClose   float64   `json:"input_close"`
	OutputOpen   float64   `json:"output_open"`
	OutputHigh   float64   `json:"output_high"`
	OutputLow    float64   `json:"output_low"`
	OutputClose  float64   `json:"output_close"`
	VolumeInput  int       `json:"volume_input"`
	VolumeOutput int       `json:"volume_output"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// ModelPricingDataResponse represents candlestick chart data for API responses
type ModelPricingDataResponse struct {
	ModelName    string  `json:"model_name"`
	Timestamp    string  `json:"timestamp"`
	InputOpen    float64 `json:"input_open"`
	InputHigh    float64 `json:"input_high"`
	InputLow     float64 `json:"input_low"`
	InputClose   float64 `json:"input_close"`
	OutputOpen   float64 `json:"output_open"`
	OutputHigh   float64 `json:"output_high"`
	OutputLow    float64 `json:"output_low"`
	OutputClose  float64 `json:"output_close"`
	VolumeInput  int     `json:"volume_input"`
	VolumeOutput int     `json:"volume_output"`
}

// GetPricingDataRequest represents the request for getting model pricing data
type GetPricingDataRequest struct {
	ModelName string `json:"model_name"`
	Limit     int    `json:"limit"`
}

// GetPricingDataResponse represents the response for getting model pricing data
type GetPricingDataResponse struct {
	Data []ModelPricingDataResponse `json:"data"`
}
