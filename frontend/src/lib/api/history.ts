import { apiFetch } from './_client';

export type HistoricalSession = {
  id?: string | number;
  username?: string;
  user?: string;
  connectedAt?: string;
  disconnectedAt?: string;
  startedAt?: string;
  endedAt?: string;
  durationSeconds?: number;
  rxBytes?: number;
  txBytes?: number;
  bytesIn?: number;
  bytesOut?: number;
  remoteAddress?: string;
  [key: string]: unknown;
};

export type HistoryAnalytics = {
  summary?: Record<string, unknown>;
  recentSessions?: HistoricalSession[];
  sessions?: HistoricalSession[];
  sampleData?: boolean | { enabled?: boolean; present?: boolean; sessionCount?: number; message?: string };
  [key: string]: unknown;
};

export type UserSessionSummary = {
  username: string;
  sessionCount: number;
  totalDurationSeconds: number;
  totalBytesIn: number;
  totalBytesOut: number;
  avgBytesIn: number;
  avgBytesOut: number;
};

export function getHistoryAnalytics(): Promise<HistoryAnalytics> {
  return apiFetch<HistoryAnalytics>('/api/history/analytics');
}

export function getHistoryUsers(window: string): Promise<UserSessionSummary[]> {
  return apiFetch<UserSessionSummary[]>(`/api/history/users?window=${encodeURIComponent(window)}`);
}

export function getHistoryUserSessions(username: string, window: string): Promise<HistoricalSession[]> {
  return apiFetch<HistoricalSession[]>(
    `/api/history/users/${encodeURIComponent(username)}?window=${encodeURIComponent(window)}`
  );
}

export function purgeHistory(retentionDays: 30 | 90 | 180 | 365, confirmation: 'PURGE'): Promise<void> {
  return apiFetch<void>('/api/history/purge', {
    method: 'POST',
    body: { retentionDays, confirmation }
  });
}
