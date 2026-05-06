package sanitize

import "testing"

func TestMetadataNilReturnsEmpty(t *testing.T) {
	result := Metadata(nil)
	if result == nil {
		t.Fatal("Metadata(nil) returned nil, want non-nil empty map")
	}
	if len(result) != 0 {
		t.Fatalf("Metadata(nil) = %v, want empty map", result)
	}
}

func TestMetadataRedactsSensitiveKeys(t *testing.T) {
	tests := []struct {
		key string
	}{
		{"password"},
		{"user_password"},
		{"passphrase"},
		{"session_token"},
		{"SESSION_TOKEN"},
		{"secret"},
		{"private-key"},
		{"ssh-private-key"},
		{"credential"},
		{"authorization"},
		{"cookie"},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			input := map[string]string{tt.key: "sensitive-value"}
			result := Metadata(input)
			if result[tt.key] != "[redacted]" {
				t.Fatalf("Metadata[%q] = %q, want [redacted]", tt.key, result[tt.key])
			}
		})
	}
}

func TestMetadataRedactsSensitiveValues(t *testing.T) {
	tests := []struct {
		name  string
		value string
	}{
		{"PEM private key", "-----BEGIN PRIVATE KEY-----\ndata\n-----END PRIVATE KEY-----"},
		{"RSA private key", "-----BEGIN RSA PRIVATE KEY-----\ndata\n-----END RSA PRIVATE KEY-----"},
		{"OpenSSH key", "-----BEGIN OPENSSH PRIVATE KEY-----\ndata\n-----END OPENSSH PRIVATE KEY-----"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := map[string]string{"note": tt.value}
			result := Metadata(input)
			if result["note"] != "[redacted]" {
				t.Fatalf("Metadata[note] = %q, want [redacted]", result["note"])
			}
		})
	}
}

func TestMetadataPreservesSafeValues(t *testing.T) {
	input := map[string]string{
		"field":  "safe-value",
		"action": "login",
		"ip":     "192.168.1.1",
	}
	result := Metadata(input)
	for key, expected := range input {
		if result[key] != expected {
			t.Fatalf("Metadata[%q] = %q, want %q", key, result[key], expected)
		}
	}
}

func TestIsSensitiveKeyNormalization(t *testing.T) {
	// Verify that normalization handles underscores, hyphens, spaces, and case
	tests := []struct {
		key  string
		want bool
	}{
		{"Password", true},
		{"PRIVATE_KEY", true},
		{"private-key", true},
		{"Private Key", true},
		{"safe_field", false},
		{"username", false},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			got := isSensitiveKey(tt.key)
			if got != tt.want {
				t.Fatalf("isSensitiveKey(%q) = %v, want %v", tt.key, got, tt.want)
			}
		})
	}
}
