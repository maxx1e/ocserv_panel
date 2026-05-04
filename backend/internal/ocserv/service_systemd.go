package ocserv

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// SystemdManager manages a systemd service via systemctl commands.
type SystemdManager struct {
	unitName string
	runner   CommandRunner
}

// Compile-time interface satisfaction check.
var _ ServiceManager = (*SystemdManager)(nil)

// NewSystemdManager returns a SystemdManager for the given unit name.
func NewSystemdManager(unitName string, runner CommandRunner) *SystemdManager {
	return &SystemdManager{unitName: unitName, runner: runner}
}

// Status probes systemctl for the service's current state.
// Timeout: 5 seconds.
func (m *SystemdManager) Status(ctx context.Context) (ServiceStatus, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	result, err := m.runner.Run(ctx, "systemctl", []string{"status", m.unitName}, nil)
	if err != nil {
		// Binary or context error — treat as not found rather than hard error.
		return ServiceNotFound, nil
	}
	switch {
	case strings.Contains(result.Stdout, "active (running)"):
		return ServiceActive, nil
	case strings.Contains(result.Stdout, "failed"):
		return ServiceFailed, nil
	case strings.Contains(result.Stdout, "inactive"):
		return ServiceInactive, nil
	default:
		return ServiceNotFound, nil
	}
}

// Start calls systemctl start.
// Timeout: 30 seconds.
func (m *SystemdManager) Start(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	result, err := m.runner.Run(ctx, "systemctl", []string{"start", m.unitName}, nil)
	if err != nil {
		return fmt.Errorf("start service %s: %w", m.unitName, err)
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("start service %s: %s", m.unitName, strings.TrimSpace(result.Stderr))
	}
	return nil
}

// Stop calls systemctl stop.
// Timeout: 10 seconds.
func (m *SystemdManager) Stop(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	result, err := m.runner.Run(ctx, "systemctl", []string{"stop", m.unitName}, nil)
	if err != nil {
		return fmt.Errorf("stop service %s: %w", m.unitName, err)
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("stop service %s: %s", m.unitName, strings.TrimSpace(result.Stderr))
	}
	return nil
}

// Restart calls systemctl restart.
// Timeout: 30 seconds.
func (m *SystemdManager) Restart(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	result, err := m.runner.Run(ctx, "systemctl", []string{"restart", m.unitName}, nil)
	if err != nil {
		return fmt.Errorf("restart service %s: %w", m.unitName, err)
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("restart service %s: %s", m.unitName, strings.TrimSpace(result.Stderr))
	}
	return nil
}

// IsEnabled calls systemctl is-enabled and returns true when the output is "enabled".
// Timeout: 5 seconds.
func (m *SystemdManager) IsEnabled(ctx context.Context) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	result, err := m.runner.Run(ctx, "systemctl", []string{"is-enabled", m.unitName}, nil)
	if err != nil {
		return false, nil
	}
	return strings.TrimSpace(result.Stdout) == "enabled", nil
}
