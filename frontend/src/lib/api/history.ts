import { apiFetch } from './_client';

export type HistoricalSession = {
  id?: string | number;
  username?: string;
  user?: string;
  connectedAt?: string;
  disconnectedAt?: string;
  startedAt?: string;
  endedAt?: string;
  state?: string;
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

export type HistoricalSessionPage = {
  sessions: HistoricalSession[];
  total: number;
  page: number;
  limit: number;
};

export type TrafficBucket = {
  timestamp: string;
  bytesIn: number;
  bytesOut: number;
};

export type UserTrafficResponse = {
  username: string;
  window: string;
  buckets: TrafficBucket[];
};

export type BanEvent = {
  id: number;
  ip: string;
  score: number;
  reason: string;
  bannedAt: string;
  unbannedAt: string | null;
};

export type BanEventPage = {
  events: BanEvent[];
  total: number;
  page: number;
  limit: number;
};

export function listSessions(window: string, page = 1, limit = 25, username = ''): Promise<HistoricalSessionPage> {
  const params = new URLSearchParams({
    window,
    page: String(page),
    limit: String(limit),
  });
  if (username) params.set('username', username);
  return apiFetch<HistoricalSessionPage>(`/api/history/sessions?${params.toString()}`);
}

export function getUserTraffic(username: string, window: string): Promise<UserTrafficResponse> {
  return apiFetch<UserTrafficResponse>(
    `/api/history/users/${encodeURIComponent(username)}/traffic?window=${encodeURIComponent(window)}`
  );
}

export function listHistoricalBans(page = 1, limit = 25, window = 'all'): Promise<BanEventPage> {
  return apiFetch<BanEventPage>(`/api/history/bans?page=${page}&limit=${limit}&window=${encodeURIComponent(window)}`);
}
