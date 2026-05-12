# History Session Log Fixes — Design Spec

**Date:** 2026-05-12  
**Status:** Approved  
**Scope:** Session ingester disconnect detection, backend history filter, drawer chart consistency

---

## Problem Statement

Three bugs on the History → Sessions tab:

1. **Session Log "Ended" always shows Active** — sessions in the DB with `state = authenticated` (live sessions upserted by the ingester) show the green "Active" badge instead of a real disconnect time.
2. **Session Log contains live sessions** — the history log is supposed to show completed sessions only; currently-connected sessions appear in it because the ingester writes `state = authenticated` on every poll cycle and never marks them as ended.
3. **Drawer charts look inconsistent** — `UserSessionDrawer` uses wrong metric names (`net_interface_rx_bytes`, `active_sessions`) causing incorrect Y-axis formatting, wrong colors, and ugly ISO-string X-axis labels.

---

## Approach: Ingester Disconnect Detection + Backend Filter + Drawer Chart Fix

### 1. Session Ingester: Disconnect Detection

**File:** `backend/internal/app/session_ingester.go`

Add a `prevSessions map[string]bool` field (mutex-protected) to `SessionIngester`. On each poll cycle:

1. Fetch current connected sessions → build `currentIDs` set from session IDs.
2. For every session ID in `prevSessions` that is **not** in `currentIDs`, call `UpsertActiveSession` with:
   - `state = "disconnected"`
   - `disconnected_at = now()` (the moment of detection, one poll cycle after actual disconnect)
3. Replace `prevSessions` with `currentIDs`.

**Constraints:**
- No schema changes required — `UpsertActiveSession` already accepts `state` and `disconnected_at`.
- The `ON CONFLICT` upsert updates `state` and `disconnected_at` so existing rows get corrected.
- Accuracy: up to one poll interval (30 s) of latency before history reflects a session as ended — acceptable.
- Thread safety: `prevSessions` is only accessed from the poll goroutine (single-threaded poll), so a simple field assignment is sufficient; no mutex needed unless `Run` is ever called concurrently.

### 2. Backend: Filter Active Sessions from History Log

**File:** `backend/internal/store/history.go`, method `ListSessions`

Add `AND state != 'authenticated'` to both the count query and the paged data query. This:
- Excludes sessions the ingester has marked as currently live.
- Protects against the brief gap (≤30 s) between a session's actual disconnect and ingester detection.
- Does not change the `HistoricalSessionPage` API response shape.
- `total` count correctly reflects only completed sessions.

No migration needed — this is a query filter change only.

### 3. Frontend Drawer: Chart Consistency Fix

**File:** `frontend/src/lib/components/history/UserSessionDrawer.svelte`  
**File:** `frontend/src/lib/monitoring.ts` (add metric config entries)

#### Metric config additions (`monitoring.ts`)
Add two new metric keys to the chart config map:
- `session_bytes` — formats as bytes, uses indigo/purple gradient (same palette as `host_memory_utilization`)
- `session_duration_minutes` — formats as plain integer with "min" suffix, uses slate/neutral gradient

#### Drawer changes
- Replace `metric="net_interface_rx_bytes"` with `metric="session_bytes"` on RX and TX charts.
- Replace `metric="net_interface_rx_bytes"` with `metric="session_bytes"` on cumulative chart.
- Replace `metric="active_sessions"` with `metric="session_duration_minutes"` on duration chart.
- X-axis label formatter for bar charts: format ISO band key as short date (`May 9`) instead of raw ISO string. This is done inside `MetricChart.svelte`'s bar branch — pass an optional `xLabelFormat` prop, or apply a smarter default formatter that detects when timestamps are spaced > 1 hour apart and uses date-only format.
- All four charts use `chartType="bar"` (already set). No layout changes.

---

## What Is Not Changing

- `UpsertActiveSession` signature — no change.
- `historical_sessions` schema — no change.
- `listSessionsResponse` / `HistoricalSessionPage` API shape — no change.
- `UserSessionDrawer` layout (header, stat bar, session table) — no change.
- The "Currently active" section at the top of the Sessions tab — still shows live sessions from `getConnectedUsers()`, unaffected by the history filter.

---

## Acceptance Criteria

- Sessions that are currently connected do **not** appear in the Session Log table.
- Sessions that have ended appear with correct `disconnectedAt` time and correct `state` (e.g., `disconnected`).
- The Session Log `total` count reflects only completed sessions.
- Drawer RX/TX/Duration/Cumulative charts use consistent indigo/slate color palette matching the Memory utilization chart.
- Drawer chart X-axis labels show readable short dates, not raw ISO strings.
- No regression in the "Currently active" section or the Users summary table.
