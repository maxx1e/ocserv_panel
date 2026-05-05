package server

import (
	"encoding/json"
	"net/http"
	"testing"

	"ocserv_panel/backend/internal/app"
)

func TestHandleSystemHealth_Unauthorized(t *testing.T) {
	t.Parallel()
	h := newTestHarness(t)
	rec := serveAPI(t, h.router, http.MethodGet, "/api/health/system", nil, "")
	assertStatus(t, rec, http.StatusUnauthorized)
}

func TestHandleSystemHealth_OK(t *testing.T) {
	t.Parallel()
	h := newTestHarness(t)
	h.createOwner(t, "owner", "owner-password")
	cookie := h.login(t, "owner", "owner-password")
	rec := serveAPI(t, h.router, http.MethodGet, "/api/health/system", nil, cookie)
	assertStatus(t, rec, http.StatusOK)

	var body map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if _, ok := body["vpnServer"]; !ok {
		t.Errorf("expected vpnServer key in response")
	}
	if _, ok := body["panel"]; !ok {
		t.Errorf("expected panel key in response")
	}
	if _, ok := body["logs"]; !ok {
		t.Errorf("expected logs key in response")
	}
}

func TestHandleSystemHealth_WrongMethod(t *testing.T) {
	t.Parallel()
	h := newTestHarness(t)
	h.createOwner(t, "owner", "owner-password")
	cookie := h.login(t, "owner", "owner-password")
	rec := serveAPI(t, h.router, http.MethodPost, "/api/health/system", nil, cookie)
	assertStatus(t, rec, http.StatusMethodNotAllowed)
}

func TestHandleLogTail_Unauthorized(t *testing.T) {
	t.Parallel()
	h := newTestHarness(t)
	rec := serveAPI(t, h.router, http.MethodGet, "/api/logs/tail?source=ocserv", nil, "")
	assertStatus(t, rec, http.StatusUnauthorized)
}

func TestHandleLogTail_OK(t *testing.T) {
	t.Parallel()
	h := newTestHarness(t)
	h.createOwner(t, "owner", "owner-password")
	cookie := h.login(t, "owner", "owner-password")
	rec := serveAPI(t, h.router, http.MethodGet, "/api/logs/tail?source=ocserv&lines=5", nil, cookie)
	assertStatus(t, rec, http.StatusOK)

	var body app.LogTailResult
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Source != "ocserv" {
		t.Errorf("expected source=ocserv, got %q", body.Source)
	}
}

func TestHandleLogTail_PanelSource(t *testing.T) {
	t.Parallel()
	h := newTestHarness(t)
	h.createOwner(t, "owner", "owner-password")
	cookie := h.login(t, "owner", "owner-password")
	rec := serveAPI(t, h.router, http.MethodGet, "/api/logs/tail?source=panel", nil, cookie)
	assertStatus(t, rec, http.StatusOK)

	var body app.LogTailResult
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Source != "panel" {
		t.Errorf("expected source=panel, got %q", body.Source)
	}
}

func TestHandleLogTail_InvalidSource(t *testing.T) {
	t.Parallel()
	h := newTestHarness(t)
	h.createOwner(t, "owner", "owner-password")
	cookie := h.login(t, "owner", "owner-password")
	rec := serveAPI(t, h.router, http.MethodGet, "/api/logs/tail?source=badvalue", nil, cookie)
	assertStatus(t, rec, http.StatusBadRequest)
}

func TestHandleLogTail_MissingSource(t *testing.T) {
	t.Parallel()
	h := newTestHarness(t)
	h.createOwner(t, "owner", "owner-password")
	cookie := h.login(t, "owner", "owner-password")
	rec := serveAPI(t, h.router, http.MethodGet, "/api/logs/tail", nil, cookie)
	assertStatus(t, rec, http.StatusBadRequest)
}

func TestHandleLogTail_WrongMethod(t *testing.T) {
	t.Parallel()
	h := newTestHarness(t)
	h.createOwner(t, "owner", "owner-password")
	cookie := h.login(t, "owner", "owner-password")
	rec := serveAPI(t, h.router, http.MethodPost, "/api/logs/tail?source=ocserv", nil, cookie)
	assertStatus(t, rec, http.StatusMethodNotAllowed)
}
