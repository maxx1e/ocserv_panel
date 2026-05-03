<script lang="ts">
  import type { Component } from 'svelte';

  type Props = {
    icon?: Component;
    title: string;
    description: string;
    items?: string[];
  };

  let { icon, title, description, items = [] }: Props = $props();

  const Icon = $derived(icon);
</script>

<div class="bg-white rounded-lg shadow-sm p-8 text-center">
  {#if icon}
    <div class="mx-auto w-16 h-16 rounded-full bg-slate-100 flex items-center justify-center mb-4">
      <Icon class="w-8 h-8 text-slate-400" />
    </div>
  {/if}
  <h3 class="text-lg font-semibold text-slate-700">{title}</h3>
  <p class="text-sm text-slate-500 mt-1">{description}</p>
  {#if items.length > 0}
    <ul class="mt-4 text-sm text-slate-500 text-left max-w-md mx-auto space-y-1">
      {#each items as item}
        <li class="flex items-start gap-2">
          <span class="text-slate-400 mt-0.5">•</span>
          <span>{item}</span>
        </li>
      {/each}
    </ul>
  {/if}
</div>
