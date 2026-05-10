import { apiFetch } from './_client';
import type { AuditEvent } from './audit';
import type { OcservStatus } from './sessions';

export type DashboardSummary = {
  backendStatus?: string;
  ocservStatus?: string;
  activeSessionCount?: number;
  dashboardRefreshInterval?: number;
  recentAudit?: AuditEvent[];
  analytics?: {
    totalSessions?: number;
    sampleSessions?: number;
    totalBytesIn?: number;
    totalBytesOut?: number;
  };
  sampleData?: boolean | { enabled?: boolean; present?: boolean; message?: string };
  ocserv?: OcservStatus;
  setupRequired?: boolean;
  adminCount?: number;
  history?: {
    totalSessions?: number;
    sampleSessions?: number;
    totalBytesIn?: number;
    totalBytesOut?: number;
  };
  [key: string]: unknown;
};

export type UserSessionCount = {
  username: string;
  sessionCount: number;
};

export type ActiveUserSessionsResult = {
  users: UserSessionCount[];
  total: number;
};

export type MonitoringMetricState = 'collecting' | 'unavailable';

export type MonitoringMetric = {
  name: string;
  description: string;
  state: MonitoringMetricState;
  reason?: string;
};

export type MonitoringPoint = {
  timestamp: string;
  value: number;
};

export type MonitoringSeries = {
  metric: string;
  labels: Record<string, string>;
  points: MonitoringPoint[];
};

export type MonitoringCatalog = MonitoringMetric[];

export type MonitoringOverview = {
  window: string;
  generatedAt: string;
  catalog: MonitoringCatalog;
  series: MonitoringSeries[];
};

export type MonitoringWindow = '15m' | '1h' | '6h' | '24h' | '7d' | '30d';

export function getDashboardSummary(): Promise<DashboardSummary> {
  return apiFetch<DashboardSummary>('/api/dashboard/summary');
}

export function getDashboardActiveSessions(): Promise<ActiveUserSessionsResult> {
  return apiFetch<ActiveUserSessionsResult>('/api/dashboard/active-sessions');
}

export function getMonitoringOverview(window: MonitoringWindow): Promise<MonitoringOverview> {
  const query = new URLSearchParams({ window });
  return apiFetch<MonitoringOverview>(`/api/monitoring/overview?${query.toString()}`);
}

export type SystemInfo = {
  cpuCores: number;
  memoryTotalGB: number;
  netInterface: string;
  vpnInterfaces: string[];
};

export function getSystemInfo(): Promise<SystemInfo> {
  return apiFetch<SystemInfo>('/api/monitoring/system-info');
}
