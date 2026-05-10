package parse

import (
	"strconv"
	"strings"
	"time"
)

// DurationString parses duration strings produced by occtl such as:
// "1h:27m", "3days", "45m", "30s", "2h:00m", "1h:30m:45s".
// Returns 0 for empty or unparseable strings.
func DurationString(s string) time.Duration {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}

	// Handle "Ndays" format.
	if strings.HasSuffix(s, "days") {
		numStr := strings.TrimSuffix(s, "days")
		if n, err := strconv.Atoi(numStr); err == nil {
			return time.Duration(n) * 24 * time.Hour
		}
		return 0
	}
	// Handle "Nday" format (singular).
	if strings.HasSuffix(s, "day") {
		numStr := strings.TrimSuffix(s, "day")
		if n, err := strconv.Atoi(numStr); err == nil {
			return time.Duration(n) * 24 * time.Hour
		}
		return 0
	}

	// Split by ":" for segments like "1h:27m" or "1h:30m:45s".
	segments := strings.Split(s, ":")
	var total time.Duration

	for _, seg := range segments {
		seg = strings.TrimSpace(seg)
		if seg == "" {
			continue
		}
		d := parseSegment(seg)
		if d < 0 {
			return 0
		}
		total += d
	}
	return total
}

// parseSegment parses a single segment like "1h", "27m", "45s", or "00m".
// Returns -1 if the segment cannot be parsed.
func parseSegment(seg string) time.Duration {
	if len(seg) == 0 {
		return -1
	}

	suffix := seg[len(seg)-1]
	numStr := seg[:len(seg)-1]

	switch suffix {
	case 'h':
		if n, err := strconv.Atoi(numStr); err == nil {
			return time.Duration(n) * time.Hour
		}
	case 'm':
		if n, err := strconv.Atoi(numStr); err == nil {
			return time.Duration(n) * time.Minute
		}
	case 's':
		if n, err := strconv.Atoi(numStr); err == nil {
			return time.Duration(n) * time.Second
		}
	default:
		// Try to parse as plain number (assume minutes for unlabeled segments).
		return -1
	}
	return -1
}

// ExtractValue extracts the value after a "Key:" prefix, trimming whitespace.
// If the line contains multiple key-value pairs separated by multiple spaces,
// only the first value (up to 3+ spaces or end of string) is returned.
func ExtractValue(line, key string) string {
	idx := strings.Index(line, key)
	if idx < 0 {
		return ""
	}
	rest := strings.TrimSpace(line[idx+len(key):])

	// For multi-pair lines, the next key is separated by 2+ spaces.
	// Find the boundary: look for 2+ consecutive spaces.
	if sepIdx := FindMultiSpaceSeparator(rest); sepIdx >= 0 {
		rest = rest[:sepIdx]
	}
	return strings.TrimSpace(rest)
}

// FindMultiSpaceSeparator finds the index of 2+ consecutive spaces in s that
// are followed by a non-space character (indicating a new key-value pair).
func FindMultiSpaceSeparator(s string) int {
	for i := 0; i < len(s)-1; i++ {
		if s[i] == ' ' && s[i+1] == ' ' {
			// Find end of space run.
			j := i + 2
			for j < len(s) && s[j] == ' ' {
				j++
			}
			// Only treat as separator if followed by more content.
			if j < len(s) {
				return i
			}
		}
	}
	return -1
}

// ByteValue parses the raw byte count from a string like "4359893 (4.4 MB)".
// Only the integer before the first space or parenthesis is used.
func ByteValue(s string) int64 {
	s = strings.TrimSpace(s)
	// Take only the numeric portion before any space or paren.
	end := strings.IndexAny(s, " (")
	if end >= 0 {
		s = s[:end]
	}
	val, _ := strconv.ParseInt(s, 10, 64)
	return val
}

// IntSafe parses an integer string, returning 0 on failure.
func IntSafe(s string) int {
	s = strings.TrimSpace(s)
	val, _ := strconv.Atoi(s)
	return val
}

// ConnectedAt extracts a time from "Connected at: 2026-05-07 11:58 ( 1h:28m)".
// The relative duration in parentheses is ignored.
func ConnectedAt(line string) time.Time {
	const prefix = "Connected at:"
	idx := strings.Index(line, prefix)
	if idx < 0 {
		return time.Time{}
	}
	rest := strings.TrimSpace(line[idx+len(prefix):])

	// Remove the relative-time portion in parentheses.
	if parenIdx := strings.Index(rest, "("); parenIdx >= 0 {
		rest = strings.TrimSpace(rest[:parenIdx])
	}

	t, _ := time.Parse("2006-01-02 15:04", rest)
	return t
}

// RebuildTail returns the substring of line after the first n whitespace-delimited fields.
func RebuildTail(line string, fields []string, n int) string {
	pos := 0
	for i := 0; i < n && i < len(fields); i++ {
		idx := strings.Index(line[pos:], fields[i])
		if idx < 0 {
			return ""
		}
		pos += idx + len(fields[i])
	}
	return line[pos:]
}

// ExtractBandwidthValue extracts bandwidth string after a "RX:" or "TX:" prefix.
// Returns the full bandwidth string including units (e.g., "820 bytes/s", "68.1 kB/s").
func ExtractBandwidthValue(line, key string) string {
	idx := strings.Index(line, key)
	if idx < 0 {
		return ""
	}
	rest := strings.TrimSpace(line[idx+len(key):])

	// Bandwidth values end with "/s" — find the unit suffix.
	// Format: "<number> <unit>/s"
	endIdx := strings.Index(rest, "/s")
	if endIdx >= 0 {
		return strings.TrimSpace(rest[:endIdx+2])
	}
	// Fallback: take up to the next multi-space separator.
	if sepIdx := FindMultiSpaceSeparator(rest); sepIdx >= 0 {
		return strings.TrimSpace(rest[:sepIdx])
	}
	return strings.TrimSpace(rest)
}

// OcctlTimestamp parses an occtl timestamp like "2026-05-04 07:29 ( 3days)" by extracting
// just the "2026-05-04 07:29" portion and parsing it with time.Parse.
func OcctlTimestamp(s string) time.Time {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}
	}

	// Strip any trailing relative time in parentheses, e.g. " ( 3days)"
	if idx := strings.Index(s, "("); idx >= 0 {
		s = strings.TrimSpace(s[:idx])
	}

	t, err := time.Parse("2006-01-02 15:04", s)
	if err != nil {
		return time.Time{}
	}
	return t
}

// Timestamp attempts to parse a timestamp string using RFC3339 or RFC3339Nano.
// Returns the zero time if the string is empty or fails to parse.
func Timestamp(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
		return t
	}
	t, _ := time.Parse(time.RFC3339, s)
	return t
}
