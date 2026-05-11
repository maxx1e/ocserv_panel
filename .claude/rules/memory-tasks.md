# Project Memory — ocserv_panel

**Date:** 2026-05-04
**Branch:** master
**Sync status:** In sync with origin/master (working tree clean)

---

## Project Overview

`ocserv_panel` is a local-first Web UI admin panel for `ocserv` (OpenConnect VPN server). It runs on the same host as ocserv and provides a web dashboard for managing users, sessions, configuration, and monitoring.

### Architecture

- **Backend:** Go (HTTP API, RBAC, audit logging, system integration)
- **Frontend:** SvelteKit (admin dashboard, Tailwind CSS, LayerChart)
- **Persistence:** SQLite (users, sessions, audit events, settings)
- **Runtime:** systemd service management
- **TLS:** HAProxy (optional)

---

## Completed Work

### 1. UI Design Overhaul (Completed)

A complete frontend redesign using Tailwind CSS v4, lucide-svelte icons, and LayerChart for data visualization.

**Commits:**
- `aced981` — Install Tailwind CSS v4, lucide-svelte, layerchart
- `bca49dc` — Rebuild layout shell with slate sidebar and top navbar
- `9c8010a` — Rebuild dashboard with CardStats, CardChart, CardTable, WindowSelector
- `f9bc404` — Fix icons and component placements
- `e30bdaa` — Rebuild login and setup pages
- `dfc4fdc` — Rebuild settings page with Tailwind
- `562a9f6` — Rebuild history page with Tailwind
- `00490dd` — Replace PlaceholderPage with EmptyState
- `4f17a55` — Remove old components and CSS
- `f8914ca` — Fix dashboard graphs (metric states, auto-detect network interface)

**Key Components:**
- `CardStats` — Stat cards with icons and trend indicators
- `CardChart` — Chart wrapper with title and latest value
- `CardTable` — Table wrapper with pagination and search
- `EmptyState` — Placeholder for empty data states
- `WindowSelector` — Time range selector for metrics

### 2. Settings Tab UI Redesign (Completed)

Refactored the settings page with a two-column layout, type-specific field components, and improved UX.

**Commits:**
- `88c63c3` — Design spec
- `b12bba1` — Implementation plan
- `17db1b5` — Frontend-only setting descriptions map
- `f988229` — SettingTooltip component
- `34f3cb2` — SettingRow two-column layout
- `f608a49` — SettingSelect component
- `007159c` — SettingText component
- `e18b160` — SettingNumber component
- `2608148` — SettingPassword component
- `8e37190` — SettingMultiCheckbox component
- `ba4a295` — SettingReadonly component
- `463632e` — SettingButton component
- `a7fda5b` — SettingToggle with SettingRow
- `4efeebe` — SettingField as thin type dispatcher
- `76df95e` — CardSettings row layout
- `c50b6fd` — ARIA role for SettingTooltip

### 3. Settings API Backend (Completed)

Granular settings management with RBAC, sections, and restart handling.

**Commits:**
- `abd2309` — Granular settings permissions
- `af9311d` — SettingsService with GetAllSections and UpdateSetting
- `e605f80` — Settings API handlers
- `0e3adf6` — RBAC checks, Value field, restart tests
- `51614b9` — Frontend API types and functions
- `2f2e5b0` — Settings page with section navigation
- `5d67859` — Wire TOML config loading in main.go

### 4. ocserv Integration — Phase 1 (In Progress)

Replacing the `NoopManager` stub and hardcoded "not_configured" dashboard status with real ocserv integration.

**Design Spec:** `docs/superpowers/specs/2026-05-03-ocserv-integration-design.md`
**Implementation Plan:** `docs/superpowers/plans/2026-05-04-ocserv-integration.md`

**Completed Tasks:**

#### Task 1: Config & Binary Path Extensions ✅
- Added `OcservBinPath`, `OcctlBinPath`, `OcpasswdBinPath` to `InstallationConfig`
- Renamed `OcpasswdPath` → `OcpasswdFilePath`
- Added `SystemdUnitName` to `ServiceConfig`
- Updated settings registry with new installation and service settings
- Backward-compatible TOML loading for old `ocpasswd_path` key
- **Commit:** `a2756a7`

#### Task 2: Command Runner Interface ✅
- `CommandRunner` interface for testable command execution
- `OSCommandRunner` — real implementation via `exec.CommandContext`
- `FakeCommandRunner` — test double with pre-configured results and call recording
- **Commit:** `8dc3c1d`

**Pending Tasks (3-13):**

| Task | Description | Files |
|------|-------------|-------|
| 3 | RBAC Permissions for ocserv | `backend/internal/rbac/rbac.go` |
| 4 | Expand Manager Interface + Rename NoopManager | `backend/internal/ocserv/client.go` |
| 5 | ocserv Readiness Checker | `backend/internal/ocserv/readiness.go` |
| 6 | Systemd Service Manager | `backend/internal/ocserv/service*.go` |
| 7 | OcctlAdapter for Session Management | `backend/internal/ocserv/occtl_adapter.go` |
| 8 | OcpasswdAdapter for User Management | `backend/internal/ocserv/passwd*.go` |
| 9 | Config Validator | `backend/internal/ocserv/config_validator.go` |
| 10 | OcservService + Dashboard Integration | `backend/internal/app/ocserv_service.go`, `dashboard_service.go` |
| 11 | Frontend API Types and Functions | `frontend/src/lib/api.ts`, `+page.svelte` |
| 12 | Sudoers Rules | `deploy/sudoers.d/ocserv-panel` |
| 13 | Full Integration Verification | All tests + build |

### 5. Frontend Chart Improvements (Completed)

**Commit:** `94a1217` (part of memory-tasks.md commit)

**Changes:**
- New `MetricChart.svelte` component wrapping LayerChart with metric-aware colors, gradients, and tooltips
- Updated `monitoring.ts` with chart color definitions (CPU, memory, network, VPN, user, session metrics)
- Updated `app.css` with LayerChart grid/rule/tick styles
- Dashboard and history pages now use `MetricChart` instead of direct `LineChart`
- CardChart now shows placeholder for unavailable metrics instead of hiding charts

---

## Current State

### Server Installation (from validation)
- **ocserv binary:** `/usr/local/sbin/ocserv` (1.4.2)
- **occtl binary:** `/usr/local/bin/occtl`
- **ocpasswd binary:** `/usr/local/bin/ocpasswd`
- **ocserv config:** `/etc/ocserv/ocserv.conf`
- **ocserv NOT running:** No systemd unit, no socket, no VPN interface
- **Config issues:** Relative cert paths (`../tests/certs/`), missing password file

### Backend Tests
- All existing tests pass (12/12 packages)
- New `runner_test.go` passes (8/8 tests)

### Frontend
- TypeScript/Svelte compilation clean
- Build succeeds

---

## Next Steps

1. **Complete ocserv integration tasks 3-13** using subagent-driven development
2. **Address production config issues** (relative cert paths in ocserv.conf)
3. **Set up systemd service** for ocserv
4. **Configure sudoers rules** for panel service account

---

## Design Documents

| Document | Path | Status |
|----------|------|--------|
| ocserv Integration Design | `docs/superpowers/specs/2026-05-03-ocserv-integration-design.md` | Approved |
| ocserv Integration Plan | `docs/superpowers/plans/2026-05-04-ocserv-integration.md` | Approved |
| Settings UI Design | `docs/superpowers/specs/2026-04-XX-settings-ui-design.md` | Completed |
| Settings UI Plan | `docs/superpowers/plans/2026-04-XX-settings-ui-plan.md` | Completed |
| UI Overhaul Design | `docs/superpowers/specs/2026-04-XX-ui-overhaul-design.md` | Completed |
| UI Overhaul Plan | `docs/superpowers/plans/2026-04-XX-ui-overhaul-plan.md` | Completed |

---

*Generated by AI coding agent on 2026-05-04*
