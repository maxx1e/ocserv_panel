package monitor

import (
	"context"
	"sync"
	"time"
)

const vpnWatcherDefaultTickInterval = 15 * time.Second

// VPNMultiDiscovererFunc attempts to find all active VPN interfaces.
type VPNMultiDiscovererFunc func() ([]string, error)

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
