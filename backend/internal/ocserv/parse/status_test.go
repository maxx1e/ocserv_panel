package parse_test

import (
	"testing"
	"time"

	"ocserv_panel/backend/internal/ocserv/parse"
)

func TestStatusOutput_FullSample(t *testing.T) {
	input := `Note: the printed statistics are not real-time; session time
as well as RX and TX data are updated on user disconnect

General info:
        Status: online
        Server PID: 3108823
        Sec-mod PID: 3108828
        Sec-mod instance count: 1
        Up since: 2026-05-04 07:29 ( 3days)
        Active sessions: 2
        Total sessions: 22
        Total authentication failures: 6
        IPs in ban list: 0

Current stats period:
        Last stats reset: 2026-05-04 07:29 ( 3days)
        Sessions handled: 22
        Timed out sessions: 0
        Timed out (idle) sessions: 0
        Closed due to error sessions: 14
        Authentication failures: 6
        Average auth time:     4s
        Max auth time:    10s
        Average session time:  3h:14m
        Max session time:  2days
        Min MTU: 1284
        Max MTU: 1284
        RX: 91.5 MB
        TX: 2.6 GB`

	status, err := parse.StatusOutput(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedUpSince := time.Date(2026, 5, 4, 7, 29, 0, 0, time.UTC)
	expectedLastReset := time.Date(2026, 5, 4, 7, 29, 0, 0, time.UTC)

	tests := []struct {
		name string
		got  interface{}
		want interface{}
	}{
		{"Status", status.Status, "online"},
		{"ServerPID", status.ServerPID, 3108823},
		{"SecModPID", status.SecModPID, 3108828},
		{"SecModInstances", status.SecModInstances, 1},
		{"UpSince", status.UpSince, expectedUpSince},
		{"ActiveSessions", status.ActiveSessions, 2},
		{"TotalSessions", status.TotalSessions, 22},
		{"TotalAuthFailures", status.TotalAuthFailures, 6},
		{"BannedIPs", status.BannedIPs, 0},
		{"LastStatsReset", status.LastStatsReset, expectedLastReset},
		{"SessionsHandled", status.SessionsHandled, 22},
		{"TimedOutSessions", status.TimedOutSessions, 0},
		{"IdleTimedOutSessions", status.IdleTimedOutSessions, 0},
		{"ErrorClosedSessions", status.ErrorClosedSessions, 14},
		{"AuthFailures", status.AuthFailures, 6},
		{"AvgAuthTime", status.AvgAuthTime, 4 * time.Second},
		{"MaxAuthTime", status.MaxAuthTime, 10 * time.Second},
		{"AvgSessionTime", status.AvgSessionTime, 3*time.Hour + 14*time.Minute},
		{"MaxSessionTime", status.MaxSessionTime, 48 * time.Hour},
		{"MinMTU", status.MinMTU, 1284},
		{"MaxMTU", status.MaxMTU, 1284},
		{"RxBytes", status.RxBytes, int64(95944704)},
		{"TxBytes", status.TxBytes, int64(2791728742)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Errorf("got %v, want %v", tt.got, tt.want)
			}
		})
	}
}

func TestStatusOutput_EmptyInput(t *testing.T) {
	_, err := parse.StatusOutput("")
	if err == nil {
		t.Fatal("expected error for empty input, got nil")
	}
}

func TestStatusJSON_FullSample(t *testing.T) {
	input := []byte(`{
    "Status":  "online",
    "Server PID":  5523,
    "Sec-mod PID":  5526,
    "Sec-mod instance count":  1,
    "raw_up_since":  1778333803,
    "Active sessions":  0,
    "Total sessions":  0,
    "Total authentication failures":  0,
    "IPs in ban list":  0,
    "raw_last_stats_reset":  1778333803,
    "Sessions handled":  0,
    "Timed out sessions":  0,
    "Timed out (idle) sessions":  0,
    "Closed due to error sessions":  0,
    "Authentication failures":  0,
    "raw_avg_auth_time":  0,
    "raw_max_auth_time":  0,
    "raw_avg_session_time":  0,
    "raw_max_session_time":  0,
    "raw_rx":  0,
    "raw_tx":  0
}`)

	status, err := parse.StatusJSON(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if status.Status != "online" {
		t.Errorf("Status: got %q, want %q", status.Status, "online")
	}
	if status.ServerPID != 5523 {
		t.Errorf("ServerPID: got %d, want %d", status.ServerPID, 5523)
	}
	expectedUpSince := time.Unix(1778333803, 0)
	if !status.UpSince.Equal(expectedUpSince) {
		t.Errorf("UpSince: got %v, want %v", status.UpSince, expectedUpSince)
	}
}

func TestStatusJSON_EmptyInput(t *testing.T) {
	_, err := parse.StatusJSON([]byte{})
	if err == nil {
		t.Fatal("expected error for empty input, got nil")
	}
}

func TestStatusJSON_InvalidJSON(t *testing.T) {
	_, err := parse.StatusJSON([]byte("not json"))
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
}

func TestBytes(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  int64
	}{
		{"megabytes float", "91.5 MB", 95944704},
		{"gigabytes float", "2.6 GB", 2791728742},
		{"kilobytes float", "1.0 KB", 1024},
		{"zero bytes", "0 bytes", 0},
		{"exact kilobytes", "1024 KB", 1048576},
		{"terabyte", "1.0 TB", 1099511627776},
		{"single byte text", "152 bytes", 152},
		{"empty string", "", 0},
		{"megabytes integer", "100 MB", 104857600},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parse.Bytes(tt.input)
			if got != tt.want {
				t.Errorf("Bytes(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

func TestOcctlDuration(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  time.Duration
	}{
		{"seconds", "4s", 4 * time.Second},
		{"seconds double digit", "10s", 10 * time.Second},
		{"zero seconds", "0s", 0},
		{"hours and minutes", "3h:14m", 3*time.Hour + 14*time.Minute},
		{"hours minutes seconds", "1h:30m:45s", 1*time.Hour + 30*time.Minute + 45*time.Second},
		{"days plural", "2days", 48 * time.Hour},
		{"days singular", "1day", 24 * time.Hour},
		{"empty", "", 0},
		{"minutes only", "45m", 45 * time.Minute},
		{"hours only", "2h", 2 * time.Hour},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parse.OcctlDuration(tt.input)
			if got != tt.want {
				t.Errorf("OcctlDuration(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestOcctlTimestamp(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  time.Time
	}{
		{
			"with relative time",
			"2026-05-04 07:29 ( 3days)",
			time.Date(2026, 5, 4, 7, 29, 0, 0, time.UTC),
		},
		{
			"without relative time",
			"2026-05-04 07:29",
			time.Date(2026, 5, 4, 7, 29, 0, 0, time.UTC),
		},
		{"empty string", "", time.Time{}},
		{"invalid format", "not-a-timestamp", time.Time{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parse.OcctlTimestamp(tt.input)
			if !got.Equal(tt.want) {
				t.Errorf("OcctlTimestamp(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}
