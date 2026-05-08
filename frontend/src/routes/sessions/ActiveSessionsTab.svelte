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
    if (sortColumn !== column) return ' ⇕';
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
  <div class={selectedSessionId !== null ? 'flex gap-4 items-start' : ''}>
    <!-- Sessions table -->
    <div class={selectedSessionId !== null ? 'flex-1 min-w-0' : ''}>
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
                        onclick={(e: MouseEvent) => { e.stopPropagation(); navigator.clipboard.writeText(session.id); }}
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
                          onclick={(e: MouseEvent) => { e.stopPropagation(); onDisconnectSession(session); }}
                          class="px-2 py-1 text-xs font-medium rounded bg-orange-100 text-orange-700 hover:bg-orange-200 transition-colors"
                        >
                          Disconnect
                        </button>
                        <button
                          type="button"
                          onclick={(e: MouseEvent) => { e.stopPropagation(); onTerminateSession(session); }}
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
        userConnection={selectedUserConn}
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
