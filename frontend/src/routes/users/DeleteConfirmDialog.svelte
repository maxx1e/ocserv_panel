<script lang="ts">
  import { AlertTriangle } from 'lucide-svelte';

  type Props = {
    open: boolean;
    itemName: string;
    onConfirm: () => Promise<void>;
  };

  let { open = $bindable(), itemName, onConfirm }: Props = $props();

  let submitting = $state(false);
  let error = $state('');

  async function handleConfirm() {
    submitting = true;
    error = '';
    try {
      await onConfirm();
      open = false;
    } catch (caught) {
      error = caught instanceof Error ? caught.message : 'Deletion failed.';
    } finally {
      submitting = false;
    }
  }

  function handleCancel() {
    error = '';
    open = false;
  }
</script>

{#if open}
<div class="fixed inset-0 z-50 flex items-center justify-center" role="dialog" aria-modal="true" aria-labelledby="delete-dialog-title">
  <button type="button" class="fixed inset-0 bg-black/50 cursor-default" onclick={handleCancel} aria-label="Close dialog"></button>
  <div class="relative bg-white dark:bg-slate-900 rounded-lg shadow-xl w-full max-w-md p-6 m-4">
    <div class="flex items-center gap-3 mb-4">
      <div class="w-10 h-10 rounded-full bg-red-100 dark:bg-red-900/30 flex items-center justify-center flex-shrink-0">
        <AlertTriangle class="w-5 h-5 text-red-600 dark:text-red-400" />
      </div>
      <h2 id="delete-dialog-title" class="text-lg font-semibold text-slate-900 dark:text-slate-100">Confirm deletion</h2>
    </div>

    <p class="text-sm text-slate-600 dark:text-slate-400 mb-4">
      Are you sure you want to delete <strong class="text-slate-900 dark:text-slate-100">{itemName}</strong>? This action cannot be undone.
    </p>

    {#if error}
      <div class="mb-4 px-3 py-2 bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 rounded-lg text-sm text-red-700 dark:text-red-300" role="alert">
        {error}
      </div>
    {/if}

    <div class="flex justify-end gap-3">
      <button
        type="button"
        onclick={handleCancel}
        disabled={submitting}
        class="px-4 py-2 bg-slate-200 dark:bg-slate-700 text-slate-700 dark:text-slate-200 rounded-lg hover:bg-slate-300 dark:hover:bg-slate-600 disabled:opacity-50 text-sm font-medium transition-colors"
      >
        Cancel
      </button>
      <button
        type="button"
        onclick={handleConfirm}
        disabled={submitting}
        class="px-4 py-2 bg-red-600 text-white rounded-lg hover:bg-red-700 disabled:opacity-50 text-sm font-medium transition-colors"
      >
        {submitting ? 'Deleting...' : 'Delete'}
      </button>
    </div>
  </div>
</div>
{/if}
