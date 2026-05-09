package app

// SettingType defines the UI input type for a setting.
type SettingType string

const (
	SettingTypeText          SettingType = "text"
	SettingTypeNumber        SettingType = "number"
	SettingTypeToggle        SettingType = "toggle"
	SettingTypeSelect        SettingType = "select"
	SettingTypePassword      SettingType = "password"
	SettingTypeMultiCheckbox SettingType = "multi-checkbox"
	SettingTypeReadonly      SettingType = "readonly"
	SettingTypeButton        SettingType = "button"
)

// SettingSource defines where a setting's value is stored.
type SettingSource string

const (
	SettingSourceConfig   SettingSource = "config"
	SettingSourceDatabase SettingSource = "database"
)

// SettingDef describes a single setting's metadata and current value.
type SettingDef struct {
	Key             string        `json:"key"`
	Label           string        `json:"label"`
	Section         string        `json:"section"`
	Type            SettingType   `json:"type"`
	Source          SettingSource `json:"source"`
	RequiresRestart bool          `json:"requires_restart"`
	Default         string        `json:"default"`
	Value           string        `json:"value"`
	Options         []string      `json:"options,omitempty"`
	Validation      string        `json:"validation,omitempty"`
	SortOrder       int           `json:"sort_order"`
}

// SectionDef describes a settings section.
type SectionDef struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
}

// AllSettingDefs returns the complete registry of setting definitions.
func AllSettingDefs() []SettingDef {
	return []SettingDef{
		// General
		{Key: "general.app_name", Label: "Application Name", Section: "general", Type: SettingTypeText, Source: SettingSourceConfig, RequiresRestart: true, Default: "ocserv Panel", SortOrder: 1},
		{Key: "general.timezone", Label: "Timezone", Section: "general", Type: SettingTypeSelect, Source: SettingSourceConfig, RequiresRestart: true, Default: "UTC", Options: []string{"UTC", "US/Eastern", "US/Central", "US/Mountain", "US/Pacific", "Europe/London", "Europe/Berlin", "Europe/Paris", "Asia/Tokyo", "Asia/Shanghai"}, SortOrder: 2},
		{Key: "general.theme", Label: "Theme", Section: "general", Type: SettingTypeSelect, Source: SettingSourceDatabase, RequiresRestart: false, Default: "system", Options: []string{"system", "light", "dark"}, SortOrder: 3},
		{Key: "general.language", Label: "Language", Section: "general", Type: SettingTypeSelect, Source: SettingSourceDatabase, RequiresRestart: false, Default: "en", Options: []string{"en"}, SortOrder: 4},

		// Installation
		{Key: "installation.config_path", Label: "Config Path", Section: "installation", Type: SettingTypeText, Source: SettingSourceConfig, RequiresRestart: true, Default: "/etc/ocserv-panel/config.toml", SortOrder: 1},
		{Key: "installation.data_dir", Label: "Data Directory", Section: "installation", Type: SettingTypeText, Source: SettingSourceConfig, RequiresRestart: true, Default: "/var/lib/ocserv-panel", SortOrder: 2},
		{Key: "installation.log_dir", Label: "Log Directory", Section: "installation", Type: SettingTypeText, Source: SettingSourceConfig, RequiresRestart: true, Default: "/var/log/ocserv-panel", SortOrder: 3},
		{Key: "installation.ocserv_config_path", Label: "ocserv Config Path", Section: "installation", Type: SettingTypeText, Source: SettingSourceConfig, RequiresRestart: true, Default: "/etc/ocserv/ocserv.conf", SortOrder: 4},
		{Key: "installation.ocserv_socket_path", Label: "occtl Socket Path", Section: "installation", Type: SettingTypeText, Source: SettingSourceConfig, RequiresRestart: true, Default: "", SortOrder: 5},
		{Key: "installation.ocserv_bin_path", Label: "ocserv Binary Path", Section: "installation", Type: SettingTypeText, Source: SettingSourceConfig, RequiresRestart: true, Default: "/usr/local/sbin/ocserv", SortOrder: 6},
		{Key: "installation.occtl_bin_path", Label: "occtl Binary Path", Section: "installation", Type: SettingTypeText, Source: SettingSourceConfig, RequiresRestart: true, Default: "/usr/local/bin/occtl", SortOrder: 7},
		{Key: "installation.ocpasswd_bin_path", Label: "ocpasswd Binary Path", Section: "installation", Type: SettingTypeText, Source: SettingSourceConfig, RequiresRestart: true, Default: "/usr/local/bin/ocpasswd", SortOrder: 8},
		{Key: "installation.ocpasswd_file_path", Label: "ocpasswd Password File", Section: "installation", Type: SettingTypeText, Source: SettingSourceConfig, RequiresRestart: true, Default: "/etc/ocserv/ocpasswd", SortOrder: 9},
		{Key: "installation.cert_dir", Label: "Certificate Directory", Section: "installation", Type: SettingTypeText, Source: SettingSourceConfig, RequiresRestart: true, Default: "/etc/ocserv/certs", SortOrder: 10},
		{Key: "installation.ocserv_log_path", Label: "ocserv Log Path", Section: "installation", Type: SettingTypeText, Source: SettingSourceConfig, RequiresRestart: false, Default: "/var/log/ocserv/ocserv.log", SortOrder: 11},
		{Key: "installation.panel_log_path", Label: "Panel Log Path", Section: "installation", Type: SettingTypeText, Source: SettingSourceConfig, RequiresRestart: false, Default: "/var/log/ocserv-panel/panel.log", SortOrder: 12},

		// Service
		{Key: "service.name", Label: "Service Name", Section: "service", Type: SettingTypeText, Source: SettingSourceConfig, RequiresRestart: true, Default: "ocserv-panel", SortOrder: 1},
		{Key: "service.user", Label: "Run as User", Section: "service", Type: SettingTypeText, Source: SettingSourceConfig, RequiresRestart: true, Default: "ocserv-panel", SortOrder: 2},
		{Key: "service.group", Label: "Run as Group", Section: "service", Type: SettingTypeText, Source: SettingSourceConfig, RequiresRestart: true, Default: "ocserv-panel", SortOrder: 3},
		{Key: "service.start_on_boot", Label: "Start on Boot", Section: "service", Type: SettingTypeToggle, Source: SettingSourceConfig, RequiresRestart: true, Default: "true", SortOrder: 4},
		{Key: "service.restart_on_failure", Label: "Restart on Failure", Section: "service", Type: SettingTypeToggle, Source: SettingSourceDatabase, RequiresRestart: false, Default: "true", SortOrder: 5},
		{Key: "service.maintenance_mode", Label: "Maintenance Mode", Section: "service", Type: SettingTypeToggle, Source: SettingSourceConfig, RequiresRestart: true, Default: "false", SortOrder: 6},
		{Key: "service.systemd_unit_name", Label: "Systemd Unit Name", Section: "service", Type: SettingTypeText, Source: SettingSourceConfig, RequiresRestart: true, Default: "ocserv", SortOrder: 7},

		// Web Access
		{Key: "web.host", Label: "Listen Host", Section: "web", Type: SettingTypeText, Source: SettingSourceConfig, RequiresRestart: true, Default: "127.0.0.1", SortOrder: 1},
		{Key: "web.port", Label: "Listen Port", Section: "web", Type: SettingTypeNumber, Source: SettingSourceConfig, RequiresRestart: true, Default: "8080", Validation: "min:1,max:65535", SortOrder: 2},
		{Key: "web.base_url", Label: "Base URL", Section: "web", Type: SettingTypeText, Source: SettingSourceConfig, RequiresRestart: true, Default: "http://localhost:8080", SortOrder: 3},
		{Key: "web.reverse_proxy", Label: "Behind Reverse Proxy", Section: "web", Type: SettingTypeToggle, Source: SettingSourceConfig, RequiresRestart: true, Default: "false", SortOrder: 4},
		{Key: "web.trusted_proxies", Label: "Trusted Proxies", Section: "web", Type: SettingTypeText, Source: SettingSourceConfig, RequiresRestart: true, Default: "", SortOrder: 5},
		{Key: "web.https", Label: "Enable HTTPS", Section: "web", Type: SettingTypeToggle, Source: SettingSourceConfig, RequiresRestart: true, Default: "false", SortOrder: 6},

		// Security
		{Key: "security.allowed_admin_ips", Label: "Allowed Admin IPs", Section: "security", Type: SettingTypeText, Source: SettingSourceConfig, RequiresRestart: true, Default: "", SortOrder: 1},
		{Key: "security.tls_cert_path", Label: "TLS Certificate Path", Section: "security", Type: SettingTypePassword, Source: SettingSourceConfig, RequiresRestart: true, Default: "", SortOrder: 2},
		{Key: "security.tls_key_path", Label: "TLS Key Path", Section: "security", Type: SettingTypePassword, Source: SettingSourceConfig, RequiresRestart: true, Default: "", SortOrder: 3},
		{Key: "security.encryption_key_path", Label: "Encryption Key Path", Section: "security", Type: SettingTypePassword, Source: SettingSourceConfig, RequiresRestart: true, Default: "", SortOrder: 4},
		{Key: "security.csrf_enabled", Label: "Enable CSRF Protection", Section: "security", Type: SettingTypeToggle, Source: SettingSourceConfig, RequiresRestart: true, Default: "true", SortOrder: 5},
		{Key: "security.security_headers", Label: "Security Headers", Section: "security", Type: SettingTypeToggle, Source: SettingSourceConfig, RequiresRestart: true, Default: "true", SortOrder: 6},

		// Database
		{Key: "database.dsn", Label: "Database DSN", Section: "database", Type: SettingTypePassword, Source: SettingSourceConfig, RequiresRestart: true, Default: "ocserv-panel.db", SortOrder: 1},
		{Key: "database.max_open_conns", Label: "Max Open Connections", Section: "database", Type: SettingTypeNumber, Source: SettingSourceConfig, RequiresRestart: true, Default: "5", Validation: "min:1", SortOrder: 2},
		{Key: "database.max_idle_conns", Label: "Max Idle Connections", Section: "database", Type: SettingTypeNumber, Source: SettingSourceConfig, RequiresRestart: true, Default: "2", Validation: "min:0", SortOrder: 3},
		{Key: "database.conn_max_lifetime", Label: "Connection Max Lifetime (s)", Section: "database", Type: SettingTypeNumber, Source: SettingSourceConfig, RequiresRestart: true, Default: "300", Validation: "min:0", SortOrder: 4},

		// Users & Permissions
		{Key: "users.default_role", Label: "Default User Role", Section: "users", Type: SettingTypeSelect, Source: SettingSourceDatabase, RequiresRestart: false, Default: "viewer", Options: []string{"viewer", "owner"}, SortOrder: 1},
		{Key: "users.allow_self_registration", Label: "Allow Self-Registration", Section: "users", Type: SettingTypeToggle, Source: SettingSourceDatabase, RequiresRestart: false, Default: "false", SortOrder: 2},
		{Key: "users.max_users", Label: "Max Users (0 = unlimited)", Section: "users", Type: SettingTypeNumber, Source: SettingSourceDatabase, RequiresRestart: false, Default: "0", SortOrder: 3},
		{Key: "users.auto_disable_expired_certs", Label: "Auto-Disable Expired Certs", Section: "users", Type: SettingTypeToggle, Source: SettingSourceDatabase, RequiresRestart: false, Default: "true", SortOrder: 4},

		// Password Policy
		{Key: "password_policy.min_length", Label: "Minimum Length", Section: "password_policy", Type: SettingTypeNumber, Source: SettingSourceDatabase, RequiresRestart: false, Default: "8", Validation: "min:6,max:128", SortOrder: 1},
		{Key: "password_policy.require_uppercase", Label: "Require Uppercase", Section: "password_policy", Type: SettingTypeToggle, Source: SettingSourceDatabase, RequiresRestart: false, Default: "true", SortOrder: 2},
		{Key: "password_policy.require_lowercase", Label: "Require Lowercase", Section: "password_policy", Type: SettingTypeToggle, Source: SettingSourceDatabase, RequiresRestart: false, Default: "true", SortOrder: 3},
		{Key: "password_policy.require_digit", Label: "Require Digit", Section: "password_policy", Type: SettingTypeToggle, Source: SettingSourceDatabase, RequiresRestart: false, Default: "true", SortOrder: 4},
		{Key: "password_policy.require_special", Label: "Require Special Char", Section: "password_policy", Type: SettingTypeToggle, Source: SettingSourceDatabase, RequiresRestart: false, Default: "false", SortOrder: 5},
		{Key: "password_policy.expiry_days", Label: "Password Expiry (days, 0 = never)", Section: "password_policy", Type: SettingTypeNumber, Source: SettingSourceDatabase, RequiresRestart: false, Default: "0", SortOrder: 6},
		{Key: "password_policy.reuse_count", Label: "Prevent Reuse Count", Section: "password_policy", Type: SettingTypeNumber, Source: SettingSourceDatabase, RequiresRestart: false, Default: "3", SortOrder: 7},

		// Logs & Audit
		{Key: "logs.level", Label: "Log Level", Section: "logs", Type: SettingTypeSelect, Source: SettingSourceDatabase, RequiresRestart: false, Default: "info", Options: []string{"debug", "info", "warn", "error"}, SortOrder: 1},
		{Key: "logs.retention_days", Label: "Log Retention (days)", Section: "logs", Type: SettingTypeNumber, Source: SettingSourceDatabase, RequiresRestart: false, Default: "30", SortOrder: 2},
		{Key: "logs.audit_enabled", Label: "Enable Audit Logging", Section: "logs", Type: SettingTypeToggle, Source: SettingSourceDatabase, RequiresRestart: false, Default: "true", SortOrder: 3},
		{Key: "logs.audit_retention_days", Label: "Audit Retention (days)", Section: "logs", Type: SettingTypeNumber, Source: SettingSourceDatabase, RequiresRestart: false, Default: "90", SortOrder: 4},

		// Monitoring
		{Key: "monitoring.health_endpoint", Label: "Health Check Endpoint", Section: "monitoring", Type: SettingTypeToggle, Source: SettingSourceConfig, RequiresRestart: true, Default: "true", SortOrder: 1},
		{Key: "monitoring.metrics_endpoint", Label: "Metrics Endpoint", Section: "monitoring", Type: SettingTypeToggle, Source: SettingSourceConfig, RequiresRestart: true, Default: "false", SortOrder: 2},
		{Key: "monitoring.dashboard_refresh_interval", Label: "Dashboard Refresh Interval (s)", Section: "monitoring", Type: SettingTypeNumber, Source: SettingSourceConfig, RequiresRestart: true, Default: "15", Validation: "min:5", SortOrder: 3},
		{Key: "monitoring.log_file_read_enabled", Label: "Enable Log File Reading", Section: "monitoring", Type: SettingTypeToggle, Source: SettingSourceConfig, RequiresRestart: false, Default: "true", SortOrder: 4},
		{Key: "monitoring.network_interface", Label: "Network Interface", Section: "monitoring", Type: SettingTypeText, Source: SettingSourceConfig, RequiresRestart: true, Default: "", SortOrder: 5},
		{Key: "monitoring.vpn_interface", Label: "VPN Interface", Section: "monitoring", Type: SettingTypeText, Source: SettingSourceConfig, RequiresRestart: true, Default: "", SortOrder: 6},
		{Key: "monitoring.collector_interval", Label: "Collector Interval (s)", Section: "monitoring", Type: SettingTypeNumber, Source: SettingSourceConfig, RequiresRestart: true, Default: "15", Validation: "min:5", SortOrder: 7},

		// Backup & Restore
		{Key: "backup.schedule", Label: "Auto-Backup Schedule", Section: "backup", Type: SettingTypeSelect, Source: SettingSourceDatabase, RequiresRestart: false, Default: "disabled", Options: []string{"disabled", "daily", "weekly", "monthly"}, SortOrder: 1},
		{Key: "backup.destination", Label: "Backup Destination", Section: "backup", Type: SettingTypeText, Source: SettingSourceDatabase, RequiresRestart: false, Default: "/var/lib/ocserv-panel/backups", SortOrder: 2},
		{Key: "backup.retention_days", Label: "Backup Retention (days)", Section: "backup", Type: SettingTypeNumber, Source: SettingSourceDatabase, RequiresRestart: false, Default: "30", SortOrder: 3},
		{Key: "backup.encryption", Label: "Encrypt Backups", Section: "backup", Type: SettingTypeToggle, Source: SettingSourceDatabase, RequiresRestart: false, Default: "true", SortOrder: 4},

		// Notifications
		{Key: "notifications.enabled", Label: "Enable Notifications", Section: "notifications", Type: SettingTypeToggle, Source: SettingSourceDatabase, RequiresRestart: false, Default: "false", SortOrder: 1},
		{Key: "notifications.events", Label: "Notification Events", Section: "notifications", Type: SettingTypeMultiCheckbox, Source: SettingSourceDatabase, RequiresRestart: false, Default: "user_created,user_deleted,session_started,session_ended,cert_expiring,config_changed,service_restarted", Options: []string{"user_created", "user_deleted", "session_started", "session_ended", "cert_expiring", "config_changed", "service_restarted"}, SortOrder: 2},
		{Key: "notifications.method", Label: "Notification Method", Section: "notifications", Type: SettingTypeSelect, Source: SettingSourceDatabase, RequiresRestart: false, Default: "webhook", Options: []string{"webhook"}, SortOrder: 3},
		{Key: "notifications.webhook_url", Label: "Webhook URL", Section: "notifications", Type: SettingTypeText, Source: SettingSourceDatabase, RequiresRestart: false, Default: "", SortOrder: 4},
	}
}

// AllSectionDefs returns the ordered list of settings sections.
func AllSectionDefs() []SectionDef {
	return []SectionDef{
		{ID: "general", Label: "General", Title: "General Settings", Description: "Application name, timezone, and appearance preferences."},
		{ID: "installation", Label: "Installation", Title: "Installation Paths", Description: "Filesystem paths for configuration, data, and certificates."},
		{ID: "service", Label: "Service", Title: "Service Configuration", Description: "Service runtime behavior and startup options."},
		{ID: "web", Label: "Web Access", Title: "Web Access Settings", Description: "HTTP/HTTPS listener and reverse proxy configuration."},
		{ID: "security", Label: "Security", Title: "Security Settings", Description: "TLS, CSRF, IP restrictions, and encryption key paths."},
		{ID: "database", Label: "Database", Title: "Database Configuration", Description: "SQLite connection and pool settings."},
		{ID: "users", Label: "Users & Permissions", Title: "Users & Permissions", Description: "Default roles, self-registration, and certificate policies."},
		{ID: "password_policy", Label: "Password Policy", Title: "Password Policy", Description: "Password complexity, expiry, and reuse rules."},
		{ID: "logs", Label: "Logs & Audit", Title: "Logs & Audit", Description: "Log levels, retention, and audit trail settings."},
		{ID: "monitoring", Label: "Monitoring", Title: "Monitoring Settings", Description: "Health checks, metrics, and dashboard refresh."},
		{ID: "backup", Label: "Backup & Restore", Title: "Backup & Restore", Description: "Automatic backup schedule, destination, and encryption."},
		{ID: "notifications", Label: "Notifications", Title: "Notifications", Description: "Webhook notifications for system events."},
	}
}

// SettingDefsBySection groups setting definitions by section ID.
func SettingDefsBySection() map[string][]SettingDef {
	defs := AllSettingDefs()
	result := make(map[string][]SettingDef, 12)
	for _, d := range defs {
		result[d.Section] = append(result[d.Section], d)
	}
	return result
}

// FindSettingDef looks up a setting definition by key.
func FindSettingDef(key string) (SettingDef, bool) {
	for _, d := range AllSettingDefs() {
		if d.Key == key {
			return d, true
		}
	}
	return SettingDef{}, false
}
