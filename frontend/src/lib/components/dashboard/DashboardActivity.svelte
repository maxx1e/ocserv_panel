<script lang="ts">
  import CardTable from '$lib/components/ui/CardTable.svelte';
  import PaginatedTable from '$lib/components/ui/PaginatedTable.svelte';
  import { formatRelativeTime } from '$lib/utils/format';
  import { paginate } from '$lib/utils/pagination';
  import {
    getLogTail,
    type AuditEvent,
    type LogTailResult
  } from '$lib/api';

  type Props = {
    recentAudit: AuditEvent[];
  };

  let { recentAudit }: Props = $props();

  let auditPage = $state(1);
  let activeLogTab = $state<'audit' | 'ocserv' | 'panel'>('audit');
  let ocservLog = $state<LogTailResult | null>(null);
  let panelLog = $state<LogTailResult | null>(null);
  let logLoading = $state(false);
  let logError = $state('');

  const pagedAudit = $derived(paginate(recentAudit, auditPage, 10));

  async function loadLogTab(source: 'ocserv' | 'panel') {
    logLoading = true;
    logError = '';
    try {
      const result = await getLogTail(source, 25);
      if (source === 'ocserv') ocservLog = result;
      else panelLog = result;
    } catch (caught) {
      logError = caught instanceof Error ? caught.message : 'Unable to load log.';
    } finally {
      logLoading = false;
    }
  }

  function handleLogTabChange(tab: 'audit' | 'ocserv' | 'panel') {
    activeLogTab = tab;
    if (tab === 'ocserv') void loadLogTab('ocserv');
    if (tab === 'panel') void loadLogTab('panel');
  }
</script>

<section aria-labelledby="audit-logs-heading" class="mb-8">
  <h2 id="audit-logs-heading" class="sr-only">Audit &amp; Logs</h2>

  <!-- Tab bar -->
  <div class="flex gap-1 mb-4 border-b border-slate-200">
    {#each ([['audit', 'Audit Events'], ['ocserv', 'ocserv Logs'], ['panel', 'Panel Logs']] as const) as [tab, label]}
      <button
        type="button"
        onclick={() => handleLogTabChange(tab)}
        class="px-4 py-2 text-sm font-medium rounded-t-md transition-colors {activeLogTab === tab ? 'bg-white border border-b-white border-slate-200 -mb-px text-slate-800' : 'text-slate-500 hover:text-slate-700'}"
        aria-selected={activeLogTab === tab}
        role="tab"
      >
        {label}
      </button>
    {/each}
  </div>

  <!-- Audit Events tab -->
  {#if activeLogTab === 'audit'}
    {#if recentAudit.length > 0}
      <CardTable title="Recent Audit">
        <PaginatedTable
          totalPages={pagedAudit.totalPages}
          currentPage={pagedAudit.page}
          onPageChange={(p) => (auditPage = p)}
        >
          <table class="w-full text-sm">
            <thead class="bg-slate-50">
              <tr class="text-left text-slate-500 font-medium">
                <th class="px-6 py-3">Time</th>
                <th class="px-6 py-3">Actor</th>
                <th class="px-6 py-3">Action</th>
                <th class="px-6 py-3">Result</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-slate-100">
              {#each pagedAudit.items as event}
                <tr class="hover:bg-slate-50">
                  <td class="px-6 py-3 text-slate-700 whitespace-nowrap" title={event.createdAt ?? event.timestamp ?? ''}>{formatRelativeTime(event.createdAt ?? event.timestamp)}</td>
                  <td class="px-6 py-3 text-slate-700">{event.actorUsername ?? event.actor ?? '—'}</td>
                  <td class="px-6 py-3 text-slate-700">{event.action ?? event.message ?? '—'}</td>
                  <td class="px-6 py-3">
                    <span class="inline-block px-2 py-0.5 text-xs font-medium rounded-full {event.outcome === 'success' || event.result === 'success' ? 'bg-emerald-50 text-emerald-700' : event.outcome === 'failure' || event.result === 'failure' ? 'bg-red-50 text-red-700' : 'bg-slate-100 text-slate-600'}">
                      {event.outcome ?? event.result ?? '—'}
                    </span>
                  </td>
                </tr>
              {/each}
            </tbody>
          </table>
        </PaginatedTable>
      </CardTable>
    {:else}
      <p class="text-sm text-slate-500">No audit events yet.</p>
    {/if}
  {/if}

  <!-- ocserv Logs tab -->
  {#if activeLogTab === 'ocserv'}
    {#if logLoading}
      <div class="flex items-center gap-2 py-8 text-slate-500 text-sm">
        <div class="w-4 h-4 border-2 border-slate-200 border-t-slate-500 rounded-full animate-spin"></div>
        Loading ocserv log...
      </div>
    {:else if ocservLog}
      {#if !ocservLog.enabled}
        <div class="bg-slate-50 border border-slate-200 rounded-lg p-4 text-sm text-slate-600">
          Log file reading is disabled.
          <a href="/settings#monitoring" class="underline text-slate-700 ml-1">Enable it in Settings &rarr; Monitoring.</a>
        </div>
      {:else if !ocservLog.present}
        <div class="bg-amber-50 border border-amber-200 rounded-lg p-4 text-sm text-amber-700">
          Log file not found at <code class="font-mono">{ocservLog.path}</code>.
          <a href="/settings#installation" class="underline text-amber-800 ml-1">You can update the path in Settings &rarr; Installation.</a>
        </div>
      {:else if ocservLog.error}
        <div class="bg-red-50 border border-red-200 rounded-lg p-4 text-sm text-red-700">
          {ocservLog.error}
        </div>
      {:else}
        <div class="bg-white border border-slate-200 rounded-lg">
          <div class="flex items-center justify-between px-4 py-2 border-b border-slate-100">
            <span class="text-xs text-slate-500 font-mono">{ocservLog.path}</span>
            <button
              type="button"
              onclick={() => loadLogTab('ocserv')}
              disabled={logLoading}
              class="text-xs text-slate-500 hover:text-slate-700 disabled:opacity-50"
            >Refresh</button>
          </div>
          <pre class="text-xs font-mono text-slate-700 p-4 overflow-x-auto max-h-64 overflow-y-auto">{ocservLog.lines.join('\n')}</pre>
        </div>
      {/if}
    {:else if logError}
      <div class="bg-red-50 border border-red-200 rounded-lg p-4 text-sm text-red-700">
        {logError}
      </div>
    {/if}
  {/if}

  <!-- Panel Logs tab -->
  {#if activeLogTab === 'panel'}
    {#if logLoading}
      <div class="flex items-center gap-2 py-8 text-slate-500 text-sm">
        <div class="w-4 h-4 border-2 border-slate-200 border-t-slate-500 rounded-full animate-spin"></div>
        Loading panel log...
      </div>
    {:else if panelLog}
      {#if !panelLog.enabled}
        <div class="bg-slate-50 border border-slate-200 rounded-lg p-4 text-sm text-slate-600">
          Log file reading is disabled.
          <a href="/settings#monitoring" class="underline text-slate-700 ml-1">Enable it in Settings &rarr; Monitoring.</a>
        </div>
      {:else if !panelLog.present}
        <div class="bg-amber-50 border border-amber-200 rounded-lg p-4 text-sm text-amber-700">
          Log file not found at <code class="font-mono">{panelLog.path}</code>.
          <a href="/settings#installation" class="underline text-amber-800 ml-1">You can update the path in Settings &rarr; Installation.</a>
        </div>
      {:else if panelLog.error}
        <div class="bg-red-50 border border-red-200 rounded-lg p-4 text-sm text-red-700">
          {panelLog.error}
        </div>
      {:else}
        <div class="bg-white border border-slate-200 rounded-lg">
          <div class="flex items-center justify-between px-4 py-2 border-b border-slate-100">
            <span class="text-xs text-slate-500 font-mono">{panelLog.path}</span>
            <button
              type="button"
              onclick={() => loadLogTab('panel')}
              disabled={logLoading}
              class="text-xs text-slate-500 hover:text-slate-700 disabled:opacity-50"
            >Refresh</button>
          </div>
          <pre class="text-xs font-mono text-slate-700 p-4 overflow-x-auto max-h-64 overflow-y-auto">{panelLog.lines.join('\n')}</pre>
        </div>
      {/if}
    {:else if logError}
      <div class="bg-red-50 border border-red-200 rounded-lg p-4 text-sm text-red-700">
        {logError}
      </div>
    {/if}
  {/if}
</section>
