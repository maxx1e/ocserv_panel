# Audit Logging Resilience & Zerolog Migration — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace `log/slog` with `rs/zerolog` so the backend writes structured JSON logs to a file, and make the Audit tab gracefully handle unavailable log sources with informational banners instead of errors.

**Architecture:** New `backend/internal/logging` package handles zerolog setup with multi-writer (file + stderr). The `AuditLogService.GetSources` adds a `reason` field explaining WHY a source is unavailable. The frontend replaces error states with an amber informational banner component.

**Tech Stack:** Go + rs/zerolog, SvelteKit + Tailwind CSS, existing `logs` package readers

---

## File Layout

| Action | Path | Purpose |
|--------|------|---------|
| Create | `backend/internal/logging/logging.go` | Zerolog setup, file writer, level control, SIGHUP reopen |
| Create | `backend/internal/logging/logging_test.go` | Unit tests for setup, level changes, reopen |
| Modify | `backend/go.mod` | Add `github.com/rs/zerolog` dependency |
| Modify | `backend/cmd/ocserv-panel/main.go` | Replace all slog with zerolog, call logging.Setup |
| Modify | `backend/internal/app/housekeeping.go` | Replace `*slog.Logger` with `zerolog.Logger` |
| Modify | `backend/internal/app/settings_service.go` | Replace slog.Warn with zerolog |
| Modify | `backend/internal/metrics/tsdb_store.go` | Keep slog for prometheus (external lib needs it), remove import where possible |
| Modify | `backend/internal/app/audit_log_service.go` | Add `Reason` field to `LogSourceInfo`, populate in GetSources |
| Modify | `backend/internal/app/audit_log_service_test.go` | Add tests for reason field |
| Modify | `frontend/src/lib/api.ts` | Add `reason` to `AuditLogSource` type |
| Create | `frontend/src/routes/audit/LogSourceUnavailable.svelte` | Amber banner component for unavailable sources |
| Modify | `frontend/src/routes/audit/+page.svelte` | Use new banner instead of error state |

---

### Task 1: Add zerolog Dependency

**Files:**
- Modify: `backend/go.mod`

- [ ] **Step 1: Add zerolog to go.mod**

```bash
cd /opt/ocserv_panel/backend && go get github.com/rs/zerolog@latest
```

- [ ] **Step 2: Verify dependency resolves**

Run: `cd /opt/ocserv_panel/backend && go mod tidy`
Expected: Clean exit, no errors

- [ ] **Step 3: Commit**

```bash
git add backend/go.mod backend/go.sum
git commit -m "chore: add rs/zerolog dependency"
```

---

### Task 2: Create `logging` Package

**Files:**
- Create: `backend/internal/logging/logging.go`
- Create: `backend/internal/logging/logging_test.go`

- [ ] **Step 1: Write failing tests for Setup**

Create `backend/internal/logging/logging_test.go`:

```go
package logging

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func TestSetup_FileOutput(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "test.log")

	closer, err := Setup(Config{Level: "info", FilePath: logPath})
	if err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	defer closer()

	log.Info().Str("component", "test").Msg("hello from test")

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read log file: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, `"message":"hello from test"`) {
		t.Errorf("log file does not contain expected message, got: %s", content)
	}
	if !strings.Contains(content, `"component":"test"`) {
		t.Errorf("log file does not contain component field, got: %s", content)
	}
}

func TestSetup_CreatesDirectory(t *testing.T) {
	dir := t.TempDir()
	nested := filepath.Join(dir, "sub", "logs")
	logPath := filepath.Join(nested, "panel.log")

	closer, err := Setup(Config{Level: "debug", FilePath: logPath})
	if err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	defer closer()

	if _, err := os.Stat(nested); os.IsNotExist(err) {
		t.Error("expected nested directory to be created")
	}
}

func TestSetup_InvalidPath_FallsBackToStderr(t *testing.T) {
	// /dev/null/impossible is not a valid directory
	closer, err := Setup(Config{Level: "info", FilePath: "/dev/null/impossible/panel.log"})
	if err != nil {
		t.Fatalf("Setup should not fail even with bad path: %v", err)
	}
	defer closer()

	// Should still be able to log (to stderr)
	log.Info().Msg("still works")
}

func TestSetup_EmptyPath_StderrOnly(t *testing.T) {
	closer, err := Setup(Config{Level: "warn", FilePath: ""})
	if err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	defer closer()

	// Global level should be warn
	if zerolog.GlobalLevel() != zerolog.WarnLevel {
		t.Errorf("level = %v, want WarnLevel", zerolog.GlobalLevel())
	}
}

func TestSetLevel(t *testing.T) {
	closer, err := Setup(Config{Level: "info", FilePath: ""})
	if err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	defer closer()

	SetLevel("debug")
	if zerolog.GlobalLevel() != zerolog.DebugLevel {
		t.Errorf("level = %v, want DebugLevel", zerolog.GlobalLevel())
	}

	SetLevel("error")
	if zerolog.GlobalLevel() != zerolog.ErrorLevel {
		t.Errorf("level = %v, want ErrorLevel", zerolog.GlobalLevel())
	}

	// Invalid level defaults to info
	SetLevel("invalid")
	if zerolog.GlobalLevel() != zerolog.InfoLevel {
		t.Errorf("level = %v, want InfoLevel for invalid input", zerolog.GlobalLevel())
	}
}

func TestReopenFile(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "reopen.log")

	closer, err := Setup(Config{Level: "info", FilePath: logPath})
	if err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	defer closer()

	log.Info().Msg("before reopen")

	// Simulate logrotate: rename old file
	rotated := logPath + ".1"
	if err := os.Rename(logPath, rotated); err != nil {
		t.Fatalf("rename: %v", err)
	}

	// Reopen
	if err := ReopenFile(); err != nil {
		t.Fatalf("ReopenFile failed: %v", err)
	}

	log.Info().Msg("after reopen")

	// New file should exist with new content
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read new log file: %v", err)
	}
	if !strings.Contains(string(data), "after reopen") {
		t.Errorf("new log file should contain 'after reopen', got: %s", string(data))
	}

	// Old file should have old content
	oldData, _ := os.ReadFile(rotated)
	if !strings.Contains(string(oldData), "before reopen") {
		t.Errorf("rotated file should contain 'before reopen', got: %s", string(oldData))
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /opt/ocserv_panel/backend && go test ./internal/logging/...`
Expected: Compilation error (package doesn't exist yet)

- [ ] **Step 3: Implement the logging package**

Create `backend/internal/logging/logging.go`:

```go
package logging

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// Config holds logging configuration.
type Config struct {
	Level    string // debug, info, warn, error
	FilePath string // path to log file (empty = stderr only)
}

var (
	mu       sync.Mutex
	filePath string
	logFile  *os.File
)

// Setup initializes the global zerolog logger.
// Returns a closer function for the log file.
func Setup(cfg Config) (closer func(), err error) {
	mu.Lock()
	defer mu.Unlock()

	level := parseLevel(cfg.Level)
	zerolog.SetGlobalLevel(level)
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnixMs

	// Console writer for stderr (human-friendly in dev, structured in prod)
	consoleWriter := zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: "15:04:05"}

	if cfg.FilePath == "" {
		// Stderr-only mode
		log.Logger = zerolog.New(consoleWriter).With().Timestamp().Logger()
		return func() {}, nil
	}

	// Create directory if needed
	dir := filepath.Dir(cfg.FilePath)
	if mkErr := os.MkdirAll(dir, 0750); mkErr != nil {
		// Fallback to stderr only
		fmt.Fprintf(os.Stderr, "logging: failed to create log directory %s: %v (using stderr only)\n", dir, mkErr)
		log.Logger = zerolog.New(consoleWriter).With().Timestamp().Logger()
		return func() {}, nil
	}

	// Open log file
	f, openErr := os.OpenFile(cfg.FilePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0640)
	if openErr != nil {
		// Fallback to stderr only
		fmt.Fprintf(os.Stderr, "logging: failed to open log file %s: %v (using stderr only)\n", cfg.FilePath, openErr)
		log.Logger = zerolog.New(consoleWriter).With().Timestamp().Logger()
		return func() {}, nil
	}

	filePath = cfg.FilePath
	logFile = f

	// Multi-writer: JSON to file, console to stderr
	multi := io.MultiWriter(f, consoleWriter)
	log.Logger = zerolog.New(multi).With().Timestamp().Logger()

	return func() {
		mu.Lock()
		defer mu.Unlock()
		if logFile != nil {
			logFile.Close()
			logFile = nil
		}
	}, nil
}

// ReopenFile closes and reopens the log file (for logrotate support).
func ReopenFile() error {
	mu.Lock()
	defer mu.Unlock()

	if filePath == "" || logFile == nil {
		return nil
	}

	// Close old file
	if err := logFile.Close(); err != nil {
		return fmt.Errorf("close old log file: %w", err)
	}

	// Open new file
	f, err := os.OpenFile(filePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0640)
	if err != nil {
		return fmt.Errorf("reopen log file %s: %w", filePath, err)
	}
	logFile = f

	// Rebuild logger with new file
	consoleWriter := zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: "15:04:05"}
	multi := io.MultiWriter(f, consoleWriter)
	log.Logger = zerolog.New(multi).With().Timestamp().Logger()

	return nil
}

// SetLevel changes the global log level at runtime.
func SetLevel(level string) {
	zerolog.SetGlobalLevel(parseLevel(level))
}

func parseLevel(s string) zerolog.Level {
	switch s {
	case "debug":
		return zerolog.DebugLevel
	case "info":
		return zerolog.InfoLevel
	case "warn":
		return zerolog.WarnLevel
	case "error":
		return zerolog.ErrorLevel
	default:
		return zerolog.InfoLevel
	}
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /opt/ocserv_panel/backend && go test ./internal/logging/... -v`
Expected: All tests pass

- [ ] **Step 5: Commit**

```bash
git add backend/internal/logging/
git commit -m "feat: add logging package with zerolog setup, file writer, and SIGHUP reopen"
```

---

### Task 3: Migrate `main.go` from slog to zerolog

**Files:**
- Modify: `backend/cmd/ocserv-panel/main.go`

- [ ] **Step 1: Replace all slog usage in main.go with zerolog**

Replace `import "log/slog"` with:
```go
import (
	"github.com/rs/zerolog/log"
	"ocserv_panel/backend/internal/logging"
)
```

Add logging setup after config load (after line 64 `config.MergeEnv(&cfg, os.Getenv)`):
```go
	// Initialize structured logging.
	logLevel := "info" // TODO: could read from DB setting in future
	logCloser, _ := logging.Setup(logging.Config{
		Level:    logLevel,
		FilePath: cfg.Installation.PanelLogPath,
	})
	defer logCloser()

	// Handle SIGHUP for log rotation.
	sighup := make(chan os.Signal, 1)
	signal.Notify(sighup, syscall.SIGHUP)
	go func() {
		for range sighup {
			if err := logging.ReopenFile(); err != nil {
				log.Error().Err(err).Msg("failed to reopen log file on SIGHUP")
			} else {
				log.Info().Msg("log file reopened after SIGHUP")
			}
		}
	}()
```

Replace all `slog.*` calls with zerolog equivalents:

| Before | After |
|--------|-------|
| `slog.Error("open sqlite database failed", "error", err)` | `log.Error().Err(err).Msg("open sqlite database failed")` |
| `slog.Info("sqlite database ready", "dsn_set", dbDSN != "")` | `log.Info().Bool("dsn_set", dbDSN != "").Msg("sqlite database ready")` |
| `slog.Error("close sqlite database failed", "error", err)` | `log.Error().Err(err).Msg("close sqlite database failed")` |
| `slog.Error("apply sqlite migrations failed", "error", err)` | `log.Error().Err(err).Msg("apply sqlite migrations failed")` |
| `slog.Info("migrations applied successfully")` | `log.Info().Msg("migrations applied successfully")` |
| `slog.Warn("loading config file failed...", "path", configPath, "error", cfgErr)` | `log.Warn().Str("path", configPath).Err(cfgErr).Msg("loading config file failed, continuing with defaults")` |
| `slog.Info("config loaded", "path", configPath)` | `log.Info().Str("path", configPath).Msg("config loaded")` |
| `slog.Info("ocserv integration configured", ...)` | `log.Info().Str("config_path", cfg.Installation.OcservConfigPath).Str("socket_path", cfg.Installation.OcservSocketPath).Str("unit", cfg.Service.SystemdUnitName).Msg("ocserv integration configured")` |
| `slog.Error("close metric store failed", "error", err)` | `log.Error().Err(err).Msg("close metric store failed")` |
| `slog.Warn("vpn interface discovery failed", "error", err)` | `log.Warn().Err(err).Msg("vpn interface discovery failed")` |
| `slog.Info("vpn interface discovered", "interface", vpnIface, "source", vpnSource)` | `log.Info().Str("interface", vpnIface).Str("source", vpnSource).Msg("vpn interface discovered")` |
| `slog.Info("no vpn interface discovered...")` | `log.Info().Msg("no vpn interface discovered; vpn metrics will remain unavailable")` |
| `slog.Info("vpn interface set via environment variable...")` | `log.Info().Msg("vpn interface set via environment variable; skipping auto-discovery")` |
| `slog.Error("backend shutdown failed", "error", err)` | `log.Error().Err(err).Msg("backend shutdown failed")` |
| `slog.Info("starting ocserv panel backend", ...)` | `log.Info().Str("addr", addr).Str("config", configPath).Bool("dsn_set", dbDSN != "").Str("metrics_path", tsdbPathFromEnv()).Str("ocserv_config", cfg.Installation.OcservConfigPath).Msg("starting ocserv panel backend")` |
| `slog.Error("backend server failed", "error", err)` | `log.Error().Err(err).Msg("backend server failed")` |
| `slog.Info("metrics store ready", ...)` | `log.Info().Str("type", "tsdb").Str("path", tsdbPath).Str("retention", "30d").Msg("metrics store ready")` |
| `slog.Warn("open metrics tsdb failed...", "error", err)` | `log.Warn().Err(err).Msg("open metrics tsdb failed; using in-memory metrics store")` |
| `slog.Warn("default interface discovery failed", "error", err)` | `log.Warn().Err(err).Msg("default interface discovery failed")` |
| `slog.Info("default interface discovered", "interface", defaultIface)` | `log.Info().Str("interface", defaultIface).Msg("default interface discovered")` |
| `slog.Info("no default interface discovered...")` | `log.Info().Msg("no default interface discovered; network interface metrics will remain unavailable")` |

Replace the `HousekeepingWithLogger(slog.Default())` call with `app.HousekeepingWithLogger(log.Logger)`.

Replace `slog.Default()` passed to `NewCollectorService` with a zerolog adapter. Create a thin adapter in main:
```go
type zerologAdapter struct {
	logger zerolog.Logger
}

func (z zerologAdapter) Debug(msg string, args ...any) {
	z.logger.Debug().Fields(pairsToMap(args)).Msg(msg)
}

func (z zerologAdapter) Warn(msg string, args ...any) {
	z.logger.Warn().Fields(pairsToMap(args)).Msg(msg)
}

func pairsToMap(args []any) map[string]interface{} {
	m := make(map[string]interface{}, len(args)/2)
	for i := 0; i+1 < len(args); i += 2 {
		if k, ok := args[i].(string); ok {
			m[k] = args[i+1]
		}
	}
	return m
}
```

Then pass `zerologAdapter{logger: log.Logger}` to `NewCollectorService`.

- [ ] **Step 2: Verify compilation**

Run: `cd /opt/ocserv_panel/backend && go build ./cmd/ocserv-panel/`
Expected: Compiles cleanly

- [ ] **Step 3: Run all tests**

Run: `cd /opt/ocserv_panel/backend && go test ./...`
Expected: All tests pass

- [ ] **Step 4: Commit**

```bash
git add backend/cmd/ocserv-panel/main.go
git commit -m "feat: migrate main.go from slog to zerolog with file output and SIGHUP support"
```

---

### Task 4: Migrate `housekeeping.go` from slog to zerolog

**Files:**
- Modify: `backend/internal/app/housekeeping.go`

- [ ] **Step 1: Replace slog with zerolog in housekeeping.go**

Change the import from `"log/slog"` to `"github.com/rs/zerolog"` and `"github.com/rs/zerolog/log"`.

Replace the struct field:
```go
// Before:
logger   *slog.Logger

// After:
logger   zerolog.Logger
```

Replace the default in the constructor:
```go
// Before:
logger:   slog.Default(),

// After:
logger:   log.Logger,
```

Replace the option function:
```go
// Before:
func HousekeepingWithLogger(logger *slog.Logger) HousekeepingOption {
	return func(h *HousekeepingScheduler) { h.logger = logger }
}

// After:
func HousekeepingWithLogger(logger zerolog.Logger) HousekeepingOption {
	return func(h *HousekeepingScheduler) { h.logger = logger }
}
```

Replace all logger calls in Run and runOnce:
```go
// Before:
h.logger.Info("housekeeping scheduler started", "interval", h.interval)
// After:
h.logger.Info().Dur("interval", h.interval).Msg("housekeeping scheduler started")

// Before:
h.logger.Info("housekeeping scheduler stopped")
// After:
h.logger.Info().Msg("housekeeping scheduler stopped")

// Before:
h.logger.Info("housekeeping purge starting", "retention_days", retentionDays)
// After:
h.logger.Info().Int("retention_days", retentionDays).Msg("housekeeping purge starting")

// Before:
h.logger.Error("housekeeping purge failed", "error", err, "retention_days", retentionDays)
// After:
h.logger.Error().Err(err).Int("retention_days", retentionDays).Msg("housekeeping purge failed")

// Before:
h.logger.Info("housekeeping purge completed", "deleted_rows", deleted, "retention_days", retentionDays)
// After:
h.logger.Info().Int64("deleted_rows", deleted).Int("retention_days", retentionDays).Msg("housekeeping purge completed")
```

- [ ] **Step 2: Verify compilation**

Run: `cd /opt/ocserv_panel/backend && go build ./...`
Expected: Compiles cleanly

- [ ] **Step 3: Run tests**

Run: `cd /opt/ocserv_panel/backend && go test ./internal/app/...`
Expected: All tests pass

- [ ] **Step 4: Commit**

```bash
git add backend/internal/app/housekeeping.go
git commit -m "refactor: migrate housekeeping from slog to zerolog"
```

---

### Task 5: Migrate `settings_service.go` from slog to zerolog

**Files:**
- Modify: `backend/internal/app/settings_service.go`

- [ ] **Step 1: Replace slog with zerolog in settings_service.go**

Change the import from `"log/slog"` to `"github.com/rs/zerolog/log"`.

Replace the two slog.Warn calls:

```go
// Line 134 — Before:
slog.Warn("settings: failed to load TOML config for GetAllSections, using startup snapshot", "path", s.configPath, "error", err)
// After:
log.Warn().Str("path", s.configPath).Err(err).Msg("settings: failed to load TOML config for GetAllSections, using startup snapshot")

// Line 392 — Before:
slog.Warn("settings: failed to load TOML config for RestartStatus, using startup snapshot", "path", s.configPath, "error", loadErr)
// After:
log.Warn().Str("path", s.configPath).Err(loadErr).Msg("settings: failed to load TOML config for RestartStatus, using startup snapshot")
```

- [ ] **Step 2: Verify compilation and tests**

Run: `cd /opt/ocserv_panel/backend && go build ./... && go test ./internal/app/...`
Expected: Compiles and all tests pass

- [ ] **Step 3: Commit**

```bash
git add backend/internal/app/settings_service.go
git commit -m "refactor: migrate settings_service from slog to zerolog"
```

---

### Task 6: Handle TSDB slog (keep for external library)

**Files:**
- Modify: `backend/internal/metrics/tsdb_store.go`

- [ ] **Step 1: Review and keep slog in tsdb_store.go**

The TSDB store passes a `*slog.Logger` with `io.Discard` handler to `tsdb.Open()`. This is required by the prometheus/tsdb library API which expects `*slog.Logger`. Keep `"log/slog"` import here since it's used for an external library interface. No changes needed — this file is an exception since the prometheus tsdb package requires `*slog.Logger`.

- [ ] **Step 2: Verify no compilation issues remain**

Run: `cd /opt/ocserv_panel/backend && go build ./...`
Expected: Compiles cleanly with no unused imports

- [ ] **Step 3: Commit (if any import cleanup was needed)**

```bash
git add backend/internal/metrics/tsdb_store.go
git commit -m "chore: document slog retention in tsdb_store for prometheus library requirement"
```

---

### Task 7: Add `Reason` Field to `LogSourceInfo`

**Files:**
- Modify: `backend/internal/app/audit_log_service.go`
- Modify: `backend/internal/app/audit_log_service_test.go`

- [ ] **Step 1: Write tests for the reason field**

Add to `backend/internal/app/audit_log_service_test.go`:

```go
func TestAuditLogService_GetSources_Reason_FileNotFound(t *testing.T) {
	// File reader unavailable, journal reader unavailable => "service_not_running"
	fakeFile := logs.NewFakeLogReader(false, nil, nil)
	fakeJournal := logs.NewFakeLogReader(false, nil, nil)
	svc := NewAuditLogService(
		func() config.Config { return config.Defaults() },
		map[string]logs.LogReader{"panel": fakeFile, "ocserv": fakeFile},
		map[string]logs.LogReader{"panel": fakeJournal, "ocserv": fakeJournal},
		nil,
		rbac.NewDefaultAuthorizer(),
	)
	principal := auth.Principal{ID: 1, Username: "admin", Roles: []string{"owner"}}
	sources, err := svc.GetSources(context.Background(), principal)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, src := range sources {
		if src.Available {
			t.Errorf("source %s should not be available", src.Name)
		}
		if src.Reason != "service_not_running" {
			t.Errorf("source %s reason = %q, want service_not_running", src.Name, src.Reason)
		}
	}
}

func TestAuditLogService_GetSources_Reason_Available(t *testing.T) {
	fakeFile := logs.NewFakeLogReader(true, &logs.ReadResult{}, nil)
	svc := NewAuditLogService(
		func() config.Config { return config.Defaults() },
		map[string]logs.LogReader{"panel": fakeFile, "ocserv": fakeFile},
		map[string]logs.LogReader{},
		nil,
		rbac.NewDefaultAuthorizer(),
	)
	principal := auth.Principal{ID: 1, Username: "admin", Roles: []string{"owner"}}
	sources, err := svc.GetSources(context.Background(), principal)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, src := range sources {
		if !src.Available {
			t.Errorf("source %s should be available", src.Name)
		}
		if src.Reason != "" {
			t.Errorf("source %s reason = %q, want empty", src.Name, src.Reason)
		}
	}
}
```

- [ ] **Step 2: Run tests to verify new tests fail**

Run: `cd /opt/ocserv_panel/backend && go test ./internal/app/ -run TestAuditLogService_GetSources_Reason -v`
Expected: Compilation error (Reason field doesn't exist yet)

- [ ] **Step 3: Add Reason field to LogSourceInfo and populate in GetSources**

In `backend/internal/app/audit_log_service.go`, add `Reason` to the struct:

```go
type LogSourceInfo struct {
	Name          string `json:"name"`
	Label         string `json:"label"`
	Available     bool   `json:"available"`
	Path          string `json:"path"`
	Reader        string `json:"reader"`
	Reason        string `json:"reason"`
	LogLevel      *int   `json:"logLevel"`
	LogLevelLabel string `json:"logLevelLabel,omitempty"`
}
```

Update the `GetSources` method's loop to populate the reason:

```go
	for _, name := range []string{"panel", "ocserv"} {
		info := LogSourceInfo{
			Name:  name,
			Label: sourceLabels[name],
		}

		switch name {
		case "panel":
			info.Path = cfg.Installation.PanelLogPath
		case "ocserv":
			info.Path = cfg.Installation.OcservLogPath
		}

		reader := s.resolveReader(ctx, name)
		info.Available = reader != nil

		if fileReader, ok := s.fileReaders[name]; ok && fileReader.Available(ctx) {
			info.Reader = "file"
		} else if journalReader, ok := s.journalReaders[name]; ok && journalReader.Available(ctx) {
			info.Reader = "journal"
		}

		// Populate reason when unavailable
		if !info.Available {
			info.Reason = "service_not_running"
		}

		sources = append(sources, info)
	}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /opt/ocserv_panel/backend && go test ./internal/app/ -run TestAuditLogService -v`
Expected: All audit log service tests pass

- [ ] **Step 5: Commit**

```bash
git add backend/internal/app/audit_log_service.go backend/internal/app/audit_log_service_test.go
git commit -m "feat: add reason field to LogSourceInfo for unavailability explanation"
```

---

### Task 8: Update Frontend API Types

**Files:**
- Modify: `frontend/src/lib/api.ts`

- [ ] **Step 1: Add `reason` field to AuditLogSource type**

In `frontend/src/lib/api.ts`, find the `AuditLogSource` type (around line 732) and add the `reason` field:

```typescript
export type AuditLogSource = {
  name: string;
  label: string;
  available: boolean;
  path: string;
  reader: string;
  reason: string;
  logLevel: number | null;
  logLevelLabel: string | null;
};
```

- [ ] **Step 2: Verify TypeScript compilation**

Run: `cd /opt/ocserv_panel/frontend && npx svelte-check --tsconfig ./tsconfig.json 2>&1 | head -30`
Expected: No new errors related to `AuditLogSource`

- [ ] **Step 3: Commit**

```bash
git add frontend/src/lib/api.ts
git commit -m "feat: add reason field to AuditLogSource frontend type"
```

---

### Task 9: Create `LogSourceUnavailable.svelte` Component

**Files:**
- Create: `frontend/src/routes/audit/LogSourceUnavailable.svelte`

- [ ] **Step 1: Create the informational banner component**

Create `frontend/src/routes/audit/LogSourceUnavailable.svelte`:

```svelte
<script lang="ts">
  import { AlertTriangle, RefreshCw } from 'lucide-svelte';
  import type { AuditLogSource } from '$lib/api';

  type Props = {
    source: AuditLogSource;
    onRetry: () => void;
  };

  let { source, onRetry }: Props = $props();

  const reasonMessages: Record<string, string> = {
    service_not_running:
      'The service is not running or the log source is not accessible.',
    file_not_found: 'The log file does not exist at the configured path.',
    permission_denied:
      'The log file exists but cannot be read. Check file permissions.'
  };

  const message = $derived(
    reasonMessages[source.reason] ??
      'The log source is not available. Check service status and configuration.'
  );

  const settingsHint = $derived(
    source.name === 'panel'
      ? 'Settings → Installation → Panel Log Path'
      : 'Settings → Installation → ocserv Log Path'
  );
</script>

<div
  class="rounded-lg border border-amber-200 dark:border-amber-800 bg-amber-50 dark:bg-amber-950/30 p-5"
>
  <div class="flex items-start gap-3">
    <AlertTriangle class="w-5 h-5 text-amber-500 dark:text-amber-400 mt-0.5 shrink-0" />
    <div class="flex-1 min-w-0">
      <h3 class="text-sm font-semibold text-amber-800 dark:text-amber-200">
        {source.label} Unavailable
      </h3>
      <p class="text-sm text-amber-700 dark:text-amber-300 mt-1">
        {message}
      </p>
      <div class="mt-3 space-y-1">
        <p class="text-xs text-amber-600 dark:text-amber-400">
          <span class="font-medium">Expected path:</span>
          <code class="ml-1 px-1.5 py-0.5 bg-amber-100 dark:bg-amber-900/50 rounded text-amber-800 dark:text-amber-200">
            {source.path}
          </code>
        </p>
        <p class="text-xs text-amber-600 dark:text-amber-400">
          <span class="font-medium">Configured via:</span>
          <span class="ml-1">{settingsHint}</span>
        </p>
      </div>
      <button
        type="button"
        onclick={onRetry}
        class="mt-4 inline-flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium text-amber-700 dark:text-amber-200 bg-amber-100 dark:bg-amber-900/50 hover:bg-amber-200 dark:hover:bg-amber-900 rounded-md transition-colors"
      >
        <RefreshCw class="w-3.5 h-3.5" />
        Retry
      </button>
    </div>
  </div>
</div>
```

- [ ] **Step 2: Verify component compiles**

Run: `cd /opt/ocserv_panel/frontend && npx svelte-check --tsconfig ./tsconfig.json 2>&1 | grep -i "error" | head -10`
Expected: No errors related to LogSourceUnavailable

- [ ] **Step 3: Commit**

```bash
git add frontend/src/routes/audit/LogSourceUnavailable.svelte
git commit -m "feat: add LogSourceUnavailable banner component for audit tab"
```

---

### Task 10: Update Audit Page to Use Informational Banner

**Files:**
- Modify: `frontend/src/routes/audit/+page.svelte`

- [ ] **Step 1: Import the new component and update the unavailable state**

Add the import at the top of the script section:
```typescript
import LogSourceUnavailable from './LogSourceUnavailable.svelte';
```

Replace the existing "Source unavailable" block (lines 316-324 approximately):

```svelte
    <!-- Source unavailable — informational banner instead of error -->
    {#if activeSource && !activeSource.available}
      <LogSourceUnavailable source={activeSource} onRetry={handleRefresh} />
    {:else}
```

This replaces the old block:
```svelte
    {#if activeSource && !activeSource.available}
      <div class="bg-slate-50 dark:bg-slate-800/50 border border-slate-200 dark:border-slate-700 rounded-lg p-4 text-center">
        <p class="text-sm text-slate-500 dark:text-slate-400">
          This log source is not configured or the file is not accessible.
        </p>
        <p class="text-xs text-slate-400 dark:text-slate-500 mt-1">
          Expected at: {activeSource.path}
        </p>
      </div>
    {:else}
```

- [ ] **Step 2: Update 503 error handling to be less alarming**

In the `loadLogs()` function's catch block, change the 503 handling to set a softer message (since the banner handles the primary UX now, but we still want to handle the case where sources haven't loaded yet):

```typescript
    } catch (caught) {
      if (caught instanceof ApiError) {
        if (caught.status === 503) {
          // Source unavailable — the banner handles this via source metadata.
          // Only show error text if sources haven't been loaded yet.
          if (!activeSource) {
            logError = `Log source "${activeTab}" is currently unavailable.`;
          }
          // Otherwise the LogSourceUnavailable banner is showing.
        } else {
          logError = caught.message;
        }
      } else {
        logError = caught instanceof Error ? caught.message : 'Failed to load logs.';
      }
      lines = [];
      total = 0;
      hasMore = false;
    } finally {
      logLoading = false;
    }
```

- [ ] **Step 3: Verify TypeScript compilation and build**

Run: `cd /opt/ocserv_panel/frontend && npx svelte-check --tsconfig ./tsconfig.json 2>&1 | tail -5`
Expected: No errors

Run: `cd /opt/ocserv_panel/frontend && npm run build 2>&1 | tail -5`
Expected: Build succeeds

- [ ] **Step 4: Commit**

```bash
git add frontend/src/routes/audit/+page.svelte
git commit -m "feat: replace audit tab error state with informational banner for unavailable sources"
```

---

### Task 11: Integration Verification

**Files:**
- All files from previous tasks

- [ ] **Step 1: Run full backend test suite**

Run: `cd /opt/ocserv_panel/backend && go test ./... -count=1`
Expected: All packages pass

- [ ] **Step 2: Run frontend build**

Run: `cd /opt/ocserv_panel/frontend && npm run build`
Expected: Clean build with no errors

- [ ] **Step 3: Verify panel log file would be created**

Run: `cd /opt/ocserv_panel/backend && go build -o /tmp/ocserv-panel-test ./cmd/ocserv-panel/ && echo "Build OK"`
Expected: Binary builds successfully

- [ ] **Step 4: Run logging package tests specifically**

Run: `cd /opt/ocserv_panel/backend && go test ./internal/logging/... -v -count=1`
Expected: All logging tests pass

- [ ] **Step 5: Verify no remaining slog usage (except tsdb_store.go)**

Run: `grep -rn "\"log/slog\"" /opt/ocserv_panel/backend/ --include="*.go" | grep -v _test.go | grep -v tsdb_store.go`
Expected: No output (all slog replaced except the prometheus library requirement)

- [ ] **Step 6: Final commit if any fixups needed**

```bash
# Only if there were fixups
git add -A
git commit -m "fix: integration fixups for zerolog migration"
```

---

## Summary of Changes

1. **Task 1**: Add `rs/zerolog` dependency
2. **Task 2**: Create `backend/internal/logging` package (setup, file writer, level control, SIGHUP)
3. **Task 3**: Migrate `main.go` from slog to zerolog
4. **Task 4**: Migrate `housekeeping.go` from slog to zerolog
5. **Task 5**: Migrate `settings_service.go` from slog to zerolog
6. **Task 6**: Keep slog in `tsdb_store.go` (prometheus library requirement)
7. **Task 7**: Add `Reason` field to `LogSourceInfo` + tests
8. **Task 8**: Update frontend `AuditLogSource` type with `reason`
9. **Task 9**: Create `LogSourceUnavailable.svelte` banner component
10. **Task 10**: Update audit page to use informational banner
11. **Task 11**: Full integration verification
