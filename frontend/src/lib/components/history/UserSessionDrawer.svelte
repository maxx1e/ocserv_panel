<script lang="ts">
  import { X } from 'lucide-svelte';
  import MetricChart from '$lib/components/ui/MetricChart.svelte';
  import { formatBytes, formatShortDate } from '$lib/monitoring';
  import { formatRelativeTime, formatAbsoluteTime } from '$lib/utils/format';
  import type { HistoricalSession } from '$lib/api';
  import type { UserSessionSummary } from '$lib/api/history';

  type Props = {
    username: string;
    window: string;
    sessions: HistoricalSession[];
    summary?: UserSessionSummary | null;
    onclose: () => void;
  };

  let { username, window: timeWindow, sessions, summary = null, onclose }: Props = $props();

  const windowLabel = $derived(
    timeWindow === '7d' ? 'last 7 days' : timeWindow === '30d' ? 'last 30 days' : 'last 24 hours'
  );

  const sortedSessions = $derived(
    [...sessions].sort((a, b) => {
      const ta = Date.parse(a.connectedAt ?? a.startedAt ?? '');
      const tb = Date.parse(b.connectedAt ?? b.startedAt ?? '');
      return ta - tb;
    })
  );

  const rxData = $derived(
    sortedSessions
      .filter(s => Number.isFinite(Date.parse(s.connectedAt ?? s.startedAt ?? '')))
      .map(s => ({
        timestamp: new Date(s.connectedAt ?? s.startedAt ?? ''),
        value: s.bytesIn ?? s.rxBytes ?? 0
      }))
  );

  const txData = $derived(
    sortedSessions
      .filter(s => Number.isFinite(Date.parse(s.connectedAt ?? s.startedAt ?? '')))
      .map(s => ({
        timestamp: new Date(s.connectedAt ?? s.startedAt ?? ''),
        value: s.bytesOut ?? s.txBytes ?? 0
      }))
  );

  const durationData = $derived(
    sortedSessions
      .filter(s => Number.isFinite(Date.parse(s.connectedAt ?? s.startedAt ?? '')))
      .map(s => ({
        timestamp: new Date(s.connectedAt ?? s.startedAt ?? ''),
        value: Math.round((s.durationSeconds ?? 0) / 60)
      }))
  );

  const cumulativeData = $derived.by(() => {
    let running = 0;
    return sortedSessions
      .filter(s => Number.isFinite(Date.parse(s.connectedAt ?? s.startedAt ?? '')))
      .map(s => {
        running += (s.bytesIn ?? s.rxBytes ?? 0) + (s.bytesOut ?? s.txBytes ?? 0);
        return { timestamp: new Date(s.connectedAt ?? s.startedAt ?? ''), value: running };
      });
  });

  const statusColors: Record<string, string> = {
    active: 'bg-green-100 text-green-700',
    blocked: 'bg-amber-100 text-amber-700',
    deleted: 'bg-slate-100 text-slate-500',
  };

  function sessionDuration(s: HistoricalSession): string {
    const secs = s.durationSeconds ?? 0;
    if (secs === 0) return '—';
    const h = Math.floor(secs / 3600);
    const m = Math.floor((secs % 3600) / 60);
    if (h > 0) return `${h}h ${m}m`;
    return `${m}m`;
  }

  function sessionThroughput(s: HistoricalSession): string {
    const secs = s.durationSeconds ?? 0;
    if (secs === 0) return '—';
    const bytes = (s.bytesIn ?? s.rxBytes ?? 0) + (s.bytesOut ?? s.txBytes ?? 0);
    return `${formatBytes(Math.round(bytes / secs))}/s`;
  }

  function handleBackdropClick(e: MouseEvent) {
    if (e.target === e.currentTarget) onclose();
  }

  function handleKeydown(e: KeyboardEvent) {
    if (e.key === 'Escape') onclose();
  }
</script>

<svelte:window onkeydown={handleKeydown} />

<!-- Backdrop -->
<div
  class="fixed inset-0 bg-black/40 z-40"
  role="button"
  tabindex="-1"
  aria-label="Close drawer"
  onclick={handleBackdropClick}
  onkeydown={() => {}}
></div>

<!-- Drawer panel -->
<div
  class="fixed right-0 top-0 h-full w-full max-w-2xl bg-white shadow-xl z-50 flex flex-col overflow-hidden dark:bg-slate-900"
  role="dialog"
  aria-modal="true"
  aria-label="{username} session history"
>
  <!-- Header -->
  <div class="flex items-center justify-between px-6 py-4 border-b border-slate-200 dark:border-slate-700 flex-shrink-0">
    <div class="flex items-center gap-3">
      <div class="w-10 h-10 rounded-full bg-indigo-100 text-indigo-700 flex items-center justify-center text-sm font-bold flex-shrink-0">
        {username.slice(0, 1).toUpperCase()}
      </div>
      <div>
        <div class="flex items-center gap-2">
          <h2 class="text-lg font-semibold text-slate-800 dark:text-slate-100">{username}</h2>
          {#if summary}
            <span class="px-2 py-0.5 rounded-full text-xs font-medium {statusColors[summary.status] ?? statusColors.deleted}">
              {summary.status}
            </span>
          {/if}
        </div>
        <p class="text-sm text-slate-500 dark:text-slate-400">{windowLabel}</p>
      </div>
    </div>
    <button
      type="button"
      onclick={onclose}
      class="p-2 rounded-md text-slate-400 hover:text-slate-600 hover:bg-slate-100 dark:hover:bg-slate-800 transition-colors"
      aria-label="Close"
    >
      <X class="w-5 h-5" />
    </button>
  </div>

  <!-- Summary stat bar -->
  {#if summary}
    <div class="px-6 py-3 bg-slate-50 dark:bg-slate-800/50 border-b border-slate-200 dark:border-slate-700 flex-shrink-0">
      <div class="grid grid-cols-4 gap-4">
        <div>
          <p class="text-xs text-slate-500 dark:text-slate-400">Sessions</p>
          <p class="text-sm font-semibold text-slate-800 dark:text-slate-100">{summary.sessionCount}</p>
        </div>
        <div>
          <p class="text-xs text-slate-500 dark:text-slate-400">Total traffic</p>
          <p class="text-sm font-semibold text-slate-800 dark:text-slate-100">{formatBytes(summary.totalBytesIn + summary.totalBytesOut)}</p>
        </div>
        <div>
          <p class="text-xs text-slate-500 dark:text-slate-400">Avg throughput</p>
          <p class="text-sm font-semibold text-slate-800 dark:text-slate-100">
            {summary.avgThroughputBps > 0 ? `${formatBytes(summary.avgThroughputBps)}/s` : '—'}
          </p>
        </div>
        <div>
          <p class="text-xs text-slate-500 dark:text-slate-400">First seen</p>
          <p class="text-sm font-semibold text-slate-800 dark:text-slate-100 text-xs">{formatAbsoluteTime(summary.firstSeenAt)}</p>
        </div>
      </div>
      <div class="grid grid-cols-3 gap-4 mt-2">
        <div>
          <p class="text-xs text-slate-500 dark:text-slate-400">Download</p>
          <p class="text-sm text-slate-700 dark:text-slate-300">{formatBytes(summary.totalBytesIn)}</p>
        </div>
        <div>
          <p class="text-xs text-slate-500 dark:text-slate-400">Upload</p>
          <p class="text-sm text-slate-700 dark:text-slate-300">{formatBytes(summary.totalBytesOut)}</p>
        </div>
        <div>
          <p class="text-xs text-slate-500 dark:text-slate-400">Total connected time</p>
          <p class="text-sm text-slate-700 dark:text-slate-300">{
            (() => {
              const secs = summary.totalDurationSeconds;
              const h = Math.floor(secs / 3600);
              const m = Math.floor((secs % 3600) / 60);
              return h > 0 ? `${h}h ${m}m` : `${m}m`;
            })()
          }</p>
        </div>
      </div>
    </div>
  {/if}

  <!-- Scrollable content -->
  <div class="flex-1 overflow-y-auto px-6 py-4 space-y-6">
    {#if sessions.length === 0}
      <p class="text-sm text-slate-500 text-center py-8">No sessions in this period.</p>
    {:else}
      <!-- Chart 1: Data usage per session -->
      <div>
        <h3 class="text-sm font-medium text-slate-700 dark:text-slate-300 mb-2">Data usage per session</h3>
        <div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
          <div>
            <p class="text-xs text-slate-500 mb-1">Download (RX)</p>
            <MetricChart data={rxData} metric="session_bytes" height={120} chartType="bar" xLabelFormat={formatShortDate} gridDashed={true} />
          </div>
          <div>
            <p class="text-xs text-slate-500 mb-1">Upload (TX)</p>
            <MetricChart data={txData} metric="session_bytes" height={120} chartType="bar" xLabelFormat={formatShortDate} gridDashed={true} />
          </div>
        </div>
      </div>

      <!-- Chart 2: Session duration -->
      <div>
        <h3 class="text-sm font-medium text-slate-700 dark:text-slate-300 mb-2">Session duration (minutes)</h3>
        <MetricChart data={durationData} metric="session_duration_minutes" height={120} chartType="bar" xLabelFormat={formatShortDate} gridDashed={true} />
      </div>

      <!-- Chart 3: Cumulative data -->
      <div>
        <h3 class="text-sm font-medium text-slate-700 dark:text-slate-300 mb-2">Cumulative data transferred</h3>
        <MetricChart data={cumulativeData} metric="session_bytes" height={120} chartType="bar" xLabelFormat={formatShortDate} gridDashed={true} />
      </div>

      <!-- Session list -->
      <div>
        <h3 class="text-sm font-medium text-slate-700 dark:text-slate-300 mb-2">Sessions</h3>
        <div class="overflow-x-auto rounded-lg border border-slate-200 dark:border-slate-700">
          <table class="w-full text-sm">
            <thead class="bg-slate-50 dark:bg-slate-800">
              <tr class="text-left text-slate-500 dark:text-slate-400 font-medium text-xs">
                <th class="px-4 py-2">Started</th>
                <th class="px-4 py-2">Ended</th>
                <th class="px-4 py-2">Duration</th>
                <th class="px-4 py-2">RX</th>
                <th class="px-4 py-2">TX</th>
                <th class="px-4 py-2">Throughput</th>
                <th class="px-4 py-2">State</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-slate-100 dark:divide-slate-700">
              {#each sortedSessions as session (session.id ?? session.connectedAt ?? session.startedAt)}
                {@const stateColors: Record<string, string> = {
                  authenticated: 'bg-green-100 text-green-700',
                  connected: 'bg-green-100 text-green-700',
                  disconnected: 'bg-slate-100 text-slate-600',
                  timed_out: 'bg-amber-100 text-amber-700',
                  error: 'bg-red-100 text-red-700',
                }}
                <tr class="hover:bg-slate-50 dark:hover:bg-slate-800/50">
                  <td class="px-4 py-2 text-slate-700 dark:text-slate-300 text-xs" title={session.connectedAt ?? session.startedAt ?? ''}>
                    {formatAbsoluteTime(session.connectedAt ?? session.startedAt)}
                  </td>
                  <td class="px-4 py-2 text-slate-700 dark:text-slate-300 text-xs" title={session.disconnectedAt ?? session.endedAt ?? ''}>
                    {#if session.state === 'authenticated' || session.state === 'connected'}
                      <span class="inline-flex items-center rounded-full bg-green-100 px-2 py-0.5 text-xs font-medium text-green-800">Active</span>
                    {:else}
                      {formatRelativeTime(session.disconnectedAt ?? session.endedAt)}
                    {/if}
                  </td>
                  <td class="px-4 py-2 text-slate-700 dark:text-slate-300 text-xs">{sessionDuration(session)}</td>
                  <td class="px-4 py-2 text-slate-700 dark:text-slate-300 text-xs">{formatBytes(session.bytesIn ?? session.rxBytes ?? 0)}</td>
                  <td class="px-4 py-2 text-slate-700 dark:text-slate-300 text-xs">{formatBytes(session.bytesOut ?? session.txBytes ?? 0)}</td>
                  <td class="px-4 py-2 text-slate-600 dark:text-slate-400 text-xs">{sessionThroughput(session)}</td>
                  <td class="px-4 py-2 text-xs">
                    <span class="px-2 py-0.5 rounded-full text-xs font-medium {stateColors[session.state ?? 'disconnected'] ?? 'bg-slate-100 text-slate-600'}">
                      {session.state ?? 'disconnected'}
                    </span>
                  </td>
                </tr>
              {/each}
            </tbody>
          </table>
        </div>
      </div>
    {/if}
  </div>
</div>
