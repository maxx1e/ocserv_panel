<script lang="ts">
  import { Search, RefreshCw, Download, Timer } from 'lucide-svelte';

  type Props = {
    search: string;
    level: string;
    timeRange: string;
    autoRefresh: boolean;
    isOwner: boolean;
    loading: boolean;
    onSearchChange: (value: string) => void;
    onLevelChange: (value: string) => void;
    onTimeRangeChange: (value: string) => void;
    onAutoRefreshToggle: () => void;
    onRefresh: () => void;
    onExport: () => void;
  };

  let {
    search,
    level,
    timeRange,
    autoRefresh,
    isOwner,
    loading,
    onSearchChange,
    onLevelChange,
    onTimeRangeChange,
    onAutoRefreshToggle,
    onRefresh,
    onExport
  }: Props = $props();

  let searchInput = $state('');
  let debounceTimer: ReturnType<typeof setTimeout> | null = null;

  $effect(() => {
    searchInput = search;
  });

  function handleSearchInput(e: Event) {
    const target = e.target as HTMLInputElement;
    searchInput = target.value;
    if (debounceTimer) clearTimeout(debounceTimer);
    debounceTimer = setTimeout(() => {
      onSearchChange(searchInput);
    }, 300);
  }

  const levels = [
    { value: '', label: 'All levels' },
    { value: 'debug', label: 'Debug' },
    { value: 'info', label: 'Info' },
    { value: 'warn', label: 'Warning' },
    { value: 'error', label: 'Error' }
  ];

  const timeRanges = [
    { value: '1h', label: 'Last 1 hour' },
    { value: '6h', label: 'Last 6 hours' },
    { value: '24h', label: 'Last 24 hours' },
    { value: '7d', label: 'Last 7 days' },
    { value: '', label: 'All time' }
  ];
</script>

<div class="flex flex-wrap items-center gap-3">
  <!-- Search -->
  <div class="relative flex-1 min-w-[200px] max-w-sm">
    <Search class="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-slate-400" />
    <input
      type="text"
      placeholder="Search logs..."
      value={searchInput}
      oninput={handleSearchInput}
      class="w-full pl-9 pr-3 py-2 text-sm border border-slate-200 dark:border-slate-700 rounded-lg bg-white dark:bg-slate-800 text-slate-700 dark:text-slate-300 placeholder-slate-400 focus:outline-none focus:ring-2 focus:ring-blue-500/20 focus:border-blue-400 transition-colors"
    />
  </div>

  <!-- Level filter -->
  <select
    value={level}
    onchange={(e) => onLevelChange((e.target as HTMLSelectElement).value)}
    class="px-3 py-2 text-sm border border-slate-200 dark:border-slate-700 rounded-lg bg-white dark:bg-slate-800 text-slate-700 dark:text-slate-300 focus:outline-none focus:ring-2 focus:ring-blue-500/20 focus:border-blue-400 transition-colors"
  >
    {#each levels as l}
      <option value={l.value}>{l.label}</option>
    {/each}
  </select>

  <!-- Time range -->
  <select
    value={timeRange}
    onchange={(e) => onTimeRangeChange((e.target as HTMLSelectElement).value)}
    class="px-3 py-2 text-sm border border-slate-200 dark:border-slate-700 rounded-lg bg-white dark:bg-slate-800 text-slate-700 dark:text-slate-300 focus:outline-none focus:ring-2 focus:ring-blue-500/20 focus:border-blue-400 transition-colors"
  >
    {#each timeRanges as tr}
      <option value={tr.value}>{tr.label}</option>
    {/each}
  </select>

  <!-- Auto-refresh toggle -->
  <button
    type="button"
    onclick={onAutoRefreshToggle}
    class="inline-flex items-center gap-1.5 px-3 py-2 text-sm rounded-lg border transition-colors {autoRefresh
      ? 'bg-green-50 border-green-200 text-green-700 dark:bg-green-900/20 dark:border-green-800 dark:text-green-300'
      : 'bg-white border-slate-200 text-slate-600 dark:bg-slate-800 dark:border-slate-700 dark:text-slate-400 hover:bg-slate-50 dark:hover:bg-slate-700'}"
    title={autoRefresh ? 'Auto-refresh on (10s)' : 'Auto-refresh off'}
  >
    <Timer class="w-4 h-4" />
    <span class="hidden sm:inline">{autoRefresh ? 'Live' : 'Auto'}</span>
  </button>

  <!-- Manual refresh -->
  <button
    type="button"
    onclick={onRefresh}
    disabled={loading}
    class="p-2 rounded-lg border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-800 text-slate-600 dark:text-slate-400 hover:bg-slate-50 dark:hover:bg-slate-700 disabled:opacity-50 transition-colors"
    title="Refresh"
  >
    <RefreshCw class="w-4 h-4 {loading ? 'animate-spin' : ''}" />
  </button>

  <!-- Export (owner only) -->
  {#if isOwner}
    <button
      type="button"
      onclick={onExport}
      class="inline-flex items-center gap-1.5 px-3 py-2 text-sm rounded-lg border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-800 text-slate-600 dark:text-slate-400 hover:bg-slate-50 dark:hover:bg-slate-700 transition-colors"
      title="Export logs"
    >
      <Download class="w-4 h-4" />
      <span class="hidden sm:inline">Export</span>
    </button>
  {/if}
</div>
