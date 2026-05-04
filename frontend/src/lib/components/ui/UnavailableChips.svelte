<script lang="ts">
  export type Chip = {
    name: string;    // display label on the chip
    reason: string;  // shown on hover via title attribute
  };

  type Props = {
    chips: Chip[];
  };

  let { chips }: Props = $props();
</script>

{#if chips.length > 0}
  <div class="mt-4 flex flex-wrap items-center gap-2">
    <span class="text-xs font-semibold uppercase tracking-wider text-slate-400">Unavailable:</span>
    {#each chips as chip}
      <span
        class="inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium bg-slate-100 text-slate-400 border border-slate-200 cursor-default"
        title={chip.reason}
      >
        {chip.name}
      </span>
    {/each}
  </div>
{/if}
