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

	var detail SessionDetail
	if err := json.Unmarshal([]byte(strings.TrimSpace(result.Stdout)), &detail); err != nil {
		return nil, fmt.Errorf("parse session detail: %w", err)
	}
	return &detail, nil
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
