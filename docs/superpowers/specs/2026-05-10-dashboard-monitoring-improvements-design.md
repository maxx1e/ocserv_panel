# Dashboard Monitoring Improvements — Design Spec

**Date:** 2026-05-10  
**Status:** Approved

---

## Objective

Improve the dashboard monitoring section with richer metric card context, remove unused sections, add multi-VPN interface tracking, and introduce a multi-line VPN chart.

---

## Changes Summary

| Area | Change |
|------|--------|
| Dashboard cleanup | Remove audit/log activity section, active sessions card, active/concurrent sessions charts |
| Backend system info | New `GET /api/monitoring/system-info` endpoint |
| Backend VPN tracking | Extend collector to track all VPN interfaces, not just the first found |
| Frontend card enrichment | CPU cores subtitle, memory total subtitle, interface name subtitles on network cards and analytics cards |
| Frontend VPN chart | New `MultiLineChart` component; one line per VPN interface |
| Chart style | All charts already use consistent area+gradient style via `MetricChart` — verify during implementation |

---

## Section 1: Dashboard Cleanup

### Remove DashboardActivity

Remove `<DashboardActivity>` from `frontend/src/routes/+page.svelte`:
- Remove the component import
- Remove the `<DashboardActivity recentAudit={recentAudit} />` usage
- Remove the `recentAudit` derived state
- Remove `getLogTail` from the `$lib/api` import

The `DashboardActivity.svelte` file itself can be deleted.

### Remove Active Sessions card

In `frontend/src/lib/components/dashboard/DashboardSessions.svelte`:
- Remove the `<CardStats title="Active sessions" .../>` card
- Keep the `activeUserSessions` prop and the `DonutChart` (sessions-by-user remains)

### Remove Active Sessions and Concurrent Sessions charts

In `frontend/src/lib/components/dashboard/DashboardMetrics.svelte`:
- Remove `ACTIVE_SESSIONS` and `CONCURRENT_SESSIONS` metric name constants
- Remove their entries from `chartDefinitions`

---

## Section 2: System Info Backend Endpoint

### New endpoint

`GET /api/monitoring/system-info`  
Permission: `dashboard:read`

### Response shape

```json
{
  "cpuCores": 4,
  "memoryTotalGB": 15.6,
  "netInterface": "eth0",
  "vpnInterfaces": ["vpns0", "vpns1"]
}
```

### Implementation

**CPU cores** — Read from `runtime.NumCPU()`. Fast and reliable; no `/proc/stat` parsing needed.

**Memory total GB** — Re-use the existing `parseMeminfo()` function in `backend/internal/monitoring/collectors.go`. Read `/proc/meminfo`, extract `MemTotal` (kB), convert to GB. Round to 1 decimal.

**Net interface** — Query the metrics store for the most recent sample of `net_interface_rx_bytes`. Extract `labels["interface"]`. Returns empty string if no net interface sample exists yet.

**VPN interfaces** — Query the metrics store for all distinct `labels["interface"]` values across `vpn_interface_rx_bytes` samples. Returns empty array if no VPN data collected yet.

### Files

- `backend/internal/server/monitoring_helpers.go` — add `systemInfoResponse` struct and helper to build it
- `backend/internal/server/server.go` — register handler at `/api/monitoring/system-info`
- `backend/internal/metrics/types.go` or store — add `DistinctLabels(ctx, metric, labelKey)` method to `Store` interface if not already present, or query via existing range API

---

## Section 3: Multi-VPN Backend Tracking

### Goal

Track all active VPN interfaces simultaneously so the monitoring overview returns one labeled series per interface.

### Changes to `CollectorService`

Replace `hasVPN bool` with `discoveredVPN map[string]bool` (interface name → registered).

Replace `VPNDiscovererFunc` returning `(string, string, error)` with a new `VPNMultiDiscovererFunc` returning `([]string, error)` — a slice of all currently active VPN interface names.

On each rediscovery tick (`maybeDiscoverVPN` → `maybeDiscoverAllVPN`):
1. Call the multi-discoverer to get current interface list
2. For each interface not yet in `discoveredVPN`, create a new `InterfaceCollector` and add it to `collectors`
3. Mark the interface as discovered

### Changes to `vpn_discovery.go`

Add `DiscoverAllVPNInterfaces()` alongside the existing `DiscoverVPNInterface()`. It returns all matching interfaces from `/sys/class/net` (prefixes: `vpns`, `tun`, `tap`, `wg`), not just the first one.

`resolveInterfaceName` is replaced by `resolveAllInterfaceNames(cfg, baseName) []string` which returns every entry in `/sys/class/net` that either matches `baseName` exactly or starts with `baseName` (e.g. `vpns0`, `vpns1`). The ocserv-config strategy uses this to find all numbered instances of the configured device name. The sysfs-scan strategy returns all matching prefixes directly.

### Backward compatibility

Existing single-VPN behavior is preserved when only one interface exists. The `labels["interface"]` field was already set on every sample, so the metrics store requires no changes.

---

## Section 4: Frontend Card Enrichments

### New API type and function

In `frontend/src/lib/api/monitoring.ts`:

```ts
export type SystemInfo = {
  cpuCores: number;
  memoryTotalGB: number;
  netInterface: string;
  vpnInterfaces: string[];
};

export function getSystemInfo(): Promise<SystemInfo> {
  return apiFetch<SystemInfo>('/api/monitoring/system-info');
}
```

### Fetch in +page.svelte

Add `systemInfo = $state<SystemInfo | null>(null)` and fetch alongside the other dashboard loads. Pass to `DashboardMetrics` and `DashboardSessions` as a new `systemInfo` prop. A fetch failure is non-fatal — cards degrade gracefully by showing no subtitle.

### DashboardMetrics card enrichments

| Card | Subtitle when systemInfo available |
|------|------------------------------------|
| CPU | `"{cpuCores} cores"` |
| Memory | `"{memoryTotalGB} GB total"` |
| Network inbound | `"{netInterface}"` |
| Network outbound | `"{netInterface}"` |

### DashboardSessions analytics card enrichments

| Card | Subtitle when systemInfo available |
|------|------------------------------------|
| Total received | `"{netInterface}"` |
| Total sent | `"{netInterface}"` |
| Total traffic | `"{netInterface}` + `{vpnInterfaces.join(', ')}"` |

If `vpnInterfaces` is empty, Total Traffic subtitle shows only `netInterface`. If `netInterface` is empty string, no subtitle is shown.

---

## Section 5: Multi-Line VPN Chart

### New component: `MultiLineChart.svelte`

Location: `frontend/src/lib/components/ui/MultiLineChart.svelte`

```ts
type LineData = {
  name: string;
  data: { timestamp: Date; value: number }[];
};

type Props = {
  lines: LineData[];
  metric: string;
  height?: number;
};
```

Each line renders as an `Area + LinearGradient` pair using the same LayerChart primitives as `MetricChart`. Colors are drawn from a fixed VPN palette (distinct from CPU/memory/network colors):

```ts
const VPN_LINE_COLORS = [
  { stroke: 'stroke-violet-500 stroke-2', top: 'rgba(139, 92, 246, 0.25)', bottom: 'rgba(139, 92, 246, 0.05)' },
  { stroke: 'stroke-fuchsia-500 stroke-2', top: 'rgba(217, 70, 239, 0.25)', bottom: 'rgba(217, 70, 239, 0.05)' },
  { stroke: 'stroke-rose-500 stroke-2', top: 'rgba(244, 63, 94, 0.25)', bottom: 'rgba(244, 63, 94, 0.05)' },
  { stroke: 'stroke-amber-500 stroke-2', top: 'rgba(245, 158, 11, 0.25)', bottom: 'rgba(245, 158, 11, 0.05)' },
];
```

A legend row below the chart shows a colored dot and interface name for each line.

If `lines` is empty or all lines have no data, show the existing "No data available" placeholder.

### DashboardMetrics VPN chart

Replace the two single-line VPN chart entries (`VPN inbound`, `VPN outbound`) with two `MultiLineChart` cards — one for inbound, one for outbound — to stay consistent with the network section layout:

```ts
const vpnInterfaceNames = $derived(
  [...new Set(
    (monitoring?.series ?? [])
      .filter(s => s.metric === VPN_RX)
      .map(s => s.labels['interface'])
      .filter(Boolean)
  )]
);

const vpnRXLines = $derived(
  vpnInterfaceNames.map(iface => ({
    name: iface,
    data: chartDataForMetricAndInterface(VPN_RX, iface),
  }))
);

const vpnTXLines = $derived(
  vpnInterfaceNames.map(iface => ({
    name: iface,
    data: chartDataForMetricAndInterface(VPN_TX, iface),
  }))
);
```

`chartDataForMetricAndInterface(metric, iface)` filters `monitoring.series` for the matching `metric` and `labels.interface`, then maps points to `{ timestamp, value }`.

The two VPN cards show in the chart grid alongside CPU, memory, and network cards. If no VPN data is available, both cards show the "No data available" placeholder.

---

## Section 6: Chart Style Consistency

All existing charts use `MetricChart.svelte` which already applies `Area + LinearGradient` with the correct stroke/fill. No code changes required — verify visually during implementation that all charts render with the filled-area style.

The new `MultiLineChart` mirrors the same LayerChart primitives to stay consistent.

---

## Files Touched

### Backend
- `backend/internal/monitoring/vpn_discovery.go` — add `DiscoverAllVPNInterfaces()`
- `backend/internal/monitoring/collector_service.go` — multi-VPN tracking
- `backend/internal/metrics/types.go` — add `DistinctLabels` to Store interface if needed
- `backend/internal/server/monitoring_helpers.go` — add `systemInfoResponse`, handler helper
- `backend/internal/server/server.go` — register `/api/monitoring/system-info` route

### Frontend
- `frontend/src/lib/api/monitoring.ts` — add `SystemInfo` type and `getSystemInfo()`
- `frontend/src/routes/+page.svelte` — fetch systemInfo, remove DashboardActivity
- `frontend/src/lib/components/dashboard/DashboardMetrics.svelte` — card subtitles, remove session charts, VPN multi-line
- `frontend/src/lib/components/dashboard/DashboardSessions.svelte` — remove active sessions card, add interface subtitles to analytics cards
- `frontend/src/lib/components/dashboard/DashboardActivity.svelte` — **delete**
- `frontend/src/lib/components/ui/MultiLineChart.svelte` — **new file**

---

## Out of Scope

- VPN interface removal detection (when an interface disappears, its collector continues sending unavailable results — no cleanup needed)
- Combining RX+TX on one VPN chart
- User-traffic charts (user_rx_bytes / user_tx_bytes) — not requested
