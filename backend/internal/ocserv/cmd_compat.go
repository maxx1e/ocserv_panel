package ocserv

import "ocserv_panel/backend/internal/ocserv/cmd"

// Re-export cmd types so that existing test files and external callers that
// reference these names via the ocserv package continue to compile.
// New code should import ocserv/cmd directly.
type CommandRunner = cmd.CommandRunner
type CommandResult = cmd.CommandResult
type FakeCommandRunner = cmd.FakeCommandRunner
type FakeCommandCall = cmd.FakeCommandCall
type OSCommandRunner = cmd.OSCommandRunner

// NewOSCommandRunner returns an OSCommandRunner that executes real system commands.
func NewOSCommandRunner() *cmd.OSCommandRunner { return cmd.NewOSCommandRunner() }

// NewFakeCommandRunner returns a FakeCommandRunner for testing.
func NewFakeCommandRunner() *cmd.FakeCommandRunner { return cmd.NewFakeCommandRunner() }
