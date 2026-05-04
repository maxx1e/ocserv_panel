package ocserv

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

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

// ListSessions calls `occtl show sessions --output-type json` and returns parsed results.
// Timeout: 10 seconds.
func (a *OcctlAdapter) ListSessions(ctx context.Context) ([]Session, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	result, err := a.runner.Run(ctx, a.occtlPath, a.withSocket([]string{"show", "sessions", "--output-type", "json"}), nil)
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

	var sessions []Session
	if err := json.Unmarshal([]byte(out), &sessions); err != nil {
		return nil, fmt.Errorf("parse sessions output: %w", err)
	}
	return sessions, nil
}

// DisconnectSession calls `occtl disconnect id <sessionID>`.
// Timeout: 15 seconds.
func (a *OcctlAdapter) DisconnectSession(ctx context.Context, sessionID string) error {
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

// ListUsers calls `occtl show users --output-type json` and returns parsed results.
// Timeout: 10 seconds.
func (a *OcctlAdapter) ListUsers(ctx context.Context) ([]User, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	result, err := a.runner.Run(ctx, a.occtlPath, a.withSocket([]string{"show", "users", "--output-type", "json"}), nil)
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	if result.ExitCode != 0 {
		if a.isNotConfiguredErr(result.Stderr) {
			return nil, ErrNotConfigured
		}
		return nil, fmt.Errorf("list users: %s", strings.TrimSpace(result.Stderr))
	}

	out := strings.TrimSpace(result.Stdout)
	if out == "" || strings.Contains(out, "No users") {
		return []User{}, nil
	}

	var users []User
	if err := json.Unmarshal([]byte(out), &users); err != nil {
		return nil, fmt.Errorf("parse users output: %w", err)
	}
	return users, nil
}

// occtlSessionDetail is an intermediate struct that captures the JSON field names
// that occtl uses in its `show session <id> --output-type json` output.
// Fields are mapped to SessionDetail after parsing.
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

// GetSessionDetails calls `occtl show session <id> --output-type json`.
// Timeout: 10 seconds.
func (a *OcctlAdapter) GetSessionDetails(ctx context.Context, id string) (*SessionDetail, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	result, err := a.runner.Run(ctx, a.occtlPath, a.withSocket([]string{"show", "session", id, "--output-type", "json"}), nil)
	if err != nil {
		return nil, fmt.Errorf("get session details %s: %w", id, err)
	}
	if result.ExitCode != 0 {
		return nil, fmt.Errorf("get session details %s: %s", id, strings.TrimSpace(result.Stderr))
	}

	var raw occtlSessionDetail
	if err := json.Unmarshal([]byte(strings.TrimSpace(result.Stdout)), &raw); err != nil {
		return nil, fmt.Errorf("parse session detail: %w", err)
	}

	connectedAt, _ := time.Parse(time.RFC3339, raw.ConnectedAt)
	sd := &SessionDetail{
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

// TerminateSession calls `occtl terminate id <sessionID>`.
// Timeout: 15 seconds.
func (a *OcctlAdapter) TerminateSession(ctx context.Context, sessionID string) error {
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

// withSocket prepends --socket-file if a socket path was configured.
func (a *OcctlAdapter) withSocket(args []string) []string {
	if a.socketPath == "" {
		return args
	}
	return append([]string{"--socket-file", a.socketPath}, args...)
}

// isNotConfiguredErr returns true when the stderr looks like a socket-not-found error.
func (a *OcctlAdapter) isNotConfiguredErr(stderr string) bool {
	low := strings.ToLower(stderr)
	return strings.Contains(low, "no such file") ||
		strings.Contains(low, "not found") ||
		strings.Contains(low, "connection refused")
}
