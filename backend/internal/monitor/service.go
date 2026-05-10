package monitor

import (
	"context"
	"sync"
	"time"

	
)

const (
	defaultCollectorInterval = 15 * time.Second
	vpnRediscoveryInterval   = 15 * time.Second
)

// VPNMultiDiscovererFunc attempts to find all active VPN interfaces.
// It returns a slice of interface names (may be empty) and an error.
type VPNMultiDiscovererFunc func() ([]string, error)

type Logger interface {
	Debug(msg string, args ...any)
	Warn(msg string, args ...any)
}

type CollectorService struct {
	store    Store
	interval time.Duration
	logger   Logger
	now      func() time.Time

	mu            sync.Mutex
	collectors    []Collector
	vpnDiscoverer VPNMultiDiscovererFunc
	vpnReader     FileReader
	vpnRXMetric   MetricName
	vpnTXMetric   MetricName
	discoveredVPN map[string]bool
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
// When set, the Run loop will periodically call fn to look for VPN
// interfaces and, if found, create new InterfaceCollectors for them.
func (s *CollectorService) SetVPNDiscoverer(
	fn VPNMultiDiscovererFunc,
	reader FileReader,
	rxMetric, txMetric MetricName,
) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.vpnDiscoverer = fn
	s.vpnReader = reader
	s.vpnRXMetric = rxMetric
	s.vpnTXMetric = txMetric
}

// HasVPNCollector reports whether any VPN interface collector has been added.
func (s *CollectorService) HasVPNCollector() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.discoveredVPN) > 0
}

// MarkVPNPresent marks a specific VPN interface as already present, for example
// when a VPN interface was discovered at startup before the Run loop.
func (s *CollectorService) MarkVPNPresent(iface string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.discoveredVPN == nil {
		s.discoveredVPN = make(map[string]bool)
	}
	s.discoveredVPN[iface] = true
}

func (s *CollectorService) CollectOnce(ctx context.Context) {
	if err := ctx.Err(); err != nil {
		return
	}

	// Snapshot collectors under lock to avoid races with maybeDiscoverAllVPN.
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
			s.maybeDiscoverAllVPN()
		}
	}
}

// maybeDiscoverAllVPN checks for VPN interfaces and adds collectors for any
// newly discovered ones.
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
