package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"ocserv_panel/backend/internal/audit"
	"ocserv_panel/backend/internal/auth"
	"ocserv_panel/backend/internal/config"
	"ocserv_panel/backend/internal/db"
	"ocserv_panel/backend/internal/rbac"
	"ocserv_panel/backend/internal/store"
)

func TestSettingsServiceGetAllSectionsRequiresAuth(t *testing.T) {
	ctx := context.Background()
	s, recorder := newTestServices(t)
	cfg := config.Defaults()
	service := NewSettingsService(&cfg, "", s, recorder, rbac.NewDefaultAuthorizer())

	_, err := service.GetAllSections(ctx, auth.Principal{})
	if !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("unauthenticated GetAllSections err = %v, want ErrUnauthenticated", err)
	}
}

func TestSettingsServiceGetAllSectionsRequiresPermission(t *testing.T) {
	ctx := context.Background()
	s, recorder := newTestServices(t)
	viewer := createAdminWithRole(t, ctx, s, "viewer", rbac.RoleViewer)
	cfg := config.Defaults()
	service := NewSettingsService(&cfg, "", s, recorder, rbac.NewDefaultAuthorizer())

	// Viewer has settings:read permission, so this should succeed
	_, err := service.GetAllSections(ctx, viewer)
	if err != nil {
		t.Fatalf("viewer GetAllSections should succeed with settings:read, got err = %v", err)
	}
}

func TestSettingsServiceGetAllSectionsReturnsAllSections(t *testing.T) {
	ctx := context.Background()
	s, recorder := newTestServices(t)
	owner := createAdminWithRole(t, ctx, s, "owner", rbac.RoleOwner)
	cfg := config.Defaults()
	service := NewSettingsService(&cfg, "", s, recorder, rbac.NewDefaultAuthorizer())

	resp, err := service.GetAllSections(ctx, owner)
	if err != nil {
		t.Fatalf("GetAllSections: %v", err)
	}
	if len(resp.Sections) != len(AllSectionDefs()) {
		t.Fatalf("expected %d sections, got %d", len(AllSectionDefs()), len(resp.Sections))
	}
	// Verify first section is "general"
	if resp.Sections[0].ID != "general" {
		t.Fatalf("first section ID = %q, want %q", resp.Sections[0].ID, "general")
	}
	// Verify general section has settings
	if len(resp.Sections[0].Settings) == 0 {
		t.Fatal("general section should have settings")
	}
}

func TestSettingsServiceUpdateSettingRequiresAuth(t *testing.T) {
	ctx := context.Background()
	s, recorder := newTestServices(t)
	cfg := config.Defaults()
	service := NewSettingsService(&cfg, "", s, recorder, rbac.NewDefaultAuthorizer())

	_, err := service.UpdateSetting(ctx, auth.Principal{}, "general.theme", "dark")
	if !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("unauthenticated UpdateSetting err = %v, want ErrUnauthenticated", err)
	}
}

func TestSettingsServiceUpdateSettingRequiresOwner(t *testing.T) {
	ctx := context.Background()
	s, recorder := newTestServices(t)
	viewer := createAdminWithRole(t, ctx, s, "viewer", rbac.RoleViewer)
	cfg := config.Defaults()
	service := NewSettingsService(&cfg, "", s, recorder, rbac.NewDefaultAuthorizer())

	_, err := service.UpdateSetting(ctx, viewer, "general.theme", "dark")
	if !errors.Is(err, ErrPermissionDenied) {
		t.Fatalf("viewer UpdateSetting err = %v, want ErrPermissionDenied", err)
	}
}

func TestSettingsServiceUpdateSettingRejectsUnknownKey(t *testing.T) {
	ctx := context.Background()
	s, recorder := newTestServices(t)
	owner := createAdminWithRole(t, ctx, s, "owner", rbac.RoleOwner)
	cfg := config.Defaults()
	service := NewSettingsService(&cfg, "", s, recorder, rbac.NewDefaultAuthorizer())

	_, err := service.UpdateSetting(ctx, owner, "unknown.key", "value")
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("unknown key UpdateSetting err = %v, want ErrInvalidInput", err)
	}
}

func TestSettingsServiceUpdateSettingSavesDatabaseSetting(t *testing.T) {
	ctx := context.Background()
	s, recorder := newTestServices(t)
	owner := createAdminWithRole(t, ctx, s, "owner", rbac.RoleOwner)
	cfg := config.Defaults()
	service := NewSettingsService(&cfg, "", s, recorder, rbac.NewDefaultAuthorizer())

	result, err := service.UpdateSetting(ctx, owner, "general.theme", "dark")
	if err != nil {
		t.Fatalf("UpdateSetting: %v", err)
	}
	if result.Key != "general.theme" {
		t.Fatalf("result.Key = %q, want %q", result.Key, "general.theme")
	}

	// Verify stored value
	stored, err := s.GetSetting(ctx, "general.theme")
	if err != nil {
		t.Fatalf("GetSetting: %v", err)
	}
	if stored != "dark" {
		t.Fatalf("stored = %q, want %q", stored, "dark")
	}
}

func TestSettingsServiceUpdateSettingValidatesToggle(t *testing.T) {
	ctx := context.Background()
	s, recorder := newTestServices(t)
	owner := createAdminWithRole(t, ctx, s, "owner", rbac.RoleOwner)
	cfg := config.Defaults()
	service := NewSettingsService(&cfg, "", s, recorder, rbac.NewDefaultAuthorizer())

	_, err := service.UpdateSetting(ctx, owner, "security.csrf_enabled", "maybe")
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("invalid toggle UpdateSetting err = %v, want ErrInvalidInput", err)
	}
}

func TestSettingsServiceUpdateSettingValidatesSelect(t *testing.T) {
	ctx := context.Background()
	s, recorder := newTestServices(t)
	owner := createAdminWithRole(t, ctx, s, "owner", rbac.RoleOwner)
	cfg := config.Defaults()
	service := NewSettingsService(&cfg, "", s, recorder, rbac.NewDefaultAuthorizer())

	_, err := service.UpdateSetting(ctx, owner, "general.theme", "invalid_theme")
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("invalid select UpdateSetting err = %v, want ErrInvalidInput", err)
	}
}

func TestSettingsServiceUpdateSettingValidatesNumber(t *testing.T) {
	ctx := context.Background()
	s, recorder := newTestServices(t)
	owner := createAdminWithRole(t, ctx, s, "owner", rbac.RoleOwner)

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.toml")
	defaults := config.Defaults()
	if err := config.Save(defaults, cfgPath); err != nil {
		t.Fatal(err)
	}
	service := NewSettingsService(&defaults, cfgPath, s, recorder, rbac.NewDefaultAuthorizer())

	// Port must be between 1 and 65535 — validation happens before config write.
	_, err := service.UpdateSetting(ctx, owner, "web.port", "0")
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("port 0 UpdateSetting err = %v, want ErrInvalidInput", err)
	}

	_, err = service.UpdateSetting(ctx, owner, "web.port", "99999")
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("port 99999 UpdateSetting err = %v, want ErrInvalidInput", err)
	}

	// Valid port should succeed.
	_, err = service.UpdateSetting(ctx, owner, "web.port", "8080")
	if err != nil {
		t.Fatalf("valid port UpdateSetting err = %v", err)
	}
}

func TestValidateNumberWithConstraints(t *testing.T) {
	tests := []struct {
		name       string
		value      string
		validation string
		wantErr    bool
	}{
		{"valid number no constraints", "42", "", false},
		{"invalid number", "abc", "", true},
		{"min constraint pass", "5", "min:1", false},
		{"min constraint fail", "0", "min:1", true},
		{"max constraint pass", "100", "max:65535", false},
		{"max constraint fail", "70000", "max:65535", true},
		{"min and max pass", "8080", "min:1,max:65535", false},
		{"min and max fail low", "0", "min:1,max:65535", true},
		{"min and max fail high", "99999", "min:1,max:65535", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateNumberWithConstraints(tt.value, tt.validation)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateNumberWithConstraints(%q, %q) error = %v, wantErr %v", tt.value, tt.validation, err, tt.wantErr)
			}
		})
	}
}

// newSettingsSvcWithPath creates a SettingsService backed by an in-memory SQLite
// store and the given TOML config path. Use t.TempDir() to produce a
// config path that has already been written with config.Save.
func newSettingsSvcWithPath(t *testing.T, cfgPath string) *SettingsService {
	t.Helper()
	database, err := db.OpenSQLite(context.Background(), filepath.Join(t.TempDir(), "svc.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })
	if err := db.ApplyMigrations(context.Background(), database, "../../../migrations"); err != nil {
		t.Fatal(err)
	}
	s := store.New(database)
	defaults := config.Defaults()
	return NewSettingsService(&defaults, cfgPath, s, audit.NoopRecorder{}, rbac.NewDefaultAuthorizer())
}

func TestSettingsServiceUpdateSettingConfigSourceWritesToTOML(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.toml")
	defaults := config.Defaults()
	if err := config.Save(defaults, cfgPath); err != nil {
		t.Fatal(err)
	}

	svc := newSettingsSvcWithPath(t, cfgPath)
	principal := ownerPrincipal()

	_, err := svc.UpdateSetting(context.Background(), principal, "installation.ocserv_bin_path", "/usr/sbin/ocserv")
	if err != nil {
		t.Fatalf("UpdateSetting: %v", err)
	}

	// Read back from TOML — should have new value.
	updated, err := config.Load(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Installation.OcservBinPath != "/usr/sbin/ocserv" {
		t.Errorf("TOML OcservBinPath = %q, want /usr/sbin/ocserv", updated.Installation.OcservBinPath)
	}
}

func TestSettingsServiceUpdateSettingDatabaseSourceDoesNotWriteTOML(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.toml")
	defaults := config.Defaults()
	if err := config.Save(defaults, cfgPath); err != nil {
		t.Fatal(err)
	}

	svc := newSettingsSvcWithPath(t, cfgPath)
	principal := ownerPrincipal()

	// notifications.enabled is database-source.
	_, err := svc.UpdateSetting(context.Background(), principal, "notifications.enabled", "true")
	if err != nil {
		t.Fatalf("UpdateSetting database-source: %v", err)
	}

	// TOML should be unchanged (still default config).
	updated, err := config.Load(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Web.Port != defaults.Web.Port {
		t.Errorf("TOML was modified for database-source setting: port %d != %d", updated.Web.Port, defaults.Web.Port)
	}
}

func TestSettingsServiceGetAllSectionsReadsConfigSourceFromTOML(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.toml")

	// Write a custom config to TOML.
	defaults := config.Defaults()
	defaults.Installation.OcservBinPath = "/custom/path/ocserv"
	if err := config.Save(defaults, cfgPath); err != nil {
		t.Fatal(err)
	}

	svc := newSettingsSvcWithPath(t, cfgPath)
	principal := ownerPrincipal()

	resp, err := svc.GetAllSections(context.Background(), principal)
	if err != nil {
		t.Fatalf("GetAllSections: %v", err)
	}

	// Find installation section.
	var installSec *SettingsSection
	for i := range resp.Sections {
		if resp.Sections[i].ID == "installation" {
			installSec = &resp.Sections[i]
			break
		}
	}
	if installSec == nil {
		t.Fatal("installation section not found")
	}

	// Find the ocserv_bin_path setting.
	var found bool
	for _, s := range installSec.Settings {
		if s.Key == "installation.ocserv_bin_path" {
			if s.Value != "/custom/path/ocserv" {
				t.Errorf("setting value = %q, want /custom/path/ocserv", s.Value)
			}
			found = true
			break
		}
	}
	if !found {
		t.Error("installation.ocserv_bin_path not found in section settings")
	}
}

func TestSettingsServiceGetAllSectionsRestartRequired(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.toml")
	defaults := config.Defaults()
	if err := config.Save(defaults, cfgPath); err != nil {
		t.Fatal(err)
	}

	svc := newSettingsSvcWithPath(t, cfgPath)
	principal := ownerPrincipal()

	// Initially no pending changes.
	resp, err := svc.GetAllSections(context.Background(), principal)
	if err != nil {
		t.Fatalf("GetAllSections: %v", err)
	}
	for _, sec := range resp.Sections {
		if sec.RestartRequired {
			t.Errorf("section %q has RestartRequired=true but nothing changed", sec.ID)
		}
	}

	// Update a config-source setting.
	if _, err := svc.UpdateSetting(context.Background(), principal, "installation.ocserv_bin_path", "/usr/sbin/ocserv"); err != nil {
		t.Fatalf("UpdateSetting: %v", err)
	}

	// Now installation section should have RestartRequired=true.
	resp, err = svc.GetAllSections(context.Background(), principal)
	if err != nil {
		t.Fatalf("GetAllSections after change: %v", err)
	}

	var installSec *SettingsSection
	for i := range resp.Sections {
		if resp.Sections[i].ID == "installation" {
			installSec = &resp.Sections[i]
			break
		}
	}
	if installSec == nil {
		t.Fatal("installation section not found")
	}
	if !installSec.RestartRequired {
		t.Error("expected RestartRequired=true after config-source update")
	}
	found := false
	for _, k := range installSec.PendingChanges {
		if k == "installation.ocserv_bin_path" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected installation.ocserv_bin_path in PendingChanges, got %v", installSec.PendingChanges)
	}
}

func TestSettingsServiceRestartStatusMethod(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.toml")
	defaults := config.Defaults()
	if err := config.Save(defaults, cfgPath); err != nil {
		t.Fatal(err)
	}

	svc := newSettingsSvcWithPath(t, cfgPath)
	principal := ownerPrincipal()

	// No changes — restart not required.
	required, pending, err := svc.RestartStatus(context.Background(), principal)
	if err != nil {
		t.Fatalf("RestartStatus: %v", err)
	}
	if required {
		t.Error("expected restart_required=false with no changes")
	}
	if len(pending) != 0 {
		t.Errorf("expected empty pending, got %v", pending)
	}

	// Update a config-source key.
	if _, err := svc.UpdateSetting(context.Background(), principal, "web.port", "9443"); err != nil {
		t.Fatalf("UpdateSetting: %v", err)
	}

	required, pending, err = svc.RestartStatus(context.Background(), principal)
	if err != nil {
		t.Fatalf("RestartStatus after change: %v", err)
	}
	if !required {
		t.Error("expected restart_required=true after config-source change")
	}
	found := false
	for _, k := range pending {
		if k == "web.port" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected web.port in pending, got %v", pending)
	}
}

func TestSettingsServiceGetAllSectionsConfigStatusOK(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.toml")
	defaults := config.Defaults()
	if err := config.Save(defaults, cfgPath); err != nil {
		t.Fatal(err)
	}

	svc := newSettingsSvcWithPath(t, cfgPath)
	principal := ownerPrincipal()

	resp, err := svc.GetAllSections(context.Background(), principal)
	if err != nil {
		t.Fatalf("GetAllSections: %v", err)
	}
	if resp.ConfigStatus != "ok" {
		t.Errorf("config_status = %q, want %q", resp.ConfigStatus, "ok")
	}
	if resp.ConfigError != "" {
		t.Errorf("config_error = %q, want empty", resp.ConfigError)
	}
}

func TestSettingsServiceGetAllSectionsConfigStatusFileMissing(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "nonexistent.toml")

	svc := newSettingsSvcWithPath(t, cfgPath)
	principal := ownerPrincipal()

	resp, err := svc.GetAllSections(context.Background(), principal)
	if err != nil {
		t.Fatalf("GetAllSections: %v", err)
	}
	// config.Load returns defaults for missing files, so status stays ok.
	// Only LoadStrict errors on missing files.
	if resp.ConfigStatus != "ok" {
		t.Errorf("config_status = %q, want %q (Load returns defaults for missing files)", resp.ConfigStatus, "ok")
	}
}

func TestSettingsServiceGetAllSectionsConfigStatusParseError(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "bad.toml")
	if err := os.WriteFile(cfgPath, []byte("this is not valid {{{{ TOML"), 0644); err != nil {
		t.Fatal(err)
	}

	svc := newSettingsSvcWithPath(t, cfgPath)
	principal := ownerPrincipal()

	resp, err := svc.GetAllSections(context.Background(), principal)
	if err != nil {
		t.Fatalf("GetAllSections should not return error on parse failure, got: %v", err)
	}
	if resp.ConfigStatus != "parse_error" {
		t.Errorf("config_status = %q, want %q", resp.ConfigStatus, "parse_error")
	}
	if resp.ConfigError == "" {
		t.Error("config_error should be populated for parse errors")
	}
	// Should still return sections (from startup snapshot).
	if len(resp.Sections) == 0 {
		t.Error("expected sections from startup snapshot even with parse error")
	}
}

func TestSettingsServiceGetAllSectionsConfigStatusPermissionError(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("skipping permission test when running as root")
	}
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "noperm.toml")
	defaults := config.Defaults()
	if err := config.Save(defaults, cfgPath); err != nil {
		t.Fatal(err)
	}
	// Remove read permission.
	if err := os.Chmod(cfgPath, 0000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(cfgPath, 0644) })

	svc := newSettingsSvcWithPath(t, cfgPath)
	principal := ownerPrincipal()

	resp, err := svc.GetAllSections(context.Background(), principal)
	if err != nil {
		t.Fatalf("GetAllSections should not return error on permission failure, got: %v", err)
	}
	if resp.ConfigStatus != "permission_error" {
		t.Errorf("config_status = %q, want %q", resp.ConfigStatus, "permission_error")
	}
	if resp.ConfigError == "" {
		t.Error("config_error should be populated for permission errors")
	}
}

func TestSettingsServiceUpdateSettingConfigFileMissing(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "nonexistent.toml")

	svc := newSettingsSvcWithPath(t, cfgPath)
	principal := ownerPrincipal()

	_, err := svc.UpdateSetting(context.Background(), principal, "installation.ocserv_bin_path", "/usr/sbin/ocserv")
	if err == nil {
		t.Fatal("expected error for UpdateSetting with missing config file")
	}
	if !errors.Is(err, ErrConfigFileMissing) {
		t.Errorf("err = %v, want ErrConfigFileMissing in chain", err)
	}
}

func TestSettingsServiceUpdateSettingConfigParseError(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "bad.toml")
	if err := os.WriteFile(cfgPath, []byte("this is not valid {{{{ TOML"), 0644); err != nil {
		t.Fatal(err)
	}

	svc := newSettingsSvcWithPath(t, cfgPath)
	principal := ownerPrincipal()

	_, err := svc.UpdateSetting(context.Background(), principal, "installation.ocserv_bin_path", "/usr/sbin/ocserv")
	if err == nil {
		t.Fatal("expected error for UpdateSetting with unparseable config file")
	}
	if !errors.Is(err, ErrConfigFileParseFailed) {
		t.Errorf("err = %v, want ErrConfigFileParseFailed in chain", err)
	}
}

func TestSettingsServiceUpdateSettingConfigPermissionError(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("skipping permission test when running as root")
	}
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "noperm.toml")
	defaults := config.Defaults()
	if err := config.Save(defaults, cfgPath); err != nil {
		t.Fatal(err)
	}
	// Remove read permission.
	if err := os.Chmod(cfgPath, 0000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(cfgPath, 0644) })

	svc := newSettingsSvcWithPath(t, cfgPath)
	principal := ownerPrincipal()

	_, err := svc.UpdateSetting(context.Background(), principal, "installation.ocserv_bin_path", "/usr/sbin/ocserv")
	if err == nil {
		t.Fatal("expected error for UpdateSetting with unreadable config file")
	}
	if !errors.Is(err, ErrConfigFileUnreadable) {
		t.Errorf("err = %v, want ErrConfigFileUnreadable in chain", err)
	}
}

func TestClassifyConfigError(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		expect string
	}{
		{"not exist", os.ErrNotExist, "file_missing"},
		{"permission", os.ErrPermission, "permission_error"},
		{"parse error", errors.New("parse config file /etc/foo.toml: unexpected character"), "parse_error"},
		{"generic read error", errors.New("read config file: i/o timeout"), "read_error"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := classifyConfigError(tt.err)
			if got != tt.expect {
				t.Errorf("classifyConfigError(%v) = %q, want %q", tt.err, got, tt.expect)
			}
		})
	}
}

func TestWrapConfigLoadError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		sentinel error
	}{
		{"not exist", os.ErrNotExist, ErrConfigFileMissing},
		{"permission", os.ErrPermission, ErrConfigFileUnreadable},
		{"parse error", errors.New("parse config file: bad token"), ErrConfigFileParseFailed},
		{"other error", errors.New("read config file: i/o timeout"), ErrConfigFileUnreadable},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wrapped := wrapConfigLoadError(tt.err)
			if !errors.Is(wrapped, tt.sentinel) {
				t.Errorf("wrapConfigLoadError(%v) does not wrap %v", tt.err, tt.sentinel)
			}
		})
	}
}
