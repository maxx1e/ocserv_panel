package server

import (
	"bytes"
	"context"
	"crypto/tls"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ocserv_panel/backend/internal/app"
	"ocserv_panel/backend/internal/audit"
	"ocserv_panel/backend/internal/auth"
	"ocserv_panel/backend/internal/db"
	"ocserv_panel/backend/internal/metrics"
	"ocserv_panel/backend/internal/rbac"
	"ocserv_panel/backend/internal/store"
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

func TestAPIHealthReturnsEquivalentStatus(t *testing.T) {
	t.Parallel()

	rec := serveAPI(t, NewRouter(), http.MethodGet, "/api/health", nil, "")

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var body HealthResponse
	decodeJSON(t, rec, &body)
	if body.Status != "ok" {
		t.Fatalf("expected status ok, got %q", body.Status)
	}
}

func TestWebUISettingsReportsSecureDefault(t *testing.T) {
	h := newTestHarnessWithCookieSecure(t, true)

	rec := serveAPI(t, h.router, http.MethodGet, "/api/settings/webui", nil, "")

	assertStatus(t, rec, http.StatusOK)
	bodyText := rec.Body.String()
	body := decodeWebUISettingsBody(t, bodyText)
	assertMapField(t, body, "cookieSecure", true)
	assertMapField(t, body, "httpLoginAllowed", false)
	assertMapField(t, body, "requestSecure", false)
	assertMapField(t, body, "transport", "http")
	assertMapFieldContains(t, body, "warning", "not encrypted")
	assertBodyExcludesTerms(
		t,
		bodyText,
		"ocserv_panel_session",
		"password",
		"token",
		"hash",
		"PRIVATE KEY",
	)
}

func TestWebUISettingsReportsHTTPCompatibleCookieMode(t *testing.T) {
	h := newTestHarnessWithCookieSecure(t, false)

	rec := serveAPI(t, h.router, http.MethodGet, "/api/settings/webui", nil, "")

	assertStatus(t, rec, http.StatusOK)
	body := decodeWebUISettingsBody(t, rec.Body.String())
	assertMapField(t, body, "cookieSecure", false)
	assertMapField(t, body, "httpLoginAllowed", true)
	assertMapField(t, body, "requestSecure", false)
	assertMapField(t, body, "transport", "http")
	assertMapFieldContains(t, body, "warning", "HTTP login sends credentials")
}

func TestWebUISettingsReportsSecureRequest(t *testing.T) {
	h := newTestHarnessWithCookieSecure(t, true)
	req := httptest.NewRequest(http.MethodGet, "/api/settings/webui", nil)
	req.TLS = &tls.ConnectionState{}
	rec := httptest.NewRecorder()

	h.router.ServeHTTP(rec, req)

	assertStatus(t, rec, http.StatusOK)
	body := decodeWebUISettingsBody(t, rec.Body.String())
	assertMapField(t, body, "requestSecure", true)
	assertMapField(t, body, "transport", "https")
	assertMapField(t, body, "cookieSecure", true)
	assertMapField(t, body, "httpLoginAllowed", false)
	assertMapField(t, body, "warning", "")
}

func TestWebUISettingsRejectsNonGET(t *testing.T) {
	h := newTestHarness(t)

	rec := serveAPI(t, h.router, http.MethodPost, "/api/settings/webui", nil, "")

	assertStatus(t, rec, http.StatusMethodNotAllowed)
	if got := rec.Header().Get("Allow"); got != http.MethodGet {
		t.Fatalf("expected Allow header %q, got %q", http.MethodGet, got)
	}
}

func TestSetupOwnerLifecycle(t *testing.T) {
	h := newTestHarness(t)

	statusBefore := serveAPI(t, h.router, http.MethodGet, "/api/setup/status", nil, "")
	assertStatus(t, statusBefore, http.StatusOK)
	assertJSONField(t, statusBefore, "required", true)

	setupBody := map[string]string{"username": "owner", "password": "correct horse battery staple"}
	setup := serveAPI(t, h.router, http.MethodPost, "/api/setup/owner", setupBody, "")
	assertStatus(t, setup, http.StatusOK)
	setupCookie := requireSessionCookie(t, setup, true)
	assertBodyExcludesSecrets(t, setup.Body.String())
	assertJSONField(t, setup, "username", "owner")

	statusAfter := serveAPI(t, h.router, http.MethodGet, "/api/setup/status", nil, "")
	assertStatus(t, statusAfter, http.StatusOK)
	assertJSONField(t, statusAfter, "required", false)

	secondSetup := serveAPI(t, h.router, http.MethodPost, "/api/setup/owner", setupBody, "")
	assertStatus(t, secondSetup, http.StatusConflict)
	assertNoSetCookie(t, secondSetup)
	assertBodyExcludesSecrets(t, secondSetup.Body.String())

	badJSON := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/setup/owner", strings.NewReader(`{"username":"owner","password":`))
	req.Header.Set("Content-Type", "application/json")
	h.router.ServeHTTP(badJSON, req)
	assertStatus(t, badJSON, http.StatusBadRequest)
	assertNoSetCookie(t, badJSON)
	assertBodyExcludesSecrets(t, badJSON.Body.String())

	emptyFields := serveAPI(t, h.router, http.MethodPost, "/api/setup/owner", map[string]string{"username": " ", "password": ""}, "")
	assertStatus(t, emptyFields, http.StatusBadRequest)
	assertNoSetCookie(t, emptyFields)

	me := serveAPI(t, h.router, http.MethodGet, "/api/auth/me", nil, setupCookie.Value)
	assertStatus(t, me, http.StatusOK)
}

func TestLoginMeAndLogoutSessionCookieLifecycle(t *testing.T) {
	h := newTestHarness(t)
	h.createOwner(t, "owner", "correct horse battery staple")

	failedLogin := serveAPI(t, h.router, http.MethodPost, "/api/auth/login", map[string]string{"username": "owner", "password": "wrong"}, "")
	assertStatus(t, failedLogin, http.StatusUnauthorized)
	assertNoSetCookie(t, failedLogin)
	assertBodyExcludesSecrets(t, failedLogin.Body.String())
	assertJSONField(t, failedLogin, "error", "invalid username or password")

	login := serveAPI(t, h.router, http.MethodPost, "/api/auth/login", map[string]string{"username": "owner", "password": "correct horse battery staple"}, "")
	assertStatus(t, login, http.StatusOK)
	cookie := requireSessionCookie(t, login, true)
	assertBodyExcludesSecrets(t, login.Body.String())
	assertJSONField(t, login, "username", "owner")

	withoutCookie := serveAPI(t, h.router, http.MethodGet, "/api/auth/me", nil, "")
	assertStatus(t, withoutCookie, http.StatusUnauthorized)

	invalidCookie := serveAPI(t, h.router, http.MethodGet, "/api/auth/me", nil, "not-a-valid-token")
	assertStatus(t, invalidCookie, http.StatusUnauthorized)

	me := serveAPI(t, h.router, http.MethodGet, "/api/auth/me", nil, cookie.Value)
	assertStatus(t, me, http.StatusOK)
	assertJSONField(t, me, "username", "owner")

	logout := serveAPI(t, h.router, http.MethodPost, "/api/auth/logout", nil, cookie.Value)
	assertStatus(t, logout, http.StatusOK)
	cleared := requireSessionCookie(t, logout, true)
	if cleared.MaxAge >= 0 || cleared.Expires.After(time.Now()) {
		t.Fatalf("expected logout to expire cookie, got MaxAge=%d Expires=%s", cleared.MaxAge, cleared.Expires)
	}

	meAfterLogout := serveAPI(t, h.router, http.MethodGet, "/api/auth/me", nil, cookie.Value)
	assertStatus(t, meAfterLogout, http.StatusUnauthorized)
}

func TestAuthMeUnauthenticatedDoesNotRecordMonitoringAudit(t *testing.T) {
	h := newTestHarness(t)

	rec := serveAPI(t, h.router, http.MethodGet, "/api/auth/me", nil, "")

	assertStatus(t, rec, http.StatusUnauthorized)
	assertNoAuditActionPrefix(t, h.store, "monitoring.")
}

func TestLogoutClearsCookieWhenRevocationFails(t *testing.T) {
	h := newTestHarness(t)
	h.createOwner(t, "owner", "owner-password")
	cookie := h.login(t, "owner", "owner-password")

	if err := h.database.Close(); err != nil {
		t.Fatalf("close database to force logout failure: %v", err)
	}

	logout := serveAPI(t, h.router, http.MethodPost, "/api/auth/logout", nil, cookie)
	assertStatus(t, logout, http.StatusInternalServerError)
	cleared := requireSessionCookie(t, logout, true)
	if cleared.MaxAge >= 0 || cleared.Expires.After(time.Now()) {
		t.Fatalf("expected logout failure to expire cookie, got MaxAge=%d Expires=%s", cleared.MaxAge, cleared.Expires)
	}
}

func TestProtectedDashboardHistoryAndPurgeAuthorization(t *testing.T) {
	h := newTestHarness(t)
	h.createOwner(t, "owner", "owner-password")
	h.createViewer(t, "viewer", "viewer-password")
	ownerCookie := h.login(t, "owner", "owner-password")
	viewerCookie := h.login(t, "viewer", "viewer-password")

	for _, endpoint := range []string{"/api/dashboard/summary", "/api/history/analytics"} {
		t.Run(endpoint+" unauthenticated", func(t *testing.T) {
			assertStatus(t, serveAPI(t, h.router, http.MethodGet, endpoint, nil, ""), http.StatusUnauthorized)
		})
		t.Run(endpoint+" invalid cookie", func(t *testing.T) {
			assertStatus(t, serveAPI(t, h.router, http.MethodGet, endpoint, nil, "invalid"), http.StatusUnauthorized)
		})
		t.Run(endpoint+" viewer can read", func(t *testing.T) {
			assertStatus(t, serveAPI(t, h.router, http.MethodGet, endpoint, nil, viewerCookie), http.StatusOK)
		})
	}

	unauthPurge := serveAPI(t, h.router, http.MethodPost, "/api/history/purge", purgeRequest(30, "PURGE"), "")
	assertStatus(t, unauthPurge, http.StatusUnauthorized)

	viewerPurge := serveAPI(t, h.router, http.MethodPost, "/api/history/purge", purgeRequest(30, "PURGE"), viewerCookie)
	assertStatus(t, viewerPurge, http.StatusForbidden)

	badConfirmation := serveAPI(t, h.router, http.MethodPost, "/api/history/purge", purgeRequest(30, "NOPE"), ownerCookie)
	assertStatus(t, badConfirmation, http.StatusBadRequest)

	badRetention := serveAPI(t, h.router, http.MethodPost, "/api/history/purge", purgeRequest(31, "PURGE"), ownerCookie)
	assertStatus(t, badRetention, http.StatusBadRequest)

	ownerPurge := serveAPI(t, h.router, http.MethodPost, "/api/history/purge", purgeRequest(30, "PURGE"), ownerCookie)
	assertStatus(t, ownerPurge, http.StatusOK)
	var body map[string]any
	decodeJSON(t, ownerPurge, &body)
	if _, ok := body["deletedRows"].(float64); !ok {
		t.Fatalf("expected deletedRows number in %v", body)
	}
	if _, ok := body["cutoff"].(string); !ok {
		t.Fatalf("expected cutoff string in %v", body)
	}
}

func TestMonitoringEndpointsRequireAuthentication(t *testing.T) {
	h := newTestHarness(t)

	for _, path := range []string{
		"/api/monitoring/catalog",
		"/api/monitoring/overview",
		"/api/monitoring/series?metric=host_cpu_utilization",
	} {
		t.Run(path, func(t *testing.T) {
			rec := serveAPI(t, h.router, http.MethodGet, path, nil, "")

			assertStatus(t, rec, http.StatusUnauthorized)
			assertJSONField(t, rec, "error", "authentication required")
		})
	}
}

func TestMonitoringCatalogReturnsDefaultMetrics(t *testing.T) {
	h := newTestHarness(t)
	h.createOwner(t, "owner", "owner-password")
	cookie := h.login(t, "owner", "owner-password")

	rec := serveAPI(t, h.router, http.MethodGet, "/api/monitoring/catalog", nil, cookie)

	assertStatus(t, rec, http.StatusOK)
	var body []map[string]any
	decodeJSON(t, rec, &body)
	assertCatalogContainsMetric(t, body, string(metrics.MetricHostCPUUtilization))
	assertNoMapField(t, body[0], "Name")
	assertMapHasFields(t, body[0], "name", "description", "state")
}

func TestMonitoringOverviewReturnsDefaultWindowAndSeries(t *testing.T) {
	h := newTestHarness(t)
	h.createOwner(t, "owner", "owner-password")
	cookie := h.login(t, "owner", "owner-password")

	rec := serveAPI(t, h.router, http.MethodGet, "/api/monitoring/overview", nil, cookie)

	assertStatus(t, rec, http.StatusOK)
	var body map[string]any
	decodeJSON(t, rec, &body)
	assertMapField(t, body, "window", "1h")
	if _, ok := body["generatedAt"].(string); !ok {
		t.Fatalf("expected generatedAt string in %v", body)
	}
	if _, ok := body["series"].([]any); !ok {
		t.Fatalf("expected series array in %v", body)
	}
	assertNoMapField(t, body, "generated_at")
}

func TestMonitoringOverviewRejectsInvalidWindow(t *testing.T) {
	h := newTestHarness(t)
	h.createOwner(t, "owner", "owner-password")
	cookie := h.login(t, "owner", "owner-password")

	rec := serveAPI(t, h.router, http.MethodGet, "/api/monitoring/overview?window=2h", nil, cookie)

	assertStatus(t, rec, http.StatusBadRequest)
	assertJSONField(t, rec, "error", "invalid request")
}

func TestMonitoringSeriesReturnsRequestedMetric(t *testing.T) {
	h := newTestHarness(t)
	h.createOwner(t, "owner", "owner-password")
	cookie := h.login(t, "owner", "owner-password")

	rec := serveAPI(
		t,
		h.router,
		http.MethodGet,
		"/api/monitoring/series?metric=host_cpu_utilization&window=1h&step=60s",
		nil,
		cookie,
	)

	assertStatus(t, rec, http.StatusOK)
	var body map[string]any
	decodeJSON(t, rec, &body)
	assertMapField(t, body, "metric", string(metrics.MetricHostCPUUtilization))
	assertNoMapField(t, body, "Metric")
	if _, ok := body["points"].([]any); !ok {
		t.Fatalf("expected points array in %v", body)
	}
	assertNoMapField(t, body, "Points")
}

func TestMonitoringSeriesRejectsInvalidQuery(t *testing.T) {
	h := newTestHarness(t)
	h.createOwner(t, "owner", "owner-password")
	cookie := h.login(t, "owner", "owner-password")

	tests := []struct {
		name string
		path string
	}{
		{name: "missing metric", path: "/api/monitoring/series"},
		{name: "invalid metric", path: "/api/monitoring/series?metric=unknown&window=1h&step=60s"},
		{name: "invalid window", path: "/api/monitoring/series?metric=host_cpu_utilization&window=2h&step=60s"},
		{name: "invalid step", path: "/api/monitoring/series?metric=host_cpu_utilization&window=1h&step=1s"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := serveAPI(t, h.router, http.MethodGet, tt.path, nil, cookie)

			assertStatus(t, rec, http.StatusBadRequest)
			assertJSONField(t, rec, "error", "invalid request")
		})
	}
}

func TestMonitoringEndpointsDenyUsersWithoutDashboardRead(t *testing.T) {
	h := newTestHarness(t)
	h.createOwner(t, "owner", "owner-password")
	h.createUserWithoutRoles(t, "limited", "limited-password")
	cookie := h.login(t, "limited", "limited-password")

	rec := serveAPI(t, h.router, http.MethodGet, "/api/monitoring/catalog", nil, cookie)

	assertStatus(t, rec, http.StatusForbidden)
	assertJSONField(t, rec, "error", "permission denied")
}

func TestDashboardSummaryIncludesRequiredStatusAndLocalData(t *testing.T) {
	h := newTestHarness(t)
	h.createOwner(t, "owner", "owner-password")
	cookie := h.login(t, "owner", "owner-password")

	rec := serveAPI(t, h.router, http.MethodGet, "/api/dashboard/summary", nil, cookie)
	assertStatus(t, rec, http.StatusOK)

	var body map[string]any
	decodeJSON(t, rec, &body)
	if got := body["backendStatus"]; got != "ok" {
		t.Fatalf("backendStatus = %#v, want ok in %v", got, body)
	}
	if got := body["ocservStatus"]; got != "not_configured" {
		t.Fatalf("ocservStatus = %#v, want not_configured in %v", got, body)
	}
	if got := body["activeSessionCount"]; got != float64(0) {
		t.Fatalf("activeSessionCount = %#v, want 0 in %v", got, body)
	}
	if got := body["sampleData"]; got != true {
		t.Fatalf("sampleData = %#v, want true in %v", got, body)
	}
	if _, ok := body["recentAudit"].([]any); !ok {
		t.Fatalf("recentAudit missing or not an array in %v", body)
	}
	analytics, ok := body["analytics"].(map[string]any)
	if !ok {
		t.Fatalf("analytics missing or not an object in %v", body)
	}
	if analytics["sampleSessions"] == float64(0) {
		t.Fatalf("analytics sampleSessions = 0, want sample-backed summary in %v", analytics)
	}
	ocserv, ok := body["ocserv"].(map[string]any)
	if !ok {
		t.Fatalf("ocserv missing or not an object in %v", body)
	}
	if got := ocserv["status"]; got != "not_configured" {
		t.Fatalf("ocserv.status = %#v, want not_configured in %v", got, ocserv)
	}
}

func TestProtectedEndpointsAuditUnauthenticatedAttempts(t *testing.T) {
	tests := []struct {
		name   string
		method string
		path   string
		body   any
		action string
	}{
		{
			name:   "dashboard summary",
			method: http.MethodGet,
			path:   "/api/dashboard/summary",
			action: "dashboard.summary",
		},
		{
			name:   "history analytics",
			method: http.MethodGet,
			path:   "/api/history/analytics",
			action: "history.analytics",
		},
		{
			name:   "history purge",
			method: http.MethodPost,
			path:   "/api/history/purge",
			body:   purgeRequest(30, "PURGE"),
			action: "history.purge",
		},
		{
			name:   "monitoring catalog",
			method: http.MethodGet,
			path:   "/api/monitoring/catalog",
			action: "monitoring.catalog",
		},
		{
			name:   "monitoring overview",
			method: http.MethodGet,
			path:   "/api/monitoring/overview",
			action: "monitoring.overview",
		},
		{
			name:   "monitoring series",
			method: http.MethodGet,
			path:   "/api/monitoring/series?metric=host_cpu_utilization",
			action: "monitoring.series",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newTestHarness(t)

			rec := serveAPI(t, h.router, tt.method, tt.path, tt.body, "")
			assertStatus(t, rec, http.StatusUnauthorized)

			assertDeniedAuditAction(t, h.store, tt.action)
		})
	}
}

func TestJSONDecodeRejectsTrailingValues(t *testing.T) {
	h := newTestHarness(t)
	h.createOwner(t, "owner", "owner-password")

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/auth/login",
		strings.NewReader(`{"username":"owner","password":"owner-password"}{}`),
	)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.router.ServeHTTP(rec, req)

	assertStatus(t, rec, http.StatusBadRequest)
	assertNoSetCookie(t, rec)
	assertBodyExcludesSecrets(t, rec.Body.String())
}

func TestJSONDecodeRejectsUnknownFieldsWithoutLeakingSecrets(t *testing.T) {
	h := newTestHarness(t)
	h.createOwner(t, "owner", "owner-password")

	rec := serveRawAPI(
		t,
		h.router,
		http.MethodPost,
		"/api/auth/login",
		`{"username":"owner","password":"super-secret-password","unexpected":"super-secret-password"}`,
		"",
	)

	assertStatus(t, rec, http.StatusBadRequest)
	assertNoSetCookie(t, rec)
	assertJSONField(t, rec, "error", "invalid JSON request body")
	assertBodyExcludesTerms(t, rec.Body.String(), "super-secret-password", "unexpected")
}

func TestJSONDecodeRejectsOversizedBodiesWithoutLeakingSecrets(t *testing.T) {
	h := newTestHarness(t)
	secret := "super-secret-password"
	body := `{"username":"owner","password":"` + secret + strings.Repeat("x", maxJSONBodyBytes) + `"}`

	rec := serveRawAPI(t, h.router, http.MethodPost, "/api/setup/owner", body, "")

	assertStatus(t, rec, http.StatusBadRequest)
	assertNoSetCookie(t, rec)
	assertJSONField(t, rec, "error", "invalid JSON request body")
	assertBodyExcludesTerms(t, rec.Body.String(), secret, strings.Repeat("x", 64))
}

func TestRepresentativeWrongMethodsReturn405(t *testing.T) {
	h := newTestHarness(t)

	tests := []struct {
		name   string
		method string
		path   string
	}{
		{name: "setup status post", method: http.MethodPost, path: "/api/setup/status"},
		{name: "setup owner get", method: http.MethodGet, path: "/api/setup/owner"},
		{name: "login get", method: http.MethodGet, path: "/api/auth/login"},
		{name: "logout get", method: http.MethodGet, path: "/api/auth/logout"},
		{name: "me post", method: http.MethodPost, path: "/api/auth/me"},
		{name: "dashboard post", method: http.MethodPost, path: "/api/dashboard/summary"},
		{name: "analytics post", method: http.MethodPost, path: "/api/history/analytics"},
		{name: "purge get", method: http.MethodGet, path: "/api/history/purge"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertStatus(t, serveAPI(t, h.router, tt.method, tt.path, nil, ""), http.StatusMethodNotAllowed)
		})
	}
}

type testHarness struct {
	database *sql.DB
	store    *store.Store
	router   http.Handler
}

func newTestHarness(t *testing.T) *testHarness {
	t.Helper()
	return newTestHarnessWithCookieSecure(t, true)
}

func newTestHarnessWithCookieSecure(t *testing.T, cookieSecure bool) *testHarness {
	t.Helper()

	ctx := context.Background()
	database, err := db.OpenSQLite(ctx, filepath.Join(t.TempDir(), "panel.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() {
		if err := database.Close(); err != nil {
			t.Fatalf("close sqlite: %v", err)
		}
	})
	if err := db.ApplyMigrations(ctx, database, migrationsDir(t)); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}

	s := store.New(database)
	recorder := audit.NewSQLiteRecorder(s)
	authService := app.NewAuthService(s, recorder)
	dashboardService := app.NewDashboardService(s, recorder, rbac.NewDefaultAuthorizer())
	monitoringService := app.NewMonitoringService(metrics.NewMemoryStore(), rbac.NewDefaultAuthorizer(), nil, recorder)
	settingsService := app.NewSettingsService(s, recorder, rbac.NewDefaultAuthorizer())

	return &testHarness{
		database: database,
		store:    s,
		router: NewAppRouter(
			authService,
			dashboardService,
			monitoringService,
			settingsService,
			AppOptions{CookieSecure: cookieSecure},
		),
	}
}

func (h *testHarness) createOwner(t *testing.T, username string, password string) {
	t.Helper()
	principal, err := app.NewAuthService(h.store, audit.NewSQLiteRecorder(h.store)).CreateFirstOwner(context.Background(), username, password)
	if err != nil {
		t.Fatalf("create owner: %v", err)
	}
	if !principal.Authenticated() {
		t.Fatal("expected authenticated owner principal")
	}
}

func (h *testHarness) createViewer(t *testing.T, username string, password string) {
	t.Helper()
	passwordHash, err := auth.HashPassword(password)
	if err != nil {
		t.Fatalf("hash viewer password: %v", err)
	}
	result, err := h.database.ExecContext(
		context.Background(),
		`INSERT INTO admin_users (username, password_hash, created_at, updated_at) VALUES (?, ?, ?, ?)`,
		username,
		passwordHash,
		"2026-01-05T00:00:00Z",
		"2026-01-05T00:00:00Z",
	)
	if err != nil {
		t.Fatalf("insert viewer: %v", err)
	}
	adminID, err := result.LastInsertId()
	if err != nil {
		t.Fatalf("read viewer id: %v", err)
	}
	if _, err := h.database.ExecContext(
		context.Background(),
		`INSERT INTO admin_user_roles (admin_user_id, role_id, created_at) SELECT ?, id, ? FROM roles WHERE name = ?`,
		adminID,
		"2026-01-05T00:00:00Z",
		string(rbac.RoleViewer),
	); err != nil {
		t.Fatalf("assign viewer role: %v", err)
	}
}

func (h *testHarness) createUserWithoutRoles(t *testing.T, username string, password string) {
	t.Helper()
	passwordHash, err := auth.HashPassword(password)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	if _, err := h.database.ExecContext(
		context.Background(),
		`INSERT INTO admin_users (username, password_hash, created_at, updated_at) VALUES (?, ?, ?, ?)`,
		username,
		passwordHash,
		"2026-01-05T00:00:00Z",
		"2026-01-05T00:00:00Z",
	); err != nil {
		t.Fatalf("insert user without roles: %v", err)
	}
}

func (h *testHarness) login(t *testing.T, username string, password string) string {
	t.Helper()
	rec := serveAPI(t, h.router, http.MethodPost, "/api/auth/login", map[string]string{"username": username, "password": password}, "")
	assertStatus(t, rec, http.StatusOK)
	return requireSessionCookie(t, rec, true).Value
}

func migrationsDir(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}
	for _, candidate := range []string{
		filepath.Join(wd, "..", "..", "..", "migrations"),
		filepath.Join(wd, "..", "migrations"),
		"migrations",
	} {
		if _, err := os.Stat(filepath.Join(candidate, "0001_initial.sql")); err == nil {
			return candidate
		}
	}
	t.Fatal("could not find migrations directory")
	return ""
}

func serveAPI(t *testing.T, handler http.Handler, method string, path string, body any, sessionToken string) *httptest.ResponseRecorder {
	t.Helper()

	var reader *bytes.Reader
	if body == nil {
		reader = bytes.NewReader(nil)
	} else {
		encoded, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
		reader = bytes.NewReader(encoded)
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	if sessionToken != "" {
		req.AddCookie(&http.Cookie{Name: SessionCookieName, Value: sessionToken})
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func serveRawAPI(t *testing.T, handler http.Handler, method string, path string, body string, sessionToken string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if sessionToken != "" {
		req.AddCookie(&http.Cookie{Name: SessionCookieName, Value: sessionToken})
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func purgeRequest(retentionDays int, confirmation string) map[string]any {
	return map[string]any{"retentionDays": retentionDays, "confirmation": confirmation}
}

func assertStatus(t *testing.T, rec *httptest.ResponseRecorder, want int) {
	t.Helper()
	if rec.Code != want {
		t.Fatalf("expected status %d, got %d body=%s", want, rec.Code, rec.Body.String())
	}
}

func decodeJSON(t *testing.T, rec *httptest.ResponseRecorder, v any) {
	t.Helper()
	if err := json.NewDecoder(rec.Body).Decode(v); err != nil {
		t.Fatalf("decode JSON response: %v body=%s", err, rec.Body.String())
	}
}

func assertJSONField(t *testing.T, rec *httptest.ResponseRecorder, field string, want any) {
	t.Helper()
	var body map[string]any
	decodeJSON(t, rec, &body)
	got, ok := body[field]
	if !ok {
		t.Fatalf("expected field %q in %v", field, body)
	}
	if got != want {
		t.Fatalf("expected field %q to be %#v, got %#v", field, want, got)
	}
}

func decodeWebUISettingsBody(t *testing.T, bodyText string) map[string]any {
	t.Helper()
	body := map[string]any{}
	if err := json.Unmarshal([]byte(bodyText), &body); err != nil {
		t.Fatalf("decode web UI settings response: %v body=%s", err, bodyText)
	}
	return body
}

func assertMapField(t *testing.T, body map[string]any, field string, want any) {
	t.Helper()
	got, ok := body[field]
	if !ok {
		t.Fatalf("expected field %q in %v", field, body)
	}
	if got != want {
		t.Fatalf("expected field %q to be %#v, got %#v", field, want, got)
	}
}

func assertNoMapField(t *testing.T, body map[string]any, field string) {
	t.Helper()
	if _, ok := body[field]; ok {
		t.Fatalf("expected field %q to be absent in %v", field, body)
	}
}

func assertMapHasFields(t *testing.T, body map[string]any, fields ...string) {
	t.Helper()
	for _, field := range fields {
		if _, ok := body[field]; !ok {
			t.Fatalf("expected field %q in %v", field, body)
		}
	}
}

func assertMapFieldContains(t *testing.T, body map[string]any, field string, want string) {
	t.Helper()
	got, ok := body[field].(string)
	if !ok {
		t.Fatalf("expected field %q to be a string in %v", field, body)
	}
	if !strings.Contains(got, want) {
		t.Fatalf("expected field %q to contain %q, got %q", field, want, got)
	}
}

func assertCatalogContainsMetric(t *testing.T, catalog []map[string]any, want string) {
	t.Helper()
	for _, metric := range catalog {
		if metric["name"] == want {
			return
		}
	}
	t.Fatalf("expected catalog to contain metric %q in %v", want, catalog)
}

func requireSessionCookie(t *testing.T, rec *httptest.ResponseRecorder, expectSecure bool) *http.Cookie {
	t.Helper()
	for _, cookie := range rec.Result().Cookies() {
		if cookie.Name != SessionCookieName {
			continue
		}
		if !cookie.HttpOnly {
			t.Fatal("expected session cookie to be HttpOnly")
		}
		if cookie.SameSite != http.SameSiteLaxMode {
			t.Fatalf("expected SameSite=Lax, got %v", cookie.SameSite)
		}
		if cookie.Path != "/" {
			t.Fatalf("expected cookie path /, got %q", cookie.Path)
		}
		if cookie.Secure != expectSecure {
			t.Fatalf("expected Secure=%v, got %v", expectSecure, cookie.Secure)
		}
		return cookie
	}
	t.Fatalf("expected %s cookie in %v", SessionCookieName, rec.Result().Cookies())
	return nil
}

func assertNoSetCookie(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()
	for _, cookie := range rec.Result().Cookies() {
		if cookie.Name == SessionCookieName {
			t.Fatalf("expected no session cookie, got %v", cookie)
		}
	}
}

func assertBodyExcludesSecrets(t *testing.T, body string) {
	t.Helper()
	for _, forbidden := range []string{"token", "hash", "correct horse", "battery", "staple"} {
		if strings.Contains(strings.ToLower(body), forbidden) {
			t.Fatalf("response body leaks forbidden term %q: %s", forbidden, body)
		}
	}
}

func assertBodyExcludesTerms(t *testing.T, body string, terms ...string) {
	t.Helper()
	for _, term := range terms {
		if strings.Contains(body, term) {
			t.Fatalf("response body leaks forbidden term %q: %s", term, body)
		}
	}
}

func assertDeniedAuditAction(t *testing.T, s *store.Store, action string) {
	t.Helper()
	events, err := s.RecentAuditEvents(context.Background(), 20)
	if err != nil {
		t.Fatalf("load audit events: %v", err)
	}
	for _, event := range events {
		if event.Action == action && event.Outcome == string(audit.OutcomeDenied) {
			return
		}
	}
	t.Fatalf("expected denied audit action %q in %#v", action, events)
}

func assertNoAuditActionPrefix(t *testing.T, s *store.Store, prefix string) {
	t.Helper()
	events, err := s.RecentAuditEvents(context.Background(), 20)
	if err != nil {
		t.Fatalf("load audit events: %v", err)
	}
	for _, event := range events {
		if strings.HasPrefix(event.Action, prefix) {
			t.Fatalf("expected no audit action with prefix %q in %#v", prefix, events)
		}
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
