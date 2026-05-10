package parse

import (
	"encoding/json"
	"strconv"
	"strings"
	"time"
)

// UserDetailJSON parses JSON output from `occtl -j -n show user <name>`.
// The output is an array of user connection objects (same struct as `show users`).
func UserDetailJSON(data []byte) ([]UserConnection, error) {
	var raw []occtlUserJSON
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}

	conns := make([]UserConnection, 0, len(raw))
	for _, r := range raw {
		rxBytes, _ := strconv.ParseInt(r.RX, 10, 64)
		txBytes, _ := strconv.ParseInt(r.TX, 10, 64)

		connAt := OcctlTimestamp(r.ConnectedAt)
		if connAt.IsZero() && r.RawConnectedAt > 0 {
			connAt = time.Unix(r.RawConnectedAt, 0)
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
