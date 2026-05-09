package ocserv

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestOcctlAdapterListSessionsEmpty(t *testing.T) {
	t.Parallel()
	runner := NewFakeCommandRunner()
	runner.Results["/usr/local/bin/occtl"] = CommandResult{ExitCode: 0, Stdout: "[]"}
	adapter := NewOcctlAdapter("/usr/local/bin/occtl", "/var/run/occtl.socket", runner)

	sessions, err := adapter.ListSessions(context.Background())
	if err != nil {
		t.Fatalf("list sessions: %v", err)
	}
	if len(sessions) != 0 {
		t.Fatalf("expected 0 sessions, got %d", len(sessions))
	}
}

func TestOcctlAdapterListSessionsWithData(t *testing.T) {
	t.Parallel()
	runner := NewFakeCommandRunner()
	runner.Results["/usr/local/bin/occtl"] = CommandResult{
		ExitCode: 0,
		Stdout:   `[{"Session":"8Oh70G","Username":"alice","vhost":"default","Remote IP":"10.0.0.1","User-Agent":"AnyConnect Android 5.1.1","Created":"2026-05-05 11:57","State":"authenticated","session_is_open":1}]`,
	}
	adapter := NewOcctlAdapter("/usr/local/bin/occtl", "/var/run/occtl.socket", runner)

	sessions, err := adapter.ListSessions(context.Background())
	if err != nil {
		t.Fatalf("list sessions: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}
	if sessions[0].Username != "alice" {
		t.Fatalf("expected alice, got %s", sessions[0].Username)
	}
	if sessions[0].ID != "8Oh70G" {
		t.Fatalf("expected session ID 8Oh70G, got %s", sessions[0].ID)
	}
	if sessions[0].RemoteAddress != "10.0.0.1" {
		t.Fatalf("expected remote address 10.0.0.1, got %s", sessions[0].RemoteAddress)
	}
	if sessions[0].UserAgent != "AnyConnect Android 5.1.1" {
		t.Fatalf("expected user agent 'AnyConnect Android 5.1.1', got %s", sessions[0].UserAgent)
	}
	if sessions[0].ConnectedAt.IsZero() {
		t.Fatal("expected non-zero ConnectedAt for timestamp '2026-05-05 11:57'")
	}
}

func TestOcctlAdapterListSessionsNotConfigured(t *testing.T) {
	t.Parallel()
	runner := NewFakeCommandRunner()
	runner.Results["/usr/local/bin/occtl"] = CommandResult{
		ExitCode: 1,
		Stderr:   "occtl: connect: no such file or directory",
	}
	adapter := NewOcctlAdapter("/usr/local/bin/occtl", "/var/run/occtl.socket", runner)

	_, err := adapter.ListSessions(context.Background())
	if !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("expected ErrNotConfigured, got %v", err)
	}
}

func TestOcctlAdapterListSessionsEmptyOutput(t *testing.T) {
	t.Parallel()
	runner := NewFakeCommandRunner()
	runner.Results["/usr/local/bin/occtl"] = CommandResult{ExitCode: 0, Stdout: ""}
	adapter := NewOcctlAdapter("/usr/local/bin/occtl", "", runner)

	sessions, err := adapter.ListSessions(context.Background())
	if err != nil {
		t.Fatalf("list sessions with empty output: %v", err)
	}
	if len(sessions) != 0 {
		t.Fatalf("expected 0 sessions, got %d", len(sessions))
	}
}

func TestOcctlAdapterDisconnectSession(t *testing.T) {
	t.Parallel()
	runner := NewFakeCommandRunner()
	runner.Results["/usr/local/bin/occtl"] = CommandResult{ExitCode: 0}
	adapter := NewOcctlAdapter("/usr/local/bin/occtl", "/var/run/occtl.socket", runner)

	if err := adapter.DisconnectSession(context.Background(), "s1"); err != nil {
		t.Fatalf("disconnect: %v", err)
	}
}

func TestOcctlAdapterDisconnectSessionFails(t *testing.T) {
	t.Parallel()
	runner := NewFakeCommandRunner()
	runner.Results["/usr/local/bin/occtl"] = CommandResult{ExitCode: 1, Stderr: "session not found"}
	adapter := NewOcctlAdapter("/usr/local/bin/occtl", "/var/run/occtl.socket", runner)

	if err := adapter.DisconnectSession(context.Background(), "s1"); err == nil {
		t.Fatal("expected disconnect to fail")
	}
}

func TestOcctlAdapterDisconnectSessionNotConfigured(t *testing.T) {
	t.Parallel()
	runner := NewFakeCommandRunner()
	runner.Results["/usr/local/bin/occtl"] = CommandResult{
		ExitCode: 1,
		Stderr:   "occtl: connect: no such file or directory",
	}
	adapter := NewOcctlAdapter("/usr/local/bin/occtl", "/var/run/occtl.socket", runner)

	err := adapter.DisconnectSession(context.Background(), "s99")
	if !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("expected ErrNotConfigured, got %v", err)
	}
}

func TestOcctlAdapterGetSessionDetails(t *testing.T) {
	t.Parallel()
	runner := NewFakeCommandRunner()
	runner.Results["/usr/local/bin/occtl"] = CommandResult{
		ExitCode: 0,
		Stdout:   `{"Session":"8Oh70G","Username":"alice","Remote IP":"10.0.0.1","User-Agent":"AnyConnect 4.10","Created":"2026-05-05 11:57","State":"authenticated","vhost":"default"}`,
	}
	adapter := NewOcctlAdapter("/usr/local/bin/occtl", "/var/run/occtl.socket", runner)

	detail, err := adapter.GetSessionDetails(context.Background(), "8Oh70G")
	if err != nil {
		t.Fatalf("get session details: %v", err)
	}
	if detail.Username != "alice" {
		t.Fatalf("expected alice, got %s", detail.Username)
	}
	if detail.ID != "8Oh70G" {
		t.Fatalf("expected session ID 8Oh70G, got %s", detail.ID)
	}
	if detail.RemoteAddress != "10.0.0.1" {
		t.Fatalf("expected remote address 10.0.0.1, got %s", detail.RemoteAddress)
	}
	if detail.UserAgent != "AnyConnect 4.10" {
		t.Fatalf("expected user agent 'AnyConnect 4.10', got %s", detail.UserAgent)
	}
}

func TestOcctlAdapterWithSocketInjectsFlag(t *testing.T) {
	t.Parallel()

	// Create a temp file to simulate an existing socket path.
	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "occtl.socket")
	if err := os.WriteFile(socketPath, nil, 0600); err != nil {
		t.Fatalf("create temp socket file: %v", err)
	}

	runner := NewFakeCommandRunner()
	runner.Results["/usr/local/bin/occtl"] = CommandResult{ExitCode: 0, Stdout: "[]"}
	adapter := NewOcctlAdapter("/usr/local/bin/occtl", socketPath, runner)

	_, _ = adapter.ListSessions(context.Background())

	if len(runner.Calls) == 0 {
		t.Fatal("expected at least one call recorded")
	}
	call := runner.Calls[0]
	// ListSessions uses withJSONAndSocket, so args should start with: -j -n -s <socket> show sessions all
	if len(call.Args) < 4 || call.Args[0] != "-j" || call.Args[1] != "-n" || call.Args[2] != "-s" {
		t.Fatalf("expected -j -n -s as first args, got %v", call.Args)
	}
	if call.Args[3] != socketPath {
		t.Fatalf("expected socket path as fourth arg, got %v", call.Args)
	}
}

func TestOcctlAdapterWithoutSocketOmitsFlag(t *testing.T) {
	t.Parallel()
	runner := NewFakeCommandRunner()
	runner.Results["/usr/local/bin/occtl"] = CommandResult{ExitCode: 0, Stdout: "[]"}
	adapter := NewOcctlAdapter("/usr/local/bin/occtl", "", runner) // empty socket

	_, _ = adapter.ListSessions(context.Background())

	if len(runner.Calls) == 0 {
		t.Fatal("expected at least one call recorded")
	}
	call := runner.Calls[0]
	// Without socket: args should start with -j -n followed by the command (no -s)
	if len(call.Args) < 2 || call.Args[0] != "-j" || call.Args[1] != "-n" {
		t.Fatalf("expected -j -n as first args, got %v", call.Args)
	}
	if len(call.Args) > 2 && call.Args[2] == "-s" {
		t.Fatalf("expected no -s when socket is empty, got %v", call.Args)
	}
}

func TestOcctlAdapterMissingSocketOmitsFlag(t *testing.T) {
	t.Parallel()
	// Configure a socket path that does not exist on disk.
	runner := NewFakeCommandRunner()
	runner.Results["/usr/local/bin/occtl"] = CommandResult{ExitCode: 0, Stdout: "[]"}
	adapter := NewOcctlAdapter("/usr/local/bin/occtl", "/var/run/ocserv-socket.nonexistent", runner)

	_, _ = adapter.ListSessions(context.Background())

	if len(runner.Calls) == 0 {
		t.Fatal("expected at least one call recorded")
	}
	call := runner.Calls[0]
	// Socket path is configured but file doesn't exist — -s must be omitted.
	if len(call.Args) < 2 || call.Args[0] != "-j" || call.Args[1] != "-n" {
		t.Fatalf("expected -j -n as first args, got %v", call.Args)
	}
	for i, a := range call.Args {
		if a == "-s" {
			t.Fatalf("expected no -s when socket file is missing, but found -s at position %d: %v", i, call.Args)
		}
	}
}

func TestOcctlAdapterTerminateSessionSuccess(t *testing.T) {
	t.Parallel()
	runner := NewFakeCommandRunner()
	runner.Results["/usr/local/bin/occtl"] = CommandResult{ExitCode: 0, Stdout: ""}
	adapter := NewOcctlAdapter("/usr/local/bin/occtl", "/var/run/occtl.socket", runner)

	if err := adapter.TerminateSession(context.Background(), "s42"); err != nil {
		t.Fatalf("terminate session: %v", err)
	}

	if len(runner.Calls) == 0 {
		t.Fatal("expected at least one call recorded")
	}
	args := runner.Calls[0].Args
	// Expect: --socket-file <path> terminate id s42
	found := false
	for i, a := range args {
		if a == "terminate" && i+2 < len(args) && args[i+1] == "id" && args[i+2] == "s42" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected args to contain 'terminate id s42', got %v", args)
	}
}

func TestOcctlAdapterTerminateSessionRunnerError(t *testing.T) {
	t.Parallel()
	runner := NewFakeCommandRunner()
	runner.Err = fmt.Errorf("binary not found")
	adapter := NewOcctlAdapter("/usr/local/bin/occtl", "/var/run/occtl.socket", runner)

	err := adapter.TerminateSession(context.Background(), "s42")
	if err == nil {
		t.Fatal("expected error when runner returns an error")
	}
	if !errors.Is(err, runner.Err) {
		t.Fatalf("expected error to wrap runner error, got %v", err)
	}
}

func TestOcctlAdapterTerminateSessionNonZeroExit(t *testing.T) {
	t.Parallel()
	runner := NewFakeCommandRunner()
	runner.Results["/usr/local/bin/occtl"] = CommandResult{
		ExitCode: 1,
		Stderr:   "no session found",
	}
	adapter := NewOcctlAdapter("/usr/local/bin/occtl", "/var/run/occtl.socket", runner)

	err := adapter.TerminateSession(context.Background(), "s99")
	if err == nil {
		t.Fatal("expected error for non-zero exit code")
	}
	const wantSubstr = "no session found"
	if !containsSubstr(err.Error(), wantSubstr) {
		t.Fatalf("expected error to contain %q, got %q", wantSubstr, err.Error())
	}
}

func TestOcctlAdapterTerminateSessionNotConfigured(t *testing.T) {
	t.Parallel()
	runner := NewFakeCommandRunner()
	runner.Results["/usr/local/bin/occtl"] = CommandResult{
		ExitCode: 1,
		Stderr:   "occtl: connect: no such file or directory",
	}
	adapter := NewOcctlAdapter("/usr/local/bin/occtl", "/var/run/occtl.socket", runner)

	err := adapter.TerminateSession(context.Background(), "s99")
	if !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("expected ErrNotConfigured, got %v", err)
	}
}

func TestOcctlAdapterGetSessionDetailsExpandedFields(t *testing.T) {
	t.Parallel()
	runner := NewFakeCommandRunner()
	runner.Results["/usr/local/bin/occtl"] = CommandResult{
		ExitCode: 0,
		Stdout: `{
			"Session": "s1",
			"Username": "alice",
			"Remote IP": "203.0.113.5",
			"Created": "2026-01-15 10:30",
			"State": "connected",
			"vhost": "default",
			"User-Agent": "AnyConnect/4.10",
			"Device": "laptop",
			"IPv4": "10.8.0.2",
			"IPv6": "fd00::2",
			"raw_rx": 1048576,
			"raw_tx": 2097152,
			"Routes": ["10.0.0.0/8", "192.168.0.0/16"],
			"DNS": ["8.8.8.8", "1.1.1.1"]
		}`,
	}
	adapter := NewOcctlAdapter("/usr/local/bin/occtl", "/var/run/occtl.socket", runner)

	detail, err := adapter.GetSessionDetails(context.Background(), "s1")
	if err != nil {
		t.Fatalf("get session details: %v", err)
	}

	if detail.ID != "s1" {
		t.Errorf("ID: got %q, want %q", detail.ID, "s1")
	}
	if detail.Username != "alice" {
		t.Errorf("Username: got %q, want %q", detail.Username, "alice")
	}
	if detail.RemoteAddress != "203.0.113.5" {
		t.Errorf("RemoteAddress: got %q, want %q", detail.RemoteAddress, "203.0.113.5")
	}
	if detail.VPNAddress != "10.8.0.2" {
		t.Errorf("VPNAddress: got %q, want %q", detail.VPNAddress, "10.8.0.2")
	}
	if detail.IPv4 != "10.8.0.2" {
		t.Errorf("IPv4: got %q, want %q", detail.IPv4, "10.8.0.2")
	}
	if detail.IPv6 != "fd00::2" {
		t.Errorf("IPv6: got %q, want %q", detail.IPv6, "fd00::2")
	}
	if detail.RxBytes != 1048576 {
		t.Errorf("RxBytes: got %d, want %d", detail.RxBytes, 1048576)
	}
	if detail.TxBytes != 2097152 {
		t.Errorf("TxBytes: got %d, want %d", detail.TxBytes, 2097152)
	}
	if detail.UserAgent != "AnyConnect/4.10" {
		t.Errorf("UserAgent: got %q, want %q", detail.UserAgent, "AnyConnect/4.10")
	}
	if detail.Device != "laptop" {
		t.Errorf("Device: got %q, want %q", detail.Device, "laptop")
	}
	if len(detail.Routes) != 2 || detail.Routes[0] != "10.0.0.0/8" || detail.Routes[1] != "192.168.0.0/16" {
		t.Errorf("Routes: got %v, want [10.0.0.0/8 192.168.0.0/16]", detail.Routes)
	}
	if len(detail.DNS) != 2 || detail.DNS[0] != "8.8.8.8" || detail.DNS[1] != "1.1.1.1" {
		t.Errorf("DNS: got %v, want [8.8.8.8 1.1.1.1]", detail.DNS)
	}
	if detail.ConnectedAt.IsZero() {
		t.Error("ConnectedAt should not be zero when a valid timestamp is provided")
	}
}

func TestOcctlAdapterDisconnectSessionInvalidID(t *testing.T) {
	t.Parallel()
	runner := NewFakeCommandRunner()
	adapter := NewOcctlAdapter("/usr/local/bin/occtl", "/var/run/occtl.socket", runner)

	badIDs := []string{
		"",
		"--socket-file=/evil",
		"id with spaces",
		"id\nnewline",
		string(make([]byte, 129)),
	}
	for _, id := range badIDs {
		err := adapter.DisconnectSession(context.Background(), id)
		if err == nil {
			t.Errorf("DisconnectSession(%q) = nil, want error", id)
		}
	}
	if len(runner.Calls) > 0 {
		t.Errorf("expected no commands executed for invalid IDs, got %d", len(runner.Calls))
	}
}

func TestOcctlAdapterTerminateSessionInvalidID(t *testing.T) {
	t.Parallel()
	runner := NewFakeCommandRunner()
	adapter := NewOcctlAdapter("/usr/local/bin/occtl", "/var/run/occtl.socket", runner)

	err := adapter.TerminateSession(context.Background(), "--malicious")
	if err == nil {
		t.Fatal("TerminateSession with flag-like ID should fail")
	}
	if len(runner.Calls) > 0 {
		t.Error("no command should be executed for invalid session ID")
	}
}

func TestOcctlAdapterGetSessionDetailsInvalidID(t *testing.T) {
	t.Parallel()
	runner := NewFakeCommandRunner()
	adapter := NewOcctlAdapter("/usr/local/bin/occtl", "/var/run/occtl.socket", runner)

	_, err := adapter.GetSessionDetails(context.Background(), "../etc/shadow")
	if err == nil {
		t.Fatal("GetSessionDetails with path traversal ID should fail")
	}
	if len(runner.Calls) > 0 {
		t.Error("no command should be executed for invalid session ID")
	}
}

// containsSubstr is a helper that avoids importing strings in this test file
// (strings is already used in the production code, not the test file).
func containsSubstr(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 || findSubstr(s, sub))
}

func findSubstr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// --- Tests for GetServerStatus ---

func TestOcctlAdapterGetServerStatusSuccess(t *testing.T) {
	t.Parallel()
	runner := NewFakeCommandRunner()
	runner.Results["/usr/local/bin/occtl"] = CommandResult{
		ExitCode: 0,
		Stdout: `{
    "Status": "online",
    "Server PID": 1234,
    "Sec-mod PID": 5678,
    "Sec-mod instance count": 1,
    "raw_up_since": 1778000000,
    "Active sessions": 5,
    "Total sessions": 22,
    "Total authentication failures": 3,
    "IPs in ban list": 1,
    "raw_last_stats_reset": 1778000000,
    "Sessions handled": 22,
    "Timed out sessions": 0,
    "Timed out (idle) sessions": 0,
    "Closed due to error sessions": 0,
    "Authentication failures": 3,
    "raw_avg_auth_time": 0,
    "raw_max_auth_time": 0,
    "raw_avg_session_time": 0,
    "raw_max_session_time": 0,
    "raw_rx": 0,
    "raw_tx": 0
}`,
	}
	adapter := NewOcctlAdapter("/usr/local/bin/occtl", "/var/run/occtl.socket", runner)

	status, err := adapter.GetServerStatus(context.Background())
	if err != nil {
		t.Fatalf("get server status: %v", err)
	}
	if status.Status != "online" {
		t.Errorf("Status: got %q, want %q", status.Status, "online")
	}
	if status.ServerPID != 1234 {
		t.Errorf("ServerPID: got %d, want %d", status.ServerPID, 1234)
	}
	if status.ActiveSessions != 5 {
		t.Errorf("ActiveSessions: got %d, want %d", status.ActiveSessions, 5)
	}
	if status.TotalSessions != 22 {
		t.Errorf("TotalSessions: got %d, want %d", status.TotalSessions, 22)
	}
	if status.TotalAuthFailures != 3 {
		t.Errorf("TotalAuthFailures: got %d, want %d", status.TotalAuthFailures, 3)
	}
	if status.BannedIPs != 1 {
		t.Errorf("BannedIPs: got %d, want %d", status.BannedIPs, 1)
	}

	// Verify the adapter now uses -j -n flags (withJSONAndSocket)
	if len(runner.Calls) == 0 {
		t.Fatal("expected at least one call recorded")
	}
	args := runner.Calls[0].Args
	if len(args) < 2 || args[0] != "-j" || args[1] != "-n" {
		t.Fatalf("expected -j -n as first args (JSON mode), got %v", args)
	}
}

func TestOcctlAdapterGetServerStatusNotConfigured(t *testing.T) {
	t.Parallel()
	runner := NewFakeCommandRunner()
	runner.Results["/usr/local/bin/occtl"] = CommandResult{
		ExitCode: 1,
		Stderr:   "occtl: connect: no such file or directory",
	}
	adapter := NewOcctlAdapter("/usr/local/bin/occtl", "/var/run/occtl.socket", runner)

	_, err := adapter.GetServerStatus(context.Background())
	if !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("expected ErrNotConfigured, got %v", err)
	}
}

func TestOcctlAdapterGetServerStatusRunnerError(t *testing.T) {
	t.Parallel()
	runner := NewFakeCommandRunner()
	runner.Err = fmt.Errorf("command not found")
	adapter := NewOcctlAdapter("/usr/local/bin/occtl", "/var/run/occtl.socket", runner)

	_, err := adapter.GetServerStatus(context.Background())
	if err == nil {
		t.Fatal("expected error when runner fails")
	}
}

// --- Tests for ListConnectedUsers ---

func TestOcctlAdapterListConnectedUsersJSON(t *testing.T) {
	t.Parallel()
	runner := NewFakeCommandRunner()
	runner.Results["/usr/local/bin/occtl"] = CommandResult{
		ExitCode: 0,
		Stdout:   `[{"ID":123,"Username":"testuser","vhost":"default","Remote IP":"1.2.3.4","IPv4":"10.0.0.1","Device":"vpns0","_Connected at":" 1h:30m","DTLS cipher":"(AES-128-GCM)","State":"connected","Connected at":"2026-05-09 10:00","raw_connected_at":1778518800}]`,
	}
	adapter := NewOcctlAdapter("/usr/local/bin/occtl", "/var/run/occtl.socket", runner)

	users, err := adapter.ListConnectedUsers(context.Background())
	if err != nil {
		t.Fatalf("list connected users: %v", err)
	}
	if len(users) != 1 {
		t.Fatalf("expected 1 user, got %d", len(users))
	}
	if users[0].Username != "testuser" {
		t.Errorf("Username: got %q, want %q", users[0].Username, "testuser")
	}
	if users[0].RemoteIP != "1.2.3.4" {
		t.Errorf("RemoteIP: got %q, want %q", users[0].RemoteIP, "1.2.3.4")
	}
	if users[0].Status != "connected" {
		t.Errorf("Status: got %q, want %q", users[0].Status, "connected")
	}
}

func TestOcctlAdapterListConnectedUsersFallbackToText(t *testing.T) {
	t.Parallel()
	runner := NewFakeCommandRunner()
	// Provide output that is NOT valid JSON to trigger the text fallback.
	runner.Results["/usr/local/bin/occtl"] = CommandResult{
		ExitCode: 0,
		Stdout:   "  3318457   alice  default  94.204.188.26   10.10.10.228  vpns2  1h:27m  (AES-128-GCM) connected\n",
	}
	adapter := NewOcctlAdapter("/usr/local/bin/occtl", "/var/run/occtl.socket", runner)

	users, err := adapter.ListConnectedUsers(context.Background())
	if err != nil {
		t.Fatalf("list connected users (text fallback): %v", err)
	}
	if len(users) != 1 {
		t.Fatalf("expected 1 user, got %d", len(users))
	}
	if users[0].Username != "alice" {
		t.Errorf("Username: got %q, want %q", users[0].Username, "alice")
	}
}

func TestOcctlAdapterListConnectedUsersEmpty(t *testing.T) {
	t.Parallel()
	runner := NewFakeCommandRunner()
	runner.Results["/usr/local/bin/occtl"] = CommandResult{ExitCode: 0, Stdout: ""}
	adapter := NewOcctlAdapter("/usr/local/bin/occtl", "/var/run/occtl.socket", runner)

	users, err := adapter.ListConnectedUsers(context.Background())
	if err != nil {
		t.Fatalf("list connected users (empty): %v", err)
	}
	if len(users) != 0 {
		t.Fatalf("expected 0 users, got %d", len(users))
	}
}

func TestOcctlAdapterListConnectedUsersNotConfigured(t *testing.T) {
	t.Parallel()
	runner := NewFakeCommandRunner()
	runner.Results["/usr/local/bin/occtl"] = CommandResult{
		ExitCode: 1,
		Stderr:   "connection refused",
	}
	adapter := NewOcctlAdapter("/usr/local/bin/occtl", "/var/run/occtl.socket", runner)

	_, err := adapter.ListConnectedUsers(context.Background())
	if !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("expected ErrNotConfigured, got %v", err)
	}
}

// --- Tests for GetUserDetails ---

func TestOcctlAdapterGetUserDetailsSuccess(t *testing.T) {
	t.Parallel()
	runner := NewFakeCommandRunner()
	runner.Results["/usr/local/bin/occtl"] = CommandResult{
		ExitCode: 0,
		Stdout: `[{
			"ID": 12345,
			"Username": "testuser",
			"Groupname": "(none)",
			"State": "connected",
			"vhost": "default",
			"Device": "vpns0",
			"MTU": "1284",
			"Remote IP": "94.204.188.26",
			"Location": "unknown"
		}]`,
	}
	adapter := NewOcctlAdapter("/usr/local/bin/occtl", "/var/run/occtl.socket", runner)

	conns, err := adapter.GetUserDetails(context.Background(), "testuser")
	if err != nil {
		t.Fatalf("get user details: %v", err)
	}
	if len(conns) != 1 {
		t.Fatalf("expected 1 connection, got %d", len(conns))
	}
	if conns[0].ID != "12345" {
		t.Errorf("ID: got %q, want %q", conns[0].ID, "12345")
	}
	if conns[0].Username != "testuser" {
		t.Errorf("Username: got %q, want %q", conns[0].Username, "testuser")
	}
	if conns[0].State != "connected" {
		t.Errorf("State: got %q, want %q", conns[0].State, "connected")
	}
	if conns[0].Device != "vpns0" {
		t.Errorf("Device: got %q, want %q", conns[0].Device, "vpns0")
	}
	if conns[0].MTU != 1284 {
		t.Errorf("MTU: got %d, want %d", conns[0].MTU, 1284)
	}
	if conns[0].RemoteIP != "94.204.188.26" {
		t.Errorf("RemoteIP: got %q, want %q", conns[0].RemoteIP, "94.204.188.26")
	}
}

func TestOcctlAdapterGetUserDetailsTextFallback(t *testing.T) {
	t.Parallel()
	runner := NewFakeCommandRunner()
	runner.Results["/usr/local/bin/occtl"] = CommandResult{
		ExitCode: 0,
		Stdout: `        ID: 12345
        Username: testuser   Groupname: (none)
        State: connected
        vhost: default
        Device: vpns0   MTU: 1284
        Remote IP: 94.204.188.26   Location: unknown
`,
	}
	adapter := NewOcctlAdapter("/usr/local/bin/occtl", "/var/run/occtl.socket", runner)

	conns, err := adapter.GetUserDetails(context.Background(), "testuser")
	if err != nil {
		t.Fatalf("get user details: %v", err)
	}
	if len(conns) != 1 {
		t.Fatalf("expected 1 connection, got %d", len(conns))
	}
	if conns[0].ID != "12345" {
		t.Errorf("ID: got %q, want %q", conns[0].ID, "12345")
	}
	if conns[0].Username != "testuser" {
		t.Errorf("Username: got %q, want %q", conns[0].Username, "testuser")
	}
	if conns[0].State != "connected" {
		t.Errorf("State: got %q, want %q", conns[0].State, "connected")
	}
	if conns[0].Device != "vpns0" {
		t.Errorf("Device: got %q, want %q", conns[0].Device, "vpns0")
	}
	if conns[0].MTU != 1284 {
		t.Errorf("MTU: got %d, want %d", conns[0].MTU, 1284)
	}
}

func TestOcctlAdapterGetUserDetailsInvalidUsername(t *testing.T) {
	t.Parallel()
	runner := NewFakeCommandRunner()
	adapter := NewOcctlAdapter("/usr/local/bin/occtl", "/var/run/occtl.socket", runner)

	_, err := adapter.GetUserDetails(context.Background(), "invalid user!")
	if err == nil {
		t.Fatal("expected error for invalid username")
	}
	if len(runner.Calls) > 0 {
		t.Error("no command should be executed for invalid username")
	}
}

func TestOcctlAdapterGetUserDetailsNotConfigured(t *testing.T) {
	t.Parallel()
	runner := NewFakeCommandRunner()
	runner.Results["/usr/local/bin/occtl"] = CommandResult{
		ExitCode: 1,
		Stderr:   "occtl: connect: no such file or directory",
	}
	adapter := NewOcctlAdapter("/usr/local/bin/occtl", "/var/run/occtl.socket", runner)

	_, err := adapter.GetUserDetails(context.Background(), "validuser")
	if !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("expected ErrNotConfigured, got %v", err)
	}
}

// --- Tests for ListEvents ---

func TestOcctlAdapterListEventsSuccess(t *testing.T) {
	t.Parallel()
	runner := NewFakeCommandRunner()
	runner.Results["/usr/local/bin/occtl"] = CommandResult{
		ExitCode: 0,
		Stdout:   `[{"timestamp":"2026-05-07T12:00:00Z","username":"testuser","remote_ip":"1.2.3.4","event_type":"connect","message":"connected"}]`,
	}
	adapter := NewOcctlAdapter("/usr/local/bin/occtl", "/var/run/occtl.socket", runner)

	events, err := adapter.ListEvents(context.Background())
	if err != nil {
		t.Fatalf("list events: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Username != "testuser" {
		t.Errorf("Username: got %q, want %q", events[0].Username, "testuser")
	}
	if events[0].EventType != "connect" {
		t.Errorf("EventType: got %q, want %q", events[0].EventType, "connect")
	}
	if events[0].Timestamp.IsZero() {
		t.Error("Timestamp should not be zero")
	}
}

func TestOcctlAdapterListEventsEmpty(t *testing.T) {
	t.Parallel()
	runner := NewFakeCommandRunner()
	runner.Results["/usr/local/bin/occtl"] = CommandResult{ExitCode: 0, Stdout: ""}
	adapter := NewOcctlAdapter("/usr/local/bin/occtl", "/var/run/occtl.socket", runner)

	events, err := adapter.ListEvents(context.Background())
	if err != nil {
		t.Fatalf("list events (empty): %v", err)
	}
	if len(events) != 0 {
		t.Fatalf("expected 0 events, got %d", len(events))
	}
}

func TestOcctlAdapterListEventsNotConfigured(t *testing.T) {
	t.Parallel()
	runner := NewFakeCommandRunner()
	runner.Results["/usr/local/bin/occtl"] = CommandResult{
		ExitCode: 1,
		Stderr:   "not found",
	}
	adapter := NewOcctlAdapter("/usr/local/bin/occtl", "/var/run/occtl.socket", runner)

	_, err := adapter.ListEvents(context.Background())
	if !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("expected ErrNotConfigured, got %v", err)
	}
}

// --- Tests for ListRoutes ---

func TestOcctlAdapterListRoutesSuccess(t *testing.T) {
	t.Parallel()
	runner := NewFakeCommandRunner()
	runner.Results["/usr/local/bin/occtl"] = CommandResult{
		ExitCode: 0,
		Stdout: `    id     user    vhost device           vpn-ip                       iroute
    42   alice  default  vpns0  10.10.10.5  192.168.1.0/24
`,
	}
	adapter := NewOcctlAdapter("/usr/local/bin/occtl", "/var/run/occtl.socket", runner)

	routes, err := adapter.ListRoutes(context.Background())
	if err != nil {
		t.Fatalf("list routes: %v", err)
	}
	if len(routes) != 1 {
		t.Fatalf("expected 1 route, got %d", len(routes))
	}
	if routes[0].Username != "alice" {
		t.Errorf("Username: got %q, want %q", routes[0].Username, "alice")
	}
	if routes[0].Route != "192.168.1.0/24" {
		t.Errorf("Route: got %q, want %q", routes[0].Route, "192.168.1.0/24")
	}
}

func TestOcctlAdapterListRoutesEmpty(t *testing.T) {
	t.Parallel()
	runner := NewFakeCommandRunner()
	runner.Results["/usr/local/bin/occtl"] = CommandResult{
		ExitCode: 0,
		Stdout:   "    id     user    vhost device           vpn-ip                       iroute\n",
	}
	adapter := NewOcctlAdapter("/usr/local/bin/occtl", "/var/run/occtl.socket", runner)

	routes, err := adapter.ListRoutes(context.Background())
	if err != nil {
		t.Fatalf("list routes (empty): %v", err)
	}
	if len(routes) != 0 {
		t.Fatalf("expected 0 routes, got %d", len(routes))
	}
}

func TestOcctlAdapterListRoutesNotConfigured(t *testing.T) {
	t.Parallel()
	runner := NewFakeCommandRunner()
	runner.Results["/usr/local/bin/occtl"] = CommandResult{
		ExitCode: 1,
		Stderr:   "no such file or directory",
	}
	adapter := NewOcctlAdapter("/usr/local/bin/occtl", "/var/run/occtl.socket", runner)

	_, err := adapter.ListRoutes(context.Background())
	if !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("expected ErrNotConfigured, got %v", err)
	}
}

// --- Tests for ListBannedIPs ---

func TestOcctlAdapterListBannedIPsSuccess(t *testing.T) {
	t.Parallel()
	runner := NewFakeCommandRunner()
	runner.Results["/usr/local/bin/occtl"] = CommandResult{
		ExitCode: 0,
		Stdout:   `[{"ip":"10.0.0.5","score":50,"reason":"auth failures"}]`,
	}
	adapter := NewOcctlAdapter("/usr/local/bin/occtl", "/var/run/occtl.socket", runner)

	bans, err := adapter.ListBannedIPs(context.Background())
	if err != nil {
		t.Fatalf("list banned ips: %v", err)
	}
	if len(bans) != 1 {
		t.Fatalf("expected 1 ban, got %d", len(bans))
	}
	if bans[0].IP != "10.0.0.5" {
		t.Errorf("IP: got %q, want %q", bans[0].IP, "10.0.0.5")
	}
	if bans[0].Score != 50 {
		t.Errorf("Score: got %d, want %d", bans[0].Score, 50)
	}
	if bans[0].Reason != "auth failures" {
		t.Errorf("Reason: got %q, want %q", bans[0].Reason, "auth failures")
	}
}

func TestOcctlAdapterListBannedIPsEmpty(t *testing.T) {
	t.Parallel()
	runner := NewFakeCommandRunner()
	runner.Results["/usr/local/bin/occtl"] = CommandResult{ExitCode: 0, Stdout: ""}
	adapter := NewOcctlAdapter("/usr/local/bin/occtl", "/var/run/occtl.socket", runner)

	bans, err := adapter.ListBannedIPs(context.Background())
	if err != nil {
		t.Fatalf("list banned ips (empty): %v", err)
	}
	if len(bans) != 0 {
		t.Fatalf("expected 0 bans, got %d", len(bans))
	}
}

func TestOcctlAdapterListBannedIPsNotConfigured(t *testing.T) {
	t.Parallel()
	runner := NewFakeCommandRunner()
	runner.Results["/usr/local/bin/occtl"] = CommandResult{
		ExitCode: 1,
		Stderr:   "connection refused",
	}
	adapter := NewOcctlAdapter("/usr/local/bin/occtl", "/var/run/occtl.socket", runner)

	_, err := adapter.ListBannedIPs(context.Background())
	if !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("expected ErrNotConfigured, got %v", err)
	}
}

// --- Tests for ListBanPoints ---

func TestOcctlAdapterListBanPointsSuccess(t *testing.T) {
	t.Parallel()
	runner := NewFakeCommandRunner()
	runner.Results["/usr/local/bin/occtl"] = CommandResult{
		ExitCode: 0,
		Stdout:   `[{"ip":"10.0.0.5","points":30,"since":"2026-05-07T08:00:00Z"}]`,
	}
	adapter := NewOcctlAdapter("/usr/local/bin/occtl", "/var/run/occtl.socket", runner)

	points, err := adapter.ListBanPoints(context.Background())
	if err != nil {
		t.Fatalf("list ban points: %v", err)
	}
	if len(points) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(points))
	}
	if points[0].IP != "10.0.0.5" {
		t.Errorf("IP: got %q, want %q", points[0].IP, "10.0.0.5")
	}
	if points[0].Points != 30 {
		t.Errorf("Points: got %d, want %d", points[0].Points, 30)
	}
	if points[0].Since.IsZero() {
		t.Error("Since should not be zero")
	}
}

func TestOcctlAdapterListBanPointsEmpty(t *testing.T) {
	t.Parallel()
	runner := NewFakeCommandRunner()
	runner.Results["/usr/local/bin/occtl"] = CommandResult{ExitCode: 0, Stdout: ""}
	adapter := NewOcctlAdapter("/usr/local/bin/occtl", "/var/run/occtl.socket", runner)

	points, err := adapter.ListBanPoints(context.Background())
	if err != nil {
		t.Fatalf("list ban points (empty): %v", err)
	}
	if len(points) != 0 {
		t.Fatalf("expected 0 entries, got %d", len(points))
	}
}

func TestOcctlAdapterListBanPointsNotConfigured(t *testing.T) {
	t.Parallel()
	runner := NewFakeCommandRunner()
	runner.Results["/usr/local/bin/occtl"] = CommandResult{
		ExitCode: 1,
		Stderr:   "no such file or directory",
	}
	adapter := NewOcctlAdapter("/usr/local/bin/occtl", "/var/run/occtl.socket", runner)

	_, err := adapter.ListBanPoints(context.Background())
	if !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("expected ErrNotConfigured, got %v", err)
	}
}

// --- Tests for DisconnectUser ---

func TestOcctlAdapterDisconnectUserSuccess(t *testing.T) {
	t.Parallel()
	runner := NewFakeCommandRunner()
	runner.Results["/usr/local/bin/occtl"] = CommandResult{ExitCode: 0}
	adapter := NewOcctlAdapter("/usr/local/bin/occtl", "/var/run/occtl.socket", runner)

	if err := adapter.DisconnectUser(context.Background(), "testuser"); err != nil {
		t.Fatalf("disconnect user: %v", err)
	}

	if len(runner.Calls) == 0 {
		t.Fatal("expected at least one call")
	}
	args := runner.Calls[0].Args
	found := false
	for i, a := range args {
		if a == "disconnect" && i+2 < len(args) && args[i+1] == "user" && args[i+2] == "testuser" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected args to contain 'disconnect user testuser', got %v", args)
	}
}

func TestOcctlAdapterDisconnectUserInvalidUsername(t *testing.T) {
	t.Parallel()
	runner := NewFakeCommandRunner()
	adapter := NewOcctlAdapter("/usr/local/bin/occtl", "/var/run/occtl.socket", runner)

	err := adapter.DisconnectUser(context.Background(), "bad user!")
	if err == nil {
		t.Fatal("expected error for invalid username")
	}
	if len(runner.Calls) > 0 {
		t.Error("no command should be executed for invalid username")
	}
}

func TestOcctlAdapterDisconnectUserNotConfigured(t *testing.T) {
	t.Parallel()
	runner := NewFakeCommandRunner()
	runner.Results["/usr/local/bin/occtl"] = CommandResult{
		ExitCode: 1,
		Stderr:   "no such file or directory",
	}
	adapter := NewOcctlAdapter("/usr/local/bin/occtl", "/var/run/occtl.socket", runner)

	err := adapter.DisconnectUser(context.Background(), "validuser")
	if !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("expected ErrNotConfigured, got %v", err)
	}
}

// --- Tests for UnbanIP ---

func TestOcctlAdapterUnbanIPSuccess(t *testing.T) {
	t.Parallel()
	runner := NewFakeCommandRunner()
	runner.Results["/usr/local/bin/occtl"] = CommandResult{ExitCode: 0}
	adapter := NewOcctlAdapter("/usr/local/bin/occtl", "/var/run/occtl.socket", runner)

	if err := adapter.UnbanIP(context.Background(), "10.0.0.5"); err != nil {
		t.Fatalf("unban ip: %v", err)
	}

	if len(runner.Calls) == 0 {
		t.Fatal("expected at least one call")
	}
	args := runner.Calls[0].Args
	found := false
	for i, a := range args {
		if a == "unban" && i+2 < len(args) && args[i+1] == "ip" && args[i+2] == "10.0.0.5" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected args to contain 'unban ip 10.0.0.5', got %v", args)
	}
}

func TestOcctlAdapterUnbanIPInvalidIP(t *testing.T) {
	t.Parallel()
	runner := NewFakeCommandRunner()
	adapter := NewOcctlAdapter("/usr/local/bin/occtl", "/var/run/occtl.socket", runner)

	badIPs := []string{
		"",
		"not-an-ip",
		"--flag-injection",
		"10.0.0.0/24",
	}
	for _, ip := range badIPs {
		err := adapter.UnbanIP(context.Background(), ip)
		if err == nil {
			t.Errorf("UnbanIP(%q) = nil, want error", ip)
		}
	}
	if len(runner.Calls) > 0 {
		t.Errorf("expected no commands executed for invalid IPs, got %d", len(runner.Calls))
	}
}

func TestOcctlAdapterUnbanIPNotConfigured(t *testing.T) {
	t.Parallel()
	runner := NewFakeCommandRunner()
	runner.Results["/usr/local/bin/occtl"] = CommandResult{
		ExitCode: 1,
		Stderr:   "connection refused",
	}
	adapter := NewOcctlAdapter("/usr/local/bin/occtl", "/var/run/occtl.socket", runner)

	err := adapter.UnbanIP(context.Background(), "192.168.1.1")
	if !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("expected ErrNotConfigured, got %v", err)
	}
}

func TestOcctlAdapterUnbanIPv6Success(t *testing.T) {
	t.Parallel()
	runner := NewFakeCommandRunner()
	runner.Results["/usr/local/bin/occtl"] = CommandResult{ExitCode: 0}
	adapter := NewOcctlAdapter("/usr/local/bin/occtl", "/var/run/occtl.socket", runner)

	if err := adapter.UnbanIP(context.Background(), "2001:db8::1"); err != nil {
		t.Fatalf("unban ipv6: %v", err)
	}
}

// --- Tests for GetConnectionByID ---

func TestOcctlAdapterGetConnectionByID(t *testing.T) {
	t.Parallel()
	runner := NewFakeCommandRunner()
	runner.Results["/usr/local/bin/occtl"] = CommandResult{
		ExitCode: 0,
		Stdout: `[{
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
			"Average RX": "0 bytes/s",
			"Average TX": "0 bytes/s",
			"DPD": "90",
			"KeepAlive": "32400",
			"Hostname": "LemonGrass",
			"Connected at": "2026-05-09 12:23",
			"raw_connected_at": 1778322239,
			"Full session": "YwDQr54wGdgKCAju+hgsYQcJLc0=",
			"Session": "YwDQr5",
			"TLS ciphersuite": "(TLS1.3)-(ECDHE-SECP256R1)",
			"DTLS cipher": "(DTLS1.2)-(PSK)-(CHACHA20-POLY1305)",
			"DNS": ["1.1.1.1", "9.9.9.9"],
			"Routes": "defaultroute",
			"Restricted to routes": "False"
		}]`,
	}
	adapter := NewOcctlAdapter("/usr/local/bin/occtl", "/var/run/occtl.socket", runner)

	conn, err := adapter.GetConnectionByID(context.Background(), 3549163)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if conn == nil {
		t.Fatal("expected non-nil connection")
	}
	if conn.Username != "maxxie" {
		t.Fatalf("expected username maxxie, got %s", conn.Username)
	}
	if conn.IPv4 != "10.10.10.208" {
		t.Fatalf("expected IPv4 10.10.10.208, got %s", conn.IPv4)
	}
	if conn.Hostname != "LemonGrass" {
		t.Fatalf("expected hostname LemonGrass, got %s", conn.Hostname)
	}
	if conn.SessionID != "YwDQr5" {
		t.Fatalf("expected session ID YwDQr5, got %s", conn.SessionID)
	}
}

func TestOcctlAdapterGetConnectionByIDNotFound(t *testing.T) {
	t.Parallel()
	runner := NewFakeCommandRunner()
	runner.Results["/usr/local/bin/occtl"] = CommandResult{
		ExitCode: 0,
		Stdout:   "",
	}
	adapter := NewOcctlAdapter("/usr/local/bin/occtl", "/var/run/occtl.socket", runner)

	conn, err := adapter.GetConnectionByID(context.Background(), 9999999)
	if err != ErrSessionNotFound {
		t.Fatalf("expected ErrSessionNotFound, got %v (conn=%v)", err, conn)
	}
}

func TestOcctlAdapterGetConnectionByIDInvalidID(t *testing.T) {
	t.Parallel()
	runner := NewFakeCommandRunner()
	adapter := NewOcctlAdapter("/usr/local/bin/occtl", "/var/run/occtl.socket", runner)

	_, err := adapter.GetConnectionByID(context.Background(), -1)
	if err == nil {
		t.Fatal("expected error for negative ID")
	}

	_, err = adapter.GetConnectionByID(context.Background(), 0)
	if err == nil {
		t.Fatal("expected error for zero ID")
	}

	if len(runner.Calls) > 0 {
		t.Error("no command should be executed for invalid connection IDs")
	}
}

// --- Tests for ListSessions text fallback ---

func TestOcctlAdapterListSessionsTextFallback(t *testing.T) {
	t.Parallel()
	runner := NewFakeCommandRunner()
	// Provide output that is NOT valid JSON to trigger the text fallback.
	runner.Results["/usr/local/bin/occtl"] = CommandResult{
		ExitCode: 0,
		Stdout:   "session     user    vhost             ip               user agent  created   status\n8Oh70G   maxxie  default  94.204.188.26 AnyConnect Android 5.1.1    2days authenticated\n",
	}
	adapter := NewOcctlAdapter("/usr/local/bin/occtl", "/var/run/occtl.socket", runner)

	sessions, err := adapter.ListSessions(context.Background())
	if err != nil {
		t.Fatalf("list sessions (text fallback): %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}
	if sessions[0].ID != "8Oh70G" {
		t.Errorf("ID: got %q, want %q", sessions[0].ID, "8Oh70G")
	}
	if sessions[0].Username != "maxxie" {
		t.Errorf("Username: got %q, want %q", sessions[0].Username, "maxxie")
	}
	if sessions[0].RemoteAddress != "94.204.188.26" {
		t.Errorf("RemoteAddress: got %q, want %q", sessions[0].RemoteAddress, "94.204.188.26")
	}
}

// --- Tests for GetSessionDetails legacy snake_case JSON fallback ---

func TestOcctlAdapterGetSessionDetailsLegacyJSON(t *testing.T) {
	t.Parallel()
	runner := NewFakeCommandRunner()
	runner.Results["/usr/local/bin/occtl"] = CommandResult{
		ExitCode: 0,
		Stdout:   `{"id":"s1","username":"alice","remote_address":"10.0.0.1","connected_at":"2026-01-15T10:30:00Z","vpn_address":"10.8.0.2","vpn_ipv4":"10.8.0.2","rx_bytes":1024,"tx_bytes":2048,"user_agent":"AnyConnect/4.10","device":"vpns0"}`,
	}
	adapter := NewOcctlAdapter("/usr/local/bin/occtl", "/var/run/occtl.socket", runner)

	detail, err := adapter.GetSessionDetails(context.Background(), "s1")
	if err != nil {
		t.Fatalf("get session details (legacy): %v", err)
	}
	if detail.Username != "alice" {
		t.Errorf("Username: got %q, want %q", detail.Username, "alice")
	}
	if detail.RemoteAddress != "10.0.0.1" {
		t.Errorf("RemoteAddress: got %q, want %q", detail.RemoteAddress, "10.0.0.1")
	}
	if detail.VPNAddress != "10.8.0.2" {
		t.Errorf("VPNAddress: got %q, want %q", detail.VPNAddress, "10.8.0.2")
	}
	if detail.RxBytes != 1024 {
		t.Errorf("RxBytes: got %d, want %d", detail.RxBytes, 1024)
	}
	if detail.ConnectedAt.IsZero() {
		t.Error("ConnectedAt should not be zero for valid RFC3339 timestamp")
	}
}

// --- Test for GetSessionDetails text fallback ---

func TestOcctlAdapterGetSessionDetailsTextFallback(t *testing.T) {
	t.Parallel()
	runner := NewFakeCommandRunner()
	// Provide non-JSON text output to trigger text parsing.
	runner.Results["/usr/local/bin/occtl"] = CommandResult{
		ExitCode: 0,
		Stdout: `Session: 8Oh70G
        Full session ID: 8Oh70Ggl2oChT0K4e/5oC+1G/zk=
        Created: 2026-05-05 11:57
        State: authenticated
        Username: maxxie   Groupname: (none)
        vhost: default   User-Agent: AnyConnect Android 5.1.15.295
        Remote IP: 94.204.188.26   Location: unknown
        In use: True   Activated: True
        Certificate auth: False
`,
	}
	adapter := NewOcctlAdapter("/usr/local/bin/occtl", "/var/run/occtl.socket", runner)

	detail, err := adapter.GetSessionDetails(context.Background(), "8Oh70G")
	if err != nil {
		t.Fatalf("get session details (text fallback): %v", err)
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
}
