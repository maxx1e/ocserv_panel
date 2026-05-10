package app

import "os"

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
