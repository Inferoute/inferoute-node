package ai_applications

// ModelResponse represents the response for the /v1/models endpoint
type ModelResponse struct {
	Object string        `json:"object"`
	Data   []ModelDetail `json:"data"`
}

// ModelDetail represents a single model in the response
type ModelDetail struct {
	ID          string      `json:"id"`
	Object      string      `json:"object"`
	Created     int64       `json:"created"`
	OwnedBy     string      `json:"owned_by"`
	Root        string      `json:"root"`
	Parent      interface{} `json:"parent"`
	MaxModelLen int         `json:"max_model_len"`
}
