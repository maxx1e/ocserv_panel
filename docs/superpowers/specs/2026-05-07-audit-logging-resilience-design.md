# Design: Audit Tab Logging Resilience & Zerolog Migration

**Date:** 2026-05-07  
**Status:** Draft  
**Scope:** Backend logging framework, audit tab UX resilience

---

## Problem Statement

The Audit tab fails when the ocserv service is not running because both the file reader and journal reader return "unavailable", resulting in a 503 error. Additionally, the "Web UI Backend Logs" (panel) tab is non-functional because:

1. The backend uses Go's `log/slog` which outputs unstructured text to stderr
2. No log file is ever written to the configured `PanelLogPath`
3. The log parser (`ParsePanelLine`) expects zerolog JSON format

The panel's own log file should **always** be available since the backend service itself generates the logs.

---

## Goals

1. Replace `log/slog` with `rs/zerolog` across the backend for structured JSON logging
2. Write panel logs to the configured file path (`PanelLogPath` from settings) with multi-writer output (file + stderr)
3. Ensure the panel log file is created and maintained by the backend service
4. Make the Audit tab gracefully handle ocserv log unavailability with an informational banner
5. Show a notification when a log file doesn't exist (distinct from "service not running")
6. Ensure log file paths are controllable via the Settings tab

---

## Architecture

### Zerolog Integration

Replace all `log/slog` usage with `github.com/rs/zerolog`. The logging setup happens at process start in `main.go`:

```
┌─────────────────────────────────────────────────────┐
│  main.go                                            │
│                                                     │
│  1. Read config (PanelLogPath, log level)           │
│  2. Open/create log file at PanelLogPath            │
│  3. Create zerolog.MultiLevelWriter(file, stderr)   │
│  4. Set global zerolog.Logger                       │
│  5. Set log level from config                       │
│                                                     │
│  Output: zerolog JSON to file + console output      │
└─────────────────────────────────────────────────────┘
```

**File management:**
- Create the log directory if it doesn't exist (with appropriate permissions 0750)
- Open log file with `os.O_CREATE|os.O_APPEND|os.O_WRONLY` and permissions 0640
- The file is never rotated by the backend itself — external log rotation (logrotate) is expected
- On SIGHUP, reopen the log file (to support logrotate)

**Console output:**
- Stderr gets human-friendly `zerolog.ConsoleWriter` for development
- In production (when `PanelLogPath` is set and writable), file gets JSON, stderr gets console format
- If file cannot be opened, log a warning to stderr and continue with stderr-only output (no crash)

### Log Level Control

The log level is read from the `logs.level` setting (already in settings registry). On startup:
- Read level from config
- Map to zerolog level: `debug` → `zerolog.DebugLevel`, `info` → `zerolog.InfoLevel`, `warn` → `zerolog.WarnLevel`, `error` → `zerolog.ErrorLevel`
- Default: `info`

The level setting is stored in SQLite (storage type: "database") and does **not** require service restart — zerolog supports runtime level changes via `zerolog.SetGlobalLevel()`.

### Component Logging Pattern

Each subsystem uses a sub-logger with a `component` field:

```go
logger := zerolog.Ctx(ctx).With().Str("component", "auth").Logger()
logger.Info().Str("user", username).Msg("login attempt")
```

This produces JSON that matches what `ParsePanelLine` already expects:
```json
{"level":"info","time":"2026-05-07T10:23:45Z","component":"auth","user":"admin","message":"login attempt"}
```

### New Package: `backend/internal/logging`

A thin logging setup package:

```go
package logging

// Config holds logging configuration.
type Config struct {
    Level    string // debug, info, warn, error
    FilePath string // path to log file (empty = stderr only)
}

// Setup initializes the global zerolog logger.
// Returns a closer function for the log file (nil if no file).
func Setup(cfg Config) (closer func() error, err error)

// ReopenFile reopens the log file after rotation (SIGHUP handler).
func ReopenFile() error

// SetLevel changes the global log level at runtime.
func SetLevel(level string)
```

---

## Audit Tab Frontend Changes

### Source Availability States

The audit tab needs to distinguish three states per source:

| State | Panel Tab | ocserv Tab |
|-------|-----------|------------|
| **Available** | Logs load normally | Logs load normally |
| **File missing** | Should NOT happen (backend creates it) — show error | Informational banner: log file not found at path |
| **Service not running** | N/A (panel service IS running) | Informational banner: service unavailable |

### Informational Banner Component

When the ocserv source is unavailable, show an amber/yellow banner instead of a blocking error:

```
┌─────────────────────────────────────────────────────────────────┐
│ ⚠️  ocserv VPN Logs Unavailable                                 │
│                                                                 │
│  The ocserv service is not running or the log file is not       │
│  accessible.                                                    │
│                                                                 │
│  Expected log path: /var/log/ocserv/ocserv.log                  │
│  Configured via: Settings → Installation → ocserv Log Path      │
│                                                                 │
│  [Retry]                                                        │
└─────────────────────────────────────────────────────────────────┘
```

For the panel tab, if somehow unavailable (extremely unlikely since the service is generating it):
```
┌─────────────────────────────────────────────────────────────────┐
│ ⚠️  Web UI Backend Logs Unavailable                              │
│                                                                 │
│  Unable to read the panel log file. Check file permissions.     │
│                                                                 │
│  Expected log path: /var/log/ocserv-panel/panel.log             │
│  Configured via: Settings → Installation → Panel Log Path       │
│                                                                 │
│  [Retry]                                                        │
└─────────────────────────────────────────────────────────────────┘
```

### Frontend Behavior Changes

1. **On 503 response**: Instead of showing a red error banner, show the amber informational banner
2. **The tab remains selectable**: User can still click the unavailable tab and see the status
3. **Retry button**: Triggers a fresh `loadLogs()` call
4. **Source metadata**: Use the `getAuditLogSources()` response to determine availability BEFORE attempting to load logs

### New Component: `LogSourceUnavailable.svelte`

Props:
- `source: AuditLogSource` — the source metadata (name, label, path, available)
- `onRetry: () => void` — retry callback

---

## Backend API Changes

### Enhanced Source Info

Add a `reason` field to `LogSourceInfo` to communicate WHY a source is unavailable:

```go
type LogSourceInfo struct {
    Name          string  `json:"name"`
    Label         string  `json:"label"`
    Available     bool    `json:"available"`
    Path          string  `json:"path"`
    Reader        string  `json:"reader"`      // "file", "journal", or ""
    Reason        string  `json:"reason"`      // "", "file_not_found", "permission_denied", "service_not_running"
    LogLevel      *int    `json:"logLevel"`
    LogLevelLabel string  `json:"logLevelLabel,omitempty"`
}
```

**Reason values:**
- `""` — source is available (no reason needed)
- `"file_not_found"` — the configured log file does not exist
- `"permission_denied"` — file exists but not readable
- `"service_not_running"` — neither file nor journal available (ocserv not running)

### Frontend API Type Update

Add `reason` to `AuditLogSource`:

```typescript
interface AuditLogSource {
  name: string;
  label: string;
  available: boolean;
  path: string;
  reader: string;
  reason: string;  // new field
  logLevel: number | null;
  logLevelLabel: string | null;
}
```

---

## slog → zerolog Migration

### Files to Change

All files using `log/slog`:

| File | Change |
|------|--------|
| `cmd/ocserv-panel/main.go` | Replace `slog.*` with `log.*` (zerolog global), add logging setup |
| `internal/app/housekeeping.go` | Replace `*slog.Logger` field with `zerolog.Logger` |
| `internal/app/settings_service.go` | Replace `slog.Warn` calls |
| `internal/metrics/tsdb_store.go` | Replace `slog.New(...)` discard handler |
| `internal/monitoring/collector_service.go` | If uses slog, replace |

### Migration Pattern

Before:
```go
slog.Info("message", "key", value)
slog.Error("message", "error", err)
slog.Warn("message", "key", value)
```

After:
```go
log.Info().Str("key", value).Msg("message")
log.Error().Err(err).Msg("message")
log.Warn().Str("key", value).Msg("message")
```

### Dependency Addition

Add to `go.mod`:
```
github.com/rs/zerolog v1.34.0
```

---

## Log File Creation & Lifecycle

### Startup Sequence

1. Load config (get `PanelLogPath` and `logs.level`)
2. Call `logging.Setup(Config{Level: level, FilePath: panelLogPath})`
3. Inside Setup:
   - Create parent directory with `os.MkdirAll(dir, 0750)` 
   - Open file with `os.OpenFile(path, O_CREATE|O_APPEND|O_WRONLY, 0640)`
   - Create `zerolog.MultiLevelWriter(file, consoleWriter)`
   - Set as global logger
4. Return file closer for deferred cleanup

### Error Cases

- **Directory creation fails** (permission denied): Log warning to stderr, continue stderr-only
- **File open fails** (permission denied): Log warning to stderr, continue stderr-only
- **File disappears at runtime** (deleted by user): Writes will fail silently until SIGHUP reopens

### SIGHUP Handling

Register SIGHUP handler in main.go to support logrotate:
```go
sighup := make(chan os.Signal, 1)
signal.Notify(sighup, syscall.SIGHUP)
go func() {
    for range sighup {
        logging.ReopenFile()
    }
}()
```

---

## Settings Integration

The following settings already exist and control log behavior:

| Setting | Storage | Effect |
|---------|---------|--------|
| `installation.panel_log_path` | config (TOML) | Where panel logs are written |
| `installation.ocserv_log_path` | config (TOML) | Where ocserv logs are read from |
| `logs.level` | database | Runtime log level (no restart needed) |
| `logs.retention_days` | database | How long to keep logs (future: rotation) |

**Runtime level change**: When `logs.level` is updated via Settings API, the settings service calls `logging.SetLevel(newLevel)` to apply immediately without restart.

---

## Testing Strategy

### Unit Tests

1. **`logging` package tests:**
   - Setup with valid file path creates file and writes JSON
   - Setup with invalid path falls back to stderr-only (no panic)
   - SetLevel changes log output filtering
   - ReopenFile closes old file and opens new one

2. **Audit log service tests (existing, update):**
   - Source with `reason` field populated correctly
   - File not found → reason = "file_not_found"
   - Permission denied → reason = "permission_denied"
   - Neither available → reason = "service_not_running"

3. **Frontend component tests (if applicable):**
   - `LogSourceUnavailable` renders correct message per reason
   - Tab remains interactive when source is unavailable

### Integration Verification

- Backend builds cleanly with zerolog dependency
- Panel log file is created at configured path on startup
- Audit tab "panel" source shows as available after startup
- Audit tab "ocserv" source shows informational banner when ocserv is off
- Log level change via settings takes effect without restart

---

## File Layout

```
backend/
├── internal/
│   ├── logging/           # NEW: logging setup package
│   │   ├── logging.go    # Setup, ReopenFile, SetLevel
│   │   └── logging_test.go
│   ├── app/
│   │   ├── audit_log_service.go  # MODIFY: add reason to LogSourceInfo
│   │   └── settings_service.go   # MODIFY: call SetLevel on change
│   ├── logs/              # EXISTING: no changes needed (parser already expects zerolog JSON)
│   └── ...
├── cmd/ocserv-panel/
│   └── main.go           # MODIFY: replace slog with zerolog setup
└── go.mod                # MODIFY: add rs/zerolog dependency

frontend/
└── src/
    ├── routes/audit/
    │   ├── +page.svelte               # MODIFY: use source reason for UX
    │   └── LogSourceUnavailable.svelte # NEW: informational banner component
    └── lib/
        └── api.ts                      # MODIFY: add reason to AuditLogSource type
```

---

## Risks & Mitigations

| Risk | Mitigation |
|------|------------|
| Log file fills disk | Not our concern — logrotate handles it. Document in deploy/ |
| Permission issues creating log dir | Graceful fallback to stderr-only, surface in audit sources |
| Breaking change to existing log consumers | Only syslog/stderr consumers affected — adding file doesn't remove stderr |
| zerolog API unfamiliarity | Zerolog API is simpler than slog — method chaining |
| Settings level change race | zerolog's SetGlobalLevel is goroutine-safe |

---

## Out of Scope

- Log rotation (handled by external logrotate)
- Log shipping to remote systems
- Audit event logs in SQLite (separate system, already working)
- ocserv log format changes (we only read, not write)
