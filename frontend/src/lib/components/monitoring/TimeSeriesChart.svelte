<script lang="ts">
  import type { MonitoringSeries } from '$lib/api';
  import { formatMetricValue } from '$lib/monitoring';

  type ChartPoint = {
    timestamp: string;
    time: number;
    value: number;
  };

  let { title, series, unit, height = 220 }: {
    title: string;
    series?: MonitoringSeries | null;
    unit?: string;
    height?: number;
  } = $props();

  const width = 640;
  const padding = { top: 14, right: 18, bottom: 28, left: 46 };
  const innerWidth = $derived(width - padding.left - padding.right);
  const innerHeight = $derived(Math.max(80, height) - padding.top - padding.bottom);
  const points = $derived(sortedFinitePoints(series));
  const extent = $derived(valueExtent(points));
  const latest = $derived(points.at(-1));
  const linePath = $derived(buildLinePath(points, extent.min, extent.max));
  const areaPath = $derived(buildAreaPath(points, extent.min, extent.max));
  const latestLabel = $derived(latest ? formatValue(series?.metric ?? '', latest.value, unit) : '--');
  const empty = $derived(points.length === 0);

  function sortedFinitePoints(input: MonitoringSeries | null | undefined): ChartPoint[] {
    if (!Array.isArray(input?.points)) return [];

    return input.points
      .map((point) => ({
        timestamp: point.timestamp,
        time: Date.parse(point.timestamp),
        value: point.value
      }))
      .filter((point) => Number.isFinite(point.time) && Number.isFinite(point.value))
      .sort((left, right) => left.time - right.time);
  }

  function valueExtent(chartPoints: ChartPoint[]): { min: number; max: number } {
    if (chartPoints.length === 0) return { min: 0, max: 1 };

    const values = chartPoints.map((point) => point.value);
    let min = Math.min(...values);
    let max = Math.max(...values);

    if (min === max) {
      const spread = Math.max(Math.abs(min) * 0.1, 1);
      min -= spread;
      max += spread;
    }

    return { min, max };
  }

  function xFor(index: number, count: number): number {
    if (count <= 1) return padding.left + innerWidth / 2;
    return padding.left + (index / (count - 1)) * innerWidth;
  }

  function yFor(value: number, min: number, max: number): number {
    const range = max - min || 1;
    return padding.top + (1 - (value - min) / range) * innerHeight;
  }

  function buildLinePath(chartPoints: ChartPoint[], min: number, max: number): string {
    if (chartPoints.length === 0) return '';

    return chartPoints
      .map((point, index) => `${index === 0 ? 'M' : 'L'} ${xFor(index, chartPoints.length).toFixed(2)} ${yFor(point.value, min, max).toFixed(2)}`)
      .join(' ');
  }

  function buildAreaPath(chartPoints: ChartPoint[], min: number, max: number): string {
    if (chartPoints.length === 0) return '';

    const baseY = padding.top + innerHeight;
    const line = buildLinePath(chartPoints, min, max);
    const lastX = xFor(chartPoints.length - 1, chartPoints.length);
    const firstX = xFor(0, chartPoints.length);
    return `${line} L ${lastX.toFixed(2)} ${baseY.toFixed(2)} L ${firstX.toFixed(2)} ${baseY.toFixed(2)} Z`;
  }

  function formatValue(metric: string, value: number, suffix: string | undefined): string {
    const formatted = formatMetricValue(metric, value);
    return suffix && formatted !== '--' ? `${formatted} ${suffix}` : formatted;
  }

  function formatTime(timestamp: string): string {
    const date = new Date(timestamp);
    if (Number.isNaN(date.getTime())) return '—';
    return new Intl.DateTimeFormat(undefined, { hour: '2-digit', minute: '2-digit' }).format(date);
  }
</script>

<article class="chart-card panel">
  <div class="chart-heading">
    <div>
      <h3>{title}</h3>
      {#if series?.labels && Object.keys(series.labels).length > 0}
        <p class="muted">{Object.entries(series.labels).map(([key, value]) => `${key}: ${value}`).join(' · ')}</p>
      {/if}
    </div>
    <strong>{latestLabel}</strong>
  </div>

  {#if empty}
    <div class="unavailable-metric" role="status">
      <strong>No samples available</strong>
      <p>Metric data has not been collected for this window yet.</p>
    </div>
  {:else}
    <svg viewBox={`0 0 ${width} ${Math.max(80, height)}`} role="img" aria-label={`${title} time series chart`} preserveAspectRatio="none">
      <line class="chart-axis" x1={padding.left} y1={padding.top + innerHeight} x2={width - padding.right} y2={padding.top + innerHeight} />
      <line class="chart-axis" x1={padding.left} y1={padding.top} x2={padding.left} y2={padding.top + innerHeight} />
      {#if areaPath}
        <path class="chart-area" d={areaPath} />
      {/if}
      {#if linePath}
        <path class="chart-line" d={linePath} />
      {/if}
      {#each points as point, index}
        <circle class="chart-point" cx={xFor(index, points.length)} cy={yFor(point.value, extent.min, extent.max)} r={points.length === 1 ? 4 : 2.5}>
          <title>{formatTime(point.timestamp)} · {formatValue(series?.metric ?? '', point.value, unit)}</title>
        </circle>
      {/each}
      <text class="chart-tick" x={padding.left} y={Math.max(80, height) - 8}>{formatTime(points[0].timestamp)}</text>
      <text class="chart-tick chart-tick-end" x={width - padding.right} y={Math.max(80, height) - 8}>{formatTime(points.at(-1)?.timestamp ?? '')}</text>
    </svg>
  {/if}
</article>
