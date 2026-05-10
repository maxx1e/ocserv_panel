import { apiFetch } from './_client';

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
