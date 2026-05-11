# Sessions Fix Design Spec

**Date:** 2026-05-08
**Status:** Draft

## Problem Statement

1. The sessions page shows a dead-end "ocserv is not available" error when ocserv isn't running or the socket is unreachable. Users have no diagnostic information.
2. The panel config has wrong binary paths (`/usr/bin/occtl` should be `/usr/bin/occtl` — actually correct; `/usr/sbin/ocserv` should match `ocserv_bin_path`).
3. The "All Sessions" tab doesn't show traffic (RX/TX) data per session despite the data being available in the API response.
4. There's no per-user aggregate traffic view (total bytes across all of a user's sessions).

## Design

### 1. Fix Configuration Paths

Update `config.toml` to match actual binary locations:
- `ocserv_bin_path` = `/usr/sbin/ocserv` (currently `/usr/sbin/ocserv` — already correct)
- `occtl_bin_path` = `/usr/bin/occtl` (currently `/usr/bin/occtl` — already correct)
- `ocpasswd_bin_path` = `/usr/bin/ocpasswd` (currently `/usr/bin/ocpasswd` — already correct)
- `ocserv_socket_path` = `/var/run/ocserv-socket` (matches ocserv.conf `socket-file`)

The socket path is correct — the real issue is that ocserv isn't running. The fix is the improved UX below.

### 2. Improved "Not Available" Error State

Instead of a single dead-end `EmptyState` saying "ocserv is not available", show a **diagnostic card** that:

1. Calls the existing `/api/ocserv/readiness` endpoint which checks:
   - `binaryPresent` — ocserv binary exists
   - `binaryVersion` — ocserv version string
   - `configPresent` — config file exists
   - `configValid` — config passes validation
   - `serviceRunning` — systemd service is active
   - `socketPresent` — management socket exists
   - `occtlPresent` — occtl binary exists
   - `ocpasswdPresent` — ocpasswd binary exists

2. Renders a checklist showing pass/fail for each item with:
   - Green check for passing items
   - Red X for failing items with a hint about what to do
   
3. Provides action suggestions:
   - "Start ocserv service" button if serviceRunning is false (only for owners)
   - "Check config file" hint if configValid is false
   - "Socket not found" with path shown if socketPresent is false

**Flow change:** When `initPage()` gets a 503 from `getServerStatus()`, instead of immediately setting `notConfigured = true`, it calls `getOcservReadiness()` to get the diagnostic breakdown and renders the diagnostic panel.

### 3. Traffic Data in All Sessions Tab

The API already returns `rxBytes` and `txBytes` in the `OcservSession` response. The `AllSessionsTab` simply doesn't render these columns. Add them:

- Add "RX / TX" column after "Client IP" column
- Use the existing `formatBytes()` utility from `$lib/monitoring`
- Format as: `{formatBytes(session.rxBytes)} / {formatBytes(session.txBytes)}`

### 4. Per-User Aggregate Traffic

In the `ActiveSessionsTab` detail drawer (`SessionDetailDrawer`), add a **traffic summary section** when `userConnection` data is available:

- Show RX bytes, TX bytes, average RX bandwidth, average TX bandwidth
- These fields already exist in `UserConnection` type: `rxBytes`, `txBytes`, `avgRxBandwidth`, `avgTxBandwidth`
- Format bytes with `formatBytes()` and show bandwidth strings as-is (they come pre-formatted from occtl like "820 bytes/s", "68.1 kB/s")

Additionally, in the Active Sessions table, since a user might have multiple sessions, we can group by username in a small summary card within the drawer showing total traffic across all their connections.

## Files to Modify

### Backend
- `backend/config.toml` — Verify paths are correct (no changes needed — already correct)

### Frontend
- `frontend/src/routes/sessions/+page.svelte` — Change `notConfigured` state to load readiness diagnostic
- `frontend/src/routes/sessions/ReadinessDiagnostic.svelte` — **NEW** component showing readiness checklist
- `frontend/src/routes/sessions/AllSessionsTab.svelte` — Add RX/TX column
- `frontend/src/routes/sessions/SessionDetailDrawer.svelte` — Add traffic summary section

### No backend changes needed
- The readiness endpoint already exists
- Session/user data already includes traffic bytes
- Error handling and audit logging work correctly as-is

## Out of Scope

- Starting/stopping ocserv from the sessions page (can link to a future service management feature)
- Historical per-user bandwidth tracking (would require tsdb integration)
- Modifying the audit_log table for read-only operation failures (user confirmed log file is sufficient)
