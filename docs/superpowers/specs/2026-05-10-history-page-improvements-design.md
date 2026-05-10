# History Page Improvements Design

**Date:** 2026-05-10
**Status:** Approved

## Overview

Improve the history page to align monitoring charts with the dashboard approach, and replace the raw session table with a per-user aggregated stats view. Clicking a user row opens a drawer with multiple usage charts for that user.

## Goals

- Monitoring charts on history page mirror dashboard chart grouping and visual style
- Single window selector (24h / 7d / 30d) controls both monitoring charts and session stats table
- "Recent Historical Sessions" becomes a per-user aggregated stats table
- Clicking a user row opens a right-side drawer with per-session charts

## Backend Changes

### New Store Methods

Add to `backend/internal/store/history.go`:

```go
type UserSessionSummary struct {
    Username             string
    SessionCount         int64
    TotalDurationSeconds int64
    TotalBytesIn         int64
    TotalBytesOut        int64
    AvgBytesIn           int64
    AvgBytesOut          int64
}

func (s *Store) UserSessionSummaries(ctx context.Context, since time.Time) ([]UserSessionSummary, error)
func (s *Store) UserSessions(ctx context.Context, username string, since time.Time) ([]HistoricalSession, error)
```

`UserSessionSummaries` groups `historical_sessions` by `username` filtered by `connected_at >= since`, ordered by `TotalBytesOut + TotalBytesIn DESC`.

`UserSessions` returns all sessions for a given username filtered by `connected_at >= since`, ordered by `connected_at ASC` (chronological for charting).

### Window-to-Duration Mapping

```go
func windowDuration(window string) (time.Duration, error) {
    switch window {
    case "24h": return 24 * time.Hour, nil
    case "7d":  return 7 * 24 * time.Hour, nil
    case "30d": return 30 * 24 * time.Hour, nil
    default:    return 0, fmt.Errorf("unsupported window: %s", window)
    }
}
```

### New API Endpoints

Both endpoints require `PermissionReadDashboard`. Both accept `?window=24h|7d|30d`, defaulting to `24h`.

**`GET /api/history/users?window=24h`**

Returns per-user aggregated stats:

```json
[
  {
    "username": "maxxie",
    "sessionCount": 5,
    "totalDurationSeconds": 97200,
    "totalBytesIn": 31700355,
    "totalBytesOut": 995051116,
    "avgBytesIn": 6340071,
    "avgBytesOut": 199010223
  }
]
```

**`GET /api/history/users/{username}?window=24h`**

Returns individual sessions for the user in the window, same shape as existing `historicalSessionResponse`:

```json
[
  {
    "id": 42,
    "username": "maxxie",
    "connectedAt": "2026-05-09T12:24:00Z",
    "disconnectedAt": "2026-05-09T16:16:00Z",
    "durationSeconds": 14520,
    "bytesIn": 304,
    "bytesOut": 304,
    "source": "ingested"
  }
]
```

Username is validated — reject empty or path-traversal attempts. Return 404 if no sessions found for the user in the window.

### Wiring

Add handlers in `backend/internal/server/server.go`:
- `handleHistoryUsers` — calls new store method, maps to response structs
- `handleHistoryUserSessions` — extracts username from path, calls store, returns session slice

Register routes in `newRouter`:
```
GET /api/history/users           → requirePermission(PermissionReadDashboard)(handleHistoryUsers)
GET /api/history/users/{username} → requirePermission(PermissionReadDashboard)(handleHistoryUserSessions)
```

## Frontend Changes

### API Layer

Add to `frontend/src/lib/api/history.ts`:

```typescript
export type UserSessionSummary = {
  username: string;
  sessionCount: number;
  totalDurationSeconds: number;
  totalBytesIn: number;
  totalBytesOut: number;
  avgBytesIn: number;
  avgBytesOut: number;
};

export async function getHistoryUsers(window: string): Promise<UserSessionSummary[]>
export async function getHistoryUserSessions(username: string, window: string): Promise<HistoricalSession[]>
```

### Window Selector Scope

The existing `WindowSelector` component on the history page currently controls only the monitoring charts. It is expanded to also control the per-user stats table. A single `$state` variable `selectedWindow: MonitoringWindow` drives both `getMonitoringOverview(selectedWindow)` and `getHistoryUsers(selectedWindow)`.

History-specific windows: `24h`, `7d`, `30d` (same as existing history selector). The 15m/1h/6h options used on the dashboard are not relevant here and are not shown.

### Per-User Stats Table

Replaces the current raw session table. Columns:

| User | Sessions | Total Time | Avg Download | Avg Upload | Total Received | Total Sent | Total |
|------|----------|------------|--------------|------------|----------------|------------|-------|

- **User** — username string
- **Sessions** — integer count
- **Total Time** — formatted as `Xd Xh Xm` or `Xh Xm`
- **Avg Download / Avg Upload** — `avgBytesIn` / `avgBytesOut` formatted as human-readable bytes
- **Total Received / Total Sent** — `totalBytesIn` / `totalBytesOut` formatted
- **Total** — `totalBytesIn + totalBytesOut` formatted

Each row: `cursor-pointer`, hover highlight (same as existing table rows). Clicking calls `getHistoryUserSessions(username, selectedWindow)` and opens the drawer.

Pagination: 10 rows per page, same as existing table. Empty state when no sessions in window.

### User Detail Drawer

A right-side slide-in drawer component at `frontend/src/lib/components/history/UserSessionDrawer.svelte`.

**Props:**
```typescript
{
  username: string;
  window: string;
  sessions: HistoricalSession[];
  onclose: () => void;
}
```

**Header:** Username + window label (e.g. "maxxie — last 24h"). Close (X) button top-right.

**Charts** (using existing `MetricChart` pattern and chart colors):

1. **Data usage per session** — dual line chart, RX bytes and TX bytes per session plotted by `connectedAt` timestamp. X-axis: session start time. Y-axis: bytes (auto-scaled).
2. **Session duration** — bar chart, duration in minutes per session plotted by `connectedAt`. X-axis: session start time. Y-axis: minutes.
3. **Cumulative data** — line chart, running total of `bytesIn + bytesOut` across sessions ordered chronologically.

**Session list** below charts: compact table with Started, Ended, Duration, RX, TX columns. No pagination (all sessions for the window).

Drawer closes via X button or clicking the backdrop overlay.

### Chart Alignment with Dashboard

The monitoring section of the history page retains its existing charts but groups them consistently with the dashboard:

- **Resources:** CPU utilization, Memory utilization (2-col grid)
- **Network:** Net interface RX, Net interface TX (2-col grid)
- **VPN:** VPN interface RX, VPN interface TX (2-col grid)
- **Sessions:** Active sessions, Connection rate, Concurrent sessions vs limit (2-col grid, last card full-width or 2-col)

Uses the same `MetricChart` component, same color/gradient definitions from `monitoring.ts`. No new chart components needed.

## Data Flow

```
User selects window (24h/7d/30d)
  ├── getMonitoringOverview(window) → monitoring charts update
  └── getHistoryUsers(window) → per-user table updates

User clicks a row
  └── getHistoryUserSessions(username, window) → drawer opens with charts

User closes drawer
  └── drawer unmounts, no state persisted
```

## Error Handling

- Loading states: skeleton rows in table, spinner in drawer while fetching sessions
- Empty window: show "No sessions in this period" empty state
- API error: show inline error message, allow retry

## Testing

**Backend:**
- `UserSessionSummaries` returns correct aggregates for multi-session users
- `UserSessions` returns sessions filtered by window and username
- `handleHistoryUsers` returns 401 without auth, 200 with correct shape
- `handleHistoryUserSessions` returns 404 for unknown username, 200 with sessions

**Frontend:**
- Per-user table renders correct formatted values
- Window change triggers re-fetch of both monitoring and user stats
- Drawer opens on row click, closes on X and backdrop click
- Empty state shown when no sessions returned

## Files Changed

| File | Change |
|------|--------|
| `backend/internal/store/history.go` | Add `UserSessionSummary` type, `UserSessionSummaries`, `UserSessions` methods |
| `backend/internal/server/server.go` | Add `handleHistoryUsers`, `handleHistoryUserSessions`, register routes |
| `frontend/src/lib/api/history.ts` | Add `UserSessionSummary` type, `getHistoryUsers`, `getHistoryUserSessions` |
| `frontend/src/lib/components/history/UserSessionDrawer.svelte` | New drawer component |
| `frontend/src/routes/history/+page.svelte` | Wire unified window selector, replace session table, integrate drawer |
