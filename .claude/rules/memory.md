# Project Memory — ocserv_panel

> Canonical index of all design specs and implementation plans.
> AI agents must consult this before starting any feature work to understand what has been designed and planned.

---

## Design Specs (`docs/superpowers/specs/`)

| Date | File | Topic | Status |
|------|------|--------|--------|
| 2026-05-05 | `2026-05-05-config-management-backend-design.md` | Config management REST API (`/api/config/*`) | Approved |
| 2026-05-05 | `2026-05-05-config-management-frontend-design.md` | Config management frontend (`/config` route) | Approved |
| 2026-05-05 | `2026-05-05-config-management-webui-design.md` | Config management WebUI (superseded by the two above) | SUPERSEDED |
| 2026-05-06 | `2026-05-06-users-management-design.md` | Full CRUD for admin + VPN users, tabbed page | Approved |
| 2026-05-07 | `2026-05-07-audit-logging-resilience-design.md` | Zerolog migration, audit tab resilience | Draft |
| 2026-05-07 | `2026-05-07-audit-page-design.md` | Audit page log viewer (full-stack) | Approved |
| 2026-05-07 | `2026-05-07-bulk-operations-design.md` | Bulk delete/disable/enable/lock/unlock for users | Approved |
| 2026-05-07 | `2026-05-07-deployment-installation-design.md` | Build pipeline, installer, TLS, systemd, permissions | Draft |
| 2026-05-07 | `2026-05-07-root-execution-design.md` | Run service as root (no sudoers) | Draft |
| 2026-05-08 | `2026-05-08-sessions-backend-design.md` | Sessions page backend — occtl operations (Phase 1) | Approved |
| 2026-05-08 | `2026-05-08-sessions-fix-design.md` | Sessions quick-fix | Draft |
| 2026-05-08 | `2026-05-08-sessions-frontend-design.md` | Sessions page frontend — tabbed UI (Phase 2) | Approved |
| 2026-05-09 | `2026-05-09-dashboard-improvements-design.md` | Dashboard accuracy, VPN discovery, rate metrics | Draft |
| 2026-05-09 | `2026-05-09-sessions-tab-improvements-design.md` | Sessions refresh timer, click-to-detail, ban badges | Draft |
| 2026-05-10 | `2026-05-10-architecture-simplification-design.md` | Collapse thin services, split packages/files | Approved |
| 2026-05-10 | `2026-05-10-backend-restructure-design.md` | Go backend domain-task layout (`vpn/`, `monitor/`) | Approved |
| 2026-05-10 | `2026-05-10-dashboard-monitoring-improvements-design.md` | Multi-VPN charts, system metadata on cards | Approved |
| 2026-05-10 | `2026-05-10-history-page-improvements-design.md` | Per-user aggregated stats, aligned monitoring charts | Approved |
| 2026-05-10 | `2026-05-10-sessions-status-cards-design.md` | Sessions page status cards (Service/Sessions/Auth/Network) | Approved |
| 2026-05-10 | `2026-05-10-settings-page-improvements-design.md` | Settings page improvements | Approved |
| 2026-05-10 | `2026-05-10-vpn-interface-watcher-design.md` | VPNWatcher replacing timer-based re-discovery | Approved |
| 2026-05-11 | `2026-05-11-history-page-tabs-design.md` | History page full tab overhaul (Monitoring/Sessions/Users/Bans/Settings) | Approved |
| 2026-05-11 | `2026-05-11-history-page-visual-improvements-design.md` | History page visual polish: ChartCard, rich tooltips, Sessions two-section, Bans score table, Users inline chart | Approved |
| 2026-05-12 | `2026-05-12-sessions-history-data-accuracy-design.md` | Sessions All Sessions tab data fix (merge show users), History page Users+Sessions → single Sessions tab | Approved |

---

## Implementation Plans (`docs/superpowers/plans/`)

| Date | File | Goal |
|------|------|------|
| 2026-05-05 | `2026-05-05-config-tab-improvements.md` | Config tab: CodeMirror 6 editor, validation, CIDR auto-fix |
| 2026-05-06 | `2026-05-06-backend-quality-remediation.md` | Fix Critical/Important backend quality findings |
| 2026-05-06 | `2026-05-06-users-management.md` | Users page: two-tab CRUD, RBAC, audit logging |
| 2026-05-07 | `2026-05-07-audit-logging-resilience.md` | rs/zerolog migration, graceful audit tab error handling |
| 2026-05-07 | `2026-05-07-audit-page-backend.md` | Audit backend API, log reader, RBAC, service layer |
| 2026-05-07 | `2026-05-07-audit-page-frontend.md` | Audit frontend: tabs, search/filter/export, auto-refresh |
| 2026-05-07 | `2026-05-07-bulk-operations.md` | Bulk user operations with partial success and audit |
| 2026-05-07 | `2026-05-07-deployment-installation.md` | Zero-dependency deployment, native TLS, installer |
| 2026-05-07 | `2026-05-07-root-execution.md` | Root execution model, eliminate sudoers |
| 2026-05-08 | `2026-05-08-config-management-backend.md` | Close gaps in config management backend vs design spec |
| 2026-05-08 | `2026-05-08-sessions-frontend.md` | Sessions page: status cards, 5-tab layout, detail drawer |
| 2026-05-09 | `2026-05-09-dashboard-improvements.md` | Rate metrics, session collection, VPN discovery, donut chart |
| 2026-05-09 | `2026-05-09-sessions-tab-improvements.md` | Sessions: configurable refresh, click-to-detail, pagination |
| 2026-05-10 | `2026-05-10-architecture-simplification.md` | Collapse services, split packages/files, split api.ts |
| 2026-05-10 | `2026-05-10-backend-restructure.md` | Reorganise backend into `vpn/` + `monitor/` domain layout |
| 2026-05-10 | `2026-05-10-dashboard-monitoring-improvements.md` | Remove unused sections, multi-VPN charts, system metadata |
| 2026-05-10 | `2026-05-10-history-page-improvements.md` | Per-user aggregated stats, aligned monitoring charts |
| 2026-05-10 | `2026-05-10-sessions-status-cards.md` | Sessions status cards from `/api/ocserv/server/status` |
| 2026-05-10 | `2026-05-10-vpn-interface-watcher.md` | VPNWatcher: session-triggered interface add/remove |
| 2026-05-11 | `2026-05-11-history-page-tabs.md` | History page tab overhaul with persistent ban/session history |
| 2026-05-11 | `2026-05-11-history-page-visual-improvements.md` | History page visual polish: ChartCard, rich tooltips, Sessions two-section, Bans score table, Users inline chart | **COMPLETED** |
| 2026-05-12 | `2026-05-12-sessions-history-data-accuracy.md` | Sessions All Sessions enrichment fix (show users join); History Users+Sessions tab merge |

---

## Architecture Summary

- **Backend:** `backend/internal/vpn/` (sessions, users, config, service, health, exec sub-packages), `backend/internal/monitor/` (TSDBStore, collectors, VPNWatcher)
- **Frontend:** SvelteKit + Tailwind CSS v4 + LayerChart. Pages: dashboard, sessions, history, users, config, settings, audit
- **Persistence:** SQLite via `backend/internal/store/`, migrations in `migrations/`
- **Runtime:** systemd service, runs as root (no sudoers), optional HAProxy TLS
- **Key design decisions:** domain-task package layout (not implementation-layer), thin handlers, no compat shims, Fake* test doubles only, all I/O via context

---

## How to use this file

1. Before starting feature work, scan the tables above for existing specs and plans that cover the area.
2. Read the relevant spec before reading code — specs capture design intent the code may not express yet.
3. If a spec is SUPERSEDED, ignore it and use the replacement.
4. Draft specs go in `docs/superpowers/specs/`, plans in `docs/superpowers/plans/` following the existing date-slug naming.
