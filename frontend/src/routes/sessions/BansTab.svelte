<script lang="ts">
  import { onMount } from 'svelte';
  import CardTable from '$lib/components/ui/CardTable.svelte';
  import EmptyState from '$lib/components/ui/EmptyState.svelte';
  import UnbanConfirmDialog from './UnbanConfirmDialog.svelte';
  import { ShieldBan } from 'lucide-svelte';
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

  // Derived: set of currently banned IPs for badge lookup
  const bannedSet = $derived(new Set(bans.map(b => b.ip)));

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
                <th class="px-4 py-3 text-left text-xs font-medium text-slate-500 uppercase tracking-wider bg-slate-50">Score</th>
                <th class="px-4 py-3 text-left text-xs font-medium text-slate-500 uppercase tracking-wider bg-slate-50">Status</th>
                {#if isOwner}
                  <th class="px-4 py-3 text-left text-xs font-medium text-slate-500 uppercase tracking-wider bg-slate-50">Action</th>
                {/if}
              </tr>
            </thead>
            <tbody class="divide-y divide-slate-100">
              {#each points as entry}
                <tr class="hover:bg-slate-50">
                  <td class="px-4 py-3 text-sm font-mono text-slate-700">{entry.ip}</td>
                  <td class="px-4 py-3 text-sm text-slate-700">{entry.score}</td>
                  <td class="px-4 py-3">
                    {#if bannedSet.has(entry.ip)}
                      <span class="inline-flex px-2 py-0.5 rounded text-xs font-medium bg-red-100 text-red-700">Banned</span>
                    {:else}
                      <span class="inline-flex px-2 py-0.5 rounded text-xs font-medium bg-amber-100 text-amber-700">Warning</span>
                    {/if}
                  </td>
                  {#if isOwner}
                    <td class="px-4 py-3">
                      {#if bannedSet.has(entry.ip)}
                        <button
                          type="button"
                          onclick={() => confirmUnban(entry.ip)}
                          class="px-2 py-1 text-xs font-medium rounded bg-blue-100 text-blue-700 hover:bg-blue-200 transition-colors"
                        >
                          Unban
                        </button>
                      {/if}
                    </td>
                  {/if}
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
