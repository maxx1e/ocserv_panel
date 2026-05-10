package parse_test

import (
	"testing"
	"time"

	"ocserv_panel/backend/internal/ocserv/parse"
)

func TestUserDetailOutput_TwoConnections(t *testing.T) {
	input := `        ID: 3318457
        Username: maxxie   Groupname: (none)
        State: connected
        vhost: default
        Device: vpns2   MTU: 1284
        Remote IP: 94.204.188.26   Location: unknown
        Local Device IP: 188.68.41.144
        IPv4: 10.10.10.228   P-t-P IPv4: 10.10.10.1
        User-Agent: AnyConnect Android 5.1.15.295
        RX: 4359893 (4.4 MB)   TX: 361852318 (361.9 MB)
        Average bandwidth RX: 820 bytes/s  TX: 68.1 kB/s
        DPD: 90   KeepAlive: 32400
        Hostname: S10
        Connected at: 2026-05-07 11:58 ( 1h:28m)
        Session: 8Oh70G
        TLS ciphersuite: (TLS1.3)-(ECDHE-SECP256R1)-(ECDSA-SECP256R1-SHA256)-(AES-128-GC
        DTLS cipher: (DTLS1.2)-(ECDHE-RSA)-(AES-128-GCM)

        DNS: 1.1.1.1
                9.9.9.9
        Routes: defaultroute
        Restricted to routes: False


        ID: 3108847
        Username: maxxie   Groupname: (none)
        State: connected
        vhost: default
        Device: vpns0   MTU: 1292
        Remote IP: 5.166.152.90   Location: unknown
        Local Device IP: 188.68.41.144
        IPv4: 10.10.10.208   P-t-P IPv4: 10.10.10.1
        User-Agent: Open AnyConnect VPN Agent 9.12-3
        RX: 152 (152 bytes)   TX: 304 (304 bytes)
        Average bandwidth RX: 0 bytes/s  TX: 0 bytes/s
        DPD: 90   KeepAlive: 32400
        Hostname: LemonGrass
        Connected at: 2026-05-04 07:30 ( 3days)
        Session: TE4THI
        TLS ciphersuite: (TLS1.3)-(ECDHE-SECP256R1)-(ECDSA-SECP256R1-SHA256)-(CHACHA20-P
        DTLS cipher: (DTLS1.2)-(PSK)-(CHACHA20-POLY1305)

        DNS: 1.1.1.1
                9.9.9.9
        Routes: defaultroute
        Restricted to routes: False`

	conns, err := parse.UserDetailOutput(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(conns) != 2 {
		t.Fatalf("expected 2 connections, got %d", len(conns))
	}

	// Verify first connection
	c1 := conns[0]
	assertEqual(t, "c1.ID", "3318457", c1.ID)
	assertEqual(t, "c1.Username", "maxxie", c1.Username)
	assertEqual(t, "c1.Groupname", "", c1.Groupname)
	assertEqual(t, "c1.State", "connected", c1.State)
	assertEqual(t, "c1.Vhost", "default", c1.Vhost)
	assertEqual(t, "c1.Device", "vpns2", c1.Device)
	assertEqualInt(t, "c1.MTU", 1284, c1.MTU)
	assertEqual(t, "c1.RemoteIP", "94.204.188.26", c1.RemoteIP)
	assertEqual(t, "c1.Location", "unknown", c1.Location)
	assertEqual(t, "c1.LocalDeviceIP", "188.68.41.144", c1.LocalDeviceIP)
	assertEqual(t, "c1.IPv4", "10.10.10.228", c1.IPv4)
	assertEqual(t, "c1.PtPIPv4", "10.10.10.1", c1.PtPIPv4)
	assertEqual(t, "c1.UserAgent", "AnyConnect Android 5.1.15.295", c1.UserAgent)
	assertEqualInt64(t, "c1.RxBytes", 4359893, c1.RxBytes)
	assertEqualInt64(t, "c1.TxBytes", 361852318, c1.TxBytes)
	assertEqual(t, "c1.AvgRxBandwidth", "820 bytes/s", c1.AvgRxBandwidth)
	assertEqual(t, "c1.AvgTxBandwidth", "68.1 kB/s", c1.AvgTxBandwidth)
	assertEqualInt(t, "c1.DPD", 90, c1.DPD)
	assertEqualInt(t, "c1.KeepAlive", 32400, c1.KeepAlive)
	assertEqual(t, "c1.Hostname", "S10", c1.Hostname)
	expectedTime1 := time.Date(2026, 5, 7, 11, 58, 0, 0, time.UTC)
	if !c1.ConnectedAt.Equal(expectedTime1) {
		t.Errorf("c1.ConnectedAt: got %v, want %v", c1.ConnectedAt, expectedTime1)
	}
	assertEqual(t, "c1.SessionID", "8Oh70G", c1.SessionID)
	assertEqual(t, "c1.TLSCipher", "(TLS1.3)-(ECDHE-SECP256R1)-(ECDSA-SECP256R1-SHA256)-(AES-128-GC", c1.TLSCipher)
	assertEqual(t, "c1.DTLSCipher", "(DTLS1.2)-(ECDHE-RSA)-(AES-128-GCM)", c1.DTLSCipher)
	assertStringSlice(t, "c1.DNS", []string{"1.1.1.1", "9.9.9.9"}, c1.DNS)
	assertStringSlice(t, "c1.Routes", []string{"defaultroute"}, c1.Routes)
	if c1.RestrictedToRoutes {
		t.Errorf("c1.RestrictedToRoutes: got true, want false")
	}

	// Verify second connection
	c2 := conns[1]
	assertEqual(t, "c2.ID", "3108847", c2.ID)
	assertEqual(t, "c2.Username", "maxxie", c2.Username)
	assertEqual(t, "c2.Device", "vpns0", c2.Device)
	assertEqualInt(t, "c2.MTU", 1292, c2.MTU)
	assertEqual(t, "c2.RemoteIP", "5.166.152.90", c2.RemoteIP)
	assertEqualInt64(t, "c2.RxBytes", 152, c2.RxBytes)
	assertEqualInt64(t, "c2.TxBytes", 304, c2.TxBytes)
	expectedTime2 := time.Date(2026, 5, 4, 7, 30, 0, 0, time.UTC)
	if !c2.ConnectedAt.Equal(expectedTime2) {
		t.Errorf("c2.ConnectedAt: got %v, want %v", c2.ConnectedAt, expectedTime2)
	}
}

func TestUserDetailOutput_EmptyInput(t *testing.T) {
	conns, err := parse.UserDetailOutput("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(conns) != 0 {
		t.Fatalf("expected 0 connections, got %d", len(conns))
	}
}

func TestUserDetailJSON_RealOutput(t *testing.T) {
	input := []byte(`[
		{
			"ID": 3549163,
			"Username": "maxxie",
			"Groupname": "(none)",
			"State": "connected",
			"vhost": "default",
			"Device": "vpns0",
			"MTU": "1292",
			"Remote IP": "5.166.153.114",
			"Location": "unknown",
			"Local Device IP": "188.68.41.144",
			"IPv4": "10.10.10.208",
			"P-t-P IPv4": "10.10.10.1",
			"User-Agent": "Open AnyConnect VPN Agent 9.12-3",
			"RX": "304",
			"TX": "304",
			"_RX": "304 bytes",
			"_TX": "304 bytes",
			"Average RX": "0 bytes/s",
			"Average TX": "0 bytes/s",
			"DPD": "90",
			"KeepAlive": "32400",
			"Hostname": "LemonGrass",
			"Connected at": "2026-05-09 12:23",
			"_Connected at": " 3h:52m",
			"raw_connected_at": 1778322239,
			"Full session": "YwDQr54wGdgKCAju+hgsYQcJLc0=",
			"Session": "YwDQr5",
			"TLS ciphersuite": "(TLS1.3)-(ECDHE-SECP256R1)-(ECDSA-SECP256R1-SHA256)-(CHACHA20-P",
			"DTLS cipher": "(DTLS1.2)-(PSK)-(CHACHA20-POLY1305)",
			"DNS": ["1.1.1.1", "9.9.9.9"],
			"Routes": "defaultroute",
			"Restricted to routes": "False"
		}
	]`)

	conns, err := parse.UserDetailJSON(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(conns) != 1 {
		t.Fatalf("expected 1 connection, got %d", len(conns))
	}

	c := conns[0]
	assertEqual(t, "ID", "3549163", c.ID)
	assertEqual(t, "Username", "maxxie", c.Username)
	assertEqual(t, "Groupname", "", c.Groupname)
	assertEqual(t, "State", "connected", c.State)
	assertEqual(t, "Device", "vpns0", c.Device)
	assertEqualInt(t, "MTU", 1292, c.MTU)
	assertEqual(t, "RemoteIP", "5.166.153.114", c.RemoteIP)
	assertEqualInt64(t, "RxBytes", 304, c.RxBytes)
	assertEqualInt64(t, "TxBytes", 304, c.TxBytes)
	assertEqual(t, "SessionID", "YwDQr5", c.SessionID)
	assertStringSlice(t, "DNS", []string{"1.1.1.1", "9.9.9.9"}, c.DNS)
	assertStringSlice(t, "Routes", []string{"defaultroute"}, c.Routes)
	if c.RestrictedToRoutes {
		t.Errorf("RestrictedToRoutes: got true, want false")
	}
	if c.ConnectedAt.IsZero() {
		t.Error("ConnectedAt should not be zero")
	}
}

func TestUserDetailJSON_EmptyArray(t *testing.T) {
	input := []byte(`[]`)
	conns, err := parse.UserDetailJSON(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(conns) != 0 {
		t.Fatalf("expected 0 connections, got %d", len(conns))
	}
}

func TestUserDetailJSON_InvalidJSON(t *testing.T) {
	input := []byte(`not json`)
	_, err := parse.UserDetailJSON(input)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

// Helper functions for assertions.

func assertEqual(t *testing.T, field, want, got string) {
	t.Helper()
	if got != want {
		t.Errorf("%s: got %q, want %q", field, got, want)
	}
}

func assertEqualInt(t *testing.T, field string, want, got int) {
	t.Helper()
	if got != want {
		t.Errorf("%s: got %d, want %d", field, got, want)
	}
}

func assertEqualInt64(t *testing.T, field string, want, got int64) {
	t.Helper()
	if got != want {
		t.Errorf("%s: got %d, want %d", field, got, want)
	}
}

func assertStringSlice(t *testing.T, field string, want, got []string) {
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
