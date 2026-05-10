# Backend Package Restructuring Design

**Date:** 2026-05-10  
**Status:** Approved  
**Scope:** Backend only — frontend, migrations, and deploy assets are untouched

---

## Problem

The current backend has grown over-partitioned in two areas:

1. **`internal/ocserv/`** split prematurely into four sub-packages (`cmd/`, `config/`, `parse/`, `service/`) that are never used independently. Two compat shim files (`cmd_compat.go`, `config_service_compat.go`) exist solely to re-export types from sub-packages back to callers that still reference the parent package. The sub-package split also created a cross-dependency: `config/validator.go` imports `parse/cidr.go` for CIDR helpers, meaning the config package depends on session-parse types — a leaky domain boundary.

2. **`internal/monitoring/` and `internal/metrics/`** are a single polling-and-storage concern split across two packages with no independent consumers. Every file in `monitoring/` imports `metrics/`.

The result is 19 packages and 150 Go files for a single-node admin panel. A new developer must read 5 packages to understand ocserv integration.

---

## Goals

- Package names describe what the service **does**, not how it is implemented.
- Each package has one clear owner and one reason to change.
- No compat shims. Every type lives in exactly one place.
- No cross-sub-package dependencies inside `vpn/` except through `vpn/exec/`.
- Extension points are obvious: adding certificate lifecycle means adding `vpn/certs/`, not deciding between three existing packages.
- All existing tests continue to pass without modification to test logic.

---

## Package Map

### Before → After summary

| Before | After | Change |
|---|---|---|
| `internal/ocserv/` (root + 4 sub-pkgs + 2 shims) | `internal/vpn/` (6 sub-packages, no shims) | Reorganised by business task |
| `internal/monitoring/` + `internal/metrics/` | `internal/monitor/` | Merged into one package |
| `internal/audit/` (3 files) | `internal/audit/` (1 file) | Collapsed |
| All others | Unchanged | — |

### New layout

```
backend/internal/
  vpn/
    exec/          command execution primitive (no vpn deps)
    sessions/      active session visibility and control
    users/         VPN user lifecycle (passwd file)
    config/        ocserv.conf read/write/validate/diff/rollback
    service/       systemd unit control
    health/        readiness checks and network prereq inspection
  monitor/         metric collection + tsdb/memory storage (merged)
  store/           SQLite repositories (unchanged shape)
  app/             business services — cross-domain logic (unchanged shape)
  server/          HTTP router + handlers (unchanged shape)
  auth/            tokens, password hashing, rate limiting (unchanged)
  audit/           event types + recorder interface (collapsed to 1 file)
  rbac/            roles and permissions (unchanged)
  config/          app TOML config (unchanged)
  db/              SQLite connection + migrations (unchanged)
  logs/            log reading — file + journald (unchanged)
  validation/      input validation rules (unchanged)
  sanitize/        metadata redaction (unchanged)
  logging/         zerolog init (unchanged)
```

---

## vpn/ Sub-Package Detail

### `vpn/exec/`
Command execution primitive. No dependencies on any other `vpn/` package.

| File | Contents |
|---|---|
| `runner.go` | `CommandRunner` interface, `CommandResult`, `OSCommandRunner` |
| `fake.go` | `FakeCommandRunner`, `FakeCommandCall` (test double) |

Source: `ocserv/cmd/runner.go`, `ocserv/cmd/fake.go`  
Compat shim `ocserv/cmd_compat.go` deleted. All callers updated to import `vpn/exec` directly.

---

### `vpn/sessions/`
Active session visibility and control. Owns all occtl JSON parsing and the `Manager` interface.

| File | Contents |
|---|---|
| `types.go` | `Session`, `SessionDetail`, `ServerStatus`, `ConnectedUser`, `UserConnection`, `ConnectionEvent`, `UserRoute`, `BannedIP`, `BanPointEntry` |
| `parse.go` | All occtl output parsers — merged from `parse/sessions.go`, `parse/user.go`, `parse/users.go`, `parse/status.go`, `parse/bans.go`, `parse/routes.go`, `parse/helpers.go`, `parse/sanitize.go` |
| `manager.go` | `Manager` interface, `FakeManager`, `ErrNotConfigured`, `ErrSessionNotFound` |
| `adapter.go` | `OcctlAdapter` (implements `Manager`) |
| `adapter_test.go` | Existing adapter tests |

Source: `ocserv/client.go`, `ocserv/occtl_adapter.go`, `ocserv/parse/*`  
`parse/` sub-package deleted entirely. `client.go` type aliases deleted — `Manager` and its types are defined here directly.

---

### `vpn/users/`
VPN user lifecycle through the ocpasswd file.

| File | Contents |
|---|---|
| `manager.go` | `PasswdManager` interface, `PasswdUser` type, `ErrPasswordFileMissing` |
| `adapter.go` | `OcpasswdAdapter` (implements `PasswdManager`), `ParsePasswdFile` |
| `fake.go` | `FakePasswdManager` |
| `adapter_test.go` | Existing adapter tests |

Source: `ocserv/passwd.go`, `ocserv/passwd_adapter.go`, `ocserv/passwd_fake.go`

---

### `vpn/config/`
Safe read/write/validate/diff/rollback of `ocserv.conf`. CIDR helper functions from `parse/cidr.go` are inlined here — they have exactly one caller (`validator.go`) and no consumers outside this sub-package.

| File | Contents |
|---|---|
| `file.go` | `ConfigFileManager` — backup, atomic write, diff, rollback |
| `validator.go` | `ConfigValidator`, `ConfigIssue`, inlined `isNetmaskNotation`, `netmaskToCIDR` |
| `reload.go` | `ConfigReloader`, `ReloadResult` |
| `restart.go` | `RestartDetector`, `RestartAnalysis` |
| `file_test.go` | Existing file manager tests |
| `validator_test.go` | Existing validator tests |

Source: `ocserv/config/*`  
Compat shim `ocserv/config_service_compat.go` deleted. All callers updated to import `vpn/config` directly.

---

### `vpn/service/`
systemd unit lifecycle. No dependency on any other `vpn/` sub-package.

| File | Contents |
|---|---|
| `manager.go` | `ServiceManager` interface, `ServiceStatus` constants |
| `systemd.go` | `SystemdManager` |
| `fake.go` | `FakeServiceManager` |

Source: `ocserv/service/*`

---

### `vpn/health/`
Readiness checks and network prerequisite inspection. The only `vpn/` sub-package that imports other sub-packages.

| File | Contents |
|---|---|
| `readiness.go` | `ReadinessChecker`, `OcservReadiness`, `ReadinessConfig` |
| `network.go` | `NetworkInspector`, `NetworkPrerequisites`, `PrerequisiteCheck` |
| `readiness_test.go` | Existing readiness tests |

Source: `ocserv/readiness.go`, `ocserv/network_inspector.go`

Imports within `vpn/`:
- `vpn/exec/` — for command execution
- `vpn/service/` — readiness checks service status
- `vpn/config/` — network inspector uses `ParseConfigLine`

---

## monitor/ Package Detail

Merges `internal/monitoring/` and `internal/metrics/` into a single package. No public API changes — all types, interfaces, and constructors keep their names.

| File | Contents | Source |
|---|---|---|
| `types.go` | `MetricName`, `Point`, `Series`, `Store` interface | `metrics/types.go` |
| `tsdb.go` | `TSDBStore` | `metrics/tsdb_store.go` |
| `memory.go` | `MemoryStore` | `metrics/memory_store.go` |
| `collectors.go` | `CPUCollector`, `MemoryCollector`, `InterfaceCollector`, `VPNCollector` | `monitoring/collectors.go` |
| `service.go` | `CollectorService` (poll loop) | `monitoring/collector_service.go` |
| `discovery.go` | interface and VPN interface auto-discovery | `monitoring/iface_discovery.go`, `monitoring/vpn_discovery.go` |
| `session_collector.go` | `SessionCollector`, `SessionStatusProvider` interface | `monitoring/session_collector.go` |

Import path changes for callers:
- `internal/metrics` → `internal/monitor`
- `internal/monitoring` → `internal/monitor`

---

## audit/ Collapse

`audit/audit.go`, `audit/sqlite.go`, and `audit/sqlite_test.go` remain as-is. No merge needed — the package is already correctly sized. The `sqlite.go` file holds the `SQLiteRecorder` implementation; since `audit` defines the `Recorder` interface and `store` implements storage, the existing cycle-break via `AuditStore` interface is preserved unchanged.

---

## Dependency Flow (updated)

```
server    → app, auth, rbac, config, monitor, vpn/sessions, store
app       → store, audit, auth, rbac, config, vpn/sessions, vpn/users,
            vpn/config, vpn/service, vpn/health, monitor, logs, validation
audit     → store (via AuditStore interface)
store     → db
vpn/health   → vpn/exec, vpn/service, vpn/config
vpn/sessions → vpn/exec, validation
vpn/users    → vpn/exec
vpn/config   → vpn/exec
vpn/service  → vpn/exec
monitor      → vpn/sessions (via SessionStatusProvider interface)
logging      → (zerolog only)
validation, sanitize, rbac → (stdlib only)
```

No cycles. `vpn/exec/` is the only shared leaf within `vpn/`.

---

## Files Deleted

| File | Reason |
|---|---|
| `internal/ocserv/cmd_compat.go` | Shim replaced by direct `vpn/exec/` imports |
| `internal/ocserv/config_service_compat.go` | Shim replaced by direct `vpn/config/`, `vpn/service/` imports |
| `internal/ocserv/parse/cidr.go` | Inlined into `vpn/config/validator.go` |
| `internal/ocserv/parse/sanitize.go` | Moved into `vpn/sessions/parse.go` |
| All `internal/ocserv/parse/*.go` | Merged into `vpn/sessions/parse.go` and `vpn/sessions/types.go` |
| All `internal/metrics/*.go` | Merged into `internal/monitor/` |

---

## Import Path Migration

| Old import path | New import path |
|---|---|
| `ocserv_panel/backend/internal/ocserv` | `ocserv_panel/backend/internal/vpn/sessions` |
| `ocserv_panel/backend/internal/ocserv/cmd` | `ocserv_panel/backend/internal/vpn/exec` |
| `ocserv_panel/backend/internal/ocserv/config` | `ocserv_panel/backend/internal/vpn/config` |
| `ocserv_panel/backend/internal/ocserv/parse` | `ocserv_panel/backend/internal/vpn/sessions` |
| `ocserv_panel/backend/internal/ocserv/service` | `ocserv_panel/backend/internal/vpn/service` |
| `ocserv_panel/backend/internal/metrics` | `ocserv_panel/backend/internal/monitor` |
| `ocserv_panel/backend/internal/monitoring` | `ocserv_panel/backend/internal/monitor` |

---

## What Does Not Change

- All public type names, interface method signatures, constructor signatures
- `internal/store/` — shape, files, and SQL unchanged
- `internal/app/` — service names and method signatures unchanged; only import paths updated
- `internal/server/` — handler names unchanged; only import paths updated
- `internal/auth/`, `internal/rbac/`, `internal/config/`, `internal/db/`, `internal/logs/`, `internal/validation/`, `internal/sanitize/`, `internal/logging/` — fully unchanged
- `frontend/`, `migrations/`, `deploy/`, `docs/` — untouched
- All test logic — tests move with their source files; no test rewriting

---

## Migration Strategy

Move in package dependency order — leaves first, root last:

1. `vpn/exec/` — no vpn deps, safe to move first
2. `vpn/sessions/` — depends only on `vpn/exec/`
3. `vpn/users/` — depends only on `vpn/exec/`
4. `vpn/config/` — depends only on `vpn/exec/`, inline cidr helpers
5. `vpn/service/` — depends only on `vpn/exec/`
6. `vpn/health/` — depends on `vpn/exec/`, `vpn/service/`, `vpn/config/`
7. `monitor/` — merge `metrics/` + `monitoring/`
8. Delete old packages (`ocserv/`, `metrics/`, `monitoring/`)
9. Update import paths in `app/`, `server/`, `cmd/`, `audit/`
10. Build + test

Each step is independently compilable. The old packages are deleted only after all callers are updated.

---

## Success Criteria

- `go build ./...` passes with zero errors
- `go test ./...` passes with all existing tests green
- No compat shim files remain
- No import of `internal/ocserv`, `internal/metrics`, or `internal/monitoring` anywhere
- `vpn/` sub-packages have zero cross-imports except through `vpn/exec/` and the `vpn/health/` → `vpn/service/` + `vpn/config/` allowed edges
