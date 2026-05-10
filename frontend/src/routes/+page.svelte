<script lang="ts">
  import { onDestroy, onMount } from 'svelte';
  import type { HealthItem } from '$lib/components/ui/HealthCard.svelte';
  import { formatBytes } from '$lib/monitoring';

  import DashboardHealth from '$lib/components/dashboard/DashboardHealth.svelte';
  import DashboardMetrics from '$lib/components/dashboard/DashboardMetrics.svelte';
  import DashboardSessions from '$lib/components/dashboard/DashboardSessions.svelte';

  import {
    getDashboardSummary,
    getDashboardActiveSessions,
    getMe,
    getMonitoringOverview,
    getSetupStatus,
    getSystemHealth,
    getSystemInfo,
    principalFromAuth,
    type ActiveUserSessionsResult,
    type DashboardSummary,
    type MonitoringOverview,
    type MonitoringWindow,
    type Principal,
    type SystemHealth,
    type SystemInfo
  } from '$lib/api';

  const MONITORING_WINDOWS: MonitoringWindow[] = ['15m', '1h', '6h', '24h'];

  // --- State ---
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

  let health = $state<SystemHealth | null>(null);
  let healthLoading = $state(false);
  let healthError = $state('');
  let healthCheckedAt = $state<Date | null>(null);
  let healthRefreshInterval: ReturnType<typeof setInterval> | null = null;

  let activeUserSessions = $state<ActiveUserSessionsResult | null>(null);
  let systemInfo = $state<SystemInfo | null>(null);

  const STORAGE_TOOLTIPS: Record<string, Record<string, { title: string; description: string; action: string }>> = {
    sqlite: {
      warning: { title: 'SQLite Database', description: 'Database size approaching threshold.', action: 'Consider cleanup or VACUUM.' },
      critical: { title: 'SQLite Database', description: 'Database exceeded safe limits or disk nearly full.', action: 'Immediate action required: free disk space or purge old data.' },
    },
    tsdb: {
      warning: { title: 'TSDB Metrics', description: 'TSDB growth high or nearing retention limits.', action: 'Review retention policy or reduce collection frequency.' },
      critical: { title: 'TSDB Metrics', description: 'TSDB exceeded limits or ingestion backlog detected.', action: 'Urgent: check disk space and TSDB compaction.' },
    },
  };

  const storageItems = $derived<HealthItem[]>(
    healthError ? [{ label: 'Storage check unavailable', status: 'unknown' as HealthItem['status'], detail: healthError }] :
    !health?.storage ? [] : [
      {
        label: 'SQLite Database',
        status: (health.storage.sqlite.status ?? 'unknown') as HealthItem['status'],
        detail: formatBytes(health.storage.sqlite.sizeBytes),
        tooltip: health.storage.sqlite.status !== 'healthy'
          ? STORAGE_TOOLTIPS.sqlite[health.storage.sqlite.status] ?? null
          : null,
      },
      {
        label: 'TSDB (metrics)',
        status: (health.storage.tsdb.status ?? 'unknown') as HealthItem['status'],
        detail: formatBytes(health.storage.tsdb.sizeBytes),
        tooltip: health.storage.tsdb.status !== 'healthy'
          ? STORAGE_TOOLTIPS.tsdb[health.storage.tsdb.status] ?? null
          : null,
      },
    ]
  );

  // --- Lifecycle ---
  onMount(() => {
    void loadDashboard();
    refreshInterval = setInterval(() => {
      if (document.visibilityState === 'visible') {
        void reloadMonitoring();
        void loadActiveSessions();
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
    if (refreshInterval) clearInterval(refreshInterval);
    if (healthRefreshInterval) clearInterval(healthRefreshInterval);
  });

  // --- Load functions ---
  async function loadDashboard() {
    loading = true;
    error = '';
    try {
      const setup = await getSetupStatus();
      setupRequired = setup.required;
      if (setup.required) return;

      principal = principalFromAuth(await getMe());
      summary = await getDashboardSummary();

      const backendInterval = summary?.dashboardRefreshInterval;
      if (backendInterval && backendInterval > 0) {
        const ms = backendInterval * 1000;
        if (refreshInterval) clearInterval(refreshInterval);
        refreshInterval = setInterval(() => {
          if (document.visibilityState === 'visible') {
            void reloadMonitoring();
            void loadActiveSessions();
          }
        }, ms);
      }

      void loadActiveSessions();
      void reloadMonitoring(monitoringWindow);
      void loadSystemInfo();
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

  async function loadHealth() {
    healthLoading = true;
    healthError = '';
    try {
      health = await getSystemHealth();
      healthCheckedAt = new Date();
    } catch (caught) {
      healthError = caught instanceof Error ? caught.message : 'Unable to load system health.';
      healthCheckedAt = null;
    } finally {
      healthLoading = false;
    }
  }

  async function loadActiveSessions() {
    try {
      activeUserSessions = await getDashboardActiveSessions();
    } catch {
      activeUserSessions = null;
    }
  }

  async function loadSystemInfo() {
    try {
      systemInfo = await getSystemInfo();
    } catch {
      // Non-fatal — cards degrade gracefully without subtitle
    }
  }
</script>

{#if loading}
  <div class="flex items-center justify-center py-16">
    <div class="text-center">
      <div class="w-8 h-8 border-2 border-slate-200 border-t-slate-500 rounded-full animate-spin mx-auto mb-3"></div>
      <p class="text-slate-500 text-sm">Loading dashboard state...</p>
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

  <DashboardHealth
    {health}
    {healthLoading}
    {healthError}
    {healthCheckedAt}
    {storageItems}
  />

  <DashboardSessions
    {summary}
    {activeUserSessions}
    {storageItems}
    {healthCheckedAt}
    {healthLoading}
    {health}
    {systemInfo}
  />

  <DashboardMetrics
    {monitoring}
    {monitoringLoading}
    {monitoringError}
    {monitoringWindow}
    onWindowChange={handleMonitoringWindowChange}
    onRefresh={() => reloadMonitoring()}
    {systemInfo}
  />

{:else}
  <div class="bg-red-50 border border-red-200 rounded-lg p-6">
    <p class="text-red-700 text-sm">{error || 'Dashboard summary was not returned.'}</p>
  </div>
{/if}
