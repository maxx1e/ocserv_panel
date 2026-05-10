# Architecture Simplification Design

**Date:** 2026-05-10  
**Status:** Approved  
**Scope:** Backend (Go) + Frontend (SvelteKit)

---

## Problem Statement

The current codebase has three structural problems that increase complexity without adding value:

1. **Thin app service layer.** `internal/app/` contains 29 files that are mostly RBAC+audit wrappers. The pattern — authenticate, check permission, delegate to one downstream call, record audit event — is repeated independently in `MonitoringService`, `LogTailService`, `HealthService`, and others. No centralized enforcement means inconsistent audit coverage (e.g., `LogTailService` has no audit recording at all).

2. **Flat `internal/ocserv/` package.** 42 files in one package mix parsers, command adapters, config management, service lifecycle, input validation, and test fakes. Navigating to "where is the config validator?" requires scanning all 42 files.

3. **Monolithic `store.go` and large frontend files.** `internal/store/store.go` (647 lines) conflates admin user CRUD, audit event storage, historical session analytics, and settings. `frontend/routes/+page.svelte` is 31KB. `frontend/lib/api.ts` is 29KB.

---

## Goals

- Reduce navigational complexity without changing observable behavior.
- Collapse thin wrapper files that add no domain logic.
- Make RBAC enforcement consistent and centrally auditable.
- Keep all tests green throughout; only import paths change in test files.

## Non-Goals

- Changing any API contract or wire format.
- Changing any business logic or RBAC rules.
- Replacing SQLite, tsdb, or any dependency.
- Introducing new features.

---

## Change 1 — RBAC Middleware + App Service Layer Collapse

### Current State

Each service method in `internal/app/` independently implements:

```go
func (s *Service) DoThing(ctx, principal, ...) error {
    if !principal.Authenticated() { return ErrUnauthorized }
    if !s.authorizer.Allowed(roles(principal), rbac.PermXxx) { return ErrForbidden }
    // 2-3 lines of actual work
    s.recorder.Record(ctx, ...)
}
```

Services with this pattern and no other domain logic: `MonitoringService`, `LogTailService`, `HealthService`, `HousekeepingService`.

### Target State

**Add `RequirePermission(perm rbac.Permission) func(http.Handler) http.Handler` middleware** in `internal/server/middleware.go`:

- Extracts the session token from the request.
- Validates it against the store.
- Checks the principal has the required permission.
- Injects the validated principal into the request context via `auth.WithPrincipal(ctx, principal)`.
- Returns `401` or `403` if checks fail.
- Records an audit event for the authorization decision (replaces per-service read-path audit).

**Router wiring example:**

```go
r.With(a.RequirePermission(rbac.PermissionReadDashboard)).Get("/api/dashboard/metrics", a.metricsHandler)
```

**Collapse thin services** — services that had no logic beyond the auth+audit pattern are removed. Their HTTP handlers call the downstream packages directly:

| Removed service | Handler calls instead |
|---|---|
| `MonitoringService` | `metrics.Store` directly |
| `LogTailService` | `logs.Reader` directly |
| `HealthService` | `health.Check(cfg, store)` function in new `internal/health/` |
| `HousekeepingService` (`housekeeping.go`) | `store.StartHousekeeper(interval, recorder)` method |

**Services retained** (have real domain logic beyond auth+audit):

- `OcservService` — orchestrates 5 downstream dependencies, has mutation logic
- `AdminUserService` — user lifecycle, password hashing, bootstrap owner
- `SettingsService` — settings registry with restart detection
- `ConfigManagementService` — config backup/restore/validate pipeline
- `DashboardService` — aggregates metrics, health, and session data into a single dashboard snapshot (multi-source fan-in, not a single-call passthrough)
- `AuditLogService` — pagination, export formatting, redaction

**`internal/app/` file count:** 29 → ~12.

### Audit Consistency Fix

`LogTailService` currently has no audit recording. After the refactor, the middleware records all authenticated `GET /api/logs/tail` requests. Mutation operations (session disconnect, user create/delete, config apply) keep per-handler audit with structured metadata.

---

## Change 2 — `internal/ocserv/` Sub-Package Split

### Current State

42 flat files in one package covering: occtl output parsers, command runner, occtl adapter, passwd adapter, config file I/O, config validator, config reload, restart detection, service lifecycle, network inspection, input validation, and test fakes.

### Target State

```
internal/ocserv/
├── client.go          ← Manager interface, Session/User types, FakeManager
├── passwd.go          ← PasswdManager interface, PasswdUser type
├── passwd_adapter.go  ← OcpasswdAdapter implementation
├── passwd_fake.go     ← FakePasswdManager test double
├── readiness.go       ← ReadinessChecker (used by OcservService and server startup)
├── network_inspector.go ← Network prerequisite checks (depends on config + cmd)
│
├── cmd/               ← Command execution infrastructure
│   ├── runner.go      ← CommandRunner interface + OSCommandRunner
│   └── fake.go        ← FakeCommandRunner test double
│
├── parse/             ← occtl output parsers (pure functions, no I/O)
│   ├── sessions.go    ← ParseSessionsList (was occtl_sessions_parser.go)
│   ├── users.go       ← ParseUsersList (was occtl_users_parser.go)
│   ├── user.go        ← ParseUser (was occtl_user_parser.go)
│   ├── bans.go        ← ParseBansList (was occtl_bans_parser.go)
│   ├── routes.go      ← ParseRoutesList (was occtl_routes_parser.go)
│   ├── status.go      ← ParseStatus (was occtl_status_parser.go)
│   └── cidr.go        ← CIDRToMask utility (was cidr_convert.go)
│
├── config/            ← Config file management
│   ├── file.go        ← ConfigFile read/write (was config_file.go)
│   ├── validator.go   ← ConfigValidator (was config_validator.go)
│   ├── reload.go      ← Reload signaling (was config_reload.go)
│   └── restart.go     ← RestartDetector (was config_restart_detector.go)
│
└── service/           ← Systemd service lifecycle
    ├── manager.go     ← ServiceManager interface, ServiceStatus enum
    ├── systemd.go     ← SystemdManager implementation
    └── fake.go        ← FakeServiceManager test double
```

**`validate.go`** (username/password/IP validation utilities) moves to **`internal/validation/`** as a new shared package — these validators are not ocserv-specific and may be used by admin user flows.

### Import Changes

| Old import | New import |
|---|---|
| `ocserv.CommandRunner` | `ocserv/cmd.CommandRunner` |
| `ocserv.FakeCommandRunner` | `ocserv/cmd.FakeCommandRunner` |
| `ocserv.ParseSessions(...)` | `ocserv/parse.ParseSessions(...)` |
| `ocserv.ConfigFile` | `ocserv/config.ConfigFile` |
| `ocserv.ConfigValidator` | `ocserv/config.Validator` |
| `ocserv.ServiceManager` | `ocserv/service.Manager` |
| `ocserv.ValidateUsername(...)` | `validation.Username(...)` |

`ocserv.Manager`, `ocserv.Session`, `ocserv.FakeManager`, `ocserv.PasswdManager` remain at the root — no import change for callers of the primary interface.

---

## Change 3 — `internal/store/` File Split

### Current State

`store.go` (647 lines) mixes four concerns in one file.

### Target State

Same `package store`, same exported types, only file boundaries change. No import changes anywhere in the codebase.

```
internal/store/
├── db.go         ← Store struct, NewStore(), OpenDB(), migration bootstrap
├── admin.go      ← Admin type, CreateFirstOwner, FindAdminByUsername, CreateAdmin,
│                    UpdateAdmin, DeleteAdmin, ListAdmins, AdminCount,
│                    CreateSession, RevokeSession, FindAdminBySessionTokenHash
├── audit.go      ← AuditEvent type, RecordAudit, RecentAuditEvents
├── history.go    ← HistoricalSession type, HistoricalSessions, AnalyticsSummary,
│                    PurgeHistoricalBefore, StartHousekeeper
└── settings.go   ← GetSetting, SetSetting
```

`StartHousekeeper` in `history.go` replaces `app/housekeeping.go` — it's a method on `*Store` that starts the background purge loop with an injected audit recorder and interval.

---

## Change 4 — Frontend File Splits

### `routes/+page.svelte` (31KB → thin orchestrator + 4 sub-components)

The dashboard page is split into co-located section components. The page itself becomes a thin orchestrator responsible only for data fetching and layout composition.

```
routes/
├── +page.svelte              ← Data fetching + layout grid only (~200 lines)
├── DashboardHealth.svelte    ← Health cards row (HealthSummaryStrip + status)
├── DashboardMetrics.svelte   ← Metric charts section (bandwidth, CPU, memory)
├── DashboardSessions.svelte  ← Active sessions section (donut chart + table)
└── DashboardActivity.svelte  ← Recent audit events / activity feed
```

Props flow down from `+page.svelte` via Svelte 5 `$props`. No shared store needed.

### `lib/api.ts` (29KB → domain modules with re-export index)

```
lib/api/
├── index.ts       ← Re-exports everything; existing `import { ... } from '$lib/api'` unchanged
├── auth.ts        ← login(), logout(), getSetupStatus(), completeSetup()
├── sessions.ts    ← getActiveSessions(), disconnectSession(), getBans(), unbanIP()
├── users.ts       ← getVpnUsers(), createVpnUser(), deleteVpnUser(),
│                     getAdminUsers(), createAdminUser(), updateAdminUser()
├── monitoring.ts  ← getDashboardOverview(), getMetricSeries(), getSessionsDonut()
├── settings.ts    ← getSettings(), updateSetting(), restartService()
├── config.ts      ← getConfig(), validateConfig(), applyConfig(), downloadConfig()
├── audit.ts       ← getAuditLogs(), exportAuditLogs()
└── health.ts      ← getHealth(), getReadiness()
```

The `index.ts` re-export ensures zero changes to any existing import in other files.

---

## Implementation Constraints

- **Tests green throughout.** Only import path updates in test files; no test logic changes.
- **No behavior change.** RBAC rules, API contracts, wire formats, and error codes are identical.
- **Atomic per-change commits.** Each numbered change is one commit (or a small series). Changes are sequenced: store split first (no imports change), then ocserv sub-packages, then app service collapse (depends on middleware), then frontend.
- **Existing `var _ Interface = (*Impl)(nil)` checks** must be preserved in the new file locations.

---

## Sequencing

1. `store/` file split — zero import changes, safest first
2. `internal/validation/` extraction from `ocserv/validate.go`
3. `ocserv/` sub-package split — update all callers' imports
4. RBAC middleware + collapse thin app services — most logic change, but no behavior change
5. Frontend: `lib/api.ts` → `lib/api/` with index re-export
6. Frontend: `+page.svelte` section extraction
7. Full test suite verification

---

## Success Criteria

- `internal/app/` file count drops from 29 to ≤12
- `internal/ocserv/` root has ≤12 files (parsers, adapters, config, cmd in sub-packages)
- `internal/store/` has 5 focused files instead of 1
- `lib/api.ts` replaced by `lib/api/` directory; no other imports change
- `routes/+page.svelte` ≤ 300 lines
- All backend tests pass (`go test ./...`)
- All frontend type checks pass (`pnpm check`)
- No new features introduced
