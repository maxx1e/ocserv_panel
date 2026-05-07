package logs

import (
	"testing"
	"time"
)

func TestParsePanelLine_ValidJSON(t *testing.T) {
	raw := `{"level":"info","time":"2026-05-07T10:23:45Z","message":"request completed","component":"server","request_id":"abc123"}`
	line, err := ParsePanelLine(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if line.Level != "info" {
		t.Errorf("level = %q, want %q", line.Level, "info")
	}
	if line.Message != "request completed" {
		t.Errorf("message = %q, want %q", line.Message, "request completed")
	}
	if line.Component != "server" {
		t.Errorf("component = %q, want %q", line.Component, "server")
	}
	if line.Metadata["request_id"] != "abc123" {
		t.Errorf("metadata[request_id] = %q, want %q", line.Metadata["request_id"], "abc123")
	}
	wantTime := time.Date(2026, 5, 7, 10, 23, 45, 0, time.UTC)
	if !line.Timestamp.Equal(wantTime) {
		t.Errorf("timestamp = %v, want %v", line.Timestamp, wantTime)
	}
	if line.Raw != raw {
		t.Errorf("raw mismatch")
	}
}

func TestParsePanelLine_MalformedJSON(t *testing.T) {
	_, err := ParsePanelLine("not json at all")
	if err == nil {
		t.Fatal("expected error for malformed JSON")
	}
}

func TestParsePanelLine_MissingFields(t *testing.T) {
	raw := `{"level":"warn","message":"partial"}`
	line, err := ParsePanelLine(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if line.Level != "warn" {
		t.Errorf("level = %q, want %q", line.Level, "warn")
	}
	if line.Component != "" {
		t.Errorf("component = %q, want empty", line.Component)
	}
}

func TestParseOcservLine_Standard(t *testing.T) {
	raw := "May  7 10:23:45 myhost ocserv[1234]: worker: connected user 'admin' from 192.168.1.1"
	line, err := ParseOcservLine(raw, 2026)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if line.Level != "info" {
		t.Errorf("level = %q, want %q", line.Level, "info")
	}
	if line.Component != "worker" {
		t.Errorf("component = %q, want %q", line.Component, "worker")
	}
	if line.Raw != raw {
		t.Errorf("raw mismatch")
	}
	wantTime := time.Date(2026, 5, 7, 10, 23, 45, 0, time.UTC)
	if !line.Timestamp.Equal(wantTime) {
		t.Errorf("timestamp = %v, want %v", line.Timestamp, wantTime)
	}
}

func TestParseOcservLine_ErrorLevel(t *testing.T) {
	raw := "May  7 10:00:00 host ocserv[99]: sec-mod: error connecting to client"
	line, err := ParseOcservLine(raw, 2026)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if line.Level != "error" {
		t.Errorf("level = %q, want %q", line.Level, "error")
	}
}

func TestParseOcservLine_WarnLevel(t *testing.T) {
	raw := "May  7 10:00:00 host ocserv[99]: main: warning: no IPv6 configured"
	line, err := ParseOcservLine(raw, 2026)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if line.Level != "warn" {
		t.Errorf("level = %q, want %q", line.Level, "warn")
	}
}

func TestParseOcservLine_Malformed(t *testing.T) {
	_, err := ParseOcservLine("totally invalid line", 2026)
	if err == nil {
		t.Fatal("expected error for malformed syslog")
	}
}
