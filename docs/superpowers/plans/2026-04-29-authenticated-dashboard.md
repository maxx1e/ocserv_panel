# Authenticated Dashboard Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the first real ocserv_panel Web UI slice: first-run owner setup, login/logout sessions, authenticated dashboard/status/history APIs, sample historical analytics, and an owner-only historical purge action.

**Architecture:** Keep the custom Go backend as the only API and privileged-boundary owner. Use SQLite through the approved pure-Go `modernc.org/sqlite` driver, stdlib PBKDF2-HMAC-SHA256 password hashes, hashed opaque session tokens in HttpOnly cookies, SvelteKit same-origin `/api` calls, and the existing no-op ocserv adapter for safe `not_configured` status.

**Tech Stack:** Go 1.24+ (`crypto/pbkdf2`, `database/sql`, `net/http`), `modernc.org/sqlite`, SvelteKit/Svelte 5/Vite, SQLite migrations, fake/no-op ocserv integration in tests.

---

## Coordination Rules

- Work in `/opt/git/ocserv_panel/.worktrees/initial-repository-baseline`.
- Do not commit implementation changes unless the user explicitly requests a commit.
- Do not run `ocserv`, `occtl`, `ocpasswd`, `systemctl`, `sudo`, service-control commands, or live config/certificate mutations.
- Run backend commands with `PATH="$HOME/.local/bin:$PATH"` so the user-installed Go and Node toolchains are used.
- Use subagents sequentially where files overlap. Do not dispatch two agents that edit the same migration, API contract, auth/session type, or frontend API client at the same time.
- Every security-sensitive task needs tests and a review before moving to dependent tasks.

## File Map

### Backend dependency and database foundation

- Modify: `backend/go.mod` — set Go version to 1.24 or newer and add `modernc.org/sqlite`.
- Create/modify: `backend/go.sum` — checksums for SQLite driver and transitive dependencies.
- Modify: `backend/internal/db/sqlite.go` — switch driver name to `sqlite`, add connection pool settings, keep empty DSN guard.
- Create: `backend/internal/db/migrate.go` — apply migration files from a directory in order.
- Create: `backend/internal/db/migrate_test.go` — migration runner tests using in-memory SQLite.
- Modify: `migrations/0001_initial.sql` — extend the initial unreleased schema with sessions, role mapping, audit persistence, historical analytics, and sample data.

### Backend auth, store, and services

- Modify: `backend/internal/auth/auth.go` — align `Principal.Roles` with RBAC roles while keeping JSON-safe API types.
- Create: `backend/internal/auth/password.go` and `backend/internal/auth/password_test.go` — PBKDF2-HMAC-SHA256 encoding and verification.
- Create: `backend/internal/auth/session.go` and `backend/internal/auth/session_test.go` — random session token generation and SHA-256 token hashing.
- Create: `backend/internal/store/store.go` and `backend/internal/store/store_test.go` — SQLite repository for admins, sessions, audit, analytics, and purge.
- Modify: `backend/internal/audit/audit.go` — make audit metadata JSON-compatible and keep `Recorder` interface.
- Create: `backend/internal/audit/sqlite.go` and `backend/internal/audit/sqlite_test.go` — persisted audit recorder.
- Create: `backend/internal/app/auth_service.go` and `backend/internal/app/auth_service_test.go` — setup, login, logout, current principal service.
- Create: `backend/internal/app/dashboard_service.go` and `backend/internal/app/dashboard_service_test.go` — dashboard/history/purge service logic.

### Backend HTTP API

- Modify: `backend/internal/server/server.go` — keep health behavior and route through an `App` handler.
- Create: `backend/internal/server/app.go` — server dependencies, cookie config, route registration.
- Create: `backend/internal/server/json.go` — JSON request/response helpers with consistent error shape.
- Create: `backend/internal/server/auth_middleware.go` — session-cookie authentication and RBAC guards.
- Create: `backend/internal/server/auth_handlers.go` and `backend/internal/server/auth_handlers_test.go` — setup/login/logout/me API tests.
- Create: `backend/internal/server/dashboard_handlers.go` and `backend/internal/server/dashboard_handlers_test.go` — dashboard/history/purge API tests.
- Modify: `backend/cmd/ocserv-panel/main.go` — wire DB open, migration apply, store, services, no-op ocserv manager, and cookie config.

### Frontend

- Modify: `frontend/vite.config.ts` — proxy `/api` to `http://127.0.0.1:8080` in dev.
- Create: `frontend/src/lib/api.ts` — same-origin JSON API client and typed responses.
- Create: `frontend/src/lib/auth.ts` — small auth state helpers for setup/me/login/logout calls.
- Modify: `frontend/src/routes/+layout.svelte` — auth-aware shell, logout button, read-only navigation state.
- Modify: `frontend/src/routes/+page.svelte` — authenticated dashboard state.
- Modify: `frontend/src/routes/login/+page.svelte` — real login form.
- Create: `frontend/src/routes/setup/+page.svelte` — first-run owner setup form.
- Modify: `frontend/src/routes/history/+page.svelte` — sample analytics and owner-only purge UI.
- Modify: `frontend/src/app.css` — operational form, card, alert, and destructive-action styles.

### Docs and verification

- Modify: `docs/OPERATIONS.md` — document local SQLite DSN, migration directory, first-run setup, dev run commands.
- Modify: `docs/SECURITY.md` — document PBKDF2 hashing, session cookie model, RBAC, purge audit behavior.
- Modify: `docs/ADR-0001-architecture.md` — note approved pure-Go SQLite driver and custom backend decision after PocketBase evaluation.

---

## Task 1: SQLite Driver, Migration Runner, and Schema

**Owner:** SQLite persistence agent.

**Files:**
- Modify: `backend/go.mod`
- Create/modify: `backend/go.sum`
- Modify: `backend/internal/db/sqlite.go`
- Create: `backend/internal/db/migrate.go`
- Create: `backend/internal/db/migrate_test.go`
- Modify: `migrations/0001_initial.sql`

- [ ] **Step 1: Add the approved SQLite driver dependency**

Run from `backend/`:

```bash
PATH="$HOME/.local/bin:$PATH" go mod edit -go=1.24
PATH="$HOME/.local/bin:$PATH" go get modernc.org/sqlite@latest
```

Expected: `backend/go.mod` has `go 1.24` or newer and a `modernc.org/sqlite` requirement. `backend/go.sum` is created or updated.

- [ ] **Step 2: Write migration runner tests first**

Create `backend/internal/db/migrate_test.go` with tests for ordered migration application and empty migration directory behavior. Required test names:

```go
func TestApplyMigrationsAppliesSQLFilesInOrder(t *testing.T)
func TestApplyMigrationsRejectsMissingDirectory(t *testing.T)
```

The first test must create a temporary directory with `0001_create.sql` and `0002_insert.sql`, open an in-memory SQLite database with driver `sqlite`, call `ApplyMigrations(ctx, database, dir)`, and assert the inserted row exists.

- [ ] **Step 3: Run tests and confirm RED**

Run from `backend/`:

```bash
PATH="$HOME/.local/bin:$PATH" go test ./internal/db
```

Expected: fail because `ApplyMigrations` is not defined.

- [ ] **Step 4: Implement `ApplyMigrations` and update SQLite open helper**

Implement in `backend/internal/db/migrate.go`:

```go
func ApplyMigrations(ctx context.Context, database *sql.DB, dir string) error
```

Rules:

- Read directory entries with `os.ReadDir`.
- Select files ending in `.sql`.
- Sort names lexicographically.
- For each migration, read it with `os.ReadFile` and apply with `database.ExecContext(ctx, string(sqlBytes))`.
- Return wrapped lowercase errors.
- Return an error when the directory is missing or no `.sql` files exist.
- Do not shell out to migration tools.

Modify `backend/internal/db/sqlite.go`:

- `OpenSQLite` calls `Open(ctx, "sqlite", dsn)`.
- Configure pool conservatively for SQLite: `SetMaxOpenConns(1)`, `SetMaxIdleConns(1)`, `SetConnMaxLifetime(30*time.Minute)`.

- [ ] **Step 5: Update the initial schema**

Modify `migrations/0001_initial.sql` so the unreleased baseline schema contains:

- `roles` with unique `name` and seeded `owner`, `viewer`.
- `admin_users` with `id`, `username`, `password_hash`, `disabled_at`, `created_at`, `updated_at`; no plaintext password column.
- `admin_user_roles` linking users to roles.
- `admin_sessions` storing `token_hash`, `expires_at`, `revoked_at`, timestamps, and never raw tokens.
- `audit_log` with safe metadata JSON and outcomes `success`, `failure`, `denied`.
- `historical_sessions` with username, connected/disconnected timestamps, bytes in/out, duration seconds, and `source` constrained to `sample` or `ingested`.
- sample `historical_sessions` rows marked `source = 'sample'`.
- `app_settings` with `app.schema_version = 1`.

Use deterministic static sample rows. Do not create a seed admin user or seed password.

- [ ] **Step 6: Run GREEN verification**

Run from `backend/`:

```bash
PATH="$HOME/.local/bin:$PATH" go test ./internal/db
PATH="$HOME/.local/bin:$PATH" go test ./...
```

Expected: all backend tests pass.

- [ ] **Step 7: Review for dependency and secret safety**

Check `backend/go.mod`, `backend/go.sum`, and `migrations/0001_initial.sql` for:

- only approved new Go dependency family for SQLite;
- no plaintext passwords, raw session tokens, private keys, generated credentials, or seed admin credentials;
- no live ocserv/system commands.

Do not commit.

---

## Task 2: Password and Session Primitives

**Owner:** Backend API agent, with security review after implementation.

**Files:**
- Modify: `backend/internal/auth/auth.go`
- Create: `backend/internal/auth/password.go`
- Create: `backend/internal/auth/password_test.go`
- Create: `backend/internal/auth/session.go`
- Create: `backend/internal/auth/session_test.go`

- [ ] **Step 1: Write password hashing tests first**

Create `backend/internal/auth/password_test.go` with these test names:

```go
func TestHashPasswordDoesNotExposePlaintextAndVerifies(t *testing.T)
func TestHashPasswordUsesRandomSalt(t *testing.T)
func TestVerifyPasswordRejectsMalformedHash(t *testing.T)
```

Required assertions:

- Encoded hash starts with `pbkdf2-sha256$`.
- Encoded hash does not contain the plaintext password.
- Correct password verifies.
- Incorrect password does not verify.
- Two hashes of the same password differ.
- malformed hash returns an error or false without panic.

- [ ] **Step 2: Write session token tests first**

Create `backend/internal/auth/session_test.go` with these test names:

```go
func TestNewSessionTokenReturnsOpaqueRandomToken(t *testing.T)
func TestHashSessionTokenIsStableAndDoesNotExposeToken(t *testing.T)
```

Required assertions:

- Two generated tokens differ.
- Token strings are URL/base64 safe and at least 32 bytes of entropy before encoding.
- Token hash is deterministic for the same token.
- Token hash does not equal or contain the raw token.

- [ ] **Step 3: Run tests and confirm RED**

Run from `backend/`:

```bash
PATH="$HOME/.local/bin:$PATH" go test ./internal/auth
```

Expected: fail because password/session functions are not defined.

- [ ] **Step 4: Implement password hashing**

Implement `backend/internal/auth/password.go` with:

```go
const PasswordHashAlgorithm = "pbkdf2-sha256"

func HashPassword(password string) (string, error)
func VerifyPassword(encoded string, password string) (bool, error)
```

Required implementation details:

- Reject empty passwords before hashing.
- Use `crypto/rand` for a 16-byte salt.
- Use `crypto/pbkdf2.Key(sha256.New, password, salt, 600000, 32)`.
- Encode as `pbkdf2-sha256$600000$<base64-rawurl-salt>$<base64-rawurl-key>`.
- Parse and validate all fields during verification.
- Compare derived key with stored key using `subtle.ConstantTimeCompare`.

- [ ] **Step 5: Implement session token helpers**

Implement `backend/internal/auth/session.go` with:

```go
const SessionTokenBytes = 32

func NewSessionToken() (string, error)
func HashSessionToken(token string) string
```

Required implementation details:

- Generate 32 bytes with `crypto/rand`.
- Return base64 raw URL encoding for the cookie token.
- Hash token with SHA-256 and return hex encoding for SQLite storage.

- [ ] **Step 6: Run GREEN verification**

Run from `backend/`:

```bash
PATH="$HOME/.local/bin:$PATH" go test ./internal/auth
PATH="$HOME/.local/bin:$PATH" go test ./...
```

Expected: all backend tests pass.

Do not commit.

---

## Task 3: SQLite Store and Audit Recorder

**Owner:** SQLite persistence agent.

**Files:**
- Create: `backend/internal/store/store.go`
- Create: `backend/internal/store/store_test.go`
- Modify: `backend/internal/audit/audit.go`
- Create: `backend/internal/audit/sqlite.go`
- Create: `backend/internal/audit/sqlite_test.go`

- [ ] **Step 1: Write store tests first**

Create `backend/internal/store/store_test.go`. Use a helper that opens `sqlite` in-memory DB, applies `../../migrations/0001_initial.sql` through `db.ApplyMigrations`, and returns a `*Store`.

Required test names:

```go
func TestStoreCreateFirstOwnerOnlyOnce(t *testing.T)
func TestStoreFindAdminByUsernameIncludesRoles(t *testing.T)
func TestStoreSessionLifecycle(t *testing.T)
func TestStoreHistoricalAnalyticsReturnsSampleRows(t *testing.T)
func TestStorePurgeHistoricalDeletesOnlyOlderAnalytics(t *testing.T)
```

Required behavior:

- `CreateFirstOwner` succeeds when no admin exists and refuses once an admin exists.
- `FindAdminByUsername` returns roles including `owner`.
- Session creation stores a hash and current lookup by token hash works until revoked or expired.
- Analytics query returns sample rows and sample marker.
- Purge deletes only historical rows older than cutoff and leaves audit/admin/session tables intact.

- [ ] **Step 2: Write audit recorder tests first**

Create `backend/internal/audit/sqlite_test.go` with:

```go
func TestSQLiteRecorderPersistsAuditEvent(t *testing.T)
func TestSQLiteRecorderDoesNotRequireActor(t *testing.T)
```

The tests must assert metadata is stored as JSON and no plaintext password/token appears in the inserted test metadata.

- [ ] **Step 3: Run tests and confirm RED**

Run from `backend/`:

```bash
PATH="$HOME/.local/bin:$PATH" go test ./internal/store ./internal/audit
```

Expected: fail because store and SQLite recorder do not exist.

- [ ] **Step 4: Implement store types and methods**

Implement `backend/internal/store/store.go` with:

```go
type Store struct { DB *sql.DB }
type Admin struct { ID int64; Username string; PasswordHash string; Roles []rbac.Role; DisabledAt *time.Time; CreatedAt time.Time }
type Session struct { ID int64; AdminID int64; TokenHash string; ExpiresAt time.Time; RevokedAt *time.Time }
type AuditEvent struct { ID int64; ActorAdminUserID *int64; Action string; ResourceType string; ResourceID string; Outcome audit.Outcome; Message string; Metadata map[string]string; CreatedAt time.Time }
type HistoricalSession struct { ID int64; Username string; ConnectedAt time.Time; DisconnectedAt time.Time; DurationSeconds int64; BytesIn int64; BytesOut int64; Source string }
type AnalyticsSummary struct { TotalSessions int64; SampleSessions int64; TotalBytesIn int64; TotalBytesOut int64 }
```

Required methods:

```go
func New(db *sql.DB) *Store
func (s *Store) AdminCount(ctx context.Context) (int64, error)
func (s *Store) CreateFirstOwner(ctx context.Context, username string, passwordHash string) (Admin, error)
func (s *Store) FindAdminByUsername(ctx context.Context, username string) (Admin, bool, error)
func (s *Store) FindAdminBySessionTokenHash(ctx context.Context, tokenHash string, now time.Time) (Admin, bool, error)
func (s *Store) CreateSession(ctx context.Context, adminID int64, tokenHash string, expiresAt time.Time) (Session, error)
func (s *Store) RevokeSessionByTokenHash(ctx context.Context, tokenHash string, revokedAt time.Time) error
func (s *Store) RecentAuditEvents(ctx context.Context, limit int) ([]AuditEvent, error)
func (s *Store) RecordAudit(ctx context.Context, event AuditEvent) error
func (s *Store) HistoricalSessions(ctx context.Context, limit int) ([]HistoricalSession, error)
func (s *Store) AnalyticsSummary(ctx context.Context) (AnalyticsSummary, error)
func (s *Store) PurgeHistoricalBefore(ctx context.Context, cutoff time.Time) (int64, error)
```

Implementation rules:

- Use `QueryContext`, `QueryRowContext`, and `ExecContext`.
- Always close rows and check `rows.Err()`.
- Use parameterized SQL only.
- Wrap database errors with lowercase context.
- Distinguish not found with `(value, false, nil)`.
- `CreateFirstOwner` must use a transaction.

- [ ] **Step 5: Implement SQLite audit recorder**

Implement `backend/internal/audit/sqlite.go` with:

```go
type SQLiteRecorder struct { Store interface{ RecordAudit(context.Context, store.AuditEvent) error } }
func NewSQLiteRecorder(s *store.Store) SQLiteRecorder
func (r SQLiteRecorder) Record(ctx context.Context, event Event) error
```

Map `audit.Event` into `store.AuditEvent`. Preserve safe string metadata only.

- [ ] **Step 6: Run GREEN verification**

Run from `backend/`:

```bash
PATH="$HOME/.local/bin:$PATH" go test ./internal/store ./internal/audit
PATH="$HOME/.local/bin:$PATH" go test ./...
```

Expected: all backend tests pass.

Do not commit.

---

## Task 4: Auth and Dashboard Services

**Owner:** Backend API agent.

**Files:**
- Create: `backend/internal/app/auth_service.go`
- Create: `backend/internal/app/auth_service_test.go`
- Create: `backend/internal/app/dashboard_service.go`
- Create: `backend/internal/app/dashboard_service_test.go`
- Modify: `backend/internal/rbac/rbac.go`
- Modify: `backend/internal/rbac/rbac_test.go`

- [ ] **Step 1: Extend RBAC tests first**

Modify `backend/internal/rbac/rbac_test.go` to include purge permission behavior:

```go
func TestDefaultAuthorizerOwnerCanPurgeHistory(t *testing.T)
func TestDefaultAuthorizerViewerCannotPurgeHistory(t *testing.T)
```

Run:

```bash
PATH="$HOME/.local/bin:$PATH" go test ./internal/rbac
```

Expected: fail because purge permission is not defined.

- [ ] **Step 2: Add purge permission**

Modify `backend/internal/rbac/rbac.go`:

```go
const PermissionPurgeHistory Permission = "history:purge"
```

Grant it only to `RoleOwner`.

- [ ] **Step 3: Write auth service tests first**

Create `backend/internal/app/auth_service_test.go` with tests:

```go
func TestAuthServiceSetupOwnerSucceedsOnce(t *testing.T)
func TestAuthServiceLoginCreatesSessionAndAudits(t *testing.T)
func TestAuthServiceLoginFailureIsGenericAndAudited(t *testing.T)
func TestAuthServiceLogoutRevokesCurrentSession(t *testing.T)
```

Use the real in-memory SQLite store, real password hasher, and deterministic `time.Now` injection through a service clock function.

- [ ] **Step 4: Write dashboard service tests first**

Create `backend/internal/app/dashboard_service_test.go` with tests:

```go
func TestDashboardServiceSummaryUsesNoopOcservStatus(t *testing.T)
func TestDashboardServiceHistoryReturnsSampleMarker(t *testing.T)
func TestDashboardServicePurgeRequiresOwnerAndConfirmation(t *testing.T)
func TestDashboardServicePurgeDeletesHistoricalOnlyAndAudits(t *testing.T)
```

Viewer purge must return an authorization error and record a denied audit event. Owner purge with confirmation `PURGE` and retention days in `{30,90,180,365}` deletes eligible historical records.

- [ ] **Step 5: Run tests and confirm RED**

Run:

```bash
PATH="$HOME/.local/bin:$PATH" go test ./internal/app ./internal/rbac
```

Expected: fail because services are not implemented.

- [ ] **Step 6: Implement auth service**

Implement `backend/internal/app/auth_service.go` with:

```go
type AuthService struct { Store *store.Store; SessionTTL time.Duration; Now func() time.Time }
type SetupOwnerInput struct { Username string; Password string }
type LoginInput struct { Username string; Password string }
type LoginResult struct { Principal auth.Principal; Token string; ExpiresAt time.Time }

func (s *AuthService) SetupRequired(ctx context.Context) (bool, error)
func (s *AuthService) SetupOwner(ctx context.Context, input SetupOwnerInput) (auth.Principal, error)
func (s *AuthService) Login(ctx context.Context, input LoginInput) (LoginResult, error)
func (s *AuthService) CurrentPrincipal(ctx context.Context, token string) (auth.Principal, bool, error)
func (s *AuthService) Logout(ctx context.Context, token string) error
```

Validation rules:

- Username trim length between 3 and 64.
- Password length at least 12.
- Login failure returns a generic auth error for missing user, disabled user, or wrong password.
- Audit events never include plaintext passwords or session tokens.

- [ ] **Step 7: Implement dashboard service**

Implement `backend/internal/app/dashboard_service.go` with:

```go
type DashboardService struct { Store *store.Store; Ocserv ocserv.Manager; Authorizer rbac.Authorizer; Now func() time.Time }
type Summary struct { BackendStatus string; OcservStatus string; ActiveSessionCount int; RecentAudit []store.AuditEvent; Analytics store.AnalyticsSummary; SampleData bool }
type PurgeInput struct { Principal auth.Principal; RetentionDays int; Confirmation string }

func (s *DashboardService) Summary(ctx context.Context, principal auth.Principal) (Summary, error)
func (s *DashboardService) History(ctx context.Context, principal auth.Principal) ([]store.HistoricalSession, store.AnalyticsSummary, error)
func (s *DashboardService) PurgeHistory(ctx context.Context, input PurgeInput) (int64, error)
```

Rules:

- `OcservStatus` is `not_configured` when using the no-op manager or when the manager returns `ocserv.ErrNotConfigured`.
- Retention days must be exactly 30, 90, 180, or 365.
- Confirmation must be exactly `PURGE`.
- Purge must authorize `history:purge`.
- Purge audit metadata includes retention days and deleted row count only.

- [ ] **Step 8: Run GREEN verification**

Run:

```bash
PATH="$HOME/.local/bin:$PATH" go test ./internal/app ./internal/rbac
PATH="$HOME/.local/bin:$PATH" go test ./...
```

Expected: all backend tests pass.

Do not commit.

---

## Task 5: Backend HTTP API and Main Wiring

**Owner:** Backend API agent, followed by security/RBAC review.

**Files:**
- Modify: `backend/internal/server/server.go`
- Create: `backend/internal/server/app.go`
- Create: `backend/internal/server/json.go`
- Create: `backend/internal/server/auth_middleware.go`
- Create: `backend/internal/server/auth_handlers.go`
- Create: `backend/internal/server/auth_handlers_test.go`
- Create: `backend/internal/server/dashboard_handlers.go`
- Create: `backend/internal/server/dashboard_handlers_test.go`
- Modify: `backend/cmd/ocserv-panel/main.go`

- [ ] **Step 1: Write auth handler tests first**

Create `backend/internal/server/auth_handlers_test.go` with tests:

```go
func TestSetupStatusReportsRequired(t *testing.T)
func TestSetupOwnerCreatesAdminAndSetsNoSession(t *testing.T)
func TestSetupOwnerRefusesSecondAdmin(t *testing.T)
func TestLoginSetsHttpOnlySameSiteCookie(t *testing.T)
func TestMeRequiresSession(t *testing.T)
func TestLogoutClearsCookieAndRevokesSession(t *testing.T)
```

Use `httptest`, an in-memory migrated SQLite store, and the real services.

- [ ] **Step 2: Write dashboard handler tests first**

Create `backend/internal/server/dashboard_handlers_test.go` with tests:

```go
func TestDashboardSummaryRequiresAuthentication(t *testing.T)
func TestDashboardSummaryReturnsNotConfiguredOcservState(t *testing.T)
func TestHistoryAnalyticsRequiresAuthentication(t *testing.T)
func TestHistoryPurgeRequiresOwnerRole(t *testing.T)
func TestHistoryPurgeRequiresConfirmation(t *testing.T)
func TestHistoryPurgeOwnerSuccessReturnsDeletedCount(t *testing.T)
```

Viewer purge should return `403`. Missing session should return `401`.

- [ ] **Step 3: Run tests and confirm RED**

Run:

```bash
PATH="$HOME/.local/bin:$PATH" go test ./internal/server
```

Expected: fail because API handlers and app router are not implemented.

- [ ] **Step 4: Implement JSON helpers and app router**

Implement:

```go
type ErrorResponse struct { Error string `json:"error"` }
type App struct { Auth *app.AuthService; Dashboard *app.DashboardService; Cookie CookieConfig }
type CookieConfig struct { Name string; Secure bool }
func NewAppRouter(app App) http.Handler
func NewRouter() http.Handler
```

Keep `NewRouter()` as a health-only fallback for existing tests. `NewAppRouter` registers all `/api/...` routes and `/healthz`.

- [ ] **Step 5: Implement auth middleware**

Implement request context helpers:

```go
func PrincipalFromContext(ctx context.Context) (auth.Principal, bool)
func RequireAuth(next http.HandlerFunc) http.HandlerFunc
func RequirePermission(permission rbac.Permission, next http.HandlerFunc) http.HandlerFunc
```

Rules:

- Read the session cookie by configured name.
- Resolve principal with `AuthService.CurrentPrincipal`.
- Return JSON `401` for missing/invalid session.
- Return JSON `403` for missing permission.

- [ ] **Step 6: Implement API handlers**

Implement endpoint behavior exactly from the spec:

- `GET /api/health`
- `GET /api/setup/status`
- `POST /api/setup/owner`
- `POST /api/auth/login`
- `POST /api/auth/logout`
- `GET /api/auth/me`
- `GET /api/dashboard/summary`
- `GET /api/history/analytics`
- `POST /api/history/purge`

Do not add CORS. Reject unsupported methods with `405`.

- [ ] **Step 7: Wire main**

Modify `backend/cmd/ocserv-panel/main.go`:

- Read `OCSERV_PANEL_ADDR`, default `127.0.0.1:8080`.
- Read `OCSERV_PANEL_DB_DSN`, default `file:ocserv_panel.db?_pragma=foreign_keys(1)`.
- Read `OCSERV_PANEL_MIGRATIONS_DIR`, default `../migrations`.
- Read `OCSERV_PANEL_COOKIE_SECURE`, default false for local development.
- Open SQLite with `db.OpenSQLite`, apply migrations, create store/services, and serve `server.NewAppRouter`.

- [ ] **Step 8: Run GREEN verification and smoke API**

Run:

```bash
PATH="$HOME/.local/bin:$PATH" go test ./internal/server ./cmd/ocserv-panel
PATH="$HOME/.local/bin:$PATH" go test ./...
```

Expected: all backend tests pass.

Do not commit.

---

## Task 6: Frontend API Client, Auth States, Dashboard, History, and Purge UI

**Owner:** Frontend admin UX agent.

**Files:**
- Modify: `frontend/vite.config.ts`
- Create: `frontend/src/lib/api.ts`
- Create: `frontend/src/lib/auth.ts`
- Modify: `frontend/src/routes/+layout.svelte`
- Modify: `frontend/src/routes/+page.svelte`
- Modify: `frontend/src/routes/login/+page.svelte`
- Create: `frontend/src/routes/setup/+page.svelte`
- Modify: `frontend/src/routes/history/+page.svelte`
- Modify: `frontend/src/app.css`

- [ ] **Step 1: Configure same-origin dev proxy**

Modify `frontend/vite.config.ts`:

```ts
export default defineConfig({
  plugins: [sveltekit()],
  server: {
    proxy: {
      '/api': 'http://127.0.0.1:8080'
    }
  }
});
```

- [ ] **Step 2: Add typed API helpers**

Create `frontend/src/lib/api.ts` with:

```ts
export type ApiError = { error: string };
export async function apiGet<T>(path: string): Promise<T>;
export async function apiPost<T>(path: string, body?: unknown): Promise<T>;
```

Rules:

- Use same-origin `fetch(path, { credentials: 'same-origin' })`.
- Parse JSON responses.
- Throw `Error` with server error string or `HTTP <status>`.
- Do not use local storage or session storage.

- [ ] **Step 3: Add auth/data types and helpers**

Create `frontend/src/lib/auth.ts` exporting types and functions:

```ts
export type Principal = { id: number; username: string; roles: string[] };
export type SetupStatus = { setupRequired: boolean };
export type DashboardSummary = { backendStatus: string; ocservStatus: string; activeSessionCount: number; sampleData: boolean };
export type HistoricalSession = { id: number; username: string; connectedAt: string; disconnectedAt: string; durationSeconds: number; bytesIn: number; bytesOut: number; source: string };

export const getSetupStatus = () => apiGet<SetupStatus>('/api/setup/status');
export const getMe = () => apiGet<{ principal: Principal }>('/api/auth/me');
export const login = (username: string, password: string) => apiPost<{ principal: Principal }>('/api/auth/login', { username, password });
export const logout = () => apiPost<{ ok: true }>('/api/auth/logout');
```

- [ ] **Step 4: Implement first-run setup page**

Create `frontend/src/routes/setup/+page.svelte`:

- Form fields: username, password.
- Minimum password help text: 12 characters.
- Calls `POST /api/setup/owner`.
- On success, show message instructing operator to log in and provide link to `/login`.
- On error, show an alert.
- Do not store password in local state after submit completes; clear password variable.

- [ ] **Step 5: Implement real login page**

Modify `frontend/src/routes/login/+page.svelte`:

- Form fields: username, password.
- Calls `login` helper.
- On success, navigate to `/`.
- On error, show generic error from API.
- Clear password variable after submit.

- [ ] **Step 6: Implement auth-aware layout**

Modify `frontend/src/routes/+layout.svelte`:

- On mount, call `/api/setup/status` then `/api/auth/me` when setup is complete.
- If setup required, show a prominent setup link.
- If logged in, show username/roles and logout button.
- Logout calls `/api/auth/logout`, clears local principal state, and navigates to `/login`.
- Keep navigation visible but make current auth state clear.

- [ ] **Step 7: Implement dashboard page**

Modify `frontend/src/routes/+page.svelte`:

- If setup required, show setup call-to-action.
- If unauthenticated, show login call-to-action.
- If authenticated, fetch `/api/dashboard/summary` and render cards for backend status, ocserv status, active sessions, sample data state, and recent audit rows.

- [ ] **Step 8: Implement history and purge UI**

Modify `frontend/src/routes/history/+page.svelte`:

- Fetch `/api/history/analytics`.
- Show sample/demo warning when response reports sample data.
- Render records and aggregate cards.
- If current principal roles include `owner`, show purge panel with retention select `{30,90,180,365}` and confirmation input.
- Disable submit unless confirmation is exactly `PURGE`.
- On purge success, show deleted count and reload analytics.
- If not owner, show read-only note.

- [ ] **Step 9: Add styles**

Modify `frontend/src/app.css` for forms, cards, alerts, destructive zone, status pills, and tables. Do not import external CSS frameworks.

- [ ] **Step 10: Verify frontend**

Run from `frontend/`:

```bash
PATH="$HOME/.local/bin:$PATH" npm run check
PATH="$HOME/.local/bin:$PATH" npm run build
```

Expected: both commands pass. The existing adapter-auto warning during build is acceptable for this baseline until deployment packaging is finalized.

Do not commit.

---

## Task 7: Documentation Updates

**Owner:** Documentation/Ops agent.

**Files:**
- Modify: `docs/ADR-0001-architecture.md`
- Modify: `docs/OPERATIONS.md`
- Modify: `docs/SECURITY.md`

- [ ] **Step 1: Update architecture ADR**

Add notes that:

- The project considered PocketBase and intentionally kept the custom Go backend.
- The approved SQLite driver is `modernc.org/sqlite` to avoid CGO/system compiler requirements.
- The first real Web UI slice uses same-origin `/api` and SQLite-backed auth/session/audit/analytics.

- [ ] **Step 2: Update operations guide**

Document:

- Dev backend command: `OCSERV_PANEL_ADDR=127.0.0.1:8080 OCSERV_PANEL_DB_DSN='file:ocserv_panel.db?_pragma=foreign_keys(1)' OCSERV_PANEL_MIGRATIONS_DIR=../migrations go run ./cmd/ocserv-panel` from `backend/`.
- Dev frontend command: `npm run dev -- --host 127.0.0.1 --port 5173` from `frontend/`.
- First-run setup creates the initial owner in the browser.
- Sample historical analytics are demo data and may be purged by owner role.

- [ ] **Step 3: Update security guide**

Document:

- PBKDF2-HMAC-SHA256 password hashes with random salts and no plaintext password storage.
- Hashed session tokens in SQLite and `HttpOnly`, `SameSite=Lax` cookies.
- Owner-only historical analytics purge with confirmation and audit logging.
- No live ocserv/system/config/cert operations in this slice.

- [ ] **Step 4: Verify docs contain no unsafe instructions**

Run read-only searches from repo root:

```bash
grep -R "sudo\|systemctl\|ocpasswd\|occtl" docs deploy || true
```

Expected: matches are explanatory/future-boundary text only, not commands required for this slice.

Do not commit.

---

## Task 8: Security/RBAC Review and Fix Loop

**Owner:** Security/RBAC agent, read-only first; implementer fixes only requested issues.

**Files reviewed:**
- `backend/internal/auth/`
- `backend/internal/app/`
- `backend/internal/store/`
- `backend/internal/server/`
- `migrations/0001_initial.sql`
- `frontend/src/lib/`
- `frontend/src/routes/`

- [ ] **Step 1: Review security-sensitive behavior**

Check:

- No plaintext password/session token storage.
- PBKDF2 parameters match spec.
- Session cookies are `HttpOnly`, `SameSite=Lax`, and configurable `Secure`.
- Login failure does not disclose whether username exists.
- Dashboard/history require auth.
- Purge requires owner, exact `PURGE`, allowed retention days, and audit event.
- Viewer cannot purge through API even if UI is bypassed.
- No `os/exec`, `systemctl`, `sudo`, `occtl`, `ocpasswd`, or live ocserv socket calls.

- [ ] **Step 2: Fix required issues one at a time**

If the review requests changes, assign the relevant owning agent to fix them and rerun the specific tests for the touched package before re-review.

- [ ] **Step 3: Confirm approval**

Proceed only when the security/RBAC review returns no required fixes.

---

## Task 9: Full Verification

**Owner:** Test/verification agent.

**Files:** all changed files.

- [ ] **Step 1: Run backend tests**

Run from `backend/`:

```bash
PATH="$HOME/.local/bin:$PATH" go test ./...
```

Expected: all packages pass.

- [ ] **Step 2: Run frontend checks**

Run from `frontend/`:

```bash
PATH="$HOME/.local/bin:$PATH" npm run check
PATH="$HOME/.local/bin:$PATH" npm run build
```

Expected: `npm run check` has 0 errors and 0 warnings; `npm run build` passes. The adapter-auto warning is acceptable only if build exits 0.

- [ ] **Step 3: Run read-only safety searches**

Run from repo root using `rg` if available:

```bash
rg "os/exec|exec\.Command|CommandContext|child_process|node:fs|occtl|ocpasswd|systemctl|sudo" backend frontend migrations docs deploy || true
```

Expected: no source code command-execution or frontend filesystem access. Docs may mention future/prohibited boundaries.

- [ ] **Step 4: Run local smoke test without ocserv**

Run backend with a temp DB and call APIs:

```bash
tmpdb="$(mktemp -u /tmp/ocserv-panel-smoke-XXXXXX.db)"
PATH="$HOME/.local/bin:$PATH" OCSERV_PANEL_ADDR=127.0.0.1:18080 OCSERV_PANEL_DB_DSN="file:${tmpdb}?_pragma=foreign_keys(1)" OCSERV_PANEL_MIGRATIONS_DIR=../migrations go run ./cmd/ocserv-panel
```

In another shell, verify:

```bash
curl -sS http://127.0.0.1:18080/api/setup/status
```

Expected: JSON indicates setup is required. Stop the backend process after the smoke test.

Do not use live ocserv.

- [ ] **Step 5: Report verification evidence**

Return command outputs, failures, and limitations. Do not claim success without fresh outputs.

---

## Task 10: Final Code Review

**Owner:** Code review agent.

**Files:** all changed files.

- [ ] **Step 1: Review implementation against the approved spec**

Check all acceptance criteria in `docs/superpowers/specs/2026-04-29-authenticated-dashboard-design.md`.

- [ ] **Step 2: Review architecture drift**

Confirm:

- Custom Go backend remains the backend.
- No PocketBase, external service, queue, observability stack, cloud control plane, or container-first runtime was introduced.
- SQLite remains local persistence.
- Frontend calls backend APIs only.

- [ ] **Step 3: Review security-sensitive changes**

Confirm no Critical/High/Medium findings for auth/session/RBAC/audit/purge/password storage/secret handling/live command execution.

- [ ] **Step 4: Require fixes for any blocking finding**

If changes are requested, route them to the owning implementation agent and repeat affected verification before final handoff.

---

## Completion Notes

- Implementation changes remain uncommitted unless the user explicitly requests a commit.
- Final response must include created/modified structure, tests/checks run, known gaps, and how to run the Web UI without live ocserv.
- Future production work still needs final SvelteKit adapter/runtime decision, real ocserv integration, user lifecycle management, config safety workflow, certificate lifecycle controls, and deployment assets.
