<script lang="ts">
  import type { ComponentType, SvelteComponent } from 'svelte';

  type Props = {
    icon?: ComponentType<SvelteComponent>;
    title: string;
    description: string;
    items?: string[];
  };

  let { icon, title, description, items = [] }: Props = $props();
</script>

<div class="bg-card rounded-lg shadow-sm p-8 text-center">
  {#if icon}
    {@const Icon = icon}
    <div class="mx-auto w-16 h-16 rounded-full bg-muted flex items-center justify-center mb-4">
      <Icon class="w-8 h-8 text-muted-foreground" />
    </div>
  {/if}
  <h3 class="text-lg font-semibold text-card-foreground">{title}</h3>
  <p class="text-sm text-muted-foreground mt-1">{description}</p>
  {#if items.length > 0}
    <ul class="mt-4 text-sm text-muted-foreground text-left max-w-md mx-auto space-y-1">
      {#each items as item}
        <li class="flex items-start gap-2">
          <span class="text-muted-foreground mt-0.5">•</span>
          <span>{item}</span>
        </li>
      {/each}
    </ul>
  {/if}
</div>
