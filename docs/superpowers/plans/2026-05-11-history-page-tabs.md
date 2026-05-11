# History Page Tab Overhaul Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Convert the History page from a single long-scroll page into a fully tabbed layout (Monitoring, Sessions, Users, Bans, Settings), adding persistent ban history, session state tracking, and per-user traffic charts.

**Architecture:** Two database migrations add `state` to `historical_sessions` and create `ban_events`. A new `BanWatcher` goroutine polls occtl bans every 60 seconds and reconciles against SQLite. Three new API endpoints expose paginated sessions, user traffic buckets, and ban history. The frontend page is restructured around a tab bar with lazy per-tab data loading.

**Tech Stack:** Go 1.25, SQLite (via `mattn/go-sqlite3`), SvelteKit 5 (Svelte runes), Tailwind CSS v4, LayerChart / MetricChart, lucide-svelte.

---

## File Map

### Backend — new files
- `backend/internal/store/bans.go` — `BanEvent` type + store methods: `InsertBanEvent`, `CloseBanEvent`, `ListBanEvents`, `OpenBanEvents`
- `backend/internal/vpn/bans/watcher.go` — `BanWatcher` struct + `Run` loop
- `backend/internal/vpn/bans/watcher_test.go` — unit tests for BanWatcher reconciliation

### Backend — modified files
- `migrations/0006_session_state.sql` — add `state` column to `historical_sessions`
- `migrations/0007_ban_events.sql` — create `ban_events` table + indexes
- `backend/internal/store/history.go` — add `State` to `HistoricalSession`, `ActiveSessionRecord`; add `ListSessions` method; update `UpsertActiveSession` and `HistoricalSessions` SQL to include `state`; update `UserSessions` SQL to include `state`
- `backend/internal/store/store_test.go` — add tests for `ListSessions`, `InsertBanEvent`, `CloseBanEvent`, `ListBanEvents`
- `backend/internal/app/session_ingester.go` — propagate `State` field from `UserConnection` → `ActiveSessionRecord`
- `backend/internal/server/server.go` — add 3 routes + 3 handlers + 4 response types; wire `BanWatcher` dependency
- `backend/cmd/ocserv-panel/main.go` — instantiate and start `BanWatcher` goroutine

### Frontend — modified files
- `frontend/src/lib/api/history.ts` — add `HistoricalSessionPage`, `UserTrafficBucket`, `UserTrafficResponse`, `BanEvent`, `BanEventPage` types; add `listSessions`, `getUserTraffic`, `listHistoricalBans` functions
- `frontend/src/routes/history/+page.svelte` — replace section layout with tab bar + 5 lazy tab panels

---

## Task 1: Migration — session state column

**Files:**
- Create: `migrations/0006_session_state.sql`

- [ ] **Step 1: Write the migration**

```sql
-- Add state to historical_sessions for tracking session outcome.
-- Existing rows default to 'disconnected' (safe fallback).
ALTER TABLE historical_sessions ADD COLUMN state TEXT NOT NULL DEFAULT 'disconnected'
  CHECK (state IN ('authenticated', 'disconnected', 'timed_out', 'error'));
```

- [ ] **Step 2: Verify migration applies**

```bash
cd backend && go test ./internal/store/... -run TestStoreCreate -v
```
Expected: PASS (migrations run automatically via `newTestStore`)

- [ ] **Step 3: Commit**

```bash
git add migrations/0006_session_state.sql
git commit -m "feat(migrations): add state column to historical_sessions"
```

---

## Task 2: Migration — ban_events table

**Files:**
- Create: `migrations/0007_ban_events.sql`

- [ ] **Step 1: Write the migration**

```sql
-- Persistent log of IP ban events captured from ocserv.
-- unbanned_at is NULL while the ban is active.
CREATE TABLE IF NOT EXISTS ban_events (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  ip TEXT NOT NULL,
  score INTEGER NOT NULL DEFAULT 0,
  reason TEXT NOT NULL DEFAULT '',
  banned_at TEXT NOT NULL,
  unbanned_at TEXT,
  created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_ban_events_ip ON ban_events(ip);
CREATE INDEX IF NOT EXISTS idx_ban_events_banned_at ON ban_events(banned_at DESC);
```

- [ ] **Step 2: Verify migration applies**

```bash
cd backend && go test ./internal/store/... -run TestStoreCreate -v
```
Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add migrations/0007_ban_events.sql
git commit -m "feat(migrations): add ban_events table for persistent ban history"
```

---

## Task 3: Store — update HistoricalSession types and queries for state

**Files:**
- Modify: `backend/internal/store/history.go`

- [ ] **Step 1: Write failing tests for state propagation**

Add to `backend/internal/store/store_test.go`:

```go
func TestUpsertActiveSession_PersistsState(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	rec := store.ActiveSessionRecord{
		OcservSessionID: "abc123",
		Username:        "alice",
		ConnectedAt:     time.Now().UTC().Add(-time.Hour),
		DisconnectedAt:  time.Now().UTC(),
		DurationSeconds: 3600,
		BytesIn:         1000,
		BytesOut:        2000,
		Source:          "ingested",
		State:           "authenticated",
	}
	if err := s.UpsertActiveSession(ctx, rec); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	sessions, err := s.UserSessions(ctx, "alice", time.Now().UTC().Add(-2*time.Hour))
	if err != nil {
		t.Fatalf("user sessions: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("got %d sessions, want 1", len(sessions))
	}
	if sessions[0].State != "authenticated" {
		t.Errorf("state = %q, want %q", sessions[0].State, "authenticated")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd backend && go test ./internal/store/... -run TestUpsertActiveSession_PersistsState -v
```
Expected: compile error — `State` field not yet on `ActiveSessionRecord` or `HistoricalSession`

- [ ] **Step 3: Add State to types and update SQL**

In `backend/internal/store/history.go`, update `HistoricalSession`:

```go
type HistoricalSession struct {
	ID              int64
	Username        string
	ConnectedAt     time.Time
	DisconnectedAt  time.Time
	DurationSeconds int64
	BytesIn         int64
	BytesOut        int64
	Source          string
	State           string
}
```

Update `ActiveSessionRecord`:

```go
type ActiveSessionRecord struct {
	OcservSessionID string
	Username        string
	ConnectedAt     time.Time
	DisconnectedAt  time.Time
	DurationSeconds int64
	BytesIn         int64
	BytesOut        int64
	Source          string // "ingested"
	State           string // "authenticated", "disconnected", "timed_out", "error"
}
```

Update `UpsertActiveSession` — change the INSERT to include `state`:

```go
func (s *Store) UpsertActiveSession(ctx context.Context, rec ActiveSessionRecord) error {
	if rec.OcservSessionID == "" {
		return fmt.Errorf("upsert active session: ocserv_session_id must not be empty")
	}
	if rec.Username == "" {
		return fmt.Errorf("upsert active session: username must not be empty")
	}
	if rec.Source == "" {
		rec.Source = "ingested"
	}
	if rec.State == "" {
		rec.State = "disconnected"
	}

	connectedAt := rec.ConnectedAt.UTC().Format(time.RFC3339)
	disconnectedAt := rec.DisconnectedAt.UTC().Format(time.RFC3339)
	now := time.Now().UTC().Format(time.RFC3339)

	_, err := s.DB.ExecContext(ctx, `
		INSERT INTO historical_sessions (
			ocserv_session_id, username, connected_at, disconnected_at,
			duration_seconds, bytes_in, bytes_out, source, state, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(ocserv_session_id) WHERE ocserv_session_id != ''
		DO UPDATE SET
			disconnected_at = excluded.disconnected_at,
			duration_seconds = excluded.duration_seconds,
			bytes_in = excluded.bytes_in,
			bytes_out = excluded.bytes_out,
			state = excluded.state
	`,
		rec.OcservSessionID, rec.Username, connectedAt, disconnectedAt,
		rec.DurationSeconds, rec.BytesIn, rec.BytesOut, rec.Source, rec.State, now,
	)
	if err != nil {
		return fmt.Errorf("upsert active session %s: %w", rec.OcservSessionID, err)
	}
	return nil
}
```

Update `HistoricalSessions` — add `state` to SELECT and Scan:

```go
func (s *Store) HistoricalSessions(ctx context.Context, limit int) ([]HistoricalSession, error) {
	if limit <= 0 {
		return []HistoricalSession{}, nil
	}
	rows, err := s.DB.QueryContext(ctx, `SELECT id, username, connected_at, disconnected_at, duration_seconds, bytes_in, bytes_out, source, state FROM historical_sessions ORDER BY disconnected_at DESC, id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("query historical sessions: %w", err)
	}
	defer rows.Close()

	sessions := make([]HistoricalSession, 0)
	for rows.Next() {
		var session HistoricalSession
		var connectedAt, disconnectedAt string
		if err := rows.Scan(&session.ID, &session.Username, &connectedAt, &disconnectedAt, &session.DurationSeconds, &session.BytesIn, &session.BytesOut, &session.Source, &session.State); err != nil {
			return nil, fmt.Errorf("scan historical session: %w", err)
		}
		parsedConnectedAt, err := parseTime(connectedAt)
		if err != nil {
			return nil, fmt.Errorf("parse historical connected_at: %w", err)
		}
		parsedDisconnectedAt, err := parseTime(disconnectedAt)
		if err != nil {
			return nil, fmt.Errorf("parse historical disconnected_at: %w", err)
		}
		session.ConnectedAt = parsedConnectedAt
		session.DisconnectedAt = parsedDisconnectedAt
		sessions = append(sessions, session)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate historical sessions: %w", err)
	}
	return sessions, nil
}
```

Update `UserSessions` — add `state` to SELECT and Scan:

```go
func (s *Store) UserSessions(ctx context.Context, username string, since time.Time) ([]HistoricalSession, error) {
	sinceStr := since.UTC().Format(time.RFC3339)
	rows, err := s.DB.QueryContext(ctx,
		`SELECT id, username, connected_at, disconnected_at, duration_seconds, bytes_in, bytes_out, source, state
		 FROM historical_sessions
		 WHERE username = ? AND connected_at >= ?
		 ORDER BY connected_at ASC`,
		username, sinceStr,
	)
	if err != nil {
		return nil, fmt.Errorf("query user sessions: %w", err)
	}
	defer rows.Close()

	sessions := make([]HistoricalSession, 0)
	for rows.Next() {
		var session HistoricalSession
		var connectedAt, disconnectedAt string
		if err := rows.Scan(&session.ID, &session.Username, &connectedAt, &disconnectedAt, &session.DurationSeconds, &session.BytesIn, &session.BytesOut, &session.Source, &session.State); err != nil {
			return nil, fmt.Errorf("scan user session: %w", err)
		}
		parsedConnectedAt, err := parseTime(connectedAt)
		if err != nil {
			return nil, fmt.Errorf("parse user session connected_at: %w", err)
		}
		parsedDisconnectedAt, err := parseTime(disconnectedAt)
		if err != nil {
			return nil, fmt.Errorf("parse user session disconnected_at: %w", err)
		}
		session.ConnectedAt = parsedConnectedAt
		session.DisconnectedAt = parsedDisconnectedAt
		sessions = append(sessions, session)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate user sessions: %w", err)
	}
	return sessions, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

```bash
cd backend && go test ./internal/store/... -run TestUpsertActiveSession_PersistsState -v
```
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add backend/internal/store/history.go backend/internal/store/store_test.go
git commit -m "feat(store): add state field to HistoricalSession and propagate through upsert"
```

---

## Task 4: Store — ListSessions method

**Files:**
- Modify: `backend/internal/store/history.go`
- Modify: `backend/internal/store/store_test.go`

- [ ] **Step 1: Write failing test**

Add to `backend/internal/store/store_test.go`:

```go
func TestListSessions_ReturnsPagedResults(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	base := time.Now().UTC().Add(-10 * time.Hour)
	for i := 0; i < 5; i++ {
		if err := s.UpsertActiveSession(ctx, store.ActiveSessionRecord{
			OcservSessionID: fmt.Sprintf("sid%d", i),
			Username:        fmt.Sprintf("user%d", i),
			ConnectedAt:     base.Add(time.Duration(i) * time.Hour),
			DisconnectedAt:  base.Add(time.Duration(i)*time.Hour + 30*time.Minute),
			DurationSeconds: 1800,
			BytesIn:         int64(i * 1000),
			BytesOut:        int64(i * 2000),
			Source:          "ingested",
			State:           "disconnected",
		}); err != nil {
			t.Fatalf("upsert %d: %v", i, err)
		}
	}

	// Page 1: limit 3
	sessions, total, err := s.ListSessions(ctx, base.Add(-time.Hour), 3, 0)
	if err != nil {
		t.Fatalf("list sessions: %v", err)
	}
	if total != 5 {
		t.Errorf("total = %d, want 5", total)
	}
	if len(sessions) != 3 {
		t.Errorf("got %d sessions, want 3", len(sessions))
	}

	// Page 2: offset 3
	sessions2, total2, err := s.ListSessions(ctx, base.Add(-time.Hour), 3, 3)
	if err != nil {
		t.Fatalf("list sessions page 2: %v", err)
	}
	if total2 != 5 {
		t.Errorf("total2 = %d, want 5", total2)
	}
	if len(sessions2) != 2 {
		t.Errorf("got %d sessions page 2, want 2", len(sessions2))
	}
}

func TestListSessions_FiltersBySince(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	base := time.Now().UTC().Add(-48 * time.Hour)
	for i := 0; i < 3; i++ {
		if err := s.UpsertActiveSession(ctx, store.ActiveSessionRecord{
			OcservSessionID: fmt.Sprintf("fid%d", i),
			Username:        "bob",
			ConnectedAt:     base.Add(time.Duration(i*12) * time.Hour),
			DisconnectedAt:  base.Add(time.Duration(i*12)*time.Hour + time.Hour),
			DurationSeconds: 3600,
			BytesIn:         100,
			BytesOut:        200,
			Source:          "ingested",
			State:           "disconnected",
		}); err != nil {
			t.Fatalf("upsert %d: %v", i, err)
		}
	}

	// since = 25h ago cuts out the oldest session (48h ago)
	since := time.Now().UTC().Add(-25 * time.Hour)
	sessions, total, err := s.ListSessions(ctx, since, 10, 0)
	if err != nil {
		t.Fatalf("list sessions: %v", err)
	}
	if total != 2 {
		t.Errorf("total = %d, want 2", total)
	}
	if len(sessions) != 2 {
		t.Errorf("len = %d, want 2", len(sessions))
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd backend && go test ./internal/store/... -run "TestListSessions" -v
```
Expected: compile error — `ListSessions` not defined

- [ ] **Step 3: Implement ListSessions**

Add to `backend/internal/store/history.go`:

```go
// ListSessions returns historical sessions with connected_at >= since, newest
// first. Returns the total matching count alongside the paged slice.
func (s *Store) ListSessions(ctx context.Context, since time.Time, limit, offset int) ([]HistoricalSession, int64, error) {
	sinceStr := since.UTC().Format(time.RFC3339)

	var total int64
	if err := s.DB.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM historical_sessions WHERE connected_at >= ?`, sinceStr,
	).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count sessions: %w", err)
	}

	rows, err := s.DB.QueryContext(ctx,
		`SELECT id, username, connected_at, disconnected_at, duration_seconds, bytes_in, bytes_out, source, state
		 FROM historical_sessions
		 WHERE connected_at >= ?
		 ORDER BY connected_at DESC, id DESC
		 LIMIT ? OFFSET ?`,
		sinceStr, limit, offset,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("query sessions: %w", err)
	}
	defer rows.Close()

	sessions := make([]HistoricalSession, 0)
	for rows.Next() {
		var session HistoricalSession
		var connectedAt, disconnectedAt string
		if err := rows.Scan(&session.ID, &session.Username, &connectedAt, &disconnectedAt,
			&session.DurationSeconds, &session.BytesIn, &session.BytesOut, &session.Source, &session.State); err != nil {
			return nil, 0, fmt.Errorf("scan session: %w", err)
		}
		parsedConnectedAt, err := parseTime(connectedAt)
		if err != nil {
			return nil, 0, fmt.Errorf("parse connected_at: %w", err)
		}
		parsedDisconnectedAt, err := parseTime(disconnectedAt)
		if err != nil {
			return nil, 0, fmt.Errorf("parse disconnected_at: %w", err)
		}
		session.ConnectedAt = parsedConnectedAt
		session.DisconnectedAt = parsedDisconnectedAt
		sessions = append(sessions, session)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate sessions: %w", err)
	}
	return sessions, total, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd backend && go test ./internal/store/... -run "TestListSessions" -v
```
Expected: PASS

- [ ] **Step 5: Run full store tests**

```bash
cd backend && go test ./internal/store/... -v
```
Expected: all PASS

- [ ] **Step 6: Commit**

```bash
git add backend/internal/store/history.go backend/internal/store/store_test.go
git commit -m "feat(store): add ListSessions with pagination and time window filtering"
```

---

## Task 5: Store — ban event methods

**Files:**
- Create: `backend/internal/store/bans.go`
- Modify: `backend/internal/store/store_test.go`

- [ ] **Step 1: Write failing tests**

Add to `backend/internal/store/store_test.go`:

```go
func TestBanEvents_InsertAndList(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	now := time.Now().UTC()
	if err := s.InsertBanEvent(ctx, "1.2.3.4", 100, "auth failure", now); err != nil {
		t.Fatalf("insert: %v", err)
	}
	if err := s.InsertBanEvent(ctx, "5.6.7.8", 50, "too many attempts", now.Add(-time.Hour)); err != nil {
		t.Fatalf("insert 2: %v", err)
	}

	events, total, err := s.ListBanEvents(ctx, 10, 0)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if total != 2 {
		t.Errorf("total = %d, want 2", total)
	}
	if len(events) != 2 {
		t.Errorf("len = %d, want 2", len(events))
	}
	// Newest first
	if events[0].IP != "1.2.3.4" {
		t.Errorf("first IP = %q, want 1.2.3.4", events[0].IP)
	}
	if events[0].UnbannedAt != nil {
		t.Error("unbanned_at should be nil for active ban")
	}
}

func TestBanEvents_CloseEvent(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	now := time.Now().UTC()
	if err := s.InsertBanEvent(ctx, "1.2.3.4", 100, "auth failure", now); err != nil {
		t.Fatalf("insert: %v", err)
	}

	unbanTime := now.Add(time.Hour)
	if err := s.CloseBanEvent(ctx, "1.2.3.4", unbanTime); err != nil {
		t.Fatalf("close: %v", err)
	}

	events, _, err := s.ListBanEvents(ctx, 10, 0)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("len = %d, want 1", len(events))
	}
	if events[0].UnbannedAt == nil {
		t.Fatal("unbanned_at should be set")
	}
	if !events[0].UnbannedAt.Equal(unbanTime) {
		t.Errorf("unbanned_at = %v, want %v", events[0].UnbannedAt, unbanTime)
	}
}

func TestBanEvents_OpenBanEvents(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	now := time.Now().UTC()
	if err := s.InsertBanEvent(ctx, "1.2.3.4", 100, "a", now); err != nil {
		t.Fatalf("insert: %v", err)
	}
	if err := s.InsertBanEvent(ctx, "5.6.7.8", 50, "b", now); err != nil {
		t.Fatalf("insert 2: %v", err)
	}
	if err := s.CloseBanEvent(ctx, "5.6.7.8", now.Add(time.Minute)); err != nil {
		t.Fatalf("close: %v", err)
	}

	open, err := s.OpenBanEvents(ctx)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if len(open) != 1 {
		t.Fatalf("len = %d, want 1", len(open))
	}
	if open[0].IP != "1.2.3.4" {
		t.Errorf("IP = %q, want 1.2.3.4", open[0].IP)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd backend && go test ./internal/store/... -run "TestBanEvents" -v
```
Expected: compile error — types and methods not defined

- [ ] **Step 3: Create bans.go**

Create `backend/internal/store/bans.go`:

```go
package store

import (
	"context"
	"fmt"
	"time"
)

// BanEvent represents a single IP ban event recorded from ocserv.
type BanEvent struct {
	ID          int64
	IP          string
	Score       int
	Reason      string
	BannedAt    time.Time
	UnbannedAt  *time.Time
	CreatedAt   time.Time
}

// InsertBanEvent records a new ban for ip with the given score, reason, and timestamp.
func (s *Store) InsertBanEvent(ctx context.Context, ip string, score int, reason string, bannedAt time.Time) error {
	_, err := s.DB.ExecContext(ctx,
		`INSERT INTO ban_events (ip, score, reason, banned_at, created_at) VALUES (?, ?, ?, ?, ?)`,
		ip, score, reason,
		bannedAt.UTC().Format(time.RFC3339),
		time.Now().UTC().Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("insert ban event for %s: %w", ip, err)
	}
	return nil
}

// CloseBanEvent sets unbanned_at on the most recent open (unbanned_at IS NULL)
// ban event for ip.
func (s *Store) CloseBanEvent(ctx context.Context, ip string, unbanAt time.Time) error {
	_, err := s.DB.ExecContext(ctx,
		`UPDATE ban_events SET unbanned_at = ?
		 WHERE ip = ? AND unbanned_at IS NULL
		 ORDER BY banned_at DESC LIMIT 1`,
		unbanAt.UTC().Format(time.RFC3339), ip,
	)
	if err != nil {
		return fmt.Errorf("close ban event for %s: %w", ip, err)
	}
	return nil
}

// ListBanEvents returns ban events newest-first with total count.
func (s *Store) ListBanEvents(ctx context.Context, limit, offset int) ([]BanEvent, int64, error) {
	var total int64
	if err := s.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM ban_events`).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count ban events: %w", err)
	}

	rows, err := s.DB.QueryContext(ctx,
		`SELECT id, ip, score, reason, banned_at, unbanned_at, created_at
		 FROM ban_events ORDER BY banned_at DESC, id DESC LIMIT ? OFFSET ?`,
		limit, offset,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("query ban events: %w", err)
	}
	defer rows.Close()

	events := make([]BanEvent, 0)
	for rows.Next() {
		var e BanEvent
		var bannedAt, createdAt string
		var unbannedAt *string
		if err := rows.Scan(&e.ID, &e.IP, &e.Score, &e.Reason, &bannedAt, &unbannedAt, &createdAt); err != nil {
			return nil, 0, fmt.Errorf("scan ban event: %w", err)
		}
		t, err := parseTime(bannedAt)
		if err != nil {
			return nil, 0, fmt.Errorf("parse banned_at: %w", err)
		}
		e.BannedAt = t
		if unbannedAt != nil {
			ut, err := parseTime(*unbannedAt)
			if err != nil {
				return nil, 0, fmt.Errorf("parse unbanned_at: %w", err)
			}
			e.UnbannedAt = &ut
		}
		ct, err := parseTime(createdAt)
		if err != nil {
			return nil, 0, fmt.Errorf("parse created_at: %w", err)
		}
		e.CreatedAt = ct
		events = append(events, e)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate ban events: %w", err)
	}
	return events, total, nil
}

// OpenBanEvents returns all ban events where unbanned_at IS NULL.
func (s *Store) OpenBanEvents(ctx context.Context) ([]BanEvent, error) {
	rows, err := s.DB.QueryContext(ctx,
		`SELECT id, ip, score, reason, banned_at, unbanned_at, created_at
		 FROM ban_events WHERE unbanned_at IS NULL`,
	)
	if err != nil {
		return nil, fmt.Errorf("query open ban events: %w", err)
	}
	defer rows.Close()

	events := make([]BanEvent, 0)
	for rows.Next() {
		var e BanEvent
		var bannedAt, createdAt string
		var unbannedAt *string
		if err := rows.Scan(&e.ID, &e.IP, &e.Score, &e.Reason, &bannedAt, &unbannedAt, &createdAt); err != nil {
			return nil, fmt.Errorf("scan open ban event: %w", err)
		}
		t, err := parseTime(bannedAt)
		if err != nil {
			return nil, fmt.Errorf("parse banned_at: %w", err)
		}
		e.BannedAt = t
		ct, err := parseTime(createdAt)
		if err != nil {
			return nil, fmt.Errorf("parse created_at: %w", err)
		}
		e.CreatedAt = ct
		events = append(events, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate open ban events: %w", err)
	}
	return events, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd backend && go test ./internal/store/... -run "TestBanEvents" -v
```
Expected: all PASS

- [ ] **Step 5: Run full store tests**

```bash
cd backend && go test ./internal/store/... -v
```
Expected: all PASS

- [ ] **Step 6: Commit**

```bash
git add backend/internal/store/bans.go backend/internal/store/store_test.go
git commit -m "feat(store): add BanEvent type and ban event persistence methods"
```

---

## Task 6: Session ingester — propagate state

**Files:**
- Modify: `backend/internal/app/session_ingester.go`

`sessions.UserConnection` has a `State` field (from `occtl show users`, JSON key `"State"`). The ingester already builds `ActiveSessionRecord` from `UserConnection`. We just need to pass `State` through.

- [ ] **Step 1: Check UserConnection has State**

```bash
grep -n "State\|UserConnection" /opt/ocserv_panel/backend/internal/vpn/sessions/types.go | head -20
```
Expected: `UserConnection` struct with `State string` field (from occtl JSON `"State": "connected"`)

- [ ] **Step 2: Update poll() to pass State**

In `backend/internal/app/session_ingester.go`, in the `poll()` method, update the `rec` construction:

```go
rec := store.ActiveSessionRecord{
    OcservSessionID: sessionID,
    Username:        username,
    ConnectedAt:     connectedAt,
    DisconnectedAt:  now,
    DurationSeconds: durationSeconds,
    BytesIn:         conn.RxBytes,
    BytesOut:        conn.TxBytes,
    Source:          "ingested",
    State:           conn.State,
}
```

- [ ] **Step 3: Verify build**

```bash
cd backend && go build ./...
```
Expected: builds with no errors

- [ ] **Step 4: Commit**

```bash
git add backend/internal/app/session_ingester.go
git commit -m "feat(ingester): propagate session state from occtl to historical_sessions"
```

---

## Task 7: BanWatcher

**Files:**
- Create: `backend/internal/vpn/bans/watcher.go`
- Create: `backend/internal/vpn/bans/watcher_test.go`

- [ ] **Step 1: Write failing tests**

Create `backend/internal/vpn/bans/watcher_test.go`:

```go
package bans_test

import (
	"context"
	"testing"
	"time"

	"ocserv_panel/backend/internal/vpn/bans"
	"ocserv_panel/backend/internal/vpn/sessions"
)

// fakeBanStore implements bans.BanStore for tests.
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
	store := &fakeBanStore{}

	w := bans.NewBanWatcher(mgr, store)
	ctx := context.Background()
	if err := w.Sync(ctx); err != nil {
		t.Fatalf("sync: %v", err)
	}

	if len(store.inserted) != 1 {
		t.Fatalf("inserted = %d, want 1", len(store.inserted))
	}
	if store.inserted[0].IP != "1.2.3.4" {
		t.Errorf("IP = %q, want 1.2.3.4", store.inserted[0].IP)
	}
}

func TestBanWatcher_ClosesUnbannedIPs(t *testing.T) {
	mgr := &sessions.FakeManager{
		BannedIPsResult: []sessions.BannedIP{}, // no currently banned IPs
	}
	store := &fakeBanStore{
		openList: []bans.OpenBan{
			{IP: "9.9.9.9"},
		},
	}

	w := bans.NewBanWatcher(mgr, store)
	ctx := context.Background()
	if err := w.Sync(ctx); err != nil {
		t.Fatalf("sync: %v", err)
	}

	if len(store.closed) != 1 {
		t.Fatalf("closed = %d, want 1", len(store.closed))
	}
	if store.closed[0] != "9.9.9.9" {
		t.Errorf("closed IP = %q, want 9.9.9.9", store.closed[0])
	}
}

func TestBanWatcher_DoesNotDuplicateOpenBans(t *testing.T) {
	mgr := &sessions.FakeManager{
		BannedIPsResult: []sessions.BannedIP{
			{IP: "1.2.3.4", Score: 100, Reason: "auth failure"},
		},
	}
	store := &fakeBanStore{
		openList: []bans.OpenBan{
			{IP: "1.2.3.4"}, // already open
		},
	}

	w := bans.NewBanWatcher(mgr, store)
	ctx := context.Background()
	if err := w.Sync(ctx); err != nil {
		t.Fatalf("sync: %v", err)
	}

	if len(store.inserted) != 0 {
		t.Errorf("inserted = %d, want 0 (already open)", len(store.inserted))
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd backend && go test ./internal/vpn/bans/... -v
```
Expected: compile error — package not found

- [ ] **Step 3: Create watcher.go**

Create `backend/internal/vpn/bans/watcher.go`:

```go
// Package bans provides a background watcher that reconciles the live ocserv
// ban list against a persistent ban_events table in SQLite.
package bans

import (
	"context"
	"time"

	"github.com/rs/zerolog/log"

	"ocserv_panel/backend/internal/vpn/sessions"
)

const defaultTickInterval = 60 * time.Second

// OpenBan is the subset of store.BanEvent needed for reconciliation.
type OpenBan struct {
	IP string
}

// BanStore is the narrow persistence interface required by BanWatcher.
type BanStore interface {
	InsertBanEvent(ctx context.Context, ip string, score int, reason string, bannedAt time.Time) error
	CloseBanEvent(ctx context.Context, ip string, unbanAt time.Time) error
	OpenBanEvents(ctx context.Context) ([]OpenBan, error)
}

// banManager is the narrow interface for fetching live bans.
type banManager interface {
	ListBannedIPs(ctx context.Context) ([]sessions.BannedIP, error)
}

// BanWatcher polls ocserv for the live ban list and reconciles it against
// the persistent ban_events table.
type BanWatcher struct {
	manager      banManager
	store        BanStore
	tickInterval time.Duration
}

// NewBanWatcher constructs a BanWatcher. Call Run(ctx) in a goroutine.
func NewBanWatcher(manager banManager, store BanStore) *BanWatcher {
	return &BanWatcher{
		manager:      manager,
		store:        store,
		tickInterval: defaultTickInterval,
	}
}

// Run starts the poll loop. Blocks until ctx is cancelled.
func (w *BanWatcher) Run(ctx context.Context) {
	ticker := time.NewTicker(w.tickInterval)
	defer ticker.Stop()

	// Run once immediately on start.
	if err := w.Sync(ctx); err != nil {
		log.Warn().Err(err).Msg("ban watcher: initial sync failed")
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := w.Sync(ctx); err != nil {
				log.Warn().Err(err).Msg("ban watcher: sync failed")
			}
		}
	}
}

// Sync performs one reconciliation: fetches the live ban list and the open
// ban events, inserts new bans, and closes bans that have been lifted.
func (w *BanWatcher) Sync(ctx context.Context) error {
	live, err := w.manager.ListBannedIPs(ctx)
	if err != nil {
		return err
	}

	open, err := w.store.OpenBanEvents(ctx)
	if err != nil {
		return err
	}

	liveSet := make(map[string]sessions.BannedIP, len(live))
	for _, b := range live {
		liveSet[b.IP] = b
	}

	openSet := make(map[string]bool, len(open))
	for _, o := range open {
		openSet[o.IP] = true
	}

	now := time.Now().UTC()

	// Insert new bans not yet tracked.
	for _, b := range live {
		if !openSet[b.IP] {
			if err := w.store.InsertBanEvent(ctx, b.IP, b.Score, b.Reason, now); err != nil {
				log.Warn().Err(err).Str("ip", b.IP).Msg("ban watcher: insert failed")
			}
		}
	}

	// Close bans that are no longer in the live list.
	for _, o := range open {
		if !liveSet[o.IP].IP.IsValid() && liveSet[o.IP] == (sessions.BannedIP{}) {
			if _, ok := liveSet[o.IP]; !ok {
				if err := w.store.CloseBanEvent(ctx, o.IP, now); err != nil {
					log.Warn().Err(err).Str("ip", o.IP).Msg("ban watcher: close failed")
				}
			}
		}
	}

	return nil
}
```

Note: The `sessions.BannedIP` struct check above has a redundant pattern. Simplify the close loop:

```go
// Close bans that are no longer in the live list.
for _, o := range open {
    if _, stillBanned := liveSet[o.IP]; !stillBanned {
        if err := w.store.CloseBanEvent(ctx, o.IP, now); err != nil {
            log.Warn().Err(err).Str("ip", o.IP).Msg("ban watcher: close failed")
        }
    }
}
```

The final `Sync` method body:

```go
func (w *BanWatcher) Sync(ctx context.Context) error {
	live, err := w.manager.ListBannedIPs(ctx)
	if err != nil {
		return err
	}

	open, err := w.store.OpenBanEvents(ctx)
	if err != nil {
		return err
	}

	liveSet := make(map[string]sessions.BannedIP, len(live))
	for _, b := range live {
		liveSet[b.IP] = b
	}

	openSet := make(map[string]bool, len(open))
	for _, o := range open {
		openSet[o.IP] = true
	}

	now := time.Now().UTC()

	for _, b := range live {
		if !openSet[b.IP] {
			if err := w.store.InsertBanEvent(ctx, b.IP, b.Score, b.Reason, now); err != nil {
				log.Warn().Err(err).Str("ip", b.IP).Msg("ban watcher: insert failed")
			}
		}
	}

	for _, o := range open {
		if _, stillBanned := liveSet[o.IP]; !stillBanned {
			if err := w.store.CloseBanEvent(ctx, o.IP, now); err != nil {
				log.Warn().Err(err).Str("ip", o.IP).Msg("ban watcher: close failed")
			}
		}
	}

	return nil
}
```

- [ ] **Step 4: Adapt BanStore interface to match store.Store**

The `store.Store.OpenBanEvents` returns `[]store.BanEvent`, not `[]bans.OpenBan`. The `BanWatcher` uses `BanStore` with `[]OpenBan` to avoid a circular import. The `store.Store` does not implement `BanStore` directly — an adapter is needed in main.go. Add a thin adapter type to `backend/internal/vpn/bans/watcher.go`:

```go
// StoreBanAdapter adapts *store.Store to BanStore using a closure for OpenBanEvents.
// Used in main.go to wire the store to the watcher without a circular import.
// (Alternatively, main.go can implement the adapter inline.)
```

Actually, the simplest approach: define `BanStore.OpenBanEvents` to return `[]OpenBan` and write an inline adapter closure in main.go. The watcher does not import `store`. The adapter lives in main.go. The `store.Store` has `OpenBanEvents(ctx) ([]store.BanEvent, error)` — main.go wraps it.

- [ ] **Step 5: Run tests to verify they pass**

```bash
cd backend && go test ./internal/vpn/bans/... -v
```
Expected: all PASS

- [ ] **Step 6: Commit**

```bash
git add backend/internal/vpn/bans/
git commit -m "feat(bans): add BanWatcher with reconciliation logic and tests"
```

---

## Task 8: API — three new history endpoints

**Files:**
- Modify: `backend/internal/server/server.go`

- [ ] **Step 1: Add response types**

In `server.go`, add these types near the other history response types (around line 165):

```go
type listSessionsResponse struct {
	Sessions []historicalSessionResponse `json:"sessions"`
	Total    int64                       `json:"total"`
	Page     int                         `json:"page"`
	Limit    int                         `json:"limit"`
}

type trafficBucket struct {
	Timestamp time.Time `json:"timestamp"`
	BytesIn   int64     `json:"bytesIn"`
	BytesOut  int64     `json:"bytesOut"`
}

type userTrafficResponse struct {
	Username string          `json:"username"`
	Window   string          `json:"window"`
	Buckets  []trafficBucket `json:"buckets"`
}

type banEventResponse struct {
	ID         int64      `json:"id"`
	IP         string     `json:"ip"`
	Score      int        `json:"score"`
	Reason     string     `json:"reason"`
	BannedAt   time.Time  `json:"bannedAt"`
	UnbannedAt *time.Time `json:"unbannedAt"`
}

type listBanEventsResponse struct {
	Events []banEventResponse `json:"events"`
	Total  int64              `json:"total"`
	Page   int                `json:"page"`
	Limit  int                `json:"limit"`
}
```

- [ ] **Step 2: Add `state` to historicalSessionResponse**

Update the existing type:

```go
type historicalSessionResponse struct {
	ID              int64     `json:"id"`
	Username        string    `json:"username"`
	ConnectedAt     time.Time `json:"connectedAt"`
	DisconnectedAt  time.Time `json:"disconnectedAt"`
	DurationSeconds int64     `json:"durationSeconds"`
	BytesIn         int64     `json:"bytesIn"`
	BytesOut        int64     `json:"bytesOut"`
	Source          string    `json:"source"`
	State           string    `json:"state"`
}
```

Update `historyAnalyticsToResponse` (or wherever `historicalSessionResponse` is constructed from `HistoricalSession`) to include `State: s.State`. Search with:

```bash
grep -n "historicalSessionResponse{" /opt/ocserv_panel/backend/internal/server/server.go
```

Add `State: s.State` to every construction site.

- [ ] **Step 3: Add route registrations**

In `NewAppRouter`, after the existing history routes (around line 258), add:

```go
mux.Handle("GET /api/history/sessions", appHandler.requirePermission(rbac.PermissionReadDashboard)(http.HandlerFunc(appHandler.handleHistorySessions)))
mux.Handle("GET /api/history/users/{username}/traffic", appHandler.requirePermission(rbac.PermissionReadDashboard)(http.HandlerFunc(appHandler.handleHistoryUserTraffic)))
mux.Handle("GET /api/history/bans", appHandler.requirePermission(rbac.PermissionReadDashboard)(http.HandlerFunc(appHandler.handleHistoryBans)))
```

- [ ] **Step 4: Implement handleHistorySessions**

Add to `server.go`:

```go
func (a *App) handleHistorySessions(w http.ResponseWriter, r *http.Request) {
	window := queryValueDefault(r, "window", "24h")
	since := a.now().UTC().Add(-historyWindowDuration(window))

	pageStr := queryValueDefault(r, "page", "1")
	limitStr := queryValueDefault(r, "limit", "25")

	page := 1
	if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
		page = p
	}
	limit := 25
	if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 100 {
		limit = l
	}
	offset := (page - 1) * limit

	sessions, total, err := a.store.ListSessions(r.Context(), since, limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	resp := make([]historicalSessionResponse, 0, len(sessions))
	for _, s := range sessions {
		resp = append(resp, historicalSessionResponse{
			ID:              s.ID,
			Username:        s.Username,
			ConnectedAt:     s.ConnectedAt,
			DisconnectedAt:  s.DisconnectedAt,
			DurationSeconds: s.DurationSeconds,
			BytesIn:         s.BytesIn,
			BytesOut:        s.BytesOut,
			Source:          s.Source,
			State:           s.State,
		})
	}
	writeJSON(w, http.StatusOK, listSessionsResponse{
		Sessions: resp,
		Total:    total,
		Page:     page,
		Limit:    limit,
	})
}
```

Make sure `"strconv"` is imported (add to the import block if not already present).

- [ ] **Step 5: Implement handleHistoryUserTraffic**

Add to `server.go`:

```go
func (a *App) handleHistoryUserTraffic(w http.ResponseWriter, r *http.Request) {
	username := r.PathValue("username")
	if username == "" {
		writeError(w, http.StatusBadRequest, "username is required")
		return
	}

	window := queryValueDefault(r, "window", "24h")
	windowDur := historyWindowDuration(window)
	since := a.now().UTC().Add(-windowDur)

	// Determine bucket size: 1h for 24h window, 1d for 7d and 30d.
	var bucketFormat, bucketTrunc string
	var bucketDur time.Duration
	switch window {
	case "7d", "30d":
		bucketFormat = "2006-01-02"
		bucketTrunc = "day"
		bucketDur = 24 * time.Hour
	default:
		bucketFormat = "2006-01-02T15"
		bucketTrunc = "hour"
		bucketDur = time.Hour
	}
	_ = bucketTrunc // used implicitly via bucketFormat

	sessions, err := a.store.UserSessions(r.Context(), username, since)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	// Build time-bucketed aggregation.
	bucketMap := make(map[string]*trafficBucket)
	for _, s := range sessions {
		key := s.ConnectedAt.UTC().Format(bucketFormat)
		b := bucketMap[key]
		if b == nil {
			// Truncate to bucket start.
			var ts time.Time
			switch window {
			case "7d", "30d":
				y, m, d := s.ConnectedAt.UTC().Date()
				ts = time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
			default:
				ts = s.ConnectedAt.UTC().Truncate(time.Hour)
			}
			b = &trafficBucket{Timestamp: ts}
			bucketMap[key] = b
		}
		b.BytesIn += s.BytesIn
		b.BytesOut += s.BytesOut
	}

	// Build zero-filled ordered slice covering the full window.
	var buckets []trafficBucket
	numBuckets := int(windowDur / bucketDur)
	for i := 0; i < numBuckets; i++ {
		var ts time.Time
		switch window {
		case "7d", "30d":
			y, m, d := since.UTC().AddDate(0, 0, i).Date()
			ts = time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
		default:
			ts = since.UTC().Truncate(time.Hour).Add(time.Duration(i) * time.Hour)
		}
		key := ts.Format(bucketFormat)
		if b, ok := bucketMap[key]; ok {
			buckets = append(buckets, *b)
		} else {
			buckets = append(buckets, trafficBucket{Timestamp: ts})
		}
	}

	writeJSON(w, http.StatusOK, userTrafficResponse{
		Username: username,
		Window:   window,
		Buckets:  buckets,
	})
}
```

- [ ] **Step 6: Implement handleHistoryBans**

Add to `server.go`:

```go
func (a *App) handleHistoryBans(w http.ResponseWriter, r *http.Request) {
	pageStr := queryValueDefault(r, "page", "1")
	limitStr := queryValueDefault(r, "limit", "25")

	page := 1
	if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
		page = p
	}
	limit := 25
	if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 100 {
		limit = l
	}
	offset := (page - 1) * limit

	events, total, err := a.store.ListBanEvents(r.Context(), limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	resp := make([]banEventResponse, 0, len(events))
	for _, e := range events {
		resp = append(resp, banEventResponse{
			ID:         e.ID,
			IP:         e.IP,
			Score:      e.Score,
			Reason:     e.Reason,
			BannedAt:   e.BannedAt,
			UnbannedAt: e.UnbannedAt,
		})
	}
	writeJSON(w, http.StatusOK, listBanEventsResponse{
		Events: resp,
		Total:  total,
		Page:   page,
		Limit:  limit,
	})
}
```

- [ ] **Step 7: Verify build**

```bash
cd backend && go build ./...
```
Expected: builds with no errors

- [ ] **Step 8: Run full backend tests**

```bash
cd backend && go test ./... 2>&1 | tail -20
```
Expected: all PASS

- [ ] **Step 9: Commit**

```bash
git add backend/internal/server/server.go
git commit -m "feat(api): add /history/sessions, /history/users/{username}/traffic, /history/bans endpoints"
```

---

## Task 9: Wire BanWatcher in main.go

**Files:**
- Modify: `backend/cmd/ocserv-panel/main.go`

- [ ] **Step 1: Add BanWatcher import and adapter**

In `backend/cmd/ocserv-panel/main.go`, add the `vpn/bans` import:

```go
"ocserv_panel/backend/internal/vpn/bans"
```

- [ ] **Step 2: Implement the store adapter**

After `sessionIngester` is created (around line 186), add:

```go
// banStoreAdapter adapts *store.Store to bans.BanStore.
type banStoreAdapter struct {
	s *store.Store
}

func (a *banStoreAdapter) InsertBanEvent(ctx context.Context, ip string, score int, reason string, bannedAt time.Time) error {
	return a.s.InsertBanEvent(ctx, ip, score, reason, bannedAt)
}

func (a *banStoreAdapter) CloseBanEvent(ctx context.Context, ip string, unbanAt time.Time) error {
	return a.s.CloseBanEvent(ctx, ip, unbanAt)
}

func (a *banStoreAdapter) OpenBanEvents(ctx context.Context) ([]bans.OpenBan, error) {
	events, err := a.s.OpenBanEvents(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]bans.OpenBan, len(events))
	for i, e := range events {
		result[i] = bans.OpenBan{IP: e.IP}
	}
	return result, nil
}
```

Define this as a file-level type (not inside `main()`). Then inside `main()`, wire it:

```go
banWatcher := bans.NewBanWatcher(sessionManager, &banStoreAdapter{s: persistence})
go banWatcher.Run(ctx)
```

Place this after `go vpnWatcher.Run(ctx)`.

- [ ] **Step 3: Verify build**

```bash
cd backend && go build ./...
```
Expected: builds with no errors

- [ ] **Step 4: Commit**

```bash
git add backend/cmd/ocserv-panel/main.go
git commit -m "feat(main): wire BanWatcher goroutine for persistent ban event tracking"
```

---

## Task 10: Frontend API types and functions

**Files:**
- Modify: `frontend/src/lib/api/history.ts`

- [ ] **Step 1: Add new types and functions**

Replace the contents of `frontend/src/lib/api/history.ts` with:

```typescript
import { apiFetch } from './_client';

export type HistoricalSession = {
  id?: string | number;
  username?: string;
  user?: string;
  connectedAt?: string;
  disconnectedAt?: string;
  startedAt?: string;
  endedAt?: string;
  durationSeconds?: number;
  rxBytes?: number;
  txBytes?: number;
  bytesIn?: number;
  bytesOut?: number;
  remoteAddress?: string;
  state?: string;
  source?: string;
  [key: string]: unknown;
};

export type HistoryAnalytics = {
  summary?: Record<string, unknown>;
  recentSessions?: HistoricalSession[];
  sessions?: HistoricalSession[];
  sampleData?: boolean | { enabled?: boolean; present?: boolean; sessionCount?: number; message?: string };
  [key: string]: unknown;
};

export type UserSessionSummary = {
  username: string;
  sessionCount: number;
  totalDurationSeconds: number;
  totalBytesIn: number;
  totalBytesOut: number;
  avgBytesIn: number;
  avgBytesOut: number;
};

export type HistoricalSessionPage = {
  sessions: HistoricalSession[];
  total: number;
  page: number;
  limit: number;
};

export type TrafficBucket = {
  timestamp: string;
  bytesIn: number;
  bytesOut: number;
};

export type UserTrafficResponse = {
  username: string;
  window: string;
  buckets: TrafficBucket[];
};

export type BanEvent = {
  id: number;
  ip: string;
  score: number;
  reason: string;
  bannedAt: string;
  unbannedAt: string | null;
};

export type BanEventPage = {
  events: BanEvent[];
  total: number;
  page: number;
  limit: number;
};

export function getHistoryAnalytics(): Promise<HistoryAnalytics> {
  return apiFetch<HistoryAnalytics>('/api/history/analytics');
}

export function getHistoryUsers(window: string): Promise<UserSessionSummary[]> {
  return apiFetch<UserSessionSummary[]>(`/api/history/users?window=${encodeURIComponent(window)}`);
}

export function getHistoryUserSessions(username: string, window: string): Promise<HistoricalSession[]> {
  return apiFetch<HistoricalSession[]>(
    `/api/history/users/${encodeURIComponent(username)}?window=${encodeURIComponent(window)}`
  );
}

export function listSessions(window: string, page = 1, limit = 25): Promise<HistoricalSessionPage> {
  return apiFetch<HistoricalSessionPage>(
    `/api/history/sessions?window=${encodeURIComponent(window)}&page=${page}&limit=${limit}`
  );
}

export function getUserTraffic(username: string, window: string): Promise<UserTrafficResponse> {
  return apiFetch<UserTrafficResponse>(
    `/api/history/users/${encodeURIComponent(username)}/traffic?window=${encodeURIComponent(window)}`
  );
}

export function listHistoricalBans(page = 1, limit = 25): Promise<BanEventPage> {
  return apiFetch<BanEventPage>(`/api/history/bans?page=${page}&limit=${limit}`);
}

export function purgeHistory(retentionDays: 30 | 90 | 180 | 365, confirmation: 'PURGE'): Promise<void> {
  return apiFetch<void>('/api/history/purge', {
    method: 'POST',
    body: { retentionDays, confirmation }
  });
}
```

- [ ] **Step 2: Verify TypeScript compiles**

```bash
cd frontend && npx svelte-check --tsconfig ./tsconfig.json 2>&1 | tail -20
```
Expected: 0 errors

- [ ] **Step 3: Commit**

```bash
git add frontend/src/lib/api/history.ts
git commit -m "feat(frontend): add listSessions, getUserTraffic, listHistoricalBans API functions"
```

---

## Task 11: Frontend — History page tab overhaul

**Files:**
- Modify: `frontend/src/routes/history/+page.svelte`

This is the largest task. Replace the existing single-scroll page with a tab-based layout. The approach: keep all existing logic, reorganize it under tabs, add Sessions/Users/Bans tab logic.

- [ ] **Step 1: Read the current page**

```bash
wc -l /opt/ocserv_panel/frontend/src/routes/history/+page.svelte
```

Review the full file to understand all existing state variables and sections before modifying.

- [ ] **Step 2: Rewrite +page.svelte**

Replace the entire file with the tabbed version. Key structure:

```svelte
<script lang="ts">
  import { onDestroy, onMount } from 'svelte';
  import { BarChart2, List, Users, Shield, Settings } from 'lucide-svelte';
  import MetricChart from '$lib/components/ui/MetricChart.svelte';
  import CardStats from '$lib/components/ui/CardStats.svelte';
  import * as Card from '$lib/components/ui/card/index';
  import { mode } from 'mode-watcher';
  import CardTable from '$lib/components/ui/CardTable.svelte';
  import WindowSelector from '$lib/components/ui/WindowSelector.svelte';
  import UnavailableChips from '$lib/components/ui/UnavailableChips.svelte';
  import PaginatedTable from '$lib/components/ui/PaginatedTable.svelte';
  import UserSessionDrawer from '$lib/components/history/UserSessionDrawer.svelte';
  import UnbanConfirmDialog from '../sessions/UnbanConfirmDialog.svelte';
  import type { Chip } from '$lib/components/ui/UnavailableChips.svelte';
  import { formatRelativeTime, formatAbsoluteTime } from '$lib/utils/format';
  import { paginate } from '$lib/utils/pagination';
  import {
    getHistoryAnalytics, getHistoryUsers, getHistoryUserSessions,
    getMe, getMonitoringOverview, getSettings, getSetupStatus,
    hasOwnerRole, principalFromAuth, purgeHistory, updateSetting,
    listSessions, getUserTraffic, listHistoricalBans, unbanIP,
    type HistoricalSession, type HistoryAnalytics, type MonitoringOverview,
    type MonitoringSeries, type MonitoringWindow, type Principal,
    type Setting, type UserSessionSummary,
    type HistoricalSessionPage, type UserTrafficResponse, type BanEvent, type BanEventPage
  } from '$lib/api';
  import { formatBytes, metricByName, seriesByMetric } from '$lib/monitoring';

  type TabId = 'monitoring' | 'sessions' | 'users' | 'bans' | 'settings';

  const tabs: { id: TabId; label: string; icon: typeof BarChart2 }[] = [
    { id: 'monitoring', label: 'Monitoring', icon: BarChart2 },
    { id: 'sessions',   label: 'Sessions',   icon: List },
    { id: 'users',      label: 'Users',      icon: Users },
    { id: 'bans',       label: 'Bans',       icon: Shield },
    { id: 'settings',   label: 'Settings',   icon: Settings },
  ];

  // --- Shared state ---
  let activeTab = $state<TabId>('monitoring');
  let loading = $state(true);
  let error = $state('');
  let setupRequired = $state(false);
  let principal = $state<Principal | null>(null);
  const isOwner = $derived(hasOwnerRole(principal));
  let refreshInterval: ReturnType<typeof setInterval> | null = null;

  // Track which tabs have been activated (lazy load)
  let activated = $state<Record<TabId, boolean>>({ monitoring: true, sessions: false, users: false, bans: false, settings: false });

  // --- Monitoring tab state (existing) ---
  const HISTORY_WINDOWS: MonitoringWindow[] = ['24h', '7d', '30d'];
  // ... (keep all existing monitoring constants and state) ...
  let monitoringWindow = $state<MonitoringWindow>('24h');
  let monitoring = $state<MonitoringOverview | null>(null);
  let monitoringLoading = $state(false);
  let monitoringError = $state('');
  let analytics = $state<HistoryAnalytics | null>(null);

  // --- Sessions tab state ---
  let sessionsWindow = $state<MonitoringWindow>('24h');
  let sessionsPage = $state(1);
  let sessionsData = $state<HistoricalSessionPage | null>(null);
  let sessionsLoading = $state(false);
  let sessionsError = $state('');
  let sessionsSearch = $state('');

  // --- Users tab state ---
  let usersWindow = $state<MonitoringWindow>('24h');
  let userSummaries = $state<UserSessionSummary[]>([]);
  let userSummariesLoading = $state(false);
  let userSummariesError = $state('');
  let userSummariesPage = $state(1);
  let selectedUser = $state<string | null>(null);
  let userTraffic = $state<UserTrafficResponse | null>(null);
  let userTrafficLoading = $state(false);

  // --- Bans tab state ---
  let bansPage = $state(1);
  let bansData = $state<BanEventPage | null>(null);
  let bansLoading = $state(false);
  let bansError = $state('');
  let bansSearch = $state('');
  let unbanTarget = $state<string | null>(null);
  let unbanSubmitting = $state(false);
  let unbanError = $state('');

  // --- Settings tab state (existing) ---
  let retentionDays = $state<30 | 90 | 180 | 365>(90);
  let confirmation = $state('');
  let purgeSubmitting = $state(false);
  let purgeError = $state('');
  let purgeSuccess = $state('');
  let retentionSetting = $state<string>('365');
  let settingsLoading = $state(false);
  let settingsError = $state('');
  let settingsSuccess = $state('');
  const purgeEnabled = $derived(isOwner && confirmation === 'PURGE' && !purgeSubmitting);

  // --- Drawer state (shared between Sessions and Users tabs) ---
  let drawerUsername = $state<string | null>(null);
  let drawerSessions = $state<HistoricalSession[]>([]);
  let drawerLoading = $state(false);
  let drawerWindow = $state<MonitoringWindow>('24h');

  // ... (keep existing computed values: sampleWarning, totals, unavailableChips, chartDefinitions) ...

  function activateTab(tab: TabId) {
    activeTab = tab;
    if (!activated[tab]) {
      activated[tab] = true;
      loadTabData(tab);
    }
  }

  function loadTabData(tab: TabId) {
    if (tab === 'sessions') void loadSessions();
    else if (tab === 'users') void loadUserSummaries();
    else if (tab === 'bans') void loadBans();
    else if (tab === 'settings') void loadSettings();
  }

  // ... keep all existing load functions; add new ones below ...

  async function loadSessions() {
    sessionsLoading = true;
    sessionsError = '';
    try {
      sessionsData = await listSessions(sessionsWindow, sessionsPage);
    } catch (e) {
      sessionsError = e instanceof Error ? e.message : 'Failed to load sessions';
    } finally {
      sessionsLoading = false;
    }
  }

  async function loadUserSummaries() {
    userSummariesLoading = true;
    userSummariesError = '';
    try {
      userSummaries = await getHistoryUsers(usersWindow);
    } catch (e) {
      userSummariesError = e instanceof Error ? e.message : 'Failed to load users';
    } finally {
      userSummariesLoading = false;
    }
  }

  async function loadUserTraffic(username: string) {
    selectedUser = username;
    userTrafficLoading = true;
    userTraffic = null;
    try {
      userTraffic = await getUserTraffic(username, usersWindow);
    } catch {
      userTraffic = null;
    } finally {
      userTrafficLoading = false;
    }
  }

  async function loadBans() {
    bansLoading = true;
    bansError = '';
    try {
      bansData = await listHistoricalBans(bansPage);
    } catch (e) {
      bansError = e instanceof Error ? e.message : 'Failed to load ban history';
    } finally {
      bansLoading = false;
    }
  }

  async function handleUnban() {
    if (!unbanTarget) return;
    unbanSubmitting = true;
    unbanError = '';
    try {
      await unbanIP(unbanTarget);
      unbanTarget = null;
      await loadBans();
    } catch (e) {
      unbanError = e instanceof Error ? e.message : 'Unban failed';
    } finally {
      unbanSubmitting = false;
    }
  }

  async function openDrawer(username: string, window: MonitoringWindow) {
    drawerUsername = username;
    drawerWindow = window;
    drawerSessions = [];
    drawerLoading = true;
    try {
      drawerSessions = await getHistoryUserSessions(username, window);
    } catch {
      drawerSessions = [];
    } finally {
      drawerLoading = false;
    }
  }

  // ... rest of existing functions (loadHistory, reloadMonitoring, loadSettings, purge, etc.) ...

  onMount(() => {
    void loadHistory();
    refreshInterval = setInterval(() => {
      if (document.visibilityState === 'visible') {
        if (activeTab === 'monitoring') void reloadMonitoring();
        if (activeTab === 'sessions') void loadSessions();
        if (activeTab === 'users') void loadUserSummaries();
        if (activeTab === 'bans') void loadBans();
      }
    }, 15_000);
  });

  onDestroy(() => {
    if (refreshInterval) clearInterval(refreshInterval);
  });
</script>

<!-- Tab bar -->
<div class="sticky top-0 z-10 bg-white border-b border-slate-200 mb-6">
  <nav class="flex gap-1 px-4 overflow-x-auto" aria-label="History tabs">
    {#each tabs as tab}
      <button
        type="button"
        onclick={() => activateTab(tab.id)}
        class="flex items-center gap-2 px-4 py-3 text-sm font-medium border-b-2 transition-colors whitespace-nowrap
          {activeTab === tab.id
            ? 'border-blue-600 text-blue-600'
            : 'border-transparent text-slate-500 hover:text-slate-700 hover:border-slate-300'}"
        aria-selected={activeTab === tab.id}
      >
        <tab.icon class="w-4 h-4" />
        {tab.label}
      </button>
    {/each}
  </nav>
</div>

<!-- Tab panels -->
{#if activeTab === 'monitoring'}
  <!-- MONITORING TAB: paste existing monitoring section content here -->
  <!-- Keep: WindowSelector, chartDefinitions loop, UnavailableChips -->

{:else if activeTab === 'sessions'}
  <!-- SESSIONS TAB -->
  <div class="space-y-4">
    <div class="flex items-center justify-between">
      <WindowSelector windows={HISTORY_WINDOWS} bind:value={sessionsWindow}
        onchange={() => { sessionsPage = 1; void loadSessions(); }} />
      <input type="search" placeholder="Filter by username…" bind:value={sessionsSearch}
        class="px-3 py-1.5 text-sm border border-slate-200 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500" />
    </div>

    {#if sessionsLoading}
      <div class="text-sm text-slate-500 py-8 text-center">Loading sessions…</div>
    {:else if sessionsError}
      <div class="text-sm text-red-600 py-4">{sessionsError}</div>
    {:else if sessionsData}
      <Card.Root>
        <Card.Content class="p-0">
          <table class="w-full text-sm">
            <thead class="bg-slate-50 border-b border-slate-200">
              <tr>
                <th class="px-4 py-2 text-left font-medium text-slate-600">Username</th>
                <th class="px-4 py-2 text-left font-medium text-slate-600">Connected</th>
                <th class="px-4 py-2 text-left font-medium text-slate-600">Disconnected</th>
                <th class="px-4 py-2 text-left font-medium text-slate-600">Duration</th>
                <th class="px-4 py-2 text-right font-medium text-slate-600">RX</th>
                <th class="px-4 py-2 text-right font-medium text-slate-600">TX</th>
                <th class="px-4 py-2 text-center font-medium text-slate-600">State</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-slate-100">
              {#each sessionsData.sessions.filter(s => !sessionsSearch || s.username?.toLowerCase().includes(sessionsSearch.toLowerCase())) as session}
                <tr class="hover:bg-slate-50 cursor-pointer"
                    onclick={() => session.username && openDrawer(session.username, sessionsWindow)}>
                  <td class="px-4 py-2 font-mono text-xs">{session.username ?? '—'}</td>
                  <td class="px-4 py-2 text-slate-600 text-xs">{formatAbsoluteTime(session.connectedAt ?? '')}</td>
                  <td class="px-4 py-2 text-slate-600 text-xs">{formatAbsoluteTime(session.disconnectedAt ?? '')}</td>
                  <td class="px-4 py-2 text-slate-600 text-xs">{formatDuration(session.durationSeconds ?? 0)}</td>
                  <td class="px-4 py-2 text-right text-xs">{formatBytes(session.bytesIn ?? 0)}</td>
                  <td class="px-4 py-2 text-right text-xs">{formatBytes(session.bytesOut ?? 0)}</td>
                  <td class="px-4 py-2 text-center">
                    {@const stateColors: Record<string, string> = {
                      authenticated: 'bg-green-100 text-green-700',
                      disconnected:  'bg-slate-100 text-slate-600',
                      timed_out:     'bg-amber-100 text-amber-700',
                      error:         'bg-red-100 text-red-700',
                    }}
                    <span class="px-2 py-0.5 rounded-full text-xs font-medium {stateColors[session.state ?? 'disconnected'] ?? 'bg-slate-100 text-slate-600'}">
                      {session.state ?? 'disconnected'}
                    </span>
                  </td>
                </tr>
              {/each}
            </tbody>
          </table>
        </Card.Content>
      </Card.Root>
      <!-- Pagination -->
      <div class="flex items-center justify-between text-sm text-slate-500">
        <span>{sessionsData.total} total sessions</span>
        <div class="flex gap-2">
          <button disabled={sessionsPage <= 1} onclick={() => { sessionsPage--; void loadSessions(); }}
            class="px-3 py-1 rounded border disabled:opacity-40 hover:bg-slate-50">Previous</button>
          <span class="px-3 py-1">Page {sessionsPage}</span>
          <button disabled={sessionsPage * sessionsData.limit >= sessionsData.total}
            onclick={() => { sessionsPage++; void loadSessions(); }}
            class="px-3 py-1 rounded border disabled:opacity-40 hover:bg-slate-50">Next</button>
        </div>
      </div>
    {/if}
  </div>

{:else if activeTab === 'users'}
  <!-- USERS TAB -->
  <div class="space-y-6">
    <WindowSelector windows={HISTORY_WINDOWS} bind:value={usersWindow}
      onchange={() => { selectedUser = null; userTraffic = null; void loadUserSummaries(); }} />

    <Card.Root>
      <Card.Header><Card.Title>Top users by bandwidth — last {usersWindow}</Card.Title></Card.Header>
      <Card.Content class="p-0">
        {#if userSummariesLoading}
          <div class="py-8 text-center text-sm text-slate-500">Loading…</div>
        {:else if userSummariesError}
          <div class="py-4 text-sm text-red-600">{userSummariesError}</div>
        {:else}
          <table class="w-full text-sm">
            <thead class="bg-slate-50 border-b border-slate-200">
              <tr>
                <th class="px-4 py-2 text-left font-medium text-slate-600">Username</th>
                <th class="px-4 py-2 text-right font-medium text-slate-600">Sessions</th>
                <th class="px-4 py-2 text-right font-medium text-slate-600">Total RX</th>
                <th class="px-4 py-2 text-right font-medium text-slate-600">Total TX</th>
                <th class="px-4 py-2 text-right font-medium text-slate-600">Total Traffic</th>
                <th class="px-4 py-2 text-right font-medium text-slate-600">Avg RX</th>
                <th class="px-4 py-2 text-right font-medium text-slate-600">Avg TX</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-slate-100">
              {#each userSummaries as user}
                <tr class="hover:bg-slate-50 cursor-pointer {selectedUser === user.username ? 'bg-blue-50' : ''}"
                    onclick={() => loadUserTraffic(user.username)}>
                  <td class="px-4 py-2 font-mono text-xs">{user.username}</td>
                  <td class="px-4 py-2 text-right text-xs">{user.sessionCount}</td>
                  <td class="px-4 py-2 text-right text-xs">{formatBytes(user.totalBytesIn)}</td>
                  <td class="px-4 py-2 text-right text-xs">{formatBytes(user.totalBytesOut)}</td>
                  <td class="px-4 py-2 text-right text-xs font-medium">{formatBytes(user.totalBytesIn + user.totalBytesOut)}</td>
                  <td class="px-4 py-2 text-right text-xs">{formatBytes(user.avgBytesIn)}</td>
                  <td class="px-4 py-2 text-right text-xs">{formatBytes(user.avgBytesOut)}</td>
                </tr>
              {/each}
            </tbody>
          </table>
        {/if}
      </Card.Content>
    </Card.Root>

    <!-- Per-user traffic chart -->
    {#if selectedUser}
      <Card.Root>
        <Card.Header><Card.Title>Traffic — {selectedUser}</Card.Title></Card.Header>
        <Card.Content>
          {#if userTrafficLoading}
            <div class="py-8 text-center text-sm text-slate-500">Loading…</div>
          {:else if userTraffic}
            {@const rxData = userTraffic.buckets.map(b => ({ timestamp: new Date(b.timestamp), value: b.bytesIn }))}
            {@const txData = userTraffic.buckets.map(b => ({ timestamp: new Date(b.timestamp), value: b.bytesOut }))}
            <MetricChart
              title="RX"
              data={rxData}
              metric="net_interface_rx_bytes"
              available={true}
            />
            <MetricChart
              title="TX"
              data={txData}
              metric="net_interface_tx_bytes"
              available={true}
              class="mt-4"
            />
          {/if}
        </Card.Content>
      </Card.Root>
    {:else}
      <div class="py-12 text-center text-sm text-slate-400">Select a user to view their traffic over time</div>
    {/if}
  </div>

{:else if activeTab === 'bans'}
  <!-- BANS TAB -->
  <div class="space-y-4">
    <div class="flex items-center justify-between">
      <h2 class="text-base font-semibold text-slate-800">IP Ban History</h2>
      <input type="search" placeholder="Filter by IP…" bind:value={bansSearch}
        class="px-3 py-1.5 text-sm border border-slate-200 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500" />
    </div>

    {#if bansLoading}
      <div class="py-8 text-center text-sm text-slate-500">Loading…</div>
    {:else if bansError}
      <div class="py-4 text-sm text-red-600">{bansError}</div>
    {:else if bansData}
      <Card.Root>
        <Card.Content class="p-0">
          <table class="w-full text-sm">
            <thead class="bg-slate-50 border-b border-slate-200">
              <tr>
                <th class="px-4 py-2 text-left font-medium text-slate-600">IP</th>
                <th class="px-4 py-2 text-right font-medium text-slate-600">Score</th>
                <th class="px-4 py-2 text-left font-medium text-slate-600">Reason</th>
                <th class="px-4 py-2 text-left font-medium text-slate-600">Banned At</th>
                <th class="px-4 py-2 text-left font-medium text-slate-600">Unbanned At</th>
                <th class="px-4 py-2 text-center font-medium text-slate-600">Status</th>
                {#if isOwner}<th class="px-4 py-2"></th>{/if}
              </tr>
            </thead>
            <tbody class="divide-y divide-slate-100">
              {#each bansData.events.filter(e => !bansSearch || e.ip.includes(bansSearch)) as event}
                <tr class="hover:bg-slate-50">
                  <td class="px-4 py-2 font-mono text-xs">{event.ip}</td>
                  <td class="px-4 py-2 text-right text-xs">{event.score}</td>
                  <td class="px-4 py-2 text-xs text-slate-600">{event.reason || '—'}</td>
                  <td class="px-4 py-2 text-xs text-slate-600">{formatAbsoluteTime(event.bannedAt)}</td>
                  <td class="px-4 py-2 text-xs text-slate-600">{event.unbannedAt ? formatAbsoluteTime(event.unbannedAt) : '—'}</td>
                  <td class="px-4 py-2 text-center">
                    {#if event.unbannedAt}
                      <span class="px-2 py-0.5 rounded-full text-xs font-medium bg-slate-100 text-slate-600">Unbanned</span>
                    {:else}
                      <span class="px-2 py-0.5 rounded-full text-xs font-medium bg-red-100 text-red-700">Active</span>
                    {/if}
                  </td>
                  {#if isOwner}
                    <td class="px-4 py-2">
                      {#if !event.unbannedAt}
                        <button onclick={() => unbanTarget = event.ip}
                          class="text-xs text-blue-600 hover:underline">Unban</button>
                      {/if}
                    </td>
                  {/if}
                </tr>
              {/each}
            </tbody>
          </table>
        </Card.Content>
      </Card.Root>
      <!-- Pagination -->
      <div class="flex items-center justify-between text-sm text-slate-500">
        <span>{bansData.total} total ban events</span>
        <div class="flex gap-2">
          <button disabled={bansPage <= 1} onclick={() => { bansPage--; void loadBans(); }}
            class="px-3 py-1 rounded border disabled:opacity-40 hover:bg-slate-50">Previous</button>
          <span class="px-3 py-1">Page {bansPage}</span>
          <button disabled={bansPage * bansData.limit >= bansData.total}
            onclick={() => { bansPage++; void loadBans(); }}
            class="px-3 py-1 rounded border disabled:opacity-40 hover:bg-slate-50">Next</button>
        </div>
      </div>
    {/if}
  </div>

{:else if activeTab === 'settings'}
  <!-- SETTINGS TAB: paste existing retention/purge section content here -->
{/if}

<!-- Drawer (shared) -->
{#if drawerUsername}
  <UserSessionDrawer
    username={drawerUsername}
    window={drawerWindow}
    sessions={drawerSessions}
    onclose={() => drawerUsername = null}
  />
{/if}

<!-- Unban dialog -->
{#if unbanTarget}
  <UnbanConfirmDialog
    ip={unbanTarget}
    submitting={unbanSubmitting}
    error={unbanError}
    onConfirm={handleUnban}
    onCancel={() => { unbanTarget = null; unbanError = ''; }}
  />
{/if}
```

The code above is the complete structural template. Fill in the existing Monitoring and Settings tab content by moving the corresponding sections from the current page verbatim.

- [ ] **Step 3: Verify TypeScript compiles**

```bash
cd frontend && npx svelte-check --tsconfig ./tsconfig.json 2>&1 | tail -20
```
Expected: 0 errors

- [ ] **Step 4: Verify frontend build**

```bash
cd frontend && npm run build 2>&1 | tail -10
```
Expected: build succeeds

- [ ] **Step 5: Commit**

```bash
git add frontend/src/routes/history/+page.svelte
git commit -m "feat(frontend): convert history page to tabbed layout with Sessions, Users, Bans tabs"
```

---

## Task 12: Full verification

- [ ] **Step 1: Run all backend tests**

```bash
cd backend && go test ./... 2>&1
```
Expected: all PASS, no failures

- [ ] **Step 2: Run frontend type check**

```bash
cd frontend && npx svelte-check --tsconfig ./tsconfig.json 2>&1 | tail -10
```
Expected: 0 errors

- [ ] **Step 3: Run frontend build**

```bash
cd frontend && npm run build 2>&1 | tail -10
```
Expected: build succeeds

- [ ] **Step 4: Commit if any fixes were needed**

```bash
git add -p && git commit -m "fix: address verification issues"
```

---

## Self-Review Against Spec

**Spec coverage check:**

| Spec requirement | Covered by |
|---|---|
| Tab bar with 5 tabs (Monitoring, Sessions, Users, Bans, Settings) | Task 11 |
| Lazy tab activation | Task 11 (`activated` map + `activateTab`) |
| WindowSelector per-tab | Task 11 (separate `monitoringWindow`, `sessionsWindow`, `usersWindow`) |
| `state` column on `historical_sessions` | Task 1 |
| `State` field on `HistoricalSession` and `ActiveSessionRecord` | Task 3 |
| State propagated from occtl through ingester | Task 6 |
| `ListSessions` store method with pagination | Task 4 |
| `GET /api/history/sessions` endpoint | Task 8 |
| Sessions tab with state badge | Task 11 |
| `ban_events` table | Task 2 |
| `InsertBanEvent`, `CloseBanEvent`, `ListBanEvents`, `OpenBanEvents` | Task 5 |
| `BanWatcher` with Sync reconciliation | Task 7 |
| `GET /api/history/bans` endpoint | Task 8 |
| Bans tab with status badge and Unban action | Task 11 |
| Existing users endpoint reused for ranked table | Task 11 (calls `getHistoryUsers`) |
| `GET /api/history/users/{username}/traffic` endpoint | Task 8 |
| Per-user traffic chart in Users tab | Task 11 |
| Settings tab = existing retention/purge moved | Task 11 |
| Monitoring tab = existing charts moved | Task 11 |
| `UnbanConfirmDialog` reused | Task 11 |
| `UserSessionDrawer` reused | Task 11 |

All spec requirements covered.
