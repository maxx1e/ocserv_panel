package config

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultsReturnsValidConfig(t *testing.T) {
	cfg := Defaults()
	if cfg.General.AppName != "ocserv Panel" {
		t.Fatalf("General.AppName = %q, want %q", cfg.General.AppName, "ocserv Panel")
	}
	if cfg.Web.Port != DefaultPort {
		t.Fatalf("Web.Port = %d, want %d", cfg.Web.Port, DefaultPort)
	}
	if cfg.Database.MaxOpenConns != 5 {
		t.Fatalf("Database.MaxOpenConns = %d, want 5", cfg.Database.MaxOpenConns)
	}
}

func TestLoadMissingFileReturnsDefaults(t *testing.T) {
	cfg, err := Load(filepath.Join(t.TempDir(), "nonexistent.toml"))
	if err != nil {
		t.Fatalf("Load missing file: %v", err)
	}
	if cfg.General.AppName != "ocserv Panel" {
		t.Fatalf("expected default app name, got %q", cfg.General.AppName)
	}
}

func TestLoadAndSaveRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	cfg := Defaults()
	cfg.General.AppName = "Test Panel"
	cfg.Web.Port = 9090

	if err := Save(cfg, path); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.General.AppName != "Test Panel" {
		t.Fatalf("AppName = %q, want %q", loaded.General.AppName, "Test Panel")
	}
	if loaded.Web.Port != 9090 {
		t.Fatalf("Port = %d, want 9090", loaded.Web.Port)
	}
}

func TestSaveCreatesBackup(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	cfg := Defaults()
	if err := Save(cfg, path); err != nil {
		t.Fatalf("initial Save: %v", err)
	}
	cfg.General.AppName = "Updated Panel"
	if err := Save(cfg, path); err != nil {
		t.Fatalf("second Save: %v", err)
	}
	backup := path + ".bak"
	if _, err := os.Stat(backup); os.IsNotExist(err) {
		t.Fatal("expected backup file to exist")
	}
}

func TestSaveSetsFilePermissions(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	cfg := Defaults()
	if err := Save(cfg, path); err != nil {
		t.Fatalf("Save: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("permissions = %v, want 0600", info.Mode().Perm())
	}
}

func TestValidateRejectsInvalidPort(t *testing.T) {
	cfg := Defaults()
	cfg.Web.Port = 0
	if err := Validate(cfg); err == nil {
		t.Fatal("expected validation error for port 0")
	}
	cfg.Web.Port = 99999
	if err := Validate(cfg); err == nil {
		t.Fatal("expected validation error for port 99999")
	}
}

func TestValidateAcceptsValidConfig(t *testing.T) {
	cfg := Defaults()
	if err := Validate(cfg); err != nil {
		t.Fatalf("valid config rejected: %v", err)
	}
}

func TestMergeEnvOverrides(t *testing.T) {
	cfg := Defaults()
	env := map[string]string{
		"OCSERV_PANEL_ADDR":         "0.0.0.0:3000",
		"OCSERV_PANEL_DB_DSN":       "file:test.db",
		"OCSERV_PANEL_COOKIE_SECURE": "false",
	}
	MergeEnv(&cfg, func(k string) string { return env[k] })
	if cfg.Web.Port != 3000 {
		t.Fatalf("Port = %d, want 3000", cfg.Web.Port)
	}
	if cfg.Database.DSN != "file:test.db" {
		t.Fatalf("DSN = %q, want %q", cfg.Database.DSN, "file:test.db")
	}
}

func TestLoadStrictMissingFileReturnsError(t *testing.T) {
	_, err := LoadStrict(filepath.Join(t.TempDir(), "nonexistent.toml"))
	if err == nil {
		t.Fatal("LoadStrict should return error for missing file")
	}
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected os.ErrNotExist in error chain, got: %v", err)
	}
}

func TestLoadStrictValidFileReturnsConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	cfg := Defaults()
	cfg.General.AppName = "Strict Test"
	if err := Save(cfg, path); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := LoadStrict(path)
	if err != nil {
		t.Fatalf("LoadStrict: %v", err)
	}
	if loaded.General.AppName != "Strict Test" {
		t.Fatalf("AppName = %q, want %q", loaded.General.AppName, "Strict Test")
	}
}

func TestLoadStrictParseErrorReturnsError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.toml")
	if err := os.WriteFile(path, []byte("not valid {{{{ TOML content"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadStrict(path)
	if err == nil {
		t.Fatal("LoadStrict should return error for unparseable file")
	}
}

// isWrappedNotExist checks for os.ErrNotExist in the error chain using errors.Is.
func isWrappedNotExist(err error) bool {
	return os.IsNotExist(err)
}

func TestDefaultsIncludeStaticDir(t *testing.T) {
	cfg := Defaults()
	if cfg.Installation.StaticDir != "/usr/share/ocserv-panel/static" {
		t.Fatalf("expected default StaticDir=/usr/share/ocserv-panel/static, got %q", cfg.Installation.StaticDir)
	}
}

func TestMergeEnvStaticDir(t *testing.T) {
	cfg := Defaults()
	MergeEnv(&cfg, func(key string) string {
		if key == "OCSERV_PANEL_STATIC_DIR" {
			return "/opt/custom/static"
		}
		return ""
	})
	if cfg.Installation.StaticDir != "/opt/custom/static" {
		t.Fatalf("expected StaticDir=/opt/custom/static after merge, got %q", cfg.Installation.StaticDir)
	}
}

func TestMergeEnvTLSCert(t *testing.T) {
	cfg := Defaults()
	MergeEnv(&cfg, func(key string) string {
		switch key {
		case "OCSERV_PANEL_TLS_CERT":
			return "/etc/letsencrypt/live/example.com/fullchain.pem"
		case "OCSERV_PANEL_TLS_KEY":
			return "/etc/letsencrypt/live/example.com/privkey.pem"
		}
		return ""
	})
	if cfg.Security.TLSCertPath != "/etc/letsencrypt/live/example.com/fullchain.pem" {
		t.Fatalf("TLSCertPath not merged from env")
	}
	if cfg.Security.TLSKeyPath != "/etc/letsencrypt/live/example.com/privkey.pem" {
		t.Fatalf("TLSKeyPath not merged from env")
	}
}
