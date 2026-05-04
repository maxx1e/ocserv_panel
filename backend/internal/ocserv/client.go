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
	ID            string    `json:"ID"`
	Username      string    `json:"Username"`
	RemoteAddress string    `json:"RemoteAddress"`
	ConnectedAt   time.Time `json:"ConnectedAt"`
}

// User represents an ocserv user as reported by occtl.
type User struct {
	Username string `json:"username"`
	Group    string `json:"group"`
	Active   bool   `json:"active"`
}

// SessionDetail represents detailed information about a single ocserv session.
type SessionDetail struct {
	ID            string        `json:"id"`
	Username      string        `json:"username"`
	RemoteAddress string        `json:"remote_address"`
	ConnectedAt   time.Time     `json:"connected_at"`
	Duration      time.Duration `json:"duration"`
	BytesIn       int64         `json:"bytes_in"`
	BytesOut      int64         `json:"bytes_out"`
	Device        string        `json:"device"`
	IPv4          string        `json:"ipv4"`
	IPv6          string        `json:"ipv6"`
}

// Manager is the boundary for ocserv management operations.
type Manager interface {
	ListSessions(ctx context.Context) ([]Session, error)
	DisconnectSession(ctx context.Context, sessionID string) error
	ListUsers(ctx context.Context) ([]User, error)
	GetSessionDetails(ctx context.Context, id string) (*SessionDetail, error)
}

// Compile-time interface satisfaction check.
var _ Manager = (*FakeManager)(nil)

// FakeManager is a test double that returns configurable results.
// Use NewFakeManager to construct one.
type FakeManager struct {
	Sessions       []Session
	SessionDetails map[string]*SessionDetail
	Users          []User
	DisconnectErr  error
	ListUsersErr   error
}

// NewFakeManager returns a FakeManager with empty default data.
// By default, DisconnectSession returns ErrNotConfigured to signal
// that no live adapter has been wired.
func NewFakeManager() *FakeManager {
	return &FakeManager{
		SessionDetails: make(map[string]*SessionDetail),
		DisconnectErr:  ErrNotConfigured,
	}
}

// ListSessions returns the configured sessions without error.
func (f *FakeManager) ListSessions(ctx context.Context) ([]Session, error) {
	_ = ctx
	return f.Sessions, nil
}

// DisconnectSession returns the configured error (ErrNotConfigured by default).
func (f *FakeManager) DisconnectSession(ctx context.Context, sessionID string) error {
	_ = ctx
	_ = sessionID
	return f.DisconnectErr
}

// ListUsers returns the configured users or ListUsersErr.
func (f *FakeManager) ListUsers(ctx context.Context) ([]User, error) {
	_ = ctx
	return f.Users, f.ListUsersErr
}

// GetSessionDetails returns the detail for the given id, or ErrNotConfigured
// if no detail is configured for that id.
func (f *FakeManager) GetSessionDetails(ctx context.Context, id string) (*SessionDetail, error) {
	_ = ctx
	if detail, ok := f.SessionDetails[id]; ok {
		return detail, nil
	}
	return nil, ErrNotConfigured
}
