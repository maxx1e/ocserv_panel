# Dashboard Improvements Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix dashboard data accuracy (rate-based network metrics, session collection, VPN discovery), add donut chart for active sessions by user, fix analytics card labels, and make monitoring settings consistent with the settings page.

**Architecture:** Backend monitoring collectors are converted to rate-based (bytes/sec). A new SessionCollector uses the existing `ocserv.Manager.GetServerStatus()` via the occtl adapter. VPN discovery adds `vpns*` prefix support and periodic re-discovery. Frontend adds a donut chart component, fixes analytics cards, and uses the configurable refresh interval.

**Tech Stack:** Go backend (monitoring package, config, server), SvelteKit frontend (LayerChart Pie/Arc), SQLite settings, TSDB metrics store.

---

### Task 1: Rate-Based InterfaceCollector

**Files:**
- Modify: `backend/internal/monitoring/collectors.go`
- Modify: `backend/internal/monitoring/collectors_test.go`

- [ ] **Step 1: Write failing test for rate-based collector**

Add to `backend/internal/monitoring/collectors_test.go`:

```go
func TestInterfaceCollector_RateBasedOutput(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 9, 12, 0, 0, 0, time.UTC)
	reader := &fakeFileReader{
		files: map[string][]string{
			"/sys/class/net/eth0/statistics/rx_bytes": {"1000\n", "2000\n"},
			"/sys/class/net/eth0/statistics/tx_bytes": {"5000\n", "8000\n"},
		},
	}
	collector := NewInterfaceCollector(reader, "eth0", metrics.MetricNetInterfaceRXBytes, metrics.MetricNetInterfaceTXBytes)

	// First call stores baseline — returns unavailable
	first := collector.Collect(context.Background(), now)
	if first.Status != CollectStatusUnavailable {
		t.Fatalf("first status = %q, want %q", first.Status, CollectStatusUnavailable)
	}

	// Second call computes rate: (2000-1000)/15 = 66.67 rx, (8000-5000)/15 = 200 tx
	second := collector.Collect(context.Background(), now.Add(15*time.Second))
	if second.Status != CollectStatusAvailable {
		t.Fatalf("second status = %q, want %q; message = %q", second.Status, CollectStatusAvailable, second.Message)
	}
	if len(second.Samples) != 2 {
		t.Fatalf("samples length = %d, want 2", len(second.Samples))
	}

	rxRate := second.Samples[0].Value
	txRate := second.Samples[1].Value
	// (2000-1000) / 15s = 66.666...
	if rxRate < 66.6 || rxRate > 66.7 {
		t.Fatalf("rx rate = %v, want ~66.67", rxRate)
	}
	// (8000-5000) / 15s = 200
	if txRate != 200.0 {
		t.Fatalf("tx rate = %v, want 200.0", txRate)
	}
}

func TestInterfaceCollector_CounterWrapReturnsUnavailable(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 9, 12, 0, 0, 0, time.UTC)
	reader := &fakeFileReader{
		files: map[string][]string{
			"/sys/class/net/eth0/statistics/rx_bytes": {"5000\n", "1000\n"},
			"/sys/class/net/eth0/statistics/tx_bytes": {"8000\n", "2000\n"},
		},
	}
	collector := NewInterfaceCollector(reader, "eth0", metrics.MetricNetInterfaceRXBytes, metrics.MetricNetInterfaceTXBytes)

	_ = collector.Collect(context.Background(), now) // baseline
	second := collector.Collect(context.Background(), now.Add(15*time.Second))
	if second.Status != CollectStatusUnavailable {
		t.Fatalf("status = %q, want %q on counter wrap", second.Status, CollectStatusUnavailable)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /opt/ocserv_panel/backend && go test ./internal/monitoring/ -run "TestInterfaceCollector_RateBasedOutput|TestInterfaceCollector_CounterWrapReturnsUnavailable" -v`

Expected: FAIL — first test expects unavailable on first call but current code returns available with raw counters.

- [ ] **Step 3: Implement rate-based InterfaceCollector**

Replace the `InterfaceCollector` struct and `Collect` method in `backend/internal/monitoring/collectors.go`:

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

var _ Collector = (*InterfaceCollector)(nil)

func NewInterfaceCollector(
	reader FileReader,
	iface string,
	rxMetric metrics.MetricName,
	txMetric metrics.MetricName,
) *InterfaceCollector {
	return &InterfaceCollector{
		reader:   reader,
		iface:    iface,
		rxMetric: rxMetric,
		txMetric: txMetric,
	}
}

func (c *InterfaceCollector) Collect(ctx context.Context, now time.Time) CollectResult {
	if err := ctx.Err(); err != nil {
		return unavailable(err.Error())
	}
	if c.iface == "" {
		return unavailable("interface name is required")
	}

	rxBytes, err := c.readCounter(ctx, "rx_bytes")
	if err != nil {
		return unavailable(err.Error())
	}

	txBytes, err := c.readCounter(ctx, "tx_bytes")
	if err != nil {
		return unavailable(err.Error())
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.hasPrev {
		c.prevRX = rxBytes
		c.prevTX = txBytes
		c.prevTime = now
		c.hasPrev = true
		return unavailable("rate calculation requires a previous sample")
	}

	elapsed := now.Sub(c.prevTime).Seconds()
	if elapsed <= 0 {
		return unavailable("no time elapsed between samples")
	}

	// Detect counter wrap (reboot or overflow) — skip this sample.
	if rxBytes < c.prevRX || txBytes < c.prevTX {
		c.prevRX = rxBytes
		c.prevTX = txBytes
		c.prevTime = now
		return unavailable("counter wrap detected, resetting baseline")
	}

	rxRate := float64(rxBytes-c.prevRX) / elapsed
	txRate := float64(txBytes-c.prevTX) / elapsed

	c.prevRX = rxBytes
	c.prevTX = txBytes
	c.prevTime = now

	labels := map[string]string{"interface": c.iface}

	return available([]metrics.Sample{
		{
			Name:      c.rxMetric,
			Labels:    labels,
			Timestamp: now,
			Value:     rxRate,
		},
		{
			Name:      c.txMetric,
			Labels:    labels,
			Timestamp: now,
			Value:     txRate,
		},
	})
}
```

- [ ] **Step 4: Update existing InterfaceCollector tests**

The existing `TestInterfaceCollector` test expects raw values on first call. Update it:

```go
func TestInterfaceCollector(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 30, 12, 0, 0, 0, time.UTC)
	collector := NewInterfaceCollector(
		&fakeFileReader{
			files: map[string][]string{
				"/sys/class/net/eth0/statistics/rx_bytes": {"123\n", "223\n"},
				"/sys/class/net/eth0/statistics/tx_bytes": {"456\n", "556\n"},
			},
		},
		"eth0",
		metrics.MetricNetInterfaceRXBytes,
		metrics.MetricNetInterfaceTXBytes,
	)

	// First call: baseline stored, returns unavailable
	first := collector.Collect(context.Background(), now)
	if first.Status != CollectStatusUnavailable {
		t.Fatalf("first status = %q, want %q", first.Status, CollectStatusUnavailable)
	}

	// Second call: rate = (223-123)/10 = 10, (556-456)/10 = 10
	second := collector.Collect(context.Background(), now.Add(10*time.Second))
	if second.Status != CollectStatusAvailable {
		t.Fatalf("second status = %q, want %q; message = %q", second.Status, CollectStatusAvailable, second.Message)
	}
	if len(second.Samples) != 2 {
		t.Fatalf("samples length = %d, want 2", len(second.Samples))
	}

	assertInterfaceSample(t, second.Samples[0], metrics.MetricNetInterfaceRXBytes, now.Add(10*time.Second), 10.0, "eth0")
	assertInterfaceSample(t, second.Samples[1], metrics.MetricNetInterfaceTXBytes, now.Add(10*time.Second), 10.0, "eth0")
}
```

Also update `TestInterfaceCollector_ContextCanceledAfterRXSkipsTX` — it remains valid since it tests context cancellation during read, which still applies.

- [ ] **Step 5: Run all monitoring tests**

Run: `cd /opt/ocserv_panel/backend && go test ./internal/monitoring/ -v`

Expected: All PASS

- [ ] **Step 6: Commit**

```bash
git add backend/internal/monitoring/collectors.go backend/internal/monitoring/collectors_test.go
git commit -m "feat(monitoring): convert InterfaceCollector to rate-based bytes/sec"
```

---

### Task 2: VPN Discovery Fix — Add vpns Prefix

**Files:**
- Modify: `backend/internal/monitoring/vpn_discovery.go`
- Modify: `backend/internal/monitoring/vpn_discovery_test.go`

- [ ] **Step 1: Write failing test for vpns prefix discovery**

Add to `backend/internal/monitoring/vpn_discovery_test.go`:

```go
func TestDiscoverVPNInterfaceVpnsPrefixFromSysFS(t *testing.T) {
	t.Parallel()

	iface, source, err := DiscoverVPNInterface(
		WithOcservConfigPath("/nonexistent"),
		WithSysNetPath("/fake"),
		WithReadDirFunc(func(dirname string) ([]os.DirEntry, error) {
			return []os.DirEntry{
				fakeDirEntry("lo"),
				fakeDirEntry("eth0"),
				fakeDirEntry("vpns0"),
				fakeDirEntry("vpns1"),
			}, nil
		}),
	)
	if err != nil {
		t.Fatalf("DiscoverVPNInterface: %v", err)
	}
	if iface != "vpns0" {
		t.Fatalf("iface = %q, want %q", iface, "vpns0")
	}
	if source != "sysfs-scan" {
		t.Fatalf("source = %q, want %q", source, "sysfs-scan")
	}
}

func TestDiscoverVPNInterfaceConfigDeviceBaseNameResolvesToNumbered(t *testing.T) {
	t.Parallel()

	configData := "device = vpns\n"
	iface, source, err := DiscoverVPNInterface(
		WithFileReader(&fakeFileReader{
			files: map[string][]string{"/etc/ocserv/ocserv.conf": {configData}},
		}),
		WithSysNetPath("/fake"),
		WithReadDirFunc(func(dirname string) ([]os.DirEntry, error) {
			return []os.DirEntry{
				fakeDirEntry("lo"),
				fakeDirEntry("eth0"),
				fakeDirEntry("vpns0"),
				fakeDirEntry("vpns1"),
			}, nil
		}),
	)
	if err != nil {
		t.Fatalf("DiscoverVPNInterface: %v", err)
	}
	if iface != "vpns0" {
		t.Fatalf("iface = %q, want %q (config said 'vpns', should resolve to first numbered instance)", iface, "vpns0")
	}
	if source != "ocserv-config" {
		t.Fatalf("source = %q, want %q", source, "ocserv-config")
	}
}
```

Note: `fakeDirEntry` is likely already defined in the test file. If not, add:

```go
type fakeDirEntry struct {
	name string
}

func (f fakeDirEntry) Name() string               { return f.name }
func (f fakeDirEntry) IsDir() bool                 { return false }
func (f fakeDirEntry) Type() os.FileMode           { return 0 }
func (f fakeDirEntry) Info() (os.FileInfo, error)  { return nil, nil }
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /opt/ocserv_panel/backend && go test ./internal/monitoring/ -run "TestDiscoverVPNInterfaceVpnsPrefix|TestDiscoverVPNInterfaceConfigDeviceBaseName" -v`

Expected: FAIL — `vpns` not in prefix list, config device base name not resolved.

- [ ] **Step 3: Fix vpn_discovery.go**

In `discoverFromSysFS()`, update the prefix list:

```go
vpnPrefixes := []string{"vpns", "tun", "tap", "wg"}
```

In `DiscoverVPNInterface()`, after getting the config device name, check if it exists as-is in sysfs. If not, scan for numbered instances:

```go
func DiscoverVPNInterface(opts ...VPNDiscoveryOption) (string, string, error) {
	cfg := vpnDiscoveryConfig{
		reader:     OSReader{},
		configPath: "/etc/ocserv/ocserv.conf",
		sysNetPath: "/sys/class/net",
		readDir:    os.ReadDir,
	}
	for _, opt := range opts {
		opt(&cfg)
	}

	// Strategy 1: Parse ocserv config for the "device" directive.
	iface, err := discoverFromOcservConfig(cfg)
	if err == nil && iface != "" {
		// The config may specify a base name like "vpns" but the kernel creates
		// numbered instances like "vpns0". Resolve by scanning sysfs.
		resolved := resolveInterfaceName(cfg, iface)
		if resolved != "" {
			return resolved, "ocserv-config", nil
		}
	}

	// Strategy 2: Scan /sys/class/net/ for VPN interface patterns.
	iface, err = discoverFromSysFS(cfg)
	if err == nil && iface != "" {
		return iface, "sysfs-scan", nil
	}

	// No VPN interface found.
	return "", "", nil
}

// resolveInterfaceName checks if the given interface name exists as-is in sysfs.
// If not, it scans for interfaces starting with that name (e.g., "vpns" -> "vpns0").
// Returns the resolved name or empty string if not found.
func resolveInterfaceName(cfg vpnDiscoveryConfig, baseName string) string {
	entries, err := cfg.readDir(cfg.sysNetPath)
	if err != nil {
		return ""
	}

	// Check if exact name exists
	for _, entry := range entries {
		if entry.Name() == baseName {
			return baseName
		}
	}

	// Scan for numbered instances (e.g., vpns0, vpns1)
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), baseName) && len(entry.Name()) > len(baseName) {
			return entry.Name()
		}
	}

	return ""
}
```

Add `"strings"` to imports if not already present.

- [ ] **Step 4: Run all VPN discovery tests**

Run: `cd /opt/ocserv_panel/backend && go test ./internal/monitoring/ -run "TestDiscoverVPN" -v`

Expected: All PASS

- [ ] **Step 5: Commit**

```bash
git add backend/internal/monitoring/vpn_discovery.go backend/internal/monitoring/vpn_discovery_test.go
git commit -m "feat(monitoring): add vpns prefix to VPN discovery and resolve numbered instances"
```

---

### Task 3: Session Collector

**Files:**
- Create: `backend/internal/monitoring/session_collector.go`
- Create: `backend/internal/monitoring/session_collector_test.go`

- [ ] **Step 1: Write failing test**

Create `backend/internal/monitoring/session_collector_test.go`:

```go
package monitoring

import (
	"context"
	"testing"
	"time"

	"ocserv_panel/backend/internal/metrics"
	"ocserv_panel/backend/internal/ocserv"
)

func TestSessionCollector_CollectsActiveSessions(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 9, 12, 0, 0, 0, time.UTC)
	manager := ocserv.NewFakeManager()
	manager.ServerStatusResult = &ocserv.ServerStatus{
		Status:         "online",
		ActiveSessions: 5,
		TotalSessions:  42,
	}

	collector := NewSessionCollector(manager)
	result := collector.Collect(context.Background(), now)

	if result.Status != CollectStatusAvailable {
		t.Fatalf("status = %q, want %q; message = %q", result.Status, CollectStatusAvailable, result.Message)
	}
	if len(result.Samples) != 2 {
		t.Fatalf("samples length = %d, want 2", len(result.Samples))
	}

	// Check active sessions sample
	active := result.Samples[0]
	if active.Name != metrics.MetricActiveSessions {
		t.Fatalf("sample[0] name = %q, want %q", active.Name, metrics.MetricActiveSessions)
	}
	if active.Value != 5 {
		t.Fatalf("sample[0] value = %v, want 5", active.Value)
	}
	if active.Timestamp != now {
		t.Fatalf("sample[0] timestamp = %v, want %v", active.Timestamp, now)
	}

	// Check concurrent sessions sample
	concurrent := result.Samples[1]
	if concurrent.Name != metrics.MetricConcurrentSessions {
		t.Fatalf("sample[1] name = %q, want %q", concurrent.Name, metrics.MetricConcurrentSessions)
	}
	if concurrent.Value != 5 {
		t.Fatalf("sample[1] value = %v, want 5", concurrent.Value)
	}
}

func TestSessionCollector_UnavailableWhenError(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 9, 12, 0, 0, 0, time.UTC)
	manager := ocserv.NewFakeManager()
	manager.ServerStatusErr = context.DeadlineExceeded

	collector := NewSessionCollector(manager)
	result := collector.Collect(context.Background(), now)

	if result.Status != CollectStatusUnavailable {
		t.Fatalf("status = %q, want %q", result.Status, CollectStatusUnavailable)
	}
	if len(result.Samples) != 0 {
		t.Fatalf("samples length = %d, want 0", len(result.Samples))
	}
}

func TestSessionCollector_UnavailableWhenNilStatus(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 9, 12, 0, 0, 0, time.UTC)
	manager := ocserv.NewFakeManager()
	manager.ServerStatusResult = nil

	collector := NewSessionCollector(manager)
	result := collector.Collect(context.Background(), now)

	if result.Status != CollectStatusUnavailable {
		t.Fatalf("status = %q, want %q", result.Status, CollectStatusUnavailable)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /opt/ocserv_panel/backend && go test ./internal/monitoring/ -run "TestSessionCollector" -v`

Expected: FAIL — `NewSessionCollector` not defined.

- [ ] **Step 3: Implement SessionCollector**

Create `backend/internal/monitoring/session_collector.go`:

```go
package monitoring

import (
	"context"
	"time"

	"ocserv_panel/backend/internal/metrics"
	"ocserv_panel/backend/internal/ocserv"
)

// SessionStatusProvider is the subset of ocserv.Manager needed by SessionCollector.
type SessionStatusProvider interface {
	GetServerStatus(ctx context.Context) (*ocserv.ServerStatus, error)
}

// SessionCollector collects active session counts from ocserv via occtl.
type SessionCollector struct {
	provider SessionStatusProvider
}

var _ Collector = (*SessionCollector)(nil)

// NewSessionCollector returns a collector that queries ocserv for session metrics.
func NewSessionCollector(provider SessionStatusProvider) *SessionCollector {
	return &SessionCollector{provider: provider}
}

func (c *SessionCollector) Collect(ctx context.Context, now time.Time) CollectResult {
	if err := ctx.Err(); err != nil {
		return unavailable(err.Error())
	}

	status, err := c.provider.GetServerStatus(ctx)
	if err != nil {
		return unavailable("occtl status unavailable: " + err.Error())
	}
	if status == nil {
		return unavailable("occtl returned nil status")
	}

	return available([]metrics.Sample{
		{
			Name:      metrics.MetricActiveSessions,
			Labels:    map[string]string{},
			Timestamp: now,
			Value:     float64(status.ActiveSessions),
		},
		{
			Name:      metrics.MetricConcurrentSessions,
			Labels:    map[string]string{},
			Timestamp: now,
			Value:     float64(status.ActiveSessions),
		},
	})
}
```

- [ ] **Step 4: Run tests**

Run: `cd /opt/ocserv_panel/backend && go test ./internal/monitoring/ -run "TestSessionCollector" -v`

Expected: All PASS

- [ ] **Step 5: Commit**

```bash
git add backend/internal/monitoring/session_collector.go backend/internal/monitoring/session_collector_test.go
git commit -m "feat(monitoring): add SessionCollector for active/concurrent session metrics"
```

---

### Task 4: Collector Service — Periodic VPN Re-Discovery

**Files:**
- Modify: `backend/internal/monitoring/collector_service.go`
- Modify: `backend/internal/monitoring/collector_service_test.go`

- [ ] **Step 1: Write failing test**

Add to `backend/internal/monitoring/collector_service_test.go`:

```go
func TestCollectorService_VPNRediscovery(t *testing.T) {
	t.Parallel()

	store := metrics.NewMemoryStore()
	defer store.Close()

	callCount := 0
	discoverer := func() (string, string, error) {
		callCount++
		if callCount < 3 {
			return "", "", nil // not found yet
		}
		return "vpns0", "sysfs-scan", nil // found on 3rd call
	}

	svc := NewCollectorService(store, nil, 100*time.Millisecond, nil, nil)
	svc.SetVPNDiscoverer(discoverer, OSReader{}, metrics.MetricVPNInterfaceRXBytes, metrics.MetricVPNInterfaceTXBytes)

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	svc.Run(ctx)

	if !svc.HasVPNCollector() {
		t.Fatal("expected VPN collector to be added after discovery")
	}
}

func TestCollectorService_VPNRediscoverySkipsWhenAlreadyPresent(t *testing.T) {
	t.Parallel()

	store := metrics.NewMemoryStore()
	defer store.Close()

	callCount := 0
	discoverer := func() (string, string, error) {
		callCount++
		return "vpns0", "sysfs-scan", nil
	}

	svc := NewCollectorService(store, nil, 100*time.Millisecond, nil, nil)
	svc.SetVPNDiscoverer(discoverer, OSReader{}, metrics.MetricVPNInterfaceRXBytes, metrics.MetricVPNInterfaceTXBytes)

	ctx, cancel := context.WithTimeout(context.Background(), 400*time.Millisecond)
	defer cancel()

	svc.Run(ctx)

	// Discoverer should have been called once (found immediately) then never again.
	if callCount != 1 {
		t.Fatalf("discoverer called %d times, want 1 (should stop after finding VPN)", callCount)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /opt/ocserv_panel/backend && go test ./internal/monitoring/ -run "TestCollectorService_VPN" -v`

Expected: FAIL — `SetVPNDiscoverer` and `HasVPNCollector` not defined.

- [ ] **Step 3: Implement VPN re-discovery in collector service**

Modify `backend/internal/monitoring/collector_service.go`:

```go
package monitoring

import (
	"context"
	"sync"
	"time"

	"ocserv_panel/backend/internal/metrics"
)

const (
	defaultCollectorInterval    = 15 * time.Second
	vpnRediscoveryInterval      = 60 * time.Second
)

// VPNDiscovererFunc attempts to discover a VPN interface.
// Returns (interfaceName, source, error). Empty name = not found.
type VPNDiscovererFunc func() (string, string, error)

type Logger interface {
	Debug(msg string, args ...any)
	Warn(msg string, args ...any)
}

type CollectorService struct {
	store      metrics.Store
	collectors []Collector
	interval   time.Duration
	logger     Logger
	now        func() time.Time

	mu              sync.Mutex
	vpnDiscoverer   VPNDiscovererFunc
	vpnReader       FileReader
	vpnRXMetric     metrics.MetricName
	vpnTXMetric     metrics.MetricName
	hasVPN          bool
}

func NewCollectorService(
	store metrics.Store,
	collectors []Collector,
	interval time.Duration,
	logger Logger,
	now func() time.Time,
) *CollectorService {
	if interval <= 0 {
		interval = defaultCollectorInterval
	}
	if logger == nil {
		logger = noopLogger{}
	}
	if now == nil {
		now = time.Now
	}

	clonedCollectors := make([]Collector, 0, len(collectors))
	if collectors != nil {
		clonedCollectors = append(clonedCollectors, collectors...)
	}

	return &CollectorService{
		store:      store,
		collectors: clonedCollectors,
		interval:   interval,
		logger:     logger,
		now:        now,
	}
}

// SetVPNDiscoverer configures periodic VPN interface discovery.
// If no VPN collector is present, the discoverer is called periodically.
func (s *CollectorService) SetVPNDiscoverer(fn VPNDiscovererFunc, reader FileReader, rxMetric, txMetric metrics.MetricName) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.vpnDiscoverer = fn
	s.vpnReader = reader
	s.vpnRXMetric = rxMetric
	s.vpnTXMetric = txMetric
}

// HasVPNCollector returns whether a VPN collector has been dynamically added.
func (s *CollectorService) HasVPNCollector() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.hasVPN
}

// MarkVPNPresent marks that a VPN collector was added externally (e.g., at startup).
func (s *CollectorService) MarkVPNPresent() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.hasVPN = true
}

func (s *CollectorService) CollectOnce(ctx context.Context) {
	if err := ctx.Err(); err != nil {
		return
	}

	s.mu.Lock()
	collectors := make([]Collector, len(s.collectors))
	copy(collectors, s.collectors)
	s.mu.Unlock()

	for _, collector := range collectors {
		if err := ctx.Err(); err != nil {
			return
		}

		result := collector.Collect(ctx, s.now())
		if result.Status == CollectStatusUnavailable {
			s.logger.Debug("collector unavailable", "message", result.Message)
			continue
		}

		for _, sample := range result.Samples {
			if err := ctx.Err(); err != nil {
				return
			}
			if err := s.store.Append(ctx, []metrics.Sample{sample}); err != nil {
				s.logger.Warn("append metric sample failed", "error", err)
				continue
			}
		}
	}
}

func (s *CollectorService) Run(ctx context.Context) {
	if err := ctx.Err(); err != nil {
		return
	}

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	var vpnTicker *time.Ticker
	s.mu.Lock()
	hasDiscoverer := s.vpnDiscoverer != nil
	s.mu.Unlock()

	if hasDiscoverer {
		vpnTicker = time.NewTicker(vpnRediscoveryInterval)
		defer vpnTicker.Stop()
		// Try immediately
		s.maybeDiscoverVPN()
	}

	s.CollectOnce(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.CollectOnce(ctx)
		case <-vpnTickerChan(vpnTicker):
			s.maybeDiscoverVPN()
		}
	}
}

func (s *CollectorService) maybeDiscoverVPN() {
	s.mu.Lock()
	if s.hasVPN || s.vpnDiscoverer == nil {
		s.mu.Unlock()
		return
	}
	discoverer := s.vpnDiscoverer
	reader := s.vpnReader
	rxMetric := s.vpnRXMetric
	txMetric := s.vpnTXMetric
	s.mu.Unlock()

	iface, source, err := discoverer()
	if err != nil || iface == "" {
		return
	}

	s.logger.Debug("vpn interface discovered dynamically", "interface", iface, "source", source)

	collector := NewInterfaceCollector(reader, iface, rxMetric, txMetric)

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.hasVPN {
		return // race: another goroutine already added it
	}
	s.collectors = append(s.collectors, collector)
	s.hasVPN = true
}

func vpnTickerChan(t *time.Ticker) <-chan time.Time {
	if t == nil {
		return nil
	}
	return t.C
}

type noopLogger struct{}

func (noopLogger) Debug(string, ...any) {}

func (noopLogger) Warn(string, ...any) {}
```

- [ ] **Step 4: Run tests**

Run: `cd /opt/ocserv_panel/backend && go test ./internal/monitoring/ -v -timeout 10s`

Expected: All PASS

- [ ] **Step 5: Commit**

```bash
git add backend/internal/monitoring/collector_service.go backend/internal/monitoring/collector_service_test.go
git commit -m "feat(monitoring): add periodic VPN interface re-discovery to collector service"
```

---

### Task 5: Config & Settings — Monitoring Fields

**Files:**
- Modify: `backend/internal/config/config.go`
- Modify: `backend/internal/app/settings_registry.go`
- Modify: `backend/internal/app/settings_config_apply.go`
- Modify: `frontend/src/lib/settings/descriptions.ts`

- [ ] **Step 1: Add fields to MonitoringConfig**

In `backend/internal/config/config.go`, update the `MonitoringConfig` struct:

```go
// MonitoringConfig holds monitoring endpoint settings.
type MonitoringConfig struct {
	HealthEndpoint           bool   `toml:"health_endpoint"`
	MetricsEndpoint          bool   `toml:"metrics_endpoint"`
	DashboardRefreshInterval int    `toml:"dashboard_refresh_interval"`
	LogFileReadEnabled       bool   `toml:"log_file_read_enabled"`
	NetworkInterface         string `toml:"network_interface"`
	VPNInterface             string `toml:"vpn_interface"`
	CollectorInterval        int    `toml:"collector_interval"`
}
```

Update the `Defaults()` function's `Monitoring` block:

```go
Monitoring: MonitoringConfig{
	HealthEndpoint:           true,
	MetricsEndpoint:          false,
	DashboardRefreshInterval: 15,
	LogFileReadEnabled:       true,
	NetworkInterface:         "",
	VPNInterface:             "",
	CollectorInterval:        15,
},
```

- [ ] **Step 2: Add settings registry entries**

In `backend/internal/app/settings_registry.go`, add after the `log_file_read_enabled` entry:

```go
{Key: "monitoring.network_interface", Label: "Network Interface", Section: "monitoring", Type: SettingTypeText, Source: SettingSourceConfig, RequiresRestart: true, Default: "", SortOrder: 5},
{Key: "monitoring.vpn_interface", Label: "VPN Interface", Section: "monitoring", Type: SettingTypeText, Source: SettingSourceConfig, RequiresRestart: true, Default: "", SortOrder: 6},
{Key: "monitoring.collector_interval", Label: "Collector Interval (s)", Section: "monitoring", Type: SettingTypeNumber, Source: SettingSourceConfig, RequiresRestart: true, Default: "15", Validation: "min:5", SortOrder: 7},
```

- [ ] **Step 3: Add apply/read cases in settings_config_apply.go**

In `applyConfigSetting()`, before the `default:` case, add:

```go
case "monitoring.network_interface":
	cfg.Monitoring.NetworkInterface = value
case "monitoring.vpn_interface":
	cfg.Monitoring.VPNInterface = value
case "monitoring.collector_interval":
	n, err := strconv.Atoi(value)
	if err != nil {
		return fmt.Errorf("%w: monitoring.collector_interval must be an integer", ErrInvalidInput)
	}
	if n < 5 {
		return fmt.Errorf("%w: monitoring.collector_interval must be at least 5", ErrInvalidInput)
	}
	cfg.Monitoring.CollectorInterval = n
```

In `configValueForKey()`, before the `default:` case, add:

```go
case "monitoring.network_interface":
	return cfg.Monitoring.NetworkInterface
case "monitoring.vpn_interface":
	return cfg.Monitoring.VPNInterface
case "monitoring.collector_interval":
	return strconv.Itoa(cfg.Monitoring.CollectorInterval)
```

- [ ] **Step 4: Update frontend descriptions**

In `frontend/src/lib/settings/descriptions.ts`, update the `// Monitoring` section:

```ts
// Monitoring
'monitoring.health_endpoint': 'Enable the /health HTTP endpoint for uptime checks.',
'monitoring.metrics_endpoint': 'Enable the /metrics HTTP endpoint for monitoring dashboards.',
'monitoring.dashboard_refresh_interval': 'How often (in seconds) the dashboard auto-refreshes its data.',
'monitoring.log_file_read_enabled': 'Allow the panel to read log files for display in the dashboard.',
'monitoring.network_interface': 'Network interface for bandwidth metrics. Leave empty for auto-discovery.',
'monitoring.vpn_interface': 'VPN interface for VPN bandwidth metrics. Leave empty for auto-discovery (reads ocserv config).',
'monitoring.collector_interval': 'How often (in seconds) the backend collects system and VPN metrics.',
```

- [ ] **Step 5: Run backend tests**

Run: `cd /opt/ocserv_panel/backend && go test ./internal/app/ -run "TestSettings" -v -count=1`

Expected: All PASS

- [ ] **Step 6: Commit**

```bash
git add backend/internal/config/config.go backend/internal/app/settings_registry.go backend/internal/app/settings_config_apply.go frontend/src/lib/settings/descriptions.ts
git commit -m "feat(settings): add network_interface, vpn_interface, collector_interval monitoring settings"
```

---

### Task 6: Dashboard Service — Populate ActiveSessionCount

**Files:**
- Modify: `backend/internal/app/dashboard_service.go`
- Modify: `backend/internal/server/server.go`

- [ ] **Step 1: Add metric store dependency to DashboardService**

In `backend/internal/app/dashboard_service.go`, update the struct and constructor:

```go
type DashboardService struct {
	store        dashboardStore
	recorder     audit.Recorder
	authorizer   rbac.Authorizer
	readiness    *ocserv.ReadinessChecker
	metricStore  metrics.Store
	now          func() time.Time
}

func NewDashboardService(s *store.Store, recorder audit.Recorder, authorizer rbac.Authorizer, readiness *ocserv.ReadinessChecker, metricStore ...metrics.Store) *DashboardService {
	if recorder == nil {
		recorder = audit.NoopRecorder{}
	}
	var ms metrics.Store
	if len(metricStore) > 0 {
		ms = metricStore[0]
	}
	return &DashboardService{store: s, recorder: recorder, authorizer: authorizer, readiness: readiness, metricStore: ms, now: func() time.Time { return time.Now().UTC() }}
}
```

Add a helper method:

```go
// latestSessionCount queries the TSDB for the most recent active_sessions value.
func (s *DashboardService) latestSessionCount(ctx context.Context) int {
	if s.metricStore == nil {
		return 0
	}
	now := s.now()
	series, err := s.metricStore.QueryRange(ctx, metrics.Query{
		Metric: metrics.MetricActiveSessions,
		Labels: map[string]string{},
		Start:  now.Add(-2 * time.Minute),
		End:    now,
		Step:   15 * time.Second,
	})
	if err != nil || len(series.Points) == 0 {
		return 0
	}
	last := series.Points[len(series.Points)-1]
	return int(last.Value)
}
```

Update the `Summary()` return to use it:

```go
return DashboardSummary{
	BackendStatus:      "ok",
	ActiveSessionCount: s.latestSessionCount(ctx),
	// ... rest unchanged
```

- [ ] **Step 2: Add dashboardRefreshInterval to response**

In `backend/internal/server/server.go`, update the `dashboardSummaryResponse` struct:

```go
type dashboardSummaryResponse struct {
	BackendStatus             string                   `json:"backendStatus"`
	OcservStatus              string                   `json:"ocservStatus"`
	ActiveSessionCount        int                      `json:"activeSessionCount"`
	RecentAudit               []auditEventResponse     `json:"recentAudit"`
	Analytics                 analyticsSummaryResponse `json:"analytics"`
	SampleData                bool                     `json:"sampleData"`
	Ocserv                    ocservStatusResponse     `json:"ocserv"`
	SetupRequired             bool                     `json:"setupRequired"`
	AdminCount                int64                    `json:"adminCount"`
	History                   analyticsSummaryResponse `json:"history"`
	DashboardRefreshInterval  int                      `json:"dashboardRefreshInterval"`
}
```

Update `dashboardSummaryToResponse()`:

```go
func dashboardSummaryToResponse(summary app.DashboardSummary, refreshInterval int) dashboardSummaryResponse {
	return dashboardSummaryResponse{
		BackendStatus:            summary.BackendStatus,
		OcservStatus:             summary.Ocserv.Status,
		ActiveSessionCount:      summary.ActiveSessionCount,
		RecentAudit:             auditEventsToResponse(summary.RecentAudit),
		Analytics:               analyticsSummaryToResponse(summary.Analytics),
		SampleData:              summary.SampleData,
		Ocserv: ocservStatusResponse{
			Status:     summary.Ocserv.Status,
			Configured: summary.Ocserv.Configured,
			Message:    summary.Ocserv.Message,
		},
		SetupRequired:            summary.SetupRequired,
		AdminCount:               summary.AdminCount,
		History:                  analyticsSummaryToResponse(summary.History),
		DashboardRefreshInterval: refreshInterval,
	}
}
```

Update the handler to pass the refresh interval. The handler needs access to the config. Add `refreshInterval int` to `AppOptions`:

In the `AppOptions` struct (or wherever server options are defined), add:

```go
type AppOptions struct {
	CookieSecure    bool
	LoginLimiter    *auth.RateLimiter
	RefreshInterval int
}
```

Update `handleDashboardSummary`:

```go
func (a *App) handleDashboardSummary(w http.ResponseWriter, r *http.Request) {
	if !allowMethod(w, r, http.MethodGet) {
		return
	}
	principal, ok := a.authenticate(w, r)
	if !ok {
		return
	}
	summary, err := a.dashboardService.Summary(r.Context(), principal)
	if err != nil {
		a.writeAppError(w, err)
		return
	}
	refreshInterval := a.opts.RefreshInterval
	if refreshInterval < 5 {
		refreshInterval = 15
	}
	writeJSON(w, http.StatusOK, dashboardSummaryToResponse(summary, refreshInterval))
}
```

- [ ] **Step 3: Update main.go to pass metricStore and refreshInterval**

In `backend/cmd/ocserv-panel/main.go`, update the `NewDashboardService` call:

```go
dashboardService := app.NewDashboardService(persistence, recorder, rbac.NewDefaultAuthorizer(), readinessChecker, metricStore)
```

Update the `server.AppOptions`:

```go
server.AppOptions{
	CookieSecure:    cookieSecureFromEnv(),
	LoginLimiter:    loginLimiter,
	RefreshInterval: cfg.Monitoring.DashboardRefreshInterval,
},
```

- [ ] **Step 4: Run tests**

Run: `cd /opt/ocserv_panel/backend && go test ./internal/app/ ./internal/server/ -v -count=1 -timeout 30s`

Expected: All PASS (tests use nil metricStore which returns 0 — backward compatible)

- [ ] **Step 5: Commit**

```bash
git add backend/internal/app/dashboard_service.go backend/internal/server/server.go backend/cmd/ocserv-panel/main.go
git commit -m "feat(dashboard): populate activeSessionCount from TSDB, add dashboardRefreshInterval to response"
```

---

### Task 7: Backend — Active User Sessions Endpoint

**Files:**
- Modify: `backend/internal/app/ocserv_service.go`
- Modify: `backend/internal/server/server.go`

- [ ] **Step 1: Add ActiveUserSessions to OcservService**

In `backend/internal/app/ocserv_service.go`, add:

```go
// UserSessionCount represents one user's active session count for the donut chart.
type UserSessionCount struct {
	Username     string `json:"username"`
	SessionCount int    `json:"sessionCount"`
}

// ActiveUserSessionsResult is the response for the active sessions donut endpoint.
type ActiveUserSessionsResult struct {
	Users []UserSessionCount `json:"users"`
	Total int                `json:"total"`
}

// ActiveUserSessions returns per-user session counts for the dashboard donut chart.
func (s *OcservService) ActiveUserSessions(ctx context.Context, principal auth.Principal) (ActiveUserSessionsResult, error) {
	if err := s.requirePerm(ctx, principal, rbac.PermissionOcservRead, "ocserv.active_user_sessions"); err != nil {
		return ActiveUserSessionsResult{}, err
	}

	users, err := s.sessions.ListConnectedUsers(ctx)
	if err != nil {
		return ActiveUserSessionsResult{}, fmt.Errorf("list connected users: %w", err)
	}

	counts := make(map[string]int)
	for _, u := range users {
		counts[u.Username]++
	}

	result := make([]UserSessionCount, 0, len(counts))
	total := 0
	for username, count := range counts {
		result = append(result, UserSessionCount{Username: username, SessionCount: count})
		total += count
	}

	return ActiveUserSessionsResult{Users: result, Total: total}, nil
}
```

- [ ] **Step 2: Add the endpoint handler**

In `backend/internal/server/server.go`, add the handler:

```go
func (a *App) handleDashboardActiveSessions(w http.ResponseWriter, r *http.Request) {
	if !allowMethod(w, r, http.MethodGet) {
		return
	}
	principal, ok := a.authenticate(w, r)
	if !ok {
		return
	}
	result, err := a.ocservService.ActiveUserSessions(r.Context(), principal)
	if err != nil {
		a.writeAppError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}
```

Register the route in `NewAppRouter`:

```go
mux.HandleFunc("GET /api/dashboard/active-sessions", appHandler.handleDashboardActiveSessions)
```

- [ ] **Step 3: Run tests**

Run: `cd /opt/ocserv_panel/backend && go build ./... && go test ./internal/server/ -v -count=1 -timeout 30s`

Expected: Build succeeds, tests PASS.

- [ ] **Step 4: Commit**

```bash
git add backend/internal/app/ocserv_service.go backend/internal/server/server.go
git commit -m "feat(api): add GET /api/dashboard/active-sessions for donut chart data"
```

---

### Task 8: Wire Collectors in main.go

**Files:**
- Modify: `backend/cmd/ocserv-panel/main.go`

- [ ] **Step 1: Update main.go to use config for interfaces and add SessionCollector**

Update the `configuredCollectors` function and the main wiring:

```go
func configuredCollectors(reader monitoring.FileReader, cfg config.MonitoringConfig) []monitoring.Collector {
	collectors := []monitoring.Collector{
		monitoring.NewCPUCollector(reader),
		monitoring.NewMemoryCollector(reader),
	}

	// Network interface: config > env > auto-discover
	netIface := cfg.NetworkInterface
	if netIface == "" {
		netIface = strings.TrimSpace(os.Getenv("OCSERV_PANEL_NET_IFACE"))
	}
	if netIface == "" {
		if defaultIface, err := monitoring.DiscoverDefaultInterface(); err != nil {
			log.Warn().Err(err).Msg("default interface discovery failed")
		} else if defaultIface != "" {
			log.Info().Str("interface", defaultIface).Msg("default interface discovered")
			netIface = defaultIface
		} else {
			log.Info().Msg("no default interface discovered; network interface metrics will remain unavailable")
		}
	}
	if netIface != "" {
		collectors = append(collectors, monitoring.NewInterfaceCollector(
			reader,
			netIface,
			metrics.MetricNetInterfaceRXBytes,
			metrics.MetricNetInterfaceTXBytes,
		))
	}

	// VPN interface: config > env (startup only; re-discovery handles the dynamic case)
	vpnIface := cfg.VPNInterface
	if vpnIface == "" {
		vpnIface = strings.TrimSpace(os.Getenv("OCSERV_PANEL_VPN_IFACE"))
	}
	if vpnIface != "" {
		collectors = append(collectors, monitoring.NewInterfaceCollector(
			reader,
			vpnIface,
			metrics.MetricVPNInterfaceRXBytes,
			metrics.MetricVPNInterfaceTXBytes,
		))
	}

	return collectors
}
```

Update the main function to pass config, add SessionCollector, and wire VPN re-discovery:

In `main()`, replace the existing collector setup block with:

```go
collectors := configuredCollectors(monitoring.OSReader{}, cfg.Monitoring)

// Add session collector if occtl is available.
collectors = append(collectors, monitoring.NewSessionCollector(sessionManager))

collectorInterval := time.Duration(cfg.Monitoring.CollectorInterval) * time.Second
if collectorInterval < 5*time.Second {
	collectorInterval = 15 * time.Second
}

collectorService := monitoring.NewCollectorService(
	metricStore,
	collectors,
	collectorInterval,
	zerologAdapter{},
	nil,
)

// VPN re-discovery: if no VPN collector was added at startup, set up periodic re-discovery.
vpnExplicitlySet := cfg.Monitoring.VPNInterface != "" || strings.TrimSpace(os.Getenv("OCSERV_PANEL_VPN_IFACE")) != ""
if vpnExplicitlySet {
	collectorService.MarkVPNPresent()
	monitoringService.SetMetricState(metrics.MetricVPNInterfaceRXBytes, metrics.StateCollecting, "")
	monitoringService.SetMetricState(metrics.MetricVPNInterfaceTXBytes, metrics.StateCollecting, "")
} else {
	// Try discovery once at startup
	vpnIface, vpnSource, err := monitoring.DiscoverVPNInterface(
		monitoring.WithOcservConfigPath(cfg.Installation.OcservConfigPath),
	)
	if err != nil {
		log.Warn().Err(err).Msg("vpn interface discovery failed")
	} else if vpnIface != "" {
		log.Info().Str("interface", vpnIface).Str("source", vpnSource).Msg("vpn interface discovered")
		collectorService.MarkVPNPresent()
		monitoringService.SetMetricState(metrics.MetricVPNInterfaceRXBytes, metrics.StateCollecting, "")
		monitoringService.SetMetricState(metrics.MetricVPNInterfaceTXBytes, metrics.StateCollecting, "")
		// We already missed adding the collector; add dynamically since configuredCollectors was called before discovery
		// Actually it's simpler to just add it to the service now
	} else {
		log.Info().Msg("no vpn interface discovered at startup; will re-try periodically")
		collectorService.SetVPNDiscoverer(func() (string, string, error) {
			return monitoring.DiscoverVPNInterface(
				monitoring.WithOcservConfigPath(cfg.Installation.OcservConfigPath),
			)
		}, monitoring.OSReader{}, metrics.MetricVPNInterfaceRXBytes, metrics.MetricVPNInterfaceTXBytes)
	}
}

go collectorService.Run(ctx)
```

Remove the old VPN discovery block that was duplicating this logic.

- [ ] **Step 2: Build and verify**

Run: `cd /opt/ocserv_panel/backend && go build ./...`

Expected: Build succeeds.

- [ ] **Step 3: Commit**

```bash
git add backend/cmd/ocserv-panel/main.go
git commit -m "feat(main): wire SessionCollector, config-based interfaces, VPN re-discovery"
```

---

### Task 9: Frontend — Monitoring Formatting (bytes/sec)

**Files:**
- Modify: `frontend/src/lib/monitoring.ts`

- [ ] **Step 1: Update formatMetricValue for rate metrics and zero counts**

In `frontend/src/lib/monitoring.ts`, update `formatMetricValue`:

```ts
export function formatMetricValue(metric: string, value: number | undefined | null): string {
  if (value === 0) return '0';
  if (!isFiniteValue(value)) return UNAVAILABLE_VALUE;

  if (isUtilizationMetric(metric)) {
    return `${value.toFixed(1)}%`;
  }

  if (isByteMetric(metric)) {
    return `${formatBytes(value, 'iec')}/s`;
  }

  return formatNumber(value);
}
```

Update `formatAxisValue` for byte metrics to show rate:

```ts
export function formatAxisValue(metric: string, value: number): string {
  if (isUtilizationMetric(metric)) {
    return `${Math.round(value)}%`;
  }
  if (isByteMetric(metric)) {
    return `${formatBytesCompact(value)}/s`;
  }
  // count / session metrics
  if (Math.abs(value) >= 10000) {
    return `${(value / 1000).toFixed(0)}k`;
  }
  if (Math.abs(value) >= 1000) {
    return `${(value / 1000).toFixed(1)}k`;
  }
  return String(Math.round(value));
}
```

- [ ] **Step 2: Verify TypeScript compiles**

Run: `cd /opt/ocserv_panel/frontend && npx svelte-check --threshold error 2>&1 | tail -5`

Expected: No errors

- [ ] **Step 3: Commit**

```bash
git add frontend/src/lib/monitoring.ts
git commit -m "feat(frontend): format network metrics as bytes/sec rate, show 0 instead of --"
```

---

### Task 10: Frontend — API Type and Fetcher for Active Sessions

**Files:**
- Modify: `frontend/src/lib/api.ts`

- [ ] **Step 1: Add types and fetcher**

At the end of the types section in `frontend/src/lib/api.ts`, add:

```ts
export type UserSessionCount = {
  username: string;
  sessionCount: number;
};

export type ActiveUserSessionsResult = {
  users: UserSessionCount[];
  total: number;
};
```

Add the fetcher function (near the other `getDashboard*` functions):

```ts
export async function getDashboardActiveSessions(): Promise<ActiveUserSessionsResult> {
  return apiFetch<ActiveUserSessionsResult>('/api/dashboard/active-sessions');
}
```

Also update the `DashboardSummary` type to include `dashboardRefreshInterval`:

```ts
export type DashboardSummary = {
  backendStatus?: string;
  ocservStatus?: string;
  activeSessionCount?: number;
  recentAudit?: AuditEvent[];
  analytics?: {
    totalSessions?: number;
    sampleSessions?: number;
    totalBytesIn?: number;
    totalBytesOut?: number;
  };
  sampleData?: boolean | { enabled?: boolean; present?: boolean; message?: string };
  ocserv?: OcservStatus;
  setupRequired?: boolean;
  adminCount?: number;
  history?: {
    totalSessions?: number;
    sampleSessions?: number;
    totalBytesIn?: number;
    totalBytesOut?: number;
  };
  dashboardRefreshInterval?: number;
  [key: string]: unknown;
};
```

- [ ] **Step 2: Verify TypeScript compiles**

Run: `cd /opt/ocserv_panel/frontend && npx svelte-check --threshold error 2>&1 | tail -5`

Expected: No errors

- [ ] **Step 3: Commit**

```bash
git add frontend/src/lib/api.ts
git commit -m "feat(frontend): add ActiveUserSessions API type and fetcher, dashboardRefreshInterval"
```

---

### Task 11: Frontend — DonutChart Component

**Files:**
- Create: `frontend/src/lib/components/ui/DonutChart.svelte`

- [ ] **Step 1: Create DonutChart component**

Create `frontend/src/lib/components/ui/DonutChart.svelte`:

```svelte
<script lang="ts">
  import { Chart, Svg, Arc, Pie, Tooltip, Group, Text } from 'layerchart';

  export type DonutSegment = {
    label: string;
    value: number;
    color: string;
  };

  type Props = {
    segments: DonutSegment[];
    title?: string;
    size?: number;
  };

  let { segments, title, size = 180 }: Props = $props();

  const total = $derived(segments.reduce((sum, s) => sum + s.value, 0));

  const COLORS = [
    'hsl(262, 83%, 58%)',  // violet
    'hsl(199, 89%, 48%)',  // sky
    'hsl(160, 84%, 39%)',  // emerald
    'hsl(38, 92%, 50%)',   // amber
    'hsl(346, 77%, 50%)',  // rose
    'hsl(217, 91%, 60%)',  // blue
  ];

  const coloredSegments = $derived(
    segments.map((s, i) => ({
      ...s,
      color: s.color || COLORS[i % COLORS.length],
    }))
  );
</script>

<div class="flex flex-col items-center gap-3">
  {#if title}
    <p class="text-sm font-medium text-muted-foreground">{title}</p>
  {/if}

  {#if segments.length === 0 || total === 0}
    <div
      class="flex items-center justify-center rounded-full border-2 border-dashed border-border bg-muted/30"
      style="width: {size}px; height: {size}px"
    >
      <p class="text-xs text-muted-foreground text-center px-4">No active sessions</p>
    </div>
  {:else}
    <div style="width: {size}px; height: {size}px" class="relative">
      <Chart data={coloredSegments} let:tooltip>
        <Svg>
          <Pie value="value" let:arcs>
            {#each arcs as arc, i}
              <Group>
                <Arc
                  startAngle={arc.startAngle}
                  endAngle={arc.endAngle}
                  innerRadius={size * 0.32}
                  outerRadius={size * 0.46}
                  padAngle={0.03}
                  cornerRadius={3}
                  fill={coloredSegments[i].color}
                  class="transition-opacity hover:opacity-80"
                />
              </Group>
            {/each}
          </Pie>
          <!-- Center text -->
          <Text
            value={String(total)}
            textAnchor="middle"
            dy={-4}
            class="text-2xl font-bold fill-foreground"
          />
          <Text
            value={total === 1 ? 'session' : 'sessions'}
            textAnchor="middle"
            dy={16}
            class="text-xs fill-muted-foreground"
          />
        </Svg>
      </Chart>
    </div>

    <!-- Legend -->
    <div class="flex flex-wrap gap-x-4 gap-y-1 justify-center">
      {#each coloredSegments as segment}
        <div class="flex items-center gap-1.5 text-xs">
          <span
            class="w-2.5 h-2.5 rounded-full flex-shrink-0"
            style="background-color: {segment.color}"
          ></span>
          <span class="text-muted-foreground">{segment.label}</span>
          <span class="font-medium text-foreground">{segment.value}</span>
        </div>
      {/each}
    </div>
  {/if}
</div>
```

- [ ] **Step 2: Verify TypeScript compiles**

Run: `cd /opt/ocserv_panel/frontend && npx svelte-check --threshold error 2>&1 | tail -5`

Expected: No errors (may have warnings about unused imports, acceptable)

- [ ] **Step 3: Commit**

```bash
git add frontend/src/lib/components/ui/DonutChart.svelte
git commit -m "feat(frontend): add DonutChart component for active sessions by user"
```

---

### Task 12: Frontend — Dashboard Page Updates

**Files:**
- Modify: `frontend/src/routes/+page.svelte`

- [ ] **Step 1: Add imports and state for donut chart and dynamic refresh**

At the top of the `<script>` block, add the new imports:

```ts
import DonutChart from '$lib/components/ui/DonutChart.svelte';
import type { DonutSegment } from '$lib/components/ui/DonutChart.svelte';
import {
  getDashboardSummary,
  getDashboardActiveSessions,
  // ... existing imports
  type ActiveUserSessionsResult,
} from '$lib/api';
```

Add state variables:

```ts
let activeSessions = $state<ActiveUserSessionsResult | null>(null);
let activeSessionsLoading = $state(false);
```

- [ ] **Step 2: Update loadDashboard to fetch active sessions and use dynamic refresh**

In `loadDashboard()`, after `summary = await getDashboardSummary();`, add:

```ts
void loadActiveSessions();
```

Add the `loadActiveSessions` function:

```ts
async function loadActiveSessions() {
  activeSessionsLoading = true;
  try {
    activeSessions = await getDashboardActiveSessions();
  } catch {
    activeSessions = null;
  } finally {
    activeSessionsLoading = false;
  }
}
```

Update the refresh interval setup in `onMount` to use the dynamic value:

```ts
onMount(() => {
  void loadDashboard().then(() => {
    const refreshMs = ((summary?.dashboardRefreshInterval ?? 15) * 1000);
    refreshInterval = setInterval(() => {
      if (document.visibilityState === 'visible') {
        void reloadMonitoring();
        void loadActiveSessions();
      }
    }, refreshMs);
  });

  void loadHealth();
  healthRefreshInterval = setInterval(() => {
    if (document.visibilityState === 'visible') {
      void loadHealth();
    }
  }, 30_000);
});
```

- [ ] **Step 3: Add donut chart derived data**

```ts
const donutSegments = $derived<DonutSegment[]>(
  activeSessions?.users?.map((u, i) => ({
    label: u.username,
    value: u.sessionCount,
    color: '',  // DonutChart assigns default colors
  })) ?? []
);
```

- [ ] **Step 4: Replace the Active Sessions CardStats with donut layout**

Replace the existing "Active sessions + Storage metrics" grid:

```svelte
<!-- Active sessions + Storage metrics -->
<div class="grid grid-cols-1 md:grid-cols-2 gap-6 mb-8">
  <Card.Root>
    <Card.Header>
      <div>
        <Card.Title>Active Sessions</Card.Title>
        <Card.Description>
          {#if summary?.ocserv?.status === 'running'}
            Sessions by connected user
          {:else}
            ocserv is not running
          {/if}
        </Card.Description>
      </div>
      <span class="text-2xl font-bold text-foreground tabular-nums">
        {summary?.activeSessionCount ?? 0}
      </span>
    </Card.Header>
    <Card.Content class="flex items-center justify-center py-4">
      {#if activeSessionsLoading && !activeSessions}
        <div class="w-8 h-8 border-2 border-slate-200 border-t-slate-500 rounded-full animate-spin"></div>
      {:else}
        <DonutChart segments={donutSegments} size={160} />
      {/if}
    </Card.Content>
  </Card.Root>
  <HealthCard
    title="Storage Metrics"
    items={storageItems}
    lastChecked={healthCheckedAt}
    loading={healthLoading && !health}
  />
</div>
```

- [ ] **Step 5: Fix Analytics Summary Cards**

Replace the analytics section:

```svelte
<!-- Analytics section -->
<section aria-labelledby="analytics-heading" class="mb-8">
  <h2 id="analytics-heading" class="text-lg font-semibold text-slate-700 mb-4">Analytics Summary</h2>
  {#if summary?.analytics}
    {@const analytics = summary.analytics}
    {@const realSessions = (analytics.totalSessions ?? 0) - (analytics.sampleSessions ?? 0)}
    <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-5 gap-6">
      <CardStats
        title="Total sessions"
        value={String(analytics.totalSessions ?? 0)}
        subtitle={analytics.sampleSessions ? `incl. ${analytics.sampleSessions} samples` : undefined}
        icon={Users}
      />
      <CardStats
        title="Recorded sessions"
        value={String(realSessions)}
        subtitle="Excludes sample data"
        icon={Database}
      />
      <CardStats
        title="Total received"
        value={formatBytes(analytics.totalBytesIn)}
        icon={ArrowDown}
      />
      <CardStats
        title="Total sent"
        value={formatBytes(analytics.totalBytesOut)}
        icon={ArrowUp}
      />
      <CardStats
        title="Total traffic"
        value={formatBytes((analytics.totalBytesIn ?? 0) + (analytics.totalBytesOut ?? 0))}
        icon={Server}
      />
    </div>
  {:else}
    <p class="text-sm text-slate-500">No analytics summary returned yet.</p>
  {/if}
</section>
```

- [ ] **Step 6: Verify TypeScript compiles and build succeeds**

Run: `cd /opt/ocserv_panel/frontend && npx svelte-check --threshold error 2>&1 | tail -10`

Then: `cd /opt/ocserv_panel/frontend && npm run build 2>&1 | tail -5`

Expected: No errors

- [ ] **Step 7: Commit**

```bash
git add frontend/src/routes/+page.svelte
git commit -m "feat(frontend): add donut chart, fix analytics cards, dynamic refresh interval"
```

---

### Task 13: Full Build Verification

**Files:** All

- [ ] **Step 1: Run full backend test suite**

Run: `cd /opt/ocserv_panel/backend && go test ./... -count=1 -timeout 60s`

Expected: All packages PASS

- [ ] **Step 2: Run frontend build**

Run: `cd /opt/ocserv_panel/frontend && npm run build`

Expected: Build succeeds

- [ ] **Step 3: Run frontend type check**

Run: `cd /opt/ocserv_panel/frontend && npx svelte-check --threshold error`

Expected: No errors

- [ ] **Step 4: Verify no unintended file changes**

Run: `git status`

Expected: Only the files listed in this plan are modified/created.

---

### Summary of Changes

| Area | Files | Purpose |
|------|-------|---------|
| Backend — Monitoring | `collectors.go` | Rate-based InterfaceCollector (bytes/sec) |
| Backend — Monitoring | `session_collector.go` (NEW) | SessionCollector via occtl |
| Backend — Monitoring | `vpn_discovery.go` | Add vpns prefix, resolve numbered instances |
| Backend — Monitoring | `collector_service.go` | Periodic VPN re-discovery |
| Backend — Config | `config.go` | New monitoring fields |
| Backend — Settings | `settings_registry.go`, `settings_config_apply.go` | New settings entries + apply/read |
| Backend — Dashboard | `dashboard_service.go` | Populate ActiveSessionCount from TSDB |
| Backend — API | `ocserv_service.go`, `server.go` | Active sessions endpoint, refresh interval in response |
| Backend — Main | `main.go` | Wire new collectors, config-based interfaces |
| Frontend — Monitoring | `monitoring.ts` | Rate formatting (bytes/sec), fix zero display |
| Frontend — API | `api.ts` | New types + fetcher |
| Frontend — Component | `DonutChart.svelte` (NEW) | Donut chart for sessions by user |
| Frontend — Dashboard | `+page.svelte` | Donut section, fixed cards, dynamic refresh |
| Frontend — Settings | `descriptions.ts` | New setting descriptions |
