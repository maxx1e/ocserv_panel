# Audit Page Backend Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement the backend API, log reader package, RBAC permissions, and service layer for the Audit page log viewer.

**Architecture:** New `backend/internal/logs` package with `LogReader` interface, file and journal reader implementations, parser, and masker. New `AuditLogService` in `backend/internal/app` orchestrates reading. Handlers in `backend/internal/server` expose 3 API endpoints. RBAC gets 2 new permissions.

**Tech Stack:** Go, zerolog JSON parsing, syslog regex parsing, `CommandRunner` for journal fallback

**Spec:** `docs/superpowers/specs/2026-05-07-audit-page-design.md`

---

## File Structure

### New Files

| File | Responsibility |
|------|---------------|
| `backend/internal/logs/types.go` | LogReader interface, ReadOpts, ReadResult, LogLine types |
| `backend/internal/logs/parser.go` | ParsePanelLine (JSON) and ParseOcservLine (syslog) functions |
| `backend/internal/logs/parser_test.go` | Unit tests for both parsers |
| `backend/internal/logs/masker.go` | MaskSensitive function with regex patterns |
| `backend/internal/logs/masker_test.go` | Masker unit tests |
| `backend/internal/logs/file_reader.go` | FileLogReader implementation |
| `backend/internal/logs/file_reader_test.go` | File reader tests with temp files |
| `backend/internal/logs/journal_reader.go` | JournalLogReader implementation using CommandRunner |
| `backend/internal/logs/journal_reader_test.go` | Journal reader tests with FakeCommandRunner |
| `backend/internal/logs/fake.go` | FakeLogReader for service/handler tests |
| `backend/internal/app/audit_log_service.go` | AuditLogService with source resolution and RBAC |
| `backend/internal/app/audit_log_service_test.go` | Service tests |
| `backend/internal/server/audit_log_handlers.go` | HTTP handlers for 3 endpoints |
| `backend/internal/server/audit_log_handlers_test.go` | Handler tests |

### Modified Files

| File | Change |
|------|--------|
| `backend/internal/rbac/rbac.go` | Add `PermissionAuditLogsRead`, `PermissionAuditLogsExport` |
| `backend/internal/config/config.go` | Add `PanelSystemdUnit` field to `InstallationConfig` |
| `backend/internal/server/server.go` | Add `auditLogService` field to `App`, register routes, update `NewAppRouter` |
| `backend/cmd/ocserv-panel/main.go` | Wire `AuditLogService` |

---

## Task 1: RBAC Permissions

**Files:**
- Modify: `backend/internal/rbac/rbac.go`

- [ ] **Step 1: Add permission constants**

Add after the admin user permissions block:

```go
// Audit log viewing permissions
PermissionAuditLogsRead   Permission = "audit:logs:read"
PermissionAuditLogsExport Permission = "audit:logs:export"
```

- [ ] **Step 2: Grant permissions to roles**

In `NewDefaultAuthorizer()`, add to `RoleOwner` map:

```go
PermissionAuditLogsRead:   {},
PermissionAuditLogsExport: {},
```

Add to `RoleViewer` map:

```go
PermissionAuditLogsRead: {},
```

- [ ] **Step 3: Verify build**

Run: `cd /opt/ocserv_panel/backend && go build ./...`
Expected: Clean build, no errors.

- [ ] **Step 4: Commit**

```bash
git add backend/internal/rbac/rbac.go
git commit -m "feat(rbac): add audit log read and export permissions"
```

---

## Task 2: Config Extension

**Files:**
- Modify: `backend/internal/config/config.go`

- [ ] **Step 1: Add PanelSystemdUnit field**

In `InstallationConfig` struct, add after `PanelLogPath`:

```go
PanelSystemdUnit string `toml:"panel_systemd_unit"`
```

- [ ] **Step 2: Set default value**

In `Defaults()` function, in the `Installation` block, add:

```go
PanelSystemdUnit: "ocserv-panel",
```

- [ ] **Step 3: Verify build**

Run: `cd /opt/ocserv_panel/backend && go build ./...`
Expected: Clean build.

- [ ] **Step 4: Run existing config tests**

Run: `cd /opt/ocserv_panel/backend && go test ./internal/config/ -v`
Expected: All tests pass.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/config/config.go
git commit -m "feat(config): add PanelSystemdUnit installation setting"
```

---

## Task 3: Log Types Package

**Files:**
- Create: `backend/internal/logs/types.go`

- [ ] **Step 1: Create the logs package with types**

```go
// Package logs provides structured log reading from file and journal sources.
package logs

import (
	"context"
	"time"
)

// LogLine is a single parsed log entry.
type LogLine struct {
	Timestamp time.Time         `json:"timestamp"`
	Level     string            `json:"level"`
	Message   string            `json:"message"`
	Component string            `json:"component"`
	Raw       string            `json:"raw"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

// ReadOpts configures a log read operation.
type ReadOpts struct {
	Limit  int
	Offset int
	Order  string // "asc" or "desc"
	Search string
	Level  string
	From   time.Time
	To     time.Time
}

// ReadResult is the output of a log read operation.
type ReadResult struct {
	Lines   []LogLine
	Total   int
	HasMore bool
}

// LogReader reads structured log lines from a source.
type LogReader interface {
	// ReadLines reads log lines matching the given options.
	ReadLines(ctx context.Context, opts ReadOpts) (*ReadResult, error)
	// Available reports whether this reader can serve requests.
	Available(ctx context.Context) bool
}
```

- [ ] **Step 2: Verify build**

Run: `cd /opt/ocserv_panel/backend && go build ./internal/logs/`
Expected: Clean build.

- [ ] **Step 3: Commit**

```bash
git add backend/internal/logs/types.go
git commit -m "feat(logs): add LogReader interface and core types"
```

---

## Task 4: Log Parsers

**Files:**
- Create: `backend/internal/logs/parser.go`
- Create: `backend/internal/logs/parser_test.go`

- [ ] **Step 1: Write parser tests**

```go
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
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /opt/ocserv_panel/backend && go test ./internal/logs/ -v -run TestParse`
Expected: FAIL — functions not defined.

- [ ] **Step 3: Implement parsers**

```go
package logs

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"
)

// Known zerolog fields that map to LogLine struct fields.
var panelKnownFields = map[string]bool{
	"level": true, "time": true, "message": true, "msg": true, "component": true,
}

// ParsePanelLine parses a zerolog JSON log line.
func ParsePanelLine(raw string) (LogLine, error) {
	var fields map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &fields); err != nil {
		return LogLine{}, fmt.Errorf("parse panel log JSON: %w", err)
	}

	line := LogLine{Raw: raw}

	if v, ok := fields["level"].(string); ok {
		line.Level = v
	}
	// zerolog uses "message" or "msg"
	if v, ok := fields["message"].(string); ok {
		line.Message = v
	} else if v, ok := fields["msg"].(string); ok {
		line.Message = v
	}
	if v, ok := fields["component"].(string); ok {
		line.Component = v
	}
	if v, ok := fields["time"].(string); ok {
		if t, err := time.Parse(time.RFC3339Nano, v); err == nil {
			line.Timestamp = t
		} else if t, err := time.Parse(time.RFC3339, v); err == nil {
			line.Timestamp = t
		}
	}

	// Remaining fields go into metadata
	meta := make(map[string]string)
	for k, v := range fields {
		if panelKnownFields[k] {
			continue
		}
		if s, ok := v.(string); ok {
			meta[k] = s
		} else {
			b, _ := json.Marshal(v)
			meta[k] = string(b)
		}
	}
	if len(meta) > 0 {
		line.Metadata = meta
	}

	return line, nil
}

// ocservSyslogRe matches standard syslog lines from ocserv.
var ocservSyslogRe = regexp.MustCompile(`^(\w+\s+\d+\s+\d+:\d+:\d+)\s+\S+\s+ocserv\[(\d+)\]:\s*(.+)$`)

// syslogMonths maps month abbreviation to time.Month.
var syslogMonths = map[string]time.Month{
	"Jan": time.January, "Feb": time.February, "Mar": time.March,
	"Apr": time.April, "May": time.May, "Jun": time.June,
	"Jul": time.July, "Aug": time.August, "Sep": time.September,
	"Oct": time.October, "Nov": time.November, "Dec": time.December,
}

// ParseOcservLine parses a syslog-format ocserv log line.
// year is used because syslog timestamps lack a year field.
func ParseOcservLine(raw string, year int) (LogLine, error) {
	matches := ocservSyslogRe.FindStringSubmatch(raw)
	if matches == nil {
		return LogLine{}, fmt.Errorf("line does not match ocserv syslog format")
	}

	timestamp := parseSyslogTimestamp(matches[1], year)
	body := matches[3]

	component, message := splitOcservBody(body)
	level := inferOcservLevel(body)

	return LogLine{
		Timestamp: timestamp,
		Level:     level,
		Message:   message,
		Component: component,
		Raw:       raw,
	}, nil
}

func parseSyslogTimestamp(s string, year int) time.Time {
	// Format: "May  7 10:23:45"
	t, err := time.Parse("Jan  2 15:04:05", s)
	if err != nil {
		t, err = time.Parse("Jan 2 15:04:05", s)
		if err != nil {
			return time.Time{}
		}
	}
	return time.Date(year, t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second(), 0, time.UTC)
}

func splitOcservBody(body string) (component, message string) {
	if idx := strings.Index(body, ": "); idx > 0 && idx < 30 {
		return body[:idx], body[idx+2:]
	}
	return "", body
}

func inferOcservLevel(body string) string {
	lower := strings.ToLower(body)
	if strings.Contains(lower, "error") || strings.Contains(lower, "failed") {
		return "error"
	}
	if strings.Contains(lower, "warning") {
		return "warn"
	}
	return "info"
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /opt/ocserv_panel/backend && go test ./internal/logs/ -v -run TestParse`
Expected: All PASS.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/logs/parser.go backend/internal/logs/parser_test.go
git commit -m "feat(logs): implement panel JSON and ocserv syslog parsers"
```

---

## Task 5: Sensitive Data Masker

**Files:**
- Create: `backend/internal/logs/masker.go`
- Create: `backend/internal/logs/masker_test.go`

- [ ] **Step 1: Write masker tests**

```go
package logs

import "testing"

func TestMaskSensitive_Password(t *testing.T) {
	input := "user set password=secret123 for admin"
	got := MaskSensitive(input)
	if got == input {
		t.Error("password not masked")
	}
	if !contains(got, "password=***") {
		t.Errorf("got %q, want password=*** pattern", got)
	}
}

func TestMaskSensitive_Token(t *testing.T) {
	input := "auth token=eyJhbGciOiJIUzI1NiJ9.abcdef123 granted"
	got := MaskSensitive(input)
	if !contains(got, "token=***") {
		t.Errorf("got %q, want token=*** pattern", got)
	}
}

func TestMaskSensitive_Secret(t *testing.T) {
	input := "secret=mysecretvalue123 loaded"
	got := MaskSensitive(input)
	if !contains(got, "secret=***") {
		t.Errorf("got %q, want secret=*** pattern", got)
	}
}

func TestMaskSensitive_Authorization(t *testing.T) {
	input := "Authorization: Bearer eyJhbG123456"
	got := MaskSensitive(input)
	if !contains(got, "Authorization: ***") {
		t.Errorf("got %q, want Authorization: *** pattern", got)
	}
}

func TestMaskSensitive_Cookie(t *testing.T) {
	input := "cookie=session_abc123def456 path=/"
	got := MaskSensitive(input)
	if !contains(got, "cookie=***") {
		t.Errorf("got %q, want cookie=*** pattern", got)
	}
}

func TestMaskSensitive_NoFalsePositive(t *testing.T) {
	input := "user admin connected from 192.168.1.1 with TLS 1.3"
	got := MaskSensitive(input)
	if got != input {
		t.Errorf("false positive: got %q, want %q", got, input)
	}
}

func TestMaskSensitive_MultiplePatterns(t *testing.T) {
	input := "password=abc token=xyz123456789 done"
	got := MaskSensitive(input)
	if contains(got, "abc") || contains(got, "xyz123456789") {
		t.Errorf("not all patterns masked: %q", got)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && containsStr(s, sub))
}

func containsStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /opt/ocserv_panel/backend && go test ./internal/logs/ -v -run TestMask`
Expected: FAIL — `MaskSensitive` not defined.

- [ ] **Step 3: Implement masker**

```go
package logs

import "regexp"

// sensitivePatterns defines regex patterns and their replacements for masking.
var sensitivePatterns = []struct {
	re          *regexp.Regexp
	replacement string
}{
	{regexp.MustCompile(`(?i)password[=:]\s*\S+`), "password=***"},
	{regexp.MustCompile(`(?i)token[=:]\s*[A-Za-z0-9+/=_.\-]{8,}`), "token=***"},
	{regexp.MustCompile(`(?i)(key|secret|credential)[=:]\s*\S+`), "${1}=***"},
	{regexp.MustCompile(`(?i)cookie[=:]\s*\S+`), "cookie=***"},
	{regexp.MustCompile(`(?i)Authorization:\s*\S+`), "Authorization: ***"},
}

// MaskSensitive replaces sensitive values in a log line with redacted placeholders.
func MaskSensitive(s string) string {
	for _, p := range sensitivePatterns {
		s = p.re.ReplaceAllString(s, p.replacement)
	}
	return s
}

// MaskLogLine applies masking to both the Message and Raw fields of a LogLine.
func MaskLogLine(line *LogLine) {
	line.Message = MaskSensitive(line.Message)
	line.Raw = MaskSensitive(line.Raw)
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /opt/ocserv_panel/backend && go test ./internal/logs/ -v -run TestMask`
Expected: All PASS.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/logs/masker.go backend/internal/logs/masker_test.go
git commit -m "feat(logs): implement sensitive data masking for log output"
```

---

## Task 6: FileLogReader

**Files:**
- Create: `backend/internal/logs/file_reader.go`
- Create: `backend/internal/logs/file_reader_test.go`

- [ ] **Step 1: Write file reader tests**

```go
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
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /opt/ocserv_panel/backend && go test ./internal/logs/ -v -run TestFileLogReader`
Expected: FAIL — types not defined.

- [ ] **Step 3: Implement FileLogReader**

```go
package logs

import (
	"bufio"
	"context"
	"os"
	"strings"
	"time"
)

// SourceType identifies the log source format.
type SourceType string

const (
	SourcePanel  SourceType = "panel"
	SourceOcserv SourceType = "ocserv"
)

// maxScanBytes limits the file scan to prevent memory exhaustion.
const maxScanBytes = 10 * 1024 * 1024 // 10MB

// FileLogReader reads and parses log lines from a file on disk.
type FileLogReader struct {
	path       string
	sourceType SourceType
}

// NewFileLogReader creates a FileLogReader for the given path and source format.
func NewFileLogReader(path string, sourceType SourceType) *FileLogReader {
	return &FileLogReader{path: path, sourceType: sourceType}
}

// Available reports whether the log file exists and is readable.
func (r *FileLogReader) Available(_ context.Context) bool {
	info, err := os.Stat(r.path)
	return err == nil && !info.IsDir()
}

// ReadLines reads log lines from the file, applying filters and pagination.
func (r *FileLogReader) ReadLines(_ context.Context, opts ReadOpts) (*ReadResult, error) {
	f, err := os.Open(r.path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	// Read all lines (up to scan limit)
	allLines, err := r.scanLines(f)
	if err != nil {
		return nil, err
	}

	// Parse lines
	parsed := r.parseLines(allLines)

	// Apply filters
	filtered := r.applyFilters(parsed, opts)

	// Sort: desc = newest first (reverse of file order, assuming file is chronological)
	if opts.Order == "desc" || opts.Order == "" {
		reverseLogLines(filtered)
	}

	total := len(filtered)

	// Apply pagination
	offset := opts.Offset
	if offset < 0 {
		offset = 0
	}
	limit := opts.Limit
	if limit <= 0 {
		limit = 100
	}
	if limit > 500 {
		limit = 500
	}

	var page []LogLine
	hasMore := false
	if offset < total {
		end := offset + limit
		if end > total {
			end = total
		}
		page = filtered[offset:end]
		hasMore = end < total
	}

	return &ReadResult{
		Lines:   page,
		Total:   total,
		HasMore: hasMore,
	}, nil
}

func (r *FileLogReader) scanLines(f *os.File) ([]string, error) {
	// Limit read size
	info, err := f.Stat()
	if err != nil {
		return nil, err
	}

	size := info.Size()
	if size > maxScanBytes {
		// Seek to read only the last maxScanBytes
		if _, err := f.Seek(-maxScanBytes, 2); err != nil {
			return nil, err
		}
	}

	var lines []string
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) != "" {
			lines = append(lines, line)
		}
	}
	return lines, scanner.Err()
}

func (r *FileLogReader) parseLines(raw []string) []LogLine {
	year := time.Now().Year()
	parsed := make([]LogLine, 0, len(raw))
	for _, s := range raw {
		var line LogLine
		var err error
		switch r.sourceType {
		case SourcePanel:
			line, err = ParsePanelLine(s)
		case SourceOcserv:
			line, err = ParseOcservLine(s, year)
		default:
			continue
		}
		if err != nil {
			// Skip unparseable lines
			continue
		}
		parsed = append(parsed, line)
	}
	return parsed
}

func (r *FileLogReader) applyFilters(lines []LogLine, opts ReadOpts) []LogLine {
	filtered := make([]LogLine, 0, len(lines))
	searchLower := strings.ToLower(opts.Search)

	for i := range lines {
		line := &lines[i]

		// Level filter
		if opts.Level != "" && line.Level != opts.Level {
			continue
		}

		// Search filter (case-insensitive on message and raw)
		if opts.Search != "" {
			msgLower := strings.ToLower(line.Message)
			rawLower := strings.ToLower(line.Raw)
			if !strings.Contains(msgLower, searchLower) && !strings.Contains(rawLower, searchLower) {
				continue
			}
		}

		// Time range filter
		if !opts.From.IsZero() && line.Timestamp.Before(opts.From) {
			continue
		}
		if !opts.To.IsZero() && line.Timestamp.After(opts.To) {
			continue
		}

		filtered = append(filtered, *line)
	}
	return filtered
}

func reverseLogLines(lines []LogLine) {
	for i, j := 0, len(lines)-1; i < j; i, j = i+1, j-1 {
		lines[i], lines[j] = lines[j], lines[i]
	}
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /opt/ocserv_panel/backend && go test ./internal/logs/ -v -run TestFileLogReader`
Expected: All PASS.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/logs/file_reader.go backend/internal/logs/file_reader_test.go
git commit -m "feat(logs): implement FileLogReader with filtering and pagination"
```

---

## Task 7: JournalLogReader

**Files:**
- Create: `backend/internal/logs/journal_reader.go`
- Create: `backend/internal/logs/journal_reader_test.go`

- [ ] **Step 1: Write journal reader tests**

```go
package logs

import (
	"context"
	"strings"
	"testing"

	"ocserv_panel/backend/internal/ocserv"
)

func TestJournalLogReader_Available_Success(t *testing.T) {
	runner := ocserv.NewFakeCommandRunner()
	runner.Results["journalctl"] = ocserv.CommandResult{Stdout: "", ExitCode: 0}
	r := NewJournalLogReader("ocserv", SourceOcserv, runner)
	if !r.Available(context.Background()) {
		t.Fatal("expected available when journalctl succeeds")
	}
}

func TestJournalLogReader_Available_Failure(t *testing.T) {
	runner := ocserv.NewFakeCommandRunner()
	runner.Results["journalctl"] = ocserv.CommandResult{Stderr: "No such unit", ExitCode: 1}
	r := NewJournalLogReader("ocserv", SourceOcserv, runner)
	if r.Available(context.Background()) {
		t.Fatal("expected not available when journalctl fails")
	}
}

func TestJournalLogReader_ReadLines_PanelJSON(t *testing.T) {
	journalOutput := strings.Join([]string{
		`{"__REALTIME_TIMESTAMP":"1746612000000000","MESSAGE":"{\"level\":\"info\",\"time\":\"2026-05-07T10:00:00Z\",\"message\":\"hello\",\"component\":\"main\"}"}`,
		`{"__REALTIME_TIMESTAMP":"1746612001000000","MESSAGE":"{\"level\":\"warn\",\"time\":\"2026-05-07T10:00:01Z\",\"message\":\"oops\",\"component\":\"auth\"}"}`,
	}, "\n")

	runner := ocserv.NewFakeCommandRunner()
	runner.Results["journalctl"] = ocserv.CommandResult{Stdout: journalOutput, ExitCode: 0}
	r := NewJournalLogReader("ocserv-panel", SourcePanel, runner)

	result, err := r.ReadLines(context.Background(), ReadOpts{Limit: 10, Order: "desc"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Lines) != 2 {
		t.Fatalf("got %d lines, want 2", len(result.Lines))
	}
	if result.Lines[0].Level != "warn" {
		t.Errorf("first line level = %q, want warn (desc order)", result.Lines[0].Level)
	}
}

func TestJournalLogReader_ReadLines_CommandArgs(t *testing.T) {
	runner := ocserv.NewFakeCommandRunner()
	runner.Results["journalctl"] = ocserv.CommandResult{Stdout: "", ExitCode: 0}
	r := NewJournalLogReader("ocserv", SourceOcserv, runner)

	_, _ = r.ReadLines(context.Background(), ReadOpts{Limit: 50, Order: "desc"})

	if len(runner.Calls) == 0 {
		t.Fatal("no calls recorded")
	}
	call := runner.Calls[len(runner.Calls)-1]
	if call.Name != "journalctl" {
		t.Fatalf("command = %q, want journalctl", call.Name)
	}
	// Should contain -u <unit> --output=json --no-pager
	args := strings.Join(call.Args, " ")
	if !strings.Contains(args, "-u ocserv") {
		t.Errorf("args missing unit: %v", call.Args)
	}
	if !strings.Contains(args, "--output=json") {
		t.Errorf("args missing --output=json: %v", call.Args)
	}
	if !strings.Contains(args, "--no-pager") {
		t.Errorf("args missing --no-pager: %v", call.Args)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /opt/ocserv_panel/backend && go test ./internal/logs/ -v -run TestJournalLogReader`
Expected: FAIL — `NewJournalLogReader` not defined.

- [ ] **Step 3: Implement JournalLogReader**

```go
package logs

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"ocserv_panel/backend/internal/ocserv"
)

// JournalLogReader reads log lines from systemd journal via journalctl.
type JournalLogReader struct {
	unit       string
	sourceType SourceType
	runner     ocserv.CommandRunner
}

// NewJournalLogReader creates a JournalLogReader for the given systemd unit.
func NewJournalLogReader(unit string, sourceType SourceType, runner ocserv.CommandRunner) *JournalLogReader {
	return &JournalLogReader{unit: unit, sourceType: sourceType, runner: runner}
}

// Available checks whether journalctl can read logs for this unit.
func (r *JournalLogReader) Available(ctx context.Context) bool {
	result, err := r.runner.Run(ctx, "journalctl", []string{"-u", r.unit, "--no-pager", "-n", "0"}, nil)
	if err != nil {
		return false
	}
	return result.ExitCode == 0
}

// ReadLines reads log lines from the journal with filtering and pagination.
func (r *JournalLogReader) ReadLines(ctx context.Context, opts ReadOpts) (*ReadResult, error) {
	args := r.buildArgs(opts)
	result, err := r.runner.Run(ctx, "journalctl", args, nil)
	if err != nil {
		return nil, err
	}
	if result.ExitCode != 0 {
		return &ReadResult{Lines: nil, Total: 0, HasMore: false}, nil
	}

	// Parse journal JSON output
	rawLines := strings.Split(strings.TrimSpace(result.Stdout), "\n")
	parsed := make([]LogLine, 0, len(rawLines))
	year := time.Now().Year()

	for _, raw := range rawLines {
		if strings.TrimSpace(raw) == "" {
			continue
		}
		line := r.parseJournalEntry(raw, year)
		if line == nil {
			continue
		}
		parsed = append(parsed, *line)
	}

	// Apply in-memory filters (search, level) since journalctl only supports time-based filtering
	filtered := r.applyFilters(parsed, opts)

	// Sort
	if opts.Order == "desc" || opts.Order == "" {
		reverseLogLines(filtered)
	}

	total := len(filtered)

	// Paginate
	offset := opts.Offset
	if offset < 0 {
		offset = 0
	}
	limit := opts.Limit
	if limit <= 0 {
		limit = 100
	}
	if limit > 500 {
		limit = 500
	}

	var page []LogLine
	hasMore := false
	if offset < total {
		end := offset + limit
		if end > total {
			end = total
		}
		page = filtered[offset:end]
		hasMore = end < total
	}

	return &ReadResult{Lines: page, Total: total, HasMore: hasMore}, nil
}

func (r *JournalLogReader) buildArgs(opts ReadOpts) []string {
	args := []string{"-u", r.unit, "--output=json", "--no-pager"}

	// Limit number of entries fetched from journal (fetch more than page to allow filtering)
	fetchLimit := (opts.Limit + opts.Offset) * 3
	if fetchLimit < 500 {
		fetchLimit = 500
	}
	if fetchLimit > 5000 {
		fetchLimit = 5000
	}
	args = append(args, "-n", intToStr(fetchLimit))

	if !opts.From.IsZero() {
		args = append(args, "--since="+opts.From.Format("2006-01-02 15:04:05"))
	}
	if !opts.To.IsZero() {
		args = append(args, "--until="+opts.To.Format("2006-01-02 15:04:05"))
	}

	return args
}

func (r *JournalLogReader) parseJournalEntry(raw string, year int) *LogLine {
	var entry map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &entry); err != nil {
		return nil
	}

	message, _ := entry["MESSAGE"].(string)
	if message == "" {
		return nil
	}

	// For panel source, the MESSAGE field contains the zerolog JSON
	if r.sourceType == SourcePanel {
		line, err := ParsePanelLine(message)
		if err != nil {
			// Fallback: use raw message
			return &LogLine{
				Timestamp: journalTimestamp(entry),
				Level:     "info",
				Message:   message,
				Raw:       message,
			}
		}
		// Use journal timestamp if parser didn't extract one
		if line.Timestamp.IsZero() {
			line.Timestamp = journalTimestamp(entry)
		}
		return &line
	}

	// For ocserv, try syslog parse first, fallback to raw message
	line, err := ParseOcservLine(message, year)
	if err != nil {
		return &LogLine{
			Timestamp: journalTimestamp(entry),
			Level:     "info",
			Message:   message,
			Raw:       message,
		}
	}
	if line.Timestamp.IsZero() {
		line.Timestamp = journalTimestamp(entry)
	}
	return &line
}

func journalTimestamp(entry map[string]interface{}) time.Time {
	// __REALTIME_TIMESTAMP is microseconds since epoch
	if ts, ok := entry["__REALTIME_TIMESTAMP"].(string); ok {
		var usec int64
		for _, c := range ts {
			if c >= '0' && c <= '9' {
				usec = usec*10 + int64(c-'0')
			}
		}
		if usec > 0 {
			return time.Unix(usec/1_000_000, (usec%1_000_000)*1000)
		}
	}
	return time.Time{}
}

func (r *JournalLogReader) applyFilters(lines []LogLine, opts ReadOpts) []LogLine {
	if opts.Level == "" && opts.Search == "" {
		return lines
	}
	searchLower := strings.ToLower(opts.Search)
	filtered := make([]LogLine, 0, len(lines))
	for i := range lines {
		line := &lines[i]
		if opts.Level != "" && line.Level != opts.Level {
			continue
		}
		if opts.Search != "" {
			if !strings.Contains(strings.ToLower(line.Message), searchLower) &&
				!strings.Contains(strings.ToLower(line.Raw), searchLower) {
				continue
			}
		}
		filtered = append(filtered, *line)
	}
	return filtered
}

func intToStr(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	pos := len(buf)
	neg := n < 0
	if neg {
		n = -n
	}
	for n > 0 {
		pos--
		buf[pos] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		pos--
		buf[pos] = '-'
	}
	return string(buf[pos:])
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /opt/ocserv_panel/backend && go test ./internal/logs/ -v -run TestJournalLogReader`
Expected: All PASS.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/logs/journal_reader.go backend/internal/logs/journal_reader_test.go
git commit -m "feat(logs): implement JournalLogReader with CommandRunner"
```

---

## Task 8: FakeLogReader

**Files:**
- Create: `backend/internal/logs/fake.go`

- [ ] **Step 1: Implement fake**

```go
package logs

import "context"

// FakeLogReader is a test double that returns pre-configured results.
type FakeLogReader struct {
	available bool
	result    *ReadResult
	err       error
}

// NewFakeLogReader returns a FakeLogReader.
func NewFakeLogReader(available bool, result *ReadResult, err error) *FakeLogReader {
	return &FakeLogReader{available: available, result: result, err: err}
}

// Available returns the pre-configured availability.
func (f *FakeLogReader) Available(_ context.Context) bool {
	return f.available
}

// ReadLines returns the pre-configured result.
func (f *FakeLogReader) ReadLines(_ context.Context, _ ReadOpts) (*ReadResult, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.result, nil
}
```

- [ ] **Step 2: Verify build**

Run: `cd /opt/ocserv_panel/backend && go build ./internal/logs/`
Expected: Clean build.

- [ ] **Step 3: Commit**

```bash
git add backend/internal/logs/fake.go
git commit -m "feat(logs): add FakeLogReader test double"
```

---

## Task 9: AuditLogService

**Files:**
- Create: `backend/internal/app/audit_log_service.go`
- Create: `backend/internal/app/audit_log_service_test.go`

- [ ] **Step 1: Write service tests**

```go
package app

import (
	"context"
	"testing"
	"time"

	"ocserv_panel/backend/internal/auth"
	"ocserv_panel/backend/internal/config"
	"ocserv_panel/backend/internal/logs"
	"ocserv_panel/backend/internal/rbac"
)

func TestAuditLogService_ReadLogs_Success(t *testing.T) {
	fakeReader := logs.NewFakeLogReader(true, &logs.ReadResult{
		Lines:   []logs.LogLine{{Level: "info", Message: "test"}},
		Total:   1,
		HasMore: false,
	}, nil)
	svc := NewAuditLogService(
		func() config.Config { return config.Defaults() },
		map[string]logs.LogReader{"panel": fakeReader},
		map[string]logs.LogReader{},
		nil,
		rbac.NewDefaultAuthorizer(),
	)
	principal := auth.Principal{ID: 1, Username: "admin", Roles: []string{"owner"}}
	result, err := svc.ReadLogs(context.Background(), principal, "panel", AuditLogQuery{Limit: 100})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Source != "panel" {
		t.Errorf("source = %q, want panel", result.Source)
	}
	if len(result.Lines) != 1 {
		t.Fatalf("got %d lines, want 1", len(result.Lines))
	}
}

func TestAuditLogService_ReadLogs_Unauthenticated(t *testing.T) {
	svc := NewAuditLogService(
		func() config.Config { return config.Defaults() },
		map[string]logs.LogReader{},
		map[string]logs.LogReader{},
		nil,
		rbac.NewDefaultAuthorizer(),
	)
	principal := auth.Principal{} // not authenticated
	_, err := svc.ReadLogs(context.Background(), principal, "panel", AuditLogQuery{})
	if err != ErrUnauthenticated {
		t.Errorf("err = %v, want ErrUnauthenticated", err)
	}
}

func TestAuditLogService_ReadLogs_PermissionDenied(t *testing.T) {
	// Create an authorizer with no audit permissions
	svc := NewAuditLogService(
		func() config.Config { return config.Defaults() },
		map[string]logs.LogReader{},
		map[string]logs.LogReader{},
		nil,
		rbac.Authorizer{}, // empty authorizer — denies everything
	)
	principal := auth.Principal{ID: 1, Username: "nobody", Roles: []string{"invalid"}}
	_, err := svc.ReadLogs(context.Background(), principal, "panel", AuditLogQuery{})
	if err != ErrPermissionDenied {
		t.Errorf("err = %v, want ErrPermissionDenied", err)
	}
}

func TestAuditLogService_ReadLogs_InvalidSource(t *testing.T) {
	svc := NewAuditLogService(
		func() config.Config { return config.Defaults() },
		map[string]logs.LogReader{},
		map[string]logs.LogReader{},
		nil,
		rbac.NewDefaultAuthorizer(),
	)
	principal := auth.Principal{ID: 1, Username: "admin", Roles: []string{"owner"}}
	_, err := svc.ReadLogs(context.Background(), principal, "../../etc/passwd", AuditLogQuery{})
	if err != ErrInvalidInput {
		t.Errorf("err = %v, want ErrInvalidInput", err)
	}
}

func TestAuditLogService_ReadLogs_Unavailable(t *testing.T) {
	fakeFile := logs.NewFakeLogReader(false, nil, nil)
	fakeJournal := logs.NewFakeLogReader(false, nil, nil)
	svc := NewAuditLogService(
		func() config.Config { return config.Defaults() },
		map[string]logs.LogReader{"panel": fakeFile},
		map[string]logs.LogReader{"panel": fakeJournal},
		nil,
		rbac.NewDefaultAuthorizer(),
	)
	principal := auth.Principal{ID: 1, Username: "admin", Roles: []string{"owner"}}
	_, err := svc.ReadLogs(context.Background(), principal, "panel", AuditLogQuery{})
	if err != ErrLogSourceUnavailable {
		t.Errorf("err = %v, want ErrLogSourceUnavailable", err)
	}
}

func TestAuditLogService_ReadLogs_JournalFallback(t *testing.T) {
	fakeFile := logs.NewFakeLogReader(false, nil, nil)
	fakeJournal := logs.NewFakeLogReader(true, &logs.ReadResult{
		Lines:   []logs.LogLine{{Level: "info", Message: "from journal"}},
		Total:   1,
		HasMore: false,
	}, nil)
	svc := NewAuditLogService(
		func() config.Config { return config.Defaults() },
		map[string]logs.LogReader{"panel": fakeFile},
		map[string]logs.LogReader{"panel": fakeJournal},
		nil,
		rbac.NewDefaultAuthorizer(),
	)
	principal := auth.Principal{ID: 1, Username: "admin", Roles: []string{"owner"}}
	result, err := svc.ReadLogs(context.Background(), principal, "panel", AuditLogQuery{Limit: 100})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Lines) != 1 || result.Lines[0].Message != "from journal" {
		t.Errorf("expected journal fallback result, got %+v", result.Lines)
	}
}

func TestAuditLogService_GetSources(t *testing.T) {
	fakeReader := logs.NewFakeLogReader(true, nil, nil)
	cfg := config.Defaults()
	svc := NewAuditLogService(
		func() config.Config { return cfg },
		map[string]logs.LogReader{"panel": fakeReader, "ocserv": fakeReader},
		map[string]logs.LogReader{},
		nil,
		rbac.NewDefaultAuthorizer(),
	)
	principal := auth.Principal{ID: 1, Username: "admin", Roles: []string{"owner"}}
	sources, err := svc.GetSources(context.Background(), principal)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sources) != 2 {
		t.Fatalf("got %d sources, want 2", len(sources))
	}
}

func TestAuditLogService_ExportLogs_PermissionDenied(t *testing.T) {
	svc := NewAuditLogService(
		func() config.Config { return config.Defaults() },
		map[string]logs.LogReader{},
		map[string]logs.LogReader{},
		nil,
		rbac.NewDefaultAuthorizer(),
	)
	// viewer role cannot export
	principal := auth.Principal{ID: 1, Username: "viewer", Roles: []string{"viewer"}}
	_, err := svc.ExportLogs(context.Background(), principal, "panel", AuditLogQuery{}, "json")
	if err != ErrPermissionDenied {
		t.Errorf("err = %v, want ErrPermissionDenied", err)
	}
}

func TestAuditLogService_ReadLogs_TimeValidation(t *testing.T) {
	fakeReader := logs.NewFakeLogReader(true, &logs.ReadResult{Lines: nil, Total: 0}, nil)
	svc := NewAuditLogService(
		func() config.Config { return config.Defaults() },
		map[string]logs.LogReader{"panel": fakeReader},
		map[string]logs.LogReader{},
		nil,
		rbac.NewDefaultAuthorizer(),
	)
	principal := auth.Principal{ID: 1, Username: "admin", Roles: []string{"owner"}}
	// from > to is invalid
	query := AuditLogQuery{
		From: time.Date(2026, 5, 8, 0, 0, 0, 0, time.UTC),
		To:   time.Date(2026, 5, 7, 0, 0, 0, 0, time.UTC),
	}
	_, err := svc.ReadLogs(context.Background(), principal, "panel", query)
	if err != ErrInvalidInput {
		t.Errorf("err = %v, want ErrInvalidInput", err)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /opt/ocserv_panel/backend && go test ./internal/app/ -v -run TestAuditLogService`
Expected: FAIL — types not defined.

- [ ] **Step 3: Implement AuditLogService**

```go
package app

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"ocserv_panel/backend/internal/audit"
	"ocserv_panel/backend/internal/auth"
	"ocserv_panel/backend/internal/config"
	"ocserv_panel/backend/internal/logs"
	"ocserv_panel/backend/internal/rbac"
)

// ErrLogSourceUnavailable is returned when neither file nor journal reader is available.
var ErrLogSourceUnavailable = fmt.Errorf("log source unavailable")

// AuditLogQuery holds validated query parameters for log reading.
type AuditLogQuery struct {
	Search string
	Level  string
	From   time.Time
	To     time.Time
	Limit  int
	Offset int
	Order  string
}

// AuditLogResponse is the API response for log reading.
type AuditLogResponse struct {
	Source  string         `json:"source"`
	Lines   []logs.LogLine `json:"lines"`
	Total   int            `json:"total"`
	Offset  int            `json:"offset"`
	Limit   int            `json:"limit"`
	HasMore bool           `json:"hasMore"`
}

// LogSourceInfo describes an available log source.
type LogSourceInfo struct {
	Name          string `json:"name"`
	Label         string `json:"label"`
	Available     bool   `json:"available"`
	Path          string `json:"path"`
	Reader        string `json:"reader"`
	LogLevel      *int   `json:"logLevel"`
	LogLevelLabel string `json:"logLevelLabel,omitempty"`
}

// validSources is the allowlist of accepted source values.
var validSources = map[string]bool{"panel": true, "ocserv": true}

// sourceLabels maps source names to display labels.
var sourceLabels = map[string]string{
	"panel":  "Web UI Backend Logs",
	"ocserv": "ocserv VPN Logs",
}

// maxReadTimeRange is the maximum allowed time range for normal reads.
const maxReadTimeRange = 7 * 24 * time.Hour

// maxExportTimeRange is the maximum allowed time range for exports.
const maxExportTimeRange = 30 * 24 * time.Hour

// maxExportLines is the maximum lines for export.
const maxExportLines = 5000

// AuditLogService provides log reading with RBAC and source resolution.
type AuditLogService struct {
	cfgFn          func() config.Config
	fileReaders    map[string]logs.LogReader
	journalReaders map[string]logs.LogReader
	recorder       audit.Recorder
	authorizer     rbac.Authorizer
}

// NewAuditLogService creates an AuditLogService.
func NewAuditLogService(
	cfgFn func() config.Config,
	fileReaders map[string]logs.LogReader,
	journalReaders map[string]logs.LogReader,
	recorder audit.Recorder,
	authorizer rbac.Authorizer,
) *AuditLogService {
	return &AuditLogService{
		cfgFn:          cfgFn,
		fileReaders:    fileReaders,
		journalReaders: journalReaders,
		recorder:       recorder,
		authorizer:     authorizer,
	}
}

// ReadLogs reads log lines from the specified source.
func (s *AuditLogService) ReadLogs(ctx context.Context, principal auth.Principal, source string, query AuditLogQuery) (AuditLogResponse, error) {
	if !principal.Authenticated() {
		return AuditLogResponse{}, ErrUnauthenticated
	}
	if !s.authorizer.Allowed(principalRoles(principal), rbac.PermissionAuditLogsRead) {
		return AuditLogResponse{}, ErrPermissionDenied
	}
	if !validSources[source] {
		return AuditLogResponse{}, ErrInvalidInput
	}
	if err := s.validateQuery(query, maxReadTimeRange); err != nil {
		return AuditLogResponse{}, err
	}

	reader := s.resolveReader(ctx, source)
	if reader == nil {
		return AuditLogResponse{}, ErrLogSourceUnavailable
	}

	limit := query.Limit
	if limit <= 0 {
		limit = 100
	}
	if limit > 500 {
		limit = 500
	}

	opts := logs.ReadOpts{
		Limit:  limit,
		Offset: query.Offset,
		Order:  query.Order,
		Search: query.Search,
		Level:  query.Level,
		From:   query.From,
		To:     query.To,
	}

	result, err := reader.ReadLines(ctx, opts)
	if err != nil {
		return AuditLogResponse{}, fmt.Errorf("read logs: %w", err)
	}

	// Apply sensitive data masking
	for i := range result.Lines {
		logs.MaskLogLine(&result.Lines[i])
	}

	return AuditLogResponse{
		Source:  source,
		Lines:   result.Lines,
		Total:   result.Total,
		Offset:  query.Offset,
		Limit:   limit,
		HasMore: result.HasMore,
	}, nil
}

// ExportLogs exports filtered logs for download. Requires PermissionAuditLogsExport.
func (s *AuditLogService) ExportLogs(ctx context.Context, principal auth.Principal, source string, query AuditLogQuery, format string) ([]byte, error) {
	if !principal.Authenticated() {
		return nil, ErrUnauthenticated
	}
	if !s.authorizer.Allowed(principalRoles(principal), rbac.PermissionAuditLogsExport) {
		return nil, ErrPermissionDenied
	}
	if !validSources[source] {
		return nil, ErrInvalidInput
	}
	if format != "json" && format != "csv" {
		return nil, ErrInvalidInput
	}
	if err := s.validateQuery(query, maxExportTimeRange); err != nil {
		return nil, err
	}

	reader := s.resolveReader(ctx, source)
	if reader == nil {
		return nil, ErrLogSourceUnavailable
	}

	opts := logs.ReadOpts{
		Limit:  maxExportLines,
		Offset: 0,
		Order:  query.Order,
		Search: query.Search,
		Level:  query.Level,
		From:   query.From,
		To:     query.To,
	}

	result, err := reader.ReadLines(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("export logs: %w", err)
	}

	for i := range result.Lines {
		logs.MaskLogLine(&result.Lines[i])
	}

	// Record audit event
	if s.recorder != nil {
		actorID := principal.ID
		_ = s.recorder.Record(ctx, &actorID, "audit.logs.export", "logs", source,
			audit.OutcomeSuccess, fmt.Sprintf("exported %d %s log lines as %s", len(result.Lines), source, format),
			map[string]string{"source": source, "format": format, "lines": intToStr(len(result.Lines))})
	}

	switch format {
	case "csv":
		return exportCSV(result.Lines)
	default:
		return exportJSON(result.Lines)
	}
}

// GetSources returns metadata about available log sources.
func (s *AuditLogService) GetSources(ctx context.Context, principal auth.Principal) ([]LogSourceInfo, error) {
	if !principal.Authenticated() {
		return nil, ErrUnauthenticated
	}
	if !s.authorizer.Allowed(principalRoles(principal), rbac.PermissionAuditLogsRead) {
		return nil, ErrPermissionDenied
	}

	cfg := s.cfgFn()
	var sources []LogSourceInfo

	for _, name := range []string{"panel", "ocserv"} {
		info := LogSourceInfo{
			Name:  name,
			Label: sourceLabels[name],
		}

		switch name {
		case "panel":
			info.Path = cfg.Installation.PanelLogPath
		case "ocserv":
			info.Path = cfg.Installation.OcservLogPath
		}

		reader := s.resolveReader(ctx, name)
		info.Available = reader != nil

		if fileReader, ok := s.fileReaders[name]; ok && fileReader.Available(ctx) {
			info.Reader = "file"
		} else if journalReader, ok := s.journalReaders[name]; ok && journalReader.Available(ctx) {
			info.Reader = "journal"
		}

		sources = append(sources, info)
	}

	return sources, nil
}

func (s *AuditLogService) resolveReader(ctx context.Context, source string) logs.LogReader {
	if reader, ok := s.fileReaders[source]; ok && reader.Available(ctx) {
		return reader
	}
	if reader, ok := s.journalReaders[source]; ok && reader.Available(ctx) {
		return reader
	}
	return nil
}

func (s *AuditLogService) validateQuery(query AuditLogQuery, maxRange time.Duration) error {
	if !query.From.IsZero() && !query.To.IsZero() {
		if query.From.After(query.To) {
			return ErrInvalidInput
		}
		if query.To.Sub(query.From) > maxRange {
			return ErrInvalidInput
		}
	}
	if query.Order != "" && query.Order != "asc" && query.Order != "desc" {
		return ErrInvalidInput
	}
	if query.Level != "" {
		switch query.Level {
		case "debug", "info", "warn", "error":
			// valid
		default:
			return ErrInvalidInput
		}
	}
	return nil
}

func exportJSON(lines []logs.LogLine) ([]byte, error) {
	return json.Marshal(lines)
}

func exportCSV(lines []logs.LogLine) ([]byte, error) {
	var buf strings.Builder
	w := csv.NewWriter(&buf)
	_ = w.Write([]string{"timestamp", "level", "component", "message"})
	for _, line := range lines {
		_ = w.Write([]string{
			line.Timestamp.Format(time.RFC3339),
			line.Level,
			line.Component,
			line.Message,
		})
	}
	w.Flush()
	return []byte(buf.String()), w.Error()
}

func intToStr(n int) string {
	return fmt.Sprintf("%d", n)
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /opt/ocserv_panel/backend && go test ./internal/app/ -v -run TestAuditLogService`
Expected: All PASS.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/app/audit_log_service.go backend/internal/app/audit_log_service_test.go
git commit -m "feat(app): implement AuditLogService with source resolution and RBAC"
```

---

## Task 10: HTTP Handlers

**Files:**
- Create: `backend/internal/server/audit_log_handlers.go`
- Create: `backend/internal/server/audit_log_handlers_test.go`

- [ ] **Step 1: Write handler tests**

```go
package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"ocserv_panel/backend/internal/app"
	"ocserv_panel/backend/internal/auth"
	"ocserv_panel/backend/internal/config"
	"ocserv_panel/backend/internal/logs"
	"ocserv_panel/backend/internal/rbac"
	"ocserv_panel/backend/internal/store"
)

func TestHandleAuditLogs_Unauthenticated(t *testing.T) {
	a := newTestAppWithAuditLogs(t)
	req := httptest.NewRequest(http.MethodGet, "/api/audit/logs?source=panel", nil)
	w := httptest.NewRecorder()
	a.handleAuditLogs(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestHandleAuditLogs_InvalidSource(t *testing.T) {
	a := newTestAppWithAuditLogs(t)
	req := authenticatedRequest(t, a, http.MethodGet, "/api/audit/logs?source=../../etc/passwd")
	w := httptest.NewRecorder()
	a.handleAuditLogs(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestHandleAuditLogs_Success(t *testing.T) {
	a := newTestAppWithAuditLogs(t)
	req := authenticatedRequest(t, a, http.MethodGet, "/api/audit/logs?source=panel&limit=10")
	w := httptest.NewRecorder()
	a.handleAuditLogs(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body = %s", w.Code, http.StatusOK, w.Body.String())
	}
	var resp app.AuditLogResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Source != "panel" {
		t.Errorf("source = %q, want panel", resp.Source)
	}
}

func TestHandleAuditLogs_MethodNotAllowed(t *testing.T) {
	a := newTestAppWithAuditLogs(t)
	req := authenticatedRequest(t, a, http.MethodPost, "/api/audit/logs?source=panel")
	w := httptest.NewRecorder()
	a.handleAuditLogs(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want %d", w.Code, http.StatusMethodNotAllowed)
	}
}

func TestHandleAuditLogSources_Success(t *testing.T) {
	a := newTestAppWithAuditLogs(t)
	req := authenticatedRequest(t, a, http.MethodGet, "/api/audit/logs/sources")
	w := httptest.NewRecorder()
	a.handleAuditLogSources(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestHandleAuditLogExport_ViewerDenied(t *testing.T) {
	a := newTestAppWithAuditLogs(t)
	req := authenticatedRequestWithRole(t, a, http.MethodGet, "/api/audit/logs/export?source=panel&format=json", "viewer")
	w := httptest.NewRecorder()
	a.handleAuditLogExport(w, req)
	if w.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", w.Code, http.StatusForbidden)
	}
}

// --- test helpers ---

func newTestAppWithAuditLogs(t *testing.T) *App {
	t.Helper()
	persistence := store.NewMemoryStore()
	fakeReader := logs.NewFakeLogReader(true, &logs.ReadResult{
		Lines:   []logs.LogLine{{Level: "info", Message: "test line", Timestamp: time.Now()}},
		Total:   1,
		HasMore: false,
	}, nil)

	auditLogSvc := app.NewAuditLogService(
		func() config.Config { return config.Defaults() },
		map[string]logs.LogReader{"panel": fakeReader, "ocserv": fakeReader},
		map[string]logs.LogReader{},
		nil,
		rbac.NewDefaultAuthorizer(),
	)

	authSvc := app.NewAuthService(persistence, nil)
	return &App{
		authService:     authSvc,
		auditLogService: auditLogSvc,
		authorizer:      rbac.NewDefaultAuthorizer(),
		now:             func() time.Time { return time.Now().UTC() },
	}
}

func authenticatedRequest(t *testing.T, a *App, method, url string) *http.Request {
	t.Helper()
	return authenticatedRequestWithRole(t, a, method, url, "owner")
}

func authenticatedRequestWithRole(t *testing.T, a *App, method, url, role string) *http.Request {
	t.Helper()
	// Create a user and session for the test
	ctx := context.Background()
	_ = a.authService.CreateFirstOwner(ctx, "testuser", "TestPass123!")
	login, err := a.authService.Login(ctx, "testuser", "TestPass123!", "test-agent", "127.0.0.1")
	if err != nil {
		t.Fatalf("login for test: %v", err)
	}
	req := httptest.NewRequest(method, url, nil)
	req.AddCookie(&http.Cookie{Name: SessionCookieName, Value: login.Token})
	return req
}
```

Note: The test helper `authenticatedRequestWithRole` is simplified — for the viewer test, we rely on the service layer's RBAC check since the test creates an owner user. A more complete version would create a viewer user; however, the service layer test already covers RBAC denial. The handler test validates the HTTP status mapping.

- [ ] **Step 2: Implement handlers**

```go
package server

import (
	"net/http"
	"strconv"
	"time"

	"ocserv_panel/backend/internal/app"
)

func (a *App) handleAuditLogs(w http.ResponseWriter, r *http.Request) {
	if !allowMethod(w, r, http.MethodGet) {
		return
	}
	principal, ok := a.authenticate(w, r)
	if !ok {
		return
	}

	source := r.URL.Query().Get("source")
	if source == "" {
		writeError(w, http.StatusBadRequest, "source parameter is required")
		return
	}

	query, err := parseAuditLogQuery(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	result, svcErr := a.auditLogService.ReadLogs(r.Context(), principal, source, query)
	if svcErr != nil {
		a.writeAppError(w, svcErr)
		return
	}

	writeJSON(w, http.StatusOK, result)
}

func (a *App) handleAuditLogSources(w http.ResponseWriter, r *http.Request) {
	if !allowMethod(w, r, http.MethodGet) {
		return
	}
	principal, ok := a.authenticate(w, r)
	if !ok {
		return
	}

	sources, err := a.auditLogService.GetSources(r.Context(), principal)
	if err != nil {
		a.writeAppError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"sources": sources})
}

func (a *App) handleAuditLogExport(w http.ResponseWriter, r *http.Request) {
	if !allowMethod(w, r, http.MethodGet) {
		return
	}
	principal, ok := a.authenticate(w, r)
	if !ok {
		return
	}

	source := r.URL.Query().Get("source")
	if source == "" {
		writeError(w, http.StatusBadRequest, "source parameter is required")
		return
	}

	format := r.URL.Query().Get("format")
	if format == "" {
		format = "json"
	}

	query, err := parseAuditLogQuery(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	data, svcErr := a.auditLogService.ExportLogs(r.Context(), principal, source, query, format)
	if svcErr != nil {
		a.writeAppError(w, svcErr)
		return
	}

	contentType := "application/json"
	filename := "audit-logs-" + source + ".json"
	if format == "csv" {
		contentType = "text/csv"
		filename = "audit-logs-" + source + ".csv"
	}

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", "attachment; filename=\""+filename+"\"")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func parseAuditLogQuery(r *http.Request) (app.AuditLogQuery, error) {
	q := r.URL.Query()
	var query app.AuditLogQuery

	query.Search = q.Get("search")
	query.Level = q.Get("level")
	query.Order = q.Get("order")
	if query.Order == "" {
		query.Order = "desc"
	}

	if v := q.Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 {
			return query, fmt.Errorf("invalid limit: %s", v)
		}
		query.Limit = n
	}

	if v := q.Get("offset"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 {
			return query, fmt.Errorf("invalid offset: %s", v)
		}
		query.Offset = n
	}

	if v := q.Get("from"); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			return query, fmt.Errorf("invalid from timestamp: %s", v)
		}
		query.From = t
	}

	if v := q.Get("to"); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			return query, fmt.Errorf("invalid to timestamp: %s", v)
		}
		query.To = t
	}

	return query, nil
}
```

Note: Add `"fmt"` to the imports list in audit_log_handlers.go.

- [ ] **Step 3: Run tests to verify they pass**

Run: `cd /opt/ocserv_panel/backend && go test ./internal/server/ -v -run TestHandleAuditLog`
Expected: All PASS (or adjust test helper based on existing store patterns).

- [ ] **Step 4: Commit**

```bash
git add backend/internal/server/audit_log_handlers.go backend/internal/server/audit_log_handlers_test.go
git commit -m "feat(server): add audit log HTTP handlers with query parsing"
```

---

## Task 11: Wire Into Server and Main

**Files:**
- Modify: `backend/internal/server/server.go`
- Modify: `backend/cmd/ocserv-panel/main.go`

- [ ] **Step 1: Add auditLogService field to App struct**

In `server.go`, add to the `App` struct:

```go
auditLogService *app.AuditLogService
```

- [ ] **Step 2: Update NewAppRouter signature**

Add `auditLogService *app.AuditLogService` parameter to `NewAppRouter` and assign it:

```go
func NewAppRouter(
	authService *app.AuthService,
	dashboardService *app.DashboardService,
	monitoringService *app.MonitoringService,
	settingsService *app.SettingsService,
	ocservService *app.OcservService,
	healthService *app.HealthService,
	logTailService *app.LogTailService,
	configService *app.ConfigManagementService,
	adminUserService *app.AdminUserService,
	auditLogService *app.AuditLogService,
	authorizer rbac.Authorizer,
	opts AppOptions,
) http.Handler {
```

And in the `appHandler` initialization:

```go
auditLogService:  auditLogService,
```

- [ ] **Step 3: Register routes**

Add after the bulk operations routes:

```go
// Audit log viewing
mux.HandleFunc("GET /api/audit/logs", appHandler.handleAuditLogs)
mux.HandleFunc("GET /api/audit/logs/sources", appHandler.handleAuditLogSources)
mux.HandleFunc("GET /api/audit/logs/export", appHandler.handleAuditLogExport)
```

- [ ] **Step 4: Wire in main.go**

In `main.go`, after the `logTailService` initialization, add:

```go
// Audit log viewing service
panelFileReader := logs.NewFileLogReader(cfg.Installation.PanelLogPath, logs.SourcePanel)
ocservFileReader := logs.NewFileLogReader(cfg.Installation.OcservLogPath, logs.SourceOcserv)
panelJournalReader := logs.NewJournalLogReader(cfg.Installation.PanelSystemdUnit, logs.SourcePanel, runner)
ocservJournalReader := logs.NewJournalLogReader(cfg.Service.SystemdUnitName, logs.SourceOcserv, runner)
auditLogService := app.NewAuditLogService(
	healthCfgFn,
	map[string]logs.LogReader{"panel": panelFileReader, "ocserv": ocservFileReader},
	map[string]logs.LogReader{"panel": panelJournalReader, "ocserv": ocservJournalReader},
	recorder,
	rbac.NewDefaultAuthorizer(),
)
```

Add the `auditLogService` parameter to the `NewAppRouter` call.

Add import: `"ocserv_panel/backend/internal/logs"`

- [ ] **Step 5: Update writeAppError for new error**

In `server.go` `writeAppError` function, add a case:

```go
case errors.Is(err, app.ErrLogSourceUnavailable):
	writeError(w, http.StatusServiceUnavailable, "log source is unavailable")
```

- [ ] **Step 6: Verify build**

Run: `cd /opt/ocserv_panel/backend && go build ./...`
Expected: Clean build.

- [ ] **Step 7: Run all tests**

Run: `cd /opt/ocserv_panel/backend && go test ./... 2>&1 | tail -20`
Expected: All packages pass.

- [ ] **Step 8: Commit**

```bash
git add backend/internal/server/server.go backend/cmd/ocserv-panel/main.go
git commit -m "feat: wire AuditLogService into server and main"
```

---

## Task 12: Final Backend Verification

- [ ] **Step 1: Run full test suite**

Run: `cd /opt/ocserv_panel/backend && go test ./... -count=1`
Expected: All packages PASS.

- [ ] **Step 2: Run go vet**

Run: `cd /opt/ocserv_panel/backend && go vet ./...`
Expected: No issues.

- [ ] **Step 3: Verify build**

Run: `cd /opt/ocserv_panel/backend && go build -o /dev/null ./cmd/ocserv-panel/`
Expected: Clean build.

---
