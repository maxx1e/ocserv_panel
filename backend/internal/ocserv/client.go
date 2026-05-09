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

// ServerStatus represents parsed output from `occtl show status`.
type ServerStatus struct {
	Status               string        `json:"status"`
	ServerPID            int           `json:"server_pid"`
	SecModPID            int           `json:"secmod_pid"`
	SecModInstances      int           `json:"secmod_instances"`
	UpSince              time.Time     `json:"up_since"`
	ActiveSessions       int           `json:"active_sessions"`
	TotalSessions        int           `json:"total_sessions"`
	TotalAuthFailures    int           `json:"total_auth_failures"`
	BannedIPs            int           `json:"banned_ips"`
	LastStatsReset       time.Time     `json:"last_stats_reset"`
	SessionsHandled      int           `json:"sessions_handled"`
	TimedOutSessions     int           `json:"timed_out_sessions"`
	IdleTimedOutSessions int           `json:"idle_timed_out_sessions"`
	ErrorClosedSessions  int           `json:"error_closed_sessions"`
	AuthFailures         int           `json:"auth_failures"`
	AvgAuthTime          time.Duration `json:"avg_auth_time"`
	MaxAuthTime          time.Duration `json:"max_auth_time"`
	AvgSessionTime       time.Duration `json:"avg_session_time"`
	MaxSessionTime       time.Duration `json:"max_session_time"`
	MinMTU               int           `json:"min_mtu"`
	MaxMTU               int           `json:"max_mtu"`
	RxBytes              int64         `json:"rx_bytes"`
	TxBytes              int64         `json:"tx_bytes"`
}

// ConnectedUser represents a connected user from `occtl show users`.
type ConnectedUser struct {
	ID         string        `json:"id"`
	Username   string        `json:"username"`
	Vhost      string        `json:"vhost"`
	RemoteIP   string        `json:"remote_ip"`
	VpnIP      string        `json:"vpn_ip"`
	Device     string        `json:"device"`
	Since      string        `json:"since"`
	Duration   time.Duration `json:"duration"`
	DTLSCipher string        `json:"dtls_cipher"`
	Status     string        `json:"status"`
}

// UserConnection represents a single connection block from `occtl show user <name>`.
type UserConnection struct {
	ID                 string    `json:"id"`
	Username           string    `json:"username"`
	Groupname          string    `json:"groupname"`
	State              string    `json:"state"`
	Vhost              string    `json:"vhost"`
	Device             string    `json:"device"`
	MTU                int       `json:"mtu"`
	RemoteIP           string    `json:"remote_ip"`
	Location           string    `json:"location"`
	LocalDeviceIP      string    `json:"local_device_ip"`
	IPv4               string    `json:"ipv4"`
	PtPIPv4            string    `json:"ptp_ipv4"`
	UserAgent          string    `json:"user_agent"`
	RxBytes            int64     `json:"rx_bytes"`
	TxBytes            int64     `json:"tx_bytes"`
	AvgRxBandwidth     string    `json:"avg_rx_bandwidth"`
	AvgTxBandwidth     string    `json:"avg_tx_bandwidth"`
	DPD                int       `json:"dpd"`
	KeepAlive          int       `json:"keep_alive"`
	Hostname           string    `json:"hostname"`
	ConnectedAt        time.Time `json:"connected_at"`
	SessionID          string    `json:"session_id"`
	TLSCipher          string    `json:"tls_cipher"`
	DTLSCipher         string    `json:"dtls_cipher"`
	DNS                []string  `json:"dns"`
	Routes             []string  `json:"routes"`
	RestrictedToRoutes bool      `json:"restricted_to_routes"`
}

// ConnectionEvent represents a connection event from `occtl show events`.
type ConnectionEvent struct {
	Timestamp time.Time `json:"timestamp"`
	Username  string    `json:"username"`
	RemoteIP  string    `json:"remote_ip"`
	EventType string    `json:"event_type"`
	Message   string    `json:"message"`
}

// UserRoute represents a user-provided route from `occtl show iroutes`.
type UserRoute struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Vhost    string `json:"vhost"`
	Device   string `json:"device"`
	VpnIP    string `json:"vpn_ip"`
	Route    string `json:"route"`
}

// BannedIP represents a banned IP from `occtl show ip bans`.
type BannedIP struct {
	IP     string `json:"ip"`
	Score  int    `json:"score"`
	Reason string `json:"reason"`
}

// BanPointEntry represents ban point info from `occtl show ip ban points`.
type BanPointEntry struct {
	IP     string    `json:"ip"`
	Points int       `json:"points"`
	Since  time.Time `json:"since"`
}

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
