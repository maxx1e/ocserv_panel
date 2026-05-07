package logs

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"ocserv_panel/backend/internal/ocserv"
)

// JournalLogReader reads log lines from systemd journal via journalctl.
type JournalLogReader struct {
	unit       string
	sourceType SourceType
	runner     ocserv.CommandRunner
}

// NewJournalLogReader creates a JournalLogReader for the given systemd unit.
func NewJournalLogReader(unit string, sourceType SourceType, runner ocserv.CommandRunner) *JournalLogReader {
	return &JournalLogReader{unit: unit, sourceType: sourceType, runner: runner}
}

// Available checks whether journalctl can read logs for this unit.
func (r *JournalLogReader) Available(ctx context.Context) bool {
	result, err := r.runner.Run(ctx, "journalctl", []string{"-u", r.unit, "--no-pager", "-n", "0"}, nil)
	if err != nil {
		return false
	}
	return result.ExitCode == 0
}

// ReadLines reads log lines from the journal with filtering and pagination.
func (r *JournalLogReader) ReadLines(ctx context.Context, opts ReadOpts) (*ReadResult, error) {
	args := r.buildArgs(opts)
	result, err := r.runner.Run(ctx, "journalctl", args, nil)
	if err != nil {
		return nil, err
	}
	if result.ExitCode != 0 {
		return &ReadResult{Lines: nil, Total: 0, HasMore: false}, nil
	}

	// Parse journal JSON output
	rawLines := strings.Split(strings.TrimSpace(result.Stdout), "\n")
	parsed := make([]LogLine, 0, len(rawLines))
	year := time.Now().Year()

	for _, raw := range rawLines {
		if strings.TrimSpace(raw) == "" {
			continue
		}
		line := r.parseJournalEntry(raw, year)
		if line == nil {
			continue
		}
		parsed = append(parsed, *line)
	}

	// Apply in-memory filters (search, level) since journalctl only supports time-based filtering
	filtered := r.applyFilters(parsed, opts)

	// Sort
	if opts.Order == "desc" || opts.Order == "" {
		reverseLogLines(filtered)
	}

	total := len(filtered)

	// Paginate
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

	return &ReadResult{Lines: page, Total: total, HasMore: hasMore}, nil
}

func (r *JournalLogReader) buildArgs(opts ReadOpts) []string {
	args := []string{"-u", r.unit, "--output=json", "--no-pager"}

	// Limit number of entries fetched from journal (fetch more than page to allow filtering)
	fetchLimit := (opts.Limit + opts.Offset) * 3
	if fetchLimit < 500 {
		fetchLimit = 500
	}
	if fetchLimit > 5000 {
		fetchLimit = 5000
	}
	args = append(args, "-n", intToStr(fetchLimit))

	if !opts.From.IsZero() {
		args = append(args, "--since="+opts.From.Format("2006-01-02 15:04:05"))
	}
	if !opts.To.IsZero() {
		args = append(args, "--until="+opts.To.Format("2006-01-02 15:04:05"))
	}

	return args
}

func (r *JournalLogReader) parseJournalEntry(raw string, year int) *LogLine {
	var entry map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &entry); err != nil {
		return nil
	}

	message, _ := entry["MESSAGE"].(string)
	if message == "" {
		return nil
	}

	// For panel source, the MESSAGE field contains the zerolog JSON
	if r.sourceType == SourcePanel {
		line, err := ParsePanelLine(message)
		if err != nil {
			// Fallback: use raw message
			return &LogLine{
				Timestamp: journalTimestamp(entry),
				Level:     "info",
				Message:   message,
				Raw:       message,
			}
		}
		// Use journal timestamp if parser didn't extract one
		if line.Timestamp.IsZero() {
			line.Timestamp = journalTimestamp(entry)
		}
		return &line
	}

	// For ocserv, try syslog parse first, fallback to raw message
	line, err := ParseOcservLine(message, year)
	if err != nil {
		return &LogLine{
			Timestamp: journalTimestamp(entry),
			Level:     "info",
			Message:   message,
			Raw:       message,
		}
	}
	if line.Timestamp.IsZero() {
		line.Timestamp = journalTimestamp(entry)
	}
	return &line
}

func journalTimestamp(entry map[string]interface{}) time.Time {
	// __REALTIME_TIMESTAMP is microseconds since epoch
	if ts, ok := entry["__REALTIME_TIMESTAMP"].(string); ok {
		var usec int64
		for _, c := range ts {
			if c >= '0' && c <= '9' {
				usec = usec*10 + int64(c-'0')
			}
		}
		if usec > 0 {
			return time.Unix(usec/1_000_000, (usec%1_000_000)*1000)
		}
	}
	return time.Time{}
}

func (r *JournalLogReader) applyFilters(lines []LogLine, opts ReadOpts) []LogLine {
	if opts.Level == "" && opts.Search == "" {
		return lines
	}
	searchLower := strings.ToLower(opts.Search)
	filtered := make([]LogLine, 0, len(lines))
	for i := range lines {
		line := &lines[i]
		if opts.Level != "" && line.Level != opts.Level {
			continue
		}
		if opts.Search != "" {
			if !strings.Contains(strings.ToLower(line.Message), searchLower) &&
				!strings.Contains(strings.ToLower(line.Raw), searchLower) {
				continue
			}
		}
		filtered = append(filtered, *line)
	}
	return filtered
}

func intToStr(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	pos := len(buf)
	neg := n < 0
	if neg {
		n = -n
	}
	for n > 0 {
		pos--
		buf[pos] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		pos--
		buf[pos] = '-'
	}
	return string(buf[pos:])
}
