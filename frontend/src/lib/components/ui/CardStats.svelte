<script lang="ts">
  import type { Component } from 'svelte';
  import type { Snippet } from 'svelte';

  type Props = {
    title: string;
    value: string;
    subtitle?: string;
    icon?: Component;
    trend?: { direction: 'up' | 'down'; value: string };
    unavailable?: boolean;
    hint?: string;
  };

  let { title, value, subtitle, icon, trend, unavailable = false, hint }: Props = $props();

  const Icon = $derived(icon);
</script>

<article
  class="bg-white rounded-lg shadow-sm p-6 {unavailable ? 'bg-slate-50 border border-dashed border-slate-300' : ''}"
  aria-live="polite"
>
  <div class="flex items-center justify-between">
    <div class="flex-1 min-w-0">
      <p class="text-sm font-semibold text-slate-500 uppercase tracking-wide">{title}</p>
      <p class="mt-1 text-2xl font-bold {unavailable ? 'text-slate-400' : 'text-slate-700'} truncate">{value}</p>
      {#if trend}
        <p class="mt-1 text-sm">
          <span class={trend.direction === 'up' ? 'text-emerald-600' : 'text-red-600'}>
            {trend.direction === 'up' ? '↑' : '↓'} {trend.value}
          </span>
        </p>
      {/if}
      {#if subtitle}
        <p class="mt-1 text-sm text-slate-500">{subtitle}</p>
      {/if}
      {#if hint && unavailable}
        <p class="mt-1 text-xs text-slate-400">{hint}</p>
      {/if}
    </div>
    {#if icon}
      <div class="flex-shrink-0 ml-4">
        <div class="w-12 h-12 rounded-full bg-slate-100 flex items-center justify-center">
          <Icon class="w-6 h-6 text-slate-600" />
        </div>
      </div>
    {/if}
  </div>
</article>
