<script lang="ts">
  import { ArrowDown, ArrowUp, Database, Server, Users } from 'lucide-svelte';
  import CardStats from '$lib/components/ui/CardStats.svelte';
  import * as Card from '$lib/components/ui/card/index';
  import HealthCard from '$lib/components/ui/HealthCard.svelte';
  import type { HealthItem } from '$lib/components/ui/HealthCard.svelte';
  import DonutChart from '$lib/components/ui/DonutChart.svelte';
  import { formatBytes } from '$lib/monitoring';
  import type {
    ActiveUserSessionsResult,
    DashboardSummary,
    SystemHealth,
    SystemInfo
  } from '$lib/api';

  type Props = {
    summary: DashboardSummary;
    activeUserSessions: ActiveUserSessionsResult | null;
    storageItems: HealthItem[];
    healthCheckedAt: Date | null;
    healthLoading: boolean;
    health: SystemHealth | null;
    systemInfo: SystemInfo | null;
  };

  let { summary, activeUserSessions, storageItems, healthCheckedAt, healthLoading, health, systemInfo }: Props = $props();

  const DONUT_COLORS = [
    '#6366f1', '#10b981', '#f59e0b', '#ec4899',
    '#06b6d4', '#8b5cf6', '#ef4444', '#14b8a6',
  ];

  const donutSegments = $derived(
    (activeUserSessions?.users ?? []).map((u, i) => ({
      label: u.username,
      value: u.sessionCount,
      color: DONUT_COLORS[i % DONUT_COLORS.length],
    }))
  );

  const trafficSubtitle = $derived((): string | undefined => {
    if (!systemInfo?.netInterface) return undefined;
    const parts = [systemInfo.netInterface];
    if (systemInfo.vpnInterfaces?.length) {
      parts.push(...systemInfo.vpnInterfaces);
    }
    return parts.join(' + ');
  });
</script>

<!-- Donut + Storage -->
<div class="grid grid-cols-1 md:grid-cols-2 gap-6 mb-8">
  <div>
    <Card.Root>
      <Card.Header>
        <Card.Title>Sessions by User</Card.Title>
      </Card.Header>
      <Card.Content>
        <DonutChart segments={donutSegments} height={180} />
      </Card.Content>
    </Card.Root>
  </div>
  <HealthCard
    title="Storage Metrics"
    items={storageItems}
    lastChecked={healthCheckedAt}
    loading={healthLoading && !health}
  />
</div>

<!-- Analytics section -->
<section aria-labelledby="analytics-heading" class="mb-8">
  <h2 id="analytics-heading" class="text-lg font-semibold text-slate-700 mb-4">Analytics Summary</h2>
  {#if summary?.analytics}
    <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-5 gap-6">
      <CardStats
        title="Total sessions"
        value={String(summary.analytics?.totalSessions ?? 0)}
        subtitle={summary.analytics?.sampleSessions ? `incl. ${summary.analytics.sampleSessions} samples` : undefined}
        icon={Users}
      />
      <CardStats
        title="Recorded sessions"
        value={String((summary.analytics?.totalSessions ?? 0) - (summary.analytics?.sampleSessions ?? 0))}
        subtitle="Excluding sample data"
        icon={Database}
      />
      <CardStats
        title="Total received"
        value={formatBytes(summary.analytics?.totalBytesIn)}
        icon={ArrowDown}
        subtitle={systemInfo?.netInterface || undefined}
      />
      <CardStats
        title="Total sent"
        value={formatBytes(summary.analytics?.totalBytesOut)}
        icon={ArrowUp}
        subtitle={systemInfo?.netInterface || undefined}
      />
      <CardStats
        title="Total traffic"
        value={formatBytes((summary.analytics?.totalBytesIn ?? 0) + (summary.analytics?.totalBytesOut ?? 0))}
        icon={Server}
        subtitle={trafficSubtitle()}
      />
    </div>
  {:else}
    <p class="text-sm text-slate-500">No analytics summary returned yet.</p>
  {/if}
</section>
