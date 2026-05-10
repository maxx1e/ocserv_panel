<script lang="ts">
  import CardStats from '$lib/components/ui/CardStats.svelte';
  import { Server, Users, ShieldAlert, ArrowUpDown } from 'lucide-svelte';
  import { formatBytes } from '$lib/monitoring';
  import type { ServerStatus } from '$lib/api';

  type Props = {
    status: ServerStatus | null;
    loading: boolean;
    error: string;
  };

  let { status, loading, error }: Props = $props();

  function formatUptime(upSince: string | undefined): string {
    if (!upSince) return '—';
    const date = new Date(upSince);
    if (isNaN(date.getTime())) return '—';
    const ms = Date.now() - date.getTime();
    if (ms < 0) return '—';
    const totalMin = Math.floor(ms / 60000);
    const totalHours = Math.floor(totalMin / 60);
    const totalDays = Math.floor(totalHours / 24);
    if (totalDays > 0) return `${totalDays}d ${totalHours % 24}h`;
    if (totalHours > 0) return `${totalHours}h ${totalMin % 60}m`;
    return `${totalMin}m`;
  }

  function formatDuration(seconds: number | undefined): string {
    if (!seconds) return '—';
    if (seconds >= 3600) {
      const h = Math.floor(seconds / 3600);
      const m = Math.floor((seconds % 3600) / 60);
      return `${h}h ${m}m`;
    }
    if (seconds >= 60) return `${Math.floor(seconds / 60)}m`;
    return `${seconds}s`;
  }

  function relativeDate(iso: string | undefined): string {
    if (!iso) return '—';
    const date = new Date(iso);
    if (isNaN(date.getTime())) return '—';
    const diffMs = Date.now() - date.getTime();
    const diffMin = Math.floor(diffMs / 60000);
    const diffHours = Math.floor(diffMin / 60);
    const diffDays = Math.floor(diffHours / 24);
    if (diffDays === 0) return 'today';
    return `${diffDays}d ago`;
  }

  const isOnline = $derived(status?.status === 'online');

  const serviceValue = $derived(
    loading ? '...' : (isOnline ? formatUptime(status?.upSince) : 'Offline')
  );
  const serviceSubtitle = $derived(
    isOnline && status?.serverPid ? `PID: ${status.serverPid}` : undefined
  );

  const sessionsValue = $derived(loading ? '...' : String(status?.activeSessions ?? 0));
  const sessionsSubtitle = $derived(
    status
      ? `${status.sessionsHandled} handled · avg ${formatDuration(status.avgSessionTimeSeconds)}`
      : undefined
  );

  const authValue = $derived(loading ? '...' : String(status?.authFailures ?? 0));
  const authSubtitle = $derived(
    status
      ? `Avg: ${status.avgAuthTimeSeconds}s · Max: ${status.maxAuthTimeSeconds}s`
      : undefined
  );

  const networkValue = $derived(
    loading
      ? '...'
      : status
        ? `${formatBytes(status.rxBytes)} / ${formatBytes(status.txBytes)}`
        : '—'
  );
  const networkSubtitle = $derived(
    status
      ? `MTU ${status.minMtu}–${status.maxMtu} · reset ${relativeDate(status.lastStatsReset)}`
      : undefined
  );
</script>

{#if error}
  <div class="px-4 py-2 rounded-lg text-sm bg-amber-50 text-amber-700 border border-amber-200" role="alert">
    Server status unavailable: {error}
  </div>
{:else}
  <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
    <CardStats
      title="Service"
      value={serviceValue}
      subtitle={serviceSubtitle}
      icon={Server}
      statusValue={isOnline ? 'online' : 'offline'}
    />
    <CardStats
      title="Sessions"
      value={sessionsValue}
      subtitle={sessionsSubtitle}
      icon={Users}
    />
    <CardStats
      title="Auth"
      value={authValue}
      subtitle={authSubtitle}
      icon={ShieldAlert}
    />
    <CardStats
      title="Network"
      value={networkValue}
      subtitle={networkSubtitle}
      icon={ArrowUpDown}
    />
  </div>
{/if}
