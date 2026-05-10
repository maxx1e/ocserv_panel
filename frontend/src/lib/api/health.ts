import { apiFetch } from './_client';

export type VPNServerHealth = {
  systemdStatus: string;
  processRunning: boolean;
  socketPresent: boolean;
  configPresent: boolean;
  configValid: boolean;
  configErrors?: string;
  passwordFilePresent: boolean;
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

export function getSystemHealth(): Promise<SystemHealth> {
  return apiFetch<SystemHealth>('/api/health/system');
}

export function getLogTail(source: 'ocserv' | 'panel', lines = 25): Promise<LogTailResult> {
  return apiFetch<LogTailResult>(`/api/logs/tail?source=${source}&lines=${lines}`);
}
