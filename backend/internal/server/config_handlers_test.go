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
