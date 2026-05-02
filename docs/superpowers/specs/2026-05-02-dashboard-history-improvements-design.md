# Dashboard & History Improvements Design

## Overview

Four features to improve the Dashboard and History pages:

1. **Dashboard auto-refresh** — live polling for monitoring data
2. **Analytics Summary cards** — human-readable formatted values in card layout
3. **VPN interface auto-discovery** — OS-level detection of the VPN interface
4. **Settings service & housekeeping** — unified settings CRUD with auto-purge scheduling

## Section 1: Dashboard Auto-Refresh

### Problem
The Dashboard page fetches monitoring data once on load. Users must manually refresh to see updated metrics.

### Solution
Add a `setInterval` that re-polls `/api/monitoring/overview` every 15 seconds (matching the backend collection interval).

### Frontend changes
- `+page.svelte`: Add a reactive interval that calls the existing `loadMonitoringOverview()` function every 15s, starting after initial load and clearing on component destroy.
- The interval is only active when the page is visible (using `document.visibilitychange` or Svelte's `onMount`/`onDestroy` lifecycle).
- No backend changes needed — the existing endpoint already returns fresh data.

### Error handling
If a poll fails, keep the last good data and retry on the next interval. Don't show error states for transient failures.

### History page
The History page monitoring charts should also get the same auto-refresh behavior.

---

## Section 2: Analytics Summary Cards

### Problem
The Dashboard shows raw key-value pairs (e.g., `totalBytesIn: 1048576`) with no formatting or visual structure.

### Solution
Replace the raw list with a card grid showing formatted values.

### Frontend changes
- New `AnalyticsCard.svelte` component: takes a label, value (number), and format hint (bytes, count, percent). Uses existing `formatBytes()` and `formatMetricValue()` from `monitoring.ts`.
- `+page.svelte`: Replace `Object.entries(summary.analytics)` rendering with a grid of `AnalyticsCard` components, each with appropriate format hints.
- Cards: Total Sessions (count), Sample Sessions (count), Total Bytes In (bytes), Total Bytes Out (bytes), Total Traffic (bytes, sum of in+out).

### Backend changes
None. The existing `AnalyticsSummary` struct returns raw numbers. Formatting is a presentation concern handled client-side.

### Layout
2×2 grid on desktop, single column on mobile. Each card has a label, large formatted value, and a subtle unit indicator.

---

## Section 3: VPN Interface Auto-Discovery

### Problem
The `InterfaceCollector` needs an explicit interface name and returns "unavailable" if empty. VPN interface metrics are currently disabled.

### Solution
Add OS-level discovery to automatically find the VPN interface at collector service startup.

### Discovery strategy (priority order)
1. **Check ocserv config**: Parse `ocserv.conf` for the `device` directive (e.g., `device = tun0`). Most reliable source since ocserv explicitly declares which device it uses.
2. **Scan `/sys/class/net/`**: Look for interfaces matching common VPN patterns (`tun*`, `tap*`, `wg*`). If exactly one matches, use it. If multiple match, prefer `tun0` or the first match.
3. **Fallback**: If neither source finds an interface, mark VPN metrics as "unavailable" with reason "no VPN interface detected" — same behavior as now but with a clearer message.

### Backend changes
- New `monitoring/vpn_discovery.go`: `DiscoverVPNInterface()` function implementing the strategy above. Returns `(ifaceName string, source string, err error)`.
- `collector_service.go`: At startup, call `DiscoverVPNInterface()`. If found, create an `InterfaceCollector` with the discovered name and add it to the collector list. Log the discovery result (interface name + source).
- Re-discovery: On collector service restart or SIGHUP, re-run discovery. No runtime re-discovery — VPN interfaces don't change while ocserv is running.

### Ocserv config parsing
Read-only. Look for the `device` line in the default ocserv config path (`/etc/ocserv/ocserv.conf`). If the file doesn't exist or isn't readable, skip to step 2.

### Metric catalog update
When VPN interface is discovered, update the metric catalog entries for `vpn_interface_rx_bytes` and `vpn_interface_tx_bytes` from `unavailable` to `collecting`.

---

## Section 4: Settings Service & Housekeeping

### Problem
No auto-purge for historical data. Retention is only configurable per-purge-action, not as a persistent setting. No general settings infrastructure for future configuration.

### Solution
A new `SettingsService` provides CRUD for app settings stored in `app_settings`, starting with `history.retention_days`. A background goroutine runs purge daily using the configured retention.

### Backend changes

#### SettingsService (`app/settings_service.go`)
- `Get(key string) (string, error)` — read a setting
- `Set(key, value string) error` — write a setting (upsert)
- `GetRetentionDays() int` — returns configured retention, defaulting to 365 if not set
- `SetRetentionDays(days int) error` — validates days (must be 30/90/180/365) and saves

#### API endpoints (in `server.go`)
- `GET /api/settings` — returns all settings as key-value pairs
- `PUT /api/settings/:key` — update a specific setting (body: `{value: "90"}`)
- Both require admin auth (existing middleware)

#### Housekeeping scheduler (`app/housekeeping.go`)
- On service start, read `history.retention_days`. If not set, default to 365.
- Start a background goroutine that runs `PurgeHistoricalBefore` daily at a fixed time (e.g., 03:00 UTC).
- Log each purge run: how many records deleted, or if nothing to purge.
- The scheduler respects context cancellation for clean shutdown.

### Frontend changes (`history/+page.svelte`)
- Add a "Housekeeping" settings section showing current retention period
- Dropdown to select retention (30/90/180/365 days), with a "Save" button
- Show last purge timestamp if available
- Keep existing manual purge form as a "Purge Now" action

### Database
Uses existing `app_settings` table. New row: `history.retention_days` = `"30"` (default). No migration needed — the table already supports arbitrary key-value pairs.

---

## Implementation Order

1. Settings service & API (backend foundation for housekeeping)
2. Housekeeping scheduler (uses settings service)
3. VPN interface auto-discovery (independent backend change)
4. Dashboard auto-refresh (frontend-only)
5. Analytics Summary cards (frontend-only)

This order ensures backend dependencies are ready before frontend features that depend on them, and independent changes can proceed in parallel.