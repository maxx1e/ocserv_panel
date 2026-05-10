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
