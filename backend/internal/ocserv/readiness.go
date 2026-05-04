package ocserv

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"
)

// OcservReadiness describes the installation and runtime state of ocserv.
type OcservReadiness struct {
	BinaryPresent   bool   `json:"binary_present"`
	BinaryVersion   string `json:"binary_version,omitempty"`
	ConfigPresent   bool   `json:"config_present"`
	ConfigValid     bool   `json:"config_valid"`
	ConfigErrors    string `json:"config_errors,omitempty"`
	ServiceRunning  bool   `json:"service_running"`
	SocketPresent   bool   `json:"socket_present"`
	OcctlPresent    bool   `json:"occtl_present"`
	OcpasswdPresent bool   `json:"ocpasswd_present"`
}

// ReadinessConfig holds the paths needed for readiness checks.
type ReadinessConfig struct {
	OcservBinPath   string
	OcctlBinPath    string
	OcpasswdBinPath string
	ConfigPath      string
	SocketPath      string
	UnitName        string
}

// ReadinessChecker probes the system for ocserv installation and runtime state.
type ReadinessChecker struct {
	cfg    ReadinessConfig
	runner CommandRunner
	svc    ServiceManager
}

// Compile-time check.
var _ interface {
	Check(ctx context.Context) (*OcservReadiness, error)
} = (*ReadinessChecker)(nil)

// NewReadinessChecker returns a ReadinessChecker with the given configuration.
func NewReadinessChecker(cfg ReadinessConfig, runner CommandRunner, svc ServiceManager) *ReadinessChecker {
	return &ReadinessChecker{cfg: cfg, runner: runner, svc: svc}
}

// Check probes the system and returns the current ocserv readiness state.
// Each probe is independent — a failure in one does not block the others.
func (c *ReadinessChecker) Check(ctx context.Context) (*OcservReadiness, error) {
	r := &OcservReadiness{}

	// Binary existence checks (filesystem only — fast and safe).
	r.BinaryPresent = fileExists(c.cfg.OcservBinPath)
	r.OcctlPresent = fileExists(c.cfg.OcctlBinPath)
	r.OcpasswdPresent = fileExists(c.cfg.OcpasswdBinPath)

	// Version from binary — only if binary exists.
	if r.BinaryPresent {
		r.BinaryVersion = c.checkVersion(ctx)
	}

	// Config file presence.
	r.ConfigPresent = fileExists(c.cfg.ConfigPath)

	// Config validation — only if both binary and config are present.
	if r.ConfigPresent && r.BinaryPresent {
		r.ConfigValid, r.ConfigErrors = c.validateConfig(ctx)
	}

	// Service status from ServiceManager.
	if status, err := c.svc.Status(ctx); err == nil {
		r.ServiceRunning = status == ServiceActive
	}

	// Management socket presence.
	r.SocketPresent = fileExists(c.cfg.SocketPath)

	return r, nil
}

// checkVersion runs the binary with --version and extracts the first line.
func (c *ReadinessChecker) checkVersion(ctx context.Context) string {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	result, err := c.runner.Run(ctx, c.cfg.OcservBinPath, []string{"--version"}, nil)
	if err != nil {
		return ""
	}
	// ocserv --version typically prints "ocserv 1.4.2" to stdout or stderr.
	out := result.Stdout
	if out == "" {
		out = result.Stderr
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) > 0 {
		return strings.TrimSpace(lines[0])
	}
	return ""
}

// validateConfig runs `ocserv -f -t` to check the configuration file.
func (c *ReadinessChecker) validateConfig(ctx context.Context) (bool, string) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	result, err := c.runner.Run(ctx, c.cfg.OcservBinPath, []string{"-f", "-t"}, nil)
	if err != nil {
		return false, fmt.Sprintf("config validation failed: %v", err)
	}
	if result.ExitCode != 0 {
		msg := strings.TrimSpace(result.Stderr)
		if msg == "" {
			msg = strings.TrimSpace(result.Stdout)
		}
		return false, msg
	}
	return true, ""
}

// fileExists returns true when the path refers to an existing file or directory.
func fileExists(path string) bool {
	if path == "" {
		return false
	}
	_, err := os.Stat(path)
	return err == nil
}
