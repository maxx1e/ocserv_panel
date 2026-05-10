package app

import (
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

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
	case "installation.ocserv_log_path":
		cfg.Installation.OcservLogPath = value
	case "installation.panel_log_path":
		cfg.Installation.PanelLogPath = value

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
	case "monitoring.log_file_read_enabled":
		b, err := parseBool(value)
		if err != nil {
			return fmt.Errorf("%w: monitoring.log_file_read_enabled must be true or false", ErrInvalidInput)
		}
		cfg.Monitoring.LogFileReadEnabled = b
	case "monitoring.network_interface":
		cfg.Monitoring.NetworkInterface = value
	case "monitoring.vpn_interface":
		cfg.Monitoring.VPNInterface = value
	case "monitoring.collector_interval":
		n, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("%w: monitoring.collector_interval must be an integer", ErrInvalidInput)
		}
		if n < 5 {
			return fmt.Errorf("%w: monitoring.collector_interval must be at least 5", ErrInvalidInput)
		}
		cfg.Monitoring.CollectorInterval = n

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
	case "installation.ocserv_log_path":
		return cfg.Installation.OcservLogPath
	case "installation.panel_log_path":
		return cfg.Installation.PanelLogPath
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
		return "/api/health (always enabled)"
	case "monitoring.metrics_endpoint":
		return "Not implemented"
	case "monitoring.dashboard_refresh_interval":
		return strconv.Itoa(cfg.Monitoring.DashboardRefreshInterval)
	case "monitoring.log_file_read_enabled":
		return strconv.FormatBool(cfg.Monitoring.LogFileReadEnabled)
	case "monitoring.network_interface":
		return cfg.Monitoring.NetworkInterface
	case "monitoring.vpn_interface":
		return cfg.Monitoring.VPNInterface
	case "monitoring.collector_interval":
		return strconv.Itoa(cfg.Monitoring.CollectorInterval)
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

// allowedBinDirs restricts binary path settings to system directories.
var allowedBinDirs = []string{
	"/usr/bin",
	"/usr/local/bin",
	"/usr/sbin",
	"/usr/local/sbin",
}

// allowedLogDirs restricts log path settings to safe directories.
var allowedLogDirs = []string{
	"/var/log",
	"/opt/ocserv_panel",
}

// systemdUnitPattern only allows unit names starting with "ocserv".
var systemdUnitPattern = regexp.MustCompile(`^ocserv[a-zA-Z0-9._-]*$`)

// validateSettingValueSecurity performs security validation for sensitive settings.
// It enforces allow-lists for binary paths, systemd unit names, and log paths
// to prevent arbitrary code execution and arbitrary file reads.
func validateSettingValueSecurity(key, value string) error {
	switch key {
	case "installation.ocserv_bin_path", "installation.occtl_bin_path", "installation.ocpasswd_bin_path":
		return validateBinaryPath(value)
	case "service.systemd_unit_name":
		return validateSystemdUnitName(value)
	case "installation.ocserv_log_path", "installation.panel_log_path":
		return validateLogPath(value)
	}
	return nil
}

func validateBinaryPath(path string) error {
	if !filepath.IsAbs(path) {
		return fmt.Errorf("%w: binary path must be absolute", ErrInvalidInput)
	}
	cleaned := filepath.Clean(path)
	dir := filepath.Dir(cleaned)
	for _, allowed := range allowedBinDirs {
		if dir == allowed {
			return nil
		}
	}
	return fmt.Errorf("%w: binary path must be in /usr/bin, /usr/local/bin, /usr/sbin, or /usr/local/sbin", ErrInvalidInput)
}

func validateSystemdUnitName(name string) error {
	if name == "" {
		return fmt.Errorf("%w: systemd unit name must not be empty", ErrInvalidInput)
	}
	if !systemdUnitPattern.MatchString(name) {
		return fmt.Errorf("%w: systemd unit name must start with 'ocserv' and contain only [a-zA-Z0-9._-]", ErrInvalidInput)
	}
	return nil
}

func validateLogPath(path string) error {
	if !filepath.IsAbs(path) {
		return fmt.Errorf("%w: log path must be absolute", ErrInvalidInput)
	}
	cleaned := filepath.Clean(path)
	for _, allowed := range allowedLogDirs {
		if strings.HasPrefix(cleaned, allowed+"/") || cleaned == allowed {
			return nil
		}
	}
	return fmt.Errorf("%w: log path must be within /var/log or /opt/ocserv_panel", ErrInvalidInput)
}
