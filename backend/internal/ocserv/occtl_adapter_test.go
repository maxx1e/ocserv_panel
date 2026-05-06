package ocserv

import (
	"context"
	"errors"
	"fmt"
	"testing"
)

func TestOcctlAdapterListSessionsEmpty(t *testing.T) {
	t.Parallel()
	runner := NewFakeCommandRunner()
	runner.Results["/usr/local/bin/occtl"] = CommandResult{ExitCode: 0, Stdout: "[]"}
	adapter := NewOcctlAdapter("/usr/local/bin/occtl", "/var/run/ocserv-socket", runner)

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
		Stdout:   `[{"ID":"s1","Username":"alice","RemoteAddress":"10.0.0.1","ConnectedAt":"2026-01-01T00:00:00Z"}]`,
	}
	adapter := NewOcctlAdapter("/usr/local/bin/occtl", "/var/run/ocserv-socket", runner)

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
}

func TestOcctlAdapterListSessionsNotConfigured(t *testing.T) {
	t.Parallel()
	runner := NewFakeCommandRunner()
	runner.Results["/usr/local/bin/occtl"] = CommandResult{
		ExitCode: 1,
		Stderr:   "occtl: connect: no such file or directory",
	}
	adapter := NewOcctlAdapter("/usr/local/bin/occtl", "/var/run/ocserv-socket", runner)

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
	adapter := NewOcctlAdapter("/usr/local/bin/occtl", "/var/run/ocserv-socket", runner)

	if err := adapter.DisconnectSession(context.Background(), "s1"); err != nil {
		t.Fatalf("disconnect: %v", err)
	}
}

func TestOcctlAdapterDisconnectSessionFails(t *testing.T) {
	t.Parallel()
	runner := NewFakeCommandRunner()
	runner.Results["/usr/local/bin/occtl"] = CommandResult{ExitCode: 1, Stderr: "session not found"}
	adapter := NewOcctlAdapter("/usr/local/bin/occtl", "/var/run/ocserv-socket", runner)

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
	adapter := NewOcctlAdapter("/usr/local/bin/occtl", "/var/run/ocserv-socket", runner)

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
		Stdout:   `{"id":"s1","username":"alice","remote_address":"10.0.0.1"}`,
	}
	adapter := NewOcctlAdapter("/usr/local/bin/occtl", "/var/run/ocserv-socket", runner)

	detail, err := adapter.GetSessionDetails(context.Background(), "s1")
	if err != nil {
		t.Fatalf("get session details: %v", err)
	}
	if detail.Username != "alice" {
		t.Fatalf("expected alice, got %s", detail.Username)
	}
}

func TestOcctlAdapterWithSocketInjectsFlag(t *testing.T) {
	t.Parallel()
	runner := NewFakeCommandRunner()
	runner.Results["/usr/local/bin/occtl"] = CommandResult{ExitCode: 0, Stdout: "[]"}
	adapter := NewOcctlAdapter("/usr/local/bin/occtl", "/var/run/ocserv-socket", runner)

	_, _ = adapter.ListSessions(context.Background())

	if len(runner.Calls) == 0 {
		t.Fatal("expected at least one call recorded")
	}
	call := runner.Calls[0]
	if len(call.Args) < 2 || call.Args[0] != "--socket-file" {
		t.Fatalf("expected --socket-file as first arg, got %v", call.Args)
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
	if len(call.Args) > 0 && call.Args[0] == "--socket-file" {
		t.Fatalf("expected no --socket-file when socket is empty, got %v", call.Args)
	}
}

func TestOcctlAdapterTerminateSessionSuccess(t *testing.T) {
	t.Parallel()
	runner := NewFakeCommandRunner()
	runner.Results["/usr/local/bin/occtl"] = CommandResult{ExitCode: 0, Stdout: ""}
	adapter := NewOcctlAdapter("/usr/local/bin/occtl", "/var/run/ocserv-socket", runner)

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
	adapter := NewOcctlAdapter("/usr/local/bin/occtl", "/var/run/ocserv-socket", runner)

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
	adapter := NewOcctlAdapter("/usr/local/bin/occtl", "/var/run/ocserv-socket", runner)

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
	adapter := NewOcctlAdapter("/usr/local/bin/occtl", "/var/run/ocserv-socket", runner)

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
			"id": "s1",
			"username": "alice",
			"remote_address": "203.0.113.5",
			"connected_at": "2026-01-15T10:30:00Z",
			"vpn_address": "10.8.0.2",
			"vpn_ipv4": "10.8.0.2",
			"vpn_ipv6": "fd00::2",
			"rx_bytes": 1048576,
			"tx_bytes": 2097152,
			"user_agent": "AnyConnect/4.10",
			"device": "laptop",
			"routes": ["10.0.0.0/8", "192.168.0.0/16"],
			"dns": ["8.8.8.8", "1.1.1.1"],
			"reconnect_count": 3
		}`,
	}
	adapter := NewOcctlAdapter("/usr/local/bin/occtl", "/var/run/ocserv-socket", runner)

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
	if detail.ReconnectCount != 3 {
		t.Errorf("ReconnectCount: got %d, want %d", detail.ReconnectCount, 3)
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
	adapter := NewOcctlAdapter("/usr/local/bin/occtl", "/var/run/ocserv-socket", runner)

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
	adapter := NewOcctlAdapter("/usr/local/bin/occtl", "/var/run/ocserv-socket", runner)

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
	adapter := NewOcctlAdapter("/usr/local/bin/occtl", "/var/run/ocserv-socket", runner)

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
