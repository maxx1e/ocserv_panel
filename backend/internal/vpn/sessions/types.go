// Package sessions provides types, parsers, and a Manager interface for ocserv
// session and server management via the occtl command-line tool.
package sessions

import "time"

// Session represents an active ocserv session as reported by occtl show sessions.
type Session struct {
	ID             string    `json:"ID"`
	FullSession    string    `json:"FullSession"`
	Username       string    `json:"Username"`
	RemoteAddress  string    `json:"RemoteAddress"`
	ConnectedAt    time.Time `json:"ConnectedAt"`
	VPNAddress     string    `json:"VPNAddress"`
	RxBytes        int64     `json:"RxBytes"`
	TxBytes        int64     `json:"TxBytes"`
	UserAgent      string    `json:"UserAgent"`
	Device         string    `json:"Device"`
	ReconnectCount int       `json:"ReconnectCount"`
	State          string    `json:"State"`
	Groupname      string    `json:"Groupname"`
	Vhost          string    `json:"Vhost"`
	Location       string    `json:"Location"`
	SessionIsOpen  bool      `json:"SessionIsOpen"`
	InUse          bool      `json:"InUse"`
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
	State          string        `json:"state"`
	Groupname      string        `json:"groupname"`
	Vhost          string        `json:"vhost"`
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
	ID            string        `json:"id"`
	Username      string        `json:"username"`
	Groupname     string        `json:"groupname"`
	Vhost         string        `json:"vhost"`
	RemoteIP      string        `json:"remote_ip"`
	Location      string        `json:"location"`
	VpnIP         string        `json:"vpn_ip"`
	Device        string        `json:"device"`
	MTU           int           `json:"mtu"`
	LocalDeviceIP string        `json:"local_device_ip"`
	PtPIPv4       string        `json:"ptp_ipv4"`
	UserAgent     string        `json:"user_agent"`
	RxBytes       int64         `json:"rx_bytes"`
	TxBytes       int64         `json:"tx_bytes"`
	Hostname      string        `json:"hostname"`
	Since         string        `json:"since"`
	Duration      time.Duration `json:"duration"`
	ConnectedAt   time.Time     `json:"connected_at"`
	SessionID     string        `json:"session_id"`
	FullSession   string        `json:"full_session"`
	TLSCipher     string        `json:"tls_cipher"`
	DTLSCipher    string        `json:"dtls_cipher"`
	Status        string        `json:"status"`
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
	IP    string `json:"ip"`
	Score int    `json:"score"`
}
