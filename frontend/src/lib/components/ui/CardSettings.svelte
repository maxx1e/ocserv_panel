<script lang="ts">
  type Props = {
    id?: string;
    title: string;
    description?: string;
    children: import('svelte').Snippet;
  };

  let { id, title, description, children }: Props = $props();
</script>

<section id={id} class="bg-white rounded-lg shadow-sm p-6 scroll-mt-24">
  <p class="text-xs font-bold uppercase tracking-wider text-slate-500">{title}</p>
  <h3 class="text-lg font-semibold text-slate-800 mt-1">{title}</h3>
  {#if description}
    <p class="text-sm text-slate-500 mt-1">{description}</p>
  {/if}
  <div class="mt-4 space-y-4">
    {@render children()}
  </div>
</section>
