# Backend Code Quality Review Report

**Date:** 2026-05-06  
**Scope:** Full backend (`/opt/ocserv_panel/backend/`) — 20,754 lines of Go  
**Build Status:** ✅ Compiles cleanly  
**Test Status:** ✅ All 12 packages pass (0 failures)

---

## Executive Summary

The backend is well-structured with strong fundamentals: parameterized SQL everywhere, no shell injection vectors, proper PBKDF2 password hashing, atomic config writes, and good RBAC coverage. However, the review identified **3 Critical**, **9 Important**, and **14 Minor** issues across dead code, implementation gaps, security concerns, and test coverage.

**Top priorities for immediate action:**
1. Input validation for ocserv usernames/passwords (corruption risk)
2. Login rate limiting (brute-force risk)
3. Binary path allow-listing for editable settings (arbitrary execution risk)
4. Remove dead code from `store.go` and `settings_service.go`

---

## Section 1: Dead Code & Unused Exports

### High Confidence — Should Remove

| # | Symbol | File | Reason |
|---|--------|------|--------|
| 1 | `SettingDefinition` struct + `GetSettingDefinitions()` | `store/store.go:305-348` | Never called; leftover from DB-based settings design |
| 2 | `SettingsService.GetAll()` + `Setting` type + `knownSettingKeys` | `app/settings_service.go:28-152` | Superseded by `GetAllSections`; only tests call it |
| 3 | `SettingsService.GetRetentionDays()` | `app/settings_service.go:76` | Duplicated independently in `housekeeping.go:61` |
| 4 | `PermissionSettingsExport`, `PermissionSettingsBackup` | `rbac/rbac.go:24-25` | Defined but never checked anywhere |
| 5 | `PermissionManageUsers`, `PermissionManageConfig`, `PermissionManageService` | `rbac/rbac.go:18-20` | Superseded by granular `PermissionOcserv*` permissions |
| 6 | `User` struct | `ocserv/client.go:31-35` | Never referenced; `PasswdUser` is used instead |
| 7 | `Manager.ListUsers()` interface method | `ocserv/client.go:62` | Unreachable from service layer (uses `PasswdManager.ListUsers`) |
| 8 | `NewNetworkInspector()` (zero-arg) | `ocserv/network_inspector.go:37-39` | Never called; production uses `NewNetworkInspectorWithConfig` |
| 9 | `NewRouter()` | `server/server.go:29` | Test-only; production uses `NewAppRouter()` |

### Low Confidence — Needlessly Exported (Internal Package)

`DefaultPort`, `PasswordHashAlgorithm`, `ErrEmptyDSN`, `ErrInvalidSample`, `ErrStoreClosed`, `HealthResponse` — all exported but only used within their own `internal/` packages.

### Duplicate Code

- **Metadata sanitization** is implemented identically in both `audit/sqlite.go:40-83` AND `store/store.go:502-545`. Data is double-sanitized on the write path. Should consolidate to a single shared implementation.

---

## Section 2: Implementation Gaps & Logic Errors

### Critical

| # | Issue | File | Impact |
|---|-------|------|--------|
| 1 | **Latent data race on `Catalog.metrics` map** — no mutex. Safe today (writes only at startup before server starts), but will panic if `SetMetricState` is ever called post-startup (e.g., dynamic VPN interface discovery). | `metrics/types.go:100-108` | Crash if called concurrently |

### Important

| # | Issue | File | Impact |
|---|-------|------|--------|
| 2 | **Adapters don't pick up config changes** — `OcctlAdapter`, `OcpasswdAdapter`, `ConfigFileManager`, `ConfigValidator` bake in paths at startup. If admin changes paths via Settings UI, they require a restart. But `ReadinessChecker` uses live config. Inconsistency confuses users. | `main.go:93-110` | Settings appear saved but don't take effect |
| 3 | **Double-call pattern on auth failure** — When `authenticate()` returns false, handlers still call the service with an empty `Principal{}`. Creates audit log pollution and wasted computation. | `server/server.go:363-395` | Phantom "denied" audit records |
| 4 | **No audit logging for restart handler** — `handleRestart` checks RBAC but produces no audit record. Currently a stub, but gap will persist when implemented. | `server/settings_handlers.go:73-89` | No forensic trace of restart attempts |
| 5 | **`PurgeHistoricalBefore` loads all rows into memory** — Selects all historical sessions, iterates in Go, then deletes one-by-one. Could OOM on large datasets. | `store/store.go` | Memory exhaustion on purge |
| 6 | **`occtl` time parsing silently ignores errors** — `ConnectedAt` parsed with `time.RFC3339` but error discarded. Wrong format = zero time = massive bogus Duration. | `ocserv/occtl_adapter.go:147` | Nonsensical duration in UI |
| 7 | **Audit error handling inconsistency** — `GetAllSections` returns audit write errors to caller (→ 500). `UpdateSetting` silently discards them. Same subsystem, different behavior. | `app/settings_service.go` | Transient audit failures may break reads but not writes |

### Minor

| # | Issue | File |
|---|-------|------|
| 8 | `handleRestart` always returns "not yet implemented" but 200 OK | `server/settings_handlers.go` |
| 9 | `GET /api/settings/typo` matches the prefix route and returns 405 | `server/server.go:215-216` |
| 10 | `Authorizer` is a struct not an interface; copied 3x in main.go | `main.go` / `rbac/rbac.go` |

---

## Section 3: Security Issues

### Critical / High

| # | Severity | Issue | File | Exploit Scenario |
|---|----------|-------|------|------------------|
| 1 | **High** | **No login rate limiting** — Unlimited password guesses at ~100/min given PBKDF2 cost. No lockout. | `server.go:317`, `auth_service.go:110` | Brute-force attack on known username |
| 2 | **High** | **No username validation before ocpasswd** — Colons, newlines, null bytes pass through. Password with newlines confuses stdin protocol. | `ocserv_handlers.go:311`, `passwd_adapter.go:97` | Corrupted ocpasswd file; auth bypass |
| 3 | **High** | **Binary path settings editable without allow-list** — Owner can set `ocpasswd_bin_path` to `/tmp/evil`. Next user creation executes it with password on stdin. | `settings_config_apply.go:34-46` | Arbitrary code execution as panel user |

### Medium

| # | Issue | File | Risk |
|---|-------|------|------|
| 4 | **`systemd_unit_name` editable** — Owner can point it at `sshd`, then stop it. | `service_systemd.go:30` | DoS other services |
| 5 | **Missing `ReadTimeout`/`WriteTimeout`** on `http.Server` — Slow-body DoS | `main.go:177` | Goroutine exhaustion |
| 6 | **Session IDs not format-validated** before passing to `occtl` | `occtl_adapter.go:66` | Flag injection via `--` prefix |
| 7 | **DSN logged to stdout** — Safe today but bad pattern | `main.go:38` | Future credential leak |
| 8 | **Log path editable → arbitrary file read** — Admin changes log path to `/etc/shadow`, tail endpoint serves it | `log_tail_service.go:85-91` | Reads any file as panel user |
| 9 | **No CSRF middleware** — `security.csrf_enabled` exists in config but nothing enforces it | N/A | Cross-site mutation via JavaScript |

### Low

- Session validation failures not audited (F1)
- Config read operations not audited (F2)
- `configError` field exposes internal paths to Viewers (C3)
- Filesystem paths in log tail response (D4)
- File permission mode inconsistency between `config.Save()` (0600) and `ConfigFileManager.atomicWrite()` (0640)

---

## Section 4: Test Coverage Gaps

### Coverage by Package

| Package | Coverage | Risk |
|---------|----------|------|
| `internal/server` | 65.5% | ⚠️ **Highest risk** — mutation handlers untested |
| `internal/ocserv` | 78.3% | ⚠️ `OcpasswdAdapter` completely untested |
| `internal/config` | 63.4% | ⚠️ Boundary values untested |
| `internal/store` | 70.2% | Some repo methods lack tests |
| `internal/app` | 77.9% | `UpdateOcservUserPassword`, log tail success untested |
| `cmd/ocserv-panel` | 19.5% | Acceptable (main startup) |
| `internal/rbac` | 100% | ✅ |
| `internal/monitoring` | 88.9% | ✅ |
| `internal/audit` | 82.6% | ✅ |

### Key Gaps

1. **No HTTP handler tests** for settings endpoints, ocserv endpoints, or config management endpoints
2. **No RBAC deny-path tests at HTTP layer** — only service-layer denial is tested
3. **`OcpasswdAdapter`** — zero test coverage with `FakeCommandRunner`
4. **Config validation boundary values** — min/max port, edge cases untested
5. **No test for the double-call auth pattern** — behavior on unauthenticated requests untested at handler level

---

## Section 5: Strengths

These are things done well that should be preserved:

- ✅ All SQL is parameterized — zero injection risk
- ✅ `exec.Command` with separate args — zero shell injection
- ✅ PBKDF2-SHA256 with 600K iterations + constant-time compare
- ✅ 32-byte crypto/rand session tokens, stored as SHA-256 hashes
- ✅ Dummy password verification prevents username enumeration
- ✅ Atomic temp-file-rename with flock for config writes
- ✅ `json.MaxBytesReader` (1 MiB) on all request bodies
- ✅ Security headers + HttpOnly + SameSite=Lax cookie
- ✅ Clean separation of concerns (adapters, services, handlers)
- ✅ Fakes for all system interactions (no live ocserv needed for tests)
- ✅ Zero `TODO`/`FIXME`/`HACK` comments — code is clean

---

## Recommended Action Plan

### Phase 1 — Security Fixes (Immediate)

1. Add username validation: `^[a-zA-Z0-9._-]{1,64}$`
2. Add password validation: reject newlines, limit to 128 chars
3. Add login rate limiter (per-IP, 5 attempts/minute)
4. Add allow-list for binary path settings (must exist, must be in `/usr/bin`, `/usr/local/bin`, `/usr/sbin`, `/usr/local/sbin`)
5. Add allow-list for `systemd_unit_name` (must match `ocserv*`)
6. Add `ReadTimeout: 30s`, `WriteTimeout: 60s` to HTTP server
7. Validate log paths are within allowed directories

### Phase 2 — Dead Code Removal

1. Remove `GetSettingDefinitions`, `SettingDefinition` from store
2. Remove `GetAll`, `Setting`, `knownSettingKeys` from settings_service
3. Remove `GetRetentionDays` from settings_service (keep housekeeping's copy)
4. Remove unused RBAC permissions (5 constants)
5. Remove `User` struct and `Manager.ListUsers` from ocserv/client.go
6. Remove `NewNetworkInspector` zero-arg constructor
7. Consolidate duplicate metadata sanitization

### Phase 3 — Test Coverage

1. Add HTTP handler tests for settings + ocserv endpoints
2. Add `OcpasswdAdapter` tests with `FakeCommandRunner`
3. Add RBAC deny-path tests at HTTP layer
4. Add config validation boundary tests
5. Create shared HTTP test helper

### Phase 4 — Implementation Polish

1. Remove double-call pattern in auth failure handlers
2. Add mutex to `Catalog` for thread safety
3. Fix audit error handling consistency
4. Add audit logging for restart handler
5. Fix `PurgeHistoricalBefore` to use SQL-level DELETE
6. Handle `occtl` time parse errors gracefully

---

*Report generated by 4 specialized review agents analyzing the full backend codebase.*
