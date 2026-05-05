package ocserv

import (
	"context"
	"fmt"
	"time"
)

const reloadTimeout = 10 * time.Second

// ReloadResult carries the outcome of an "occtl reload" invocation.
type ReloadResult struct {
	Success bool   `json:"success"`
	Stderr  string `json:"stderr,omitempty"`
}

// ConfigReloader issues a live configuration reload to ocserv via occtl.
// It isolates the command execution behind the CommandRunner interface so
// tests can use FakeCommandRunner without a running ocserv instance.
type ConfigReloader struct {
	runner   CommandRunner
	occtlBin string
}

// NewConfigReloader returns a ConfigReloader that calls occtlBin for reload
// operations using the provided runner.
func NewConfigReloader(runner CommandRunner, occtlBin string) *ConfigReloader {
	return &ConfigReloader{runner: runner, occtlBin: occtlBin}
}

// Reload executes "occtl reload" with a 10-second timeout applied on top of
// any deadline already present in ctx.
//
// A non-zero exit code from occtl is not a Go error — it populates
// ReloadResult.Success=false and ReloadResult.Stderr so callers can surface
// the config error message to the user.
//
// A Go error is returned only when the command cannot be executed at all
// (binary missing, permission denied, context already cancelled).
func (r *ConfigReloader) Reload(ctx context.Context) (ReloadResult, error) {
	ctx, cancel := context.WithTimeout(ctx, reloadTimeout)
	defer cancel()

	result, err := r.runner.Run(ctx, r.occtlBin, []string{"reload"}, nil)
	if err != nil {
		return ReloadResult{}, fmt.Errorf("execute occtl reload: %w", err)
	}

	if result.ExitCode != 0 {
		return ReloadResult{Success: false, Stderr: result.Stderr}, nil
	}

	return ReloadResult{Success: true}, nil
}
