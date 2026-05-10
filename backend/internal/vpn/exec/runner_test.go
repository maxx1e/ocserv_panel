package exec_test

import (
	"context"
	"io"
	"strings"
	"testing"

	vpnexec "ocserv_panel/backend/internal/vpn/exec"
)

func TestFakeCommandRunner_DefaultSuccess(t *testing.T) {
	t.Parallel()

	f := vpnexec.NewFakeCommandRunner()
	result, err := f.Run(context.Background(), "occtl", []string{"show", "users"}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ExitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", result.ExitCode)
	}
	if len(f.Calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(f.Calls))
	}
}

func TestFakeCommandRunner_ExactMatch(t *testing.T) {
	t.Parallel()

	f := vpnexec.NewFakeCommandRunner()
	f.Results["occtl show users"] = vpnexec.CommandResult{
		Stdout:   "user1\nuser2\n",
		ExitCode: 0,
	}

	result, err := f.Run(context.Background(), "occtl", []string{"show", "users"}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Stdout != "user1\nuser2\n" {
		t.Fatalf("unexpected stdout: %q", result.Stdout)
	}
}

func TestFakeCommandRunner_NameFallback(t *testing.T) {
	t.Parallel()

	f := vpnexec.NewFakeCommandRunner()
	f.Results["occtl"] = vpnexec.CommandResult{
		Stdout:   "fallback-output\n",
		ExitCode: 0,
	}

	result, err := f.Run(context.Background(), "occtl", []string{"unknown", "subcommand"}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Stdout != "fallback-output\n" {
		t.Fatalf("unexpected stdout: %q", result.Stdout)
	}
}

func TestFakeCommandRunner_RecordsCalls(t *testing.T) {
	t.Parallel()

	f := vpnexec.NewFakeCommandRunner()
	_, _ = f.Run(context.Background(), "ocpasswd", []string{"-c", "config", "user1"}, strings.NewReader("password"))
	_, _ = f.Run(context.Background(), "systemctl", []string{"restart", "ocserv"}, nil)

	if len(f.Calls) != 2 {
		t.Fatalf("expected 2 calls, got %d", len(f.Calls))
	}

	call := f.Calls[0]
	if call.Name != "ocpasswd" {
		t.Fatalf("expected ocpasswd, got %s", call.Name)
	}
	if len(call.Args) != 3 {
		t.Fatalf("expected 3 args, got %d", len(call.Args))
	}

	call2 := f.Calls[1]
	if call2.Name != "systemctl" {
		t.Fatalf("expected systemctl, got %s", call2.Name)
	}
}

func TestFakeCommandRunner_WithErr(t *testing.T) {
	t.Parallel()

	f := vpnexec.NewFakeCommandRunner()
	f.Err = io.ErrUnexpectedEOF

	_, err := f.Run(context.Background(), "occtl", []string{"show", "users"}, nil)
	if err != io.ErrUnexpectedEOF {
		t.Fatalf("expected io.ErrUnexpectedEOF, got %v", err)
	}
}

func TestFakeCommandRunner_ImplementsCommandRunner(t *testing.T) {
	// Compile-time check is in runner.go; this verifies at test time too.
	var cr vpnexec.CommandRunner = vpnexec.NewFakeCommandRunner()
	_ = cr
}
