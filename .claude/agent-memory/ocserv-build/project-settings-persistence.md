---
name: settings-persistence-dashboard-status
description: Settings persistence to TOML + restart_required computation + ReadinessChecker live paths — all completed 2026-05-04
type: project
---

Settings persistence and dashboard status feature — completed 2026-05-04.

**Why:** Config-source settings were silently dropped on restart (only SQLite was written). restart_required was hardcoded false. ReadinessChecker used frozen startup paths.

**What was built:**
- `applySettingToConfig` / `configValueForKey` / `computePendingChanges` in `backend/internal/app/settings_config_apply.go`
- `SettingsService` now dispatches UpdateSetting by source: config → TOML, database → SQLite
- `GetAllSections` reads live TOML for config-source values; computes real restart diff
- `RestartStatus(ctx, principal)` method on SettingsService
- `handleRestartStatus` handler wired to `RestartStatus` in `settings_handlers.go`
- `ReadinessChecker` refactored to hold `cfgFn func() ReadinessConfig` (called per Check)
- Frontend: per-row amber "restart pending" chip in each Setting* component
- Password-type values masked in audit logs via `auditValue()`

**Key commits:** e471c83 → e832b79 (10 commits total)

**How to apply:** When touching settings persistence, ReadinessChecker, or restart-required logic, these files are the authoritative implementations.
