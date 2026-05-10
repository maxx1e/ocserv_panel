package cmd

import (
	"context"
	"io"
	"strings"
)

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
