package logs

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestFileLogReader_Available_FileExists(t *testing.T) {
	tmp := writeTempLog(t, "panel", `{"level":"info","time":"2026-05-07T10:00:00Z","message":"hello"}`)
	r := NewFileLogReader(tmp, SourcePanel)
	if !r.Available(context.Background()) {
		t.Fatal("expected available for existing file")
	}
}

func TestFileLogReader_Available_FileMissing(t *testing.T) {
	r := NewFileLogReader("/nonexistent/path.log", SourcePanel)
	if r.Available(context.Background()) {
		t.Fatal("expected not available for missing file")
	}
}

func TestFileLogReader_ReadLines_PanelJSON(t *testing.T) {
	lines := []string{
		`{"level":"info","time":"2026-05-07T10:00:00Z","message":"line1","component":"a"}`,
		`{"level":"warn","time":"2026-05-07T10:00:01Z","message":"line2","component":"b"}`,
		`{"level":"error","time":"2026-05-07T10:00:02Z","message":"line3","component":"c"}`,
	}
	tmp := writeTempLog(t, "panel", strings.Join(lines, "\n"))
	r := NewFileLogReader(tmp, SourcePanel)

	result, err := r.ReadLines(context.Background(), ReadOpts{Limit: 10, Order: "desc"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Lines) != 3 {
		t.Fatalf("got %d lines, want 3", len(result.Lines))
	}
	// desc order: newest first
	if result.Lines[0].Message != "line3" {
		t.Errorf("first line = %q, want line3 (newest)", result.Lines[0].Message)
	}
}

func TestFileLogReader_ReadLines_LevelFilter(t *testing.T) {
	lines := []string{
		`{"level":"info","time":"2026-05-07T10:00:00Z","message":"a"}`,
		`{"level":"error","time":"2026-05-07T10:00:01Z","message":"b"}`,
		`{"level":"info","time":"2026-05-07T10:00:02Z","message":"c"}`,
	}
	tmp := writeTempLog(t, "panel", strings.Join(lines, "\n"))
	r := NewFileLogReader(tmp, SourcePanel)

	result, err := r.ReadLines(context.Background(), ReadOpts{Limit: 10, Order: "desc", Level: "error"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Lines) != 1 {
		t.Fatalf("got %d lines, want 1", len(result.Lines))
	}
	if result.Lines[0].Message != "b" {
		t.Errorf("message = %q, want %q", result.Lines[0].Message, "b")
	}
}

func TestFileLogReader_ReadLines_SearchFilter(t *testing.T) {
	lines := []string{
		`{"level":"info","time":"2026-05-07T10:00:00Z","message":"user login"}`,
		`{"level":"info","time":"2026-05-07T10:00:01Z","message":"config saved"}`,
		`{"level":"info","time":"2026-05-07T10:00:02Z","message":"user logout"}`,
	}
	tmp := writeTempLog(t, "panel", strings.Join(lines, "\n"))
	r := NewFileLogReader(tmp, SourcePanel)

	result, err := r.ReadLines(context.Background(), ReadOpts{Limit: 10, Order: "desc", Search: "user"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Lines) != 2 {
		t.Fatalf("got %d lines, want 2", len(result.Lines))
	}
}

func TestFileLogReader_ReadLines_TimeRange(t *testing.T) {
	lines := []string{
		`{"level":"info","time":"2026-05-07T08:00:00Z","message":"early"}`,
		`{"level":"info","time":"2026-05-07T10:00:00Z","message":"mid"}`,
		`{"level":"info","time":"2026-05-07T12:00:00Z","message":"late"}`,
	}
	tmp := writeTempLog(t, "panel", strings.Join(lines, "\n"))
	r := NewFileLogReader(tmp, SourcePanel)

	from := time.Date(2026, 5, 7, 9, 0, 0, 0, time.UTC)
	to := time.Date(2026, 5, 7, 11, 0, 0, 0, time.UTC)
	result, err := r.ReadLines(context.Background(), ReadOpts{Limit: 10, Order: "desc", From: from, To: to})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Lines) != 1 {
		t.Fatalf("got %d lines, want 1", len(result.Lines))
	}
	if result.Lines[0].Message != "mid" {
		t.Errorf("message = %q, want %q", result.Lines[0].Message, "mid")
	}
}

func TestFileLogReader_ReadLines_Pagination(t *testing.T) {
	var lines []string
	for i := 0; i < 10; i++ {
		lines = append(lines, `{"level":"info","time":"2026-05-07T10:00:0`+string(rune('0'+i))+`Z","message":"line`+string(rune('0'+i))+`"}`)
	}
	tmp := writeTempLog(t, "panel", strings.Join(lines, "\n"))
	r := NewFileLogReader(tmp, SourcePanel)

	result, err := r.ReadLines(context.Background(), ReadOpts{Limit: 3, Offset: 2, Order: "desc"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Lines) != 3 {
		t.Fatalf("got %d lines, want 3", len(result.Lines))
	}
	if !result.HasMore {
		t.Error("expected HasMore=true")
	}
	if result.Total != 10 {
		t.Errorf("total = %d, want 10", result.Total)
	}
}

func TestFileLogReader_ReadLines_OcservSyslog(t *testing.T) {
	lines := []string{
		"May  7 10:00:00 host ocserv[1]: main: started",
		"May  7 10:00:01 host ocserv[1]: worker: error connecting",
	}
	tmp := writeTempLog(t, "ocserv", strings.Join(lines, "\n"))
	r := NewFileLogReader(tmp, SourceOcserv)

	result, err := r.ReadLines(context.Background(), ReadOpts{Limit: 10, Order: "desc"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Lines) != 2 {
		t.Fatalf("got %d lines, want 2", len(result.Lines))
	}
	if result.Lines[0].Level != "error" {
		t.Errorf("first line level = %q, want error (newest, desc)", result.Lines[0].Level)
	}
}

func writeTempLog(t *testing.T, prefix, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, prefix+".log")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write temp log: %v", err)
	}
	return path
}
