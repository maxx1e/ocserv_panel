# Sessions Page Enhancement Design

**Date:** 2026-05-08  
**Status:** Approved  
**Approach:** Phased (Backend → Frontend → Historical Persistence)

---

## Summary

Enhance the existing Sessions page from a basic active-sessions table into a comprehensive VPN session management view. The work is split into three phases to minimize regression risk:

1. **Phase 1 — Backend:** New occtl command parsers, Manager interface extensions, API endpoints
2. **Phase 2 — Frontend:** Tabbed layout, status cards, events/routes/bans tabs, detail drawer enhancements
3. **Phase 3 — Historical Persistence:** SQLite session snapshots for post-disconnect analytics

---

## Phase 1: Backend — New occtl Operations

### Interface Extension

Extend the existing `ocserv.Manager` interface with new methods. All new operations use JSON output from occtl where available, with text parsing fallback for commands that lack JSON support.

```go
type Manager interface {
    // Existing (unchanged)
    ListSessions(ctx context.Context) ([]Session, error)
    DisconnectSession(ctx context.Context, sessionID string) error
    TerminateSession(ctx context.Context, sessionID string) error
    GetSessionDetails(ctx context.Context, id string) (*SessionDetail, error)

    // New — Server Status
    GetServerStatus(ctx context.Context) (*ServerStatus, error)

    // New — User Inspection
    ListConnectedUsers(ctx context.Context) ([]ConnectedUser, error)
    GetUserDetails(ctx context.Context, username string) ([]UserConnection, error)

    // New — Events, Routes, Bans
    ListEvents(ctx context.Context) ([]ConnectionEvent, error)
    ListRoutes(ctx context.Context) ([]UserRoute, error)
    ListBannedIPs(ctx context.Context) ([]BannedIP, error)
    ListBanPoints(ctx context.Context) ([]BanPointEntry, error)

    // New — Actions
    DisconnectUser(ctx context.Context, username string) error
    UnbanIP(ctx context.Context, ip string) error
}
```

### New Domain Types

#### ServerStatus

Parsed from `occtl show status` (text output — this command does not support `--output-type json` reliably).

```go
type ServerStatus struct {
    Status               string        // "online" or "offline"
    ServerPID            int
    SecModPID            int
    SecModInstances      int
    UpSince              time.Time
    ActiveSessions       int
    TotalSessions        int
    TotalAuthFailures    int
    BannedIPs            int
    LastStatsReset       time.Time
    SessionsHandled      int
    TimedOutSessions     int
    IdleTimedOutSessions int
    ErrorClosedSessions  int
    AuthFailures         int
    AvgAuthTime          time.Duration
    MaxAuthTime          time.Duration
    AvgSessionTime       time.Duration
    MaxSessionTime       time.Duration
    MinMTU               int
    MaxMTU               int
    RxBytes              int64
    TxBytes              int64
}
```

#### ConnectedUser

Parsed from `occtl show users` (attempt JSON first, fall back to text parsing).

```go
type ConnectedUser struct {
    ID         string
    Username   string
    Vhost      string
    RemoteIP   string
    VpnIP      string
    Device     string
    Since      string        // raw duration string "1h:27m", "3days"
    Duration   time.Duration // parsed version
    DTLSCipher string
    Status     string        // "connected"
}
```

#### UserConnection

Parsed from `occtl show user <username>` (text output — contains multiple blocks per user).

```go
type UserConnection struct {
    ID                 string
    Username           string
    Groupname          string
    State              string
    Vhost              string
    Device             string
    MTU                int
    RemoteIP           string
    Location           string
    LocalDeviceIP      string
    IPv4               string
    PtPIPv4            string
    UserAgent          string
    RxBytes            int64
    TxBytes            int64
    AvgRxBandwidth     string
    AvgTxBandwidth     string
    DPD                int
    KeepAlive          int
    Hostname           string
    ConnectedAt        time.Time
    SessionID          string
    TLSCipher          string
    DTLSCipher         string
    DNS                []string
    Routes             []string
    RestrictedToRoutes bool
}
```

#### ConnectionEvent

Parsed from `occtl show events` (attempt JSON first).

```go
type ConnectionEvent struct {
    Timestamp time.Time
    Username  string
    RemoteIP  string
    EventType string // "connect", "disconnect", "auth_failure"
    Message   string
}
```

#### UserRoute

Parsed from `occtl show iroutes` (text output).

```go
type UserRoute struct {
    ID       string
    Username string
    Vhost    string
    Device   string
    VpnIP    string
    Route    string
}
```

#### BannedIP and BanPointEntry

Parsed from `occtl show ip bans` and `occtl show ip ban points`.

```go
type BannedIP struct {
    IP     string
    Score  int
    Reason string
}

type BanPointEntry struct {
    IP     string
    Points int
    Since  time.Time
}
```

### Parsing Strategy

1. **JSON-first:** For commands that support `--output-type json`, use JSON parsing.
2. **Text fallback:** For `show status`, `show user <name>`, `show iroutes`, implement text parsers.
3. **Robustness requirements:**
   - Handle variable whitespace and alignment
   - Handle missing/optional fields gracefully (return zero values)
   - Handle truncated cipher strings (occtl truncates long values)
   - Handle multi-line values (DNS servers, routes in user detail)
   - Handle multiple blocks per user in `show user` output
   - Handle empty output (return empty slice, not error)
   - Handle "no such file" / "connection refused" errors → return `ErrNotConfigured`

### Input Validation

- **Username:** Reuse existing `ValidateUsername()` for `DisconnectUser` and `GetUserDetails`
- **IP address:** New `ValidateIPAddress(ip string) error` — must be valid IPv4 or IPv6, no CIDR, no leading dash
- **Session ID:** Reuse existing `validateSessionID()`

### New API Endpoints

| Method | Path | RBAC Permission | Description |
|--------|------|-----------------|-------------|
| GET | `/api/ocserv/server/status` | `ocserv.read` | Server status (from `show status`) |
| GET | `/api/ocserv/users/connected` | `ocserv.sessions.read` | Connected users list |
| GET | `/api/ocserv/users/{username}/connections` | `ocserv.sessions.read` | User's active connections |
| GET | `/api/ocserv/events` | `ocserv.sessions.read` | Connection events |
| GET | `/api/ocserv/routes` | `ocserv.sessions.read` | User-provided routes |
| GET | `/api/ocserv/bans` | `ocserv.sessions.read` | Banned IPs list |
| GET | `/api/ocserv/bans/points` | `ocserv.sessions.read` | IP ban points |
| POST | `/api/ocserv/users/{username}/disconnect` | `ocserv.sessions.disconnect` | Disconnect user (all sessions) |
| POST | `/api/ocserv/bans/{ip}/unban` | `ocserv.bans.manage` | Unban IP address |

### New RBAC Permission

```go
PermissionOcservBanManage = "ocserv.bans.manage"
```

Assigned to `owner` role by default. The `admin` role gets read-only ban visibility via existing `ocserv.sessions.read`.

### Audit Events

| Action | Resource Type | Resource ID | Trigger |
|--------|--------------|-------------|---------|
| `ocserv.user.disconnect` | `ocserv_user` | username | POST disconnect by username |
| `ocserv.ban.unban` | `ocserv_ban` | IP address | POST unban |

### OcservService Extensions

Add methods to `app.OcservService`:

```go
func (s *OcservService) GetServerStatus(ctx, principal) (*ServerStatus, error)
func (s *OcservService) ListConnectedUsers(ctx, principal) ([]ConnectedUser, error)
func (s *OcservService) GetUserConnections(ctx, principal, username) ([]UserConnection, error)
func (s *OcservService) ListEvents(ctx, principal) ([]ConnectionEvent, error)
func (s *OcservService) ListRoutes(ctx, principal) ([]UserRoute, error)
func (s *OcservService) ListBannedIPs(ctx, principal) ([]BannedIP, error)
func (s *OcservService) ListBanPoints(ctx, principal) ([]BanPointEntry, error)
func (s *OcservService) DisconnectUser(ctx, principal, username) error
func (s *OcservService) UnbanIP(ctx, principal, ip) error
```

Each method: RBAC check → delegate to Manager → audit on mutations → return.

### FakeManager Updates

Extend `FakeManager` with configurable return values for all new methods. Pattern: add fields like `ServerStatusResult *ServerStatus`, `ConnectedUsers []ConnectedUser`, etc. with corresponding method implementations.

### Test Strategy

- **Unit tests for parsers:** Test each text/JSON parser with real-world occtl output samples
- **Unit tests for validation:** IP address validation, username validation for disconnect
- **Service-level tests:** RBAC enforcement, audit recording, error propagation
- **Handler tests:** HTTP status codes, JSON marshaling, error responses

---

## Phase 2: Frontend — Tabbed Sessions Page

### Layout Structure

```
┌─────────────────────────────────────────────────────────┐
│ VPN Sessions                    [Updated 5s ago] [↻]    │
├─────────────────────────────────────────────────────────┤
│ ┌──────┐ ┌──────────┐ ┌──────────┐ ┌─────────┐        │
│ │Server│ │  Active  │ │  Auth    │ │ Traffic │        │
│ │Online│ │ Sessions │ │ Failures │ │ RX / TX │        │
│ │ 3d   │ │    2     │ │    6     │ │91MB/2.6G│        │
│ └──────┘ └──────────┘ └──────────┘ └─────────┘        │
├─────────────────────────────────────────────────────────┤
│ [Active Sessions] [All Sessions] [Events] [Routes] [Bans]│
├─────────────────────────────────────────────────────────┤
│                                                         │
│  (Tab content area — existing table or new sections)    │
│                                                         │
└─────────────────────────────────────────────────────────┘
```

### Status Summary Cards

Four compact cards powered by `GET /api/ocserv/server/status`:

1. **Server** — online/offline badge, uptime
2. **Active Sessions** — count, total handled
3. **Authentication** — failure count, avg auth time
4. **Traffic** — total RX / TX

Include a small notice: "Statistics update on user disconnect."

### Tab Structure

| Tab | Data Source | Content |
|-----|-------------|---------|
| Active Sessions | Existing `/api/ocserv/sessions` | Current table + drawer (unchanged) |
| All Sessions | `/api/ocserv/sessions` (shows all including authenticated-but-not-connected) | Session inventory with status badges (connected/authenticated/reconnectable) |
| Events | `/api/ocserv/events` | Connection events table |
| Routes | `/api/ocserv/routes` | User-provided routes table |
| Bans | `/api/ocserv/bans` + `/api/ocserv/bans/points` | Banned IPs + points with unban action |

### New Components

| Component | Purpose |
|-----------|---------|
| `SessionStatusCards.svelte` | Server status summary cards |
| `SessionTabs.svelte` | Horizontal tab navigation |
| `EventsTab.svelte` | Events table with filtering |
| `RoutesTab.svelte` | Routes table with empty state |
| `BansTab.svelte` | Ban list with unban button |
| `UnbanConfirmDialog.svelte` | Unban confirmation modal |
| `DisconnectUserDialog.svelte` | Disconnect-user confirmation modal |

### Detail Drawer Enhancements

Add to existing drawer:
- **Security** section: TLS ciphersuite, DTLS cipher, certificate auth status
- **Network Policy** section: MTU, DPD, KeepAlive, restricted-to-routes
- **Disconnect User** button (disconnects all sessions for that username)

These require data from `GET /api/ocserv/users/{username}/connections`.

### Frontend API Additions

```typescript
export type ServerStatus = {
  status: string;
  serverPid: number;
  secModPid: number;
  upSince: string;
  activeSessions: number;
  totalSessions: number;
  totalAuthFailures: number;
  bannedIPs: number;
  sessionsHandled: number;
  timedOutSessions: number;
  idleTimedOutSessions: number;
  errorClosedSessions: number;
  authFailures: number;
  avgAuthTimeSecs: number;
  maxAuthTimeSecs: number;
  avgSessionTimeSecs: number;
  maxSessionTimeSecs: number;
  minMtu: number;
  maxMtu: number;
  rxBytes: number;
  txBytes: number;
};

export type ConnectionEvent = {
  timestamp: string;
  username: string;
  remoteIp: string;
  eventType: string;
  message: string;
};

export type UserRoute = {
  id: string;
  username: string;
  vhost: string;
  device: string;
  vpnIp: string;
  route: string;
};

export type BannedIP = {
  ip: string;
  score: number;
  reason: string;
};

export type BanPointEntry = {
  ip: string;
  points: number;
  since: string;
};

export function getServerStatus(): Promise<ServerStatus>;
export function getConnectionEvents(): Promise<ConnectionEvent[]>;
export function getUserRoutes(): Promise<UserRoute[]>;
export function getBannedIPs(): Promise<BannedIP[]>;
export function getBanPoints(): Promise<BanPointEntry[]>;
export function disconnectUser(username: string): Promise<void>;
export function unbanIP(ip: string): Promise<void>;
```

### UX Requirements

- Auto-refresh continues (30s default) but only refreshes the active tab's data
- Manual refresh refreshes status cards + active tab
- Disconnect-user and Unban require explicit confirmation dialogs
- Tabs preserve their state when switching (don't re-fetch unless stale)
- Empty states for each tab (no events, no routes, no bans)
- Loading skeletons for initial data fetch
- Error alerts scoped to each section

---

## Phase 3: Historical Persistence (Future Phase)

### SQLite Schema

```sql
CREATE TABLE session_history (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    session_id TEXT NOT NULL,
    username TEXT NOT NULL,
    remote_ip TEXT NOT NULL,
    vpn_ip TEXT,
    hostname TEXT,
    user_agent TEXT,
    device TEXT,
    connected_at DATETIME NOT NULL,
    disconnected_at DATETIME,
    duration_seconds INTEGER,
    rx_bytes INTEGER DEFAULT 0,
    tx_bytes INTEGER DEFAULT 0,
    disconnect_reason TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_session_history_username ON session_history(username);
CREATE INDEX idx_session_history_connected_at ON session_history(connected_at);
CREATE INDEX idx_session_history_remote_ip ON session_history(remote_ip);
```

### Collection Strategy

- Compare current session list to previous poll; sessions that disappeared are considered disconnected
- Store final RX/TX values from the last seen poll
- Do NOT store sensitive data (passwords, private keys, ciphers) in history
- Retention: configurable, default 90 days

### Historical Query API

```
GET /api/ocserv/sessions/history?from=<ISO>&to=<ISO>&username=<optional>&limit=100
```

---

## Security Considerations

1. **Command injection:** All user-provided values (usernames, IPs, session IDs) are validated before being passed to exec.Command as separate args (never concatenated into a shell string)
2. **IP validation:** Strict IPv4/IPv6 format check, reject CIDR notation, reject empty/blank
3. **Username validation:** Reuse existing `ValidateUsername` which rejects special characters
4. **RBAC:** New `ocserv.bans.manage` permission required for unban; existing permissions cover all other operations
5. **Audit trail:** All mutating operations (disconnect user, unban IP) are audit-logged with actor, target, outcome
6. **No server control:** The Sessions page MUST NOT expose stop/reload/restart ocserv actions — those belong to the service management area

---

## Out of Scope

- Disconnect by numeric connection ID (use session ID which maps to it)
- Geographic IP lookup
- Rate limiting on admin actions (covered by general API rate limiting)
- WebSocket-based real-time updates (polling is sufficient for admin use)
- Multi-node session correlation

---

## File Impact Summary

### Phase 1 (Backend)

| File | Change |
|------|--------|
| `backend/internal/ocserv/client.go` | Extend Manager interface, add new types, extend FakeManager |
| `backend/internal/ocserv/occtl_adapter.go` | Add new method implementations |
| `backend/internal/ocserv/occtl_status_parser.go` | New file: text parser for `show status` |
| `backend/internal/ocserv/occtl_user_parser.go` | New file: text parser for `show user` output |
| `backend/internal/ocserv/occtl_routes_parser.go` | New file: text parser for `show iroutes` |
| `backend/internal/ocserv/occtl_adapter_test.go` | Tests for new adapter methods |
| `backend/internal/ocserv/occtl_status_parser_test.go` | Parser tests with sample output |
| `backend/internal/ocserv/occtl_user_parser_test.go` | Parser tests |
| `backend/internal/ocserv/occtl_routes_parser_test.go` | Parser tests |
| `backend/internal/ocserv/validate.go` | Add `ValidateIPAddress` |
| `backend/internal/ocserv/validate_test.go` | IP validation tests |
| `backend/internal/app/ocserv_service.go` | New service methods |
| `backend/internal/app/ocserv_service_test.go` | Service tests |
| `backend/internal/server/ocserv_handlers.go` | New HTTP handlers |
| `backend/internal/server/server.go` | Register new routes |
| `backend/internal/rbac/rbac.go` | Add `PermissionOcservBanManage` |

### Phase 2 (Frontend)

| File | Change |
|------|--------|
| `frontend/src/lib/api.ts` | New types and API functions |
| `frontend/src/routes/sessions/+page.svelte` | Add tabs wrapper and status cards |
| `frontend/src/routes/sessions/SessionStatusCards.svelte` | New component |
| `frontend/src/routes/sessions/SessionTabs.svelte` | New component |
| `frontend/src/routes/sessions/EventsTab.svelte` | New component |
| `frontend/src/routes/sessions/RoutesTab.svelte` | New component |
| `frontend/src/routes/sessions/BansTab.svelte` | New component |
| `frontend/src/routes/sessions/UnbanConfirmDialog.svelte` | New component |
| `frontend/src/routes/sessions/DisconnectUserDialog.svelte` | New component |

### Phase 3 (Persistence)

| File | Change |
|------|--------|
| `migrations/NNN_session_history.sql` | New migration |
| `backend/internal/persistence/session_history_repo.go` | New repository |
| `backend/internal/app/session_history_service.go` | New service |
| `backend/internal/server/session_history_handlers.go` | New handlers |
