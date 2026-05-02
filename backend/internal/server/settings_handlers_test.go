package server

import (
	"net/http"
	"testing"
)

func TestSettingsSectionsGetRequiresAuth(t *testing.T) {
	h := newTestHarness(t)

	rec := serveAPI(t, h.router, http.MethodGet, "/api/settings", nil, "")

	assertStatus(t, rec, http.StatusUnauthorized)
	assertJSONField(t, rec, "error", "authentication required")
}

func TestSettingsSectionsGetReturnsSections(t *testing.T) {
	h := newTestHarness(t)
	h.createOwner(t, "testowner", "testpass123")
	cookie := h.login(t, "testowner", "testpass123")

	rec := serveAPI(t, h.router, http.MethodGet, "/api/settings", nil, cookie)

	assertStatus(t, rec, http.StatusOK)
	var body map[string]any
	decodeJSON(t, rec, &body)
	if _, ok := body["restart_required"]; !ok {
		t.Fatalf("response missing restart_required in %v", body)
	}
	sections, ok := body["sections"].([]any)
	if !ok {
		t.Fatalf("sections is not an array in %v", body)
	}
	if len(sections) != 12 {
		t.Fatalf("expected 12 sections, got %d", len(sections))
	}
}

func TestSettingsSectionsGetViewerCanRead(t *testing.T) {
	h := newTestHarness(t)
	h.createOwner(t, "testowner", "testpass123")
	h.createViewer(t, "testviewer", "viewerpass123")
	cookie := h.login(t, "testviewer", "viewerpass123")

	rec := serveAPI(t, h.router, http.MethodGet, "/api/settings", nil, cookie)

	assertStatus(t, rec, http.StatusOK)
	var body map[string]any
	decodeJSON(t, rec, &body)
	if _, ok := body["restart_required"]; !ok {
		t.Fatalf("response missing restart_required in %v", body)
	}
	sections, ok := body["sections"].([]any)
	if !ok {
		t.Fatalf("sections is not an array in %v", body)
	}
	if len(sections) != 12 {
		t.Fatalf("expected 12 sections, got %d", len(sections))
	}
}

func TestSettingsUpdateRequiresAuth(t *testing.T) {
	h := newTestHarness(t)

	rec := serveAPI(t, h.router, http.MethodPut, "/api/settings/logs.level", map[string]string{"value": "debug"}, "")

	assertStatus(t, rec, http.StatusUnauthorized)
	assertJSONField(t, rec, "error", "authentication required")
}

func TestSettingsUpdateRequiresOwner(t *testing.T) {
	h := newTestHarness(t)
	h.createOwner(t, "testowner", "testpass123")
	h.createViewer(t, "testviewer", "viewerpass123")
	cookie := h.login(t, "testviewer", "viewerpass123")

	rec := serveAPI(t, h.router, http.MethodPut, "/api/settings/logs.level", map[string]string{"value": "debug"}, cookie)

	assertStatus(t, rec, http.StatusForbidden)
	assertJSONField(t, rec, "error", "permission denied")
}

func TestSettingsUpdateSavesValue(t *testing.T) {
	h := newTestHarness(t)
	h.createOwner(t, "testowner", "testpass123")
	cookie := h.login(t, "testowner", "testpass123")

	rec := serveAPI(t, h.router, http.MethodPut, "/api/settings/logs.level", map[string]string{"value": "debug"}, cookie)

	assertStatus(t, rec, http.StatusOK)
	var body map[string]any
	decodeJSON(t, rec, &body)
	if key, ok := body["key"].(string); !ok || key != "logs.level" {
		t.Fatalf("expected key 'logs.level' in response, got %v", body)
	}
	if val, ok := body["label"].(string); !ok || val != "Log Level" {
		t.Fatalf("expected label 'Log Level' in response, got %v", body)
	}
}

func TestSettingsUpdateRejectsUnknownKey(t *testing.T) {
	h := newTestHarness(t)
	h.createOwner(t, "testowner", "testpass123")
	cookie := h.login(t, "testowner", "testpass123")

	rec := serveAPI(t, h.router, http.MethodPut, "/api/settings/nonexistent.key", map[string]string{"value": "test"}, cookie)

	assertStatus(t, rec, http.StatusBadRequest)
	assertJSONField(t, rec, "error", "invalid request")
}

func TestSettingsUpdateRejectsInvalidToggle(t *testing.T) {
	h := newTestHarness(t)
	h.createOwner(t, "testowner", "testpass123")
	cookie := h.login(t, "testowner", "testpass123")

	rec := serveAPI(t, h.router, http.MethodPut, "/api/settings/service.restart_on_failure", map[string]string{"value": "yes"}, cookie)

	assertStatus(t, rec, http.StatusBadRequest)
	assertJSONField(t, rec, "error", "invalid request")
}

func TestRestartStatusRequiresAuth(t *testing.T) {
	h := newTestHarness(t)

	rec := serveAPI(t, h.router, http.MethodGet, "/api/settings/restart-status", nil, "")

	assertStatus(t, rec, http.StatusUnauthorized)
	assertJSONField(t, rec, "error", "authentication required")
}

func TestRestartStatusReturnsNotRequired(t *testing.T) {
	h := newTestHarness(t)
	h.createOwner(t, "testowner", "testpass123")
	cookie := h.login(t, "testowner", "testpass123")

	rec := serveAPI(t, h.router, http.MethodGet, "/api/settings/restart-status", nil, cookie)

	assertStatus(t, rec, http.StatusOK)
	var body map[string]any
	decodeJSON(t, rec, &body)
	rr, ok := body["restart_required"].(bool)
	if !ok || rr != false {
		t.Fatalf("expected restart_required=false, got %v", body["restart_required"])
	}
	pc, ok := body["pending_changes"].([]any)
	if !ok {
		t.Fatalf("pending_changes is not an array in %v", body)
	}
	if len(pc) != 0 {
		t.Fatalf("expected empty pending_changes, got %v", pc)
	}
}

func TestRestartStatusViewerCanRead(t *testing.T) {
	h := newTestHarness(t)
	h.createOwner(t, "testowner", "testpass123")
	h.createViewer(t, "testviewer", "viewerpass123")
	cookie := h.login(t, "testviewer", "viewerpass123")

	rec := serveAPI(t, h.router, http.MethodGet, "/api/settings/restart-status", nil, cookie)

	assertStatus(t, rec, http.StatusOK)
}

func TestRestartRequiresAuth(t *testing.T) {
	h := newTestHarness(t)

	rec := serveAPI(t, h.router, http.MethodPost, "/api/settings/restart", nil, "")

	assertStatus(t, rec, http.StatusUnauthorized)
	assertJSONField(t, rec, "error", "authentication required")
}

func TestRestartRequiresOwner(t *testing.T) {
	h := newTestHarness(t)
	h.createOwner(t, "testowner", "testpass123")
	h.createViewer(t, "testviewer", "viewerpass123")
	cookie := h.login(t, "testviewer", "viewerpass123")

	rec := serveAPI(t, h.router, http.MethodPost, "/api/settings/restart", nil, cookie)

	assertStatus(t, rec, http.StatusForbidden)
	assertJSONField(t, rec, "error", "permission denied")
}

func TestRestartReturnsPlaceholder(t *testing.T) {
	h := newTestHarness(t)
	h.createOwner(t, "testowner", "testpass123")
	cookie := h.login(t, "testowner", "testpass123")

	rec := serveAPI(t, h.router, http.MethodPost, "/api/settings/restart", nil, cookie)

	assertStatus(t, rec, http.StatusOK)
	assertJSONField(t, rec, "ok", true)
}

func TestSettingsSectionsGetIncludesValues(t *testing.T) {
	h := newTestHarness(t)
	h.createOwner(t, "testowner", "testpass123")
	cookie := h.login(t, "testowner", "testpass123")

	// Set a database-sourced setting value first
	rec := serveAPI(t, h.router, http.MethodPut, "/api/settings/logs.level", map[string]string{"value": "debug"}, cookie)
	assertStatus(t, rec, http.StatusOK)

	// Now get sections and verify the value is present
	rec = serveAPI(t, h.router, http.MethodGet, "/api/settings", nil, cookie)
	assertStatus(t, rec, http.StatusOK)

	var body map[string]any
	decodeJSON(t, rec, &body)
	sections, ok := body["sections"].([]any)
	if !ok {
		t.Fatalf("sections is not an array in %v", body)
	}
	// Find the logs section and verify it has a value field
	for _, sec := range sections {
		secMap, ok := sec.(map[string]any)
		if !ok {
			continue
		}
		if secMap["id"] != "logs" {
			continue
		}
		settings, ok := secMap["settings"].([]any)
		if !ok {
			t.Fatalf("settings is not an array in logs section")
		}
		for _, s := range settings {
			sMap, ok := s.(map[string]any)
			if !ok {
				continue
			}
			if sMap["key"] == "logs.level" {
				if _, hasValue := sMap["value"]; !hasValue {
					t.Fatalf("logs.level setting missing 'value' field in %v", sMap)
				}
				if sMap["value"] != "debug" {
					t.Fatalf("expected logs.level value='debug', got %v", sMap["value"])
				}
				return
			}
		}
	}
	t.Fatalf("logs.level setting not found in sections")
}
