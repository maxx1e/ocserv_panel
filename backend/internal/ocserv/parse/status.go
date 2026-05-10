package parse

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

// StatusOutput parses the text output of `occtl show status` into a ServerStatus struct.
// It returns an error if the input is empty. Missing fields are left at their zero values.
func StatusOutput(text string) (*ServerStatus, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil, fmt.Errorf("empty status output")
	}

	status := &ServerStatus{}
	lines := strings.Split(text, "\n")

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "Note:") || strings.HasPrefix(trimmed, "General info:") || strings.HasPrefix(trimmed, "Current stats period:") {
			continue
		}
		// Skip continuation lines (lines that don't contain a colon as a key-value separator)
		colonIdx := strings.Index(trimmed, ":")
		if colonIdx < 0 {
			continue
		}

		key := strings.TrimSpace(trimmed[:colonIdx])
		value := strings.TrimSpace(trimmed[colonIdx+1:])

		switch key {
		case "Status":
			status.Status = value
		case "Server PID":
			status.ServerPID = IntSafe(value)
		case "Sec-mod PID":
			status.SecModPID = IntSafe(value)
		case "Sec-mod instance count":
			status.SecModInstances = IntSafe(value)
		case "Up since":
			status.UpSince = OcctlTimestamp(value)
		case "Active sessions":
			status.ActiveSessions = IntSafe(value)
		case "Total sessions":
			status.TotalSessions = IntSafe(value)
		case "Total authentication failures":
			status.TotalAuthFailures = IntSafe(value)
		case "IPs in ban list":
			status.BannedIPs = IntSafe(value)
		case "Last stats reset":
			status.LastStatsReset = OcctlTimestamp(value)
		case "Sessions handled":
			status.SessionsHandled = IntSafe(value)
		case "Timed out sessions":
			status.TimedOutSessions = IntSafe(value)
		case "Timed out (idle) sessions":
			status.IdleTimedOutSessions = IntSafe(value)
		case "Closed due to error sessions":
			status.ErrorClosedSessions = IntSafe(value)
		case "Authentication failures":
			status.AuthFailures = IntSafe(value)
		case "Average auth time":
			status.AvgAuthTime = OcctlDuration(value)
		case "Max auth time":
			status.MaxAuthTime = OcctlDuration(value)
		case "Average session time":
			status.AvgSessionTime = OcctlDuration(value)
		case "Max session time":
			status.MaxSessionTime = OcctlDuration(value)
		case "Min MTU":
			status.MinMTU = IntSafe(value)
		case "Max MTU":
			status.MaxMTU = IntSafe(value)
		case "RX":
			status.RxBytes = Bytes(value)
		case "TX":
			status.TxBytes = Bytes(value)
		}
	}

	return status, nil
}

// Bytes parses a human-readable byte string like "91.5 MB", "2.6 GB", "1024 KB", or "0 bytes"
// into the corresponding number of bytes.
func Bytes(s string) int64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}

	parts := strings.Fields(s)
	if len(parts) < 2 {
		// Try parsing as raw number
		v, _ := strconv.ParseInt(s, 10, 64)
		return v
	}

	numStr := parts[0]
	unit := strings.ToLower(parts[1])

	num, err := strconv.ParseFloat(numStr, 64)
	if err != nil {
		return 0
	}

	var multiplier float64
	switch unit {
	case "bytes", "byte":
		multiplier = 1
	case "kb":
		multiplier = 1024
	case "mb":
		multiplier = 1024 * 1024
	case "gb":
		multiplier = 1024 * 1024 * 1024
	case "tb":
		multiplier = 1024 * 1024 * 1024 * 1024
	default:
		return 0
	}

	return int64(math.Round(num * multiplier))
}

// OcctlDuration parses occtl duration strings such as "4s", "10s", "3h:14m", "2days", "1h:30m:45s", or "0s".
func OcctlDuration(s string) time.Duration {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}

	// Handle "Xdays" format
	if strings.HasSuffix(s, "days") {
		numStr := strings.TrimSuffix(s, "days")
		days, err := strconv.Atoi(strings.TrimSpace(numStr))
		if err != nil {
			return 0
		}
		return time.Duration(days) * 24 * time.Hour
	}
	if strings.HasSuffix(s, "day") {
		numStr := strings.TrimSuffix(s, "day")
		days, err := strconv.Atoi(strings.TrimSpace(numStr))
		if err != nil {
			return 0
		}
		return time.Duration(days) * 24 * time.Hour
	}

	// Handle colon-separated format: "3h:14m", "1h:30m:45s"
	if strings.Contains(s, ":") {
		var total time.Duration
		parts := strings.Split(s, ":")
		for _, part := range parts {
			total += parseSingleDurationUnit(part)
		}
		return total
	}

	// Handle simple format: "4s", "10s"
	return parseSingleDurationUnit(s)
}

// parseSingleDurationUnit parses a single duration component like "3h", "14m", "45s".
func parseSingleDurationUnit(s string) time.Duration {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}

	// Find where the numeric part ends
	numEnd := 0
	for numEnd < len(s) && (s[numEnd] >= '0' && s[numEnd] <= '9') {
		numEnd++
	}

	if numEnd == 0 {
		return 0
	}

	numStr := s[:numEnd]
	unit := s[numEnd:]

	num, err := strconv.Atoi(numStr)
	if err != nil {
		return 0
	}

	switch unit {
	case "s":
		return time.Duration(num) * time.Second
	case "m":
		return time.Duration(num) * time.Minute
	case "h":
		return time.Duration(num) * time.Hour
	default:
		return 0
	}
}

// StatusJSON parses the JSON output of `occtl -j show status` into a ServerStatus struct.
// The JSON uses human-readable keys with spaces (e.g. "Server PID", "Active sessions") so we
// decode into a map[string]interface{} and extract fields by name.
func StatusJSON(data []byte) (*ServerStatus, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("empty status JSON")
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("decode status JSON: %w", err)
	}

	status := &ServerStatus{}

	// String fields
	if v, ok := raw["Status"].(string); ok {
		status.Status = v
	}

	// Integer fields
	status.ServerPID = jsonInt(raw, "Server PID")
	status.SecModPID = jsonInt(raw, "Sec-mod PID")
	status.SecModInstances = jsonInt(raw, "Sec-mod instance count")
	status.ActiveSessions = jsonInt(raw, "Active sessions")
	status.TotalSessions = jsonInt(raw, "Total sessions")
	status.TotalAuthFailures = jsonInt(raw, "Total authentication failures")
	status.BannedIPs = jsonInt(raw, "IPs in ban list")
	status.SessionsHandled = jsonInt(raw, "Sessions handled")
	status.TimedOutSessions = jsonInt(raw, "Timed out sessions")
	status.IdleTimedOutSessions = jsonInt(raw, "Timed out (idle) sessions")
	status.ErrorClosedSessions = jsonInt(raw, "Closed due to error sessions")
	status.AuthFailures = jsonInt(raw, "Authentication failures")

	// Timestamps from raw unix epoch fields
	if v, ok := raw["raw_up_since"]; ok {
		status.UpSince = jsonUnixTime(v)
	}
	if v, ok := raw["raw_last_stats_reset"]; ok {
		status.LastStatsReset = jsonUnixTime(v)
	}

	// Durations from raw seconds fields
	status.AvgAuthTime = jsonDurationSeconds(raw, "raw_avg_auth_time")
	status.MaxAuthTime = jsonDurationSeconds(raw, "raw_max_auth_time")
	status.AvgSessionTime = jsonDurationSeconds(raw, "raw_avg_session_time")
	status.MaxSessionTime = jsonDurationSeconds(raw, "raw_max_session_time")

	// Byte counts from raw fields
	status.RxBytes = jsonInt64(raw, "raw_rx")
	status.TxBytes = jsonInt64(raw, "raw_tx")

	return status, nil
}

// jsonInt extracts an integer value from a map. JSON numbers are float64 by default.
func jsonInt(m map[string]interface{}, key string) int {
	v, ok := m[key]
	if !ok {
		return 0
	}
	switch n := v.(type) {
	case float64:
		return int(n)
	case json.Number:
		i, _ := n.Int64()
		return int(i)
	default:
		return 0
	}
}

// jsonInt64 extracts an int64 value from a map. JSON numbers are float64 by default.
func jsonInt64(m map[string]interface{}, key string) int64 {
	v, ok := m[key]
	if !ok {
		return 0
	}
	switch n := v.(type) {
	case float64:
		return int64(n)
	case json.Number:
		i, _ := n.Int64()
		return i
	default:
		return 0
	}
}

// jsonUnixTime converts a JSON number (unix timestamp) to time.Time.
func jsonUnixTime(v interface{}) time.Time {
	switch n := v.(type) {
	case float64:
		if n == 0 {
			return time.Time{}
		}
		return time.Unix(int64(n), 0)
	case json.Number:
		i, _ := n.Int64()
		if i == 0 {
			return time.Time{}
		}
		return time.Unix(i, 0)
	default:
		return time.Time{}
	}
}

// jsonDurationSeconds extracts a duration (in seconds) from a map's raw_* field.
func jsonDurationSeconds(m map[string]interface{}, key string) time.Duration {
	v, ok := m[key]
	if !ok {
		return 0
	}
	switch n := v.(type) {
	case float64:
		return time.Duration(int64(n)) * time.Second
	case json.Number:
		i, _ := n.Int64()
		return time.Duration(i) * time.Second
	default:
		return 0
	}
}
