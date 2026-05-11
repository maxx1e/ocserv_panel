# Sessions Page Frontend Enhancement — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Enhance the Sessions page with server status cards, a five-tab layout (Active Sessions, All Sessions, Events, Routes, Bans), enhanced detail drawer with security/network sections, and new admin action dialogs (disconnect-user, unban-IP).

**Architecture:** The frontend-only plan restructures `+page.svelte` into a tab-based layout with extracted tab components. Each tab fetches its own data independently. State is managed via Svelte 5 runes ($state/$derived). URL hash tracks the active tab. Status cards use the existing `CardStats` component pattern.

**Tech Stack:** SvelteKit, Svelte 5 runes, Tailwind CSS v4, lucide-svelte icons, existing `$lib/api.ts` pattern

**Depends on:** All backend endpoints already exist:
- `GET /api/ocserv/server/status`
- `GET /api/ocserv/users/connected`
- `GET /api/ocserv/users/{username}/connections` (via `handleOcservUserConnections`)
- `GET /api/ocserv/events`
- `GET /api/ocserv/routes`
- `GET /api/ocserv/bans`
- `GET /api/ocserv/bans/points`
- `POST /api/ocserv/users/{username}/disconnect`
- `POST /api/ocserv/bans/{ip}/unban`

---

## File Structure

| File | Role |
|------|------|
| `frontend/src/lib/api.ts` | Add new types + API functions for server status, connected users, user connections, events, routes, bans, disconnect-user, unban-IP |
| `frontend/src/routes/sessions/+page.svelte` | Orchestrator: status cards + tab switcher + tab rendering; delegates to tab components |
| `frontend/src/routes/sessions/SessionStatusCards.svelte` | Server status summary cards (4 cards using `CardStats`) |
| `frontend/src/routes/sessions/SessionTabs.svelte` | Horizontal tab navigation with badge counts and URL hash sync |
| `frontend/src/routes/sessions/ActiveSessionsTab.svelte` | Extract existing active sessions table + drawer from +page.svelte |
| `frontend/src/routes/sessions/AllSessionsTab.svelte` | All sessions (connected + authenticated) table with status badges |
| `frontend/src/routes/sessions/EventsTab.svelte` | Connection events table |
| `frontend/src/routes/sessions/RoutesTab.svelte` | User-provided routes table |
| `frontend/src/routes/sessions/BansTab.svelte` | Banned IPs + ban points + unban action |
| `frontend/src/routes/sessions/UnbanConfirmDialog.svelte` | Confirmation dialog for unbanning an IP |
| `frontend/src/routes/sessions/DisconnectUserDialog.svelte` | Confirmation dialog for disconnecting all user sessions |
| `frontend/src/routes/sessions/SessionDetailDrawer.svelte` | Extracted side panel with security + network policy sections added |

---

## Task 1: Add Frontend API Types and Functions

**Files:**
- Modify: `frontend/src/lib/api.ts`

- [ ] **Step 1: Add new types to api.ts**

Append after the existing `OcservStatus` type block (around line 403):

```typescript
// ─── Sessions Page Types (Phase 2) ─────────────────────────────────────────

export type ServerStatus = {
  status: string;
  serverPid: number;
  secModPid: number;
  secModInstances: number;
  upSince: string;
  activeSessions: number;
  totalSessions: number;
  totalAuthFailures: number;
  bannedIps: number;
  lastStatsReset: string;
  sessionsHandled: number;
  timedOutSessions: number;
  idleTimedOutSessions: number;
  errorClosedSessions: number;
  authFailures: number;
  avgAuthTimeSeconds: number;
  maxAuthTimeSeconds: number;
  avgSessionTimeSeconds: number;
  maxSessionTimeSeconds: number;
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

- [ ] **Step 2: Add API functions**

Append after the existing `terminateOcservSession` function:

```typescript
export function getServerStatus(): Promise<ServerStatus> {
  return apiFetch<ServerStatus>('/api/ocserv/server/status');
}

export function getConnectedUsers(): Promise<ConnectedUser[]> {
  return apiFetch<ConnectedUser[]>('/api/ocserv/users/connected');
}

export function getUserConnections(username: string): Promise<UserConnection[]> {
  return apiFetch<UserConnection[]>(`/api/ocserv/users/${encodeURIComponent(username)}/connections`);
}

export function getConnectionEvents(): Promise<ConnectionEvent[]> {
  return apiFetch<ConnectionEvent[]>('/api/ocserv/events');
}

export function getUserRoutes(): Promise<UserRoute[]> {
  return apiFetch<UserRoute[]>('/api/ocserv/routes');
}

export function getBannedIPs(): Promise<BannedIP[]> {
  return apiFetch<BannedIP[]>('/api/ocserv/bans');
}

export function getBanPoints(): Promise<BanPointEntry[]> {
  return apiFetch<BanPointEntry[]>('/api/ocserv/bans/points');
}

export function disconnectUser(username: string): Promise<void> {
  return apiFetch<void>(`/api/ocserv/users/${encodeURIComponent(username)}/disconnect`, {
    method: 'POST'
  });
}

export function unbanIP(ip: string): Promise<void> {
  return apiFetch<void>(`/api/ocserv/bans/${encodeURIComponent(ip)}/unban`, {
    method: 'POST'
  });
}
```

- [ ] **Step 3: Verify TypeScript compiles**

Run: `cd /opt/ocserv_panel/frontend && npx svelte-check --threshold error 2>&1 | tail -5`
Expected: No errors related to api.ts

- [ ] **Step 4: Commit**

```bash
git add frontend/src/lib/api.ts
git commit -m "feat(sessions): add API types and functions for sessions page phase 2"
```

---

## Task 2: Create SessionTabs Component

**Files:**
- Create: `frontend/src/routes/sessions/SessionTabs.svelte`

- [ ] **Step 1: Create SessionTabs.svelte**

```svelte
<script lang="ts">
  type TabDef = {
    id: string;
    label: string;
    badge?: number;
  };

  type Props = {
    tabs: TabDef[];
    activeTab: string;
    onTabChange: (tabId: string) => void;
  };

  let { tabs, activeTab, onTabChange }: Props = $props();
</script>

<div class="border-b border-slate-200" role="tablist" aria-label="Sessions navigation">
  <nav class="flex gap-0 -mb-px overflow-x-auto">
    {#each tabs as tab (tab.id)}
      <button
        type="button"
        role="tab"
        id="tab-{tab.id}"
        aria-selected={activeTab === tab.id}
        aria-controls="tabpanel-{tab.id}"
        class="relative px-4 py-3 text-sm font-medium whitespace-nowrap transition-colors
          {activeTab === tab.id
            ? 'text-blue-600 border-b-2 border-blue-600'
            : 'text-slate-500 hover:text-slate-700 hover:border-b-2 hover:border-slate-300'}"
        onclick={() => onTabChange(tab.id)}
      >
        {tab.label}
        {#if tab.badge && tab.badge > 0}
          <span class="ml-1.5 inline-flex items-center justify-center px-1.5 py-0.5 text-xs font-medium rounded-full bg-red-100 text-red-700 min-w-[1.25rem]">
            {tab.badge}
          </span>
        {/if}
      </button>
    {/each}
  </nav>
</div>
```

- [ ] **Step 2: Commit**

```bash
git add frontend/src/routes/sessions/SessionTabs.svelte
git commit -m "feat(sessions): add SessionTabs horizontal tab navigation component"
```

---

## Task 3: Create SessionStatusCards Component

**Files:**
- Create: `frontend/src/routes/sessions/SessionStatusCards.svelte`

- [ ] **Step 1: Create SessionStatusCards.svelte**

```svelte
<script lang="ts">
  import CardStats from '$lib/components/ui/CardStats.svelte';
  import { Server, Users, ShieldAlert, ArrowUpDown } from 'lucide-svelte';
  import { formatBytes } from '$lib/monitoring';
  import type { ServerStatus } from '$lib/api';

  type Props = {
    status: ServerStatus | null;
    loading: boolean;
    error: string;
  };

  let { status, loading, error }: Props = $props();

  function formatUptime(upSince: string | undefined): string {
    if (!upSince) return '—';
    const date = new Date(upSince);
    if (isNaN(date.getTime())) return '—';
    const ms = Date.now() - date.getTime();
    if (ms < 0) return '—';
    const totalMin = Math.floor(ms / 60000);
    const totalHours = Math.floor(totalMin / 60);
    const totalDays = Math.floor(totalHours / 24);
    if (totalDays > 0) return `${totalDays}d ${totalHours % 24}h`;
    if (totalHours > 0) return `${totalHours}h ${totalMin % 60}m`;
    return `${totalMin}m`;
  }

  const isOnline = $derived(status?.status === 'online');
  const uptimeStr = $derived(formatUptime(status?.upSince));
  const trafficStr = $derived(
    status ? `${formatBytes(status.rxBytes)} / ${formatBytes(status.txBytes)}` : '—'
  );
</script>

{#if error}
  <div class="px-4 py-2 rounded-lg text-sm bg-amber-50 text-amber-700 border border-amber-200" role="alert">
    Server status unavailable: {error}
  </div>
{:else}
  <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
    <CardStats
      title="Server"
      value={loading ? '...' : (isOnline ? 'Online' : 'Offline')}
      subtitle={isOnline ? `Uptime: ${uptimeStr}` : undefined}
      icon={Server}
      statusValue={isOnline ? 'online' : 'offline'}
    />
    <CardStats
      title="Active Sessions"
      value={loading ? '...' : String(status?.activeSessions ?? 0)}
      subtitle={status ? `${status.sessionsHandled} handled this period` : undefined}
      icon={Users}
    />
    <CardStats
      title="Auth Failures"
      value={loading ? '...' : String(status?.authFailures ?? 0)}
      subtitle={status?.avgAuthTimeSeconds ? `Avg auth: ${status.avgAuthTimeSeconds}s` : undefined}
      icon={ShieldAlert}
    />
    <CardStats
      title="Traffic (Period)"
      value={loading ? '...' : trafficStr}
      subtitle="RX / TX — updates on disconnect"
      icon={ArrowUpDown}
    />
  </div>
{/if}
```

- [ ] **Step 2: Commit**

```bash
git add frontend/src/routes/sessions/SessionStatusCards.svelte
git commit -m "feat(sessions): add SessionStatusCards component with server stats"
```

---

## Task 4: Create DisconnectUserDialog Component

**Files:**
- Create: `frontend/src/routes/sessions/DisconnectUserDialog.svelte`

- [ ] **Step 1: Create DisconnectUserDialog.svelte**

```svelte
<script lang="ts">
  import { AlertTriangle } from 'lucide-svelte';

  type Props = {
    username: string;
    submitting: boolean;
    error: string;
    onConfirm: () => void;
    onCancel: () => void;
  };

  let { username, submitting, error, onConfirm, onCancel }: Props = $props();
</script>

<div
  class="fixed inset-0 bg-black/50 flex items-center justify-center z-50"
  role="dialog"
  aria-modal="true"
  aria-labelledby="disconnect-user-title"
>
  <div class="bg-white rounded-lg shadow-xl max-w-md w-full mx-4 p-6">
    <div class="flex items-start gap-3">
      <div class="flex-shrink-0 w-10 h-10 rounded-full bg-orange-100 flex items-center justify-center">
        <AlertTriangle class="w-5 h-5 text-orange-600" />
      </div>
      <div class="flex-1">
        <h2 id="disconnect-user-title" class="text-lg font-semibold text-slate-800">
          Disconnect user?
        </h2>
        <p class="text-sm text-slate-600 mt-2">
          This will drop <strong>all active sessions</strong> for <strong>{username}</strong>. The user will need to reconnect.
        </p>
      </div>
    </div>

    {#if error}
      <div class="mt-3 px-3 py-2 bg-red-50 rounded text-sm text-red-700" role="alert">
        {error}
      </div>
    {/if}

    <div class="flex items-center justify-end gap-3 mt-6">
      <button
        type="button"
        onclick={onCancel}
        disabled={submitting}
        class="px-4 py-2 text-sm font-medium rounded-md border border-slate-200 text-slate-600 bg-white hover:bg-slate-50 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
      >
        Cancel
      </button>
      <button
        type="button"
        onclick={onConfirm}
        disabled={submitting}
        class="px-4 py-2 text-sm font-medium rounded-md bg-orange-600 text-white hover:bg-orange-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
      >
        {submitting ? 'Disconnecting...' : 'Disconnect All Sessions'}
      </button>
    </div>
  </div>
</div>
```

- [ ] **Step 2: Commit**

```bash
git add frontend/src/routes/sessions/DisconnectUserDialog.svelte
git commit -m "feat(sessions): add DisconnectUserDialog confirmation modal"
```

---

## Task 5: Create UnbanConfirmDialog Component

**Files:**
- Create: `frontend/src/routes/sessions/UnbanConfirmDialog.svelte`

- [ ] **Step 1: Create UnbanConfirmDialog.svelte**

```svelte
<script lang="ts">
  import { ShieldOff } from 'lucide-svelte';

  type Props = {
    ip: string;
    submitting: boolean;
    error: string;
    onConfirm: () => void;
    onCancel: () => void;
  };

  let { ip, submitting, error, onConfirm, onCancel }: Props = $props();
</script>

<div
  class="fixed inset-0 bg-black/50 flex items-center justify-center z-50"
  role="dialog"
  aria-modal="true"
  aria-labelledby="unban-ip-title"
>
  <div class="bg-white rounded-lg shadow-xl max-w-md w-full mx-4 p-6">
    <div class="flex items-start gap-3">
      <div class="flex-shrink-0 w-10 h-10 rounded-full bg-blue-100 flex items-center justify-center">
        <ShieldOff class="w-5 h-5 text-blue-600" />
      </div>
      <div class="flex-1">
        <h2 id="unban-ip-title" class="text-lg font-semibold text-slate-800">
          Unban IP address?
        </h2>
        <p class="text-sm text-slate-600 mt-2">
          Remove <strong class="font-mono">{ip}</strong> from the ban list. The IP will be allowed to attempt connections again.
        </p>
      </div>
    </div>

    {#if error}
      <div class="mt-3 px-3 py-2 bg-red-50 rounded text-sm text-red-700" role="alert">
        {error}
      </div>
    {/if}

    <div class="flex items-center justify-end gap-3 mt-6">
      <button
        type="button"
        onclick={onCancel}
        disabled={submitting}
        class="px-4 py-2 text-sm font-medium rounded-md border border-slate-200 text-slate-600 bg-white hover:bg-slate-50 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
      >
        Cancel
      </button>
      <button
        type="button"
        onclick={onConfirm}
        disabled={submitting}
        class="px-4 py-2 text-sm font-medium rounded-md bg-blue-600 text-white hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
      >
        {submitting ? 'Unbanning...' : 'Unban IP'}
      </button>
    </div>
  </div>
</div>
```

- [ ] **Step 2: Commit**

```bash
git add frontend/src/routes/sessions/UnbanConfirmDialog.svelte
git commit -m "feat(sessions): add UnbanConfirmDialog confirmation modal"
```

---

## Task 6: Create SessionDetailDrawer Component

**Files:**
- Create: `frontend/src/routes/sessions/SessionDetailDrawer.svelte`

This extracts the existing detail side panel from `+page.svelte` and adds new Security and Network Policy sections using `UserConnection` data.

- [ ] **Step 1: Create SessionDetailDrawer.svelte**

```svelte
<script lang="ts">
  import { X, Copy, Shield, Network } from 'lucide-svelte';
  import { formatAbsoluteTime } from '$lib/utils/format';
  import { formatBytes } from '$lib/monitoring';
  import type { OcservSession, OcservSessionDetail, UserConnection } from '$lib/api';

  type Props = {
    session: OcservSession | undefined;
    detail: OcservSessionDetail | null;
    userConnection: UserConnection | null;
    loading: boolean;
    error: string;
    sessionGone: boolean;
    isOwner: boolean;
    onClose: () => void;
    onDisconnectSession: (session: OcservSession) => void;
    onTerminateSession: (session: OcservSession) => void;
    onDisconnectUser: (username: string) => void;
  };

  let {
    session,
    detail,
    userConnection,
    loading,
    error,
    sessionGone,
    isOwner,
    onClose,
    onDisconnectSession,
    onTerminateSession,
    onDisconnectUser
  }: Props = $props();

  function formatDurationSecs(secs: number): string {
    if (!secs || secs < 0) return '—';
    const h = Math.floor(secs / 3600);
    const m = Math.floor((secs % 3600) / 60);
    if (h > 0) return `${h}h ${m}m`;
    return `${m}m`;
  }

  function copyToClipboard(text: string) {
    navigator.clipboard.writeText(text);
  }

  const username = $derived(session?.username ?? 'Session');
  const sessionIdShort = $derived(session?.id?.slice(0, 8) ?? '');
</script>

<div class="w-96 shrink-0 bg-white rounded-lg shadow-sm border border-slate-100 overflow-hidden">
  <!-- Panel header -->
  <div class="flex items-center justify-between px-4 py-3 border-b border-slate-100 bg-slate-50">
    <div class="min-w-0">
      <h2 class="text-sm font-semibold text-slate-800 truncate">
        {username} — <span class="font-mono font-normal text-slate-500">{sessionIdShort}</span>
      </h2>
    </div>
    <button
      type="button"
      onclick={onClose}
      class="ml-2 text-slate-400 hover:text-slate-600 transition-colors flex-shrink-0"
      aria-label="Close panel"
    >
      <X class="w-4 h-4" />
    </button>
  </div>

  <!-- Panel body -->
  <div class="px-4 py-3 space-y-1 overflow-y-auto max-h-[calc(100vh-16rem)]">
    {#if loading}
      <div class="py-8 flex items-center justify-center">
        <div class="w-6 h-6 border-2 border-slate-200 border-t-slate-500 rounded-full animate-spin"></div>
      </div>

    {:else if sessionGone}
      <p class="text-sm text-slate-500 py-6 text-center">Session no longer active.</p>

    {:else if error}
      <p class="text-sm text-red-600 py-4">{error}</p>

    {:else if detail}
      <!-- Connection Info -->
      <p class="text-xs font-semibold text-slate-500 uppercase tracking-wider mt-4 mb-2">Connection</p>
      <div class="flex justify-between text-sm py-1 border-b border-slate-50">
        <span class="text-slate-500">VPN IPv4</span>
        <span class="text-slate-700 font-medium font-mono text-xs">{detail.ipv4 || '—'}</span>
      </div>
      <div class="flex justify-between text-sm py-1 border-b border-slate-50">
        <span class="text-slate-500">VPN IPv6</span>
        <span class="text-slate-700 font-medium font-mono text-xs">{detail.ipv6 || '—'}</span>
      </div>
      <div class="flex justify-between text-sm py-1 border-b border-slate-50">
        <span class="text-slate-500">Client IP</span>
        <span class="text-slate-700 font-medium font-mono text-xs">{detail.remoteAddress || '—'}</span>
      </div>
      <div class="flex justify-between text-sm py-1 border-b border-slate-50">
        <span class="text-slate-500">Connected</span>
        <span class="text-slate-700 font-medium text-right">{formatAbsoluteTime(detail.connectedAt)}</span>
      </div>
      <div class="flex justify-between text-sm py-1 border-b border-slate-50">
        <span class="text-slate-500">Duration</span>
        <span class="text-slate-700 font-medium">{formatDurationSecs(detail.durationSeconds)}</span>
      </div>
      <div class="flex justify-between text-sm py-1 border-b border-slate-50">
        <span class="text-slate-500">Device</span>
        <span class="text-slate-700 font-medium">{detail.device || '—'}</span>
      </div>

      <!-- Traffic -->
      <p class="text-xs font-semibold text-slate-500 uppercase tracking-wider mt-4 mb-2">Traffic</p>
      <div class="flex justify-between text-sm py-1 border-b border-slate-50">
        <span class="text-slate-500">Received</span>
        <span class="text-slate-700 font-medium">{formatBytes(detail.rxBytes)}</span>
      </div>
      <div class="flex justify-between text-sm py-1 border-b border-slate-50">
        <span class="text-slate-500">Transmitted</span>
        <span class="text-slate-700 font-medium">{formatBytes(detail.txBytes)}</span>
      </div>

      <!-- Client -->
      <p class="text-xs font-semibold text-slate-500 uppercase tracking-wider mt-4 mb-2">Client</p>
      <div class="flex justify-between text-sm py-1 border-b border-slate-50">
        <span class="text-slate-500">User Agent</span>
        <span class="text-slate-700 font-medium text-right max-w-[60%] break-words">{detail.userAgent || '—'}</span>
      </div>
      <div class="flex justify-between text-sm py-1 border-b border-slate-50">
        <span class="text-slate-500">Stability</span>
        {#if detail.reconnectCount > 2}
          <span class="px-2 py-0.5 rounded text-xs font-medium bg-yellow-100 text-yellow-700">
            {detail.reconnectCount} reconnects
          </span>
        {:else}
          <span class="text-slate-700 font-medium">Stable</span>
        {/if}
      </div>

      <!-- Security Section (new) -->
      {#if userConnection}
        <p class="text-xs font-semibold text-slate-500 uppercase tracking-wider mt-4 mb-2 flex items-center gap-1.5">
          <Shield class="w-3 h-3" /> Security
        </p>
        <div class="flex justify-between text-sm py-1 border-b border-slate-50">
          <span class="text-slate-500">TLS Cipher</span>
          <button
            type="button"
            title="Copy TLS cipher"
            onclick={() => copyToClipboard(userConnection.tlsCipher)}
            class="text-slate-700 font-mono text-xs text-right max-w-[60%] truncate hover:text-blue-600 cursor-pointer"
          >
            {userConnection.tlsCipher || '—'}
          </button>
        </div>
        <div class="flex justify-between text-sm py-1 border-b border-slate-50">
          <span class="text-slate-500">DTLS Cipher</span>
          <button
            type="button"
            title="Copy DTLS cipher"
            onclick={() => copyToClipboard(userConnection.dtlsCipher)}
            class="text-slate-700 font-mono text-xs text-right max-w-[60%] truncate hover:text-blue-600 cursor-pointer"
          >
            {userConnection.dtlsCipher || '—'}
          </button>
        </div>
        <div class="flex justify-between text-sm py-1 border-b border-slate-50">
          <span class="text-slate-500">Cert Auth</span>
          <span class="text-slate-700 font-medium">No</span>
        </div>

        <!-- Network Policy Section (new) -->
        <p class="text-xs font-semibold text-slate-500 uppercase tracking-wider mt-4 mb-2 flex items-center gap-1.5">
          <Network class="w-3 h-3" /> Network Policy
        </p>
        <div class="flex justify-between text-sm py-1 border-b border-slate-50">
          <span class="text-slate-500">MTU</span>
          <span class="text-slate-700 font-medium">{userConnection.mtu || '—'}</span>
        </div>
        <div class="flex justify-between text-sm py-1 border-b border-slate-50">
          <span class="text-slate-500">DPD Timeout</span>
          <span class="text-slate-700 font-medium">{userConnection.dpd ? `${userConnection.dpd}s` : '—'}</span>
        </div>
        <div class="flex justify-between text-sm py-1 border-b border-slate-50">
          <span class="text-slate-500">KeepAlive</span>
          <span class="text-slate-700 font-medium">{userConnection.keepAlive ? `${userConnection.keepAlive}s` : '—'}</span>
        </div>
        <div class="flex justify-between text-sm py-1 border-b border-slate-50">
          <span class="text-slate-500">Restricted Routes</span>
          <span class="text-slate-700 font-medium">{userConnection.restrictedToRoutes ? 'Yes' : 'No'}</span>
        </div>
      {/if}

      <!-- Routes -->
      <p class="text-xs font-semibold text-slate-500 uppercase tracking-wider mt-4 mb-2">Assigned Routes</p>
      {#if detail.routes && detail.routes.length > 0}
        <ul class="space-y-0.5">
          {#each detail.routes as route}
            <li class="text-xs font-mono text-slate-700 py-0.5">{route}</li>
          {/each}
        </ul>
      {:else}
        <p class="text-xs text-slate-500">None assigned</p>
      {/if}

      <!-- DNS -->
      <p class="text-xs font-semibold text-slate-500 uppercase tracking-wider mt-4 mb-2">Assigned DNS</p>
      {#if detail.dns && detail.dns.length > 0}
        <ul class="space-y-0.5">
          {#each detail.dns as entry}
            <li class="text-xs font-mono text-slate-700 py-0.5">{entry}</li>
          {/each}
        </ul>
      {:else}
        <p class="text-xs text-slate-500">None assigned</p>
      {/if}

      <!-- Panel actions -->
      {#if isOwner && session}
        <div class="flex flex-col gap-2 pt-4 pb-2 border-t border-slate-100 mt-4">
          <div class="flex items-center gap-2">
            <button
              type="button"
              onclick={() => onDisconnectSession(session)}
              class="flex-1 px-3 py-2 text-sm font-medium rounded-md bg-orange-100 text-orange-700 hover:bg-orange-200 transition-colors"
            >
              Disconnect
            </button>
            <button
              type="button"
              onclick={() => onTerminateSession(session)}
              class="flex-1 px-3 py-2 text-sm font-medium rounded-md bg-red-100 text-red-700 hover:bg-red-200 transition-colors"
            >
              Terminate
            </button>
          </div>
          <button
            type="button"
            onclick={() => onDisconnectUser(session.username)}
            class="w-full px-3 py-2 text-sm font-medium rounded-md bg-orange-50 text-orange-600 border border-orange-200 hover:bg-orange-100 transition-colors"
          >
            Disconnect All Sessions for {session.username}
          </button>
        </div>
      {/if}
    {/if}
  </div>
</div>
```

- [ ] **Step 2: Commit**

```bash
git add frontend/src/routes/sessions/SessionDetailDrawer.svelte
git commit -m "feat(sessions): add SessionDetailDrawer with security and network policy sections"
```

---

## Task 7: Create EventsTab Component

**Files:**
- Create: `frontend/src/routes/sessions/EventsTab.svelte`

- [ ] **Step 1: Create EventsTab.svelte**

```svelte
<script lang="ts">
  import { onMount } from 'svelte';
  import CardTable from '$lib/components/ui/CardTable.svelte';
  import EmptyState from '$lib/components/ui/EmptyState.svelte';
  import { Activity } from 'lucide-svelte';
  import { formatRelativeTime } from '$lib/utils/format';
  import { getConnectionEvents, ApiError, type ConnectionEvent } from '$lib/api';

  type Props = {
    refreshSignal: number;
  };

  let { refreshSignal }: Props = $props();

  let events = $state<ConnectionEvent[]>([]);
  let loading = $state(true);
  let error = $state('');
  let lastFetchedSignal = $state(0);

  async function loadEvents() {
    loading = events.length === 0;
    error = '';
    try {
      events = await getConnectionEvents();
    } catch (caught) {
      if (caught instanceof ApiError && caught.status === 503) {
        error = 'ocserv is not available.';
      } else {
        error = caught instanceof Error ? caught.message : 'Failed to load events.';
      }
    } finally {
      loading = false;
    }
  }

  onMount(() => { void loadEvents(); });

  $effect(() => {
    if (refreshSignal > lastFetchedSignal) {
      lastFetchedSignal = refreshSignal;
      void loadEvents();
    }
  });
</script>

{#if loading}
  <div class="flex items-center justify-center py-12">
    <div class="w-6 h-6 border-2 border-slate-200 border-t-slate-500 rounded-full animate-spin"></div>
  </div>
{:else if error}
  <div class="px-4 py-3 rounded-lg text-sm bg-red-50 text-red-800 m-4" role="alert">{error}</div>
{:else if events.length === 0}
  <EmptyState
    icon={Activity}
    title="No connection events"
    description="Connection events will appear here when users connect or disconnect."
  />
{:else}
  <CardTable title="Connection Events">
    <table class="w-full">
      <thead>
        <tr>
          <th class="px-4 py-3 text-left text-xs font-medium text-slate-500 uppercase tracking-wider bg-slate-50">Time</th>
          <th class="px-4 py-3 text-left text-xs font-medium text-slate-500 uppercase tracking-wider bg-slate-50">Username</th>
          <th class="px-4 py-3 text-left text-xs font-medium text-slate-500 uppercase tracking-wider bg-slate-50">Remote IP</th>
          <th class="px-4 py-3 text-left text-xs font-medium text-slate-500 uppercase tracking-wider bg-slate-50">Event</th>
          <th class="px-4 py-3 text-left text-xs font-medium text-slate-500 uppercase tracking-wider bg-slate-50">Message</th>
        </tr>
      </thead>
      <tbody class="divide-y divide-slate-100">
        {#each events as event}
          <tr class="hover:bg-slate-50">
            <td class="px-4 py-3 text-sm text-slate-700">{formatRelativeTime(event.timestamp)}</td>
            <td class="px-4 py-3 text-sm text-slate-700 font-medium">{event.username || '—'}</td>
            <td class="px-4 py-3 text-sm font-mono text-xs text-slate-700">{event.remoteIp || '—'}</td>
            <td class="px-4 py-3">
              <span class="inline-flex px-2 py-0.5 rounded text-xs font-medium
                {event.eventType === 'connect' ? 'bg-green-100 text-green-700' :
                 event.eventType === 'disconnect' ? 'bg-orange-100 text-orange-700' :
                 'bg-slate-100 text-slate-700'}">
                {event.eventType}
              </span>
            </td>
            <td class="px-4 py-3 text-sm text-slate-600 max-w-xs truncate">{event.message || '—'}</td>
          </tr>
        {/each}
      </tbody>
    </table>
  </CardTable>
{/if}
```

- [ ] **Step 2: Commit**

```bash
git add frontend/src/routes/sessions/EventsTab.svelte
git commit -m "feat(sessions): add EventsTab component for connection events"
```

---

## Task 8: Create RoutesTab Component

**Files:**
- Create: `frontend/src/routes/sessions/RoutesTab.svelte`

- [ ] **Step 1: Create RoutesTab.svelte**

```svelte
<script lang="ts">
  import { onMount } from 'svelte';
  import CardTable from '$lib/components/ui/CardTable.svelte';
  import EmptyState from '$lib/components/ui/EmptyState.svelte';
  import { Route } from 'lucide-svelte';
  import { getUserRoutes, ApiError, type UserRoute } from '$lib/api';

  type Props = {
    refreshSignal: number;
  };

  let { refreshSignal }: Props = $props();

  let routes = $state<UserRoute[]>([]);
  let loading = $state(true);
  let error = $state('');
  let lastFetchedSignal = $state(0);

  async function loadRoutes() {
    loading = routes.length === 0;
    error = '';
    try {
      routes = await getUserRoutes();
    } catch (caught) {
      if (caught instanceof ApiError && caught.status === 503) {
        error = 'ocserv is not available.';
      } else {
        error = caught instanceof Error ? caught.message : 'Failed to load routes.';
      }
    } finally {
      loading = false;
    }
  }

  onMount(() => { void loadRoutes(); });

  $effect(() => {
    if (refreshSignal > lastFetchedSignal) {
      lastFetchedSignal = refreshSignal;
      void loadRoutes();
    }
  });
</script>

{#if loading}
  <div class="flex items-center justify-center py-12">
    <div class="w-6 h-6 border-2 border-slate-200 border-t-slate-500 rounded-full animate-spin"></div>
  </div>
{:else if error}
  <div class="px-4 py-3 rounded-lg text-sm bg-red-50 text-red-800 m-4" role="alert">{error}</div>
{:else if routes.length === 0}
  <EmptyState
    icon={Route}
    title="No user-provided routes"
    description="When users push routes to the server, they will appear here."
  />
{:else}
  <CardTable title="User Routes (iroutes)">
    <table class="w-full">
      <thead>
        <tr>
          <th class="px-4 py-3 text-left text-xs font-medium text-slate-500 uppercase tracking-wider bg-slate-50">ID</th>
          <th class="px-4 py-3 text-left text-xs font-medium text-slate-500 uppercase tracking-wider bg-slate-50">Username</th>
          <th class="px-4 py-3 text-left text-xs font-medium text-slate-500 uppercase tracking-wider bg-slate-50">VHost</th>
          <th class="px-4 py-3 text-left text-xs font-medium text-slate-500 uppercase tracking-wider bg-slate-50">Device</th>
          <th class="px-4 py-3 text-left text-xs font-medium text-slate-500 uppercase tracking-wider bg-slate-50">VPN IP</th>
          <th class="px-4 py-3 text-left text-xs font-medium text-slate-500 uppercase tracking-wider bg-slate-50">Route</th>
        </tr>
      </thead>
      <tbody class="divide-y divide-slate-100">
        {#each routes as route}
          <tr class="hover:bg-slate-50">
            <td class="px-4 py-3 text-sm font-mono text-xs text-slate-700">{route.id}</td>
            <td class="px-4 py-3 text-sm text-slate-700 font-medium">{route.username}</td>
            <td class="px-4 py-3 text-sm text-slate-700">{route.vhost}</td>
            <td class="px-4 py-3 text-sm font-mono text-xs text-slate-700">{route.device}</td>
            <td class="px-4 py-3 text-sm font-mono text-xs text-slate-700">{route.vpnIp}</td>
            <td class="px-4 py-3 text-sm font-mono text-xs text-slate-700">{route.route}</td>
          </tr>
        {/each}
      </tbody>
    </table>
  </CardTable>
{/if}
```

- [ ] **Step 2: Commit**

```bash
git add frontend/src/routes/sessions/RoutesTab.svelte
git commit -m "feat(sessions): add RoutesTab component for user-provided routes"
```

---

## Task 9: Create BansTab Component

**Files:**
- Create: `frontend/src/routes/sessions/BansTab.svelte`

- [ ] **Step 1: Create BansTab.svelte**

```svelte
<script lang="ts">
  import { onMount } from 'svelte';
  import CardTable from '$lib/components/ui/CardTable.svelte';
  import EmptyState from '$lib/components/ui/EmptyState.svelte';
  import UnbanConfirmDialog from './UnbanConfirmDialog.svelte';
  import { ShieldBan } from 'lucide-svelte';
  import { formatRelativeTime } from '$lib/utils/format';
  import {
    getBannedIPs,
    getBanPoints,
    unbanIP,
    ApiError,
    type BannedIP,
    type BanPointEntry
  } from '$lib/api';

  type Props = {
    refreshSignal: number;
    isOwner: boolean;
  };

  let { refreshSignal, isOwner }: Props = $props();

  let bans = $state<BannedIP[]>([]);
  let points = $state<BanPointEntry[]>([]);
  let loading = $state(true);
  let error = $state('');
  let lastFetchedSignal = $state(0);

  // Unban dialog state
  let unbanTarget = $state<string | null>(null);
  let unbanSubmitting = $state(false);
  let unbanError = $state('');
  let successBanner = $state('');

  async function loadBans() {
    loading = bans.length === 0 && points.length === 0;
    error = '';
    try {
      const [bansData, pointsData] = await Promise.all([getBannedIPs(), getBanPoints()]);
      bans = bansData;
      points = pointsData;
    } catch (caught) {
      if (caught instanceof ApiError && caught.status === 503) {
        error = 'ocserv is not available.';
      } else {
        error = caught instanceof Error ? caught.message : 'Failed to load ban data.';
      }
    } finally {
      loading = false;
    }
  }

  onMount(() => { void loadBans(); });

  $effect(() => {
    if (refreshSignal > lastFetchedSignal) {
      lastFetchedSignal = refreshSignal;
      void loadBans();
    }
  });

  function confirmUnban(ip: string) {
    unbanTarget = ip;
    unbanError = '';
  }

  function cancelUnban() {
    unbanTarget = null;
    unbanError = '';
  }

  async function executeUnban() {
    if (!unbanTarget || unbanSubmitting) return;
    unbanSubmitting = true;
    unbanError = '';
    try {
      await unbanIP(unbanTarget);
      bans = bans.filter(b => b.ip !== unbanTarget);
      points = points.filter(p => p.ip !== unbanTarget);
      successBanner = `Unbanned ${unbanTarget}`;
      unbanTarget = null;
    } catch (caught) {
      unbanError = caught instanceof Error ? caught.message : 'Failed to unban IP.';
    } finally {
      unbanSubmitting = false;
    }
  }
</script>

{#if loading}
  <div class="flex items-center justify-center py-12">
    <div class="w-6 h-6 border-2 border-slate-200 border-t-slate-500 rounded-full animate-spin"></div>
  </div>
{:else if error}
  <div class="px-4 py-3 rounded-lg text-sm bg-red-50 text-red-800 m-4" role="alert">{error}</div>
{:else}
  <div class="space-y-6 p-4">
    {#if successBanner}
      <div class="flex items-center justify-between px-4 py-3 rounded-lg text-sm bg-green-50 text-green-800" role="status">
        <span>{successBanner}</span>
        <button type="button" onclick={() => (successBanner = '')} class="ml-4 text-green-600 hover:text-green-800" aria-label="Dismiss">
          &times;
        </button>
      </div>
    {/if}

    <!-- Banned IPs -->
    {#if bans.length === 0 && points.length === 0}
      <EmptyState
        icon={ShieldBan}
        title="No banned IPs"
        description="When IPs are banned due to excessive authentication failures, they will appear here."
      />
    {:else}
      {#if bans.length > 0}
        <CardTable title="Banned IPs ({bans.length})">
          <table class="w-full">
            <thead>
              <tr>
                <th class="px-4 py-3 text-left text-xs font-medium text-slate-500 uppercase tracking-wider bg-slate-50">IP Address</th>
                <th class="px-4 py-3 text-left text-xs font-medium text-slate-500 uppercase tracking-wider bg-slate-50">Score</th>
                <th class="px-4 py-3 text-left text-xs font-medium text-slate-500 uppercase tracking-wider bg-slate-50">Reason</th>
                {#if isOwner}
                  <th class="px-4 py-3 text-left text-xs font-medium text-slate-500 uppercase tracking-wider bg-slate-50">Action</th>
                {/if}
              </tr>
            </thead>
            <tbody class="divide-y divide-slate-100">
              {#each bans as ban}
                <tr class="hover:bg-slate-50">
                  <td class="px-4 py-3 text-sm font-mono text-slate-700">{ban.ip}</td>
                  <td class="px-4 py-3 text-sm text-slate-700">{ban.score}</td>
                  <td class="px-4 py-3 text-sm text-slate-600">{ban.reason || '—'}</td>
                  {#if isOwner}
                    <td class="px-4 py-3">
                      <button
                        type="button"
                        onclick={() => confirmUnban(ban.ip)}
                        class="px-2 py-1 text-xs font-medium rounded bg-blue-100 text-blue-700 hover:bg-blue-200 transition-colors"
                      >
                        Unban
                      </button>
                    </td>
                  {/if}
                </tr>
              {/each}
            </tbody>
          </table>
        </CardTable>
      {/if}

      {#if points.length > 0}
        <CardTable title="IP Ban Points ({points.length})">
          <table class="w-full">
            <thead>
              <tr>
                <th class="px-4 py-3 text-left text-xs font-medium text-slate-500 uppercase tracking-wider bg-slate-50">IP Address</th>
                <th class="px-4 py-3 text-left text-xs font-medium text-slate-500 uppercase tracking-wider bg-slate-50">Points</th>
                <th class="px-4 py-3 text-left text-xs font-medium text-slate-500 uppercase tracking-wider bg-slate-50">Since</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-slate-100">
              {#each points as entry}
                <tr class="hover:bg-slate-50">
                  <td class="px-4 py-3 text-sm font-mono text-slate-700">{entry.ip}</td>
                  <td class="px-4 py-3 text-sm text-slate-700">{entry.points}</td>
                  <td class="px-4 py-3 text-sm text-slate-700">{formatRelativeTime(entry.since)}</td>
                </tr>
              {/each}
            </tbody>
          </table>
        </CardTable>
      {/if}
    {/if}
  </div>
{/if}

{#if unbanTarget}
  <UnbanConfirmDialog
    ip={unbanTarget}
    submitting={unbanSubmitting}
    error={unbanError}
    onConfirm={executeUnban}
    onCancel={cancelUnban}
  />
{/if}
```

- [ ] **Step 2: Commit**

```bash
git add frontend/src/routes/sessions/BansTab.svelte
git commit -m "feat(sessions): add BansTab component with unban action"
```

---

## Task 10: Create AllSessionsTab Component

**Files:**
- Create: `frontend/src/routes/sessions/AllSessionsTab.svelte`

- [ ] **Step 1: Create AllSessionsTab.svelte**

```svelte
<script lang="ts">
  import { onMount } from 'svelte';
  import CardTable from '$lib/components/ui/CardTable.svelte';
  import PaginatedTable from '$lib/components/ui/PaginatedTable.svelte';
  import EmptyState from '$lib/components/ui/EmptyState.svelte';
  import { Copy, Users } from 'lucide-svelte';
  import { formatRelativeTime } from '$lib/utils/format';
  import { paginate } from '$lib/utils/pagination';
  import { getOcservSessions, ApiError, type OcservSession } from '$lib/api';

  type Props = {
    refreshSignal: number;
  };

  let { refreshSignal }: Props = $props();

  const PAGE_SIZE = 20;

  let sessions = $state<OcservSession[]>([]);
  let loading = $state(true);
  let error = $state('');
  let page = $state(1);
  let lastFetchedSignal = $state(0);

  const pagedSessions = $derived(paginate(sessions, page, PAGE_SIZE));

  async function loadSessions() {
    loading = sessions.length === 0;
    error = '';
    try {
      sessions = await getOcservSessions();
    } catch (caught) {
      if (caught instanceof ApiError && caught.status === 503) {
        error = 'ocserv is not available.';
      } else {
        error = caught instanceof Error ? caught.message : 'Failed to load sessions.';
      }
    } finally {
      loading = false;
    }
  }

  onMount(() => { void loadSessions(); });

  $effect(() => {
    if (refreshSignal > lastFetchedSignal) {
      lastFetchedSignal = refreshSignal;
      void loadSessions();
    }
  });

  function statusBadgeClass(session: OcservSession): string {
    // Determine status from session fields
    if (session.device) return 'bg-green-100 text-green-700';
    return 'bg-blue-100 text-blue-700';
  }

  function statusLabel(session: OcservSession): string {
    if (session.device) return 'connected';
    return 'authenticated';
  }
</script>

{#if loading}
  <div class="flex items-center justify-center py-12">
    <div class="w-6 h-6 border-2 border-slate-200 border-t-slate-500 rounded-full animate-spin"></div>
  </div>
{:else if error}
  <div class="px-4 py-3 rounded-lg text-sm bg-red-50 text-red-800 m-4" role="alert">{error}</div>
{:else if sessions.length === 0}
  <EmptyState
    icon={Users}
    title="No sessions"
    description="No active or authenticated sessions found."
  />
{:else}
  <CardTable title="All Sessions ({sessions.length})">
    <PaginatedTable
      totalPages={pagedSessions.totalPages}
      currentPage={pagedSessions.page}
      onPageChange={(p) => (page = p)}
    >
      <table class="w-full">
        <thead>
          <tr>
            <th class="px-4 py-3 text-left text-xs font-medium text-slate-500 uppercase tracking-wider bg-slate-50">Username</th>
            <th class="px-4 py-3 text-left text-xs font-medium text-slate-500 uppercase tracking-wider bg-slate-50">Session ID</th>
            <th class="px-4 py-3 text-left text-xs font-medium text-slate-500 uppercase tracking-wider bg-slate-50">Status</th>
            <th class="px-4 py-3 text-left text-xs font-medium text-slate-500 uppercase tracking-wider bg-slate-50">VPN IP</th>
            <th class="px-4 py-3 text-left text-xs font-medium text-slate-500 uppercase tracking-wider bg-slate-50">Client IP</th>
            <th class="px-4 py-3 text-left text-xs font-medium text-slate-500 uppercase tracking-wider bg-slate-50">Connected</th>
            <th class="px-4 py-3 text-left text-xs font-medium text-slate-500 uppercase tracking-wider bg-slate-50">Client</th>
          </tr>
        </thead>
        <tbody class="divide-y divide-slate-100">
          {#each pagedSessions.items as session (session.id)}
            <tr class="hover:bg-slate-50">
              <td class="px-4 py-3 text-sm text-slate-700 font-medium">{session.username}</td>
              <td class="px-4 py-3">
                <div class="flex items-center gap-1">
                  <span class="font-mono text-xs text-slate-500">{session.id.slice(0, 8)}</span>
                  <button
                    type="button"
                    title="Copy session ID"
                    onclick={() => navigator.clipboard.writeText(session.id)}
                    class="p-0.5 text-slate-300 hover:text-slate-500 transition-colors"
                    aria-label="Copy session ID"
                  >
                    <Copy class="w-3 h-3" />
                  </button>
                </div>
              </td>
              <td class="px-4 py-3">
                <span class="inline-flex px-2 py-0.5 rounded text-xs font-medium {statusBadgeClass(session)}">
                  {statusLabel(session)}
                </span>
              </td>
              <td class="px-4 py-3 text-sm font-mono text-xs text-slate-700">{session.vpnAddress || '—'}</td>
              <td class="px-4 py-3 text-sm font-mono text-xs text-slate-700">{session.remoteAddress || '—'}</td>
              <td class="px-4 py-3 text-sm text-slate-700">{formatRelativeTime(session.connectedAt)}</td>
              <td class="px-4 py-3 text-sm text-slate-700">{session.userAgent || '—'}</td>
            </tr>
          {/each}
        </tbody>
      </table>
    </PaginatedTable>
  </CardTable>
{/if}
```

- [ ] **Step 2: Commit**

```bash
git add frontend/src/routes/sessions/AllSessionsTab.svelte
git commit -m "feat(sessions): add AllSessionsTab component with status badges"
```

---

## Task 11: Create ActiveSessionsTab Component

**Files:**
- Create: `frontend/src/routes/sessions/ActiveSessionsTab.svelte`

This extracts the existing table logic from `+page.svelte` into a standalone tab component.

- [ ] **Step 1: Create ActiveSessionsTab.svelte**

```svelte
<script lang="ts">
  import { onMount } from 'svelte';
  import CardTable from '$lib/components/ui/CardTable.svelte';
  import PaginatedTable from '$lib/components/ui/PaginatedTable.svelte';
  import EmptyState from '$lib/components/ui/EmptyState.svelte';
  import SessionDetailDrawer from './SessionDetailDrawer.svelte';
  import { Copy, Wifi } from 'lucide-svelte';
  import { formatRelativeTime, formatAbsoluteTime } from '$lib/utils/format';
  import { paginate } from '$lib/utils/pagination';
  import { formatBytes } from '$lib/monitoring';
  import {
    getOcservSessions,
    getOcservSessionDetail,
    getUserConnections,
    ApiError,
    type OcservSession,
    type OcservSessionDetail,
    type UserConnection
  } from '$lib/api';

  type Props = {
    isOwner: boolean;
    refreshSignal: number;
    onDisconnectSession: (session: OcservSession) => void;
    onTerminateSession: (session: OcservSession) => void;
    onDisconnectUser: (username: string) => void;
  };

  let { isOwner, refreshSignal, onDisconnectSession, onTerminateSession, onDisconnectUser }: Props = $props();

  const PAGE_SIZE = 20;

  let sessions = $state<OcservSession[]>([]);
  let loading = $state(true);
  let error = $state('');
  let page = $state(1);
  let lastFetchedSignal = $state(0);

  // Sorting
  let sortColumn = $state<'username' | 'connected' | 'duration'>('connected');
  let sortDirection = $state<'asc' | 'desc'>('asc');

  // Detail panel
  let selectedSessionId = $state<string | null>(null);
  let selectedDetail = $state<OcservSessionDetail | null>(null);
  let selectedUserConn = $state<UserConnection | null>(null);
  let detailLoading = $state(false);
  let detailError = $state('');
  let sessionGone = $state(false);

  // Derived
  const sortedSessions = $derived((() => {
    const copy = [...sessions];
    copy.sort((a, b) => {
      let cmp = 0;
      if (sortColumn === 'username') {
        cmp = (a.username ?? '').localeCompare(b.username ?? '');
      } else if (sortColumn === 'connected') {
        const ta = a.connectedAt ? new Date(a.connectedAt).getTime() : 0;
        const tb = b.connectedAt ? new Date(b.connectedAt).getTime() : 0;
        cmp = ta - tb;
      } else if (sortColumn === 'duration') {
        const da = a.connectedAt ? Date.now() - new Date(a.connectedAt).getTime() : 0;
        const db = b.connectedAt ? Date.now() - new Date(b.connectedAt).getTime() : 0;
        cmp = da - db;
      }
      return sortDirection === 'asc' ? cmp : -cmp;
    });
    return copy;
  })());

  const pagedSessions = $derived(paginate(sortedSessions, page, PAGE_SIZE));
  const selectedSession = $derived(sessions.find(s => s.id === selectedSessionId));

  // Helpers
  function formatDuration(connectedAt: string | undefined): string {
    if (!connectedAt) return '—';
    const ms = Date.now() - new Date(connectedAt).getTime();
    if (isNaN(ms) || ms < 0) return '—';
    const totalSec = Math.floor(ms / 1000);
    const h = Math.floor(totalSec / 3600);
    const m = Math.floor((totalSec % 3600) / 60);
    if (h > 0) return `${h}h ${m}m`;
    return `${m}m`;
  }

  function isLongRunning(connectedAt: string | undefined): boolean {
    if (!connectedAt) return false;
    const ms = Date.now() - new Date(connectedAt).getTime();
    return !isNaN(ms) && ms > 24 * 60 * 60 * 1000;
  }

  function userAgentShort(ua: string): string {
    if (!ua) return 'Unknown';
    if (/anyconnect/i.test(ua)) return 'AnyConnect';
    if (/openconnect/i.test(ua)) return 'OpenConnect';
    return 'Unknown';
  }

  function toggleSort(column: 'username' | 'connected' | 'duration') {
    if (sortColumn === column) {
      sortDirection = sortDirection === 'asc' ? 'desc' : 'asc';
    } else {
      sortColumn = column;
      sortDirection = 'asc';
    }
    page = 1;
  }

  function sortIcon(column: 'username' | 'connected' | 'duration'): string {
    if (sortColumn !== column) return ' ↕';
    return sortDirection === 'asc' ? ' ↑' : ' ↓';
  }

  // Data loading
  async function loadSessions() {
    const isFirstLoad = sessions.length === 0;
    if (isFirstLoad) loading = true;
    error = '';
    try {
      sessions = await getOcservSessions();
    } catch (caught) {
      if (caught instanceof ApiError && caught.status === 503) {
        error = 'ocserv is not available.';
      } else {
        error = caught instanceof Error ? caught.message : 'Failed to load sessions.';
      }
    } finally {
      loading = false;
    }
  }

  async function loadSessionDetail(id: string, username: string) {
    selectedDetail = null;
    selectedUserConn = null;
    detailError = '';
    sessionGone = false;
    detailLoading = true;
    try {
      const [detail, conns] = await Promise.all([
        getOcservSessionDetail(id),
        getUserConnections(username)
      ]);
      selectedDetail = detail;
      // Find the matching connection by session ID
      selectedUserConn = conns.find(c => c.sessionId === id) ?? conns[0] ?? null;
    } catch (caught) {
      if (caught instanceof ApiError && caught.status === 404) {
        sessionGone = true;
      } else {
        detailError = caught instanceof Error ? caught.message : 'Unable to load session detail.';
      }
    } finally {
      detailLoading = false;
    }
  }

  function selectSession(session: OcservSession) {
    selectedSessionId = session.id;
    void loadSessionDetail(session.id, session.username);
  }

  function closePanel() {
    selectedSessionId = null;
    selectedDetail = null;
    selectedUserConn = null;
    detailError = '';
    sessionGone = false;
  }

  // Remove a session from local list (called after disconnect/terminate)
  export function removeSession(sessionId: string) {
    sessions = sessions.filter(s => s.id !== sessionId);
    if (selectedSessionId === sessionId) closePanel();
  }

  onMount(() => { void loadSessions(); });

  $effect(() => {
    if (refreshSignal > lastFetchedSignal) {
      lastFetchedSignal = refreshSignal;
      void loadSessions();
    }
  });
</script>

{#if loading}
  <div class="flex items-center justify-center py-12">
    <div class="w-6 h-6 border-2 border-slate-200 border-t-slate-500 rounded-full animate-spin"></div>
  </div>
{:else if error}
  <div class="px-4 py-3 rounded-lg text-sm bg-red-50 text-red-800 m-4" role="alert">{error}</div>
{:else if sessions.length === 0}
  <EmptyState
    icon={Wifi}
    title="No active sessions"
    description="No users are currently connected to the VPN."
  />
{:else}
  <div class:flex={selectedSessionId !== null} class:gap-4={selectedSessionId !== null} class:items-start={selectedSessionId !== null}>
    <!-- Sessions table -->
    <div class:flex-1={selectedSessionId !== null} class:min-w-0={selectedSessionId !== null}>
      <CardTable title="Active sessions ({sessions.length})">
        <PaginatedTable
          totalPages={pagedSessions.totalPages}
          currentPage={pagedSessions.page}
          onPageChange={(p) => (page = p)}
        >
          <table class="w-full">
            <thead>
              <tr>
                <th
                  class="px-4 py-3 text-left text-xs font-medium text-slate-500 uppercase tracking-wider bg-slate-50 cursor-pointer hover:bg-slate-100 select-none"
                  role="columnheader"
                  aria-sort={sortColumn === 'username' ? (sortDirection === 'asc' ? 'ascending' : 'descending') : 'none'}
                  tabindex="0"
                  onclick={() => toggleSort('username')}
                  onkeydown={(e: KeyboardEvent) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); toggleSort('username'); } }}
                >
                  Username{sortIcon('username')}
                </th>
                <th class="px-4 py-3 text-left text-xs font-medium text-slate-500 uppercase tracking-wider bg-slate-50">Session ID</th>
                <th class="px-4 py-3 text-left text-xs font-medium text-slate-500 uppercase tracking-wider bg-slate-50">VPN IP</th>
                <th class="px-4 py-3 text-left text-xs font-medium text-slate-500 uppercase tracking-wider bg-slate-50">Client IP</th>
                <th
                  class="px-4 py-3 text-left text-xs font-medium text-slate-500 uppercase tracking-wider bg-slate-50 cursor-pointer hover:bg-slate-100 select-none"
                  role="columnheader"
                  aria-sort={sortColumn === 'connected' ? (sortDirection === 'asc' ? 'ascending' : 'descending') : 'none'}
                  tabindex="0"
                  onclick={() => toggleSort('connected')}
                  onkeydown={(e: KeyboardEvent) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); toggleSort('connected'); } }}
                >
                  Connected{sortIcon('connected')}
                </th>
                <th
                  class="px-4 py-3 text-left text-xs font-medium text-slate-500 uppercase tracking-wider bg-slate-50 cursor-pointer hover:bg-slate-100 select-none"
                  role="columnheader"
                  aria-sort={sortColumn === 'duration' ? (sortDirection === 'asc' ? 'ascending' : 'descending') : 'none'}
                  tabindex="0"
                  onclick={() => toggleSort('duration')}
                  onkeydown={(e: KeyboardEvent) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); toggleSort('duration'); } }}
                >
                  Duration{sortIcon('duration')}
                </th>
                <th class="px-4 py-3 text-left text-xs font-medium text-slate-500 uppercase tracking-wider bg-slate-50">RX / TX</th>
                {#if !selectedSessionId}
                  <th class="px-4 py-3 text-left text-xs font-medium text-slate-500 uppercase tracking-wider bg-slate-50">Client</th>
                  <th class="px-4 py-3 text-left text-xs font-medium text-slate-500 uppercase tracking-wider bg-slate-50">Stability</th>
                {/if}
                {#if isOwner}
                  <th class="px-4 py-3 text-left text-xs font-medium text-slate-500 uppercase tracking-wider bg-slate-50">Actions</th>
                {/if}
              </tr>
            </thead>
            <tbody class="divide-y divide-slate-100">
              {#each pagedSessions.items as session (session.id)}
                <tr
                  class="hover:bg-slate-50 cursor-pointer transition-colors"
                  class:bg-blue-50={session.id === selectedSessionId}
                  onclick={() => selectSession(session)}
                >
                  <td class="px-4 py-3 text-sm text-slate-700 font-medium">{session.username}</td>
                  <td class="px-4 py-3">
                    <div class="flex items-center gap-1">
                      <span class="font-mono text-xs text-slate-500">{session.id.slice(0, 8)}</span>
                      <button
                        type="button"
                        title="Copy full session ID"
                        onclick={(e) => { e.stopPropagation(); navigator.clipboard.writeText(session.id); }}
                        class="p-0.5 text-slate-300 hover:text-slate-500 transition-colors"
                        aria-label="Copy session ID"
                      >
                        <Copy class="w-3 h-3" />
                      </button>
                    </div>
                  </td>
                  <td class="px-4 py-3 font-mono text-xs text-slate-700">{session.vpnAddress || '—'}</td>
                  <td class="px-4 py-3 font-mono text-xs text-slate-700">{session.remoteAddress || '—'}</td>
                  <td class="px-4 py-3 text-sm text-slate-700" title={formatAbsoluteTime(session.connectedAt)}>
                    {formatRelativeTime(session.connectedAt)}
                  </td>
                  <td class="px-4 py-3 text-sm">
                    {#if isLongRunning(session.connectedAt)}
                      <span class="px-2 py-0.5 rounded text-xs font-medium bg-red-100 text-red-700">{formatDuration(session.connectedAt)}</span>
                    {:else}
                      <span class="text-slate-700">{formatDuration(session.connectedAt)}</span>
                    {/if}
                  </td>
                  <td class="px-4 py-3 text-sm text-slate-700">{formatBytes(session.rxBytes)} / {formatBytes(session.txBytes)}</td>
                  {#if !selectedSessionId}
                    <td class="px-4 py-3 text-sm text-slate-700">{userAgentShort(session.userAgent)}</td>
                    <td class="px-4 py-3 text-sm">
                      {#if session.reconnectCount > 2}
                        <span class="px-2 py-0.5 rounded text-xs font-medium bg-yellow-100 text-yellow-700">{session.reconnectCount} reconnects</span>
                      {:else}
                        <span class="w-2 h-2 rounded-full bg-green-400 inline-block" role="img" aria-label="Stable"></span>
                      {/if}
                    </td>
                  {/if}
                  {#if isOwner}
                    <td class="px-4 py-3">
                      <div class="flex items-center gap-1">
                        <button
                          type="button"
                          onclick={(e) => { e.stopPropagation(); onDisconnectSession(session); }}
                          class="px-2 py-1 text-xs font-medium rounded bg-orange-100 text-orange-700 hover:bg-orange-200 transition-colors"
                        >
                          Disconnect
                        </button>
                        <button
                          type="button"
                          onclick={(e) => { e.stopPropagation(); onTerminateSession(session); }}
                          class="px-2 py-1 text-xs font-medium rounded bg-red-100 text-red-700 hover:bg-red-200 transition-colors"
                        >
                          Terminate
                        </button>
                      </div>
                    </td>
                  {/if}
                </tr>
              {/each}
            </tbody>
          </table>
        </PaginatedTable>
      </CardTable>
    </div>

    <!-- Detail drawer -->
    {#if selectedSessionId !== null}
      <SessionDetailDrawer
        session={selectedSession}
        detail={selectedDetail}
        {userConnection}={selectedUserConn}
        loading={detailLoading}
        error={detailError}
        {sessionGone}
        {isOwner}
        onClose={closePanel}
        {onDisconnectSession}
        {onTerminateSession}
        {onDisconnectUser}
      />
    {/if}
  </div>
{/if}
```

- [ ] **Step 2: Commit**

```bash
git add frontend/src/routes/sessions/ActiveSessionsTab.svelte
git commit -m "feat(sessions): add ActiveSessionsTab extracted from page with detail drawer"
```

---

## Task 12: Rewrite +page.svelte as Tabbed Orchestrator

**Files:**
- Modify: `frontend/src/routes/sessions/+page.svelte`

This is the final assembly. The page becomes a thin orchestrator that renders status cards, tabs, and delegates to tab components.

- [ ] **Step 1: Rewrite +page.svelte**

Replace the entire file content with:

```svelte
<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { RefreshCw, X, AlertTriangle } from 'lucide-svelte';
  import EmptyState from '$lib/components/ui/EmptyState.svelte';
  import SessionStatusCards from './SessionStatusCards.svelte';
  import SessionTabs from './SessionTabs.svelte';
  import ActiveSessionsTab from './ActiveSessionsTab.svelte';
  import AllSessionsTab from './AllSessionsTab.svelte';
  import EventsTab from './EventsTab.svelte';
  import RoutesTab from './RoutesTab.svelte';
  import BansTab from './BansTab.svelte';
  import DisconnectUserDialog from './DisconnectUserDialog.svelte';
  import {
    getServerStatus,
    getMe,
    getSetupStatus,
    disconnectOcservSession,
    terminateOcservSession,
    disconnectUser,
    principalFromAuth,
    hasOwnerRole,
    ApiError,
    type ServerStatus,
    type OcservSession,
    type Principal
  } from '$lib/api';

  // ── page state ─────────────────────────────────────────────────
  let pageLoading = $state(true);
  let pageError = $state('');
  let notConfigured = $state(false);
  let forbidden = $state(false);
  let setupRequired = $state(false);
  let principal = $state<Principal | null>(null);

  // ── server status ──────────────────────────────────────────────
  let serverStatus = $state<ServerStatus | null>(null);
  let statusLoading = $state(true);
  let statusError = $state('');

  // ── tabs ───────────────────────────────────────────────────────
  type TabId = 'active' | 'all' | 'events' | 'routes' | 'bans';
  let activeTab = $state<TabId>('active');
  let refreshSignal = $state(0);

  // ── refresh ────────────────────────────────────────────────────
  let lastRefreshed = $state(new Date());
  let nowTick = $state(Date.now());
  let refreshInterval: ReturnType<typeof setInterval> | null = null;
  let tickInterval: ReturnType<typeof setInterval> | null = null;

  // ── banners ────────────────────────────────────────────────────
  let successBanner = $state('');
  let errorBanner = $state('');

  // ── confirm modals ─────────────────────────────────────────────
  type SessionAction = { type: 'disconnect' | 'terminate'; sessionId: string; username: string } | null;
  let pendingSessionAction = $state<SessionAction>(null);
  let sessionActionError = $state('');
  let sessionActionSubmitting = $state(false);

  let disconnectUserTarget = $state<string | null>(null);
  let disconnectUserError = $state('');
  let disconnectUserSubmitting = $state(false);

  // ── derived ────────────────────────────────────────────────────
  const isOwner = $derived(hasOwnerRole(principal));

  const tabs = $derived([
    { id: 'active', label: 'Active Sessions' },
    { id: 'all', label: 'All Sessions' },
    { id: 'events', label: 'Events' },
    { id: 'routes', label: 'Routes' },
    { id: 'bans', label: 'Bans', badge: serverStatus?.bannedIps ?? 0 }
  ]);

  const updatedAgoText = $derived((() => {
    const diffSec = Math.floor((nowTick - lastRefreshed.getTime()) / 1000);
    if (diffSec < 10) return 'just now';
    if (diffSec < 60) return `${diffSec}s ago`;
    const diffMin = Math.floor(diffSec / 60);
    return `${diffMin}m ago`;
  })());

  // ── lifecycle ──────────────────────────────────────────────────
  onMount(() => {
    // Read initial tab from URL hash
    const hash = window.location.hash.replace('#', '');
    if (['active', 'all', 'events', 'routes', 'bans'].includes(hash)) {
      activeTab = hash as TabId;
    }

    void initPage();
    refreshInterval = setInterval(() => void refreshData(), 30_000);
    tickInterval = setInterval(() => { nowTick = Date.now(); }, 1000);
  });

  onDestroy(() => {
    if (refreshInterval) clearInterval(refreshInterval);
    if (tickInterval) clearInterval(tickInterval);
  });

  // ── data ───────────────────────────────────────────────────────
  async function initPage() {
    pageLoading = true;
    pageError = '';
    notConfigured = false;
    forbidden = false;
    setupRequired = false;
    try {
      const setup = await getSetupStatus();
      if (setup.required) { setupRequired = true; return; }
      principal = principalFromAuth(await getMe());
      await loadServerStatus();
    } catch (caught) {
      if (caught instanceof ApiError) {
        if (caught.status === 503) { notConfigured = true; return; }
        if (caught.status === 403) { forbidden = true; return; }
      }
      pageError = caught instanceof Error ? caught.message : 'Unable to load sessions page.';
    } finally {
      pageLoading = false;
    }
  }

  async function loadServerStatus() {
    statusLoading = true;
    statusError = '';
    try {
      serverStatus = await getServerStatus();
    } catch (caught) {
      statusError = caught instanceof Error ? caught.message : 'Status unavailable';
    } finally {
      statusLoading = false;
    }
  }

  async function refreshData() {
    // Only refresh if no dialog is open
    if (pendingSessionAction || disconnectUserTarget) return;
    await loadServerStatus();
    refreshSignal++;
    lastRefreshed = new Date();
  }

  async function handleManualRefresh() {
    await refreshData();
  }

  function handleTabChange(tabId: string) {
    activeTab = tabId as TabId;
    window.location.hash = tabId;
  }

  // ── session actions (disconnect/terminate by ID) ───────────────
  function confirmDisconnectSession(session: OcservSession) {
    pendingSessionAction = { type: 'disconnect', sessionId: session.id, username: session.username };
    sessionActionError = '';
  }

  function confirmTerminateSession(session: OcservSession) {
    pendingSessionAction = { type: 'terminate', sessionId: session.id, username: session.username };
    sessionActionError = '';
  }

  function cancelSessionAction() {
    pendingSessionAction = null;
    sessionActionError = '';
  }

  async function executeSessionAction() {
    if (!pendingSessionAction || sessionActionSubmitting) return;
    sessionActionSubmitting = true;
    sessionActionError = '';
    const { type, sessionId, username } = pendingSessionAction;
    try {
      if (type === 'disconnect') {
        await disconnectOcservSession(sessionId);
      } else {
        await terminateOcservSession(sessionId);
      }
      const verb = type === 'disconnect' ? 'disconnected' : 'terminated';
      successBanner = `Session for ${username} ${verb}.`;
      pendingSessionAction = null;
      // Trigger refresh to update table
      refreshSignal++;
    } catch (caught) {
      sessionActionError = caught instanceof Error ? caught.message : `Failed to ${type} session.`;
    } finally {
      sessionActionSubmitting = false;
    }
  }

  // ── disconnect user (all sessions) ─────────────────────────────
  function confirmDisconnectUser(username: string) {
    disconnectUserTarget = username;
    disconnectUserError = '';
  }

  function cancelDisconnectUser() {
    disconnectUserTarget = null;
    disconnectUserError = '';
  }

  async function executeDisconnectUser() {
    if (!disconnectUserTarget || disconnectUserSubmitting) return;
    disconnectUserSubmitting = true;
    disconnectUserError = '';
    try {
      await disconnectUser(disconnectUserTarget);
      successBanner = `All sessions for ${disconnectUserTarget} disconnected.`;
      disconnectUserTarget = null;
      refreshSignal++;
    } catch (caught) {
      disconnectUserError = caught instanceof Error ? caught.message : 'Failed to disconnect user.';
    } finally {
      disconnectUserSubmitting = false;
    }
  }
</script>

<!-- loading -->
{#if pageLoading}
  <div class="flex items-center justify-center py-16">
    <div class="text-center">
      <div class="w-8 h-8 border-2 border-slate-200 border-t-slate-500 rounded-full animate-spin mx-auto mb-3"></div>
      <p class="text-slate-500 text-sm">Loading sessions...</p>
    </div>
  </div>

<!-- setup required -->
{:else if setupRequired}
  <div class="bg-amber-50 border border-amber-200 rounded-lg p-6">
    <h3 class="text-lg font-semibold text-amber-800">Initial setup required</h3>
    <p class="text-amber-700 text-sm mt-1">Create the first owner before viewing active sessions.</p>
    <a class="inline-block mt-4 px-4 py-2 bg-amber-600 text-white text-sm font-medium rounded-md hover:bg-amber-700 transition-colors" href="/setup">
      Set up owner
    </a>
  </div>

<!-- not configured (503) -->
{:else if notConfigured}
  <EmptyState
    icon={AlertTriangle}
    title="ocserv is not available"
    description="ocserv is not available. Is the ocserv service running?"
  />

<!-- forbidden (403) -->
{:else if forbidden}
  <EmptyState
    icon={AlertTriangle}
    title="Access denied"
    description="Your role cannot view sessions."
  />

<!-- not authenticated -->
{:else if !principal}
  <div class="bg-slate-50 border border-slate-200 rounded-lg p-6">
    <h3 class="text-lg font-semibold text-slate-700">Sign in required</h3>
    <p class="text-slate-500 text-sm mt-1">Active session data requires authentication.</p>
    <a class="inline-block mt-4 px-4 py-2 bg-slate-700 text-white text-sm font-medium rounded-md hover:bg-slate-800 transition-colors" href="/login">
      Go to login
    </a>
    {#if pageError}
      <p class="mt-3 text-sm text-red-600">{pageError}</p>
    {/if}
  </div>

<!-- main view -->
{:else}
  <div class="space-y-4">
    <!-- Banners -->
    {#if errorBanner}
      <div class="flex items-center justify-between px-4 py-3 rounded-lg text-sm bg-red-50 text-red-800" role="alert">
        <span>{errorBanner}</span>
        <button type="button" onclick={() => (errorBanner = '')} class="ml-4 text-red-600 hover:text-red-800" aria-label="Dismiss">
          <X class="w-4 h-4" />
        </button>
      </div>
    {/if}
    {#if successBanner}
      <div class="flex items-center justify-between px-4 py-3 rounded-lg text-sm bg-green-50 text-green-800" role="status">
        <span>{successBanner}</span>
        <button type="button" onclick={() => (successBanner = '')} class="ml-4 text-green-600 hover:text-green-800" aria-label="Dismiss">
          <X class="w-4 h-4" />
        </button>
      </div>
    {/if}

    <!-- Page header -->
    <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-3">
      <h1 class="text-2xl font-bold text-slate-800">VPN Sessions</h1>
      <div class="flex items-center gap-3">
        <span class="text-sm text-slate-500">Updated {updatedAgoText}</span>
        <button
          type="button"
          onclick={handleManualRefresh}
          class="inline-flex items-center gap-1.5 px-3 py-1.5 text-sm font-medium rounded-md bg-white border border-slate-200 text-slate-600 hover:bg-slate-50 transition-colors"
          aria-label="Refresh"
        >
          <RefreshCw class="w-4 h-4" />
          Refresh
        </button>
      </div>
    </div>

    <!-- Status cards -->
    <SessionStatusCards status={serverStatus} loading={statusLoading} error={statusError} />

    <!-- Tabs -->
    <SessionTabs {tabs} {activeTab} onTabChange={handleTabChange} />

    <!-- Tab content -->
    <div role="tabpanel" id="tabpanel-{activeTab}" aria-labelledby="tab-{activeTab}">
      {#if activeTab === 'active'}
        <ActiveSessionsTab
          {isOwner}
          {refreshSignal}
          onDisconnectSession={confirmDisconnectSession}
          onTerminateSession={confirmTerminateSession}
          onDisconnectUser={confirmDisconnectUser}
        />
      {:else if activeTab === 'all'}
        <AllSessionsTab {refreshSignal} />
      {:else if activeTab === 'events'}
        <EventsTab {refreshSignal} />
      {:else if activeTab === 'routes'}
        <RoutesTab {refreshSignal} />
      {:else if activeTab === 'bans'}
        <BansTab {refreshSignal} {isOwner} />
      {/if}
    </div>
  </div>
{/if}

<!-- Session disconnect/terminate modal -->
{#if pendingSessionAction !== null}
  <div
    class="fixed inset-0 bg-black/50 flex items-center justify-center z-50"
    role="dialog"
    aria-modal="true"
    aria-labelledby="session-action-title"
  >
    <div class="bg-white rounded-lg shadow-xl max-w-md w-full mx-4 p-6">
      <h2 id="session-action-title" class="text-lg font-semibold text-slate-800">
        {pendingSessionAction.type === 'disconnect' ? 'Disconnect session?' : 'Terminate session?'}
      </h2>
      <p class="text-sm text-slate-600 mt-2">
        {#if pendingSessionAction.type === 'disconnect'}
          This will drop the current connection for <strong>{pendingSessionAction.username}</strong>. The user may reconnect if their session cookie is still valid.
        {:else}
          This will drop the connection and invalidate the session cookie for <strong>{pendingSessionAction.username}</strong>. The user must log in again.
        {/if}
      </p>
      {#if sessionActionError}
        <div class="mt-3 px-3 py-2 bg-red-50 rounded text-sm text-red-700" role="alert">{sessionActionError}</div>
      {/if}
      <div class="flex items-center justify-end gap-3 mt-6">
        <button
          type="button"
          onclick={cancelSessionAction}
          disabled={sessionActionSubmitting}
          class="px-4 py-2 text-sm font-medium rounded-md border border-slate-200 text-slate-600 bg-white hover:bg-slate-50 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
        >
          Cancel
        </button>
        <button
          type="button"
          onclick={executeSessionAction}
          disabled={sessionActionSubmitting}
          class={pendingSessionAction.type === 'disconnect'
            ? 'px-4 py-2 text-sm font-medium rounded-md bg-orange-600 text-white hover:bg-orange-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors'
            : 'px-4 py-2 text-sm font-medium rounded-md bg-red-600 text-white hover:bg-red-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors'}
        >
          {#if sessionActionSubmitting}
            {pendingSessionAction.type === 'disconnect' ? 'Disconnecting...' : 'Terminating...'}
          {:else}
            {pendingSessionAction.type === 'disconnect' ? 'Disconnect' : 'Terminate'}
          {/if}
        </button>
      </div>
    </div>
  </div>
{/if}

<!-- Disconnect user dialog -->
{#if disconnectUserTarget}
  <DisconnectUserDialog
    username={disconnectUserTarget}
    submitting={disconnectUserSubmitting}
    error={disconnectUserError}
    onConfirm={executeDisconnectUser}
    onCancel={cancelDisconnectUser}
  />
{/if}
```

- [ ] **Step 2: Verify TypeScript compiles**

Run: `cd /opt/ocserv_panel/frontend && npx svelte-check --threshold error 2>&1 | tail -10`
Expected: No errors

- [ ] **Step 3: Verify dev build compiles**

Run: `cd /opt/ocserv_panel/frontend && npm run build 2>&1 | tail -10`
Expected: Build succeeds

- [ ] **Step 4: Commit**

```bash
git add frontend/src/routes/sessions/+page.svelte
git commit -m "feat(sessions): rewrite page as tabbed orchestrator with status cards"
```

---

## Task 13: Fix Compilation Issues and Final Verification

**Files:**
- Modify: any files with TypeScript or Svelte errors

- [ ] **Step 1: Run svelte-check and fix any errors**

Run: `cd /opt/ocserv_panel/frontend && npx svelte-check --threshold error 2>&1`

Fix any TypeScript errors found. Common issues to watch for:
- Svelte 5 shorthand prop syntax: `{userConnection}={selectedUserConn}` should be `userConnection={selectedUserConn}`
- Missing exports or type mismatches
- lucide-svelte icon names (verify `Route` exists, else use `Navigation`)

- [ ] **Step 2: Run the full build**

Run: `cd /opt/ocserv_panel/frontend && npm run build 2>&1`
Expected: Build succeeds with no errors

- [ ] **Step 3: Verify backend still compiles**

Run: `cd /opt/ocserv_panel/backend && go build ./... 2>&1`
Expected: No errors

- [ ] **Step 4: Run backend tests**

Run: `cd /opt/ocserv_panel/backend && go test ./... 2>&1 | tail -20`
Expected: All tests pass

- [ ] **Step 5: Final commit for any fixes**

```bash
git add -A
git commit -m "fix(sessions): resolve compilation issues in tabbed sessions page"
```

---

## Summary of Changes

| Component | Purpose |
|-----------|---------|
| API types/functions | 9 new types + 9 API functions matching backend endpoints |
| SessionStatusCards | 4 server stat cards (status, sessions, auth, traffic) |
| SessionTabs | Tab navigation with URL hash sync and badge counts |
| ActiveSessionsTab | Existing table extracted with sorting, pagination, drawer |
| AllSessionsTab | Full session inventory with status badges |
| EventsTab | Connection events table |
| RoutesTab | User-provided routes table |
| BansTab | Ban list + ban points + unban action |
| SessionDetailDrawer | Side panel with new Security + Network Policy sections |
| DisconnectUserDialog | Confirm dialog for disconnect-all-user-sessions |
| UnbanConfirmDialog | Confirm dialog for unbanning an IP |
| +page.svelte | Thin orchestrator: status cards + tabs + delegate |
