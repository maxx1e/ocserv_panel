# Sessions Page Enhancement — Frontend Design (Phase 2)

**Date:** 2026-05-08  
**Status:** Approved  
**Phase:** 2 of 2 — Frontend Tabbed UI  
**Depends on:** Phase 1 (Backend) completion

---

## Summary

Enhance the existing Sessions page with server status cards, a tabbed layout (Active Sessions, All Sessions, Events, Routes, Bans), detail drawer enhancements (security + network policy sections), and new admin action dialogs (disconnect-user, unban-IP).

---

## Layout Structure

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

---

## Status Summary Cards

Four compact cards powered by `GET /api/ocserv/server/status`:

1. **Server** — online/offline badge, uptime duration
2. **Active Sessions** — count, total handled in stats period
3. **Authentication** — failure count, average auth time
4. **Traffic** — total RX / TX in stats period

Include a small notice: "Statistics update on user disconnect."

---

## Tab Structure

| Tab | Data Source | Content |
|-----|-------------|---------|
| Active Sessions | Existing `GET /api/ocserv/sessions` | Current table + drawer (unchanged) |
| All Sessions | `GET /api/ocserv/sessions` (shows all including authenticated-but-not-connected) | Session inventory with status badges (connected/authenticated/reconnectable) |
| Events | `GET /api/ocserv/events` | Connection events table |
| Routes | `GET /api/ocserv/routes` | User-provided routes table |
| Bans | `GET /api/ocserv/bans` + `GET /api/ocserv/bans/points` | Banned IPs + points with unban action |

---

## New Components

| Component | Purpose |
|-----------|---------|
| `SessionStatusCards.svelte` | Server status summary cards |
| `SessionTabs.svelte` | Horizontal tab navigation |
| `EventsTab.svelte` | Events table with filtering |
| `RoutesTab.svelte` | Routes table with empty state |
| `BansTab.svelte` | Ban list with unban button |
| `UnbanConfirmDialog.svelte` | Unban confirmation modal |
| `DisconnectUserDialog.svelte` | Disconnect-user confirmation modal |

---

## Detail Drawer Enhancements

Add to existing session detail drawer:

### Security Section
- TLS ciphersuite (full string, word-wrap for long values)
- DTLS cipher
- Certificate authentication status (yes/no badge)

### Network Policy Section
- MTU value
- DPD timeout
- KeepAlive interval
- Routes assigned
- Restricted-to-routes status (yes/no)

### Disconnect User Button
- Disconnects ALL sessions for that username (not just the selected one)
- Opens `DisconnectUserDialog` for confirmation
- Shows warning that all sessions for the user will be dropped

These require data from `GET /api/ocserv/users/{username}/connections`.

---

## Frontend API Additions

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

export type ConnectedUser = {
  id: string;
  username: string;
  vhost: string;
  remoteIp: string;
  vpnIp: string;
  device: string;
  since: string;
  durationSeconds: number;
  dtlsCipher: string;
  status: string;
};

export type UserConnection = {
  id: string;
  username: string;
  groupname: string;
  state: string;
  vhost: string;
  device: string;
  mtu: number;
  remoteIp: string;
  location: string;
  localDeviceIp: string;
  ipv4: string;
  ptpIpv4: string;
  userAgent: string;
  rxBytes: number;
  txBytes: number;
  avgRxBandwidth: string;
  avgTxBandwidth: string;
  dpd: number;
  keepAlive: number;
  hostname: string;
  connectedAt: string;
  sessionId: string;
  tlsCipher: string;
  dtlsCipher: string;
  dns: string[];
  routes: string[];
  restrictedToRoutes: boolean;
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
```

### API Functions

```typescript
export function getServerStatus(): Promise<ServerStatus>;
export function getConnectedUsers(): Promise<ConnectedUser[]>;
export function getUserConnections(username: string): Promise<UserConnection[]>;
export function getConnectionEvents(): Promise<ConnectionEvent[]>;
export function getUserRoutes(): Promise<UserRoute[]>;
export function getBannedIPs(): Promise<BannedIP[]>;
export function getBanPoints(): Promise<BanPointEntry[]>;
export function disconnectUser(username: string): Promise<void>;
export function unbanIP(ip: string): Promise<void>;
```

---

## UX Requirements

### Refresh Behavior
- Auto-refresh continues (30s default) but only refreshes the active tab's data
- Manual refresh refreshes status cards + active tab
- Status cards refresh alongside whichever tab is active
- Pause refresh while confirmation dialogs are open

### Tab Behavior
- Tabs preserve their state when switching (don't re-fetch unless stale > 30s)
- URL hash tracks active tab (`#events`, `#routes`, `#bans`) for bookmarkability
- Tab badge counts where applicable (e.g., Bans tab shows count if > 0)

### Confirmation Dialogs
- Disconnect-user requires explicit confirmation, shows username and "all sessions" warning
- Unban-IP requires explicit confirmation, shows IP address
- Both show success/error banners after completion

### Loading & Error States
- Loading skeletons for initial data fetch per tab
- Error alerts scoped to each section (don't break other tabs)
- Empty states for each tab (no events, no routes, no bans)

### Responsiveness
- Desktop: full layout with side drawer
- Tablet: drawer overlays content
- Tables support horizontal scrolling for long cipher/UA values
- Long text values (ciphers, user agents, full session IDs) truncate with copy-to-clipboard

---

## Safe Admin Actions

### Allowed on Sessions page:
- Refresh data
- View session/user details
- Copy values (IP, session ID, cipher)
- Disconnect by connection ID (existing)
- Terminate by connection ID (existing)
- Disconnect by username (new — drops all sessions for user)
- Unban IP (new)

### Require explicit confirmation:
- Disconnect by ID
- Terminate by ID
- Disconnect by username
- Unban IP

### NOT exposed on this page (belong elsewhere):
- Stop ocserv server
- Reload ocserv configuration
- Restart systemd service
- Edit ocserv configuration
- Modify firewall rules

---

## File Impact

| File | Change |
|------|--------|
| `frontend/src/lib/api.ts` | New types and API functions |
| `frontend/src/routes/sessions/+page.svelte` | Add tabs wrapper and status cards, restructure |
| `frontend/src/routes/sessions/SessionStatusCards.svelte` | New component |
| `frontend/src/routes/sessions/SessionTabs.svelte` | New component |
| `frontend/src/routes/sessions/ActiveSessionsTab.svelte` | Extract from current +page.svelte |
| `frontend/src/routes/sessions/AllSessionsTab.svelte` | New component |
| `frontend/src/routes/sessions/EventsTab.svelte` | New component |
| `frontend/src/routes/sessions/RoutesTab.svelte` | New component |
| `frontend/src/routes/sessions/BansTab.svelte` | New component |
| `frontend/src/routes/sessions/UnbanConfirmDialog.svelte` | New component |
| `frontend/src/routes/sessions/DisconnectUserDialog.svelte` | New component |

---

## Future Phase: Historical Persistence (Phase 3 — not in scope)

### SQLite Schema (for future reference)

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
- Compare current session list to previous poll; disappeared sessions → disconnected
- Store final RX/TX values from the last seen poll
- Retention: configurable, default 90 days

### Historical Query API
```
GET /api/ocserv/sessions/history?from=<ISO>&to=<ISO>&username=<optional>&limit=100
```

This phase will be designed and planned separately after Phase 1 and 2 are complete.
