<script lang="ts">
  import type { ComponentType, SvelteComponent } from 'svelte';
  import StatusDot from '$lib/components/ui/StatusDot.svelte';

  type Props = {
    title: string;
    value: string;
    subtitle?: string;
    icon?: ComponentType<SvelteComponent>;
    trend?: { direction: 'up' | 'down'; value: string };
    unavailable?: boolean;
    hint?: string;
    statusValue?: string;
  };

  let { title, value, subtitle, icon, trend, unavailable = false, hint, statusValue }: Props = $props();
</script>

<article
  class="bg-card rounded-lg shadow-sm p-6 {unavailable ? 'bg-muted/50 border border-dashed border-border' : ''}"
  aria-live="polite"
>
  <div class="flex items-center justify-between">
    <div class="flex-1 min-w-0">
      <p class="text-sm font-semibold text-muted-foreground uppercase tracking-wide">{title}</p>
      <div class="mt-1 flex items-center gap-2">
        <p class="text-2xl font-bold {unavailable ? 'text-muted-foreground' : 'text-card-foreground'} truncate">{value}</p>
        {#if statusValue}
          <StatusDot status={statusValue} size="md" />
        {/if}
      </div>
      {#if trend}
        <p class="mt-1 text-sm">
          <span class={trend.direction === 'up' ? 'text-emerald-600' : 'text-red-600'}>
            {trend.direction === 'up' ? '↑' : '↓'} {trend.value}
          </span>
        </p>
      {/if}
      {#if subtitle}
        <p class="mt-1 text-sm text-muted-foreground">{subtitle}</p>
      {/if}
      {#if hint && unavailable}
        <p class="mt-1 text-xs text-muted-foreground">{hint}</p>
      {/if}
    </div>
    {#if icon}
      {@const Icon = icon}
      <div class="flex-shrink-0 ml-4">
        <div class="w-12 h-12 rounded-full bg-muted flex items-center justify-center">
          <Icon class="w-6 h-6 text-muted-foreground" />
        </div>
      </div>
    {/if}
  </div>
</article>
