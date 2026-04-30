package auth

import (
	"encoding/base64"
	"regexp"
	"strings"
	"testing"
)

func TestNewSessionTokenReturnsOpaqueRandomToken(t *testing.T) {
	first, err := NewSessionToken()
	if err != nil {
		t.Fatalf("NewSessionToken(first) error = %v", err)
	}
	second, err := NewSessionToken()
	if err != nil {
		t.Fatalf("NewSessionToken(second) error = %v", err)
	}

	if first == second {
		t.Fatal("NewSessionToken() returned identical tokens, want random opaque tokens")
	}

	decoded, err := base64.RawURLEncoding.DecodeString(first)
	if err != nil {
		t.Fatalf("NewSessionToken() = %q, not raw URL base64: %v", first, err)
	}
	if len(decoded) < 32 {
		t.Fatalf("decoded token length = %d, want at least 32 bytes", len(decoded))
	}

	base64URLSafe := regexp.MustCompile(`^[A-Za-z0-9_-]+$`)
	if !base64URLSafe.MatchString(first) || !base64URLSafe.MatchString(second) {
		t.Fatalf("tokens are not raw URL/base64 safe: %q %q", first, second)
	}
}

func TestHashSessionTokenIsStableAndDoesNotExposeToken(t *testing.T) {
	token, err := NewSessionToken()
	if err != nil {
		t.Fatalf("NewSessionToken() error = %v", err)
	}

	first := HashSessionToken(token)
	second := HashSessionToken(token)

	if first != second {
		t.Fatalf("HashSessionToken() is not deterministic: %q != %q", first, second)
	}
	if first == token {
		t.Fatal("HashSessionToken() equals raw token, want stored hash")
	}
	if strings.Contains(first, token) {
		t.Fatal("HashSessionToken() contains raw token, want opaque hash")
	}
}
