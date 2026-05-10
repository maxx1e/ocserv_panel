<script lang="ts">
  type Props = {
    pendingChanges?: string[] | null;
    onRestart: () => void;
    restarting: boolean;
    labelMap?: Map<string, string>;
  };

  let { pendingChanges = [], onRestart, restarting, labelMap }: Props = $props();
</script>

{#if pendingChanges && pendingChanges.length > 0}
  <div class="bg-amber-50 border border-amber-200 rounded-lg p-4">
    <p class="font-semibold text-amber-800">Restart required</p>
    <p class="text-sm text-amber-700 mt-1">
      {restarting
        ? 'Restarting... the page will reload when the service is back.'
        : 'Configuration changes need a service restart to take effect.'}
    </p>
    {#if !restarting}
      <ul class="mt-2 text-sm text-amber-700 list-disc list-inside">
        {#each pendingChanges as change}
          <li>{labelMap?.get(change) ?? change}</li>
        {/each}
      </ul>
      <button
        type="button"
        onclick={onRestart}
        class="mt-3 px-4 py-2 bg-red-600 text-white text-sm font-semibold rounded-lg hover:bg-red-700 transition-colors"
      >
        Restart Now
      </button>
    {/if}
  </div>
{/if}
