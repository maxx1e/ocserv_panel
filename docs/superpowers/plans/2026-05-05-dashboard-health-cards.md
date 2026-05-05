# Dashboard Health Cards & Log Tail Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the top dashboard stat cards with two structured `HealthCard` components (VPN Server and WebUI Panel), add a tabbed Audit/Logs section with ocserv and panel log tail, and expose log path and toggle settings in the Settings UI.

**Architecture:** New `HealthService` and `LogTailService` in `backend/internal/app/` back two new endpoints (`GET /api/health/system` and `GET /api/logs/tail`). Three new settings keys (`installation.ocserv_log_path`, `installation.panel_log_path`, `monitoring.log_file_read_enabled`) wire through the existing config/settings infrastructure. The SvelteKit dashboard page gains a new `HealthCard.svelte` component and a tabbed audit section.

**Tech Stack:** Go (net/http, os, io), SvelteKit 5 runes API, Tailwind CSS v4, existing `ReadinessChecker`, `FileReader` interface pattern from `backend/internal/monitoring/collectors.go`.

---

## File Map

### New files
- `backend/internal/app/health_service.go` — `HealthService` with `SystemHealth()` method
- `backend/internal/app/health_service_test.go` — unit tests for `HealthService`
- `backend/internal/app/log_tail_service.go` — `LogTailService` with `TailLog()` method + `LogFileReader` interface
- `backend/internal/app/log_tail_service_test.go` — unit tests for `LogTailService`
- `backend/internal/server/health_handlers.go` — HTTP handlers for both new endpoints
- `backend/internal/server/health_handlers_test.go` — handler tests
- `frontend/src/lib/components/ui/HealthCard.svelte` — new HealthCard component

### Modified files
- `backend/internal/config/config.go` — add `OcservLogPath`, `PanelLogPath` to `InstallationConfig`; add `LogFileReadEnabled` to `MonitoringConfig`; update `Defaults()`
- `backend/internal/app/settings_registry.go` — add 3 new settings keys
- `backend/internal/app/settings_config_apply.go` — add apply and read cases for the 3 new keys
- `backend/internal/server/server.go` — add `healthService` and `logTailService` to `App` struct; wire them in `NewAppRouter`; register 2 new routes; pass `cfgFn` from main
- `backend/cmd/ocserv-panel/main.go` — construct `HealthService` and `LogTailService`, pass to router
- `frontend/src/lib/api.ts` — add `SystemHealth`, `LogTailResult` types and `getSystemHealth()`, `getLogTail()` functions
- `frontend/src/routes/+page.svelte` — replace top stat cards with two `HealthCard` instances; add tabbed audit/logs section

---

## Task 1: Extend config struct with log path and toggle fields

**Files:**
- Modify: `backend/internal/config/config.go`

- [ ] **Step 1: Add fields to `InstallationConfig` and `MonitoringConfig`**

In `backend/internal/config/config.go`, add to `InstallationConfig`:
```go
OcservLogPath string `toml:"ocserv_log_path"`
PanelLogPath  string `toml:"panel_log_path"`
```

Add to `MonitoringConfig`:
```go
LogFileReadEnabled bool `toml:"log_file_read_enabled"`
```

- [ ] **Step 2: Update `Defaults()` to set sensible defaults**

In the `Defaults()` function, update the `Installation` block:
```go
OcservLogPath: "/var/log/ocserv/ocserv.log",
PanelLogPath:  "/var/log/ocserv-panel/panel.log",
```

And the `Monitoring` block:
```go
LogFileReadEnabled: true,
```

- [ ] **Step 3: Build and verify no compile errors**

```bash
cd /opt/ocserv_panel/backend && go build ./...
```
Expected: no errors

- [ ] **Step 4: Commit**

```bash
git add backend/internal/config/config.go
git commit -m "feat(config): add ocserv/panel log paths and log_file_read_enabled to config"
```

---

## Task 2: Add settings registry entries and apply/read cases

**Files:**
- Modify: `backend/internal/app/settings_registry.go`
- Modify: `backend/internal/app/settings_config_apply.go`

- [ ] **Step 1: Add 3 new settings keys to `settings_registry.go`**

In `registeredSettings()`, inside the `installation` block (after `installation.cert_dir`, SortOrder 10), add:
```go
{Key: "installation.ocserv_log_path", Label: "ocserv Log Path", Section: "installation", Type: SettingTypeText, Source: SettingSourceConfig, RequiresRestart: false, Default: "/var/log/ocserv/ocserv.log", SortOrder: 11},
{Key: "installation.panel_log_path", Label: "Panel Log Path", Section: "installation", Type: SettingTypeText, Source: SettingSourceConfig, RequiresRestart: false, Default: "/var/log/ocserv-panel/panel.log", SortOrder: 12},
```

Inside the `monitoring` block (after `monitoring.dashboard_refresh_interval`, SortOrder 3), add:
```go
{Key: "monitoring.log_file_read_enabled", Label: "Enable Log File Reading", Section: "monitoring", Type: SettingTypeToggle, Source: SettingSourceConfig, RequiresRestart: false, Default: "true", SortOrder: 4},
```

- [ ] **Step 2: Add apply cases to `applySettingToConfig` in `settings_config_apply.go`**

In the `// Installation` block, add after `case "installation.cert_dir"`:
```go
case "installation.ocserv_log_path":
    cfg.Installation.OcservLogPath = value
case "installation.panel_log_path":
    cfg.Installation.PanelLogPath = value
```

In the `// Monitoring` block, add after `case "monitoring.dashboard_refresh_interval"`:
```go
case "monitoring.log_file_read_enabled":
    b, err := parseBool(value)
    if err != nil {
        return fmt.Errorf("%w: monitoring.log_file_read_enabled must be true or false", ErrInvalidInput)
    }
    cfg.Monitoring.LogFileReadEnabled = b
```

- [ ] **Step 3: Add read cases to `configValueForKey` in `settings_config_apply.go`**

After `case "installation.cert_dir"`:
```go
case "installation.ocserv_log_path":
    return cfg.Installation.OcservLogPath
case "installation.panel_log_path":
    return cfg.Installation.PanelLogPath
```

After `case "monitoring.dashboard_refresh_interval"`:
```go
case "monitoring.log_file_read_enabled":
    return strconv.FormatBool(cfg.Monitoring.LogFileReadEnabled)
```

- [ ] **Step 4: Build and run existing settings tests**

```bash
cd /opt/ocserv_panel/backend && go build ./... && go test ./internal/app/... -count=1 -timeout 60s
```
Expected: all pass

- [ ] **Step 5: Commit**

```bash
git add backend/internal/app/settings_registry.go backend/internal/app/settings_config_apply.go
git commit -m "feat(settings): add ocserv/panel log path and log_file_read_enabled settings"
```

---

## Task 3: Implement `HealthService`

**Files:**
- Create: `backend/internal/app/health_service.go`
- Create: `backend/internal/app/health_service_test.go`

- [ ] **Step 1: Write the failing test first**

Create `backend/internal/app/health_service_test.go`:
```go
package app_test

import (
	"context"
	"testing"

	"ocserv_panel/backend/internal/app"
	"ocserv_panel/backend/internal/auth"
	"ocserv_panel/backend/internal/config"
	"ocserv_panel/backend/internal/ocserv"
	"ocserv_panel/backend/internal/rbac"
)

func TestHealthServiceSystemHealth_HappyPath(t *testing.T) {
	t.Parallel()

	checker := ocserv.NewReadinessChecker(
		func() ocserv.ReadinessConfig {
			return ocserv.ReadinessConfig{
				OcservBinPath: "/nonexistent/ocserv",
				ConfigPath:    "/nonexistent/ocserv.conf",
				SocketPath:    "/nonexistent/ocserv.sock",
			}
		},
		ocserv.NewFakeCommandRunner(),
		ocserv.NewFakeServiceManager(ocserv.ServiceActive),
	)

	cfg := config.Defaults()
	cfgFn := func() config.Config { return cfg }

	svc := app.NewHealthService(checker, stubAdminCounter(2), cfgFn, rbac.NewDefaultAuthorizer())

	principal := auth.Principal{ID: 1, Username: "owner", Roles: []string{"owner"}}
	result, err := svc.SystemHealth(context.Background(), principal)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Panel.BackendStatus != "ok" {
		t.Errorf("expected panel.backendStatus=ok, got %q", result.Panel.BackendStatus)
	}
	if result.Panel.WebuiUserCount != 2 {
		t.Errorf("expected panel.webuiUserCount=2, got %d", result.Panel.WebuiUserCount)
	}
}

func TestHealthServiceSystemHealth_Unauthenticated(t *testing.T) {
	t.Parallel()

	cfg := config.Defaults()
	svc := app.NewHealthService(nil, stubAdminCounter(0), func() config.Config { return cfg }, rbac.NewDefaultAuthorizer())

	_, err := svc.SystemHealth(context.Background(), auth.Principal{})
	if err != app.ErrUnauthenticated {
		t.Fatalf("expected ErrUnauthenticated, got %v", err)
	}
}

func TestHealthServiceSystemHealth_PermissionDenied(t *testing.T) {
	t.Parallel()

	cfg := config.Defaults()
	svc := app.NewHealthService(nil, stubAdminCounter(0), func() config.Config { return cfg }, rbac.NewDefaultAuthorizer())

	principal := auth.Principal{ID: 2, Username: "viewer", Roles: []string{"viewer"}}
	// viewer has PermissionReadDashboard in the default authorizer, so use a role that doesn't
	principal2 := auth.Principal{ID: 3, Username: "noperm", Roles: []string{"unknown_role"}}
	_, err := svc.SystemHealth(context.Background(), principal2)
	if err != app.ErrPermissionDenied {
		t.Fatalf("expected ErrPermissionDenied for unknown role, got %v", err)
	}
	// viewer should succeed
	_, err2 := svc.SystemHealth(context.Background(), principal)
	if err2 != nil {
		t.Fatalf("viewer should have read dashboard perm: %v", err2)
	}
}

// stubAdminCounter is a minimal adminCounter for tests.
type stubAdminCounter int64

func (s stubAdminCounter) AdminCount(_ context.Context) (int64, error) {
	return int64(s), nil
}
```

- [ ] **Step 2: Run to confirm it fails (HealthService not yet defined)**

```bash
cd /opt/ocserv_panel/backend && go test ./internal/app/... -run TestHealthService -count=1 2>&1 | head -20
```
Expected: compile error "undefined: app.NewHealthService"

- [ ] **Step 3: Implement `health_service.go`**

Create `backend/internal/app/health_service.go`:
```go
package app

import (
	"context"
	"fmt"
	"os"

	"ocserv_panel/backend/internal/auth"
	"ocserv_panel/backend/internal/config"
	"ocserv_panel/backend/internal/ocserv"
	"ocserv_panel/backend/internal/rbac"
)

// adminCounter is a minimal store interface for HealthService.
type adminCounter interface {
	AdminCount(context.Context) (int64, error)
}

// SystemHealthResult is the payload returned by GET /api/health/system.
type SystemHealthResult struct {
	VPNServer VPNServerHealth `json:"vpnServer"`
	Panel     PanelHealth     `json:"panel"`
	Logs      LogsHealth      `json:"logs"`
}

// VPNServerHealth describes the ocserv process and config state.
type VPNServerHealth struct {
	SystemdStatus  string `json:"systemdStatus"`
	ProcessRunning bool   `json:"processRunning"`
	SocketPresent  bool   `json:"socketPresent"`
	ConfigPresent  bool   `json:"configPresent"`
	ConfigValid    bool   `json:"configValid"`
	ConfigErrors   string `json:"configErrors,omitempty"`
	BinaryPresent  bool   `json:"binaryPresent"`
	BinaryVersion  string `json:"binaryVersion,omitempty"`
}

// PanelHealth describes the WebUI panel backend state.
type PanelHealth struct {
	BackendStatus  string `json:"backendStatus"`
	DbReachable    bool   `json:"dbReachable"`
	DbPath         string `json:"dbPath"`
	ConfigPresent  bool   `json:"configPresent"`
	ConfigPath     string `json:"configPath"`
	WebuiUserCount int64  `json:"webuiUserCount"`
}

// LogsHealth describes log file availability.
type LogsHealth struct {
	OcservLogEnabled bool   `json:"ocservLogEnabled"`
	OcservLogPresent bool   `json:"ocservLogPresent"`
	OcservLogPath    string `json:"ocservLogPath"`
	PanelLogEnabled  bool   `json:"panelLogEnabled"`
	PanelLogPresent  bool   `json:"panelLogPresent"`
	PanelLogPath     string `json:"panelLogPath"`
}

// HealthService builds the system health snapshot.
type HealthService struct {
	readiness  *ocserv.ReadinessChecker
	store      adminCounter
	cfgFn      func() config.Config
	authorizer rbac.Authorizer
}

// NewHealthService returns a HealthService.
// readiness may be nil; VPN server fields will be reported as unknown when nil.
func NewHealthService(readiness *ocserv.ReadinessChecker, store adminCounter, cfgFn func() config.Config, authorizer rbac.Authorizer) *HealthService {
	return &HealthService{readiness: readiness, store: store, cfgFn: cfgFn, authorizer: authorizer}
}

// SystemHealth returns the current system health snapshot.
func (s *HealthService) SystemHealth(ctx context.Context, principal auth.Principal) (SystemHealthResult, error) {
	if !principal.Authenticated() {
		return SystemHealthResult{}, ErrUnauthenticated
	}
	if !s.authorizer.Allowed(principalRoles(principal), rbac.PermissionReadDashboard) {
		return SystemHealthResult{}, ErrPermissionDenied
	}

	cfg := s.cfgFn()

	vpn := s.buildVPNHealth(ctx)
	panel := s.buildPanelHealth(ctx, cfg)
	logs := s.buildLogsHealth(cfg)

	return SystemHealthResult{VPNServer: vpn, Panel: panel, Logs: logs}, nil
}

func (s *HealthService) buildVPNHealth(ctx context.Context) VPNServerHealth {
	if s.readiness == nil {
		return VPNServerHealth{SystemdStatus: "unknown"}
	}
	r, err := s.readiness.Check(ctx)
	if err != nil {
		return VPNServerHealth{SystemdStatus: "unknown"}
	}
	status := "unknown"
	if r.ServiceRunning {
		status = "active"
	} else if r.BinaryPresent {
		status = "inactive"
	}
	return VPNServerHealth{
		SystemdStatus:  status,
		ProcessRunning: r.ServiceRunning,
		SocketPresent:  r.SocketPresent,
		ConfigPresent:  r.ConfigPresent,
		ConfigValid:    r.ConfigValid,
		ConfigErrors:   r.ConfigErrors,
		BinaryPresent:  r.BinaryPresent,
		BinaryVersion:  r.BinaryVersion,
	}
}

func (s *HealthService) buildPanelHealth(ctx context.Context, cfg config.Config) PanelHealth {
	count, err := s.store.AdminCount(ctx)
	if err != nil {
		count = 0
	}
	configPath := cfg.Installation.ConfigPath
	dbPath := fmt.Sprintf("%s/panel.db", cfg.Installation.DataDir)
	configPresent := fileStatExists(configPath)
	dbReachable := fileStatExists(dbPath)
	return PanelHealth{
		BackendStatus:  "ok",
		DbReachable:    dbReachable,
		DbPath:         dbPath,
		ConfigPresent:  configPresent,
		ConfigPath:     configPath,
		WebuiUserCount: count,
	}
}

func (s *HealthService) buildLogsHealth(cfg config.Config) LogsHealth {
	enabled := cfg.Monitoring.LogFileReadEnabled
	ocservPath := cfg.Installation.OcservLogPath
	panelPath := cfg.Installation.PanelLogPath

	ocservPresent := false
	panelPresent := false
	if enabled {
		ocservPresent = fileStatExists(ocservPath)
		panelPresent = fileStatExists(panelPath)
	}
	return LogsHealth{
		OcservLogEnabled: enabled,
		OcservLogPresent: ocservPresent,
		OcservLogPath:    ocservPath,
		PanelLogEnabled:  enabled,
		PanelLogPresent:  panelPresent,
		PanelLogPath:     panelPath,
	}
}

// fileStatExists returns true if the path exists (file or directory).
func fileStatExists(path string) bool {
	if path == "" {
		return false
	}
	_, err := os.Stat(path)
	return err == nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd /opt/ocserv_panel/backend && go test ./internal/app/... -run TestHealthService -count=1 -v
```
Expected: all 3 tests PASS

- [ ] **Step 5: Run full app test suite to check no regressions**

```bash
cd /opt/ocserv_panel/backend && go test ./... -count=1 -timeout 120s
```
Expected: all pass

- [ ] **Step 6: Commit**

```bash
git add backend/internal/app/health_service.go backend/internal/app/health_service_test.go
git commit -m "feat(app): add HealthService for system health snapshot"
```

---

## Task 4: Implement `LogTailService`

**Files:**
- Create: `backend/internal/app/log_tail_service.go`
- Create: `backend/internal/app/log_tail_service_test.go`

- [ ] **Step 1: Write failing tests first**

Create `backend/internal/app/log_tail_service_test.go`:
```go
package app_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"ocserv_panel/backend/internal/app"
	"ocserv_panel/backend/internal/auth"
	"ocserv_panel/backend/internal/config"
	"ocserv_panel/backend/internal/rbac"
)

func testLogTailCfg(ocservLog, panelLog string, enabled bool) func() config.Config {
	cfg := config.Defaults()
	cfg.Installation.OcservLogPath = ocservLog
	cfg.Installation.PanelLogPath = panelLog
	cfg.Monitoring.LogFileReadEnabled = enabled
	return func() config.Config { return cfg }
}

func ownerPrincipal() auth.Principal {
	return auth.Principal{ID: 1, Username: "owner", Roles: []string{"owner"}}
}

func TestLogTailService_Disabled(t *testing.T) {
	t.Parallel()

	svc := app.NewLogTailService(
		testLogTailCfg("/var/log/ocserv.log", "/var/log/panel.log", false),
		app.FakeLogFileReader{},
		rbac.NewDefaultAuthorizer(),
	)

	result, err := svc.TailLog(context.Background(), ownerPrincipal(), "ocserv", 25)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Enabled {
		t.Errorf("expected enabled=false")
	}
	if len(result.Lines) != 0 {
		t.Errorf("expected no lines when disabled")
	}
}

func TestLogTailService_FileNotFound(t *testing.T) {
	t.Parallel()

	svc := app.NewLogTailService(
		testLogTailCfg("/nonexistent/ocserv.log", "/nonexistent/panel.log", true),
		app.FakeLogFileReader{},
		rbac.NewDefaultAuthorizer(),
	)

	result, err := svc.TailLog(context.Background(), ownerPrincipal(), "ocserv", 25)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Enabled {
		t.Errorf("expected enabled=true")
	}
	if result.Present {
		t.Errorf("expected present=false for nonexistent file")
	}
}

func TestLogTailService_NormalTail(t *testing.T) {
	t.Parallel()

	lines := []string{"line1", "line2", "line3", "line4", "line5"}
	content := strings.Join(lines, "\n") + "\n"

	reader := app.FakeLogFileReader{"/var/log/ocserv.log": []byte(content)}
	svc := app.NewLogTailService(
		testLogTailCfg("/var/log/ocserv.log", "/var/log/panel.log", true),
		reader,
		rbac.NewDefaultAuthorizer(),
	)

	result, err := svc.TailLog(context.Background(), ownerPrincipal(), "ocserv", 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Enabled || !result.Present {
		t.Errorf("expected enabled=true, present=true")
	}
	if len(result.Lines) != 3 {
		t.Errorf("expected 3 lines, got %d: %v", len(result.Lines), result.Lines)
	}
	if result.Lines[0] != "line3" || result.Lines[2] != "line5" {
		t.Errorf("expected last 3 lines: %v", result.Lines)
	}
}

func TestLogTailService_LinesCap(t *testing.T) {
	t.Parallel()

	var sb strings.Builder
	for i := 0; i < 100; i++ {
		sb.WriteString("line\n")
	}

	reader := app.FakeLogFileReader{"/var/log/ocserv.log": []byte(sb.String())}
	svc := app.NewLogTailService(
		testLogTailCfg("/var/log/ocserv.log", "/var/log/panel.log", true),
		reader,
		rbac.NewDefaultAuthorizer(),
	)

	// Request 200 lines — hard cap should clamp to 50
	result, err := svc.TailLog(context.Background(), ownerPrincipal(), "ocserv", 200)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Lines) > 50 {
		t.Errorf("expected max 50 lines, got %d", len(result.Lines))
	}
}

func TestLogTailService_ReadError(t *testing.T) {
	t.Parallel()

	reader := app.FakeLogFileReader{"/var/log/ocserv.log": nil, "__err__/var/log/ocserv.log": []byte("permission denied")}
	reader2 := app.ErrorLogFileReader{Path: "/var/log/ocserv.log", Err: errors.New("permission denied")}
	svc := app.NewLogTailService(
		testLogTailCfg("/var/log/ocserv.log", "/var/log/panel.log", true),
		reader2,
		rbac.NewDefaultAuthorizer(),
	)

	result, err := svc.TailLog(context.Background(), ownerPrincipal(), "ocserv", 25)
	if err != nil {
		t.Fatalf("unexpected error (should be in result, not returned): %v", err)
	}
	if result.Error == "" {
		t.Errorf("expected error field to be set on read failure")
	}
}

func TestLogTailService_InvalidSource(t *testing.T) {
	t.Parallel()

	svc := app.NewLogTailService(
		testLogTailCfg("/var/log/ocserv.log", "/var/log/panel.log", true),
		app.FakeLogFileReader{},
		rbac.NewDefaultAuthorizer(),
	)

	_, err := svc.TailLog(context.Background(), ownerPrincipal(), "invalid", 25)
	if err == nil {
		t.Errorf("expected error for invalid source")
	}
}

func TestLogTailService_Unauthenticated(t *testing.T) {
	t.Parallel()

	svc := app.NewLogTailService(
		testLogTailCfg("/var/log/ocserv.log", "/var/log/panel.log", true),
		app.FakeLogFileReader{},
		rbac.NewDefaultAuthorizer(),
	)

	_, err := svc.TailLog(context.Background(), auth.Principal{}, "ocserv", 25)
	if err != app.ErrUnauthenticated {
		t.Fatalf("expected ErrUnauthenticated, got %v", err)
	}
}
```

- [ ] **Step 2: Run to confirm it fails**

```bash
cd /opt/ocserv_panel/backend && go test ./internal/app/... -run TestLogTailService -count=1 2>&1 | head -20
```
Expected: compile error "undefined: app.NewLogTailService"

- [ ] **Step 3: Implement `log_tail_service.go`**

Create `backend/internal/app/log_tail_service.go`:
```go
package app

import (
	"bytes"
	"context"
	"fmt"
	"os"

	"ocserv_panel/backend/internal/auth"
	"ocserv_panel/backend/internal/config"
	"ocserv_panel/backend/internal/rbac"
)

const logTailMaxLines = 50
const logTailDefaultLines = 25

// LogFileReader is the interface used to read log files. It is abstracted for testing.
type LogFileReader interface {
	ReadFile(path string) ([]byte, error)
}

// OSLogFileReader reads files from the real filesystem.
type OSLogFileReader struct{}

func (r OSLogFileReader) ReadFile(path string) ([]byte, error) { return os.ReadFile(path) }

// FakeLogFileReader is a test double keyed by path. A nil value means "file not found".
type FakeLogFileReader map[string][]byte

func (f FakeLogFileReader) ReadFile(path string) ([]byte, error) {
	data, ok := f[path]
	if !ok {
		return nil, os.ErrNotExist
	}
	return data, nil
}

// ErrorLogFileReader always returns an error for a specific path.
type ErrorLogFileReader struct {
	Path string
	Err  error
}

func (r ErrorLogFileReader) ReadFile(path string) ([]byte, error) {
	if path == r.Path {
		return nil, r.Err
	}
	return nil, os.ErrNotExist
}

// LogTailResult is the response payload for GET /api/logs/tail.
type LogTailResult struct {
	Source  string   `json:"source"`
	Enabled bool     `json:"enabled"`
	Present bool     `json:"present"`
	Path    string   `json:"path"`
	Lines   []string `json:"lines"`
	Error   string   `json:"error,omitempty"`
}

// LogTailService reads the tail of configured log files.
type LogTailService struct {
	cfgFn      func() config.Config
	reader     LogFileReader
	authorizer rbac.Authorizer
}

// NewLogTailService returns a LogTailService.
func NewLogTailService(cfgFn func() config.Config, reader LogFileReader, authorizer rbac.Authorizer) *LogTailService {
	return &LogTailService{cfgFn: cfgFn, reader: reader, authorizer: authorizer}
}

// TailLog returns the last N lines from the configured log file for source.
// source must be "ocserv" or "panel". lines is clamped to [1, 50].
func (s *LogTailService) TailLog(ctx context.Context, principal auth.Principal, source string, lines int) (LogTailResult, error) {
	if !principal.Authenticated() {
		return LogTailResult{}, ErrUnauthenticated
	}
	if !s.authorizer.Allowed(principalRoles(principal), rbac.PermissionReadDashboard) {
		return LogTailResult{}, ErrPermissionDenied
	}

	cfg := s.cfgFn()

	var path string
	switch source {
	case "ocserv":
		path = cfg.Installation.OcservLogPath
	case "panel":
		path = cfg.Installation.PanelLogPath
	default:
		return LogTailResult{}, fmt.Errorf("invalid log source %q: must be ocserv or panel", source)
	}

	if lines < 1 {
		lines = logTailDefaultLines
	}
	if lines > logTailMaxLines {
		lines = logTailMaxLines
	}

	enabled := cfg.Monitoring.LogFileReadEnabled
	if !enabled {
		return LogTailResult{Source: source, Enabled: false, Present: false, Path: path, Lines: []string{}}, nil
	}

	data, err := s.reader.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return LogTailResult{Source: source, Enabled: true, Present: false, Path: path, Lines: []string{}}, nil
		}
		return LogTailResult{
			Source:  source,
			Enabled: true,
			Present: false,
			Path:    path,
			Lines:   []string{},
			Error:   "log file could not be read",
		}, nil
	}

	tail := tailLines(data, lines)
	return LogTailResult{Source: source, Enabled: true, Present: true, Path: path, Lines: tail}, nil
}

// tailLines returns the last n non-empty lines from data.
func tailLines(data []byte, n int) []string {
	data = bytes.TrimRight(data, "\n\r")
	if len(data) == 0 {
		return []string{}
	}
	all := bytes.Split(data, []byte("\n"))
	if len(all) <= n {
		result := make([]string, 0, len(all))
		for _, l := range all {
			result = append(result, string(l))
		}
		return result
	}
	tail := all[len(all)-n:]
	result := make([]string, 0, len(tail))
	for _, l := range tail {
		result = append(result, string(l))
	}
	return result
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd /opt/ocserv_panel/backend && go test ./internal/app/... -run TestLogTailService -count=1 -v
```
Expected: all tests PASS

- [ ] **Step 5: Run full test suite**

```bash
cd /opt/ocserv_panel/backend && go test ./... -count=1 -timeout 120s
```
Expected: all pass

- [ ] **Step 6: Commit**

```bash
git add backend/internal/app/log_tail_service.go backend/internal/app/log_tail_service_test.go
git commit -m "feat(app): add LogTailService for log file tail reads"
```

---

## Task 5: Add HTTP handlers and wire into server

**Files:**
- Create: `backend/internal/server/health_handlers.go`
- Create: `backend/internal/server/health_handlers_test.go`
- Modify: `backend/internal/server/server.go`
- Modify: `backend/cmd/ocserv-panel/main.go`

- [ ] **Step 1: Write failing handler tests**

Create `backend/internal/server/health_handlers_test.go`:
```go
package server

import (
	"encoding/json"
	"net/http"
	"testing"

	"ocserv_panel/backend/internal/app"
	"ocserv_panel/backend/internal/config"
	"ocserv_panel/backend/internal/ocserv"
	"ocserv_panel/backend/internal/rbac"
)

func newTestHarnessWithHealth(t *testing.T) *testHarness {
	t.Helper()
	h := newTestHarness(t)
	cfg := config.Defaults()
	cfgFn := func() config.Config { return cfg }
	checker := ocserv.NewReadinessChecker(
		func() ocserv.ReadinessConfig { return ocserv.ReadinessConfig{} },
		ocserv.NewFakeCommandRunner(),
		ocserv.NewFakeServiceManager(ocserv.ServiceInactive),
	)
	h.healthService = app.NewHealthService(checker, h.store, cfgFn, rbac.NewDefaultAuthorizer())
	h.logTailService = app.NewLogTailService(cfgFn, app.FakeLogFileReader{}, rbac.NewDefaultAuthorizer())
	h.router = NewAppRouter(
		h.router.(*appRouterWrapper).authService,
		h.router.(*appRouterWrapper).dashboardService,
		h.router.(*appRouterWrapper).monitoringService,
		h.router.(*appRouterWrapper).settingsService,
		h.router.(*appRouterWrapper).ocservService,
		h.healthService,
		h.logTailService,
		rbac.NewDefaultAuthorizer(),
		AppOptions{CookieSecure: false},
	)
	return h
}

func TestHandleSystemHealth_Unauthorized(t *testing.T) {
	t.Parallel()
	h := newTestHarness(t)
	rec := serveAPI(t, h.router, http.MethodGet, "/api/health/system", nil, "")
	assertStatus(t, rec, http.StatusUnauthorized)
}

func TestHandleSystemHealth_OK(t *testing.T) {
	t.Parallel()
	h := newTestHarness(t)
	cookie := h.loginOwner(t)
	rec := serveAPI(t, h.router, http.MethodGet, "/api/health/system", nil, cookie.Value)
	assertStatus(t, rec, http.StatusOK)

	var body map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if _, ok := body["vpnServer"]; !ok {
		t.Errorf("expected vpnServer key in response")
	}
	if _, ok := body["panel"]; !ok {
		t.Errorf("expected panel key in response")
	}
	if _, ok := body["logs"]; !ok {
		t.Errorf("expected logs key in response")
	}
}

func TestHandleLogTail_Unauthorized(t *testing.T) {
	t.Parallel()
	h := newTestHarness(t)
	rec := serveAPI(t, h.router, http.MethodGet, "/api/logs/tail?source=ocserv", nil, "")
	assertStatus(t, rec, http.StatusUnauthorized)
}

func TestHandleLogTail_OK(t *testing.T) {
	t.Parallel()
	h := newTestHarness(t)
	cookie := h.loginOwner(t)
	rec := serveAPI(t, h.router, http.MethodGet, "/api/logs/tail?source=ocserv&lines=5", nil, cookie.Value)
	assertStatus(t, rec, http.StatusOK)

	var body app.LogTailResult
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Source != "ocserv" {
		t.Errorf("expected source=ocserv, got %q", body.Source)
	}
}

func TestHandleLogTail_InvalidSource(t *testing.T) {
	t.Parallel()
	h := newTestHarness(t)
	cookie := h.loginOwner(t)
	rec := serveAPI(t, h.router, http.MethodGet, "/api/logs/tail?source=badvalue", nil, cookie.Value)
	assertStatus(t, rec, http.StatusBadRequest)
}
```

**Note:** These tests use a simpler pattern — the existing `newTestHarness` already wires up a `healthService` and `logTailService` once the `App` struct and `NewAppRouter` are updated in the next steps. The `appRouterWrapper` pattern above is illustrative; use the actual approach from the existing test harness in the codebase.

- [ ] **Step 2: Create the handlers file**

Create `backend/internal/server/health_handlers.go`:
```go
package server

import (
	"net/http"
	"strconv"

	"ocserv_panel/backend/internal/auth"
)

// handleSystemHealth returns the current system health snapshot.
func (a *App) handleSystemHealth(w http.ResponseWriter, r *http.Request) {
	if !allowMethod(w, r, http.MethodGet) {
		return
	}
	principal, ok := a.authenticate(w, r)
	if !ok {
		return
	}
	result, err := a.healthService.SystemHealth(r.Context(), principal)
	if err != nil {
		a.writeAppError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// handleLogTail returns the tail of a configured log file.
func (a *App) handleLogTail(w http.ResponseWriter, r *http.Request) {
	if !allowMethod(w, r, http.MethodGet) {
		return
	}
	principal, ok := a.authenticate(w, r)
	if !ok {
		return
	}

	source := r.URL.Query().Get("source")
	if source != "ocserv" && source != "panel" {
		writeError(w, http.StatusBadRequest, "source must be ocserv or panel")
		return
	}

	lines := 25
	if raw := r.URL.Query().Get("lines"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 {
			lines = n
		}
	}

	result, err := a.logTailService.TailLog(r.Context(), principal, source, lines)
	if err != nil {
		a.writeAppError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// authenticate is already defined in server.go — this file just uses it.
// Ensure the handlers use the existing a.authenticate(w, r) pattern.
```

**Note:** The `handleLogTail` function references `a.authenticate` which already exists in `server.go`. Keep only the two handler functions in this file — no re-declaration of helpers.

- [ ] **Step 3: Extend `App` struct and `NewAppRouter` in `server.go`**

In `backend/internal/server/server.go`, add to the `App` struct:
```go
healthService  *app.HealthService
logTailService *app.LogTailService
```

Add to `NewAppRouter` parameters (after `ocservService *app.OcservService`):
```go
healthService *app.HealthService,
logTailService *app.LogTailService,
```

In the `appHandler` initializer block, add:
```go
healthService:  healthService,
logTailService: logTailService,
```

Register the two new routes in the mux (after the ocserv routes):
```go
mux.HandleFunc("/api/health/system", appHandler.handleSystemHealth)
mux.HandleFunc("/api/logs/tail", appHandler.handleLogTail)
```

- [ ] **Step 4: Wire services in `main.go`**

In `backend/cmd/ocserv-panel/main.go`, after the `ocservService` construction, add:
```go
cfgFn := func() config.Config {
    liveCfg, err := config.Load(configPath)
    if err != nil {
        return cfg
    }
    return liveCfg
}
healthService := app.NewHealthService(readinessChecker, persistence, cfgFn, rbac.NewDefaultAuthorizer())
logTailService := app.NewLogTailService(cfgFn, app.OSLogFileReader{}, rbac.NewDefaultAuthorizer())
```

Update the `server.NewAppRouter(...)` call to pass the two new services after `ocservService`.

- [ ] **Step 5: Update test harness in `server_test.go`**

In `newTestHarnessWithCookieSecure`, add health and log tail services:
```go
cfg := config.Defaults()
cfgFn := func() config.Config { return cfg }
checker2 := ocserv.NewReadinessChecker(func() ocserv.ReadinessConfig { return ocserv.ReadinessConfig{} }, runner, fakeSvc)
// Use existing runner/fakeSvc from newTestOcservServiceWithFake or create fresh ones:
fakeRunner := ocserv.NewFakeCommandRunner()
fakeSvcMgr := ocserv.NewFakeServiceManager(ocserv.ServiceInactive)
checker3 := ocserv.NewReadinessChecker(func() ocserv.ReadinessConfig { return ocserv.ReadinessConfig{} }, fakeRunner, fakeSvcMgr)
healthSvc := app.NewHealthService(checker3, s, cfgFn, rbac.NewDefaultAuthorizer())
logTailSvc := app.NewLogTailService(cfgFn, app.FakeLogFileReader{}, rbac.NewDefaultAuthorizer())
```

And pass them to `NewAppRouter(...)`:
```go
return &testHarness{
    ...
    router: NewAppRouter(
        authService,
        dashboardService,
        monitoringService,
        settingsService,
        ocservSvc,
        healthSvc,
        logTailSvc,
        rbac.NewDefaultAuthorizer(),
        AppOptions{CookieSecure: cookieSecure},
    ),
}
```

Also update `newTestHarnessWithTempConfig` if it calls `NewAppRouter` directly.

- [ ] **Step 6: Build and run all tests**

```bash
cd /opt/ocserv_panel/backend && go build ./... && go test ./... -count=1 -timeout 120s
```
Expected: all pass

- [ ] **Step 7: Commit**

```bash
git add backend/internal/server/health_handlers.go backend/internal/server/health_handlers_test.go backend/internal/server/server.go backend/cmd/ocserv-panel/main.go
git commit -m "feat(server): add /api/health/system and /api/logs/tail endpoints"
```

---

## Task 6: Add frontend API types and fetch functions

**Files:**
- Modify: `frontend/src/lib/api.ts`

- [ ] **Step 1: Add TypeScript types and fetch functions**

In `frontend/src/lib/api.ts`, add the following types after the existing `OcservStatus` type:

```typescript
export type VPNServerHealth = {
  systemdStatus: string;
  processRunning: boolean;
  socketPresent: boolean;
  configPresent: boolean;
  configValid: boolean;
  configErrors?: string;
  binaryPresent: boolean;
  binaryVersion?: string;
};

export type PanelHealth = {
  backendStatus: string;
  dbReachable: boolean;
  dbPath: string;
  configPresent: boolean;
  configPath: string;
  webuiUserCount: number;
};

export type LogsHealth = {
  ocservLogEnabled: boolean;
  ocservLogPresent: boolean;
  ocservLogPath: string;
  panelLogEnabled: boolean;
  panelLogPresent: boolean;
  panelLogPath: string;
};

export type SystemHealth = {
  vpnServer: VPNServerHealth;
  panel: PanelHealth;
  logs: LogsHealth;
};

export type LogTailResult = {
  source: string;
  enabled: boolean;
  present: boolean;
  path: string;
  lines: string[];
  error?: string;
};
```

Add these fetch functions at the end of the file, before the `principalFromAuth` function:

```typescript
export function getSystemHealth(): Promise<SystemHealth> {
  return apiFetch<SystemHealth>('/api/health/system');
}

export function getLogTail(source: 'ocserv' | 'panel', lines = 25): Promise<LogTailResult> {
  return apiFetch<LogTailResult>(`/api/logs/tail?source=${source}&lines=${lines}`);
}
```

- [ ] **Step 2: Check TypeScript compiles cleanly**

```bash
cd /opt/ocserv_panel/frontend && npx tsc --noEmit 2>&1 | head -30
```
Expected: no errors

- [ ] **Step 3: Commit**

```bash
git add frontend/src/lib/api.ts
git commit -m "feat(frontend): add SystemHealth and LogTailResult types and API functions"
```

---

## Task 7: Create `HealthCard.svelte` component

**Files:**
- Create: `frontend/src/lib/components/ui/HealthCard.svelte`

- [ ] **Step 1: Create the component**

Create `frontend/src/lib/components/ui/HealthCard.svelte`:
```svelte
<script lang="ts">
  import { formatRelativeTime } from '$lib/utils/format';

  export type HealthStatus = 'healthy' | 'warning' | 'critical' | 'unknown';

  export type HealthItem = {
    label: string;
    status: HealthStatus;
    detail?: string;
  };

  type Props = {
    title: string;
    items: HealthItem[];
    lastChecked?: Date | null;
    loading?: boolean;
  };

  let { title, items, lastChecked = null, loading = false }: Props = $props();

  const STATUS_DOT: Record<HealthStatus, string> = {
    healthy:  'bg-emerald-500',
    warning:  'bg-amber-400',
    critical: 'bg-red-500',
    unknown:  'bg-slate-400',
  };

  const STATUS_TEXT: Record<HealthStatus, string> = {
    healthy:  'text-emerald-700',
    warning:  'text-amber-700',
    critical: 'text-red-700',
    unknown:  'text-slate-500',
  };

  const STATUS_LABEL: Record<HealthStatus, string> = {
    healthy:  'Healthy',
    warning:  'Warning',
    critical: 'Critical',
    unknown:  'Unknown',
  };

  // Overall card severity: highest status wins
  const overallStatus = $derived<HealthStatus>(
    items.some(i => i.status === 'critical') ? 'critical' :
    items.some(i => i.status === 'warning')  ? 'warning'  :
    items.every(i => i.status === 'healthy') ? 'healthy'  :
    'unknown'
  );

  const CARD_BORDER: Record<HealthStatus, string> = {
    healthy:  'border-emerald-200',
    warning:  'border-amber-200',
    critical: 'border-red-200',
    unknown:  'border-slate-200',
  };
</script>

<div class="bg-white border {CARD_BORDER[overallStatus]} rounded-lg p-5 shadow-sm">
  <div class="flex items-center justify-between mb-4">
    <h3 class="text-sm font-semibold text-slate-700">{title}</h3>
    {#if loading}
      <span class="text-xs text-slate-400">Checking…</span>
    {:else if lastChecked}
      <span class="text-xs text-slate-400">Checked {formatRelativeTime(lastChecked.toISOString())}</span>
    {/if}
  </div>

  <ul class="space-y-3">
    {#each items as item}
      <li class="flex items-start gap-2.5">
        <span
          class="mt-1 w-2.5 h-2.5 rounded-full flex-shrink-0 {STATUS_DOT[item.status]}"
          role="img"
          aria-label="{item.status}"
        ></span>
        <div class="min-w-0 flex-1">
          <div class="flex items-baseline justify-between gap-2">
            <span class="text-sm text-slate-700">{item.label}</span>
            <span class="text-xs font-medium {STATUS_TEXT[item.status]}">{STATUS_LABEL[item.status]}</span>
          </div>
          {#if item.detail}
            <p class="text-xs text-slate-500 mt-0.5 break-words">{item.detail}</p>
          {/if}
        </div>
      </li>
    {/each}
  </ul>
</div>
```

- [ ] **Step 2: Verify TypeScript compiles**

```bash
cd /opt/ocserv_panel/frontend && npx tsc --noEmit 2>&1 | head -20
```
Expected: no errors

- [ ] **Step 3: Commit**

```bash
git add frontend/src/lib/components/ui/HealthCard.svelte
git commit -m "feat(frontend): add HealthCard component with health/warning/critical/unknown states"
```

---

## Task 8: Update dashboard page — health cards and tabbed audit/logs section

**Files:**
- Modify: `frontend/src/routes/+page.svelte`

- [ ] **Step 1: Add health state and loading to the script block**

In the `<script lang="ts">` section of `frontend/src/routes/+page.svelte`:

1. Add import at the top:
```typescript
import HealthCard from '$lib/components/ui/HealthCard.svelte';
import type { HealthItem } from '$lib/components/ui/HealthCard.svelte';
import {
  getSystemHealth,
  getLogTail,
  type SystemHealth,
  type LogTailResult
} from '$lib/api';
```

2. Add state variables (alongside existing `let loading`, `let summary`, etc.):
```typescript
let health = $state<SystemHealth | null>(null);
let healthLoading = $state(false);
let healthError = $state('');
let healthCheckedAt = $state<Date | null>(null);
let healthRefreshInterval: ReturnType<typeof setInterval> | null = null;

// Log tabs
let activeLogTab = $state<'audit' | 'ocserv' | 'panel'>('audit');
let ocservLog = $state<LogTailResult | null>(null);
let panelLog = $state<LogTailResult | null>(null);
let logLoading = $state(false);
let logError = $state('');
```

3. In `onMount`, add the health fetch and interval:
```typescript
void loadHealth();
healthRefreshInterval = setInterval(() => {
  if (document.visibilityState === 'visible') {
    void loadHealth();
  }
}, 30_000);
```

4. In `onDestroy`, add:
```typescript
if (healthRefreshInterval) clearInterval(healthRefreshInterval);
```

5. Add helper functions:
```typescript
async function loadHealth() {
  healthLoading = true;
  healthError = '';
  try {
    health = await getSystemHealth();
    healthCheckedAt = new Date();
  } catch (caught) {
    healthError = caught instanceof Error ? caught.message : 'Unable to load system health.';
  } finally {
    healthLoading = false;
  }
}

async function loadLogTab(source: 'ocserv' | 'panel') {
  logLoading = true;
  logError = '';
  try {
    const result = await getLogTail(source, 25);
    if (source === 'ocserv') ocservLog = result;
    else panelLog = result;
  } catch (caught) {
    logError = caught instanceof Error ? caught.message : 'Unable to load log.';
  } finally {
    logLoading = false;
  }
}

function handleLogTabChange(tab: 'audit' | 'ocserv' | 'panel') {
  activeLogTab = tab;
  if (tab === 'ocserv' && !ocservLog) void loadLogTab('ocserv');
  if (tab === 'panel' && !panelLog) void loadLogTab('panel');
}

// Derive HealthCard item arrays from health response
const vpnServerItems = $derived<HealthItem[]>(() => {
  if (!health) return [];
  const v = health.vpnServer;
  const svcStatus = v.systemdStatus === 'active' ? 'healthy'
    : v.systemdStatus === 'failed' ? 'critical'
    : v.systemdStatus === 'inactive' ? 'warning'
    : 'unknown';
  return [
    { label: 'Systemd service', status: svcStatus, detail: svcStatus !== 'healthy' ? `Status: ${v.systemdStatus}` : undefined },
    { label: 'Socket / process', status: v.socketPresent ? 'healthy' : 'warning' },
    {
      label: 'Config file',
      status: v.configPresent && v.configValid ? 'healthy' : v.configPresent && !v.configValid ? 'critical' : 'warning',
      detail: v.configErrors || undefined
    },
  ];
}());

const panelItems = $derived<HealthItem[]>(() => {
  if (!health) return [];
  const p = health.panel;
  return [
    { label: 'Backend', status: 'healthy' },
    { label: 'Database', status: p.dbReachable ? 'healthy' : 'critical' },
    { label: 'Config file', status: p.configPresent ? 'healthy' : 'warning' },
    { label: `WebUI users`, status: 'healthy', detail: `${p.webuiUserCount} user${p.webuiUserCount === 1 ? '' : 's'}` },
  ];
}());
```

- [ ] **Step 2: Replace top stat cards with HealthCard components in the template**

In the `{:else if summary}` block, replace:
```svelte
<!-- Summary stat cards -->
<div class="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-4 gap-6 mb-8">
  <CardStats
    title="Backend status"
    ...
  />
  <CardStats
    title="ocserv status"
    ...
  />
  <CardStats
    title="Active sessions"
    ...
  />
  <CardStats
    title="Sample data"
    ...
  />
</div>
```

With:
```svelte
<!-- Health cards -->
<div class="grid grid-cols-1 md:grid-cols-2 gap-6 mb-8">
  {#if healthError}
    <div class="col-span-2 bg-amber-50 border border-amber-200 rounded-lg p-4 text-sm text-amber-700">
      Health check unavailable: {healthError}
    </div>
  {:else}
    <HealthCard
      title="VPN Server"
      items={vpnServerItems}
      lastChecked={healthCheckedAt}
      loading={healthLoading && !health}
    />
    <HealthCard
      title="WebUI Panel"
      items={panelItems}
      lastChecked={healthCheckedAt}
      loading={healthLoading && !health}
    />
  {/if}
</div>

<!-- Secondary stat cards (sessions + sample data) -->
<div class="grid grid-cols-1 md:grid-cols-2 gap-6 mb-8">
  <CardStats
    title="Active sessions"
    value={String(summary.activeSessionCount ?? 0)}
    icon={Users}
  />
  <CardStats
    title="Sample data"
    value={sampleDataMessage}
    icon={Database}
  />
</div>
```

- [ ] **Step 3: Replace the audit section with a tabbed audit/logs section**

Replace the `<!-- Recent audit section -->` block at the bottom with:

```svelte
<!-- Audit / Logs section -->
<section aria-labelledby="audit-logs-heading" class="mb-8">
  <h2 id="audit-logs-heading" class="sr-only">Audit &amp; Logs</h2>

  <!-- Tab bar -->
  <div class="flex gap-1 mb-4 border-b border-slate-200">
    {#each [['audit', 'Audit Events'], ['ocserv', 'ocserv Logs'], ['panel', 'Panel Logs']] as [tab, label]}
      <button
        type="button"
        onclick={() => handleLogTabChange(tab as 'audit' | 'ocserv' | 'panel')}
        class="px-4 py-2 text-sm font-medium rounded-t-md transition-colors
          {activeLogTab === tab
            ? 'bg-white border border-b-white border-slate-200 -mb-px text-slate-800'
            : 'text-slate-500 hover:text-slate-700'}"
        aria-selected={activeLogTab === tab}
        role="tab"
      >
        {label}
      </button>
    {/each}
  </div>

  <!-- Audit Events tab -->
  {#if activeLogTab === 'audit'}
    {#if recentAudit.length > 0}
      <CardTable title="Recent Audit">
        <PaginatedTable
          totalPages={pagedAudit.totalPages}
          currentPage={pagedAudit.page}
          onPageChange={(p) => (auditPage = p)}
        >
          <table class="w-full text-sm">
            <thead class="bg-slate-50">
              <tr class="text-left text-slate-500 font-medium">
                <th class="px-6 py-3">Time</th>
                <th class="px-6 py-3">Actor</th>
                <th class="px-6 py-3">Action</th>
                <th class="px-6 py-3">Result</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-slate-100">
              {#each pagedAudit.items as event}
                <tr class="hover:bg-slate-50">
                  <td class="px-6 py-3 text-slate-700 whitespace-nowrap" title={event.createdAt ?? event.timestamp ?? ''}>{formatRelativeTime(event.createdAt ?? event.timestamp)}</td>
                  <td class="px-6 py-3 text-slate-700">{event.actorUsername ?? event.actor ?? '—'}</td>
                  <td class="px-6 py-3 text-slate-700">{event.action ?? event.message ?? '—'}</td>
                  <td class="px-6 py-3">
                    <span class="inline-block px-2 py-0.5 text-xs font-medium rounded-full {event.outcome === 'success' || event.result === 'success' ? 'bg-emerald-50 text-emerald-700' : event.outcome === 'failure' || event.result === 'failure' ? 'bg-red-50 text-red-700' : 'bg-slate-100 text-slate-600'}">
                      {event.outcome ?? event.result ?? '—'}
                    </span>
                  </td>
                </tr>
              {/each}
            </tbody>
          </table>
        </PaginatedTable>
      </CardTable>
    {:else}
      <p class="text-sm text-slate-500">No audit events yet.</p>
    {/if}
  {/if}

  <!-- ocserv Logs tab -->
  {#if activeLogTab === 'ocserv'}
    {#if logLoading && !ocservLog}
      <div class="flex items-center gap-2 py-8 text-slate-500 text-sm">
        <div class="w-4 h-4 border-2 border-slate-200 border-t-slate-500 rounded-full animate-spin"></div>
        Loading ocserv log…
      </div>
    {:else if ocservLog}
      {#if !ocservLog.enabled}
        <div class="bg-slate-50 border border-slate-200 rounded-lg p-4 text-sm text-slate-600">
          Log file reading is disabled.
          <a href="/settings#monitoring" class="underline text-slate-700 ml-1">Enable it in Settings → Monitoring.</a>
        </div>
      {:else if !ocservLog.present}
        <div class="bg-amber-50 border border-amber-200 rounded-lg p-4 text-sm text-amber-700">
          Log file not found at <code class="font-mono">{ocservLog.path}</code>.
          <a href="/settings#installation" class="underline text-amber-800 ml-1">Update the path in Settings → Installation.</a>
        </div>
      {:else if ocservLog.error}
        <div class="bg-red-50 border border-red-200 rounded-lg p-4 text-sm text-red-700">
          {ocservLog.error}
        </div>
      {:else}
        <div class="bg-white border border-slate-200 rounded-lg">
          <div class="flex items-center justify-between px-4 py-2 border-b border-slate-100">
            <span class="text-xs text-slate-500 font-mono">{ocservLog.path}</span>
            <button
              type="button"
              onclick={() => loadLogTab('ocserv')}
              disabled={logLoading}
              class="text-xs text-slate-500 hover:text-slate-700 disabled:opacity-50"
            >Refresh</button>
          </div>
          <pre class="text-xs font-mono text-slate-700 p-4 overflow-x-auto max-h-64 overflow-y-auto">{ocservLog.lines.join('\n')}</pre>
        </div>
      {/if}
    {/if}
  {/if}

  <!-- Panel Logs tab -->
  {#if activeLogTab === 'panel'}
    {#if logLoading && !panelLog}
      <div class="flex items-center gap-2 py-8 text-slate-500 text-sm">
        <div class="w-4 h-4 border-2 border-slate-200 border-t-slate-500 rounded-full animate-spin"></div>
        Loading panel log…
      </div>
    {:else if panelLog}
      {#if !panelLog.enabled}
        <div class="bg-slate-50 border border-slate-200 rounded-lg p-4 text-sm text-slate-600">
          Log file reading is disabled.
          <a href="/settings#monitoring" class="underline text-slate-700 ml-1">Enable it in Settings → Monitoring.</a>
        </div>
      {:else if !panelLog.present}
        <div class="bg-amber-50 border border-amber-200 rounded-lg p-4 text-sm text-amber-700">
          Log file not found at <code class="font-mono">{panelLog.path}</code>.
          <a href="/settings#installation" class="underline text-amber-800 ml-1">Update the path in Settings → Installation.</a>
        </div>
      {:else if panelLog.error}
        <div class="bg-red-50 border border-red-200 rounded-lg p-4 text-sm text-red-700">
          {panelLog.error}
        </div>
      {:else}
        <div class="bg-white border border-slate-200 rounded-lg">
          <div class="flex items-center justify-between px-4 py-2 border-b border-slate-100">
            <span class="text-xs text-slate-500 font-mono">{panelLog.path}</span>
            <button
              type="button"
              onclick={() => loadLogTab('panel')}
              disabled={logLoading}
              class="text-xs text-slate-500 hover:text-slate-700 disabled:opacity-50"
            >Refresh</button>
          </div>
          <pre class="text-xs font-mono text-slate-700 p-4 overflow-x-auto max-h-64 overflow-y-auto">{panelLog.lines.join('\n')}</pre>
        </div>
      {/if}
    {/if}
  {/if}
</section>
```

- [ ] **Step 4: Verify the frontend builds**

```bash
cd /opt/ocserv_panel/frontend && npm run build 2>&1 | tail -20
```
Expected: build succeeds with no errors

- [ ] **Step 5: Run TypeScript check**

```bash
cd /opt/ocserv_panel/frontend && npx tsc --noEmit 2>&1 | head -30
```
Expected: no errors

- [ ] **Step 6: Commit**

```bash
git add frontend/src/routes/+page.svelte
git commit -m "feat(frontend): replace stat cards with HealthCard components, add tabbed audit/logs section"
```

---

## Task 9: Final verification

- [ ] **Step 1: Run full backend test suite**

```bash
cd /opt/ocserv_panel/backend && go test ./... -count=1 -timeout 120s -v 2>&1 | tail -40
```
Expected: all packages pass, no failures

- [ ] **Step 2: Run frontend build and type check**

```bash
cd /opt/ocserv_panel/frontend && npm run build 2>&1 | tail -10 && npx tsc --noEmit
```
Expected: build succeeds, no type errors

- [ ] **Step 3: Verify new endpoints are reachable (optional smoke test if backend running)**

```bash
curl -s http://localhost:8080/api/health/system | python3 -m json.tool 2>/dev/null || echo "backend not running, skip"
```

- [ ] **Step 4: Final commit with summary**

```bash
git log --oneline -8
```

---

## Self-Review Checklist

**Spec coverage:**
- ✅ `GET /api/health/system` — Task 3 (service) + Task 5 (handler)
- ✅ `GET /api/logs/tail` — Task 4 (service) + Task 5 (handler)
- ✅ `HealthService` with `ReadinessChecker` reuse — Task 3
- ✅ `LogTailService` with `LogFileReader` interface + fakes — Task 4
- ✅ New settings keys (`ocserv_log_path`, `panel_log_path`, `log_file_read_enabled`) — Tasks 1 + 2
- ✅ Config struct additions — Task 1
- ✅ Settings registry + apply + read cases — Task 2
- ✅ `HealthCard.svelte` component with 4 states — Task 7
- ✅ Dashboard top section replaced with two `HealthCard` instances — Task 8
- ✅ VPN Server card items derived from `health.vpnServer` — Task 8
- ✅ WebUI Panel card items derived from `health.panel` — Task 8
- ✅ Tabbed audit/logs section (3 tabs) — Task 8
- ✅ Log tab: disabled / file not found / error / normal states — Task 8
- ✅ Log fetch on tab switch + Refresh button — Task 8
- ✅ Health fetch every 30s — Task 8
- ✅ Settings page always editable (no changes to settings page) — by design (no settings page modification)
- ✅ Tests: HealthService unit tests — Task 3
- ✅ Tests: LogTailService unit tests (disabled/not found/normal/cap/error/invalid source/unauth) — Task 4
- ✅ Tests: handler tests for both endpoints (auth, 200, bad source) — Task 5
- ✅ Frontend API types — Task 6

**Type consistency check:**
- `SystemHealthResult` defined in Task 3, used in Task 5 handler and Task 6 frontend type `SystemHealth`
- `LogTailResult` defined in Task 4, used in Task 5 handler and Task 6 frontend type `LogTailResult`
- `FakeLogFileReader` and `ErrorLogFileReader` defined in Task 4 `log_tail_service.go`, used in Task 4 tests and Task 5 test harness
- `OSLogFileReader` defined in Task 4, used in Task 5 `main.go` wiring
- `HealthItem` exported from `HealthCard.svelte`, imported in Task 8 dashboard
- All method names consistent across tasks
