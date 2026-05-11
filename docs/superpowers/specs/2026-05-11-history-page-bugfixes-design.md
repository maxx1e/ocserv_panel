# History Page Bug Fixes & Functional Gaps — Design Spec

**Date:** 2026-05-11  
**Status:** Approved  
**Scope:** History page — Sessions tab, Users tab, Bans tab, Monitoring tab, summary cards

---

## Problem Summary

Six distinct bugs and functional gaps on the History page, spanning frontend-only, frontend+backend, and backend-only changes:

| ID | Area | Issue |
|----|------|-------|
| A1 | Sessions tab | Active sessions show "Disconnected: just now" and wrong "Ended" label in drawer |
| A2 | Sessions tab | Drawer charts use line charts — should use bar/column for per-session discrete data |
| A3 | History page | Summary stat cards (Sessions, Received, Sent, Total traffic) should be removed |
| B1 | Users tab | Server error (404) returned when no sessions found for a window — should be 200 empty array |
| C1 | Bans tab | No time window selection — always returns all-time bans |
| D1 | Monitoring tab | VPN inbound/outbound shows only one line — should show one line per VPN interface |

---

## Section A1 — Active session state display

### Root cause

`session_ingester.go` sets `DisconnectedAt = now()` on every poll for still-active sessions. This makes the "Disconnected" column in the sessions table and the "Ended" column in the drawer always show a recent timestamp.

The `state` field is already correctly stored as `"authenticated"` for live sessions.

### Fix — Frontend only

**Sessions table (`+page.svelte`):**
- "Disconnected" column: if `session.state === "authenticated"` or `session.state === "connected"`, render a green `"Active"` badge instead of `formatAbsoluteTime(session.disconnectedAt)`.

**Session drawer (`UserSessionDrawer.svelte`):**
- "Ended" column: same check — if state is active, show an `"Active"` label (small green badge) instead of `formatRelativeTime(session.disconnectedAt)`.

No backend changes. The `state` field is already authoritative.

---

## Section A2 — Drawer charts: line → bar

### Root cause

`MetricChart.svelte` always renders a `LineChart`. For per-session discrete data (RX per session, duration per session, cumulative total), bar/column charts are more appropriate because each data point represents a distinct session, not a continuous time series.

### Fix — Frontend only

- Add `chartType?: "line" | "bar"` prop to `MetricChart.svelte`. Default: `"line"` (no regressions elsewhere).
- When `chartType="bar"`, render LayerChart `Bar` instead of `LineChart`.
- `UserSessionDrawer.svelte` passes `chartType="bar"` to all charts: Data usage RX/TX, Session duration, Cumulative data transferred.

---

## Section A3 — Remove History summary cards

### Decision

The 4 `CardStats` components (Sessions, Received, Sent, Total traffic) at the top of the History page are not needed.

### Fix — Frontend only

- Remove the 4 `CardStats` components and the grid wrapper from `+page.svelte`.
- Remove the `getHistoryAnalytics` call from `loadHistory`.
- Remove the now-unused `analytics` state, `totals` derived, `sampleWarning` derived, and the `calculateTotals`/`sampleDataWarning`/`numberFrom` helpers.
- Remove the sample data warning amber banner (it depends on `analytics`).
- Remove unused imports (`CardStats`, `ArrowDown`, `ArrowUp`, `Server`, `getHistoryAnalytics`, `HistoryAnalytics`).

---

## Section B1 — Users tab: 404 on empty sessions

### Root cause

`handleHistoryUserSessions` (server.go ~line 601) returns `HTTP 404` when `len(sessions) == 0`. The frontend treats any non-200 response as an error, so expanding a user's drawer on a window with no sessions produces a visible error message.

### Fix — Backend only

In `handleHistoryUserSessions`: remove the 404 guard. Return `200 []` when the session list is empty.

```go
// Before:
if len(sessions) == 0 {
    writeError(w, http.StatusNotFound, "no sessions found for user in this window")
    return
}

// After: fall through — writeJSON(w, http.StatusOK, resp) with empty resp slice
```

The drawer frontend already handles an empty array with "No sessions in this period." — no frontend change needed.

---

## Section C1 — Bans tab: time window filter

### Backend changes

**`store.ListBanEvents`** — add `since *time.Time` parameter:
- When non-nil, add `AND banned_at >= ?` to the WHERE clause and the COUNT query.
- When nil, no time filter (all-time, preserves current behaviour).

**`handleHistoryBans`** — add `window` query param:
- Valid values: `"24h"`, `"7d"`, `"30d"`, `"all"` (default `"all"`).
- For `"all"`: pass `since = nil`.
- For others: compute `since = now - historyWindowDuration(window)`, pass pointer.

### Frontend changes

- Add `bansWindow` state, default `"all"`.
- Add `WindowSelector` to the Bans tab header with windows `["24h", "7d", "30d", "all"]`.
- Update `listHistoricalBans` API function to accept an optional `window` param and append `&window=...` to the query string.
- On window change: reset `bansPage = 1`, call `loadBans()`.

---

## Section D1 — Monitoring tab: VPN per-interface lines

### Backend — Store interface extension

Add `QueryAllLabels(ctx context.Context, metric MetricName) ([]map[string]string, error)` to the `monitor.Store` interface.

- **TSDBStore:** iterate all series for the metric within a recent time window, collect distinct label sets (one map per unique `interface` label value). Return them as a slice.
- **MemoryStore:** scan all samples, collect distinct label sets by `interface` key.

### Backend — Overview handler

In `handleMonitoringOverview`, for `MetricVPNInterfaceRXBytes` and `MetricVPNInterfaceTXBytes`:

1. Call `QueryAllLabels` to discover active VPN interface names.
2. For each interface, call `QueryRange` with `Labels: map[string]string{"interface": ifaceName}`.
3. Append each resulting `Series` (which already carries `Labels` with the interface name) to the response.

All other metrics remain unchanged (single series, empty labels query).

The response `Series` slice may now contain multiple entries for the same metric name, distinguished by their `Labels["interface"]` value.

### Frontend — Multi-series chart

The monitoring overview `Series` array now contains multiple entries for VPN metrics (one per interface). The frontend needs to handle this.

**`MonitoringSeries` / `MonitoringOverview` types** — no structural changes needed; `series` is already `MonitoringSeries[]` and each `Series` has a `labels` map.

**New `MultiLineMetricChart.svelte` component:**
- Accepts `series: MonitoringSeries[]` (multiple series) and `metric: string`.
- Renders one `LineChart` per series, overlaid on the same chart area using LayerChart composition.
- Shows a small legend with interface names sourced from `series[i].labels.interface`.

**History page monitoring tab:**
- For VPN inbound/outbound chart slots, collect all `Series` where `metric === "vpn_interface_rx_bytes"` (or tx) using `allSeriesForMetric`. If more than one series, use `MultiLineMetricChart`. If exactly one, fall back to `MetricChart` as today.
- Helper: `allSeriesForMetric(monitoring, metric): MonitoringSeries[]` — returns all series matching metric name (distinct from existing `seriesByMetric` which returns a single series).

---

## Affected Files

### Backend
- `backend/internal/store/bans.go` — `ListBanEvents` signature + query (C1)
- `backend/internal/server/server.go` — `handleHistoryUserSessions` (B1), `handleHistoryBans` (C1), `handleMonitoringOverview` (D1)
- `backend/internal/monitor/types.go` — `Store` interface: add `QueryAllLabels` (D1)
- `backend/internal/monitor/tsdb.go` — implement `QueryAllLabels` (D1)
- `backend/internal/monitor/memory.go` — implement `QueryAllLabels` (D1)

### Frontend
- `frontend/src/lib/components/ui/MetricChart.svelte` — add `chartType` prop (A2)
- `frontend/src/lib/components/ui/MultiLineMetricChart.svelte` — new component (D1)
- `frontend/src/lib/components/history/UserSessionDrawer.svelte` — bar charts + active session label (A1, A2)
- `frontend/src/lib/api/history.ts` — `listHistoricalBans` adds window param (C1)
- `frontend/src/lib/monitoring.ts` — add `seriesForMetric` helper (D1)
- `frontend/src/routes/history/+page.svelte` — remove cards (A3), active session label (A1), bans window selector (C1), multi-line VPN charts (D1)

---

## Testing

- **B1:** Unit test that `handleHistoryUserSessions` returns 200 + empty array when store returns no sessions.
- **C1:** Unit test that `handleHistoryBans` passes correct `since` for each window value; store test that `ListBanEvents` filters by `since`.
- **D1:** Unit test that `QueryAllLabels` returns distinct label sets; test that `handleMonitoringOverview` returns multiple series for VPN metrics when multiple interfaces are registered.
- **A1/A2/A3:** Frontend-only — visual verification.

---

## Implementation notes

- Implement via sub-agents per architecture boundary: one backend agent (B1, C1-backend, D1-backend), one frontend agent (A1, A2, A3, C1-frontend, D1-frontend).
- Backend agent must not touch frontend files; frontend agent must not touch backend files.
- Backend agent delivers first (API contract must be stable before frontend integration).
