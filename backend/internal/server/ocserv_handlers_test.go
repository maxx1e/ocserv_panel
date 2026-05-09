package server

import (
	"net/http"
	"testing"
	"time"

	"ocserv_panel/backend/internal/ocserv"
)

// --- handleOcservSessionDetail tests ---

func TestOcservSessionDetailRequiresAuth(t *testing.T) {
	t.Parallel()

	h := newTestHarness(t)

	rec := serveAPI(t, h.router, http.MethodGet, "/api/ocserv/sessions/sess-1", nil, "")

	assertStatus(t, rec, http.StatusUnauthorized)
	assertJSONField(t, rec, "error", "authentication required")
}

func TestOcservSessionDetailViewerCanRead(t *testing.T) {
	h := newTestHarness(t)
	h.createOwner(t, "owner", "owner-pass")
	h.createViewer(t, "viewer", "viewer-pass")
	viewerCookie := h.login(t, "viewer", "viewer-pass")

	// FakeManager returns ErrNotConfigured for unknown IDs, which maps to 503.
	// Viewer has read permission so 503 (not 403) means auth passed.
	rec := serveAPI(t, h.router, http.MethodGet, "/api/ocserv/sessions/no-such-id", nil, viewerCookie)

	assertStatus(t, rec, http.StatusServiceUnavailable)
}

func TestOcservSessionDetailReturnsDetail(t *testing.T) {
	h := newTestHarnessWithSessionDetail(t, "sess-abc", &ocserv.SessionDetail{
		ID:             "sess-abc",
		Username:       "alice",
		RemoteAddress:  "1.2.3.4",
		VPNAddress:     "10.0.0.2",
		ConnectedAt:    time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC),
		Duration:       90 * time.Second,
		RxBytes:        1024,
		TxBytes:        2048,
		UserAgent:      "OpenConnect/8.20",
		Device:         "laptop",
		IPv4:           "10.0.0.2",
		IPv6:           "",
		Routes:         []string{"0.0.0.0/0"},
		DNS:            []string{"8.8.8.8"},
		ReconnectCount: 3,
	})
	h.createOwner(t, "owner", "owner-pass")
	cookie := h.login(t, "owner", "owner-pass")

	rec := serveAPI(t, h.router, http.MethodGet, "/api/ocserv/sessions/sess-abc", nil, cookie)

	assertStatus(t, rec, http.StatusOK)
	var body map[string]any
	decodeJSON(t, rec, &body)
	assertMapField(t, body, "id", "sess-abc")
	assertMapField(t, body, "username", "alice")
	assertMapField(t, body, "remoteAddress", "1.2.3.4")
	assertMapField(t, body, "vpnAddress", "10.0.0.2")
	assertMapField(t, body, "durationSeconds", float64(90))
	assertMapField(t, body, "rxBytes", float64(1024))
	assertMapField(t, body, "txBytes", float64(2048))
	assertMapField(t, body, "userAgent", "OpenConnect/8.20")
	assertMapField(t, body, "device", "laptop")
	assertMapField(t, body, "ipv4", "10.0.0.2")
	assertMapField(t, body, "reconnectCount", float64(3))
	if connectedAt, ok := body["connectedAt"].(string); !ok || connectedAt == "" {
		t.Fatalf("expected non-empty connectedAt string, got %v", body["connectedAt"])
	}
	routes, ok := body["routes"].([]any)
	if !ok {
		t.Fatalf("expected routes array, got %T in %v", body["routes"], body)
	}
	if len(routes) != 1 || routes[0] != "0.0.0.0/0" {
		t.Fatalf("unexpected routes %v", routes)
	}
	dns, ok := body["dns"].([]any)
	if !ok {
		t.Fatalf("expected dns array, got %T in %v", body["dns"], body)
	}
	if len(dns) != 1 || dns[0] != "8.8.8.8" {
		t.Fatalf("unexpected dns %v", dns)
	}
}

func TestOcservSessionDetailReturnsEmptySlicesForNilCollections(t *testing.T) {
	h := newTestHarnessWithSessionDetail(t, "sess-nil", &ocserv.SessionDetail{
		ID:       "sess-nil",
		Username: "bob",
		Routes:   nil,
		DNS:      nil,
	})
	h.createOwner(t, "owner", "owner-pass")
	cookie := h.login(t, "owner", "owner-pass")

	rec := serveAPI(t, h.router, http.MethodGet, "/api/ocserv/sessions/sess-nil", nil, cookie)

	assertStatus(t, rec, http.StatusOK)
	var body map[string]any
	decodeJSON(t, rec, &body)
	routes, ok := body["routes"].([]any)
	if !ok {
		t.Fatalf("expected routes to be [] not null, got %T in %v", body["routes"], body)
	}
	if len(routes) != 0 {
		t.Fatalf("expected empty routes, got %v", routes)
	}
	dns, ok := body["dns"].([]any)
	if !ok {
		t.Fatalf("expected dns to be [] not null, got %T in %v", body["dns"], body)
	}
	if len(dns) != 0 {
		t.Fatalf("expected empty dns, got %v", dns)
	}
}

func TestOcservSessionDetailReturns404WhenSessionNotFound(t *testing.T) {
	h := newTestHarnessWithSessionNotFound(t, "missing-id")
	h.createOwner(t, "owner", "owner-pass")
	cookie := h.login(t, "owner", "owner-pass")

	rec := serveAPI(t, h.router, http.MethodGet, "/api/ocserv/sessions/missing-id", nil, cookie)

	assertStatus(t, rec, http.StatusNotFound)
	assertJSONField(t, rec, "error", "session not found")
}

func TestOcservSessionDetailRejectsNonGET(t *testing.T) {
	t.Parallel()

	h := newTestHarness(t)
	h.createOwner(t, "owner", "owner-pass")
	cookie := h.login(t, "owner", "owner-pass")

	rec := serveAPI(t, h.router, http.MethodPost, "/api/ocserv/sessions/sess-1", nil, cookie)

	assertStatus(t, rec, http.StatusMethodNotAllowed)
}

// --- handleOcservSessionTerminate tests ---

func TestOcservSessionTerminateRequiresAuth(t *testing.T) {
	t.Parallel()

	h := newTestHarness(t)

	rec := serveAPI(t, h.router, http.MethodPost, "/api/ocserv/sessions/terminate", map[string]string{"sessionId": "sess-1"}, "")

	assertStatus(t, rec, http.StatusUnauthorized)
	assertJSONField(t, rec, "error", "authentication required")
}

func TestOcservSessionTerminateViewerCannotTerminate(t *testing.T) {
	h := newTestHarness(t)
	h.createOwner(t, "owner", "owner-pass")
	h.createViewer(t, "viewer", "viewer-pass")
	viewerCookie := h.login(t, "viewer", "viewer-pass")

	rec := serveAPI(t, h.router, http.MethodPost, "/api/ocserv/sessions/terminate", map[string]string{"sessionId": "sess-1"}, viewerCookie)

	assertStatus(t, rec, http.StatusForbidden)
	assertJSONField(t, rec, "error", "permission denied")
}

func TestOcservSessionTerminateRejectsMissingSessionID(t *testing.T) {
	h := newTestHarness(t)
	h.createOwner(t, "owner", "owner-pass")
	cookie := h.login(t, "owner", "owner-pass")

	rec := serveAPI(t, h.router, http.MethodPost, "/api/ocserv/sessions/terminate", map[string]string{"sessionId": ""}, cookie)

	assertStatus(t, rec, http.StatusBadRequest)
	assertJSONField(t, rec, "error", "sessionId is required")
}

func TestOcservSessionTerminateReturns204OnSuccess(t *testing.T) {
	h := newTestHarnessWithTerminateOK(t)
	h.createOwner(t, "owner", "owner-pass")
	cookie := h.login(t, "owner", "owner-pass")

	rec := serveAPI(t, h.router, http.MethodPost, "/api/ocserv/sessions/terminate", map[string]string{"sessionId": "sess-xyz"}, cookie)

	assertStatus(t, rec, http.StatusNoContent)
}

func TestOcservSessionTerminateReturns503WhenNotConfigured(t *testing.T) {
	// Default FakeManager returns ErrNotConfigured for TerminateSession.
	h := newTestHarness(t)
	h.createOwner(t, "owner", "owner-pass")
	cookie := h.login(t, "owner", "owner-pass")

	rec := serveAPI(t, h.router, http.MethodPost, "/api/ocserv/sessions/terminate", map[string]string{"sessionId": "sess-1"}, cookie)

	assertStatus(t, rec, http.StatusServiceUnavailable)
}

func TestOcservSessionTerminateRejectsNonPOST(t *testing.T) {
	t.Parallel()

	h := newTestHarness(t)
	h.createOwner(t, "owner", "owner-pass")
	cookie := h.login(t, "owner", "owner-pass")

	// DELETE /api/ocserv/sessions/terminate is handled by sessions/{id} for GET,
	// but DELETE is not registered, so the mux itself returns 405.
	rec := serveAPI(t, h.router, http.MethodDelete, "/api/ocserv/sessions/terminate", nil, cookie)

	assertStatus(t, rec, http.StatusMethodNotAllowed)
}

// --- List sessions includes enriched fields ---

func TestOcservSessionsListIncludesEnrichedFields(t *testing.T) {
	h := newTestHarnessWithSessions(t, []ocserv.Session{
		{
			ID:             "s1",
			Username:       "carol",
			RemoteAddress:  "5.6.7.8",
			VPNAddress:     "10.8.0.5",
			RxBytes:        512,
			TxBytes:        1024,
			UserAgent:      "AnyConnect/4.0",
			Device:         "phone",
			ReconnectCount: 1,
		},
	})
	h.createOwner(t, "owner", "owner-pass")
	cookie := h.login(t, "owner", "owner-pass")

	rec := serveAPI(t, h.router, http.MethodGet, "/api/ocserv/sessions", nil, cookie)

	assertStatus(t, rec, http.StatusOK)
	var body []map[string]any
	decodeJSON(t, rec, &body)
	if len(body) != 1 {
		t.Fatalf("expected 1 session, got %d", len(body))
	}
	s := body[0]
	assertMapField(t, s, "id", "s1")
	assertMapField(t, s, "username", "carol")
	assertMapField(t, s, "vpnAddress", "10.8.0.5")
	assertMapField(t, s, "rxBytes", float64(512))
	assertMapField(t, s, "txBytes", float64(1024))
	assertMapField(t, s, "userAgent", "AnyConnect/4.0")
	assertMapField(t, s, "device", "phone")
	assertMapField(t, s, "reconnectCount", float64(1))
}

// --- Test harness helpers for ocserv handlers ---

// newTestHarnessWithSessionDetail wires a FakeManager that returns the given detail for the given ID.
func newTestHarnessWithSessionDetail(t *testing.T, id string, detail *ocserv.SessionDetail) *testHarness {
	t.Helper()
	h := newTestHarness(t)
	fakeManager := h.fakeManager(t)
	fakeManager.SessionDetails[id] = detail
	return h
}

// newTestHarnessWithSessionNotFound wires a FakeManager that returns ErrSessionNotFound for the given ID.
func newTestHarnessWithSessionNotFound(t *testing.T, id string) *testHarness {
	t.Helper()
	h := newTestHarness(t)
	fakeManager := h.fakeManager(t)
	fakeManager.SessionDetailNotFound[id] = true
	return h
}

// newTestHarnessWithTerminateOK wires a FakeManager that returns nil from TerminateSession.
func newTestHarnessWithTerminateOK(t *testing.T) *testHarness {
	t.Helper()
	h := newTestHarness(t)
	fakeManager := h.fakeManager(t)
	fakeManager.TerminateErr = nil
	return h
}

// newTestHarnessWithSessions wires a FakeManager with the given sessions.
func newTestHarnessWithSessions(t *testing.T, sessions []ocserv.Session) *testHarness {
	t.Helper()
	h := newTestHarness(t)
	fakeManager := h.fakeManager(t)
	fakeManager.Sessions = sessions
	return h
}

// --- handleOcservServerStatus tests ---

func TestOcservServerStatusRequiresAuth(t *testing.T) {
	t.Parallel()
	h := newTestHarness(t)
	rec := serveAPI(t, h.router, http.MethodGet, "/api/ocserv/server/status", nil, "")
	assertStatus(t, rec, http.StatusUnauthorized)
}

func TestOcservServerStatusSuccess(t *testing.T) {
	t.Parallel()
	h := newTestHarness(t)
	h.ocservFake.ServerStatusResult = &ocserv.ServerStatus{
		Status:         "online",
		ActiveSessions: 5,
		ServerPID:      1234,
	}
	h.createOwner(t, "admin", "password123")
	token := h.login(t, "admin", "password123")
	rec := serveAPI(t, h.router, http.MethodGet, "/api/ocserv/server/status", nil, token)
	assertStatus(t, rec, http.StatusOK)
	assertJSONField(t, rec, "status", "online")
}

// --- handleOcservConnectedUsers tests ---

func TestOcservConnectedUsersRequiresAuth(t *testing.T) {
	t.Parallel()
	h := newTestHarness(t)
	rec := serveAPI(t, h.router, http.MethodGet, "/api/ocserv/users/connected", nil, "")
	assertStatus(t, rec, http.StatusUnauthorized)
}

func TestOcservConnectedUsersSuccess(t *testing.T) {
	t.Parallel()
	h := newTestHarness(t)
	h.ocservFake.ConnectedUsersResult = []ocserv.ConnectedUser{
		{ID: "123", Username: "user1", Status: "connected"},
	}
	h.createOwner(t, "admin", "password123")
	token := h.login(t, "admin", "password123")
	rec := serveAPI(t, h.router, http.MethodGet, "/api/ocserv/users/connected", nil, token)
	assertStatus(t, rec, http.StatusOK)
}

// --- handleOcservUserConnections tests ---

func TestOcservUserConnectionsRequiresAuth(t *testing.T) {
	t.Parallel()
	h := newTestHarness(t)
	rec := serveAPI(t, h.router, http.MethodGet, "/api/ocserv/users/testuser/connections", nil, "")
	assertStatus(t, rec, http.StatusUnauthorized)
}

// --- handleOcservEvents tests ---

func TestOcservEventsRequiresAuth(t *testing.T) {
	t.Parallel()
	h := newTestHarness(t)
	rec := serveAPI(t, h.router, http.MethodGet, "/api/ocserv/events", nil, "")
	assertStatus(t, rec, http.StatusUnauthorized)
}

// --- handleOcservRoutes tests ---

func TestOcservRoutesRequiresAuth(t *testing.T) {
	t.Parallel()
	h := newTestHarness(t)
	rec := serveAPI(t, h.router, http.MethodGet, "/api/ocserv/routes", nil, "")
	assertStatus(t, rec, http.StatusUnauthorized)
}

// --- handleOcservBans tests ---

func TestOcservBansRequiresAuth(t *testing.T) {
	t.Parallel()
	h := newTestHarness(t)
	rec := serveAPI(t, h.router, http.MethodGet, "/api/ocserv/bans", nil, "")
	assertStatus(t, rec, http.StatusUnauthorized)
}

// --- handleOcservBanPoints tests ---

func TestOcservBanPointsRequiresAuth(t *testing.T) {
	t.Parallel()
	h := newTestHarness(t)
	rec := serveAPI(t, h.router, http.MethodGet, "/api/ocserv/bans/points", nil, "")
	assertStatus(t, rec, http.StatusUnauthorized)
}

// --- handleOcservUserDisconnect tests ---

func TestOcservUserDisconnectRequiresAuth(t *testing.T) {
	t.Parallel()
	h := newTestHarness(t)
	rec := serveAPI(t, h.router, http.MethodPost, "/api/ocserv/users/testuser/disconnect", nil, "")
	assertStatus(t, rec, http.StatusUnauthorized)
}

func TestOcservUserDisconnectSuccess(t *testing.T) {
	t.Parallel()
	h := newTestHarness(t)
	h.ocservFake.DisconnectUserErr = nil
	h.createOwner(t, "admin", "password123")
	token := h.login(t, "admin", "password123")
	rec := serveAPI(t, h.router, http.MethodPost, "/api/ocserv/users/testuser/disconnect", nil, token)
	assertStatus(t, rec, http.StatusNoContent)
}

// --- handleOcservUnbanIP tests ---

func TestOcservUnbanIPRequiresAuth(t *testing.T) {
	t.Parallel()
	h := newTestHarness(t)
	rec := serveAPI(t, h.router, http.MethodPost, "/api/ocserv/bans/10.0.0.1/unban", nil, "")
	assertStatus(t, rec, http.StatusUnauthorized)
}

func TestOcservUnbanIPSuccess(t *testing.T) {
	t.Parallel()
	h := newTestHarness(t)
	h.ocservFake.UnbanIPErr = nil
	h.createOwner(t, "admin", "password123")
	token := h.login(t, "admin", "password123")
	rec := serveAPI(t, h.router, http.MethodPost, "/api/ocserv/bans/10.0.0.1/unban", nil, token)
	assertStatus(t, rec, http.StatusNoContent)
}

// --- handleOcservConnectionDetail tests ---

func TestOcservConnectionDetailRequiresAuth(t *testing.T) {
	t.Parallel()
	h := newTestHarness(t)
	rec := serveAPI(t, h.router, http.MethodGet, "/api/ocserv/connections/3549163", nil, "")
	assertStatus(t, rec, http.StatusUnauthorized)
}

func TestOcservConnectionDetailInvalidID(t *testing.T) {
	t.Parallel()
	h := newTestHarness(t)
	h.createOwner(t, "owner", "owner-pass")
	cookie := h.login(t, "owner", "owner-pass")

	rec := serveAPI(t, h.router, http.MethodGet, "/api/ocserv/connections/abc", nil, cookie)
	assertStatus(t, rec, http.StatusBadRequest)
}

func TestOcservConnectionDetailNegativeID(t *testing.T) {
	t.Parallel()
	h := newTestHarness(t)
	h.createOwner(t, "owner", "owner-pass")
	cookie := h.login(t, "owner", "owner-pass")

	rec := serveAPI(t, h.router, http.MethodGet, "/api/ocserv/connections/-1", nil, cookie)
	assertStatus(t, rec, http.StatusBadRequest)
}

func TestOcservConnectionDetailSuccess(t *testing.T) {
	t.Parallel()
	h := newTestHarness(t)
	h.ocservFake.ConnectionByIDResult[3549163] = &ocserv.UserConnection{
		ID:       "3549163",
		Username: "maxxie",
		IPv4:     "10.10.10.208",
		Device:   "vpns0",
		Hostname: "LemonGrass",
	}
	h.createOwner(t, "owner", "owner-pass")
	cookie := h.login(t, "owner", "owner-pass")

	rec := serveAPI(t, h.router, http.MethodGet, "/api/ocserv/connections/3549163", nil, cookie)
	assertStatus(t, rec, http.StatusOK)

	var body map[string]any
	decodeJSON(t, rec, &body)
	assertMapField(t, body, "username", "maxxie")
	assertMapField(t, body, "ipv4", "10.10.10.208")
	assertMapField(t, body, "hostname", "LemonGrass")
}

func TestOcservConnectionDetailNotFound(t *testing.T) {
	t.Parallel()
	h := newTestHarness(t)
	h.createOwner(t, "owner", "owner-pass")
	cookie := h.login(t, "owner", "owner-pass")

	// FakeManager returns ErrSessionNotFound for unknown IDs
	rec := serveAPI(t, h.router, http.MethodGet, "/api/ocserv/connections/9999999", nil, cookie)
	assertStatus(t, rec, http.StatusNotFound)
}
