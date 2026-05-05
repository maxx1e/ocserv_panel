<script lang="ts">
  import { formatRelativeTime } from '$lib/utils/format';

  export type HealthStatus = 'healthy' | 'warning' | 'critical' | 'unknown';

  export type HealthItem = {
    label: string;
    status: HealthStatus;
    detail?: string;
  };

  type Props = {
    title: string;
    items: HealthItem[];
    lastChecked?: Date | null;
    loading?: boolean;
  };

  let { title, items, lastChecked = null, loading = false }: Props = $props();

  const STATUS_DOT: Record<HealthStatus, string> = {
    healthy:  'bg-emerald-500',
    warning:  'bg-amber-400',
    critical: 'bg-red-500',
    unknown:  'bg-slate-400',
  };

  const STATUS_TEXT: Record<HealthStatus, string> = {
    healthy:  'text-emerald-700',
    warning:  'text-amber-700',
    critical: 'text-red-700',
    unknown:  'text-slate-500',
  };

  const STATUS_LABEL: Record<HealthStatus, string> = {
    healthy:  'Healthy',
    warning:  'Warning',
    critical: 'Critical',
    unknown:  'Unknown',
  };

  // Overall card severity: highest status wins
  const overallStatus = $derived<HealthStatus>(
    items.some(i => i.status === 'critical') ? 'critical' :
    items.some(i => i.status === 'warning')  ? 'warning'  :
    items.every(i => i.status === 'healthy') ? 'healthy'  :
    'unknown'
  );

  const CARD_BORDER: Record<HealthStatus, string> = {
    healthy:  'border-emerald-200',
    warning:  'border-amber-200',
    critical: 'border-red-200',
    unknown:  'border-slate-200',
  };
</script>

<div class="bg-white border {CARD_BORDER[overallStatus]} rounded-lg p-5 shadow-sm">
  <div class="flex items-center justify-between mb-4">
    <h3 class="text-sm font-semibold text-slate-700">{title}</h3>
    {#if loading}
      <span class="text-xs text-slate-400">Checking…</span>
    {:else if lastChecked}
      <span class="text-xs text-slate-400">Checked {formatRelativeTime(lastChecked.toISOString())}</span>
    {/if}
  </div>

  <ul class="space-y-3">
    {#each items as item}
      <li class="flex items-start gap-2.5">
        <span
          class="mt-1 w-2.5 h-2.5 rounded-full flex-shrink-0 {STATUS_DOT[item.status]}"
          role="img"
          aria-label="{item.status}"
        ></span>
        <div class="min-w-0 flex-1">
          <div class="flex items-baseline justify-between gap-2">
            <span class="text-sm text-slate-700">{item.label}</span>
            <span class="text-xs font-medium {STATUS_TEXT[item.status]}">{STATUS_LABEL[item.status]}</span>
          </div>
          {#if item.detail}
            <p class="text-xs text-slate-500 mt-0.5 break-words">{item.detail}</p>
          {/if}
        </div>
      </li>
    {/each}
  </ul>
</div>
