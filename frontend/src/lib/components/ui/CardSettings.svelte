<script lang="ts">
  type Props = {
    id?: string;
    title: string;
    label?: string;
    description?: string;
    stub?: boolean;
    children: import('svelte').Snippet;
  };

  let { id, title, label, description, stub = false, children }: Props = $props();
</script>

<section id={id} class="bg-white rounded-lg shadow-sm p-6 scroll-mt-24">
  <p class="text-xs font-bold uppercase tracking-wider text-slate-500">{label ?? title}</p>
  <h3 class="text-lg font-semibold text-slate-800 mt-1">{title}</h3>
  {#if description}
    <p class="text-sm text-slate-500 mt-1">{description}</p>
  {/if}
  <div class="mt-4">
    {#if stub}
      <div class="mb-3 px-3 py-2 bg-slate-50 border border-slate-200 rounded-lg text-xs text-slate-500">
        This feature is not yet implemented. Settings are saved but have no effect.
      </div>
    {/if}
    {@render children()}
  </div>
</section>
