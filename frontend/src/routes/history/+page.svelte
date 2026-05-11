<script lang="ts">
  import { onDestroy, onMount } from 'svelte';
  import MetricChart from '$lib/components/ui/MetricChart.svelte';
  import MultiLineChart from '$lib/components/ui/MultiLineChart.svelte';
  import type { LineData } from '$lib/components/ui/MultiLineChart.svelte';
  import { Users, BarChart2, List, Shield, Settings } from 'lucide-svelte';
  import * as Card from '$lib/components/ui/card/index';
  import { mode } from 'mode-watcher';
  import CardTable from '$lib/components/ui/CardTable.svelte';
  import CardSettings from '$lib/components/ui/CardSettings.svelte';
  import WindowSelector from '$lib/components/ui/WindowSelector.svelte';
  import UnavailableChips from '$lib/components/ui/UnavailableChips.svelte';
  import PaginatedTable from '$lib/components/ui/PaginatedTable.svelte';
  import UserSessionDrawer from '$lib/components/history/UserSessionDrawer.svelte';
  import UnbanConfirmDialog from '../sessions/UnbanConfirmDialog.svelte';
  import type { Chip } from '$lib/components/ui/UnavailableChips.svelte';
  import { formatRelativeTime, formatAbsoluteTime } from '$lib/utils/format';
  import { paginate } from '$lib/utils/pagination';

  import {
    getHistoryUsers,
    getHistoryUserSessions,
    getMe,
    getMonitoringOverview,
    getSettings,
    getSetupStatus,
    hasOwnerRole,
    listSessions,
    listHistoricalBans,
    getUserTraffic,
    unbanIP,
    principalFromAuth,
    purgeHistory,
    updateSetting,
    type HistoricalSession,
    type HistoricalSessionPage,
    type MonitoringOverview,
    type MonitoringSeries,
    type MonitoringWindow,
    type Principal,
    type Setting,
    type UserSessionSummary,
    type UserTrafficResponse,
    type BanEvent,
    type BanEventPage
  } from '$lib/api';
  import { formatBytes, metricByName, seriesByMetric, allSeriesForMetric } from '$lib/monitoring';

  // --- Constants ---
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
    { metric: ACTIVE_SESSIONS, title: 'Active sessions' },
    { metric: CONNECTION_RATE, title: 'Connection rate' },
    { metric: CONCURRENT_SESSIONS, title: 'Concurrent sessions' }
  ];

  // --- Tab definitions ---
  type TabId = 'monitoring' | 'sessions' | 'users' | 'bans' | 'settings';

  const tabs: { id: TabId; label: string; icon: typeof BarChart2 }[] = [
    { id: 'monitoring', label: 'Monitoring', icon: BarChart2 },
    { id: 'sessions', label: 'Sessions', icon: List },
    { id: 'users', label: 'Users', icon: Users },
    { id: 'bans', label: 'Bans', icon: Shield },
    { id: 'settings', label: 'Settings', icon: Settings },
  ];

  // --- Global state ---
  let loading = $state(true);
  let error = $state('');
  let setupRequired = $state(false);
  let principal = $state<Principal | null>(null);

  // --- Tab state ---
  let activeTab = $state<TabId>('monitoring');
  let activated = $state<Record<TabId, boolean>>({
    monitoring: true,
    sessions: false,
    users: false,
    bans: false,
    settings: false
  });

  // --- Monitoring state ---
  let monitoringWindow = $state<MonitoringWindow>('24h');
  let monitoring = $state<MonitoringOverview | null>(null);
  let monitoringLoading = $state(false);
  let monitoringError = $state('');
  let monitoringRequestId = 0;
  let refreshInterval: ReturnType<typeof setInterval> | null = null;

  // --- Sessions tab state ---
  let sessionsWindow = $state<MonitoringWindow>('24h');
  let sessionsPage = $state(1);
  let sessionsData = $state<HistoricalSessionPage | null>(null);
  let sessionsLoading = $state(false);
  let sessionsError = $state('');
  let sessionsSearch = $state('');

  // --- Users tab state ---
  let usersWindow = $state<MonitoringWindow>('24h');
  let userSummaries = $state<UserSessionSummary[]>([]);
  let userSummariesLoading = $state(false);
  let userSummariesError = $state('');
  let userSummariesPage = $state(1);
  let selectedUser = $state<string | null>(null);
  let userTraffic = $state<UserTrafficResponse | null>(null);
  let userTrafficLoading = $state(false);

  // --- Users tab sorting ---
  type SortField = 'totalBytesIn' | 'totalBytesOut' | 'totalTraffic' | 'avgBytesIn' | 'avgBytesOut';
  let usersSortField = $state<SortField>('totalTraffic');
  let usersSortDir = $state<'asc' | 'desc'>('desc');

  // --- Bans tab state ---
  let bansWindow = $state('all');
  let bansPage = $state(1);
  let bansData = $state<BanEventPage | null>(null);
  let bansLoading = $state(false);
  let bansError = $state('');
  let bansSearch = $state('');
  let unbanTarget = $state<string | null>(null);
  let unbanSubmitting = $state(false);
  let unbanError = $state('');

  // --- Settings tab state ---
  let retentionDays = $state<30 | 90 | 180 | 365>(90);
  let confirmation = $state('');
  let purgeSubmitting = $state(false);
  let purgeError = $state('');
  let purgeSuccess = $state('');
  let retentionSetting = $state<string>('365');
  let settingsLoading = $state(false);
  let settingsError = $state('');
  let settingsSuccess = $state('');

  // --- Drawer state ---
  let drawerUsername = $state<string | null>(null);
  let drawerSessions = $state<HistoricalSession[]>([]);
  let drawerLoading = $state(false);
  let drawerWindow = $state<MonitoringWindow>('24h');

  // --- Derived ---
  const isOwner = $derived(hasOwnerRole(principal));
  const purgeEnabled = $derived(isOwner && confirmation === 'PURGE' && !purgeSubmitting);
  const sortedUserSummaries = $derived((() => {
    const arr = [...userSummaries];
    arr.sort((a, b) => {
      let av = 0, bv = 0;
      if (usersSortField === 'totalBytesIn') { av = a.totalBytesIn; bv = b.totalBytesIn; }
      else if (usersSortField === 'totalBytesOut') { av = a.totalBytesOut; bv = b.totalBytesOut; }
      else if (usersSortField === 'totalTraffic') { av = a.totalBytesIn + a.totalBytesOut; bv = b.totalBytesIn + b.totalBytesOut; }
      else if (usersSortField === 'avgBytesIn') { av = a.avgBytesIn; bv = b.avgBytesIn; }
      else if (usersSortField === 'avgBytesOut') { av = a.avgBytesOut; bv = b.avgBytesOut; }
      return usersSortDir === 'asc' ? av - bv : bv - av;
    });
    return arr;
  })());
  const pagedUserSummaries = $derived(paginate(sortedUserSummaries, userSummariesPage, 10));
  const unavailableChips = $derived<Chip[]>(
    chartDefinitions
      .filter(chart => !shouldShowChart(chart.metric))
      .map(chart => ({ name: chart.title, reason: unavailableReason(chart.metric) }))
  );

  // VPN per-interface multi-line chart data
  const vpnRXLines = $derived<LineData[]>(
    allSeriesForMetric(monitoring, VPN_RX).map(s => ({
      name: s.labels?.['interface'] ?? s.metric,
      data: (s.points ?? [])
        .filter(p => Number.isFinite(Date.parse(p.timestamp)) && Number.isFinite(p.value))
        .map(p => ({ timestamp: new Date(p.timestamp), value: p.value }))
        .sort((a, b) => a.timestamp.getTime() - b.timestamp.getTime()),
    }))
  );

  const vpnTXLines = $derived<LineData[]>(
    allSeriesForMetric(monitoring, VPN_TX).map(s => ({
      name: s.labels?.['interface'] ?? s.metric,
      data: (s.points ?? [])
        .filter(p => Number.isFinite(Date.parse(p.timestamp)) && Number.isFinite(p.value))
        .map(p => ({ timestamp: new Date(p.timestamp), value: p.value }))
        .sort((a, b) => a.timestamp.getTime() - b.timestamp.getTime()),
    }))
  );

  const hasVpnRXData = $derived(vpnRXLines.some(l => l.data.length > 0));
  const hasVpnTXData = $derived(vpnTXLines.some(l => l.data.length > 0));

  // --- Lifecycle ---
  onMount(() => {
    void loadHistory();
    refreshInterval = setInterval(() => {
      if (document.visibilityState === 'visible') {
        if (activeTab === 'monitoring') void reloadMonitoring();
        if (activeTab === 'users') void reloadUserSummaries();
      }
    }, 15_000);
  });

  onDestroy(() => {
    if (refreshInterval) {
      clearInterval(refreshInterval);
    }
  });

  // --- Tab activation ---
  function activateTab(tab: TabId) {
    activeTab = tab;
    if (!activated[tab]) {
      activated[tab] = true;
      loadTabData(tab);
    }
  }

  function loadTabData(tab: TabId) {
    if (tab === 'sessions') void loadSessions();
    else if (tab === 'users') void reloadUserSummaries(usersWindow);
    else if (tab === 'bans') void loadBans();
    else if (tab === 'settings') void loadSettings();
  }

  // --- Load functions ---
  async function loadHistory() {
    loading = true;
    error = '';
    try {
      const setup = await getSetupStatus();
      setupRequired = setup.required;
      if (setup.required) return;

      principal = principalFromAuth(await getMe());
      void reloadMonitoring(monitoringWindow);
    } catch (caught) {
      error = caught instanceof Error ? caught.message : 'Unable to load historical analytics.';
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
    if (!HISTORY_WINDOWS.includes(window) || window === monitoringWindow) return;
    monitoringWindow = window;
    void reloadMonitoring(window);
  }

  // --- Sessions tab ---
  async function loadSessions() {
    sessionsLoading = true;
    sessionsError = '';
    try {
      sessionsData = await listSessions(sessionsWindow, sessionsPage);
    } catch (e) {
      sessionsError = e instanceof Error ? e.message : 'Failed to load sessions';
    } finally {
      sessionsLoading = false;
    }
  }

  function handleSessionsWindowChange(nextWindow: string) {
    const window = nextWindow as MonitoringWindow;
    if (!HISTORY_WINDOWS.includes(window) || window === sessionsWindow) return;
    sessionsWindow = window;
    sessionsPage = 1;
    void loadSessions();
  }

  // --- Users tab ---
  async function reloadUserSummaries(window: MonitoringWindow = usersWindow) {
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

  function handleUsersWindowChange(nextWindow: string) {
    const window = nextWindow as MonitoringWindow;
    if (!HISTORY_WINDOWS.includes(window) || window === usersWindow) return;
    usersWindow = window;
    selectedUser = null;
    userTraffic = null;
    void reloadUserSummaries(window);
  }

  function setSortField(field: SortField) {
    if (usersSortField === field) {
      usersSortDir = usersSortDir === 'asc' ? 'desc' : 'asc';
    } else {
      usersSortField = field;
      usersSortDir = 'desc';
    }
  }

  async function loadUserTraffic(username: string) {
    selectedUser = username;
    userTrafficLoading = true;
    userTraffic = null;
    try {
      userTraffic = await getUserTraffic(username, usersWindow);
    } catch {
      userTraffic = null;
    } finally {
      userTrafficLoading = false;
    }
  }

  function closeDrawer() {
    drawerUsername = null;
    drawerSessions = [];
  }

  // --- Bans tab ---
  async function loadBans() {
    bansLoading = true;
    bansError = '';
    try {
      bansData = await listHistoricalBans(bansPage, 25, bansWindow);
    } catch (e) {
      bansError = e instanceof Error ? e.message : 'Failed to load ban history';
    } finally {
      bansLoading = false;
    }
  }

  function handleBansWindowChange(nextWindow: string) {
    bansWindow = nextWindow;
    bansPage = 1;
    void loadBans();
  }

  async function handleUnban() {
    if (!unbanTarget) return;
    unbanSubmitting = true;
    unbanError = '';
    try {
      await unbanIP(unbanTarget);
      unbanTarget = null;
      await loadBans();
    } catch (e) {
      unbanError = e instanceof Error ? e.message : 'Unban failed';
    } finally {
      unbanSubmitting = false;
    }
  }

  // --- Settings tab ---
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

  // --- Sessions tab drawer (from row click) ---
  async function openSessionDrawer(username: string, window: MonitoringWindow) {
    drawerUsername = username;
    drawerWindow = window;
    drawerSessions = [];
    drawerLoading = true;
    try {
      drawerSessions = await getHistoryUserSessions(username, window);
    } catch {
      drawerSessions = [];
    } finally {
      drawerLoading = false;
    }
  }

  // --- Utility functions ---
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
      <p class="text-slate-500 text-sm">Loading history...</p>
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

{:else}
  <div class="mb-6">
    <h1 class="text-xl font-bold text-slate-700">History</h1>
    <p class="text-sm text-slate-500 mt-1">Analytics are read from the local backend API and persisted SQLite history.</p>
  </div>

  <!-- Tab bar -->
  <div class="sticky top-0 z-10 bg-white border-b border-slate-200 mb-6 dark:bg-slate-900 dark:border-slate-700">
    <div class="flex gap-1 px-4 overflow-x-auto" role="tablist" aria-label="History tabs">
      {#each tabs as tab}
        <button
          type="button"
          role="tab"
          onclick={() => activateTab(tab.id)}
          class="flex items-center gap-2 px-4 py-3 text-sm font-medium border-b-2 transition-colors whitespace-nowrap
            {activeTab === tab.id
              ? 'border-blue-600 text-blue-600'
              : 'border-transparent text-slate-500 hover:text-slate-700 hover:border-slate-300 dark:text-slate-400 dark:hover:text-slate-200'}"
          aria-selected={activeTab === tab.id}
        >
          <tab.icon class="w-4 h-4" />
          {tab.label}
        </button>
      {/each}
    </div>
  </div>

  <!-- Tab content -->
  {#if activeTab === 'monitoring'}
    <!-- Monitoring tab -->
    <section aria-labelledby="history-monitoring-heading">
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
            {monitoringLoading ? 'Refreshing...' : 'Refresh'}
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
            <p class="text-slate-500 text-sm">Loading monitoring history...</p>
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
            {#if hasVpnRXData}
              <Card.Root>
                <Card.Header>
                  <div>
                    <Card.Title>VPN inbound</Card.Title>
                    <Card.Description>Last {monitoringWindow}</Card.Description>
                  </div>
                </Card.Header>
                <Card.Content>
                  {#if vpnRXLines.length > 1}
                    <MultiLineChart lines={vpnRXLines} metric={VPN_RX} height={160} />
                  {:else}
                    <MetricChart data={vpnRXLines[0].data} metric={VPN_RX} height={160} />
                  {/if}
                </Card.Content>
              </Card.Root>
            {/if}
            {#if hasVpnTXData}
              <Card.Root>
                <Card.Header>
                  <div>
                    <Card.Title>VPN outbound</Card.Title>
                    <Card.Description>Last {monitoringWindow}</Card.Description>
                  </div>
                </Card.Header>
                <Card.Content>
                  {#if vpnTXLines.length > 1}
                    <MultiLineChart lines={vpnTXLines} metric={VPN_TX} height={160} />
                  {:else}
                    <MetricChart data={vpnTXLines[0].data} metric={VPN_TX} height={160} />
                  {/if}
                </Card.Content>
              </Card.Root>
            {/if}
          </div>
        {/key}
        <UnavailableChips chips={unavailableChips} />
      {/if}
    </section>

  {:else if activeTab === 'sessions'}
    <!-- Sessions tab -->
    <div class="space-y-4">
      <div class="flex items-center justify-between flex-wrap gap-3">
        <WindowSelector
          value={sessionsWindow}
          windows={HISTORY_WINDOWS}
          onchange={handleSessionsWindowChange}
        />
        <input
          type="search"
          placeholder="Filter by username..."
          bind:value={sessionsSearch}
          class="px-3 py-1.5 text-sm border border-slate-200 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500 dark:bg-slate-800 dark:border-slate-700 dark:text-slate-200"
        />
      </div>

      {#if sessionsLoading}
        <div class="text-sm text-slate-500 py-8 text-center">Loading sessions...</div>
      {:else if sessionsError}
        <div class="text-sm text-red-600 py-4">{sessionsError}</div>
      {:else if sessionsData}
        {@const filteredSessions = sessionsData.sessions.filter(s =>
          !sessionsSearch || (s.username ?? '').toLowerCase().includes(sessionsSearch.toLowerCase())
        )}
        <div class="rounded-lg border border-slate-200 overflow-hidden dark:border-slate-700">
          <table class="w-full text-sm">
            <thead class="bg-slate-50 border-b border-slate-200 dark:bg-slate-800 dark:border-slate-700">
              <tr>
                <th class="px-4 py-2 text-left font-medium text-slate-600 dark:text-slate-400">Username</th>
                <th class="px-4 py-2 text-left font-medium text-slate-600 dark:text-slate-400">Connected</th>
                <th class="px-4 py-2 text-left font-medium text-slate-600 dark:text-slate-400">Disconnected</th>
                <th class="px-4 py-2 text-left font-medium text-slate-600 dark:text-slate-400">Duration</th>
                <th class="px-4 py-2 text-right font-medium text-slate-600 dark:text-slate-400">RX</th>
                <th class="px-4 py-2 text-right font-medium text-slate-600 dark:text-slate-400">TX</th>
                <th class="px-4 py-2 text-center font-medium text-slate-600 dark:text-slate-400">State</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-slate-100 dark:divide-slate-700">
              {#each filteredSessions as session}
                {@const stateColors: Record<string, string> = {
                  authenticated: 'bg-green-100 text-green-700',
                  connected: 'bg-green-100 text-green-700',
                  disconnected: 'bg-slate-100 text-slate-600',
                  timed_out: 'bg-amber-100 text-amber-700',
                  error: 'bg-red-100 text-red-700',
                }}
                <tr
                  class="hover:bg-slate-50 cursor-pointer dark:hover:bg-slate-800/50"
                  onclick={() => session.username && openSessionDrawer(session.username, sessionsWindow)}
                >
                  <td class="px-4 py-2 font-mono text-xs text-slate-800 dark:text-slate-200">{session.username ?? '—'}</td>
                  <td class="px-4 py-2 text-slate-600 text-xs dark:text-slate-400">{formatAbsoluteTime(session.connectedAt ?? session.startedAt)}</td>
                  <td class="px-4 py-2 text-slate-600 text-xs dark:text-slate-400">
                    {#if session.state === 'authenticated' || session.state === 'connected'}
                      <span class="inline-flex items-center rounded-full bg-green-100 px-2 py-0.5 text-xs font-medium text-green-800 dark:bg-green-900 dark:text-green-200">Active</span>
                    {:else}
                      {formatAbsoluteTime(session.disconnectedAt ?? session.endedAt)}
                    {/if}
                  </td>
                  <td class="px-4 py-2 text-slate-600 text-xs dark:text-slate-400">{formatDuration(session.durationSeconds ?? 0)}</td>
                  <td class="px-4 py-2 text-right text-xs">{formatBytes(session.bytesIn ?? session.rxBytes ?? 0)}</td>
                  <td class="px-4 py-2 text-right text-xs">{formatBytes(session.bytesOut ?? session.txBytes ?? 0)}</td>
                  <td class="px-4 py-2 text-center">
                    <span class="px-2 py-0.5 rounded-full text-xs font-medium {stateColors[session.state ?? 'disconnected'] ?? 'bg-slate-100 text-slate-600'}">
                      {session.state ?? 'disconnected'}
                    </span>
                  </td>
                </tr>
              {/each}
            </tbody>
          </table>
        </div>
        <!-- Pagination -->
        <div class="flex items-center justify-between text-sm text-slate-500">
          <span>{sessionsData.total} total sessions</span>
          <div class="flex gap-2">
            <button
              disabled={sessionsPage <= 1}
              onclick={() => { sessionsPage--; void loadSessions(); }}
              class="px-3 py-1 rounded border border-slate-200 disabled:opacity-40 hover:bg-slate-50 dark:border-slate-700 dark:hover:bg-slate-800"
            >Previous</button>
            <span class="px-3 py-1">Page {sessionsPage}</span>
            <button
              disabled={sessionsPage * sessionsData.limit >= sessionsData.total}
              onclick={() => { sessionsPage++; void loadSessions(); }}
              class="px-3 py-1 rounded border border-slate-200 disabled:opacity-40 hover:bg-slate-50 dark:border-slate-700 dark:hover:bg-slate-800"
            >Next</button>
          </div>
        </div>
      {/if}
    </div>

  {:else if activeTab === 'users'}
    <!-- Users tab -->
    <section aria-labelledby="users-heading" class="space-y-4">
      <div class="flex items-center justify-between flex-wrap gap-3">
        <h2 id="users-heading" class="text-lg font-semibold text-slate-700">User sessions</h2>
        <WindowSelector
          value={usersWindow}
          windows={HISTORY_WINDOWS}
          onchange={handleUsersWindowChange}
        />
      </div>

      {#if userSummariesError}
        <div class="flex items-center gap-3 py-4">
          <p class="text-red-600 text-sm">{userSummariesError}</p>
          <button
            class="text-sm text-blue-600 hover:underline"
            onclick={() => reloadUserSummaries(usersWindow)}
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
        <CardTable title="User sessions — last {usersWindow}">
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
                  <th class="px-6 py-3 cursor-pointer hover:text-slate-900 dark:hover:text-slate-100"
                      onclick={() => setSortField('avgBytesIn')}>
                    Avg download
                    {#if usersSortField === 'avgBytesIn'}
                      <span class="ml-1">{usersSortDir === 'desc' ? '↓' : '↑'}</span>
                    {/if}
                  </th>
                  <th class="px-6 py-3 cursor-pointer hover:text-slate-900 dark:hover:text-slate-100"
                      onclick={() => setSortField('avgBytesOut')}>
                    Avg upload
                    {#if usersSortField === 'avgBytesOut'}
                      <span class="ml-1">{usersSortDir === 'desc' ? '↓' : '↑'}</span>
                    {/if}
                  </th>
                  <th class="px-6 py-3 cursor-pointer hover:text-slate-900 dark:hover:text-slate-100"
                      onclick={() => setSortField('totalBytesIn')}>
                    Total received
                    {#if usersSortField === 'totalBytesIn'}
                      <span class="ml-1">{usersSortDir === 'desc' ? '↓' : '↑'}</span>
                    {/if}
                  </th>
                  <th class="px-6 py-3 cursor-pointer hover:text-slate-900 dark:hover:text-slate-100"
                      onclick={() => setSortField('totalBytesOut')}>
                    Total sent
                    {#if usersSortField === 'totalBytesOut'}
                      <span class="ml-1">{usersSortDir === 'desc' ? '↓' : '↑'}</span>
                    {/if}
                  </th>
                  <th class="px-6 py-3 cursor-pointer hover:text-slate-900 dark:hover:text-slate-100"
                      onclick={() => setSortField('totalTraffic')}>
                    Total
                    {#if usersSortField === 'totalTraffic'}
                      <span class="ml-1">{usersSortDir === 'desc' ? '↓' : '↑'}</span>
                    {/if}
                  </th>
                </tr>
              </thead>
              <tbody class="divide-y divide-slate-100">
                {#each pagedUserSummaries.items as row (row.username)}
                  <tr
                    class="hover:bg-slate-50 cursor-pointer {selectedUser === row.username ? 'bg-blue-50' : ''}"
                    role="button"
                    tabindex="0"
                    onclick={() => { loadUserTraffic(row.username); }}
                    onkeydown={(e) => { if (e.key === 'Enter') { loadUserTraffic(row.username); } }}
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

      <!-- Per-user traffic chart -->
      {#if selectedUser}
        <div class="mt-4 space-y-2">
          <h3 class="text-sm font-semibold text-slate-800 dark:text-slate-200">Traffic — {selectedUser}</h3>
          {#if userTrafficLoading}
            <div class="py-8 text-center text-sm text-slate-500">Loading...</div>
          {:else if userTraffic && userTraffic.buckets.length > 0}
            {@const rxData = userTraffic.buckets.map(b => ({ timestamp: new Date(b.timestamp), value: b.bytesIn }))}
            {@const txData = userTraffic.buckets.map(b => ({ timestamp: new Date(b.timestamp), value: b.bytesOut }))}
            <div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
              <Card.Root>
                <Card.Header>
                  <div>
                    <Card.Title>RX (bytes in)</Card.Title>
                    <Card.Description>Last {usersWindow}</Card.Description>
                  </div>
                </Card.Header>
                <Card.Content>
                  <MetricChart data={rxData} metric="net_interface_rx_bytes" height={140} />
                </Card.Content>
              </Card.Root>
              <Card.Root>
                <Card.Header>
                  <div>
                    <Card.Title>TX (bytes out)</Card.Title>
                    <Card.Description>Last {usersWindow}</Card.Description>
                  </div>
                </Card.Header>
                <Card.Content>
                  <MetricChart data={txData} metric="net_interface_tx_bytes" height={140} />
                </Card.Content>
              </Card.Root>
            </div>
          {:else}
            <div class="py-8 text-center text-sm text-slate-400">No traffic data for this window</div>
          {/if}
        </div>
      {:else}
        <div class="mt-6 py-12 text-center text-sm text-slate-400">Select a user to view their traffic over time</div>
      {/if}
    </section>

  {:else if activeTab === 'bans'}
    <!-- Bans tab -->
    <div class="space-y-4">
      <div class="flex items-center justify-between flex-wrap gap-3">
        <h2 class="text-base font-semibold text-slate-800 dark:text-slate-200">IP Ban History</h2>
        <div class="flex items-center gap-3">
          <WindowSelector
            value={bansWindow}
            windows={['24h', '7d', '30d', 'all']}
            onchange={handleBansWindowChange}
          />
          <input
            type="search"
            placeholder="Filter by IP..."
            bind:value={bansSearch}
            class="px-3 py-1.5 text-sm border border-slate-200 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500 dark:bg-slate-800 dark:border-slate-700 dark:text-slate-200"
          />
        </div>
      </div>

      {#if bansLoading}
        <div class="py-8 text-center text-sm text-slate-500">Loading...</div>
      {:else if bansError}
        <div class="py-4 text-sm text-red-600">{bansError}</div>
      {:else if bansData}
        {@const filteredBans = bansData.events.filter(e => !bansSearch || e.ip.includes(bansSearch))}
        <div class="rounded-lg border border-slate-200 overflow-hidden dark:border-slate-700">
          <table class="w-full text-sm">
            <thead class="bg-slate-50 border-b border-slate-200 dark:bg-slate-800 dark:border-slate-700">
              <tr>
                <th class="px-4 py-2 text-left font-medium text-slate-600 dark:text-slate-400">IP</th>
                <th class="px-4 py-2 text-right font-medium text-slate-600 dark:text-slate-400">Score</th>
                <th class="px-4 py-2 text-left font-medium text-slate-600 dark:text-slate-400">Reason</th>
                <th class="px-4 py-2 text-left font-medium text-slate-600 dark:text-slate-400">Banned At</th>
                <th class="px-4 py-2 text-left font-medium text-slate-600 dark:text-slate-400">Unbanned At</th>
                <th class="px-4 py-2 text-center font-medium text-slate-600 dark:text-slate-400">Status</th>
                {#if isOwner}<th class="px-4 py-2"></th>{/if}
              </tr>
            </thead>
            <tbody class="divide-y divide-slate-100 dark:divide-slate-700">
              {#each filteredBans as event}
                <tr class="hover:bg-slate-50 dark:hover:bg-slate-800/50">
                  <td class="px-4 py-2 font-mono text-xs text-slate-800 dark:text-slate-200">{event.ip}</td>
                  <td class="px-4 py-2 text-right text-xs text-slate-600 dark:text-slate-400">{event.score}</td>
                  <td class="px-4 py-2 text-xs text-slate-600 dark:text-slate-400">{event.reason || '—'}</td>
                  <td class="px-4 py-2 text-xs text-slate-600 dark:text-slate-400">{formatAbsoluteTime(event.bannedAt)}</td>
                  <td class="px-4 py-2 text-xs text-slate-600 dark:text-slate-400">{event.unbannedAt ? formatAbsoluteTime(event.unbannedAt) : '—'}</td>
                  <td class="px-4 py-2 text-center">
                    {#if event.unbannedAt}
                      <span class="px-2 py-0.5 rounded-full text-xs font-medium bg-slate-100 text-slate-600">Unbanned</span>
                    {:else}
                      <span class="px-2 py-0.5 rounded-full text-xs font-medium bg-red-100 text-red-700">Active</span>
                    {/if}
                  </td>
                  {#if isOwner}
                    <td class="px-4 py-2">
                      {#if !event.unbannedAt}
                        <button
                          onclick={() => unbanTarget = event.ip}
                          class="text-xs text-blue-600 hover:underline"
                        >Unban</button>
                      {/if}
                    </td>
                  {/if}
                </tr>
              {/each}
            </tbody>
          </table>
        </div>
        <!-- Pagination -->
        <div class="flex items-center justify-between text-sm text-slate-500">
          <span>{bansData.total} total ban events</span>
          <div class="flex gap-2">
            <button
              disabled={bansPage <= 1}
              onclick={() => { bansPage--; void loadBans(); }}
              class="px-3 py-1 rounded border border-slate-200 disabled:opacity-40 hover:bg-slate-50 dark:border-slate-700 dark:hover:bg-slate-800"
            >Previous</button>
            <span class="px-3 py-1">Page {bansPage}</span>
            <button
              disabled={bansPage * bansData.limit >= bansData.total}
              onclick={() => { bansPage++; void loadBans(); }}
              class="px-3 py-1 rounded border border-slate-200 disabled:opacity-40 hover:bg-slate-50 dark:border-slate-700 dark:hover:bg-slate-800"
            >Next</button>
          </div>
        </div>
      {/if}
    </div>

  {:else if activeTab === 'settings'}
    <!-- Settings tab -->
    <section class="space-y-8">
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
              {settingsLoading ? 'Saving...' : 'Save retention setting'}
            </button>
          </form>
        {:else}
          <p class="text-sm text-slate-500">Your role can view analytics but cannot change retention settings.</p>
        {/if}
      </CardSettings>

      <!-- History retention purge -->
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
              {purgeSubmitting ? 'Purging...' : 'Purge old history'}
            </button>
          </form>
        {:else}
          <p class="text-sm text-red-600 mt-4">Your role can view analytics but cannot purge historical records.</p>
        {/if}
      </div>
    </section>
  {/if}

  <!-- User session drawer -->
  {#if drawerUsername !== null}
    {#if drawerLoading}
      <div class="fixed inset-0 bg-black/40 z-40 flex items-center justify-center">
        <div class="bg-white rounded-lg p-8 flex flex-col items-center gap-3">
          <div class="w-6 h-6 border-2 border-slate-200 border-t-slate-500 rounded-full animate-spin"></div>
          <p class="text-sm text-slate-500">Loading sessions...</p>
        </div>
      </div>
    {:else}
      <UserSessionDrawer
        username={drawerUsername}
        window={drawerWindow}
        sessions={drawerSessions}
        onclose={closeDrawer}
      />
    {/if}
  {/if}

  <!-- Unban confirm dialog -->
  {#if unbanTarget}
    <UnbanConfirmDialog
      ip={unbanTarget}
      submitting={unbanSubmitting}
      error={unbanError}
      onConfirm={handleUnban}
      onCancel={() => { unbanTarget = null; unbanError = ''; }}
    />
  {/if}

{/if}
