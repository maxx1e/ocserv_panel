// Package ocserv defines safe integration boundaries for ocserv operations.
package ocserv

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os/exec"
	"strings"
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

// FakeCommandRunner is a test double that returns pre-configured results.
// It records every call for later inspection.
type FakeCommandRunner struct {
	// Results maps a command key ("name arg1 arg2" or just "name") to its result.
	Results map[string]CommandResult
	// Err is returned alongside every Result (useful for simulating runner-level failures).
	Err error
	// Calls records every invocation of Run.
	Calls []FakeCommandCall
}

// FakeCommandCall records a single call to the fake runner.
type FakeCommandCall struct {
	Name string
	Args []string
}

// NewFakeCommandRunner returns a FakeCommandRunner that returns a zero-value
// success result for any unrecognised command.
func NewFakeCommandRunner() *FakeCommandRunner {
	return &FakeCommandRunner{
		Results: make(map[string]CommandResult),
	}
}

// Run records the call and returns the configured result.
// Lookup order: exact match ("name arg1 arg2"), then name-only fallback,
// finally the zero-value success default.
func (f *FakeCommandRunner) Run(ctx context.Context, name string, args []string, stdin io.Reader) (CommandResult, error) {
	_ = ctx   // context ignored in fake; test authors control timing via test helpers
	_ = stdin // stdin ignored in fake; test authors pre-configure Results

	f.Calls = append(f.Calls, FakeCommandCall{Name: name, Args: args})

	key := name + " " + strings.Join(args, " ")
	if result, ok := f.Results[key]; ok {
		return result, f.Err
	}

	if result, ok := f.Results[name]; ok {
		return result, f.Err
	}

	return CommandResult{}, f.Err
}
