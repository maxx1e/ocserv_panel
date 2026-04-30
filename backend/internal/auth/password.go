package auth

import (
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
)

const (
	// PasswordHashAlgorithm identifies the password hash format used by this package.
	PasswordHashAlgorithm = "pbkdf2-sha256"

	passwordHashIterations = 600000
	passwordSaltBytes      = 16
	passwordKeyBytes       = 32
)

// HashPassword derives and encodes a PBKDF2-HMAC-SHA256 password hash.
func HashPassword(password string) (string, error) {
	if password == "" {
		return "", fmt.Errorf("password must not be empty")
	}

	salt := make([]byte, passwordSaltBytes)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate password salt: %w", err)
	}

	key, err := pbkdf2.Key(sha256.New, password, salt, passwordHashIterations, passwordKeyBytes)
	if err != nil {
		return "", fmt.Errorf("derive password hash: %w", err)
	}

	encodedSalt := base64.RawURLEncoding.EncodeToString(salt)
	encodedKey := base64.RawURLEncoding.EncodeToString(key)

	return fmt.Sprintf("%s$%d$%s$%s", PasswordHashAlgorithm, passwordHashIterations, encodedSalt, encodedKey), nil
}

// VerifyPassword verifies a password against an encoded PBKDF2-HMAC-SHA256 hash.
func VerifyPassword(encoded string, password string) (bool, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 4 {
		return false, fmt.Errorf("password hash has invalid format")
	}

	if parts[0] != PasswordHashAlgorithm {
		return false, fmt.Errorf("password hash has unsupported algorithm")
	}

	iterations, err := strconv.Atoi(parts[1])
	if err != nil {
		return false, fmt.Errorf("password hash has invalid iterations")
	}
	if iterations != passwordHashIterations {
		return false, fmt.Errorf("password hash has unsupported iterations")
	}

	salt, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return false, fmt.Errorf("password hash has invalid salt")
	}
	if len(salt) != passwordSaltBytes {
		return false, fmt.Errorf("password hash has invalid salt length")
	}

	wantKey, err := base64.RawURLEncoding.DecodeString(parts[3])
	if err != nil {
		return false, fmt.Errorf("password hash has invalid key")
	}
	if len(wantKey) != passwordKeyBytes {
		return false, fmt.Errorf("password hash has invalid key length")
	}

	gotKey, err := pbkdf2.Key(sha256.New, password, salt, iterations, len(wantKey))
	if err != nil {
		return false, fmt.Errorf("derive password hash: %w", err)
	}

	return subtle.ConstantTimeCompare(gotKey, wantKey) == 1, nil
}
