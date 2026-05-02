# Dashboard History Monitoring Design

## Objective

Add a dashboard/history implementation slice focused on representing monitoring and historical data in the Web UI. This slice introduces local embedded time-series storage for monitoring metrics, host metric collection, authenticated monitoring APIs, and reusable Svelte chart components for the Dashboard and History tabs.

The slice remains single-node and local-first. It does not add an external Prometheus server, Grafana, hosted dependency, queue, cloud service, or live `ocserv` management integration.

## Decisions

- Use embedded Prometheus TSDB as a local Go dependency and file-backed time-series store.
- Treat this TSDB dependency as an approved exception to the default no-new-monitoring-stack rule.
- Do not run Prometheus as a daemon or external service.
- Keep SQLite as the source of truth for admin/auth/audit/history session metadata.
- Store only numeric time-series samples and safe labels in TSDB.
- Add a narrow backend `MetricsStore` abstraction so API/service/UI contracts do not depend on Prometheus TSDB internals.
- Use host collectors for CPU, memory, and interface counters in this slice.
- Include the full target metric contract now, but return explicit unavailable states for ocserv-only metrics that cannot be collected without later ocserv ingestion.
- Use shared frontend chart components across Dashboard and History.
- Avoid adding a frontend charting dependency in the first implementation unless the implementation proves lightweight SVG charts are insufficient.

## Explicit Non-Goals

- No Prometheus server, Grafana, InfluxDB, VictoriaMetrics service, or other external monitoring daemon.
- No live `ocserv`, `occtl`, `ocpasswd`, management socket, log ingestion, `systemctl`, or `sudo` calls.
- No live session termination, user lifecycle changes, certificate lifecycle changes, or configuration mutation.
- No fake live values for ocserv-only monitoring metrics.
- No storage of secrets, passwords, raw session tokens, private keys, command output, or credentials in TSDB.
- No frontend access to local files, sockets, commands, certificates, or secrets.

## Backend Architecture

The Go backend owns the monitoring boundary. It will add:

- A `MetricsStore` interface for appending samples and querying range data.
- A Prometheus TSDB-backed implementation stored in a local configurable data directory.
- A collector service that samples host metrics on a fixed interval.
- A monitoring service that validates metric/window/step inputs, queries TSDB ranges, downsamples responses, and reports unavailable metric states.
- Authenticated HTTP handlers for monitoring catalog, overview, and series queries.

The TSDB data directory is configured with `OCSERV_PANEL_TSDB_PATH` and defaults to `./data/metrics` relative to the backend working directory. The default retention window is 30 days. The backend must handle TSDB open failures, corrupt/missing data directories, and write/query failures with operational errors that do not expose sensitive details.

## Metric Contract

The API and UI include the full target metric set:

- `host_cpu_utilization`: host CPU utilization.
- `host_memory_utilization`: host memory utilization.
- `net_interface_rx_bytes`: inbound bytes for the configured network interface.
- `net_interface_tx_bytes`: outbound bytes for the configured network interface.
- `vpn_interface_rx_bytes`: inbound bytes for the configured VPN interface.
- `vpn_interface_tx_bytes`: outbound bytes for the configured VPN interface.
- `user_rx_bytes`: per-user inbound bytes.
- `user_tx_bytes`: per-user outbound bytes.
- `active_sessions`: current active VPN sessions.
- `connection_rate`: new sessions per second.
- `concurrent_sessions`: current concurrent sessions.
- `concurrent_sessions_limit`: configured concurrent-session limit.

The first implementation collects real host CPU, memory, and available interface counters. VPN interface metrics collect real counter samples only when the configured interface exists. Per-user bandwidth, active sessions, connection rate, and concurrent-session limit series return explicit unavailable states until a later ocserv ingestion slice provides real data.

## Host Collectors

Collectors read local kernel-provided files rather than shelling out. The collector interval is 15 seconds.

- CPU utilization from `/proc/stat` deltas.
- Memory utilization from `/proc/meminfo`.
- Interface RX/TX counters from `/sys/class/net/<iface>/statistics/rx_bytes` and `tx_bytes`.

The primary network interface is configured with `OCSERV_PANEL_NET_IFACE`. If it is unset, network-interface metrics are unavailable. The VPN interface is configured with `OCSERV_PANEL_VPN_IFACE`. If it is unset or absent from `/sys/class/net`, VPN-interface metrics are unavailable.

Collectors must be testable with fixture readers so normal tests do not depend on the developer host. Missing files, missing interfaces, malformed values, and permission errors mark the affected metric unavailable and avoid crashing the backend.

## API Design

Add authenticated read APIs separate from the existing summary/history endpoints:

- `GET /api/monitoring/catalog`
- `GET /api/monitoring/overview?window=1h`
- `GET /api/monitoring/series?metric=host_cpu_utilization&window=1h&step=60s`

`catalog` returns metric metadata, labels, current collection state, and unavailability reasons where applicable.

`overview` returns a compact dashboard payload for a selected short window, including latest values and small trend series for key metrics.

`series` returns one validated metric range, sampled or downsampled to the requested step.

Allowed windows are `15m`, `1h`, `6h`, `24h`, `7d`, and `30d`. Allowed step values are `15s`, `60s`, `5m`, and `1h`. The API rejects combinations where `window / step` would return more than 1,500 points.

All monitoring endpoints require an authenticated admin with dashboard read permission. Invalid metric names, invalid windows, invalid steps, and unsupported labels return consistent safe JSON errors.

## Data Flow

1. Backend startup opens SQLite as before and opens the embedded TSDB data directory.
2. The monitoring collector samples host metrics on a fixed interval.
3. The collector appends labeled samples to TSDB through `MetricsStore`.
4. The monitoring service reads catalog state and TSDB ranges through `MetricsStore`.
5. Dashboard uses `overview` and selected short-window `series` data.
6. History uses `catalog` and `series` with longer windows and range controls.

The collector must not block authentication, setup, or existing dashboard/history APIs from operating. If TSDB initialization fails, monitoring endpoints report monitoring unavailable while the rest of the panel remains usable.

## Frontend UX

Use the balanced shared-components direction:

- Dashboard shows short-window operational trends for CPU, memory, network RX/TX, VPN RX/TX when available, active sessions, and concurrent sessions vs limit.
- History shows longer-range exploration with `24h`, `7d`, and `30d` windows using the same chart components.
- Existing historical session table and owner-only purge remain in the History tab.
- ocserv-only metrics appear as intentional "not collecting yet" panels, not zero-value charts.
- Existing sample/demo labels continue to apply only to seeded historical sessions, not monitoring metrics.

Add reusable Svelte components for:

- Metric summary cards.
- Time-series line/area chart.
- Empty or unavailable metric panel.
- Window selector.

Charts must be readable on desktop and mobile. The first implementation uses lightweight SVG in Svelte to avoid adding another frontend dependency.

## RBAC, Audit, And Security

Monitoring read APIs require authenticated dashboard-read permission. Unknown or missing roles deny by default through existing RBAC behavior.

Monitoring reads do not need per-request audit records at the same level as destructive actions, but startup/collector failures must be visible through operational logs or safe status fields. Any future monitoring mutation or retention-changing endpoint must add owner-only authorization and audit coverage.

TSDB labels must remain safe. Allowed labels include metric names, interface names, directions, and usernames only when real ocserv/user ingestion is later implemented. Labels must not include passwords, tokens, private keys, raw request data, command output, or credentials.

## Operations

Document these assumptions:

- Embedded TSDB data path and retention/default storage behavior.
- Collector interval.
- Network interface configuration for host traffic.
- VPN interface configuration and behavior when the interface is absent.
- Monitoring remains local-only and does not require Prometheus or Grafana.

The app continues to run without root. Interface counter reads under `/sys/class/net` and `/proc` are expected to be readable on typical Linux hosts, but permission or namespace limitations must be handled gracefully.

## Testing And Verification

Backend tests:

- Host CPU collector parses `/proc/stat` fixtures and computes utilization from deltas.
- Memory collector parses `/proc/meminfo` fixtures and computes utilization.
- Interface collector parses RX/TX fixtures and reports missing interfaces safely.
- TSDB wrapper appends and queries samples using a temporary data directory.
- Monitoring service validates windows, steps, metric names, and unavailable ocserv-only states.
- Monitoring API endpoints require authentication and reject invalid query inputs.

Frontend checks:

- Dashboard renders collecting metrics, unavailable metrics, and empty metric states.
- History renders longer range controls and keeps existing historical session/purge behavior.
- Shared chart components handle no data, single-point data, and multi-point data.

Repository verification:

- `go test ./...` from `backend/`.
- `npm run check` from `frontend/`.
- `npm run build` from `frontend/`.
- Safety search confirms no live `ocserv`, `occtl`, `ocpasswd`, `systemctl`, `sudo`, shell command collectors, `os/exec`, `child_process`, or frontend filesystem access was introduced.

## Acceptance Criteria

- Backend uses embedded Prometheus TSDB through an internal abstraction, not an external monitoring service.
- Host CPU, memory, and configured network interface metrics are sampled and queryable.
- VPN interface metrics are sampled when the configured interface exists and otherwise marked unavailable.
- ocserv-only metric slots exist in API/UI but show unavailable states instead of fake live values.
- Dashboard shows short-window monitoring trends through shared components.
- History shows longer-window monitoring trends while preserving existing historical session analytics and purge UI.
- Monitoring APIs require authentication and validate metric/window/step inputs.
- TSDB stores numeric samples and safe labels only.
- Operational assumptions and TSDB data path behavior are documented.
- Backend tests and frontend verification pass before completion claims.
