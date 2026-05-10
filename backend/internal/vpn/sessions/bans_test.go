package sessions_test

import (
	"testing"
	"time"

	"ocserv_panel/backend/internal/vpn/sessions"
)

// --- EventsJSON tests ---

func TestEventsJSONValid(t *testing.T) {
	input := []byte(`[
		{
			"timestamp": "2026-05-07T12:00:00Z",
			"username": "testuser",
			"remote_ip": "192.168.1.100",
			"event_type": "connect",
			"message": "user connected successfully"
		},
		{
			"timestamp": "2026-05-07T11:55:00Z",
			"username": "baduser",
			"remote_ip": "10.0.0.5",
			"event_type": "auth_failure",
			"message": "authentication failed"
		}
	]`)

	events, err := sessions.EventsJSON(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}

	if events[0].Username != "testuser" {
		t.Errorf("events[0].Username = %q, want %q", events[0].Username, "testuser")
	}
	if events[0].RemoteIP != "192.168.1.100" {
		t.Errorf("events[0].RemoteIP = %q, want %q", events[0].RemoteIP, "192.168.1.100")
	}
	if events[0].EventType != "connect" {
		t.Errorf("events[0].EventType = %q, want %q", events[0].EventType, "connect")
	}
	expectedTime := time.Date(2026, 5, 7, 12, 0, 0, 0, time.UTC)
	if !events[0].Timestamp.Equal(expectedTime) {
		t.Errorf("events[0].Timestamp = %v, want %v", events[0].Timestamp, expectedTime)
	}

	if events[1].Username != "baduser" {
		t.Errorf("events[1].Username = %q, want %q", events[1].Username, "baduser")
	}
}

func TestEventsJSONEmpty(t *testing.T) {
	tests := []struct {
		name  string
		input []byte
	}{
		{"empty byte slice", []byte{}},
		{"nil byte slice", nil},
		{"empty JSON array", []byte(`[]`)},
		{"null JSON", []byte(`null`)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			events, err := sessions.EventsJSON(tt.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if events == nil {
				t.Fatal("expected non-nil slice, got nil")
			}
			if len(events) != 0 {
				t.Errorf("expected empty slice, got %d elements", len(events))
			}
		})
	}
}

func TestEventsJSONMalformed(t *testing.T) {
	tests := []struct {
		name  string
		input []byte
	}{
		{"invalid JSON", []byte(`{not json}`)},
		{"truncated JSON", []byte(`[{"timestamp":`)},
		{"wrong type", []byte(`"just a string"`)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := sessions.EventsJSON(tt.input)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
		})
	}
}

// --- BannedIPsJSON tests ---

func TestBannedIPsJSONValid(t *testing.T) {
	input := []byte(`[
		{
			"ip": "192.168.1.50",
			"score": 80,
			"reason": "too many authentication failures"
		},
		{
			"ip": "10.0.0.99",
			"score": 100,
			"reason": "brute force detected"
		}
	]`)

	bans, err := sessions.BannedIPsJSON(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(bans) != 2 {
		t.Fatalf("expected 2 bans, got %d", len(bans))
	}

	if bans[0].IP != "192.168.1.50" {
		t.Errorf("bans[0].IP = %q, want %q", bans[0].IP, "192.168.1.50")
	}
	if bans[0].Score != 80 {
		t.Errorf("bans[0].Score = %d, want %d", bans[0].Score, 80)
	}
	if bans[0].Reason != "too many authentication failures" {
		t.Errorf("bans[0].Reason = %q, want %q", bans[0].Reason, "too many authentication failures")
	}
}

func TestBannedIPsJSONEmpty(t *testing.T) {
	tests := []struct {
		name  string
		input []byte
	}{
		{"empty byte slice", []byte{}},
		{"nil byte slice", nil},
		{"empty JSON array", []byte(`[]`)},
		{"null JSON", []byte(`null`)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bans, err := sessions.BannedIPsJSON(tt.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if bans == nil {
				t.Fatal("expected non-nil slice, got nil")
			}
			if len(bans) != 0 {
				t.Errorf("expected empty slice, got %d elements", len(bans))
			}
		})
	}
}

func TestBannedIPsJSONMalformed(t *testing.T) {
	tests := []struct {
		name  string
		input []byte
	}{
		{"invalid JSON", []byte(`{not json}`)},
		{"truncated JSON", []byte(`[{"ip":`)},
		{"wrong type", []byte(`42`)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := sessions.BannedIPsJSON(tt.input)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
		})
	}
}

// --- BanPointsJSON tests ---

func TestBanPointsJSONValid(t *testing.T) {
	input := []byte(`[
		{
			"ip": "10.0.0.1",
			"points": 30,
			"since": "2026-05-06T10:00:00Z"
		},
		{
			"ip": "172.16.0.5",
			"points": 10,
			"since": "2026-05-07T08:30:00Z"
		}
	]`)

	entries, err := sessions.BanPointsJSON(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}

	if entries[0].IP != "10.0.0.1" {
		t.Errorf("entries[0].IP = %q, want %q", entries[0].IP, "10.0.0.1")
	}
	if entries[0].Points != 30 {
		t.Errorf("entries[0].Points = %d, want %d", entries[0].Points, 30)
	}
	expectedTime := time.Date(2026, 5, 6, 10, 0, 0, 0, time.UTC)
	if !entries[0].Since.Equal(expectedTime) {
		t.Errorf("entries[0].Since = %v, want %v", entries[0].Since, expectedTime)
	}
}

func TestBanPointsJSONEmpty(t *testing.T) {
	tests := []struct {
		name  string
		input []byte
	}{
		{"empty byte slice", []byte{}},
		{"nil byte slice", nil},
		{"empty JSON array", []byte(`[]`)},
		{"null JSON", []byte(`null`)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entries, err := sessions.BanPointsJSON(tt.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if entries == nil {
				t.Fatal("expected non-nil slice, got nil")
			}
			if len(entries) != 0 {
				t.Errorf("expected empty slice, got %d elements", len(entries))
			}
		})
	}
}

func TestBanPointsJSONMalformed(t *testing.T) {
	tests := []struct {
		name  string
		input []byte
	}{
		{"invalid JSON", []byte(`{not json}`)},
		{"truncated JSON", []byte(`[{"ip":`)},
		{"wrong type", []byte(`true`)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := sessions.BanPointsJSON(tt.input)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
		})
	}
}
