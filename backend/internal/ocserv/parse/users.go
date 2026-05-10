package parse

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

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
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse users json: %w", err)
	}

	users := make([]ConnectedUser, 0, len(raw))
	for _, r := range raw {
		since := strings.TrimSpace(r.ConnectedAtRel)
		users = append(users, ConnectedUser{
			ID:         strconv.Itoa(r.ID),
			Username:   r.Username,
			Vhost:      r.Vhost,
			RemoteIP:   r.RemoteIP,
			VpnIP:      r.IPv4,
			Device:     r.Device,
			Since:      since,
			Duration:   DurationString(since),
			DTLSCipher: r.DTLSCipher,
			Status:     r.State,
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
