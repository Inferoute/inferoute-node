package apikey

import (
	"crypto/rand"
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

// HashAPIKey hashes an API key using bcrypt
func HashAPIKey(key string) string {
	hash, err := bcrypt.GenerateFromPassword([]byte(key), bcrypt.DefaultCost)
	if err != nil {
		panic(err) // This should never happen with bcrypt
	}
	return string(hash)
}

// CompareAPIKey compares a plain text API key with a hashed one
func CompareAPIKey(plaintext, hashed string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hashed), []byte(plaintext))
	return err == nil
}

// GenerateAPIKey generates a new API key with the format "sk-..." followed by 32 random characters
func GenerateAPIKey() string {
	b := make([]byte, 32) // 32 bytes = 64 hex characters
	_, err := rand.Read(b)
	if err != nil {
		panic(err) // This should never happen with crypto/rand
	}
	return fmt.Sprintf("sk-%x", b)
}
