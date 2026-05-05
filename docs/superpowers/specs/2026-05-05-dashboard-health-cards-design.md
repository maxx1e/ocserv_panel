# Dashboard Health Cards & Log Tail Design

**Date:** 2026-05-05  
**Status:** Approved  

---

## Overview

Improve the WebUI dashboard by replacing the top-level "Backend status" and "ocserv status" stat cards with two structured `HealthCard` components. Add a tabbed Audit / Logs section at the bottom of the dashboard. Expose log file paths and the log-read toggle in the Settings UI.

---

## Backend API

### New endpoint: `GET /api/health/system`

- **Auth:** required; same `read:dashboard` RBAC permission as `GET /api/dashboard/summary`
- **Purpose:** Single call that aggregates VPN server state, WebUI panel state, and log file availability
- **Implementation:** Reuses `ReadinessChecker.Check()` for VPN server fields; adds lightweight `os.Stat` probes for DB and config paths; calls `store.AdminCount` for the WebUI user count
- **Does not expose:** raw config values, secrets, or log content

**Response shape:**
```json
{
  "vpnServer": {
    "systemdStatus": "active|inactive|failed|unknown",
    "processRunning": true,
    "socketPresent": true,
    "configPresent": true,
    "configValid": true,
    "configErrors": "",
    "binaryPresent": true,
    "binaryVersion": "ocserv 1.4.2"
  },
  "panel": {
    "backendStatus": "ok",
    "dbReachable": true,
    "dbPath": "/var/lib/ocserv-panel/panel.db",
    "configPresent": true,
    "configPath": "/etc/ocserv-panel/config.toml",
    "webuiUserCount": 2
  },
  "logs": {
    "ocservLogEnabled": true,
    "ocservLogPresent": true,
    "ocservLogPath": "/var/log/ocserv/ocserv.log",
    "panelLogEnabled": true,
    "panelLogPresent": true,
    "panelLogPath": "/var/log/ocserv-panel/panel.log"
  }
}
```

- `vpnServer.systemdStatus` maps from `ServiceManager.Status()` return values
- `panel.dbPath` and `panel.configPath` are taken from live config, not hardcoded
- `logs.*Enabled` reflects the `monitoring.log_file_read_enabled` setting
- `logs.*Present` reflects `os.Stat` on the configured path (only checked when enabled)

### New endpoint: `GET /api/logs/tail?source=ocserv|panel&lines=50`

- **Auth:** required; `read:dashboard` RBAC permission
- **Purpose:** Return the last N lines from a configured log file
- **Parameters:**
  - `source`: `ocserv` or `panel` (required)
  - `lines`: integer 1–50 (default 25, hard cap 50)
- **Implementation:** Reads from the tail of the file using a rolling byte-buffer; never loads the full file into memory
- **Log paths:** Read live from config via the same `cfgFn` closure pattern used in `ReadinessChecker` — path changes take effect immediately without restart

**Response shape:**
```json
{
  "source": "ocserv",
  "enabled": true,
  "present": true,
  "path": "/var/log/ocserv/ocserv.log",
  "lines": ["2026-05-05 10:01:02 ...", "..."]
}
```

- If `enabled=false`: returns `{ "source": "ocserv", "enabled": false, "present": false, "path": "...", "lines": [] }` — HTTP 200, no error
- If `present=false`: returns `{ ..., "enabled": true, "present": false, "lines": [] }` — HTTP 200, no error
- If the file exists but cannot be read (permission error): returns HTTP 200 with `"present": false` and a `"error"` field describing the issue — does not expose the raw OS error string directly

### New service: `HealthService`

A new `app.HealthService` struct owns the `GET /api/health/system` logic:
- Takes `ReadinessChecker`, `dashboardStore` (for `AdminCount`), and a `cfgFn func() config.Config` for live path reads
- `SystemHealth(ctx, principal)` returns a `SystemHealthResult` struct

`GET /api/logs/tail` is handled by a new `LogTailService`:
- Takes `cfgFn func() config.Config` and reads paths + enabled flag from live config
- `TailLog(ctx, principal, source string, lines int)` returns a `LogTailResult` struct
- Uses a `FileReader` interface (same pattern as monitoring collectors) so it is testable without real files

---

## Settings

### New settings keys

Added to the settings registry. All three are config-source (`SettingSourceConfig`) — written to TOML, read back on next request.

| Key | Section | Label | Type | Default | Restart required |
|-----|---------|-------|------|---------|-----------------|
| `installation.ocserv_log_path` | installation | ocserv Log Path | Text | `/var/log/ocserv/ocserv.log` | No |
| `installation.panel_log_path` | installation | Panel Log Path | Text | `/var/log/ocserv-panel/panel.log` | No |
| `monitoring.log_file_read_enabled` | monitoring | Enable Log File Reading | Toggle | `true` | No |

### Config struct additions

```go
// InstallationConfig
OcservLogPath string `toml:"ocserv_log_path"`
PanelLogPath  string `toml:"panel_log_path"`

// MonitoringConfig
LogFileReadEnabled bool `toml:"log_file_read_enabled"`
```

### Settings UI

No new components required. The existing `SettingText` and `SettingToggle` components render these automatically via the settings registry. The Settings page is always fully editable — it has no awareness of whether log files exist. Log file availability is only indicated in the dashboard.

---

## Frontend

### HealthCard component

New `HealthCard.svelte` component. Props:
- `title: string`
- `items: HealthItem[]` where `HealthItem = { label: string; status: 'healthy'|'warning'|'critical'|'unknown'; detail?: string }`
- `lastChecked?: Date`

Visual states (Tailwind):
- `healthy` → `text-emerald-700` + emerald status dot
- `warning` → `text-amber-700` + amber dot
- `critical` → `text-red-700` + red dot  
- `unknown` → `text-slate-500` + slate dot

Each item renders label, coloured dot, status word, and optional `detail` string below it. The card shows a "last checked" relative timestamp in the footer.

### Dashboard changes

**Top section:** The two `CardStats` entries ("Backend status" and "ocserv status") are removed. A new 2-column grid replaces them with two `HealthCard` instances:

**VPN Server card** — items derived from `health.vpnServer`:
- Systemd service: `active` → healthy; `inactive` → warning; `failed` → critical; else → unknown
- Socket / process: `socketPresent` → healthy; else → warning
- Config file: `configPresent && configValid` → healthy; `configPresent && !configValid` → critical (detail = `configErrors`); `!configPresent` → warning

**WebUI Panel card** — items derived from `health.panel`:
- Backend: always healthy (the response arriving means the backend is up)
- Database: `dbReachable` → healthy; else → critical
- Config file: `configPresent` → healthy; else → warning
- WebUI users: shown as a label (`"2 users"`) with `healthy` status; no severity

### Audit / Logs section

The existing "Recent Audit" section gains a tab bar with three tabs:
- **Audit Events** — existing paginated table, no changes
- **ocserv Logs** — fetches `GET /api/logs/tail?source=ocserv`
- **Panel Logs** — fetches `GET /api/logs/tail?source=panel`

Log tab states:
- `enabled=false`: shows chip _"Log file reading is disabled. Enable it in Settings → Monitoring."_
- `enabled=true, present=false`: shows chip _"Log file not found at `<path>`. You can update the path in Settings → Installation."_ with a link
- `enabled=true, present=true, error set`: shows inline error message
- Normal: monospace scrollable list of lines, file path as subtitle, "Refresh" button

### Data loading

- `GET /api/health/system`: fetched on mount, auto-refreshed every 30 seconds (same interval pattern as monitoring). Errors shown inline within the health cards — not a page-level error.
- `GET /api/logs/tail`: fetched on tab switch and on explicit "Refresh" click. Not auto-refreshed. Shows a loading spinner during fetch.

---

## Testing

- `HealthService` unit tests with a fake `ReadinessChecker` and fake store
- `LogTailService` unit tests using a `FileReader` fake — covers: disabled, file not found, permission error, normal tail, lines-cap enforcement
- Handler tests for `GET /api/health/system` and `GET /api/logs/tail` covering auth, permission denied, and happy path
- No live `ocserv`, real filesystem, or real log files required in normal tests

---

## Constraints and Limitations

- Log tail reads up to 50 lines only — not a streaming or live-follow view
- Log paths are configured in TOML; if the process has no read permission on the log file, the tab shows an error — no sudo or privilege escalation is attempted
- The `HealthService` does not attempt to repair or restart any component — it is read-only
- The Settings page cannot detect log file availability; that indication lives only in the dashboard
- `monitoring.log_file_read_enabled` disables disk reads for both log sources together; per-source toggles are out of scope
