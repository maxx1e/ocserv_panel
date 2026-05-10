import { apiFetch } from './_client';

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
