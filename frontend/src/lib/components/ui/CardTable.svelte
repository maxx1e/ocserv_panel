<script lang="ts">
  type Props = {
    title: string;
    actions?: import('svelte').Snippet;
    children: import('svelte').Snippet;
  };

  let { title, actions, children }: Props = $props();
</script>

<div class="bg-white rounded-lg shadow-sm">
  <div class="flex items-center justify-between px-6 py-4 border-b border-slate-100">
    <h3 class="text-sm font-semibold text-slate-700">{title}</h3>
    {#if actions}
      <div>{@render actions()}</div>
    {/if}
  </div>
  <div class="overflow-x-auto">
    {@render children()}
  </div>
</div>
