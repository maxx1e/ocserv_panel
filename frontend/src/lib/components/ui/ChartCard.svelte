<script lang="ts">
  import MetricChart from './MetricChart.svelte';
  import MultiLineChart from './MultiLineChart.svelte';
  import type { LineData } from './MultiLineChart.svelte';
  import {
    getChartClasses,
    formatMetricValue,
    seriesStats,
  } from '$lib/monitoring';

  type TimeData = { timestamp: Date; value: number };

  type Props = {
    title: string;
    metric: string;
    window: string;
    height?: number;
    chartType?: 'line' | 'bar';
  } & (
    | { data: TimeData[]; lines?: never }
    | { lines: LineData[]; data?: never }
  );

  let { title, metric, window: w, height = 160, chartType = 'line', data, lines }: Props = $props();

  const classes = $derived(getChartClasses(metric));

  const allPoints = $derived<TimeData[]>(
    data ? data : (lines ?? []).flatMap(l => l.data)
  );

  const stats = $derived(seriesStats(allPoints));
  const latest = $derived(
    allPoints.length > 0
      ? allPoints.reduce((max, p) => p.timestamp > max.timestamp ? p : max).value
      : null
  );

  const trendValue = $derived(latest !== null ? latest - stats.avg : null);
  const trendFormatted = $derived(
    trendValue !== null
      ? `${trendValue >= 0 ? '▲' : '▼'} ${formatMetricValue(metric, Math.abs(trendValue))} vs avg`
      : null
  );

  // Derive solid accent color from gradientTop rgba string
  // e.g. "rgba(99,102,241,0.30)" -> "rgba(99,102,241,0.85)" for border/pill
  const accentColor = $derived(classes.gradientTop.replace(/[\d.]+\)$/, '0.85)'));
  // Pill background: very faint version of accent
  const pillBg = $derived(classes.gradientTop.replace(/[\d.]+\)$/, '0.12)'));
  // Trend badge colors
  const trendPositive = $derived((trendValue ?? 0) >= 0);

  const hasData = $derived(allPoints.length > 0);
</script>

<div class="bg-white border border-slate-200 rounded-xl shadow-sm overflow-hidden">
  <!-- Header -->
  <div
    class="bg-slate-50 px-4 py-2.5 flex items-center justify-between"
    style="border-left: 4px solid {accentColor}"
  >
    <span class="text-[11px] font-bold uppercase tracking-wider text-slate-500">{title}</span>
    {#if latest !== null}
      <span
        class="text-[11px] font-bold px-2.5 py-0.5 rounded-full"
        style="color: {accentColor}; background: {pillBg}"
      >
        {formatMetricValue(metric, latest)}
      </span>
    {:else}
      <span class="text-[11px] text-slate-300">&mdash;</span>
    {/if}
  </div>

  <!-- Chart -->
  <div class="px-2 pb-1 pt-1">
    {#if data !== undefined}
      <MetricChart {data} {metric} {height} {chartType} gridDashed={true} />
    {:else if lines !== undefined}
      <MultiLineChart {lines} {metric} {height} />
    {/if}
  </div>

  <!-- Footer -->
  <div class="px-4 pb-3 flex items-center gap-2 flex-wrap">
    {#if hasData && trendFormatted !== null}
      {#if trendPositive}
        <span class="text-[10px] font-medium px-2 py-0.5 rounded-full bg-green-100 text-green-700">
          {trendFormatted}
        </span>
      {:else}
        <span class="text-[10px] font-medium px-2 py-0.5 rounded-full bg-red-100 text-red-700">
          {trendFormatted}
        </span>
      {/if}
    {/if}
    {#if hasData}
      <p class="text-[10px] text-slate-400">
        min {formatMetricValue(metric, stats.min)} &middot; max {formatMetricValue(metric, stats.max)} &middot; {w}
      </p>
    {:else}
      <p class="text-[10px] text-slate-400">No data</p>
    {/if}
  </div>
</div>
