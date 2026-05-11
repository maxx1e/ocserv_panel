<script lang="ts">
  import { onMount } from 'svelte';
  import CardTable from '$lib/components/ui/CardTable.svelte';
  import PaginatedTable from '$lib/components/ui/PaginatedTable.svelte';
  import EmptyState from '$lib/components/ui/EmptyState.svelte';
  import SessionDetailDrawer from './SessionDetailDrawer.svelte';
  import { Wifi } from 'lucide-svelte';
  import { paginate } from '$lib/utils/pagination';
  import {
    getConnectedUsers,
    getUserConnections,
    ApiError,
    type ConnectedUser,
    type UserConnection
  } from '$lib/api';

  type SessionRef = { id: string; username: string };

  type Props = {
    isOwner: boolean;
    refreshSignal: number;
    onDisconnectSession: (session: SessionRef) => void;
    onTerminateSession: (session: SessionRef) => void;
    onDisconnectUser: (username: string) => void;
  };

  let { isOwner, refreshSignal, onDisconnectSession, onTerminateSession, onDisconnectUser }: Props = $props();

  const PAGE_SIZE = 20;

  let users = $state<ConnectedUser[]>([]);
  let loading = $state(true);
  let error = $state('');
  let page = $state(1);
  let lastFetchedSignal = $state(0);

  // Sorting
  type SortColumn = 'username' | 'duration' | 'status';
  let sortColumn = $state<SortColumn>('duration');
  let sortDirection = $state<'asc' | 'desc'>('desc');

  // Detail panel
  let selectedUser = $state<ConnectedUser | null>(null);
  let selectedConnections = $state<UserConnection[]>([]);
  let detailLoading = $state(false);
  let detailError = $state('');

  // Derived
  const sortedUsers = $derived((() => {
    const copy = [...users];
    copy.sort((a, b) => {
      let cmp = 0;
      if (sortColumn === 'username') {
        cmp = a.username.localeCompare(b.username);
      } else if (sortColumn === 'duration') {
        cmp = a.durationSeconds - b.durationSeconds;
      } else if (sortColumn === 'status') {
        cmp = a.status.localeCompare(b.status);
      }
      return sortDirection === 'asc' ? cmp : -cmp;
    });
    return copy;
  })());

  const pagedUsers = $derived(paginate(sortedUsers, page, PAGE_SIZE));

  // Helpers
  function statusBadgeClass(status: string): string {
    if (status === 'connected') return 'bg-green-100 text-green-700';
    if (status === 'authenticated') return 'bg-blue-100 text-blue-700';
    return 'bg-slate-100 text-slate-600';
  }

  function cleanCipher(cipher: string): string {
    if (!cipher) return '-';
    // Remove surrounding parentheses, e.g. "(AES-128-GCM)" -> "AES-128-GCM"
    return cipher.replace(/^\(|\)$/g, '');
  }

  function isLongRunning(seconds: number): boolean {
    return seconds > 24 * 60 * 60;
  }

  function toggleSort(column: SortColumn) {
    if (sortColumn === column) {
      sortDirection = sortDirection === 'asc' ? 'desc' : 'asc';
    } else {
      sortColumn = column;
      sortDirection = column === 'username' ? 'asc' : 'desc';
    }
    page = 1;
  }

  function sortIcon(column: SortColumn): string {
    if (sortColumn !== column) return ' ⇕';
    return sortDirection === 'asc' ? ' ↑' : ' ↓';
  }

  // Data loading
  async function loadUsers() {
    const isFirstLoad = users.length === 0;
    if (isFirstLoad) loading = true;
    error = '';
    try {
      users = await getConnectedUsers();
    } catch (caught) {
      if (caught instanceof ApiError && caught.status === 503) {
        error = 'ocserv is not available.';
      } else {
        error = caught instanceof Error ? caught.message : 'Failed to load connected users.';
      }
    } finally {
      loading = false;
    }
  }

  async function loadUserDetail(user: ConnectedUser) {
    selectedConnections = [];
    detailError = '';
    detailLoading = true;
    try {
      selectedConnections = await getUserConnections(user.username);
    } catch (caught) {
      if (caught instanceof ApiError && caught.status === 404) {
        detailError = 'User no longer connected.';
      } else {
        detailError = caught instanceof Error ? caught.message : 'Unable to load connection detail.';
      }
    } finally {
      detailLoading = false;
    }
  }

  function selectUser(user: ConnectedUser) {
    selectedUser = user;
    void loadUserDetail(user);
  }

  function closePanel() {
    selectedUser = null;
    selectedConnections = [];
    detailError = '';
  }

  onMount(() => { void loadUsers(); });

  $effect(() => {
    if (refreshSignal > lastFetchedSignal) {
      lastFetchedSignal = refreshSignal;
      void loadUsers();
    }
  });
</script>

{#if loading}
  <div class="flex items-center justify-center py-12">
    <div class="w-6 h-6 border-2 border-slate-200 border-t-slate-500 rounded-full animate-spin"></div>
  </div>
{:else if error}
  <div class="px-4 py-3 rounded-lg text-sm bg-red-50 text-red-800 m-4" role="alert">{error}</div>
{:else if users.length === 0}
  <EmptyState
    icon={Wifi}
    title="No active sessions"
    description="No users are currently connected to the VPN."
  />
{:else}
  <div class={selectedUser !== null ? 'flex gap-4 items-start' : ''}>
    <!-- Connected users table -->
    <div class={selectedUser !== null ? 'flex-1 min-w-0' : ''}>
      <CardTable title="Active Users ({users.length})">
        <PaginatedTable
          totalPages={pagedUsers.totalPages}
          currentPage={pagedUsers.page}
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
                <th class="px-4 py-3 text-left text-xs font-medium text-slate-500 uppercase tracking-wider bg-slate-50">VPN IP</th>
                <th class="px-4 py-3 text-left text-xs font-medium text-slate-500 uppercase tracking-wider bg-slate-50">Remote IP</th>
                <th class="px-4 py-3 text-left text-xs font-medium text-slate-500 uppercase tracking-wider bg-slate-50">Device</th>
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
                {#if selectedUser === null}
                  <th class="px-4 py-3 text-left text-xs font-medium text-slate-500 uppercase tracking-wider bg-slate-50">Cipher</th>
                {/if}
                <th
                  class="px-4 py-3 text-left text-xs font-medium text-slate-500 uppercase tracking-wider bg-slate-50 cursor-pointer hover:bg-slate-100 select-none"
                  role="columnheader"
                  aria-sort={sortColumn === 'status' ? (sortDirection === 'asc' ? 'ascending' : 'descending') : 'none'}
                  tabindex="0"
                  onclick={() => toggleSort('status')}
                  onkeydown={(e: KeyboardEvent) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); toggleSort('status'); } }}
                >
                  Status{sortIcon('status')}
                </th>
                {#if isOwner}
                  <th class="px-4 py-3 text-left text-xs font-medium text-slate-500 uppercase tracking-wider bg-slate-50">Actions</th>
                {/if}
              </tr>
            </thead>
            <tbody class="divide-y divide-slate-100">
              {#each pagedUsers.items as user (user.id)}
                <tr
                  class="hover:bg-slate-50 cursor-pointer transition-colors"
                  class:bg-blue-50={selectedUser?.id === user.id}
                  onclick={() => selectUser(user)}
                >
                  <td class="px-4 py-3 text-sm text-slate-700 font-medium">{user.username}</td>
                  <td class="px-4 py-3 font-mono text-xs text-slate-700">{user.vpnIp || '—'}</td>
                  <td class="px-4 py-3 font-mono text-xs text-slate-700">{user.remoteIp || '—'}</td>
                  <td class="px-4 py-3 font-mono text-xs text-slate-600">{user.device || '—'}</td>
                  <td class="px-4 py-3 text-sm">
                    {#if isLongRunning(user.durationSeconds)}
                      <span class="px-2 py-0.5 rounded text-xs font-medium bg-red-100 text-red-700">{user.since}</span>
                    {:else}
                      <span class="text-slate-700">{user.since}</span>
                    {/if}
                  </td>
                  {#if selectedUser === null}
                    <td class="px-4 py-3 text-xs text-slate-600 font-mono">{cleanCipher(user.dtlsCipher)}</td>
                  {/if}
                  <td class="px-4 py-3">
                    <span class="px-2 py-0.5 rounded text-xs font-medium {statusBadgeClass(user.status)}">
                      {user.status}
                    </span>
                  </td>
                  {#if isOwner}
                    <td class="px-4 py-3">
                      <div class="flex items-center gap-1">
                        <button
                          type="button"
                          onclick={(e: MouseEvent) => { e.stopPropagation(); onDisconnectSession({ id: user.id, username: user.username }); }}
                          class="px-2 py-1 text-xs font-medium rounded bg-orange-100 text-orange-700 hover:bg-orange-200 transition-colors"
                        >
                          Disconnect
                        </button>
                        <button
                          type="button"
                          onclick={(e: MouseEvent) => { e.stopPropagation(); onTerminateSession({ id: user.id, username: user.username }); }}
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
    {#if selectedUser !== null}
      <SessionDetailDrawer
        session={{ id: selectedUser.id, username: selectedUser.username, remoteAddress: selectedUser.remoteIp, vpnAddress: selectedUser.vpnIp, connectedAt: undefined, rxBytes: 0, txBytes: 0, userAgent: '', device: selectedUser.device, reconnectCount: 0, state: selectedUser.status || 'connected', groupname: '', vhost: selectedUser.vhost || '' }}
        detail={null}
        userConnection={selectedConnections.length > 0 ? selectedConnections[0] : null}
        loading={detailLoading}
        error={detailError}
        sessionGone={detailError === 'User no longer connected.'}
        {isOwner}
        onClose={closePanel}
        onDisconnectSession={(s) => onDisconnectSession({ id: s.id, username: s.username })}
        onTerminateSession={(s) => onTerminateSession({ id: s.id, username: s.username })}
        {onDisconnectUser}
      />
    {/if}
  </div>
{/if}
