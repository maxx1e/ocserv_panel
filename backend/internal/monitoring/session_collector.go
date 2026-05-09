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
