# Initial Repository Baseline Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the initial local-first repository baseline for the single-node `ocserv` Web UI admin panel.

**Architecture:** Create separate backend, frontend, migration, deployment, and documentation boundaries. The Go backend exposes only a safe health endpoint and placeholder interfaces for auth, RBAC, audit, SQLite, and `ocserv`; the SvelteKit frontend is hand-authored without dependency installation; SQLite, systemd, and HAProxy are represented by deterministic migrations and documentation only.

**Tech Stack:** Go standard library HTTP server, `database/sql` SQLite placeholder, SvelteKit/Vite/Svelte skeleton files, SQLite SQL migrations, Markdown documentation.

---

## Required Subagent Ownership

Execution must involve project subagents as requested by the user. Use one fresh subagent per task or clearly separated ownership area, and do not run agents in parallel if they would edit the same files.

- **Primary coordinator:** Task 1 and integration of handoffs.
- **backend-api:** Tasks 2 and 3.
- **sqlite-persistence:** Task 4.
- **frontend-admin-ux:** Task 5.
- **deployment-ops:** Task 6A for `deploy/README.md`.
- **docs-adr:** Task 6B for `docs/ADR-0001-architecture.md`, `docs/OPERATIONS.md`, and `docs/SECURITY.md`.
- **test-verification:** Task 7 verification pass.
- **code-review:** Task 8 final security/architecture review.

Every subagent handoff must include changed files, assumptions, tests/checks run, known risks, and next steps. Agents must not execute `ocserv`, `occtl`, `ocpasswd`, `systemctl`, `sudo`, or destructive filesystem commands.

## File Map

- `.gitignore`: Prevent local dependencies, build outputs, SQLite files, and environment files from being committed.
- `backend/go.mod`: Local Go module using only the standard library.
- `backend/cmd/ocserv-panel/main.go`: HTTP server entrypoint with graceful shutdown.
- `backend/internal/server/server.go`: Router and `GET /healthz` handler.
- `backend/internal/server/server_test.go`: Health endpoint tests.
- `backend/internal/auth/auth.go`: Authentication principal/session placeholder types.
- `backend/internal/rbac/rbac.go`: Role/permission constants and authorization helper.
- `backend/internal/rbac/rbac_test.go`: RBAC allow/deny tests.
- `backend/internal/audit/audit.go`: Audit event model and recorder interface.
- `backend/internal/db/sqlite.go`: SQLite `database/sql` connection helper placeholder.
- `backend/internal/ocserv/client.go`: Safe `ocserv` manager interface and no-op implementation.
- `backend/internal/ocserv/client_test.go`: Tests proving no-op integration does not perform destructive actions.
- `migrations/0001_initial.sql`: Initial deterministic SQLite schema placeholders.
- `frontend/package.json`: Manual frontend package metadata and scripts.
- `frontend/svelte.config.js`: SvelteKit adapter configuration.
- `frontend/vite.config.ts`: Vite/SvelteKit config.
- `frontend/tsconfig.json`: TypeScript configuration extending generated SvelteKit config.
- `frontend/src/app.html`: SvelteKit HTML shell.
- `frontend/src/app.css`: Basic local UI styling.
- `frontend/src/lib/navigation.ts`: Navigation placeholder data.
- `frontend/src/lib/PlaceholderPage.svelte`: Shared placeholder content component.
- `frontend/src/routes/+layout.svelte`: App shell and navigation.
- `frontend/src/routes/+page.svelte`: Dashboard placeholder.
- `frontend/src/routes/login/+page.svelte`: Login placeholder.
- `frontend/src/routes/users/+page.svelte`: User management placeholder.
- `frontend/src/routes/sessions/+page.svelte`: Active sessions placeholder.
- `frontend/src/routes/history/+page.svelte`: Historical sessions placeholder.
- `frontend/src/routes/config/+page.svelte`: Configuration placeholder.
- `frontend/src/routes/certificates/+page.svelte`: Certificate lifecycle placeholder.
- `frontend/src/routes/audit/+page.svelte`: Audit log placeholder.
- `frontend/src/routes/settings/+page.svelte`: Settings placeholder.
- `deploy/README.md`: Non-installed systemd/HAProxy deployment notes.
- `docs/ADR-0001-architecture.md`: Initial architecture decision record.
- `docs/OPERATIONS.md`: Operations assumptions and safe future runtime model.
- `docs/SECURITY.md`: Security boundaries and audit expectations.

### Task 1: Coordinator Baseline Guardrails

**Owner:** Primary coordinator.

**Files:**
- Create: `.gitignore`

- [ ] **Step 1: Add root ignore rules for unsafe or generated local files**

Create `.gitignore` with this exact content:

```gitignore
# Local environment and secrets
.env
.env.*
!.env.example

# SQLite runtime files
*.db
*.db-*
*.sqlite
*.sqlite3

# Go build outputs
backend/bin/
backend/ocserv-panel
backend/*.test

# Frontend dependencies and build outputs
frontend/node_modules/
frontend/.svelte-kit/
frontend/build/
frontend/.vite/

# OpenCode local dependencies
.opencode/node_modules/
```

- [ ] **Step 2: Check working tree after guardrail file**

Run:

```bash
git status --short
```

Expected: `.gitignore` appears as untracked or modified. Existing untracked project guidance files may also appear. Do not stage or commit unless the user explicitly requests a commit.

### Task 2: Backend Health Server

**Owner:** `backend-api` subagent.

**Allowed files:** `backend/go.mod`, `backend/cmd/ocserv-panel/main.go`, `backend/internal/server/server.go`, `backend/internal/server/server_test.go`.

**Forbidden files:** `frontend/**`, `migrations/**`, `deploy/**`, `docs/**` except task handoff text outside the repository if needed.

- [ ] **Step 1: Create the Go module and failing health endpoint tests**

Create `backend/go.mod`:

```go
module ocserv_panel/backend

go 1.22
```

Create `backend/internal/server/server_test.go`:

```go
package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthzReturnsOK(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()

	NewRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("expected JSON content type, got %q", got)
	}

	var body HealthResponse
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode health response: %v", err)
	}

	if body.Status != "ok" {
		t.Fatalf("expected status ok, got %q", body.Status)
	}
}

func TestHealthzRejectsNonGET(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodPost, "/healthz", nil)
	rec := httptest.NewRecorder()

	NewRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
}
```

- [ ] **Step 2: Run the server tests and verify they fail for missing implementation**

Run from `backend/`:

```bash
go test ./internal/server
```

Expected: FAIL with compiler errors mentioning `undefined: NewRouter` and `undefined: HealthResponse`.

- [ ] **Step 3: Implement the health router**

Create `backend/internal/server/server.go`:

```go
package server

import (
	"encoding/json"
	"net/http"
)

// HealthResponse is returned by the health endpoint.
type HealthResponse struct {
	Status string `json:"status"`
}

// NewRouter builds the backend HTTP router.
func NewRouter() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", handleHealthz)

	return securityHeaders(mux)
}

func handleHealthz(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	writeJSON(w, http.StatusOK, HealthResponse{Status: "ok"})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(value); err != nil {
		// The response is already committed. Future structured logging can record this.
		return
	}
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		next.ServeHTTP(w, r)
	})
}
```

- [ ] **Step 4: Add the backend server entrypoint**

Create `backend/cmd/ocserv-panel/main.go`:

```go
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"ocserv_panel/backend/internal/server"
)

func main() {
	addr := envOrDefault("OCSERV_PANEL_ADDR", "127.0.0.1:8080")

	httpServer := &http.Server{
		Addr:              addr,
		Handler:           server.NewRouter(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		<-ctx.Done()

		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			slog.Error("backend shutdown failed", "error", err)
		}
	}()

	slog.Info("starting ocserv panel backend", "addr", addr)
	if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		slog.Error("backend server failed", "error", err)
		os.Exit(1)
	}
}

func envOrDefault(key string, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}

	return fallback
}
```

- [ ] **Step 5: Run backend tests**

Run from `backend/`:

```bash
go test ./...
```

Expected: PASS for `internal/server` and successful compilation of `cmd/ocserv-panel`.

### Task 3: Backend Security-Sensitive Placeholders

**Owner:** `backend-api` subagent, with final review by `security-rbac` or `code-review` in Task 8.

**Allowed files:** `backend/internal/auth/auth.go`, `backend/internal/rbac/rbac.go`, `backend/internal/rbac/rbac_test.go`, `backend/internal/audit/audit.go`, `backend/internal/db/sqlite.go`, `backend/internal/ocserv/client.go`, `backend/internal/ocserv/client_test.go`.

**Forbidden files:** `frontend/**`, `migrations/**`, `deploy/**`, `docs/**`.

- [ ] **Step 1: Write failing RBAC tests**

Create `backend/internal/rbac/rbac_test.go`:

```go
package rbac

import "testing"

func TestAuthorizerAllowsKnownRolePermission(t *testing.T) {
	t.Parallel()

	authorizer := NewDefaultAuthorizer()

	if !authorizer.Allowed([]Role{RoleOwner}, PermissionReadDashboard) {
		t.Fatal("expected owner role to read dashboard")
	}
}

func TestAuthorizerDeniesMissingPermission(t *testing.T) {
	t.Parallel()

	authorizer := NewDefaultAuthorizer()

	if authorizer.Allowed([]Role{RoleViewer}, PermissionManageConfig) {
		t.Fatal("expected viewer role to be denied config management")
	}
}

func TestAuthorizerDeniesUnknownRole(t *testing.T) {
	t.Parallel()

	authorizer := NewDefaultAuthorizer()

	if authorizer.Allowed([]Role{"unknown"}, PermissionReadDashboard) {
		t.Fatal("expected unknown role to be denied")
	}
}
```

- [ ] **Step 2: Run RBAC tests and verify they fail for missing implementation**

Run from `backend/`:

```bash
go test ./internal/rbac
```

Expected: FAIL with compiler errors mentioning missing `NewDefaultAuthorizer`, `RoleOwner`, `RoleViewer`, `PermissionReadDashboard`, or `PermissionManageConfig`.

- [ ] **Step 3: Implement authentication and RBAC placeholders**

Create `backend/internal/auth/auth.go`:

```go
// Package auth contains authentication types and interfaces.
package auth

import "time"

// Principal represents an authenticated administrator.
type Principal struct {
	ID        int64
	Username  string
	Roles     []string
	CreatedAt time.Time
}

// Authenticated reports whether the principal has the minimum identity fields.
func (p Principal) Authenticated() bool {
	return p.ID > 0 && p.Username != ""
}
```

Create `backend/internal/rbac/rbac.go`:

```go
// Package rbac contains role and permission helpers for privileged actions.
package rbac

// Role identifies an administrative role.
type Role string

const (
	RoleOwner  Role = "owner"
	RoleViewer Role = "viewer"
)

// Permission identifies an action guarded by authorization checks.
type Permission string

const (
	PermissionReadDashboard Permission = "dashboard:read"
	PermissionManageUsers   Permission = "users:manage"
	PermissionManageConfig  Permission = "config:manage"
	PermissionManageService Permission = "service:manage"
)

// Authorizer evaluates role grants.
type Authorizer struct {
	grants map[Role]map[Permission]struct{}
}

// NewDefaultAuthorizer returns the initial local-first role grants.
func NewDefaultAuthorizer() Authorizer {
	return Authorizer{
		grants: map[Role]map[Permission]struct{}{
			RoleOwner: {
				PermissionReadDashboard: {},
				PermissionManageUsers:   {},
				PermissionManageConfig:  {},
				PermissionManageService: {},
			},
			RoleViewer: {
				PermissionReadDashboard: {},
			},
		},
	}
}

// Allowed reports whether any role grants the requested permission.
func (a Authorizer) Allowed(roles []Role, permission Permission) bool {
	for _, role := range roles {
		permissions, ok := a.grants[role]
		if !ok {
			continue
		}

		if _, ok := permissions[permission]; ok {
			return true
		}
	}

	return false
}
```

- [ ] **Step 4: Run RBAC tests and verify they pass**

Run from `backend/`:

```bash
go test ./internal/rbac
```

Expected: PASS.

- [ ] **Step 5: Write failing `ocserv` no-op integration tests**

Create `backend/internal/ocserv/client_test.go`:

```go
package ocserv

import (
	"context"
	"errors"
	"testing"
)

func TestNoopManagerListsNoSessions(t *testing.T) {
	t.Parallel()

	manager := NewNoopManager()
	sessions, err := manager.ListSessions(context.Background())
	if err != nil {
		t.Fatalf("list sessions: %v", err)
	}

	if len(sessions) != 0 {
		t.Fatalf("expected no sessions, got %d", len(sessions))
	}
}

func TestNoopManagerRefusesDisconnect(t *testing.T) {
	t.Parallel()

	manager := NewNoopManager()
	err := manager.DisconnectSession(context.Background(), "session-1")
	if !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("expected ErrNotConfigured, got %v", err)
	}
}
```

- [ ] **Step 6: Run `ocserv` tests and verify they fail for missing implementation**

Run from `backend/`:

```bash
go test ./internal/ocserv
```

Expected: FAIL with compiler errors mentioning missing `NewNoopManager` and `ErrNotConfigured`.

- [ ] **Step 7: Implement audit, DB, and `ocserv` placeholders**

Create `backend/internal/audit/audit.go`:

```go
// Package audit defines audit event boundaries for administrative actions.
package audit

import (
	"context"
	"time"
)

// Outcome records the result of an administrative action.
type Outcome string

const (
	OutcomeSuccess Outcome = "success"
	OutcomeFailure Outcome = "failure"
	OutcomeDenied  Outcome = "denied"
)

// Event describes an auditable administrative action.
type Event struct {
	ID           int64
	ActorID      int64
	Action       string
	ResourceType string
	ResourceID   string
	Outcome      Outcome
	Message      string
	Metadata     map[string]string
	CreatedAt    time.Time
}

// Recorder persists audit events.
type Recorder interface {
	Record(ctx context.Context, event Event) error
}

// NoopRecorder accepts audit events without persisting them.
type NoopRecorder struct{}

// Record implements Recorder.
func (NoopRecorder) Record(ctx context.Context, event Event) error {
	_ = ctx
	_ = event
	return nil
}
```

Create `backend/internal/db/sqlite.go`:

```go
// Package db contains local SQLite connection helpers.
package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

// ErrEmptyDSN is returned when no SQLite path or DSN is configured.
var ErrEmptyDSN = errors.New("sqlite dsn is empty")

// OpenSQLite opens a SQLite database using the conventional sqlite3 driver name.
// A concrete driver must be linked by the final application when persistence is implemented.
func OpenSQLite(ctx context.Context, dsn string) (*sql.DB, error) {
	return Open(ctx, "sqlite3", dsn)
}

// Open opens and pings a database connection using database/sql.
func Open(ctx context.Context, driverName string, dsn string) (*sql.DB, error) {
	if strings.TrimSpace(dsn) == "" {
		return nil, ErrEmptyDSN
	}

	database, err := sql.Open(driverName, dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite database: %w", err)
	}

	if err := database.PingContext(ctx); err != nil {
		_ = database.Close()
		return nil, fmt.Errorf("ping sqlite database: %w", err)
	}

	return database, nil
}
```

Create `backend/internal/ocserv/client.go`:

```go
// Package ocserv defines safe integration boundaries for ocserv operations.
package ocserv

import (
	"context"
	"errors"
	"time"
)

// ErrNotConfigured is returned by placeholder integrations that cannot touch live ocserv state.
var ErrNotConfigured = errors.New("ocserv integration is not configured")

// Session represents an active ocserv session as reported by a future adapter.
type Session struct {
	ID            string
	Username      string
	RemoteAddress string
	ConnectedAt   time.Time
}

// Manager is the boundary for ocserv management operations.
type Manager interface {
	ListSessions(ctx context.Context) ([]Session, error)
	DisconnectSession(ctx context.Context, sessionID string) error
}

// NoopManager is a safe development implementation that never calls ocserv.
type NoopManager struct{}

// NewNoopManager returns a safe no-op ocserv manager.
func NewNoopManager() NoopManager {
	return NoopManager{}
}

// ListSessions returns an empty session list without touching the system.
func (NoopManager) ListSessions(ctx context.Context) ([]Session, error) {
	_ = ctx
	return []Session{}, nil
}

// DisconnectSession refuses to terminate sessions because no live adapter is configured.
func (NoopManager) DisconnectSession(ctx context.Context, sessionID string) error {
	_ = ctx
	_ = sessionID
	return ErrNotConfigured
}
```

- [ ] **Step 8: Run backend tests after placeholders**

Run from `backend/`:

```bash
go test ./...
```

Expected: PASS for all backend packages. No command should call `ocserv`, `occtl`, `ocpasswd`, `systemctl`, or `sudo`.

### Task 4: Initial SQLite Migration

**Owner:** `sqlite-persistence` subagent.

**Allowed files:** `migrations/0001_initial.sql`.

**Forbidden files:** `backend/**`, `frontend/**`, `deploy/**`, `docs/**`.

- [ ] **Step 1: Create the initial migration**

Create `migrations/0001_initial.sql`:

```sql
PRAGMA foreign_keys = ON;

CREATE TABLE IF NOT EXISTS roles (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE,
    description TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CHECK (length(trim(name)) > 0)
);

CREATE TABLE IF NOT EXISTS admin_users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    username TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    role_id INTEGER NOT NULL,
    disabled_at TEXT,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (role_id) REFERENCES roles(id) ON UPDATE CASCADE ON DELETE RESTRICT,
    CHECK (length(trim(username)) > 0),
    CHECK (length(password_hash) > 0)
);

CREATE TABLE IF NOT EXISTS audit_log (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    actor_admin_user_id INTEGER,
    action TEXT NOT NULL,
    resource_type TEXT NOT NULL,
    resource_id TEXT,
    outcome TEXT NOT NULL CHECK (outcome IN ('success', 'failure', 'denied')),
    message TEXT NOT NULL DEFAULT '',
    metadata_json TEXT NOT NULL DEFAULT '{}',
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (actor_admin_user_id) REFERENCES admin_users(id) ON UPDATE CASCADE ON DELETE SET NULL,
    CHECK (length(trim(action)) > 0),
    CHECK (length(trim(resource_type)) > 0)
);

CREATE TABLE IF NOT EXISTS app_settings (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CHECK (length(trim(key)) > 0)
);

INSERT OR IGNORE INTO roles (name, description)
VALUES
    ('owner', 'Full local administration role for this single-node ocserv panel'),
    ('viewer', 'Read-only local administration role for dashboard visibility');

INSERT OR IGNORE INTO app_settings (key, value)
VALUES
    ('app.schema_version', '1');
```

- [ ] **Step 2: Review migration for secret safety**

Confirm the migration contains no plaintext passwords, private keys, tokens, or credentials. The `password_hash` column is for future hashed passwords only.

- [ ] **Step 3: Check migration file is present**

Run:

```bash
git status --short migrations/0001_initial.sql
```

Expected: `?? migrations/0001_initial.sql` or a modified status for that path. Do not apply the migration to a live database in this baseline task.

### Task 5: Hand-Authored SvelteKit Frontend Skeleton

**Owner:** `frontend-admin-ux` subagent.

**Allowed files:** `frontend/**`.

**Forbidden files:** `backend/**`, `migrations/**`, `deploy/**`, `docs/**`.

- [ ] **Step 1: Create package and SvelteKit config files without installing dependencies**

Create `frontend/package.json`:

```json
{
  "name": "ocserv-panel-frontend",
  "version": "0.1.0",
  "private": true,
  "type": "module",
  "scripts": {
    "dev": "vite dev",
    "build": "vite build",
    "preview": "vite preview",
    "check": "svelte-kit sync && svelte-check --tsconfig ./tsconfig.json"
  },
  "devDependencies": {
    "@sveltejs/adapter-auto": "^6.0.0",
    "@sveltejs/kit": "^2.0.0",
    "@sveltejs/vite-plugin-svelte": "^5.0.0",
    "svelte": "^5.0.0",
    "svelte-check": "^4.0.0",
    "typescript": "^5.0.0",
    "vite": "^6.0.0"
  }
}
```

Create `frontend/svelte.config.js`:

```js
import adapter from '@sveltejs/adapter-auto';
import { vitePreprocess } from '@sveltejs/vite-plugin-svelte';

/** @type {import('@sveltejs/kit').Config} */
const config = {
  preprocess: vitePreprocess(),
  kit: {
    adapter: adapter()
  }
};

export default config;
```

Create `frontend/vite.config.ts`:

```ts
import { sveltekit } from '@sveltejs/kit/vite';
import { defineConfig } from 'vite';

export default defineConfig({
  plugins: [sveltekit()]
});
```

Create `frontend/tsconfig.json`:

```json
{
  "extends": "./.svelte-kit/tsconfig.json",
  "compilerOptions": {
    "allowJs": true,
    "checkJs": true,
    "esModuleInterop": true,
    "forceConsistentCasingInFileNames": true,
    "resolveJsonModule": true,
    "skipLibCheck": true,
    "sourceMap": true,
    "strict": true,
    "moduleResolution": "bundler"
  }
}
```

- [ ] **Step 2: Create app shell files and shared navigation**

Create `frontend/src/app.html`:

```html
<!doctype html>
<html lang="en">
  <head>
    <meta charset="utf-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1" />
    %sveltekit.head%
  </head>
  <body data-sveltekit-preload-data="hover">
    <div style="display: contents">%sveltekit.body%</div>
  </body>
</html>
```

Create `frontend/src/app.css`:

```css
:root {
  color: #172033;
  background: #f6f8fb;
  font-family:
    Inter, ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
}

body {
  margin: 0;
  min-width: 320px;
}

a {
  color: inherit;
}

.shell {
  display: grid;
  grid-template-columns: 260px minmax(0, 1fr);
  min-height: 100vh;
}

.sidebar {
  background: #101828;
  color: #f9fafb;
  padding: 1.5rem;
}

.brand {
  margin: 0 0 1.5rem;
  font-size: 1.15rem;
}

.nav {
  display: grid;
  gap: 0.4rem;
}

.nav a {
  border-radius: 0.6rem;
  color: #d0d5dd;
  padding: 0.65rem 0.75rem;
  text-decoration: none;
}

.nav a:hover {
  background: #1d2939;
  color: #ffffff;
}

.content {
  padding: 2rem;
}

.panel {
  background: #ffffff;
  border: 1px solid #e4e7ec;
  border-radius: 0.9rem;
  box-shadow: 0 1px 2px rgb(16 24 40 / 6%);
  padding: 1.5rem;
}

.eyebrow {
  color: #475467;
  font-size: 0.85rem;
  font-weight: 700;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}

.muted {
  color: #667085;
}

@media (max-width: 760px) {
  .shell {
    grid-template-columns: 1fr;
  }
}
```

Create `frontend/src/lib/navigation.ts`:

```ts
export type NavItem = {
  href: string;
  label: string;
  description: string;
};

export const navItems: NavItem[] = [
  { href: '/', label: 'Dashboard', description: 'Local ocserv status overview' },
  { href: '/users', label: 'Users', description: 'Password and certificate users' },
  { href: '/sessions', label: 'Sessions', description: 'Active session visibility and control' },
  { href: '/history', label: 'History', description: 'Historical session analytics' },
  { href: '/config', label: 'Config', description: 'Validated configuration editing' },
  { href: '/certificates', label: 'Certificates', description: 'Certificate visibility and lifecycle' },
  { href: '/audit', label: 'Audit', description: 'Administrative action history' },
  { href: '/settings', label: 'Settings', description: 'Local panel settings' }
];
```

Create `frontend/src/lib/PlaceholderPage.svelte`:

```svelte
<script lang="ts">
  export let eyebrow = 'Placeholder';
  export let title: string;
  export let description: string;
  export let items: string[] = [];
</script>

<section class="panel">
  <p class="eyebrow">{eyebrow}</p>
  <h1>{title}</h1>
  <p class="muted">{description}</p>

  {#if items.length > 0}
    <ul>
      {#each items as item}
        <li>{item}</li>
      {/each}
    </ul>
  {/if}
</section>
```

Create `frontend/src/routes/+layout.svelte`:

```svelte
<script lang="ts">
  import '../app.css';
  import { navItems } from '$lib/navigation';

  let { children } = $props();
</script>

<svelte:head>
  <title>ocserv Panel</title>
  <meta
    name="description"
    content="Local-first web administration panel for a single ocserv server"
  />
</svelte:head>

<div class="shell">
  <aside class="sidebar" aria-label="Primary navigation">
    <h1 class="brand">ocserv Panel</h1>
    <nav class="nav">
      {#each navItems as item}
        <a href={item.href} title={item.description}>{item.label}</a>
      {/each}
      <a href="/login">Login</a>
    </nav>
  </aside>

  <main class="content">
    {@render children()}
  </main>
</div>
```

- [ ] **Step 3: Create dashboard and login pages**

Create `frontend/src/routes/+page.svelte`:

```svelte
<script lang="ts">
  import PlaceholderPage from '$lib/PlaceholderPage.svelte';
</script>

<PlaceholderPage
  eyebrow="Dashboard"
  title="Local ocserv administration"
  description="This dashboard will summarize local ocserv state through backend APIs. No live ocserv integration is active in the baseline."
  items={[
    'Health endpoint available from the Go backend.',
    'Future cards will show active sessions, certificate expiry, and recent audit events.',
    'No Prometheus, Grafana, cloud service, or external control plane is used.'
  ]}
/>
```

Create `frontend/src/routes/login/+page.svelte`:

```svelte
<script lang="ts">
  import PlaceholderPage from '$lib/PlaceholderPage.svelte';
</script>

<PlaceholderPage
  eyebrow="Login"
  title="Admin login placeholder"
  description="Authentication will be implemented through backend APIs with hashed credentials and RBAC checks. This placeholder does not collect or store passwords."
  items={[
    'No credentials are stored in frontend code.',
    'Future login will create a backend-managed admin session.',
    'Password changes and session termination will require audit events.'
  ]}
/>
```

- [ ] **Step 4: Create administrative placeholder pages**

Create `frontend/src/routes/users/+page.svelte`:

```svelte
<script lang="ts">
  import PlaceholderPage from '$lib/PlaceholderPage.svelte';
</script>

<PlaceholderPage
  eyebrow="Users"
  title="User lifecycle management"
  description="Future workflows will manage password and certificate-based ocserv users through authorized backend APIs."
  items={[
    'User creation and deletion will require RBAC authorization.',
    'Password handling will avoid plaintext storage.',
    'Certificate workflows will avoid storing private keys in SQLite.'
  ]}
/>
```

Create `frontend/src/routes/sessions/+page.svelte`:

```svelte
<script lang="ts">
  import PlaceholderPage from '$lib/PlaceholderPage.svelte';
</script>

<PlaceholderPage
  eyebrow="Sessions"
  title="Active sessions"
  description="Future session visibility and disconnect actions will use a backend ocserv integration interface with fakes in tests."
  items={[
    'No management socket access occurs in the frontend.',
    'Session termination will require explicit confirmation and audit logging.',
    'Missing socket or permission errors will be surfaced safely.'
  ]}
/>
```

Create `frontend/src/routes/history/+page.svelte`:

```svelte
<script lang="ts">
  import PlaceholderPage from '$lib/PlaceholderPage.svelte';
</script>

<PlaceholderPage
  eyebrow="History"
  title="Historical session analytics"
  description="Future views will summarize locally persisted session history from SQLite without external monitoring stacks."
  items={[
    'Analytics will be local to this host.',
    'No Prometheus or Grafana dependency is planned.',
    'Historical data will be read through backend APIs.'
  ]}
/>
```

Create `frontend/src/routes/config/+page.svelte`:

```svelte
<script lang="ts">
  import PlaceholderPage from '$lib/PlaceholderPage.svelte';
</script>

<PlaceholderPage
  eyebrow="Config"
  title="Safe configuration editing"
  description="Future config changes will require validation, backup, preview, and safe apply paths before touching live ocserv configuration."
  items={[
    'The baseline does not mutate live configuration.',
    'Future changes must validate before apply.',
    'Backups and rollback assumptions must be documented.'
  ]}
/>
```

Create `frontend/src/routes/certificates/+page.svelte`:

```svelte
<script lang="ts">
  import PlaceholderPage from '$lib/PlaceholderPage.svelte';
</script>

<PlaceholderPage
  eyebrow="Certificates"
  title="Certificate lifecycle"
  description="Future certificate views will show safe metadata, expiry, issuance, and revocation status without storing private keys."
  items={[
    'Private keys must not be stored in source control or SQLite.',
    'Revocation and issuance will require backend authorization.',
    'Certificate filesystem access must be isolated behind interfaces.'
  ]}
/>
```

Create `frontend/src/routes/audit/+page.svelte`:

```svelte
<script lang="ts">
  import PlaceholderPage from '$lib/PlaceholderPage.svelte';
</script>

<PlaceholderPage
  eyebrow="Audit"
  title="Audit log"
  description="Future administrative actions that mutate users, sessions, config, certificates, or service state will produce audit records."
  items={[
    'Denied actions should be auditable where practical.',
    'Audit records should avoid secrets and sensitive payloads.',
    'The frontend will read audit history through backend APIs.'
  ]}
/>
```

Create `frontend/src/routes/settings/+page.svelte`:

```svelte
<script lang="ts">
  import PlaceholderPage from '$lib/PlaceholderPage.svelte';
</script>

<PlaceholderPage
  eyebrow="Settings"
  title="Panel settings"
  description="Future local settings will be stored in SQLite and managed through backend APIs."
  items={[
    'Settings must not contain plaintext secrets.',
    'Operational paths and permission assumptions must be documented.',
    'Runtime changes that affect services require safe validation and audit paths.'
  ]}
/>
```

- [ ] **Step 5: Check frontend without installing dependencies**

Run from `frontend/`:

```bash
test -d node_modules && npm run check || true
```

Expected: If `frontend/node_modules` exists, `npm run check` should run. If it does not exist, the command exits successfully without installing dependencies. Record in the handoff whether the check ran or was skipped.

### Task 6A: Deployment Placeholder Documentation

**Owner:** `deployment-ops` subagent.

**Allowed files:** `deploy/README.md`.

**Forbidden files:** `backend/**`, `frontend/**`, `migrations/**`, `docs/**`.

- [ ] **Step 1: Create deployment README**

Create `deploy/README.md`:

```markdown
# Deployment Assets

This directory is reserved for local deployment assets for the `ocserv` admin panel.

The intended runtime model is:

- The Go backend runs as a local systemd service on the same host as `ocserv`.
- HAProxy terminates TLS and forwards requests to the backend loopback listener.
- Future privileged operations use narrow, documented permission boundaries.
- SQLite remains local to the host.

No files in this baseline are installed automatically. Do not run `systemctl`, `sudo`, `ocserv`, `occtl`, or `ocpasswd` as part of baseline verification.

Future deployment work should add reviewed example assets for:

- `ocserv-panel.service`
- HAProxy frontend/backend snippets
- Minimal sudoers rules, if privileged operations require them
- Filesystem ownership and permission guidance

Any future service-control or configuration mutation path must validate inputs, create backups where applicable, and produce audit records.
```

- [ ] **Step 2: Check deployment file status**

Run:

```bash
git status --short deploy/README.md
```

Expected: `?? deploy/README.md` or modified status for that path.

### Task 6B: Architecture, Operations, and Security Docs

**Owner:** `docs-adr` subagent.

**Allowed files:** `docs/ADR-0001-architecture.md`, `docs/OPERATIONS.md`, `docs/SECURITY.md`.

**Forbidden files:** `backend/**`, `frontend/**`, `migrations/**`, `deploy/**`.

- [ ] **Step 1: Create the architecture ADR**

Create `docs/ADR-0001-architecture.md`:

```markdown
# ADR-0001: Local-First Single-Node Architecture

## Status

Accepted.

## Context

The project provides a personal-use Web UI admin panel for an `ocserv` server running on the same host. It replaces routine manual administration with a local dashboard while avoiding external services and distributed state.

## Decision

Use the following architecture:

- Go backend for HTTP APIs, RBAC, audit logging, SQLite access, and isolated privileged-operation interfaces.
- SvelteKit frontend for the admin dashboard and operator workflows.
- SQLite for local persistence.
- systemd for runtime management.
- HAProxy for TLS termination in front of the Go backend.

The backend owns all access to local files, sockets, commands, certificates, and service-control boundaries. The frontend communicates only through backend APIs.

## Consequences

- The system stays lightweight and suitable for a single host.
- Normal tests must use fakes or adapters and must not require a live `ocserv` instance.
- Privileged operations require explicit interfaces, authorization checks, validation, error handling, and audit logging.
- Multi-node orchestration, hosted dependencies, external monitoring stacks, queues, and cloud control planes remain out of scope unless explicitly requested.
```

- [ ] **Step 2: Create operations documentation**

Create `docs/OPERATIONS.md`:

```markdown
# Operations Guide

## Runtime Model

`ocserv_panel` is designed to run on the same host as `ocserv`.

Expected components:

- Go backend listening on a loopback address such as `127.0.0.1:8080`.
- HAProxy terminating TLS and forwarding to the backend.
- SQLite database stored on the local filesystem.
- systemd managing the backend process in future deployment work.

## Baseline Safety

The initial baseline does not install services, modify live `ocserv` configuration, access the `ocserv` management socket, or run privileged commands.

Do not assume root access is available. Future operations that require elevated permissions must document:

- Required runtime user and groups.
- Exact filesystem paths and permissions.
- Minimal sudoers rules, if any.
- Validation and backup steps before configuration changes.
- Audit events emitted for administrative mutations.

## Future Safe Apply Expectations

Configuration editing must follow this sequence:

1. Parse and validate the proposed configuration.
2. Show a preview or diff to the administrator.
3. Create a backup of the current configuration.
4. Apply through a narrow backend interface.
5. Verify the result and report errors clearly.
6. Preserve an audit record for success, failure, or denial.

Normal tests should use fakes for service control, filesystem writes, and `ocserv` integration.
```

- [ ] **Step 3: Create security documentation**

Create `docs/SECURITY.md`:

```markdown
# Security Guide

## Security-Sensitive Boundaries

The following areas are security-sensitive:

- Authentication and session management.
- RBAC checks.
- User creation, deletion, and password changes.
- Certificate issuance, visibility, and revocation.
- Active session termination.
- `ocserv` configuration edits.
- systemd, sudo, filesystem, and management socket access.

## Required Controls

Every privileged backend action should include:

- Authentication.
- Authorization through RBAC.
- Input validation.
- Clear error handling.
- Audit logging.
- Tests using fakes or adapters where practical.

## Secret Handling

Do not store plaintext passwords, private keys, API tokens, credentials, or generated secrets in source control or SQLite.

Password-related persistence must store only password hashes. Certificate persistence must store safe metadata and must not store private keys.

## Frontend Boundary

The SvelteKit frontend must not access local files, sockets, commands, certificates, or secrets directly. It must call backend APIs for all administrative behavior.

## Baseline Posture

The initial baseline includes placeholders and no-op interfaces only. It does not execute `ocserv`, `occtl`, `ocpasswd`, `systemctl`, `sudo`, or destructive filesystem commands.
```

- [ ] **Step 4: Check documentation file status**

Run:

```bash
git status --short docs/ADR-0001-architecture.md docs/OPERATIONS.md docs/SECURITY.md
```

Expected: each new documentation file appears as untracked or modified.

### Task 7: Verification Pass

**Owner:** `test-verification` subagent.

**Allowed files:** none unless a verification-only fix is explicitly requested by the coordinator after reviewing failures.

**Forbidden commands:** `ocserv`, `occtl`, `ocpasswd`, `systemctl`, `sudo`, destructive filesystem commands, dependency installation commands.

- [ ] **Step 1: Run backend tests**

Run from `backend/`:

```bash
go test ./...
```

Expected: PASS.

- [ ] **Step 2: Run frontend check only if dependencies already exist**

Run from `frontend/`:

```bash
test -d node_modules && npm run check || true
```

Expected: If dependencies exist, record the `npm run check` result. If dependencies do not exist, record that frontend verification was skipped because dependency installation was intentionally avoided.

- [ ] **Step 3: Inspect repository status**

Run:

```bash
git status --short
```

Expected: new baseline files are visible. Existing untracked `.opencode/`, `AGENTS.md`, or `docs/superpowers/**` may also be visible depending on the current repository state.

- [ ] **Step 4: Report verification handoff**

Return a handoff with these exact headings and real observed values:

```text
Changed files checked:
- List each path shown by git status that belongs to this baseline.

Commands run:
- go test ./... from backend/: record PASS or the exact failure summary.
- frontend dependency-gated check: record skipped because node_modules was absent, or record PASS/failure if npm run check executed.
- git status --short: summarize new and modified baseline paths.

Safety confirmation:
- No live ocserv/system/sudo commands were executed.
- No dependency installation was executed.

Risks:
- Write "none found in verification" if no issue was found; otherwise list each concrete issue.
```

### Task 8: Final Code and Security Review

**Owner:** `code-review` subagent. Use `security-rbac` as an additional reviewer if the coordinator wants separate RBAC/security review.

**Allowed files:** read-only review. Do not edit files unless the coordinator explicitly assigns a follow-up fix task.

- [ ] **Step 1: Review baseline changes**

Review these concerns:

- Backend health server compiles and exposes only safe behavior.
- RBAC placeholders are deny-by-default for unknown roles and missing permissions.
- `ocserv` no-op integration cannot terminate sessions or run commands.
- SQLite migration stores no plaintext passwords, private keys, tokens, or credentials.
- Frontend does not access local files, sockets, commands, certificates, or secrets.
- Deployment docs do not instruct automatic service mutation.
- Architecture remains local-first, single-node, SQLite-backed, systemd-managed, and HAProxy-terminated.

- [ ] **Step 2: Return review handoff**

Return a handoff with these exact headings and real review findings:

```text
Review result: write "pass" if there are no blocking issues; otherwise write "issues found".

Files reviewed:
- List each reviewed file path.

Findings:
- For each finding, include severity, file, issue, and recommendation. If there are no findings, write "none".

Security notes:
- Summarize RBAC, authentication, audit, secret-handling, and system-boundary observations.

Required fixes before final response:
- List each required fix. If none are required, write "none".
```

### Task 9: Coordinator Final Response

**Owner:** Primary coordinator.

**Files:** no edits unless review requires fixes.

- [ ] **Step 1: Reconcile subagent handoffs**

Confirm every development subagent provided changed files, assumptions, tests/checks run, risks, and next steps.

- [ ] **Step 2: Apply required fixes if review found issues**

If Task 8 reports issues, create a focused follow-up task for the correct owner subagent. Do not silently discard any subagent work.

- [ ] **Step 3: Prepare final output**

Final response must include:

```text
Created structure:
- backend/
- frontend/
- migrations/
- deploy/
- docs/

Files added:
- List every added baseline file.

Tests/checks run:
- List every verification command and result.

Known gaps:
- No real auth flow.
- No live ocserv integration.
- No config apply path.
- No certificate lifecycle implementation.
- No frontend/backend API integration beyond placeholders.

Recommended next development step:
- Name one focused next step, such as implementing real admin authentication with hashed passwords and RBAC-protected sessions.
```

Do not claim work is complete until verification output and code-review handoff have been reviewed.
