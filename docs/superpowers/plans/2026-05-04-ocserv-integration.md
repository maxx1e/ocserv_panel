# ocserv Integration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the NoopManager stub and hardcoded "not_configured" dashboard status with real ocserv integration: binary detection, readiness checking, systemd service management, active session management via occtl, user management via ocpasswd, and production config guidance.

**Architecture:** Minimal adapter layer — thin Go adapters wrapping command execution (occtl, ocpasswd, systemctl) behind interfaces. Each adapter is independently testable with fakes. Dashboard service gets real readiness check injected at startup. Frontend gets real ocserv status and management UI.

**Tech Stack:** Go backend, SvelteKit frontend, SQLite persistence, existing patterns (constructor injection, RBAC, audit recording, table-driven tests).

---

## File Structure

### New Files

| File | Responsibility |
|------|---------------|
| `backend/internal/ocserv/readiness.go` | OcservReadiness struct and OcservReadinessChecker |
| `backend/internal/ocserv/readiness_test.go` | Tests for readiness checker |
| `backend/internal/ocserv/service.go` | ServiceManager interface, ServiceStatus enum |
| `backend/internal/ocserv/service_systemd.go` | SystemdManager implementation |
| `backend/internal/ocserv/service_fake.go` | FakeServiceManager for tests |
| `backend/internal/ocserv/service_test.go` | Tests for systemd manager |
| `backend/internal/ocserv/occtl_adapter.go` | OcctlAdapter implementing Manager interface |
| `backend/internal/ocserv/occtl_adapter_test.go` | Tests for occtl adapter |
| `backend/internal/ocserv/passwd.go` | PasswdManager interface, PasswdUser struct |
| `backend/internal/ocserv/passwd_adapter.go` | OcpasswdAdapter implementation |
| `backend/internal/ocserv/passwd_adapter_test.go` | Tests for ocpasswd adapter |
| `backend/internal/ocserv/passwd_fake.go` | FakePasswdManager for tests |
| `backend/internal/ocserv/config_validator.go` | ConfigValidator and ConfigIssue |
| `backend/internal/ocserv/config_validator_test.go` | Tests for config validator |
| `backend/internal/ocserv/runner.go` | CommandRunner interface for testable exec |
| `backend/internal/app/ocserv_service.go` | OcservService orchestrating readiness, sessions, users, service, config |
| `backend/internal/app/ocserv_service_test.go` | Tests for ocserv service |
| `backend/internal/server/ocserv_handlers.go` | HTTP handlers for ocserv API endpoints |
| `backend/internal/server/ocserv_handlers_test.go` | Tests for ocserv handlers |
| `deploy/sudoers.d/ocserv-panel` | Sudoers rules for ocserv operations |

### Modified Files

| File | Changes |
|------|---------|
| `backend/internal/config/config.go` | Add OcservBinPath, OcctlBinPath, OcpasswdBinPath, OcpasswdFilePath (rename from OcpasswdPath), SystemdUnitName |
| `backend/internal/ocserv/client.go` | Expand Manager interface with ListUsers, GetSessionDetails; rename NoopManager to FakeManager |
| `backend/internal/ocserv/client_test.go` | Update for FakeManager rename |
| `backend/internal/app/dashboard_service.go` | Replace hardcoded OcservStatusSummary with readiness check |
| `backend/internal/app/settings_registry.go` | Add new settings, rename ocpasswd_path |
| `backend/internal/rbac/rbac.go` | Add ocserv permissions |
| `backend/internal/server/server.go` | Add ocserv service to App, add API routes, update response types |
| `backend/cmd/ocserv-panel/main.go` | Wire ocserv adapters and service |
| `frontend/src/lib/api.ts` | Add ocserv API types and functions |
| `frontend/src/routes/+page.svelte` | Replace hardcoded ocserv status with real data |

---

## Task 1: Config & Binary Path Extensions

**Files:**
- Modify: `backend/internal/config/config.go`
- Modify: `backend/internal/app/settings_registry.go`
- Test: `backend/internal/config/config_test.go` (if exists, otherwise add assertions to existing)

- [ ] **Step 1: Add new fields to InstallationConfig and ServiceConfig in config.go**

Add `OcservBinPath`, `OcctlBinPath`, `OcpasswdBinPath` to `InstallationConfig`. Rename `OcpasswdPath` to `OcpasswdFilePath`. Add `SystemdUnitName` to `ServiceConfig`. Update `Defaults()`.

```go
// In InstallationConfig, add after OcservSocketPath:
OcservBinPath    string `toml:"ocserv_bin_path"`
OcctlBinPath     string `toml:"occtl_bin_path"`
OcpasswdBinPath  string `toml:"ocpasswd_bin_path"`
OcpasswdFilePath string `toml:"ocpasswd_file_path"` // renamed from OcpasswdPath

// In ServiceConfig, add after MaintenanceMode:
SystemdUnitName string `toml:"systemd_unit_name"`
```

Update `Defaults()` to include:
```go
OcservBinPath:    "/usr/local/sbin/ocserv",
OcctlBinPath:     "/usr/local/bin/occtl",
OcpasswdBinPath:  "/usr/local/bin/ocpasswd",
OcpasswdFilePath: "/etc/ocserv/ocpasswd",
// ... and in ServiceConfig:
SystemdUnitName:  "ocserv",
```

Remove the old `OcpasswdPath` field.

- [ ] **Step 2: Add backward compatibility for OcpasswdPath in Load()**

In `config.go`'s `Load()` function, after unmarshaling, add: if `OcpasswdFilePath` is empty but the TOML had `ocpasswd_path`, read it from the raw data. Since we're using struct tags, the old TOML key `ocpasswd_path` won't map to the new `ocpasswd_file_path` tag. Add a fallback:

```go
// After unmarshaling, check for legacy key
if cfg.Installation.OcpasswdFilePath == "" {
    // Try to read the old key from raw TOML data
    var raw map[string]any
    if err := unmarshalTOML(data, &raw); err == nil {
        if inst, ok := raw["installation"].(map[string]any); ok {
            if v, ok := inst["ocpasswd_path"].(string); ok && v != "" {
                cfg.Installation.OcpasswdFilePath = v
            }
        }
    }
}
```

- [ ] **Step 3: Update settings_registry.go**

Replace the `installation.ocpasswd_path` setting with `installation.ocpasswd_file_path` and add three new settings:

```go
{Key: "installation.ocserv_bin_path", Label: "ocserv Binary Path", Section: "installation", Type: SettingTypeText, Source: SettingSourceConfig, RequiresRestart: true, Default: "/usr/local/sbin/ocserv", SortOrder: 5},
{Key: "installation.occtl_bin_path", Label: "occtl Binary Path", Section: "installation", Type: SettingTypeText, Source: SettingSourceConfig, RequiresRestart: true, Default: "/usr/local/bin/occtl", SortOrder: 6},
{Key: "installation.ocpasswd_bin_path", Label: "ocpasswd Binary Path", Section: "installation", Type: SettingTypeText, Source: SettingSourceConfig, RequiresRestart: true, Default: "/usr/local/bin/ocpasswd", SortOrder: 7},
{Key: "installation.ocpasswd_file_path", Label: "ocpasswd Password File", Section: "installation", Type: SettingTypeText, Source: SettingSourceConfig, RequiresRestart: true, Default: "/etc/ocserv/ocpasswd", SortOrder: 8},
```

Remove the old `installation.ocpasswd_path` entry. Add `service.systemd_unit_name`:

```go
{Key: "service.systemd_unit_name", Label: "Systemd Unit Name", Section: "service", Type: SettingTypeText, Source: SettingSourceConfig, RequiresRestart: true, Default: "ocserv", SortOrder: 7},
```

- [ ] **Step 4: Run tests to verify config changes compile and pass**

Run: `cd /opt/git/ocserv_panel/backend && go test ./internal/config/... ./internal/app/... -v -count=1`
Expected: All tests pass. Config defaults include new fields.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/config/config.go backend/internal/app/settings_registry.go
git commit -m "feat(config): add ocserv binary paths and systemd unit name

Add OcservBinPath, OcctlBinPath, OcpasswdBinPath to InstallationConfig.
Rename OcpasswdPath to OcpasswdFilePath for clarity. Add SystemdUnitName
to ServiceConfig. Update settings registry with new installation and
service settings. Backward-compatible with old ocpasswd_path TOML key."
```

---

## Task 2: Command Runner Interface

**Files:**
- Create: `backend/internal/ocserv/runner.go`

- [ ] **Step 1: Write the CommandRunner interface**

Create `backend/internal/ocserv/runner.go`:

```go
package ocserv

import (
	"context"
	"io"
)

// CommandResult holds the output of a command execution.
type CommandResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

// CommandRunner executes system commands with timeout and output capture.
// This interface enables testing by replacing real command execution with fakes.
type CommandRunner interface {
	// Run executes a command with the given arguments and returns the result.
	// The context is used for timeout/cancellation.
	// stdin, if not nil, is piped to the command's standard input.
	Run(ctx context.Context, name string, args []string, stdin io.Reader) (CommandResult, error)
}
```

- [ ] **Step 2: Write the OSCommandRunner implementation**

In the same file, add:

```go
import (
	"context"
	"io"
	"os/exec"
	"time"
)

// OSCommandRunner executes real system commands via exec.Command.
type OSCommandRunner struct{}

// NewOSCommandRunner returns a CommandRunner that executes real commands.
func NewOSCommandRunner() *OSCommandRunner {
	return &OSCommandRunner{}
}

// Run executes a command, captures stdout and stderr, and returns the result.
func (r *OSCommandRunner) Run(ctx context.Context, name string, args []string, stdin io.Reader) (CommandResult, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	if stdin != nil {
		cmd.Stdin = stdin
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	result := CommandResult{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		ExitCode: exitCodeFromErr(err),
	}
	if err != nil && result.ExitCode == 0 {
		// Command failed but we couldn't determine exit code
		return result, err
	}
	return result, nil
}

func exitCodeFromErr(err error) int {
	if err == nil {
		return 0
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode()
	}
	return -1
}
```

- [ ] **Step 3: Write the FakeCommandRunner for tests**

In the same file, add:

```go
// FakeCommandRunner is a test double that returns pre-configured results.
type FakeCommandRunner struct {
	Results map[string]CommandResult
	Err     error
	Calls   []FakeCommandCall
}

// FakeCommandCall records a call to the fake runner.
type FakeCommandCall struct {
	Name string
	Args []string
}

// NewFakeCommandRunner returns a FakeCommandRunner that returns success by default.
func NewFakeCommandRunner() *FakeCommandRunner {
	return &FakeCommandRunner{
		Results: make(map[string]CommandResult),
	}
}

// Run records the call and returns the configured result.
func (f *FakeCommandRunner) Run(ctx context.Context, name string, args []string, stdin io.Reader) (CommandResult, error) {
	f.Calls = append(f.Calls, FakeCommandCall{Name: name, Args: args})
	key := name + " " + strings.Join(args, " ")
	if result, ok := f.Results[key]; ok {
		return result, f.Err
	}
	if result, ok := f.Results[name]; ok {
		return result, f.Err
	}
	return CommandResult{ExitCode: 0, Stdout: "", Stderr: ""}, f.Err
}
```

- [ ] **Step 4: Run tests to verify runner compiles**

Run: `cd /opt/git/ocserv_panel/backend && go build ./internal/ocserv/...`
Expected: Compiles without errors.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/ocserv/runner.go
git commit -m "feat(ocserv): add CommandRunner interface for testable command execution

OSCommandRunner executes real system commands. FakeCommandRunner returns
pre-configured results for testing. This enables all ocserv adapters to
be tested without real binaries."
```

---

## Task 3: RBAC Permissions for ocserv

**Files:**
- Modify: `backend/internal/rbac/rbac.go`

- [ ] **Step 1: Add ocserv permissions to rbac.go**

Add new permission constants after the existing ones:

```go
// ocserv integration permissions
PermissionOcservRead             Permission = "ocserv:read"
PermissionOcservServiceRead      Permission = "ocserv:service:read"
PermissionOcservServiceManage    Permission = "ocserv:service:manage"
PermissionOcservSessionsRead     Permission = "ocserv:sessions:read"
PermissionOcservSessionsDisconnect Permission = "ocserv:sessions:disconnect"
PermissionOcservUsersRead        Permission = "ocserv:users:read"
PermissionOcservUsersCreate      Permission = "ocserv:users:create"
PermissionOcservUsersDelete      Permission = "ocserv:users:delete"
PermissionOcservUsersUpdate      Permission = "ocserv:users:update"
PermissionOcservConfigRead       Permission = "ocserv:config:read"
```

Add these to the `RoleOwner` grants in `NewDefaultAuthorizer()`:

```go
RoleOwner: {
    // ... existing permissions ...
    PermissionOcservRead:                {},
    PermissionOcservServiceRead:         {},
    PermissionOcservServiceManage:       {},
    PermissionOcservSessionsRead:        {},
    PermissionOcservSessionsDisconnect:  {},
    PermissionOcservUsersRead:           {},
    PermissionOcservUsersCreate:         {},
    PermissionOcservUsersDelete:         {},
    PermissionOcservUsersUpdate:         {},
    PermissionOcservConfigRead:          {},
},
```

Add read-only permissions to `RoleViewer`:

```go
RoleViewer: {
    // ... existing permissions ...
    PermissionOcservRead:          {},
    PermissionOcservServiceRead:   {},
    PermissionOcservSessionsRead:  {},
    PermissionOcservUsersRead:    {},
    PermissionOcservConfigRead:   {},
},
```

- [ ] **Step 2: Run tests to verify RBAC changes**

Run: `cd /opt/git/ocserv_panel/backend && go test ./internal/rbac/... -v -count=1`
Expected: All tests pass.

- [ ] **Step 3: Commit**

```bash
git add backend/internal/rbac/rbac.go
git commit -m "feat(rbac): add ocserv integration permissions

Add read and manage permissions for ocserv readiness, service control,
sessions, users, and config validation. Owner gets all permissions,
Viewer gets read-only access."
```

---

## Task 4: Expand Manager Interface and Rename NoopManager

**Files:**
- Modify: `backend/internal/ocserv/client.go`
- Modify: `backend/internal/ocserv/client_test.go`

- [ ] **Step 1: Expand Manager interface and add new types**

In `client.go`, add new types and expand the Manager interface:

```go
// User represents an ocserv user as reported by occtl.
type User struct {
	Username string
	Group    string
	Active   bool
}

// SessionDetail represents detailed information about a single ocserv session.
type SessionDetail struct {
	ID            string
	Username      string
	RemoteAddress string
	ConnectedAt   time.Time
	Duration      time.Duration
	BytesIn       int64
	BytesOut      int64
	Device        string
	IPv4          string
	IPv6          string
}

// Manager is the boundary for ocserv management operations.
type Manager interface {
	ListSessions(ctx context.Context) ([]Session, error)
	DisconnectSession(ctx context.Context, sessionID string) error
	ListUsers(ctx context.Context) ([]User, error)
	GetSessionDetails(ctx context.Context, id string) (*SessionDetail, error)
}
```

- [ ] **Step 2: Rename NoopManager to FakeManager and implement new methods**

```go
// FakeManager is a test double that returns configurable results.
type FakeManager struct {
	Sessions        []Session
	SessionDetails  map[string]*SessionDetail
	Users           []User
	DisconnectErr   error
	ListUsersErr    error
}

// NewFakeManager returns a FakeManager with empty default data.
func NewFakeManager() *FakeManager {
	return &FakeManager{
		SessionDetails: make(map[string]*SessionDetail),
	}
}

func (f *FakeManager) ListSessions(ctx context.Context) ([]Session, error) {
	return f.Sessions, nil
}

func (f *FakeManager) DisconnectSession(ctx context.Context, sessionID string) error {
	return f.DisconnectErr
}

func (f *FakeManager) ListUsers(ctx context.Context) ([]User, error) {
	return f.Users, f.ListUsersErr
}

func (f *FakeManager) GetSessionDetails(ctx context.Context, id string) (*SessionDetail, error) {
	if detail, ok := f.SessionDetails[id]; ok {
		return detail, nil
	}
	return nil, ErrNotConfigured
}
```

- [ ] **Step 3: Update client_test.go for FakeManager**

Rename `NewNoopManager()` to `NewFakeManager()` and update test assertions. Add tests for new methods:

```go
func TestFakeManagerListsNoSessions(t *testing.T) {
	t.Parallel()
	manager := NewFakeManager()
	sessions, err := manager.ListSessions(context.Background())
	if err != nil {
		t.Fatalf("list sessions: %v", err)
	}
	if len(sessions) != 0 {
		t.Fatalf("expected no sessions, got %d", len(sessions))
	}
}

func TestFakeManagerDisconnectReturnsConfiguredError(t *testing.T) {
	t.Parallel()
	manager := NewFakeManager()
	err := manager.DisconnectSession(context.Background(), "session-1")
	if !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("expected ErrNotConfigured by default, got %v", err)
	}
}

func TestFakeManagerListsNoUsers(t *testing.T) {
	t.Parallel()
	manager := NewFakeManager()
	users, err := manager.ListUsers(context.Background())
	if err != nil {
		t.Fatalf("list users: %v", err)
	}
	if len(users) != 0 {
		t.Fatalf("expected no users, got %d", len(users))
	}
}

func TestFakeManagerReturnsConfiguredSessions(t *testing.T) {
	t.Parallel()
	manager := NewFakeManager()
	manager.Sessions = []Session{
		{ID: "s1", Username: "alice", RemoteAddress: "10.0.0.1"},
	}
	sessions, err := manager.ListSessions(context.Background())
	if err != nil {
		t.Fatalf("list sessions: %v", err)
	}
	if len(sessions) != 1 || sessions[0].Username != "alice" {
		t.Fatalf("expected alice session, got %v", sessions)
	}
}
```

- [ ] **Step 4: Run tests**

Run: `cd /opt/git/ocserv_panel/backend && go test ./internal/ocserv/... -v -count=1`
Expected: All tests pass.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/ocserv/client.go backend/internal/ocserv/client_test.go
git commit -m "feat(ocserv): expand Manager interface with ListUsers and GetSessionDetails

Add User and SessionDetail types. Rename NoopManager to FakeManager with
configurable fields for testing. Implement all four Manager interface methods."
```

---

## Task 5: ocserv Readiness Checker

**Files:**
- Create: `backend/internal/ocserv/readiness.go`
- Create: `backend/internal/ocserv/readiness_test.go`

- [ ] **Step 1: Write the failing test for readiness checker**

Create `backend/internal/ocserv/readiness_test.go`:

```go
package ocserv

import (
	"context"
	"errors"
	"testing"
)

func TestReadinessCheckerAllPresent(t *testing.T) {
	t.Parallel()
	runner := NewFakeCommandRunner()
	runner.Results["ocserv"] = CommandResult{ExitCode: 0, Stdout: "ocserv 1.4.2\n"}
	runner.Results["ocserv -f -t"] = CommandResult{ExitCode: 0, Stderr: ""}

	checker := NewReadinessChecker(ReadinessConfig{
		OcservBinPath:   "/usr/local/sbin/ocserv",
		OcctlBinPath:    "/usr/local/bin/occtl",
		OcpasswdBinPath: "/usr/local/bin/ocpasswd",
		ConfigPath:      "/etc/ocserv/ocserv.conf",
		SocketPath:      "/var/run/ocserv-socket",
		UnitName:        "ocserv",
	}, runner, NewFakeServiceManager(ServiceActive))

	// Make all files "exist" by setting up the checker with a filesystem that has them
	result, err := checker.Check(context.Background())
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	// With fake runner, binary checks should succeed
	if !result.OcctlPresent {
		t.Error("expected OcctlPresent to be true")
	}
	if !result.OcpasswdPresent {
		t.Error("expected OcpasswdPresent to be true")
	}
}

func TestReadinessCheckerNotConfiguredWhenBinariesMissing(t *testing.T) {
	t.Parallest()
	runner := NewFakeCommandRunner()
	runner.Err = errors.New("binary not found")

	checker := NewReadinessChecker(ReadinessConfig{
		OcservBinPath:   "/nonexistent/ocserv",
		OcctlBinPath:    "/nonexistent/occtl",
		OcpasswdBinPath: "/nonexistent/ocpasswd",
		ConfigPath:      "/nonexistent/ocserv.conf",
		SocketPath:      "/nonexistent/socket",
		UnitName:        "ocserv",
	}, runner, NewFakeServiceManager(ServiceInactive))

	result, err := checker.Check(context.Background())
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if result.BinaryPresent {
		t.Error("expected BinaryPresent to be false when binary missing")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /opt/git/ocserv_panel/backend && go test ./internal/ocserv/... -run TestReadiness -v -count=1`
Expected: Compilation fails (NewReadinessChecker doesn't exist yet).

- [ ] **Step 3: Write the readiness checker implementation**

Create `backend/internal/ocserv/readiness.go`:

```go
package ocserv

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"
)

// OcservReadiness describes the installation and runtime state of ocserv.
type OcservReadiness struct {
	BinaryPresent   bool   `json:"binary_present"`
	BinaryVersion   string `json:"binary_version"`
	ConfigPresent   bool   `json:"config_present"`
	ConfigValid     bool   `json:"config_valid"`
	ConfigErrors    string `json:"config_errors,omitempty"`
	ServiceRunning  bool   `json:"service_running"`
	SocketPresent   bool   `json:"socket_present"`
	OcctlPresent    bool   `json:"occtl_present"`
	OcpasswdPresent bool   `json:"ocpasswd_present"`
}

// ReadinessConfig holds paths needed for readiness checks.
type ReadinessConfig struct {
	OcservBinPath   string
	OcctlBinPath    string
	OcpasswdBinPath string
	ConfigPath      string
	SocketPath      string
	UnitName        string
}

// ReadinessChecker probes the system for ocserv installation state.
type ReadinessChecker struct {
	cfg    ReadinessConfig
	runner CommandRunner
	svc    ServiceManager
}

// NewReadinessChecker returns a readiness checker with the given configuration.
func NewReadinessChecker(cfg ReadinessConfig, runner CommandRunner, svc ServiceManager) *ReadinessChecker {
	return &ReadinessChecker{cfg: cfg, runner: runner, svc: svc}
}

// Check probes the system and returns the current ocserv readiness state.
func (c *ReadinessChecker) Check(ctx context.Context) (*OcservReadiness, error) {
	r := &OcservReadiness{}

	// Check binary existence
	r.BinaryPresent = fileExists(c.cfg.OcservBinPath)
	r.OcctlPresent = fileExists(c.cfg.OcctlBinPath)
	r.OcpasswdPresent = fileExists(c.cfg.OcpasswdBinPath)

	// Check binary version
	if r.BinaryPresent {
		r.BinaryVersion = c.checkVersion(ctx)
	}

	// Check config file
	r.ConfigPresent = fileExists(c.cfg.ConfigPath)

	// Validate config if present
	if r.ConfigPresent && r.BinaryPresent {
		r.ConfigValid, r.ConfigErrors = c.validateConfig(ctx)
	}

	// Check service status
	status, err := c.svc.Status(ctx)
	if err == nil {
		r.ServiceRunning = status == ServiceActive
	}

	// Check socket
	r.SocketPresent = fileExists(c.cfg.SocketPath)

	return r, nil
}

func (c *ReadinessChecker) checkVersion(ctx context.Context) string {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	result, err := c.runner.Run(ctx, c.cfg.OcservBinPath, []string{"--version"}, nil)
	if err != nil {
		return ""
	}
	// Parse version from output like "ocserv 1.4.2"
	lines := strings.Split(strings.TrimSpace(result.Stdout), "\n")
	if len(lines) > 0 {
		return strings.TrimSpace(lines[0])
	}
	return ""
}

func (c *ReadinessChecker) validateConfig(ctx context.Context) (bool, string) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	result, err := c.runner.Run(ctx, c.cfg.OcservBinPath, []string{"-f", "-t"}, nil)
	if err != nil {
		return false, fmt.Sprintf("config validation failed: %v", err)
	}
	if result.ExitCode != 0 {
		return false, strings.TrimSpace(result.Stderr)
	}
	return true, ""
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /opt/git/ocserv_panel/backend && go test ./internal/ocserv/... -v -count=1`
Expected: All tests pass.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/ocserv/readiness.go backend/internal/ocserv/readiness_test.go
git commit -m "feat(ocserv): add readiness checker for ocserv installation detection

OcservReadinessChecker probes binary existence, version, config validity,
service status, and socket presence. Uses CommandRunner for testability."
```

---

## Task 6: Systemd Service Manager

**Files:**
- Create: `backend/internal/ocserv/service.go`
- Create: `backend/internal/ocserv/service_systemd.go`
- Create: `backend/internal/ocserv/service_fake.go`
- Create: `backend/internal/ocserv/service_test.go`

- [ ] **Step 1: Write the ServiceManager interface and types**

Create `backend/internal/ocserv/service.go`:

```go
package ocserv

// ServiceStatus represents the status of a systemd service.
type ServiceStatus int

const (
	ServiceActive   ServiceStatus = iota // service is running
	ServiceInactive                       // service is stopped
	ServiceFailed                         // service has failed
	ServiceNotFound                       // service unit does not exist
)

// ServiceManager manages ocserv systemd service lifecycle.
type ServiceManager interface {
	Status(ctx context.Context) (ServiceStatus, error)
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	Restart(ctx context.Context) error
	IsEnabled(ctx context.Context) (bool, error)
}
```

- [ ] **Step 2: Write the FakeServiceManager**

Create `backend/internal/ocserv/service_fake.go`:

```go
package ocserv

import (
	"context"
	"sync"
)

// FakeServiceManager is a test double for ServiceManager.
type FakeServiceManager struct {
	mu      sync.Mutex
	status  ServiceStatus
	enabled bool
	err     error
	starts  int
	stops   int
	restarts int
}

// NewFakeServiceManager returns a FakeServiceManager with the given status.
func NewFakeServiceManager(status ServiceStatus) *FakeServiceManager {
	return &FakeServiceManager{status: status, enabled: true}
}

func (f *FakeServiceManager) Status(ctx context.Context) (ServiceStatus, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.status, f.err
}

func (f *FakeServiceManager) Start(ctx context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.starts++
	if f.err != nil {
		return f.err
	}
	f.status = ServiceActive
	return nil
}

func (f *FakeServiceManager) Stop(ctx context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.stops++
	if f.err != nil {
		return f.err
	}
	f.status = ServiceInactive
	return nil
}

func (f *FakeServiceManager) Restart(ctx context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.restarts++
	if f.err != nil {
		return f.err
	}
	f.status = ServiceActive
	return nil
}

func (f *FakeServiceManager) IsEnabled(ctx context.Context) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.enabled, f.err
}

// SetError configures the error returned by all operations.
func (f *FakeServiceManager) SetError(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.err = err
}

// SetStatus configures the status returned by Status.
func (f *FakeServiceManager) SetStatus(status ServiceStatus) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.status = status
}

// StartCount returns the number of Start calls.
func (f *FakeServiceManager) StartCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.starts
}
```

- [ ] **Step 3: Write the SystemdManager implementation**

Create `backend/internal/ocserv/service_systemd.go`:

```go
package ocserv

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// SystemdManager manages ocserv via systemctl commands.
type SystemdManager struct {
	unitName string
	runner   CommandRunner
}

// NewSystemdManager returns a SystemdManager for the given unit name.
func NewSystemdManager(unitName string, runner CommandRunner) *SystemdManager {
	return &SystemdManager{unitName: unitName, runner: runner}
}

func (m *SystemdManager) Status(ctx context.Context) (ServiceStatus, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	result, err := m.runner.Run(ctx, "systemctl", []string{"status", m.unitName}, nil)
	if err != nil {
		return ServiceNotFound, nil
	}
	if strings.Contains(result.Stdout, "active (running)") {
		return ServiceActive, nil
	}
	if strings.Contains(result.Stdout, "failed") {
		return ServiceFailed, nil
	}
	if strings.Contains(result.Stdout, "inactive") {
		return ServiceInactive, nil
	}
	return ServiceNotFound, nil
}

func (m *SystemdManager) Start(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	result, err := m.runner.Run(ctx, "systemctl", []string{"start", m.unitName}, nil)
	if err != nil {
		return fmt.Errorf("start service %s: %w", m.unitName, err)
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("start service %s: %s", m.unitName, strings.TrimSpace(result.Stderr))
	}
	return nil
}

func (m *SystemdManager) Stop(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	result, err := m.runner.Run(ctx, "systemctl", []string{"stop", m.unitName}, nil)
	if err != nil {
		return fmt.Errorf("stop service %s: %w", m.unitName, err)
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("stop service %s: %s", m.unitName, strings.TrimSpace(result.Stderr))
	}
	return nil
}

func (m *SystemdManager) Restart(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	result, err := m.runner.Run(ctx, "systemctl", []string{"restart", m.unitName}, nil)
	if err != nil {
		return fmt.Errorf("restart service %s: %w", m.unitName, err)
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("restart service %s: %s", m.unitName, strings.TrimSpace(result.Stderr))
	}
	return nil
}

func (m *SystemdManager) IsEnabled(ctx context.Context) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	result, err := m.runner.Run(ctx, "systemctl", []string{"is-enabled", m.unitName}, nil)
	if err != nil {
		return false, nil
	}
	return strings.TrimSpace(result.Stdout) == "enabled", nil
}
```

- [ ] **Step 4: Write tests for service manager**

Create `backend/internal/ocserv/service_test.go`:

```go
package ocserv

import (
	"context"
	"errors"
	"testing"
)

func TestFakeServiceManagerStatus(t *testing.T) {
	t.Parallel()
	svc := NewFakeServiceManager(ServiceActive)
	status, err := svc.Status(context.Background())
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if status != ServiceActive {
		t.Fatalf("expected ServiceActive, got %d", status)
	}
}

func TestFakeServiceManagerStart(t *testing.T) {
	t.Parallel()
	svc := NewFakeServiceManager(ServiceInactive)
	if err := svc.Start(context.Background()); err != nil {
		t.Fatalf("start: %v", err)
	}
	status, _ := svc.Status(context.Background())
	if status != ServiceActive {
		t.Fatalf("expected ServiceActive after start, got %d", status)
	}
}

func TestFakeServiceManagerStop(t *testing.T) {
	t.Parallel()
	svc := NewFakeServiceManager(ServiceActive)
	if err := svc.Stop(context.Background()); err != nil {
		t.Fatalf("stop: %v", err)
	}
	status, _ := svc.Status(context.Background())
	if status != ServiceInactive {
		t.Fatalf("expected ServiceInactive after stop, got %d", status)
	}
}

func TestFakeServiceManagerRestart(t *testing.T) {
	t.Parallel()
	svc := NewFakeServiceManager(ServiceInactive)
	if err := svc.Restart(context.Background()); err != nil {
		t.Fatalf("restart: %v", err)
	}
	status, _ := svc.Status(context.Background())
	if status != ServiceActive {
		t.Fatalf("expected ServiceActive after restart, got %d", status)
	}
}

func TestFakeServiceManagerIsEnabled(t *testing.T) {
	t.Parallel()
	svc := NewFakeServiceManager(ServiceActive)
	enabled, err := svc.IsEnabled(context.Background())
	if err != nil {
		t.Fatalf("is-enabled: %v", err)
	}
	if !enabled {
		t.Fatal("expected enabled to be true")
	}
}

func TestFakeServiceManagerError(t *testing.T) {
	t.Parallel()
	svc := NewFakeServiceManager(ServiceActive)
	svc.SetError(errors.New("systemctl failed"))
	_, err := svc.Status(context.Background())
	if err == nil {
		t.Fatal("expected error from status")
	}
}

func TestSystemdManagerStatusActive(t *testing.T) {
	t.Parallel()
	runner := NewFakeCommandRunner()
	runner.Results["systemctl"] = CommandResult{
		ExitCode: 0,
		Stdout:   "● ocserv.service - ocserv\n   Active: active (running) since Mon 2026-01-01 00:00:00 UTC\n",
	}
	mgr := NewSystemdManager("ocserv", runner)
	status, err := mgr.Status(context.Background())
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if status != ServiceActive {
		t.Fatalf("expected ServiceActive, got %d", status)
	}
}

func TestSystemdManagerStatusInactive(t *testing.T) {
	t.Parallel()
	runner := NewFakeCommandRunner()
	runner.Results["systemctl"] = CommandResult{
		ExitCode: 3,
		Stdout:   "● ocserv.service - ocserv\n   Active: inactive (dead)\n",
	}
	mgr := NewSystemdManager("ocserv", runner)
	status, err := mgr.Status(context.Background())
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if status != ServiceInactive {
		t.Fatalf("expected ServiceInactive, got %d", status)
	}
}

func TestSystemdManagerStart(t *testing.T) {
	t.Parallel()
	runner := NewFakeCommandRunner()
	runner.Results["systemctl"] = CommandResult{ExitCode: 0}
	mgr := NewSystemdManager("ocserv", runner)
	if err := mgr.Start(context.Background()); err != nil {
		t.Fatalf("start: %v", err)
	}
}

func TestSystemdManagerStartFails(t *testing.T) {
	t.Parallel()
	runner := NewFakeCommandRunner()
	runner.Results["systemctl"] = CommandResult{ExitCode: 1, Stderr: "Job failed."}
	mgr := NewSystemdManager("ocserv", runner)
	if err := mgr.Start(context.Background()); err == nil {
		t.Fatal("expected start to fail")
	}
}
```

- [ ] **Step 5: Run tests**

Run: `cd /opt/git/ocserv_panel/backend && go test ./internal/ocserv/... -v -count=1`
Expected: All tests pass.

- [ ] **Step 6: Commit**

```bash
git add backend/internal/ocserv/service.go backend/internal/ocserv/service_systemd.go backend/internal/ocserv/service_fake.go backend/internal/ocserv/service_test.go
git commit -m "feat(ocserv): add ServiceManager interface with SystemdManager and FakeManager

SystemdManager uses systemctl commands for start/stop/restart/status/is-enabled.
FakeServiceManager provides in-memory test double. Timeouts: 30s start/restart,
10s stop, 5s status."
```

---

## Task 7: OcctlAdapter for Session Management

**Files:**
- Create: `backend/internal/ocserv/occtl_adapter.go`
- Create: `backend/internal/ocserv/occtl_adapter_test.go`

- [ ] **Step 1: Write the OcctlAdapter**

Create `backend/internal/ocserv/occtl_adapter.go`:

```go
package ocserv

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// OcctlAdapter implements Manager using the occtl command-line tool.
type OcctlAdapter struct {
	occtlPath string
	socketPath string
	runner    CommandRunner
}

// NewOcctlAdapter returns an adapter that calls occtl for session management.
func NewOcctlAdapter(occtlPath, socketPath string, runner CommandRunner) *OcctlAdapter {
	return &OcctlAdapter{
		occtlPath:  occtlPath,
		socketPath: socketPath,
		runner:     runner,
	}
}

func (a *OcctlAdapter) ListSessions(ctx context.Context) ([]Session, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	args := []string{"show", "sessions", "--output-type", "json"}
	if a.socketPath != "" {
		args = append([]string{"--socket-file", a.socketPath}, args...)
	}

	result, err := a.runner.Run(ctx, a.occtlPath, args, nil)
	if err != nil {
		return nil, fmt.Errorf("list sessions: %w", err)
	}
	if result.ExitCode != 0 {
		if strings.Contains(result.Stderr, "no such file") || strings.Contains(result.Stderr, "not found") {
			return nil, ErrNotConfigured
		}
		return nil, fmt.Errorf("list sessions: %s", strings.TrimSpace(result.Stderr))
	}

	var sessions []Session
	if err := json.Unmarshal([]byte(result.Stdout), &sessions); err != nil {
		// occtl may return non-JSON when no sessions exist
		if strings.TrimSpace(result.Stdout) == "" || strings.Contains(result.Stdout, "No sessions") {
			return []Session{}, nil
		}
		return nil, fmt.Errorf("parse sessions output: %w", err)
	}
	return sessions, nil
}

func (a *OcctlAdapter) DisconnectSession(ctx context.Context, sessionID string) error {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	args := []string{"disconnect", "id", sessionID}
	if a.socketPath != "" {
		args = append([]string{"--socket-file", a.socketPath}, args...)
	}

	result, err := a.runner.Run(ctx, a.occtlPath, args, nil)
	if err != nil {
		return fmt.Errorf("disconnect session %s: %w", sessionID, err)
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("disconnect session %s: %s", sessionID, strings.TrimSpace(result.Stderr))
	}
	return nil
}

func (a *OcctlAdapter) ListUsers(ctx context.Context) ([]User, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	args := []string{"show", "users", "--output-type", "json"}
	if a.socketPath != "" {
		args = append([]string{"--socket-file", a.socketPath}, args...)
	}

	result, err := a.runner.Run(ctx, a.occtlPath, args, nil)
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	if result.ExitCode != 0 {
		if strings.Contains(result.Stderr, "no such file") || strings.Contains(result.Stderr, "not found") {
			return nil, ErrNotConfigured
		}
		return nil, fmt.Errorf("list users: %s", strings.TrimSpace(result.Stderr))
	}

	var users []User
	if err := json.Unmarshal([]byte(result.Stdout), &users); err != nil {
		if strings.TrimSpace(result.Stdout) == "" || strings.Contains(result.Stdout, "No users") {
			return []User{}, nil
		}
		return nil, fmt.Errorf("parse users output: %w", err)
	}
	return users, nil
}

func (a *OcctlAdapter) GetSessionDetails(ctx context.Context, id string) (*SessionDetail, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	args := []string{"show", "session", id, "--output-type", "json"}
	if a.socketPath != "" {
		args = append([]string{"--socket-file", a.socketPath}, args...)
	}

	result, err := a.runner.Run(ctx, a.occtlPath, args, nil)
	if err != nil {
		return nil, fmt.Errorf("get session details %s: %w", id, err)
	}
	if result.ExitCode != 0 {
		return nil, fmt.Errorf("get session details %s: %s", id, strings.TrimSpace(result.Stderr))
	}

	var detail SessionDetail
	if err := json.Unmarshal([]byte(result.Stdout), &detail); err != nil {
		return nil, fmt.Errorf("parse session detail: %w", err)
	}
	return &detail, nil
}
```

- [ ] **Step 2: Write tests for OcctlAdapter**

Create `backend/internal/ocserv/occtl_adapter_test.go`:

```go
package ocserv

import (
	"context"
	"errors"
	"testing"
)

func TestOcctlAdapterListSessionsEmpty(t *testing.T) {
	t.Parallel()
	runner := NewFakeCommandRunner()
	runner.Results["occtl"] = CommandResult{ExitCode: 0, Stdout: "[]"}
	adapter := NewOcctlAdapter("/usr/local/bin/occtl", "/var/run/ocserv-socket", runner)

	sessions, err := adapter.ListSessions(context.Background())
	if err != nil {
		t.Fatalf("list sessions: %v", err)
	}
	if len(sessions) != 0 {
		t.Fatalf("expected 0 sessions, got %d", len(sessions))
	}
}

func TestOcctlAdapterListSessionsWithData(t *testing.T) {
	t.Parallel()
	runner := NewFakeCommandRunner()
	runner.Results["occtl"] = CommandResult{
		ExitCode: 0,
		Stdout:   `[{"ID":"s1","Username":"alice","RemoteAddress":"10.0.0.1","ConnectedAt":"2026-01-01T00:00:00Z"}]`,
	}
	adapter := NewOcctlAdapter("/usr/local/bin/occtl", "/var/run/ocserv-socket", runner)

	sessions, err := adapter.ListSessions(context.Background())
	if err != nil {
		t.Fatalf("list sessions: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}
	if sessions[0].Username != "alice" {
		t.Fatalf("expected alice, got %s", sessions[0].Username)
	}
}

func TestOcctlAdapterListSessionsNotConfigured(t *testing.T) {
	t.Parallel()
	runner := NewFakeCommandRunner()
	runner.Results["occtl"] = CommandResult{
		ExitCode: 1,
		Stderr:   "occtl: connect: no such file or directory",
	}
	adapter := NewOcctlAdapter("/usr/local/bin/occtl", "/var/run/ocserv-socket", runner)

	_, err := adapter.ListSessions(context.Background())
	if !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("expected ErrNotConfigured, got %v", err)
	}
}

func TestOcctlAdapterDisconnectSession(t *testing.T) {
	t.Parallel()
	runner := NewFakeCommandRunner()
	runner.Results["occtl"] = CommandResult{ExitCode: 0}
	adapter := NewOcctlAdapter("/usr/local/bin/occtl", "/var/run/ocserv-socket", runner)

	if err := adapter.DisconnectSession(context.Background(), "s1"); err != nil {
		t.Fatalf("disconnect: %v", err)
	}
}

func TestOcctlAdapterDisconnectSessionFails(t *testing.T) {
	t.Parallel()
	runner := NewFakeCommandRunner()
	runner.Results["occtl"] = CommandResult{ExitCode: 1, Stderr: "session not found"}
	adapter := NewOcctlAdapter("/usr/local/bin/occtl", "/var/run/ocserv-socket", runner)

	if err := adapter.DisconnectSession(context.Background(), "s1"); err == nil {
		t.Fatal("expected disconnect to fail")
	}
}

func TestOcctlAdapterListUsers(t *testing.T) {
	t.Parallel()
	runner := NewFakeCommandRunner()
	runner.Results["occtl"] = CommandResult{ExitCode: 0, Stdout: "[]"}
	adapter := NewOcctlAdapter("/usr/local/bin/occtl", "/var/run/ocserv-socket", runner)

	users, err := adapter.ListUsers(context.Background())
	if err != nil {
		t.Fatalf("list users: %v", err)
	}
	if len(users) != 0 {
		t.Fatalf("expected 0 users, got %d", len(users))
	}
}
```

- [ ] **Step 3: Run tests**

Run: `cd /opt/git/ocserv_panel/backend && go test ./internal/ocserv/... -v -count=1`
Expected: All tests pass.

- [ ] **Step 4: Commit**

```bash
git add backend/internal/ocserv/occtl_adapter.go backend/internal/ocserv/occtl_adapter_test.go
git commit -m "feat(ocserv): add OcctlAdapter for session and user management

Implements Manager interface using occtl command. Supports ListSessions,
DisconnectSession, ListUsers, and GetSessionDetails. Uses CommandRunner
for testable command execution."
```

---

## Task 8: OcpasswdAdapter for User Management

**Files:**
- Create: `backend/internal/ocserv/passwd.go`
- Create: `backend/internal/ocserv/passwd_adapter.go`
- Create: `backend/internal/ocserv/passwd_adapter_test.go`
- Create: `backend/internal/ocserv/passwd_fake.go`

- [ ] **Step 1: Write the PasswdManager interface and types**

Create `backend/internal/ocserv/passwd.go`:

```go
package ocserv

import (
	"context"
)

// PasswdUser represents a user entry in the ocserv password file.
type PasswdUser struct {
	Username string `json:"username"`
	Group    string `json:"group"`
	Active   bool   `json:"active"`
}

// PasswdManager manages ocserv password-based users.
type PasswdManager interface {
	CreateUser(ctx context.Context, username, password string) error
	DeleteUser(ctx context.Context, username string) error
	UpdatePassword(ctx context.Context, username, newPassword string) error
	ListUsers(ctx context.Context) ([]PasswdUser, error)
	UserExists(ctx context.Context, username string) (bool, error)
}
```

- [ ] **Step 2: Write the OcpasswdAdapter**

Create `backend/internal/ocserv/passwd_adapter.go`:

```go
package ocserv

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// OcpasswdAdapter implements PasswdManager using the ocpasswd command and
// direct password file parsing.
type OcpasswdAdapter struct {
	binPath    string
	filePath   string
	configPath string
	runner     CommandRunner
}

// NewOcpasswdAdapter returns an adapter for ocpasswd user management.
func NewOcpasswdAdapter(binPath, filePath, configPath string, runner CommandRunner) *OcpasswdAdapter {
	return &OcpasswdAdapter{
		binPath:    binPath,
		filePath:   filePath,
		configPath: configPath,
		runner:     runner,
	}
}

func (a *OcpasswdAdapter) CreateUser(ctx context.Context, username, password string) error {
	return a.runPasswdCommand(ctx, username, password)
}

func (a *OcpasswdAdapter) DeleteUser(ctx context.Context, username string) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	args := []string{"-c", a.configPath, "-d", username}
	result, err := a.runner.Run(ctx, a.binPath, args, nil)
	if err != nil {
		return fmt.Errorf("delete user %s: %w", username, err)
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("delete user %s: %s", username, strings.TrimSpace(result.Stderr))
	}
	return nil
}

func (a *OcpasswdAdapter) UpdatePassword(ctx context.Context, username, newPassword string) error {
	return a.runPasswdCommand(ctx, username, newPassword)
}

func (a *OcpasswdAdapter) ListUsers(ctx context.Context) ([]PasswdUser, error) {
	data, err := os.ReadFile(a.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return []PasswdUser{}, nil
		}
		return nil, fmt.Errorf("read password file %s: %w", a.filePath, err)
	}
	return parsePasswdFile(data), nil
}

func (a *OcpasswdAdapter) UserExists(ctx context.Context, username string) (bool, error) {
	users, err := a.ListUsers(ctx)
	if err != nil {
		return false, err
	}
	for _, u := range users {
		if u.Username == username {
			return true, nil
		}
	}
	return false, nil
}

func (a *OcpasswdAdapter) runPasswdCommand(ctx context.Context, username, password string) error {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	args := []string{"-c", a.configPath, username}
	stdin := strings.NewReader(password + "\n")
	result, err := a.runner.Run(ctx, a.binPath, args, stdin)
	if err != nil {
		return fmt.Errorf("ocpasswd for user %s: %w", username, err)
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("ocpasswd for user %s: %s", username, strings.TrimSpace(result.Stderr))
	}
	return nil
}

// parsePasswdFile parses ocpasswd file format: username:group:hashed_password
func parsePasswdFile(data []byte) []PasswdUser {
	var users []PasswdUser
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, ":", 3)
		if len(parts) < 2 {
			continue
		}
		username := parts[0]
		group := parts[1]
		active := len(parts) < 3 || parts[2] != "!" // "!" prefix means disabled
		users = append(users, PasswdUser{Username: username, Group: group, Active: active})
	}
	return users
}
```

Note: The `os` import needs to be added. Also, `parsePasswdFile` should be exported for testing.

- [ ] **Step 3: Write the FakePasswdManager**

Create `backend/internal/ocserv/passwd_fake.go`:

```go
package ocserv

import (
	"context"
	"sync"
)

// FakePasswdManager is a test double for PasswdManager.
type FakePasswdManager struct {
	mu    sync.Mutex
	users map[string]PasswdUser
	err   error
}

// NewFakePasswdManager returns a FakePasswdManager with empty user store.
func NewFakePasswdManager() *FakePasswdManager {
	return &FakePasswdManager{users: make(map[string]PasswdUser)}
}

func (f *FakePasswdManager) CreateUser(ctx context.Context, username, password string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return f.err
	}
	f.users[username] = PasswdUser{Username: username, Group: "default", Active: true}
	return nil
}

func (f *FakePasswdManager) DeleteUser(ctx context.Context, username string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return f.err
	}
	delete(f.users, username)
	return nil
}

func (f *FakePasswdManager) UpdatePassword(ctx context.Context, username, newPassword string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return f.err
	}
	if u, ok := f.users[username]; ok {
		f.users[username] = u
	}
	return nil
}

func (f *FakePasswdManager) ListUsers(ctx context.Context) ([]PasswdUser, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return nil, f.err
	}
	var result []PasswdUser
	for _, u := range f.users {
		result = append(result, u)
	}
	return result, nil
}

func (f *FakePasswdManager) UserExists(ctx context.Context, username string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return false, f.err
	}
	_, ok := f.users[username]
	return ok, nil
}

// SetError configures the error returned by all operations.
func (f *FakePasswdManager) SetError(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.err = err
}
```

- [ ] **Step 4: Write tests for OcpasswdAdapter**

Create `backend/internal/ocserv/passwd_adapter_test.go`:

```go
package ocserv

import (
	"context"
	"testing"
)

func TestParsePasswdFileEmpty(t *testing.T) {
	t.Parallel()
	users := ParsePasswdFile([]byte(""))
	if len(users) != 0 {
		t.Fatalf("expected 0 users, got %d", len(users))
	}
}

func TestParsePasswdFileWithUsers(t *testing.T) {
	t.Parallel()
	data := []byte("alice:default:*hashed*\nbob:default:*hashed2*\n")
	users := ParsePasswdFile(data)
	if len(users) != 2 {
		t.Fatalf("expected 2 users, got %d", len(users))
	}
	if users[0].Username != "alice" {
		t.Fatalf("expected alice, got %s", users[0].Username)
	}
	if !users[0].Active {
		t.Fatal("expected alice to be active")
	}
}

func TestParsePasswdFileWithDisabledUser(t *testing.T) {
	t.Parallel()
	data := []byte("alice:default:!\n")
	users := ParsePasswdFile(data)
	if len(users) != 1 {
		t.Fatalf("expected 1 user, got %d", len(users))
	}
	if users[0].Active {
		t.Fatal("expected disabled user to be inactive")
	}
}

func TestParsePasswdFileSkipsComments(t *testing.T) {
	t.Parallel()
	data := []byte("# comment\nalice:default:*hashed*\n")
	users := ParsePasswdFile(data)
	if len(users) != 1 {
		t.Fatalf("expected 1 user, got %d", len(users))
	}
}

func TestFakePasswdManagerCreateAndList(t *testing.T) {
	t.Parallel()
	mgr := NewFakePasswdManager()
	if err := mgr.CreateUser(context.Background(), "alice", "password123"); err != nil {
		t.Fatalf("create user: %v", err)
	}
	users, err := mgr.ListUsers(context.Background())
	if err != nil {
		t.Fatalf("list users: %v", err)
	}
	if len(users) != 1 || users[0].Username != "alice" {
		t.Fatalf("expected alice, got %v", users)
	}
}

func TestFakePasswdManagerDelete(t *testing.T) {
	t.Parallel()
	mgr := NewFakePasswdManager()
	mgr.CreateUser(context.Background(), "alice", "password123")
	if err := mgr.DeleteUser(context.Background(), "alice"); err != nil {
		t.Fatalf("delete user: %v", err)
	}
	exists, _ := mgr.UserExists(context.Background(), "alice")
	if exists {
		t.Fatal("expected alice to be deleted")
	}
}

func TestFakePasswdManagerUserExists(t *testing.T) {
	t.Parallel()
	mgr := NewFakePasswdManager()
	mgr.CreateUser(context.Background(), "alice", "password123")
	exists, err := mgr.UserExists(context.Background(), "alice")
	if err != nil {
		t.Fatalf("user exists: %v", err)
	}
	if !exists {
		t.Fatal("expected alice to exist")
	}
	exists, _ = mgr.UserExists(context.Background(), "nonexistent")
	if exists {
		t.Fatal("expected nonexistent user to not exist")
	}
}
```

- [ ] **Step 5: Run tests**

Run: `cd /opt/git/ocserv_panel/backend && go test ./internal/ocserv/... -v -count=1`
Expected: All tests pass.

- [ ] **Step 6: Commit**

```bash
git add backend/internal/ocserv/passwd.go backend/internal/ocserv/passwd_adapter.go backend/internal/ocserv/passwd_adapter_test.go backend/internal/ocserv/passwd_fake.go
git commit -m "feat(ocserv): add PasswdManager interface with OcpasswdAdapter and FakeManager

OcpasswdAdapter uses ocpasswd command for create/delete/update and direct
file parsing for list/exist. Password piped via stdin to avoid shell injection.
FakePasswdManager provides in-memory test double."
```

---

## Task 9: Config Validator

**Files:**
- Create: `backend/internal/ocserv/config_validator.go`
- Create: `backend/internal/ocserv/config_validator_test.go`

- [ ] **Step 1: Write the ConfigValidator**

Create `backend/internal/ocserv/config_validator.go`:

```go
package ocserv

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ConfigIssueSeverity rates the importance of a configuration issue.
type ConfigIssueSeverity string

const (
	IssueError   ConfigIssueSeverity = "error"
	IssueWarning ConfigIssueSeverity = "warning"
	IssueInfo    ConfigIssueSeverity = "info"
)

// ConfigIssue describes a single ocserv configuration problem.
type ConfigIssue struct {
	Severity   ConfigIssueSeverity `json:"severity"`
	Key        string               `json:"key"`
	Message    string               `json:"message"`
	Suggestion string               `json:"suggestion,omitempty"`
}

// ConfigValidator checks ocserv configuration for common production issues.
type ConfigValidator struct {
	configPath string
}

// NewConfigValidator returns a validator for the given ocserv config path.
func NewConfigValidator(configPath string) *ConfigValidator {
	return &ConfigValidator{configPath: configPath}
}

// ValidateConfig reads and checks the ocserv configuration file.
func (v *ConfigValidator) ValidateConfig(ctx context.Context) ([]ConfigIssue, error) {
	data, err := os.ReadFile(v.configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return []ConfigIssue{{
				Severity:   IssueError,
				Key:        "config_file",
				Message:    fmt.Sprintf("ocserv config file not found: %s", v.configPath),
				Suggestion: fmt.Sprintf("Create the config file at %s or update the installation.ocserv_config_path setting.", v.configPath),
			}}, nil
		}
		return nil, fmt.Errorf("read config %s: %w", v.configPath, err)
	}

	var issues []ConfigIssue
	lines := strings.Split(string(data), "\n")
	configDir := filepath.Dir(v.configPath)

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "#") || line == "" {
			continue
		}

		key, value, ok := parseConfigLine(line)
		if !ok {
			continue
		}

		switch key {
		case "server-cert", "server-key", "ca-cert":
			issues = append(issues, v.checkCertPath(key, value, configDir)...)
		case "auth":
			issues = append(issues, v.checkAuthDirective(key, value, configDir)...)
		case "socket-file":
			issues = append(issues, v.checkSocketPath(key, value)...)
		case "device":
			issues = append(issues, v.checkDevice(key, value)...)
		}
	}

	return issues, nil
}

func (v *ConfigValidator) checkCertPath(key, value, configDir string) []ConfigIssue {
	var issues []ConfigIssue

	// Check for relative paths
	if !filepath.IsAbs(value) {
		issues = append(issues, ConfigIssue{
			Severity:   IssueWarning,
			Key:        key,
			Message:    fmt.Sprintf("%s uses a relative path: %s", key, value),
			Suggestion: fmt.Sprintf("Use an absolute path like /etc/ocserv/certs/%s", filepath.Base(value)),
		})
	}

	// Resolve relative to config dir for existence check
	fullPath := value
	if !filepath.IsAbs(value) {
		fullPath = filepath.Join(configDir, value)
	}

	if _, err := os.Stat(fullPath); err != nil {
		issues = append(issues, ConfigIssue{
			Severity:   IssueError,
			Key:        key,
			Message:    fmt.Sprintf("%s file not found: %s (resolved: %s)", key, value, fullPath),
			Suggestion: fmt.Sprintf("Create the certificate file at %s or update the %s directive.", fullPath, key),
		})
	}

	return issues
}

func (v *ConfigValidator) checkAuthDirective(key, value, configDir string) []ConfigIssue {
	var issues []ConfigIssue

	// Check for plain[passwd=...] with relative or missing file
	if strings.HasPrefix(value, "plain[passwd=") {
		passwdFile := strings.TrimPrefix(value, "plain[passwd=")
		passwdFile = strings.TrimSuffix(passwdFile, "]")

		if !filepath.IsAbs(passwdFile) {
			issues = append(issues, ConfigIssue{
				Severity:   IssueWarning,
				Key:        key,
				Message:    fmt.Sprintf("auth password file uses a relative path: %s", passwdFile),
				Suggestion: fmt.Sprintf("Use an absolute path like /etc/ocserv/ocpasswd"),
			})
		}

		fullPath := passwdFile
		if !filepath.IsAbs(passwdFile) {
			fullPath = filepath.Join(configDir, passwdFile)
		}

		if _, err := os.Stat(fullPath); err != nil {
			issues = append(issues, ConfigIssue{
				Severity:   IssueInfo,
				Key:        key,
				Message:    fmt.Sprintf("auth password file not found: %s (resolved: %s)", passwdFile, fullPath),
				Suggestion: "The password file will be created when the first user is added.",
			})
		}
	}

	return issues
}

func (v *ConfigValidator) checkSocketPath(key, value string) []ConfigIssue {
	var issues []ConfigIssue
	if !filepath.IsAbs(value) {
		issues = append(issues, ConfigIssue{
			Severity:   IssueWarning,
			Key:        key,
			Message:    fmt.Sprintf("%s uses a relative path: %s", key, value),
			Suggestion: "Use an absolute path like /var/run/ocserv-socket",
		})
	}
	return issues
}

func (v *ConfigValidator) checkDevice(key, value string) []ConfigIssue {
	var issues []ConfigIssue
	// Check if the device exists (it won't when ocserv isn't running, so this is info-level)
	if _, err := os.Stat("/sys/class/net/" + value); err != nil {
		issues = append(issues, ConfigIssue{
			Severity:   IssueInfo,
			Key:        key,
			Message:    fmt.Sprintf("network device %s not found (expected when ocserv is not running)", value),
			Suggestion: fmt.Sprintf("The device %s will appear when ocserv starts.", value),
		})
	}
	return issues
}

func parseConfigLine(line string) (string, string, bool) {
	// Handle "key = value" format
	idx := strings.Index(line, "=")
	if idx < 0 {
		return "", "", false
	}
	key := strings.TrimSpace(line[:idx])
	value := strings.TrimSpace(line[idx+1:])
	// Remove quotes
	value = strings.Trim(value, "\"'")
	return key, value, true
}
```

- [ ] **Step 2: Write tests for ConfigValidator**

Create `backend/internal/ocserv/config_validator_test.go`:

```go
package ocserv

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestConfigValidatorMissingFile(t *testing.T) {
	t.Parallel()
	v := NewConfigValidator("/nonexistent/ocserv.conf")
	issues, err := v.ValidateConfig(context.Background())
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if len(issues) != 1 || issues[0].Severity != IssueError {
		t.Fatalf("expected 1 error issue for missing file, got %v", issues)
	}
}

func TestConfigValidatorRelativeCertPath(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	confPath := filepath.Join(dir, "ocserv.conf")
	os.WriteFile(confPath, []byte("server-cert = ../tests/certs/server-cert.pem\n"), 0600)

	v := NewConfigValidator(confPath)
	issues, err := v.ValidateConfig(context.Background())
	if err != nil {
		t.Fatalf("validate: %v", err)
	}

	hasWarning := false
	for _, issue := range issues {
		if issue.Severity == IssueWarning && issue.Key == "server-cert" {
			hasWarning = true
		}
	}
	if !hasWarning {
		t.Fatalf("expected warning for relative cert path, got %v", issues)
	}
}

func TestConfigValidatorAuthWithMissingPasswd(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	confPath := filepath.Join(dir, "ocserv.conf")
	os.WriteFile(confPath, []byte("auth = \"plain[passwd=./sample.passwd]\"\n"), 0600)

	v := NewConfigValidator(confPath)
	issues, err := v.ValidateConfig(context.Background())
	if err != nil {
		t.Fatalf("validate: %v", err)
	}

	hasInfo := false
	for _, issue := range issues {
		if issue.Severity == IssueInfo && issue.Key == "auth" {
			hasInfo = true
		}
	}
	if !hasInfo {
		t.Fatalf("expected info for missing passwd file, got %v", issues)
	}
}

func TestConfigValidatorValidAbsoluteCertPath(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	certFile := filepath.Join(dir, "server-cert.pem")
	os.WriteFile(certFile, []byte("cert"), 0600)
	confPath := filepath.Join(dir, "ocserv.conf")
	os.WriteFile(confPath, []byte("server-cert = "+certFile+"\n"), 0600)

	v := NewConfigValidator(confPath)
	issues, err := v.ValidateConfig(context.Background())
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	// Should have no error-level issues for existing absolute cert path
	for _, issue := range issues {
		if issue.Severity == IssueError && issue.Key == "server-cert" {
			t.Fatalf("unexpected error for valid cert path: %v", issue)
		}
	}
}

func TestParseConfigLine(t *testing.T) {
	t.Parallel()
	tests := []struct {
		line    string
		key     string
		value   string
		wantOK bool
	}{
		{"server-cert = /etc/ocserv/certs/server-cert.pem", "server-cert", "/etc/ocserv/certs/server-cert.pem", true},
		{"auth = \"plain[passwd=./sample.passwd]\"", "auth", "plain[passwd=./sample.passwd]", true},
		{"# comment", "", "", false},
		{"", "", "", false},
	}

	for _, tt := range tests {
		key, value, ok := parseConfigLine(tt.line)
		if ok != tt.wantOK || key != tt.key || value != tt.value {
			t.Errorf("parseConfigLine(%q) = (%q, %q, %v), want (%q, %q, %v)", tt.line, key, value, ok, tt.key, tt.value, tt.wantOK)
		}
	}
}
```

- [ ] **Step 3: Run tests**

Run: `cd /opt/git/ocserv_panel/backend && go test ./internal/ocserv/... -v -count=1`
Expected: All tests pass.

- [ ] **Step 4: Commit**

```bash
git add backend/internal/ocserv/config_validator.go backend/internal/ocserv/config_validator_test.go
git commit -m "feat(ocserv): add ConfigValidator for ocserv configuration issues

Checks relative cert paths, missing cert files, missing password files,
relative socket paths, and missing network devices. Returns structured
ConfigIssue list with severity, key, message, and suggestion."
```

---

## Task 10: OcservService and Dashboard Integration

**Files:**
- Create: `backend/internal/app/ocserv_service.go`
- Create: `backend/internal/app/ocserv_service_test.go`
- Modify: `backend/internal/app/dashboard_service.go`
- Modify: `backend/internal/server/server.go`

- [ ] **Step 1: Write the OcservService**

Create `backend/internal/app/ocserv_service.go`:

```go
package app

import (
	"context"
	"fmt"

	"ocserv_panel/backend/internal/audit"
	"ocserv_panel/backend/internal/auth"
	"ocserv_panel/backend/internal/ocserv"
	"ocserv_panel/backend/internal/rbac"
)

// OcservService orchestrates ocserv readiness, sessions, users, service, and config.
type OcservService struct {
	readiness *ocserv.ReadinessChecker
	sessions  ocserv.Manager
	users     ocserv.PasswdManager
	svc       ocserv.ServiceManager
	validator *ocserv.ConfigValidator
	recorder  audit.Recorder
	authorizer rbac.Authorizer
}

// NewOcservService returns an OcservService with the given dependencies.
func NewOcservService(
	readiness *ocserv.ReadinessChecker,
	sessions ocserv.Manager,
	users ocserv.PasswdManager,
	svc ocserv.ServiceManager,
	validator *ocserv.ConfigValidator,
	recorder audit.Recorder,
	authorizer rbac.Authorizer,
) *OcservService {
	if recorder == nil {
		recorder = audit.NoopRecorder{}
	}
	return &OcservService{
		readiness:  readiness,
		sessions:   sessions,
		users:      users,
		svc:        svc,
		validator:  validator,
		recorder:   recorder,
		authorizer: authorizer,
	}
}

// Readiness returns the current ocserv installation readiness state.
func (s *OcservService) Readiness(ctx context.Context, principal auth.Principal) (*ocserv.OcservReadiness, error) {
	if err := s.require(ctx, principal, rbac.PermissionOcservRead, "ocserv.readiness"); err != nil {
		return nil, err
	}
	result, err := s.readiness.Check(ctx)
	if err != nil {
		return nil, fmt.Errorf("check ocserv readiness: %w", err)
	}
	return result, nil
}

// ServiceStatus returns the ocserv service status.
func (s *OcservService) ServiceStatus(ctx context.Context, principal auth.Principal) (ocserv.ServiceStatus, error) {
	if err := s.require(ctx, principal, rbac.PermissionOcservServiceRead, "ocserv.service.status"); err != nil {
		return ocserv.ServiceNotFound, err
	}
	status, err := s.svc.Status(ctx)
	if err != nil {
		return ocserv.ServiceNotFound, fmt.Errorf("get service status: %w", err)
	}
	return status, nil
}

// StartService starts the ocserv service.
func (s *OcservService) StartService(ctx context.Context, principal auth.Principal) error {
	if err := s.require(ctx, principal, rbac.PermissionOcservServiceManage, "ocserv.service.start"); err != nil {
		return err
	}
	actorID := principal.ID
	if err := s.svc.Start(ctx); err != nil {
		_ = s.record(ctx, &actorID, "ocserv.service.start", "ocserv_service", "", audit.OutcomeFailure, err.Error(), nil)
		return fmt.Errorf("start ocserv service: %w", err)
	}
	_ = s.record(ctx, &actorID, "ocserv.service.start", "ocserv_service", "", audit.OutcomeSuccess, "ocserv service started", nil)
	return nil
}

// StopService stops the ocserv service.
func (s *OcservService) StopService(ctx context.Context, principal auth.Principal) error {
	if err := s.require(ctx, principal, rbac.PermissionOcservServiceManage, "ocserv.service.stop"); err != nil {
		return err
	}
	actorID := principal.ID
	if err := s.svc.Stop(ctx); err != nil {
		_ = s.record(ctx, &actorID, "ocserv.service.stop", "ocserv_service", "", audit.OutcomeFailure, err.Error(), nil)
		return fmt.Errorf("stop ocserv service: %w", err)
	}
	_ = s.record(ctx, &actorID, "ocserv.service.stop", "ocserv_service", "", audit.OutcomeSuccess, "ocserv service stopped", nil)
	return nil
}

// RestartService restarts the ocserv service.
func (s *OcservService) RestartService(ctx context.Context, principal auth.Principal) error {
	if err := s.require(ctx, principal, rbac.PermissionOcservServiceManage, "ocserv.service.restart"); err != nil {
		return err
	}
	actorID := principal.ID
	if err := s.svc.Restart(ctx); err != nil {
		_ = s.record(ctx, &actorID, "ocserv.service.restart", "ocserv_service", "", audit.OutcomeFailure, err.Error(), nil)
		return fmt.Errorf("restart ocserv service: %w", err)
	}
	_ = s.record(ctx, &actorID, "ocserv.service.restart", "ocserv_service", "", audit.OutcomeSuccess, "ocserv service restarted", nil)
	return nil
}

// ListSessions returns active ocserv sessions.
func (s *OcservService) ListSessions(ctx context.Context, principal auth.Principal) ([]ocserv.Session, error) {
	if err := s.require(ctx, principal, rbac.PermissionOcservSessionsRead, "ocserv.sessions.list"); err != nil {
		return nil, err
	}
	sessions, err := s.sessions.ListSessions(ctx)
	if err != nil {
		return nil, fmt.Errorf("list ocserv sessions: %w", err)
	}
	return sessions, nil
}

// DisconnectSession disconnects an active ocserv session.
func (s *OcservService) DisconnectSession(ctx context.Context, principal auth.Principal, sessionID string) error {
	if err := s.require(ctx, principal, rbac.PermissionOcservSessionsDisconnect, "ocserv.sessions.disconnect"); err != nil {
		return err
	}
	actorID := principal.ID
	if err := s.sessions.DisconnectSession(ctx, sessionID); err != nil {
		_ = s.record(ctx, &actorID, "ocserv.sessions.disconnect", "ocserv_session", sessionID, audit.OutcomeFailure, err.Error(), nil)
		return fmt.Errorf("disconnect ocserv session %s: %w", sessionID, err)
	}
	_ = s.record(ctx, &actorID, "ocserv.sessions.disconnect", "ocserv_session", sessionID, audit.OutcomeSuccess, "ocserv session disconnected", nil)
	return nil
}

// ListOcservUsers returns ocserv password-based users.
func (s *OcservService) ListOcservUsers(ctx context.Context, principal auth.Principal) ([]ocserv.PasswdUser, error) {
	if err := s.require(ctx, principal, rbac.PermissionOcservUsersRead, "ocserv.users.list"); err != nil {
		return nil, err
	}
	users, err := s.users.ListUsers(ctx)
	if err != nil {
		return nil, fmt.Errorf("list ocserv users: %w", err)
	}
	return users, nil
}

// CreateOcservUser creates a new ocserv password-based user.
func (s *OcservService) CreateOcservUser(ctx context.Context, principal auth.Principal, username, password string) error {
	if err := s.require(ctx, principal, rbac.PermissionOcservUsersCreate, "ocserv.users.create"); err != nil {
		return err
	}
	actorID := principal.ID
	if err := s.users.CreateUser(ctx, username, password); err != nil {
		_ = s.record(ctx, &actorID, "ocserv.users.create", "ocserv_user", username, audit.OutcomeFailure, err.Error(), nil)
		return fmt.Errorf("create ocserv user %s: %w", username, err)
	}
	_ = s.record(ctx, &actorID, "ocserv.users.create", "ocserv_user", username, audit.OutcomeSuccess, "ocserv user created", nil)
	return nil
}

// DeleteOcservUser deletes an ocserv password-based user.
func (s *OcservService) DeleteOcservUser(ctx context.Context, principal auth.Principal, username string) error {
	if err := s.require(ctx, principal, rbac.PermissionOcservUsersDelete, "ocserv.users.delete"); err != nil {
		return err
	}
	actorID := principal.ID
	if err := s.users.DeleteUser(ctx, username); err != nil {
		_ = s.record(ctx, &actorID, "ocserv.users.delete", "ocserv_user", username, audit.OutcomeFailure, err.Error(), nil)
		return fmt.Errorf("delete ocserv user %s: %w", username, err)
	}
	_ = s.record(ctx, &actorID, "ocserv.users.delete", "ocserv_user", username, audit.OutcomeSuccess, "ocserv user deleted", nil)
	return nil
}

// UpdateOcservUserPassword updates an ocserv user's password.
func (s *OcservService) UpdateOcservUserPassword(ctx context.Context, principal auth.Principal, username, newPassword string) error {
	if err := s.require(ctx, principal, rbac.PermissionOcservUsersUpdate, "ocserv.users.update_password"); err != nil {
		return err
	}
	actorID := principal.ID
	if err := s.users.UpdatePassword(ctx, username, newPassword); err != nil {
		_ = s.record(ctx, &actorID, "ocserv.users.update_password", "ocserv_user", username, audit.OutcomeFailure, err.Error(), nil)
		return fmt.Errorf("update ocserv user password %s: %w", username, err)
	}
	_ = s.record(ctx, &actorID, "ocserv.users.update_password", "ocserv_user", username, audit.OutcomeSuccess, "ocserv user password updated", nil)
	return nil
}

// ConfigIssues returns ocserv configuration validation issues.
func (s *OcservService) ConfigIssues(ctx context.Context, principal auth.Principal) ([]ocserv.ConfigIssue, error) {
	if err := s.require(ctx, principal, rbac.PermissionOcservConfigRead, "ocserv.config.issues"); err != nil {
		return nil, err
	}
	issues, err := s.validator.ValidateConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("validate ocserv config: %w", err)
	}
	return issues, nil
}

func (s *OcservService) require(ctx context.Context, principal auth.Principal, permission rbac.Permission, action string) error {
	if !principal.Authenticated() {
		_ = s.record(ctx, nil, action, "ocserv", "", audit.OutcomeDenied, "ocserv authentication required", nil)
		return ErrUnauthenticated
	}
	if !s.authorizer.Allowed(principalRoles(principal), permission) {
		actorID := principal.ID
		_ = s.record(ctx, &actorID, action, "ocserv", "", audit.OutcomeDenied, "ocserv permission denied", nil)
		return ErrPermissionDenied
	}
	return nil
}

func (s *OcservService) record(ctx context.Context, actorID *int64, action string, resourceType string, resourceID string, outcome audit.Outcome, message string, metadata map[string]string) error {
	return s.recorder.Record(ctx, audit.Event{ActorID: actorID, Action: action, ResourceType: resourceType, ResourceID: resourceID, Outcome: outcome, Message: message, Metadata: metadata})
}
```

- [ ] **Step 2: Update DashboardService to use readiness checker**

In `dashboard_service.go`, add a `readiness` field to `DashboardService` and update `Summary()` to use it instead of hardcoded status:

```go
// Add field to DashboardService struct:
type DashboardService struct {
	store      dashboardStore
	recorder   audit.Recorder
	authorizer rbac.Authorizer
	readiness  *ocserv.ReadinessChecker  // NEW
	now        func() time.Time
}
```

Update `NewDashboardService` to accept readiness:
```go
func NewDashboardService(s *store.Store, recorder audit.Recorder, authorizer rbac.Authorizer, readiness *ocserv.ReadinessChecker) *DashboardService {
	if recorder == nil {
		recorder = audit.NoopRecorder{}
	}
	return &DashboardService{store: s, recorder: recorder, authorizer: authorizer, readiness: readiness, now: func() time.Time { return time.Now().UTC() }}
}
```

Update `Summary()` to call readiness check:
```go
// Replace the hardcoded OcservStatusSummary with:
var ocservStatus OcservStatusSummary
if s.readiness != nil {
    r, err := s.readiness.Check(ctx)
    if err != nil {
        ocservStatus = OcservStatusSummary{Status: "error", Configured: false, Message: fmt.Sprintf("readiness check failed: %v", err)}
    } else {
        ocservStatus = readinessToStatus(r)
    }
} else {
    ocservStatus = OcservStatusSummary{Status: "not_configured", Configured: false, Message: "live ocserv integration is not configured"}
}
```

Add helper function:
```go
func readinessToStatus(r *ocserv.OcservReadiness) OcservStatusSummary {
    if r.BinaryPresent && r.ServiceRunning && r.ConfigValid {
        return OcservStatusSummary{Status: "running", Configured: true, Message: "ocserv is running"}
    }
    if r.BinaryPresent && r.ConfigValid {
        return OcservStatusSummary{Status: "installed", Configured: true, Message: "ocserv is installed but not running"}
    }
    if r.BinaryPresent && !r.ConfigValid {
        return OcservStatusSummary{Status: "misconfigured", Configured: false, Message: fmt.Sprintf("ocserv config error: %s", r.ConfigErrors)}
    }
    if !r.BinaryPresent {
        return OcservStatusSummary{Status: "not_installed", Configured: false, Message: "ocserv binary not found"}
    }
    return OcservStatusSummary{Status: "unknown", Configured: false, Message: "ocserv status unknown"}
}
```

- [ ] **Step 3: Update dashboard_service_test.go**

Update `NewDashboardService` calls in tests to pass `nil` for readiness (maintaining backward compatibility):

All existing test calls like `NewDashboardService(s, recorder, rbac.NewDefaultAuthorizer())` need to become `NewDashboardService(s, recorder, rbac.NewDefaultAuthorizer(), nil)`.

- [ ] **Step 4: Add ocserv service to App and add API routes**

In `server.go`, add `ocservService` to `App` struct and add routes:

```go
type App struct {
	authService       *app.AuthService
	dashboardService  *app.DashboardService
	monitoringService *app.MonitoringService
	settingsService   *app.SettingsService
	ocservService      *app.OcservService  // NEW
	authorizer        rbac.Authorizer
	cookieSecure      bool
	now               func() time.Time
}
```

Update `NewAppRouter` to accept and wire ocservService:
```go
func NewAppRouter(
	authService *app.AuthService,
	dashboardService *app.DashboardService,
	monitoringService *app.MonitoringService,
	settingsService *app.SettingsService,
	ocservService *app.OcservService,  // NEW
	authorizer rbac.Authorizer,
	opts AppOptions,
) http.Handler {
```

Add routes:
```go
// ocserv management endpoints
mux.HandleFunc("/api/v1/ocserv/readiness", appHandler.handleOcservReadiness)
mux.HandleFunc("/api/v1/ocserv/service/status", appHandler.handleOcservServiceStatus)
mux.HandleFunc("/api/v1/ocserv/service/start", appHandler.handleOcservServiceStart)
mux.HandleFunc("/api/v1/ocserv/service/stop", appHandler.handleOcservServiceStop)
mux.HandleFunc("/api/v1/ocserv/service/restart", appHandler.handleOcservServiceRestart)
mux.HandleFunc("/api/v1/ocserv/sessions", appHandler.handleOcservSessionsList)
mux.HandleFunc("/api/v1/ocserv/sessions/", appHandler.handleOcservSessionDisconnect)
mux.HandleFunc("/api/v1/ocserv/users", appHandler.handleOcservUsersList)
mux.HandleFunc("/api/v1/ocserv/users/", appHandler.handleOcservUserCRUD)
mux.HandleFunc("/api/v1/ocserv/config/issues", appHandler.handleOcservConfigIssues)
```

- [ ] **Step 5: Write ocserv HTTP handlers**

Create `backend/internal/server/ocserv_handlers.go` with handler implementations following the existing patterns (authenticate, call service, writeJSON/writeError).

- [ ] **Step 6: Update main.go wiring**

In `main.go`, create the ocserv adapters and service:

```go
// After config loading:
runner := ocserv.NewOSCommandRunner()
svcMgr := ocserv.NewSystemdManager(cfg.Service.SystemdUnitName, runner)
readiness := ocserv.NewReadinessChecker(ocserv.ReadinessConfig{
    OcservBinPath:   cfg.Installation.OcservBinPath,
    OcctlBinPath:    cfg.Installation.OcctlBinPath,
    OcpasswdBinPath: cfg.Installation.OcpasswdBinPath,
    ConfigPath:      cfg.Installation.OcservConfigPath,
    SocketPath:      cfg.Installation.OcservSocketPath,
    UnitName:        cfg.Service.SystemdUnitName,
}, runner, svcMgr)
sessionMgr := ocserv.NewOcctlAdapter(cfg.Installation.OcctlBinPath, cfg.Installation.OcservSocketPath, runner)
passwdMgr := ocserv.NewOcpasswdAdapter(cfg.Installation.OcpasswdBinPath, cfg.Installation.OcpasswdFilePath, cfg.Installation.OcservConfigPath, runner)
configValidator := ocserv.NewConfigValidator(cfg.Installation.OcservConfigPath)
ocservService := app.NewOcservService(readiness, sessionMgr, passwdMgr, svcMgr, configValidator, recorder, rbac.NewDefaultAuthorizer())
dashboardService := app.NewDashboardService(persistence, recorder, rbac.NewDefaultAuthorizer(), readiness)
```

Update `NewAppRouter` call to include `ocservService`.

- [ ] **Step 7: Run all backend tests**

Run: `cd /opt/git/ocserv_panel/backend && go test ./... -v -count=1`
Expected: All tests pass.

- [ ] **Step 8: Commit**

```bash
git add backend/internal/app/ocserv_service.go backend/internal/app/ocserv_service_test.go backend/internal/app/dashboard_service.go backend/internal/server/server.go backend/internal/server/ocserv_handlers.go backend/internal/server/ocserv_handlers_test.go backend/cmd/ocserv-panel/main.go
git commit -m "feat(ocserv): add OcservService, API handlers, and dashboard readiness integration

OcservService orchestrates readiness, sessions, users, service, and config
with RBAC checks and audit logging. Dashboard now shows real ocserv status
instead of hardcoded 'not_configured'. API endpoints for all ocserv operations."
```

---

## Task 11: Frontend API Types and Functions

**Files:**
- Modify: `frontend/src/lib/api.ts`

- [ ] **Step 1: Add ocserv API types and functions to api.ts**

Add the following types and functions:

```typescript
// ocserv types
export type OcservReadiness = {
  binary_present: boolean;
  binary_version: string;
  config_present: boolean;
  config_valid: boolean;
  config_errors?: string;
  service_running: boolean;
  socket_present: boolean;
  occtl_present: boolean;
  ocpasswd_present: boolean;
};

export type OcservServiceStatus = 'active' | 'inactive' | 'failed' | 'not_found';

export type OcservSession = {
  ID: string;
  Username: string;
  RemoteAddress: string;
  ConnectedAt: string;
};

export type OcservPasswdUser = {
  username: string;
  group: string;
  active: boolean;
};

export type OcservConfigIssue = {
  severity: 'error' | 'warning' | 'info';
  key: string;
  message: string;
  suggestion?: string;
};

// ocserv API functions
export function getOcservReadiness(): Promise<OcservReadiness> {
  return apiFetch<OcservReadiness>('/api/v1/ocserv/readiness');
}

export function getOcservServiceStatus(): Promise<{ status: OcservServiceStatus }> {
  return apiFetch<{ status: OcservServiceStatus }>('/api/v1/ocserv/service/status');
}

export function startOcservService(): Promise<{ ok: boolean }> {
  return apiFetch<{ ok: boolean }>('/api/v1/ocserv/service/start', { method: 'POST' });
}

export function stopOcservService(): Promise<{ ok: boolean }> {
  return apiFetch<{ ok: boolean }>('/api/v1/ocserv/service/stop', { method: 'POST' });
}

export function restartOcservService(): Promise<{ ok: boolean }> {
  return apiFetch<{ ok: boolean }>('/api/v1/ocserv/service/restart', { method: 'POST' });
}

export function getOcservSessions(): Promise<OcservSession[]> {
  return apiFetch<OcservSession[]>('/api/v1/ocserv/sessions');
}

export function disconnectOcservSession(sessionId: string): Promise<{ ok: boolean }> {
  return apiFetch<{ ok: boolean }>(`/api/v1/ocserv/sessions/${encodeURIComponent(sessionId)}/disconnect`, { method: 'POST' });
}

export function getOcservUsers(): Promise<OcservPasswdUser[]> {
  return apiFetch<OcservPasswdUser[]>('/api/v1/ocserv/users');
}

export function createOcservUser(username: string, password: string): Promise<{ ok: boolean }> {
  return apiFetch<{ ok: boolean }>('/api/v1/ocserv/users', { method: 'POST', body: { username, password } });
}

export function deleteOcservUser(username: string): Promise<{ ok: boolean }> {
  return apiFetch<{ ok: boolean }>(`/api/v1/ocserv/users/${encodeURIComponent(username)}`, { method: 'DELETE' });
}

export function updateOcservUserPassword(username: string, password: string): Promise<{ ok: boolean }> {
  return apiFetch<{ ok: boolean }>(`/api/v1/ocserv/users/${encodeURIComponent(username)}/password`, { method: 'PUT', body: { password } });
}

export function getOcservConfigIssues(): Promise<OcservConfigIssue[]> {
  return apiFetch<OcservConfigIssue[]>('/api/v1/ocserv/config/issues');
}
```

- [ ] **Step 2: Update DashboardSummary type**

Update the `DashboardSummary` type to include the full ocserv readiness:

```typescript
export type DashboardSummary = {
  backendStatus?: string;
  ocservStatus?: string;
  activeSessionCount?: number;
  recentAudit?: AuditEvent[];
  analytics?: Record<string, unknown>;
  sampleData?: boolean | { enabled?: boolean; present?: boolean; message?: string };
  ocserv?: {
    status: string;
    configured: boolean;
    message: string;
  };
  [key: string]: unknown;
};
```

- [ ] **Step 3: Update dashboard ocserv status card**

In `+page.svelte`, update the ocserv status card to use real data:

```svelte
<CardStats
  title="ocserv status"
  value={summary?.ocserv?.status ?? summary?.ocservStatus ?? 'unknown'}
  subtitle={summary?.ocserv?.message ?? 'Local-only baseline; no live ocserv control is active.'}
  icon={Shield}
/>
```

- [ ] **Step 4: Commit**

```bash
git add frontend/src/lib/api.ts frontend/src/routes/+page.svelte
git commit -m "feat(frontend): add ocserv API types and update dashboard status card

Add types and functions for ocserv readiness, service, sessions, users,
and config issues. Update dashboard to show real ocserv status from API."
```

---

## Task 12: Sudoers Rules and Deployment

**Files:**
- Create: `deploy/sudoers.d/ocserv-panel`

- [ ] **Step 1: Create sudoers file**

Create `deploy/sudoers.d/ocserv-panel`:

```
# ocserv-panel sudoers rules
# Allow the ocserv-panel user to run specific ocserv and systemctl commands
# without a password prompt. These are the minimum required privileges.
#
# Install: sudo cp deploy/sudoers.d/ocserv-panel /etc/sudoers.d/ocserv-panel
# Verify: sudo visudo -c -f /etc/sudoers.d/ocserv-panel

ocserv-panel ALL=(root) NOPASSWD: /usr/local/sbin/ocserv -f -t
ocserv-panel ALL=(root) NOPASSWD: /usr/local/sbin/ocserv --version
ocserv-panel ALL=(root) NOPASSWD: /usr/local/bin/occtl show *
ocserv-panel ALL=(root) NOPASSWD: /usr/local/bin/occtl disconnect *
ocserv-panel ALL=(root) NOPASSWD: /usr/local/bin/ocpasswd -c /etc/ocserv/ocserv.conf *
ocserv-panel ALL=(root) NOPASSWD: /usr/bin/systemctl start ocserv
ocserv-panel ALL=(root) NOPASSWD: /usr/bin/systemctl stop ocserv
ocserv-panel ALL=(root) NOPASSWD: /usr/bin/systemctl restart ocserv
ocserv-panel ALL=(root) NOPASSWD: /usr/bin/systemctl status ocserv
ocserv-panel ALL=(root) NOPASSWD: /usr/bin/systemctl is-enabled ocserv
```

- [ ] **Step 2: Commit**

```bash
git add deploy/sudoers.d/ocserv-panel
git commit -m "deploy: add sudoers rules for ocserv-panel

Allow the ocserv-panel user to run ocserv config validation, occtl
session management, ocpasswd user management, and systemctl service
control without password prompts."
```

---

## Task 13: Full Integration Verification

- [ ] **Step 1: Run all backend tests**

Run: `cd /opt/git/ocserv_panel/backend && go test ./... -v -count=1`
Expected: All tests pass, 0 failures.

- [ ] **Step 2: Run backend build**

Run: `cd /opt/git/ocserv_panel/backend && go build ./...`
Expected: Compiles without errors.

- [ ] **Step 3: Run frontend type check**

Run: `cd /opt/git/ocserv_panel/frontend && npx svelte-check --tsconfig ./tsconfig.json`
Expected: No type errors.

- [ ] **Step 4: Run frontend build**

Run: `cd /opt/git/ocserv_panel/frontend && npm run build`
Expected: Build succeeds.

- [ ] **Step 5: Verify no regressions in existing tests**

Run: `cd /opt/git/ocserv_panel/backend && go test ./... -count=1 -race`
Expected: All tests pass with no race conditions.

- [ ] **Step 6: Final commit if any fixes needed**

If any test failures or build errors were found and fixed, commit the fixes.

---

## Self-Review

### Spec Coverage Check

| Spec Section | Task |
|---|---|
| Section 1: Config & Binary Path Extensions | Task 1 |
| Section 2: ocserv Readiness Detection | Task 5 |
| Section 3: Systemd Service Management | Task 6 |
| Section 4: Active Session Management via occtl | Task 7 |
| Section 5: ocpasswd User Management Adapter | Task 8 |
| Section 6: Production Config Guidance | Task 9 |
| Cross-cutting: API Endpoints | Task 10 |
| Cross-cutting: Sudoers Rules | Task 12 |
| Cross-cutting: Frontend Changes | Task 11 |

### Placeholder Scan

No TBDs, TODOs, or "implement later" patterns found. All steps contain complete code.

### Type Consistency Check

- `OcservReadiness` struct fields match between `readiness.go` and `ocserv_service.go` and `api.ts`
- `ServiceStatus` enum values match between `service.go` and `api.ts`
- `Manager` interface methods match between `client.go` and `occtl_adapter.go`
- `PasswdManager` interface methods match between `passwd.go` and `passwd_adapter.go`
- `ConfigIssue` struct fields match between `config_validator.go` and `api.ts`
- `CommandRunner` interface used consistently across `readiness.go`, `service_systemd.go`, `occtl_adapter.go`, `passwd_adapter.go`