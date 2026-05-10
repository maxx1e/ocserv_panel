# Backend Package Restructure Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Reorganise the Go backend from an over-partitioned implementation-layer layout into a domain-task layout: collapse `internal/ocserv/` and its 4 sub-packages into `internal/vpn/` with 6 business-task sub-packages, merge `internal/metrics/` + `internal/monitoring/` into `internal/monitor/`, and delete all compat shims.

**Architecture:** Move files in dependency order — leaves first (`vpn/exec`), then peers (`vpn/sessions`, `vpn/users`, `vpn/config`, `vpn/service`), then dependents (`vpn/health`), then the merged `monitor/`. Delete old packages only after all callers are updated. Each task ends with a passing `go build ./... && go test ./...`.

**Tech Stack:** Go 1.25, module `ocserv_panel/backend`, `go test ./...` for verification, no new dependencies.

---

## File Map

### New files created (empty shell → filled per task)

```
backend/internal/vpn/exec/runner.go        ← from ocserv/cmd/runner.go
backend/internal/vpn/exec/fake.go          ← from ocserv/cmd/fake.go
backend/internal/vpn/sessions/types.go     ← from ocserv/client.go (type aliases removed, direct defs)
backend/internal/vpn/sessions/parse.go     ← merged from ocserv/parse/*.go (all 10 source files)
backend/internal/vpn/sessions/manager.go   ← from ocserv/client.go (Manager interface + FakeManager)
backend/internal/vpn/sessions/adapter.go   ← from ocserv/occtl_adapter.go
backend/internal/vpn/users/manager.go      ← from ocserv/passwd.go
backend/internal/vpn/users/adapter.go      ← from ocserv/passwd_adapter.go
backend/internal/vpn/users/fake.go         ← from ocserv/passwd_fake.go
backend/internal/vpn/config/file.go        ← from ocserv/config/file.go
backend/internal/vpn/config/validator.go   ← from ocserv/config/validator.go (cidr inlined)
backend/internal/vpn/config/reload.go      ← from ocserv/config/reload.go
backend/internal/vpn/config/restart.go     ← from ocserv/config/restart.go
backend/internal/vpn/service/manager.go    ← from ocserv/service/manager.go
backend/internal/vpn/service/systemd.go    ← from ocserv/service/systemd.go
backend/internal/vpn/service/fake.go       ← from ocserv/service/fake.go
backend/internal/vpn/health/readiness.go   ← from ocserv/readiness.go
backend/internal/vpn/health/network.go     ← from ocserv/network_inspector.go
backend/internal/monitor/types.go          ← from metrics/types.go
backend/internal/monitor/tsdb.go           ← from metrics/tsdb_store.go
backend/internal/monitor/memory.go         ← from metrics/memory_store.go
backend/internal/monitor/collectors.go     ← from monitoring/collectors.go
backend/internal/monitor/service.go        ← from monitoring/collector_service.go
backend/internal/monitor/discovery.go      ← merged monitoring/iface_discovery.go + vpn_discovery.go
backend/internal/monitor/session_collector.go ← from monitoring/session_collector.go
```

### Files deleted (after all callers updated)

```
backend/internal/ocserv/          (entire directory — all 14 files)
backend/internal/ocserv/cmd/      (entire sub-package)
backend/internal/ocserv/config/   (entire sub-package)
backend/internal/ocserv/parse/    (entire sub-package)
backend/internal/ocserv/service/  (entire sub-package)
backend/internal/metrics/         (entire package)
backend/internal/monitoring/      (entire package)
```

### Files modified (import path updates only)

```
backend/cmd/ocserv-panel/main.go
backend/cmd/ocserv-panel/main_test.go
backend/internal/app/config_management_service.go
backend/internal/app/config_management_service_test.go
backend/internal/app/dashboard_service.go
backend/internal/app/health_service.go
backend/internal/app/ocserv_service.go
backend/internal/app/ocserv_service_test.go
backend/internal/app/ocserv_bulk_test.go
backend/internal/app/session_ingester.go
backend/internal/app/session_ingester_test.go
backend/internal/logs/journal_reader.go
backend/internal/logs/journal_reader_test.go
backend/internal/server/config_handlers.go
backend/internal/server/monitoring_helpers.go
backend/internal/server/ocserv_handlers.go
backend/internal/server/ocserv_handlers_test.go
backend/internal/server/server.go
backend/internal/server/server_test.go
```

---

## Task 1: Create `vpn/exec/` — command execution primitive

**Files:**
- Create: `backend/internal/vpn/exec/runner.go`
- Create: `backend/internal/vpn/exec/fake.go`

Move `ocserv/cmd/runner.go` and `ocserv/cmd/fake.go` into the new package. Only the `package` declaration and import paths inside the files change.

- [ ] **Step 1: Create the directory and copy runner.go**

```bash
mkdir -p /opt/ocserv_panel/backend/internal/vpn/exec
```

Create `backend/internal/vpn/exec/runner.go` with content identical to `ocserv/cmd/runner.go` except:
- First line: `package exec`
- No internal imports to update (runner.go has no `ocserv_panel` imports)

```go
package exec

import (
	"bytes"
	"context"
	"io"
	"os/exec"
	"strings"
)

// CommandResult holds the output of a completed command.
type CommandResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

// CommandRunner executes system commands.
type CommandRunner interface {
	Run(ctx context.Context, name string, args []string, stdin io.Reader) (CommandResult, error)
}

var (
	_ CommandRunner = (*OSCommandRunner)(nil)
)

// OSCommandRunner executes real system commands via exec.CommandContext.
type OSCommandRunner struct{}

// NewOSCommandRunner returns an OSCommandRunner.
func NewOSCommandRunner() *OSCommandRunner {
	return &OSCommandRunner{}
}

// Run executes the named command with the given args and optional stdin.
// args must be passed separately — never interpolate into a shell string.
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
		Stdout:   strings.TrimSpace(stdout.String()),
		Stderr:   strings.TrimSpace(stderr.String()),
		ExitCode: 0,
	}
	if code, ok := exitCodeFromErr(err); ok {
		result.ExitCode = code
		return result, nil
	}
	if err != nil {
		return result, err
	}
	return result, nil
}

func exitCodeFromErr(err error) (int, bool) {
	if err == nil {
		return 0, true
	}
	var exitErr *exec.ExitError
	if ok := isExitError(err, &exitErr); ok {
		return exitErr.ExitCode(), true
	}
	return 0, false
}

func isExitError(err error, target **exec.ExitError) bool {
	e, ok := err.(*exec.ExitError)
	if ok {
		*target = e
	}
	return ok
}
```

> **Note:** Read the actual content of `backend/internal/ocserv/cmd/runner.go` and copy it verbatim — only change `package cmd` → `package exec`. The code block above is illustrative; the real file is authoritative.

- [ ] **Step 2: Create `vpn/exec/fake.go`**

Read `backend/internal/ocserv/cmd/fake.go` and copy verbatim, changing only:
- `package cmd` → `package exec`
- No import path changes needed (fake.go has no `ocserv_panel` imports)

- [ ] **Step 3: Verify the new package compiles in isolation**

```bash
cd /opt/ocserv_panel/backend && go build ocserv_panel/backend/internal/vpn/exec
```

Expected: no output, exit 0.

- [ ] **Step 4: Copy the existing runner test**

Read `backend/internal/ocserv/cmd/runner_test.go` and create `backend/internal/vpn/exec/runner_test.go`, changing only:
- `package cmd` → `package exec` (or `package exec_test` if it was an external test)
- Import path `ocserv_panel/backend/internal/ocserv/cmd` → `ocserv_panel/backend/internal/vpn/exec`

- [ ] **Step 5: Run the new package tests**

```bash
cd /opt/ocserv_panel/backend && go test ocserv_panel/backend/internal/vpn/exec/... -v
```

Expected: all tests pass.

- [ ] **Step 6: Commit**

```bash
cd /opt/ocserv_panel && git add backend/internal/vpn/exec/ && git commit -m "refactor: add vpn/exec package (CommandRunner, OSCommandRunner, FakeCommandRunner)"
```

---

## Task 2: Create `vpn/sessions/` — session types, parsers, Manager

**Files:**
- Create: `backend/internal/vpn/sessions/types.go`
- Create: `backend/internal/vpn/sessions/parse.go`
- Create: `backend/internal/vpn/sessions/manager.go`
- Create: `backend/internal/vpn/sessions/adapter.go`

This is the largest task. The `parse/` sub-package (10 source files + 9 test files) merges into two files: `types.go` for data types and `parse.go` for all parsing logic.

- [ ] **Step 1: Create `vpn/sessions/types.go`**

```bash
mkdir -p /opt/ocserv_panel/backend/internal/vpn/sessions
```

Read `backend/internal/ocserv/parse/types.go`. Create `backend/internal/vpn/sessions/types.go`:
- `package sessions`
- Copy all type definitions verbatim

- [ ] **Step 2: Create `vpn/sessions/parse.go` — merge all parse source files**

Read these files in order and merge their non-`package`/non-`import` content into a single `backend/internal/vpn/sessions/parse.go`:
1. `backend/internal/ocserv/parse/helpers.go`
2. `backend/internal/ocserv/parse/sanitize.go`
3. `backend/internal/ocserv/parse/cidr.go`
4. `backend/internal/ocserv/parse/sessions.go`
5. `backend/internal/ocserv/parse/status.go`
6. `backend/internal/ocserv/parse/user.go`
7. `backend/internal/ocserv/parse/users.go`
8. `backend/internal/ocserv/parse/bans.go`
9. `backend/internal/ocserv/parse/routes.go`

The merged file header:
```go
package sessions

import (
    // collect and dedup all imports from all 9 source files
    "encoding/json"
    "fmt"
    "net"
    "regexp"
    "strconv"
    "strings"
    "time"
)
```

All function bodies are copied verbatim. No logic changes.

- [ ] **Step 3: Create `vpn/sessions/manager.go`**

Read `backend/internal/ocserv/client.go`. Create `backend/internal/vpn/sessions/manager.go`:
- `package sessions`
- Remove the `parse.*` type alias block (types are now defined directly in `types.go` in this package)
- Keep `ErrNotConfigured`, `ErrSessionNotFound` sentinel errors
- Keep the `Manager` interface definition verbatim
- Keep `FakeManager` struct and all its methods verbatim
- Update `var _ Manager = (*FakeManager)(nil)` assertion

- [ ] **Step 4: Create `vpn/sessions/adapter.go`**

Read `backend/internal/ocserv/occtl_adapter.go`. Create `backend/internal/vpn/sessions/adapter.go`:
- `package sessions`
- Replace import `"ocserv_panel/backend/internal/ocserv/cmd"` → `"ocserv_panel/backend/internal/vpn/exec"`
- Replace import `"ocserv_panel/backend/internal/ocserv/parse"` — remove it (parse functions are now in the same package)
- Replace all `parse.XXX(` calls with `XXX(` (same package, no qualifier needed)
- Replace all `cmd.CommandRunner` → `exec.CommandRunner`
- Replace import `"ocserv_panel/backend/internal/validation"` — keep as-is

- [ ] **Step 5: Verify the package compiles**

```bash
cd /opt/ocserv_panel/backend && go build ocserv_panel/backend/internal/vpn/sessions
```

Expected: no output, exit 0.

- [ ] **Step 6: Create `vpn/sessions/manager_test.go`**

Read `backend/internal/ocserv/client_test.go`. Create `backend/internal/vpn/sessions/manager_test.go`:
- Update package declaration (`package ocserv` or `package ocserv_test` → `package sessions` or `package sessions_test`)
- Replace import `"ocserv_panel/backend/internal/ocserv"` → `"ocserv_panel/backend/internal/vpn/sessions"`
- All test bodies verbatim

- [ ] **Step 7: Create merged test file for parse logic**

Read all `backend/internal/ocserv/parse/*_test.go` files. Create `backend/internal/vpn/sessions/parse_test.go` by merging them:
- Set `package sessions_test` (external test package, matching the original `package parse_test`)
- Import `"ocserv_panel/backend/internal/vpn/sessions"` replacing `"ocserv_panel/backend/internal/ocserv/parse"`
- Replace all `parse.XXX` references with `sessions.XXX`
- All test function bodies are copied verbatim

- [ ] **Step 7: Create adapter test**

Read `backend/internal/ocserv/occtl_adapter_test.go`. Create `backend/internal/vpn/sessions/adapter_test.go`:
- Update package declaration to match (external or internal)
- Replace all old import paths:
  - `ocserv_panel/backend/internal/ocserv` → `ocserv_panel/backend/internal/vpn/sessions`
  - `ocserv_panel/backend/internal/ocserv/cmd` → `ocserv_panel/backend/internal/vpn/exec`

- [ ] **Step 9: Run the package tests**

```bash
cd /opt/ocserv_panel/backend && go test ocserv_panel/backend/internal/vpn/sessions/... -v 2>&1 | tail -30
```

Expected: all tests pass.

- [ ] **Step 10: Commit**

```bash
cd /opt/ocserv_panel && git add backend/internal/vpn/sessions/ && git commit -m "refactor: add vpn/sessions package (Manager, OcctlAdapter, merged parsers)"
```

---

## Task 3: Create `vpn/users/` — VPN user lifecycle

**Files:**
- Create: `backend/internal/vpn/users/manager.go`
- Create: `backend/internal/vpn/users/adapter.go`
- Create: `backend/internal/vpn/users/fake.go`

- [ ] **Step 1: Create `vpn/users/manager.go`**

```bash
mkdir -p /opt/ocserv_panel/backend/internal/vpn/users
```

Read `backend/internal/ocserv/passwd.go`. Create `backend/internal/vpn/users/manager.go`:
- `package users`
- Copy `PasswdUser` type, `PasswdManager` interface, `ErrPasswordFileMissing` verbatim

- [ ] **Step 2: Create `vpn/users/adapter.go`**

Read `backend/internal/ocserv/passwd_adapter.go`. Create `backend/internal/vpn/users/adapter.go`:
- `package users`
- Replace import `"ocserv_panel/backend/internal/ocserv/cmd"` → `"ocserv_panel/backend/internal/vpn/exec"`
- Replace all `cmd.CommandRunner` → `exec.CommandRunner`
- All method bodies verbatim

- [ ] **Step 3: Create `vpn/users/fake.go`**

Read `backend/internal/ocserv/passwd_fake.go`. Create `backend/internal/vpn/users/fake.go`:
- `package users`
- No import path changes needed (passwd_fake.go has no internal imports)

- [ ] **Step 4: Verify the package compiles**

```bash
cd /opt/ocserv_panel/backend && go build ocserv_panel/backend/internal/vpn/users
```

Expected: no output, exit 0.

- [ ] **Step 5: Create adapter test**

Read `backend/internal/ocserv/passwd_adapter_test.go`. Create `backend/internal/vpn/users/adapter_test.go`:
- Update package declaration
- Replace import `ocserv_panel/backend/internal/ocserv` → `ocserv_panel/backend/internal/vpn/users`
- Replace import `ocserv_panel/backend/internal/ocserv/cmd` → `ocserv_panel/backend/internal/vpn/exec`
- Replace `ocserv.PasswdUser`, `ocserv.FakePasswdManager` etc. with `users.PasswdUser`, `users.FakePasswdManager`

- [ ] **Step 6: Run the package tests**

```bash
cd /opt/ocserv_panel/backend && go test ocserv_panel/backend/internal/vpn/users/... -v 2>&1 | tail -20
```

Expected: all tests pass.

- [ ] **Step 7: Commit**

```bash
cd /opt/ocserv_panel && git add backend/internal/vpn/users/ && git commit -m "refactor: add vpn/users package (PasswdManager, OcpasswdAdapter, FakePasswdManager)"
```

---

## Task 4: Create `vpn/config/` — config file management

**Files:**
- Create: `backend/internal/vpn/config/file.go`
- Create: `backend/internal/vpn/config/validator.go`
- Create: `backend/internal/vpn/config/reload.go`
- Create: `backend/internal/vpn/config/restart.go`

Key difference from the source: `validator.go` absorbs the two functions from `parse/cidr.go` (`IsNetmaskNotation`, `NetmaskToCIDR`) directly — they become unexported helpers `isNetmaskNotation` and `netmaskToCIDR` since they have no callers outside this file.

- [ ] **Step 1: Create `vpn/config/file.go`**

```bash
mkdir -p /opt/ocserv_panel/backend/internal/vpn/config
```

Read `backend/internal/ocserv/config/file.go`. Create `backend/internal/vpn/config/file.go`:
- `package config`
- No internal import path changes needed (file.go has no `ocserv_panel` imports)
- Copy verbatim

- [ ] **Step 2: Create `vpn/config/validator.go`**

Read both `backend/internal/ocserv/config/validator.go` and `backend/internal/ocserv/parse/cidr.go`.

Create `backend/internal/vpn/config/validator.go`:
- `package config`
- Remove import `"ocserv_panel/backend/internal/ocserv/parse"`
- Add `"net"` and `"strconv"` to imports (needed by the inlined cidr functions)
- Copy all of `validator.go` content verbatim
- At the end of the file, add the two cidr functions from `cidr.go` with unexported names:

```go
// isNetmaskNotation checks if a value is in "ip/dotted-quad-mask" format.
func isNetmaskNotation(value string) (ip, mask string, ok bool) {
    // copy body from parse/cidr.go IsNetmaskNotation verbatim
}

// netmaskToCIDR converts an IP and dotted-quad mask to CIDR notation.
func netmaskToCIDR(ip, mask string) (string, bool) {
    // copy body from parse/cidr.go NetmaskToCIDR verbatim
}
```

- Replace all `parse.IsNetmaskNotation(` → `isNetmaskNotation(` in the file
- Replace all `parse.NetmaskToCIDR(` → `netmaskToCIDR(` in the file

- [ ] **Step 3: Create `vpn/config/reload.go`**

Read `backend/internal/ocserv/config/reload.go`. Create `backend/internal/vpn/config/reload.go`:
- `package config`
- Replace import `"ocserv_panel/backend/internal/ocserv/cmd"` → `"ocserv_panel/backend/internal/vpn/exec"`
- Replace `cmd.CommandRunner` → `exec.CommandRunner`

- [ ] **Step 4: Create `vpn/config/restart.go`**

Read `backend/internal/ocserv/config/restart.go`. Create `backend/internal/vpn/config/restart.go`:
- `package config`
- No import path changes (restart.go has no `ocserv_panel` imports)
- Copy verbatim

- [ ] **Step 5: Verify the package compiles**

```bash
cd /opt/ocserv_panel/backend && go build ocserv_panel/backend/internal/vpn/config
```

Expected: no output, exit 0.

- [ ] **Step 6: Create config tests**

For each test file in `backend/internal/ocserv/config/`, create the corresponding file in `backend/internal/vpn/config/`:
- `file_test.go` → update `package config_test`, import `ocserv_panel/backend/internal/vpn/config`
- `validator_test.go` → update package + import; note: `cidr_test.go` tests are now part of `validator_test.go` — inline the cidr test cases into the validator test file or keep them as a separate `cidr_test.go` in `vpn/config/` with `package config_test`
- `reload_test.go` → update package + imports (`ocserv_panel/backend/internal/ocserv/cmd` → `ocserv_panel/backend/internal/vpn/exec`, `ocserv_panel/backend/internal/ocserv/config` → `ocserv_panel/backend/internal/vpn/config`)
- `restart_test.go` → update package + import path

- [ ] **Step 7: Run the package tests**

```bash
cd /opt/ocserv_panel/backend && go test ocserv_panel/backend/internal/vpn/config/... -v 2>&1 | tail -20
```

Expected: all tests pass.

- [ ] **Step 8: Commit**

```bash
cd /opt/ocserv_panel && git add backend/internal/vpn/config/ && git commit -m "refactor: add vpn/config package (ConfigFileManager, ConfigValidator, cidr inlined)"
```

---

## Task 5: Create `vpn/service/` — systemd unit control

**Files:**
- Create: `backend/internal/vpn/service/manager.go`
- Create: `backend/internal/vpn/service/systemd.go`
- Create: `backend/internal/vpn/service/fake.go`

- [ ] **Step 1: Create `vpn/service/manager.go`**

```bash
mkdir -p /opt/ocserv_panel/backend/internal/vpn/service
```

Read `backend/internal/ocserv/service/manager.go`. Create `backend/internal/vpn/service/manager.go`:
- `package service`
- No import path changes needed
- Copy verbatim

- [ ] **Step 2: Create `vpn/service/systemd.go`**

Read `backend/internal/ocserv/service/systemd.go`. Create `backend/internal/vpn/service/systemd.go`:
- `package service`
- Replace import `"ocserv_panel/backend/internal/ocserv/cmd"` → `"ocserv_panel/backend/internal/vpn/exec"`
- Replace `cmd.CommandRunner` → `exec.CommandRunner`

- [ ] **Step 3: Create `vpn/service/fake.go`**

Read `backend/internal/ocserv/service/fake.go`. Create `backend/internal/vpn/service/fake.go`:
- `package service`
- No import path changes needed
- Copy verbatim

- [ ] **Step 4: Verify the package compiles**

```bash
cd /opt/ocserv_panel/backend && go build ocserv_panel/backend/internal/vpn/service
```

Expected: no output, exit 0.

- [ ] **Step 5: Create service test**

Read `backend/internal/ocserv/service/systemd_test.go`. Create `backend/internal/vpn/service/systemd_test.go`:
- Update package declaration
- Replace `"ocserv_panel/backend/internal/ocserv/cmd"` → `"ocserv_panel/backend/internal/vpn/exec"`
- Replace `"ocserv_panel/backend/internal/ocserv/service"` → `"ocserv_panel/backend/internal/vpn/service"`

- [ ] **Step 6: Run the package tests**

```bash
cd /opt/ocserv_panel/backend && go test ocserv_panel/backend/internal/vpn/service/... -v 2>&1 | tail -20
```

Expected: all tests pass.

- [ ] **Step 7: Commit**

```bash
cd /opt/ocserv_panel && git add backend/internal/vpn/service/ && git commit -m "refactor: add vpn/service package (ServiceManager, SystemdManager, FakeServiceManager)"
```

---

## Task 6: Create `vpn/health/` — readiness and network inspection

**Files:**
- Create: `backend/internal/vpn/health/readiness.go`
- Create: `backend/internal/vpn/health/network.go`

This package imports `vpn/exec`, `vpn/service`, and `vpn/config` — all created in Tasks 1–5.

- [ ] **Step 1: Create `vpn/health/readiness.go`**

```bash
mkdir -p /opt/ocserv_panel/backend/internal/vpn/health
```

Read `backend/internal/ocserv/readiness.go`. Create `backend/internal/vpn/health/readiness.go`:
- `package health`
- Replace import `"ocserv_panel/backend/internal/ocserv/cmd"` → `"ocserv_panel/backend/internal/vpn/exec"`
- The type reference `ServiceManager` currently comes from the `ocserv` package (via compat shim from `service`). After the move it must import `"ocserv_panel/backend/internal/vpn/service"` directly and use `service.ServiceManager`
- Replace `cmd.CommandRunner` → `exec.CommandRunner`

- [ ] **Step 2: Create `vpn/health/network.go`**

Read `backend/internal/ocserv/network_inspector.go`. Create `backend/internal/vpn/health/network.go`:
- `package health`
- Replace import `"ocserv_panel/backend/internal/ocserv/cmd"` → `"ocserv_panel/backend/internal/vpn/exec"`
- The file calls `parseConfigLine(line)` which was routed through `config_service_compat.go`. Replace with a direct import of `"ocserv_panel/backend/internal/vpn/config"` and call `config.ParseConfigLine(line)`
- Replace `cmd.CommandRunner` → `exec.CommandRunner`

- [ ] **Step 3: Verify the package compiles**

```bash
cd /opt/ocserv_panel/backend && go build ocserv_panel/backend/internal/vpn/health
```

Expected: no output, exit 0.

- [ ] **Step 4: Create health tests**

Read `backend/internal/ocserv/readiness_test.go` and `backend/internal/ocserv/network_inspector_test.go` (if it exists). Create corresponding test files in `backend/internal/vpn/health/` updating import paths:
- `ocserv_panel/backend/internal/ocserv` → `ocserv_panel/backend/internal/vpn/health`
- `ocserv_panel/backend/internal/ocserv/cmd` → `ocserv_panel/backend/internal/vpn/exec`
- `ocserv_panel/backend/internal/ocserv/service` → `ocserv_panel/backend/internal/vpn/service`

- [ ] **Step 5: Run the package tests**

```bash
cd /opt/ocserv_panel/backend && go test ocserv_panel/backend/internal/vpn/health/... -v 2>&1 | tail -20
```

Expected: all tests pass.

- [ ] **Step 6: Commit**

```bash
cd /opt/ocserv_panel && git add backend/internal/vpn/health/ && git commit -m "refactor: add vpn/health package (ReadinessChecker, NetworkInspector)"
```

---

## Task 7: Create `monitor/` — merged metrics + monitoring

**Files:**
- Create: `backend/internal/monitor/types.go`
- Create: `backend/internal/monitor/tsdb.go`
- Create: `backend/internal/monitor/memory.go`
- Create: `backend/internal/monitor/collectors.go`
- Create: `backend/internal/monitor/service.go`
- Create: `backend/internal/monitor/discovery.go`
- Create: `backend/internal/monitor/session_collector.go`

- [ ] **Step 1: Create `monitor/types.go`**

```bash
mkdir -p /opt/ocserv_panel/backend/internal/monitor
```

Read `backend/internal/metrics/types.go`. Create `backend/internal/monitor/types.go`:
- `package monitor`
- No internal import changes needed
- Copy verbatim

- [ ] **Step 2: Create `monitor/tsdb.go`**

Read `backend/internal/metrics/tsdb_store.go`. Create `backend/internal/monitor/tsdb.go`:
- `package monitor`
- No internal import changes needed
- Copy verbatim

- [ ] **Step 3: Create `monitor/memory.go`**

Read `backend/internal/metrics/memory_store.go`. Create `backend/internal/monitor/memory.go`:
- `package monitor`
- No internal import changes needed
- Copy verbatim

- [ ] **Step 4: Create `monitor/collectors.go`**

Read `backend/internal/monitoring/collectors.go`. Create `backend/internal/monitor/collectors.go`:
- `package monitor`
- Replace import `"ocserv_panel/backend/internal/metrics"` → remove it (types are now in same package)
- Replace all `metrics.XXX` references with `XXX` (same package)

- [ ] **Step 5: Create `monitor/service.go`**

Read `backend/internal/monitoring/collector_service.go`. Create `backend/internal/monitor/service.go`:
- `package monitor`
- Replace import `"ocserv_panel/backend/internal/metrics"` → remove it (same package)
- Replace all `metrics.XXX` references with `XXX`

- [ ] **Step 6: Create `monitor/discovery.go`**

Read both `backend/internal/monitoring/iface_discovery.go` and `backend/internal/monitoring/vpn_discovery.go`. Merge into `backend/internal/monitor/discovery.go`:
- `package monitor`
- No internal import changes needed for either file
- Collect and dedup stdlib imports from both files

- [ ] **Step 7: Create `monitor/session_collector.go`**

Read `backend/internal/monitoring/session_collector.go`. Create `backend/internal/monitor/session_collector.go`:
- `package monitor`
- Replace import `"ocserv_panel/backend/internal/metrics"` → remove it (same package)
- Replace import `"ocserv_panel/backend/internal/ocserv"` → `"ocserv_panel/backend/internal/vpn/sessions"`
- Replace `ocserv.SessionStatusProvider` (or equivalent type) with `sessions.Manager` — read the source to confirm the exact interface used; the `SessionStatusProvider` interface in `monitoring` wraps a subset of `sessions.Manager`
- Replace all `metrics.XXX` and `ocserv.XXX` references accordingly

- [ ] **Step 8: Verify the package compiles**

```bash
cd /opt/ocserv_panel/backend && go build ocserv_panel/backend/internal/monitor
```

Expected: no output, exit 0.

- [ ] **Step 9: Create monitor tests**

For each test file:
- `metrics/memory_store_test.go` → `monitor/memory_test.go`: `package monitor` or `package monitor_test`, update import to `ocserv_panel/backend/internal/monitor`
- `metrics/tsdb_store_test.go` → `monitor/tsdb_test.go`: same
- `monitoring/collector_service_test.go` → `monitor/service_test.go`: update all imports (`metrics` and `monitoring` → `monitor`, `ocserv` → `vpn/sessions`)
- `monitoring/collectors_test.go` → `monitor/collectors_test.go`: update imports
- `monitoring/session_collector_test.go` → `monitor/session_collector_test.go`: update imports
- `monitoring/iface_discovery_test.go` → `monitor/discovery_test.go`: update imports
- `monitoring/vpn_discovery_test.go` → append to `monitor/discovery_test.go` or keep as `monitor/vpn_discovery_test.go`: update imports

- [ ] **Step 10: Run the package tests**

```bash
cd /opt/ocserv_panel/backend && go test ocserv_panel/backend/internal/monitor/... -v 2>&1 | tail -30
```

Expected: all tests pass.

- [ ] **Step 11: Commit**

```bash
cd /opt/ocserv_panel && git add backend/internal/monitor/ && git commit -m "refactor: add monitor package (merged metrics + monitoring)"
```

---

## Task 8: Update `internal/logs/` — import vpn/exec

**Files:**
- Modify: `backend/internal/logs/journal_reader.go`
- Modify: `backend/internal/logs/journal_reader_test.go`

`journal_reader.go` uses `cmd.CommandRunner` to shell out to `journalctl`. It must now import `vpn/exec` instead.

- [ ] **Step 1: Update `journal_reader.go`**

In `backend/internal/logs/journal_reader.go`:
- Replace import `"ocserv_panel/backend/internal/ocserv/cmd"` → `"ocserv_panel/backend/internal/vpn/exec"`
- Replace all `cmd.CommandRunner` → `exec.CommandRunner`
- Replace all `cmd.FakeCommandRunner` → `exec.FakeCommandRunner` (if present in this file)

- [ ] **Step 2: Update `journal_reader_test.go`**

In `backend/internal/logs/journal_reader_test.go`:
- Replace import `"ocserv_panel/backend/internal/ocserv"` (used for `ocserv.FakeCommandRunner` or similar) → `"ocserv_panel/backend/internal/vpn/exec"`
- Update all type references accordingly

- [ ] **Step 3: Verify logs package compiles and tests pass**

```bash
cd /opt/ocserv_panel/backend && go test ocserv_panel/backend/internal/logs/... -v 2>&1 | tail -20
```

Expected: all tests pass.

- [ ] **Step 4: Commit**

```bash
cd /opt/ocserv_panel && git add backend/internal/logs/ && git commit -m "refactor(logs): import vpn/exec instead of ocserv/cmd"
```

---

## Task 9: Update `internal/app/` — import new vpn/* and monitor paths

**Files:**
- Modify: `backend/internal/app/ocserv_service.go`
- Modify: `backend/internal/app/ocserv_service_test.go`
- Modify: `backend/internal/app/ocserv_bulk_test.go`
- Modify: `backend/internal/app/config_management_service.go`
- Modify: `backend/internal/app/config_management_service_test.go`
- Modify: `backend/internal/app/dashboard_service.go`
- Modify: `backend/internal/app/health_service.go`
- Modify: `backend/internal/app/session_ingester.go`
- Modify: `backend/internal/app/session_ingester_test.go`

- [ ] **Step 1: Update `ocserv_service.go`**

Read `backend/internal/app/ocserv_service.go`. Apply these import replacements:
- `"ocserv_panel/backend/internal/ocserv"` → `"ocserv_panel/backend/internal/vpn/sessions"` + `"ocserv_panel/backend/internal/vpn/users"`
- `ocservconfig "ocserv_panel/backend/internal/ocserv/config"` → `vpnconfig "ocserv_panel/backend/internal/vpn/config"`
- `"ocserv_panel/backend/internal/ocserv/service"` → `"ocserv_panel/backend/internal/vpn/service"`

Then update all type references in the file body:
- `ocserv.Manager` → `sessions.Manager`
- `ocserv.PasswdManager` → `users.PasswdManager`
- `ocserv.Session` → `sessions.Session`
- `ocserv.ConnectedUser` → `sessions.ConnectedUser`
- `ocserv.ErrNotConfigured` → `sessions.ErrNotConfigured`
- `ocserv.ErrPasswordFileMissing` → `users.ErrPasswordFileMissing`
- `ocservconfig.XXX` → `vpnconfig.XXX`
- `service.ServiceManager` → already correct since `service` package name is unchanged

- [ ] **Step 2: Update `ocserv_service_test.go` and `ocserv_bulk_test.go`**

Apply the same import and type reference substitutions from Step 1 to both test files:
- `ocserv_panel/backend/internal/ocserv` → `vpn/sessions` or `vpn/users` depending on which types are used
- `ocserv_panel/backend/internal/ocserv/config` → `vpn/config`
- `ocserv_panel/backend/internal/ocserv/service` → `vpn/service`

- [ ] **Step 3: Update `config_management_service.go`**

Read `backend/internal/app/config_management_service.go`. Apply:
- `"ocserv_panel/backend/internal/ocserv"` → `"ocserv_panel/backend/internal/vpn/health"` (for `NetworkInspector`)
- `ocservconfig "ocserv_panel/backend/internal/ocserv/config"` → `vpnconfig "ocserv_panel/backend/internal/vpn/config"`
- Replace `ocserv.NetworkInspector` → `health.NetworkInspector`
- Replace all `ocservconfig.XXX` → `vpnconfig.XXX`

- [ ] **Step 4: Update `config_management_service_test.go`**

Apply the same substitutions:
- `ocserv_panel/backend/internal/ocserv` → `ocserv_panel/backend/internal/vpn/health`
- `ocserv_panel/backend/internal/ocserv/cmd` → `ocserv_panel/backend/internal/vpn/exec`
- `ocserv_panel/backend/internal/ocserv/config` → `ocserv_panel/backend/internal/vpn/config`

- [ ] **Step 5: Update `dashboard_service.go`**

Read `backend/internal/app/dashboard_service.go`. Apply:
- `"ocserv_panel/backend/internal/metrics"` → `"ocserv_panel/backend/internal/monitor"`
- `"ocserv_panel/backend/internal/ocserv"` → `"ocserv_panel/backend/internal/vpn/health"`
- Replace `metrics.Store` → `monitor.Store`
- Replace `ocserv.ReadinessChecker` → `health.ReadinessChecker`
- Replace `ocserv.OcservReadiness` → `health.OcservReadiness`

- [ ] **Step 6: Update `health_service.go`**

Read `backend/internal/app/health_service.go`. Apply:
- `"ocserv_panel/backend/internal/ocserv"` → `"ocserv_panel/backend/internal/vpn/health"`
- Replace `ocserv.ReadinessChecker` → `health.ReadinessChecker`
- Replace other `ocserv.XXX` types as needed

- [ ] **Step 7: Update `session_ingester.go` and `session_ingester_test.go`**

Read both files. Apply:
- `"ocserv_panel/backend/internal/ocserv"` → `"ocserv_panel/backend/internal/vpn/sessions"`
- Replace `ocserv.Manager` → `sessions.Manager`
- Replace `ocserv.FakeManager` → `sessions.FakeManager`
- Replace `ocserv.Session` → `sessions.Session`

- [ ] **Step 8: Verify app package compiles and tests pass**

```bash
cd /opt/ocserv_panel/backend && go test ocserv_panel/backend/internal/app/... -v 2>&1 | tail -30
```

Expected: all tests pass.

- [ ] **Step 9: Commit**

```bash
cd /opt/ocserv_panel && git add backend/internal/app/ && git commit -m "refactor(app): import vpn/* and monitor instead of ocserv/* and metrics"
```

---

## Task 10: Update `internal/server/` — import new paths

**Files:**
- Modify: `backend/internal/server/server.go`
- Modify: `backend/internal/server/ocserv_handlers.go`
- Modify: `backend/internal/server/ocserv_handlers_test.go`
- Modify: `backend/internal/server/config_handlers.go`
- Modify: `backend/internal/server/monitoring_helpers.go`
- Modify: `backend/internal/server/server_test.go`

- [ ] **Step 1: Update `server.go`**

Read `backend/internal/server/server.go`. Apply:
- `"ocserv_panel/backend/internal/metrics"` → `"ocserv_panel/backend/internal/monitor"`
- `"ocserv_panel/backend/internal/ocserv"` → `"ocserv_panel/backend/internal/vpn/sessions"`
- Replace `metrics.Store` → `monitor.Store`
- Replace `ocserv.Manager` → `sessions.Manager`
- Replace `ocserv.PasswdManager` → add import for `vpn/users` and use `users.PasswdManager`
- Replace `ocserv.ReadinessChecker` → add import for `vpn/health` and use `health.ReadinessChecker`

- [ ] **Step 2: Update `ocserv_handlers.go`**

Read `backend/internal/server/ocserv_handlers.go`. Apply:
- `"ocserv_panel/backend/internal/ocserv"` → split into `vpn/sessions`, `vpn/users`, `vpn/health` as needed
- `"ocserv_panel/backend/internal/ocserv/service"` → `"ocserv_panel/backend/internal/vpn/service"`
- Replace all `ocserv.XXX` type references with the appropriate new package qualifier
- Replace all `service.ServiceStatus`, `service.ServiceActive` etc. — package name `service` is unchanged so these stay as-is, only the import path changes

- [ ] **Step 3: Update `ocserv_handlers_test.go`**

Read `backend/internal/server/ocserv_handlers_test.go`. Apply same substitutions as Step 2.

- [ ] **Step 4: Update `config_handlers.go`**

Read `backend/internal/server/config_handlers.go`. Apply:
- `ocservconfig "ocserv_panel/backend/internal/ocserv/config"` → `vpnconfig "ocserv_panel/backend/internal/vpn/config"`
- Replace all `ocservconfig.XXX` → `vpnconfig.XXX`

- [ ] **Step 5: Update `monitoring_helpers.go`**

Read `backend/internal/server/monitoring_helpers.go`. Apply:
- `"ocserv_panel/backend/internal/metrics"` → `"ocserv_panel/backend/internal/monitor"`
- `"ocserv_panel/backend/internal/monitoring"` → `"ocserv_panel/backend/internal/monitor"`
- Replace all `metrics.XXX` and `monitoring.XXX` type references with `monitor.XXX`

- [ ] **Step 6: Update `server_test.go`**

Read `backend/internal/server/server_test.go`. Apply:
- `"ocserv_panel/backend/internal/metrics"` → `"ocserv_panel/backend/internal/monitor"`
- `"ocserv_panel/backend/internal/ocserv"` → appropriate `vpn/*` packages
- `"ocserv_panel/backend/internal/ocserv/cmd"` → `"ocserv_panel/backend/internal/vpn/exec"`
- `ocservconfig "ocserv_panel/backend/internal/ocserv/config"` → `vpnconfig "ocserv_panel/backend/internal/vpn/config"`
- `"ocserv_panel/backend/internal/ocserv/service"` → `"ocserv_panel/backend/internal/vpn/service"`
- Update all type references accordingly

- [ ] **Step 7: Verify server package compiles and tests pass**

```bash
cd /opt/ocserv_panel/backend && go test ocserv_panel/backend/internal/server/... -v 2>&1 | tail -30
```

Expected: all tests pass.

- [ ] **Step 8: Commit**

```bash
cd /opt/ocserv_panel && git add backend/internal/server/ && git commit -m "refactor(server): import vpn/* and monitor instead of ocserv/* and metrics"
```

---

## Task 11: Update `cmd/ocserv-panel/main.go`

**Files:**
- Modify: `backend/cmd/ocserv-panel/main.go`
- Modify: `backend/cmd/ocserv-panel/main_test.go`

- [ ] **Step 1: Update `main.go`**

Read `backend/cmd/ocserv-panel/main.go`. Apply all import replacements:
- `"ocserv_panel/backend/internal/metrics"` → `"ocserv_panel/backend/internal/monitor"`
- `"ocserv_panel/backend/internal/monitoring"` → `"ocserv_panel/backend/internal/monitor"`
- `"ocserv_panel/backend/internal/ocserv"` → appropriate `vpn/*` packages
- `"ocserv_panel/backend/internal/ocserv/cmd"` → `"ocserv_panel/backend/internal/vpn/exec"`
- `ocservconfig "ocserv_panel/backend/internal/ocserv/config"` → `vpnconfig "ocserv_panel/backend/internal/vpn/config"`
- `"ocserv_panel/backend/internal/ocserv/service"` → `"ocserv_panel/backend/internal/vpn/service"`

Update all constructor calls and type references in the function bodies:
- `cmd.NewOSCommandRunner()` → `exec.NewOSCommandRunner()`
- `service.NewSystemdManager(...)` → unchanged (package name `service` stays, only import path changed)
- `ocserv.NewReadinessChecker(...)` → `health.NewReadinessChecker(...)`; add import for `vpn/health`
- `ocserv.NewOcctlAdapter(...)` → `sessions.NewOcctlAdapter(...)`
- `ocserv.NewOcpasswdAdapter(...)` → `users.NewOcpasswdAdapter(...)`
- `ocservconfig.NewConfigFileManager(...)` → `vpnconfig.NewConfigFileManager(...)`
- `ocservconfig.NewConfigReloader(...)` → `vpnconfig.NewConfigReloader(...)`
- `ocservconfig.NewConfigValidator(...)` → `vpnconfig.NewConfigValidator(...)`
- `ocservconfig.NewRestartDetector()` → `vpnconfig.NewRestartDetector()`
- `ocserv.NewNetworkInspectorWithConfig(...)` → `health.NewNetworkInspectorWithConfig(...)`
- `monitoring.NewCollectorService(...)` → `monitor.NewCollectorService(...)`
- `monitoring.NewSessionCollector(...)` → `monitor.NewSessionCollector(...)`
- `monitoring.NewCPUCollector(...)` → `monitor.NewCPUCollector(...)`
- `monitoring.NewMemoryCollector(...)` → `monitor.NewMemoryCollector(...)`
- `monitoring.NewInterfaceCollector(...)` → `monitor.NewInterfaceCollector(...)`
- `metrics.OpenTSDBStore(...)` → `monitor.OpenTSDBStore(...)`
- `metrics.NewMemoryStore()` → `monitor.NewMemoryStore()`

- [ ] **Step 2: Update `main_test.go`**

Apply the same import and reference substitutions to `backend/cmd/ocserv-panel/main_test.go`.

- [ ] **Step 3: Full build and test**

```bash
cd /opt/ocserv_panel/backend && go build ./... 2>&1
```

Expected: no errors.

```bash
cd /opt/ocserv_panel/backend && go test ./... 2>&1 | tail -40
```

Expected: all packages pass.

- [ ] **Step 4: Commit**

```bash
cd /opt/ocserv_panel && git add backend/cmd/ && git commit -m "refactor(main): import vpn/* and monitor, complete wiring update"
```

---

## Task 12: Delete old packages

Only run this task after Task 11's `go build ./...` and `go test ./...` pass with zero errors.

- [ ] **Step 1: Verify no remaining references to old packages**

```bash
grep -r "\"ocserv_panel/backend/internal/ocserv\|\"ocserv_panel/backend/internal/metrics\|\"ocserv_panel/backend/internal/monitoring" /opt/ocserv_panel/backend --include="*.go"
```

Expected: **no output**. If any matches appear, go back and fix them before proceeding.

- [ ] **Step 2: Delete old packages**

```bash
rm -rf /opt/ocserv_panel/backend/internal/ocserv
rm -rf /opt/ocserv_panel/backend/internal/metrics
rm -rf /opt/ocserv_panel/backend/internal/monitoring
```

- [ ] **Step 3: Full build and test after deletion**

```bash
cd /opt/ocserv_panel/backend && go build ./... && go test ./...
```

Expected: all packages pass, zero errors.

- [ ] **Step 4: Verify no compat shims remain**

```bash
find /opt/ocserv_panel/backend -name "*_compat.go"
```

Expected: **no output**.

- [ ] **Step 5: Final commit**

```bash
cd /opt/ocserv_panel && git add -A && git commit -m "refactor: delete ocserv/, metrics/, monitoring/ — replaced by vpn/* and monitor/"
```

---

## Task 13: Final verification

- [ ] **Step 1: Full build**

```bash
cd /opt/ocserv_panel/backend && go build ./...
```

Expected: no output, exit 0.

- [ ] **Step 2: Full test suite**

```bash
cd /opt/ocserv_panel/backend && go test ./... -count=1
```

Expected: all packages `ok`, no failures.

- [ ] **Step 3: Confirm package structure**

```bash
find /opt/ocserv_panel/backend/internal -type d | sort
```

Expected output includes:
```
.../internal/app
.../internal/audit
.../internal/auth
.../internal/config
.../internal/db
.../internal/logging
.../internal/logs
.../internal/monitor
.../internal/rbac
.../internal/sanitize
.../internal/server
.../internal/store
.../internal/validation
.../internal/vpn
.../internal/vpn/config
.../internal/vpn/exec
.../internal/vpn/health
.../internal/vpn/service
.../internal/vpn/sessions
.../internal/vpn/users
```

Does **not** include: `ocserv`, `metrics`, `monitoring`.

- [ ] **Step 4: Confirm no compat shims and no old imports**

```bash
find /opt/ocserv_panel/backend -name "*_compat.go" && \
grep -r "internal/ocserv\|internal/metrics\b\|internal/monitoring" /opt/ocserv_panel/backend --include="*.go"
```

Expected: **no output on either command**.

- [ ] **Step 5: Commit summary**

```bash
cd /opt/ocserv_panel && git commit --allow-empty -m "chore: backend restructure complete — vpn/* + monitor/ replace ocserv/* + metrics + monitoring"
```
