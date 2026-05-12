# Chart Improvements Design

**Date:** 2026-05-12  
**Status:** Approved  
**Scope:** Dashboard, History (Monitoring tab), Session Detail Drawer

---

## Summary

Improve all frontend charts with a consistent "elevated with badges" visual style and replace the two separate VPN RX/TX MultiLineChart cards with a single Sankey chart that shows traffic share per VPN interface.

---

## Goals

1. Apply a unified Material-elevated visual style to all ChartCard instances: colored left-border accent, metric-colored latest-value pill, dashed grid lines, trend badge in footer.
2. Replace the two VPN traffic MultiLineChart cards (inbound + outbound) on the Dashboard and History pages with a single `SankeyChart` component.
3. Dashboard Sankey shows **live rate** (latest sample bytes/sec per interface).
4. History Sankey shows **cumulative bytes** over the selected monitoring window.
5. Session Detail Drawer charts inherit the improved `ChartCard` polish automatically.

---

## Out of Scope

- Adding new metrics or changing metric collection.
- Changing chart types for CPU, Memory, Network, Sessions, or Connection Rate (area charts stay).
- Dark mode support changes (existing mode-watcher behavior unchanged).
- Replacing LayerChart with another library.

---

## Component Design

### Enhanced `ChartCard.svelte`

The existing `ChartCard` wraps `MetricChart` and `MultiLineChart`. Visual changes:

- **Header:** Light slate background (`bg-slate-50`), left colored border accent (4px, color matches metric), title in small uppercase tracking, latest value in a rounded pill badge colored to match the metric.
- **Body:** `MetricChart` receives dashed grid lines (passed via a new `gridDashed` prop defaulting to `true`).
- **Footer:** Trend badge — green pill with `▲ +X vs avg` when latest > avg, red pill with `▼ −X vs avg` when latest < avg. Followed by `min X · max X · {window}` in muted text.
- The `dotColor` already derived from `classes.gradientTop` drives both the left border and pill color.
- No new props on `ChartCard` itself — the badge logic uses existing `stats`, `latest`, and `trendValue` derivations already present.

### `MetricChart.svelte` — dashed grid

Add a `gridDashed?: boolean` prop (default `false`). When true, pass `grid={{ class: 'stroke-slate-200 stroke-dashed' }}` (or equivalent LayerChart grid styling) to both `Axis` placements. `ChartCard` passes `gridDashed={true}`; standalone uses of `MetricChart` (drawer) also pass `gridDashed={true}`.

### New `SankeyChart.svelte`

**Location:** `frontend/src/lib/components/ui/SankeyChart.svelte`

**Props:**
```typescript
type Props = {
  /** Array of interface traffic entries */
  interfaces: { name: string; bytes: number }[];
  /** Label for the source node (e.g. "Total VPN") */
  sourceLabel?: string;
  /** Display mode: 'rate' shows bytes/s label, 'cumulative' shows total bytes */
  mode: 'rate' | 'cumulative';
  /** Pixel height of the SVG canvas. Default 180. */
  height?: number;
};
```

**Data shape for LayerChart `Sankey`:**
- Nodes: `[{ id: 'total' }, { id: 'vpns0' }, { id: 'vpns1' }, ...]`
- Links: `[{ source: 'total', target: 'vpns0', value: bytes0 }, ...]`
- Use `graphFromNode` / `nodesFromLinks` utilities from `layerchart/utils/graph`.

**Color palette:** Up to 6 interfaces — violet-500, cyan-500, amber-500, rose-500, emerald-500, fuchsia-500 (same palette as `MultiLineChart`). Source node uses slate-400.

**Layout:**
- `Sankey` component from `layerchart` with `nodeWidth=18`, `nodePadding=12`, `nodeAlign="left"`.
- Nodes rendered as rounded rects; links rendered as curved paths with matching interface color at 40% opacity.
- Percentage label beside each target node.
- Footer legend row: colored dot + interface name + formatted value (bytes/s or total bytes).
- Empty state: dashed border placeholder ("No VPN interfaces active").

**Value formatting:**
- `mode='rate'`: use `formatBytes(bytes, 'iec') + '/s'`
- `mode='cumulative'`: use `formatBytes(bytes, 'iec')`

### `SankeyChartCard.svelte`

A thin card wrapper matching the `ChartCard` elevated style, specific to the Sankey:

**Props:**
```typescript
type Props = {
  interfaces: { name: string; bytes: number }[];
  totalLabel: string;       // e.g. "14.3 MB/s total" or "42.1 GB window"
  mode: 'rate' | 'cumulative';
  window?: string;          // e.g. "1h", "24h" — shown in footer
  height?: number;
};
```

- Header: violet left-border accent, "VPN TRAFFIC SHARE" label, total formatted value as pill.
- Body: `SankeyChart`.
- Footer: window label in muted text.

---

## Data Derivation

### Dashboard (`DashboardMetrics.svelte`)

Replace `vpnRXLines` + `vpnTXLines` + two `MultiLineChart` cards with:

```typescript
const vpnSankeyInterfaces = $derived<{ name: string; bytes: number }[]>(
  vpnInterfaceNames.map(iface => {
    // Latest point from vpn_interface_rx_bytes + vpn_interface_tx_bytes
    const rxSeries = monitoring?.series?.find(s => s.metric === VPN_RX && s.labels['interface'] === iface);
    const txSeries = monitoring?.series?.find(s => s.metric === VPN_TX && s.labels['interface'] === iface);
    const rx = rxSeries?.points.at(-1)?.value ?? 0;
    const tx = txSeries?.points.at(-1)?.value ?? 0;
    return { name: iface, bytes: rx + tx };
  }).filter(i => i.bytes > 0)
);
```

Total label: sum of all interface bytes, formatted as `X MB/s total`.

### History (`history/+page.svelte` — Monitoring tab)

Replace the same two MultiLineChart cards with:

```typescript
const vpnSankeyInterfaces = $derived<{ name: string; bytes: number }[]>(
  vpnInterfaceNames.map(iface => {
    // Sum all points in window for rx + tx
    const rxSeries = monitoring?.series?.find(s => s.metric === VPN_RX && s.labels['interface'] === iface);
    const txSeries = monitoring?.series?.find(s => s.metric === VPN_TX && s.labels['interface'] === iface);
    const rxTotal = (rxSeries?.points ?? []).reduce((sum, p) => sum + (p.value ?? 0), 0);
    const txTotal = (txSeries?.points ?? []).reduce((sum, p) => sum + (p.value ?? 0), 0);
    return { name: iface, bytes: rxTotal + txTotal };
  }).filter(i => i.bytes > 0)
);
```

Total label: sum formatted as `X GB window` (or MB/KB as appropriate).

### Session Detail Drawer

No data changes — the drawer uses `MetricChart` directly. It benefits from the dashed-grid change when `gridDashed={true}` is passed. The drawer's existing `rxData`, `txData`, `durationData`, `cumulativeData` derivations are unchanged.

---

## Affected Files

| File | Change |
|------|--------|
| `frontend/src/lib/components/ui/ChartCard.svelte` | Add left-border accent, colored pill, trend badge footer |
| `frontend/src/lib/components/ui/MetricChart.svelte` | Add `gridDashed` prop, pass to Axis grid styling |
| `frontend/src/lib/components/ui/SankeyChart.svelte` | **New** — Sankey using layerchart `Sankey` + d3-sankey |
| `frontend/src/lib/components/ui/SankeyChartCard.svelte` | **New** — elevated card wrapper for Sankey |
| `frontend/src/lib/components/dashboard/DashboardMetrics.svelte` | Replace two VPN MultiLineChart cards with `SankeyChartCard`, derive `vpnSankeyInterfaces` |
| `frontend/src/routes/history/+page.svelte` | Replace two VPN MultiLineChart cards with `SankeyChartCard`, derive `vpnSankeyInterfaces` |
| `frontend/src/lib/components/history/UserSessionDrawer.svelte` | Pass `gridDashed={true}` to `MetricChart` calls |

---

## Visual Specification

### ChartCard header

```
┌─────────────────────────────────────────────────────┐
│ ▐ CPU UTILIZATION                      [ 42% ]      │  ← slate-50 bg, 4px indigo border-l, indigo pill
├─────────────────────────────────────────────────────┤
│  [dashed-grid area chart]                            │
├─────────────────────────────────────────────────────┤
│ ▲ +6% vs avg   min 28% · max 71% · 1h               │  ← green badge, muted footer
└─────────────────────────────────────────────────────┘
```

### SankeyChartCard

```
┌─────────────────────────────────────────────────────┐
│ ▐ VPN TRAFFIC SHARE            [ 14.3 MB/s total ]  │  ← violet pill
├─────────────────────────────────────────────────────┤
│                                                      │
│  [Total VPN] ══════════════════ [vpns0] 58%         │
│             ════════           [vpns1] 30%           │
│             ══                 [vpns2] 12%           │
│                                                      │
│  ● vpns0 8.3 MB/s  ● vpns1 4.3 MB/s  ● vpns2 1.7   │
├─────────────────────────────────────────────────────┤
│ Last 1h                                              │
└─────────────────────────────────────────────────────┘
```

---

## Metric Color Map (left-border and pill accent)

| Metric | Color |
|--------|-------|
| `host_cpu_utilization` | indigo-500 |
| `host_memory_utilization` | indigo-500 |
| `net_interface_rx_bytes` | emerald-500 |
| `net_interface_tx_bytes` | sky-500 |
| `active_sessions` | violet-500 |
| `concurrent_sessions` | violet-500 |
| `connection_rate` | amber-500 |
| VPN Sankey | violet-500 |

Colors derived from existing `CHART_COLORS` in `monitoring.ts` — no new color definitions needed.

---

## Non-Goals / Constraints

- Do not remove `MultiLineChart.svelte` — it may be used elsewhere or in future.
- Do not add new backend API endpoints — all data comes from the existing `/api/monitoring/overview` response.
- Do not change the `DonutChart` (Sessions by User on Dashboard) — it is not a ChartCard.
- The `gridDashed` prop on `MetricChart` defaults to `false` so existing standalone usages are unaffected until explicitly opted in.
