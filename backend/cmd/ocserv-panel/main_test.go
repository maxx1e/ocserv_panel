package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnvOrDefault(t *testing.T) {
	tests := []struct {
		name     string
		setValue *string
		fallback string
		want     string
	}{
		{name: "unset uses fallback", fallback: "fallback", want: "fallback"},
		{name: "empty uses fallback", setValue: stringPtr(""), fallback: "fallback", want: "fallback"},
		{name: "non-empty overrides fallback", setValue: stringPtr("configured"), fallback: "fallback", want: "configured"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key := "OCSERV_PANEL_TEST_ENV_OR_DEFAULT"
			if tt.setValue == nil {
				t.Setenv(key, "temporary")
				if err := os.Unsetenv(key); err != nil {
					t.Fatalf("unset env: %v", err)
				}
			} else {
				t.Setenv(key, *tt.setValue)
			}

			if got := envOrDefault(key, tt.fallback); got != tt.want {
				t.Fatalf("expected %q, got %q", tt.want, got)
			}
		})
	}
}

func TestCookieSecureFromEnv(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  bool
	}{
		{name: "empty defaults true", value: "", want: true},
		{name: "true enables secure cookies", value: "true", want: true},
		{name: "one enables secure cookies", value: "1", want: true},
		{name: "false disables secure cookies", value: "false", want: false},
		{name: "zero disables secure cookies", value: "0", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("OCSERV_PANEL_COOKIE_SECURE", tt.value)
			if got := cookieSecureFromEnv(); got != tt.want {
				t.Fatalf("expected %v, got %v", tt.want, got)
			}
		})
	}
}

func TestResolveMigrationsDirHonorsEnvAndFallbacks(t *testing.T) {
	t.Setenv("OCSERV_PANEL_MIGRATIONS_DIR", "/custom/migrations")
	if got := resolveMigrationsDir(); got != "/custom/migrations" {
		t.Fatalf("expected env migrations dir, got %q", got)
	}

	t.Setenv("OCSERV_PANEL_MIGRATIONS_DIR", "")
	temp := t.TempDir()
	if err := os.Mkdir(filepath.Join(temp, "migrations"), 0o755); err != nil {
		t.Fatalf("make migrations dir: %v", err)
	}
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("get wd: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(oldWD); err != nil {
			t.Fatalf("restore wd: %v", err)
		}
	})
	if err := os.Chdir(temp); err != nil {
		t.Fatalf("chdir temp: %v", err)
	}

	if got := resolveMigrationsDir(); got != "migrations" {
		t.Fatalf("expected local migrations fallback, got %q", got)
	}
}

func stringPtr(value string) *string {
	return &value
}
