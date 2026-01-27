package apikey

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

// GenerateKey creates a new API key with the given prefix.
// Format: {prefix}_{24_random_hex_chars}
// Example: sk_test_RANDOM_HEX_STRING
func GenerateKey(prefix, secret string) (key string, hash string, err error) {
	bytes := make([]byte, 24)
	if _, err := rand.Read(bytes); err != nil {
		return "", "", err
	}
	keyPart := hex.EncodeToString(bytes)
	fullKey := fmt.Sprintf("%s_%s", prefix, keyPart)
	return fullKey, HashKey(fullKey, secret), nil
}

// HashKey hashes the full API key for storage using HMAC-SHA256.
func HashKey(key, secret string) string {
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(key))
	return hex.EncodeToString(h.Sum(nil))
}

// ValidateKeyFormat checks if the key matches the expected format prefix.
func ValidateKeyFormat(key, expectedPrefix string) bool {
	return strings.HasPrefix(key, expectedPrefix)
}
