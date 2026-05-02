# Settings Tab Design

## Overview

A comprehensive Settings page that allows administrators to view and modify all application configuration through a unified web UI. Settings are split between deployment-level configuration (stored in a TOML config file, requiring service restart) and application-level preferences (stored in SQLite, taking immediate effect).

### Goals

- Replace the current read-only Web UI transport info display with a full settings management interface
- Provide a single unified API that abstracts over both TOML config and SQLite app_settings
- Make it clear which settings require a restart and which take effect immediately
- Support all 12 settings sections in a single implementation cycle

### Non-Goals

- Multi-node configuration management
- External configuration services (Consul, etcd, etc.)
- Hot-reloading of deployment-level settings without restart
- Configuration version control beyond backup/restore

## Architecture

### Approach: TOML + SQLite Hybrid with Unified API

Settings come from two sources:

1. **TOML config file** (`/etc/ocserv-panel/config.toml`, overridable via `OCSERV_PANEL_CONFIG_PATH`) — deployment-level settings that require a service restart to take effect
2. **SQLite `app_settings` table** — application-level settings that take effect immediately

The backend exposes a unified `/api/settings` API that abstracts over both stores. Each setting has metadata indicating its source and whether it requires a restart.

**Precedence:** env vars > TOML config > SQLite defaults > hardcoded defaults

## Settings Sections

### Section 1: General

| Setting | Key | Type | Source | Restart | Default |
|---------|-----|------|--------|---------|---------|
| Application Name | `general.app_name` | text | config | yes | `ocserv Panel` |
| Timezone | `general.timezone` | select | config | yes | `UTC` |
| Theme | `general.theme` | select | database | no | `system` |
| Language | `general.language` | select | database | no | `en` |

### Section 2: Installation

| Setting | Key | Type | Source | Restart | Default |
|---------|-----|------|--------|---------|---------|
| Config Path | `installation.config_path` | text | config | yes | `/etc/ocserv-panel/config.toml` |
| Data Directory | `installation.data_dir` | text | config | yes | `/var/lib/ocserv-panel` |
| Log Directory | `installation.log_dir` | text | config | yes | `/var/log/ocserv-panel` |
| ocserv Config Path | `installation.ocserv_config_path` | text | config | yes | `/etc/ocserv/ocserv.conf` |
| ocserv Socket Path | `installation.ocserv_socket_path` | text | config | yes | `/var/run/ocserv-socket` |
| ocpasswd Path | `installation.ocpasswd_path` | text | config | yes | `/etc/ocserv/ocpasswd` |
| Certificate Directory | `installation.cert_dir` | text | config | yes | `/etc/ocserv/certs` |

### Section 3: Service

| Setting | Key | Type | Source | Restart | Default |
|---------|-----|------|--------|---------|---------|
| Service Name | `service.name` | text | config | yes | `ocserv-panel` |
| Run as User | `service.user` | text | config | yes | `ocserv-panel` |
| Run as Group | `service.group` | text | config | yes | `ocserv-panel` |
| Start on Boot | `service.start_on_boot` | toggle | config | yes | `true` |
| Restart on Failure | `service.restart_on_failure` | toggle | database | no | `true` |
| Maintenance Mode | `service.maintenance_mode` | toggle | config | yes | `false` |

### Section 4: Web Access

| Setting | Key | Type | Source | Restart | Default |
|---------|-----|------|--------|---------|---------|
| Listen Host | `web.host` | text | config | yes | `127.0.0.1` |
| Listen Port | `web.port` | number | config | yes | `8080` |
| Base URL | `web.base_url` | text | config | yes | `http://localhost:8080` |
| Behind Reverse Proxy | `web.reverse_proxy` | toggle | config | yes | `false` |
| Trusted Proxies | `web.trusted_proxies` | text | config | yes | `` |
| Enable HTTPS | `web.https` | toggle | config | yes | `false` |

### Section 5: Security

| Setting | Key | Type | Source | Restart | Default |
|---------|-----|------|--------|---------|---------|
| Allowed Admin IPs | `security.allowed_admin_ips` | text | config | yes | `` (all) |
| TLS Certificate Path | `security.tls_cert_path` | text | config | yes | `` |
| TLS Key Path | `security.tls_key_path` | text | config | yes | `` |
| Encryption Key Path | `security.encryption_key_path` | text | config | yes | `` |
| Enable CSRF Protection | `security.csrf_enabled` | toggle | config | yes | `true` |
| Security Headers | `security.security_headers` | toggle | config | yes | `true` |

### Section 6: Database

| Setting | Key | Type | Source | Restart | Default |
|---------|-----|------|--------|---------|---------|
| Database DSN | `database.dsn` | text | config | yes | `ocserv-panel.db` |
| Max Open Connections | `database.max_open_conns` | number | config | yes | `5` |
| Max Idle Connections | `database.max_idle_conns` | number | config | yes | `2` |
| Connection Max Lifetime (s) | `database.conn_max_lifetime` | number | config | yes | `300` |

### Section 7: Users & Permissions

| Setting | Key | Type | Source | Restart | Default |
|---------|-----|------|--------|---------|---------|
| Default User Role | `users.default_role` | select | database | no | `viewer` |
| Allow Self-Registration | `users.allow_self_registration` | toggle | database | no | `false` |
| Max Users | `users.max_users` | number | database | no | `0` (unlimited) |
| Auto-Disable Expired Certs | `users.auto_disable_expired_certs` | toggle | database | no | `true` |

### Section 8: Password Policy

| Setting | Key | Type | Source | Restart | Default |
|---------|-----|------|--------|---------|---------|
| Minimum Length | `password_policy.min_length` | number | database | no | `8` |
| Require Uppercase | `password_policy.require_uppercase` | toggle | database | no | `true` |
| Require Lowercase | `password_policy.require_lowercase` | toggle | database | no | `true` |
| Require Digit | `password_policy.require_digit` | toggle | database | no | `true` |
| Require Special Char | `password_policy.require_special` | toggle | database | no | `false` |
| Password Expiry (days) | `password_policy.expiry_days` | number | database | no | `0` (never) |
| Prevent Reuse Count | `password_policy.reuse_count` | number | database | no | `3` |

### Section 9: Logs & Audit

| Setting | Key | Type | Source | Restart | Default |
|---------|-----|------|--------|---------|---------|
| Log Level | `logs.level` | select | database | no | `info` |
| Log Retention (days) | `logs.retention_days` | number | database | no | `30` |
| Enable Audit Logging | `logs.audit_enabled` | toggle | database | no | `true` |
| Audit Retention (days) | `logs.audit_retention_days` | number | database | no | `90` |

### Section 10: Monitoring

| Setting | Key | Type | Source | Restart | Default |
|---------|-----|------|--------|---------|---------|
| Health Check Endpoint | `monitoring.health_endpoint` | toggle | config | yes | `true` |
| Metrics Endpoint | `monitoring.metrics_endpoint` | toggle | config | yes | `false` |
| Dashboard Refresh Interval (s) | `monitoring.dashboard_refresh_interval` | number | config | yes | `15` |

### Section 11: Backup & Restore

| Setting | Key | Type | Source | Restart | Default |
|---------|-----|------|--------|---------|---------|
| Auto-Backup Schedule | `backup.schedule` | select | database | no | `disabled` |
| Backup Destination | `backup.destination` | text | database | no | `/var/lib/ocserv-panel/backups` |
| Backup Retention (days) | `backup.retention_days` | number | database | no | `30` |
| Encrypt Backups | `backup.encryption` | toggle | database | no | `true` |

Actions: Trigger Backup Now (button), Restore from Backup (button with file upload)

### Section 12: Notifications

| Setting | Key | Type | Source | Restart | Default |
|---------|-----|------|--------|---------|---------|
| Enable Notifications | `notifications.enabled` | toggle | database | no | `false` |
| Notification Events | `notifications.events` | multi-checkbox | database | no | (all) |
| Notification Method | `notifications.method` | select | database | no | `webhook` |
| Webhook URL | `notifications.webhook_url` | text | database | no | `` |

Notification events options: `user_created`, `user_deleted`, `session_started`, `session_ended`, `cert_expiring`, `config_changed`, `service_restarted`

## Backend Design

### New Package: `internal/config`

Responsible for reading, writing, and validating the TOML configuration file.

```go
type Config struct {
    General     GeneralConfig     `toml:"general"`
    Installation InstallationConfig `toml:"installation"`
    Service     ServiceConfig     `toml:"service"`
    Web         WebConfig         `toml:"web"`
    Security    SecurityConfig    `toml:"security"`
    Database    DatabaseConfig    `toml:"database"`
    Monitoring  MonitoringConfig  `toml:"monitoring"`
}
```

Key functions:
- `Load(path string) (*Config, error)` — reads TOML file, falls back to defaults
- `Save(cfg *Config, path string) error` — atomic write (temp file + rename), creates .bak backup first, sets 0600 permissions
- `Validate(cfg *Config) error` — validates all fields before write
- `ApplyDefaults(cfg *Config)` — fills in zero values with hardcoded defaults
- `MergeEnv(cfg *Config) *Config` — overrides config values with env vars (OCSERV_PANEL_*)

### Extended SettingsService

```go
type SettingDef struct {
    Key             string
    Label           string
    Section         string
    Type            string   // text, number, toggle, select, password, multi-checkbox, readonly, button
    Source          string   // config, database
    RequiresRestart bool
    Default         string
    Options         []string // for select/multi-checkbox types
    Validation      string   // regex or validation rule
}

type SettingsService struct {
    config     *config.Config
    store      *store.Store
    registry   map[string]SettingDef
    configPath string
}
```

Key methods:
- `GetAllSections(ctx context.Context, principal auth.Principal) ([]Section, error)` — returns all sections with current values
- `UpdateSetting(ctx context.Context, principal auth.Principal, key, value string) (SettingDef, error)` — updates a setting in the appropriate store
- `RestartRequired(ctx context.Context) (bool, []string)` — checks if any config-file settings have pending changes
- `TriggerRestart(ctx context.Context, principal auth.Principal) error` — triggers service restart via systemd

### API Endpoints

| Method | Path | Permission | Description |
|--------|------|------------|-------------|
| GET | `/api/settings` | `settings:read` | Returns all sections with current values |
| PUT | `/api/settings/:key` | `settings:manage` | Updates a single setting |
| GET | `/api/settings/restart-status` | `settings:read` | Checks if restart is required |
| POST | `/api/settings/restart` | `settings:restart` | Triggers service restart |
| GET | `/api/settings/export/logs` | `settings:export` | Downloads log file |
| GET | `/api/settings/export/audit` | `settings:export` | Downloads audit log |
| POST | `/api/settings/backup` | `settings:backup` | Triggers a backup |
| POST | `/api/settings/restore` | `settings:backup` | Restores from uploaded backup |

### RBAC Permissions

New granular permissions:
- `settings:read` — view all settings (owner, viewer)
- `settings:manage` — modify settings (owner only)
- `settings:restart` — trigger service restart (owner only)
- `settings:export` — export logs and audit data (owner only)
- `settings:backup` — trigger backup and restore (owner only)

### Audit Events

All setting changes produce audit records:
- `setting.update` — key, old value, new value, source
- `setting.restart` — triggered by, timestamp
- `setting.export` — type (logs/audit), timestamp
- `setting.backup` — triggered by, timestamp
- `setting.restore` — triggered by, timestamp

### TOML File Safety

When writing the TOML config file:
1. Validate the new config against all validation rules
2. Copy current file to `.bak` (preserving one backup)
3. Write to a temp file in the same directory
4. Rename temp file to the config file (atomic on same filesystem)
5. Set file permissions to 0600 (owner read/write only)

## Frontend Design

### Component Hierarchy

```
+page.svelte
├── SettingsNav.svelte          (sticky anchor navigation)
├── RestartBanner.svelte        (shown when restart required)
├── SettingsSection.svelte      (reusable section wrapper)
│   ├── SettingField.svelte     (reusable field renderer)
│   └── SettingToggle.svelte    (toggle switch component)
├── PasswordPolicySection.svelte
├── BackupRestoreSection.svelte (with action buttons)
└── ExportButtons.svelte         (log/audit export)
```

### SettingsNav

Horizontal scrollable pill bar at the top. Each pill links to its section via anchor `href="#general"`. Active section highlighted via `IntersectionObserver`.

### RestartBanner

Persistent `callout.warning` that appears when any config-file setting has been changed. Shows "Restart required" message with a "Restart Now" button. After triggering restart, shows "Restarting… the page will reload when the service is back." Polls `getRestartStatus()` every 30 seconds.

### SettingsSection

Each section wrapped in a consistent container using `.panel`, `.eyebrow`, `.form-stack` classes.

### SettingField

Renders a single setting field based on its type:
- **text** → `<input type="text">` with save button
- **number** → `<input type="number">` with min/max and save button
- **toggle** → Custom toggle switch, saves immediately on change
- **select** → `<select>` dropdown with save button
- **password** → `<input type="password">` with show/hide toggle and save button
- **multi-checkbox** → Group of checkboxes (for complexity rules, notification events)
- **readonly** → Display-only value with no input
- **button** → Action button (for backup, restore, export)

Each field shows: label, 🔄 icon if `requires_restart: true`, current value, input control, save/cancel buttons (for text/number/select), immediate save (for toggles).

### Confirmation Dialogs

- **Restart service** → "Are you sure you want to restart the service? All active sessions will be disconnected."
- **Restore from backup** → "This will replace all current data with the backup. Type RESTORE to confirm."
- **Export logs/audit** → Direct download, no confirmation needed

### API Client Updates

New functions in `$lib/api.ts`:

```typescript
export type SettingDef = {
  key: string;
  value: string;
  type: 'text' | 'number' | 'toggle' | 'select' | 'password' | 'multi-checkbox' | 'readonly' | 'button';
  source: 'config' | 'database';
  requires_restart: boolean;
  default: string;
  options?: string[];
  label: string;
  section: string;
};

export type SettingsSectionsResponse = {
  sections: {
    id: string;
    label: string;
    title: string;
    description?: string;
    settings: SettingDef[];
  }[];
  restart_required: boolean;
  pending_changes: string[];
};

export function getSettingsSections(): Promise<SettingsSectionsResponse> { ... }
export function updateSetting(key: string, value: string): Promise<SettingDef> { ... }
export function getRestartStatus(): Promise<{ restart_required: boolean; pending_changes: string[] }> { ... }
export function triggerRestart(): Promise<void> { ... }
export function exportLogs(): Promise<Blob> { ... }
export function exportAudit(): Promise<Blob> { ... }
export function triggerBackup(): Promise<{ ok: boolean; message: string }> { ... }
export function restoreBackup(file: File): Promise<{ ok: boolean; message: string }> { ... }
```

### Styling

Uses existing CSS classes: `.page-stack`, `.panel`, `.eyebrow`, `.form-stack`, `.callout`, `.button`.

New CSS additions:
- `.settings-nav` — horizontal scrollable pill bar
- `.settings-nav a.active` — active section highlight
- `.setting-field` — field row with label, input, and restart icon
- `.setting-field .restart-badge` — small 🔄 indicator
- `.toggle-switch` — custom toggle component
- `.multi-checkbox-group` — checkbox group layout

### Loading & Error States

- Page shows loading spinner while `getSettingsSections()` fetches
- Individual fields show inline error messages on save failure
- Restart banner polls `getRestartStatus()` every 30 seconds
- After restart trigger, page enters "reconnecting" state with retry loop

## Database Changes

### New Migration: `0002_settings_registry.sql`

Creates a `setting_definitions` table to store the registry of all known settings with their metadata. This allows the backend to dynamically build the settings UI without hardcoding section/field definitions in Go structs.

```sql
CREATE TABLE IF NOT EXISTS setting_definitions (
    key TEXT PRIMARY KEY,
    label TEXT NOT NULL,
    section TEXT NOT NULL,
    type TEXT NOT NULL CHECK(type IN ('text','number','toggle','select','password','multi-checkbox','readonly','button')),
    source TEXT NOT NULL CHECK(source IN ('config','database')),
    requires_restart INTEGER NOT NULL DEFAULT 0,
    default_value TEXT NOT NULL DEFAULT '',
    options TEXT,  -- JSON array for select/multi-checkbox options
    validation TEXT,  -- regex or validation rule
    sort_order INTEGER NOT NULL DEFAULT 0
);
```

The `app_settings` table continues to store runtime values for database-sourced settings. Config-sourced settings are read from the TOML file at startup and when requested.

## Security Considerations

- **TOML config file** is written with 0600 permissions (owner read/write only)
- **Sensitive fields** (database DSN, encryption key path, TLS key path) are masked in API responses — only the last 4 characters shown
- **CSRF protection** applies to all setting mutations
- **RBAC** enforces that only owners can modify settings, restart, export, or backup
- **Audit logging** records all setting changes with old/new values
- **Backup files** are encrypted by default (configurable)
- **Restore** requires typing "RESTORE" to confirm
- **Service restart** requires explicit confirmation with warning about session disconnection