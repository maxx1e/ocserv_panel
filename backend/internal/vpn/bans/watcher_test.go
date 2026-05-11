package bans_test

import (
	"context"
	"testing"
	"time"

	"ocserv_panel/backend/internal/vpn/bans"
	"ocserv_panel/backend/internal/vpn/sessions"
)

type fakeBanStore struct {
	inserted []fakeInsert
	closed   []string
	openList []bans.OpenBan
}

type fakeInsert struct {
	IP     string
	Score  int
	Reason string
}

func (f *fakeBanStore) InsertBanEvent(_ context.Context, ip string, score int, reason string, _ time.Time) error {
	f.inserted = append(f.inserted, fakeInsert{IP: ip, Score: score, Reason: reason})
	return nil
}

func (f *fakeBanStore) CloseBanEvent(_ context.Context, ip string, _ time.Time) error {
	f.closed = append(f.closed, ip)
	return nil
}

func (f *fakeBanStore) OpenBanEvents(_ context.Context) ([]bans.OpenBan, error) {
	return f.openList, nil
}

func TestBanWatcher_InsertsNewBans(t *testing.T) {
	mgr := &sessions.FakeManager{
		BannedIPsResult: []sessions.BannedIP{
			{IP: "1.2.3.4", Score: 100, Reason: "auth failure"},
		},
	}
	st := &fakeBanStore{}

	w := bans.NewBanWatcher(mgr, st)
	if err := w.Sync(context.Background()); err != nil {
		t.Fatalf("sync: %v", err)
	}

	if len(st.inserted) != 1 {
		t.Fatalf("inserted = %d, want 1", len(st.inserted))
	}
	if st.inserted[0].IP != "1.2.3.4" {
		t.Errorf("IP = %q, want 1.2.3.4", st.inserted[0].IP)
	}
	if st.inserted[0].Score != 100 {
		t.Errorf("Score = %d, want 100", st.inserted[0].Score)
	}
	if st.inserted[0].Reason != "auth failure" {
		t.Errorf("Reason = %q, want %q", st.inserted[0].Reason, "auth failure")
	}
}

func TestBanWatcher_ClosesUnbannedIPs(t *testing.T) {
	mgr := &sessions.FakeManager{
		BannedIPsResult: []sessions.BannedIP{}, // no currently banned IPs
	}
	st := &fakeBanStore{
		openList: []bans.OpenBan{{IP: "9.9.9.9"}},
	}

	w := bans.NewBanWatcher(mgr, st)
	if err := w.Sync(context.Background()); err != nil {
		t.Fatalf("sync: %v", err)
	}

	if len(st.closed) != 1 {
		t.Fatalf("closed = %d, want 1", len(st.closed))
	}
	if st.closed[0] != "9.9.9.9" {
		t.Errorf("closed IP = %q, want 9.9.9.9", st.closed[0])
	}
}

func TestBanWatcher_DoesNotDuplicateOpenBans(t *testing.T) {
	mgr := &sessions.FakeManager{
		BannedIPsResult: []sessions.BannedIP{
			{IP: "1.2.3.4", Score: 100, Reason: "auth failure"},
		},
	}
	st := &fakeBanStore{
		openList: []bans.OpenBan{{IP: "1.2.3.4"}},
	}

	w := bans.NewBanWatcher(mgr, st)
	if err := w.Sync(context.Background()); err != nil {
		t.Fatalf("sync: %v", err)
	}

	if len(st.inserted) != 0 {
		t.Errorf("inserted = %d, want 0 (already open)", len(st.inserted))
	}
}

func TestBanWatcher_MultipleBansAndCloses(t *testing.T) {
	mgr := &sessions.FakeManager{
		BannedIPsResult: []sessions.BannedIP{
			{IP: "1.1.1.1", Score: 50, Reason: "brute force"},
			{IP: "2.2.2.2", Score: 80, Reason: "scanning"},
		},
	}
	st := &fakeBanStore{
		openList: []bans.OpenBan{
			{IP: "2.2.2.2"}, // already tracked
			{IP: "3.3.3.3"}, // no longer banned
		},
	}

	w := bans.NewBanWatcher(mgr, st)
	if err := w.Sync(context.Background()); err != nil {
		t.Fatalf("sync: %v", err)
	}

	// Should insert 1.1.1.1 (new), not insert 2.2.2.2 (already open)
	if len(st.inserted) != 1 {
		t.Fatalf("inserted = %d, want 1", len(st.inserted))
	}
	if st.inserted[0].IP != "1.1.1.1" {
		t.Errorf("inserted IP = %q, want 1.1.1.1", st.inserted[0].IP)
	}

	// Should close 3.3.3.3 (no longer banned)
	if len(st.closed) != 1 {
		t.Fatalf("closed = %d, want 1", len(st.closed))
	}
	if st.closed[0] != "3.3.3.3" {
		t.Errorf("closed IP = %q, want 3.3.3.3", st.closed[0])
	}
}
