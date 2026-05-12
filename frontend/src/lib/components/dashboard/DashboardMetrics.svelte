<script lang="ts">
  import MetricChart from '$lib/components/ui/MetricChart.svelte';
  import SankeyChartCard from '$lib/components/ui/SankeyChartCard.svelte';
  import { ArrowDown, ArrowUp, Cpu, MemoryStick } from 'lucide-svelte';
  import CardStats from '$lib/components/ui/CardStats.svelte';
  import * as Card from '$lib/components/ui/card/index';
  import { mode } from 'mode-watcher';
  import WindowSelector from '$lib/components/ui/WindowSelector.svelte';
  import UnavailableChips from '$lib/components/ui/UnavailableChips.svelte';
  import type { Chip } from '$lib/components/ui/UnavailableChips.svelte';
  import {
    formatMetricValue,
    latestPoint,
    metricByName,
    seriesByMetric
  } from '$lib/monitoring';
  import type {
    MonitoringOverview,
    MonitoringSeries,
    MonitoringWindow,
    SystemInfo
  } from '$lib/api';

  type ChartDefinition = {
    metric: string;
    title: string;
  };

  type Props = {
    monitoring: MonitoringOverview | null;
    monitoringLoading: boolean;
    monitoringError: string;
    monitoringWindow: MonitoringWindow;
    onWindowChange: (window: string) => void;
    onRefresh: () => void;
    systemInfo: SystemInfo | null;
  };

  let {
    monitoring,
    monitoringLoading,
    monitoringError,
    monitoringWindow,
    onWindowChange,
    onRefresh,
    systemInfo
  }: Props = $props();

  const MONITORING_WINDOWS: MonitoringWindow[] = ['15m', '1h', '6h', '24h'];

  const HOST_CPU = 'host_cpu_utilization';
  const HOST_MEMORY = 'host_memory_utilization';
  const NET_RX = 'net_interface_rx_bytes';
  const NET_TX = 'net_interface_tx_bytes';
  const VPN_RX = 'vpn_interface_rx_bytes';
  const VPN_TX = 'vpn_interface_tx_bytes';
  const chartDefinitions: ChartDefinition[] = [
    { metric: HOST_CPU, title: 'CPU utilization' },
    { metric: HOST_MEMORY, title: 'Memory utilization' },
    { metric: NET_RX, title: 'Network inbound' },
    { metric: NET_TX, title: 'Network outbound' }
  ];

  function metricSeries(metric: string): MonitoringSeries | undefined {
    return seriesByMetric(monitoring, metric);
  }

  function metricValue(metric: string): string {
    return formatMetricValue(metric, latestPoint(metricSeries(metric))?.value);
  }

  function metricState(metric: string): 'collecting' | 'unavailable' {
    if (metricByName(monitoring?.catalog, metric)?.state === 'unavailable') return 'unavailable';
    return latestPoint(metricSeries(metric)) ? 'collecting' : 'unavailable';
  }

  function unavailableReason(metric: string): string {
    const catalogMetric = metricByName(monitoring?.catalog, metric);
    if (catalogMetric?.reason) return catalogMetric.reason;
    if (catalogMetric?.state === 'unavailable') return 'The backend reports this metric as unavailable.';
    return 'Metric data has not been collected for this window yet.';
  }

  function shouldShowChart(metric: string): boolean {
    if (metricByName(monitoring?.catalog, metric)?.state === 'unavailable') return false;
    return chartData(metric).length > 0;
  }

  function chartData(metric: string): { timestamp: Date; value: number }[] {
    const series = metricSeries(metric);
    if (!series) return [];
    return series.points
      .filter(p => Number.isFinite(Date.parse(p.timestamp)) && Number.isFinite(p.value))
      .map(p => ({ timestamp: new Date(p.timestamp), value: p.value }))
      .sort((a, b) => a.timestamp.getTime() - b.timestamp.getTime());
  }


  const vpnInterfaceNames = $derived(
    [...new Set(
      (monitoring?.series ?? [])
        .filter(s => s.metric === VPN_RX)
        .map(s => s.labels['interface'])
        .filter((v): v is string => Boolean(v))
    )]
  );

  const vpnSankeyInterfaces = $derived(
    vpnInterfaceNames
      .map(iface => {
        const rxSeries = monitoring?.series?.find(
          s => s.metric === VPN_RX && s.labels['interface'] === iface
        );
        const txSeries = monitoring?.series?.find(
          s => s.metric === VPN_TX && s.labels['interface'] === iface
        );
        const rx = rxSeries?.points.at(-1)?.value ?? 0;
        const tx = txSeries?.points.at(-1)?.value ?? 0;
        return { name: iface, bytes: rx + tx };
      })
      .filter(i => i.bytes > 0)
  );

  const unavailableChips = $derived<Chip[]>(
    chartDefinitions
      .filter(chart => !shouldShowChart(chart.metric))
      .map(chart => ({ name: chart.title, reason: unavailableReason(chart.metric) }))
  );
</script>

<section aria-labelledby="monitoring-heading" class="mb-8">
  <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-3 mb-1">
    <div>
      <h2 id="monitoring-heading" class="text-lg font-semibold text-slate-700">Monitoring</h2>
    </div>
    <div class="flex items-center gap-3">
      <WindowSelector
        value={monitoringWindow}
        windows={MONITORING_WINDOWS}
        onchange={onWindowChange}
      />
      <button
        type="button"
        disabled={monitoringLoading}
        onclick={() => onRefresh()}
        class="inline-flex items-center px-3 py-1.5 text-sm font-medium rounded-md bg-white border border-slate-200 text-slate-600 hover:bg-slate-50 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
      >
        {monitoringLoading ? 'Refreshing...' : 'Refresh'}
      </button>
    </div>
  </div>
  <p class="text-sm text-slate-500 mt-1 mb-6">
    Latest backend-collected samples for this node. Session-only metrics remain unavailable until the backend can collect them.
  </p>

  {#if monitoringError}
    <div class="bg-red-50 border border-red-200 rounded-lg p-4 mb-6" role="status">
      <strong class="text-red-800 text-sm">Monitoring unavailable</strong>
      <p class="text-red-700 text-sm mt-1">{monitoringError}</p>
    </div>
  {/if}

  <!-- Metric cards -->
  <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-6 mb-6">
    <CardStats
      title="CPU"
      value={metricValue(HOST_CPU)}
      icon={Cpu}
      unavailable={metricState(HOST_CPU) === 'unavailable'}
      hint={unavailableReason(HOST_CPU)}
      subtitle={systemInfo ? `${systemInfo.cpuCores} cores` : undefined}
    />
    <CardStats
      title="Memory"
      value={metricValue(HOST_MEMORY)}
      icon={MemoryStick}
      unavailable={metricState(HOST_MEMORY) === 'unavailable'}
      hint={unavailableReason(HOST_MEMORY)}
      subtitle={systemInfo?.memoryTotalGB ? `${systemInfo.memoryTotalGB} GB total` : undefined}
    />
    <CardStats
      title="Network inbound"
      value={metricValue(NET_RX)}
      icon={ArrowDown}
      unavailable={metricState(NET_RX) === 'unavailable'}
      hint={unavailableReason(NET_RX)}
      subtitle={systemInfo?.netInterface || undefined}
    />
    <CardStats
      title="Network outbound"
      value={metricValue(NET_TX)}
      icon={ArrowUp}
      unavailable={metricState(NET_TX) === 'unavailable'}
      hint={unavailableReason(NET_TX)}
      subtitle={systemInfo?.netInterface || undefined}
    />
  </div>

  <!-- Chart grid -->
  {#if monitoringLoading && !monitoring}
    <div class="grid grid-cols-1 lg:grid-cols-2 gap-4">
      {#each Array(4) as _}
        <Card.Root>
          <Card.Header>
            <div>
              <div class="h-4 w-24 rounded bg-muted animate-pulse"></div>
              <div class="h-3 w-16 rounded bg-muted animate-pulse mt-1.5"></div>
            </div>
            <div class="h-5 w-12 rounded bg-muted animate-pulse"></div>
          </Card.Header>
          <Card.Content>
            <div class="h-[160px] w-full rounded-lg bg-muted/30 animate-pulse flex items-end gap-1 px-4 pb-4">
              {#each [40, 55, 35, 70, 50, 65, 45, 60, 75, 50, 80, 55] as h}
                <div class="flex-1 rounded-sm bg-muted" style="height: {h}%"></div>
              {/each}
            </div>
          </Card.Content>
        </Card.Root>
      {/each}
    </div>
  {:else}
    {#key mode.current}
      <div class="grid grid-cols-1 lg:grid-cols-2 gap-4">
        {#each chartDefinitions as chart}
          {#if shouldShowChart(chart.metric)}
            <Card.Root>
              <Card.Header>
                <div>
                  <Card.Title>{chart.title}</Card.Title>
                  <Card.Description>Last {monitoringWindow}</Card.Description>
                </div>
                <span class="text-sm font-semibold text-foreground tabular-nums">
                  {metricValue(chart.metric)}
                </span>
              </Card.Header>
              <Card.Content>
                <MetricChart data={chartData(chart.metric)} metric={chart.metric} height={160} />
              </Card.Content>
            </Card.Root>
          {/if}
        {/each}
        <div class="lg:col-span-2">
          <SankeyChartCard
            interfaces={vpnSankeyInterfaces}
            mode="rate"
            window={monitoringWindow}
          />
        </div>
      </div>
    {/key}
    <UnavailableChips chips={unavailableChips} />
  {/if}
</section>
