import { apiFetch } from './_client';

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

export type BulkResult = {
  total: number;
  succeeded: number;
  failed: number;
  errors: { id: string; error: string }[];
};

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

export function createVpnUser(username: string, password: string, group?: string): Promise<void> {
  const body: { username: string; password: string; group?: string } = { username, password };
  if (group) body.group = group;
  return apiFetch<void>('/api/ocserv/users/create', { method: 'POST', body });
}

export function updateVpnUserGroup(username: string, group: string): Promise<void> {
  return apiFetch<void>(`/api/ocserv/users/${encodeURIComponent(username)}/group`, {
    method: 'PUT',
    body: { group }
  });
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

// ─── Bulk Operations ────────────────────────────────────────────────────────

export function bulkDeleteAdminUsers(ids: number[]): Promise<BulkResult> {
  return apiFetch<BulkResult>('/api/admin/users/bulk-delete', { method: 'POST', body: { ids } });
}

export function bulkDisableAdminUsers(ids: number[]): Promise<BulkResult> {
  return apiFetch<BulkResult>('/api/admin/users/bulk-disable', { method: 'POST', body: { ids } });
}

export function bulkEnableAdminUsers(ids: number[]): Promise<BulkResult> {
  return apiFetch<BulkResult>('/api/admin/users/bulk-enable', { method: 'POST', body: { ids } });
}

export function bulkDeleteVpnUsers(usernames: string[]): Promise<BulkResult> {
  return apiFetch<BulkResult>('/api/ocserv/users/bulk-delete', { method: 'POST', body: { usernames } });
}

export function bulkLockVpnUsers(usernames: string[]): Promise<BulkResult> {
  return apiFetch<BulkResult>('/api/ocserv/users/bulk-lock', { method: 'POST', body: { usernames } });
}

export function bulkUnlockVpnUsers(usernames: string[]): Promise<BulkResult> {
  return apiFetch<BulkResult>('/api/ocserv/users/bulk-unlock', { method: 'POST', body: { usernames } });
}
