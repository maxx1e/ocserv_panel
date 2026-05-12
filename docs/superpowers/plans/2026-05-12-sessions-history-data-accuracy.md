# Sessions & History Data Accuracy Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix Sessions page All Sessions tab to show accurate data by merging `show sessions all` with `show users`, and merge the History page Sessions and Users tabs into a single combined Sessions tab.

**Architecture:** Two independent areas: (1) Backend handler for `GET /api/ocserv/sessions` calls both `ListSessions()` and `ListConnectedUsers()`, joins on `FullSession`, returns enriched response. (2) History page frontend collapses the `sessions` and `users` tabs into one `sessions` tab — shared time window + username filter, user summary section on top, session log below, user detail in a side drawer.

**Tech Stack:** Go backend (net/http, no new deps), SvelteKit 5 + Svelte runes, Tailwind CSS v4, LayerChart via ChartCard.

---

## File Map

**Backend — modified:**
- `backend/internal/server/ocserv_handlers.go` — add `enriched bool` field to `ocservSessionResponse`; rewrite `handleOcservSessions` to merge `ListSessions` + `ListConnectedUsers`
- `backend/internal/store/history.go` — add optional `username` filter to `ListSessions`
- `backend/internal/server/server.go` — update `handleHistorySessions` to pass username filter from query param

**Frontend — modified:**
- `frontend/src/lib/api/history.ts` — add `username` param to `listSessions()`
- `frontend/src/lib/api/sessions.ts` — add `enriched: boolean` field to `OcservSession`
- `frontend/src/routes/sessions/AllSessionsTab.svelte` — handle `enriched=false` with `—` for VPN IP/RX/TX
- `frontend/src/routes/history/+page.svelte` — remove `users` tab, restructure `sessions` tab to include Section A (user summaries) + Section B (session log), shared window + username filter, drawer open from user row

**Frontend — no changes needed:**
- `frontend/src/lib/components/history/UserSessionDrawer.svelte` — already correct, will be reused for the drawer triggered by user row click from History Sessions tab

---

## Task 1: Backend — Add `enriched` field and merge sessions with users

**Files:**
- Modify: `backend/internal/server/ocserv_handlers.go`

### What this does
`handleOcservSessions` currently calls only `ListSessions()`. We change it to also call `ListConnectedUsers()`, build a lookup map by `FullSession`, and enrich each session. If a match is found, `vpnAddress`, `rxBytes`, `txBytes`, `device` come from the `ConnectedUser`. The `enriched` bool tells the frontend whether enrichment was available.

The `ConnectedAt` field is already present in the `Session` struct (parsed from `"Created"` in the occtl adapter) and already mapped in `ocservSessionToResponse`. No fix needed there — it was already working.

- [ ] **Step 1: Add `enriched` field to `ocservSessionResponse`**

In `backend/internal/server/ocserv_handlers.go`, locate the `ocservSessionResponse` struct (line ~32). Add the `Enriched` field:

```go
type ocservSessionResponse struct {
	ID            string `json:"id"`
	FullSession   string `json:"fullSession"`
	Username      string `json:"username"`
	RemoteAddress string `json:"remoteAddress"`
	Location      string `json:"location"`
	ConnectedAt   string `json:"connectedAt,omitempty"`
	VPNAddress    string `json:"vpnAddress"`
	RxBytes       int64  `json:"rxBytes"`
	TxBytes       int64  `json:"txBytes"`
	UserAgent     string `json:"userAgent"`
	Device        string `json:"device"`
	State         string `json:"state"`
	Groupname     string `json:"groupname"`
	Vhost         string `json:"vhost"`
	SessionIsOpen bool   `json:"sessionIsOpen"`
	InUse         bool   `json:"inUse"`
	Enriched      bool   `json:"enriched"`
}
```

- [ ] **Step 2: Add `buildUserLookup` helper and `ocservEnrichedSessionToResponse` function**

Add these two functions just above `ocservSessionToResponse` in `backend/internal/server/ocserv_handlers.go`:

```go
// buildUserLookup builds a map from FullSession → ConnectedUser for O(1) join.
func buildUserLookup(users []sessions.ConnectedUser) map[string]sessions.ConnectedUser {
	m := make(map[string]sessions.ConnectedUser, len(users))
	for _, u := range users {
		if u.FullSession != "" {
			m[u.FullSession] = u
		}
	}
	return m
}

// ocservEnrichedSessionToResponse maps a Session to a response, enriching with
// ConnectedUser data when available (matched by FullSession).
func ocservEnrichedSessionToResponse(s sessions.Session, userLookup map[string]sessions.ConnectedUser) ocservSessionResponse {
	resp := ocservSessionToResponse(s)
	user, ok := userLookup[s.FullSession]
	if ok {
		resp.VPNAddress = user.VpnIP
		resp.RxBytes = user.RxBytes
		resp.TxBytes = user.TxBytes
		resp.Device = user.Device
		resp.Enriched = true
	}
	return resp
}
```

- [ ] **Step 3: Rewrite `handleOcservSessions` to call both adapters**

Replace the existing `handleOcservSessions` function body:

```go
func (a *App) handleOcservSessions(w http.ResponseWriter, r *http.Request) {
	if !allowMethod(w, r, http.MethodGet) {
		return
	}
	principal, ok := a.authenticate(w, r)
	if !ok {
		return
	}
	sess, err := a.ocservService.ListSessions(r.Context(), principal)
	if err != nil {
		a.writeAppError(w, err)
		return
	}
	users, err := a.ocservService.ListConnectedUsers(r.Context(), principal)
	if err != nil {
		// Enrichment failure is non-fatal: serve un-enriched sessions.
		users = nil
	}
	userLookup := buildUserLookup(users)
	resp := make([]ocservSessionResponse, 0, len(sess))
	for _, s := range sess {
		resp = append(resp, ocservEnrichedSessionToResponse(s, userLookup))
	}
	writeJSON(w, http.StatusOK, resp)
}
```

- [ ] **Step 4: Run backend tests**

```bash
cd /opt/ocserv_panel/backend && go test ./internal/server/... -v -run TestHandleOcservSessions 2>&1 | tail -20
```

Expected: any existing session handler tests pass. If no test matches that name, run:

```bash
cd /opt/ocserv_panel/backend && go test ./... 2>&1 | tail -20
```

Expected: `ok` for all packages, no compile errors.

- [ ] **Step 5: Commit**

```bash
cd /opt/ocserv_panel && git add backend/internal/server/ocserv_handlers.go
git commit -m "fix(sessions): enrich All Sessions response with show users data joined by FullSession"
```

---

## Task 2: Backend — Add username filter to `ListSessions` store method

**Files:**
- Modify: `backend/internal/store/history.go`
- Modify: `backend/internal/server/server.go` (the `handleHistorySessions` func at line ~617)

### What this does
`ListSessions` currently has no username filter. When the History page Sessions tab user clicks a username in the User Summary section, Section B (session log) should filter to that user. We add an optional `username string` parameter. Empty string = no filter (existing behaviour preserved).

- [ ] **Step 1: Write a failing test for username-filtered ListSessions**

In `backend/internal/store/store_test.go`, find the existing `TestListSessions` test or add after it:

```go
func TestListSessionsUsernameFilter(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	now := time.Now().UTC()

	for _, row := range []struct {
		username string
		at       time.Time
	}{
		{"alice", now.Add(-1 * time.Hour)},
		{"alice", now.Add(-2 * time.Hour)},
		{"bob", now.Add(-1 * time.Hour)},
	} {
		_, err := s.DB.ExecContext(ctx,
			`INSERT INTO historical_sessions (username, connected_at, disconnected_at, bytes_in, bytes_out, duration_seconds, source, created_at)
			 VALUES (?, ?, ?, 100, 200, 60, 'ingested', ?)`,
			row.username,
			row.at.Format(time.RFC3339),
			row.at.Add(time.Hour).Format(time.RFC3339),
			now.Format(time.RFC3339),
		)
		if err != nil {
			t.Fatalf("insert: %v", err)
		}
	}

	since := now.Add(-24 * time.Hour)

	// No filter returns all 3.
	all, total, err := s.ListSessions(ctx, since, 10, 0, "")
	if err != nil {
		t.Fatalf("ListSessions no filter: %v", err)
	}
	if total != 3 {
		t.Errorf("total no filter: got %d, want 3", total)
	}
	if len(all) != 3 {
		t.Errorf("rows no filter: got %d, want 3", len(all))
	}

	// Filter by alice returns 2.
	aliceSessions, aliceTotal, err := s.ListSessions(ctx, since, 10, 0, "alice")
	if err != nil {
		t.Fatalf("ListSessions alice: %v", err)
	}
	if aliceTotal != 2 {
		t.Errorf("total alice: got %d, want 2", aliceTotal)
	}
	if len(aliceSessions) != 2 {
		t.Errorf("rows alice: got %d, want 2", len(aliceSessions))
	}
	for _, sess := range aliceSessions {
		if sess.Username != "alice" {
			t.Errorf("expected alice, got %q", sess.Username)
		}
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

```bash
cd /opt/ocserv_panel/backend && go test ./internal/store/... -run TestListSessionsUsernameFilter -v 2>&1 | tail -20
```

Expected: compile error because `ListSessions` does not yet accept a `username` argument.

- [ ] **Step 3: Update `ListSessions` signature in `store/history.go`**

Change the signature and query in `backend/internal/store/history.go` at the `ListSessions` function (line ~266):

```go
// ListSessions returns historical sessions with connected_at >= since, newest
// first. If username is non-empty, only sessions for that user are returned.
// Returns the total matching count alongside the paged slice.
func (s *Store) ListSessions(ctx context.Context, since time.Time, limit, offset int, username string) ([]HistoricalSession, int64, error) {
	sinceStr := since.UTC().Format(time.RFC3339)

	var (
		total     int64
		countErr  error
		rows      *sql.Rows
		queryErr  error
	)

	if username != "" {
		countErr = s.DB.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM historical_sessions WHERE connected_at >= ? AND username = ?`,
			sinceStr, username,
		).Scan(&total)
	} else {
		countErr = s.DB.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM historical_sessions WHERE connected_at >= ?`,
			sinceStr,
		).Scan(&total)
	}
	if countErr != nil {
		return nil, 0, fmt.Errorf("count sessions: %w", countErr)
	}

	if username != "" {
		rows, queryErr = s.DB.QueryContext(ctx,
			`SELECT id, username, connected_at, disconnected_at, duration_seconds, bytes_in, bytes_out, source, state
			 FROM historical_sessions
			 WHERE connected_at >= ? AND username = ?
			 ORDER BY connected_at DESC, id DESC
			 LIMIT ? OFFSET ?`,
			sinceStr, username, limit, offset,
		)
	} else {
		rows, queryErr = s.DB.QueryContext(ctx,
			`SELECT id, username, connected_at, disconnected_at, duration_seconds, bytes_in, bytes_out, source, state
			 FROM historical_sessions
			 WHERE connected_at >= ?
			 ORDER BY connected_at DESC, id DESC
			 LIMIT ? OFFSET ?`,
			sinceStr, limit, offset,
		)
	}
	if queryErr != nil {
		return nil, 0, fmt.Errorf("query sessions: %w", queryErr)
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

- [ ] **Step 4: Fix all callers of `store.ListSessions` (compile fix)**

The only caller is `handleHistorySessions` in `backend/internal/server/server.go` (line ~635). Update the call site to pass a `username` query parameter:

```go
func (a *App) handleHistorySessions(w http.ResponseWriter, r *http.Request) {
	if !allowMethod(w, r, http.MethodGet) {
		return
	}

	window := queryValueDefault(r, "window", "24h")
	since := a.now().UTC().Add(-historyWindowDuration(window))

	page := 1
	if p, err := strconv.Atoi(queryValueDefault(r, "page", "1")); err == nil && p > 0 {
		page = p
	}
	limit := 25
	if l, err := strconv.Atoi(queryValueDefault(r, "limit", "25")); err == nil && l > 0 && l <= 100 {
		limit = l
	}
	offset := (page - 1) * limit
	username := r.URL.Query().Get("username")

	sessions, total, err := a.store.ListSessions(r.Context(), since, limit, offset, username)
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

- [ ] **Step 5: Run tests**

```bash
cd /opt/ocserv_panel/backend && go test ./internal/store/... -run TestListSessionsUsernameFilter -v 2>&1 | tail -20
```

Expected: `PASS`.

```bash
cd /opt/ocserv_panel/backend && go test ./... 2>&1 | tail -10
```

Expected: all packages `ok`.

- [ ] **Step 6: Commit**

```bash
cd /opt/ocserv_panel && git add backend/internal/store/history.go backend/internal/server/server.go
git commit -m "feat(history): add username filter to ListSessions store method and API handler"
```

---

## Task 3: Frontend — Update `listSessions` API function and `OcservSession` type

**Files:**
- Modify: `frontend/src/lib/api/history.ts`
- Modify: `frontend/src/lib/api/sessions.ts`

### What this does
`listSessions()` needs to accept an optional `username` filter to pass as a query param. `OcservSession` needs the `enriched` boolean so the UI can show `—` for unenriched fields.

- [ ] **Step 1: Update `listSessions` in `frontend/src/lib/api/history.ts`**

Change the `listSessions` function signature and URL construction:

```typescript
export function listSessions(window: string, page = 1, limit = 25, username = ''): Promise<HistoricalSessionPage> {
  const params = new URLSearchParams({
    window,
    page: String(page),
    limit: String(limit),
  });
  if (username) params.set('username', username);
  return apiFetch<HistoricalSessionPage>(`/api/history/sessions?${params.toString()}`);
}
```

- [ ] **Step 2: Add `enriched` field to `OcservSession` in `frontend/src/lib/api/sessions.ts`**

In `frontend/src/lib/api/sessions.ts`, update the `OcservSession` type to add `enriched`:

```typescript
export type OcservSession = {
  id: string;
  username: string;
  remoteAddress: string;
  vpnAddress: string;
  connectedAt?: string;
  rxBytes: number;
  txBytes: number;
  userAgent: string;
  device: string;
  reconnectCount: number;
  state: string;
  groupname: string;
  vhost: string;
  enriched: boolean;
};
```

- [ ] **Step 3: Run frontend type check**

```bash
cd /opt/ocserv_panel/frontend && npx tsc --noEmit 2>&1 | head -30
```

Expected: no errors (or only pre-existing errors unrelated to these files).

- [ ] **Step 4: Commit**

```bash
cd /opt/ocserv_panel && git add frontend/src/lib/api/history.ts frontend/src/lib/api/sessions.ts
git commit -m "feat(api): add enriched field to OcservSession type; add username filter to listSessions"
```

---

## Task 4: Frontend — Fix All Sessions tab: show `—` for unenriched fields

**Files:**
- Modify: `frontend/src/routes/sessions/AllSessionsTab.svelte`

### What this does
The `AllSessionsTab` currently renders `session.vpnAddress` and `rxBytes/txBytes` unconditionally. When `enriched === false`, these fields are empty/zero because `show sessions all` doesn't provide them. We show `—` for unenriched sessions, and actual values for enriched ones.

The `Connected` column (`session.connectedAt`) is already correctly mapped by the backend (`ocservSessionToResponse` maps `s.ConnectedAt` → `connectedAt`). It was already working. No change needed for the `Connected` column.

- [ ] **Step 1: Update the table rows in `AllSessionsTab.svelte` to handle unenriched sessions**

In `frontend/src/routes/sessions/AllSessionsTab.svelte`, replace the VPN IP, RX/TX, and RX/TX format cells in the table `<tbody>` (around lines 159-161):

```svelte
<td class="px-4 py-3 font-mono text-xs text-slate-700">
  {session.enriched && session.vpnAddress ? session.vpnAddress : '—'}
</td>
<td class="px-4 py-3 font-mono text-xs text-slate-700">{session.remoteAddress || '—'}</td>
<td class="px-4 py-3 font-mono text-xs text-slate-700">
  {#if session.enriched && (session.rxBytes > 0 || session.txBytes > 0)}
    {formatBytes(session.rxBytes)} / {formatBytes(session.txBytes)}
  {:else}
    —
  {/if}
</td>
```

- [ ] **Step 2: Run frontend type check**

```bash
cd /opt/ocserv_panel/frontend && npx tsc --noEmit 2>&1 | head -20
```

Expected: no new errors.

- [ ] **Step 3: Commit**

```bash
cd /opt/ocserv_panel && git add frontend/src/routes/sessions/AllSessionsTab.svelte
git commit -m "fix(sessions): show em-dash for VPN IP and RX/TX on unenriched sessions"
```

---

## Task 5: Frontend — History page: merge Sessions + Users tabs into single Sessions tab

**Files:**
- Modify: `frontend/src/routes/history/+page.svelte`

### What this does
Remove the `users` tab from the tab list and the `{:else if activeTab === 'users'}` block. Move all the users tab content (user summaries table, sort controls, search) into the `sessions` tab as **Section A** at the top. Section B (the existing historical session log) follows below. 

The key new interactions:
1. Shared `sessionsCombinedWindow` replaces the separate `sessionsWindow` and `usersWindow` state.
2. Shared `sessionsCombinedSearch` filter applies to both sections.
3. Clicking a username row in Section A opens the existing `UserSessionDrawer` (no new component needed — it's already used by the sessions tab row click).
4. Clicking a username row in Section A also pre-fills `sessionsCombinedSearch` to cross-filter Section B.

- [ ] **Step 1: Update tab definitions — remove `users` tab**

In `frontend/src/routes/history/+page.svelte`, locate the `tabs` array definition (around line 76). Remove the `users` entry:

```typescript
const tabs: { id: TabId; label: string; icon: typeof BarChart2 }[] = [
  { id: 'monitoring', label: 'Monitoring', icon: BarChart2 },
  { id: 'sessions', label: 'Sessions', icon: List },
  { id: 'bans', label: 'Bans', icon: Shield },
  { id: 'settings', label: 'Settings', icon: Settings },
];
```

Also update `TabId` type and the `activated` state object:

```typescript
type TabId = 'monitoring' | 'sessions' | 'bans' | 'settings';

let activated = $state<Record<TabId, boolean>>({
  monitoring: true,
  sessions: false,
  bans: false,
  settings: false,
});
```

- [ ] **Step 2: Unify sessions + users state under shared window and search**

Replace the separate `sessionsWindow`/`usersWindow` and `sessionsSearch`/`usersSearch` with unified state. Find these state declarations (around lines 109-131) and replace them:

```typescript
// --- Combined Sessions tab state ---
let sessionsWindow = $state<MonitoringWindow>('24h');
let sessionsPage = $state(1);
let sessionsData = $state<HistoricalSessionPage | null>(null);
let sessionsLoading = $state(false);
let sessionsError = $state('');
let sessionsSearch = $state('');
let activeSessions = $state<ConnectedUser[]>([]);
let activeSessionsLoading = $state(false);

// User summaries (was users tab, now Section A of sessions tab)
let userSummaries = $state<UserSessionSummary[]>([]);
let userSummariesLoading = $state(false);
let userSummariesError = $state('');
let expandedUser = $state<string | null>(null);
let userTraffic = $state<UserTrafficResponse | null>(null);
let userTrafficLoading = $state(false);

type SortField = 'totalBytesIn' | 'totalBytesOut' | 'totalTraffic' | 'avgBytesIn' | 'avgBytesOut';
let usersSortField = $state<SortField>('totalTraffic');
let usersSortDir = $state<'asc' | 'desc'>('desc');
```

Remove the old separate `usersWindow`, `userSummaries*`, `usersSearch`, and Users sort state — they are now the same variables used for both sections (search is shared via `sessionsSearch`).

- [ ] **Step 3: Update `loadTabData` to load user summaries when sessions tab activates**

Locate `loadTabData` (around line 314) and update:

```typescript
function loadTabData(tab: TabId) {
  if (tab === 'sessions') {
    void loadSessions();
    void loadActiveSessions();
    void reloadUserSummaries(sessionsWindow);
  } else if (tab === 'bans') {
    void loadBans();
  } else if (tab === 'settings') {
    void loadSettings();
  }
}
```

Also update `handleSessionsWindowChange` to reload user summaries when the window changes:

```typescript
function handleSessionsWindowChange(nextWindow: string) {
  const window = nextWindow as MonitoringWindow;
  if (!HISTORY_WINDOWS.includes(window) || window === sessionsWindow) return;
  sessionsWindow = window;
  sessionsPage = 1;
  expandedUser = null;
  userTraffic = null;
  void loadSessions();
  void reloadUserSummaries(window);
}
```

- [ ] **Step 4: Update `reloadUserSummaries` to use `sessionsWindow`**

The `reloadUserSummaries` function currently uses `usersWindow`. Change it to use `sessionsWindow` as parameter and remove `usersWindow`:

```typescript
async function reloadUserSummaries(window: MonitoringWindow = sessionsWindow) {
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
```

- [ ] **Step 5: Update `loadSessions` to pass username filter**

Update the `loadSessions` function to pass `sessionsSearch` as the username filter:

```typescript
async function loadSessions() {
  sessionsLoading = true;
  sessionsError = '';
  try {
    sessionsData = await listSessions(sessionsWindow, sessionsPage, 25, sessionsSearch);
  } catch (e) {
    sessionsError = e instanceof Error ? e.message : 'Failed to load sessions';
  } finally {
    sessionsLoading = false;
  }
}
```

- [ ] **Step 6: Update derived state — user summaries sort and filter**

The `sortedUserSummaries` derived still references `usersSearch`/`usersSortField`/`usersSortDir`. Update it to use `sessionsSearch`:

```typescript
const sortedUserSummaries = $derived.by(() => {
  const arr = [...userSummaries];
  arr.sort((a, b) => {
    let av = 0, bv = 0;
    if (usersSortField === 'totalBytesIn') { av = a.totalBytesIn; bv = b.totalBytesIn; }
    else if (usersSortField === 'totalBytesOut') { av = a.totalBytesOut; bv = b.totalBytesOut; }
    else if (usersSortField === 'totalTraffic') { av = a.totalBytesIn + a.totalBytesOut; bv = b.totalBytesIn + b.totalBytesOut; }
    else if (usersSortField === 'avgBytesIn') { av = a.avgBytesIn; bv = b.avgBytesIn; }
    else if (usersSortField === 'avgBytesOut') { av = a.avgBytesOut; bv = b.avgBytesOut; }
    return usersSortDir === 'asc' ? av - bv : bv - av;
  });
  return arr;
});

const activeUsernames = $derived(new Set(activeSessions.map(s => s.username)));

const filteredUserSummaries = $derived(
  sessionsSearch
    ? sortedUserSummaries.filter(u =>
        u.username.toLowerCase().includes(sessionsSearch.toLowerCase())
      )
    : sortedUserSummaries
);
```

- [ ] **Step 7: Update the refreshInterval in `onMount` to remove users tab auto-refresh reference**

Locate the `setInterval` block (around line 290). Remove `users` from the condition since it's now part of sessions:

```typescript
refreshInterval = setInterval(() => {
  if (document.visibilityState === 'visible') {
    if (activeTab === 'monitoring') void reloadMonitoring();
    if (activeTab === 'sessions') {
      void loadActiveSessions();
      void reloadUserSummaries(sessionsWindow);
    }
  }
}, 15_000);
```

- [ ] **Step 8: Replace sessions tab template content**

Locate the `{:else if activeTab === 'sessions'}` block (around line 721). Replace its entire content with the combined Sessions tab:

```svelte
{:else if activeTab === 'sessions'}
  <div class="space-y-6">
    <!-- Shared toolbar -->
    <div class="flex items-center justify-between flex-wrap gap-3">
      <WindowSelector
        value={sessionsWindow}
        windows={HISTORY_WINDOWS}
        onchange={handleSessionsWindowChange}
      />
      <div class="flex items-center gap-3">
        <input
          type="search"
          placeholder="Filter by username..."
          bind:value={sessionsSearch}
          oninput={() => { sessionsPage = 1; void loadSessions(); }}
          class="px-3 py-1.5 text-sm border border-slate-200 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500 dark:bg-slate-800 dark:border-slate-700 dark:text-slate-200"
        />
        <div class="flex items-center gap-2 text-sm text-slate-500">
          <label for="users-sort-combined" class="text-xs font-medium">Sort:</label>
          <select
            id="users-sort-combined"
            bind:value={usersSortField}
            onchange={() => { usersSortDir = 'desc'; }}
            class="text-xs border border-slate-200 rounded-md px-2 py-1.5 bg-white text-slate-700 dark:bg-slate-800 dark:border-slate-700 dark:text-slate-200"
          >
            <option value="totalTraffic">Total traffic ↓</option>
            <option value="totalBytesIn">Total received ↓</option>
            <option value="totalBytesOut">Total sent ↓</option>
            <option value="avgBytesIn">Avg download ↓</option>
            <option value="avgBytesOut">Avg upload ↓</option>
          </select>
        </div>
      </div>
    </div>

    <!-- Section A: Currently Active -->
    <div class="bg-green-50 border border-green-200 rounded-xl p-4 dark:bg-green-950/30 dark:border-green-800">
      <div class="flex items-center gap-2 mb-3">
        <span class="relative flex h-2.5 w-2.5">
          <span class="animate-ping absolute inline-flex h-full w-full rounded-full bg-green-400 opacity-75"></span>
          <span class="relative inline-flex rounded-full h-2.5 w-2.5 bg-green-500"></span>
        </span>
        <span class="text-sm font-semibold text-green-800 dark:text-green-200">
          Currently active ({activeSessions.length})
        </span>
      </div>
      {#if activeSessionsLoading && activeSessions.length === 0}
        <p class="text-xs text-green-700 dark:text-green-300">Loading...</p>
      {:else if activeSessions.length === 0}
        <p class="text-xs text-green-700 dark:text-green-300">No active sessions right now</p>
      {:else}
        <div class="overflow-x-auto">
          <table class="w-full text-sm">
            <thead>
              <tr class="text-left text-xs text-green-700 dark:text-green-300 font-semibold">
                <th class="pb-1 pr-4">Username</th>
                <th class="pb-1 pr-4">Device</th>
                <th class="pb-1 pr-4">Connected since</th>
                <th class="pb-1 pr-4">Remote IP</th>
                <th class="pb-1 pr-4">VPN IP</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-green-100 dark:divide-green-800">
              {#each activeSessions as s (s.id)}
                <tr>
                  <td class="py-1.5 pr-4 font-mono text-xs text-green-900 dark:text-green-100 font-semibold">{s.username}</td>
                  <td class="py-1.5 pr-4 text-xs text-green-800 dark:text-green-200">{s.device}</td>
                  <td class="py-1.5 pr-4 text-xs text-green-800 dark:text-green-200">{formatDuration(Math.floor((Date.now() - new Date(s.since).getTime()) / 1000))}</td>
                  <td class="py-1.5 pr-4 text-xs text-green-800 dark:text-green-200">{s.remoteIp}</td>
                  <td class="py-1.5 text-xs text-green-800 dark:text-green-200">{s.vpnIp}</td>
                </tr>
              {/each}
            </tbody>
          </table>
        </div>
      {/if}
    </div>

    <!-- Section A: Per-User Summary -->
    <div>
      <p class="text-xs font-semibold text-slate-500 uppercase tracking-wider mb-2">
        Users — last {sessionsWindow}
      </p>
      {#if userSummariesError}
        <div class="flex items-center gap-3 py-4">
          <p class="text-red-600 text-sm">{userSummariesError}</p>
          <button class="text-sm text-blue-600 hover:underline" onclick={() => reloadUserSummaries(sessionsWindow)}>Retry</button>
        </div>
      {/if}
      {#if userSummariesLoading && userSummaries.length === 0}
        <div class="space-y-2">
          {#each Array(3) as _}
            <div class="h-10 bg-slate-100 rounded animate-pulse"></div>
          {/each}
        </div>
      {:else if filteredUserSummaries.length === 0 && !userSummariesLoading}
        <p class="text-sm text-slate-500">No user sessions in this period.</p>
      {:else}
        <div class="rounded-lg border border-slate-200 overflow-hidden dark:border-slate-700">
          <table class="w-full text-sm">
            <thead class="bg-slate-50 dark:bg-slate-800">
              <tr class="text-left text-xs text-slate-500 font-medium">
                <th class="px-4 py-3">User</th>
                <th class="px-4 py-3">Sessions</th>
                <th class="px-4 py-3">Total time</th>
                <th class="px-4 py-3">↓ Total</th>
                <th class="px-4 py-3">↑ Total</th>
                <th class="px-4 py-3 text-blue-600 font-semibold">Total ↕</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-slate-100 dark:divide-slate-700">
              {#each filteredUserSummaries as row (row.username)}
                {@const initials = row.username.slice(0, 1).toUpperCase()}
                {@const isActive = activeUsernames.has(row.username)}
                <tr
                  class="hover:bg-slate-50 cursor-pointer dark:hover:bg-slate-800/50"
                  role="button"
                  tabindex="0"
                  onclick={() => {
                    sessionsSearch = row.username;
                    sessionsPage = 1;
                    void loadSessions();
                    openSessionDrawer(row.username, sessionsWindow);
                  }}
                  onkeydown={(e) => {
                    if (e.key === 'Enter') {
                      sessionsSearch = row.username;
                      sessionsPage = 1;
                      void loadSessions();
                      openSessionDrawer(row.username, sessionsWindow);
                    }
                  }}
                >
                  <td class="px-4 py-3">
                    <div class="flex items-center gap-2">
                      <div class="w-7 h-7 rounded-full bg-indigo-100 text-indigo-700 flex items-center justify-center text-xs font-bold flex-shrink-0">{initials}</div>
                      <span class="text-slate-700 font-medium dark:text-slate-200">{row.username}</span>
                      {#if isActive}
                        <span class="w-2 h-2 rounded-full bg-green-500 flex-shrink-0" title="Currently active"></span>
                      {/if}
                    </div>
                  </td>
                  <td class="px-4 py-3 text-slate-600 dark:text-slate-400">{row.sessionCount}</td>
                  <td class="px-4 py-3 text-slate-600 dark:text-slate-400">{formatDuration(row.totalDurationSeconds)}</td>
                  <td class="px-4 py-3 text-slate-600 dark:text-slate-400">{formatBytes(row.totalBytesIn)}</td>
                  <td class="px-4 py-3 text-slate-600 dark:text-slate-400">{formatBytes(row.totalBytesOut)}</td>
                  <td class="px-4 py-3 font-semibold text-slate-800 dark:text-slate-100">{formatBytes(row.totalBytesIn + row.totalBytesOut)}</td>
                </tr>
              {/each}
            </tbody>
          </table>
        </div>
        <div class="flex items-center justify-between text-sm text-slate-500 mt-1">
          <span>{filteredUserSummaries.length} users</span>
        </div>
      {/if}
    </div>

    <!-- Section B: Session Log -->
    <div>
      <p class="text-xs font-semibold text-slate-500 uppercase tracking-wider mb-2">
        Session log — last {sessionsWindow}{sessionsSearch ? ` · ${sessionsSearch}` : ''} ({sessionsData?.total ?? '…'} sessions)
      </p>
      {#if sessionsLoading && !sessionsData}
        <div class="text-sm text-slate-500 py-8 text-center">Loading sessions...</div>
      {:else if sessionsError}
        <div class="text-sm text-red-600 py-4">{sessionsError}</div>
      {:else if sessionsData}
        <div class="rounded-lg border border-slate-200 overflow-hidden dark:border-slate-700">
          <table class="w-full text-sm">
            <thead class="bg-slate-50 border-b border-slate-200 dark:bg-slate-800 dark:border-slate-700">
              <tr>
                <th class="px-4 py-2 text-left font-medium text-slate-600 dark:text-slate-400">Username</th>
                <th class="px-4 py-2 text-left font-medium text-slate-600 dark:text-slate-400">Connected</th>
                <th class="px-4 py-2 text-left font-medium text-slate-600 dark:text-slate-400">Ended</th>
                <th class="px-4 py-2 text-left font-medium text-slate-600 dark:text-slate-400">Duration</th>
                <th class="px-4 py-2 text-right font-medium text-slate-600 dark:text-slate-400">RX</th>
                <th class="px-4 py-2 text-right font-medium text-slate-600 dark:text-slate-400">TX</th>
                <th class="px-4 py-2 text-center font-medium text-slate-600 dark:text-slate-400">State</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-slate-100 dark:divide-slate-700">
              {#each sessionsData.sessions as session}
                {@const stateColors: Record<string, string> = {
                  authenticated: 'bg-green-100 text-green-700',
                  connected: 'bg-green-100 text-green-700',
                  disconnected: 'bg-slate-100 text-slate-600',
                  timed_out: 'bg-amber-100 text-amber-700',
                  error: 'bg-red-100 text-red-700',
                }}
                <tr
                  class="hover:bg-slate-50 cursor-pointer dark:hover:bg-slate-800/50"
                  onclick={() => session.username && openSessionDrawer(session.username, sessionsWindow)}
                >
                  <td class="px-4 py-2 font-mono text-xs text-slate-800 dark:text-slate-200">{session.username ?? '—'}</td>
                  <td class="px-4 py-2 text-slate-600 text-xs dark:text-slate-400">{formatAbsoluteTime(session.connectedAt ?? session.startedAt)}</td>
                  <td class="px-4 py-2 text-slate-600 text-xs dark:text-slate-400">
                    {#if session.state === 'authenticated' || session.state === 'connected'}
                      <span class="inline-flex items-center rounded-full bg-green-100 px-2 py-0.5 text-xs font-medium text-green-800 dark:bg-green-900 dark:text-green-200">Active</span>
                    {:else}
                      {formatAbsoluteTime(session.disconnectedAt ?? session.endedAt)}
                    {/if}
                  </td>
                  <td class="px-4 py-2 text-slate-600 text-xs dark:text-slate-400">{formatDuration(session.durationSeconds ?? 0)}</td>
                  <td class="px-4 py-2 text-right text-xs">{formatBytes(session.bytesIn ?? session.rxBytes ?? 0)}</td>
                  <td class="px-4 py-2 text-right text-xs">{formatBytes(session.bytesOut ?? session.txBytes ?? 0)}</td>
                  <td class="px-4 py-2 text-center">
                    <span class="px-2 py-0.5 rounded-full text-xs font-medium {stateColors[session.state ?? 'disconnected'] ?? 'bg-slate-100 text-slate-600'}">
                      {session.state ?? 'disconnected'}
                    </span>
                  </td>
                </tr>
              {/each}
            </tbody>
          </table>
        </div>
        <div class="flex items-center justify-between text-sm text-slate-500 mt-3">
          <span>{sessionsData.total} total sessions</span>
          <div class="flex gap-2">
            <button
              disabled={sessionsPage <= 1}
              onclick={() => { sessionsPage--; void loadSessions(); }}
              class="px-3 py-1 rounded border border-slate-200 disabled:opacity-40 hover:bg-slate-50 dark:border-slate-700 dark:hover:bg-slate-800"
            >Previous</button>
            <span class="px-3 py-1">Page {sessionsPage}</span>
            <button
              disabled={sessionsPage * sessionsData.limit >= sessionsData.total}
              onclick={() => { sessionsPage++; void loadSessions(); }}
              class="px-3 py-1 rounded border border-slate-200 disabled:opacity-40 hover:bg-slate-50 dark:border-slate-700 dark:hover:bg-slate-800"
            >Next</button>
          </div>
        </div>
      {/if}
    </div>
  </div>
```

- [ ] **Step 9: Remove the entire `{:else if activeTab === 'users'}` template block**

After the sessions block, find and delete the entire `{:else if activeTab === 'users'}` section (from line ~864 to ~987 in the original file). It is now incorporated into the sessions tab above.

- [ ] **Step 10: Run frontend type check**

```bash
cd /opt/ocserv_panel/frontend && npx tsc --noEmit 2>&1 | head -30
```

Expected: no errors.

- [ ] **Step 11: Commit**

```bash
cd /opt/ocserv_panel && git add frontend/src/routes/history/+page.svelte
git commit -m "feat(history): merge Users tab into Sessions tab — Section A (user summaries) + Section B (session log)"
```

---

## Task 6: Update `memory.md` and do a final build check

**Files:**
- Modify: `.claude/rules/memory.md` — add implementation plan entry

- [ ] **Step 1: Add plan entry to memory.md**

In `.claude/rules/memory.md`, add the following row to the Implementation Plans table:

```
| 2026-05-12 | `2026-05-12-sessions-history-data-accuracy.md` | Sessions All Sessions enrichment fix; History Users+Sessions tab merge |
```

- [ ] **Step 2: Run full backend test suite**

```bash
cd /opt/ocserv_panel/backend && go test ./... 2>&1 | tail -15
```

Expected: all packages `ok`, no failures.

- [ ] **Step 3: Run full frontend build**

```bash
cd /opt/ocserv_panel/frontend && npm run build 2>&1 | tail -20
```

Expected: build succeeds with no errors (warnings about unused CSS are acceptable).

- [ ] **Step 4: Commit**

```bash
cd /opt/ocserv_panel && git add .claude/rules/memory.md
git commit -m "docs: update memory.md with sessions/history data accuracy plan"
```

---

## Self-Review Notes

**Spec coverage check:**

| Spec requirement | Task covering it |
|-----------------|-----------------|
| Remove VPN IP / RX/TX from unenriched sessions | Task 1, Task 4 |
| Merge `show sessions all` + `show users` by FullSession | Task 1 |
| `Connected` column populated from `Created` field | Already working — confirmed in `ocservSessionToResponse` |
| Session detail drawer uses `show session <id>` | Existing, unchanged |
| History Sessions + Users → single Sessions tab | Task 5 |
| Shared time window + username filter across sections | Task 5 |
| Username click in Section A pre-fills Section B filter | Task 5 (Step 8) |
| Username click opens drawer with stats + chart + session list | Task 5 — uses existing `UserSessionDrawer` + `openSessionDrawer` |
| Username filter on `GET /api/history/sessions` | Task 2 |
| Backend ListSessions signature update | Task 2 |
| Frontend `listSessions` username param | Task 3 |

**Placeholder scan:** No TBDs, all code complete.

**Type consistency:**
- `ocservEnrichedSessionToResponse` uses `sessions.ConnectedUser.VpnIP` (matches `types.go:85 VpnIP`)
- `buildUserLookup` keyed on `ConnectedUser.FullSession` (matches `types.go:98 FullSession`)
- `OcservSession.enriched` added in Task 3 matches `ocservSessionResponse.Enriched` from Task 1
- `listSessions(window, page, limit, username)` signature matches usage in Task 5 Step 5
