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

export type AdminUser = {
  id: number;
  username: string;
  displayName: string;
  email: string;
  roles: string[];
  disabled: boolean;
  createdAt: string;
  updatedAt: string;
};

export type VpnUser = {
  username: string;
  group: string;
  active: boolean;
};

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

export function getMonitoringOverview(window: MonitoringWindow): Promise<MonitoringOverview> {
  const query = new URLSearchParams({ window });
  return apiFetch<MonitoringOverview>(`/api/monitoring/overview?${query.toString()}`);
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

export type ConfigStatus = 'ok' | 'parse_error' | 'permission_error' | 'file_missing' | 'read_error';

export type SettingsSectionsResponse = {
  sections: SettingsSection[];
  restart_required: boolean;
  pending_changes: string[];
  config_status: ConfigStatus;
  config_error?: string;
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

export type OcservStatus = {
  status: string;
  configured: boolean;
  message: string;
};

export type VPNServerHealth = {
  systemdStatus: string;
  processRunning: boolean;
  socketPresent: boolean;
  configPresent: boolean;
  configValid: boolean;
  configErrors?: string;
  binaryPresent: boolean;
  binaryVersion?: string;
};

export type PanelHealth = {
  backendStatus: string;
  dbReachable: boolean;
  dbPath: string;
  configPresent: boolean;
  configPath: string;
  webuiUserCount: number;
};

export type LogsHealth = {
  ocservLogEnabled: boolean;
  ocservLogPresent: boolean;
  ocservLogPath: string;
  panelLogEnabled: boolean;
  panelLogPresent: boolean;
  panelLogPath: string;
};

export type StorageItemHealth = {
  engine: string;
  sizeBytes: number;
  status: 'healthy' | 'warning' | 'critical';
  path: string;
};

export type StorageHealth = {
  sqlite: StorageItemHealth;
  tsdb: StorageItemHealth;
};

export type SystemHealth = {
  vpnServer: VPNServerHealth;
  panel: PanelHealth;
  logs: LogsHealth;
  storage?: StorageHealth;
};

export type LogTailResult = {
  source: string;
  enabled: boolean;
  present: boolean;
  path: string;
  lines: string[];
  error?: string;
};

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

export function getSystemHealth(): Promise<SystemHealth> {
  return apiFetch<SystemHealth>('/api/health/system');
}

export function getLogTail(source: 'ocserv' | 'panel', lines = 25): Promise<LogTailResult> {
  return apiFetch<LogTailResult>(`/api/logs/tail?source=${source}&lines=${lines}`);
}

// ─── Config Management Types ────────────────────────────────────────────────

export type ConfigIssue = {
  severity: 'error' | 'warning' | 'info';
  key: string;
  message: string;
  suggestion?: string;
  line?: number;
  fixedValue?: string;
};

export type ConfigMetadata = {
  path: string;
  lastModified: string;
  size: number;
};

export type ConfigReadResult = {
  content: string;
  metadata: ConfigMetadata;
  issues: ConfigIssue[];
};

export type ConfigValidateResult = {
  valid: boolean;
  issues: ConfigIssue[];
  requiresRestart?: boolean;
  restartReasons?: string[];
};

export type ConfigDiffResult = {
  diff: string;
};

export type ConfigApplyResult = {
  backupNumber: number;
  reloadSuccess: boolean;
  reloadError?: string;
  issues: ConfigIssue[];
  requiresRestart?: boolean;
  restartReasons?: string[];
};

export type ConfigBackupInfo = {
  number: number;
  timestamp: string;
  size: number;
};

export type ConfigBackupContent = {
  number: number;
  content: string;
};

export type ConfigRollbackResult = {
  reloadSuccess: boolean;
  reloadError?: string;
};

export type NetworkPrerequisiteCheck = {
  category: 'kernel' | 'firewall' | 'routing';
  name: string;
  status: 'configured' | 'missing' | 'partial' | 'unknown';
  expected: string;
  actual: string;
  suggestion: string;
};

export type NetworkPrerequisites = {
  items: NetworkPrerequisiteCheck[];
};

// ─── Config Management API ──────────────────────────────────────────────────

export function getConfig(): Promise<ConfigReadResult> {
  return apiFetch<ConfigReadResult>('/api/config');
}

export function validateConfig(content: string): Promise<ConfigValidateResult> {
  return apiFetch<ConfigValidateResult>('/api/config/validate', {
    method: 'POST',
    body: { content }
  });
}

export function diffConfig(content: string): Promise<ConfigDiffResult> {
  return apiFetch<ConfigDiffResult>('/api/config/diff', {
    method: 'POST',
    body: { content }
  });
}

export function applyConfig(content: string): Promise<ConfigApplyResult> {
  return apiFetch<ConfigApplyResult>('/api/config/apply', {
    method: 'POST',
    body: { content }
  });
}

export function getConfigBackups(): Promise<ConfigBackupInfo[]> {
  return apiFetch<ConfigBackupInfo[]>('/api/config/backups');
}

export function getConfigBackup(n: number): Promise<ConfigBackupContent> {
  return apiFetch<ConfigBackupContent>(`/api/config/backups/${n}`);
}

export function diffConfigBackup(n: number): Promise<ConfigDiffResult> {
  return apiFetch<ConfigDiffResult>(`/api/config/backups/${n}/diff`);
}

export function rollbackConfig(n: number): Promise<ConfigRollbackResult> {
  return apiFetch<ConfigRollbackResult>(`/api/config/rollback/${n}`, {
    method: 'POST'
  });
}

export function getNetworkPrerequisites(): Promise<NetworkPrerequisites> {
  return apiFetch<NetworkPrerequisites>('/api/config/network');
}

// ─── Admin User Management ─────────────────────────────────────────────────

export function getAdminUsers(): Promise<{ users: AdminUser[] }> {
  return apiFetch<{ users: AdminUser[] }>('/api/admin/users');
}

export function createAdminUser(body: { username: string; password: string; displayName?: string; email?: string; role?: string }): Promise<AdminUser> {
  return apiFetch<AdminUser>('/api/admin/users', { method: 'POST', body });
}

export function updateAdminUser(id: number, body: { displayName?: string; email?: string; role?: string }): Promise<void> {
  return apiFetch<void>(`/api/admin/users/${id}`, { method: 'PUT', body });
}

export function deleteAdminUser(id: number): Promise<void> {
  return apiFetch<void>(`/api/admin/users/${id}`, { method: 'DELETE' });
}

export function disableAdminUser(id: number): Promise<void> {
  return apiFetch<void>(`/api/admin/users/${id}/disable`, { method: 'POST' });
}

export function enableAdminUser(id: number): Promise<void> {
  return apiFetch<void>(`/api/admin/users/${id}/enable`, { method: 'POST' });
}

export function resetAdminPassword(id: number, password: string): Promise<void> {
  return apiFetch<void>(`/api/admin/users/${id}/reset-password`, { method: 'POST', body: { password } });
}

// ─── VPN User Management ───────────────────────────────────────────────────

export function getVpnUsers(): Promise<VpnUser[]> {
  return apiFetch<VpnUser[]>('/api/ocserv/users');
}

export function createVpnUser(username: string, password: string): Promise<void> {
  return apiFetch<void>('/api/ocserv/users/create', { method: 'POST', body: { username, password } });
}

export function deleteVpnUser(username: string): Promise<void> {
  return apiFetch<void>(`/api/ocserv/users/${encodeURIComponent(username)}`, { method: 'DELETE' });
}

export function resetVpnPassword(username: string, password: string): Promise<void> {
  return apiFetch<void>(`/api/ocserv/users/${encodeURIComponent(username)}/password`, { method: 'PUT', body: { password } });
}

export function lockVpnUser(username: string): Promise<void> {
  return apiFetch<void>(`/api/ocserv/users/${encodeURIComponent(username)}/lock`, { method: 'POST' });
}

export function unlockVpnUser(username: string): Promise<void> {
  return apiFetch<void>(`/api/ocserv/users/${encodeURIComponent(username)}/unlock`, { method: 'POST' });
}

// ─── Utility Functions ─────────────────────────────────────────────────────

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
