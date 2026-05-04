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

export type Setting = {
  key: string;
  value: string;
};

export type SettingsResponse = {
  settings: Setting[];
};

export function getSettings(): Promise<SettingsResponse> {
  return apiFetch<SettingsResponse>('/api/settings');
}

export function updateSetting(key: string, value: string): Promise<Setting> {
  return apiFetch<Setting>(`/api/settings/${encodeURIComponent(key)}`, {
    method: 'PUT',
    body: { value }
  });
}

// Settings sections types (new unified API)
export type SettingDef = {
  key: string;
  label: string;
  section: string;
  type: 'text' | 'number' | 'toggle' | 'select' | 'password' | 'multi-checkbox' | 'readonly' | 'button';
  source: 'config' | 'database';
  requires_restart: boolean;
  default: string;
  value: string;
  options?: string[];
  validation?: string;
  sort_order: number;
};

export type SettingsSection = {
  id: string;
  label: string;
  title: string;
  description?: string;
  settings: SettingDef[];
};

export type SettingsSectionsResponse = {
  sections: SettingsSection[];
  restart_required: boolean;
  pending_changes: string[];
};

export type RestartStatus = {
  restart_required: boolean;
  pending_changes: string[];
};

export function getSettingsSections(): Promise<SettingsSectionsResponse> {
  return apiFetch<SettingsSectionsResponse>('/api/settings');
}

export function updateSettingByKey(key: string, value: string): Promise<SettingDef> {
  return apiFetch<SettingDef>(`/api/settings/${encodeURIComponent(key)}`, {
    method: 'PUT',
    body: { value }
  });
}

export function getRestartStatus(): Promise<RestartStatus> {
  return apiFetch<RestartStatus>('/api/settings/restart-status');
}

export function triggerRestart(): Promise<{ ok: boolean; message?: string }> {
  return apiFetch<{ ok: boolean; message?: string }>('/api/settings/restart', { method: 'POST' });
}

export type OcservReadiness = {
  binaryPresent: boolean;
  binaryVersion: string;
  configPresent: boolean;
  configValid: boolean;
  serviceRunning: boolean;
  socketPresent: boolean;
  occtlPresent: boolean;
  ocpasswdPresent: boolean;
};

export type OcservServiceStatus = {
  status: 'active' | 'inactive' | 'failed' | 'not_found';
  message: string;
};

export type OcservSession = {
  id: string;
  username: string;
  remoteAddress: string;
  vpnAddress: string;
  connectedAt?: string;
  rxBytes: number;
  txBytes: number;
  userAgent: string;
  device: string;
  reconnectCount: number;
};

export type OcservSessionDetail = {
  id: string;
  username: string;
  remoteAddress: string;
  vpnAddress: string;
  connectedAt?: string;
  durationSeconds: number;
  rxBytes: number;
  txBytes: number;
  userAgent: string;
  device: string;
  ipv4: string;
  ipv6: string;
  routes: string[];
  dns: string[];
  reconnectCount: number;
};

export type OcservUser = {
  username: string;
  group: string;
  active: boolean;
};

export type OcservConfigIssueSeverity = 'error' | 'warning' | 'info';

export type OcservConfigIssue = {
  severity: OcservConfigIssueSeverity;
  key: string;
  message: string;
  suggestion: string;
};

export type OcservStatus = {
  status: string;
  configured: boolean;
  message: string;
};

export function getOcservReadiness(): Promise<OcservReadiness> {
  return apiFetch<OcservReadiness>('/api/ocserv/readiness');
}

export function getOcservServiceStatus(): Promise<OcservServiceStatus> {
  return apiFetch<OcservServiceStatus>('/api/ocserv/service/status');
}

export function startOcservService(): Promise<void> {
  return apiFetch<void>('/api/ocserv/service/start', { method: 'POST' });
}

export function stopOcservService(): Promise<void> {
  return apiFetch<void>('/api/ocserv/service/stop', { method: 'POST' });
}

export function restartOcservService(): Promise<void> {
  return apiFetch<void>('/api/ocserv/service/restart', { method: 'POST' });
}

export function getOcservSessions(): Promise<OcservSession[]> {
  return apiFetch<OcservSession[]>('/api/ocserv/sessions');
}

export function disconnectOcservSession(sessionId: string): Promise<void> {
  return apiFetch<void>('/api/ocserv/sessions/disconnect', {
    method: 'POST',
    body: { sessionId }
  });
}

export function getOcservSessionDetail(id: string): Promise<OcservSessionDetail> {
  return apiFetch<OcservSessionDetail>(`/api/ocserv/sessions/${encodeURIComponent(id)}`);
}

export function terminateOcservSession(sessionId: string): Promise<void> {
  return apiFetch<void>('/api/ocserv/sessions/terminate', {
    method: 'POST',
    body: { sessionId }
  });
}

export function getOcservUsers(): Promise<OcservUser[]> {
  return apiFetch<OcservUser[]>('/api/ocserv/users');
}

export function createOcservUser(username: string, password: string): Promise<void> {
  return apiFetch<void>('/api/ocserv/users/create', {
    method: 'POST',
    body: { username, password }
  });
}

export function deleteOcservUser(username: string): Promise<void> {
  return apiFetch<void>(`/api/ocserv/users/${encodeURIComponent(username)}`, {
    method: 'DELETE'
  });
}

export function updateOcservUserPassword(username: string, password: string): Promise<void> {
  return apiFetch<void>(`/api/ocserv/users/${encodeURIComponent(username)}/password`, {
    method: 'PUT',
    body: { password }
  });
}

export function getOcservConfigIssues(): Promise<OcservConfigIssue[]> {
  return apiFetch<OcservConfigIssue[]>('/api/ocserv/config/issues');
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
