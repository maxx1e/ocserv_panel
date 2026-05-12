# Chart Improvements Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Improve all frontend charts with an "elevated with badges" visual style and replace the two VPN RX/TX MultiLineChart cards with a single Sankey chart showing traffic share per interface.

**Architecture:** Enhance `ChartCard.svelte` with a colored left-border accent, metric-colored pill, dashed grid, and trend badge footer. Add `gridDashed` prop to `MetricChart.svelte`. Build a new `SankeyChart.svelte` (pure SVG using layerchart's `Sankey` + d3-sankey) and `SankeyChartCard.svelte` wrapper. Replace the two VPN MultiLineChart cards in `DashboardMetrics.svelte` and `history/+page.svelte` with `SankeyChartCard`. Update `UserSessionDrawer.svelte` to use `gridDashed`.

**Tech Stack:** SvelteKit 5 (Svelte runes), Tailwind CSS v4, layerchart 1.0.13, d3-sankey 0.12.3 (already in node_modules)

---

## File Map

| Action | File | Responsibility |
|--------|------|----------------|
| Modify | `frontend/src/lib/components/ui/MetricChart.svelte` | Add `gridDashed` prop; pass `grid` class to `Axis` |
| Modify | `frontend/src/lib/components/ui/ChartCard.svelte` | Left-border accent, colored pill, dashed grid, trend badge footer |
| Create | `frontend/src/lib/components/ui/SankeyChart.svelte` | Pure Sankey SVG via layerchart `Sankey` + manual SVG nodes/links |
| Create | `frontend/src/lib/components/ui/SankeyChartCard.svelte` | Elevated card wrapper for Sankey |
| Modify | `frontend/src/lib/components/dashboard/DashboardMetrics.svelte` | Replace two VPN MultiLineChart cards → `SankeyChartCard` (rate mode) |
| Modify | `frontend/src/routes/history/+page.svelte` | Replace two VPN MultiLineChart cards → `SankeyChartCard` (cumulative mode) |
| Modify | `frontend/src/lib/components/history/UserSessionDrawer.svelte` | Pass `gridDashed={true}` to all `MetricChart` calls |

---

## Task 1: Add `gridDashed` prop to `MetricChart.svelte`

**Files:**
- Modify: `frontend/src/lib/components/ui/MetricChart.svelte`

The layerchart `Axis` component accepts `grid` as either `boolean` or `Pick<SVGAttributes, 'class' | 'style'>`. When `gridDashed` is true, pass `grid={{ class: 'stroke-slate-200 [stroke-dasharray:4_4]' }}` instead of the default `grid` boolean.

- [ ] **Step 1: Read the current MetricChart file**

Read `frontend/src/lib/components/ui/MetricChart.svelte` to confirm the current `Axis` usage.

- [ ] **Step 2: Add `gridDashed` prop and update both Axis placements**

In the `<script>` section, add to the `Props` type:
```typescript
/** When true, renders grid lines as dashes. Default false. */
gridDashed?: boolean;
```

Destructure it:
```typescript
let { data, metric, height = 160, chartType = 'line', xLabelFormat, gridDashed = false }: Props = $props();
```

Define a derived grid prop:
```typescript
const gridProp = $derived(
  gridDashed
    ? { class: 'stroke-slate-200 [stroke-dasharray:4_4]' }
    : true
);
```

In the bar chart `<Chart>` block, change both `<Axis>` elements:
```svelte
<Axis placement="left" grid={gridProp} format={config.yAxisFormat} />
<Axis
  placement="bottom"
  format={(d: string) => xLabelFormat ? xLabelFormat(d) : formatXAxisTime(new Date(d))}
  ticks={xTickCount}
/>
```

In the line chart `<Chart>` block, change the left axis:
```svelte
<Axis placement="left" grid={gridProp} format={config.yAxisFormat} />
<Axis
  placement="bottom"
  format={(d: Date | number) => formatXAxisTime(d)}
  ticks={xTickCount}
/>
```

- [ ] **Step 3: Build to check for TypeScript errors**

```bash
cd frontend && pnpm run check 2>&1 | head -40
```

Expected: no new errors related to `MetricChart`.

- [ ] **Step 4: Commit**

```bash
git add frontend/src/lib/components/ui/MetricChart.svelte
git commit -m "feat(charts): add gridDashed prop to MetricChart"
```

---

## Task 2: Enhance `ChartCard.svelte` with elevated style

**Files:**
- Modify: `frontend/src/lib/components/ui/ChartCard.svelte`

The current `ChartCard` has a white card with a small dot + uppercase label, a bold latest value, and a small footer. Replace with:
- Header: `bg-slate-50` row with 4px left border in the metric accent color, uppercase label, latest value as a colored pill badge.
- Body: pass `gridDashed={true}` to `MetricChart`.
- Footer: green/red trend badge pill + `min X · max X · {window}` muted text.

The `dotColor` already derived from `classes.gradientTop` (e.g. `rgba(99,102,241,0.80)`) will drive both the left border and the pill. To get a Tailwind-compatible hex or CSS color from it, use it directly via `style=` attributes.

- [ ] **Step 1: Read the current ChartCard file**

Read `frontend/src/lib/components/ui/ChartCard.svelte` in full.

- [ ] **Step 2: Replace the template with elevated style**

Replace the entire file content with:

```svelte
<script lang="ts">
  import MetricChart from './MetricChart.svelte';
  import MultiLineChart from './MultiLineChart.svelte';
  import type { LineData } from './MultiLineChart.svelte';
  import {
    getChartClasses,
    formatMetricValue,
    seriesStats,
  } from '$lib/monitoring';

  type TimeData = { timestamp: Date; value: number };

  type Props = {
    title: string;
    metric: string;
    window: string;
    height?: number;
    chartType?: 'line' | 'bar';
  } & (
    | { data: TimeData[]; lines?: never }
    | { lines: LineData[]; data?: never }
  );

  let { title, metric, window: w, height = 160, chartType = 'line', data, lines }: Props = $props();

  const classes = $derived(getChartClasses(metric));

  const allPoints = $derived<TimeData[]>(
    data ? data : (lines ?? []).flatMap(l => l.data)
  );

  const stats = $derived(seriesStats(allPoints));
  const latest = $derived(
    allPoints.length > 0
      ? allPoints.reduce((max, p) => p.timestamp > max.timestamp ? p : max).value
      : null
  );

  const trendValue = $derived(latest !== null ? latest - stats.avg : null);
  const trendFormatted = $derived(
    trendValue !== null
      ? `${trendValue >= 0 ? '▲' : '▼'} ${formatMetricValue(metric, Math.abs(trendValue))} vs avg`
      : null
  );

  // Derive solid accent color from gradientTop rgba string
  // e.g. "rgba(99,102,241,0.30)" → "rgba(99,102,241,0.80)" for border/pill
  const accentColor = $derived(classes.gradientTop.replace(/[\d.]+\)$/, '0.85)'));
  // Pill background: very faint version of accent
  const pillBg = $derived(classes.gradientTop.replace(/[\d.]+\)$/, '0.12)'));
  // Trend badge colors
  const trendPositive = $derived((trendValue ?? 0) >= 0);

  const hasData = $derived(allPoints.length > 0);
</script>

<div class="bg-white border border-slate-200 rounded-xl shadow-sm overflow-hidden">
  <!-- Header -->
  <div
    class="bg-slate-50 px-4 py-2.5 flex items-center justify-between"
    style="border-left: 4px solid {accentColor}"
  >
    <span class="text-[11px] font-bold uppercase tracking-wider text-slate-500">{title}</span>
    {#if latest !== null}
      <span
        class="text-[11px] font-bold px-2.5 py-0.5 rounded-full"
        style="color: {accentColor}; background: {pillBg}"
      >
        {formatMetricValue(metric, latest)}
      </span>
    {:else}
      <span class="text-[11px] text-slate-300">—</span>
    {/if}
  </div>

  <!-- Chart -->
  <div class="px-2 pb-1 pt-1">
    {#if data !== undefined}
      <MetricChart {data} {metric} {height} {chartType} gridDashed={true} />
    {:else if lines !== undefined}
      <MultiLineChart {lines} {metric} {height} />
    {/if}
  </div>

  <!-- Footer -->
  <div class="px-4 pb-3 flex items-center gap-2 flex-wrap">
    {#if hasData && trendFormatted !== null}
      {#if trendPositive}
        <span class="text-[10px] font-medium px-2 py-0.5 rounded-full bg-green-100 text-green-700">
          {trendFormatted}
        </span>
      {:else}
        <span class="text-[10px] font-medium px-2 py-0.5 rounded-full bg-red-100 text-red-700">
          {trendFormatted}
        </span>
      {/if}
    {/if}
    {#if hasData}
      <p class="text-[10px] text-slate-400">
        min {formatMetricValue(metric, stats.min)} · max {formatMetricValue(metric, stats.max)} · {w}
      </p>
    {:else}
      <p class="text-[10px] text-slate-400">No data</p>
    {/if}
  </div>
</div>
```

- [ ] **Step 3: Build to check for TypeScript errors**

```bash
cd frontend && pnpm run check 2>&1 | head -40
```

Expected: no errors from `ChartCard`.

- [ ] **Step 4: Commit**

```bash
git add frontend/src/lib/components/ui/ChartCard.svelte
git commit -m "feat(charts): elevated ChartCard style — accent border, pill badge, trend footer"
```

---

## Task 3: Create `SankeyChart.svelte`

**Files:**
- Create: `frontend/src/lib/components/ui/SankeyChart.svelte`

The layerchart `Sankey` component requires a `Chart` context (from `layerchart`'s `Chart`) with `data` set to `{ nodes, links }`. It exposes `nodes` and `links` via a slot. We then render nodes as SVG `<rect>` and links as SVG `<path>` using d3-sankey's computed `x0/x1/y0/y1` for nodes and `sankeyLinkHorizontal()` for link paths.

Note: layerchart's `Sankey` is a Svelte 3/4 component (uses `export let`), not Svelte 5 runes — but it is called from a Svelte 5 parent, which is fine. Do NOT use `$props()` inside this component when interacting with layerchart slots — use the slot data directly.

The color palette (6 interface colors):
```typescript
const IFACE_COLORS = [
  { solid: 'rgba(139,92,246,0.85)', fill: 'rgba(139,92,246,0.40)' },   // violet-500
  { solid: 'rgba(6,182,212,0.85)',  fill: 'rgba(6,182,212,0.40)' },    // cyan-500
  { solid: 'rgba(245,158,11,0.85)', fill: 'rgba(245,158,11,0.40)' },   // amber-500
  { solid: 'rgba(244,63,94,0.85)',  fill: 'rgba(244,63,94,0.40)' },    // rose-500
  { solid: 'rgba(16,185,129,0.85)', fill: 'rgba(16,185,129,0.40)' },   // emerald-500
  { solid: 'rgba(217,70,239,0.85)', fill: 'rgba(217,70,239,0.40)' },   // fuchsia-500
];
const SOURCE_COLOR = { solid: 'rgba(148,163,184,0.85)', fill: 'rgba(148,163,184,0.40)' }; // slate-400
```

- [ ] **Step 1: Create the file**

Create `frontend/src/lib/components/ui/SankeyChart.svelte` with:

```svelte
<script lang="ts">
  import { Chart, Sankey, Svg } from 'layerchart';
  import { sankeyLinkHorizontal } from 'd3-sankey';
  import { formatBytes } from '$lib/monitoring';

  type InterfaceEntry = { name: string; bytes: number };

  type Props = {
    interfaces: InterfaceEntry[];
    sourceLabel?: string;
    mode: 'rate' | 'cumulative';
    height?: number;
  };

  let { interfaces, sourceLabel = 'Total VPN', mode, height = 180 }: Props = $props();

  const IFACE_COLORS = [
    { solid: 'rgba(139,92,246,0.85)', fill: 'rgba(139,92,246,0.40)' },
    { solid: 'rgba(6,182,212,0.85)',  fill: 'rgba(6,182,212,0.40)' },
    { solid: 'rgba(245,158,11,0.85)', fill: 'rgba(245,158,11,0.40)' },
    { solid: 'rgba(244,63,94,0.85)',  fill: 'rgba(244,63,94,0.40)' },
    { solid: 'rgba(16,185,129,0.85)', fill: 'rgba(16,185,129,0.40)' },
    { solid: 'rgba(217,70,239,0.85)', fill: 'rgba(217,70,239,0.40)' },
  ];
  const SOURCE_COLOR = { solid: 'rgba(148,163,184,0.85)', fill: 'rgba(148,163,184,0.40)' };

  const total = $derived(interfaces.reduce((s, i) => s + i.bytes, 0));

  // Build nodes and links for d3-sankey
  const sankeyData = $derived(() => {
    const nodes = [
      { id: '__source__', label: sourceLabel },
      ...interfaces.map(i => ({ id: i.name, label: i.name }))
    ];
    const links = interfaces.map(i => ({
      source: '__source__',
      target: i.name,
      value: Math.max(i.bytes, 1), // d3-sankey requires value > 0
    }));
    return { nodes, links };
  });

  function colorForIndex(index: number) {
    return IFACE_COLORS[index % IFACE_COLORS.length];
  }

  function formatValue(bytes: number): string {
    if (mode === 'rate') return `${formatBytes(bytes, 'iec')}/s`;
    return formatBytes(bytes, 'iec');
  }

  function pct(bytes: number): string {
    if (total === 0) return '0%';
    return `${Math.round((bytes / total) * 100)}%`;
  }

  const linkPath = sankeyLinkHorizontal();
</script>

<div style="height: {height}px" class="w-full">
  {#if interfaces.length === 0 || total === 0}
    <div class="h-full flex items-center justify-center rounded-lg border border-dashed border-slate-200 bg-slate-50/50">
      <p class="text-sm text-slate-400">No VPN interfaces active</p>
    </div>
  {:else}
    <div style="height: {height - 32}px" class="w-full">
      <Chart
        data={sankeyData()}
        padding={{ left: 8, right: 120, top: 8, bottom: 8 }}
      >
        <Svg>
          <Sankey
            nodeId={(d) => d.id}
            nodeWidth={16}
            nodePadding={14}
            nodeAlign="left"
          >
            {#snippet children({ nodes, links }: { nodes: any[]; links: any[] })}
              <!-- Links -->
              {#each links as link, i}
                {@const iface = interfaces.findIndex(ifc => ifc.name === link.target.id)}
                {@const color = iface >= 0 ? colorForIndex(iface) : SOURCE_COLOR}
                <path
                  d={linkPath(link) ?? ''}
                  stroke={color.fill}
                  stroke-width={Math.max(link.width ?? 1, 1)}
                  fill="none"
                />
              {/each}
              <!-- Nodes -->
              {#each nodes as node}
                {@const isSource = node.id === '__source__'}
                {@const ifaceIdx = interfaces.findIndex(i => i.name === node.id)}
                {@const color = isSource ? SOURCE_COLOR : colorForIndex(ifaceIdx)}
                {@const nodeH = (node.y1 ?? 0) - (node.y0 ?? 0)}
                <rect
                  x={node.x0 ?? 0}
                  y={node.y0 ?? 0}
                  width={(node.x1 ?? 0) - (node.x0 ?? 0)}
                  height={Math.max(nodeH, 4)}
                  rx="3"
                  fill={color.solid}
                />
                {#if !isSource}
                  {@const ifaceEntry = interfaces[ifaceIdx]}
                  <text
                    x={(node.x1 ?? 0) + 8}
                    y={(node.y0 ?? 0) + nodeH / 2 - 5}
                    font-size="9"
                    font-weight="600"
                    fill="#374151"
                    dominant-baseline="middle"
                  >{node.id}</text>
                  <text
                    x={(node.x1 ?? 0) + 8}
                    y={(node.y0 ?? 0) + nodeH / 2 + 7}
                    font-size="8"
                    fill={color.solid}
                    dominant-baseline="middle"
                  >{pct(ifaceEntry?.bytes ?? 0)}</text>
                {/if}
              {/each}
            {/snippet}
          </Sankey>
        </Svg>
      </Chart>
    </div>

    <!-- Legend row -->
    <div class="flex flex-wrap gap-x-4 gap-y-1 px-2 mt-1">
      {#each interfaces as iface, i}
        <div class="flex items-center gap-1.5 text-[10px] text-slate-500">
          <span
            class="inline-block w-2 h-2 rounded-full flex-shrink-0"
            style="background: {colorForIndex(i).solid}"
          ></span>
          <span>{iface.name}</span>
          <span class="font-medium text-slate-700">{formatValue(iface.bytes)}</span>
        </div>
      {/each}
    </div>
  {/if}
</div>
```

- [ ] **Step 2: Build to check for TypeScript / Svelte errors**

```bash
cd frontend && pnpm run check 2>&1 | grep -E "error|Error|SankeyChart" | head -20
```

Expected: no errors from `SankeyChart.svelte`.

**Troubleshooting:** layerchart's `Sankey` uses Svelte 3/4 slot syntax (`<slot nodes={...} links={...} />`). If the `{#snippet children(...)}` pattern does not resolve correctly (type error on slot), replace the snippet with a legacy `let:` slot directive:

```svelte
<Sankey nodeId={(d) => d.id} nodeWidth={16} nodePadding={14} nodeAlign="left"
  let:nodes let:links>
  <!-- same node/link rendering -->
</Sankey>
```

- [ ] **Step 3: Commit**

```bash
git add frontend/src/lib/components/ui/SankeyChart.svelte
git commit -m "feat(charts): add SankeyChart component for VPN traffic share"
```

---

## Task 4: Create `SankeyChartCard.svelte`

**Files:**
- Create: `frontend/src/lib/components/ui/SankeyChartCard.svelte`

A thin elevated card wrapper matching ChartCard style, violet-accented, with "VPN TRAFFIC SHARE" label.

- [ ] **Step 1: Create the file**

Create `frontend/src/lib/components/ui/SankeyChartCard.svelte` with:

```svelte
<script lang="ts">
  import SankeyChart from './SankeyChart.svelte';
  import { formatBytes } from '$lib/monitoring';

  type InterfaceEntry = { name: string; bytes: number };

  type Props = {
    interfaces: InterfaceEntry[];
    mode: 'rate' | 'cumulative';
    window?: string;
    height?: number;
  };

  let { interfaces, mode, window: w, height = 180 }: Props = $props();

  const VIOLET_SOLID = 'rgba(139,92,246,0.85)';
  const VIOLET_PILL_BG = 'rgba(139,92,246,0.12)';

  const total = $derived(interfaces.reduce((s, i) => s + i.bytes, 0));

  const totalLabel = $derived(() => {
    if (total === 0) return '—';
    if (mode === 'rate') return `${formatBytes(total, 'iec')}/s total`;
    return `${formatBytes(total, 'iec')} window`;
  });
</script>

<div class="bg-white border border-slate-200 rounded-xl shadow-sm overflow-hidden">
  <!-- Header -->
  <div
    class="bg-slate-50 px-4 py-2.5 flex items-center justify-between"
    style="border-left: 4px solid {VIOLET_SOLID}"
  >
    <span class="text-[11px] font-bold uppercase tracking-wider text-slate-500">VPN Traffic Share</span>
    <span
      class="text-[11px] font-bold px-2.5 py-0.5 rounded-full"
      style="color: {VIOLET_SOLID}; background: {VIOLET_PILL_BG}"
    >
      {totalLabel()}
    </span>
  </div>

  <!-- Chart -->
  <div class="px-2 pb-1 pt-1">
    <SankeyChart {interfaces} {mode} {height} />
  </div>

  <!-- Footer -->
  <div class="px-4 pb-3">
    {#if w}
      <p class="text-[10px] text-slate-400">Last {w}</p>
    {/if}
  </div>
</div>
```

- [ ] **Step 2: Build check**

```bash
cd frontend && pnpm run check 2>&1 | grep -E "error|Error|SankeyChartCard" | head -20
```

Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add frontend/src/lib/components/ui/SankeyChartCard.svelte
git commit -m "feat(charts): add SankeyChartCard wrapper component"
```

---

## Task 5: Update `DashboardMetrics.svelte` — VPN Sankey (rate mode)

**Files:**
- Modify: `frontend/src/lib/components/dashboard/DashboardMetrics.svelte`

Replace the two `<Card.Root>` blocks containing `<MultiLineChart lines={vpnRXLines} ...>` and `<MultiLineChart lines={vpnTXLines} ...>` with a single `<SankeyChartCard>`. Add the `vpnSankeyInterfaces` derived value.

- [ ] **Step 1: Read the current DashboardMetrics file**

Read `frontend/src/lib/components/dashboard/DashboardMetrics.svelte` in full to confirm current import list and VPN card markup.

- [ ] **Step 2: Add import and derived value**

In the `<script>` section, add the import:
```typescript
import SankeyChartCard from '$lib/components/ui/SankeyChartCard.svelte';
```

Remove the `vpnRXLines` and `vpnTXLines` derived values and replace with:
```typescript
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
```

- [ ] **Step 3: Replace the two VPN card blocks in the template**

Find and remove both `<Card.Root>` blocks that contain `MultiLineChart lines={vpnRXLines}` and `MultiLineChart lines={vpnTXLines}`. Replace them with a single card spanning both grid columns:

```svelte
<div class="lg:col-span-2">
  <SankeyChartCard
    interfaces={vpnSankeyInterfaces}
    mode="rate"
    window={monitoringWindow}
  />
</div>
```

- [ ] **Step 4: Remove unused `LineData` import if no longer needed**

Check whether `MultiLineChart` or `LineData` is still referenced anywhere in the file. If not, remove:
```typescript
import MultiLineChart from '$lib/components/ui/MultiLineChart.svelte';
import type { LineData } from '$lib/components/ui/MultiLineChart.svelte';
```

- [ ] **Step 5: Build check**

```bash
cd frontend && pnpm run check 2>&1 | grep -E "error|Error" | head -20
```

Expected: no errors.

- [ ] **Step 6: Commit**

```bash
git add frontend/src/lib/components/dashboard/DashboardMetrics.svelte
git commit -m "feat(dashboard): replace VPN MultiLine charts with SankeyChartCard (rate mode)"
```

---

## Task 6: Update `history/+page.svelte` — VPN Sankey (cumulative mode)

**Files:**
- Modify: `frontend/src/routes/history/+page.svelte`

Same replacement as Task 5 but for the History page monitoring tab, using `mode="cumulative"`.

- [ ] **Step 1: Read the history page to find VPN card markup**

Read `frontend/src/routes/history/+page.svelte` and search for `vpnRXLines`, `vpnTXLines`, and the two VPN `<Card.Root>` blocks in the monitoring tab section.

- [ ] **Step 2: Add import**

In the `<script>` section add:
```typescript
import SankeyChartCard from '$lib/components/ui/SankeyChartCard.svelte';
```

- [ ] **Step 3: Add `vpnSankeyInterfaces` derived value**

The history page likely already derives `vpnRXLines` / `vpnTXLines`. After the existing `vpnInterfaceNames` derivation, add:

```typescript
const vpnSankeyInterfaces = $derived(
  vpnInterfaceNames
    .map(iface => {
      const rxSeries = monitoring?.series?.find(
        s => s.metric === VPN_RX && s.labels['interface'] === iface
      );
      const txSeries = monitoring?.series?.find(
        s => s.metric === VPN_TX && s.labels['interface'] === iface
      );
      const rxTotal = (rxSeries?.points ?? []).reduce((sum, p) => sum + (p.value ?? 0), 0);
      const txTotal = (txSeries?.points ?? []).reduce((sum, p) => sum + (p.value ?? 0), 0);
      return { name: iface, bytes: rxTotal + txTotal };
    })
    .filter(i => i.bytes > 0)
);
```

- [ ] **Step 4: Replace the two VPN Card.Root blocks**

In the monitoring tab template (inside the `{#key mode.current}` block or equivalent), find the two `<Card.Root>` blocks containing VPN inbound and outbound `MultiLineChart`. Replace them with:

```svelte
<div class="lg:col-span-2">
  <SankeyChartCard
    interfaces={vpnSankeyInterfaces}
    mode="cumulative"
    window={monitoringWindow}
  />
</div>
```

- [ ] **Step 5: Remove unused VPN line derivations if no longer referenced**

Remove `vpnRXLines`, `vpnTXLines` derived values if they are only used in the two removed card blocks. Also remove `MultiLineChart` import if no longer referenced.

- [ ] **Step 6: Build check**

```bash
cd frontend && pnpm run check 2>&1 | grep -E "error|Error" | head -20
```

Expected: no errors.

- [ ] **Step 7: Commit**

```bash
git add frontend/src/routes/history/+page.svelte
git commit -m "feat(history): replace VPN MultiLine charts with SankeyChartCard (cumulative mode)"
```

---

## Task 7: Update `UserSessionDrawer.svelte` — dashed grid

**Files:**
- Modify: `frontend/src/lib/components/history/UserSessionDrawer.svelte`

The drawer renders multiple `MetricChart` instances for per-session metrics. Pass `gridDashed={true}` to each call.

- [ ] **Step 1: Read the drawer file**

Read `frontend/src/lib/components/history/UserSessionDrawer.svelte` and identify all `<MetricChart` occurrences.

- [ ] **Step 2: Add `gridDashed` to every MetricChart call**

For each `<MetricChart` tag in the template (there should be 3-4 for rxData, txData, durationData, cumulativeData), add `gridDashed={true}`:

```svelte
<MetricChart data={rxData} metric="user_rx_bytes" height={120} gridDashed={true} />
<MetricChart data={txData} metric="user_tx_bytes" height={120} gridDashed={true} />
<MetricChart data={durationData} metric="session_duration_minutes" height={120} gridDashed={true} />
<MetricChart data={cumulativeData} metric="session_bytes" height={120} gridDashed={true} />
```

Adjust `height` values to match what is already in the file — only add `gridDashed={true}`, do not change anything else.

- [ ] **Step 3: Build check**

```bash
cd frontend && pnpm run check 2>&1 | grep -E "error|Error" | head -20
```

Expected: no errors.

- [ ] **Step 4: Commit**

```bash
git add frontend/src/lib/components/history/UserSessionDrawer.svelte
git commit -m "feat(drawer): use gridDashed on MetricChart in session detail drawer"
```

---

## Task 8: Final build verification

- [ ] **Step 1: Full TypeScript/Svelte check**

```bash
cd frontend && pnpm run check 2>&1
```

Expected: exit 0, no errors.

- [ ] **Step 2: Production build**

```bash
cd frontend && pnpm run build 2>&1 | tail -20
```

Expected: build succeeds with no errors. Warnings about unused CSS are acceptable.

- [ ] **Step 3: Start dev server and visually verify**

```bash
cd frontend && pnpm run dev &
```

Open the dashboard at `http://localhost:5173` (or whatever port is shown). Verify:
- CPU, Memory, Network charts show left-border accent, colored pill, dashed grid lines, trend badge in footer.
- VPN Traffic Share Sankey card is visible with flows from "Total VPN" → per-interface nodes, with percentage labels and legend row.
- If no VPN interfaces are active, the Sankey card shows the empty state placeholder.

Open the History page → Monitoring tab. Verify:
- Same elevated ChartCard style on metric charts.
- VPN Sankey card shows cumulative bytes with "X window" footer.

Open a session detail drawer. Verify:
- Per-session MetricChart instances have dashed grid lines.

- [ ] **Step 4: Update memory index**

In `.claude/rules/memory.md`, add the plan entry under Implementation Plans:

```markdown
| 2026-05-12 | `2026-05-12-chart-improvements.md` | Chart improvements: elevated ChartCard, dashed grid, Sankey VPN chart | |
```

- [ ] **Step 5: Final commit**

```bash
git add .claude/rules/memory.md
git commit -m "docs: add chart-improvements plan to memory index"
```
