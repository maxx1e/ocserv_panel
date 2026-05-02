package monitoring

import (
	"context"
	"time"

	"ocserv_panel/backend/internal/metrics"
)

const defaultCollectorInterval = 15 * time.Second

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

func (s *CollectorService) CollectOnce(ctx context.Context) {
	if err := ctx.Err(); err != nil {
		return
	}

	for _, collector := range s.collectors {
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

func (noopLogger) Warn(string, ...any) {}
