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
