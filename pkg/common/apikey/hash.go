package apikey

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

type HashedKey struct {
	BcryptHash string
	LookupKey  string // First 8 chars of SHA256 for fast comparison
}

// HashAPIKey hashes an API key using bcrypt and creates a fast lookup key
func HashAPIKey(key string) HashedKey {
	// Generate bcrypt hash
	hash, err := bcrypt.GenerateFromPassword([]byte(key), bcrypt.DefaultCost)
	if err != nil {
		panic(err) // This should never happen with bcrypt
	}

	// Generate fast lookup key (first 8 chars of SHA256)
	hasher := sha256.New()
	hasher.Write([]byte(key))
	lookupKey := hex.EncodeToString(hasher.Sum(nil))[:8]

	return HashedKey{
		BcryptHash: string(hash),
		LookupKey:  lookupKey,
	}
}

// CompareAPIKey compares a plain text API key with a hashed one
func CompareAPIKey(plaintext, hashedKey string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hashedKey), []byte(plaintext))
	return err == nil
}

// GenerateLookupKey generates a fast lookup key for an API key
func GenerateLookupKey(key string) string {
	hasher := sha256.New()
	hasher.Write([]byte(key))
	return hex.EncodeToString(hasher.Sum(nil))[:8]
}

// GenerateAPIKey generates a new API key with the format "sk-..." followed by 32 random characters
func GenerateAPIKey() string {
	b := make([]byte, 16) // 16 bytes = 32 hex characters
	_, err := rand.Read(b)
	if err != nil {
		panic(err) // This should never happen with crypto/rand
	}
	return fmt.Sprintf("sk-%x", b)
}
