<script lang="ts">
  import { X, Copy, Shield, Network } from 'lucide-svelte';
  import { formatAbsoluteTime } from '$lib/utils/format';
  import { formatBytes } from '$lib/monitoring';
  import type { OcservSession, OcservSessionDetail, UserConnection } from '$lib/api';

  type Props = {
    session: OcservSession | undefined;
    detail: OcservSessionDetail | null;
    userConnection: UserConnection | null;
    loading: boolean;
    error: string;
    sessionGone: boolean;
    isOwner: boolean;
    onClose: () => void;
    onDisconnectSession: (session: OcservSession) => void;
    onTerminateSession: (session: OcservSession) => void;
    onDisconnectUser: (username: string) => void;
  };

  let {
    session,
    detail,
    userConnection,
    loading,
    error,
    sessionGone,
    isOwner,
    onClose,
    onDisconnectSession,
    onTerminateSession,
    onDisconnectUser
  }: Props = $props();

  function formatDurationSecs(secs: number | undefined): string {
    if (!secs || secs < 0) return '—';
    const h = Math.floor(secs / 3600);
    const m = Math.floor((secs % 3600) / 60);
    if (h > 0) return `${h}h ${m}m`;
    return `${m}m`;
  }

  function computeDurationSecs(connectedAt: string | undefined): number {
    if (!connectedAt) return 0;
    const ms = Date.now() - new Date(connectedAt).getTime();
    return isNaN(ms) || ms < 0 ? 0 : Math.floor(ms / 1000);
  }

  function copyToClipboard(text: string) {
    navigator.clipboard.writeText(text);
  }

  function displayGroupname(g: string | undefined): string {
    if (!g || g === '(none)') return '';
    return g;
  }

  const username = $derived(session?.username ?? 'Session');
  const sessionIdShort = $derived(session?.id?.slice(0, 8) ?? '');
</script>

<div class="w-96 shrink-0 bg-white rounded-lg shadow-sm border border-slate-100 overflow-hidden">
  <!-- Panel header -->
  <div class="flex items-center justify-between px-4 py-3 border-b border-slate-100 bg-slate-50">
    <div class="min-w-0">
      <h2 class="text-sm font-semibold text-slate-800 truncate">
        {username} — <span class="font-mono font-normal text-slate-500">{sessionIdShort}</span>
      </h2>
    </div>
    <button
      type="button"
      onclick={onClose}
      class="ml-2 text-slate-400 hover:text-slate-600 transition-colors flex-shrink-0"
      aria-label="Close panel"
    >
      <X class="w-4 h-4" />
    </button>
  </div>

  <!-- Panel body -->
  <div class="px-4 py-3 space-y-1 overflow-y-auto max-h-[calc(100vh-16rem)]">
    {#if loading}
      <div class="py-8 flex items-center justify-center">
        <div class="w-6 h-6 border-2 border-slate-200 border-t-slate-500 rounded-full animate-spin"></div>
      </div>

    {:else if sessionGone}
      <p class="text-sm text-slate-500 py-6 text-center">Session no longer active.</p>

    {:else if error}
      <p class="text-sm text-red-600 py-4">{error}</p>

    {:else if detail || userConnection}
      <!-- Connection Info -->
      <p class="text-xs font-semibold text-slate-500 uppercase tracking-wider mt-4 mb-2">Connection</p>
      <div class="flex justify-between text-sm py-1 border-b border-slate-50">
        <span class="text-slate-500">VPN IPv4</span>
        <span class="text-slate-700 font-medium font-mono text-xs">{detail?.ipv4 || userConnection?.ipv4 || session?.vpnAddress || '—'}</span>
      </div>
      {#if detail?.ipv6}
        <div class="flex justify-between text-sm py-1 border-b border-slate-50">
          <span class="text-slate-500">VPN IPv6</span>
          <span class="text-slate-700 font-medium font-mono text-xs">{detail.ipv6}</span>
        </div>
      {/if}
      <div class="flex justify-between text-sm py-1 border-b border-slate-50">
        <span class="text-slate-500">Client IP</span>
        <span class="text-slate-700 font-medium font-mono text-xs">{detail?.remoteAddress || userConnection?.remoteIp || session?.remoteAddress || '—'}</span>
      </div>
      <div class="flex justify-between text-sm py-1 border-b border-slate-50">
        <span class="text-slate-500">Connected</span>
        <span class="text-slate-700 font-medium text-right">{formatAbsoluteTime(detail?.connectedAt || userConnection?.connectedAt)}</span>
      </div>
      <div class="flex justify-between text-sm py-1 border-b border-slate-50">
        <span class="text-slate-500">Duration</span>
        <span class="text-slate-700 font-medium">{formatDurationSecs(detail?.durationSeconds ?? computeDurationSecs(userConnection?.connectedAt))}</span>
      </div>
      <div class="flex justify-between text-sm py-1 border-b border-slate-50">
        <span class="text-slate-500">Device</span>
        <span class="text-slate-700 font-medium">{detail?.device || userConnection?.device || session?.device || '—'}</span>
      </div>
      <div class="flex justify-between text-sm py-1 border-b border-slate-50">
        <span class="text-slate-500">State</span>
        <span class="text-slate-700 font-medium">{detail?.state || userConnection?.state || session?.state || '—'}</span>
      </div>
      {#if displayGroupname(detail?.groupname || userConnection?.groupname || session?.groupname)}
        <div class="flex justify-between text-sm py-1 border-b border-slate-50">
          <span class="text-slate-500">Group</span>
          <span class="text-slate-700 font-medium">{displayGroupname(detail?.groupname || userConnection?.groupname || session?.groupname)}</span>
        </div>
      {/if}
      {#if detail?.vhost || userConnection?.vhost || session?.vhost}
        <div class="flex justify-between text-sm py-1 border-b border-slate-50">
          <span class="text-slate-500">Virtual Host</span>
          <span class="text-slate-700 font-medium">{detail?.vhost || userConnection?.vhost || session?.vhost || '—'}</span>
        </div>
      {/if}
      {#if userConnection?.hostname}
        <div class="flex justify-between text-sm py-1 border-b border-slate-50">
          <span class="text-slate-500">Hostname</span>
          <span class="text-slate-700 font-medium">{userConnection.hostname}</span>
        </div>
      {/if}

      <!-- Traffic -->
      <p class="text-xs font-semibold text-slate-500 uppercase tracking-wider mt-4 mb-2">Traffic</p>
      <div class="flex justify-between text-sm py-1 border-b border-slate-50">
        <span class="text-slate-500">Received</span>
        <span class="text-slate-700 font-medium">{formatBytes(detail?.rxBytes ?? userConnection?.rxBytes ?? 0)}</span>
      </div>
      <div class="flex justify-between text-sm py-1 border-b border-slate-50">
        <span class="text-slate-500">Transmitted</span>
        <span class="text-slate-700 font-medium">{formatBytes(detail?.txBytes ?? userConnection?.txBytes ?? 0)}</span>
      </div>
      {#if userConnection}
        <div class="flex justify-between text-sm py-1 border-b border-slate-50">
          <span class="text-slate-500">Avg Download</span>
          <span class="text-slate-700 font-medium">{userConnection.avgRxBandwidth || '—'}</span>
        </div>
        <div class="flex justify-between text-sm py-1 border-b border-slate-50">
          <span class="text-slate-500">Avg Upload</span>
          <span class="text-slate-700 font-medium">{userConnection.avgTxBandwidth || '—'}</span>
        </div>
      {/if}

      <!-- Client -->
      <p class="text-xs font-semibold text-slate-500 uppercase tracking-wider mt-4 mb-2">Client</p>
      <div class="flex justify-between text-sm py-1 border-b border-slate-50">
        <span class="text-slate-500">User Agent</span>
        <span class="text-slate-700 font-medium text-right max-w-[60%] break-words">{detail?.userAgent || userConnection?.userAgent || '—'}</span>
      </div>
      {#if detail}
        <div class="flex justify-between text-sm py-1 border-b border-slate-50">
          <span class="text-slate-500">Stability</span>
          {#if detail.reconnectCount > 2}
            <span class="px-2 py-0.5 rounded text-xs font-medium bg-yellow-100 text-yellow-700">
              {detail.reconnectCount} reconnects
            </span>
          {:else}
            <span class="text-slate-700 font-medium">Stable</span>
          {/if}
        </div>
      {/if}

      <!-- Security Section (new) -->
      {#if userConnection}
        <p class="text-xs font-semibold text-slate-500 uppercase tracking-wider mt-4 mb-2 flex items-center gap-1.5">
          <Shield class="w-3 h-3" /> Security
        </p>
        <div class="flex justify-between text-sm py-1 border-b border-slate-50">
          <span class="text-slate-500">TLS Cipher</span>
          <button
            type="button"
            title="Copy TLS cipher"
            onclick={() => copyToClipboard(userConnection.tlsCipher)}
            class="text-slate-700 font-mono text-xs text-right max-w-[60%] truncate hover:text-blue-600 cursor-pointer"
          >
            {userConnection.tlsCipher || '—'}
          </button>
        </div>
        <div class="flex justify-between text-sm py-1 border-b border-slate-50">
          <span class="text-slate-500">DTLS Cipher</span>
          <button
            type="button"
            title="Copy DTLS cipher"
            onclick={() => copyToClipboard(userConnection.dtlsCipher)}
            class="text-slate-700 font-mono text-xs text-right max-w-[60%] truncate hover:text-blue-600 cursor-pointer"
          >
            {userConnection.dtlsCipher || '—'}
          </button>
        </div>
        <div class="flex justify-between text-sm py-1 border-b border-slate-50">
          <span class="text-slate-500">Cert Auth</span>
          <span class="text-slate-700 font-medium">No</span>
        </div>

        <!-- Network Policy Section (new) -->
        <p class="text-xs font-semibold text-slate-500 uppercase tracking-wider mt-4 mb-2 flex items-center gap-1.5">
          <Network class="w-3 h-3" /> Network Policy
        </p>
        <div class="flex justify-between text-sm py-1 border-b border-slate-50">
          <span class="text-slate-500">MTU</span>
          <span class="text-slate-700 font-medium">{userConnection.mtu || '—'}</span>
        </div>
        <div class="flex justify-between text-sm py-1 border-b border-slate-50">
          <span class="text-slate-500">DPD Timeout</span>
          <span class="text-slate-700 font-medium">{userConnection.dpd ? `${userConnection.dpd}s` : '—'}</span>
        </div>
        <div class="flex justify-between text-sm py-1 border-b border-slate-50">
          <span class="text-slate-500">KeepAlive</span>
          <span class="text-slate-700 font-medium">{userConnection.keepAlive ? `${userConnection.keepAlive}s` : '—'}</span>
        </div>
        <div class="flex justify-between text-sm py-1 border-b border-slate-50">
          <span class="text-slate-500">Restricted Routes</span>
          <span class="text-slate-700 font-medium">{userConnection.restrictedToRoutes ? 'Yes' : 'No'}</span>
        </div>
      {/if}

      <!-- Routes -->
      <p class="text-xs font-semibold text-slate-500 uppercase tracking-wider mt-4 mb-2">Assigned Routes</p>
      {#if (detail?.routes && detail.routes.length > 0) || (userConnection?.routes && userConnection.routes.length > 0)}
        <ul class="space-y-0.5">
          {#each (detail?.routes ?? userConnection?.routes ?? []) as route}
            <li class="text-xs font-mono text-slate-700 py-0.5">{route}</li>
          {/each}
        </ul>
      {:else}
        <p class="text-xs text-slate-500">None assigned</p>
      {/if}

      <!-- DNS -->
      <p class="text-xs font-semibold text-slate-500 uppercase tracking-wider mt-4 mb-2">Assigned DNS</p>
      {#if (detail?.dns && detail.dns.length > 0) || (userConnection?.dns && userConnection.dns.length > 0)}
        <ul class="space-y-0.5">
          {#each (detail?.dns ?? userConnection?.dns ?? []) as entry}
            <li class="text-xs font-mono text-slate-700 py-0.5">{entry}</li>
          {/each}
        </ul>
      {:else}
        <p class="text-xs text-slate-500">None assigned</p>
      {/if}

      <!-- Panel actions -->
      {#if isOwner && session}
        <div class="flex flex-col gap-2 pt-4 pb-2 border-t border-slate-100 mt-4">
          <div class="flex items-center gap-2">
            <button
              type="button"
              onclick={() => onDisconnectSession(session)}
              class="flex-1 px-3 py-2 text-sm font-medium rounded-md bg-orange-100 text-orange-700 hover:bg-orange-200 transition-colors"
            >
              Disconnect
            </button>
            <button
              type="button"
              onclick={() => onTerminateSession(session)}
              class="flex-1 px-3 py-2 text-sm font-medium rounded-md bg-red-100 text-red-700 hover:bg-red-200 transition-colors"
            >
              Terminate
            </button>
          </div>
          <button
            type="button"
            onclick={() => onDisconnectUser(session.username)}
            class="w-full px-3 py-2 text-sm font-medium rounded-md bg-orange-50 text-orange-600 border border-orange-200 hover:bg-orange-100 transition-colors"
          >
            Disconnect All Sessions for {session.username}
          </button>
        </div>
      {/if}
    {/if}
  </div>
</div>
