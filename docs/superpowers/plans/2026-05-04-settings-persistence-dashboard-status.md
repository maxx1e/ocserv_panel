# Settings Persistence & Dashboard Status Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make config-source settings persist to TOML (not just SQLite), compute `restart_required`/`pending_changes` from a real diff, and have the dashboard ocserv status reflect updated installation paths without a process restart.

**Architecture:** `UpdateSetting` dispatches on `SettingDef.Source` — config-source keys load→patch→validate→save the TOML file; database-source keys write to SQLite as before. `GetAllSections` reads config-source values from the live TOML and computes a startup-vs-disk diff for `restart_required`. `ReadinessChecker` accepts a `func() ReadinessConfig` closure so it re-reads paths from disk on every `Check()` call.

**Tech Stack:** Go 1.25, `github.com/pelletier/go-toml/v2` (already a direct dep), `modernc.org/sqlite`, SvelteKit + Svelte 5 runes, Tailwind CSS v4.

---

## File Map

| File | Action | Responsibility |
|------|--------|---------------|
| `backend/internal/app/settings_config_apply.go` | **Create** | `applySettingToConfig`, `configValueForKey`, `computePendingChanges` |
| `backend/internal/app/settings_config_apply_test.go` | **Create** | Unit tests for key↔field mapping, round-trips, type coercion |
| `backend/internal/app/settings_service.go` | **Modify** | Add `configPath`+`startupCfg` fields; update `NewSettingsService`; dispatch in `UpdateSetting`; TOML-read + diff in `GetAllSections` |
| `backend/internal/app/settings_service_ext_test.go` | **Modify** | Add tests for config-source persistence, GetAllSections TOML read, restart diff |
| `backend/internal/ocserv/readiness.go` | **Modify** | Replace `cfg ReadinessConfig` with `cfgFn func() ReadinessConfig` |
| `backend/internal/ocserv/readiness_test.go` | **Modify** | Add tests proving `cfgFn` is called per `Check()` |
| `backend/internal/server/settings_handlers.go` | **Modify** | Wire `handleRestartStatus` to real diff from `SettingsService` |
| `backend/internal/server/settings_handlers_test.go` | **Modify** | Update `TestRestartReturnsPlaceholder`; add restart-required-after-change test |
| `backend/internal/server/server_test.go` | **Modify** | Pass `configPath` (temp dir TOML) to `NewSettingsService` in test harness |
| `backend/cmd/ocserv-panel/main.go` | **Modify** | Pass `configPath` to `NewSettingsService`; wrap `ReadinessChecker` with `cfgFn` closure |
| `frontend/src/lib/components/ui/SettingRow.svelte` | **Modify** | Add per-row amber "restart pending" chip shown after a successful save of a `requires_restart` setting |

---

## Task 1: Create `applySettingToConfig` and `configValueForKey`

**Files:**
- Create: `backend/internal/app/settings_config_apply.go`
- Create: `backend/internal/app/settings_config_apply_test.go`

- [ ] **Step 1: Write the failing tests**

Create `backend/internal/app/settings_config_apply_test.go`:

```go
package app

import (
	"errors"
	"strconv"
	"testing"

	"ocserv_panel/backend/internal/config"
)

func TestApplySettingToConfigUnknownKey(t *testing.T) {
	cfg := config.Defaults()
	err := applySettingToConfig(&cfg, "does.not.exist", "value")
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("unknown key err = %v, want ErrInvalidInput", err)
	}
}

func TestApplySettingToConfigStringField(t *testing.T) {
	cfg := config.Defaults()
	if err := applySettingToConfig(&cfg, "installation.ocserv_bin_path", "/custom/ocserv"); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if cfg.Installation.OcservBinPath != "/custom/ocserv" {
		t.Fatalf("OcservBinPath = %q, want %q", cfg.Installation.OcservBinPath, "/custom/ocserv")
	}
}

func TestApplySettingToConfigNumberField(t *testing.T) {
	cfg := config.Defaults()
	if err := applySettingToConfig(&cfg, "web.port", "9090"); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if cfg.Web.Port != 9090 {
		t.Fatalf("Web.Port = %d, want 9090", cfg.Web.Port)
	}
}

func TestApplySettingToConfigNumberFieldInvalid(t *testing.T) {
	cfg := config.Defaults()
	err := applySettingToConfig(&cfg, "web.port", "notanumber")
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("invalid number err = %v, want ErrInvalidInput", err)
	}
}

func TestApplySettingToConfigBoolField(t *testing.T) {
	cfg := config.Defaults()
	cfg.Service.StartOnBoot = true
	if err := applySettingToConfig(&cfg, "service.start_on_boot", "false"); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if cfg.Service.StartOnBoot != false {
		t.Fatalf("Service.StartOnBoot = %v, want false", cfg.Service.StartOnBoot)
	}
}

func TestApplySettingToConfigBoolFieldInvalid(t *testing.T) {
	cfg := config.Defaults()
	err := applySettingToConfig(&cfg, "service.start_on_boot", "yes")
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("invalid bool err = %v, want ErrInvalidInput", err)
	}
}

func TestConfigValueForKeyAllConfigSourceKeys(t *testing.T) {
	cfg := config.Defaults()
	// Every config-source key must round-trip through applySettingToConfig → configValueForKey.
	keys := []struct {
		key   string
		value string
	}{
		{"general.app_name", "MyPanel"},
		{"general.timezone", "UTC"},
		{"installation.config_path", "/etc/panel.toml"},
		{"installation.data_dir", "/var/lib/panel"},
		{"installation.log_dir", "/var/log/panel"},
		{"installation.ocserv_config_path", "/etc/ocserv/ocserv.conf"},
		{"installation.ocserv_socket_path", "/run/ocserv.sock"},
		{"installation.ocserv_bin_path", "/usr/sbin/ocserv"},
		{"installation.occtl_bin_path", "/usr/bin/occtl"},
		{"installation.ocpasswd_bin_path", "/usr/bin/ocpasswd"},
		{"installation.ocpasswd_file_path", "/etc/ocserv/ocpasswd"},
		{"installation.cert_dir", "/etc/certs"},
		{"service.name", "my-panel"},
		{"service.user", "paneluser"},
		{"service.group", "panelgroup"},
		{"service.start_on_boot", "false"},
		{"service.maintenance_mode", "true"},
		{"service.systemd_unit_name", "ocserv"},
		{"web.host", "0.0.0.0"},
		{"web.port", "9443"},
		{"web.base_url", "https://panel.example.com"},
		{"web.reverse_proxy", "true"},
		{"web.trusted_proxies", "10.0.0.1"},
		{"web.https", "true"},
		{"security.allowed_admin_ips", "192.168.1.0/24"},
		{"security.tls_cert_path", "/etc/certs/tls.crt"},
		{"security.tls_key_path", "/etc/certs/tls.key"},
		{"security.encryption_key_path", "/etc/certs/enc.key"},
		{"security.csrf_enabled", "false"},
		{"security.security_headers", "false"},
		{"database.dsn", "custom.db"},
		{"database.max_open_conns", "10"},
		{"database.max_idle_conns", "3"},
		{"database.conn_max_lifetime", "600"},
		{"monitoring.health_endpoint", "false"},
		{"monitoring.metrics_endpoint", "true"},
		{"monitoring.dashboard_refresh_interval", "30"},
	}
	for _, tc := range keys {
		t.Run(tc.key, func(t *testing.T) {
			c := cfg // copy
			if err := applySettingToConfig(&c, tc.key, tc.value); err != nil {
				t.Fatalf("applySettingToConfig(%q, %q): %v", tc.key, tc.value, err)
			}
			got := configValueForKey(c, tc.key)
			if got != tc.value {
				t.Fatalf("configValueForKey after apply = %q, want %q", got, tc.value)
			}
		})
	}
}

func TestComputePendingChangesNoDiff(t *testing.T) {
	cfg := config.Defaults()
	pending := computePendingChanges(cfg, cfg)
	if len(pending) != 0 {
		t.Fatalf("expected no pending changes when configs are identical, got %v", pending)
	}
}

func TestComputePendingChangesDetectsDiff(t *testing.T) {
	startup := config.Defaults()
	current := config.Defaults()
	current.Installation.OcservBinPath = "/new/path/ocserv"
	current.Web.Port = 9090

	pending := computePendingChanges(startup, current)
	if len(pending) == 0 {
		t.Fatal("expected pending changes when configs differ")
	}

	found := map[string]bool{}
	for _, k := range pending {
		found[k] = true
	}
	if !found["installation.ocserv_bin_path"] {
		t.Errorf("expected installation.ocserv_bin_path in pending, got %v", pending)
	}
	if !found["web.port"] {
		t.Errorf("expected web.port in pending, got %v", pending)
	}
}

// Helper: parse int without error (only call after validation).
func mustAtoi(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}
```

- [ ] **Step 2: Run tests to confirm they fail**

```bash
cd /opt/ocserv_panel/backend && go test ./internal/app/... -run "TestApply|TestConfigValue|TestComputePending" -v 2>&1 | head -30
```

Expected: compile error — `applySettingToConfig`, `configValueForKey`, `computePendingChanges` not defined.

- [ ] **Step 3: Implement `settings_config_apply.go`**

Create `backend/internal/app/settings_config_apply.go`:

```go
package app

import (
	"fmt"
	"sort"
	"strconv"

	"ocserv_panel/backend/internal/config"
)

// applySettingToConfig writes a setting key's string value into the correct
// field of cfg. Type coercion (string→int, string→bool) is performed here.
// Validation of the string form must have been done before calling this
// function via validateSettingValue — coercion panics are not expected.
func applySettingToConfig(cfg *config.Config, key, value string) error {
	switch key {
	// General
	case "general.app_name":
		cfg.General.AppName = value
	case "general.timezone":
		cfg.General.Timezone = value

	// Installation
	case "installation.config_path":
		cfg.Installation.ConfigPath = value
	case "installation.data_dir":
		cfg.Installation.DataDir = value
	case "installation.log_dir":
		cfg.Installation.LogDir = value
	case "installation.ocserv_config_path":
		cfg.Installation.OcservConfigPath = value
	case "installation.ocserv_socket_path":
		cfg.Installation.OcservSocketPath = value
	case "installation.ocserv_bin_path":
		cfg.Installation.OcservBinPath = value
	case "installation.occtl_bin_path":
		cfg.Installation.OcctlBinPath = value
	case "installation.ocpasswd_bin_path":
		cfg.Installation.OcpasswdBinPath = value
	case "installation.ocpasswd_file_path":
		cfg.Installation.OcpasswdFilePath = value
	case "installation.cert_dir":
		cfg.Installation.CertDir = value

	// Service
	case "service.name":
		cfg.Service.Name = value
	case "service.user":
		cfg.Service.User = value
	case "service.group":
		cfg.Service.Group = value
	case "service.start_on_boot":
		b, err := parseBool(value)
		if err != nil {
			return fmt.Errorf("%w: service.start_on_boot must be true or false", ErrInvalidInput)
		}
		cfg.Service.StartOnBoot = b
	case "service.maintenance_mode":
		b, err := parseBool(value)
		if err != nil {
			return fmt.Errorf("%w: service.maintenance_mode must be true or false", ErrInvalidInput)
		}
		cfg.Service.MaintenanceMode = b
	case "service.systemd_unit_name":
		cfg.Service.SystemdUnitName = value

	// Web
	case "web.host":
		cfg.Web.Host = value
	case "web.port":
		n, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("%w: web.port must be an integer", ErrInvalidInput)
		}
		cfg.Web.Port = n
	case "web.base_url":
		cfg.Web.BaseURL = value
	case "web.reverse_proxy":
		b, err := parseBool(value)
		if err != nil {
			return fmt.Errorf("%w: web.reverse_proxy must be true or false", ErrInvalidInput)
		}
		cfg.Web.ReverseProxy = b
	case "web.trusted_proxies":
		cfg.Web.TrustedProxies = value
	case "web.https":
		b, err := parseBool(value)
		if err != nil {
			return fmt.Errorf("%w: web.https must be true or false", ErrInvalidInput)
		}
		cfg.Web.HTTPS = b

	// Security
	case "security.allowed_admin_ips":
		cfg.Security.AllowedAdminIPs = value
	case "security.tls_cert_path":
		cfg.Security.TLSCertPath = value
	case "security.tls_key_path":
		cfg.Security.TLSKeyPath = value
	case "security.encryption_key_path":
		cfg.Security.EncryptionKeyPath = value
	case "security.csrf_enabled":
		b, err := parseBool(value)
		if err != nil {
			return fmt.Errorf("%w: security.csrf_enabled must be true or false", ErrInvalidInput)
		}
		cfg.Security.CSRFEnabled = b
	case "security.security_headers":
		b, err := parseBool(value)
		if err != nil {
			return fmt.Errorf("%w: security.security_headers must be true or false", ErrInvalidInput)
		}
		cfg.Security.SecurityHeaders = b

	// Database
	case "database.dsn":
		cfg.Database.DSN = value
	case "database.max_open_conns":
		n, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("%w: database.max_open_conns must be an integer", ErrInvalidInput)
		}
		cfg.Database.MaxOpenConns = n
	case "database.max_idle_conns":
		n, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("%w: database.max_idle_conns must be an integer", ErrInvalidInput)
		}
		cfg.Database.MaxIdleConns = n
	case "database.conn_max_lifetime":
		n, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("%w: database.conn_max_lifetime must be an integer", ErrInvalidInput)
		}
		cfg.Database.ConnMaxLifetime = n

	// Monitoring
	case "monitoring.health_endpoint":
		b, err := parseBool(value)
		if err != nil {
			return fmt.Errorf("%w: monitoring.health_endpoint must be true or false", ErrInvalidInput)
		}
		cfg.Monitoring.HealthEndpoint = b
	case "monitoring.metrics_endpoint":
		b, err := parseBool(value)
		if err != nil {
			return fmt.Errorf("%w: monitoring.metrics_endpoint must be true or false", ErrInvalidInput)
		}
		cfg.Monitoring.MetricsEndpoint = b
	case "monitoring.dashboard_refresh_interval":
		n, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("%w: monitoring.dashboard_refresh_interval must be an integer", ErrInvalidInput)
		}
		cfg.Monitoring.DashboardRefreshInterval = n

	default:
		return fmt.Errorf("%w: %s is not a config-source setting", ErrInvalidInput, key)
	}
	return nil
}

// configValueForKey extracts a config field as a string for a given key.
// Returns empty string for unknown keys. Only covers config-source keys.
func configValueForKey(cfg config.Config, key string) string {
	switch key {
	case "general.app_name":
		return cfg.General.AppName
	case "general.timezone":
		return cfg.General.Timezone
	case "installation.config_path":
		return cfg.Installation.ConfigPath
	case "installation.data_dir":
		return cfg.Installation.DataDir
	case "installation.log_dir":
		return cfg.Installation.LogDir
	case "installation.ocserv_config_path":
		return cfg.Installation.OcservConfigPath
	case "installation.ocserv_socket_path":
		return cfg.Installation.OcservSocketPath
	case "installation.ocserv_bin_path":
		return cfg.Installation.OcservBinPath
	case "installation.occtl_bin_path":
		return cfg.Installation.OcctlBinPath
	case "installation.ocpasswd_bin_path":
		return cfg.Installation.OcpasswdBinPath
	case "installation.ocpasswd_file_path":
		return cfg.Installation.OcpasswdFilePath
	case "installation.cert_dir":
		return cfg.Installation.CertDir
	case "service.name":
		return cfg.Service.Name
	case "service.user":
		return cfg.Service.User
	case "service.group":
		return cfg.Service.Group
	case "service.start_on_boot":
		return strconv.FormatBool(cfg.Service.StartOnBoot)
	case "service.maintenance_mode":
		return strconv.FormatBool(cfg.Service.MaintenanceMode)
	case "service.systemd_unit_name":
		return cfg.Service.SystemdUnitName
	case "web.host":
		return cfg.Web.Host
	case "web.port":
		return strconv.Itoa(cfg.Web.Port)
	case "web.base_url":
		return cfg.Web.BaseURL
	case "web.reverse_proxy":
		return strconv.FormatBool(cfg.Web.ReverseProxy)
	case "web.trusted_proxies":
		return cfg.Web.TrustedProxies
	case "web.https":
		return strconv.FormatBool(cfg.Web.HTTPS)
	case "security.allowed_admin_ips":
		return cfg.Security.AllowedAdminIPs
	case "security.tls_cert_path":
		return cfg.Security.TLSCertPath
	case "security.tls_key_path":
		return cfg.Security.TLSKeyPath
	case "security.encryption_key_path":
		return cfg.Security.EncryptionKeyPath
	case "security.csrf_enabled":
		return strconv.FormatBool(cfg.Security.CSRFEnabled)
	case "security.security_headers":
		return strconv.FormatBool(cfg.Security.SecurityHeaders)
	case "database.dsn":
		return cfg.Database.DSN
	case "database.max_open_conns":
		return strconv.Itoa(cfg.Database.MaxOpenConns)
	case "database.max_idle_conns":
		return strconv.Itoa(cfg.Database.MaxIdleConns)
	case "database.conn_max_lifetime":
		return strconv.Itoa(cfg.Database.ConnMaxLifetime)
	case "monitoring.health_endpoint":
		return strconv.FormatBool(cfg.Monitoring.HealthEndpoint)
	case "monitoring.metrics_endpoint":
		return strconv.FormatBool(cfg.Monitoring.MetricsEndpoint)
	case "monitoring.dashboard_refresh_interval":
		return strconv.Itoa(cfg.Monitoring.DashboardRefreshInterval)
	default:
		return ""
	}
}

// computePendingChanges returns the config-source setting keys where the current
// on-disk config differs from the startup config snapshot.
func computePendingChanges(startup, current config.Config) []string {
	var pending []string
	for _, def := range AllSettingDefs() {
		if def.Source != SettingSourceConfig {
			continue
		}
		startupVal := configValueForKey(startup, def.Key)
		currentVal := configValueForKey(current, def.Key)
		if startupVal != currentVal {
			pending = append(pending, def.Key)
		}
	}
	sort.Strings(pending)
	return pending
}

// parseBool parses "true"/"false" strictly. Returns error for any other value.
func parseBool(s string) (bool, error) {
	switch s {
	case "true":
		return true, nil
	case "false":
		return false, nil
	default:
		return false, fmt.Errorf("must be true or false, got %q", s)
	}
}
```

- [ ] **Step 4: Run tests and confirm they pass**

```bash
cd /opt/ocserv_panel/backend && go test ./internal/app/... -run "TestApply|TestConfigValue|TestComputePending" -v 2>&1 | tail -20
```

Expected: all tests PASS.

- [ ] **Step 5: Commit**

```bash
cd /opt/ocserv_panel/backend && git add internal/app/settings_config_apply.go internal/app/settings_config_apply_test.go && git commit -m "feat(app): add applySettingToConfig, configValueForKey, computePendingChanges"
```

---

## Task 2: Update `SettingsService` to persist config-source settings to TOML

**Files:**
- Modify: `backend/internal/app/settings_service.go`
- Modify: `backend/internal/app/settings_service_ext_test.go`

- [ ] **Step 1: Write the failing tests**

Add these tests to `backend/internal/app/settings_service_ext_test.go` (append to end of file):

```go
func TestSettingsServiceUpdateSettingConfigSourceWritesToTOML(t *testing.T) {
	ctx := context.Background()
	s, recorder := newTestServices(t)
	owner := createAdminWithRole(t, ctx, s, "owner", rbac.RoleOwner)

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.toml")
	cfg := config.Defaults()
	service := NewSettingsService(&cfg, cfgPath, s, recorder, rbac.NewDefaultAuthorizer())

	_, err := service.UpdateSetting(ctx, owner, "installation.ocserv_bin_path", "/custom/ocserv")
	if err != nil {
		t.Fatalf("UpdateSetting: %v", err)
	}

	// TOML file must exist and contain the new value.
	loaded, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("config.Load after update: %v", err)
	}
	if loaded.Installation.OcservBinPath != "/custom/ocserv" {
		t.Fatalf("OcservBinPath = %q, want %q", loaded.Installation.OcservBinPath, "/custom/ocserv")
	}

	// SQLite must NOT have been written for this config-source key.
	stored, err := s.GetSetting(ctx, "installation.ocserv_bin_path")
	if err != nil {
		t.Fatalf("GetSetting: %v", err)
	}
	if stored != "" {
		t.Fatalf("config-source setting should not be in SQLite, got %q", stored)
	}
}

func TestSettingsServiceUpdateSettingDatabaseSourceDoesNotWriteTOML(t *testing.T) {
	ctx := context.Background()
	s, recorder := newTestServices(t)
	owner := createAdminWithRole(t, ctx, s, "owner", rbac.RoleOwner)

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.toml")
	cfg := config.Defaults()
	service := NewSettingsService(&cfg, cfgPath, s, recorder, rbac.NewDefaultAuthorizer())

	_, err := service.UpdateSetting(ctx, owner, "general.theme", "dark")
	if err != nil {
		t.Fatalf("UpdateSetting: %v", err)
	}

	// TOML file must NOT exist (database-source only).
	if _, statErr := os.Stat(cfgPath); statErr == nil {
		t.Fatal("TOML file should not be written for database-source setting")
	}

	// SQLite must have the value.
	stored, err := s.GetSetting(ctx, "general.theme")
	if err != nil {
		t.Fatalf("GetSetting: %v", err)
	}
	if stored != "dark" {
		t.Fatalf("stored = %q, want %q", stored, "dark")
	}
}

func TestSettingsServiceGetAllSectionsReadsConfigSourceFromTOML(t *testing.T) {
	ctx := context.Background()
	s, recorder := newTestServices(t)
	owner := createAdminWithRole(t, ctx, s, "owner", rbac.RoleOwner)

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.toml")
	cfg := config.Defaults()
	service := NewSettingsService(&cfg, cfgPath, s, recorder, rbac.NewDefaultAuthorizer())

	// Write a TOML with a custom path before calling GetAllSections.
	customCfg := config.Defaults()
	customCfg.Installation.OcservBinPath = "/custom/ocserv"
	if err := config.Save(customCfg, cfgPath); err != nil {
		t.Fatalf("config.Save: %v", err)
	}

	resp, err := service.GetAllSections(ctx, owner)
	if err != nil {
		t.Fatalf("GetAllSections: %v", err)
	}

	// Find installation.ocserv_bin_path and verify value comes from TOML.
	for _, sec := range resp.Sections {
		if sec.ID != "installation" {
			continue
		}
		for _, setting := range sec.Settings {
			if setting.Key == "installation.ocserv_bin_path" {
				if setting.Value != "/custom/ocserv" {
					t.Fatalf("ocserv_bin_path = %q, want %q", setting.Value, "/custom/ocserv")
				}
				return
			}
		}
	}
	t.Fatal("installation.ocserv_bin_path not found in sections")
}

func TestSettingsServiceGetAllSectionsRestartRequired(t *testing.T) {
	ctx := context.Background()
	s, recorder := newTestServices(t)
	owner := createAdminWithRole(t, ctx, s, "owner", rbac.RoleOwner)

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.toml")
	startupCfg := config.Defaults()
	service := NewSettingsService(&startupCfg, cfgPath, s, recorder, rbac.NewDefaultAuthorizer())

	// No TOML yet → no diff → restart_required=false.
	resp, err := service.GetAllSections(ctx, owner)
	if err != nil {
		t.Fatalf("GetAllSections (no toml): %v", err)
	}
	if resp.RestartRequired {
		t.Fatal("expected restart_required=false when no TOML exists")
	}

	// Save a TOML that differs from startup.
	changed := config.Defaults()
	changed.Web.Port = 9090
	if err := config.Save(changed, cfgPath); err != nil {
		t.Fatalf("config.Save: %v", err)
	}

	resp, err = service.GetAllSections(ctx, owner)
	if err != nil {
		t.Fatalf("GetAllSections (changed toml): %v", err)
	}
	if !resp.RestartRequired {
		t.Fatal("expected restart_required=true when TOML differs from startup")
	}
	found := false
	for _, k := range resp.PendingChanges {
		if k == "web.port" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected web.port in pending_changes, got %v", resp.PendingChanges)
	}
}

func TestSettingsServiceRestartStatusMethod(t *testing.T) {
	ctx := context.Background()
	s, recorder := newTestServices(t)
	owner := createAdminWithRole(t, ctx, s, "owner", rbac.RoleOwner)

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.toml")
	startupCfg := config.Defaults()
	service := NewSettingsService(&startupCfg, cfgPath, s, recorder, rbac.NewDefaultAuthorizer())

	// No diff → not required.
	required, pending, err := service.RestartStatus(ctx, owner)
	if err != nil {
		t.Fatalf("RestartStatus: %v", err)
	}
	if required {
		t.Fatal("expected restart_required=false with no TOML")
	}
	if len(pending) != 0 {
		t.Fatalf("expected empty pending, got %v", pending)
	}

	// Apply a config-source change.
	if _, err := service.UpdateSetting(ctx, owner, "web.port", "9090"); err != nil {
		t.Fatalf("UpdateSetting: %v", err)
	}

	required, pending, err = service.RestartStatus(ctx, owner)
	if err != nil {
		t.Fatalf("RestartStatus after update: %v", err)
	}
	if !required {
		t.Fatal("expected restart_required=true after config-source update")
	}
	found := false
	for _, k := range pending {
		if k == "web.port" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected web.port in pending, got %v", pending)
	}
}
```

Also add `"os"` and `"path/filepath"` to the existing imports block at the top of that file. The file currently starts with:
```go
package app

import (
	"context"
	"errors"
	"testing"

	"ocserv_panel/backend/internal/auth"
	"ocserv_panel/backend/internal/config"
	"ocserv_panel/backend/internal/rbac"
)
```

Change to:
```go
package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"ocserv_panel/backend/internal/auth"
	"ocserv_panel/backend/internal/config"
	"ocserv_panel/backend/internal/rbac"
)
```

- [ ] **Step 2: Run tests to confirm they fail**

```bash
cd /opt/ocserv_panel/backend && go test ./internal/app/... -run "TestSettingsServiceUpdateSettingConfigSource|TestSettingsServiceUpdateSettingDatabase|TestSettingsServiceGetAllSectionsReadsConfig|TestSettingsServiceGetAllSectionsRestartRequired|TestSettingsServiceRestartStatus" -v 2>&1 | head -40
```

Expected: compile error — `NewSettingsService` doesn't accept `configPath`, `RestartStatus` not defined.

- [ ] **Step 3: Update `settings_service.go`**

Replace the `SettingsService` struct, `NewSettingsService`, `GetAllSections`, and `UpdateSetting` in `backend/internal/app/settings_service.go`.

**a) Update the struct** — find the existing struct and replace:

```go
// SettingsService manages application settings stored in app_settings.
type SettingsService struct {
	config        *config.Config
	configPath    string
	startupCfg    config.Config
	settingsStore settingsStore
	recorder      audit.Recorder
	authorizer    rbac.Authorizer
	now           func() time.Time
}
```

**b) Update `NewSettingsService`** — replace the existing function:

```go
// NewSettingsService returns a SettingsService backed by SQLite storage.
// configPath is the path to the TOML config file on disk; it is used for
// config-source setting reads and writes. Pass an empty string to disable
// TOML persistence (tests that don't need it).
func NewSettingsService(cfg *config.Config, configPath string, s *store.Store, recorder audit.Recorder, authorizer rbac.Authorizer) *SettingsService {
	if recorder == nil {
		recorder = audit.NoopRecorder{}
	}
	var startupCfg config.Config
	if cfg != nil {
		startupCfg = *cfg
	}
	return &SettingsService{
		config:        cfg,
		configPath:    configPath,
		startupCfg:    startupCfg,
		settingsStore: s,
		recorder:      recorder,
		authorizer:    authorizer,
		now:           func() time.Time { return time.Now().UTC() },
	}
}
```

**c) Update `UpdateSetting`** — replace the existing `UpdateSetting` method. The new version dispatches on `def.Source`:

```go
// UpdateSetting updates a single setting value.
// Config-source settings are written to the TOML file; database-source
// settings are written to SQLite.
func (s *SettingsService) UpdateSetting(ctx context.Context, principal auth.Principal, key, value string) (SettingDef, error) {
	if !principal.Authenticated() {
		_ = s.record(ctx, nil, "settings.update", "setting", key, audit.OutcomeDenied, "settings update authentication required", map[string]string{"value": value})
		return SettingDef{}, ErrUnauthenticated
	}
	actorID := principal.ID
	if !s.authorizer.Allowed(principalRoles(principal), rbac.PermissionSettingsManage) {
		_ = s.record(ctx, &actorID, "settings.update", "setting", key, audit.OutcomeDenied, "settings update permission denied", map[string]string{"value": value})
		return SettingDef{}, ErrPermissionDenied
	}

	def, found := FindSettingDef(key)
	if !found {
		_ = s.record(ctx, &actorID, "settings.update", "setting", key, audit.OutcomeDenied, "settings update unknown key", map[string]string{"value": value})
		return SettingDef{}, ErrInvalidInput
	}

	// Validate the value based on type (string form, before coercion).
	if err := validateSettingValue(def, value); err != nil {
		_ = s.record(ctx, &actorID, "settings.update", "setting", key, audit.OutcomeDenied, "settings update validation failed", map[string]string{"value": value, "error": err.Error()})
		return SettingDef{}, err
	}

	if def.Source == SettingSourceConfig {
		if err := s.updateConfigSetting(ctx, actorID, key, value); err != nil {
			return SettingDef{}, err
		}
	} else {
		oldValue, _ := s.settingsStore.GetSetting(ctx, key)
		if oldValue == "" {
			oldValue = def.Default
		}
		if err := s.settingsStore.SetSetting(ctx, key, value); err != nil {
			_ = s.record(ctx, &actorID, "settings.update", "setting", key, audit.OutcomeFailure, "settings update save failed", map[string]string{"value": value})
			return SettingDef{}, fmt.Errorf("update setting %q: %w", key, err)
		}
		_ = s.record(ctx, &actorID, "settings.update", "setting", key, audit.OutcomeSuccess, "settings update saved", map[string]string{"old_value": oldValue, "new_value": value})
	}

	return SettingDef{
		Key:             def.Key,
		Label:           def.Label,
		Section:         def.Section,
		Type:            def.Type,
		Source:          def.Source,
		RequiresRestart: def.RequiresRestart,
		Default:         def.Default,
		Value:           value,
		Options:         def.Options,
		Validation:      def.Validation,
		SortOrder:       def.SortOrder,
	}, nil
}

// updateConfigSetting loads the current TOML, applies the key change, validates,
// and saves atomically.
func (s *SettingsService) updateConfigSetting(ctx context.Context, actorID int64, key, value string) error {
	current, err := config.Load(s.configPath)
	if err != nil {
		_ = s.record(ctx, &actorID, "settings.update", "setting", key, audit.OutcomeFailure, "settings update config load failed", map[string]string{"value": value, "error": err.Error()})
		return fmt.Errorf("load config for update %q: %w", key, err)
	}

	oldValue := configValueForKey(current, key)

	if err := applySettingToConfig(&current, key, value); err != nil {
		_ = s.record(ctx, &actorID, "settings.update", "setting", key, audit.OutcomeDenied, "settings update apply failed", map[string]string{"value": value, "error": err.Error()})
		return err
	}

	if err := config.Validate(current); err != nil {
		_ = s.record(ctx, &actorID, "settings.update", "setting", key, audit.OutcomeDenied, "settings update config validation failed", map[string]string{"value": value, "error": err.Error()})
		return fmt.Errorf("%w: %v", ErrInvalidInput, err)
	}

	if err := config.Save(current, s.configPath); err != nil {
		_ = s.record(ctx, &actorID, "settings.update", "setting", key, audit.OutcomeFailure, "settings update config save failed", map[string]string{"value": value, "error": err.Error()})
		return fmt.Errorf("save config for update %q: %w", key, err)
	}

	_ = s.record(ctx, &actorID, "settings.update", "setting", key, audit.OutcomeSuccess, "settings update saved to config", map[string]string{"old_value": oldValue, "new_value": value})
	return nil
}
```

**d) Update `GetAllSections`** — replace the config-source value assignment and the final return. Find the loop inside `GetAllSections` that assigns `value`:

Replace:
```go
		for _, d := range defs {
			value := d.Default
			if d.Source == SettingSourceDatabase {
				stored, err := s.settingsStore.GetSetting(ctx, d.Key)
				if err == nil && stored != "" {
					value = stored
				}
			}
```

With:
```go
		// Load current TOML once for this request (for config-source fields).
		currentCfg, _ := config.Load(s.configPath)

		for _, d := range defs {
			value := d.Default
			if d.Source == SettingSourceDatabase {
				stored, err := s.settingsStore.GetSetting(ctx, d.Key)
				if err == nil && stored != "" {
					value = stored
				}
			} else {
				// Config-source: read from the live TOML on disk.
				if v := configValueForKey(currentCfg, d.Key); v != "" {
					value = v
				}
			}
```

Note: the `currentCfg, _ := config.Load(s.configPath)` line must be placed **before** the `for _, d := range defs` loop but **inside** the `for _, sec := range sectionDefs` loop — i.e., it is loaded once per request (outside all inner loops). Place it just before the `for _, sec := range sectionDefs {` line instead:

```go
	// Load current TOML once for this request (all config-source reads use this).
	currentCfg, _ := config.Load(s.configPath)

	for _, sec := range sectionDefs {
		defs := defsBySection[sec.ID]
		settings := make([]SettingDef, 0, len(defs))
		for _, d := range defs {
			value := d.Default
			if d.Source == SettingSourceDatabase {
				stored, err := s.settingsStore.GetSetting(ctx, d.Key)
				if err == nil && stored != "" {
					value = stored
				}
			} else {
				// Config-source: read from the live TOML on disk.
				if v := configValueForKey(currentCfg, d.Key); v != "" {
					value = v
				}
			}
```

Also update the final `return` in `GetAllSections` to compute the real diff:

Replace:
```go
	return &SettingsSectionsResponse{
		Sections:        sections,
		RestartRequired: false, // will be computed from config state
		PendingChanges:  []string{},
	}, nil
```

With:
```go
	pending := computePendingChanges(s.startupCfg, currentCfg)
	return &SettingsSectionsResponse{
		Sections:        sections,
		RestartRequired: len(pending) > 0,
		PendingChanges:  pending,
	}, nil
```

**e) Add `RestartStatus` method** — add this new method after `GetAllSections`:

```go
// RestartStatus returns whether a service restart is needed and which config keys
// differ between the startup snapshot and the current TOML on disk.
func (s *SettingsService) RestartStatus(ctx context.Context, principal auth.Principal) (restartRequired bool, pendingChanges []string, err error) {
	if !principal.Authenticated() {
		_ = s.record(ctx, nil, "settings.restart_status", "setting", "", audit.OutcomeDenied, "restart status authentication required", nil)
		return false, nil, ErrUnauthenticated
	}
	actorID := principal.ID
	if !s.authorizer.Allowed(principalRoles(principal), rbac.PermissionSettingsRead) {
		_ = s.record(ctx, &actorID, "settings.restart_status", "setting", "", audit.OutcomeDenied, "restart status permission denied", nil)
		return false, nil, ErrPermissionDenied
	}
	currentCfg, _ := config.Load(s.configPath)
	pending := computePendingChanges(s.startupCfg, currentCfg)
	return len(pending) > 0, pending, nil
}
```

- [ ] **Step 4: Run the new tests**

```bash
cd /opt/ocserv_panel/backend && go test ./internal/app/... -run "TestSettingsServiceUpdateSettingConfigSource|TestSettingsServiceUpdateSettingDatabase|TestSettingsServiceGetAllSectionsReadsConfig|TestSettingsServiceGetAllSectionsRestartRequired|TestSettingsServiceRestartStatus" -v 2>&1 | tail -30
```

Expected: all PASS.

- [ ] **Step 5: Run the full app test suite to confirm no regressions**

```bash
cd /opt/ocserv_panel/backend && go test ./internal/app/... -v 2>&1 | grep -E "^(ok|FAIL|---)" | head -40
```

Expected: all PASS. Fix any compile errors from `NewSettingsService` signature change before committing.

- [ ] **Step 6: Commit**

```bash
cd /opt/ocserv_panel/backend && git add internal/app/settings_service.go internal/app/settings_service_ext_test.go && git commit -m "feat(app): dispatch UpdateSetting by source — config→TOML, db→SQLite; real restart diff"
```

---

## Task 3: Update `ReadinessChecker` to use `cfgFn` closure

**Files:**
- Modify: `backend/internal/ocserv/readiness.go`
- Modify: `backend/internal/ocserv/readiness_test.go`

- [ ] **Step 1: Write the failing tests**

Add to `backend/internal/ocserv/readiness_test.go`:

```go
func TestReadinessCheckerCfgFnCalledPerCheck(t *testing.T) {
	t.Parallel()

	callCount := 0
	runner := NewFakeCommandRunner()
	svc := NewFakeServiceManager(ServiceInactive)

	checker := NewReadinessChecker(func() ReadinessConfig {
		callCount++
		return ReadinessConfig{
			OcservBinPath:   "/nonexistent/ocserv",
			OcctlBinPath:    "/nonexistent/occtl",
			OcpasswdBinPath: "/nonexistent/ocpasswd",
			ConfigPath:      "/nonexistent/ocserv.conf",
			SocketPath:      "/nonexistent/socket",
			UnitName:        "ocserv",
		}
	}, runner, svc)

	if _, err := checker.Check(context.Background()); err != nil {
		t.Fatalf("first Check: %v", err)
	}
	if _, err := checker.Check(context.Background()); err != nil {
		t.Fatalf("second Check: %v", err)
	}

	if callCount != 2 {
		t.Fatalf("cfgFn called %d times, want 2", callCount)
	}
}

func TestReadinessCheckerPicksUpPathChange(t *testing.T) {
	t.Parallel()

	// Create a real temp file to simulate a binary existing on disk.
	dir := t.TempDir()
	binPath := filepath.Join(dir, "ocserv")
	if err := os.WriteFile(binPath, []byte("fake"), 0755); err != nil {
		t.Fatalf("create fake binary: %v", err)
	}

	usePath := "/nonexistent/ocserv" // start with missing path
	runner := NewFakeCommandRunner()
	svc := NewFakeServiceManager(ServiceInactive)

	checker := NewReadinessChecker(func() ReadinessConfig {
		return ReadinessConfig{
			OcservBinPath:   usePath,
			OcctlBinPath:    "/nonexistent/occtl",
			OcpasswdBinPath: "/nonexistent/ocpasswd",
			ConfigPath:      "/nonexistent/ocserv.conf",
			SocketPath:      "/nonexistent/socket",
			UnitName:        "ocserv",
		}
	}, runner, svc)

	// First check: binary missing.
	r1, err := checker.Check(context.Background())
	if err != nil {
		t.Fatalf("first Check: %v", err)
	}
	if r1.BinaryPresent {
		t.Error("expected BinaryPresent=false on first check")
	}

	// Update the path to the real temp file.
	usePath = binPath

	// Second check: binary present.
	r2, err := checker.Check(context.Background())
	if err != nil {
		t.Fatalf("second Check: %v", err)
	}
	if !r2.BinaryPresent {
		t.Error("expected BinaryPresent=true after path change")
	}
}
```

Also add these imports to `readiness_test.go` (they are needed by the new tests):
```go
import (
	"context"
	"os"
	"path/filepath"
	"testing"
)
```

- [ ] **Step 2: Run tests to confirm they fail**

```bash
cd /opt/ocserv_panel/backend && go test ./internal/ocserv/... -run "TestReadinessCheckerCfgFn|TestReadinessCheckerPicksUp" -v 2>&1 | head -20
```

Expected: compile error — `NewReadinessChecker` signature mismatch.

- [ ] **Step 3: Update `readiness.go`**

In `backend/internal/ocserv/readiness.go`:

**a) Replace the struct field:**

```go
// ReadinessChecker probes the system for ocserv installation and runtime state.
type ReadinessChecker struct {
	cfgFn  func() ReadinessConfig
	runner CommandRunner
	svc    ServiceManager
}
```

**b) Replace `NewReadinessChecker`:**

```go
// NewReadinessChecker returns a ReadinessChecker that calls cfgFn on each Check
// to get the current configuration. This allows path changes written to the
// config file to be picked up without a process restart.
func NewReadinessChecker(cfgFn func() ReadinessConfig, runner CommandRunner, svc ServiceManager) *ReadinessChecker {
	return &ReadinessChecker{cfgFn: cfgFn, runner: runner, svc: svc}
}
```

**c) Update `Check` to call `cfgFn()`:**

```go
// Check probes the system and returns the current ocserv readiness state.
// Each probe is independent — a failure in one does not block the others.
func (c *ReadinessChecker) Check(ctx context.Context) (*OcservReadiness, error) {
	cfg := c.cfgFn()
	r := &OcservReadiness{}

	// Binary existence checks (filesystem only — fast and safe).
	r.BinaryPresent = fileExists(cfg.OcservBinPath)
	r.OcctlPresent = fileExists(cfg.OcctlBinPath)
	r.OcpasswdPresent = fileExists(cfg.OcpasswdBinPath)

	// Version from binary — only if binary exists.
	if r.BinaryPresent {
		r.BinaryVersion = c.checkVersion(ctx, cfg.OcservBinPath)
	}

	// Config file presence.
	r.ConfigPresent = fileExists(cfg.ConfigPath)

	// Config validation — only if both binary and config are present.
	if r.ConfigPresent && r.BinaryPresent {
		r.ConfigValid, r.ConfigErrors = c.validateConfig(ctx, cfg.OcservBinPath)
	}

	// Service status from ServiceManager.
	if status, err := c.svc.Status(ctx); err == nil {
		r.ServiceRunning = status == ServiceActive
	}

	// Management socket presence.
	r.SocketPresent = fileExists(cfg.SocketPath)

	return r, nil
}
```

**d) Update `checkVersion` and `validateConfig` to accept the binary path as a parameter** (they currently read from `c.cfg`):

```go
// checkVersion runs the binary with --version and extracts the first line.
func (c *ReadinessChecker) checkVersion(ctx context.Context, binPath string) string {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	result, err := c.runner.Run(ctx, binPath, []string{"--version"}, nil)
	if err != nil {
		return ""
	}
	out := result.Stdout
	if out == "" {
		out = result.Stderr
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) > 0 {
		return strings.TrimSpace(lines[0])
	}
	return ""
}

// validateConfig runs `ocserv -f -t` to check the configuration file.
func (c *ReadinessChecker) validateConfig(ctx context.Context, binPath string) (bool, string) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	result, err := c.runner.Run(ctx, binPath, []string{"-f", "-t"}, nil)
	if err != nil {
		return false, fmt.Sprintf("config validation failed: %v", err)
	}
	if result.ExitCode != 0 {
		msg := strings.TrimSpace(result.Stderr)
		if msg == "" {
			msg = strings.TrimSpace(result.Stdout)
		}
		return false, msg
	}
	return true, ""
}
```

- [ ] **Step 4: Update existing readiness tests to use the new `cfgFn` signature**

In `readiness_test.go`, all existing calls to `NewReadinessChecker(ReadinessConfig{...}, runner, svc)` must become:

```go
// Example: TestReadinessCheckerAllBinariesMissing
checker := NewReadinessChecker(func() ReadinessConfig {
    return ReadinessConfig{
        OcservBinPath:   "/nonexistent/ocserv",
        OcctlBinPath:    "/nonexistent/occtl",
        OcpasswdBinPath: "/nonexistent/ocpasswd",
        ConfigPath:      "/nonexistent/ocserv.conf",
        SocketPath:      "/nonexistent/socket",
        UnitName:        "ocserv",
    }
}, runner, svc)
```

Apply this pattern to all 5 existing tests (`TestReadinessCheckerAllBinariesMissing`, `TestReadinessCheckerVersionExtracted`, `TestReadinessCheckerServiceRunning`, `TestReadinessCheckerConfigValidationOnlyRunsWhenBothPresent`, `TestReadinessCheckerRealBinariesNotRequired`).

For `TestReadinessCheckerRealBinariesNotRequired`, change:
```go
checker := NewReadinessChecker(ReadinessConfig{}, runner, svc)
```
To:
```go
checker := NewReadinessChecker(func() ReadinessConfig { return ReadinessConfig{} }, runner, svc)
```

- [ ] **Step 5: Run the full ocserv test suite**

```bash
cd /opt/ocserv_panel/backend && go test ./internal/ocserv/... -v 2>&1 | grep -E "^(ok|FAIL|---)" | head -30
```

Expected: all PASS.

- [ ] **Step 6: Commit**

```bash
cd /opt/ocserv_panel/backend && git add internal/ocserv/readiness.go internal/ocserv/readiness_test.go && git commit -m "feat(ocserv): ReadinessChecker reads config via cfgFn closure for live path updates"
```

---

## Task 4: Wire `handleRestartStatus` to real diff; update `main.go` and test harness

**Files:**
- Modify: `backend/internal/server/settings_handlers.go`
- Modify: `backend/internal/server/settings_handlers_test.go`
- Modify: `backend/internal/server/server_test.go`
- Modify: `backend/cmd/ocserv-panel/main.go`

- [ ] **Step 1: Update `settings_handlers.go`**

The `settingsService` interface needs a `RestartStatus` method. Find the `settingsService` interface in `server.go` or wherever it is declared. Add `RestartStatus` to it:

First, check where the interface is:
```bash
grep -n "settingsService\|SettingsService" /opt/ocserv_panel/backend/internal/server/server.go | head -20
```

Add `RestartStatus` to the interface:
```go
RestartStatus(ctx context.Context, principal auth.Principal) (restartRequired bool, pendingChanges []string, err error)
```

Then replace `handleRestartStatus` in `settings_handlers.go`:

```go
func (a *App) handleRestartStatus(w http.ResponseWriter, r *http.Request) {
	if !allowMethod(w, r, http.MethodGet) {
		return
	}
	principal, ok := a.authenticate(w, r)
	if !ok {
		return
	}
	required, pending, err := a.settingsService.RestartStatus(r.Context(), principal)
	if err != nil {
		a.writeAppError(w, err)
		return
	}
	if pending == nil {
		pending = []string{}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"restart_required": required,
		"pending_changes":  pending,
	})
}
```

- [ ] **Step 2: Update `settings_handlers_test.go`**

Replace `TestRestartReturnsPlaceholder` with a test that verifies the placeholder message is gone, and add a test that verifies `restart_required` becomes true after a config-source change:

Find and replace `TestRestartReturnsPlaceholder`:
```go
func TestRestartReturnsOK(t *testing.T) {
	h := newTestHarness(t)
	h.createOwner(t, "testowner", "testpass123")
	cookie := h.login(t, "testowner", "testpass123")

	rec := serveAPI(t, h.router, http.MethodPost, "/api/settings/restart", nil, cookie)

	assertStatus(t, rec, http.StatusOK)
	assertJSONField(t, rec, "ok", true)
}
```

Add a new test after `TestRestartStatusReturnsNotRequired`:

```go
func TestRestartStatusTrueAfterConfigSourceChange(t *testing.T) {
	h := newTestHarnessWithTempConfig(t)
	h.createOwner(t, "testowner", "testpass123")
	cookie := h.login(t, "testowner", "testpass123")

	// Initially not required.
	rec := serveAPI(t, h.router, http.MethodGet, "/api/settings/restart-status", nil, cookie)
	assertStatus(t, rec, http.StatusOK)
	var before map[string]any
	decodeJSON(t, rec, &before)
	if before["restart_required"].(bool) {
		t.Fatal("expected restart_required=false before any changes")
	}

	// Update a config-source setting.
	rec = serveAPI(t, h.router, http.MethodPut, "/api/settings/web.port", map[string]string{"value": "9090"}, cookie)
	assertStatus(t, rec, http.StatusOK)

	// Now restart should be required.
	rec = serveAPI(t, h.router, http.MethodGet, "/api/settings/restart-status", nil, cookie)
	assertStatus(t, rec, http.StatusOK)
	var after map[string]any
	decodeJSON(t, rec, &after)
	if !after["restart_required"].(bool) {
		t.Fatal("expected restart_required=true after config-source change")
	}
	pending, ok := after["pending_changes"].([]any)
	if !ok || len(pending) == 0 {
		t.Fatalf("expected non-empty pending_changes, got %v", after["pending_changes"])
	}
}
```

- [ ] **Step 3: Update `server_test.go` test harness**

In `backend/internal/server/server_test.go`:

**a)** Update the `NewSettingsService` call in `newTestHarnessWithCookieSecure` to pass an empty `configPath`:

Find:
```go
settingsService := app.NewSettingsService(&cfg, s, recorder, rbac.NewDefaultAuthorizer())
```

Replace with:
```go
settingsService := app.NewSettingsService(&cfg, "", s, recorder, rbac.NewDefaultAuthorizer())
```

**b)** Add a `newTestHarnessWithTempConfig` constructor. This constructor builds a fully independent harness (does not reuse `newTestHarness`) to avoid having to expose internal services:

```go
// newTestHarnessWithTempConfig creates a harness with a real TOML config path so
// that config-source settings are persisted to disk and restart diffs work.
func newTestHarnessWithTempConfig(t *testing.T) *testHarness {
	t.Helper()

	ctx := context.Background()
	database, err := db.OpenSQLite(ctx, filepath.Join(t.TempDir(), "panel.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() {
		if err := database.Close(); err != nil {
			t.Fatalf("close sqlite: %v", err)
		}
	})
	if err := db.ApplyMigrations(ctx, database, migrationsDir(t)); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}

	s := store.New(database)
	recorder := audit.NewSQLiteRecorder(s)
	authService := app.NewAuthService(s, recorder)
	dashboardService := app.NewDashboardService(s, recorder, rbac.NewDefaultAuthorizer(), nil)
	monitoringService := app.NewMonitoringService(metrics.NewMemoryStore(), rbac.NewDefaultAuthorizer(), nil, recorder)
	cfg := config.Defaults()
	cfgPath := filepath.Join(t.TempDir(), "config.toml")
	settingsService := app.NewSettingsService(&cfg, cfgPath, s, recorder, rbac.NewDefaultAuthorizer())
	ocservSvc, fakeManager := newTestOcservServiceWithFake(s, recorder)

	return &testHarness{
		database:   database,
		store:      s,
		ocservFake: fakeManager,
		router: NewAppRouter(
			authService,
			dashboardService,
			monitoringService,
			settingsService,
			ocservSvc,
			rbac.NewDefaultAuthorizer(),
			AppOptions{CookieSecure: false},
		),
	}
}
```

Note: `filepath` is already imported in `server_test.go`. No new imports needed.

- [ ] **Step 4: Update `main.go`**

In `backend/cmd/ocserv-panel/main.go`:

**a)** Find the `settingsService` construction and add `configPath`:
```go
settingsService := app.NewSettingsService(&cfg, configPath, persistence, recorder, rbac.NewDefaultAuthorizer())
```

**b)** Replace the static `NewReadinessChecker` call with a `cfgFn` closure:
```go
readinessChecker := ocserv.NewReadinessChecker(
    func() ocserv.ReadinessConfig {
        liveCfg, _ := config.Load(configPath)
        return ocserv.ReadinessConfig{
            OcservBinPath:   liveCfg.Installation.OcservBinPath,
            OcctlBinPath:    liveCfg.Installation.OcctlBinPath,
            OcpasswdBinPath: liveCfg.Installation.OcpasswdBinPath,
            ConfigPath:      liveCfg.Installation.OcservConfigPath,
            SocketPath:      liveCfg.Installation.OcservSocketPath,
            UnitName:        liveCfg.Service.SystemdUnitName,
        }
    },
    runner,
    svcManager,
)
```

- [ ] **Step 5: Build to catch compile errors**

```bash
cd /opt/ocserv_panel/backend && go build ./... 2>&1
```

Expected: no errors. Fix any interface satisfaction issues (the `settingsService` interface must include `RestartStatus`).

- [ ] **Step 6: Run full backend test suite**

```bash
cd /opt/ocserv_panel/backend && go test ./... 2>&1 | grep -E "^(ok|FAIL)" | head -30
```

Expected: all packages PASS.

- [ ] **Step 7: Commit**

```bash
cd /opt/ocserv_panel/backend && git add internal/server/settings_handlers.go internal/server/settings_handlers_test.go internal/server/server_test.go cmd/ocserv-panel/main.go && git commit -m "feat(server): wire real restart diff to handleRestartStatus; update main.go wiring"
```

---

## Task 5: Frontend — per-row "restart pending" indicator in `SettingRow`

**Files:**
- Modify: `frontend/src/lib/components/ui/SettingRow.svelte`

- [ ] **Step 1: Examine the current `SettingRow` structure**

```bash
cat /opt/ocserv_panel/frontend/src/lib/components/ui/SettingRow.svelte
```

The component already has `requiresRestart` prop and shows a static ↻ icon when `requiresRestart=true` (this means "this setting type always requires restart"). We need to add a separate `savedPending` prop that shows an amber "restart pending" chip **after a successful save** — distinct from the static indicator that merely signals this setting *would* require a restart.

- [ ] **Step 2: Update `SettingRow.svelte`**

Replace the entire file content with:

```svelte
<script lang="ts">
  import SettingTooltip from './SettingTooltip.svelte';

  type Props = {
    label: string;
    description?: string;
    requiresRestart?: boolean;
    savedPending?: boolean;
    isSaving?: boolean;
    children: import('svelte').Snippet;
  };

  let { label, description, requiresRestart = false, savedPending = false, isSaving = false, children }: Props = $props();
</script>

<div class="grid grid-cols-1 md:grid-cols-[240px_1fr] gap-1 md:gap-4 py-3 border-b border-slate-100 last:border-0 items-center">
  <div class="flex items-center justify-end md:text-right">
    <span class="font-medium text-slate-700 text-sm">{label}</span>
    <SettingTooltip {description} />
    {#if requiresRestart}
      <span
        class="inline-flex items-center justify-center w-5 h-5 rounded-full bg-amber-100 text-amber-600 text-xs font-bold ml-1"
        title="Requires service restart"
      >&#x21BB;</span>
    {/if}
    {#if isSaving}
      <span class="text-xs text-slate-400 ml-2">Saving…</span>
    {/if}
  </div>
  <div class="flex items-center gap-2">
    {@render children()}
    {#if savedPending}
      <span class="inline-flex items-center gap-1 px-2 py-0.5 rounded-full bg-amber-50 border border-amber-200 text-amber-700 text-xs font-medium">
        <span class="text-amber-500">&#x21BB;</span> restart pending
      </span>
    {/if}
  </div>
</div>
```

- [ ] **Step 3: Update individual Setting components to pass `savedPending`**

Each setting component (`SettingToggle`, `SettingText`, `SettingNumber`, `SettingSelect`, `SettingPassword`, `SettingMultiCheckbox`) wraps `SettingRow`. They need to track whether the last save was for a `requires_restart` setting.

The pattern is: after `onUpdate` resolves without error, if `setting.requires_restart` is true, set `savedPending = true`. The `savedPending` state resets if the component is re-rendered with a new value (which happens when `data` is refreshed in the parent page after `handleUpdate`).

Update `SettingToggle.svelte` as the reference pattern:

```svelte
<script lang="ts">
  import SettingRow from './SettingRow.svelte';
  import { settingDescriptions } from '$lib/settings/descriptions';
  import type { SettingDef } from '$lib/api';

  type Props = {
    setting: SettingDef;
    onUpdate: (key: string, value: string) => Promise<void>;
    savingKey: string | null;
  };

  let { setting, onUpdate, savingKey }: Props = $props();
  const isSaving = $derived(savingKey === setting.key);
  let savedPending = $state(false);

  async function toggleChange(newValue: boolean) {
    try {
      await onUpdate(setting.key, String(newValue));
      if (setting.requires_restart) {
        savedPending = true;
      }
    } catch {
      // Error handled by parent
    }
  }
</script>

<SettingRow
  label={setting.label}
  description={settingDescriptions[setting.key]}
  requiresRestart={setting.requires_restart}
  {savedPending}
  {isSaving}
>
  <div class="flex items-center gap-3">
    <button
      class="relative inline-flex h-6 w-11 items-center rounded-full transition-colors {setting.value === 'true' ? 'bg-slate-600' : 'bg-slate-300'} {isSaving ? 'opacity-50 cursor-not-allowed' : 'cursor-pointer'}"
      onclick={() => toggleChange(setting.value !== 'true')}
      disabled={isSaving}
      role="switch"
      aria-checked={setting.value === 'true'}
      aria-label={setting.label}
      type="button"
    >
      <span class="inline-block h-4 w-4 transform rounded-full bg-white transition-transform {setting.value === 'true' ? 'translate-x-6' : 'translate-x-1'}"></span>
    </button>
    <span class="text-sm text-slate-500">{setting.value === 'true' ? 'Enabled' : 'Disabled'}</span>
  </div>
</SettingRow>
```

Apply the same `savedPending` pattern to `SettingText.svelte`, `SettingNumber.svelte`, `SettingSelect.svelte`, `SettingPassword.svelte`, and `SettingMultiCheckbox.svelte`. In each:
1. Add `let savedPending = $state(false);`
2. After the `await onUpdate(...)` call succeeds, add: `if (setting.requires_restart) { savedPending = true; }`
3. Pass `{savedPending}` to `<SettingRow>`.

`SettingButton` and `SettingReadonly` do not need this change (readonly has no save, button has its own flow).

- [ ] **Step 4: Type-check the frontend**

```bash
cd /opt/ocserv_panel/frontend && npx svelte-check --tsconfig ./tsconfig.json 2>&1 | tail -10
```

Expected: 0 errors.

- [ ] **Step 5: Build the frontend**

```bash
cd /opt/ocserv_panel/frontend && npm run build 2>&1 | tail -10
```

Expected: build succeeds.

- [ ] **Step 6: Commit**

```bash
cd /opt/ocserv_panel && git add frontend/src/lib/components/ui/SettingRow.svelte frontend/src/lib/components/ui/SettingToggle.svelte frontend/src/lib/components/ui/SettingText.svelte frontend/src/lib/components/ui/SettingNumber.svelte frontend/src/lib/components/ui/SettingSelect.svelte frontend/src/lib/components/ui/SettingPassword.svelte frontend/src/lib/components/ui/SettingMultiCheckbox.svelte && git commit -m "feat(frontend): show per-row restart-pending chip after saving config-source settings"
```

---

## Task 6: Final verification

- [ ] **Step 1: Run the complete backend test suite**

```bash
cd /opt/ocserv_panel/backend && go test ./... 2>&1 | grep -E "^(ok|FAIL)"
```

Expected: all packages `ok`.

- [ ] **Step 2: Run the frontend type-check and build**

```bash
cd /opt/ocserv_panel/frontend && npx svelte-check --tsconfig ./tsconfig.json 2>&1 | tail -5 && npm run build 2>&1 | tail -5
```

Expected: 0 errors, build succeeds.

- [ ] **Step 3: Verify the binary compiles cleanly**

```bash
cd /opt/ocserv_panel/backend && go build -o /dev/null ./cmd/ocserv-panel/ 2>&1
```

Expected: no output (clean compile).

- [ ] **Step 4: Commit final state if any loose files remain**

```bash
cd /opt/ocserv_panel && git status
```

If any files are untracked or modified, add and commit them.
