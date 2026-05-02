import type {
  MonitoringCatalog,
  MonitoringMetric,
  MonitoringOverview,
  MonitoringPoint,
  MonitoringSeries
} from './api';

const UNAVAILABLE_VALUE = '--';
const BYTE_UNITS = ['B', 'KiB', 'MiB', 'GiB', 'TiB'] as const;

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
    return formatBytes(value);
  }

  return formatNumber(value);
}

export function formatBytes(value: number | undefined | null): string {
  if (!isFiniteValue(value)) return UNAVAILABLE_VALUE;

  const sign = value < 0 ? '-' : '';
  let scaled = Math.abs(value);
  let unitIndex = 0;

  while (scaled >= 1024 && unitIndex < BYTE_UNITS.length - 1) {
    scaled /= 1024;
    unitIndex += 1;
  }

  if (unitIndex === 0) {
    return `${sign}${Math.round(scaled).toLocaleString()} ${BYTE_UNITS[unitIndex]}`;
  }

  return `${sign}${scaled.toFixed(1)} ${BYTE_UNITS[unitIndex]}`;
}

function isUtilizationMetric(metric: string): boolean {
  return metric.endsWith('_utilization') || metric.includes('cpu') || metric.includes('memory');
}

function isByteMetric(metric: string): boolean {
  return metric.includes('bytes') || metric.includes('bandwidth');
}

function isFiniteValue(value: number | undefined | null): value is number {
  return typeof value === 'number' && Number.isFinite(value);
}

function formatNumber(value: number): string {
  return new Intl.NumberFormat(undefined, { maximumFractionDigits: 2 }).format(value);
}
