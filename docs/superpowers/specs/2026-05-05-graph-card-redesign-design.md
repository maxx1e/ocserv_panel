# Graph Card Redesign — Design Spec

**Date:** 2026-05-05  
**Status:** Approved  
**Scope:** Frontend-only (zero backend changes)

---

## Problem

Dashboard graph cards look oversized, sparse, and visually disconnected from the admin UI:

- Charts are 200px tall in a 2-column grid — feels oversized.
- Axis labels overlap, especially on short time windows.
- Gridlines and chart-area fills are too dominant.
- Card layout lacks the compact, polished feel of the svelteforge reference.
- No dark mode support.
- Tooltip styling is basic (LayerChart defaults).

---

## Design Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Card system | Adopt shadcn-svelte Card primitives | Matches svelteforge 1:1; tiny components; consistent design language |
| Chart wrapper | ChartContainer with deep LayerChart CSS overrides | Solves gridline/axis/tick problems at the container level |
| Chart height | 160px | Compact; denser dashboard; less wasted whitespace |
| Dark mode | Full support with mode-watcher and ThemeToggle in navbar | User-requested; CSS custom properties make it systematic |
| Tooltip | ChartTooltip with dot indicator, formatted value, timestamp | Consistent with svelteforge; better than LayerChart defaults |
| X-axis format | Short time labels ("HH:MM" for same-day) with max 4-5 ticks | Prevents overlap in compact charts |
| Color system | CSS custom properties (--chart-1 through --chart-5) | Enables dark/light variants; consistent across charts |

---

## Architecture

### New Components

#### Card Primitives (`frontend/src/lib/components/ui/card/`)

Five tiny components ported from svelteforge's shadcn-svelte implementation:

- **card.svelte** (Card.Root): `bg-card text-card-foreground flex flex-col gap-6 rounded-xl border py-6 shadow-sm`
- **card-header.svelte**: `grid auto-rows-min grid-rows-[auto_auto] items-start gap-1.5 px-6` with container query for action slots
- **card-content.svelte**: `px-6` wrapper
- **card-title.svelte**: `leading-none font-semibold`
- **card-description.svelte**: `text-muted-foreground text-sm`
- **index.ts**: re-exports

#### Chart System (`frontend/src/lib/components/ui/chart/`)

- **chart-container.svelte** (ChartContainer): Wraps LayerChart with data-chart attribute and deep Tailwind CSS overrides that:
  - Hide tick marks (`[&_.lc-axis-tick]:stroke-0`)
  - Remove axis rules (`[&_.lc-rule-x-line]:stroke-0`)
  - Soften gridlines (`[&_.lc-grid-x-radial-line]:stroke-border`)
  - Keep area paths at full opacity (`[&_.lc-area-path]:opacity-100`)
  - Style tick labels (`[&_.lc-axis-tick-label]:fill-muted-foreground`)
  - Set root container width (`[&_.lc-root-container]:w-full`)
  - Standard text size: `text-xs`

- **chart-tooltip.svelte** (ChartTooltip): Custom tooltip component that:
  - Uses `border-border/50 bg-background rounded-lg border px-2.5 py-1.5 text-xs shadow-xl`
  - Supports indicator types: "dot", "line", "dashed"
  - Shows metric label and formatted value with `font-mono tabular-nums`
  - Integrates with LayerChart's `getTooltipContext()`

- **chart-style.svelte** (ChartStyle): Generates scoped CSS for `--color-{key}` variables per chart instance, supporting both light and dark themes.

- **chart-utils.ts**: ChartConfig type definition, context management (`setChartContext`, `useChart`), and payload helpers.

- **index.ts**: re-exports

#### Dark Mode Infrastructure

- **mode-watcher** (npm dependency): Provides reactive `mode` store and `toggleMode()` function.
- **ThemeToggle.svelte** (`frontend/src/lib/components/ui/ThemeToggle.svelte`): Sun/Moon icon button using `toggleMode()`. Placed in the top navigation bar.
- `{#key mode.current}` wrapper around chart sections to force re-render on theme change.

---

### CSS Token System

Update `frontend/src/app.css` to define semantic CSS custom properties:

```css
:root {
  --background: oklch(1 0 0);
  --foreground: oklch(0.145 0 0);
  --card: oklch(1 0 0);
  --card-foreground: oklch(0.145 0 0);
  --muted: oklch(0.96 0 0);
  --muted-foreground: oklch(0.556 0 0);
  --border: oklch(0.922 0 0);
  --chart-1: oklch(0.585 0.233 277.117);
  --chart-2: oklch(0.68 0.162 149.48);
  --chart-3: oklch(0.715 0.143 215.221);
  --chart-4: oklch(0.627 0.265 303.9);
  --chart-5: oklch(0.769 0.188 85.394);
}

.dark {
  --background: oklch(0.145 0 0);
  --foreground: oklch(0.985 0 0);
  --card: oklch(0.205 0 0);
  --card-foreground: oklch(0.985 0 0);
  --muted: oklch(0.269 0 0);
  --muted-foreground: oklch(0.708 0 0);
  --border: oklch(0.35 0 0);
  --chart-1: oklch(0.673 0.245 277.117);
  --chart-2: oklch(0.74 0.17 149.48);
  --chart-3: oklch(0.77 0.15 215.221);
  --chart-4: oklch(0.7 0.27 303.9);
  --chart-5: oklch(0.82 0.2 85.394);
}
```

Remove the existing hardcoded LayerChart grid/rule/tick overrides from `app.css` — these responsibilities move into `ChartContainer`.

---

### Chart Card Layout

Each metric chart renders as:

```
┌─────────────────────────────────────────────────────┐
│ Card.Root (rounded-xl, border, shadow-sm, py-6)     │
│ ┌─────────────────────────────────────────────────┐ │
│ │ Card.Header (px-6)                              │ │
│ │  [left]  Card.Title: "CPU Utilization"          │ │
│ │          Card.Description: "Last 1h"            │ │
│ │  [right] Latest value: "42.1%" (font-semibold)  │ │
│ └─────────────────────────────────────────────────┘ │
│ ┌─────────────────────────────────────────────────┐ │
│ │ Card.Content (px-6)                             │ │
│ │  ChartContainer (h-[160px] w-full)              │ │
│ │    AreaChart with:                              │ │
│ │    - Subtle gridlines (stroke-border)           │ │
│ │    - No tick marks, no axis rules               │ │
│ │    - Soft gradient fill (area opacity: 0.15)    │ │
│ │    - stroke-2 line                              │ │
│ │    - Highlight point on hover                   │ │
│ │    - ChartTooltip (dot indicator)               │ │
│ └─────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────┘
```

Grid: `grid grid-cols-1 lg:grid-cols-2 gap-4`

---

### X-Axis Improvements

| Window | Format | Max Ticks |
|--------|--------|-----------|
| 15m | `"HH:MM"` (e.g., "14:32") | 4 |
| 1h | `"HH:MM"` (e.g., "14:00") | 5 |
| 6h | `"HH:MM"` (e.g., "09:00") | 5 |
| 24h | `"HH:MM"` or `"Mon DD"` if spans midnight | 5 |

The `formatXAxisTime` function becomes window-aware. Current format `"Mon DD HH:MM"` is too long for 160px charts; shortened to `"HH:MM"` for intraday windows.

---

### MetricChart Refactoring

The existing `MetricChart.svelte` is refactored to:

1. Remove its own `<div style="height: {height}px">` wrapper (height managed by Card.Content/ChartContainer)
2. Remove inline `padding` prop from `<Chart>` (ChartContainer manages spacing)
3. Remove direct `<Axis>` components with `grid` / `rule` (ChartContainer's CSS handles this)
4. Use `AreaChart` component from LayerChart (higher-level API matching svelteforge pattern)
5. Use `ChartTooltip` instead of inline `<Tooltip.Root>` with manual formatting
6. Accept `chartConfig` prop for CSS variable color binding

MetricChart remains as a thin wrapper that accepts `data` + `metric` and renders the AreaChart inside a ChartContainer with the correct series config and color bindings. This preserves the metric-to-color mapping logic and keeps the dashboard page clean.

---

### Tooltip Behavior

- **Trigger:** Hover/focus on chart area (bisect-x mode)
- **Content:**
  - Timestamp in short format (e.g., "May 5, 14:32")
  - Metric label (e.g., "CPU")
  - Formatted value with unit (e.g., "42.1%", "1.2 GiB")
  - Dot indicator colored by metric's chart color
- **Style:** `min-w-[9rem] rounded-lg border border-border/50 bg-background px-2.5 py-1.5 text-xs shadow-xl`

---

### Empty/Loading/Error States

| State | Rendering |
|-------|-----------|
| Loading (no data yet) | 3-4 skeleton bars of varying height within 160px container |
| Empty (metric unavailable) | Dashed-border container, muted text "No data available" |
| Error (monitoring failed) | Red-tinted alert banner (existing pattern, above chart grid) |

---

### Dark Mode Toggle Placement

The ThemeToggle button goes in the existing top navbar / header area of the layout shell. It uses:
- `Sun` icon when in dark mode (clicking switches to light)
- `Moon` icon when in light mode (clicking switches to dark)
- `lucide-svelte` icons for consistency

---

## Files Changed

| File | Action | Description |
|------|--------|-------------|
| `frontend/src/lib/components/ui/card/card.svelte` | CREATE | Card.Root wrapper |
| `frontend/src/lib/components/ui/card/card-header.svelte` | CREATE | Card header with grid layout |
| `frontend/src/lib/components/ui/card/card-content.svelte` | CREATE | Card content wrapper |
| `frontend/src/lib/components/ui/card/card-title.svelte` | CREATE | Title typography |
| `frontend/src/lib/components/ui/card/card-description.svelte` | CREATE | Description typography |
| `frontend/src/lib/components/ui/card/index.ts` | CREATE | Re-exports |
| `frontend/src/lib/components/ui/chart/chart-container.svelte` | CREATE | LayerChart wrapper with CSS overrides |
| `frontend/src/lib/components/ui/chart/chart-tooltip.svelte` | CREATE | Custom tooltip component |
| `frontend/src/lib/components/ui/chart/chart-style.svelte` | CREATE | Dynamic theme CSS generation |
| `frontend/src/lib/components/ui/chart/chart-utils.ts` | CREATE | Config types and context helpers |
| `frontend/src/lib/components/ui/chart/index.ts` | CREATE | Re-exports |
| `frontend/src/lib/components/ui/ThemeToggle.svelte` | CREATE | Dark/light mode toggle |
| `frontend/src/lib/components/ui/MetricChart.svelte` | REWRITE | Use ChartContainer, AreaChart, ChartTooltip |
| `frontend/src/lib/components/ui/CardChart.svelte` | DELETE | Replaced by Card.* + ChartContainer |
| `frontend/src/routes/+page.svelte` | UPDATE | Use Card.* for charts, add {#key mode.current}, 160px |
| `frontend/src/app.css` | UPDATE | Add CSS tokens, dark mode, remove old LayerChart overrides |
| `frontend/src/lib/monitoring.ts` | UPDATE | Shorter x-axis format, fewer ticks, CSS var chart config |
| `frontend/src/routes/+layout.svelte` | UPDATE | Add ThemeToggle to navbar, wrap with ModeWatcher |
| `frontend/package.json` | UPDATE | Add mode-watcher dependency |

---

## What Stays the Same

- All data fetching and monitoring API logic
- Backend (zero changes)
- Health cards, stat cards, audit table (unchanged in this phase)
- WindowSelector component
- UnavailableChips component
- Color mapping strategy per metric (expressed as CSS vars instead of hardcoded rgba)

---

## Acceptance Criteria

1. Graph cards visually match the polished admin-dashboard style from svelteforge-admin-main
2. No overlapping axis labels at any monitoring window (15m, 1h, 6h, 24h)
3. Cards are responsive: single column on mobile, 2-column on lg+
4. Tooltips show on hover with timestamp, metric name, and formatted value
5. Dark/light mode toggle works, charts re-render cleanly on switch
6. Existing metrics/data fetching behavior is preserved (no API changes)
7. Empty/loading states render gracefully
8. Chart height is 160px (compact)
9. Frontend builds without TypeScript errors
10. No backend changes required
