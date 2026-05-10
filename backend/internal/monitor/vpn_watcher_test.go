package monitor

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"
)

// fakeCollectorService is a minimal stand-in for CollectorService used in watcher tests.
type fakeCollectorService struct {
	mu      sync.Mutex
	added   []string
	removed []string
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
	watcher := newWatcherForTest(svc, cat, func() []string {
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
