<script lang="ts">
  import { X, AlertTriangle, Info, Wrench } from 'lucide-svelte';
  import type { ConfigIssue } from '$lib/api';

  let {
    issues = [] as ConfigIssue[],
    onjump,
    onfix,
  }: {
    issues: ConfigIssue[];
    onjump?: (line: number) => void;
    onfix?: (issue: ConfigIssue) => void;
  } = $props();
</script>

{#if issues.length > 0}
  <div class="bg-white rounded-lg shadow-sm border border-slate-200 p-4">
    <h3 class="text-sm font-semibold text-slate-700 mb-3">Issues ({issues.length})</h3>
    <div class="space-y-2">
      {#each issues as issue}
        <div class="flex items-start gap-2 text-sm">
          {#if issue.severity === 'error'}
            <X class="w-4 h-4 mt-0.5 flex-shrink-0 text-red-500" />
          {:else if issue.severity === 'warning'}
            <AlertTriangle class="w-4 h-4 mt-0.5 flex-shrink-0 text-amber-500" />
          {:else}
            <Info class="w-4 h-4 mt-0.5 flex-shrink-0 text-blue-500" />
          {/if}
          <div class="flex-1 min-w-0">
            <div class="flex items-center gap-2 flex-wrap">
              {#if issue.line}
                <button
                  type="button"
                  onclick={() => onjump?.(issue.line!)}
                  class="inline-flex items-center px-1.5 py-0.5 text-xs font-mono font-medium text-blue-700 bg-blue-50 rounded hover:bg-blue-100 transition-colors"
                  title="Jump to line {issue.line}"
                >
                  L{issue.line}
                </button>
              {/if}
              <span class="font-medium text-slate-700">{issue.key}:</span>
              <span class="text-slate-600">{issue.message}</span>
            </div>
            {#if issue.suggestion}
              <p class="text-xs text-slate-500 mt-0.5">{issue.suggestion}</p>
            {/if}
            {#if issue.fixedValue}
              <button
                type="button"
                onclick={() => onfix?.(issue)}
                class="inline-flex items-center gap-1 mt-1 px-2 py-1 text-xs font-medium text-emerald-700 bg-emerald-50 border border-emerald-200 rounded hover:bg-emerald-100 transition-colors"
              >
                <Wrench class="w-3 h-3" />
                Fix: <code class="ml-0.5 font-mono">{issue.fixedValue}</code>
              </button>
            {/if}
          </div>
        </div>
      {/each}
    </div>
  </div>
{/if}
