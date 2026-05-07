// Package ocserv defines safe integration boundaries for ocserv operations.
package ocserv

import (
	"context"
	"errors"
	"time"
)

// ErrNotConfigured is returned by placeholder integrations that cannot touch live ocserv state.
var ErrNotConfigured = errors.New("ocserv integration is not configured")

// ErrSessionNotFound is returned when a requested session ID does not exist.
var ErrSessionNotFound = errors.New("ocserv session not found")

// ErrPasswordFileMissing is returned when the ocserv password file does not exist at the configured path.
var ErrPasswordFileMissing = errors.New("ocserv password file does not exist")

// Session represents an active ocserv session as reported by occtl show sessions.
type Session struct {
	ID             string    `json:"ID"`
	Username       string    `json:"Username"`
	RemoteAddress  string    `json:"RemoteAddress"`
	ConnectedAt    time.Time `json:"ConnectedAt"`
	VPNAddress     string    `json:"VPNAddress"`
	RxBytes        int64     `json:"RxBytes"`
	TxBytes        int64     `json:"TxBytes"`
	UserAgent      string    `json:"UserAgent"`
	Device         string    `json:"Device"`
	ReconnectCount int       `json:"ReconnectCount"`
}

// SessionDetail represents full information about a single active session.
type SessionDetail struct {
	ID             string        `json:"id"`
	Username       string        `json:"username"`
	RemoteAddress  string        `json:"remote_address"`
	ConnectedAt    time.Time     `json:"connected_at"`
	Duration       time.Duration `json:"duration"`
	VPNAddress     string        `json:"vpn_address"`
	RxBytes        int64         `json:"rx_bytes"`
	TxBytes        int64         `json:"tx_bytes"`
	UserAgent      string        `json:"user_agent"`
	Device         string        `json:"device"`
	IPv4           string        `json:"ipv4"`
	IPv6           string        `json:"ipv6"`
	Routes         []string      `json:"routes"`
	DNS            []string      `json:"dns"`
	ReconnectCount int           `json:"reconnect_count"`
}

// Manager is the boundary for ocserv management operations.
type Manager interface {
	ListSessions(ctx context.Context) ([]Session, error)
	DisconnectSession(ctx context.Context, sessionID string) error
	TerminateSession(ctx context.Context, sessionID string) error
	GetSessionDetails(ctx context.Context, id string) (*SessionDetail, error)
}

// Compile-time interface satisfaction check.
var _ Manager = (*FakeManager)(nil)

// FakeManager is a test double that returns configurable results.
// Use NewFakeManager to construct one.
type FakeManager struct {
	Sessions              []Session
	SessionDetails        map[string]*SessionDetail
	SessionDetailNotFound map[string]bool
	DisconnectErr         error
	TerminateErr          error
}

// NewFakeManager returns a FakeManager with empty default data.
// By default, DisconnectSession and TerminateSession return ErrNotConfigured.
func NewFakeManager() *FakeManager {
	return &FakeManager{
		SessionDetails:        make(map[string]*SessionDetail),
		SessionDetailNotFound: make(map[string]bool),
		DisconnectErr:         ErrNotConfigured,
		TerminateErr:          ErrNotConfigured,
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

// TerminateSession returns the configured error (ErrNotConfigured by default).
func (f *FakeManager) TerminateSession(ctx context.Context, sessionID string) error {
	_ = ctx
	_ = sessionID
	return f.TerminateErr
}

// GetSessionDetails returns the detail for the given id, ErrSessionNotFound when
// configured as not-found for that id, or ErrNotConfigured for unknown ids.
func (f *FakeManager) GetSessionDetails(ctx context.Context, id string) (*SessionDetail, error) {
	_ = ctx
	if detail, ok := f.SessionDetails[id]; ok {
		return detail, nil
	}
	if f.SessionDetailNotFound[id] {
		return nil, ErrSessionNotFound
	}
	return nil, ErrNotConfigured
}
