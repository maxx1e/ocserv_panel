package server

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"ocserv_panel/backend/internal/app"
	"ocserv_panel/backend/internal/auth"
	"ocserv_panel/backend/internal/ocserv"
)

// --- request/response types ---

type configReadResponse struct {
	Content  string               `json:"content"`
	Metadata configMetadataJSON   `json:"metadata"`
	Issues   []configIssueJSON    `json:"issues"`
}

type configMetadataJSON struct {
	Path         string    `json:"path"`
	LastModified time.Time `json:"lastModified"`
	Size         int64     `json:"size"`
}

type configIssueJSON struct {
	Line       int    `json:"line,omitempty"`
	Severity   string `json:"severity"`
	Key        string `json:"key"`
	Message    string `json:"message"`
	Suggestion string `json:"suggestion,omitempty"`
	FixedValue string `json:"fixedValue,omitempty"`
}

type configValidateRequest struct {
	Content string `json:"content"`
}

type configValidateResponse struct {
	Valid           bool              `json:"valid"`
	Issues          []configIssueJSON `json:"issues"`
	RequiresRestart bool              `json:"requiresRestart,omitempty"`
	RestartReasons  []string          `json:"restartReasons,omitempty"`
}

type configDiffRequest struct {
	Content string `json:"content"`
}

type configDiffResponse struct {
	Diff string `json:"diff"`
}

type configApplyRequest struct {
	Content string `json:"content"`
}

type configApplyResponse struct {
	BackupNumber  int               `json:"backupNumber"`
	ReloadSuccess bool              `json:"reloadSuccess"`
	ReloadError   string            `json:"reloadError,omitempty"`
	Issues        []configIssueJSON `json:"issues"`
}

type configBackupResponse struct {
	Number    int       `json:"number"`
	Timestamp time.Time `json:"timestamp"`
	Size      int64     `json:"size"`
}

type configBackupContentResponse struct {
	Number  int    `json:"number"`
	Content string `json:"content"`
}

type configRollbackResponse struct {
	ReloadSuccess bool   `json:"reloadSuccess"`
	ReloadError   string `json:"reloadError,omitempty"`
}

// --- handlers ---

// handleConfigRead returns the current ocserv config file content, metadata, and issues.
func (a *App) handleConfigRead(w http.ResponseWriter, r *http.Request) {
	principal, ok := a.authenticate(w, r)
	if !ok {
		_, _ = a.configService.ReadConfig(r.Context(), auth.Principal{})
		return
	}

	result, err := a.configService.ReadConfig(r.Context(), principal)
	if err != nil {
		a.writeConfigError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, configReadResponse{
		Content:  result.Content,
		Metadata: toConfigMetadataJSON(result.Metadata),
		Issues:   toConfigIssuesJSON(result.Issues),
	})
}

// handleConfigValidate validates proposed config content without writing to disk.
func (a *App) handleConfigValidate(w http.ResponseWriter, r *http.Request) {
	principal, ok := a.authenticate(w, r)
	if !ok {
		_, _ = a.configService.ValidateContent(r.Context(), auth.Principal{}, "")
		return
	}

	var req configValidateRequest
	if !decodeRequestJSON(w, r, &req) {
		return
	}
	if req.Content == "" {
		writeError(w, http.StatusBadRequest, "content is required")
		return
	}

	result, err := a.configService.ValidateContent(r.Context(), principal, req.Content)
	if err != nil {
		a.writeConfigError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, configValidateResponse{
		Valid:           result.Valid,
		Issues:          toConfigIssuesJSON(result.Issues),
		RequiresRestart: result.RequiresRestart,
		RestartReasons:  result.RestartReasons,
	})
}

// handleConfigDiff returns a unified diff between the current config and proposed content.
func (a *App) handleConfigDiff(w http.ResponseWriter, r *http.Request) {
	principal, ok := a.authenticate(w, r)
	if !ok {
		_, _ = a.configService.DiffWithContent(r.Context(), auth.Principal{}, "")
		return
	}

	var req configDiffRequest
	if !decodeRequestJSON(w, r, &req) {
		return
	}
	if req.Content == "" {
		writeError(w, http.StatusBadRequest, "content is required")
		return
	}

	diff, err := a.configService.DiffWithContent(r.Context(), principal, req.Content)
	if err != nil {
		a.writeConfigError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, configDiffResponse{Diff: diff})
}

// handleConfigApply validates, writes, and reloads ocserv with new config content.
func (a *App) handleConfigApply(w http.ResponseWriter, r *http.Request) {
	principal, ok := a.authenticate(w, r)
	if !ok {
		_, _ = a.configService.ApplyConfig(r.Context(), auth.Principal{}, "")
		return
	}

	var req configApplyRequest
	if !decodeRequestJSON(w, r, &req) {
		return
	}
	if req.Content == "" {
		writeError(w, http.StatusBadRequest, "content is required")
		return
	}

	result, err := a.configService.ApplyConfig(r.Context(), principal, req.Content)
	if err != nil {
		// ApplyConfig may return both a result (with issues) and an error.
		if errors.Is(err, app.ErrConfigValidationFailed) && len(result.Issues) > 0 {
			writeJSON(w, http.StatusUnprocessableEntity, configApplyResponse{
				Issues: toConfigIssuesJSON(result.Issues),
			})
			return
		}
		a.writeConfigError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, configApplyResponse{
		BackupNumber:  result.BackupNumber,
		ReloadSuccess: result.ReloadSuccess,
		ReloadError:   result.ReloadError,
		Issues:        toConfigIssuesJSON(result.Issues),
	})
}

// handleConfigBackups lists all available config backups.
func (a *App) handleConfigBackups(w http.ResponseWriter, r *http.Request) {
	principal, ok := a.authenticate(w, r)
	if !ok {
		_, _ = a.configService.ListBackups(r.Context(), auth.Principal{})
		return
	}

	backups, err := a.configService.ListBackups(r.Context(), principal)
	if err != nil {
		a.writeConfigError(w, err)
		return
	}

	resp := make([]configBackupResponse, 0, len(backups))
	for _, b := range backups {
		resp = append(resp, configBackupResponse{
			Number:    b.Number,
			Timestamp: b.Timestamp,
			Size:      b.Size,
		})
	}
	writeJSON(w, http.StatusOK, resp)
}

// handleConfigBackupGet returns the content of a specific backup.
func (a *App) handleConfigBackupGet(w http.ResponseWriter, r *http.Request) {
	principal, ok := a.authenticate(w, r)
	if !ok {
		_, _ = a.configService.GetBackup(r.Context(), auth.Principal{}, 0)
		return
	}

	n, ok := a.parseBackupNumber(w, r)
	if !ok {
		return
	}

	content, err := a.configService.GetBackup(r.Context(), principal, n)
	if err != nil {
		a.writeConfigError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, configBackupContentResponse{
		Number:  n,
		Content: content,
	})
}

// handleConfigBackupDiff returns a unified diff between backup n and the current config.
func (a *App) handleConfigBackupDiff(w http.ResponseWriter, r *http.Request) {
	principal, ok := a.authenticate(w, r)
	if !ok {
		_, _ = a.configService.DiffWithBackup(r.Context(), auth.Principal{}, 0)
		return
	}

	n, ok := a.parseBackupNumber(w, r)
	if !ok {
		return
	}

	diff, err := a.configService.DiffWithBackup(r.Context(), principal, n)
	if err != nil {
		a.writeConfigError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, configDiffResponse{Diff: diff})
}

// handleConfigRollback restores backup n as the live config and reloads ocserv.
func (a *App) handleConfigRollback(w http.ResponseWriter, r *http.Request) {
	principal, ok := a.authenticate(w, r)
	if !ok {
		_, _ = a.configService.Rollback(r.Context(), auth.Principal{}, 0)
		return
	}

	n, ok := a.parseBackupNumber(w, r)
	if !ok {
		return
	}

	result, err := a.configService.Rollback(r.Context(), principal, n)
	if err != nil {
		a.writeConfigError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, configRollbackResponse{
		ReloadSuccess: result.ReloadSuccess,
		ReloadError:   result.ReloadError,
	})
}

// handleConfigNetwork returns host network prerequisite checks.
func (a *App) handleConfigNetwork(w http.ResponseWriter, r *http.Request) {
	principal, ok := a.authenticate(w, r)
	if !ok {
		_, _ = a.configService.InspectNetwork(r.Context(), auth.Principal{})
		return
	}

	prereqs, err := a.configService.InspectNetwork(r.Context(), principal)
	if err != nil {
		a.writeConfigError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, prereqs)
}

// --- helpers ---

// parseBackupNumber extracts the {n} path variable and validates it is a positive integer.
// Returns (n, true) on success or writes a 400 error and returns (0, false).
func (a *App) parseBackupNumber(w http.ResponseWriter, r *http.Request) (int, bool) {
	raw := r.PathValue("n")
	if raw == "" {
		writeError(w, http.StatusBadRequest, "backup number is required")
		return 0, false
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 {
		writeError(w, http.StatusBadRequest, "backup number must be a positive integer")
		return 0, false
	}
	return n, true
}

// writeConfigError maps config service errors to appropriate HTTP responses.
func (a *App) writeConfigError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, app.ErrUnauthenticated):
		writeError(w, http.StatusUnauthorized, "authentication required")
	case errors.Is(err, app.ErrPermissionDenied):
		writeError(w, http.StatusForbidden, "permission denied")
	case errors.Is(err, app.ErrConfigBackupNotFound):
		writeError(w, http.StatusNotFound, "backup not found")
	case errors.Is(err, app.ErrConfigFileLocked):
		writeError(w, http.StatusConflict, "config file is locked by another operation")
	case errors.Is(err, app.ErrConfigValidationFailed):
		writeError(w, http.StatusUnprocessableEntity, "config validation failed")
	case errors.Is(err, ocserv.ErrConfigTooLarge):
		writeError(w, http.StatusBadRequest, "config content exceeds size limit")
	default:
		writeError(w, http.StatusInternalServerError, "internal server error")
	}
}

// toConfigMetadataJSON converts domain metadata to the JSON response type.
func toConfigMetadataJSON(m ocserv.ConfigMetadata) configMetadataJSON {
	return configMetadataJSON{
		Path:         m.Path,
		LastModified: m.LastModified,
		Size:         m.Size,
	}
}

// toConfigIssuesJSON converts a slice of domain issues to the JSON response type.
func toConfigIssuesJSON(issues []ocserv.ConfigIssue) []configIssueJSON {
	result := make([]configIssueJSON, 0, len(issues))
	for _, issue := range issues {
		result = append(result, configIssueJSON{
			Line:       issue.Line,
			Severity:   string(issue.Severity),
			Key:        issue.Key,
			Message:    issue.Message,
			Suggestion: issue.Suggestion,
			FixedValue: issue.FixedValue,
		})
	}
	return result
}
