// Package cmd defines safe command execution boundaries for ocserv operations.
package cmd

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os/exec"
)

// CommandResult holds the output of a command execution.
type CommandResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

// CommandRunner executes system commands with timeout and output capture.
// This interface enables testing by replacing real command execution with fakes.
type CommandRunner interface {
	// Run executes a command with the given arguments and returns the result.
	// The context is used for timeout/cancellation.
	// stdin, if not nil, is piped to the command's standard input.
	Run(ctx context.Context, name string, args []string, stdin io.Reader) (CommandResult, error)
}

// Compile-time interface satisfaction checks.
var (
	_ CommandRunner = (*OSCommandRunner)(nil)
	_ CommandRunner = (*FakeCommandRunner)(nil)
)

// OSCommandRunner executes real system commands via exec.Command.
type OSCommandRunner struct{}

// NewOSCommandRunner returns a CommandRunner that executes real commands.
func NewOSCommandRunner() *OSCommandRunner {
	return &OSCommandRunner{}
}

// Run executes a command, captures stdout and stderr, and returns the result.
// Args are passed separately to avoid shell injection.
func (r *OSCommandRunner) Run(ctx context.Context, name string, args []string, stdin io.Reader) (CommandResult, error) {
	_ = r // receiver not needed for now; may be used for config in the future

	cmd := exec.CommandContext(ctx, name, args...)
	if stdin != nil {
		cmd.Stdin = stdin
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	exitCode, isExitErr := exitCodeFromErr(err)
	result := CommandResult{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		ExitCode: exitCode,
	}

	if err != nil {
		// If it's an ExitError the command ran but exited non-zero;
		// return the result so callers can inspect stdout/stderr.
		if isExitErr {
			return result, nil
		}
		// Otherwise the command failed to run at all (e.g. binary not found,
		// context deadline exceeded). Propagate the raw error.
		return result, err
	}

	return result, nil
}

// exitCodeFromErr extracts the exit code from a command error.
// The second return value is true when the error was an *exec.ExitError
// (the command ran but exited non-zero).
func exitCodeFromErr(err error) (int, bool) {
	if err == nil {
		return 0, false
	}

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode(), true
	}

	return -1, false
}
