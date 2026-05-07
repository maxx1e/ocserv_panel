<script lang="ts">
  import { AlertTriangle, X } from 'lucide-svelte';

  type Props = {
    open: boolean;
    action: string;
    count: number;
    variant?: 'default' | 'destructive';
    loading?: boolean;
    onConfirm: () => void;
    onCancel: () => void;
  };

  let { open, action, count, variant = 'default', loading = false, onConfirm, onCancel }: Props = $props();
</script>

{#if open}
  <div class="fixed inset-0 z-50 flex items-center justify-center">
    <div class="absolute inset-0 bg-black/50" onclick={onCancel} role="presentation"></div>

    <div class="relative w-full max-w-md rounded-lg bg-white p-6 shadow-xl dark:bg-slate-800">
      <button
        type="button"
        class="absolute right-3 top-3 text-slate-400 hover:text-slate-600 dark:hover:text-slate-300"
        onclick={onCancel}
        disabled={loading}
      >
        <X class="h-5 w-5" />
      </button>

      <div class="flex items-start gap-3">
        <div class={variant === 'destructive' ? 'text-red-500' : 'text-amber-500'}>
          <AlertTriangle class="h-6 w-6" />
        </div>
        <div class="flex-1">
          <h3 class="text-lg font-semibold text-slate-900 dark:text-white">
            Confirm Bulk {action}
          </h3>
          <p class="mt-2 text-sm text-slate-600 dark:text-slate-400">
            Are you sure you want to {action.toLowerCase()} <strong>{count}</strong> user{count !== 1 ? 's' : ''}? This cannot be undone.
          </p>
        </div>
      </div>

      <div class="mt-6 flex justify-end gap-3">
        <button
          type="button"
          class="rounded-md border border-slate-300 px-4 py-2 text-sm font-medium text-slate-700 hover:bg-slate-50 dark:border-slate-600 dark:text-slate-300 dark:hover:bg-slate-700"
          onclick={onCancel}
          disabled={loading}
        >
          Cancel
        </button>
        <button
          type="button"
          class="{variant === 'destructive' ? 'bg-red-600 hover:bg-red-700' : 'bg-blue-600 hover:bg-blue-700'} rounded-md px-4 py-2 text-sm font-medium text-white disabled:opacity-50"
          onclick={onConfirm}
          disabled={loading}
        >
          {#if loading}
            Processing...
          {:else}
            {action}
          {/if}
        </button>
      </div>
    </div>
  </div>
{/if}
