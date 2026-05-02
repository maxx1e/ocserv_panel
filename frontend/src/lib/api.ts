export type SetupStatus = {
  required: boolean;
};

export type Principal = {
  id?: string | number;
  username: string;
  roles?: string[];
  role?: string;
};

export type SessionSummary = {
  id?: string;
  createdAt?: string;
  expiresAt?: string;
};

export type AuthSummary = {
  principal?: Principal;
  session?: SessionSummary;
  user?: Principal;
  roles?: string[];
};

export type WebUISettings = {
  cookieSecure: boolean;
  httpLoginAllowed: boolean;
  requestSecure: boolean;
  transport: 'http' | 'https';
  warning: string;
};

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

export type DashboardSummary = {
  backendStatus?: string;
  ocservStatus?: string;
  activeSessionCount?: number;
  recentAudit?: AuditEvent[];
  analytics?: Record<string, unknown>;
  sampleData?: boolean | { enabled?: boolean; present?: boolean; message?: string };
  [key: string]: unknown;
};

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

export type MonitoringStep = '15s' | '60s' | '5m' | '1h';

export class ApiError extends Error {
  status: number;
  code?: string;

  constructor(message: string, status: number, code?: string) {
    super(message);
    this.name = 'ApiError';
    this.status = status;
    this.code = code;
  }
}

type RequestOptions = Omit<RequestInit, 'body' | 'credentials'> & {
  body?: unknown;
};

async function apiFetch<T>(path: string, options: RequestOptions = {}): Promise<T> {
  const headers = new Headers(options.headers);
  let body: BodyInit | undefined;

  if (options.body !== undefined) {
    headers.set('content-type', 'application/json');
    body = JSON.stringify(options.body);
  }

  const response = await fetch(path, {
    ...options,
    headers,
    body,
    credentials: 'same-origin'
  });

  if (!response.ok) {
    const structuredError = await safeStructuredError(response);
    throw new ApiError(
      structuredError?.message ?? safeErrorMessage(path, response.status),
      response.status,
      structuredError?.code
    );
  }

  if (response.status === 204) {
    return undefined as T;
  }

  return (await response.json()) as T;
}

type StructuredError = {
  message: string;
  code?: string;
};

async function safeStructuredError(response: Response): Promise<StructuredError | null> {
  const contentType = response.headers.get('content-type') ?? '';
  if (!contentType.toLowerCase().includes('application/json')) return null;

  try {
    const payload: unknown = await response.json();
    if (!isPlainRecord(payload)) return null;

    const parts = ['error', 'message', 'details']
      .map((field) => safeErrorField(payload[field]))
      .filter((value): value is string => Boolean(value));
    const code = safeErrorField(payload.code);

    if (parts.length === 0 && !code) return null;
    const message = parts.length > 0 ? parts.join(' ') : `Backend error code: ${code}`;
    return code ? { message, code } : { message };
  } catch {
    return null;
  }
}

function isPlainRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function safeErrorField(value: unknown): string | null {
  if (typeof value !== 'string') return null;

  const trimmed = value.trim();
  if (!trimmed || trimmed.length > 300) return null;
  if (/password|token|secret|private\s*key|credential|hash/i.test(trimmed)) return null;

  return trimmed;
}

function safeErrorMessage(path: string, status: number): string {
  if (status === 401) return 'Sign in is required or the credentials were not accepted.';
  if (status === 403) return 'Your role is not allowed to perform this action.';
  if (status === 404) return 'The requested backend endpoint was not found.';
  if (status >= 500) return 'The backend could not complete the request. Check service logs for details.';
  if (path.includes('/history/purge')) return 'History purge validation failed. Check the retention window and confirmation.';
  if (path.includes('/setup/owner')) return 'Owner setup failed. Check the username and password requirements.';
  if (path.includes('/auth/login')) return 'Login failed. Check the username and password.';
  return 'The backend rejected the request.';
}

export function getSetupStatus(): Promise<SetupStatus> {
  return apiFetch<SetupStatus>('/api/setup/status');
}

export function getWebUISettings(): Promise<WebUISettings> {
  return apiFetch<WebUISettings>('/api/settings/webui');
}

export function setupOwner(username: string, password: string): Promise<AuthSummary> {
  return apiFetch<AuthSummary>('/api/setup/owner', {
    method: 'POST',
    body: { username, password }
  });
}

export function login(username: string, password: string): Promise<AuthSummary> {
  return apiFetch<AuthSummary>('/api/auth/login', {
    method: 'POST',
    body: { username, password }
  });
}

export function logout(): Promise<void> {
  return apiFetch<void>('/api/auth/logout', { method: 'POST' });
}

export function getMe(): Promise<AuthSummary> {
  return apiFetch<AuthSummary>('/api/auth/me');
}

export function getDashboardSummary(): Promise<DashboardSummary> {
  return apiFetch<DashboardSummary>('/api/dashboard/summary');
}

export function getHistoryAnalytics(): Promise<HistoryAnalytics> {
  return apiFetch<HistoryAnalytics>('/api/history/analytics');
}

export function getMonitoringCatalog(): Promise<MonitoringCatalog> {
  return apiFetch<MonitoringCatalog>('/api/monitoring/catalog');
}

export function getMonitoringOverview(window: MonitoringWindow): Promise<MonitoringOverview> {
  const query = new URLSearchParams({ window });
  return apiFetch<MonitoringOverview>(`/api/monitoring/overview?${query.toString()}`);
}

export function getMonitoringSeries(
  metric: string,
  window: MonitoringWindow,
  step: MonitoringStep
): Promise<MonitoringSeries> {
  const query = new URLSearchParams({ metric, window, step });
  return apiFetch<MonitoringSeries>(`/api/monitoring/series?${query.toString()}`);
}

export function purgeHistory(retentionDays: 30 | 90 | 180 | 365, confirmation: 'PURGE'): Promise<void> {
  return apiFetch<void>('/api/history/purge', {
    method: 'POST',
    body: { retentionDays, confirmation }
  });
}

export function principalFromAuth(auth: AuthSummary | null): Principal | null {
  if (!auth) return null;
  const principal = auth.principal ?? auth.user;
  if (!principal) return null;
  return {
    ...principal,
    roles: rolesFromPrincipal(principal, auth.roles)
  };
}

export function rolesFromPrincipal(principal: Principal | null | undefined, fallback: string[] = []): string[] {
  const roles = principal?.roles ?? fallback;
  if (roles.length > 0) return roles;
  return principal?.role ? [principal.role] : [];
}

export function hasOwnerRole(principal: Principal | null | undefined): boolean {
  return rolesFromPrincipal(principal).includes('owner');
}
