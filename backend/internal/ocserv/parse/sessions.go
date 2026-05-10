package parse

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

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
	TLSAuthOK  int    `json:"tls_auth_ok"`
	InUse      int    `json:"in_use"`
}

// SessionsJSON parses JSON array output from `occtl -j -n show sessions all`.
func SessionsJSON(data []byte) ([]Session, error) {
	var raw []occtlSessionJSON
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse sessions json: %w", err)
	}

	sessions := make([]Session, 0, len(raw))
	for _, r := range raw {
		s := Session{
			ID:            r.Session,
			Username:      r.Username,
			RemoteAddress: r.RemoteIP,
			UserAgent:     r.UserAgent,
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
		}

		if d := DurationString(created); d > 0 {
			s.ConnectedAt = time.Now().Add(-d)
		}
		_ = status // Status not stored in Session struct currently.

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
	TLSAuthOK  int    `json:"tls_auth_ok"`
	InUse      int    `json:"in_use"`
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
	// Try array format first (actual occtl behavior).
	var rawArr []occtlSessionDetailJSON
	if err := json.Unmarshal(data, &rawArr); err == nil && len(rawArr) > 0 {
		return sessionDetailFromRaw(&rawArr[0]), nil
	}

	// Try single object format (for test compatibility).
	var raw occtlSessionDetailJSON
	if err := json.Unmarshal(data, &raw); err != nil {
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
