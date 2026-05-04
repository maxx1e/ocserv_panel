<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import EmptyState from '$lib/components/ui/EmptyState.svelte';
  import CardTable from '$lib/components/ui/CardTable.svelte';
  import PaginatedTable from '$lib/components/ui/PaginatedTable.svelte';
  import { RefreshCw, X, Link, AlertTriangle } from 'lucide-svelte';
  import { formatRelativeTime, formatAbsoluteTime } from '$lib/utils/format';
  import { paginate } from '$lib/utils/pagination';
  import {
    getOcservSessions,
    getOcservSessionDetail,
    disconnectOcservSession,
    terminateOcservSession,
    getMe,
    getSetupStatus,
    principalFromAuth,
    hasOwnerRole,
    ApiError,
    type OcservSession,
    type OcservSessionDetail,
    type Principal
  } from '$lib/api';

  const PAGE_SIZE = 20;

  // ── page state ────────────────────────────────────────────────────────────

  let loading = $state(true);
  let error = $state('');
  let notConfigured = $state(false);
  let forbidden = $state(false);
  let setupRequired = $state(false);
  let principal = $state<Principal | null>(null);
  let sessions = $state<OcservSession[]>([]);
  let sessionsPage = $state(1);
  let lastRefreshed = $state(new Date());
  let refreshInterval: ReturnType<typeof setInterval> | null = null;

  // ── sorting ───────────────────────────────────────────────────────────────

  let sortColumn = $state<'username' | 'connected' | 'duration'>('connected');
  let sortDirection = $state<'asc' | 'desc'>('asc');

  // ── detail panel ─────────────────────────────────────────────────────────

  let selectedSessionId = $state<string | null>(null);
  let selectedDetail = $state<OcservSessionDetail | null>(null);
  let detailLoading = $state(false);
  let detailError = $state('');
  let sessionGone = $state(false);

  // ── banners ───────────────────────────────────────────────────────────────

  let successBanner = $state('');
  let sessionGoneNotice = $state('');

  // ── confirm modal ─────────────────────────────────────────────────────────

  type PendingAction = { type: 'disconnect' | 'terminate'; sessionId: string; username: string } | null;
  let pendingAction = $state<PendingAction>(null);
  let actionError = $state('');
  let actionSubmitting = $state(false);

  // ── derived ───────────────────────────────────────────────────────────────

  const isOwner = $derived(hasOwnerRole(principal));

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

  const pagedSessions = $derived(paginate(sortedSessions, sessionsPage, PAGE_SIZE));

  const totalRx = $derived(sessions.reduce((sum, s) => sum + (s.rxBytes ?? 0), 0));
  const totalTx = $derived(sessions.reduce((sum, s) => sum + (s.txBytes ?? 0), 0));

  const updatedAgoText = $derived((() => {
    const diffSec = Math.floor((Date.now() - lastRefreshed.getTime()) / 1000);
    if (diffSec < 5) return 'just now';
    return `${diffSec}s ago`;
  })());

  // ── helpers ───────────────────────────────────────────────────────────────

  function formatBytes(bytes: number): string {
    if (bytes === 0) return '0 B';
    const k = 1024;
    const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return `${(bytes / Math.pow(k, i)).toFixed(1)} ${sizes[i]}`;
  }

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

  function formatDurationSecs(secs: number): string {
    if (!secs || secs < 0) return '—';
    const h = Math.floor(secs / 3600);
    const m = Math.floor((secs % 3600) / 60);
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
    sessionsPage = 1;
  }

  function sortIcon(column: 'username' | 'connected' | 'duration'): string {
    if (sortColumn !== column) return ' ↕';
    return sortDirection === 'asc' ? ' ↑' : ' ↓';
  }

  // ── lifecycle ─────────────────────────────────────────────────────────────

  onMount(() => {
    void loadSessions();
    refreshInterval = setInterval(() => void loadSessions(), 30_000);
  });

  onDestroy(() => {
    if (refreshInterval) clearInterval(refreshInterval);
  });

  // ── data loading ──────────────────────────────────────────────────────────

  async function loadSessions() {
    if (!loading) {
      // silent refresh — don't reset the whole page
      try {
        const data = await getOcservSessions();
        sessions = data;
        lastRefreshed = new Date();
        // If the selected session disappeared, surface a notice
        if (selectedSessionId && !data.some(s => s.id === selectedSessionId)) {
          const gone = sessions.find(s => s.id === selectedSessionId);
          sessionGoneNotice = gone ? gone.username : selectedSessionId ?? '';
          selectedSessionId = null;
          selectedDetail = null;
          sessionGone = false;
        }
      } catch {
        // Silent refresh failures do not clear the current view
      }
      return;
    }

    loading = true;
    error = '';
    notConfigured = false;
    forbidden = false;
    setupRequired = false;

    try {
      const setup = await getSetupStatus();
      setupRequired = setup.required;
      if (setup.required) return;

      principal = principalFromAuth(await getMe());
      const data = await getOcservSessions();
      sessions = data;
      lastRefreshed = new Date();
    } catch (caught) {
      if (caught instanceof ApiError) {
        if (caught.status === 503) {
          notConfigured = true;
          return;
        }
        if (caught.status === 403) {
          forbidden = true;
          return;
        }
      }
      error = caught instanceof Error ? caught.message : 'Unable to load sessions.';
    } finally {
      loading = false;
    }
  }

  async function loadSessionDetail(id: string) {
    selectedDetail = null;
    detailError = '';
    sessionGone = false;
    detailLoading = true;
    try {
      selectedDetail = await getOcservSessionDetail(id);
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
    sessionsPage = pagedSessions.page; // keep same page
    void loadSessionDetail(session.id);
  }

  function closePanel() {
    selectedSessionId = null;
    selectedDetail = null;
    detailError = '';
    sessionGone = false;
  }

  // ── actions ───────────────────────────────────────────────────────────────

  function confirmDisconnect(session: OcservSession) {
    pendingAction = { type: 'disconnect', sessionId: session.id, username: session.username };
    actionError = '';
  }

  function confirmTerminate(session: OcservSession) {
    pendingAction = { type: 'terminate', sessionId: session.id, username: session.username };
    actionError = '';
  }

  function cancelAction() {
    pendingAction = null;
    actionError = '';
  }

  async function executeAction() {
    if (!pendingAction || actionSubmitting) return;
    actionError = '';
    actionSubmitting = true;
    const { type, sessionId, username } = pendingAction;
    try {
      if (type === 'disconnect') {
        await disconnectOcservSession(sessionId);
      } else {
        await terminateOcservSession(sessionId);
      }
      // Remove session from local list
      sessions = sessions.filter(s => s.id !== sessionId);
      // Close panel if it was for this session
      if (selectedSessionId === sessionId) {
        closePanel();
      }
      const verb = type === 'disconnect' ? 'disconnected' : 'terminated';
      successBanner = `Session for ${username} ${verb}.`;
      pendingAction = null;
    } catch (caught) {
      actionError = caught instanceof Error ? caught.message : `Failed to ${type} session.`;
    } finally {
      actionSubmitting = false;
    }
  }

  async function handleRefresh() {
    lastRefreshed = new Date(); // reset timer
    try {
      const data = await getOcservSessions();
      sessions = data;
      lastRefreshed = new Date();
    } catch (caught) {
      error = caught instanceof Error ? caught.message : 'Refresh failed.';
    }
  }
</script>

<!-- ── loading ── -->
{#if loading}
  <div class="flex items-center justify-center py-16">
    <div class="text-center">
      <div class="w-8 h-8 border-2 border-slate-200 border-t-slate-500 rounded-full animate-spin mx-auto mb-3"></div>
      <p class="text-slate-500 text-sm">Loading sessions…</p>
    </div>
  </div>

<!-- ── setup required ── -->
{:else if setupRequired}
  <div class="bg-amber-50 border border-amber-200 rounded-lg p-6">
    <h3 class="text-lg font-semibold text-amber-800">Initial setup required</h3>
    <p class="text-amber-700 text-sm mt-1">Create the first owner before viewing active sessions.</p>
    <a class="inline-block mt-4 px-4 py-2 bg-amber-600 text-white text-sm font-medium rounded-md hover:bg-amber-700 transition-colors" href="/setup">
      Set up owner
    </a>
  </div>

<!-- ── not configured (503) ── -->
{:else if notConfigured}
  <EmptyState
    icon={AlertTriangle}
    title="ocserv is not available"
    description="ocserv is not available. Is the ocserv service running?"
  />

<!-- ── forbidden (403) ── -->
{:else if forbidden}
  <EmptyState
    icon={AlertTriangle}
    title="Access denied"
    description="Your role cannot view sessions."
  />

<!-- ── not authenticated ── -->
{:else if !principal}
  <div class="bg-slate-50 border border-slate-200 rounded-lg p-6">
    <h3 class="text-lg font-semibold text-slate-700">Sign in required</h3>
    <p class="text-slate-500 text-sm mt-1">Active session data requires a backend-confirmed session.</p>
    <a class="inline-block mt-4 px-4 py-2 bg-slate-700 text-white text-sm font-medium rounded-md hover:bg-slate-800 transition-colors" href="/login">
      Go to login
    </a>
    {#if error}
      <p class="mt-3 text-sm text-red-600">{error}</p>
    {/if}
  </div>

<!-- ── main view ── -->
{:else}
  <div class="space-y-4">

    <!-- Error banner -->
    {#if error}
      <div class="flex items-center justify-between px-4 py-3 rounded-lg text-sm bg-red-50 text-red-800" role="alert">
        <span>{error}</span>
        <button type="button" onclick={() => (error = '')} class="ml-4 text-red-600 hover:text-red-800" aria-label="Dismiss">
          <X class="w-4 h-4" />
        </button>
      </div>
    {/if}

    <!-- Success banner -->
    {#if successBanner}
      <div class="flex items-center justify-between px-4 py-3 rounded-lg text-sm bg-green-50 text-green-800" role="status">
        <span>{successBanner}</span>
        <button type="button" onclick={() => (successBanner = '')} class="ml-4 text-green-600 hover:text-green-800" aria-label="Dismiss">
          <X class="w-4 h-4" />
        </button>
      </div>
    {/if}

    <!-- Session-gone notice -->
    {#if sessionGoneNotice}
      <div class="flex items-center justify-between px-4 py-3 rounded-lg text-sm bg-yellow-50 text-yellow-800" role="status">
        <span>Session for <strong>{sessionGoneNotice}</strong> is no longer active and was closed.</span>
        <button type="button" onclick={() => (sessionGoneNotice = '')} class="ml-4 text-yellow-600 hover:text-yellow-800" aria-label="Dismiss">
          <X class="w-4 h-4" />
        </button>
      </div>
    {/if}

    <!-- Page header -->
    <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-3">
      <div class="flex items-center gap-3">
        <h1 class="text-2xl font-bold text-slate-800">Active Sessions</h1>
        <span class="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-blue-100 text-blue-800">
          {sessions.length} connected
        </span>
      </div>
      <div class="flex items-center gap-3">
        <span class="text-sm text-slate-500">Updated {updatedAgoText}</span>
        <button
          type="button"
          onclick={handleRefresh}
          class="inline-flex items-center gap-1.5 px-3 py-1.5 text-sm font-medium rounded-md bg-white border border-slate-200 text-slate-600 hover:bg-slate-50 transition-colors"
          aria-label="Refresh sessions"
        >
          <RefreshCw class="w-4 h-4" />
          Refresh
        </button>
      </div>
    </div>

    <!-- Summary chips -->
    <div class="flex flex-wrap gap-2">
      <span class="inline-flex items-center gap-1.5 px-3 py-1.5 bg-white rounded-lg shadow-sm text-sm text-slate-600">
        <span class="w-2 h-2 rounded-full bg-green-400 inline-block"></span>
        Connected: {sessions.length}
      </span>
      <span class="inline-flex items-center gap-1.5 px-3 py-1.5 bg-white rounded-lg shadow-sm text-sm text-slate-600">
        Total RX: {formatBytes(totalRx)}
      </span>
      <span class="inline-flex items-center gap-1.5 px-3 py-1.5 bg-white rounded-lg shadow-sm text-sm text-slate-600">
        Total TX: {formatBytes(totalTx)}
      </span>
    </div>

    <!-- Main content: table + optional side panel -->
    <div class:flex={selectedSessionId !== null} class:gap-4={selectedSessionId !== null} class:items-start={selectedSessionId !== null}>

      <!-- Sessions table -->
      <div class:flex-1={selectedSessionId !== null} class:min-w-0={selectedSessionId !== null}>
        <CardTable title="Active sessions">
          <PaginatedTable
            totalPages={pagedSessions.totalPages}
            currentPage={pagedSessions.page}
            onPageChange={(p) => (sessionsPage = p)}
          >
            {#if sessions.length === 0}
              <div class="px-6 py-10 text-center">
                <p class="text-sm text-slate-500">No active sessions</p>
              </div>
            {:else}
              <table class="w-full">
                <thead>
                  <tr>
                    <th
                      class="px-4 py-3 text-left text-xs font-medium text-slate-500 uppercase tracking-wider bg-slate-50 cursor-pointer hover:bg-slate-100"
                      onclick={() => toggleSort('username')}
                    >
                      Username{sortIcon('username')}
                    </th>
                    <th class="px-4 py-3 text-left text-xs font-medium text-slate-500 uppercase tracking-wider bg-slate-50">
                      Session ID
                    </th>
                    <th class="px-4 py-3 text-left text-xs font-medium text-slate-500 uppercase tracking-wider bg-slate-50">
                      VPN IP
                    </th>
                    <th class="px-4 py-3 text-left text-xs font-medium text-slate-500 uppercase tracking-wider bg-slate-50">
                      Client IP
                    </th>
                    <th
                      class="px-4 py-3 text-left text-xs font-medium text-slate-500 uppercase tracking-wider bg-slate-50 cursor-pointer hover:bg-slate-100"
                      onclick={() => toggleSort('connected')}
                    >
                      Connected{sortIcon('connected')}
                    </th>
                    <th
                      class="px-4 py-3 text-left text-xs font-medium text-slate-500 uppercase tracking-wider bg-slate-50 cursor-pointer hover:bg-slate-100"
                      onclick={() => toggleSort('duration')}
                    >
                      Duration{sortIcon('duration')}
                    </th>
                    <th class="px-4 py-3 text-left text-xs font-medium text-slate-500 uppercase tracking-wider bg-slate-50">
                      RX / TX
                    </th>
                    {#if !selectedSessionId}
                      <th class="px-4 py-3 text-left text-xs font-medium text-slate-500 uppercase tracking-wider bg-slate-50">
                        Client
                      </th>
                      <th class="px-4 py-3 text-left text-xs font-medium text-slate-500 uppercase tracking-wider bg-slate-50">
                        Stability
                      </th>
                    {/if}
                    {#if isOwner}
                      <th class="px-4 py-3 text-left text-xs font-medium text-slate-500 uppercase tracking-wider bg-slate-50">
                        Actions
                      </th>
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
                      <td class="px-4 py-3 text-sm text-slate-700 font-medium">
                        {session.username}
                      </td>
                      <td class="px-4 py-3">
                        <span class="font-mono text-xs text-slate-500">{session.id.slice(0, 8)}</span>
                      </td>
                      <td class="px-4 py-3">
                        <span class="font-mono text-xs text-slate-700">{session.vpnAddress || '—'}</span>
                      </td>
                      <td class="px-4 py-3">
                        <span class="font-mono text-xs text-slate-700">{session.remoteAddress || '—'}</span>
                      </td>
                      <td class="px-4 py-3 text-sm text-slate-700" title={formatAbsoluteTime(session.connectedAt)}>
                        {formatRelativeTime(session.connectedAt)}
                      </td>
                      <td class="px-4 py-3 text-sm">
                        {#if isLongRunning(session.connectedAt)}
                          <span class="px-2 py-0.5 rounded text-xs font-medium bg-red-100 text-red-700">
                            {formatDuration(session.connectedAt)}
                          </span>
                        {:else}
                          <span class="text-slate-700">{formatDuration(session.connectedAt)}</span>
                        {/if}
                      </td>
                      <td class="px-4 py-3 text-sm text-slate-700">
                        {formatBytes(session.rxBytes)} / {formatBytes(session.txBytes)}
                      </td>
                      {#if !selectedSessionId}
                        <td class="px-4 py-3 text-sm text-slate-700">
                          {userAgentShort(session.userAgent)}
                        </td>
                        <td class="px-4 py-3 text-sm">
                          {#if session.reconnectCount > 2}
                            <span class="px-2 py-0.5 rounded text-xs font-medium bg-yellow-100 text-yellow-700">
                              ⚠ {session.reconnectCount} reconnects
                            </span>
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
                              onclick={(e) => { e.stopPropagation(); confirmDisconnect(session); }}
                              class="px-2 py-1 text-xs font-medium rounded bg-orange-100 text-orange-700 hover:bg-orange-200 transition-colors"
                            >
                              Disconnect
                            </button>
                            <button
                              type="button"
                              onclick={(e) => { e.stopPropagation(); confirmTerminate(session); }}
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
            {/if}
          </PaginatedTable>
        </CardTable>
      </div>

      <!-- Detail side panel -->
      {#if selectedSessionId !== null}
        {@const panelSession = sessions.find(s => s.id === selectedSessionId)}
        <div class="w-96 shrink-0 bg-white rounded-lg shadow-sm border border-slate-100 overflow-hidden">
          <!-- Panel header -->
          <div class="flex items-center justify-between px-4 py-3 border-b border-slate-100 bg-slate-50">
            <div class="min-w-0">
              <h2 class="text-sm font-semibold text-slate-800 truncate">
                {panelSession?.username ?? 'Session'} — <span class="font-mono font-normal text-slate-500">{selectedSessionId.slice(0, 8)}</span>
              </h2>
            </div>
            <button
              type="button"
              onclick={closePanel}
              class="ml-2 text-slate-400 hover:text-slate-600 transition-colors flex-shrink-0"
              aria-label="Close panel"
            >
              <X class="w-4 h-4" />
            </button>
          </div>

          <!-- Panel body -->
          <div class="px-4 py-3 space-y-1 overflow-y-auto max-h-[calc(100vh-16rem)]">
            {#if detailLoading}
              <div class="py-8 flex items-center justify-center">
                <div class="w-6 h-6 border-2 border-slate-200 border-t-slate-500 rounded-full animate-spin"></div>
              </div>

            {:else if sessionGone}
              <p class="text-sm text-slate-500 py-6 text-center">Session no longer active.</p>

            {:else if detailError}
              <p class="text-sm text-red-600 py-4">{detailError}</p>

            {:else if selectedDetail}
              <!-- Connection Info -->
              <p class="text-xs font-semibold text-slate-500 uppercase tracking-wider mt-4 mb-2">Connection</p>
              <div class="flex justify-between text-sm py-1 border-b border-slate-50">
                <span class="text-slate-500">VPN IPv4</span>
                <span class="text-slate-700 font-medium font-mono text-xs">{selectedDetail.ipv4 || '—'}</span>
              </div>
              <div class="flex justify-between text-sm py-1 border-b border-slate-50">
                <span class="text-slate-500">VPN IPv6</span>
                <span class="text-slate-700 font-medium font-mono text-xs">{selectedDetail.ipv6 || '—'}</span>
              </div>
              <div class="flex justify-between text-sm py-1 border-b border-slate-50">
                <span class="text-slate-500">Client IP</span>
                <span class="text-slate-700 font-medium font-mono text-xs">{selectedDetail.remoteAddress || '—'}</span>
              </div>
              <div class="flex justify-between text-sm py-1 border-b border-slate-50">
                <span class="text-slate-500">Connected</span>
                <span class="text-slate-700 font-medium text-right">{formatAbsoluteTime(selectedDetail.connectedAt)}</span>
              </div>
              <div class="flex justify-between text-sm py-1 border-b border-slate-50">
                <span class="text-slate-500">Duration</span>
                <span class="text-slate-700 font-medium">{formatDurationSecs(selectedDetail.durationSeconds)}</span>
              </div>
              <div class="flex justify-between text-sm py-1 border-b border-slate-50">
                <span class="text-slate-500">Device</span>
                <span class="text-slate-700 font-medium">{selectedDetail.device || '—'}</span>
              </div>

              <!-- Traffic -->
              <p class="text-xs font-semibold text-slate-500 uppercase tracking-wider mt-4 mb-2">Traffic</p>
              <div class="flex justify-between text-sm py-1 border-b border-slate-50">
                <span class="text-slate-500">Received</span>
                <span class="text-slate-700 font-medium">{formatBytes(selectedDetail.rxBytes)}</span>
              </div>
              <div class="flex justify-between text-sm py-1 border-b border-slate-50">
                <span class="text-slate-500">Transmitted</span>
                <span class="text-slate-700 font-medium">{formatBytes(selectedDetail.txBytes)}</span>
              </div>

              <!-- Client -->
              <p class="text-xs font-semibold text-slate-500 uppercase tracking-wider mt-4 mb-2">Client</p>
              <div class="flex justify-between text-sm py-1 border-b border-slate-50">
                <span class="text-slate-500">User Agent</span>
                <span class="text-slate-700 font-medium text-right max-w-[60%] break-words">{selectedDetail.userAgent || '—'}</span>
              </div>
              <div class="flex justify-between text-sm py-1 border-b border-slate-50">
                <span class="text-slate-500">Stability</span>
                {#if selectedDetail.reconnectCount > 2}
                  <span class="px-2 py-0.5 rounded text-xs font-medium bg-yellow-100 text-yellow-700">
                    ⚠ {selectedDetail.reconnectCount} reconnects
                  </span>
                {:else}
                  <span class="text-slate-700 font-medium">Stable</span>
                {/if}
              </div>

              <!-- Routes -->
              <p class="text-xs font-semibold text-slate-500 uppercase tracking-wider mt-4 mb-2">Assigned Routes</p>
              {#if selectedDetail.routes && selectedDetail.routes.length > 0}
                <ul class="space-y-0.5">
                  {#each selectedDetail.routes as route}
                    <li class="text-xs font-mono text-slate-700 py-0.5">{route}</li>
                  {/each}
                </ul>
              {:else}
                <p class="text-xs text-slate-500">None assigned</p>
              {/if}

              <!-- DNS -->
              <p class="text-xs font-semibold text-slate-500 uppercase tracking-wider mt-4 mb-2">Assigned DNS</p>
              {#if selectedDetail.dns && selectedDetail.dns.length > 0}
                <ul class="space-y-0.5">
                  {#each selectedDetail.dns as entry}
                    <li class="text-xs font-mono text-slate-700 py-0.5">{entry}</li>
                  {/each}
                </ul>
              {:else}
                <p class="text-xs text-slate-500">None assigned</p>
              {/if}

              <!-- Panel actions -->
              {#if isOwner && panelSession}
                <div class="flex items-center gap-2 pt-4 pb-2 border-t border-slate-100 mt-4">
                  <button
                    type="button"
                    onclick={() => confirmDisconnect(panelSession)}
                    class="flex-1 px-3 py-2 text-sm font-medium rounded-md bg-orange-100 text-orange-700 hover:bg-orange-200 transition-colors"
                  >
                    Disconnect
                  </button>
                  <button
                    type="button"
                    onclick={() => confirmTerminate(panelSession)}
                    class="flex-1 px-3 py-2 text-sm font-medium rounded-md bg-red-100 text-red-700 hover:bg-red-200 transition-colors"
                  >
                    Terminate
                  </button>
                </div>
              {/if}
            {/if}
          </div>
        </div>
      {/if}
    </div>
  </div>
{/if}

<!-- ── Confirmation modal ── -->
{#if pendingAction !== null}
  <div
    class="fixed inset-0 bg-black/50 flex items-center justify-center z-50"
    role="dialog"
    aria-modal="true"
    aria-labelledby="modal-title"
  >
    <div class="bg-white rounded-lg shadow-xl max-w-md w-full mx-4 p-6">
      <h2 id="modal-title" class="text-lg font-semibold text-slate-800">
        {pendingAction.type === 'disconnect' ? 'Disconnect session?' : 'Terminate session?'}
      </h2>
      <p class="text-sm text-slate-600 mt-2">
        {#if pendingAction.type === 'disconnect'}
          This will drop the current connection for <strong>{pendingAction.username}</strong>. The user may reconnect if their session cookie is still valid.
        {:else}
          This will drop the connection and invalidate the session cookie for <strong>{pendingAction.username}</strong>. The user must log in again.
        {/if}
      </p>

      {#if actionError}
        <div class="mt-3 px-3 py-2 bg-red-50 rounded text-sm text-red-700" role="alert">
          {actionError}
        </div>
      {/if}

      <div class="flex items-center justify-end gap-3 mt-6">
        <button
          type="button"
          onclick={cancelAction}
          disabled={actionSubmitting}
          class="px-4 py-2 text-sm font-medium rounded-md border border-slate-200 text-slate-600 bg-white hover:bg-slate-50 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
        >
          Cancel
        </button>
        <button
          type="button"
          onclick={executeAction}
          disabled={actionSubmitting}
          class={pendingAction.type === 'disconnect'
            ? 'px-4 py-2 text-sm font-medium rounded-md bg-orange-600 text-white hover:bg-orange-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors'
            : 'px-4 py-2 text-sm font-medium rounded-md bg-red-600 text-white hover:bg-red-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors'}
        >
          {#if actionSubmitting}
            {pendingAction.type === 'disconnect' ? 'Disconnecting…' : 'Terminating…'}
          {:else}
            {pendingAction.type === 'disconnect' ? 'Disconnect' : 'Terminate'}
          {/if}
        </button>
      </div>
    </div>
  </div>
{/if}
