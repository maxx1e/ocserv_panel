# UI Polish — Dashboard & History Page Improvements

**Date:** 2026-05-04  
**Status:** Approved  
**Scope:** Frontend only — Dashboard (`/`) and History (`/history`) pages, plus new shared utilities and components

---

## Objective

Polish the two fully-implemented pages (Dashboard and History) with four targeted improvements:

1. Collapse empty charts — no broken empty frames
2. Surface unavailable metrics as dimmed chips below the chart grid
3. Add colored status dot indicators to top-level status cards
4. Add human-readable timestamps and client-side pagination to tables

No new backend APIs are required. No loading-state changes. No stub pages are touched.

---

## Approach

Utility-first incremental patch (Option 1). Add shared utility functions and small helper components, then update the two pages to use them. Changes are narrow and isolated — each improvement is independently reviewable.

---

## New Files

### `frontend/src/lib/utils/format.ts`

Pure formatting functions. No Svelte dependencies.

```ts
// Human-readable relative time from ISO string
// "2 minutes ago", "yesterday at 14:30", falls back to locale date for older entries
formatRelativeTime(iso: string): string

// Full absolute datetime string, suitable for title/tooltip attributes
formatAbsoluteTime(iso: string): string
```

- `formatRelativeTime` thresholds:
  - < 60s → "just now"
  - < 60m → "N minutes ago"
  - < 24h → "N hours ago" or "today at HH:MM"
  - < 48h → "yesterday at HH:MM"
  - older → locale date + time (e.g. "4 May 2026, 14:30")
- Both functions return a safe fallback (`'—'`) for empty, null, or unparseable inputs.

---

### `frontend/src/lib/utils/pagination.ts`

Pure pagination utility. No Svelte dependencies.

```ts
type PaginationResult<T> = {
  items: T[];
  totalPages: number;
  page: number;        // clamped to [1, totalPages]
  pageSize: number;
};

paginate<T>(items: T[], page: number, pageSize: number): PaginationResult<T>
```

- Default page size: 10
- `page` is 1-indexed
- Returns empty `items` array (not error) when `items` is empty
- `totalPages` is at least 1 even for empty input

---

### `frontend/src/lib/components/ui/StatusDot.svelte`

Colored dot indicator. Props:

```ts
type Props = {
  status: string;    // maps to color
  size?: 'sm' | 'md';  // default 'sm'
};
```

Status-to-color mapping:

| Status values | Color |
|---|---|
| `running`, `active`, `success`, `ok` | Green (`bg-emerald-500`) |
| `stopped`, `failure`, `error`, `failed` | Red (`bg-red-500`) |
| `not_configured`, `unknown`, `warning`, `starting`, `stopping` | Amber (`bg-amber-400`) |
| anything else | Slate (`bg-slate-400`) |

Renders as a small filled circle (`rounded-full`), inline-flex, vertically centered. No text label — the value text in `CardStats` already provides that.

---

### `frontend/src/lib/components/ui/UnavailableChips.svelte`

Dimmed chips row for unavailable/empty metrics. Props:

```ts
type Chip = {
  name: string;    // display name shown on chip
  reason: string;  // shown on hover (title attribute)
};

type Props = {
  chips: Chip[];
};
```

- Hidden entirely (renders nothing) when `chips` is empty
- Chips are small, muted (`text-slate-400`, `bg-slate-100`, `border border-slate-200`)
- Hover reveals reason via native `title` attribute
- Prefix label: "Unavailable:" in small uppercase slate text
- Renders as a flex-wrap row

---

### `frontend/src/lib/components/ui/PaginatedTable.svelte`

Pagination wrapper for `CardTable`. Props:

```ts
type Props = {
  totalPages: number;
  currentPage: number;
  onPageChange: (page: number) => void;
  children: Snippet;  // table body content
};
```

- Renders `{@render children()}` then a pagination footer below
- Footer: `← Prev  |  Page N of M  |  Next →`
- Prev disabled on page 1, Next disabled on last page
- Hidden entirely when `totalPages <= 1`

---

## Modified Files

### `frontend/src/lib/components/ui/CardStats.svelte`

Add one optional prop:

```ts
statusValue?: string;  // when present, renders a StatusDot after the value text
```

Existing usages without `statusValue` are unaffected. The dot renders inline-flex after the value `<p>`, vertically centered, with a small left margin.

---

### `frontend/src/routes/+page.svelte` (Dashboard)

**Status cards:**
- `CardStats` for "Backend status" and "ocserv status" receive `statusValue` prop
- Backend status: `statusValue={summary.backendStatus ?? 'unknown'}`
- ocserv status: `statusValue={summary.ocserv?.status ?? summary.ocservStatus ?? 'not_configured'}`

**Chart grid:**
- `shouldShowChart(metric)` extended: returns `true` only when catalog state is not `unavailable` AND `chartData(metric).length > 0`
- Charts with zero points are excluded from the `{#each}` render entirely

**Unavailable chips row:**
- After the chart grid `<div>`, add `<UnavailableChips chips={unavailableChips} />`
- `unavailableChips` is a `$derived` array built from `chartDefinitions` where `!shouldShowChart(chart.metric)`
- Each chip: `{ name: chart.title, reason: unavailableReason(chart.metric) }`

**Audit table:**
- Wrap audit table with `PaginatedTable`
- Add `let auditPage = $state(1)` 
- `$derived` slice: `paginate(recentAudit, auditPage, 10).items`
- `createdAt` / `timestamp` column: `formatRelativeTime(event.createdAt ?? event.timestamp)` with `title={event.createdAt ?? event.timestamp ?? ''}` for hover
- Reset `auditPage` to 1 on dashboard reload

---

### `frontend/src/routes/history/+page.svelte` (History)

**Chart grid:**
- Add `shouldShowChart(metric)` function (mirrors Dashboard): returns `true` only when catalog state is not `unavailable` AND `chartData(metric).length > 0`
- The existing `{#each chartDefinitions as chart}` loop gains the same guard as Dashboard
- `UnavailableChips` row after monitoring chart grid
- `unavailableChips` derived from `chartDefinitions` filtered by `!shouldShowChart(metric)` — reacts automatically when `monitoring` state changes (window switch)

**Historical sessions table:**
- Add `let sessionsPage = $state(1)`
- `$derived` paginated slice: `paginate(sessions, sessionsPage, 10)`
- `connectedAt` / `startedAt` column: `formatRelativeTime(...)` with raw ISO in `title`
- `disconnectedAt` / `endedAt` column: same
- Wrap with `PaginatedTable`
- Reset `sessionsPage` to 1 when `analytics` reloads

**Monitoring generated-at timestamp:**
- Replace inline `formatGeneratedAt()` function with `formatAbsoluteTime()` from `$lib/utils/format`
- Remove the now-unused `formatGeneratedAt` function from the script block

---

## What Is Not Changing

- Loading states — full-page spinner kept as-is
- Stub pages (Sessions, Users, Config, Certificates, Audit) — not touched
- Backend — no new API endpoints
- Settings page — not touched
- Sidebar, Navbar, layout — not touched
- `CardChart`, `CardTable` internals — not modified (only wrapped or extended via props)

---

## Testing Notes

- `format.ts` and `pagination.ts` are pure functions — unit-testable without Svelte
- `StatusDot`, `UnavailableChips`, `PaginatedTable` are presentational — test via visual review and prop edge cases
- Dashboard and History page changes are integration-level; verify with browser against backend running in sample-data mode
- Key edge cases to verify:
  - Window switch (15m → 24h) collapses/expands chips correctly
  - Empty sessions list → `PaginatedTable` footer hidden
  - Single-page result → pagination footer hidden
  - Timestamps with `null`/`undefined` values → display `'—'`
  - `not_configured` status → amber dot
  - `running` status → green dot

---

## File Summary

| File | Action |
|---|---|
| `frontend/src/lib/utils/format.ts` | New |
| `frontend/src/lib/utils/pagination.ts` | New |
| `frontend/src/lib/components/ui/StatusDot.svelte` | New |
| `frontend/src/lib/components/ui/UnavailableChips.svelte` | New |
| `frontend/src/lib/components/ui/PaginatedTable.svelte` | New |
| `frontend/src/lib/components/ui/CardStats.svelte` | Modified (add `statusValue` prop) |
| `frontend/src/routes/+page.svelte` | Modified (4 improvements) |
| `frontend/src/routes/history/+page.svelte` | Modified (3 improvements) |
