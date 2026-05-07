package logs

import (
	"bufio"
	"context"
	"os"
	"strings"
	"time"
)

// SourceType identifies the log source format.
type SourceType string

const (
	SourcePanel  SourceType = "panel"
	SourceOcserv SourceType = "ocserv"
)

// maxScanBytes limits the file scan to prevent memory exhaustion.
const maxScanBytes = 10 * 1024 * 1024 // 10MB

// FileLogReader reads and parses log lines from a file on disk.
type FileLogReader struct {
	path       string
	sourceType SourceType
}

// NewFileLogReader creates a FileLogReader for the given path and source format.
func NewFileLogReader(path string, sourceType SourceType) *FileLogReader {
	return &FileLogReader{path: path, sourceType: sourceType}
}

// Available reports whether the log file exists and is readable.
func (r *FileLogReader) Available(_ context.Context) bool {
	info, err := os.Stat(r.path)
	return err == nil && !info.IsDir()
}

// ReadLines reads log lines from the file, applying filters and pagination.
func (r *FileLogReader) ReadLines(_ context.Context, opts ReadOpts) (*ReadResult, error) {
	f, err := os.Open(r.path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	// Read all lines (up to scan limit)
	allLines, err := r.scanLines(f)
	if err != nil {
		return nil, err
	}

	// Parse lines
	parsed := r.parseLines(allLines)

	// Apply filters
	filtered := r.applyFilters(parsed, opts)

	// Sort: desc = newest first (reverse of file order, assuming file is chronological)
	if opts.Order == "desc" || opts.Order == "" {
		reverseLogLines(filtered)
	}

	total := len(filtered)

	// Apply pagination
	offset := opts.Offset
	if offset < 0 {
		offset = 0
	}
	limit := opts.Limit
	if limit <= 0 {
		limit = 100
	}
	if limit > 500 {
		limit = 500
	}

	var page []LogLine
	hasMore := false
	if offset < total {
		end := offset + limit
		if end > total {
			end = total
		}
		page = filtered[offset:end]
		hasMore = end < total
	}

	return &ReadResult{
		Lines:   page,
		Total:   total,
		HasMore: hasMore,
	}, nil
}

func (r *FileLogReader) scanLines(f *os.File) ([]string, error) {
	// Limit read size
	info, err := f.Stat()
	if err != nil {
		return nil, err
	}

	size := info.Size()
	if size > maxScanBytes {
		// Seek to read only the last maxScanBytes
		if _, err := f.Seek(-maxScanBytes, 2); err != nil {
			return nil, err
		}
	}

	var lines []string
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) != "" {
			lines = append(lines, line)
		}
	}
	return lines, scanner.Err()
}

func (r *FileLogReader) parseLines(raw []string) []LogLine {
	year := time.Now().Year()
	parsed := make([]LogLine, 0, len(raw))
	for _, s := range raw {
		var line LogLine
		var err error
		switch r.sourceType {
		case SourcePanel:
			line, err = ParsePanelLine(s)
		case SourceOcserv:
			line, err = ParseOcservLine(s, year)
		default:
			continue
		}
		if err != nil {
			// Skip unparseable lines
			continue
		}
		parsed = append(parsed, line)
	}
	return parsed
}

func (r *FileLogReader) applyFilters(lines []LogLine, opts ReadOpts) []LogLine {
	filtered := make([]LogLine, 0, len(lines))
	searchLower := strings.ToLower(opts.Search)

	for i := range lines {
		line := &lines[i]

		// Level filter
		if opts.Level != "" && line.Level != opts.Level {
			continue
		}

		// Search filter (case-insensitive on message and raw)
		if opts.Search != "" {
			msgLower := strings.ToLower(line.Message)
			rawLower := strings.ToLower(line.Raw)
			if !strings.Contains(msgLower, searchLower) && !strings.Contains(rawLower, searchLower) {
				continue
			}
		}

		// Time range filter
		if !opts.From.IsZero() && line.Timestamp.Before(opts.From) {
			continue
		}
		if !opts.To.IsZero() && line.Timestamp.After(opts.To) {
			continue
		}

		filtered = append(filtered, *line)
	}
	return filtered
}

func reverseLogLines(lines []LogLine) {
	for i, j := 0, len(lines)-1; i < j; i, j = i+1, j-1 {
		lines[i], lines[j] = lines[j], lines[i]
	}
}
