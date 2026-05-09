# Dashboard Improvements — Design Spec

**Date:** 2026-05-09  
**Status:** Draft  
**Scope:** Backend monitoring collectors, VPN discovery, frontend dashboard page, settings consistency

---

## Problem Statement

The dashboard page has 8 issues affecting data accuracy and usability:

1. **VPN inbound/outbound graphs are empty** — VPN interface discovery misses `vpns*` prefix used by ocserv
2. **VPN metrics stay unavailable after startup** — discovery runs once; if ocserv starts later, VPN metrics remain unavailable
3. **Active sessions / concurrent sessions graphs are empty** — metrics declared as "collecting" but no collector populates them
4. **Network graphs show useless data** — `InterfaceCollector` stores cumulative byte counters, not per-second rates
5. **Active sessions card hardcoded to 0** — `DashboardSummary.ActiveSessionCount` is always 0
6. **Total/Sample sessions cards show confusing info** — labels unclear, no contextual subtitles
7. **No donut chart for active session breakdown** — user requested sessions-by-user donut visualization
8. **Monitoring settings inconsistent** — network/VPN interface names and collector interval only configurable via env vars, not settings page; frontend hardcodes 15s refresh

---

## Architecture

### Backend Changes

```
backend/internal/monitoring/
├── collectors.go            — Convert InterfaceCollector to rate-based (delta bytes/sec)
├── session_collector.go     — NEW: SessionCollector using occtl
├── vpn_discovery.go         — Fix: add "vpns" prefix, handle numbered instances
├── collector_service.go     — Add periodic VPN interface re-discovery

backend/internal/config/
├── config.go               — Add NetworkInterface, VPNInterface, CollectorInterval to MonitoringConfig

backend/internal/app/
├── settings_registry.go    — Add monitoring.network_interface, monitoring.vpn_interface, monitoring.collector_interval
├── settings_config_apply.go — Add apply/read for new monitoring settings
├── dashboard_service.go    — Populate ActiveSessionCount from monitoring store
├── ocserv_service.go       — Add ActiveUserSessions() for donut data

backend/internal/server/
├── server.go               — Add GET /api/dashboard/active-sessions endpoint; include dashboardRefreshInterval in summary response

backend/cmd/ocserv-panel/
├── main.go                 — Read interface names from config (fall back to env, then auto-discovery)
```

### Frontend Changes

```
frontend/src/
├── routes/+page.svelte              — Fix analytics cards, add donut section, use dynamic refresh interval
├── lib/components/ui/DonutChart.svelte — NEW: donut chart using LayerChart Arc/Pie
├── lib/monitoring.ts                — Add rate formatting (bytes/sec), fix formatMetricValue for counts
├── lib/api.ts                       — Add ActiveUserSessions type, getDashboardActiveSessions()
├── lib/settings/descriptions.ts     — Add descriptions for new monitoring settings
```

---

## Detailed Design

### A. Rate-Based Interface Collector (Fix #4)

The current `InterfaceCollector` reads `/sys/class/net/<iface>/statistics/rx_bytes` and stores the raw cumulative counter. This produces a monotonically-increasing line on charts — useless for bandwidth visualization.

**Change:** Make `InterfaceCollector` stateful — store previous sample and compute rate:

```go
type InterfaceCollector struct {
    reader   FileReader
    iface    string
    rxMetric metrics.MetricName
    txMetric metrics.MetricName

    mu       sync.Mutex
    prevRX   uint64
    prevTX   uint64
    prevTime time.Time
    hasPrev  bool
}

func (c *InterfaceCollector) Collect(ctx context.Context, now time.Time) CollectResult {
    rxBytes, txBytes := readCounters()

    if !c.hasPrev {
        c.prevRX, c.prevTX, c.prevTime, c.hasPrev = rxBytes, txBytes, now, true
        return unavailable("rate requires previous sample")
    }

    elapsed := now.Sub(c.prevTime).Seconds()
    if elapsed <= 0 {
        return unavailable("no time elapsed")
    }

    rxRate := float64(rxBytes - c.prevRX) / elapsed  // bytes/sec
    txRate := float64(txBytes - c.prevTX) / elapsed

    c.prevRX, c.prevTX, c.prevTime = rxBytes, txBytes, now

    return available([]{
        {Name: c.rxMetric, Value: rxRate},
        {Name: c.txMetric, Value: txRate},
    })
}
```

**Impact on charts:** Y-axis now shows "bytes/sec" values. The frontend `formatMetricValue` for byte metrics already formats to human-readable sizes — charts will display "1.2 MB/s", "500 KB/s", etc. Add `/s` suffix in `formatAxisValue` for byte rate metrics.

### B. Session Collector (Fix #3, #5)

New `SessionCollector` that calls `occtl -j -n show status` to retrieve active/total session counts.

```go
type SessionCollector struct {
    runner     CommandRunner
    occtlPath  string
    socketPath string
}

func (c *SessionCollector) Collect(ctx context.Context, now time.Time) CollectResult {
    output, err := c.runner.Run(ctx, c.occtlPath, "-j", "-n", "show", "status")
    if err != nil {
        return unavailable("occtl not available: " + err.Error())
    }

    status := parseOcctlStatus(output)
    return available([]{
        {Name: MetricActiveSessions, Value: float64(status.ActiveSessions)},
        {Name: MetricConcurrentSessions, Value: float64(status.ActiveSessions)},
    })
}
```

The `CommandRunner` interface already exists. The collector gracefully returns "unavailable" when occtl is not present or ocserv is not running.

**Dashboard summary integration:** `DashboardService.Summary()` queries the latest `active_sessions` point from the metric store instead of hardcoding 0:

```go
ActiveSessionCount: s.latestSessionCount(ctx),
```

### C. VPN Interface Discovery Fix (Fix #1, #2)

**Discovery fix:** Add `"vpns"` to the `vpnPrefixes` list in `discoverFromSysFS()`. Also, when `discoverFromOcservConfig()` returns a base device name (e.g., `"vpns"`), scan `/sys/class/net/` for interfaces matching that prefix and return the first match (e.g., `vpns0`).

```go
vpnPrefixes := []string{"vpns", "tun", "tap", "wg"}
```

**Re-discovery:** The collector service gains a `vpnDiscoveryTicker` that fires every 60s. If no VPN collector is registered and discovery succeeds, it dynamically adds one:

```go
func (s *CollectorService) maybeDiscoverVPN() {
    if s.hasVPNCollector { return }
    iface, _, err := monitoring.DiscoverVPNInterface(opts...)
    if err != nil || iface == "" { return }
    s.addCollector(NewInterfaceCollector(reader, iface, MetricVPNRX, MetricVPNTX))
    s.hasVPNCollector = true
}
```

This runs within the collector service's `Run()` loop alongside the regular collection tick.

### D. Active User Sessions Endpoint (Fix #7 — Donut Chart)

New backend method and endpoint:

```go
// OcservService
func (s *OcservService) ActiveUserSessions(ctx context.Context, principal auth.Principal) ([]UserSessionCount, error)

type UserSessionCount struct {
    Username     string `json:"username"`
    SessionCount int    `json:"sessionCount"`
}
```

Implementation: calls `occtl -j -n show users`, groups by username, counts sessions per user.

**Endpoint:** `GET /api/dashboard/active-sessions`  
**Response:** `{"users": [{"username": "alice", "sessionCount": 2}, ...], "total": 3}`

### E. Frontend — DonutChart Component

New `DonutChart.svelte` using LayerChart's `Pie`/`Arc`:

```svelte
<script lang="ts">
  import { Chart, Svg, Arc, Pie } from 'layerchart';
  type Segment = { label: string; value: number; color: string };
  let { segments, title }: { segments: Segment[]; title: string } = $props();
</script>

<Chart data={segments}>
  <Svg>
    <Pie value="value">
      <Arc padAngle={0.02} cornerRadius={4} innerRadius={60%} />
    </Pie>
  </Svg>
</Chart>
```

Placed in the "Active sessions" section of the dashboard, replacing the simple CardStats. Shows user segments with distinct colors and legend.

### F. Analytics Summary Cards Fix (Fix #6)

Current state:
- "Total sessions" — shows count from `analytics.totalSessions`
- "Sample sessions" — shows count from `analytics.sampleSessions`

These are confusing because "sample sessions" is internal terminology for seeded test data.

**Fix:**
- "Total sessions" — keep, add subtitle "(incl. X samples)" when sampleSessions > 0
- "Sample sessions" → rename to "Recorded sessions" — shows `totalSessions - sampleSessions` (real sessions)
- Ensure `formatMetricValue` returns `"0"` instead of `"--"` for count values that are explicitly 0

### G. Monitoring Settings Consistency (Fix #8)

**New config fields:**

```go
type MonitoringConfig struct {
    HealthEndpoint           bool   `toml:"health_endpoint"`
    MetricsEndpoint          bool   `toml:"metrics_endpoint"`
    DashboardRefreshInterval int    `toml:"dashboard_refresh_interval"`
    LogFileReadEnabled       bool   `toml:"log_file_read_enabled"`
    NetworkInterface         string `toml:"network_interface"`      // NEW
    VPNInterface             string `toml:"vpn_interface"`          // NEW
    CollectorInterval        int    `toml:"collector_interval"`     // NEW (seconds)
}
```

**New settings registry entries:**

| Key | Label | Type | Default | Notes |
|-----|-------|------|---------|-------|
| `monitoring.network_interface` | Network Interface | text | (empty = auto-discover) | Shows discovered value as placeholder |
| `monitoring.vpn_interface` | VPN Interface | text | (empty = auto-discover) | Shows discovered value as placeholder |
| `monitoring.collector_interval` | Collector Interval (s) | number | 15 | min:5, max:300 |

**Frontend descriptions:**

```ts
'monitoring.network_interface': 'Network interface for bandwidth metrics. Leave empty for auto-discovery.',
'monitoring.vpn_interface': 'VPN interface for VPN bandwidth metrics. Leave empty for auto-discovery (reads ocserv config).',
'monitoring.collector_interval': 'How often (in seconds) the backend collects system and VPN metrics.',
'monitoring.log_file_read_enabled': 'Allow the panel to read log files for display in the dashboard.',
```

**Frontend dynamic refresh interval:**

The dashboard summary response will include `dashboardRefreshInterval` (seconds). The frontend reads this and uses it for `setInterval` instead of hardcoded 15000ms:

```typescript
const refreshMs = (summary.dashboardRefreshInterval ?? 15) * 1000;
refreshInterval = setInterval(() => { ... }, refreshMs);
```

**Config loading priority** (for interface names):
1. Settings page value (from TOML config) — highest
2. Environment variable (`OCSERV_PANEL_NET_IFACE` / `OCSERV_PANEL_VPN_IFACE`)
3. Auto-discovery — lowest

---

## Graceful Degradation

| Condition | Behavior |
|-----------|----------|
| ocserv not running | Session metrics → "unavailable" chips; donut → empty state; active sessions card → "0 (ocserv offline)" |
| No VPN interface found | VPN metrics → "unavailable" chips; re-discovery checks every 60s |
| occtl binary missing | SessionCollector returns unavailable; dashboard falls back to 0 |
| Network interface gone | InterfaceCollector returns unavailable on next tick; chart stops updating |

---

## Testing

| Component | Test Strategy |
|-----------|--------------|
| RateInterfaceCollector | Unit test: provide two consecutive reads, verify bytes/sec calculation |
| SessionCollector | Unit test with FakeCommandRunner: mock occtl JSON output |
| VPN discovery fix | Existing tests + new test for `vpns` prefix |
| Re-discovery | Unit test: verify collector added after simulated interface appearance |
| DonutChart | Visual/snapshot test with mock data |
| Analytics cards | Verify "0" displays instead of "--" for zero counts |
| Settings consistency | Existing settings apply tests + new entries |

---

## Files Modified

### Backend (8 files)
- `backend/internal/monitoring/collectors.go` — rate-based InterfaceCollector
- `backend/internal/monitoring/session_collector.go` — NEW
- `backend/internal/monitoring/vpn_discovery.go` — add vpns prefix
- `backend/internal/monitoring/collector_service.go` — periodic VPN re-discovery
- `backend/internal/config/config.go` — MonitoringConfig fields
- `backend/internal/app/settings_registry.go` — new settings
- `backend/internal/app/settings_config_apply.go` — apply/read new settings
- `backend/internal/app/dashboard_service.go` — populate ActiveSessionCount
- `backend/internal/app/ocserv_service.go` — ActiveUserSessions method
- `backend/internal/server/server.go` — active-sessions endpoint, dashboardRefreshInterval in response
- `backend/cmd/ocserv-panel/main.go` — read interface from config, pass to collectors

### Frontend (5 files)
- `frontend/src/routes/+page.svelte` — fix cards, add donut, dynamic refresh
- `frontend/src/lib/components/ui/DonutChart.svelte` — NEW
- `frontend/src/lib/monitoring.ts` — rate formatting with /s suffix
- `frontend/src/lib/api.ts` — ActiveUserSessions type + fetcher
- `frontend/src/lib/settings/descriptions.ts` — new descriptions

---

## Out of Scope

- `concurrent_sessions_limit` population (requires parsing ocserv config `max-clients`) — deferred
- User-level bandwidth metrics (`user_rx_bytes`, `user_tx_bytes`) — requires per-user occtl parsing, deferred
- Historical rate computation for already-stored cumulative data — new rate data starts fresh
