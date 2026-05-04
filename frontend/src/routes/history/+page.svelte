<script lang="ts">
  import { onDestroy, onMount } from 'svelte';
  import MetricChart from '$lib/components/ui/MetricChart.svelte';
  import { ArrowDown, ArrowUp, Server, Users } from 'lucide-svelte';

  import CardStats from '$lib/components/ui/CardStats.svelte';
  import CardChart from '$lib/components/ui/CardChart.svelte';
  import CardTable from '$lib/components/ui/CardTable.svelte';
  import CardSettings from '$lib/components/ui/CardSettings.svelte';
  import WindowSelector from '$lib/components/ui/WindowSelector.svelte';
  import UnavailableChips from '$lib/components/ui/UnavailableChips.svelte';
  import PaginatedTable from '$lib/components/ui/PaginatedTable.svelte';
  import type { Chip } from '$lib/components/ui/UnavailableChips.svelte';
  import { formatRelativeTime, formatAbsoluteTime } from '$lib/utils/format';
  import { paginate } from '$lib/utils/pagination';

  import {
    getHistoryAnalytics,
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
    type Setting
  } from '$lib/api';
  import { formatBytes, metricByName, seriesByMetric } from '$lib/monitoring';

  const HISTORY_WINDOWS: MonitoringWindow[] = ['24h', '7d', '30d'];
  const HOST_CPU = 'host_cpu_utilization';
  const HOST_MEMORY = 'host_memory_utilization';
  const NET_RX = 'net_interface_rx_bytes';
  const NET_TX = 'net_interface_tx_bytes';
  const VPN_RX = 'vpn_interface_rx_bytes';
  const VPN_TX = 'vpn_interface_tx_bytes';
  const USER_RX = 'user_rx_bytes';
  const USER_TX = 'user_tx_bytes';
  const CONNECTION_RATE = 'connection_rate';

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
    { metric: USER_RX, title: 'User inbound' },
    { metric: USER_TX, title: 'User outbound' },
    { metric: CONNECTION_RATE, title: 'Connection rate' }
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
  let sessionsPage = $state(1);

  const isOwner = $derived(hasOwnerRole(principal));
  const sessions = $derived(analytics?.recentSessions ?? analytics?.sessions ?? []);
  const sampleWarning = $derived(sampleDataWarning(analytics));
  const totals = $derived(calculateTotals(sessions, analytics?.summary));
  const purgeEnabled = $derived(isOwner && confirmation === 'PURGE' && !purgeSubmitting);
  const pagedSessions = $derived(paginate(sessions, sessionsPage, 10));
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
      }
    }, 15_000);
  });

  onDestroy(() => {
    if (refreshInterval) {
      clearInterval(refreshInterval);
    }
  });

  async function loadHistory() {
    sessionsPage = 1;
    loading = true;
    error = '';
    try {
      const setup = await getSetupStatus();
      setupRequired = setup.required;
      if (setup.required) return;

      principal = principalFromAuth(await getMe());
      analytics = await getHistoryAnalytics();
      void reloadMonitoring(monitoringWindow);
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
      // Settings are optional — don't block the page
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

  function isLatestMonitoringRequest(requestId: number, window: MonitoringWindow): boolean {
    return requestId === monitoringRequestId && window === monitoringWindow;
  }

  function handleMonitoringWindowChange(nextWindow: string) {
    const window = nextWindow as MonitoringWindow;
    if (!HISTORY_WINDOWS.includes(window) || window === monitoringWindow) return;
    monitoringWindow = window;
    void reloadMonitoring(window);
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

  function calculateTotals(sessionsToTotal: HistoricalSession[], summary: Record<string, unknown> | undefined) {
    const sessionCount = numberFrom(summary?.sessionCount) ?? numberFrom(summary?.totalSessions) ?? sessionsToTotal.length;
    const rxBytes = numberFrom(summary?.rxBytes) ?? numberFrom(summary?.bytesIn) ?? sessionsToTotal.reduce((total, session) => total + (session.rxBytes ?? session.bytesIn ?? 0), 0);
    const txBytes = numberFrom(summary?.txBytes) ?? numberFrom(summary?.bytesOut) ?? sessionsToTotal.reduce((total, session) => total + (session.txBytes ?? session.bytesOut ?? 0), 0);
    return { sessionCount, rxBytes, txBytes, totalBytes: rxBytes + txBytes };
  }

  function numberFrom(value: unknown): number | null {
    return typeof value === 'number' && Number.isFinite(value) ? value : null;
  }

  function sessionUser(session: HistoricalSession): string {
    return session.username ?? session.user ?? '—';
  }

  function chartData(metric: string): { timestamp: Date; value: number }[] {
    const series = metricSeries(metric);
    if (!series) return [];
    return series.points
      .filter(p => Number.isFinite(Date.parse(p.timestamp)) && Number.isFinite(p.value))
      .map(p => ({ timestamp: new Date(p.timestamp), value: p.value }))
      .sort((a, b) => a.timestamp.getTime() - b.timestamp.getTime());
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
          onchange={handleMonitoringWindowChange}
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
      <div class="grid grid-cols-1 lg:grid-cols-2 gap-6">
        {#each chartDefinitions as chart}
          {#if shouldShowChart(chart.metric)}
            <CardChart title={chart.title} chartHeight={200}>
              <MetricChart data={chartData(chart.metric)} metric={chart.metric} height={200} />
            </CardChart>
          {/if}
        {/each}
      </div>
      <UnavailableChips chips={unavailableChips} />
    {/if}
  </section>

  <!-- Recent historical sessions -->
  <section aria-labelledby="sessions-heading" class="mb-8">
    {#if sessions.length > 0}
      <CardTable title="Recent historical sessions">
        <PaginatedTable
          totalPages={pagedSessions.totalPages}
          currentPage={pagedSessions.page}
          onPageChange={(p) => (sessionsPage = p)}
        >
          <table class="w-full text-sm">
            <thead class="bg-slate-50">
              <tr class="text-left text-slate-500 font-medium">
                <th class="px-6 py-3">User</th>
                <th class="px-6 py-3">Started</th>
                <th class="px-6 py-3">Ended</th>
                <th class="px-6 py-3">Received</th>
                <th class="px-6 py-3">Sent</th>
                <th class="px-6 py-3">Remote</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-slate-100">
              {#each pagedSessions.items as session}
                <tr class="hover:bg-slate-50">
                  <td class="px-6 py-3 text-slate-700">{sessionUser(session)}</td>
                  <td class="px-6 py-3 text-slate-700" title={session.connectedAt ?? session.startedAt ?? ''}>{formatRelativeTime(session.connectedAt ?? session.startedAt)}</td>
                  <td class="px-6 py-3 text-slate-700" title={session.disconnectedAt ?? session.endedAt ?? ''}>{formatRelativeTime(session.disconnectedAt ?? session.endedAt)}</td>
                  <td class="px-6 py-3 text-slate-700">{formatBytes(session.rxBytes ?? session.bytesIn ?? 0)}</td>
                  <td class="px-6 py-3 text-slate-700">{formatBytes(session.txBytes ?? session.bytesOut ?? 0)}</td>
                  <td class="px-6 py-3 text-slate-700">{session.remoteAddress ?? '—'}</td>
                </tr>
              {/each}
            </tbody>
          </table>
        </PaginatedTable>
      </CardTable>
    {:else}
      <h2 id="sessions-heading" class="text-lg font-semibold text-slate-700 mb-4">Recent historical sessions</h2>
      <p class="text-sm text-slate-500">No historical sessions returned yet.</p>
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

{:else}
  <div class="bg-red-50 border border-red-200 rounded-lg p-6">
    <p class="text-red-700 text-sm">{error || 'Historical analytics were not returned.'}</p>
  </div>
{/if}
