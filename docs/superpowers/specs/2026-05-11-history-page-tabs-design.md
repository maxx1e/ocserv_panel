# History Page Tab Overhaul — Design Spec

**Date:** 2026-05-11  
**Status:** Approved

---

## Overview

The History page (`/history`) is converted from a single long-scroll page to a fully tabbed layout. Three new tabs are added (Sessions, Users, Bans) alongside existing content reorganized into Monitoring and Settings tabs. The goal is to make VPN usage history — session timelines, bandwidth consumption per user, and IP ban history — navigable and actionable.

---

## Tab Structure

Five tabs in order:

| # | Label | Icon | Content |
|---|-------|------|---------|
| 1 | Monitoring | `BarChart2` | Existing metric charts |
| 2 | Sessions | `List` | Historical session list with state |
| 3 | Users | `Users` | Ranked bandwidth table + per-user traffic chart |
| 4 | Bans | `Shield` | Persistent ban event log |
| 5 | Settings | `Settings` | Retention config + purge action |

**Behavior:**
- Active tab tracked in `$state`, default: `Monitoring`.
- Each tab loads its own data on first activation (lazy load), not all at once.
- `WindowSelector` (24h / 7d / 30d) appears inside each tab that uses a time window (Monitoring, Sessions, Users). Not a shared global selector.
- Tab bar is sticky at the top of the page content area.

---

## Tab 1: Monitoring

Existing content relocated into this tab. No functional changes.

**Content:**
- `WindowSelector` for time range (24h / 7d / 30d)
- 9 `MetricChart` instances: CPU utilization, memory utilization, net RX/TX, VPN RX/TX, active sessions, connection rate, concurrent sessions
- `UnavailableChips` showing unavailable metrics with reasons

No backend changes required.

---

## Tab 2: Sessions

A searchable, paginated table of all historical sessions across all users, with session state visibility.

### Schema Change

Migration adds a `state` column to `historical_sessions`:

```sql
ALTER TABLE historical_sessions ADD COLUMN state TEXT NOT NULL DEFAULT 'disconnected'
  CHECK (state IN ('authenticated', 'disconnected', 'timed_out', 'error'));
```

Existing rows default to `'disconnected'`. New sessions ingested from ocserv carry the state from the occtl session data. The `UpsertActiveSession` store method and `ActiveSessionRecord` type must include the `state` field so the ingestion path (VPN watcher → store) propagates it correctly.

### Backend

**New store method:**
```go
ListSessions(ctx context.Context, since time.Time, limit, offset int) ([]HistoricalSession, int64, error)
```
Returns sessions with `connected_at >= since`, newest first, with total count for pagination.

**New API endpoint:**
```
GET /api/history/sessions?window=24h&page=1&limit=25
```
- Auth: `PermissionReadDashboard`
- Query params: `window` (24h/7d/30d, default 24h), `page` (default 1), `limit` (default 25, max 100)
- Response: `{ sessions: [...], total: int, page: int, limit: int }`

**Response shape per session:**
```json
{
  "id": 42,
  "username": "maxxie",
  "connectedAt": "2026-05-09T12:24:00Z",
  "disconnectedAt": "2026-05-09T14:10:00Z",
  "durationSeconds": 6360,
  "bytesIn": 31683792,
  "bytesOut": 994919220,
  "state": "disconnected",
  "source": "ingested"
}
```

### UI

- `WindowSelector` at top
- Client-side search input filtering by username
- Paginated table columns: Username, Connected At, Disconnected At, Duration, RX, TX, State
- State rendered as colored badge:
  - `authenticated` → green
  - `disconnected` → slate
  - `timed_out` → amber
  - `error` → red
- Clicking a row opens existing `UserSessionDrawer` scoped to that session's username (same drawer used on the Users tab)

---

## Tab 3: Users

Bandwidth consumption ranked by user, with a per-user traffic chart.

### Backend

**Existing endpoint (no change needed):**
```
GET /api/history/users?window=24h
```
Returns `[]UserSessionSummary` — used for the ranked table.

**New endpoint:**
```
GET /api/history/users/{username}/traffic?window=24h
```
- Auth: `PermissionReadDashboard`
- Returns time-bucketed RX/TX data points for chart rendering
- Bucket size: 1 hour for `24h` window, 1 day for `7d` and `30d` windows
- Queried from `historical_sessions` grouped by time bucket on `connected_at`

**Response shape:**
```json
{
  "username": "maxxie",
  "window": "7d",
  "buckets": [
    { "timestamp": "2026-05-05T00:00:00Z", "bytesIn": 12345678, "bytesOut": 98765432 },
    { "timestamp": "2026-05-06T00:00:00Z", "bytesIn": 0, "bytesOut": 0 }
  ]
}
```

Zero-filled buckets are included for gaps (no sessions in that period) to produce a continuous chart line.

### UI

- `WindowSelector` at top
- **Ranked table**: columns Username, Sessions, Total RX, Total TX, Total Traffic (RX+TX), Avg RX/session, Avg TX/session
  - Default sort: Total Traffic descending
  - All byte columns sortable
  - Clicking a row selects the user and loads their traffic chart
- **Per-user traffic chart**: `MetricChart` below the table showing RX and TX as two lines over time for the selected user
  - If no user selected: empty state — "Select a user to view their traffic over time"
  - Chart title shows selected username
  - Uses existing chart color scheme for network bytes

---

## Tab 4: Bans

A persistent log of IP ban events captured over time, enabling historical visibility beyond the live ban list.

### Schema Change

New migration creates `ban_events` table:

```sql
CREATE TABLE ban_events (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  ip TEXT NOT NULL,
  score INTEGER NOT NULL DEFAULT 0,
  reason TEXT NOT NULL DEFAULT '',
  banned_at TEXT NOT NULL,
  unbanned_at TEXT,
  created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_ban_events_ip ON ban_events(ip);
CREATE INDEX idx_ban_events_banned_at ON ban_events(banned_at DESC);
```

`unbanned_at` is NULL while the ban is active. Set when the IP disappears from the live ban list.

### Background Ban Poller

A new `BanWatcher` component polls the ocserv ban list on a configurable interval (default: 60 seconds). It runs inside the existing service lifecycle alongside `VPNWatcher`.

**Algorithm:**
1. Fetch live ban list via `GET /api/ocserv/bans` (calls `occtl -j -n show ip bans` internally)
2. Load all open ban events (where `unbanned_at IS NULL`) from `ban_events`
3. For each IP in live list not in open events → insert new `ban_events` row with `banned_at = now`
4. For each IP in open events not in live list → set `unbanned_at = now`

**Implementation:** New `BanWatcher` struct in a new `backend/internal/vpn/bans/` package. Keeping it separate from `vpn/health/` (which is focused on readiness checks) gives it a clear single responsibility. Depends on `vpn/sessions.Manager` for the live ban list and the store for persistence.

### New Store Methods

```go
InsertBanEvent(ctx context.Context, ip string, score int, reason string, bannedAt time.Time) error
CloseBanEvent(ctx context.Context, ip string, unbanAt time.Time) error
ListBanEvents(ctx context.Context, limit, offset int) ([]BanEvent, int64, error)
OpenBanEvents(ctx context.Context) ([]BanEvent, error)
```

`BanEvent` type mirrors the table columns.

### New API Endpoint

```
GET /api/history/bans?page=1&limit=25
```
- Auth: `PermissionReadDashboard`
- Returns paginated ban events, newest first
- Response: `{ events: [...], total: int, page: int, limit: int }`

**Response shape per event:**
```json
{
  "id": 7,
  "ip": "1.2.3.4",
  "score": 120,
  "reason": "failed authentication",
  "bannedAt": "2026-05-10T08:15:00Z",
  "unbannedAt": null
}
```

### UI

- No `WindowSelector` — shows all history, newest first
- Search input filtering by IP (client-side)
- Paginated table columns: IP, Score, Reason, Banned At, Unbanned At, Status
- Status badge: `Active` (red) when `unbannedAt` is null, `Unbanned` (slate) otherwise
- **Unban action**: for Active rows, owner-only "Unban" button calling existing `POST /api/ocserv/bans/{ip}/unban` endpoint; triggers ban list refresh after success
- Unban confirmation uses existing `UnbanConfirmDialog` pattern

---

## Tab 5: Settings

Existing retention settings and purge action relocated here. No functional changes.

**Content:**
- Retention days selector (30 / 90 / 180 / 365)
- Purge confirmation input (type `PURGE` to enable)
- Purge button (owner only)
- Success/error feedback messages

---

## Data Flow Summary

```
History page load
  └─ Monitoring tab (default): loads monitoring overview
  └─ Sessions tab (on first activate): GET /api/history/sessions?window=24h
  └─ Users tab (on first activate): GET /api/history/users?window=24h
       └─ Row click: GET /api/history/users/{username}/traffic?window=24h
  └─ Bans tab (on first activate): GET /api/history/bans
  └─ Settings tab (on first activate): GET /api/settings (retention value)

Background (server-side):
  BanWatcher polls occtl every 60s → upserts ban_events in SQLite
```

---

## Backend Changes Summary

| Area | Change |
|------|--------|
| Migration | Add `state` to `historical_sessions` |
| Migration | Create `ban_events` table |
| Store | `ListSessions(ctx, since, limit, offset)` |
| Store | `InsertBanEvent`, `CloseBanEvent`, `ListBanEvents`, `OpenBanEvents` |
| API | `GET /api/history/sessions` |
| API | `GET /api/history/users/{username}/traffic` |
| API | `GET /api/history/bans` |
| Service | `BanWatcher` background poller in `vpn/bans/` package |

---

## Frontend Changes Summary

| Area | Change |
|------|--------|
| `/history/+page.svelte` | Replace section layout with tab bar + lazy tab panels |
| Tab: Monitoring | Extract existing chart section into tab panel |
| Tab: Sessions | New paginated sessions table with state badge |
| Tab: Users | Ranked table + per-user `MetricChart`, reuse `UserSessionDrawer` |
| Tab: Bans | New ban events table with status badges + unban action |
| Tab: Settings | Move existing retention/purge section here |
| `$lib/api/history.ts` | Add `listSessions`, `getUserTraffic`, `listHistoricalBans` |

---

## RBAC

All new read endpoints require `PermissionReadDashboard` (owner + viewer).  
Unban action on Bans tab requires `PermissionManageBans` (owner only) — same as existing sessions ban management.

---

## Testing

- Store: `ListSessions` with time window filtering; `InsertBanEvent` / `CloseBanEvent` reconciliation logic
- API: sessions endpoint pagination; ban history endpoint; user traffic bucketing
- `BanWatcher`: unit test with `FakeManager` — new IPs get inserted, disappeared IPs get closed
- Frontend: tab activation triggers correct API call; state badge rendering; ranked table sort; chart renders with no selection (empty state)
