# Config Management Backend — Design Spec

**Date:** 2026-05-05
**Status:** Approved
**Produces:** REST API at `/api/config/*`

---

## Overview

Backend for safe ocserv configuration management: read, validate, backup, write, reload, rollback, and read-only network prerequisites inspection. All operations are RBAC-gated, audit-logged, and tested without a live ocserv instance.

This is distinct from the `/settings` backend which manages the panel's own `config.toml`.

---

## Architecture

A single `ConfigManagementService` (in `backend/internal/app/`) orchestrates the full lifecycle. It delegates to focused adapters in `backend/internal/ocserv/`.

### New Files

```
backend/internal/ocserv/
├── config_file.go           — file I/O with flock locking and backup rotation
├── config_file_test.go      — read, write, backup rotation, rollback, locking
├── config_reload.go         — occtl reload adapter
├── config_reload_test.go    — success, timeout, stderr via FakeCommandRunner
├── network_inspector.go     — read-only kernel params + firewall + routing checks
├── network_inspector_test.go — fake /proc reads, fake iptables/nftables output
├── config_validator.go      — (exists) extended with syntax/port/route/DNS checks
├── config_validator_test.go — (exists) extended with new check tests

backend/internal/app/
├── config_management_service.go   — orchestrates all config operations with RBAC + audit
├── config_management_service_test.go — full apply/rollback/lock flows

backend/internal/server/
├── config_handlers.go       — HTTP API handlers for /api/config/*
├── config_handlers_test.go  — RBAC, validation errors, JSON shapes
```

### Modified Files

```
backend/internal/rbac/rbac.go          — add PermissionOcservConfigEdit, PermissionOcservConfigRollback
backend/internal/server/server.go      — register /api/config/* routes, wire ConfigManagementService
backend/cmd/ocserv-panel/main.go       — instantiate ConfigManagementService
```

### Data Flow

```
HTTP Handler → ConfigManagementService
                   ├── Read:     ConfigFileManager.Read() → content + metadata
                   ├── Validate: extended ConfigValidator.ValidateContent(text) → issues[]
                   ├── Diff:     ConfigFileManager.DiffWithContent(proposed) → unified diff
                   ├── Apply:    validate → lock → rotate backups → write → reload → audit
                   ├── Rollback: lock → rotate backups → copy backup N → reload → audit
                   └── Network:  NetworkInspector.Inspect() → prerequisites[]
```

---

## RBAC Permissions

| Constant | String | Description |
|----------|--------|-------------|
| `PermissionOcservConfigRead` | `ocserv:config:read` | (exists) View config, backups, network status |
| `PermissionOcservConfigEdit` | `ocserv:config:edit` | (new) Modify and apply config |
| `PermissionOcservConfigRollback` | `ocserv:config:rollback` | (new) Restore from backup |

- `RoleOwner` gets all three.
- `RoleViewer` keeps only `ocserv:config:read`.

---

## Backend — Config File I/O (`config_file.go`)

### Types

```go
// ConfigFileManager handles atomic reads, writes, backups and rollback for a single
// ocserv config file. All mutating operations acquire an OS-level flock before proceeding.
type ConfigFileManager struct {
    path       string
    maxBackups int
}

func NewConfigFileManager(path string, maxBackups int) *ConfigFileManager

type ConfigMetadata struct {
    Path         string    `json:"path"`
    LastModified time.Time `json:"lastModified"`
    Size         int64     `json:"size"`
}

type BackupInfo struct {
    Number    int       `json:"number"`
    Timestamp time.Time `json:"timestamp"`
    Size      int64     `json:"size"`
}
```

### Methods

```go
func (m *ConfigFileManager) Read() (content string, meta ConfigMetadata, err error)
func (m *ConfigFileManager) ListBackups() ([]BackupInfo, error)
func (m *ConfigFileManager) GetBackup(n int) (string, error)
func (m *ConfigFileManager) DiffWithContent(proposed string) (string, error)
func (m *ConfigFileManager) DiffWithBackup(n int) (string, error)
// Write rotates backups then writes content atomically with 0640 perms.
// Acquires flock; returns ErrConfigLocked if flock cannot be acquired in 100ms.
func (m *ConfigFileManager) Write(content string) (backupNumber int, err error)
// Rollback rotates current into .1 first (making rollback reversible),
// then copies backup N to the live path.
func (m *ConfigFileManager) Rollback(n int) error
```

### Backup Rotation

```
Before Write():
  .9 → deleted (if max=10)
  .8 → .9
  ...
  .1 → .2
  live → .1
  new  → live (0640)
```

Rollback(n):
1. Acquire flock
2. Rotate current into `.1` (so rollback is reversible)
3. Copy backup `.n` to live path (0640)
4. Release flock

### Diff Generation

Pure Go line-by-line Myers diff, output in unified format. No external library. Max 500-line context (truncated with a note beyond that).

### Sentinel Errors

```go
var ErrConfigLocked     = errors.New("config file is locked by another operation")
var ErrBackupNotFound   = errors.New("config backup not found")
var ErrConfigTooLarge   = errors.New("config content exceeds 64KB limit")
```

---

## Backend — Config Reload (`config_reload.go`)

```go
type ConfigReloader struct {
    runner   CommandRunner
    occtlBin string
}

func NewConfigReloader(runner CommandRunner, occtlBin string) *ConfigReloader

type ReloadResult struct {
    Success bool
    Stderr  string
}

func (r *ConfigReloader) Reload(ctx context.Context) (ReloadResult, error)
```

- Runs `occtl reload` with a 10-second timeout.
- Returns `ReloadResult{Success: false, Stderr: ...}` when the command exits non-zero (not a Go error — caller decides how to surface this).
- Returns a Go `error` only on execution failure (binary not found, context cancelled, etc.).

---

## Backend — Extended ConfigValidator

Existing static-analysis checks are preserved. Add:

| Check | Severity | Detail |
|-------|----------|--------|
| Malformed line | `error` | Line has content but no `=` separator and is not a comment |
| Unclosed quote | `error` | Value has odd number of `"` or `'` characters |
| Port range | `error` | `tcp-port`/`udp-port` value outside 1–65535 |
| CIDR notation | `error` | `route`/`no-route` values that fail `net.ParseCIDR` |
| DNS IP | `warning` | `dns` directive value that fails `net.ParseIP` |

New entry point for validating arbitrary content (not just the on-disk file):

```go
// ValidateContent validates a config string without reading from disk.
// Used by the apply flow to pre-validate proposed content.
func (v *ConfigValidator) ValidateContent(ctx context.Context, content string) ([]ConfigIssue, error)
```

The existing `ValidateConfig(ctx)` is retained (reads from `v.configPath`) for dashboard/readiness use.

---

## Backend — Network Inspector (`network_inspector.go`)

Read-only inspection of host prerequisites for VPN routing.

```go
type NetworkInspector struct {
    runner  CommandRunner
    procDir string // default: "/proc/sys", overridable in tests
}

func NewNetworkInspector(runner CommandRunner) *NetworkInspector
func NewNetworkInspectorWithProcDir(runner CommandRunner, procDir string) *NetworkInspector

type NetworkPrerequisites struct {
    Items []PrerequisiteCheck `json:"items"`
}

type PrerequisiteCheck struct {
    Category   string `json:"category"`   // "kernel" | "firewall" | "routing"
    Name       string `json:"name"`
    Status     string `json:"status"`     // "configured" | "missing" | "partial" | "unknown"
    Expected   string `json:"expected"`
    Actual     string `json:"actual"`
    Suggestion string `json:"suggestion"`
}

func (i *NetworkInspector) Inspect(ctx context.Context) (NetworkPrerequisites, error)
```

### Checks

**Kernel (read from `procDir`):**
- `net/ipv4/ip_forward` — expected `1`
- `net/ipv6/conf/all/forwarding` — expected `1`

**Firewall (via CommandRunner, auto-detect iptables vs nftables):**
- Detection: try `iptables -L -n --line-numbers 2>/dev/null`; if exit non-zero, try `nft list tables`
- NAT POSTROUTING masquerade present
- FORWARD chain accepts `vpns+`
- INPUT chain allows the ocserv listening port

**Routing:**
- Reads `ip route show` output, checks VPN subnet (parsed from `/proc/sys/net/...` or skipped if unavailable) doesn't conflict with a host route.

Inspector never fails hard — if a command is unavailable or errors, the relevant check is `"unknown"`.

---

## Service Layer (`config_management_service.go`)

```go
type ConfigManagementService struct {
    files      *ocserv.ConfigFileManager
    validator  *ocserv.ConfigValidator
    reloader   *ocserv.ConfigReloader
    inspector  *ocserv.NetworkInspector
    recorder   audit.Recorder
    authorizer rbac.Authorizer
}

func NewConfigManagementService(
    files     *ocserv.ConfigFileManager,
    validator *ocserv.ConfigValidator,
    reloader  *ocserv.ConfigReloader,
    inspector *ocserv.NetworkInspector,
    recorder  audit.Recorder,
    authorizer rbac.Authorizer,
) *ConfigManagementService
```

### Public Methods (all require `auth.Principal`)

```go
ReadConfig(ctx, principal) → (ConfigReadResult, error)
ListBackups(ctx, principal) → ([]ocserv.BackupInfo, error)
GetBackup(ctx, principal, n int) → (string, error)
DiffWithContent(ctx, principal, proposed string) → (string, error)
DiffWithBackup(ctx, principal, n int) → (string, error)
ValidateContent(ctx, principal, content string) → (ValidationResult, error)
ApplyConfig(ctx, principal, content string) → (ApplyResult, error)
Rollback(ctx, principal, n int) → (RollbackResult, error)
InspectNetwork(ctx, principal) → (ocserv.NetworkPrerequisites, error)
```

### Result Types

```go
type ConfigReadResult struct {
    Content  string                `json:"content"`
    Metadata ocserv.ConfigMetadata `json:"metadata"`
    Issues   []ocserv.ConfigIssue  `json:"issues"`
}

type ValidationResult struct {
    Valid  bool                 `json:"valid"`
    Issues []ocserv.ConfigIssue `json:"issues"`
}

type ApplyResult struct {
    BackupNumber  int                  `json:"backupNumber"`
    ReloadSuccess bool                 `json:"reloadSuccess"`
    ReloadError   string               `json:"reloadError,omitempty"`
    Issues        []ocserv.ConfigIssue `json:"issues"`
}

type RollbackResult struct {
    ReloadSuccess bool   `json:"reloadSuccess"`
    ReloadError   string `json:"reloadError,omitempty"`
}
```

### Apply Flow

```
1. requirePerm(ocserv:config:edit)
2. size check — reject > 64KB
3. ValidateContent → block if any error-severity issues
4. files.Write(content) → acquires flock internally; returns ErrConfigLocked → 409
5. reloader.Reload(ctx)
6. audit: config.applied (backup number, line diff summary ≤500 chars, reload result)
```

If `files.Write` succeeds but `reloader.Reload` fails: return `ApplyResult{ReloadSuccess: false, ReloadError: stderr}` with HTTP 200 (partial success — config is on disk and was validated).

### Rollback Flow

```
1. requirePerm(ocserv:config:rollback)
2. files.Rollback(n) → acquires flock; returns ErrBackupNotFound or ErrConfigLocked
3. reloader.Reload(ctx)
4. audit: config.rollback (n, reload result)
```

### Sentinel Errors (app package)

```go
var ErrConfigValidationFailed = errors.New("config validation failed")
var ErrConfigLocked            = errors.New("config file is locked")
var ErrConfigBackupNotFound    = errors.New("config backup not found")
```

---

## API Endpoints

**Base path:** `/api/config`

| Method | Path | Permission | HTTP Success | Description |
|--------|------|-----------|-------------|-------------|
| GET | `/api/config` | `ocserv:config:read` | 200 | Read config + metadata + current issues |
| POST | `/api/config/validate` | `ocserv:config:read` | 200 | Validate proposed content |
| POST | `/api/config/diff` | `ocserv:config:read` | 200 | Diff proposed vs current |
| POST | `/api/config/apply` | `ocserv:config:edit` | 200 | Validate → backup → write → reload |
| GET | `/api/config/backups` | `ocserv:config:read` | 200 | List backups |
| GET | `/api/config/backups/{n}` | `ocserv:config:read` | 200 | Get backup content |
| GET | `/api/config/backups/{n}/diff` | `ocserv:config:read` | 200 | Diff backup N vs current |
| POST | `/api/config/rollback/{n}` | `ocserv:config:rollback` | 200 | Restore backup N |
| GET | `/api/config/network` | `ocserv:config:read` | 200 | Network prerequisites |

### Request/Response Shapes

```json
// GET /api/config → 200
{ "content": "...", "path": "/etc/ocserv/ocserv.conf",
  "lastModified": "2026-05-05T14:00:00Z", "size": 2048,
  "issues": [{ "severity": "warning", "key": "server-cert", "message": "...", "suggestion": "..." }] }

// POST /api/config/validate  body: { "content": "..." }
{ "valid": false, "issues": [...] }

// POST /api/config/diff  body: { "content": "..." }
{ "diff": "--- a/ocserv.conf\n+++ b/ocserv.conf\n..." }

// POST /api/config/apply  body: { "content": "..." }
{ "backupNumber": 1, "reloadSuccess": true, "reloadError": "", "issues": [] }

// GET /api/config/backups
{ "backups": [{ "number": 1, "timestamp": "...", "size": 2048 }], "maxBackups": 10 }

// GET /api/config/backups/{n}
{ "content": "..." }

// GET /api/config/backups/{n}/diff
{ "diff": "..." }

// POST /api/config/rollback/{n}
{ "reloadSuccess": true, "reloadError": "" }

// GET /api/config/network
{ "items": [{ "category": "kernel", "name": "IPv4 Forwarding",
  "status": "configured", "expected": "1", "actual": "1", "suggestion": "" }] }
```

### Error Codes

| Code | Cause |
|------|-------|
| 400 | Content exceeds 64KB, or missing required body field |
| 401 | Not authenticated |
| 403 | Insufficient RBAC permission |
| 404 | Backup N not found |
| 409 | File locked (concurrent operation) |
| 422 | Validation failed — body contains `issues[]` |
| 500 | Filesystem or reload error |
| 503 | ocserv not available (binary/socket missing) |

---

## Audit Events

| Action | Outcome | Metadata |
|--------|---------|----------|
| `config.read` | success / denied | — |
| `config.validated` | success / denied | `valid`, `issue_count` |
| `config.applied` | success / failure / denied | `backup_number`, `diff_summary` (≤500 chars), `reload_success` |
| `config.rollback` | success / failure / denied | `backup_number`, `reload_success` |
| `config.network_inspected` | success / denied | — |

---

## Security Constraints

- **Path restriction:** Only `InstallationConfig.OcservConfigPath` is ever read or written — no path parameters accepted
- **Content size limit:** Reject content > 64KB (checked before any I/O)
- **No shell injection:** All commands use `CommandRunner` argument arrays (never shell strings)
- **File permissions:** Config and backups written with 0640
- **flock-based locking:** OS-managed; released on process death; 100ms acquire timeout returns ErrConfigLocked
- **Audit diff truncation:** Diff summaries stored in audit are capped at 500 chars

---

## Testing Strategy

All tests run without a live ocserv instance.

| File | What's tested |
|------|---------------|
| `config_file_test.go` | Read/write/backup rotation/rollback/flock/permissions via `t.TempDir()` |
| `config_validator_test.go` | New syntax/port/CIDR/DNS checks; `ValidateContent()` |
| `config_reload_test.go` | Success, non-zero exit, timeout — all via `FakeCommandRunner` |
| `network_inspector_test.go` | Kernel param parsing, iptables/nftables fake output, unknown status |
| `config_management_service_test.go` | Apply/rollback flows, 64KB limit, lock contention, validation blocking, partial-success (write ok / reload fail) |
| `config_handlers_test.go` | RBAC deny paths, JSON shapes, 409/422 error codes |

---

## Dependencies

- No new external Go packages
- Reuses: `CommandRunner`, `FakeCommandRunner`, `ConfigValidator`, RBAC, audit logging
- Diff: internal pure-Go implementation (no external lib)
