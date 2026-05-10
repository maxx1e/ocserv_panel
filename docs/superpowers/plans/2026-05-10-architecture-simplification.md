# Architecture Simplification Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Reduce complexity by collapsing thin RBAC-wrapper services, splitting the flat `ocserv/` package into navigable sub-packages, splitting `store.go` into focused files, and splitting the frontend's monolithic `api.ts` and dashboard page.

**Architecture:** RBAC enforcement moves to HTTP middleware so services only contain real domain logic. `internal/ocserv/` gains `cmd/`, `parse/`, `config/`, `service/` sub-packages. `store/store.go` splits into five same-package files. Frontend `lib/api.ts` (1026 lines) becomes a `lib/api/` directory with a re-export index so all existing imports are unchanged.

**Tech Stack:** Go 1.25, `github.com/rs/zerolog`, `modernc.org/sqlite`, SvelteKit 2 + Svelte 5 runes, Tailwind CSS v4, TypeScript 5.

**Skill Notes:**
- Go files: follow `golang-code-style` and `golang-testing` skills.
- Svelte files: follow `svelte-runes` and `sveltekit-data-flow` skills.
- Each task ends with `go test ./...` or `pnpm check` verification before commit.

---

## File Map

### Backend — files created / moved / deleted

| Status | Path |
|--------|------|
| **Split** | `internal/store/store.go` → `db.go`, `admin.go`, `audit.go`, `history.go`, `settings.go` |
| **New** | `internal/validation/validation.go` |
| **New** | `internal/validation/validation_test.go` |
| **Move** | `internal/ocserv/validate.go` → deleted (content moves to `internal/validation/`) |
| **Move** | `internal/ocserv/validate_test.go` → deleted (test moves to `internal/validation/`) |
| **New dir** | `internal/ocserv/cmd/` |
| **Move** | `internal/ocserv/runner.go` → `internal/ocserv/cmd/runner.go` |
| **Move** | `internal/ocserv/runner_test.go` → `internal/ocserv/cmd/runner_test.go` |
| **New** | `internal/ocserv/cmd/fake.go` (FakeCommandRunner extracted from runner.go) |
| **New dir** | `internal/ocserv/parse/` |
| **Move** | `internal/ocserv/occtl_sessions_parser.go` → `internal/ocserv/parse/sessions.go` |
| **Move** | `internal/ocserv/occtl_sessions_parser_test.go` → `internal/ocserv/parse/sessions_test.go` |
| **Move** | `internal/ocserv/occtl_users_parser.go` → `internal/ocserv/parse/users.go` |
| **Move** | `internal/ocserv/occtl_users_parser_test.go` → `internal/ocserv/parse/users_test.go` |
| **Move** | `internal/ocserv/occtl_user_parser.go` → `internal/ocserv/parse/user.go` |
| **Move** | `internal/ocserv/occtl_user_parser_test.go` → `internal/ocserv/parse/user_test.go` |
| **Move** | `internal/ocserv/occtl_bans_parser.go` → `internal/ocserv/parse/bans.go` |
| **Move** | `internal/ocserv/occtl_bans_parser_test.go` → `internal/ocserv/parse/bans_test.go` |
| **Move** | `internal/ocserv/occtl_routes_parser.go` → `internal/ocserv/parse/routes.go` |
| **Move** | `internal/ocserv/occtl_routes_parser_test.go` → `internal/ocserv/parse/routes_test.go` |
| **Move** | `internal/ocserv/occtl_status_parser.go` → `internal/ocserv/parse/status.go` |
| **Move** | `internal/ocserv/occtl_status_parser_test.go` → `internal/ocserv/parse/status_test.go` |
| **Move** | `internal/ocserv/cidr_convert.go` → `internal/ocserv/parse/cidr.go` |
| **Move** | `internal/ocserv/cidr_convert_test.go` → `internal/ocserv/parse/cidr_test.go` |
| **New dir** | `internal/ocserv/config/` |
| **Move** | `internal/ocserv/config_file.go` → `internal/ocserv/config/file.go` |
| **Move** | `internal/ocserv/config_file_test.go` → `internal/ocserv/config/file_test.go` |
| **Move** | `internal/ocserv/config_validator.go` → `internal/ocserv/config/validator.go` |
| **Move** | `internal/ocserv/config_validator_test.go` → `internal/ocserv/config/validator_test.go` |
| **Move** | `internal/ocserv/config_reload.go` → `internal/ocserv/config/reload.go` |
| **Move** | `internal/ocserv/config_reload_test.go` → `internal/ocserv/config/reload_test.go` |
| **Move** | `internal/ocserv/config_restart_detector.go` → `internal/ocserv/config/restart.go` |
| **Move** | `internal/ocserv/config_restart_detector_test.go` → `internal/ocserv/config/restart_test.go` |
| **New dir** | `internal/ocserv/service/` |
| **Move** | `internal/ocserv/service.go` → `internal/ocserv/service/manager.go` |
| **Move** | `internal/ocserv/service_systemd.go` → `internal/ocserv/service/systemd.go` |
| **Move** | `internal/ocserv/service_systemd_test.go` → `internal/ocserv/service/systemd_test.go` |
| **Move** | `internal/ocserv/service_fake.go` → `internal/ocserv/service/fake.go` |
| **New** | `internal/server/middleware.go` |
| **Delete** | `internal/app/monitoring_service.go` |
| **Delete** | `internal/app/monitoring_service_test.go` |
| **Delete** | `internal/app/log_tail_service.go` |
| **Delete** | `internal/app/log_tail_service_test.go` |
| **Delete** | `internal/app/health_service.go` |
| **Delete** | `internal/app/health_service_test.go` |
| **Delete** | `internal/app/housekeeping.go` |
| **Delete** | `internal/app/housekeeping_test.go` |
| **Modify** | `internal/app/app_errors.go` (or wherever ErrLogSourceUnavailable lives — keep) |
| **Modify** | `internal/server/health_handlers.go` (direct calls, no services) |
| **Modify** | `internal/server/server.go` (remove 3 service fields, add middleware field, update router) |
| **Modify** | `internal/store/store.go` → deleted after split |
| **Modify** | `internal/ocserv/occtl_adapter.go` (update import `ocserv/cmd`, `ocserv/parse`) |
| **Modify** | `internal/ocserv/passwd_adapter.go` (update import `ocserv/cmd`) |
| **Modify** | `internal/ocserv/readiness.go` (update import `ocserv/config`, `ocserv/cmd`) |
| **Modify** | `internal/ocserv/network_inspector.go` (update imports) |
| **Modify** | `internal/app/ocserv_service.go` (update 16 validate call sites + service/config imports) |
| **Modify** | `internal/app/config_management_service.go` (update config/ imports) |
| **Modify** | `backend/cmd/ocserv-panel/main.go` (update imports, remove housekeeping, wire store.StartHousekeeper) |

### Frontend — files created / moved / deleted

| Status | Path |
|--------|------|
| **New dir** | `frontend/src/lib/api/` |
| **New** | `frontend/src/lib/api/index.ts` (re-exports all) |
| **New** | `frontend/src/lib/api/auth.ts` |
| **New** | `frontend/src/lib/api/sessions.ts` |
| **New** | `frontend/src/lib/api/users.ts` |
| **New** | `frontend/src/lib/api/monitoring.ts` |
| **New** | `frontend/src/lib/api/settings.ts` |
| **New** | `frontend/src/lib/api/config.ts` |
| **New** | `frontend/src/lib/api/audit.ts` |
| **New** | `frontend/src/lib/api/health.ts` |
| **Delete** | `frontend/src/lib/api.ts` (replaced by `lib/api/index.ts`) |
| **New** | `frontend/src/routes/DashboardHealth.svelte` |
| **New** | `frontend/src/routes/DashboardMetrics.svelte` |
| **New** | `frontend/src/routes/DashboardSessions.svelte` |
| **New** | `frontend/src/routes/DashboardActivity.svelte` |
| **Modify** | `frontend/src/routes/+page.svelte` (thin orchestrator, delegate to sub-components) |

---

## Task 1: Split `internal/store/store.go` into focused files

**Files:**
- Create: `backend/internal/store/db.go`
- Create: `backend/internal/store/admin.go`
- Create: `backend/internal/store/audit.go`
- Create: `backend/internal/store/history.go`
- Create: `backend/internal/store/settings.go`
- Delete: `backend/internal/store/store.go`

- [ ] **Step 1: Verify current tests pass**

```bash
cd /opt/ocserv_panel && go test ./backend/internal/store/... -v 2>&1 | tail -20
```
Expected: all PASS. Note the test count.

- [ ] **Step 2: Create `db.go` with Store struct, package errors, and `New()`**

Create `/opt/ocserv_panel/backend/internal/store/db.go`:

```go
package store

import (
	"database/sql"
	"errors"
)

var ErrFirstOwnerExists = errors.New("admin user already exists")
var ErrDuplicateUsername = errors.New("username already exists")

// Store wraps a SQLite database with application-level query methods.
type Store struct {
	db *sql.DB
}

// New returns a Store backed by the given database connection.
func New(db *sql.DB) *Store {
	return &Store{db: db}
}
```

- [ ] **Step 3: Create `admin.go` with Admin, Session types and all admin/session methods**

Create `/opt/ocserv_panel/backend/internal/store/admin.go` — copy the `Admin` struct, `Session` struct, and all methods currently in `store.go` that operate on admins and sessions:
- Types: `Admin`, `Session`
- Functions: `AdminCount`, `CreateFirstOwner`, `FindAdminByUsername`, `FindAdminBySessionTokenHash`, `CreateSession`, `RevokeSessionByTokenHash`, `ListAdmins`, `FindAdminByID`, `CreateAdmin`, `UpdateAdmin`, `DeleteAdmin`, `DisableAdmin`, `EnableAdmin`, `UpdateAdminPassword`, `CountOwners`
- Private helpers: `findAdmin`, `withRoles`, `scanAuditEvent`, `formatTime`, `parseTime`

The imports needed are:
```go
package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"ocserv_panel/backend/internal/rbac"
)
```

- [ ] **Step 4: Create `audit.go` with AuditEvent type and audit methods**

Create `/opt/ocserv_panel/backend/internal/store/audit.go`:

```go
package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"
)

// AuditEvent is a single recorded admin action.
type AuditEvent struct {
	ID           int64
	ActorID      *int64
	Action       string
	ResourceType string
	ResourceID   string
	Outcome      string
	Message      string
	Metadata     map[string]string
	CreatedAt    time.Time
}
```

Move the `RecentAuditEvents` and `RecordAudit` methods into this file. Move `scanAuditEvent` here (remove from `admin.go` if duplicated — keep only one copy).

- [ ] **Step 5: Create `history.go` with HistoricalSession, analytics, and StartHousekeeper**

Create `/opt/ocserv_panel/backend/internal/store/history.go`:

```go
package store

import (
	"context"
	"database/sql"
	"time"

	"ocserv_panel/backend/internal/audit"
	"github.com/rs/zerolog/log"
)

// HistoricalSession is a completed VPN session record.
type HistoricalSession struct {
	ID              int64
	Username        string
	RemoteAddress   string
	ConnectedAt     time.Time
	DisconnectedAt  *time.Time
	DurationSeconds int64
	RXBytes         int64
	TXBytes         int64
}

// AnalyticsSummary is aggregate statistics over all historical sessions.
type AnalyticsSummary struct {
	TotalSessions  int64
	SampleSessions int64
	TotalBytesIn   int64
	TotalBytesOut  int64
}
```

Move `HistoricalSessions`, `AnalyticsSummary`, `PurgeHistoricalBefore` methods here.

Add `StartHousekeeper` (replaces `app.HousekeepingScheduler`):

```go
// StartHousekeeper starts a background goroutine that purges sessions older than
// the configured retention window. It stops when ctx is cancelled.
func (s *Store) StartHousekeeper(ctx context.Context, interval time.Duration, recorder audit.Recorder, retentionFn func() int) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				retention := retentionFn()
				cutoff := time.Now().UTC().AddDate(0, 0, -retention)
				deleted, err := s.PurgeHistoricalBefore(ctx, cutoff)
				if err != nil {
					log.Error().Err(err).Msg("housekeeping purge failed")
					_ = recorder.Record(ctx, audit.Event{
						Action:       "housekeeping.purge",
						ResourceType: "history",
						Outcome:      "failure",
						Message:      "historical session purge failed",
						Metadata:     map[string]string{"error": err.Error()},
						CreatedAt:    time.Now().UTC(),
					})
					continue
				}
				_ = recorder.Record(ctx, audit.Event{
					Action:       "housekeeping.purge",
					ResourceType: "history",
					Outcome:      "success",
					Message:      "historical sessions purged",
					Metadata:     map[string]string{"deleted_rows": fmt.Sprintf("%d", deleted)},
					CreatedAt:    time.Now().UTC(),
				})
			}
		}
	}()
}
```

- [ ] **Step 6: Create `settings.go` with GetSetting and SetSetting**

Create `/opt/ocserv_panel/backend/internal/store/settings.go`:

```go
package store

import (
	"context"
	"database/sql"
	"errors"
)

// GetSetting retrieves a stored application setting by key.
// Returns ("", nil) if the key does not exist.
func (s *Store) GetSetting(ctx context.Context, key string) (string, error) {
	var value string
	err := s.db.QueryRowContext(ctx, `SELECT value FROM settings WHERE key = ?`, key).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return value, err
}

// SetSetting upserts an application setting.
func (s *Store) SetSetting(ctx context.Context, key string, value string) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO settings (key, value) VALUES (?, ?)
		 ON CONFLICT(key) DO UPDATE SET value = excluded.value`,
		key, value,
	)
	return err
}
```

- [ ] **Step 7: Delete `store.go`**

Remove the original file once all content is confirmed in the new files:

```bash
rm /opt/ocserv_panel/backend/internal/store/store.go
```

- [ ] **Step 8: Run tests to confirm zero regressions**

```bash
cd /opt/ocserv_panel && go test ./backend/internal/store/... -v 2>&1 | tail -30
```
Expected: same PASS count as Step 1. If compile errors appear, the split missed a method — add it to the correct file.

- [ ] **Step 9: Run full backend build**

```bash
cd /opt/ocserv_panel && go build ./backend/... 2>&1
```
Expected: no output (success). If errors appear they will be import issues — fix in the flagged file.

- [ ] **Step 10: Commit**

```bash
git add backend/internal/store/ && git commit -m "refactor(store): split store.go into focused files (db, admin, audit, history, settings)"
```

---

## Task 2: Extract `internal/validation/` from `ocserv/validate.go`

**Files:**
- Create: `backend/internal/validation/validation.go`
- Create: `backend/internal/validation/validation_test.go`
- Delete: `backend/internal/ocserv/validate.go`
- Delete: `backend/internal/ocserv/validate_test.go`
- Modify: `backend/internal/app/ocserv_service.go` (16 call sites)

- [ ] **Step 1: Verify validate tests pass before touching anything**

```bash
cd /opt/ocserv_panel && go test ./backend/internal/ocserv/... -run TestValidate -v 2>&1
```
Expected: all PASS.

- [ ] **Step 2: Create `internal/validation/validation.go`**

Create `/opt/ocserv_panel/backend/internal/validation/validation.go` — copy the entire contents of `ocserv/validate.go` and change the package declaration:

```go
package validation

import (
	"fmt"
	"net"
	"regexp"
	"strings"
)

var (
	usernameRe = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)
	groupRe    = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)
)

// Username returns an error if the username is invalid for ocserv.
func Username(username string) error {
	if username == "" {
		return fmt.Errorf("username is required")
	}
	if len(username) > 64 {
		return fmt.Errorf("username must be 64 characters or fewer")
	}
	if !usernameRe.MatchString(username) {
		return fmt.Errorf("username may only contain letters, digits, dots, underscores, and hyphens")
	}
	return nil
}

// Password returns an error if the password is invalid for ocserv.
func Password(password string) error {
	if password == "" {
		return fmt.Errorf("password is required")
	}
	if len(password) > 128 {
		return fmt.Errorf("password must be 128 characters or fewer")
	}
	if strings.ContainsAny(password, "\n\r") {
		return fmt.Errorf("password must not contain newlines")
	}
	return nil
}

// IPAddress returns an error if ip is not a valid non-CIDR IP address.
func IPAddress(ip string) error {
	if ip == "" {
		return fmt.Errorf("IP address is required")
	}
	if strings.HasPrefix(ip, "-") {
		return fmt.Errorf("IP address must not start with a dash")
	}
	if strings.Contains(ip, "/") {
		return fmt.Errorf("IP address must not include a CIDR prefix")
	}
	if net.ParseIP(ip) == nil {
		return fmt.Errorf("invalid IP address: %q", ip)
	}
	return nil
}

// Group returns an error if the group name is invalid for ocserv.
// An empty group is allowed (means default group).
func Group(group string) error {
	if group == "" {
		return nil
	}
	if len(group) < 2 || len(group) > 64 {
		return fmt.Errorf("group must be between 2 and 64 characters")
	}
	if !groupRe.MatchString(group) {
		return fmt.Errorf("group may only contain letters, digits, underscores, and hyphens")
	}
	return nil
}
```

NOTE: Copy the exact validation logic from `ocserv/validate.go` — do NOT write new logic. Open and read that file first with the Read tool to ensure exact parity.

- [ ] **Step 3: Create `internal/validation/validation_test.go`**

Copy the test file from `ocserv/validate_test.go` and update package and function names:

```go
package validation_test

import (
	"testing"

	"ocserv_panel/backend/internal/validation"
	"github.com/stretchr/testify/assert"
)

func TestUsername(t *testing.T) {
	assert.NoError(t, validation.Username("alice"))
	assert.NoError(t, validation.Username("bob-smith"))
	assert.Error(t, validation.Username(""))
	assert.Error(t, validation.Username("has space"))
	assert.Error(t, validation.Username(strings.Repeat("a", 65)))
}
// ... copy all test cases from validate_test.go with renamed function calls
```

Open `ocserv/validate_test.go` and copy the test cases verbatim, updating `ocserv.ValidateUsername(` → `validation.Username(` etc.

- [ ] **Step 4: Run new validation tests**

```bash
cd /opt/ocserv_panel && go test ./backend/internal/validation/... -v 2>&1
```
Expected: all PASS.

- [ ] **Step 5: Update `ocserv_service.go` to use `validation.` package**

In `/opt/ocserv_panel/backend/internal/app/ocserv_service.go`, replace every call to the old validate functions.

Add import: `"ocserv_panel/backend/internal/validation"`
Remove import of ocserv validate (the main ocserv package import stays for other types).

Replace (16 occurrences):
- `ocserv.ValidateUsername(` → `validation.Username(`
- `ocserv.ValidatePassword(` → `validation.Password(`
- `ocserv.ValidateIPAddress(` → `validation.IPAddress(`
- `ocserv.ValidateGroup(` → `validation.Group(`

Run:
```bash
grep -n "ocserv\.Validate" /opt/ocserv_panel/backend/internal/app/ocserv_service.go
```
Expected: zero matches after replacement.

- [ ] **Step 6: Delete the old validate files**

```bash
rm /opt/ocserv_panel/backend/internal/ocserv/validate.go
rm /opt/ocserv_panel/backend/internal/ocserv/validate_test.go
```

- [ ] **Step 7: Build and test**

```bash
cd /opt/ocserv_panel && go build ./backend/... && go test ./backend/internal/validation/... ./backend/internal/app/... 2>&1 | tail -20
```
Expected: build succeeds, all tests PASS.

- [ ] **Step 8: Commit**

```bash
git add backend/internal/validation/ backend/internal/ocserv/ backend/internal/app/ocserv_service.go && git commit -m "refactor(validation): extract validation package from ocserv, update call sites"
```

---

## Task 3: Split `internal/ocserv/` — `cmd/` sub-package

**Files:**
- Create: `backend/internal/ocserv/cmd/runner.go`
- Create: `backend/internal/ocserv/cmd/fake.go`
- Create: `backend/internal/ocserv/cmd/runner_test.go`
- Delete: `backend/internal/ocserv/runner.go`
- Delete: `backend/internal/ocserv/runner_test.go`
- Modify: `backend/internal/ocserv/occtl_adapter.go`
- Modify: `backend/internal/ocserv/passwd_adapter.go`
- Modify: `backend/internal/ocserv/readiness.go`
- Modify: `backend/internal/ocserv/network_inspector.go`
- Modify: `backend/cmd/ocserv-panel/main.go`

- [ ] **Step 1: Create `backend/internal/ocserv/cmd/` directory and `runner.go`**

Create `/opt/ocserv_panel/backend/internal/ocserv/cmd/runner.go`:

Copy the `CommandRunner` interface, `CommandResult` struct, and `OSCommandRunner` implementation from `ocserv/runner.go`, changing the package:

```go
package cmd

import (
	"bytes"
	"context"
	"io"
	"os/exec"
	"strings"
)

// CommandResult holds the output and exit code of a command invocation.
type CommandResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

// CommandRunner executes system commands and returns their output.
// Implementations must be safe for concurrent use.
type CommandRunner interface {
	Run(ctx context.Context, name string, args []string, stdin io.Reader) (CommandResult, error)
}

// OSCommandRunner executes real system commands via exec.Command.
type OSCommandRunner struct{}

// NewOSCommandRunner returns a CommandRunner that executes real system commands.
func NewOSCommandRunner() *OSCommandRunner { return &OSCommandRunner{} }

func (r *OSCommandRunner) Run(ctx context.Context, name string, args []string, stdin io.Reader) (CommandResult, error) {
	// Copy exact implementation from ocserv/runner.go
}
```

Open `backend/internal/ocserv/runner.go` with the Read tool and copy the `OSCommandRunner.Run` implementation exactly.

- [ ] **Step 2: Create `backend/internal/ocserv/cmd/fake.go`**

Create `/opt/ocserv_panel/backend/internal/ocserv/cmd/fake.go`:

```go
package cmd

import (
	"context"
	"fmt"
	"io"
	"strings"
)

// FakeCommandCall records one invocation for test assertions.
type FakeCommandCall struct {
	Name  string
	Args  []string
	Stdin string
}

// FakeCommandRunner is a CommandRunner test double.
// Configure Results before use; inspect Calls after.
type FakeCommandRunner struct {
	// Results maps "name arg1 arg2" or just "name" to the result to return.
	Results map[string]CommandResult
	Err     error
	Calls   []FakeCommandCall
}

func (f *FakeCommandRunner) Run(ctx context.Context, name string, args []string, stdin io.Reader) (CommandResult, error) {
	// Copy exact implementation from ocserv/runner.go FakeCommandRunner.Run
}
```

Open `backend/internal/ocserv/runner.go` and copy the `FakeCommandRunner` struct and `Run` method exactly.

- [ ] **Step 3: Create `backend/internal/ocserv/cmd/runner_test.go`**

Copy `backend/internal/ocserv/runner_test.go` and update the package declaration and imports:

```go
package cmd_test

import (
	"context"
	"strings"
	"testing"

	"ocserv_panel/backend/internal/ocserv/cmd"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)
```

Update all references from `ocserv.CommandRunner` to `cmd.CommandRunner`, `ocserv.FakeCommandRunner` to `cmd.FakeCommandRunner`, etc.

- [ ] **Step 4: Run new cmd package tests**

```bash
cd /opt/ocserv_panel && go test ./backend/internal/ocserv/cmd/... -v 2>&1
```
Expected: all PASS.

- [ ] **Step 5: Add a compatibility shim in `ocserv/runner.go` (temporary bridge)**

To avoid updating all callers in one shot, add a shim in `ocserv/runner.go` that re-exports the cmd types:

```go
package ocserv

import "ocserv_panel/backend/internal/ocserv/cmd"

// CommandRunner is an alias for cmd.CommandRunner.
// Deprecated: import ocserv_panel/backend/internal/ocserv/cmd directly.
type CommandRunner = cmd.CommandRunner
type CommandResult = cmd.CommandResult
type FakeCommandRunner = cmd.FakeCommandRunner
type FakeCommandCall = cmd.FakeCommandCall

// NewOSCommandRunner returns an OSCommandRunner that executes real system commands.
func NewOSCommandRunner() *cmd.OSCommandRunner { return cmd.NewOSCommandRunner() }
```

This lets the old callers compile while we migrate them in subsequent steps.

- [ ] **Step 6: Build to confirm shim works**

```bash
cd /opt/ocserv_panel && go build ./backend/... 2>&1
```
Expected: no output.

- [ ] **Step 7: Update direct callers to import `ocserv/cmd` directly**

Files to update:
- `backend/internal/ocserv/occtl_adapter.go` — field `runner ocserv.CommandRunner` → `runner cmd.CommandRunner`; add import `ocserv_panel/backend/internal/ocserv/cmd`
- `backend/internal/ocserv/passwd_adapter.go` — same
- `backend/internal/ocserv/readiness.go` — same  
- `backend/internal/ocserv/network_inspector.go` — same
- `backend/cmd/ocserv-panel/main.go` — `runner := ocserv.NewOSCommandRunner()` stays working via shim, leave for now

In each file, add `"ocserv_panel/backend/internal/ocserv/cmd"` to imports and change:
- `ocserv.CommandRunner` → `cmd.CommandRunner`
- `ocserv.FakeCommandRunner` → `cmd.FakeCommandRunner`
- `ocserv.CommandResult` → `cmd.CommandResult`

- [ ] **Step 8: Remove the shim from `ocserv/runner.go` and delete the old file**

Once all callers are updated, replace `ocserv/runner.go` content with nothing (or remove it). The `ocserv/runner_test.go` also gets removed (tests are now in `cmd/`).

```bash
rm /opt/ocserv_panel/backend/internal/ocserv/runner.go
rm /opt/ocserv_panel/backend/internal/ocserv/runner_test.go
```

- [ ] **Step 9: Build and test**

```bash
cd /opt/ocserv_panel && go build ./backend/... && go test ./backend/... 2>&1 | grep -E "FAIL|ok" | head -30
```
Expected: all packages `ok`, zero `FAIL`.

- [ ] **Step 10: Commit**

```bash
git add backend/internal/ocserv/ backend/cmd/ && git commit -m "refactor(ocserv): extract cmd sub-package for CommandRunner"
```

---

## Task 4: Split `internal/ocserv/` — `parse/` sub-package

**Files:**
- Create: `backend/internal/ocserv/parse/sessions.go`, `users.go`, `user.go`, `bans.go`, `routes.go`, `status.go`, `cidr.go` (+ matching `_test.go` files)
- Delete: all 14 source files from `ocserv/` root that are being moved
- Modify: `backend/internal/ocserv/occtl_adapter.go` (import parse)

- [ ] **Step 1: Verify parser tests pass before touching anything**

```bash
cd /opt/ocserv_panel && go test ./backend/internal/ocserv/... -run "TestParse\|TestCIDR\|TestSessions\|TestUsers\|TestBans\|TestRoutes\|TestStatus" -v 2>&1 | tail -30
```
Expected: all PASS.

- [ ] **Step 2: Create the parse sub-package directory and move each parser**

For each of the 7 source files, create the corresponding file in `backend/internal/ocserv/parse/` with `package parse` instead of `package ocserv`. The function signatures remain identical.

Files to move (read each with the Read tool before writing):
- `occtl_sessions_parser.go` → `parse/sessions.go`
- `occtl_users_parser.go` → `parse/users.go`
- `occtl_user_parser.go` → `parse/user.go`
- `occtl_bans_parser.go` → `parse/bans.go`
- `occtl_routes_parser.go` → `parse/routes.go`
- `occtl_status_parser.go` → `parse/status.go`
- `cidr_convert.go` → `parse/cidr.go`

For each file:
1. Read the original with the Read tool.
2. Write to the new path with `package parse` as the first line.
3. Keep all imports and function bodies identical.

For each test file, follow the same process, using `package parse_test` and updating the import to `"ocserv_panel/backend/internal/ocserv/parse"`.

- [ ] **Step 3: Add temporary re-export shim in `ocserv/`**

In the root `ocserv/` package, create `ocserv/parsers.go` that re-exports the parse functions:

```go
package ocserv

import "ocserv_panel/backend/internal/ocserv/parse"

// ParseSessionsList parses occtl -j show sessions output.
// Deprecated: import ocserv_panel/backend/internal/ocserv/parse directly.
var ParseSessionsList = parse.ParseSessionsList
var ParseUsersList = parse.ParseUsersList
var ParseUser = parse.ParseUser
var ParseBansList = parse.ParseBansList
var ParseRoutesList = parse.ParseRoutesList
var ParseStatus = parse.ParseStatus
var CIDRToMask = parse.CIDRToMask
```

(The exact function names must match what is in the original parser files — read them first.)

- [ ] **Step 4: Run tests to confirm shim compiles and passes**

```bash
cd /opt/ocserv_panel && go build ./backend/... && go test ./backend/internal/ocserv/... 2>&1 | tail -20
```
Expected: all pass.

- [ ] **Step 5: Update `occtl_adapter.go` to import `parse` directly**

In `backend/internal/ocserv/occtl_adapter.go`:
- Add import `"ocserv_panel/backend/internal/ocserv/parse"`
- Replace all `ParseSessionsList(` → `parse.ParseSessionsList(`, etc.

Run:
```bash
grep -n "ocserv\.Parse\|ParseSessions\|ParseUsers\|ParseBans\|ParseRoutes\|ParseStatus\|CIDRToMask" /opt/ocserv_panel/backend/internal/ocserv/occtl_adapter.go
```
Expected: only `parse.` prefixed calls remain.

- [ ] **Step 6: Remove the shim and delete old parser source files**

```bash
rm /opt/ocserv_panel/backend/internal/ocserv/parsers.go
rm /opt/ocserv_panel/backend/internal/ocserv/occtl_sessions_parser.go
rm /opt/ocserv_panel/backend/internal/ocserv/occtl_sessions_parser_test.go
rm /opt/ocserv_panel/backend/internal/ocserv/occtl_users_parser.go
rm /opt/ocserv_panel/backend/internal/ocserv/occtl_users_parser_test.go
rm /opt/ocserv_panel/backend/internal/ocserv/occtl_user_parser.go
rm /opt/ocserv_panel/backend/internal/ocserv/occtl_user_parser_test.go
rm /opt/ocserv_panel/backend/internal/ocserv/occtl_bans_parser.go
rm /opt/ocserv_panel/backend/internal/ocserv/occtl_bans_parser_test.go
rm /opt/ocserv_panel/backend/internal/ocserv/occtl_routes_parser.go
rm /opt/ocserv_panel/backend/internal/ocserv/occtl_routes_parser_test.go
rm /opt/ocserv_panel/backend/internal/ocserv/occtl_status_parser.go
rm /opt/ocserv_panel/backend/internal/ocserv/occtl_status_parser_test.go
rm /opt/ocserv_panel/backend/internal/ocserv/cidr_convert.go
rm /opt/ocserv_panel/backend/internal/ocserv/cidr_convert_test.go
```

- [ ] **Step 7: Build and test**

```bash
cd /opt/ocserv_panel && go build ./backend/... && go test ./backend/... 2>&1 | grep -E "FAIL|ok"
```
Expected: all `ok`, zero `FAIL`.

- [ ] **Step 8: Commit**

```bash
git add backend/internal/ocserv/ && git commit -m "refactor(ocserv): extract parse sub-package for occtl output parsers"
```

---

## Task 5: Split `internal/ocserv/` — `config/` and `service/` sub-packages

**Files:**
- Create: `backend/internal/ocserv/config/file.go`, `validator.go`, `reload.go`, `restart.go` (+ tests)
- Create: `backend/internal/ocserv/service/manager.go`, `systemd.go`, `fake.go` (+ tests)
- Delete: corresponding root `ocserv/` files
- Modify: `backend/internal/ocserv/readiness.go` (imports)
- Modify: `backend/internal/ocserv/network_inspector.go` (imports)
- Modify: `backend/internal/app/config_management_service.go` (imports)
- Modify: `backend/cmd/ocserv-panel/main.go` (imports)

- [ ] **Step 1: Verify all affected tests pass**

```bash
cd /opt/ocserv_panel && go test ./backend/internal/ocserv/... -v 2>&1 | grep -E "PASS|FAIL|ok" | head -20
```

- [ ] **Step 2: Create `config/` sub-package**

Create `backend/internal/ocserv/config/` directory. For each file, read the original with the Read tool, then write to the new path with `package config`:

- `config_file.go` → `config/file.go`
- `config_validator.go` → `config/validator.go`
- `config_reload.go` → `config/reload.go`
- `config_restart_detector.go` → `config/restart.go`

Each test file mirrors the source file with `package config_test` and import `"ocserv_panel/backend/internal/ocserv/config"`.

Note: `config_reload.go` uses `CommandRunner` — update its import to `"ocserv_panel/backend/internal/ocserv/cmd"`.

- [ ] **Step 3: Create `service/` sub-package**

Create `backend/internal/ocserv/service/` directory:

- `service.go` → `service/manager.go` (package `service`)
- `service_systemd.go` → `service/systemd.go` (package `service`)
- `service_fake.go` → `service/fake.go` (package `service`)
- `service_systemd_test.go` → `service/systemd_test.go` (package `service_test`)

`systemd.go` uses `CommandRunner` — update import to `"ocserv_panel/backend/internal/ocserv/cmd"`.

- [ ] **Step 4: Add re-export shims in `ocserv/` root**

Create `ocserv/config_compat.go`:

```go
package ocserv

import (
	"ocserv_panel/backend/internal/ocserv/config"
	"ocserv_panel/backend/internal/ocserv/service"
)

// Deprecated aliases — import the sub-packages directly.
type ConfigFileManager = config.ConfigFileManager
type ConfigValidator = config.ConfigValidator
type ConfigReloader = config.ConfigReloader
type RestartDetector = config.RestartDetector
type ServiceManager = service.Manager
type ServiceStatus = service.Status
type SystemdManager = service.SystemdManager
type FakeServiceManager = service.FakeManager

var NewConfigFileManager = config.NewConfigFileManager
var NewConfigValidator = config.NewConfigValidator
var NewConfigReloader = config.NewConfigReloader
var NewRestartDetector = config.NewRestartDetector
var NewSystemdManager = service.NewSystemdManager
```

(Exact names must match the originals — read the source files to verify.)

- [ ] **Step 5: Build to confirm shims work**

```bash
cd /opt/ocserv_panel && go build ./backend/... 2>&1
```

- [ ] **Step 6: Update callers to import sub-packages directly**

Files to update:
- `backend/internal/app/config_management_service.go`: replace `ocserv.ConfigFileManager`, `ocserv.ConfigValidator`, `ocserv.ConfigReloader`, `ocserv.RestartDetector` with `config.ConfigFileManager` etc.
- `backend/internal/app/ocserv_service.go`: update `ocserv.ServiceManager` → `service.Manager`, `ocserv.NewSystemdManager` → `service.NewSystemdManager` (or import service directly)
- `backend/cmd/ocserv-panel/main.go`: update `ocserv.NewConfigFileManager(...)`, `ocserv.NewConfigReloader(...)`, `ocserv.NewSystemdManager(...)`, `ocserv.NewConfigValidator(...)` to import from sub-packages

- [ ] **Step 7: Remove shims, delete old source files**

```bash
rm /opt/ocserv_panel/backend/internal/ocserv/config_compat.go
rm /opt/ocserv_panel/backend/internal/ocserv/config_file.go
rm /opt/ocserv_panel/backend/internal/ocserv/config_file_test.go
rm /opt/ocserv_panel/backend/internal/ocserv/config_validator.go
rm /opt/ocserv_panel/backend/internal/ocserv/config_validator_test.go
rm /opt/ocserv_panel/backend/internal/ocserv/config_reload.go
rm /opt/ocserv_panel/backend/internal/ocserv/config_reload_test.go
rm /opt/ocserv_panel/backend/internal/ocserv/config_restart_detector.go
rm /opt/ocserv_panel/backend/internal/ocserv/config_restart_detector_test.go
rm /opt/ocserv_panel/backend/internal/ocserv/service.go
rm /opt/ocserv_panel/backend/internal/ocserv/service_systemd.go
rm /opt/ocserv_panel/backend/internal/ocserv/service_systemd_test.go
rm /opt/ocserv_panel/backend/internal/ocserv/service_fake.go
```

- [ ] **Step 8: Build and test**

```bash
cd /opt/ocserv_panel && go build ./backend/... && go test ./backend/... 2>&1 | grep -E "FAIL|ok"
```
Expected: all `ok`.

- [ ] **Step 9: Commit**

```bash
git add backend/internal/ocserv/ backend/internal/app/ backend/cmd/ && git commit -m "refactor(ocserv): extract config and service sub-packages"
```

---

## Task 6: Add RBAC middleware and collapse thin app services

**Files:**
- Create: `backend/internal/server/middleware.go`
- Modify: `backend/internal/server/server.go` (remove 3 service fields, wire middleware)
- Modify: `backend/internal/server/health_handlers.go` (call directly, no services)
- Delete: `backend/internal/app/monitoring_service.go` + test
- Delete: `backend/internal/app/log_tail_service.go` + test
- Delete: `backend/internal/app/health_service.go` + test
- Delete: `backend/internal/app/housekeeping.go` + test
- Modify: `backend/cmd/ocserv-panel/main.go` (remove 3 services, wire StartHousekeeper)

- [ ] **Step 1: Check what types SystemHealthResult references that must stay**

```bash
grep -rn "SystemHealthResult\|VPNServerHealth\|PanelHealth\|LogsHealth\|StorageHealth\|LogTailResult\|MonitoringOverview" /opt/ocserv_panel/backend/internal/ | grep -v "_test.go" | grep -v "health_service\|monitoring_service\|log_tail_service"
```
These types are returned from handlers and used by the frontend — they must stay but can move to a new file or to the server package.

- [ ] **Step 2: Create `backend/internal/server/middleware.go`**

Create `/opt/ocserv_panel/backend/internal/server/middleware.go`:

```go
package server

import (
	"context"
	"net/http"

	"ocserv_panel/backend/internal/auth"
	"ocserv_panel/backend/internal/rbac"
)

type contextKey string

const principalKey contextKey = "principal"

// requirePermission returns middleware that validates the session cookie,
// checks the given permission, and injects the principal into the context.
// On failure it writes 401/403 and stops the chain.
func (a *App) requirePermission(perm rbac.Permission) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			principal, ok := a.authenticate(w, r)
			if !ok {
				return
			}
			roles := make([]rbac.Role, 0, len(principal.Roles))
			for _, role := range principal.Roles {
				roles = append(roles, rbac.Role(role))
			}
			if !a.authorizer.Allowed(roles, perm) {
				writeError(w, http.StatusForbidden, "permission denied")
				return
			}
			ctx := context.WithValue(r.Context(), principalKey, principal)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// principalFromContext retrieves the injected principal. Panics if missing —
// only call inside handlers protected by requirePermission.
func principalFromContext(ctx context.Context) auth.Principal {
	p, _ := ctx.Value(principalKey).(auth.Principal)
	return p
}
```

- [ ] **Step 3: Move `LogFileReader` interface before deleting `log_tail_service.go`**

The `LogFileReader` interface and `OSLogFileReader` / `FakeLogFileReader` implementations currently live in `log_tail_service.go`. Before deleting that file, move them to `backend/internal/logs/reader.go` (it already has a `logs` package). Add `package logs` header and move the three types.

Also move `tailLines()` private function from `log_tail_service.go` to a new `backend/internal/logs/tail.go` file. The `handleLogTail` handler in `health_handlers.go` will call `logs.TailFile(reader, path, lines)`.

Update `OSLogFileReader` and `FakeLogFileReader` to use `package logs`. Then in `main.go`, change `app.OSLogFileReader{}` → `logs.OSLogFileReader{}`.

- [ ] **Step 4: Move response types out of `health_service.go` before deleting it**

The types `SystemHealthResult`, `VPNServerHealth`, `PanelHealth`, `LogsHealth`, `StorageItemHealth`, `StorageHealth` are used by the server layer. Create `backend/internal/server/health_types.go`:

```go
package server

// SystemHealthResult is the payload returned by GET /api/health/system.
type SystemHealthResult struct {
	VPNServer VPNServerHealth `json:"vpnServer"`
	Panel     PanelHealth     `json:"panel"`
	Logs      LogsHealth      `json:"logs"`
	Storage   StorageHealth   `json:"storage"`
}
// ... copy all 6 type definitions from health_service.go verbatim (read the file first)
// ... copy the 6 storage size threshold constants as well
```

Also move `LogTailResult` (currently in `log_tail_service.go`) to `backend/internal/server/log_types.go`:
```go
package server

// LogTailResult is the payload returned by GET /api/logs/tail.
type LogTailResult struct {
	// ... copy from log_tail_service.go
}
```

- [ ] **Step 4: Rewrite `health_handlers.go` to call health logic directly**

The `SystemHealth` method in `health_service.go` aggregates 4 sub-systems. After deletion, move those private build methods (`buildVPNHealth`, `buildPanelHealth`, `buildLogsHealth`, `buildStorageHealth`) to `health_handlers.go` as package-level functions.

Read `health_service.go` fully to capture all 4 build functions and their dependencies. Rewrite `handleSystemHealth`:

```go
func (a *App) handleSystemHealth(w http.ResponseWriter, r *http.Request) {
	if !allowMethod(w, r, http.MethodGet) {
		return
	}
	principal, ok := a.authenticate(w, r)
	if !ok {
		return
	}
	roles := make([]rbac.Role, 0, len(principal.Roles))
	for _, role := range principal.Roles {
		roles = append(roles, rbac.Role(role))
	}
	if !a.authorizer.Allowed(roles, rbac.PermissionReadDashboard) {
		writeError(w, http.StatusForbidden, "permission denied")
		return
	}
	cfg := a.cfgFn()
	result := SystemHealthResult{
		VPNServer: buildVPNHealth(r.Context(), a.readinessChecker),
		Panel:     buildPanelHealth(r.Context(), cfg, a.adminStore),
		Logs:      buildLogsHealth(cfg),
		Storage:   buildStorageHealth(cfg, a.tsdbPath),
	}
	writeJSON(w, http.StatusOK, result)
}
```

The `App` struct needs `readinessChecker`, `adminStore`, `cfgFn`, and `tsdbPath` fields added (or reuse existing ones if already present). Read `server.go` App struct definition to see what's already there.

Rewrite `handleLogTail`:

```go
func (a *App) handleLogTail(w http.ResponseWriter, r *http.Request) {
	if !allowMethod(w, r, http.MethodGet) {
		return
	}
	principal, ok := a.authenticate(w, r)
	if !ok {
		return
	}
	roles := make([]rbac.Role, 0, len(principal.Roles))
	for _, role := range principal.Roles {
		roles = append(roles, rbac.Role(role))
	}
	if !a.authorizer.Allowed(roles, rbac.PermissionReadDashboard) {
		writeError(w, http.StatusForbidden, "permission denied")
		return
	}
	source := r.URL.Query().Get("source")
	if source != "ocserv" && source != "panel" {
		writeError(w, http.StatusBadRequest, "source must be ocserv or panel")
		return
	}
	lines := 25
	if raw := r.URL.Query().Get("lines"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 {
			writeError(w, http.StatusBadRequest, "lines must be a positive integer")
			return
		}
		lines = n
	}
	result, err := a.logFileReader.TailLog(r.Context(), source, lines)
	if err != nil {
		a.writeAppError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}
```

The `App` struct gets a `logFileReader` field (the reader used by `LogTailService`). Read `log_tail_service.go` to see the `LogFileReader` interface and `OSLogFileReader` implementation.

- [ ] **Step 5: Rewrite monitoring handlers to call `metrics.Store` directly**

In `server.go`, the three monitoring handlers currently call `a.monitoringService`. After deletion they call the store directly. The `MonitoringOverview` and catalog types must be accessible — move them to `backend/internal/server/monitoring_types.go`:

```go
package server

import "ocserv_panel/backend/internal/metrics"

// MonitoringOverview is the dashboard-ready snapshot returned by GET /api/monitoring/overview.
type MonitoringOverview struct {
	Window      string                     `json:"window"`
	GeneratedAt time.Time                  `json:"generatedAt"`
	Catalog     []metrics.MetricDefinition `json:"catalog"`
	Series      []metrics.Series           `json:"series"`
}
```

Rewrite the three handlers in `server.go` to call `a.metricStore` and `a.metricCatalog` directly:

```go
func (a *App) handleMonitoringCatalog(w http.ResponseWriter, r *http.Request) {
	if !allowMethod(w, r, http.MethodGet) {
		return
	}
	principal, ok := a.authenticate(w, r)
	if !ok {
		return
	}
	roles := make([]rbac.Role, 0, len(principal.Roles))
	for _, role := range principal.Roles {
		roles = append(roles, rbac.Role(role))
	}
	if !a.authorizer.Allowed(roles, rbac.PermissionReadDashboard) {
		writeError(w, http.StatusForbidden, "permission denied")
		return
	}
	writeJSON(w, http.StatusOK, a.metricCatalog.List())
}
```

Repeat for `handleMonitoringOverview` and `handleMonitoringSeries` — read `monitoring_service.go` to see the full query logic and replicate it directly in the handlers.

Add `metricStore metrics.Store` and `metricCatalog *metrics.Catalog` to the `App` struct and pass them in `NewAppRouter`.

- [ ] **Step 6: Update `NewAppRouter` signature**

Remove from `NewAppRouter` parameters:
- `monitoringService *app.MonitoringService`
- `healthService *app.HealthService`
- `logTailService *app.LogTailService`

Add:
- `metricStore metrics.Store`
- `metricCatalog *metrics.Catalog`
- `readinessChecker *ocserv.ReadinessChecker`
- `logFileReader app.LogFileReader` (the interface from log_tail_service — move to a shared location or inline)
- `cfgFn func() config.Config`
- `tsdbPath string`
- `adminStore interface{ AdminCount(ctx context.Context) (int64, error) }`

Update the `App` struct to store these fields.

- [ ] **Step 7: Update `main.go` to match new `NewAppRouter` signature**

In `backend/cmd/ocserv-panel/main.go`:
- Remove construction of `monitoringService`, `healthService`, `logTailService`
- Remove `housekeepingScheduler` construction and `go housekeepingScheduler.Run(ctx)`
- Add `persistence.StartHousekeeper(ctx, 24*time.Hour, recorder, func() int { ... })` call
- Pass `metricStore`, `catalog` (get from `monitoringService.SetMetricState` calls — now done directly on the catalog), `readinessChecker`, etc. to `NewAppRouter`
- The `monitoringService.SetMetricState(...)` calls now become `metricCatalog.SetState(...)` calls directly

- [ ] **Step 8: Build after all wiring is done**

```bash
cd /opt/ocserv_panel && go build ./backend/... 2>&1
```
Fix any compile errors before proceeding.

- [ ] **Step 9: Delete the four thin service files**

```bash
rm /opt/ocserv_panel/backend/internal/app/monitoring_service.go
rm /opt/ocserv_panel/backend/internal/app/monitoring_service_test.go
rm /opt/ocserv_panel/backend/internal/app/log_tail_service.go
rm /opt/ocserv_panel/backend/internal/app/log_tail_service_test.go
rm /opt/ocserv_panel/backend/internal/app/health_service.go
rm /opt/ocserv_panel/backend/internal/app/health_service_test.go
rm /opt/ocserv_panel/backend/internal/app/housekeeping.go
rm /opt/ocserv_panel/backend/internal/app/housekeeping_test.go
```

- [ ] **Step 10: Run full test suite**

```bash
cd /opt/ocserv_panel && go test ./backend/... 2>&1 | grep -E "FAIL|ok"
```
Expected: all `ok`, zero `FAIL`.

- [ ] **Step 11: Commit**

```bash
git add backend/ && git commit -m "refactor(server): add RBAC middleware, collapse monitoring/health/log/housekeeping thin services"
```

---

## Task 7: Split `frontend/src/lib/api.ts` into domain modules

**Files:**
- Create: `frontend/src/lib/api/` directory with 9 domain files + `index.ts`
- Delete: `frontend/src/lib/api.ts`

- [ ] **Step 1: Verify frontend type check passes before changes**

```bash
cd /opt/ocserv_panel/frontend && pnpm check 2>&1 | tail -20
```
Expected: zero errors.

- [ ] **Step 2: Create `lib/api/` directory structure**

The shared `apiFetch` function and base URL belong in a private `_client.ts` file imported by each domain module.

Create `/opt/ocserv_panel/frontend/src/lib/api/_client.ts`:

```typescript
const BASE = '';

export async function apiFetch<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(BASE + path, {
    credentials: 'include',
    headers: { 'Content-Type': 'application/json', ...(init?.headers ?? {}) },
    ...init,
  });
  if (!res.ok) {
    const text = await res.text().catch(() => '');
    throw new Error(`${res.status}: ${text || res.statusText}`);
  }
  if (res.status === 204 || res.headers.get('content-length') === '0') return undefined as T;
  return res.json() as Promise<T>;
}
```

(Read the current `apiFetch` implementation in `api.ts` and copy it exactly.)

- [ ] **Step 3: Create `lib/api/auth.ts`**

Create `/opt/ocserv_panel/frontend/src/lib/api/auth.ts`:

```typescript
import { apiFetch } from './_client';

export type SetupStatus = { required: boolean; };
export type Principal = { id?: string | number; username: string; roles?: string[]; role?: string; };
export type SessionSummary = { id?: string; createdAt?: string; expiresAt?: string; };
export type AuthSummary = { principal?: Principal; session?: SessionSummary; user?: Principal; roles?: string[]; };
export type WebUISettings = { cookieSecure: boolean; httpLoginAllowed: boolean; requestSecure: boolean; transport: 'http' | 'https'; warning: string; };

export function getSetupStatus(): Promise<SetupStatus> {
  return apiFetch<SetupStatus>('/api/setup/status');
}
export function getWebUISettings(): Promise<WebUISettings> {
  return apiFetch<WebUISettings>('/api/settings/webui');
}
export function setupOwner(username: string, password: string): Promise<AuthSummary> {
  return apiFetch<AuthSummary>('/api/setup/owner', { method: 'POST', body: JSON.stringify({ username, password }) });
}
export function login(username: string, password: string): Promise<AuthSummary> {
  return apiFetch<AuthSummary>('/api/auth/login', { method: 'POST', body: JSON.stringify({ username, password }) });
}
export function logout(): Promise<void> {
  return apiFetch<void>('/api/auth/logout', { method: 'POST' });
}
export function getMe(): Promise<AuthSummary> {
  return apiFetch<AuthSummary>('/api/auth/me');
}
export function principalFromAuth(auth: AuthSummary | null): Principal | null {
  if (!auth) return null;
  return auth.principal ?? auth.user ?? null;
}
export function rolesFromPrincipal(principal: Principal | null | undefined, fallback: string[] = []): string[] {
  if (!principal) return fallback;
  return principal.roles ?? (principal.role ? [principal.role] : fallback);
}
export function hasOwnerRole(principal: Principal | null | undefined): boolean {
  return rolesFromPrincipal(principal).includes('owner');
}
```

(Read the current api.ts functions for exact bodies — copy verbatim.)

- [ ] **Step 4: Create remaining domain files**

Create each file by reading the corresponding section of `api.ts` and extracting the relevant types and functions:

**`lib/api/monitoring.ts`** — types: `MonitoringMetricState`, `MonitoringMetric`, `MonitoringPoint`, `MonitoringSeries`, `MonitoringCatalog`, `MonitoringOverview`, `MonitoringWindow`, `DashboardSummary`, `UserSessionCount`, `ActiveUserSessionsResult`; functions: `getDashboardSummary`, `getDashboardActiveSessions`, `getMonitoringOverview`

**`lib/api/sessions.ts`** — types: `OcservSession`, `OcservSessionDetail`, `OcservStatus`, `ServerStatus`, `ConnectedUser`, `UserConnection`, `ConnectionEvent`, `UserRoute`, `BannedIP`, `BanPointEntry`, `OcservReadiness`; functions: `getOcservSessions`, `disconnectOcservSession`, `getOcservSessionDetail`, `terminateOcservSession`, `getOcservReadiness`, `getServerStatus`, `getConnectedUsers`, `getUserConnections`, `getConnectionById`, `getConnectionEvents`, `getUserRoutes`, `getBannedIPs`, `getBanPoints`, `disconnectUser`, `unbanIP`

**`lib/api/users.ts`** — types: `AdminUser`, `VpnUser`, `BulkResult`; functions: `getAdminUsers`, `createAdminUser`, `updateAdminUser`, `deleteAdminUser`, `disableAdminUser`, `enableAdminUser`, `resetAdminPassword`, `getVpnUsers`, `createVpnUser`, `updateVpnUserGroup`, `deleteVpnUser`, `resetVpnPassword`, `lockVpnUser`, `unlockVpnUser`, `bulkDeleteAdminUsers`, `bulkDisableAdminUsers`, `bulkEnableAdminUsers`, `bulkDeleteVpnUsers`, `bulkLockVpnUsers`, `bulkUnlockVpnUsers`

**`lib/api/history.ts`** — types: `HistoricalSession`, `HistoryAnalytics`; functions: `getHistoryAnalytics`, `purgeHistory`

**`lib/api/settings.ts`** — types: `Setting`, `SettingsResponse`, `SettingDef`, `SettingsSection`, `SettingsSectionsResponse`, `RestartStatus`, `ConfigStatus`; functions: `getSettings`, `updateSetting`, `getSettingsSections`, `updateSettingByKey`, `getRestartStatus`, `triggerRestart`

**`lib/api/config.ts`** — types: `ConfigIssue`, `ConfigMetadata`, `ConfigReadResult`, `ConfigValidateResult`, `ConfigDiffResult`, `ConfigApplyResult`, `ConfigBackupInfo`, `ConfigBackupContent`, `ConfigRollbackResult`, `NetworkPrerequisiteCheck`, `NetworkPrerequisites`; functions: `getConfig`, `validateConfig`, `diffConfig`, `applyConfig`, `getConfigBackups`, `getConfigBackup`, `diffConfigBackup`, `rollbackConfig`, `getNetworkPrerequisites`

**`lib/api/audit.ts`** — types: `AuditEvent`, `AuditLogLine`, `AuditLogResponse`, `AuditLogSource`, `AuditLogSourcesResponse`, `AuditLogParams`; functions: `getAuditLogs`, `getAuditLogSources`, `exportAuditLogs`

**`lib/api/health.ts`** — types: `VPNServerHealth`, `PanelHealth`, `LogsHealth`, `StorageItemHealth`, `StorageHealth`, `SystemHealth`, `LogTailResult`; functions: `getSystemHealth`, `getLogTail`

- [ ] **Step 5: Create `lib/api/index.ts` that re-exports everything**

Create `/opt/ocserv_panel/frontend/src/lib/api/index.ts`:

```typescript
export * from './auth';
export * from './monitoring';
export * from './sessions';
export * from './users';
export * from './history';
export * from './settings';
export * from './config';
export * from './audit';
export * from './health';
```

- [ ] **Step 6: Update SvelteKit alias to point to the new index**

Check `frontend/svelte.config.js` or `vite.config.ts` for the `$lib` alias. The default is `src/lib` so `$lib/api` will resolve to `src/lib/api/index.ts` automatically — no config change needed if the `index.ts` exists.

Verify by running:
```bash
cd /opt/ocserv_panel/frontend && pnpm check 2>&1 | head -30
```

- [ ] **Step 7: Delete the old `api.ts`**

```bash
rm /opt/ocserv_panel/frontend/src/lib/api.ts
```

- [ ] **Step 8: Final type check**

```bash
cd /opt/ocserv_panel/frontend && pnpm check 2>&1 | tail -20
```
Expected: zero errors. If type errors appear, a type was missed — add it to the correct domain file.

- [ ] **Step 9: Commit**

```bash
git add frontend/src/lib/api/ frontend/src/lib/api.ts && git commit -m "refactor(frontend): split api.ts into domain modules with re-export index"
```

---

## Task 8: Split `routes/+page.svelte` into section components

**Files:**
- Create: `frontend/src/routes/DashboardHealth.svelte`
- Create: `frontend/src/routes/DashboardMetrics.svelte`
- Create: `frontend/src/routes/DashboardSessions.svelte`
- Create: `frontend/src/routes/DashboardActivity.svelte`
- Modify: `frontend/src/routes/+page.svelte`

- [ ] **Step 1: Read the full `+page.svelte` before any changes**

```bash
wc -l /opt/ocserv_panel/frontend/src/routes/+page.svelte
```

Use the Read tool to read the entire file (782 lines). Note which lines correspond to each section:
- Health cards (HealthSummaryStrip + HealthCard grid)
- Metrics charts (WindowSelector + MetricChart grid)
- Sessions section (DonutChart + sessions table)
- Activity / logs section (audit table + log tail tabs)

- [ ] **Step 2: Create `DashboardHealth.svelte`**

Extract the health section. Props accepted via Svelte 5 `$props()`:

```svelte
<script lang="ts">
  import HealthCard from '$lib/components/ui/HealthCard.svelte';
  import HealthSummaryStrip from '$lib/components/ui/HealthSummaryStrip.svelte';
  import type { HealthItem } from '$lib/components/ui/HealthCard.svelte';
  import type { SystemHealth } from '$lib/api/health';

  const {
    health,
    healthLoading,
    healthCheckedAt,
    allHealthItems,
    vpnServerItems,
    panelItems,
    storageItems,
  }: {
    health: SystemHealth | null;
    healthLoading: boolean;
    healthCheckedAt: Date | null;
    allHealthItems: HealthItem[];
    vpnServerItems: HealthItem[];
    panelItems: HealthItem[];
    storageItems: HealthItem[];
  } = $props();
</script>
```

Copy the relevant HTML from `+page.svelte` for the HealthSummaryStrip and HealthCard grid.

- [ ] **Step 3: Create `DashboardMetrics.svelte`**

Extract the metrics charts section. Props:

```svelte
<script lang="ts">
  import MetricChart from '$lib/components/ui/MetricChart.svelte';
  import WindowSelector from '$lib/components/ui/WindowSelector.svelte';
  import type { MonitoringOverview, MonitoringWindow } from '$lib/api/monitoring';

  const {
    overview,
    selectedWindow,
    onWindowChange,
  }: {
    overview: MonitoringOverview | null;
    selectedWindow: MonitoringWindow;
    onWindowChange: (w: MonitoringWindow) => void;
  } = $props();
</script>
```

Copy the WindowSelector and MetricChart grid HTML.

- [ ] **Step 4: Create `DashboardSessions.svelte`**

Extract the active sessions section. Props:

```svelte
<script lang="ts">
  import DonutChart from '$lib/components/ui/DonutChart.svelte';
  import PaginatedTable from '$lib/components/ui/PaginatedTable.svelte';
  import type { ActiveUserSessionsResult } from '$lib/api/monitoring';

  const {
    sessionData,
    sessionLoading,
  }: {
    sessionData: ActiveUserSessionsResult | null;
    sessionLoading: boolean;
  } = $props();
</script>
```

Copy the DonutChart and sessions table HTML.

- [ ] **Step 5: Create `DashboardActivity.svelte`**

Extract the audit events + log tail section. Props:

```svelte
<script lang="ts">
  import type { DashboardSummary } from '$lib/api/monitoring';
  import type { LogTailResult } from '$lib/api/health';

  const {
    summary,
    ocservLog,
    panelLog,
    logLoading,
    logError,
    activeLogTab,
    onTabChange,
    onRefreshLog,
  }: {
    summary: DashboardSummary | null;
    ocservLog: LogTailResult | null;
    panelLog: LogTailResult | null;
    logLoading: boolean;
    logError: string;
    activeLogTab: 'ocserv' | 'panel';
    onTabChange: (tab: 'ocserv' | 'panel') => void;
    onRefreshLog: (tab: 'ocserv' | 'panel') => void;
  } = $props();
</script>
```

Copy the audit table and log tail tab HTML.

- [ ] **Step 6: Reduce `+page.svelte` to a thin orchestrator**

Replace the HTML sections with component calls:

```svelte
{#if loading}
  <!-- loading spinner -->
{:else if setupRequired}
  <!-- setup banner -->
{:else if !principal}
  <!-- login prompt -->
{:else if summary}
  <div class="mb-6">
    <h1 class="text-xl font-bold text-slate-700">Dashboard</h1>
    <p class="text-sm text-slate-500 mt-1">Local ocserv administration overview.</p>
  </div>

  <DashboardHealth
    {health} {healthLoading} {healthCheckedAt}
    {allHealthItems} {vpnServerItems} {panelItems} {storageItems}
  />

  <DashboardMetrics
    {overview} {selectedWindow}
    onWindowChange={(w) => { selectedWindow = w; loadOverview(); }}
  />

  <DashboardSessions {sessionData} {sessionLoading} />

  <DashboardActivity
    {summary} {ocservLog} {panelLog} {logLoading} {logError}
    {activeLogTab}
    onTabChange={(tab) => activeLogTab = tab}
    onRefreshLog={loadLogTab}
  />
{:else}
  <!-- error state -->
{/if}
```

Keep all data fetching (`onMount`, state variables, fetch functions) in the `<script>` block of `+page.svelte`.

- [ ] **Step 7: Type check**

```bash
cd /opt/ocserv_panel/frontend && pnpm check 2>&1 | tail -20
```
Expected: zero errors.

- [ ] **Step 8: Verify line counts meet success criteria**

```bash
wc -l /opt/ocserv_panel/frontend/src/routes/+page.svelte
wc -l /opt/ocserv_panel/frontend/src/routes/Dashboard*.svelte
```
Expected: `+page.svelte` ≤ 300 lines; each component is manageable.

- [ ] **Step 9: Commit**

```bash
git add frontend/src/routes/ && git commit -m "refactor(dashboard): split +page.svelte into DashboardHealth, DashboardMetrics, DashboardSessions, DashboardActivity components"
```

---

## Task 9: Final verification

- [ ] **Step 1: Full backend test suite**

```bash
cd /opt/ocserv_panel && go test ./backend/... -count=1 2>&1 | grep -E "FAIL|ok|---"
```
Expected: all packages show `ok`, zero `FAIL`.

- [ ] **Step 2: Backend build**

```bash
cd /opt/ocserv_panel && go build ./backend/... 2>&1
```
Expected: no output.

- [ ] **Step 3: Frontend type check**

```bash
cd /opt/ocserv_panel/frontend && pnpm check 2>&1
```
Expected: zero errors.

- [ ] **Step 4: Confirm success criteria from spec**

```bash
# internal/app/ file count (should be ≤12)
find /opt/ocserv_panel/backend/internal/app -name "*.go" | grep -v "_test.go" | wc -l

# internal/ocserv/ root file count (should be ≤12)
find /opt/ocserv_panel/backend/internal/ocserv -maxdepth 1 -name "*.go" | grep -v "_test.go" | wc -l

# internal/store/ file count (should be 5)
find /opt/ocserv_panel/backend/internal/store -name "*.go" | grep -v "_test.go" | wc -l

# +page.svelte line count (should be ≤300)
wc -l /opt/ocserv_panel/frontend/src/routes/+page.svelte

# lib/api.ts should be gone, lib/api/ should exist
ls /opt/ocserv_panel/frontend/src/lib/api/
```

- [ ] **Step 5: Final commit**

```bash
git add -A && git commit -m "chore: verify architecture simplification complete — all success criteria met"
```

---

## Notes for Subagents

**Go skills to use:**
- `golang-code-style` — naming, error handling patterns
- `golang-testing` — test structure with `testify`
- `golang-dependency-injection` — interface wiring in handlers

**Svelte skills to use:**
- `svelte-runes` — use `$props()` for component props, not `export let`
- `sveltekit-data-flow` — page-level data fetching stays in `+page.svelte`, sub-components are pure rendering

**Key invariants:**
1. Never change a function signature or return type visible to the frontend.
2. Every file move uses a shim → migrate callers → remove shim sequence.
3. Build after every task. Tests must pass before committing.
4. Import paths change; API contracts do not.
