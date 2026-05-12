# History Session Log Fixes Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix the History page Session Log so it only shows completed sessions with correct end times, detect session disconnects in the ingester, and make UserSessionDrawer charts visually consistent with the Memory utilization chart style.

**Architecture:** Three independent changes: (1) the session ingester tracks previously-seen session IDs and issues a `disconnected` upsert when a session disappears from the active list; (2) `store.ListSessions` filters out `state = 'authenticated'` rows; (3) two new metric keys in `monitoring.ts` give the drawer charts correct byte formatting and consistent indigo color palette.

**Tech Stack:** Go (backend/internal/app, backend/internal/store), TypeScript/Svelte (frontend)

---

## File Map

| File | Change |
|------|--------|
| `backend/internal/app/session_ingester.go` | Add `prevSessions map[string]bool` field + disconnect detection in `poll` |
| `backend/internal/app/session_ingester_test.go` | Add test for disconnect detection |
| `backend/internal/store/history.go` | Add `AND state != 'authenticated'` filter to `ListSessions` |
| `backend/internal/store/store_test.go` | Add test that `ListSessions` excludes authenticated sessions |
| `frontend/src/lib/monitoring.ts` | Add `session_bytes` and `session_duration_minutes` metric config entries |
| `frontend/src/lib/components/history/UserSessionDrawer.svelte` | Use new metric keys; fix X-axis label format |

---

### Task 1: Ingester — disconnect detection test

**Files:**
- Modify: `backend/internal/app/session_ingester_test.go`

- [ ] **Step 1: Add a failing test for disconnect detection**

Append this test to `backend/internal/app/session_ingester_test.go`:

```go
func TestSessionIngester_PollMarksDisconnectedSessions(t *testing.T) {
	fixedNow := time.Date(2026, 5, 10, 12, 0, 0, 0, time.UTC)
	connectedAt := time.Date(2026, 5, 10, 10, 0, 0, 0, time.UTC)

	// First poll: alice and bob are connected.
	mgr := &fakeIngesterManager{
		connectedUsers: []sessions.ConnectedUser{
			{ID: "1", Username: "alice"},
			{ID: "2", Username: "bob"},
		},
		userDetails: map[string][]sessions.UserConnection{
			"alice": {{ID: "1", Username: "alice", SessionID: "AliceSess", ConnectedAt: connectedAt, RxBytes: 100, TxBytes: 200}},
			"bob":   {{ID: "2", Username: "bob", SessionID: "BobSess", ConnectedAt: connectedAt, RxBytes: 300, TxBytes: 400}},
		},
	}
	st := &fakeIngesterStore{}
	ingester := NewSessionIngester(st, mgr,
		IngesterWithNow(func() time.Time { return fixedNow }),
		IngesterWithLogger(zerolog.Nop()),
	)
	ctx := context.Background()
	ingester.poll(ctx)

	// Second poll: only bob is still connected; alice has disconnected.
	disconnectNow := fixedNow.Add(5 * time.Minute)
	ingester.now = func() time.Time { return disconnectNow }
	mgr.connectedUsers = []sessions.ConnectedUser{
		{ID: "2", Username: "bob"},
	}
	ingester.poll(ctx)

	records := st.getRecords()
	// Find the upsert for alice's session after disconnect (state = "disconnected").
	var disconnectRec *store.ActiveSessionRecord
	for i, r := range records {
		if r.OcservSessionID == "AliceSess" && r.State == "disconnected" {
			disconnectRec = &records[i]
			break
		}
	}
	if disconnectRec == nil {
		t.Fatal("expected a disconnected upsert for AliceSess, got none")
	}
	if !disconnectRec.DisconnectedAt.Equal(disconnectNow) {
		t.Errorf("disconnected_at = %v, want %v", disconnectRec.DisconnectedAt, disconnectNow)
	}
	if disconnectRec.Username != "alice" {
		t.Errorf("username = %q, want %q", disconnectRec.Username, "alice")
	}
}
```

- [ ] **Step 2: Run the failing test**

```bash
cd /opt/ocserv_panel/backend && go test ./internal/app/ -run TestSessionIngester_PollMarksDisconnectedSessions -v
```

Expected: FAIL — `expected a disconnected upsert for AliceSess, got none`

---

### Task 2: Ingester — implement disconnect detection

**Files:**
- Modify: `backend/internal/app/session_ingester.go`

- [ ] **Step 1: Add `prevSessions` field to `SessionIngester`**

In `backend/internal/app/session_ingester.go`, update the struct definition:

```go
type SessionIngester struct {
	manager      sessionIngesterManager
	store        sessionIngesterStore
	interval     time.Duration
	logger       zerolog.Logger
	now          func() time.Time
	prevSessions map[string]sessionSnapshot // keyed by ocserv session ID
}
```

Add the snapshot type just above the struct (before the `SessionIngester` type):

```go
// sessionSnapshot holds just enough state about a previously-seen session to
// emit a disconnected upsert when the session disappears from the active list.
type sessionSnapshot struct {
	username    string
	connectedAt time.Time
}
```

- [ ] **Step 2: Update `poll` to build current session set and detect disconnects**

Replace the entire `poll` method body in `session_ingester.go` with the following. The new logic inserts a disconnect upsert for every session ID that was in `prevSessions` but is absent from this poll's results, then replaces `prevSessions`:

```go
func (si *SessionIngester) poll(ctx context.Context) {
	if err := ctx.Err(); err != nil {
		return
	}

	users, err := si.manager.ListConnectedUsers(ctx)
	if err != nil {
		si.logger.Warn().Err(err).Msg("session ingester: failed to list connected users")
		return
	}

	// Deduplicate usernames.
	seen := make(map[string]bool, len(users))
	var uniqueUsernames []string
	for _, u := range users {
		if u.Username != "" && !seen[u.Username] {
			seen[u.Username] = true
			uniqueUsernames = append(uniqueUsernames, u.Username)
		}
	}

	now := si.now()
	currentSessions := make(map[string]sessionSnapshot)
	var upserted int

	for _, username := range uniqueUsernames {
		conns, err := si.manager.GetUserDetails(ctx, username)
		if err != nil {
			si.logger.Warn().Err(err).Str("username", username).Msg("session ingester: failed to get user details")
			continue
		}

		for _, conn := range conns {
			sessionID := conn.SessionID
			if sessionID == "" {
				si.logger.Debug().Str("username", username).Str("device", conn.Device).Msg("session ingester: skipping connection without session ID")
				continue
			}

			connectedAt := conn.ConnectedAt
			if connectedAt.IsZero() {
				connectedAt = now
			}

			durationSeconds := int64(now.Sub(connectedAt).Seconds())
			if durationSeconds < 0 {
				durationSeconds = 0
			}

			state := conn.State
			if state == "connected" {
				state = "authenticated"
			}

			rec := store.ActiveSessionRecord{
				OcservSessionID: sessionID,
				Username:        username,
				ConnectedAt:     connectedAt,
				DisconnectedAt:  now,
				DurationSeconds: durationSeconds,
				BytesIn:         conn.RxBytes,
				BytesOut:        conn.TxBytes,
				Source:          "ingested",
				State:           state,
			}

			if err := si.store.UpsertActiveSession(ctx, rec); err != nil {
				si.logger.Warn().Err(err).
					Str("username", username).
					Str("session_id", sessionID).
					Msg("session ingester: upsert failed")
				continue
			}
			upserted++
			currentSessions[sessionID] = sessionSnapshot{
				username:    username,
				connectedAt: connectedAt,
			}
		}
	}

	// Detect disconnects: sessions present in prevSessions but absent from this poll.
	for sessionID, snap := range si.prevSessions {
		if _, stillActive := currentSessions[sessionID]; !stillActive {
			durationSeconds := int64(now.Sub(snap.connectedAt).Seconds())
			if durationSeconds < 0 {
				durationSeconds = 0
			}
			rec := store.ActiveSessionRecord{
				OcservSessionID: sessionID,
				Username:        snap.username,
				ConnectedAt:     snap.connectedAt,
				DisconnectedAt:  now,
				DurationSeconds: durationSeconds,
				BytesIn:         0,
				BytesOut:        0,
				Source:          "ingested",
				State:           "disconnected",
			}
			if err := si.store.UpsertActiveSession(ctx, rec); err != nil {
				si.logger.Warn().Err(err).
					Str("username", snap.username).
					Str("session_id", sessionID).
					Msg("session ingester: disconnect upsert failed")
			} else {
				si.logger.Debug().
					Str("username", snap.username).
					Str("session_id", sessionID).
					Msg("session ingester: marked session disconnected")
			}
		}
	}

	si.prevSessions = currentSessions

	si.logger.Debug().
		Int("users", len(uniqueUsernames)).
		Int("upserted", upserted).
		Msg("session ingester: poll complete")
}
```

- [ ] **Step 3: Run all ingester tests**

```bash
cd /opt/ocserv_panel/backend && go test ./internal/app/ -run TestSessionIngester -v
```

Expected: all 6 tests PASS (the 5 existing + the new disconnect test).

- [ ] **Step 4: Commit**

```bash
cd /opt/ocserv_panel && git add backend/internal/app/session_ingester.go backend/internal/app/session_ingester_test.go
git commit -m "feat(ingester): detect session disconnects and mark state=disconnected"
```

---

### Task 3: Store — filter authenticated sessions from ListSessions

**Files:**
- Modify: `backend/internal/store/history.go`
- Modify: `backend/internal/store/store_test.go`

- [ ] **Step 1: Add a failing test**

Append this test to `backend/internal/store/store_test.go`:

```go
func TestListSessions_ExcludesAuthenticatedSessions(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	base := time.Now().UTC().Add(-2 * time.Hour)

	// One completed session.
	if err := s.UpsertActiveSession(ctx, ActiveSessionRecord{
		OcservSessionID: "done-sess",
		Username:        "alice",
		ConnectedAt:     base,
		DisconnectedAt:  base.Add(30 * time.Minute),
		DurationSeconds: 1800,
		BytesIn:         1000,
		BytesOut:        2000,
		Source:          "ingested",
		State:           "disconnected",
	}); err != nil {
		t.Fatalf("upsert disconnected: %v", err)
	}

	// One currently active session (state = "authenticated").
	if err := s.UpsertActiveSession(ctx, ActiveSessionRecord{
		OcservSessionID: "live-sess",
		Username:        "bob",
		ConnectedAt:     base.Add(time.Hour),
		DisconnectedAt:  base.Add(time.Hour),
		DurationSeconds: 0,
		BytesIn:         500,
		BytesOut:        500,
		Source:          "ingested",
		State:           "authenticated",
	}); err != nil {
		t.Fatalf("upsert authenticated: %v", err)
	}

	sessions, total, err := s.ListSessions(ctx, base.Add(-time.Hour), 10, 0, "")
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if total != 1 {
		t.Errorf("total = %d, want 1 (authenticated session must be excluded)", total)
	}
	if len(sessions) != 1 {
		t.Fatalf("got %d sessions, want 1", len(sessions))
	}
	if sessions[0].State != "disconnected" {
		t.Errorf("state = %q, want %q", sessions[0].State, "disconnected")
	}
}
```

- [ ] **Step 2: Run the failing test**

```bash
cd /opt/ocserv_panel/backend && go test ./internal/store/ -run TestListSessions_ExcludesAuthenticatedSessions -v
```

Expected: FAIL — `total = 2, want 1`

- [ ] **Step 3: Add state filter to `ListSessions`**

In `backend/internal/store/history.go`, update the `ListSessions` method. Change **both** the count queries and the data queries to add `AND state != 'authenticated'`.

Replace the count block (lines ~286–296):

```go
if username != "" {
    countErr = s.DB.QueryRowContext(ctx,
        `SELECT COUNT(*) FROM historical_sessions WHERE connected_at >= ? AND username = ? AND state != 'authenticated'`,
        sinceStr, username,
    ).Scan(&total)
} else {
    countErr = s.DB.QueryRowContext(ctx,
        `SELECT COUNT(*) FROM historical_sessions WHERE connected_at >= ? AND state != 'authenticated'`,
        sinceStr,
    ).Scan(&total)
}
```

Replace the data query block (lines ~300–318):

```go
if username != "" {
    rows, queryErr = s.DB.QueryContext(ctx,
        `SELECT id, username, connected_at, disconnected_at, duration_seconds, bytes_in, bytes_out, source, state
         FROM historical_sessions
         WHERE connected_at >= ? AND username = ? AND state != 'authenticated'
         ORDER BY connected_at DESC, id DESC
         LIMIT ? OFFSET ?`,
        sinceStr, username, limit, offset,
    )
} else {
    rows, queryErr = s.DB.QueryContext(ctx,
        `SELECT id, username, connected_at, disconnected_at, duration_seconds, bytes_in, bytes_out, source, state
         FROM historical_sessions
         WHERE connected_at >= ? AND state != 'authenticated'
         ORDER BY connected_at DESC, id DESC
         LIMIT ? OFFSET ?`,
        sinceStr, limit, offset,
    )
}
```

- [ ] **Step 4: Run all store tests**

```bash
cd /opt/ocserv_panel/backend && go test ./internal/store/ -v 2>&1 | tail -20
```

Expected: all tests PASS.

- [ ] **Step 5: Commit**

```bash
cd /opt/ocserv_panel && git add backend/internal/store/history.go backend/internal/store/store_test.go
git commit -m "fix(store): exclude authenticated sessions from ListSessions history query"
```

---

### Task 4: Frontend — add session metric config entries

**Files:**
- Modify: `frontend/src/lib/monitoring.ts`

- [ ] **Step 1: Add `session_bytes` and `session_duration_minutes` to `CHART_COLORS`**

In `frontend/src/lib/monitoring.ts`, add two entries to the `CHART_COLORS` map (after the `connection_rate` entry, before the closing `}`):

```typescript
  session_bytes: {
    stroke: 'stroke-indigo-500 stroke-2',
    gradientTop: 'rgba(99, 102, 241, 0.30)',
    gradientBottom: 'rgba(99, 102, 241, 0.05)'
  },
  session_duration_minutes: {
    stroke: 'stroke-slate-500 stroke-2',
    gradientTop: 'rgba(100, 116, 139, 0.30)',
    gradientBottom: 'rgba(100, 116, 139, 0.05)'
  },
```

- [ ] **Step 2: Add `session_bytes` to `isByteMetric` detection**

The existing `isByteMetric` function checks `metric.includes('bytes')`. The new key `session_bytes` already contains `bytes`, so it will be picked up automatically — no change needed. Verify this is the case:

```bash
grep "isByteMetric\|includes('bytes')" /opt/ocserv_panel/frontend/src/lib/monitoring.ts
```

Expected output includes: `return metric.includes('bytes') || metric.includes('bandwidth');`

- [ ] **Step 3: Add `session_bytes` and `session_duration_minutes` to `getChartColorVar`**

In `monitoring.ts`, add entries to the `map` inside `getChartColorVar`:

```typescript
    session_bytes: 'var(--chart-1)',
    session_duration_minutes: 'var(--chart-1)',
```

- [ ] **Step 4: Add a `formatMetricValue` entry for `session_duration_minutes`**

The existing `formatMetricValue` handles bytes via `isByteMetric` and percentages via `isUtilizationMetric`. `session_duration_minutes` should format as `"Xm"`. Add a guard before the generic `formatNumber` fallback:

In `formatMetricValue`, add after the `isByteMetric` block:

```typescript
  if (metric === 'session_duration_minutes') {
    return `${Math.round(value)}m`;
  }
```

The full updated `formatMetricValue` function:

```typescript
export function formatMetricValue(metric: string, value: number | undefined | null): string {
  if (!isFiniteValue(value)) return UNAVAILABLE_VALUE;

  if (isUtilizationMetric(metric)) {
    return `${value.toFixed(1)}%`;
  }

  if (isByteMetric(metric)) {
    return `${formatBytes(value, 'iec')}/s`;
  }

  if (metric === 'session_duration_minutes') {
    return `${Math.round(value)}m`;
  }

  return formatNumber(value);
}
```

- [ ] **Step 5: Add `session_duration_minutes` to `formatAxisValue`**

In `formatAxisValue`, add after the `isByteMetric` block:

```typescript
  if (metric === 'session_duration_minutes') {
    return `${Math.round(value)}m`;
  }
```

The full updated `formatAxisValue`:

```typescript
export function formatAxisValue(metric: string, value: number): string {
  if (isUtilizationMetric(metric)) {
    return `${Math.round(value)}%`;
  }
  if (isByteMetric(metric)) {
    return `${formatBytesCompact(value)}/s`;
  }
  if (metric === 'session_duration_minutes') {
    return `${Math.round(value)}m`;
  }
  // count / session metrics
  if (Math.abs(value) >= 10000) {
    return `${(value / 1000).toFixed(0)}k`;
  }
  if (Math.abs(value) >= 1000) {
    return `${(value / 1000).toFixed(1)}k`;
  }
  return String(Math.round(value));
}
```

- [ ] **Step 6: TypeScript check**

```bash
cd /opt/ocserv_panel/frontend && npx tsc --noEmit 2>&1 | head -20
```

Expected: no errors.

- [ ] **Step 7: Commit**

```bash
cd /opt/ocserv_panel && git add frontend/src/lib/monitoring.ts
git commit -m "feat(monitoring): add session_bytes and session_duration_minutes metric config"
```

---

### Task 5: Frontend — fix UserSessionDrawer chart metrics and X-axis labels

**Files:**
- Modify: `frontend/src/lib/components/history/UserSessionDrawer.svelte`

- [ ] **Step 1: Fix metric names in chart data derivations and chart components**

In `UserSessionDrawer.svelte`, the charts use wrong metric names. Apply these changes:

**1. Fix the `rxData` / `txData` charts** — replace the two `MetricChart` calls in the "Data usage per session" section:

Find:
```svelte
<MetricChart data={rxData} metric="net_interface_rx_bytes" height={120} chartType="bar" />
```
Replace with:
```svelte
<MetricChart data={rxData} metric="session_bytes" height={120} chartType="bar" />
```

Find:
```svelte
<MetricChart data={txData} metric="net_interface_tx_bytes" height={120} chartType="bar" />
```
Replace with:
```svelte
<MetricChart data={txData} metric="session_bytes" height={120} chartType="bar" />
```

**2. Fix the duration chart** — replace:
```svelte
<MetricChart data={durationData} metric="active_sessions" height={120} chartType="bar" />
```
With:
```svelte
<MetricChart data={durationData} metric="session_duration_minutes" height={120} chartType="bar" />
```

**3. Fix the cumulative data chart** — replace:
```svelte
<MetricChart data={cumulativeData} metric="net_interface_rx_bytes" height={120} chartType="bar" />
```
With:
```svelte
<MetricChart data={cumulativeData} metric="session_bytes" height={120} chartType="bar" />
```

- [ ] **Step 2: Fix X-axis date labels in MetricChart for sparse bar data**

The bar chart X-axis currently calls `formatXAxisTime(new Date(d))` which outputs `HH:MM`. For per-session data spanning multiple days, this is ambiguous. Add a short-date formatter to `monitoring.ts` and pass it through `MetricChart`.

In `frontend/src/lib/monitoring.ts`, add this function after `formatXAxisTime`:

```typescript
/**
 * Formats a Date as a short date label ("May 9") for bar charts where data
 * points are spaced hours or days apart rather than minutes.
 */
export function formatShortDate(d: Date | string): string {
  const date = typeof d === 'string' ? new Date(d) : d;
  return date.toLocaleDateString(undefined, { month: 'short', day: 'numeric' });
}
```

In `frontend/src/lib/components/ui/MetricChart.svelte`, add an optional `xLabelFormat` prop and use it in the bar chart branch:

Update the `Props` type:

```typescript
type Props = {
  data: TimeData[];
  metric: string;
  height?: number;
  chartType?: 'line' | 'bar';
  /** Optional custom formatter for X-axis labels on bar charts. Defaults to HH:MM. */
  xLabelFormat?: (d: string) => string;
};
```

Update destructuring:

```typescript
let { data, metric, height = 160, chartType = 'line', xLabelFormat }: Props = $props();
```

In the bar chart `<Axis placement="bottom">`:

```svelte
<Axis
  placement="bottom"
  format={(d: string) => xLabelFormat ? xLabelFormat(d) : formatXAxisTime(new Date(d))}
  ticks={xTickCount}
/>
```

Back in `UserSessionDrawer.svelte`, add the import and pass the prop to all four charts:

At the top of the `<script>` block, add to the existing imports from `$lib/monitoring`:

```typescript
import { formatBytes, formatShortDate } from '$lib/monitoring';
```

Pass `xLabelFormat` to each `MetricChart` call:

```svelte
<MetricChart data={rxData} metric="session_bytes" height={120} chartType="bar" xLabelFormat={formatShortDate} />
```

```svelte
<MetricChart data={txData} metric="session_bytes" height={120} chartType="bar" xLabelFormat={formatShortDate} />
```

```svelte
<MetricChart data={durationData} metric="session_duration_minutes" height={120} chartType="bar" xLabelFormat={formatShortDate} />
```

```svelte
<MetricChart data={cumulativeData} metric="session_bytes" height={120} chartType="bar" xLabelFormat={formatShortDate} />
```

- [ ] **Step 3: TypeScript check**

```bash
cd /opt/ocserv_panel/frontend && npx tsc --noEmit 2>&1 | head -20
```

Expected: no errors.

- [ ] **Step 4: Build check**

```bash
cd /opt/ocserv_panel/frontend && npm run build 2>&1 | tail -20
```

Expected: build succeeds with no errors.

- [ ] **Step 5: Commit**

```bash
cd /opt/ocserv_panel && git add frontend/src/lib/monitoring.ts frontend/src/lib/components/ui/MetricChart.svelte frontend/src/lib/components/history/UserSessionDrawer.svelte
git commit -m "fix(drawer): use session_bytes/session_duration_minutes metrics; fix X-axis labels"
```

---

### Task 6: Full backend test suite verification

**Files:** (no changes)

- [ ] **Step 1: Run all backend tests**

```bash
cd /opt/ocserv_panel/backend && go test ./... 2>&1 | tail -30
```

Expected: all packages PASS, no failures.

- [ ] **Step 2: Commit if there are no failures (no changes needed)**

If all tests pass, no commit required. If any test fails, investigate before proceeding.

---

## Self-Review

**Spec coverage check:**

| Spec requirement | Task |
|---|---|
| Ingester detects session disappearance, upserts `state=disconnected` | Task 1 + 2 |
| Backend `ListSessions` filters `state != 'authenticated'` | Task 3 |
| `total` count reflects only completed sessions | Task 3 (filter applies to count query too) |
| Drawer charts use indigo/slate palette matching Memory chart | Task 4 (`session_bytes` uses indigo) |
| Drawer X-axis shows readable short dates | Task 5 (`formatShortDate` + `xLabelFormat` prop) |
| No regression in Currently Active section | Not changed — uses `getConnectedUsers()` |
| No regression in Users summary table | Not changed |

**Placeholder scan:** None found — all steps contain real code.

**Type consistency:** `session_bytes` and `session_duration_minutes` are string literals defined in Task 4 (`monitoring.ts`) and consumed in Task 5 (`UserSessionDrawer.svelte`) — consistent. `xLabelFormat` prop added in Task 5 to both `MetricChart.svelte` and all call sites in `UserSessionDrawer.svelte` — consistent. `sessionSnapshot` struct defined and used within Task 2 only.
