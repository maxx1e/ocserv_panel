export const settingDescriptions: Record<string, string> = {
  // General
  'general.app_name': 'The display name for this ocserv panel instance.',
  'general.timezone': 'Timezone used for timestamps and scheduled tasks.',
  'general.theme': 'UI color theme. "system" follows the operating system preference.',
  'general.language': 'Interface language. Currently only English is available.',

  // Installation
  'installation.config_path': 'Path to the main panel configuration file (TOML format).',
  'installation.data_dir': 'Directory for persistent application data and SQLite database.',
  'installation.log_dir': 'Directory where application log files are written.',
  'installation.ocserv_config_path': 'Path to the ocserv configuration file.',
  'installation.ocserv_socket_path': 'Path to the ocserv management socket for runtime control.',
  'installation.ocserv_bin_path': 'Path to the ocserv binary used for version detection and config validation.',
  'installation.occtl_bin_path': 'Path to the occtl binary used for session management.',
  'installation.ocpasswd_bin_path': 'Path to the ocpasswd binary used for password-based user management.',
  'installation.ocpasswd_file_path': 'Path to the ocpasswd file containing password-based user accounts.',
  'installation.cert_dir': 'Directory containing TLS certificates for VPN users.',

  // Service
  'service.name': 'Systemd service name for the panel process.',
  'service.user': 'System user under which the panel service runs.',
  'service.group': 'System group under which the panel service runs.',
  'service.start_on_boot': 'Automatically start the panel service when the system boots.',
  'service.restart_on_failure': 'Automatically restart the service if it crashes.',
  'service.maintenance_mode': 'Enable maintenance mode to block user access during upgrades.',
  'service.systemd_unit_name': 'Name of the systemd unit controlling the ocserv process (used for start/stop/restart operations).',

  // Web Access
  'web.host': 'IP address the HTTP server listens on. Use 0.0.0.0 for all interfaces.',
  'web.port': 'TCP port for the HTTP server. Must be between 1 and 65535.',
  'web.base_url': 'Full base URL used to generate links and redirects.',
  'web.reverse_proxy': 'Enable when running behind HAProxy, nginx, or another reverse proxy.',
  'web.trusted_proxies': 'Comma-separated list of trusted reverse proxy IP addresses.',
  'web.https': 'Enable HTTPS with TLS certificate for the web server.',

  // Security
  'security.allowed_admin_ips': 'Comma-separated list of IP addresses allowed to access the admin panel. Empty allows all.',
  'security.tls_cert_path': 'Path to the TLS certificate file for HTTPS connections.',
  'security.tls_key_path': 'Path to the TLS private key file for HTTPS connections.',
  'security.encryption_key_path': 'Path to the encryption key used for protecting sensitive data at rest.',
  'security.csrf_enabled': 'Enable Cross-Site Request Forgery protection for all state-changing requests.',
  'security.security_headers': 'Add standard security headers (X-Frame-Options, CSP, etc.) to responses.',

  // Database
  'database.dsn': 'SQLite database connection string or file path.',
  'database.max_open_conns': 'Maximum number of simultaneous open database connections.',
  'database.max_idle_conns': 'Maximum number of idle database connections kept in the pool.',
  'database.conn_max_lifetime': 'Maximum time in seconds a database connection may be reused.',

  // Users & Permissions
  'users.default_role': 'Role assigned to newly registered users.',
  'users.allow_self_registration': 'Allow new users to register accounts without an invitation.',
  'users.max_users': 'Maximum number of user accounts. Set to 0 for unlimited.',
  'users.auto_disable_expired_certs': 'Automatically disable user accounts when their certificate expires.',

  // Password Policy
  'password_policy.min_length': 'Minimum number of characters required for a password.',
  'password_policy.require_uppercase': 'Require at least one uppercase letter in passwords.',
  'password_policy.require_lowercase': 'Require at least one lowercase letter in passwords.',
  'password_policy.require_digit': 'Require at least one digit in passwords.',
  'password_policy.require_special': 'Require at least one special character in passwords.',
  'password_policy.expiry_days': 'Number of days before passwords expire. Set to 0 for no expiry.',
  'password_policy.reuse_count': 'Number of previous passwords to prevent from being reused.',

  // Logs & Audit
  'logs.level': 'Minimum log level. Lower levels produce more output.',
  'logs.retention_days': 'Number of days to keep application log files before automatic cleanup.',
  'logs.audit_enabled': 'Record administrative actions to the audit log for traceability.',
  'logs.audit_retention_days': 'Number of days to keep audit log entries before automatic cleanup.',

  // Monitoring
  'monitoring.health_endpoint': 'Enable the /health HTTP endpoint for uptime checks.',
  'monitoring.metrics_endpoint': 'Enable the /metrics HTTP endpoint for monitoring dashboards.',
  'monitoring.dashboard_refresh_interval': 'How often (in seconds) the dashboard auto-refreshes its data.',
  'monitoring.log_file_read_enabled': 'Allow the panel to read log files for display in the dashboard.',
  'monitoring.network_interface': 'Network interface for bandwidth metrics. Leave empty for auto-discovery.',
  'monitoring.vpn_interface': 'VPN interface for VPN bandwidth metrics. Leave empty for auto-discovery (reads ocserv config).',
  'monitoring.collector_interval': 'How often (in seconds) the backend collects system and VPN metrics.',

  // Backup & Restore
  'backup.schedule': 'How often automatic database backups are created.',
  'backup.destination': 'Directory path where backup files are stored.',
  'backup.retention_days': 'Number of days to keep backup files before automatic cleanup.',
  'backup.encryption': 'Encrypt backup files using the application encryption key.',

  // Notifications
  'notifications.enabled': 'Enable outgoing webhook notifications for system events.',
  'notifications.events': 'Select which system events trigger a notification.',
  'notifications.method': 'Method used to send notifications. Currently only webhooks are supported.',
  'notifications.webhook_url': 'URL to receive webhook POST requests when selected events occur.',
};
