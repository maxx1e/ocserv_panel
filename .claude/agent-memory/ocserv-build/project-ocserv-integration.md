---
name: ocserv-integration-completion
description: ocserv integration tasks 1-13 completed 2026-05-04, including all backend adapters, service orchestrator, HTTP handlers, frontend types, and deploy files
type: project
---

All 13 ocserv integration tasks completed as of 2026-05-04. The branch is master.

**Why:** Replace the NoopManager stub and hardcoded "not_configured" dashboard status with real ocserv integration.

**How to apply:** All new adapters, services, and handlers are in place. Future work can build on these interfaces without touching the wire-up in main.go.

## Completed tasks (commits)

| Task | Commit | Description |
|------|--------|-------------|
| 1 | a2756a7 | Config & binary path extensions |
| 2 | 8dc3c1d | CommandRunner interface |
| 3 | 9e38470 | RBAC permissions for ocserv |
| 4 | ebc9f37 | Expand Manager interface + FakeManager |
| 5 | 1ae438f | ReadinessChecker |
| 6 | c573ec6 | ServiceManager + SystemdManager + FakeServiceManager |
| 7 | 4aa5f74 | OcctlAdapter |
| 8 | c9a0838 | PasswdManager + OcpasswdAdapter + FakePasswdManager |
| 9 | 222f297 | ConfigValidator |
| 10 | 470bf2c | OcservService orchestrator + dashboard readiness integration |
| 11 | 5849c5c | HTTP handlers for ocserv API + frontend api.ts types |
| 12 | feedbd9 | Deploy files: systemd unit, sudoers rules, HAProxy snippet |
| 13 | verified | All 12 packages pass, binary builds, go vet clean |

## Key files

- `backend/internal/ocserv/` — all adapter interfaces and implementations
- `backend/internal/app/ocserv_service.go` — service orchestrator with RBAC + audit
- `backend/internal/server/ocserv_handlers.go` — HTTP endpoints
- `frontend/src/lib/api.ts` — OcservReadiness, OcservSession, OcservUser, OcservConfigIssue types + API functions
- `deploy/` — systemd unit, sudoers.d, haproxy.cfg.snippet

## Architecture notes

- `DashboardService` now accepts `*ocserv.ReadinessChecker` (nil-safe), wired with real checker in production
- `NewAppRouter` takes `*app.OcservService` as 5th parameter
- Password always piped via stdin to ocpasswd, never in command args
- All handlers audit denied + privileged operations via `audit.Recorder`
