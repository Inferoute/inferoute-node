package common

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// HMACGenerator handles HMAC generation
type HMACGenerator struct {
	secret []byte
}

// NewHMACGenerator creates a new HMAC generator with the given secret
func NewHMACGenerator(secret string) *HMACGenerator {
	return &HMACGenerator{
		secret: []byte(secret),
	}
}

// Generate creates a new HMAC for a consumer
func (g *HMACGenerator) Generate(consumerID uuid.UUID) (string, error) {
	// Generate random nonce
	nonce := make([]byte, 16)
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("error generating nonce: %v", err)
	}

	// Generate HMAC using consumer ID and nonce
	h := hmac.New(sha256.New, g.secret)
	h.Write([]byte(consumerID.String()))
	h.Write([]byte(time.Now().Format(time.RFC3339Nano)))
	h.Write(nonce)

	return "sk-" + base64.URLEncoding.EncodeToString(h.Sum(nil)), nil
}

// GenerateWithData generates an HMAC using structured data
func (g *HMACGenerator) GenerateWithData(data interface{}) (string, error) {
	// Convert data to JSON bytes
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return "", fmt.Errorf("failed to marshal data: %w", err)
	}

	// Create new HMAC hasher
	hasher := hmac.New(sha256.New, []byte(g.secret))

	// Write data to hasher
	hasher.Write(jsonBytes)

	// Get the result and encode as base64
	hashBytes := hasher.Sum(nil)
	return base64.URLEncoding.EncodeToString(hashBytes), nil
}
