# Settings Persistence & Dashboard Status Design

**Date:** 2026-05-04  
**Status:** Approved  
**Scope:** Settings UI validation, config-source persistence to TOML, restart-required tracking, dashboard ocserv status reflecting path changes

---

## Problem Statement

Two related gaps exist in the current implementation:

1. **Settings persistence is incomplete.** `UpdateSetting` stores all settings — regardless of `SettingDef.Source` — into SQLite. Config-source settings (Installation, Service, Web, Security, Database, Monitoring sections) are never written back to the TOML config file. As a result, changes made via the Settings UI to these fields are silently dropped on process restart.

2. **`restart_required` / `pending_changes` are hardcoded.** Both `GetAllSections` and `handleRestartStatus` always return `restart_required: false` and `pending_changes: []`. The `RestartBanner` in the UI is therefore permanently inert.

3. **Dashboard status does not reflect path changes.** `ReadinessChecker` is constructed once at startup with a frozen path snapshot from `config.Load()`. Changing `installation.ocserv_bin_path` or similar in the Settings UI writes to SQLite (and soon to TOML), but the running checker still probes the old paths until the process is restarted. The dashboard "ocserv status" card never reflects the update.

---

## Architecture Decision

- **`SettingSourceConfig` settings** → persist to TOML file (atomic write with backup via `config.Save()`). Service restart required to apply to the running process.
- **`SettingSourceDatabase` settings** → persist to SQLite. Take effect live, no restart needed.
- **No new libraries.** `github.com/pelletier/go-toml/v2` (already a direct dependency) and the existing `config.Save()` are sufficient. Viper is not introduced — it adds no value here and is significantly heavier.

---

## Design

### 1. Settings Persistence Layer

#### `SettingsService` — config path injection

`SettingsService` already holds `*config.Config` (the startup snapshot). It gains one additional field: `configPath string` — the path to the TOML file on disk. This allows `UpdateSetting` to call `config.Load()` + patch + `config.Save()` for config-source settings.

Constructor signature change:
```go
func NewSettingsService(
    cfg *config.Config,
    configPath string,          // ← new
    s *store.Store,
    recorder audit.Recorder,
    authorizer rbac.Authorizer,
) *SettingsService
```

#### `UpdateSetting` — dispatch by source

```
if def.Source == SettingSourceConfig:
    current, err := config.Load(configPath)
    if err: return error
    if err := applySettingToConfig(&current, key, value); err: return error
    if err := config.Validate(current); err: return error
    if err := config.Save(current, configPath); err: return error
    // Do NOT write to SQLite for config-source settings
else:  // SettingSourceDatabase
    settingsStore.SetSetting(key, value)   ← unchanged
```

#### `applySettingToConfig(cfg *config.Config, key, value string) error`

A new function in the `app` package (or `config` package — see boundary note below). Maps dot-notation setting keys to struct fields via a plain switch/case. All config-source keys are known at compile time from the registry, so no reflection is needed.

```go
switch key {
case "installation.ocserv_bin_path":
    cfg.Installation.OcservBinPath = value
case "installation.occtl_bin_path":
    cfg.Installation.OcctlBinPath = value
case "service.systemd_unit_name":
    cfg.Service.SystemdUnitName = value
case "web.port":
    port, err := strconv.Atoi(value)
    // ...
    cfg.Web.Port = port
// ... all other config-source keys
default:
    return fmt.Errorf("%w: %s is not a config-source setting", ErrInvalidInput, key)
}
```

Type coercion (string→int for number fields, string→bool for toggles) happens inside `applySettingToConfig`. The existing `validateSettingValue` already validates the string form before this function is called, so coercion should not fail for valid inputs.

**Boundary note:** `applySettingToConfig` lives in the `app` package alongside `SettingsService`. It imports `config.Config` but does not need to be inside the `config` package. This keeps the config package as a pure data/IO layer.

#### `GetAllSections` — reading config-source values from TOML

Currently config-source settings return `d.Default`. After the change: for `SettingSourceConfig` settings, read from the live TOML file. `config.Load(configPath)` is called once per `GetAllSections` request and the result is used for all config-source fields in that request.

A helper `configValueForKey(cfg config.Config, key string) string` mirrors `applySettingToConfig` in reverse — extracts a field value as a string for a given key.

#### `restart_required` / `pending_changes` computation

The startup config snapshot is held in `SettingsService.startupCfg` (the `*config.Config` passed at construction, copied by value). On each `GetAllSections` or `handleRestartStatus` call:

```
currentCfg, _ := config.Load(configPath)
pending := []string{}
for each config-source SettingDef key:
    startupVal := configValueForKey(startupCfg, key)
    currentVal := configValueForKey(currentCfg, key)
    if startupVal != currentVal:
        pending = append(pending, key)
restartRequired := len(pending) > 0
```

This is deterministic and requires no additional state — the comparison is always startup snapshot vs current TOML on disk.

---

### 2. Dashboard Status Reflecting Path Changes

#### `ReadinessChecker` — config-loader function instead of frozen snapshot

`ReadinessChecker` currently holds `cfg ReadinessConfig` (frozen at construction). It is changed to hold `cfgFn func() ReadinessConfig` — a function called on each `Check()` invocation. This means path changes written to TOML take effect on the next dashboard load or 15s auto-refresh without a process restart.

```go
// Before
type ReadinessChecker struct {
    cfg    ReadinessConfig
    runner CommandRunner
    svc    ServiceManager
}

// After  
type ReadinessChecker struct {
    cfgFn  func() ReadinessConfig
    runner CommandRunner
    svc    ServiceManager
}

func (c *ReadinessChecker) Check(ctx context.Context) (*OcservReadiness, error) {
    cfg := c.cfgFn()   // read current config on each call
    // ... rest unchanged, using cfg.OcservBinPath etc.
}
```

#### Wiring in `main.go`

```go
readinessChecker := ocserv.NewReadinessChecker(
    func() ocserv.ReadinessConfig {
        cfg, _ := config.Load(configPath)
        return ocserv.ReadinessConfig{
            OcservBinPath:   cfg.Installation.OcservBinPath,
            OcctlBinPath:    cfg.Installation.OcctlBinPath,
            OcpasswdBinPath: cfg.Installation.OcpasswdBinPath,
            ConfigPath:      cfg.Installation.OcservConfigPath,
            SocketPath:      cfg.Installation.OcservSocketPath,
            UnitName:        cfg.Service.SystemdUnitName,
        }
    },
    runner,
    svcManager,
)
```

#### Scope limit — session/passwd adapters

`OcctlAdapter` and `OcpasswdAdapter` also hold frozen paths from startup config. These are **not** changed to live-read. They are invoked only on explicit user actions (list sessions, create user), not on dashboard load. The user is prompted to restart after changing installation paths; after restart, all adapters pick up the new values. Introducing live-read into adapters that run under request context adds concurrency complexity without meaningful UX benefit.

---

### 3. Settings UI Validation & Feedback

**No new frontend components are needed.** The existing infrastructure is complete:
- `RestartBanner` — displays when `restart_required: true`, shows `pending_changes`
- `SettingField` — surfaces per-row save errors via `onUpdate` rejection
- Restart polling loop — already implemented, polls `GET /api/settings/restart-status`

**One small frontend adjustment:** After `handleUpdate` succeeds, the returned `SettingDef` includes `requires_restart: true` for config-source settings. The `SettingRow` component should display a subtle per-row indicator (e.g. small amber "restart required" chip) so the user sees which saved changes are pending restart vs already live. This is a cosmetic addition to `SettingRow.svelte`.

**Error flow:** If `config.Save()` fails (e.g. permission denied on TOML file), `UpdateSetting` returns an error. The HTTP handler maps it to a 500. The frontend `SettingField` already shows errors per-row. No new UI error handling needed.

---

### 4. Testing Strategy

#### Backend unit tests — `SettingsService`

| Test | Assertion |
|------|-----------|
| `UpdateSetting` config-source key | TOML file is written; SQLite is not touched |
| `UpdateSetting` database-source key | SQLite is written; TOML file is not touched |
| `UpdateSetting` invalid value | Rejected before any write |
| `UpdateSetting` TOML write failure | Error propagated, no partial state |
| `GetAllSections` | Config-source values read from TOML, not defaults |
| `GetAllSections` with changed TOML | `restart_required: true`, correct `pending_changes` |
| `GetAllSections` with unchanged TOML | `restart_required: false`, empty `pending_changes` |

#### Backend unit tests — `applySettingToConfig`

| Test | Assertion |
|------|-----------|
| Each config-source key | Maps to correct struct field |
| Round-trip | apply → marshal → unmarshal → field matches original value |
| Unknown key | Returns `ErrInvalidInput` |
| Number field coercion | String "8443" → `cfg.Web.Port = 8443` |
| Bool field coercion | String "true" → `cfg.Service.StartOnBoot = true` |

#### Backend unit tests — `ReadinessChecker`

| Test | Assertion |
|------|-----------|
| `cfgFn` called on each `Check()` | Two calls with different `cfgFn` return values use different paths |
| Path change between calls | New binary path picked up on second `Check()` |

#### Backend unit tests — `handleRestartStatus`

| Test | Assertion |
|------|-----------|
| TOML matches startup | Returns `restart_required: false` |
| TOML differs from startup | Returns `restart_required: true` with changed keys |

#### Test infrastructure

- `SettingsService` tests use `t.TempDir()` for TOML file — real filesystem, no mock
- `ReadinessChecker` tests use `FakeCommandRunner` (already exists) and closure-controlled `cfgFn`
- No live `ocserv`, no live systemd

#### Frontend

No new component tests. The `RestartBanner` and `SettingField` are already exercised. The `SettingRow` amber chip is a cosmetic addition — verified visually.

---

## Files Affected

### Backend
| File | Change |
|------|--------|
| `backend/internal/app/settings_service.go` | Add `configPath`, update `UpdateSetting` dispatch, add `GetAllSections` TOML read + restart diff |
| `backend/internal/app/settings_config_apply.go` | New: `applySettingToConfig` and `configValueForKey` |
| `backend/internal/app/settings_service_test.go` | Expand with config-source persistence tests |
| `backend/internal/app/settings_config_apply_test.go` | New: round-trip and mapping tests |
| `backend/internal/ocserv/readiness.go` | Change `cfg ReadinessConfig` → `cfgFn func() ReadinessConfig` |
| `backend/internal/ocserv/readiness_test.go` | Add cfgFn call-time tests |
| `backend/internal/server/settings_handlers.go` | Wire `handleRestartStatus` to real restart diff |
| `backend/cmd/ocserv-panel/main.go` | Pass `configPath` to `NewSettingsService`; wire `cfgFn` closure to `NewReadinessChecker` |

### Frontend
| File | Change |
|------|--------|
| `frontend/src/lib/components/ui/SettingRow.svelte` | Add per-row amber "restart required" chip when `requires_restart` is true and setting was just saved |

---

## Explicitly Out of Scope

- Hot-swapping `OcctlAdapter` / `OcpasswdAdapter` paths at runtime
- Staged pending-changes commit flow (Approach B)
- Viper or any new config library
- Changes to TOML file format or schema
- Restart flow implementation changes (restart via systemd is already wired in `OcservService.RestartService`)
