package app

import (
	"bytes"
	"context"
	"fmt"
	"os"

	"ocserv_panel/backend/internal/auth"
	"ocserv_panel/backend/internal/config"
	"ocserv_panel/backend/internal/rbac"
)

const logTailMaxLines = 50
const logTailDefaultLines = 25

// LogFileReader is the interface used to read log files.
type LogFileReader interface {
	ReadFile(path string) ([]byte, error)
}

// OSLogFileReader reads files from the real filesystem.
type OSLogFileReader struct{}

func (r OSLogFileReader) ReadFile(path string) ([]byte, error) { return os.ReadFile(path) }

// FakeLogFileReader is a test double keyed by path. Missing key = file not found.
type FakeLogFileReader map[string][]byte

func (f FakeLogFileReader) ReadFile(path string) ([]byte, error) {
	data, ok := f[path]
	if !ok {
		return nil, os.ErrNotExist
	}
	return data, nil
}

// ErrorLogFileReader always returns an error for a specific path.
type ErrorLogFileReader struct {
	Path string
	Err  error
}

func (r ErrorLogFileReader) ReadFile(path string) ([]byte, error) {
	if path == r.Path {
		return nil, r.Err
	}
	return nil, os.ErrNotExist
}

// LogTailResult is the response payload for GET /api/logs/tail.
type LogTailResult struct {
	Source  string   `json:"source"`
	Enabled bool     `json:"enabled"`
	Present bool     `json:"present"`
	Path    string   `json:"path"`
	Lines   []string `json:"lines"`
	Error   string   `json:"error,omitempty"`
}

// LogTailService reads the tail of configured log files.
type LogTailService struct {
	cfgFn      func() config.Config
	reader     LogFileReader
	authorizer rbac.Authorizer
}

// NewLogTailService returns a LogTailService.
func NewLogTailService(cfgFn func() config.Config, reader LogFileReader, authorizer rbac.Authorizer) *LogTailService {
	return &LogTailService{cfgFn: cfgFn, reader: reader, authorizer: authorizer}
}

// TailLog returns the last N lines from the configured log file for source.
// source must be "ocserv" or "panel". lines is clamped to [1, 50].
func (s *LogTailService) TailLog(ctx context.Context, principal auth.Principal, source string, lines int) (LogTailResult, error) {
	if !principal.Authenticated() {
		return LogTailResult{}, ErrUnauthenticated
	}
	if !s.authorizer.Allowed(principalRoles(principal), rbac.PermissionReadDashboard) {
		return LogTailResult{}, ErrPermissionDenied
	}

	cfg := s.cfgFn()

	var path string
	switch source {
	case "ocserv":
		path = cfg.Installation.OcservLogPath
	case "panel":
		path = cfg.Installation.PanelLogPath
	default:
		return LogTailResult{}, fmt.Errorf("invalid log source %q: must be ocserv or panel", source)
	}

	if lines < 1 {
		lines = logTailDefaultLines
	}
	if lines > logTailMaxLines {
		lines = logTailMaxLines
	}

	enabled := cfg.Monitoring.LogFileReadEnabled
	if !enabled {
		return LogTailResult{Source: source, Enabled: false, Present: false, Path: path, Lines: []string{}}, nil
	}

	data, err := s.reader.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return LogTailResult{Source: source, Enabled: true, Present: false, Path: path, Lines: []string{}}, nil
		}
		return LogTailResult{
			Source:  source,
			Enabled: true,
			Present: false,
			Path:    path,
			Lines:   []string{},
			Error:   "log file could not be read",
		}, nil
	}

	tail := tailLines(data, lines)
	return LogTailResult{Source: source, Enabled: true, Present: true, Path: path, Lines: tail}, nil
}

// tailLines returns the last n lines from data.
func tailLines(data []byte, n int) []string {
	data = bytes.TrimRight(data, "\n\r")
	if len(data) == 0 {
		return []string{}
	}
	all := bytes.Split(data, []byte("\n"))
	if len(all) <= n {
		result := make([]string, 0, len(all))
		for _, l := range all {
			result = append(result, string(l))
		}
		return result
	}
	tail := all[len(all)-n:]
	result := make([]string, 0, len(tail))
	for _, l := range tail {
		result = append(result, string(l))
	}
	return result
}
