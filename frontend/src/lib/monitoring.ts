import type {
  MonitoringCatalog,
  MonitoringMetric,
  MonitoringOverview,
  MonitoringPoint,
  MonitoringSeries
} from './api';

const UNAVAILABLE_VALUE = '--';
const BYTE_UNITS_SI = ['B', 'KB', 'MB', 'GB', 'TB'] as const;
const BYTE_UNITS_IEC = ['B', 'KiB', 'MiB', 'GiB', 'TiB'] as const;

// ---------------------------------------------------------------------------
// Chart color definitions
// Tailwind class strings appear as literals here so Tailwind v4's scanner
// detects them. For SVG gradient fills we provide explicit rgba() values
// because Tailwind v4's color-mix() gradient variables may not render
// correctly on SVG stop-color attributes in all browsers.
// ---------------------------------------------------------------------------

/** Tailwind stroke class and explicit gradient stop colours. */
export type ChartClassSet = { stroke: string; gradientTop: string; gradientBottom: string };

const CHART_COLORS: Record<string, ChartClassSet> = {
  host_cpu_utilization: {
    stroke: 'stroke-indigo-500 stroke-2',
    gradientTop: 'rgba(99, 102, 241, 0.30)',    // indigo-500 / 30 %
    gradientBottom: 'rgba(99, 102, 241, 0.05)'  // indigo-500 / 5 %
  },
  host_memory_utilization: {
    stroke: 'stroke-indigo-500 stroke-2',
    gradientTop: 'rgba(99, 102, 241, 0.30)',
    gradientBottom: 'rgba(99, 102, 241, 0.05)'
  },
  net_interface_rx_bytes: {
    stroke: 'stroke-emerald-500 stroke-2',
    gradientTop: 'rgba(16, 185, 129, 0.30)',    // emerald-500 / 30 %
    gradientBottom: 'rgba(16, 185, 129, 0.05)'
  },
  net_interface_tx_bytes: {
    stroke: 'stroke-sky-500 stroke-2',
    gradientTop: 'rgba(14, 165, 233, 0.30)',    // sky-500 / 30 %
    gradientBottom: 'rgba(14, 165, 233, 0.05)'
  },
  vpn_interface_rx_bytes: {
    stroke: 'stroke-emerald-600 stroke-2',
    gradientTop: 'rgba(5, 150, 105, 0.30)',     // emerald-600 / 30 %
    gradientBottom: 'rgba(5, 150, 105, 0.05)'
  },
  vpn_interface_tx_bytes: {
    stroke: 'stroke-sky-600 stroke-2',
    gradientTop: 'rgba(2, 132, 199, 0.30)',     // sky-600 / 30 %
    gradientBottom: 'rgba(2, 132, 199, 0.05)'
  },
  user_rx_bytes: {
    stroke: 'stroke-teal-500 stroke-2',
    gradientTop: 'rgba(20, 184, 166, 0.30)',    // teal-500 / 30 %
    gradientBottom: 'rgba(20, 184, 166, 0.05)'
  },
  user_tx_bytes: {
    stroke: 'stroke-cyan-500 stroke-2',
    gradientTop: 'rgba(6, 182, 212, 0.30)',     // cyan-500 / 30 %
    gradientBottom: 'rgba(6, 182, 212, 0.05)'
  },
  active_sessions: {
    stroke: 'stroke-violet-500 stroke-2',
    gradientTop: 'rgba(139, 92, 246, 0.30)',    // violet-500 / 30 %
    gradientBottom: 'rgba(139, 92, 246, 0.05)'
  },
  concurrent_sessions: {
    stroke: 'stroke-violet-500 stroke-2',
    gradientTop: 'rgba(139, 92, 246, 0.30)',
    gradientBottom: 'rgba(139, 92, 246, 0.05)'
  },
  connection_rate: {
    stroke: 'stroke-amber-500 stroke-2',
    gradientTop: 'rgba(245, 158, 11, 0.30)',    // amber-500 / 30 %
    gradientBottom: 'rgba(245, 158, 11, 0.05)'
  }
};

const FALLBACK_CHART_COLORS: ChartClassSet = {
  stroke: 'stroke-slate-500 stroke-2',
  gradientTop: 'rgba(100, 116, 139, 0.30)',     // slate-500 / 30 %
  gradientBottom: 'rgba(100, 116, 139, 0.05)'
};

export function metricByName(
  catalog: MonitoringCatalog | undefined | null,
  name: string
): MonitoringMetric | undefined {
  if (!Array.isArray(catalog)) return undefined;
  return catalog.find((metric) => metric.name === name);
}

export function seriesByMetric(
  overview: MonitoringOverview | undefined | null,
  metric: string
): MonitoringSeries | undefined {
  if (!Array.isArray(overview?.series)) return undefined;
  return overview.series.find((series) => series.metric === metric);
}

export function latestPoint(series: MonitoringSeries | undefined | null): MonitoringPoint | undefined {
  if (!Array.isArray(series?.points)) return undefined;
  return series.points.at(-1);
}

export function formatMetricValue(metric: string, value: number | undefined | null): string {
  if (!isFiniteValue(value)) return UNAVAILABLE_VALUE;

  if (isUtilizationMetric(metric)) {
    return `${value.toFixed(1)}%`;
  }

  if (isByteMetric(metric)) {
    return `${formatBytes(value, 'iec')}/s`;
  }

  return formatNumber(value);
}

export function formatBytes(value: number | undefined | null, style: 'iec' | 'si' = 'si'): string {
  if (!isFiniteValue(value)) return UNAVAILABLE_VALUE;

  const sign = value < 0 ? '-' : '';
  let scaled = Math.abs(value);
  let unitIndex = 0;
  const units = style === 'iec' ? BYTE_UNITS_IEC : BYTE_UNITS_SI;

  while (scaled >= 1024 && unitIndex < units.length - 1) {
    scaled /= 1024;
    unitIndex += 1;
  }

  if (unitIndex === 0) {
    return `${sign}${Math.round(scaled).toLocaleString()} ${units[unitIndex]}`;
  }

  return `${sign}${scaled.toFixed(1)} ${units[unitIndex]}`;
}

export function isUtilizationMetric(metric: string): boolean {
  return metric.endsWith('_utilization');
}

export function isByteMetric(metric: string): boolean {
  return metric.includes('bytes') || metric.includes('bandwidth');
}

function isFiniteValue(value: number | undefined | null): value is number {
  return typeof value === 'number' && Number.isFinite(value);
}

// ---------------------------------------------------------------------------
// Chart helpers
// ---------------------------------------------------------------------------

/**
 * Compact byte formatter for chart axis ticks.
 * Produces abbreviated values like "1.2G", "500M", "128K" — one letter per
 * SI prefix; unlike `formatBytes` which produces verbose labels like "1.2 GiB".
 */
export function formatAxisValue(metric: string, value: number): string {
  if (isUtilizationMetric(metric)) {
    return `${Math.round(value)}%`;
  }
  if (isByteMetric(metric)) {
    return `${formatBytesCompact(value)}/s`;
  }
  // count / session metrics
  if (Math.abs(value) >= 10000) {
    return `${(value / 1000).toFixed(0)}k`;
  }
  if (Math.abs(value) >= 1000) {
    return `${(value / 1000).toFixed(1)}k`;
  }
  return String(Math.round(value));
}

function formatBytesCompact(value: number): string {
  if (!Number.isFinite(value)) return '0';
  if (value === 0) return '0';
  const units = ['', 'K', 'M', 'G', 'T'];
  let unitIndex = 0;
  let scaled = Math.abs(value);
  while (scaled >= 1024 && unitIndex < units.length - 1) {
    scaled /= 1024;
    unitIndex++;
  }
  const sign = value < 0 ? '-' : '';
  if (unitIndex === 0) return `${sign}${Math.round(scaled)}`;
  return `${sign}${scaled.toFixed(1)}${units[unitIndex]}`;
}

/** Returns stroke class and explicit rgba() gradient stop colours for a metric. */
export function getChartClasses(metric: string): ChartClassSet {
  return CHART_COLORS[metric] ?? FALLBACK_CHART_COLORS;
}

/** Returns y-axis domain and formatting configuration for a given metric. */
export function getChartConfig(metric: string): {
  yDomain: [number, number | null];
  yNice: boolean;
  yAxisFormat: (value: number) => string;
} {
  if (isUtilizationMetric(metric)) {
    return {
      yDomain: [0, 100],
      yNice: false,
      yAxisFormat: (v: number) => `${Math.round(v)}%`
    };
  }
  return {
    yDomain: [0, null],
    yNice: true,
    yAxisFormat: (v: number) => formatAxisValue(metric, v)
  };
}

/**
 * Formats a Date for display on the chart X-axis.
 * Short format "HH:MM" for compact 160px charts.
 */
export function formatXAxisTime(d: Date | number): string {
  const date = d instanceof Date ? d : new Date(d);
  const hours = date.getHours().toString().padStart(2, '0');
  const minutes = date.getMinutes().toString().padStart(2, '0');
  return `${hours}:${minutes}`;
}

/**
 * Returns the number of ticks for the X (time) axis.
 * Capped at 4-5 to prevent overlap in compact 160px-height charts.
 */
export function getXTickCount(dataLength: number): number {
  if (dataLength <= 30) return Math.min(dataLength, 3);
  if (dataLength <= 60) return 4;
  return 5;
}

function formatNumber(value: number): string {
  return new Intl.NumberFormat(undefined, { maximumFractionDigits: 2 }).format(value);
}

/**
 * Returns the CSS variable color string for a metric, suitable for
 * use in ChartConfig color fields.
 */
export function getChartColorVar(metric: string): string {
  const map: Record<string, string> = {
    host_cpu_utilization: 'var(--chart-1)',
    host_memory_utilization: 'var(--chart-1)',
    net_interface_rx_bytes: 'var(--chart-2)',
    net_interface_tx_bytes: 'var(--chart-3)',
    vpn_interface_rx_bytes: 'var(--chart-2)',
    vpn_interface_tx_bytes: 'var(--chart-3)',
    user_rx_bytes: 'var(--chart-2)',
    user_tx_bytes: 'var(--chart-3)',
    active_sessions: 'var(--chart-4)',
    concurrent_sessions: 'var(--chart-4)',
    connection_rate: 'var(--chart-5)',
  };
  return map[metric] ?? 'var(--chart-1)';
}
