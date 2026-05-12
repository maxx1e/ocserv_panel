<script lang="ts">
  import { onDestroy, onMount } from 'svelte';
  import ChartCard from '$lib/components/ui/ChartCard.svelte';
  import type { LineData } from '$lib/components/ui/MultiLineChart.svelte';
  import { Users, BarChart2, List, Shield, Settings } from 'lucide-svelte';
  import CardSettings from '$lib/components/ui/CardSettings.svelte';  import WindowSelector from '$lib/components/ui/WindowSelector.svelte';
  import UnavailableChips from '$lib/components/ui/UnavailableChips.svelte';
  import UserSessionDrawer from '$lib/components/history/UserSessionDrawer.svelte';
  import UnbanConfirmDialog from '../sessions/UnbanConfirmDialog.svelte';
  import type { Chip } from '$lib/components/ui/UnavailableChips.svelte';
  import { formatAbsoluteTime } from '$lib/utils/format';

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
    getConnectedUsers,
    getBanPoints,
    unbanIP,
    principalFromAuth,
    purgeHistory,
    updateSetting,
    type ConnectedUser,
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
    type BanEventPage,
    type BanPointEntry
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
  let activeSessions = $state<ConnectedUser[]>([]);
  let activeSessionsLoading = $state(false);

  // --- Users tab state ---
  let usersWindow = $state<MonitoringWindow>('24h');
  let userSummaries = $state<UserSessionSummary[]>([]);
  let userSummariesLoading = $state(false);
  let userSummariesError = $state('');
  let expandedUser = $state<string | null>(null);
  let usersSearch = $state('');
  let userTraffic = $state<UserTrafficResponse | null>(null);
  let userTrafficLoading = $state(false);

  // --- Users tab sorting ---
  type SortField = 'totalBytesIn' | 'totalBytesOut' | 'totalTraffic' | 'avgBytesIn' | 'avgBytesOut';
  let usersSortField = $state<SortField>('totalTraffic');
  let usersSortDir = $state<'asc' | 'desc'>('desc');

  // --- Bans tab state ---
  type BanStatus = 'banned' | 'watching' | 'known' | 'unbanned';
  type MergedBanRow = {
    ip: string;
    score: number;
    reason: string;
    bannedAt: string | null;
    unbannedAt: string | null;
    status: BanStatus;
  };

  const BAN_WATCHING_THRESHOLD = 50;

  let bansWindow = $state('all');
  let bansPage = $state(1);
  let bansData = $state<BanEventPage | null>(null);
  let bansLoading = $state(false);
  let bansError = $state('');
  let bansSearch = $state('');
  let banPoints = $state<BanPointEntry[]>([]);
  let bansStatusFilter = $state<'all' | BanStatus>('all');
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
  const activeUsernames = $derived(new Set(activeSessions.map(s => s.username)));
  const filteredUserSummaries = $derived(
    usersSearch
      ? sortedUserSummaries.filter(u =>
          u.username.toLowerCase().includes(usersSearch.toLowerCase())
        )
      : sortedUserSummaries
  );
  const unavailableChips = $derived<Chip[]>(
    chartDefinitions
      .filter(chart => !shouldShowChart(chart.metric))
      .map(chart => ({ name: chart.title, reason: unavailableReason(chart.metric) }))
  );

  // --- Bans derived ---
  const mergedBanRows = $derived<MergedBanRow[]>((() => {
    const rows: MergedBanRow[] = banPoints.map(bp => {
      const hist = bansData?.events.find(e => e.ip === bp.ip);
      let status: BanStatus;
      if (hist && !hist.unbannedAt) {
        status = 'banned';
      } else if (hist && hist.unbannedAt) {
        status = 'unbanned';
      } else {
        status = bp.score >= BAN_WATCHING_THRESHOLD ? 'watching' : 'known';
      }
      return {
        ip: bp.ip,
        score: bp.score,
        reason: hist?.reason ?? '',
        bannedAt: hist?.bannedAt ?? null,
        unbannedAt: hist?.unbannedAt ?? null,
        status,
      };
    });

    if (bansData) {
      for (const evt of bansData.events) {
        if (!rows.find(r => r.ip === evt.ip)) {
          rows.push({
            ip: evt.ip,
            score: evt.score,
            reason: evt.reason,
            bannedAt: evt.bannedAt,
            unbannedAt: evt.unbannedAt ?? null,
            status: evt.unbannedAt ? 'unbanned' : 'banned',
          });
        }
      }
    }

    return rows.sort((a, b) => b.score - a.score);
  })());

  const filteredBanRows = $derived(
    mergedBanRows.filter(r => {
      if (bansStatusFilter !== 'all' && r.status !== bansStatusFilter) return false;
      if (bansSearch && !r.ip.includes(bansSearch)) return false;
      return true;
    })
  );

  const banCounts = $derived({
    all: mergedBanRows.length,
    banned: mergedBanRows.filter(r => r.status === 'banned').length,
    watching: mergedBanRows.filter(r => r.status === 'watching').length,
    known: mergedBanRows.filter(r => r.status === 'known' || r.status === 'unbanned').length,
  });

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
        if (activeTab === 'sessions') void loadActiveSessions();
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
    if (tab === 'sessions') { void loadSessions(); void loadActiveSessions(); }
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

  async function loadActiveSessions() {
    activeSessionsLoading = true;
    try {
      activeSessions = await getConnectedUsers();
    } catch {
      activeSessions = [];
    } finally {
      activeSessionsLoading = false;
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
    expandedUser = null;
    userTraffic = null;
    void reloadUserSummaries(window);
  }

  async function loadUserTraffic(username: string) {
    if (expandedUser === username) {
      expandedUser = null;
      userTraffic = null;
      return;
    }
    expandedUser = username;
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
      const [page, points] = await Promise.all([
        listHistoricalBans(bansPage, 25, bansWindow),
        getBanPoints(),
      ]);
      bansData = page;
      banPoints = points;
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
        <div class="grid grid-cols-1 lg:grid-cols-2 gap-4">
          {#each chartDefinitions as chart}
            {#if shouldShowChart(chart.metric)}
              <ChartCard
                title={chart.title}
                metric={chart.metric}
                data={chartData(chart.metric)}
                window={monitoringWindow}
                height={160}
              />
            {/if}
          {/each}
          {#if hasVpnRXData}
            <ChartCard
              title="VPN inbound"
              metric={VPN_RX}
              lines={vpnRXLines}
              window={monitoringWindow}
              height={160}
            />
          {/if}
          {#if hasVpnTXData}
            <ChartCard
              title="VPN outbound"
              metric={VPN_TX}
              lines={vpnTXLines}
              window={monitoringWindow}
              height={160}
            />
          {/if}
        </div>
        <UnavailableChips chips={unavailableChips} />
      {/if}
    </section>

  {:else if activeTab === 'sessions'}
    <!-- Sessions tab -->
    <div class="space-y-6">
      <!-- Toolbar -->
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

      <!-- Section A: Currently Active -->
      <div class="bg-green-50 border border-green-200 rounded-xl p-4">
        <div class="flex items-center gap-2 mb-3">
          <span class="relative flex h-2.5 w-2.5">
            <span class="animate-ping absolute inline-flex h-full w-full rounded-full bg-green-400 opacity-75"></span>
            <span class="relative inline-flex rounded-full h-2.5 w-2.5 bg-green-500"></span>
          </span>
          <span class="text-sm font-semibold text-green-800">
            Currently active ({activeSessions.length})
          </span>
        </div>
        {#if activeSessionsLoading && activeSessions.length === 0}
          <p class="text-xs text-green-700">Loading...</p>
        {:else if activeSessions.length === 0}
          <p class="text-xs text-green-700">No active sessions right now</p>
        {:else}
          <div class="overflow-x-auto">
            <table class="w-full text-sm">
              <thead>
                <tr class="text-left text-xs text-green-700 font-semibold">
                  <th class="pb-1 pr-4">Username</th>
                  <th class="pb-1 pr-4">Device</th>
                  <th class="pb-1 pr-4">Connected since</th>
                  <th class="pb-1 pr-4">Remote IP</th>
                  <th class="pb-1 pr-4">VPN IP</th>
                </tr>
              </thead>
              <tbody class="divide-y divide-green-100">
                {#each activeSessions as s (s.id)}
                  <tr>
                    <td class="py-1.5 pr-4 font-mono text-xs text-green-900 font-semibold">{s.username}</td>
                    <td class="py-1.5 pr-4 text-xs text-green-800">{s.device}</td>
                    <td class="py-1.5 pr-4 text-xs text-green-800">{formatDuration(Math.floor((Date.now() - new Date(s.since).getTime()) / 1000))}</td>
                    <td class="py-1.5 pr-4 text-xs text-green-800">{s.remoteIp}</td>
                    <td class="py-1.5 text-xs text-green-800">{s.vpnIp}</td>
                  </tr>
                {/each}
              </tbody>
            </table>
          </div>
        {/if}
      </div>

      <!-- Section B: History -->
      {#if sessionsLoading && !sessionsData}
        <div class="text-sm text-slate-500 py-8 text-center">Loading sessions...</div>
      {:else if sessionsError}
        <div class="text-sm text-red-600 py-4">{sessionsError}</div>
      {:else if sessionsData}
        {@const filteredSessions = sessionsData.sessions.filter(s =>
          !sessionsSearch || (s.username ?? '').toLowerCase().includes(sessionsSearch.toLowerCase())
        )}
        <div>
          <p class="text-xs font-semibold text-slate-500 uppercase tracking-wider mb-2">
            History — last {sessionsWindow} ({sessionsData.total} sessions)
          </p>
          <div class="rounded-lg border border-slate-200 overflow-hidden dark:border-slate-700">
            <table class="w-full text-sm">
              <thead class="bg-slate-50 border-b border-slate-200 dark:bg-slate-800 dark:border-slate-700">
                <tr>
                  <th class="px-4 py-2 text-left font-medium text-slate-600 dark:text-slate-400">Username</th>
                  <th class="px-4 py-2 text-left font-medium text-slate-600 dark:text-slate-400">Connected</th>
                  <th class="px-4 py-2 text-left font-medium text-slate-600 dark:text-slate-400">Ended</th>
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
          <div class="flex items-center justify-between text-sm text-slate-500 mt-3">
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
        </div>
      {/if}
    </div>

  {:else if activeTab === 'users'}
    <!-- Users tab -->
    <section aria-labelledby="users-heading" class="space-y-4">
      <!-- Compact toolbar -->
      <div class="flex items-center gap-3 flex-wrap">
        <WindowSelector
          value={usersWindow}
          windows={HISTORY_WINDOWS}
          onchange={handleUsersWindowChange}
        />
        <input
          type="search"
          placeholder="Filter user..."
          bind:value={usersSearch}
          class="px-3 py-1.5 text-sm border border-slate-200 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500 dark:bg-slate-800 dark:border-slate-700 dark:text-slate-200 w-40"
        />
        <div class="flex items-center gap-2 ml-auto text-sm text-slate-500">
          <label for="users-sort" class="text-xs font-medium">Sort:</label>
          <select
            id="users-sort"
            bind:value={usersSortField}
            onchange={() => { usersSortDir = 'desc'; }}
            class="text-xs border border-slate-200 rounded-md px-2 py-1.5 bg-white text-slate-700 dark:bg-slate-800 dark:border-slate-700 dark:text-slate-200"
          >
            <option value="totalTraffic">Total traffic ↓</option>
            <option value="totalBytesIn">Total received ↓</option>
            <option value="totalBytesOut">Total sent ↓</option>
            <option value="avgBytesIn">Avg download ↓</option>
            <option value="avgBytesOut">Avg upload ↓</option>
          </select>
        </div>
      </div>

      {#if userSummariesError}
        <div class="flex items-center gap-3 py-4">
          <p class="text-red-600 text-sm">{userSummariesError}</p>
          <button class="text-sm text-blue-600 hover:underline" onclick={() => reloadUserSummaries(usersWindow)}>Retry</button>
        </div>
      {/if}

      {#if userSummariesLoading && userSummaries.length === 0}
        <div class="space-y-2">
          {#each Array(5) as _}
            <div class="h-10 bg-slate-100 rounded animate-pulse"></div>
          {/each}
        </div>
      {:else if filteredUserSummaries.length === 0}
        <p class="text-sm text-slate-500">No sessions in this period.</p>
      {:else}
        <div class="rounded-lg border border-slate-200 overflow-hidden dark:border-slate-700">
          <table class="w-full text-sm">
            <thead class="bg-slate-50 dark:bg-slate-800">
              <tr class="text-left text-xs text-slate-500 font-medium">
                <th class="px-4 py-3">User</th>
                <th class="px-4 py-3">Sessions</th>
                <th class="px-4 py-3">Total time</th>
                <th class="px-4 py-3">↓ Total</th>
                <th class="px-4 py-3">↑ Total</th>
                <th class="px-4 py-3 text-blue-600 font-semibold">Total ↕</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-slate-100 dark:divide-slate-700">
              {#each filteredUserSummaries as row (row.username)}
                {@const initials = row.username.slice(0, 1).toUpperCase()}
                {@const isActive = activeUsernames.has(row.username)}
                {@const isExpanded = expandedUser === row.username}
                <tr
                  class="hover:bg-slate-50 cursor-pointer dark:hover:bg-slate-800/50 {isExpanded ? 'bg-blue-50 dark:bg-blue-900/20' : ''}"
                  role="button"
                  tabindex="0"
                  onclick={() => loadUserTraffic(row.username)}
                  onkeydown={(e) => { if (e.key === 'Enter') loadUserTraffic(row.username); }}
                >
                  <td class="px-4 py-3">
                    <div class="flex items-center gap-2">
                      <div class="w-7 h-7 rounded-full bg-indigo-100 text-indigo-700 flex items-center justify-center text-xs font-bold flex-shrink-0">{initials}</div>
                      <span class="text-slate-700 font-medium dark:text-slate-200">{row.username}</span>
                      {#if isActive}
                        <span class="w-2 h-2 rounded-full bg-green-500 flex-shrink-0" title="Currently active"></span>
                      {/if}
                    </div>
                  </td>
                  <td class="px-4 py-3 text-slate-600 dark:text-slate-400">{row.sessionCount}</td>
                  <td class="px-4 py-3 text-slate-600 dark:text-slate-400">{formatDuration(row.totalDurationSeconds)}</td>
                  <td class="px-4 py-3 text-slate-600 dark:text-slate-400">{formatBytes(row.totalBytesIn)}</td>
                  <td class="px-4 py-3 text-slate-600 dark:text-slate-400">{formatBytes(row.totalBytesOut)}</td>
                  <td class="px-4 py-3 font-semibold text-slate-800 dark:text-slate-100">{formatBytes(row.totalBytesIn + row.totalBytesOut)}</td>
                </tr>
                {#if isExpanded}
                  <tr>
                    <td colspan="6" class="px-4 py-4 bg-slate-50 dark:bg-slate-800/50 border-t border-slate-100 dark:border-slate-700">
                      <div class="flex items-center justify-between mb-3">
                        <span class="text-sm font-semibold text-slate-700 dark:text-slate-200">Traffic — {row.username}</span>
                        <button
                          type="button"
                          onclick={(e) => { e.stopPropagation(); expandedUser = null; userTraffic = null; }}
                          class="text-slate-400 hover:text-slate-600 text-lg leading-none"
                          aria-label="Close"
                        >×</button>
                      </div>
                      {#if userTrafficLoading}
                        <div class="py-6 text-center text-sm text-slate-500">Loading...</div>
                      {:else if userTraffic && userTraffic.buckets.length > 0}
                        {@const rxData = userTraffic.buckets.map(b => ({ timestamp: new Date(b.timestamp), value: b.bytesIn }))}
                        {@const txData = userTraffic.buckets.map(b => ({ timestamp: new Date(b.timestamp), value: b.bytesOut }))}
                        <div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
                          <ChartCard title="Download (RX)" metric="user_rx_bytes" data={rxData} window={usersWindow} height={140} />
                          <ChartCard title="Upload (TX)" metric="user_tx_bytes" data={txData} window={usersWindow} height={140} />
                        </div>
                      {:else}
                        <div class="py-6 text-center text-sm text-slate-400">No traffic data for this window</div>
                      {/if}
                    </td>
                  </tr>
                {/if}
              {/each}
            </tbody>
          </table>
        </div>
        <div class="flex items-center justify-between text-sm text-slate-500">
          <span>{filteredUserSummaries.length} users</span>
        </div>
      {/if}
    </section>

  {:else if activeTab === 'bans'}
    <!-- Bans tab -->
    <div class="space-y-4">
      <!-- Toolbar -->
      <div class="flex items-center gap-2 flex-wrap">
        <!-- Filter chips -->
        <div class="flex gap-1.5">
          {#each (['all', 'banned', 'watching', 'known'] as const) as val}
            {@const label = val === 'all' ? 'All' : val === 'banned' ? 'Banned' : val === 'watching' ? 'Watching' : 'Known'}
            {@const count = val === 'all' ? banCounts.all : val === 'banned' ? banCounts.banned : val === 'watching' ? banCounts.watching : banCounts.known}
            <button
              type="button"
              onclick={() => bansStatusFilter = val}
              class="px-3 py-1 text-xs font-medium rounded-full border transition-colors {bansStatusFilter === val
                ? val === 'banned' ? 'bg-red-100 text-red-700 border-red-200'
                : val === 'watching' ? 'bg-amber-100 text-amber-700 border-amber-200'
                : 'bg-slate-800 text-white border-slate-800'
                : 'bg-white text-slate-500 border-slate-200 hover:bg-slate-50'}"
            >
              {label} ({count})
            </button>
          {/each}
        </div>
        <!-- Search -->
        <input
          type="search"
          placeholder="Filter by IP..."
          bind:value={bansSearch}
          class="px-3 py-1.5 text-sm border border-slate-200 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500 dark:bg-slate-800 dark:border-slate-700 dark:text-slate-200 w-36"
        />
        <!-- Window selector (right-aligned) -->
        <div class="ml-auto">
          <WindowSelector
            value={bansWindow}
            windows={['24h', '7d', '30d', 'all']}
            onchange={handleBansWindowChange}
          />
        </div>
      </div>

      <p class="text-xs text-slate-400">
        Showing ban history for last {bansWindow === 'all' ? 'all time' : bansWindow} · Live scores always current
      </p>

      {#if bansLoading}
        <div class="py-8 text-center text-sm text-slate-500">Loading...</div>
      {:else if bansError}
        <div class="py-4 text-sm text-red-600">{bansError}</div>
      {:else}
        <div class="rounded-lg border border-slate-200 overflow-hidden dark:border-slate-700">
          <table class="w-full text-sm">
            <thead class="bg-slate-50 border-b border-slate-200 dark:bg-slate-800 dark:border-slate-700">
              <tr>
                <th class="px-4 py-2 text-left font-medium text-slate-600 dark:text-slate-400">IP Address</th>
                <th class="px-4 py-2 text-left font-medium text-slate-600 dark:text-slate-400 w-32">Score</th>
                <th class="px-4 py-2 text-left font-medium text-slate-600 dark:text-slate-400">Reason</th>
                <th class="px-4 py-2 text-left font-medium text-slate-600 dark:text-slate-400">Banned At</th>
                <th class="px-4 py-2 text-center font-medium text-slate-600 dark:text-slate-400">Status</th>
                {#if isOwner}<th class="px-4 py-2"></th>{/if}
              </tr>
            </thead>
            <tbody class="divide-y divide-slate-100 dark:divide-slate-700">
              {#each filteredBanRows as row}
                {@const barColor =
                  row.status === 'banned' ? 'bg-red-500' :
                  row.status === 'watching' ? 'bg-amber-400' : 'bg-slate-300'}
                {@const barWidth = Math.min(100, Math.max(2, row.score))}
                {@const badgeClass =
                  row.status === 'banned' ? 'bg-red-100 text-red-700' :
                  row.status === 'watching' ? 'bg-amber-100 text-amber-700' :
                  'bg-slate-100 text-slate-600'}
                <tr class="hover:bg-slate-50 dark:hover:bg-slate-800/50">
                  <td class="px-4 py-2 font-mono text-xs text-slate-800 dark:text-slate-200">{row.ip}</td>
                  <td class="px-4 py-2">
                    <div class="flex items-center gap-2">
                      <div class="flex-1 h-1.5 bg-slate-100 rounded-full overflow-hidden">
                        <div class="{barColor} h-full rounded-full" style="width: {barWidth}%"></div>
                      </div>
                      <span class="text-xs font-semibold text-slate-700 dark:text-slate-300 tabular-nums w-8 text-right">{row.score}</span>
                    </div>
                  </td>
                  <td class="px-4 py-2 text-xs text-slate-600 dark:text-slate-400">{row.reason || '—'}</td>
                  <td class="px-4 py-2 text-xs text-slate-600 dark:text-slate-400">{row.bannedAt ? formatAbsoluteTime(row.bannedAt) : '—'}</td>
                  <td class="px-4 py-2 text-center">
                    <span class="px-2 py-0.5 rounded-full text-xs font-medium capitalize {badgeClass}">
                      {row.status}
                    </span>
                  </td>
                  {#if isOwner}
                    <td class="px-4 py-2">
                      {#if row.status === 'banned'}
                        <button
                          onclick={() => unbanTarget = row.ip}
                          class="text-xs text-blue-600 hover:underline"
                        >Unban</button>
                      {/if}
                    </td>
                  {/if}
                </tr>
              {/each}
              {#if filteredBanRows.length === 0}
                <tr>
                  <td colspan="6" class="px-4 py-8 text-center text-sm text-slate-400">No entries match the current filter.</td>
                </tr>
              {/if}
            </tbody>
          </table>
        </div>
        <div class="flex items-center justify-between text-sm text-slate-500">
          <span>{filteredBanRows.length} of {mergedBanRows.length} entries</span>
          {#if bansData}
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
          {/if}
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
