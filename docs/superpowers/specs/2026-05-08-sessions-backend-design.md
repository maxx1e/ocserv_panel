# Sessions Page Enhancement — Backend Design (Phase 1)

**Date:** 2026-05-08  
**Status:** Approved  
**Phase:** 1 of 2 — Backend occtl Operations

---

## Summary

Extend the backend `ocserv.Manager` interface with new occtl command parsers and API endpoints for server status, user inspection, events, routes, bans, disconnect-by-username, and unban-IP.

---

## Interface Extension

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

---

## New Domain Types

### ServerStatus

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

### ConnectedUser

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

### UserConnection

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

### ConnectionEvent

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

### UserRoute

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

### BannedIP and BanPointEntry

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

---

## Parsing Strategy

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

---

## Input Validation

- **Username:** Reuse existing `ValidateUsername()` for `DisconnectUser` and `GetUserDetails`
- **IP address:** New `ValidateIPAddress(ip string) error` — must be valid IPv4 or IPv6, no CIDR, no leading dash
- **Session ID:** Reuse existing `validateSessionID()`

---

## New API Endpoints

| Method | Path | RBAC Permission | Description |
|--------|------|-----------------|-------------|
| GET | `/api/ocserv/server/status` | `ocserv:read` | Server status (from `show status`) |
| GET | `/api/ocserv/users/connected` | `ocserv:sessions:read` | Connected users list |
| GET | `/api/ocserv/users/{username}/connections` | `ocserv:sessions:read` | User's active connections |
| GET | `/api/ocserv/events` | `ocserv:sessions:read` | Connection events |
| GET | `/api/ocserv/routes` | `ocserv:sessions:read` | User-provided routes |
| GET | `/api/ocserv/bans` | `ocserv:sessions:read` | Banned IPs list |
| GET | `/api/ocserv/bans/points` | `ocserv:sessions:read` | IP ban points |
| POST | `/api/ocserv/users/{username}/disconnect` | `ocserv:sessions:disconnect` | Disconnect user (all sessions) |
| POST | `/api/ocserv/bans/{ip}/unban` | `ocserv:bans:manage` | Unban IP address |

---

## New RBAC Permission

```go
PermissionOcservBanManage = "ocserv:bans:manage"
```

Assigned to `owner` role by default. The `admin` role gets read-only ban visibility via existing `ocserv:sessions:read`.

---

## Audit Events

| Action | Resource Type | Resource ID | Trigger |
|--------|--------------|-------------|---------|
| `ocserv.user.disconnect` | `ocserv_user` | username | POST disconnect by username |
| `ocserv.ban.unban` | `ocserv_ban` | IP address | POST unban |

---

## OcservService Extensions

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

---

## FakeManager Updates

Extend `FakeManager` with configurable return values for all new methods. Pattern: add fields like `ServerStatusResult *ServerStatus`, `ConnectedUsers []ConnectedUser`, etc. with corresponding method implementations.

---

## Test Strategy

- **Unit tests for parsers:** Test each text/JSON parser with real-world occtl output samples
- **Unit tests for validation:** IP address validation, username validation for disconnect
- **Service-level tests:** RBAC enforcement, audit recording, error propagation
- **Handler tests:** HTTP status codes, JSON marshaling, error responses

---

## Security Considerations

1. **Command injection:** All user-provided values (usernames, IPs, session IDs) are validated before being passed to exec.Command as separate args (never concatenated into a shell string)
2. **IP validation:** Strict IPv4/IPv6 format check, reject CIDR notation, reject empty/blank
3. **Username validation:** Reuse existing `ValidateUsername` which rejects special characters
4. **RBAC:** New `ocserv:bans:manage` permission required for unban; existing permissions cover all other operations
5. **Audit trail:** All mutating operations (disconnect user, unban IP) are audit-logged with actor, target, outcome
6. **No server control:** The Sessions page MUST NOT expose stop/reload/restart ocserv actions

---

## File Impact

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
