<script lang="ts">
  type Props = {
    title: string;
    subtitle?: string;
    latestValue?: string;
    children: import('svelte').Snippet;
    actions?: import('svelte').Snippet;
  };

  let { title, subtitle, latestValue, children, actions }: Props = $props();
</script>

<div class="bg-white rounded-lg shadow-sm">
  <div class="flex items-center justify-between px-6 py-4 border-b border-slate-100">
    <div>
      <h3 class="text-sm font-semibold text-slate-700">{title}</h3>
      {#if subtitle}
        <p class="text-xs text-slate-500 mt-0.5">{subtitle}</p>
      {/if}
    </div>
    <div class="flex items-center gap-3">
      {#if latestValue}
        <span class="text-sm font-semibold text-slate-700">{latestValue}</span>
      {/if}
      {#if actions}
        {@render actions()}
      {/if}
    </div>
  </div>
  <div class="p-4">
    {@render children()}
  </div>
</div>
