# History Page Improvements Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Align history page monitoring charts with the dashboard, and replace the raw session table with a per-user aggregated stats view with a click-through detail drawer.

**Architecture:** Two new backend endpoints (`GET /api/history/users` and `GET /api/history/users/{username}`) return SQLite-aggregated per-user stats and per-session lists filtered by a time window. The frontend history page wires a single window selector to control both monitoring charts and the new per-user table, and adds a slide-in drawer component for per-user session charts.

**Tech Stack:** Go (net/http, database/sql, SQLite), SvelteKit 5 runes, Tailwind CSS v4, LayerChart via MetricChart component.

---

## File Map

| File | Action | Responsibility |
|------|--------|---------------|
| `backend/internal/store/history.go` | Modify | Add `UserSessionSummary` type + `UserSessionSummaries` and `UserSessions` store methods |
| `backend/internal/store/store_test.go` | Modify | Add tests for new store methods |
| `backend/internal/server/server.go` | Modify | Add response types, handlers, and route registrations |
| `backend/internal/server/server_test.go` | Modify | Add handler tests |
| `frontend/src/lib/api/history.ts` | Modify | Add `UserSessionSummary` type and two fetch functions |
| `frontend/src/lib/components/history/UserSessionDrawer.svelte` | Create | Right-side drawer with per-user session charts and session list |
| `frontend/src/routes/history/+page.svelte` | Modify | Unified window selector, replace session table with per-user table, integrate drawer |

---

## Task 1: Add store methods for per-user aggregates

**Files:**
- Modify: `backend/internal/store/history.go`
- Modify: `backend/internal/store/store_test.go`

- [ ] **Step 1: Write failing tests**

Add to `backend/internal/store/store_test.go`, after the existing history tests:

```go
func TestUserSessionSummaries(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	now := time.Now().UTC()
	insert := func(username string, connectedAt time.Time, bytesIn, bytesOut int64) {
		_, err := s.DB.ExecContext(ctx,
			`INSERT INTO historical_sessions (username, connected_at, disconnected_at, bytes_in, bytes_out, duration_seconds, source, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			username, connectedAt.Format(time.RFC3339), connectedAt.Add(time.Hour).Format(time.RFC3339),
			bytesIn, bytesOut, 3600, "ingested", now.Format(time.RFC3339),
		)
		if err != nil {
			t.Fatalf("insert session: %v", err)
		}
	}

	// Two sessions for "alice" within window
	insert("alice", now.Add(-2*time.Hour), 100, 200)
	insert("alice", now.Add(-1*time.Hour), 300, 400)
	// One session for "bob" within window
	insert("bob", now.Add(-3*time.Hour), 50, 150)
	// One old session outside window (should be excluded)
	insert("alice", now.Add(-48*time.Hour), 9999, 9999)

	since := now.Add(-24 * time.Hour)
	summaries, err := s.UserSessionSummaries(ctx, since)
	if err != nil {
		t.Fatalf("UserSessionSummaries: %v", err)
	}
	if len(summaries) != 2 {
		t.Fatalf("got %d summaries, want 2", len(summaries))
	}

	// alice has more total bytes, should be first
	alice := summaries[0]
	if alice.Username != "alice" {
		t.Fatalf("summaries[0].Username = %q, want alice", alice.Username)
	}
	if alice.SessionCount != 2 {
		t.Fatalf("alice.SessionCount = %d, want 2", alice.SessionCount)
	}
	if alice.TotalBytesIn != 400 {
		t.Fatalf("alice.TotalBytesIn = %d, want 400", alice.TotalBytesIn)
	}
	if alice.TotalBytesOut != 600 {
		t.Fatalf("alice.TotalBytesOut = %d, want 600", alice.TotalBytesOut)
	}
	if alice.AvgBytesIn != 200 {
		t.Fatalf("alice.AvgBytesIn = %d, want 200", alice.AvgBytesIn)
	}
	if alice.AvgBytesOut != 300 {
		t.Fatalf("alice.AvgBytesOut = %d, want 300", alice.AvgBytesOut)
	}

	bob := summaries[1]
	if bob.Username != "bob" {
		t.Fatalf("summaries[1].Username = %q, want bob", bob.Username)
	}
	if bob.SessionCount != 1 {
		t.Fatalf("bob.SessionCount = %d, want 1", bob.SessionCount)
	}
}

func TestUserSessions(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	now := time.Now().UTC()
	insert := func(username string, connectedAt time.Time, bytesIn, bytesOut int64) {
		_, err := s.DB.ExecContext(ctx,
			`INSERT INTO historical_sessions (username, connected_at, disconnected_at, bytes_in, bytes_out, duration_seconds, source, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			username, connectedAt.Format(time.RFC3339), connectedAt.Add(time.Hour).Format(time.RFC3339),
			bytesIn, bytesOut, 3600, "ingested", now.Format(time.RFC3339),
		)
		if err != nil {
			t.Fatalf("insert session: %v", err)
		}
	}

	insert("alice", now.Add(-2*time.Hour), 100, 200)
	insert("alice", now.Add(-1*time.Hour), 300, 400)
	insert("bob", now.Add(-1*time.Hour), 50, 150)
	// Outside window
	insert("alice", now.Add(-48*time.Hour), 9999, 9999)

	since := now.Add(-24 * time.Hour)

	aliceSessions, err := s.UserSessions(ctx, "alice", since)
	if err != nil {
		t.Fatalf("UserSessions alice: %v", err)
	}
	if len(aliceSessions) != 2 {
		t.Fatalf("alice got %d sessions, want 2", len(aliceSessions))
	}
	// Chronological order: oldest first
	if !aliceSessions[0].ConnectedAt.Before(aliceSessions[1].ConnectedAt) {
		t.Fatal("sessions not in chronological order")
	}
	for _, s := range aliceSessions {
		if s.Username != "alice" {
			t.Fatalf("session username = %q, want alice", s.Username)
		}
	}

	bobSessions, err := s.UserSessions(ctx, "bob", since)
	if err != nil {
		t.Fatalf("UserSessions bob: %v", err)
	}
	if len(bobSessions) != 1 {
		t.Fatalf("bob got %d sessions, want 1", len(bobSessions))
	}

	// Unknown user returns empty slice
	unknown, err := s.UserSessions(ctx, "nobody", since)
	if err != nil {
		t.Fatalf("UserSessions unknown: %v", err)
	}
	if len(unknown) != 0 {
		t.Fatalf("unknown user got %d sessions, want 0", len(unknown))
	}
}
```

- [ ] **Step 2: Run tests to confirm they fail**

```bash
cd /opt/ocserv_panel/backend && go test ./internal/store/ -run "TestUserSessionSummaries|TestUserSessions" -v
```
Expected: compile error — `UserSessionSummaries` and `UserSessions` not defined.

- [ ] **Step 3: Add type and methods to history.go**

Add to the bottom of `backend/internal/store/history.go`:

```go
// UserSessionSummary aggregates historical session data for a single user.
type UserSessionSummary struct {
	Username             string
	SessionCount         int64
	TotalDurationSeconds int64
	TotalBytesIn         int64
	TotalBytesOut        int64
	AvgBytesIn           int64
	AvgBytesOut          int64
}

// UserSessionSummaries returns per-user aggregated session stats for sessions
// where connected_at >= since, ordered by total bytes descending.
func (s *Store) UserSessionSummaries(ctx context.Context, since time.Time) ([]UserSessionSummary, error) {
	sinceStr := since.UTC().Format(time.RFC3339)
	rows, err := s.DB.QueryContext(ctx, `
		SELECT
			username,
			COUNT(*) AS session_count,
			COALESCE(SUM(duration_seconds), 0) AS total_duration_seconds,
			COALESCE(SUM(bytes_in), 0) AS total_bytes_in,
			COALESCE(SUM(bytes_out), 0) AS total_bytes_out,
			COALESCE(AVG(bytes_in), 0) AS avg_bytes_in,
			COALESCE(AVG(bytes_out), 0) AS avg_bytes_out
		FROM historical_sessions
		WHERE connected_at >= ?
		GROUP BY username
		ORDER BY (SUM(bytes_in) + SUM(bytes_out)) DESC
	`, sinceStr)
	if err != nil {
		return nil, fmt.Errorf("query user session summaries: %w", err)
	}
	defer rows.Close()

	summaries := make([]UserSessionSummary, 0)
	for rows.Next() {
		var sum UserSessionSummary
		if err := rows.Scan(
			&sum.Username,
			&sum.SessionCount,
			&sum.TotalDurationSeconds,
			&sum.TotalBytesIn,
			&sum.TotalBytesOut,
			&sum.AvgBytesIn,
			&sum.AvgBytesOut,
		); err != nil {
			return nil, fmt.Errorf("scan user session summary: %w", err)
		}
		summaries = append(summaries, sum)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate user session summaries: %w", err)
	}
	return summaries, nil
}

// UserSessions returns individual historical sessions for username where
// connected_at >= since, ordered chronologically (oldest first).
func (s *Store) UserSessions(ctx context.Context, username string, since time.Time) ([]HistoricalSession, error) {
	sinceStr := since.UTC().Format(time.RFC3339)
	rows, err := s.DB.QueryContext(ctx,
		`SELECT id, username, connected_at, disconnected_at, duration_seconds, bytes_in, bytes_out, source
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
		if err := rows.Scan(&session.ID, &session.Username, &connectedAt, &disconnectedAt, &session.DurationSeconds, &session.BytesIn, &session.BytesOut, &session.Source); err != nil {
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

- [ ] **Step 4: Run tests to confirm they pass**

```bash
cd /opt/ocserv_panel/backend && go test ./internal/store/ -run "TestUserSessionSummaries|TestUserSessions" -v
```
Expected: PASS (2 tests).

- [ ] **Step 5: Run all store tests to check for regressions**

```bash
cd /opt/ocserv_panel/backend && go test ./internal/store/ -v
```
Expected: all PASS.

- [ ] **Step 6: Commit**

```bash
cd /opt/ocserv_panel && git add backend/internal/store/history.go backend/internal/store/store_test.go
git commit -m "feat(store): add UserSessionSummaries and UserSessions methods"
```

---

## Task 2: Add backend API handlers and routes

**Files:**
- Modify: `backend/internal/server/server.go`
- Modify: `backend/internal/server/server_test.go`

- [ ] **Step 1: Write failing handler tests**

Add to `backend/internal/server/server_test.go` (after existing history tests, before helper functions):

```go
func TestHistoryUsersRequiresAuth(t *testing.T) {
	h := newTestHarness(t)
	h.createOwner(t, "owner", "owner-password")

	rec := serveAPI(t, h.router, http.MethodGet, "/api/history/users", nil, "")
	assertStatus(t, rec, http.StatusUnauthorized)
}

func TestHistoryUsersReturnsAggregates(t *testing.T) {
	h := newTestHarness(t)
	h.createOwner(t, "owner", "owner-password")

	// Insert two sessions for "alice"
	now := time.Now().UTC()
	for _, offset := range []time.Duration{-2 * time.Hour, -1 * time.Hour} {
		connectedAt := now.Add(offset)
		_, err := h.database.ExecContext(context.Background(),
			`INSERT INTO historical_sessions (username, connected_at, disconnected_at, bytes_in, bytes_out, duration_seconds, source, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			"alice", connectedAt.Format(time.RFC3339), connectedAt.Add(time.Hour).Format(time.RFC3339),
			100, 200, 3600, "ingested", now.Format(time.RFC3339),
		)
		if err != nil {
			t.Fatalf("insert session: %v", err)
		}
	}

	cookie := h.login(t, "owner", "owner-password")
	rec := serveAPI(t, h.router, http.MethodGet, "/api/history/users?window=24h", nil, cookie)
	assertStatus(t, rec, http.StatusOK)

	var body []map[string]any
	decodeJSON(t, rec, &body)
	if len(body) != 1 {
		t.Fatalf("got %d user summaries, want 1", len(body))
	}
	row := body[0]
	if row["username"] != "alice" {
		t.Fatalf("username = %v, want alice", row["username"])
	}
	if row["sessionCount"] != float64(2) {
		t.Fatalf("sessionCount = %v, want 2", row["sessionCount"])
	}
}

func TestHistoryUsersDefaultsTo24h(t *testing.T) {
	h := newTestHarness(t)
	h.createOwner(t, "owner", "owner-password")

	// Session within 24h
	now := time.Now().UTC()
	_, err := h.database.ExecContext(context.Background(),
		`INSERT INTO historical_sessions (username, connected_at, disconnected_at, bytes_in, bytes_out, duration_seconds, source, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		"alice", now.Add(-1*time.Hour).Format(time.RFC3339), now.Format(time.RFC3339),
		100, 200, 3600, "ingested", now.Format(time.RFC3339),
	)
	if err != nil {
		t.Fatalf("insert session: %v", err)
	}

	cookie := h.login(t, "owner", "owner-password")
	rec := serveAPI(t, h.router, http.MethodGet, "/api/history/users", nil, cookie)
	assertStatus(t, rec, http.StatusOK)

	var body []map[string]any
	decodeJSON(t, rec, &body)
	if len(body) != 1 {
		t.Fatalf("got %d summaries without window param, want 1", len(body))
	}
}

func TestHistoryUserSessionsRequiresAuth(t *testing.T) {
	h := newTestHarness(t)
	h.createOwner(t, "owner", "owner-password")

	rec := serveAPI(t, h.router, http.MethodGet, "/api/history/users/alice", nil, "")
	assertStatus(t, rec, http.StatusUnauthorized)
}

func TestHistoryUserSessionsReturnsSessionsForUser(t *testing.T) {
	h := newTestHarness(t)
	h.createOwner(t, "owner", "owner-password")

	now := time.Now().UTC()
	for i, username := range []string{"alice", "alice", "bob"} {
		connectedAt := now.Add(-time.Duration(i+1) * time.Hour)
		_, err := h.database.ExecContext(context.Background(),
			`INSERT INTO historical_sessions (username, connected_at, disconnected_at, bytes_in, bytes_out, duration_seconds, source, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			username, connectedAt.Format(time.RFC3339), connectedAt.Add(time.Hour).Format(time.RFC3339),
			100, 200, 3600, "ingested", now.Format(time.RFC3339),
		)
		if err != nil {
			t.Fatalf("insert session: %v", err)
		}
	}

	cookie := h.login(t, "owner", "owner-password")
	rec := serveAPI(t, h.router, http.MethodGet, "/api/history/users/alice?window=24h", nil, cookie)
	assertStatus(t, rec, http.StatusOK)

	var body []map[string]any
	decodeJSON(t, rec, &body)
	if len(body) != 2 {
		t.Fatalf("got %d sessions for alice, want 2", len(body))
	}
	for _, s := range body {
		if s["username"] != "alice" {
			t.Fatalf("session username = %v, want alice", s["username"])
		}
	}
}

func TestHistoryUserSessionsUnknownUserReturns404(t *testing.T) {
	h := newTestHarness(t)
	h.createOwner(t, "owner", "owner-password")

	cookie := h.login(t, "owner", "owner-password")
	rec := serveAPI(t, h.router, http.MethodGet, "/api/history/users/nobody?window=24h", nil, cookie)
	assertStatus(t, rec, http.StatusNotFound)
}

func TestHistoryUsersViewerCanRead(t *testing.T) {
	h := newTestHarness(t)
	h.createOwner(t, "owner", "owner-password")
	h.createViewer(t, "viewer", "viewer-password")

	cookie := h.login(t, "viewer", "viewer-password")
	rec := serveAPI(t, h.router, http.MethodGet, "/api/history/users", nil, cookie)
	assertStatus(t, rec, http.StatusOK)
}
```

- [ ] **Step 2: Run tests to confirm they fail**

```bash
cd /opt/ocserv_panel/backend && go test ./internal/server/ -run "TestHistoryUsers|TestHistoryUserSessions" -v
```
Expected: FAIL — routes not registered yet.

- [ ] **Step 3: Add response types to server.go**

In `backend/internal/server/server.go`, add after the `purgeHistoryResponse` struct (around line 177):

```go
type userSessionSummaryResponse struct {
	Username             string `json:"username"`
	SessionCount         int64  `json:"sessionCount"`
	TotalDurationSeconds int64  `json:"totalDurationSeconds"`
	TotalBytesIn         int64  `json:"totalBytesIn"`
	TotalBytesOut        int64  `json:"totalBytesOut"`
	AvgBytesIn           int64  `json:"avgBytesIn"`
	AvgBytesOut          int64  `json:"avgBytesOut"`
}
```

- [ ] **Step 4: Add `historyWindowDuration` helper to server.go**

Add after `queryValueDefault` function (around line 650):

```go
// historyWindowDuration converts a history window string to a time.Duration.
// Valid values: "24h", "7d", "30d". Defaults to 24h for unrecognized values.
func historyWindowDuration(window string) time.Duration {
	switch window {
	case "7d":
		return 7 * 24 * time.Hour
	case "30d":
		return 30 * 24 * time.Hour
	default:
		return 24 * time.Hour
	}
}
```

- [ ] **Step 5: Add handler methods to server.go**

Add after `handleHistoryPurge` (around line 499):

```go
func (a *App) handleHistoryUsers(w http.ResponseWriter, r *http.Request) {
	if !allowMethod(w, r, http.MethodGet) {
		return
	}
	window := queryValueDefault(r, "window", "24h")
	since := a.now().UTC().Add(-historyWindowDuration(window))
	summaries, err := a.store.UserSessionSummaries(r.Context(), since)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	resp := make([]userSessionSummaryResponse, 0, len(summaries))
	for _, s := range summaries {
		resp = append(resp, userSessionSummaryResponse{
			Username:             s.Username,
			SessionCount:         s.SessionCount,
			TotalDurationSeconds: s.TotalDurationSeconds,
			TotalBytesIn:         s.TotalBytesIn,
			TotalBytesOut:        s.TotalBytesOut,
			AvgBytesIn:           s.AvgBytesIn,
			AvgBytesOut:          s.AvgBytesOut,
		})
	}
	writeJSON(w, http.StatusOK, resp)
}

func (a *App) handleHistoryUserSessions(w http.ResponseWriter, r *http.Request) {
	if !allowMethod(w, r, http.MethodGet) {
		return
	}
	username := r.PathValue("username")
	if username == "" {
		writeError(w, http.StatusBadRequest, "username is required")
		return
	}
	window := queryValueDefault(r, "window", "24h")
	since := a.now().UTC().Add(-historyWindowDuration(window))
	sessions, err := a.store.UserSessions(r.Context(), username, since)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	if len(sessions) == 0 {
		writeError(w, http.StatusNotFound, "no sessions found for user in this window")
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
		})
	}
	writeJSON(w, http.StatusOK, resp)
}
```

- [ ] **Step 6: Check that `a.store` field exists on the App struct**

```bash
grep -n "store\s\+\*store.Store\|store \*store.Store" /opt/ocserv_panel/backend/internal/server/server.go | head -5
```

If `a.store` is not a field on `App`, find how the store is accessed (it may be via `dashboardService` or another field). If there's no direct store field, add one. Check `NewAppRouter` signature:

```bash
grep -n "func NewAppRouter\|store.*Store" /opt/ocserv_panel/backend/internal/server/server.go | head -10
```

If the store is passed to `NewAppRouter` already (look for `s *store.Store` in the parameter list), assign it to `App.store`. If not, add it as a parameter and wire it from `cmd/ocserv-panel/main.go`.

- [ ] **Step 7: Register routes in newAppRouter**

In `backend/internal/server/server.go`, add after line 241 (after the existing history routes):

```go
mux.Handle("GET /api/history/users", appHandler.requirePermission(rbac.PermissionReadDashboard)(http.HandlerFunc(appHandler.handleHistoryUsers)))
mux.Handle("GET /api/history/users/{username}", appHandler.requirePermission(rbac.PermissionReadDashboard)(http.HandlerFunc(appHandler.handleHistoryUserSessions)))
```

- [ ] **Step 8: Build to check compilation**

```bash
cd /opt/ocserv_panel/backend && go build ./...
```
Expected: no errors.

- [ ] **Step 9: Run new handler tests**

```bash
cd /opt/ocserv_panel/backend && go test ./internal/server/ -run "TestHistoryUsers|TestHistoryUserSessions" -v
```
Expected: all PASS.

- [ ] **Step 10: Run all server tests to check for regressions**

```bash
cd /opt/ocserv_panel/backend && go test ./internal/server/ -v 2>&1 | tail -20
```
Expected: all PASS.

- [ ] **Step 11: Commit**

```bash
cd /opt/ocserv_panel && git add backend/internal/server/server.go backend/internal/server/server_test.go
git commit -m "feat(api): add history users endpoints with per-user session aggregates"
```

---

## Task 3: Add frontend API functions

**Files:**
- Modify: `frontend/src/lib/api/history.ts`

- [ ] **Step 1: Add types and functions**

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

export function purgeHistory(retentionDays: 30 | 90 | 180 | 365, confirmation: 'PURGE'): Promise<void> {
  return apiFetch<void>('/api/history/purge', {
    method: 'POST',
    body: { retentionDays, confirmation }
  });
}
```

- [ ] **Step 2: Check TypeScript compilation**

```bash
cd /opt/ocserv_panel/frontend && npx tsc --noEmit 2>&1 | head -20
```
Expected: no errors related to `history.ts`.

- [ ] **Step 3: Commit**

```bash
cd /opt/ocserv_panel && git add frontend/src/lib/api/history.ts
git commit -m "feat(frontend): add getHistoryUsers and getHistoryUserSessions API functions"
```

---

## Task 4: Create UserSessionDrawer component

**Files:**
- Create: `frontend/src/lib/components/history/UserSessionDrawer.svelte`

- [ ] **Step 1: Create the directory**

```bash
mkdir -p /opt/ocserv_panel/frontend/src/lib/components/history
```

- [ ] **Step 2: Create the drawer component**

Create `frontend/src/lib/components/history/UserSessionDrawer.svelte`:

```svelte
<script lang="ts">
  import { X } from 'lucide-svelte';
  import { Chart, Bars, Axis, Svg } from 'layerchart';
  import MetricChart from '$lib/components/ui/MetricChart.svelte';
  import { formatBytes, formatMetricValue } from '$lib/monitoring';
  import { formatRelativeTime } from '$lib/utils/format';
  import type { HistoricalSession } from '$lib/api';

  type Props = {
    username: string;
    window: string;
    sessions: HistoricalSession[];
    onclose: () => void;
  };

  let { username, window, sessions, onclose }: Props = $props();

  const windowLabel = $derived(
    window === '7d' ? 'last 7 days' : window === '30d' ? 'last 30 days' : 'last 24 hours'
  );

  // Sorted chronologically for charts
  const sortedSessions = $derived(
    [...sessions].sort((a, b) => {
      const ta = Date.parse(a.connectedAt ?? a.startedAt ?? '');
      const tb = Date.parse(b.connectedAt ?? b.startedAt ?? '');
      return ta - tb;
    })
  );

  // Data for RX/TX per session chart (dual lines via MetricChart-style data)
  const rxData = $derived(
    sortedSessions
      .filter(s => Number.isFinite(Date.parse(s.connectedAt ?? s.startedAt ?? '')))
      .map(s => ({
        timestamp: new Date(s.connectedAt ?? s.startedAt ?? ''),
        value: s.bytesIn ?? s.rxBytes ?? 0
      }))
  );

  const txData = $derived(
    sortedSessions
      .filter(s => Number.isFinite(Date.parse(s.connectedAt ?? s.startedAt ?? '')))
      .map(s => ({
        timestamp: new Date(s.connectedAt ?? s.startedAt ?? ''),
        value: s.bytesOut ?? s.txBytes ?? 0
      }))
  );

  const durationData = $derived(
    sortedSessions
      .filter(s => Number.isFinite(Date.parse(s.connectedAt ?? s.startedAt ?? '')))
      .map(s => ({
        timestamp: new Date(s.connectedAt ?? s.startedAt ?? ''),
        value: Math.round((s.durationSeconds ?? 0) / 60)
      }))
  );

  const cumulativeData = $derived(() => {
    let running = 0;
    return sortedSessions
      .filter(s => Number.isFinite(Date.parse(s.connectedAt ?? s.startedAt ?? '')))
      .map(s => {
        running += (s.bytesIn ?? s.rxBytes ?? 0) + (s.bytesOut ?? s.txBytes ?? 0);
        return { timestamp: new Date(s.connectedAt ?? s.startedAt ?? ''), value: running };
      });
  });

  function sessionDuration(s: HistoricalSession): string {
    const secs = s.durationSeconds ?? 0;
    if (secs === 0) return '—';
    const h = Math.floor(secs / 3600);
    const m = Math.floor((secs % 3600) / 60);
    if (h > 0) return `${h}h ${m}m`;
    return `${m}m`;
  }

  function handleBackdropClick(e: MouseEvent) {
    if (e.target === e.currentTarget) onclose();
  }

  function handleKeydown(e: KeyboardEvent) {
    if (e.key === 'Escape') onclose();
  }
</script>

<svelte:window onkeydown={handleKeydown} />

<!-- Backdrop -->
<div
  class="fixed inset-0 bg-black/40 z-40"
  role="button"
  tabindex="-1"
  aria-label="Close drawer"
  onclick={handleBackdropClick}
  onkeydown={handleBackdropClick}
></div>

<!-- Drawer panel -->
<div
  class="fixed right-0 top-0 h-full w-full max-w-2xl bg-white shadow-xl z-50 flex flex-col overflow-hidden"
  role="dialog"
  aria-modal="true"
  aria-label="{username} session history"
>
  <!-- Header -->
  <div class="flex items-center justify-between px-6 py-4 border-b border-slate-200 flex-shrink-0">
    <div>
      <h2 class="text-lg font-semibold text-slate-800">{username}</h2>
      <p class="text-sm text-slate-500">{windowLabel}</p>
    </div>
    <button
      type="button"
      onclick={onclose}
      class="p-2 rounded-md text-slate-400 hover:text-slate-600 hover:bg-slate-100 transition-colors"
      aria-label="Close"
    >
      <X class="w-5 h-5" />
    </button>
  </div>

  <!-- Scrollable content -->
  <div class="flex-1 overflow-y-auto px-6 py-4 space-y-6">
    {#if sessions.length === 0}
      <p class="text-sm text-slate-500 text-center py-8">No sessions in this period.</p>
    {:else}
      <!-- Chart 1: Data usage per session -->
      <div>
        <h3 class="text-sm font-medium text-slate-700 mb-2">Data usage per session</h3>
        <div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
          <div>
            <p class="text-xs text-slate-500 mb-1">Download (RX)</p>
            <MetricChart data={rxData} metric="net_interface_rx_bytes" height={120} />
          </div>
          <div>
            <p class="text-xs text-slate-500 mb-1">Upload (TX)</p>
            <MetricChart data={txData} metric="net_interface_tx_bytes" height={120} />
          </div>
        </div>
      </div>

      <!-- Chart 2: Session duration -->
      <div>
        <h3 class="text-sm font-medium text-slate-700 mb-2">Session duration (minutes)</h3>
        <MetricChart data={durationData} metric="active_sessions" height={120} />
      </div>

      <!-- Chart 3: Cumulative data -->
      <div>
        <h3 class="text-sm font-medium text-slate-700 mb-2">Cumulative data transferred</h3>
        <MetricChart data={cumulativeData()} metric="net_interface_rx_bytes" height={120} />
      </div>

      <!-- Session list -->
      <div>
        <h3 class="text-sm font-medium text-slate-700 mb-2">Sessions</h3>
        <div class="overflow-x-auto rounded-lg border border-slate-200">
          <table class="w-full text-sm">
            <thead class="bg-slate-50">
              <tr class="text-left text-slate-500 font-medium text-xs">
                <th class="px-4 py-2">Started</th>
                <th class="px-4 py-2">Ended</th>
                <th class="px-4 py-2">Duration</th>
                <th class="px-4 py-2">RX</th>
                <th class="px-4 py-2">TX</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-slate-100">
              {#each sortedSessions as session (session.id ?? session.connectedAt ?? session.startedAt)}
                <tr class="hover:bg-slate-50">
                  <td class="px-4 py-2 text-slate-700" title={session.connectedAt ?? session.startedAt ?? ''}>
                    {formatRelativeTime(session.connectedAt ?? session.startedAt)}
                  </td>
                  <td class="px-4 py-2 text-slate-700" title={session.disconnectedAt ?? session.endedAt ?? ''}>
                    {formatRelativeTime(session.disconnectedAt ?? session.endedAt)}
                  </td>
                  <td class="px-4 py-2 text-slate-700">{sessionDuration(session)}</td>
                  <td class="px-4 py-2 text-slate-700">{formatBytes(session.bytesIn ?? session.rxBytes ?? 0)}</td>
                  <td class="px-4 py-2 text-slate-700">{formatBytes(session.bytesOut ?? session.txBytes ?? 0)}</td>
                </tr>
              {/each}
            </tbody>
          </table>
        </div>
      </div>
    {/if}
  </div>
</div>
```

- [ ] **Step 3: Check TypeScript compilation**

```bash
cd /opt/ocserv_panel/frontend && npx tsc --noEmit 2>&1 | head -20
```
Expected: no new errors.

- [ ] **Step 4: Commit**

```bash
cd /opt/ocserv_panel && git add frontend/src/lib/components/history/UserSessionDrawer.svelte
git commit -m "feat(frontend): add UserSessionDrawer component with session charts"
```

---

## Task 5: Update history page

**Files:**
- Modify: `frontend/src/routes/history/+page.svelte`

The history page needs three changes:
1. Window selector drives both monitoring AND the user stats table
2. Replace the raw session table with the per-user aggregated stats table
3. Wire the drawer on row click

- [ ] **Step 1: Update the script section**

Replace the `<script lang="ts">` block (lines 1–259) of `frontend/src/routes/history/+page.svelte` with:

```svelte
<script lang="ts">
  import { onDestroy, onMount } from 'svelte';
  import MetricChart from '$lib/components/ui/MetricChart.svelte';
  import { ArrowDown, ArrowUp, Server, Users } from 'lucide-svelte';

  import CardStats from '$lib/components/ui/CardStats.svelte';
  import * as Card from '$lib/components/ui/card/index';
  import { mode } from 'mode-watcher';
  import CardTable from '$lib/components/ui/CardTable.svelte';
  import CardSettings from '$lib/components/ui/CardSettings.svelte';
  import WindowSelector from '$lib/components/ui/WindowSelector.svelte';
  import UnavailableChips from '$lib/components/ui/UnavailableChips.svelte';
  import PaginatedTable from '$lib/components/ui/PaginatedTable.svelte';
  import UserSessionDrawer from '$lib/components/history/UserSessionDrawer.svelte';
  import type { Chip } from '$lib/components/ui/UnavailableChips.svelte';
  import { formatRelativeTime, formatAbsoluteTime } from '$lib/utils/format';
  import { paginate } from '$lib/utils/pagination';

  import {
    getHistoryAnalytics,
    getHistoryUsers,
    getHistoryUserSessions,
    getMe,
    getMonitoringOverview,
    getSettings,
    getSetupStatus,
    hasOwnerRole,
    principalFromAuth,
    purgeHistory,
    updateSetting,
    type HistoricalSession,
    type HistoryAnalytics,
    type MonitoringOverview,
    type MonitoringSeries,
    type MonitoringWindow,
    type Principal,
    type Setting,
    type UserSessionSummary
  } from '$lib/api';
  import { formatBytes, metricByName, seriesByMetric } from '$lib/monitoring';

  const HISTORY_WINDOWS: MonitoringWindow[] = ['24h', '7d', '30d'];
  const HOST_CPU = 'host_cpu_utilization';
  const HOST_MEMORY = 'host_memory_utilization';
  const NET_RX = 'net_interface_rx_bytes';
  const NET_TX = 'net_interface_tx_bytes';
  const VPN_RX = 'vpn_interface_rx_bytes';
  const VPN_TX = 'vpn_interface_tx_bytes';
  const CONNECTION_RATE = 'connection_rate';
  const ACTIVE_SESSIONS = 'active_sessions';
  const CONCURRENT_SESSIONS = 'concurrent_sessions';

  type ChartDefinition = {
    metric: string;
    title: string;
  };

  const chartDefinitions: ChartDefinition[] = [
    { metric: HOST_CPU, title: 'CPU utilization' },
    { metric: HOST_MEMORY, title: 'Memory utilization' },
    { metric: NET_RX, title: 'Network inbound' },
    { metric: NET_TX, title: 'Network outbound' },
    { metric: VPN_RX, title: 'VPN inbound' },
    { metric: VPN_TX, title: 'VPN outbound' },
    { metric: ACTIVE_SESSIONS, title: 'Active sessions' },
    { metric: CONNECTION_RATE, title: 'Connection rate' },
    { metric: CONCURRENT_SESSIONS, title: 'Concurrent sessions' }
  ];

  let loading = $state(true);
  let error = $state('');
  let purgeError = $state('');
  let purgeSuccess = $state('');
  let setupRequired = $state(false);
  let principal = $state<Principal | null>(null);
  let analytics = $state<HistoryAnalytics | null>(null);
  let monitoringWindow = $state<MonitoringWindow>('24h');
  let monitoring = $state<MonitoringOverview | null>(null);
  let monitoringLoading = $state(false);
  let monitoringError = $state('');
  let retentionDays = $state<30 | 90 | 180 | 365>(90);
  let confirmation = $state('');
  let purgeSubmitting = $state(false);
  let monitoringRequestId = 0;
  let refreshInterval: ReturnType<typeof setInterval> | null = null;
  let retentionSetting = $state<string>('365');
  let settingsLoading = $state(false);
  let settingsError = $state('');
  let settingsSuccess = $state('');

  // Per-user table state
  let userSummaries = $state<UserSessionSummary[]>([]);
  let userSummariesLoading = $state(false);
  let userSummariesError = $state('');
  let userSummariesPage = $state(1);

  // Drawer state
  let drawerUsername = $state<string | null>(null);
  let drawerSessions = $state<HistoricalSession[]>([]);
  let drawerLoading = $state(false);

  const isOwner = $derived(hasOwnerRole(principal));
  const sampleWarning = $derived(sampleDataWarning(analytics));
  const totals = $derived(calculateTotals(analytics?.summary));
  const purgeEnabled = $derived(isOwner && confirmation === 'PURGE' && !purgeSubmitting);
  const pagedUserSummaries = $derived(paginate(userSummaries, userSummariesPage, 10));
  const unavailableChips = $derived<Chip[]>(
    chartDefinitions
      .filter(chart => !shouldShowChart(chart.metric))
      .map(chart => ({ name: chart.title, reason: unavailableReason(chart.metric) }))
  );

  onMount(() => {
    void loadHistory();
    refreshInterval = setInterval(() => {
      if (document.visibilityState === 'visible') {
        void reloadMonitoring();
        void reloadUserSummaries();
      }
    }, 15_000);
  });

  onDestroy(() => {
    if (refreshInterval) {
      clearInterval(refreshInterval);
    }
  });

  async function loadHistory() {
    loading = true;
    error = '';
    try {
      const setup = await getSetupStatus();
      setupRequired = setup.required;
      if (setup.required) return;

      principal = principalFromAuth(await getMe());
      analytics = await getHistoryAnalytics();
      void reloadMonitoring(monitoringWindow);
      void reloadUserSummaries(monitoringWindow);
      void loadSettings();
    } catch (caught) {
      error = caught instanceof Error ? caught.message : 'Unable to load historical analytics.';
      principal = null;
    } finally {
      loading = false;
    }
  }

  async function loadSettings() {
    settingsLoading = true;
    try {
      const response = await getSettings();
      const retention = response.settings.find((s: Setting) => s.key === 'history.retention_days');
      if (retention) {
        retentionSetting = retention.value;
      }
    } catch {
      // Settings are optional
    } finally {
      settingsLoading = false;
    }
  }

  async function handleSaveRetention(event: SubmitEvent) {
    event.preventDefault();
    settingsError = '';
    settingsSuccess = '';
    try {
      await updateSetting('history.retention_days', retentionSetting);
      settingsSuccess = 'Retention setting saved.';
    } catch (caught) {
      settingsError = caught instanceof Error ? caught.message : 'Failed to save retention setting.';
    }
  }

  async function reloadMonitoring(window: MonitoringWindow = monitoringWindow) {
    const requestId = ++monitoringRequestId;
    monitoringLoading = true;
    monitoringError = '';
    try {
      const overview = await getMonitoringOverview(window);
      if (!isLatestMonitoringRequest(requestId, window)) return;
      monitoring = overview;
    } catch (caught) {
      if (!isLatestMonitoringRequest(requestId, window)) return;
      monitoringError = caught instanceof Error ? caught.message : 'Unable to load monitoring overview.';
      monitoring = null;
    } finally {
      if (isLatestMonitoringRequest(requestId, window)) {
        monitoringLoading = false;
      }
    }
  }

  async function reloadUserSummaries(window: MonitoringWindow = monitoringWindow) {
    userSummariesPage = 1;
    userSummariesLoading = true;
    userSummariesError = '';
    try {
      userSummaries = await getHistoryUsers(window);
    } catch (caught) {
      userSummariesError = caught instanceof Error ? caught.message : 'Unable to load user session summaries.';
      userSummaries = [];
    } finally {
      userSummariesLoading = false;
    }
  }

  function isLatestMonitoringRequest(requestId: number, window: MonitoringWindow): boolean {
    return requestId === monitoringRequestId && window === monitoringWindow;
  }

  function handleWindowChange(nextWindow: string) {
    const window = nextWindow as MonitoringWindow;
    if (!HISTORY_WINDOWS.includes(window) || window === monitoringWindow) return;
    monitoringWindow = window;
    void reloadMonitoring(window);
    void reloadUserSummaries(window);
  }

  async function handleUserRowClick(username: string) {
    drawerUsername = username;
    drawerSessions = [];
    drawerLoading = true;
    try {
      drawerSessions = await getHistoryUserSessions(username, monitoringWindow);
    } catch {
      drawerSessions = [];
    } finally {
      drawerLoading = false;
    }
  }

  function closeDrawer() {
    drawerUsername = null;
    drawerSessions = [];
  }

  function metricSeries(metric: string): MonitoringSeries | undefined {
    return seriesByMetric(monitoring, metric);
  }

  function unavailableReason(metric: string): string {
    const catalogMetric = metricByName(monitoring?.catalog, metric);
    if (catalogMetric?.reason) return catalogMetric.reason;
    if (catalogMetric?.state === 'unavailable') return 'The backend reports this metric as unavailable.';
    return 'Metric data has not been collected for this window yet.';
  }

  function shouldShowChart(metric: string): boolean {
    if (metricByName(monitoring?.catalog, metric)?.state === 'unavailable') return false;
    return chartData(metric).length > 0;
  }

  async function handlePurge(event: SubmitEvent) {
    event.preventDefault();
    purgeError = '';
    purgeSuccess = '';
    if (!purgeEnabled) return;
    purgeSubmitting = true;
    try {
      await purgeHistory(retentionDays, 'PURGE');
      confirmation = '';
      purgeSuccess = 'History purge completed. Analytics were refreshed.';
      await loadHistory();
    } catch (caught) {
      purgeError = caught instanceof Error ? caught.message : 'History purge failed.';
    } finally {
      purgeSubmitting = false;
    }
  }

  function sampleDataWarning(value: HistoryAnalytics | null): string {
    const sampleData = value?.sampleData;
    if (typeof sampleData === 'boolean') return sampleData ? 'Sample historical sessions are present.' : '';
    if (sampleData?.enabled || sampleData?.present || Number(sampleData?.sessionCount ?? 0) > 0) {
      return sampleData?.message ?? 'Sample historical sessions are present.';
    }
    return '';
  }

  function calculateTotals(summary: Record<string, unknown> | undefined) {
    const sessionCount = numberFrom(summary?.sessionCount) ?? numberFrom(summary?.totalSessions) ?? 0;
    const rxBytes = numberFrom(summary?.rxBytes) ?? numberFrom(summary?.bytesIn) ?? 0;
    const txBytes = numberFrom(summary?.txBytes) ?? numberFrom(summary?.bytesOut) ?? 0;
    return { sessionCount, rxBytes, txBytes, totalBytes: rxBytes + txBytes };
  }

  function numberFrom(value: unknown): number | null {
    return typeof value === 'number' && Number.isFinite(value) ? value : null;
  }

  function chartData(metric: string): { timestamp: Date; value: number }[] {
    const series = metricSeries(metric);
    if (!series) return [];
    return series.points
      .filter(p => Number.isFinite(Date.parse(p.timestamp)) && Number.isFinite(p.value))
      .map(p => ({ timestamp: new Date(p.timestamp), value: p.value }))
      .sort((a, b) => a.timestamp.getTime() - b.timestamp.getTime());
  }

  function formatDuration(secs: number): string {
    if (secs === 0) return '—';
    const d = Math.floor(secs / 86400);
    const h = Math.floor((secs % 86400) / 3600);
    const m = Math.floor((secs % 3600) / 60);
    if (d > 0) return `${d}d ${h}h`;
    if (h > 0) return `${h}h ${m}m`;
    return `${m}m`;
  }
</script>
```

- [ ] **Step 2: Update the template section**

In the template (HTML) section of `frontend/src/routes/history/+page.svelte`, make these changes:

**2a.** The window selector currently sits inside the "Monitoring history" section header. Move it up to a shared position above both sections, or keep it in the monitoring header and pass it down. The simplest approach: keep the `WindowSelector` in the monitoring section header (already there), but call `handleWindowChange` instead of `handleMonitoringWindowChange`.

Replace (in the monitoring section header):
```svelte
        <WindowSelector
          value={monitoringWindow}
          windows={HISTORY_WINDOWS}
          onchange={handleMonitoringWindowChange}
        />
```
With:
```svelte
        <WindowSelector
          value={monitoringWindow}
          windows={HISTORY_WINDOWS}
          onchange={handleWindowChange}
        />
```

**2b.** Replace the entire "Recent historical sessions" section (from `<!-- Recent historical sessions -->` through the closing `</section>`) with:

```svelte
  <!-- User session summaries -->
  <section aria-labelledby="sessions-heading" class="mb-8">
    <div class="flex items-center justify-between mb-4">
      <h2 id="sessions-heading" class="text-lg font-semibold text-slate-700">User sessions</h2>
    </div>

    {#if userSummariesError}
      <div class="bg-red-50 border border-red-200 rounded-lg p-4 mb-4" role="alert">
        <p class="text-red-700 text-sm">{userSummariesError}</p>
      </div>
    {/if}

    {#if userSummariesLoading && userSummaries.length === 0}
      <div class="flex items-center justify-center py-12">
        <div class="text-center">
          <div class="w-6 h-6 border-2 border-slate-200 border-t-slate-500 rounded-full animate-spin mx-auto mb-2"></div>
          <p class="text-slate-500 text-sm">Loading user sessions…</p>
        </div>
      </div>
    {:else if userSummaries.length === 0}
      <p class="text-sm text-slate-500">No sessions in this period.</p>
    {:else}
      <CardTable title="User sessions — last {monitoringWindow}">
        <PaginatedTable
          totalPages={pagedUserSummaries.totalPages}
          currentPage={pagedUserSummaries.page}
          onPageChange={(p) => (userSummariesPage = p)}
        >
          <table class="w-full text-sm">
            <thead class="bg-slate-50">
              <tr class="text-left text-slate-500 font-medium text-xs">
                <th class="px-6 py-3">User</th>
                <th class="px-6 py-3">Sessions</th>
                <th class="px-6 py-3">Total time</th>
                <th class="px-6 py-3">Avg download</th>
                <th class="px-6 py-3">Avg upload</th>
                <th class="px-6 py-3">Total received</th>
                <th class="px-6 py-3">Total sent</th>
                <th class="px-6 py-3">Total</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-slate-100">
              {#each pagedUserSummaries.items as row (row.username)}
                <tr
                  class="hover:bg-slate-50 cursor-pointer"
                  role="button"
                  tabindex="0"
                  onclick={() => handleUserRowClick(row.username)}
                  onkeydown={(e) => e.key === 'Enter' && handleUserRowClick(row.username)}
                >
                  <td class="px-6 py-3 text-slate-700 font-medium">{row.username}</td>
                  <td class="px-6 py-3 text-slate-700">{row.sessionCount}</td>
                  <td class="px-6 py-3 text-slate-700">{formatDuration(row.totalDurationSeconds)}</td>
                  <td class="px-6 py-3 text-slate-700">{formatBytes(row.avgBytesIn)}</td>
                  <td class="px-6 py-3 text-slate-700">{formatBytes(row.avgBytesOut)}</td>
                  <td class="px-6 py-3 text-slate-700">{formatBytes(row.totalBytesIn)}</td>
                  <td class="px-6 py-3 text-slate-700">{formatBytes(row.totalBytesOut)}</td>
                  <td class="px-6 py-3 text-slate-700">{formatBytes(row.totalBytesIn + row.totalBytesOut)}</td>
                </tr>
              {/each}
            </tbody>
          </table>
        </PaginatedTable>
      </CardTable>
    {/if}
  </section>
```

**2c.** Add the drawer at the very bottom of the template, just before the final `{/if}` that closes the `{:else if analytics}` block:

```svelte
  <!-- User session drawer -->
  {#if drawerUsername !== null}
    {#if drawerLoading}
      <div class="fixed inset-0 bg-black/40 z-40 flex items-center justify-center">
        <div class="bg-white rounded-lg p-8 flex flex-col items-center gap-3">
          <div class="w-6 h-6 border-2 border-slate-200 border-t-slate-500 rounded-full animate-spin"></div>
          <p class="text-sm text-slate-500">Loading sessions…</p>
        </div>
      </div>
    {:else}
      <UserSessionDrawer
        username={drawerUsername}
        window={monitoringWindow}
        sessions={drawerSessions}
        onclose={closeDrawer}
      />
    {/if}
  {/if}
```

- [ ] **Step 3: Check TypeScript compilation**

```bash
cd /opt/ocserv_panel/frontend && npx tsc --noEmit 2>&1 | head -30
```
Expected: no errors.

- [ ] **Step 4: Build the frontend**

```bash
cd /opt/ocserv_panel/frontend && npm run build 2>&1 | tail -20
```
Expected: build succeeds.

- [ ] **Step 5: Commit**

```bash
cd /opt/ocserv_panel && git add frontend/src/routes/history/+page.svelte
git commit -m "feat(history): unified window selector, per-user stats table, session drawer"
```

---

## Task 6: Verify exports from `$lib/api`

**Files:**
- Check: `frontend/src/lib/api/index.ts` (or wherever the barrel export lives)

- [ ] **Step 1: Check the API barrel export**

```bash
grep -n "UserSessionSummary\|getHistoryUsers\|getHistoryUserSessions" /opt/ocserv_panel/frontend/src/lib/api/index.ts 2>/dev/null || grep -rn "export.*from.*history" /opt/ocserv_panel/frontend/src/lib/api/ | head -5
```

- [ ] **Step 2: Add missing exports if needed**

If `UserSessionSummary`, `getHistoryUsers`, and `getHistoryUserSessions` are not re-exported from the barrel file, add them. Open `frontend/src/lib/api/index.ts` (or equivalent) and add:

```typescript
export { getHistoryUsers, getHistoryUserSessions, type UserSessionSummary } from './history';
```

- [ ] **Step 3: Rebuild to confirm clean**

```bash
cd /opt/ocserv_panel/frontend && npm run build 2>&1 | tail -10
```
Expected: no errors.

- [ ] **Step 4: Commit if changed**

```bash
cd /opt/ocserv_panel && git add frontend/src/lib/api/index.ts
git commit -m "chore(api): export UserSessionSummary and user session history functions"
```

---

## Task 7: Run full test suite and verify

- [ ] **Step 1: Run all backend tests**

```bash
cd /opt/ocserv_panel/backend && go test ./... 2>&1 | tail -20
```
Expected: all packages PASS, zero failures.

- [ ] **Step 2: Run frontend type-check**

```bash
cd /opt/ocserv_panel/frontend && npx tsc --noEmit 2>&1 | head -20
```
Expected: no errors.

- [ ] **Step 3: Build backend**

```bash
cd /opt/ocserv_panel/backend && go build ./... 2>&1
```
Expected: no errors.

- [ ] **Step 4: Build frontend**

```bash
cd /opt/ocserv_panel/frontend && npm run build 2>&1 | tail -10
```
Expected: build succeeds.

- [ ] **Step 5: Commit if any fixes were needed**

If any fixes were needed in the above steps, commit them:

```bash
cd /opt/ocserv_panel && git add -p && git commit -m "fix: address issues found during full test suite verification"
```
