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
