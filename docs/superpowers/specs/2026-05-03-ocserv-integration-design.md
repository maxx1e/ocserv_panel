# ocserv Integration Design

**Date:** 2026-05-03
**Status:** Draft

## Overview

Replace the `NoopManager` stub and hardcoded "not_configured" dashboard status with real ocserv integration: binary detection, readiness checking, systemd service management, active session management via `occtl`, user management via `ocpasswd`, and production config guidance.

## Approach

**Minimal Adapter Layer** — thin adapters wrapping command execution (`occtl`, `ocpasswd`, `systemctl`) behind Go interfaces. Each adapter is independently testable with fakes. No external dependencies, no gRPC, no sidecar processes.

## Section 1: Config & Binary Path Extensions

### Problem

The panel cannot locate the ocserv, occtl, or ocpasswd binaries. The `OcpasswdPath` setting conflates the binary path with the password file path.

### Changes

**`InstallationConfig` in `config.go`:**

| Field | Type | Default | Purpose |
|-------|------|---------|---------|
| `OcservBinPath` | string | `/usr/local/sbin/ocserv` | Path to ocserv binary |
| `OcctlBinPath` | string | `/usr/local/bin/occtl` | Path to occtl binary |
| `OcpasswdBinPath` | string | `/usr/local/bin/ocpasswd` | Path to ocpasswd binary |
| `OcpasswdFilePath` | string | `/etc/ocserv/ocpasswd` | Path to ocpasswd password file (renamed from `OcpasswdPath`) |

**`ServiceConfig` in `config.go`:**

| Field | Type | Default | Purpose |
|-------|------|---------|---------|
| `SystemdUnitName` | string | `ocserv` | systemd unit name for service management |

**`settings_registry.go`:**

- Add `installation.ocserv_bin_path`, `installation.occtl_bin_path`, `installation.ocpasswd_bin_path`
- Rename `installation.ocpasswd_path` → `installation.ocpasswd_file_path` with label "ocpasswd Password File"
- Add `service.systemd_unit_name`

**Backward compatibility:** If `OcpasswdFilePath` is empty at load time, check the old `OcpasswdPath` key. This handles existing config files that still use the old key name.

### Sub-agent ownership

- **Backend config agent:** `config.go`, `settings_registry.go`, config loading tests
- **SQLite persistence agent:** migration for any stored settings key rename (if settings are persisted in DB)

## Section 2: ocserv Readiness Detection

### Problem

Dashboard shows hardcoded `OcservStatusSummary{Status: "not_configured", Configured: false, Message: "live ocserv integration is not configured"}`. No detection of whether ocserv is installed, configured, or running.

### Solution

Add `OcservReadinessChecker` that probes the system and returns structured status:

```go
type OcservReadiness struct {
    BinaryPresent    bool   // ocserv binary exists and is executable
    BinaryVersion    string // parsed from `ocserv --version`
    ConfigPresent    bool   // ocserv.conf exists at configured path
    ConfigValid      bool   // `ocserv -f -t` succeeds
    ConfigErrors     string // stderr from config test if invalid
    ServiceRunning   bool   // systemd unit is active
    SocketPresent    bool   // management socket file exists
    OcctlPresent     bool   // occtl binary exists
    OcpasswdPresent  bool   // ocpasswd binary exists
}
```

```go
type OcservReadinessChecker struct { /* cfg, serviceMgr */ }

func (c *OcservReadinessChecker) Check(ctx context.Context) (*OcservReadiness, error)
```

**Integration:**
- `NewOcservReadinessChecker(cfg InstallationConfig, serviceMgr ServiceManager) *OcservReadinessChecker`
- Dashboard service calls `checker.Check(ctx)` instead of returning hardcoded status
- Frontend status card maps readiness fields to meaningful display states:
  - All true → "Running" (green)
  - Binary present, service not running → "Installed, not running" (yellow)
  - Binary missing → "Not installed" (red)
  - Config invalid → "Config error: ..." (red)

**Error handling:** Each probe catches errors individually — one missing binary does not prevent checking the others. Config validation has a 5s timeout to avoid hanging on broken configs.

### Sub-agent ownership

- **Backend API agent:** `ocserv/readiness.go`, `dashboard_service.go` changes, API response updates
- **Frontend admin UX agent:** Dashboard status card redesign

## Section 3: Systemd Service Management

### Problem

No way to start/stop/restart ocserv or check its service status from the panel.

### Solution

Add `ServiceManager` interface and `SystemdManager` implementation:

```go
type ServiceStatus int

const (
    ServiceActive ServiceStatus = iota
    ServiceInactive
    ServiceFailed
    ServiceNotFound
)

type ServiceManager interface {
    Status(ctx context.Context) (ServiceStatus, error)
    Start(ctx context.Context) error
    Stop(ctx context.Context) error
    Restart(ctx context.Context) error
    IsEnabled(ctx context.Context) (bool, error)
}
```

```go
type SystemdManager struct {
    unitName string
    timeout  time.Duration
}

func NewSystemdManager(unitName string) *SystemdManager
```

**Command mapping:**
- `Status` → `systemctl status <unit>` (parse output)
- `Start` → `systemctl start <unit>`
- `Stop` → `systemctl stop <unit>`
- `Restart` → `systemctl restart <unit>`
- `IsEnabled` → `systemctl is-enabled <unit>`

**Timeouts:** 30s for start/restart, 10s for stop, 5s for status.

**Security:**
- All operations require RBAC check (`ocserv.service.manage` permission)
- All operations produce audit records
- Start/Stop/Restart are destructive — frontend must show confirmation dialog
- Commands run via `exec.Command` — panel process needs sudo/sudoers rules

**Sudoers integration:** Document required rules in `deploy/sudoers.d/ocserv-panel`:
```
ocserv-panel ALL=(root) NOPASSWD: /usr/bin/systemctl start ocserv, /usr/bin/systemctl stop ocserv, /usr/bin/systemctl restart ocserv, /usr/bin/systemctl status ocserv, /usr/bin/systemctl is-enabled ocserv
```

**Fake for testing:** `FakeServiceManager` with in-memory state.

### Sub-agent ownership

- **Backend API agent:** `ocserv/service.go`, `ocserv/service_fake.go`, API endpoints, RBAC checks
- **Deployment/Ops agent:** `deploy/sudoers.d/ocserv-panel`
- **Security/RBAC agent:** Review RBAC rules for service management

## Section 4: Active Session Management via occtl

### Problem

`NoopManager` stub returns empty sessions and `ErrNotConfigured`. No real ocserv session visibility.

### Solution

Replace `NoopManager` with `OcctlAdapter` implementing the full `Manager` interface:

```go
type Manager interface {
    ListSessions(ctx context.Context) ([]Session, error)
    DisconnectSession(ctx context.Context, sessionID string) error
    ListUsers(ctx context.Context) ([]User, error)
    GetSessionDetails(ctx context.Context, id string) (*SessionDetail, error)
}
```

**OcctlAdapter command mapping:**
- `ListSessions` → `occtl show sessions --output-type json` → parse JSON
- `DisconnectSession` → `occtl disconnect id <sessionID>`
- `ListUsers` → `occtl show users --output-type json` → parse JSON
- `GetSessionDetails` → `occtl show session <id> --output-type json` → parse JSON

**Constructor:** `NewOcctlAdapter(occtlPath, socketPath string) *OcctlAdapter`

**Error handling:**
- Socket not found → return `ErrNotConfigured`
- occtl not found → return `ErrNotConfigured`
- occtl returns non-zero → parse stderr, return descriptive error
- Timeout: 10s for list, 15s for disconnect

**Security:**
- Disconnect requires RBAC (`ocserv.sessions.disconnect`)
- Disconnect produces audit record (who disconnected which session)
- occtl may need sudo — document in sudoers

**Fake for testing:** `FakeManager` (rename from NoopManager) with controllable session data.

### Sub-agent ownership

- **ocserv integration agent:** `ocserv/client.go`, `ocserv/occtl_adapter.go`, `ocserv/occtl_adapter_test.go`, `ocserv/client_fake.go`
- **Backend API agent:** API endpoints for session listing/disconnect, RBAC checks
- **Security/RBAC agent:** Review RBAC rules for session disconnect

## Section 5: ocpasswd User Management Adapter

### Problem

No way to create, delete, or modify password-based ocserv users from the panel.

### Solution

Add `PasswdManager` interface and `OcpasswdAdapter`:

```go
type PasswdManager interface {
    CreateUser(ctx context.Context, username, password string) error
    DeleteUser(ctx context.Context, username string) error
    UpdatePassword(ctx context.Context, username, newPassword string) error
    ListUsers(ctx context.Context) ([]PasswdUser, error)
    UserExists(ctx context.Context, username string) (bool, error)
}

type PasswdUser struct {
    Username string
    Group    string
    Active   bool
}
```

**OcpasswdAdapter command mapping:**
- `CreateUser` → `ocpasswd -c <config> <username>` with password piped via `cmd.Stdin` (not shell echo, to avoid injection)
- `DeleteUser` → `ocpasswd -c <config> -d <username>`
- `UpdatePassword` → same as create (ocpasswd overwrites), password piped via `cmd.Stdin`
- `ListUsers` → parse `/etc/ocserv/ocpasswd` file directly (format: `username:group:hashed_password`)
- `UserExists` → check if username appears in password file

**Constructor:** `NewOcpasswdAdapter(ocpasswdBinPath, ocpasswdFilePath, ocservConfigPath string) *OcpasswdAdapter`

**Security:**
- All operations require RBAC (`ocserv.users.create`, `ocserv.users.delete`, `ocserv.users.update`)
- All operations produce audit records
- Password is never logged — only username and operation type
- ocpasswd may need sudo — document in sudoers
- Password file should be readable by panel user but not world-readable

**Fake for testing:** `FakePasswdManager` with in-memory user store.

### Sub-agent ownership

- **ocserv integration agent:** `ocserv/passwd.go`, `ocserv/passwd_adapter.go`, `ocserv/passwd_adapter_test.go`, `ocserv/passwd_fake.go`
- **Backend API agent:** API endpoints for user CRUD, RBAC checks
- **Security/RBAC agent:** Review RBAC rules for user management

## Section 6: Production Config Guidance

### Problem

ocserv.conf uses relative test paths (`../tests/certs/`) for certificates, and the panel has no way to help users identify and fix configuration issues.

### Solution

Add `ConfigValidator` that checks ocserv config for common production issues:

```go
type ConfigIssueSeverity string

const (
    IssueError   ConfigIssueSeverity = "error"
    IssueWarning ConfigIssueSeverity = "warning"
    IssueInfo    ConfigIssueSeverity = "info"
)

type ConfigIssue struct {
    Severity   ConfigIssueSeverity
    Key        string // config directive, e.g. "server-cert"
    Message    string // human-readable description
    Suggestion string // suggested fix
}

type ConfigValidator struct {
    cfgPath string
    fs      fs.FS
}

func (v *ConfigValidator) ValidateConfig(ctx context.Context) ([]ConfigIssue, error)
```

**Checks performed:**
1. **Relative cert paths** — `server-cert`, `server-key`, `ca-cert` using relative paths → suggest absolute paths
2. **Missing cert files** — cert paths that don't exist on disk
3. **Missing password file** — `auth = "plain[passwd=...]"` references a file that doesn't exist
4. **Socket file path** — `socket-file` points to a non-standard location
5. **Device not found** — `device` directive references an interface that doesn't exist

**Integration:**
- Dashboard readiness check includes config validation results
- Frontend shows config issues as a list with severity badges and suggested fixes
- Not auto-fixing — only surfacing guidance. User must apply fixes manually.

**No sudo needed** — only reads config file and checks filesystem paths.

### Sub-agent ownership

- **Config safety agent:** `ocserv/config_validator.go`, `ocserv/config_validator_test.go`
- **Frontend admin UX agent:** Config issues display in dashboard

## Cross-cutting Concerns

### API Endpoints

New endpoints needed:

| Method | Path | Purpose | RBAC Permission |
|--------|------|---------|-----------------|
| GET | `/api/v1/ocserv/readiness` | Check ocserv installation status | `ocserv.read` |
| GET | `/api/v1/ocserv/service/status` | Get service status | `ocserv.service.read` |
| POST | `/api/v1/ocserv/service/start` | Start ocserv service | `ocserv.service.manage` |
| POST | `/api/v1/ocserv/service/stop` | Stop ocserv service | `ocserv.service.manage` |
| POST | `/api/v1/ocserv/service/restart` | Restart ocserv service | `ocserv.service.manage` |
| GET | `/api/v1/ocserv/sessions` | List active sessions | `ocserv.sessions.read` |
| POST | `/api/v1/ocserv/sessions/{id}/disconnect` | Disconnect session | `ocserv.sessions.disconnect` |
| GET | `/api/v1/ocserv/users` | List ocserv users | `ocserv.users.read` |
| POST | `/api/v1/ocserv/users` | Create ocserv user | `ocserv.users.create` |
| DELETE | `/api/v1/ocserv/users/{username}` | Delete ocserv user | `ocserv.users.delete` |
| PUT | `/api/v1/ocserv/users/{username}/password` | Update user password | `ocserv.users.update` |
| GET | `/api/v1/ocserv/config/issues` | List config issues | `ocserv.config.read` |

### Sudoers Rules

All required sudoers rules consolidated in `deploy/sudoers.d/ocserv-panel`:
```
ocserv-panel ALL=(root) NOPASSWD: /usr/local/sbin/ocserv -f -t
ocserv-panel ALL=(root) NOPASSWD: /usr/local/bin/occtl show *
ocserv-panel ALL=(root) NOPASSWD: /usr/local/bin/occtl disconnect *
ocserv-panel ALL=(root) NOPASSWD: /usr/local/bin/ocpasswd -c /etc/ocserv/ocserv.conf *
ocserv-panel ALL=(root) NOPASSWD: /usr/bin/systemctl start ocserv
ocserv-panel ALL=(root) NOPASSWD: /usr/bin/systemctl stop ocserv
ocserv-panel ALL=(root) NOPASSWD: /usr/bin/systemctl restart ocserv
ocserv-panel ALL=(root) NOPASSWD: /usr/bin/systemctl status ocserv
ocserv-panel ALL=(root) NOPASSWD: /usr/bin/systemctl is-enabled ocserv
```

### Frontend Changes

- Dashboard ocserv status card: replace hardcoded "not_configured" with real readiness data
- New ocserv section in sidebar navigation
- Sessions page: list active sessions, disconnect button with confirmation
- Users page: list/create/delete/update password users
- Service controls: start/stop/restart buttons with confirmation dialogs
- Config issues: list with severity badges and suggested fixes

### Sub-agent Parallelism Plan

The following agents can work in parallel (no shared file ownership):

**Wave 1 (parallel):**
1. **Backend config agent:** config.go, settings_registry.go, config tests
2. **ocserv integration agent:** ocserv/client.go, ocserv/occtl_adapter.go, ocserv/passwd_adapter.go, fakes
3. **Config safety agent:** ocserv/config_validator.go, tests

**Wave 2 (parallel, after Wave 1):**
4. **Backend API agent:** API endpoints, RBAC, wiring in main.go
5. **Deployment/Ops agent:** deploy/sudoers.d/ocserv-panel, systemd unit file

**Wave 3 (after Wave 2):**
6. **Frontend admin UX agent:** Dashboard status, sessions page, users page, service controls
7. **Security/RBAC agent:** Review all RBAC rules, sudoers, audit coverage

**Wave 4:**
8. **Test/verification agent:** Full integration verification