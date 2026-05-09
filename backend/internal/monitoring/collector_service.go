package monitoring

import (
	"context"
	"sync"
	"time"

	"ocserv_panel/backend/internal/metrics"
)

const (
	defaultCollectorInterval = 15 * time.Second
	vpnRediscoveryInterval   = 60 * time.Second
)

// VPNDiscovererFunc attempts to find a VPN interface.
// It returns (interfaceName, source, error). An empty interfaceName
// indicates no VPN interface was found.
type VPNDiscovererFunc func() (string, string, error)

type Logger interface {
	Debug(msg string, args ...any)
	Warn(msg string, args ...any)
}

type CollectorService struct {
	store    metrics.Store
	interval time.Duration
	logger   Logger
	now      func() time.Time

	mu             sync.Mutex
	collectors     []Collector
	vpnDiscoverer  VPNDiscovererFunc
	vpnReader      FileReader
	vpnRXMetric    metrics.MetricName
	vpnTXMetric    metrics.MetricName
	hasVPN         bool
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

	clonedCollectors := make([]Collector, len(collectors))
	copy(clonedCollectors, collectors)

	return &CollectorService{
		store:      store,
		collectors: clonedCollectors,
		interval:   interval,
		logger:     logger,
		now:        now,
	}
}

// SetVPNDiscoverer configures periodic VPN interface re-discovery.
// When set, the Run loop will periodically call fn to look for a VPN
// interface and, if found, create a new InterfaceCollector for it.
func (s *CollectorService) SetVPNDiscoverer(
	fn VPNDiscovererFunc,
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

// HasVPNCollector reports whether a VPN interface collector has been added.
func (s *CollectorService) HasVPNCollector() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.hasVPN
}

// MarkVPNPresent marks the VPN collector as already present, for example
// when a VPN interface was discovered at startup before the Run loop.
func (s *CollectorService) MarkVPNPresent() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.hasVPN = true
}

func (s *CollectorService) CollectOnce(ctx context.Context) {
	if err := ctx.Err(); err != nil {
		return
	}

	// Snapshot collectors under lock to avoid races with maybeDiscoverVPN.
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

	collectTicker := time.NewTicker(s.interval)
	defer collectTicker.Stop()

	// Only create a VPN ticker if a discoverer is configured.
	var vpnTicker *time.Ticker
	s.mu.Lock()
	hasDiscoverer := s.vpnDiscoverer != nil
	s.mu.Unlock()
	if hasDiscoverer {
		vpnTicker = time.NewTicker(vpnRediscoveryInterval)
		defer vpnTicker.Stop()
	}

	s.CollectOnce(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-collectTicker.C:
			s.CollectOnce(ctx)
		case <-vpnTickerChan(vpnTicker):
			s.maybeDiscoverVPN()
		}
	}
}

// maybeDiscoverVPN checks for a VPN interface and adds a collector if found.
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
	if err != nil {
		s.logger.Warn("vpn discovery error", "error", err)
		return
	}
	if iface == "" {
		s.logger.Debug("vpn discovery found no interface")
		return
	}

	s.logger.Debug("vpn interface discovered", "interface", iface, "source", source)

	collector := NewInterfaceCollector(reader, iface, rxMetric, txMetric)

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.hasVPN {
		return // Another goroutine already added it.
	}
	s.collectors = append(s.collectors, collector)
	s.hasVPN = true
}

// vpnTickerChan returns the ticker channel or nil if the ticker is nil.
// A nil channel blocks forever in select, effectively disabling that case.
func vpnTickerChan(t *time.Ticker) <-chan time.Time {
	if t == nil {
		return nil
	}
	return t.C
}

type noopLogger struct{}

func (noopLogger) Debug(string, ...any) {}

func (noopLogger) Warn(string, ...any) {}
