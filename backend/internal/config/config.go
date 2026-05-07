package config

import (
	"fmt"
	"os"
	"path/filepath"
)

// DefaultPort is the default HTTP port for the web server.
const DefaultPort = 8080

// GeneralConfig holds general application settings.
type GeneralConfig struct {
	AppName  string `toml:"app_name"`
	Timezone string `toml:"timezone"`
}

// InstallationConfig holds filesystem path settings.
type InstallationConfig struct {
	ConfigPath       string `toml:"config_path"`
	DataDir          string `toml:"data_dir"`
	LogDir           string `toml:"log_dir"`
	OcservConfigPath string `toml:"ocserv_config_path"`
	OcservSocketPath string `toml:"ocserv_socket_path"`
	OcservBinPath    string `toml:"ocserv_bin_path"`
	OcctlBinPath     string `toml:"occtl_bin_path"`
	OcpasswdBinPath  string `toml:"ocpasswd_bin_path"`
	OcpasswdFilePath string `toml:"ocpasswd_file_path"`
	CertDir          string `toml:"cert_dir"`
	OcservLogPath    string `toml:"ocserv_log_path"`
	PanelLogPath     string `toml:"panel_log_path"`
	PanelSystemdUnit string `toml:"panel_systemd_unit"`
}

// ServiceConfig holds service runtime settings.
type ServiceConfig struct {
	Name            string `toml:"name"`
	User            string `toml:"user"`
	Group           string `toml:"group"`
	StartOnBoot     bool   `toml:"start_on_boot"`
	MaintenanceMode bool   `toml:"maintenance_mode"`
	SystemdUnitName string `toml:"systemd_unit_name"`
}

// WebConfig holds web server settings.
type WebConfig struct {
	Host           string `toml:"host"`
	Port           int    `toml:"port"`
	BaseURL        string `toml:"base_url"`
	ReverseProxy   bool   `toml:"reverse_proxy"`
	TrustedProxies string `toml:"trusted_proxies"`
	HTTPS          bool   `toml:"https"`
}

// SecurityConfig holds security settings.
type SecurityConfig struct {
	AllowedAdminIPs   string `toml:"allowed_admin_ips"`
	TLSCertPath       string `toml:"tls_cert_path"`
	TLSKeyPath        string `toml:"tls_key_path"`
	EncryptionKeyPath string `toml:"encryption_key_path"`
	CSRFEnabled       bool   `toml:"csrf_enabled"`
	SecurityHeaders   bool   `toml:"security_headers"`
}

// DatabaseConfig holds database connection settings.
type DatabaseConfig struct {
	DSN             string `toml:"dsn"`
	MaxOpenConns    int    `toml:"max_open_conns"`
	MaxIdleConns    int    `toml:"max_idle_conns"`
	ConnMaxLifetime int    `toml:"conn_max_lifetime"`
}

// MonitoringConfig holds monitoring endpoint settings.
type MonitoringConfig struct {
	HealthEndpoint           bool `toml:"health_endpoint"`
	MetricsEndpoint          bool `toml:"metrics_endpoint"`
	DashboardRefreshInterval int  `toml:"dashboard_refresh_interval"`
	LogFileReadEnabled       bool `toml:"log_file_read_enabled"`
}

// Config is the top-level TOML configuration structure.
type Config struct {
	General      GeneralConfig      `toml:"general"`
	Installation InstallationConfig `toml:"installation"`
	Service      ServiceConfig      `toml:"service"`
	Web          WebConfig          `toml:"web"`
	Security     SecurityConfig     `toml:"security"`
	Database     DatabaseConfig     `toml:"database"`
	Monitoring   MonitoringConfig   `toml:"monitoring"`
}

// Defaults returns a Config with all values set to their defaults.
func Defaults() Config {
	return Config{
		General: GeneralConfig{
			AppName:  "ocserv Panel",
			Timezone: "UTC",
		},
		Installation: InstallationConfig{
			ConfigPath:       "/etc/ocserv-panel/config.toml",
			DataDir:          "/var/lib/ocserv-panel",
			LogDir:           "/var/log/ocserv-panel",
			OcservConfigPath: "/etc/ocserv/ocserv.conf",
			OcservSocketPath: "/var/run/ocserv-socket",
			OcservBinPath:    "/usr/local/sbin/ocserv",
			OcctlBinPath:     "/usr/local/bin/occtl",
			OcpasswdBinPath:  "/usr/local/bin/ocpasswd",
			OcpasswdFilePath: "/etc/ocserv/ocpasswd",
			CertDir:          "/etc/ocserv/certs",
			OcservLogPath:    "/var/log/ocserv/ocserv.log",
			PanelLogPath:     "/var/log/ocserv-panel/panel.log",
			PanelSystemdUnit: "ocserv-panel",
		},
		Service: ServiceConfig{
			Name:            "ocserv-panel",
			User:            "ocserv-panel",
			Group:           "ocserv-panel",
			StartOnBoot:     true,
			MaintenanceMode: false,
			SystemdUnitName: "ocserv",
		},
		Web: WebConfig{
			Host:    "127.0.0.1",
			Port:    DefaultPort,
			BaseURL: "http://localhost:8080",
		},
		Security: SecurityConfig{
			CSRFEnabled:     true,
			SecurityHeaders: true,
		},
		Database: DatabaseConfig{
			DSN:             "ocserv-panel.db",
			MaxOpenConns:    5,
			MaxIdleConns:    2,
			ConnMaxLifetime: 300,
		},
		Monitoring: MonitoringConfig{
			HealthEndpoint:           true,
			MetricsEndpoint:          false,
			DashboardRefreshInterval: 15,
			LogFileReadEnabled:       true,
		},
	}
}

// Load reads a TOML config file and merges with defaults.
// If the file does not exist, returns defaults without error.
func Load(path string) (Config, error) {
	cfg := Defaults()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return Config{}, fmt.Errorf("read config file %s: %w", path, err)
	}
	if err := unmarshalTOML(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse config file %s: %w", path, err)
	}

	// Backward compatibility: if OcpasswdFilePath is empty, try the legacy
	// ocpasswd_path TOML key that was renamed in v2.
	if cfg.Installation.OcpasswdFilePath == "" {
		var raw map[string]any
		if err := unmarshalTOMLRaw(data, &raw); err == nil {
			if inst, ok := raw["installation"].(map[string]any); ok {
				if v, ok := inst["ocpasswd_path"].(string); ok && v != "" {
					cfg.Installation.OcpasswdFilePath = v
				}
			}
		}
	}

	return cfg, nil
}

// LoadStrict loads config from disk and returns an error if the file does not
// exist. Use this for write operations where you need to ensure you are
// modifying an existing file rather than silently creating one from defaults.
func LoadStrict(path string) (Config, error) {
	cfg := Defaults()
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read config file %s: %w", path, err)
	}
	if err := unmarshalTOML(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse config file %s: %w", path, err)
	}

	// Backward compatibility: if OcpasswdFilePath is empty, try the legacy
	// ocpasswd_path TOML key that was renamed in v2.
	if cfg.Installation.OcpasswdFilePath == "" {
		var raw map[string]any
		if err := unmarshalTOMLRaw(data, &raw); err == nil {
			if inst, ok := raw["installation"].(map[string]any); ok {
				if v, ok := inst["ocpasswd_path"].(string); ok && v != "" {
					cfg.Installation.OcpasswdFilePath = v
				}
			}
		}
	}

	return cfg, nil
}

// Save writes a Config to a TOML file with atomic write and backup.
func Save(cfg Config, path string) error {
	data, err := marshalTOML(cfg)
	if err != nil {
		return fmt.Errorf("marshal config for %s: %w", path, err)
	}
	// Backup existing file
	if _, statErr := os.Stat(path); statErr == nil {
		backup := path + ".bak"
		existing, readErr := os.ReadFile(path)
		if readErr != nil {
			return fmt.Errorf("read existing config %s for backup: %w", path, readErr)
		}
		if writeErr := os.WriteFile(backup, existing, 0600); writeErr != nil {
			return fmt.Errorf("write config backup %s: %w", backup, writeErr)
		}
	}
	// Atomic write: temp file + rename
	dir := filepath.Dir(path)
	tmp, tmpErr := os.CreateTemp(dir, "config-*.tmp")
	if tmpErr != nil {
		return fmt.Errorf("create temp file in %s: %w", dir, tmpErr)
	}
	tmpName := tmp.Name()
	if _, writeErr := tmp.Write(data); writeErr != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("write temp config %s: %w", tmpName, writeErr)
	}
	if closeErr := tmp.Close(); closeErr != nil {
		os.Remove(tmpName)
		return fmt.Errorf("close temp config %s: %w", tmpName, closeErr)
	}
	if chmodErr := os.Chmod(tmpName, 0600); chmodErr != nil {
		os.Remove(tmpName)
		return fmt.Errorf("chmod temp config %s: %w", tmpName, chmodErr)
	}
	if renameErr := os.Rename(tmpName, path); renameErr != nil {
		os.Remove(tmpName)
		return fmt.Errorf("rename temp config to %s: %w", path, renameErr)
	}
	return nil
}

// Validate checks a Config for obvious errors.
func Validate(cfg Config) error {
	if cfg.Web.Port < 1 || cfg.Web.Port > 65535 {
		return fmt.Errorf("web.port must be between 1 and 65535, got %d", cfg.Web.Port)
	}
	if cfg.Database.MaxOpenConns < 1 {
		return fmt.Errorf("database.max_open_conns must be at least 1, got %d", cfg.Database.MaxOpenConns)
	}
	if cfg.Database.MaxIdleConns < 0 {
		return fmt.Errorf("database.max_idle_conns must be at least 0, got %d", cfg.Database.MaxIdleConns)
	}
	if cfg.Database.ConnMaxLifetime < 0 {
		return fmt.Errorf("database.conn_max_lifetime must be at least 0, got %d", cfg.Database.ConnMaxLifetime)
	}
	if cfg.Monitoring.DashboardRefreshInterval < 5 {
		return fmt.Errorf("monitoring.dashboard_refresh_interval must be at least 5 seconds, got %d", cfg.Monitoring.DashboardRefreshInterval)
	}
	return nil
}

// MergeEnv overrides config values from environment variables.
// Supported env vars: OCSERV_PANEL_ADDR (host:port), OCSERV_PANEL_DB_DSN,
// OCSERV_PANEL_COOKIE_SECURE (bool), OCSERV_PANEL_DATA_DIR, OCSERV_PANEL_CONFIG_PATH.
func MergeEnv(cfg *Config, getenv func(string) string) {
	if v := getenv("OCSERV_PANEL_ADDR"); v != "" {
		cfg.Web.Host = hostFromAddr(v)
		cfg.Web.Port = portFromAddr(v)
	}
	if v := getenv("OCSERV_PANEL_DB_DSN"); v != "" {
		cfg.Database.DSN = v
	}
	if v := getenv("OCSERV_PANEL_COOKIE_SECURE"); v != "" {
		cfg.Security.CSRFEnabled = isTruthy(v)
	}
	if v := getenv("OCSERV_PANEL_DATA_DIR"); v != "" {
		cfg.Installation.DataDir = v
	}
	if v := getenv("OCSERV_PANEL_CONFIG_PATH"); v != "" {
		cfg.Installation.ConfigPath = v
	}
}

func hostFromAddr(addr string) string {
	host, _, err := splitHostPort(addr)
	if err != nil {
		return addr
	}
	return host
}

func portFromAddr(addr string) int {
	_, port, err := splitHostPort(addr)
	if err != nil {
		return DefaultPort
	}
	return port
}

func isTruthy(v string) bool {
	switch v {
	case "1", "t", "true", "yes", "y", "on":
		return true
	default:
		return false
	}
}
