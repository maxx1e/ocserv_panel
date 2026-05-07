<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { FileText, Shield } from 'lucide-svelte';
  import EmptyState from '$lib/components/ui/EmptyState.svelte';
  import {
    getMe,
    getSetupStatus,
    getAuditLogs,
    getAuditLogSources,
    exportAuditLogs,
    principalFromAuth,
    hasOwnerRole,
    ApiError,
    type Principal,
    type AuditLogLine,
    type AuditLogSource,
    type AuditLogParams
  } from '$lib/api';
  import LogToolbar from './LogToolbar.svelte';
  import LogTable from './LogTable.svelte';
  import LogLevelBanner from './LogLevelBanner.svelte';

  // ── page state ────────────────────────────────────────────────────────────

  let loading = $state(true);
  let error = $state('');
  let forbidden = $state(false);
  let setupRequired = $state(false);
  let principal = $state<Principal | null>(null);

  // Log data
  let activeTab = $state<'panel' | 'ocserv'>('panel');
  let lines = $state<AuditLogLine[]>([]);
  let total = $state(0);
  let offset = $state(0);
  let limit = $state(100);
  let hasMore = $state(false);
  let logLoading = $state(false);
  let logError = $state('');

  // Sources metadata
  let sources = $state<AuditLogSource[]>([]);

  // Filters
  let search = $state('');
  let level = $state('');
  let timeRange = $state('24h');
  let autoRefresh = $state(false);

  // Derived
  const isOwner = $derived(hasOwnerRole(principal));
  const ocservSource = $derived(sources.find((s) => s.name === 'ocserv'));
  const activeSource = $derived(sources.find((s) => s.name === activeTab));

  // Auto-refresh interval
  let refreshInterval: ReturnType<typeof setInterval> | null = null;

  // ── lifecycle ─────────────────────────────────────────────────────────────

  onMount(() => {
    void initialize();
  });

  onDestroy(() => {
    if (refreshInterval) clearInterval(refreshInterval);
  });

  async function initialize() {
    loading = true;
    error = '';
    forbidden = false;

    try {
      const setup = await getSetupStatus();
      setupRequired = setup.required;
      if (setup.required) return;

      principal = principalFromAuth(await getMe());

      // Load sources metadata
      try {
        const resp = await getAuditLogSources();
        sources = resp.sources;
      } catch {
        // non-fatal, we can still try to load logs
      }

      await loadLogs();
    } catch (caught) {
      if (caught instanceof ApiError && caught.status === 403) {
        forbidden = true;
        return;
      }
      error = caught instanceof Error ? caught.message : 'Unable to load audit page.';
    } finally {
      loading = false;
    }
  }

  // ── data loading ──────────────────────────────────────────────────────────

  async function loadLogs() {
    logLoading = true;
    logError = '';

    try {
      const params: AuditLogParams = {
        source: activeTab,
        limit,
        offset,
        order: 'desc'
      };
      if (search) params.search = search;
      if (level) params.level = level;

      const timeParams = getTimeParams();
      if (timeParams.from) params.from = timeParams.from;
      if (timeParams.to) params.to = timeParams.to;

      const result = await getAuditLogs(params);
      lines = result.lines;
      total = result.total;
      hasMore = result.hasMore;
    } catch (caught) {
      if (caught instanceof ApiError) {
        if (caught.status === 503) {
          logError = `Log source "${activeTab}" is unavailable. Check that the log file exists or systemd journal is accessible.`;
        } else {
          logError = caught.message;
        }
      } else {
        logError = caught instanceof Error ? caught.message : 'Failed to load logs.';
      }
      lines = [];
      total = 0;
      hasMore = false;
    } finally {
      logLoading = false;
    }
  }

  function getTimeParams(): { from?: string; to?: string } {
    if (!timeRange) return {};
    const now = new Date();
    let from: Date;
    switch (timeRange) {
      case '1h':
        from = new Date(now.getTime() - 60 * 60 * 1000);
        break;
      case '6h':
        from = new Date(now.getTime() - 6 * 60 * 60 * 1000);
        break;
      case '24h':
        from = new Date(now.getTime() - 24 * 60 * 60 * 1000);
        break;
      case '7d':
        from = new Date(now.getTime() - 7 * 24 * 60 * 60 * 1000);
        break;
      default:
        return {};
    }
    return { from: from.toISOString(), to: now.toISOString() };
  }

  // ── event handlers ────────────────────────────────────────────────────────

  function handleTabChange(tab: 'panel' | 'ocserv') {
    activeTab = tab;
    offset = 0;
    void loadLogs();
  }

  function handleSearchChange(value: string) {
    search = value;
    offset = 0;
    void loadLogs();
  }

  function handleLevelChange(value: string) {
    level = value;
    offset = 0;
    void loadLogs();
  }

  function handleTimeRangeChange(value: string) {
    timeRange = value;
    offset = 0;
    void loadLogs();
  }

  function handlePageChange(newOffset: number) {
    offset = newOffset;
    void loadLogs();
  }

  function handleRefresh() {
    void loadLogs();
  }

  function handleAutoRefreshToggle() {
    autoRefresh = !autoRefresh;
    if (autoRefresh) {
      refreshInterval = setInterval(() => void loadLogs(), 10_000);
    } else {
      if (refreshInterval) {
        clearInterval(refreshInterval);
        refreshInterval = null;
      }
    }
  }

  async function handleExport() {
    try {
      const params: AuditLogParams = { source: activeTab };
      if (search) params.search = search;
      if (level) params.level = level;
      const timeParams = getTimeParams();
      if (timeParams.from) params.from = timeParams.from;
      if (timeParams.to) params.to = timeParams.to;

      const blob = await exportAuditLogs({ ...params, format: 'json' });
      const url = URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = `audit-logs-${activeTab}-${new Date().toISOString().slice(0, 10)}.json`;
      document.body.appendChild(a);
      a.click();
      document.body.removeChild(a);
      URL.revokeObjectURL(url);
    } catch (caught) {
      logError = caught instanceof Error ? caught.message : 'Export failed.';
    }
  }
</script>

<!-- Loading -->
{#if loading}
  <div class="flex items-center justify-center py-16">
    <div class="text-center">
      <div class="w-8 h-8 border-2 border-slate-200 border-t-slate-500 rounded-full animate-spin mx-auto mb-3"></div>
      <p class="text-slate-500 text-sm">Loading audit...</p>
    </div>
  </div>

<!-- Setup required -->
{:else if setupRequired}
  <div class="bg-amber-50 border border-amber-200 rounded-lg p-6">
    <h3 class="text-lg font-semibold text-amber-800">Initial setup required</h3>
    <p class="text-amber-700 text-sm mt-1">Create the first owner before viewing audit logs.</p>
    <a class="inline-block mt-4 px-4 py-2 bg-amber-600 text-white text-sm font-medium rounded-md hover:bg-amber-700 transition-colors" href="/setup">
      Set up owner
    </a>
  </div>

<!-- Forbidden -->
{:else if forbidden}
  <EmptyState
    icon={Shield}
    title="Access denied"
    description="Your role cannot view audit logs."
  />

<!-- Not authenticated -->
{:else if !principal}
  <div class="bg-slate-50 border border-slate-200 rounded-lg p-6">
    <h3 class="text-lg font-semibold text-slate-700">Sign in required</h3>
    <p class="text-slate-500 text-sm mt-1">Audit logs require authentication.</p>
    <a class="inline-block mt-4 px-4 py-2 bg-slate-700 text-white text-sm font-medium rounded-md hover:bg-slate-800 transition-colors" href="/login">
      Go to login
    </a>
  </div>

<!-- Main content -->
{:else}
  <div class="space-y-5">
    <!-- Page header -->
    <div>
      <h1 class="text-2xl font-bold text-slate-800 dark:text-slate-100">Audit</h1>
      <p class="text-sm text-slate-500 dark:text-slate-400 mt-1">Review backend and VPN service logs.</p>
    </div>

    <!-- Tab buttons -->
    <div class="flex border-b border-slate-200 dark:border-slate-700">
      <button
        type="button"
        onclick={() => handleTabChange('panel')}
        class="px-4 py-2.5 text-sm font-medium border-b-2 transition-colors {activeTab === 'panel'
          ? 'border-blue-600 text-blue-600 dark:text-blue-400 dark:border-blue-400'
          : 'border-transparent text-slate-500 dark:text-slate-400 hover:text-slate-700 dark:hover:text-slate-200 hover:border-slate-300'}"
      >
        <span class="inline-flex items-center gap-2">
          <FileText class="w-4 h-4" />
          Web UI Backend Logs
        </span>
      </button>
      <button
        type="button"
        onclick={() => handleTabChange('ocserv')}
        class="px-4 py-2.5 text-sm font-medium border-b-2 transition-colors {activeTab === 'ocserv'
          ? 'border-blue-600 text-blue-600 dark:text-blue-400 dark:border-blue-400'
          : 'border-transparent text-slate-500 dark:text-slate-400 hover:text-slate-700 dark:hover:text-slate-200 hover:border-slate-300'}"
      >
        <span class="inline-flex items-center gap-2">
          <FileText class="w-4 h-4" />
          ocserv VPN Logs
        </span>
      </button>
    </div>

    <!-- Log level banner (ocserv tab) -->
    {#if activeTab === 'ocserv' && ocservSource}
      <LogLevelBanner logLevel={ocservSource.logLevel} logLevelLabel={ocservSource.logLevelLabel} />
    {/if}

    <!-- Source unavailable -->
    {#if activeSource && !activeSource.available}
      <div class="bg-slate-50 dark:bg-slate-800/50 border border-slate-200 dark:border-slate-700 rounded-lg p-4 text-center">
        <p class="text-sm text-slate-500 dark:text-slate-400">
          This log source is not configured or the file is not accessible.
        </p>
        <p class="text-xs text-slate-400 dark:text-slate-500 mt-1">
          Expected at: {activeSource.path}
        </p>
      </div>
    {:else}
      <!-- Toolbar -->
      <LogToolbar
        {search}
        {level}
        {timeRange}
        {autoRefresh}
        {isOwner}
        loading={logLoading}
        onSearchChange={handleSearchChange}
        onLevelChange={handleLevelChange}
        onTimeRangeChange={handleTimeRangeChange}
        onAutoRefreshToggle={handleAutoRefreshToggle}
        onRefresh={handleRefresh}
        onExport={handleExport}
      />

      <!-- Error banner -->
      {#if logError}
        <div class="flex items-center justify-between gap-3 rounded-lg border border-red-200 dark:border-red-800 bg-red-50 dark:bg-red-950/30 px-4 py-3">
          <p class="text-sm text-red-700 dark:text-red-300">{logError}</p>
          <button
            type="button"
            onclick={() => (logError = '')}
            class="text-red-400 hover:text-red-600 text-sm font-medium"
          >
            Dismiss
          </button>
        </div>
      {/if}

      <!-- Log table -->
      <LogTable
        {lines}
        {total}
        {offset}
        {limit}
        {hasMore}
        loading={logLoading}
        onPageChange={handlePageChange}
      />
    {/if}
  </div>
{/if}
