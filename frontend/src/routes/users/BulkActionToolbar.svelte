<script lang="ts">
  import { X } from 'lucide-svelte';

  type BulkAction = {
    label: string;
    variant: 'default' | 'destructive';
    action: () => void;
  };

  type Props = {
    count: number;
    actions: BulkAction[];
    onClear: () => void;
  };

  let { count, actions, onClear }: Props = $props();
</script>

{#if count > 0}
  <div class="sticky top-0 z-10 flex items-center gap-3 rounded-lg border border-blue-200 bg-blue-50 px-4 py-2.5 shadow-sm dark:border-blue-800 dark:bg-blue-900/30">
    <span class="text-sm font-medium text-blue-800 dark:text-blue-200">
      ✓ {count} user{count !== 1 ? 's' : ''} selected
    </span>

    <div class="ml-auto flex items-center gap-2">
      {#each actions as btn}
        <button
          type="button"
          class="{btn.variant === 'destructive' ? 'bg-red-600 text-white hover:bg-red-700' : 'bg-white text-slate-700 border border-slate-300 hover:bg-slate-50 dark:bg-slate-700 dark:text-slate-200 dark:border-slate-600 dark:hover:bg-slate-600'} rounded-md px-3 py-1.5 text-xs font-medium"
          onclick={btn.action}
        >
          {btn.label}
        </button>
      {/each}

      <button
        type="button"
        class="ml-2 rounded-md p-1.5 text-slate-500 hover:bg-slate-200 dark:text-slate-400 dark:hover:bg-slate-700"
        title="Clear selection"
        onclick={onClear}
      >
        <X class="h-4 w-4" />
      </button>
    </div>
  </div>
{/if}
