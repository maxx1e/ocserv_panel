# Sessions Page — Status Cards Redesign

**Date:** 2026-05-10  
**Status:** Approved

## Goal

Replace the current 4-card row on the Sessions page (`SessionStatusCards.svelte`) with four domain-focused cards: Service, Sessions, Auth, and Network. All data comes from the existing `GET /api/ocserv/server/status` endpoint, which calls `occtl -j -n show status` under the hood. No backend changes required.

## Data Source

`occtl -j -n show status` → parsed by `OcctlAdapter.GetServerStatus` → `serverStatusToResponse` → frontend `ServerStatus` type.

All fields used by the new cards are already present in the `ServerStatus` type:

| Card | Fields used |
|------|-------------|
| Service | `status`, `upSince`, `serverPid` |
| Sessions | `activeSessions`, `sessionsHandled`, `avgSessionTimeSeconds` |
| Auth | `authFailures`, `avgAuthTimeSeconds`, `maxAuthTimeSeconds` |
| Network | `rxBytes`, `txBytes`, `minMtu`, `maxMtu`, `lastStatsReset` |

## Layout

Four equal-width cards in a responsive grid:

```
grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4
```

## Card Specifications

### Card 1 — Service

Reuses `CardStats` component.

- **Title:** "Service"
- **Headline value:** uptime duration computed from `upSince` (e.g. `6d 2h`, `3h 15m`, `45m`)
  - Offline state: show `"Offline"` as value
- **Status indicator:** `StatusDot` component in top-right via `statusValue` prop (`"online"` / `"offline"`)
- **Subtitle:** `PID: {serverPid}` when online; omit when offline
- **Icon:** `Server` (lucide-svelte)

### Card 2 — Sessions

- **Title:** "Sessions"
- **Headline value:** `activeSessions` count
- **Subtitle:** `{sessionsHandled} handled · avg {formatDuration(avgSessionTimeSeconds)}`
- **Icon:** `Users` (lucide-svelte)

### Card 3 — Auth

- **Title:** "Auth"
- **Headline value:** `authFailures` count
- **Subtitle:** `Avg: {avgAuthTimeSeconds}s · Max: {maxAuthTimeSeconds}s`
- **Icon:** `ShieldAlert` (lucide-svelte)

### Card 4 — Network

- **Title:** "Network"
- **Headline value:** `{formatBytes(rxBytes)} / {formatBytes(txBytes)}`
- **Subtitle:** `MTU {minMtu}–{maxMtu} · reset {relativeDate(lastStatsReset)}`
- **Icon:** `ArrowUpDown` (lucide-svelte)

## Helper Functions (frontend only)

Add to `SessionStatusCards.svelte` script block:

### `formatUptime(upSince: string | undefined): string`
Already exists — returns `"—"` when missing, otherwise `Xd Yh`, `Xh Ym`, or `Xm`.

### `formatDuration(seconds: number | undefined): string`
Converts seconds to human-readable: `"—"` if falsy, `Xh Ym` if ≥ 3600, `Xm` if ≥ 60, `Xs` otherwise.

### `relativeDate(iso: string | undefined): string`
Returns relative label from ISO timestamp: `"today"`, `"Xd ago"`, `"Xh ago"`, `"Xm ago"`.

`formatBytes` is already imported from `$lib/monitoring`.

## Component Changes

**File:** `frontend/src/routes/sessions/SessionStatusCards.svelte`

- Remove current 4-card layout
- Add three helper functions (`formatDuration`, `relativeDate`)
- Render new 4-card grid using `CardStats` for each domain card

No other files change. `CardStats.svelte` is reused as-is.

## Error Handling

- Loading state: all card values show `"..."` 
- Error state: existing amber alert banner is preserved above the cards
- Missing/zero values: show `"—"` for times, `"0"` for counts

## Testing

Manual verification:
- Cards render correctly with live `occtl` data
- Cards degrade gracefully when `status` is null (loading) or when server is offline
- Uptime, session time, and relative date helpers produce correct output for edge cases (< 1 min, > 1 day)
