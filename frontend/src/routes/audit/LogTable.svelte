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
  <div class="bg-slate-50 dark:bg-slate-800/50 border-b border-slate-200 dark:border-slate-700 px-4 py-2 flex items-center gap-3">
    <span class="w-4"></span>
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
