# Sessions & History Data Accuracy — Design Spec

**Date:** 2026-05-12  
**Status:** Approved  

---

## Problem Statement

Two UI correctness issues exist today:

1. **Sessions page / All Sessions tab** shows `VPN IP` and `RX/TX` columns populated from `show sessions all`, but those fields are not present in that command's output — they come from `show users`. The columns show empty/zero data. The `Connected` column may also not render because the `connectedAt` field is dropped in the HTTP response mapping.

2. **History page / Sessions and Users tabs** are separate, but the user wants them unified: a single `Sessions` tab with per-user aggregated stats at the top (Section A) and the historical session log below (Section B), linked by username click.

---

## Area 1: Sessions Page — All Sessions Tab Data Fix

### occtl Data Sources

| Command | Available Fields |
|---------|-----------------|
| `occtl show sessions all` | Session ID, Full session, Username, Groupname, vhost, User-Agent, Remote IP, Location, State, session_is_open, tls_auth_ok, in_use, **Created** |
| `occtl show users` | All above + **IPv4, RX, TX, Device, MTU, LocalDeviceIP, AvgRX, AvgTX, Hostname, TLSCipher, DTLSCipher, DPD, KeepAlive** |

`show sessions all` does **not** include VPN IP or RX/TX. `show users` does include them via `raw_connected_at` and the session's `Full session` ID for cross-reference.

### Backend Change: Enriched Session Response

The `GET /api/ocserv/sessions` handler currently calls only `ListSessions()` (which runs `show sessions all`). It will be changed to:

1. Call `ListSessions()` → `[]Session` (from `show sessions all`)
2. Call `ListConnectedUsers()` → `[]ConnectedUser` (from `show users`)
3. Build a lookup map: `FullSession → ConnectedUser`
4. For each `Session`, attempt to find a matching `ConnectedUser` by `FullSession` field
5. Return an enriched response where `vpnAddress`, `rxBytes`, `txBytes`, `device`, `mtu`, `avgRxBandwidth`, `avgTxBandwidth` come from the matched `ConnectedUser` (or zero/empty if no match)

**Merge key:** `Session.FullSession` matches `ConnectedUser.FullSession`. Sessions in `authenticate` state (not yet connected) will have no match and show `—` for merged fields.

### Connected Column Fix

The `Session.ConnectedAt` is already parsed from `"Created"` in the adapter. The HTTP response struct `ocservSessionResponse` must include `connectedAt` (RFC3339 string). Verify `ocservSessionToResponse()` in the handler maps `s.ConnectedAt` → `connectedAt` correctly.

### All Sessions Table Columns (after fix)

| Column | Source | Notes |
|--------|--------|-------|
| Username | `show sessions all` → `Username` | |
| Session ID | `show sessions all` → `Session` | Truncated to 8 chars |
| Status | `show sessions all` → `State` | Color-coded badge |
| Client IP | `show sessions all` → `Remote IP` | |
| VPN IP | `show users` merge → `IPv4` | `—` if not yet connected |
| RX / TX | `show users` merge → `RX`/`TX` | `—` if not yet connected |
| Connected | `show sessions all` → `Created` → `ConnectedAt` | Relative time ("3h ago") |
| Client | `show sessions all` → `User-Agent` | Truncated |

### Session Detail Drawer

The drawer already calls `GET /api/ocserv/sessions/{id}` (which runs `show session <id>`) and `GET /api/ocserv/users/{username}` for enrichment. No structural change needed — the drawer already handles the merge correctly. Only verify that `connectedAt` is populated in the drawer too.

---

## Area 2: History Page — Sessions Tab Consolidation

### Current State

Two tabs: `Sessions` (historical session log) and `Users` (per-user aggregated stats with expandable traffic charts).

### New State

Single `Sessions` tab replaces both. The tab has two visual sections:

```
┌─────────────────────────────────────────────────────────────────┐
│  Sessions  [time window: 24h ▾]  [search by username __]       │
├─────────────────────────────────────────────────────────────────┤
│  SECTION A — Per-User Summary                                   │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │ ● User   Sessions  Total time  ↓ Total  ↑ Total  ↕ Total│   │
│  │ alice    12        14h 20m     1.2 GB   300 MB   1.5 GB │   │
│  │ bob       3         2h 10m     50 MB     12 MB   62 MB  │   │
│  └─────────────────────────────────────────────────────────┘   │
│                                                                 │
│  SECTION B — Session Log                        [page 1/4]     │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │ Username  Connected   Ended   Duration  RX    TX  State │   │
│  │ alice     May 11 ...  ...     3h 20m    ...   ...  ●    │   │
│  └─────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────┘
```

### Time Window Selector

Placed at the top of the tab. Applies to both sections simultaneously. Options: 24h, 7d, 30d (existing options, preserved).

### Search / Username Filter

A single search field at the top filters **both** sections by username substring match. Typing a username narrows both the user summary rows in Section A and the session log rows in Section B.

Clicking a username row in Section A pre-fills the search field with that exact username, effectively cross-filtering Section B to show only that user's sessions.

### Section A: Per-User Summary Table

**Columns:**
| Column | Field | Notes |
|--------|-------|-------|
| User | `username` | Avatar initials + active status dot (green if user has active sessions) |
| Sessions | `sessionCount` | Count in selected window |
| Total time | `totalDurationSeconds` | Formatted: "1d 2h 30m" |
| ↓ Total | `totalBytesIn` | Human-readable bytes |
| ↑ Total | `totalBytesOut` | Human-readable bytes |
| ↕ Total | `totalBytesIn + totalBytesOut` | Bold highlight |

**Interaction:** Clicking a row opens a side drawer (not row expansion).

### User Detail Drawer

Triggered by clicking a username row in Section A.

**Drawer contents:**
1. **Header:** Username, avatar, active/inactive badge, time window context
2. **Stats cards (4 cards):**
   - Total sessions (count)
   - Total active time (formatted duration)
   - Total data downloaded (↓)
   - Total data uploaded (↑)
3. **Traffic chart:** RX and TX over time (uses existing `UserTrafficResponse` from `/api/history/users/{username}/traffic`)
4. **Session list table:** Individual sessions for this user — columns: Connected, Ended, Duration, RX, TX, State (same as Section B but pre-filtered)

### Section B: Session Log

Functionally identical to the existing History Sessions tab. Columns unchanged: Username, Connected, Ended, Duration, RX, TX, State. Pagination and page size unchanged (25 per page). Search pre-populated if user clicked from Section A.

---

## API Changes

### Sessions Page

**`GET /api/ocserv/sessions`** — response shape gains two optional enriched fields:

```json
{
  "id": "YwDQr5",
  "username": "maxxie",
  "remoteAddress": "5.166.153.114",
  "connectedAt": "2026-05-09T12:24:00Z",
  "vpnAddress": "10.10.10.208",
  "rxBytes": 304,
  "txBytes": 304,
  "device": "vpns0",
  "userAgent": "Open AnyConnect VPN Agent 9.12-3",
  "state": "authenticated",
  "groupname": "(none)",
  "vhost": "default",
  "enriched": true
}
```

When `enriched` is `false` (no matching `show users` entry), `vpnAddress`, `rxBytes`, `txBytes`, `device` will be empty/zero. The frontend shows `—` for empty enriched fields.

### History Page

No new API endpoints required. Existing endpoints are sufficient:
- `GET /api/history/users?window=` — Section A data
- `GET /api/history/sessions?window=&page=&limit=&username=` — Section B data (username filter may need to be added if not already present)
- `GET /api/history/users/{username}/traffic?window=` — Drawer chart data
- `GET /api/history/users/{username}?window=` — Drawer session list

Verify `username` query param filter exists on `GET /api/history/sessions`. If missing, add it.

---

## Frontend Component Changes

### Sessions Page (`/sessions`)

**`AllSessionsTab.svelte`** (or equivalent):
- Remove `vpnAddress` and `rxBytes/txBytes` column rendering that assumed those fields always exist
- Show `—` (em dash) when `enriched === false` or fields are empty/zero for enriched sessions
- Fix `Connected` column to render `session.connectedAt` as relative time (already coded, verify the data flows through)

### History Page (`/history`)

**`+page.svelte`** (History):
- Remove `Users` tab from tab list
- Rename `Sessions` tab to `Sessions` (keep label, but restructure content)
- Pass `usernameFilter` state shared between Section A and Section B

**New `SessionsTab.svelte`** (combined):
- Imports: `UserSummarySection.svelte`, `SessionLogSection.svelte`
- Shared state: `selectedWindow`, `usernameFilter`
- Layout: stacked vertically with a visible section separator

**`UserSummarySection.svelte`** (renamed/evolved from old `UsersSection` in Users tab):
- Table with avatar rows, active dot, sortable by traffic
- Row click → set `usernameFilter` + open drawer
- No row expansion (drawer replaces it)

**`UserDetailDrawer.svelte`** (new):
- Triggered by `UserSummarySection` row click
- Fetches: `getUserTraffic(username, window)`, `getHistoryUserSessions(username, window)`
- Renders: stats cards, `ChartCard` for RX/TX, session list table

**`SessionLogSection.svelte`** (extracted from existing Sessions tab):
- Accepts `usernameFilter` prop
- Passes `username` param to `GET /api/history/sessions` when set

---

## Data Flow Summary

```
Sessions Page / All Sessions tab:
  render ← enrich(ListSessions(), ListConnectedUsers()) [backend join]
  drawer ← GetSessionDetails(id) + GetUserConnections(username) [existing, unchanged]

History Page / Sessions tab:
  Section A ← GET /api/history/users?window=
  Section B ← GET /api/history/sessions?window=&username=
  Drawer ← GET /api/history/users/{u}/traffic + GET /api/history/users/{u}?window=
```

---

## Out of Scope

- Changing the Sessions page tabs other than `All Sessions`
- Backend storage schema changes
- Monitoring tab on History page
- Bans tab changes
