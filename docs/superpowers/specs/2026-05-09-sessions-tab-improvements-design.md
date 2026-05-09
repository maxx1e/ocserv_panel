# Sessions Tab Improvements Design

**Date:** 2026-05-09  
**Status:** Draft  
**Scope:** Frontend (Sessions page tabs), Backend (new endpoint), UX (refresh timer)

---

## Objective

Improve the Sessions page with:
1. Rename "Active Sessions" tab to "Active Users"
2. Configurable auto-refresh with countdown timer UI (10s/15s/20s/30s/1m)
3. Click-to-detail on All Sessions rows (using `show session [SID]`)
4. Click-to-detail on Routes rows (using `show id [ID]`)
5. Ban status badge in Ban Points table (cross-reference with banned IPs list)
6. Pagination for Events tab

---

## 1. Configurable Refresh Timer

### Current Behavior
- Fixed 30-second `setInterval` in `+page.svelte`
- "Updated X ago" text + "Refresh" button

### New Behavior
- Replace with a `RefreshTimer` component showing:
  - A circular countdown indicator (SVG ring that depletes)
  - Current interval label (e.g. "30s")
  - Dropdown to select: 10s, 15s, 20s, 30s, 1m
  - Manual refresh button (resets timer)
- Selected interval stored in `localStorage` key `sessions_refresh_interval`
- Default: 30s (backward compatible)

### Component: `RefreshTimer.svelte`
```
Props:
  - onRefresh: () => void        // callback to trigger data refresh
  - defaultInterval?: number     // default 30000ms

State:
  - selectedInterval: number     // from localStorage or default
  - timeRemaining: number        // countdown in ms
  - intervalOptions: [10000, 15000, 20000, 30000, 60000]

Visual:
  - Small circular SVG (20x20px) with stroke-dashoffset animation
  - Dropdown trigger on click of the interval label
  - Refresh icon button next to it
```

### Integration
- `+page.svelte` replaces `refreshInterval`/`tickInterval` logic with `<RefreshTimer onRefresh={refreshData} />`
- Timer pauses when a confirmation dialog is open (same as current guard)

---

## 2. Tab Rename: Active Sessions → Active Users

### Changes
- `+page.svelte`: tab definition `{ id: 'active', label: 'Active Users' }`
- `ActiveSessionsTab.svelte`: CardTable title "Active Users ({users.length})"
- No backend changes (endpoint returns connected users, semantics unchanged)

---

## 3. All Sessions: Click-to-Detail

### Current Behavior
- Table rows are non-interactive (no hover cursor, no click handler)
- Shows: username, session ID, status, VPN IP, client IP, RX/TX, connected time, client

### New Behavior
- Rows become clickable (`cursor-pointer`, `hover:bg-slate-50`)
- On click: call `getOcservSessionDetail(session.id)` — **endpoint already exists**: `GET /api/ocserv/sessions/{id}`
- Open `SessionDetailDrawer` with the returned `OcservSessionDetail` data
- Highlight selected row with `bg-blue-50`
- Same split-panel pattern as ActiveSessionsTab (table left, drawer right)

### Frontend Changes (`AllSessionsTab.svelte`)
- Add state: `selectedSession`, `sessionDetail`, `detailLoading`, `detailError`
- Add click handler on `<tr>` → `selectSession(session)`
- Add `SessionDetailDrawer` rendering when `selectedSession !== null`
- Reuse existing drawer component (it already accepts `detail: OcservSessionDetail | null`)

### No Backend Changes
- `GET /api/ocserv/sessions/{id}` already calls `GetSessionDetails(ctx, id)` in the adapter
- The session ID is the short form from `show sessions all` output (e.g. "YwDQr5")

---

## 4. Routes: Click-to-Detail via `show id [ID]`

### New Backend Method

**Interface addition** (`client.go`):
```go
// GetConnectionByID returns full connection details for a numeric connection ID.
GetConnectionByID(ctx context.Context, id int) (*UserConnection, error)
```

**OcctlAdapter implementation**:
- Command: `occtl -j -n show id <ID>`
- Timeout: 10 seconds
- Validation: ID must be a positive integer
- Parse: same JSON structure as `show user` (returns single connection object)
- Returns `*UserConnection` (single result, not slice — an ID maps to one connection)

**FakeManager addition**:
- `GetConnectionByID(ctx, id) → *UserConnection` with configurable return

**New handler** (`ocserv_handlers.go`):
- `GET /api/ocserv/connections/{id}` → `handleOcservConnectionDetail()`
- Path param `id` parsed as integer
- Permission: `ocserv:sessions:read` (same as listing)
- Returns `UserConnection` JSON

**New service method** (`ocserv_service.go`):
- `GetConnectionByID(ctx, principal, id int) (*UserConnection, error)`
- RBAC check: `ocserv:sessions:read`
- Audit: read-only, no audit record needed

### Frontend Changes

**New API function** (`api.ts`):
```typescript
export async function getConnectionById(id: number): Promise<UserConnection> {
  return apiFetch<UserConnection>(`/api/ocserv/connections/${id}`);
}
```

**RoutesTab.svelte**:
- Rows become clickable
- On click: call `getConnectionById(route.id)` 
- Open a detail panel showing full connection info (device, IPs, cipher, bandwidth, hostname, DNS, routes)
- Reuse the same drawer pattern or a simplified version showing `UserConnection` fields

### Detail Panel for Routes
Reuse `SessionDetailDrawer` which already accepts `userConnection` prop. Pass the fetched `UserConnection` directly.

---

## 5. Bans: Status Badge

### Current Behavior
- Two separate tables: "Banned IPs" (with unban button) and "IP Ban Points"
- No cross-reference between them

### New Behavior
- In the "IP Ban Points" table, add a **Status** column
- For each `BanPointEntry`, check if `entry.ip` exists in the `bans` list:
  - If banned: red badge **"Banned"** + unban button
  - If not banned but points > 0: amber badge **"Warning"**
- This is frontend-only logic (both datasets already fetched in parallel)

### Frontend Changes (`BansTab.svelte`)
- Add a derived `bannedSet = new Set(bans.map(b => b.ip))`
- In the ban points table, add Status column with badge
- Move unban button into the ban points table for banned IPs (consolidates UX)
- Keep the standalone "Banned IPs" table as a summary at the top

---

## 6. Events: Pagination

### Current Behavior
- All events rendered in a single table (no pagination)
- Could become unwieldy with many events

### New Behavior
- Add `PaginatedTable` wrapper (20 items per page, same as other tabs)
- Add sorting by timestamp (default: newest first)
- Existing refresh behavior stays (now driven by configurable timer)

### Frontend Changes (`EventsTab.svelte`)
- Import `PaginatedTable` and `paginate` utility
- Add page state and derived `pagedEvents`
- Wrap table in `PaginatedTable`

---

## File Impact Summary

### Backend (3 files modified, 0 new)
| File | Change |
|------|--------|
| `backend/internal/ocserv/client.go` | Add `GetConnectionByID` to `Manager` interface + `UserConnection` note |
| `backend/internal/ocserv/occtl_adapter.go` | Implement `GetConnectionByID` |
| `backend/internal/ocserv/occtl_adapter_test.go` | Tests for new method |
| `backend/internal/app/ocserv_service.go` | Add `GetConnectionByID` service method |
| `backend/internal/server/ocserv_handlers.go` | Add `handleOcservConnectionDetail` handler |
| `backend/internal/server/ocserv_handlers_test.go` | Handler test |
| `backend/internal/server/server.go` | Register route |
| `backend/internal/ocserv/passwd_fake.go` (FakeManager) | Add stub |

### Frontend (6 files modified, 1 new)
| File | Change |
|------|--------|
| `frontend/src/routes/sessions/+page.svelte` | Replace refresh logic, rename tab |
| `frontend/src/routes/sessions/RefreshTimer.svelte` | **New** — countdown + interval selector |
| `frontend/src/routes/sessions/ActiveSessionsTab.svelte` | Update CardTable title |
| `frontend/src/routes/sessions/AllSessionsTab.svelte` | Add click-to-detail drawer |
| `frontend/src/routes/sessions/EventsTab.svelte` | Add pagination |
| `frontend/src/routes/sessions/RoutesTab.svelte` | Add click-to-detail drawer |
| `frontend/src/routes/sessions/BansTab.svelte` | Add status badges |
| `frontend/src/lib/api.ts` | Add `getConnectionById` function |

---

## Testing

- **Backend**: Unit test for `GetConnectionByID` in adapter (mock command output)
- **Backend**: Handler test for `GET /api/ocserv/connections/{id}` (valid ID, invalid ID, not found)
- **Frontend**: Build verification (TypeScript compilation)
- **Integration**: Manual verification with live ocserv

---

## Risks & Mitigations

| Risk | Mitigation |
|------|-----------|
| `show id [ID]` output format differs from `show user` | Parse with same fallback chain (JSON → text) |
| Fast refresh (10s) overloads ocserv socket | Each command has 10s timeout; if slow, UI shows stale data gracefully |
| Session ID might not match between `show sessions all` and `show session [SID]` | Use the short `Session` field directly (matches occtl expectations) |
