<script lang="ts">
  import type { Snippet } from 'svelte';

  type Props = {
    totalPages: number;
    currentPage: number;
    onPageChange: (page: number) => void;
    children: Snippet;
  };

  let { totalPages, currentPage, onPageChange, children }: Props = $props();

  const showPagination = $derived(totalPages > 1);
  const canPrev = $derived(currentPage > 1);
  const canNext = $derived(currentPage < totalPages);
</script>

{@render children()}

{#if showPagination}
  <div class="flex items-center justify-between px-6 py-3 border-t border-slate-100 bg-white rounded-b-lg">
    <button
      type="button"
      disabled={!canPrev}
      onclick={() => onPageChange(currentPage - 1)}
      class="inline-flex items-center px-3 py-1.5 text-sm font-medium rounded-md border border-slate-200 text-slate-600 bg-white hover:bg-slate-50 disabled:opacity-40 disabled:cursor-not-allowed transition-colors"
    >
      ← Prev
    </button>
    <span class="text-sm text-slate-500">
      Page {currentPage} of {totalPages}
    </span>
    <button
      type="button"
      disabled={!canNext}
      onclick={() => onPageChange(currentPage + 1)}
      class="inline-flex items-center px-3 py-1.5 text-sm font-medium rounded-md border border-slate-200 text-slate-600 bg-white hover:bg-slate-50 disabled:opacity-40 disabled:cursor-not-allowed transition-colors"
    >
      Next →
    </button>
  </div>
{/if}
