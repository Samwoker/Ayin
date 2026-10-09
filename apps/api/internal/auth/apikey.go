package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

// GeneratedKey holds the raw token (shown once) and hash to store in DB.
type GeneratedKey struct {
	RawKey    string `json:"raw_key"`
	KeyPrefix string `json:"key_prefix"`
	KeyHash   string `json:"key_hash"`
}

// GenerateAPIKey generates a new API key with a readable prefix and secure entropy.
func GenerateAPIKey(prefix string) (*GeneratedKey, error) {
	cleanPrefix := strings.ToLower(strings.TrimSpace(prefix))
	if cleanPrefix == "" {
		cleanPrefix = "org"
	}

	randomBytes := make([]byte, 24)
	if _, err := rand.Read(randomBytes); err != nil {
		return nil, fmt.Errorf("failed to generate random bytes: %w", err)
	}

	rawEntropy := hex.EncodeToString(randomBytes)
	rawKey := fmt.Sprintf("tr_live_%s_%s", cleanPrefix, rawEntropy)
	keyPrefix := fmt.Sprintf("tr_live_%s", cleanPrefix)
	keyHash := HashAPIKey(rawKey)

	return &GeneratedKey{
		RawKey:    rawKey,
		KeyPrefix: keyPrefix,
		KeyHash:   keyHash,
	}, nil
}

// HashAPIKey computes the SHA-256 hash of an API key for safe database storage.
func HashAPIKey(rawKey string) string {
	hash := sha256.Sum256([]byte(rawKey))
	return hex.EncodeToString(hash[:])
}
