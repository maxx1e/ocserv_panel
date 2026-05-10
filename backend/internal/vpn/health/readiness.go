package health

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"ocserv_panel/backend/internal/vpn/exec"
	"ocserv_panel/backend/internal/vpn/service"
)

// OcservReadiness describes the installation and runtime state of ocserv.
type OcservReadiness struct {
	BinaryPresent       bool   `json:"binary_present"`
	BinaryVersion       string `json:"binary_version,omitempty"`
	ConfigPresent       bool   `json:"config_present"`
	ConfigValid         bool   `json:"config_valid"`
	ConfigErrors        string `json:"config_errors,omitempty"`
	PasswordFilePresent bool   `json:"password_file_present"`
	ServiceRunning      bool   `json:"service_running"`
	SocketPresent       bool   `json:"socket_present"`
	OcctlPresent        bool   `json:"occtl_present"`
	OcpasswdPresent     bool   `json:"ocpasswd_present"`
}

// ReadinessConfig holds the paths needed for readiness checks.
type ReadinessConfig struct {
	OcservBinPath    string
	OcctlBinPath     string
	OcpasswdBinPath  string
	ConfigPath       string
	PasswordFilePath string
	SocketPath       string
	UnitName         string
}

// ReadinessChecker probes the system for ocserv installation and runtime state.
type ReadinessChecker struct {
	cfgFn  func() ReadinessConfig
	runner exec.CommandRunner
	svc    service.ServiceManager
}

// Compile-time check.
var _ interface {
	Check(ctx context.Context) (*OcservReadiness, error)
} = (*ReadinessChecker)(nil)

// NewReadinessChecker returns a ReadinessChecker that calls cfgFn on every
// Check invocation, so path changes written to the TOML config are picked up
// without a process restart.
func NewReadinessChecker(cfgFn func() ReadinessConfig, runner exec.CommandRunner, svc service.ServiceManager) *ReadinessChecker {
	return &ReadinessChecker{cfgFn: cfgFn, runner: runner, svc: svc}
}

// Check probes the system and returns the current ocserv readiness state.
// Each probe is independent — a failure in one does not block the others.
func (c *ReadinessChecker) Check(ctx context.Context) (*OcservReadiness, error) {
	cfg := c.cfgFn()
	r := &OcservReadiness{}

	// Binary existence checks (filesystem only — fast and safe).
	r.BinaryPresent = fileExists(cfg.OcservBinPath)
	r.OcctlPresent = fileExists(cfg.OcctlBinPath)
	r.OcpasswdPresent = fileExists(cfg.OcpasswdBinPath)

	// Version from binary — only if binary exists.
	if r.BinaryPresent {
		r.BinaryVersion = c.checkVersion(ctx, cfg)
	}

	// Config file presence.
	r.ConfigPresent = fileExists(cfg.ConfigPath)

	// Password file presence.
	r.PasswordFilePresent = fileExists(cfg.PasswordFilePath)

	// Config validation — only if both binary and config are present.
	if r.ConfigPresent && r.BinaryPresent {
		r.ConfigValid, r.ConfigErrors = c.validateConfig(ctx, cfg)
	}

	// Service status from service.ServiceManager.
	if status, err := c.svc.Status(ctx); err == nil {
		r.ServiceRunning = status == service.ServiceActive
	}

	// Management socket presence.
	r.SocketPresent = fileExists(cfg.SocketPath)

	return r, nil
}

// checkVersion runs the binary with --version and extracts the first line.
func (c *ReadinessChecker) checkVersion(ctx context.Context, cfg ReadinessConfig) string {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	result, err := c.runner.Run(ctx, cfg.OcservBinPath, []string{"--version"}, nil)
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
func (c *ReadinessChecker) validateConfig(ctx context.Context, cfg ReadinessConfig) (bool, string) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	result, err := c.runner.Run(ctx, cfg.OcservBinPath, []string{"-f", "-t"}, nil)
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
