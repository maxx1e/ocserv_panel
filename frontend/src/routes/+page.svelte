<script lang="ts">
  import { onDestroy, onMount } from 'svelte';
  import MetricChart from '$lib/components/ui/MetricChart.svelte';
  import { ArrowDown, ArrowUp, Cpu, Database, MemoryStick, Server, Users } from 'lucide-svelte';

  import CardStats from '$lib/components/ui/CardStats.svelte';
  import CardChart from '$lib/components/ui/CardChart.svelte';
  import CardTable from '$lib/components/ui/CardTable.svelte';
  import WindowSelector from '$lib/components/ui/WindowSelector.svelte';
  import UnavailableChips from '$lib/components/ui/UnavailableChips.svelte';
  import PaginatedTable from '$lib/components/ui/PaginatedTable.svelte';
  import HealthCard from '$lib/components/ui/HealthCard.svelte';
  import type { Chip } from '$lib/components/ui/UnavailableChips.svelte';
  import type { HealthItem } from '$lib/components/ui/HealthCard.svelte';
  import { formatRelativeTime } from '$lib/utils/format';
  import { paginate } from '$lib/utils/pagination';

  import {
    getDashboardSummary,
    getMe,
    getMonitoringOverview,
    getSetupStatus,
    getSystemHealth,
    getLogTail,
    principalFromAuth,
    type DashboardSummary,
    type MonitoringOverview,
    type MonitoringSeries,
    type MonitoringWindow,
    type Principal,
    type SystemHealth,
    type LogTailResult
  } from '$lib/api';
  import { formatBytes, formatMetricValue, latestPoint, metricByName, seriesByMetric } from '$lib/monitoring';

  type ChartDefinition = {
    metric: string;
    title: string;
  };

  const MONITORING_WINDOWS: MonitoringWindow[] = ['15m', '1h', '6h', '24h'];
  const HOST_CPU = 'host_cpu_utilization';
  const HOST_MEMORY = 'host_memory_utilization';
  const NET_RX = 'net_interface_rx_bytes';
  const NET_TX = 'net_interface_tx_bytes';
  const VPN_RX = 'vpn_interface_rx_bytes';
  const VPN_TX = 'vpn_interface_tx_bytes';
  const ACTIVE_SESSIONS = 'active_sessions';
  const CONCURRENT_SESSIONS = 'concurrent_sessions';

  const chartDefinitions: ChartDefinition[] = [
    { metric: HOST_CPU, title: 'CPU utilization' },
    { metric: HOST_MEMORY, title: 'Memory utilization' },
    { metric: NET_RX, title: 'Network inbound' },
    { metric: NET_TX, title: 'Network outbound' },
    { metric: VPN_RX, title: 'VPN inbound' },
    { metric: VPN_TX, title: 'VPN outbound' },
    { metric: ACTIVE_SESSIONS, title: 'Active sessions' },
    { metric: CONCURRENT_SESSIONS, title: 'Concurrent sessions' }
  ];

  let loading = $state(true);
  let error = $state('');
  let setupRequired = $state(false);
  let principal = $state<Principal | null>(null);
  let summary = $state<DashboardSummary | null>(null);
  let monitoringWindow = $state<MonitoringWindow>('1h');
  let monitoring = $state<MonitoringOverview | null>(null);
  let monitoringLoading = $state(false);
  let monitoringError = $state('');
  let monitoringRequestId = 0;
  let refreshInterval: ReturnType<typeof setInterval> | null = null;
  let auditPage = $state(1);

  let health = $state<SystemHealth | null>(null);
  let healthLoading = $state(false);
  let healthError = $state('');
  let healthCheckedAt = $state<Date | null>(null);
  let healthRefreshInterval: ReturnType<typeof setInterval> | null = null;

  // Log tabs
  let activeLogTab = $state<'audit' | 'ocserv' | 'panel'>('audit');
  let ocservLog = $state<LogTailResult | null>(null);
  let panelLog = $state<LogTailResult | null>(null);
  let logLoading = $state(false);
  let logError = $state('');

  const sampleDataMessage = $derived(sampleDataText(summary?.sampleData));
  const recentAudit = $derived(summary?.recentAudit ?? []);

  const unavailableChips = $derived<Chip[]>(
    chartDefinitions
      .filter(chart => !shouldShowChart(chart.metric))
      .map(chart => ({ name: chart.title, reason: unavailableReason(chart.metric) }))
  );

  const pagedAudit = $derived(paginate(recentAudit, auditPage, 10));

  const vpnServerItems = $derived<HealthItem[]>(
    !health ? [] : (() => {
      const v = health.vpnServer;
      const svcStatus: HealthItem['status'] = v.systemdStatus === 'active' ? 'healthy'
        : v.systemdStatus === 'failed' ? 'critical'
        : v.systemdStatus === 'inactive' ? 'warning'
        : 'unknown';
      return [
        { label: 'Systemd service', status: svcStatus, detail: svcStatus !== 'healthy' ? `Status: ${v.systemdStatus}` : undefined },
        { label: 'Socket / process', status: v.socketPresent ? 'healthy' : 'warning' } as HealthItem,
        {
          label: 'Config file',
          status: (v.configPresent && v.configValid ? 'healthy' : v.configPresent && !v.configValid ? 'critical' : 'warning') as HealthItem['status'],
          detail: v.configErrors || undefined
        },
      ];
    })()
  );

  const panelItems = $derived<HealthItem[]>(
    !health ? [] : [
      { label: 'Backend', status: 'healthy' as HealthItem['status'] },
      { label: 'Database', status: (health.panel.dbReachable ? 'healthy' : 'critical') as HealthItem['status'] },
      { label: 'Config file', status: (health.panel.configPresent ? 'healthy' : 'warning') as HealthItem['status'] },
      { label: 'WebUI users', status: 'healthy' as HealthItem['status'], detail: `${health.panel.webuiUserCount} user${health.panel.webuiUserCount === 1 ? '' : 's'}` },
    ]
  );

  onMount(() => {
    void loadDashboard();
    refreshInterval = setInterval(() => {
      if (document.visibilityState === 'visible') {
        void reloadMonitoring();
      }
    }, 15_000);

    void loadHealth();
    healthRefreshInterval = setInterval(() => {
      if (document.visibilityState === 'visible') {
        void loadHealth();
      }
    }, 30_000);
  });

  onDestroy(() => {
    if (refreshInterval) {
      clearInterval(refreshInterval);
    }
    if (healthRefreshInterval) clearInterval(healthRefreshInterval);
  });

  async function loadDashboard() {
    auditPage = 1;
    loading = true;
    error = '';
    try {
      const setup = await getSetupStatus();
      setupRequired = setup.required;
      if (setup.required) return;

      principal = principalFromAuth(await getMe());
      summary = await getDashboardSummary();
      void reloadMonitoring(monitoringWindow);
    } catch (caught) {
      error = caught instanceof Error ? caught.message : 'Unable to load dashboard state.';
      principal = null;
    } finally {
      loading = false;
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
    if (!MONITORING_WINDOWS.includes(window) || window === monitoringWindow) return;
    monitoringWindow = window;
    void reloadMonitoring(window);
  }

  function metricSeries(metric: string): MonitoringSeries | undefined {
    return seriesByMetric(monitoring, metric);
  }

  function metricValue(metric: string): string {
    return formatMetricValue(metric, latestPoint(metricSeries(metric))?.value);
  }

  function metricState(metric: string): 'collecting' | 'unavailable' {
    if (metricByName(monitoring?.catalog, metric)?.state === 'unavailable') return 'unavailable';
    return latestPoint(metricSeries(metric)) ? 'collecting' : 'unavailable';
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

  function sampleDataText(value: DashboardSummary['sampleData']): string {
    if (typeof value === 'boolean') return value ? 'Sample data is present for baseline visibility.' : 'No sample data reported.';
    if (value?.enabled || value?.present) return value.message ?? 'Sample data is present for baseline visibility.';
    return 'No sample data reported.';
  }

  function chartData(metric: string): { timestamp: Date; value: number }[] {
    const series = metricSeries(metric);
    if (!series) return [];
    return series.points
      .filter(p => Number.isFinite(Date.parse(p.timestamp)) && Number.isFinite(p.value))
      .map(p => ({ timestamp: new Date(p.timestamp), value: p.value }))
      .sort((a, b) => a.timestamp.getTime() - b.timestamp.getTime());
  }

  async function loadHealth() {
    healthLoading = true;
    healthError = '';
    try {
      health = await getSystemHealth();
      healthCheckedAt = new Date();
    } catch (caught) {
      healthError = caught instanceof Error ? caught.message : 'Unable to load system health.';
    } finally {
      healthLoading = false;
    }
  }

  async function loadLogTab(source: 'ocserv' | 'panel') {
    logLoading = true;
    logError = '';
    try {
      const result = await getLogTail(source, 25);
      if (source === 'ocserv') ocservLog = result;
      else panelLog = result;
    } catch (caught) {
      logError = caught instanceof Error ? caught.message : 'Unable to load log.';
    } finally {
      logLoading = false;
    }
  }

  function handleLogTabChange(tab: 'audit' | 'ocserv' | 'panel') {
    activeLogTab = tab;
    if (tab === 'ocserv' && !ocservLog) void loadLogTab('ocserv');
    if (tab === 'panel' && !panelLog) void loadLogTab('panel');
  }
</script>

{#if loading}
  <div class="flex items-center justify-center py-16">
    <div class="text-center">
      <div class="w-8 h-8 border-2 border-slate-200 border-t-slate-500 rounded-full animate-spin mx-auto mb-3"></div>
      <p class="text-slate-500 text-sm">Loading dashboard state…</p>
    </div>
  </div>

{:else if setupRequired}
  <div class="bg-amber-50 border border-amber-200 rounded-lg p-6">
    <h3 class="text-lg font-semibold text-amber-800">Initial setup required</h3>
    <p class="text-amber-700 text-sm mt-1">Create the first owner before viewing privileged dashboard data.</p>
    <a class="inline-block mt-4 px-4 py-2 bg-amber-600 text-white text-sm font-medium rounded-md hover:bg-amber-700 transition-colors" href="/setup">
      Set up owner
    </a>
  </div>

{:else if !principal}
  <div class="bg-slate-50 border border-slate-200 rounded-lg p-6">
    <h3 class="text-lg font-semibold text-slate-700">Sign in required</h3>
    <p class="text-slate-500 text-sm mt-1">Dashboard data is available only after the backend confirms an HttpOnly session cookie.</p>
    <a class="inline-block mt-4 px-4 py-2 bg-slate-700 text-white text-sm font-medium rounded-md hover:bg-slate-800 transition-colors" href="/login">
      Go to login
    </a>
    {#if error}
      <p class="mt-3 text-sm text-red-600">{error}</p>
    {/if}
  </div>

{:else if summary}
  <div class="mb-6">
    <h1 class="text-xl font-bold text-slate-700">Dashboard</h1>
    <p class="text-sm text-slate-500 mt-1">Local ocserv administration overview.</p>
  </div>

  <!-- Health cards -->
  <div class="grid grid-cols-1 md:grid-cols-2 gap-6 mb-6">
    {#if healthError}
      <div class="col-span-2 bg-amber-50 border border-amber-200 rounded-lg p-4 text-sm text-amber-700">
        Health check unavailable: {healthError}
      </div>
    {:else}
      <HealthCard
        title="VPN Server"
        items={vpnServerItems}
        lastChecked={healthCheckedAt}
        loading={healthLoading && !health}
      />
      <HealthCard
        title="WebUI Panel"
        items={panelItems}
        lastChecked={healthCheckedAt}
        loading={healthLoading && !health}
      />
    {/if}
  </div>

  <!-- Secondary stat cards (sessions + sample data) -->
  <div class="grid grid-cols-1 md:grid-cols-2 gap-6 mb-8">
    <CardStats
      title="Active sessions"
      value={String(summary.activeSessionCount ?? 0)}
      icon={Users}
    />
    <CardStats
      title="Sample data"
      value={sampleDataMessage}
      icon={Database}
    />
  </div>

  <!-- Monitoring section -->
  <section aria-labelledby="monitoring-heading" class="mb-8">
    <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-3 mb-1">
      <div>
        <h2 id="monitoring-heading" class="text-lg font-semibold text-slate-700">Monitoring</h2>
      </div>
      <div class="flex items-center gap-3">
        <WindowSelector
          value={monitoringWindow}
          windows={MONITORING_WINDOWS}
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
    <p class="text-sm text-slate-500 mt-1 mb-6">
      Latest backend-collected samples for this node. Session-only metrics remain unavailable until the backend can collect them.
    </p>

    {#if monitoringError}
      <div class="bg-red-50 border border-red-200 rounded-lg p-4 mb-6" role="status">
        <strong class="text-red-800 text-sm">Monitoring unavailable</strong>
        <p class="text-red-700 text-sm mt-1">{monitoringError}</p>
      </div>
    {/if}

    <!-- Metric cards -->
    <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-6 mb-6">
      <CardStats
        title="CPU"
        value={metricValue(HOST_CPU)}
        icon={Cpu}
        unavailable={metricState(HOST_CPU) === 'unavailable'}
        hint={unavailableReason(HOST_CPU)}
      />
      <CardStats
        title="Memory"
        value={metricValue(HOST_MEMORY)}
        icon={MemoryStick}
        unavailable={metricState(HOST_MEMORY) === 'unavailable'}
        hint={unavailableReason(HOST_MEMORY)}
      />
      <CardStats
        title="Network inbound"
        value={metricValue(NET_RX)}
        icon={ArrowDown}
        unavailable={metricState(NET_RX) === 'unavailable'}
        hint={unavailableReason(NET_RX)}
      />
      <CardStats
        title="Network outbound"
        value={metricValue(NET_TX)}
        icon={ArrowUp}
        unavailable={metricState(NET_TX) === 'unavailable'}
        hint={unavailableReason(NET_TX)}
      />
    </div>

    <!-- Chart grid -->
    {#if monitoringLoading && !monitoring}
      <div class="flex items-center justify-center py-12">
        <div class="text-center">
          <div class="w-6 h-6 border-2 border-slate-200 border-t-slate-500 rounded-full animate-spin mx-auto mb-2"></div>
          <p class="text-slate-500 text-sm">Loading monitoring overview…</p>
        </div>
      </div>
    {:else}
      <div class="grid grid-cols-1 lg:grid-cols-2 gap-6">
        {#each chartDefinitions as chart}
          {#if shouldShowChart(chart.metric)}
            <CardChart title={chart.title} latestValue={metricValue(chart.metric)} chartHeight={200}>
              <MetricChart data={chartData(chart.metric)} metric={chart.metric} height={200} />
            </CardChart>
          {/if}
        {/each}
      </div>
      <UnavailableChips chips={unavailableChips} />
    {/if}
  </section>

  <!-- Analytics section -->
  <section aria-labelledby="analytics-heading" class="mb-8">
    <h2 id="analytics-heading" class="text-lg font-semibold text-slate-700 mb-4">Analytics Summary</h2>
    {#if summary?.analytics}
      <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-5 gap-6">
        <CardStats
          title="Total sessions"
          value={formatMetricValue('totalSessions', summary.analytics?.totalSessions)}
          icon={Users}
        />
        <CardStats
          title="Sample sessions"
          value={formatMetricValue('sampleSessions', summary.analytics?.sampleSessions)}
          icon={Database}
        />
        <CardStats
          title="Total received"
          value={formatBytes(summary.analytics?.totalBytesIn)}
          icon={ArrowDown}
        />
        <CardStats
          title="Total sent"
          value={formatBytes(summary.analytics?.totalBytesOut)}
          icon={ArrowUp}
        />
        <CardStats
          title="Total traffic"
          value={formatBytes((summary.analytics?.totalBytesIn ?? 0) + (summary.analytics?.totalBytesOut ?? 0))}
          icon={Server}
        />
      </div>
    {:else}
      <p class="text-sm text-slate-500">No analytics summary returned yet.</p>
    {/if}
  </section>

  <!-- Audit / Logs section -->
  <section aria-labelledby="audit-logs-heading" class="mb-8">
    <h2 id="audit-logs-heading" class="sr-only">Audit &amp; Logs</h2>

    <!-- Tab bar -->
    <div class="flex gap-1 mb-4 border-b border-slate-200">
      {#each ([['audit', 'Audit Events'], ['ocserv', 'ocserv Logs'], ['panel', 'Panel Logs']] as const) as [tab, label]}
        <button
          type="button"
          onclick={() => handleLogTabChange(tab)}
          class="px-4 py-2 text-sm font-medium rounded-t-md transition-colors {activeLogTab === tab ? 'bg-white border border-b-white border-slate-200 -mb-px text-slate-800' : 'text-slate-500 hover:text-slate-700'}"
          aria-selected={activeLogTab === tab}
          role="tab"
        >
          {label}
        </button>
      {/each}
    </div>

    <!-- Audit Events tab -->
    {#if activeLogTab === 'audit'}
      {#if recentAudit.length > 0}
        <CardTable title="Recent Audit">
          <PaginatedTable
            totalPages={pagedAudit.totalPages}
            currentPage={pagedAudit.page}
            onPageChange={(p) => (auditPage = p)}
          >
            <table class="w-full text-sm">
              <thead class="bg-slate-50">
                <tr class="text-left text-slate-500 font-medium">
                  <th class="px-6 py-3">Time</th>
                  <th class="px-6 py-3">Actor</th>
                  <th class="px-6 py-3">Action</th>
                  <th class="px-6 py-3">Result</th>
                </tr>
              </thead>
              <tbody class="divide-y divide-slate-100">
                {#each pagedAudit.items as event}
                  <tr class="hover:bg-slate-50">
                    <td class="px-6 py-3 text-slate-700 whitespace-nowrap" title={event.createdAt ?? event.timestamp ?? ''}>{formatRelativeTime(event.createdAt ?? event.timestamp)}</td>
                    <td class="px-6 py-3 text-slate-700">{event.actorUsername ?? event.actor ?? '—'}</td>
                    <td class="px-6 py-3 text-slate-700">{event.action ?? event.message ?? '—'}</td>
                    <td class="px-6 py-3">
                      <span class="inline-block px-2 py-0.5 text-xs font-medium rounded-full {event.outcome === 'success' || event.result === 'success' ? 'bg-emerald-50 text-emerald-700' : event.outcome === 'failure' || event.result === 'failure' ? 'bg-red-50 text-red-700' : 'bg-slate-100 text-slate-600'}">
                        {event.outcome ?? event.result ?? '—'}
                      </span>
                    </td>
                  </tr>
                {/each}
              </tbody>
            </table>
          </PaginatedTable>
        </CardTable>
      {:else}
        <p class="text-sm text-slate-500">No audit events yet.</p>
      {/if}
    {/if}

    <!-- ocserv Logs tab -->
    {#if activeLogTab === 'ocserv'}
      {#if logLoading && !ocservLog}
        <div class="flex items-center gap-2 py-8 text-slate-500 text-sm">
          <div class="w-4 h-4 border-2 border-slate-200 border-t-slate-500 rounded-full animate-spin"></div>
          Loading ocserv log…
        </div>
      {:else if ocservLog}
        {#if !ocservLog.enabled}
          <div class="bg-slate-50 border border-slate-200 rounded-lg p-4 text-sm text-slate-600">
            Log file reading is disabled.
            <a href="/settings#monitoring" class="underline text-slate-700 ml-1">Enable it in Settings &rarr; Monitoring.</a>
          </div>
        {:else if !ocservLog.present}
          <div class="bg-amber-50 border border-amber-200 rounded-lg p-4 text-sm text-amber-700">
            Log file not found at <code class="font-mono">{ocservLog.path}</code>.
            <a href="/settings#installation" class="underline text-amber-800 ml-1">Update the path in Settings &rarr; Installation.</a>
          </div>
        {:else if ocservLog.error}
          <div class="bg-red-50 border border-red-200 rounded-lg p-4 text-sm text-red-700">
            {ocservLog.error}
          </div>
        {:else}
          <div class="bg-white border border-slate-200 rounded-lg">
            <div class="flex items-center justify-between px-4 py-2 border-b border-slate-100">
              <span class="text-xs text-slate-500 font-mono">{ocservLog.path}</span>
              <button
                type="button"
                onclick={() => loadLogTab('ocserv')}
                disabled={logLoading}
                class="text-xs text-slate-500 hover:text-slate-700 disabled:opacity-50"
              >Refresh</button>
            </div>
            <pre class="text-xs font-mono text-slate-700 p-4 overflow-x-auto max-h-64 overflow-y-auto">{ocservLog.lines.join('\n')}</pre>
          </div>
        {/if}
      {/if}
    {/if}

    <!-- Panel Logs tab -->
    {#if activeLogTab === 'panel'}
      {#if logLoading && !panelLog}
        <div class="flex items-center gap-2 py-8 text-slate-500 text-sm">
          <div class="w-4 h-4 border-2 border-slate-200 border-t-slate-500 rounded-full animate-spin"></div>
          Loading panel log…
        </div>
      {:else if panelLog}
        {#if !panelLog.enabled}
          <div class="bg-slate-50 border border-slate-200 rounded-lg p-4 text-sm text-slate-600">
            Log file reading is disabled.
            <a href="/settings#monitoring" class="underline text-slate-700 ml-1">Enable it in Settings &rarr; Monitoring.</a>
          </div>
        {:else if !panelLog.present}
          <div class="bg-amber-50 border border-amber-200 rounded-lg p-4 text-sm text-amber-700">
            Log file not found at <code class="font-mono">{panelLog.path}</code>.
            <a href="/settings#installation" class="underline text-amber-800 ml-1">Update the path in Settings &rarr; Installation.</a>
          </div>
        {:else if panelLog.error}
          <div class="bg-red-50 border border-red-200 rounded-lg p-4 text-sm text-red-700">
            {panelLog.error}
          </div>
        {:else}
          <div class="bg-white border border-slate-200 rounded-lg">
            <div class="flex items-center justify-between px-4 py-2 border-b border-slate-100">
              <span class="text-xs text-slate-500 font-mono">{panelLog.path}</span>
              <button
                type="button"
                onclick={() => loadLogTab('panel')}
                disabled={logLoading}
                class="text-xs text-slate-500 hover:text-slate-700 disabled:opacity-50"
              >Refresh</button>
            </div>
            <pre class="text-xs font-mono text-slate-700 p-4 overflow-x-auto max-h-64 overflow-y-auto">{panelLog.lines.join('\n')}</pre>
          </div>
        {/if}
      {/if}
    {/if}
  </section>

{:else}
  <div class="bg-red-50 border border-red-200 rounded-lg p-6">
    <p class="text-red-700 text-sm">{error || 'Dashboard summary was not returned.'}</p>
  </div>
{/if}
