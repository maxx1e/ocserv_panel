# Audit Page Frontend Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement the frontend Audit page with tab-based log viewing, search/filter/export, auto-refresh, and pagination — matching the existing admin UI style.

**Architecture:** Replace the EmptyState placeholder at `/audit` with a full-featured log viewer. Add API client functions to `api.ts`. Create sub-components for toolbar, log table, log rows, and level warning banner.

**Tech Stack:** SvelteKit, Svelte 5 runes, Tailwind CSS, lucide-svelte icons

**Spec:** `docs/superpowers/specs/2026-05-07-audit-page-design.md`

**Depends on:** Backend plan (`2026-05-07-audit-page-backend.md`) must be completed first — API endpoints must exist.

---

## File Structure

### Modified Files

| File | Change |
|------|--------|
| `frontend/src/lib/api.ts` | Add audit log types and API functions |
| `frontend/src/routes/audit/+page.svelte` | Replace EmptyState with full implementation |

### New Files

| File | Responsibility |
|------|---------------|
| `frontend/src/routes/audit/LogToolbar.svelte` | Search, level filter, time range, auto-refresh toggle, export button |
| `frontend/src/routes/audit/LogTable.svelte` | Log line table with pagination |
| `frontend/src/routes/audit/LogRow.svelte` | Single expandable log entry with copy button |
| `frontend/src/routes/audit/LogLevelBanner.svelte` | Sensitivity warning for high ocserv log levels |

---

## Task 1: API Client Types and Functions

**Files:**
- Modify: `frontend/src/lib/api.ts`

- [ ] **Step 1: Add types at end of file before utility functions**

Add after the `BulkResult` type and bulk operation functions, before the `// Utility Functions` section:

```typescript
// ─── Audit Log Types ──────────────────────────────────────────────────────

export type AuditLogLine = {
  timestamp: string;
  level: string;
  message: string;
  component: string;
  raw: string;
  metadata?: Record<string, string>;
};

export type AuditLogResponse = {
  source: string;
  lines: AuditLogLine[];
  total: number;
  offset: number;
  limit: number;
  hasMore: boolean;
};

export type AuditLogSource = {
  name: string;
  label: string;
  available: boolean;
  path: string;
  reader: string;
  logLevel: number | null;
  logLevelLabel: string | null;
};

export type AuditLogSourcesResponse = {
  sources: AuditLogSource[];
};

export type AuditLogParams = {
  source: string;
  search?: string;
  level?: string;
  from?: string;
  to?: string;
  limit?: number;
  offset?: number;
  order?: 'asc' | 'desc';
};
```

- [ ] **Step 2: Add API functions**

Add after the types:

```typescript
// ─── Audit Log API ────────────────────────────────────────────────────────

export function getAuditLogs(params: AuditLogParams): Promise<AuditLogResponse> {
  const query = new URLSearchParams();
  query.set('source', params.source);
  if (params.search) query.set('search', params.search);
  if (params.level) query.set('level', params.level);
  if (params.from) query.set('from', params.from);
  if (params.to) query.set('to', params.to);
  if (params.limit) query.set('limit', String(params.limit));
  if (params.offset) query.set('offset', String(params.offset));
  if (params.order) query.set('order', params.order);
  return apiFetch<AuditLogResponse>(`/api/audit/logs?${query.toString()}`);
}

export function getAuditLogSources(): Promise<AuditLogSourcesResponse> {
  return apiFetch<AuditLogSourcesResponse>('/api/audit/logs/sources');
}

export async function exportAuditLogs(params: AuditLogParams & { format?: string }): Promise<Blob> {
  const query = new URLSearchParams();
  query.set('source', params.source);
  if (params.search) query.set('search', params.search);
  if (params.level) query.set('level', params.level);
  if (params.from) query.set('from', params.from);
  if (params.to) query.set('to', params.to);
  if (params.format) query.set('format', params.format);
  query.set('limit', '5000');

  const response = await fetch(`/api/audit/logs/export?${query.toString()}`, {
    credentials: 'same-origin'
  });
  if (!response.ok) {
    throw new ApiError(
      response.status === 403 ? 'Export requires owner permissions.' : 'Export failed.',
      response.status
    );
  }
  return response.blob();
}
```

- [ ] **Step 3: Verify TypeScript compiles**

Run: `cd /opt/ocserv_panel/frontend && npx svelte-check --threshold error 2>&1 | tail -5`
Expected: No errors in api.ts.

- [ ] **Step 4: Commit**

```bash
git add frontend/src/lib/api.ts
git commit -m "feat(frontend): add audit log API types and functions"
```

---

## Task 2: LogLevelBanner Component

**Files:**
- Create: `frontend/src/routes/audit/LogLevelBanner.svelte`

- [ ] **Step 1: Create the component**

```svelte
<script lang="ts">
  import { AlertTriangle } from 'lucide-svelte';

  type Props = {
    logLevel: number | null;
    logLevelLabel: string | null;
  };

  let { logLevel, logLevelLabel }: Props = $props();

  const isSensitive = $derived(logLevel !== null && logLevel >= 8);

  const levelDescriptions: Record<number, string> = {
    0: 'default/info',
    1: 'basic',
    2: 'info',
    3: 'debug',
    4: 'http',
    8: 'sensitive (may contain credentials)',
    9: 'TLS (may contain keys and handshake data)'
  };
</script>

{#if isSensitive}
  <div class="flex items-start gap-3 rounded-lg border border-amber-200 bg-amber-50 dark:border-amber-800 dark:bg-amber-950/30 p-4">
    <AlertTriangle class="w-5 h-5 text-amber-600 dark:text-amber-400 shrink-0 mt-0.5" />
    <div>
      <p class="text-sm font-medium text-amber-800 dark:text-amber-200">
        ocserv log-level is set to {logLevel} ({logLevelLabel ?? levelDescriptions[logLevel ?? 0] ?? 'unknown'})
      </p>
      <p class="text-xs text-amber-700 dark:text-amber-300 mt-1">
        Logs at this level may contain sensitive data including credentials, session tokens, and TLS handshake details. Exercise caution when sharing or exporting.
      </p>
    </div>
  </div>
{:else if logLevel !== null}
  <div class="flex items-center gap-2 text-xs text-slate-500 dark:text-slate-400">
    <span>ocserv log-level: {logLevel} ({logLevelLabel ?? levelDescriptions[logLevel] ?? 'unknown'})</span>
  </div>
{/if}
```

- [ ] **Step 2: Verify no TypeScript errors**

Run: `cd /opt/ocserv_panel/frontend && npx svelte-check --threshold error 2>&1 | grep -c "Error"` (expect 0)

- [ ] **Step 3: Commit**

```bash
git add frontend/src/routes/audit/LogLevelBanner.svelte
git commit -m "feat(frontend): add LogLevelBanner component for ocserv sensitivity warning"
```

---

## Task 3: LogRow Component

**Files:**
- Create: `frontend/src/routes/audit/LogRow.svelte`

- [ ] **Step 1: Create the component**

```svelte
<script lang="ts">
  import { Copy, ChevronDown, ChevronRight } from 'lucide-svelte';
  import type { AuditLogLine } from '$lib/api';

  type Props = {
    line: AuditLogLine;
  };

  let { line }: Props = $props();
  let expanded = $state(false);
  let copied = $state(false);

  const levelColors: Record<string, string> = {
    debug: 'bg-slate-100 text-slate-600 dark:bg-slate-800 dark:text-slate-400',
    info: 'bg-blue-100 text-blue-700 dark:bg-blue-900/40 dark:text-blue-300',
    warn: 'bg-amber-100 text-amber-700 dark:bg-amber-900/40 dark:text-amber-300',
    error: 'bg-red-100 text-red-700 dark:bg-red-900/40 dark:text-red-300'
  };

  function formatTime(ts: string): string {
    if (!ts) return '—';
    const d = new Date(ts);
    return d.toLocaleTimeString('en-US', { hour12: false, hour: '2-digit', minute: '2-digit', second: '2-digit' })
      + '.' + String(d.getMilliseconds()).padStart(3, '0');
  }

  function formatDate(ts: string): string {
    if (!ts) return '';
    const d = new Date(ts);
    return d.toLocaleDateString('en-US', { month: 'short', day: 'numeric' });
  }

  async function copyRaw() {
    try {
      await navigator.clipboard.writeText(line.raw || line.message);
      copied = true;
      setTimeout(() => (copied = false), 2000);
    } catch {
      // fallback
    }
  }
</script>

<div class="border-b border-slate-100 dark:border-slate-800 hover:bg-slate-50/50 dark:hover:bg-slate-800/30 transition-colors">
  <!-- Main row -->
  <button
    type="button"
    onclick={() => (expanded = !expanded)}
    class="w-full flex items-center gap-3 px-4 py-2 text-left text-sm"
  >
    <span class="text-slate-400 dark:text-slate-500 shrink-0">
      {#if expanded}
        <ChevronDown class="w-3.5 h-3.5" />
      {:else}
        <ChevronRight class="w-3.5 h-3.5" />
      {/if}
    </span>

    <span class="text-xs text-slate-400 dark:text-slate-500 font-mono w-20 shrink-0" title={line.timestamp}>
      {formatTime(line.timestamp)}
    </span>

    <span class="text-xs text-slate-400 dark:text-slate-500 font-mono w-14 shrink-0">
      {formatDate(line.timestamp)}
    </span>

    <span class="inline-flex items-center px-1.5 py-0.5 rounded text-xs font-medium w-12 justify-center shrink-0 {levelColors[line.level] ?? levelColors['info']}">
      {line.level}
    </span>

    {#if line.component}
      <span class="text-xs text-slate-500 dark:text-slate-400 font-mono w-20 shrink-0 truncate" title={line.component}>
        {line.component}
      </span>
    {/if}

    <span class="text-sm text-slate-700 dark:text-slate-300 truncate flex-1">
      {line.message}
    </span>

    <button
      type="button"
      onclick={(e) => { e.stopPropagation(); copyRaw(); }}
      class="shrink-0 p-1 rounded text-slate-400 hover:text-slate-600 dark:hover:text-slate-300 hover:bg-slate-100 dark:hover:bg-slate-700 transition-colors"
      title={copied ? 'Copied!' : 'Copy log line'}
    >
      <Copy class="w-3.5 h-3.5" />
    </button>
  </button>

  <!-- Expanded detail -->
  {#if expanded}
    <div class="px-4 pb-3 pl-12">
      <pre class="text-xs font-mono text-slate-600 dark:text-slate-400 bg-slate-50 dark:bg-slate-800/50 rounded p-3 overflow-x-auto whitespace-pre-wrap break-all">{line.raw}</pre>
      {#if line.metadata && Object.keys(line.metadata).length > 0}
        <div class="mt-2 flex flex-wrap gap-2">
          {#each Object.entries(line.metadata) as [key, value]}
            <span class="inline-flex items-center gap-1 px-2 py-0.5 rounded bg-slate-100 dark:bg-slate-700 text-xs text-slate-600 dark:text-slate-300">
              <span class="font-medium">{key}:</span> {value}
            </span>
          {/each}
        </div>
      {/if}
    </div>
  {/if}
</div>
```

- [ ] **Step 2: Commit**

```bash
git add frontend/src/routes/audit/LogRow.svelte
git commit -m "feat(frontend): add LogRow expandable log entry component"
```

---

## Task 4: LogTable Component

**Files:**
- Create: `frontend/src/routes/audit/LogTable.svelte`

- [ ] **Step 1: Create the component**

```svelte
<script lang="ts">
  import { ChevronLeft, ChevronRight } from 'lucide-svelte';
  import type { AuditLogLine } from '$lib/api';
  import LogRow from './LogRow.svelte';

  type Props = {
    lines: AuditLogLine[];
    total: number;
    offset: number;
    limit: number;
    hasMore: boolean;
    loading: boolean;
    onPageChange: (newOffset: number) => void;
  };

  let { lines, total, offset, limit, hasMore, loading, onPageChange }: Props = $props();

  const currentPage = $derived(Math.floor(offset / limit) + 1);
  const totalPages = $derived(Math.ceil(total / limit));

  function prevPage() {
    const newOffset = Math.max(0, offset - limit);
    onPageChange(newOffset);
  }

  function nextPage() {
    if (hasMore) {
      onPageChange(offset + limit);
    }
  }
</script>

<div class="border border-slate-200 dark:border-slate-700 rounded-lg overflow-hidden">
  <!-- Header -->
  <div class="bg-slate-50 dark:bg-slate-800/50 border-b border-slate-200 dark:border-slate-700 px-4 py-2 flex items-center gap-4">
    <span class="text-xs font-medium text-slate-500 dark:text-slate-400 uppercase tracking-wide w-20">Time</span>
    <span class="text-xs font-medium text-slate-500 dark:text-slate-400 uppercase tracking-wide w-14">Date</span>
    <span class="text-xs font-medium text-slate-500 dark:text-slate-400 uppercase tracking-wide w-12">Level</span>
    <span class="text-xs font-medium text-slate-500 dark:text-slate-400 uppercase tracking-wide w-20">Source</span>
    <span class="text-xs font-medium text-slate-500 dark:text-slate-400 uppercase tracking-wide flex-1">Message</span>
  </div>

  <!-- Loading overlay -->
  {#if loading}
    <div class="flex items-center justify-center py-12">
      <div class="w-6 h-6 border-2 border-slate-200 border-t-slate-500 rounded-full animate-spin"></div>
    </div>
  {:else if lines.length === 0}
    <div class="flex flex-col items-center justify-center py-12 text-center">
      <p class="text-sm text-slate-500 dark:text-slate-400">No log entries match your filters.</p>
      <p class="text-xs text-slate-400 dark:text-slate-500 mt-1">Try adjusting the search, level, or time range.</p>
    </div>
  {:else}
    <!-- Log lines -->
    <div class="max-h-[600px] overflow-y-auto">
      {#each lines as line (line.raw + line.timestamp)}
        <LogRow {line} />
      {/each}
    </div>
  {/if}

  <!-- Pagination footer -->
  {#if total > 0}
    <div class="bg-slate-50 dark:bg-slate-800/50 border-t border-slate-200 dark:border-slate-700 px-4 py-2 flex items-center justify-between">
      <span class="text-xs text-slate-500 dark:text-slate-400">
        {total.toLocaleString()} total entries
      </span>
      <div class="flex items-center gap-2">
        <button
          type="button"
          onclick={prevPage}
          disabled={offset === 0}
          class="p-1 rounded text-slate-500 hover:text-slate-700 dark:text-slate-400 dark:hover:text-slate-200 disabled:opacity-30 disabled:cursor-not-allowed transition-colors"
        >
          <ChevronLeft class="w-4 h-4" />
        </button>
        <span class="text-xs text-slate-600 dark:text-slate-300 font-medium">
          {currentPage} / {totalPages}
        </span>
        <button
          type="button"
          onclick={nextPage}
          disabled={!hasMore}
          class="p-1 rounded text-slate-500 hover:text-slate-700 dark:text-slate-400 dark:hover:text-slate-200 disabled:opacity-30 disabled:cursor-not-allowed transition-colors"
        >
          <ChevronRight class="w-4 h-4" />
        </button>
      </div>
    </div>
  {/if}
</div>
```

- [ ] **Step 2: Commit**

```bash
git add frontend/src/routes/audit/LogTable.svelte
git commit -m "feat(frontend): add LogTable component with pagination"
```

---

## Task 5: LogToolbar Component

**Files:**
- Create: `frontend/src/routes/audit/LogToolbar.svelte`

- [ ] **Step 1: Create the component**

```svelte
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

  let searchInput = $state(search);
  let debounceTimer: ReturnType<typeof setTimeout> | null = null;

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
```

- [ ] **Step 2: Commit**

```bash
git add frontend/src/routes/audit/LogToolbar.svelte
git commit -m "feat(frontend): add LogToolbar with search, filters, and actions"
```

---

## Task 6: Main Audit Page

**Files:**
- Modify: `frontend/src/routes/audit/+page.svelte`

- [ ] **Step 1: Replace the page content**

```svelte
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
    type AuditLogSource
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
      const params: Record<string, string | number> = {
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

      const result = await getAuditLogs(params as any);
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
      const params: Record<string, string> = { source: activeTab };
      if (search) params.search = search;
      if (level) params.level = level;
      const timeParams = getTimeParams();
      if (timeParams.from) params.from = timeParams.from;
      if (timeParams.to) params.to = timeParams.to;
      params.format = 'json';

      const blob = await exportAuditLogs(params as any);
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
```

- [ ] **Step 2: Verify TypeScript compiles**

Run: `cd /opt/ocserv_panel/frontend && npx svelte-check --threshold error 2>&1 | tail -10`
Expected: No errors.

- [ ] **Step 3: Verify build succeeds**

Run: `cd /opt/ocserv_panel/frontend && npm run build 2>&1 | tail -5`
Expected: Build success.

- [ ] **Step 4: Commit**

```bash
git add frontend/src/routes/audit/+page.svelte
git commit -m "feat(frontend): implement full Audit page with log viewing UI"
```

---

## Task 7: Final Frontend Verification

- [ ] **Step 1: Check TypeScript**

Run: `cd /opt/ocserv_panel/frontend && npx svelte-check --threshold error`
Expected: No errors.

- [ ] **Step 2: Build**

Run: `cd /opt/ocserv_panel/frontend && npm run build`
Expected: Build succeeds.

- [ ] **Step 3: Verify all new files exist**

Run: `ls -la frontend/src/routes/audit/`
Expected: `+page.svelte`, `LogToolbar.svelte`, `LogTable.svelte`, `LogRow.svelte`, `LogLevelBanner.svelte`

---
