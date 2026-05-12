import type { OcservSession, OcservSessionDetail, Principal, AuthSummary } from '$lib/api';

export function makeSession(overrides: Partial<OcservSession> = {}): OcservSession {
  return {
    id: 'aaaabbbbccccdddd',
    username: 'testuser',
    remoteAddress: '10.0.0.1',
    vpnAddress: '172.16.0.2',
    connectedAt: new Date(Date.now() - 5 * 60 * 1000).toISOString(),
    rxBytes: 1024,
    txBytes: 2048,
    userAgent: 'AnyConnect',
    device: 'Linux',
    reconnectCount: 0,
    state: 'connected',
    groupname: '(none)',
    vhost: 'default',
    enriched: false,
    ...overrides
  };
}

export function makeSessionDetail(overrides: Partial<OcservSessionDetail> = {}): OcservSessionDetail {
  return {
    id: 'aaaabbbbccccdddd',
    username: 'testuser',
    remoteAddress: '10.0.0.1',
    vpnAddress: '172.16.0.2',
    connectedAt: new Date(Date.now() - 5 * 60 * 1000).toISOString(),
    durationSeconds: 300,
    rxBytes: 1024,
    txBytes: 2048,
    userAgent: 'AnyConnect/4.10',
    device: 'Linux',
    ipv4: '172.16.0.2',
    ipv6: '',
    routes: ['0.0.0.0/0'],
    dns: ['8.8.8.8'],
    reconnectCount: 0,
    state: 'connected',
    groupname: '(none)',
    vhost: 'default',
    ...overrides
  };
}

export function makeOwnerPrincipal(): Principal {
  return { username: 'admin', roles: ['owner'] };
}

export function makeViewerPrincipal(): Principal {
  return { username: 'viewer', roles: [] };
}

export function makeAuthSummary(principal: Principal): AuthSummary {
  return { principal };
}
