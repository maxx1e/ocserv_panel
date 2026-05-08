<script lang="ts">
  import { ShieldOff } from 'lucide-svelte';

  type Props = {
    ip: string;
    submitting: boolean;
    error: string;
    onConfirm: () => void;
    onCancel: () => void;
  };

  let { ip, submitting, error, onConfirm, onCancel }: Props = $props();
</script>

<div
  class="fixed inset-0 bg-black/50 flex items-center justify-center z-50"
  role="dialog"
  aria-modal="true"
  aria-labelledby="unban-ip-title"
>
  <div class="bg-white rounded-lg shadow-xl max-w-md w-full mx-4 p-6">
    <div class="flex items-start gap-3">
      <div class="flex-shrink-0 w-10 h-10 rounded-full bg-blue-100 flex items-center justify-center">
        <ShieldOff class="w-5 h-5 text-blue-600" />
      </div>
      <div class="flex-1">
        <h2 id="unban-ip-title" class="text-lg font-semibold text-slate-800">
          Unban IP address?
        </h2>
        <p class="text-sm text-slate-600 mt-2">
          Remove <strong class="font-mono">{ip}</strong> from the ban list. The IP will be allowed to attempt connections again.
        </p>
      </div>
    </div>

    {#if error}
      <div class="mt-3 px-3 py-2 bg-red-50 rounded text-sm text-red-700" role="alert">
        {error}
      </div>
    {/if}

    <div class="flex items-center justify-end gap-3 mt-6">
      <button
        type="button"
        onclick={onCancel}
        disabled={submitting}
        class="px-4 py-2 text-sm font-medium rounded-md border border-slate-200 text-slate-600 bg-white hover:bg-slate-50 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
      >
        Cancel
      </button>
      <button
        type="button"
        onclick={onConfirm}
        disabled={submitting}
        class="px-4 py-2 text-sm font-medium rounded-md bg-blue-600 text-white hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
      >
        {submitting ? 'Unbanning...' : 'Unban IP'}
      </button>
    </div>
  </div>
</div>
