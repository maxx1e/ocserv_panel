# VPN Interface Watcher Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the broken timer-based VPN re-discovery in `CollectorService` with a session-triggered `VPNWatcher` that correctly adds/removes `InterfaceCollector`s and updates the metric catalog when VPN interfaces appear and disappear in sysfs.

**Architecture:** A new `VPNWatcher` type owns the set of tracked VPN interfaces, receives notifications from `SessionCollector` when session counts change, diffs the tracked set against sysfs discovery, and calls `CollectorService.AddCollector`/`RemoveCollectorByIface` plus `Catalog.SetState` to keep metrics accurate. A 15-second fallback ticker fires if no notification arrives.

**Tech Stack:** Go stdlib (`sync`, `time`, `context`), existing `monitor` package types (`CollectorService`, `Catalog`, `VPNMultiDiscovererFunc`, `InterfaceCollector`, `FileReader`).

---

## File Map

| File | Action | Responsibility |
|---|---|---|
| `backend/internal/monitor/collectors.go` | Modify | Add `Iface() string` getter to `InterfaceCollector` |
| `backend/internal/monitor/service.go` | Modify | Add `AddCollector`/`RemoveCollectorByIface`; remove VPN discoverer fields and methods |
| `backend/internal/monitor/service_test.go` | Modify | Remove obsolete VPN discoverer tests; add `AddCollector`/`RemoveCollectorByIface` tests |
| `backend/internal/monitor/session_collector.go` | Modify | Add `SessionChangeNotifier` interface + `lastCount`; add `NewSessionCollectorWithNotifier` |
| `backend/internal/monitor/session_collector_test.go` | Modify | Add tests for notifier firing on count change |
| `backend/internal/monitor/vpn_watcher.go` | Create | `VPNWatcher` type — owns tracked map, runs sync loop |
| `backend/internal/monitor/vpn_watcher_test.go` | Create | Unit tests for all `VPNWatcher` behaviours |
| `backend/internal/monitor/types.go` | Modify | Change VPN metrics default catalog state to `unavailable` |
| `backend/cmd/ocserv-panel/main.go` | Modify | Wire `VPNWatcher`; remove `vpnConfigured` block and VPN branch in `configuredCollectors` |

---

## Task 1: Add `Iface()` getter to `InterfaceCollector`

**Files:**
- Modify: `backend/internal/monitor/collectors.go:162-175`

- [ ] **Step 1.1: Write a failing test for the getter**

Add to the end of `backend/internal/monitor/collectors_test.go`:

```go
func TestInterfaceCollector_IfaceGetter(t *testing.T) {
	t.Parallel()

	c := NewInterfaceCollector(
		&fakeFileReader{files: map[string][]string{}},
		"vpns0",
		MetricVPNInterfaceRXBytes,
		MetricVPNInterfaceTXBytes,
	)
	if got := c.Iface(); got != "vpns0" {
		t.Fatalf("Iface() = %q, want %q", got, "vpns0")
	}
}
```

- [ ] **Step 1.2: Run to confirm it fails**

```bash
cd backend && go test ./internal/monitor/... -run TestInterfaceCollector_IfaceGetter -v
```

Expected: `undefined: (*InterfaceCollector).Iface`

- [ ] **Step 1.3: Add the getter**

In `backend/internal/monitor/collectors.go`, after the `Collect` method of `InterfaceCollector` (around line 242), add:

```go
func (c *InterfaceCollector) Iface() string { return c.iface }
```

- [ ] **Step 1.4: Run to confirm pass**

```bash
cd backend && go test ./internal/monitor/... -run TestInterfaceCollector_IfaceGetter -v
```

Expected: `PASS`

- [ ] **Step 1.5: Commit**

```bash
git add backend/internal/monitor/collectors.go backend/internal/monitor/collectors_test.go
git commit -m "feat(monitor): add Iface() getter to InterfaceCollector"
```

---

## Task 2: Add `AddCollector` and `RemoveCollectorByIface` to `CollectorService`; remove VPN discoverer infrastructure

**Files:**
- Modify: `backend/internal/monitor/service.go`
- Modify: `backend/internal/monitor/service_test.go`

- [ ] **Step 2.1: Write failing tests for `AddCollector` and `RemoveCollectorByIface`**

Add to `backend/internal/monitor/service_test.go` (before the `fakeFileReaderForVPN` block):

```go
func TestCollectorService_AddCollector(t *testing.T) {
	store := NewMemoryStore()
	now := time.Date(2026, 5, 10, 12, 0, 0, 0, time.UTC)
	service := NewCollectorService(store, []Collector{}, time.Second, &fakeLogger{}, func() time.Time { return now })

	called := false
	service.AddCollector(fakeCollector{result: available([]Sample{{
		Name:      MetricVPNInterfaceRXBytes,
		Labels:    map[string]string{},
		Timestamp: now,
		Value:     99,
	}})})

	service.CollectOnce(context.Background())
	_ = called

	series, err := store.QueryRange(context.Background(), Query{
		Metric: MetricVPNInterfaceRXBytes,
		Start:  now.Add(-time.Second),
		End:    now.Add(time.Second),
		Step:   time.Second,
	})
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if len(series.Points) != 1 {
		t.Fatalf("points = %d, want 1", len(series.Points))
	}
	if series.Points[0].Value != 99 {
		t.Fatalf("value = %v, want 99", series.Points[0].Value)
	}
}

func TestCollectorService_RemoveCollectorByIface(t *testing.T) {
	now := time.Date(2026, 5, 10, 12, 0, 0, 0, time.UTC)
	store := NewMemoryStore()

	c0 := NewInterfaceCollector(
		&fakeFileReaderForVPN{files: map[string]string{
			"/sys/class/net/vpns0/statistics/rx_bytes": "1000",
			"/sys/class/net/vpns0/statistics/tx_bytes": "2000",
		}},
		"vpns0",
		MetricVPNInterfaceRXBytes,
		MetricVPNInterfaceTXBytes,
	)
	c1 := NewInterfaceCollector(
		&fakeFileReaderForVPN{files: map[string]string{
			"/sys/class/net/vpns1/statistics/rx_bytes": "3000",
			"/sys/class/net/vpns1/statistics/tx_bytes": "4000",
		}},
		"vpns1",
		MetricVPNInterfaceRXBytes,
		MetricVPNInterfaceTXBytes,
	)

	service := NewCollectorService(store, []Collector{c0, c1}, time.Second, &fakeLogger{}, func() time.Time { return now })
	if len(service.collectors) != 2 {
		t.Fatalf("before remove: collectors = %d, want 2", len(service.collectors))
	}

	service.RemoveCollectorByIface("vpns0")

	if len(service.collectors) != 1 {
		t.Fatalf("after remove: collectors = %d, want 1", len(service.collectors))
	}
	remaining := service.collectors[0].(*InterfaceCollector)
	if remaining.Iface() != "vpns1" {
		t.Fatalf("remaining collector iface = %q, want vpns1", remaining.Iface())
	}
}

func TestCollectorService_RemoveCollectorByIface_NonExistentIsNoop(t *testing.T) {
	store := NewMemoryStore()
	service := NewCollectorService(store, []Collector{}, time.Second, &fakeLogger{}, func() time.Time { return time.Now() })
	// Should not panic when interface is not tracked.
	service.RemoveCollectorByIface("vpns99")
	if len(service.collectors) != 0 {
		t.Fatalf("collectors = %d, want 0", len(service.collectors))
	}
}
```

- [ ] **Step 2.2: Run to confirm they fail**

```bash
cd backend && go test ./internal/monitor/... -run "TestCollectorService_AddCollector|TestCollectorService_RemoveCollector" -v
```

Expected: `undefined: (*CollectorService).AddCollector` and `RemoveCollectorByIface`

- [ ] **Step 2.3: Rewrite `service.go`**

Replace the entire content of `backend/internal/monitor/service.go` with:

```go
package monitor

import (
	"context"
	"sync"
	"time"
)

const defaultCollectorInterval = 15 * time.Second

type Logger interface {
	Debug(msg string, args ...any)
	Warn(msg string, args ...any)
}

type CollectorService struct {
	store    Store
	interval time.Duration
	logger   Logger
	now      func() time.Time

	mu         sync.Mutex
	collectors []Collector
}

func NewCollectorService(
	store Store,
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

	cloned := make([]Collector, len(collectors))
	copy(cloned, collectors)

	return &CollectorService{
		store:      store,
		collectors: cloned,
		interval:   interval,
		logger:     logger,
		now:        now,
	}
}

// AddCollector appends a collector to the running service.
func (s *CollectorService) AddCollector(c Collector) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.collectors = append(s.collectors, c)
}

// RemoveCollectorByIface removes all InterfaceCollectors whose Iface() equals iface.
func (s *CollectorService) RemoveCollectorByIface(iface string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	kept := s.collectors[:0]
	for _, c := range s.collectors {
		if ic, ok := c.(*InterfaceCollector); ok && ic.Iface() == iface {
			continue
		}
		kept = append(kept, c)
	}
	s.collectors = kept
}

func (s *CollectorService) CollectOnce(ctx context.Context) {
	if err := ctx.Err(); err != nil {
		return
	}

	s.mu.Lock()
	snapshot := make([]Collector, len(s.collectors))
	copy(snapshot, s.collectors)
	s.mu.Unlock()

	for _, collector := range snapshot {
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
			if err := s.store.Append(ctx, []Sample{sample}); err != nil {
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

	s.CollectOnce(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.CollectOnce(ctx)
		}
	}
}

type noopLogger struct{}

func (noopLogger) Debug(string, ...any) {}
func (noopLogger) Warn(string, ...any)  {}
```

- [ ] **Step 2.4: Delete tests that test removed methods**

In `backend/internal/monitor/service_test.go`, delete the following test functions (they test the now-removed VPN discoverer infrastructure):

- `TestCollectorService_VPNRediscovery`
- `TestCollectorService_VPNRediscoverySkipsWhenAlreadyPresent`
- `TestCollectorService_MarkVPNPresentPreventsDiscovery`
- `TestCollectorService_CollectOnceThreadSafeWithDiscovery`
- `TestCollectorService_MultiVPNDiscoveryAddsAllInterfaces`
- `TestCollectorService_MultiVPNDiscoverySkipsAlreadyRegistered`

Also remove the `fakeFileReaderForVPN` test helper struct at the bottom of `service_test.go` — it will move to `vpn_watcher_test.go` in Task 4.

- [ ] **Step 2.5: Run the full monitor package tests**

```bash
cd backend && go test ./internal/monitor/... -v 2>&1 | tail -30
```

Expected: All tests `PASS`, no compilation errors.

- [ ] **Step 2.6: Commit**

```bash
git add backend/internal/monitor/service.go backend/internal/monitor/service_test.go
git commit -m "refactor(monitor): replace VPN discoverer with AddCollector/RemoveCollectorByIface"
```

---

## Task 3: Add `SessionChangeNotifier` to `SessionCollector`

**Files:**
- Modify: `backend/internal/monitor/session_collector.go`
- Modify: `backend/internal/monitor/session_collector_test.go`

- [ ] **Step 3.1: Write failing tests**

Add to `backend/internal/monitor/session_collector_test.go`:

```go
type fakeNotifier struct {
	mu    sync.Mutex
	calls int
}

func (n *fakeNotifier) NotifySessionChange() {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.calls++
}

func (n *fakeNotifier) count() int {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.calls
}

func TestSessionCollector_NotifierCalledOnCountChange(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 10, 12, 0, 0, 0, time.UTC)
	manager := sessions.NewFakeManager()
	manager.ServerStatusResult = &sessions.ServerStatus{ActiveSessions: 3}

	notifier := &fakeNotifier{}
	collector := NewSessionCollectorWithNotifier(manager, notifier)

	// First collect: lastCount is 0, active is 3 — count changed.
	collector.Collect(context.Background(), now)
	if notifier.count() != 1 {
		t.Fatalf("notifier calls = %d, want 1 after first collect", notifier.count())
	}

	// Second collect: same count (3) — no notification.
	collector.Collect(context.Background(), now.Add(time.Second))
	if notifier.count() != 1 {
		t.Fatalf("notifier calls = %d, want still 1 after same count", notifier.count())
	}

	// Third collect: count changed to 5.
	manager.ServerStatusResult = &sessions.ServerStatus{ActiveSessions: 5}
	collector.Collect(context.Background(), now.Add(2*time.Second))
	if notifier.count() != 2 {
		t.Fatalf("notifier calls = %d, want 2 after count increase", notifier.count())
	}
}

func TestSessionCollector_NotifierNotCalledOnError(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 10, 12, 0, 0, 0, time.UTC)
	manager := sessions.NewFakeManager()
	manager.ServerStatusErr = context.DeadlineExceeded

	notifier := &fakeNotifier{}
	collector := NewSessionCollectorWithNotifier(manager, notifier)
	collector.Collect(context.Background(), now)

	if notifier.count() != 0 {
		t.Fatalf("notifier calls = %d, want 0 on error", notifier.count())
	}
}

func TestSessionCollector_NilNotifierIsSafe(t *testing.T) {
	t.Parallel()

	manager := sessions.NewFakeManager()
	manager.ServerStatusResult = &sessions.ServerStatus{ActiveSessions: 2}

	// NewSessionCollectorWithNotifier(provider, nil) must not panic.
	collector := NewSessionCollectorWithNotifier(manager, nil)
	result := collector.Collect(context.Background(), time.Now())
	if result.Status != CollectStatusAvailable {
		t.Fatalf("status = %q, want available", result.Status)
	}
}
```

Also add `"sync"` to the import block in `session_collector_test.go` if not already present.

- [ ] **Step 3.2: Run to confirm they fail**

```bash
cd backend && go test ./internal/monitor/... -run "TestSessionCollector_Notifier|TestSessionCollector_Nil" -v
```

Expected: `undefined: NewSessionCollectorWithNotifier`

- [ ] **Step 3.3: Rewrite `session_collector.go`**

Replace the entire content of `backend/internal/monitor/session_collector.go` with:

```go
package monitor

import (
	"context"
	"time"

	"ocserv_panel/backend/internal/vpn/sessions"
)

// SessionStatusProvider is the subset of ocserv.Manager needed by SessionCollector.
type SessionStatusProvider interface {
	GetServerStatus(ctx context.Context) (*sessions.ServerStatus, error)
}

// SessionChangeNotifier is called when the active session count changes.
// VPNWatcher implements this interface.
type SessionChangeNotifier interface {
	NotifySessionChange()
}

// SessionCollector collects active session counts from ocserv via occtl.
type SessionCollector struct {
	provider  SessionStatusProvider
	notifier  SessionChangeNotifier
	lastCount int
}

var _ Collector = (*SessionCollector)(nil)

// NewSessionCollector returns a collector that queries ocserv for session metrics.
// No session-change notifications are fired.
func NewSessionCollector(provider SessionStatusProvider) *SessionCollector {
	return &SessionCollector{provider: provider}
}

// NewSessionCollectorWithNotifier returns a collector that fires notifier.NotifySessionChange()
// whenever the active session count changes. notifier may be nil.
func NewSessionCollectorWithNotifier(provider SessionStatusProvider, notifier SessionChangeNotifier) *SessionCollector {
	return &SessionCollector{provider: provider, notifier: notifier}
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

	if status.ActiveSessions != c.lastCount {
		c.lastCount = status.ActiveSessions
		if c.notifier != nil {
			c.notifier.NotifySessionChange()
		}
	}

	return available([]Sample{
		{
			Name:      MetricActiveSessions,
			Labels:    map[string]string{},
			Timestamp: now,
			Value:     float64(status.ActiveSessions),
		},
		{
			Name:      MetricConcurrentSessions,
			Labels:    map[string]string{},
			Timestamp: now,
			Value:     float64(status.ActiveSessions),
		},
	})
}
```

- [ ] **Step 3.4: Run all monitor tests**

```bash
cd backend && go test ./internal/monitor/... -v 2>&1 | tail -30
```

Expected: All `PASS`.

- [ ] **Step 3.5: Commit**

```bash
git add backend/internal/monitor/session_collector.go backend/internal/monitor/session_collector_test.go
git commit -m "feat(monitor): add SessionChangeNotifier to SessionCollector"
```

---

## Task 4: Implement `VPNWatcher`

**Files:**
- Create: `backend/internal/monitor/vpn_watcher.go`
- Create: `backend/internal/monitor/vpn_watcher_test.go`

- [ ] **Step 4.1: Write all failing tests first**

Create `backend/internal/monitor/vpn_watcher_test.go`:

```go
package monitor

import (
	"context"
	"sync"
	"testing"
	"time"
)

// fakeCollectorService is a minimal stand-in for CollectorService used in watcher tests.
type fakeCollectorService struct {
	mu       sync.Mutex
	added    []string
	removed  []string
}

func (s *fakeCollectorService) AddCollector(c Collector) {
	if ic, ok := c.(*InterfaceCollector); ok {
		s.mu.Lock()
		defer s.mu.Unlock()
		s.added = append(s.added, ic.Iface())
	}
}

func (s *fakeCollectorService) RemoveCollectorByIface(iface string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.removed = append(s.removed, iface)
}

func (s *fakeCollectorService) addedIfaces() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := make([]string, len(s.added))
	copy(cp, s.added)
	return cp
}

func (s *fakeCollectorService) removedIfaces() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := make([]string, len(s.removed))
	copy(cp, s.removed)
	return cp
}

// fakeFileReaderForVPN is a test double that returns pre-configured file contents.
type fakeFileReaderForVPN struct {
	files map[string]string
}

func (r *fakeFileReaderForVPN) ReadFile(path string) ([]byte, error) {
	data, ok := r.files[path]
	if !ok {
		return nil, fmt.Errorf("file not found: %s", path)
	}
	return []byte(data), nil
}

func newWatcherForTest(
	svc collectorAdder,
	cat *Catalog,
	discover func() []string,
) *VPNWatcher {
	discoverer := func() ([]string, error) { return discover(), nil }
	reader := &fakeFileReaderForVPN{files: map[string]string{
		"/sys/class/net/vpns0/statistics/rx_bytes": "0",
		"/sys/class/net/vpns0/statistics/tx_bytes": "0",
		"/sys/class/net/vpns1/statistics/rx_bytes": "0",
		"/sys/class/net/vpns1/statistics/tx_bytes": "0",
	}}
	return NewVPNWatcher(svc, cat, discoverer, reader,
		MetricVPNInterfaceRXBytes, MetricVPNInterfaceTXBytes, &fakeLogger{})
}

func TestVPNWatcher_AddOnNotification(t *testing.T) {
	t.Parallel()

	svc := &fakeCollectorService{}
	cat := DefaultCatalog()
	watcher := newWatcherForTest(svc, cat, func() []string { return []string{"vpns0"} })

	watcher.syncInterfaces()

	added := svc.addedIfaces()
	if len(added) != 1 || added[0] != "vpns0" {
		t.Fatalf("added = %v, want [vpns0]", added)
	}

	def, _ := cat.Find(MetricVPNInterfaceRXBytes)
	if def.State != StateCollecting {
		t.Fatalf("rx state = %q, want collecting", def.State)
	}
	def, _ = cat.Find(MetricVPNInterfaceTXBytes)
	if def.State != StateCollecting {
		t.Fatalf("tx state = %q, want collecting", def.State)
	}
}

func TestVPNWatcher_RemoveWhenGone(t *testing.T) {
	t.Parallel()

	svc := &fakeCollectorService{}
	cat := DefaultCatalog()

	present := true
	watcher := newWatcherForTest(svc, cat, func() []string {
		if present {
			return []string{"vpns0"}
		}
		return nil
	})

	// First sync: interface appears.
	watcher.syncInterfaces()
	if len(svc.addedIfaces()) != 1 {
		t.Fatal("expected vpns0 added")
	}

	// Second sync: interface gone.
	present = false
	watcher.syncInterfaces()

	removed := svc.removedIfaces()
	if len(removed) != 1 || removed[0] != "vpns0" {
		t.Fatalf("removed = %v, want [vpns0]", removed)
	}
	def, _ := cat.Find(MetricVPNInterfaceRXBytes)
	if def.State != StateUnavailable {
		t.Fatalf("rx state = %q, want unavailable after removal", def.State)
	}
}

func TestVPNWatcher_NoDuplicateOnRepeatNotification(t *testing.T) {
	t.Parallel()

	svc := &fakeCollectorService{}
	cat := DefaultCatalog()
	watcher := newWatcherForTest(svc, cat, func() []string { return []string{"vpns0"} })

	watcher.syncInterfaces()
	watcher.syncInterfaces()
	watcher.syncInterfaces()

	if len(svc.addedIfaces()) != 1 {
		t.Fatalf("added = %v, want exactly one add (no duplicates)", svc.addedIfaces())
	}
}

func TestVPNWatcher_MultiInterfaceLifecycle(t *testing.T) {
	t.Parallel()

	svc := &fakeCollectorService{}
	cat := DefaultCatalog()
	vpns1Present := true
	watcher := newWatcherForTest(svc, cat, func() []string {
		ifaces := []string{"vpns0"}
		if vpns1Present {
			ifaces = append(ifaces, "vpns1")
		}
		return ifaces
	})

	// Both interfaces appear.
	watcher.syncInterfaces()
	if len(svc.addedIfaces()) != 2 {
		t.Fatalf("added = %v, want [vpns0, vpns1]", svc.addedIfaces())
	}

	// vpns1 disconnects.
	vpns1Present = false
	watcher.syncInterfaces()

	removed := svc.removedIfaces()
	if len(removed) != 1 || removed[0] != "vpns1" {
		t.Fatalf("removed = %v, want [vpns1]", removed)
	}
}

func TestVPNWatcher_ContextCancelStopsLoop(t *testing.T) {
	t.Parallel()

	svc := &fakeCollectorService{}
	cat := DefaultCatalog()
	watcher := newWatcherForTest(svc, cat, func() []string { return nil })
	// Use a very long ticker interval so the test doesn't rely on ticker fires.
	watcher.tickInterval = 10 * time.Minute

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		watcher.Run(ctx)
		close(done)
	}()

	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Run did not stop after context cancellation")
	}
}

func TestVPNWatcher_NotifySessionChangeSendsSignal(t *testing.T) {
	t.Parallel()

	svc := &fakeCollectorService{}
	cat := DefaultCatalog()
	syncCount := 0
	watcher := newWatcherForTest(svc, cat, func() []string {
		syncCount++
		return nil
	})

	// Non-blocking: can be called multiple times without blocking.
	watcher.NotifySessionChange()
	watcher.NotifySessionChange()
	watcher.NotifySessionChange()

	// Channel is buffered(1); at most one pending signal.
	if len(watcher.notify) > 1 {
		t.Fatalf("notify channel length = %d, want at most 1", len(watcher.notify))
	}
}

func TestVPNWatcher_DiscoveryErrorLogsWarn(t *testing.T) {
	t.Parallel()

	svc := &fakeCollectorService{}
	cat := DefaultCatalog()
	logger := &fakeLogger{}

	watcher := NewVPNWatcher(
		svc, cat,
		func() ([]string, error) { return nil, fmt.Errorf("sysfs unavailable") },
		&fakeFileReaderForVPN{files: map[string]string{}},
		MetricVPNInterfaceRXBytes, MetricVPNInterfaceTXBytes,
		logger,
	)

	watcher.syncInterfaces()

	if !logger.hasWarnContaining("sysfs unavailable") {
		t.Fatalf("warn logs = %v, expected sysfs unavailable", logger.warn)
	}
}
```

Also add `"fmt"` to the imports of `vpn_watcher_test.go`.

- [ ] **Step 4.2: Run to confirm they fail**

```bash
cd backend && go test ./internal/monitor/... -run "TestVPNWatcher" -v
```

Expected: `undefined: VPNWatcher`

- [ ] **Step 4.3: Implement `vpn_watcher.go`**

Create `backend/internal/monitor/vpn_watcher.go`:

```go
package monitor

import (
	"context"
	"sync"
	"time"
)

const vpnWatcherDefaultTickInterval = 15 * time.Second

// collectorAdder is the subset of CollectorService used by VPNWatcher.
// Defined here so tests can inject a fake without depending on CollectorService.
type collectorAdder interface {
	AddCollector(c Collector)
	RemoveCollectorByIface(iface string)
}

// VPNWatcher listens for session-change notifications and sysfs poll ticks,
// then diffs the tracked VPN interface set to add/remove InterfaceCollectors
// and update the metric catalog accordingly.
type VPNWatcher struct {
	service      collectorAdder
	catalog      *Catalog
	discoverer   VPNMultiDiscovererFunc
	reader       FileReader
	rxMetric     MetricName
	txMetric     MetricName
	logger       Logger
	notify       chan struct{}
	tickInterval time.Duration

	mu      sync.Mutex
	tracked map[string]bool
}

// NewVPNWatcher constructs a VPNWatcher. Call Run(ctx) in a goroutine.
func NewVPNWatcher(
	service collectorAdder,
	catalog *Catalog,
	discoverer VPNMultiDiscovererFunc,
	reader FileReader,
	rxMetric, txMetric MetricName,
	logger Logger,
) *VPNWatcher {
	if logger == nil {
		logger = noopLogger{}
	}
	return &VPNWatcher{
		service:      service,
		catalog:      catalog,
		discoverer:   discoverer,
		reader:       reader,
		rxMetric:     rxMetric,
		txMetric:     txMetric,
		logger:       logger,
		notify:       make(chan struct{}, 1),
		tickInterval: vpnWatcherDefaultTickInterval,
		tracked:      make(map[string]bool),
	}
}

// NotifySessionChange signals the watcher that the active session count changed.
// Non-blocking: if a signal is already pending, this is a no-op.
// Satisfies SessionChangeNotifier.
func (w *VPNWatcher) NotifySessionChange() {
	select {
	case w.notify <- struct{}{}:
	default:
	}
}

// Run starts the watcher loop. Blocks until ctx is cancelled.
func (w *VPNWatcher) Run(ctx context.Context) {
	ticker := time.NewTicker(w.tickInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-w.notify:
			w.syncInterfaces()
		case <-ticker.C:
			w.syncInterfaces()
		}
	}
}

// syncInterfaces discovers current VPN interfaces and reconciles the tracked set.
func (w *VPNWatcher) syncInterfaces() {
	discovered, err := w.discoverer()
	if err != nil {
		w.logger.Warn("vpn interface discovery error", "error", err)
		return
	}

	discoveredSet := make(map[string]bool, len(discovered))
	for _, iface := range discovered {
		discoveredSet[iface] = true
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	// Add newly discovered interfaces.
	for iface := range discoveredSet {
		if w.tracked[iface] {
			continue
		}
		w.logger.Debug("vpn interface appeared", "interface", iface)
		w.service.AddCollector(NewInterfaceCollector(w.reader, iface, w.rxMetric, w.txMetric))
		w.tracked[iface] = true
	}

	// Remove interfaces that are no longer present.
	for iface := range w.tracked {
		if discoveredSet[iface] {
			continue
		}
		w.logger.Debug("vpn interface disappeared", "interface", iface)
		w.service.RemoveCollectorByIface(iface)
		delete(w.tracked, iface)
	}

	// Update catalog state based on whether any VPN interface is active.
	if len(w.tracked) > 0 {
		w.catalog.SetState(w.rxMetric, StateCollecting, "")
		w.catalog.SetState(w.txMetric, StateCollecting, "")
	} else {
		w.catalog.SetState(w.rxMetric, StateUnavailable, "no active VPN interface")
		w.catalog.SetState(w.txMetric, StateUnavailable, "no active VPN interface")
	}
}
```

- [ ] **Step 4.4: Run all watcher tests**

```bash
cd backend && go test ./internal/monitor/... -run "TestVPNWatcher" -v
```

Expected: All `PASS`.

- [ ] **Step 4.5: Run full monitor package**

```bash
cd backend && go test ./internal/monitor/... -v 2>&1 | tail -20
```

Expected: All `PASS`, no compilation errors.

- [ ] **Step 4.6: Commit**

```bash
git add backend/internal/monitor/vpn_watcher.go backend/internal/monitor/vpn_watcher_test.go
git commit -m "feat(monitor): implement VPNWatcher with session-triggered interface discovery"
```

---

## Task 5: Fix `DefaultCatalog` VPN metric initial state

**Files:**
- Modify: `backend/internal/monitor/types.go:79-80`

- [ ] **Step 5.1: Write a failing test for the initial catalog state**

Add to `backend/internal/monitor/service_test.go` (or a new file; add in `service_test.go` for proximity):

```go
func TestDefaultCatalog_VPNMetricsStartUnavailable(t *testing.T) {
	t.Parallel()

	cat := DefaultCatalog()

	rx, ok := cat.Find(MetricVPNInterfaceRXBytes)
	if !ok {
		t.Fatal("MetricVPNInterfaceRXBytes not in catalog")
	}
	if rx.State != StateUnavailable {
		t.Fatalf("rx state = %q, want unavailable at startup", rx.State)
	}

	tx, ok := cat.Find(MetricVPNInterfaceTXBytes)
	if !ok {
		t.Fatal("MetricVPNInterfaceTXBytes not in catalog")
	}
	if tx.State != StateUnavailable {
		t.Fatalf("tx state = %q, want unavailable at startup", tx.State)
	}
}
```

- [ ] **Step 5.2: Run to confirm it fails**

```bash
cd backend && go test ./internal/monitor/... -run TestDefaultCatalog_VPNMetricsStartUnavailable -v
```

Expected: `FAIL` — state is `collecting` not `unavailable`.

- [ ] **Step 5.3: Fix `DefaultCatalog` in `types.go`**

In `backend/internal/monitor/types.go`, change lines 79-80 from:

```go
		collectingMetric(MetricVPNInterfaceRXBytes, "VPN interface received bytes"),
		collectingMetric(MetricVPNInterfaceTXBytes, "VPN interface transmitted bytes"),
```

to:

```go
		unavailableMetric(MetricVPNInterfaceRXBytes, "VPN interface received bytes"),
		unavailableMetric(MetricVPNInterfaceTXBytes, "VPN interface transmitted bytes"),
```

But `unavailableMetric` currently uses a hard-coded reason `"ocserv collection is not implemented yet"`. Update `unavailableMetric` to accept a reason, or use a specific call. The cleanest change: add an `unavailableWithReason` helper alongside the existing helpers:

In `backend/internal/monitor/types.go` (near the bottom), add:

```go
func unavailableWithReason(name MetricName, description, reason string) MetricDefinition {
	return MetricDefinition{
		Name:        name,
		Description: description,
		State:       StateUnavailable,
		Reason:      reason,
	}
}
```

Then in `DefaultCatalog`:

```go
		unavailableWithReason(MetricVPNInterfaceRXBytes, "VPN interface received bytes", "waiting for VPN interface"),
		unavailableWithReason(MetricVPNInterfaceTXBytes, "VPN interface transmitted bytes", "waiting for VPN interface"),
```

- [ ] **Step 5.4: Run to confirm pass**

```bash
cd backend && go test ./internal/monitor/... -run TestDefaultCatalog_VPNMetricsStartUnavailable -v
```

Expected: `PASS`

- [ ] **Step 5.5: Run full monitor suite**

```bash
cd backend && go test ./internal/monitor/... -v 2>&1 | tail -20
```

Expected: All `PASS`.

- [ ] **Step 5.6: Commit**

```bash
git add backend/internal/monitor/types.go backend/internal/monitor/service_test.go
git commit -m "fix(monitor): VPN metrics start unavailable in DefaultCatalog"
```

---

## Task 6: Wire `VPNWatcher` in `main.go`; remove obsolete VPN config path

**Files:**
- Modify: `backend/cmd/ocserv-panel/main.go`

- [ ] **Step 6.1: Replace the VPN wiring block**

In `backend/cmd/ocserv-panel/main.go`, locate the block from line ~199 (session collector + VPN config):

```go
	// Add session collector using the ocserv session manager.
	sessionCollector := monitor.NewSessionCollector(sessionManager)
	collectors = append(collectors, sessionCollector)
	metricCatalog.SetState(monitor.MetricActiveSessions, monitor.StateCollecting, "")
	metricCatalog.SetState(monitor.MetricConcurrentSessions, monitor.StateCollecting, "")

	// Use config-based collector interval with fallback.
	collectorInterval := time.Duration(cfg.Monitoring.CollectorInterval) * time.Second
	if collectorInterval <= 0 {
		collectorInterval = 15 * time.Second
	}

	// Check if VPN is already configured (from config or env).
	vpnConfigured := strings.TrimSpace(cfg.Monitoring.VPNInterface) != "" || strings.TrimSpace(os.Getenv("OCSERV_PANEL_VPN_IFACE")) != ""

	collectorService := monitor.NewCollectorService(
		metricStore,
		collectors,
		collectorInterval,
		zerologAdapter{},
		nil,
	)

	if vpnConfigured {
		vpnIface := strings.TrimSpace(cfg.Monitoring.VPNInterface)
		if vpnIface == "" {
			vpnIface = strings.TrimSpace(os.Getenv("OCSERV_PANEL_VPN_IFACE"))
		}
		collectorService.MarkVPNPresent(vpnIface)
		metricCatalog.SetState(monitor.MetricVPNInterfaceRXBytes, monitor.StateCollecting, "")
		metricCatalog.SetState(monitor.MetricVPNInterfaceTXBytes, monitor.StateCollecting, "")
	} else {
		// Enable periodic re-discovery of VPN interface.
		collectorService.SetVPNDiscoverer(
			func() ([]string, error) { return monitor.DiscoverAllVPNInterfaces() },
			monitor.OSReader{},
			monitor.MetricVPNInterfaceRXBytes,
			monitor.MetricVPNInterfaceTXBytes,
		)
	}
	go collectorService.Run(ctx)
```

Replace it with:

```go
	// Use config-based collector interval with fallback.
	collectorInterval := time.Duration(cfg.Monitoring.CollectorInterval) * time.Second
	if collectorInterval <= 0 {
		collectorInterval = 15 * time.Second
	}

	collectorService := monitor.NewCollectorService(
		metricStore,
		collectors,
		collectorInterval,
		zerologAdapter{},
		nil,
	)

	vpnWatcher := monitor.NewVPNWatcher(
		collectorService,
		metricCatalog,
		func() ([]string, error) { return monitor.DiscoverAllVPNInterfaces() },
		monitor.OSReader{},
		monitor.MetricVPNInterfaceRXBytes,
		monitor.MetricVPNInterfaceTXBytes,
		zerologAdapter{},
	)

	// Add session collector using the ocserv session manager.
	// NewSessionCollectorWithNotifier fires vpnWatcher.NotifySessionChange() on count change.
	sessionCollector := monitor.NewSessionCollectorWithNotifier(sessionManager, vpnWatcher)
	collectorService.AddCollector(sessionCollector)
	metricCatalog.SetState(monitor.MetricActiveSessions, monitor.StateCollecting, "")
	metricCatalog.SetState(monitor.MetricConcurrentSessions, monitor.StateCollecting, "")

	go vpnWatcher.Run(ctx)
	go collectorService.Run(ctx)
```

- [ ] **Step 6.2: Remove the VPN branch from `configuredCollectors`**

In `backend/cmd/ocserv-panel/main.go`, inside `configuredCollectors`, remove the entire VPN block (the comment + the `vpnIface` variable + the `if vpnIface != ""` block):

```go
	// VPN interface: config > env > (auto-discovery handled later via re-discovery)
	vpnIface := strings.TrimSpace(monCfg.VPNInterface)
	if vpnIface == "" {
		vpnIface = strings.TrimSpace(os.Getenv("OCSERV_PANEL_VPN_IFACE"))
	}
	if vpnIface != "" {
		collectors = append(collectors, monitor.NewInterfaceCollector(
			reader,
			vpnIface,
			monitor.MetricVPNInterfaceRXBytes,
			monitor.MetricVPNInterfaceTXBytes,
		))
	}
```

Delete those lines entirely. VPN collectors are now managed exclusively by `VPNWatcher`.

- [ ] **Step 6.3: Remove the unused `vpnConfigured` reference and clean up imports if needed**

Remove any now-unused imports. Likely `"strings"` is still used for network interface discovery; verify with the compiler.

- [ ] **Step 6.4: Build to confirm no compilation errors**

```bash
cd backend && go build ./...
```

Expected: No errors.

- [ ] **Step 6.5: Run full test suite**

```bash
cd backend && go test ./... 2>&1 | grep -E "FAIL|ok"
```

Expected: All packages show `ok`, no `FAIL`.

- [ ] **Step 6.6: Commit**

```bash
git add backend/cmd/ocserv-panel/main.go
git commit -m "feat(main): wire VPNWatcher; remove vpnConfigured branch from main.go"
```

---

## Task 7: Final verification

- [ ] **Step 7.1: Run the full test suite with race detector**

```bash
cd backend && go test -race ./... 2>&1 | grep -E "FAIL|ok|DATA RACE"
```

Expected: All `ok`, no `DATA RACE`, no `FAIL`.

- [ ] **Step 7.2: Confirm monitor package test count increased**

```bash
cd backend && go test ./internal/monitor/... -v 2>&1 | grep -c "^--- PASS"
```

The count should be higher than before this feature (was ~30 passing tests in the monitor package; new tests in Tasks 1–5 add at least 10 more).

- [ ] **Step 7.3: Commit summary**

```bash
git log --oneline -6
```

Expected output (6 commits from this feature):
```
<hash> feat(main): wire VPNWatcher; remove vpnConfigured branch from main.go
<hash> fix(monitor): VPN metrics start unavailable in DefaultCatalog
<hash> feat(monitor): implement VPNWatcher with session-triggered interface discovery
<hash> feat(monitor): add SessionChangeNotifier to SessionCollector
<hash> refactor(monitor): replace VPN discoverer with AddCollector/RemoveCollectorByIface
<hash> feat(monitor): add Iface() getter to InterfaceCollector
```
