package parse_test

import (
	"testing"
	"time"

	"ocserv_panel/backend/internal/ocserv/parse"
)

func TestSessionsJSONBasic(t *testing.T) {
	t.Parallel()
	input := `[
		{"Session":"8Oh70G","Username":"maxxie","vhost":"default","Remote IP":"94.204.188.26","User-Agent":"AnyConnect Android 5.1.1","Created":"2026-05-05 11:57","State":"authenticated","session_is_open":1},
		{"Session":"TE4THI","Username":"maxxie","vhost":"default","Remote IP":"5.166.152.90","User-Agent":"Open AnyConnect VPN Agent","Created":"2026-05-04 07:30","State":"authenticated","session_is_open":1}
	]`

	sessions, err := parse.SessionsJSON([]byte(input))
	if err != nil {
		t.Fatalf("SessionsJSON: %v", err)
	}
	if len(sessions) != 2 {
		t.Fatalf("expected 2 sessions, got %d", len(sessions))
	}

	if sessions[0].ID != "8Oh70G" {
		t.Errorf("sessions[0].ID: got %q, want %q", sessions[0].ID, "8Oh70G")
	}
	if sessions[0].Username != "maxxie" {
		t.Errorf("sessions[0].Username: got %q, want %q", sessions[0].Username, "maxxie")
	}
	if sessions[0].RemoteAddress != "94.204.188.26" {
		t.Errorf("sessions[0].RemoteAddress: got %q, want %q", sessions[0].RemoteAddress, "94.204.188.26")
	}
	if sessions[0].UserAgent != "AnyConnect Android 5.1.1" {
		t.Errorf("sessions[0].UserAgent: got %q, want %q", sessions[0].UserAgent, "AnyConnect Android 5.1.1")
	}
	if sessions[0].ConnectedAt.IsZero() {
		t.Error("sessions[0].ConnectedAt should not be zero for timestamp '2026-05-05 11:57'")
	}
	// Verify the timestamp was parsed correctly.
	expectedTime := time.Date(2026, 5, 5, 11, 57, 0, 0, time.UTC)
	if !sessions[0].ConnectedAt.Equal(expectedTime) {
		t.Errorf("sessions[0].ConnectedAt: got %v, want %v", sessions[0].ConnectedAt, expectedTime)
	}

	if sessions[1].ID != "TE4THI" {
		t.Errorf("sessions[1].ID: got %q, want %q", sessions[1].ID, "TE4THI")
	}
	if sessions[1].RemoteAddress != "5.166.152.90" {
		t.Errorf("sessions[1].RemoteAddress: got %q, want %q", sessions[1].RemoteAddress, "5.166.152.90")
	}
}

func TestSessionsJSONEmpty(t *testing.T) {
	t.Parallel()
	sessions, err := parse.SessionsJSON([]byte(`[]`))
	if err != nil {
		t.Fatalf("SessionsJSON (empty): %v", err)
	}
	if len(sessions) != 0 {
		t.Fatalf("expected 0 sessions, got %d", len(sessions))
	}
}

func TestSessionsJSONInvalid(t *testing.T) {
	t.Parallel()
	_, err := parse.SessionsJSON([]byte(`not json`))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestSessionsJSONDurationFallback(t *testing.T) {
	t.Parallel()
	// Older occtl versions may produce a duration string in "Created" instead of a timestamp.
	input := `[{"Session":"abc123","Username":"bob","Remote IP":"1.2.3.4","Created":"1h:30m","State":"connected"}]`

	sessions, err := parse.SessionsJSON([]byte(input))
	if err != nil {
		t.Fatalf("SessionsJSON: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}
	if sessions[0].ConnectedAt.IsZero() {
		t.Error("ConnectedAt should not be zero for Created '1h:30m'")
	}
}

func TestSessionsTextBasic(t *testing.T) {
	t.Parallel()
	input := `session     user    vhost             ip               user agent  created   status
8Oh70G   maxxie  default  94.204.188.26 AnyConnect Android 5.1.1    2days authenticated
TE4THI   maxxie  default   5.166.152.90 Open AnyConnect VPN Agen    3days authenticated`

	sessions, err := parse.SessionsText(input)
	if err != nil {
		t.Fatalf("SessionsText: %v", err)
	}
	if len(sessions) != 2 {
		t.Fatalf("expected 2 sessions, got %d", len(sessions))
	}

	if sessions[0].ID != "8Oh70G" {
		t.Errorf("sessions[0].ID: got %q, want %q", sessions[0].ID, "8Oh70G")
	}
	if sessions[0].Username != "maxxie" {
		t.Errorf("sessions[0].Username: got %q, want %q", sessions[0].Username, "maxxie")
	}
	if sessions[0].RemoteAddress != "94.204.188.26" {
		t.Errorf("sessions[0].RemoteAddress: got %q, want %q", sessions[0].RemoteAddress, "94.204.188.26")
	}
	if sessions[0].ConnectedAt.IsZero() {
		t.Error("sessions[0].ConnectedAt should not be zero")
	}

	if sessions[1].ID != "TE4THI" {
		t.Errorf("sessions[1].ID: got %q, want %q", sessions[1].ID, "TE4THI")
	}
	if sessions[1].RemoteAddress != "5.166.152.90" {
		t.Errorf("sessions[1].RemoteAddress: got %q, want %q", sessions[1].RemoteAddress, "5.166.152.90")
	}
}

func TestSessionsTextEmpty(t *testing.T) {
	t.Parallel()
	sessions, err := parse.SessionsText("")
	if err != nil {
		t.Fatalf("SessionsText (empty): %v", err)
	}
	if len(sessions) != 0 {
		t.Fatalf("expected 0 sessions, got %d", len(sessions))
	}
}

func TestSessionsTextHeaderOnly(t *testing.T) {
	t.Parallel()
	input := "session     user    vhost             ip               user agent  created   status\n"
	sessions, err := parse.SessionsText(input)
	if err != nil {
		t.Fatalf("SessionsText (header only): %v", err)
	}
	if len(sessions) != 0 {
		t.Fatalf("expected 0 sessions, got %d", len(sessions))
	}
}

func TestSessionDetailJSONBasic(t *testing.T) {
	t.Parallel()
	input := `{
		"Session": "8Oh70G",
		"Full session": "8Oh70Ggl2oChT0K4e/5oC+1G/zk=",
		"Created": "2026-05-05 11:57",
		"State": "authenticated",
		"Username": "maxxie",
		"Groupname": "(none)",
		"vhost": "default",
		"User-Agent": "AnyConnect Android 5.1.15.295",
		"Remote IP": "94.204.188.26",
		"Location": "unknown",
		"session_is_open": 1,
		"tls_auth_ok": 0,
		"in_use": 1
	}`

	detail, err := parse.SessionDetailJSON([]byte(input))
	if err != nil {
		t.Fatalf("SessionDetailJSON: %v", err)
	}
	if detail.ID != "8Oh70G" {
		t.Errorf("ID: got %q, want %q", detail.ID, "8Oh70G")
	}
	if detail.Username != "maxxie" {
		t.Errorf("Username: got %q, want %q", detail.Username, "maxxie")
	}
	if detail.RemoteAddress != "94.204.188.26" {
		t.Errorf("RemoteAddress: got %q, want %q", detail.RemoteAddress, "94.204.188.26")
	}
	if detail.UserAgent != "AnyConnect Android 5.1.15.295" {
		t.Errorf("UserAgent: got %q, want %q", detail.UserAgent, "AnyConnect Android 5.1.15.295")
	}
	if detail.ConnectedAt.IsZero() {
		t.Error("ConnectedAt should not be zero")
	}
}

func TestSessionDetailJSONWithExtendedFields(t *testing.T) {
	t.Parallel()
	input := `{
		"Session": "s1",
		"Username": "alice",
		"Remote IP": "203.0.113.5",
		"Connected at": "2026-01-15 10:30",
		"User-Agent": "AnyConnect/4.10",
		"Device": "vpns0",
		"IPv4": "10.8.0.2",
		"IPv6": "fd00::2",
		"raw_rx": 1048576,
		"raw_tx": 2097152,
		"DNS": ["8.8.8.8", "1.1.1.1"],
		"Routes": ["10.0.0.0/8", "192.168.0.0/16"]
	}`

	detail, err := parse.SessionDetailJSON([]byte(input))
	if err != nil {
		t.Fatalf("SessionDetailJSON: %v", err)
	}
	if detail.IPv4 != "10.8.0.2" {
		t.Errorf("IPv4: got %q, want %q", detail.IPv4, "10.8.0.2")
	}
	if detail.IPv6 != "fd00::2" {
		t.Errorf("IPv6: got %q, want %q", detail.IPv6, "fd00::2")
	}
	if detail.VPNAddress != "10.8.0.2" {
		t.Errorf("VPNAddress: got %q, want %q", detail.VPNAddress, "10.8.0.2")
	}
	if detail.RxBytes != 1048576 {
		t.Errorf("RxBytes: got %d, want %d", detail.RxBytes, 1048576)
	}
	if detail.TxBytes != 2097152 {
		t.Errorf("TxBytes: got %d, want %d", detail.TxBytes, 2097152)
	}
	if len(detail.DNS) != 2 || detail.DNS[0] != "8.8.8.8" {
		t.Errorf("DNS: got %v", detail.DNS)
	}
	if len(detail.Routes) != 2 || detail.Routes[0] != "10.0.0.0/8" {
		t.Errorf("Routes: got %v", detail.Routes)
	}
	if detail.ConnectedAt.IsZero() {
		t.Error("ConnectedAt should not be zero")
	}
}

func TestSessionDetailJSONInvalid(t *testing.T) {
	t.Parallel()
	_, err := parse.SessionDetailJSON([]byte(`not json`))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestSessionDetailJSONArrayFormat(t *testing.T) {
	t.Parallel()
	// Actual occtl output wraps session detail in an array.
	input := `[
		{
			"Session": "YwDQr5",
			"Full session": "YwDQr54wGdgKCAju+hgsYQcJLc0=",
			"Created": "2026-05-09 12:24",
			"State": "authenticated",
			"Username": "maxxie",
			"Groupname": "(none)",
			"vhost": "default",
			"User-Agent": "Open AnyConnect VPN Agent 9.12-3",
			"Remote IP": "5.166.153.114",
			"Location": "unknown",
			"session_is_open": 1,
			"tls_auth_ok": 0,
			"in_use": 1
		}
	]`

	detail, err := parse.SessionDetailJSON([]byte(input))
	if err != nil {
		t.Fatalf("SessionDetailJSON (array): %v", err)
	}
	if detail.ID != "YwDQr5" {
		t.Errorf("ID: got %q, want %q", detail.ID, "YwDQr5")
	}
	if detail.Username != "maxxie" {
		t.Errorf("Username: got %q, want %q", detail.Username, "maxxie")
	}
	if detail.RemoteAddress != "5.166.153.114" {
		t.Errorf("RemoteAddress: got %q, want %q", detail.RemoteAddress, "5.166.153.114")
	}
	if detail.UserAgent != "Open AnyConnect VPN Agent 9.12-3" {
		t.Errorf("UserAgent: got %q, want %q", detail.UserAgent, "Open AnyConnect VPN Agent 9.12-3")
	}
	if detail.ConnectedAt.IsZero() {
		t.Error("ConnectedAt should not be zero")
	}
}

func TestSessionDetailTextBasic(t *testing.T) {
	t.Parallel()
	input := `Session: 8Oh70G
        Full session ID: 8Oh70Ggl2oChT0K4e/5oC+1G/zk=
        Created: 2026-05-05 11:57
        State: authenticated
        Username: maxxie   Groupname: (none)
        vhost: default   User-Agent: AnyConnect Android 5.1.15.295
        Remote IP: 94.204.188.26   Location: unknown
        In use: True   Activated: True
        Certificate auth: False`

	detail, err := parse.SessionDetailText(input)
	if err != nil {
		t.Fatalf("SessionDetailText: %v", err)
	}
	if detail.ID != "8Oh70G" {
		t.Errorf("ID: got %q, want %q", detail.ID, "8Oh70G")
	}
	if detail.Username != "maxxie" {
		t.Errorf("Username: got %q, want %q", detail.Username, "maxxie")
	}
	if detail.RemoteAddress != "94.204.188.26" {
		t.Errorf("RemoteAddress: got %q, want %q", detail.RemoteAddress, "94.204.188.26")
	}
	if detail.UserAgent != "AnyConnect Android 5.1.15.295" {
		t.Errorf("UserAgent: got %q, want %q", detail.UserAgent, "AnyConnect Android 5.1.15.295")
	}
	if detail.ConnectedAt.IsZero() {
		t.Error("ConnectedAt should not be zero")
	}
}

func TestSessionDetailTextWithConnectedUser(t *testing.T) {
	t.Parallel()
	// This is the format for a connected (not just authenticated) session.
	input := `Session: 8Oh70G
        Username: maxxie   Groupname: (none)
        Device: vpns2   MTU: 1284
        Remote IP: 94.204.188.26   Location: unknown
        IPv4: 10.10.10.228   P-t-P IPv4: 10.10.10.1
        User-Agent: AnyConnect Android 5.1.15.295
        RX: 4359893 (4.4 MB)   TX: 361852318 (361.9 MB)
        Connected at: 2026-05-07 11:58 ( 1h:28m)
        DNS: 1.1.1.1
                9.9.9.9
        Routes: defaultroute`

	detail, err := parse.SessionDetailText(input)
	if err != nil {
		t.Fatalf("SessionDetailText: %v", err)
	}
	if detail.ID != "8Oh70G" {
		t.Errorf("ID: got %q, want %q", detail.ID, "8Oh70G")
	}
	if detail.Username != "maxxie" {
		t.Errorf("Username: got %q, want %q", detail.Username, "maxxie")
	}
	if detail.Device != "vpns2" {
		t.Errorf("Device: got %q, want %q", detail.Device, "vpns2")
	}
	if detail.IPv4 != "10.10.10.228" {
		t.Errorf("IPv4: got %q, want %q", detail.IPv4, "10.10.10.228")
	}
	if detail.VPNAddress != "10.10.10.228" {
		t.Errorf("VPNAddress: got %q, want %q", detail.VPNAddress, "10.10.10.228")
	}
	if detail.RxBytes != 4359893 {
		t.Errorf("RxBytes: got %d, want %d", detail.RxBytes, 4359893)
	}
	if detail.TxBytes != 361852318 {
		t.Errorf("TxBytes: got %d, want %d", detail.TxBytes, 361852318)
	}
	if detail.ConnectedAt.IsZero() {
		t.Error("ConnectedAt should not be zero")
	}
	if len(detail.DNS) != 2 || detail.DNS[0] != "1.1.1.1" || detail.DNS[1] != "9.9.9.9" {
		t.Errorf("DNS: got %v, want [1.1.1.1 9.9.9.9]", detail.DNS)
	}
	if len(detail.Routes) != 1 || detail.Routes[0] != "defaultroute" {
		t.Errorf("Routes: got %v, want [defaultroute]", detail.Routes)
	}
}

func TestSessionDetailTextEmpty(t *testing.T) {
	t.Parallel()
	_, err := parse.SessionDetailText("")
	if err == nil {
		t.Fatal("expected error for empty input")
	}
}
