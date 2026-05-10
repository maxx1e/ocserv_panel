# VPN Interface Watcher — Design Spec

**Date:** 2026-05-10  
**Status:** Approved  
**Scope:** `backend/internal/monitor`, `backend/cmd/ocserv-panel`

---

## Problem

VPN bandwidth metrics (`vpn_interface_rx_bytes`, `vpn_interface_tx_bytes`) never appear in the dashboard even when ocserv has active sessions.

### Root Causes

**1. Discovery misses the interface at startup.**  
The ocserv config has `device = vpns`. The kernel only creates `vpns0`, `vpns1`, etc. in `/sys/class/net` when users are actively connected. When the panel starts with no active sessions, `DiscoverAllVPNInterfaces()` reads the config, finds the base name `vpns`, but `resolveAllInterfaceNames` finds nothing in sysfs — so no `InterfaceCollector` is created.

**2. The 15-second re-discovery ticker is not session-aware.**  
`CollectorService.maybeDiscoverAllVPN` runs on a fixed timer regardless of session state. Between the time a user connects and the next timer tick, no VPN metrics are collected. More importantly, if no users are connected at tick time, discovery silently returns empty and doesn't retry until the next tick.

**3. Metric catalog state is not updated when dynamic discovery fires.**  
When `maybeDiscoverAllVPN` adds a new collector, it does not call `metricCatalog.SetState(...)`. The catalog state for VPN metrics stays at its initial value, causing the dashboard to show them as unavailable even when collection has started.

**4. Stale collectors persist after VPN interfaces disappear.**  
When all users disconnect, `vpns*` interfaces vanish from sysfs. The existing `InterfaceCollector` keeps running and silently returns unavailable on every tick. The catalog is never updated and the collector is never cleaned up.

---

## Solution

Replace the timer-based re-discovery inside `CollectorService` with a dedicated `VPNWatcher` that:

- Subscribes to session-change notifications from the `SessionCollector`
- Diffs the current set of active VPN interfaces against what sysfs reports
- Adds `InterfaceCollector`s for newly discovered interfaces and updates the metric catalog
- Removes stale collectors when interfaces disappear and flips the catalog back to unavailable
- Runs a 15-second fallback poll ticker as a safety net against missed notifications

---

## Architecture

```
SessionCollector.Collect() → detects count change → calls SessionChangeNotifier.NotifySessionChange()
                                                              ↓
                                                       VPNWatcher (channel recv)
                                                              ↓
                                          calls DiscoverAllVPNInterfaces()
                                                              ↓
                                    diff: new ifaces → CollectorService.AddCollector()
                                                              → Catalog.SetState(Collecting)
                                    diff: gone ifaces → CollectorService.RemoveCollector()
                                                              → Catalog.SetState(Unavailable)
```

---

## New Type: `monitor.VPNWatcher`

**File:** `backend/internal/monitor/vpn_watcher.go`

```go
type VPNWatcher struct {
    service    *CollectorService
    catalog    *Catalog
    discoverer VPNMultiDiscovererFunc
    reader     FileReader
    rxMetric   MetricName
    txMetric   MetricName
    logger     Logger
    notify     chan struct{}   // buffered(1), session-change signal
    mu         sync.Mutex
    tracked    map[string]bool // interfaces currently monitored
}
```

**Constructor:** `NewVPNWatcher(service, catalog, discoverer, reader, rxMetric, txMetric, logger)`

**Methods:**

- `NotifySessionChange()` — non-blocking send to `notify` channel; satisfies `SessionChangeNotifier`
- `Run(ctx context.Context)` — goroutine loop; exits when ctx is cancelled
- `syncInterfaces()` — private; diffs and applies add/remove

**Loop behavior:**

```
ticker := time.NewTicker(15s)
for {
    select {
    case <-ctx.Done(): return
    case <-notify:    syncInterfaces()
    case <-ticker.C:  syncInterfaces()
    }
}
```

**`syncInterfaces()` logic:**

1. Call `discoverer()` to get current sysfs interfaces.
2. Lock `tracked` map.
3. Add: for each discovered interface not in `tracked`, call `service.AddCollector(iface)` and `catalog.SetState(rxMetric, StateCollecting, "")`.
4. Remove: for each interface in `tracked` not in discovered, call `service.RemoveCollector(iface)` and `catalog.SetState(rxMetric, StateUnavailable, "no active VPN interface")`.
5. Update `tracked` to match discovered.

---

## Changes to `monitor.CollectorService`

**Add:**

```go
// AddCollector appends a new collector to the running service.
func (s *CollectorService) AddCollector(c Collector)

// RemoveCollectorByIface removes all InterfaceCollectors for the given interface name.
func (s *CollectorService) RemoveCollectorByIface(iface string)
```

**Remove** (replaced by `VPNWatcher`):

- `SetVPNDiscoverer`
- `maybeDiscoverAllVPN`
- `HasVPNCollector`
- `MarkVPNPresent`
- `discoveredVPN` field
- `vpnDiscoverer`, `vpnReader`, `vpnRXMetric`, `vpnTXMetric` fields
- VPN ticker branch in `Run`

---

## Changes to `monitor.SessionCollector`

Add an optional `SessionChangeNotifier` dependency:

```go
type SessionChangeNotifier interface {
    NotifySessionChange()
}

type SessionCollector struct {
    provider  SessionStatusProvider
    notifier  SessionChangeNotifier // optional, may be nil
    lastCount int
}

func NewSessionCollectorWithNotifier(provider SessionStatusProvider, notifier SessionChangeNotifier) *SessionCollector
```

After each successful `Collect`, if `status.ActiveSessions != lastCount`, call `notifier.NotifySessionChange()` and update `lastCount`. `NewSessionCollector(provider)` remains unchanged (sets notifier to nil).

---

## Changes to `cmd/ocserv-panel/main.go`

Replace the `vpnConfigured` / `SetVPNDiscoverer` / `MarkVPNPresent` block with:

```go
vpnWatcher := monitor.NewVPNWatcher(
    collectorService,
    metricCatalog,
    func() ([]string, error) { return monitor.DiscoverAllVPNInterfaces() },
    monitor.OSReader{},
    monitor.MetricVPNInterfaceRXBytes,
    monitor.MetricVPNInterfaceTXBytes,
    zerologAdapter{},
)
sessionCollector := monitor.NewSessionCollectorWithNotifier(sessionManager, vpnWatcher)
go vpnWatcher.Run(ctx)
go collectorService.Run(ctx)
```

Remove: `vpnConfigured` check, `SetVPNDiscoverer`, `MarkVPNPresent`, the else-branch VPN iface collector in `configuredCollectors`.

---

## Testing Plan

### `monitor/vpn_watcher_test.go`

| Test | Scenario |
|---|---|
| `TestVPNWatcher_AddOnNotification` | Notification triggers discovery; interface found → collector added, catalog set to Collecting |
| `TestVPNWatcher_RemoveWhenGone` | Second notification; interface gone → collector removed, catalog set to Unavailable |
| `TestVPNWatcher_NoDuplicateOnRepeatNotification` | Same interface discovered twice → only one collector added |
| `TestVPNWatcher_MultiInterfaceLifecycle` | vpns0 and vpns1 appear, then vpns1 disappears → correct add/remove |
| `TestVPNWatcher_FallbackTickerTriggersSync` | No notification; watcher ticker fires → discovery still runs |
| `TestVPNWatcher_ContextCancelStopsLoop` | ctx cancelled → Run returns cleanly |
| `TestVPNWatcher_DiscoveryErrorLogsWarn` | Discoverer returns error → warn logged, no panic |

### `monitor/service_test.go`

Update tests that reference removed methods (`SetVPNDiscoverer`, `maybeDiscoverAllVPN`, `HasVPNCollector`, `MarkVPNPresent`). Add tests for `AddCollector` and `RemoveCollectorByIface`.

### `monitor/session_collector_test.go`

Add tests for `NewSessionCollectorWithNotifier`: notifier called when count changes, not called when count unchanged, nil notifier is safe.

---

## Catalog State Transitions

| Event | `vpn_interface_rx_bytes` state | `vpn_interface_tx_bytes` state |
|---|---|---|
| Startup (no VPN) | `unavailable` | `unavailable` |
| VPN interface discovered | `collecting` | `collecting` |
| VPN interface removed | `unavailable` | `unavailable` |

The initial state in `DefaultCatalog()` for VPN metrics is already `StateCollecting`, which is incorrect when no interface exists. This spec changes the startup default to `StateUnavailable` with reason `"waiting for VPN interface"`.

---

## Minor Addition to `InterfaceCollector`

Add a `Iface() string` getter so `CollectorService.RemoveCollectorByIface` can identify which collectors belong to a given interface without exporting the `iface` field or using unsafe type assertions:

```go
func (c *InterfaceCollector) Iface() string { return c.iface }
```

`RemoveCollectorByIface` iterates `s.collectors`, type-asserts each to `*InterfaceCollector`, calls `Iface()`, and removes matches.

---

## What Does Not Change

- `DiscoverAllVPNInterfaces` and its functional options — no changes
- `InterfaceCollector` — gains `Iface()` getter only, no behavioral change
- `VPNMultiDiscovererFunc` type — reused in `VPNWatcher`
- All existing monitor metric names, `Sample`, `Series`, `Store` types
- The `SessionStatusProvider` interface

---

## Non-Goals

- Per-user bandwidth metrics (separate feature)
- Watching for new VPN interface prefixes beyond the existing `vpns`, `tun`, `tap`, `wg` set
- Persisting VPN interface state across panel restarts
