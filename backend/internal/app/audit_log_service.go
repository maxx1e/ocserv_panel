package app

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"ocserv_panel/backend/internal/audit"
	"ocserv_panel/backend/internal/auth"
	"ocserv_panel/backend/internal/config"
	"ocserv_panel/backend/internal/logs"
	"ocserv_panel/backend/internal/rbac"
)

// ErrLogSourceUnavailable is returned when neither file nor journal reader is available.
var ErrLogSourceUnavailable = fmt.Errorf("log source unavailable")

// AuditLogQuery holds validated query parameters for log reading.
type AuditLogQuery struct {
	Search string
	Level  string
	From   time.Time
	To     time.Time
	Limit  int
	Offset int
	Order  string
}

// AuditLogResponse is the API response for log reading.
type AuditLogResponse struct {
	Source  string         `json:"source"`
	Lines   []logs.LogLine `json:"lines"`
	Total   int            `json:"total"`
	Offset  int            `json:"offset"`
	Limit   int            `json:"limit"`
	HasMore bool           `json:"hasMore"`
}

// LogSourceInfo describes an available log source.
type LogSourceInfo struct {
	Name          string `json:"name"`
	Label         string `json:"label"`
	Available     bool   `json:"available"`
	Path          string `json:"path"`
	Reader        string `json:"reader"`
	Reason        string `json:"reason"`
	LogLevel      *int   `json:"logLevel"`
	LogLevelLabel string `json:"logLevelLabel,omitempty"`
}

// validSources is the allowlist of accepted source values.
var validSources = map[string]bool{"panel": true, "ocserv": true}

// sourceLabels maps source names to display labels.
var sourceLabels = map[string]string{
	"panel":  "Web UI Backend Logs",
	"ocserv": "ocserv VPN Logs",
}

// maxReadTimeRange is the maximum allowed time range for normal reads.
const maxReadTimeRange = 7 * 24 * time.Hour

// maxExportTimeRange is the maximum allowed time range for exports.
const maxExportTimeRange = 30 * 24 * time.Hour

// maxExportLines is the maximum lines for export.
const maxExportLines = 5000

// AuditLogService provides log reading with RBAC and source resolution.
type AuditLogService struct {
	cfgFn          func() config.Config
	fileReaders    map[string]logs.LogReader
	journalReaders map[string]logs.LogReader
	recorder       audit.Recorder
	authorizer     rbac.Authorizer
}

// NewAuditLogService creates an AuditLogService.
func NewAuditLogService(
	cfgFn func() config.Config,
	fileReaders map[string]logs.LogReader,
	journalReaders map[string]logs.LogReader,
	recorder audit.Recorder,
	authorizer rbac.Authorizer,
) *AuditLogService {
	return &AuditLogService{
		cfgFn:          cfgFn,
		fileReaders:    fileReaders,
		journalReaders: journalReaders,
		recorder:       recorder,
		authorizer:     authorizer,
	}
}

// ReadLogs reads log lines from the specified source.
func (s *AuditLogService) ReadLogs(ctx context.Context, principal auth.Principal, source string, query AuditLogQuery) (AuditLogResponse, error) {
	if !principal.Authenticated() {
		return AuditLogResponse{}, ErrUnauthenticated
	}
	if !s.authorizer.Allowed(principalRoles(principal), rbac.PermissionAuditLogsRead) {
		return AuditLogResponse{}, ErrPermissionDenied
	}
	if !validSources[source] {
		return AuditLogResponse{}, ErrInvalidInput
	}
	if err := validateLogQuery(query, maxReadTimeRange); err != nil {
		return AuditLogResponse{}, err
	}

	reader := s.resolveReader(ctx, source)
	if reader == nil {
		return AuditLogResponse{}, ErrLogSourceUnavailable
	}

	limit := query.Limit
	if limit <= 0 {
		limit = 100
	}
	if limit > 500 {
		limit = 500
	}

	opts := logs.ReadOpts{
		Limit:  limit,
		Offset: query.Offset,
		Order:  query.Order,
		Search: query.Search,
		Level:  query.Level,
		From:   query.From,
		To:     query.To,
	}

	result, err := reader.ReadLines(ctx, opts)
	if err != nil {
		return AuditLogResponse{}, fmt.Errorf("read logs: %w", err)
	}

	for i := range result.Lines {
		logs.MaskLogLine(&result.Lines[i])
	}

	return AuditLogResponse{
		Source:  source,
		Lines:   result.Lines,
		Total:   result.Total,
		Offset:  query.Offset,
		Limit:   limit,
		HasMore: result.HasMore,
	}, nil
}

// ExportLogs exports filtered logs for download. Requires PermissionAuditLogsExport.
func (s *AuditLogService) ExportLogs(ctx context.Context, principal auth.Principal, source string, query AuditLogQuery, format string) ([]byte, error) {
	if !principal.Authenticated() {
		return nil, ErrUnauthenticated
	}
	if !s.authorizer.Allowed(principalRoles(principal), rbac.PermissionAuditLogsExport) {
		return nil, ErrPermissionDenied
	}
	if !validSources[source] {
		return nil, ErrInvalidInput
	}
	if format != "json" && format != "csv" {
		return nil, ErrInvalidInput
	}
	if err := validateLogQuery(query, maxExportTimeRange); err != nil {
		return nil, err
	}

	reader := s.resolveReader(ctx, source)
	if reader == nil {
		return nil, ErrLogSourceUnavailable
	}

	opts := logs.ReadOpts{
		Limit:  maxExportLines,
		Offset: 0,
		Order:  query.Order,
		Search: query.Search,
		Level:  query.Level,
		From:   query.From,
		To:     query.To,
	}

	result, err := reader.ReadLines(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("export logs: %w", err)
	}

	for i := range result.Lines {
		logs.MaskLogLine(&result.Lines[i])
	}

	if s.recorder != nil {
		actorID := principal.ID
		_ = s.recorder.Record(ctx, audit.Event{
			ActorID:      &actorID,
			Action:       "audit.logs.export",
			ResourceType: "logs",
			ResourceID:   source,
			Outcome:      audit.OutcomeSuccess,
			Message:      fmt.Sprintf("exported %d %s log lines as %s", len(result.Lines), source, format),
			Metadata:     map[string]string{"source": source, "format": format, "lines": fmt.Sprintf("%d", len(result.Lines))},
		})
	}

	switch format {
	case "csv":
		return exportCSV(result.Lines)
	default:
		return exportJSON(result.Lines)
	}
}

// GetSources returns metadata about available log sources.
func (s *AuditLogService) GetSources(ctx context.Context, principal auth.Principal) ([]LogSourceInfo, error) {
	if !principal.Authenticated() {
		return nil, ErrUnauthenticated
	}
	if !s.authorizer.Allowed(principalRoles(principal), rbac.PermissionAuditLogsRead) {
		return nil, ErrPermissionDenied
	}

	cfg := s.cfgFn()
	sources := []LogSourceInfo{}

	for _, name := range []string{"panel", "ocserv"} {
		info := LogSourceInfo{
			Name:  name,
			Label: sourceLabels[name],
		}

		switch name {
		case "panel":
			info.Path = cfg.Installation.PanelLogPath
		case "ocserv":
			info.Path = cfg.Installation.OcservLogPath
		}

		reader := s.resolveReader(ctx, name)
		info.Available = reader != nil

		if fileReader, ok := s.fileReaders[name]; ok && fileReader.Available(ctx) {
			info.Reader = "file"
		} else if journalReader, ok := s.journalReaders[name]; ok && journalReader.Available(ctx) {
			info.Reader = "journal"
		}

		if !info.Available {
			info.Reason = "service_not_running"
		}

		sources = append(sources, info)
	}

	return sources, nil
}

func (s *AuditLogService) resolveReader(ctx context.Context, source string) logs.LogReader {
	if reader, ok := s.fileReaders[source]; ok && reader.Available(ctx) {
		return reader
	}
	if reader, ok := s.journalReaders[source]; ok && reader.Available(ctx) {
		return reader
	}
	return nil
}

func validateLogQuery(query AuditLogQuery, maxRange time.Duration) error {
	if !query.From.IsZero() && !query.To.IsZero() {
		if query.From.After(query.To) {
			return ErrInvalidInput
		}
		if query.To.Sub(query.From) > maxRange {
			return ErrInvalidInput
		}
	}
	if query.Order != "" && query.Order != "asc" && query.Order != "desc" {
		return ErrInvalidInput
	}
	if query.Level != "" {
		switch query.Level {
		case "debug", "info", "warn", "error":
			// valid
		default:
			return ErrInvalidInput
		}
	}
	return nil
}

func exportJSON(lines []logs.LogLine) ([]byte, error) {
	return json.Marshal(lines)
}

func exportCSV(lines []logs.LogLine) ([]byte, error) {
	var buf strings.Builder
	w := csv.NewWriter(&buf)
	_ = w.Write([]string{"timestamp", "level", "component", "message"})
	for _, line := range lines {
		_ = w.Write([]string{
			line.Timestamp.Format(time.RFC3339),
			line.Level,
			line.Component,
			line.Message,
		})
	}
	w.Flush()
	return []byte(buf.String()), w.Error()
}
