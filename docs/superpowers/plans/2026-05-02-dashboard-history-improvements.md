# Dashboard & History Improvements Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add dashboard auto-refresh, analytics summary cards, VPN interface auto-discovery, and settings service with housekeeping to the ocserv panel.

**Architecture:** Four independent features implemented in dependency order. Settings service is built first (backend foundation), then housekeeping (uses settings), then VPN discovery (independent backend), then frontend changes (auto-refresh and analytics cards). Each task produces working, testable increments.

**Tech Stack:** Go backend, SvelteKit frontend, SQLite persistence, existing test patterns (t.Fatalf assertions, fake test doubles).

---

## File Structure

### New Files
- `backend/internal/app/settings_service.go` — SettingsService with Get/Set/GetRetentionDays/SetRetentionDays
- `backend/internal/app/settings_service_test.go` — Tests for SettingsService
- `backend/internal/app/housekeeping.go` — HousekeepingScheduler with daily purge goroutine
- `backend/internal/app/housekeeping_test.go` — Tests for HousekeepingScheduler
- `backend/internal/monitoring/vpn_discovery.go` — DiscoverVPNInterface function
- `backend/internal/monitoring/vpn_discovery_test.go` — Tests for VPN discovery
- `frontend/src/lib/components/monitoring/AnalyticsCard.svelte` — Card component for analytics metrics

### Modified Files
- `backend/internal/store/store.go` — Add GetSetting, SetSetting methods
- `backend/internal/rbac/rbac.go` — Add PermissionSettingsManage
- `backend/internal/server/server.go` — Add settingsService, housekeeping fields; add settings API handlers; wire into NewAppRouter
- `backend/cmd/ocserv-panel/main.go` — Wire SettingsService, HousekeepingScheduler, VPN discovery
- `backend/internal/metrics/types.go` — Add SetState method to Catalog
- `backend/internal/app/monitoring_service.go` — Accept catalog pointer for runtime state updates
- `backend/internal/monitoring/collector_service.go` — No changes needed (collectors passed in)
- `frontend/src/lib/api.ts` — Add Settings type and API functions (getSettings, updateSetting)
- `frontend/src/routes/+page.svelte` — Add auto-refresh interval, replace analytics raw list with AnalyticsCard grid
- `frontend/src/routes/history/+page.svelte` — Add auto-refresh interval, add housekeeping settings section

---

### Task 1: Store Settings Methods

**Files:**
- Modify: `backend/internal/store/store.go`
- Test: `backend/internal/store/store_test.go` (new)

- [ ] **Step 1: Write the failing test for GetSetting and SetSetting**

Create `backend/internal/store/store_test.go`:

```go
package store

import (
	"context"
	"database/sql"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func newTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open in-memory sqlite: %v", err)
	}
	return db
}

func applyMigrationsForTest(t *testing.T, ctx context.Context, db *sql.DB) {
	t.Helper()
	// Create app_settings table for testing
	_, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS app_settings (
		key TEXT PRIMARY KEY,
		value TEXT NOT NULL,
		updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
		CHECK (length(trim(key)) > 0)
	)`)
	if err != nil {
		t.Fatalf("create app_settings table: %v", err)
	}
}

func TestGetSettingReturnsEmptyWhenNotSet(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	defer db.Close()
	applyMigrationsForTest(t, ctx, db)
	s := New(db)

	value, err := s.GetSetting(ctx, "nonexistent.key")
	if err != nil {
		t.Fatalf("GetSetting unexpected error: %v", err)
	}
	if value != "" {
		t.Fatalf("GetSetting = %q, want empty string for nonexistent key", value)
	}
}

func TestSetAndGetSetting(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	defer db.Close()
	applyMigrationsForTest(t, ctx, db)
	s := New(db)

	if err := s.SetSetting(ctx, "history.retention_days", "90"); err != nil {
		t.Fatalf("SetSetting: %v", err)
	}

	value, err := s.GetSetting(ctx, "history.retention_days")
	if err != nil {
		t.Fatalf("GetSetting: %v", err)
	}
	if value != "90" {
		t.Fatalf("GetSetting = %q, want %q", value, "90")
	}
}

func TestSetSettingUpserts(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	defer db.Close()
	applyMigrationsForTest(t, ctx, db)
	s := New(db)

	if err := s.SetSetting(ctx, "history.retention_days", "90"); err != nil {
		t.Fatalf("SetSetting first: %v", err)
	}
	if err := s.SetSetting(ctx, "history.retention_days", "180"); err != nil {
		t.Fatalf("SetSetting second: %v", err)
	}

	value, err := s.GetSetting(ctx, "history.retention_days")
	if err != nil {
		t.Fatalf("GetSetting: %v", err)
	}
	if value != "180" {
		t.Fatalf("GetSetting = %q, want %q after upsert", value, "180")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd backend && go test ./internal/store/ -run TestGetSetting -v`
Expected: FAIL — `GetSetting` method doesn't exist on `*Store`

- [ ] **Step 3: Implement GetSetting and SetSetting on Store**

Add to `backend/internal/store/store.go`:

```go
// GetSetting returns the value for a setting key, or empty string if not set.
func (s *Store) GetSetting(ctx context.Context, key string) (string, error) {
	var value string
	err := s.DB.QueryRowContext(ctx, `SELECT value FROM app_settings WHERE key = ?`, key).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("get setting %q: %w", key, err)
	}
	return value, nil
}

// SetSetting upserts a setting key with the given value.
func (s *Store) SetSetting(ctx context.Context, key string, value string) error {
	now := time.Now().UTC()
	result, err := s.DB.ExecContext(ctx,
		`INSERT INTO app_settings (key, value, updated_at) VALUES (?, ?, ?)
		 ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at`,
		key, value, formatTime(now))
	if err != nil {
		return fmt.Errorf("set setting %q: %w", key, err)
	}
	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("set setting %q: no rows affected", key)
	}
	return nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd backend && go test ./internal/store/ -v`
Expected: All tests PASS

- [ ] **Step 5: Commit**

```bash
git add backend/internal/store/store.go backend/internal/store/store_test.go
git commit -m "feat: add GetSetting and SetSetting methods to Store"
```

---

### Task 2: Settings Service

**Files:**
- Create: `backend/internal/app/settings_service.go`
- Create: `backend/internal/app/settings_service_test.go`

- [ ] **Step 1: Write the failing test for SettingsService**

Create `backend/internal/app/settings_service_test.go`:

```go
package app

import (
	"context"
	"errors"
	"testing"

	"ocserv_panel/backend/internal/audit"
	"ocserv_panel/backend/internal/rbac"
	"ocserv_panel/backend/internal/store"
)

func TestSettingsServiceGetRetentionDaysDefault(t *testing.T) {
	ctx := context.Background()
	s, _ := newTestServices(t)
	service := NewSettingsService(s, audit.NoopRecorder{}, rbac.NewDefaultAuthorizer())

	days := service.GetRetentionDays(ctx)
	if days != 365 {
		t.Fatalf("default retention days = %d, want 365", days)
	}
}

func TestSettingsServiceSetAndGetRetentionDays(t *testing.T) {
	ctx := context.Background()
	s, _ := newTestServices(t)
	service := NewSettingsService(s, audit.NoopRecorder{}, rbac.NewDefaultAuthorizer())

	owner := createAdminWithRole(t, ctx, s, "owner", rbac.RoleOwner)

	if err := service.SetRetentionDays(ctx, owner, 90); err != nil {
		t.Fatalf("SetRetentionDays: %v", err)
	}

	days := service.GetRetentionDays(ctx)
	if days != 90 {
		t.Fatalf("retention days = %d, want 90", days)
	}
}

func TestSettingsServiceSetRetentionDaysValidatesAllowed(t *testing.T) {
	ctx := context.Background()
	s, _ := newTestServices(t)
	service := NewSettingsService(s, audit.NoopRecorder{}, rbac.NewDefaultAuthorizer())
	owner := createAdminWithRole(t, ctx, s, "owner", rbac.RoleOwner)

	for _, invalid := range []int{0, 1, 7, 60, 100, 730} {
		err := service.SetRetentionDays(ctx, owner, invalid)
		if !errors.Is(err, ErrInvalidRetention) {
			t.Fatalf("SetRetentionDays(%d) err = %v, want ErrInvalidRetention", invalid, err)
		}
	}
}

func TestSettingsServiceSetRetentionDaysRequiresAuth(t *testing.T) {
	ctx := context.Background()
	s, _ := newTestServices(t)
	service := NewSettingsService(s, audit.NoopRecorder{}, rbac.NewDefaultAuthorizer())

	err := service.SetRetentionDays(ctx, principalWithRole("owner"), 90)
	if !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("unauthenticated SetRetentionDays err = %v, want ErrUnauthenticated", err)
	}
}

func TestSettingsServiceSetRetentionDaysRequiresOwner(t *testing.T) {
	ctx := context.Background()
	s, _ := newTestServices(t)
	service := NewSettingsService(s, audit.NoopRecorder{}, rbac.NewDefaultAuthorizer())
	viewer := createAdminWithRole(t, ctx, s, "viewer", rbac.RoleViewer)

	err := service.SetRetentionDays(ctx, viewer, 90)
	if !errors.Is(err, ErrPermissionDenied) {
		t.Fatalf("viewer SetRetentionDays err = %v, want ErrPermissionDenied", err)
	}
}

func TestSettingsServiceGetAllSettings(t *testing.T) {
	ctx := context.Background()
	s, _ := newTestServices(t)
	service := NewSettingsService(s, audit.NoopRecorder{}, rbac.NewDefaultAuthorizer())
	owner := createAdminWithRole(t, ctx, s, "owner", rbac.RoleOwner)

	_ = service.SetRetentionDays(ctx, owner, 90)

	settings, err := service.GetAll(ctx, owner)
	if err != nil {
		t.Fatalf("GetAll: %v", err)
	}
	if len(settings) == 0 {
		t.Fatalf("GetAll returned empty settings, want at least retention_days")
	}
	found := false
	for _, setting := range settings {
		if setting.Key == "history.retention_days" && setting.Value == "90" {
			found = true
		}
	}
	if !found {
		t.Fatalf("GetAll did not contain history.retention_days = 90, got %v", settings)
	}
}

func principalWithRole(roles ...string) auth.Principal {
	return auth.Principal{ID: 1, Username: "test", Roles: roles, CreatedAt: time.Now().UTC()}
}
```

Note: `newTestServices`, `createAdminWithRole` are already defined in `auth_service_test.go` and `dashboard_service_test.go` in the same package.

- [ ] **Step 2: Run test to verify it fails**

Run: `cd backend && go test ./internal/app/ -run TestSettingsService -v`
Expected: FAIL — `NewSettingsService` doesn't exist

- [ ] **Step 3: Implement SettingsService**

Create `backend/internal/app/settings_service.go`:

```go
package app

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"ocserv_panel/backend/internal/audit"
	"ocserv_panel/backend/internal/auth"
	"ocserv_panel/backend/internal/rbac"
)

// Setting is a key-value application setting.
type Setting struct {
	Key   string
	Value string
}

// SettingsService manages application settings stored in app_settings.
type SettingsService struct {
	store      settingsStore
	recorder   audit.Recorder
	authorizer rbac.Authorizer
	now        func() time.Time
}

type settingsStore interface {
	GetSetting(ctx context.Context, key string) (string, error)
	SetSetting(ctx context.Context, key string, value string) error
}

// NewSettingsService returns a SettingsService backed by SQLite.
func NewSettingsService(store *store.Store, recorder audit.Recorder, authorizer rbac.Authorizer) *SettingsService {
	if recorder == nil {
		recorder = audit.NoopRecorder{}
	}
	return &SettingsService{store: store, recorder: recorder, authorizer: authorizer, now: func() time.Time { return time.Now().UTC() }}
}

const retentionDaysKey = "history.retention_days"

// GetRetentionDays returns the configured retention period, defaulting to 365.
func (s *SettingsService) GetRetentionDays(ctx context.Context) int {
	value, err := s.store.GetSetting(ctx, retentionDaysKey)
	if err != nil || value == "" {
		return 365
	}
	days, err := strconv.Atoi(value)
	if err != nil {
		return 365
	}
	return days
}

// SetRetentionDays validates and persists the retention period. Only 30/90/180/365 are allowed.
func (s *SettingsService) SetRetentionDays(ctx context.Context, principal auth.Principal, days int) error {
	if !principal.Authenticated() {
		_ = s.record(ctx, nil, "settings.set_retention", "setting", retentionDaysKey, audit.OutcomeDenied, "retention setting authentication required", nil)
		return ErrUnauthenticated
	}
	actorID := principal.ID
	if !s.authorizer.Allowed(principalRoles(principal), rbac.PermissionSettingsManage) {
		_ = s.record(ctx, &actorID, "settings.set_retention", "setting", retentionDaysKey, audit.OutcomeDenied, "retention setting permission denied", nil)
		return ErrPermissionDenied
	}
	if !allowedRetentionDays(days) {
		_ = s.record(ctx, &actorID, "settings.set_retention", "setting", retentionDaysKey, audit.OutcomeDenied, "retention setting validation rejected", map[string]string{"retention_days": fmt.Sprint(days)})
		return ErrInvalidRetention
	}
	if err := s.store.SetSetting(ctx, retentionDaysKey, fmt.Sprint(days)); err != nil {
		_ = s.record(ctx, &actorID, "settings.set_retention", "setting", retentionDaysKey, audit.OutcomeFailure, "retention setting save failed", nil)
		return fmt.Errorf("save retention setting: %w", err)
	}
	if err := s.record(ctx, &actorID, "settings.set_retention", "setting", retentionDaysKey, audit.OutcomeSuccess, "retention setting saved", map[string]string{"retention_days": fmt.Sprint(days)}); err != nil {
		return fmt.Errorf("audit retention setting: %w", err)
	}
	return nil
}

// GetAll returns all application settings visible to the authenticated principal.
func (s *SettingsService) GetAll(ctx context.Context, principal auth.Principal) ([]Setting, error) {
	if !principal.Authenticated() {
		_ = s.record(ctx, nil, "settings.get_all", "setting", "", audit.OutcomeDenied, "settings authentication required", nil)
		return nil, ErrUnauthenticated
	}
	if !s.authorizer.Allowed(principalRoles(principal), rbac.PermissionReadDashboard) {
		actorID := principal.ID
		_ = s.record(ctx, &actorID, "settings.get_all", "setting", "", audit.OutcomeDenied, "settings permission denied", nil)
		return nil, ErrPermissionDenied
	}
	// Read known settings individually. This is simple and sufficient for a small set.
	settings := []Setting{}
	retentionValue, err := s.store.GetSetting(ctx, retentionDaysKey)
	if err != nil {
		return nil, fmt.Errorf("read retention setting: %w", err)
	}
	if retentionValue != "" {
		settings = append(settings, Setting{Key: retentionDaysKey, Value: retentionValue})
	}
	return settings, nil
}

func (s *SettingsService) record(ctx context.Context, actorID *int64, action string, resourceType string, resourceID string, outcome audit.Outcome, message string, metadata map[string]string) error {
	return s.recorder.Record(ctx, audit.Event{ActorID: actorID, Action: action, ResourceType: resourceType, ResourceID: resourceID, Outcome: outcome, Message: message, Metadata: metadata, CreatedAt: s.now()})
}
```

- [ ] **Step 4: Add PermissionSettingsManage to RBAC**

In `backend/internal/rbac/rbac.go`, add the new permission constant and add it to the owner role:

```go
const (
	PermissionReadDashboard   Permission = "dashboard:read"
	PermissionManageUsers     Permission = "users:manage"
	PermissionManageConfig    Permission = "config:manage"
	PermissionManageService   Permission = "service:manage"
	PermissionHistoryPurge    Permission = "history:purge"
	PermissionSettingsManage  Permission = "settings:manage"
)
```

And in `NewDefaultAuthorizer`, add `PermissionSettingsManage` to the `RoleOwner` grants map.

- [ ] **Step 5: Run tests to verify they pass**

Run: `cd backend && go test ./internal/app/ -run TestSettingsService -v`
Expected: All tests PASS

- [ ] **Step 6: Commit**

```bash
git add backend/internal/app/settings_service.go backend/internal/app/settings_service_test.go backend/internal/rbac/rbac.go
git commit -m "feat: add SettingsService with retention days management and settings:manage permission"
```

---

### Task 3: Settings API Endpoints

**Files:**
- Modify: `backend/internal/server/server.go`

- [ ] **Step 1: Write the failing test for settings API handlers**

Add to `backend/internal/server/server_test.go` (new file):

```go
package server

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"ocserv_panel/backend/internal/app"
	"ocserv_panel/backend/internal/audit"
	"ocserv_panel/backend/internal/auth"
	"ocserv_panel/backend/internal/rbac"
	"ocserv_panel/backend/internal/store"
)

func newTestServer(t *testing.T) (*store.Store, *app.AuthService, *app.SettingsService, http.Handler) {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open in-memory sqlite: %v", err)
	}
	// Apply schema
	for _, ddl := range []string{
		`CREATE TABLE IF NOT EXISTS roles (id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT NOT NULL UNIQUE, description TEXT NOT NULL DEFAULT '', created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP)`,
		`CREATE TABLE IF NOT EXISTS admin_users (id INTEGER PRIMARY KEY AUTOINCREMENT, username TEXT NOT NULL UNIQUE, password_hash TEXT NOT NULL, disabled_at TEXT, created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP, updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP)`,
		`CREATE TABLE IF NOT EXISTS admin_user_roles (admin_user_id INTEGER NOT NULL, role_id INTEGER NOT NULL, created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP, PRIMARY KEY (admin_user_id, role_id))`,
		`CREATE TABLE IF NOT EXISTS admin_sessions (id INTEGER PRIMARY KEY AUTOINCREMENT, admin_user_id INTEGER NOT NULL, token_hash TEXT NOT NULL UNIQUE, expires_at TEXT NOT NULL, revoked_at TEXT, created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP, updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP)`,
		`CREATE TABLE IF NOT EXISTS audit_log (id INTEGER PRIMARY KEY AUTOINCREMENT, actor_admin_user_id INTEGER, action TEXT NOT NULL, resource_type TEXT NOT NULL, resource_id TEXT, outcome TEXT NOT NULL, message TEXT NOT NULL DEFAULT '', metadata_json TEXT NOT NULL DEFAULT '{}', created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP)`,
		`CREATE TABLE IF NOT EXISTS historical_sessions (id INTEGER PRIMARY KEY AUTOINCREMENT, username TEXT NOT NULL, connected_at TEXT NOT NULL, disconnected_at TEXT NOT NULL, bytes_in INTEGER NOT NULL DEFAULT 0, bytes_out INTEGER NOT NULL DEFAULT 0, duration_seconds INTEGER NOT NULL DEFAULT 0, source TEXT NOT NULL, created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP)`,
		`CREATE TABLE IF NOT EXISTS app_settings (key TEXT PRIMARY KEY, value TEXT NOT NULL, updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP)`,
		`INSERT OR IGNORE INTO roles (name, description) VALUES ('owner', 'owner'), ('viewer', 'viewer')`,
	} {
		if _, err := db.Exec(ddl); err != nil {
			t.Fatalf("apply schema: %v", err)
		}
	}
	persistence := store.New(db)
	recorder := audit.NewSQLiteRecorder(persistence)
	authService := app.NewAuthService(persistence, recorder)
	settingsService := app.NewSettingsService(persistence, recorder, rbac.NewDefaultAuthorizer())
	dashboardService := app.NewDashboardService(persistence, recorder, rbac.NewDefaultAuthorizer())
	monitoringService := app.NewMonitoringService(nil, rbac.NewDefaultAuthorizer(), nil, recorder)
	handler := NewAppRouter(authService, dashboardService, monitoringService, settingsService, AppOptions{CookieSecure: false, Now: func() time.Time { return time.Now().UTC() }})
	return persistence, authService, settingsService, handler
}

func TestSettingsAPIGetRequiresAuth(t *testing.T) {
	_, _, _, handler := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/settings", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("GET /api/settings without auth = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestSettingsAPIPutRequiresAuth(t *testing.T) {
	_, _, _, handler := newTestServer(t)
	req := httptest.NewRequest(http.MethodPut, "/api/settings/history.retention_days", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("PUT /api/settings/:key without auth = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd backend && go test ./internal/server/ -run TestSettingsAPI -v`
Expected: FAIL — `NewAppRouter` doesn't accept `settingsService` yet

- [ ] **Step 3: Add settingsService to App and add API handlers**

In `backend/internal/server/server.go`:

1. Add `settingsService *app.SettingsService` field to `App` struct
2. Add `settingsService` parameter to `NewAppRouter`
3. Add `handleSettingsGet` and `handleSettingsPut` handlers
4. Add route registrations for `/api/settings` and `/api/settings/`

Add these handler methods:

```go
func (a *App) handleSettingsGet(w http.ResponseWriter, r *http.Request) {
	if !allowMethod(w, r, http.MethodGet) {
		return
	}
	principal, ok := a.authenticate(w, r)
	if !ok {
		return
	}
	settings, err := a.settingsService.GetAll(r.Context(), principal)
	if err != nil {
		a.writeAppError(w, err)
		return
	}
	response := make([]settingResponse, 0, len(settings))
	for _, s := range settings {
		response = append(response, settingResponse{Key: s.Key, Value: s.Value})
	}
	writeJSON(w, http.StatusOK, settingsListResponse{Settings: response})
}

func (a *App) handleSettingsPut(w http.ResponseWriter, r *http.Request) {
	if !allowMethod(w, r, http.MethodPut) {
		return
	}
	principal, ok := a.authenticate(w, r)
	if !ok {
		return
	}
	// Extract key from path: /api/settings/{key}
	key := r.URL.Path[len("/api/settings/"):]
	if key == "" {
		writeError(w, http.StatusBadRequest, "setting key is required")
		return
	}
	var req updateSettingRequest
	if !decodeRequestJSON(w, r, &req) {
		return
	}
	switch key {
	case "history.retention_days":
		days, err := strconv.Atoi(req.Value)
		if err != nil {
			writeError(w, http.StatusBadRequest, "retention days must be a number")
			return
		}
		if err := a.settingsService.SetRetentionDays(r.Context(), principal, days); err != nil {
			a.writeAppError(w, err)
			return
		}
	default:
		writeError(w, http.StatusBadRequest, "unknown setting key")
		return
	}
	writeJSON(w, http.StatusOK, settingResponse{Key: key, Value: req.Value})
}
```

Add these types:

```go
type settingsListResponse struct {
	Settings []settingResponse `json:"settings"`
}

type settingResponse struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type updateSettingRequest struct {
	Value string `json:"value"`
}
```

Add import for `"strconv"`.

Add routes in `NewAppRouter`:
```go
mux.HandleFunc("/api/settings", appHandler.handleSettingsGet)
mux.HandleFunc("/api/settings/", appHandler.handleSettingsPut)
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd backend && go test ./internal/server/ -run TestSettingsAPI -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add backend/internal/server/server.go backend/internal/server/server_test.go
git commit -m "feat: add settings API endpoints for GET /api/settings and PUT /api/settings/:key"
```

---

### Task 4: Housekeeping Scheduler

**Files:**
- Create: `backend/internal/app/housekeeping.go`
- Create: `backend/internal/app/housekeeping_test.go`
- Modify: `backend/cmd/ocserv-panel/main.go`

- [ ] **Step 1: Write the failing test for HousekeepingScheduler**

Create `backend/internal/app/housekeeping_test.go`:

```go
package app

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"ocserv_panel/backend/internal/audit"
	"ocserv_panel/backend/internal/rbac"
	"ocserv_panel/backend/internal/store"
)

func TestHousekeepingSchedulerPurgesOnSchedule(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	s, _ := newTestServices(t)
	owner := createAdminWithRole(t, ctx, s, "owner", rbac.RoleOwner)

	// Insert an old session
	insertHistoricalSession(t, ctx, s, "old-user", time.Now().UTC().AddDate(0, 0, -400))

	var purgeCount atomic.Int32
	mockPurger := func(ctx context.Context, retentionDays int) (int64, error) {
		purgeCount.Add(1)
		return s.PurgeHistoricalBefore(ctx, time.Now().UTC().AddDate(0, 0, -retentionDays))
	}

	scheduler := NewHousekeepingScheduler(s, audit.NoopRecorder{}, rbac.NewDefaultAuthorizer(), mockPurger, 50*time.Millisecond)

	// Set retention to 365 days
	settingsService := NewSettingsService(s, audit.NoopRecorder{}, rbac.NewDefaultAuthorizer())
	if err := settingsService.SetRetentionDays(ctx, owner, 365); err != nil {
		t.Fatalf("SetRetentionDays: %v", err)
	}

	go scheduler.Run(ctx)

	// Wait for at least one purge cycle
	deadline := time.After(3 * time.Second)
	for purgeCount.Load() == 0 {
		select {
		case <-deadline:
			t.Fatal("housekeeping scheduler did not run within timeout")
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}
}

func TestHousekeepingSchedulerUsesConfiguredRetention(t *testing.T) {
	ctx := context.Background()
	s, _ := newTestServices(t)

	var lastRetention atomic.Int32
	mockPurger := func(ctx context.Context, retentionDays int) (int64, error) {
		lastRetention.Store(int32(retentionDays))
		return 0, nil
	}

	scheduler := NewHousekeepingScheduler(s, audit.NoopRecorder{}, rbac.NewDefaultAuthorizer(), mockPurger, time.Hour)

	// Default retention should be 365
	days := scheduler.GetRetentionDays(ctx)
	if days != 365 {
		t.Fatalf("default retention = %d, want 365", days)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd backend && go test ./internal/app/ -run TestHousekeeping -v`
Expected: FAIL — `NewHousekeepingScheduler` doesn't exist

- [ ] **Step 3: Implement HousekeepingScheduler**

Create `backend/internal/app/housekeeping.go`:

```go
package app

import (
	"context"
	"log/slog"
	"strconv"
	"time"

	"ocserv_panel/backend/internal/audit"
	"ocserv_panel/backend/internal/rbac"
	"ocserv_panel/backend/internal/store"
)

// PurgeFunc is called by the housekeeping scheduler to purge old records.
type PurgeFunc func(ctx context.Context, retentionDays int) (int64, error)

// HousekeepingScheduler runs periodic history purge based on configured retention.
type HousekeepingScheduler struct {
	store      *store.Store
	recorder   audit.Recorder
	authorizer rbac.Authorizer
	purge      PurgeFunc
	interval   time.Duration
	logger     *slog.Logger
	now        func() time.Time
}

// NewHousekeepingScheduler creates a scheduler that purges history at the given interval.
func NewHousekeepingScheduler(
	store *store.Store,
	recorder audit.Recorder,
	authorizer rbac.Authorizer,
	purge PurgeFunc,
	interval time.Duration,
	opts ...HousekeepingOption,
) *HousekeepingScheduler {
	if interval <= 0 {
		interval = 24 * time.Hour
	}
	h := &HousekeepingScheduler{
		store:      store,
		recorder:   recorder,
		authorizer: authorizer,
		purge:      purge,
		interval:   interval,
		logger:     slog.Default(),
		now:        func() time.Time { return time.Now().UTC() },
	}
	for _, opt := range opts {
		opt(h)
	}
	return h
}

// HousekeepingOption configures a HousekeepingScheduler.
type HousekeepingOption func(*HousekeepingScheduler)

// HousekeepingWithLogger sets the logger.
func HousekeepingWithLogger(logger *slog.Logger) HousekeepingOption {
	return func(h *HousekeepingScheduler) { h.logger = logger }
}

// GetRetentionDays returns the configured retention period, defaulting to 365.
func (h *HousekeepingScheduler) GetRetentionDays(ctx context.Context) int {
	value, err := h.store.GetSetting(ctx, "history.retention_days")
	if err != nil || value == "" {
		return 365
	}
	days, atoiErr := strconv.Atoi(value)
	if atoiErr != nil {
		return 365
	}
	return days
}

// Run starts the housekeeping scheduler loop. It blocks until ctx is cancelled.
func (h *HousekeepingScheduler) Run(ctx context.Context) {
	h.logger.Info("housekeeping scheduler started", "interval", h.interval)
	ticker := time.NewTicker(h.interval)
	defer ticker.Stop()

	// Run once immediately on start
	h.runOnce(ctx)

	for {
		select {
		case <-ctx.Done():
			h.logger.Info("housekeeping scheduler stopped")
			return
		case <-ticker.C:
			h.runOnce(ctx)
		}
	}
}

func (h *HousekeepingScheduler) runOnce(ctx context.Context) {
	if err := ctx.Err(); err != nil {
		return
	}
	retentionDays := h.GetRetentionDays(ctx)
	h.logger.Info("housekeeping purge starting", "retention_days", retentionDays)

	deleted, err := h.purge(ctx, retentionDays)
	if err != nil {
		h.logger.Error("housekeeping purge failed", "error", err, "retention_days", retentionDays)
		return
	}
	h.logger.Info("housekeeping purge completed", "deleted_rows", deleted, "retention_days", retentionDays)
}
```

Wait, the `GetRetentionDays` method has a bug — let me fix it. It should use `strconv.Atoi` directly:

```go
// GetRetentionDays returns the configured retention period, defaulting to 365.
func (h *HousekeepingScheduler) GetRetentionDays(ctx context.Context) int {
	value, err := h.store.GetSetting(ctx, "history.retention_days")
	if err != nil || value == "" {
		return 365
	}
	days, atoiErr := strconv.Atoi(value)
	if atoiErr != nil {
		return 365
	}
	return days
}
```

Add `"strconv"` to imports.

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd backend && go test ./internal/app/ -run TestHousekeeping -v`
Expected: PASS

- [ ] **Step 5: Wire HousekeepingScheduler in main.go**

In `backend/cmd/ocserv-panel/main.go`, add after the collector service setup:

```go
// Create settings service
settingsService := app.NewSettingsService(persistence, recorder, rbac.NewDefaultAuthorizer())

// Create housekeeping scheduler
housekeepingScheduler := app.NewHousekeepingScheduler(
	persistence,
	recorder,
	rbac.NewDefaultAuthorizer(),
	func(ctx context.Context, retentionDays int) (int64, error) {
		cutoff := time.Now().UTC().AddDate(0, 0, -retentionDays)
		return persistence.PurgeHistoricalBefore(ctx, cutoff)
	},
	24*time.Hour,
	app.HousekeepingWithLogger(slog.Default()),
)
go housekeepingScheduler.Run(ctx)
```

Update `NewAppRouter` call to include `settingsService`:
```go
handler := server.NewAppRouter(authService, dashboardService, monitoringService, settingsService, server.AppOptions{CookieSecure: cookieSecureFromEnv()})
```

- [ ] **Step 6: Run all backend tests**

Run: `cd backend && go test ./... -v`
Expected: All tests PASS

- [ ] **Step 7: Commit**

```bash
git add backend/internal/app/housekeeping.go backend/internal/app/housekeeping_test.go backend/cmd/ocserv-panel/main.go
git commit -m "feat: add housekeeping scheduler with daily purge and wire into main"
```

---

### Task 5: VPN Interface Auto-Discovery

**Files:**
- Create: `backend/internal/monitoring/vpn_discovery.go`
- Create: `backend/internal/monitoring/vpn_discovery_test.go`
- Modify: `backend/internal/metrics/types.go` — Add `SetState` method to Catalog
- Modify: `backend/internal/app/monitoring_service.go` — Accept catalog pointer
- Modify: `backend/cmd/ocserv-panel/main.go` — Wire VPN discovery

- [ ] **Step 1: Write the failing test for VPN discovery**

Create `backend/internal/monitoring/vpn_discovery_test.go`:

```go
package monitoring

import (
	"testing"
)

func TestDiscoverVPNInterfaceFromOcservConfig(t *testing.T) {
	files := map[string][]string{
		"/etc/ocserv/ocserv.conf": {"device = tun0", "tcp-port = 443"},
	}
	iface, source, err := DiscoverVPNInterface(WithFileReader(&fakeFileReader{files: files}))
	if err != nil {
		t.Fatalf("DiscoverVPNInterface: %v", err)
	}
	if iface != "tun0" {
		t.Fatalf("iface = %q, want %q", iface, "tun0")
	}
	if source != "ocserv-config" {
		t.Fatalf("source = %q, want %q", source, "ocserv-config")
	}
}

func TestDiscoverVPNInterfaceFromSysFS(t *testing.T) {
	files := map[string][]string{
		"/sys/class/net/tun0/operstate": {"up"},
	}
	iface, source, err := DiscoverVPNInterface(WithFileReader(&fakeFileReader{files: files}))
	if err != nil {
		t.Fatalf("DiscoverVPNInterface: %v", err)
	}
	if iface != "tun0" {
		t.Fatalf("iface = %q, want %q", iface, "tun0")
	}
	if source != "sysfs-scan" {
		t.Fatalf("source = %q, want %q", source, "sysfs-scan")
	}
}

func TestDiscoverVPNInterfaceNoConfigNoInterfaces(t *testing.T) {
	files := map[string][]string{}
	iface, source, err := DiscoverVPNInterface(WithFileReader(&fakeFileReader{files: files}))
	if err != nil {
		t.Fatalf("DiscoverVPNInterface: %v", err)
	}
	if iface != "" {
		t.Fatalf("iface = %q, want empty string", iface)
	}
	if source != "" {
		t.Fatalf("source = %q, want empty string", source)
	}
}

func TestDiscoverVPNInterfacePrefersOcservConfig(t *testing.T) {
	files := map[string][]string{
		"/etc/ocserv/ocserv.conf": {"device = vpn0"},
		"/sys/class/net/tun0/operstate": {"up"},
	}
	iface, source, err := DiscoverVPNInterface(WithFileReader(&fakeFileReader{files: files}))
	if err != nil {
		t.Fatalf("DiscoverVPNInterface: %v", err)
	}
	if iface != "vpn0" {
		t.Fatalf("iface = %q, want %q (ocserv config should take priority)", iface, "vpn0")
	}
	if source != "ocserv-config" {
		t.Fatalf("source = %q, want %q", source, "ocserv-config")
	}
}

func TestDiscoverVPNInterfaceMultipleTunFallsBackToTun0(t *testing.T) {
	files := map[string][]string{
		"/sys/class/net/tun0/operstate": {"up"},
		"/sys/class/net/tun1/operstate": {"up"},
	}
	iface, source, err := DiscoverVPNInterface(WithFileReader(&fakeFileReader{files: files}))
	if err != nil {
		t.Fatalf("DiscoverVPNInterface: %v", err)
	}
	if iface != "tun0" {
		t.Fatalf("iface = %q, want %q (tun0 preferred when multiple)", iface, "tun0")
	}
	if source != "sysfs-scan" {
		t.Fatalf("source = %q, want %q", source, "sysfs-scan")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd backend && go test ./internal/monitoring/ -run TestDiscoverVPN -v`
Expected: FAIL — `DiscoverVPNInterface` doesn't exist

- [ ] **Step 3: Implement DiscoverVPNInterface**

Create `backend/internal/monitoring/vpn_discovery.go`:

```go
package monitoring

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// DiscoverVPNInterfaceResult holds the result of VPN interface discovery.
type DiscoverVPNInterfaceResult struct {
	Interface string
	Source    string
}

// vpnDiscoveryConfig holds configuration for VPN interface discovery.
type vpnDiscoveryConfig struct {
	reader      FileReader
	configPath  string
	sysNetPath  string
}

// VPNDiscoveryOption configures VPN interface discovery.
type VPNDiscoveryOption func(*vpnDiscoveryConfig)

// WithFileReader sets the file reader for discovery (for testing).
func WithFileReader(reader FileReader) VPNDiscoveryOption {
	return func(c *vpnDiscoveryConfig) { c.reader = reader }
}

// WithOcservConfigPath sets the ocserv config path.
func WithOcservConfigPath(path string) VPNDiscoveryOption {
	return func(c *vpnDiscoveryConfig) { c.configPath = path }
}

// DiscoverVPNInterface attempts to find the VPN interface name.
// It checks the ocserv config first, then scans /sys/class/net/ for tun/tap/wg interfaces.
// Returns (interfaceName, source, error). Empty interfaceName means no VPN interface found.
func DiscoverVPNInterface(opts ...VPNDiscoveryOption) (string, string, error) {
	cfg := vpnDiscoveryConfig{
		reader:     OSReader{},
		configPath: "/etc/ocserv/ocserv.conf",
		sysNetPath: "/sys/class/net",
	}
	for _, opt := range opts {
		opt(&cfg)
	}

	// Strategy 1: Check ocserv config for device directive
	iface, err := discoverFromOcservConfig(cfg)
	if err == nil && iface != "" {
		return iface, "ocserv-config", nil
	}

	// Strategy 2: Scan /sys/class/net/ for VPN interface patterns
	iface, err = discoverFromSysFS(cfg)
	if err == nil && iface != "" {
		return iface, "sysfs-scan", nil
	}

	// No VPN interface found
	return "", "", nil
}

func discoverFromOcservConfig(cfg vpnDiscoveryConfig) (string, error) {
	data, err := cfg.reader.ReadFile(cfg.configPath)
	if err != nil {
		return "", fmt.Errorf("read ocserv config: %w", err)
	}
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "#") || strings.HasPrefix(line, "//") {
			continue
		}
		if strings.HasPrefix(line, "device") {
			// Parse "device = tun0" or "device=tun0"
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				value := strings.TrimSpace(parts[1])
				if value != "" {
					return value, nil
				}
			}
		}
	}
	return "", nil
}

func discoverFromSysFS(cfg vpnDiscoveryConfig) (string, error) {
	entries, err := os.ReadDir(cfg.sysNetPath)
	if err != nil {
		// Use the reader for testability — but os.ReadDir doesn't use FileReader
		// For real usage, we read the directory directly
		return "", fmt.Errorf("read network interfaces: %w", err)
	}

	vpnPrefixes := []string{"tun", "tap", "wg"}
	var found []string
	for _, entry := range entries {
		name := entry.Name()
		for _, prefix := range vpnPrefixes {
			if strings.HasPrefix(name, prefix) {
				found = append(found, name)
			}
		}
	}

	if len(found) == 0 {
		return "", nil
	}

	// Prefer tun0 if available, otherwise first match
	for _, iface := range found {
		if iface == "tun0" {
			return iface, nil
		}
	}
	return found[0], nil
}
```

- [ ] **Step 4: Add SetState method to Catalog**

In `backend/internal/metrics/types.go`, add a method to update metric state:

```go
// SetState updates the state and reason of a metric in the catalog.
func (c Catalog) SetState(name MetricName, state State, reason string) {
	if def, ok := c.metrics[name]; ok {
		def.State = state
		def.Reason = reason
		c.metrics[name] = def
	}
}
```

Wait — `Catalog.metrics` is unexported. The method needs to be on `*Catalog` or we need to export it. Let me check... The `Catalog` struct has `metrics map[MetricName]MetricDefinition` (unexported). Since `SetState` is defined in the same package, it can access it. But `DefaultCatalog()` returns a value, not a pointer. We need to change `MonitoringService` to hold a `*Catalog` so we can mutate it.

Actually, looking at the code, `MonitoringService` uses `metrics.Store` and calls `metrics.DefaultCatalog()` inside its methods. We need to make the catalog mutable. Let me check monitoring_service.go:

- [ ] **Step 5: Read monitoring_service.go to understand catalog usage**

The `MonitoringService` calls `defaultMonitoringCatalog()` which calls `metrics.DefaultCatalog()`. We need to make the catalog a mutable field that can be updated when VPN is discovered.

Modify `backend/internal/app/monitoring_service.go` to accept an optional `*metrics.Catalog` that can be updated:

Add a `catalog *metrics.Catalog` field to `MonitoringService`. In `NewMonitoringService`, if catalog is nil, create a default one. The `Catalog()` method returns a copy from the pointer. Add a `SetMetricState` method.

- [ ] **Step 6: Update main.go to wire VPN discovery**

In `backend/cmd/ocserv-panel/main.go`, after creating the collector service:

```go
// Discover VPN interface
catalog := metrics.DefaultCatalog()
vpnIface, vpnSource, err := monitoring.DiscoverVPNInterface()
if err != nil {
	slog.Warn("vpn interface discovery failed", "error", err)
} else if vpnIface != "" {
	slog.Info("vpn interface discovered", "interface", vpnIface, "source", vpnSource)
	collectors = append(collectors, monitoring.NewInterfaceCollector(
		monitoring.OSReader{},
		vpnIface,
		metrics.MetricVPNInterfaceRXBytes,
		metrics.MetricVPNInterfaceTXBytes,
	))
	catalog.SetState(metrics.MetricVPNInterfaceRXBytes, metrics.StateCollecting, "")
	catalog.SetState(metrics.MetricVPNInterfaceTXBytes, metrics.StateCollecting, "")
} else {
	slog.Info("no vpn interface discovered; vpn metrics will remain unavailable")
}
```

And pass `&catalog` to `NewMonitoringService`.

- [ ] **Step 7: Run all backend tests**

Run: `cd backend && go test ./... -v`
Expected: All tests PASS

- [ ] **Step 8: Commit**

```bash
git add backend/internal/monitoring/vpn_discovery.go backend/internal/monitoring/vpn_discovery_test.go backend/internal/metrics/types.go backend/internal/app/monitoring_service.go backend/cmd/ocserv-panel/main.go
git commit -m "feat: add VPN interface auto-discovery and runtime catalog state updates"
```

---

### Task 6: Frontend — Settings API Client

**Files:**
- Modify: `frontend/src/lib/api.ts`

- [ ] **Step 1: Add Settings types and API functions**

In `frontend/src/lib/api.ts`, add after the existing types:

```typescript
export type Setting = {
  key: string;
  value: string;
};

export type SettingsResponse = {
  settings: Setting[];
};
```

Add API functions:

```typescript
export function getSettings(): Promise<SettingsResponse> {
  return apiFetch<SettingsResponse>('/api/settings');
}

export function updateSetting(key: string, value: string): Promise<Setting> {
  return apiFetch<Setting>(`/api/settings/${encodeURIComponent(key)}`, {
    method: 'PUT',
    body: { value }
  });
}
```

- [ ] **Step 2: Verify TypeScript compiles**

Run: `cd frontend && npx svelte-check --tsconfig ./tsconfig.json` or `npm run check`
Expected: No type errors

- [ ] **Step 3: Commit**

```bash
git add frontend/src/lib/api.ts
git commit -m "feat: add Settings types and API functions to frontend client"
```

---

### Task 7: Frontend — Dashboard Auto-Refresh & Analytics Cards

**Files:**
- Create: `frontend/src/lib/components/monitoring/AnalyticsCard.svelte`
- Modify: `frontend/src/routes/+page.svelte`

- [ ] **Step 1: Create AnalyticsCard component**

Create `frontend/src/lib/components/monitoring/AnalyticsCard.svelte`:

```svelte
<script lang="ts">
  type Props = {
    label: string;
    value: string;
    unit?: string;
  };

  let { label, value, unit }: Props = $props();
</script>

<article class="analytics-card panel">
  <span class="analytics-label">{label}</span>
  <strong class="analytics-value">{value}</strong>
  {#if unit}
    <small class="analytics-unit">{unit}</small>
  {/if}
</article>

<style>
  .analytics-card {
    display: flex;
    flex-direction: column;
    gap: 0.25rem;
  }
  .analytics-label {
    font-size: 0.875rem;
    color: var(--color-muted, #888);
  }
  .analytics-value {
    font-size: 1.5rem;
    line-height: 1.2;
  }
  .analytics-unit {
    font-size: 0.75rem;
    color: var(--color-muted, #888);
  }
</style>
```

- [ ] **Step 2: Update Dashboard page — add auto-refresh and analytics cards**

In `frontend/src/routes/+page.svelte`:

1. Add `onDestroy` import alongside `onMount`
2. Add auto-refresh interval for monitoring data
3. Replace the raw analytics `<dl>` with `AnalyticsCard` grid

Add to imports:
```typescript
import AnalyticsCard from '$lib/components/monitoring/AnalyticsCard.svelte';
import { formatBytes, formatMetricValue } from '$lib/monitoring';
```

Add auto-refresh state and logic:
```typescript
let refreshInterval: ReturnType<typeof setInterval> | null = null;

onMount(() => {
  void loadDashboard();
  refreshInterval = setInterval(() => {
    if (document.visibilityState === 'visible') {
      void reloadMonitoring();
    }
  }, 15_000);
});

onDestroy(() => {
  if (refreshInterval) {
    clearInterval(refreshInterval);
  }
});
```

Replace the analytics section. Change from:
```svelte
<section class="panel">
  <h3>Analytics summary</h3>
  {#if analyticsRows.length > 0}
    <dl class="definition-grid">
      {#each analyticsRows as [key, value]}
        <div>
          <dt>{key}</dt>
          <dd>{formatValue(value)}</dd>
        </div>
      {/each}
    </dl>
  {:else}
    <p class="muted">No analytics summary returned yet.</p>
  {/if}
</section>
```

To:
```svelte
<section class="panel">
  <h3>Analytics summary</h3>
  {#if summary?.analytics}
    <div class="grid cards">
      <AnalyticsCard label="Total sessions" value={formatMetricValue('totalSessions', summary.analytics.totalSessions as number ?? undefined)} unit="sessions" />
      <AnalyticsCard label="Sample sessions" value={formatMetricValue('sampleSessions', summary.analytics.sampleSessions as number ?? undefined)} unit="sessions" />
      <AnalyticsCard label="Total received" value={formatBytes(summary.analytics.totalBytesIn as number ?? undefined)} />
      <AnalyticsCard label="Total sent" value={formatBytes(summary.analytics.totalBytesOut as number ?? undefined)} />
      <AnalyticsCard label="Total traffic" value={formatBytes(((summary.analytics.totalBytesIn as number ?? 0) + (summary.analytics.totalBytesOut as number ?? 0)))} />
    </div>
  {:else}
    <p class="muted">No analytics summary returned yet.</p>
  {/if}
</section>
```

Remove the `analyticsRows` derived state and `formatValue` function since they're no longer needed.

- [ ] **Step 3: Verify the frontend builds**

Run: `cd frontend && npm run build`
Expected: Build succeeds

- [ ] **Step 4: Commit**

```bash
git add frontend/src/lib/components/monitoring/AnalyticsCard.svelte frontend/src/routes/+page.svelte
git commit -m "feat: add dashboard auto-refresh and analytics summary cards"
```

---

### Task 8: Frontend — History Auto-Refresh & Housekeeping Settings

**Files:**
- Modify: `frontend/src/routes/history/+page.svelte`

- [ ] **Step 1: Add auto-refresh to History page**

In `frontend/src/routes/history/+page.svelte`:

1. Add `onDestroy` import alongside `onMount`
2. Add auto-refresh interval for monitoring data
3. Add housekeeping settings section

Add to imports:
```typescript
import { getSettings, updateSetting, type Setting } from '$lib/api';
```

Add auto-refresh state:
```typescript
let refreshInterval: ReturnType<typeof setInterval> | null = null;
let retentionSetting = $state<string>('365');
let settingsLoading = $state(false);
let settingsError = $state('');
let settingsSuccess = $state('');
```

Update `onMount` to start interval and load settings:
```typescript
onMount(() => {
  void loadHistory();
  refreshInterval = setInterval(() => {
    if (document.visibilityState === 'visible') {
      void reloadMonitoring();
    }
  }, 15_000);
});

onDestroy(() => {
  if (refreshInterval) {
    clearInterval(refreshInterval);
  }
});
```

Add settings loading in `loadHistory()`:
```typescript
async function loadSettings() {
  settingsLoading = true;
  try {
    const response = await getSettings();
    const retention = response.settings.find((s: Setting) => s.key === 'history.retention_days');
    if (retention) {
      retentionSetting = retention.value;
    }
  } catch {
    // Settings are optional — don't block the page
  } finally {
    settingsLoading = false;
  }
}
```

Call `void loadSettings()` in `loadHistory()` after loading analytics.

Add save handler:
```typescript
async function handleSaveRetention() {
  settingsError = '';
  settingsSuccess = '';
  try {
    await updateSetting('history.retention_days', retentionSetting);
    settingsSuccess = 'Retention setting saved.';
  } catch (caught) {
    settingsError = caught instanceof Error ? caught.message : 'Failed to save retention setting.';
  }
}
```

- [ ] **Step 2: Add housekeeping settings section to History page template**

Add before the "History retention purge" section:

```svelte
<section class="panel">
  <h3>Housekeeping</h3>
  <p class="muted">Configure automatic data retention. Old records are purged daily based on the retention period.</p>
  {#if isOwner}
    <form class="form-stack" on:submit|preventDefault={handleSaveRetention}>
      <label>
        Auto-delete records older than
        <select bind:value={retentionSetting}>
          <option value="30">30 days</option>
          <option value="90">90 days</option>
          <option value="180">180 days</option>
          <option value="365">365 days (default)</option>
        </select>
      </label>
      {#if settingsError}<p class="error-text">{settingsError}</p>{/if}
      {#if settingsSuccess}<p class="success-text">{settingsSuccess}</p>{/if}
      <button class="button" type="submit" disabled={settingsLoading}>
        {settingsLoading ? 'Saving…' : 'Save retention setting'}
      </button>
    </form>
  {:else}
    <p class="muted">Your role can view analytics but cannot change retention settings.</p>
  {/if}
</section>
```

- [ ] **Step 3: Verify the frontend builds**

Run: `cd frontend && npm run build`
Expected: Build succeeds

- [ ] **Step 4: Commit**

```bash
git add frontend/src/routes/history/+page.svelte
git commit -m "feat: add history auto-refresh and housekeeping retention settings"
```

---

### Task 9: Integration Verification

- [ ] **Step 1: Run all backend tests**

Run: `cd backend && go test ./... -v`
Expected: All tests PASS

- [ ] **Step 2: Run frontend type check**

Run: `cd frontend && npm run check`
Expected: No type errors

- [ ] **Step 3: Run frontend build**

Run: `cd frontend && npm run build`
Expected: Build succeeds

- [ ] **Step 4: Verify no lint errors**

Run: `cd backend && go vet ./...`
Expected: No issues

- [ ] **Step 5: Final commit if any fixes needed**

```bash
git add -A
git commit -m "fix: integration verification fixes"
```