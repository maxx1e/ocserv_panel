package ocserv

import (
	"context"
	"errors"
	"testing"
)

func TestNoopManagerListsNoSessions(t *testing.T) {
	t.Parallel()

	manager := NewNoopManager()
	sessions, err := manager.ListSessions(context.Background())
	if err != nil {
		t.Fatalf("list sessions: %v", err)
	}

	if len(sessions) != 0 {
		t.Fatalf("expected no sessions, got %d", len(sessions))
	}
}

func TestNoopManagerRefusesDisconnect(t *testing.T) {
	t.Parallel()

	manager := NewNoopManager()
	err := manager.DisconnectSession(context.Background(), "session-1")
	if !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("expected ErrNotConfigured, got %v", err)
	}
}
