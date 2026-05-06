package ocserv

import (
	"context"
	"errors"
	"testing"
)

func TestFakeManagerListsNoSessions(t *testing.T) {
	t.Parallel()

	manager := NewFakeManager()
	sessions, err := manager.ListSessions(context.Background())
	if err != nil {
		t.Fatalf("list sessions: %v", err)
	}

	if len(sessions) != 0 {
		t.Fatalf("expected no sessions, got %d", len(sessions))
	}
}

func TestFakeManagerDisconnectReturnsErrNotConfiguredByDefault(t *testing.T) {
	t.Parallel()

	manager := NewFakeManager()
	err := manager.DisconnectSession(context.Background(), "session-1")
	if !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("expected ErrNotConfigured, got %v", err)
	}
}

func TestFakeManagerDisconnectReturnsConfiguredError(t *testing.T) {
	t.Parallel()

	manager := NewFakeManager()
	manager.DisconnectErr = nil // no error configured
	err := manager.DisconnectSession(context.Background(), "session-1")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
}

func TestFakeManagerReturnsConfiguredSessions(t *testing.T) {
	t.Parallel()

	manager := NewFakeManager()
	manager.Sessions = []Session{
		{ID: "s1", Username: "alice", RemoteAddress: "10.0.0.1"},
	}
	sessions, err := manager.ListSessions(context.Background())
	if err != nil {
		t.Fatalf("list sessions: %v", err)
	}
	if len(sessions) != 1 || sessions[0].Username != "alice" {
		t.Fatalf("expected alice session, got %v", sessions)
	}
}

func TestFakeManagerGetSessionDetailsReturnsErrNotConfiguredForUnknownID(t *testing.T) {
	t.Parallel()

	manager := NewFakeManager()
	_, err := manager.GetSessionDetails(context.Background(), "unknown")
	if !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("expected ErrNotConfigured, got %v", err)
	}
}

func TestFakeManagerGetSessionDetailsReturnsConfiguredDetail(t *testing.T) {
	t.Parallel()

	manager := NewFakeManager()
	manager.SessionDetails["s1"] = &SessionDetail{
		ID:       "s1",
		Username: "alice",
	}
	detail, err := manager.GetSessionDetails(context.Background(), "s1")
	if err != nil {
		t.Fatalf("get session details: %v", err)
	}
	if detail.Username != "alice" {
		t.Fatalf("expected alice, got %s", detail.Username)
	}
}
