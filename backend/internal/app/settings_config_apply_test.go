package app

import (
	"errors"
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

	// Verify the returned slice is sorted.
	for i := 1; i < len(pending); i++ {
		if pending[i] < pending[i-1] {
			t.Errorf("expected pending changes to be sorted, got %v", pending)
			break
		}
	}
}

// TestConfigValueForKeyCoversAllRegistryConfigSourceKeys verifies that every
// config-source key in the settings registry is handled by both
// applySettingToConfig and configValueForKey. A key missing from either switch
// is caught here before it can silently corrupt computePendingChanges.
func TestConfigValueForKeyCoversAllRegistryConfigSourceKeys(t *testing.T) {
	for _, def := range AllSettingDefs() {
		if def.Source != SettingSourceConfig {
			continue
		}
		// Readonly settings are informational only and cannot be applied via the settings UI.
		if def.Type == SettingTypeReadonly {
			continue
		}
		t.Run(def.Key, func(t *testing.T) {
			// Use a sentinel value for string-type fields so that a missing
			// case in configValueForKey (which returns "") is distinguishable
			// from a legitimately empty default value.
			const sentinel = "__coverage_sentinel__"

			applyValue := def.Default
			expectValue := def.Default
			switch def.Type {
			case SettingTypeText, SettingTypePassword, SettingTypeSelect:
				applyValue = sentinel
				expectValue = sentinel
			}

			cfg := config.Defaults()

			// applySettingToConfig must not return an error for a valid value.
			if err := applySettingToConfig(&cfg, def.Key, applyValue); err != nil {
				t.Fatalf("applySettingToConfig(%q, %q) returned unexpected error: %v — key may be missing from applySettingToConfig switch", def.Key, applyValue, err)
			}

			// configValueForKey must return the applied value, confirming the
			// key is present in its switch statement.
			got := configValueForKey(cfg, def.Key)
			if got != expectValue {
				t.Fatalf("configValueForKey(%q) = %q, want %q — key may be missing from configValueForKey switch", def.Key, got, expectValue)
			}
		})
	}
}

func TestApplySettingToConfigEmptyStringField(t *testing.T) {
	cfg := config.Defaults()
	if err := applySettingToConfig(&cfg, "general.app_name", ""); err != nil {
		t.Fatalf("expected no error for empty string field, got: %v", err)
	}
	if cfg.General.AppName != "" {
		t.Errorf("expected empty app name, got %q", cfg.General.AppName)
	}
}

func TestValidateSettingValueSecurity_BinaryPathAllowList(t *testing.T) {
	binPathKeys := []string{
		"installation.ocserv_bin_path",
		"installation.occtl_bin_path",
		"installation.ocpasswd_bin_path",
	}
	allowedPaths := []string{
		"/usr/bin/ocserv",
		"/usr/local/bin/occtl",
		"/usr/sbin/ocserv",
		"/usr/local/sbin/ocserv",
	}
	blockedPaths := []string{
		"/tmp/evil",
		"../relative",
		"ocserv",
		"/home/user/bin/ocserv",
		"/var/tmp/hack",
	}
	for _, key := range binPathKeys {
		for _, path := range allowedPaths {
			if err := validateSettingValueSecurity(key, path); err != nil {
				t.Errorf("validateSettingValueSecurity(%q, %q) = %v, want nil", key, path, err)
			}
		}
		for _, path := range blockedPaths {
			if err := validateSettingValueSecurity(key, path); err == nil {
				t.Errorf("validateSettingValueSecurity(%q, %q) = nil, want error", key, path)
			}
		}
	}
}

func TestValidateSettingValueSecurity_SystemdUnitName(t *testing.T) {
	allowed := []string{"ocserv", "ocserv.service", "ocserv-vpn", "ocserv_custom"}
	for _, name := range allowed {
		if err := validateSettingValueSecurity("service.systemd_unit_name", name); err != nil {
			t.Errorf("validateSettingValueSecurity(unit, %q) = %v, want nil", name, err)
		}
	}
	blocked := []string{"sshd", "nginx", "ssh.service", "", "ocserv; rm -rf /"}
	for _, name := range blocked {
		if err := validateSettingValueSecurity("service.systemd_unit_name", name); err == nil {
			t.Errorf("validateSettingValueSecurity(unit, %q) = nil, want error", name)
		}
	}
}

func TestValidateSettingValueSecurity_LogPath(t *testing.T) {
	allowed := []string{"/var/log/ocserv.log", "/var/log/ocserv-panel/app.log", "/opt/ocserv_panel/panel.log"}
	for _, p := range allowed {
		if err := validateSettingValueSecurity("installation.ocserv_log_path", p); err != nil {
			t.Errorf("validateSettingValueSecurity(log, %q) = %v, want nil", p, err)
		}
	}
	blocked := []string{"/etc/shadow", "/etc/passwd", "../../../etc/shadow", "relative.log"}
	for _, p := range blocked {
		if err := validateSettingValueSecurity("installation.ocserv_log_path", p); err == nil {
			t.Errorf("validateSettingValueSecurity(log, %q) = nil, want error", p)
		}
	}
}

func TestValidateSettingValueSecurity_NonSensitiveKeysPassThrough(t *testing.T) {
	// Non-sensitive keys should always return nil
	if err := validateSettingValueSecurity("general.app_name", "anything"); err != nil {
		t.Errorf("non-sensitive key should pass, got %v", err)
	}
	if err := validateSettingValueSecurity("web.port", "8080"); err != nil {
		t.Errorf("non-sensitive key should pass, got %v", err)
	}
}
