// Package ocserv defines safe integration boundaries for ocserv operations.
package ocserv

import (
	"context"
	"errors"
	"time"
)

// ErrNotConfigured is returned by placeholder integrations that cannot touch live ocserv state.
var ErrNotConfigured = errors.New("ocserv integration is not configured")

// Session represents an active ocserv session as reported by a future adapter.
type Session struct {
	ID            string
	Username      string
	RemoteAddress string
	ConnectedAt   time.Time
}

// Manager is the boundary for ocserv management operations.
type Manager interface {
	ListSessions(ctx context.Context) ([]Session, error)
	DisconnectSession(ctx context.Context, sessionID string) error
}

// NoopManager is a safe development implementation that never calls ocserv.
type NoopManager struct{}

// NewNoopManager returns a safe no-op ocserv manager.
func NewNoopManager() NoopManager {
	return NoopManager{}
}

// ListSessions returns an empty session list without touching the system.
func (NoopManager) ListSessions(ctx context.Context) ([]Session, error) {
	_ = ctx
	return []Session{}, nil
}

// DisconnectSession refuses to terminate sessions because no live adapter is configured.
func (NoopManager) DisconnectSession(ctx context.Context, sessionID string) error {
	_ = ctx
	_ = sessionID
	return ErrNotConfigured
}
