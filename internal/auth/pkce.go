package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"strings"
)

// PKCECodeChallengeMethod defines the PKCE challenge method.
const (
	PKCEMethodPlain = "plain"
	PKCEMethodS256  = "S256"
)

// GenerateCodeVerifier generates a cryptographically random code verifier
// that meets RFC 7636 requirements (43-128 characters, URL-safe).
func GenerateCodeVerifier() (string, error) {
	b := make([]byte, 32) // 32 bytes = 43 characters when base64url encoded
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// GenerateCodeChallenge generates a code challenge from the verifier.
// Supports "plain" and "S256" methods per RFC 7636.
func GenerateCodeChallenge(verifier string, method string) string {
	switch method {
	case PKCEMethodS256:
		hash := sha256.Sum256([]byte(verifier))
		return base64.RawURLEncoding.EncodeToString(hash[:])
	case PKCEMethodPlain:
		return verifier
	default:
		// Default to S256 for security
		hash := sha256.Sum256([]byte(verifier))
		return base64.RawURLEncoding.EncodeToString(hash[:])
	}
}

// VerifyCodeChallenge verifies the code_verifier against the stored code_challenge.
// Returns true if the verifier is valid.
func VerifyCodeChallenge(verifier, challenge, method string) bool {
	if verifier == "" || challenge == "" {
		return false
	}

	// Normalize method to uppercase
	method = strings.ToUpper(method)

	switch method {
	case PKCEMethodS256:
		expectedChallenge := GenerateCodeChallenge(verifier, PKCEMethodS256)
		return expectedChallenge == challenge
	case PKCEMethodPlain, "PLAIN":
		return verifier == challenge
	default:
		// Unknown method, default to S256 comparison
		expectedChallenge := GenerateCodeChallenge(verifier, PKCEMethodS256)
		return expectedChallenge == challenge
	}
}

// ValidatePKCEMethod checks if the provided method is valid.
func ValidatePKCEMethod(method string) bool {
	method = strings.ToUpper(method)
	return method == PKCEMethodS256 || method == PKCEMethodPlain || method == "PLAIN"
}
