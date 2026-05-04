<script lang="ts">
  type Props = {
    title: string;
    subtitle?: string;
    latestValue?: string;
    /** Explicit pixel height for the chart container.
     *  Required because LayerChart uses `height: 100%` which only resolves
     *  when the parent has a definite height. Without this, the container
     *  collapses to 0 px after mount and the chart becomes invisible. */
    chartHeight?: number;
    children: import('svelte').Snippet;
    actions?: import('svelte').Snippet;
  };

  let { title, subtitle, latestValue, chartHeight, children, actions }: Props = $props();
</script>

<div class="relative flex flex-col min-w-0 break-words w-full mb-6 shadow-lg rounded-lg bg-white">
  <!-- Header -->
  <div class="rounded-t px-5 py-4 border-b border-slate-100">
    <div class="flex items-center justify-between">
      <div class="relative w-full max-w-full flex-grow flex-1">
        <h6 class="uppercase text-slate-400 mb-1 text-xs font-semibold">
          {title}
        </h6>
        {#if subtitle}
          <h2 class="text-slate-700 text-xl font-semibold">
            {subtitle}
          </h2>
        {/if}
      </div>
      <div class="flex items-center gap-3">
        {#if latestValue}
          <span class="text-sm font-semibold text-slate-600">{latestValue}</span>
        {/if}
        {#if actions}
          {@render actions()}
        {/if}
      </div>
    </div>
  </div>
  <!-- Chart body -->
  <div class="px-4 py-3 flex-auto">
    {#if chartHeight}
      <div style="height: {chartHeight}px">
        {@render children()}
      </div>
    {:else}
      {@render children()}
    {/if}
  </div>
</div>
