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
            <td class="px-4 py-3 font-mono text-xs text-slate-700">{route.id}</td>
            <td class="px-4 py-3 text-sm text-slate-700 font-medium">{route.username}</td>
            <td class="px-4 py-3 text-sm text-slate-700">{route.vhost}</td>
            <td class="px-4 py-3 font-mono text-xs text-slate-700">{route.device}</td>
            <td class="px-4 py-3 font-mono text-xs text-slate-700">{route.vpnIp}</td>
            <td class="px-4 py-3 font-mono text-xs text-slate-700">{route.route}</td>
          </tr>
        {/each}
      </tbody>
    </table>
  </CardTable>
{/if}
