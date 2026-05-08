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

  const isOnline = $derived(status?.status === 'online');
  const uptimeStr = $derived(formatUptime(status?.upSince));
  const trafficStr = $derived(
    status ? `${formatBytes(status.rxBytes)} / ${formatBytes(status.txBytes)}` : '—'
  );
</script>

{#if error}
  <div class="px-4 py-2 rounded-lg text-sm bg-amber-50 text-amber-700 border border-amber-200" role="alert">
    Server status unavailable: {error}
  </div>
{:else}
  <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
    <CardStats
      title="Server"
      value={loading ? '...' : (isOnline ? 'Online' : 'Offline')}
      subtitle={isOnline ? `Uptime: ${uptimeStr}` : undefined}
      icon={Server}
      statusValue={isOnline ? 'online' : 'offline'}
    />
    <CardStats
      title="Active Sessions"
      value={loading ? '...' : String(status?.activeSessions ?? 0)}
      subtitle={status ? `${status.sessionsHandled} handled this period` : undefined}
      icon={Users}
    />
    <CardStats
      title="Auth Failures"
      value={loading ? '...' : String(status?.authFailures ?? 0)}
      subtitle={status?.avgAuthTimeSeconds ? `Avg auth: ${status.avgAuthTimeSeconds}s` : undefined}
      icon={ShieldAlert}
    />
    <CardStats
      title="Traffic (Period)"
      value={loading ? '...' : trafficStr}
      subtitle="RX / TX — updates on disconnect"
      icon={ArrowUpDown}
    />
  </div>
{/if}
