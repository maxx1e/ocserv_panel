<script lang="ts">
  import type { HealthItem } from '$lib/components/ui/HealthCard.svelte';

  type Props = {
    items: HealthItem[];
  };

  let { items }: Props = $props();

  const counts = $derived({
    healthy: items.filter(i => i.status === 'healthy').length,
    warning: items.filter(i => i.status === 'warning').length,
    critical: items.filter(i => i.status === 'critical').length,
    unknown: items.filter(i => i.status === 'unknown').length,
  });

  const overallStatus = $derived(
    counts.critical > 0 ? 'critical' :
    counts.warning > 0 ? 'warning' :
    counts.unknown > 0 && counts.healthy === 0 ? 'unknown' :
    'healthy'
  );

  const OVERALL_BADGE: Record<string, { label: string; classes: string }> = {
    healthy:  { label: 'All Healthy', classes: 'bg-emerald-100 text-emerald-700' },
    warning:  { label: 'Issues Detected', classes: 'bg-amber-100 text-amber-700' },
    critical: { label: 'Critical', classes: 'bg-red-100 text-red-700' },
    unknown:  { label: 'Unknown', classes: 'bg-slate-100 text-slate-600' },
  };

  const DOT_COLOR: Record<string, string> = {
    healthy: 'bg-emerald-500',
    warning: 'bg-amber-400',
    critical: 'bg-red-500',
    unknown: 'bg-slate-400',
  };
</script>

<div class="bg-slate-50 border border-slate-200 rounded-lg px-4 py-2.5 flex flex-wrap items-center justify-between gap-3 mb-6">
  <span class="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-semibold {OVERALL_BADGE[overallStatus].classes}">
    {OVERALL_BADGE[overallStatus].label}
  </span>
  <div class="flex items-center gap-4 text-xs text-slate-600">
    {#each (['healthy', 'warning', 'critical', 'unknown'] as const) as status}
      <span class="inline-flex items-center gap-1.5 {counts[status] === 0 ? 'opacity-40' : ''}">
        <span class="w-2 h-2 rounded-full {DOT_COLOR[status]}"></span>
        <span class="font-medium">{counts[status]}</span>
        <span class="capitalize">{status}</span>
      </span>
    {/each}
  </div>
</div>
