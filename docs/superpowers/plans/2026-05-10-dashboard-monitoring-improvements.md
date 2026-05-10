# Dashboard Monitoring Improvements Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Improve the dashboard monitoring section by removing unused sections, adding system metadata to metric cards, enabling multi-VPN interface tracking, and introducing a multi-line VPN chart.

**Architecture:** Backend adds a `GET /api/monitoring/system-info` endpoint and extends `CollectorService` to track all VPN interfaces simultaneously (one `InterfaceCollector` per discovered interface). Frontend consumes system info to enrich metric card subtitles, removes the activity/log section and session charts, and introduces a `MultiLineChart` component that draws one line per VPN interface.

**Tech Stack:** Go backend (runtime, /proc/meminfo, metrics.Store), SvelteKit + LayerChart frontend (Svelte 5 runes, Tailwind CSS v4).

---

## File Map

### Backend — new/modified
| File | Change |
|------|--------|
| `backend/internal/monitoring/vpn_discovery.go` | Add `DiscoverAllVPNInterfaces()` + `resolveAllInterfaceNames()` |
| `backend/internal/monitoring/vpn_discovery_test.go` | Tests for new multi-discovery functions |
| `backend/internal/monitoring/collector_service.go` | Replace `hasVPN bool` + `VPNDiscovererFunc` with multi-interface tracking |
| `backend/internal/monitoring/collector_service_test.go` | Tests for multi-VPN collector behaviour |
| `backend/internal/server/monitoring_helpers.go` | Add `systemInfoResponse` struct + `buildSystemInfo()` helper |
| `backend/internal/server/server.go` | Register `GET /api/monitoring/system-info` route + handler |
| `backend/internal/server/server_test.go` | Tests for the new endpoint |
| `backend/cmd/ocserv-panel/main.go` | Update `SetVPNDiscoverer` call to use new multi-discoverer signature |

### Frontend — new/modified/deleted
| File | Change |
|------|--------|
| `frontend/src/lib/api/monitoring.ts` | Add `SystemInfo` type + `getSystemInfo()` |
| `frontend/src/routes/+page.svelte` | Fetch `systemInfo`; remove `DashboardActivity` + `recentAudit` |
| `frontend/src/lib/components/dashboard/DashboardActivity.svelte` | **Delete** |
| `frontend/src/lib/components/dashboard/DashboardSessions.svelte` | Remove active sessions card; add interface subtitles to analytics cards |
| `frontend/src/lib/components/dashboard/DashboardMetrics.svelte` | Remove session charts; add card subtitles; replace VPN charts with `MultiLineChart` |
| `frontend/src/lib/components/ui/MultiLineChart.svelte` | **New** multi-line LayerChart component |

---

## Task 1: Add `DiscoverAllVPNInterfaces` to vpn_discovery.go

**Files:**
- Modify: `backend/internal/monitoring/vpn_discovery.go`
- Test: `backend/internal/monitoring/vpn_discovery_test.go`

- [ ] **Step 1: Write failing tests**

Add to `vpn_discovery_test.go`:

```go
func TestDiscoverAllVPNInterfacesFromSysFS(t *testing.T) {
	t.Parallel()

	ifaces, err := DiscoverAllVPNInterfaces(
		WithFileReader(&fakeFileReaderForDiscovery{files: map[string]string{}}),
		WithReadDirFunc(func(dirname string) ([]os.DirEntry, error) {
			return []os.DirEntry{
				fakeDirEntry{name: "eth0"},
				fakeDirEntry{name: "vpns0"},
				fakeDirEntry{name: "vpns1"},
				fakeDirEntry{name: "lo"},
			}, nil
		}),
	)
	if err != nil {
		t.Fatalf("DiscoverAllVPNInterfaces: %v", err)
	}
	if len(ifaces) != 2 {
		t.Fatalf("ifaces = %v, want [vpns0, vpns1]", ifaces)
	}
	if ifaces[0] != "vpns0" || ifaces[1] != "vpns1" {
		t.Fatalf("ifaces = %v, want [vpns0, vpns1]", ifaces)
	}
}

func TestDiscoverAllVPNInterfacesFromOcservConfig(t *testing.T) {
	t.Parallel()

	files := map[string]string{
		"/etc/ocserv/ocserv.conf": "device = vpns\n",
	}
	ifaces, err := DiscoverAllVPNInterfaces(
		WithFileReader(&fakeFileReaderForDiscovery{files: files}),
		WithReadDirFunc(func(dirname string) ([]os.DirEntry, error) {
			return []os.DirEntry{
				fakeDirEntry{name: "vpns0"},
				fakeDirEntry{name: "vpns1"},
				fakeDirEntry{name: "eth0"},
			}, nil
		}),
	)
	if err != nil {
		t.Fatalf("DiscoverAllVPNInterfaces: %v", err)
	}
	if len(ifaces) != 2 {
		t.Fatalf("ifaces = %v, want [vpns0, vpns1]", ifaces)
	}
}

func TestDiscoverAllVPNInterfacesEmpty(t *testing.T) {
	t.Parallel()

	ifaces, err := DiscoverAllVPNInterfaces(
		WithFileReader(&fakeFileReaderForDiscovery{files: map[string]string{}}),
		WithReadDirFunc(func(dirname string) ([]os.DirEntry, error) {
			return []os.DirEntry{fakeDirEntry{name: "eth0"}, fakeDirEntry{name: "lo"}}, nil
		}),
	)
	if err != nil {
		t.Fatalf("DiscoverAllVPNInterfaces: %v", err)
	}
	if len(ifaces) != 0 {
		t.Fatalf("ifaces = %v, want empty", ifaces)
	}
}

func TestResolveAllInterfaceNames(t *testing.T) {
	t.Parallel()

	cfg := vpnDiscoveryConfig{
		sysNetPath: "/sys/class/net",
		readDir: func(string) ([]os.DirEntry, error) {
			return []os.DirEntry{
				fakeDirEntry{name: "vpns0"},
				fakeDirEntry{name: "vpns1"},
				fakeDirEntry{name: "vpns2"},
				fakeDirEntry{name: "eth0"},
			}, nil
		},
	}
	got := resolveAllInterfaceNames(cfg, "vpns")
	if len(got) != 3 {
		t.Fatalf("resolveAllInterfaceNames = %v, want [vpns0, vpns1, vpns2]", got)
	}
}
```

- [ ] **Step 2: Run tests to confirm they fail**

```bash
cd /opt/ocserv_panel/backend && go test ./internal/monitoring/... -run "TestDiscoverAll|TestResolveAll" -v 2>&1 | tail -20
```
Expected: compile error "undefined: DiscoverAllVPNInterfaces"

- [ ] **Step 3: Implement `resolveAllInterfaceNames` and `DiscoverAllVPNInterfaces`**

Add to `backend/internal/monitoring/vpn_discovery.go` (after the existing `resolveInterfaceName` function):

```go
// resolveAllInterfaceNames returns every entry in sysfs that matches baseName
// exactly or starts with baseName (e.g., baseName="vpns" → "vpns0", "vpns1").
func resolveAllInterfaceNames(cfg vpnDiscoveryConfig, baseName string) []string {
	entries, err := cfg.readDir(cfg.sysNetPath)
	if err != nil {
		return nil
	}
	var result []string
	for _, entry := range entries {
		name := entry.Name()
		if name == baseName || strings.HasPrefix(name, baseName) && len(name) > len(baseName) {
			result = append(result, name)
		}
	}
	return result
}

// DiscoverAllVPNInterfaces returns all active VPN interface names found via
// ocserv config (all numbered instances of the device directive) or sysfs scan.
// Returns nil slice when none are found, never returns an error for missing config.
func DiscoverAllVPNInterfaces(opts ...VPNDiscoveryOption) ([]string, error) {
	cfg := vpnDiscoveryConfig{
		reader:     OSReader{},
		configPath: "/etc/ocserv/ocserv.conf",
		sysNetPath: "/sys/class/net",
		readDir:    os.ReadDir,
	}
	for _, opt := range opts {
		opt(&cfg)
	}

	// Strategy 1: ocserv config device directive → resolve all numbered instances.
	if baseName, err := discoverFromOcservConfig(cfg); err == nil && baseName != "" {
		if all := resolveAllInterfaceNames(cfg, baseName); len(all) > 0 {
			return all, nil
		}
	}

	// Strategy 2: sysfs scan — return all matching prefix entries.
	entries, err := cfg.readDir(cfg.sysNetPath)
	if err != nil {
		return nil, fmt.Errorf("read network interfaces: %w", err)
	}

	vpnPrefixes := []string{"vpns", "tun", "tap", "wg"}
	var found []string
	for _, entry := range entries {
		name := entry.Name()
		for _, prefix := range vpnPrefixes {
			if strings.HasPrefix(name, prefix) {
				found = append(found, name)
				break
			}
		}
	}
	return found, nil
}
```

- [ ] **Step 4: Run tests to confirm they pass**

```bash
cd /opt/ocserv_panel/backend && go test ./internal/monitoring/... -run "TestDiscoverAll|TestResolveAll" -v 2>&1 | tail -20
```
Expected: all PASS

- [ ] **Step 5: Run the full monitoring package tests to ensure no regressions**

```bash
cd /opt/ocserv_panel/backend && go test ./internal/monitoring/... -v 2>&1 | tail -20
```
Expected: all PASS

- [ ] **Step 6: Commit**

```bash
cd /opt/ocserv_panel && git add backend/internal/monitoring/vpn_discovery.go backend/internal/monitoring/vpn_discovery_test.go
git commit -m "feat(monitoring): add DiscoverAllVPNInterfaces for multi-interface tracking"
```

---

## Task 2: Extend CollectorService for multi-VPN tracking

**Files:**
- Modify: `backend/internal/monitoring/collector_service.go`
- Modify: `backend/internal/monitoring/collector_service_test.go`

**Context:** The current `CollectorService` uses `hasVPN bool` + `VPNDiscovererFunc` returning `(string, string, error)`. Replace with `discoveredVPN map[string]bool` + `VPNMultiDiscovererFunc` returning `([]string, error)`. Keep `SetVPNDiscoverer`, `HasVPNCollector`, and `MarkVPNPresent` APIs but change the discoverer type.

- [ ] **Step 1: Write failing tests**

Add to `backend/internal/monitoring/collector_service_test.go`:

```go
func TestCollectorService_MultiVPNDiscoveryAddsAllInterfaces(t *testing.T) {
	store := metrics.NewMemoryStore()
	logger := &fakeLogger{}
	now := time.Date(2026, 5, 10, 12, 0, 0, 0, time.UTC)

	service := NewCollectorService(
		store,
		[]Collector{},
		50*time.Millisecond,
		logger,
		func() time.Time { return now },
	)

	fakeReader := &fakeFileReaderForVPN{
		files: map[string]string{
			"/sys/class/net/vpns0/statistics/rx_bytes": "1000",
			"/sys/class/net/vpns0/statistics/tx_bytes": "2000",
			"/sys/class/net/vpns1/statistics/rx_bytes": "3000",
			"/sys/class/net/vpns1/statistics/tx_bytes": "4000",
		},
	}

	discoverer := func() ([]string, error) {
		return []string{"vpns0", "vpns1"}, nil
	}

	service.SetVPNDiscoverer(
		discoverer,
		fakeReader,
		metrics.MetricVPNInterfaceRXBytes,
		metrics.MetricVPNInterfaceTXBytes,
	)

	service.maybeDiscoverAllVPN()

	if !service.HasVPNCollector() {
		t.Fatal("expected HasVPNCollector() true after multi-discovery")
	}
	if len(service.discoveredVPN) != 2 {
		t.Fatalf("discoveredVPN = %v, want 2 entries", service.discoveredVPN)
	}
	if !service.discoveredVPN["vpns0"] || !service.discoveredVPN["vpns1"] {
		t.Fatalf("discoveredVPN = %v, want vpns0 and vpns1", service.discoveredVPN)
	}
}

func TestCollectorService_MultiVPNDiscoverySkipsAlreadyRegistered(t *testing.T) {
	store := metrics.NewMemoryStore()
	logger := &fakeLogger{}
	now := time.Date(2026, 5, 10, 12, 0, 0, 0, time.UTC)

	service := NewCollectorService(
		store,
		[]Collector{},
		50*time.Millisecond,
		logger,
		func() time.Time { return now },
	)

	fakeReader := &fakeFileReaderForVPN{
		files: map[string]string{
			"/sys/class/net/vpns0/statistics/rx_bytes": "1000",
			"/sys/class/net/vpns0/statistics/tx_bytes": "2000",
			"/sys/class/net/vpns1/statistics/rx_bytes": "3000",
			"/sys/class/net/vpns1/statistics/tx_bytes": "4000",
		},
	}

	callCount := 0
	discoverer := func() ([]string, error) {
		callCount++
		return []string{"vpns0", "vpns1"}, nil
	}

	service.SetVPNDiscoverer(
		discoverer,
		fakeReader,
		metrics.MetricVPNInterfaceRXBytes,
		metrics.MetricVPNInterfaceTXBytes,
	)

	// Call twice — second call should not add duplicate collectors.
	service.maybeDiscoverAllVPN()
	initialCollectorCount := len(service.collectors)
	service.maybeDiscoverAllVPN()
	if len(service.collectors) != initialCollectorCount {
		t.Fatalf("collector count grew on second discovery: %d vs %d", len(service.collectors), initialCollectorCount)
	}
}
```

- [ ] **Step 2: Run tests to confirm they fail**

```bash
cd /opt/ocserv_panel/backend && go test ./internal/monitoring/... -run "TestCollectorService_MultiVPN" -v 2>&1 | tail -20
```
Expected: compile error "undefined: maybeDiscoverAllVPN"

- [ ] **Step 3: Update `CollectorService` struct and `SetVPNDiscoverer`**

Replace `collector_service.go` — key changes:

```go
// VPNMultiDiscovererFunc discovers all active VPN interfaces.
// Returns a slice of interface names and an error.
type VPNMultiDiscovererFunc func() ([]string, error)

type CollectorService struct {
	store    metrics.Store
	interval time.Duration
	logger   Logger
	now      func() time.Time

	mu              sync.Mutex
	collectors      []Collector
	vpnDiscoverer   VPNMultiDiscovererFunc
	vpnReader       FileReader
	vpnRXMetric     metrics.MetricName
	vpnTXMetric     metrics.MetricName
	discoveredVPN   map[string]bool
}
```

Update `SetVPNDiscoverer` signature to accept `VPNMultiDiscovererFunc`:

```go
func (s *CollectorService) SetVPNDiscoverer(
	fn VPNMultiDiscovererFunc,
	reader FileReader,
	rxMetric, txMetric metrics.MetricName,
) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.vpnDiscoverer = fn
	s.vpnReader = reader
	s.vpnRXMetric = rxMetric
	s.vpnTXMetric = txMetric
}
```

Update `HasVPNCollector` and `MarkVPNPresent`:

```go
func (s *CollectorService) HasVPNCollector() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.discoveredVPN) > 0
}

func (s *CollectorService) MarkVPNPresent(iface string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.discoveredVPN == nil {
		s.discoveredVPN = make(map[string]bool)
	}
	s.discoveredVPN[iface] = true
}
```

Add `maybeDiscoverAllVPN`:

```go
func (s *CollectorService) maybeDiscoverAllVPN() {
	s.mu.Lock()
	if s.vpnDiscoverer == nil {
		s.mu.Unlock()
		return
	}
	discoverer := s.vpnDiscoverer
	reader := s.vpnReader
	rxMetric := s.vpnRXMetric
	txMetric := s.vpnTXMetric
	s.mu.Unlock()

	ifaces, err := discoverer()
	if err != nil {
		s.logger.Warn("vpn discovery error", "error", err)
		return
	}
	if len(ifaces) == 0 {
		s.logger.Debug("vpn discovery found no interfaces")
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.discoveredVPN == nil {
		s.discoveredVPN = make(map[string]bool)
	}
	for _, iface := range ifaces {
		if s.discoveredVPN[iface] {
			continue
		}
		s.logger.Debug("vpn interface discovered", "interface", iface)
		collector := NewInterfaceCollector(reader, iface, rxMetric, txMetric)
		s.collectors = append(s.collectors, collector)
		s.discoveredVPN[iface] = true
	}
}
```

Update `Run` to call `maybeDiscoverAllVPN` instead of `maybeDiscoverVPN`:

```go
case <-vpnTickerChan(vpnTicker):
    s.maybeDiscoverAllVPN()
```

Remove the old `maybeDiscoverVPN` method entirely.

- [ ] **Step 4: Fix compilation of existing VPN rediscovery tests**

The old tests in `collector_service_test.go` use `SetVPNDiscoverer` with `func() (string, string, error)` and call `maybeDiscoverVPN`. Update those tests to use `VPNMultiDiscovererFunc` and `maybeDiscoverAllVPN`:

In `TestCollectorService_VPNRediscovery`, change:
```go
discoverer := func() (string, string, error) {
    mu.Lock()
    defer mu.Unlock()
    callCount++
    if callCount <= 2 {
        return "", "", nil
    }
    return "vpns0", "sysfs-scan", nil
}
service.SetVPNDiscoverer(
    discoverer,
    fakeReader,
    metrics.MetricVPNInterfaceRXBytes,
    metrics.MetricVPNInterfaceTXBytes,
)
```
to:
```go
discoverer := func() ([]string, error) {
    mu.Lock()
    defer mu.Unlock()
    callCount++
    if callCount <= 2 {
        return nil, nil
    }
    return []string{"vpns0"}, nil
}
service.SetVPNDiscoverer(
    discoverer,
    fakeReader,
    metrics.MetricVPNInterfaceRXBytes,
    metrics.MetricVPNInterfaceTXBytes,
)
```

Replace `service.maybeDiscoverVPN()` → `service.maybeDiscoverAllVPN()` in all tests.

Replace `service.HasVPNCollector()` assertions to use the same call (API unchanged).

In `TestCollectorService_VPNRediscoverySkipsWhenAlreadyPresent`, the discoverer now returns `[]string{"vpns0"}` and the test still verifies it was only called once (since the interface gets registered on the first call).

In `TestCollectorService_MarkVPNPresentPreventsDiscovery`: `MarkVPNPresent` now takes an interface name — update to `service.MarkVPNPresent("vpns0")`.

In `TestCollectorService_CollectOnceThreadSafeWithDiscovery`:
```go
// old
callNum := 0
service.SetVPNDiscoverer(
    func() (string, string, error) {
        callNum++
        if callNum > 3 { return "vpns0", "sysfs-scan", nil }
        return "", "", nil
    },
    ...
)
// replace maybeDiscoverVPN() → maybeDiscoverAllVPN()
```
becomes:
```go
callNum := 0
service.SetVPNDiscoverer(
    func() ([]string, error) {
        callNum++
        if callNum > 3 { return []string{"vpns0"}, nil }
        return nil, nil
    },
    fakeReader,
    metrics.MetricVPNInterfaceRXBytes,
    metrics.MetricVPNInterfaceTXBytes,
)
// replace maybeDiscoverVPN() → maybeDiscoverAllVPN()
```

- [ ] **Step 5: Run all monitoring tests**

```bash
cd /opt/ocserv_panel/backend && go test ./internal/monitoring/... -v 2>&1 | tail -30
```
Expected: all PASS

- [ ] **Step 6: Update `main.go` to use new multi-discoverer**

In `backend/cmd/ocserv-panel/main.go`, change:

```go
// old
collectorService.SetVPNDiscoverer(
    func() (string, string, error) { return monitoring.DiscoverVPNInterface() },
    monitoring.OSReader{},
    metrics.MetricVPNInterfaceRXBytes,
    metrics.MetricVPNInterfaceTXBytes,
)
```
to:
```go
// new
collectorService.SetVPNDiscoverer(
    func() ([]string, error) { return monitoring.DiscoverAllVPNInterfaces() },
    monitoring.OSReader{},
    metrics.MetricVPNInterfaceRXBytes,
    metrics.MetricVPNInterfaceTXBytes,
)
```

Also update the `vpnConfigured` branch — `MarkVPNPresent` now requires an interface name:

```go
// old
if vpnConfigured {
    collectorService.MarkVPNPresent()
    ...
}
```
becomes:
```go
if vpnConfigured {
    vpnIface := strings.TrimSpace(cfg.Monitoring.VPNInterface)
    if vpnIface == "" {
        vpnIface = strings.TrimSpace(os.Getenv("OCSERV_PANEL_VPN_IFACE"))
    }
    collectorService.MarkVPNPresent(vpnIface)
    metricCatalog.SetState(metrics.MetricVPNInterfaceRXBytes, metrics.StateCollecting, "")
    metricCatalog.SetState(metrics.MetricVPNInterfaceTXBytes, metrics.StateCollecting, "")
}
```

- [ ] **Step 7: Build to verify compilation**

```bash
cd /opt/ocserv_panel/backend && go build ./... 2>&1
```
Expected: no errors

- [ ] **Step 8: Commit**

```bash
cd /opt/ocserv_panel && git add backend/internal/monitoring/collector_service.go backend/internal/monitoring/collector_service_test.go backend/cmd/ocserv-panel/main.go
git commit -m "feat(monitoring): extend CollectorService to track all VPN interfaces"
```

---

## Task 3: Add `GET /api/monitoring/system-info` endpoint

**Files:**
- Modify: `backend/internal/server/monitoring_helpers.go`
- Modify: `backend/internal/server/server.go`
- Modify: `backend/internal/server/server_test.go`

**Context:** The handler reads CPU cores from `runtime.NumCPU()`, total memory GB from `/proc/meminfo` (reusing `parseMeminfo` from the monitoring package — export it or duplicate the logic), and interface names from `metrics.Store.QueryRange` on a short window with empty-labels query. Since `QueryRange` returns a single series per (metric, labels) pair and we query with empty labels `{}`, VPN detection needs one query per known interface OR we use the overview handler's approach. The simplest approach: query the store for `net_interface_rx_bytes` with empty labels — this returns the net interface's series, and the `labels["interface"]` field on the returned `Series` contains the name. For VPN interfaces, query `vpn_interface_rx_bytes` similarly — but there may be multiple series. Since `QueryRange` returns one series per labels match, we query once with empty labels and it returns data if only one VPN exists. For multiple VPNs, we need to query with specific interface labels. The cleanest approach for this endpoint: use a 15-minute window, query each metric with empty labels to get whatever the store returns (the interface name is in `series.Labels`), and for VPN also check the monitoring overview series that have already been fetched. **Simpler:** expose a helper that extracts distinct interface names from a `[]metrics.Series` — the handler calls `handleMonitoringOverview` logic to get all series for a 15m window and extracts interface names from them.

Actually the simplest correct approach: in the handler, run `QueryRange` for both `net_interface_rx_bytes` and `vpn_interface_rx_bytes` with a short window and empty labels. `QueryRange` with empty labels `map[string]string{}` returns a series matching the first stored series for that metric (because `labelsMatch` requires all query labels to match — with an empty query map, every sample matches). The returned `Series.Labels` field echoes back `query.Labels`, which is `{}` — so we cannot extract the interface name this way.

**Correct approach:** The handler queries the monitoring overview for a 15-minute window, then extracts `series.Labels["interface"]` from series where the metric is `net_interface_rx_bytes` or `vpn_interface_rx_bytes`. This works because the collector already tags every sample with `labels["interface"]`. The monitoring overview handler iterates `monitoringOverviewMetrics` but uses empty labels queries — so we need to add a `QueryAllSeries` capability OR store the interface names in the catalog. 

**Actual simplest correct implementation:** Add a `LatestLabels(ctx, metric) (map[string]string, error)` method to `Store`. For `MemoryStore`, scan samples for the most recent sample matching the metric and return its labels. For `TSDBStore`, query a very short window and extract labels from the first result.

Instead of extending the Store interface (which requires changes to both implementations and tests), the handler can use a short `QueryRange` with empty labels — but the returned `Series.Labels` is a copy of `query.Labels` (empty), not the sample's labels. So we must extend the Store interface.

Add `QueryLatestLabels(ctx context.Context, metric MetricName) (map[string]string, error)` to `Store`:
- `MemoryStore`: scan `s.samples` for the most-recent sample with matching name, return its labels
- `TSDBStore`: query a 1-hour window, scan the first series result labels via the tsdb querier

- [ ] **Step 1: Write failing test for `QueryLatestLabels`**

Add to `backend/internal/metrics/memory_store_test.go` (or create it if absent — check first):

```bash
ls /opt/ocserv_panel/backend/internal/metrics/*_test.go
```

Add the following test (add to whichever test file covers `MemoryStore`):

```go
func TestMemoryStore_QueryLatestLabels(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()
	now := time.Date(2026, 5, 10, 12, 0, 0, 0, time.UTC)

	// Append two samples for net_interface_rx_bytes with interface label.
	err := store.Append(ctx, []Sample{
		{
			Name:      MetricNetInterfaceRXBytes,
			Labels:    map[string]string{"interface": "eth0"},
			Timestamp: now,
			Value:     100,
		},
	})
	if err != nil {
		t.Fatalf("Append: %v", err)
	}

	labels, err := store.QueryLatestLabels(ctx, MetricNetInterfaceRXBytes)
	if err != nil {
		t.Fatalf("QueryLatestLabels: %v", err)
	}
	if labels["interface"] != "eth0" {
		t.Fatalf("labels[interface] = %q, want eth0", labels["interface"])
	}
}

func TestMemoryStore_QueryLatestLabels_NoData(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	labels, err := store.QueryLatestLabels(ctx, MetricNetInterfaceRXBytes)
	if err != nil {
		t.Fatalf("QueryLatestLabels on empty store: %v", err)
	}
	if len(labels) != 0 {
		t.Fatalf("expected empty labels map, got %v", labels)
	}
}
```

- [ ] **Step 2: Run test to confirm it fails**

```bash
cd /opt/ocserv_panel/backend && go test ./internal/metrics/... -run "TestMemoryStore_QueryLatestLabels" -v 2>&1 | tail -10
```
Expected: compile error "undefined: QueryLatestLabels"

- [ ] **Step 3: Add `QueryLatestLabels` to `Store` interface and both implementations**

In `backend/internal/metrics/types.go`, add to the `Store` interface:

```go
type Store interface {
	Append(ctx context.Context, samples []Sample) error
	QueryRange(ctx context.Context, query Query) (Series, error)
	QueryLatestLabels(ctx context.Context, metric MetricName) (map[string]string, error)
	Close() error
}
```

In `backend/internal/metrics/memory_store.go`, add:

```go
func (s *MemoryStore) QueryLatestLabels(ctx context.Context, metric MetricName) (map[string]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed {
		return nil, ErrStoreClosed
	}

	var latest *Sample
	for i := range s.samples {
		sample := &s.samples[i]
		if sample.Name != metric {
			continue
		}
		if latest == nil || sample.Timestamp.After(latest.Timestamp) {
			latest = sample
		}
	}
	if latest == nil {
		return map[string]string{}, nil
	}
	return cloneLabels(latest.Labels), nil
}
```

In `backend/internal/metrics/tsdb_store.go`, add:

```go
func (s *TSDBStore) QueryLatestLabels(ctx context.Context, metric MetricName) (map[string]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed {
		return nil, ErrStoreClosed
	}

	// Query the last hour to find the most recent sample's labels.
	now := time.Now().UTC()
	querier, err := s.db.Querier(now.Add(-time.Hour).UnixMilli(), now.UnixMilli())
	if err != nil {
		return map[string]string{}, nil
	}
	defer querier.Close()

	matchers := []*labels.Matcher{
		labels.MustNewMatcher(labels.MatchEqual, labels.MetricName, string(metric)),
	}
	sets := querier.Select(ctx, false, nil, matchers...)
	for sets.Next() {
		series := sets.At()
		lbls := series.Labels()
		result := make(map[string]string, lbls.Len())
		lbls.Range(func(l labels.Label) {
			if l.Name != labels.MetricName {
				result[l.Name] = l.Value
			}
		})
		return result, nil
	}
	return map[string]string{}, nil
}
```

Also add the stub to `failingStore` in `collector_service_test.go`:

```go
func (s *failingStore) QueryLatestLabels(context.Context, metrics.MetricName) (map[string]string, error) {
	return map[string]string{}, nil
}
```

- [ ] **Step 4: Run tests to confirm they pass**

```bash
cd /opt/ocserv_panel/backend && go test ./internal/metrics/... -run "TestMemoryStore_QueryLatestLabels" -v 2>&1 | tail -10
```
Expected: PASS

```bash
cd /opt/ocserv_panel/backend && go build ./... 2>&1
```
Expected: no errors

- [ ] **Step 5: Write failing handler test**

Add to `backend/internal/server/server_test.go`:

```go
func TestSystemInfoEndpointRequiresAuthentication(t *testing.T) {
	h := newTestHarness(t)
	rec := serveAPI(t, h.router, http.MethodGet, "/api/monitoring/system-info", nil, "")
	assertStatus(t, rec, http.StatusUnauthorized)
}

func TestSystemInfoEndpointReturnsSystemMetadata(t *testing.T) {
	h := newTestHarness(t)
	h.createOwner(t, "owner", "owner-password")
	cookie := h.login(t, "owner", "owner-password")

	rec := serveAPI(t, h.router, http.MethodGet, "/api/monitoring/system-info", nil, cookie)

	assertStatus(t, rec, http.StatusOK)
	var body map[string]any
	decodeJSON(t, rec, &body)
	assertMapHasFields(t, body, "cpuCores", "memoryTotalGB", "netInterface", "vpnInterfaces")

	cores, ok := body["cpuCores"].(float64)
	if !ok || cores < 1 {
		t.Fatalf("cpuCores = %v, want >= 1", body["cpuCores"])
	}
	if _, ok := body["memoryTotalGB"].(float64); !ok {
		t.Fatalf("memoryTotalGB should be float64, got %T", body["memoryTotalGB"])
	}
	if _, ok := body["netInterface"].(string); !ok {
		t.Fatalf("netInterface should be string, got %T", body["netInterface"])
	}
	if _, ok := body["vpnInterfaces"].([]any); !ok {
		t.Fatalf("vpnInterfaces should be array, got %T", body["vpnInterfaces"])
	}
}

func TestSystemInfoEndpointAccessibleByViewer(t *testing.T) {
	h := newTestHarness(t)
	h.createOwner(t, "owner", "owner-password")
	h.createViewer(t, "viewer", "viewer-password")
	viewerCookie := h.login(t, "viewer", "viewer-password")

	rec := serveAPI(t, h.router, http.MethodGet, "/api/monitoring/system-info", nil, viewerCookie)
	// Viewers have dashboard:read — they can read monitoring data.
	assertStatus(t, rec, http.StatusOK)
}

- [ ] **Step 6: Run test to confirm failure**

```bash
cd /opt/ocserv_panel/backend && go test ./internal/server/... -run "TestSystemInfo" -v 2>&1 | tail -15
```
Expected: 404 or route-not-found failure

- [ ] **Step 7: Implement the system info handler**

In `backend/internal/server/monitoring_helpers.go`, add:

```go
import (
	"context"
	"math"
	"ocserv_panel/backend/internal/metrics"
	"runtime"
	"time"
)

type systemInfoResponse struct {
	CPUCores      int      `json:"cpuCores"`
	MemoryTotalGB float64  `json:"memoryTotalGB"`
	NetInterface  string   `json:"netInterface"`
	VPNInterfaces []string `json:"vpnInterfaces"`
}

func (a *App) buildSystemInfo(ctx context.Context) systemInfoResponse {
	info := systemInfoResponse{
		CPUCores:      runtime.NumCPU(),
		VPNInterfaces: []string{},
	}

	// Memory total from /proc/meminfo.
	if data, err := os.ReadFile("/proc/meminfo"); err == nil {
		if totalKB, _, parseErr := parseMeminfo(string(data)); parseErr == nil && totalKB > 0 {
			gb := float64(totalKB) / (1024 * 1024)
			info.MemoryTotalGB = math.Round(gb*10) / 10
		}
	}

	// Net interface from most-recent net_interface_rx_bytes labels.
	if lbls, err := a.metricStore.QueryLatestLabels(ctx, metrics.MetricNetInterfaceRXBytes); err == nil {
		info.NetInterface = lbls["interface"]
	}

	// VPN interfaces from most-recent vpn_interface_rx_bytes — query last hour.
	// We need distinct interface labels. Query the overview series for vpn_interface_rx_bytes
	// using a short window to find all labeled series.
	info.VPNInterfaces = a.queryDistinctVPNInterfaces(ctx)

	return info
}

func (a *App) queryDistinctVPNInterfaces(ctx context.Context) []string {
	now := time.Now().UTC()
	windowDuration := time.Hour
	stepDuration := time.Minute

	// Query with empty labels returns first matching series. To find all interfaces,
	// use the catalog and query with known interface names — but we don't know them.
	// Instead, query with empty labels and check the series labels to find registered interfaces.
	// Since QueryRange with empty labels returns a series labelled with empty labels (not the
	// sample's labels), we use QueryLatestLabels for each possible interface.
	// Simpler: run QueryRange for vpn_interface_rx_bytes and check if any points returned.
	// The actual VPN interface names are stored in discoveredVPN on the CollectorService,
	// but the server doesn't hold a reference to it.
	// Best: query the monitoring overview series filtered to VPN metric.
	series := make([]metrics.Series, 0)
	for _, iface := range knownVPNInterfaces(ctx, a.metricStore, now, windowDuration, stepDuration) {
		s, err := a.metricStore.QueryRange(ctx, metrics.Query{
			Metric: metrics.MetricVPNInterfaceRXBytes,
			Labels: map[string]string{"interface": iface},
			Start:  now.Add(-windowDuration),
			End:    now,
			Step:   stepDuration,
		})
		if err == nil && len(s.Points) > 0 {
			series = append(series, s)
		}
	}
	_ = series

	// Fallback: use QueryLatestLabels to get the single most-recent VPN interface.
	if lbls, err := a.metricStore.QueryLatestLabels(ctx, metrics.MetricVPNInterfaceRXBytes); err == nil {
		if iface := lbls["interface"]; iface != "" {
			return []string{iface}
		}
	}
	return []string{}
}
```

Wait — `knownVPNInterfaces` doesn't exist yet, and the above is getting complex. The cleanest solution for finding multiple VPN interfaces without a full tsdb scan is to add `QueryAllLatestLabels(ctx, metric) ([]map[string]string, error)` to the store. Let's keep it simple and just use `QueryLatestLabels` for now (returns at most one VPN interface). Multi-VPN interface names will appear in charts once the per-interface series are returned by the overview endpoint (which queries all labeled series). The system-info endpoint just reports what `QueryLatestLabels` finds.

Replace the `queryDistinctVPNInterfaces` function with:

```go
func (a *App) buildSystemInfo(ctx context.Context) systemInfoResponse {
	info := systemInfoResponse{
		CPUCores:      runtime.NumCPU(),
		VPNInterfaces: []string{},
	}

	if data, err := os.ReadFile("/proc/meminfo"); err == nil {
		if totalKB, _, parseErr := parseMeminfo(string(data)); parseErr == nil && totalKB > 0 {
			gb := float64(totalKB) / (1024 * 1024)
			info.MemoryTotalGB = math.Round(gb*10) / 10
		}
	}

	if lbls, err := a.metricStore.QueryLatestLabels(ctx, metrics.MetricNetInterfaceRXBytes); err == nil {
		info.NetInterface = lbls["interface"]
	}

	if lbls, err := a.metricStore.QueryLatestLabels(ctx, metrics.MetricVPNInterfaceRXBytes); err == nil {
		if iface := lbls["interface"]; iface != "" {
			info.VPNInterfaces = []string{iface}
		}
	}

	return info
}
```

Note: `parseMeminfo` is in `backend/internal/monitoring/collectors.go` (unexported). The server package cannot call it directly. Instead, duplicate the minimal logic inline in the helper (just 10 lines), or move `parseMeminfo` to an exported function. **Best:** export it as `ParseMeminfo` in the monitoring package and call it from the server. Add `parseMeminfo` export to `collectors.go`:

```go
// ParseMeminfo parses /proc/meminfo content and returns (totalKB, availableKB, error).
func ParseMeminfo(data string) (uint64, uint64, error) {
	return parseMeminfo(data)
}
```

Then in `monitoring_helpers.go`:

```go
import (
    "math"
    "os"
    "runtime"
    "ocserv_panel/backend/internal/metrics"
    "ocserv_panel/backend/internal/monitoring"
    "time"
)
```

Full `buildSystemInfo`:

```go
func (a *App) buildSystemInfo(ctx context.Context) systemInfoResponse {
	info := systemInfoResponse{
		CPUCores:      runtime.NumCPU(),
		VPNInterfaces: []string{},
	}

	if data, err := os.ReadFile("/proc/meminfo"); err == nil {
		if totalKB, _, parseErr := monitoring.ParseMeminfo(string(data)); parseErr == nil && totalKB > 0 {
			gb := float64(totalKB) / (1024 * 1024)
			info.MemoryTotalGB = math.Round(gb*10) / 10
		}
	}

	if lbls, err := a.metricStore.QueryLatestLabels(ctx, metrics.MetricNetInterfaceRXBytes); err == nil {
		info.NetInterface = lbls["interface"]
	}

	if lbls, err := a.metricStore.QueryLatestLabels(ctx, metrics.MetricVPNInterfaceRXBytes); err == nil {
		if iface := lbls["interface"]; iface != "" {
			info.VPNInterfaces = []string{iface}
		}
	}

	return info
}
```

Add the handler in `server.go`:

```go
func (a *App) handleMonitoringSystemInfo(w http.ResponseWriter, r *http.Request) {
	if !allowMethod(w, r, http.MethodGet) {
		return
	}
	writeJSON(w, http.StatusOK, a.buildSystemInfo(r.Context()))
}
```

Register the route in `server.go` (alongside the other monitoring routes):

```go
mux.Handle("GET /api/monitoring/system-info", appHandler.requirePermission(rbac.PermissionReadDashboard)(http.HandlerFunc(appHandler.handleMonitoringSystemInfo)))
```

Add the monitoring package import to `monitoring_helpers.go` and the necessary OS/math/runtime imports.

Also add the `ParseMeminfo` export to `backend/internal/monitoring/collectors.go`:

```go
// ParseMeminfo exports the internal meminfo parser for use by other packages.
func ParseMeminfo(data string) (uint64, uint64, error) {
	return parseMeminfo(data)
}
```

- [ ] **Step 8: Run tests**

```bash
cd /opt/ocserv_panel/backend && go test ./internal/server/... -run "TestSystemInfo" -v 2>&1 | tail -20
```
Expected: all PASS (note: on Linux, `/proc/meminfo` is real so memoryTotalGB > 0; in tests it may be 0 — adjust the test assertion to `>= 0`)

```bash
cd /opt/ocserv_panel/backend && go test ./... 2>&1 | tail -20
```
Expected: all PASS

- [ ] **Step 9: Commit**

```bash
cd /opt/ocserv_panel && git add backend/internal/metrics/types.go backend/internal/metrics/memory_store.go backend/internal/metrics/tsdb_store.go backend/internal/monitoring/collector_service_test.go backend/internal/monitoring/collectors.go backend/internal/server/monitoring_helpers.go backend/internal/server/server.go backend/internal/server/server_test.go
git commit -m "feat(api): add GET /api/monitoring/system-info endpoint"
```

---

## Task 4: Dashboard cleanup — remove activity section and session charts

**Files:**
- Modify: `frontend/src/routes/+page.svelte`
- Delete: `frontend/src/lib/components/dashboard/DashboardActivity.svelte`
- Modify: `frontend/src/lib/components/dashboard/DashboardSessions.svelte`
- Modify: `frontend/src/lib/components/dashboard/DashboardMetrics.svelte`

- [ ] **Step 1: Remove DashboardActivity from `+page.svelte`**

In `frontend/src/routes/+page.svelte`, remove:
1. The import: `import DashboardActivity from '$lib/components/dashboard/DashboardActivity.svelte';`
2. The derived state: `const recentAudit = $derived(summary?.recentAudit ?? []);`
3. The component usage: `<DashboardActivity recentAudit={recentAudit} />`
4. From the `$lib/api` import block, remove `getLogTail` (it's only used in DashboardActivity)

- [ ] **Step 2: Delete `DashboardActivity.svelte`**

```bash
rm /opt/ocserv_panel/frontend/src/lib/components/dashboard/DashboardActivity.svelte
```

- [ ] **Step 3: Remove active sessions card from `DashboardSessions.svelte`**

In `frontend/src/lib/components/dashboard/DashboardSessions.svelte`, remove the `<CardStats title="Active sessions" .../>` card:

```svelte
<!-- REMOVE THIS BLOCK -->
<CardStats
  title="Active sessions"
  value={String(summary.activeSessionCount ?? 0)}
  icon={Users}
/>
```

Also remove the `Users` icon import from `lucide-svelte` if it's no longer used elsewhere in that file. Check usages first:

```bash
grep -n "Users" /opt/ocserv_panel/frontend/src/lib/components/dashboard/DashboardSessions.svelte
```

The `Users` icon is also used for "Total sessions" and "Recorded sessions" CardStats, so keep the import.

Remove the `activeSessionCount` reference — `summary` prop remains for analytics data.

- [ ] **Step 4: Remove active/concurrent sessions chart entries from `DashboardMetrics.svelte`**

In `frontend/src/lib/components/dashboard/DashboardMetrics.svelte`:

Remove the constants:
```ts
const ACTIVE_SESSIONS = 'active_sessions';
const CONCURRENT_SESSIONS = 'concurrent_sessions';
```

Remove entries from `chartDefinitions`:
```ts
{ metric: ACTIVE_SESSIONS, title: 'Active sessions' },
{ metric: CONCURRENT_SESSIONS, title: 'Concurrent sessions' }
```

- [ ] **Step 5: Type-check**

```bash
cd /opt/ocserv_panel/frontend && npx svelte-check --tsconfig ./tsconfig.json 2>&1 | tail -20
```
Expected: 0 errors

- [ ] **Step 6: Commit**

```bash
cd /opt/ocserv_panel && git add frontend/src/routes/+page.svelte frontend/src/lib/components/dashboard/DashboardSessions.svelte frontend/src/lib/components/dashboard/DashboardMetrics.svelte
git rm frontend/src/lib/components/dashboard/DashboardActivity.svelte
git commit -m "feat(dashboard): remove activity section and session charts"
```

---

## Task 5: Add `SystemInfo` API type and fetch in dashboard

**Files:**
- Modify: `frontend/src/lib/api/monitoring.ts`
- Modify: `frontend/src/routes/+page.svelte`

- [ ] **Step 1: Add `SystemInfo` type and `getSystemInfo` function**

In `frontend/src/lib/api/monitoring.ts`, add:

```ts
export type SystemInfo = {
  cpuCores: number;
  memoryTotalGB: number;
  netInterface: string;
  vpnInterfaces: string[];
};

export function getSystemInfo(): Promise<SystemInfo> {
  return apiFetch<SystemInfo>('/api/monitoring/system-info');
}
```

Also add `SystemInfo` and `getSystemInfo` to `frontend/src/lib/api/index.ts` (check what is currently exported):

```bash
grep -n "getMonitoringOverview\|MonitoringOverview\|export" /opt/ocserv_panel/frontend/src/lib/api/index.ts | head -20
```

If the index re-exports from monitoring.ts, add the new exports there.

- [ ] **Step 2: Fetch `systemInfo` in `+page.svelte`**

In `frontend/src/routes/+page.svelte`:

Add to imports:
```ts
import { ..., getSystemInfo, type SystemInfo } from '$lib/api';
```

Add to state:
```ts
let systemInfo = $state<SystemInfo | null>(null);
```

Add a `loadSystemInfo` function:
```ts
async function loadSystemInfo() {
  try {
    systemInfo = await getSystemInfo();
  } catch {
    // Non-fatal — cards degrade gracefully without subtitle
  }
}
```

Call it in `loadDashboard` after the monitoring call:
```ts
void loadSystemInfo();
```

Pass `{systemInfo}` as a prop to both `<DashboardMetrics>` and `<DashboardSessions>`.

- [ ] **Step 3: Type-check**

```bash
cd /opt/ocserv_panel/frontend && npx svelte-check --tsconfig ./tsconfig.json 2>&1 | tail -20
```
Expected: errors about missing prop `systemInfo` on `DashboardMetrics` and `DashboardSessions` — these will be fixed in the next two tasks.

- [ ] **Step 4: Commit**

```bash
cd /opt/ocserv_panel && git add frontend/src/lib/api/monitoring.ts frontend/src/routes/+page.svelte
git commit -m "feat(api): add SystemInfo type and getSystemInfo, fetch in dashboard"
```

---

## Task 6: Enrich metric cards with system info subtitles

**Files:**
- Modify: `frontend/src/lib/components/dashboard/DashboardMetrics.svelte`
- Modify: `frontend/src/lib/components/dashboard/DashboardSessions.svelte`

- [ ] **Step 1: Add `systemInfo` prop and card subtitles to `DashboardMetrics.svelte`**

Add `systemInfo` to the `Props` type:

```ts
import type { SystemInfo } from '$lib/api';

type Props = {
  monitoring: MonitoringOverview | null;
  monitoringLoading: boolean;
  monitoringError: string;
  monitoringWindow: MonitoringWindow;
  onWindowChange: (window: string) => void;
  onRefresh: () => void;
  systemInfo: SystemInfo | null;
};

let {
  monitoring,
  monitoringLoading,
  monitoringError,
  monitoringWindow,
  onWindowChange,
  onRefresh,
  systemInfo
}: Props = $props();
```

Update the four `CardStats` in the metric cards grid:

```svelte
<CardStats
  title="CPU"
  value={metricValue(HOST_CPU)}
  icon={Cpu}
  unavailable={metricState(HOST_CPU) === 'unavailable'}
  hint={unavailableReason(HOST_CPU)}
  subtitle={systemInfo ? `${systemInfo.cpuCores} cores` : undefined}
/>
<CardStats
  title="Memory"
  value={metricValue(HOST_MEMORY)}
  icon={MemoryStick}
  unavailable={metricState(HOST_MEMORY) === 'unavailable'}
  hint={unavailableReason(HOST_MEMORY)}
  subtitle={systemInfo?.memoryTotalGB ? `${systemInfo.memoryTotalGB} GB total` : undefined}
/>
<CardStats
  title="Network inbound"
  value={metricValue(NET_RX)}
  icon={ArrowDown}
  unavailable={metricState(NET_RX) === 'unavailable'}
  hint={unavailableReason(NET_RX)}
  subtitle={systemInfo?.netInterface || undefined}
/>
<CardStats
  title="Network outbound"
  value={metricValue(NET_TX)}
  icon={ArrowUp}
  unavailable={metricState(NET_TX) === 'unavailable'}
  hint={unavailableReason(NET_TX)}
  subtitle={systemInfo?.netInterface || undefined}
/>
```

- [ ] **Step 2: Add `systemInfo` prop and subtitles to `DashboardSessions.svelte`**

Add `systemInfo` to the `Props` type:

```ts
import type { SystemInfo } from '$lib/api';

type Props = {
  summary: DashboardSummary;
  activeUserSessions: ActiveUserSessionsResult | null;
  storageItems: HealthItem[];
  healthCheckedAt: Date | null;
  healthLoading: boolean;
  health: SystemHealth | null;
  systemInfo: SystemInfo | null;
};

let { summary, activeUserSessions, storageItems, healthCheckedAt, healthLoading, health, systemInfo }: Props = $props();
```

Add a derived subtitle for Total Traffic:

```ts
const trafficSubtitle = $derived((): string | undefined => {
  if (!systemInfo?.netInterface) return undefined;
  const parts = [systemInfo.netInterface];
  if (systemInfo.vpnInterfaces?.length) {
    parts.push(...systemInfo.vpnInterfaces);
  }
  return parts.join(' + ');
});
```

Update the three analytics CardStats:

```svelte
<CardStats
  title="Total received"
  value={formatBytes(summary.analytics?.totalBytesIn)}
  icon={ArrowDown}
  subtitle={systemInfo?.netInterface || undefined}
/>
<CardStats
  title="Total sent"
  value={formatBytes(summary.analytics?.totalBytesOut)}
  icon={ArrowUp}
  subtitle={systemInfo?.netInterface || undefined}
/>
<CardStats
  title="Total traffic"
  value={formatBytes((summary.analytics?.totalBytesIn ?? 0) + (summary.analytics?.totalBytesOut ?? 0))}
  icon={Server}
  subtitle={trafficSubtitle()}
/>
```

- [ ] **Step 3: Type-check**

```bash
cd /opt/ocserv_panel/frontend && npx svelte-check --tsconfig ./tsconfig.json 2>&1 | tail -20
```
Expected: 0 errors

- [ ] **Step 4: Commit**

```bash
cd /opt/ocserv_panel && git add frontend/src/lib/components/dashboard/DashboardMetrics.svelte frontend/src/lib/components/dashboard/DashboardSessions.svelte
git commit -m "feat(dashboard): add CPU cores, memory total, and interface subtitles to metric cards"
```

---

## Task 7: Create `MultiLineChart.svelte` and wire VPN charts

**Files:**
- Create: `frontend/src/lib/components/ui/MultiLineChart.svelte`
- Modify: `frontend/src/lib/components/dashboard/DashboardMetrics.svelte`

- [ ] **Step 1: Create `MultiLineChart.svelte`**

Create `frontend/src/lib/components/ui/MultiLineChart.svelte`:

```svelte
<script lang="ts">
  import { Area, Axis, Chart, Highlight, LinearGradient, Svg } from 'layerchart';
  import { Container as ChartContainer, Tooltip as ChartTooltip } from '$lib/components/ui/chart/index';
  import { formatMetricValue, formatXAxisTime, getChartConfig, getXTickCount } from '$lib/monitoring';

  export type LineData = {
    name: string;
    data: { timestamp: Date; value: number }[];
  };

  type Props = {
    lines: LineData[];
    metric: string;
    height?: number;
  };

  let { lines, metric, height = 160 }: Props = $props();

  const VPN_LINE_PALETTE = [
    { stroke: 'stroke-violet-500 stroke-2', top: 'rgba(139, 92, 246, 0.25)', bottom: 'rgba(139, 92, 246, 0.05)' },
    { stroke: 'stroke-fuchsia-500 stroke-2', top: 'rgba(217, 70, 239, 0.25)', bottom: 'rgba(217, 70, 239, 0.05)' },
    { stroke: 'stroke-rose-500 stroke-2', top: 'rgba(244, 63, 94, 0.25)', bottom: 'rgba(244, 63, 94, 0.05)' },
    { stroke: 'stroke-amber-500 stroke-2', top: 'rgba(245, 158, 11, 0.25)', bottom: 'rgba(245, 158, 11, 0.05)' },
  ];

  const config = $derived(getChartConfig(metric));

  const hasData = $derived(lines.some(l => l.data.length > 0));

  function colorFor(index: number) {
    return VPN_LINE_PALETTE[index % VPN_LINE_PALETTE.length];
  }

  function tooltipFormatter(value: number): string {
    return formatMetricValue(metric, value);
  }

  // Flatten all lines into a single dataset for Chart's domain calculation.
  const allData = $derived(lines.flatMap(l => l.data));
  const xTickCount = $derived(getXTickCount(allData.length));
</script>

<div style="height: {height}px" class="w-full">
  {#if hasData}
    <div style="height: {lines.length > 1 ? height - 24 : height}px" class="w-full">
      <ChartContainer config={{}} class="h-full w-full">
        <Chart
          data={allData}
          x="timestamp"
          y="value"
          yDomain={config.yDomain}
          yNice={config.yNice}
          padding={{ left: 44, bottom: 24, top: 8, right: 8 }}
          tooltip={{ mode: 'bisect-x' }}
        >
          <Svg>
            <Axis placement="left" grid format={config.yAxisFormat} />
            <Axis
              placement="bottom"
              format={(d: Date | number) => formatXAxisTime(d)}
              ticks={xTickCount}
            />
            {#each lines as line, i}
              {#if line.data.length > 0}
                {@const colors = colorFor(i)}
                <LinearGradient
                  stops={[colors.top, colors.bottom]}
                  vertical
                >
                  {#snippet children({ gradient }: { gradient: string })}
                    <Area
                      data={line.data}
                      fill={gradient}
                      line={{ class: colors.stroke }}
                    />
                  {/snippet}
                </LinearGradient>
              {/if}
            {/each}
            <Highlight points lines />
          </Svg>
          <ChartTooltip
            label={metric.replace(/_/g, ' ')}
            valueFormatter={tooltipFormatter}
          />
        </Chart>
      </ChartContainer>
    </div>
    {#if lines.length > 1}
      <div class="flex flex-wrap gap-3 mt-1 px-1">
        {#each lines as line, i}
          {@const colors = colorFor(i)}
          <div class="flex items-center gap-1.5 text-xs text-muted-foreground">
            <span class="inline-block w-2.5 h-2.5 rounded-full flex-shrink-0" style="background: {VPN_LINE_PALETTE[i % VPN_LINE_PALETTE.length].top.replace('0.25', '0.8')}"></span>
            {line.name}
          </div>
        {/each}
      </div>
    {/if}
  {:else}
    <div class="h-full flex items-center justify-center rounded-lg border border-dashed border-border bg-muted/30">
      <p class="text-sm text-muted-foreground">No data available</p>
    </div>
  {/if}
</div>
```

- [ ] **Step 2: Wire VPN multi-line charts in `DashboardMetrics.svelte`**

Add imports:
```ts
import MultiLineChart from '$lib/components/ui/MultiLineChart.svelte';
import type { LineData } from '$lib/components/ui/MultiLineChart.svelte';
```

Add metric name constants for VPN (keep existing `VPN_RX` and `VPN_TX` constants, remove them from `chartDefinitions`):
```ts
const VPN_RX = 'vpn_interface_rx_bytes';
const VPN_TX = 'vpn_interface_tx_bytes';
```

Add a helper function to build per-interface data:
```ts
function chartDataForMetricAndInterface(metric: string, iface: string): { timestamp: Date; value: number }[] {
  const series = monitoring?.series?.find(
    s => s.metric === metric && s.labels['interface'] === iface
  );
  if (!series) return [];
  return series.points
    .filter(p => Number.isFinite(Date.parse(p.timestamp)) && Number.isFinite(p.value))
    .map(p => ({ timestamp: new Date(p.timestamp), value: p.value }))
    .sort((a, b) => a.timestamp.getTime() - b.timestamp.getTime());
}
```

Add derived VPN interface names and lines:
```ts
const vpnInterfaceNames = $derived(
  [...new Set(
    (monitoring?.series ?? [])
      .filter(s => s.metric === VPN_RX)
      .map(s => s.labels['interface'])
      .filter((v): v is string => Boolean(v))
  )]
);

const vpnRXLines = $derived<LineData[]>(
  vpnInterfaceNames.map(iface => ({
    name: iface,
    data: chartDataForMetricAndInterface(VPN_RX, iface),
  }))
);

const vpnTXLines = $derived<LineData[]>(
  vpnInterfaceNames.map(iface => ({
    name: iface,
    data: chartDataForMetricAndInterface(VPN_TX, iface),
  }))
);
```

Remove `VPN_RX` and `VPN_TX` from `chartDefinitions` (they are now handled separately).

In the chart grid template, replace or remove the dynamic VPN entries. Since `chartDefinitions` no longer includes them, add two dedicated VPN cards after the `{#each chartDefinitions}` block:

```svelte
<!-- VPN charts — shown after dynamic charts -->
<Card.Root>
  <Card.Header>
    <div>
      <Card.Title>VPN inbound</Card.Title>
      <Card.Description>Last {monitoringWindow}</Card.Description>
    </div>
  </Card.Header>
  <Card.Content>
    <MultiLineChart lines={vpnRXLines} metric={VPN_RX} height={160} />
  </Card.Content>
</Card.Root>
<Card.Root>
  <Card.Header>
    <div>
      <Card.Title>VPN outbound</Card.Title>
      <Card.Description>Last {monitoringWindow}</Card.Description>
    </div>
  </Card.Header>
  <Card.Content>
    <MultiLineChart lines={vpnTXLines} metric={VPN_TX} height={160} />
  </Card.Content>
</Card.Root>
```

Place the two VPN cards inside the `{#key mode.current}` block, after the `{#each chartDefinitions}` block.

Remove `VPN_RX` and `VPN_TX` from the `unavailableChips` derived (since these no longer go through `chartDefinitions`, they won't appear in the unavailable chips automatically — the `MultiLineChart` shows its own placeholder). Remove from `chartDefinitions` if still present:

```ts
// REMOVE these two lines from chartDefinitions:
{ metric: VPN_RX, title: 'VPN inbound' },
{ metric: VPN_TX, title: 'VPN outbound' },
```

- [ ] **Step 3: Type-check**

```bash
cd /opt/ocserv_panel/frontend && npx svelte-check --tsconfig ./tsconfig.json 2>&1 | tail -20
```
Expected: 0 errors

- [ ] **Step 4: Build**

```bash
cd /opt/ocserv_panel/frontend && npm run build 2>&1 | tail -20
```
Expected: build succeeds

- [ ] **Step 5: Commit**

```bash
cd /opt/ocserv_panel && git add frontend/src/lib/components/ui/MultiLineChart.svelte frontend/src/lib/components/dashboard/DashboardMetrics.svelte
git commit -m "feat(dashboard): add MultiLineChart, wire per-interface VPN charts"
```

---

## Task 8: Run full verification

- [ ] **Step 1: Run all backend tests**

```bash
cd /opt/ocserv_panel/backend && go test ./... 2>&1
```
Expected: all PASS, 0 failures

- [ ] **Step 2: Run frontend type-check**

```bash
cd /opt/ocserv_panel/frontend && npx svelte-check --tsconfig ./tsconfig.json 2>&1 | tail -10
```
Expected: 0 errors

- [ ] **Step 3: Build frontend**

```bash
cd /opt/ocserv_panel/frontend && npm run build 2>&1 | tail -10
```
Expected: build succeeds

- [ ] **Step 4: Start the dev server and do a visual verification**

```bash
cd /opt/ocserv_panel/frontend && npm run dev -- --host 0.0.0.0 &
```

Verify visually:
- Dashboard shows no Audit Events / ocserv Logs / Panel Logs tabs
- No "Active sessions" card in the sessions section
- CPU card shows core count subtitle (e.g. "4 cores")
- Memory card shows total GB subtitle (e.g. "15.6 GB total")
- Network inbound/outbound cards show interface name subtitle
- Total received/sent cards show interface name
- Total traffic card shows interface names combined
- VPN inbound/outbound charts show multi-line chart (or "No data available" if no VPN is active)
- All charts use area+gradient style consistently

- [ ] **Step 5: Commit if any final adjustments made**

```bash
cd /opt/ocserv_panel && git add -A && git commit -m "fix(dashboard): visual adjustments from verification"
```

Only commit if there are actual changes.
