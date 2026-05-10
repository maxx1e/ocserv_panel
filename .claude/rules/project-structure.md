# Go Backend Project Structure

**Module:** `ocserv_panel/backend`  
**Go version:** 1.25.0  
**Root:** `backend/`

---

## Package Map

```
backend/
├── cmd/ocserv-panel/        Entry point — wires all deps, starts HTTP server
└── internal/
    ├── app/                 Business logic services (one file per service)
    ├── audit/               Audit event types and recording interface
    ├── auth/                Principal type, password hashing, session tokens, rate limiter
    ├── config/              TOML app config (load, merge env vars)
    ├── db/                  SQLite connection setup and schema migrations
    ├── logging/             zerolog initialisation (file + stderr, SIGHUP reopen)
    ├── logs/                Log file / journal reading interface and implementations
    ├── metrics/             Time-series metric types, Store interface, TSDBStore, MemoryStore
    ├── monitoring/          Metric collectors (CPU, memory, network, sessions)
    ├── ocserv/              ocserv integration boundary
    │   ├── cmd/             CommandRunner interface + OSCommandRunner + FakeCommandRunner
    │   ├── config/          ConfigValidator, ConfigFileManager, ConfigReloader, RestartDetector
    │   ├── parse/           occtl JSON/text output parsers and result types
    │   └── service/         ServiceManager interface, SystemdManager, FakeServiceManager
    ├── rbac/                Role and permission definitions, Authorizer
    ├── sanitize/            Redact sensitive keys from metadata maps
    ├── server/              HTTP router (App struct), middleware, all request handlers
    ├── store/               SQLite repositories (admin users, sessions, audit, history, settings)
    └── validation/          Input validation rules (username, password, group, IP)
```

---

## Dependency Flow

Arrows point from importer → dependency. No cycles permitted.

```
server   → app, auth, rbac, config, metrics, ocserv, store
app      → store, audit, auth, rbac, config, ocserv, metrics, logs, monitoring, validation
audit    → store (via AuditStore interface), sanitize
store    → db
ocserv   → ocserv/cmd, ocserv/parse, ocserv/config, ocserv/service, validation
monitoring → metrics, ocserv (via SessionStatusProvider interface)
logging  → (zerolog only)
validation, sanitize, rbac → (stdlib only)
```

`audit` imports `store`; therefore `store` must never import `audit`. Break this cycle with an interface (`AuditStore`) defined in `audit`, implemented by `store.Store`.

---

## Key Types

### `auth.Principal`
```go
type Principal struct {
    ID        int64
    Username  string
    Roles     []string
    CreatedAt time.Time
}
```
Injected into HTTP context by `server.requirePermission` middleware; extracted with `principalFromContext(ctx)`.

### `rbac.Permission` constants (selected)
```
dashboard:read          settings:manage         settings:read
ocserv:read             ocserv:service:manage   ocserv:sessions:read
ocserv:sessions:disconnect  ocserv:sessions:terminate
ocserv:users:create     ocserv:users:delete     ocserv:users:update
ocserv:config:read      ocserv:config:edit      ocserv:bans:manage
admin:users:read        admin:users:create      admin:users:update
admin:users:delete      audit:logs:read         audit:logs:export
history:purge
```

`RoleOwner` holds all permissions. `RoleViewer` holds read-only subset (dashboard, settings, sessions read, admin users read, audit read).

### `audit.Event`
```go
type Event struct {
    ActorID      *int64
    Action       string            // e.g. "auth.login", "ocserv.sessions.terminate"
    ResourceType string            // e.g. "admin_session", "ocserv_session"
    ResourceID   string
    Outcome      Outcome           // "success" | "failure" | "denied"
    Message      string
    Metadata     map[string]string // redacted before persistence
}
```

### `ocserv.Manager` interface
Implemented by `OcctlAdapter` (production) and `FakeManager` (tests). All session and user inspection goes through this interface.

### `ocserv.PasswdManager` interface
Implemented by `OcpasswdAdapter` (production) and `FakePasswdManager` (tests). User lifecycle (create, delete, lock, unlock, update password) goes through this interface.

### `ocserv/service.ServiceManager` interface
Implemented by `SystemdManager` (production) and `FakeServiceManager` (tests). Start/stop/restart/status of the systemd unit.

### `ocserv/cmd.CommandRunner` interface
```go
type CommandRunner interface {
    Run(ctx context.Context, name string, args []string, stdin string) (CommandResult, error)
}
```
`OSCommandRunner` uses `exec.CommandContext`. **Always pass args separately — never interpolate into a shell string.**

---

## Design Principles

### 1. Every privileged operation requires three things
1. **RBAC check** via `s.authorizer.Allowed(principalRoles(principal), perm)` or `requirePermission` middleware
2. **Input validation** (via `internal/validation` for user input, or explicit guards)
3. **Audit record** via `s.recorder.Record(ctx, audit.Event{...})`

Missing any one of these is a security defect.

### 2. Service method signature
```go
func (s *FooService) DoThing(ctx context.Context, principal auth.Principal, ...) (Result, error)
```
- `ctx` first, `principal` second — always.
- Guard with `requirePerm` / `authorizer.Allowed` at the top of the method body.
- On `ErrUnauthenticated`: principal has zero value (`ID == 0`). Check `principal.ID == 0` before asserting a role.

### 3. App services own business logic; handlers own HTTP concerns
`internal/app/` services must not import `net/http`. Handlers must not contain business logic. The mapping is:
```
Handler: parse request → call service → write response
Service: validate input → check RBAC → execute → audit → return result
```

### 4. Thin services are not wanted
Do not create a service that only calls `authorizer.Allowed` and then delegates to another layer. If a handler can call the downstream directly and the RBAC is enforced by the `requirePermission` middleware, prefer the direct call. The middleware injects the principal via `principalFromContext`; use that.

### 5. Interface placement
Define interfaces where they are consumed, not where they are produced.
- `CommandRunner` lives in `ocserv/cmd` — the cmd package defines and implements it; callers import the interface.
- `ServiceManager` lives in `ocserv/service`.
- `LogReader` lives in `internal/logs`.

### 6. Shim files for backward compat
When a type moves to a sub-package, add a `*_compat.go` file in the old package with type aliases:
```go
type CommandRunner = cmd.CommandRunner
func NewOSCommandRunner() *cmd.OSCommandRunner { return cmd.NewOSCommandRunner() }
```
This allows incremental migration without a big-bang rename. Remove shims after all callers are updated.

### 7. Test doubles follow the `Fake*` naming convention
- Prefix: `Fake` (e.g. `FakeCommandRunner`, `FakePasswdManager`, `FakeServiceManager`)
- Live in the same package as the interface (`cmd/fake.go`, `service/fake.go`, `passwd_fake.go`)
- Thread-safe (use `sync.Mutex`)
- Expose call counters and an injectable error: `StartCount()`, `SetError(err)`
- Never use `testify/mock` — fakes are preferred

### 8. All I/O accepts `context.Context`
Database calls use `QueryContext`/`ExecContext`. Command runners use `exec.CommandContext`. Log reads accept `ctx`. This enables timeout propagation and graceful shutdown.

### 9. Time is always UTC
`time.Now().UTC()`. Times stored in SQLite as RFC3339 strings. API responses use RFC3339.

### 10. Never interpolate user input into shell commands
Use `exec.Command(name, arg1, arg2, ...)` — never `exec.Command("sh", "-c", "cmd "+userInput)`. The `CommandRunner.Run` signature enforces this: `name string, args []string` are separate.

---

## Service Constructor Pattern
```go
type FooService struct {
    store      FooStore
    recorder   audit.Recorder
    authorizer *rbac.Authorizer
}

func NewFooService(store FooStore, recorder audit.Recorder, authorizer *rbac.Authorizer) *FooService {
    if recorder == nil {
        recorder = audit.NoopRecorder{}
    }
    return &FooService{store: store, recorder: recorder, authorizer: authorizer}
}
```
- All dependencies injected; no package-level globals.
- Nil recorder → `NoopRecorder` (safe default; never crashes on nil dereference).
- `*rbac.Authorizer` is never nil in production; test code uses `rbac.NewDefaultAuthorizer()`.

---

## RBAC Guard Pattern
```go
func (s *FooService) DoThing(ctx context.Context, principal auth.Principal, id string) error {
    if principal.ID == 0 {
        return ErrUnauthenticated
    }
    if !s.authorizer.Allowed(principalRoles(principal), rbac.PermissionFooManage) {
        _ = s.recorder.Record(ctx, audit.Event{
            ActorID:      &principal.ID,
            Action:       "foo.do_thing",
            ResourceType: "foo",
            ResourceID:   id,
            Outcome:      audit.OutcomeDenied,
        })
        return ErrPermissionDenied
    }
    // ... business logic ...
}
```

Where `principalRoles` converts `[]string` → `[]rbac.Role`:
```go
func principalRoles(p auth.Principal) []rbac.Role {
    roles := make([]rbac.Role, len(p.Roles))
    for i, r := range p.Roles { roles[i] = rbac.Role(r) }
    return roles
}
```

---

## HTTP Middleware Pattern
Routes that require auth use `requirePermission` instead of inline auth checks:
```go
mux.Handle("/api/foo", app.requirePermission(rbac.PermissionFooRead)(http.HandlerFunc(app.handleFoo)))
```
Inside the handler:
```go
func (a *App) handleFoo(w http.ResponseWriter, r *http.Request) {
    if !allowMethod(w, r, http.MethodGet) { return }
    principal := principalFromContext(r.Context()) // guaranteed non-zero after middleware
    // call service directly
}
```
**Do not add inline `a.authenticate` + `a.authorizer.Allowed` inside handlers that are already behind `requirePermission`.**

---

## Error Conventions
| Layer | Pattern |
|---|---|
| Validation | Typed sentinel errors: `ErrUsernameEmpty`, `ErrPasswordTooLong`, ... |
| Services | `ErrUnauthenticated`, `ErrPermissionDenied`, `ErrNotFound`, `ErrConflict` |
| Store | Wrap with `fmt.Errorf("store.Foo: %w", err)` |
| Handlers | Map errors to HTTP status codes; never leak internal errors to the client |

Service errors must be distinguishable with `errors.Is`. Do not use `errors.New` for the same sentinel in multiple places.

---

## Audit Action Naming Convention
Format: `<domain>.<resource>.<verb>`

| Domain | Examples |
|---|---|
| `auth` | `auth.login`, `auth.logout`, `auth.setup` |
| `ocserv.sessions` | `ocserv.sessions.disconnect`, `ocserv.sessions.terminate` |
| `ocserv.users` | `ocserv.users.create`, `ocserv.users.delete` |
| `ocserv.service` | `ocserv.service.start`, `ocserv.service.stop` |
| `ocserv.config` | `ocserv.config.apply`, `ocserv.config.rollback` |
| `admin.users` | `admin.users.create`, `admin.users.delete` |
| `settings` | `settings.update` |
| `housekeeping` | `housekeeping.purge` |

Resource types are singular nouns: `admin_session`, `ocserv_session`, `admin_user`, `ocserv_user`, `config_file`, `setting`.

---

## Migrations
SQL migration files live in `migrations/`. Applied at startup via `db.ApplyMigrations`. Files must be deterministic and safe to run on an existing schema (use `CREATE TABLE IF NOT EXISTS`, `ALTER TABLE ... IF NOT EXISTS`). Never drop columns or tables in a migration without a confirmed data-loss plan.

---

## Testing Checklist
When adding a new service method, add tests for:
- [ ] Unauthenticated principal returns `ErrUnauthenticated`
- [ ] Viewer principal returns `ErrPermissionDenied` when permission requires owner
- [ ] Owner principal succeeds and produces the expected side effect
- [ ] Audit event is recorded with correct action, resource type, outcome
- [ ] Service error produces `OutcomeFailure` audit event (not `OutcomeSuccess`)

When adding a new HTTP handler, add tests for:
- [ ] Missing session cookie → 401
- [ ] Viewer cookie on owner-only route → 403
- [ ] Valid request → 200 with expected body shape
- [ ] Wrong HTTP method → 405

Use `newTestHarness(t)` in `internal/server/server_test.go` for handler tests. Use `newTestOcservService(t, ...)` in `internal/app/ocserv_service_test.go` for service tests.
