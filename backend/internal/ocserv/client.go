// Package ocserv defines safe integration boundaries for ocserv operations.
package ocserv

import (
	"context"
	"errors"

	"ocserv_panel/backend/internal/ocserv/parse"
)

// ErrNotConfigured is returned by placeholder integrations that cannot touch live ocserv state.
var ErrNotConfigured = errors.New("ocserv integration is not configured")

// ErrSessionNotFound is returned when a requested session ID does not exist.
var ErrSessionNotFound = errors.New("ocserv session not found")

// ErrPasswordFileMissing is returned when the ocserv password file does not exist at the configured path.
var ErrPasswordFileMissing = errors.New("ocserv password file does not exist")

// Type aliases — canonical definitions live in ocserv/parse.
type Session = parse.Session
type SessionDetail = parse.SessionDetail
type ServerStatus = parse.ServerStatus
type ConnectedUser = parse.ConnectedUser
type UserConnection = parse.UserConnection
type ConnectionEvent = parse.ConnectionEvent
type UserRoute = parse.UserRoute
type BannedIP = parse.BannedIP
type BanPointEntry = parse.BanPointEntry

// Manager is the boundary for ocserv management operations.
type Manager interface {
	ListSessions(ctx context.Context) ([]Session, error)
	DisconnectSession(ctx context.Context, sessionID string) error
	TerminateSession(ctx context.Context, sessionID string) error
	GetSessionDetails(ctx context.Context, id string) (*SessionDetail, error)

	// Server Status
	GetServerStatus(ctx context.Context) (*ServerStatus, error)

	// User Inspection
	ListConnectedUsers(ctx context.Context) ([]ConnectedUser, error)
	GetUserDetails(ctx context.Context, username string) ([]UserConnection, error)

	// GetConnectionByID returns full connection details for a numeric connection ID.
	GetConnectionByID(ctx context.Context, id int) (*UserConnection, error)

	// Events, Routes, Bans
	ListEvents(ctx context.Context) ([]ConnectionEvent, error)
	ListRoutes(ctx context.Context) ([]UserRoute, error)
	ListBannedIPs(ctx context.Context) ([]BannedIP, error)
	ListBanPoints(ctx context.Context) ([]BanPointEntry, error)

	// Actions
	DisconnectUser(ctx context.Context, username string) error
	UnbanIP(ctx context.Context, ip string) error
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
	ServerStatusResult    *ServerStatus
	ServerStatusErr       error
	ConnectedUsersResult  []ConnectedUser
	ConnectedUsersErr     error
	UserDetailsResult     map[string][]UserConnection
	UserDetailsErr        error
	ConnectionByIDResult  map[int]*UserConnection
	ConnectionByIDErr     error
	EventsResult          []ConnectionEvent
	EventsErr             error
	RoutesResult          []UserRoute
	RoutesErr             error
	BannedIPsResult       []BannedIP
	BannedIPsErr          error
	BanPointsResult       []BanPointEntry
	BanPointsErr          error
	DisconnectUserErr     error
	UnbanIPErr            error
}

// NewFakeManager returns a FakeManager with empty default data.
// By default, DisconnectSession and TerminateSession return ErrNotConfigured.
func NewFakeManager() *FakeManager {
	return &FakeManager{
		SessionDetails:        make(map[string]*SessionDetail),
		SessionDetailNotFound: make(map[string]bool),
		UserDetailsResult:     make(map[string][]UserConnection),
		ConnectionByIDResult:  make(map[int]*UserConnection),
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

// GetServerStatus returns the configured server status result.
func (f *FakeManager) GetServerStatus(ctx context.Context) (*ServerStatus, error) {
	_ = ctx
	return f.ServerStatusResult, f.ServerStatusErr
}

// ListConnectedUsers returns the configured connected users result.
func (f *FakeManager) ListConnectedUsers(ctx context.Context) ([]ConnectedUser, error) {
	_ = ctx
	return f.ConnectedUsersResult, f.ConnectedUsersErr
}

// GetUserDetails returns connections for the given username from the configured map.
func (f *FakeManager) GetUserDetails(ctx context.Context, username string) ([]UserConnection, error) {
	_ = ctx
	if conns, ok := f.UserDetailsResult[username]; ok {
		return conns, nil
	}
	if f.UserDetailsErr != nil {
		return nil, f.UserDetailsErr
	}
	return nil, nil
}

// GetConnectionByID returns the configured connection for the given ID.
func (f *FakeManager) GetConnectionByID(ctx context.Context, id int) (*UserConnection, error) {
	_ = ctx
	if conn, ok := f.ConnectionByIDResult[id]; ok {
		return conn, nil
	}
	if f.ConnectionByIDErr != nil {
		return nil, f.ConnectionByIDErr
	}
	return nil, ErrSessionNotFound
}

// ListEvents returns the configured events result.
func (f *FakeManager) ListEvents(ctx context.Context) ([]ConnectionEvent, error) {
	_ = ctx
	return f.EventsResult, f.EventsErr
}

// ListRoutes returns the configured routes result.
func (f *FakeManager) ListRoutes(ctx context.Context) ([]UserRoute, error) {
	_ = ctx
	return f.RoutesResult, f.RoutesErr
}

// ListBannedIPs returns the configured banned IPs result.
func (f *FakeManager) ListBannedIPs(ctx context.Context) ([]BannedIP, error) {
	_ = ctx
	return f.BannedIPsResult, f.BannedIPsErr
}

// ListBanPoints returns the configured ban points result.
func (f *FakeManager) ListBanPoints(ctx context.Context) ([]BanPointEntry, error) {
	_ = ctx
	return f.BanPointsResult, f.BanPointsErr
}

// DisconnectUser returns the configured disconnect user error.
func (f *FakeManager) DisconnectUser(ctx context.Context, username string) error {
	_ = ctx
	_ = username
	return f.DisconnectUserErr
}

// UnbanIP returns the configured unban IP error.
func (f *FakeManager) UnbanIP(ctx context.Context, ip string) error {
	_ = ctx
	_ = ip
	return f.UnbanIPErr
}
