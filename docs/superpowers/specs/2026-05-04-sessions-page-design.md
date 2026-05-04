# Sessions Page Design

**Date:** 2026-05-04
**Status:** Approved

## Overview

Implement a fully functional Sessions tab in the ocserv panel admin UI. The page provides live visibility into connected VPN sessions with traffic stats, client metadata, assigned routes/DNS, and safe session control (disconnect and terminate) — all backed by real `occtl` integration.

## Approach

**Table-first with slide-in detail panel.** The sessions list is the primary view. Clicking a row opens a detail panel alongside the table (no navigation). Auto-refreshes every 30 seconds. Both disconnect and terminate require a confirmation modal with clear explanatory text distinguishing their effects.

---

## Section 1: Backend Changes

### 1.1 Expanded Data Structures (`backend/internal/ocserv/client.go`)

**`Session` struct** — enriched to include enough data for the table row without a per-row detail fetch:

```go
type Session struct {
    ID            string    `json:"ID"`
    Username      string    `json:"Username"`
    RemoteAddress string    `json:"RemoteAddress"`
    ConnectedAt   time.Time `json:"ConnectedAt"`
    VPNAddress    string    `json:"VPNAddress"`    // IPv4/IPv6 assigned to client
    RxBytes       int64     `json:"RxBytes"`       // cumulative received bytes
    TxBytes       int64     `json:"TxBytes"`       // cumulative transmitted bytes
    UserAgent     string    `json:"UserAgent"`     // client software string
    Device        string    `json:"Device"`        // VPN device name
    ReconnectCount int      `json:"ReconnectCount"` // reconnect detection counter
}
```

**`SessionDetail` struct** — full data for the side panel:

```go
type SessionDetail struct {
    ID             string        `json:"id"`
    Username       string        `json:"username"`
    RemoteAddress  string        `json:"remote_address"`
    ConnectedAt    time.Time     `json:"connected_at"`
    Duration       time.Duration `json:"duration"`
    VPNAddress     string        `json:"vpn_address"`
    RxBytes        int64         `json:"rx_bytes"`
    TxBytes        int64         `json:"tx_bytes"`
    UserAgent      string        `json:"user_agent"`
    Device         string        `json:"device"`
    IPv4           string        `json:"ipv4"`
    IPv6           string        `json:"ipv6"`
    Routes         []string      `json:"routes"`
    DNS            []string      `json:"dns"`
    ReconnectCount int           `json:"reconnect_count"`
}
```

### 1.2 Manager Interface (`backend/internal/ocserv/client.go`)

Add `TerminateSession` to the `Manager` interface:

```go
type Manager interface {
    ListSessions(ctx context.Context) ([]Session, error)
    DisconnectSession(ctx context.Context, sessionID string) error
    TerminateSession(ctx context.Context, sessionID string) error
    GetSessionDetails(ctx context.Context, id string) (*SessionDetail, error)
    ListUsers(ctx context.Context) ([]User, error)
}
```

`TerminateSession` maps to `occtl terminate id <sessionID>` — drops the connection and invalidates the session cookie.

Update `FakeManager` to implement the new method with a configurable `TerminateErr` field (default `ErrNotConfigured`).

### 1.3 OcctlAdapter (`backend/internal/ocserv/occtl_adapter.go`)

Add `TerminateSession`:
```go
func (a *OcctlAdapter) TerminateSession(ctx context.Context, sessionID string) error {
    // occtl terminate id <sessionID>
    // Timeout: 15s
}
```

The existing `GetSessionDetails` uses `occtl show id <id> --output-type json`. Extend the parsed struct to capture `Routes`, `DNS`, `ReconnectCount`, `UserAgent`, and `VPNAddress` fields from the occtl JSON output.

### 1.4 RBAC (`backend/internal/rbac/rbac.go`)

New permission:
```go
PermissionOcservSessionsTerminate Permission = "ocserv:sessions:terminate"
```

Grant matrix:
- `owner`: gets `PermissionOcservSessionsTerminate` (in addition to existing disconnect)
- `viewer`: read-only, no terminate or disconnect

### 1.5 OcservService (`backend/internal/app/ocserv_service.go`)

New methods:

**`TerminateSession(ctx, principal, sessionID)`**
- Requires `PermissionOcservSessionsTerminate`
- Calls `sessions.TerminateSession(ctx, sessionID)`
- Produces audit record: action `"ocserv.sessions.terminate"`, resource `"ocserv_session"`, resourceID = sessionID
- Logs failure with outcome `audit.OutcomeFailure`

**`GetSessionDetail(ctx, principal, sessionID)`**
- Requires `PermissionOcservSessionsRead`
- Calls `sessions.GetSessionDetails(ctx, sessionID)`
- No audit record (read-only)

### 1.6 API Handlers (`backend/internal/server/ocserv_handlers.go`)

New handler and response type:

```go
type ocservSessionDetailResponse struct {
    ID             string   `json:"id"`
    Username       string   `json:"username"`
    RemoteAddress  string   `json:"remoteAddress"`
    VPNAddress     string   `json:"vpnAddress"`
    ConnectedAt    string   `json:"connectedAt,omitempty"`
    DurationSecs   int64    `json:"durationSeconds"`
    RxBytes        int64    `json:"rxBytes"`
    TxBytes        int64    `json:"txBytes"`
    UserAgent      string   `json:"userAgent"`
    Device         string   `json:"device"`
    IPv4           string   `json:"ipv4"`
    IPv6           string   `json:"ipv6"`
    Routes         []string `json:"routes"`
    DNS            []string `json:"dns"`
    ReconnectCount int      `json:"reconnectCount"`
}

type terminateSessionRequest struct {
    SessionID string `json:"sessionId"`
}
```

New route handlers:
- `handleOcservSessionDetail` — `GET /api/ocserv/sessions/{id}`
- `handleOcservSessionTerminate` — `POST /api/ocserv/sessions/terminate`

**Updated list response:** `ocservSessionResponse` gains `vpnAddress`, `rxBytes`, `txBytes`, `userAgent`, `device`, `reconnectCount` fields to match the enriched `Session` struct.

### 1.7 Routes (`backend/internal/server/server.go`)

```go
mux.HandleFunc("/api/ocserv/sessions/{id}", appHandler.handleOcservSessionDetail)
mux.HandleFunc("/api/ocserv/sessions/terminate", appHandler.handleOcservSessionTerminate)
```

Note: `/api/ocserv/sessions/terminate` must be registered **before** the `{id}` pattern to avoid ambiguity in Go's `net/http` mux.

### 1.8 Sudoers (`deploy/sudoers.d/ocserv-panel`)

Add terminate command:
```
ocserv-panel ALL=(root) NOPASSWD: /usr/local/bin/occtl terminate id *
```

---

## Section 2: Frontend — Sessions Table (`frontend/src/routes/sessions/+page.svelte`)

### Layout

```
[Page header: "Active Sessions" | count badge | "Updated Xs ago" | Refresh button]
[Summary chips: Connected: N | Total RX: X MB | Total TX: Y MB]
[CardTable]
  [Table with columns below]
  [PaginatedTable for > 20 rows]
[Side panel (conditionally rendered alongside table when a row is selected)]
```

### Table Columns

| Column | Content | Sortable |
|--------|---------|---------|
| Username | text | yes |
| Session ID | first 8 chars + copy-to-clipboard icon | no |
| VPN IP | `vpnAddress` | no |
| Client IP | `remoteAddress` | no |
| Connected | relative time with absolute tooltip | yes |
| Duration | formatted (2h 14m) — red badge if > threshold | yes |
| RX / TX | formatted bytes side by side | no |
| Client | short name from userAgent: "AnyConnect", "OpenConnect", or "Unknown" | no |
| Stability | green dot (stable) or yellow chip with count (reconnecting) | no |
| Actions | Disconnect (orange) + Terminate (red) buttons | no |

**Long-running flag:** Duration > 24h (default threshold, read from settings or hardcoded initially) renders as a red `<span>` badge.

**Stability:** `reconnectCount > 2` → yellow chip "⚠ N reconnects". Otherwise green dot.

**Action button visibility:** Both buttons respect RBAC — hidden if `!hasPermission(principal, 'ocserv:sessions:disconnect')` / `ocserv:sessions:terminate`.

### Auto-refresh

```ts
let refreshInterval = setInterval(() => loadSessions(), 30_000);
onDestroy(() => clearInterval(refreshInterval));
```

On each refresh:
- Re-fetch session list
- If selected session ID no longer in list → close panel, show inline dismissable banner: "Session {id} is no longer active"
- Update "Updated Xs ago" counter

### Empty / Error States

| Condition | Display |
|-----------|---------|
| 503 / ErrNotConfigured | `EmptyState` with "ocserv socket not available. Is ocserv running?" |
| 403 | `EmptyState` with "Your role cannot view sessions." |
| Empty array | Inline message: "No active sessions" |
| Network error | Error banner at top |

---

## Section 3: Frontend — Session Detail Side Panel

A slide-in panel at the right side of the screen, rendered only when `selectedSessionId !== null`.

### Panel Structure

```
[Header: Username — Session abc12345  ×]

Connection Info
  VPN IPv4: 10.8.0.5
  VPN IPv6: —
  Client IP: 203.0.113.42
  Connected: 2026-05-04 14:22:01 UTC
  Duration: 2h 14m
  Device: vpns0

Traffic
  Received: 142.3 MB
  Transmitted: 38.1 MB

Client
  User Agent: OpenConnect/8.20 (Linux)
  Stability: ⚠ 3 reconnects

Assigned Routes
  • 10.0.0.0/8
  • 192.168.1.0/24
  (or "None assigned")

Assigned DNS
  • 10.8.0.1
  • 8.8.8.8
  (or "None assigned")

[Disconnect button (orange)]  [Terminate button (red)]
```

### Fetch Behavior

- Row click → set `selectedSessionId` → trigger `GET /api/ocserv/sessions/{id}`
- While loading: skeleton placeholder in panel
- 404 response: "Session no longer active" message, panel stays open with close button
- Clicking a different row while panel is open: replaces detail (no stacking)

### Confirmation Modals

**Disconnect modal:**
> "Disconnect session **{username}**?"
> This will drop the current connection. The user may reconnect if their session cookie is still valid.
> [Cancel] [Disconnect]

**Terminate modal:**
> "Terminate session **{username}**?"
> This will drop the connection and invalidate the session cookie. The user must log in again.
> [Cancel] [Terminate]

Both modals show an inline error if the action fails (modal stays open for retry).

---

## Section 4: Frontend API Types (`frontend/src/lib/api.ts`)

New and updated types:

```ts
// Enriched live session (table row)
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
};

// Full session detail (side panel)
export type OcservSessionDetail = {
  id: string;
  username: string;
  remoteAddress: string;
  vpnAddress: string;
  connectedAt?: string;
  durationSeconds: number;
  rxBytes: number;
  txBytes: number;
  userAgent: string;
  device: string;
  ipv4: string;
  ipv6: string;
  routes: string[];
  dns: string[];
  reconnectCount: number;
};
```

New API functions:
```ts
export function getOcservSessionDetail(id: string): Promise<OcservSessionDetail>
export function terminateOcservSession(sessionId: string): Promise<void>
```

---

## Section 5: Data Flow Summary

```
Page mount
  → GET /api/ocserv/sessions → table rows
  → setInterval(30s) → repeat

Row click
  → GET /api/ocserv/sessions/{id} → panel detail

Disconnect click
  → confirm modal
  → POST /api/ocserv/sessions/disconnect {sessionId}
  → success: close panel + remove row + dismissable success banner
  → error: show in modal

Terminate click
  → confirm modal (stronger warning text)
  → POST /api/ocserv/sessions/terminate {sessionId}
  → success: close panel + remove row + dismissable success banner
  → error: show in modal

Auto-refresh fires
  → GET /api/ocserv/sessions
  → if selectedSessionId not in new list → close panel + dismissable banner
  → update "Updated Xs ago" counter
```

---

## Section 6: RBAC Summary

| Action | Permission | Owner | Viewer |
|--------|-----------|-------|--------|
| List sessions | `ocserv:sessions:read` | ✓ | ✓ |
| View session detail | `ocserv:sessions:read` | ✓ | ✓ |
| Disconnect session | `ocserv:sessions:disconnect` | ✓ | ✗ |
| Terminate session | `ocserv:sessions:terminate` | ✓ | ✗ |

---

## Section 7: Audit Events

| Event | Action string | Outcome |
|-------|--------------|---------|
| Disconnect session | `ocserv.sessions.disconnect` | success / failure |
| Terminate session | `ocserv.sessions.terminate` | success / failure |

Both events record `actorID`, `sessionID` (resourceID), and `username` in metadata.

---

## Section 8: Testing Requirements

**Backend:**
- `OcctlAdapter.TerminateSession` — success, not-configured, non-zero exit
- `OcservService.TerminateSession` — RBAC denied, success, failure propagation
- `handleOcservSessionDetail` — 200 with session detail, 404 for missing session
- `handleOcservSessionTerminate` — 204 on success, 400 missing sessionId, 403 for viewer

**Frontend:**
- Confirmation modal renders for disconnect and terminate
- Terminate modal shows cookie-invalidation warning text
- Auto-refresh closes panel when selected session disappears
- RBAC hides Disconnect/Terminate buttons for viewer role

---

## File Ownership

| File | Owner |
|------|-------|
| `backend/internal/ocserv/client.go` | ocserv integration agent |
| `backend/internal/ocserv/occtl_adapter.go` | ocserv integration agent |
| `backend/internal/ocserv/occtl_adapter_test.go` | ocserv integration agent |
| `backend/internal/rbac/rbac.go` | security/RBAC agent |
| `backend/internal/app/ocserv_service.go` | backend API agent |
| `backend/internal/app/ocserv_service_test.go` | backend API agent |
| `backend/internal/server/ocserv_handlers.go` | backend API agent |
| `backend/internal/server/server.go` | backend API agent |
| `frontend/src/routes/sessions/+page.svelte` | frontend admin UX agent |
| `frontend/src/lib/api.ts` | frontend admin UX agent |
| `deploy/sudoers.d/ocserv-panel` | deployment/ops agent |
