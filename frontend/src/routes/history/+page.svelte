<script lang="ts">
  import { onDestroy, onMount } from 'svelte';
  import TimeSeriesChart from '$lib/components/monitoring/TimeSeriesChart.svelte';
  import UnavailableMetric from '$lib/components/monitoring/UnavailableMetric.svelte';
  import WindowSelector from '$lib/components/monitoring/WindowSelector.svelte';
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
  import { metricByName, seriesByMetric } from '$lib/monitoring';

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

  const isOwner = $derived(hasOwnerRole(principal));
  const sessions = $derived(analytics?.recentSessions ?? analytics?.sessions ?? []);
  const sampleWarning = $derived(sampleDataWarning(analytics));
  const totals = $derived(calculateTotals(sessions, analytics?.summary));
  const purgeEnabled = $derived(isOwner && confirmation === 'PURGE' && !purgeSubmitting);

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

  function handleMonitoringWindowChange(event: CustomEvent<string>) {
    const nextWindow = event.detail as MonitoringWindow;
    if (!HISTORY_WINDOWS.includes(nextWindow) || nextWindow === monitoringWindow) return;
    monitoringWindow = nextWindow;
    void reloadMonitoring(nextWindow);
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
    return metricByName(monitoring?.catalog, metric)?.state !== 'unavailable' && Boolean(metricSeries(metric));
  }

  function formatGeneratedAt(value: string | undefined): string {
    if (!value) return 'not loaded yet';
    const date = new Date(value);
    if (Number.isNaN(date.getTime())) return value;
    return new Intl.DateTimeFormat(undefined, {
      dateStyle: 'medium',
      timeStyle: 'short'
    }).format(date);
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

  function formatBytes(value: number): string {
    if (value < 1024) return `${value} B`;
    const units = ['KB', 'MB', 'GB', 'TB'];
    let scaled = value / 1024;
    let unit = units[0];
    for (let index = 1; scaled >= 1024 && index < units.length; index += 1) {
      scaled /= 1024;
      unit = units[index];
    }
    return `${scaled.toFixed(1)} ${unit}`;
  }

  function sessionUser(session: HistoricalSession): string {
    return session.username ?? session.user ?? '—';
  }
</script>

<section class="page-stack">
  <div class="hero panel">
    <p class="eyebrow">History</p>
    <h2>Historical session analytics</h2>
    <p class="muted">Analytics are read from the local backend API and persisted SQLite history.</p>
  </div>

  {#if loading}
    <div class="panel">Loading history…</div>
  {:else if setupRequired}
    <div class="panel callout warning">
      <h3>Initial setup required</h3>
      <p>Create the first owner before viewing historical analytics.</p>
      <a class="button" href="/setup">Set up owner</a>
    </div>
  {:else if !principal}
    <div class="panel callout">
      <h3>Sign in required</h3>
      <p>Historical analytics require a backend-confirmed session.</p>
      <a class="button" href="/login">Go to login</a>
      {#if error}<p class="error-text">{error}</p>{/if}
    </div>
  {:else if analytics}
    {#if sampleWarning}
      <div class="callout warning"><strong>Sample data warning:</strong> {sampleWarning}</div>
    {/if}

    <div class="grid cards">
      <article class="panel stat-card"><span>Sessions</span><strong>{totals.sessionCount}</strong></article>
      <article class="panel stat-card"><span>Received</span><strong>{formatBytes(totals.rxBytes)}</strong></article>
      <article class="panel stat-card"><span>Sent</span><strong>{formatBytes(totals.txBytes)}</strong></article>
      <article class="panel stat-card"><span>Total traffic</span><strong>{formatBytes(totals.totalBytes)}</strong></article>
    </div>

    <section class="panel page-stack" aria-labelledby="history-monitoring-heading">
      <div class="monitoring-toolbar">
        <div>
          <p class="eyebrow">Monitoring history</p>
          <h3 id="history-monitoring-heading">Resource and traffic trends</h3>
          <p class="muted">
            Backend-collected samples for longer history windows. User and ocserv-specific metrics show unavailable states when the backend cannot collect them.
          </p>
          <p class="muted">Generated: {formatGeneratedAt(monitoring?.generatedAt)} · Window: {monitoring?.window ?? monitoringWindow}</p>
        </div>
        <div class="monitoring-toolbar">
          <WindowSelector value={monitoringWindow} windows={HISTORY_WINDOWS} on:change={handleMonitoringWindowChange} />
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

      {#if monitoringLoading && !monitoring}
        <div class="panel">Loading monitoring history…</div>
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
      <h3>Recent historical sessions</h3>
      {#if sessions.length > 0}
        <div class="table-wrap">
          <table>
            <thead><tr><th>User</th><th>Started</th><th>Ended</th><th>Received</th><th>Sent</th><th>Remote</th></tr></thead>
            <tbody>
              {#each sessions as session}
                <tr>
                  <td>{sessionUser(session)}</td>
                  <td>{session.connectedAt ?? session.startedAt ?? '—'}</td>
                  <td>{session.disconnectedAt ?? session.endedAt ?? '—'}</td>
                  <td>{formatBytes(session.rxBytes ?? session.bytesIn ?? 0)}</td>
                  <td>{formatBytes(session.txBytes ?? session.bytesOut ?? 0)}</td>
                  <td>{session.remoteAddress ?? '—'}</td>
                </tr>
              {/each}
            </tbody>
          </table>
        </div>
      {:else}
        <p class="muted">No historical sessions returned yet.</p>
      {/if}
    </section>

    <section class="panel">
      <h3>Housekeeping</h3>
      <p class="muted">Configure automatic data retention. Old records are purged daily based on the retention period.</p>
      {#if isOwner}
        <form class="form-stack" onsubmit={handleSaveRetention}>
          <label>
            Auto-delete records older than
            <select bind:value={retentionSetting}>
              <option value="30">30 days</option>
              <option value="90">90 days</option>
              <option value="180">180 days</option>
              <option value="365">365 days (default)</option>
            </select>
          </label>
          {#if settingsError}<p class="error-text">{settingsError}</p>{/if}
          {#if settingsSuccess}<p class="success-text">{settingsSuccess}</p>{/if}
          <button class="button" type="submit" disabled={settingsLoading}>
            {settingsLoading ? 'Saving…' : 'Save retention setting'}
          </button>
        </form>
      {:else}
        <p class="muted">Your role can view analytics but cannot change retention settings.</p>
      {/if}
    </section>

    <section class="panel danger-panel">
      <h3>History retention purge</h3>
      {#if isOwner}
        <p class="muted">Destructive owner-only action. The backend enforces authorization and audit logging.</p>
        <form class="form-stack" onsubmit={handlePurge}>
          <label>
            Keep sessions newer than
            <select bind:value={retentionDays}>
              <option value={30}>30 days</option>
              <option value={90}>90 days</option>
              <option value={180}>180 days</option>
              <option value={365}>365 days</option>
            </select>
          </label>
          <label>
            Type PURGE to confirm deletion of older history
            <input bind:value={confirmation} autocomplete="off" />
          </label>
          {#if purgeError}<p class="error-text">{purgeError}</p>{/if}
          {#if purgeSuccess}<p class="success-text">{purgeSuccess}</p>{/if}
          <button class="button danger" type="submit" disabled={!purgeEnabled}>
            {purgeSubmitting ? 'Purging…' : 'Purge old history'}
          </button>
        </form>
      {:else}
        <p class="muted">Your role can view analytics but cannot purge historical records.</p>
      {/if}
    </section>
  {:else}
    <div class="panel error-text">{error || 'Historical analytics were not returned.'}</div>
  {/if}
</section>
