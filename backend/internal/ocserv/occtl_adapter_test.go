package ocserv

import (
	"context"
	"errors"
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

func TestOcctlAdapterListUsersEmpty(t *testing.T) {
	t.Parallel()
	runner := NewFakeCommandRunner()
	runner.Results["/usr/local/bin/occtl"] = CommandResult{ExitCode: 0, Stdout: "[]"}
	adapter := NewOcctlAdapter("/usr/local/bin/occtl", "/var/run/ocserv-socket", runner)

	users, err := adapter.ListUsers(context.Background())
	if err != nil {
		t.Fatalf("list users: %v", err)
	}
	if len(users) != 0 {
		t.Fatalf("expected 0 users, got %d", len(users))
	}
}

func TestOcctlAdapterListUsersWithData(t *testing.T) {
	t.Parallel()
	runner := NewFakeCommandRunner()
	runner.Results["/usr/local/bin/occtl"] = CommandResult{
		ExitCode: 0,
		Stdout:   `[{"username":"alice","group":"default","active":true}]`,
	}
	adapter := NewOcctlAdapter("/usr/local/bin/occtl", "/var/run/ocserv-socket", runner)

	users, err := adapter.ListUsers(context.Background())
	if err != nil {
		t.Fatalf("list users: %v", err)
	}
	if len(users) != 1 || users[0].Username != "alice" {
		t.Fatalf("expected alice, got %v", users)
	}
}

func TestOcctlAdapterListUsersNotConfigured(t *testing.T) {
	t.Parallel()
	runner := NewFakeCommandRunner()
	runner.Results["/usr/local/bin/occtl"] = CommandResult{
		ExitCode: 1,
		Stderr:   "connection refused",
	}
	adapter := NewOcctlAdapter("/usr/local/bin/occtl", "/var/run/ocserv-socket", runner)

	_, err := adapter.ListUsers(context.Background())
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
