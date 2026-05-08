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
              <td class="px-4 py-3 font-mono text-xs text-slate-700">{session.vpnAddress || '—'}</td>
              <td class="px-4 py-3 font-mono text-xs text-slate-700">{session.remoteAddress || '—'}</td>
              <td class="px-4 py-3 text-sm text-slate-700">{formatRelativeTime(session.connectedAt)}</td>
              <td class="px-4 py-3 text-sm text-slate-700">{session.userAgent || '—'}</td>
            </tr>
          {/each}
        </tbody>
      </table>
    </PaginatedTable>
  </CardTable>
{/if}
