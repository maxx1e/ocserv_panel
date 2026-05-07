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
  <div
    role="button"
    tabindex="0"
    onclick={() => (expanded = !expanded)}
    onkeydown={(e) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); expanded = !expanded; } }}
    class="w-full flex items-center gap-3 px-4 py-2 text-left text-sm cursor-pointer"
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
  </div>

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
