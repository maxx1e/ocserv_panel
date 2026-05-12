import { apiFetch } from './_client';

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
  state: string;
  groupname: string;
  vhost: string;
  enriched: boolean;
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
  state: string;
  groupname: string;
  vhost: string;
};

export type OcservStatus = {
  status: string;
  configured: boolean;
  message: string;
};

export type ServerStatus = {
  status: string;
  serverPid: number;
  secModPid: number;
  secModInstances: number;
  upSince: string;
  activeSessions: number;
  totalSessions: number;
  totalAuthFailures: number;
  bannedIps: number;
  lastStatsReset: string;
  sessionsHandled: number;
  timedOutSessions: number;
  idleTimedOutSessions: number;
  errorClosedSessions: number;
  authFailures: number;
  avgAuthTimeSeconds: number;
  maxAuthTimeSeconds: number;
  avgSessionTimeSeconds: number;
  maxSessionTimeSeconds: number;
  minMtu: number;
  maxMtu: number;
  rxBytes: number;
  txBytes: number;
};

export type ConnectedUser = {
  id: string;
  username: string;
  vhost: string;
  remoteIp: string;
  vpnIp: string;
  device: string;
  since: string;
  durationSeconds: number;
  dtlsCipher: string;
  status: string;
};

export type UserConnection = {
  id: string;
  username: string;
  groupname: string;
  state: string;
  vhost: string;
  device: string;
  mtu: number;
  remoteIp: string;
  location: string;
  localDeviceIp: string;
  ipv4: string;
  ptpIpv4: string;
  userAgent: string;
  rxBytes: number;
  txBytes: number;
  avgRxBandwidth: string;
  avgTxBandwidth: string;
  dpd: number;
  keepAlive: number;
  hostname: string;
  connectedAt: string;
  sessionId: string;
  tlsCipher: string;
  dtlsCipher: string;
  dns: string[];
  routes: string[];
  restrictedToRoutes: boolean;
};

export type ConnectionEvent = {
  timestamp: string;
  username: string;
  remoteIp: string;
  eventType: string;
  message: string;
};

export type UserRoute = {
  id: string;
  username: string;
  vhost: string;
  device: string;
  vpnIp: string;
  route: string;
};

export type BannedIP = {
  ip: string;
  score: number;
  reason: string;
};

export type BanPointEntry = {
  ip: string;
  score: number;
};

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

export function getOcservReadiness(): Promise<OcservReadiness> {
  return apiFetch<OcservReadiness>('/api/ocserv/readiness');
}

export function getServerStatus(): Promise<ServerStatus> {
  return apiFetch<ServerStatus>('/api/ocserv/server/status');
}

export function getConnectedUsers(): Promise<ConnectedUser[]> {
  return apiFetch<ConnectedUser[]>('/api/ocserv/users/connected');
}

export function getUserConnections(username: string): Promise<UserConnection[]> {
  return apiFetch<UserConnection[]>(`/api/ocserv/users/${encodeURIComponent(username)}/connections`);
}

export function getConnectionById(id: number): Promise<UserConnection> {
  return apiFetch<UserConnection>(`/api/ocserv/connections/${id}`);
}

export function getConnectionEvents(): Promise<ConnectionEvent[]> {
  return apiFetch<ConnectionEvent[]>('/api/ocserv/events');
}

export function getUserRoutes(): Promise<UserRoute[]> {
  return apiFetch<UserRoute[]>('/api/ocserv/routes');
}

export function getBannedIPs(): Promise<BannedIP[]> {
  return apiFetch<BannedIP[]>('/api/ocserv/bans');
}

export function getBanPoints(): Promise<BanPointEntry[]> {
  return apiFetch<BanPointEntry[]>('/api/ocserv/bans/points');
}

export function disconnectUser(username: string): Promise<void> {
  return apiFetch<void>(`/api/ocserv/users/${encodeURIComponent(username)}/disconnect`, {
    method: 'POST'
  });
}

export function unbanIP(ip: string): Promise<void> {
  return apiFetch<void>(`/api/ocserv/bans/${encodeURIComponent(ip)}/unban`, {
    method: 'POST'
  });
}
