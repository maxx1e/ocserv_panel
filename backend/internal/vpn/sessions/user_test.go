package sessions_test

import (
	"testing"
	"time"

	"ocserv_panel/backend/internal/vpn/sessions"
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

	conns, err := sessions.UserDetailOutput(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(conns) != 2 {
		t.Fatalf("expected 2 connections, got %d", len(conns))
	}

	// Verify first connection
	c1 := conns[0]
	assertEqual(t, "c1.ID", c1.ID, "3318457")
	assertEqual(t, "c1.Username", c1.Username, "maxxie")
	assertEqual(t, "c1.Groupname", c1.Groupname, "")
	assertEqual(t, "c1.State", c1.State, "connected")
	assertEqual(t, "c1.Vhost", c1.Vhost, "default")
	assertEqual(t, "c1.Device", c1.Device, "vpns2")
	assertEqualInt(t, "c1.MTU", c1.MTU, 1284)
	assertEqual(t, "c1.RemoteIP", c1.RemoteIP, "94.204.188.26")
	assertEqual(t, "c1.Location", c1.Location, "unknown")
	assertEqual(t, "c1.LocalDeviceIP", c1.LocalDeviceIP, "188.68.41.144")
	assertEqual(t, "c1.IPv4", c1.IPv4, "10.10.10.228")
	assertEqual(t, "c1.PtPIPv4", c1.PtPIPv4, "10.10.10.1")
	assertEqual(t, "c1.UserAgent", c1.UserAgent, "AnyConnect Android 5.1.15.295")
	assertEqualInt64(t, "c1.RxBytes", c1.RxBytes, 4359893)
	assertEqualInt64(t, "c1.TxBytes", c1.TxBytes, 361852318)
	assertEqual(t, "c1.AvgRxBandwidth", c1.AvgRxBandwidth, "820 bytes/s")
	assertEqual(t, "c1.AvgTxBandwidth", c1.AvgTxBandwidth, "68.1 kB/s")
	assertEqualInt(t, "c1.DPD", c1.DPD, 90)
	assertEqualInt(t, "c1.KeepAlive", c1.KeepAlive, 32400)
	assertEqual(t, "c1.Hostname", c1.Hostname, "S10")
	expectedTime1 := time.Date(2026, 5, 7, 11, 58, 0, 0, time.UTC)
	if !c1.ConnectedAt.Equal(expectedTime1) {
		t.Errorf("c1.ConnectedAt: got %v, want %v", c1.ConnectedAt, expectedTime1)
	}
	assertEqual(t, "c1.SessionID", c1.SessionID, "8Oh70G")
	assertEqual(t, "c1.TLSCipher", c1.TLSCipher, "(TLS1.3)-(ECDHE-SECP256R1)-(ECDSA-SECP256R1-SHA256)-(AES-128-GC")
	assertEqual(t, "c1.DTLSCipher", c1.DTLSCipher, "(DTLS1.2)-(ECDHE-RSA)-(AES-128-GCM)")
	assertStringSlice(t, "c1.DNS", c1.DNS, []string{"1.1.1.1", "9.9.9.9"})
	assertStringSlice(t, "c1.Routes", c1.Routes, []string{"defaultroute"})
	if c1.RestrictedToRoutes {
		t.Errorf("c1.RestrictedToRoutes: got true, want false")
	}

	// Verify second connection
	c2 := conns[1]
	assertEqual(t, "c2.ID", c2.ID, "3108847")
	assertEqual(t, "c2.Username", c2.Username, "maxxie")
	assertEqual(t, "c2.Device", c2.Device, "vpns0")
	assertEqualInt(t, "c2.MTU", c2.MTU, 1292)
	assertEqual(t, "c2.RemoteIP", c2.RemoteIP, "5.166.152.90")
	assertEqualInt64(t, "c2.RxBytes", c2.RxBytes, 152)
	assertEqualInt64(t, "c2.TxBytes", c2.TxBytes, 304)
	expectedTime2 := time.Date(2026, 5, 4, 7, 30, 0, 0, time.UTC)
	if !c2.ConnectedAt.Equal(expectedTime2) {
		t.Errorf("c2.ConnectedAt: got %v, want %v", c2.ConnectedAt, expectedTime2)
	}
}

func TestUserDetailOutput_EmptyInput(t *testing.T) {
	conns, err := sessions.UserDetailOutput("")
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

	conns, err := sessions.UserDetailJSON(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(conns) != 1 {
		t.Fatalf("expected 1 connection, got %d", len(conns))
	}

	c := conns[0]
	assertEqual(t, "ID", c.ID, "3549163")
	assertEqual(t, "Username", c.Username, "maxxie")
	assertEqual(t, "Groupname", c.Groupname, "")
	assertEqual(t, "State", c.State, "connected")
	assertEqual(t, "Device", c.Device, "vpns0")
	assertEqualInt(t, "MTU", c.MTU, 1292)
	assertEqual(t, "RemoteIP", c.RemoteIP, "5.166.153.114")
	assertEqualInt64(t, "RxBytes", c.RxBytes, 304)
	assertEqualInt64(t, "TxBytes", c.TxBytes, 304)
	assertEqual(t, "SessionID", c.SessionID, "YwDQr5")
	assertStringSlice(t, "DNS", c.DNS, []string{"1.1.1.1", "9.9.9.9"})
	assertStringSlice(t, "Routes", c.Routes, []string{"defaultroute"})
	if c.RestrictedToRoutes {
		t.Errorf("RestrictedToRoutes: got true, want false")
	}
	if c.ConnectedAt.IsZero() {
		t.Error("ConnectedAt should not be zero")
	}
}

func TestUserDetailJSON_EmptyArray(t *testing.T) {
	input := []byte(`[]`)
	conns, err := sessions.UserDetailJSON(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(conns) != 0 {
		t.Fatalf("expected 0 connections, got %d", len(conns))
	}
}

func TestUserDetailJSON_InvalidJSON(t *testing.T) {
	input := []byte(`not json`)
	_, err := sessions.UserDetailJSON(input)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}
