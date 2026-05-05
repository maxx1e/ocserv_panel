package ocserv

import (
	"context"
	"testing"
	"time"
)

func TestConfigReloaderSuccess(t *testing.T) {
	t.Parallel()

	runner := NewFakeCommandRunner()
	runner.Results["occtl reload"] = CommandResult{Stdout: "", ExitCode: 0}

	r := NewConfigReloader(runner, "occtl")
	result, err := r.Reload(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Fatal("expected success")
	}
	if len(runner.Calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(runner.Calls))
	}
}

func TestConfigReloaderNonZeroExit(t *testing.T) {
	t.Parallel()

	runner := NewFakeCommandRunner()
	runner.Results["occtl reload"] = CommandResult{Stderr: "config error in line 5", ExitCode: 1}

	r := NewConfigReloader(runner, "occtl")
	result, err := r.Reload(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Success {
		t.Fatal("expected failure for non-zero exit")
	}
	if result.Stderr != "config error in line 5" {
		t.Fatalf("got stderr %q", result.Stderr)
	}
}

func TestConfigReloaderRunnerError(t *testing.T) {
	t.Parallel()

	runner := NewFakeCommandRunner()
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()
	time.Sleep(5 * time.Millisecond)

	runner.Err = ctx.Err()
	r := NewConfigReloader(runner, "occtl")
	_, err := r.Reload(ctx)
	if err == nil {
		t.Fatal("expected error for runner failure")
	}
}
