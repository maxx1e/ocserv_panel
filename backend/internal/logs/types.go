// Package logs provides structured log reading from file and journal sources.
package logs

import (
	"context"
	"time"
)

// LogLine is a single parsed log entry.
type LogLine struct {
	Timestamp time.Time         `json:"timestamp"`
	Level     string            `json:"level"`
	Message   string            `json:"message"`
	Component string            `json:"component"`
	Raw       string            `json:"raw"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

// ReadOpts configures a log read operation.
type ReadOpts struct {
	Limit  int
	Offset int
	Order  string // "asc" or "desc"
	Search string
	Level  string
	From   time.Time
	To     time.Time
}

// ReadResult is the output of a log read operation.
type ReadResult struct {
	Lines   []LogLine
	Total   int
	HasMore bool
}

// LogReader reads structured log lines from a source.
type LogReader interface {
	// ReadLines reads log lines matching the given options.
	ReadLines(ctx context.Context, opts ReadOpts) (*ReadResult, error)
	// Available reports whether this reader can serve requests.
	Available(ctx context.Context) bool
}
