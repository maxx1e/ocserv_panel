package app

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/rs/zerolog"

	"ocserv_panel/backend/internal/vpn/sessions"
	"ocserv_panel/backend/internal/store"
)

// fakeIngesterStore records all upsert calls for verification.
type fakeIngesterStore struct {
	mu      sync.Mutex
	records []store.ActiveSessionRecord
	err     error
}

func (f *fakeIngesterStore) UpsertActiveSession(_ context.Context, rec store.ActiveSessionRecord) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return f.err
	}
	f.records = append(f.records, rec)
	return nil
}

func (f *fakeIngesterStore) getRecords() []store.ActiveSessionRecord {
	f.mu.Lock()
	defer f.mu.Unlock()
	cp := make([]store.ActiveSessionRecord, len(f.records))
	copy(cp, f.records)
	return cp
}

// fakeIngesterManager implements sessionIngesterManager for tests.
type fakeIngesterManager struct {
	connectedUsers    []sessions.ConnectedUser
	connectedUsersErr error
	userDetails       map[string][]sessions.UserConnection
	userDetailsErr    error
}

func (f *fakeIngesterManager) ListConnectedUsers(_ context.Context) ([]sessions.ConnectedUser, error) {
	return f.connectedUsers, f.connectedUsersErr
}

func (f *fakeIngesterManager) GetUserDetails(_ context.Context, username string) ([]sessions.UserConnection, error) {
	if f.userDetailsErr != nil {
		return nil, f.userDetailsErr
	}
	return f.userDetails[username], nil
}

func TestSessionIngester_PollUpsertsConnectedUsers(t *testing.T) {
	fixedNow := time.Date(2026, 5, 10, 12, 0, 0, 0, time.UTC)
	connectedAt := time.Date(2026, 5, 10, 10, 0, 0, 0, time.UTC) // 2 hours ago

	mgr := &fakeIngesterManager{
		connectedUsers: []sessions.ConnectedUser{
			{ID: "123", Username: "alice", Status: "connected"},
			{ID: "456", Username: "bob", Status: "connected"},
		},
		userDetails: map[string][]sessions.UserConnection{
			"alice": {
				{
					ID:          "123",
					Username:    "alice",
					SessionID:   "AbCdEf",
					ConnectedAt: connectedAt,
					RxBytes:     1024,
					TxBytes:     2048,
				},
			},
			"bob": {
				{
					ID:          "456",
					Username:    "bob",
					SessionID:   "GhIjKl",
					ConnectedAt: connectedAt.Add(30 * time.Minute),
					RxBytes:     4096,
					TxBytes:     8192,
				},
			},
		},
	}

	st := &fakeIngesterStore{}
	ingester := NewSessionIngester(st, mgr,
		IngesterWithNow(func() time.Time { return fixedNow }),
		IngesterWithLogger(zerolog.Nop()),
	)

	ctx := context.Background()
	ingester.poll(ctx)

	records := st.getRecords()
	if len(records) != 2 {
		t.Fatalf("expected 2 upserted records, got %d", len(records))
	}

	// Verify alice's record
	var aliceRec, bobRec store.ActiveSessionRecord
	for _, r := range records {
		switch r.Username {
		case "alice":
			aliceRec = r
		case "bob":
			bobRec = r
		}
	}

	if aliceRec.OcservSessionID != "AbCdEf" {
		t.Errorf("alice session ID = %q, want %q", aliceRec.OcservSessionID, "AbCdEf")
	}
	if aliceRec.BytesIn != 1024 {
		t.Errorf("alice bytes_in = %d, want %d", aliceRec.BytesIn, 1024)
	}
	if aliceRec.BytesOut != 2048 {
		t.Errorf("alice bytes_out = %d, want %d", aliceRec.BytesOut, 2048)
	}
	if aliceRec.Source != "ingested" {
		t.Errorf("alice source = %q, want %q", aliceRec.Source, "ingested")
	}
	// Duration should be 2 hours = 7200 seconds
	if aliceRec.DurationSeconds != 7200 {
		t.Errorf("alice duration_seconds = %d, want %d", aliceRec.DurationSeconds, 7200)
	}
	if !aliceRec.DisconnectedAt.Equal(fixedNow) {
		t.Errorf("alice disconnected_at = %v, want %v", aliceRec.DisconnectedAt, fixedNow)
	}

	// Verify bob's record
	if bobRec.OcservSessionID != "GhIjKl" {
		t.Errorf("bob session ID = %q, want %q", bobRec.OcservSessionID, "GhIjKl")
	}
	if bobRec.BytesIn != 4096 {
		t.Errorf("bob bytes_in = %d, want %d", bobRec.BytesIn, 4096)
	}
	// Duration should be 1.5 hours = 5400 seconds
	if bobRec.DurationSeconds != 5400 {
		t.Errorf("bob duration_seconds = %d, want %d", bobRec.DurationSeconds, 5400)
	}
}

func TestSessionIngester_PollDeduplicatesUsernames(t *testing.T) {
	fixedNow := time.Date(2026, 5, 10, 12, 0, 0, 0, time.UTC)
	connectedAt := time.Date(2026, 5, 10, 10, 0, 0, 0, time.UTC)

	// Same user with two connected entries (multiple devices)
	mgr := &fakeIngesterManager{
		connectedUsers: []sessions.ConnectedUser{
			{ID: "100", Username: "alice", Status: "connected"},
			{ID: "101", Username: "alice", Status: "connected"},
		},
		userDetails: map[string][]sessions.UserConnection{
			"alice": {
				{ID: "100", Username: "alice", SessionID: "Sess1", ConnectedAt: connectedAt, RxBytes: 100, TxBytes: 200},
				{ID: "101", Username: "alice", SessionID: "Sess2", ConnectedAt: connectedAt.Add(time.Hour), RxBytes: 300, TxBytes: 400},
			},
		},
	}

	st := &fakeIngesterStore{}
	ingester := NewSessionIngester(st, mgr,
		IngesterWithNow(func() time.Time { return fixedNow }),
		IngesterWithLogger(zerolog.Nop()),
	)

	ctx := context.Background()
	ingester.poll(ctx)

	records := st.getRecords()
	// GetUserDetails is called once for "alice" (deduplicated), but returns 2 connections.
	if len(records) != 2 {
		t.Fatalf("expected 2 records (2 sessions for same user), got %d", len(records))
	}
}

func TestSessionIngester_PollSkipsOnOcservError(t *testing.T) {
	mgr := &fakeIngesterManager{
		connectedUsersErr: sessions.ErrNotConfigured,
	}

	st := &fakeIngesterStore{}
	ingester := NewSessionIngester(st, mgr,
		IngesterWithLogger(zerolog.Nop()),
	)

	ctx := context.Background()
	ingester.poll(ctx)

	records := st.getRecords()
	if len(records) != 0 {
		t.Fatalf("expected 0 records when ocserv is unavailable, got %d", len(records))
	}
}

func TestSessionIngester_PollSkipsConnectionsWithoutSessionID(t *testing.T) {
	fixedNow := time.Date(2026, 5, 10, 12, 0, 0, 0, time.UTC)
	connectedAt := time.Date(2026, 5, 10, 10, 0, 0, 0, time.UTC)

	mgr := &fakeIngesterManager{
		connectedUsers: []sessions.ConnectedUser{
			{ID: "100", Username: "alice", Status: "connected"},
		},
		userDetails: map[string][]sessions.UserConnection{
			"alice": {
				{ID: "100", Username: "alice", SessionID: "", ConnectedAt: connectedAt, RxBytes: 100, TxBytes: 200},
			},
		},
	}

	st := &fakeIngesterStore{}
	ingester := NewSessionIngester(st, mgr,
		IngesterWithNow(func() time.Time { return fixedNow }),
		IngesterWithLogger(zerolog.Nop()),
	)

	ctx := context.Background()
	ingester.poll(ctx)

	records := st.getRecords()
	if len(records) != 0 {
		t.Fatalf("expected 0 records (no session ID), got %d", len(records))
	}
}

func TestSessionIngester_RunStopsOnContextCancel(t *testing.T) {
	mgr := &fakeIngesterManager{
		connectedUsers: []sessions.ConnectedUser{},
	}
	st := &fakeIngesterStore{}

	ingester := NewSessionIngester(st, mgr,
		IngesterWithInterval(50*time.Millisecond),
		IngesterWithLogger(zerolog.Nop()),
	)

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		ingester.Run(ctx)
		close(done)
	}()

	// Let it run for a couple ticks.
	time.Sleep(120 * time.Millisecond)
	cancel()

	select {
	case <-done:
		// Run exited as expected.
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not exit after context cancellation")
	}
}
