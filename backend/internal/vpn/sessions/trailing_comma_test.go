package sessions_test

import (
	"testing"

	"ocserv_panel/backend/internal/vpn/sessions"
)

// TestSessionsJSONTrailingComma verifies that SessionsJSON handles the non-standard
// trailing commas produced by occtl -j output.
func TestSessionsJSONTrailingComma(t *testing.T) {
	t.Parallel()
	// occtl -j produces trailing commas which are not valid JSON.
	// Verify our parser handles them gracefully.
	input := `[
  {
    "Session":  "VcHB+d",
    "Full session":  "VcHB+dpjuHST3W1iIHRpsENaAWQ=",
    "Created":  "2026-05-09 18:26",
    "State":  "authenticated",
    "Username":  "spbclient",
    "Groupname":  "(none)",
    "vhost":  "default",
    "User-Agent":  "Open AnyConnect VPN Agent 9.12-3",
    "Remote IP":  "5.166.153.114",
    "Location":  "unknown",
    "session_is_open":  1,
    "tls_auth_ok":  0,
    "in_use":  1,
  }
]`
	sessions, err := sessions.SessionsJSON([]byte(input))
	if err != nil {
		t.Fatalf("SessionsJSON with trailing commas: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}
	if sessions[0].ID != "VcHB+d" {
		t.Errorf("ID: got %q want %q", sessions[0].ID, "VcHB+d")
	}
	if sessions[0].Username != "spbclient" {
		t.Errorf("Username: got %q want %q", sessions[0].Username, "spbclient")
	}
	if sessions[0].State != "authenticated" {
		t.Errorf("State: got %q want %q", sessions[0].State, "authenticated")
	}
	if sessions[0].RemoteAddress != "5.166.153.114" {
		t.Errorf("RemoteAddress: got %q want %q", sessions[0].RemoteAddress, "5.166.153.114")
	}
}

// TestSessionDetailJSONTrailingComma verifies that SessionDetailJSON handles trailing commas.
func TestSessionDetailJSONTrailingComma(t *testing.T) {
	t.Parallel()
	input := `[
  {
    "Session":  "YwDQr5",
    "Full session":  "YwDQr54wGdgKCAju+hgsYQcJLc0=",
    "Created":  "2026-05-09 12:24",
    "State":  "authenticated",
    "Username":  "maxxie",
    "Groupname":  "(none)",
    "vhost":  "default",
    "User-Agent":  "Open AnyConnect VPN Agent 9.12-3",
    "Remote IP":  "5.166.153.114",
    "Location":  "unknown",
    "session_is_open":  1,
    "tls_auth_ok":  0,
    "in_use":  1,
  }
]`
	detail, err := sessions.SessionDetailJSON([]byte(input))
	if err != nil {
		t.Fatalf("SessionDetailJSON with trailing commas: %v", err)
	}
	if detail.ID != "YwDQr5" {
		t.Errorf("ID: got %q want %q", detail.ID, "YwDQr5")
	}
	if detail.Username != "maxxie" {
		t.Errorf("Username: got %q want %q", detail.Username, "maxxie")
	}
	if detail.RemoteAddress != "5.166.153.114" {
		t.Errorf("RemoteAddress: got %q want %q", detail.RemoteAddress, "5.166.153.114")
	}
}

// TestUsersJSONTrailingComma verifies that UsersJSON handles trailing commas.
func TestUsersJSONTrailingComma(t *testing.T) {
	t.Parallel()
	input := `[
  {
    "ID":  3549163,
    "Username":  "maxxie",
    "Groupname":  "(none)",
    "State":  "connected",
    "vhost":  "default",
    "Device":  "vpns0",
    "MTU":  "1292",
    "Remote IP":  "5.166.153.114",
    "Location":  "unknown",
    "Local Device IP":  "188.68.41.144",
    "IPv4":  "10.10.10.208",
    "P-t-P IPv4":  "10.10.10.1",
    "User-Agent":  "Open AnyConnect VPN Agent 9.12-3",
    "RX":  "304",
    "TX":  "304",
    "Average RX":  "0 bytes/s",
    "Average TX":  "0 bytes/s",
    "DPD":  "90",
    "KeepAlive":  "32400",
    "Hostname":  "LemonGrass",
    "Connected at":  "2026-05-09 12:23",
    "_Connected at":  " 3h:52m",
    "raw_connected_at":  1778322239,
    "Full session":  "YwDQr54wGdgKCAju+hgsYQcJLc0=",
    "Session":  "YwDQr5",
    "TLS ciphersuite":  "(TLS1.3)-(ECDHE-SECP256R1)-(ECDSA-SECP256R1-SHA256)-(CHACHA20-P",
    "DTLS cipher":  "(DTLS1.2)-(PSK)-(CHACHA20-POLY1305)",
    "DNS":      ["1.1.1.1", "9.9.9.9"],
    "Routes":  "defaultroute",
    "Restricted to routes":  "False",
  }
]`
	users, err := sessions.UsersJSON([]byte(input))
	if err != nil {
		t.Fatalf("UsersJSON with trailing commas: %v", err)
	}
	if len(users) != 1 {
		t.Fatalf("expected 1 user, got %d", len(users))
	}
	if users[0].Username != "maxxie" {
		t.Errorf("Username: got %q want %q", users[0].Username, "maxxie")
	}
	if users[0].RemoteIP != "5.166.153.114" {
		t.Errorf("RemoteIP: got %q want %q", users[0].RemoteIP, "5.166.153.114")
	}
	if users[0].Status != "connected" {
		t.Errorf("Status: got %q want %q", users[0].Status, "connected")
	}
}

// TestUserDetailJSONTrailingComma verifies that UserDetailJSON handles trailing commas.
func TestUserDetailJSONTrailingComma(t *testing.T) {
	t.Parallel()
	input := `[
  {
    "ID":  3549163,
    "Username":  "maxxie",
    "Groupname":  "(none)",
    "State":  "connected",
    "vhost":  "default",
    "Device":  "vpns0",
    "MTU":  "1292",
    "Remote IP":  "5.166.153.114",
    "IPv4":  "10.10.10.208",
    "User-Agent":  "Open AnyConnect VPN Agent 9.12-3",
    "RX":  "304",
    "TX":  "304",
    "Connected at":  "2026-05-09 12:23",
    "raw_connected_at":  1778322239,
    "Session":  "YwDQr5",
    "DTLS cipher":  "(DTLS1.2)-(PSK)-(CHACHA20-POLY1305)",
    "Restricted to routes":  "False",
  }
]`
	conns, err := sessions.UserDetailJSON([]byte(input))
	if err != nil {
		t.Fatalf("UserDetailJSON with trailing commas: %v", err)
	}
	if len(conns) != 1 {
		t.Fatalf("expected 1 connection, got %d", len(conns))
	}
	if conns[0].Username != "maxxie" {
		t.Errorf("Username: got %q want %q", conns[0].Username, "maxxie")
	}
	if conns[0].RemoteIP != "5.166.153.114" {
		t.Errorf("RemoteIP: got %q want %q", conns[0].RemoteIP, "5.166.153.114")
	}
	if conns[0].SessionID != "YwDQr5" {
		t.Errorf("SessionID: got %q want %q", conns[0].SessionID, "YwDQr5")
	}
}

// TestStatusJSONTrailingComma verifies that StatusJSON handles trailing commas.
func TestStatusJSONTrailingComma(t *testing.T) {
	t.Parallel()
	input := `{
    "Status":  "online",
    "Server PID":  3108823,
    "Sec-mod PID":  3108828,
    "Active sessions":  2,
    "Total sessions":  30,
    "raw_up_since":  1777872597,
    "raw_rx":  122275000,
    "raw_tx":  3619653000,
  }`
	status, err := sessions.StatusJSON([]byte(input))
	if err != nil {
		t.Fatalf("StatusJSON with trailing commas: %v", err)
	}
	if status.Status != "online" {
		t.Errorf("Status: got %q want %q", status.Status, "online")
	}
	if status.ActiveSessions != 2 {
		t.Errorf("ActiveSessions: got %d want %d", status.ActiveSessions, 2)
	}
	if status.TotalSessions != 30 {
		t.Errorf("TotalSessions: got %d want %d", status.TotalSessions, 30)
	}
	if status.RxBytes != 122275000 {
		t.Errorf("RxBytes: got %d want %d", status.RxBytes, 122275000)
	}
}

// TestEventsJSONTrailingComma verifies that EventsJSON handles trailing commas.
func TestEventsJSONTrailingComma(t *testing.T) {
	t.Parallel()
	input := `[
  {
    "timestamp": "2026-05-07T12:00:00Z",
    "username": "testuser",
    "remote_ip": "192.168.1.100",
    "event_type": "connect",
    "message": "user connected",
  }
]`
	events, err := sessions.EventsJSON([]byte(input))
	if err != nil {
		t.Fatalf("EventsJSON with trailing commas: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Username != "testuser" {
		t.Errorf("Username: got %q want %q", events[0].Username, "testuser")
	}
}

// TestBannedIPsJSONTrailingComma verifies that BannedIPsJSON handles trailing commas.
func TestBannedIPsJSONTrailingComma(t *testing.T) {
	t.Parallel()
	input := `[
  {
    "IP": "10.0.0.1",
    "Score": 100,
    "Reason": "brute force",
  }
]`
	bans, err := sessions.BannedIPsJSON([]byte(input))
	if err != nil {
		t.Fatalf("BannedIPsJSON with trailing commas: %v", err)
	}
	if len(bans) != 1 {
		t.Fatalf("expected 1 ban, got %d", len(bans))
	}
	if bans[0].IP != "10.0.0.1" {
		t.Errorf("IP: got %q want %q", bans[0].IP, "10.0.0.1")
	}
	if bans[0].Score != 100 {
		t.Errorf("Score: got %d want %d", bans[0].Score, 100)
	}
}

// TestBanPointsJSONTrailingComma verifies that BanPointsJSON handles trailing commas.
func TestBanPointsJSONTrailingComma(t *testing.T) {
	t.Parallel()
	input := `[
  {
    "IP": "172.16.0.5",
    "Score": 42,
  }
]`
	entries, err := sessions.BanPointsJSON([]byte(input))
	if err != nil {
		t.Fatalf("BanPointsJSON with trailing commas: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].IP != "172.16.0.5" {
		t.Errorf("IP: got %q want %q", entries[0].IP, "172.16.0.5")
	}
	if entries[0].Score != 42 {
		t.Errorf("Score: got %d want %d", entries[0].Score, 42)
	}
}
