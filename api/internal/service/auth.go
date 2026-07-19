package service

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
)

// GenerateAPIKey returns a new API key: "zaas_" + 32 random hex chars (128 bits).
func GenerateAPIKey() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic("crypto/rand failed: " + err.Error())
	}
	return "zaas_" + hex.EncodeToString(b)
}

// HashKey returns the SHA-256 hex digest of a key or token.
func HashKey(key string) string {
	h := sha256.Sum256([]byte(key))
	return hex.EncodeToString(h[:])
}

// KeyPrefix returns the first 8 characters of an API key (for display/identification).
func KeyPrefix(key string) string {
	if len(key) < 8 {
		return key
	}
	return key[:8]
}

// GenerateToken returns a random 32-byte hex-encoded token (for verification links).
func GenerateToken() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic("crypto/rand failed: " + err.Error())
	}
	return hex.EncodeToString(b)
}
