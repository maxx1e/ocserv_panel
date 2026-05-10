import { apiFetch } from './_client';

export type Setting = {
  key: string;
  value: string;
};

export type SettingsResponse = {
  settings: Setting[];
};

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

export function getSettings(): Promise<SettingsResponse> {
  return apiFetch<SettingsResponse>('/api/settings');
}

export function updateSetting(key: string, value: string): Promise<Setting> {
  return apiFetch<Setting>(`/api/settings/${encodeURIComponent(key)}`, {
    method: 'PUT',
    body: { value }
  });
}

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
