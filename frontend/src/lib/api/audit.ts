import { ApiError } from './_client';
import { apiFetch } from './_client';

export type AuditEvent = {
  id?: string | number;
  action?: string;
  actor?: string;
  actorUsername?: string;
  outcome?: string;
  result?: string;
  createdAt?: string;
  timestamp?: string;
  message?: string;
};

export type AuditLogLine = {
  timestamp: string;
  level: string;
  message: string;
  component: string;
  raw: string;
  metadata?: Record<string, string>;
};

export type AuditLogResponse = {
  source: string;
  lines: AuditLogLine[];
  total: number;
  offset: number;
  limit: number;
  hasMore: boolean;
};

export type AuditLogSource = {
  name: string;
  label: string;
  available: boolean;
  path: string;
  reader: string;
  reason: string;
  logLevel: number | null;
  logLevelLabel: string | null;
};

export type AuditLogSourcesResponse = {
  sources: AuditLogSource[];
};

export type AuditLogParams = {
  source: string;
  search?: string;
  level?: string;
  from?: string;
  to?: string;
  limit?: number;
  offset?: number;
  order?: 'asc' | 'desc';
};

export function getAuditLogs(params: AuditLogParams): Promise<AuditLogResponse> {
  const query = new URLSearchParams();
  query.set('source', params.source);
  if (params.search) query.set('search', params.search);
  if (params.level) query.set('level', params.level);
  if (params.from) query.set('from', params.from);
  if (params.to) query.set('to', params.to);
  if (params.limit) query.set('limit', String(params.limit));
  if (params.offset) query.set('offset', String(params.offset));
  if (params.order) query.set('order', params.order);
  return apiFetch<AuditLogResponse>(`/api/audit/logs?${query.toString()}`);
}

export function getAuditLogSources(): Promise<AuditLogSourcesResponse> {
  return apiFetch<AuditLogSourcesResponse>('/api/audit/logs/sources');
}

export async function exportAuditLogs(params: AuditLogParams & { format?: string }): Promise<Blob> {
  const query = new URLSearchParams();
  query.set('source', params.source);
  if (params.search) query.set('search', params.search);
  if (params.level) query.set('level', params.level);
  if (params.from) query.set('from', params.from);
  if (params.to) query.set('to', params.to);
  if (params.format) query.set('format', params.format);
  query.set('limit', '5000');

  const response = await fetch(`/api/audit/logs/export?${query.toString()}`, {
    credentials: 'same-origin'
  });
  if (!response.ok) {
    throw new ApiError(
      response.status === 403 ? 'Export requires owner permissions.' : 'Export failed.',
      response.status
    );
  }
  return response.blob();
}
