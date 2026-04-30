package auth

import (
	"strings"
	"testing"
)

func TestHashPasswordDoesNotExposePlaintextAndVerifies(t *testing.T) {
	passphrase := "correct horse battery staple"

	encoded, err := HashPassword(passphrase)
	if err != nil {
		t.Fatalf("HashPassword() error = %v", err)
	}

	if !strings.HasPrefix(encoded, "pbkdf2-sha256$") {
		t.Fatalf("HashPassword() = %q, want pbkdf2-sha256 prefix", encoded)
	}
	if strings.Contains(encoded, passphrase) {
		t.Fatalf("HashPassword() exposed plaintext passphrase in %q", encoded)
	}

	verified, err := VerifyPassword(encoded, passphrase)
	if err != nil {
		t.Fatalf("VerifyPassword(correct) error = %v", err)
	}
	if !verified {
		t.Fatal("VerifyPassword(correct) = false, want true")
	}

	verified, err = VerifyPassword(encoded, "incorrect passphrase")
	if err != nil {
		t.Fatalf("VerifyPassword(incorrect) error = %v", err)
	}
	if verified {
		t.Fatal("VerifyPassword(incorrect) = true, want false")
	}
}

func TestHashPasswordUsesRandomSalt(t *testing.T) {
	passphrase := "same passphrase"

	first, err := HashPassword(passphrase)
	if err != nil {
		t.Fatalf("HashPassword(first) error = %v", err)
	}
	second, err := HashPassword(passphrase)
	if err != nil {
		t.Fatalf("HashPassword(second) error = %v", err)
	}

	if first == second {
		t.Fatal("HashPassword() returned identical hashes for the same passphrase, want random salts")
	}
}

func TestVerifyPasswordRejectsMalformedHash(t *testing.T) {
	tests := []struct {
		name    string
		encoded string
	}{
		{name: "empty", encoded: ""},
		{name: "missing fields", encoded: "pbkdf2-sha256$600000"},
		{name: "wrong algorithm", encoded: "pbkdf2-sha1$600000$c2FsdA$a2V5"},
		{name: "bad iterations", encoded: "pbkdf2-sha256$many$c2FsdA$a2V5"},
		{name: "bad salt", encoded: "pbkdf2-sha256$600000$not base64$a2V5"},
		{name: "bad key", encoded: "pbkdf2-sha256$600000$c2FsdA$not base64"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			verified, err := VerifyPassword(tt.encoded, "passphrase")
			if err == nil && verified {
				t.Fatalf("VerifyPassword(%q) = true, nil; want false or error", tt.encoded)
			}
		})
	}
}
