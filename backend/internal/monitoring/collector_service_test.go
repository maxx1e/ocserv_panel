package monitoring

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"ocserv_panel/backend/internal/metrics"
)

func TestCollectorService_CollectOnceAppendsAvailableSamples(t *testing.T) {
	store := metrics.NewMemoryStore()
	now := time.Date(2026, 4, 30, 12, 0, 0, 0, time.UTC)
	service := NewCollectorService(
		store,
		[]Collector{fakeCollector{result: available([]metrics.Sample{sampleAt(now, 42)})}},
		time.Second,
		&fakeLogger{},
		func() time.Time { return now },
	)

	service.CollectOnce(context.Background())

	series, err := store.QueryRange(context.Background(), metrics.Query{
		Metric: metrics.MetricHostCPUUtilization,
		Start:  now.Add(-time.Second),
		End:    now.Add(time.Second),
		Step:   time.Second,
	})
	if err != nil {
		t.Fatalf("query samples: %v", err)
	}
	if len(series.Points) != 1 {
		t.Fatalf("stored points = %d, want 1", len(series.Points))
	}
	if series.Points[0].Value != 42 {
		t.Fatalf("stored value = %v, want 42", series.Points[0].Value)
	}
}

func TestCollectorService_CollectOnceSkipsUnavailableAndLogsDebug(t *testing.T) {
	store := metrics.NewMemoryStore()
	logger := &fakeLogger{}
	now := time.Date(2026, 4, 30, 12, 0, 0, 0, time.UTC)
	service := NewCollectorService(
		store,
		[]Collector{fakeCollector{result: unavailable("cpu missing")}},
		time.Second,
		logger,
		func() time.Time { return now },
	)

	service.CollectOnce(context.Background())

	series, err := store.QueryRange(context.Background(), metrics.Query{
		Metric: metrics.MetricHostCPUUtilization,
		Start:  now.Add(-time.Second),
		End:    now.Add(time.Second),
		Step:   time.Second,
	})
	if err != nil {
		t.Fatalf("query samples: %v", err)
	}
	if len(series.Points) != 0 {
		t.Fatalf("stored points = %d, want 0", len(series.Points))
	}
	if !logger.hasDebugContaining("cpu missing") {
		t.Fatalf("debug logs = %#v, want message containing cpu missing", logger.debug)
	}
}

func TestCollectorService_CollectOnceAvailableWithZeroSamplesIsNoop(t *testing.T) {
	store := &failingStore{}
	logger := &fakeLogger{}
	service := NewCollectorService(
		store,
		[]Collector{fakeCollector{result: available([]metrics.Sample{})}},
		time.Second,
		logger,
		func() time.Time { return time.Now() },
	)

	service.CollectOnce(context.Background())

	if got := store.appendCount(); got != 0 {
		t.Fatalf("append calls = %d, want 0", got)
	}
	if got := logger.debugCount(); got != 0 {
		t.Fatalf("debug logs = %d, want 0", got)
	}
	if got := logger.warnCount(); got != 0 {
		t.Fatalf("warn logs = %d, want 0", got)
	}
}

func TestCollectorService_CollectOnceWarnsAndContinuesAfterAppendFailure(t *testing.T) {
	store := &failingStore{failValues: map[float64]bool{2: true}}
	logger := &fakeLogger{}
	now := time.Date(2026, 4, 30, 12, 0, 0, 0, time.UTC)
	service := NewCollectorService(
		store,
		[]Collector{fakeCollector{result: available([]metrics.Sample{
			sampleAt(now, 1),
			sampleAt(now, 2),
			sampleAt(now, 3),
		})}},
		time.Second,
		logger,
		func() time.Time { return now },
	)

	service.CollectOnce(context.Background())

	if got, want := store.values(), []float64{1, 3}; !equalFloat64s(got, want) {
		t.Fatalf("appended values = %#v, want %#v", got, want)
	}
	if !logger.hasWarnContaining("append metric sample") {
		t.Fatalf("warn logs = %#v, want append warning", logger.warn)
	}
}

func TestCollectorService_CollectOnceContinuesToLaterCollectorsAfterAppendFailure(t *testing.T) {
	store := &failingStore{failValues: map[float64]bool{2: true}}
	logger := &fakeLogger{}
	now := time.Date(2026, 4, 30, 12, 0, 0, 0, time.UTC)
	service := NewCollectorService(
		store,
		[]Collector{
			fakeCollector{result: available([]metrics.Sample{sampleAt(now, 2)})},
			fakeCollector{result: available([]metrics.Sample{sampleAt(now, 4)})},
		},
		time.Second,
		logger,
		func() time.Time { return now },
	)

	service.CollectOnce(context.Background())

	if got, want := store.values(), []float64{4}; !equalFloat64s(got, want) {
		t.Fatalf("appended values = %#v, want %#v", got, want)
	}
	if !logger.hasWarnContaining("append metric sample") {
		t.Fatalf("warn logs = %#v, want append warning", logger.warn)
	}
}

func TestCollectorService_CollectOnceSkipsCollectorsWhenContextCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	collector := &countingCollector{}
	service := NewCollectorService(
		metrics.NewMemoryStore(),
		[]Collector{collector},
		time.Second,
		&fakeLogger{},
		func() time.Time { return time.Now() },
	)

	service.CollectOnce(ctx)

	if got := collector.count(); got != 0 {
		t.Fatalf("collector calls = %d, want 0", got)
	}
}

func TestNewCollectorServiceDefaultsAndCopiesCollectors(t *testing.T) {
	collector := &countingCollector{}
	collectors := []Collector{collector}
	service := NewCollectorService(metrics.NewMemoryStore(), collectors, 0, nil, nil)
	collectors[0] = fakeCollector{result: available(nil)}

	if service.interval != 15*time.Second {
		t.Fatalf("interval = %v, want 15s", service.interval)
	}
	service.CollectOnce(context.Background())
	if got := collector.count(); got != 1 {
		t.Fatalf("original collector calls = %d, want 1", got)
	}
}

func TestNewCollectorServiceDefaultClockUsesCurrentTime(t *testing.T) {
	collector := &timeRecordingCollector{}
	service := NewCollectorService(
		metrics.NewMemoryStore(),
		[]Collector{collector},
		time.Second,
		&fakeLogger{},
		nil,
	)

	before := time.Now()
	service.CollectOnce(context.Background())
	after := time.Now()

	collectedAt, ok := collector.collectedAt()
	if !ok {
		t.Fatal("collector did not record collection time")
	}
	if collectedAt.Before(before) || collectedAt.After(after) {
		t.Fatalf("collection time = %v, want between %v and %v", collectedAt, before, after)
	}
}

func TestNewCollectorServiceDefaultLoggerIsNoopForLoggingPath(t *testing.T) {
	service := NewCollectorService(
		metrics.NewMemoryStore(),
		[]Collector{fakeCollector{result: unavailable("not available")}},
		time.Second,
		nil,
		func() time.Time { return time.Now() },
	)

	service.CollectOnce(context.Background())
}

func TestCollectorService_RunCollectsImmediatelyAndStopsOnCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	collector := &signalCollector{collected: make(chan struct{}, 1)}
	service := NewCollectorService(
		metrics.NewMemoryStore(),
		[]Collector{collector},
		time.Hour,
		&fakeLogger{},
		func() time.Time { return time.Now() },
	)
	done := make(chan struct{})

	go func() {
		service.Run(ctx)
		close(done)
	}()

	select {
	case <-collector.collected:
	case <-time.After(time.Second):
		t.Fatal("Run did not collect immediately")
	}
	cancel()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Run did not stop after context cancellation")
	}
}

func TestCollectorService_VPNRediscovery(t *testing.T) {
	store := metrics.NewMemoryStore()
	logger := &fakeLogger{}
	now := time.Date(2026, 5, 9, 12, 0, 0, 0, time.UTC)

	service := NewCollectorService(
		store,
		[]Collector{},
		50*time.Millisecond,
		logger,
		func() time.Time { return now },
	)

	var mu sync.Mutex
	callCount := 0
	discoverer := func() ([]string, error) {
		mu.Lock()
		defer mu.Unlock()
		callCount++
		if callCount <= 2 {
			return nil, nil
		}
		return []string{"vpns0"}, nil
	}

	fakeReader := &fakeFileReaderForVPN{
		files: map[string]string{
			"/sys/class/net/vpns0/statistics/rx_bytes": "1000",
			"/sys/class/net/vpns0/statistics/tx_bytes": "2000",
		},
	}

	service.SetVPNDiscoverer(
		discoverer,
		fakeReader,
		metrics.MetricVPNInterfaceRXBytes,
		metrics.MetricVPNInterfaceTXBytes,
	)

	// Override the vpnRediscoveryInterval by running maybeDiscoverAllVPN manually
	// in a goroutine loop with short sleeps to keep the test fast.
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	done := make(chan struct{})
	go func() {
		// Simulate periodic VPN discovery at a fast rate.
		ticker := time.NewTicker(30 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				close(done)
				return
			case <-ticker.C:
				service.maybeDiscoverAllVPN()
			}
		}
	}()

	<-done

	if !service.HasVPNCollector() {
		t.Fatal("expected HasVPNCollector() to be true after discovery")
	}

	mu.Lock()
	finalCount := callCount
	mu.Unlock()
	if finalCount < 3 {
		t.Fatalf("discoverer called %d times, expected at least 3", finalCount)
	}
}

func TestCollectorService_VPNRediscoverySkipsWhenAlreadyPresent(t *testing.T) {
	store := metrics.NewMemoryStore()
	logger := &fakeLogger{}
	now := time.Date(2026, 5, 9, 12, 0, 0, 0, time.UTC)

	service := NewCollectorService(
		store,
		[]Collector{},
		50*time.Millisecond,
		logger,
		func() time.Time { return now },
	)

	var mu sync.Mutex
	callCount := 0
	discoverer := func() ([]string, error) {
		mu.Lock()
		defer mu.Unlock()
		callCount++
		return []string{"vpns0"}, nil
	}

	fakeReader := &fakeFileReaderForVPN{
		files: map[string]string{
			"/sys/class/net/vpns0/statistics/rx_bytes": "1000",
			"/sys/class/net/vpns0/statistics/tx_bytes": "2000",
		},
	}

	service.SetVPNDiscoverer(
		discoverer,
		fakeReader,
		metrics.MetricVPNInterfaceRXBytes,
		metrics.MetricVPNInterfaceTXBytes,
	)

	// Call maybeDiscoverAllVPN multiple times.
	for i := 0; i < 5; i++ {
		service.maybeDiscoverAllVPN()
	}

	if !service.HasVPNCollector() {
		t.Fatal("expected HasVPNCollector() to be true")
	}

	// The discoverer is called every time (no early exit), but only one
	// collector should be added because vpns0 is already registered.
	mu.Lock()
	finalCount := callCount
	mu.Unlock()
	if finalCount != 5 {
		t.Fatalf("discoverer called %d times, expected 5", finalCount)
	}
	if len(service.collectors) != 1 {
		t.Fatalf("collector count = %d, want 1 (no duplicates)", len(service.collectors))
	}
}

func TestCollectorService_MarkVPNPresentPreventsDiscovery(t *testing.T) {
	store := metrics.NewMemoryStore()
	logger := &fakeLogger{}

	service := NewCollectorService(
		store,
		[]Collector{},
		50*time.Millisecond,
		logger,
		func() time.Time { return time.Now() },
	)

	var mu sync.Mutex
	callCount := 0
	discoverer := func() ([]string, error) {
		mu.Lock()
		defer mu.Unlock()
		callCount++
		return []string{"vpns0"}, nil
	}

	fakeReader := &fakeFileReaderForVPN{
		files: map[string]string{
			"/sys/class/net/vpns0/statistics/rx_bytes": "1000",
			"/sys/class/net/vpns0/statistics/tx_bytes": "2000",
		},
	}

	service.SetVPNDiscoverer(
		discoverer,
		fakeReader,
		metrics.MetricVPNInterfaceRXBytes,
		metrics.MetricVPNInterfaceTXBytes,
	)

	// Mark VPN as already present (simulating startup discovery).
	service.MarkVPNPresent("vpns0")

	// Call maybeDiscoverAllVPN — the discoverer will be called but vpns0 is
	// already registered so no new collectors should be added.
	for i := 0; i < 3; i++ {
		service.maybeDiscoverAllVPN()
	}

	mu.Lock()
	finalCount := callCount
	mu.Unlock()
	// The discoverer IS called (no early exit), but vpns0 is already in discoveredVPN.
	if finalCount != 3 {
		t.Fatalf("discoverer called %d times, expected 3", finalCount)
	}
	// No new collectors should have been added.
	if len(service.collectors) != 0 {
		t.Fatalf("collector count = %d, expected 0 (vpns0 already marked)", len(service.collectors))
	}
}

func TestCollectorService_CollectOnceThreadSafeWithDiscovery(t *testing.T) {
	store := metrics.NewMemoryStore()
	logger := &fakeLogger{}
	now := time.Date(2026, 5, 9, 12, 0, 0, 0, time.UTC)

	collector := &countingCollector{}
	service := NewCollectorService(
		store,
		[]Collector{collector},
		50*time.Millisecond,
		logger,
		func() time.Time { return now },
	)

	// Run CollectOnce concurrently with maybeDiscoverAllVPN to verify no race.
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	fakeReader := &fakeFileReaderForVPN{
		files: map[string]string{
			"/sys/class/net/vpns0/statistics/rx_bytes": "1000",
			"/sys/class/net/vpns0/statistics/tx_bytes": "2000",
		},
	}

	callNum := 0
	service.SetVPNDiscoverer(
		func() ([]string, error) {
			callNum++
			if callNum > 3 {
				return []string{"vpns0"}, nil
			}
			return nil, nil
		},
		fakeReader,
		metrics.MetricVPNInterfaceRXBytes,
		metrics.MetricVPNInterfaceTXBytes,
	)

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		for {
			select {
			case <-ctx.Done():
				return
			default:
				service.CollectOnce(ctx)
				time.Sleep(10 * time.Millisecond)
			}
		}
	}()

	go func() {
		defer wg.Done()
		for {
			select {
			case <-ctx.Done():
				return
			default:
				service.maybeDiscoverAllVPN()
				time.Sleep(15 * time.Millisecond)
			}
		}
	}()

	wg.Wait()

	if collector.count() == 0 {
		t.Fatal("expected at least one collect call")
	}
}

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

type fakeCollector struct {
	result CollectResult
}

func (c fakeCollector) Collect(context.Context, time.Time) CollectResult {
	return c.result
}

type countingCollector struct {
	mu    sync.Mutex
	calls int
}

func (c *countingCollector) Collect(context.Context, time.Time) CollectResult {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.calls++

	return available(nil)
}

func (c *countingCollector) count() int {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.calls
}

type signalCollector struct {
	collected chan struct{}
}

func (c *signalCollector) Collect(context.Context, time.Time) CollectResult {
	select {
	case c.collected <- struct{}{}:
	default:
	}

	return available(nil)
}

type timeRecordingCollector struct {
	mu  sync.Mutex
	now time.Time
	ok  bool
}

func (c *timeRecordingCollector) Collect(_ context.Context, now time.Time) CollectResult {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = now
	c.ok = true

	return available(nil)
}

func (c *timeRecordingCollector) collectedAt() (time.Time, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.now, c.ok
}

type fakeLogger struct {
	mu    sync.Mutex
	debug []string
	warn  []string
}

func (l *fakeLogger) Debug(msg string, args ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.debug = append(l.debug, logLine(msg, args...))
}

func (l *fakeLogger) Warn(msg string, args ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.warn = append(l.warn, logLine(msg, args...))
}

func (l *fakeLogger) hasDebugContaining(value string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	return containsLog(l.debug, value)
}

func (l *fakeLogger) hasWarnContaining(value string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	return containsLog(l.warn, value)
}

func (l *fakeLogger) debugCount() int {
	l.mu.Lock()
	defer l.mu.Unlock()

	return len(l.debug)
}

func (l *fakeLogger) warnCount() int {
	l.mu.Lock()
	defer l.mu.Unlock()

	return len(l.warn)
}

type failingStore struct {
	mu         sync.Mutex
	failValues map[float64]bool
	samples    []metrics.Sample
	appends    int
}

func (s *failingStore) Append(ctx context.Context, samples []metrics.Sample) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.appends++
	for _, sample := range samples {
		if s.failValues[sample.Value] {
			return errors.New("append failed")
		}
	}
	s.samples = append(s.samples, samples...)

	return nil
}

func (s *failingStore) QueryRange(context.Context, metrics.Query) (metrics.Series, error) {
	return metrics.Series{}, nil
}

func (s *failingStore) Close() error {
	return nil
}

func (s *failingStore) values() []float64 {
	s.mu.Lock()
	defer s.mu.Unlock()

	values := make([]float64, 0, len(s.samples))
	for _, sample := range s.samples {
		values = append(values, sample.Value)
	}

	return values
}

func (s *failingStore) appendCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.appends
}

func sampleAt(timestamp time.Time, value float64) metrics.Sample {
	return metrics.Sample{
		Name:      metrics.MetricHostCPUUtilization,
		Labels:    map[string]string{},
		Timestamp: timestamp,
		Value:     value,
	}
}

func logLine(msg string, args ...any) string {
	var builder strings.Builder
	builder.WriteString(msg)
	for _, arg := range args {
		builder.WriteString(" ")
		builder.WriteString(fmt.Sprint(arg))
	}

	return builder.String()
}

func containsLog(logs []string, value string) bool {
	for _, line := range logs {
		if strings.Contains(line, value) {
			return true
		}
	}

	return false
}

func equalFloat64s(got []float64, want []float64) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}

	return true
}
