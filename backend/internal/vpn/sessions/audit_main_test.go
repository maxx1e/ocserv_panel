package sessions_test

import (
	"encoding/json"
	"testing"
	"time"

	"ocserv_panel/backend/internal/vpn/sessions"
)

// TestReferenceExamples validates all parsers against the exact occtl output
// documented in .claude/rules/occtl-excamples.md.

// ---------------------------------------------------------------------------
// show status
// ---------------------------------------------------------------------------

func TestReferenceStatusJSON(t *testing.T) {
	t.Parallel()
	input := []byte(` {
    "Status":  "online",
    "Server PID":  3108823,
    "Sec-mod PID":  3108828,
    "Sec-mod instance count":  1,
    "Up since":  "2026-05-04 07:29",
    "_Up since":  " 5days",
    "raw_up_since":  1777872597,
    "uptime":  463303,
    "Active sessions":  2,
    "Total sessions":  30,
    "Total authentication failures":  9,
    "IPs in ban list":  0,
    "Last stats reset":  "2026-05-04 07:29",
    "_Last stats reset":  " 5days",
    "raw_last_stats_reset":  1777872597,
    "Sessions handled":  30,
    "Timed out sessions":  0,
    "Timed out (idle) sessions":  0,
    "Closed due to error sessions":  16,
    "Authentication failures":  9,
    "Average auth time":  "    2s",
    "raw_avg_auth_time":  2,
    "Max auth time":  "   10s",
    "raw_max_auth_time":  10,
    "Average session time":  " 7h:40m",
    "raw_avg_session_time":  27600,
    "Max session time":  " 5days",
    "raw_max_session_time":  449520,
    "Min MTU":  1284,
    "Max MTU":  1292,
    "RX":  "122.3 MB",
    "raw_rx":  122275000,
    "TX":  "3.6 GB",
    "raw_tx":  3619653000
  }`)

	st, err := sessions.StatusJSON(input)
	if err != nil {
		t.Fatalf("StatusJSON: %v", err)
	}

	// Validate every mapped field from the reference output.
	assertEqual(t, "Status", st.Status, "online")
	assertEqualInt(t, "ServerPID", st.ServerPID, 3108823)
	assertEqualInt(t, "SecModPID", st.SecModPID, 3108828)
	assertEqualInt(t, "SecModInstances", st.SecModInstances, 1)
	assertEqualInt(t, "ActiveSessions", st.ActiveSessions, 2)
	assertEqualInt(t, "TotalSessions", st.TotalSessions, 30)
	assertEqualInt(t, "TotalAuthFailures", st.TotalAuthFailures, 9)
	assertEqualInt(t, "BannedIPs", st.BannedIPs, 0)
	assertEqualInt(t, "SessionsHandled", st.SessionsHandled, 30)
	assertEqualInt(t, "TimedOutSessions", st.TimedOutSessions, 0)
	assertEqualInt(t, "IdleTimedOutSessions", st.IdleTimedOutSessions, 0)
	assertEqualInt(t, "ErrorClosedSessions", st.ErrorClosedSessions, 16)
	assertEqualInt(t, "AuthFailures", st.AuthFailures, 9)
	assertEqualInt(t, "MinMTU", st.MinMTU, 1284)
	assertEqualInt(t, "MaxMTU", st.MaxMTU, 1292)
	assertEqualInt64(t, "RxBytes", st.RxBytes, 122275000)
	assertEqualInt64(t, "TxBytes", st.TxBytes, 3619653000)

	// Durations from raw_* fields (seconds).
	assertEqualDuration(t, "AvgAuthTime", st.AvgAuthTime, 2*time.Second)
	assertEqualDuration(t, "MaxAuthTime", st.MaxAuthTime, 10*time.Second)
	assertEqualDuration(t, "AvgSessionTime", st.AvgSessionTime, 27600*time.Second)
	assertEqualDuration(t, "MaxSessionTime", st.MaxSessionTime, 449520*time.Second)

	// Timestamps from raw_up_since (unix epoch).
	expectedUpSince := time.Unix(1777872597, 0)
	if !st.UpSince.Equal(expectedUpSince) {
		t.Errorf("UpSince: got %v, want %v", st.UpSince, expectedUpSince)
	}
	expectedLastReset := time.Unix(1777872597, 0)
	if !st.LastStatsReset.Equal(expectedLastReset) {
		t.Errorf("LastStatsReset: got %v, want %v", st.LastStatsReset, expectedLastReset)
	}
}

// ---------------------------------------------------------------------------
// show sessions all
// ---------------------------------------------------------------------------

func TestReferenceSessionsAllJSON(t *testing.T) {
	t.Parallel()
	input := []byte(`[
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
  },

  {
    "Session":  "fySBxd",
    "Full session":  "fySBxdqjHqDbKTT4g/IoG4H0VYY=",
    "Created":  "2026-05-08 15:07",
    "State":  "authenticated",
    "Username":  "maxxie",
    "Groupname":  "(none)",
    "vhost":  "default",
    "User-Agent":  "AnyConnect Android 5.1.15.295",
    "Remote IP":  "94.204.188.26",
    "Location":  "unknown",
    "session_is_open":  1,
    "tls_auth_ok":  0,
    "in_use":  1,
  }
]`)

	sessions, err := sessions.SessionsJSON(input)
	if err != nil {
		t.Fatalf("SessionsJSON: %v", err)
	}
	if len(sessions) != 2 {
		t.Fatalf("count: got %d want 2", len(sessions))
	}

	// --- Session 0 field mapping ---
	s0 := sessions[0]
	assertEqual(t, "s0.ID", s0.ID, "YwDQr5")
	assertEqual(t, "s0.FullSession", s0.FullSession, "YwDQr54wGdgKCAju+hgsYQcJLc0=")
	assertEqual(t, "s0.Username", s0.Username, "maxxie")
	assertEqual(t, "s0.State", s0.State, "authenticated")
	assertEqual(t, "s0.Groupname", s0.Groupname, "(none)")
	assertEqual(t, "s0.Vhost", s0.Vhost, "default")
	assertEqual(t, "s0.UserAgent", s0.UserAgent, "Open AnyConnect VPN Agent 9.12-3")
	assertEqual(t, "s0.RemoteAddress", s0.RemoteAddress, "5.166.153.114")
	assertEqual(t, "s0.Location", s0.Location, "unknown")
	assertBool(t, "s0.SessionIsOpen", s0.SessionIsOpen, true)
	assertBool(t, "s0.InUse", s0.InUse, true)

	// ConnectedAt derived from "Created": "2026-05-09 12:24"
	expectedTime := time.Date(2026, 5, 9, 12, 24, 0, 0, time.UTC)
	if !s0.ConnectedAt.Equal(expectedTime) {
		t.Errorf("s0.ConnectedAt: got %v, want %v", s0.ConnectedAt, expectedTime)
	}

	// Fields that are NOT populated from sessions output should be zero.
	assertEqualInt64(t, "s0.RxBytes", s0.RxBytes, 0)
	assertEqualInt64(t, "s0.TxBytes", s0.TxBytes, 0)
	assertEqual(t, "s0.VPNAddress", s0.VPNAddress, "")
	assertEqual(t, "s0.Device", s0.Device, "")

	// --- Session 1 field mapping ---
	s1 := sessions[1]
	assertEqual(t, "s1.ID", s1.ID, "fySBxd")
	assertEqual(t, "s1.FullSession", s1.FullSession, "fySBxdqjHqDbKTT4g/IoG4H0VYY=")
	assertEqual(t, "s1.Username", s1.Username, "maxxie")
	assertEqual(t, "s1.State", s1.State, "authenticated")
	assertEqual(t, "s1.UserAgent", s1.UserAgent, "AnyConnect Android 5.1.15.295")
	assertEqual(t, "s1.RemoteAddress", s1.RemoteAddress, "94.204.188.26")
	assertEqual(t, "s1.Location", s1.Location, "unknown")
	assertBool(t, "s1.SessionIsOpen", s1.SessionIsOpen, true)
	assertBool(t, "s1.InUse", s1.InUse, true)

	expectedTime1 := time.Date(2026, 5, 8, 15, 7, 0, 0, time.UTC)
	if !s1.ConnectedAt.Equal(expectedTime1) {
		t.Errorf("s1.ConnectedAt: got %v, want %v", s1.ConnectedAt, expectedTime1)
	}
}

// ---------------------------------------------------------------------------
// show session <id>
// ---------------------------------------------------------------------------

func TestReferenceSessionDetailJSON(t *testing.T) {
	t.Parallel()
	input := []byte(`[
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
  },
]`)

	sd, err := sessions.SessionDetailJSON(input)
	if err != nil {
		t.Fatalf("SessionDetailJSON: %v", err)
	}
	if sd == nil {
		t.Fatal("got nil SessionDetail")
	}

	assertEqual(t, "ID", sd.ID, "YwDQr5")
	assertEqual(t, "Username", sd.Username, "maxxie")
	assertEqual(t, "State", sd.State, "authenticated")
	assertEqual(t, "Groupname", sd.Groupname, "(none)")
	assertEqual(t, "Vhost", sd.Vhost, "default")
	assertEqual(t, "UserAgent", sd.UserAgent, "Open AnyConnect VPN Agent 9.12-3")
	assertEqual(t, "RemoteAddress", sd.RemoteAddress, "5.166.153.114")

	// ConnectedAt from "Created" field.
	expectedTime := time.Date(2026, 5, 9, 12, 24, 0, 0, time.UTC)
	if !sd.ConnectedAt.Equal(expectedTime) {
		t.Errorf("ConnectedAt: got %v, want %v", sd.ConnectedAt, expectedTime)
	}
}

// ---------------------------------------------------------------------------
// show users
// ---------------------------------------------------------------------------

func TestReferenceUsersJSON(t *testing.T) {
	t.Parallel()
	input := []byte(`[
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
    "_RX":  "304 bytes",
    "_TX":  "304 bytes",
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
    "NBNS":     [],
    "Split-DNS-Domains":        [],
    "Routes":   [],
    "Routes":  "defaultroute",
    "No-routes":        [],
    "iRoutes":  [],
    "Restricted to routes":  "False",
    "Restricted to ports":      []
  },

  {
    "ID":  3488123,
    "Username":  "maxxie",
    "Groupname":  "(none)",
    "State":  "connected",
    "vhost":  "default",
    "Device":  "vpns1",
    "MTU":  "1284",
    "Remote IP":  "94.204.188.26",
    "Location":  "unknown",
    "Local Device IP":  "188.68.41.144",
    "IPv4":  "10.10.10.228",
    "P-t-P IPv4":  "10.10.10.1",
    "User-Agent":  "AnyConnect Android 5.1.15.295",
    "RX":  "31683792",
    "TX":  "994919220",
    "_RX":  "31.7 MB",
    "_TX":  "994.9 MB",
    "Average RX":  "350 bytes/s",
    "Average TX":  "11.0 kB/s",
    "DPD":  "90",
    "KeepAlive":  "32400",
    "Hostname":  "S10",
    "Connected at":  "2026-05-08 15:07",
    "_Connected at":  "25h:08m",
    "raw_connected_at":  1778245667,
    "Full session":  "fySBxdqjHqDbKTT4g/IoG4H0VYY=",
    "Session":  "fySBxd",
    "TLS ciphersuite":  "(TLS1.3)-(ECDHE-SECP256R1)-(ECDSA-SECP256R1-SHA256)-(AES-128-GC",
    "DTLS cipher":  "(DTLS1.2)-(ECDHE-RSA)-(AES-128-GCM)",
    "DNS":      ["1.1.1.1", "9.9.9.9"],
    "NBNS":     [],
    "Split-DNS-Domains":        [],
    "Routes":   [],
    "Routes":  "defaultroute",
    "No-routes":        [],
    "iRoutes":  [],
    "Restricted to routes":  "False",
    "Restricted to ports":      []
  }
]`)

	users, err := sessions.UsersJSON(input)
	if err != nil {
		t.Fatalf("UsersJSON: %v", err)
	}
	if len(users) != 2 {
		t.Fatalf("count: got %d want 2", len(users))
	}

	// --- User 0: comprehensive field mapping ---
	u0 := users[0]
	assertEqual(t, "u0.ID", u0.ID, "3549163")
	assertEqual(t, "u0.Username", u0.Username, "maxxie")
	assertEqual(t, "u0.Groupname", u0.Groupname, "(none)")
	assertEqual(t, "u0.Status", u0.Status, "connected")
	assertEqual(t, "u0.Vhost", u0.Vhost, "default")
	assertEqual(t, "u0.Device", u0.Device, "vpns0")
	assertEqualInt(t, "u0.MTU", u0.MTU, 1292)
	assertEqual(t, "u0.RemoteIP", u0.RemoteIP, "5.166.153.114")
	assertEqual(t, "u0.Location", u0.Location, "unknown")
	assertEqual(t, "u0.LocalDeviceIP", u0.LocalDeviceIP, "188.68.41.144")
	assertEqual(t, "u0.VpnIP", u0.VpnIP, "10.10.10.208")
	assertEqual(t, "u0.PtPIPv4", u0.PtPIPv4, "10.10.10.1")
	assertEqual(t, "u0.UserAgent", u0.UserAgent, "Open AnyConnect VPN Agent 9.12-3")
	assertEqualInt64(t, "u0.RxBytes", u0.RxBytes, 304)
	assertEqualInt64(t, "u0.TxBytes", u0.TxBytes, 304)
	assertEqual(t, "u0.Hostname", u0.Hostname, "LemonGrass")
	assertEqual(t, "u0.Since", u0.Since, "3h:52m")
	assertEqualDuration(t, "u0.Duration", u0.Duration, 3*time.Hour+52*time.Minute)
	assertEqual(t, "u0.SessionID", u0.SessionID, "YwDQr5")
	assertEqual(t, "u0.FullSession", u0.FullSession, "YwDQr54wGdgKCAju+hgsYQcJLc0=")
	assertEqual(t, "u0.TLSCipher", u0.TLSCipher, "(TLS1.3)-(ECDHE-SECP256R1)-(ECDSA-SECP256R1-SHA256)-(CHACHA20-P")
	assertEqual(t, "u0.DTLSCipher", u0.DTLSCipher, "(DTLS1.2)-(PSK)-(CHACHA20-POLY1305)")

	// ConnectedAt from raw_connected_at (unix timestamp).
	expectedConnAt0 := time.Unix(1778322239, 0)
	if !u0.ConnectedAt.Equal(expectedConnAt0) {
		t.Errorf("u0.ConnectedAt: got %v, want %v", u0.ConnectedAt, expectedConnAt0)
	}

	// --- User 1: validate field mapping ---
	u1 := users[1]
	assertEqual(t, "u1.ID", u1.ID, "3488123")
	assertEqual(t, "u1.Username", u1.Username, "maxxie")
	assertEqual(t, "u1.Groupname", u1.Groupname, "(none)")
	assertEqual(t, "u1.Status", u1.Status, "connected")
	assertEqual(t, "u1.Device", u1.Device, "vpns1")
	assertEqualInt(t, "u1.MTU", u1.MTU, 1284)
	assertEqual(t, "u1.RemoteIP", u1.RemoteIP, "94.204.188.26")
	assertEqual(t, "u1.Location", u1.Location, "unknown")
	assertEqual(t, "u1.LocalDeviceIP", u1.LocalDeviceIP, "188.68.41.144")
	assertEqual(t, "u1.VpnIP", u1.VpnIP, "10.10.10.228")
	assertEqual(t, "u1.PtPIPv4", u1.PtPIPv4, "10.10.10.1")
	assertEqual(t, "u1.UserAgent", u1.UserAgent, "AnyConnect Android 5.1.15.295")
	assertEqualInt64(t, "u1.RxBytes", u1.RxBytes, 31683792)
	assertEqualInt64(t, "u1.TxBytes", u1.TxBytes, 994919220)
	assertEqual(t, "u1.Hostname", u1.Hostname, "S10")
	assertEqual(t, "u1.SessionID", u1.SessionID, "fySBxd")
	assertEqual(t, "u1.FullSession", u1.FullSession, "fySBxdqjHqDbKTT4g/IoG4H0VYY=")
	assertEqual(t, "u1.TLSCipher", u1.TLSCipher, "(TLS1.3)-(ECDHE-SECP256R1)-(ECDSA-SECP256R1-SHA256)-(AES-128-GC")
	assertEqual(t, "u1.DTLSCipher", u1.DTLSCipher, "(DTLS1.2)-(ECDHE-RSA)-(AES-128-GCM)")

	expectedConnAt1 := time.Unix(1778245667, 0)
	if !u1.ConnectedAt.Equal(expectedConnAt1) {
		t.Errorf("u1.ConnectedAt: got %v, want %v", u1.ConnectedAt, expectedConnAt1)
	}
}

// ---------------------------------------------------------------------------
// show user <name> (UserDetailJSON)
// ---------------------------------------------------------------------------

func TestReferenceUserDetailJSON(t *testing.T) {
	t.Parallel()
	input := []byte(`[
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
    "_RX":  "304 bytes",
    "_TX":  "304 bytes",
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
    "NBNS":     [],
    "Split-DNS-Domains":        [],
    "Routes":   [],
    "Routes":  "defaultroute",
    "No-routes":        [],
    "iRoutes":  [],
    "Restricted to routes":  "False",
    "Restricted to ports":      []
  },

  {
    "ID":  3488123,
    "Username":  "maxxie",
    "Groupname":  "(none)",
    "State":  "connected",
    "vhost":  "default",
    "Device":  "vpns1",
    "MTU":  "1284",
    "Remote IP":  "94.204.188.26",
    "Location":  "unknown",
    "Local Device IP":  "188.68.41.144",
    "IPv4":  "10.10.10.228",
    "P-t-P IPv4":  "10.10.10.1",
    "User-Agent":  "AnyConnect Android 5.1.15.295",
    "RX":  "31700355",
    "TX":  "995051116",
    "_RX":  "31.7 MB",
    "_TX":  "995.1 MB",
    "Average RX":  "350 bytes/s",
    "Average TX":  "11.0 kB/s",
    "DPD":  "90",
    "KeepAlive":  "32400",
    "Hostname":  "S10",
    "Connected at":  "2026-05-08 15:07",
    "_Connected at":  "25h:09m",
    "raw_connected_at":  1778245667,
    "Full session":  "fySBxdqjHqDbKTT4g/IoG4H0VYY=",
    "Session":  "fySBxd",
    "TLS ciphersuite":  "(TLS1.3)-(ECDHE-SECP256R1)-(ECDSA-SECP256R1-SHA256)-(AES-128-GC",
    "DTLS cipher":  "(DTLS1.2)-(ECDHE-RSA)-(AES-128-GCM)",
    "DNS":      ["1.1.1.1", "9.9.9.9"],
    "NBNS":     [],
    "Split-DNS-Domains":        [],
    "Routes":   [],
    "Routes":  "defaultroute",
    "No-routes":        [],
    "iRoutes":  [],
    "Restricted to routes":  "False",
    "Restricted to ports":      []
  }
]`)

	conns, err := sessions.UserDetailJSON(input)
	if err != nil {
		t.Fatalf("UserDetailJSON: %v", err)
	}
	if len(conns) != 2 {
		t.Fatalf("count: got %d want 2", len(conns))
	}

	// --- Connection 0: full field mapping ---
	c0 := conns[0]
	assertEqual(t, "c0.ID", c0.ID, "3549163")
	assertEqual(t, "c0.Username", c0.Username, "maxxie")
	assertEqual(t, "c0.Groupname", c0.Groupname, "") // "(none)" is normalized to empty
	assertEqual(t, "c0.State", c0.State, "connected")
	assertEqual(t, "c0.Vhost", c0.Vhost, "default")
	assertEqual(t, "c0.Device", c0.Device, "vpns0")
	assertEqualInt(t, "c0.MTU", c0.MTU, 1292)
	assertEqual(t, "c0.RemoteIP", c0.RemoteIP, "5.166.153.114")
	assertEqual(t, "c0.Location", c0.Location, "unknown")
	assertEqual(t, "c0.LocalDeviceIP", c0.LocalDeviceIP, "188.68.41.144")
	assertEqual(t, "c0.IPv4", c0.IPv4, "10.10.10.208")
	assertEqual(t, "c0.PtPIPv4", c0.PtPIPv4, "10.10.10.1")
	assertEqual(t, "c0.UserAgent", c0.UserAgent, "Open AnyConnect VPN Agent 9.12-3")
	assertEqualInt64(t, "c0.RxBytes", c0.RxBytes, 304)
	assertEqualInt64(t, "c0.TxBytes", c0.TxBytes, 304)
	assertEqual(t, "c0.AvgRxBandwidth", c0.AvgRxBandwidth, "0 bytes/s")
	assertEqual(t, "c0.AvgTxBandwidth", c0.AvgTxBandwidth, "0 bytes/s")
	assertEqualInt(t, "c0.DPD", c0.DPD, 90)
	assertEqualInt(t, "c0.KeepAlive", c0.KeepAlive, 32400)
	assertEqual(t, "c0.Hostname", c0.Hostname, "LemonGrass")
	assertEqual(t, "c0.SessionID", c0.SessionID, "YwDQr5")
	assertEqual(t, "c0.TLSCipher", c0.TLSCipher, "(TLS1.3)-(ECDHE-SECP256R1)-(ECDSA-SECP256R1-SHA256)-(CHACHA20-P")
	assertEqual(t, "c0.DTLSCipher", c0.DTLSCipher, "(DTLS1.2)-(PSK)-(CHACHA20-POLY1305)")
	assertBool(t, "c0.RestrictedToRoutes", c0.RestrictedToRoutes, false)

	// ConnectedAt should be populated from raw_connected_at.
	expectedConnAt0 := time.Unix(1778322239, 0)
	if !c0.ConnectedAt.Equal(expectedConnAt0) {
		t.Errorf("c0.ConnectedAt: got %v, want %v", c0.ConnectedAt, expectedConnAt0)
	}

	// DNS should have been parsed.
	if len(c0.DNS) != 2 || c0.DNS[0] != "1.1.1.1" || c0.DNS[1] != "9.9.9.9" {
		t.Errorf("c0.DNS: got %v, want [1.1.1.1 9.9.9.9]", c0.DNS)
	}

	// Routes: Go's encoding/json keeps the last value for duplicate keys,
	// so "Routes" should be "defaultroute" (parsed as single-element slice).
	if len(c0.Routes) != 1 || c0.Routes[0] != "defaultroute" {
		t.Errorf("c0.Routes: got %v, want [defaultroute]", c0.Routes)
	}

	// --- Connection 1 ---
	c1 := conns[1]
	assertEqual(t, "c1.ID", c1.ID, "3488123")
	assertEqual(t, "c1.Device", c1.Device, "vpns1")
	assertEqualInt(t, "c1.MTU", c1.MTU, 1284)
	assertEqual(t, "c1.UserAgent", c1.UserAgent, "AnyConnect Android 5.1.15.295")
	assertEqualInt64(t, "c1.RxBytes", c1.RxBytes, 31700355)
	assertEqualInt64(t, "c1.TxBytes", c1.TxBytes, 995051116)
	assertEqual(t, "c1.Hostname", c1.Hostname, "S10")
	assertEqual(t, "c1.SessionID", c1.SessionID, "fySBxd")
	assertEqual(t, "c1.DTLSCipher", c1.DTLSCipher, "(DTLS1.2)-(ECDHE-RSA)-(AES-128-GCM)")

	expectedConnAt1 := time.Unix(1778245667, 0)
	if !c1.ConnectedAt.Equal(expectedConnAt1) {
		t.Errorf("c1.ConnectedAt: got %v, want %v", c1.ConnectedAt, expectedConnAt1)
	}
}

// ---------------------------------------------------------------------------
// Duplicate Routes key behavior
// ---------------------------------------------------------------------------

func TestReferenceDuplicateRoutesKey(t *testing.T) {
	t.Parallel()
	input := []byte(`{"Routes": [], "Routes": "defaultroute"}`)
	var m map[string]json.RawMessage
	if err := json.Unmarshal(input, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	// Go's encoding/json keeps the last value for duplicate keys.
	var routeStr string
	var routeArr []string
	strErr := json.Unmarshal(m["Routes"], &routeStr)
	arrErr := json.Unmarshal(m["Routes"], &routeArr)
	if strErr != nil && arrErr != nil {
		t.Error("Routes value is neither string nor array")
	}
	// The last duplicate wins: "defaultroute" string.
	if strErr == nil && routeStr != "defaultroute" {
		t.Errorf("Routes string value: got %q want 'defaultroute'", routeStr)
	}
}

// ---------------------------------------------------------------------------
// Sessions with SessionIsOpen=false and InUse=false
// ---------------------------------------------------------------------------

func TestSessionsJSON_BooleanFieldsFalse(t *testing.T) {
	t.Parallel()
	input := []byte(`[
  {
    "Session":  "abc123",
    "Full session":  "abc123fullbase64==",
    "Created":  "2026-05-01 08:00",
    "State":  "pre-auth",
    "Username":  "testuser",
    "Groupname":  "",
    "vhost":  "other",
    "User-Agent":  "TestAgent/1.0",
    "Remote IP":  "192.168.1.1",
    "Location":  "local",
    "session_is_open":  0,
    "tls_auth_ok":  1,
    "in_use":  0
  }
]`)

	sessions, err := sessions.SessionsJSON(input)
	if err != nil {
		t.Fatalf("SessionsJSON: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("count: got %d want 1", len(sessions))
	}

	s := sessions[0]
	assertBool(t, "SessionIsOpen", s.SessionIsOpen, false)
	assertBool(t, "InUse", s.InUse, false)
	assertEqual(t, "Location", s.Location, "local")
	assertEqual(t, "FullSession", s.FullSession, "abc123fullbase64==")
}

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

func assertEqual(t *testing.T, field, got, want string) {
	t.Helper()
	if got != want {
		t.Errorf("%s: got %q, want %q", field, got, want)
	}
}

func assertEqualInt(t *testing.T, field string, got, want int) {
	t.Helper()
	if got != want {
		t.Errorf("%s: got %d, want %d", field, got, want)
	}
}

func assertEqualInt64(t *testing.T, field string, got, want int64) {
	t.Helper()
	if got != want {
		t.Errorf("%s: got %d, want %d", field, got, want)
	}
}

func assertEqualDuration(t *testing.T, field string, got, want time.Duration) {
	t.Helper()
	if got != want {
		t.Errorf("%s: got %v, want %v", field, got, want)
	}
}

func assertBool(t *testing.T, field string, got, want bool) {
	t.Helper()
	if got != want {
		t.Errorf("%s: got %v, want %v", field, got, want)
	}
}

func assertStringSlice(t *testing.T, field string, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Errorf("%s: length mismatch: got %d (%v), want %d (%v)", field, len(got), got, len(want), want)
		return
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("%s[%d]: got %q, want %q", field, i, got[i], want[i])
		}
	}
}
