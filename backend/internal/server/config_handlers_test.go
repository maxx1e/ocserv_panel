package server

import (
	"net/http"
	"testing"
)

// --- Authentication tests (all endpoints require auth) ---

func TestConfigReadRequiresAuth(t *testing.T) {
	t.Parallel()
	h := newTestHarness(t)
	rec := serveAPI(t, h.router, http.MethodGet, "/api/config", nil, "")
	assertStatus(t, rec, http.StatusUnauthorized)
}

func TestConfigValidateRequiresAuth(t *testing.T) {
	t.Parallel()
	h := newTestHarness(t)
	rec := serveAPI(t, h.router, http.MethodPost, "/api/config/validate", map[string]string{"content": "x"}, "")
	assertStatus(t, rec, http.StatusUnauthorized)
}

func TestConfigDiffRequiresAuth(t *testing.T) {
	t.Parallel()
	h := newTestHarness(t)
	rec := serveAPI(t, h.router, http.MethodPost, "/api/config/diff", map[string]string{"content": "x"}, "")
	assertStatus(t, rec, http.StatusUnauthorized)
}

func TestConfigApplyRequiresAuth(t *testing.T) {
	t.Parallel()
	h := newTestHarness(t)
	rec := serveAPI(t, h.router, http.MethodPost, "/api/config/apply", map[string]string{"content": "x"}, "")
	assertStatus(t, rec, http.StatusUnauthorized)
}

func TestConfigBackupsRequiresAuth(t *testing.T) {
	t.Parallel()
	h := newTestHarness(t)
	rec := serveAPI(t, h.router, http.MethodGet, "/api/config/backups", nil, "")
	assertStatus(t, rec, http.StatusUnauthorized)
}

func TestConfigBackupGetRequiresAuth(t *testing.T) {
	t.Parallel()
	h := newTestHarness(t)
	rec := serveAPI(t, h.router, http.MethodGet, "/api/config/backups/1", nil, "")
	assertStatus(t, rec, http.StatusUnauthorized)
}

func TestConfigBackupDiffRequiresAuth(t *testing.T) {
	t.Parallel()
	h := newTestHarness(t)
	rec := serveAPI(t, h.router, http.MethodGet, "/api/config/backups/1/diff", nil, "")
	assertStatus(t, rec, http.StatusUnauthorized)
}

func TestConfigRollbackRequiresAuth(t *testing.T) {
	t.Parallel()
	h := newTestHarness(t)
	rec := serveAPI(t, h.router, http.MethodPost, "/api/config/rollback/1", nil, "")
	assertStatus(t, rec, http.StatusUnauthorized)
}

func TestConfigNetworkRequiresAuth(t *testing.T) {
	t.Parallel()
	h := newTestHarness(t)
	rec := serveAPI(t, h.router, http.MethodGet, "/api/config/network", nil, "")
	assertStatus(t, rec, http.StatusUnauthorized)
}

// --- Owner success paths ---

func TestConfigReadOwnerSuccess(t *testing.T) {
	t.Parallel()
	h := newTestHarness(t)
	h.createOwner(t, "admin", "password123")
	cookie := h.login(t, "admin", "password123")
	rec := serveAPI(t, h.router, http.MethodGet, "/api/config", nil, cookie)
	assertStatus(t, rec, http.StatusOK)
}

func TestConfigValidateOwnerSuccess(t *testing.T) {
	t.Parallel()
	h := newTestHarness(t)
	h.createOwner(t, "admin", "password123")
	cookie := h.login(t, "admin", "password123")
	rec := serveAPI(t, h.router, http.MethodPost, "/api/config/validate", map[string]string{"content": "tcp-port = 443\n"}, cookie)
	assertStatus(t, rec, http.StatusOK)
}

func TestConfigDiffOwnerSuccess(t *testing.T) {
	t.Parallel()
	h := newTestHarness(t)
	h.createOwner(t, "admin", "password123")
	cookie := h.login(t, "admin", "password123")
	rec := serveAPI(t, h.router, http.MethodPost, "/api/config/diff", map[string]string{"content": "tcp-port = 4443\n"}, cookie)
	assertStatus(t, rec, http.StatusOK)
}

func TestConfigApplyOwnerSuccess(t *testing.T) {
	t.Parallel()
	h := newTestHarness(t)
	h.createOwner(t, "admin", "password123")
	cookie := h.login(t, "admin", "password123")
	rec := serveAPI(t, h.router, http.MethodPost, "/api/config/apply", map[string]string{"content": "tcp-port = 4443\n"}, cookie)
	assertStatus(t, rec, http.StatusOK)
}

func TestConfigBackupsOwnerSuccess(t *testing.T) {
	t.Parallel()
	h := newTestHarness(t)
	h.createOwner(t, "admin", "password123")
	cookie := h.login(t, "admin", "password123")
	rec := serveAPI(t, h.router, http.MethodGet, "/api/config/backups", nil, cookie)
	assertStatus(t, rec, http.StatusOK)
}

func TestConfigNetworkOwnerSuccess(t *testing.T) {
	t.Parallel()
	h := newTestHarness(t)
	h.createOwner(t, "admin", "password123")
	cookie := h.login(t, "admin", "password123")
	rec := serveAPI(t, h.router, http.MethodGet, "/api/config/network", nil, cookie)
	assertStatus(t, rec, http.StatusOK)
}

// --- Viewer RBAC denial tests (edit and rollback denied) ---

func TestConfigApplyViewerForbidden(t *testing.T) {
	t.Parallel()
	h := newTestHarness(t)
	h.createOwner(t, "admin", "password123")
	h.createViewer(t, "viewer", "password123")
	cookie := h.login(t, "viewer", "password123")
	rec := serveAPI(t, h.router, http.MethodPost, "/api/config/apply", map[string]string{"content": "tcp-port = 4443\n"}, cookie)
	assertStatus(t, rec, http.StatusForbidden)
}

func TestConfigRollbackViewerForbidden(t *testing.T) {
	t.Parallel()
	h := newTestHarness(t)
	h.createOwner(t, "admin", "password123")
	h.createViewer(t, "viewer", "password123")
	cookie := h.login(t, "viewer", "password123")
	rec := serveAPI(t, h.router, http.MethodPost, "/api/config/rollback/1", nil, cookie)
	assertStatus(t, rec, http.StatusForbidden)
}

// --- Viewer allowed read operations ---

func TestConfigReadViewerAllowed(t *testing.T) {
	t.Parallel()
	h := newTestHarness(t)
	h.createOwner(t, "admin", "password123")
	h.createViewer(t, "viewer", "password123")
	cookie := h.login(t, "viewer", "password123")
	rec := serveAPI(t, h.router, http.MethodGet, "/api/config", nil, cookie)
	assertStatus(t, rec, http.StatusOK)
}

func TestConfigValidateViewerAllowed(t *testing.T) {
	t.Parallel()
	h := newTestHarness(t)
	h.createOwner(t, "admin", "password123")
	h.createViewer(t, "viewer", "password123")
	cookie := h.login(t, "viewer", "password123")
	rec := serveAPI(t, h.router, http.MethodPost, "/api/config/validate", map[string]string{"content": "tcp-port = 443\n"}, cookie)
	assertStatus(t, rec, http.StatusOK)
}

// --- Input validation ---

func TestConfigApplyEmptyContentBadRequest(t *testing.T) {
	t.Parallel()
	h := newTestHarness(t)
	h.createOwner(t, "admin", "password123")
	cookie := h.login(t, "admin", "password123")
	rec := serveAPI(t, h.router, http.MethodPost, "/api/config/apply", map[string]string{"content": ""}, cookie)
	assertStatus(t, rec, http.StatusBadRequest)
}

func TestConfigBackupGetInvalidNumber(t *testing.T) {
	t.Parallel()
	h := newTestHarness(t)
	h.createOwner(t, "admin", "password123")
	cookie := h.login(t, "admin", "password123")
	rec := serveAPI(t, h.router, http.MethodGet, "/api/config/backups/0", nil, cookie)
	assertStatus(t, rec, http.StatusBadRequest)
}

func TestConfigBackupGetNonexistent(t *testing.T) {
	t.Parallel()
	h := newTestHarness(t)
	h.createOwner(t, "admin", "password123")
	cookie := h.login(t, "admin", "password123")
	rec := serveAPI(t, h.router, http.MethodGet, "/api/config/backups/99", nil, cookie)
	assertStatus(t, rec, http.StatusNotFound)
}

// --- Validation failure returns 422 ---

func TestConfigApplyValidationFailureReturns422(t *testing.T) {
	t.Parallel()
	h := newTestHarness(t)
	h.createOwner(t, "admin", "password123")
	cookie := h.login(t, "admin", "password123")
	// Port 99999 is invalid (out of range 1-65535), should trigger validation error
	rec := serveAPI(t, h.router, http.MethodPost, "/api/config/apply", map[string]string{"content": "tcp-port = 99999\n"}, cookie)
	assertStatus(t, rec, http.StatusUnprocessableEntity)
}

// --- Validate handler response field tests ---

func TestConfigValidateHandler_ReturnsLineNumbers(t *testing.T) {
	t.Parallel()
	h := newTestHarness(t)
	h.createOwner(t, "admin", "password123")
	cookie := h.login(t, "admin", "password123")

	rec := serveAPI(t, h.router, http.MethodPost, "/api/config/validate",
		map[string]string{"content": "# comment\ntcp-port = 99999\n"}, cookie)
	assertStatus(t, rec, http.StatusOK)

	var body map[string]any
	decodeJSON(t, rec, &body)

	issues, ok := body["issues"].([]any)
	if !ok || len(issues) < 1 {
		t.Fatalf("expected issues array with at least 1 element, got %v", body["issues"])
	}

	first, ok := issues[0].(map[string]any)
	if !ok {
		t.Fatalf("expected first issue to be an object, got %T", issues[0])
	}

	if first["line"] != float64(2) {
		t.Fatalf("expected line = 2, got %v", first["line"])
	}
	if first["severity"] != "error" {
		t.Fatalf("expected severity = \"error\", got %v", first["severity"])
	}
	if first["key"] != "tcp-port" {
		t.Fatalf("expected key = \"tcp-port\", got %v", first["key"])
	}
}

func TestConfigValidateHandler_ReturnsCIDRFixedValue(t *testing.T) {
	t.Parallel()
	h := newTestHarness(t)
	h.createOwner(t, "admin", "password123")
	cookie := h.login(t, "admin", "password123")

	rec := serveAPI(t, h.router, http.MethodPost, "/api/config/validate",
		map[string]string{"content": "route = 10.10.10.0/255.255.255.0\n"}, cookie)
	assertStatus(t, rec, http.StatusOK)

	var body map[string]any
	decodeJSON(t, rec, &body)

	issues, ok := body["issues"].([]any)
	if !ok || len(issues) < 1 {
		t.Fatalf("expected issues array with at least 1 element, got %v", body["issues"])
	}

	first, ok := issues[0].(map[string]any)
	if !ok {
		t.Fatalf("expected first issue to be an object, got %T", issues[0])
	}

	if first["fixedValue"] != "route = 10.10.10.0/24" {
		t.Fatalf("expected fixedValue = \"route = 10.10.10.0/24\", got %v", first["fixedValue"])
	}
	if first["severity"] != "warning" {
		t.Fatalf("expected severity = \"warning\", got %v", first["severity"])
	}
	if first["line"] != float64(1) {
		t.Fatalf("expected line = 1, got %v", first["line"])
	}
}

func TestConfigValidateHandler_ReturnsRestartRequired(t *testing.T) {
	t.Parallel()
	h := newTestHarness(t)
	h.createOwner(t, "admin", "password123")
	cookie := h.login(t, "admin", "password123")

	rec := serveAPI(t, h.router, http.MethodPost, "/api/config/validate",
		map[string]string{"content": "tcp-port = 8443\n"}, cookie)
	assertStatus(t, rec, http.StatusOK)

	var body map[string]any
	decodeJSON(t, rec, &body)

	if body["requiresRestart"] != true {
		t.Fatalf("expected requiresRestart = true, got %v", body["requiresRestart"])
	}

	reasons, ok := body["restartReasons"].([]any)
	if !ok || len(reasons) == 0 {
		t.Fatalf("expected restartReasons to be a non-empty array, got %v", body["restartReasons"])
	}
}
