# Config Management WebUI — Design Spec

**Date:** 2026-05-05
**Route:** `/config`
**Status:** SUPERSEDED — split into:
- `2026-05-05-config-management-backend-design.md`
- `2026-05-05-config-management-frontend-design.md`

---

## Overview

A WebUI page to manage the ocserv configuration file (`ocserv.conf`) safely, with full visibility, validation, backup/versioning, controlled updates via `occtl reload`, and read-only network prerequisites inspection.

This is distinct from the `/settings` page which manages the panel's own `config.toml`.

---

## Architecture

### Approach: Monolithic Config Service

A single `ConfigManagementService` orchestrates the full lifecycle: read, edit, validate, backup, write, reload. It leverages the existing `ConfigValidator`, `CommandRunner`, and RBAC infrastructure.

### New Files

```
backend/internal/app/
├── config_management_service.go   — orchestrates read/validate/backup/write/reload
├── config_management_handlers.go  — HTTP API handlers

backend/internal/ocserv/
├── config_validator.go            — (exists) extended with syntax/port/route checks
├── config_file.go                 — file I/O with locking, backup rotation
├── config_reload.go               — occtl reload adapter
├── network_inspector.go           — iptables + kernel params + routing checks

frontend/src/routes/config/
├── +page.svelte                   — replaces current placeholder
```

### Data Flow

```
UI → API Handler → ConfigManagementService
                       ├── Read:     config_file.go → returns file content + metadata
                       ├── Validate: config_validator.go → returns issues[]
                       ├── Apply:    config_file.go (backup + write) → config_reload.go (occtl reload)
                       ├── Rollback: config_file.go (restore from numbered backup)
                       └── Inspect:  network_inspector.go → returns network prerequisites status
```

### RBAC Permissions

| Permission | Description |
|-----------|-------------|
| `ocserv:config:read` | (exists) View config content, backups, network status |
| `ocserv:config:edit` | (new) Modify and apply config |
| `ocserv:config:rollback` | (new) Restore from backup |

---

## Backend — Config File Management

### File I/O (`config_file.go`)

- Read ocserv.conf from the path in `InstallationConfig.OcservConfigPath`
- File-level locking via `flock` to prevent concurrent writes
- Numbered backup rotation stored alongside the config file

### Backup Rotation Logic

```
Before write:
  ocserv.conf.9 → deleted (if max=10)
  ocserv.conf.8 → ocserv.conf.9
  ...
  ocserv.conf.1 → ocserv.conf.2
  ocserv.conf   → ocserv.conf.1   (current becomes most recent backup)
  new content   → ocserv.conf     (write new)
```

- Max backups: 10 (configurable)
- Metadata per backup: modification timestamp (filesystem), file size, sequence number
- Rollback: rotates current into `.1` first (so rollback is itself reversible), then copies backup N back to `ocserv.conf`

### Service Methods

```go
ReadConfig() → (content string, metadata ConfigMetadata, err error)
ListBackups() → ([]BackupInfo, error)
GetBackup(n int) → (content string, err error)
DiffWithBackup(n int) → (diff string, err error)
DiffWithContent(proposed string) → (diff string, err error)
ValidateContent(content string) → (ValidationResult, error)
ApplyConfig(content string, actor string) → (ApplyResult, error)
Rollback(n int, actor string) → (RollbackResult, error)
InspectNetwork() → (NetworkPrerequisites, error)
```

---

## Backend — Validation & Reload

### Extended ConfigValidator

Existing checks (cert paths, auth directives, socket paths, TUN devices) plus:

- **Syntax validation:** Malformed lines, unclosed quotes, invalid directives
- **Port conflict detection:** Warn if port already bound
- **Route validation:** CIDR notation validity for `route` / `no-route`
- **DNS validation:** Valid IP addresses for DNS servers
- **Cross-reference:** If `auth = "plain[passwd=/path]"`, ensure passwd file exists

### Severity Levels

- `error` — blocks apply
- `warning` — allows apply, shown prominently
- `info` — advisory

### Config Reload (`config_reload.go`)

- Executes `occtl reload` via `CommandRunner`
- Timeout: 10 seconds
- On failure: returns error with stderr (config stays written — was validated)
- Audit log on success and failure

### Apply Flow (atomic sequence)

```
1. ValidateContent(proposed)     → block if errors exist
2. AcquireFileLock()             → fail 409 if locked
3. RotateBackups()               → shift numbered files
4. WriteConfig(proposed)         → write with 0640 permissions
5. ReloadService()               → occtl reload
6. ReleaseFileLock()
7. AuditLog(actor, "config_applied", diff_summary)
```

If reload fails, config is written (was validated). UI shows reload error; user can retry or rollback.

---

## Backend — Network Inspector

### Read-only host inspection for VPN router prerequisites

**Kernel Parameters:**
- `net.ipv4.ip_forward` — read from `/proc/sys/net/ipv4/ip_forward`
- `net.ipv6.conf.all.forwarding` — read from `/proc/sys/net/ipv6/conf/all/forwarding`

**Firewall Rules (auto-detect iptables-legacy vs nftables):**
- NAT/masquerade on outbound interface (POSTROUTING)
- FORWARD rules allowing VPN interface traffic (`vpns+`)
- INPUT rules allowing ocserv listening port (TCP/UDP)

**Inspection commands:**
- `iptables -t nat -L POSTROUTING -n` or `nft list chain ip nat POSTROUTING`
- `iptables -L FORWARD -n` or equivalent nft
- `iptables -L INPUT -n` for port exposure

**Routing:**
- Verify VPN subnet (from ocserv config `ipv4-network`/`ipv6-network`) doesn't conflict with host routes

### Response Model

```go
type NetworkPrerequisites struct {
    Items []PrerequisiteCheck
}

type PrerequisiteCheck struct {
    Category   string // "kernel", "firewall", "routing"
    Name       string // e.g., "IPv4 Forwarding"
    Status     string // "configured" | "missing" | "partial" | "unknown"
    Expected   string
    Actual     string
    Suggestion string // copyable command to fix
}
```

---

## API Endpoints

**Base path:** `/api/config`

| Method | Path | Permission | Description |
|--------|------|-----------|-------------|
| GET | `/api/config` | `ocserv:config:read` | Read current config + metadata |
| POST | `/api/config/validate` | `ocserv:config:read` | Validate proposed content |
| POST | `/api/config/diff` | `ocserv:config:read` | Diff proposed vs current |
| POST | `/api/config/apply` | `ocserv:config:edit` | Validate → backup → write → reload |
| GET | `/api/config/backups` | `ocserv:config:read` | List backups with metadata |
| GET | `/api/config/backups/:n` | `ocserv:config:read` | Get backup content |
| GET | `/api/config/backups/:n/diff` | `ocserv:config:read` | Diff backup N vs current |
| POST | `/api/config/rollback/:n` | `ocserv:config:rollback` | Restore backup N |
| GET | `/api/config/network` | `ocserv:config:read` | Network prerequisites |

### Request/Response Shapes

```json
// GET /api/config
{
  "content": "string",
  "path": "string",
  "last_modified": "timestamp",
  "size": 2048,
  "valid": true,
  "issues": []
}

// POST /api/config/validate  { "content": "..." }
{
  "valid": false,
  "issues": [
    { "severity": "error", "field": "tcp-port", "message": "...", "suggestion": "..." }
  ]
}

// POST /api/config/diff  { "content": "..." }
{
  "diff": "unified diff string"
}

// POST /api/config/apply  { "content": "..." }
{
  "success": true,
  "backup_number": 1,
  "reload_success": true,
  "reload_error": null,
  "issues": []
}

// GET /api/config/backups
{
  "backups": [
    { "number": 1, "timestamp": "2026-05-05T14:32:01Z", "size": 2048 }
  ],
  "max_backups": 10
}

// POST /api/config/rollback/:n
{
  "success": true,
  "reload_success": true,
  "reload_error": null
}

// GET /api/config/network
{
  "items": [
    { "category": "kernel", "name": "IPv4 Forwarding", "status": "configured", "expected": "1", "actual": "1", "suggestion": "" },
    { "category": "firewall", "name": "VPN Forward", "status": "missing", "expected": "ACCEPT vpns+", "actual": "(not found)", "suggestion": "iptables -A FORWARD -i vpns+ -j ACCEPT" }
  ]
}
```

### Error Codes

- 400 — validation failed (issues array in body)
- 403 — insufficient RBAC permission
- 409 — file locked (concurrent edit)
- 500 — filesystem/reload error
- 503 — ocserv not available

---

## Frontend — Page Layout

**Route:** `/config` (replaces current placeholder)

### Three Tabs

1. **Editor** — View and edit config with validation and apply
2. **Backups** — List, view, diff, and rollback previous versions
3. **Network** — Read-only network prerequisites status

### Editor Tab

- Config file path and last-modified timestamp displayed
- Validation status indicator (valid/warning/invalid)
- Monospace styled textarea (no external editor dependency)
- Action buttons: Validate, Preview Changes, Apply
- Inline validation results with severity icons and suggestions
- Diff modal before apply (unified diff, current vs proposed)
- Confirmation dialog on Apply with "This will reload ocserv" warning

### Backups Tab

- Table: number, timestamp, size, action buttons
- View: read-only textarea showing backup content
- Diff: unified diff between backup and current
- Rollback: confirmation dialog → executes rollback → shows result

### Network Tab

- Grouped by category (Kernel, Firewall, Routing)
- Status icons: filled circle (configured), empty circle (missing), half circle (partial)
- Suggestions shown as copyable monospace text
- Refresh button to re-run inspection

### RBAC in UI

- `ocserv:config:read` — view all tabs (editor is read-only)
- `ocserv:config:edit` — enables textarea editing, Validate/Preview/Apply buttons
- `ocserv:config:rollback` — enables Rollback buttons in Backups tab

---

## Audit Logging

| Event | Details Logged |
|-------|---------------|
| `config.viewed` | Config file read |
| `config.validated` | Result (valid/invalid), issue count |
| `config.applied` | Backup number, line change summary (+N/-M), reload result |
| `config.apply_failed` | Failure reason (validation, lock, write, reload) |
| `config.rollback` | Backup number restored, reload result |
| `config.rollback_failed` | Failure reason |
| `config.network_inspected` | Prerequisites check performed |

---

## Security Constraints

- **No secrets in responses:** Show file paths but never read/return private key contents
- **Content size limit:** Reject config > 64KB
- **Path restriction:** Only read/write `InstallationConfig.OcservConfigPath` — no arbitrary file access
- **Lock timeout:** Auto-release after 30 seconds
- **Rate limiting:** Max 1 apply per 10 seconds
- **Backup permissions:** 0640, owned by service user
- **No shell injection:** All commands via `CommandRunner` with argument arrays
- **Audit diff truncation:** Summaries limited to 500 chars; full diffs not stored in audit
- **Concurrency:** `flock` on config file; API returns 409 if locked; UI shows lock message

---

## Testing Strategy

### Backend Tests (fakes, no live ocserv)

| Component | What's Tested |
|-----------|---------------|
| `config_file_test.go` | Read, write, backup rotation, rollback, permissions, lock |
| `config_validator_test.go` | Syntax, paths, ports, routes, cross-references |
| `config_reload_test.go` | Success, timeout, stderr (FakeCommandRunner) |
| `network_inspector_test.go` | Kernel param parsing, iptables output, nftables, subnet conflict |
| `config_management_service_test.go` | Full apply/rollback flows, lock contention, validation blocking |
| `config_handlers_test.go` | RBAC enforcement, request validation, error responses |

### Approach

- File system: `os.MkdirTemp` for isolated test directories
- Commands: `FakeCommandRunner` (exists) for occtl, iptables, sysctl
- RBAC: Test both permitted and denied paths
- Network inspector: Fake `/proc/sys/` reads and fake command output

### Frontend Tests

- Validation display (error/warning/info rendering)
- Diff rendering
- RBAC-gated button visibility
- Apply confirmation flow

---

## Dependencies

- No new external dependencies
- Reuses: `CommandRunner`, `FakeCommandRunner`, `ConfigValidator`, RBAC, audit logging
- Frontend: existing Tailwind CSS, lucide-svelte icons
- Diff generation: Go stdlib or simple unified-diff implementation (no external lib)

---

## Out of Scope

- Guided/form-based config editing (future enhancement)
- Config linting / best practices warnings (future)
- Firewall rule modification (read-only only)
- Multi-file config support
- Import/export via file upload
- Dry-run mode (validation serves this purpose)
