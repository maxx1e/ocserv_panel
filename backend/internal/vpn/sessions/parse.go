package sessions

import (
	"encoding/json"
	"fmt"
	"math"
	"net"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// ---------------------------------------------------------------------------
// helpers.go
// ---------------------------------------------------------------------------

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

// ---------------------------------------------------------------------------
// sanitize.go
// ---------------------------------------------------------------------------

// trailingCommaRe matches a comma followed by optional whitespace followed by
// a closing brace or bracket. This pattern appears in occtl -j output which
// produces non-standard JSON with trailing commas after the last field in objects.
var trailingCommaRe = regexp.MustCompile(`,\s*([}\]])`)

// sanitizeOcctlJSON removes trailing commas before } or ] in occtl JSON output.
// Go's encoding/json rejects trailing commas ("invalid character '}' looking for
// beginning of object key string"), so we must strip them before unmarshaling.
func sanitizeOcctlJSON(data []byte) []byte {
	return trailingCommaRe.ReplaceAll(data, []byte("$1"))
}

// ---------------------------------------------------------------------------
// cidr.go
// ---------------------------------------------------------------------------

// IsNetmaskNotation checks if a value is in "ip/dotted-quad-mask" format (e.g., "10.0.0.0/255.255.255.0").
// Returns the ip and mask parts if detected. Returns false for prefix notation like "10.0.0.0/24".
func IsNetmaskNotation(value string) (ip, mask string, ok bool) {
	parts := strings.SplitN(value, "/", 2)
	if len(parts) != 2 {
		return "", "", false
	}
	ip = parts[0]
	mask = parts[1]
	// A dotted-quad mask has exactly 3 dots. A prefix integer has none.
	if strings.Count(mask, ".") != 3 {
		return "", "", false
	}
	// Both parts must parse as valid IPs
	if net.ParseIP(ip) == nil || net.ParseIP(mask) == nil {
		return "", "", false
	}
	return ip, mask, true
}

// NetmaskToCIDR converts an IP and dotted-quad mask to CIDR notation.
// Returns ("", false) if the mask is non-contiguous (invalid).
func NetmaskToCIDR(ip, mask string) (string, bool) {
	maskIP := net.ParseIP(mask)
	if maskIP == nil {
		return "", false
	}
	maskBytes := maskIP.To4()
	if maskBytes == nil {
		return "", false
	}
	ipMask := net.IPMask(maskBytes)
	ones, bits := ipMask.Size()
	// Size() returns (0, 0) for non-contiguous masks
	if bits == 0 {
		return "", false
	}
	return ip + "/" + strconv.Itoa(ones), true
}

// ---------------------------------------------------------------------------
// sessions.go
// ---------------------------------------------------------------------------

// occtlSessionJSON is the intermediate struct that captures the JSON field names
// produced by `occtl -j -n show sessions all`. Fields use the exact names from
// the actual occtl JSON output (some have spaces, some use snake_case).
type occtlSessionJSON struct {
	Session     string `json:"Session"`
	FullSession string `json:"Full session"`
	Created     string `json:"Created"`
	State       string `json:"State"`
	Username    string `json:"Username"`
	Groupname   string `json:"Groupname"`
	Vhost       string `json:"vhost"`
	UserAgent   string `json:"User-Agent"`
	RemoteIP    string `json:"Remote IP"`
	Location    string `json:"Location"`
	SessionOpen int    `json:"session_is_open"`
	TLSAuthOK   int    `json:"tls_auth_ok"`
	InUse       int    `json:"in_use"`
}

// SessionsJSON parses JSON array output from `occtl -j -n show sessions all`.
func SessionsJSON(data []byte) ([]Session, error) {
	var raw []occtlSessionJSON
	if err := json.Unmarshal(sanitizeOcctlJSON(data), &raw); err != nil {
		return nil, fmt.Errorf("parse sessions json: %w", err)
	}

	sessions := make([]Session, 0, len(raw))
	for _, r := range raw {
		s := Session{
			ID:            r.Session,
			FullSession:   r.FullSession,
			Username:      r.Username,
			RemoteAddress: r.RemoteIP,
			UserAgent:     r.UserAgent,
			State:         r.State,
			Groupname:     r.Groupname,
			Vhost:         r.Vhost,
			Location:      r.Location,
			SessionIsOpen: r.SessionOpen != 0,
			InUse:         r.InUse != 0,
		}

		// ConnectedAt: try parsing "Created" as a timestamp (e.g. "2026-05-09 12:24").
		// Fall back to treating it as a duration string (e.g. "2days", "1h:30m") for
		// older occtl versions.
		if r.Created != "" {
			if t, err := time.Parse("2006-01-02 15:04", r.Created); err == nil {
				s.ConnectedAt = t
			} else if d := DurationString(r.Created); d > 0 {
				s.ConnectedAt = time.Now().Add(-d)
			}
		}

		sessions = append(sessions, s)
	}
	return sessions, nil
}

// SessionsText parses the tabular text output from `occtl show sessions all`
// as a fallback when JSON parsing fails. The text format columns are:
//
//	session     user    vhost             ip               user agent  created   status
//	8Oh70G   maxxie  default  94.204.188.26 AnyConnect Android 5.1.1    2days authenticated
func SessionsText(text string) ([]Session, error) {
	lines := strings.Split(text, "\n")
	var sessions []Session

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		// Skip header line.
		lower := strings.ToLower(trimmed)
		if strings.Contains(lower, "session") && strings.Contains(lower, "user") && strings.Contains(lower, "status") {
			continue
		}

		// The text format is whitespace-delimited but "user agent" can contain spaces.
		// Strategy: extract the known fields from the beginning, then handle the
		// variable-width user agent + created + status from the tail.
		fields := strings.Fields(trimmed)
		if len(fields) < 6 {
			continue
		}

		sessionID := fields[0]
		username := fields[1]
		// vhost is field[2]
		remoteIP := fields[3]

		// The status is the last field (e.g., "authenticated", "connected").
		// The created/since is second to last (e.g., "2days", "1h:30m").
		// Everything between remoteIP and created is the user agent.
		status := fields[len(fields)-1]
		created := fields[len(fields)-2]

		// User agent is fields[4] through fields[len(fields)-3].
		var userAgent string
		if len(fields) > 6 {
			userAgent = strings.Join(fields[4:len(fields)-2], " ")
		}

		s := Session{
			ID:            sessionID,
			Username:      username,
			RemoteAddress: remoteIP,
			UserAgent:     userAgent,
			State:         status,
		}

		if d := DurationString(created); d > 0 {
			s.ConnectedAt = time.Now().Add(-d)
		}

		sessions = append(sessions, s)
	}
	return sessions, nil
}

// occtlSessionDetailJSON is the intermediate struct for JSON output from
// `occtl -j -n show session <id>`. The output is an array (even for a single
// session). Fields match the actual occtl JSON keys.
type occtlSessionDetailJSON struct {
	Session     string `json:"Session"`
	FullSession string `json:"Full session"`
	Created     string `json:"Created"`
	State       string `json:"State"`
	Username    string `json:"Username"`
	Groupname   string `json:"Groupname"`
	Vhost       string `json:"vhost"`
	UserAgent   string `json:"User-Agent"`
	RemoteIP    string `json:"Remote IP"`
	Location    string `json:"Location"`
	SessionOpen int    `json:"session_is_open"`
	TLSAuthOK   int    `json:"tls_auth_ok"`
	InUse       int    `json:"in_use"`
	// Extended fields that may appear when the session is fully connected.
	Device         string          `json:"Device"`
	MTU            string          `json:"MTU"`
	IPv4           string          `json:"IPv4"`
	IPv6           string          `json:"IPv6"`
	RX             string          `json:"RX"`
	TX             string          `json:"TX"`
	RawRx          int64           `json:"raw_rx"`
	RawTx          int64           `json:"raw_tx"`
	DNS            []string        `json:"DNS"`
	Routes         json.RawMessage `json:"Routes"`
	ConnectedAt    string          `json:"Connected at"`
	RawConnectedAt int64           `json:"raw_connected_at"`
}

// SessionDetailJSON parses JSON output from `occtl -j -n show session <id>`.
// The output is an array (even for a single session). Tries array first, then single object.
func SessionDetailJSON(data []byte) (*SessionDetail, error) {
	// Sanitize trailing commas from occtl JSON output.
	clean := sanitizeOcctlJSON(data)

	// Try array format first (actual occtl behavior).
	var rawArr []occtlSessionDetailJSON
	if err := json.Unmarshal(clean, &rawArr); err == nil && len(rawArr) > 0 {
		return sessionDetailFromRaw(&rawArr[0]), nil
	}

	// Try single object format (for test compatibility).
	var raw occtlSessionDetailJSON
	if err := json.Unmarshal(clean, &raw); err != nil {
		return nil, fmt.Errorf("parse session detail json: %w", err)
	}
	return sessionDetailFromRaw(&raw), nil
}

// sessionDetailFromRaw converts an occtlSessionDetailJSON into a SessionDetail.
func sessionDetailFromRaw(raw *occtlSessionDetailJSON) *SessionDetail {
	sd := &SessionDetail{
		ID:            raw.Session,
		Username:      raw.Username,
		RemoteAddress: raw.RemoteIP,
		UserAgent:     raw.UserAgent,
		Device:        raw.Device,
		IPv4:          raw.IPv4,
		IPv6:          raw.IPv6,
		DNS:           raw.DNS,
		State:         raw.State,
		Groupname:     raw.Groupname,
		Vhost:         raw.Vhost,
	}

	// Parse Routes: can be a string or array in JSON due to duplicate keys.
	if raw.Routes != nil {
		var routesArr []string
		if err := json.Unmarshal(raw.Routes, &routesArr); err == nil {
			sd.Routes = routesArr
		} else {
			var routeStr string
			if err := json.Unmarshal(raw.Routes, &routeStr); err == nil && routeStr != "" {
				sd.Routes = []string{routeStr}
			}
		}
	}

	// Handle byte counts: prefer raw_rx/raw_tx if present, otherwise parse RX/TX strings.
	if raw.RawRx > 0 {
		sd.RxBytes = raw.RawRx
	} else if raw.RX != "" {
		sd.RxBytes = ByteValue(raw.RX)
	}
	if raw.RawTx > 0 {
		sd.TxBytes = raw.RawTx
	} else if raw.TX != "" {
		sd.TxBytes = ByteValue(raw.TX)
	}

	// VPNAddress: use IPv4 as primary VPN address.
	if raw.IPv4 != "" {
		sd.VPNAddress = raw.IPv4
	}

	// Parse connected time. Try "Connected at" first, then "Created", then raw_connected_at.
	if raw.ConnectedAt != "" {
		if t, err := time.Parse("2006-01-02 15:04", raw.ConnectedAt); err == nil {
			sd.ConnectedAt = t
		}
	}
	if sd.ConnectedAt.IsZero() && raw.RawConnectedAt > 0 {
		sd.ConnectedAt = time.Unix(raw.RawConnectedAt, 0)
	}
	if sd.ConnectedAt.IsZero() && raw.Created != "" {
		// Try as timestamp.
		if t, err := time.Parse(time.RFC3339, raw.Created); err == nil {
			sd.ConnectedAt = t
		} else if t, err := time.Parse("2006-01-02 15:04", raw.Created); err == nil {
			sd.ConnectedAt = t
		} else {
			// It might be a duration string like "2days" -- approximate.
			if d := DurationString(raw.Created); d > 0 {
				sd.ConnectedAt = time.Now().Add(-d)
			}
		}
	}

	if !sd.ConnectedAt.IsZero() {
		sd.Duration = time.Since(sd.ConnectedAt)
	}

	return sd
}

// SessionDetailText parses text output from `occtl show session <id>`.
// The format is similar to `occtl show user <name>` but contains session-specific
// fields like "Full session ID", "In use", "Activated", "Certificate auth".
func SessionDetailText(text string) (*SessionDetail, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil, fmt.Errorf("empty session detail output")
	}

	sd := &SessionDetail{}
	lines := strings.Split(text, "\n")

	var collectingDNS bool
	var collectingRoutes bool

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		// Handle DNS and Routes continuation lines.
		if collectingDNS && !strings.Contains(trimmed, ":") {
			sd.DNS = append(sd.DNS, trimmed)
			continue
		} else if collectingDNS && !strings.HasPrefix(trimmed, "DNS:") {
			collectingDNS = false
		}

		if collectingRoutes && !strings.Contains(trimmed, ":") {
			sd.Routes = append(sd.Routes, trimmed)
			continue
		} else if collectingRoutes && !strings.HasPrefix(trimmed, "Routes:") {
			collectingRoutes = false
		}

		switch {
		case strings.HasPrefix(trimmed, "Session:"):
			sd.ID = ExtractValue(trimmed, "Session:")

		case strings.HasPrefix(trimmed, "Username:"):
			sd.Username = ExtractValue(trimmed, "Username:")

		case strings.HasPrefix(trimmed, "Remote IP:"):
			sd.RemoteAddress = ExtractValue(trimmed, "Remote IP:")

		case strings.HasPrefix(trimmed, "User-Agent:"):
			sd.UserAgent = ExtractValue(trimmed, "User-Agent:")

		case strings.HasPrefix(trimmed, "vhost:"):
			// "vhost: default   User-Agent: AnyConnect Android 5.1.15.295"
			if idx := strings.Index(trimmed, "User-Agent:"); idx >= 0 {
				sd.UserAgent = ExtractValue(trimmed[idx:], "User-Agent:")
			}

		case strings.HasPrefix(trimmed, "Device:"):
			sd.Device = ExtractValue(trimmed, "Device:")

		case strings.HasPrefix(trimmed, "IPv4:"):
			sd.IPv4 = ExtractValue(trimmed, "IPv4:")
			if sd.VPNAddress == "" {
				sd.VPNAddress = sd.IPv4
			}
			if idx := strings.Index(trimmed, "P-t-P IPv4:"); idx >= 0 {
				// P-t-P IPv4 is not stored in SessionDetail but we skip it.
			}

		case strings.HasPrefix(trimmed, "IPv6:"):
			sd.IPv6 = ExtractValue(trimmed, "IPv6:")

		case strings.HasPrefix(trimmed, "RX:"):
			sd.RxBytes = ByteValue(ExtractValue(trimmed, "RX:"))
			if idx := strings.Index(trimmed, "TX:"); idx >= 0 {
				sd.TxBytes = ByteValue(ExtractValue(trimmed[idx:], "TX:"))
			}

		case strings.HasPrefix(trimmed, "Connected at:"):
			sd.ConnectedAt = ConnectedAt(trimmed)

		case strings.HasPrefix(trimmed, "Created:"):
			if sd.ConnectedAt.IsZero() {
				rest := ExtractValue(trimmed, "Created:")
				if t, err := time.Parse("2006-01-02 15:04", rest); err == nil {
					sd.ConnectedAt = t
				}
			}

		case strings.HasPrefix(trimmed, "DNS:"):
			collectingDNS = true
			collectingRoutes = false
			v := ExtractValue(trimmed, "DNS:")
			if v != "" {
				sd.DNS = append(sd.DNS, v)
			}

		case strings.HasPrefix(trimmed, "Routes:"):
			collectingRoutes = true
			collectingDNS = false
			v := ExtractValue(trimmed, "Routes:")
			if v != "" {
				sd.Routes = append(sd.Routes, v)
			}

		default:
			// Unknown line — skip gracefully.
		}
	}

	if !sd.ConnectedAt.IsZero() {
		sd.Duration = time.Since(sd.ConnectedAt)
	}

	return sd, nil
}

// ---------------------------------------------------------------------------
// status.go
// ---------------------------------------------------------------------------

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
	if err := json.Unmarshal(sanitizeOcctlJSON(data), &raw); err != nil {
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

	// MTU fields
	status.MinMTU = jsonInt(raw, "Min MTU")
	status.MaxMTU = jsonInt(raw, "Max MTU")

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

// ---------------------------------------------------------------------------
// user.go
// ---------------------------------------------------------------------------

// UserDetailJSON parses JSON output from `occtl -j -n show user <name>`.
// The output is an array of user connection objects (same struct as `show users`).
func UserDetailJSON(data []byte) ([]UserConnection, error) {
	var raw []occtlUserJSON
	if err := json.Unmarshal(sanitizeOcctlJSON(data), &raw); err != nil {
		return nil, err
	}

	conns := make([]UserConnection, 0, len(raw))
	for _, r := range raw {
		rxBytes, _ := strconv.ParseInt(r.RX, 10, 64)
		txBytes, _ := strconv.ParseInt(r.TX, 10, 64)

		connAt := time.Time{}
		if r.RawConnectedAt > 0 {
			connAt = time.Unix(r.RawConnectedAt, 0)
		} else {
			connAt = OcctlTimestamp(r.ConnectedAt)
		}

		groupname := r.Groupname
		if groupname == "(none)" {
			groupname = ""
		}

		var routes []string
		if r.Routes != nil {
			var routesArr []string
			if err := json.Unmarshal(r.Routes, &routesArr); err == nil {
				routes = routesArr
			} else {
				var routeStr string
				if err := json.Unmarshal(r.Routes, &routeStr); err == nil && routeStr != "" {
					routes = []string{routeStr}
				}
			}
		}

		conn := UserConnection{
			ID:                 strconv.Itoa(r.ID),
			Username:           r.Username,
			Groupname:          groupname,
			State:              r.State,
			Vhost:              r.Vhost,
			Device:             r.Device,
			MTU:                IntSafe(r.MTU),
			RemoteIP:           r.RemoteIP,
			Location:           r.Location,
			LocalDeviceIP:      r.LocalDeviceIP,
			IPv4:               r.IPv4,
			PtPIPv4:            r.PtPIPv4,
			UserAgent:          r.UserAgent,
			RxBytes:            rxBytes,
			TxBytes:            txBytes,
			AvgRxBandwidth:     r.AvgRX,
			AvgTxBandwidth:     r.AvgTX,
			DPD:                IntSafe(r.DPD),
			KeepAlive:          IntSafe(r.KeepAlive),
			Hostname:           r.Hostname,
			ConnectedAt:        connAt,
			SessionID:          r.Session,
			TLSCipher:          r.TLSCipher,
			DTLSCipher:         r.DTLSCipher,
			DNS:                r.DNS,
			Routes:             routes,
			RestrictedToRoutes: strings.EqualFold(r.RestrictedRoute, "true"),
		}
		conns = append(conns, conn)
	}
	return conns, nil
}

// UserDetailOutput parses the text output of `occtl show user <name>` into
// a slice of UserConnection structs. A single user may have multiple active
// connections, each represented as a separate block in the output.
//
// Returns an empty slice (not an error) when the input is empty.
func UserDetailOutput(text string) ([]UserConnection, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return []UserConnection{}, nil
	}

	blocks := splitUserBlocks(text)
	connections := make([]UserConnection, 0, len(blocks))
	for _, block := range blocks {
		conn := parseUserBlock(block)
		connections = append(connections, conn)
	}
	return connections, nil
}

// splitUserBlocks splits the raw occtl output into individual connection blocks.
// Each block starts with a line containing "ID:" and is separated from the next
// block by one or more blank lines followed by a new "ID:" line.
func splitUserBlocks(text string) []string {
	lines := strings.Split(text, "\n")
	var blocks []string
	var current []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "ID:") && len(current) > 0 {
			// Check if there were blank lines before this ID line (block separator).
			// Also handle the case where current block exists — start a new one.
			blocks = append(blocks, strings.Join(current, "\n"))
			current = nil
		}
		current = append(current, line)
	}
	if len(current) > 0 {
		blocks = append(blocks, strings.Join(current, "\n"))
	}
	return blocks
}

// parseUserBlock parses a single connection block into a UserConnection.
func parseUserBlock(block string) UserConnection {
	var conn UserConnection
	lines := strings.Split(block, "\n")

	// Track multi-value continuation state for DNS and Routes.
	var collectingDNS bool
	var collectingRoutes bool

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		// Continuation lines for DNS or Routes are more deeply indented
		// (typically starting with extra whitespace and no colon-key prefix).
		if collectingDNS && !strings.Contains(trimmed, ":") {
			conn.DNS = append(conn.DNS, trimmed)
			continue
		} else if collectingDNS && !strings.HasPrefix(trimmed, "DNS:") {
			collectingDNS = false
		}

		if collectingRoutes && !strings.Contains(trimmed, ":") {
			conn.Routes = append(conn.Routes, trimmed)
			continue
		} else if collectingRoutes && !strings.HasPrefix(trimmed, "Routes:") {
			collectingRoutes = false
		}

		// Parse key-value pairs. Some lines have multiple key-value pairs
		// separated by multiple spaces.
		switch {
		case strings.HasPrefix(trimmed, "ID:"):
			conn.ID = ExtractValue(trimmed, "ID:")

		case strings.HasPrefix(trimmed, "Username:"):
			// "Username: maxxie   Groupname: (none)"
			conn.Username = ExtractValue(trimmed, "Username:")
			if idx := strings.Index(trimmed, "Groupname:"); idx >= 0 {
				g := ExtractValue(trimmed[idx:], "Groupname:")
				if g == "(none)" {
					g = ""
				}
				conn.Groupname = g
			}

		case strings.HasPrefix(trimmed, "State:"):
			conn.State = ExtractValue(trimmed, "State:")

		case strings.HasPrefix(trimmed, "vhost:"):
			conn.Vhost = ExtractValue(trimmed, "vhost:")

		case strings.HasPrefix(trimmed, "Device:"):
			// "Device: vpns2   MTU: 1284"
			conn.Device = ExtractValue(trimmed, "Device:")
			if idx := strings.Index(trimmed, "MTU:"); idx >= 0 {
				conn.MTU = IntSafe(ExtractValue(trimmed[idx:], "MTU:"))
			}

		case strings.HasPrefix(trimmed, "Remote IP:"):
			// "Remote IP: 94.204.188.26   Location: unknown"
			conn.RemoteIP = ExtractValue(trimmed, "Remote IP:")
			if idx := strings.Index(trimmed, "Location:"); idx >= 0 {
				conn.Location = ExtractValue(trimmed[idx:], "Location:")
			}

		case strings.HasPrefix(trimmed, "Local Device IP:"):
			conn.LocalDeviceIP = ExtractValue(trimmed, "Local Device IP:")

		case strings.HasPrefix(trimmed, "IPv4:"):
			// "IPv4: 10.10.10.228   P-t-P IPv4: 10.10.10.1"
			conn.IPv4 = ExtractValue(trimmed, "IPv4:")
			if idx := strings.Index(trimmed, "P-t-P IPv4:"); idx >= 0 {
				conn.PtPIPv4 = ExtractValue(trimmed[idx:], "P-t-P IPv4:")
			}

		case strings.HasPrefix(trimmed, "User-Agent:"):
			conn.UserAgent = ExtractValue(trimmed, "User-Agent:")

		case strings.HasPrefix(trimmed, "RX:"):
			// "RX: 4359893 (4.4 MB)   TX: 361852318 (361.9 MB)"
			conn.RxBytes = ByteValue(ExtractValue(trimmed, "RX:"))
			if idx := strings.Index(trimmed, "TX:"); idx >= 0 {
				conn.TxBytes = ByteValue(ExtractValue(trimmed[idx:], "TX:"))
			}

		case strings.HasPrefix(trimmed, "Average bandwidth"):
			// "Average bandwidth RX: 820 bytes/s  TX: 68.1 kB/s"
			if idx := strings.Index(trimmed, "RX:"); idx >= 0 {
				conn.AvgRxBandwidth = ExtractBandwidthValue(trimmed[idx:], "RX:")
			}
			if idx := strings.LastIndex(trimmed, "TX:"); idx >= 0 {
				conn.AvgTxBandwidth = ExtractBandwidthValue(trimmed[idx:], "TX:")
			}

		case strings.HasPrefix(trimmed, "DPD:"):
			// "DPD: 90   KeepAlive: 32400"
			conn.DPD = IntSafe(ExtractValue(trimmed, "DPD:"))
			if idx := strings.Index(trimmed, "KeepAlive:"); idx >= 0 {
				conn.KeepAlive = IntSafe(ExtractValue(trimmed[idx:], "KeepAlive:"))
			}

		case strings.HasPrefix(trimmed, "Hostname:"):
			conn.Hostname = ExtractValue(trimmed, "Hostname:")

		case strings.HasPrefix(trimmed, "Connected at:"):
			// "Connected at: 2026-05-07 11:58 ( 1h:28m)"
			conn.ConnectedAt = ConnectedAt(trimmed)

		case strings.HasPrefix(trimmed, "Session:"):
			conn.SessionID = ExtractValue(trimmed, "Session:")

		case strings.HasPrefix(trimmed, "TLS ciphersuite:"):
			conn.TLSCipher = ExtractValue(trimmed, "TLS ciphersuite:")

		case strings.HasPrefix(trimmed, "DTLS cipher:"):
			conn.DTLSCipher = ExtractValue(trimmed, "DTLS cipher:")

		case strings.HasPrefix(trimmed, "DNS:"):
			collectingDNS = true
			collectingRoutes = false
			v := ExtractValue(trimmed, "DNS:")
			if v != "" {
				conn.DNS = append(conn.DNS, v)
			}

		case strings.HasPrefix(trimmed, "Routes:"):
			collectingRoutes = true
			collectingDNS = false
			v := ExtractValue(trimmed, "Routes:")
			if v != "" {
				conn.Routes = append(conn.Routes, v)
			}

		case strings.HasPrefix(trimmed, "Restricted to routes:"):
			v := ExtractValue(trimmed, "Restricted to routes:")
			conn.RestrictedToRoutes = strings.EqualFold(v, "true")

		default:
			// Unknown line — skip gracefully.
		}
	}

	return conn
}

// ---------------------------------------------------------------------------
// users.go
// ---------------------------------------------------------------------------

// occtlUserJSON is the intermediate JSON struct for unmarshaling `occtl -j -n show users` output.
// Fields use the exact keys from actual occtl JSON output (spaces in names, strings for numbers).
type occtlUserJSON struct {
	ID              int             `json:"ID"`
	Username        string          `json:"Username"`
	Groupname       string          `json:"Groupname"`
	State           string          `json:"State"`
	Vhost           string          `json:"vhost"`
	Device          string          `json:"Device"`
	MTU             string          `json:"MTU"`
	RemoteIP        string          `json:"Remote IP"`
	Location        string          `json:"Location"`
	LocalDeviceIP   string          `json:"Local Device IP"`
	IPv4            string          `json:"IPv4"`
	PtPIPv4         string          `json:"P-t-P IPv4"`
	UserAgent       string          `json:"User-Agent"`
	RX              string          `json:"RX"`
	TX              string          `json:"TX"`
	AvgRX           string          `json:"Average RX"`
	AvgTX           string          `json:"Average TX"`
	DPD             string          `json:"DPD"`
	KeepAlive       string          `json:"KeepAlive"`
	Hostname        string          `json:"Hostname"`
	ConnectedAt     string          `json:"Connected at"`
	ConnectedAtRel  string          `json:"_Connected at"`
	RawConnectedAt  int64           `json:"raw_connected_at"`
	FullSession     string          `json:"Full session"`
	Session         string          `json:"Session"`
	TLSCipher       string          `json:"TLS ciphersuite"`
	DTLSCipher      string          `json:"DTLS cipher"`
	DNS             []string        `json:"DNS"`
	Routes          json.RawMessage `json:"Routes"`
	RestrictedRoute string          `json:"Restricted to routes"`
}

// UsersJSON parses JSON array output from `occtl -j -n show users`.
func UsersJSON(data []byte) ([]ConnectedUser, error) {
	var raw []occtlUserJSON
	if err := json.Unmarshal(sanitizeOcctlJSON(data), &raw); err != nil {
		return nil, fmt.Errorf("parse users json: %w", err)
	}

	users := make([]ConnectedUser, 0, len(raw))
	for _, r := range raw {
		since := strings.TrimSpace(r.ConnectedAtRel)

		rxBytes, _ := strconv.ParseInt(r.RX, 10, 64)
		txBytes, _ := strconv.ParseInt(r.TX, 10, 64)

		var connAt time.Time
		if r.RawConnectedAt > 0 {
			connAt = time.Unix(r.RawConnectedAt, 0)
		} else if r.ConnectedAt != "" {
			connAt = OcctlTimestamp(r.ConnectedAt)
		}

		users = append(users, ConnectedUser{
			ID:            strconv.Itoa(r.ID),
			Username:      r.Username,
			Groupname:     r.Groupname,
			Vhost:         r.Vhost,
			RemoteIP:      r.RemoteIP,
			Location:      r.Location,
			VpnIP:         r.IPv4,
			Device:        r.Device,
			MTU:           IntSafe(r.MTU),
			LocalDeviceIP: r.LocalDeviceIP,
			PtPIPv4:       r.PtPIPv4,
			UserAgent:     r.UserAgent,
			RxBytes:       rxBytes,
			TxBytes:       txBytes,
			Hostname:      r.Hostname,
			Since:         since,
			Duration:      DurationString(since),
			ConnectedAt:   connAt,
			SessionID:     r.Session,
			FullSession:   r.FullSession,
			TLSCipher:     r.TLSCipher,
			DTLSCipher:    r.DTLSCipher,
			Status:        r.State,
		})
	}
	return users, nil
}

// cipherPattern matches a parenthesized DTLS cipher string like "(AES-128-GCM)" or "(CHACHA20-POLY1305)".
var cipherPattern = regexp.MustCompile(`\([A-Z0-9][-A-Z0-9]*\)`)

// UsersText parses tabular text output from `occtl show users` as a fallback when JSON is unavailable.
func UsersText(text string) ([]ConnectedUser, error) {
	lines := strings.Split(text, "\n")
	var users []ConnectedUser

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		// Skip header line (contains "id" and "user" and "vhost").
		lower := strings.ToLower(trimmed)
		if strings.Contains(lower, "id") && strings.Contains(lower, "user") && strings.Contains(lower, "vhost") {
			continue
		}

		// Strategy: split the first 7 fields by whitespace, then find cipher via regex,
		// and the status is whatever comes after the cipher.
		fields := strings.Fields(trimmed)
		if len(fields) < 8 {
			continue
		}

		id := fields[0]
		username := fields[1]
		vhost := fields[2]
		remoteIP := fields[3]
		vpnIP := fields[4]
		device := fields[5]
		since := fields[6]

		// Find the cipher by searching for (...) pattern in the remainder of the line.
		// Rebuild the tail after the first 7 fields.
		tail := RebuildTail(trimmed, fields, 7)

		var dtlsCipher string
		var status string

		loc := cipherPattern.FindStringIndex(tail)
		if loc != nil {
			dtlsCipher = tail[loc[0]:loc[1]]
			status = strings.TrimSpace(tail[loc[1]:])
		} else {
			// No cipher found; assume the remaining fields are cipher and status.
			remaining := fields[7:]
			if len(remaining) >= 2 {
				dtlsCipher = remaining[0]
				status = remaining[len(remaining)-1]
			} else if len(remaining) == 1 {
				status = remaining[0]
			}
		}

		users = append(users, ConnectedUser{
			ID:         id,
			Username:   username,
			Vhost:      vhost,
			RemoteIP:   remoteIP,
			VpnIP:      vpnIP,
			Device:     device,
			Since:      since,
			Duration:   DurationString(since),
			DTLSCipher: dtlsCipher,
			Status:     status,
		})
	}

	return users, nil
}

// ---------------------------------------------------------------------------
// bans.go
// ---------------------------------------------------------------------------

// EventsJSON parses the JSON output of `occtl show events --output-type json`.
// It returns an empty slice (never nil) when data is empty or represents an empty array.
// Timestamps that fail to parse are left as zero time without returning an error.
func EventsJSON(data []byte) ([]ConnectionEvent, error) {
	if len(data) == 0 {
		return []ConnectionEvent{}, nil
	}

	type rawEvent struct {
		Timestamp string `json:"timestamp"`
		Username  string `json:"username"`
		RemoteIP  string `json:"remote_ip"`
		EventType string `json:"event_type"`
		Message   string `json:"message"`
	}

	var raw []rawEvent
	if err := json.Unmarshal(sanitizeOcctlJSON(data), &raw); err != nil {
		return nil, fmt.Errorf("parse events JSON: %w", err)
	}

	events := make([]ConnectionEvent, 0, len(raw))
	for _, r := range raw {
		ts := Timestamp(r.Timestamp)
		events = append(events, ConnectionEvent{
			Timestamp: ts,
			Username:  r.Username,
			RemoteIP:  r.RemoteIP,
			EventType: r.EventType,
			Message:   r.Message,
		})
	}
	return events, nil
}

// BannedIPsJSON parses the JSON output of `occtl show ip bans --output-type json`.
// It returns an empty slice (never nil) when data is empty or represents an empty array.
func BannedIPsJSON(data []byte) ([]BannedIP, error) {
	if len(data) == 0 {
		return []BannedIP{}, nil
	}

	type rawBan struct {
		IP     string `json:"ip"`
		Score  int    `json:"score"`
		Reason string `json:"reason"`
	}

	var raw []rawBan
	if err := json.Unmarshal(sanitizeOcctlJSON(data), &raw); err != nil {
		return nil, fmt.Errorf("parse banned IPs JSON: %w", err)
	}

	bans := make([]BannedIP, 0, len(raw))
	for _, r := range raw {
		bans = append(bans, BannedIP{
			IP:     r.IP,
			Score:  r.Score,
			Reason: r.Reason,
		})
	}
	return bans, nil
}

// BanPointsJSON parses the JSON output of `occtl show ip ban points --output-type json`.
// It returns an empty slice (never nil) when data is empty or represents an empty array.
// Timestamps that fail to parse are left as zero time without returning an error.
func BanPointsJSON(data []byte) ([]BanPointEntry, error) {
	if len(data) == 0 {
		return []BanPointEntry{}, nil
	}

	type rawBanPoint struct {
		IP     string `json:"ip"`
		Points int    `json:"points"`
		Since  string `json:"since"`
	}

	var raw []rawBanPoint
	if err := json.Unmarshal(sanitizeOcctlJSON(data), &raw); err != nil {
		return nil, fmt.Errorf("parse ban points JSON: %w", err)
	}

	entries := make([]BanPointEntry, 0, len(raw))
	for _, r := range raw {
		ts := Timestamp(r.Since)
		entries = append(entries, BanPointEntry{
			IP:     r.IP,
			Points: r.Points,
			Since:  ts,
		})
	}
	return entries, nil
}

// parseTimestamp is an internal alias kept for readability.
func parseTimestamp(s string) time.Time {
	return Timestamp(s)
}

// ---------------------------------------------------------------------------
// routes.go
// ---------------------------------------------------------------------------

// RoutesText parses the text output of `occtl show iroutes`.
func RoutesText(text string) ([]UserRoute, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return []UserRoute{}, nil
	}

	lines := strings.Split(text, "\n")
	var routes []UserRoute

	headerSkipped := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		// Skip header line
		if !headerSkipped {
			if strings.Contains(trimmed, "iroute") || strings.Contains(trimmed, "id") {
				headerSkipped = true
				continue
			}
		}

		fields := strings.Fields(trimmed)
		if len(fields) < 6 {
			continue // incomplete line, skip
		}

		route := UserRoute{
			ID:       fields[0],
			Username: fields[1],
			Vhost:    fields[2],
			Device:   fields[3],
			VpnIP:    fields[4],
			Route:    strings.Join(fields[5:], " "),
		}
		routes = append(routes, route)
	}

	if routes == nil {
		routes = []UserRoute{}
	}
	return routes, nil
}
