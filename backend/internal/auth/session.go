package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
)

// SessionTokenBytes is the number of random bytes in a new session token.
const SessionTokenBytes = 32

// NewSessionToken returns a new opaque token suitable for use in a cookie.
func NewSessionToken() (string, error) {
	token := make([]byte, SessionTokenBytes)
	if _, err := rand.Read(token); err != nil {
		return "", fmt.Errorf("generate session token: %w", err)
	}

	return base64.RawURLEncoding.EncodeToString(token), nil
}

// HashSessionToken returns the SHA-256 hex digest used for session token storage.
func HashSessionToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
