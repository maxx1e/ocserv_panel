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

  // For single-series: derive stats from data. For multi-line: flatten all points.
  const allPoints = $derived<TimeData[]>(
    data
      ? data
      : (lines ?? []).flatMap(l => l.data)
  );

  const stats = $derived(seriesStats(allPoints));
  const latest = $derived(allPoints.length > 0 ? allPoints[allPoints.length - 1].value : null);

  // Trend: difference between latest value and the window average, formatted
  const trendValue = $derived(
    latest !== null && stats.avg !== 0
      ? latest - stats.avg
      : null
  );
  const trendFormatted = $derived(
    trendValue !== null
      ? `${trendValue >= 0 ? '▲' : '▼'} ${formatMetricValue(metric, Math.abs(trendValue))} vs avg`
      : null
  );

  // Color dot uses gradientTop at higher opacity
  const dotColor = $derived(classes.gradientTop.replace(/[\d.]+\)$/, '0.8)'));

  const hasData = $derived(allPoints.length > 0);
</script>

<div class="bg-white border border-slate-200 rounded-xl shadow-sm overflow-hidden">
  <!-- Header -->
  <div class="px-4 pt-4 pb-2">
    <div class="flex items-center gap-2 mb-1">
      <span
        class="inline-block w-2.5 h-2.5 rounded-full flex-shrink-0"
        style="background: {dotColor}"
      ></span>
      <span class="text-[11px] font-semibold uppercase tracking-wider text-slate-500">{title}</span>
    </div>
    {#if latest !== null}
      <div class="text-xl font-bold text-slate-800 leading-tight">
        {formatMetricValue(metric, latest)}
      </div>
      {#if trendFormatted}
        {#if trendValue !== null && trendValue >= 0}
          <div class="text-[10px] mt-0.5" style="color: {dotColor}">
            {trendFormatted}
          </div>
        {:else}
          <div class="text-[10px] mt-0.5 text-slate-500">
            {trendFormatted}
          </div>
        {/if}
      {/if}
    {:else}
      <div class="text-xl font-bold text-slate-300">—</div>
    {/if}
  </div>

  <!-- Chart -->
  <div class="px-2 pb-1">
    {#if data !== undefined}
      <MetricChart {data} {metric} {height} {chartType} />
    {:else if lines !== undefined}
      <MultiLineChart {lines} {metric} {height} />
    {/if}
  </div>

  <!-- Footer: min / max / window -->
  <div class="px-4 pb-3">
    {#if hasData}
      <p class="text-[10px] text-slate-400">
        min {formatMetricValue(metric, stats.min)} · max {formatMetricValue(metric, stats.max)} · {w}
      </p>
    {:else}
      <p class="text-[10px] text-slate-400">No data</p>
    {/if}
  </div>
</div>
