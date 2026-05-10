package parse_test

import (
	"testing"
	"time"

	"ocserv_panel/backend/internal/ocserv/parse"
)

func TestUsersJSON_ValidArray(t *testing.T) {
	input := []byte(`[
		{
			"ID": 3318457,
			"Username": "maxxie",
			"Groupname": "(none)",
			"State": "connected",
			"vhost": "default",
			"Device": "vpns2",
			"MTU": "1284",
			"Remote IP": "94.204.188.26",
			"Location": "unknown",
			"Local Device IP": "188.68.41.144",
			"IPv4": "10.10.10.228",
			"P-t-P IPv4": "10.10.10.1",
			"User-Agent": "AnyConnect Android 5.1.15.295",
			"RX": "4359893",
			"TX": "361852318",
			"Average RX": "820 bytes/s",
			"Average TX": "68.1 kB/s",
			"DPD": "90",
			"KeepAlive": "32400",
			"Hostname": "S10",
			"Connected at": "2026-05-07 11:58",
			"_Connected at": " 1h:27m",
			"raw_connected_at": 1778349480,
			"Full session": "8Oh70Ggl2oChT0K4e/5oC+1G/zk=",
			"Session": "8Oh70G",
			"TLS ciphersuite": "(TLS1.3)-(ECDHE-SECP256R1)",
			"DTLS cipher": "(AES-128-GCM)",
			"DNS": ["1.1.1.1", "9.9.9.9"],
			"Routes": "defaultroute",
			"Restricted to routes": "False"
		},
		{
			"ID": 3108847,
			"Username": "maxxie",
			"Groupname": "(none)",
			"State": "connected",
			"vhost": "default",
			"Device": "vpns0",
			"MTU": "1292",
			"Remote IP": "5.166.152.90",
			"Location": "unknown",
			"Local Device IP": "188.68.41.144",
			"IPv4": "10.10.10.208",
			"P-t-P IPv4": "10.10.10.1",
			"User-Agent": "Open AnyConnect VPN Agent 9.12-3",
			"RX": "152",
			"TX": "304",
			"Average RX": "0 bytes/s",
			"Average TX": "0 bytes/s",
			"DPD": "90",
			"KeepAlive": "32400",
			"Hostname": "LemonGrass",
			"Connected at": "2026-05-04 07:30",
			"_Connected at": " 3days",
			"raw_connected_at": 1778056200,
			"Full session": "TE4THIxyz",
			"Session": "TE4THI",
			"TLS ciphersuite": "(TLS1.3)-(CHACHA20-P",
			"DTLS cipher": "(CHACHA20-POLY1305)",
			"DNS": ["1.1.1.1", "9.9.9.9"],
			"Routes": "defaultroute",
			"Restricted to routes": "False"
		}
	]`)

	users, err := parse.UsersJSON(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(users) != 2 {
		t.Fatalf("expected 2 users, got %d", len(users))
	}

	u1 := users[0]
	if u1.ID != "3318457" {
		t.Errorf("user[0].ID = %q, want %q", u1.ID, "3318457")
	}
	if u1.Username != "maxxie" {
		t.Errorf("user[0].Username = %q, want %q", u1.Username, "maxxie")
	}
	if u1.Vhost != "default" {
		t.Errorf("user[0].Vhost = %q, want %q", u1.Vhost, "default")
	}
	if u1.RemoteIP != "94.204.188.26" {
		t.Errorf("user[0].RemoteIP = %q, want %q", u1.RemoteIP, "94.204.188.26")
	}
	if u1.VpnIP != "10.10.10.228" {
		t.Errorf("user[0].VpnIP = %q, want %q", u1.VpnIP, "10.10.10.228")
	}
	if u1.Device != "vpns2" {
		t.Errorf("user[0].Device = %q, want %q", u1.Device, "vpns2")
	}
	if u1.Since != "1h:27m" {
		t.Errorf("user[0].Since = %q, want %q", u1.Since, "1h:27m")
	}
	if u1.Duration != 1*time.Hour+27*time.Minute {
		t.Errorf("user[0].Duration = %v, want %v", u1.Duration, 1*time.Hour+27*time.Minute)
	}
	if u1.DTLSCipher != "(AES-128-GCM)" {
		t.Errorf("user[0].DTLSCipher = %q, want %q", u1.DTLSCipher, "(AES-128-GCM)")
	}
	if u1.Status != "connected" {
		t.Errorf("user[0].Status = %q, want %q", u1.Status, "connected")
	}

	u2 := users[1]
	if u2.ID != "3108847" {
		t.Errorf("user[1].ID = %q, want %q", u2.ID, "3108847")
	}
	if u2.Duration != 72*time.Hour {
		t.Errorf("user[1].Duration = %v, want %v", u2.Duration, 72*time.Hour)
	}
	if u2.DTLSCipher != "(CHACHA20-POLY1305)" {
		t.Errorf("user[1].DTLSCipher = %q, want %q", u2.DTLSCipher, "(CHACHA20-POLY1305)")
	}
}

func TestUsersJSON_EmptyArray(t *testing.T) {
	input := []byte(`[]`)

	users, err := parse.UsersJSON(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(users) != 0 {
		t.Fatalf("expected 0 users, got %d", len(users))
	}
}

func TestUsersJSON_MalformedJSON(t *testing.T) {
	input := []byte(`{not valid json`)

	_, err := parse.UsersJSON(input)
	if err == nil {
		t.Fatal("expected error for malformed JSON, got nil")
	}
}

func TestUsersText_RealSample(t *testing.T) {
	text := `      id     user    vhost             ip         vpn-ip device   since    dtls-cipher    status
 3318457   maxxie  default  94.204.188.26   10.10.10.228  vpns2  1h:27m  (AES-128-GCM) connected
 3108847   maxxie  default   5.166.152.90   10.10.10.208  vpns0  3days (CHACHA20-POLY1305) connected`

	users, err := parse.UsersText(text)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(users) != 2 {
		t.Fatalf("expected 2 users, got %d", len(users))
	}

	u1 := users[0]
	if u1.ID != "3318457" {
		t.Errorf("user[0].ID = %q, want %q", u1.ID, "3318457")
	}
	if u1.Username != "maxxie" {
		t.Errorf("user[0].Username = %q, want %q", u1.Username, "maxxie")
	}
	if u1.Vhost != "default" {
		t.Errorf("user[0].Vhost = %q, want %q", u1.Vhost, "default")
	}
	if u1.RemoteIP != "94.204.188.26" {
		t.Errorf("user[0].RemoteIP = %q, want %q", u1.RemoteIP, "94.204.188.26")
	}
	if u1.VpnIP != "10.10.10.228" {
		t.Errorf("user[0].VpnIP = %q, want %q", u1.VpnIP, "10.10.10.228")
	}
	if u1.Device != "vpns2" {
		t.Errorf("user[0].Device = %q, want %q", u1.Device, "vpns2")
	}
	if u1.Since != "1h:27m" {
		t.Errorf("user[0].Since = %q, want %q", u1.Since, "1h:27m")
	}
	if u1.Duration != 1*time.Hour+27*time.Minute {
		t.Errorf("user[0].Duration = %v, want %v", u1.Duration, 1*time.Hour+27*time.Minute)
	}
	if u1.DTLSCipher != "(AES-128-GCM)" {
		t.Errorf("user[0].DTLSCipher = %q, want %q", u1.DTLSCipher, "(AES-128-GCM)")
	}
	if u1.Status != "connected" {
		t.Errorf("user[0].Status = %q, want %q", u1.Status, "connected")
	}

	u2 := users[1]
	if u2.ID != "3108847" {
		t.Errorf("user[1].ID = %q, want %q", u2.ID, "3108847")
	}
	if u2.RemoteIP != "5.166.152.90" {
		t.Errorf("user[1].RemoteIP = %q, want %q", u2.RemoteIP, "5.166.152.90")
	}
	if u2.VpnIP != "10.10.10.208" {
		t.Errorf("user[1].VpnIP = %q, want %q", u2.VpnIP, "10.10.10.208")
	}
	if u2.Device != "vpns0" {
		t.Errorf("user[1].Device = %q, want %q", u2.Device, "vpns0")
	}
	if u2.Since != "3days" {
		t.Errorf("user[1].Since = %q, want %q", u2.Since, "3days")
	}
	if u2.Duration != 72*time.Hour {
		t.Errorf("user[1].Duration = %v, want %v", u2.Duration, 72*time.Hour)
	}
	if u2.DTLSCipher != "(CHACHA20-POLY1305)" {
		t.Errorf("user[1].DTLSCipher = %q, want %q", u2.DTLSCipher, "(CHACHA20-POLY1305)")
	}
	if u2.Status != "connected" {
		t.Errorf("user[1].Status = %q, want %q", u2.Status, "connected")
	}
}

func TestUsersText_EmptyOutput(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"empty string", ""},
		{"header only", "      id     user    vhost             ip         vpn-ip device   since    dtls-cipher    status\n"},
		{"whitespace only", "   \n   \n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			users, err := parse.UsersText(tt.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(users) != 0 {
				t.Fatalf("expected 0 users, got %d", len(users))
			}
		})
	}
}

func TestUsersText_SingleUser(t *testing.T) {
	text := `      id     user    vhost             ip         vpn-ip device   since    dtls-cipher    status
 1234567   alice  default  192.168.1.100   10.10.10.5  vpns1  45m  (AES-256-GCM) connected`

	users, err := parse.UsersText(text)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(users) != 1 {
		t.Fatalf("expected 1 user, got %d", len(users))
	}

	u := users[0]
	if u.ID != "1234567" {
		t.Errorf("ID = %q, want %q", u.ID, "1234567")
	}
	if u.Username != "alice" {
		t.Errorf("Username = %q, want %q", u.Username, "alice")
	}
	if u.RemoteIP != "192.168.1.100" {
		t.Errorf("RemoteIP = %q, want %q", u.RemoteIP, "192.168.1.100")
	}
	if u.VpnIP != "10.10.10.5" {
		t.Errorf("VpnIP = %q, want %q", u.VpnIP, "10.10.10.5")
	}
	if u.Duration != 45*time.Minute {
		t.Errorf("Duration = %v, want %v", u.Duration, 45*time.Minute)
	}
	if u.DTLSCipher != "(AES-256-GCM)" {
		t.Errorf("DTLSCipher = %q, want %q", u.DTLSCipher, "(AES-256-GCM)")
	}
	if u.Status != "connected" {
		t.Errorf("Status = %q, want %q", u.Status, "connected")
	}
}

func TestDurationString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected time.Duration
	}{
		{"hours and minutes", "1h:27m", 1*time.Hour + 27*time.Minute},
		{"days", "3days", 72 * time.Hour},
		{"minutes only", "45m", 45 * time.Minute},
		{"seconds only", "30s", 30 * time.Second},
		{"zero seconds", "0s", 0},
		{"hours with zero minutes", "2h:00m", 2 * time.Hour},
		{"hours minutes seconds", "1h:30m:45s", 1*time.Hour + 30*time.Minute + 45*time.Second},
		{"empty string", "", 0},
		{"invalid string", "invalid", 0},
		{"single day", "1day", 24 * time.Hour},
		{"whitespace", "  2h:15m  ", 2*time.Hour + 15*time.Minute},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parse.DurationString(tt.input)
			if got != tt.expected {
				t.Errorf("DurationString(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}
