-- Settings registry: stores metadata for all known settings.
-- This table is seeded on migration and used by the backend to build the settings UI.
-- Runtime values for database-sourced settings are stored in app_settings.

CREATE TABLE IF NOT EXISTS setting_definitions (
    key TEXT PRIMARY KEY,
    label TEXT NOT NULL,
    section TEXT NOT NULL,
    type TEXT NOT NULL CHECK(type IN ('text','number','toggle','select','password','multi-checkbox','readonly','button')),
    source TEXT NOT NULL CHECK(source IN ('config','database')),
    requires_restart INTEGER NOT NULL DEFAULT 0,
    default_value TEXT NOT NULL DEFAULT '',
    options TEXT,
    validation TEXT,
    sort_order INTEGER NOT NULL DEFAULT 0
);

-- Seed all setting definitions.
-- These rows are inserted with OR IGNORE so they don't conflict on re-migration.
INSERT OR IGNORE INTO setting_definitions (key, label, section, type, source, requires_restart, default_value, options, validation, sort_order) VALUES
-- General
('general.app_name', 'Application Name', 'general', 'text', 'config', 1, 'ocserv Panel', NULL, NULL, 1),
('general.timezone', 'Timezone', 'general', 'select', 'config', 1, 'UTC', '["UTC","US/Eastern","US/Central","US/Mountain","US/Pacific","Europe/London","Europe/Berlin","Europe/Paris","Asia/Tokyo","Asia/Shanghai"]', NULL, 2),
('general.theme', 'Theme', 'general', 'select', 'database', 0, 'system', '["system","light","dark"]', NULL, 3),
('general.language', 'Language', 'general', 'select', 'database', 0, 'en', '["en"]', NULL, 4),
-- Installation
('installation.config_path', 'Config Path', 'installation', 'text', 'config', 1, '/etc/ocserv-panel/config.toml', NULL, NULL, 1),
('installation.data_dir', 'Data Directory', 'installation', 'text', 'config', 1, '/var/lib/ocserv-panel', NULL, NULL, 2),
('installation.log_dir', 'Log Directory', 'installation', 'text', 'config', 1, '/var/log/ocserv-panel', NULL, NULL, 3),
('installation.ocserv_config_path', 'ocserv Config Path', 'installation', 'text', 'config', 1, '/etc/ocserv/ocserv.conf', NULL, NULL, 4),
('installation.ocserv_socket_path', 'ocserv Socket Path', 'installation', 'text', 'config', 1, '/var/run/ocserv-socket', NULL, NULL, 5),
('installation.ocpasswd_path', 'ocpasswd Path', 'installation', 'text', 'config', 1, '/etc/ocserv/ocpasswd', NULL, NULL, 6),
('installation.cert_dir', 'Certificate Directory', 'installation', 'text', 'config', 1, '/etc/ocserv/certs', NULL, NULL, 7),
-- Service
('service.name', 'Service Name', 'service', 'text', 'config', 1, 'ocserv-panel', NULL, NULL, 1),
('service.user', 'Run as User', 'service', 'text', 'config', 1, 'ocserv-panel', NULL, NULL, 2),
('service.group', 'Run as Group', 'service', 'text', 'config', 1, 'ocserv-panel', NULL, NULL, 3),
('service.start_on_boot', 'Start on Boot', 'service', 'toggle', 'config', 1, 'true', NULL, NULL, 4),
('service.restart_on_failure', 'Restart on Failure', 'service', 'toggle', 'database', 0, 'true', NULL, NULL, 5),
('service.maintenance_mode', 'Maintenance Mode', 'service', 'toggle', 'config', 1, 'false', NULL, NULL, 6),
-- Web Access
('web.host', 'Listen Host', 'web', 'text', 'config', 1, '127.0.0.1', NULL, NULL, 1),
('web.port', 'Listen Port', 'web', 'number', 'config', 1, '8080', NULL, 'min:1,max:65535', 2),
('web.base_url', 'Base URL', 'web', 'text', 'config', 1, 'http://localhost:8080', NULL, NULL, 3),
('web.reverse_proxy', 'Behind Reverse Proxy', 'web', 'toggle', 'config', 1, 'false', NULL, NULL, 4),
('web.trusted_proxies', 'Trusted Proxies', 'web', 'text', 'config', 1, '', NULL, NULL, 5),
('web.https', 'Enable HTTPS', 'web', 'toggle', 'config', 1, 'false', NULL, NULL, 6),
-- Security
('security.allowed_admin_ips', 'Allowed Admin IPs', 'security', 'text', 'config', 1, '', NULL, NULL, 1),
('security.tls_cert_path', 'TLS Certificate Path', 'security', 'password', 'config', 1, '', NULL, NULL, 2),
('security.tls_key_path', 'TLS Key Path', 'security', 'password', 'config', 1, '', NULL, NULL, 3),
('security.encryption_key_path', 'Encryption Key Path', 'security', 'password', 'config', 1, '', NULL, NULL, 4),
('security.csrf_enabled', 'Enable CSRF Protection', 'security', 'toggle', 'config', 1, 'true', NULL, NULL, 5),
('security.security_headers', 'Security Headers', 'security', 'toggle', 'config', 1, 'true', NULL, NULL, 6),
-- Database
('database.dsn', 'Database DSN', 'database', 'password', 'config', 1, 'ocserv-panel.db', NULL, NULL, 1),
('database.max_open_conns', 'Max Open Connections', 'database', 'number', 'config', 1, '5', NULL, 'min:1', 2),
('database.max_idle_conns', 'Max Idle Connections', 'database', 'number', 'config', 1, '2', NULL, 'min:0', 3),
('database.conn_max_lifetime', 'Connection Max Lifetime (s)', 'database', 'number', 'config', 1, '300', NULL, 'min:0', 4),
-- Users & Permissions
('users.default_role', 'Default User Role', 'users', 'select', 'database', 0, 'viewer', '["viewer","owner"]', NULL, 1),
('users.allow_self_registration', 'Allow Self-Registration', 'users', 'toggle', 'database', 0, 'false', NULL, NULL, 2),
('users.max_users', 'Max Users (0 = unlimited)', 'users', 'number', 'database', 0, '0', NULL, NULL, 3),
('users.auto_disable_expired_certs', 'Auto-Disable Expired Certs', 'users', 'toggle', 'database', 0, 'true', NULL, NULL, 4),
-- Password Policy
('password_policy.min_length', 'Minimum Length', 'password_policy', 'number', 'database', 0, '8', NULL, 'min:6,max:128', 1),
('password_policy.require_uppercase', 'Require Uppercase', 'password_policy', 'toggle', 'database', 0, 'true', NULL, NULL, 2),
('password_policy.require_lowercase', 'Require Lowercase', 'password_policy', 'toggle', 'database', 0, 'true', NULL, NULL, 3),
('password_policy.require_digit', 'Require Digit', 'password_policy', 'toggle', 'database', 0, 'true', NULL, NULL, 4),
('password_policy.require_special', 'Require Special Char', 'password_policy', 'toggle', 'database', 0, 'false', NULL, NULL, 5),
('password_policy.expiry_days', 'Password Expiry (days, 0 = never)', 'password_policy', 'number', 'database', 0, '0', NULL, NULL, 6),
('password_policy.reuse_count', 'Prevent Reuse Count', 'password_policy', 'number', 'database', 0, '3', NULL, NULL, 7),
-- Logs & Audit
('logs.level', 'Log Level', 'logs', 'select', 'database', 0, 'info', '["debug","info","warn","error"]', NULL, 1),
('logs.retention_days', 'Log Retention (days)', 'logs', 'number', 'database', 0, '30', NULL, NULL, 2),
('logs.audit_enabled', 'Enable Audit Logging', 'logs', 'toggle', 'database', 0, 'true', NULL, NULL, 3),
('logs.audit_retention_days', 'Audit Retention (days)', 'logs', 'number', 'database', 0, '90', NULL, NULL, 4),
-- Monitoring
('monitoring.health_endpoint', 'Health Check Endpoint', 'monitoring', 'toggle', 'config', 1, 'true', NULL, NULL, 1),
('monitoring.metrics_endpoint', 'Metrics Endpoint', 'monitoring', 'toggle', 'config', 1, 'false', NULL, NULL, 2),
('monitoring.dashboard_refresh_interval', 'Dashboard Refresh Interval (s)', 'monitoring', 'number', 'config', 1, '15', NULL, 'min:5', 3),
-- Backup & Restore
('backup.schedule', 'Auto-Backup Schedule', 'backup', 'select', 'database', 0, 'disabled', '["disabled","daily","weekly","monthly"]', NULL, 1),
('backup.destination', 'Backup Destination', 'backup', 'text', 'database', 0, '/var/lib/ocserv-panel/backups', NULL, NULL, 2),
('backup.retention_days', 'Backup Retention (days)', 'backup', 'number', 'database', 0, '30', NULL, NULL, 3),
('backup.encryption', 'Encrypt Backups', 'backup', 'toggle', 'database', 0, 'true', NULL, NULL, 4),
-- Notifications
('notifications.enabled', 'Enable Notifications', 'notifications', 'toggle', 'database', 0, 'false', NULL, NULL, 1),
('notifications.events', 'Notification Events', 'notifications', 'multi-checkbox', 'database', 0, 'user_created,user_deleted,session_started,session_ended,cert_expiring,config_changed,service_restarted', '["user_created","user_deleted","session_started","session_ended","cert_expiring","config_changed","service_restarted"]', NULL, 2),
('notifications.method', 'Notification Method', 'notifications', 'select', 'database', 0, 'webhook', '["webhook"]', NULL, 3),
('notifications.webhook_url', 'Webhook URL', 'notifications', 'text', 'database', 0, '', NULL, NULL, 4);
