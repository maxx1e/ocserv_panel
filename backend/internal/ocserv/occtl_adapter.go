package ocserv

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"ocserv_panel/backend/internal/validation"
)

// sessionIDPattern matches valid ocserv session IDs (alphanumeric, hyphens, dots, underscores).
var sessionIDPattern = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)

func validateSessionID(id string) error {
	if id == "" {
		return errors.New("session ID must not be empty")
	}
	if len(id) > 128 {
		return errors.New("session ID too long")
	}
	if id[0] == '-' {
		return fmt.Errorf("session ID must not start with a dash: %q", id)
	}
	if !sessionIDPattern.MatchString(id) {
		return fmt.Errorf("session ID contains invalid characters: %q", id)
	}
	return nil
}

// OcctlAdapter implements Manager using the occtl command-line tool.
type OcctlAdapter struct {
	occtlPath  string
	socketPath string
	runner     CommandRunner
}

// Compile-time interface satisfaction check.
var _ Manager = (*OcctlAdapter)(nil)

// NewOcctlAdapter returns an adapter that calls occtl for session and user management.
// socketPath may be empty to use the occtl default.
func NewOcctlAdapter(occtlPath, socketPath string, runner CommandRunner) *OcctlAdapter {
	return &OcctlAdapter{
		occtlPath:  occtlPath,
		socketPath: socketPath,
		runner:     runner,
	}
}

// ListSessions calls `occtl -j show sessions all` and returns parsed results.
// Falls back to text parsing if JSON decoding fails.
// Timeout: 10 seconds.
func (a *OcctlAdapter) ListSessions(ctx context.Context) ([]Session, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	result, err := a.runner.Run(ctx, a.occtlPath, a.withJSONAndSocket([]string{"show", "sessions", "all"}), nil)
	if err != nil {
		return nil, fmt.Errorf("list sessions: %w", err)
	}
	if result.ExitCode != 0 {
		if a.isNotConfiguredErr(result.Stderr) {
			return nil, ErrNotConfigured
		}
		return nil, fmt.Errorf("list sessions: %s", strings.TrimSpace(result.Stderr))
	}

	out := strings.TrimSpace(result.Stdout)
	if out == "" || strings.Contains(out, "No sessions") {
		return []Session{}, nil
	}

	// Try JSON parsing first (native occtl format).
	sessions, err := parseSessionsJSON([]byte(out))
	if err == nil {
		return sessions, nil
	}

	// Fall back to text parsing.
	sessions, err = parseSessionsText(out)
	if err != nil {
		return nil, fmt.Errorf("parse sessions output: %w", err)
	}
	return sessions, nil
}

// DisconnectSession calls `occtl disconnect id <sessionID>`.
// Timeout: 15 seconds.
func (a *OcctlAdapter) DisconnectSession(ctx context.Context, sessionID string) error {
	if err := validateSessionID(sessionID); err != nil {
		return fmt.Errorf("disconnect session: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	result, err := a.runner.Run(ctx, a.occtlPath, a.withSocket([]string{"disconnect", "id", sessionID}), nil)
	if err != nil {
		return fmt.Errorf("disconnect session %s: %w", sessionID, err)
	}
	if result.ExitCode != 0 {
		if a.isNotConfiguredErr(result.Stderr) {
			return ErrNotConfigured
		}
		return fmt.Errorf("disconnect session %s: %s", sessionID, strings.TrimSpace(result.Stderr))
	}
	return nil
}

// occtlSessionDetail is kept as a compatibility alias for the old snake_case
// JSON format that some tests and internal callers may use. The actual occtl
// JSON format uses different keys (see occtlSessionDetailJSON in
// occtl_sessions_parser.go). Both formats are tried during parsing.
type occtlSessionDetail struct {
	ID             string   `json:"id"`
	Username       string   `json:"username"`
	RemoteAddress  string   `json:"remote_address"`
	ConnectedAt    string   `json:"connected_at"`
	VPNAddress     string   `json:"vpn_address"`
	IPv4           string   `json:"vpn_ipv4"`
	IPv6           string   `json:"vpn_ipv6"`
	RxBytes        int64    `json:"rx_bytes"`
	TxBytes        int64    `json:"tx_bytes"`
	UserAgent      string   `json:"user_agent"`
	Device         string   `json:"device"`
	Routes         []string `json:"routes"`
	DNS            []string `json:"dns"`
	ReconnectCount int      `json:"reconnect_count"`
}

// GetSessionDetails calls `occtl -j show session <id>`.
// Tries occtl-native JSON format first, falls back to snake_case JSON, then text parsing.
// Timeout: 10 seconds.
func (a *OcctlAdapter) GetSessionDetails(ctx context.Context, id string) (*SessionDetail, error) {
	if err := validateSessionID(id); err != nil {
		return nil, fmt.Errorf("get session details: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	result, err := a.runner.Run(ctx, a.occtlPath, a.withJSONAndSocket([]string{"show", "session", id}), nil)
	if err != nil {
		return nil, fmt.Errorf("get session details %s: %w", id, err)
	}
	if result.ExitCode != 0 {
		return nil, fmt.Errorf("get session details %s: %s", id, strings.TrimSpace(result.Stderr))
	}

	out := strings.TrimSpace(result.Stdout)
	if out == "" {
		return nil, fmt.Errorf("get session details %s: empty output", id)
	}

	// Try native occtl JSON format first (field names like "Session", "Remote IP", etc.).
	// The native format uses "Session" as the ID field — if that's populated, it's native format.
	sd, err := parseSessionDetailJSON([]byte(out))
	if err == nil && sd.ID != "" {
		return sd, nil
	}

	// Try the snake_case JSON format (legacy/internal format).
	var raw occtlSessionDetail
	if err2 := json.Unmarshal([]byte(out), &raw); err2 == nil && raw.Username != "" {
		connectedAt, _ := time.Parse(time.RFC3339, raw.ConnectedAt)
		sd = &SessionDetail{
			ID:             raw.ID,
			Username:       raw.Username,
			RemoteAddress:  raw.RemoteAddress,
			ConnectedAt:    connectedAt,
			VPNAddress:     raw.VPNAddress,
			IPv4:           raw.IPv4,
			IPv6:           raw.IPv6,
			RxBytes:        raw.RxBytes,
			TxBytes:        raw.TxBytes,
			UserAgent:      raw.UserAgent,
			Device:         raw.Device,
			Routes:         raw.Routes,
			DNS:            raw.DNS,
			ReconnectCount: raw.ReconnectCount,
		}
		if !sd.ConnectedAt.IsZero() {
			sd.Duration = time.Since(sd.ConnectedAt)
		}
		return sd, nil
	}

	// Fall back to text parsing.
	sd, err = parseSessionDetailText(out)
	if err != nil {
		return nil, fmt.Errorf("parse session detail %s: %w", id, err)
	}
	return sd, nil
}

// TerminateSession calls `occtl terminate id <sessionID>`.
// Timeout: 15 seconds.
func (a *OcctlAdapter) TerminateSession(ctx context.Context, sessionID string) error {
	if err := validateSessionID(sessionID); err != nil {
		return fmt.Errorf("terminate session: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	result, err := a.runner.Run(ctx, a.occtlPath, a.withSocket([]string{"terminate", "id", sessionID}), nil)
	if err != nil {
		return fmt.Errorf("terminate session %s: %w", sessionID, err)
	}
	if result.ExitCode != 0 {
		if a.isNotConfiguredErr(result.Stderr) {
			return ErrNotConfigured
		}
		return fmt.Errorf("terminate session %s: %s", sessionID, strings.TrimSpace(result.Stderr))
	}
	return nil
}

// withSocket prepends -s (short for --socket-file) if a socket path was configured
// and the socket file exists on disk. If the configured path is stale (e.g. ocserv
// appends a random suffix), we omit -s and let occtl use its built-in discovery.
func (a *OcctlAdapter) withSocket(args []string) []string {
	if a.socketPath == "" {
		return args
	}
	if _, err := os.Stat(a.socketPath); err != nil {
		return args
	}
	return append([]string{"-s", a.socketPath}, args...)
}

// withJSONAndSocket prepends -j -n (JSON output, no color/formatting) and optionally
// -s <socket> as global flags before the subcommand arguments. The socket path is only
// passed if it is non-empty AND the file exists on disk. Use this for commands
// that return parseable JSON output. For action commands (disconnect, unban) that do
// not return JSON, use withSocket instead.
func (a *OcctlAdapter) withJSONAndSocket(args []string) []string {
	var prefix []string
	prefix = append(prefix, "-j", "-n")
	if a.socketPath != "" {
		if _, err := os.Stat(a.socketPath); err == nil {
			prefix = append(prefix, "-s", a.socketPath)
		}
	}
	return append(prefix, args...)
}

// isNotConfiguredErr returns true when the stderr looks like a socket-not-found error.
func (a *OcctlAdapter) isNotConfiguredErr(stderr string) bool {
	low := strings.ToLower(stderr)
	return strings.Contains(low, "no such file") ||
		strings.Contains(low, "not found") ||
		strings.Contains(low, "connection refused")
}

// GetServerStatus calls `occtl -j show status` and parses the JSON output.
// Timeout: 10 seconds.
func (a *OcctlAdapter) GetServerStatus(ctx context.Context) (*ServerStatus, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	result, err := a.runner.Run(ctx, a.occtlPath, a.withJSONAndSocket([]string{"show", "status"}), nil)
	if err != nil {
		return nil, fmt.Errorf("get server status: %w", err)
	}
	if result.ExitCode != 0 {
		if a.isNotConfiguredErr(result.Stderr) {
			return nil, ErrNotConfigured
		}
		return nil, fmt.Errorf("get server status: %s", strings.TrimSpace(result.Stderr))
	}
	status, err := parseStatusJSON([]byte(strings.TrimSpace(result.Stdout)))
	if err != nil {
		return nil, fmt.Errorf("parse server status: %w", err)
	}
	return status, nil
}

// ListConnectedUsers calls `occtl -j show users` and parses the result.
// Falls back to text parsing if JSON decoding fails.
// Timeout: 10 seconds.
func (a *OcctlAdapter) ListConnectedUsers(ctx context.Context) ([]ConnectedUser, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	result, err := a.runner.Run(ctx, a.occtlPath, a.withJSONAndSocket([]string{"show", "users"}), nil)
	if err != nil {
		return nil, fmt.Errorf("list connected users: %w", err)
	}
	if result.ExitCode != 0 {
		if a.isNotConfiguredErr(result.Stderr) {
			return nil, ErrNotConfigured
		}
		return nil, fmt.Errorf("list connected users: %s", strings.TrimSpace(result.Stderr))
	}

	out := strings.TrimSpace(result.Stdout)
	if out == "" {
		return []ConnectedUser{}, nil
	}

	// Try JSON first
	users, err := parseUsersJSON([]byte(out))
	if err == nil {
		return users, nil
	}
	// Fall back to text parsing
	users, err = parseUsersText(out)
	if err != nil {
		return nil, fmt.Errorf("parse connected users: %w", err)
	}
	return users, nil
}

// GetUserDetails calls `occtl -j -n show user <username>` and parses connection blocks.
// Tries JSON parsing first (native occtl -j output), falls back to text parsing.
// Timeout: 10 seconds.
func (a *OcctlAdapter) GetUserDetails(ctx context.Context, username string) ([]UserConnection, error) {
	if err := validation.Username(username); err != nil {
		return nil, fmt.Errorf("get user details: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	result, err := a.runner.Run(ctx, a.occtlPath, a.withJSONAndSocket([]string{"show", "user", username}), nil)
	if err != nil {
		return nil, fmt.Errorf("get user details %s: %w", username, err)
	}
	if result.ExitCode != 0 {
		if a.isNotConfiguredErr(result.Stderr) {
			return nil, ErrNotConfigured
		}
		return nil, fmt.Errorf("get user details %s: %s", username, strings.TrimSpace(result.Stderr))
	}

	out := strings.TrimSpace(result.Stdout)
	if out == "" {
		return []UserConnection{}, nil
	}

	// Try JSON first (occtl -j -n returns JSON array of user connections).
	conns, err := parseUserDetailJSON([]byte(out))
	if err == nil {
		return conns, nil
	}

	// Fall back to text parsing.
	conns, err = parseUserDetailOutput(result.Stdout)
	if err != nil {
		return nil, fmt.Errorf("parse user details %s: %w", username, err)
	}
	return conns, nil
}

// GetConnectionByID calls `occtl -j -n show id <ID>` and parses the connection detail.
// The output format is the same as `show user` (array of connection objects).
// Returns ErrSessionNotFound if the ID has no active connection.
// Timeout: 10 seconds.
func (a *OcctlAdapter) GetConnectionByID(ctx context.Context, id int) (*UserConnection, error) {
	if id <= 0 {
		return nil, fmt.Errorf("connection ID must be positive, got %d", id)
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	idStr := strconv.Itoa(id)
	result, err := a.runner.Run(ctx, a.occtlPath, a.withJSONAndSocket([]string{"show", "id", idStr}), nil)
	if err != nil {
		return nil, fmt.Errorf("get connection by id %d: %w", id, err)
	}
	if result.ExitCode != 0 {
		if a.isNotConfiguredErr(result.Stderr) {
			return nil, ErrNotConfigured
		}
		return nil, fmt.Errorf("get connection by id %d: %s", id, strings.TrimSpace(result.Stderr))
	}

	out := strings.TrimSpace(result.Stdout)
	if out == "" {
		return nil, ErrSessionNotFound
	}

	// Parse using the same user detail JSON parser (output format is identical).
	conns, err := parseUserDetailJSON([]byte(out))
	if err != nil {
		// Fall back to text parsing.
		conns, err = parseUserDetailOutput(out)
		if err != nil {
			return nil, fmt.Errorf("parse connection by id %d: %w", id, err)
		}
	}

	if len(conns) == 0 {
		return nil, ErrSessionNotFound
	}
	return &conns[0], nil
}

// ListEvents calls `occtl -j show events` and parses the result.
// Timeout: 10 seconds (show events may stream on some installations).
func (a *OcctlAdapter) ListEvents(ctx context.Context) ([]ConnectionEvent, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	result, err := a.runner.Run(ctx, a.occtlPath, a.withJSONAndSocket([]string{"show", "events"}), nil)
	if err != nil {
		return nil, fmt.Errorf("list events: %w", err)
	}
	if result.ExitCode != 0 {
		if a.isNotConfiguredErr(result.Stderr) {
			return nil, ErrNotConfigured
		}
		return nil, fmt.Errorf("list events: %s", strings.TrimSpace(result.Stderr))
	}

	out := strings.TrimSpace(result.Stdout)
	if out == "" {
		return []ConnectionEvent{}, nil
	}

	events, err := parseEventsJSON([]byte(out))
	if err != nil {
		return nil, fmt.Errorf("parse events: %w", err)
	}
	return events, nil
}

// ListRoutes calls `occtl show iroutes` and parses the text output.
// Timeout: 10 seconds.
func (a *OcctlAdapter) ListRoutes(ctx context.Context) ([]UserRoute, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	result, err := a.runner.Run(ctx, a.occtlPath, a.withSocket([]string{"show", "iroutes"}), nil)
	if err != nil {
		return nil, fmt.Errorf("list routes: %w", err)
	}
	if result.ExitCode != 0 {
		if a.isNotConfiguredErr(result.Stderr) {
			return nil, ErrNotConfigured
		}
		return nil, fmt.Errorf("list routes: %s", strings.TrimSpace(result.Stderr))
	}

	routes, err := parseRoutesText(result.Stdout)
	if err != nil {
		return nil, fmt.Errorf("parse routes: %w", err)
	}
	return routes, nil
}

// ListBannedIPs calls `occtl -j show ip bans` and parses the result.
// Timeout: 10 seconds.
func (a *OcctlAdapter) ListBannedIPs(ctx context.Context) ([]BannedIP, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	result, err := a.runner.Run(ctx, a.occtlPath, a.withJSONAndSocket([]string{"show", "ip", "bans"}), nil)
	if err != nil {
		return nil, fmt.Errorf("list banned ips: %w", err)
	}
	if result.ExitCode != 0 {
		if a.isNotConfiguredErr(result.Stderr) {
			return nil, ErrNotConfigured
		}
		return nil, fmt.Errorf("list banned ips: %s", strings.TrimSpace(result.Stderr))
	}

	out := strings.TrimSpace(result.Stdout)
	if out == "" {
		return []BannedIP{}, nil
	}

	bans, err := parseBannedIPsJSON([]byte(out))
	if err != nil {
		return nil, fmt.Errorf("parse banned ips: %w", err)
	}
	return bans, nil
}

// ListBanPoints calls `occtl -j show ip ban points` and parses the result.
// Timeout: 10 seconds.
func (a *OcctlAdapter) ListBanPoints(ctx context.Context) ([]BanPointEntry, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	result, err := a.runner.Run(ctx, a.occtlPath, a.withJSONAndSocket([]string{"show", "ip", "ban", "points"}), nil)
	if err != nil {
		return nil, fmt.Errorf("list ban points: %w", err)
	}
	if result.ExitCode != 0 {
		if a.isNotConfiguredErr(result.Stderr) {
			return nil, ErrNotConfigured
		}
		return nil, fmt.Errorf("list ban points: %s", strings.TrimSpace(result.Stderr))
	}

	out := strings.TrimSpace(result.Stdout)
	if out == "" {
		return []BanPointEntry{}, nil
	}

	points, err := parseBanPointsJSON([]byte(out))
	if err != nil {
		return nil, fmt.Errorf("parse ban points: %w", err)
	}
	return points, nil
}

// DisconnectUser calls `occtl disconnect user <username>`.
// Timeout: 15 seconds.
func (a *OcctlAdapter) DisconnectUser(ctx context.Context, username string) error {
	if err := validation.Username(username); err != nil {
		return fmt.Errorf("disconnect user: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	result, err := a.runner.Run(ctx, a.occtlPath, a.withSocket([]string{"disconnect", "user", username}), nil)
	if err != nil {
		return fmt.Errorf("disconnect user %s: %w", username, err)
	}
	if result.ExitCode != 0 {
		if a.isNotConfiguredErr(result.Stderr) {
			return ErrNotConfigured
		}
		return fmt.Errorf("disconnect user %s: %s", username, strings.TrimSpace(result.Stderr))
	}
	return nil
}

// UnbanIP calls `occtl unban ip <ip>`.
// Timeout: 15 seconds.
func (a *OcctlAdapter) UnbanIP(ctx context.Context, ip string) error {
	if err := validation.IPAddress(ip); err != nil {
		return fmt.Errorf("unban ip: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	result, err := a.runner.Run(ctx, a.occtlPath, a.withSocket([]string{"unban", "ip", ip}), nil)
	if err != nil {
		return fmt.Errorf("unban ip %s: %w", ip, err)
	}
	if result.ExitCode != 0 {
		if a.isNotConfiguredErr(result.Stderr) {
			return ErrNotConfigured
		}
		return fmt.Errorf("unban ip %s: %s", ip, strings.TrimSpace(result.Stderr))
	}
	return nil
}
