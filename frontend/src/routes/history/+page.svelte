<script lang="ts">
  import { onDestroy, onMount } from 'svelte';
  import MetricChart from '$lib/components/ui/MetricChart.svelte';
  import { ArrowDown, ArrowUp, Server, Users } from 'lucide-svelte';

  import CardStats from '$lib/components/ui/CardStats.svelte';
  import * as Card from '$lib/components/ui/card/index';
  import { mode } from 'mode-watcher';
  import CardTable from '$lib/components/ui/CardTable.svelte';
  import CardSettings from '$lib/components/ui/CardSettings.svelte';
  import WindowSelector from '$lib/components/ui/WindowSelector.svelte';
  import UnavailableChips from '$lib/components/ui/UnavailableChips.svelte';
  import PaginatedTable from '$lib/components/ui/PaginatedTable.svelte';
  import UserSessionDrawer from '$lib/components/history/UserSessionDrawer.svelte';
  import type { Chip } from '$lib/components/ui/UnavailableChips.svelte';
  import { formatRelativeTime, formatAbsoluteTime } from '$lib/utils/format';
  import { paginate } from '$lib/utils/pagination';

  import {
    getHistoryAnalytics,
    getHistoryUsers,
    getHistoryUserSessions,
    getMe,
    getMonitoringOverview,
    getSettings,
    getSetupStatus,
    hasOwnerRole,
    principalFromAuth,
    purgeHistory,
    updateSetting,
    type HistoricalSession,
    type HistoryAnalytics,
    type MonitoringOverview,
    type MonitoringSeries,
    type MonitoringWindow,
    type Principal,
    type Setting,
    type UserSessionSummary
  } from '$lib/api';
  import { formatBytes, metricByName, seriesByMetric } from '$lib/monitoring';

  const HISTORY_WINDOWS: MonitoringWindow[] = ['24h', '7d', '30d'];
  const HOST_CPU = 'host_cpu_utilization';
  const HOST_MEMORY = 'host_memory_utilization';
  const NET_RX = 'net_interface_rx_bytes';
  const NET_TX = 'net_interface_tx_bytes';
  const VPN_RX = 'vpn_interface_rx_bytes';
  const VPN_TX = 'vpn_interface_tx_bytes';
  const CONNECTION_RATE = 'connection_rate';
  const ACTIVE_SESSIONS = 'active_sessions';
  const CONCURRENT_SESSIONS = 'concurrent_sessions';

  type ChartDefinition = {
    metric: string;
    title: string;
  };

  const chartDefinitions: ChartDefinition[] = [
    { metric: HOST_CPU, title: 'CPU utilization' },
    { metric: HOST_MEMORY, title: 'Memory utilization' },
    { metric: NET_RX, title: 'Network inbound' },
    { metric: NET_TX, title: 'Network outbound' },
    { metric: VPN_RX, title: 'VPN inbound' },
    { metric: VPN_TX, title: 'VPN outbound' },
    { metric: ACTIVE_SESSIONS, title: 'Active sessions' },
    { metric: CONNECTION_RATE, title: 'Connection rate' },
    { metric: CONCURRENT_SESSIONS, title: 'Concurrent sessions' }
  ];

  let loading = $state(true);
  let error = $state('');
  let purgeError = $state('');
  let purgeSuccess = $state('');
  let setupRequired = $state(false);
  let principal = $state<Principal | null>(null);
  let analytics = $state<HistoryAnalytics | null>(null);
  let monitoringWindow = $state<MonitoringWindow>('24h');
  let monitoring = $state<MonitoringOverview | null>(null);
  let monitoringLoading = $state(false);
  let monitoringError = $state('');
  let retentionDays = $state<30 | 90 | 180 | 365>(90);
  let confirmation = $state('');
  let purgeSubmitting = $state(false);
  let monitoringRequestId = 0;
  let refreshInterval: ReturnType<typeof setInterval> | null = null;
  let retentionSetting = $state<string>('365');
  let settingsLoading = $state(false);
  let settingsError = $state('');
  let settingsSuccess = $state('');

  // Per-user table state
  let userSummaries = $state<UserSessionSummary[]>([]);
  let userSummariesLoading = $state(false);
  let userSummariesError = $state('');
  let userSummariesPage = $state(1);

  // Drawer state
  let drawerUsername = $state<string | null>(null);
  let drawerSessions = $state<HistoricalSession[]>([]);
  let drawerLoading = $state(false);

  const isOwner = $derived(hasOwnerRole(principal));
  const sampleWarning = $derived(sampleDataWarning(analytics));
  const totals = $derived(calculateTotals(analytics?.summary));
  const purgeEnabled = $derived(isOwner && confirmation === 'PURGE' && !purgeSubmitting);
  const pagedUserSummaries = $derived(paginate(userSummaries, userSummariesPage, 10));
  const unavailableChips = $derived<Chip[]>(
    chartDefinitions
      .filter(chart => !shouldShowChart(chart.metric))
      .map(chart => ({ name: chart.title, reason: unavailableReason(chart.metric) }))
  );

  onMount(() => {
    void loadHistory();
    refreshInterval = setInterval(() => {
      if (document.visibilityState === 'visible') {
        void reloadMonitoring();
        void reloadUserSummaries();
      }
    }, 15_000);
  });

  onDestroy(() => {
    if (refreshInterval) {
      clearInterval(refreshInterval);
    }
  });

  async function loadHistory() {
    loading = true;
    error = '';
    try {
      const setup = await getSetupStatus();
      setupRequired = setup.required;
      if (setup.required) return;

      principal = principalFromAuth(await getMe());
      analytics = await getHistoryAnalytics();
      void reloadMonitoring(monitoringWindow);
      void reloadUserSummaries(monitoringWindow);
      void loadSettings();
    } catch (caught) {
      error = caught instanceof Error ? caught.message : 'Unable to load historical analytics.';
      principal = null;
    } finally {
      loading = false;
    }
  }

  async function loadSettings() {
    settingsLoading = true;
    try {
      const response = await getSettings();
      const retention = response.settings.find((s: Setting) => s.key === 'history.retention_days');
      if (retention) {
        retentionSetting = retention.value;
      }
    } catch {
      // Settings are optional
    } finally {
      settingsLoading = false;
    }
  }

  async function handleSaveRetention(event: SubmitEvent) {
    event.preventDefault();
    settingsError = '';
    settingsSuccess = '';
    try {
      await updateSetting('history.retention_days', retentionSetting);
      settingsSuccess = 'Retention setting saved.';
    } catch (caught) {
      settingsError = caught instanceof Error ? caught.message : 'Failed to save retention setting.';
    }
  }

  async function reloadMonitoring(window: MonitoringWindow = monitoringWindow) {
    const requestId = ++monitoringRequestId;
    monitoringLoading = true;
    monitoringError = '';
    try {
      const overview = await getMonitoringOverview(window);
      if (!isLatestMonitoringRequest(requestId, window)) return;
      monitoring = overview;
    } catch (caught) {
      if (!isLatestMonitoringRequest(requestId, window)) return;
      monitoringError = caught instanceof Error ? caught.message : 'Unable to load monitoring overview.';
      monitoring = null;
    } finally {
      if (isLatestMonitoringRequest(requestId, window)) {
        monitoringLoading = false;
      }
    }
  }

  async function reloadUserSummaries(window: MonitoringWindow = monitoringWindow) {
    userSummariesPage = 1;
    userSummariesLoading = true;
    userSummariesError = '';
    try {
      userSummaries = await getHistoryUsers(window);
    } catch (caught) {
      userSummariesError = caught instanceof Error ? caught.message : 'Unable to load user session summaries.';
      userSummaries = [];
    } finally {
      userSummariesLoading = false;
    }
  }

  function isLatestMonitoringRequest(requestId: number, window: MonitoringWindow): boolean {
    return requestId === monitoringRequestId && window === monitoringWindow;
  }

  function handleWindowChange(nextWindow: string) {
    const window = nextWindow as MonitoringWindow;
    if (!HISTORY_WINDOWS.includes(window) || window === monitoringWindow) return;
    monitoringWindow = window;
    void reloadMonitoring(window);
    void reloadUserSummaries(window);
  }

  async function handleUserRowClick(username: string) {
    drawerUsername = username;
    drawerSessions = [];
    drawerLoading = true;
    try {
      drawerSessions = await getHistoryUserSessions(username, monitoringWindow);
    } catch {
      // 404 means no sessions in window — show empty drawer rather than error
      drawerSessions = [];
    } finally {
      drawerLoading = false;
    }
  }

  function closeDrawer() {
    drawerUsername = null;
    drawerSessions = [];
  }

  function metricSeries(metric: string): MonitoringSeries | undefined {
    return seriesByMetric(monitoring, metric);
  }

  function unavailableReason(metric: string): string {
    const catalogMetric = metricByName(monitoring?.catalog, metric);
    if (catalogMetric?.reason) return catalogMetric.reason;
    if (catalogMetric?.state === 'unavailable') return 'The backend reports this metric as unavailable.';
    return 'Metric data has not been collected for this window yet.';
  }

  function shouldShowChart(metric: string): boolean {
    if (metricByName(monitoring?.catalog, metric)?.state === 'unavailable') return false;
    return chartData(metric).length > 0;
  }

  async function handlePurge(event: SubmitEvent) {
    event.preventDefault();
    purgeError = '';
    purgeSuccess = '';
    if (!purgeEnabled) return;
    purgeSubmitting = true;
    try {
      await purgeHistory(retentionDays, 'PURGE');
      confirmation = '';
      purgeSuccess = 'History purge completed. Analytics were refreshed.';
      await loadHistory();
    } catch (caught) {
      purgeError = caught instanceof Error ? caught.message : 'History purge failed.';
    } finally {
      purgeSubmitting = false;
    }
  }

  function sampleDataWarning(value: HistoryAnalytics | null): string {
    const sampleData = value?.sampleData;
    if (typeof sampleData === 'boolean') return sampleData ? 'Sample historical sessions are present.' : '';
    if (sampleData?.enabled || sampleData?.present || Number(sampleData?.sessionCount ?? 0) > 0) {
      return sampleData?.message ?? 'Sample historical sessions are present.';
    }
    return '';
  }

  function calculateTotals(summary: Record<string, unknown> | undefined) {
    const sessionCount = numberFrom(summary?.sessionCount) ?? numberFrom(summary?.totalSessions) ?? 0;
    const rxBytes = numberFrom(summary?.rxBytes) ?? numberFrom(summary?.bytesIn) ?? 0;
    const txBytes = numberFrom(summary?.txBytes) ?? numberFrom(summary?.bytesOut) ?? 0;
    return { sessionCount, rxBytes, txBytes, totalBytes: rxBytes + txBytes };
  }

  function numberFrom(value: unknown): number | null {
    return typeof value === 'number' && Number.isFinite(value) ? value : null;
  }

  function chartData(metric: string): { timestamp: Date; value: number }[] {
    const series = metricSeries(metric);
    if (!series) return [];
    return series.points
      .filter(p => Number.isFinite(Date.parse(p.timestamp)) && Number.isFinite(p.value))
      .map(p => ({ timestamp: new Date(p.timestamp), value: p.value }))
      .sort((a, b) => a.timestamp.getTime() - b.timestamp.getTime());
  }

  function formatDuration(secs: number): string {
    if (secs === 0) return '—';
    const d = Math.floor(secs / 86400);
    const h = Math.floor((secs % 86400) / 3600);
    const m = Math.floor((secs % 3600) / 60);
    if (d > 0) return `${d}d ${h}h ${m}m`;
    if (h > 0) return `${h}h ${m}m`;
    return `${m}m`;
  }
</script>

{#if loading}
  <div class="flex items-center justify-center py-16">
    <div class="text-center">
      <div class="w-8 h-8 border-2 border-slate-200 border-t-slate-500 rounded-full animate-spin mx-auto mb-3"></div>
      <p class="text-slate-500 text-sm">Loading history…</p>
    </div>
  </div>

{:else if setupRequired}
  <div class="bg-amber-50 border border-amber-200 rounded-lg p-6">
    <h3 class="text-lg font-semibold text-amber-800">Initial setup required</h3>
    <p class="text-amber-700 text-sm mt-1">Create the first owner before viewing historical analytics.</p>
    <a class="inline-block mt-4 px-4 py-2 bg-amber-600 text-white text-sm font-medium rounded-md hover:bg-amber-700 transition-colors" href="/setup">
      Set up owner
    </a>
  </div>

{:else if !principal}
  <div class="bg-slate-50 border border-slate-200 rounded-lg p-6">
    <h3 class="text-lg font-semibold text-slate-700">Sign in required</h3>
    <p class="text-slate-500 text-sm mt-1">Historical analytics require a backend-confirmed session.</p>
    <a class="inline-block mt-4 px-4 py-2 bg-slate-700 text-white text-sm font-medium rounded-md hover:bg-slate-800 transition-colors" href="/login">
      Go to login
    </a>
    {#if error}
      <p class="mt-3 text-sm text-red-600">{error}</p>
    {/if}
  </div>

{:else if analytics}
  <div class="mb-6">
    <h1 class="text-xl font-bold text-slate-700">History</h1>
    <p class="text-sm text-slate-500 mt-1">Analytics are read from the local backend API and persisted SQLite history.</p>
  </div>

  {#if sampleWarning}
    <div class="bg-amber-50 border border-amber-200 rounded-lg p-4 mb-6">
      <strong class="text-amber-800 text-sm">Sample data warning:</strong>
      <span class="text-amber-700 text-sm ml-1">{sampleWarning}</span>
    </div>
  {/if}

  <!-- Summary stat cards -->
  <div class="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-4 gap-6 mb-8">
    <CardStats
      title="Sessions"
      value={String(totals.sessionCount)}
      icon={Users}
    />
    <CardStats
      title="Received"
      value={formatBytes(totals.rxBytes)}
      icon={ArrowDown}
    />
    <CardStats
      title="Sent"
      value={formatBytes(totals.txBytes)}
      icon={ArrowUp}
    />
    <CardStats
      title="Total traffic"
      value={formatBytes(totals.totalBytes)}
      icon={Server}
    />
  </div>

  <!-- Monitoring section -->
  <section aria-labelledby="history-monitoring-heading" class="mb-8">
    <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-3 mb-1">
      <div>
        <h2 id="history-monitoring-heading" class="text-lg font-semibold text-slate-700">Monitoring history</h2>
      </div>
      <div class="flex items-center gap-3">
        <WindowSelector
          value={monitoringWindow}
          windows={HISTORY_WINDOWS}
          onchange={handleWindowChange}
        />
        <button
          type="button"
          disabled={monitoringLoading}
          onclick={() => reloadMonitoring()}
          class="inline-flex items-center px-3 py-1.5 text-sm font-medium rounded-md bg-white border border-slate-200 text-slate-600 hover:bg-slate-50 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
        >
          {monitoringLoading ? 'Refreshing…' : 'Refresh'}
        </button>
      </div>
    </div>
    <p class="text-sm text-slate-500 mt-1">
      Backend-collected samples for longer history windows. User and ocserv-specific metrics show unavailable states when the backend cannot collect them.
    </p>
    <p class="text-sm text-slate-400 mt-1 mb-6">
      Generated: {formatAbsoluteTime(monitoring?.generatedAt)} · Window: {monitoring?.window ?? monitoringWindow}
    </p>

    {#if monitoringError}
      <div class="bg-red-50 border border-red-200 rounded-lg p-4 mb-6" role="status">
        <strong class="text-red-800 text-sm">Monitoring unavailable</strong>
        <p class="text-red-700 text-sm mt-1">{monitoringError}</p>
      </div>
    {/if}

    {#if monitoringLoading && !monitoring}
      <div class="flex items-center justify-center py-12">
        <div class="text-center">
          <div class="w-6 h-6 border-2 border-slate-200 border-t-slate-500 rounded-full animate-spin mx-auto mb-2"></div>
          <p class="text-slate-500 text-sm">Loading monitoring history…</p>
        </div>
      </div>
    {:else}
      {#key mode.current}
        <div class="grid grid-cols-1 lg:grid-cols-2 gap-4">
          {#each chartDefinitions as chart}
            {#if shouldShowChart(chart.metric)}
              <Card.Root>
                <Card.Header>
                  <div>
                    <Card.Title>{chart.title}</Card.Title>
                    <Card.Description>Last {monitoringWindow}</Card.Description>
                  </div>
                </Card.Header>
                <Card.Content>
                  <MetricChart data={chartData(chart.metric)} metric={chart.metric} height={160} />
                </Card.Content>
              </Card.Root>
            {/if}
          {/each}
        </div>
      {/key}
      <UnavailableChips chips={unavailableChips} />
    {/if}
  </section>

  <!-- User session summaries -->
  <section aria-labelledby="sessions-heading" class="mb-8">
    <div class="flex items-center justify-between mb-4">
      <h2 id="sessions-heading" class="text-lg font-semibold text-slate-700">User sessions</h2>
    </div>

    {#if userSummariesError}
      <div class="flex items-center gap-3 py-4">
        <p class="text-red-600 text-sm">{userSummariesError}</p>
        <button
          class="text-sm text-blue-600 hover:underline"
          onclick={() => reloadUserSummaries(monitoringWindow)}
        >Retry</button>
      </div>
    {/if}

    {#if userSummariesLoading && userSummaries.length === 0}
      <table class="min-w-full divide-y divide-slate-200 text-sm">
        <tbody class="divide-y divide-slate-100">
          {#each Array(5) as _}
            <tr>
              {#each Array(8) as _}
                <td class="px-4 py-3"><div class="h-4 bg-slate-200 rounded animate-pulse w-20"></div></td>
              {/each}
            </tr>
          {/each}
        </tbody>
      </table>
    {:else if userSummaries.length === 0}
      <p class="text-sm text-slate-500">No sessions in this period.</p>
    {:else}
      <CardTable title="User sessions — last {monitoringWindow}">
        <PaginatedTable
          totalPages={pagedUserSummaries.totalPages}
          currentPage={pagedUserSummaries.page}
          onPageChange={(p) => (userSummariesPage = p)}
        >
          <table class="w-full text-sm">
            <thead class="bg-slate-50">
              <tr class="text-left text-slate-500 font-medium text-xs">
                <th class="px-6 py-3">User</th>
                <th class="px-6 py-3">Sessions</th>
                <th class="px-6 py-3">Total time</th>
                <th class="px-6 py-3">Avg download</th>
                <th class="px-6 py-3">Avg upload</th>
                <th class="px-6 py-3">Total received</th>
                <th class="px-6 py-3">Total sent</th>
                <th class="px-6 py-3">Total</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-slate-100">
              {#each pagedUserSummaries.items as row (row.username)}
                <tr
                  class="hover:bg-slate-50 cursor-pointer"
                  role="button"
                  tabindex="0"
                  onclick={() => handleUserRowClick(row.username)}
                  onkeydown={(e) => e.key === 'Enter' && handleUserRowClick(row.username)}
                >
                  <td class="px-6 py-3 text-slate-700 font-medium">{row.username}</td>
                  <td class="px-6 py-3 text-slate-700">{row.sessionCount}</td>
                  <td class="px-6 py-3 text-slate-700">{formatDuration(row.totalDurationSeconds)}</td>
                  <td class="px-6 py-3 text-slate-700">{formatBytes(row.avgBytesIn)}</td>
                  <td class="px-6 py-3 text-slate-700">{formatBytes(row.avgBytesOut)}</td>
                  <td class="px-6 py-3 text-slate-700">{formatBytes(row.totalBytesIn)}</td>
                  <td class="px-6 py-3 text-slate-700">{formatBytes(row.totalBytesOut)}</td>
                  <td class="px-6 py-3 text-slate-700">{formatBytes(row.totalBytesIn + row.totalBytesOut)}</td>
                </tr>
              {/each}
            </tbody>
          </table>
        </PaginatedTable>
      </CardTable>
    {/if}
  </section>

  <!-- Housekeeping -->
  <section class="mb-8">
    <CardSettings
      id="housekeeping"
      title="Housekeeping"
      description="Configure automatic data retention. Old records are purged daily based on the retention period."
    >
      {#if isOwner}
        <form onsubmit={handleSaveRetention} class="space-y-4">
          <label class="block">
            <span class="text-sm font-medium text-slate-700">Auto-delete records older than</span>
            <select bind:value={retentionSetting} class="mt-1 block w-full max-w-xs rounded-md border border-slate-200 bg-white px-3 py-2 text-sm text-slate-700 shadow-sm focus:border-slate-400 focus:outline-none focus:ring-1 focus:ring-slate-400">
              <option value="30">30 days</option>
              <option value="90">90 days</option>
              <option value="180">180 days</option>
              <option value="365">365 days (default)</option>
            </select>
          </label>
          {#if settingsError}
            <p class="text-sm text-red-600">{settingsError}</p>
          {/if}
          {#if settingsSuccess}
            <p class="text-sm text-emerald-600">{settingsSuccess}</p>
          {/if}
          <button
            type="submit"
            disabled={settingsLoading}
            class="inline-flex items-center px-4 py-2 text-sm font-medium rounded-md bg-slate-700 text-white hover:bg-slate-800 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
          >
            {settingsLoading ? 'Saving…' : 'Save retention setting'}
          </button>
        </form>
      {:else}
        <p class="text-sm text-slate-500">Your role can view analytics but cannot change retention settings.</p>
      {/if}
    </CardSettings>
  </section>

  <!-- History retention purge -->
  <section class="mb-8">
    <div class="bg-red-50 border border-red-200 rounded-lg p-6">
      <p class="text-xs font-bold uppercase tracking-wider text-red-600">History retention purge</p>
      <h3 class="text-lg font-semibold text-red-800 mt-1">Purge old session records</h3>
      <p class="text-sm text-red-600 mt-1">Destructive owner-only action. The backend enforces authorization and audit logging.</p>
      {#if isOwner}
        <form onsubmit={handlePurge} class="mt-4 space-y-4">
          <label class="block">
            <span class="text-sm font-medium text-red-700">Keep sessions newer than</span>
            <select bind:value={retentionDays} class="mt-1 block w-full max-w-xs rounded-md border border-red-200 bg-white px-3 py-2 text-sm text-slate-700 shadow-sm focus:border-red-400 focus:outline-none focus:ring-1 focus:ring-red-400">
              <option value={30}>30 days</option>
              <option value={90}>90 days</option>
              <option value={180}>180 days</option>
              <option value={365}>365 days</option>
            </select>
          </label>
          <label class="block">
            <span class="text-sm font-medium text-red-700">Type PURGE to confirm deletion of older history</span>
            <input
              bind:value={confirmation}
              autocomplete="off"
              class="mt-1 block w-full max-w-xs rounded-md border border-red-200 bg-white px-3 py-2 text-sm text-slate-700 shadow-sm placeholder:text-slate-400 focus:border-red-400 focus:outline-none focus:ring-1 focus:ring-red-400"
              placeholder="PURGE"
            />
          </label>
          {#if purgeError}
            <p class="text-sm text-red-600">{purgeError}</p>
          {/if}
          {#if purgeSuccess}
            <p class="text-sm text-emerald-600">{purgeSuccess}</p>
          {/if}
          <button
            type="submit"
            disabled={!purgeEnabled}
            class="inline-flex items-center px-4 py-2 text-sm font-medium rounded-md bg-red-600 text-white hover:bg-red-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
          >
            {purgeSubmitting ? 'Purging…' : 'Purge old history'}
          </button>
        </form>
      {:else}
        <p class="text-sm text-red-600 mt-4">Your role can view analytics but cannot purge historical records.</p>
      {/if}
    </div>
  </section>

  <!-- User session drawer -->
  {#if drawerUsername !== null}
    {#if drawerLoading}
      <div class="fixed inset-0 bg-black/40 z-40 flex items-center justify-center">
        <div class="bg-white rounded-lg p-8 flex flex-col items-center gap-3">
          <div class="w-6 h-6 border-2 border-slate-200 border-t-slate-500 rounded-full animate-spin"></div>
          <p class="text-sm text-slate-500">Loading sessions…</p>
        </div>
      </div>
    {:else}
      <UserSessionDrawer
        username={drawerUsername}
        window={monitoringWindow}
        sessions={drawerSessions}
        onclose={closeDrawer}
      />
    {/if}
  {/if}

{:else}
  <div class="bg-red-50 border border-red-200 rounded-lg p-6">
    <p class="text-red-700 text-sm">{error || 'Historical analytics were not returned.'}</p>
  </div>
{/if}
