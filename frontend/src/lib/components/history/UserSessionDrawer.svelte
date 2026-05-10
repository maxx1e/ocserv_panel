<script lang="ts">
  import { X } from 'lucide-svelte';
  import MetricChart from '$lib/components/ui/MetricChart.svelte';
  import { formatBytes } from '$lib/monitoring';
  import { formatRelativeTime } from '$lib/utils/format';
  import type { HistoricalSession } from '$lib/api';

  type Props = {
    username: string;
    window: string;
    sessions: HistoricalSession[];
    onclose: () => void;
  };

  let { username, window: timeWindow, sessions, onclose }: Props = $props();

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

  function sessionDuration(s: HistoricalSession): string {
    const secs = s.durationSeconds ?? 0;
    if (secs === 0) return '—';
    const h = Math.floor(secs / 3600);
    const m = Math.floor((secs % 3600) / 60);
    if (h > 0) return `${h}h ${m}m`;
    return `${m}m`;
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
  class="fixed right-0 top-0 h-full w-full max-w-2xl bg-white shadow-xl z-50 flex flex-col overflow-hidden"
  role="dialog"
  aria-modal="true"
  aria-label="{username} session history"
>
  <!-- Header -->
  <div class="flex items-center justify-between px-6 py-4 border-b border-slate-200 flex-shrink-0">
    <div>
      <h2 class="text-lg font-semibold text-slate-800">{username}</h2>
      <p class="text-sm text-slate-500">{windowLabel}</p>
    </div>
    <button
      type="button"
      onclick={onclose}
      class="p-2 rounded-md text-slate-400 hover:text-slate-600 hover:bg-slate-100 transition-colors"
      aria-label="Close"
    >
      <X class="w-5 h-5" />
    </button>
  </div>

  <!-- Scrollable content -->
  <div class="flex-1 overflow-y-auto px-6 py-4 space-y-6">
    {#if sessions.length === 0}
      <p class="text-sm text-slate-500 text-center py-8">No sessions in this period.</p>
    {:else}
      <!-- Chart 1: Data usage per session -->
      <div>
        <h3 class="text-sm font-medium text-slate-700 mb-2">Data usage per session</h3>
        <div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
          <div>
            <p class="text-xs text-slate-500 mb-1">Download (RX)</p>
            <MetricChart data={rxData} metric="net_interface_rx_bytes" height={120} />
          </div>
          <div>
            <p class="text-xs text-slate-500 mb-1">Upload (TX)</p>
            <MetricChart data={txData} metric="net_interface_tx_bytes" height={120} />
          </div>
        </div>
      </div>

      <!-- Chart 2: Session duration -->
      <div>
        <h3 class="text-sm font-medium text-slate-700 mb-2">Session duration (minutes)</h3>
        <MetricChart data={durationData} metric="active_sessions" height={120} />
      </div>

      <!-- Chart 3: Cumulative data -->
      <div>
        <h3 class="text-sm font-medium text-slate-700 mb-2">Cumulative data transferred</h3>
        <MetricChart data={cumulativeData} metric="net_interface_rx_bytes" height={120} />
      </div>

      <!-- Session list -->
      <div>
        <h3 class="text-sm font-medium text-slate-700 mb-2">Sessions</h3>
        <div class="overflow-x-auto rounded-lg border border-slate-200">
          <table class="w-full text-sm">
            <thead class="bg-slate-50">
              <tr class="text-left text-slate-500 font-medium text-xs">
                <th class="px-4 py-2">Started</th>
                <th class="px-4 py-2">Ended</th>
                <th class="px-4 py-2">Duration</th>
                <th class="px-4 py-2">RX</th>
                <th class="px-4 py-2">TX</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-slate-100">
              {#each sortedSessions as session (session.id ?? session.connectedAt ?? session.startedAt)}
                <tr class="hover:bg-slate-50">
                  <td class="px-4 py-2 text-slate-700" title={session.connectedAt ?? session.startedAt ?? ''}>
                    {formatRelativeTime(session.connectedAt ?? session.startedAt)}
                  </td>
                  <td class="px-4 py-2 text-slate-700" title={session.disconnectedAt ?? session.endedAt ?? ''}>
                    {formatRelativeTime(session.disconnectedAt ?? session.endedAt)}
                  </td>
                  <td class="px-4 py-2 text-slate-700">{sessionDuration(session)}</td>
                  <td class="px-4 py-2 text-slate-700">{formatBytes(session.bytesIn ?? session.rxBytes ?? 0)}</td>
                  <td class="px-4 py-2 text-slate-700">{formatBytes(session.bytesOut ?? session.txBytes ?? 0)}</td>
                </tr>
              {/each}
            </tbody>
          </table>
        </div>
      </div>
    {/if}
  </div>
</div>
