<script lang="ts">
  import { onMount } from 'svelte';
  import MetricCard from '$lib/components/monitoring/MetricCard.svelte';
  import TimeSeriesChart from '$lib/components/monitoring/TimeSeriesChart.svelte';
  import UnavailableMetric from '$lib/components/monitoring/UnavailableMetric.svelte';
  import WindowSelector from '$lib/components/monitoring/WindowSelector.svelte';
  import {
    getDashboardSummary,
    getMe,
    getMonitoringOverview,
    getSetupStatus,
    principalFromAuth,
    type DashboardSummary,
    type MonitoringOverview,
    type MonitoringSeries,
    type MonitoringWindow,
    type Principal
  } from '$lib/api';
  import { formatMetricValue, latestPoint, metricByName, seriesByMetric } from '$lib/monitoring';

  const MONITORING_WINDOWS: MonitoringWindow[] = ['15m', '1h', '6h', '24h'];
  const HOST_CPU = 'host_cpu_utilization';
  const HOST_MEMORY = 'host_memory_utilization';
  const NET_RX = 'net_interface_rx_bytes';
  const NET_TX = 'net_interface_tx_bytes';
  const VPN_RX = 'vpn_interface_rx_bytes';
  const VPN_TX = 'vpn_interface_tx_bytes';
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

  const sampleDataMessage = $derived(sampleDataText(summary?.sampleData));
  const recentAudit = $derived(summary?.recentAudit ?? []);
  const analyticsRows = $derived(summary?.analytics ? Object.entries(summary.analytics) : []);

  onMount(() => {
    void loadDashboard();
  });

  async function loadDashboard() {
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

  function handleMonitoringWindowChange(event: CustomEvent<string>) {
    const nextWindow = event.detail as MonitoringWindow;
    if (!MONITORING_WINDOWS.includes(nextWindow) || nextWindow === monitoringWindow) return;
    monitoringWindow = nextWindow;
    void reloadMonitoring(nextWindow);
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
    return metricByName(monitoring?.catalog, metric)?.state !== 'unavailable' && Boolean(metricSeries(metric));
  }

  function sampleDataText(value: DashboardSummary['sampleData']): string {
    if (typeof value === 'boolean') return value ? 'Sample data is present for baseline visibility.' : 'No sample data reported.';
    if (value?.enabled || value?.present) return value.message ?? 'Sample data is present for baseline visibility.';
    return 'No sample data reported.';
  }

  function formatValue(value: unknown): string {
    if (value === null || value === undefined) return '—';
    if (typeof value === 'object') return JSON.stringify(value);
    return String(value);
  }
</script>

<section class="page-stack">
  <div class="hero panel">
    <p class="eyebrow">Dashboard</p>
    <h2>Local ocserv administration</h2>
    <p class="muted">
      This UI reads same-origin backend APIs. It does not access local files, sockets, commands,
      certificates, or secrets directly.
    </p>
  </div>

  {#if loading}
    <div class="panel">Loading dashboard state…</div>
  {:else if setupRequired}
    <div class="panel callout warning">
      <h3>Initial setup required</h3>
      <p>Create the first owner before viewing privileged dashboard data.</p>
      <a class="button" href="/setup">Set up owner</a>
    </div>
  {:else if !principal}
    <div class="panel callout">
      <h3>Sign in required</h3>
      <p>Dashboard data is available only after the backend confirms an HttpOnly session cookie.</p>
      <a class="button" href="/login">Go to login</a>
      {#if error}<p class="error-text">{error}</p>{/if}
    </div>
  {:else if summary}
    <div class="grid cards">
      <article class="panel stat-card">
        <span>Backend status</span>
        <strong>{summary.backendStatus ?? 'unknown'}</strong>
      </article>
      <article class="panel stat-card">
        <span>ocserv status</span>
        <strong>{summary.ocservStatus ?? 'not_configured'}</strong>
        <small>Local-only baseline; no live ocserv control is active.</small>
      </article>
      <article class="panel stat-card">
        <span>Active sessions</span>
        <strong>{summary.activeSessionCount ?? 0}</strong>
      </article>
      <article class="panel stat-card">
        <span>Sample data</span>
        <strong>{sampleDataMessage}</strong>
      </article>
    </div>

    <section class="panel page-stack" aria-labelledby="dashboard-monitoring-heading">
      <div class="monitoring-toolbar">
        <div>
          <p class="eyebrow">Monitoring</p>
          <h3 id="dashboard-monitoring-heading">Operational overview</h3>
          <p class="muted">
            Latest backend-collected samples for this node. Session-only metrics remain unavailable until the backend can collect them.
          </p>
        </div>
        <div class="monitoring-toolbar">
          <WindowSelector value={monitoringWindow} windows={MONITORING_WINDOWS} on:change={handleMonitoringWindowChange} />
          <button class="button" type="button" disabled={monitoringLoading} onclick={() => reloadMonitoring()}>
            {monitoringLoading ? 'Refreshing…' : 'Refresh'}
          </button>
        </div>
      </div>

      {#if monitoringError}
        <div class="callout warning" role="status">
          <strong>Monitoring unavailable</strong>
          <p>{monitoringError}</p>
        </div>
      {/if}

      <div class="grid cards">
        <MetricCard label="CPU" value={metricValue(HOST_CPU)} state={metricState(HOST_CPU)} hint={unavailableReason(HOST_CPU)} />
        <MetricCard label="Memory" value={metricValue(HOST_MEMORY)} state={metricState(HOST_MEMORY)} hint={unavailableReason(HOST_MEMORY)} />
        <MetricCard label="Network inbound" value={metricValue(NET_RX)} state={metricState(NET_RX)} hint={unavailableReason(NET_RX)} />
        <MetricCard label="Network outbound" value={metricValue(NET_TX)} state={metricState(NET_TX)} hint={unavailableReason(NET_TX)} />
      </div>

      {#if monitoringLoading && !monitoring}
        <div class="panel">Loading monitoring overview…</div>
      {:else}
        <div class="chart-grid">
          {#each chartDefinitions as chart}
            {#if shouldShowChart(chart.metric)}
              <TimeSeriesChart title={chart.title} series={metricSeries(chart.metric)} />
            {:else}
              <UnavailableMetric title={chart.title} reason={unavailableReason(chart.metric)} />
            {/if}
          {/each}
        </div>
      {/if}
    </section>

    <section class="panel">
      <h3>Analytics summary</h3>
      {#if analyticsRows.length > 0}
        <dl class="definition-grid">
          {#each analyticsRows as [key, value]}
            <div>
              <dt>{key}</dt>
              <dd>{formatValue(value)}</dd>
            </div>
          {/each}
        </dl>
      {:else}
        <p class="muted">No analytics summary returned yet.</p>
      {/if}
    </section>

    <section class="panel">
      <h3>Recent audit</h3>
      {#if recentAudit.length > 0}
        <div class="table-wrap">
          <table>
            <thead><tr><th>Time</th><th>Actor</th><th>Action</th><th>Result</th></tr></thead>
            <tbody>
              {#each recentAudit as event}
                <tr>
                  <td>{event.createdAt ?? event.timestamp ?? '—'}</td>
                  <td>{event.actorUsername ?? event.actor ?? '—'}</td>
                  <td>{event.action ?? event.message ?? '—'}</td>
                  <td>{event.outcome ?? event.result ?? '—'}</td>
                </tr>
              {/each}
            </tbody>
          </table>
        </div>
      {:else}
        <p class="muted">No audit events returned yet.</p>
      {/if}
    </section>
  {:else}
    <div class="panel error-text">{error || 'Dashboard summary was not returned.'}</div>
  {/if}
</section>
