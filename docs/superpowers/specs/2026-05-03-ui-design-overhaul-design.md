# UI Design Overhaul — ocserv Panel

**Date:** 2026-05-03
**Status:** Approved
**Approach:** Full page rebuild (Approach 2)

## Summary

Full visual overhaul of the ocserv panel frontend, adopting the notus-svelte reference design language with Tailwind CSS, Lucide icons, and LayerChart. Each page is rebuilt from scratch while preserving all existing functionality and API contracts.

## Design Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Scope | Full visual overhaul | User chose comprehensive redesign |
| Accent color | Slate/neutral | Understated, professional, lets data speak |
| Sidebar | Slate-700 background | Middle ground — dark enough for separation, matches slate theme |
| Icons | Lucide (lucide-svelte) | Svelte-native, tree-shakable, MIT licensed, no CDN |
| Charts | LayerChart | Svelte-native, D3-based, handles time/distribution/categorical metrics |
| Dashboard layout | Card grid (flat) | White cards on gray background, no colored header bar |
| Top navbar | Yes | Page title + user dropdown, polished layout feel |
| Footer | No | Admin tool, not marketing site |
| Implementation | Full page rebuild | Each page rebuilt from scratch, one at a time |

## Constraints

- **No CDN dependencies** — all dependencies are npm packages bundled locally
- **No backend changes** — only the visual layer changes; all API endpoints and data flow preserved
- **No functionality loss** — all existing features must work identically after rebuild
- **Single-node, local-first** — no external services introduced

## Foundation

### Tailwind CSS Setup

- Tailwind CSS v4 as dev dependency
- `tailwind.config.js` with custom slate color palette and content paths for Svelte files
- `@tailwindcss/forms` plugin for form element styling
- Small `app.css` for any truly custom styles that don't map to utilities
- Replace current `app.css` custom styles with Tailwind utility classes

### Design Tokens (Slate Theme)

| Token | Value | Usage |
|-------|-------|-------|
| Primary accent | `slate-600` (#475569) | Buttons, active states, links |
| Sidebar background | `slate-700` (#334155) | Sidebar panel |
| Sidebar text inactive | `slate-300` (#cbd5e1) | Inactive nav items |
| Sidebar text active | `slate-200` (#e2e8f0) | Active nav item text |
| Sidebar active bg | `bg-slate-600/30` | Active nav item background |
| Content background | `slate-50` (#f8fafc) | Page background |
| Card background | `white` | Stat cards, chart cards, tables |
| Card shadow | `shadow-sm` | Subtle elevation |
| Rounded corners | `rounded-lg` (8px) | Cards, buttons, inputs |
| Font | System font stack | Tailwind default |

### Dependencies to Add

- `tailwindcss` (v4)
- `@tailwindcss/forms`
- `lucide-svelte`
- `layerchart` (with D3 peer dependencies)

## Layout Shell

### Sidebar (slate-700)

- Fixed position, `w-64` on desktop, collapsible on mobile
- **Top:** Brand name "OCSERV" in white, bold, with Lucide shield/lock icon
- **Navigation sections** with subtle section headings:
  - **Main:** Dashboard, Users, Sessions, History
  - **Management:** Config, Certificates
  - **System:** Audit, Settings
- **Nav items:** Lucide icon + label, `slate-300` inactive, `slate-200` active with `bg-slate-600/30` highlight
- **Bottom:** User identity section (username + role badge) + logout button
- **Mobile:** Slides in from left with overlay backdrop, hamburger toggle in navbar

### Top Navbar (white)

- `h-16`, `px-6`, `bg-white`, `border-b border-slate-200`
- **Left:** Dynamic page title based on current route
- **Right:** User dropdown (avatar circle with initials, dropdown with Profile, Settings, Logout)
- Clean, minimal — no search bar (single-user admin tool)

### Content Area

- `ml-64` on desktop (offset by sidebar width)
- `pt-16` (offset by navbar height) + `px-6` padding
- `bg-slate-50` background
- Scrollable independently

## Shared Components

### CardStats

- White card, `shadow-sm`, `rounded-lg`, `p-6`
- Icon in `bg-slate-100` circle (40x40px) on the left
- Stat title (small, `text-slate-500`, uppercase) and value (large, `text-slate-700`, bold)
- Optional trend indicator: green arrow up or red arrow down with percentage
- Used on: Dashboard, History page

### CardChart

- White card, `shadow-sm`, `rounded-lg`
- Header: title (`font-semibold text-slate-700`) + optional time range selector
- Body: LayerChart component filling the card
- Used on: Dashboard monitoring, History analytics

### CardTable

- White card, `shadow-sm`, `rounded-lg`
- Header: title + optional action button
- Table: `text-sm`, `text-slate-600`, rows with `hover:bg-slate-50`
- Used on: Dashboard audit, Users, Sessions, Audit pages

### CardSettings

- White card, `shadow-sm`, `rounded-lg`
- Header: section title + description
- Body: form fields with `@tailwindcss/forms` styling
- Used on: Settings, Config pages

### EmptyState

- Centered Lucide icon + message + optional action button
- Used on: placeholder pages (Users, Sessions, Certificates until built out)

## Page Designs

### Dashboard Page

- **Top row:** 4 CardStats in `grid-cols-1 md:grid-cols-2 xl:grid-cols-4`
  - Total Users (Users icon)
  - Active Sessions (Link icon)
  - Bandwidth (Activity icon)
  - Uptime (Zap icon)
- **Middle row:** 2 CardCharts in `grid-cols-1 lg:grid-cols-2`
  - Bandwidth Over Time (area chart)
  - Active Sessions (line chart)
- **Bottom row:** CardTable for recent audit events (last 10)
- Window selector for monitoring time range stays as dropdown in chart card headers

### History Page

- Top: CardStats row (history-specific metrics)
- Middle: CardCharts for analytics (bandwidth, CPU, memory, sessions over time)
- Bottom: AnalyticsCard components become CardTable or CardChart wrappers

### Settings Page

- Left sidebar nav (SettingsNav) with Tailwind styling
- Right content area: CardSettings for each settings section
- SettingField/SettingToggle get Tailwind form styling via `@tailwindcss/forms`
- RestartBanner becomes `bg-amber-50 border-amber-200` alert card

### Placeholder Pages (Users, Sessions, Config, Certificates, Audit)

- Each gets an EmptyState component with relevant icon and message
- Clean, consistent look even though not fully built out

### Login Page

- Centered card on `bg-slate-50` background
- Logo/brand at top
- Form fields with `@tailwindcss/forms` styling
- Slate accent button for submit

## Implementation Order

### Phase 1 — Foundation

1. Install Tailwind CSS v4, lucide-svelte, and LayerChart as npm dependencies
2. Configure Tailwind with slate theme tokens and content paths
3. Set up `app.css` with Tailwind directives, remove old custom CSS
4. Create shared components: CardStats, CardChart, CardTable, CardSettings, EmptyState

### Phase 2 — Layout Shell

5. Rebuild `+layout.svelte` with slate sidebar + top navbar + content area
6. Add mobile responsive behavior (sidebar toggle, responsive grid)

### Phase 3 — Pages (one at a time, each fully rebuilt)

7. Dashboard page — most complex, validates all shared components
8. Login page — simple, validates form styling
9. Settings page — validates CardSettings and form patterns
10. History page — validates CardChart and analytics patterns
11. Placeholder pages (Users, Sessions, Config, Certificates, Audit) — EmptyState components

### Phase 4 — Cleanup

12. Remove old `app.css` custom styles no longer used
13. Remove old components fully replaced
14. Verify all pages render correctly and all functionality preserved

## What Gets Preserved

- All existing API endpoints and data flow (no backend changes)
- All existing SvelteKit routes and page structure
- All existing functionality (monitoring, settings, history analytics, etc.)
- Only the visual layer changes — components get rebuilt with Tailwind + new design

## What Gets Removed

- Current `app.css` custom styles (replaced by Tailwind utilities)
- Current custom CSS components (MetricCard, TimeSeriesChart, etc.) — replaced by new shared components
- Current sidebar and layout CSS — replaced by Tailwind-styled layout shell